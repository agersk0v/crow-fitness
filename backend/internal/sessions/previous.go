package sessions

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

type PreviousOccurrence struct {
	Position int64      `json:"position"`
	Name     string     `json:"name"`
	Sets     []SavedSet `json:"sets"`
}
type PreviousPerformance struct {
	ExerciseID  string               `json:"exercise_id"`
	SessionID   string               `json:"session_id"`
	CompletedAt time.Time            `json:"completed_at"`
	Occurrences []PreviousOccurrence `json:"occurrences"`
}

func (p Postgres) Previous(ctx context.Context, owner int64, sessionID string) ([]PreviousPerformance, error) {
	tx, err := p.Pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	// Verify the requesting active session before exposing any history. Use the
	// same read snapshot for membership, source selection, and returned sets.
	var id string
	err = tx.QueryRow(ctx, "SELECT id::text FROM workout_sessions WHERE owner_id=$1 AND id=$2 AND completed_at IS NULL", owner, sessionID).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	rows, err := tx.Query(ctx, `WITH wanted AS (
 SELECT DISTINCT exercise_id FROM session_exercises WHERE owner_id=$1 AND session_id=$2
 ), latest AS (
 SELECT DISTINCT ON (e.exercise_id) e.exercise_id,s.id AS session_id,s.completed_at
 FROM workout_sessions s JOIN session_exercises e ON e.owner_id=s.owner_id AND e.session_id=s.id
 JOIN wanted w ON w.exercise_id=e.exercise_id
 WHERE s.owner_id=$1 AND s.completed_at IS NOT NULL
 AND EXISTS(SELECT 1 FROM session_sets ss WHERE ss.owner_id=$1 AND ss.session_id=s.id AND ss.exercise_position=e.position AND NOT ss.deleted)
 ORDER BY e.exercise_id,s.completed_at DESC,s.id DESC
 )
 SELECT l.exercise_id::text,l.session_id::text,l.completed_at,e.position,e.name,ss.id::text,ss.weight::text,ss.reps
 FROM latest l JOIN session_exercises e ON e.session_id=l.session_id AND e.owner_id=$1 AND e.exercise_id=l.exercise_id
 JOIN session_sets ss ON ss.session_id=e.session_id AND ss.owner_id=e.owner_id AND ss.exercise_position=e.position AND NOT ss.deleted
 ORDER BY l.exercise_id,e.position,ss.saved_at,ss.id`, owner, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []PreviousPerformance{}
	for rows.Next() {
		var exerciseID, sourceID, name string
		var date time.Time
		var set SavedSet
		if err = rows.Scan(&exerciseID, &sourceID, &date, &set.Position, &name, &set.ID, &set.Weight, &set.Reps); err != nil {
			return nil, err
		}
		if len(items) == 0 || items[len(items)-1].ExerciseID != exerciseID {
			items = append(items, PreviousPerformance{ExerciseID: exerciseID, SessionID: sourceID, CompletedAt: date, Occurrences: []PreviousOccurrence{}})
		}
		item := &items[len(items)-1]
		if len(item.Occurrences) == 0 || item.Occurrences[len(item.Occurrences)-1].Position != set.Position {
			item.Occurrences = append(item.Occurrences, PreviousOccurrence{Position: set.Position, Name: name, Sets: []SavedSet{}})
		}
		occurrence := &item.Occurrences[len(item.Occurrences)-1]
		occurrence.Sets = append(occurrence.Sets, set)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	rows.Close()
	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	return items, nil
}
