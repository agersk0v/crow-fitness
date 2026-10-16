package sessions

import (
	"context"
	"errors"
	"fmt"
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

func TestExerciseAlternativesLifecycleAndConcurrency(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL required for disposable PostgreSQL test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if err = migrations.Apply(ctx, pool); err != nil {
		t.Fatal(err)
	}
	const owner int64 = 172
	if _, err = pool.Exec(ctx, "INSERT INTO owners(id,label) VALUES($1,'Alternatives test')", owner); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_, err := pool.Exec(context.Background(), `DELETE FROM workout_sessions WHERE owner_id=172; DELETE FROM template_exercises WHERE owner_id=172; DELETE FROM templates WHERE owner_id=172; DELETE FROM exercises WHERE owner_id=172; DELETE FROM owners WHERE id=172`)
		if err != nil {
			t.Error(err)
		}
	}()
	library := exercises.Postgres{Pool: pool}
	ts := templates.Postgres{Pool: pool}
	store := Postgres{Pool: pool}
	makeExercise := func(name string) string {
		e, err := library.Create(ctx, owner, exercises.Input{Name: name, MuscleGroups: []string{"chest"}})
		if err != nil {
			t.Fatal(err)
		}
		return e.ID
	}
	dips, bench := makeExercise("Dips"), makeExercise("Bench")
	// Distinct sources prove references and comparisons follow identity, not slot.
	source, err := ts.Create(ctx, owner, templates.Input{Name: "Previous", Exercises: []templates.InputEntry{{ExerciseID: bench, SetCount: 2}, {ExerciseID: dips, SetCount: 1}}})
	if err != nil {
		t.Fatal(err)
	}
	previous, err := store.Start(ctx, owner, source.ID)
	if err != nil {
		t.Fatal(err)
	}
	for i, input := range []SetInput{{Position: 1, Weight: "80.50", Reps: 8, Technique: "good"}, {Position: 1, Weight: "75.25", Reps: 9, Technique: "decent"}, {Position: 2, Weight: "5", Reps: 10, Technique: "decent"}} {
		if _, err = store.SaveSet(ctx, owner, previous.ID, fmt.Sprintf("72000000-0000-4000-8000-%012d", i), input); err != nil {
			t.Fatal(err)
		}
	}
	if err = store.Complete(ctx, owner, previous.ID); err != nil {
		t.Fatal(err)
	}
	rir := templates.Decimal("1.1234567890123456789")
	min, max := int64(6), int64(10)
	input := templates.Input{Name: "Choices", Description: "Saved description", Exercises: []templates.InputEntry{
		{ExerciseID: dips, SetCount: 1, Alternatives: []templates.Choice{{ExerciseID: bench, SetCount: 2, RepMin: &min, RepMax: &max, TargetRIR: &rir}}},
		{ExerciseID: dips, SetCount: 3},
	}}
	template, err := ts.Create(ctx, owner, input)
	if err != nil {
		t.Fatal(err)
	}
	if len(template.Exercises[0].Alternatives) != 1 || *template.Exercises[0].Alternatives[0].TargetRIR != rir {
		t.Fatal(template)
	}
	copy, err := ts.Duplicate(ctx, owner, template.ID, "Copy")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(copy.Exercises, template.Exercises) || copy.Description != template.Description {
		t.Fatal("copy lost choices", copy)
	}
	if _, err = ts.SetArchived(ctx, owner, copy.ID, true); err != nil {
		t.Fatal(err)
	}
	refs, err := library.Usage(ctx, owner, bench)
	if err != nil || len(refs) != 3 {
		t.Fatal("alternative usage not distinct or archived missing", refs, err)
	}
	if err = library.Delete(ctx, owner, bench); err == nil {
		t.Fatal("deleted an alternative")
	}
	session, err := store.Start(ctx, owner, template.ID)
	if err != nil {
		t.Fatal(err)
	}
	if session.Exercises[0].ExerciseID != dips || len(session.Exercises[0].Choices) != 2 || session.Exercises[0].Choices[0].ExerciseID != dips {
		t.Fatal(session)
	}
	zero, one, two := int64(0), int64(1), int64(2)
	selection := SelectionInput{Position: 1, ExerciseID: bench, Revision: &zero}
	if _, err = store.SelectExercise(ctx, owner+1, session.ID, selection); !errors.Is(err, ErrNotFound) {
		t.Fatal("foreign selection", err)
	}
	invalid := selection
	invalid.ExerciseID = "999999999"
	if _, err = store.SelectExercise(ctx, owner, session.ID, invalid); err == nil {
		t.Fatal("uncaptured choice accepted")
	}
	// Later template/name changes cannot rewrite captured choices or targets.
	if _, err = ts.Update(ctx, owner, template.ID, templates.Input{Name: "Changed", Exercises: []templates.InputEntry{{ExerciseID: dips, SetCount: 4}}}); err != nil {
		t.Fatal(err)
	}
	if _, err = library.Update(ctx, owner, bench, exercises.Input{Name: "Renamed bench", MuscleGroups: []string{"chest"}}); err != nil {
		t.Fatal(err)
	}
	selected, err := store.SelectExercise(ctx, owner, session.ID, selection)
	if err != nil {
		t.Fatal(err)
	}
	e := selected.Exercises[0]
	if e.ExerciseID != bench || e.Name != "Bench" || e.SetCount != 2 || *e.TargetRIR != rir || e.SelectionRevision != 1 || selected.Exercises[1].SetCount != 3 {
		t.Fatal("switch lost snapshot", selected)
	}
	if _, err = store.SelectExercise(ctx, owner, session.ID, selection); !errors.Is(err, ErrSelection) {
		t.Fatal("CAS replay accepted", err)
	}
	refsPrevious, err := store.Previous(ctx, owner, session.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, ref := range refsPrevious {
		if ref.ExerciseID == bench && (len(ref.Occurrences) != 1 || ref.Occurrences[0].Sets[0].Weight != "80.50") {
			t.Fatal("wrong exercise reference", ref)
		}
	}
	bests, err := store.PersonalBests(ctx, owner, session.ID)
	if err != nil || len(bests) != 2 {
		t.Fatal(bests, err)
	}
	id := "72000000-0000-4000-8000-000000000020"
	set := SetInput{Position: 1, Weight: "82.50", Reps: 8, Technique: "good", SelectionRevision: &zero}
	if _, err = store.SaveSet(ctx, owner, session.ID, id, set); !errors.Is(err, ErrSelection) {
		t.Fatal("stale save accepted", err)
	}
	set.SelectionRevision = nil
	if _, err = store.SaveSet(ctx, owner, session.ID, id, set); !errors.Is(err, ErrSelection) {
		t.Fatal("unbound save accepted", err)
	}
	set.SelectionRevision = &one
	if _, err = store.SaveSet(ctx, owner, session.ID, id, set); err != nil {
		t.Fatal(err)
	}
	if _, err = store.SelectExercise(ctx, owner, session.ID, SelectionInput{Position: 1, ExerciseID: dips, Revision: &one}); !errors.Is(err, ErrHasSets) {
		t.Fatal("switched with saved set", err)
	}
	if err = store.DeleteSet(ctx, owner, session.ID, id); err != nil {
		t.Fatal(err)
	}
	if _, err = store.SelectExercise(ctx, owner, session.ID, SelectionInput{Position: 1, ExerciseID: dips, Revision: &one}); err != nil {
		t.Fatal(err)
	}
	set.SelectionRevision = &two
	if _, err = store.SaveSet(ctx, owner, session.ID, id, set); !errors.Is(err, ErrDeleted) {
		t.Fatal("UUID resurrected", err)
	}
	// A->B->A cannot make an old A request valid again.
	set.SelectionRevision = &zero
	if _, err = store.SaveSet(ctx, owner, session.ID, "72000000-0000-4000-8000-000000000021", set); !errors.Is(err, ErrSelection) {
		t.Fatal("ABA stale save accepted", err)
	}
	if _, err = store.ReorderSets(ctx, owner, session.ID, SetOrderInput{Position: 1, SelectionRevision: &zero}); !errors.Is(err, ErrSelection) {
		t.Fatal("stale order accepted", err)
	}
	active, err := store.Active(ctx, owner)
	if err != nil || active.Exercises[0].ExerciseID != dips || active.Exercises[0].SelectionRevision != 2 {
		t.Fatal("selection not persisted", active, err)
	}
	if err = store.End(ctx, owner, session.ID); err != nil {
		t.Fatal(err)
	}
	// Repeated races must yield either the old exercise's set OR the new
	// exercise selection with no set, never a reassigned set.
	for i := 0; i < 8; i++ {
		s, err := store.Start(ctx, owner, copy.ID)
		if errors.Is(err, ErrArchived) {
			if _, err = ts.SetArchived(ctx, owner, copy.ID, false); err != nil {
				t.Fatal(err)
			}
			s, err = store.Start(ctx, owner, copy.ID)
		}
		if err != nil {
			t.Fatal(err)
		}
		var saveErr, switchErr error
		var wg sync.WaitGroup
		wg.Add(2)
		start := make(chan struct{})
		go func() {
			defer wg.Done()
			<-start
			_, saveErr = store.SaveSet(ctx, owner, s.ID, id, SetInput{Position: 1, Weight: "5", Reps: 10, Technique: "decent", SelectionRevision: &zero})
		}()
		go func() { defer wg.Done(); <-start; _, switchErr = store.SelectExercise(ctx, owner, s.ID, selection) }()
		close(start)
		wg.Wait()
		state, err := store.Get(ctx, owner, s.ID)
		if err != nil {
			t.Fatal(err)
		}
		if saveErr == nil {
			if !errors.Is(switchErr, ErrHasSets) || state.Exercises[0].ExerciseID != dips || len(state.Sets) != 1 {
				t.Fatal(saveErr, switchErr, state)
			}
		} else if switchErr == nil {
			if !errors.Is(saveErr, ErrSelection) || state.Exercises[0].ExerciseID != bench || len(state.Sets) != 0 {
				t.Fatal(saveErr, switchErr, state)
			}
		} else {
			t.Fatal(saveErr, switchErr)
		}
		if i == 7 {
			rev := state.Exercises[0].SelectionRevision
			if len(state.Sets) == 0 {
				if _, err = store.SaveSet(ctx, owner, s.ID, id, SetInput{Position: 1, Weight: "82.50", Reps: 8, Technique: "good", SelectionRevision: &rev}); err != nil {
					t.Fatal(err)
				}
			}
			if err = store.Complete(ctx, owner, s.ID); err != nil {
				t.Fatal(err)
			}
			if _, err = store.SelectExercise(ctx, owner, s.ID, SelectionInput{Position: 1, ExerciseID: dips, Revision: &rev}); !errors.Is(err, ErrCompleted) {
				t.Fatal("completed selection changed", err)
			}
			completed, err := store.GetCompleted(ctx, owner, s.ID)
			if err != nil || completed.Exercises[0].ExerciseID != state.Exercises[0].ExerciseID {
				t.Fatal(completed, err)
			}
			results, err := store.CompletedSets(ctx, owner, HistoryFilter{})
			if err != nil {
				t.Fatal(err)
			}
			for _, r := range results {
				if r.SessionID == s.ID && r.ExerciseID != state.Exercises[0].ExerciseID {
					t.Fatal("history misattributed", r)
				}
			}
		} else if err = store.End(ctx, owner, s.ID); err != nil {
			t.Fatal(err)
		}
	}
}
