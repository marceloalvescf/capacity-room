//go:build postgres

package reports

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"

	"github.com/marceloalvescf/capacity-room/internal/platform/auth"
	"github.com/marceloalvescf/capacity-room/internal/platform/store"
)

func TestRequestStoresAndQueues(t *testing.T) {
	f := newFixture(t)
	revID := f.addProfile(t, "prod", baseDocument())
	got, err := f.svc.Request(context.Background(), f.op, "k1", f.selection("payments", "billing"))
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != "queued" || got.Phase != "waiting" || got.Replayed {
		t.Fatalf("accepted = %+v", got)
	}
	if want := fixedNow.Add(7 * 24 * time.Hour); !got.ExpiresAt.Equal(want) {
		t.Fatalf("expires_at = %v, want %v", got.ExpiresAt, want)
	}

	r, err := store.GetReport(context.Background(), f.pool, got.ID)
	if err != nil {
		t.Fatal(err)
	}
	if r.ProfileRevisionID != revID || r.ProfileID != "prod" || r.OwnerID != f.op.ID || r.Status != "queued" || r.Phase != "waiting" {
		t.Fatalf("report = %+v", r)
	}
	var stored Resolved
	if err := json.Unmarshal(r.Request, &stored); err != nil {
		t.Fatal(err)
	}
	want := Resolved{Namespaces: []string{"payments", "billing"}, Period: "24h", Timezone: "America/Sao_Paulo", ReportProfile: "detailed", Language: "en"}
	if !reflect.DeepEqual(stored, want) {
		t.Fatalf("stored selection = %+v, want %+v", stored, want)
	}

	jobs := f.jobs(t, "collect")
	if len(jobs) != 1 {
		t.Fatalf("collect jobs = %d", len(jobs))
	}
	j := jobs[0]
	if r.CollectJobID != j.ID || j.Queue != "reports" || j.Priority != 2 || j.MaxAttempts != 3 {
		t.Fatalf("job = %+v, report job id %d", j, r.CollectJobID)
	}
	var args CollectArgs
	if err := json.Unmarshal([]byte(j.Args), &args); err != nil || args.ReportID != got.ID {
		t.Fatalf("job args = %s (%v)", j.Args, err)
	}

	if n := f.count(t, `SELECT count(*) FROM report_idempotency_keys WHERE owner_id = $1 AND key = 'k1' AND report_id = $2`, f.op.ID, got.ID); n != 1 {
		t.Fatalf("idempotency rows = %d", n)
	}
	events := f.auditActions(t, ActionReportSubmit)
	if len(events) != 1 {
		t.Fatalf("submit rows = %d", len(events))
	}
	e := events[0]
	var meta map[string]any
	if err := json.Unmarshal(e.Metadata, &meta); err != nil {
		t.Fatal(err)
	}
	if e.ActorID == nil || *e.ActorID != f.op.ID || *e.TargetType != "report" || *e.TargetID != got.ID.String() ||
		meta["profile_id"] != "prod" || meta["revision_id"] != revID.String() || len(meta) != 2 {
		t.Fatalf("audit = %+v %v", e, meta)
	}
}

func TestRequestReplays(t *testing.T) {
	f := newFixture(t)
	f.addProfile(t, "prod", baseDocument())
	first, err := f.svc.Request(context.Background(), f.op, "k1", f.selection())
	if err != nil {
		t.Fatal(err)
	}
	second, err := f.svc.Request(context.Background(), f.op, "k1", f.selection())
	if err != nil {
		t.Fatal(err)
	}
	if second.ID != first.ID || !second.Replayed || first.Replayed {
		t.Fatalf("first %+v second %+v", first, second)
	}
	if !second.ExpiresAt.Equal(first.ExpiresAt) || second.Status != "queued" {
		t.Fatalf("second = %+v", second)
	}
	if f.count(t, `SELECT count(*) FROM reports`) != 1 || len(f.jobs(t, "collect")) != 1 ||
		f.count(t, `SELECT count(*) FROM report_idempotency_keys`) != 1 || len(f.auditActions(t, ActionReportSubmit)) != 1 {
		t.Fatal("a replay wrote a row")
	}
}

