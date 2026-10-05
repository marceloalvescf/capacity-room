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

func TestProblemOf(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want Code
	}{
		{"throttled", &auth.ErrThrottled{RetryAfter: 42}, CodeAuthRateLimited},
		{"credentials", auth.ErrInvalidCredentials, CodeInvalidCredentials},
		{"unauthenticated", auth.ErrUnauthenticated, CodeUnauthenticated},
		{"csrf", auth.ErrCSRF, CodeCSRFRejected},
		{"short", auth.ErrPasswordTooShort, CodePasswordTooShort},
		{"long", auth.ErrPasswordTooLong, CodePasswordTooLong},
		{"language", auth.ErrInvalidLanguage, CodeRequestInvalid},
		{"deadline", context.DeadlineExceeded, CodeDatabaseUnavailable},
		{"net", &net.OpError{Op: "dial", Err: errors.New("refused")}, CodeDatabaseUnavailable},
		{"unknown", errors.New("other"), CodeInternalError},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			p := problemOf(c.err)
			if p.Code != c.want {
				t.Fatalf("got %s, want %s", p.Code, c.want)
			}
			if c.name == "throttled" && p.RetryAfter != 42 {
				t.Fatalf("RetryAfter %d", p.RetryAfter)
			}
			if c.name == "language" && p.Fields["language"] != "invalid" {
				t.Fatalf("fields %v", p.Fields)
			}
			if status := problems[p.Code].Status; c.name == "short" && status != 422 {
				t.Fatalf("status %d", status)
			}
		})
	}
}
