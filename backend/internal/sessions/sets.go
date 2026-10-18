package sessions

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"time"

	"github.com/agersk0v/crow-fitness/backend/internal/templates"
	"github.com/jackc/pgx/v5"
)

type SetInput struct {
	SelectionRevision *int64             `json:"selection_revision"`
	PartialReps       *int64             `json:"partial_reps"`
	Technique         string             `json:"technique"`
	RIR               *templates.Decimal `json:"rir"`
	Position          int64              `json:"exercise_position"`
	Weight            string             `json:"weight"`
	Reps              int64              `json:"reps"`
}
type SavedSet struct {
	PartialReps *int64     `json:"partial_reps"`
	Technique   string     `json:"technique"`
	RIR         *string    `json:"rir"`
	Change      *SetChange `json:"change,omitempty"`
	ID          string     `json:"id"`
	Position    int64      `json:"exercise_position"`
	Weight      string     `json:"weight"`
	Reps        int64      `json:"reps"`
}
type CompletedSet struct {
	SavedSet
	SessionName  string    `json:"session_name"`
	ExerciseID   string    `json:"exercise_id"`
	SessionID    string    `json:"session_id"`
	ExerciseName string    `json:"exercise_name"`
	CompletedAt  time.Time `json:"completed_at"`
}
type ValidationError map[string]string

func (e ValidationError) Error() string { return "Check the set values." }

var ErrCompleted = errors.New("session is completed")
var ErrDeleted = errors.New("set has been deleted")
var ErrOccurrence = errors.New("set belongs to another exercise occurrence")
var weightPattern = regexp.MustCompile(`^(0|[1-9][0-9]*)(\.[0-9]{1,2})?$`)
var rirPattern = regexp.MustCompile(`^-?(0|[1-9][0-9]*)(\.[0-9]+)?([eE][+-]?[0-9]+)?$`)
var uuidPattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

func ValidateSet(input SetInput) ValidationError {
	fields := ValidationError{}
	if input.PartialReps != nil && (*input.PartialReps < 0 || *input.PartialReps > 1<<53-1) {
		fields["partial_reps"] = "Enter a nonnegative whole-number partial-rep count, or leave it blank."
	}
	if input.Technique != "good" && input.Technique != "decent" && input.Technique != "bad" {
		fields["technique"] = "Choose Good, Decent, or Bad technique."
	}
	if input.Position <= 0 {
		fields["exercise_position"] = "Choose an exercise in this workout."
	}
	if !weightPattern.MatchString(input.Weight) {
		fields["weight"] = "Enter nonnegative kilograms with at most two decimals."
	}
	if input.Reps <= 0 {
		fields["reps"] = "Enter a positive whole-number rep count."
	}
	if input.RIR != nil {
		value := string(*input.RIR)
		mantissa := strings.Split(strings.ToLower(value), "e")[0]
		negative := strings.HasPrefix(value, "-") && strings.Trim(mantissa, "-0.") != ""
		if !rirPattern.MatchString(value) || negative {
			fields["rir"] = "Enter a nonnegative RIR, or leave it blank."
		}
	}
	return fields
}
func lockSession(ctx context.Context, tx pgx.Tx, owner int64, id string) error {
	var completed *time.Time
	err := tx.QueryRow(ctx, "SELECT completed_at FROM workout_sessions WHERE owner_id=$1 AND id=$2 FOR UPDATE", owner, id).Scan(&completed)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	if completed != nil {
		return ErrCompleted
	}
	return nil
}
func (p Postgres) Complete(ctx context.Context, owner int64, id string) error {
	tx, err := p.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err = lockOwner(ctx, tx, owner); err != nil {
		return err
	}
	// The parent row lock orders completion against every set save/delete.
	if _, err = tx.Exec(ctx, "UPDATE workout_sessions SET completed_at=GREATEST(clock_timestamp(),started_at) WHERE owner_id=$1 AND id=$2 AND completed_at IS NULL", owner, id); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
func (p Postgres) SaveSet(ctx context.Context, owner int64, sessionID, id string, input SetInput) (SavedSet, error) {
	if fields := ValidateSet(input); len(fields) > 0 {
		return SavedSet{}, fields
	}
	if !uuidPattern.MatchString(id) {
		return SavedSet{}, ValidationError{"id": "Invalid set ID."}
	}
	tx, err := p.Pool.Begin(ctx)
	if err != nil {
		return SavedSet{}, err
	}
	defer tx.Rollback(ctx)
	if err = lockSession(ctx, tx, owner, sessionID); err != nil {
		return SavedSet{}, err
	}
	if err = checkSelection(ctx, tx, owner, sessionID, input.Position, input.SelectionRevision); err != nil {
		return SavedSet{}, err
	}
	var existingPosition int64
	var deleted bool
	err = tx.QueryRow(ctx, "SELECT exercise_position,deleted FROM session_sets WHERE owner_id=$1 AND session_id=$2 AND id=$3", owner, sessionID, id).Scan(&existingPosition, &deleted)
	isNew := errors.Is(err, pgx.ErrNoRows)
	if err == nil {
		if deleted {
			return SavedSet{}, ErrDeleted
		}
		if existingPosition != input.Position {
			return SavedSet{}, ErrOccurrence
		}
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return SavedSet{}, err
	}
	var exists bool
	if err = tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM session_exercises WHERE owner_id=$1 AND session_id=$2 AND position=$3)", owner, sessionID, input.Position).Scan(&exists); err != nil {
		return SavedSet{}, err
	}
	if !exists {
		return SavedSet{}, ValidationError{"exercise_position": "This exercise is not in the active workout."}
	}
	if isNew {
		var room bool
		if err = tx.QueryRow(ctx, `SELECT (SELECT count(*) FROM session_sets WHERE owner_id=$1 AND session_id=$2 AND exercise_position=$3 AND NOT deleted)<set_count FROM session_exercises WHERE owner_id=$1 AND session_id=$2 AND position=$3`, owner, sessionID, input.Position).Scan(&room); err != nil {
			return SavedSet{}, err
		}
		if !room {
			return SavedSet{}, ValidationError{"set_count": "All planned sets for this exercise are saved. Refresh to see the latest sets."}
		}
	}
	var item SavedSet
	partials := input.PartialReps
	if partials != nil && *partials == 0 {
		partials = nil
	}
	var rir any
	if input.RIR != nil {
		rir = string(*input.RIR)
	}
	err = tx.QueryRow(ctx, `INSERT INTO session_sets(session_id,owner_id,id,exercise_position,weight,reps,rir,technique,partial_reps,set_order)
 VALUES($1,$2,$3,$4,$5::text::numeric,$6,$7::text::numeric,$8,$9,(SELECT COALESCE(max(set_order),0)+1 FROM session_sets WHERE session_id=$1 AND exercise_position=$4))
 ON CONFLICT(session_id,id) DO UPDATE SET weight=EXCLUDED.weight,reps=EXCLUDED.reps,rir=EXCLUDED.rir,technique=EXCLUDED.technique,partial_reps=EXCLUDED.partial_reps
 RETURNING id::text,exercise_position,weight::text,reps,rir::text,technique,partial_reps`, sessionID, owner, id, input.Position, input.Weight, input.Reps, rir, input.Technique, partials).Scan(&item.ID, &item.Position, &item.Weight, &item.Reps, &item.RIR, &item.Technique, &item.PartialReps)
	if err != nil {
		return item, err
	}
	if err = tx.Commit(ctx); err != nil {
		return item, err
	}
	return item, nil
}
func (p Postgres) DeleteSet(ctx context.Context, owner int64, sessionID, id string) error {
	tx, err := p.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err = lockSession(ctx, tx, owner, sessionID); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, "UPDATE session_sets SET deleted=true WHERE owner_id=$1 AND session_id=$2 AND id=$3", owner, sessionID, id); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
