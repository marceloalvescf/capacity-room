//go:build postgres

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"

	"github.com/marceloalvescf/k8s-capacity-report/internal/platform/auth"
	"github.com/marceloalvescf/k8s-capacity-report/internal/platform/profiles"
	"github.com/marceloalvescf/k8s-capacity-report/internal/platform/reports"
	"github.com/marceloalvescf/k8s-capacity-report/internal/platform/settings"
	"github.com/marceloalvescf/k8s-capacity-report/internal/platform/store"
	"github.com/marceloalvescf/k8s-capacity-report/internal/platform/storetest"
	"github.com/marceloalvescf/k8s-capacity-report/internal/promtest"
	"github.com/marceloalvescf/k8s-capacity-report/internal/redact"
	"github.com/marceloalvescf/k8s-capacity-report/internal/typst"
)

const workerWait = 20 * time.Second

// swap is an HTTP handler whose answer a test replaces while it runs.
type swap struct {
	current atomic.Pointer[http.Handler]
	seen    atomic.Int64
}

func (s *swap) set(h http.Handler) { s.current.Store(&h) }

func (s *swap) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.seen.Add(1)
	(*s.current.Load()).ServeHTTP(w, r)
}

// recorded is a fake Prometheus that answers direct-complete as recorded.
func recorded(t *testing.T) http.Handler {
	t.Helper()
	path, err := promtest.Path("direct-complete")
	if err != nil {
		t.Fatal(err)
	}
	f, err := promtest.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	return promtest.NewServer(f)
}

// credentialToken is the credential the profile of the rig names.
const credentialToken = "kcr-worker-credential-1"

// requiring answers 401 to a request without the bearer credential and
// passes every other to next.
func requiring(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer "+credentialToken {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// holding is a fake Prometheus that keeps every request until its client
// goes away.
func holding() http.Handler {
	return http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		// The body is read so that the server notices a client that left.
		_, _ = io.Copy(io.Discard, r.Body)
		<-r.Context().Done()
	})
}

// rig is a migrated database with a profile that reaches a fake Prometheus,
// the settings file of a worker for it, and the means to queue a report.
type rig struct {
	t       *testing.T
	pool    *pgxpool.Pool
	config  string
	loaded  settings.Settings
	env     Env
	prom    *swap
	reports *reports.Service
	op      auth.Principal
}

