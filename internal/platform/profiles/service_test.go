//go:build postgres

package profiles

import (
	"context"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"net/netip"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/marceloalvescf/capacity-room/internal/platform/auth"
	"github.com/marceloalvescf/capacity-room/internal/platform/netpolicy"
	"github.com/marceloalvescf/capacity-room/internal/platform/settings"
	"github.com/marceloalvescf/capacity-room/internal/platform/store"
	"github.com/marceloalvescf/capacity-room/internal/platform/storetest"
	"github.com/marceloalvescf/capacity-room/internal/promtest"
)

var fixedNow = time.Date(2026, 10, 5, 12, 0, 0, 0, time.UTC)

type fixture struct {
	pool    *pgxpool.Pool
	svc     *Service
	root    auth.Principal
	eve     auth.Principal
	credDir string
	caDir   string
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
	f := &fixture{pool: pool, credDir: t.TempDir(), caDir: t.TempDir()}
	f.root = newPrincipal(t, pool, "root", auth.RoleAdministrator)
	f.eve = newPrincipal(t, pool, "eve", auth.RoleReader)
	f.svc = &Service{
		Pool:     pool,
		Policy:   netpolicy.Policy{Networks: []netip.Prefix{netip.MustParsePrefix("127.0.0.1/32")}},
		Settings: settings.Settings{CredentialDir: f.credDir, CABundleDir: f.caDir},
		Now:      func() time.Time { return fixedNow },
		Resolve: func(_ context.Context, host string) ([]netip.Addr, error) {
			if host != "metrics.test" {
				t.Errorf("resolver asked for %q", host)
				return nil, errors.New("unexpected host")
			}
			return []netip.Addr{netip.MustParseAddr("127.0.0.1")}, nil
		},
	}
	return f
}

func baseDocument() Document {
	d := Defaults()
	d.DisplayName = "Production"
	d.Active = true
	d.EndpointURL = "https://metrics.test/api/v1"
	d.ClusterName = "prod"
	d.AllowedNamespaces = []string{"payments"}
	return d
}

