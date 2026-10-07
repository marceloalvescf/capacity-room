package reports

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"

	"github.com/marceloalvescf/k8s-capacity-report/internal/platform/audit"
	"github.com/marceloalvescf/k8s-capacity-report/internal/platform/auth"
	"github.com/marceloalvescf/k8s-capacity-report/internal/platform/settings"
	"github.com/marceloalvescf/k8s-capacity-report/internal/platform/store"
)

// The audit actions of the reports, with target type report.
const (
	ActionReportSubmit   = "report.submit"
	ActionReportCollect  = "report.collect"
	ActionReportRender   = "report.render"
	ActionReportPDFRetry = "report.pdf_retry"
	ActionReportExpire   = "report.expire"
	ActionReportPurge    = "report.purge"
)

const targetReport = "report"

// The codes of an Error, as the API carries them.
const (
	CodeRequestInvalid        = "REQUEST_INVALID"
	CodeReportNotFound        = "REPORT_NOT_FOUND"
	CodeReportExpired         = "REPORT_EXPIRED"
	CodeSnapshotNotReady      = "SNAPSHOT_NOT_READY"
	CodePDFNotReady           = "PDF_NOT_READY"
	CodePDFRetryNotAllowed    = "PDF_RETRY_NOT_ALLOWED"
	CodeProfileNotFound       = "PROFILE_NOT_FOUND"
	CodeProfileInactive       = "PROFILE_INACTIVE"
	CodeIdempotencyKeyInvalid = "IDEMPOTENCY_KEY_INVALID"
	CodeIdempotencyKeyReused  = "IDEMPOTENCY_KEY_REUSED"
	CodeSelectionNotPermitted = "REPORT_SELECTION_NOT_PERMITTED"
	CodeAdmissionGlobalLimit  = "ADMISSION_GLOBAL_LIMIT"
	CodeAdmissionProfileLimit = "ADMISSION_PROFILE_LIMIT"
	CodeCursorInvalid         = "CURSOR_INVALID"
)

// The words of Error.Fields.
const (
	wordMissing      = "missing"
	wordInvalid      = "invalid"
	wordRepeated     = "repeated"
	wordEmpty        = "empty"
	wordTooMany      = "too_many"
	wordNotPermitted = "not_permitted"
)

// Service requests reports and retries their PDFs.
type Service struct {
	Pool     *pgxpool.Pool
	Bodies   *pgxpool.Pool // reads the bodies of snapshots and PDFs
	Queue    *river.Client[pgx.Tx]
	Settings settings.Settings
	Now      func() time.Time
	NewID    func() uuid.UUID

	// afterKeyRead, when set, runs in Request between the read of an
	// idempotency row and the read of the report it names.
	afterKeyRead func(context.Context)

	// beforeMark, when set, runs in DeleteMany after every id is judged and
	// before the first is marked.
	beforeMark func(context.Context)
}

// Selection is a request as the operator sends it; a nil field takes the profile's default.
type Selection struct {
	ProfileID     string   `json:"profile_id"`
	Namespaces    []string `json:"namespaces"`
	Period        *string  `json:"period"`
	Timezone      *string  `json:"timezone"`
	ReportProfile *string  `json:"report_profile"`
	MinSeverity   *string  `json:"min_severity"`
}

// Resolved is the selection a report stores and the worker runs.
type Resolved struct {
	Namespaces    []string `json:"namespaces"`
	Period        string   `json:"period"`
	Timezone      string   `json:"timezone"`
	ReportProfile string   `json:"report_profile"`
	MinSeverity   string   `json:"min_severity"`
}

// Accepted is the answer to a request.
type Accepted struct {
	ID        uuid.UUID
	Status    string
	Phase     string
	ExpiresAt time.Time
	Replayed  bool
}

// Error is a refusal; Fields map a field of the selection to one of
// missing, invalid, repeated, empty, not_permitted.
type Error struct {
	Code       string
	Fields     map[string]string
	RetryAfter time.Duration // set for the two admission codes
}

// Error returns the code.
func (e *Error) Error() string { return e.Code }

func (s *Service) now() time.Time {
	if s.Now != nil {
		return s.Now().UTC()
	}
	return time.Now().UTC()
}

func (s *Service) newID() uuid.UUID {
	if s.NewID != nil {
		return s.NewID()
	}
	return uuid.New()
}

// isExpired reports whether a report is past its retention at now, or was
// marked expired by maintenance.
func isExpired(expiresAt time.Time, expiredAt *time.Time, now time.Time) bool {
	return !expiresAt.After(now) || expiredAt != nil
}

func validLanguage(language string) bool { return language == "pt-BR" || language == "en" }

func requireOperator(actor auth.Principal) error {
	if actor.Role != auth.RoleOperator && actor.Role != auth.RoleAdministrator {
		return auth.ErrForbidden
	}
	return nil
}

// RetryPDF queues the render of one language of a collected report whose PDF
// failed. A language that is already pending is accepted and changes nothing.
func (s *Service) RetryPDF(ctx context.Context, actor auth.Principal, reportID uuid.UUID, language string) error {
	if err := requireOperator(actor); err != nil {
		return err
	}
	if !validLanguage(language) {
		return &Error{Code: CodeRequestInvalid, Fields: map[string]string{"language": wordInvalid}}
	}
	now := s.now()
	return store.Transact(ctx, s.Pool, func(tx pgx.Tx) error {
		report, err := store.GetReportForUpdate(ctx, tx, reportID)
		if errors.Is(err, store.ErrNotFound) {
			return &Error{Code: CodeReportNotFound}
		}
		if err != nil {
			return err
		}
		if report.DeletedAt != nil {
			return &Error{Code: CodeReportNotFound}
		}
		if isExpired(report.ExpiresAt, report.ExpiredAt, now) {
			return &Error{Code: CodeReportExpired}
		}
		if report.Status != "collected" {
			return &Error{Code: CodeSnapshotNotReady}
		}
		row, err := store.GetPDF(ctx, tx, reportID, language)
		if errors.Is(err, store.ErrNotFound) {
			return &Error{Code: CodePDFRetryNotAllowed}
		}
		if err != nil {
			return err
		}
		switch row.Status {
		case "pending":
			return nil
		case "failed":
		default:
			return &Error{Code: CodePDFRetryNotAllowed}
		}
		job, err := s.Queue.InsertTx(ctx, tx, RenderArgs{ReportID: reportID, Language: language}, RenderInsertOpts(s.Settings))
		if err != nil {
			return fmt.Errorf("queue render: %w", err)
		}
		if err := store.RetryPDF(ctx, tx, reportID, language, job.Job.ID, now); err != nil {
			return err
		}
		if err := store.SetReportPhase(ctx, tx, reportID, "rendering", nil, now); err != nil {
			return err
		}
		return audit.Append(ctx, tx, now, audit.Event{
			ActorID: &actor.ID, Action: ActionReportPDFRetry, TargetType: targetReport, TargetID: reportID.String(),
			Metadata: audit.Metadata{"language": language},
		})
	})
}
