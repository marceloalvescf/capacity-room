//go:build postgres

package profiles

import (
	"context"
	"math"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/google/uuid"

	"github.com/marceloalvescf/capacity-room/internal/platform/store"
)

func (f *fixture) importFile(t *testing.T, content string) ([]Outcome, error) {
	t.Helper()
	entries, err := ReadImportFile(writeImport(t, content), staticPolicy())
	if err != nil {
		t.Fatal(err)
	}
	return f.svc.Import(context.Background(), entries)
}

// actorless counts the revisions of id written with no creator and the audit
// events of the profile written with no actor.
func (f *fixture) actorless(t *testing.T, id string) (revisions, events int) {
	t.Helper()
	ctx := context.Background()
	if err := f.pool.QueryRow(ctx, `SELECT count(*) FROM profile_revisions WHERE profile_id = $1 AND created_by IS NULL`, id).Scan(&revisions); err != nil {
		t.Fatal(err)
	}
	if err := f.pool.QueryRow(ctx, `SELECT count(*) FROM audit_events WHERE target_id = $1 AND actor_id IS NULL`, id).Scan(&events); err != nil {
		t.Fatal(err)
	}
	return revisions, events
}

func TestImportCreatesRevisesAndLeavesUnchanged(t *testing.T) {
	f := newFixture(t)

	got, err := f.importFile(t, importExample)
	if err != nil {
		t.Fatal(err)
	}
	if want := []Outcome{{"prod", "created"}, {"stage", "created"}}; !reflect.DeepEqual(got, want) {
		t.Fatalf("first run = %+v, want %+v", got, want)
	}
	first, err := store.GetCurrentRevision(context.Background(), f.pool, "prod")
	if err != nil {
		t.Fatal(err)
	}

	changed := strings.Replace(importExample, `display_name = "Staging"`, `display_name = "Staging 2"`, 1)
	got, err = f.importFile(t, changed)
	if err != nil {
		t.Fatal(err)
	}
	if want := []Outcome{{"prod", "unchanged"}, {"stage", "revised"}}; !reflect.DeepEqual(got, want) {
		t.Fatalf("second run = %+v, want %+v", got, want)
	}

	got, err = f.importFile(t, changed)
	if err != nil {
		t.Fatal(err)
	}
	if want := []Outcome{{"prod", "unchanged"}, {"stage", "unchanged"}}; !reflect.DeepEqual(got, want) {
		t.Fatalf("third run = %+v, want %+v", got, want)
	}

	after, err := store.GetCurrentRevision(context.Background(), f.pool, "prod")
	if err != nil || after.ID != first.ID || after.CreatedBy != nil {
		t.Fatalf("prod after = %+v, err %v; want the first revision, no creator", after, err)
	}
	if want := 1; len(f.revisions(t, "prod")) != want {
		t.Fatalf("prod revisions = %d, want %d", len(f.revisions(t, "prod")), want)
	}
	stage := f.revisions(t, "stage")
	if len(stage) != 2 || stage[1].Number != 2 {
		t.Fatalf("stage revisions = %+v, want two", stage)
	}
	if r, e := f.actorless(t, "stage"); r != 2 || e != 2 {
		t.Fatalf("stage actorless revisions = %d, events = %d, want 2 and 2", r, e)
	}
	if r, e := f.actorless(t, "prod"); r != 1 || e != 1 {
		t.Fatalf("prod actorless revisions = %d, events = %d, want 1 and 1", r, e)
	}
	events := f.audits(t, "profile.")
	var actions []string
	for _, e := range events {
		actions = append(actions, e.Action+" "+e.TargetID)
	}
	if want := []string{"profile.create prod", "profile.create stage", "profile.revise stage"}; !reflect.DeepEqual(actions, want) {
		t.Fatalf("audit = %v, want %v", actions, want)
	}
	revise := events[2]
	cur, err := store.GetProfileRevision(context.Background(), f.pool, mustUUID(t, revise.Meta["base_revision_id"]))
	if err != nil || cur.Revision != 1 {
		t.Fatalf("base revision %v: %+v, %v", revise.Meta["base_revision_id"], cur, err)
	}
}

