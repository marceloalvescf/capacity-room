package cli

import (
	"bytes"
	"context"
	"fmt"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/marceloalvescf/k8s-capacity-report/internal/catalog"
	"github.com/marceloalvescf/k8s-capacity-report/internal/goldentest"
	"github.com/marceloalvescf/k8s-capacity-report/internal/promtest"
	"github.com/marceloalvescf/k8s-capacity-report/internal/snapshot"
)

// reportFlagNames lists every flag report.go declares; TestReportHelpListsEveryFlag
// checks each one appears in the command's own help text.
var reportFlagNames = []string{
	"config", "metrics-url", "endpoint-type", "token", "insecure",
	"cluster", "cluster-label", "cluster-filter", "namespaces", "period", "start", "end",
	"timezone", "node-autoscaling", "min-severity", "report-profile", "lang",
	"output", "snapshot", "from-snapshot", "check", "verbose",
}

func TestReportHelpListsEveryFlag(t *testing.T) {
	stdout, stderr, code := runArgs(context.Background(), testEnv(t), []string{"report", "--help"})
	if code != 0 {
		t.Fatalf("code = %d, want 0; stderr = %q", code, stderr)
	}
	for _, name := range reportFlagNames {
		if !strings.Contains(stdout, "--"+name) {
			t.Errorf("help text does not mention --%s", name)
		}
	}
}

// directCompleteCapabilityLines are the nine capability lines
// testdata/snapshot/direct-complete.json holds, in their fixed order: every
// capability is available.
var directCompleteCapabilityLines = []string{
	"namespace_usage: available",
	"workload_usage: available",
	"requests_limits: available",
	"hpa_history: available",
	"workloads_without_hpa: available",
	"ip_node_mapping: available",
	"nodepool_discovery: available",
	"node_utilization: available",
	"node_autoscaling: available",
}

func directCompletePreflightLines() []string {
	lines := []string{"config: loaded", "endpoint: available", "scope: resolved", "capability: discovered"}
	return append(lines, directCompleteCapabilityLines...)
}

func TestCheckPrintsThePreflight(t *testing.T) {
	server, _, scenario := startScenario(t, "direct-complete")
	defer server.Close()

	cfgPath := writeConfig(t, scenarioConfig(server, scenario, t.TempDir()))
	env := testEnv(t)

	stdout, stderr, code := runArgs(context.Background(), env, []string{"report", "--config", cfgPath, "--check"})
	if code != 0 {
		t.Fatalf("code = %d, want 0; stderr = %q", code, stderr)
	}
	if stderr != "" {
		t.Fatalf("stderr = %q, want empty", stderr)
	}

	want := strings.Join(directCompletePreflightLines(), "\n") + "\n"
	if stdout != want {
		t.Fatalf("stdout =\n%s\nwant\n%s", stdout, want)
	}
}

func TestCheckMakesNoRangeRequest(t *testing.T) {
	server, handler, scenario := startScenario(t, "direct-complete")
	defer server.Close()

	cfgPath := writeConfig(t, scenarioConfig(server, scenario, t.TempDir()))
	_, _, code := runArgs(context.Background(), testEnv(t), []string{"report", "--config", cfgPath, "--check"})
	if code != 0 {
		t.Fatalf("code = %d, want 0", code)
	}

	for _, seen := range handler.Seen() {
		if strings.Contains(seen.Path, "query_range") {
			t.Fatalf("check mode made a request to %s", seen.Path)
		}
	}
}

func TestCheckVerbose(t *testing.T) {
	server, _, scenario := startScenario(t, "direct-complete")
	defer server.Close()

	cfgPath := writeConfig(t, scenarioConfig(server, scenario, t.TempDir()))
	stdout, stderr, code := runArgs(context.Background(), testEnv(t), []string{"report", "--config", cfgPath, "--check", "--verbose"})
	if code != 0 {
		t.Fatalf("code = %d, want 0; stderr = %q", code, stderr)
	}

	want := strings.Join(directCompletePreflightLines(), "\n") + "\n" +
		"config path: " + cfgPath + "\n" +
		"endpoint path: /api/v1\n" +
		"retries: 2 (3 attempts)\n" +
		"strategies: raw_metrics, recording_rule\n"
	if stdout != want {
		t.Fatalf("stdout =\n%s\nwant\n%s", stdout, want)
	}
}

