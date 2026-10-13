package exercises

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/jackc/pgx/v5/pgxpool"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"strings"
	"time"
)

// LocalOwnerID is selected by the server, never from browser input.
const LocalOwnerID int64 = 1

type Exercise struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}
type Store interface {
	List(context.Context, int64) ([]Exercise, error)
	Create(context.Context, int64, string) (Exercise, error)
}
type Postgres struct{ Pool *pgxpool.Pool }

func (p Postgres) List(ctx context.Context, owner int64) ([]Exercise, error) {
	rows, err := p.Pool.Query(ctx, "SELECT id::text, name FROM exercises WHERE owner_id=$1 ORDER BY id", owner)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []Exercise{}
	for rows.Next() {
		var item Exercise
		if err := rows.Scan(&item.ID, &item.Name); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}
func (p Postgres) Create(ctx context.Context, owner int64, name string) (Exercise, error) {
	var item Exercise
	err := p.Pool.QueryRow(ctx, "INSERT INTO exercises(owner_id,name) VALUES ($1,$2) RETURNING id::text,name", owner, name).Scan(&item.ID, &item.Name)
	return item, err
}
func Register(mux *http.ServeMux, store Store) {
	mux.HandleFunc("GET /api/exercises", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
		defer cancel()
		items, err := store.List(ctx, LocalOwnerID)
		if err != nil {
			failure(w, err)
			return
		}
		if items == nil {
			items = []Exercise{}
		}
		write(w, 200, map[string]any{"items": items})
	})
	mux.HandleFunc("POST /api/exercises", func(w http.ResponseWriter, r *http.Request) {
		media, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
		if err != nil || media != "application/json" {
			problem(w, 415, "unsupported_media_type", "Use application/json.", nil)
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, 16384)
		decoder := json.NewDecoder(r.Body)
		decoder.DisallowUnknownFields()
		var input struct {
			Name string `json:"name"`
		}
		err = decoder.Decode(&input)
		if err == nil {
			var extra any
			if next := decoder.Decode(&extra); next != io.EOF {
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
				problem(w, 400, "validation_error", "Send a JSON object containing an exercise name.", nil)
			}
			return
		}
		name := strings.TrimSpace(input.Name)
		if name == "" {
			problem(w, 400, "validation_error", "Enter an exercise name.", map[string]string{"name": "Enter an exercise name."})
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
		defer cancel()
		item, err := store.Create(ctx, LocalOwnerID, name)
		if err != nil {
			failure(w, err)
			return
		}
		write(w, 201, item)
	})
}
func failure(w http.ResponseWriter, err error) {
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
