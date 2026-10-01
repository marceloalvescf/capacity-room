package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/marceloalvescf/k8s-capacity-report/internal/catalog"
	"github.com/marceloalvescf/k8s-capacity-report/internal/goldentest"
	"github.com/marceloalvescf/k8s-capacity-report/internal/promtest"
	"github.com/marceloalvescf/k8s-capacity-report/internal/snapshot"
)

// minimalConfig is a config.Config valid enough for config.Load to accept,
// pointed at an address nothing needs to answer: every test that uses it
// fails, or succeeds, before any request would matter to its assertion.
func minimalConfig(outputDir string) string {
	return fmt.Sprintf(`[metrics]
url = "http://127.0.0.1:1/api/v1"

[cluster]
name = "prod"
namespaces = ["payments"]

[report]
output_directory = %q
`, outputDir)
}

// writeScript writes an executable shell script into a fresh temp directory
// and returns its absolute path, the same shape internal/typst's own tests
// use.
func writeScript(t *testing.T, script string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "typst")
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatalf("writing script: %v", err)
	}
	return path
}

// envWithTypst returns env with KCR_TYPST pointed at binary.
func envWithTypst(env Env, binary string) Env {
	inner := env.Getenv
	env.Getenv = func(k string) string {
		if k == "KCR_TYPST" {
			return binary
		}
		return inner(k)
	}
	return env
}

// modeConflictRow is one row of the mode-conflict table: the flags given
// (report is prepended by the caller) and the message they must produce.
// It is a package-level type, rather than a literal local to
// TestModeConflicts, so TestEveryFailureHasThePrefix can walk the same
// table without copying it.
type modeConflictRow struct {
	name string
	args []string
	want string
}

func modeConflictRows() []modeConflictRow {
	rows := []modeConflictRow{
		{"check with output", []string{"--check", "--output", "r.pdf"}, "--check cannot be combined with --output"},
		{"check with snapshot", []string{"--check", "--snapshot", "s.json"}, "--check cannot be combined with --snapshot"},
		{"check with from-snapshot", []string{"--check", "--from-snapshot", "s.json"}, "--check cannot be combined with --from-snapshot"},
		{"snapshot with output", []string{"--snapshot", "s.json", "--output", "r.pdf"}, "--snapshot cannot be combined with --output"},
		{"from-snapshot with snapshot", []string{"--from-snapshot", "s.json", "--snapshot", "t.json"}, "--from-snapshot cannot be combined with --snapshot"},
		{"from-snapshot with config", []string{"--from-snapshot", "s.json", "--config", "c.toml"}, "--from-snapshot cannot be combined with --config"},
		{"lang with check", []string{"--check", "--lang", "en"}, "--lang applies only when a PDF is written"},
		{"lang with snapshot", []string{"--snapshot", "s.json", "--lang", "en"}, "--lang applies only when a PDF is written"},
	}
	for _, name := range collectingFlagNames {
		flagArgs := []string{"--from-snapshot", "s.json"}
		switch name {
		case "insecure":
			flagArgs = append(flagArgs, "--insecure")
		case "token":
			// A placeholder eight characters long: shorter values now stop
			// the run at the token length gate, before this mode conflict
			// is ever checked.
			flagArgs = append(flagArgs, "--"+name, "xxxxxxxx")
		default:
			flagArgs = append(flagArgs, "--"+name, "x")
		}
		rows = append(rows, modeConflictRow{
			"from-snapshot with " + name,
			flagArgs,
			"--from-snapshot cannot be combined with --" + name,
		})
	}
	return rows
}

func TestModeConflicts(t *testing.T) {
	for _, tt := range modeConflictRows() {
		t.Run(tt.name, func(t *testing.T) {
			args := append([]string{"report"}, tt.args...)
			stdout, stderr, code := runArgs(context.Background(), testEnv(t), args)
			if code != 2 {
				t.Fatalf("code = %d, want 2; stderr = %q", code, stderr)
			}
			if want := "error: " + tt.want + "\n"; stderr != want {
				t.Fatalf("stderr = %q, want %q", stderr, want)
			}
			if stdout != "" {
				t.Fatalf("stdout = %q, want empty", stdout)
			}
		})
	}
}

// argsRow is a bare name-and-arguments row, shared by the cobra-error and
// positional-argument tables below and by TestEveryFailureHasThePrefix.
type argsRow struct {
	name string
	args []string
}

// cobraErrorRows are invocations that fail during cobra's own flag
// resolution, before report's RunE ever runs: an unknown flag, in every
// position relative to a --token; the same at the root, where the unknown
// "command" is the word right after a --token; and --token given with no
// value at all.
func cobraErrorRows() []argsRow {
	return []argsRow{
		{"token equals then unknown flag", []string{"report", "--token=s3cr3t-value", "--nope"}},
		{"token space then unknown flag", []string{"report", "--token", "s3cr3t-value", "--nope"}},
		{"unknown flag then token equals", []string{"report", "--nope", "--token=s3cr3t-value"}},
		{"token equals at the root", []string{"--token=s3cr3t-value", "nope"}},
		{"token with no value", []string{"report", "--token"}},
	}
}

func TestCobraErrorsPassTheRedactor(t *testing.T) {
	for _, tt := range cobraErrorRows() {
		t.Run(tt.name, func(t *testing.T) {
			stdout, stderr, code := runArgs(context.Background(), testEnv(t), tt.args)
			if code != 2 {
				t.Fatalf("code = %d, want 2; stdout=%q stderr=%q", code, stdout, stderr)
			}
			assertExactlyOneErrorLine(t, stderr)
			if strings.Contains(stdout, "s3cr3t-value") || strings.Contains(stderr, "s3cr3t-value") {
				t.Fatalf("a stream held the token: stdout=%q stderr=%q", stdout, stderr)
			}
		})
	}
}

// positionalArgRows are invocations report refuses for carrying a
// positional argument; the row name doubles as the key into the exact
// wanted message in TestReportRefusesAPositionalArgument.
func positionalArgRows() []argsRow {
	return []argsRow{
		{"one argument", []string{"report", "s3cr3t-value", "--check"}},
		{"two arguments", []string{"report", "a", "b"}},
	}
}

func TestReportRefusesAPositionalArgument(t *testing.T) {
	want := map[string]string{
		"one argument":  "error: report takes no argument; found 1\n",
		"two arguments": "error: report takes no argument; found 2\n",
	}
	for _, tt := range positionalArgRows() {
		t.Run(tt.name, func(t *testing.T) {
			stdout, stderr, code := runArgs(context.Background(), testEnv(t), tt.args)
			if code != 2 {
				t.Fatalf("code = %d, want 2; stdout=%q stderr=%q", code, stdout, stderr)
			}
			if stderr != want[tt.name] {
				t.Fatalf("stderr = %q, want %q", stderr, want[tt.name])
			}
			if strings.Contains(stdout, "s3cr3t-value") || strings.Contains(stderr, "s3cr3t-value") {
				t.Fatalf("a stream held the argument: stdout=%q stderr=%q", stdout, stderr)
			}
		})
	}
}

// assertExactlyOneErrorLine requires stderr to be exactly one line, ending
// in a line break, that starts with "error: ".
func assertExactlyOneErrorLine(t *testing.T, stderr string) {
	t.Helper()
	if stderr == "" {
		t.Fatalf("stderr is empty, want one line starting with %q", "error: ")
	}
	lines := strings.Split(strings.TrimRight(stderr, "\n"), "\n")
	if len(lines) != 1 || !strings.HasPrefix(lines[0], "error: ") || !strings.HasSuffix(stderr, "\n") {
		t.Fatalf("stderr = %q, want exactly one line starting with %q", stderr, "error: ")
	}
}

// TestEveryFailureHasThePrefix walks every row of TestExitCodes,
// TestModeConflicts, TestCobraErrorsPassTheRedactor, and
// TestReportRefusesAPositionalArgument, from the same tables those tests
// use, and checks only the shape every one of them must share: standard
// error is exactly one line, starting with "error: ".
func TestEveryFailureHasThePrefix(t *testing.T) {
	for _, r := range exitCodeRows() {
		t.Run("exit-code/"+r.name, func(t *testing.T) {
			_, stderr, _ := r.run(t)
			assertExactlyOneErrorLine(t, stderr)
		})
	}
	for _, tt := range modeConflictRows() {
		t.Run("mode-conflict/"+tt.name, func(t *testing.T) {
			args := append([]string{"report"}, tt.args...)
			_, stderr, _ := runArgs(context.Background(), testEnv(t), args)
			assertExactlyOneErrorLine(t, stderr)
		})
	}
	for _, tt := range cobraErrorRows() {
		t.Run("cobra/"+tt.name, func(t *testing.T) {
			_, stderr, _ := runArgs(context.Background(), testEnv(t), tt.args)
			assertExactlyOneErrorLine(t, stderr)
		})
	}
	for _, tt := range positionalArgRows() {
		t.Run("positional/"+tt.name, func(t *testing.T) {
			_, stderr, _ := runArgs(context.Background(), testEnv(t), tt.args)
			assertExactlyOneErrorLine(t, stderr)
		})
	}
}

// assertExecutiveFile checks that path is a PDF whose data holds, as
// level-1 headings, the executive heading and the appendix heading and
// nothing else.
func assertExecutiveFile(t *testing.T, path string) {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	if !strings.HasPrefix(string(raw), "%PDF") {
		t.Fatalf("%s does not start as a PDF", path)
	}
	_, data, _ := strings.Cut(string(raw), "\n")
	var doc struct {
		Blocks []struct {
			Type  string `json:"type"`
			Level int    `json:"level"`
			Runs  []struct {
				Text string `json:"text"`
			} `json:"runs"`
		} `json:"blocks"`
	}
	if err := json.Unmarshal([]byte(data), &doc); err != nil {
		t.Fatalf("decoding the data of %s: %v", path, err)
	}
	var got []string
	for _, blk := range doc.Blocks {
		if blk.Type == "heading" && blk.Level == 1 && len(blk.Runs) > 0 {
			got = append(got, blk.Runs[0].Text)
		}
	}
	cat, err := catalog.Load(catalog.English)
	if err != nil {
		t.Fatalf("catalog.Load: %v", err)
	}
	var want []string
	for _, key := range []catalog.Key{catalog.SectionsExecutiveHeading, catalog.AppendixHeading} {
		heading, err := cat.Text(key, nil)
		if err != nil {
			t.Fatalf("Text(%s): %v", key, err)
		}
		want = append(want, heading)
	}
	if !slices.Equal(got, want) {
		t.Errorf("level-1 headings = %q, want %q", got, want)
	}
}

