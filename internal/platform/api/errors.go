package api

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"net"
	"net/http"
	"strconv"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/marceloalvescf/capacity-room/internal/platform/auth"
	"github.com/marceloalvescf/capacity-room/internal/platform/reports"
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

	CodeProfileNotFound         Code = "PROFILE_NOT_FOUND"
	CodeProfileIDTaken          Code = "PROFILE_ID_TAKEN"
	CodeProfileRevisionStale    Code = "PROFILE_REVISION_STALE"
	CodeProfileDocumentInvalid  Code = "PROFILE_DOCUMENT_INVALID"
	CodeEndpointURLRejected     Code = "ENDPOINT_URL_REJECTED"
	CodeEndpointAddressRejected Code = "ENDPOINT_ADDRESS_REJECTED"
	CodeInternalError           Code = "INTERNAL_ERROR"

	CodeIdempotencyKeyInvalid Code = "IDEMPOTENCY_KEY_INVALID"
	CodeCursorInvalid         Code = "CURSOR_INVALID"
	CodeReportNotFound        Code = "REPORT_NOT_FOUND"
	CodeIdempotencyKeyReused  Code = "IDEMPOTENCY_KEY_REUSED"
	CodeProfileInactive       Code = "PROFILE_INACTIVE"
	CodeSnapshotNotReady      Code = "SNAPSHOT_NOT_READY"
	CodePDFNotReady           Code = "PDF_NOT_READY"
	CodePDFAlreadyReady       Code = "PDF_ALREADY_READY"
	CodeReportExpired         Code = "REPORT_EXPIRED"
	CodeSelectionNotPermitted Code = "REPORT_SELECTION_NOT_PERMITTED"
	CodeAdmissionGlobalLimit  Code = "ADMISSION_GLOBAL_LIMIT"
	CodeAdmissionProfileLimit Code = "ADMISSION_PROFILE_LIMIT"
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
	CodeRequestInvalid:          {400, "The request could not be parsed."},
	CodeInvalidCredentials:      {401, "The username or password is incorrect."},
	CodeUnauthenticated:         {401, "Authentication is required."},
	CodeCSRFRejected:            {403, "The request could not be verified."},
	CodeForbidden:               {403, "You are not allowed to perform this action."},
	CodeUserNotFound:            {404, "No such user was found."},
	CodeProfileNotFound:         {404, "No such profile was found."},
	CodeProfileIDTaken:          {409, "This profile id is already in use."},
	CodeProfileRevisionStale:    {409, "This profile changed since it was last read."},
	CodeProfileDocumentInvalid:  {422, "This profile request is not valid."},
	CodeEndpointURLRejected:     {422, "This endpoint URL is not allowed."},
	CodeEndpointAddressRejected: {422, "This endpoint address is not allowed."},
	CodeNotFound:                {404, "The requested resource was not found."},
	CodeMethodNotAllowed:        {405, "This method is not allowed for this resource."},
	CodeUsernameTaken:           {409, "This username is already in use."},
	CodeLastAdmin:               {409, "The last active administrator cannot be changed this way."},
	CodeSelfDelete:              {409, "You cannot remove your own account."},
	CodeSelfDeactivate:          {409, "You cannot deactivate your own account."},
	CodeSelfDemote:              {409, "You cannot demote your own account."},
	CodeInvalidUsername:         {422, "This username is not valid."},
	CodePasswordTooShort:        {422, "This password is too short."},
	CodePasswordTooLong:         {422, "This password is too long."},
	CodeAuthRateLimited:         {429, "Too many login attempts. Try again later."},
	CodeDatabaseUnavailable:     {503, "The database is temporarily unavailable."},
	CodeInternalError:           {500, "An unexpected error occurred."},

	CodeIdempotencyKeyInvalid: {400, "The Idempotency-Key header is not valid."},
	CodeCursorInvalid:         {400, "The cursor is not valid."},
	CodeReportNotFound:        {404, "No such report was found."},
	CodeIdempotencyKeyReused:  {409, "This idempotency key was already used for a different request."},
	CodeProfileInactive:       {409, "This profile is not active."},
	CodeSnapshotNotReady:      {409, "This report's data is not ready yet."},
	CodePDFNotReady:           {409, "This report's PDF is not ready yet."},
	CodePDFAlreadyReady:       {409, "This PDF is already ready."},
	CodeReportExpired:         {410, "This report has expired."},
	CodeSelectionNotPermitted: {422, "This selection is not permitted by the profile."},
	CodeAdmissionGlobalLimit:  {429, "The platform is at its global report admission limit."},
	CodeAdmissionProfileLimit: {429, "This profile is at its report admission limit."},
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
	var refused *reports.Error
	switch {
	case errors.As(err, &refused):
		return reportProblem(refused)
	case errors.As(err, &throttled):
		return &Problem{Code: CodeAuthRateLimited, RetryAfter: throttled.RetryAfter}
	case errors.Is(err, auth.ErrInvalidCredentials):
		return &Problem{Code: CodeInvalidCredentials}
	case errors.Is(err, auth.ErrUnauthenticated):
		return &Problem{Code: CodeUnauthenticated}
	case errors.Is(err, auth.ErrCSRF):
		return &Problem{Code: CodeCSRFRejected}
	case errors.Is(err, auth.ErrForbidden):
		return &Problem{Code: CodeForbidden}
	case errors.Is(err, auth.ErrUserNotFound):
		return &Problem{Code: CodeUserNotFound}
	case errors.Is(err, auth.ErrUsernameTaken):
		return &Problem{Code: CodeUsernameTaken}
	case errors.Is(err, auth.ErrLastAdmin):
		return &Problem{Code: CodeLastAdmin}
	case errors.Is(err, auth.ErrSelfDelete):
		return &Problem{Code: CodeSelfDelete}
	case errors.Is(err, auth.ErrSelfDeactivate):
		return &Problem{Code: CodeSelfDeactivate}
	case errors.Is(err, auth.ErrSelfDemote):
		return &Problem{Code: CodeSelfDemote}
	case errors.Is(err, auth.ErrInvalidUsername):
		return &Problem{Code: CodeInvalidUsername}
	case errors.Is(err, auth.ErrInvalidRole):
		return &Problem{Code: CodeRequestInvalid, Fields: map[string]string{"role": "invalid"}}
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

// reportProblem maps a refusal of the reports service to its problem. The
// two admission codes carry Retry-After in whole seconds, rounded up, at
// least 1.
func reportProblem(e *reports.Error) *Problem {
	p := &Problem{Code: Code(e.Code), Fields: e.Fields}
	if p.Code == CodeAdmissionGlobalLimit || p.Code == CodeAdmissionProfileLimit {
		p.RetryAfter = max(1, int(math.Ceil(e.RetryAfter.Seconds())))
	}
	return p
}
