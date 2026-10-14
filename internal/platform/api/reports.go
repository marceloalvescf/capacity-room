package api

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"

	"github.com/marceloalvescf/k8s-capacity-report/internal/catalog"
	"github.com/marceloalvescf/k8s-capacity-report/internal/document"
	"github.com/marceloalvescf/k8s-capacity-report/internal/platform/auth"
	"github.com/marceloalvescf/k8s-capacity-report/internal/platform/reports"
	"github.com/marceloalvescf/k8s-capacity-report/internal/snapshot"
)

const (
	defaultPageLimit = 50
	maxLanguage      = 16
)

var errNoReports = errors.New("the reports service is not configured")

// readerRoute wraps a handler of a route any signed-in user may call.
func (s *Server) readerRoute(next SessionHandler) http.HandlerFunc {
	return s.requireSession(s.withReports(next))
}

// writerRoute wraps a handler of a route that changes state: an operator or
// an administrator, with a valid CSRF token.
func (s *Server) writerRoute(next SessionHandler) http.HandlerFunc {
	return s.requireRole([]auth.Role{auth.RoleOperator, auth.RoleAdministrator}, s.requireCSRF(s.withReports(next)))
}

// withReports answers 500 when the server has no reports service.
func (s *Server) withReports(next SessionHandler) SessionHandler {
	return func(w http.ResponseWriter, r *http.Request, a auth.Authenticated) {
		if s.Reports == nil {
			s.fail(w, r, errNoReports)
			return
		}
		next(w, r, a)
	}
}

// reportID reads the id of the path and answers 404 REPORT_NOT_FOUND when it
// cannot name a report.
func reportID(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		WriteProblem(w, &Problem{Code: CodeReportNotFound})
		return uuid.UUID{}, false
	}
	return id, true
}

// langParam reads the query parameter lang and answers 400 when it is
// missing or not one of the two languages.
func langParam(w http.ResponseWriter, r *http.Request) (catalog.Language, bool) {
	query := r.URL.Query()
	if !query.Has("lang") {
		WriteProblem(w, &Problem{Code: CodeRequestInvalid, Fields: map[string]string{"lang": "missing"}})
		return "", false
	}
	lang, err := catalog.ParseLanguage(query.Get("lang"))
	if err != nil {
		WriteProblem(w, &Problem{Code: CodeRequestInvalid, Fields: map[string]string{"lang": "invalid"}})
		return "", false
	}
	return lang, true
}

func utc(t *time.Time) *time.Time {
	if t == nil {
		return nil
	}
	u := t.UTC()
	return &u
}

func pdfStatesOf(in []reports.PDFState) []ReportPDFState {
	out := make([]ReportPDFState, len(in))
	for i, p := range in {
		out[i] = ReportPDFState{Language: Language(p.Language), Status: PDFStatus(p.Status)}
		if p.FailureCode != nil {
			code := PDFFailureCode(*p.FailureCode)
			out[i].FailureCode = &code
		}
	}
	return out
}

func failureCodeOf(code *string) *ReportFailureCode {
	if code == nil {
		return nil
	}
	c := ReportFailureCode(*code)
	return &c
}

func reportSelectionOf(r reports.Resolved) ReportSelection {
	return ReportSelection{
		Namespaces: r.Namespaces, Period: nullable(r.Period), Start: nullable(r.Start), End: nullable(r.End), Timezone: r.Timezone,
		ReportProfile: ReportProfile(r.ReportProfile), MinSeverity: MinSeverity(r.MinSeverity),
		Language: Language(r.Language),
	}
}

func reportSummaryOf(s reports.Summary) ReportSummary {
	var selection *ReportSelection
	if s.Selection != nil {
		sel := reportSelectionOf(*s.Selection)
		selection = &sel
	}
	return ReportSummary{
		Selection: selection,
		Id:        s.ID, OwnerId: s.OwnerID, OwnerUsername: s.OwnerUsername, ProfileId: s.ProfileID,
		Status: ReportStatus(s.Status), Phase: ReportPhase(s.Phase), FailureCode: failureCodeOf(s.FailureCode),
		Pdfs: pdfStatesOf(s.PDFs), CreatedAt: s.CreatedAt.UTC(), CompletedAt: utc(s.CompletedAt),
		ExpiresAt: s.ExpiresAt.UTC(), Expired: s.Expired,
		WorstSeverity: severityOf(s.WorstSeverity), ClusterName: s.ClusterName,
	}
}

