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
	if err = pool.QueryRow(ctx, `SELECT (SELECT description FROM exercises WHERE id=1)='' AND (SELECT description FROM templates WHERE id=1)='' AND EXISTS(SELECT 1 FROM workout_sessions s JOIN session_exercises e ON e.session_id=s.id
 WHERE s.id=1 AND s.owner_id=1 AND s.name='Saved legacy name' AND s.completed_at IS NULL AND s.started_at='2026-10-13T08:00:00Z'
 AND e.name='Saved legacy exercise' AND e.set_count=3 AND e.target_rir=0)`).Scan(&intact); err != nil || !intact {
		t.Fatal("upgrade rewrote active snapshot", err)
	}
}

func TestSetMetadataUpgradePreservesExistingSets(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL required for disposable PostgreSQL test")
	}
	ctx := context.Background()
	root, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	if _, err = root.Exec(ctx, "CREATE SCHEMA technique_upgrade"); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if _, err := root.Exec(ctx, "DROP SCHEMA technique_upgrade CASCADE"); err != nil {
			t.Error(err)
		}
	}()
	config, err := pgxpool.ParseConfig(url)
	if err != nil {
		t.Fatal(err)
	}
	config.ConnConfig.RuntimeParams["search_path"] = "technique_upgrade"
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if _, err = pool.Exec(ctx, "CREATE TABLE schema_migrations(version text PRIMARY KEY)"); err != nil {
		t.Fatal(err)
	}
	entries, err := files.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.Name() >= "011_set_technique.sql" {
			continue
		}
		sql, err := files.ReadFile(entry.Name())
		if err != nil {
			t.Fatal(err)
		}
		if _, err = pool.Exec(ctx, string(sql)); err != nil {
			t.Fatal(err)
		}
		if _, err = pool.Exec(ctx, "INSERT INTO schema_migrations(version) VALUES($1)", entry.Name()); err != nil {
			t.Fatal(err)
		}
	}
	if _, err = pool.Exec(ctx, `INSERT INTO exercises(owner_id,name) VALUES(1,'Legacy');
 INSERT INTO templates(owner_id,name) VALUES(1,'Legacy');
 INSERT INTO workout_sessions(owner_id,template_id,name,completed_at) VALUES(1,1,'Active',NULL),(1,1,'Completed',now());
 INSERT INTO session_exercises(session_id,owner_id,position,exercise_id,name,set_count) VALUES(1,1,1,1,'Legacy',2),(2,1,1,1,'Legacy',2);
 INSERT INTO session_sets(session_id,owner_id,id,exercise_position,weight,reps,rir,set_order,deleted) VALUES
 (1,1,'11111111-1111-4111-8111-111111111111',1,12.50,8,1.5,2,false),
 (2,1,'22222222-2222-4222-8222-222222222222',1,12.50,8,1.5,2,false),
 (2,1,'33333333-3333-4333-8333-333333333333',1,12.50,8,1.5,2,true)`); err != nil {
		t.Fatal(err)
	}
	var before, after string
	const records = `SELECT jsonb_agg(to_jsonb(s) - 'technique' - 'partial_reps' ORDER BY session_id,id)::text FROM session_sets s`
	if err = pool.QueryRow(ctx, records).Scan(&before); err != nil {
		t.Fatal(err)
	}
	if err = Apply(ctx, pool); err != nil {
		t.Fatal(err)
	}
	var noPartials bool
	if err = pool.QueryRow(ctx, "SELECT count(*)=3 AND count(*) FILTER(WHERE partial_reps IS NOT NULL)=0 FROM session_sets").Scan(&noPartials); err != nil || !noPartials {
		t.Fatal("migration invented partial counts", noPartials, err)
	}
	// Re-running migration application must preserve user changes.
	if _, err = pool.Exec(ctx, "UPDATE session_sets SET technique='good',partial_reps=2 WHERE session_id=1"); err != nil {
		t.Fatal(err)
	}
	if err = Apply(ctx, pool); err != nil {
		t.Fatal(err)
	}
	if err = pool.QueryRow(ctx, records).Scan(&after); err != nil || before != after {
		t.Fatal("migration changed existing set data", before, after, err)
	}
	var valid bool
	if err = pool.QueryRow(ctx, "SELECT count(*)=3 AND count(*) FILTER(WHERE technique='decent')=2 AND count(*) FILTER(WHERE technique='good')=1 AND count(*) FILTER(WHERE partial_reps=2)=1 AND count(*) FILTER(WHERE partial_reps IS NULL)=2 FROM session_sets").Scan(&valid); err != nil || !valid {
		t.Fatal("wrong backfill/rerun", valid, err)
	}
	if _, err = pool.Exec(ctx, "UPDATE session_sets SET technique='excellent'"); err == nil {
		t.Fatal("invalid stored technique accepted")
	}
	if _, err = pool.Exec(ctx, "UPDATE session_sets SET technique=NULL"); err == nil {
		t.Fatal("null stored technique accepted")
	}
	for _, value := range []int{0, -1} {
		if _, err = pool.Exec(ctx, "UPDATE session_sets SET partial_reps=$1", value); err == nil {
			t.Fatal("invalid stored partial count accepted", value)
		}
	}

}
