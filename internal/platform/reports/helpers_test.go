//go:build postgres

package reports

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/marceloalvescf/k8s-capacity-report/internal/platform/auth"
	"github.com/marceloalvescf/k8s-capacity-report/internal/platform/profiles"
	"github.com/marceloalvescf/k8s-capacity-report/internal/platform/settings"
	"github.com/marceloalvescf/k8s-capacity-report/internal/platform/store"
	"github.com/marceloalvescf/k8s-capacity-report/internal/platform/storetest"
)

var fixedNow = time.Date(2026, 10, 6, 12, 0, 0, 0, time.UTC)

type fixture struct {
	pool *pgxpool.Pool
	svc  *Service
	op   auth.Principal
	read auth.Principal
}

func testSettings() settings.Settings {
	return settings.Settings{
		JobMaxAttempts:        3,
		ReportRetention:       7 * 24 * time.Hour,
		AdmissionLimit:        20,
		AdmissionProfileLimit: 5,
		AdmissionRetry:        5 * time.Second,
	}
}

func newPrincipal(t *testing.T, pool *pgxpool.Pool, name, role string) auth.Principal {
	t.Helper()
	u := store.User{ID: uuid.New(), Username: name, PasswordHash: "x", Role: role, Language: "en", Active: true, CreatedAt: fixedNow, UpdatedAt: fixedNow}
	if err := store.InsertUser(context.Background(), pool, u); err != nil {
		t.Fatal(err)
	}
	return auth.Principal{ID: u.ID, Username: name, Role: role, Language: "en"}
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	pool := storetest.New(t)
	queue, err := NewQueue(pool, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	bodies, err := store.OpenDownloads(context.Background(), pool.Config().ConnString())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(bodies.Close)
	f := &fixture{pool: pool}
	f.op = newPrincipal(t, pool, "ops", auth.RoleOperator)
	f.read = newPrincipal(t, pool, "eve", auth.RoleReader)
	f.svc = &Service{
		Pool: pool, Bodies: bodies, Queue: queue, Settings: testSettings(),
		Now: func() time.Time { return fixedNow },
	}
	return f
}

func baseDocument() profiles.Document {
	d := profiles.Defaults()
	d.DisplayName = "Production"
	d.Active = true
	d.EndpointURL = "https://metrics.test/api/v1"
	d.EndpointType = "prometheus"
	d.ClusterName = "prod"
	d.AllowedNamespaces = []string{"payments", "billing"}
	return d
}

// addProfile writes a profile with one revision and returns the revision id.
func (f *fixture) addProfile(t *testing.T, id string, d profiles.Document) uuid.UUID {
	t.Helper()
	raw, err := json.Marshal(d)
	if err != nil {
		t.Fatal(err)
	}
	var revID uuid.UUID
	err = store.Transact(context.Background(), f.pool, func(tx pgx.Tx) error {
		if _, err := store.GetOrCreateProfile(context.Background(), tx, id, fixedNow); err != nil {
			return err
		}
		rev, err := store.AppendProfileRevision(context.Background(), tx, store.ProfileRevision{
			ID: uuid.New(), ProfileID: id, Fingerprint: uuid.NewString(), DisplayName: d.DisplayName,
			Active: d.Active, Document: raw, CreatedAt: fixedNow,
		})
		if err != nil {
			return err
		}
		revID = rev.ID
		return store.SetCurrentRevision(context.Background(), tx, id, &rev.ID, fixedNow)
	})
	if err != nil {
		t.Fatal(err)
	}
	return revID
}

func (f *fixture) selection(namespaces ...string) Selection {
	if len(namespaces) == 0 {
		namespaces = []string{"payments"}
	}
	return Selection{ProfileID: "prod", Namespaces: namespaces, Language: ptr("en")}
}

func (f *fixture) count(t *testing.T, query string, args ...any) int {
	t.Helper()
	var n int
	if err := f.pool.QueryRow(context.Background(), query, args...).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func (f *fixture) auditActions(t *testing.T, action string) []store.AuditEvent {
	t.Helper()
	events, err := store.ListAuditEvents(context.Background(), f.pool)
	if err != nil {
		t.Fatal(err)
	}
	var out []store.AuditEvent
	for _, e := range events {
		if e.Action == action {
			out = append(out, e)
		}
	}
	return out
}

type riverRow struct {
	ID          int64
	Kind, Queue string
	Priority    int
	MaxAttempts int
	Args        string
}

func (f *fixture) jobs(t *testing.T, kind string) []riverRow {
	t.Helper()
	rows, err := f.pool.Query(context.Background(),
		`SELECT id, kind, queue, priority, max_attempts, args::text FROM river_job WHERE kind = $1 ORDER BY id`, kind)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out []riverRow
	for rows.Next() {
		var r riverRow
		if err := rows.Scan(&r.ID, &r.Kind, &r.Queue, &r.Priority, &r.MaxAttempts, &r.Args); err != nil {
			t.Fatal(err)
		}
		out = append(out, r)
	}
	return out
}

func ptr(s string) *string { return &s }

func asError(t *testing.T, err error) *Error {
	t.Helper()
	var e *Error
	if err == nil {
		t.Fatal("no error")
	}
	if !errors.As(err, &e) {
		t.Fatalf("error %v is not a reports.Error", err)
	}
	return e
}

func repeat(c string, n int) string { return strings.Repeat(c, n) }