func readSets(ctx context.Context, tx pgx.Tx, owner int64, id string) ([]SavedSet, error) {
	rows, err := tx.Query(ctx, "SELECT id::text,exercise_position,weight::text,reps,rir::text,technique,partial_reps FROM session_sets WHERE owner_id=$1 AND session_id=$2 AND NOT deleted ORDER BY exercise_position,set_order,saved_at,id", owner, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []SavedSet{}
	for rows.Next() {
		var item SavedSet
		if err = rows.Scan(&item.ID, &item.Position, &item.Weight, &item.Reps, &item.RIR, &item.Technique, &item.PartialReps); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}
func (p Postgres) CompletedSets(ctx context.Context, owner int64, filter HistoryFilter) ([]CompletedSet, error) {
	rows, err := p.Pool.Query(ctx, completedSetComparisons+`SELECT ss.id::text,ss.exercise_position,ss.weight::text,ss.reps,s.id::text,se.name,s.completed_at,s.name,se.exercise_id::text,ss.weight_change,ss.reps_change,ss.rir::text,ss.technique,ss.previous_technique,ss.partial_reps
 FROM compared_sets ss JOIN workout_sessions s ON s.owner_id=ss.owner_id AND s.id=ss.session_id
 JOIN session_exercises se ON se.session_id=ss.session_id AND se.owner_id=ss.owner_id AND se.position=ss.exercise_position
 WHERE ss.owner_id=$1
`+completedSetFilter+`
 ORDER BY s.completed_at DESC,s.id DESC,ss.exercise_position,ss.set_order,ss.saved_at,ss.id`, owner, filter.Query, filter.Exercise, filter.From, filter.Until)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []CompletedSet{}
	for rows.Next() {
		var item CompletedSet
		var weight, reps, previousTechnique *string
		if err = rows.Scan(&item.ID, &item.Position, &item.Weight, &item.Reps, &item.SessionID, &item.ExerciseName, &item.CompletedAt, &item.SessionName, &item.ExerciseID, &weight, &reps, &item.RIR, &item.Technique, &previousTechnique, &item.PartialReps); err != nil {
			return nil, err
		}
		item.Change = setChange(weight, reps, item.Technique, previousTechnique)
		items = append(items, item)
	}
	return items, rows.Err()
}
