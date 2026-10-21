//go:build postgres

package store_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/marceloalvescf/capacity-room/internal/platform/store"
	"github.com/marceloalvescf/capacity-room/internal/platform/storetest"
)

// homeReport inserts a report of the profile created at the given time,
// collected when status is "collected".
func homeReport(t *testing.T, pool *pgxpool.Pool, f reportFixture, profile string, revision uuid.UUID, status string, created time.Time) store.Report {
	t.Helper()
	r := queuedReport(f, profile, revision)
	r.CreatedAt, r.UpdatedAt = created, created
	r.ExpiresAt = created.Add(7 * 24 * time.Hour)
	if status == "collected" {
		r.Status, r.Phase, r.Attempt = "collected", "complete", 1
		r.CollectedAt, r.CompletedAt = &created, &created
	}
	mustInsertReport(t, pool, r)
	return r
}

func TestLatestReportsByProfileGivesBothReports(t *testing.T) {
	ctx := context.Background()
	pool := storetest.New(t)
	f := newReportFixture(t, pool)
	older := homeReport(t, pool, f, f.profile, f.revision, "collected", now)
	newer := homeReport(t, pool, f, f.profile, f.revision, "queued", now.Add(time.Minute))

	got, err := store.LatestReportsByProfile(ctx, pool, []string{f.profile}, now.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Latest == nil || got[0].Newest == nil {
		t.Fatalf("got %+v", got)
	}
	if got[0].Latest.ID != older.ID || got[0].Newest.ID != newer.ID {
		t.Fatalf("latest %v newest %v, want %v and %v", got[0].Latest.ID, got[0].Newest.ID, older.ID, newer.ID)
	}
	if got[0].Latest.OwnerUsername == "" || got[0].Newest.OwnerUsername != got[0].Latest.OwnerUsername {
		t.Fatalf("owners = %q and %q", got[0].Latest.OwnerUsername, got[0].Newest.OwnerUsername)
	}
}

func TestLatestReportsByProfileKeepsPDFsOfASharedReport(t *testing.T) {
	ctx := context.Background()
	pool := storetest.New(t)
	f := newReportFixture(t, pool)
	r := collectedReport(t, pool, f, now.Add(24*time.Hour), map[string]string{"en": "ready", "pt-BR": "failed"}, map[string]int64{"en": 1, "pt-BR": 2})

	got, err := store.LatestReportsByProfile(ctx, pool, []string{f.profile}, now)
	if err != nil {
		t.Fatal(err)
	}
	if got[0].Latest == nil || got[0].Newest == nil || got[0].Latest.ID != r.ID || got[0].Newest.ID != r.ID {
		t.Fatalf("got %+v", got)
	}
	if len(got[0].Latest.PDFs) != 2 || len(got[0].Newest.PDFs) != 2 {
		t.Fatalf("pdfs = %d and %d, want 2 and 2", len(got[0].Latest.PDFs), len(got[0].Newest.PDFs))
	}
}

func TestLatestReportsByProfileLeavesDeletedOut(t *testing.T) {
	ctx := context.Background()
	pool := storetest.New(t)
	f := newReportFixture(t, pool)
	kept := homeReport(t, pool, f, f.profile, f.revision, "collected", now)
	gone := homeReport(t, pool, f, f.profile, f.revision, "collected", now.Add(time.Minute))
	if _, err := pool.Exec(ctx, `UPDATE reports SET deleted_at = $2 WHERE id = $1`, gone.ID, now.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}

	got, err := store.LatestReportsByProfile(ctx, pool, []string{f.profile}, now.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if got[0].Latest == nil || got[0].Newest == nil || got[0].Latest.ID != kept.ID || got[0].Newest.ID != kept.ID {
		t.Fatalf("got %+v, want both %v", got[0], kept.ID)
	}
}

func TestLatestReportsByProfileLeavesExpiredOutOfLatest(t *testing.T) {
	ctx := context.Background()
	pool := storetest.New(t)
	f := newReportFixture(t, pool)
	atNow := homeReport(t, pool, f, f.profile, f.revision, "collected", now)
	if _, err := pool.Exec(ctx, `UPDATE reports SET expires_at = $2 WHERE id = $1`, atNow.ID, now.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	marked := homeReport(t, pool, f, f.profile, f.revision, "collected", now.Add(time.Minute))
	if _, err := pool.Exec(ctx, `UPDATE reports SET expired_at = $2 WHERE id = $1`, marked.ID, now.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}

	got, err := store.LatestReportsByProfile(ctx, pool, []string{f.profile}, now.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if got[0].Latest != nil {
		t.Fatalf("latest = %v, want nil", got[0].Latest.ID)
	}
	if got[0].Newest == nil || got[0].Newest.ID != marked.ID {
		t.Fatalf("newest = %+v, want %v", got[0].Newest, marked.ID)
	}
}

func TestLatestReportsByProfileKeepsTheGivenOrder(t *testing.T) {
	ctx := context.Background()
	pool := storetest.New(t)
	f := newReportFixture(t, pool)
	revA := addReportProfile(t, pool, "a")
	revB := addReportProfile(t, pool, "b")
	a := homeReport(t, pool, f, "a", revA, "queued", now)
	b := homeReport(t, pool, f, "b", revB, "queued", now)

	got, err := store.LatestReportsByProfile(ctx, pool, []string{"b", "a", "none"}, now)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 {
		t.Fatalf("len = %d, want 3", len(got))
	}
	if got[0].ProfileID != "b" || got[0].Newest == nil || got[0].Newest.ID != b.ID {
		t.Fatalf("entry 0 = %+v", got[0])
	}
	if got[1].ProfileID != "a" || got[1].Newest == nil || got[1].Newest.ID != a.ID {
		t.Fatalf("entry 1 = %+v", got[1])
	}
	if got[2].ProfileID != "none" || got[2].Latest != nil || got[2].Newest != nil {
		t.Fatalf("entry 2 = %+v", got[2])
	}
}

func TestLatestReportsByProfileUsesTheIndex(t *testing.T) {
	ctx := context.Background()
	pool := storetest.New(t)
	f := newReportFixture(t, pool)
	ids := make([]string, 20)
	for i := range ids {
		ids[i] = "p" + string(rune('a'+i))
		addReportProfile(t, pool, ids[i])
	}
	if _, err := pool.Exec(ctx, `
INSERT INTO reports (id, owner_id, profile_id, profile_revision_id, request, status, phase, attempt, collect_job_id,
                     created_at, expires_at, updated_at)
SELECT gen_random_uuid(), $1, p.id, (SELECT id FROM profile_revisions WHERE profile_id = p.id), '{}', 'queued', 'waiting', 0, 1,
       $2::timestamptz + n * interval '1 second', $2::timestamptz + interval '7 days', $2
FROM unnest($3::text[]) AS p(id), generate_series(1, 500) AS n`, f.owner, now, ids); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `ANALYZE reports`); err != nil {
		t.Fatal(err)
	}

	cfg := pool.Config()
	tracer := &queryTracer{}
	cfg.ConnConfig.Tracer = tracer
	traced, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(traced.Close)

	if _, err := store.LatestReportsByProfile(ctx, traced, ids, now); err != nil {
		t.Fatal(err)
	}
	tracer.mu.Lock()
	var sql string
	var args []any
	for i, s := range tracer.sqls {
		if strings.Contains(s, "LATERAL") {
			sql, args = s, tracer.args[i]
		}
	}
	tracer.mu.Unlock()
	if sql == "" {
		t.Fatal("the lateral query was not seen")
	}
	rows, err := pool.Query(ctx, "EXPLAIN "+sql, args...)
	if err != nil {
		t.Fatal(err)
	}
	var plan []string
	for rows.Next() {
		var line string
		if err := rows.Scan(&line); err != nil {
			t.Fatal(err)
		}
		plan = append(plan, line)
	}
	rows.Close()
	if text := strings.Join(plan, "\n"); !strings.Contains(text, "ix_reports_by_profile") {
		t.Fatalf("the plan does not use ix_reports_by_profile:\n%s", text)
	}
}
