package api

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/marceloalvescf/k8s-capacity-report/internal/platform/auth"
)

func TestFailureIsLoggedOnce(t *testing.T) {
	var buf bytes.Buffer
	s := &Server{Logger: slog.New(slog.NewTextHandler(&buf, nil))}
	s.extraRoutes = []Route{{Method: "GET", Pattern: "/api/v1/probe", Handler: func(w http.ResponseWriter, r *http.Request) {
		s.fail(w, r, errors.New("disk on fire"))
	}}}
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, httptest.NewRequest("GET", "/api/v1/probe", nil))
	if w.Code != 500 {
		t.Fatalf("%d", w.Code)
	}
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 1 {
		t.Fatalf("%d log lines: %s", len(lines), buf.String())
	}
	for _, want := range []string{"method=GET", `route="GET /api/v1/probe"`, "status=500", "code=INTERNAL_ERROR", `err="disk on fire"`} {
		if !strings.Contains(lines[0], want) {
			t.Errorf("log line lacks %s: %s", want, lines[0])
		}
	}
}

func TestPanicAfterTheResponseIsLogged(t *testing.T) {
	var buf bytes.Buffer
	s := &Server{Logger: slog.New(slog.NewTextHandler(&buf, nil))}
	s.extraRoutes = []Route{{Method: "GET", Pattern: "/api/v1/probe", Handler: func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		panic("late boom")
	}}}
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, httptest.NewRequest("GET", "/api/v1/probe", nil))
	if w.Code != 200 {
		t.Fatalf("%d", w.Code)
	}
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 1 {
		t.Fatalf("%d log lines: %s", len(lines), buf.String())
	}
	for _, want := range []string{"level=ERROR", "method=GET", `route="GET /api/v1/probe"`, `panic="late boom"`} {
		if !strings.Contains(lines[0], want) {
			t.Errorf("log line lacks %s: %s", want, lines[0])
		}
	}
}

func TestProblemOf(t *testing.T) {
	cases := []struct {
		name   string
		err    error
		want   Code
		status int
	}{
		{"throttled", &auth.ErrThrottled{RetryAfter: 42}, CodeAuthRateLimited, 429},
		{"credentials", auth.ErrInvalidCredentials, CodeInvalidCredentials, 401},
		{"unauthenticated", auth.ErrUnauthenticated, CodeUnauthenticated, 401},
		{"csrf", auth.ErrCSRF, CodeCSRFRejected, 403},
		{"short", auth.ErrPasswordTooShort, CodePasswordTooShort, 422},
		{"long", auth.ErrPasswordTooLong, CodePasswordTooLong, 422},
		{"language", auth.ErrInvalidLanguage, CodeRequestInvalid, 400},
		{"deadline", context.DeadlineExceeded, CodeDatabaseUnavailable, 503},
		{"net", &net.OpError{Op: "dial", Err: errors.New("refused")}, CodeDatabaseUnavailable, 503},
		{"unknown", errors.New("other"), CodeInternalError, 500},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			p := problemOf(c.err)
			if p.Code != c.want {
				t.Fatalf("got %s, want %s", p.Code, c.want)
			}
			w := httptest.NewRecorder()
			WriteProblem(w, p)
			if w.Code != c.status {
				t.Fatalf("status %d, want %d", w.Code, c.status)
			}
			if c.name == "throttled" && (p.RetryAfter != 42 || w.Header().Get("Retry-After") != "42") {
				t.Fatalf("RetryAfter %d, header %q", p.RetryAfter, w.Header().Get("Retry-After"))
			}
			if c.name == "language" && p.Fields["language"] != "invalid" {
				t.Fatalf("fields %v", p.Fields)
			}
		})
	}
}