func (f *fixture) create(t *testing.T, id string, d Document) Current {
	t.Helper()
	c, err := f.svc.Create(context.Background(), f.root, id, d)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func fields(t *testing.T, kv ...any) map[string]json.RawMessage {
	t.Helper()
	m := map[string]json.RawMessage{}
	for i := 0; i < len(kv); i += 2 {
		raw, err := json.Marshal(kv[i+1])
		if err != nil {
			t.Fatal(err)
		}
		m[kv[i].(string)] = raw
	}
	return m
}

func (f *fixture) revise(t *testing.T, id string, base uuid.UUID, kv ...any) (Current, error) {
	t.Helper()
	return f.svc.Revise(context.Background(), f.root, id, Change{BaseRevisionID: base, Fields: fields(t, kv...)})
}

type revisionSummary struct {
	Number      int
	Fingerprint string
}

func (f *fixture) revisions(t *testing.T, id string) []revisionSummary {
	t.Helper()
	rows, err := f.pool.Query(context.Background(), `SELECT revision, fingerprint FROM profile_revisions WHERE profile_id = $1 ORDER BY revision`, id)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out []revisionSummary
	for rows.Next() {
		var r revisionSummary
		if err := rows.Scan(&r.Number, &r.Fingerprint); err != nil {
			t.Fatal(err)
		}
		out = append(out, r)
	}
	return out
}

type auditRow struct {
	Action, Outcome, TargetID string
	Meta                      map[string]any
}

func (f *fixture) audits(t *testing.T, prefix string) []auditRow {
	t.Helper()
	events, err := store.ListAuditEvents(context.Background(), f.pool)
	if err != nil {
		t.Fatal(err)
	}
	var out []auditRow
	for _, e := range events {
		if !strings.HasPrefix(e.Action, prefix) {
			continue
		}
		r := auditRow{Action: e.Action, Outcome: e.Outcome}
		if e.TargetID != nil {
			r.TargetID = *e.TargetID
		}
		if err := json.Unmarshal(e.Metadata, &r.Meta); err != nil {
			t.Fatal(err)
		}
		out = append(out, r)
	}
	return out
}

func problemsOf(t *testing.T, err error) []Problem {
	t.Helper()
	var de *DocumentError
	if !errors.As(err, &de) {
		t.Fatalf("err = %v, want a *DocumentError", err)
	}
	return de.Problems
}

func policyCode(t *testing.T, err error) netpolicy.Code {
	t.Helper()
	var pe *netpolicy.Error
	if !errors.As(err, &pe) {
		t.Fatalf("err = %v, want a *netpolicy.Error", err)
	}
	return pe.Code
}

func TestCreateStoresCanonicalAndAudits(t *testing.T) {
	f := newFixture(t)
	d := baseDocument()
	d.EndpointURL = "HTTPS://Metrics.Test./api/v1/"
	c := f.create(t, "prod", d)

	if c.ID != "prod" || c.Revision != 1 || c.Document.EndpointURL != "https://metrics.test/api/v1" || !c.CreatedAt.Equal(fixedNow) {
		t.Fatalf("got %+v", c)
	}
	row, err := store.GetCurrentRevision(context.Background(), f.pool, "prod")
	if err != nil {
		t.Fatal(err)
	}
	var stored Document
	if err := json.Unmarshal(row.Document, &stored); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(stored, c.Document) || row.ID != c.RevisionID || row.DisplayName != "Production" || !row.Active ||
		row.CreatedBy == nil || *row.CreatedBy != f.root.ID {
		t.Fatalf("row %+v, document %+v", row, stored)
	}
	got := f.audits(t, "profile.")
	if len(got) != 1 || got[0].Action != "profile.create" || got[0].TargetID != "prod" || got[0].Outcome != "succeeded" {
		t.Fatalf("audit %+v", got)
	}
	wantFields := []any{
		"active", "allowed_namespaces", "ca_bundle_file", "cluster_label", "cluster_name", "credential_file",
		"default_period", "default_report_profile", "default_timezone", "display_name",
		"endpoint_url", "excluded_containers", "filter_mode", "node_autoscaling", "nodepool_label",
		"permitted_periods", "permitted_report_profiles", "permitted_timezones",
		"thresholds", "timeout_seconds", "tls_verify",
	}
	if !reflect.DeepEqual(got[0].Meta["fields"], wantFields) || got[0].Meta["revision"] != float64(1) || got[0].Meta["revision_id"] != c.RevisionID.String() {
		t.Fatalf("metadata %+v", got[0].Meta)
	}
}

func TestCreateRefusesNonAdministrator(t *testing.T) {
	f := newFixture(t)
	_, err := f.svc.Create(context.Background(), f.eve, "prod", baseDocument())
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("err = %v", err)
	}
	assertNothingWritten(t, f, "prod")
}

func assertNothingWritten(t *testing.T, f *fixture, id string) {
	t.Helper()
	if n := len(f.revisions(t, id)); n != 0 {
		t.Fatalf("%d revisions written", n)
	}
	var profiles int
	if err := f.pool.QueryRow(context.Background(), `SELECT count(*) FROM profiles`).Scan(&profiles); err != nil || profiles != 0 {
		t.Fatalf("%d profiles, %v", profiles, err)
	}
	if got := f.audits(t, "profile."); len(got) != 0 {
		t.Fatalf("audit %+v", got)
	}
}

func TestCreateRefusesDocument(t *testing.T) {
	f := newFixture(t)
	d := baseDocument()
	d.ClusterName = ""
	_, err := f.svc.Create(context.Background(), f.root, "prod", d)
	if got := problemsOf(t, err); !reflect.DeepEqual(got, []Problem{{Field: "cluster_name", Word: "empty"}}) {
		t.Fatalf("problems %+v", got)
	}
	assertNothingWritten(t, f, "prod")

	_, err = f.svc.Create(context.Background(), f.root, "Bad Id", baseDocument())
	if got := problemsOf(t, err); !reflect.DeepEqual(got, []Problem{{Field: "id", Word: "invalid"}}) {
		t.Fatalf("problems %+v", got)
	}
}

