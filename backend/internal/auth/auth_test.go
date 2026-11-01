package auth

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/agersk0v/crow-fitness/backend/internal/migrations"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/oauth2"
)

func testPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	connection := os.Getenv("TEST_DATABASE_URL")
	if connection == "" {
		t.Skip("TEST_DATABASE_URL required for disposable PostgreSQL tests")
	}
	ctx := context.Background()
	admin, err := pgxpool.New(ctx, connection)
	if err != nil {
		t.Fatal(err)
	}
	schema := "auth_" + strings.ToLower(fmt.Sprintf("%x", time.Now().UnixNano()))
	if _, err = admin.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}
	cfg, err := pgxpool.ParseConfig(connection)
	if err != nil {
		t.Fatal(err)
	}
	cfg.ConnConfig.RuntimeParams["search_path"] = schema
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { pool.Close(); _, _ = admin.Exec(ctx, "DROP SCHEMA "+schema+" CASCADE"); admin.Close() })
	if err = migrations.Apply(ctx, pool); err != nil {
		t.Fatal(err)
	}
	return pool
}

type fakeProvider struct {
	mu              sync.Mutex
	nonce, verifier string
}

func (p *fakeProvider) URL(state, nonce, verifier string) string {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.nonce = nonce
	p.verifier = verifier
	return "/api/auth/google/callback?state=" + url.QueryEscape(state) + "&code=valid"
}
func (p *fakeProvider) Exchange(_ context.Context, code, verifier, nonce string) (Identity, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if code != "valid" || nonce != p.nonce || verifier != p.verifier {
		return Identity{}, fmt.Errorf("bad exchange")
	}
	return Identity{Subject: "google-user", Email: "user@example.com", Name: "User"}, nil
}

type browser struct {
	t       *testing.T
	m       *Manager
	handler http.Handler
	cookie  *http.Cookie
	csrf    string
}