func TestCheckNeedsNoTypst(t *testing.T) {
	server, _, scenario := startScenario(t, "direct-complete")
	defer server.Close()

	cfgPath := writeConfig(t, scenarioConfig(server, scenario, t.TempDir()))
	env := testEnv(t) // LookPath always fails, no KCR_TYPST set
	_, stderr, code := runArgs(context.Background(), env, []string{"report", "--config", cfgPath, "--check"})
	if code != 0 {
		t.Fatalf("code = %d, want 0; stderr = %q", code, stderr)
	}
}

// fixedGoldenEnv reproduces the fixed input internal/goldentest.Snapshot
// runs every scenario with: the same clock (14:30:00 then 14:30:42
// America/Sao_Paulo), the same report identifier, and the same generator
// version.
func fixedGoldenEnv(t *testing.T) Env {
	t.Helper()
	loc, err := time.LoadLocation("America/Sao_Paulo")
	if err != nil {
		t.Fatalf("loading America/Sao_Paulo: %v", err)
	}
	base := time.Date(2026, time.July, 15, 14, 30, 0, 0, loc)
	return Env{
		Getenv:   envFunc(nil),
		LookPath: noTypstLookPath,
		Now:      sequentialClock(base, base.Add(42*time.Second)),
		NewID:    fixedID("00000000-0000-4000-8000-000000000001"),
		TempDir:  t.TempDir(),
		Version:  "0.0.0-golden",
	}
}

// fixedGoldenConfig builds the TOML configuration matching the fixed input
// of internal/goldentest.fixedConfig, pointed at server.
func fixedGoldenConfig(server *httptest.Server, scenario promtest.Scenario, outputDir string) string {
	return fmt.Sprintf(`[metrics]
url = %q
endpoint_type = %q

[cluster]
name = "prod"
namespaces = ["payments"]

[analysis]
node_autoscaling = %q
excluded_containers = ["istio-proxy"]

[report]
organization = "ACME"
classification = "Uso interno"
author = "Time SRE"
output_directory = %q
`, server.URL+scenario.APIRoot, scenario.EndpointType, scenario.NodeAutoscaling, outputDir)
}

func scenarioArgs(cfgPath string, scenario promtest.Scenario, extra ...string) []string {
	args := []string{"report", "--config", cfgPath}
	if scenario.Token != "" {
		args = append(args, "--token", scenario.Token)
	}
	return append(args, extra...)
}

func TestSnapshotModeWritesTheGoldenFile(t *testing.T) {
	for _, scenario := range promtest.Scenarios {
		t.Run(scenario.Name, func(t *testing.T) {
			server, _, scenario := startScenario(t, scenario.Name)
			defer server.Close()

			outDir := t.TempDir()
			cfgPath := writeConfig(t, fixedGoldenConfig(server, scenario, outDir))
			snapPath := filepath.Join(t.TempDir(), "s.json")

			args := scenarioArgs(cfgPath, scenario, "--snapshot", snapPath)
			_, stderr, code := runArgs(context.Background(), fixedGoldenEnv(t), args)
			if code != 0 {
				t.Fatalf("code = %d, want 0; stderr = %q", code, stderr)
			}

			got, err := os.ReadFile(snapPath)
			if err != nil {
				t.Fatalf("reading %s: %v", snapPath, err)
			}
			wantPath, err := goldentest.SnapshotPath(scenario.Name)
			if err != nil {
				t.Fatalf("SnapshotPath: %v", err)
			}
			want, err := os.ReadFile(wantPath)
			if err != nil {
				t.Fatalf("reading %s: %v", wantPath, err)
			}
			if !bytes.Equal(got, want) {
				t.Fatalf("snapshot for %s does not match %s", scenario.Name, wantPath)
			}
		})
	}
}

func TestSnapshotModeNeedsNoTypst(t *testing.T) {
	server, _, scenario := startScenario(t, "direct-complete")
	defer server.Close()

	cfgPath := writeConfig(t, scenarioConfig(server, scenario, t.TempDir()))
	snapPath := filepath.Join(t.TempDir(), "s.json")
	_, stderr, code := runArgs(context.Background(), testEnv(t), []string{"report", "--config", cfgPath, "--snapshot", snapPath})
	if code != 0 {
		t.Fatalf("code = %d, want 0; stderr = %q", code, stderr)
	}
}