func TestExecutiveProfile(t *testing.T) {
	// runCase serves direct-complete and runs the command with the profile
	// given by the flag, the file, or the environment, as selected.
	runCase := func(t *testing.T, viaFlag, viaFile, viaEnv bool) {
		server, _, scenario := startScenario(t, "direct-complete")
		defer server.Close()

		outDir := t.TempDir()
		cfg := scenarioConfig(server, scenario, outDir)
		if viaFile {
			cfg += "profile = \"executive\"\n"
		}
		cfgPath := writeConfig(t, cfg)
		outPath := filepath.Join(outDir, "r.pdf")

		env := withTypst(t, testEnv(t))
		if viaEnv {
			inner := env.Getenv
			env.Getenv = func(k string) string {
				if k == "KCR_REPORT_PROFILE" {
					return "executive"
				}
				return inner(k)
			}
		}
		extra := []string{"--lang", "en", "--output", outPath}
		if viaFlag {
			extra = append(extra, "--report-profile", "executive")
		}
		_, stderr, code := runArgs(context.Background(), env, scenarioArgs(cfgPath, scenario, extra...))
		if code != 0 {
			t.Fatalf("code = %d, want 0; stderr = %q", code, stderr)
		}
		assertExecutiveFile(t, outPath)
	}

	t.Run("flag", func(t *testing.T) { runCase(t, true, false, false) })
	t.Run("configuration file", func(t *testing.T) { runCase(t, false, true, false) })
	t.Run("environment", func(t *testing.T) { runCase(t, false, false, true) })

	t.Run("from snapshot, flag", func(t *testing.T) {
		snapPath, err := snapshotFixturePath(t, "direct-complete")
		if err != nil {
			t.Fatalf("snapshotFixturePath: %v", err)
		}
		outPath := filepath.Join(t.TempDir(), "r.pdf")
		env := withTypst(t, testEnv(t))
		_, stderr, code := runArgs(context.Background(), env, []string{"report", "--from-snapshot", snapPath, "--report-profile", "executive", "--lang", "en", "--output", outPath})
		if code != 0 {
			t.Fatalf("code = %d, want 0; stderr = %q", code, stderr)
		}
		assertExecutiveFile(t, outPath)
	})

	t.Run("from snapshot, environment", func(t *testing.T) {
		snapPath, err := snapshotFixturePath(t, "direct-complete")
		if err != nil {
			t.Fatalf("snapshotFixturePath: %v", err)
		}
		outPath := filepath.Join(t.TempDir(), "r.pdf")
		env := testEnv(t)
		env.Getenv = func(k string) string {
			if k == "KCR_REPORT_PROFILE" {
				return "executive"
			}
			return ""
		}
		env = withTypst(t, env)
		_, stderr, code := runArgs(context.Background(), env, []string{"report", "--from-snapshot", snapPath, "--lang", "en", "--output", outPath})
		if code != 0 {
			t.Fatalf("code = %d, want 0; stderr = %q", code, stderr)
		}
		assertExecutiveFile(t, outPath)
	})
}

func TestExecutiveOutputName(t *testing.T) {
	snapPath, err := snapshotFixturePath(t, "direct-complete")
	if err != nil {
		t.Fatalf("snapshotFixturePath: %v", err)
	}
	outDir := t.TempDir()
	env := withTypst(t, testEnv(t))
	inner := env.Getenv
	env.Getenv = func(k string) string {
		if k == "KCR_OUTPUT_DIRECTORY" {
			return outDir
		}
		return inner(k)
	}
	_, stderr, code := runArgs(context.Background(), env, []string{"report", "--from-snapshot", snapPath, "--report-profile", "executive", "--lang", "en"})
	if code != 0 {
		t.Fatalf("code = %d, want 0; stderr = %q", code, stderr)
	}
	want := "report-resources-prod-executive-en-20260715-143000.pdf"
	if _, err := os.Stat(filepath.Join(outDir, want)); err != nil {
		t.Fatalf("%s was not written in %s: %v", want, outDir, err)
	}
}

// snapshotFixturePath returns the path of a committed golden snapshot; every
// TestExecutiveProfile from-snapshot case only needs a valid, decodable
// file, and its stored profile ("detailed") never matters since the flag or
// the environment always wins.
func snapshotFixturePath(t *testing.T, name string) (string, error) {
	t.Helper()
	abs, err := filepath.Abs(filepath.Join("..", "..", "testdata", "snapshot", name+".json"))
	if err != nil {
		return "", err
	}
	return abs, nil
}

// insufficientServer answers the probe and the scope query normally, and
// every capability probe (every other "count(" query) with an empty
// result, so that discovery.CheckSufficient refuses the run.
func insufficientServer() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		query := r.PostFormValue("query")
		w.Header().Set("Content-Type", "application/json")
		switch {
		case query == "vector(1)":
			_, _ = fmt.Fprint(w, `{"status":"success","data":{"resultType":"vector","result":[{"metric":{},"value":[0,"1"]}]}}`)
		case strings.HasPrefix(query, "count by ("):
			_, _ = fmt.Fprint(w, `{"status":"success","data":{"resultType":"vector","result":[{"metric":{"cluster":"prod"},"value":[0,"1"]}]}}`)
		default:
			_, _ = fmt.Fprint(w, `{"status":"success","data":{"resultType":"vector","result":[]}}`)
		}
	}))
}

// alwaysHandler answers every request with the given status and a
// Prometheus-shaped body, used for the exit-code rows that only need a
// fixed HTTP failure.
func alwaysStatus(status int) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = fmt.Fprint(w, `{"status":"error","errorType":"forbidden","error":"no"}`)
	}))
}

// exitCodeRow is one row of the exit-code table: a package-level type,
// rather than a literal local to TestExitCodes, so TestEveryFailureHasThePrefix
// can walk the same table without copying it.
type exitCodeRow struct {
	name string
	want int
	run  func(t *testing.T) (stdout, stderr string, code int)
}

func exitCodeRows() []exitCodeRow {
	return []exitCodeRow{
		{
			name: "missing configuration file",
			want: 2,
			run: func(t *testing.T) (string, string, int) {
				env := withTypst(t, testEnv(t))
				return runArgs(context.Background(), env, []string{"report", "--config", filepath.Join(t.TempDir(), "missing.toml")})
			},
		},
		{
			name: "unknown configuration key",
			want: 2,
			run: func(t *testing.T) (string, string, int) {
				text := fmt.Sprintf(`[metrics]
url = "http://127.0.0.1:1/api/v1"
bogus = 1

[cluster]
name = "prod"
namespaces = ["payments"]

[report]
output_directory = %q
`, t.TempDir())
				cfgPath := writeConfig(t, text)
				env := withTypst(t, testEnv(t))
				return runArgs(context.Background(), env, []string{"report", "--config", cfgPath})
			},
		},
		{
			name: "401",
			want: 3,
			run: func(t *testing.T) (string, string, int) {
				server := alwaysStatus(http.StatusUnauthorized)
				defer server.Close()
				cfgPath := writeConfig(t, minimalConfigAt(t, server.URL+"/api/v1", t.TempDir()))
				env := withTypst(t, testEnv(t))
				return runArgs(context.Background(), env, []string{"report", "--config", cfgPath})
			},
		},
		{
			name: "503 every time",
			want: 3,
			run: func(t *testing.T) (string, string, int) {
				server := alwaysStatus(http.StatusServiceUnavailable)
				defer server.Close()
				cfgPath := writeConfig(t, minimalConfigAt(t, server.URL+"/api/v1", t.TempDir()))
				env := withTypst(t, testEnv(t))
				return runArgs(context.Background(), env, []string{"report", "--config", cfgPath})
			},
		},
		{
			name: "TLS server with no bundle",
			want: 3,
			run: func(t *testing.T) (string, string, int) {
				server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					w.WriteHeader(200)
				}))
				defer server.Close()
				cfgPath := writeConfig(t, minimalConfigAt(t, server.URL+"/api/v1", t.TempDir()))
				env := withTypst(t, testEnv(t))
				return runArgs(context.Background(), env, []string{"report", "--config", cfgPath})
			},
		},
		{
			name: "thanos-multicluster with cluster-filter none",
			want: 4,
			run: func(t *testing.T) (string, string, int) {
				server, _, scenario := startScenario(t, "thanos-multicluster")
				defer server.Close()
				cfgPath := writeConfig(t, scenarioConfig(server, scenario, t.TempDir()))
				env := withTypst(t, testEnv(t))
				return runArgs(context.Background(), env, []string{"report", "--config", cfgPath, "--cluster-filter", "none"})
			},
		},
		{
			name: "cluster other",
			want: 4,
			run: func(t *testing.T) (string, string, int) {
				server, _, scenario := startScenario(t, "direct-complete")
				defer server.Close()
				cfgPath := writeConfig(t, scenarioConfig(server, scenario, t.TempDir()))
				env := withTypst(t, testEnv(t))
				return runArgs(context.Background(), env, []string{"report", "--config", cfgPath, "--cluster", "other"})
			},
		},
		{
			name: "every count() empty",
			want: 5,
			run: func(t *testing.T) (string, string, int) {
				server := insufficientServer()
				defer server.Close()
				cfgPath := writeConfig(t, minimalConfigAt(t, server.URL+"/api/v1", t.TempDir()))
				env := withTypst(t, testEnv(t))
				return runArgs(context.Background(), env, []string{"report", "--config", cfgPath})
			},
		},
		{
			name: "no compiler",
			want: 6,
			run: func(t *testing.T) (string, string, int) {
				cfgPath := writeConfig(t, minimalConfig(t.TempDir()))
				env := testEnv(t) // no KCR_TYPST, LookPath fails
				return runArgs(context.Background(), env, []string{"report", "--config", cfgPath})
			},
		},
		{
			name: "wrong typst version",
			want: 6,
			run: func(t *testing.T) (string, string, int) {
				cfgPath := writeConfig(t, minimalConfig(t.TempDir()))
				binary := writeScript(t, "#!/bin/sh\necho 'typst 0.15.0 (x)'\n")
				env := envWithTypst(testEnv(t), binary)
				return runArgs(context.Background(), env, []string{"report", "--config", cfgPath})
			},
		},
		{
			name: "compile exits 1",
			want: 6,
			run: func(t *testing.T) (string, string, int) {
				server, _, scenario := startScenario(t, "direct-complete")
				defer server.Close()
				cfgPath := writeConfig(t, scenarioConfig(server, scenario, t.TempDir()))
				binary := writeScript(t, "#!/bin/sh\ncase \"$1\" in --version) /bin/echo 'typst 0.15.1 (x)'; exit 0;; esac\nexit 1\n")
				env := envWithTypst(testEnv(t), binary)
				return runArgs(context.Background(), env, []string{"report", "--config", cfgPath})
			},
		},
		{
			name: "compile warns",
			want: 6,
			run: func(t *testing.T) (string, string, int) {
				server, _, scenario := startScenario(t, "direct-complete")
				defer server.Close()
				cfgPath := writeConfig(t, scenarioConfig(server, scenario, t.TempDir()))
				binary := writeScript(t, "#!/bin/sh\n"+
					"case \"$1\" in --version) /bin/echo 'typst 0.15.1 (x)'; exit 0;; esac\n"+
					"/bin/echo 'x.typ:1:1: warning: x' >&2\n"+
					"exit 0\n")
				env := envWithTypst(testEnv(t), binary)
				return runArgs(context.Background(), env, []string{"report", "--config", cfgPath})
			},
		},
	}
}

