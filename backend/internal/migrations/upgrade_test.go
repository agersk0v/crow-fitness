package migrations

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestSessionUpgradePreservesActiveWorkout(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL required for disposable PostgreSQL test")
	}
	ctx := context.Background()
	admin, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	// Own a separate schema to exercise an actual 005 -> 007 upgrade.
	if _, err = admin.Exec(ctx, "CREATE SCHEMA issue33_upgrade"); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if _, err := admin.Exec(ctx, "DROP SCHEMA issue33_upgrade CASCADE"); err != nil {
			t.Error(err)
		}
	}()
	config, err := pgxpool.ParseConfig(url)
	if err != nil {
		t.Fatal(err)
	}
	config.ConnConfig.RuntimeParams["search_path"] = "issue33_upgrade"
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if _, err = pool.Exec(ctx, "CREATE TABLE schema_migrations(version text PRIMARY KEY,applied_at timestamptz NOT NULL DEFAULT now())"); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"001_exercises.sql", "002_templates.sql", "003_exercise_tags.sql", "004_template_archive.sql", "005_active_sessions.sql"} {
		sql, err := files.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = pool.Exec(ctx, string(sql)); err != nil {
			t.Fatal(err)
		}
		if _, err = pool.Exec(ctx, "INSERT INTO schema_migrations(version) VALUES($1)", name); err != nil {
			t.Fatal(err)
		}
	}
	if _, err = pool.Exec(ctx, `INSERT INTO exercises(owner_id,name,muscle_groups) VALUES(1,'Legacy press',ARRAY['chest']);
 INSERT INTO templates(owner_id,name) VALUES(1,'Legacy routine');
 INSERT INTO template_exercises(template_id,owner_id,position,exercise_id,set_count) VALUES(1,1,1,1,3);
 INSERT INTO active_sessions(owner_id,template_id,name,started_at) VALUES(1,1,'Saved legacy name','2026-10-13T08:00:00Z');
 INSERT INTO active_session_exercises(session_id,owner_id,position,exercise_id,name,set_count,target_rir) VALUES(1,1,1,1,'Saved legacy exercise',3,0)`); err != nil {
		t.Fatal(err)
	}
	if err = Apply(ctx, pool); err != nil {
		t.Fatal(err)
	}
	if err = Apply(ctx, pool); err != nil {
		t.Fatal(err)
	}
	var intact bool
	if err = pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM workout_sessions s JOIN session_exercises e ON e.session_id=s.id
 WHERE s.id=1 AND s.owner_id=1 AND s.name='Saved legacy name' AND s.completed_at IS NULL AND s.started_at='2026-10-13T08:00:00Z'
 AND e.name='Saved legacy exercise' AND e.set_count=3 AND e.target_rir=0)`).Scan(&intact); err != nil || !intact {
		t.Fatal("upgrade rewrote active snapshot", err)
	}
}
