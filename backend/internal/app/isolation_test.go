package app

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/agersk0v/crow-fitness/backend/internal/auth"
	"github.com/agersk0v/crow-fitness/backend/internal/migrations"
	"github.com/jackc/pgx/v5/pgxpool"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"
)

func isolationPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	connection := os.Getenv("TEST_DATABASE_URL")
	if connection == "" {
		t.Skip("TEST_DATABASE_URL required")
	}
	ctx := context.Background()
	admin, err := pgxpool.New(ctx, connection)
	if err != nil {
		t.Fatal(err)
	}
	schema := fmt.Sprintf("isolation_%x", time.Now().UnixNano())
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

// Test provider only: every request still passes the real login/session middleware.
type isolationProvider struct{}

func (isolationProvider) URL(state, nonce, verifier string) string {
	return "/api/auth/google/callback?state=" + url.QueryEscape(state) + "&code=alice"
}
func (isolationProvider) Exchange(_ context.Context, code, verifier, nonce string) (auth.Identity, error) {
	if (code != "alice" && code != "bob") || verifier == "" || nonce == "" {
		return auth.Identity{}, fmt.Errorf("bad test exchange")
	}
	return auth.Identity{Subject: code, Name: code}, nil
}

type accountBrowser struct {
	t             *testing.T
	handler       http.Handler
	cookie        *http.Cookie
	csrf, account string
}

