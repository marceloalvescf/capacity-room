//go:build postgres

package worker

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"github.com/riverqueue/river/rivertype"

	"github.com/marceloalvescf/k8s-capacity-report/internal/collect"
	"github.com/marceloalvescf/k8s-capacity-report/internal/pipeline"
	"github.com/marceloalvescf/k8s-capacity-report/internal/platform/auth"
	"github.com/marceloalvescf/k8s-capacity-report/internal/platform/netpolicy"
	"github.com/marceloalvescf/k8s-capacity-report/internal/platform/profiles"
	"github.com/marceloalvescf/k8s-capacity-report/internal/platform/reports"
	"github.com/marceloalvescf/k8s-capacity-report/internal/platform/settings"
	"github.com/marceloalvescf/k8s-capacity-report/internal/platform/store"
	"github.com/marceloalvescf/k8s-capacity-report/internal/platform/storetest"
	"github.com/marceloalvescf/k8s-capacity-report/internal/promtest"
	"github.com/marceloalvescf/k8s-capacity-report/internal/redact"
	"github.com/marceloalvescf/k8s-capacity-report/internal/snapshot"
)

var fixedNow = time.Date(2026, 10, 6, 12, 0, 0, 0, time.UTC)

const (
	testVersion = "0.0.0-test"
	secretToken = "s3cr3t-t0ken-value"
	waitLimit   = 30 * time.Second
)

// syncBuffer is a log destination that several goroutines write.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// errorLog records what the handlers returned to River.
type errorLog struct {
	mu   sync.Mutex
	errs []error
}

func (l *errorLog) HandleError(_ context.Context, _ *rivertype.JobRow, err error) *river.ErrorHandlerResult {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.errs = append(l.errs, err)
	return nil
}

func (l *errorLog) HandlePanic(context.Context, *rivertype.JobRow, any, string) *river.ErrorHandlerResult {
	return nil
}

func (l *errorLog) all() []error {
	l.mu.Lock()
	defer l.mu.Unlock()
	return slices.Clone(l.errs)
}

// swapHandler serves the handler that is current, so a test can replace a
// scenario server without changing the address.
type swapHandler struct {
	current atomic.Pointer[http.Handler]
	served  atomic.Int64 // requests received
}

func (s *swapHandler) set(h http.Handler) { s.current.Store(&h) }

func (s *swapHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.served.Add(1)
	(*s.current.Load()).ServeHTTP(w, r)
}

// idleRender registers the render kind, which the collect job queues, and
// leaves every render job running until the client stops.
type idleRender struct {
	river.WorkerDefaults[reports.RenderArgs]
}

func (*idleRender) Work(ctx context.Context, _ *river.Job[reports.RenderArgs]) error {
	<-ctx.Done()
	return ctx.Err()
}

type env struct {
	t               *testing.T
	pool            *pgxpool.Pool
	profiles        *profiles.Service
	reports         *reports.Service
	settings        settings.Settings
	deps            Deps
	op              auth.Principal
	root            auth.Principal
	credDir         string
	logs            *syncBuffer
	requestLanguage string
	raw             *syncBuffer // what the logger wrote before the redactor, when redacted was called
	errs            *errorLog
	client          *river.Client[pgx.Tx]
	stopped         bool
}

func newPrincipal(t *testing.T, pool *pgxpool.Pool, name, role string) auth.Principal {
	t.Helper()
	u := store.User{ID: uuid.New(), Username: name, PasswordHash: "x", Role: role, Language: "en", Active: true, CreatedAt: fixedNow, UpdatedAt: fixedNow}
	if err := store.InsertUser(context.Background(), pool, u); err != nil {
		t.Fatal(err)
	}
	return auth.Principal{ID: u.ID, Username: name, Role: role, Language: "en"}
}

func newEnv(t *testing.T, mod func(*settings.Settings)) *env {
	t.Helper()
	pool := storetest.New(t)
	e := &env{t: t, pool: pool, credDir: t.TempDir(), logs: &syncBuffer{}, errs: &errorLog{}}
	e.settings = settings.Settings{
		CredentialDir:         e.credDir,
		CABundleDir:           t.TempDir(),
		CollectTimeout:        30 * time.Second,
		JobMaxAttempts:        3,
		ReportRetention:       7 * 24 * time.Hour,
		AdmissionLimit:        20,
		AdmissionProfileLimit: 5,
		AdmissionRetry:        5 * time.Second,
	}
	if mod != nil {
		mod(&e.settings)
	}
	t.Cleanup(func() {
		if t.Failed() {
			t.Logf("logs:\n%s", e.logs.String())
		}
	})
	e.op = newPrincipal(t, pool, "ops", auth.RoleOperator)
	e.root = newPrincipal(t, pool, "root", auth.RoleAdministrator)
	e.profiles = &profiles.Service{
		Pool:     pool,
		Policy:   netpolicy.Policy{Networks: []netip.Prefix{netip.MustParsePrefix("127.0.0.1/32")}},
		Settings: e.settings,
		Now:      func() time.Time { return fixedNow },
		Resolve: func(_ context.Context, host string) ([]netip.Addr, error) {
			if host != "metrics.test" {
				return nil, errors.New("unexpected host")
			}
			return []netip.Addr{netip.MustParseAddr("127.0.0.1")}, nil
		},
	}
	logger := slog.New(slog.NewTextHandler(e.logs, nil))
	queue, err := reports.NewQueue(pool, logger)
	if err != nil {
		t.Fatal(err)
	}
	e.reports = &reports.Service{Pool: pool, Queue: queue, Settings: e.settings, Now: func() time.Time { return fixedNow }}
	e.deps = Deps{
		Pool: pool, Profiles: e.profiles, Settings: e.settings, Version: testVersion,
		Now: func() time.Time { return fixedNow }, Logger: logger,
	}
	return e
}