func TestCreateRefusesPolicy(t *testing.T) {
	f := newFixture(t)
	d := baseDocument()
	d.EndpointURL = "http://169.254.169.254/"
	_, err := f.svc.Create(context.Background(), f.root, "prod", d)
	if code := policyCode(t, err); code != netpolicy.CodeAddressRejected {
		t.Fatalf("code %s", code)
	}
	assertNothingWritten(t, f, "prod")
}

func TestCreateRefusesTakenID(t *testing.T) {
	f := newFixture(t)
	f.create(t, "prod", baseDocument())
	_, err := f.svc.Create(context.Background(), f.root, "prod", baseDocument())
	if !errors.Is(err, ErrIDTaken) {
		t.Fatalf("err = %v", err)
	}
	if n := len(f.revisions(t, "prod")); n != 1 {
		t.Fatalf("%d revisions", n)
	}
}

func TestCreateAfterDeleteContinuesTheCount(t *testing.T) {
	f := newFixture(t)
	f.create(t, "prod", baseDocument())
	if err := f.svc.Delete(context.Background(), f.root, "prod"); err != nil {
		t.Fatal(err)
	}
	c := f.create(t, "prod", baseDocument())
	if c.Revision != 2 {
		t.Fatalf("revision %d", c.Revision)
	}
	if n := len(f.revisions(t, "prod")); n != 2 {
		t.Fatalf("%d revisions", n)
	}
	// The return of a deleted profile is audited as the import audits it.
	if got := f.audits(t, ActionProfileCreate); len(got) != 1 {
		t.Fatalf("%d profile.create rows, want only the first: %+v", len(got), got)
	}
	got := f.audits(t, ActionProfileRecreate)
	if len(got) != 1 || got[0].TargetID != "prod" || got[0].Meta["revision_id"] != c.RevisionID.String() ||
		got[0].Meta["revision"] != float64(2) {
		t.Fatalf("profile.recreate rows %+v", got)
	}
}

func TestReviseAppendsAndKeepsHistory(t *testing.T) {
	f := newFixture(t)
	first := f.create(t, "prod", baseDocument())
	second, err := f.revise(t, "prod", first.RevisionID, "display_name", "Production EU")
	if err != nil {
		t.Fatal(err)
	}
	if second.Revision != 2 || second.Document.DisplayName != "Production EU" || second.RevisionID == first.RevisionID {
		t.Fatalf("got %+v", second)
	}
	old, err := store.GetProfileRevision(context.Background(), f.pool, first.RevisionID)
	if err != nil || old.Revision != 1 || old.DisplayName != "Production" {
		t.Fatalf("old %+v, %v", old, err)
	}
	got := f.audits(t, "profile.revise")
	if len(got) != 1 || got[0].TargetID != "prod" ||
		!reflect.DeepEqual(got[0].Meta["fields"], []any{"display_name"}) ||
		got[0].Meta["revision_id"] != second.RevisionID.String() ||
		got[0].Meta["base_revision_id"] != first.RevisionID.String() ||
		got[0].Meta["revision"] != float64(2) {
		t.Fatalf("audit %+v", got)
	}
}

func TestReviseStale(t *testing.T) {
	f := newFixture(t)
	first := f.create(t, "prod", baseDocument())
	if _, err := f.revise(t, "prod", first.RevisionID, "display_name", "Two"); err != nil {
		t.Fatal(err)
	}
	_, err := f.revise(t, "prod", first.RevisionID, "display_name", "Three")
	if !errors.Is(err, ErrRevisionStale) {
		t.Fatalf("err = %v", err)
	}
	if n := len(f.revisions(t, "prod")); n != 2 {
		t.Fatalf("%d revisions", n)
	}
}

func TestReviseUnknownProfile(t *testing.T) {
	f := newFixture(t)
	_, err := f.revise(t, "ghost", uuid.New(), "display_name", "x")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v", err)
	}
}

