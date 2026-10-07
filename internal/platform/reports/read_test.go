//go:build postgres

package reports

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"maps"
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/marceloalvescf/k8s-capacity-report/internal/platform/auth"
	"github.com/marceloalvescf/k8s-capacity-report/internal/platform/store"
)

func (f *fixture) exec(t *testing.T, sql string, args ...any) {
	t.Helper()
	if _, err := f.pool.Exec(context.Background(), sql, args...); err != nil {
		t.Fatal(err)
	}
}

func (f *fixture) setOwner(t *testing.T, id, owner uuid.UUID) {
	t.Helper()
	f.exec(t, `UPDATE reports SET owner_id = $2 WHERE id = $1`, id, owner)
}

func (f *fixture) setCreated(t *testing.T, id uuid.UUID, at time.Time) {
	t.Helper()
	f.exec(t, `UPDATE reports SET created_at = $2 WHERE id = $1`, id, at)
}

func (f *fixture) markDeleted(t *testing.T, id uuid.UUID) {
	t.Helper()
	err := store.Transact(context.Background(), f.pool, func(tx pgx.Tx) error {
		_, err := store.MarkReportDeleted(context.Background(), tx, id, fixedNow)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
}

func (f *fixture) addSnapshot(t *testing.T, id uuid.UUID, body string) {
	t.Helper()
	if err := store.InsertSnapshot(context.Background(), f.pool, id, "1", []byte(body), fixedNow); err != nil {
		t.Fatal(err)
	}
}

func (f *fixture) deletedAt(t *testing.T, id uuid.UUID) *time.Time {
	t.Helper()
	var at *time.Time
	if err := f.pool.QueryRow(context.Background(), `SELECT deleted_at FROM reports WHERE id = $1`, id).Scan(&at); err != nil {
		t.Fatal(err)
	}
	return at
}

func cursorOf(raw string) string { return base64.RawURLEncoding.EncodeToString([]byte(raw)) }

func sumOf(body string) string {
	sum := sha256.Sum256([]byte(body))
	return hex.EncodeToString(sum[:])
}

func TestListPagesWithTheCursor(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	var ids []uuid.UUID
	for i := range 5 {
		id := f.seedReport(t, "queued", "waiting", fixedNow.Add(time.Hour), "")
		f.setCreated(t, id, fixedNow.Add(-time.Duration(i)*time.Minute))
		ids = append(ids, id)
	}
	var got []uuid.UUID
	var sizes []int
	cursor := ""
	for range 5 {
		page, err := f.svc.List(ctx, f.read, cursor, 2)
		if err != nil {
			t.Fatal(err)
		}
		sizes = append(sizes, len(page.Items))
		for _, it := range page.Items {
			got = append(got, it.ID)
		}
		if page.NextCursor == nil {
			break
		}
		cursor = *page.NextCursor
	}
	if !slices.Equal(sizes, []int{2, 2, 1}) {
		t.Fatalf("page sizes %v", sizes)
	}
	if !slices.Equal(got, ids) {
		t.Fatalf("order %v, want %v", got, ids)
	}
}

func TestListRefusesABadCursor(t *testing.T) {
	f := newFixture(t)
	id := uuid.NewString()
	for name, cursor := range map[string]string{
		"not base64":   "!!",
		"no separator": cursorOf("x"),
		"bad id":       cursorOf("2026-10-06T12:00:00Z\nnot-a-uuid"),
		"no zone":      cursorOf("2026-10-06T12:00:00\n" + id),
		"not UTC":      cursorOf("2026-10-06T12:00:00+02:00\n" + id),
		"upper id":     cursorOf("2026-10-06T12:00:00Z\n" + "ABCDEF00-0000-0000-0000-000000000000"),
		"padded":       cursorOf("2026-10-06T12:00:00Z\n"+id) + "=",
		"extra line":   cursorOf("2026-10-06T12:00:00Z\n" + id + "\nx"),
	} {
		_, err := f.svc.List(context.Background(), f.read, cursor, 10)
		if e := asError(t, err); e.Code != CodeCursorInvalid {
			t.Errorf("%s: code %s", name, e.Code)
		}
	}
	valid := cursorOf("2026-10-06T12:00:00Z\n" + id)
	if _, err := f.svc.List(context.Background(), f.read, valid, 10); err != nil {
		t.Fatalf("a canonical cursor: %v", err)
	}
}

func TestListRefusesALimit(t *testing.T) {
	f := newFixture(t)
	for _, limit := range []int{0, -1, 201} {
		_, err := f.svc.List(context.Background(), f.read, "", limit)
		if e := asError(t, err); e.Code != CodeRequestInvalid || !maps.Equal(e.Fields, map[string]string{"limit": "invalid"}) {
			t.Errorf("limit %d: %+v", limit, e)
		}
	}
}

func TestListMarksExpired(t *testing.T) {
	f := newFixture(t)
	past := f.seedReport(t, "failed", "complete", fixedNow, "")
	marked := f.seedReport(t, "failed", "complete", fixedNow.Add(time.Hour), "")
	f.exec(t, `UPDATE reports SET expired_at = $2 WHERE id = $1`, marked, fixedNow)
	live := f.seedReport(t, "failed", "complete", fixedNow.Add(time.Hour), "")
	page, err := f.svc.List(context.Background(), f.read, "", 10)
	if err != nil {
		t.Fatal(err)
	}
	want := map[uuid.UUID]bool{past: true, marked: true, live: false}
	if len(page.Items) != 3 {
		t.Fatalf("%d items", len(page.Items))
	}
	for _, it := range page.Items {
		if it.Expired != want[it.ID] {
			t.Errorf("report %s expired %v", it.ID, it.Expired)
		}
	}
}

func TestListShowsTheRowsOfAReport(t *testing.T) {
	f := newFixture(t)
	id := f.seedReport(t, "collected", "complete", fixedNow.Add(time.Hour), "failed")
	page, err := f.svc.List(context.Background(), f.read, "", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Items) != 1 || page.NextCursor != nil {
		t.Fatalf("page %+v", page)
	}
	it := page.Items[0]
	code := "PDF_COMPILE_FAILED"
	if it.ID != id || it.OwnerID != f.op.ID || it.OwnerUsername != "ops" || it.ProfileID != "prod" || it.Status != "collected" ||
		it.Phase != "complete" || it.Expired || !it.CreatedAt.Equal(fixedNow) || it.CompletedAt == nil || !it.ExpiresAt.Equal(fixedNow.Add(time.Hour)) {
		t.Fatalf("item %+v", it)
	}
	want := []PDFState{{Language: "en", Status: "failed", FailureCode: &code}, {Language: "pt-BR", Status: "failed", FailureCode: &code}}
	if !slices.EqualFunc(it.PDFs, want, func(a, b PDFState) bool {
		return a.Language == b.Language && a.Status == b.Status && *a.FailureCode == *b.FailureCode
	}) {
		t.Fatalf("pdfs %+v", it.PDFs)
	}
}

func TestListLeavesOutDeletedReports(t *testing.T) {
	f := newFixture(t)
	gone := f.seedReport(t, "queued", "waiting", fixedNow.Add(time.Hour), "")
	f.seedReport(t, "queued", "waiting", fixedNow.Add(time.Hour), "")
	f.markDeleted(t, gone)
	page, err := f.svc.List(context.Background(), f.read, "", 10)
	if err != nil || len(page.Items) != 1 || page.Items[0].ID == gone {
		t.Fatalf("page %+v, %v", page, err)
	}
}

// readable seeds a collected report with its snapshot and two ready PDFs
// that another operator requested.
func (f *fixture) readable(t *testing.T) (id uuid.UUID, owner auth.Principal) {
	t.Helper()
	owner = newPrincipal(t, f.pool, "other", auth.RoleOperator)
	id = f.seedReport(t, "collected", "complete", fixedNow.Add(time.Hour), "ready")
	f.setOwner(t, id, owner.ID)
	f.addSnapshot(t, id, `{"k":1}`)
	return id, owner
}

func TestReadsByEveryRole(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	id, _ := f.readable(t)
	admin := newPrincipal(t, f.pool, "root", auth.RoleAdministrator)
	for _, actor := range []auth.Principal{f.read, f.op, admin} {
		detail, err := f.svc.Get(ctx, actor, id)
		if err != nil || detail.ID != id || detail.OwnerUsername != "other" {
			t.Fatalf("%s get: %+v, %v", actor.Role, detail, err)
		}
		snap, err := f.svc.Snapshot(ctx, actor, id)
		if err != nil || string(snap.Body) != `{"k":1}` || snap.SHA256 != sumOf(`{"k":1}`) {
			t.Fatalf("%s snapshot: %+v, %v", actor.Role, snap, err)
		}
		pdf, err := f.svc.PDF(ctx, actor, id, "pt-BR")
		if err != nil || string(pdf.Body) != "%PDF-1.7 body" || pdf.SHA256 != sumOf("%PDF-1.7 body") {
			t.Fatalf("%s pdf: %+v, %v", actor.Role, pdf, err)
		}
	}
}

func TestGetCarriesTheRequest(t *testing.T) {
	f := newFixture(t)
	id := f.seedReport(t, "running", "collecting", fixedNow.Add(time.Hour), "")
	f.exec(t, `UPDATE reports SET request = $2, attempt = 2, started_at = $3 WHERE id = $1`, id,
		`{"namespaces":["payments"],"period":"24h","timezone":"UTC","report_profile":"detailed","min_severity":"all","language":"pt-BR"}`, fixedNow)
	d, err := f.svc.Get(context.Background(), f.read, id)
	if err != nil {
		t.Fatal(err)
	}
	want := Resolved{Namespaces: []string{"payments"}, Period: "24h", Timezone: "UTC", ReportProfile: "detailed", MinSeverity: "all", Language: "pt-BR"}
	if d.Attempt != 2 || d.StartedAt == nil || d.CollectedAt != nil || d.ProfileRevisionID == uuid.Nil ||
		!slices.Equal(d.Selection.Namespaces, want.Namespaces) || d.Selection.Period != want.Period ||
		d.Selection.Timezone != want.Timezone || d.Selection.ReportProfile != want.ReportProfile || d.Selection.MinSeverity != want.MinSeverity ||
		d.Selection.Language != want.Language {
		t.Fatalf("detail %+v", d)
	}
}

func TestReadsJudgeInOrder(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	future := fixedNow.Add(time.Hour)
	deleted := f.seedReport(t, "collected", "complete", future, "ready")
	f.markDeleted(t, deleted)
	expiredDeleted := f.seedReport(t, "collected", "complete", fixedNow, "ready")
	f.markDeleted(t, expiredDeleted)
	expired := f.seedReport(t, "collected", "complete", fixedNow, "ready")
	marked := f.seedReport(t, "collected", "complete", future, "ready")
	f.exec(t, `UPDATE reports SET expired_at = $2 WHERE id = $1`, marked, fixedNow)
	queued := f.seedReport(t, "queued", "waiting", future, "")
	failed := f.seedReport(t, "failed", "complete", future, "")
	for _, c := range []struct {
		name string
		id   uuid.UUID
		code string
	}{
		{"absent", uuid.New(), CodeReportNotFound},
		{"deleted", deleted, CodeReportNotFound},
		{"expired and deleted", expiredDeleted, CodeReportNotFound},
		{"expired", expired, CodeReportExpired},
		{"marked expired", marked, CodeReportExpired},
		{"queued", queued, CodeSnapshotNotReady},
		{"failed", failed, CodeSnapshotNotReady},
	} {
		_, err := f.svc.Snapshot(ctx, f.read, c.id)
		if e := asError(t, err); e.Code != c.code {
			t.Errorf("snapshot of %s: %s", c.name, e.Code)
		}
		_, err = f.svc.PDF(ctx, f.read, c.id, "en")
		if e := asError(t, err); e.Code != c.code {
			t.Errorf("pdf of %s: %s", c.name, e.Code)
		}
		if c.code == CodeSnapshotNotReady {
			continue
		}
		_, err = f.svc.Get(ctx, f.read, c.id)
		if e := asError(t, err); e.Code != c.code {
			t.Errorf("get of %s: %s", c.name, e.Code)
		}
	}
	if _, err := f.svc.Get(ctx, f.read, queued); err != nil {
		t.Fatalf("get of a queued report: %v", err)
	}
}

func TestPDFNotReadyCarriesTheRow(t *testing.T) {
	f := newFixture(t)
	future := fixedNow.Add(time.Hour)
	pending := f.seedReport(t, "collected", "rendering", future, "pending")
	failed := f.seedReport(t, "collected", "complete", future, "failed")
	absent := f.seedReport(t, "collected", "complete", future, "")
	for name, c := range map[string]struct {
		id     uuid.UUID
		fields map[string]string
	}{
		"pending": {pending, map[string]string{"status": "pending"}},
		"failed":  {failed, map[string]string{"status": "failed", "failure_code": "PDF_COMPILE_FAILED"}},
		"absent":  {absent, map[string]string{"status": "pending"}},
	} {
		_, err := f.svc.PDF(context.Background(), f.read, c.id, "en")
		e := asError(t, err)
		if e.Code != CodePDFNotReady || !maps.Equal(e.Fields, c.fields) {
			t.Errorf("%s: %+v", name, e)
		}
	}
}

func TestPDFRefusesALanguage(t *testing.T) {
	f := newFixture(t)
	_, err := f.svc.PDF(context.Background(), f.read, uuid.New(), "fr")
	if e := asError(t, err); e.Code != CodeRequestInvalid || !maps.Equal(e.Fields, map[string]string{"language": "invalid"}) {
		t.Fatalf("%+v", e)
	}
}

func TestBodiesReadThroughTheDownloadPool(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	id, _ := f.readable(t)

	main, bodies := f.svc.Pool.Stat().AcquireCount(), f.svc.Bodies.Stat().AcquireCount()
	if _, err := f.svc.Snapshot(ctx, f.read, id); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.PDF(ctx, f.read, id, "en"); err != nil {
		t.Fatal(err)
	}
	if f.svc.Bodies == f.svc.Pool {
		t.Fatal("the service reads bodies from its main pool")
	}
	if got := f.svc.Bodies.Stat().AcquireCount() - bodies; got != 2 {
		t.Fatalf("the download pool acquired %d times, want 2", got)
	}
	if got := f.svc.Pool.Stat().AcquireCount() - main; got != 0 {
		t.Fatalf("the main pool acquired %d times, want 0", got)
	}
}
