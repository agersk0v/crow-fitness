package exercises

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// LocalOwnerID is selected by the server, never from browser input.
const LocalOwnerID int64 = 1

var MuscleGroups = []string{"chest", "back", "shoulders", "biceps", "triceps", "forearms", "core", "glutes", "quadriceps", "hamstrings", "calves"}

type Exercise struct {
	Description  string   `json:"description"`
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	MuscleGroups []string `json:"muscle_groups"`
}
type Input struct {
	Description  string   `json:"description"`
	Name         string   `json:"name"`
	MuscleGroups []string `json:"muscle_groups"`
}
type ValidationError map[string]string

func (e ValidationError) Error() string { return "Check the exercise values." }
func Validate(input *Input) ValidationError {
	fields := ValidationError{}
	input.Name = strings.TrimSpace(input.Name)
	input.Description = strings.TrimSpace(input.Description)
	if input.Name == "" {
		fields["name"] = "Enter an exercise name."
	}
	selected := map[string]bool{}
	for _, tag := range input.MuscleGroups {
		selected[tag] = true
	}
	normalized := []string{}
	for _, tag := range MuscleGroups {
		if selected[tag] {
			normalized = append(normalized, tag)
			delete(selected, tag)
		}
	}
	if len(normalized) == 0 {
		fields["muscle_groups"] = "Select at least one muscle group."
	}
	if len(selected) > 0 {
		fields["muscle_groups"] = "Choose muscle groups from the predefined list."
	}
	input.MuscleGroups = normalized
	return fields
}

type Store interface {
	List(context.Context, int64) ([]Exercise, error)
	Create(context.Context, int64, Input) (Exercise, error)
	Update(context.Context, int64, string, Input) (Exercise, error)
	Usage(context.Context, int64, string) ([]TemplateReference, error)
	Delete(context.Context, int64, string) error
}
type Postgres struct{ Pool *pgxpool.Pool }

func (p Postgres) List(ctx context.Context, owner int64) ([]Exercise, error) {
	rows, err := p.Pool.Query(ctx, "SELECT id::text,name,muscle_groups,description FROM exercises WHERE owner_id=$1 ORDER BY id", owner)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []Exercise{}
	for rows.Next() {
		var item Exercise
		if err = rows.Scan(&item.ID, &item.Name, &item.MuscleGroups, &item.Description); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}
func (p Postgres) Create(ctx context.Context, owner int64, input Input) (Exercise, error) {
	if fields := Validate(&input); len(fields) > 0 {
		return Exercise{}, fields
	}
	var item Exercise
	err := p.Pool.QueryRow(ctx, "INSERT INTO exercises(owner_id,name,muscle_groups,description) VALUES ($1,$2,$3,$4) RETURNING id::text,name,muscle_groups,description", owner, input.Name, input.MuscleGroups, input.Description).Scan(&item.ID, &item.Name, &item.MuscleGroups, &item.Description)
	return item, err
}
func (p Postgres) Update(ctx context.Context, owner int64, id string, input Input) (Exercise, error) {
	if fields := Validate(&input); len(fields) > 0 {
		return Exercise{}, fields
	}
	var item Exercise
	err := p.Pool.QueryRow(ctx, "UPDATE exercises SET name=$3,muscle_groups=$4,description=$5 WHERE owner_id=$1 AND id=$2 RETURNING id::text,name,muscle_groups,description", owner, id, input.Name, input.MuscleGroups, input.Description).Scan(&item.ID, &item.Name, &item.MuscleGroups, &item.Description)
	if errors.Is(err, pgx.ErrNoRows) {
		return Exercise{}, ErrNotFound
	}
	return item, err
}
func Register(mux *http.ServeMux, store Store) {
	mux.HandleFunc("GET /api/exercises", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
		defer cancel()
		items, err := store.List(ctx, LocalOwnerID)
		if err != nil {
			operationError(w, err)
			return
		}
		if items == nil {
			items = []Exercise{}
		}
		write(w, 200, map[string]any{"items": items, "muscle_groups": MuscleGroups})
	})
	save := func(w http.ResponseWriter, r *http.Request) {
		updating := r.Method == http.MethodPut
		if updating && !validID(r.PathValue("id")) {
			problem(w, 400, "validation_error", "Invalid exercise ID.", nil)
			return
		}
		var input Input
		if !decode(w, r, &input) {
			return
		}
		if fields := Validate(&input); len(fields) > 0 {
			problem(w, 400, "validation_error", fields.Error(), fields)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
		defer cancel()
		var item Exercise
		var err error
		if updating {
			item, err = store.Update(ctx, LocalOwnerID, r.PathValue("id"), input)
		} else {
			item, err = store.Create(ctx, LocalOwnerID, input)
		}
		if err != nil {
			operationError(w, err)
			return
		}
		if updating {
			write(w, 200, item)
		} else {
			write(w, 201, item)
		}
	}
	mux.HandleFunc("POST /api/exercises", save)
	mux.HandleFunc("PUT /api/exercises/{id}", save)
	mux.HandleFunc("GET /api/exercises/{id}/templates", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if !validID(id) {
			problem(w, 400, "validation_error", "Invalid exercise ID.", nil)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
		defer cancel()
		items, err := store.Usage(ctx, LocalOwnerID, id)
		if err != nil {
			operationError(w, err)
			return
		}
		write(w, 200, map[string]any{"items": items})
	})
	mux.HandleFunc("DELETE /api/exercises/{id}", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if !validID(id) {
			problem(w, 400, "validation_error", "Invalid exercise ID.", nil)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
		defer cancel()
		if err := store.Delete(ctx, LocalOwnerID, id); err != nil {
			operationError(w, err)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		w.WriteHeader(http.StatusNoContent)
	})
}
func validID(id string) bool {
	n, err := strconv.ParseInt(id, 10, 64)
	return err == nil && n > 0 && strconv.FormatInt(n, 10) == id
}
func decode(w http.ResponseWriter, r *http.Request, input *Input) bool {
	media, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || media != "application/json" {
		problem(w, 415, "unsupported_media_type", "Use application/json.", nil)
		return false
	}
	r.Body = http.MaxBytesReader(w, r.Body, 16384)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	err = decoder.Decode(input)
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
			problem(w, 400, "validation_error", "Send an exercise name and at least one muscle-group tag as JSON.", nil)
		}
		return false
	}
	return true
}
func operationError(w http.ResponseWriter, err error) {
	if errors.Is(err, ErrNotFound) {
		problem(w, 404, "not_found", "Exercise not found.", nil)
		return
	}
	var used *InUseError
	if errors.As(err, &used) {
		write(w, 409, map[string]any{"error": map[string]any{"code": "exercise_in_use", "message": used.Error(), "templates": used.Templates}})
		return
	}
	var fields ValidationError
	if errors.As(err, &fields) {
		problem(w, 400, "validation_error", fields.Error(), fields)
		return
	}
	slog.Error("Exercise persistence failed", "error", err)
	problem(w, 500, "internal_error", "Exercises could not be saved or loaded. Please try again.", nil)
}
func problem(w http.ResponseWriter, status int, code, message string, fields map[string]string) {
	write(w, status, map[string]any{"error": map[string]any{"code": code, "message": message, "fields": fields}})
}
func write(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