func TestExitCodes(t *testing.T) {
	for _, r := range exitCodeRows() {
		t.Run(r.name, func(t *testing.T) {
			stdout, stderr, code := r.run(t)
			if code != r.want {
				t.Fatalf("code = %d, want %d; stderr = %q", code, r.want, stderr)
			}
			assertExactlyOneErrorLine(t, stderr)
			if stdout != "" {
				t.Fatalf("stdout = %q, want empty", stdout)
			}
		})
	}
}

// minimalConfigAt is minimalConfig pointed at a specific metrics URL.
func minimalConfigAt(t *testing.T, url, outputDir string) string {
	t.Helper()
	return fmt.Sprintf(`[metrics]
url = %q

[cluster]
name = "prod"
namespaces = ["payments"]

[report]
output_directory = %q
`, url, outputDir)
}

func TestTypstIsProbedBeforeAnyRequest(t *testing.T) {
	server, handler, scenario := startScenario(t, "direct-complete")
	defer server.Close()

	cfgPath := writeConfig(t, scenarioConfig(server, scenario, t.TempDir()))
	env := testEnv(t) // no compiler anywhere
	_, stderr, code := runArgs(context.Background(), env, []string{"report", "--config", cfgPath})
	if code != 6 {
		t.Fatalf("code = %d, want 6; stderr = %q", code, stderr)
	}
	if len(handler.Seen()) != 0 {
		t.Fatalf("the server received %d requests, want 0", len(handler.Seen()))
	}
}

func TestOutputPathIsCheckedBeforeAnyRequest(t *testing.T) {
	tests := []struct {
		name string
		args func(t *testing.T, cfgPath string) []string
		want string
	}{
		{
			name: "output not pdf",
			args: func(t *testing.T, cfgPath string) []string {
				return []string{"report", "--config", cfgPath, "--output", filepath.Join(t.TempDir(), "r.txt")}
			},
			want: "output path must end in .pdf",
		},
		{
			name: "output directory missing",
			args: func(t *testing.T, cfgPath string) []string {
				return []string{"report", "--config", cfgPath, "--output", filepath.Join(t.TempDir(), "missing", "r.pdf")}
			},
		},
		{
			name: "output is a folder",
			args: func(t *testing.T, cfgPath string) []string {
				dir := t.TempDir()
				path := filepath.Join(dir, "r.pdf")
				if err := os.Mkdir(path, 0o755); err != nil {
					t.Fatalf("mkdir: %v", err)
				}
				return []string{"report", "--config", cfgPath, "--output", path}
			},
		},
		{
			name: "snapshot not json",
			args: func(t *testing.T, cfgPath string) []string {
				return []string{"report", "--config", cfgPath, "--snapshot", filepath.Join(t.TempDir(), "s.txt")}
			},
			want: "snapshot path must end in .json",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server, handler, scenario := startScenario(t, "direct-complete")
			defer server.Close()
			cfgPath := writeConfig(t, scenarioConfig(server, scenario, t.TempDir()))
			env := withTypst(t, testEnv(t))

			_, stderr, code := runArgs(context.Background(), env, tt.args(t, cfgPath))
			if code != 6 {
				t.Fatalf("code = %d, want 6; stderr = %q", code, stderr)
			}
			if tt.want != "" && stderr != "error: "+tt.want+"\n" {
				t.Fatalf("stderr = %q, want %q", stderr, "error: "+tt.want+"\n")
			}
			if len(handler.Seen()) != 0 {
				t.Fatalf("the server received %d requests, want 0", len(handler.Seen()))
			}
		})
	}
}

// TestPDFModeAcceptsARelativeOutputPath is a regression test: typst.Args
// requires Job.Output to be absolute, so a relative --output (or a relative
// --snapshot's output directory building a relative default PDF name) must
// be resolved to an absolute path before compileToPath ever builds the job.
func TestPDFModeAcceptsARelativeOutputPath(t *testing.T) {
	server, _, scenario := startScenario(t, "direct-complete")
	defer server.Close()

	cfgPath := writeConfig(t, scenarioConfig(server, scenario, t.TempDir()))
	env := withTypst(t, testEnv(t))
	dir := t.TempDir()
	t.Chdir(dir)

	_, stderr, code := runArgs(context.Background(), env, []string{"report", "--config", cfgPath, "--output", "relative.pdf"})
	if code != 0 {
		t.Fatalf("code = %d, want 0; stderr = %q", code, stderr)
	}
	if _, err := os.Stat(filepath.Join(dir, "relative.pdf")); err != nil {
		t.Fatalf("relative.pdf was not written in the working directory: %v", err)
	}
}

func TestOutputReplacesAnExistingFile(t *testing.T) {
	server, _, scenario := startScenario(t, "direct-complete")
	defer server.Close()

	cfgPath := writeConfig(t, scenarioConfig(server, scenario, t.TempDir()))
	dir := t.TempDir()
	outPath := filepath.Join(dir, "r.pdf")
	if err := os.WriteFile(outPath, []byte("old"), 0o644); err != nil {
		t.Fatalf("writing old file: %v", err)
	}

	env := withTypst(t, testEnv(t))
	_, stderr, code := runArgs(context.Background(), env, []string{"report", "--config", cfgPath, "--output", outPath})
	if code != 0 {
		t.Fatalf("code = %d, want 0; stderr = %q", code, stderr)
	}

	got, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("reading %s: %v", outPath, err)
	}
	if string(got) == "old" {
		t.Fatalf("output file was not replaced")
	}
	assertNoTempFilesBeside(t, dir)
}

func TestFailedCompileKeepsTheOldFile(t *testing.T) {
	server, _, scenario := startScenario(t, "direct-complete")
	defer server.Close()

	cfgPath := writeConfig(t, scenarioConfig(server, scenario, t.TempDir()))
	dir := t.TempDir()
	outPath := filepath.Join(dir, "r.pdf")
	if err := os.WriteFile(outPath, []byte("old"), 0o644); err != nil {
		t.Fatalf("writing old file: %v", err)
	}

	binary := writeScript(t, "#!/bin/sh\ncase \"$1\" in --version) /bin/echo 'typst 0.15.1 (x)'; exit 0;; esac\nexit 1\n")
	env := envWithTypst(testEnv(t), binary)
	_, stderr, code := runArgs(context.Background(), env, []string{"report", "--config", cfgPath, "--output", outPath})
	if code != 6 {
		t.Fatalf("code = %d, want 6; stderr = %q", code, stderr)
	}

	got, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("reading %s: %v", outPath, err)
	}
	if string(got) != "old" {
		t.Fatalf("output file = %q, want %q (unchanged)", got, "old")
	}
	assertNoTempFilesBeside(t, dir)
}

func TestSecretInARefusedAddress(t *testing.T) {
	env := testEnv(t)
	_, stderr, code := runArgs(context.Background(), env, []string{
		"report", "--metrics-url", "https://user:hunter2@prom.example/?token=abc",
		"--cluster", "prod", "--namespaces", "payments",
	})
	if code != 2 {
		t.Fatalf("code = %d, want 2; stderr = %q", code, stderr)
	}
	if strings.Contains(stderr, "hunter2") || strings.Contains(stderr, "abc") {
		t.Fatalf("stderr = %q, holds a secret", stderr)
	}
}

