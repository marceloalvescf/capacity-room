//go:build postgres

package api

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/marceloalvescf/k8s-capacity-report/internal/platform/auth"
	"github.com/marceloalvescf/k8s-capacity-report/internal/platform/profiles"
	"github.com/marceloalvescf/k8s-capacity-report/internal/platform/reports"
	"github.com/marceloalvescf/k8s-capacity-report/internal/platform/store"
)

const (
	opsPassword   = "ops-password-123"
	otherPassword = "other-password-12"
)

// actor is a signed-in caller of the handler.
type actor struct {
	cookie *http.Cookie
	csrf   string
	user   store.User
}

// cast is the four callers the report routes tell apart.
type cast struct{ ops, other, root, eve actor }

func (ts *testServer) signInAs(t *testing.T, user store.User, password string) actor {
	t.Helper()
	w := ts.login(t, user.Username, password)
	if w.Code != 200 {
		t.Fatalf("sign in %s: %d %s", user.Username, w.Code, w.Body.String())
	}
	var s SignedInSession
	if err := json.Unmarshal(w.Body.Bytes(), &s); err != nil {
		t.Fatal(err)
	}
	return actor{cookie: cookieNamed(w, SessionCookie), csrf: s.CsrfToken, user: user}
}

func (ts *testServer) cast(t *testing.T) cast {
	t.Helper()
	ops := seed(t, ts.pool, ts.now, "ops", opsPassword, auth.RoleOperator)
	other := seed(t, ts.pool, ts.now, "other", otherPassword, auth.RoleOperator)
	root := store.User{}
	if err := ts.pool.QueryRow(context.Background(), `SELECT id, username FROM users WHERE username = 'root'`).Scan(&root.ID, &root.Username); err != nil {
		t.Fatal(err)
	}
	return cast{
		ops:   ts.signInAs(t, ops, opsPassword),
		other: ts.signInAs(t, other, otherPassword),
		root:  ts.signInAs(t, root, rootPassword),
		eve:   ts.signInAs(t, ts.eve, evePassword),
	}
}

// as sends a request as a, with its CSRF token on every method that
// changes state; a header of the call overrides it.
func (ts *testServer) as(t *testing.T, a *actor, method, path, body string, header map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	h := map[string]string{}
	if a != nil && method != "GET" && method != "HEAD" {
		h[CSRFHeaderName] = a.csrf
	}
	for k, v := range header {
		h[k] = v
	}
	if a == nil {
		return ts.call(t, method, path, body, h)
	}
	return ts.call(t, method, path, body, h, a.cookie)
}

func decodeStrict[T any](t *testing.T, w *httptest.ResponseRecorder) T {
	t.Helper()
	var v T
	dec := json.NewDecoder(bytes.NewReader(w.Body.Bytes()))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&v); err != nil {
		t.Fatalf("body %q: %v", w.Body.String(), err)
	}
	return v
}

func sha(body []byte) string {
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}

// profileRevision seeds the profile "prod" once.
func (ts *testServer) profileRevision(t *testing.T) uuid.UUID {
	t.Helper()
	if ts.rev != uuid.Nil {
		return ts.rev
	}
	ctx := context.Background()
	d := profiles.Defaults()
	d.DisplayName = "Production"
	d.Active = true
	d.EndpointURL = "https://metrics.test/api/v1"
	d.EndpointType = "prometheus"
	d.ClusterName = "prod"
	d.AllowedNamespaces = []string{"payments", "billing"}
	raw, err := json.Marshal(d)
	if err != nil {
		t.Fatal(err)
	}
	err = store.Transact(ctx, ts.pool, func(tx pgx.Tx) error {
		if _, err := store.GetOrCreateProfile(ctx, tx, "prod", ts.now); err != nil {
			return err
		}
		rev, err := store.AppendProfileRevision(ctx, tx, store.ProfileRevision{
			ID: uuid.New(), ProfileID: "prod", Fingerprint: uuid.NewString(), DisplayName: d.DisplayName,
			Active: d.Active, Document: raw, CreatedAt: ts.now,
		})
		if err != nil {
			return err
		}
		ts.rev = rev.ID
		return store.SetCurrentRevision(ctx, tx, "prod", &rev.ID, ts.now)
	})
	if err != nil {
		t.Fatal(err)
	}
	return ts.rev
}

// stored describes a report to seed; the zero value is a collected report of
// the operator "ops" with two ready PDFs and no snapshot.
type stored struct {
	owner    uuid.UUID
	status   string
	phase    string
	pdfs     string // "", ready, failed, or pending; ready when the status is collected and this is empty
	expires  time.Time
	created  time.Time
	snapshot []byte
	pdfBody  []byte
}

var pdfBody = []byte("%PDF-1.7 stored body")

