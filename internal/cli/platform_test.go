package cli

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/marceloalvescf/capacity-room/internal/redact"
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

func TestAdminListsFiveCommands(t *testing.T) {
	code, stdout, stderr := runCommand(context.Background(), Env{}, "admin")
	if code != 0 {
		t.Fatalf("code = %d, want 0; stderr = %q", code, stderr)
	}
	for _, name := range []string{"bootstrap-admin", "import-profiles", "migrate", "probe", "reset-password"} {
		if !strings.Contains(stdout, name) {
			t.Errorf("help does not name %q: %q", name, stdout)
		}
	}

	code, stdout, stderr = runCommand(context.Background(), Env{}, "admin", "hunter2")
	const want = "error: unknown command; the commands are: bootstrap-admin, import-profiles, migrate, probe, reset-password\n"
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
	env := platformEnv(map[string]string{"CAPACITYROOM_DATABASE_DSN": closedPortDSN}, "")
	code, stdout, stderr := runCommand(context.Background(), env, "serve", "--config", "nope.toml")
	if code != 2 || stdout != "" || !strings.HasPrefix(stderr, "error: cannot read nope.toml: ") {
		t.Fatalf("code = %d, stdout = %q, stderr = %q", code, stdout, stderr)
	}
}

func TestServeRedactsDSN(t *testing.T) {
	env := platformEnv(map[string]string{"CAPACITYROOM_DATABASE_DSN": closedPortDSN}, "")
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
	env := platformEnv(map[string]string{"CAPACITYROOM_DATABASE_DSN": dsn}, "")
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

func TestServerTimeouts(t *testing.T) {
	server := newHTTPServer(http.NotFoundHandler(), newLogger(io.Discard, redact.New()))
	if server.ReadHeaderTimeout != 10*time.Second || server.ReadTimeout != 30*time.Second ||
		server.IdleTimeout != 2*time.Minute || server.WriteTimeout != 0 {
		t.Fatalf("timeouts: header %v, read %v, idle %v, write %v",
			server.ReadHeaderTimeout, server.ReadTimeout, server.IdleTimeout, server.WriteTimeout)
	}
}

func TestOpenPlatformArmsTheRedactor(t *testing.T) {
	env := platformEnv(map[string]string{"CAPACITYROOM_DATABASE_DSN": closedPortDSN}, "")
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
		env := platformEnv(map[string]string{"CAPACITYROOM_BOOTSTRAP_MODE": value}, "")
		env.Stdin = failingReader{t}
		code, stdout, stderr := runCommand(context.Background(), env,
			"admin", "bootstrap-admin", "--username", "root", "--password-stdin")
		const want = "error: bootstrap-admin is off; set CAPACITYROOM_BOOTSTRAP_MODE=1 to run it\n"
		if code != 1 || stderr != want || stdout != "" {
			t.Fatalf("value %q: code = %d, stdout = %q, stderr = %q", value, code, stdout, stderr)
		}
	}
}

