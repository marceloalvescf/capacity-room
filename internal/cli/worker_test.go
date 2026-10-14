package cli

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/marceloalvescf/capacity-room/internal/platform/settings"
)

func TestWorkerHelpListsItsTwoFlags(t *testing.T) {
	code, stdout, stderr := runCommand(context.Background(), Env{}, "worker", "--help")
	if code != 0 || stderr != "" {
		t.Fatalf("code = %d, stderr = %q", code, stderr)
	}
	flags := regexp.MustCompile(`(?m)^\s+(?:-[a-z], )?--([a-z-]+)`).FindAllStringSubmatch(stdout, -1)
	var got []string
	for _, m := range flags {
		got = append(got, m[1])
	}
	// cobra adds --help itself
	if want := "config health-listen help"; strings.Join(got, " ") != want {
		t.Fatalf("flags = %v, want %q\n%s", got, want, stdout)
	}
}

func TestWorkerRefusesArguments(t *testing.T) {
	code, stdout, stderr := runCommand(context.Background(), Env{}, "worker", "hunter2")
	const want = "error: worker takes no argument; found 1\n"
	if code != 2 || stderr != want || stdout != "" {
		t.Fatalf("code = %d, stdout = %q, stderr = %q, want 2, empty, %q", code, stdout, stderr, want)
	}
}

func TestWorkerCommandErrors(t *testing.T) {
	good := writePlatformFiles(t)
	bad := filepath.Join(t.TempDir(), "bad.toml")
	if err := os.WriteFile(bad, []byte("no_such_key = 1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	withDSN := func(dsn string, typst string) Env {
		env := platformEnv(map[string]string{"CAPACITYROOM_DATABASE_DSN": dsn, "CAPACITYROOM_TYPST": typst}, "")
		return env
	}
	cases := []struct {
		name   string
		env    Env
		config string
		code   int
		prefix string
	}{
		{"a bad settings file", withDSN(closedPortDSN, fakeTypstPath(t)), bad, 2, "error: "},
		{"typst pointing at nothing", withDSN(closedPortDSN, "/nonexistent/typst"), good, 1, "error: CAPACITYROOM_TYPST names no file"},
		{"an unreachable database", withDSN(closedPortDSN, fakeTypstPath(t)), good, 1, "error: cannot connect to the database: "},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			code, stdout, stderr := runCommand(context.Background(), tc.env, "worker", "--config", tc.config)
			if code != tc.code || stdout != "" || !strings.HasPrefix(stderr, tc.prefix) ||
				strings.Count(stderr, "\n") != 1 || !strings.HasSuffix(stderr, "\n") {
				t.Fatalf("code = %d, stdout = %q, stderr = %q", code, stdout, stderr)
			}
			if strings.Contains(stderr, "hunter2secret") {
				t.Fatalf("the password came out: %q", stderr)
			}
		})
	}
}

func TestWorkerPoolConns(t *testing.T) {
	for slots, want := range map[int]int32{1: 9, 2: 10, 16: 24} {
		if got := workerPoolConns(settings.Settings{WorkerSlots: slots}); got != want {
			t.Errorf("slots %d: %d, want %d", slots, got, want)
		}
	}
}
