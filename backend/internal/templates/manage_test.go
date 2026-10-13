package templates

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

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
	input := `{"name":" Updated ","exercises":[{"exercise_id":"1","set_count":3}]}`
	w := call("PUT", "/api/templates/7", input)
	if w.Code != 200 || store.owner != 1 || store.input.Name != "Updated" {
		t.Fatal(w.Body)
	}
	before := store.calls
	w = call("PUT", "/api/templates/7", `{"name":"Invalid","exercises":[]}`)
	if w.Code != 400 || store.calls != before {
		t.Fatal(w.Body)
	}
	w = call("POST", "/api/templates/7/duplicate", `{"name":" Copy "}`)
	if w.Code != 201 || store.input.Name != "Copy" || store.owner != 1 || w.Header().Get("Location") != "/api/templates/7" {
		t.Fatal(w.Body)
	}
	before = store.calls
	for _, body := range []string{`{"name":" "}`, `{"name":"Copy","exercises":[]}`, `{"name":"Copy","owner_id":2}`} {
		w = call("POST", "/api/templates/7/duplicate", body)
		if w.Code != 400 || store.calls != before {
			t.Fatal(w.Body)
		}
	}
	w = call("POST", "/api/templates/7/archive", "")
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"archived":true`) {
		t.Fatal(w.Body)
	}
	w = call("POST", "/api/templates/7/restore", "")
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"archived":false`) {
		t.Fatal(w.Body)
	}
	for _, query := range []string{"true", "false"} {
		w = call("GET", "/api/templates?archived="+query, "")
		if w.Code != 200 || store.owner != 1 {
			t.Fatal(w.Body)
		}
	}
	for _, query := range []string{"maybe", "true&archived=false"} {
		w = call("GET", "/api/templates?archived="+query, "")
		if w.Code != 400 {
			t.Fatal(w.Body)
		}
	}
	store.err = ErrArchived
	for _, method := range []string{"PUT", "POST"} {
		path := "/api/templates/7"
		body := input
		if method == "POST" {
			path += "/duplicate"
			body = `{"name":"Copy"}`
		}
		w = call(method, path, body)
		if w.Code != 409 || !strings.Contains(w.Body.String(), "template_archived") {
			t.Fatal(w.Body)
		}
	}
	store.err = ErrNotFound
	for _, action := range []string{"archive", "restore", "duplicate"} {
		w = call("POST", "/api/templates/7/"+action, `{"name":"Copy"}`)
		if w.Code != 404 {
			t.Fatal(w.Body)
		}
	}
	w = call("PUT", "/api/templates/7", input)
	if w.Code != 404 {
		t.Fatal(w.Body)
	}
	w = call("DELETE", "/api/templates/7", "")
	if w.Code != 405 {
		t.Fatal("permanent delete unexpectedly available", w.Code)
	}
}
