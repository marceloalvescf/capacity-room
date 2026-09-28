package cli

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
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

func TestModeConflicts(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
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
		if name == "insecure" {
			flagArgs = append(flagArgs, "--insecure")
		} else {
			flagArgs = append(flagArgs, "--"+name, "x")
		}
		tests = append(tests, struct {
			name string
			args []string
			want string
		}{
			"from-snapshot with " + name,
			flagArgs,
			"--from-snapshot cannot be combined with --" + name,
		})
	}

	for _, tt := range tests {
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

func TestExecutiveIsRefused(t *testing.T) {
	wantMsg := `error: report profile "executive" is not available in this version` + "\n"

	t.Run("flag", func(t *testing.T) {
		cfgPath := writeConfig(t, minimalConfig(t.TempDir()))
		env := withTypst(t, testEnv(t))
		_, stderr, code := runArgs(context.Background(), env, []string{"report", "--config", cfgPath, "--report-profile", "executive"})
		if code != 2 {
			t.Fatalf("code = %d, want 2; stderr = %q", code, stderr)
		}
		if stderr != wantMsg {
			t.Fatalf("stderr = %q, want %q", stderr, wantMsg)
		}
	})

	t.Run("configuration file", func(t *testing.T) {
		text := fmt.Sprintf(`[metrics]
url = "http://127.0.0.1:1/api/v1"

[cluster]
name = "prod"
namespaces = ["payments"]

[report]
output_directory = %q
profile = "executive"
`, t.TempDir())
		cfgPath := writeConfig(t, text)
		env := withTypst(t, testEnv(t))
		_, stderr, code := runArgs(context.Background(), env, []string{"report", "--config", cfgPath})
		if code != 2 {
			t.Fatalf("code = %d, want 2; stderr = %q", code, stderr)
		}
		if stderr != wantMsg {
			t.Fatalf("stderr = %q, want %q", stderr, wantMsg)
		}
	})

	t.Run("environment", func(t *testing.T) {
		cfgPath := writeConfig(t, minimalConfig(t.TempDir()))
		env := withTypst(t, testEnv(t))
		env.Getenv = func(k string) string {
			if k == "KCR_REPORT_PROFILE" {
				return "executive"
			}
			return ""
		}
		env = withTypst(t, env)
		_, stderr, code := runArgs(context.Background(), env, []string{"report", "--config", cfgPath})
		if code != 2 {
			t.Fatalf("code = %d, want 2; stderr = %q", code, stderr)
		}
		if stderr != wantMsg {
			t.Fatalf("stderr = %q, want %q", stderr, wantMsg)
		}
	})

	t.Run("from snapshot, flag", func(t *testing.T) {
		snapPath, err := snapshotFixturePath(t, "direct-complete")
		if err != nil {
			t.Fatalf("snapshotFixturePath: %v", err)
		}
		env := withTypst(t, testEnv(t))
		_, stderr, code := runArgs(context.Background(), env, []string{"report", "--from-snapshot", snapPath, "--report-profile", "executive"})
		if code != 2 {
			t.Fatalf("code = %d, want 2; stderr = %q", code, stderr)
		}
		if stderr != wantMsg {
			t.Fatalf("stderr = %q, want %q", stderr, wantMsg)
		}
	})

	t.Run("from snapshot, environment", func(t *testing.T) {
		snapPath, err := snapshotFixturePath(t, "direct-complete")
		if err != nil {
			t.Fatalf("snapshotFixturePath: %v", err)
		}
		env := testEnv(t)
		env.Getenv = func(k string) string {
			if k == "KCR_REPORT_PROFILE" {
				return "executive"
			}
			return ""
		}
		env = withTypst(t, env)
		_, stderr, code := runArgs(context.Background(), env, []string{"report", "--from-snapshot", snapPath})
		if code != 2 {
			t.Fatalf("code = %d, want 2; stderr = %q", code, stderr)
		}
		if stderr != wantMsg {
			t.Fatalf("stderr = %q, want %q", stderr, wantMsg)
		}
	})
}

// snapshotFixturePath returns the path of a committed golden snapshot; every
// TestExecutiveIsRefused/from-snapshot case only needs a valid, decodable
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

func TestExitCodes(t *testing.T) {
	type row struct {
		name string
		want int
		run  func(t *testing.T) (stdout, stderr string, code int)
	}

	rows := []row{
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

	for _, r := range rows {
		t.Run(r.name, func(t *testing.T) {
			stdout, stderr, code := r.run(t)
			if code != r.want {
				t.Fatalf("code = %d, want %d; stderr = %q", code, r.want, stderr)
			}
			if !strings.HasPrefix(stderr, "error: ") {
				t.Fatalf("stderr = %q, want it to start with %q", stderr, "error: ")
			}
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
}