func newRig(t *testing.T, slots, drain int) *rig {
	t.Helper()
	pool := storetest.New(t)
	ctx := context.Background()
	now := time.Now().UTC()

	dir := t.TempDir()
	credentials := filepath.Join(dir, "credentials")
	if err := os.Mkdir(credentials, 0o700); err != nil {
		t.Fatal(err)
	}
	secret := filepath.Join(dir, "secret")
	if err := os.WriteFile(secret, []byte(strings.Repeat("s", 32)), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(credentials, "token"), []byte(credentialToken+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	config := filepath.Join(dir, "platform.toml")
	toml := fmt.Sprintf("server_secret_path = %q\ncredential_dir = %q\nca_bundle_dir = %q\n"+
		"allowed_endpoint_cidrs = [\"127.0.0.1/32\"]\nworker_slots = %d\nworker_drain_seconds = %d\n",
		secret, credentials, filepath.Join(dir, "ca"), slots, drain)
	if err := os.WriteFile(config, []byte(toml), 0o600); err != nil {
		t.Fatal(err)
	}

	env := platformEnv(map[string]string{
		"KCR_DATABASE_DSN": pool.Config().ConnString(),
		"KCR_TYPST":        fakeTypstPath(t),
	}, "")
	env.TempDir = t.TempDir()
	env.Version = "0.0.0-test"
	loaded, err := settings.Load(config, env.Getenv)
	if err != nil {
		t.Fatal(err)
	}

	prom := &swap{}
	prom.set(requiring(recorded(t)))
	source := httptest.NewServer(prom)
	t.Cleanup(source.Close)

	admin := store.User{ID: uuid.New(), Username: "root", PasswordHash: "x", Role: auth.RoleAdministrator, Language: "en", Active: true, CreatedAt: now, UpdatedAt: now}
	operator := store.User{ID: uuid.New(), Username: "ops", PasswordHash: "x", Role: auth.RoleOperator, Language: "en", Active: true, CreatedAt: now, UpdatedAt: now}
	for _, u := range []store.User{admin, operator} {
		if err := store.InsertUser(ctx, pool, u); err != nil {
			t.Fatal(err)
		}
	}
	doc := profiles.Defaults()
	doc.DisplayName = "Production"
	doc.Active = true
	doc.EndpointURL = "http://" + source.Listener.Addr().String() + "/api/v1"
	doc.EndpointType = "prometheus"
	doc.ClusterName = "prod"
	doc.AllowedNamespaces = []string{"payments"}
	doc.CredentialFile = "token"
	seed := &profiles.Service{Pool: pool, Settings: loaded, Now: time.Now}
	seed.Policy.Networks = loaded.AllowedEndpointCIDRs
	if _, err := seed.Create(ctx, auth.Principal{ID: admin.ID, Role: auth.RoleAdministrator}, "prod", doc); err != nil {
		t.Fatal(err)
	}

	queue, err := reports.NewQueue(pool, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	return &rig{
		t: t, pool: pool, config: config, loaded: loaded, env: env, prom: prom,
		reports: &reports.Service{Pool: pool, Queue: queue, Settings: loaded, Now: time.Now},
		op:      auth.Principal{ID: operator.ID, Username: "ops", Role: auth.RoleOperator, Language: "en"},
	}
}

// request queues a report.
func (r *rig) request() store.Report {
	r.t.Helper()
	language := "en"
	acc, err := r.reports.Request(context.Background(), r.op, uuid.NewString(),
		reports.Selection{ProfileID: "prod", Namespaces: []string{"payments"}, Language: &language})
	if err != nil {
		r.t.Fatal(err)
	}
	return r.report(acc.ID)
}

func (r *rig) report(id uuid.UUID) store.Report {
	r.t.Helper()
	report, err := store.GetReport(context.Background(), r.pool, id)
	if err != nil {
		r.t.Fatal(err)
	}
	return report
}

// waitReport waits until the report's status is one of statuses.
func (r *rig) waitReport(id uuid.UUID, statuses ...string) store.Report {
	r.t.Helper()
	deadline := time.Now().Add(workerWait)
	var last store.Report
	for time.Now().Before(deadline) {
		last = r.report(id)
		for _, s := range statuses {
			if last.Status == s {
				return last
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	r.t.Fatalf("report stayed %s/%s, want one of %v", last.Status, last.Phase, statuses)
	return last
}

// job returns the state and the attempt of a River job.
func (r *rig) job(id int64) (state string, attempt int) {
	r.t.Helper()
	err := r.pool.QueryRow(context.Background(), "SELECT state::text, attempt FROM river_job WHERE id = $1", id).Scan(&state, &attempt)
	if err != nil {
		r.t.Fatal(err)
	}
	return state, attempt
}

// built is buildWorker over the rig, started, and stopped when the test ends.
func (r *rig) built(redactor *redact.Redactor) *river.Client[pgx.Tx] {
	r.t.Helper()
	tc, err := typst.Locate(r.env.Getenv, r.env.LookPath)
	if err != nil {
		r.t.Fatal(err)
	}
	client, err := buildWorker(r.pool, r.loaded, tc, r.env, slog.New(slog.NewTextHandler(io.Discard, nil)), redactor)
	if err != nil {
		r.t.Fatal(err)
	}
	return client
}

func startClient(t *testing.T, client *river.Client[pgx.Tx]) {
	t.Helper()
	if err := client.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = client.StopAndCancel(ctx)
	})
}

type finished struct {
	code           int
	stdout, stderr string
}

// startWorker runs the worker command in a goroutine against the rig and
// returns its health address, the function that cancels its context, and the
// channel of its result.
func (r *rig) startWorker() (health string, cancel func(), done <-chan finished) {
	r.t.Helper()
	health = freeAddress(r.t)
	ctx, stop := context.WithCancel(context.Background())
	var stdout, stderr syncBuffer
	out := make(chan finished, 1)
	go func() {
		code := Run(ctx, r.env, []string{"worker", "--config", r.config, "--health-listen", health}, &stdout, &stderr)
		out <- finished{code, stdout.String(), stderr.String()}
	}()
	r.t.Cleanup(stop)
	return health, stop, out
}

// waitReady waits until the health route answers 200 and returns its body.
func waitReady(t *testing.T, address string, done <-chan finished) string {
	t.Helper()
	deadline := time.Now().Add(workerWait)
	for time.Now().Before(deadline) {
		select {
		case f := <-done:
			t.Fatalf("the worker ended: %+v", f)
		default:
		}
		resp, err := http.Get("http://" + address + "/health/ready")
		if err == nil {
			body, _ := io.ReadAll(resp.Body)
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return string(body)
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("the worker was not ready in time")
	return ""
}

func waitFinished(t *testing.T, done <-chan finished, within time.Duration) finished {
	t.Helper()
	select {
	case f := <-done:
		return f
	case <-time.After(within):
		t.Fatalf("the worker did not end within %v", within)
		return finished{}
	}
}

func TestWorkerWiresTheJobs(t *testing.T) {
	r := newRig(t, 1, 1)
	client := r.built(redact.New())
	ctx := context.Background()

	// An unregistered kind is refused at insert, so these inserts prove the
	// client holds the three kinds.
	if _, err := client.Insert(ctx, reports.CollectArgs{ReportID: uuid.New()}, reports.CollectInsertOpts(r.loaded)); err != nil {
		t.Fatalf("collect: %v", err)
	}
	if _, err := client.Insert(ctx, reports.RenderArgs{ReportID: uuid.New(), Language: "en"}, reports.RenderInsertOpts(r.loaded)); err != nil {
		t.Fatalf("render: %v", err)
	}
	report := r.request()
	startClient(t, client)

	r.waitReport(report.ID, "collected")

	// The periodic maintenance job runs when the client starts, on its queue.
	deadline := time.Now().Add(workerWait)
	for {
		var n int
		err := r.pool.QueryRow(ctx, "SELECT count(*) FROM river_job WHERE kind = 'maintenance' AND queue = 'maintenance' AND state = 'completed'").Scan(&n)
		if err != nil {
			t.Fatal(err)
		}
		if n > 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("the maintenance job never ran on its queue")
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func TestWorkerWiresTheSlots(t *testing.T) {
	r := newRig(t, 2, 1)
	r.prom.set(requiring(holding()))
	client := r.built(redact.New())
	startClient(t, client)

	var reportsQueued []store.Report
	for range 3 {
		reportsQueued = append(reportsQueued, r.request())
	}
	deadline := time.Now().Add(workerWait)
	for {
		running := 0
		for _, rep := range reportsQueued {
			if state, _ := r.job(rep.CollectJobID); state == "running" {
				running++
			}
		}
		if running == 2 {
			break
		}
		if running > 2 || time.Now().After(deadline) {
			t.Fatalf("%d jobs running, want 2", running)
		}
		time.Sleep(50 * time.Millisecond)
	}
	time.Sleep(500 * time.Millisecond)
	running := 0
	for _, rep := range reportsQueued {
		if state, _ := r.job(rep.CollectJobID); state == "running" {
			running++
		}
	}
	if running != 2 {
		t.Fatalf("%d jobs running after a pause, want 2", running)
	}
}

func TestWorkerStopReturnsTheJob(t *testing.T) {
	r := newRig(t, 1, 1)
	r.prom.set(requiring(holding()))
	_, cancel, done := r.startWorker()
	report := r.request()

	deadline := time.Now().Add(workerWait)
	for r.prom.seen.Load() == 0 {
		if time.Now().After(deadline) {
			t.Fatal("the collection never reached the source")
		}
		time.Sleep(20 * time.Millisecond)
	}
	cancel()
	f := waitFinished(t, done, 3*time.Second)
	if f.code != 0 || f.stdout != "" {
		t.Fatalf("code = %d, stdout = %q, stderr = %q", f.code, f.stdout, f.stderr)
	}
	for _, want := range []string{"worker: started with 1 slots", "worker: stopping", "worker: stopped"} {
		if strings.Count(f.stderr, want) != 1 {
			t.Errorf("log %q does not hold %q once", f.stderr, want)
		}
	}
	state, attempt := r.job(report.CollectJobID)
	if state != "available" || attempt != 0 {
		t.Fatalf("job is %s with attempt %d, want available with 0", state, attempt)
	}
	if got := r.report(report.ID); got.Status != "running" {
		t.Fatalf("report is %s, want running", got.Status)
	}

	r.prom.set(requiring(recorded(t)))
	_, cancel, done = r.startWorker()
	r.waitReport(report.ID, "collected")
	cancel()
	if f := waitFinished(t, done, 10*time.Second); f.code != 0 {
		t.Fatalf("second run: %+v", f)
	}
}

func TestWorkerHealthAnswers(t *testing.T) {
	r := newRig(t, 1, 1)
	address, cancel, done := r.startWorker()
	body := waitReady(t, address, done)
	var decoded struct {
		Status string
		Checks map[string]bool
	}
	if err := json.Unmarshal([]byte(body), &decoded); err != nil {
		t.Fatal(err)
	}
	want := map[string]bool{"database": true, "migrations": true, "worker": true}
	if decoded.Status != "ready" || len(decoded.Checks) != 3 || !decoded.Checks["database"] || !decoded.Checks["migrations"] || !decoded.Checks["worker"] {
		t.Fatalf("body %s, want checks %v", body, want)
	}
	resp, err := http.Get("http://" + address + "/health/live")
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("live = %d", resp.StatusCode)
	}
	cancel()
	if f := waitFinished(t, done, 10*time.Second); f.code != 0 {
		t.Fatalf("%+v", f)
	}
}

func TestWorkerCannotListen(t *testing.T) {
	r := newRig(t, 1, 1)
	held, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = held.Close() }()
	taken := held.Addr().String()
	code, stdout, stderr := runCommand(context.Background(), r.env, "worker", "--config", r.config, "--health-listen", taken)
	if code != 1 || stdout != "" || !strings.HasPrefix(stderr, "error: cannot listen on "+taken+": ") {
		t.Fatalf("code = %d, stdout = %q, stderr = %q", code, stdout, stderr)
	}
}

func TestWorkerRevealsACredential(t *testing.T) {
	r := newRig(t, 1, 1)
	redactor := redact.New()
	if redactor.Text(credentialToken) != credentialToken {
		t.Fatal("the redactor knew the credential before the collection")
	}
	client := r.built(redactor)
	startClient(t, client)
	report := r.request()
	r.waitReport(report.ID, "collected")
	if redactor.Text(credentialToken) == credentialToken {
		t.Fatal("the credential read by the worker is not in the run's redactor")
	}
}

func TestWorkerPoolHoldsTheSlots(t *testing.T) {
	r := newRig(t, 1, 1)
	for _, tc := range []struct {
		dsnMax  string
		atLeast int32
		want    int32
	}{
		{"", 24, 24},
		{"&pool_max_conns=40", 24, 40},
		{"&pool_max_conns=5", 0, 5},
	} {
		loaded := r.loaded
		loaded.DatabaseDSN += tc.dsnMax
		pool, err := openPool(context.Background(), loaded, tc.atLeast)
		if err != nil {
			t.Fatal(err)
		}
		if got := pool.Config().MaxConns; got != tc.want {
			t.Errorf("dsn %q, at least %d: MaxConns = %d, want %d", tc.dsnMax, tc.atLeast, got, tc.want)
		}
		pool.Close()
	}
}