// severityOf maps a stored severity to the contract type; nil stays null.
func severityOf(s *string) *Severity {
	if s == nil {
		return nil
	}
	v := Severity(*s)
	return &v
}

// nullable maps an empty string to null.
func nullable(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func reportOf(d reports.Detail) Report {
	return Report{
		Id: d.ID, OwnerId: d.OwnerID, OwnerUsername: d.OwnerUsername, ProfileId: d.ProfileID,
		ProfileRevisionId: d.ProfileRevisionID, Attempt: d.Attempt,
		Status: ReportStatus(d.Status), Phase: ReportPhase(d.Phase), FailureCode: failureCodeOf(d.FailureCode),
		Pdfs: pdfStatesOf(d.PDFs), CreatedAt: d.CreatedAt.UTC(), StartedAt: utc(d.StartedAt),
		CollectedAt: utc(d.CollectedAt), CompletedAt: utc(d.CompletedAt),
		ExpiresAt: d.ExpiresAt.UTC(), Expired: d.Expired,
		WorstSeverity: severityOf(d.WorstSeverity), ClusterName: d.ClusterName,
		Selection: reportSelectionOf(*d.Selection),
	}
}

// requestReport takes a report request under the Idempotency-Key header.
func (s *Server) requestReport(w http.ResponseWriter, r *http.Request, a auth.Authenticated) {
	var sel reports.Selection
	if p := DecodeBody(r, &sel, []string{"language"}, nil); p != nil {
		WriteProblem(w, p)
		return
	}
	accepted, err := s.Reports.Request(r.Context(), a.Principal, r.Header.Get("Idempotency-Key"), sel)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusAccepted, ReportAccepted{
		Id: accepted.ID, Status: ReportStatus(accepted.Status), Phase: ReportPhase(accepted.Phase),
		ExpiresAt: accepted.ExpiresAt.UTC(), Replayed: accepted.Replayed,
	})
}

func (s *Server) listReports(w http.ResponseWriter, r *http.Request, a auth.Authenticated) {
	query := r.URL.Query()
	limit := defaultPageLimit
	if query.Has("limit") {
		n, err := strconv.Atoi(query.Get("limit"))
		if err != nil {
			WriteProblem(w, &Problem{Code: CodeRequestInvalid, Fields: map[string]string{"limit": "invalid"}})
			return
		}
		limit = n
	}
	page, err := s.Reports.List(r.Context(), a.Principal, query.Get("cursor"), limit)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	out := ReportPage{Items: make([]ReportSummary, len(page.Items)), NextCursor: page.NextCursor}
	for i, it := range page.Items {
		out.Items[i] = reportSummaryOf(it)
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) getReport(w http.ResponseWriter, r *http.Request, a auth.Authenticated) {
	id, ok := reportID(w, r)
	if !ok {
		return
	}
	detail, err := s.Reports.Get(r.Context(), a.Principal, id)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, reportOf(detail))
}

// serveBody answers a download: the tag, the cache rule, and the type are
// set, and ServeContent does the conditional answer, Range, and HEAD. The
// write must end within the write deadline of the server.
func (s *Server) serveBody(w http.ResponseWriter, r *http.Request, body []byte, tag, contentType, disposition string) {
	// A writer that cannot take a deadline is a test recorder.
	_ = http.NewResponseController(w).SetWriteDeadline(time.Now().Add(s.writeDeadline()))
	h := w.Header()
	h.Set("ETag", `"`+tag+`"`)
	h.Set("Cache-Control", "private, no-cache")
	h.Set("Content-Type", contentType)
	if disposition != "" {
		h.Set("Content-Disposition", disposition)
	}
	http.ServeContent(w, r, "", time.Time{}, bytes.NewReader(body))
}

// viewReport words the stored snapshot in one language for the web.
func (s *Server) viewReport(w http.ResponseWriter, r *http.Request, a auth.Authenticated) {
	lang, ok := langParam(w, r)
	if !ok {
		return
	}
	id, ok := reportID(w, r)
	if !ok {
		return
	}
	release, ok := s.acquireDownload(r)
	if !ok {
		return
	}
	defer release()
	stored, err := s.Reports.Snapshot(r.Context(), a.Principal, id)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	body, err := encodeView(stored.Body, lang)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	s.serveBody(w, r, body, sha256Hex(body), "application/json", "")
}

