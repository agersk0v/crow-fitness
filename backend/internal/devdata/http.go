package devdata

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"
)

type Store interface {
	Reset(context.Context, time.Time) (Result, error)
}

func Register(mux *http.ServeMux, store Store, enabled bool) {
	// No handler is registered unless the explicit testing setting is enabled.
	if !enabled {
		return
	}
	mux.HandleFunc("POST /api/dev/reset", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Type", "application/json")
		// A required custom header prevents cross-origin HTML form submissions.
		if r.Header.Get("X-Confirm-Reset") != "replace-workout-data" {
			w.WriteHeader(400)
			json.NewEncoder(w).Encode(map[string]any{"error": map[string]string{"code": "confirmation_required", "message": "Confirm replacement of workout data."}})
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
		defer cancel()
		result, err := store.Reset(ctx, time.Now())
		if err != nil {
			slog.Error("Dummy data reset failed", "error", err)
			w.WriteHeader(500)
			json.NewEncoder(w).Encode(map[string]any{"error": map[string]string{"code": "reset_failed", "message": "Could not reset and add dummy data. Refresh to check the current data before trying again."}})
			return
		}
		json.NewEncoder(w).Encode(result)
	})
}