// start runs a real River client with the collect worker and stops it when
// the test ends.
func (e *env) start(hooks collectHooks) {
	e.t.Helper()
	workers := river.NewWorkers()
	river.AddWorker(workers, &CollectWorker{Deps: e.deps, hooks: hooks})
	river.AddWorker(workers, &idleRender{})
	client, err := river.NewClient(riverpgxv5.New(e.pool), &river.Config{
		TestOnly:     true,
		Logger:       slog.New(slog.NewTextHandler(e.logs, nil)),
		Queues:       map[string]river.QueueConfig{"reports": {MaxWorkers: 1}},
		Workers:      workers,
		ErrorHandler: e.errs,
	})
	if err != nil {
		e.t.Fatal(err)
	}
	if err := client.Start(context.Background()); err != nil {
		e.t.Fatal(err)
	}
	e.client = client
	e.t.Cleanup(func() {
		if e.stopped {
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := client.StopAndCancel(ctx); err != nil {
			e.t.Errorf("stop client: %v", err)
		}
	})
}

func (e *env) addProfile(endpoint, credentialFile string) {
	e.t.Helper()
	d := profiles.Defaults()
	d.DisplayName = "Production"
	d.Active = true
	d.EndpointURL = endpoint
	d.EndpointType = "prometheus"
	d.ClusterName = "prod"
	d.AllowedNamespaces = []string{"payments"}
	d.CredentialFile = credentialFile
	if _, err := e.profiles.Create(context.Background(), e.root, "prod", d); err != nil {
		e.t.Fatal(err)
	}
}

// language is the language a request of the env names; pt-BR unless a test sets requestLanguage.
func (e *env) language() string {
	if e.requestLanguage != "" {
		return e.requestLanguage
	}
	return "pt-BR"
}

// requestPDF asks for the PDF of lang as the operator of the env.
func (e *env) requestPDF(id uuid.UUID, lang string) {
	e.t.Helper()
	if err := e.reports.RequestPDF(context.Background(), e.op, id, lang); err != nil {
		e.t.Fatal(err)
	}
}

// request queues a report and returns it with its collect job.
func (e *env) request() store.Report {
	e.t.Helper()
	acc, err := e.reports.Request(context.Background(), e.op, uuid.NewString(),
		reports.Selection{ProfileID: "prod", Namespaces: []string{"payments"}, Language: ptrTo(e.language())})
	if err != nil {
		e.t.Fatal(err)
	}
	r, err := store.GetReport(context.Background(), e.pool, acc.ID)
	if err != nil {
		e.t.Fatal(err)
	}
	return r
}

func (e *env) report(id uuid.UUID) store.Report {
	e.t.Helper()
	r, err := store.GetReport(context.Background(), e.pool, id)
	if err != nil {
		e.t.Fatal(err)
	}
	return r
}

// waitJob waits for the job to be in one of the states and returns its row.
func (e *env) waitJob(id int64, states ...rivertype.JobState) *rivertype.JobRow {
	e.t.Helper()
	deadline := time.Now().Add(waitLimit)
	var last rivertype.JobState
	for time.Now().Before(deadline) {
		job, err := e.client.JobGet(context.Background(), id)
		if err != nil {
			e.t.Fatal(err)
		}
		last = job.State
		if slices.Contains(states, job.State) {
			return job
		}
		time.Sleep(50 * time.Millisecond)
	}
	e.t.Fatalf("job %d stayed %s, want one of %v", id, last, states)
	return nil
}

func (e *env) count(query string, args ...any) int {
	e.t.Helper()
	var n int
	if err := e.pool.QueryRow(context.Background(), query, args...).Scan(&n); err != nil {
		e.t.Fatal(err)
	}
	return n
}

type auditRow struct {
	Outcome string
	ActorID *uuid.UUID
	Meta    map[string]any
}

func (e *env) audits(action string) []auditRow {
	e.t.Helper()
	events, err := store.ListAuditEvents(context.Background(), e.pool)
	if err != nil {
		e.t.Fatal(err)
	}
	var out []auditRow
	for _, ev := range events {
		if ev.Action != action {
			continue
		}
		var m map[string]any
		if err := json.Unmarshal(ev.Metadata, &m); err != nil {
			e.t.Fatal(err)
		}
		out = append(out, auditRow{Outcome: ev.Outcome, ActorID: ev.ActorID, Meta: m})
	}
	return out
}

func scenarioFixture(t *testing.T) promtest.Fixture {
	t.Helper()
	path, err := promtest.Path("direct-complete")
	if err != nil {
		t.Fatal(err)
	}
	f, err := promtest.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	return f
}

func endpointOf(ts *httptest.Server) string {
	addr := ts.Listener.Addr().String()
	return "http://metrics.test:" + addr[strings.LastIndex(addr, ":")+1:] + "/api/v1"
}

// serve starts a server that answers with h and returns its profile endpoint.
func serve(t *testing.T, h http.Handler) string {
	t.Helper()
	ts := httptest.NewServer(h)
	t.Cleanup(ts.Close)
	return endpointOf(ts)
}

func closedEndpoint(t *testing.T) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := l.Addr().String()
	_ = l.Close()
	return "http://metrics.test:" + addr[strings.LastIndex(addr, ":")+1:] + "/api/v1"
}

// preflightRequests is the number of requests of the preflight of
// direct-complete.
const preflightRequests = 29

// emptyCollectors answers the preflight of direct-complete as recorded,
// except the probe of the cluster autoscaler metrics, and every later
// request with an empty result.
func emptyCollectors(t *testing.T) http.Handler {
	t.Helper()
	fake := promtest.NewServer(scenarioFixture(t))
	var received atomic.Int64
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		n := received.Add(1)
		if n <= preflightRequests && !strings.Contains(r.PostFormValue("query"), "cluster_autoscaler_") {
			fake.ServeHTTP(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		resultType := "vector"
		if strings.HasSuffix(r.URL.Path, "query_range") {
			resultType = "matrix"
		}
		_, _ = fmt.Fprintf(w, `{"status":"success","data":{"resultType":%q,"result":[]}}`, resultType)
	})
}

