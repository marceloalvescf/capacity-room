//go:build postgres

package reports

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"slices"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/marceloalvescf/k8s-capacity-report/internal/platform/auth"
	"github.com/marceloalvescf/k8s-capacity-report/internal/platform/store"
)

func TestDeleteByRole(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	future := fixedNow.Add(time.Hour)
	other := newPrincipal(t, f.pool, "other", auth.RoleOperator)
	admin := newPrincipal(t, f.pool, "root", auth.RoleAdministrator)

	own := f.seedReport(t, "queued", "waiting", future, "")
	foreign := f.seedReport(t, "queued", "waiting", future, "")
	f.setOwner(t, foreign, other.ID)

	if err := f.svc.Delete(ctx, f.read, own); !errors.Is(err, auth.ErrForbidden) {
		t.Fatalf("reader: %v", err)
	}
	if err := f.svc.Delete(ctx, f.op, foreign); !errors.Is(err, auth.ErrForbidden) {
		t.Fatalf("operator on another's: %v", err)
	}
	if f.deletedAt(t, own) != nil || f.deletedAt(t, foreign) != nil {
		t.Fatal("a refused delete marked a report")
	}
	if err := f.svc.Delete(ctx, f.op, own); err != nil {
		t.Fatalf("operator on its own: %v", err)
	}
	if err := f.svc.Delete(ctx, admin, foreign); err != nil {
		t.Fatalf("administrator on another's: %v", err)
	}
	for _, id := range []uuid.UUID{own, foreign} {
		at := f.deletedAt(t, id)
		if at == nil || !at.Equal(fixedNow) {
			t.Fatalf("deleted_at of %s is %v", id, at)
		}
	}
}

func TestDeleteJudgesInOrder(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	deleted := f.seedReport(t, "queued", "waiting", fixedNow.Add(time.Hour), "")
	f.markDeleted(t, deleted)
	expired := f.seedReport(t, "failed", "complete", fixedNow, "")
	marked := f.seedReport(t, "failed", "complete", fixedNow.Add(time.Hour), "")
	f.exec(t, `UPDATE reports SET expired_at = $2 WHERE id = $1`, marked, fixedNow)
	before := f.count(t, `SELECT count(*) FROM audit_events`)

	for name, c := range map[string]struct {
		actor auth.Principal
		id    uuid.UUID
		code  string
	}{
		"absent":         {f.op, uuid.New(), CodeReportNotFound},
		"deleted":        {f.op, deleted, CodeReportNotFound},
		"expired":        {f.op, expired, CodeReportExpired},
		"marked expired": {f.op, marked, CodeReportExpired},
	} {
		if e := asError(t, f.svc.Delete(ctx, c.actor, c.id)); e.Code != c.code {
			t.Errorf("%s: %s", name, e.Code)
		}
	}
	if f.deletedAt(t, expired) != nil || f.deletedAt(t, marked) != nil {
		t.Fatal("an expired report was marked")
	}
	if f.count(t, `SELECT count(*) FROM audit_events`) != before {
		t.Fatal("a refused delete wrote an audit row")
	}
}

func TestDeleteAudits(t *testing.T) {
	f := newFixture(t)
	id := f.seedReport(t, "queued", "waiting", fixedNow.Add(time.Hour), "")
	if err := f.svc.Delete(context.Background(), f.op, id); err != nil {
		t.Fatal(err)
	}
	rows := f.auditActions(t, ActionReportDelete)
	if len(rows) != 1 || rows[0].ActorID == nil || *rows[0].ActorID != f.op.ID || rows[0].TargetType == nil ||
		*rows[0].TargetType != "report" || *rows[0].TargetID != id.String() || rows[0].Outcome != "succeeded" {
		t.Fatalf("audit %+v", rows)
	}
	var meta map[string]any
	if err := json.Unmarshal(rows[0].Metadata, &meta); err != nil || len(meta) != 1 || meta["status"] != "queued" {
		t.Fatalf("metadata %s, %v", rows[0].Metadata, err)
	}
}

func idsOf(results []DeleteResult) []uuid.UUID {
	out := make([]uuid.UUID, len(results))
	for i, r := range results {
		out[i] = r.ID
	}
	return out
}