func TestReviseNullClearsADefault(t *testing.T) {
	f := newFixture(t)
	d := baseDocument()
	period := "7d"
	d.DefaultPeriod = &period
	first := f.create(t, "prod", d)
	got, err := f.svc.Revise(context.Background(), f.root, "prod", Change{
		BaseRevisionID: first.RevisionID,
		Fields:         map[string]json.RawMessage{"default_period": json.RawMessage("null")},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.Document.DefaultPeriod != nil || got.Revision != 2 {
		t.Fatalf("got %+v", got)
	}
	audits := f.audits(t, "profile.revise")
	if len(audits) != 1 || !reflect.DeepEqual(audits[0].Meta["fields"], []any{"default_period"}) {
		t.Fatalf("audit %+v", audits)
	}

	_, err = f.svc.Revise(context.Background(), f.root, "prod", Change{
		BaseRevisionID: got.RevisionID,
		Fields:         map[string]json.RawMessage{"display_name": json.RawMessage("null"), "bogus": json.RawMessage("1")},
	})
	want := []Problem{{Field: "bogus", Word: "invalid"}, {Field: "display_name", Word: "invalid"}}
	if p := problemsOf(t, err); !reflect.DeepEqual(p, want) {
		t.Fatalf("problems %+v", p)
	}
}

func TestReviseAbsentKeepsValue(t *testing.T) {
	f := newFixture(t)
	d := baseDocument()
	period := "7d"
	d.DefaultPeriod = &period
	d.ExcludedContainers = []string{"istio-proxy"}
	d.Thresholds.WarningPercent = 55
	first := f.create(t, "prod", d)
	got, err := f.revise(t, "prod", first.RevisionID, "display_name", "Other")
	if err != nil {
		t.Fatal(err)
	}
	want := first.Document
	want.DisplayName = "Other"
	if !reflect.DeepEqual(got.Document, want) {
		t.Fatalf("got %+v, want %+v", got.Document, want)
	}
}

func TestReviseThresholdsOverlay(t *testing.T) {
	f := newFixture(t)
	first := f.create(t, "prod", baseDocument())
	got, err := f.revise(t, "prod", first.RevisionID, "thresholds", map[string]any{"warning_percent": 60})
	if err != nil {
		t.Fatal(err)
	}
	want := first.Document.Thresholds
	want.WarningPercent = 60
	if !reflect.DeepEqual(got.Document.Thresholds, want) {
		t.Fatalf("got %+v, want %+v", got.Document.Thresholds, want)
	}
}

func TestReviseRefusesANullThreshold(t *testing.T) {
	f := newFixture(t)
	first := f.create(t, "prod", baseDocument())
	_, err := f.revise(t, "prod", first.RevisionID, "thresholds", map[string]any{"warning_percent": nil})
	if got := problemsOf(t, err); !reflect.DeepEqual(got, []Problem{{Field: "thresholds", Word: "invalid"}}) {
		t.Fatalf("problems %+v", got)
	}
	if n := len(f.revisions(t, "prod")); n != 1 {
		t.Fatalf("%d revisions", n)
	}
	if a := f.audits(t, "profile.revise"); len(a) != 0 {
		t.Fatalf("audit %+v", a)
	}
}

func TestReviseNoOpWritesNothing(t *testing.T) {
	f := newFixture(t)
	first := f.create(t, "prod", baseDocument())
	got, err := f.revise(t, "prod", first.RevisionID, "display_name", "Production", "endpoint_url", "HTTPS://metrics.test/api/v1/")
	if err != nil {
		t.Fatal(err)
	}
	if got.RevisionID != first.RevisionID || got.Revision != 1 {
		t.Fatalf("got %+v", got)
	}
	if n := len(f.revisions(t, "prod")); n != 1 {
		t.Fatalf("%d revisions", n)
	}
	if a := f.audits(t, "profile.revise"); len(a) != 0 {
		t.Fatalf("audit %+v", a)
	}
}

func TestReviseRefusesMergedDocument(t *testing.T) {
	f := newFixture(t)
	d := baseDocument()
	d.CABundleFile = "ca.pem"
	first := f.create(t, "prod", d)
	_, err := f.revise(t, "prod", first.RevisionID, "tls_verify", false)
	if got := problemsOf(t, err); !reflect.DeepEqual(got, []Problem{{Field: "ca_bundle_file", Word: "incoherent"}}) {
		t.Fatalf("problems %+v", got)
	}
	if n := len(f.revisions(t, "prod")); n != 1 {
		t.Fatalf("%d revisions", n)
	}
}

func TestReviseEndpointRevalidated(t *testing.T) {
	f := newFixture(t)
	first := f.create(t, "prod", baseDocument())
	_, err := f.revise(t, "prod", first.RevisionID, "endpoint_url", "http://169.254.169.254/")
	if code := policyCode(t, err); code != netpolicy.CodeAddressRejected {
		t.Fatalf("code %s", code)
	}
	if n := len(f.revisions(t, "prod")); n != 1 {
		t.Fatalf("%d revisions", n)
	}
}

func TestReviseABAProducesThreeRevisions(t *testing.T) {
	f := newFixture(t)
	a := f.create(t, "prod", baseDocument())
	b, err := f.revise(t, "prod", a.RevisionID, "display_name", "B")
	if err != nil {
		t.Fatal(err)
	}
	c, err := f.revise(t, "prod", b.RevisionID, "display_name", "Production")
	if err != nil {
		t.Fatal(err)
	}
	if c.Revision != 3 || !reflect.DeepEqual(c.Document, a.Document) {
		t.Fatalf("got %+v", c)
	}
	rows := f.revisions(t, "prod")
	if len(rows) != 3 || rows[0].Fingerprint == rows[2].Fingerprint || rows[0].Fingerprint == rows[1].Fingerprint || rows[1].Fingerprint == rows[2].Fingerprint {
		t.Fatalf("rows %+v", rows)
	}
}

func TestReviseActiveWritesActivateOrDeactivate(t *testing.T) {
	f := newFixture(t)
	d := baseDocument()
	d.Active = false
	first := f.create(t, "prod", d)
	second, err := f.revise(t, "prod", first.RevisionID, "active", true)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.revise(t, "prod", second.RevisionID, "active", false); err != nil {
		t.Fatal(err)
	}
	got := f.audits(t, "profile.")
	var actions []string
	for _, a := range got {
		actions = append(actions, a.Action)
	}
	if want := []string{"profile.create", "profile.activate", "profile.deactivate"}; !reflect.DeepEqual(actions, want) {
		t.Fatalf("actions %v", actions)
	}
	if !reflect.DeepEqual(got[1].Meta["fields"], []any{"active"}) {
		t.Fatalf("metadata %+v", got[1].Meta)
	}
}

func TestReviseConcurrentlyOneWins(t *testing.T) {
	f := newFixture(t)
	first := f.create(t, "prod", baseDocument())
	var wg sync.WaitGroup
	start := make(chan struct{})
	type outcome struct {
		cur Current
		err error
	}
	results := make([]outcome, 2)
	for i, name := range []string{"Left", "Right"} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			results[i].cur, results[i].err = f.revise(t, "prod", first.RevisionID, "display_name", name)
		}()
	}
	close(start)
	wg.Wait()

	wins, stale := 0, 0
	for _, r := range results {
		switch {
		case r.err == nil && r.cur.Revision == 2:
			wins++
		case errors.Is(r.err, ErrRevisionStale):
			stale++
		default:
			t.Fatalf("outcome %+v", r)
		}
	}
	if wins != 1 || stale != 1 {
		t.Fatalf("wins %d, stale %d", wins, stale)
	}
	rows := f.revisions(t, "prod")
	if len(rows) != 2 || rows[0].Number != 1 || rows[1].Number != 2 {
		t.Fatalf("rows %+v", rows)
	}
}

