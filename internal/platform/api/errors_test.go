package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
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