func TestDeleteManyOutcomes(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	future := fixedNow.Add(time.Hour)
	other := newPrincipal(t, f.pool, "other", auth.RoleOperator)

	own := f.seedReport(t, "queued", "waiting", future, "")
	foreign := f.seedReport(t, "queued", "waiting", future, "")
	f.setOwner(t, foreign, other.ID)
	expired := f.seedReport(t, "failed", "complete", fixedNow, "")
	absent := uuid.New()
	already := f.seedReport(t, "queued", "waiting", future, "")
	f.markDeleted(t, already)

	ids := []uuid.UUID{own, foreign, expired, absent, already}
	want := []DeleteOutcome{OutcomeDeleted, OutcomeForbidden, OutcomeExpired, OutcomeNotFound, OutcomeNotFound}
	for len(ids) < 200 {
		id := f.seedReport(t, "queued", "waiting", future, "")
		ids = append(ids, id)
		want = append(want, OutcomeDeleted)
	}
	results, err := f.svc.DeleteMany(ctx, f.op, ids)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(idsOf(results), ids) {
		t.Fatal("the answers are not in the order given")
	}
	for i, r := range results {
		if r.Outcome != want[i] {
			t.Fatalf("id %d: %s, want %s", i, r.Outcome, want[i])
		}
	}
	if n := len(f.auditActions(t, ActionReportDelete)); n != 196 {
		t.Fatalf("%d audit rows, want 196", n)
	}
	if f.deletedAt(t, foreign) != nil || f.deletedAt(t, expired) != nil {
		t.Fatal("a refused id was marked")
	}
}

func TestDeleteManyRefusesAReader(t *testing.T) {
	f := newFixture(t)
	id := f.seedReport(t, "queued", "waiting", fixedNow.Add(time.Hour), "")
	if _, err := f.svc.DeleteMany(context.Background(), f.read, []uuid.UUID{id}); !errors.Is(err, auth.ErrForbidden) {
		t.Fatalf("reader: %v", err)
	}
	if f.deletedAt(t, id) != nil {
		t.Fatal("a reader deleted a report")
	}
}

func TestDeleteManyRefusesTheBody(t *testing.T) {
	f := newFixture(t)
	id := f.seedReport(t, "queued", "waiting", fixedNow.Add(time.Hour), "")
	many := make([]uuid.UUID, 201)
	for i := range many {
		many[i] = uuid.New()
	}
	many[0] = id
	for name, c := range map[string]struct {
		ids  []uuid.UUID
		word string
	}{
		"empty":    {nil, "empty"},
		"too many": {many, "too_many"},
		"repeated": {[]uuid.UUID{id, uuid.New(), id}, "repeated"},
	} {
		_, err := f.svc.DeleteMany(context.Background(), f.op, c.ids)
		if e := asError(t, err); e.Code != CodeRequestInvalid || !maps.Equal(e.Fields, map[string]string{"ids": c.word}) {
			t.Errorf("%s: %+v", name, e)
		}
	}
	if f.deletedAt(t, id) != nil || len(f.auditActions(t, ActionReportDelete)) != 0 {
		t.Fatal("a refused body wrote something")
	}
}

func TestDeleteManyConcurrent(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	adminA := newPrincipal(t, f.pool, "root-a", auth.RoleAdministrator)
	adminB := newPrincipal(t, f.pool, "root-b", auth.RoleAdministrator)
	ids := make([]uuid.UUID, 50)
	for i := range ids {
		ids[i] = f.seedReport(t, "queued", "waiting", fixedNow.Add(time.Hour), "")
	}

	// Both calls judge every id before either marks one. The second call
	// sends the ids reversed, so only a fixed marking order avoids a deadlock.
	reversed := slices.Clone(ids)
	slices.Reverse(reversed)
	var arrivals atomic.Int32
	bothArrived := make(chan struct{})
	f.svc.beforeMark = func(context.Context) {
		if arrivals.Add(1) == 2 {
			close(bothArrived)
		}
		select {
		case <-bothArrived:
		case <-time.After(10 * time.Second):
			t.Error("the other call never reached the mark")
		}
	}
	sent := [][]uuid.UUID{ids, reversed}
	answers := make([]map[uuid.UUID]DeleteOutcome, 2)
	errs := make([]error, 2)
	var done sync.WaitGroup
	for i, actor := range []auth.Principal{adminA, adminB} {
		done.Add(1)
		go func() {
			defer done.Done()
			results, err := f.svc.DeleteMany(ctx, actor, sent[i])
			errs[i] = err
			answers[i] = map[uuid.UUID]DeleteOutcome{}
			for j, r := range results {
				if r.ID != sent[i][j] {
					t.Errorf("call %d: answer %d is out of order", i, j)
				}
				answers[i][r.ID] = r.Outcome
			}
		}()
	}
	done.Wait()
	for _, err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	for i, id := range ids {
		pair := []DeleteOutcome{answers[0][id], answers[1][id]}
		slices.Sort(pair)
		if !slices.Equal(pair, []DeleteOutcome{OutcomeDeleted, OutcomeNotFound}) {
			t.Fatalf("id %d answered %v", i, pair)
		}
	}
	if n := len(f.auditActions(t, ActionReportDelete)); n != 50 {
		t.Fatalf("%d audit rows, want 50", n)
	}
	var marked int
	if err := f.pool.QueryRow(ctx, `SELECT count(*) FROM reports WHERE deleted_at IS NOT NULL`).Scan(&marked); err != nil || marked != 50 {
		t.Fatalf("%d marked, %v", marked, err)
	}
}