func TestTokenNeverReachesAStream(t *testing.T) {
	const token = "s3cr3t-value"

	t.Run("via flag", func(t *testing.T) {
		server, _, scenario := startScenario(t, "direct-complete")
		defer server.Close()
		cfgPath := writeConfig(t, scenarioConfig(server, scenario, t.TempDir()))
		env := withTypst(t, testEnv(t))
		stdout, stderr, _ := runArgs(context.Background(), env, []string{"report", "--config", cfgPath, "--token", token, "--verbose"})
		if strings.Contains(stdout, token) || strings.Contains(stderr, token) {
			t.Fatalf("a stream held the token: stdout=%q stderr=%q", stdout, stderr)
		}
	})

	t.Run("via variable", func(t *testing.T) {
		server, _, scenario := startScenario(t, "direct-complete")
		defer server.Close()
		text := scenarioConfig(server, scenario, t.TempDir(), "KCR_TEST_TOKEN")
		cfgPath := writeConfig(t, text)
		env := withTypst(t, testEnv(t))
		env.Getenv = func(k string) string {
			if k == "KCR_TEST_TOKEN" {
				return token
			}
			return ""
		}
		env = withTypst(t, env)
		stdout, stderr, _ := runArgs(context.Background(), env, []string{"report", "--config", cfgPath, "--verbose"})
		if strings.Contains(stdout, token) || strings.Contains(stderr, token) {
			t.Fatalf("a stream held the token: stdout=%q stderr=%q", stdout, stderr)
		}
	})

	t.Run("server echoes authorization in a 500 body", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = fmt.Fprintf(w, "authorization: %s", r.Header.Get("Authorization"))
		}))
		defer server.Close()
		cfgPath := writeConfig(t, minimalConfigAt(t, server.URL+"/api/v1", t.TempDir()))
		env := withTypst(t, testEnv(t))
		stdout, stderr, _ := runArgs(context.Background(), env, []string{"report", "--config", cfgPath, "--token", token})
		if strings.Contains(stdout, token) || strings.Contains(stderr, token) {
			t.Fatalf("a stream held the token: stdout=%q stderr=%q", stdout, stderr)
		}
	})

	t.Run("compiler prints its environment and arguments", func(t *testing.T) {
		server, _, scenario := startScenario(t, "direct-complete")
		defer server.Close()
		cfgPath := writeConfig(t, scenarioConfig(server, scenario, t.TempDir()))
		binary := writeScript(t, "#!/bin/sh\n"+
			"case \"$1\" in --version) /bin/echo 'typst 0.15.1 (x)'; exit 0;; esac\n"+
			"/usr/bin/env\n\"$@\"\nexit 1\n")
		env := envWithTypst(testEnv(t), binary)
		stdout, stderr, _ := runArgs(context.Background(), env, []string{"report", "--config", cfgPath, "--token", token, "--verbose"})
		if strings.Contains(stdout, token) || strings.Contains(stderr, token) {
			t.Fatalf("a stream held the token: stdout=%q stderr=%q", stdout, stderr)
		}
	})
}

func TestTokenIsRedactedBeforeTheConfigurationLoads(t *testing.T) {
	env := testEnv(t)
	_, stderr, code := runArgs(context.Background(), env, []string{
		"report", "--token", "s3cr3t-value", "--config", filepath.Join(t.TempDir(), "missing.toml"),
	})
	if code != 2 {
		t.Fatalf("code = %d, want 2", code)
	}
	if strings.Contains(stderr, "s3cr3t-value") {
		t.Fatalf("stderr = %q, holds the token", stderr)
	}
}

func TestSnapshotFileRefusals(t *testing.T) {
	goldenPath, err := snapshotFixturePath(t, "direct-complete")
	if err != nil {
		t.Fatalf("snapshotFixturePath: %v", err)
	}
	golden, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("reading %s: %v", goldenPath, err)
	}

	writeVariant := func(t *testing.T, data []byte) string {
		path := filepath.Join(t.TempDir(), "s.json")
		if err := os.WriteFile(path, data, 0o644); err != nil {
			t.Fatalf("writing variant: %v", err)
		}
		return path
	}

	tests := []struct {
		name string
		path func(t *testing.T) string
	}{
		{"missing file", func(t *testing.T) string { return filepath.Join(t.TempDir(), "missing.json") }},
		{"other schema version", func(t *testing.T) string {
			return writeVariant(t, []byte(strings.Replace(string(golden), `"schema_version": "1.0"`, `"schema_version": "9.9"`, 1)))
		}},
		{"unknown key", func(t *testing.T) string {
			return writeVariant(t, []byte(strings.Replace(string(golden), "\"schema_version\": \"1.0\",", "\"schema_version\": \"1.0\", \"bogus\": true,", 1)))
		}},
		{"two documents", func(t *testing.T) string {
			return writeVariant(t, append(append([]byte{}, golden...), golden...))
		}},
		{"file cut in the middle", func(t *testing.T) string {
			return writeVariant(t, golden[:len(golden)/2])
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := withTypst(t, testEnv(t))
			_, stderr, code := runArgs(context.Background(), env, []string{"report", "--from-snapshot", tt.path(t)})
			if code != 2 {
				t.Fatalf("code = %d, want 2; stderr = %q", code, stderr)
			}
			if !strings.HasPrefix(stderr, "error: ") {
				t.Fatalf("stderr = %q, want it to start with %q", stderr, "error: ")
			}
		})
	}
}

// TestFromSnapshotRefusesABadProfileOrSeverity proves that a profile or a
// minimum severity outside its values stops a run from a snapshot with the
// configuration's own message and exit code 2, before the output path is
// checked: the path given lies in a folder that does not exist.
func TestFromSnapshotRefusesABadProfileOrSeverity(t *testing.T) {
	snapPath, err := snapshotFixturePath(t, "direct-complete")
	if err != nil {
		t.Fatalf("snapshotFixturePath: %v", err)
	}
	tests := []struct {
		flag string
		want string
	}{
		{"--report-profile=Executive", "error: report.profile must be detailed or executive\n"},
		{"--min-severity=High", "error: report.min_severity must be all, attention, or critical\n"},
	}
	for _, tt := range tests {
		t.Run(tt.flag, func(t *testing.T) {
			outPath := filepath.Join(t.TempDir(), "missing", "r.pdf")
			stdout, stderr, code := runArgs(context.Background(), testEnv(t), []string{"report", "--from-snapshot", snapPath, tt.flag, "--output", outPath})
			if code != 2 {
				t.Fatalf("code = %d, want 2; stderr = %q", code, stderr)
			}
			if stderr != tt.want {
				t.Errorf("stderr = %q, want %q", stderr, tt.want)
			}
			if stdout != "" {
				t.Errorf("stdout = %q, want nothing", stdout)
			}
		})
	}
}

// TestPanicWithATokenOfTheEnvironment proves that a panic that happens
// after the configuration's own token is known (here, loaded from an
// environment variable, not --token) but before the mode function returns
// is still redacted. The clock's second call, after the collectors return, is
// the one that panics, well after runSnapshot has already added the token to
// the Redactor.
func TestPanicWithATokenOfTheEnvironment(t *testing.T) {
	const token = "s3cr3t-value"
	server, _, scenario := startScenario(t, "direct-complete")
	defer server.Close()

	text := scenarioConfig(server, scenario, t.TempDir(), "KCR_TOKEN")
	cfgPath := writeConfig(t, text)

	env := testEnv(t)
	base := time.Date(2026, time.July, 15, 14, 30, 0, 0, time.UTC)
	calls := 0
	env.Now = func() time.Time {
		calls++
		if calls == 1 {
			return base
		}
		panic("boom holding " + token)
	}
	env.Getenv = func(k string) string {
		if k == "KCR_TOKEN" {
			return token
		}
		return ""
	}
	env = withTypst(t, env)

	stdout, stderr, code := runArgs(context.Background(), env, []string{"report", "--config", cfgPath, "--snapshot", filepath.Join(t.TempDir(), "s.json")})
	if code != 1 {
		t.Fatalf("code = %d, want 1; stderr = %q", code, stderr)
	}
	if !strings.Contains(stderr, "error: internal failure:") {
		t.Fatalf("stderr = %q, want it to contain %q", stderr, "error: internal failure:")
	}
	if strings.Contains(stderr, token) || strings.Contains(stdout, token) {
		t.Fatalf("a stream held the token: stdout=%q stderr=%q", stdout, stderr)
	}
}

func TestPanicIsReportedAndRedacted(t *testing.T) {
	const token = "s3cr3t-panic-value"
	server, _, scenario := startScenario(t, "direct-complete")
	defer server.Close()
	cfgPath := writeConfig(t, scenarioConfig(server, scenario, t.TempDir()))

	env := withTypst(t, testEnv(t))
	env.Now = func() time.Time {
		panic("boom holding " + token)
	}

	stdout, stderr, code := runArgs(context.Background(), env, []string{"report", "--config", cfgPath, "--token", token})
	if code != 1 {
		t.Fatalf("code = %d, want 1; stderr = %q", code, stderr)
	}
	if !strings.Contains(stderr, "error: internal failure:") {
		t.Fatalf("stderr = %q, want it to contain %q", stderr, "error: internal failure:")
	}
	if strings.Contains(stderr, token) || strings.Contains(stdout, token) {
		t.Fatalf("a stream held the token: stdout=%q stderr=%q", stdout, stderr)
	}
}

func TestCancelledContext(t *testing.T) {
	server, _, scenario := startScenario(t, "direct-complete")
	defer server.Close()
	dir := t.TempDir()
	cfgPath := writeConfig(t, scenarioConfig(server, scenario, dir))
	outPath := filepath.Join(dir, "r.pdf")

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	env := withTypst(t, testEnv(t))
	stdout, stderr, code := runArgs(ctx, env, []string{"report", "--config", cfgPath, "--output", outPath})
	if code != 1 {
		t.Fatalf("code = %d, want 1; stderr = %q", code, stderr)
	}
	if want := "error: interrupted\n"; stderr != want {
		t.Fatalf("stderr = %q, want %q", stderr, want)
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want empty", stdout)
	}
	if _, err := os.Stat(outPath); err == nil {
		t.Fatalf("%s was created, want no output file", outPath)
	}
}

