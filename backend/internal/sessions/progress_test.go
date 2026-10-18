package sessions

import (
	"context"
	"errors"
	"fmt"
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

func TestProgressHTTP(t *testing.T) {
	for _, tc := range []struct {
		query  string
		err    error
		status int
	}{
		{"", nil, 400}, {"exercise=0", nil, 400}, {"exercise=1&exercise=2", nil, 400}, {"exercise=1&tz=Local", nil, 400},
		{"exercise=1&from=2026-10-26&to=2026-10-25", nil, 400}, {"exercise=1&unexpected=yes", nil, 400},
		{"exercise=1", nil, 200}, {"exercise=1", errors.New("private database details"), 500},
	} {
		store := &fakeStore{err: tc.err}
		mux := http.NewServeMux()
		Register(mux, store)
		response := httptest.NewRecorder()
		mux.ServeHTTP(response, httptest.NewRequest("GET", "/api/history/progress?"+tc.query, nil))
		if response.Code != tc.status || strings.Contains(response.Body.String(), "private database details") {
			t.Fatal(tc, response.Code, response.Body.String())
		}
		if tc.status == 200 && (store.owner != exercises.LocalOwnerID || response.Body.String() != "{\"items\":[],\"metric\":\"average_estimated_strength\"}\n") {
			t.Fatal(store.owner, response.Body.String())
		}
	}
}

func TestProgressSelectionFiltersAndCapturedIdentity(t *testing.T) {
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
	if _, err = pool.Exec(ctx, "INSERT INTO owners(id,label) VALUES(251,'Progress test'),(252,'Other progress test')"); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_, err := pool.Exec(context.Background(), `DELETE FROM workout_sessions WHERE owner_id IN(251,252); DELETE FROM template_exercises WHERE owner_id IN(251,252); DELETE FROM templates WHERE owner_id IN(251,252); DELETE FROM exercises WHERE owner_id IN(251,252); DELETE FROM owners WHERE id IN(251,252)`)
		if err != nil {
			t.Error(err)
		}
	}()
	library := exercises.Postgres{Pool: pool}
	ts := templates.Postgres{Pool: pool}
	store := Postgres{Pool: pool}
	makeExercise := func(owner int64) string {
		e, err := library.Create(ctx, owner, exercises.Input{Name: "Captured press", MuscleGroups: []string{"chest"}})
		if err != nil {
			t.Fatal(err)
		}
		return e.ID
	}
	exercise, other, foreign := makeExercise(251), makeExercise(251), makeExercise(252)
	template, err := ts.Create(ctx, 251, templates.Input{Name: "Original routine", Exercises: []templates.InputEntry{{ExerciseID: exercise, SetCount: 6}, {ExerciseID: other, SetCount: 2}, {ExerciseID: exercise, SetCount: 6}}})
	if err != nil {
		t.Fatal(err)
	}
	foreignTemplate, err := ts.Create(ctx, 252, templates.Input{Name: "Original routine", Exercises: []templates.InputEntry{{ExerciseID: foreign, SetCount: 1}}})
	if err != nil {
		t.Fatal(err)
	}
	uuid := func(i int) string { return fmt.Sprintf("51000000-0000-4000-8000-%012d", i) }
	partials := int64(999)
	inputs := []SetInput{
		{Position: 1, Weight: "120", Reps: 8, Technique: "good"},
		{Position: 3, Weight: "0", Reps: 12, Technique: "decent", PartialReps: &partials},
		{Position: 1, Weight: "500", Reps: 12, Technique: "bad"},
		{Position: 3, Weight: "0", Reps: 12, Technique: "good"},
		{Position: 3, Weight: "999", Reps: 12, Technique: "good", PartialReps: &partials},
		{Position: 3, Weight: "5", Reps: 99, Technique: "good"},
		{Position: 2, Weight: "900", Reps: 1000, Technique: "good"},
	}
	makeWorkout := func(owner int64, templateID, name, date string, sets []SetInput) Session {
		s, err := store.Start(ctx, owner, templateID)
		if err != nil {
			t.Fatal(err)
		}
		for i, input := range sets {
			if _, err = store.SaveSet(ctx, owner, s.ID, uuid(i), input); err != nil {
				t.Fatal(err)
			}
		}
		if err = store.Complete(ctx, owner, s.ID); err != nil {
			t.Fatal(err)
		}
		if _, err = pool.Exec(ctx, "UPDATE workout_sessions SET name=$2,started_at=$3::timestamptz-interval '1 hour',completed_at=$3 WHERE id=$1", s.ID, name, date); err != nil {
			t.Fatal(err)
		}
		return s
	}
	first := makeWorkout(251, template.ID, "Record source", "2026-10-25T00:30:00Z", inputs)
	if err = store.DeleteSet(ctx, 251, first.ID, uuid(5)); !errors.Is(err, ErrCompleted) {
		t.Fatal("completed sets writable", err)
	}
	// Fixture a tombstone before reading; normal completed history remains immutable.
	if _, err = pool.Exec(ctx, "UPDATE session_sets SET deleted=true WHERE session_id=$1 AND id=$2", first.ID, uuid(5)); err != nil {
		t.Fatal(err)
	}
	points, err := store.ExerciseProgress(ctx, 251, HistoryFilter{Exercise: exercise})
	if err != nil || len(points) != 1 {
		t.Fatal(points, err)
	}
	if p := points[0]; p.AverageEstimate != "450.12" || p.SetCount != 5 || len(p.Sets) != 5 || p.Sets[3].SetID != uuid(3) || p.Sets[3].Weight != "0" || p.Sets[3].Technique != "good" {
		t.Fatal("incorrect average or contributing sets", p)
	}
	// Technique and partials remain visible without changing the mean.
	if _, err = pool.Exec(ctx, "UPDATE session_sets SET technique='bad',partial_reps=1 WHERE session_id=$1 AND id=$2", first.ID, uuid(4)); err != nil {
		t.Fatal(err)
	}
	afterContext, err := store.ExerciseProgress(ctx, 251, HistoryFilter{Exercise: exercise})
	if err != nil || afterContext[0].AverageEstimate != points[0].AverageEstimate {
		t.Fatal("context changed average", afterContext, err)
	}
	// Reordering changes source display order, not the aggregate.
	if _, err = pool.Exec(ctx, "UPDATE session_sets SET set_order=CASE WHEN id=$2::uuid THEN 1 ELSE set_order+1 END WHERE session_id=$1 AND exercise_position=3", first.ID, uuid(4)); err != nil {
		t.Fatal(err)
	}
	points, err = store.ExerciseProgress(ctx, 251, HistoryFilter{Exercise: exercise})
	if err != nil || points[0].Sets[2].SetID != uuid(4) || points[0].Sets[2].SetNumber != 1 || points[0].AverageEstimate != "450.12" {
		t.Fatal("source order or mean", points, err)
	}
	second := makeWorkout(251, template.ID, "Second same instant", "2026-10-25T00:30:00Z", []SetInput{{Position: 1, Weight: "20", Reps: 1, Technique: "good"}})
	third := makeWorkout(251, template.ID, "Second local 02:30", "2026-10-25T01:30:00Z", []SetInput{{Position: 1, Weight: "0", Reps: 50, Technique: "bad"}})
	makeWorkout(251, template.ID, "Before local day", "2026-10-24T21:59:59Z", []SetInput{{Position: 1, Weight: "20", Reps: 100, Technique: "good"}})
	makeWorkout(251, template.ID, "After local day", "2026-10-25T23:00:00Z", []SetInput{{Position: 1, Weight: "20", Reps: 100, Technique: "good"}})
	makeWorkout(251, template.ID, "Empty", "2026-10-25T12:00:00Z", nil)
	makeWorkout(252, foreignTemplate.ID, "Foreign", "2026-10-25T12:00:00Z", []SetInput{{Position: 1, Weight: "20", Reps: 10000, Technique: "good"}})
	cancelled, err := store.Start(ctx, 251, template.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = store.SaveSet(ctx, 251, cancelled.ID, uuid(0), SetInput{Position: 1, Weight: "10", Reps: 999, Technique: "good"}); err != nil {
		t.Fatal(err)
	}
	if err = store.End(ctx, 251, cancelled.ID); err != nil {
		t.Fatal(err)
	}
	active, err := store.Start(ctx, 251, template.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = store.SaveSet(ctx, 251, active.ID, uuid(0), SetInput{Position: 1, Weight: "10", Reps: 999, Technique: "good"}); err != nil {
		t.Fatal(err)
	}
	filter, err := ParseHistoryQuery("exercise=" + exercise + "&from=2026-10-25&to=2026-10-25&tz=Europe%2FCopenhagen")
	if err != nil {
		t.Fatal(err)
	}
	points, err = store.ExerciseProgress(ctx, 251, filter)
	if err != nil || len(points) != 3 {
		t.Fatal("scope/date eligibility", points, err)
	}
	if got := []string{points[0].SessionID, points[1].SessionID, points[2].SessionID}; !reflect.DeepEqual(got, []string{first.ID, second.ID, third.ID}) {
		t.Fatal("same-day/instant ordering", got)
	}
	if points[1].AverageEstimate != "20.67" || points[2].AverageEstimate != "0.00" {
		t.Fatal("arbitrary rep cap", points)
	}
	filter.Query = "record SOURCE"
	narrowed, err := store.ExerciseProgress(ctx, 251, filter)
	if err != nil || len(narrowed) != 1 || narrowed[0].SessionID != first.ID {
		t.Fatal("search mismatch", narrowed, err)
	}
	list, err := store.CompletedSets(ctx, 251, filter)
	if err != nil {
		t.Fatal(err)
	}
	for _, source := range narrowed[0].Sets {
		found := false
		for _, set := range list {
			if set.ID == source.SetID {
				found = true
			}
		}
		if !found {
			t.Fatal("chart contributing set hidden by Sets filters", source)
		}
	}
	filter.Query = "does not match"
	if empty, err := store.ExerciseProgress(ctx, 251, filter); err != nil || len(empty) != 0 {
		t.Fatal(empty, err)
	}
	if empty, err := store.ExerciseProgress(ctx, 251, HistoryFilter{Exercise: foreign}); err != nil || len(empty) != 0 {
		t.Fatal("foreign identity leaked", empty, err)
	}
	if empty, err := store.ExerciseProgress(ctx, 251, HistoryFilter{Exercise: "9223372036854775807"}); err != nil || len(empty) != 0 {
		t.Fatal("unknown identity", empty, err)
	}
	// Mutable library/template metadata and even deletion cannot rewrite points.
	if _, err = library.Update(ctx, 251, exercise, exercises.Input{Name: "Renamed press", MuscleGroups: []string{"chest"}}); err != nil {
		t.Fatal(err)
	}
	if _, err = ts.Update(ctx, 251, template.ID, templates.Input{Name: "Changed", Exercises: []templates.InputEntry{{ExerciseID: other, SetCount: 1}}}); err != nil {
		t.Fatal(err)
	}
	if _, err = ts.SetArchived(ctx, 251, template.ID, true); err != nil {
		t.Fatal(err)
	}
	if err = library.Delete(ctx, 251, exercise); err != nil {
		t.Fatal(err)
	}
	filter.Query = ""
	after, err := store.ExerciseProgress(ctx, 251, filter)
	if err != nil || !reflect.DeepEqual(after, points) {
		t.Fatal("snapshots changed", after, err)
	}
}

func TestAverageEstimatedStrength(t *testing.T) {
	for _, tc := range []struct {
		name string
		sets []ProgressSet
		want string
	}{
		{"three sets", []ProgressSet{{Weight: "30", Reps: "10"}, {Weight: "30", Reps: "9"}, {Weight: "30", Reps: "8"}}, "39.00"},
		{"zero counted", []ProgressSet{{Weight: "30", Reps: "10"}, {Weight: "0", Reps: "999"}}, "20.00"},
		{"one saved set", []ProgressSet{{Weight: "30", Reps: "1"}}, "31.00"},
		{"round after averaging", []ProgressSet{{Weight: "0.01", Reps: "16"}, {Weight: "0.02", Reps: "8"}}, "0.02"},
		{"exact large load", []ProgressSet{{Weight: "100000000000000000000.00", Reps: "30"}}, "200000000000000000000.00"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := averageEstimates(tc.sets)
			if err != nil || got != tc.want {
				t.Fatal(got, tc.want, err)
			}
		})
	}
}
