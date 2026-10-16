package exercises

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
)

var ErrNotFound = errors.New("exercise not found")

type TemplateReference struct {
	Archived bool   `json:"archived"`
	ID       string `json:"id"`
	Name     string `json:"name"`
}
type InUseError struct{ Templates []TemplateReference }

func (e *InUseError) Error() string {
	return "This exercise is used in templates and cannot be deleted."
}

type referenceQueryer interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
}

func references(ctx context.Context, db referenceQueryer, owner int64, id string) ([]TemplateReference, error) {
	rows, err := db.Query(ctx, `SELECT DISTINCT t.id::text,t.name,t.archived FROM templates t WHERE t.owner_id=$1 AND (EXISTS(SELECT 1 FROM template_exercises te WHERE te.owner_id=t.owner_id AND te.template_id=t.id AND te.exercise_id=$2) OR EXISTS(SELECT 1 FROM template_exercise_alternatives a WHERE a.owner_id=t.owner_id AND a.template_id=t.id AND a.exercise_id=$2)) ORDER BY t.id::text`, owner, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []TemplateReference{}
	for rows.Next() {
		var item TemplateReference
		if err = rows.Scan(&item.ID, &item.Name, &item.Archived); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}
func (p Postgres) Usage(ctx context.Context, owner int64, id string) ([]TemplateReference, error) {
	var exists bool
	if err := p.Pool.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM exercises WHERE owner_id=$1 AND id=$2)", owner, id).Scan(&exists); err != nil {
		return nil, err
	}
	if !exists {
		return nil, ErrNotFound
	}
	return references(ctx, p.Pool, owner, id)
}
func (p Postgres) Delete(ctx context.Context, owner int64, id string) error {
	tx, err := p.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var found string
	// Conflicts with template creation's FOR KEY SHARE lock. Either the template
	// commits first and blocks deletion, or deletion wins and creation rejects it.
	err = tx.QueryRow(ctx, "SELECT id::text FROM exercises WHERE owner_id=$1 AND id=$2 FOR UPDATE", owner, id).Scan(&found)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	used, err := references(ctx, tx, owner, id)
	if err != nil {
		return err
	}
	if len(used) > 0 {
		return &InUseError{Templates: used}
	}
	if _, err = tx.Exec(ctx, "DELETE FROM exercises WHERE owner_id=$1 AND id=$2", owner, id); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