func TestWorkFolderIsRemoved(t *testing.T) {
	t.Run("good run", func(t *testing.T) {
		server, _, scenario := startScenario(t, "direct-complete")
		defer server.Close()
		outDir := t.TempDir()
		cfgPath := writeConfig(t, scenarioConfig(server, scenario, outDir))
		tempDir := t.TempDir()
		env := withTypst(t, testEnv(t))
		env.TempDir = tempDir

		_, stderr, code := runArgs(context.Background(), env, []string{"report", "--config", cfgPath, "--output", filepath.Join(outDir, "r.pdf")})
		if code != 0 {
			t.Fatalf("code = %d, want 0; stderr = %q", code, stderr)
		}
		assertTempDirEmpty(t, tempDir)
	})

	t.Run("compile fails", func(t *testing.T) {
		server, _, scenario := startScenario(t, "direct-complete")
		defer server.Close()
		outDir := t.TempDir()
		cfgPath := writeConfig(t, scenarioConfig(server, scenario, outDir))
		tempDir := t.TempDir()
		binary := writeScript(t, "#!/bin/sh\ncase \"$1\" in --version) /bin/echo 'typst 0.15.1 (x)'; exit 0;; esac\nexit 1\n")
		env := envWithTypst(testEnv(t), binary)
		env.TempDir = tempDir

		_, _, code := runArgs(context.Background(), env, []string{"report", "--config", cfgPath, "--output", filepath.Join(outDir, "r.pdf")})
		if code != 6 {
			t.Fatalf("code = %d, want 6", code)
		}
		assertTempDirEmpty(t, tempDir)
	})

	t.Run("panic inside the compile step", func(t *testing.T) {
		server, _, scenario := startScenario(t, "direct-complete")
		defer server.Close()
		outDir := t.TempDir()
		cfgPath := writeConfig(t, scenarioConfig(server, scenario, outDir))
		tempDir := t.TempDir()

		// A compiler binary that does not exist makes exec.CommandContext's
		// underlying syscall fail; to exercise a genuine panic inside the
		// compile step instead, TempDir itself is made a file, so
		// os.MkdirTemp panics via an unexpected type is avoided — instead we
		// simulate the panic directly by making NewID panic, which fires
		// after typst has already been probed and located, while the run is
		// past the point compileToPath's own defers would be registered had
		// collection succeeded first.
		env := withTypst(t, testEnv(t))
		env.TempDir = tempDir
		env.NewID = func() (string, error) { panic("boom") }

		_, stderr, code := runArgs(context.Background(), env, []string{"report", "--config", cfgPath, "--output", filepath.Join(outDir, "r.pdf")})
		if code != 1 {
			t.Fatalf("code = %d, want 1; stderr = %q", code, stderr)
		}
		assertTempDirEmpty(t, tempDir)
	})
}

// TestWorkFolderIsRemovedOnSuccess, TestWorkFolderIsRemovedOnError, and
// TestWorkFolderIsRemovedOnPanic test withWorkFolder directly, by itself,
// rather than through a whole collecting run: the life of the work folder
// is withWorkFolder's own responsibility (make it, defer its removal, run
// the given work), so it is provable on its own terms, including the one
// case (a panic while the folder is open) that no combination of the
// frozen Env seams can reach from a real run.
func TestWorkFolderIsRemovedOnSuccess(t *testing.T) {
	parent := t.TempDir()
	err := withWorkFolder(parent, func(string) error { return nil })
	if err != nil {
		t.Fatalf("withWorkFolder() error = %v, want nil", err)
	}
	assertTempDirEmpty(t, parent)
}

func TestWorkFolderIsRemovedOnError(t *testing.T) {
	parent := t.TempDir()
	boom := errors.New("boom")
	err := withWorkFolder(parent, func(string) error { return boom })
	if !errors.Is(err, boom) {
		t.Fatalf("withWorkFolder() error = %v, want %v", err, boom)
	}
	assertTempDirEmpty(t, parent)
}

func TestWorkFolderIsRemovedOnPanic(t *testing.T) {
	parent := t.TempDir()
	recovered := func() (r any) {
		defer func() { r = recover() }()
		_ = withWorkFolder(parent, func(dir string) error {
			if err := os.WriteFile(filepath.Join(dir, "x"), []byte("y"), 0o644); err != nil {
				t.Fatalf("writing inside the work folder: %v", err)
			}
			panic("boom")
		})
		return nil
	}()
	if recovered != "boom" {
		t.Fatalf("recovered = %v, want the panic to reach the caller", recovered)
	}
	assertTempDirEmpty(t, parent)
}

// assertTempDirEmpty fails the test if dir holds any entry: every work
// folder compileToPath makes under it must have been removed.
func assertTempDirEmpty(t *testing.T, dir string) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("reading %s: %v", dir, err)
	}
	if len(entries) != 0 {
		var names []string
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Fatalf("%s still holds %v", dir, names)
	}
}

// TestTokenLengthStopsTheRunBeforeAnythingElse proves that a --token value
// shorter than config.MinTokenLength characters stops the run at once, with
// exactly one message, before cobra reads anything else: combined with an
// otherwise-erroring flag, or given at the root before any command, the
// token message still wins.
func TestTokenLengthStopsTheRunBeforeAnythingElse(t *testing.T) {
	want := "error: the token must hold at least 8 characters\n"

	tests := []struct {
		name string
		args []string
	}{
		{"a single letter, space form", []string{"report", "--token", "e"}},
		{"seven characters, equals form", []string{"report", "--token=1234567"}},
		{"seven accented characters", []string{"report", "--token", "áéíóúçã"}},
		{"combined with an unrelated bad flag", []string{"report", "--token", "e", "--nope"}},
		{"at the root, before any command", []string{"--token=e", "nope"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stdout, stderr, code := runArgs(context.Background(), testEnv(t), tt.args)
			if code != 2 {
				t.Fatalf("code = %d, want 2; stdout=%q stderr=%q", code, stdout, stderr)
			}
			if stderr != want {
				t.Fatalf("stderr = %q, want %q", stderr, want)
			}
			if stdout != "" {
				t.Fatalf("stdout = %q, want empty", stdout)
			}
		})
	}
}

// TestTokenLongEnoughIsAccepted proves the length gate lets an eight
// character token, plain or accented, through: the run still fails, for an
// unrelated reason (no configuration file at the given path), which proves
// the gate did not swallow it.
func TestTokenLongEnoughIsAccepted(t *testing.T) {
	tests := []struct {
		name  string
		token string
	}{
		{"eight plain characters", "12345678"},
		{"eight accented characters", "áéíóúçãõ"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			missing := filepath.Join(t.TempDir(), "missing.toml")
			stdout, stderr, code := runArgs(context.Background(), testEnv(t), []string{
				"report", "--token", tt.token, "--config", missing,
			})
			if code != 2 {
				t.Fatalf("code = %d, want 2; stdout=%q stderr=%q", code, stdout, stderr)
			}
			if strings.Contains(stderr, "the token must hold at least 8 characters") {
				t.Fatalf("stderr = %q, want the length gate to accept this token", stderr)
			}
		})
	}
}

// TestTokenVariableTooShortStopsTheRun proves the same MinTokenLength rule
// applies to a token read from the variable metrics.token_env names, not
// only to one given on the command line.
func TestTokenVariableTooShortStopsTheRun(t *testing.T) {
	cfgPath := writeConfig(t, fmt.Sprintf(`[metrics]
url = "http://127.0.0.1:1/api/v1"
token_env = "KCR_TOKEN"

[cluster]
name = "prod"
namespaces = ["payments"]

[report]
output_directory = %q
`, t.TempDir()))

	env := testEnv(t)
	env.Getenv = envFunc(map[string]string{"KCR_TOKEN": "abc"})

	stdout, stderr, code := runArgs(context.Background(), env, []string{"report", "--config", cfgPath})
	if code != 2 {
		t.Fatalf("code = %d, want 2; stdout=%q stderr=%q", code, stdout, stderr)
	}
	want := "error: the token must hold at least 8 characters\n"
	if stderr != want {
		t.Fatalf("stderr = %q, want %q", stderr, want)
	}
}

// TestCompileRefusesANonPDFOutput proves that a compiler that exits 0
// without warning, but writes something other than a real PDF to the file
// it was told to write to, is never reported as a successful run: the
// command reads the first bytes of that file itself, after the compile and
// before the rename, and refuses anything that does not start with the PDF
// magic bytes.
func TestCompileRefusesANonPDFOutput(t *testing.T) {
	tests := []struct {
		name       string
		writeBogus string
	}{
		{"plain text", `/bin/echo -n 'NOT A PDF' > "$last"` + "\n"},
		{"empty file", `: > "$last"` + "\n"},
		{"only the PDF prefix, cut short", `/bin/printf '%%PDF' > "$last"` + "\n"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server, _, scenario := startScenario(t, "direct-complete")
			defer server.Close()
			outDir := t.TempDir()
			cfgPath := writeConfig(t, scenarioConfig(server, scenario, outDir))

			outputPath := filepath.Join(outDir, "r.pdf")
			const older = "an older report, untouched"
			if err := os.WriteFile(outputPath, []byte(older), 0o644); err != nil {
				t.Fatalf("writing the older file: %v", err)
			}

			script := "#!/bin/sh\n" +
				"case \"$1\" in --version) /bin/echo 'typst 0.15.1 (x)'; exit 0;; esac\n" +
				"last=\"\"\n" +
				"for a in \"$@\"; do last=\"$a\"; done\n" +
				tt.writeBogus +
				"exit 0\n"
			binary := writeScript(t, script)
			env := envWithTypst(testEnv(t), binary)

			_, stderr, code := runArgs(context.Background(), env, []string{"report", "--config", cfgPath, "--output", outputPath})
			if code != 6 {
				t.Fatalf("code = %d, want 6; stderr = %q", code, stderr)
			}
			want := "error: the compiler wrote a file that is not a PDF\n"
			if stderr != want {
				t.Fatalf("stderr = %q, want %q", stderr, want)
			}

			data, err := os.ReadFile(outputPath)
			if err != nil {
				t.Fatalf("reading the target after the failed run: %v", err)
			}
			if string(data) != older {
				t.Fatalf("the target = %q, want the older file untouched: %q", data, older)
			}

			entries, err := os.ReadDir(outDir)
			if err != nil {
				t.Fatalf("reading %s: %v", outDir, err)
			}
			if len(entries) != 1 || entries[0].Name() != "r.pdf" {
				var names []string
				for _, e := range entries {
					names = append(names, e.Name())
				}
				t.Fatalf("%s holds %v, want only r.pdf (no leftover temporary file)", outDir, names)
			}
		})
	}
}