func (ts *testServer) seedReport(t *testing.T, o stored) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	if o.status == "" {
		o.status, o.phase, o.pdfs = "collected", "complete", "ready"
	}
	if o.expires.IsZero() {
		o.expires = ts.now.Add(24 * time.Hour)
	}
	if o.created.IsZero() {
		o.created = ts.now
	}
	if o.pdfBody == nil {
		o.pdfBody = pdfBody
	}
	r := store.Report{
		ID: uuid.New(), OwnerID: o.owner, ProfileID: "prod", ProfileRevisionID: ts.profileRevision(t),
		Request: []byte(`{"namespaces":["payments"],"period":"24h","timezone":"UTC","report_profile":"detailed","min_severity":"all"}`),
		Status:  o.status, Phase: o.phase, CollectJobID: 1,
		CreatedAt: o.created, ExpiresAt: o.expires, UpdatedAt: o.created,
	}
	if o.status == "collected" && o.phase == "complete" {
		done := o.created
		r.CompletedAt = &done
	}
	if o.status == "failed" {
		code := "INTERNAL_FAILURE"
		r.FailureCode = &code
	}
	if err := store.InsertReport(ctx, ts.pool, r); err != nil {
		t.Fatal(err)
	}
	if o.snapshot != nil {
		if err := store.InsertSnapshot(ctx, ts.pool, r.ID, "1.0", o.snapshot, ts.now); err != nil {
			t.Fatal(err)
		}
	}
	for _, lang := range []string{"pt-BR", "en"} {
		if o.pdfs == "" {
			break
		}
		if err := store.InsertPendingPDF(ctx, ts.pool, r.ID, lang, 1, ts.now); err != nil {
			t.Fatal(err)
		}
		var err error
		switch o.pdfs {
		case "ready":
			err = store.FinishPDF(ctx, ts.pool, r.ID, lang, o.pdfBody, ts.now)
		case "failed":
			err = store.FailPDF(ctx, ts.pool, r.ID, lang, "PDF_COMPILE_FAILED", ts.now)
		}
		if err != nil {
			t.Fatal(err)
		}
	}
	return r.ID
}