func (b *accountBrowser) request(method, path, body string, headers map[string]string) *httptest.ResponseRecorder {
	b.t.Helper()
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	if b.cookie != nil {
		r.AddCookie(b.cookie)
	}
	r.Header.Set("Content-Type", "application/json")
	if method != "GET" {
		r.Header.Set("Origin", "http://localhost:3000")
		r.Header.Set("X-CSRF-Token", b.csrf)
	}
	r.Header.Set("X-Owner-ID", "1")
	r.Header.Set("X-Account-ID", "1")
	r.Header.Set("Authorization", "Bearer forged")
	for k, v := range headers {
		r.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	b.handler.ServeHTTP(w, r)
	for _, c := range w.Result().Cookies() {
		if c.Name == "crow_session_dev" {
			b.cookie = c
		}
	}
	return w
}
func (b *accountBrowser) expect(method, path, body string, status int) *httptest.ResponseRecorder {
	b.t.Helper()
	w := b.request(method, path, body, nil)
	if w.Code != status {
		b.t.Fatalf("%s %s: got %d want %d: %s", method, path, w.Code, status, w.Body)
	}
	if !strings.Contains(w.Header().Get("Cache-Control"), "no-store") {
		b.t.Fatalf("cacheable %s", path)
	}
	if b.account != "" && w.Header().Get("X-Crow-Account") != "" && w.Header().Get("X-Crow-Account") != b.account {
		b.t.Fatal("wrong response account")
	}
	return w
}
func (b *accountBrowser) token() {
	w := b.expect("GET", "/api/auth/csrf", "", 200)
	var v struct {
		CSRF string `json:"csrf"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &v); err != nil {
		b.t.Fatal(err)
	}
	b.csrf = v.CSRF
}
func (b *accountBrowser) login(subject string) {
	b.token()
	w := b.request("POST", "/api/auth/google/start", url.Values{"csrf": {b.csrf}}.Encode(), map[string]string{"Content-Type": "application/x-www-form-urlencoded"})
	if w.Code != 303 {
		b.t.Fatal(w.Code, w.Body)
	}
	b.expect("GET", strings.Replace(w.Header().Get("Location"), "code=alice", "code="+subject, 1), "", 303)
	b.token()
	w = b.expect("GET", "/api/auth/me", "", 200)
	var v struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &v); err != nil {
		b.t.Fatal(err)
	}
	b.account = v.ID
}
func resultID(t *testing.T, w *httptest.ResponseRecorder) string {
	t.Helper()
	var v struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &v); err != nil || v.ID == "" {
		t.Fatal(w.Body, err)
	}
	return v.ID
}
func TestPrivateAccountRoutes(t *testing.T) {
	pool := isolationPool(t)
	if _, err := pool.Exec(context.Background(), "INSERT INTO exercises(owner_id,name,muscle_groups) VALUES(1,'Unclaimed legacy lift',ARRAY['core'])"); err != nil {
		t.Fatal(err)
	}
	handler := Handler(pool, auth.NewWithProvider(pool, auth.Config{Origin: "http://localhost:3000"}, isolationProvider{}), false)
	a := &accountBrowser{t: t, handler: handler}
	b := &accountBrowser{t: t, handler: handler}
	guest := &accountBrowser{t: t, handler: handler}
	a.login("alice")
	b.login("bob")
	if a.account == b.account {
		t.Fatal("shared accounts")
	}
	for _, c := range []*accountBrowser{a, b} {
		w := c.expect("GET", "/api/exercises", "", 200)
		var v struct {
			Items []json.RawMessage `json:"items"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &v); err != nil || len(v.Items) != 0 {
			t.Fatal("new account inherited data", w.Body, err)
		}
	}

	ex := `{"name":"Alice private lift","muscle_groups":["chest"]}`
	ae := resultID(t, a.expect("POST", "/api/exercises", ex, 201))
	be := resultID(t, b.expect("POST", "/api/exercises", `{"name":"Bob private lift","muscle_groups":["back"]}`, 201))
	template := func(name, id string) string {
		return fmt.Sprintf(`{"name":%q,"exercises":[{"exercise_id":%q,"set_count":1}]}`, name, id)
	}
	at := resultID(t, a.expect("POST", "/api/templates", template("Alice private routine", ae), 201))
	bt := resultID(t, b.expect("POST", "/api/templates", template("Bob private routine", be), 201))
	start := func(c *accountBrowser, id string) string {
		return resultID(t, c.expect("POST", "/api/sessions", `{"template_id":"`+id+`"}`, 201))
	}
	as := start(a, at)
	bs := start(b, bt)
	uuid := "11111111-1111-4111-8111-111111111111"
	set := `{"exercise_position":1,"weight":"20","reps":8,"technique":"decent"}`
	a.expect("PUT", "/api/sessions/"+as+"/sets/"+uuid, set, 200)
	a.expect("PUT", "/api/sessions/"+as+"/sets/"+uuid, set, 200)
	snapshot := a.expect("GET", "/api/sessions/"+as, "", 200)
	var saved struct {
		Sets []json.RawMessage `json:"sets"`
	}
	if err := json.Unmarshal(snapshot.Body.Bytes(), &saved); err != nil || len(saved.Sets) != 1 {
		t.Fatal("retry duplicated set", snapshot.Body, err)
	}

	b.expect("PUT", "/api/sessions/"+bs+"/sets/"+uuid, set, 200)
	foreign := []struct{ method, path, body string }{
		{"PUT", "/api/exercises/" + ae, ex}, {"DELETE", "/api/exercises/" + ae, ""}, {"GET", "/api/exercises/" + ae + "/templates", ""},
		{"GET", "/api/templates/" + at, ""}, {"PUT", "/api/templates/" + at, template("Routine", be)},
		{"POST", "/api/templates/" + at + "/duplicate", `{"name":"Copy"}`}, {"POST", "/api/templates/" + at + "/archive", ""}, {"POST", "/api/templates/" + at + "/restore", ""},
		{"POST", "/api/sessions", `{"template_id":"` + at + `"}`}, {"GET", "/api/sessions/" + as, ""}, {"POST", "/api/sessions/" + as + "/cancel", ""}, {"POST", "/api/sessions/" + as + "/complete", ""},
		{"GET", "/api/sessions/" + as + "/previous", ""}, {"GET", "/api/sessions/" + as + "/bests", ""},
		{"PUT", "/api/sessions/" + as + "/selection", `{"exercise_position":1,"exercise_id":"` + ae + `","selection_revision":0}`},
		{"PUT", "/api/sessions/" + as + "/sets/" + uuid, set}, {"DELETE", "/api/sessions/" + as + "/sets/" + uuid, ""},
		{"PUT", "/api/sessions/" + as + "/sets/order", `{"exercise_position":1,"set_ids":["` + uuid + `"]}`},
		{"GET", "/api/history/sessions/" + as, ""},
	}
	for _, r := range foreign {
		t.Run(r.method+" "+r.path, func(t *testing.T) {
			guest.expect(r.method, r.path, r.body, 401)
			b.expect(r.method, r.path, r.body, 404)
			if r.method != "GET" {
				for _, headers := range []map[string]string{{"Origin": "https://evil.example"}, {"X-CSRF-Token": ""}} {
					w := b.request(r.method, r.path, r.body, headers)
					if w.Code != 403 {
						t.Fatalf("CSRF accepted: %d", w.Code)
					}
				}
			}
		})
	}
	b.expect("POST", "/api/sessions", `{"template_id":"`+bt+`"}`, 409)
	for _, id := range []string{ae, "999999999"} {
		b.expect("POST", "/api/templates", template("Invalid", id), 400)
	}
	alt := fmt.Sprintf(`{"name":"Invalid","exercises":[{"exercise_id":%q,"set_count":1,"alternatives":[{"exercise_id":%q,"set_count":1}]}]}`, be, ae)
	b.expect("POST", "/api/templates", alt, 400)
	b.expect("DELETE", "/api/sessions/"+bs+"/sets/"+uuid, "", 204)
	b.expect("PUT", "/api/sessions/"+bs+"/selection", fmt.Sprintf(`{"exercise_position":1,"exercise_id":%q,"selection_revision":0}`, ae), 400)
	a.expect("POST", "/api/sessions/"+as+"/complete", "", 204)
	a.expect("POST", "/api/sessions/"+as+"/complete", "", 204)
	b.expect("GET", "/api/history/sessions/"+as, "", 404)
	for _, path := range []string{"/api/exercises?paginated=true&page=1", "/api/templates", "/api/sessions/active", "/api/sets", "/api/history/sessions", "/api/history/exercises", "/api/history/templates", "/api/history/progress?exercise=" + ae, "/api/history/sessions?exercise=" + ae, "/api/history/sessions?template=" + at, "/api/sessions/" + bs + "/previous", "/api/sessions/" + bs + "/bests"} {
		guest.expect("GET", path, "", 401)
		w := b.expect("GET", path, "", 200)
		if strings.Contains(w.Body.String(), "Alice private") {
			t.Fatalf("leaked %s: %s", path, w.Body)
		}
	}
	for _, r := range []struct{ method, path, body string }{
		{"POST", "/api/exercises", ex}, {"POST", "/api/templates", template("Copy", be)},
	} {
		guest.expect(r.method, r.path, r.body, 401)
		w := b.request(r.method, r.path, r.body, map[string]string{"X-CSRF-Token": ""})
		if w.Code != 403 {
			t.Fatal("unguarded create", w.Code)
		}
	}
	page := b.expect("GET", "/api/exercises?paginated=true&page=1", "", 200)
	var counts struct {
		TotalItems int `json:"total_items"`
		TotalPages int `json:"total_pages"`
	}
	if err := json.Unmarshal(page.Body.Bytes(), &counts); err != nil || counts.TotalItems != 1 || counts.TotalPages != 1 {
		t.Fatal("foreign rows affected counts", page.Body, err)
	}
	for _, id := range []string{as, "999999999"} {
		b.expect("POST", "/api/sessions/"+id+"/cancel", "", 404)
		b.expect("POST", "/api/sessions/"+id+"/complete", "", 404)
	}
	a.expect("GET", "/api/history/sessions/"+as, "", 200)
	b.expect("GET", "/api/sessions/"+bs, "", 200)
	cookie := *b.cookie
	b.expect("POST", "/api/auth/logout", "", 204)
	b.cookie = &cookie
	b.expect("GET", "/api/exercises", "", 401)
	guest.expect("POST", "/api/dev/reset", "", 401)
	a.expect("POST", "/api/dev/reset", "", 404)
}

