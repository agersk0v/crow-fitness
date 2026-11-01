package app

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/agersk0v/crow-fitness/backend/internal/auth"
	"github.com/agersk0v/crow-fitness/backend/internal/config"
	"github.com/agersk0v/crow-fitness/backend/internal/migrations"
	"github.com/jackc/pgx/v5/pgxpool"
)

// This fake provider exists only in a Go test binary, never in the production
// executable. The smoke Docker target explicitly runs this test server.
type smokeProvider struct{ origin string }

func (p smokeProvider) URL(state, nonce, verifier string) string {
	return p.origin + "/api/auth/google/callback?state=" + url.QueryEscape(state) + "&code=smoke-only"
}
func (p smokeProvider) Exchange(_ context.Context, code, verifier, nonce string) (auth.Identity, error) {
	if (code != "smoke-only" && code != "smoke-other") || verifier == "" || nonce == "" {
		return auth.Identity{}, fmt.Errorf("invalid smoke exchange")
	}
	subject := "smoke-user"
	if code == "smoke-other" {
		subject = "smoke-other-user"
	}
	return auth.Identity{Subject: subject, Name: "Smoke user"}, nil
}
func TestSmokeServer(t *testing.T) {
	if os.Getenv("SMOKE_SERVER") != "1" {
		t.Skip("explicit test-only server")
	}
	cfg, err := config.Load(os.Getenv)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if err = migrations.Apply(ctx, pool); err != nil {
		t.Fatal(err)
	}
	c := auth.Config{Origin: os.Getenv("PUBLIC_ORIGIN")}
	manager := auth.NewWithProvider(pool, c, smokeProvider{c.Origin})
	server := &http.Server{Addr: ":" + cfg.HTTPPort, Handler: Handler(pool, manager, cfg.EnableDevReset), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 10 * time.Second}
	t.Fatal(server.ListenAndServe())
}