// tokenOf returns the bearer token the scenario requires, or "".
func tokenOf(f promtest.Fixture) string {
	for _, r := range f.Requests {
		if token, ok := strings.CutPrefix(r.Authorization, "Bearer "); ok {
			return token
		}
	}
	return ""
}

func (e *env) writeCredential(name, content string) {
	e.t.Helper()
	if err := os.WriteFile(filepath.Join(e.credDir, name), []byte(content), 0o600); err != nil {
		e.t.Fatal(err)
	}
}

// scenarioEnv serves direct-complete, adds the profile that reaches it, and
// returns the swappable server.
func scenarioEnv(t *testing.T, mod func(*settings.Settings)) (*env, *swapHandler, string) {
	t.Helper()
	e := newEnv(t, mod)
	fixture := scenarioFixture(t)
	sw := &swapHandler{}
	sw.set(promtest.NewServer(fixture))
	endpoint := serve(t, sw)
	cred := ""
	if token := tokenOf(fixture); token != "" {
		e.writeCredential("token", token+"\n")
		cred = "token"
	}
	e.addProfile(endpoint, cred)
	return e, sw, endpoint
}

func TestCollectStoresTheSnapshot(t *testing.T) {
	e, sw, _ := scenarioEnv(t, nil)
	e.start(collectHooks{})
	report := e.request()
	job := e.waitJob(report.CollectJobID, rivertype.JobStateCompleted)
	if job.Attempt != 1 {
		t.Fatalf("attempt %d", job.Attempt)
	}

	got := e.report(report.ID)
	if got.Status != "collected" || got.Phase != "rendering" || got.Attempt != 1 || got.FailureCode != nil {
		t.Fatalf("report %+v", got)
	}
	if got.CollectedAt == nil || !got.CollectedAt.Equal(fixedNow) {
		t.Fatalf("collected_at %v", got.CollectedAt)
	}
	_, body, sum, err := store.GetSnapshot(context.Background(), e.pool, report.ID)
	if err != nil {
		t.Fatal(err)
	}

	// The same run, made by the pipeline on a fresh server of the scenario.
	sw.set(promtest.NewServer(scenarioFixture(t)))
	rev, err := e.profiles.Revision(context.Background(), report.ProfileRevisionID)
	if err != nil {
		t.Fatal(err)
	}
	var r reports.Resolved
	if err := json.Unmarshal(report.Request, &r); err != nil {
		t.Fatal(err)
	}
	cfg := rev.Document.RunConfig(r.Period, r.Timezone, r.ReportProfile, r.MinSeverity, r.Namespaces)
	opts, err := e.profiles.Source(rev.Document, cfg.Metrics)
	if err != nil {
		t.Fatal(err)
	}
	window, err := collect.NewWindow(cfg.Analysis, fixedNow)
	if err != nil {
		t.Fatal(err)
	}
	want, err := pipeline.Collect(context.Background(), pipeline.Run{
		Options: opts, Config: cfg, Window: window, EmittedAt: fixedNow,
		Now:     func() time.Time { return fixedNow },
		NewID:   func() (string, error) { return report.ID.String(), nil },
		Version: testVersion,
	})
	if err != nil {
		t.Fatal(err)
	}
	wantBody, err := snapshot.Encode(want.Snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(body, wantBody) {
		t.Fatalf("the stored snapshot (%d bytes) differs from the pipeline's (%d bytes)", len(body), len(wantBody))
	}
	if sum != digestOf(body) {
		t.Fatalf("sha256 %s", sum)
	}

	e.requestPDF(report.ID, "en")
	for _, lang := range []string{"pt-BR", "en"} {
		row, err := store.GetPDF(context.Background(), e.pool, report.ID, lang)
		if err != nil {
			t.Fatal(err)
		}
		if row.Status != "pending" {
			t.Fatalf("%s row %+v", lang, row)
		}
		var kind, queue string
		var priority int
		var args string
		if err := e.pool.QueryRow(context.Background(),
			`SELECT kind, queue, priority, args::text FROM river_job WHERE id = $1`, row.JobID).Scan(&kind, &queue, &priority, &args); err != nil {
			t.Fatal(err)
		}
		var a reports.RenderArgs
		if err := json.Unmarshal([]byte(args), &a); err != nil {
			t.Fatal(err)
		}
		if kind != "render" || queue != "reports" || priority != 1 || a.ReportID != report.ID || a.Language != lang {
			t.Fatalf("%s job %s %s %d %s", lang, kind, queue, priority, args)
		}
	}
	audits := e.audits(reports.ActionReportCollect)
	if len(audits) != 1 || audits[0].Outcome != "succeeded" || audits[0].ActorID != nil || len(audits[0].Meta) != 0 {
		t.Fatalf("audit %+v", audits)
	}
}

func TestCollectQueuesOnePDF(t *testing.T) {
	e, _, _ := scenarioEnv(t, nil)
	e.requestLanguage = "en"
	e.start(collectHooks{})
	report := e.request()
	e.waitJob(report.CollectJobID, rivertype.JobStateCompleted)
	if got := e.report(report.ID); got.Status != "collected" {
		t.Fatalf("report %+v", got)
	}
	if n := e.count(`SELECT count(*) FROM report_pdfs WHERE report_id = $1`, report.ID); n != 1 {
		t.Fatalf("%d PDF rows", n)
	}
	row, err := store.GetPDF(context.Background(), e.pool, report.ID, "en")
	if err != nil || row.Status != "pending" {
		t.Fatalf("row %+v, %v", row, err)
	}
	if n := e.count(`SELECT count(*) FROM river_job WHERE kind = 'render'`); n != 1 {
		t.Fatalf("%d render jobs", n)
	}
}

func TestCollectFailsAStoredRequestWithoutLanguage(t *testing.T) {
	e, _, _ := scenarioEnv(t, func(s *settings.Settings) { s.JobMaxAttempts = 1 })
	report := e.request()
	if _, err := e.pool.Exec(context.Background(), `UPDATE reports SET request = request - 'language' WHERE id = $1`, report.ID); err != nil {
		t.Fatal(err)
	}
	e.start(collectHooks{})
	e.assertFailedWith(report, "INTERNAL_FAILURE")
	if n := e.count(`SELECT count(*) FROM report_pdfs`); n != 0 {
		t.Fatalf("%d PDF rows", n)
	}
}

func ptrTo(s string) *string { return &s }

func digestOf(body []byte) string {
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}

func TestCollectPhases(t *testing.T) {
	e, _, _ := scenarioEnv(t, nil)
	ctx := context.Background()
	for _, stmt := range []string{
		`CREATE TABLE phase_log (id bigserial PRIMARY KEY, phase text NOT NULL)`,
		`CREATE FUNCTION log_phase() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN INSERT INTO phase_log (phase) VALUES (NEW.phase); RETURN NEW; END $$`,
		`CREATE TRIGGER log_phase AFTER UPDATE OF phase ON reports FOR EACH ROW EXECUTE FUNCTION log_phase()`,
	} {
		if _, err := e.pool.Exec(ctx, stmt); err != nil {
			t.Fatal(err)
		}
	}
	e.start(collectHooks{})
	report := e.request()
	e.waitJob(report.CollectJobID, rivertype.JobStateCompleted)

	rows, err := e.pool.Query(ctx, `SELECT phase FROM phase_log ORDER BY id`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var got []string
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			t.Fatal(err)
		}
		got = append(got, p)
	}
	want := []string{"preflight", "collecting", "analyzing", "snapshotting", "rendering"}
	if !slices.Equal(got, want) {
		t.Fatalf("phases %v, want %v", got, want)
	}
}

