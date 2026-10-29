//go:build postgres

package profiles

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"reflect"
	"slices"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/marceloalvescf/capacity-room/internal/platform/auth"
	"github.com/marceloalvescf/capacity-room/internal/platform/netpolicy"
	"github.com/marceloalvescf/capacity-room/internal/promtest"
)

const secretToken = "s3cr3t-token-value"

func (f *fixture) runTest(t *testing.T, id string) TestResult {
	t.Helper()
	got, err := f.svc.Test(context.Background(), f.root, id)
	if err != nil {
		t.Fatal(err)
	}
	return got
}

func statusServer(t *testing.T, status int) (*httptest.Server, *int) {
	t.Helper()
	count := new(int)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		*count++
		w.WriteHeader(status)
	}))
	t.Cleanup(ts.Close)
	return ts, count
}

func TestTestOK(t *testing.T) {
	f := newFixture(t)
	server, endpoint, fixture := fakePrometheus(t)
	d := baseDocument()
	d.EndpointURL = endpoint
	for _, r := range fixture.Requests {
		if token, ok := strings.CutPrefix(r.Authorization, "Bearer "); ok {
			writeFile(t, f.credDir, "token", token+"\n")
			d.CredentialFile = "token"
			break
		}
	}
	f.create(t, "prod", d)
	got := f.runTest(t, "prod")
	want := TestResult{OK: true, Code: "PROFILE_OK", Namespaces: []string{"checkout", "payments", "zeta"}, Capabilities: []string{
		"namespace_usage", "workload_usage", "requests_limits", "hpa_history", "workloads_without_hpa",
		"ip_node_mapping", "nodepool_discovery", "node_utilization", "node_autoscaling",
	}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v", got)
	}
	if len(server.Seen()) == 0 {
		t.Fatal("the server saw nothing")
	}
	audits := f.audits(t, "profile.test")
	if len(audits) != 1 || audits[0].Outcome != "succeeded" || audits[0].Meta["code"] != "PROFILE_OK" || audits[0].TargetID != "prod" {
		t.Fatalf("audit %+v", audits)
	}
}

func (f *fixture) assertFailed(t *testing.T, got TestResult, code string) {
	t.Helper()
	if !reflect.DeepEqual(got, TestResult{Code: code}) {
		t.Fatalf("got %+v, want code %s", got, code)
	}
	audits := f.audits(t, "profile.test")
	if len(audits) == 0 {
		t.Fatal("no audit row")
	}
	last := audits[len(audits)-1]
	if last.Outcome != "failed" || last.Meta["code"] != code {
		t.Fatalf("audit %+v", last)
	}
}

func caseRefusedByPolicy(t *testing.T) (*fixture, TestResult, int) {
	f := newFixture(t)
	server, endpoint, _ := fakePrometheus(t)
	d := baseDocument()
	d.EndpointURL = endpoint
	f.create(t, "prod", d)
	f.svc.Policy.Networks = []netip.Prefix{netip.MustParsePrefix("10.0.0.0/8")}
	got := f.runTest(t, "prod")
	return f, got, len(server.Seen())
}

func TestTestRefusedByPolicy(t *testing.T) {
	f, got, seen := caseRefusedByPolicy(t)
	f.assertFailed(t, got, "ENDPOINT_ADDRESS_REJECTED")
	if seen != 0 {
		t.Fatalf("the server saw %d requests", seen)
	}
}

func caseCredentialUnavailable(t *testing.T) (*fixture, TestResult, int) {
	f := newFixture(t)
	server, endpoint, _ := fakePrometheus(t)
	d := baseDocument()
	d.EndpointURL = endpoint
	d.CredentialFile = "missing-token"
	f.create(t, "prod", d)
	return f, f.runTest(t, "prod"), len(server.Seen())
}

func TestTestCredentialUnavailable(t *testing.T) {
	f, got, seen := caseCredentialUnavailable(t)
	f.assertFailed(t, got, "PROFILE_CREDENTIAL_UNAVAILABLE")
	if seen != 0 {
		t.Fatalf("the server saw %d requests", seen)
	}
}

func caseCABundleUnavailable(t *testing.T) (*fixture, TestResult) {
	f := newFixture(t)
	_, endpoint, _ := fakePrometheus(t)
	writeFile(t, f.caDir, "bad-ca.pem", "not a certificate")
	d := baseDocument()
	d.EndpointURL = endpoint
	d.CABundleFile = "bad-ca.pem"
	f.create(t, "prod", d)
	return f, f.runTest(t, "prod")
}

