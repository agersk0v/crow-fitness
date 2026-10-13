package sessions

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/agersk0v/crow-fitness/backend/internal/exercises"
)

type fakeStore struct {
	owner      int64
	templateID string
	id         string
	calls      int
	err        error
}

func (f *fakeStore) Start(_ context.Context, owner int64, id string) (Session, error) {
	f.owner = owner
	f.templateID = id
	f.calls++
	return Session{ID: "7", TemplateID: id, Name: "Push day"}, f.err
}
func (f *fakeStore) Active(_ context.Context, owner int64) (*Session, error) {
	f.owner = owner
	return nil, f.err
}
func (f *fakeStore) Get(_ context.Context, owner int64, id string) (Session, error) {
	f.owner = owner
	f.id = id
	return Session{ID: id}, f.err
}
func (f *fakeStore) End(_ context.Context, owner int64, id string) error {
	f.owner = owner
	f.id = id
	f.calls++
	return f.err
}

func (f *fakeStore) Complete(ctx context.Context, owner int64, id string) error {
	return f.End(ctx, owner, id)
}
func (f *fakeStore) SaveSet(_ context.Context, owner int64, sessionID, id string, input SetInput) (SavedSet, error) {
	f.owner = owner
	f.calls++
	return SavedSet{ID: id, Position: input.Position, Weight: input.Weight, Reps: input.Reps}, f.err
}
func (f *fakeStore) DeleteSet(ctx context.Context, owner int64, sessionID, id string) error {
	return f.End(ctx, owner, sessionID)
}
func (f *fakeStore) CompletedSets(_ context.Context, owner int64) ([]CompletedSet, error) {
	f.owner = owner
	return nil, f.err
}

func (f *fakeStore) Previous(_ context.Context, owner int64, id string) ([]PreviousPerformance, error) {
	f.owner = owner
	f.id = id
	return nil, f.err
}
func (f *fakeStore) CompletedSessions(_ context.Context, owner int64) ([]CompletedSession, error) {
	f.owner = owner
	return nil, f.err
}
func (f *fakeStore) GetCompleted(ctx context.Context, owner int64, id string) (Session, error) {
	return f.Get(ctx, owner, id)
}

func (f *fakeStore) ExerciseHistory(_ context.Context, owner int64, id string) (ExerciseHistory, error) {
	f.owner = owner
	f.id = id
	return ExerciseHistory{ID: id, Name: "Press", Workouts: []ExerciseWorkout{}}, f.err
}

func TestHTTP(t *testing.T) {
	for _, tc := range []struct {
		name, method, path, body, content string
		err                               error
		status                            int
		calls                             int
	}{
		{name: "start", method: "POST", path: "/api/sessions", body: `{"template_id":"9"}`, content: "application/json", status: 201, calls: 1},
		{name: "browser owner rejected", method: "POST", path: "/api/sessions", body: `{"template_id":"9","owner_id":2}`, content: "application/json", status: 400},
		{name: "noncanonical id", method: "POST", path: "/api/sessions", body: `{"template_id":"09"}`, content: "application/json", status: 400},
		{name: "missing id", method: "POST", path: "/api/sessions", body: `{}`, content: "application/json", status: 400},
		{name: "null body", method: "POST", path: "/api/sessions", body: `null`, content: "application/json", status: 400},
		{name: "extra JSON", method: "POST", path: "/api/sessions", body: `{"template_id":"9"}{}`, content: "application/json", status: 400},
		{name: "oversized", method: "POST", path: "/api/sessions", body: strings.Repeat(" ", 4097), content: "application/json", status: 413},
		{name: "media", method: "POST", path: "/api/sessions", body: `{"template_id":"9"}`, status: 415},
		{name: "active conflict", method: "POST", path: "/api/sessions", body: `{"template_id":"9"}`, content: "application/json", err: ActiveConflict{Session: Session{ID: "8", Name: "Existing"}}, status: 409, calls: 1},
		{name: "archived", method: "POST", path: "/api/sessions", body: `{"template_id":"9"}`, content: "application/json", err: ErrArchived, status: 409, calls: 1},
		{name: "missing", method: "GET", path: "/api/sessions/7", err: ErrNotFound, status: 404},
		{name: "read failure", method: "GET", path: "/api/sessions/7", err: errors.New("private database details"), status: 500},
		{name: "no active", method: "GET", path: "/api/sessions/active", status: 200},
		{name: "cancel", method: "POST", path: "/api/sessions/7/cancel", status: 204, calls: 1},
		{name: "complete", method: "POST", path: "/api/sessions/7/complete", status: 204, calls: 1},
		{name: "invalid end id", method: "POST", path: "/api/sessions/0/cancel", status: 400},
		{name: "end failure", method: "POST", path: "/api/sessions/7/complete", err: errors.New("private database details"), status: 500, calls: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			store := &fakeStore{err: tc.err}
			mux := http.NewServeMux()
			Register(mux, store)
			request := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
			request.Header.Set("Content-Type", tc.content)
			response := httptest.NewRecorder()
			mux.ServeHTTP(response, request)
			if response.Code != tc.status || store.calls != tc.calls {
				t.Fatalf("status/calls: %d/%d %s", response.Code, store.calls, response.Body.String())
			}
			if store.owner != 0 && store.owner != exercises.LocalOwnerID {
				t.Fatal("browser selected owner")
			}
			if response.Header().Get("Cache-Control") != "no-store" {
				t.Fatal("cacheable session response")
			}
			if strings.Contains(response.Body.String(), "private database details") {
				t.Fatal("internal error leaked")
			}
			if tc.name == "active conflict" {
				var body struct {
					Error struct {
						Code    string  `json:"code"`
						Session Session `json:"session"`
					} `json:"error"`
				}
				if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil || body.Error.Code != "active_session_exists" || body.Error.Session.ID != "8" {
					t.Fatal("no continuation resource", response.Body.String())
				}
			}
			if tc.name == "no active" && strings.TrimSpace(response.Body.String()) != `{"session":null}` {
				t.Fatal(response.Body.String())
			}
			if tc.name == "start" && (store.templateID != "9" || response.Header().Get("Location") != "/api/sessions/7") {
				t.Fatal("incorrect start")
			}
			if tc.status == 204 && (store.id != "7" || response.Body.Len() != 0) {
				t.Fatal("incorrect ending")
			}
		})
	}
}