func TestAccountDummyDataReset(t *testing.T) {
	pool := isolationPool(t)
	if _, err := pool.Exec(context.Background(), "INSERT INTO exercises(owner_id,name,muscle_groups) VALUES(1,'Unclaimed legacy lift',ARRAY['core'])"); err != nil {
		t.Fatal(err)
	}
	handler := Handler(pool, auth.NewWithProvider(pool, auth.Config{Origin: "http://localhost:3000"}, isolationProvider{}), true)
	a := &accountBrowser{t: t, handler: handler}
	b := &accountBrowser{t: t, handler: handler}
	guest := &accountBrowser{t: t, handler: handler}
	a.login("alice")
	b.login("bob")
	ex := resultID(t, b.expect("POST", "/api/exercises", `{"name":"Bob original lift","muscle_groups":["chest"]}`, 201))
	template := resultID(t, b.expect("POST", "/api/templates", fmt.Sprintf(`{"name":"Bob original routine","exercises":[{"exercise_id":%q,"set_count":1}]}`, ex), 201))
	session := resultID(t, b.expect("POST", "/api/sessions", fmt.Sprintf(`{"template_id":%q}`, template), 201))
	b.expect("PUT", "/api/sessions/"+session+"/sets/11111111-1111-4111-8111-111111111111", `{"exercise_position":1,"weight":"20","reps":8,"technique":"decent"}`, 200)
	bobBefore := map[string]string{}
	for _, path := range []string{"/api/exercises", "/api/templates", "/api/sessions/active"} {
		bobBefore[path] = b.expect("GET", path, "", 200).Body.String()
	}
	identity := a.expect("GET", "/api/auth/me", "", 200).Body.String()
	guest.expect("POST", "/api/dev/reset", "", 401)
	a.expect("POST", "/api/dev/reset", "", 400)
	for _, headers := range []map[string]string{
		{"X-Confirm-Reset": "replace-workout-data", "X-CSRF-Token": ""},
		{"X-Confirm-Reset": "replace-workout-data", "Origin": "https://evil.example"},
	} {
		if w := a.request("POST", "/api/dev/reset", "", headers); w.Code != 403 {
			t.Fatal("reset CSRF accepted", w.Code)
		}
	}
	var previousActive string
	for range 2 {
		// Browser-supplied ownership claims have no effect; the signed-in owner wins.
		w := a.request("POST", "/api/dev/reset", `{"owner_id":1}`, map[string]string{"X-Confirm-Reset": "replace-workout-data"})
		if w.Code != 200 {
			t.Fatal(w.Code, w.Body)
		}
		var result struct {
			Exercises, Templates int
			Active               string `json:"active_session_id"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil || result.Exercises != 26 || result.Templates != 4 || result.Active == "" || result.Active == previousActive {
			t.Fatal("seed result", w.Body, err)
		}
		previousActive = result.Active
		for path, before := range bobBefore {
			if after := b.expect("GET", path, "", 200).Body.String(); after != before {
				t.Fatalf("reset changed Bob: %s", path)
			}
		}
		if a.expect("GET", "/api/auth/me", "", 200).Body.String() != identity {
			t.Fatal("reset changed identity")
		}
		var count int
		if err := pool.QueryRow(context.Background(), "SELECT count(*) FROM exercises WHERE owner_id=1 AND name='Unclaimed legacy lift'").Scan(&count); err != nil || count != 1 {
			t.Fatal("legacy data changed", count, err)
		}
		var page struct {
			Total int `json:"total_items"`
		}
		w = a.expect("GET", "/api/exercises?paginated=true", "", 200)
		if err := json.Unmarshal(w.Body.Bytes(), &page); err != nil || page.Total != 26 {
			t.Fatal("account library", w.Body, err)
		}
		w = a.expect("GET", "/api/history/sessions", "", 200)
		var history struct {
			Items []json.RawMessage `json:"items"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &history); err != nil || len(history.Items) != 48 {
			t.Fatal("account history", w.Body, err)
		}
	}
	a.expect("GET", "/api/sessions/"+session, "", 404)
	b.expect("GET", "/api/sessions/"+previousActive, "", 404)
}

