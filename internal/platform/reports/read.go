package reports

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/marceloalvescf/k8s-capacity-report/internal/platform/auth"
	"github.com/marceloalvescf/k8s-capacity-report/internal/platform/store"
)

const maxPageLimit = 200

// Summary is a report as the list and the detail show it.
type Summary struct {
	ID            uuid.UUID
	OwnerID       uuid.UUID
	OwnerUsername string
	ProfileID     string
	Status, Phase string
	FailureCode   *string
	PDFs          []PDFState
	CreatedAt     time.Time
	CompletedAt   *time.Time
	ExpiresAt     time.Time
	Expired       bool
}

// PDFState is the state of the PDF of one language.
type PDFState struct {
	Language    string
	Status      string
	FailureCode *string
}

// Detail is Summary with the request and the progress.
type Detail struct {
	Summary
	ProfileRevisionID uuid.UUID
	Attempt           int
	StartedAt         *time.Time
	CollectedAt       *time.Time
	Selection         Resolved
}

// Page is one page of the list.
type Page struct {
	Items      []Summary
	NextCursor *string
}

// Download is a body to serve with its strong tag.
type Download struct {
	Body   []byte
	SHA256 string
}

// encodeCursor is the position after a report: its creation instant in UTC
// and its id, as base64url without padding.
func encodeCursor(c store.ReportCursor) string {
	raw := c.CreatedAt.UTC().Format(time.RFC3339Nano) + "\n" + c.ID.String()
	return base64.RawURLEncoding.EncodeToString([]byte(raw))
}

// decodeCursor reads a cursor and accepts only what encodeCursor writes.
func decodeCursor(cursor string) (store.ReportCursor, error) {
	invalid := &Error{Code: CodeCursorInvalid}
	raw, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil {
		return store.ReportCursor{}, invalid
	}
	instant, id, ok := strings.Cut(string(raw), "\n")
	if !ok {
		return store.ReportCursor{}, invalid
	}
	at, err := time.Parse(time.RFC3339Nano, instant)
	if err != nil {
		return store.ReportCursor{}, invalid
	}
	uid, err := uuid.Parse(id)
	if err != nil {
		return store.ReportCursor{}, invalid
	}
	c := store.ReportCursor{CreatedAt: at.UTC(), ID: uid}
	if encodeCursor(c) != cursor {
		return store.ReportCursor{}, invalid
	}
	return c, nil
}

func summaryOf(it store.ReportListItem, now time.Time) Summary {
	pdfs := make([]PDFState, len(it.PDFs))
	for i, p := range it.PDFs {
		pdfs[i] = PDFState{Language: p.Language, Status: p.Status, FailureCode: p.FailureCode}
	}
	return Summary{
		ID: it.ID, OwnerID: it.OwnerID, OwnerUsername: it.OwnerUsername, ProfileID: it.ProfileID,
		Status: it.Status, Phase: it.Phase, FailureCode: it.FailureCode, PDFs: pdfs,
		CreatedAt: it.CreatedAt, CompletedAt: it.CompletedAt, ExpiresAt: it.ExpiresAt,
		Expired: isExpired(it.ExpiresAt, it.ExpiredAt, now),
	}
}

// List returns one page of the reports not deleted, newest first, after the
// position the cursor names; an empty cursor starts at the newest.
func (s *Service) List(ctx context.Context, _ auth.Principal, cursor string, limit int) (Page, error) {
	if limit < 1 || limit > maxPageLimit {
		return Page{}, &Error{Code: CodeRequestInvalid, Fields: map[string]string{"limit": wordInvalid}}
	}
	var after *store.ReportCursor
	if cursor != "" {
		c, err := decodeCursor(cursor)
		if err != nil {
			return Page{}, err
		}
		after = &c
	}
	now := s.now()
	items, err := store.ListReports(ctx, s.Pool, after, limit+1)
	if err != nil {
		return Page{}, err
	}
	page := Page{Items: make([]Summary, 0, min(len(items), limit))}
	if len(items) > limit {
		items = items[:limit]
		last := items[limit-1]
		next := encodeCursor(store.ReportCursor{CreatedAt: last.CreatedAt, ID: last.ID})
		page.NextCursor = &next
	}
	for _, it := range items {
		page.Items = append(page.Items, summaryOf(it, now))
	}
	return page, nil
}

