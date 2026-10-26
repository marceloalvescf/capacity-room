//go:build postgres

package store_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"github.com/marceloalvescf/capacity-room/internal/platform/storetest"
)

// fileOf reads a fixture file whole.
func fileOf(t *testing.T, path string) string {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

type carriedReport struct {
	name string
	// old is the body as 2.8 stored it, and want the body the migration must write.
	old, want string
	request   string
	stored    *string
	wantWorst *string
	collected bool
	snapshot  bool
}

func str(s string) *string { return &s }

func carriedReports(t *testing.T) []carriedReport {
	t.Helper()
	var out []carriedReport
	for _, c := range []struct{ name, request string }{
		{"direct-complete", "all"},
		{"direct-complete-range", "all"},
		{"edge-scaling", "all"},
		{"edge-values", "critical"},
		{"grafana-authenticated", "all"},
		{"partial-data", "attention"},
		{"raw-fallback", "all"},
		{"thanos-multicluster", "all"},
		{"value-edge-cases", "all"},
	} {
		out = append(out, carriedReport{
			name:      c.name,
			old:       fileOf(t, "testdata/schema-2.0/"+c.name+".json"),
			want:      fileOf(t, "testdata/schema-3.0/"+c.name+".json"),
			request:   `{"language":"en","period":"24h","min_severity":"` + c.request + `"}`,
			stored:    str("critical"),
			wantWorst: str("critical"),
			collected: true, snapshot: true,
		})
	}

	// Every finding of attention at most: 2.8 stored healthy under the floor
	// "critical", and the report now shows its attention findings.
	attentionOld := strings.ReplaceAll(fileOf(t, "testdata/schema-2.0/direct-complete.json"), `"severity": "critical"`, `"severity": "attention"`)
	attentionWant := strings.ReplaceAll(fileOf(t, "testdata/schema-3.0/direct-complete.json"), `"severity": "critical"`, `"severity": "attention"`)
	out = append(out, carriedReport{
		name: "attention-only", old: attentionOld, want: attentionWant,
		request: `{"language":"en","period":"24h","min_severity":"critical"}`,
		stored:  str("healthy"), wantWorst: str("attention"), collected: true, snapshot: true,
	})

	// Every finding healthy: the report shows none that is worse.
	healthyOld := strings.ReplaceAll(attentionOld, `"severity": "attention"`, `"severity": "healthy"`)
	healthyWant := strings.ReplaceAll(attentionWant, `"severity": "attention"`, `"severity": "healthy"`)
	out = append(out, carriedReport{
		name: "healthy-only", old: healthyOld, want: healthyWant,
		request: `{"language":"en","period":"24h","min_severity":"all"}`,
		stored:  str("attention"), wantWorst: str("healthy"), collected: true, snapshot: true,
	})

	// jsonb refuses \u0000: the body is carried forward as text and the
	// stored worst severity is left as it was.
	nulOld := strings.Replace(fileOf(t, "testdata/schema-2.0/direct-complete.json"), `"organization": "ACME"`, `"organization": "A\u0000CME"`, 1)
	nulWant := strings.Replace(fileOf(t, "testdata/schema-3.0/direct-complete.json"), `"organization": "ACME"`, `"organization": "A\u0000CME"`, 1)
	out = append(out, carriedReport{
		name: "nul-escape", old: nulOld, want: nulWant,
		request: `{"language":"en","period":"24h","min_severity":"all"}`,
		stored:  str("attention"), wantWorst: str("attention"), collected: true, snapshot: true,
	})

	// A report collected before the summary existed keeps no worst severity.
	out = append(out, carriedReport{
		name: "no-summary", old: fileOf(t, "testdata/schema-2.0/direct-complete.json"), want: fileOf(t, "testdata/schema-3.0/direct-complete.json"),
		request:   `{"language":"en","period":"24h","min_severity":"all"}`,
		collected: true, snapshot: true,
	})

	// A report with no snapshot yet.
	out = append(out, carriedReport{name: "queued", request: `{"language":"en","period":"24h","min_severity":"critical"}`})
	return out
}

const oldDocument = `{"display_name":"Prod","default_period":"7d","default_min_severity":"critical","permitted_min_severities":["all","critical"]}`
const newDocument = `{"display_name":"Prod","default_period":"7d"}`

func TestMigrationEightCarriesStoredDataForward(t *testing.T) {
	ctx := context.Background()
	pool := storetest.NewEmpty(t)
	db := stdlib.OpenDBFromPool(pool)
	defer func() { _ = db.Close() }()
	provider, err := goose.NewProvider(goose.DialectPostgres, db, os.DirFS("migrations"), goose.WithDisableGlobalRegistry(true))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = provider.Close() }()
	if _, err := provider.UpTo(ctx, 7); err != nil {
		t.Fatal(err)
	}

	owner, err := addUser(ctx, pool, "operator", "en")
	if err != nil {
		t.Fatal(err)
	}
	revision := uuid.New()
	if _, err := pool.Exec(ctx, insertProfile, "prod", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO profile_revisions (id, profile_id, revision, fingerprint, display_name, active, document, credential_file, ca_bundle_file, created_by, created_at)
VALUES ($1, 'prod', 1, 'fingerprint-one', 'Prod', true, $2::jsonb, '', '', NULL, now())`, revision, oldDocument); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `UPDATE profiles SET current_revision_id = $1 WHERE id = 'prod'`, revision); err != nil {
		t.Fatal(err)
	}

	cases := carriedReports(t)
	ids := make([]uuid.UUID, len(cases))
	for i, c := range cases {
		ids[i] = uuid.New()
		status, phase, collectedAt := "queued", "waiting", (*time.Time)(nil)
		cluster := (*string)(nil)
		if c.collected {
			at := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
			status, phase, collectedAt = "collected", "complete", &at
		}
		if c.stored != nil {
			cluster = str("prod-cluster")
		}
		if _, err := pool.Exec(ctx, `INSERT INTO reports (id, owner_id, profile_id, profile_revision_id, request, status, phase, collect_job_id,
created_at, collected_at, expires_at, updated_at, worst_severity, cluster_name)
VALUES ($1, $2, 'prod', $3, $4::jsonb, $5, $6, $7, now(), $8, now() + interval '30 days', now(), $9, $10)`,
			ids[i], owner, revision, c.request, status, phase, i+1, collectedAt, c.stored, cluster); err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		if !c.snapshot {
			continue
		}
		sum := sha256.Sum256([]byte(c.old))
		if _, err := pool.Exec(ctx, `INSERT INTO report_snapshots (report_id, schema_version, body, sha256, size_bytes, created_at)
VALUES ($1, '2.0', $2, $3, $4, now())`, ids[i], []byte(c.old), hex.EncodeToString(sum[:]), len(c.old)); err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
	}

	if _, err := provider.UpTo(ctx, 8); err != nil {
		t.Fatalf("migrate to 8: %v", err)
	}

	for i, c := range cases {
		var worst *string
		var request string
		if err := pool.QueryRow(ctx, `SELECT worst_severity, request::text FROM reports WHERE id = $1`, ids[i]).Scan(&worst, &request); err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		switch {
		case c.wantWorst == nil && worst != nil, c.wantWorst != nil && (worst == nil || *worst != *c.wantWorst):
			t.Errorf("%s: worst_severity = %v, want %v", c.name, deref(worst), deref(c.wantWorst))
		}
		if strings.Contains(request, "min_severity") || !strings.Contains(request, `"period": "24h"`) {
			t.Errorf("%s: request = %s", c.name, request)
		}
		if !c.snapshot {
			continue
		}
		var version, hash string
		var body []byte
		var size int64
		if err := pool.QueryRow(ctx, `SELECT schema_version, body, sha256, size_bytes FROM report_snapshots WHERE report_id = $1`, ids[i]).Scan(&version, &body, &hash, &size); err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		sum := sha256.Sum256([]byte(c.want))
		if version != "3.0" || string(body) != c.want || hash != hex.EncodeToString(sum[:]) || size != int64(len(c.want)) {
			t.Errorf("%s: snapshot version %q, body equal %v, hash equal %v, size %d (want %d)",
				c.name, version, string(body) == c.want, hash == hex.EncodeToString(sum[:]), size, len(c.want))
		}
	}

	var documentCarried bool
	if err := pool.QueryRow(ctx, `SELECT document = $1::jsonb FROM profile_revisions WHERE id = $2`, newDocument, revision).Scan(&documentCarried); err != nil {
		t.Fatal(err)
	}
	if !documentCarried {
		t.Error("the profile revision still holds a removed key or lost another")
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
	_, err = pool.Exec(ctx, `UPDATE report_snapshots SET created_at = now()`)
	if code, _ := sqlState(t, err); code != "23000" {
		t.Errorf("snapshot update SQLSTATE = %s, want 23000", code)
	}
	_, err = pool.Exec(ctx, `UPDATE profile_revisions SET display_name = 'other'`)
	if code, _ := sqlState(t, err); code != "23000" {
		t.Errorf("revision update SQLSTATE = %s, want 23000", code)
	}
}

func deref(s *string) string {
	if s == nil {
		return "null"
	}
	return *s
}

func TestMigrationEightRefusesASnapshotItCannotCarry(t *testing.T) {
	ctx := context.Background()
	pool := storetest.NewEmpty(t)
	db := stdlib.OpenDBFromPool(pool)
	defer func() { _ = db.Close() }()
	provider, err := goose.NewProvider(goose.DialectPostgres, db, os.DirFS("migrations"), goose.WithDisableGlobalRegistry(true))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = provider.Close() }()
	if _, err := provider.UpTo(ctx, 7); err != nil {
		t.Fatal(err)
	}
	seedReport(ctx, t, pool, strings.Replace(fileOf(t, "testdata/schema-2.0/direct-complete.json"), "\n    \"min_severity\": \"all\",", "\n    \"min_severity\": \"all\",\n    \"min_severity\": \"all\",", 1))
	if _, err := provider.UpTo(ctx, 8); err == nil {
		t.Fatal("a snapshot that keeps its minimum severity must stop the migration")
	}
	if version, err := provider.GetDBVersion(ctx); err != nil || version != 7 {
		t.Fatalf("version after the refusal = %d, %v; want 7", version, err)
	}
}

// seedReport stores one collected report with a 2.0 snapshot holding body.
func seedReport(ctx context.Context, t *testing.T, pool *pgxpool.Pool, body string) {
	t.Helper()
	owner, err := addUser(ctx, pool, "operator", "en")
	if err != nil {
		t.Fatal(err)
	}
	revision, report := uuid.New(), uuid.New()
	for _, statement := range []struct {
		sql  string
		args []any
	}{
		{insertProfile, []any{"prod", nil}},
		{insertRevision, []any{revision, "prod", 1, "fingerprint-one"}},
		{`UPDATE profiles SET current_revision_id = $1 WHERE id = 'prod'`, []any{revision}},
		{`INSERT INTO reports (id, owner_id, profile_id, profile_revision_id, request, status, phase, collect_job_id, created_at, collected_at, expires_at, updated_at)
VALUES ($1, $2, 'prod', $3, '{}', 'collected', 'complete', 1, now(), now(), now() + interval '30 days', now())`, []any{report, owner, revision}},
	} {
		if _, err := pool.Exec(ctx, statement.sql, statement.args...); err != nil {
			t.Fatal(err)
		}
	}
	sum := sha256.Sum256([]byte(body))
	if _, err := pool.Exec(ctx, `INSERT INTO report_snapshots (report_id, schema_version, body, sha256, size_bytes, created_at)
VALUES ($1, '2.0', $2, $3, $4, now())`, report, []byte(body), hex.EncodeToString(sum[:]), len(body)); err != nil {
		t.Fatal(err)
	}
}

func TestMigrationEightCannotBeReverted(t *testing.T) {
	ctx := context.Background()
	pool := storetest.NewEmpty(t)
	provider := migrateTo(ctx, t, pool, 8)
	_, err := provider.Down(ctx)
	if err == nil || !strings.Contains(err.Error(), "this migration cannot be reverted") {
		t.Fatalf("down from 8 = %v, want the refusal", err)
	}
	if version, err := provider.GetDBVersion(ctx); err != nil || version != 8 {
		t.Fatalf("version after the refusal = %d, %v; want 8", version, err)
	}
}
