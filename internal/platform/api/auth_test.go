//go:build postgres

package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/marceloalvescf/k8s-capacity-report/internal/platform/auth"
)

func TestSessionAnonymous(t *testing.T) {
	ts := newTestServer(t)
	w := ts.call(t, "GET", "/api/v1/auth/session", "", nil)
	var s AnonymousSession
	if err := json.Unmarshal(w.Body.Bytes(), &s); err != nil {
		t.Fatal(err)
	}
	if w.Code != 200 || s.Authenticated || len(s.LoginCsrf) != 43 {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
	got := w.Header().Values("Set-Cookie")
	want := LoginCSRFCookie + "=" + s.LoginCsrf + "; Path=/api; HttpOnly; Secure; SameSite=Lax"
	if len(got) != 1 || got[0] != want {
		t.Fatalf("Set-Cookie %q, want %q", got, want)
	}
}

func TestLoginRequiresLoginCSRF(t *testing.T) {
	ts := newTestServer(t)
	c := ts.loginCSRF(t)
	body := signInBody("root", rootPassword)
	cases := map[string]*struct {
		header  map[string]string
		cookies []*http.Cookie
	}{
		"no cookie":        {map[string]string{CSRFHeaderName: c.Value}, nil},
		"cookie, no token": {nil, []*http.Cookie{c}},
		"different token":  {map[string]string{CSRFHeaderName: c.Value + "x"}, []*http.Cookie{c}},
		"nothing":          {nil, nil},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			w := ts.call(t, "POST", "/api/v1/auth/login", body, tc.header, tc.cookies...)
			wantProblem(t, w, 403, CodeCSRFRejected, nil)
		})
	}
	if n := ts.count(t, "SELECT count(*) FROM sessions"); n != 0 {
		t.Fatalf("%d session rows", n)
	}
}

