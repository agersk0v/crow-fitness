package sessions

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

type ExerciseWorkout struct {
	SessionID   string               `json:"session_id"`
	Name        string               `json:"name"`
	CompletedAt time.Time            `json:"completed_at"`
	Occurrences []PreviousOccurrence `json:"occurrences"`
}
type ExerciseHistory struct {
	ID       string            `json:"id"`
	Name     string            `json:"name"`
	Workouts []ExerciseWorkout `json:"workouts"`
}

func (p Postgres) ExerciseHistory(ctx context.Context, owner int64, id string) (ExerciseHistory, error) {
	tx, err := p.Pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return ExerciseHistory{}, err
	}
	defer tx.Rollback(ctx)
	item := ExerciseHistory{ID: id, Workouts: []ExerciseWorkout{}}
	// Prefer the library name; a deleted exercise can still be identified by its
	// owner's captured completed snapshot. Neither lookup exposes foreign data.
	err = tx.QueryRow(ctx, `SELECT name FROM (
 SELECT name,0 AS priority,NULL::timestamptz AS completed_at,NULL::bigint AS session_id,0::bigint AS position
 FROM exercises WHERE owner_id=$1 AND id=$2
 UNION ALL
 SELECT e.name,1,s.completed_at,s.id,e.position FROM session_exercises e
 JOIN workout_sessions s ON s.owner_id=e.owner_id AND s.id=e.session_id
 WHERE e.owner_id=$1 AND e.exercise_id=$2 AND s.completed_at IS NOT NULL
 ) names ORDER BY priority,completed_at DESC,session_id DESC,position LIMIT 1`, owner, id).Scan(&item.Name)
	if errors.Is(err, pgx.ErrNoRows) {
		return item, ErrNotFound
	}
	if err != nil {
		return item, err
	}
	rows, err := tx.Query(ctx, `SELECT s.id::text,s.name,s.completed_at,e.position,e.name,ss.id::text,ss.weight::text,ss.reps,ss.rir::text
 FROM workout_sessions s JOIN session_exercises e ON e.owner_id=s.owner_id AND e.session_id=s.id
 JOIN session_sets ss ON ss.owner_id=e.owner_id AND ss.session_id=e.session_id AND ss.exercise_position=e.position
 WHERE s.owner_id=$1 AND s.completed_at IS NOT NULL AND e.exercise_id=$2 AND NOT ss.deleted
 ORDER BY s.completed_at DESC,s.id DESC,e.position,ss.saved_at,ss.id`, owner, id)
	if err != nil {
		return item, err
	}
	defer rows.Close()
	for rows.Next() {
		var sessionID, workoutName, exerciseName string
		var completed time.Time
		var set SavedSet
		if err = rows.Scan(&sessionID, &workoutName, &completed, &set.Position, &exerciseName, &set.ID, &set.Weight, &set.Reps, &set.RIR); err != nil {
			return item, err
		}
		if len(item.Workouts) == 0 || item.Workouts[len(item.Workouts)-1].SessionID != sessionID {
			item.Workouts = append(item.Workouts, ExerciseWorkout{SessionID: sessionID, Name: workoutName, CompletedAt: completed, Occurrences: []PreviousOccurrence{}})
		}
		workout := &item.Workouts[len(item.Workouts)-1]
		if len(workout.Occurrences) == 0 || workout.Occurrences[len(workout.Occurrences)-1].Position != set.Position {
			workout.Occurrences = append(workout.Occurrences, PreviousOccurrence{Position: set.Position, Name: exerciseName, Sets: []SavedSet{}})
		}
		occurrence := &workout.Occurrences[len(workout.Occurrences)-1]
		occurrence.Sets = append(occurrence.Sets, set)
	}
	if err = rows.Err(); err != nil {
		return item, err
	}
	rows.Close()
	if err = tx.Commit(ctx); err != nil {
		return item, err
	}
	return item, nil
}
