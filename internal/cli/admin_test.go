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
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/marceloalvescf/capacity-room/internal/platform/audit"
	"github.com/marceloalvescf/capacity-room/internal/platform/auth"
	"github.com/marceloalvescf/capacity-room/internal/platform/store"
	"github.com/marceloalvescf/capacity-room/internal/platform/storetest"
)

const bootstrapPassword = "correct horse battery staple"

// bootstrap runs bootstrap-admin against pool with stdin and returns the code
// and both streams.
func bootstrap(t *testing.T, pool *pgxpool.Pool, config, stdin string, extra ...string) (int, string, string) {
	t.Helper()
	env := platformEnv(map[string]string{
		"CAPACITYROOM_BOOTSTRAP_MODE": "1",
		"CAPACITYROOM_DATABASE_DSN":   pool.Config().ConnString(),
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
	if user.Role != auth.RoleAdministrator || user.Language != "en" || !user.LanguageFollowsBrowser {
		t.Fatalf("user = %+v", user)
	}
	if match, _, _ := auth.VerifyPassword(context.Background(), bootstrapPassword, user.PasswordHash); !match {
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

// bootstrapFromFile runs bootstrap-admin against pool with the password in a
// file and returns the code and both streams.
func bootstrapFromFile(t *testing.T, pool *pgxpool.Pool, content string) (int, string, string) {
	t.Helper()
	file := filepath.Join(t.TempDir(), "password")
	if err := os.WriteFile(file, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	env := platformEnv(map[string]string{
		"CAPACITYROOM_BOOTSTRAP_MODE": "1",
		"CAPACITYROOM_DATABASE_DSN":   pool.Config().ConnString(),
	}, "")
	return runCommand(context.Background(), env,
		"admin", "bootstrap-admin", "--username", "Root", "--password-file", file, "--config", writePlatformFiles(t))
}

func TestBootstrapPasswordFileReadsOneLine(t *testing.T) {
	pool := storetest.New(t)
	code, stdout, stderr := bootstrapFromFile(t, pool, bootstrapPassword+"\r\nmore\n")
	if code != 0 || stdout != "created administrator root\n" || stderr != "" {
		t.Fatalf("code = %d, stdout = %q, stderr = %q", code, stdout, stderr)
	}
	user, err := store.FindActiveUserByUsername(context.Background(), pool, "root")
	if err != nil {
		t.Fatal(err)
	}
	if match, _, _ := auth.VerifyPassword(context.Background(), bootstrapPassword, user.PasswordHash); !match {
		t.Fatal("the stored password is not the first line of the file without its line ending")
	}
}

func TestBootstrapPasswordFileRefusesSecondUser(t *testing.T) {
	pool := storetest.New(t)
	if code, _, stderr := bootstrapFromFile(t, pool, bootstrapPassword); code != 0 {
		t.Fatalf("first bootstrap: code = %d, stderr = %q", code, stderr)
	}
	code, stdout, stderr := bootstrapFromFile(t, pool, bootstrapPassword)
	const want = "error: a user already exists; bootstrap-admin creates only the first\n"
	if code != 1 || stdout != "" || stderr != want {
		t.Fatalf("code = %d, stdout = %q, stderr = %q, want %q and code 1", code, stdout, stderr, want)
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
	if err != nil || user.Language != "en" || user.LanguageFollowsBrowser {
		t.Fatalf("user = %+v, err = %v, want language en pinned", user, err)
	}
}

func TestBootstrapLanguagePtBR(t *testing.T) {
	pool := storetest.New(t)
	config := writePlatformFiles(t)
	if code, _, stderr := bootstrap(t, pool, config, bootstrapPassword+"\n", "--language", "pt-BR"); code != 0 {
		t.Fatalf("code = %d, stderr = %q", code, stderr)
	}
	user, err := store.FindActiveUserByUsername(context.Background(), pool, "root")
	if err != nil || user.Language != "pt-BR" || user.LanguageFollowsBrowser {
		t.Fatalf("user = %+v, err = %v, want language pt-BR pinned", user, err)
	}
}

func TestMigrate(t *testing.T) {
	pool := storetest.NewEmpty(t)
	env := platformEnv(map[string]string{"CAPACITYROOM_DATABASE_DSN": pool.Config().ConnString()}, "")
	config := writePlatformFiles(t)
	for range 2 {
		code, stdout, stderr := runCommand(context.Background(), env, "admin", "migrate", "--config", config)
		if code != 0 || stdout != "migrate: schema at version 6\n" || stderr != "" {
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
		env := platformEnv(map[string]string{"CAPACITYROOM_DATABASE_DSN": pool.Config().ConnString()}, "")
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

	if err := store.SetCurrentRevision(context.Background(), pool, "b", nil, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	code, stdout, stderr = run()
	if code != 0 || stdout != "unchanged a\nrecreated b\n" || stderr != "" {
		t.Fatalf("third run: code = %d, stdout = %q, stderr = %q", code, stdout, stderr)
	}
	cur, err = store.GetCurrentRevision(context.Background(), pool, "b")
	if err != nil || cur.Revision != 2 {
		t.Fatalf("b = %+v, err %v", cur, err)
	}
}

// resetPasswordRun runs reset-password against pool with stdin.
func resetPasswordRun(t *testing.T, pool *pgxpool.Pool, config, username, stdin string) (int, string, string) {
	t.Helper()
	env := platformEnv(map[string]string{
		"CAPACITYROOM_BOOTSTRAP_MODE": "1",
		"CAPACITYROOM_DATABASE_DSN":   pool.Config().ConnString(),
	}, stdin)
	return runCommand(context.Background(), env,
		"admin", "reset-password", "--username", username, "--password-stdin", "--config", config)
}

func TestResetPasswordCommand(t *testing.T) {
	pool := storetest.New(t)
	config := writePlatformFiles(t)
	if code, _, stderr := bootstrap(t, pool, config, bootstrapPassword+"\n"); code != 0 {
		t.Fatalf("bootstrap: code = %d, stderr = %q", code, stderr)
	}
	code, stdout, stderr := resetPasswordRun(t, pool, config, "Root", "new-password-12\n")
	if code != 0 || stdout != "reset the password of root\n" || stderr != "" {
		t.Fatalf("code = %d, stdout = %q, stderr = %q", code, stdout, stderr)
	}
	sessions := &auth.Sessions{
		Pool:     pool,
		Throttle: auth.Throttle{Secret: bytes.Repeat([]byte("s"), 32)},
		TTL:      time.Hour,
		Idle:     time.Hour,
		Now:      time.Now,
	}
	if _, err := sessions.Login(context.Background(), "root", "new-password-12", "", ""); err != nil {
		t.Fatalf("sign-in with the new password: %v", err)
	}
	if _, err := sessions.Login(context.Background(), "root", bootstrapPassword, "", ""); err == nil {
		t.Fatal("sign-in with the old password succeeded")
	}
}

func TestResetPasswordCommandRefusesAnUnknownUser(t *testing.T) {
	pool := storetest.New(t)
	code, stdout, stderr := resetPasswordRun(t, pool, writePlatformFiles(t), "nobody", "new-password-12\n")
	if code != 1 || stdout != "" || stderr != "error: no active user has that username\n" {
		t.Fatalf("code = %d, stdout = %q, stderr = %q", code, stdout, stderr)
	}
}

func TestResetPasswordReadsOneLine(t *testing.T) {
	pool := storetest.New(t)
	config := writePlatformFiles(t)
	if code, _, stderr := bootstrap(t, pool, config, bootstrapPassword+"\n"); code != 0 {
		t.Fatalf("bootstrap: code = %d, stderr = %q", code, stderr)
	}
	code, _, stderr := resetPasswordRun(t, pool, config, "root", "new-password-12\r\nmore\n")
	if code != 0 {
		t.Fatalf("code = %d, stderr = %q", code, stderr)
	}
	user, err := store.FindActiveUserByUsername(context.Background(), pool, "root")
	if err != nil {
		t.Fatal(err)
	}
	if match, _, _ := auth.VerifyPassword(context.Background(), "new-password-12", user.PasswordHash); !match {
		t.Fatal("the stored password is not the first line without its line ending")
	}
}
