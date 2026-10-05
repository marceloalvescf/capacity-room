package api

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/marceloalvescf/k8s-capacity-report/internal/platform/auth"
)

var allCodes = []Code{
	CodeRequestInvalid, CodeInvalidCredentials, CodeUnauthenticated, CodeCSRFRejected,
	CodeForbidden, CodeUserNotFound, CodeNotFound, CodeMethodNotAllowed, CodeUsernameTaken,
	CodeLastAdmin, CodeSelfDelete, CodeSelfDeactivate, CodeSelfDemote, CodeInvalidUsername,
	CodePasswordTooShort, CodePasswordTooLong, CodeAuthRateLimited, CodeDatabaseUnavailable,
	CodeInternalError,
}

func TestProblemTableIsComplete(t *testing.T) {
	if len(problems) != len(allCodes) {
		t.Fatalf("table holds %d codes, want %d", len(problems), len(allCodes))
	}
	for _, c := range allCodes {
		e, ok := problems[c]
		if !ok {
			t.Fatalf("no entry for %s", c)
		}
		if e.Status < 400 || !strings.HasSuffix(e.Sentence, ".") {
			t.Errorf("%s: bad entry %+v", c, e)
		}
	}
}

func TestWriteProblemUnknownCodeIsInternalError(t *testing.T) {
	w := httptest.NewRecorder()
	WriteProblem(w, &Problem{Code: "NOPE"})
	if w.Code != 500 || !strings.Contains(w.Body.String(), `"code":"INTERNAL_ERROR"`) {
		t.Fatalf("%d %q", w.Code, w.Body.String())
	}
}

func TestWriteProblemShape(t *testing.T) {
	w := httptest.NewRecorder()
	WriteProblem(w, &Problem{Code: CodeForbidden})
	if w.Code != http.StatusForbidden {
		t.Fatalf("status %d", w.Code)
	}
	if got := w.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("content type %q", got)
	}
	want := `{"error":{"code":"FORBIDDEN","message":"You are not allowed to perform this action.","fields":{}}}` + "\n"
	if w.Body.String() != want {
		t.Fatalf("body %q", w.Body.String())
	}
}

func TestWriteProblemFieldsAndRetryAfter(t *testing.T) {
	w := httptest.NewRecorder()
	WriteProblem(w, &Problem{Code: CodeAuthRateLimited, Fields: map[string]string{"username": "missing"}, RetryAfter: 7})
	if !strings.Contains(w.Body.String(), `"fields":{"username":"missing"}`) {
		t.Fatalf("body %q", w.Body.String())
	}
	if got := w.Header().Get("Retry-After"); got != "7" {
		t.Fatalf("Retry-After %q", got)
	}
	if w.Code != 429 {
		t.Fatalf("status %d", w.Code)
	}
}

func TestUsersErrorMapping(t *testing.T) {
	cases := []struct {
		err    error
		code   Code
		fields map[string]string
	}{
		{auth.ErrUsernameTaken, CodeUsernameTaken, nil},
		{auth.ErrLastAdmin, CodeLastAdmin, nil},
		{auth.ErrSelfDelete, CodeSelfDelete, nil},
		{auth.ErrSelfDeactivate, CodeSelfDeactivate, nil},
		{auth.ErrSelfDemote, CodeSelfDemote, nil},
		{auth.ErrInvalidUsername, CodeInvalidUsername, nil},
		{auth.ErrPasswordTooShort, CodePasswordTooShort, nil},
		{auth.ErrPasswordTooLong, CodePasswordTooLong, nil},
		{auth.ErrInvalidRole, CodeRequestInvalid, map[string]string{"role": "invalid"}},
		{auth.ErrInvalidLanguage, CodeRequestInvalid, map[string]string{"language": "invalid"}},
		{auth.ErrUserNotFound, CodeUserNotFound, nil},
		{auth.ErrForbidden, CodeForbidden, nil},
		{errors.New("unknown"), CodeInternalError, nil},
	}
	for _, c := range cases {
		p := problemOf(c.err)
		if p.Code != c.code || len(p.Fields) != len(c.fields) {
			t.Errorf("%v: got %s %v, want %s %v", c.err, p.Code, p.Fields, c.code, c.fields)
		}
		for k, v := range c.fields {
			if p.Fields[k] != v {
				t.Errorf("%v: fields %v, want %v", c.err, p.Fields, c.fields)
			}
		}
	}
}