func (b *browser) request(method, path, body string, headers map[string]string) *httptest.ResponseRecorder {
	b.t.Helper()
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	if b.cookie != nil {
		r.AddCookie(b.cookie)
	}
	if method != "GET" {
		r.Header.Set("Origin", b.m.config.Origin)
		r.Header.Set("X-CSRF-Token", b.csrf)
	}
	for k, v := range headers {
		r.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	b.handler.ServeHTTP(w, r)
	for _, c := range w.Result().Cookies() {
		if c.Name == b.m.cookie {
			b.cookie = c
		}
	}
	return w
}
func (b *browser) token() {
	w := b.request("GET", "/api/auth/csrf", "", nil)
	if w.Code != 200 {
		b.t.Fatal(w.Code, w.Body)
	}
	var v map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &v); err != nil {
		b.t.Fatal(err)
	}
	b.csrf = v["csrf"]
}
func (b *browser) login() {
	b.token()
	old := b.cookie.Value
	w := b.request("POST", "/api/auth/google/start", url.Values{"csrf": {b.csrf}}.Encode(), map[string]string{"Content-Type": "application/x-www-form-urlencoded"})
	if w.Code != 303 {
		b.t.Fatal(w.Code, w.Body)
	}
	w = b.request("GET", w.Header().Get("Location"), "", nil)
	if w.Code != 303 || w.Header().Get("Location") != "/" {
		b.t.Fatal(w.Code, w.Body)
	}
	if b.cookie.Value == old {
		b.t.Fatal("session fixation")
	}
	b.token()
}
func TestSessionsAndRevocation(t *testing.T) {
	pool := testPool(t)
	m := NewWithProvider(pool, Config{Origin: "http://localhost:3000"}, &fakeProvider{})
	mux := http.NewServeMux()
	m.Register(mux)
	mux.HandleFunc("GET /private", func(w http.ResponseWriter, r *http.Request) {
		if OwnerID(r.Context()) <= 1 {
			t.Error("legacy/fixed owner")
		}
		w.WriteHeader(200)
	})
	handler := m.Middleware(mux)
	b := browser{t: t, m: m, handler: handler}
	if w := b.request("GET", "/private", "", nil); w.Code != 401 {
		t.Fatal(w.Code)
	}
	b.login()
	saved := *b.cookie
	if !saved.HttpOnly || saved.SameSite != http.SameSiteLaxMode || saved.Path != "/" || saved.MaxAge < 29*86400 {
		t.Fatalf("cookie: %+v", saved)
	}
	other := browser{t: t, m: m, handler: handler}
	other.login()
	if other.cookie.Value == b.cookie.Value {
		t.Fatal("shared device session")
	}
	// Set an old serialized deadline and DB expiry; background reads must not renew.
	ctx, err := m.sessions.Load(context.Background(), b.cookie.Value)
	if err != nil {
		t.Fatal(err)
	}
	m.sessions.SetDeadline(ctx, time.Now().Add(time.Hour))
	if _, _, err = m.sessions.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	hash := sha256.Sum256([]byte(b.cookie.Value))
	key := base64.RawURLEncoding.EncodeToString(hash[:])
	var before, after time.Time
	if err = pool.QueryRow(context.Background(), `SELECT expiry FROM auth_sessions WHERE token=$1`, key).Scan(&before); err != nil {
		t.Fatal(err)
	}
	if w := b.request("GET", "/private", "", nil); w.Code != 200 {
		t.Fatal(w.Code, w.Body)
	}
	if err = pool.QueryRow(context.Background(), `SELECT expiry FROM auth_sessions WHERE token=$1`, key).Scan(&after); err != nil || !after.Equal(before) {
		t.Fatal("background renewal", err)
	}
	// Force the authoritative DB expiry near now to verify rolling renewal beyond it.
	if _, err = pool.Exec(context.Background(), `UPDATE auth_sessions SET expiry=now()+interval '1 hour' WHERE token=$1`, key); err != nil {
		t.Fatal(err)
	}
	if w := b.request("GET", "/private", "", map[string]string{ActivityHeader: "foreground"}); w.Code != 200 {
		t.Fatal(w.Code, w.Body)
	}
	if err = pool.QueryRow(context.Background(), `SELECT expiry FROM auth_sessions WHERE token=$1`, key).Scan(&after); err != nil || time.Until(after) < 29*24*time.Hour {
		t.Fatal("rolling deadline", after, err)
	}
	// Manager recreation models another replica/restart using the same PostgreSQL store.
	m2 := NewWithProvider(pool, m.config, &fakeProvider{})
	mux2 := http.NewServeMux()
	m2.Register(mux2)
	restarted := browser{t: t, m: m2, handler: m2.Middleware(mux2), cookie: b.cookie}
	if w := restarted.request("GET", "/api/auth/me", "", nil); w.Code != 200 {
		t.Fatal("restart", w.Code, w.Body)
	}
	stale, err := m.sessions.Load(context.Background(), b.cookie.Value)
	if err != nil {
		t.Fatal(err)
	}
	if w := b.request("POST", "/api/auth/logout", "", map[string]string{"Origin": "https://evil.example"}); w.Code != 403 {
		t.Fatal("csrf", w.Code)
	}
	if w := b.request("POST", "/api/auth/logout", "", nil); w.Code != 204 {
		t.Fatal(w.Code, w.Body)
	}
	m.sessions.SetDeadline(stale, time.Now().Add(Lifetime))
	if _, _, err = m.sessions.Commit(stale); err != ErrInvalid {
		t.Fatal("revoked session resurrected", err)
	}
	b.cookie = &saved
	if w := b.request("GET", "/private", "", nil); w.Code != 401 {
		t.Fatal("logout", w.Code)
	}
	if w := other.request("GET", "/private", "", nil); w.Code != 200 {
		t.Fatal("other device signed out", w.Code)
	}
}
func TestStateExpiryReplayAndRegistration(t *testing.T) {
	pool := testPool(t)
	m := NewWithProvider(pool, Config{Origin: "http://localhost:3000"}, &fakeProvider{})
	mux := http.NewServeMux()
	m.Register(mux)
	handler := m.Middleware(mux)
	b := browser{t: t, m: m, handler: handler}
	b.token()
	start := b.request("POST", "/api/auth/google/start", url.Values{"csrf": {b.csrf}}.Encode(), map[string]string{"Content-Type": "application/x-www-form-urlencoded"})
	location := start.Header().Get("Location")
	if location == "" {
		t.Fatal(start.Body)
	}
	foreign := browser{t: t, m: m, handler: handler}
	foreign.token()
	w := foreign.request("GET", location, "", nil)
	if w.Header().Get("Location") != "/login?error=google" {
		t.Fatal("wrong-browser accepted")
	}
	w = b.request("GET", location, "", nil)
	if w.Header().Get("Location") != "/" {
		t.Fatal(w.Body)
	}
	w = b.request("GET", location, "", nil)
	if w.Header().Get("Location") != "/login?error=google" {
		t.Fatal("replay accepted")
	}
	ctx := context.Background()
	var wg sync.WaitGroup
	ids := make(chan int64, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			a, err := m.resolve(ctx, Identity{Subject: "concurrent", Email: "same@example.com"})
			if err != nil {
				t.Error(err)
				return
			}
			ids <- a.ID
		}()
	}
	wg.Wait()
	close(ids)
	var first int64
	for id := range ids {
		if first != 0 && id != first {
			t.Fatal("duplicate account")
		}
		first = id
	}
	a, err := m.resolve(ctx, Identity{Subject: "another", Email: "same@example.com"})
	if err != nil || a.ID == first {
		t.Fatal("email merged", err)
	}
	a2, err := m.resolve(ctx, Identity{Subject: "another", Email: "renamed@example.com"})
	if err != nil || a2.ID != a.ID || a2.OwnerID != a.OwnerID {
		t.Fatal("profile update changed account", err)
	}
	// Expired cookies cannot acquire another 30 days of foreground renewal.
	if _, err = pool.Exec(ctx, `UPDATE auth_sessions SET expiry=now()-interval '1 second' WHERE NOT revoked`); err != nil {
		t.Fatal(err)
	}
	if w = b.request("GET", "/api/auth/me", "", map[string]string{ActivityHeader: "foreground"}); w.Code != 401 {
		t.Fatal("expired session accepted", w.Code)
	}
}
func TestConfiguration(t *testing.T) {
	base := map[string]string{"APP_ENV": "development", "PUBLIC_ORIGIN": "http://localhost:3000", "GOOGLE_CLIENT_ID": "id", "GOOGLE_CLIENT_SECRET": "secret"}
	for _, change := range []map[string]string{{}, {"APP_ENV": ""}, {"PUBLIC_ORIGIN": "https://example.com/path"}, {"PUBLIC_ORIGIN": "http://example.com"}, {"GOOGLE_CLIENT_SECRET": ""}, {"APP_ENV": "production"}, {"APP_ENV": "production", "PUBLIC_ORIGIN": "https://example.com", "ENABLE_DEV_RESET": "true"}} {
		values := map[string]string{}
		for k, v := range base {
			values[k] = v
		}
		for k, v := range change {
			values[k] = v
		}
		_, err := ConfigFromEnv(func(k string) string { return values[k] })
		if (err == nil) != (len(change) == 0) {
			t.Fatalf("configuration %+v: %v", change, err)
		}
	}
}
func TestPKCEAuthorizationURL(t *testing.T) {
	p := newGoogle(Config{Origin: "https://fitness.example.com", ClientID: "id", ClientSecret: "secret"})
	v := oauth2.GenerateVerifier()
	u, err := url.Parse(p.URL("state", "nonce", v))
	if err != nil {
		t.Fatal(err)
	}
	q := u.Query()
	if q.Get("state") != "state" || q.Get("nonce") != "nonce" || q.Get("code_challenge_method") != "S256" || q.Get("code_challenge") != oauth2.S256ChallengeFromVerifier(v) || q.Get("redirect_uri") != "https://fitness.example.com/api/auth/google/callback" {
		t.Fatal(u)
	}
}

func TestExpiredAttemptAndUnavailableStore(t *testing.T) {
	pool := testPool(t)
	m := NewWithProvider(pool, Config{Origin: "http://localhost:3000"}, &fakeProvider{})
	mux := http.NewServeMux()
	m.Register(mux)
	handler := m.Middleware(mux)
	b := browser{t: t, m: m, handler: handler}
	b.token()
	start := b.request("POST", "/api/auth/google/start", url.Values{"csrf": {b.csrf}}.Encode(), map[string]string{"Content-Type": "application/x-www-form-urlencoded"})
	if _, err := pool.Exec(context.Background(), `UPDATE auth_attempts SET expiry=now()-interval '1 second'`); err != nil {
		t.Fatal(err)
	}
	if w := b.request("GET", start.Header().Get("Location"), "", nil); w.Header().Get("Location") != "/login?error=google" {
		t.Fatal("expired state accepted", w.Body)
	}
	// No data/API fallback when the session database is unavailable.
	pool.Close()
	if w := b.request("GET", "/api/auth/me", "", nil); w.Code != 503 {
		t.Fatal("store outage", w.Code, w.Body)
	}
}
