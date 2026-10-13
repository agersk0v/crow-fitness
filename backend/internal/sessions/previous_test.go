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

func TestPreviousHTTP(t *testing.T) {
	for _, tc := range []struct {
		id     string
		err    error
		status int
	}{{"7", nil, 200}, {"0", nil, 400}, {"7", ErrNotFound, 404}, {"7", errors.New("private history details"), 500}} {
		store := &fakeStore{err: tc.err}
		mux := http.NewServeMux()
		Register(mux, store)
		response := httptest.NewRecorder()
		mux.ServeHTTP(response, httptest.NewRequest("GET", "/api/sessions/"+tc.id+"/previous", nil))
		if response.Code != tc.status || response.Header().Get("Cache-Control") != "no-store" {
			t.Fatal(response.Code, response.Body.String())
		}
		if tc.status == 200 && (response.Body.String() != "{\"items\":[]}\n" || store.owner != exercises.LocalOwnerID || store.id != "7") {
			t.Fatal("incorrect scope/empty representation", response.Body.String())
		}
	}
}
func TestPostgresPrevious(t *testing.T) {
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
	if _, err = pool.Exec(ctx, "INSERT INTO owners(id,label) VALUES(81,'Previous performance test'),(82,'Other previous owner')"); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_, err := pool.Exec(context.Background(), `DELETE FROM workout_sessions WHERE owner_id IN(81,82);DELETE FROM template_exercises WHERE owner_id IN(81,82);DELETE FROM templates WHERE owner_id IN(81,82);DELETE FROM exercises WHERE owner_id IN(81,82);DELETE FROM owners WHERE id IN(81,82)`)
		if err != nil {
			t.Error(err)
		}
	}()
	library := exercises.Postgres{Pool: pool}
	ts := templates.Postgres{Pool: pool}
	store := Postgres{Pool: pool}
	createExercise := func(name string) string {
		t.Helper()
		e, err := library.Create(ctx, 81, exercises.Input{Name: name, MuscleGroups: []string{"chest"}})
		if err != nil {
			t.Fatal(err)
		}
		return e.ID
	}
	press, squat, unseen := createExercise("Press"), createExercise("Squat"), createExercise("Press")
	source, err := ts.Create(ctx, 81, templates.Input{Name: "Source", Exercises: []templates.InputEntry{{ExerciseID: press, SetCount: 3}, {ExerciseID: squat, SetCount: 2}, {ExerciseID: press, SetCount: 2}}})
	if err != nil {
		t.Fatal(err)
	}
	first, err := store.Start(ctx, 81, source.ID)
	if err != nil {
		t.Fatal(err)
	}
	save := func(sessionID, id string, pos int64, weight string) {
		t.Helper()
		if _, err := store.SaveSet(ctx, 81, sessionID, id, SetInput{Position: pos, Weight: weight, Reps: 8}); err != nil {
			t.Fatal(err)
		}
	}
	save(first.ID, setID, 1, "1.50")
	save(first.ID, "22222222-2222-4222-8222-222222222222", 2, "20.25")
	if err = store.Complete(ctx, 81, first.ID); err != nil {
		t.Fatal(err)
	}
	second, err := store.Start(ctx, 81, source.ID)
	if err != nil {
		t.Fatal(err)
	}
	save(second.ID, setID, 1, "0.00")
	save(second.ID, "22222222-2222-4222-8222-222222222222", 1, "12.50")
	save(second.ID, "33333333-3333-4333-8333-333333333333", 3, "30.75")
	if err = store.Complete(ctx, 81, second.ID); err != nil {
		t.Fatal(err)
	}
	// Equal completion timestamps resolve by newest session identity, not set date.
	if _, err = pool.Exec(ctx, "UPDATE workout_sessions SET completed_at='2030-01-01T00:00:00Z' WHERE owner_id=81 AND id IN($1,$2)", first.ID, second.ID); err != nil {
		t.Fatal(err)
	}
	// Newer completed sessions with no eligible sets must not hide older results.
	empty, err := store.Start(ctx, 81, source.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err = store.Complete(ctx, 81, empty.ID); err != nil {
		t.Fatal(err)
	}
	deleted, err := store.Start(ctx, 81, source.ID)
	if err != nil {
		t.Fatal(err)
	}
	save(deleted.ID, setID, 1, "999")
	if err = store.DeleteSet(ctx, 81, deleted.ID, setID); err != nil {
		t.Fatal(err)
	}
	if err = store.Complete(ctx, 81, deleted.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, "UPDATE workout_sessions SET completed_at='2031-01-01T00:00:00Z' WHERE owner_id=81 AND id IN($1,$2)", empty.ID, deleted.ID); err != nil {
		t.Fatal(err)
	}
	cancelled, err := store.Start(ctx, 81, source.ID)
	if err != nil {
		t.Fatal(err)
	}
	save(cancelled.ID, setID, 1, "888")
	if err = store.End(ctx, 81, cancelled.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = library.Update(ctx, 81, press, exercises.Input{Name: "Renamed press", MuscleGroups: []string{"chest"}}); err != nil {
		t.Fatal(err)
	}
	if _, err = ts.SetArchived(ctx, 81, source.ID, true); err != nil {
		t.Fatal(err)
	}
	currentTemplate, err := ts.Create(ctx, 81, templates.Input{Name: "Different template", Exercises: []templates.InputEntry{{ExerciseID: press, SetCount: 1}, {ExerciseID: unseen, SetCount: 1}, {ExerciseID: press, SetCount: 4}, {ExerciseID: squat, SetCount: 1}}})
	if err != nil {
		t.Fatal(err)
	}
	current, err := store.Start(ctx, 81, currentTemplate.ID)
	if err != nil {
		t.Fatal(err)
	}
	save(current.ID, setID, 1, "777")
	before, err := store.Get(ctx, 81, current.ID)
	if err != nil {
		t.Fatal(err)
	}
	historyBefore, err := store.CompletedSets(ctx, 81)
	if err != nil {
		t.Fatal(err)
	}
	previous, err := store.Previous(ctx, 81, current.ID)
	if err != nil || len(previous) != 2 {
		t.Fatal("history selection", previous, err)
	}
	refs := map[string]PreviousPerformance{}
	for _, ref := range previous {
		refs[ref.ExerciseID] = ref
	}
	ref := refs[press]
	if ref.SessionID != second.ID || len(ref.Occurrences) != 2 || ref.Occurrences[0].Position != 1 || ref.Occurrences[1].Position != 3 || ref.Occurrences[0].Name != "Press" || len(ref.Occurrences[0].Sets) != 2 || ref.Occurrences[0].Sets[0].Weight != "0.00" || ref.Occurrences[0].Sets[1].Weight != "12.50" || ref.Occurrences[1].Sets[0].Weight != "30.75" {
		t.Fatal("repeated source occurrences/decimal order", ref)
	}
	if refs[squat].SessionID != first.ID || refs[squat].Occurrences[0].Sets[0].Weight != "20.25" {
		t.Fatal("selected globally latest instead of latest per exercise", refs[squat])
	}
	if _, found := refs[unseen]; found {
		t.Fatal("matched history by name instead of identity")
	}
	if _, err = store.Previous(ctx, 82, current.ID); !errors.Is(err, ErrNotFound) {
		t.Fatal("foreign session history exposed", err)
	}
	if _, err = store.Previous(ctx, 81, second.ID); !errors.Is(err, ErrNotFound) {
		t.Fatal("completed session accepted as active")
	}
	after, err := store.Get(ctx, 81, current.ID)
	if err != nil || !reflect.DeepEqual(before, after) {
		t.Fatal("reference read changed current sets", after, err)
	}
	historyAfter, err := store.CompletedSets(ctx, 81)
	if err != nil || !reflect.DeepEqual(historyBefore, historyAfter) {
		t.Fatal("reference read changed history", err)
	}
	fresh, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer fresh.Close()
	reloaded, err := (Postgres{Pool: fresh}).Previous(ctx, 81, current.ID)
	if err != nil || !reflect.DeepEqual(previous, reloaded) {
		t.Fatal("reference reconnect changed results", reloaded, err)
	}
}