func TestTestCABundleUnavailable(t *testing.T) {
	f, got := caseCABundleUnavailable(t)
	f.assertFailed(t, got, "PROFILE_CA_BUNDLE_UNAVAILABLE")
}

func caseStatus(t *testing.T, status int) (*fixture, TestResult) {
	f := newFixture(t)
	ts, _ := statusServer(t, status)
	d := baseDocument()
	d.EndpointURL = endpointOf(t, ts)
	writeFile(t, f.credDir, "token", secretToken)
	d.CredentialFile = "token"
	f.create(t, "prod", d)
	return f, f.runTest(t, "prod")
}

func TestTestAuthRejected(t *testing.T) {
	f, got := caseStatus(t, http.StatusUnauthorized)
	f.assertFailed(t, got, "ENDPOINT_AUTH_REJECTED")
}

func TestTestAPINotFound(t *testing.T) {
	f, got := caseStatus(t, http.StatusNotFound)
	f.assertFailed(t, got, "ENDPOINT_API_NOT_FOUND")
}

func caseUnreachable(t *testing.T) (*fixture, TestResult, time.Duration) {
	f := newFixture(t)
	d := baseDocument()
	d.EndpointURL = "http://metrics.test:1/api/v1"
	d.TimeoutSeconds = 1
	f.create(t, "prod", d)
	start := time.Now()
	got := f.runTest(t, "prod")
	return f, got, time.Since(start)
}

func TestTestUnreachable(t *testing.T) {
	f, got, took := caseUnreachable(t)
	f.assertFailed(t, got, "ENDPOINT_UNREACHABLE")
	if took >= 6*time.Second {
		t.Fatalf("took %s", took)
	}
}

func caseScopeNotFound(t *testing.T) (*fixture, TestResult) {
	f := newFixture(t)
	_, endpoint, _ := fakePrometheus(t)
	d := baseDocument()
	d.EndpointURL = endpoint
	d.ClusterName = "elsewhere"
	d.FilterMode = "required"
	f.create(t, "prod", d)
	return f, f.runTest(t, "prod")
}

func TestTestScopeNotFound(t *testing.T) {
	f, got := caseScopeNotFound(t)
	f.assertFailed(t, got, "CLUSTER_SCOPE_NOT_FOUND")
}

func TestTestRunsForInactive(t *testing.T) {
	f := newFixture(t)
	_, endpoint, _ := fakePrometheus(t)
	d := baseDocument()
	d.EndpointURL = endpoint
	d.Active = false
	f.create(t, "prod", d)
	if got := f.runTest(t, "prod"); !got.OK || got.Code != "PROFILE_OK" {
		t.Fatalf("got %+v", got)
	}
}

func TestTestUnknownProfile(t *testing.T) {
	f := newFixture(t)
	if _, err := f.svc.Test(context.Background(), f.root, "ghost"); err == nil || err.Error() != ErrNotFound.Error() {
		t.Fatalf("err = %v", err)
	}
}

func TestTestResultCarriesNoSecret(t *testing.T) {
	var results []TestResult
	_, r, _ := caseRefusedByPolicy(t)
	results = append(results, r)
	_, r, _ = caseCredentialUnavailable(t)
	results = append(results, r)
	_, r = caseCABundleUnavailable(t)
	results = append(results, r)
	_, r = caseStatus(t, http.StatusUnauthorized)
	results = append(results, r)
	_, r = caseStatus(t, http.StatusNotFound)
	results = append(results, r)
	_, r, _ = caseUnreachable(t)
	results = append(results, r)
	_, r = caseScopeNotFound(t)
	results = append(results, r)

	for _, r := range results {
		if r.OK || r.Code == "" {
			t.Fatalf("not a failure: %+v", r)
		}
		raw, err := json.Marshal(r)
		if err != nil {
			t.Fatal(err)
		}
		for _, banned := range []string{"http", "127.0.0.1", "metrics.test", "missing-token", "bad-ca", "token", secretToken} {
			if strings.Contains(strings.ToLower(string(raw)), strings.ToLower(banned)) {
				t.Errorf("%s holds %q", raw, banned)
			}
		}
	}
}

// stalledProfile creates a profile whose endpoint answers nothing until the
// request ends, and calls hold on the first request it sees. The body is
// read first: the server notices a closed connection only after that.
func stalledProfile(t *testing.T, f *fixture, hold func()) {
	t.Helper()
	var once sync.Once
	ts := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		once.Do(hold)
		<-r.Context().Done()
	}))
	t.Cleanup(ts.Close)
	d := baseDocument()
	d.EndpointURL = endpointOf(t, ts)
	d.TimeoutSeconds = 30
	f.create(t, "prod", d)
}

