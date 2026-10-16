package sessions

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
)

type PersonalBest struct {
	ExerciseID string   `json:"exercise_id"`
	Set        SavedSet `json:"set"`
}

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
	rows, err := tx.Query(ctx, `SELECT DISTINCT ON(e.exercise_id) e.exercise_id::text,ss.id::text,e.position,ss.weight::text,ss.reps
 FROM workout_sessions s JOIN session_exercises e ON e.owner_id=s.owner_id AND e.session_id=s.id
 JOIN session_sets ss ON ss.owner_id=e.owner_id AND ss.session_id=e.session_id AND ss.exercise_position=e.position
 WHERE s.owner_id=$1 AND s.completed_at IS NOT NULL AND NOT ss.deleted
 AND EXISTS(SELECT 1 FROM session_exercises wanted WHERE wanted.owner_id=$1 AND wanted.session_id=$2 AND wanted.exercise_id=e.exercise_id)
 ORDER BY e.exercise_id,ss.weight DESC,ss.reps DESC,s.completed_at DESC,s.id DESC,e.position,ss.saved_at,ss.id`, owner, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []PersonalBest{}
	for rows.Next() {
		var item PersonalBest
		if err = rows.Scan(&item.ExerciseID, &item.Set.ID, &item.Set.Position, &item.Set.Weight, &item.Set.Reps); err != nil {
			return nil, err
		}
		items = append(items, item)
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
