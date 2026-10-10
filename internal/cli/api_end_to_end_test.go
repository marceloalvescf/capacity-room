//go:build postgres && template

package cli

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/riverqueue/river"

	"github.com/marceloalvescf/k8s-capacity-report/internal/catalog"
	"github.com/marceloalvescf/k8s-capacity-report/internal/collect"
	"github.com/marceloalvescf/k8s-capacity-report/internal/document"
	"github.com/marceloalvescf/k8s-capacity-report/internal/pipeline"
	"github.com/marceloalvescf/k8s-capacity-report/internal/platform/netpolicy"
	"github.com/marceloalvescf/k8s-capacity-report/internal/platform/profiles"
	"github.com/marceloalvescf/k8s-capacity-report/internal/platform/reports"
	"github.com/marceloalvescf/k8s-capacity-report/internal/platform/settings"
	"github.com/marceloalvescf/k8s-capacity-report/internal/platform/store"
	"github.com/marceloalvescf/k8s-capacity-report/internal/platform/storetest"
	"github.com/marceloalvescf/k8s-capacity-report/internal/platform/worker"
	"github.com/marceloalvescf/k8s-capacity-report/internal/redact"
	"github.com/marceloalvescf/k8s-capacity-report/internal/render"
	"github.com/marceloalvescf/k8s-capacity-report/internal/snapshot"
	"github.com/marceloalvescf/k8s-capacity-report/internal/typst"
)

// download is one answer of the API with its raw body.
type download struct {
	status int
	header http.Header
	body   []byte
}

// fetch sends one request with the reader's cookies and returns the raw
// answer.
func (r *reader) fetch(method, path, csrf string, extra map[string]string) download {
	r.t.Helper()
	req, err := http.NewRequest(method, r.base+path, nil)
	if err != nil {
		r.t.Fatal(err)
	}
	if csrf != "" {
		req.Header.Set("X-CSRF-Token", csrf)
	}
	for name, value := range extra {
		req.Header.Set(name, value)
	}
	for name, value := range r.cookies {
		req.AddCookie(&http.Cookie{Name: name, Value: value})
	}
	resp, err := apiClient.Do(req)
	if err != nil {
		r.t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		r.t.Fatal(err)
	}
	return download{status: resp.StatusCode, header: resp.Header, body: body}
}

