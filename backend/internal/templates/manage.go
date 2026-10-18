package templates

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
)

var ErrArchived = errors.New("template archived")

func lockTemplate(ctx context.Context, tx pgx.Tx, owner int64, id string) (bool, error) {
	var archived bool
	err := tx.QueryRow(ctx, "SELECT archived FROM templates WHERE owner_id=$1 AND id=$2 FOR UPDATE", owner, id).Scan(&archived)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, ErrNotFound
	}
	return archived, err
}
func lockExercises(ctx context.Context, tx pgx.Tx, owner int64, input Input) error {
	for i, entry := range input.Exercises {
		choices := append([]Choice{{ExerciseID: entry.ExerciseID}}, entry.Alternatives...)
		for j, choice := range choices {
			var id string
			err := tx.QueryRow(ctx, "SELECT id::text FROM exercises WHERE owner_id=$1 AND id=$2 FOR KEY SHARE", owner, choice.ExerciseID).Scan(&id)
			if errors.Is(err, pgx.ErrNoRows) {
				key := fmt.Sprintf("exercises.%d.exercise_id", i)
				if j > 0 {
					key = fmt.Sprintf("exercises.%d.alternatives.%d.exercise_id", i, j-1)
				}
				return ValidationError{key: "This exercise is unavailable. Choose one from your library."}
			}
			if err != nil {
				return err
			}
		}
	}
	return nil
}
func insertEntries(ctx context.Context, tx pgx.Tx, owner int64, id string, entries []InputEntry) error {
	for i, entry := range entries {
		var rir any
		if entry.TargetRIR != nil {
			rir = string(*entry.TargetRIR)
		}
		if _, err := tx.Exec(ctx, `INSERT INTO template_exercises(template_id,owner_id,position,exercise_id,set_count,rep_min,rep_max,target_rir) VALUES($1,$2,$3,$4,$5,$6,$7,$8::text::numeric)`, id, owner, i+1, entry.ExerciseID, entry.SetCount, entry.RepMin, entry.RepMax, rir); err != nil {
			return err
		}
		for j, c := range entry.Alternatives {
			var target any
			if c.TargetRIR != nil {
				target = string(*c.TargetRIR)
			}
			if _, err := tx.Exec(ctx, `INSERT INTO template_exercise_alternatives(template_id,owner_id,position,choice_position,exercise_id,set_count,rep_min,rep_max,target_rir) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9::text::numeric)`, id, owner, i+1, j+1, c.ExerciseID, c.SetCount, c.RepMin, c.RepMax, target); err != nil {
				return err
			}
		}
	}
	return nil
}
func (p Postgres) Update(ctx context.Context, owner int64, id string, input Input) (Template, error) {
	if fields := Validate(&input); len(fields) > 0 {
		return Template{}, fields
	}
	tx, err := p.Pool.Begin(ctx)
	if err != nil {
		return Template{}, err
	}
	defer tx.Rollback(ctx)
	archived, err := lockTemplate(ctx, tx, owner, id)
	if err != nil {
		return Template{}, err
	}
	if archived {
		return Template{}, ErrArchived
	}
	if err = lockExercises(ctx, tx, owner, input); err != nil {
		return Template{}, err
	}
	if _, err = tx.Exec(ctx, "UPDATE templates SET name=$3,description=$4 WHERE owner_id=$1 AND id=$2", owner, id, input.Name, input.Description); err != nil {
		return Template{}, err
	}
	if _, err = tx.Exec(ctx, "DELETE FROM template_exercises WHERE owner_id=$1 AND template_id=$2", owner, id); err != nil {
		return Template{}, err
	}
	if err = insertEntries(ctx, tx, owner, id, input.Exercises); err != nil {
		return Template{}, err
	}
	item, err := get(ctx, tx, owner, id)
	if err != nil {
		return Template{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return Template{}, err
	}
	return item, nil
}
func (p Postgres) Duplicate(ctx context.Context, owner int64, id, name string) (Template, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return Template{}, ValidationError{"name": "Enter a template name."}
	}
	tx, err := p.Pool.Begin(ctx)
	if err != nil {
		return Template{}, err
	}
	defer tx.Rollback(ctx)
	archived, err := lockTemplate(ctx, tx, owner, id)
	if err != nil {
		return Template{}, err
	}
	if archived {
		return Template{}, ErrArchived
	}
	var copyID string
	if err = tx.QueryRow(ctx, "INSERT INTO templates(owner_id,name,description) SELECT owner_id,$3,description FROM templates WHERE owner_id=$1 AND id=$2 RETURNING id::text", owner, id, name).Scan(&copyID); err != nil {
		return Template{}, err
	}
	// Copy numeric values in PostgreSQL without client-side rounding. The parent
	// lock guarantees all occurrences come from the same committed configuration.
	if _, err = tx.Exec(ctx, `INSERT INTO template_exercises(template_id,owner_id,position,exercise_id,set_count,rep_min,rep_max,target_rir) SELECT $3,owner_id,position,exercise_id,set_count,rep_min,rep_max,target_rir FROM template_exercises WHERE owner_id=$1 AND template_id=$2`, owner, id, copyID); err != nil {
		return Template{}, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO template_exercise_alternatives(template_id,owner_id,position,choice_position,exercise_id,set_count,rep_min,rep_max,target_rir) SELECT $3,owner_id,position,choice_position,exercise_id,set_count,rep_min,rep_max,target_rir FROM template_exercise_alternatives WHERE owner_id=$1 AND template_id=$2`, owner, id, copyID); err != nil {
		return Template{}, err
	}
	item, err := get(ctx, tx, owner, copyID)
	if err != nil {
		return Template{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return Template{}, err
	}
	return item, nil
}
func (p Postgres) SetArchived(ctx context.Context, owner int64, id string, archived bool) (Template, error) {
	tx, err := p.Pool.Begin(ctx)
	if err != nil {
		return Template{}, err
	}
	defer tx.Rollback(ctx)
	current, err := lockTemplate(ctx, tx, owner, id)
	if err != nil {
		return Template{}, err
	}
	if current != archived {
		if _, err = tx.Exec(ctx, "UPDATE templates SET archived=$3 WHERE owner_id=$1 AND id=$2", owner, id, archived); err != nil {
			return Template{}, err
		}
	}
	item, err := get(ctx, tx, owner, id)
	if err != nil {
		return Template{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return Template{}, err
	}
	return item, nil
}