// pdfBody strips the fixed "%PDF-fake\n" marker the fake compiler writes and
// returns what follows: the exact bytes of report.json.
func pdfBody(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	const marker = "%PDF-fake\n"
	if !bytes.HasPrefix(data, []byte(marker)) {
		t.Fatalf("%s does not start with %q", path, marker)
	}
	return data[len(marker):]
}

func TestPDFModeWritesTheFile(t *testing.T) {
	server, _, scenario := startScenario(t, "direct-complete")
	defer server.Close()

	outDir := t.TempDir()
	cfgPath := writeConfig(t, fixedGoldenConfig(server, scenario, outDir))
	outPath := filepath.Join(t.TempDir(), "r.pdf")

	env := withTypst(t, fixedGoldenEnv(t))
	args := scenarioArgs(cfgPath, scenario, "--lang", "en", "--output", outPath)
	_, stderr, code := runArgs(context.Background(), env, args)
	if code != 0 {
		t.Fatalf("code = %d, want 0; stderr = %q", code, stderr)
	}

	got := pdfBody(t, outPath)
	wantPath, err := goldentest.DataFilePath("direct-complete", catalog.English)
	if err != nil {
		t.Fatalf("DataFilePath: %v", err)
	}
	want, err := os.ReadFile(wantPath)
	if err != nil {
		t.Fatalf("reading %s: %v", wantPath, err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("PDF body does not match %s", wantPath)
	}
}

func TestPDFModeDefaultsToPortuguese(t *testing.T) {
	server, _, scenario := startScenario(t, "direct-complete")
	defer server.Close()

	outDir := t.TempDir()
	cfgPath := writeConfig(t, fixedGoldenConfig(server, scenario, outDir))
	outPath := filepath.Join(t.TempDir(), "r.pdf")

	env := withTypst(t, fixedGoldenEnv(t))
	args := scenarioArgs(cfgPath, scenario, "--output", outPath)
	_, stderr, code := runArgs(context.Background(), env, args)
	if code != 0 {
		t.Fatalf("code = %d, want 0; stderr = %q", code, stderr)
	}

	got := pdfBody(t, outPath)
	wantPath, err := goldentest.DataFilePath("direct-complete", catalog.PortugueseBrazil)
	if err != nil {
		t.Fatalf("DataFilePath: %v", err)
	}
	want, err := os.ReadFile(wantPath)
	if err != nil {
		t.Fatalf("reading %s: %v", wantPath, err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("PDF body does not match %s", wantPath)
	}
}

// TestPDFModeDefaultName confirms the command wires the default output name
// end to end; the naming algorithm itself (slug, suffix, language, time) is
// covered exhaustively, without any network dependency, by
// TestDefaultOutputName in output_test.go.
func TestPDFModeDefaultName(t *testing.T) {
	server, _, scenario := startScenario(t, "direct-complete")
	defer server.Close()

	outDir := t.TempDir()
	cfgPath := writeConfig(t, fixedGoldenConfig(server, scenario, outDir))

	env := withTypst(t, fixedGoldenEnv(t))
	args := scenarioArgs(cfgPath, scenario, "--lang", "en")
	stdout, stderr, code := runArgs(context.Background(), env, args)
	if code != 0 {
		t.Fatalf("code = %d, want 0; stderr = %q", code, stderr)
	}

	want := "report: " + filepath.Join(outDir, "report-resources-prod-en-20260715-143000.pdf")
	if !strings.Contains(stdout, want) {
		t.Fatalf("stdout = %q, want it to contain %q", stdout, want)
	}
}

func TestFromSnapshotWritesTheFile(t *testing.T) {
	for _, name := range goldentest.DataFileScenarios {
		for _, lang := range catalog.Languages() {
			t.Run(name+"/"+string(lang), func(t *testing.T) {
				snapPath, err := goldentest.SnapshotPath(name)
				if err != nil {
					t.Fatalf("SnapshotPath: %v", err)
				}
				outPath := filepath.Join(t.TempDir(), "r.pdf")

				env := withTypst(t, testEnv(t))
				args := []string{"report", "--from-snapshot", snapPath, "--lang", string(lang), "--output", outPath}
				_, stderr, code := runArgs(context.Background(), env, args)
				if code != 0 {
					t.Fatalf("code = %d, want 0; stderr = %q", code, stderr)
				}

				got := pdfBody(t, outPath)
				wantPath, err := goldentest.DataFilePath(name, lang)
				if err != nil {
					t.Fatalf("DataFilePath: %v", err)
				}
				want, err := os.ReadFile(wantPath)
				if err != nil {
					t.Fatalf("reading %s: %v", wantPath, err)
				}
				if !bytes.Equal(got, want) {
					t.Fatalf("PDF body for %s/%s does not match %s", name, lang, wantPath)
				}
			})
		}
	}
}

func TestFromSnapshotMakesNoRequest(t *testing.T) {
	server, handler, _ := startScenario(t, "direct-complete")
	defer server.Close()

	snapPath, err := goldentest.SnapshotPath("direct-complete")
	if err != nil {
		t.Fatalf("SnapshotPath: %v", err)
	}
	outPath := filepath.Join(t.TempDir(), "r.pdf")

	env := withTypst(t, testEnv(t))
	_, stderr, code := runArgs(context.Background(), env, []string{"report", "--from-snapshot", snapPath, "--output", outPath})
	if code != 0 {
		t.Fatalf("code = %d, want 0; stderr = %q", code, stderr)
	}
	if len(handler.Seen()) != 0 {
		t.Fatalf("from-snapshot mode made %d requests, want 0", len(handler.Seen()))
	}
}

func TestFromSnapshotOverridesSeverity(t *testing.T) {
	snapPath, err := goldentest.SnapshotPath("partial-data")
	if err != nil {
		t.Fatalf("SnapshotPath: %v", err)
	}
	outPath := filepath.Join(t.TempDir(), "r.pdf")

	env := withTypst(t, testEnv(t))
	args := []string{"report", "--from-snapshot", snapPath, "--min-severity", "critical", "--output", outPath}
	_, stderr, code := runArgs(context.Background(), env, args)
	if code != 0 {
		t.Fatalf("code = %d, want 0; stderr = %q", code, stderr)
	}

	got := pdfBody(t, outPath)
	rows := bytes.Count(got, []byte(`"type": "table"`))
	if rows == 0 {
		t.Fatalf("expected at least one table in the overridden report")
	}
}

func TestSnapshotThenFromSnapshotEqualsPDFMode(t *testing.T) {
	// Direct PDF mode.
	pdfServer, _, scenario := startScenario(t, "direct-complete")
	defer pdfServer.Close()
	pdfOutDir := t.TempDir()
	pdfCfg := writeConfig(t, fixedGoldenConfig(pdfServer, scenario, pdfOutDir))
	pdfOut := filepath.Join(t.TempDir(), "direct.pdf")

	env1 := withTypst(t, fixedGoldenEnv(t))
	_, stderr, code := runArgs(context.Background(), env1, scenarioArgs(pdfCfg, scenario, "--output", pdfOut))
	if code != 0 {
		t.Fatalf("PDF mode: code = %d, want 0; stderr = %q", code, stderr)
	}

	// Snapshot, then render from it.
	snapServer, _, scenario2 := startScenario(t, "direct-complete")
	defer snapServer.Close()
	snapOutDir := t.TempDir()
	snapCfg := writeConfig(t, fixedGoldenConfig(snapServer, scenario2, snapOutDir))
	snapPath := filepath.Join(t.TempDir(), "s.json")

	env2 := fixedGoldenEnv(t)
	_, stderr, code = runArgs(context.Background(), env2, scenarioArgs(snapCfg, scenario2, "--snapshot", snapPath))
	if code != 0 {
		t.Fatalf("snapshot mode: code = %d, want 0; stderr = %q", code, stderr)
	}

	fromOut := filepath.Join(t.TempDir(), "from.pdf")
	env3 := withTypst(t, testEnv(t))
	_, stderr, code = runArgs(context.Background(), env3, []string{"report", "--from-snapshot", snapPath, "--output", fromOut})
	if code != 0 {
		t.Fatalf("from-snapshot mode: code = %d, want 0; stderr = %q", code, stderr)
	}

	a := pdfBody(t, pdfOut)
	b := pdfBody(t, fromOut)
	if !bytes.Equal(a, b) {
		t.Fatalf("PDF mode and snapshot-then-from-snapshot produced different documents")
	}
}

// TestOutputFilesAreReadableByOthers checks the mode of the file each mode
// writes: 0o644 less the umask of this test process, for the PDF (chmoded
// explicitly, since the fake compiler — like the real one, measured
// separately; see the command's report — leaves it at a mode derived from
// 0o666, not 0o644), the snapshot, and the PDF built from a snapshot.
func TestOutputFilesAreReadableByOthers(t *testing.T) {
	want := expected0644Mode(t)

	t.Run("pdf mode", func(t *testing.T) {
		server, _, scenario := startScenario(t, "direct-complete")
		defer server.Close()
		cfgPath := writeConfig(t, scenarioConfig(server, scenario, t.TempDir()))
		outPath := filepath.Join(t.TempDir(), "r.pdf")

		env := withTypst(t, testEnv(t))
		_, stderr, code := runArgs(context.Background(), env, []string{"report", "--config", cfgPath, "--output", outPath})
		if code != 0 {
			t.Fatalf("code = %d, want 0; stderr = %q", code, stderr)
		}
		info, err := os.Stat(outPath)
		if err != nil {
			t.Fatalf("stat: %v", err)
		}
		if got := info.Mode().Perm(); got != want {
			t.Fatalf("mode = %v, want %v", got, want)
		}
	})

	t.Run("snapshot mode", func(t *testing.T) {
		server, _, scenario := startScenario(t, "direct-complete")
		defer server.Close()
		cfgPath := writeConfig(t, scenarioConfig(server, scenario, t.TempDir()))
		snapPath := filepath.Join(t.TempDir(), "s.json")

		_, stderr, code := runArgs(context.Background(), testEnv(t), []string{"report", "--config", cfgPath, "--snapshot", snapPath})
		if code != 0 {
			t.Fatalf("code = %d, want 0; stderr = %q", code, stderr)
		}
		info, err := os.Stat(snapPath)
		if err != nil {
			t.Fatalf("stat: %v", err)
		}
		if got := info.Mode().Perm(); got != want {
			t.Fatalf("mode = %v, want %v", got, want)
		}
	})

	t.Run("mode from snapshot", func(t *testing.T) {
		snapPath, err := goldentest.SnapshotPath("direct-complete")
		if err != nil {
			t.Fatalf("SnapshotPath: %v", err)
		}
		outPath := filepath.Join(t.TempDir(), "r.pdf")

		env := withTypst(t, testEnv(t))
		_, stderr, code := runArgs(context.Background(), env, []string{"report", "--from-snapshot", snapPath, "--output", outPath})
		if code != 0 {
			t.Fatalf("code = %d, want 0; stderr = %q", code, stderr)
		}
		info, err := os.Stat(outPath)
		if err != nil {
			t.Fatalf("stat: %v", err)
		}
		if got := info.Mode().Perm(); got != want {
			t.Fatalf("mode = %v, want %v", got, want)
		}
	})
}

// TestOutputFilesShareOneMode runs the PDF mode (with the fake compiler)
// and the snapshot mode in the same test and checks the two files it writes
// end up at the same mode: whatever the process umask is, right now, both
// paths are meant to give "0o644 less that umask", never one file more
// open than the other.
func TestOutputFilesShareOneMode(t *testing.T) {
	pdfServer, _, pdfScenario := startScenario(t, "direct-complete")
	defer pdfServer.Close()
	pdfCfg := writeConfig(t, scenarioConfig(pdfServer, pdfScenario, t.TempDir()))
	pdfPath := filepath.Join(t.TempDir(), "r.pdf")

	env := withTypst(t, testEnv(t))
	_, stderr, code := runArgs(context.Background(), env, []string{"report", "--config", pdfCfg, "--output", pdfPath})
	if code != 0 {
		t.Fatalf("PDF mode: code = %d, want 0; stderr = %q", code, stderr)
	}

	snapServer, _, snapScenario := startScenario(t, "direct-complete")
	defer snapServer.Close()
	snapCfg := writeConfig(t, scenarioConfig(snapServer, snapScenario, t.TempDir()))
	snapPath := filepath.Join(t.TempDir(), "s.json")

	_, stderr, code = runArgs(context.Background(), testEnv(t), []string{"report", "--config", snapCfg, "--snapshot", snapPath})
	if code != 0 {
		t.Fatalf("snapshot mode: code = %d, want 0; stderr = %q", code, stderr)
	}

	pdfInfo, err := os.Stat(pdfPath)
	if err != nil {
		t.Fatalf("stat %s: %v", pdfPath, err)
	}
	snapInfo, err := os.Stat(snapPath)
	if err != nil {
		t.Fatalf("stat %s: %v", snapPath, err)
	}
	if pdfInfo.Mode().Perm() != snapInfo.Mode().Perm() {
		t.Fatalf("PDF mode = %v, snapshot mode = %v, want equal", pdfInfo.Mode().Perm(), snapInfo.Mode().Perm())
	}
}

// modifiedSnapshot decodes the golden snapshot of name, applies modify to
// it, encodes it again, and writes it to a fresh file, returning its path.
func modifiedSnapshot(t *testing.T, name string, modify func(*snapshot.Snapshot)) string {
	t.Helper()
	goldenPath, err := goldentest.SnapshotPath(name)
	if err != nil {
		t.Fatalf("SnapshotPath: %v", err)
	}
	raw, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("reading %s: %v", goldenPath, err)
	}
	snap, err := snapshot.Decode(raw)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	modify(&snap)

	data, err := snapshot.Encode(snap)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	path := filepath.Join(t.TempDir(), "s.json")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("writing %s: %v", path, err)
	}
	return path
}

