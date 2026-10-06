package api

import (
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/marceloalvescf/k8s-capacity-report/internal/platform/auth"
	"github.com/marceloalvescf/k8s-capacity-report/internal/platform/reports"
)

var allCodes = []Code{
	CodeRequestInvalid, CodeInvalidCredentials, CodeUnauthenticated, CodeCSRFRejected,
	CodeForbidden, CodeUserNotFound, CodeNotFound, CodeMethodNotAllowed, CodeUsernameTaken,
	CodeLastAdmin, CodeSelfDelete, CodeSelfDeactivate, CodeSelfDemote, CodeInvalidUsername,
	CodePasswordTooShort, CodePasswordTooLong, CodeAuthRateLimited, CodeDatabaseUnavailable,
	CodeInternalError, CodeProfileNotFound, CodeProfileIDTaken, CodeProfileRevisionStale,
	CodeProfileDocumentInvalid, CodeEndpointURLRejected, CodeEndpointAddressRejected,
	CodeIdempotencyKeyInvalid, CodeCursorInvalid, CodeReportNotFound, CodeIdempotencyKeyReused,
	CodeProfileInactive, CodeSnapshotNotReady, CodePDFNotReady, CodePDFRetryNotAllowed,
	CodeReportExpired, CodeSelectionNotPermitted, CodeAdmissionGlobalLimit, CodeAdmissionProfileLimit,
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
		status int
		fields map[string]string
	}{
		{auth.ErrUsernameTaken, CodeUsernameTaken, 409, nil},
		{auth.ErrLastAdmin, CodeLastAdmin, 409, nil},
		{auth.ErrSelfDelete, CodeSelfDelete, 409, nil},
		{auth.ErrSelfDeactivate, CodeSelfDeactivate, 409, nil},
		{auth.ErrSelfDemote, CodeSelfDemote, 409, nil},
		{auth.ErrInvalidUsername, CodeInvalidUsername, 422, nil},
		{auth.ErrPasswordTooShort, CodePasswordTooShort, 422, nil},
		{auth.ErrPasswordTooLong, CodePasswordTooLong, 422, nil},
		{auth.ErrInvalidRole, CodeRequestInvalid, 400, map[string]string{"role": "invalid"}},
		{auth.ErrInvalidLanguage, CodeRequestInvalid, 400, map[string]string{"language": "invalid"}},
		{auth.ErrUserNotFound, CodeUserNotFound, 404, nil},
		{auth.ErrForbidden, CodeForbidden, 403, nil},
		{errors.New("unknown"), CodeInternalError, 500, nil},
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
		w := httptest.NewRecorder()
		WriteProblem(w, p)
		if w.Code != c.status {
			t.Errorf("%v: status %d, want %d", c.err, w.Code, c.status)
		}
	}
}

// TestErrorTableNamesEveryCode reads the code constants of the reports
// service from its source and finds each one in the table.
func TestErrorTableNamesEveryCode(t *testing.T) {
	file, err := parser.ParseFile(token.NewFileSet(), "../reports/service.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	var codes []string
	for _, decl := range file.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || gen.Tok != token.CONST {
			continue
		}
		for _, spec := range gen.Specs {
			v := spec.(*ast.ValueSpec)
			for i, name := range v.Names {
				if !strings.HasPrefix(name.Name, "Code") {
					continue
				}
				lit, ok := v.Values[i].(*ast.BasicLit)
				if !ok {
					t.Fatalf("%s is not a literal", name.Name)
				}
				value, err := strconv.Unquote(lit.Value)
				if err != nil {
					t.Fatal(err)
				}
				codes = append(codes, value)
			}
		}
	}
	if len(codes) < 14 {
		t.Fatalf("read %d codes: %v", len(codes), codes)
	}
	for _, code := range codes {
		if _, ok := problems[Code(code)]; !ok {
			t.Errorf("the reports service returns %s, which the table does not hold", code)
		}
	}
}

func TestReportProblemRoundsRetryAfterUp(t *testing.T) {
	for _, c := range []struct {
		after time.Duration
		want  int
	}{
		{200 * time.Millisecond, 1}, {time.Second, 1}, {1200 * time.Millisecond, 2}, {5 * time.Second, 5},
	} {
		p := reportProblem(&reports.Error{Code: reports.CodeAdmissionGlobalLimit, RetryAfter: c.after})
		if p.RetryAfter != c.want {
			t.Errorf("%v: Retry-After %d, want %d", c.after, p.RetryAfter, c.want)
		}
	}
}
