package sessions

import (
	"context"
)

type SetOrderInput struct {
	SelectionRevision *int64   `json:"selection_revision"`
	Position          int64    `json:"exercise_position"`
	IDs               []string `json:"set_ids"`
}

func (p Postgres) ReorderSets(ctx context.Context, owner int64, sessionID string, input SetOrderInput) ([]SavedSet, error) {
	if input.Position <= 0 {
		return nil, ValidationError{"exercise_position": "Choose an exercise."}
	}
	tx, err := p.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	if err = lockSession(ctx, tx, owner, sessionID); err != nil {
		return nil, err
	}
	if err = checkSelection(ctx, tx, owner, sessionID, input.Position, input.SelectionRevision); err != nil {
		return nil, err
	}
	var exists bool
	if err = tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM session_exercises WHERE owner_id=$1 AND session_id=$2 AND position=$3)", owner, sessionID, input.Position).Scan(&exists); err != nil {
		return nil, err
	}
	if !exists {
		return nil, ValidationError{"exercise_position": "This exercise is not in the active workout."}
	}
	rows, err := tx.Query(ctx, "SELECT id::text FROM session_sets WHERE owner_id=$1 AND session_id=$2 AND exercise_position=$3 AND NOT deleted", owner, sessionID, input.Position)
	if err != nil {
		return nil, err
	}
	remaining := map[string]bool{}
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			rows.Close()
			return nil, err
		}
		remaining[id] = true
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	if len(input.IDs) != len(remaining) {
		return nil, ValidationError{"set_ids": "Order must include every saved set exactly once."}
	}
	for _, id := range input.IDs {
		if !remaining[id] {
			return nil, ValidationError{"set_ids": "Order must include every saved set exactly once."}
		}
		delete(remaining, id)
	}
	for index, id := range input.IDs {
		if _, err = tx.Exec(ctx, "UPDATE session_sets SET set_order=$4 WHERE owner_id=$1 AND session_id=$2 AND id=$3", owner, sessionID, id, index+1); err != nil {
			return nil, err
		}
	}
	all, err := readSets(ctx, tx, owner, sessionID)
	if err != nil {
		return nil, err
	}
	items := []SavedSet{}
	for _, set := range all {
		if set.Position == input.Position {
			items = append(items, set)
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	return items, nil
}