// A purge that commits after a replay read its idempotency row and report
// changes nothing: the replay answers for the expired report it read.
func TestRequestReplayOfAPurgedReport(t *testing.T) {
	f := newFixture(t)
	f.addProfile(t, "prod", baseDocument())
	ctx := context.Background()
	first, err := f.svc.Request(ctx, f.op, "k1", f.selection())
	if err != nil {
		t.Fatal(err)
	}
	err = store.Transact(ctx, f.pool, func(tx pgx.Tx) error {
		if err := store.ClaimReport(ctx, tx, first.ID, 1, fixedNow); err != nil {
			return err
		}
		return store.FailReport(ctx, tx, first.ID, 1, "INTERNAL_FAILURE", fixedNow)
	})
	if err != nil {
		t.Fatal(err)
	}
	expired := fixedNow.Add(8 * 24 * time.Hour)
	err = store.Transact(ctx, f.pool, func(tx pgx.Tx) error {
		_, err := store.ExpireDueReports(ctx, tx, expired, 100)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}

	purge := expired.Add(31 * 24 * time.Hour)
	f.svc.Now = func() time.Time { return purge }
	f.svc.afterKeyRead = func(ctx context.Context) {
		err := store.Transact(ctx, f.pool, func(tx pgx.Tx) error {
			ids, err := store.PurgeExpiredReports(ctx, tx, purge, 30*24*time.Hour, 100)
			if err == nil && (len(ids) != 1 || ids[0] != first.ID) {
				err = fmt.Errorf("purged %v", ids)
			}
			return err
		})
		if err != nil {
			t.Error(err)
		}
	}
	_, err = f.svc.Request(ctx, f.op, "k1", f.selection())
	if asError(t, err).Code != "REPORT_EXPIRED" {
		t.Fatalf("err = %v", err)
	}
	if n := f.count(t, `SELECT count(*) FROM reports`); n != 0 {
		t.Fatalf("%d reports", n)
	}
}

func TestRequestReplayWhileTheQueueIsFull(t *testing.T) {
	f := newFixture(t)
	f.svc.Settings.AdmissionLimit = 1
	f.svc.Settings.AdmissionProfileLimit = 1
	f.addProfile(t, "prod", baseDocument())
	first, err := f.svc.Request(context.Background(), f.op, "k1", f.selection())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.Request(context.Background(), f.op, "k2", f.selection()); asError(t, err).Code != "ADMISSION_GLOBAL_LIMIT" {
		t.Fatalf("another key was not refused: %v", err)
	}
	again, err := f.svc.Request(context.Background(), f.op, "k1", f.selection())
	if err != nil || again.ID != first.ID || !again.Replayed {
		t.Fatalf("replay = %+v, %v", again, err)
	}
}

func TestRequestSameKeyAnotherBody(t *testing.T) {
	f := newFixture(t)
	f.addProfile(t, "prod", baseDocument())
	if _, err := f.svc.Request(context.Background(), f.op, "k1", f.selection("payments")); err != nil {
		t.Fatal(err)
	}
	_, err := f.svc.Request(context.Background(), f.op, "k1", f.selection("billing"))
	if asError(t, err).Code != "IDEMPOTENCY_KEY_REUSED" {
		t.Fatalf("err = %v", err)
	}
	if f.count(t, `SELECT count(*) FROM reports`) != 1 || len(f.jobs(t, "collect")) != 1 || len(f.auditActions(t, ActionReportSubmit)) != 1 {
		t.Fatal("a refused request wrote a row")
	}
}

func TestKeyReusedWithAnotherLanguage(t *testing.T) {
	f := newFixture(t)
	f.addProfile(t, "prod", baseDocument())
	first := f.selection()
	first.Language = ptr("en")
	if _, err := f.svc.Request(context.Background(), f.op, "k1", first); err != nil {
		t.Fatal(err)
	}
	second := f.selection()
	second.Language = ptr("pt-BR")
	_, err := f.svc.Request(context.Background(), f.op, "k1", second)
	if asError(t, err).Code != "IDEMPOTENCY_KEY_REUSED" {
		t.Fatalf("err = %v", err)
	}
	if f.count(t, `SELECT count(*) FROM reports`) != 1 {
		t.Fatal("a refused request wrote a report")
	}
}

func TestRequestNeedsALanguage(t *testing.T) {
	f := newFixture(t)
	f.addProfile(t, "prod", baseDocument())
	sel := f.selection()
	sel.Language = nil
	_, err := f.svc.Request(context.Background(), f.op, "k", sel)
	e := asError(t, err)
	if e.Code != "REQUEST_INVALID" || !reflect.DeepEqual(e.Fields, map[string]string{"language": "missing"}) {
		t.Fatalf("error = %+v", e)
	}
	if f.count(t, `SELECT count(*) FROM reports`) != 0 {
		t.Fatal("a refused request wrote a report")
	}
}

func TestRequestRefusesAnUnknownLanguage(t *testing.T) {
	f := newFixture(t)
	f.addProfile(t, "prod", baseDocument())
	sel := f.selection()
	sel.Language = ptr("es")
	_, err := f.svc.Request(context.Background(), f.op, "k", sel)
	e := asError(t, err)
	if e.Code != "REQUEST_INVALID" || !reflect.DeepEqual(e.Fields, map[string]string{"language": "invalid"}) {
		t.Fatalf("error = %+v", e)
	}
}

func TestRequestStoresTheLanguage(t *testing.T) {
	f := newFixture(t)
	f.addProfile(t, "prod", baseDocument())
	sel := f.selection()
	sel.Language = ptr("en")
	acc, err := f.svc.Request(context.Background(), f.op, "k1", sel)
	if err != nil {
		t.Fatal(err)
	}
	got, err := f.svc.Get(context.Background(), f.op, acc.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Selection.Language != "en" {
		t.Fatalf("language %q", got.Selection.Language)
	}
}

func TestRequestSameKeyConcurrently(t *testing.T) {
	f := newFixture(t)
	f.addProfile(t, "prod", baseDocument())
	const n = 20
	results := make([]Accepted, n)
	errs := make([]error, n)
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			results[i], errs[i] = f.svc.Request(context.Background(), f.op, "same", f.selection())
		}()
	}
	close(start)
	wg.Wait()
	replayed := 0
	for i := 0; i < n; i++ {
		if errs[i] != nil {
			t.Fatalf("request %d: %v", i, errs[i])
		}
		if results[i].ID != results[0].ID {
			t.Fatalf("request %d answered another report", i)
		}
		if results[i].Replayed {
			replayed++
		}
	}
	if replayed != n-1 {
		t.Fatalf("replayed = %d, want %d", replayed, n-1)
	}
	if f.count(t, `SELECT count(*) FROM reports`) != 1 || len(f.jobs(t, "collect")) != 1 || len(f.auditActions(t, ActionReportSubmit)) != 1 {
		t.Fatal("more than one report, job, or audit row")
	}
}

