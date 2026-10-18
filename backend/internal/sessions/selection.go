package sessions

import (
	"context"
	"errors"

	"github.com/agersk0v/crow-fitness/backend/internal/templates"
	"github.com/jackc/pgx/v5"
)

var ErrSelection = errors.New("exercise selection changed")
var ErrHasSets = errors.New("exercise has saved sets")

type SelectionInput struct {
	Position   int64  `json:"exercise_position"`
	ExerciseID string `json:"exercise_id"`
	Revision   *int64 `json:"selection_revision"`
}

func captureChoices(ctx context.Context, tx pgx.Tx, owner int64, id, templateID string) error {
	_, err := tx.Exec(ctx, `INSERT INTO session_exercise_choices(session_id,owner_id,position,choice_position,exercise_id,name,set_count,rep_min,rep_max,target_rir)
 SELECT session_id,owner_id,position,0,exercise_id,name,set_count,rep_min,rep_max,target_rir FROM session_exercises WHERE owner_id=$1 AND session_id=$2
 UNION ALL
 SELECT $2,a.owner_id,a.position,a.choice_position,a.exercise_id,e.name,a.set_count,a.rep_min,a.rep_max,a.target_rir FROM template_exercise_alternatives a JOIN exercises e ON e.owner_id=a.owner_id AND e.id=a.exercise_id WHERE a.owner_id=$1 AND a.template_id=$3`, owner, id, templateID)
	return err
}
func readChoices(ctx context.Context, tx pgx.Tx, owner int64, id string, entries []templates.Entry) error {
	rows, err := tx.Query(ctx, `SELECT position,exercise_id::text,name,set_count,rep_min,rep_max,target_rir::text FROM session_exercise_choices WHERE owner_id=$1 AND session_id=$2 ORDER BY position,choice_position`, owner, id)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var pos int
		var c templates.Choice
		var rir *string
		if err = rows.Scan(&pos, &c.ExerciseID, &c.Name, &c.SetCount, &c.RepMin, &c.RepMax, &rir); err != nil {
			return err
		}
		if rir != nil {
			v := templates.Decimal(*rir)
			c.TargetRIR = &v
		}
		for i := range entries {
			if entries[i].Position == pos {
				entries[i].Choices = append(entries[i].Choices, c)
				break
			}
		}
	}
	for i := range entries {
		if len(entries[i].Choices) < 2 {
			entries[i].Choices = nil
		}
	}
	return rows.Err()
}

// Called only under the parent session lock. Missing revisions remain compatible
// with older single-choice clients; multi-choice slots always require a revision.
func checkSelection(ctx context.Context, tx pgx.Tx, owner int64, id string, position int64, revision *int64) error {
	var current int64
	var multiple bool
	err := tx.QueryRow(ctx, `SELECT selection_revision,(SELECT count(*)>1 FROM session_exercise_choices c WHERE c.owner_id=$1 AND c.session_id=$2 AND c.position=$3) FROM session_exercises WHERE owner_id=$1 AND session_id=$2 AND position=$3`, owner, id, position).Scan(&current, &multiple)
	if errors.Is(err, pgx.ErrNoRows) {
		return ValidationError{"exercise_position": "This exercise is not in the active workout."}
	}
	if err != nil {
		return err
	}
	if (revision == nil && (multiple || current != 0)) || (revision != nil && *revision != current) {
		return ErrSelection
	}
	return nil
}
func (p Postgres) SelectExercise(ctx context.Context, owner int64, id string, input SelectionInput) (Session, error) {
	if input.Position <= 0 || !validID(input.ExerciseID) || input.Revision == nil || *input.Revision < 0 {
		return Session{}, ValidationError{"selection": "Choose a captured exercise and its current selection revision."}
	}
	tx, err := p.Pool.Begin(ctx)
	if err != nil {
		return Session{}, err
	}
	defer tx.Rollback(ctx)
	if err = lockSession(ctx, tx, owner, id); err != nil {
		return Session{}, err
	}
	if err = checkSelection(ctx, tx, owner, id, input.Position, input.Revision); err != nil {
		return Session{}, err
	}
	var hasSets bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM session_sets WHERE owner_id=$1 AND session_id=$2 AND exercise_position=$3 AND NOT deleted)`, owner, id, input.Position).Scan(&hasSets); err != nil {
		return Session{}, err
	}
	if hasSets {
		return Session{}, ErrHasSets
	}
	result, err := tx.Exec(ctx, `UPDATE session_exercises e SET exercise_id=c.exercise_id,name=c.name,set_count=c.set_count,rep_min=c.rep_min,rep_max=c.rep_max,target_rir=c.target_rir,selection_revision=e.selection_revision+1
 FROM session_exercise_choices c WHERE e.owner_id=$1 AND e.session_id=$2 AND e.position=$3 AND c.owner_id=e.owner_id AND c.session_id=e.session_id AND c.position=e.position AND c.exercise_id=$4`, owner, id, input.Position, input.ExerciseID)
	if err != nil {
		return Session{}, err
	}
	if result.RowsAffected() != 1 {
		return Session{}, ValidationError{"exercise_id": "Choose an exercise captured for this slot."}
	}
	item, err := get(ctx, tx, owner, id)
	if err != nil {
		return Session{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return Session{}, err
	}
	return item, nil
}
