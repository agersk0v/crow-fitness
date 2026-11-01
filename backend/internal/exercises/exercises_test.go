package exercises

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type fakeStore struct {
	owner int64
	name  string
	err   error
}

func (f *fakeStore) List(_ context.Context, owner int64) ([]Exercise, error) {
	f.owner = owner
	return nil, f.err
}
func (f *fakeStore) Create(_ context.Context, owner int64, input Input) (Exercise, error) {
	f.owner = owner
	f.name = input.Name
	return Exercise{ID: "1", Name: input.Name}, f.err
}
func TestCreate(t *testing.T) {
	for _, tc := range []struct {
		body   string
		status int
		name   string
	}{
		{`{"name":"  Squat  ","muscle_groups":["chest"]}`, 201, "Squat"},
		{`{"name":"\u2003Press\u2003","muscle_groups":["chest"]}`, 201, "Press"},
		{`{"name":" \t "}`, 400, ""}, {`{}`, 400, ""},
		{`{"name":"Squat","owner_id":2}`, 400, ""},
		{`{"name":`, 400, ""}, {`{"name":2}`, 400, ""},
		{`{"name":"Squat","muscle_groups":["chest"]}{}`, 400, ""},
		{`{"name":"` + strings.Repeat("x", 17000) + `"}`, 413, ""},
	} {
		store := &fakeStore{}
		mux := http.NewServeMux()
		Register(mux, store)
		req := httptest.NewRequest("POST", "/api/exercises", strings.NewReader(tc.body))
		req.Header.Set("Content-Type", "application/json")
		response := httptest.NewRecorder()
		mux.ServeHTTP(response, req)
		if response.Code != tc.status || store.name != tc.name {
			t.Fatalf("%s: status=%d saved=%q", tc.body, response.Code, store.name)
		}
		if tc.status == 201 && store.owner != LocalOwnerID {
			t.Fatal("wrong owner")
		}
	}
}
func TestListAndErrors(t *testing.T) {
	store := &fakeStore{}
	mux := http.NewServeMux()
	Register(mux, store)
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, httptest.NewRequest("GET", "/api/exercises", nil))
	if response.Code != 200 || !strings.Contains(response.Body.String(), `"items":[]`) || store.owner != LocalOwnerID {
		t.Fatal(response.Body)
	}
	req := httptest.NewRequest("POST", "/api/exercises", strings.NewReader(`{"name":"Squat","muscle_groups":["chest"]}`))
	response = httptest.NewRecorder()
	mux.ServeHTTP(response, req)
	if response.Code != 415 {
		t.Fatal(response.Code)
	}
	store.err = errors.New("secret SQL details")
	for _, method := range []string{"GET", "POST"} {
		req = httptest.NewRequest(method, "/api/exercises", strings.NewReader(`{"name":"Squat","muscle_groups":["chest"]}`))
		req.Header.Set("Content-Type", "application/json")
		response = httptest.NewRecorder()
		mux.ServeHTTP(response, req)
		if response.Code != 500 || strings.Contains(response.Body.String(), "secret") {
			t.Fatal(response.Body)
		}
	}
}

func (f *fakeStore) Update(ctx context.Context, owner int64, id string, input Input) (Exercise, error) {
	return f.Create(ctx, owner, input)
}
func (f *fakeStore) Usage(_ context.Context, owner int64, id string) ([]TemplateReference, error) {
	f.owner = owner
	return []TemplateReference{}, f.err
}
func (f *fakeStore) Delete(_ context.Context, owner int64, id string) error {
	f.owner = owner
	return f.err
}

func (f *fakeStore) ListPage(_ context.Context, owner int64, query ListQuery) (Page, error) {
	f.owner = owner
	return Page{Items: []Exercise{}, MuscleGroups: MuscleGroups, Number: query.Page, Size: PageSize}, f.err
}

func TestPaginatedValidation(t *testing.T) {
	for _, query := range []string{"page=0", "page=-1", "page=1.5", "page=", "page=9223372036854775808", "page=1&page=2", "page=+1", "muscle_group=invalid", "q=a&q=b", "unknown=1", "paginated=false"} {
		mux := http.NewServeMux()
		Register(mux, &fakeStore{})
		response := httptest.NewRecorder()
		mux.ServeHTTP(response, httptest.NewRequest("GET", "/api/exercises?paginated=true&"+query, nil))
		if response.Code != 400 {
			t.Fatalf("%s: %d %s", query, response.Code, response.Body)
		}
	}
	store := &fakeStore{}
	mux := http.NewServeMux()
	Register(mux, store)
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, httptest.NewRequest("GET", "/api/exercises?paginated=true&page=3&q=press&muscle_group=chest", nil))
	if response.Code != 200 || store.owner != LocalOwnerID || !strings.Contains(response.Body.String(), `"page":3`) {
		t.Fatal(response.Body)
	}
	query, err := parseListQuery("paginated=true&q=%20A%25_%20&muscle_group=chest")
	if err != nil || query.Search != "A%_" || query.Page != 1 {
		t.Fatalf("%+v %v", query, err)
	}
}
