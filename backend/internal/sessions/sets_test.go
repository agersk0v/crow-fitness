package sessions

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/agersk0v/crow-fitness/backend/internal/exercises"
	"github.com/agersk0v/crow-fitness/backend/internal/migrations"
	"github.com/agersk0v/crow-fitness/backend/internal/templates"
	"github.com/jackc/pgx/v5/pgxpool"
)

const setID = "11111111-1111-4111-8111-111111111111"

func TestSetValidationAndHTTP(t *testing.T) {
	for _, tc := range []struct {
		weight string
		reps   int64
		valid  bool
	}{{"0", 1, true}, {"12.50", 8, true}, {"999999999.99", 8, true}, {"-1", 8, false}, {"1.001", 8, false}, {"NaN", 8, false}, {"1e2", 8, false}, {"", 8, false}, {"1", 0, false}, {"1", -2, false}} {
		fields := ValidateSet(SetInput{Technique: "decent", Position: 1, Weight: tc.weight, Reps: tc.reps})
		if (len(fields) == 0) != tc.valid {
			t.Fatal(tc, fields)
		}
	}
	for _, value := range []int64{-1, 0, 2, 1<<53 - 1, 1 << 53} {
		fields := ValidateSet(SetInput{Technique: "decent", Position: 1, Weight: "10", Reps: 8, PartialReps: &value})
		if (fields["partial_reps"] == "") != (value >= 0 && value <= 1<<53-1) {
			t.Fatal("partial validation", value, fields)
		}
	}
	partialOnly := int64(2)
	if ValidateSet(SetInput{Technique: "decent", Position: 1, Weight: "10", Reps: 0, PartialReps: &partialOnly})["reps"] == "" {
		t.Fatal("partials replaced required full reps")
	}
	for _, value := range []string{"good", "decent", "bad", "", "Good", "excellent"} {
		fields := ValidateSet(SetInput{Technique: value, Position: 1, Weight: "10", Reps: 8})
		valid := value == "good" || value == "decent" || value == "bad"
		if (fields["technique"] == "") != valid {
			t.Fatal("technique validation", value, fields)
		}
	}
	for _, value := range []string{"0", "1.5", "-1", "NaN", "abc"} {
		rir := templates.Decimal(value)
		fields := ValidateSet(SetInput{Technique: "decent", Position: 1, Weight: "0", Reps: 1, RIR: &rir})
		if (len(fields) == 0) != (value == "0" || value == "1.5") {
			t.Fatal("RIR validation", value, fields)
		}
	}

	for _, tc := range []struct {
		method, path, body string
		err                error
		status             int
	}{
		{"PUT", "/api/sessions/7/sets/" + setID, `{"technique":"decent","exercise_position":1,"weight":"0","reps":8}`, nil, 200},
		{"PUT", "/api/sessions/7/sets/" + setID, `{"technique":"decent","exercise_position":1,"weight":"0","reps":8,"rir":0}`, nil, 200},
		{"PUT", "/api/sessions/7/sets/" + setID, `{"technique":"decent","exercise_position":1,"weight":"0","reps":8,"rir":1.5}`, nil, 200},
		{"PUT", "/api/sessions/7/sets/" + setID, `{"technique":"decent","exercise_position":1,"weight":"0","reps":8,"rir":null}`, nil, 200},
		{"PUT", "/api/sessions/7/sets/" + setID, `{"technique":"decent","exercise_position":1,"weight":"0","reps":8,"rir":-1}`, ValidationError{"rir": "Enter a nonnegative RIR, or leave it blank."}, 400},
		{"PUT", "/api/sessions/7/sets/" + setID, `{"technique":"decent","exercise_position":1,"weight":"0","reps":8,"rir":"2"}`, nil, 400},
		{"PUT", "/api/sessions/7/sets/" + setID, `{"technique":"decent","exercise_position":1,"weight":"10","reps":8.5}`, nil, 400},
		{"PUT", "/api/sessions/7/sets/" + setID, `{"technique":"decent","exercise_position":1,"weight":10,"reps":8}`, nil, 400},
		{"PUT", "/api/sessions/7/sets/" + setID, `{"technique":"decent","exercise_position":1,"weight":"10","reps":8,"owner_id":2}`, nil, 400},
		{"PUT", "/api/sessions/7/sets/" + setID, `{"technique":"decent","exercise_position":1,"weight":"10","reps":8}`, ValidationError{"weight": "Invalid weight"}, 400},
		{"PUT", "/api/sessions/7/sets/" + setID, `{"technique":"decent","exercise_position":1,"weight":"10","reps":8}`, ErrCompleted, 409},
		{"PUT", "/api/sessions/7/sets/" + setID, `{"technique":7,"exercise_position":1,"weight":"10","reps":8}`, nil, 400},
		{"PUT", "/api/sessions/7/sets/" + setID, `{"technique":"decent","exercise_position":1,"weight":"10","reps":8,"partial_reps":1.5}`, nil, 400},
		{"PUT", "/api/sessions/7/sets/" + setID, `{"technique":"decent","exercise_position":1,"weight":"10","reps":8,"partial_reps":"2"}`, nil, 400},
		{"PUT", "/api/sessions/7/sets/bad", `{}`, nil, 400},
		{"DELETE", "/api/sessions/7/sets/" + setID, "", nil, 204},
		{"DELETE", "/api/sessions/7/sets/" + setID, "", ErrCompleted, 409},
		{"GET", "/api/sets", "", nil, 200},
	} {
		mux := http.NewServeMux()
		store := &fakeStore{err: tc.err}
		Register(mux, store)
		request := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
		request.Header.Set("Content-Type", "application/json")
		response := httptest.NewRecorder()
		mux.ServeHTTP(response, request)
		if response.Code != tc.status {
			t.Fatal(tc, response.Code, response.Body.String())
		}
		if response.Header().Get("Cache-Control") != "no-store" {
			t.Fatal("cached sets")
		}
		if tc.path == "/api/sets" && strings.TrimSpace(response.Body.String()) != `{"items":[]}` {
			t.Fatal(response.Body.String())
		}
	}
}
func TestPostgresSets(t *testing.T) {
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
	if _, err = pool.Exec(ctx, "INSERT INTO owners(id,label) VALUES (71,'Set test'),(72,'Other set test')"); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_, err := pool.Exec(context.Background(), `DELETE FROM workout_sessions WHERE owner_id IN(71,72);DELETE FROM template_exercises WHERE owner_id IN(71,72);DELETE FROM templates WHERE owner_id IN(71,72);DELETE FROM exercises WHERE owner_id IN(71,72);DELETE FROM owners WHERE id IN(71,72)`)
		if err != nil {
			t.Error(err)
		}
	}()
	library := exercises.Postgres{Pool: pool}
	ts := templates.Postgres{Pool: pool}
	store := Postgres{Pool: pool}
	exercise, err := library.Create(ctx, 71, exercises.Input{Name: "Captured press", MuscleGroups: []string{"chest"}})
	if err != nil {
		t.Fatal(err)
	}
	template, err := ts.Create(ctx, 71, templates.Input{Name: "Captured routine", Exercises: []templates.InputEntry{{ExerciseID: exercise.ID, SetCount: 3}, {ExerciseID: exercise.ID, SetCount: 2}}})
	if err != nil {
		t.Fatal(err)
	}
	session, err := store.Start(ctx, 71, template.ID)
	if err != nil {
		t.Fatal(err)
	}
	zeroRIR := templates.Decimal("0")
	partials := int64(2)
	input := SetInput{Technique: "decent", Position: 1, Weight: "12.50", Reps: 8, RIR: &zeroRIR, PartialReps: &partials}
	saved, err := store.SaveSet(ctx, 71, session.ID, setID, input)
	if err != nil {
		t.Fatal(err)
	}
	if saved.RIR == nil || *saved.RIR != "0" || saved.Technique != "decent" || saved.PartialReps == nil || *saved.PartialReps != 2 {
		t.Fatal("zero recorded RIR", saved)
	}
	replay, err := store.SaveSet(ctx, 71, session.ID, setID, input)
	if err != nil || !reflect.DeepEqual(saved, replay) {
		t.Fatal("save retry diverged", replay, err)
	}
	extraID := "99999999-9999-4999-8999-999999999999"
	if _, err = store.SaveSet(ctx, 71, session.ID, extraID, SetInput{Technique: "decent", Position: 1, Weight: "40", Reps: 6}); err != nil {
		t.Fatal(err)
	}
	ordered, err := store.ReorderSets(ctx, 71, session.ID, SetOrderInput{Position: 1, IDs: []string{extraID, setID}})
	if err != nil || len(ordered) != 2 || ordered[0].ID != extraID || ordered[1].ID != setID || ordered[1].PartialReps == nil || *ordered[1].PartialReps != 2 {
		t.Fatal("set reorder", ordered, err)
	}
	if _, err = store.SaveSet(ctx, 71, session.ID, setID, input); err != nil {
		t.Fatal(err)
	}
	reordered, err := store.Get(ctx, 71, session.ID)
	if err != nil || reordered.Sets[0].ID != extraID {
		t.Fatal("edit changed saved order", reordered, err)
	}
	for _, ids := range [][]string{{setID}, {setID, setID}, {setID, "88888888-8888-4888-8888-888888888888"}} {
		if _, err = store.ReorderSets(ctx, 71, session.ID, SetOrderInput{Position: 1, IDs: ids}); err == nil {
			t.Fatal("invalid permutation accepted", ids)
		}
	}
	if _, err = store.ReorderSets(ctx, 72, session.ID, SetOrderInput{Position: 1, IDs: []string{extraID, setID}}); !errors.Is(err, ErrNotFound) {
		t.Fatal("foreign reorder", err)
	}
	if err = store.DeleteSet(ctx, 71, session.ID, extraID); err != nil {
		t.Fatal(err)
	}

	// Two new requests compete for the final planned slot under the session lock.
	capFirst := "55555555-5555-4555-8555-555555555555"
	capSecond := "66666666-6666-4666-8666-666666666666"
	capThird := "77777777-7777-4777-8777-777777777777"
	capReplacement := "88888888-8888-4888-8888-888888888888"
	if _, err = store.SaveSet(ctx, 71, session.ID, capFirst, input); err != nil {
		t.Fatal(err)
	}
	capResults := make(chan error, 2)
	for _, id := range []string{capSecond, capThird} {
		go func(id string) { _, err := store.SaveSet(ctx, 71, session.ID, id, input); capResults <- err }(id)
	}
	capAccepted, capRejected := 0, 0
	for i := 0; i < 2; i++ {
		err := <-capResults
		if err == nil {
			capAccepted++
		} else if _, ok := err.(ValidationError); ok {
			capRejected++
		} else {
			t.Fatal(err)
		}
	}
	if capAccepted != 1 || capRejected != 1 {
		t.Fatal("planned cap race", capAccepted, capRejected)
	}
	if _, err = store.SaveSet(ctx, 71, session.ID, capFirst, input); err != nil {
		t.Fatal("retry at cap", err)
	}
	if _, err = store.SaveSet(ctx, 71, session.ID, capReplacement, input); err == nil {
		t.Fatal("extra set capAccepted")
	}
	if err = store.DeleteSet(ctx, 71, session.ID, capFirst); err != nil {
		t.Fatal(err)
	}
	if _, err = store.SaveSet(ctx, 71, session.ID, capReplacement, input); err != nil {
		t.Fatal("deleted slot unavailable", err)
	}
	for _, id := range []string{capSecond, capThird, capReplacement} {
		if err = store.DeleteSet(ctx, 71, session.ID, id); err != nil {
			t.Fatal(err)
		}
	}

	if _, err = store.SaveSet(ctx, 72, session.ID, setID, input); !errors.Is(err, ErrNotFound) {
		t.Fatal("foreign session write", err)
	}
	wrong := input
	wrong.Position = 2
	if _, err = store.SaveSet(ctx, 71, session.ID, setID, wrong); !errors.Is(err, ErrOccurrence) {
		t.Fatal("moved saved set between occurrences", err)
	}
	wrong.Position = 99
	if _, err = store.SaveSet(ctx, 71, session.ID, "22222222-2222-4222-8222-222222222222", wrong); err == nil {
		t.Fatal("foreign occurrence accepted")
	}
	decimalRIR := templates.Decimal("1.5")
	input.RIR = &decimalRIR
	withRIR, err := store.SaveSet(ctx, 71, session.ID, setID, input)
	if err != nil || withRIR.RIR == nil || *withRIR.RIR != "1.5" {
		t.Fatal("decimal RIR", withRIR, err)
	}
	input.RIR = nil
	cleared, err := store.SaveSet(ctx, 71, session.ID, setID, input)
	if err != nil || cleared.RIR != nil {
		t.Fatal("clear RIR", cleared, err)
	}

	zeroPartials := int64(0)
	input.PartialReps = &zeroPartials
	input.Technique = "good"
	input.Weight = "0"
	input.Reps = 10
	if _, err = store.SaveSet(ctx, 71, session.ID, setID, input); err != nil {
		t.Fatal(err)
	}
	secondID := "22222222-2222-4222-8222-222222222222"
	if _, err = store.SaveSet(ctx, 71, session.ID, secondID, SetInput{Technique: "decent", Position: 2, Weight: "20.25", Reps: 5, RIR: &decimalRIR, PartialReps: &partials}); err != nil {
		t.Fatal(err)
	}
	deleteID := "33333333-3333-4333-8333-333333333333"
	if _, err = store.SaveSet(ctx, 71, session.ID, deleteID, SetInput{Technique: "decent", Position: 1, Weight: "9", Reps: 2}); err != nil {
		t.Fatal(err)
	}
	updatedTechnique, err := store.Get(ctx, 71, session.ID)
	if err != nil || updatedTechnique.Sets[0].Technique != "good" || updatedTechnique.Sets[0].PartialReps != nil {
		t.Fatal("edited technique was not retained", updatedTechnique, err)
	}
	if err = store.DeleteSet(ctx, 71, session.ID, deleteID); err != nil {
		t.Fatal(err)
	}
	if err = store.DeleteSet(ctx, 71, session.ID, deleteID); err != nil {
		t.Fatal(err)
	}
	if _, err = store.SaveSet(ctx, 71, session.ID, deleteID, input); !errors.Is(err, ErrDeleted) {
		t.Fatal("late retry resurrected deleted set", err)
	}
	active, err := store.Get(ctx, 71, session.ID)
	if err != nil || len(active.Sets) != 2 || active.Sets[0].Weight != "0" || active.Sets[1].Position != 2 || active.Sets[0].RIR != nil || active.Sets[1].RIR == nil || *active.Sets[1].RIR != "1.5" {
		t.Fatal("active readback", active, err)
	}
	listed, err := store.CompletedSets(ctx, 71, HistoryFilter{})
	if err != nil || len(listed) != 0 {
		t.Fatal("active sets published", listed, err)
	}
	if _, err = pool.Exec(ctx, "UPDATE session_sets SET weight=1.001 WHERE session_id=$1 AND id=$2", session.ID, setID); err == nil {
		t.Fatal("database silently rounded weight")
	}
	if _, err = library.Update(ctx, 71, exercise.ID, exercises.Input{Name: "Renamed", MuscleGroups: []string{"chest"}}); err != nil {
		t.Fatal(err)
	}
	if _, err = ts.SetArchived(ctx, 71, template.ID, true); err != nil {
		t.Fatal(err)
	}
	if err = store.Complete(ctx, 71, session.ID); err != nil {
		t.Fatal(err)
	}
	snapshot, err := store.GetCompleted(ctx, 71, session.ID)
	if err != nil || snapshot.Sets[1].RIR == nil || *snapshot.Sets[1].RIR != "1.5" {
		t.Fatal("completed RIR", snapshot, err)
	}

	if _, err = store.ReorderSets(ctx, 71, session.ID, SetOrderInput{Position: 1, IDs: []string{setID}}); !errors.Is(err, ErrCompleted) {
		t.Fatal("completed reorder", err)
	}

	var completed time.Time
	if err = pool.QueryRow(ctx, "SELECT completed_at FROM workout_sessions WHERE owner_id=71 AND id=$1", session.ID).Scan(&completed); err != nil {
		t.Fatal(err)
	}
	if err = store.Complete(ctx, 71, session.ID); err != nil {
		t.Fatal(err)
	}
	listed, err = store.CompletedSets(ctx, 71, HistoryFilter{})
	if err != nil || len(listed) != 2 || listed[0].ExerciseName != "Captured press" || listed[0].ExerciseID != exercise.ID || listed[0].SessionName != template.Name || listed[0].Weight != "0" || !listed[0].CompletedAt.Equal(completed) || listed[1].Position != 2 {
		t.Fatal("completed list", listed, err)
	}
	if foreign, err := store.CompletedSets(ctx, 72, HistoryFilter{}); err != nil || len(foreign) != 0 {
		t.Fatal("foreign history exposed", foreign, err)
	}
	if _, err = store.SaveSet(ctx, 71, session.ID, setID, input); !errors.Is(err, ErrCompleted) {
		t.Fatal("completed set edited", err)
	}
	if err = store.DeleteSet(ctx, 71, session.ID, setID); !errors.Is(err, ErrCompleted) {
		t.Fatal("completed set deleted", err)
	}
	if err = store.End(ctx, 71, session.ID); err != nil {
		t.Fatal(err)
	}
	if after, err := store.CompletedSets(ctx, 71, HistoryFilter{}); err != nil || !reflect.DeepEqual(after, listed) {
		t.Fatal("stale cancel changed completed history", after, err)
	}
	if _, err = ts.SetArchived(ctx, 71, template.ID, false); err != nil {
		t.Fatal(err)
	}
	cancelled, err := store.Start(ctx, 71, template.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = store.SaveSet(ctx, 71, cancelled.ID, setID, input); err != nil {
		t.Fatal(err)
	}
	if err = store.End(ctx, 71, cancelled.ID); err != nil {
		t.Fatal(err)
	}
	var count int
	if err = pool.QueryRow(ctx, "SELECT count(*) FROM session_sets WHERE session_id=$1", cancelled.ID).Scan(&count); err != nil || count != 0 {
		t.Fatal("cancel retained sets", count, err)
	}
	// Empty completion is allowed and contributes no rows.
	empty, err := store.Start(ctx, 71, template.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err = store.Complete(ctx, 71, empty.ID); err != nil {
		t.Fatal(err)
	}
	if after, err := store.CompletedSets(ctx, 71, HistoryFilter{}); err != nil || !reflect.DeepEqual(after, listed) {
		t.Fatal("empty completion published sets", after, err)
	}
	// Simultaneous set saves and completion either include the whole write or reject it.
	racing, err := store.Start(ctx, 71, template.ID)
	if err != nil {
		t.Fatal(err)
	}
	gate := make(chan struct{})
	var wg sync.WaitGroup
	results := make(chan error, 2)
	wg.Add(2)
	go func() {
		defer wg.Done()
		<-gate
		_, err := store.SaveSet(ctx, 71, racing.ID, setID, input)
		results <- err
	}()
	go func() { defer wg.Done(); <-gate; results <- store.Complete(ctx, 71, racing.ID) }()
	close(gate)
	wg.Wait()
	close(results)
	rejected := 0
	for err := range results {
		if errors.Is(err, ErrCompleted) {
			rejected++
		} else if err != nil {
			t.Fatal(err)
		}
	}
	if err = pool.QueryRow(ctx, "SELECT count(*) FROM session_sets WHERE session_id=$1 AND NOT deleted", racing.ID).Scan(&count); err != nil || count != 1-rejected {
		t.Fatal("completion/write race left inconsistent sets", count, rejected, err)
	}
	fresh, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer fresh.Close()
	after, err := (Postgres{Pool: fresh}).CompletedSets(ctx, 71, HistoryFilter{})
	if err != nil || len(after) != len(listed)+count {
		t.Fatal("history lost on reconnect", after, err)
	}
	// Wire weights remain decimal strings without binary rounding.
	encoded, err := json.Marshal(listed[1])
	if err != nil || !strings.Contains(string(encoded), `"weight":"20.25"`) {
		t.Fatal(string(encoded), err)
	}
}
