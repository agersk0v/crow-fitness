package templates_test

import (
	"context"
	"errors"
	"github.com/agersk0v/crow-fitness/backend/internal/exercises"
	"github.com/agersk0v/crow-fitness/backend/internal/migrations"
	"github.com/agersk0v/crow-fitness/backend/internal/sessions"
	"github.com/agersk0v/crow-fitness/backend/internal/templates"
	"github.com/jackc/pgx/v5/pgxpool"
	"os"
	"reflect"
	"testing"
	"time"
)

func TestDescriptionsRoundTripCopyAndSnapshotIsolation(t *testing.T) {
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
	if _, err = pool.Exec(ctx, "INSERT INTO owners(id,label) VALUES(152,'Description test'),(153,'Foreign description test')"); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_, err := pool.Exec(context.Background(), `DELETE FROM workout_sessions WHERE owner_id IN(152,153);DELETE FROM template_exercises WHERE owner_id IN(152,153);DELETE FROM templates WHERE owner_id IN(152,153);DELETE FROM exercises WHERE owner_id IN(152,153);DELETE FROM owners WHERE id IN(152,153)`)
		if err != nil {
			t.Error(err)
		}
	}()
	library := exercises.Postgres{Pool: pool}
	ts := templates.Postgres{Pool: pool}
	workouts := sessions.Postgres{Pool: pool}
	exercise, err := library.Create(ctx, 152, exercises.Input{Name: "Press", Description: "  First line\n<script>plain text</script>  ", MuscleGroups: []string{"chest"}})
	if err != nil || exercise.Description != "First line\n<script>plain text</script>" {
		t.Fatal("exercise normalization", exercise, err)
	}
	input := templates.Input{Name: "Routine", Description: "  Routine notes\nSecond line  ", Exercises: []templates.InputEntry{{ExerciseID: exercise.ID, SetCount: 3}}}
	template, err := ts.Create(ctx, 152, input)
	if err != nil || template.Description != "Routine notes\nSecond line" {
		t.Fatal("template normalization", template, err)
	}
	completed, err := workouts.Start(ctx, 152, template.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = workouts.SaveSet(ctx, 152, completed.ID, "11111111-1111-4111-8111-111111111111", sessions.SetInput{Position: 1, Weight: "12.50", Reps: 8}); err != nil {
		t.Fatal(err)
	}
	if err = workouts.Complete(ctx, 152, completed.ID); err != nil {
		t.Fatal(err)
	}
	completed, err = workouts.GetCompleted(ctx, 152, completed.ID)
	if err != nil {
		t.Fatal(err)
	}
	active, err := workouts.Start(ctx, 152, template.ID)
	if err != nil {
		t.Fatal(err)
	}
	copied, err := ts.Duplicate(ctx, 152, template.ID, "Copy")
	if err != nil || copied.Description != template.Description {
		t.Fatal("description not copied", copied, err)
	}
	for _, archived := range []bool{true, false} {
		item, err := ts.SetArchived(ctx, 152, template.ID, archived)
		if err != nil || item.Description != template.Description {
			t.Fatal("archive/restore lost description", item, err)
		}
	}
	if _, err = library.Update(ctx, 153, exercise.ID, exercises.Input{Name: "Foreign", Description: "Changed", MuscleGroups: []string{"chest"}}); !errors.Is(err, exercises.ErrNotFound) {
		t.Fatal("foreign exercise update", err)
	}
	if _, err = ts.Update(ctx, 153, template.ID, input); !errors.Is(err, templates.ErrNotFound) {
		t.Fatal("foreign template update", err)
	}
	cleared, err := library.Update(ctx, 152, exercise.ID, exercises.Input{Name: "Press", Description: " \n\t ", MuscleGroups: []string{"chest"}})
	if err != nil || cleared.Description != "" {
		t.Fatal("clear exercise", cleared, err)
	}
	input.Description = " \n\t "
	template, err = ts.Update(ctx, 152, template.ID, input)
	if err != nil || template.Description != "" {
		t.Fatal("clear template", template, err)
	}
	listed, err := ts.List(ctx, 152, false)
	if err != nil || len(listed) != 2 || listed[0].Description != "" || listed[1].Description != copied.Description {
		t.Fatal("list descriptions", listed, err)
	}
	after, err := workouts.Get(ctx, 152, active.ID)
	if err != nil || !reflect.DeepEqual(active, after) {
		t.Fatal("active snapshot changed", after, err)
	}
	history, err := workouts.GetCompleted(ctx, 152, completed.ID)
	if err != nil || !reflect.DeepEqual(completed, history) {
		t.Fatal("completed snapshot changed", history, err)
	}
	fresh, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer fresh.Close()
	reopened, err := (templates.Postgres{Pool: fresh}).Get(ctx, 152, copied.ID)
	if err != nil || reopened.Description != copied.Description {
		t.Fatal("description lost on reconnect", reopened, err)
	}
}
