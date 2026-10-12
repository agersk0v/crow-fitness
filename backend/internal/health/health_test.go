package health

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type pingFunc func(context.Context) error

func (f pingFunc) Ping(ctx context.Context) error { return f(ctx) }

func TestLivenessDoesNotDependOnDatabase(t *testing.T) {
	handler := NewHandler(pingFunc(func(context.Context) error { t.Fatal("liveness queried database"); return nil }))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/health", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d", response.Code)
	}
}

func TestReadinessReflectsDatabaseState(t *testing.T) {
	for _, tc := range []struct {
		name string
		err  error
		want int
	}{
		{"connected", nil, http.StatusOK},
		{"unreachable", errors.New("private database detail"), http.StatusServiceUnavailable},
	} {
		t.Run(tc.name, func(t *testing.T) {
			handler := NewHandler(pingFunc(func(ctx context.Context) error {
				if _, ok := ctx.Deadline(); !ok {
					t.Fatal("readiness has no timeout")
				}
				return tc.err
			}))
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/ready", nil))
			if response.Code != tc.want {
				t.Fatalf("status = %d, want %d", response.Code, tc.want)
			}
			if strings.Contains(response.Body.String(), "private") {
				t.Fatal("response leaked database detail")
			}
			if response.Header().Get("Content-Type") != "application/json" || response.Header().Get("Cache-Control") != "no-store" {
				t.Fatal("unexpected response headers")
			}
		})
	}
}

func TestReadinessCancelsSlowDatabaseCheck(t *testing.T) {
	handler := NewHandler(pingFunc(func(ctx context.Context) error { <-ctx.Done(); return ctx.Err() }))
	response := httptest.NewRecorder()
	start := time.Now()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/ready", nil))
	if response.Code != http.StatusServiceUnavailable || time.Since(start) > 3*time.Second {
		t.Fatal("readiness did not fail within its timeout")
	}
}