func (e *env) assertFailedWith(report store.Report, code string) {
	e.t.Helper()
	job := e.waitJob(report.CollectJobID, rivertype.JobStateCompleted)
	if job.Attempt != 1 {
		e.t.Fatalf("%s: attempt %d", code, job.Attempt)
	}
	got := e.report(report.ID)
	if got.Status != "failed" || got.Phase != "complete" || got.FailureCode == nil || *got.FailureCode != code {
		e.t.Fatalf("%s: report %+v", code, got)
	}
	audits := e.audits(reports.ActionReportCollect)
	if len(audits) != 1 || audits[0].Outcome != "failed" || audits[0].ActorID != nil || len(audits[0].Meta) != 1 || audits[0].Meta["code"] != code {
		e.t.Fatalf("%s: audit %+v", code, audits)
	}
	if n := e.count(`SELECT count(*) FROM report_snapshots`) + e.count(`SELECT count(*) FROM report_pdfs`); n != 0 {
		e.t.Fatalf("%s: %d stored rows", code, n)
	}
}

func TestCollectTerminalCodes(t *testing.T) {
	t.Run("PROFILE_CREDENTIAL_UNAVAILABLE", func(t *testing.T) {
		e := newEnv(t, nil)
		e.addProfile(serve(t, promtest.NewServer(scenarioFixture(t))), "missing-token")
		e.start(collectHooks{})
		e.assertFailedWith(e.request(), "PROFILE_CREDENTIAL_UNAVAILABLE")
	})
	t.Run("ENDPOINT_AUTH_REJECTED", func(t *testing.T) {
		e := newEnv(t, nil)
		e.addProfile(serve(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
		})), "")
		e.start(collectHooks{})
		e.assertFailedWith(e.request(), "ENDPOINT_AUTH_REJECTED")
	})
	t.Run("ENDPOINT_ADDRESS_REJECTED", func(t *testing.T) {
		e := newEnv(t, nil)
		e.addProfile(serve(t, promtest.NewServer(scenarioFixture(t))), "")
		e.profiles.Policy.Networks = []netip.Prefix{netip.MustParsePrefix("10.0.0.0/8")}
		e.start(collectHooks{})
		e.assertFailedWith(e.request(), "ENDPOINT_ADDRESS_REJECTED")
	})
	t.Run("NOTHING_TO_REPORT", func(t *testing.T) {
		e := newEnv(t, nil)
		fixture := scenarioFixture(t)
		cred := ""
		if token := tokenOf(fixture); token != "" {
			e.writeCredential("token", token+"\n")
			cred = "token"
		}
		e.addProfile(serve(t, emptyCollectors(t)), cred)
		e.start(collectHooks{})
		e.assertFailedWith(e.request(), "NOTHING_TO_REPORT")
	})
}

func TestCollectRetriesAnUnreachableSource(t *testing.T) {
	e := newEnv(t, func(s *settings.Settings) { s.JobMaxAttempts = 2 })
	e.addProfile(closedEndpoint(t), "")
	second := newHold(2)
	e.start(collectHooks{beforeClaim: second.hook})
	report := e.request()

	// River keeps a job whose retry is due within its scheduler interval
	// available; the attempt counter and the error tell it has been tried.
	var first *rivertype.JobRow
	for deadline := time.Now().Add(waitLimit); first == nil && time.Now().Before(deadline); time.Sleep(20 * time.Millisecond) {
		job, err := e.client.JobGet(context.Background(), report.CollectJobID)
		if err != nil {
			t.Fatal(err)
		}
		if job.Attempt == 1 && job.State != rivertype.JobStateRunning {
			first = job
		}
	}
	if first == nil || first.State == rivertype.JobStateCompleted || first.State == rivertype.JobStateDiscarded {
		t.Fatalf("job after attempt 1: %+v", first)
	}
	if len(first.Errors) != 1 || first.Errors[0].Error != "collect: ENDPOINT_UNREACHABLE" {
		t.Fatalf("errors %+v", first.Errors)
	}
	got := e.report(report.ID)
	if got.Status != "queued" || got.Phase != "waiting" || got.FailureCode != nil || got.Attempt != 1 {
		t.Fatalf("report after attempt 1: %+v", got)
	}
	if n := len(e.audits(reports.ActionReportCollect)); n != 0 {
		t.Fatalf("%d collect audit rows after a retried attempt", n)
	}

	// Attempt 2 waits before its claim, so the reads above saw the report
	// as attempt 1 left it.
	second.waitReached(t)
	close(second.release)
	last := e.waitJob(report.CollectJobID, rivertype.JobStateCompleted)
	if last.Attempt != 2 {
		t.Fatalf("attempt %d", last.Attempt)
	}
	got = e.report(report.ID)
	if got.Status != "failed" || got.FailureCode == nil || *got.FailureCode != "ENDPOINT_UNREACHABLE" || got.Attempt != 2 {
		t.Fatalf("report after attempt 2: %+v", got)
	}
	audits := e.audits(reports.ActionReportCollect)
	if len(audits) != 1 || audits[0].Meta["code"] != "ENDPOINT_UNREACHABLE" {
		t.Fatalf("audit %+v", audits)
	}
}