func digest(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func TestReportThroughTheAPI(t *testing.T) {
	ctx := context.Background()
	fixed := time.Date(2026, 10, 6, 12, 0, 0, 0, time.UTC)
	clock := func() time.Time { return fixed }
	pool := storetest.New(t)

	dir := t.TempDir()
	for _, sub := range []string{"credentials", "ca"} {
		if err := os.Mkdir(filepath.Join(dir, sub), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	secret := filepath.Join(dir, "secret")
	if err := os.WriteFile(secret, []byte(strings.Repeat("s", 32)), 0o600); err != nil {
		t.Fatal(err)
	}
	config := filepath.Join(dir, "platform.toml")
	toml := fmt.Sprintf("server_secret_path = %q\ncredential_dir = %q\nca_bundle_dir = %q\n"+
		"allowed_endpoint_cidrs = [\"127.0.0.1/32\"]\nworker_slots = 2\nworker_drain_seconds = 1\n",
		secret, filepath.Join(dir, "credentials"), filepath.Join(dir, "ca"))
	if err := os.WriteFile(config, []byte(toml), 0o600); err != nil {
		t.Fatal(err)
	}

	// The fake Prometheus serves a scenario once; the swap lets the reference
	// run take a fresh one.
	prom := &swap{}
	prom.set(recorded(t))
	source := httptest.NewServer(prom)
	t.Cleanup(source.Close)

	// The administrator is bootstrapped and the profile imported by the
	// commands of the program.
	env := platformEnv(map[string]string{
		"KCR_BOOTSTRAP_MODE": "1",
		"KCR_DATABASE_DSN":   pool.Config().ConnString(),
	}, "admin password one\n")
	env.Now = clock
	env.TempDir = t.TempDir()
	env.Version = "0.0.0-test"
	if code, out, errOut := runCommand(ctx, env, "admin", "bootstrap-admin", "--username", "admin", "--password-stdin", "--config", config); code != 0 {
		t.Fatalf("bootstrap: code %d, stdout %q, stderr %q", code, out, errOut)
	}
	profileFile := filepath.Join(dir, "profiles.toml")
	profileText := fmt.Sprintf("[[profiles]]\nid = \"prod\"\ndisplay_name = \"Production\"\nendpoint_url = %q\n"+
		"endpoint_type = \"prometheus\"\ncluster_name = \"prod\"\nallowed_namespaces = [\"payments\"]\nactive = true\n",
		"http://"+source.Listener.Addr().String()+"/api/v1")
	if err := os.WriteFile(profileFile, []byte(profileText), 0o600); err != nil {
		t.Fatal(err)
	}
	if code, out, errOut := runCommand(ctx, env, "admin", "import-profiles", "--config", config, "--file", profileFile); code != 0 {
		t.Fatalf("import: code %d, stdout %q, stderr %q", code, out, errOut)
	}

	// The service and the worker are what the commands build.
	env.Getenv = envFunc(map[string]string{"KCR_DATABASE_DSN": pool.Config().ConnString()})
	loaded, err := settings.Load(config, env.Getenv)
	if err != nil {
		t.Fatal(err)
	}
	serverSecret, err := loaded.ReadServerSecret()
	if err != nil {
		t.Fatal(err)
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	server, err := buildServer(pool, openDownloads(t, pool), loaded, serverSecret, clock, logger, redact.New())
	if err != nil {
		t.Fatal(err)
	}
	front := httptest.NewServer(server.Handler())
	t.Cleanup(front.Close)

	tc, err := typst.Locate(os.Getenv, exec.LookPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := tc.Probe(ctx); err != nil {
		t.Fatal(err)
	}
	client, err := buildWorker(pool, loaded, tc, env, logger, redact.New())
	if err != nil {
		t.Fatal(err)
	}
	if err := client.Start(ctx); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		stop, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = client.StopAndCancel(stop)
	})

	// Both users sign in through the API.
	admin := &reader{t: t, base: front.URL, cookies: map[string]string{}}
	adminCSRF := admin.signIn("admin", "admin password one")
	admin.mustStatus(201, "POST", "/api/v1/admin/users", adminCSRF, map[string]string{
		"username": "olivia", "password": "olivia password one", "role": "operator", "language": "en",
	})
	operator := &reader{t: t, base: front.URL, cookies: map[string]string{}}
	csrf := operator.signIn("olivia", "olivia password one")

	// The report is requested and polled until its PDFs are done.
	operator.header = map[string]string{"Idempotency-Key": uuid.NewString()}
	accepted := operator.mustStatus(202, "POST", "/api/v1/reports", csrf,
		map[string]any{"profile_id": "prod", "namespaces": []string{"payments"}, "language": "en"})
	operator.header = nil
	idText, _ := accepted["id"].(string)
	id, err := uuid.Parse(idText)
	if err != nil || accepted["replayed"] != false {
		t.Fatalf("accepted %v", accepted)
	}
	path := "/api/v1/reports/" + idText
	deadline := time.Now().Add(60 * time.Second)
	for {
		detail := operator.mustStatus(200, "GET", path, "", nil)
		if detail["status"] == "failed" || time.Now().After(deadline) {
			t.Fatalf("report did not complete: %v", detail)
		}
		if detail["status"] == "collected" && detail["phase"] == "complete" {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	listed := operator.mustStatus(200, "GET", "/api/v1/reports", "", nil)
	items, _ := listed["items"].([]any)
	if len(items) != 1 || items[0].(map[string]any)["id"] != idText {
		t.Fatalf("list %v", listed)
	}

	// The reference: the same run on a fresh server of the scenario.
	prom.set(recorded(t))
	stored, err := store.GetReport(ctx, pool, id)
	if err != nil {
		t.Fatal(err)
	}
	service := &profiles.Service{
		Pool:     pool,
		Policy:   netpolicy.Policy{Hostnames: loaded.AllowedEndpointHostnames, Networks: loaded.AllowedEndpointCIDRs},
		Settings: loaded,
		Now:      clock,
	}
	rev, err := service.Revision(ctx, stored.ProfileRevisionID)
	if err != nil {
		t.Fatal(err)
	}
	var resolved reports.Resolved
	if err := json.Unmarshal(stored.Request, &resolved); err != nil {
		t.Fatal(err)
	}
	cfg := rev.Document.RunConfig(resolved.Period, resolved.Start, resolved.End, resolved.Timezone, resolved.ReportProfile, resolved.MinSeverity, resolved.Namespaces)
	opts, err := service.Source(rev.Document, cfg.Metrics)
	if err != nil {
		t.Fatal(err)
	}
	window, err := collect.NewWindow(cfg.Analysis, fixed)
	if err != nil {
		t.Fatal(err)
	}
	want, err := pipeline.Collect(ctx, pipeline.Run{
		Options: opts, Config: cfg, Window: window, EmittedAt: fixed, Now: clock,
		NewID:   func() (string, error) { return idText, nil },
		Version: env.Version,
	})
	if err != nil {
		t.Fatal(err)
	}
	wantSnapshot, err := snapshot.Encode(want.Snapshot)
	if err != nil {
		t.Fatal(err)
	}

	// Each download equals what the core writes, and answers 304 to its tag.
	type expected struct {
		path        string
		body        []byte
		contentType string
		disposition string
		tag         string
	}
	checks := []expected{{
		path: path + "/snapshot", body: wantSnapshot, contentType: "application/json",
		disposition: `attachment; filename="report-` + idText + `.json"`, tag: digest(wantSnapshot),
	}}
	for _, lang := range []catalog.Language{catalog.PortugueseBrazil, catalog.English} {
		cat, err := catalog.Load(lang)
		if err != nil {
			t.Fatal(err)
		}
		view, err := document.BuildView(want.Snapshot, cat)
		if err != nil {
			t.Fatal(err)
		}
		wantView, err := document.EncodeView(view)
		if err != nil {
			t.Fatal(err)
		}
		checks = append(checks,
			expected{path: path + "/view?lang=" + string(lang), body: wantView, contentType: "application/json", tag: digest(wantView)})
		if lang != catalog.English {
			continue
		}
		data, err := render.Document(want.Snapshot, lang, document.Options{})
		if err != nil {
			t.Fatal(err)
		}
		wantPDF, err := render.Compile(ctx, tc, t.TempDir(), data)
		if err != nil {
			t.Fatal(err)
		}
		checks = append(checks,
			expected{
				path: path + "/pdf?lang=" + string(lang), body: wantPDF, contentType: "application/pdf",
				disposition: `attachment; filename="report-` + idText + `-` + string(lang) + `.pdf"`, tag: digest(wantPDF),
			})
	}
	for _, c := range checks {
		got := operator.fetch("GET", c.path, "", nil)
		if got.status != 200 || !bytes.Equal(got.body, c.body) {
			t.Fatalf("GET %s: status %d, %d bytes, want 200 and %d bytes equal to the core's", c.path, got.status, len(got.body), len(c.body))
		}
		if got.header.Get("Content-Type") != c.contentType || got.header.Get("Content-Disposition") != c.disposition ||
			got.header.Get("ETag") != `"`+c.tag+`"` || got.header.Get("Cache-Control") != "private, no-cache" {
			t.Fatalf("GET %s: headers %v", c.path, got.header)
		}
		again := operator.fetch("GET", c.path, "", map[string]string{"If-None-Match": got.header.Get("ETag")})
		if again.status != http.StatusNotModified || len(again.body) != 0 || again.header.Get("ETag") != got.header.Get("ETag") {
			t.Fatalf("GET %s with its tag: status %d, %d bytes", c.path, again.status, len(again.body))
		}
	}

	// The operator deletes the report; it is gone at once, and one
	// maintenance run sweeps it.
	if got := operator.fetch("DELETE", path, csrf, nil); got.status != http.StatusNoContent {
		t.Fatalf("DELETE: status %d, body %q", got.status, got.body)
	}
	operator.mustStatus(404, "GET", path, "", nil)
	res, err := client.Insert(ctx, worker.MaintenanceArgs{}, &river.InsertOpts{Queue: "maintenance", MaxAttempts: loaded.JobMaxAttempts})
	if err != nil {
		t.Fatal(err)
	}
	count := func(query string, args ...any) (n int) {
		t.Helper()
		if err := pool.QueryRow(ctx, query, args...).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}
	deadline = time.Now().Add(30 * time.Second)
	for count(`SELECT count(*) FROM river_job WHERE id = $1 AND state = 'completed'`, res.Job.ID) == 0 {
		if time.Now().After(deadline) {
			t.Fatal("the maintenance job did not complete")
		}
		time.Sleep(50 * time.Millisecond)
	}
	for table, column := range map[string]string{
		"reports": "id", "report_snapshots": "report_id", "report_pdfs": "report_id", "report_idempotency_keys": "report_id",
	} {
		if n := count(`SELECT count(*) FROM `+table+` WHERE `+column+` = $1`, id); n != 0 {
			t.Fatalf("%d rows of %s after the sweep", n, table)
		}
	}

	events, err := store.ListAuditEvents(ctx, pool)
	if err != nil {
		t.Fatal(err)
	}
	var actions []string
	for _, ev := range events {
		if ev.TargetType == nil || *ev.TargetType != "report" || ev.TargetID == nil || *ev.TargetID != idText {
			continue
		}
		actions = append(actions, ev.Action)
		if ev.Action != reports.ActionReportPurge {
			continue
		}
		var meta map[string]any
		if err := json.Unmarshal(ev.Metadata, &meta); err != nil {
			t.Fatal(err)
		}
		if len(meta) != 1 || meta["cause"] != "deleted" {
			t.Fatalf("purge audit metadata %v", meta)
		}
	}
	wantActions := []string{
		reports.ActionReportSubmit, reports.ActionReportCollect,
		reports.ActionReportRender,
		reports.ActionReportDelete, reports.ActionReportPurge,
	}
	if !slices.Equal(actions, wantActions) {
		t.Fatalf("audit actions %v, want %v", actions, wantActions)
	}
}
