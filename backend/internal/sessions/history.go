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

func (p Postgres) CompletedSessions(ctx context.Context, owner int64, filter HistoryFilter) ([]CompletedSession, error) {
	rows, err := p.Pool.Query(ctx, `SELECT s.id::text,s.name,s.completed_at FROM workout_sessions s
 WHERE s.owner_id=$1 AND s.completed_at IS NOT NULL
 AND ($2::text='' OR strpos(lower(s.name),lower($2))>0 OR EXISTS(SELECT 1 FROM session_exercises e WHERE e.owner_id=s.owner_id AND e.session_id=s.id AND strpos(lower(e.name),lower($2))>0))
 AND ($3::text='' OR EXISTS(SELECT 1 FROM session_exercises e WHERE e.owner_id=s.owner_id AND e.session_id=s.id AND e.exercise_id::text=$3))
 AND ($4::timestamptz IS NULL OR s.completed_at >= $4)
 AND ($5::timestamptz IS NULL OR s.completed_at < $5)
 AND ($6::text='' OR s.template_id::text=$6)
 AND ($7::text='' OR s.id::text=$7)
 ORDER BY s.completed_at DESC,s.id DESC`, owner, filter.Query, filter.Exercise, filter.From, filter.Until, filter.Template, filter.Session)
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