func TestReportJSONIsWrittenInsideTheWorkFolder(t *testing.T) {
	server, _, scenario := startScenario(t, "direct-complete")
	defer server.Close()
	outDir := t.TempDir()
	cfgPath := writeConfig(t, scenarioConfig(server, scenario, outDir))

	argsPath := filepath.Join(t.TempDir(), "args.txt")
	reportJSONCopy := filepath.Join(t.TempDir(), "report.json.copy")
	script := "#!/bin/sh\n" +
		"case \"$1\" in --version) /bin/echo 'typst 0.15.1 (x)'; exit 0;; esac\n" +
		"root=\"\"\n" +
		"prev=\"\"\n" +
		"for a in \"$@\"; do\n" +
		"  if [ \"$prev\" = \"--root\" ]; then root=\"$a\"; fi\n" +
		"  /bin/echo \"$a\" >> \"" + argsPath + "\"\n" +
		"  prev=\"$a\"\n" +
		"done\n" +
		// Copy report.json out while the work folder still exists: the
		// command removes it once the run finishes, so this is the only
		// window the test can observe it in.
		"/bin/cp \"$root/report.json\" \"" + reportJSONCopy + "\"\n" +
		"last=\"\"\n" +
		"for a in \"$@\"; do last=\"$a\"; done\n" +
		"/bin/echo '%PDF-fake' > \"$last\"\n" +
		"exit 0\n"
	binary := writeScript(t, script)
	env := envWithTypst(testEnv(t), binary)

	_, stderr, code := runArgs(context.Background(), env, []string{"report", "--config", cfgPath, "--output", filepath.Join(outDir, "r.pdf")})
	if code != 0 {
		t.Fatalf("code = %d, want 0; stderr = %q", code, stderr)
	}

	data, err := os.ReadFile(argsPath)
	if err != nil {
		t.Fatalf("reading recorded args: %v", err)
	}
	args := strings.Split(strings.TrimRight(string(data), "\n"), "\n")

	rootIdx := indexOf(args, "--root")
	if rootIdx == -1 || rootIdx+1 >= len(args) {
		t.Fatalf("--root not found in %v", args)
	}
	root := args[rootIdx+1]

	if !contains(args, "--input") {
		t.Fatalf("--input not found in %v", args)
	}
	inputIdx := indexOf(args, "--input")
	if args[inputIdx+1] != "data=/report.json" {
		t.Fatalf("--input value = %q, want %q", args[inputIdx+1], "data=/report.json")
	}
	if _, err := os.Stat(reportJSONCopy); err != nil {
		t.Fatalf("report.json was not readable at %s/report.json during the compile: %v", root, err)
	}

	fontIdx := indexOf(args, "--font-path")
	if fontIdx == -1 || fontIdx+1 >= len(args) {
		t.Fatalf("--font-path not found in %v", args)
	}
	fontPath := args[fontIdx+1]
	if !strings.HasPrefix(fontPath, root) {
		t.Fatalf("--font-path %q is not inside the root %q", fontPath, root)
	}
}

func indexOf(items []string, value string) int {
	for i, v := range items {
		if v == value {
			return i
		}
	}
	return -1
}

func contains(items []string, value string) bool {
	return indexOf(items, value) != -1
}

// recordingWriter records the byte slice of each Write call it receives.
type recordingWriter struct {
	mu    sync.Mutex
	calls [][]byte
}

func (w *recordingWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	cp := append([]byte{}, p...)
	w.calls = append(w.calls, cp)
	return len(p), nil
}

// assertOneLinePerCall requires every recorded call to end in exactly one
// line break and hold exactly one line.
func assertOneLinePerCall(t *testing.T, label string, w *recordingWriter) {
	t.Helper()
	for i, call := range w.calls {
		s := string(call)
		if !strings.HasSuffix(s, "\n") {
			t.Errorf("%s call %d = %q, does not end in a line break", label, i, s)
			continue
		}
		if strings.Count(s, "\n") != 1 {
			t.Errorf("%s call %d = %q, holds more than one line", label, i, s)
		}
	}
}

func TestEachLineIsOneWrite(t *testing.T) {
	t.Run("check mode", func(t *testing.T) {
		server, _, scenario := startScenario(t, "direct-complete")
		defer server.Close()
		cfgPath := writeConfig(t, scenarioConfig(server, scenario, t.TempDir()))

		var stdout, stderr recordingWriter
		code := Run(context.Background(), testEnv(t), []string{"report", "--config", cfgPath, "--check", "--verbose"}, &stdout, &stderr)
		if code != 0 {
			t.Fatalf("code = %d, want 0", code)
		}
		assertOneLinePerCall(t, "stdout", &stdout)
		assertOneLinePerCall(t, "stderr", &stderr)
	})

	t.Run("failing run", func(t *testing.T) {
		var stdout, stderr recordingWriter
		code := Run(context.Background(), testEnv(t), []string{"report", "--check", "--snapshot", "s.json"}, &stdout, &stderr)
		if code != 2 {
			t.Fatalf("code = %d, want 2", code)
		}
		assertOneLinePerCall(t, "stdout", &stdout)
		assertOneLinePerCall(t, "stderr", &stderr)
	})

	t.Run("cobra flag error", func(t *testing.T) {
		var stdout, stderr recordingWriter
		code := Run(context.Background(), testEnv(t), []string{"report", "--token=s3cr3t-value", "--nope"}, &stdout, &stderr)
		if code != 2 {
			t.Fatalf("code = %d, want 2", code)
		}
		assertOneLinePerCall(t, "stdout", &stdout)
		assertOneLinePerCall(t, "stderr", &stderr)
	})

	t.Run("report refuses a positional argument", func(t *testing.T) {
		var stdout, stderr recordingWriter
		code := Run(context.Background(), testEnv(t), []string{"report", "s3cr3t-value", "--check"}, &stdout, &stderr)
		if code != 2 {
			t.Fatalf("code = %d, want 2", code)
		}
		assertOneLinePerCall(t, "stdout", &stdout)
		assertOneLinePerCall(t, "stderr", &stderr)
	})
}

// TestCheckSendsNoRangeQuery proves --check sends no request to
// /query_range, still prints exactly the preflight lines it printed before
// this part, and sends exactly the requests of the preflight: the same
// requests, one by one and in order, by path and by the text of the query,
// as the first requests of a --snapshot run of the same scenario.
//
// For direct-complete the check sends 29 requests: the connect probe, the
// scope query, and the probes of the nine capabilities. The run sends 40
// more, 69 in all: 14 of the namespace and workload collectors, 7 of the
// HPA and coverage collector, 13 of the node collector, and 6 of the
// cluster autoscaling collector.
func TestCheckSendsNoRangeQuery(t *testing.T) {
	checkServer, checkHandler, checkScenario := startScenario(t, "direct-complete")
	defer checkServer.Close()

	checkCfgPath := writeConfig(t, scenarioConfig(checkServer, checkScenario, t.TempDir()))
	stdout, stderr, code := runArgs(context.Background(), testEnv(t), []string{"report", "--config", checkCfgPath, "--check"})
	if code != 0 {
		t.Fatalf("check: code = %d, want 0; stderr = %q", code, stderr)
	}
	checkSeen := checkHandler.Seen()
	if len(checkSeen) != 29 {
		t.Fatalf("check sent %d requests, want 29", len(checkSeen))
	}

	for _, seen := range checkSeen {
		if strings.HasSuffix(seen.Path, "query_range") {
			t.Fatalf("check mode made a request to %s", seen.Path)
		}
	}

	want := strings.Join(directCompletePreflightLines(), "\n") + "\n"
	if stdout != want {
		t.Fatalf("stdout =\n%s\nwant\n%s", stdout, want)
	}

	snapServer, snapHandler, snapScenario := startScenario(t, "direct-complete")
	defer snapServer.Close()

	snapCfgPath := writeConfig(t, scenarioConfig(snapServer, snapScenario, t.TempDir()))
	snapPath := filepath.Join(t.TempDir(), "s.json")
	_, stderr, code = runArgs(context.Background(), testEnv(t), []string{"report", "--config", snapCfgPath, "--snapshot", snapPath})
	if code != 0 {
		t.Fatalf("snapshot: code = %d, want 0; stderr = %q", code, stderr)
	}
	snapSeen := snapHandler.Seen()

	if len(snapSeen) != 69 {
		t.Fatalf("snapshot sent %d requests, want 69 (the check's 29, plus 40 of the collectors)", len(snapSeen))
	}
	for i, wantSeen := range checkSeen {
		gotSeen := snapSeen[i]
		if gotSeen.Path != wantSeen.Path || gotSeen.Query != wantSeen.Query {
			t.Fatalf("request %d: snapshot sent %s %q, check sent %s %q", i, gotSeen.Path, gotSeen.Query, wantSeen.Path, wantSeen.Query)
		}
	}
}