// Get returns one report with its request and progress.
func (s *Service) Get(ctx context.Context, _ auth.Principal, id uuid.UUID) (Detail, error) {
	now := s.now()
	it, err := store.GetReportItem(ctx, s.Pool, id)
	if errors.Is(err, store.ErrNotFound) {
		return Detail{}, &Error{Code: CodeReportNotFound}
	}
	if err != nil {
		return Detail{}, err
	}
	if isExpired(it.ExpiresAt, it.ExpiredAt, now) {
		return Detail{}, &Error{Code: CodeReportExpired}
	}
	selection := Resolved{Namespaces: []string{}}
	if err := json.Unmarshal(it.Request, &selection); err != nil {
		return Detail{}, fmt.Errorf("decode stored selection: %w", err)
	}
	if selection.Namespaces == nil {
		selection.Namespaces = []string{}
	}
	return Detail{
		Summary: summaryOf(it, now), ProfileRevisionID: it.ProfileRevisionID, Attempt: it.Attempt,
		StartedAt: it.StartedAt, CollectedAt: it.CollectedAt, Selection: selection,
	}, nil
}

// Snapshot returns the stored snapshot of a collected report.
func (s *Service) Snapshot(ctx context.Context, _ auth.Principal, id uuid.UUID) (Download, error) {
	now := s.now()
	read, err := store.ReadSnapshot(ctx, s.Pool, id, now)
	if errors.Is(err, store.ErrNotFound) {
		return Download{}, &Error{Code: CodeReportNotFound}
	}
	if err != nil {
		return Download{}, err
	}
	if isExpired(read.ExpiresAt, read.ExpiredAt, now) {
		return Download{}, &Error{Code: CodeReportExpired}
	}
	if read.Status != "collected" {
		return Download{}, &Error{Code: CodeSnapshotNotReady}
	}
	if read.Body == nil {
		return Download{}, errors.New("collected report has no snapshot")
	}
	sum := sha256.Sum256(read.Body)
	if hex.EncodeToString(sum[:]) != read.SHA256 {
		return Download{}, errors.New("stored snapshot does not match its SHA-256")
	}
	return Download{Body: read.Body, SHA256: read.SHA256}, nil
}

// PDF returns the stored PDF of one language of a collected report.
func (s *Service) PDF(ctx context.Context, _ auth.Principal, id uuid.UUID, language string) (Download, error) {
	if !validLanguage(language) {
		return Download{}, &Error{Code: CodeRequestInvalid, Fields: map[string]string{"language": wordInvalid}}
	}
	now := s.now()
	read, err := store.ReadPDF(ctx, s.Pool, id, language, now)
	if errors.Is(err, store.ErrNotFound) {
		return Download{}, &Error{Code: CodeReportNotFound}
	}
	if err != nil {
		return Download{}, err
	}
	if isExpired(read.ExpiresAt, read.ExpiredAt, now) {
		return Download{}, &Error{Code: CodeReportExpired}
	}
	if read.Status != "collected" {
		return Download{}, &Error{Code: CodeSnapshotNotReady}
	}
	if read.PDF == nil {
		return Download{}, &Error{Code: CodePDFNotReady, Fields: map[string]string{"status": "pending"}}
	}
	if read.PDF.Status != "ready" {
		fields := map[string]string{"status": read.PDF.Status}
		if read.PDF.FailureCode != nil {
			fields["failure_code"] = *read.PDF.FailureCode
		}
		return Download{}, &Error{Code: CodePDFNotReady, Fields: fields}
	}
	if read.Body == nil || read.PDF.SHA256 == nil {
		return Download{}, errors.New("ready pdf has no body")
	}
	return Download{Body: read.Body, SHA256: *read.PDF.SHA256}, nil
}