func TestBootstrapNeedsOnePasswordSource(t *testing.T) {
	file := filepath.Join(t.TempDir(), "password")
	if err := os.WriteFile(file, []byte("correct horse battery staple\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	const want = "error: pass one of --password-stdin and --password-file\n"
	for name, extra := range map[string][]string{
		"neither": nil,
		"both":    {"--password-stdin", "--password-file", file},
	} {
		t.Run(name, func(t *testing.T) {
			env := platformEnv(map[string]string{"CAPACITYROOM_BOOTSTRAP_MODE": "1"}, "")
			env.Stdin = failingReader{t}
			args := append([]string{"admin", "bootstrap-admin", "--username", "root"}, extra...)
			code, stdout, stderr := runCommand(context.Background(), env, args...)
			if code != 2 || stderr != want || stdout != "" {
				t.Fatalf("code = %d, stdout = %q, stderr = %q", code, stdout, stderr)
			}
		})
	}
}

func TestBootstrapPasswordFileRefusals(t *testing.T) {
	dir := t.TempDir()
	large := filepath.Join(dir, "large")
	if err := os.WriteFile(large, bytes.Repeat([]byte("a"), 4097), 0o600); err != nil {
		t.Fatal(err)
	}
	short := filepath.Join(dir, "short")
	if err := os.WriteFile(short, []byte("tooshort\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	missing := filepath.Join(dir, "missing")
	tests := []struct {
		name, path, want string
	}{
		{"missing", missing, "error: cannot read the password file at " + missing + ": no such file or directory\n"},
		{"folder", dir, "error: the password file at " + dir + " is not a regular file\n"},
		{"large", large, "error: the password file at " + large + " is larger than 4 KiB\n"},
		{"short", short, "error: the password is too short\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := platformEnv(map[string]string{"CAPACITYROOM_BOOTSTRAP_MODE": "1", "CAPACITYROOM_DATABASE_DSN": closedPortDSN}, "")
			env.Stdin = failingReader{t}
			code, stdout, stderr := runCommand(context.Background(), env,
				"admin", "bootstrap-admin", "--username", "root", "--password-file", tt.path, "--config", writePlatformFiles(t))
			if code != 1 || stdout != "" || stderr != tt.want {
				t.Fatalf("code = %d, stdout = %q, stderr = %q", code, stdout, stderr)
			}
			if strings.Contains(stderr, "tooshort") {
				t.Fatalf("stderr = %q holds the content of the file", stderr)
			}
		})
	}
}

func TestBootstrapPasswordFileArmsTheRedactor(t *testing.T) {
	const password = "correct horse battery staple"
	file := filepath.Join(t.TempDir(), "password")
	if err := os.WriteFile(file, []byte(password+"\r\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	redactor := redact.New()
	_, err := bootstrapAdmin(context.Background(),
		platformEnv(map[string]string{"CAPACITYROOM_BOOTSTRAP_MODE": "1", "CAPACITYROOM_DATABASE_DSN": closedPortDSN}, ""),
		redactor, bootstrapInput{username: "root", language: "en", config: writePlatformFiles(t), passwordFile: file})
	if err == nil {
		t.Fatal("bootstrapAdmin succeeded against a closed port")
	}
	if got := redactor.Text(password); strings.Contains(got, password) {
		t.Fatalf("redactor.Text(password) = %q, want it redacted", got)
	}
}

func TestBootstrapNoPasswordFlag(t *testing.T) {
	env := platformEnv(map[string]string{"CAPACITYROOM_BOOTSTRAP_MODE": "1"}, "")
	code, stdout, stderr := runCommand(context.Background(), env,
		"admin", "bootstrap-admin", "--username", "root", "--password", "xsecretvalue")
	if code != 2 || stdout != "" || stderr != "error: unknown flag; run the command with --help to see its flags\n" {
		t.Fatalf("code = %d, stdout = %q, stderr = %q", code, stdout, stderr)
	}
	if strings.Contains(stdout+stderr, "xsecretvalue") {
		t.Fatalf("a stream held the value: %q %q", stdout, stderr)
	}
}

func TestBootstrapValidatesBeforeDatabase(t *testing.T) {
	env := platformEnv(map[string]string{"CAPACITYROOM_BOOTSTRAP_MODE": "1", "CAPACITYROOM_DATABASE_DSN": closedPortDSN},
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
			env := platformEnv(map[string]string{"CAPACITYROOM_BOOTSTRAP_MODE": "1", "CAPACITYROOM_DATABASE_DSN": closedPortDSN}, tt.stdin)
			args := append([]string{"admin", "bootstrap-admin", "--username", "root", "--password-stdin",
				"--config", writePlatformFiles(t)}, tt.extra...)
			code, stdout, stderr := runCommand(context.Background(), env, args...)
			if code != tt.code || stdout != "" || stderr != tt.want {
				t.Fatalf("code = %d, stdout = %q, stderr = %q", code, stdout, stderr)
			}
		})
	}
}

func TestResetPasswordIsOffWithoutTheMode(t *testing.T) {
	env := platformEnv(nil, "")
	env.Stdin = failingReader{t}
	code, stdout, stderr := runCommand(context.Background(), env,
		"admin", "reset-password", "--username", "root", "--password-stdin")
	const want = "error: reset-password is off; set CAPACITYROOM_BOOTSTRAP_MODE=1 to run it\n"
	if code != 1 || stderr != want || stdout != "" {
		t.Fatalf("code = %d, stdout = %q, stderr = %q", code, stdout, stderr)
	}
}

func TestResetPasswordNeedsPasswordStdin(t *testing.T) {
	env := platformEnv(map[string]string{"CAPACITYROOM_BOOTSTRAP_MODE": "1"}, "")
	env.Stdin = failingReader{t}
	code, stdout, stderr := runCommand(context.Background(), env, "admin", "reset-password", "--username", "root")
	const want = "error: the new password is read from standard input; pass --password-stdin and write it there\n"
	if code != 2 || stderr != want || stdout != "" {
		t.Fatalf("code = %d, stdout = %q, stderr = %q", code, stdout, stderr)
	}
}

func TestResetPasswordRefusesAnInvalidUsername(t *testing.T) {
	env := platformEnv(map[string]string{"CAPACITYROOM_BOOTSTRAP_MODE": "1", "CAPACITYROOM_DATABASE_DSN": closedPortDSN},
		"correct horse battery staple\n")
	code, stdout, stderr := runCommand(context.Background(), env,
		"admin", "reset-password", "--username", "a b", "--password-stdin", "--config", writePlatformFiles(t))
	if code != 1 || stdout != "" || stderr != "error: the username is not valid\n" {
		t.Fatalf("code = %d, stdout = %q, stderr = %q", code, stdout, stderr)
	}
}

func TestResetPasswordAppliesThePolicy(t *testing.T) {
	tests := []struct {
		name, stdin, want string
	}{
		{"short", strings.Repeat("a", 11) + "\n", "error: the password is too short\n"},
		{"long", strings.Repeat("a", 1025) + "\n", "error: the password is too long\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := platformEnv(map[string]string{"CAPACITYROOM_BOOTSTRAP_MODE": "1", "CAPACITYROOM_DATABASE_DSN": closedPortDSN}, tt.stdin)
			code, stdout, stderr := runCommand(context.Background(), env,
				"admin", "reset-password", "--username", "root", "--password-stdin", "--config", writePlatformFiles(t))
			if code != 1 || stdout != "" || stderr != tt.want {
				t.Fatalf("code = %d, stdout = %q, stderr = %q", code, stdout, stderr)
			}
		})
	}
}

func TestResetPasswordRefusesAPasswordFlag(t *testing.T) {
	env := platformEnv(map[string]string{"CAPACITYROOM_BOOTSTRAP_MODE": "1"}, "")
	code, stdout, stderr := runCommand(context.Background(), env,
		"admin", "reset-password", "--username", "root", "--password", "xsecretvalue")
	if code != 2 || stdout != "" || stderr != "error: unknown flag; run the command with --help to see its flags\n" {
		t.Fatalf("code = %d, stdout = %q, stderr = %q", code, stdout, stderr)
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

func TestAdminImportProfilesRefusals(t *testing.T) {
	config := writePlatformFiles(t)
	env := func() Env { return platformEnv(map[string]string{"CAPACITYROOM_DATABASE_DSN": closedPortDSN}, "") }
	dir := t.TempDir()
	invalid := filepath.Join(dir, "invalid.toml")
	if err := os.WriteFile(invalid, []byte("[[profiles]]\nid = \"a\"\ntoken = \"hunter2value\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	missing := filepath.Join(dir, "missing.toml")
	cases := []struct {
		name, want string
		args       []string
	}{
		{"no file flag", "error: --file is required\n", nil},
		{"missing file", "error: " + missing + ": cannot be read: no such file or directory\n", []string{"--file", missing}},
		{"invalid file", "error: " + invalid + ": unknown key profiles.token\n", []string{"--file", invalid}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			args := append([]string{"admin", "import-profiles", "--config", config}, c.args...)
			code, stdout, stderr := runCommand(context.Background(), env(), args...)
			if code != 2 || stdout != "" || stderr != c.want {
				t.Fatalf("code = %d, stdout = %q, stderr = %q, want 2, empty, %q", code, stdout, stderr, c.want)
			}
		})
	}
}

func TestServerErrorLogGoesThroughTheRedactor(t *testing.T) {
	const secret = "s3cr3t-t0ken-value"
	var out bytes.Buffer
	redactor := redact.New()
	redactor.Add(secret)
	server := newHTTPServer(http.NotFoundHandler(), newLogger(redactor.Writer(&out), redactor))
	if server.ErrorLog == nil {
		t.Fatal("the server has no ErrorLog")
	}
	server.ErrorLog.Printf("http: TLS handshake error: %s", secret)
	if got := out.String(); got == "" || strings.Contains(got, secret) || !strings.Contains(got, "level=ERROR") {
		t.Fatalf("ErrorLog wrote %q, want an error line without the secret", got)
	}
}