// TestDefaultNameEndToEnd proves the same five default-naming rows as
// TestDefaultOutputName, end to end, through the mode from snapshot, which
// needs no server: each row modifies a golden snapshot's own stored fields
// (or passes a flag) and checks that the command writes a file under the
// exact name defaultOutputName predicts, in KCR_OUTPUT_DIRECTORY.
func TestDefaultNameEndToEnd(t *testing.T) {
	tests := []struct {
		name   string
		modify func(*snapshot.Snapshot)
		lang   string
		want   string
	}{
		{"cluster prod", func(*snapshot.Snapshot) {}, "en", "report-resources-prod-en-20260715-143000.pdf"},
		{"cluster with punctuation", func(s *snapshot.Snapshot) { s.Scope.ClusterName = "Prod West_1" }, "en", "report-resources-prod-west-1-en-20260715-143000.pdf"},
		{"cluster all punctuation", func(s *snapshot.Snapshot) { s.Scope.ClusterName = "---" }, "en", "report-resources-cluster-en-20260715-143000.pdf"},
		{"minimum severity", func(s *snapshot.Snapshot) { s.Report.MinSeverity = "attention" }, "en", "report-resources-prod-attention-en-20260715-143000.pdf"},
		{"language pt-BR", func(*snapshot.Snapshot) {}, "pt-BR", "report-resources-prod-pt-br-20260715-143000.pdf"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			snapPath := modifiedSnapshot(t, "direct-complete", tt.modify)
			outDir := t.TempDir()

			env := withTypst(t, testEnv(t))
			inner := env.Getenv
			env.Getenv = func(k string) string {
				if k == "KCR_OUTPUT_DIRECTORY" {
					return outDir
				}
				return inner(k)
			}

			_, stderr, code := runArgs(context.Background(), env, []string{"report", "--from-snapshot", snapPath, "--lang", tt.lang})
			if code != 0 {
				t.Fatalf("code = %d, want 0; stderr = %q", code, stderr)
			}
			if _, err := os.Stat(filepath.Join(outDir, tt.want)); err != nil {
				t.Fatalf("%s was not written in %s: %v", tt.want, outDir, err)
			}
		})
	}
}