func TestRequestKeyRules(t *testing.T) {
	f := newFixture(t)
	f.addProfile(t, "prod", baseDocument())
	valid := ""
	for i := 0; i < 128; i++ {
		valid += string("abcXYZ019._:-"[i%13])
	}
	cases := []struct {
		name, key string
		invalid   bool
	}{
		{"empty", "", true},
		{"129 characters", repeat("a", 129), true},
		{"space", "a b", true},
		{"128 of the alphabet", valid, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := f.svc.Request(context.Background(), f.op, c.key, f.selection())
			if c.invalid {
				if asError(t, err).Code != "IDEMPOTENCY_KEY_INVALID" {
					t.Fatalf("err = %v", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestRequestSyntax(t *testing.T) {
	f := newFixture(t)
	cases := []struct {
		name        string
		change      func(*Selection)
		field, word string
	}{
		{"profile missing", func(s *Selection) { s.ProfileID = "" }, "profile_id", "missing"},
		{"profile invalid", func(s *Selection) { s.ProfileID = "Bad Id" }, "profile_id", "invalid"},
		{"namespaces empty", func(s *Selection) { s.Namespaces = []string{} }, "namespaces", "empty"},
		{"namespace invalid", func(s *Selection) { s.Namespaces = []string{"Pay!"} }, "namespaces", "invalid"},
		{"namespace repeated", func(s *Selection) { s.Namespaces = []string{"a", "b", "a"} }, "namespaces", "repeated"},
		{"period invalid", func(s *Selection) { s.Period = ptr("abc") }, "period", "invalid"},
		{"period empty", func(s *Selection) { s.Period = ptr("") }, "period", "invalid"},
		{"timezone invalid", func(s *Selection) { s.Timezone = ptr("Nowhere/Land") }, "timezone", "invalid"},
		{"report profile invalid", func(s *Selection) { s.ReportProfile = ptr("full") }, "report_profile", "invalid"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			sel := f.selection()
			c.change(&sel)
			_, err := f.svc.Request(context.Background(), f.op, "k", sel)
			e := asError(t, err)
			if e.Code != "REQUEST_INVALID" || !reflect.DeepEqual(e.Fields, map[string]string{c.field: c.word}) {
				t.Fatalf("error = %+v", e)
			}
		})
	}
	if f.count(t, `SELECT count(*) FROM reports`) != 0 {
		t.Fatal("a refused request wrote a report")
	}
}

func TestRequestProfileStates(t *testing.T) {
	f := newFixture(t)
	inactive := baseDocument()
	inactive.Active = false
	f.addProfile(t, "idle", inactive)
	f.addProfile(t, "gone", baseDocument())
	if err := store.SetCurrentRevision(context.Background(), f.pool, "gone", nil, fixedNow); err != nil {
		t.Fatal(err)
	}
	cases := []struct{ id, code string }{
		{"ghost", "PROFILE_NOT_FOUND"},
		{"gone", "PROFILE_NOT_FOUND"},
		{"idle", "PROFILE_INACTIVE"},
	}
	for _, c := range cases {
		t.Run(c.id, func(t *testing.T) {
			sel := f.selection()
			sel.ProfileID = c.id
			_, err := f.svc.Request(context.Background(), f.op, "k-"+c.id, sel)
			if asError(t, err).Code != c.code {
				t.Fatalf("err = %v", err)
			}
		})
	}
	if f.count(t, `SELECT count(*) FROM reports`) != 0 || f.count(t, `SELECT count(*) FROM report_idempotency_keys`) != 0 {
		t.Fatal("a refused request wrote a row")
	}
}

func TestRequestSelectionNotPermitted(t *testing.T) {
	f := newFixture(t)
	d := baseDocument()
	d.PermittedPeriods = []string{"24h"}
	f.addProfile(t, "prod", d)

	_, err := f.svc.Request(context.Background(), f.op, "k1", f.selection("secrets"))
	if e := asError(t, err); e.Code != "REPORT_SELECTION_NOT_PERMITTED" || !reflect.DeepEqual(e.Fields, map[string]string{"namespaces": "not_permitted"}) {
		t.Fatalf("namespace: %+v", e)
	}
	sel := f.selection()
	sel.Period = ptr("7d")
	_, err = f.svc.Request(context.Background(), f.op, "k2", sel)
	if e := asError(t, err); e.Code != "REPORT_SELECTION_NOT_PERMITTED" || !reflect.DeepEqual(e.Fields, map[string]string{"period": "not_permitted"}) {
		t.Fatalf("period: %+v", e)
	}

	d2 := baseDocument()
	d2.PermittedPeriods = []string{"7d"}
	d2.DefaultPeriod = ptr("7d")
	f2 := newFixture(t)
	f2.addProfile(t, "prod", d2)
	got, err := f2.svc.Request(context.Background(), f2.op, "k3", f2.selection())
	if err != nil {
		t.Fatal(err)
	}
	r, err := store.GetReport(context.Background(), f2.pool, got.ID)
	if err != nil {
		t.Fatal(err)
	}
	var stored Resolved
	if err := json.Unmarshal(r.Request, &stored); err != nil || stored.Period != "7d" {
		t.Fatalf("stored = %s (%v)", r.Request, err)
	}
}

func TestAdmissionLimits(t *testing.T) {
	f := newFixture(t)
	f.svc.Settings.AdmissionLimit = 2
	f.svc.Settings.AdmissionProfileLimit = 1
	f.addProfile(t, "prod", baseDocument())
	f.addProfile(t, "dev", baseDocument())
	other := f.selection()
	other.ProfileID = "dev"

	first, err := f.svc.Request(context.Background(), f.op, "k1", f.selection())
	if err != nil {
		t.Fatal(err)
	}
	_, err = f.svc.Request(context.Background(), f.op, "k2", f.selection())
	if e := asError(t, err); e.Code != "ADMISSION_PROFILE_LIMIT" || e.RetryAfter != 5*time.Second {
		t.Fatalf("profile limit: %+v", e)
	}
	if _, err := f.svc.Request(context.Background(), f.op, "k3", other); err != nil {
		t.Fatal(err)
	}
	_, err = f.svc.Request(context.Background(), f.op, "k4", other)
	if e := asError(t, err); e.Code != "ADMISSION_GLOBAL_LIMIT" || e.RetryAfter != 5*time.Second {
		t.Fatalf("global limit: %+v", e)
	}

	// A collected or a failed report no longer counts.
	if _, err := f.pool.Exec(context.Background(),
		`UPDATE reports SET status = 'collected', phase = 'complete' WHERE id = $1`, first.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := f.pool.Exec(context.Background(),
		`UPDATE reports SET status = 'failed', phase = 'complete', failure_code = 'INTERNAL_FAILURE' WHERE profile_id = 'dev'`); err != nil {
		t.Fatal(err)
	}
	for i, key := range []string{"k5", "k6"} {
		sel := f.selection()
		if i == 1 {
			sel = other
		}
		if _, err := f.svc.Request(context.Background(), f.op, key, sel); err != nil {
			t.Fatalf("request %s: %v", key, err)
		}
	}
	if n := f.count(t, `SELECT count(*) FROM reports`); n != 4 {
		t.Fatalf("reports = %d, want 4", n)
	}
}

func TestAdmissionConcurrentExactlyLimit(t *testing.T) {
	f := newFixture(t)
	f.svc.Settings.AdmissionLimit = 5
	f.svc.Settings.AdmissionProfileLimit = 5
	f.addProfile(t, "prod", baseDocument())
	const n = 30
	errs := make([]error, n)
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			_, errs[i] = f.svc.Request(context.Background(), f.op, string(rune('a'+i%26))+string(rune('a'+i/26)), f.selection())
		}()
	}
	close(start)
	wg.Wait()
	accepted, refused := 0, 0
	for _, err := range errs {
		var e *Error
		switch {
		case err == nil:
			accepted++
		case errors.As(err, &e) && e.Code == "ADMISSION_GLOBAL_LIMIT":
			refused++
		default:
			t.Fatalf("unexpected error: %v", err)
		}
	}
	if accepted != 5 || refused != 25 {
		t.Fatalf("accepted %d refused %d", accepted, refused)
	}
	if f.count(t, `SELECT count(*) FROM reports`) != 5 || len(f.jobs(t, "collect")) != 5 {
		t.Fatal("rows do not match the accepted requests")
	}
}

func TestRequestRefusesAReader(t *testing.T) {
	f := newFixture(t)
	f.addProfile(t, "prod", baseDocument())
	_, err := f.svc.Request(context.Background(), f.read, "k1", f.selection())
	if !errors.Is(err, auth.ErrForbidden) {
		t.Fatalf("err = %v", err)
	}
	f.requireCounts(t, 0)
}

func TestRequestRollsBackOnAQueueFailure(t *testing.T) {
	f := newFixture(t)
	f.addProfile(t, "prod", baseDocument())
	// A client that has workers, none for the collect kind, refuses the insert.
	broken, err := river.NewClient(riverpgxv5.New(f.pool), &river.Config{
		Logger:  slog.New(slog.NewTextHandler(io.Discard, nil)),
		Workers: river.NewWorkers(),
	})
	if err != nil {
		t.Fatal(err)
	}
	queue := f.svc.Queue
	f.svc.Queue = broken
	_, err = f.svc.Request(context.Background(), f.op, "k1", f.selection())
	if err == nil || !strings.Contains(err.Error(), "queue collect") {
		t.Fatalf("err = %v", err)
	}
	f.svc.Queue = queue
	f.requireCounts(t, 0)
}

// requireCounts checks that exactly n requests left their rows.
func (f *fixture) requireCounts(t *testing.T, n int) {
	t.Helper()
	if got := f.count(t, `SELECT count(*) FROM reports`); got != n {
		t.Fatalf("reports = %d, want %d", got, n)
	}
	if got := f.count(t, `SELECT count(*) FROM report_idempotency_keys`); got != n {
		t.Fatalf("idempotency rows = %d, want %d", got, n)
	}
	if got := len(f.auditActions(t, ActionReportSubmit)); got != n {
		t.Fatalf("audit rows = %d, want %d", got, n)
	}
	if got := len(f.jobs(t, "collect")); got != n {
		t.Fatalf("collect jobs = %d, want %d", got, n)
	}
}

func TestRequestRollsBackTheJobOnALaterFailure(t *testing.T) {
	f := newFixture(t)
	f.addProfile(t, "prod", baseDocument())
	first, err := f.svc.Request(context.Background(), f.op, "k1", f.selection())
	if err != nil {
		t.Fatal(err)
	}
	// The second report takes the id of the first, so the report insert fails
	// on its primary key after the job row exists.
	f.svc.NewID = func() uuid.UUID { return first.ID }
	if _, err := f.svc.Request(context.Background(), f.op, "k2", f.selection()); err == nil {
		t.Fatal("no error")
	}
	f.requireCounts(t, 1)
}

func TestRequestStoresTheDigest(t *testing.T) {
	f := newFixture(t)
	f.addProfile(t, "prod", baseDocument())
	got, err := f.svc.Request(context.Background(), f.op, "k1", f.selection())
	if err != nil {
		t.Fatal(err)
	}
	var digest string
	err = f.pool.QueryRow(context.Background(), `SELECT request_sha256 FROM report_idempotency_keys WHERE report_id = $1`, got.ID).Scan(&digest)
	if err != nil {
		t.Fatal(err)
	}
	if digest != "74cd3960b2b7a10abbf912157fc707527a502e0a16203757545f1d6cfead31df" {
		t.Fatalf("digest = %s", digest)
	}
}

func TestRequestKeepsTheOrderOfTheNamespaces(t *testing.T) {
	f := newFixture(t)
	f.addProfile(t, "prod", baseDocument())
	got, err := f.svc.Request(context.Background(), f.op, "k1", f.selection("billing", "payments"))
	if err != nil {
		t.Fatal(err)
	}
	r, err := store.GetReport(context.Background(), f.pool, got.ID)
	if err != nil {
		t.Fatal(err)
	}
	var stored Resolved
	if err := json.Unmarshal(r.Request, &stored); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(stored.Namespaces, []string{"billing", "payments"}) {
		t.Fatalf("namespaces = %v", stored.Namespaces)
	}
}

func TestRequestAcceptsAnAdministrator(t *testing.T) {
	f := newFixture(t)
	f.addProfile(t, "prod", baseDocument())
	admin := newPrincipal(t, f.pool, "root", auth.RoleAdministrator)
	if _, err := f.svc.Request(context.Background(), admin, "k1", f.selection()); err != nil {
		t.Fatal(err)
	}
}

func TestRequestTakesTheRequestLock(t *testing.T) {
	f := newFixture(t)
	f.addProfile(t, "prod", baseDocument())
	ctx := context.Background()
	tx, err := f.pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := store.LockReportRequests(ctx, tx); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() {
		_, err := f.svc.Request(ctx, f.op, "k1", f.selection())
		done <- err
	}()
	select {
	case err := <-done:
		t.Fatalf("the request finished while the lock was held: %v", err)
	case <-time.After(500 * time.Millisecond):
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("the request did not finish after the lock was released")
	}
}

func rangeSelection(f *fixture, start, end string) Selection {
	sel := f.selection()
	sel.Start, sel.End = &start, &end
	return sel
}

func wantInvalid(t *testing.T, f *fixture, sel Selection, code, field, word string) {
	t.Helper()
	_, err := f.svc.Request(context.Background(), f.op, "k-"+field+word, sel)
	e := asError(t, err)
	if e.Code != code || !reflect.DeepEqual(e.Fields, map[string]string{field: word}) {
		t.Fatalf("error = %+v, want %s %s=%s", e, code, field, word)
	}
}

func TestRequestStoresARange(t *testing.T) {
	f := newFixture(t)
	f.addProfile(t, "prod", baseDocument())
	got, err := f.svc.Request(context.Background(), f.op, "k1", rangeSelection(f, "2026-09-01 00:00", "2026-10-01 00:00"))
	if err != nil {
		t.Fatal(err)
	}
	r, err := store.GetReport(context.Background(), f.pool, got.ID)
	if err != nil {
		t.Fatal(err)
	}
	var stored Resolved
	if err := json.Unmarshal(r.Request, &stored); err != nil {
		t.Fatal(err)
	}
	want := Resolved{Namespaces: []string{"payments"}, Period: "", Start: "2026-09-01 00:00", End: "2026-10-01 00:00",
		Timezone: "America/Sao_Paulo", ReportProfile: "detailed", Language: "en"}
	if !reflect.DeepEqual(stored, want) {
		t.Fatalf("stored selection = %+v, want %+v", stored, want)
	}
}

func TestRequestRangeShape(t *testing.T) {
	f := newFixture(t)
	f.addProfile(t, "prod", baseDocument())
	sel := f.selection()
	sel.Start, sel.End = ptr("2026-9-1 0:00"), ptr("2026-10-01 00:00")
	wantInvalid(t, f, sel, "REQUEST_INVALID", "start", "invalid")
}

func TestRequestRangeNeedsBothEnds(t *testing.T) {
	f := newFixture(t)
	f.addProfile(t, "prod", baseDocument())
	sel := f.selection()
	sel.Start = ptr("2026-09-01 00:00")
	wantInvalid(t, f, sel, "REQUEST_INVALID", "end", "missing")
	sel = f.selection()
	sel.End = ptr("2026-09-01 00:00")
	wantInvalid(t, f, sel, "REQUEST_INVALID", "start", "missing")
}

func TestRequestRangeWithAPeriod(t *testing.T) {
	f := newFixture(t)
	f.addProfile(t, "prod", baseDocument())
	sel := rangeSelection(f, "2026-09-01 00:00", "2026-10-01 00:00")
	sel.Period = ptr("7d")
	wantInvalid(t, f, sel, "REQUEST_INVALID", "period", "conflict")
}

func TestRequestRangeOrder(t *testing.T) {
	f := newFixture(t)
	f.addProfile(t, "prod", baseDocument())
	wantInvalid(t, f, rangeSelection(f, "2026-10-01 00:00", "2026-09-01 00:00"), "REQUEST_INVALID", "start", "order")
	wantInvalid(t, f, rangeSelection(f, "2026-09-01 00:00", "2026-09-01 00:00"), "REQUEST_INVALID", "start", "order")
}

func TestRequestRangeInAGap(t *testing.T) {
	f := newFixture(t)
	f.addProfile(t, "prod", baseDocument())
	sel := rangeSelection(f, "2026-03-08 02:30", "2026-03-08 04:00")
	sel.Timezone = ptr("America/New_York")
	wantInvalid(t, f, sel, "REQUEST_INVALID", "start", "invalid")
}

func atWall(t *testing.T, zone, wall string) time.Time {
	t.Helper()
	loc, err := time.LoadLocation(zone)
	if err != nil {
		t.Fatal(err)
	}
	at, err := time.ParseInLocation("2006-01-02 15:04:05", wall, loc)
	if err != nil {
		t.Fatal(err)
	}
	return at
}

func TestRequestRangeEndingNow(t *testing.T) {
	f := newFixture(t)
	f.addProfile(t, "prod", baseDocument())
	now := atWall(t, "America/Sao_Paulo", "2026-10-07 18:00:42")
	f.svc.Now = func() time.Time { return now }
	if _, err := f.svc.Request(context.Background(), f.op, "k1", rangeSelection(f, "2026-10-07 00:00", "2026-10-07 18:00")); err != nil {
		t.Fatal(err)
	}
}

func TestRequestRangeInTheFuture(t *testing.T) {
	f := newFixture(t)
	f.addProfile(t, "prod", baseDocument())
	now := atWall(t, "America/Sao_Paulo", "2026-10-07 18:00:42")
	f.svc.Now = func() time.Time { return now }
	wantInvalid(t, f, rangeSelection(f, "2026-10-07 00:00", "2026-10-07 18:01"), "REQUEST_INVALID", "end", "future")
}

func TestRequestRangeUnderPermittedPeriods(t *testing.T) {
	f := newFixture(t)
	d := baseDocument()
	d.PermittedPeriods = []string{"24h"}
	f.addProfile(t, "prod", d)
	wantInvalid(t, f, rangeSelection(f, "2026-09-01 00:00", "2026-10-01 00:00"), "REPORT_SELECTION_NOT_PERMITTED", "start", "not_permitted")
}

func TestRequestRangeWordsBeforePermits(t *testing.T) {
	f := newFixture(t)
	d := baseDocument()
	d.PermittedPeriods = []string{"24h"}
	f.addProfile(t, "prod", d)
	wantInvalid(t, f, rangeSelection(f, "2026-10-01 00:00", "2026-09-01 00:00"), "REQUEST_INVALID", "start", "order")
}

func TestKeyReusedWithAnotherRange(t *testing.T) {
	f := newFixture(t)
	f.addProfile(t, "prod", baseDocument())
	if _, err := f.svc.Request(context.Background(), f.op, "k1", rangeSelection(f, "2026-09-01 00:00", "2026-10-01 00:00")); err != nil {
		t.Fatal(err)
	}
	_, err := f.svc.Request(context.Background(), f.op, "k1", rangeSelection(f, "2026-09-01 00:00", "2026-09-30 00:00"))
	if e := asError(t, err); e.Code != CodeIdempotencyKeyReused {
		t.Fatalf("error = %+v", e)
	}
}

func TestRequestStoresANullNamespaceSelection(t *testing.T) {
	f := newFixture(t)
	f.addProfile(t, "prod", baseDocument())
	sel := f.selection()
	sel.Namespaces = nil
	got, err := f.svc.Request(context.Background(), f.op, "k-null", sel)
	if err != nil {
		t.Fatal(err)
	}
	var stored string
	if err := f.pool.QueryRow(context.Background(),
		`SELECT request->'namespaces'::text FROM reports WHERE id = $1`, got.ID).Scan(&stored); err != nil {
		t.Fatal(err)
	}
	if stored != "null" {
		t.Fatalf("stored namespaces = %s, want null", stored)
	}
	again, err := f.svc.Request(context.Background(), f.op, "k-null", f.selection())
	if e := asError(t, err); e.Code != "IDEMPOTENCY_KEY_REUSED" {
		t.Fatalf("a list under the key of a null selection: %+v, %+v", again, e)
	}
}
