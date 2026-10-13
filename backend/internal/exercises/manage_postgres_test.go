package exercises_test

import (
	"context"
	"errors"
	"os"
	"reflect"
	"testing"
	"time"

	ex "github.com/agersk0v/crow-fitness/backend/internal/exercises"
	"github.com/agersk0v/crow-fitness/backend/internal/migrations"
	"github.com/agersk0v/crow-fitness/backend/internal/templates"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestManagementPersistence(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL required for disposable PostgreSQL tests")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if err = migrations.Apply(ctx, pool); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, "INSERT INTO owners(id,label) VALUES(27,'Management test'),(28,'Foreign owner')"); err != nil {
		t.Fatal(err)
	}
	defer pool.Exec(context.Background(), "DELETE FROM template_exercises WHERE owner_id IN(27,28); DELETE FROM templates WHERE owner_id IN(27,28); DELETE FROM exercises WHERE owner_id IN(27,28); DELETE FROM owners WHERE id IN(27,28)")
	store := ex.Postgres{Pool: pool}
	item, err := store.Create(ctx, 27, ex.Input{Name: "Press", MuscleGroups: []string{"shoulders", "chest", "chest"}})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(item.MuscleGroups, []string{"chest", "shoulders"}) {
		t.Fatal(item)
	}
	templateStore := templates.Postgres{Pool: pool}
	template, err := templateStore.Create(ctx, 27, templates.Input{Name: "Push", Exercises: []templates.InputEntry{{ExerciseID: item.ID, SetCount: 3}, {ExerciseID: item.ID, SetCount: 1}}})
	if err != nil {
		t.Fatal(err)
	}
	updated, err := store.Update(ctx, 27, item.ID, ex.Input{Name: " Renamed press ", MuscleGroups: []string{"back"}})
	if err != nil || updated.ID != item.ID || updated.Name != "Renamed press" {
		t.Fatalf("%+v %v", updated, err)
	}
	read, err := templateStore.Get(ctx, 27, template.ID)
	if err != nil || read.Exercises[0].Name != updated.Name {
		t.Fatal(read, err)
	}
	usage, err := store.Usage(ctx, 27, item.ID)
	if err != nil || len(usage) != 1 || usage[0].ID != template.ID {
		t.Fatal(usage, err)
	}
	err = store.Delete(ctx, 27, item.ID)
	var used *ex.InUseError
	if !errors.As(err, &used) || len(used.Templates) != 1 {
		t.Fatal(err)
	}
	for _, owner := range []int64{28} {
		if _, err = store.Update(ctx, owner, item.ID, ex.Input{Name: "Foreign", MuscleGroups: []string{"chest"}}); !errors.Is(err, ex.ErrNotFound) {
			t.Fatal(err)
		}
		if _, err = store.Usage(ctx, owner, item.ID); !errors.Is(err, ex.ErrNotFound) {
			t.Fatal(err)
		}
		if err = store.Delete(ctx, owner, item.ID); !errors.Is(err, ex.ErrNotFound) {
			t.Fatal(err)
		}
	}
	if _, err = store.Update(ctx, 27, item.ID, ex.Input{Name: "Press", MuscleGroups: []string{"unknown"}}); err == nil {
		t.Fatal("invalid tags saved")
	}
	if _, err = pool.Exec(ctx, "UPDATE exercises SET muscle_groups=ARRAY['unknown'] WHERE id=$1", item.ID); err == nil {
		t.Fatal("database accepted unknown tags")
	}
	free, err := store.Create(ctx, 27, ex.Input{Name: "Free", MuscleGroups: []string{"chest"}})
	if err != nil {
		t.Fatal(err)
	}
	if err = store.Delete(ctx, 27, free.ID); err != nil {
		t.Fatal(err)
	}
	if err = migrations.Apply(ctx, pool); err != nil {
		t.Fatal(err)
	}
	items, err := store.List(ctx, 27)
	if err != nil || len(items) != 1 || !reflect.DeepEqual(items[0], updated) {
		t.Fatal(items, err)
	}
	// Template creation holds a KEY SHARE lock until it commits. A real deletion
	// request must wait, then see the new reference and refuse deletion.
	concurrent, err := store.Create(ctx, 27, ex.Input{Name: "Concurrent", MuscleGroups: []string{"chest"}})
	if err != nil {
		t.Fatal(err)
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(context.Background())
	if _, err = tx.Exec(ctx, "SELECT id FROM exercises WHERE id=$1 FOR KEY SHARE", concurrent.ID); err != nil {
		t.Fatal(err)
	}
	var templateID string
	if err = tx.QueryRow(ctx, "INSERT INTO templates(owner_id,name) VALUES(27,'Concurrent template') RETURNING id::text").Scan(&templateID); err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(ctx, "INSERT INTO template_exercises(template_id,owner_id,position,exercise_id,set_count) VALUES($1,27,1,$2,1)", templateID, concurrent.ID); err != nil {
		t.Fatal(err)
	}
	result := make(chan error, 1)
	go func() { result <- store.Delete(ctx, 27, concurrent.ID) }()
	waitForBlockedQuery(t, ctx, pool)
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if err = <-result; !errors.As(err, &used) {
		t.Fatalf("concurrent deletion broke reference: %v", err)
	}
	// If deletion holds the lock first, actual template creation waits and fails
	// validation after deletion commits, leaving no partially saved template.
	removed, err := store.Create(ctx, 27, ex.Input{Name: "Deleted first", MuscleGroups: []string{"chest"}})
	if err != nil {
		t.Fatal(err)
	}
	tx2, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx2.Rollback(context.Background())
	if _, err = tx2.Exec(ctx, "DELETE FROM exercises WHERE owner_id=27 AND id=$1", removed.ID); err != nil {
		t.Fatal(err)
	}
	go func() {
		_, err := templateStore.Create(ctx, 27, templates.Input{Name: "Must not persist", Exercises: []templates.InputEntry{{ExerciseID: removed.ID, SetCount: 1}}})
		result <- err
	}()
	waitForBlockedQuery(t, ctx, pool)
	if err = tx2.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	var invalid templates.ValidationError
	if err = <-result; !errors.As(err, &invalid) {
		t.Fatalf("concurrent create did not reject deleted exercise: %v", err)
	}
	summaries, err := templateStore.List(ctx, 27)
	if err != nil || len(summaries) != 2 {
		t.Fatal(summaries, err)
	}
}
func waitForBlockedQuery(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()
	deadline := time.After(3 * time.Second)
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
		case <-deadline:
			t.Fatal("concurrent request never waited on lock")
		case <-ticker.C:
		}
	}
}