// TestSnapshotRunSendsTheCollectorQueries proves a --snapshot run of
// direct-complete sends 69 requests, and a --check run 29. The 29 are the
// connect probe, the scope query, and the probes of the nine capabilities.
// The 40 of the collectors are 14 of the namespace and workload collectors,
// 7 of the HPA and coverage collector, 13 of the node collector, and 6 of
// the cluster autoscaling collector.
func TestSnapshotRunSendsTheCollectorQueries(t *testing.T) {
	baseServer, baseHandler, baseScenario := startScenario(t, "direct-complete")
	defer baseServer.Close()
	baseCfg := writeConfig(t, scenarioConfig(baseServer, baseScenario, t.TempDir()))
	_, stderr, code := runArgs(context.Background(), testEnv(t), []string{"report", "--config", baseCfg, "--check"})
	if code != 0 {
		t.Fatalf("check: code = %d, want 0; stderr = %q", code, stderr)
	}
	if got := len(baseHandler.Seen()); got != 29 {
		t.Fatalf("check requests = %d, want 29", got)
	}

	server, handler, scenario := startScenario(t, "direct-complete")
	defer server.Close()
	cfgPath := writeConfig(t, scenarioConfig(server, scenario, t.TempDir()))
	snapPath := filepath.Join(t.TempDir(), "s.json")

	_, stderr, code = runArgs(context.Background(), testEnv(t), []string{"report", "--config", cfgPath, "--snapshot", snapPath})
	if code != 0 {
		t.Fatalf("snapshot: code = %d, want 0; stderr = %q", code, stderr)
	}

	if got := len(handler.Seen()); got != 69 {
		t.Fatalf("requests = %d, want 69 (29 of the preflight, 14 of namespaces and workloads, 7 of HPAs, 13 of nodes, 6 of autoscaling)", got)
	}
}

// TestRunPrintsCollectionLines proves a --snapshot run of raw-fallback
// prints the two collection lines, in order, after every capability line.
func TestRunPrintsCollectionLines(t *testing.T) {
	server, _, scenario := startScenario(t, "raw-fallback")
	defer server.Close()

	cfgPath := writeConfig(t, scenarioConfig(server, scenario, t.TempDir()))
	snapPath := filepath.Join(t.TempDir(), "s.json")

	stdout, stderr, code := runArgs(context.Background(), testEnv(t), []string{"report", "--config", cfgPath, "--snapshot", snapPath})
	if code != 0 {
		t.Fatalf("code = %d, want 0; stderr = %q", code, stderr)
	}

	capIdx := strings.Index(stdout, "capability: discovered")
	nsIdx := strings.Index(stdout, "namespace collection: available")
	wlIdx := strings.Index(stdout, "workload collection: partial")
	if capIdx == -1 || nsIdx == -1 || wlIdx == -1 {
		t.Fatalf("stdout = %q, want it to hold the capability header and the two collection lines", stdout)
	}
	if capIdx >= nsIdx || nsIdx >= wlIdx {
		t.Fatalf("stdout = %q, the collection lines are not in order after the capability lines", stdout)
	}
}

// TestCheckPrintsNoCollectionLine proves --check prints neither collection
// line: it never runs a collector.
func TestCheckPrintsNoCollectionLine(t *testing.T) {
	server, _, scenario := startScenario(t, "direct-complete")
	defer server.Close()

	cfgPath := writeConfig(t, scenarioConfig(server, scenario, t.TempDir()))
	stdout, stderr, code := runArgs(context.Background(), testEnv(t), []string{"report", "--config", cfgPath, "--check"})
	if code != 0 {
		t.Fatalf("code = %d, want 0; stderr = %q", code, stderr)
	}
	if strings.Contains(stdout, "namespace collection:") || strings.Contains(stdout, "workload collection:") {
		t.Fatalf("stdout = %q, want no collection line", stdout)
	}
}

// emptyPeriodConfig is the configuration text every mode of
// TestEmptyPeriodStopsBeforeAnyRequest shares: a period the zone's own
// spring-forward gap, at the given clock, empties.
func emptyPeriodConfig(server *httptest.Server, scenario promtest.Scenario, outDir string) string {
	return fmt.Sprintf(`[metrics]
url = %q
endpoint_type = %q

[cluster]
name = "prod"
namespaces = ["payments"]

[analysis]
period = "1h"
timezone = "Europe/Berlin"
node_autoscaling = %q

[report]
output_directory = %q
`, server.URL+scenario.APIRoot, scenario.EndpointType, scenario.NodeAutoscaling, outDir)
}

// TestEmptyPeriodStopsBeforeAnyRequest proves that a period the zone's own
// spring-forward gap empties stops every mode with exit code 2, one line
// naming analysis.period, nothing on standard output, no request, and no
// file written: the window is built before the preflight, in every mode,
// check included. The mode that writes a PDF also points KCR_TYPST at a
// compiler that does not exist, so the test itself fails, with exit code 6
// and a different message, if the window ever moved back behind the
// compiler being looked for.
func TestEmptyPeriodStopsBeforeAnyRequest(t *testing.T) {
	loc, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		t.Fatalf("loading Europe/Berlin: %v", err)
	}
	clock := time.Date(2026, time.March, 29, 3, 30, 0, 0, loc)

	tests := []struct {
		name string
		args func(cfgPath, outDir string) []string
	}{
		{"check", func(cfgPath, _ string) []string {
			return []string{"report", "--config", cfgPath, "--check"}
		}},
		{"snapshot", func(cfgPath, outDir string) []string {
			return []string{"report", "--config", cfgPath, "--snapshot", filepath.Join(outDir, "s.json")}
		}},
		{"pdf", func(cfgPath, outDir string) []string {
			return []string{"report", "--config", cfgPath, "--output", filepath.Join(outDir, "r.pdf")}
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server, handler, scenario := startScenario(t, "direct-complete")
			defer server.Close()

			outDir := t.TempDir()
			cfgPath := writeConfig(t, emptyPeriodConfig(server, scenario, outDir))

			env := testEnv(t)
			env.Now = func() time.Time { return clock }
			if tt.name == "pdf" {
				env = envWithTypst(env, filepath.Join(outDir, "no-such-compiler"))
			}

			stdout, stderr, code := runArgs(context.Background(), env, tt.args(cfgPath, outDir))
			if code != 2 {
				t.Fatalf("code = %d, want 2; stderr = %q", code, stderr)
			}
			if !strings.HasPrefix(stderr, "error: analysis.period: ") {
				t.Fatalf("stderr = %q, want it to start with %q", stderr, "error: analysis.period: ")
			}
			if strings.Count(stderr, "\n") != 1 {
				t.Fatalf("stderr = %q, want exactly one line", stderr)
			}
			if stdout != "" {
				t.Fatalf("stdout = %q, want empty", stdout)
			}
			if len(handler.Seen()) != 0 {
				t.Fatalf("the server saw %d requests, want 0", len(handler.Seen()))
			}

			entries, err := os.ReadDir(outDir)
			if err != nil {
				t.Fatalf("reading %s: %v", outDir, err)
			}
			if len(entries) != 0 {
				var names []string
				for _, e := range entries {
					names = append(names, e.Name())
				}
				t.Fatalf("%s holds %v, want no file written", outDir, names)
			}
		})
	}
}

// TestCollectorFailureStopsWithThree proves that a range query the server
// always answers with an HTTP 500 exhausts the client's three attempts and
// stops the run with exit code 3, writing no snapshot and leaving no
// temporary file beside it: the same classification an error of the client
// already gets everywhere else.
func TestCollectorFailureStopsWithThree(t *testing.T) {
	var scenario promtest.Scenario
	for _, s := range promtest.Scenarios {
		if s.Name == "direct-complete" {
			scenario = s
		}
	}

	path, err := promtest.Path("direct-complete")
	if err != nil {
		t.Fatalf("promtest.Path: %v", err)
	}
	fixture, err := promtest.Load(path)
	if err != nil {
		t.Fatalf("promtest.Load: %v", err)
	}
	handler := promtest.NewServer(fixture)

	var rangeAttempts int
	mux := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "query_range") {
			rangeAttempts++
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = fmt.Fprint(w, `{"status":"error","errorType":"internal","error":"boom"}`)
			return
		}
		handler.ServeHTTP(w, r)
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	outDir := t.TempDir()
	cfgPath := writeConfig(t, scenarioConfig(server, scenario, outDir))
	snapPath := filepath.Join(t.TempDir(), "s.json")

	env := testEnv(t)
	_, stderr, code := runArgs(context.Background(), env, []string{"report", "--config", cfgPath, "--snapshot", snapPath})
	if code != 3 {
		t.Fatalf("code = %d, want 3; stderr = %q", code, stderr)
	}
	if rangeAttempts != 3 {
		t.Fatalf("range attempts = %d, want 3", rangeAttempts)
	}
	if _, err := os.Stat(snapPath); err == nil {
		t.Fatalf("%s was written, want no file", snapPath)
	}
	assertNoTempFilesBeside(t, filepath.Dir(snapPath))
}

// TestCommandStopsOnASlash proves that a workload name holding a slash,
// answered by the metrics endpoint, stops the run with exit code 3, one
// line on standard error, and no snapshot file: the collector refuses it
// before any row is keyed. The fixture of "direct-complete" is loaded and
// changed in memory only, its one answer naming the workload "payments-web"
// rewritten to "b/c"; the scenario file on disk is never touched.
func TestCommandStopsOnASlash(t *testing.T) {
	var scenario promtest.Scenario
	for _, s := range promtest.Scenarios {
		if s.Name == "direct-complete" {
			scenario = s
		}
	}

	path, err := promtest.Path("direct-complete")
	if err != nil {
		t.Fatalf("promtest.Path: %v", err)
	}
	fixture, err := promtest.Load(path)
	if err != nil {
		t.Fatalf("promtest.Load: %v", err)
	}

	rewritten := false
	for i, req := range fixture.Requests {
		if !strings.Contains(req.QueryContains, "container_cpu_usage_seconds_total") {
			continue
		}
		if !bytes.Contains(req.Body, []byte(`"payments-web"`)) {
			continue
		}
		fixture.Requests[i].Body = bytes.ReplaceAll(req.Body, []byte(`"payments-web"`), []byte(`"b/c"`))
		rewritten = true
		break
	}
	if !rewritten {
		t.Fatal("fixture direct-complete has no CPU usage answer naming payments-web, want one to rewrite")
	}

	handler := promtest.NewServer(fixture)
	server := httptest.NewServer(handler)
	defer server.Close()

	outDir := t.TempDir()
	cfgPath := writeConfig(t, scenarioConfig(server, scenario, outDir))
	snapPath := filepath.Join(t.TempDir(), "s.json")

	env := testEnv(t)
	_, stderr, code := runArgs(context.Background(), env, []string{"report", "--config", cfgPath, "--snapshot", snapPath})
	if code != 3 {
		t.Fatalf("code = %d, want 3; stderr = %q", code, stderr)
	}
	if strings.Count(stderr, "\n") != 1 {
		t.Fatalf("stderr = %q, want exactly one line", stderr)
	}
	if _, err := os.Stat(snapPath); err == nil {
		t.Fatalf("%s was written, want no file", snapPath)
	}
	assertNoTempFilesBeside(t, filepath.Dir(snapPath))
}