func encodeView(raw []byte, lang catalog.Language) ([]byte, error) {
	snap, err := snapshot.Decode(raw)
	if err != nil {
		return nil, err
	}
	cat, err := catalog.Load(lang)
	if err != nil {
		return nil, err
	}
	view, err := document.BuildView(snap, cat)
	if err != nil {
		return nil, err
	}
	return document.EncodeView(view)
}

func (s *Server) getReportSnapshot(w http.ResponseWriter, r *http.Request, a auth.Authenticated) {
	id, ok := reportID(w, r)
	if !ok {
		return
	}
	release, ok := s.acquireDownload(r)
	if !ok {
		return
	}
	defer release()
	stored, err := s.Reports.Snapshot(r.Context(), a.Principal, id)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	s.serveBody(w, r, stored.Body, stored.SHA256, "application/json", `attachment; filename="report-`+id.String()+`.json"`)
}

func (s *Server) getReportPDF(w http.ResponseWriter, r *http.Request, a auth.Authenticated) {
	lang, ok := langParam(w, r)
	if !ok {
		return
	}
	id, ok := reportID(w, r)
	if !ok {
		return
	}
	release, ok := s.acquireDownload(r)
	if !ok {
		return
	}
	defer release()
	stored, err := s.Reports.PDF(r.Context(), a.Principal, id, string(lang))
	if err != nil {
		s.fail(w, r, err)
		return
	}
	s.serveBody(w, r, stored.Body, stored.SHA256, "application/pdf",
		`attachment; filename="report-`+id.String()+`-`+string(lang)+`.pdf"`)
}

func (s *Server) requestReportPDF(w http.ResponseWriter, r *http.Request, a auth.Authenticated) {
	var body PDFRequest
	if p := DecodeBody(r, &body, []string{"language"}, map[string]int{"language": maxLanguage}); p != nil {
		WriteProblem(w, p)
		return
	}
	if !body.Language.Valid() {
		WriteProblem(w, &Problem{Code: CodeRequestInvalid, Fields: map[string]string{"language": "invalid"}})
		return
	}
	id, ok := reportID(w, r)
	if !ok {
		return
	}
	if err := s.Reports.RequestPDF(r.Context(), a.Principal, id, string(body.Language)); err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusAccepted, PDFRequestAccepted{Language: body.Language, Status: PDFStatusPending})
}

func (s *Server) deleteReport(w http.ResponseWriter, r *http.Request, a auth.Authenticated) {
	id, ok := reportID(w, r)
	if !ok {
		return
	}
	if err := s.Reports.Delete(r.Context(), a.Principal, id); err != nil {
		s.fail(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// deleteReports reads each id as raw JSON, so that an element that is not a
// UUID string, a number included, is a problem of the field ids and not of
// the element path.
func (s *Server) deleteReports(w http.ResponseWriter, r *http.Request, a auth.Authenticated) {
	var body struct {
		IDs []json.RawMessage `json:"ids"`
	}
	if p := DecodeBody(r, &body, []string{"ids"}, nil); p != nil {
		WriteProblem(w, p)
		return
	}
	ids := make([]uuid.UUID, len(body.IDs))
	for i, raw := range body.IDs {
		var text string
		id, err := uuid.Nil, json.Unmarshal(raw, &text)
		if err == nil {
			id, err = uuid.Parse(text)
		}
		if err != nil {
			WriteProblem(w, &Problem{Code: CodeRequestInvalid, Fields: map[string]string{"ids": "invalid"}})
			return
		}
		ids[i] = id
	}
	results, err := s.Reports.DeleteMany(r.Context(), a.Principal, ids)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	out := ReportDeleteResults{Results: make([]ReportDeleteResult, len(results))}
	for i, res := range results {
		out.Results[i] = ReportDeleteResult{Id: res.ID, Outcome: ReportDeleteOutcome(res.Outcome)}
	}
	writeJSON(w, http.StatusOK, out)
}

func sha256Hex(body []byte) string {
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}
