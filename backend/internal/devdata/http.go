package devdata

import (
	"context"
	"encoding/json"
	"github.com/agersk0v/crow-fitness/backend/internal/auth"
	"log/slog"
	"net/http"
	"time"
)

type Store interface {
	FullReset(context.Context) error
	Reset(context.Context, int64, time.Time) (Result, error)
}

func Register(mux *http.ServeMux, store Store, enabled bool) {
	// No handler is registered unless the explicit testing setting is enabled.
	if !enabled {
		return
	}
	mux.HandleFunc("POST /api/dev/reset-all", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Type", "application/json")
		if auth.OwnerID(r.Context()) <= 0 {
			w.WriteHeader(401)
			return
		}
		if r.Header.Get("X-Confirm-Reset") != "delete-all-data-and-users" {
			w.WriteHeader(400)
			json.NewEncoder(w).Encode(map[string]any{"error": map[string]string{"code": "confirmation_required", "message": "Confirm deletion of all data and users."}})
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
		defer cancel()
		if err := store.FullReset(ctx); err != nil {
			slog.Error("Full development reset failed", "error", err)
			w.WriteHeader(500)
			json.NewEncoder(w).Encode(map[string]any{"error": map[string]string{"code": "reset_failed", "message": "Could not confirm the full reset. Refresh before trying again."}})
			return
		}
		w.WriteHeader(204)
	})
	mux.HandleFunc("POST /api/dev/reset", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Type", "application/json")
		owner := auth.OwnerID(r.Context())
		if owner <= 0 {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		// A required custom header prevents cross-origin HTML form submissions.
		if r.Header.Get("X-Confirm-Reset") != "replace-workout-data" {
			w.WriteHeader(400)
			json.NewEncoder(w).Encode(map[string]any{"error": map[string]string{"code": "confirmation_required", "message": "Confirm replacement of workout data."}})
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
		defer cancel()
		result, err := store.Reset(ctx, owner, time.Now())
		if err != nil {
			slog.Error("Dummy data reset failed", "error", err)
			w.WriteHeader(500)
			json.NewEncoder(w).Encode(map[string]any{"error": map[string]string{"code": "reset_failed", "message": "Could not reset and add dummy data. Refresh to check the current data before trying again."}})
			return
		}
		json.NewEncoder(w).Encode(result)
	})
}