// clockCounter returns a func() time.Time that counts its own calls and
// records the count in calls.
func clockCounter(calls *int) func() time.Time {
	base := time.Date(2026, time.July, 15, 14, 30, 0, 0, time.UTC)
	return func() time.Time {
		*calls++
		return base
	}
}

// TestCheckReadsTheClockOnce proves a check reads the clock once, before the
// preflight, and never when a collection would finish.
func TestCheckReadsTheClockOnce(t *testing.T) {
	server, _, scenario := startScenario(t, "direct-complete")
	defer server.Close()
	cfgPath := writeConfig(t, scenarioConfig(server, scenario, t.TempDir()))

	env := testEnv(t)
	var calls int
	env.Now = clockCounter(&calls)

	_, stderr, code := runArgs(context.Background(), env, []string{"report", "--config", cfgPath, "--check"})
	if code != 0 {
		t.Fatalf("code = %d, want 0; stderr = %q", code, stderr)
	}
	if calls != 1 {
		t.Fatalf("Env.Now was called %d times, want 1", calls)
	}
}

// TestSnapshotReadsTheClockTwice proves the snapshot mode reads the clock
// once for when the report is emitted and once for when collection finishes.
func TestSnapshotReadsTheClockTwice(t *testing.T) {
	server, _, scenario := startScenario(t, "direct-complete")
	defer server.Close()
	cfgPath := writeConfig(t, scenarioConfig(server, scenario, t.TempDir()))

	env := testEnv(t)
	var calls int
	env.Now = clockCounter(&calls)
	env.NewID = fixedID("00000000-0000-4000-8000-000000000099")

	_, stderr, code := runArgs(context.Background(), env, []string{"report", "--config", cfgPath, "--snapshot", filepath.Join(t.TempDir(), "s.json")})
	if code != 0 {
		t.Fatalf("code = %d, want 0; stderr = %q", code, stderr)
	}
	if calls != 2 {
		t.Fatalf("Env.Now was called %d times, want 2", calls)
	}
}

