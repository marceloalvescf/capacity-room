package api

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"strconv"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/marceloalvescf/k8s-capacity-report/internal/platform/auth"
)

// Code is an error code of the contract. The table below holds its status
// and sentence.
type Code string

// The codes of the contract.
const (
	CodeRequestInvalid      Code = "REQUEST_INVALID"
	CodeInvalidCredentials  Code = "INVALID_CREDENTIALS"
	CodeUnauthenticated     Code = "UNAUTHENTICATED"
	CodeCSRFRejected        Code = "CSRF_REJECTED"
	CodeForbidden           Code = "FORBIDDEN"
	CodeUserNotFound        Code = "USER_NOT_FOUND"
	CodeNotFound            Code = "NOT_FOUND"
	CodeMethodNotAllowed    Code = "METHOD_NOT_ALLOWED"
	CodeUsernameTaken       Code = "USERNAME_TAKEN"
	CodeLastAdmin           Code = "LAST_ADMIN"
	CodeSelfDelete          Code = "SELF_DELETE"
	CodeSelfDeactivate      Code = "SELF_DEACTIVATE"
	CodeSelfDemote          Code = "SELF_DEMOTE"
	CodeInvalidUsername     Code = "INVALID_USERNAME"
	CodePasswordTooShort    Code = "PASSWORD_TOO_SHORT"
	CodePasswordTooLong     Code = "PASSWORD_TOO_LONG"
	CodeAuthRateLimited     Code = "AUTH_RATE_LIMITED"
	CodeDatabaseUnavailable Code = "DATABASE_UNAVAILABLE"
	CodeInternalError       Code = "INTERNAL_ERROR"
)

// Problem is one failure to answer.
type Problem struct {
	Code       Code
	Fields     map[string]string // nil means {}
	RetryAfter int               // seconds; written as Retry-After when > 0
}

// Error returns the code, so that a Problem can travel as an error.
func (p *Problem) Error() string { return string(p.Code) }

// problems holds the status and the sentence of each code.
var problems = map[Code]struct {
	Status   int
	Sentence string
}{
	CodeRequestInvalid:      {400, "The request could not be parsed."},
	CodeInvalidCredentials:  {401, "The username or password is incorrect."},
	CodeUnauthenticated:     {401, "Authentication is required."},
	CodeCSRFRejected:        {403, "The request could not be verified."},
	CodeForbidden:           {403, "You are not allowed to perform this action."},
	CodeUserNotFound:        {404, "No such user was found."},
	CodeNotFound:            {404, "The requested resource was not found."},
	CodeMethodNotAllowed:    {405, "This method is not allowed for this resource."},
	CodeUsernameTaken:       {409, "This username is already in use."},
	CodeLastAdmin:           {409, "The last active administrator cannot be changed this way."},
	CodeSelfDelete:          {409, "You cannot remove your own account."},
	CodeSelfDeactivate:      {409, "You cannot deactivate your own account."},
	CodeSelfDemote:          {409, "You cannot demote your own account."},
	CodeInvalidUsername:     {422, "This username is not valid."},
	CodePasswordTooShort:    {422, "This password is too short."},
	CodePasswordTooLong:     {422, "This password is too long."},
	CodeAuthRateLimited:     {429, "Too many login attempts. Try again later."},
	CodeDatabaseUnavailable: {503, "The database is temporarily unavailable."},
	CodeInternalError:       {500, "An unexpected error occurred."},
}

type envelope struct {
	Error envelopeError `json:"error"`
}

type envelopeError struct {
	Code    Code              `json:"code"`
	Message string            `json:"message"`
	Fields  map[string]string `json:"fields"`
}

// codeNoter is implemented by the response writer of SecurityHeaders, which
// logs the code of a failure.
type codeNoter interface{ noteCode(Code) }

// errorNoter is implemented by the same writer, which keeps the error behind
// a failure for the log line.
type errorNoter interface{ noteError(error) }

// WriteProblem writes the envelope with the status and sentence of the
// table, and the Retry-After header when the problem carries one.
func WriteProblem(w http.ResponseWriter, p *Problem) {
	entry, known := problems[p.Code]
	if !known {
		p = &Problem{Code: CodeInternalError, Fields: p.Fields, RetryAfter: p.RetryAfter}
		entry = problems[CodeInternalError]
	}
	fields := p.Fields
	if fields == nil {
		fields = map[string]string{}
	}
	body, err := json.Marshal(envelope{envelopeError{p.Code, entry.Sentence, fields}})
	if err != nil {
		body = []byte(`{"error":{"code":"INTERNAL_ERROR","message":"An unexpected error occurred.","fields":{}}}`)
		entry.Status = http.StatusInternalServerError
	}
	if n, ok := w.(codeNoter); ok {
		n.noteCode(p.Code)
	}
	w.Header().Set("Content-Type", "application/json")
	if p.RetryAfter > 0 {
		w.Header().Set("Retry-After", strconv.Itoa(p.RetryAfter))
	}
	w.WriteHeader(entry.Status)
	_, _ = w.Write(append(body, '\n'))
}

// problemOf maps an error of the services to the problem that answers it.
// An error it does not know is INTERNAL_ERROR.
func problemOf(err error) *Problem {
	var throttled *auth.ErrThrottled
	var netErr *net.OpError
	switch {
	case errors.As(err, &throttled):
		return &Problem{Code: CodeAuthRateLimited, RetryAfter: throttled.RetryAfter}
	case errors.Is(err, auth.ErrInvalidCredentials):
		return &Problem{Code: CodeInvalidCredentials}
	case errors.Is(err, auth.ErrUnauthenticated):
		return &Problem{Code: CodeUnauthenticated}
	case errors.Is(err, auth.ErrCSRF):
		return &Problem{Code: CodeCSRFRejected}
	case errors.Is(err, auth.ErrPasswordTooShort):
		return &Problem{Code: CodePasswordTooShort}
	case errors.Is(err, auth.ErrPasswordTooLong):
		return &Problem{Code: CodePasswordTooLong}
	case errors.Is(err, auth.ErrInvalidLanguage):
		return &Problem{Code: CodeRequestInvalid, Fields: map[string]string{"language": "invalid"}}
	case errors.Is(err, context.DeadlineExceeded), pgconn.SafeToRetry(err), errors.As(err, &netErr):
		return &Problem{Code: CodeDatabaseUnavailable}
	}
	return &Problem{Code: CodeInternalError}
}