func TestTagMigrationPreservesExistingExercises(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL required for disposable PostgreSQL tests")
	}
	ctx := context.Background()
	admin, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	// This schema belongs only to this test inside the disposable test database.
	if _, err = admin.Exec(ctx, "CREATE SCHEMA exercise_tag_upgrade_test"); err != nil {
		t.Fatal(err)
	}
	defer admin.Exec(ctx, "DROP SCHEMA exercise_tag_upgrade_test CASCADE")
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		t.Fatal(err)
	}
	cfg.ConnConfig.RuntimeParams["search_path"] = "exercise_tag_upgrade_test"
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if _, err = pool.Exec(ctx, "CREATE TABLE schema_migrations(version text PRIMARY KEY, applied_at timestamptz NOT NULL DEFAULT now())"); err != nil {
		t.Fatal(err)
	}
	for _, file := range []string{"001_exercises.sql", "002_templates.sql"} {
		sql, err := os.ReadFile("../migrations/" + file)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = pool.Exec(ctx, string(sql)); err != nil {
			t.Fatal(err)
		}
		if _, err = pool.Exec(ctx, "INSERT INTO schema_migrations(version) VALUES($1)", file); err != nil {
			t.Fatal(err)
		}
	}
	var id string
	if err = pool.QueryRow(ctx, "INSERT INTO exercises(owner_id,name) VALUES(1,'Existing squat') RETURNING id::text").Scan(&id); err != nil {
		t.Fatal(err)
	}
	if err = migrations.Apply(ctx, pool); err != nil {
		t.Fatal(err)
	}
	if err = migrations.Apply(ctx, pool); err != nil {
		t.Fatal(err)
	}
	store := ex.Postgres{Pool: pool}
	items, err := (ex.Postgres{Pool: pool}).List(ctx, 1)
	if err != nil || len(items) != 1 || items[0].ID != id || items[0].Name != "Existing squat" || len(items[0].MuscleGroups) != 0 {
		t.Fatal(items, err)
	}

	if _, err = store.Update(ctx, 1, id, ex.Input{Name: "Renamed without tags"}); err == nil {
		t.Fatal("untagged legacy edit accepted")
	}
	items, err = store.List(ctx, 1)
	if err != nil || len(items) != 1 || items[0].Name != "Existing squat" || len(items[0].MuscleGroups) != 0 {
		t.Fatal("failed edit changed legacy record", items, err)
	}
	updated, err := store.Update(ctx, 1, id, ex.Input{Name: "Updated squat", MuscleGroups: []string{"quadriceps"}})
	if err != nil || updated.ID != id || updated.Name != "Updated squat" || !reflect.DeepEqual(updated.MuscleGroups, []string{"quadriceps"}) {
		t.Fatal(updated, err)
	}
}
