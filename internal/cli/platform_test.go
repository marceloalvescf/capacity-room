package cli

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/marceloalvescf/k8s-capacity-report/internal/redact"
)

// failingReader fails the test when anything reads from it.
type failingReader struct{ t *testing.T }

func (r failingReader) Read([]byte) (int, error) {
	r.t.Helper()
	r.t.Error("standard input was read")
	return 0, os.ErrClosed
}

// writePlatformFiles writes a settings file and a 32-byte server secret
// under a fresh folder and returns the path of the settings file.
func writePlatformFiles(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	secret := filepath.Join(dir, "secret")
	if err := os.WriteFile(secret, bytes.Repeat([]byte("s"), 32), 0o600); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "platform.toml")
	if err := os.WriteFile(path, []byte("server_secret_path = \""+secret+"\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// platformEnv is an Env whose environment holds vars and whose standard
// input is stdin.
func platformEnv(vars map[string]string, stdin string) Env {
	return Env{
		Getenv: envFunc(vars),
		Now:    time.Now,
		NewID: func() (string, error) {
			id, err := uuid.NewRandom()
			return id.String(), err
		},
		Stdin: strings.NewReader(stdin),
	}
}

// runCommand runs the program and returns its code and both streams.
func runCommand(ctx context.Context, env Env, args ...string) (int, string, string) {
	var stdout, stderr bytes.Buffer
	code := Run(ctx, env, args, &stdout, &stderr)
	return code, stdout.String(), stderr.String()
}

const closedPortDSN = "postgres://u:hunter2secret@127.0.0.1:1/x?sslmode=disable&connect_timeout=1"

func TestAdminListsItsCommands(t *testing.T) {
	code, stdout, stderr := runCommand(context.Background(), Env{}, "admin")
	if code != 0 {
		t.Fatalf("code = %d, want 0; stderr = %q", code, stderr)
	}
	for _, name := range []string{"bootstrap-admin", "migrate", "probe"} {
		if !strings.Contains(stdout, name) {
			t.Errorf("help does not name %q: %q", name, stdout)
		}
	}

	code, stdout, stderr = runCommand(context.Background(), Env{}, "admin", "hunter2")
	const want = "error: unknown command; the commands are: bootstrap-admin, migrate, probe\n"
	if code != 2 || stderr != want || stdout != "" {
		t.Fatalf("code = %d, stdout = %q, stderr = %q, want 2, empty, %q", code, stdout, stderr, want)
	}
}

func TestServeRefusesArguments(t *testing.T) {
	code, stdout, stderr := runCommand(context.Background(), Env{}, "serve", "extra")
	const want = "error: serve takes no argument; found 1\n"
	if code != 2 || stderr != want || stdout != "" {
		t.Fatalf("code = %d, stdout = %q, stderr = %q, want 2, empty, %q", code, stdout, stderr, want)
	}
}

func TestServeMissingConfig(t *testing.T) {
	env := platformEnv(map[string]string{"KCR_DATABASE_DSN": closedPortDSN}, "")
	code, stdout, stderr := runCommand(context.Background(), env, "serve", "--config", "nope.toml")
	if code != 2 || stdout != "" || !strings.HasPrefix(stderr, "error: cannot read nope.toml: ") {
		t.Fatalf("code = %d, stdout = %q, stderr = %q", code, stdout, stderr)
	}
}

func TestServeRedactsDSN(t *testing.T) {
	env := platformEnv(map[string]string{"KCR_DATABASE_DSN": closedPortDSN}, "")
	code, stdout, stderr := runCommand(context.Background(), env, "serve", "--config", writePlatformFiles(t))
	if code != 1 || stdout != "" {
		t.Fatalf("code = %d, stdout = %q, stderr = %q, want 1 and empty stdout", code, stdout, stderr)
	}
	if !strings.HasPrefix(stderr, "error: cannot connect to the database: ") || strings.Contains(stderr, "hunter2secret") {
		t.Fatalf("stderr = %q, want the connection message without the password", stderr)
	}
	if strings.Count(stderr, "\n") != 1 {
		t.Fatalf("stderr = %q, want one line", stderr)
	}
}

func TestErrorLineIsOneLine(t *testing.T) {
	// Without sslmode the driver tries TLS and then plain text, and its
	// message names each attempt on a line of its own.
	const dsn = "postgres://u:hunter2secret@127.0.0.1:1/x?connect_timeout=1"
	env := platformEnv(map[string]string{"KCR_DATABASE_DSN": dsn}, "")
	code, stdout, stderr := runCommand(context.Background(), env, "admin", "migrate", "--config", writePlatformFiles(t))
	if code != 1 || stdout != "" {
		t.Fatalf("code = %d, stdout = %q, stderr = %q, want 1 and empty stdout", code, stdout, stderr)
	}
	if !strings.HasPrefix(stderr, "error: cannot connect to the database: ") || !strings.HasSuffix(stderr, "\n") ||
		strings.Count(stderr, "\n") != 1 || strings.ContainsAny(stderr, "\t\r") || strings.Contains(stderr, "  ") {
		t.Fatalf("stderr = %q, want one line", stderr)
	}
	if strings.Contains(stderr, "hunter2secret") {
		t.Fatalf("stderr = %q holds the password", stderr)
	}
}

func TestOpenPlatformArmsTheRedactor(t *testing.T) {
	env := platformEnv(map[string]string{"KCR_DATABASE_DSN": closedPortDSN}, "")
	redactor := redact.New()
	if _, _, err := openPlatform(context.Background(), env, redactor, writePlatformFiles(t)); err == nil {
		t.Fatal("openPlatform succeeded against a closed port")
	}
	got := redactor.Text(closedPortDSN)
	if !strings.Contains(got, redact.Mark) || strings.Contains(got, "hunter2secret") {
		t.Fatalf("redactor.Text(dsn) = %q, want the mark and no password", got)
	}
}

func TestBootstrapOff(t *testing.T) {
	for _, value := range []string{"", "true"} {
		env := platformEnv(map[string]string{"KCR_BOOTSTRAP_MODE": value}, "")
		env.Stdin = failingReader{t}
		code, stdout, stderr := runCommand(context.Background(), env,
			"admin", "bootstrap-admin", "--username", "root", "--password-stdin")
		const want = "error: bootstrap-admin is off; set KCR_BOOTSTRAP_MODE=1 to run it\n"
		if code != 1 || stderr != want || stdout != "" {
			t.Fatalf("value %q: code = %d, stdout = %q, stderr = %q", value, code, stdout, stderr)
		}
	}
}

func TestBootstrapNeedsStdinFlag(t *testing.T) {
	env := platformEnv(map[string]string{"KCR_BOOTSTRAP_MODE": "1"}, "")
	env.Stdin = failingReader{t}
	code, stdout, stderr := runCommand(context.Background(), env, "admin", "bootstrap-admin", "--username", "root")
	const want = "error: the administrator password is read from standard input; pass --password-stdin and write it there\n"
	if code != 2 || stderr != want || stdout != "" {
		t.Fatalf("code = %d, stdout = %q, stderr = %q", code, stdout, stderr)
	}
}

func TestBootstrapNoPasswordFlag(t *testing.T) {
	env := platformEnv(map[string]string{"KCR_BOOTSTRAP_MODE": "1"}, "")
	code, stdout, stderr := runCommand(context.Background(), env,
		"admin", "bootstrap-admin", "--username", "root", "--password", "xsecretvalue")
	if code != 2 || stdout != "" || stderr != "error: unknown flag: --password\n" {
		t.Fatalf("code = %d, stdout = %q, stderr = %q", code, stdout, stderr)
	}
	if strings.Contains(stdout+stderr, "xsecretvalue") {
		t.Fatalf("a stream held the value: %q %q", stdout, stderr)
	}
}

func TestBootstrapValidatesBeforeDatabase(t *testing.T) {
	env := platformEnv(map[string]string{"KCR_BOOTSTRAP_MODE": "1", "KCR_DATABASE_DSN": closedPortDSN},
		"correct horse battery staple\n")
	start := time.Now()
	code, stdout, stderr := runCommand(context.Background(), env,
		"admin", "bootstrap-admin", "--username", "no spaces allowed", "--password-stdin",
		"--config", writePlatformFiles(t))
	if code != 1 || stdout != "" || stderr != "error: the username is not valid\n" {
		t.Fatalf("code = %d, stdout = %q, stderr = %q", code, stdout, stderr)
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("took %s, want a refusal before any connection", elapsed)
	}
}

func TestBootstrapRefusesBadPasswordAndLanguage(t *testing.T) {
	tests := []struct {
		name  string
		stdin string
		extra []string
		code  int
		want  string
	}{
		{"short", "short\n", nil, 1, "error: the password is too short\n"},
		{"long", strings.Repeat("a", 5000) + "\n", nil, 1, "error: the password is too long\n"},
		{"language", "correct horse battery staple\n", []string{"--language", "fr"}, 2, "error: --language must be pt-BR or en\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := platformEnv(map[string]string{"KCR_BOOTSTRAP_MODE": "1", "KCR_DATABASE_DSN": closedPortDSN}, tt.stdin)
			args := append([]string{"admin", "bootstrap-admin", "--username", "root", "--password-stdin",
				"--config", writePlatformFiles(t)}, tt.extra...)
			code, stdout, stderr := runCommand(context.Background(), env, args...)
			if code != tt.code || stdout != "" || stderr != tt.want {
				t.Fatalf("code = %d, stdout = %q, stderr = %q", code, stdout, stderr)
			}
		})
	}
}

func TestProbe(t *testing.T) {
	status := http.StatusOK
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(status)
	}))
	defer server.Close()

	code, stdout, stderr := runCommand(context.Background(), Env{}, "admin", "probe", server.URL)
	if code != 0 || stdout != "probe: ok\n" || stderr != "" {
		t.Fatalf("200: code = %d, stdout = %q, stderr = %q", code, stdout, stderr)
	}

	status = http.StatusServiceUnavailable
	code, stdout, stderr = runCommand(context.Background(), Env{}, "admin", "probe", server.URL)
	if code != 1 || stdout != "" || stderr != "error: probe: 503\n" {
		t.Fatalf("503: code = %d, stdout = %q, stderr = %q", code, stdout, stderr)
	}

	closed := httptest.NewServer(http.NotFoundHandler())
	url := closed.URL
	closed.Close()
	code, stdout, stderr = runCommand(context.Background(), Env{}, "admin", "probe", url)
	if code != 1 || stdout != "" || !strings.HasPrefix(stderr, "error: probe: cannot reach "+url+": ") {
		t.Fatalf("closed: code = %d, stdout = %q, stderr = %q", code, stdout, stderr)
	}

	code, _, stderr = runCommand(context.Background(), Env{}, "admin", "probe")
	if code != 2 || stderr != "error: probe takes one argument; found 0\n" {
		t.Fatalf("no argument: code = %d, stderr = %q", code, stderr)
	}
}
