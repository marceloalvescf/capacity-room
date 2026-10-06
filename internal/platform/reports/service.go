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
	"github.com/marceloalvescf/k8s-capacity-report/internal/platform/profiles"
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

// The words of Error.Fields.
const (
	wordMissing      = "missing"
	wordInvalid      = "invalid"
	wordRepeated     = "repeated"
	wordEmpty        = "empty"
	wordNotPermitted = "not_permitted"
)

// Service requests reports and retries their PDFs.
type Service struct {
	Pool     *pgxpool.Pool
	Queue    *river.Client[pgx.Tx]
	Profiles *profiles.Service
	Settings settings.Settings
	Now      func() time.Time
	NewID    func() uuid.UUID
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
	if language != "pt-BR" && language != "en" {
		return &Error{Code: "REQUEST_INVALID", Fields: map[string]string{"language": wordInvalid}}
	}
	now := s.now()
	return store.Transact(ctx, s.Pool, func(tx pgx.Tx) error {
		report, err := store.GetReportForUpdate(ctx, tx, reportID)
		if errors.Is(err, store.ErrNotFound) {
			return &Error{Code: "REPORT_NOT_FOUND"}
		}
		if err != nil {
			return err
		}
		if !report.ExpiresAt.After(now) {
			return &Error{Code: "REPORT_EXPIRED"}
		}
		if report.Status != "collected" {
			return &Error{Code: "SNAPSHOT_NOT_READY"}
		}
		row, err := store.GetPDF(ctx, tx, reportID, language)
		if errors.Is(err, store.ErrNotFound) {
			return &Error{Code: "PDF_RETRY_NOT_ALLOWED"}
		}
		if err != nil {
			return err
		}
		switch row.Status {
		case "pending":
			return nil
		case "failed":
		default:
			return &Error{Code: "PDF_RETRY_NOT_ALLOWED"}
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