func TestTestCancelledIsAudited(t *testing.T) {
	f := newFixture(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	stalledProfile(t, f, cancel)
	got, err := f.svc.Test(ctx, f.root, "prod")
	if err != nil {
		t.Fatal(err)
	}
	f.assertFailed(t, got, "ENDPOINT_UNREACHABLE")
}

func TestTestDeadlineIsAudited(t *testing.T) {
	f := newFixture(t)
	stalledProfile(t, f, func() {})
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	got, err := f.svc.Test(ctx, f.root, "prod")
	if err != nil {
		t.Fatal(err)
	}
	f.assertFailed(t, got, "ENDPOINT_UNREACHABLE")
}

func (f *fixture) draftEvents(t *testing.T) (total, untargeted int) {
	t.Helper()
	err := f.pool.QueryRow(context.Background(),
		`SELECT count(*), count(*) FILTER (WHERE target_type IS NULL AND target_id IS NULL) FROM audit_events WHERE action = 'profile.test'`,
	).Scan(&total, &untargeted)
	if err != nil {
		t.Fatal(err)
	}
	return total, untargeted
}

func (f *fixture) profileCount(t *testing.T) int {
	t.Helper()
	var n int
	if err := f.pool.QueryRow(context.Background(), `SELECT count(*) FROM profiles`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestTestDraftOK(t *testing.T) {
	f := newFixture(t)
	_, endpoint, _ := fakePrometheus(t)
	d := baseDocument()
	d.EndpointURL = endpoint
	got, err := f.svc.TestDraft(context.Background(), f.root, "", d)
	if err != nil {
		t.Fatal(err)
	}
	want := TestResult{OK: true, Code: "PROFILE_OK", Namespaces: []string{"checkout", "payments", "zeta"}, Capabilities: []string{
		"namespace_usage", "workload_usage", "requests_limits", "hpa_history", "workloads_without_hpa",
		"ip_node_mapping", "nodepool_discovery", "node_utilization", "node_autoscaling",
	}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v", got)
	}
	if n := f.profileCount(t); n != 0 {
		t.Fatalf("%d profiles", n)
	}
	audits := f.audits(t, "profile.test")
	if len(audits) != 1 || audits[0].Outcome != "succeeded" || audits[0].TargetID != "" ||
		!reflect.DeepEqual(audits[0].Meta, map[string]any{"code": "PROFILE_OK", "draft": true}) {
		t.Fatalf("audit %+v", audits)
	}
	if total, untargeted := f.draftEvents(t); total != 1 || untargeted != 1 {
		t.Fatalf("events %d, untargeted %d", total, untargeted)
	}
}

func TestTestDraftNamesItsTarget(t *testing.T) {
	f := newFixture(t)
	_, endpoint, _ := fakePrometheus(t)
	d := baseDocument()
	d.EndpointURL = endpoint
	if _, err := f.svc.TestDraft(context.Background(), f.root, "prod", d); err != nil {
		t.Fatal(err)
	}
	var targetType, targetID string
	err := f.pool.QueryRow(context.Background(), `SELECT target_type, target_id FROM audit_events WHERE action = 'profile.test'`).Scan(&targetType, &targetID)
	if err != nil {
		t.Fatal(err)
	}
	if targetType != "profile" || targetID != "prod" {
		t.Fatalf("target %q/%q", targetType, targetID)
	}
	if n := f.profileCount(t); n != 0 {
		t.Fatalf("%d profiles", n)
	}
}

func TestTestDraftRefusesTheDocument(t *testing.T) {
	f := newFixture(t)
	d := baseDocument()
	d.ClusterName = ""
	d.TimeoutSeconds = 0
	_, err := f.svc.TestDraft(context.Background(), f.root, "", d)
	got := problemsOf(t, err)
	sort.Slice(got, func(i, j int) bool { return got[i].Field < got[j].Field })
	want := []Problem{{Field: "cluster_name", Word: "empty"}, {Field: "timeout_seconds", Word: "invalid"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("problems %+v", got)
	}
	if total, _ := f.draftEvents(t); total != 0 {
		t.Fatalf("%d audit rows", total)
	}
}

func TestTestDraftRefusesTheID(t *testing.T) {
	f := newFixture(t)
	server, endpoint, _ := fakePrometheus(t)
	d := baseDocument()
	d.EndpointURL = endpoint
	_, err := f.svc.TestDraft(context.Background(), f.root, "Bad..Name", d)
	if got := problemsOf(t, err); !reflect.DeepEqual(got, []Problem{{Field: "id", Word: "invalid"}}) {
		t.Fatalf("problems %+v", got)
	}
	if n := len(server.Seen()); n != 0 {
		t.Fatalf("the server saw %d requests", n)
	}
}

func TestTestDraftRefusedByPolicy(t *testing.T) {
	f := newFixture(t)
	server, endpoint, _ := fakePrometheus(t)
	d := baseDocument()
	d.EndpointURL = endpoint
	f.svc.Policy.Networks = []netip.Prefix{netip.MustParsePrefix("10.0.0.0/8")}
	got, err := f.svc.TestDraft(context.Background(), f.root, "", d)
	if err != nil {
		t.Fatal(err)
	}
	f.assertFailed(t, got, "ENDPOINT_ADDRESS_REJECTED")
	if n := len(server.Seen()); n != 0 {
		t.Fatalf("the server saw %d requests", n)
	}
}

func TestTestDraftRefusesTheEndpoint(t *testing.T) {
	f := newFixture(t)
	d := baseDocument()
	d.EndpointURL = "http://169.254.169.254/"
	_, err := f.svc.TestDraft(context.Background(), f.root, "", d)
	if code := policyCode(t, err); code != netpolicy.CodeAddressRejected {
		t.Fatalf("code %s", code)
	}
	if total, _ := f.draftEvents(t); total != 0 {
		t.Fatalf("%d audit rows", total)
	}
}

func TestTestDraftUnreachable(t *testing.T) {
	f := newFixture(t)
	d := baseDocument()
	d.EndpointURL = "http://metrics.test:1/api/v1"
	d.TimeoutSeconds = 1
	start := time.Now()
	got, err := f.svc.TestDraft(context.Background(), f.root, "", d)
	if err != nil {
		t.Fatal(err)
	}
	f.assertFailed(t, got, "ENDPOINT_UNREACHABLE")
	if took := time.Since(start); took >= 6*time.Second {
		t.Fatalf("took %s", took)
	}
}

func TestTestDraftAsOperator(t *testing.T) {
	f := newFixture(t)
	op := newPrincipal(t, f.pool, "oscar", auth.RoleOperator)
	_, err := f.svc.TestDraft(context.Background(), op, "", baseDocument())
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("err = %v", err)
	}
	if total, _ := f.draftEvents(t); total != 0 {
		t.Fatalf("%d audit rows", total)
	}
}

// garbageEndpoint listens on 127.0.0.1, answers every connection with text
// that is not HTTP, and closes it.
func garbageEndpoint(t *testing.T) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = l.Close() })
	go func() {
		for {
			c, err := l.Accept()
			if err != nil {
				return
			}
			go func() {
				_, _ = c.Read(make([]byte, 4096))
				_, _ = c.Write([]byte("garbage\r\n\r\n"))
				_ = c.Close()
			}()
		}
	}()
	addr := l.Addr().String()
	return "http://metrics.test:" + addr[strings.LastIndex(addr, ":")+1:] + "/api/v1"
}

