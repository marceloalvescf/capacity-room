//go:build postgres

package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/marceloalvescf/k8s-capacity-report/internal/platform/audit"
	"github.com/marceloalvescf/k8s-capacity-report/internal/platform/auth"
	"github.com/marceloalvescf/k8s-capacity-report/internal/platform/store"
	"github.com/marceloalvescf/k8s-capacity-report/internal/platform/storetest"
)

const bootstrapPassword = "correct horse battery staple"

// bootstrap runs bootstrap-admin against pool with stdin and returns the code
// and both streams.
func bootstrap(t *testing.T, pool *pgxpool.Pool, config, stdin string, extra ...string) (int, string, string) {
	t.Helper()
	env := platformEnv(map[string]string{
		"KCR_BOOTSTRAP_MODE": "1",
		"KCR_DATABASE_DSN":   pool.Config().ConnString(),
	}, stdin)
	args := append([]string{"admin", "bootstrap-admin", "--username", "Root", "--password-stdin", "--config", config}, extra...)
	return runCommand(context.Background(), env, args...)
}

func TestBootstrapReadsOneLine(t *testing.T) {
	pool := storetest.New(t)
	code, stdout, stderr := bootstrap(t, pool, writePlatformFiles(t), bootstrapPassword+"\r\nmore\n")
	if code != 0 || stdout != "created administrator root\n" || stderr != "" {
		t.Fatalf("code = %d, stdout = %q, stderr = %q", code, stdout, stderr)
	}
	user, err := store.FindActiveUserByUsername(context.Background(), pool, "root")
	if err != nil {
		t.Fatal(err)
	}
	if user.Role != auth.RoleAdministrator || user.Language != "pt-BR" {
		t.Fatalf("user = %+v", user)
	}
	if match, _ := auth.VerifyPassword(bootstrapPassword, user.PasswordHash); !match {
		t.Fatal("the stored password is not the first line without its line ending")
	}
}

func TestBootstrapAcceptsLastLineWithoutNewline(t *testing.T) {
	pool := storetest.New(t)
	code, _, stderr := bootstrap(t, pool, writePlatformFiles(t), bootstrapPassword)
	if code != 0 {
		t.Fatalf("code = %d, stderr = %q", code, stderr)
	}
}

func TestBootstrapRefusesSecondUser(t *testing.T) {
	pool := storetest.New(t)
	config := writePlatformFiles(t)
	if code, _, stderr := bootstrap(t, pool, config, bootstrapPassword+"\n"); code != 0 {
		t.Fatalf("first: code = %d, stderr = %q", code, stderr)
	}
	code, stdout, stderr := bootstrap(t, pool, config, bootstrapPassword+"\n")
	const want = "error: a user already exists; bootstrap-admin creates only the first\n"
	if code != 1 || stdout != "" || stderr != want {
		t.Fatalf("second: code = %d, stdout = %q, stderr = %q", code, stdout, stderr)
	}
	users, err := store.ListUsers(context.Background(), pool)
	if err != nil || len(users) != 1 {
		t.Fatalf("users = %d, err = %v, want 1", len(users), err)
	}
}

func TestBootstrapAudits(t *testing.T) {
	pool := storetest.New(t)
	if code, _, stderr := bootstrap(t, pool, writePlatformFiles(t), bootstrapPassword+"\n"); code != 0 {
		t.Fatalf("code = %d, stderr = %q", code, stderr)
	}
	user, err := store.FindActiveUserByUsername(context.Background(), pool, "root")
	if err != nil {
		t.Fatal(err)
	}
	events, err := store.ListAuditEvents(context.Background(), pool)
	if err != nil || len(events) != 1 {
		t.Fatalf("events = %d, err = %v, want 1", len(events), err)
	}
	e := events[0]
	if e.Action != audit.ActionBootstrap || e.ActorID != nil || e.TargetType == nil || *e.TargetType != "user" ||
		e.TargetID == nil || *e.TargetID != user.ID.String() || !jsonEqual(e.Metadata, `{"role":"administrator"}`) {
		t.Fatalf("event = %+v", e)
	}
}

