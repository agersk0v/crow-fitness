package sessions

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

type PersonalBest = PreviousPerformance

func (p Postgres) PersonalBests(ctx context.Context, owner int64, sessionID string) ([]PersonalBest, error) {
	tx, err := p.Pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	var id string
	err = tx.QueryRow(ctx, "SELECT id::text FROM workout_sessions WHERE owner_id=$1 AND id=$2 AND completed_at IS NULL", owner, sessionID).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	rows, err := tx.Query(ctx, `WITH best AS (
 SELECT DISTINCT ON(e.exercise_id) e.exercise_id,s.id AS session_id,s.completed_at
 FROM workout_sessions s JOIN session_exercises e ON e.owner_id=s.owner_id AND e.session_id=s.id
 JOIN session_sets ss ON ss.owner_id=e.owner_id AND ss.session_id=e.session_id AND ss.exercise_position=e.position
 WHERE s.owner_id=$1 AND s.completed_at IS NOT NULL AND NOT ss.deleted
 AND EXISTS(SELECT 1 FROM session_exercises wanted WHERE wanted.owner_id=$1 AND wanted.session_id=$2 AND wanted.exercise_id=e.exercise_id)
 ORDER BY e.exercise_id,ss.weight DESC,ss.reps DESC,array_position(ARRAY['bad','decent','good'],ss.technique) DESC,s.completed_at DESC,s.id DESC,e.position,ss.set_order,ss.saved_at,ss.id
 )
 SELECT b.exercise_id::text,b.session_id::text,b.completed_at,e.position,e.name,ss.id::text,ss.weight::text,ss.reps,ss.rir::text,ss.technique,ss.partial_reps
 FROM best b JOIN session_exercises e ON e.owner_id=$1 AND e.session_id=b.session_id AND e.exercise_id=b.exercise_id
 JOIN session_sets ss ON ss.owner_id=e.owner_id AND ss.session_id=e.session_id AND ss.exercise_position=e.position AND NOT ss.deleted
 ORDER BY b.exercise_id,e.position,ss.set_order,ss.saved_at,ss.id`, owner, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []PersonalBest{}
	for rows.Next() {
		var exerciseID, sourceID, name string
		var date time.Time
		var set SavedSet
		if err = rows.Scan(&exerciseID, &sourceID, &date, &set.Position, &name, &set.ID, &set.Weight, &set.Reps, &set.RIR, &set.Technique, &set.PartialReps); err != nil {
			return nil, err
		}
		if len(items) == 0 || items[len(items)-1].ExerciseID != exerciseID {
			items = append(items, PersonalBest{ExerciseID: exerciseID, SessionID: sourceID, CompletedAt: date, Occurrences: []PreviousOccurrence{}})
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
