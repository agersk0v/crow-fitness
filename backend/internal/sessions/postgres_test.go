package sessions

import (
	"context"
	"errors"
	"os"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/agersk0v/crow-fitness/backend/internal/exercises"
	"github.com/agersk0v/crow-fitness/backend/internal/migrations"
	"github.com/agersk0v/crow-fitness/backend/internal/templates"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestPostgresLifecycle(t *testing.T) {
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
	if _, err = pool.Exec(ctx, "INSERT INTO owners(id,label) VALUES (61,'Session test'),(62,'Other session test')"); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_, err := pool.Exec(context.Background(), `DELETE FROM active_sessions WHERE owner_id IN (61,62);
   DELETE FROM template_exercises WHERE owner_id IN (61,62); DELETE FROM templates WHERE owner_id IN (61,62);
   DELETE FROM exercises WHERE owner_id IN (61,62); DELETE FROM owners WHERE id IN (61,62)`)
		if err != nil {
			t.Error(err)
		}
	}()
	library := exercises.Postgres{Pool: pool}
	templateStore := templates.Postgres{Pool: pool}
	store := Postgres{Pool: pool}
	first, err := library.Create(ctx, 61, exercises.Input{Name: "Press", MuscleGroups: []string{"chest"}})
	if err != nil {
		t.Fatal(err)
	}
	second, err := library.Create(ctx, 61, exercises.Input{Name: "Squat", MuscleGroups: []string{"quadriceps"}})
	if err != nil {
		t.Fatal(err)
	}
	min, max := int64(8), int64(12)
	rir, zero := templates.Decimal("1.1234567890123456789"), templates.Decimal("0")
	template, err := templateStore.Create(ctx, 61, templates.Input{Name: "Routine", Exercises: []templates.InputEntry{
		{ExerciseID: first.ID, SetCount: 3, RepMin: &min, RepMax: &max, TargetRIR: &rir},
		{ExerciseID: second.ID, SetCount: 2, RepMin: &min, RepMax: &min, TargetRIR: &zero},
		{ExerciseID: first.ID, SetCount: 1},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if active, err := store.Active(ctx, 61); err != nil || active != nil {
		t.Fatal(active, err)
	}
	if _, err = store.Start(ctx, 62, template.ID); !errors.Is(err, ErrNotFound) {
		t.Fatal("foreign template accepted", err)
	}
	session, err := store.Start(ctx, 61, template.ID)
	if err != nil {
		t.Fatal(err)
	}
	if session.Name != template.Name || session.TemplateID != template.ID || session.ID == "" || session.StartedAt.IsZero() || !reflect.DeepEqual(session.Exercises, template.Exercises) {
		t.Fatalf("incorrect snapshot %+v", session)
	}
	if _, err = store.Get(ctx, 62, session.ID); !errors.Is(err, ErrNotFound) {
		t.Fatal("foreign session visible", err)
	}
	if active, err := store.Active(ctx, 62); err != nil || active != nil {
		t.Fatal("foreign active visible", active, err)
	}
	if err = store.End(ctx, 62, session.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = store.Start(ctx, 61, template.ID); err == nil {
		t.Fatal("second session accepted")
	} else {
		var conflict ActiveConflict
		if !errors.As(err, &conflict) || !reflect.DeepEqual(conflict.Session, session) {
			t.Fatal("existing session replaced", err)
		}
	}
	// The unique owner constraint protects the rule even outside the Go transaction.
	if _, err = pool.Exec(ctx, "INSERT INTO active_sessions(owner_id,template_id,name) VALUES (61,$1,'Duplicate')", template.ID); err == nil {
		t.Fatal("database allowed two active sessions")
	}
	changed := templates.Input{Name: "Changed", Exercises: []templates.InputEntry{{ExerciseID: second.ID, SetCount: 5}}}
	if _, err = templateStore.Update(ctx, 61, template.ID, changed); err != nil {
		t.Fatal(err)
	}
	if _, err = library.Update(ctx, 61, second.ID, exercises.Input{Name: "Renamed", MuscleGroups: []string{"quadriceps"}}); err != nil {
		t.Fatal(err)
	}
	if _, err = templateStore.SetArchived(ctx, 61, template.ID, true); err != nil {
		t.Fatal(err)
	}
	if err = library.Delete(ctx, 61, first.ID); err != nil {
		t.Fatal("unused exercise could not be removed", err)
	}
	if err = migrations.Apply(ctx, pool); err != nil {
		t.Fatal(err)
	}
	fresh, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer fresh.Close()
	reopened, err := (Postgres{Pool: fresh}).Get(ctx, 61, session.ID)
	if err != nil || !reflect.DeepEqual(session, reopened) {
		t.Fatal("snapshot changed after edits/archive/delete/migrations/reconnect", reopened, err)
	}
	if err = store.End(ctx, 61, session.ID); err != nil {
		t.Fatal(err)
	}
	if err = store.End(ctx, 61, session.ID); err != nil {
		t.Fatal("end retry failed", err)
	}
	if _, err = store.Get(ctx, 61, session.ID); !errors.Is(err, ErrNotFound) {
		t.Fatal("ended session retained", err)
	}
	if active, err := store.Active(ctx, 61); err != nil || active != nil {
		t.Fatal("ending retained active", active, err)
	}
	var count int
	if err = pool.QueryRow(ctx, "SELECT count(*) FROM active_session_exercises WHERE session_id=$1", session.ID).Scan(&count); err != nil || count != 0 {
		t.Fatal("ending retained snapshot rows", count, err)
	}
	if _, err = store.Start(ctx, 61, template.ID); !errors.Is(err, ErrArchived) {
		t.Fatal("archived start accepted", err)
	}
	if _, err = store.Start(ctx, 61, "9223372036854775807"); !errors.Is(err, ErrNotFound) {
		t.Fatal("missing start accepted", err)
	}
	// Empty source failure occurs after inserting a session; it must roll back that insert.
	var emptyID string
	if err = pool.QueryRow(ctx, "INSERT INTO templates(owner_id,name) VALUES (61,'Empty') RETURNING id::text").Scan(&emptyID); err != nil {
		t.Fatal(err)
	}
	if _, err = store.Start(ctx, 61, emptyID); !errors.Is(err, ErrEmpty) {
		t.Fatal("empty start accepted", err)
	}
	if active, err := store.Active(ctx, 61); err != nil || active != nil {
		t.Fatal("failed start left partial session", active, err)
	}
	if _, err = templateStore.SetArchived(ctx, 61, template.ID, false); err != nil {
		t.Fatal(err)
	}
	// Simultaneous requests all resolve to one complete snapshot.
	type result struct {
		item Session
		err  error
	}
	results := make(chan result, 12)
	gate := make(chan struct{})
	var wg sync.WaitGroup
	for range 12 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-gate
			item, err := store.Start(ctx, 61, template.ID)
			results <- result{item, err}
		}()
	}
	close(gate)
	wg.Wait()
	close(results)
	winnerID := ""
	created := 0
	for result := range results {
		item := result.item
		if result.err == nil {
			created++
		} else {
			var conflict ActiveConflict
			if !errors.As(result.err, &conflict) {
				t.Fatal(result.err)
			}
			item = conflict.Session
		}
		if winnerID == "" {
			winnerID = item.ID
		}
		if item.ID != winnerID || item.Name != "Changed" || len(item.Exercises) != 1 || item.Exercises[0].Name != "Renamed" || item.Exercises[0].SetCount != 5 {
			t.Fatal("concurrent start diverged", item)
		}
	}
	if created != 1 {
		t.Fatal("incorrect created count", created)
	}
	if err = store.End(ctx, 61, session.ID); err != nil {
		t.Fatal(err)
	}
	if active, err := store.Active(ctx, 61); err != nil || active == nil || active.ID != winnerID {
		t.Fatal("stale ending cleared newer session", active, err)
	}
	// Starting waits for a complete template edit, while owner foreign-key checks
	// used by duplication remain compatible with the session's owner lock.
	if err = store.End(ctx, 61, winnerID); err != nil {
		t.Fatal(err)
	}
	edit, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer edit.Rollback(context.Background())
	if _, err = edit.Exec(ctx, "UPDATE templates SET name='Committed edit' WHERE owner_id=61 AND id=$1", template.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = edit.Exec(ctx, "UPDATE template_exercises SET set_count=6 WHERE owner_id=61 AND template_id=$1", template.ID); err != nil {
		t.Fatal(err)
	}
	started := make(chan result, 1)
	go func() { item, err := store.Start(ctx, 61, template.ID); started <- result{item, err} }()
	deadline := time.NewTimer(3 * time.Second)
	defer deadline.Stop()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		var blocked bool
		if err = pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE wait_event_type='Lock' AND query LIKE 'SELECT name,archived FROM templates%')`).Scan(&blocked); err != nil {
			t.Fatal(err)
		}
		if blocked {
			break
		}
		select {
		case early := <-started:
			t.Fatal("start did not wait for template edit", early)
		case <-deadline.C:
			t.Fatal("start did not reach template lock")
		case <-ticker.C:
		}
	}
	// FOR NO KEY UPDATE on the owner must allow this foreign-key lookup.
	if _, err = edit.Exec(ctx, "INSERT INTO templates(owner_id,name) VALUES (61,'Concurrent copy')"); err != nil {
		t.Fatal(err)
	}
	if err = edit.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	resultAfterEdit := <-started
	if resultAfterEdit.err != nil || resultAfterEdit.item.Name != "Committed edit" || resultAfterEdit.item.Exercises[0].SetCount != 6 {
		t.Fatal("start captured partial edit", resultAfterEdit)
	}

}