// TestClockIsReadTwice proves the clock is read exactly twice in the PDF
// mode, which collects, and not at all in the mode from snapshot.
func TestClockIsReadTwice(t *testing.T) {
	t.Run("pdf", func(t *testing.T) {
		server, _, scenario := startScenario(t, "direct-complete")
		defer server.Close()
		cfgPath := writeConfig(t, scenarioConfig(server, scenario, t.TempDir()))

		env := withTypst(t, testEnv(t))
		var calls int
		env.Now = clockCounter(&calls)

		_, stderr, code := runArgs(context.Background(), env, []string{"report", "--config", cfgPath, "--output", filepath.Join(t.TempDir(), "r.pdf")})
		if code != 0 {
			t.Fatalf("code = %d, want 0; stderr = %q", code, stderr)
		}
		if calls != 2 {
			t.Fatalf("Env.Now was called %d times, want 2", calls)
		}
	})

	t.Run("from snapshot", func(t *testing.T) {
		snapPath, err := goldentest.SnapshotPath("direct-complete")
		if err != nil {
			t.Fatalf("SnapshotPath: %v", err)
		}

		env := withTypst(t, testEnv(t))
		var calls int
		env.Now = clockCounter(&calls)

		_, stderr, code := runArgs(context.Background(), env, []string{"report", "--from-snapshot", snapPath, "--output", filepath.Join(t.TempDir(), "r.pdf")})
		if code != 0 {
			t.Fatalf("code = %d, want 0; stderr = %q", code, stderr)
		}
		if calls != 0 {
			t.Fatalf("Env.Now was called %d times, want 0", calls)
		}
	})
}

func TestReportCommandRefusesARange(t *testing.T) {
	cfgPath := writeConfig(t, "[metrics]\nurl = \"https://prom.example.org\"\n[cluster]\nname = \"prod\"\nnamespaces = [\"default\"]\n")
	stdout, stderr, code := runArgs(context.Background(), testEnv(t), []string{"report", "--config", cfgPath, "--check", "--start", "x", "--end", "y"})
	if code != 2 {
		t.Fatalf("code = %d, want 2; stderr = %q", code, stderr)
	}
	if want := "error: analysis.start must look like 2026-09-01 00:00\n"; stderr != want {
		t.Fatalf("stderr = %q, want %q", stderr, want)
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want empty", stdout)
	}
}