func TestImportRecreatesADeletedProfile(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	f.create(t, "prod", baseDocument())
	if err := f.svc.Delete(ctx, f.root, "prod"); err != nil {
		t.Fatal(err)
	}

	got, err := f.importFile(t, importExample)
	if err != nil {
		t.Fatal(err)
	}
	if want := []Outcome{{"prod", "recreated"}, {"stage", "created"}}; !reflect.DeepEqual(got, want) {
		t.Fatalf("outcomes = %+v, want %+v", got, want)
	}
	cur, err := store.GetCurrentRevision(ctx, f.pool, "prod")
	if err != nil || cur.Revision != 2 || cur.CreatedBy != nil {
		t.Fatalf("prod = %+v, err %v; want revision 2 with no creator", cur, err)
	}
	var rows, anonymous int
	if err := f.pool.QueryRow(ctx, `SELECT count(*), count(*) FILTER (WHERE actor_id IS NULL) FROM audit_events WHERE action = 'profile.recreate' AND target_id = 'prod'`).Scan(&rows, &anonymous); err != nil {
		t.Fatal(err)
	}
	if rows != 1 || anonymous != 1 {
		t.Fatalf("profile.recreate rows = %d, with no actor = %d, want 1 and 1", rows, anonymous)
	}
	var creates int
	if err := f.pool.QueryRow(ctx, `SELECT count(*) FROM audit_events WHERE action = 'profile.create' AND target_id = 'prod'`).Scan(&creates); err != nil {
		t.Fatal(err)
	}
	if creates != 1 {
		t.Fatalf("profile.create rows = %d, want only the first", creates)
	}
}

func TestImportLeavesOtherProfilesAlone(t *testing.T) {
	f := newFixture(t)
	other := f.create(t, "other", baseDocument())

	got, err := f.importFile(t, importExample)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("outcomes = %+v", got)
	}
	cur, err := f.svc.Get(context.Background(), f.root, "other")
	if err != nil || cur.RevisionID != other.RevisionID || cur.Revision != 1 {
		t.Fatalf("other = %+v, err %v; want the revision it had", cur, err)
	}
	if n := len(f.audits(t, "profile.")); n != 3 {
		t.Fatalf("audit events = %d, want 3", n)
	}
}

func TestImportIsOneTransaction(t *testing.T) {
	f := newFixture(t)

	// The file is judged whole before any write.
	bad := strings.Replace(importExample, `endpoint_url = "https://metrics.test/api/v1"
cluster_name = "stage"`, `endpoint_url = "http://169.254.169.254/"
cluster_name = "stage"`, 1)
	if _, err := ReadImportFile(writeImport(t, bad), staticPolicy()); err == nil || !strings.Contains(err.Error(), "profile stage: endpoint_url invalid") {
		t.Fatalf("err = %v, want the second profile refused", err)
	}

	// A failure while writing the second entry undoes the first.
	entries, err := ReadImportFile(writeImport(t, importExample), staticPolicy())
	if err != nil {
		t.Fatal(err)
	}
	entries[1].Document.TimeoutSeconds = math.NaN()
	if _, err := f.svc.Import(context.Background(), entries); err == nil {
		t.Fatal("Import accepted a document that cannot be stored")
	}
	for _, id := range []string{"prod", "stage"} {
		if _, err := store.GetCurrentRevision(context.Background(), f.pool, id); err == nil {
			t.Errorf("profile %s has a current revision after a failed import", id)
		}
	}
	var profiles, revisions int
	if err := f.pool.QueryRow(context.Background(), `SELECT (SELECT count(*) FROM profiles), (SELECT count(*) FROM profile_revisions)`).Scan(&profiles, &revisions); err != nil {
		t.Fatal(err)
	}
	if profiles != 0 || revisions != 0 || len(f.audits(t, "profile.")) != 0 {
		t.Fatalf("after a failed import: %d profiles, %d revisions, %d events", profiles, revisions, len(f.audits(t, "profile.")))
	}
}

func mustUUID(t *testing.T, v any) uuid.UUID {
	t.Helper()
	s, ok := v.(string)
	if !ok {
		t.Fatalf("value %v is not a string", v)
	}
	id, err := uuid.Parse(s)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func TestImportConcurrentOppositeOrders(t *testing.T) {
	f := newFixture(t)
	if _, err := f.importFile(t, importExample); err != nil {
		t.Fatal(err)
	}
	forward, err := ReadImportFile(writeImport(t, importExample), staticPolicy())
	if err != nil {
		t.Fatal(err)
	}
	backward := []ImportEntry{forward[1], forward[0]}

	for round := range 10 {
		var wg sync.WaitGroup
		errs := make([]error, 2)
		outs := make([][]Outcome, 2)
		for i, entries := range [][]ImportEntry{forward, backward} {
			wg.Go(func() { outs[i], errs[i] = f.svc.Import(context.Background(), entries) })
		}
		wg.Wait()
		for i, err := range errs {
			if err != nil {
				t.Fatalf("round %d, import %d: %v", round, i, err)
			}
		}
		if want := []Outcome{{"stage", "unchanged"}, {"prod", "unchanged"}}; !reflect.DeepEqual(outs[1], want) {
			t.Fatalf("round %d: backward = %+v, want %+v", round, outs[1], want)
		}
	}
}
