//go:build postgres

package profiles

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"reflect"
	"strings"
	"testing"
	"time"
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
	want := TestResult{OK: true, Code: "PROFILE_OK", Capabilities: []string{
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
