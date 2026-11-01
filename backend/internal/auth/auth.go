package auth

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/alexedwards/scs/v2"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/oauth2"
)

const Lifetime = 30 * 24 * time.Hour
const ActivityHeader = "X-Crow-Activity"

type principalKey struct{}

// WithPrincipal is for trusted server middleware and tests, never browser input.
func WithPrincipal(ctx context.Context, a Account) context.Context {
	return context.WithValue(ctx, principalKey{}, a)
}
func OwnerID(ctx context.Context) int64 {
	a, _ := ctx.Value(principalKey{}).(Account)
	return a.OwnerID
}

type Manager struct {
	pool     *pgxpool.Pool
	sessions *scs.SessionManager
	provider Provider
	config   Config
	cookie   string
}

func New(pool *pgxpool.Pool, c Config) *Manager { return NewWithProvider(pool, c, newGoogle(c)) }

// NewWithProvider supports protocol adapters and test-only fake providers.
// Production wiring always calls New with the fixed Google provider.
func NewWithProvider(pool *pgxpool.Pool, c Config, p Provider) *Manager {
	s := scs.New()
	s.Store = sessionStore{pool}
	s.Lifetime = Lifetime
	s.IdleTimeout = 0
	s.HashTokenInStore = true
	cookie := "crow_session_dev"
	if c.Production {
		cookie = "__Host-crow_session"
	}
	return &Manager{pool: pool, sessions: s, provider: p, config: c, cookie: cookie}
}
func random() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return base64.RawURLEncoding.EncodeToString(b)
}
func digest(s string) string { h := sha256.Sum256([]byte(s)); return hex.EncodeToString(h[:]) }
func jsonResponse(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "private, no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
func problem(w http.ResponseWriter, status int, code, message string) {
	jsonResponse(w, status, map[string]any{"error": map[string]string{"code": code, "message": message}})
}
func (m *Manager) setCookie(w http.ResponseWriter, token string, expiry time.Time) {
	maxAge := int(time.Until(expiry).Seconds())
	if token == "" {
		maxAge = -1
	}
	http.SetCookie(w, &http.Cookie{Name: m.cookie, Value: token, Path: "/", HttpOnly: true, Secure: strings.HasPrefix(m.config.Origin, "https://"), SameSite: http.SameSiteLaxMode, Expires: expiry, MaxAge: maxAge})
}

// Buffer private responses so failed commits never send a successful response or
// a credential cookie. Apply rolling expiry only after accepted foreground use.
type responseBuffer struct {
	header http.Header
	body   bytes.Buffer
	status int
}

func (b *responseBuffer) Header() http.Header { return b.header }
func (b *responseBuffer) WriteHeader(status int) {
	if b.status == 0 {
		b.status = status
	}
}
func (b *responseBuffer) Write(p []byte) (int, error) {
	if b.status == 0 {
		b.status = 200
	}
	return b.body.Write(p)
}
func (m *Manager) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/health" || r.URL.Path == "/api/ready" {
			next.ServeHTTP(w, r)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()
		token := ""
		if c, err := r.Cookie(m.cookie); err == nil {
			token = c.Value
		}
		if len(token) > 128 {
			m.setCookie(w, "", time.Unix(1, 0))
			problem(w, 401, "unauthenticated", "Please sign in.")
			return
		}
		ctx, err := m.sessions.Load(ctx, token)
		if err != nil {
			problem(w, 503, "auth_unavailable", "Sign-in is temporarily unavailable.")
			return
		}
		r = r.WithContext(ctx)
		public := r.URL.Path == "/api/auth/csrf" || r.URL.Path == "/api/auth/google/start" || r.URL.Path == "/api/auth/google/callback"
		var account Account
		id := m.sessions.GetInt64(ctx, "account")
		if id != 0 {
			account, err = m.account(ctx, id)
		} else {
			err = ErrInvalid
		}
		if !public && err != nil {
			if errors.Is(err, ErrInvalid) {
				m.setCookie(w, "", time.Unix(1, 0))
				problem(w, 401, "unauthenticated", "Please sign in.")
			} else {
				problem(w, 503, "auth_unavailable", "Sign-in is temporarily unavailable.")
			}
			return
		}
		if err == nil {
			r = r.WithContext(WithPrincipal(ctx, account))
		}
		if r.Method != "GET" && r.Method != "HEAD" && r.URL.Path != "/api/auth/google/callback" {
			if !m.checkCSRF(r) {
				if r.URL.Path == "/api/auth/google/start" {
					http.Redirect(w, r, "/login?error=google", 303)
					return
				}
				problem(w, 403, "csrf_failed", "Reload and try again.")
				return
			}
		}
		b := &responseBuffer{header: make(http.Header)}
		next.ServeHTTP(b, r)
		if b.status == 0 {
			b.status = 200
		}
		fullReset := r.URL.Path == "/api/dev/reset-all" && b.status == http.StatusNoContent
		// A full development reset revoked every session inside its transaction.
		// Do not renew this now-revoked cookie or turn the committed reset into 401.
		if fullReset {
			m.setCookie(w, "", time.Unix(1, 0))
		}
		foreground := !fullReset && r.Header.Get(ActivityHeader) == "foreground" && id != 0 && b.status < 400 && !strings.HasPrefix(r.URL.Path, "/api/auth/google/")
		if foreground && m.sessions.Status(ctx) != scs.Destroyed {
			m.sessions.SetDeadline(ctx, time.Now().Add(Lifetime))
		}
		switch m.sessions.Status(ctx) {
		case scs.Modified:
			saved, expiry, e := m.sessions.Commit(ctx)
			if e != nil {
				if errors.Is(e, ErrInvalid) {
					problem(w, 401, "unauthenticated", "Please sign in again.")
				} else {
					problem(w, 503, "auth_unavailable", "Sign-in is temporarily unavailable.")
				}
				return
			}
			m.setCookie(w, saved, expiry)
		case scs.Destroyed:
			m.setCookie(w, "", time.Unix(1, 0))
		}
		for key, values := range b.header {
			for _, value := range values {
				w.Header().Add(key, value)
			}
		}
		w.Header().Set("Cache-Control", "private, no-store")
		if account.ID != 0 {
			w.Header().Set("X-Crow-Account", strconv.FormatInt(account.ID, 10))
		}
		w.WriteHeader(b.status)
		_, _ = w.Write(b.body.Bytes())
	})
}
func (m *Manager) checkCSRF(r *http.Request) bool {
	if r.Header.Get("Origin") != m.config.Origin {
		return false
	}
	token := r.Header.Get("X-CSRF-Token")
	if r.URL.Path == "/api/auth/google/start" {
		r.Body = http.MaxBytesReader(nil, r.Body, 4096)
		if err := r.ParseForm(); err != nil {
			return false
		}
		if len(r.PostForm["csrf"]) != 1 {
			return false
		}
		token = r.PostForm.Get("csrf")
	}
	expected := m.sessions.GetString(r.Context(), "csrf")
	return expected != "" && subtle.ConstantTimeCompare([]byte(token), []byte(expected)) == 1
}
func (m *Manager) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/auth/csrf", func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		token := m.sessions.GetString(ctx, "csrf")
		if token == "" {
			token = random()
			m.sessions.Put(ctx, "csrf", token)
			if m.sessions.GetInt64(ctx, "account") == 0 {
				m.sessions.SetDeadline(ctx, time.Now().Add(10*time.Minute))
			}
		}
		jsonResponse(w, 200, map[string]string{"csrf": token})
	})
	mux.HandleFunc("GET /api/auth/me", func(w http.ResponseWriter, r *http.Request) {
		a := r.Context().Value(principalKey{}).(Account)
		jsonResponse(w, 200, map[string]string{"id": strconv.FormatInt(a.ID, 10), "name": a.Name, "email": a.Email})
	})
	mux.HandleFunc("POST /api/auth/logout", func(w http.ResponseWriter, r *http.Request) {
		if err := m.sessions.Destroy(r.Context()); err != nil {
			problem(w, 503, "auth_unavailable", "Could not sign out. Try again.")
			return
		}
		w.WriteHeader(204)
	})
	mux.HandleFunc("POST /api/auth/google/start", m.start)
	mux.HandleFunc("GET /api/auth/google/callback", m.callback)
}
func (m *Manager) start(w http.ResponseWriter, r *http.Request) {
	// Login from an authenticated browser must first sign out. This avoids silently
	// attaching a different identity or crossing existing account drafts.
	if m.sessions.GetInt64(r.Context(), "account") != 0 {
		problem(w, 409, "already_signed_in", "Sign out before choosing another account.")
		return
	}
	state, nonce, verifier := random(), random(), oauth2.GenerateVerifier()
	browser := m.sessions.Token(r.Context())
	if browser == "" {
		problem(w, 400, "login_expired", "Reload the login page.")
		return
	}
	_, err := m.pool.Exec(r.Context(), `INSERT INTO auth_attempts(state,browser,nonce,verifier,expiry) VALUES($1,$2,$3,$4,now()+interval '5 minutes')`, digest(state), digest(browser), nonce, verifier)
	if err != nil {
		problem(w, 503, "auth_unavailable", "Could not start sign-in. Try again.")
		return
	}
	http.Redirect(w, r, m.provider.URL(state, nonce, verifier), 303)
}
func (m *Manager) callback(w http.ResponseWriter, r *http.Request) {
	fail := func() { http.Redirect(w, r, "/login?error=google", 303) }
	q := r.URL.Query()
	if len(q["state"]) != 1 || len(q["code"]) > 1 || len(q["error"]) > 1 || len(q.Get("state")) > 128 {
		fail()
		return
	}
	var nonce, verifier string
	err := m.pool.QueryRow(r.Context(), `DELETE FROM auth_attempts WHERE state=$1 AND browser=$2 AND expiry>now() RETURNING nonce,verifier`, digest(q.Get("state")), digest(m.sessions.Token(r.Context()))).Scan(&nonce, &verifier)
	if err != nil {
		if !errors.Is(err, pgx.ErrNoRows) {
			problem(w, 503, "auth_unavailable", "Sign-in is temporarily unavailable.")
		} else {
			fail()
		}
		return
	}
	if q.Get("code") == "" || q.Get("error") != "" || m.sessions.GetInt64(r.Context(), "account") != 0 {
		fail()
		return
	}
	identity, err := m.provider.Exchange(r.Context(), q.Get("code"), verifier, nonce)
	if err != nil || identity.Subject == "" {
		fail()
		return
	}
	a, err := m.resolve(r.Context(), identity)
	if err != nil {
		problem(w, 503, "auth_unavailable", "Could not create account. Try again.")
		return
	}
	// Delete the old anonymous session before adding authenticated privileges.
	if err = m.sessions.RenewToken(r.Context()); err != nil {
		problem(w, 503, "auth_unavailable", "Could not complete sign-in. Try again.")
		return
	}
	m.sessions.Put(r.Context(), "account", a.ID)
	m.sessions.Put(r.Context(), "csrf", random())
	m.sessions.SetDeadline(r.Context(), time.Now().Add(Lifetime))
	http.Redirect(w, r, "/", 303)
}

// Cleanup never deletes revocation tombstones until well beyond the maximum
// request lifetime. No in-flight request can recreate a token after removal.
func (m *Manager) Cleanup(ctx context.Context) {
	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			_, _ = m.pool.Exec(ctx, `DELETE FROM auth_sessions WHERE expiry<now()-interval '31 days'; DELETE FROM auth_attempts WHERE expiry<now()-interval '1 day'`)
		}
	}
}
