package templates

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
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestManagementPostgres(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL required for disposable PostgreSQL tests")
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
	if _, err = pool.Exec(ctx, "INSERT INTO owners(id,label) VALUES(29,'Template management'),(30,'Foreign owner')"); err != nil {
		t.Fatal(err)
	}
	defer pool.Exec(context.Background(), "DELETE FROM template_exercises WHERE owner_id IN(29,30);DELETE FROM templates WHERE owner_id IN(29,30);DELETE FROM exercises WHERE owner_id IN(29,30);DELETE FROM owners WHERE id IN(29,30)")
	library := exercises.Postgres{Pool: pool}
	first, err := library.Create(ctx, 29, exercises.Input{Name: "Press", MuscleGroups: []string{"chest"}})
	if err != nil {
		t.Fatal(err)
	}
	second, err := library.Create(ctx, 29, exercises.Input{Name: "Squat", MuscleGroups: []string{"quadriceps"}})
	if err != nil {
		t.Fatal(err)
	}
	foreign, err := library.Create(ctx, 30, exercises.Input{Name: "Foreign", MuscleGroups: []string{"back"}})
	if err != nil {
		t.Fatal(err)
	}
	store := Postgres{Pool: pool}
	initial := Input{Name: "Original", Exercises: []InputEntry{{ExerciseID: first.ID, SetCount: 3, RepMin: integer(8), RepMax: integer(12), TargetRIR: decimal("1.234567890123456789")}, {ExerciseID: second.ID, SetCount: 2, TargetRIR: decimal("0")}, {ExerciseID: first.ID, SetCount: 1}}}
	original, err := store.Create(ctx, 29, initial)
	if err != nil {
		t.Fatal(err)
	}
	copy, err := store.Duplicate(ctx, 29, original.ID, " Copy ")
	if err != nil || copy.ID == original.ID || copy.Name != "Copy" || copy.Archived || !reflect.DeepEqual(original.Exercises, copy.Exercises) {
		t.Fatal(copy, err)
	}
	updatedInput := Input{Name: " Updated ", Exercises: []InputEntry{{ExerciseID: second.ID, SetCount: 4, RepMin: integer(10), RepMax: integer(10)}, {ExerciseID: first.ID, SetCount: 2, TargetRIR: decimal("0")}}}
	updated, err := store.Update(ctx, 29, original.ID, updatedInput)
	if err != nil || updated.ID != original.ID || updated.Name != "Updated" || len(updated.Exercises) != 2 || updated.Exercises[0].ExerciseID != second.ID {
		t.Fatal(updated, err)
	}
	unchanged, err := store.Get(ctx, 29, copy.ID)
	if err != nil || !reflect.DeepEqual(copy, unchanged) {
		t.Fatal("copy changed with source", unchanged, err)
	}
	// Roll back failures after replacing the name and removing the old entries.
	bad := Input{Name: "Must rollback", Exercises: []InputEntry{{ExerciseID: first.ID, SetCount: 1}, {ExerciseID: second.ID, SetCount: 1, TargetRIR: decimal("1e200000")}}}
	if _, err = store.Update(ctx, 29, original.ID, bad); err == nil {
		t.Fatal("expected storage failure")
	}
	read, err := store.Get(ctx, 29, original.ID)
	if err != nil || !reflect.DeepEqual(updated, read) {
		t.Fatal("partial update", read, err)
	}
	bad.Exercises[1] = InputEntry{ExerciseID: foreign.ID, SetCount: 1}
	if _, err = store.Update(ctx, 29, original.ID, bad); err == nil {
		t.Fatal("foreign exercise accepted")
	}
	for _, operation := range []func() error{
		func() error { _, err := store.Update(ctx, 30, original.ID, updatedInput); return err },
		func() error { _, err := store.Duplicate(ctx, 30, original.ID, "Foreign copy"); return err },
		func() error { _, err := store.SetArchived(ctx, 30, original.ID, true); return err },
	} {
		if err = operation(); !errors.Is(err, ErrNotFound) {
			t.Fatal("foreign template accessible", err)
		}
	}
	for i := 0; i < 2; i++ {
		archived, err := store.SetArchived(ctx, 29, original.ID, true)
		if err != nil || !archived.Archived || !reflect.DeepEqual(updated.Exercises, archived.Exercises) {
			t.Fatal(archived, err)
		}
	}
	active, err := store.List(ctx, 29, false)
	if err != nil || len(active) != 1 || active[0].ID != copy.ID {
		t.Fatal(active, err)
	}
	archived, err := store.List(ctx, 29, true)
	if err != nil || len(archived) != 1 || archived[0].ID != original.ID {
		t.Fatal(archived, err)
	}
	if _, err = store.Update(ctx, 29, original.ID, updatedInput); !errors.Is(err, ErrArchived) {
		t.Fatal("archived edit accepted", err)
	}
	if _, err = store.Duplicate(ctx, 29, original.ID, "Archived copy"); !errors.Is(err, ErrArchived) {
		t.Fatal("archived duplication accepted", err)
	}
	usage, err := library.Usage(ctx, 29, first.ID)
	if err != nil || len(usage) != 2 {
		t.Fatal(usage, err)
	}
	found := false
	for _, item := range usage {
		if item.ID == original.ID && item.Archived {
			found = true
		}
	}
	if !found {
		t.Fatal("archive status missing from usage", usage)
	}
	var used *exercises.InUseError
	if err = library.Delete(ctx, 29, first.ID); !errors.As(err, &used) {
		t.Fatal("archived references lost", err)
	}
	if err = migrations.Apply(ctx, pool); err != nil {
		t.Fatal(err)
	}
	read, err = store.Get(ctx, 29, original.ID)
	if err != nil || !read.Archived {
		t.Fatal("archive lost on migration rerun", read, err)
	}
	for i := 0; i < 2; i++ {
		restored, err := store.SetArchived(ctx, 29, original.ID, false)
		if err != nil || restored.Archived || restored.ID != original.ID || !reflect.DeepEqual(updated.Exercises, restored.Exercises) {
			t.Fatal(restored, err)
		}
	}
	// An in-flight archive owns the parent lock. Edit/duplicate must wait and
	// observe the committed archived state before mutating or copying.
	for _, kind := range []string{"update", "duplicate"} {
		if _, err = store.SetArchived(ctx, 29, original.ID, false); err != nil {
			t.Fatal(err)
		}
		tx, err := pool.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = tx.Exec(ctx, "UPDATE templates SET archived=true WHERE owner_id=29 AND id=$1", original.ID); err != nil {
			tx.Rollback(ctx)
			t.Fatal(err)
		}
		result := make(chan error, 1)
		go func() {
			var err error
			if kind == "update" {
				_, err = store.Update(ctx, 29, original.ID, updatedInput)
			} else {
				_, err = store.Duplicate(ctx, 29, original.ID, "Must not copy")
			}
			result <- err
		}()
		waitOnTemplateLock(t, ctx, pool)
		if err = tx.Commit(ctx); err != nil {
			t.Fatal(err)
		}
		if err = <-result; !errors.Is(err, ErrArchived) {
			t.Fatal("archive race bypassed state", kind, err)
		}
	}
	if _, err = store.SetArchived(ctx, 29, original.ID, false); err != nil {
		t.Fatal(err)
	}
	// A duplication waiting for a complete edit must copy the new name/configuration
	// snapshot, never a mixture of old and new occurrences.
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(ctx, "UPDATE templates SET name='Concurrent update' WHERE owner_id=29 AND id=$1", original.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(ctx, "UPDATE template_exercises SET set_count=9 WHERE owner_id=29 AND template_id=$1", original.ID); err != nil {
		t.Fatal(err)
	}
	copies := make(chan Template, 1)
	failures := make(chan error, 1)
	go func() {
		item, err := store.Duplicate(ctx, 29, original.ID, "Concurrent copy")
		copies <- item
		failures <- err
	}()
	waitOnTemplateLock(t, ctx, pool)
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	consistentCopy := <-copies
	if err = <-failures; err != nil {
		t.Fatal(err)
	}
	for _, entry := range consistentCopy.Exercises {
		if entry.SetCount != 9 {
			t.Fatal("mixed duplicate configuration", consistentCopy)
		}
	}
	// Pause a real detail read after its parent query. Concurrent edit commits,
	// but repeatable-read still returns the original complete configuration.
	tracer := &pauseRead{reached: make(chan struct{}), resume: make(chan struct{})}
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		t.Fatal(err)
	}
	cfg.ConnConfig.Tracer = tracer
	readPool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer readPool.Close()
	before, err := store.Get(ctx, 29, original.ID)
	if err != nil {
		t.Fatal(err)
	}
	reads := make(chan Template, 1)
	go func() {
		item, err := (Postgres{Pool: readPool}).Get(ctx, 29, original.ID)
		reads <- item
		failures <- err
	}()
	select {
	case <-tracer.reached:
	case <-ctx.Done():
		t.Fatal("detail read did not reach barrier")
	}
	if _, err = store.Update(ctx, 29, original.ID, initial); err != nil {
		close(tracer.resume)
		t.Fatal(err)
	}
	close(tracer.resume)
	snapshot := <-reads
	if err = <-failures; err != nil || !reflect.DeepEqual(before, snapshot) {
		t.Fatal("mixed detail read", snapshot, err)
	}
}

type readSQLKey struct{}
type pauseRead struct {
	reached, resume chan struct{}
	once            sync.Once
}

func (p *pauseRead) TraceQueryStart(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
	return context.WithValue(ctx, readSQLKey{}, data.SQL)
}
func (p *pauseRead) TraceQueryEnd(ctx context.Context, _ *pgx.Conn, _ pgx.TraceQueryEndData) {
	if ctx.Value(readSQLKey{}) == "SELECT id::text,name,archived,description FROM templates WHERE owner_id=$1 AND id=$2" {
		p.once.Do(func() {
			close(p.reached)
			select {
			case <-p.resume:
			case <-ctx.Done():
			}
		})
	}
}
func waitOnTemplateLock(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()
	timer := time.NewTimer(3 * time.Second)
	defer timer.Stop()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		var waiting bool
		if err := pool.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE datname=current_database() AND wait_event_type='Lock' AND pid<>pg_backend_pid())").Scan(&waiting); err != nil {
			t.Fatal(err)
		}
		if waiting {
			return
		}
		select {
		case <-timer.C:
			t.Fatal("template operation never waited for lock")
		case <-ticker.C:
		}
	}
}
