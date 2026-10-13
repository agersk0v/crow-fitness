package sessions

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/agersk0v/crow-fitness/backend/internal/exercises"
	"github.com/agersk0v/crow-fitness/backend/internal/migrations"
	"github.com/agersk0v/crow-fitness/backend/internal/templates"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestExerciseHistoryHTTP(t *testing.T) {
	for _, tc := range []struct {
		id     string
		err    error
		status int
	}{{"7", nil, 200}, {"0", nil, 400}, {"7", ErrNotFound, 404}, {"7", errors.New("private history details"), 500}} {
		store := &fakeStore{err: tc.err}
		mux := http.NewServeMux()
		Register(mux, store)
		response := httptest.NewRecorder()
		mux.ServeHTTP(response, httptest.NewRequest("GET", "/api/exercises/"+tc.id+"/history", nil))
		if response.Code != tc.status || response.Header().Get("Cache-Control") != "no-store" {
			t.Fatal(response.Code, response.Body.String())
		}
		if strings.Contains(response.Body.String(), "private history details") {
			t.Fatal("internal error exposed")
		}
		if tc.status == 200 && (store.owner != exercises.LocalOwnerID || store.id != "7" || !strings.Contains(response.Body.String(), `"workouts":[]`)) {
			t.Fatal("incorrect scope/empty response", response.Body.String())
		}
	}
}
func TestPostgresExerciseHistory(t *testing.T) {
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
	if _, err = pool.Exec(ctx, "INSERT INTO owners(id,label) VALUES(101,'Exercise history test'),(102,'Other exercise history test')"); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_, err := pool.Exec(context.Background(), `DELETE FROM workout_sessions WHERE owner_id IN(101,102);DELETE FROM template_exercises WHERE owner_id IN(101,102);DELETE FROM templates WHERE owner_id IN(101,102);DELETE FROM exercises WHERE owner_id IN(101,102);DELETE FROM owners WHERE id IN(101,102)`)
		if err != nil {
			t.Error(err)
		}
	}()
	library := exercises.Postgres{Pool: pool}
	ts := templates.Postgres{Pool: pool}
	store := Postgres{Pool: pool}
	createExercise := func(owner int64, name string) string {
		t.Helper()
		e, err := library.Create(ctx, owner, exercises.Input{Name: name, MuscleGroups: []string{"chest"}})
		if err != nil {
			t.Fatal(err)
		}
		return e.ID
	}
	press, squat, unseen, foreign := createExercise(101, "Press"), createExercise(101, "Squat"), createExercise(101, "Press"), createExercise(102, "Press")
	noHistory, err := store.ExerciseHistory(ctx, 101, unseen)
	if err != nil || noHistory.Name != "Press" || len(noHistory.Workouts) != 0 {
		t.Fatal("fresh exercise empty state", noHistory, err)
	}
	for _, id := range []string{foreign, "9223372036854775807"} {
		if _, err = store.ExerciseHistory(ctx, 101, id); !errors.Is(err, ErrNotFound) {
			t.Fatal("foreign/missing identity exposed", id, err)
		}
	}
	template, err := ts.Create(ctx, 101, templates.Input{Name: "Original workout", Exercises: []templates.InputEntry{{ExerciseID: press, SetCount: 3}, {ExerciseID: squat, SetCount: 1}, {ExerciseID: press, SetCount: 1}}})
	if err != nil {
		t.Fatal(err)
	}
	first, err := store.Start(ctx, 101, template.ID)
	if err != nil {
		t.Fatal(err)
	}
	save := func(sessionID, id string, position int64, weight string) {
		t.Helper()
		if _, err := store.SaveSet(ctx, 101, sessionID, id, SetInput{Position: position, Weight: weight, Reps: 8}); err != nil {
			t.Fatal(err)
		}
	}
	save(first.ID, setID, 1, "0.00")
	save(first.ID, "22222222-2222-4222-8222-222222222222", 1, "12.50")
	save(first.ID, "33333333-3333-4333-8333-333333333333", 3, "30.75")
	save(first.ID, "44444444-4444-4444-8444-444444444444", 2, "99")
	save(first.ID, "55555555-5555-4555-8555-555555555555", 1, "888")
	if err = store.DeleteSet(ctx, 101, first.ID, "55555555-5555-4555-8555-555555555555"); err != nil {
		t.Fatal(err)
	}
	if activeHistory, err := store.ExerciseHistory(ctx, 101, press); err != nil || len(activeHistory.Workouts) != 0 {
		t.Fatal("active sets exposed", activeHistory, err)
	}
	if err = store.Complete(ctx, 101, first.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = library.Update(ctx, 101, press, exercises.Input{Name: "Renamed press", MuscleGroups: []string{"chest"}}); err != nil {
		t.Fatal(err)
	}
	secondTemplate, err := ts.Create(ctx, 101, templates.Input{Name: "Other workout", Exercises: []templates.InputEntry{{ExerciseID: press, SetCount: 1}}})
	if err != nil {
		t.Fatal(err)
	}
	second, err := store.Start(ctx, 101, secondTemplate.ID)
	if err != nil {
		t.Fatal(err)
	}
	save(second.ID, setID, 1, "9.25")
	if err = store.Complete(ctx, 101, second.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, "UPDATE workout_sessions SET completed_at='2030-01-01T00:00:00Z' WHERE owner_id=101 AND id IN($1,$2)", first.ID, second.ID); err != nil {
		t.Fatal(err)
	}
	cancelled, err := store.Start(ctx, 101, template.ID)
	if err != nil {
		t.Fatal(err)
	}
	save(cancelled.ID, setID, 1, "777")
	if err = store.End(ctx, 101, cancelled.ID); err != nil {
		t.Fatal(err)
	}
	empty, err := store.Start(ctx, 101, template.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err = store.Complete(ctx, 101, empty.ID); err != nil {
		t.Fatal(err)
	}
	active, err := store.Start(ctx, 101, template.ID)
	if err != nil {
		t.Fatal(err)
	}
	save(active.ID, setID, 1, "666")
	beforeActive, err := store.Get(ctx, 101, active.ID)
	if err != nil {
		t.Fatal(err)
	}
	history, err := store.ExerciseHistory(ctx, 101, press)
	if err != nil || history.Name != "Renamed press" || len(history.Workouts) != 2 || history.Workouts[0].SessionID != second.ID || history.Workouts[1].SessionID != first.ID {
		t.Fatal("identity/ordering/filtering", history, err)
	}
	old := history.Workouts[1]
	if old.Name != "Original workout" || len(old.Occurrences) != 2 || old.Occurrences[0].Position != 1 || old.Occurrences[1].Position != 3 || old.Occurrences[0].Name != "Press" || len(old.Occurrences[0].Sets) != 2 || old.Occurrences[0].Sets[0].Weight != "0.00" || old.Occurrences[0].Sets[1].Weight != "12.50" || old.Occurrences[1].Sets[0].Weight != "30.75" {
		t.Fatal("occurrences/exact values/source order", old)
	}
	if history.Workouts[0].Name != "Other workout" || history.Workouts[0].Occurrences[0].Name != "Renamed press" {
		t.Fatal("cross-template copied names", history.Workouts[0])
	}
	if sameName, err := store.ExerciseHistory(ctx, 101, unseen); err != nil || len(sameName.Workouts) != 0 {
		t.Fatal("matched by name", sameName, err)
	}
	if _, err = store.ExerciseHistory(ctx, 102, press); !errors.Is(err, ErrNotFound) {
		t.Fatal("foreign completed identity exposed", err)
	}
	afterActive, err := store.Get(ctx, 101, active.ID)
	if err != nil || !reflect.DeepEqual(beforeActive, afterActive) {
		t.Fatal("history read changed active data", afterActive, err)
	}
	for _, id := range []string{template.ID, secondTemplate.ID} {
		if _, err = ts.Update(ctx, 101, id, templates.Input{Name: "Changed source", Exercises: []templates.InputEntry{{ExerciseID: squat, SetCount: 9}}}); err != nil {
			t.Fatal(err)
		}
		if _, err = ts.SetArchived(ctx, 101, id, true); err != nil {
			t.Fatal(err)
		}
	}
	if err = library.Delete(ctx, 101, press); err != nil {
		t.Fatal(err)
	}
	removed, err := store.ExerciseHistory(ctx, 101, press)
	if err != nil || !reflect.DeepEqual(history, removed) {
		t.Fatal("deleted library identity lost/rewrote history", removed, err)
	}
	if _, err = store.ExerciseHistory(ctx, 102, press); !errors.Is(err, ErrNotFound) {
		t.Fatal("foreign retained identity exposed", err)
	}
	fresh, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer fresh.Close()
	reloaded, err := (Postgres{Pool: fresh}).ExerciseHistory(ctx, 101, press)
	if err != nil || !reflect.DeepEqual(history, reloaded) {
		t.Fatal("history changed on reconnect", reloaded, err)
	}
}
