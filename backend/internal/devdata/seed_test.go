package devdata

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
	"github.com/agersk0v/crow-fitness/backend/internal/sessions"
	"github.com/agersk0v/crow-fitness/backend/internal/templates"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type fakeStore struct {
	calls int
	err   error
}

func (f *fakeStore) Reset(context.Context, time.Time) (Result, error) {
	f.calls++
	return Result{CompletedSessions: 48, CompletedSets: 1032}, f.err
}
func TestResetGateAndConfirmation(t *testing.T) {
	for _, tc := range []struct {
		enabled, confirm bool
		err              error
		status, calls    int
	}{{false, true, nil, 404, 0}, {true, false, nil, 400, 0}, {true, true, nil, 200, 1}, {true, true, errors.New("private details"), 500, 1}} {
		mux := http.NewServeMux()
		store := &fakeStore{err: tc.err}
		Register(mux, store, tc.enabled)
		request := httptest.NewRequest("POST", "/api/dev/reset", nil)
		if tc.confirm {
			request.Header.Set("X-Confirm-Reset", "replace-workout-data")
		}
		response := httptest.NewRecorder()
		mux.ServeHTTP(response, request)
		if response.Code != tc.status || store.calls != tc.calls || strings.Contains(response.Body.String(), "private details") {
			t.Fatal(tc, response.Code, response.Body.String(), store.calls)
		}
	}
}

