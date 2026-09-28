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
)

// reportFlagNames lists every flag report.go declares; TestReportHelpListsEveryFlag
// checks each one appears in the command's own help text.
var reportFlagNames = []string{
	"config", "metrics-url", "endpoint-type", "token", "insecure",
	"cluster", "cluster-label", "cluster-filter", "namespaces", "period",
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
// of Task 13 (internal/goldentest.fixedConfig), pointed at server.
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
