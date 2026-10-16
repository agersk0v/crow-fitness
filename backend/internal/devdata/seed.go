// Package devdata contains the temporary, explicitly enabled reset/seed tool.
package devdata

import (
	"context"
	"fmt"
	"time"

	"github.com/agersk0v/crow-fitness/backend/internal/exercises"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type movement struct {
	name   string
	groups []string
	sets   int
	cents  int
}
type routine struct {
	name      string
	movements []movement
}

var routines = []routine{
	{"Upper 1", []movement{
		{"Dips", []string{"chest", "triceps"}, 3, 0},
		{"Machine wide-grip pull-ups", []string{"back", "biceps"}, 3, 2500},
		{"Low-to-high cable fly", []string{"chest"}, 3, 750},
		{"Cable lateral raise", []string{"shoulders"}, 3, 500},
		{"Close-grip cable row (lats)", []string{"back"}, 3, 3000},
		{"Preacher curl", []string{"biceps"}, 3, 1500},
		{"JM press", []string{"triceps"}, 3, 2000},
	}},
	{"Lower 1", []movement{
		{"Platz squat", []string{"quadriceps", "glutes"}, 3, 4000},
		{"Leg curl", []string{"hamstrings"}, 3, 2000},
		{"Dumbbell pullover", []string{"back", "chest"}, 3, 1500},
		{"Decline crunches", []string{"core"}, 3, 0},
		{"Pushdowns", []string{"triceps"}, 3, 1500},
		{"Calf raises", []string{"calves"}, 3, 3000},
		{"Wrist curls", []string{"forearms"}, 2, 500},
		{"Hammer curls", []string{"biceps", "forearms"}, 2, 1000},
	}},
	{"Upper 2", []movement{
		{"Smith bench press", []string{"chest", "triceps"}, 3, 4000},
		{"Chest-supported flared-elbow row", []string{"back", "shoulders"}, 3, 2500},
		{"OHP", []string{"shoulders", "triceps"}, 3, 2000},
		{"Neutral-grip weighted pull-up", []string{"back", "biceps"}, 3, 0},
		{"Face pulls", []string{"shoulders", "back"}, 3, 1000},
		{"Overhead extensions", []string{"triceps"}, 3, 1000},
		{"Incline dumbbell curls", []string{"biceps"}, 3, 750},
	}},
	{"Lower 2", []movement{
		{"RDL", []string{"hamstrings", "glutes"}, 3, 5000},
		{"Leg extensions", []string{"quadriceps"}, 3, 2500},
		{"Cable lateral raise", []string{"shoulders"}, 3, 500},
		{"Decline crunches", []string{"core"}, 3, 0},
		{"Calf raises", []string{"calves"}, 3, 3000},
		{"Wrist curls", []string{"forearms"}, 2, 500},
		{"Pronated curls", []string{"biceps", "forearms"}, 2, 750},
		{"Reverse cable crossover", []string{"shoulders", "back"}, 3, 750},
	}},
}

type Result struct {
	Exercises         int    `json:"exercises"`
	Templates         int    `json:"templates"`
	CompletedSessions int    `json:"completed_sessions"`
	CompletedSets     int    `json:"completed_sets"`
	ActiveSessionID   string `json:"active_session_id"`
	ActiveSets        int    `json:"active_sets"`
}
type Postgres struct{ Pool *pgxpool.Pool }

func (p Postgres) Reset(ctx context.Context, now time.Time) (Result, error) {
	tx, err := p.Pool.Begin(ctx)
	if err != nil {
		return Result{}, err
	}
	defer tx.Rollback(ctx)
	// EXCLUSIVE blocks writes and row-locking readers while ordinary MVCC reads
	// can see the prior committed dataset. All replacement records commit together.
	if _, err = tx.Exec(ctx, `LOCK TABLE owners,exercises,templates,template_exercises,workout_sessions,session_exercises,session_sets IN EXCLUSIVE MODE`); err != nil {
		return Result{}, err
	}
	owner := exercises.LocalOwnerID
	if _, err = tx.Exec(ctx, `INSERT INTO owners(id,label) VALUES($1,'Local owner') ON CONFLICT(id) DO NOTHING`, owner); err != nil {
		return Result{}, err
	}
	for _, query := range []string{
		"DELETE FROM workout_sessions WHERE owner_id=$1",
		"DELETE FROM template_exercises WHERE owner_id=$1",
		"DELETE FROM templates WHERE owner_id=$1",
		"DELETE FROM exercises WHERE owner_id=$1",
	} {
		if _, err = tx.Exec(ctx, query, owner); err != nil {
			return Result{}, err
		}
	}
	result := Result{Templates: len(routines), CompletedSessions: 48, ActiveSets: 3}
	ids := map[string]string{}
	templateIDs := make([]string, len(routines))
	for index, r := range routines {
		if err = tx.QueryRow(ctx, "INSERT INTO templates(owner_id,name,description) VALUES($1,$2,$3) RETURNING id::text", owner, r.name, templateDescriptions[r.name]).Scan(&templateIDs[index]); err != nil {
			return Result{}, err
		}
		for position, m := range r.movements {
			if ids[m.name] == "" {
				var id string
				if err = tx.QueryRow(ctx, "INSERT INTO exercises(owner_id,name,muscle_groups,description) VALUES($1,$2,$3,$4) RETURNING id::text", owner, m.name, m.groups, exerciseDescriptions[m.name]).Scan(&id); err != nil {
					return Result{}, err
				}
				ids[m.name] = id
			}
			if _, err = tx.Exec(ctx, "INSERT INTO template_exercises(template_id,owner_id,position,exercise_id,set_count) VALUES($1,$2,$3,$4,$5)", templateIDs[index], owner, position+1, ids[m.name], m.sets); err != nil {
				return Result{}, err
			}
		}
	}
	result.Exercises = len(ids)
	utc := now.UTC()
	anchor := time.Date(utc.Year(), utc.Month(), utc.Day(), 12, 0, 0, 0, time.UTC).AddDate(0, 0, -84)
	for week := 0; week < 12; week++ {
		for index, r := range routines {
			started := anchor.AddDate(0, 0, week*7+[]int{0, 1, 3, 5}[index])
			completed := started.Add(time.Hour)
			var sessionID string
			if err = tx.QueryRow(ctx, "INSERT INTO workout_sessions(owner_id,template_id,name,started_at,completed_at) VALUES($1,$2,$3,$4,$5) RETURNING id::text", owner, templateIDs[index], r.name, started, completed).Scan(&sessionID); err != nil {
				return Result{}, err
			}
			count, err := seedSession(ctx, tx, owner, sessionID, r, ids, week, started, false)
			if err != nil {
				return Result{}, err
			}
			result.CompletedSets += count
		}
	}
	activeStarted := now.Add(-10 * time.Minute)
	if err = tx.QueryRow(ctx, "INSERT INTO workout_sessions(owner_id,template_id,name,started_at) VALUES($1,$2,$3,$4) RETURNING id::text", owner, templateIDs[0], routines[0].name, activeStarted).Scan(&result.ActiveSessionID); err != nil {
		return Result{}, err
	}
	if _, err = seedSession(ctx, tx, owner, result.ActiveSessionID, routines[0], ids, 12, activeStarted, true); err != nil {
		return Result{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return Result{}, err
	}
	return result, nil
}

func seedSession(ctx context.Context, tx pgx.Tx, owner int64, id string, r routine, ids map[string]string, week int, started time.Time, active bool) (int, error) {
	batch := &pgx.Batch{}
	count := 0
	for position, m := range r.movements {
		batch.Queue("INSERT INTO session_exercises(session_id,owner_id,position,exercise_id,name,set_count) VALUES($1,$2,$3,$4,$5,$6)", id, owner, position+1, ids[m.name], m.name, m.sets)
		if active && position > 0 {
			continue
		}
		for set := 0; set < m.sets; set++ {
			count++
			cents := m.cents + (week/2)*125 + set*25
			weight := fmt.Sprintf("%d.%02d", cents/100, cents%100)
			var partials any
			if (week+position+set)%3 == 0 {
				partials = 1 + set%3
			}
			batch.Queue("INSERT INTO session_sets(session_id,owner_id,id,exercise_position,weight,reps,saved_at,technique,partial_reps) VALUES($1,$2,$3,$4,$5::text::numeric,$6,$7,$8,$9)", id, owner, fmt.Sprintf("00000000-0000-4000-8000-%012d", count), position+1, weight, 8+(week/2+set)%5, started.Add(time.Duration(count)*time.Second), []string{"bad", "decent", "decent", "good"}[(week+position+set)%4], partials)
		}
	}
	results := tx.SendBatch(ctx, batch)
	if err := results.Close(); err != nil {
		return 0, err
	}
	return count, nil
}