func TestFullDevelopmentReset(t *testing.T) {
	pool := isolationPool(t)
	manager := auth.NewWithProvider(pool, auth.Config{Origin: "http://localhost:3000"}, isolationProvider{})
	handler := Handler(pool, manager, true)
	a := &accountBrowser{t: t, handler: handler}
	b := &accountBrowser{t: t, handler: handler}
	guest := &accountBrowser{t: t, handler: handler}
	a.login("alice")
	b.login("bob")
	for _, c := range []*accountBrowser{a, b} {
		w := c.request("POST", "/api/dev/reset", "", map[string]string{"X-Confirm-Reset": "replace-workout-data"})
		if w.Code != 200 {
			t.Fatal(w.Code, w.Body)
		}
	}
	staleA, staleB := *a.cookie, *b.cookie
	pending := &accountBrowser{t: t, handler: handler}
	pending.token()
	attempt := pending.request("POST", "/api/auth/google/start", url.Values{"csrf": {pending.csrf}}.Encode(), map[string]string{"Content-Type": "application/x-www-form-urlencoded"})
	guest.expect("POST", "/api/dev/reset-all", "", 401)
	a.expect("POST", "/api/dev/reset-all", "", 400)
	for _, headers := range []map[string]string{
		{"X-Confirm-Reset": "replace-workout-data"},
		{"X-Confirm-Reset": "delete-all-data-and-users", "X-CSRF-Token": ""},
		{"X-Confirm-Reset": "delete-all-data-and-users", "Origin": "https://evil.example"},
	} {
		w := a.request("POST", "/api/dev/reset-all", "", headers)
		if w.Code != 400 && w.Code != 403 {
			t.Fatal(w.Code, w.Body)
		}
	}
	disabled := &accountBrowser{t: t, handler: Handler(pool, manager, false), cookie: a.cookie, csrf: a.csrf}
	disabled.expect("POST", "/api/dev/reset-all", "", 404)
	// Force failure after TRUNCATE to prove all deletions roll back together.
	if _, err := pool.Exec(context.Background(), `CREATE FUNCTION reject_full_reset() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'private reset failure'; END $$; CREATE TRIGGER reject_full_reset BEFORE UPDATE ON auth_sessions FOR EACH ROW EXECUTE FUNCTION reject_full_reset()`); err != nil {
		t.Fatal(err)
	}
	failed := a.request("POST", "/api/dev/reset-all", "", map[string]string{"X-Confirm-Reset": "delete-all-data-and-users"})
	if failed.Code != 500 || strings.Contains(failed.Body.String(), "private reset failure") {
		t.Fatal(failed.Code, failed.Body)
	}
	var retained int
	if err := pool.QueryRow(context.Background(), "SELECT count(*) FROM accounts").Scan(&retained); err != nil || retained != 2 {
		t.Fatal("failed reset deleted accounts", retained, err)
	}
	b.expect("GET", "/api/sessions/active", "", 200)
	if _, err := pool.Exec(context.Background(), `DROP TRIGGER reject_full_reset ON auth_sessions; DROP FUNCTION reject_full_reset()`); err != nil {
		t.Fatal(err)
	}
	w := a.request("POST", "/api/dev/reset-all", "", map[string]string{"X-Confirm-Reset": "delete-all-data-and-users", "X-Crow-Activity": "foreground"})
	if w.Code != 204 {
		t.Fatal(w.Code, w.Body)
	}
	if !strings.Contains(w.Header().Get("Set-Cookie"), "Expires=") {
		t.Fatal("cookie not expired")
	}
	for _, table := range []string{"owners", "accounts", "exercises", "templates", "template_exercises", "template_exercise_alternatives", "workout_sessions", "session_exercises", "session_exercise_choices", "session_sets", "auth_attempts"} {
		var count int
		if err := pool.QueryRow(context.Background(), "SELECT count(*) FROM "+table).Scan(&count); err != nil || count != 0 {
			t.Fatal(table, count, err)
		}
	}
	var live int
	if err := pool.QueryRow(context.Background(), "SELECT count(*) FROM auth_sessions WHERE NOT revoked OR length(data)>0").Scan(&live); err != nil || live != 0 {
		t.Fatal("sessions remain live", live, err)
	}
	a.cookie = &staleA
	b.cookie = &staleB
	a.expect("GET", "/api/exercises", "", 401)
	b.expect("GET", "/api/auth/me", "", 401)
	pending.expect("GET", attempt.Header().Get("Location"), "", 303)
	// Re-signing in creates a fresh empty identity, without reusing deleted IDs.
	fresh := &accountBrowser{t: t, handler: handler}
	fresh.login("alice")
	if fresh.account == a.account {
		t.Fatal("account ID reused")
	}
	w = fresh.expect("GET", "/api/exercises?paginated=true", "", 200)
	var page struct {
		Total int `json:"total_items"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &page); err != nil || page.Total != 0 {
		t.Fatal(w.Body, err)
	}
}