func TestReplayOfADeletedReport(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	f.addProfile(t, "prod", baseDocument())
	acc, err := f.svc.Request(ctx, f.op, "k1", f.selection())
	if err != nil {
		t.Fatal(err)
	}
	if err := f.svc.Delete(ctx, f.op, acc.ID); err != nil {
		t.Fatal(err)
	}
	_, err = f.svc.Request(ctx, f.op, "k1", f.selection())
	if e := asError(t, err); e.Code != CodeReportNotFound {
		t.Fatalf("replay: %+v", e)
	}
	if n := f.count(t, `SELECT count(*) FROM reports`); n != 1 {
		t.Fatalf("%d reports", n)
	}
}

// After the sweep a deleted report's key is free: the same key and body make
// a new report.
func TestKeyIsFreeAfterTheSweep(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	f.addProfile(t, "prod", baseDocument())
	first, err := f.svc.Request(ctx, f.op, "k1", f.selection())
	if err != nil {
		t.Fatal(err)
	}
	if err := f.svc.Delete(ctx, f.op, first.ID); err != nil {
		t.Fatal(err)
	}
	sweep := func() {
		err := store.Transact(ctx, f.pool, func(tx pgx.Tx) error {
			ids, err := store.SweepDeletedReports(ctx, tx, 100)
			if err == nil && !slices.Equal(ids, []uuid.UUID{first.ID}) {
				err = fmt.Errorf("swept %v", ids)
			}
			return err
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	_, err = f.svc.Request(ctx, f.op, "k1", f.selection())
	if e := asError(t, err); e.Code != CodeReportNotFound {
		t.Fatalf("replay before the sweep: %+v", e)
	}
	sweep()
	second, err := f.svc.Request(ctx, f.op, "k1", f.selection())
	if err != nil {
		t.Fatalf("replay after the sweep: %v", err)
	}
	if second.ID == first.ID || second.Replayed {
		t.Fatalf("after the sweep: id %v (first %v), replayed %v", second.ID, first.ID, second.Replayed)
	}
}

func TestReplayOfAnExpiredReport(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	f.addProfile(t, "prod", baseDocument())
	if _, err := f.svc.Request(ctx, f.op, "k1", f.selection()); err != nil {
		t.Fatal(err)
	}
	f.svc.Now = func() time.Time { return fixedNow.Add(8 * 24 * time.Hour) }
	_, err := f.svc.Request(ctx, f.op, "k1", f.selection())
	if e := asError(t, err); e.Code != CodeReportExpired {
		t.Fatalf("replay: %+v", e)
	}
}

func TestRetryOfADeletedReport(t *testing.T) {
	f := newFixture(t)
	id := f.seedReport(t, "collected", "complete", fixedNow.Add(time.Hour), "failed")
	f.markDeleted(t, id)
	err := f.svc.RetryPDF(context.Background(), f.op, id, "en")
	if e := asError(t, err); e.Code != CodeReportNotFound {
		t.Fatalf("%+v", e)
	}
	row, err := store.GetPDF(context.Background(), f.pool, id, "en")
	if err != nil || row.Status != "failed" {
		t.Fatalf("row %+v, %v", row, err)
	}
}

func TestRetryOfAMarkedExpiredReport(t *testing.T) {
	f := newFixture(t)
	id := f.seedReport(t, "collected", "complete", fixedNow.Add(time.Hour), "failed")
	f.exec(t, `UPDATE reports SET expired_at = $2 WHERE id = $1`, id, fixedNow)
	err := f.svc.RetryPDF(context.Background(), f.op, id, "en")
	if e := asError(t, err); e.Code != CodeReportExpired {
		t.Fatalf("%+v", e)
	}
}

// A sweep that commits while a replay runs never turns a deleted report into
// an expired one.
func TestReplayRacingTheSweepIsNotFound(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	f.addProfile(t, "prod", baseDocument())
	acc, err := f.svc.Request(ctx, f.op, "k1", f.selection())
	if err != nil {
		t.Fatal(err)
	}
	if err := f.svc.Delete(ctx, f.op, acc.ID); err != nil {
		t.Fatal(err)
	}
	f.svc.afterKeyRead = func(ctx context.Context) {
		err := store.Transact(ctx, f.pool, func(tx pgx.Tx) error {
			ids, err := store.SweepDeletedReports(ctx, tx, 100)
			if err == nil && !slices.Equal(ids, []uuid.UUID{acc.ID}) {
				err = fmt.Errorf("swept %v", ids)
			}
			return err
		})
		if err != nil {
			t.Error(err)
		}
	}
	_, err = f.svc.Request(ctx, f.op, "k1", f.selection())
	if e := asError(t, err); e.Code != CodeReportNotFound {
		t.Fatalf("replay: %+v", e)
	}
	if n := f.count(t, `SELECT count(*) FROM reports`); n != 0 {
		t.Fatalf("%d reports", n)
	}
}
