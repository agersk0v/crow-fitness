package exercises

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
)

func TestTags(t *testing.T) {
	input := Input{Name: " Press ", MuscleGroups: []string{"shoulders", "chest", "chest"}}
	if fields := Validate(&input); len(fields) > 0 || input.Name != "Press" || !reflect.DeepEqual(input.MuscleGroups, []string{"chest", "shoulders"}) {
		t.Fatalf("%+v %+v", input, fields)
	}
	input = Input{Name: "Press", MuscleGroups: []string{"custom"}}
	if Validate(&input)["muscle_groups"] == "" {
		t.Fatal("unknown tag accepted")
	}
	input = Input{Name: "Press"}
	if Validate(&input)["muscle_groups"] != "Select at least one muscle group." {
		t.Fatal("missing tags accepted")
	}
}
func TestManagementHTTP(t *testing.T) {
	store := &fakeStore{}
	mux := http.NewServeMux()
	Register(mux, store)
	call := func(method, path, body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)
		return w
	}
	w := call("PUT", "/api/exercises/1", `{"name":" Press ","muscle_groups":["chest"]}`)
	if w.Code != 200 || store.name != "Press" || store.owner != LocalOwnerID {
		t.Fatal(w.Body)
	}
	for _, body := range []string{`{"name":" ","muscle_groups":[]}`, `{"name":"Press","muscle_groups":["custom"]}`, `{"name":"Press","muscle_groups":[null]}`, `{"name":"Press","owner_id":2}`} {
		w = call("PUT", "/api/exercises/1", body)
		if w.Code != 400 {
			t.Fatal(w.Body)
		}
	}
	w = call("GET", "/api/exercises/1/templates", "")
	if w.Code != 200 || store.owner != LocalOwnerID {
		t.Fatal(w.Body)
	}
	w = call("DELETE", "/api/exercises/1", "")
	if w.Code != 204 || w.Body.Len() != 0 || store.owner != LocalOwnerID {
		t.Fatal(w.Body)
	}
	store.err = ErrNotFound
	for _, method := range []string{"PUT", "DELETE", "GET"} {
		path := "/api/exercises/1"
		if method == "GET" {
			path += "/templates"
		}
		w = call(method, path, `{"name":"Press","muscle_groups":["chest"]}`)
		if w.Code != 404 {
			t.Fatal(w.Body)
		}
	}
	store.err = &InUseError{Templates: []TemplateReference{{ID: "7", Name: "Push"}}}
	w = call("DELETE", "/api/exercises/1", "")
	var body struct {
		Error struct {
			Code      string              `json:"code"`
			Templates []TemplateReference `json:"templates"`
		} `json:"error"`
	}
	if json.Unmarshal(w.Body.Bytes(), &body) != nil || w.Code != 409 || body.Error.Code != "exercise_in_use" || len(body.Error.Templates) != 1 {
		t.Fatal(w.Body)
	}
	for _, method := range []string{"PUT", "DELETE"} {
		w = call(method, "/api/exercises/not-an-id", `{"name":"Press","muscle_groups":["chest"]}`)
		if w.Code != 400 {
			t.Fatal(w.Body)
		}
	}
}

func TestRequiresMuscleGroupOnCreateAndUpdate(t *testing.T) {
	for _, method := range []string{"POST", "PUT"} {
		for _, body := range []string{`{"name":"Press"}`, `{"name":"Press","muscle_groups":null}`, `{"name":"Press","muscle_groups":[]}`} {
			store := &fakeStore{}
			mux := http.NewServeMux()
			Register(mux, store)
			path := "/api/exercises"
			if method == "PUT" {
				path += "/1"
			}
			req := httptest.NewRequest(method, path, strings.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			mux.ServeHTTP(w, req)
			if w.Code != 400 || !strings.Contains(w.Body.String(), "Select at least one muscle group.") || store.name != "" {
				t.Fatalf("%s %s: %d %s", method, body, w.Code, w.Body)
			}
		}
	}
}
