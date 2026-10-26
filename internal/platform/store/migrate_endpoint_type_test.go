//go:build postgres

package store_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"github.com/marceloalvescf/capacity-room/internal/platform/profiles"
	"github.com/marceloalvescf/capacity-room/internal/platform/storetest"
	"github.com/marceloalvescf/capacity-room/internal/snapshot"
)

var schemaThreeNames = []string{
	"direct-complete", "direct-complete-range", "edge-scaling", "edge-values", "grafana-authenticated",
	"partial-data", "raw-fallback", "thanos-multicluster", "value-edge-cases",
}

// migrateTo opens a goose provider over the migrations and applies them up to version.
func migrateTo(ctx context.Context, t *testing.T, pool *pgxpool.Pool, version int64) *goose.Provider {
	t.Helper()
	db := stdlib.OpenDBFromPool(pool)
	t.Cleanup(func() { _ = db.Close() })
	provider, err := goose.NewProvider(goose.DialectPostgres, db, os.DirFS("migrations"), goose.WithDisableGlobalRegistry(true))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = provider.Close() })
	if _, err := provider.UpTo(ctx, version); err != nil {
		t.Fatal(err)
	}
	return provider
}

// seedSnapshot stores one collected report holding a 3.0 snapshot body and returns its id.
func seedSnapshot(ctx context.Context, t *testing.T, pool *pgxpool.Pool, owner, revision uuid.UUID, job int, body string) uuid.UUID {
	t.Helper()
	report := uuid.New()
	if _, err := pool.Exec(ctx, `INSERT INTO reports (id, owner_id, profile_id, profile_revision_id, request, status, phase, collect_job_id, created_at, collected_at, expires_at, updated_at)
VALUES ($1, $2, 'prod', $3, '{}', 'collected', 'complete', $4, now(), now(), now() + interval '30 days', now())`, report, owner, revision, job); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256([]byte(body))
	if _, err := pool.Exec(ctx, `INSERT INTO report_snapshots (report_id, schema_version, body, sha256, size_bytes, created_at)
VALUES ($1, '3.0', $2, $3, $4, now())`, report, []byte(body), hex.EncodeToString(sum[:]), len(body)); err != nil {
		t.Fatal(err)
	}
	return report
}