func TestCollectTimeout(t *testing.T) {
	e := newEnv(t, func(s *settings.Settings) { s.CollectTimeout = time.Second; s.JobMaxAttempts = 3 })
	stop := make(chan struct{})
	e.addProfile(serve(t, http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-stop:
		}
	})), "")
	t.Cleanup(func() { close(stop) })
	e.start(collectHooks{})
	e.assertFailedWith(e.request(), "COLLECTION_TIMEOUT")
}

// A deadline that passes inside the transaction that stores the snapshot
// does not turn the finished collection into a failure.
func TestCollectSuccessOutlivesTheDeadline(t *testing.T) {
	e, _, _ := scenarioEnv(t, func(s *settings.Settings) { s.CollectTimeout = 5 * time.Second })
	e.start(collectHooks{insideCollected: func(ctx context.Context, _ int) {
		<-ctx.Done()
	}})
	report := e.request()
	e.waitJob(report.CollectJobID, rivertype.JobStateCompleted)
	if got := e.report(report.ID); got.Status != "collected" || got.FailureCode != nil {
		code := ""
		if got.FailureCode != nil {
			code = *got.FailureCode
		}
		t.Fatalf("report %s with code %q, want collected", got.Status, code)
	}
	if n := e.count(`SELECT count(*) FROM report_snapshots WHERE report_id = $1`, report.ID); n != 1 {
		t.Fatalf("%d snapshots", n)
	}
}

// A success write that runs longer than the outcome timeout while the job's
// budget remains is stored, not turned into a failure.
func TestCollectSlowSuccessWriteKeepsItsBudget(t *testing.T) {
	e, _, _ := scenarioEnv(t, func(s *settings.Settings) {
		s.CollectTimeout = 60 * time.Second
		s.JobMaxAttempts = 1
	})
	e.start(collectHooks{
		outcomeTimeout:  time.Second,
		insideCollected: func(context.Context, int) { time.Sleep(2 * time.Second) },
	})
	report := e.request()
	e.waitJob(report.CollectJobID, rivertype.JobStateCompleted)
	got := e.report(report.ID)
	if got.Status != "collected" || got.Phase != "rendering" || got.FailureCode != nil {
		code := ""
		if got.FailureCode != nil {
			code = *got.FailureCode
		}
		t.Fatalf("report %s/%s with code %q, want collected/rendering", got.Status, got.Phase, code)
	}
}

// A success write that outlives the job's own deadline is an internal
// failure of the platform, never a fault of the metric source or a collection
// timeout.
func TestCollectSuccessWritePastItsDeadlineIsInternal(t *testing.T) {
	e, _, _ := scenarioEnv(t, func(s *settings.Settings) {
		s.CollectTimeout = 3 * time.Second
		s.JobMaxAttempts = 1
	})
	e.start(collectHooks{
		outcomeTimeout: time.Second,
		insideCollected: func(ctx context.Context, _ int) {
			deadline, ok := ctx.Deadline()
			if !ok {
				t.Error("the job has no deadline")
				return
			}
			time.Sleep(time.Until(deadline.Add(time.Second)))
		},
	})
	report := e.request()
	// assertFailedWith also checks the failed audit row and its code.
	e.assertFailedWith(report, "INTERNAL_FAILURE")
}

// Any error of the success write is an internal failure, even one that wraps
// a deadline, which the metric source's classifier would call unreachable.
func TestCollectSuccessWriteErrorIsInternal(t *testing.T) {
	cases := []struct {
		name  string
		err   error
		sleep bool
	}{
		{"plain error", errors.New("injected"), false},
		{"wrapped deadline", fmt.Errorf("connect: %w", context.DeadlineExceeded), false},
		{"job deadline passed during the write", errors.New("injected"), true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			e, _, _ := scenarioEnv(t, func(s *settings.Settings) {
				s.CollectTimeout = 3 * time.Second
				s.JobMaxAttempts = 1
			})
			hooks := collectHooks{successError: c.err}
			if c.sleep {
				hooks.insideCollected = func(ctx context.Context, _ int) {
					deadline, _ := ctx.Deadline()
					time.Sleep(time.Until(deadline.Add(500 * time.Millisecond)))
				}
			}
			e.start(hooks)
			e.assertFailedWith(e.request(), "INTERNAL_FAILURE")
		})
	}
}

// hold blocks an attempt at a hook until released; it signals when attempt
// arrives. Other attempts pass.
type hold struct {
	attempt int
	reached chan struct{}
	release chan struct{}
}

func newHold(attempt int) *hold {
	return &hold{attempt: attempt, reached: make(chan struct{}), release: make(chan struct{})}
}

func (h *hold) hook(ctx context.Context, attempt int) {
	if attempt != h.attempt {
		return
	}
	close(h.reached)
	select {
	case <-h.release:
	case <-ctx.Done():
	}
}

func (h *hold) waitReached(t *testing.T) {
	t.Helper()
	select {
	case <-h.reached:
	case <-time.After(waitLimit):
		t.Fatal("the attempt never reached the hook")
	}
}

func claimAs(t *testing.T, e *env, id uuid.UUID, attempt int) {
	t.Helper()
	err := store.Transact(context.Background(), e.pool, func(tx pgx.Tx) error {
		return store.ClaimReport(context.Background(), tx, id, attempt, fixedNow)
	})
	if err != nil {
		t.Fatalf("claim as attempt %d: %v", attempt, err)
	}
}