func TestLoginSucceeds(t *testing.T) {
	ts := newTestServer(t)
	w := ts.login(t, "root", rootPassword)
	if w.Code != 200 {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
	var s SignedInSession
	if err := json.Unmarshal(w.Body.Bytes(), &s); err != nil {
		t.Fatal(err)
	}
	if !s.Authenticated || s.Principal.Role != "administrator" || s.Principal.Username != "root" ||
		s.Principal.Language != "pt-BR" || s.Principal.Id == uuid.Nil || s.CsrfToken == "" {
		t.Fatalf("shape %+v", s)
	}
	if !strings.Contains(w.Body.String(), `"expires_at":"2026-10-05T13:00:00Z"`) {
		t.Fatalf("expires_at in %s", w.Body.String())
	}
	if !s.ExpiresAt.Equal(ts.now.Add(time.Hour)) {
		t.Fatalf("expires_at %v", s.ExpiresAt)
	}
	session := cookieNamed(w, SessionCookie)
	if session == nil {
		t.Fatal("no session cookie")
	}
	cookies := w.Header().Values("Set-Cookie")
	wantSession := SessionCookie + "=" + session.Value + "; Path=/api; Max-Age=43200; HttpOnly; Secure; SameSite=Lax"
	wantCleared := LoginCSRFCookie + "=; Path=/api; Max-Age=0; HttpOnly; Secure; SameSite=Lax"
	if len(cookies) != 2 || !contains(cookies, wantSession) || !contains(cookies, wantCleared) {
		t.Fatalf("Set-Cookie %q", cookies)
	}
}

func contains(list []string, want string) bool {
	for _, s := range list {
		if s == want {
			return true
		}
	}
	return false
}

func TestLoginWrongPassword(t *testing.T) {
	ts := newTestServer(t)
	w := ts.login(t, "root", "not-the-password")
	wantProblem(t, w, 401, CodeInvalidCredentials, nil)
	if cookieNamed(w, SessionCookie) != nil {
		t.Fatal("a session cookie was set")
	}
}

func TestLoginBodyRefusals(t *testing.T) {
	ts := newTestServer(t)
	c := ts.loginCSRF(t)
	cases := []struct {
		name   string
		body   string
		fields map[string]string
	}{
		{"missing password", `{"username":"root"}`, map[string]string{"password": "missing"}},
		{"unknown key", `{"username":"root","password":"x","extra":1}`, map[string]string{"extra": "unknown"}},
		{"long password", `{"username":"root","password":"` + strings.Repeat("a", 1025) + `"}`, map[string]string{"password": "too_long"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := ts.call(t, "POST", "/api/v1/auth/login", tc.body, map[string]string{CSRFHeaderName: c.Value}, c)
			wantProblem(t, w, 400, CodeRequestInvalid, tc.fields)
		})
	}
	if n := ts.count(t, "SELECT count(*) FROM audit_events"); n != 0 {
		t.Fatalf("%d audit rows", n)
	}
}

func TestLoginThrottled(t *testing.T) {
	ts := newTestServer(t)
	for range 5 {
		wantProblem(t, ts.login(t, "root", "wrong-password"), 401, CodeInvalidCredentials, nil)
	}
	w := ts.login(t, "root", rootPassword)
	wantProblem(t, w, 429, CodeAuthRateLimited, nil)
	if got := w.Header().Get("Retry-After"); got != "300" {
		t.Fatalf("Retry-After %q", got)
	}
	if !strings.Contains(w.Body.String(), "Too many login attempts. Try again later.") {
		t.Fatalf("body %s", w.Body.String())
	}
}

func TestLoginCSRFBeforeThrottle(t *testing.T) {
	ts := newTestServer(t)
	for range 5 {
		ts.login(t, "root", "wrong-password")
	}
	c := ts.loginCSRF(t)
	w := ts.call(t, "POST", "/api/v1/auth/login", signInBody("root", rootPassword), map[string]string{CSRFHeaderName: "bad"}, c)
	wantProblem(t, w, 403, CodeCSRFRejected, nil)
}

func TestSessionSignedIn(t *testing.T) {
	ts := newTestServer(t)
	session, csrf := ts.signIn(t, "root")
	w := ts.call(t, "GET", "/api/v1/auth/session", "", nil, session)
	var s SignedInSession
	if err := json.Unmarshal(w.Body.Bytes(), &s); err != nil {
		t.Fatal(err)
	}
	if w.Code != 200 || !s.Authenticated || s.Principal.Username != "root" || s.CsrfToken == "" || s.CsrfToken == csrf {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
	if err := ts.Sessions.VerifyCSRF(t.Context(), mustSessionID(t, ts, session), s.CsrfToken); err != nil {
		t.Fatalf("new token does not verify: %v", err)
	}
	if err := ts.Sessions.VerifyCSRF(t.Context(), mustSessionID(t, ts, session), csrf); err == nil {
		t.Fatal("old token still verifies")
	}
}

func mustSessionID(t *testing.T, ts *testServer, session *http.Cookie) (id uuid.UUID) {
	t.Helper()
	a, err := ts.Sessions.Authenticate(t.Context(), session.Value)
	if err != nil {
		t.Fatal(err)
	}
	return a.SessionID
}

func TestLogout(t *testing.T) {
	ts := newTestServer(t)
	session, csrf := ts.signIn(t, "root")
	w := ts.call(t, "POST", "/api/v1/auth/logout", "", map[string]string{CSRFHeaderName: csrf}, session)
	if w.Code != 204 || w.Body.Len() != 0 {
		t.Fatalf("%d %q", w.Code, w.Body.String())
	}
	cleared := cookieNamed(w, SessionCookie)
	if cleared == nil || cleared.MaxAge != -1 || cleared.Value != "" {
		t.Fatalf("Set-Cookie %q", w.Header().Values("Set-Cookie"))
	}
	if n := ts.count(t, "SELECT count(*) FROM sessions WHERE revoked_reason = 'logout'"); n != 1 {
		t.Fatalf("%d sessions revoked by logout", n)
	}
	var s AnonymousSession
	again := ts.call(t, "GET", "/api/v1/auth/session", "", nil, session)
	if err := json.Unmarshal(again.Body.Bytes(), &s); err != nil || again.Code != 200 || s.Authenticated {
		t.Fatalf("%d %s", again.Code, again.Body.String())
	}
}

func TestLogoutRequiresCSRF(t *testing.T) {
	ts := newTestServer(t)
	session, _ := ts.signIn(t, "root")
	wantProblem(t, ts.call(t, "POST", "/api/v1/auth/logout", "", nil, session), 403, CodeCSRFRejected, nil)
}

func TestLogoutWithoutSession(t *testing.T) {
	ts := newTestServer(t)
	wantProblem(t, ts.call(t, "POST", "/api/v1/auth/logout", "", map[string]string{CSRFHeaderName: "x"}), 401, CodeUnauthenticated, nil)
}

func TestDeactivatedUserIsSignedOutOnNextRequest(t *testing.T) {
	ts := newTestServer(t)
	session, _ := ts.signIn(t, "eve")
	if _, err := ts.pool.Exec(t.Context(), "UPDATE users SET active = false WHERE id = $1", ts.eve.ID); err != nil {
		t.Fatal(err)
	}
	w := ts.call(t, "GET", "/api/v1/auth/session", "", nil, session)
	var s AnonymousSession
	if err := json.Unmarshal(w.Body.Bytes(), &s); err != nil || w.Code != 200 || s.Authenticated || s.LoginCsrf == "" {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
	if n := ts.count(t, "SELECT count(*) FROM sessions WHERE revoked_reason = 'user_inactive'"); n != 1 {
		t.Fatalf("%d sessions revoked as inactive", n)
	}
	cleared := cookieNamed(w, SessionCookie)
	if cleared == nil || cleared.MaxAge != -1 {
		t.Fatalf("Set-Cookie %q", w.Header().Values("Set-Cookie"))
	}
}

func TestRequireRole(t *testing.T) {
	ts := newTestServer(t)
	ts.extraRoutes = []Route{{Method: "GET", Pattern: "/api/v1/probe", Handler: ts.requireRole([]auth.Role{auth.RoleAdministrator},
		func(w http.ResponseWriter, _ *http.Request, _ auth.Authenticated) {
			w.WriteHeader(http.StatusNoContent)
		})}}
	reader, _ := ts.signIn(t, "eve")
	admin, _ := ts.signIn(t, "root")
	wantProblem(t, ts.call(t, "GET", "/api/v1/probe", "", nil, reader), 403, CodeForbidden, nil)
	if w := ts.call(t, "GET", "/api/v1/probe", "", nil, admin); w.Code != 204 {
		t.Fatalf("%d", w.Code)
	}
	wantProblem(t, ts.call(t, "GET", "/api/v1/probe", "", nil), 401, CodeUnauthenticated, nil)
}
