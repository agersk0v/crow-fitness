package templates

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func integer(n int64) *int64    { return &n }
func decimal(s string) *Decimal { v := Decimal(s); return &v }
func validInput() Input {
	return Input{Name: " Push day ", Exercises: []InputEntry{{ExerciseID: "1", SetCount: 3, RepMin: integer(8), RepMax: integer(12), TargetRIR: decimal("1.5")}}}
}
func TestValidation(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(*Input)
		field  string
	}{
		{"valid", func(i *Input) {}, ""},
		{"blank name", func(i *Input) { i.Name = " \t " }, "name"},
		{"no exercises", func(i *Input) { i.Exercises = nil }, "exercises"},
		{"bad id", func(i *Input) { i.Exercises[0].ExerciseID = "-1" }, "exercises.0.exercise_id"},
		{"zero sets", func(i *Input) { i.Exercises[0].SetCount = 0 }, "exercises.0.set_count"},
		{"negative sets", func(i *Input) { i.Exercises[0].SetCount = -1 }, "exercises.0.set_count"},
		{"missing bound", func(i *Input) { i.Exercises[0].RepMin = nil }, "exercises.0.rep_range"},
		{"zero reps", func(i *Input) { i.Exercises[0].RepMin = integer(0) }, "exercises.0.rep_range"},
		{"reversed reps", func(i *Input) { i.Exercises[0].RepMin = integer(13) }, "exercises.0.rep_range"},
		{"negative RIR", func(i *Input) { i.Exercises[0].TargetRIR = decimal("-0.1") }, "exercises.0.target_rir"},
		{"zero RIR", func(i *Input) { i.Exercises[0].TargetRIR = decimal("0") }, ""},
		{"optional", func(i *Input) {
			i.Exercises[0].RepMin = nil
			i.Exercises[0].RepMax = nil
			i.Exercises[0].TargetRIR = nil
		}, ""},
		{"fixed reps", func(i *Input) { i.Exercises[0].RepMax = integer(8) }, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			input := validInput()
			tc.change(&input)
			fields := Validate(&input)
			if tc.field == "" && len(fields) > 0 {
				t.Fatal(fields)
			}
			if tc.field != "" && fields[tc.field] == "" {
				t.Fatal(fields)
			}
			if input.Name != "" && input.Name != "Push day" {
				t.Fatal("name not trimmed")
			}
		})
	}
}

type fakeStore struct {
	owner int64
	input Input
	err   error
	calls int
}

func (f *fakeStore) Create(_ context.Context, owner int64, input Input) (Template, error) {
	f.owner = owner
	f.input = input
	f.calls++
	return Template{ID: "7", Name: input.Name, Exercises: []Entry{}}, f.err
}
func (f *fakeStore) List(_ context.Context, owner int64, archived bool) ([]Summary, error) {
	f.owner = owner
	return nil, f.err
}
func (f *fakeStore) Get(_ context.Context, owner int64, id string) (Template, error) {
	f.owner = owner
	return Template{ID: id, Exercises: []Entry{}}, f.err
}
func TestHTTP(t *testing.T) {
	for _, tc := range []struct {
		name, body, content string
		status              int
	}{
		{"valid", `{"name":" Push ","exercises":[{"exercise_id":"1","set_count":3,"target_rir":0}]}`, "application/json", 201},
		{"blank", `{"name":" ","exercises":[]}`, "application/json", 400},
		{"fractional sets", `{"name":"Push","exercises":[{"exercise_id":"1","set_count":1.5}]}`, "application/json", 400},
		{"fractional reps", `{"name":"Push","exercises":[{"exercise_id":"1","set_count":1,"rep_min":1.5,"rep_max":3}]}`, "application/json", 400},
		{"owner injection", `{"name":"Push","owner_id":2,"exercises":[]}`, "application/json", 400},
		{"nested unknown", `{"name":"Push","exercises":[{"exercise_id":"1","set_count":3,"position":2}]}`, "application/json", 400},
		{"quoted RIR", `{"name":"Push","exercises":[{"exercise_id":"1","set_count":3,"target_rir":"2"}]}`, "application/json", 400},
		{"extra JSON", `{} {}`, "application/json", 400},
		{"media", `{}`, "text/plain", 415},
		{"oversize", `{"name":"` + strings.Repeat("x", 66000) + `"}`, "application/json", 413},
	} {
		t.Run(tc.name, func(t *testing.T) {
			store := &fakeStore{}
			mux := http.NewServeMux()
			Register(mux, store)
			req := httptest.NewRequest("POST", "/api/templates", strings.NewReader(tc.body))
			req.Header.Set("Content-Type", tc.content)
			w := httptest.NewRecorder()
			mux.ServeHTTP(w, req)
			if w.Code != tc.status {
				t.Fatalf("status=%d body=%s", w.Code, w.Body)
			}
			if tc.status == 201 {
				if store.owner != 1 || store.input.Name != "Push" || w.Header().Get("Location") != "/api/templates/7" {
					t.Fatal("create scope/trim/location")
				}
			} else if store.calls != 0 {
				t.Fatal("invalid request reached persistence")
			}
		})
	}
}
func TestReadsAndStoreErrors(t *testing.T) {
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
	w := call("GET", "/api/templates", "")
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"items":[]`) || store.owner != 1 {
		t.Fatal(w.Body)
	}
	if w = call("GET", "/api/templates/7", ""); w.Code != 200 || store.owner != 1 {
		t.Fatal(w.Body)
	}
	if w = call("GET", "/api/templates/no", ""); w.Code != 400 {
		t.Fatal(w.Body)
	}
	store.err = ErrNotFound
	if w = call("GET", "/api/templates/7", ""); w.Code != 404 {
		t.Fatal(w.Body)
	}
	input, _ := json.Marshal(validInput())
	store.err = ValidationError{"exercises.0.exercise_id": "Unavailable exercise."}
	w = call("POST", "/api/templates", string(input))
	if w.Code != 400 || !strings.Contains(w.Body.String(), "Unavailable exercise") {
		t.Fatal(w.Body)
	}
	store.err = errors.New("secret database details")
	for _, method := range []string{"GET", "POST"} {
		w = call(method, "/api/templates", string(input))
		if w.Code != 500 || strings.Contains(w.Body.String(), "secret") {
			t.Fatal(w.Body)
		}
	}
}

func (f *fakeStore) Update(ctx context.Context, owner int64, id string, input Input) (Template, error) {
	return f.Create(ctx, owner, input)
}
func (f *fakeStore) Duplicate(ctx context.Context, owner int64, id, name string) (Template, error) {
	return f.Create(ctx, owner, Input{Name: name})
}
func (f *fakeStore) SetArchived(_ context.Context, owner int64, id string, archived bool) (Template, error) {
	f.owner = owner
	f.calls++
	return Template{ID: id, Archived: archived}, f.err
}