func TestPostgresResetReplacementRollbackAndWrites(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL required for disposable PostgreSQL test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	root, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	// Reset affects local owner 1, so isolate it from other packages' fixtures.
	schema := fmt.Sprintf("devdata_test_%d", time.Now().UnixNano())
	quoted := pgx.Identifier{schema}.Sanitize()
	if _, err = root.Exec(ctx, "CREATE SCHEMA "+quoted); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if _, err := root.Exec(context.Background(), "DROP SCHEMA "+quoted+" CASCADE"); err != nil {
			t.Error(err)
		}
	}()
	config, err := pgxpool.ParseConfig(url)
	if err != nil {
		t.Fatal(err)
	}
	config.ConnConfig.RuntimeParams["search_path"] = schema
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if err = migrations.Apply(ctx, pool); err != nil {
		t.Fatal(err)
	}
	store := Postgres{Pool: pool}
	library := exercises.Postgres{Pool: pool}
	workouts := sessions.Postgres{Pool: pool}
	ts := templates.Postgres{Pool: pool}
	old, err := library.Create(ctx, 1, exercises.Input{Name: "Old data", MuscleGroups: []string{"chest"}})
	if err != nil {
		t.Fatal(err)
	}
	template, err := ts.Create(ctx, 1, templates.Input{Name: "Old routine", Exercises: []templates.InputEntry{{ExerciseID: old.ID, SetCount: 1}}})
	if err != nil {
		t.Fatal(err)
	}
	session, err := workouts.Start(ctx, 1, template.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = workouts.SaveSet(ctx, 1, session.ID, "11111111-1111-4111-8111-111111111111", sessions.SetInput{Position: 1, Weight: "10", Reps: 8}); err != nil {
		t.Fatal(err)
	}
	if err = workouts.Complete(ctx, 1, session.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = workouts.Start(ctx, 1, template.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, "INSERT INTO owners(id,label) VALUES(2,'Foreign test');INSERT INTO exercises(owner_id,name,muscle_groups) VALUES(2,'Foreign exercise',ARRAY['core'])"); err != nil {
		t.Fatal(err)
	}
	var versions int
	if err = pool.QueryRow(ctx, "SELECT count(*) FROM schema_migrations").Scan(&versions); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2030, 1, 1, 9, 0, 0, 0, time.UTC)
	check := func(result Result) {
		t.Helper()
		if result.Exercises != 26 || result.Templates != 4 || result.CompletedSessions != 48 || result.CompletedSets != 1032 || result.ActiveSets != 3 || result.ActiveSessionID == "" {
			t.Fatal("wrong seed counts", result)
		}
		var exercisesCount, templateCount, completed, active, completedSets, activeSets int
		err := pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM exercises WHERE owner_id=1),(SELECT count(*) FROM templates WHERE owner_id=1),(SELECT count(*) FROM workout_sessions WHERE owner_id=1 AND completed_at IS NOT NULL),(SELECT count(*) FROM workout_sessions WHERE owner_id=1 AND completed_at IS NULL),(SELECT count(*) FROM session_sets ss JOIN workout_sessions s ON s.id=ss.session_id WHERE ss.owner_id=1 AND s.completed_at IS NOT NULL),(SELECT count(*) FROM session_sets ss JOIN workout_sessions s ON s.id=ss.session_id WHERE ss.owner_id=1 AND s.completed_at IS NULL)`).Scan(&exercisesCount, &templateCount, &completed, &active, &completedSets, &activeSets)
		if err != nil || exercisesCount != 26 || templateCount != 4 || completed != 48 || active != 1 || completedSets != 1032 || activeSets != 3 {
			t.Fatal("stored seed counts", exercisesCount, templateCount, completed, active, completedSets, activeSets, err)
		}
		var invalid int
		if err = pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM exercises WHERE owner_id=1 AND cardinality(muscle_groups)=0)+(SELECT count(*) FROM template_exercises WHERE owner_id=1 AND (rep_min IS NOT NULL OR rep_max IS NOT NULL OR target_rir IS NOT NULL))+(SELECT count(*) FROM workout_sessions WHERE owner_id=1 AND completed_at>$1)`, now).Scan(&invalid); err != nil || invalid != 0 {
			t.Fatal("invalid tags, targets or dates", invalid, err)
		}
		var preserved, foreign int
		if err = pool.QueryRow(ctx, "SELECT (SELECT count(*) FROM schema_migrations),(SELECT count(*) FROM exercises WHERE owner_id=2 AND name='Foreign exercise')").Scan(&preserved, &foreign); err != nil || preserved != versions || foreign != 1 {
			t.Fatal("metadata/foreign data changed", preserved, foreign, err)
		}
		for name, want := range map[string]int{"Upper 1": 21, "Lower 1": 22, "Upper 2": 21, "Lower 2": 22} {
			var count int
			if err = pool.QueryRow(ctx, "SELECT sum(e.set_count) FROM template_exercises e JOIN templates t ON t.id=e.template_id WHERE t.owner_id=1 AND t.name=$1", name).Scan(&count); err != nil || count != want {
				t.Fatal("routine targets", name, count, err)
			}
		}
	}
	first, err := store.Reset(ctx, now)
	if err != nil {
		t.Fatal(err)
	}
	check(first)
	if _, err = workouts.Get(ctx, 1, session.ID); !errors.Is(err, sessions.ErrNotFound) {
		t.Fatal("old session survives", err)
	}
	history, err := workouts.CompletedSets(ctx, 1, sessions.HistoryFilter{})
	if err != nil || len(history) != 1032 {
		t.Fatal("history not readable", len(history), err)
	}
	hasZero, hasDecimals := false, false
	for _, set := range history {
		if set.Weight == "0.00" {
			hasZero = true
		}
		if strings.HasSuffix(set.Weight, ".25") {
			hasDecimals = true
		}
	}
	if !hasZero || !hasDecimals {
		t.Fatal("weight edge values missing")
	}
	active, err := workouts.Get(ctx, 1, first.ActiveSessionID)
	if err != nil || len(active.Sets) != 3 || len(active.Exercises) != 7 {
		t.Fatal("active seed unusable", active, err)
	}
	previous, err := workouts.Previous(ctx, 1, first.ActiveSessionID)
	if err != nil || len(previous) != 7 {
		t.Fatal("previous performance missing", previous, err)
	}
	second, err := store.Reset(ctx, now)
	if err != nil {
		t.Fatal(err)
	}
	check(second)
	if second.ActiveSessionID == first.ActiveSessionID {
		t.Fatal("stale session ID reused")
	}
	before, err := workouts.Get(ctx, 1, second.ActiveSessionID)
	if err != nil {
		t.Fatal(err)
	}
	// Fail late, after deleting old data and inserting most replacement records.
	if _, err = pool.Exec(ctx, `CREATE FUNCTION fail_seed() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF NEW.name='Lower 2' THEN RAISE EXCEPTION 'injected seed failure'; END IF; RETURN NEW; END $$; CREATE TRIGGER fail_seed BEFORE INSERT ON workout_sessions FOR EACH ROW EXECUTE FUNCTION fail_seed()`); err != nil {
		t.Fatal(err)
	}
	if _, err = store.Reset(ctx, now); err == nil {
		t.Fatal("injected seed failure did not fail")
	}
	after, err := workouts.Get(ctx, 1, second.ActiveSessionID)
	if err != nil || !reflect.DeepEqual(before, after) {
		t.Fatal("failed reset changed active data", after, err)
	}
	check(second)
	if _, err = pool.Exec(ctx, "DROP TRIGGER fail_seed ON workout_sessions;DROP FUNCTION fail_seed()"); err != nil {
		t.Fatal(err)
	}
	// A concurrent writer must finish before reset acquires its table locks.
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, "INSERT INTO exercises(owner_id,name,muscle_groups) VALUES(1,'Concurrent exercise',ARRAY['core'])"); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { _, err := store.Reset(ctx, now); done <- err }()
	select {
	case err := <-done:
		t.Fatal("reset bypassed in-flight write", err)
	case <-time.After(50 * time.Millisecond):
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if err = <-done; err != nil {
		t.Fatal(err)
	}
	var stale int
	if err = pool.QueryRow(ctx, "SELECT count(*) FROM exercises WHERE owner_id=1 AND name='Concurrent exercise'").Scan(&stale); err != nil || stale != 0 {
		t.Fatal("reset/write ordering", stale, err)
	}
}
