package sessions

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/agersk0v/crow-fitness/backend/internal/exercises"
	"github.com/agersk0v/crow-fitness/backend/internal/migrations"
	"github.com/agersk0v/crow-fitness/backend/internal/templates"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestHistoryHTTP(t *testing.T) {
	for _, tc := range []struct {
		path   string
		err    error
		status int
	}{
		{"/api/history/sessions", nil, 200}, {"/api/history/sessions/7", nil, 200},
		{"/api/history/sessions/0", nil, 400}, {"/api/history/sessions/7", ErrNotFound, 404},
		{"/api/history/sessions", errors.New("private history details"), 500},
	} {
		store := &fakeStore{err: tc.err}
		mux := http.NewServeMux()
		Register(mux, store)
		response := httptest.NewRecorder()
		mux.ServeHTTP(response, httptest.NewRequest("GET", tc.path, nil))
		if response.Code != tc.status || response.Header().Get("Cache-Control") != "no-store" {
			t.Fatal(response.Code, response.Body.String())
		}
		if tc.status == 200 && store.owner != exercises.LocalOwnerID {
			t.Fatal("incorrect owner")
		}
		if tc.path == "/api/history/sessions" && tc.status == 200 && response.Body.String() != "{\"items\":[]}\n" {
			t.Fatal(response.Body.String())
		}
	}
}
func TestPostgresHistory(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL required for disposable PostgreSQL test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if err = migrations.Apply(ctx, pool); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, "INSERT INTO owners(id,label) VALUES(91,'History test'),(92,'Other history test')"); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_, err := pool.Exec(context.Background(), `DELETE FROM workout_sessions WHERE owner_id IN(91,92);DELETE FROM template_exercises WHERE owner_id IN(91,92);DELETE FROM templates WHERE owner_id IN(91,92);DELETE FROM exercises WHERE owner_id IN(91,92);DELETE FROM owners WHERE id IN(91,92)`)
		if err != nil {
			t.Error(err)
		}
	}()
	store := Postgres{Pool: pool}
	library := exercises.Postgres{Pool: pool}
	ts := templates.Postgres{Pool: pool}
	press, err := library.Create(ctx, 91, exercises.Input{Name: "Captured press", MuscleGroups: []string{"chest"}})
	if err != nil {
		t.Fatal(err)
	}
	squat, err := library.Create(ctx, 91, exercises.Input{Name: "Captured squat", MuscleGroups: []string{"quadriceps"}})
	if err != nil {
		t.Fatal(err)
	}
	min, max := int64(8), int64(12)
	zero := templates.Decimal("0")
	template, err := ts.Create(ctx, 91, templates.Input{Name: "Same routine", Exercises: []templates.InputEntry{{ExerciseID: press.ID, SetCount: 3, RepMin: &min, RepMax: &max, TargetRIR: &zero}, {ExerciseID: squat.ID, SetCount: 2}, {ExerciseID: press.ID, SetCount: 1}}})
	if err != nil {
		t.Fatal(err)
	}
	first, err := store.Start(ctx, 91, template.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = store.GetCompleted(ctx, 91, first.ID); !errors.Is(err, ErrNotFound) {
		t.Fatal("active session exposed as completed", err)
	}
	if list, err := store.CompletedSessions(ctx, 91); err != nil || len(list) != 0 {
		t.Fatal("active session listed", list, err)
	}
	if _, err = store.SaveSet(ctx, 91, first.ID, setID, SetInput{Position: 1, Weight: "0.00", Reps: 8}); err != nil {
		t.Fatal(err)
	}
	if _, err = store.SaveSet(ctx, 91, first.ID, "22222222-2222-4222-8222-222222222222", SetInput{Position: 3, Weight: "12.50", Reps: 5}); err != nil {
		t.Fatal(err)
	}
	if err = store.Complete(ctx, 91, first.ID); err != nil {
		t.Fatal(err)
	}
	saved, err := store.GetCompleted(ctx, 91, first.ID)
	if err != nil || saved.CompletedAt == nil || len(saved.Sets) != 2 || !reflect.DeepEqual(saved.Exercises, template.Exercises) {
		t.Fatal("completed detail changed snapshot", saved, err)
	}
	if saved.Sets[0].Weight != "0.00" || saved.Sets[1].Weight != "12.50" || saved.Sets[1].Position != 3 {
		t.Fatal("set values/occurrences changed", saved.Sets)
	}
	empty, err := store.Start(ctx, 91, template.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err = store.Complete(ctx, 91, empty.ID); err != nil {
		t.Fatal(err)
	}
	// Empty completed sessions remain listable and inspectable despite no set rows.
	emptyDetail, err := store.GetCompleted(ctx, 91, empty.ID)
	if err != nil || len(emptyDetail.Sets) != 0 || len(emptyDetail.Exercises) != 3 {
		t.Fatal("empty workout dropped", emptyDetail, err)
	}
	if _, err = pool.Exec(ctx, "UPDATE workout_sessions SET completed_at='2030-01-01T00:00:00Z' WHERE owner_id=91 AND id IN($1,$2)", first.ID, empty.ID); err != nil {
		t.Fatal(err)
	}
	list, err := store.CompletedSessions(ctx, 91)
	if err != nil || len(list) != 2 || list[0].ID != empty.ID || list[1].ID != first.ID || list[0].Name != list[1].Name {
		t.Fatal("repeated workouts/tied order", list, err)
	}
	saved, err = store.GetCompleted(ctx, 91, first.ID)
	if err != nil {
		t.Fatal(err)
	}
	cancelled, err := store.Start(ctx, 91, template.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err = store.End(ctx, 91, cancelled.ID); err != nil {
		t.Fatal(err)
	}
	active, err := store.Start(ctx, 91, template.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{active.ID, cancelled.ID, "9223372036854775807"} {
		if _, err = store.GetCompleted(ctx, 91, id); !errors.Is(err, ErrNotFound) {
			t.Fatal("non-completed workout exposed", id, err)
		}
	}
	if foreign, err := store.CompletedSessions(ctx, 92); err != nil || len(foreign) != 0 {
		t.Fatal("foreign list exposed", foreign, err)
	}
	if _, err = store.GetCompleted(ctx, 92, first.ID); !errors.Is(err, ErrNotFound) {
		t.Fatal("foreign detail exposed", err)
	}
	if _, err = ts.Update(ctx, 91, template.ID, templates.Input{Name: "Changed template", Exercises: []templates.InputEntry{{ExerciseID: squat.ID, SetCount: 9}}}); err != nil {
		t.Fatal(err)
	}
	if _, err = library.Update(ctx, 91, squat.ID, exercises.Input{Name: "Renamed squat", MuscleGroups: []string{"quadriceps"}}); err != nil {
		t.Fatal(err)
	}
	if err = library.Delete(ctx, 91, press.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = ts.SetArchived(ctx, 91, template.ID, true); err != nil {
		t.Fatal(err)
	}
	after, err := store.GetCompleted(ctx, 91, first.ID)
	if err != nil || !reflect.DeepEqual(saved, after) {
		t.Fatal("history changed after template/library changes", after, err)
	}
	afterList, err := store.CompletedSessions(ctx, 91)
	if err != nil || !reflect.DeepEqual(list, afterList) {
		t.Fatal("active/cancelled records listed or captured names changed", afterList, err)
	}
	fresh, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer fresh.Close()
	reopened, err := (Postgres{Pool: fresh}).GetCompleted(ctx, 91, first.ID)
	if err != nil || !reflect.DeepEqual(saved, reopened) {
		t.Fatal("detail lost on reconnect", reopened, err)
	}
}
