package sessions

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"strconv"
	"time"

	"github.com/agersk0v/crow-fitness/backend/internal/exercises"
)

func validID(id string) bool {
	n, err := strconv.ParseInt(id, 10, 64)
	return err == nil && n > 0 && strconv.FormatInt(n, 10) == id
}
func Register(mux *http.ServeMux, store Store) {
	mux.HandleFunc("GET /api/sessions/active", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
		defer cancel()
		item, err := store.Active(ctx, exercises.LocalOwnerID)
		if err != nil {
			operationError(w, err)
			return
		}
		write(w, 200, map[string]any{"session": item})
	})
	mux.HandleFunc("GET /api/sessions/{id}", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if !validID(id) {
			problem(w, 400, "validation_error", "Invalid session ID.")
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
		defer cancel()
		item, err := store.Get(ctx, exercises.LocalOwnerID, id)
		if err != nil {
			operationError(w, err)
			return
		}
		write(w, 200, item)
	})
	mux.HandleFunc("POST /api/sessions", func(w http.ResponseWriter, r *http.Request) {
		var input struct {
			TemplateID string `json:"template_id"`
		}
		if !decode(w, r, &input) {
			return
		}
		if !validID(input.TemplateID) {
			problem(w, 400, "validation_error", "Choose a saved template.")
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
		defer cancel()
		item, err := store.Start(ctx, exercises.LocalOwnerID, input.TemplateID)
		if err != nil {
			operationError(w, err)
			return
		}
		w.Header().Set("Location", "/api/sessions/"+item.ID)
		write(w, 201, item)
	})
	for _, action := range []string{"cancel", "complete"} {
		mux.HandleFunc("POST /api/sessions/{id}/"+action, func(w http.ResponseWriter, r *http.Request) {
			id := r.PathValue("id")
			if !validID(id) {
				problem(w, 400, "validation_error", "Invalid session ID.")
				return
			}
			ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
			defer cancel()
			if err := store.End(ctx, exercises.LocalOwnerID, id); err != nil {
				operationError(w, err)
				return
			}
			w.Header().Set("Cache-Control", "no-store")
			w.WriteHeader(http.StatusNoContent)
		})
	}
}
func decode(w http.ResponseWriter, r *http.Request, input any) bool {
	media, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || media != "application/json" {
		problem(w, 415, "unsupported_media_type", "Use application/json.")
		return false
	}
	r.Body = http.MaxBytesReader(w, r.Body, 4096)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	err = decoder.Decode(input)
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
			problem(w, 413, "body_too_large", "Request is too large.")
		} else {
			problem(w, 400, "validation_error", "Send valid session JSON with a template ID.")
		}
		return false
	}
	return true
}
func operationError(w http.ResponseWriter, err error) {
	var conflict ActiveConflict
	switch {
	case errors.As(err, &conflict):
		write(w, 409, map[string]any{"error": map[string]any{"code": "active_session_exists", "message": conflict.Error(), "session": conflict.Session}})
	case errors.Is(err, ErrNotFound):
		problem(w, 404, "not_found", "Session or template not found.")
	case errors.Is(err, ErrArchived):
		problem(w, 409, "template_archived", "Restore this template before starting a workout.")
	case errors.Is(err, ErrEmpty):
		problem(w, 409, "template_empty", "Add exercises to this template before starting a workout.")
	default:
		slog.Error("Session persistence failed", "error", err)
		problem(w, 500, "internal_error", "Session could not be saved or loaded. Please try again.")
	}
}
func problem(w http.ResponseWriter, status int, code, message string) {
	write(w, status, map[string]any{"error": map[string]string{"code": code, "message": message}})
}
func write(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
