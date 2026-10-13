package templates

import (
	"context"
	"errors"
	"os"
	"reflect"
	"testing"

	"github.com/agersk0v/crow-fitness/backend/internal/exercises"
	"github.com/agersk0v/crow-fitness/backend/internal/migrations"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestPostgres(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL required for disposable PostgreSQL test")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if err = migrations.Apply(ctx, pool); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, "INSERT INTO owners(id,label) VALUES (23,'Template test owner'),(24,'Other template test owner')"); err != nil {
		t.Fatal(err)
	}
	defer pool.Exec(ctx, "DELETE FROM owners WHERE id IN (23,24)")
	library := exercises.Postgres{Pool: pool}
	first, err := library.Create(ctx, 23, "Squat")
	if err != nil {
		t.Fatal(err)
	}
	second, err := library.Create(ctx, 23, "Press")
	if err != nil {
		t.Fatal(err)
	}
	foreign, err := library.Create(ctx, 24, "Foreign")
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Exec(ctx, "DELETE FROM exercises WHERE owner_id IN (23,24)")
	defer pool.Exec(ctx, "DELETE FROM templates WHERE owner_id IN (23,24)")
	defer pool.Exec(ctx, "DELETE FROM template_exercises WHERE owner_id IN (23,24)")
	store := Postgres{Pool: pool}
	input := Input{Name: " Routine ", Exercises: []InputEntry{
		{ExerciseID: second.ID, SetCount: 3, RepMin: integer(8), RepMax: integer(12), TargetRIR: decimal("1.5")},
		{ExerciseID: first.ID, SetCount: 2, RepMin: integer(10), RepMax: integer(10), TargetRIR: decimal("0")},
		{ExerciseID: second.ID, SetCount: 1},
	}}
	item, err := store.Create(ctx, 23, input)
	if err != nil {
		t.Fatal(err)
	}
	if item.Name != "Routine" || len(item.Exercises) != 3 || item.Exercises[0].ExerciseID != second.ID || item.Exercises[1].ExerciseID != first.ID || item.Exercises[2].Position != 3 {
		t.Fatal(item)
	}
	if item.Exercises[2].RepMin != nil || item.Exercises[2].TargetRIR != nil {
		t.Fatal("missing targets changed to zero")
	}
	if err = migrations.Apply(ctx, pool); err != nil {
		t.Fatal(err)
	}
	fresh, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer fresh.Close()
	reopened, err := (Postgres{Pool: fresh}).Get(ctx, 23, item.ID)
	if err != nil || !reflect.DeepEqual(item, reopened) {
		t.Fatalf("round trip: %+v %v", reopened, err)
	}
	if _, err = store.Get(ctx, 24, item.ID); !errors.Is(err, ErrNotFound) {
		t.Fatal("foreign template visible")
	}
	items, err := store.List(ctx, 24)
	if err != nil || len(items) != 0 {
		t.Fatal("foreign templates listed")
	}
	for _, id := range []string{foreign.ID, "9223372036854775807"} {
		bad := Input{Name: "Invalid", Exercises: []InputEntry{{ExerciseID: first.ID, SetCount: 1}, {ExerciseID: id, SetCount: 2}}}
		_, err = store.Create(ctx, 23, bad)
		var fields ValidationError
		if !errors.As(err, &fields) || fields["exercises.1.exercise_id"] == "" {
			t.Fatalf("missing ownership validation: %v", err)
		}
	}
	// A storage failure after the first occurrence must roll back the template too.
	failed := Input{Name: "Storage failure", Exercises: []InputEntry{
		{ExerciseID: first.ID, SetCount: 1},
		{ExerciseID: second.ID, SetCount: 1, TargetRIR: decimal("1e200000")},
	}}
	if _, err = store.Create(ctx, 23, failed); err == nil {
		t.Fatal("expected numeric storage failure")
	}
	items, err = store.List(ctx, 23)
	if err != nil || len(items) != 1 {
		t.Fatalf("failed create left partial template: %+v %v", items, err)
	}
	// Database constraints independently reject cross-owner references and invalid targets.
	for _, sql := range []string{
		"INSERT INTO template_exercises(template_id,owner_id,position,exercise_id,set_count) VALUES ($1,23,4,$2,1)",
		"INSERT INTO template_exercises(template_id,owner_id,position,exercise_id,set_count) VALUES ($1,23,4,$2,0)",
		"INSERT INTO template_exercises(template_id,owner_id,position,exercise_id,set_count,rep_min) VALUES ($1,23,4,$2,1,8)",
		"INSERT INTO template_exercises(template_id,owner_id,position,exercise_id,set_count,target_rir) VALUES ($1,23,4,$2,1,-1)",
	} {
		id := first.ID
		if sql == "INSERT INTO template_exercises(template_id,owner_id,position,exercise_id,set_count) VALUES ($1,23,4,$2,1)" {
			id = foreign.ID
		}
		if _, err = pool.Exec(ctx, sql, item.ID, id); err == nil {
			t.Fatal("constraint accepted invalid entry")
		}
	}
}
