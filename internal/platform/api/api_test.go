//go:build postgres

package api

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/marceloalvescf/k8s-capacity-report/internal/platform/auth"
	"github.com/marceloalvescf/k8s-capacity-report/internal/platform/netpolicy"
	"github.com/marceloalvescf/k8s-capacity-report/internal/platform/profiles"
	"github.com/marceloalvescf/k8s-capacity-report/internal/platform/reports"
	"github.com/marceloalvescf/k8s-capacity-report/internal/platform/settings"
	"github.com/marceloalvescf/k8s-capacity-report/internal/platform/store"
	"github.com/marceloalvescf/k8s-capacity-report/internal/platform/storetest"
)

const (
	rootPassword = "root-password-1"
	evePassword  = "eve-password-12"
)

// testServer is a Server on a fresh database with two seeded accounts and a
// clock the test moves.
type testServer struct {
	*Server
	pool *pgxpool.Pool
	now  time.Time
	eve  store.User
	rev  uuid.UUID // the current revision of the profile "prod", once seeded
}

func newTestServer(t *testing.T) *testServer {
	t.Helper()
	pool := storetest.New(t)
	ts := &testServer{pool: pool, now: time.Date(2026, 10, 5, 12, 0, 0, 0, time.UTC)}
	now := func() time.Time { return ts.now }
	sessions := &auth.Sessions{
		Pool:     pool,
		Throttle: auth.Throttle{Secret: []byte("0123456789abcdef0123456789abcdef")},
		TTL:      12 * time.Hour,
		Idle:     time.Hour,
		Now:      now,
	}
	ts.Server = &Server{
		Pool:     pool,
		Settings: settings.Settings{SessionTTL: 12 * time.Hour, SessionIdle: time.Hour},
		Sessions: sessions,
		Users:    &auth.Users{Pool: pool, Now: now},
		Account:  &auth.Account{Pool: pool, Sessions: sessions, Now: now},
		Profiles: &profiles.Service{
			Pool:     pool,
			Policy:   netpolicy.Policy{Networks: []netip.Prefix{netip.MustParsePrefix("127.0.0.1/32")}},
			Settings: settings.Settings{CredentialDir: t.TempDir(), CABundleDir: t.TempDir()},
			Now:      now,
			Resolve: func(_ context.Context, host string) ([]netip.Addr, error) {
				if host != "metrics.test" {
					t.Errorf("resolver asked for %q", host)
					return nil, errors.New("unexpected host")
				}
				return []netip.Addr{netip.MustParseAddr("127.0.0.1")}, nil
			},
		},
		Now: now,
	}
	queue, err := reports.NewQueue(pool, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	ts.Reports = &reports.Service{
		Pool: pool, Queue: queue, Now: now,
		Settings: settings.Settings{
			JobMaxAttempts: 3, ReportRetention: 7 * 24 * time.Hour,
			AdmissionLimit: 20, AdmissionProfileLimit: 5, AdmissionRetry: 5 * time.Second,
		},
	}
	seed(t, pool, ts.now, "root", rootPassword, auth.RoleAdministrator)
	ts.eve = seed(t, pool, ts.now, "eve", evePassword, auth.RoleReader)
	return ts
}

func seed(t *testing.T, pool *pgxpool.Pool, now time.Time, name, password, role string) store.User {
	t.Helper()
	hash, err := auth.HashPassword(password)
	if err != nil {
		t.Fatal(err)
	}
	u := store.User{
		ID: uuid.New(), Username: name, PasswordHash: hash, Role: role,
		Language: "pt-BR", Active: true, CreatedAt: now, UpdatedAt: now,
	}
	if err := store.InsertUser(context.Background(), pool, u); err != nil {
		t.Fatal(err)
	}
	return u
}

// call sends one request through the handler.
func (ts *testServer) call(t *testing.T, method, path, body string, header map[string]string, cookies ...*http.Cookie) *httptest.ResponseRecorder {
	t.Helper()
	var r *http.Request
	if body == "" {
		r = httptest.NewRequest(method, path, nil)
	} else {
		r = httptest.NewRequest(method, path, strings.NewReader(body))
	}
	for k, v := range header {
		r.Header.Set(k, v)
	}
	for _, c := range cookies {
		r.AddCookie(c)
	}
	w := httptest.NewRecorder()
	ts.Handler().ServeHTTP(w, r)
	return w
}

func cookieNamed(w *httptest.ResponseRecorder, name string) *http.Cookie {
	for _, c := range w.Result().Cookies() {
		if c.Name == name {
			return c
		}
	}
	return nil
}

// loginCSRF runs GET /auth/session as an anonymous caller and returns the
// sign-in cookie.
func (ts *testServer) loginCSRF(t *testing.T) *http.Cookie {
	t.Helper()
	w := ts.call(t, "GET", "/api/v1/auth/session", "", nil)
	c := cookieNamed(w, LoginCSRFCookie)
	if w.Code != 200 || c == nil {
		t.Fatalf("anonymous session: %d %s", w.Code, w.Body.String())
	}
	return c
}

func signInBody(username, password string) string {
	b, _ := json.Marshal(LoginRequest{Username: username, Password: password})
	return string(b)
}

// login runs the sign-in with the given password.
func (ts *testServer) login(t *testing.T, username, password string) *httptest.ResponseRecorder {
	t.Helper()
	c := ts.loginCSRF(t)
	return ts.call(t, "POST", "/api/v1/auth/login", signInBody(username, password), map[string]string{CSRFHeaderName: c.Value}, c)
}

// signIn returns the session cookie and the CSRF token of a new session.
func (ts *testServer) signIn(t *testing.T, username string) (*http.Cookie, string) {
	t.Helper()
	password := rootPassword
	if username == "eve" {
		password = evePassword
	}
	w := ts.login(t, username, password)
	if w.Code != 200 {
		t.Fatalf("sign in: %d %s", w.Code, w.Body.String())
	}
	var s SignedInSession
	if err := json.Unmarshal(w.Body.Bytes(), &s); err != nil {
		t.Fatal(err)
	}
	return cookieNamed(w, SessionCookie), s.CsrfToken
}

func (ts *testServer) count(t *testing.T, query string, args ...any) int {
	t.Helper()
	var n int
	if err := ts.pool.QueryRow(context.Background(), query, args...).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

// wantProblem checks the status, the code, and the fields of an answer.
func wantProblem(t *testing.T, w *httptest.ResponseRecorder, status int, code Code, fields map[string]string) {
	t.Helper()
	var body struct {
		Error struct {
			Code   Code              `json:"code"`
			Fields map[string]string `json:"fields"`
		} `json:"error"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("body %q: %v", w.Body.String(), err)
	}
	if w.Code != status || body.Error.Code != code {
		t.Fatalf("got %d %s, want %d %s: %s", w.Code, body.Error.Code, status, code, w.Body.String())
	}
	if fields != nil {
		if len(body.Error.Fields) != len(fields) {
			t.Fatalf("fields %v, want %v", body.Error.Fields, fields)
		}
		for k, v := range fields {
			if body.Error.Fields[k] != v {
				t.Fatalf("fields %v, want %v", body.Error.Fields, fields)
			}
		}
	}
}
