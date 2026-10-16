package cli

import (
	"bytes"
	"context"
	"fmt"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/marceloalvescf/capacity-room/internal/promtest"
)

// toolPath returns the absolute path of the named program, found on the
// test process's PATH. A script run with an empty environment cannot search
// for it, so it names it in full.
func toolPath(t *testing.T, name string) string {
	t.Helper()
	path, err := exec.LookPath(name)
	if err != nil {
		t.Fatalf("finding %s on PATH: %v", name, err)
	}
	return path
}

// fakeTypstPath writes the fake typst compiler used by every test that needs
// one into a fresh temp directory and returns its absolute path. The
// compiler is run with an empty environment (internal/typst always clears
// it), so the script names cat by its full path.
//
// --version answers with the pinned version. Any "compile" invocation copies
// the file named by "--input data=<path>" (interpreted relative to --root) to
// its last argument, after writing the line "%PDF-fake" first.
func fakeTypstPath(t *testing.T) string {
	t.Helper()
	script := `#!/bin/sh
if [ "$1" = "--version" ]; then
	echo 'typst 0.15.1 (fake)'
	exit 0
fi

root=""
data_value=""
prev=""
last=""
for arg in "$@"; do
	if [ "$prev" = "--root" ]; then
		root="$arg"
	fi
	if [ "$prev" = "--input" ]; then
		case "$arg" in
		data=*)
			data_value="${arg#data=}"
			;;
		esac
	fi
	prev="$arg"
	last="$arg"
done

output="$last"
echo '%PDF-fake' >"$output"
` + toolPath(t, "cat") + ` "$root$data_value" >>"$output"
`
	return writeScript(t, script)
}

// noTypstLookPath simulates a machine with no typst on PATH.
func noTypstLookPath(string) (string, error) {
	return "", fmt.Errorf("not found")
}

// envFunc turns a map into the function shape Env.Getenv and config.Load
// expect.
func envFunc(vars map[string]string) func(string) string {
	return func(k string) string { return vars[k] }
}

// sequentialClock returns a func() time.Time that returns each of times in
// order, and repeats the last one if called more times than times holds.
func sequentialClock(times ...time.Time) func() time.Time {
	i := 0
	return func() time.Time {
		t := times[i]
		if i < len(times)-1 {
			i++
		}
		return t
	}
}

// fixedID returns an Env.NewID that always answers id.
func fixedID(id string) func() (string, error) {
	return func() (string, error) { return id, nil }
}

// testEnv builds an Env with every field usable by default: no environment
// variables, no typst on PATH, a fixed clock starting at base and advancing
// by 42s on the second call, a fixed report identifier, and TempDir scoped
// to the test.
func testEnv(t *testing.T) Env {
	t.Helper()
	base := time.Date(2026, time.July, 15, 14, 30, 0, 0, time.UTC)
	return Env{
		Getenv:   envFunc(nil),
		LookPath: noTypstLookPath,
		Now:      sequentialClock(base, base.Add(42*time.Second)),
		NewID:    fixedID("00000000-0000-4000-8000-000000000099"),
		TempDir:  t.TempDir(),
		Version:  "0.0.0-test",
	}
}

// withTypst returns a copy of env whose Getenv also answers CAPACITYROOM_TYPST with
// the fake compiler.
func withTypst(t *testing.T, env Env) Env {
	t.Helper()
	path := fakeTypstPath(t)
	inner := env.Getenv
	env.Getenv = func(k string) string {
		if k == "CAPACITYROOM_TYPST" {
			return path
		}
		return inner(k)
	}
	return env
}

// startScenario serves scenario's fixture over HTTP and returns the HTTP
// server, the promtest.Server behind it (to inspect which requests it
// received), and the scenario. The caller must Close the server. The fake
// server answers each scripted request once, so a fresh server is needed
// for every run that collects.
func startScenario(t *testing.T, name string) (*httptest.Server, *promtest.Server, promtest.Scenario) {
	t.Helper()
	var scenario promtest.Scenario
	found := false
	for _, s := range promtest.Scenarios {
		if s.Name == name {
			scenario = s
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("no scenario named %q", name)
	}

	fixture, err := promtest.LoadScenario(name)
	if err != nil {
		t.Fatalf("promtest.LoadScenario(%s): %v", name, err)
	}
	handler := promtest.NewServer(fixture)
	server := httptest.NewServer(handler)
	return server, handler, scenario
}

// writeConfig writes text to report.toml in a fresh temp folder and returns
// its path.
func writeConfig(t *testing.T, text string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "report.toml")
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatalf("writing %s: %v", path, err)
	}
	return path
}

// scenarioConfig builds the minimal valid TOML configuration for scenario,
// pointed at server, with the given output directory. tokenEnv, when not
// empty, names the environment variable metrics.token_env points at.
func scenarioConfig(server *httptest.Server, scenario promtest.Scenario, outputDir string, tokenEnv ...string) string {
	tokenLine := ""
	if len(tokenEnv) > 0 && tokenEnv[0] != "" {
		tokenLine = fmt.Sprintf("token_env = %q\n", tokenEnv[0])
	}
	return fmt.Sprintf(`[metrics]
url = %q
endpoint_type = %q
%s
[cluster]
name = "prod"
namespaces = ["payments"]

[analysis]
node_autoscaling = %q

[report]
output_directory = %q
`, server.URL+scenario.APIRoot, scenario.EndpointType, tokenLine, scenario.NodeAutoscaling, outputDir)
}

// runArgs runs the command against args with ctx and returns its stdout,
// stderr, and exit code.
func runArgs(ctx context.Context, env Env, args []string) (stdout, stderr string, code int) {
	var out, errOut bytes.Buffer
	code = Run(ctx, env, args, &out, &errOut)
	return out.String(), errOut.String(), code
}