// seedProfile stores the profile "prod" with one revision holding document.
func seedProfile(ctx context.Context, t *testing.T, pool *pgxpool.Pool, document string) (owner, revision uuid.UUID) {
	t.Helper()
	owner, err := addUser(ctx, pool, "operator", "en")
	if err != nil {
		t.Fatal(err)
	}
	revision = uuid.New()
	if _, err := pool.Exec(ctx, insertProfile, "prod", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO profile_revisions (id, profile_id, revision, fingerprint, display_name, active, document, credential_file, ca_bundle_file, created_by, created_at)
VALUES ($1, 'prod', 1, 'fingerprint-one', 'Prod', true, $2::jsonb, '', '', NULL, now())`, revision, document); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `UPDATE profiles SET current_revision_id = $1 WHERE id = 'prod'`, revision); err != nil {
		t.Fatal(err)
	}
	return owner, revision
}

func TestMigrationNineCarriesStoredDataForward(t *testing.T) {
	ctx := context.Background()
	pool := storetest.NewEmpty(t)
	provider := migrateTo(ctx, t, pool, 8)
	owner, revision := seedProfile(ctx, t, pool, `{"display_name":"Prod"}`)

	ids := make([]uuid.UUID, len(schemaThreeNames))
	for i, name := range schemaThreeNames {
		ids[i] = seedSnapshot(ctx, t, pool, owner, revision, i+1, fileOf(t, "testdata/schema-3.0/"+name+".json"))
	}
	if _, err := provider.UpTo(ctx, 9); err != nil {
		t.Fatalf("migrate to 9: %v", err)
	}

	for i, name := range schemaThreeNames {
		var want string
		if name == "grafana-authenticated" {
			want = strings.Replace(fileOf(t, "testdata/schema-3.0/"+name+".json"), `"schema_version": "3.0"`, `"schema_version": "4.0"`, 1)
			want = strings.Replace(want, "\n      \"endpoint_type\": \"grafana\",", "", 1)
		} else {
			want = fileOf(t, "../../../testdata/snapshot/"+name+".json")
		}
		var version, hash string
		var body []byte
		var size int64
		if err := pool.QueryRow(ctx, `SELECT schema_version, body, sha256, size_bytes FROM report_snapshots WHERE report_id = $1`, ids[i]).Scan(&version, &body, &hash, &size); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		sum := sha256.Sum256([]byte(want))
		if version != "4.0" || string(body) != want || hash != hex.EncodeToString(sum[:]) || size != int64(len(want)) {
			t.Errorf("%s: version %q, body equal %v, hash equal %v, size %d (want %d)",
				name, version, string(body) == want, hash == hex.EncodeToString(sum[:]), size, len(want))
		}
		if _, err := snapshot.Decode(body); err != nil {
			t.Errorf("%s: the carried snapshot does not decode: %v", name, err)
		}
	}

	for _, trigger := range []string{"report_snapshots_are_immutable", "profile_revisions_are_immutable"} {
		var enabled string
		if err := pool.QueryRow(ctx, `SELECT tgenabled FROM pg_trigger WHERE tgname = $1`, trigger).Scan(&enabled); err != nil {
			t.Fatal(err)
		}
		if enabled != "O" {
			t.Errorf("trigger %s is not enabled: %q", trigger, enabled)
		}
	}
	_, err := pool.Exec(ctx, `UPDATE report_snapshots SET created_at = now()`)
	if code, _ := sqlState(t, err); code != "23000" {
		t.Errorf("snapshot update SQLSTATE = %s, want 23000", code)
	}
}

func TestMigrationNineDropsTheKeyFromRevisions(t *testing.T) {
	ctx := context.Background()
	pool := storetest.NewEmpty(t)
	provider := migrateTo(ctx, t, pool, 8)
	const url = "https://grafana.example.com/api/datasources/uid/prom/resources/api/v1"
	d := profiles.Defaults()
	d.DisplayName = "Prod"
	d.EndpointURL = url
	d.ClusterName = "prod"
	d.AllowedNamespaces = []string{"payments"}
	raw, err := json.Marshal(d)
	if err != nil {
		t.Fatal(err)
	}
	document := strings.Replace(string(raw), "{", `{"endpoint_type":"grafana",`, 1)
	_, revision := seedProfile(ctx, t, pool, document)
	if _, err := provider.UpTo(ctx, 9); err != nil {
		t.Fatal(err)
	}
	var stored []byte
	if err := pool.QueryRow(ctx, `SELECT document::text FROM profile_revisions WHERE id = $1`, revision).Scan(&stored); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(stored), "endpoint_type") {
		t.Errorf("the revision still holds the endpoint type: %s", stored)
	}
	var back profiles.Document
	dec := json.NewDecoder(bytes.NewReader(stored))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&back); err != nil {
		t.Fatalf("the stored document does not decode strictly: %v", err)
	}
	if problems := back.Validate(); len(problems) > 0 {
		t.Errorf("the stored document is not valid: %v", problems)
	}
	if back.EndpointURL != url {
		t.Errorf("endpoint_url = %q, want %q", back.EndpointURL, url)
	}
	var enabled string
	if err := pool.QueryRow(ctx, `SELECT tgenabled FROM pg_trigger WHERE tgname = 'profile_revisions_are_immutable'`).Scan(&enabled); err != nil || enabled != "O" {
		t.Errorf("revision trigger enabled = %q, %v", enabled, err)
	}
}

// refuseSnapshot stores body at schema 3.0, expects migration 9 to fail and
// roll back, and checks that the row is as it was.
func refuseSnapshot(t *testing.T, body string) {
	t.Helper()
	ctx := context.Background()
	pool := storetest.NewEmpty(t)
	provider := migrateTo(ctx, t, pool, 8)
	owner, revision := seedProfile(ctx, t, pool, `{"display_name":"Prod"}`)
	report := seedSnapshot(ctx, t, pool, owner, revision, 1, body)
	_, err := provider.UpTo(ctx, 9)
	if err == nil {
		t.Fatal("a snapshot the migration cannot carry must stop it")
	}
	if !strings.Contains(err.Error(), report.String()) {
		t.Errorf("the refusal does not name report %s: %v", report, err)
	}
	if version, err := provider.GetDBVersion(ctx); err != nil || version != 8 {
		t.Fatalf("version after the refusal = %d, %v; want 8", version, err)
	}
	var version string
	var stored []byte
	if err := pool.QueryRow(ctx, `SELECT schema_version, body FROM report_snapshots WHERE report_id = $1`, report).Scan(&version, &stored); err != nil {
		t.Fatal(err)
	}
	if version != "3.0" || string(stored) != body {
		t.Errorf("the row changed: version %q, body equal %v", version, string(stored) == body)
	}
}

func TestMigrationNineRefusesASnapshotItCannotCarry(t *testing.T) {
	var out bytes.Buffer
	if err := json.Indent(&out, []byte(fileOf(t, "testdata/schema-3.0/direct-complete.json")), "", "    "); err != nil {
		t.Fatal(err)
	}
	refuseSnapshot(t, out.String())
}

func TestMigrationNineRefusesARepeatedEndpointTypeLine(t *testing.T) {
	line := "\n      \"endpoint_type\": \"prometheus\","
	refuseSnapshot(t, strings.Replace(fileOf(t, "testdata/schema-3.0/direct-complete.json"), line, line+line, 1))
}

func TestMigrationNineCannotBeReverted(t *testing.T) {
	ctx := context.Background()
	pool := storetest.New(t)
	db := stdlib.OpenDBFromPool(pool)
	defer func() { _ = db.Close() }()
	provider, err := goose.NewProvider(goose.DialectPostgres, db, os.DirFS("migrations"), goose.WithDisableGlobalRegistry(true))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = provider.Close() }()
	_, err = provider.Down(ctx)
	if err == nil || !strings.Contains(err.Error(), "this migration cannot be reverted") {
		t.Fatalf("down from 9 = %v, want the refusal", err)
	}
	if version, err := provider.GetDBVersion(ctx); err != nil || version != 9 {
		t.Fatalf("version after the refusal = %d, %v; want 9", version, err)
	}
}

func TestSnapshotTableRefusesAnotherSchemaAfterMigrationNine(t *testing.T) {
	ctx := context.Background()
	pool := storetest.NewEmpty(t)
	migrateTo(ctx, t, pool, 9)
	owner, revision := seedProfile(ctx, t, pool, `{"display_name":"Prod"}`)
	body := fileOf(t, "../../../testdata/snapshot/direct-complete.json")
	sum := sha256.Sum256([]byte(body))
	insert := func(job int, version string) error {
		report := uuid.New()
		if _, err := pool.Exec(ctx, `INSERT INTO reports (id, owner_id, profile_id, profile_revision_id, request, status, phase, collect_job_id, created_at, collected_at, expires_at, updated_at)
VALUES ($1, $2, 'prod', $3, '{}', 'collected', 'complete', $4, now(), now(), now() + interval '30 days', now())`, report, owner, revision, job); err != nil {
			t.Fatal(err)
		}
		_, err := pool.Exec(ctx, `INSERT INTO report_snapshots (report_id, schema_version, body, sha256, size_bytes, created_at)
VALUES ($1, $2, $3, $4, $5, now())`, report, version, []byte(body), hex.EncodeToString(sum[:]), len(body))
		return err
	}
	if code, _ := sqlState(t, insert(1, "3.0")); code != "23514" {
		t.Errorf("insert at 3.0 SQLSTATE = %s, want 23514", code)
	}
	if err := insert(2, "4.0"); err != nil {
		t.Errorf("insert at 4.0: %v", err)
	}
}
