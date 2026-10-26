//go:build postgres

package cli

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/marceloalvescf/capacity-room/internal/platform/auth"
	"github.com/marceloalvescf/capacity-room/internal/platform/netpolicy"
	"github.com/marceloalvescf/capacity-room/internal/platform/profiles"
	"github.com/marceloalvescf/capacity-room/internal/platform/settings"
	"github.com/marceloalvescf/capacity-room/internal/platform/store"
	"github.com/marceloalvescf/capacity-room/internal/platform/storetest"
	"github.com/marceloalvescf/capacity-room/internal/redact"
)

func TestServeWiresTheProfiles(t *testing.T) {
	pool := storetest.New(t)
	now := time.Date(2026, 10, 5, 12, 0, 0, 0, time.UTC)
	hash, err := auth.HashPassword(context.Background(), "reader password one")
	if err != nil {
		t.Fatal(err)
	}
	if err := store.InsertUser(context.Background(), pool, store.User{
		ID: uuid.New(), Username: "rita", PasswordHash: hash, Role: auth.RoleReader,
		Language: "en", Active: true, CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatal(err)
	}

	loaded := settings.Settings{
		SessionTTL: 12 * time.Hour, SessionIdle: time.Hour,
		CredentialDir: t.TempDir(), CABundleDir: t.TempDir(),
	}
	server, err := buildServer(pool, openDownloads(t, pool), loaded, []byte("0123456789abcdef0123456789abcdef"),
		func() time.Time { return now }, slog.New(slog.NewTextHandler(io.Discard, nil)), redact.New())
	if err != nil {
		t.Fatal(err)
	}
	front := httptest.NewServer(server.Handler())
	defer front.Close()

	client := &reader{t: t, base: front.URL, cookies: map[string]string{}}
	client.mustStatus(401, "GET", "/api/v1/profiles", "", nil)

	client.signIn("rita", "reader password one")
	listed := client.mustStatus(200, "GET", "/api/v1/profiles", "", nil)
	if want := map[string]any{"profiles": []any{}}; !reflect.DeepEqual(listed, want) {
		t.Fatalf("GET /api/v1/profiles = %v, want %v", listed, want)
	}
}

func TestServeRevealsACredential(t *testing.T) {
	const token = "capacityroom-credential-for-the-test"
	pool := storetest.New(t)
	ctx := context.Background()
	now := time.Date(2026, 10, 5, 12, 0, 0, 0, time.UTC)
	hash, err := auth.HashPassword(context.Background(), "admin password one")
	if err != nil {
		t.Fatal(err)
	}
	admin := store.User{
		ID: uuid.New(), Username: "ada", PasswordHash: hash, Role: auth.RoleAdministrator,
		Language: "en", Active: true, CreatedAt: now, UpdatedAt: now,
	}
	if err := store.InsertUser(ctx, pool, admin); err != nil {
		t.Fatal(err)
	}

	source := httptest.NewServer(http.NotFoundHandler())
	defer source.Close()
	credentials := t.TempDir()
	if err := os.WriteFile(filepath.Join(credentials, "token"), []byte(token+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	loaded := settings.Settings{
		SessionTTL: 12 * time.Hour, SessionIdle: time.Hour,
		CredentialDir: credentials, CABundleDir: t.TempDir(),
		AllowedEndpointCIDRs: []netip.Prefix{netip.MustParsePrefix("127.0.0.1/32")},
	}
	policy := netpolicy.Policy{Networks: loaded.AllowedEndpointCIDRs}
	doc := profiles.Defaults()
	doc.DisplayName = "Production"
	doc.Active = true
	doc.EndpointURL = "http://" + source.Listener.Addr().String() + "/"
	doc.ClusterName = "prod"
	doc.AllowedNamespaces = []string{"payments"}
	doc.CredentialFile = "token"
	seed := &profiles.Service{Pool: pool, Policy: policy, Settings: loaded, Now: func() time.Time { return now }}
	if _, err := seed.Create(ctx, auth.Principal{ID: admin.ID, Role: auth.RoleAdministrator}, "prod", doc); err != nil {
		t.Fatal(err)
	}

	redactor := redact.New()
	if redactor.Text(token) != token {
		t.Fatal("the redactor knew the credential before the test")
	}
	server, err := buildServer(pool, openDownloads(t, pool), loaded, []byte("0123456789abcdef0123456789abcdef"),
		func() time.Time { return now }, slog.New(slog.NewTextHandler(io.Discard, nil)), redactor)
	if err != nil {
		t.Fatal(err)
	}
	front := httptest.NewServer(server.Handler())
	defer front.Close()

	client := &reader{t: t, base: front.URL, cookies: map[string]string{}}
	csrf := client.signIn("ada", "admin password one")
	client.mustStatus(200, "POST", "/api/v1/admin/profiles/prod/test", csrf, nil)

	var out bytes.Buffer
	if _, err := io.WriteString(redactor.Writer(&out), "diagnostic "+token+" end\n"); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out.String(), token) {
		t.Fatalf("the credential came out of the redacted stream: %q", out.String())
	}
	if !strings.Contains(out.String(), "diagnostic ") {
		t.Fatalf("the stream lost its text: %q", out.String())
	}
}

func TestServeBuildsTheReportRoutes(t *testing.T) {
	pool := storetest.New(t)
	ctx := context.Background()
	now := time.Date(2026, 10, 5, 12, 0, 0, 0, time.UTC)
	hash, err := auth.HashPassword(context.Background(), "operator password one")
	if err != nil {
		t.Fatal(err)
	}
	operator := store.User{
		ID: uuid.New(), Username: "olga", PasswordHash: hash, Role: auth.RoleOperator,
		Language: "en", Active: true, CreatedAt: now, UpdatedAt: now,
	}
	if err := store.InsertUser(ctx, pool, operator); err != nil {
		t.Fatal(err)
	}

	source := httptest.NewServer(http.NotFoundHandler())
	defer source.Close()
	loaded := settings.Settings{
		SessionTTL: 12 * time.Hour, SessionIdle: time.Hour,
		CredentialDir: t.TempDir(), CABundleDir: t.TempDir(),
		AllowedEndpointCIDRs: []netip.Prefix{netip.MustParsePrefix("127.0.0.1/32")},
		JobMaxAttempts:       3, ReportRetention: 7 * 24 * time.Hour,
		AdmissionLimit: 20, AdmissionProfileLimit: 5, AdmissionRetry: 5 * time.Second,
	}
	doc := profiles.Defaults()
	doc.DisplayName = "Production"
	doc.Active = true
	doc.EndpointURL = "http://" + source.Listener.Addr().String() + "/"
	doc.ClusterName = "prod"
	doc.AllowedNamespaces = []string{"payments"}
	seed := &profiles.Service{
		Pool: pool, Policy: netpolicy.Policy{Networks: loaded.AllowedEndpointCIDRs},
		Settings: loaded, Now: func() time.Time { return now },
	}
	if _, err := seed.Create(ctx, auth.Principal{ID: operator.ID, Role: auth.RoleAdministrator}, "prod", doc); err != nil {
		t.Fatal(err)
	}

	server, err := buildServer(pool, openDownloads(t, pool), loaded, []byte("0123456789abcdef0123456789abcdef"),
		func() time.Time { return now }, slog.New(slog.NewTextHandler(io.Discard, nil)), redact.New())
	if err != nil {
		t.Fatal(err)
	}
	front := httptest.NewServer(server.Handler())
	defer front.Close()

	client := &reader{t: t, base: front.URL, cookies: map[string]string{}, header: map[string]string{"Idempotency-Key": "wiring-1"}}
	csrf := client.signIn("olga", "operator password one")
	accepted := client.mustStatus(202, "POST", "/api/v1/reports", csrf,
		map[string]any{"profile_id": "prod", "namespaces": []string{"payments"}, "language": "en"})
	id, _ := accepted["id"].(string)
	if id == "" || accepted["replayed"] != false {
		t.Fatalf("answer %v", accepted)
	}
	var jobs int
	err = pool.QueryRow(ctx, `SELECT count(*) FROM river_job WHERE kind = 'collect' AND args->>'report_id' = $1`, id).Scan(&jobs)
	if err != nil || jobs != 1 {
		t.Fatalf("%d collect jobs for the report, %v", jobs, err)
	}
}

// openDownloads opens the pool that reads report bodies on the database of pool.
func openDownloads(t *testing.T, pool *pgxpool.Pool) *pgxpool.Pool {
	t.Helper()
	downloads, err := store.OpenDownloads(context.Background(), pool.Config().ConnString())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(downloads.Close)
	return downloads
}

func TestServeOpensTheDownloadPool(t *testing.T) {
	pool := storetest.New(t)
	downloads := openDownloads(t, pool)
	server, err := buildServer(pool, downloads, settings.Settings{}, []byte("0123456789abcdef0123456789abcdef"),
		time.Now, slog.New(slog.NewTextHandler(io.Discard, nil)), redact.New())
	if err != nil {
		t.Fatal(err)
	}
	if server.Reports.Bodies != downloads || server.Reports.Bodies == pool {
		t.Fatal("the reports service does not read bodies through the download pool")
	}
}