func (e *env) assertStale(job *rivertype.JobRow) {
	e.t.Helper()
	if job.State != rivertype.JobStateDiscarded {
		e.t.Fatalf("job state %s, want discarded (never completed by the stale attempt)", job.State)
	}
	errs := e.errs.all()
	if len(errs) != 1 || !errors.Is(errs[0], ErrStaleAttempt) || errs[0].Error() != "collect: STALE_ATTEMPT" {
		e.t.Fatalf("returned errors %v, want collect: STALE_ATTEMPT matching ErrStaleAttempt", errs)
	}
	if len(job.Errors) != 1 || job.Errors[0].Error != "collect: STALE_ATTEMPT" {
		e.t.Fatalf("river recorded %+v", job.Errors)
	}
}

func TestCollectLateClaimLoses(t *testing.T) {
	e, sw, _ := scenarioEnv(t, func(s *settings.Settings) { s.JobMaxAttempts = 1 })
	h := newHold(1)
	e.start(collectHooks{beforeClaim: h.hook})
	report := e.request()
	h.waitReached(t)
	claimAs(t, e, report.ID, 2)
	close(h.release)

	job := e.waitJob(report.CollectJobID, rivertype.JobStateDiscarded, rivertype.JobStateCompleted)
	e.assertStale(job)
	got := e.report(report.ID)
	if got.Status != "running" || got.Attempt != 2 || got.Phase != "preflight" {
		t.Fatalf("report %+v", got)
	}
	if n := sw.served.Load(); n != 0 {
		t.Fatalf("the stale attempt sent %d requests to the source", n)
	}
	if n := e.count(`SELECT count(*) FROM report_snapshots`); n != 0 {
		t.Fatalf("%d snapshots", n)
	}
}

func TestCollectStaleAttemptWritesNothing(t *testing.T) {
	e, _, _ := scenarioEnv(t, func(s *settings.Settings) { s.JobMaxAttempts = 1 })
	h := newHold(1)
	e.start(collectHooks{beforeCollected: h.hook})
	report := e.request()
	h.waitReached(t)
	claimAs(t, e, report.ID, 2)
	close(h.release)

	job := e.waitJob(report.CollectJobID, rivertype.JobStateDiscarded, rivertype.JobStateCompleted)
	e.assertStale(job)
	got := e.report(report.ID)
	if got.Status != "running" || got.Attempt != 2 {
		t.Fatalf("report %+v", got)
	}
	for _, table := range []string{"report_snapshots", "report_pdfs"} {
		if n := e.count(`SELECT count(*) FROM ` + table); n != 0 {
			t.Fatalf("%d rows in %s", n, table)
		}
	}
	if n := e.count(`SELECT count(*) FROM audit_events WHERE action = $1`, reports.ActionReportCollect); n != 0 {
		t.Fatalf("%d collect audit rows", n)
	}
	if n := e.count(`SELECT count(*) FROM river_job WHERE kind = 'render'`); n != 0 {
		t.Fatalf("%d render jobs", n)
	}
}

func TestCollectRunsAnInactiveRevision(t *testing.T) {
	e, _, _ := scenarioEnv(t, nil)
	ctx := context.Background()
	current, err := e.profiles.Get(ctx, e.root, "prod")
	if err != nil {
		t.Fatal(err)
	}
	inactive, err := e.profiles.Revise(ctx, e.root, "prod", profiles.Change{
		BaseRevisionID: current.RevisionID,
		Fields:         map[string]json.RawMessage{"active": json.RawMessage("false")},
	})
	if err != nil {
		t.Fatal(err)
	}
	if inactive.Document.Active {
		t.Fatal("the revision is still active")
	}
	request, err := json.Marshal(reports.Resolved{
		Namespaces: []string{"payments"}, Period: "24h", Timezone: "America/Sao_Paulo", ReportProfile: "detailed", MinSeverity: "all", Language: "en",
	})
	if err != nil {
		t.Fatal(err)
	}
	id := uuid.New()
	var jobID int64
	err = store.Transact(ctx, e.pool, func(tx pgx.Tx) error {
		job, err := e.reports.Queue.InsertTx(ctx, tx, reports.CollectArgs{ReportID: id}, reports.CollectInsertOpts(e.settings))
		if err != nil {
			return err
		}
		jobID = job.Job.ID
		return store.InsertReport(ctx, tx, store.Report{
			ID: id, OwnerID: e.op.ID, ProfileID: "prod", ProfileRevisionID: inactive.RevisionID, Request: request,
			Status: "queued", Phase: "waiting", CollectJobID: jobID,
			CreatedAt: fixedNow, ExpiresAt: fixedNow.Add(time.Hour), UpdatedAt: fixedNow,
		})
	})
	if err != nil {
		t.Fatal(err)
	}
	e.start(collectHooks{})
	e.waitJob(jobID, rivertype.JobStateCompleted)
	if got := e.report(id); got.Status != "collected" {
		t.Fatalf("report %+v", got)
	}
}

// redacted makes the run's redactor and the logger as the worker command
// does: the profile service adds each credential to the redactor, and the
// logger passes every value through it before formatting and writes through
// it. A second handler writes to e.raw with no redaction, so a test can see
// what the redactor was given. With reveal false the redactor never learns
// the credential.
func (e *env) redacted(reveal bool) {
	redactor := redact.New()
	if reveal {
		e.profiles.Reveal = func(secret string) { redactor.Add(secret) }
	}
	e.raw = &syncBuffer{}
	e.deps.Logger = slog.New(slog.NewMultiHandler(
		redact.NewHandler(redactor, slog.NewTextHandler(redactor.Writer(e.logs), nil)),
		slog.NewTextHandler(e.raw, nil),
	))
}

