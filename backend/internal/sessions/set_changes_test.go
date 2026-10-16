package sessions

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/agersk0v/crow-fitness/backend/internal/exercises"
	"github.com/agersk0v/crow-fitness/backend/internal/migrations"
	"github.com/agersk0v/crow-fitness/backend/internal/templates"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestPostgresSetChanges(t *testing.T) {
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
	if _, err = pool.Exec(ctx, "INSERT INTO owners(id,label) VALUES(171,'Set change test'),(172,'Foreign change test')"); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_, err := pool.Exec(context.Background(), `DELETE FROM workout_sessions WHERE owner_id IN(171,172); DELETE FROM template_exercises WHERE owner_id IN(171,172); DELETE FROM templates WHERE owner_id IN(171,172); DELETE FROM exercises WHERE owner_id IN(171,172); DELETE FROM owners WHERE id IN(171,172)`)
		if err != nil {
			t.Error(err)
		}
	}()
	store := Postgres{Pool: pool}
	library := exercises.Postgres{Pool: pool}
	ts := templates.Postgres{Pool: pool}
	press, err := library.Create(ctx, 171, exercises.Input{Name: "Change press", MuscleGroups: []string{"chest"}})
	if err != nil {
		t.Fatal(err)
	}
	row, err := library.Create(ctx, 171, exercises.Input{Name: "Change row", MuscleGroups: []string{"back"}})
	if err != nil {
		t.Fatal(err)
	}
	makeTemplate := func(name string, ids ...string) templates.Template {
		t.Helper()
		entries := []templates.InputEntry{}
		for _, id := range ids {
			entries = append(entries, templates.InputEntry{ExerciseID: id, SetCount: 3})
		}
		item, err := ts.Create(ctx, 171, templates.Input{Name: name, Exercises: entries})
		if err != nil {
			t.Fatal(err)
		}
		return item
	}
	start := func(id string) Session {
		t.Helper()
		item, err := store.Start(ctx, 171, id)
		if err != nil {
			t.Fatal(err)
		}
		return item
	}
	save := func(s Session, id int, pos int64, weight string, reps int64, technique ...string) {
		t.Helper()
		rating := "decent"
		if len(technique) > 0 {
			rating = technique[0]
		}
		_, err := store.SaveSet(ctx, 171, s.ID, fmt.Sprintf("%08d-1111-4111-8111-111111111111", id), SetInput{Technique: rating, Position: pos, Weight: weight, Reps: reps})
		if err != nil {
			t.Fatal(err)
		}
	}
	complete := func(s Session) {
		t.Helper()
		if err := store.Complete(ctx, 171, s.ID); err != nil {
			t.Fatal(err)
		}
	}
	a := makeTemplate("Earlier", press.ID, row.ID, press.ID)
	b := makeTemplate("Current", row.ID, press.ID, press.ID)
	source := start(a.ID)
	save(source, 1, 1, "20.00", 8, "bad")
	save(source, 2, 1, "999", 999)
	save(source, 3, 1, "30", 10, "good")
	save(source, 4, 3, "20", 10)
	if err := store.DeleteSet(ctx, 171, source.ID, "00000002-1111-4111-8111-111111111111"); err != nil {
		t.Fatal(err)
	}
	complete(source)
	target := start(b.ID)
	save(target, 1, 2, "22.50", 9, "good")
	save(target, 2, 2, "30", 7, "bad")
	save(target, 3, 2, "40", 8)
	save(target, 4, 3, "18", 8)
	save(target, 5, 1, "5", 5)
	active, err := store.Get(ctx, 171, target.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, set := range active.Sets {
		if set.Change != nil {
			t.Fatal("active sets gained comparison")
		}
	}
	complete(target)
	// Equal completion times use session ID as a deterministic tie breaker.
	if _, err := pool.Exec(ctx, "UPDATE workout_sessions SET completed_at='2030-01-01T12:00:00Z' WHERE owner_id=171"); err != nil {
		t.Fatal(err)
	}
	detail, err := store.GetCompleted(ctx, 171, target.ID)
	if err != nil {
		t.Fatal(err)
	}
	changes := map[string]*SetChange{}
	for _, set := range detail.Sets {
		changes[set.ID] = set.Change
	}
	assertChange := func(id int, weight, reps string) {
		t.Helper()
		change := changes[fmt.Sprintf("%08d-1111-4111-8111-111111111111", id)]
		if weight == "" {
			if change != nil {
				t.Fatal("unexpected baseline", id, change)
			}
			return
		}
		if change == nil || change.Weight != weight || change.Reps != reps {
			t.Fatal("wrong difference", id, change, weight, reps)
		}
	}
	assertChange(1, "2.50", "1")
	assertChange(2, "0", "-3")
	assertChange(3, "", "")
	assertChange(4, "-2", "-2")
	assertChange(5, "", "")
	for id, want := range map[int]string{1: "improved", 2: "regressed", 4: "unchanged"} {
		change := changes[fmt.Sprintf("%08d-1111-4111-8111-111111111111", id)]
		if change.Technique != want || change.PreviousTechnique == "" {
			t.Fatal("wrong technique change", id, change, want)
		}
	}

	// Filtering excludes the source rows, but must not exclude their comparisons.
	all, err := store.CompletedSets(ctx, 171, HistoryFilter{Query: "Current", Exercise: press.ID})
	if err != nil || len(all) != 4 {
		t.Fatal(all, err)
	}
	// Partial counts are contextual only, even when changed on both baseline and current sets.
	if _, err = pool.Exec(ctx, "UPDATE session_sets SET partial_reps=99 WHERE owner_id=171"); err != nil {
		t.Fatal(err)
	}
	all, err = store.CompletedSets(ctx, 171, HistoryFilter{Query: "Current", Exercise: press.ID})
	if err != nil {
		t.Fatal(err)
	}
	for _, set := range all {
		want := changes[set.ID]
		if set.PartialReps == nil || *set.PartialReps != 99 {
			t.Fatal("partial count missing from history", set)
		}
		if (want == nil) != (set.Change == nil) || (want != nil && *want != *set.Change) {
			t.Fatal("views disagree", set, want)
		}
	}
	if foreign, err := store.CompletedSets(ctx, 172, HistoryFilter{}); err != nil || len(foreign) != 0 {
		t.Fatal("foreign comparison", foreign, err)
	}
	first, err := store.GetCompleted(ctx, 171, source.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, set := range first.Sets {
		if set.Change != nil {
			t.Fatal("future workout used as baseline", set)
		}
	}
	// Do not skip an earlier workout containing the exercise but no sets.
	empty := start(b.ID)
	complete(empty)
	last := start(b.ID)
	save(last, 1, 2, "25", 10)
	complete(last)
	final, err := store.GetCompleted(ctx, 171, last.ID)
	if err != nil || len(final.Sets) != 1 || final.Sets[0].Change != nil {
		t.Fatal("empty previous workout was skipped", final, err)
	}
}