func (ts *testServer) markDeleted(t *testing.T, id uuid.UUID) {
	t.Helper()
	err := store.Transact(context.Background(), ts.pool, func(tx pgx.Tx) error {
		_, err := store.MarkReportDeleted(context.Background(), tx, id, ts.now)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
}

func repoFile(t *testing.T, elem ...string) []byte {
	t.Helper()
	root := filepath.Dir(filepath.Dir(contractPath(t)))
	raw, err := os.ReadFile(filepath.Join(append([]string{root}, elem...)...))
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func TestReportRoutesCheckInOrder(t *testing.T) {
	ts := newTestServer(t)
	c := ts.cast(t)
	id := uuid.NewString()
	base := "/api/v1/reports/" + id
	routes := []struct {
		name, method, path, body string
		header                   map[string]string
		write                    bool
		badPath, badBody         string
	}{
		{name: "list", method: "GET", path: "/api/v1/reports", badPath: "/api/v1/reports?limit=x"},
		{name: "request", method: "POST", path: "/api/v1/reports", body: `{"profile_id":"prod","namespaces":["payments"]}`,
			header: map[string]string{"Idempotency-Key": "k1"}, write: true, badBody: `[]`},
		{name: "detail", method: "GET", path: base},
		{name: "view", method: "GET", path: base + "/view?lang=en", badPath: base + "/view"},
		{name: "snapshot", method: "GET", path: base + "/snapshot"},
		{name: "pdf", method: "GET", path: base + "/pdf?lang=en", badPath: base + "/pdf"},
		{name: "retry", method: "POST", path: base + "/pdf-retries", body: `{"language":"en"}`, write: true, badBody: `{"language":1}`},
		{name: "delete", method: "DELETE", path: base, write: true},
		{name: "bulk delete", method: "POST", path: "/api/v1/reports/delete", body: `{"ids":["` + id + `"]}`, write: true, badBody: `{"ids":"x"}`},
	}
	for _, r := range routes {
		t.Run(r.name, func(t *testing.T) {
			wantProblem(t, ts.as(t, nil, r.method, r.path, r.body, r.header), 401, CodeUnauthenticated, nil)
			badPath, badBody := r.path, r.body
			if r.badPath != "" {
				badPath = r.badPath
			}
			if r.badBody != "" {
				badBody = r.badBody
			}
			wantProblem(t, ts.as(t, nil, r.method, badPath, badBody, r.header), 401, CodeUnauthenticated, nil)
			if r.write {
				wantProblem(t, ts.as(t, &c.eve, r.method, r.path, r.body, r.header), 403, CodeForbidden, nil)
				wantProblem(t, ts.as(t, &c.eve, r.method, badPath, badBody, r.header), 403, CodeForbidden, nil)
				noCSRF := map[string]string{CSRFHeaderName: ""}
				for k, v := range r.header {
					noCSRF[k] = v
				}
				wantProblem(t, ts.as(t, &c.ops, r.method, r.path, r.body, noCSRF), 403, CodeCSRFRejected, nil)
				wantProblem(t, ts.as(t, &c.ops, r.method, badPath, badBody, noCSRF), 403, CodeCSRFRejected, nil)
			}
			if r.badPath != "" || r.badBody != "" {
				wantProblem(t, ts.as(t, &c.ops, r.method, badPath, badBody, r.header), 400, CodeRequestInvalid, nil)
			}
		})
	}
}

func TestRequestReportRoute(t *testing.T) {
	ts := newTestServer(t)
	c := ts.cast(t)
	ts.profileRevision(t)
	body := `{"profile_id":"prod","namespaces":["payments"]}`
	key := map[string]string{"Idempotency-Key": "key-1"}

	w := ts.as(t, &c.ops, "POST", "/api/v1/reports", body, key)
	if w.Code != 202 {
		t.Fatalf("first: %d %s", w.Code, w.Body.String())
	}
	first := decodeStrict[ReportAccepted](t, w)
	if first.Replayed || first.Status != ReportStatusQueued || first.Phase != Waiting || first.Id == uuid.Nil {
		t.Fatalf("first answer %+v", first)
	}
	if want := ts.now.Add(7 * 24 * time.Hour); !first.ExpiresAt.Equal(want) {
		t.Fatalf("expires_at %v, want %v", first.ExpiresAt, want)
	}

	w = ts.as(t, &c.ops, "POST", "/api/v1/reports", body, key)
	if w.Code != 202 {
		t.Fatalf("replay: %d %s", w.Code, w.Body.String())
	}
	again := decodeStrict[ReportAccepted](t, w)
	if !again.Replayed || again.Id != first.Id {
		t.Fatalf("replay answer %+v, first %+v", again, first)
	}
	if n := ts.count(t, `SELECT count(*) FROM river_job WHERE kind = 'collect'`); n != 1 {
		t.Fatalf("%d collect jobs, want 1", n)
	}

	other := `{"profile_id":"prod","namespaces":["billing"]}`
	wantProblem(t, ts.as(t, &c.ops, "POST", "/api/v1/reports", other, key), 409, "IDEMPOTENCY_KEY_REUSED", nil)
	wantProblem(t, ts.as(t, &c.ops, "POST", "/api/v1/reports", body, nil), 400, "IDEMPOTENCY_KEY_INVALID", nil)
	wantProblem(t, ts.as(t, &c.ops, "POST", "/api/v1/reports", body, map[string]string{"Idempotency-Key": "bad key!"}), 400, "IDEMPOTENCY_KEY_INVALID", nil)
	wantProblem(t, ts.as(t, &c.ops, "POST", "/api/v1/reports", `{"profile_id":"prod","namespaces":["payments"],"extra":1}`, key), 400, CodeRequestInvalid, map[string]string{"extra": "unknown"})
	wantProblem(t, ts.as(t, &c.ops, "POST", "/api/v1/reports", `{"profile_id":"nope","namespaces":["payments"]}`, map[string]string{"Idempotency-Key": "key-2"}), 404, CodeProfileNotFound, nil)
}

func TestAdmissionAnswers429(t *testing.T) {
	ts := newTestServer(t)
	c := ts.cast(t)
	ts.profileRevision(t)
	ts.Reports.Settings.AdmissionLimit = 1
	ts.Reports.Settings.AdmissionRetry = 5 * time.Second
	body := `{"profile_id":"prod","namespaces":["payments"]}`

	if w := ts.as(t, &c.ops, "POST", "/api/v1/reports", body, map[string]string{"Idempotency-Key": "a"}); w.Code != 202 {
		t.Fatalf("first: %d %s", w.Code, w.Body.String())
	}
	w := ts.as(t, &c.ops, "POST", "/api/v1/reports", body, map[string]string{"Idempotency-Key": "b"})
	wantProblem(t, w, 429, "ADMISSION_GLOBAL_LIMIT", nil)
	if got := w.Header().Get("Retry-After"); got != "5" {
		t.Fatalf("Retry-After %q, want 5", got)
	}
}

func TestSelectionRefused(t *testing.T) {
	ts := newTestServer(t)
	c := ts.cast(t)
	ts.profileRevision(t)
	_, err := ts.Reports.Request(context.Background(),
		auth.Principal{ID: c.ops.user.ID, Username: "ops", Role: auth.RoleOperator},
		"direct", reports.Selection{ProfileID: "prod", Namespaces: []string{"secret"}})
	refused, ok := err.(*reports.Error)
	if !ok || len(refused.Fields) == 0 {
		t.Fatalf("service error %v", err)
	}
	w := ts.as(t, &c.ops, "POST", "/api/v1/reports", `{"profile_id":"prod","namespaces":["secret"]}`, map[string]string{"Idempotency-Key": "k"})
	wantProblem(t, w, 422, "REPORT_SELECTION_NOT_PERMITTED", refused.Fields)
}

func TestListRoutePages(t *testing.T) {
	ts := newTestServer(t)
	c := ts.cast(t)

	w := ts.as(t, &c.eve, "GET", "/api/v1/reports", "", nil)
	if w.Code != 200 || strings.TrimSpace(w.Body.String()) != `{"items":[],"next_cursor":null}` {
		t.Fatalf("empty list: %d %s", w.Code, w.Body.String())
	}

	var ids []uuid.UUID
	for i := 0; i < 5; i++ {
		ids = append(ids, ts.seedReport(t, stored{owner: c.ops.user.ID, created: ts.now.Add(time.Duration(i) * time.Minute)}))
	}
	var seen []uuid.UUID
	var sizes []int
	path := "/api/v1/reports?limit=2"
	for pages := 0; pages < 5; pages++ {
		w := ts.as(t, &c.eve, "GET", path, "", nil)
		if w.Code != 200 {
			t.Fatalf("page: %d %s", w.Code, w.Body.String())
		}
		page := decodeStrict[ReportPage](t, w)
		sizes = append(sizes, len(page.Items))
		for _, it := range page.Items {
			seen = append(seen, it.Id)
			if len(it.Pdfs) != 2 || it.OwnerUsername != "ops" {
				t.Fatalf("item %+v", it)
			}
		}
		if page.NextCursor == nil {
			if !strings.Contains(w.Body.String(), `"next_cursor":null`) {
				t.Fatalf("last page body %s", w.Body.String())
			}
			break
		}
		path = "/api/v1/reports?limit=2&cursor=" + *page.NextCursor
	}
	if fmt.Sprint(sizes) != "[2 2 1]" {
		t.Fatalf("page sizes %v", sizes)
	}
	for i, id := range seen {
		if want := ids[len(ids)-1-i]; id != want {
			t.Fatalf("position %d is %s, want %s", i, id, want)
		}
	}

	wantProblem(t, ts.as(t, &c.eve, "GET", "/api/v1/reports?cursor=!!", "", nil), 400, "CURSOR_INVALID", nil)
	for _, limit := range []string{"0", "201", "abc", ""} {
		wantProblem(t, ts.as(t, &c.eve, "GET", "/api/v1/reports?limit="+limit, "", nil), 400, CodeRequestInvalid, map[string]string{"limit": "invalid"})
	}
	if w := ts.as(t, &c.eve, "GET", "/api/v1/reports?unknown=1", "", nil); w.Code != 200 {
		t.Fatalf("unknown parameter: %d", w.Code)
	}
}

func TestReportRouteStatuses(t *testing.T) {
	ts := newTestServer(t)
	c := ts.cast(t)
	get := func(id string) *httptest.ResponseRecorder {
		return ts.as(t, &c.eve, "GET", "/api/v1/reports/"+id, "", nil)
	}
	wantProblem(t, get(uuid.NewString()), 404, "REPORT_NOT_FOUND", nil)
	wantProblem(t, get("not-a-uuid"), 404, "REPORT_NOT_FOUND", nil)

	deleted := ts.seedReport(t, stored{owner: c.ops.user.ID})
	ts.markDeleted(t, deleted)
	wantProblem(t, get(deleted.String()), 404, "REPORT_NOT_FOUND", nil)

	expired := ts.seedReport(t, stored{owner: c.ops.user.ID, expires: ts.now.Add(-time.Hour)})
	wantProblem(t, get(expired.String()), 410, "REPORT_EXPIRED", nil)

	queued := ts.seedReport(t, stored{owner: c.ops.user.ID, status: "queued", phase: "waiting"})
	w := get(queued.String())
	if w.Code != 200 {
		t.Fatalf("queued: %d %s", w.Code, w.Body.String())
	}
	r := decodeStrict[Report](t, w)
	if r.Id != queued || r.Status != ReportStatusQueued || r.Phase != Waiting || r.OwnerUsername != "ops" ||
		r.ProfileId != "prod" || r.Selection.Period != "24h" || len(r.Selection.Namespaces) != 1 || r.Expired {
		t.Fatalf("report %+v", r)
	}
	if !strings.Contains(w.Body.String(), `"pdfs":[]`) {
		t.Fatalf("pdfs is not an empty list: %s", w.Body.String())
	}
}

func TestViewRoute(t *testing.T) {
	ts := newTestServer(t)
	c := ts.cast(t)
	golden := repoFile(t, "testdata", "snapshot", "direct-complete.json")
	id := ts.seedReport(t, stored{owner: c.ops.user.ID, snapshot: golden})
	base := "/api/v1/reports/" + id.String() + "/view"

	for _, lang := range []string{"en", "pt-BR"} {
		w := ts.as(t, &c.eve, "GET", base+"?lang="+lang, "", nil)
		if w.Code != 200 {
			t.Fatalf("%s: %d %s", lang, w.Code, w.Body.String())
		}
		var v struct {
			Language string `json:"language"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &v); err != nil || v.Language != lang {
			t.Fatalf("%s: language %q, %v", lang, v.Language, err)
		}
		if got, want := w.Header().Get("ETag"), `"`+sha(w.Body.Bytes())+`"`; got != want {
			t.Fatalf("%s: ETag %q, want %q", lang, got, want)
		}
		if w.Header().Get("Content-Disposition") != "" || w.Header().Get("Cache-Control") != "private, no-cache" ||
			w.Header().Get("Content-Type") != "application/json" {
			t.Fatalf("%s: headers %v", lang, w.Header())
		}
	}
	wantProblem(t, ts.as(t, &c.eve, "GET", base, "", nil), 400, CodeRequestInvalid, map[string]string{"lang": "missing"})
	wantProblem(t, ts.as(t, &c.eve, "GET", base+"?lang=fr", "", nil), 400, CodeRequestInvalid, map[string]string{"lang": "invalid"})

	queued := ts.seedReport(t, stored{owner: c.ops.user.ID, status: "queued", phase: "waiting"})
	wantProblem(t, ts.as(t, &c.eve, "GET", "/api/v1/reports/"+queued.String()+"/view?lang=en", "", nil), 409, "SNAPSHOT_NOT_READY", nil)
}

func TestDownloadsCarryTheirTags(t *testing.T) {
	ts := newTestServer(t)
	c := ts.cast(t)
	snap := []byte(`{"stored":"as written","n":1}`)
	id := ts.seedReport(t, stored{owner: c.ops.user.ID, snapshot: snap})
	base := "/api/v1/reports/" + id.String()

	w := ts.as(t, &c.eve, "GET", base+"/snapshot", "", nil)
	if w.Code != 200 || !bytes.Equal(w.Body.Bytes(), snap) {
		t.Fatalf("snapshot: %d %q", w.Code, w.Body.String())
	}
	want := map[string]string{
		"ETag":                `"` + sha(snap) + `"`,
		"Cache-Control":       "private, no-cache",
		"Content-Type":        "application/json",
		"Content-Disposition": `attachment; filename="report-` + id.String() + `.json"`,
	}
	for k, v := range want {
		if got := w.Header().Get(k); got != v {
			t.Errorf("snapshot %s = %q, want %q", k, got, v)
		}
	}

	w = ts.as(t, &c.eve, "GET", base+"/pdf?lang=en", "", nil)
	if w.Code != 200 || !bytes.Equal(w.Body.Bytes(), pdfBody) {
		t.Fatalf("pdf: %d %q", w.Code, w.Body.String())
	}
	want = map[string]string{
		"ETag":                `"` + sha(pdfBody) + `"`,
		"Cache-Control":       "private, no-cache",
		"Content-Type":        "application/pdf",
		"Content-Disposition": `attachment; filename="report-` + id.String() + `-en.pdf"`,
	}
	for k, v := range want {
		if got := w.Header().Get(k); got != v {
			t.Errorf("pdf %s = %q, want %q", k, got, v)
		}
	}
	wantProblem(t, ts.as(t, &c.eve, "GET", base+"/pdf", "", nil), 400, CodeRequestInvalid, map[string]string{"lang": "missing"})
	wantProblem(t, ts.as(t, &c.eve, "GET", base+"/pdf?lang=fr", "", nil), 400, CodeRequestInvalid, map[string]string{"lang": "invalid"})

	w = ts.as(t, &c.eve, "GET", base+"/pdf?lang=pt-BR", "", map[string]string{"Range": "bytes=0-4"})
	if w.Code != 206 || w.Body.String() != "%PDF-" {
		t.Fatalf("range: %d %q", w.Code, w.Body.String())
	}
}

func TestConditionalAnswersAfterAuthorization(t *testing.T) {
	ts := newTestServer(t)
	c := ts.cast(t)
	golden := repoFile(t, "testdata", "snapshot", "direct-complete.json")
	id := ts.seedReport(t, stored{owner: c.ops.user.ID, snapshot: golden})
	expired := ts.seedReport(t, stored{owner: c.ops.user.ID, snapshot: golden, expires: ts.now.Add(-time.Hour)})
	deleted := ts.seedReport(t, stored{owner: c.ops.user.ID, snapshot: golden})
	ts.markDeleted(t, deleted)
	queued := ts.seedReport(t, stored{owner: c.ops.user.ID, status: "queued", phase: "waiting"})

	for _, route := range []string{"snapshot", "pdf?lang=en", "view?lang=en"} {
		t.Run(route, func(t *testing.T) {
			at := func(report uuid.UUID) string { return "/api/v1/reports/" + report.String() + "/" + route }
			first := ts.as(t, &c.eve, "GET", at(id), "", nil)
			tag := first.Header().Get("ETag")
			if first.Code != 200 || tag == "" {
				t.Fatalf("first: %d %v", first.Code, first.Header())
			}
			for _, match := range []string{tag, "W/" + tag, `"x", ` + tag, "*"} {
				w := ts.as(t, &c.eve, "GET", at(id), "", map[string]string{"If-None-Match": match})
				if w.Code != 304 || w.Body.Len() != 0 || w.Header().Get("ETag") != tag || w.Header().Get("Cache-Control") != "private, no-cache" {
					t.Errorf("If-None-Match %q: %d %q %v", match, w.Code, w.Body.String(), w.Header())
				}
			}
			if w := ts.as(t, &c.eve, "GET", at(id), "", map[string]string{"If-None-Match": `"x"`}); w.Code != 200 {
				t.Errorf("other tag: %d", w.Code)
			}
			match := map[string]string{"If-None-Match": tag}
			wantProblem(t, ts.as(t, &c.eve, "GET", at(expired), "", match), 410, "REPORT_EXPIRED", nil)
			wantProblem(t, ts.as(t, &c.eve, "GET", at(deleted), "", match), 404, "REPORT_NOT_FOUND", nil)
			wantProblem(t, ts.as(t, &c.eve, "GET", at(queued), "", match), 409, "SNAPSHOT_NOT_READY", nil)
			wantProblem(t, ts.as(t, nil, "GET", at(id), "", match), 401, CodeUnauthenticated, nil)
		})
	}
}

func TestPDFNotReadyRoute(t *testing.T) {
	ts := newTestServer(t)
	c := ts.cast(t)
	pending := ts.seedReport(t, stored{owner: c.ops.user.ID, status: "collected", phase: "rendering", pdfs: "pending"})
	wantProblem(t, ts.as(t, &c.eve, "GET", "/api/v1/reports/"+pending.String()+"/pdf?lang=en", "", nil), 409, "PDF_NOT_READY",
		map[string]string{"status": "pending"})
	failed := ts.seedReport(t, stored{owner: c.ops.user.ID, status: "collected", phase: "complete", pdfs: "failed"})
	wantProblem(t, ts.as(t, &c.eve, "GET", "/api/v1/reports/"+failed.String()+"/pdf?lang=en", "", nil), 409, "PDF_NOT_READY",
		map[string]string{"status": "failed", "failure_code": "PDF_COMPILE_FAILED"})
}

func TestRetryRoute(t *testing.T) {
	ts := newTestServer(t)
	c := ts.cast(t)
	failed := ts.seedReport(t, stored{owner: c.ops.user.ID, status: "collected", phase: "complete", pdfs: "failed"})
	w := ts.as(t, &c.ops, "POST", "/api/v1/reports/"+failed.String()+"/pdf-retries", `{"language":"en"}`, nil)
	if w.Code != 202 || strings.TrimSpace(w.Body.String()) != `{"language":"en","status":"pending"}` {
		t.Fatalf("retry: %d %s", w.Code, w.Body.String())
	}
	ready := ts.seedReport(t, stored{owner: c.ops.user.ID})
	wantProblem(t, ts.as(t, &c.ops, "POST", "/api/v1/reports/"+ready.String()+"/pdf-retries", `{"language":"en"}`, nil), 409, "PDF_RETRY_NOT_ALLOWED", nil)
	wantProblem(t, ts.as(t, &c.ops, "POST", "/api/v1/reports/"+ready.String()+"/pdf-retries", `{"language":"fr"}`, nil), 400, CodeRequestInvalid, nil)
	wantProblem(t, ts.as(t, &c.ops, "POST", "/api/v1/reports/"+uuid.NewString()+"/pdf-retries", `{"language":"en"}`, nil), 404, "REPORT_NOT_FOUND", nil)
	wantProblem(t, ts.as(t, &c.ops, "POST", "/api/v1/reports/nope/pdf-retries", `{"language":"en"}`, nil), 404, "REPORT_NOT_FOUND", nil)
	wantProblem(t, ts.as(t, &c.ops, "POST", "/api/v1/reports/nope/pdf-retries", `{"language":"fr"}`, nil), 400, CodeRequestInvalid, map[string]string{"language": "invalid"})
}

func TestDeleteRoute(t *testing.T) {
	ts := newTestServer(t)
	c := ts.cast(t)
	own := ts.seedReport(t, stored{owner: c.ops.user.ID})
	theirs := ts.seedReport(t, stored{owner: c.other.user.ID})
	expired := ts.seedReport(t, stored{owner: c.ops.user.ID, expires: ts.now.Add(-time.Hour)})
	del := func(a *actor, id string) *httptest.ResponseRecorder {
		return ts.as(t, a, "DELETE", "/api/v1/reports/"+id, "", nil)
	}

	if w := del(&c.ops, own.String()); w.Code != 204 || w.Body.Len() != 0 {
		t.Fatalf("own: %d %q", w.Code, w.Body.String())
	}
	wantProblem(t, ts.as(t, &c.eve, "GET", "/api/v1/reports/"+own.String(), "", nil), 404, "REPORT_NOT_FOUND", nil)
	wantProblem(t, del(&c.ops, theirs.String()), 403, CodeForbidden, nil)
	wantProblem(t, del(&c.ops, expired.String()), 410, "REPORT_EXPIRED", nil)
	wantProblem(t, del(&c.ops, uuid.NewString()), 404, "REPORT_NOT_FOUND", nil)
	if w := del(&c.root, theirs.String()); w.Code != 204 {
		t.Fatalf("administrator: %d %s", w.Code, w.Body.String())
	}
}

func TestBulkDeleteRoute(t *testing.T) {
	ts := newTestServer(t)
	c := ts.cast(t)
	own := ts.seedReport(t, stored{owner: c.ops.user.ID})
	theirs := ts.seedReport(t, stored{owner: c.other.user.ID})
	expired := ts.seedReport(t, stored{owner: c.ops.user.ID, expires: ts.now.Add(-time.Hour)})
	absent := uuid.New()
	gone := ts.seedReport(t, stored{owner: c.ops.user.ID})
	ts.markDeleted(t, gone)
	bulk := func(ids ...string) string {
		b, _ := json.Marshal(map[string][]string{"ids": ids})
		return string(b)
	}
	post := func(body string) *httptest.ResponseRecorder {
		return ts.as(t, &c.ops, "POST", "/api/v1/reports/delete", body, nil)
	}

	many := make([]string, 201)
	for i := range many {
		many[i] = uuid.NewString()
	}
	wantProblem(t, post(bulk(many...)), 400, CodeRequestInvalid, map[string]string{"ids": "too_many"})
	wantProblem(t, post(bulk(own.String(), own.String())), 400, CodeRequestInvalid, map[string]string{"ids": "repeated"})
	wantProblem(t, post(bulk(own.String(), "not-a-uuid")), 400, CodeRequestInvalid, map[string]string{"ids": "invalid"})
	for _, raw := range []string{`{"ids":[1]}`, `{"ids":[null]}`, `{"ids":[true]}`, `{"ids":[["x"]]}`} {
		wantProblem(t, post(raw), 400, CodeRequestInvalid, map[string]string{"ids": "invalid"})
	}
	wantProblem(t, post(bulk()), 400, CodeRequestInvalid, map[string]string{"ids": "empty"})
	wantProblem(t, post(`{}`), 400, CodeRequestInvalid, map[string]string{"ids": "missing"})
	if w := ts.as(t, &c.eve, "GET", "/api/v1/reports/"+own.String(), "", nil); w.Code != 200 {
		t.Fatalf("a refused body deleted a report: %d", w.Code)
	}

	w := post(bulk(own.String(), theirs.String(), expired.String(), absent.String(), gone.String()))
	if w.Code != 200 {
		t.Fatalf("mix: %d %s", w.Code, w.Body.String())
	}
	got := decodeStrict[ReportDeleteResults](t, w)
	want := []ReportDeleteResult{
		{own, ReportDeleteOutcomeDeleted}, {theirs, ReportDeleteOutcomeForbidden}, {expired, ReportDeleteOutcomeExpired},
		{absent, ReportDeleteOutcomeNotFound}, {gone, ReportDeleteOutcomeNotFound},
	}
	if fmt.Sprint(got.Results) != fmt.Sprint(want) {
		t.Fatalf("results %v, want %v", got.Results, want)
	}
}

// raceReaders runs readers that fetch the PDF of id in a loop while action
// runs once, after the first whole 200. Every answer must be a whole 200
// with the stored bytes, a 404, or a 410. It returns how many of each were
// seen.
func (ts *testServer) raceReaders(t *testing.T, a *actor, id uuid.UUID, body []byte, action func()) (ok, notFound, expired int64) {
	t.Helper()
	path := "/api/v1/reports/" + id.String() + "/pdf?lang=en"
	var okN, nfN, exN atomic.Int64
	done := make(chan struct{})
	var readers sync.WaitGroup
	for i := 0; i < 4; i++ {
		readers.Add(1)
		go func() {
			defer readers.Done()
			deadline := time.Now().Add(30 * time.Second)
			finished := 0
			for finished < 5 {
				if time.Now().After(deadline) {
					t.Errorf("no end after 30 s")
					return
				}
				w := ts.as(t, a, "GET", path, "", nil)
				switch w.Code {
				case 200:
					if !bytes.Equal(w.Body.Bytes(), body) {
						t.Errorf("a 200 with %d bytes, stored %d", w.Body.Len(), len(body))
						return
					}
					okN.Add(1)
				case 404:
					nfN.Add(1)
				case 410:
					exN.Add(1)
				default:
					t.Errorf("status %d: %s", w.Code, w.Body.String())
					return
				}
				select {
				case <-done:
					finished++
				default:
				}
			}
		}()
	}
	for okN.Load() == 0 {
		time.Sleep(time.Millisecond)
	}
	action()
	close(done)
	readers.Wait()
	return okN.Load(), nfN.Load(), exN.Load()
}

func TestDownloadRacesTheSweep(t *testing.T) {
	ts := newTestServer(t)
	c := ts.cast(t)
	ctx := context.Background()
	body := bytes.Repeat([]byte("%PDF-race-0123456789abcdef\n"), 20000)

	t.Run("delete and sweep", func(t *testing.T) {
		id := ts.seedReport(t, stored{owner: c.ops.user.ID, pdfBody: body})
		ok, notFound, expired := ts.raceReaders(t, &c.eve, id, body, func() {
			if err := ts.Reports.Delete(ctx, auth.Principal{ID: c.ops.user.ID, Role: auth.RoleOperator}, id); err != nil {
				t.Errorf("delete: %v", err)
				return
			}
			err := store.Transact(ctx, ts.pool, func(tx pgx.Tx) error {
				_, err := store.SweepDeletedReports(ctx, tx, 100)
				return err
			})
			if err != nil {
				t.Errorf("sweep: %v", err)
			}
		})
		if ok == 0 || notFound == 0 || expired != 0 {
			t.Fatalf("saw %d answers of 200, %d of 404, %d of 410", ok, notFound, expired)
		}
	})

	t.Run("expire", func(t *testing.T) {
		id := ts.seedReport(t, stored{owner: c.ops.user.ID, pdfBody: body, expires: ts.now.Add(time.Hour)})
		ok, notFound, expired := ts.raceReaders(t, &c.eve, id, body, func() {
			err := store.Transact(ctx, ts.pool, func(tx pgx.Tx) error {
				ids, err := store.ExpireDueReports(ctx, tx, ts.now.Add(2*time.Hour), 100)
				if err == nil && !slices.Contains(ids, id) {
					t.Errorf("the report was not expired: %v", ids)
				}
				return err
			})
			if err != nil {
				t.Errorf("expire: %v", err)
			}
		})
		if ok == 0 || expired == 0 || notFound != 0 {
			t.Fatalf("saw %d answers of 200, %d of 404, %d of 410", ok, notFound, expired)
		}
	})

	t.Run("states", func(t *testing.T) {
		path := func(id uuid.UUID) string { return "/api/v1/reports/" + id.String() + "/pdf?lang=en" }
		marked := ts.seedReport(t, stored{owner: c.ops.user.ID})
		ts.markDeleted(t, marked)
		wantProblem(t, ts.as(t, &c.eve, "GET", path(marked), "", nil), 404, "REPORT_NOT_FOUND", nil)

		swept := ts.seedReport(t, stored{owner: c.ops.user.ID})
		ts.markDeleted(t, swept)
		err := store.Transact(ctx, ts.pool, func(tx pgx.Tx) error {
			_, err := store.SweepDeletedReports(ctx, tx, 100)
			return err
		})
		if err != nil {
			t.Fatal(err)
		}
		wantProblem(t, ts.as(t, &c.eve, "GET", path(swept), "", nil), 404, "REPORT_NOT_FOUND", nil)

		gone := ts.seedReport(t, stored{owner: c.ops.user.ID, expires: ts.now.Add(time.Hour)})
		err = store.Transact(ctx, ts.pool, func(tx pgx.Tx) error {
			_, err := store.ExpireDueReports(ctx, tx, ts.now.Add(2*time.Hour), 100)
			return err
		})
		if err != nil {
			t.Fatal(err)
		}
		if n := ts.count(t, `SELECT count(*) FROM report_pdfs WHERE report_id = $1`, gone); n != 0 {
			t.Fatalf("%d pdf rows remain", n)
		}
		wantProblem(t, ts.as(t, &c.eve, "GET", path(gone), "", nil), 410, "REPORT_EXPIRED", nil)
	})
}

func TestReportRoutesWithoutAServiceAnswer500(t *testing.T) {
	ts := newTestServer(t)
	c := ts.cast(t)
	ts.Reports = nil
	wantProblem(t, ts.as(t, &c.eve, "GET", "/api/v1/reports", "", nil), 500, CodeInternalError, nil)
	wantProblem(t, ts.as(t, &c.ops, "POST", "/api/v1/reports/delete", `{"ids":[]}`, nil), 500, CodeInternalError, nil)
	wantProblem(t, ts.as(t, nil, "GET", "/api/v1/reports", "", nil), 401, CodeUnauthenticated, nil)
}