// TestFromSnapshotRefusesASnapshotOfThePartBefore proves that a snapshot
// stored before this part, which never held quality.warnings or
// quality.notices, is refused with exit code 2 and a message naming the
// missing key: it is read straight from internal/snapshot's own fixture,
// never copied.
func TestFromSnapshotRefusesASnapshotOfThePartBefore(t *testing.T) {
	snapPath, err := filepath.Abs(filepath.Join("..", "snapshot", "testdata", "schema-1.0-before-rows.json"))
	if err != nil {
		t.Fatalf("resolving path: %v", err)
	}

	outPath := filepath.Join(t.TempDir(), "r.pdf")
	_, stderr, code := runArgs(context.Background(), testEnv(t), []string{"report", "--from-snapshot", snapPath, "--lang", "en", "--output", outPath})
	if code != 2 {
		t.Fatalf("code = %d, want 2; stderr = %q", code, stderr)
	}
	if !strings.Contains(stderr, "quality.warnings") {
		t.Fatalf("stderr = %q, want it to name quality.warnings", stderr)
	}
}

// clearCollectionTimes blanks the two collection-time fields of s in
// place: they are the only fields of a snapshot that come from a clock
// rather than from configuration or from the metrics endpoint, so a
// comparison that means to check everything else sets them equal first.
func clearCollectionTimes(s *snapshot.Snapshot) {
	s.Quality.CollectionStartedAt = ""
	s.Quality.CollectionCompletedAt = ""
}

// TestGoldenFlowEqualsTheCommand proves that the command, run with the
// fixed clock and identifier internal/goldentest also uses, and with the
// same organization, classification, and author given through its own
// configuration file, produces the same snapshot internal/goldentest
// builds for the same scenario, for every scenario of promtest.Scenarios.
func TestGoldenFlowEqualsTheCommand(t *testing.T) {
	for _, scenario := range promtest.Scenarios {
		t.Run(scenario.Name, func(t *testing.T) {
			server, _, sc := startScenario(t, scenario.Name)
			defer server.Close()

			outDir := t.TempDir()
			cfgPath := writeConfig(t, fixedGoldenConfig(server, sc, outDir))
			snapPath := filepath.Join(t.TempDir(), "s.json")

			args := scenarioArgs(cfgPath, sc, "--snapshot", snapPath)
			_, stderr, code := runArgs(context.Background(), fixedGoldenEnv(t), args)
			if code != 0 {
				t.Fatalf("code = %d, want 0; stderr = %q", code, stderr)
			}

			gotBytes, err := os.ReadFile(snapPath)
			if err != nil {
				t.Fatalf("reading %s: %v", snapPath, err)
			}
			wantBytes, err := goldentest.Snapshot(context.Background(), sc)
			if err != nil {
				t.Fatalf("goldentest.Snapshot(%s): %v", sc.Name, err)
			}

			got, err := snapshot.Decode(gotBytes)
			if err != nil {
				t.Fatalf("decoding the command's snapshot: %v", err)
			}
			want, err := snapshot.Decode(wantBytes)
			if err != nil {
				t.Fatalf("decoding goldentest's snapshot: %v", err)
			}
			clearCollectionTimes(&got)
			clearCollectionTimes(&want)

			if !reflect.DeepEqual(got, want) {
				t.Fatalf("the command's snapshot for %s differs from goldentest's", scenario.Name)
			}
		})
	}
}

// preflightRequests is the number of requests of the preflight of
// direct-complete: the connect probe, the scope query, and the probes of
// the nine capabilities.
const preflightRequests = 29

// emptyCollectorsServer answers the preflight requests of direct-complete
// as recorded, except the probe of the cluster autoscaler metrics, which
// answers nothing, and every later request with an empty result. It counts
// the requests it receives after the preflight.
func emptyCollectorsServer(t *testing.T) (*httptest.Server, promtest.Scenario, *int) {
	t.Helper()
	var scenario promtest.Scenario
	for _, s := range promtest.Scenarios {
		if s.Name == "direct-complete" {
			scenario = s
		}
	}
	path, err := promtest.Path("direct-complete")
	if err != nil {
		t.Fatalf("promtest.Path: %v", err)
	}
	fixture, err := promtest.Load(path)
	if err != nil {
		t.Fatalf("promtest.Load: %v", err)
	}
	recorded := promtest.NewServer(fixture)

	var mu sync.Mutex
	received := 0
	collectorRequests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		mu.Lock()
		received++
		inPreflight := received <= preflightRequests
		if !inPreflight {
			collectorRequests++
		}
		mu.Unlock()

		if inPreflight && !strings.Contains(r.PostFormValue("query"), "cluster_autoscaler_") {
			recorded.ServeHTTP(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		resultType := "vector"
		if strings.HasSuffix(r.URL.Path, "query_range") {
			resultType = "matrix"
		}
		_, _ = fmt.Fprintf(w, `{"status":"success","data":{"resultType":%q,"result":[]}}`, resultType)
	}))
	t.Cleanup(server.Close)
	return server, scenario, &collectorRequests
}

// TestNoUsefulSectionsExitsFive proves a run whose collectors find nothing
// stops with exit code 5, one line, no output, and no file. With every
// capability available and the recording rule, the collectors send 14 of
// namespaces and workloads, 7 of HPAs, 13 of nodes, and 6 of autoscaling.
func TestNoUsefulSectionsExitsFive(t *testing.T) {
	for _, mode := range []string{"snapshot", "pdf"} {
		t.Run(mode, func(t *testing.T) {
			server, scenario, collectorRequests := emptyCollectorsServer(t)
			outDir := t.TempDir()
			cfgPath := writeConfig(t, scenarioConfig(server, scenario, outDir))
			target := filepath.Join(t.TempDir(), "out.json")
			args := []string{"report", "--config", cfgPath, "--snapshot", target}
			env := testEnv(t)
			if mode == "pdf" {
				target = filepath.Join(t.TempDir(), "out.pdf")
				args = []string{"report", "--config", cfgPath, "--output", target}
				env = withTypst(t, env)
			}

			stdout, stderr, code := runArgs(context.Background(), env, args)
			if code != 5 {
				t.Fatalf("code = %d, want 5; stderr = %q", code, stderr)
			}
			if stderr != "error: no useful report sections were collected\n" {
				t.Fatalf("stderr = %q, want the one line", stderr)
			}
			if stdout != "" {
				t.Fatalf("stdout = %q, want empty", stdout)
			}
			if _, err := os.Stat(target); err == nil {
				t.Fatalf("%s was written, want no file", target)
			}
			assertNoTempFilesBeside(t, filepath.Dir(target))
			if *collectorRequests != 40 {
				t.Fatalf("collector requests = %d, want 40", *collectorRequests)
			}
		})
	}
}

// TestNoUsefulSectionsSavedByConfiguredAutoscaling proves a configured
// autoscaling setting is a useful section by itself.
func TestNoUsefulSectionsSavedByConfiguredAutoscaling(t *testing.T) {
	server, scenario, _ := emptyCollectorsServer(t)
	cfgPath := writeConfig(t, scenarioConfig(server, scenario, t.TempDir()))
	target := filepath.Join(t.TempDir(), "s.json")

	stdout, stderr, code := runArgs(context.Background(), testEnv(t), []string{"report", "--config", cfgPath, "--snapshot", target, "--node-autoscaling", "disabled"})
	if code != 0 {
		t.Fatalf("code = %d, want 0; stderr = %q", code, stderr)
	}
	if !strings.Contains(stdout, "autoscaling collection: disabled\n") {
		t.Fatalf("stdout = %q, want the autoscaling line", stdout)
	}
	if _, err := os.Stat(target); err != nil {
		t.Fatalf("snapshot not written: %v", err)
	}
}

// TestRunPrintsTheCollectionLines proves a --snapshot run of
// direct-complete prints the six collection lines, in order, before the
// final line.
func TestRunPrintsTheCollectionLines(t *testing.T) {
	server, _, scenario := startScenario(t, "direct-complete")
	defer server.Close()
	cfgPath := writeConfig(t, scenarioConfig(server, scenario, t.TempDir()))
	snapPath := filepath.Join(t.TempDir(), "s.json")

	stdout, stderr, code := runArgs(context.Background(), testEnv(t), []string{"report", "--config", cfgPath, "--snapshot", snapPath})
	if code != 0 {
		t.Fatalf("code = %d, want 0; stderr = %q", code, stderr)
	}
	lines := strings.Split(strings.TrimSuffix(stdout, "\n"), "\n")
	got := lines[len(lines)-7:]
	want := []string{
		"namespace collection: available",
		"workload collection: available",
		"hpa collection: available",
		"hpa coverage collection: available",
		"node collection: available",
		"autoscaling collection: enabled",
		"snapshot: " + snapPath,
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("last lines =\n%q\nwant\n%q", got, want)
	}
}