// assertNoToken checks that token appears in no table and no log line,
// neither as written nor in the escaped form of a quoted text.
func (e *env) assertNoToken(report store.Report, token string) {
	e.t.Helper()
	forms := []string{token, escaped(token)}
	for _, table := range []string{"reports", "report_snapshots", "report_pdfs", "audit_events", "river_job"} {
		var all string
		err := e.pool.QueryRow(context.Background(),
			fmt.Sprintf(`SELECT coalesce(string_agg(to_jsonb(t)::text, ' '), '') FROM %s t`, table)).Scan(&all)
		if err != nil {
			e.t.Fatal(err)
		}
		for _, form := range forms {
			if strings.Contains(all, form) {
				e.t.Fatalf("the token is in %s", table)
			}
		}
	}
	logs := e.logs.String()
	for _, form := range forms {
		if strings.Contains(logs, form) {
			e.t.Fatalf("the logs hold the token:\n%s", logs)
		}
	}
	if !strings.Contains(logs, report.ID.String()) {
		e.t.Fatalf("the logs do not name the report:\n%s", logs)
	}
}

// escaped is token as a quoted text writes it, without the outer quotes.
func escaped(token string) string {
	q := strconv.Quote(token)
	return q[1 : len(q)-1]
}

func TestCollectNeverStoresTheToken(t *testing.T) {
	t.Run("unreachable", func(t *testing.T) {
		e := newEnv(t, func(s *settings.Settings) { s.JobMaxAttempts = 2 })
		e.writeCredential("token", secretToken+"\n")
		e.addProfile(closedEndpoint(t), "token")
		e.start(collectHooks{})
		report := e.request()
		e.waitJob(report.CollectJobID, rivertype.JobStateCompleted)
		e.assertNoToken(report, secretToken)
	})
	// A 400 with a Prometheus error body is the one answer whose text reaches
	// the error of a run. It is a refused query, which the profile
	// classification maps to ENDPOINT_UNREACHABLE, retried while attempts remain.
	refusal := func(t *testing.T, token string, reveal bool) (*env, store.Report) {
		e := newEnv(t, func(s *settings.Settings) { s.JobMaxAttempts = 2 })
		e.redacted(reveal)
		e.writeCredential("token", token+"\n")
		e.addProfile(serve(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"status":"error","errorType":"bad_data","error":"bad query near ` + escaped(token) + `"}`))
		})), "token")
		e.start(collectHooks{})
		report := e.request()
		e.waitJob(report.CollectJobID, rivertype.JobStateCompleted)
		return e, report
	}
	// The second token holds a quote and a backslash, which the log handler
	// escapes when it formats the cause.
	for _, token := range []string{secretToken, `s3cr3t"t0ken\value-123`} {
		t.Run("a refusal that holds the token, through the redactor: "+token, func(t *testing.T) {
			e, report := refusal(t, token, true)
			if !strings.Contains(e.raw.String(), escaped(token)) {
				t.Fatalf("the cause never carried the token, so nothing is proved:\n%s", e.raw.String())
			}
			e.assertNoToken(report, token)
		})
	}
	// A negative control: with Reveal off the redactor never learns the
	// token, so the log carries it. This shows that the cases above pass
	// because of Reveal, and not because the cause never held the token.
	t.Run("the redactor that never learns the token leaks it", func(t *testing.T) {
		e, _ := refusal(t, secretToken, false)
		if !strings.Contains(e.logs.String(), secretToken) {
			t.Fatal("the log line does not carry the token without Reveal, so the case above proves nothing")
		}
	})
}

func TestCollectCanceledReturnsTheAttempt(t *testing.T) {
	e, _, _ := scenarioEnv(t, nil)
	h := newHold(1)
	e.start(collectHooks{afterClaim: h.hook})
	report := e.request()
	h.waitReached(t)

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if err := e.client.StopAndCancel(ctx); err != nil {
		t.Fatal(err)
	}
	e.stopped = true

	job, err := e.client.JobGet(context.Background(), report.CollectJobID)
	if err != nil {
		t.Fatal(err)
	}
	if job.State != rivertype.JobStateAvailable || job.Attempt != 0 || len(job.Errors) != 0 {
		t.Fatalf("job %s attempt %d errors %v", job.State, job.Attempt, job.Errors)
	}
	got := e.report(report.ID)
	if got.Status != "running" || got.Attempt != 1 {
		t.Fatalf("report %+v", got)
	}
	if n := e.count(`SELECT count(*) FROM audit_events WHERE action = $1`, reports.ActionReportCollect); n != 0 {
		t.Fatalf("%d collect audit rows", n)
	}
	if errs := e.errs.all(); len(errs) != 0 {
		t.Fatalf("river recorded %v", errs)
	}
}

func TestCollectSkipsATerminalReport(t *testing.T) {
	cases := map[string]func(ctx context.Context, tx pgx.Tx, id uuid.UUID) error{
		"failed": func(ctx context.Context, tx pgx.Tx, id uuid.UUID) error {
			if err := store.ClaimReport(ctx, tx, id, 1, fixedNow); err != nil {
				return err
			}
			return store.FailReport(ctx, tx, id, 1, "ENDPOINT_UNREACHABLE", fixedNow)
		},
		"collected": func(ctx context.Context, tx pgx.Tx, id uuid.UUID) error {
			if err := store.ClaimReport(ctx, tx, id, 1, fixedNow); err != nil {
				return err
			}
			return store.CollectReport(ctx, tx, id, 1, fixedNow)
		},
		"deleted": func(ctx context.Context, tx pgx.Tx, id uuid.UUID) error {
			_, err := store.MarkReportDeleted(ctx, tx, id, fixedNow)
			return err
		},
		"expired": func(ctx context.Context, tx pgx.Tx, id uuid.UUID) error {
			if err := store.ClaimReport(ctx, tx, id, 1, fixedNow); err != nil {
				return err
			}
			if err := store.FailReport(ctx, tx, id, 1, "ENDPOINT_UNREACHABLE", fixedNow); err != nil {
				return err
			}
			_, err := tx.Exec(ctx, `UPDATE reports SET expired_at = $2 WHERE id = $1`, id, fixedNow)
			return err
		},
	}
	for name, settle := range cases {
		t.Run(name, func(t *testing.T) {
			e, sw, _ := scenarioEnv(t, nil)
			report := e.request()
			ctx := context.Background()
			if err := store.Transact(ctx, e.pool, func(tx pgx.Tx) error { return settle(ctx, tx, report.ID) }); err != nil {
				t.Fatal(err)
			}
			before := e.report(report.ID)
			e.start(collectHooks{})
			job := e.waitJob(report.CollectJobID, rivertype.JobStateCompleted)
			if job.Attempt != 1 {
				t.Fatalf("attempt %d", job.Attempt)
			}
			after := e.report(report.ID)
			if !after.UpdatedAt.Equal(before.UpdatedAt) || after.Status != before.Status || after.Phase != before.Phase || after.Attempt != before.Attempt {
				t.Fatalf("report changed: %+v, was %+v", after, before)
			}
			for _, q := range []string{`SELECT count(*) FROM report_snapshots`, `SELECT count(*) FROM report_pdfs`,
				`SELECT count(*) FROM audit_events WHERE action = 'report.collect'`, `SELECT count(*) FROM river_job WHERE kind = 'render'`} {
				if n := e.count(q); n != 0 {
					t.Fatalf("%s = %d", q, n)
				}
			}
			if n := sw.served.Load(); n != 0 {
				t.Fatalf("%d requests to the source", n)
			}
			if !strings.Contains(e.logs.String(), report.ID.String()) {
				t.Fatalf("no log line names the report:\n%s", e.logs.String())
			}
		})
	}
}

func TestCollectSettlesADeletedRunningReport(t *testing.T) {
	e, sw, _ := scenarioEnv(t, nil)
	report := e.request()
	ctx := context.Background()
	if err := store.Transact(ctx, e.pool, func(tx pgx.Tx) error {
		if err := store.ClaimReport(ctx, tx, report.ID, 1, fixedNow); err != nil {
			return err
		}
		_, err := store.MarkReportDeleted(ctx, tx, report.ID, fixedNow)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	before := e.report(report.ID)
	e.start(collectHooks{})
	e.waitJob(report.CollectJobID, rivertype.JobStateCompleted)
	if after := e.report(report.ID); after.Status != "running" || after.Attempt != 1 || !after.UpdatedAt.Equal(before.UpdatedAt) {
		t.Fatalf("report changed: %+v, was %+v", after, before)
	}
	if n := sw.served.Load(); n != 0 {
		t.Fatalf("%d requests to the source", n)
	}
	var swept []uuid.UUID
	if err := store.Transact(ctx, e.pool, func(tx pgx.Tx) (err error) {
		swept, err = store.SweepDeletedReports(ctx, tx, 100)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if len(swept) != 1 || swept[0] != report.ID {
		t.Fatalf("swept %v, want %v", swept, report.ID)
	}
	if n := e.count(`SELECT count(*) FROM reports`); n != 0 {
		t.Fatalf("%d reports left", n)
	}
}

func TestCollectSkipsAMissingReport(t *testing.T) {
	e, sw, _ := scenarioEnv(t, nil)
	e.start(collectHooks{})
	id := uuid.New()
	inserted, err := e.reports.Queue.Insert(context.Background(), reports.CollectArgs{ReportID: id}, reports.CollectInsertOpts(e.settings))
	if err != nil {
		t.Fatal(err)
	}
	job := e.waitJob(inserted.Job.ID, rivertype.JobStateCompleted)
	if job.Attempt != 1 {
		t.Fatalf("attempt %d", job.Attempt)
	}
	for _, table := range []string{"reports", "report_snapshots", "report_pdfs"} {
		if n := e.count(`SELECT count(*) FROM ` + table); n != 0 {
			t.Fatalf("%d rows in %s", n, table)
		}
	}
	if n := e.count(`SELECT count(*) FROM audit_events WHERE action LIKE 'report.%'`); n != 0 {
		t.Fatalf("%d report audit rows", n)
	}
	if n := e.count(`SELECT count(*) FROM river_job WHERE kind = 'render'`); n != 0 {
		t.Fatalf("%d render jobs", n)
	}
	if n := sw.served.Load(); n != 0 {
		t.Fatalf("%d requests to the source", n)
	}
	if !strings.Contains(e.logs.String(), id.String()) {
		t.Fatalf("no log line names the report:\n%s", e.logs.String())
	}
}

// TestCollectReadsTheClockForTheRun gives the clock a distinct value on each
// read: ten seconds times the number of reads so far.
func TestCollectReadsTheClockForTheRun(t *testing.T) {
	e, _, _ := scenarioEnv(t, nil)
	var reads atomic.Int64
	e.deps.Now = func() time.Time { return fixedNow.Add(time.Duration(reads.Add(1)) * 10 * time.Second) }
	e.start(collectHooks{})
	report := e.request()
	e.waitJob(report.CollectJobID, rivertype.JobStateCompleted)

	_, body, _, err := store.GetSnapshot(context.Background(), e.pool, report.ID)
	if err != nil {
		t.Fatal(err)
	}
	found := map[string]string{}
	var walk func(v any)
	walk = func(v any) {
		switch x := v.(type) {
		case map[string]any:
			for k, c := range x {
				if str, ok := c.(string); ok && (k == "emitted_at" || k == "collection_started_at" || k == "collection_completed_at") {
					found[k] = str
				}
				walk(c)
			}
		case []any:
			for _, c := range x {
				walk(c)
			}
		}
	}
	var tree any
	if err := json.Unmarshal(body, &tree); err != nil {
		t.Fatal(err)
	}
	walk(tree)

	// The reads, in order: the claim, the emission time, the "collecting"
	// step, the end of the collection, then the later steps and stores.
	want := map[string]time.Time{
		"emitted_at":              fixedNow.Add(20 * time.Second),
		"collection_started_at":   fixedNow.Add(20 * time.Second),
		"collection_completed_at": fixedNow.Add(40 * time.Second),
	}
	for k, w := range want {
		got, err := time.Parse(time.RFC3339, found[k])
		if err != nil || !got.Equal(w) {
			t.Fatalf("%s = %q, want %s", k, found[k], w.Format(time.RFC3339))
		}
	}
}
