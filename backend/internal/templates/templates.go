package templates

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/agersk0v/crow-fitness/backend/internal/exercises"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Decimal preserves optional RIR without rounding or an arbitrary product limit.
type Decimal string

var numberPattern = regexp.MustCompile(`^-?(0|[1-9][0-9]*)(\.[0-9]+)?([eE][+-]?[0-9]+)?$`)

func (d *Decimal) UnmarshalJSON(data []byte) error {
	if !numberPattern.Match(data) {
		return errors.New("RIR must be a number")
	}
	*d = Decimal(data)
	return nil
}
func (d Decimal) MarshalJSON() ([]byte, error) { return []byte(d), nil }

type Entry struct {
	ExerciseID string   `json:"exercise_id"`
	Name       string   `json:"name,omitempty"`
	Position   int      `json:"position"`
	SetCount   int64    `json:"set_count"`
	RepMin     *int64   `json:"rep_min"`
	RepMax     *int64   `json:"rep_max"`
	TargetRIR  *Decimal `json:"target_rir"`
}
type InputEntry struct {
	ExerciseID string   `json:"exercise_id"`
	SetCount   int64    `json:"set_count"`
	RepMin     *int64   `json:"rep_min"`
	RepMax     *int64   `json:"rep_max"`
	TargetRIR  *Decimal `json:"target_rir"`
}
type Input struct {
	Name      string       `json:"name"`
	Exercises []InputEntry `json:"exercises"`
}
type Template struct {
	ID        string  `json:"id"`
	Name      string  `json:"name"`
	Exercises []Entry `json:"exercises"`
}
type Summary struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}
type ValidationError map[string]string

func (v ValidationError) Error() string { return "Check the template values." }

var ErrNotFound = errors.New("template not found")

type Store interface {
	Create(context.Context, int64, Input) (Template, error)
	List(context.Context, int64) ([]Summary, error)
	Get(context.Context, int64, string) (Template, error)
}

func Validate(input *Input) ValidationError {
	input.Name = strings.TrimSpace(input.Name)
	fields := ValidationError{}
	if input.Name == "" {
		fields["name"] = "Enter a template name."
	}
	if len(input.Exercises) == 0 {
		fields["exercises"] = "Add at least one exercise."
	}
	for i, entry := range input.Exercises {
		key := fmt.Sprintf("exercises.%d.", i)
		if !validID(entry.ExerciseID) {
			fields[key+"exercise_id"] = "Choose a saved exercise."
		}
		if entry.SetCount <= 0 {
			fields[key+"set_count"] = "Enter a positive whole-number working-set count."
		}
		if (entry.RepMin == nil) != (entry.RepMax == nil) {
			fields[key+"rep_range"] = "Enter both rep bounds, or leave both blank."
		} else if entry.RepMin != nil && (*entry.RepMin <= 0 || *entry.RepMax < *entry.RepMin) {
			fields[key+"rep_range"] = "Use positive whole numbers with minimum no greater than maximum."
		}
		if entry.TargetRIR != nil {
			value := string(*entry.TargetRIR)
			mantissa := strings.Split(strings.ToLower(value), "e")[0]
			negative := strings.HasPrefix(value, "-") && strings.Trim(mantissa, "-0.") != ""
			if !numberPattern.MatchString(value) || negative {
				fields[key+"target_rir"] = "Enter a nonnegative target RIR, or leave it blank."
			}
		}
	}
	return fields
}
func validID(id string) bool {
	n, err := strconv.ParseInt(id, 10, 64)
	return err == nil && n > 0 && strconv.FormatInt(n, 10) == id
}

type Postgres struct{ Pool *pgxpool.Pool }