func TestDeleteClearsAndRetains(t *testing.T) {
	f := newFixture(t)
	first := f.create(t, "prod", baseDocument())
	if err := f.svc.Delete(context.Background(), f.root, "prod"); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.Get(context.Background(), f.root, "prod"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v", err)
	}
	if err := f.svc.Delete(context.Background(), f.root, "prod"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("second delete: %v", err)
	}
	if n := len(f.revisions(t, "prod")); n != 1 {
		t.Fatalf("%d revisions", n)
	}
	got := f.audits(t, "profile.delete")
	if len(got) != 1 || got[0].TargetID != "prod" || got[0].Meta["revision_id"] != first.RevisionID.String() {
		t.Fatalf("audit %+v", got)
	}
	if _, err := f.revise(t, "prod", first.RevisionID, "display_name", "x"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("revise after delete: %v", err)
	}
}

func TestDeleteUnknown(t *testing.T) {
	f := newFixture(t)
	if err := f.svc.Delete(context.Background(), f.root, "ghost"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v", err)
	}
}

func TestReadsRefuseNonAdministrator(t *testing.T) {
	f := newFixture(t)
	f.create(t, "prod", baseDocument())
	ctx := context.Background()
	if _, err := f.svc.Get(ctx, f.eve, "prod"); !errors.Is(err, ErrForbidden) {
		t.Fatalf("get: %v", err)
	}
	if _, err := f.svc.List(ctx, f.eve); !errors.Is(err, ErrForbidden) {
		t.Fatalf("list: %v", err)
	}
	if err := f.svc.Delete(ctx, f.eve, "prod"); !errors.Is(err, ErrForbidden) {
		t.Fatalf("delete: %v", err)
	}
	if _, err := f.svc.Revise(ctx, f.eve, "prod", Change{}); !errors.Is(err, ErrForbidden) {
		t.Fatalf("revise: %v", err)
	}
	if _, err := f.svc.Test(ctx, f.eve, "prod"); !errors.Is(err, ErrForbidden) {
		t.Fatalf("test: %v", err)
	}
}

