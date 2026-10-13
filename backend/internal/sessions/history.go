package sessions

import (
	"context"
	"time"
)

type CompletedSession struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	CompletedAt time.Time `json:"completed_at"`
}

func (p Postgres) CompletedSessions(ctx context.Context, owner int64) ([]CompletedSession, error) {
	rows, err := p.Pool.Query(ctx, "SELECT id::text,name,completed_at FROM workout_sessions WHERE owner_id=$1 AND completed_at IS NOT NULL ORDER BY completed_at DESC,id DESC", owner)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []CompletedSession{}
	for rows.Next() {
		var item CompletedSession
		if err = rows.Scan(&item.ID, &item.Name, &item.CompletedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}
func (p Postgres) GetCompleted(ctx context.Context, owner int64, id string) (Session, error) {
	return p.readState(ctx, owner, id, true)
}
