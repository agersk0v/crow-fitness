package sessions

import (
	"context"
	"errors"
	"time"

	"github.com/agersk0v/crow-fitness/backend/internal/templates"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Session struct {
	Sets        []SavedSet        `json:"sets"`
	ID          string            `json:"id"`
	TemplateID  string            `json:"template_id"`
	Name        string            `json:"name"`
	CompletedAt *time.Time        `json:"completed_at"`
	StartedAt   time.Time         `json:"started_at"`
	Exercises   []templates.Entry `json:"exercises"`
}

var (
	ErrNotFound = errors.New("session or template not found")
	ErrArchived = errors.New("template archived")
	ErrEmpty    = errors.New("template has no exercises")
)

type ActiveConflict struct{ Session Session }

func (e ActiveConflict) Error() string { return "Continue the active session before starting another." }

type Store interface {
	ExerciseProgress(context.Context, int64, HistoryFilter) ([]ProgressPoint, error)
	SelectExercise(context.Context, int64, string, SelectionInput) (Session, error)
	Start(context.Context, int64, string) (Session, error)
	Active(context.Context, int64) (*Session, error)
	Get(context.Context, int64, string) (Session, error)
	End(context.Context, int64, string) error
	Complete(context.Context, int64, string) error
	SaveSet(context.Context, int64, string, string, SetInput) (SavedSet, error)
	DeleteSet(context.Context, int64, string, string) error
	ReorderSets(context.Context, int64, string, SetOrderInput) ([]SavedSet, error)
	CompletedSets(context.Context, int64, HistoryFilter) ([]CompletedSet, error)
	Previous(context.Context, int64, string) ([]PreviousPerformance, error)
	PersonalBests(context.Context, int64, string) ([]PersonalBest, error)
	CompletedSessions(context.Context, int64, HistoryFilter) ([]CompletedSession, error)
	HistoryExercises(context.Context, int64) ([]HistoryExercise, error)
	HistoryTemplates(context.Context, int64) ([]HistoryExercise, error)
	GetCompleted(context.Context, int64, string) (Session, error)
}
type Postgres struct{ Pool *pgxpool.Pool }

func (p Postgres) Start(ctx context.Context, owner int64, templateID string) (Session, error) {
	tx, err := p.Pool.Begin(ctx)
	if err != nil {
		return Session{}, err
	}
	defer tx.Rollback(ctx)
	// A stable owner row serializes starts and endings, including when no session exists.
	if err = lockOwner(ctx, tx, owner); err != nil {
		return Session{}, err
	}
	var currentID string
	err = tx.QueryRow(ctx, "SELECT id::text FROM workout_sessions WHERE owner_id=$1 AND completed_at IS NULL", owner).Scan(&currentID)
	if err == nil {
		current, readErr := get(ctx, tx, owner, currentID)
		if readErr != nil {
			return Session{}, readErr
		}
		return Session{}, ActiveConflict{Session: current}
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return Session{}, err
	}
	var name string
	var archived bool
	// Template replacement/archiving locks this same parent row before any changes.
	err = tx.QueryRow(ctx, "SELECT name,archived FROM templates WHERE owner_id=$1 AND id=$2 FOR SHARE", owner, templateID).Scan(&name, &archived)
	if errors.Is(err, pgx.ErrNoRows) {
		return Session{}, ErrNotFound
	}
	if err != nil {
		return Session{}, err
	}
	if archived {
		return Session{}, ErrArchived
	}
	var id string
	if err = tx.QueryRow(ctx, "INSERT INTO workout_sessions(owner_id,template_id,name) VALUES ($1,$2,$3) RETURNING id::text", owner, templateID, name).Scan(&id); err != nil {
		return Session{}, err
	}
	// Copy all names/targets in a single statement, preserving numeric precision and nulls.
	copied, err := tx.Exec(ctx, `INSERT INTO session_exercises(session_id,owner_id,position,exercise_id,name,set_count,rep_min,rep_max,target_rir)
 SELECT $1,te.owner_id,te.position,te.exercise_id,e.name,te.set_count,te.rep_min,te.rep_max,te.target_rir
 FROM template_exercises te JOIN exercises e ON e.owner_id=te.owner_id AND e.id=te.exercise_id
 WHERE te.owner_id=$2 AND te.template_id=$3`, id, owner, templateID)
	if err != nil {
		return Session{}, err
	}
	if copied.RowsAffected() == 0 {
		return Session{}, ErrEmpty
	}
	if err = captureChoices(ctx, tx, owner, id, templateID); err != nil {
		return Session{}, err
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
func lockOwner(ctx context.Context, tx pgx.Tx, owner int64) error {
	// NO KEY UPDATE serializes session operations without blocking owner FK
	// checks from template creation/duplication.
	var id int64
	err := tx.QueryRow(ctx, "SELECT id FROM owners WHERE id=$1 FOR NO KEY UPDATE", owner).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	return err
}
func (p Postgres) read(ctx context.Context, owner int64, id string) (Session, error) {
	return p.readState(ctx, owner, id, false)
}
func (p Postgres) readState(ctx context.Context, owner int64, id string, completed bool) (Session, error) {
	tx, err := p.Pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return Session{}, err
	}
	defer tx.Rollback(ctx)
	item, err := getSnapshot(ctx, tx, owner, id, completed)
	if err != nil {
		return Session{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return Session{}, err
	}
	return item, nil
}
func (p Postgres) Active(ctx context.Context, owner int64) (*Session, error) {
	item, err := p.read(ctx, owner, "")
	if errors.Is(err, ErrNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &item, nil
}
func (p Postgres) Get(ctx context.Context, owner int64, id string) (Session, error) {
	return p.read(ctx, owner, id)
}
func get(ctx context.Context, tx pgx.Tx, owner int64, id string) (Session, error) {
	return getSnapshot(ctx, tx, owner, id, false)
}
func getSnapshot(ctx context.Context, tx pgx.Tx, owner int64, id string, completed bool) (Session, error) {
	var item Session
	err := tx.QueryRow(ctx, `SELECT id::text,template_id::text,name,started_at,completed_at FROM workout_sessions
 WHERE owner_id=$1 AND (completed_at IS NOT NULL)=$3 AND ($2='' OR id::text=$2)`, owner, id, completed).Scan(&item.ID, &item.TemplateID, &item.Name, &item.StartedAt, &item.CompletedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return item, ErrNotFound
	}
	if err != nil {
		return item, err
	}
	rows, err := tx.Query(ctx, `SELECT exercise_id::text,name,position,set_count,rep_min,rep_max,target_rir::text,selection_revision
 FROM session_exercises WHERE owner_id=$1 AND session_id=$2 ORDER BY position`, owner, item.ID)
	if err != nil {
		return item, err
	}
	defer rows.Close()
	item.Exercises = []templates.Entry{}
	for rows.Next() {
		var entry templates.Entry
		var rir *string
		if err = rows.Scan(&entry.ExerciseID, &entry.Name, &entry.Position, &entry.SetCount, &entry.RepMin, &entry.RepMax, &rir, &entry.SelectionRevision); err != nil {
			return item, err
		}
		if rir != nil {
			value := templates.Decimal(*rir)
			entry.TargetRIR = &value
		}
		item.Exercises = append(item.Exercises, entry)
	}
	if err = rows.Err(); err != nil {
		return item, err
	}
	rows.Close()
	if err = readChoices(ctx, tx, owner, item.ID, item.Exercises); err != nil {
		return item, err
	}
	if completed {
		item.Sets, err = readCompletedSets(ctx, tx, owner, item.ID)
	} else {
		item.Sets, err = readSets(ctx, tx, owner, item.ID)
	}
	return item, err
}
func (p Postgres) End(ctx context.Context, owner int64, id string) error {
	tx, err := p.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err = lockOwner(ctx, tx, owner); err != nil {
		return err
	}
	// Exact ID makes retries harmless and stale tabs unable to clear a newer workout.
	if _, err = tx.Exec(ctx, "DELETE FROM workout_sessions WHERE owner_id=$1 AND id=$2 AND completed_at IS NULL", owner, id); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