func TestListActiveSummaries(t *testing.T) {
	f := newFixture(t)
	d := baseDocument()
	d.PermittedPeriods = []string{"24h", "7d"}
	period := "7d"
	d.DefaultPeriod = &period
	f.create(t, "prod", d)
	inactive := baseDocument()
	inactive.Active = false
	inactive.DisplayName = "Development"
	f.create(t, "dev", inactive)

	got, err := f.svc.ListActive(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	want := []Summary{{
		ID: "prod", DisplayName: "Production", AllowedNamespaces: []string{"payments"},
		PermittedPeriods: []string{"24h", "7d"}, DefaultPeriod: "7d",
		PermittedTimezones: []string{}, DefaultTimezone: "America/Sao_Paulo",
		PermittedReportProfiles: []string{}, DefaultReportProfile: "detailed",
	}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v", got)
	}
	if n := reflect.TypeOf(Summary{}).NumField(); n != 9 {
		t.Fatalf("%d fields", n)
	}
}

func TestListIncludesInactiveForAdministrators(t *testing.T) {
	f := newFixture(t)
	f.create(t, "prod", baseDocument())
	inactive := baseDocument()
	inactive.Active = false
	f.create(t, "dev", inactive)
	got, err := f.svc.List(context.Background(), f.root)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].ID != "dev" || got[1].ID != "prod" {
		t.Fatalf("got %+v", got)
	}
}

// fakePrometheus serves the direct-complete scenario once and returns the
// profile endpoint that reaches it through the stubbed name.
func fakePrometheus(t *testing.T) (*promtest.Server, string, promtest.Fixture) {
	t.Helper()
	fixture, err := promtest.LoadScenario("direct-complete")
	if err != nil {
		t.Fatal(err)
	}
	server := promtest.NewServer(fixture)
	ts := httptest.NewServer(server)
	t.Cleanup(ts.Close)
	return server, endpointOf(t, ts), fixture
}

func endpointOf(t *testing.T, ts *httptest.Server) string {
	t.Helper()
	addr := ts.Listener.Addr().String()
	port := addr[strings.LastIndex(addr, ":")+1:]
	return "http://metrics.test:" + port + "/api/v1"
}

func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}