func (p Postgres) Create(ctx context.Context, owner int64, input Input) (Template, error) {
	if fields := Validate(&input); len(fields) > 0 {
		return Template{}, fields
	}
	tx, err := p.Pool.Begin(ctx)
	if err != nil {
		return Template{}, err
	}
	defer tx.Rollback(ctx)
	// Validate and lock selected records before inserting anything.
	for i, entry := range input.Exercises {
		var id string
		err = tx.QueryRow(ctx, "SELECT id::text FROM exercises WHERE owner_id=$1 AND id=$2 FOR KEY SHARE", owner, entry.ExerciseID).Scan(&id)
		if errors.Is(err, pgx.ErrNoRows) {
			return Template{}, ValidationError{fmt.Sprintf("exercises.%d.exercise_id", i): "This exercise is unavailable. Choose one from your library."}
		}
		if err != nil {
			return Template{}, err
		}
	}
	var item Template
	if err = tx.QueryRow(ctx, "INSERT INTO templates(owner_id,name) VALUES ($1,$2) RETURNING id::text,name", owner, input.Name).Scan(&item.ID, &item.Name); err != nil {
		return Template{}, err
	}
	for i, entry := range input.Exercises {
		var rir any
		if entry.TargetRIR != nil {
			rir = string(*entry.TargetRIR)
		}
		_, err = tx.Exec(ctx, `INSERT INTO template_exercises(template_id,owner_id,position,exercise_id,set_count,rep_min,rep_max,target_rir) VALUES ($1,$2,$3,$4,$5,$6,$7,$8::text::numeric)`, item.ID, owner, i+1, entry.ExerciseID, entry.SetCount, entry.RepMin, entry.RepMax, rir)
		if err != nil {
			return Template{}, err
		}
	}
	item, err = get(ctx, tx, owner, item.ID)
	if err != nil {
		return Template{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return Template{}, err
	}
	return item, nil
}
func (p Postgres) List(ctx context.Context, owner int64) ([]Summary, error) {
	rows, err := p.Pool.Query(ctx, "SELECT id::text,name FROM templates WHERE owner_id=$1 ORDER BY id", owner)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []Summary{}
	for rows.Next() {
		var item Summary
		if err = rows.Scan(&item.ID, &item.Name); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

type queryer interface {
	QueryRow(context.Context, string, ...any) pgx.Row
	Query(context.Context, string, ...any) (pgx.Rows, error)
}

func (p Postgres) Get(ctx context.Context, owner int64, id string) (Template, error) {
	return get(ctx, p.Pool, owner, id)
}
func get(ctx context.Context, db queryer, owner int64, id string) (Template, error) {
	var item Template
	err := db.QueryRow(ctx, "SELECT id::text,name FROM templates WHERE owner_id=$1 AND id=$2", owner, id).Scan(&item.ID, &item.Name)
	if errors.Is(err, pgx.ErrNoRows) {
		return item, ErrNotFound
	}
	if err != nil {
		return item, err
	}
	rows, err := db.Query(ctx, `SELECT e.id::text,e.name,te.position,te.set_count,te.rep_min,te.rep_max,te.target_rir::text FROM template_exercises te JOIN exercises e ON e.id=te.exercise_id AND e.owner_id=te.owner_id WHERE te.template_id=$1 AND te.owner_id=$2 ORDER BY te.position`, id, owner)
	if err != nil {
		return item, err
	}
	defer rows.Close()
	item.Exercises = []Entry{}
	for rows.Next() {
		var entry Entry
		var rir *string
		if err = rows.Scan(&entry.ExerciseID, &entry.Name, &entry.Position, &entry.SetCount, &entry.RepMin, &entry.RepMax, &rir); err != nil {
			return item, err
		}
		if rir != nil {
			v := Decimal(*rir)
			entry.TargetRIR = &v
		}
		item.Exercises = append(item.Exercises, entry)
	}
	return item, rows.Err()
}
func Register(mux *http.ServeMux, store Store) {
	mux.HandleFunc("GET /api/templates", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
		defer cancel()
		items, err := store.List(ctx, exercises.LocalOwnerID)
		if err != nil {
			failure(w, err)
			return
		}
		if items == nil {
			items = []Summary{}
		}
		write(w, 200, map[string]any{"items": items})
	})
	mux.HandleFunc("GET /api/templates/{id}", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if !validID(id) {
			problem(w, 400, "validation_error", "Invalid template ID.", nil)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
		defer cancel()
		item, err := store.Get(ctx, exercises.LocalOwnerID, id)
		if errors.Is(err, ErrNotFound) {
			problem(w, 404, "not_found", "Template not found.", nil)
			return
		}
		if err != nil {
			failure(w, err)
			return
		}
		write(w, 200, item)
	})
	mux.HandleFunc("POST /api/templates", func(w http.ResponseWriter, r *http.Request) {
		media, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
		if err != nil || media != "application/json" {
			problem(w, 415, "unsupported_media_type", "Use application/json.", nil)
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, 65536)
		decoder := json.NewDecoder(r.Body)
		decoder.DisallowUnknownFields()
		var input Input
		err = decoder.Decode(&input)
		if err == nil {
			var extra any
			next := decoder.Decode(&extra)
			if next != io.EOF {
				if next == nil {
					err = errors.New("extra JSON")
				} else {
					err = next
				}
			}
		}
		if err != nil {
			var oversized *http.MaxBytesError
			if errors.As(err, &oversized) {
				problem(w, 413, "body_too_large", "Request is too large.", nil)
			} else {
				problem(w, 400, "validation_error", "Send a template name and exercises with whole-number counts and numeric targets.", nil)
			}
			return
		}
		if fields := Validate(&input); len(fields) > 0 {
			problem(w, 400, "validation_error", fields.Error(), fields)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
		defer cancel()
		item, err := store.Create(ctx, exercises.LocalOwnerID, input)
		var fields ValidationError
		if errors.As(err, &fields) {
			problem(w, 400, "validation_error", fields.Error(), fields)
			return
		}
		if err != nil {
			failure(w, err)
			return
		}
		w.Header().Set("Location", "/api/templates/"+item.ID)
		write(w, 201, item)
	})
}
func failure(w http.ResponseWriter, err error) {
	slog.Error("Template persistence failed", "error", err)
	problem(w, 500, "internal_error", "Templates could not be saved or loaded. Please try again.", nil)
}
func problem(w http.ResponseWriter, status int, code, message string, fields any) {
	write(w, status, map[string]any{"error": map[string]any{"code": code, "message": message, "fields": fields}})
}
func write(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
