// Package app assembles the same authenticated routes for production and tests.
package app

import (
	"github.com/agersk0v/crow-fitness/backend/internal/auth"
	"github.com/agersk0v/crow-fitness/backend/internal/exercises"
	"github.com/agersk0v/crow-fitness/backend/internal/health"
	"github.com/agersk0v/crow-fitness/backend/internal/sessions"
	"github.com/agersk0v/crow-fitness/backend/internal/templates"
	"github.com/jackc/pgx/v5/pgxpool"
	"net/http"
)

func Handler(pool *pgxpool.Pool, manager *auth.Manager) http.Handler {
	mux := http.NewServeMux()
	manager.Register(mux)
	exercises.Register(mux, exercises.Postgres{Pool: pool})
	templates.Register(mux, templates.Postgres{Pool: pool})
	sessions.Register(mux, sessions.Postgres{Pool: pool})
	// Development reset is intentionally not exposed by the application.
	mux.Handle("/", health.NewHandler(pool))
	return manager.Middleware(mux)
}