func TestBootstrapLanguage(t *testing.T) {
	pool := storetest.New(t)
	config := writePlatformFiles(t)
	if code, _, stderr := bootstrap(t, pool, config, bootstrapPassword+"\n", "--language", "fr"); code != 2 {
		t.Fatalf("fr: code = %d, stderr = %q, want 2", code, stderr)
	}
	if code, _, stderr := bootstrap(t, pool, config, bootstrapPassword+"\n", "--language", "en"); code != 0 {
		t.Fatalf("en: code = %d, stderr = %q", code, stderr)
	}
	user, err := store.FindActiveUserByUsername(context.Background(), pool, "root")
	if err != nil || user.Language != "en" {
		t.Fatalf("user = %+v, err = %v, want language en", user, err)
	}
}

func TestMigrate(t *testing.T) {
	pool := storetest.NewEmpty(t)
	env := platformEnv(map[string]string{"KCR_DATABASE_DSN": pool.Config().ConnString()}, "")
	config := writePlatformFiles(t)
	for range 2 {
		code, stdout, stderr := runCommand(context.Background(), env, "admin", "migrate", "--config", config)
		if code != 0 || stdout != "migrate: schema at version 2\n" || stderr != "" {
			t.Fatalf("code = %d, stdout = %q, stderr = %q", code, stdout, stderr)
		}
	}
}

// jsonEqual reports whether stored and want hold the same JSON value.
func jsonEqual(stored []byte, want string) bool {
	var a, b any
	return json.Unmarshal(stored, &a) == nil && json.Unmarshal([]byte(want), &b) == nil && reflect.DeepEqual(a, b)
}

func TestBootstrapMigrates(t *testing.T) {
	pool := storetest.NewEmpty(t)
	code, stdout, stderr := bootstrap(t, pool, writePlatformFiles(t), bootstrapPassword+"\n")
	if code != 0 || stdout != "created administrator root\n" || stderr != "" {
		t.Fatalf("code = %d, stdout = %q, stderr = %q", code, stdout, stderr)
	}
	users, err := store.ListUsers(context.Background(), pool)
	if err != nil || len(users) != 1 || users[0].Role != auth.RoleAdministrator {
		t.Fatalf("users = %+v, err = %v, want the administrator", users, err)
	}
}

const importExample = `
[[profiles]]
id = "a"
display_name = "Alpha"
endpoint_url = "https://metrics.test/api/v1"
cluster_name = "alpha"
allowed_namespaces = ["payments"]
active = true

[[profiles]]
id = "b"
display_name = "Beta"
endpoint_url = "https://metrics.test/api/v1"
endpoint_type = "prometheus"
cluster_name = "beta"
allowed_namespaces = ["checkout"]
`

func TestAdminImportProfilesCommand(t *testing.T) {
	pool := storetest.New(t)
	dir := t.TempDir()
	secret := filepath.Join(dir, "secret")
	if err := os.WriteFile(secret, bytes.Repeat([]byte("s"), 32), 0o600); err != nil {
		t.Fatal(err)
	}
	config := filepath.Join(dir, "platform.toml")
	settingsFile := "server_secret_path = \"" + secret + "\"\nallowed_endpoint_hostnames = [\"metrics.test\"]\n"
	if err := os.WriteFile(config, []byte(settingsFile), 0o600); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(dir, "profiles.toml")
	if err := os.WriteFile(file, []byte(importExample), 0o600); err != nil {
		t.Fatal(err)
	}
	run := func() (int, string, string) {
		env := platformEnv(map[string]string{"KCR_DATABASE_DSN": pool.Config().ConnString()}, "")
		return runCommand(context.Background(), env, "admin", "import-profiles", "--config", config, "--file", file)
	}

	code, stdout, stderr := run()
	if code != 0 || stdout != "created a\ncreated b\n" || stderr != "" {
		t.Fatalf("first run: code = %d, stdout = %q, stderr = %q", code, stdout, stderr)
	}
	code, stdout, stderr = run()
	if code != 0 || stdout != "unchanged a\nunchanged b\n" || stderr != "" {
		t.Fatalf("second run: code = %d, stdout = %q, stderr = %q", code, stdout, stderr)
	}
	cur, err := store.GetCurrentRevision(context.Background(), pool, "b")
	if err != nil || cur.Revision != 1 || cur.CreatedBy != nil {
		t.Fatalf("b = %+v, err %v", cur, err)
	}
}