func TestTestNonHTTPAnswerIsUnreachable(t *testing.T) {
	f := newFixture(t)
	d := baseDocument()
	d.EndpointURL = garbageEndpoint(t)
	d.TimeoutSeconds = 5
	f.create(t, "prod", d)
	got := f.runTest(t, "prod")
	f.assertFailed(t, got, "ENDPOINT_UNREACHABLE")
	if n := len(f.audits(t, "profile.test")); n != 1 {
		t.Fatalf("%d audit rows", n)
	}
}

func TestTestDraftNonHTTPAnswerIsUnreachable(t *testing.T) {
	f := newFixture(t)
	d := baseDocument()
	d.EndpointURL = garbageEndpoint(t)
	d.TimeoutSeconds = 5
	got, err := f.svc.TestDraft(context.Background(), f.root, "", d)
	if err != nil {
		t.Fatal(err)
	}
	f.assertFailed(t, got, "ENDPOINT_UNREACHABLE")
	if n := len(f.audits(t, "profile.test")); n != 1 {
		t.Fatalf("%d audit rows", n)
	}
}

// createServed stores a profile whose source is the fake at endpoint, with
// the credential its scenario asks for.
func (f *fixture) createServed(t *testing.T, id, endpoint string, fixture promtest.Fixture, allowed []string) {
	t.Helper()
	d := baseDocument()
	d.EndpointURL = endpoint
	d.AllowedNamespaces = allowed
	for _, r := range fixture.Requests {
		if token, ok := strings.CutPrefix(r.Authorization, "Bearer "); ok {
			writeFile(t, f.credDir, "token", token+"\n")
			d.CredentialFile = "token"
			break
		}
	}
	f.create(t, id, d)
}

func (f *fixture) namespaces(t *testing.T, actor auth.Principal, id string) NamespaceList {
	t.Helper()
	got, err := f.svc.Namespaces(context.Background(), actor, id)
	if err != nil {
		t.Fatal(err)
	}
	return got
}

func TestTestNamespaceQueryErrorIsMapped(t *testing.T) {
	f := newFixture(t)
	_, endpoint, fixture := fakePrometheusWith(t, namespacesAnswer(401))
	f.createServed(t, "prod", endpoint, fixture, []string{"payments"})
	f.assertFailed(t, f.runTest(t, "prod"), "ENDPOINT_AUTH_REJECTED")
}

func TestTestDraftNamespaceQueryErrorIsMapped(t *testing.T) {
	f := newFixture(t)
	_, endpoint, _ := fakePrometheusWith(t, namespacesAnswer(401))
	d := baseDocument()
	d.EndpointURL = endpoint
	got, err := f.svc.TestDraft(context.Background(), f.root, "", d)
	if err != nil {
		t.Fatal(err)
	}
	f.assertFailed(t, got, "ENDPOINT_AUTH_REJECTED")
}

func TestNamespacesLimited(t *testing.T) {
	f := newFixture(t)
	server, endpoint, fixture := fakePrometheus(t)
	f.createServed(t, "prod", endpoint, fixture, []string{"payments", "zeta", "absent"})
	ops := newPrincipal(t, f.pool, "ops", auth.RoleOperator)
	got := f.namespaces(t, ops, "prod")
	want := NamespaceList{Code: "PROFILE_OK", Namespaces: []string{"absent", "payments", "zeta"}, Limited: true}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v", got)
	}
	if n := len(server.Seen()); n != 0 {
		t.Fatalf("%d requests reached the source, want 0", n)
	}
	if n := len(f.audits(t, "profile.test")); n != 0 {
		t.Fatalf("%d audit rows", n)
	}
}

func TestNamespacesUnlimited(t *testing.T) {
	f := newFixture(t)
	server, endpoint, fixture := fakePrometheus(t)
	f.createServed(t, "prod", endpoint, fixture, []string{})
	got := f.namespaces(t, f.root, "prod")
	want := NamespaceList{Code: "PROFILE_OK", Namespaces: []string{"checkout", "payments", "zeta"}, Limited: false}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v", got)
	}
	var queries []string
	for _, seen := range server.Seen() {
		if q := seen.Form.Get("query"); q != "" {
			queries = append(queries, q)
		}
	}
	if wantQueries := []string{
		"vector(1)",
		"count by (cluster) (kube_node_info)",
		`count by (namespace) (kube_pod_status_phase{cluster="prod"})`,
	}; !slices.Equal(queries, wantQueries) {
		t.Fatalf("queries = %q, want %q", queries, wantQueries)
	}
}

func TestNamespacesFailingSource(t *testing.T) {
	f := newFixture(t)
	_, endpoint, fixture := fakePrometheusWith(t, namespacesAnswer(401))
	f.createServed(t, "prod", endpoint, fixture, []string{})
	got := f.namespaces(t, f.root, "prod")
	want := NamespaceList{Code: "ENDPOINT_AUTH_REJECTED", Namespaces: []string{}, Limited: false}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v", got)
	}
}

func TestNamespacesRefusals(t *testing.T) {
	f := newFixture(t)
	_, endpoint, fixture := fakePrometheus(t)
	f.createServed(t, "prod", endpoint, fixture, []string{})
	if _, err := f.svc.Namespaces(context.Background(), f.eve, "prod"); !errors.Is(err, ErrForbidden) {
		t.Fatalf("reader: %v", err)
	}
	if _, err := f.svc.Namespaces(context.Background(), f.root, "absent"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("unknown: %v", err)
	}
	d := baseDocument()
	d.Active = false
	f.create(t, "idle", d)
	if _, err := f.svc.Namespaces(context.Background(), f.root, "idle"); !errors.Is(err, ErrInactive) {
		t.Fatalf("inactive: %v", err)
	}
}
