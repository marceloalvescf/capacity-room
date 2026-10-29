//go:build postgres

package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/marceloalvescf/capacity-room/internal/platform/auth"
	"github.com/marceloalvescf/capacity-room/internal/promtest"
)

const (
	profilesPath      = "/api/v1/admin/profiles"
	readerProfilesURL = "/api/v1/profiles"
)

// fakeEndpoint serves the direct-complete scenario and returns the endpoint
// URL that reaches it through the stubbed name, and the credential file
// name to use when the scenario's requests carry a token.
func (ts *testServer) fakeEndpoint(t *testing.T) (endpoint, credential string) {
	t.Helper()
	return ts.fakeEndpointWith(t, namespacesAnswer(200, "zeta", "checkout", "payments"))
}

// namespacesAnswer scripts the answer to the namespace listing: the names as
// the series of the query, in the order given, with the status.
func namespacesAnswer(status int, names ...string) promtest.Request {
	result := make([]map[string]any, 0, len(names))
	for _, name := range names {
		result = append(result, map[string]any{"metric": map[string]string{"namespace": name}, "value": []any{1784136600, "3"}})
	}
	body, err := json.Marshal(map[string]any{"status": "success", "data": map[string]any{"resultType": "vector", "result": result}})
	if err != nil {
		panic(err)
	}
	return promtest.Request{
		Method: "POST", Path: "/api/v1/query", QueryContains: "count by (namespace) (kube_pod_status_phase",
		Status: status, Body: body,
	}
}

// fakeEndpointWith serves the direct-complete scenario after the given
// answer to the namespace listing.
func (ts *testServer) fakeEndpointWith(t *testing.T, namespaces promtest.Request) (endpoint, credential string) {
	t.Helper()
	fixture, err := promtest.LoadScenario("direct-complete")
	if err != nil {
		t.Fatal(err)
	}
	fixture = promtest.Merge(promtest.Fixture{Requests: []promtest.Request{namespaces}}, fixture)
	server := httptest.NewServer(promtest.NewServer(fixture))
	t.Cleanup(server.Close)
	addr := server.Listener.Addr().String()
	endpoint = "http://metrics.test:" + addr[strings.LastIndex(addr, ":")+1:] + "/api/v1"
	for _, r := range fixture.Requests {
		if token, ok := strings.CutPrefix(r.Authorization, "Bearer "); ok {
			writeCredential(t, ts.Profiles.Settings.CredentialDir, "token", token+"\n")
			return endpoint, "token"
		}
	}
	return endpoint, ""
}

func writeCredential(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func createBody(endpoint, credential string, extra string) string {
	b := `{"id":"prod","display_name":"Production","endpoint_url":"` + endpoint + `",` +
		`"cluster_name":"prod","allowed_namespaces":["payments"],"active":true`
	if credential != "" {
		b += `,"credential_file":"` + credential + `"`
	}
	return b + extra + `}`
}

func decodeProfile(t *testing.T, w *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &m); err != nil {
		t.Fatalf("body %q: %v", w.Body.String(), err)
	}
	return m
}

func (ts *testServer) createProfile(t *testing.T) map[string]any {
	t.Helper()
	cookie, csrf := ts.signIn(t, "root")
	endpoint, credential := ts.fakeEndpoint(t)
	w := ts.send(t, "POST", profilesPath, createBody(endpoint, credential, ""), cookie, csrf)
	if w.Code != 201 {
		t.Fatalf("create: %d %s", w.Code, w.Body.String())
	}
	return decodeProfile(t, w)
}

func TestProfilesListAsReader(t *testing.T) {
	ts := newTestServer(t)
	ts.createProfile(t)
	cookie, _ := ts.signIn(t, "eve")
	w := ts.call(t, "GET", readerProfilesURL, "", nil, cookie)
	if w.Code != 200 {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
	var list struct {
		Profiles []map[string]any `json:"profiles"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &list); err != nil {
		t.Fatal(err)
	}
	if len(list.Profiles) != 1 {
		t.Fatalf("profiles %v", list.Profiles)
	}
	var keys []string
	for k := range list.Profiles[0] {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	want := []string{
		"allowed_namespaces", "default_period", "default_report_profile", "default_timezone",
		"display_name", "id", "permitted_periods", "permitted_report_profiles", "permitted_timezones",
	}
	if !reflect.DeepEqual(keys, want) {
		t.Fatalf("keys %v", keys)
	}
	if list.Profiles[0]["id"] != "prod" || list.Profiles[0]["default_period"] != "24h" {
		t.Fatalf("summary %v", list.Profiles[0])
	}
}

func TestProfilesListWithoutSession(t *testing.T) {
	ts := newTestServer(t)
	wantProblem(t, ts.call(t, "GET", readerProfilesURL, "", nil), 401, CodeUnauthenticated, nil)
}

func TestAdminProfilesCreateReadReviseDelete(t *testing.T) {
	ts := newTestServer(t)
	created := ts.createProfile(t)
	cookie, csrf := ts.signIn(t, "root")
	if created["id"] != "prod" || created["revision"] != float64(1) || created["created_at"] != "2026-10-05T12:00:00Z" ||
		created["revision_id"] == "" || created["cluster_name"] != "prod" || created["thresholds"] == nil {
		t.Fatalf("created %v", created)
	}

	w := ts.call(t, "GET", profilesPath+"/prod", "", nil, cookie)
	if w.Code != 200 || !reflect.DeepEqual(decodeProfile(t, w), created) {
		t.Fatalf("get: %d %s", w.Code, w.Body.String())
	}
	w = ts.call(t, "GET", profilesPath, "", nil, cookie)
	var list struct {
		Profiles []map[string]any `json:"profiles"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &list); err != nil || w.Code != 200 || len(list.Profiles) != 1 || !reflect.DeepEqual(list.Profiles[0], created) {
		t.Fatalf("list: %d %s", w.Code, w.Body.String())
	}

	w = ts.send(t, "PATCH", profilesPath+"/prod", `{"base_revision_id":"`+created["revision_id"].(string)+`","display_name":"Prod"}`, cookie, csrf)
	revised := decodeProfile(t, w)
	if w.Code != 200 || revised["revision"] != float64(2) || revised["display_name"] != "Prod" {
		t.Fatalf("patch: %d %s", w.Code, w.Body.String())
	}

	w = ts.send(t, "DELETE", profilesPath+"/prod", "", cookie, csrf)
	if w.Code != 204 || w.Body.Len() != 0 {
		t.Fatalf("delete: %d %q", w.Code, w.Body.String())
	}
	wantProblem(t, ts.call(t, "GET", profilesPath+"/prod", "", nil, cookie), 404, CodeProfileNotFound, nil)
}

func TestAdminProfilesAsReader(t *testing.T) {
	ts := newTestServer(t)
	cookie, csrf := ts.signIn(t, "eve")
	for _, c := range []struct{ method, path, body string }{
		{"GET", profilesPath, ""},
		{"POST", profilesPath, `{"id":"prod"}`},
		{"GET", profilesPath + "/nothing", ""},
		{"PATCH", profilesPath + "/nothing", `{}`},
		{"DELETE", profilesPath + "/nothing", ""},
		{"POST", profilesPath + "/nothing/test", ""},
	} {
		wantProblem(t, ts.send(t, c.method, c.path, c.body, cookie, csrf), 403, CodeForbidden, nil)
	}
}

func TestAdminProfilesCreateRefusals(t *testing.T) {
	ts := newTestServer(t)
	cookie, csrf := ts.signIn(t, "root")
	const head = `{"id":"prod","display_name":"P","endpoint_url":"https://metrics.test/api/v1"`
	for _, c := range []struct {
		name, body string
		status     int
		code       Code
		fields     map[string]string
	}{
		{"missing cluster", head + `,"allowed_namespaces":["a"]}`, 400, CodeRequestInvalid, map[string]string{"cluster_name": "missing"}},
		{"empty cluster", head + `,"cluster_name":"","allowed_namespaces":["a"]}`, 422, CodeProfileDocumentInvalid, map[string]string{"cluster_name": "empty"}},
		{"bad url", `{"id":"prod","display_name":"P","endpoint_url":"not a url","cluster_name":"c","allowed_namespaces":["a"]}`, 422, CodeEndpointURLRejected, map[string]string{"endpoint_url": "invalid"}},
		{"repeated namespace", head + `,"cluster_name":"c","allowed_namespaces":["a","a"]}`, 422, CodeProfileDocumentInvalid, map[string]string{"allowed_namespaces": "repeated"}},
		{"unknown key", head + `,"cluster_name":"c","allowed_namespaces":["a"],"x":1}`, 400, CodeRequestInvalid, map[string]string{"x": "unknown"}},
		{"endpoint type", head + `,"cluster_name":"c","allowed_namespaces":["a"],"endpoint_type":"prometheus"}`, 400, CodeRequestInvalid, map[string]string{"endpoint_type": "unknown"}},
	} {
		t.Run(c.name, func(t *testing.T) {
			wantProblem(t, ts.send(t, "POST", profilesPath, c.body, cookie, csrf), c.status, c.code, c.fields)
		})
	}
	if n := ts.count(t, `SELECT count(*) FROM profiles`); n != 0 {
		t.Fatalf("%d profiles written", n)
	}
}

func TestAdminProfilesCreateTaken(t *testing.T) {
	ts := newTestServer(t)
	ts.createProfile(t)
	cookie, csrf := ts.signIn(t, "root")
	endpoint, credential := ts.fakeEndpoint(t)
	wantProblem(t, ts.send(t, "POST", profilesPath, createBody(endpoint, credential, ""), cookie, csrf), 409, CodeProfileIDTaken, nil)
}

func TestAdminProfilesPatchStale(t *testing.T) {
	ts := newTestServer(t)
	created := ts.createProfile(t)
	cookie, csrf := ts.signIn(t, "root")
	base := created["revision_id"].(string)
	if w := ts.send(t, "PATCH", profilesPath+"/prod", `{"base_revision_id":"`+base+`","display_name":"Two"}`, cookie, csrf); w.Code != 200 {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
	wantProblem(t, ts.send(t, "PATCH", profilesPath+"/prod", `{"base_revision_id":"`+base+`","display_name":"Three"}`, cookie, csrf), 409, CodeProfileRevisionStale, nil)
}

func TestAdminProfilesPatchNull(t *testing.T) {
	ts := newTestServer(t)
	cookie, csrf := ts.signIn(t, "root")
	endpoint, credential := ts.fakeEndpoint(t)
	w := ts.send(t, "POST", profilesPath, createBody(endpoint, credential, `,"default_period":"7d"`), cookie, csrf)
	created := decodeProfile(t, w)
	if w.Code != 201 || created["default_period"] != "7d" {
		t.Fatalf("create: %d %s", w.Code, w.Body.String())
	}
	w = ts.send(t, "PATCH", profilesPath+"/prod", `{"base_revision_id":"`+created["revision_id"].(string)+`","default_period":null}`, cookie, csrf)
	revised := decodeProfile(t, w)
	if v, present := revised["default_period"]; w.Code != 200 || !present || v != nil || revised["revision"] != float64(2) {
		t.Fatalf("patch: %d %s", w.Code, w.Body.String())
	}
}

func TestAdminProfilesPatchRequiresBase(t *testing.T) {
	ts := newTestServer(t)
	ts.createProfile(t)
	cookie, csrf := ts.signIn(t, "root")
	wantProblem(t, ts.send(t, "PATCH", profilesPath+"/prod", `{"display_name":"X"}`, cookie, csrf), 400, CodeRequestInvalid, map[string]string{"base_revision_id": "missing"})
}

func TestAdminProfilesTest(t *testing.T) {
	ts := newTestServer(t)
	ts.createProfile(t)
	cookie, csrf := ts.signIn(t, "root")
	w := ts.send(t, "POST", profilesPath+"/prod/test", "", cookie, csrf)
	if w.Code != 200 {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
	var got ProfileTestResult
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	want := ProfileTestResult{Ok: true, Code: "PROFILE_OK", Namespaces: []string{"checkout", "payments", "zeta"}, Capabilities: []string{
		"namespace_usage", "workload_usage", "requests_limits", "hpa_history", "workloads_without_hpa",
		"ip_node_mapping", "nodepool_discovery", "node_utilization", "node_autoscaling",
	}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v", got)
	}
	if strings.Contains(w.Body.String(), "http") {
		t.Fatalf("body holds a URL: %s", w.Body.String())
	}
}

func TestAdminProfilesTestRefused(t *testing.T) {
	ts := newTestServer(t)
	ts.createProfile(t)
	ts.Profiles.Policy.Networks = []netip.Prefix{netip.MustParsePrefix("10.0.0.0/8")}
	cookie, csrf := ts.signIn(t, "root")
	w := ts.send(t, "POST", profilesPath+"/prod/test", "", cookie, csrf)
	var got map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	want := map[string]any{"ok": false, "code": "ENDPOINT_ADDRESS_REJECTED", "capabilities": []any{}, "namespaces": []any{}}
	if w.Code != 200 || !reflect.DeepEqual(got, want) || strings.Contains(w.Body.String(), "http") {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
}

func TestProfilesRoutesRequireCSRF(t *testing.T) {
	ts := newTestServer(t)
	cookie, _ := ts.signIn(t, "root")
	for _, c := range []struct{ method, path, body string }{
		{"POST", profilesPath, `{"id":"prod"}`},
		{"PATCH", profilesPath + "/prod", `{}`},
		{"DELETE", profilesPath + "/prod", ""},
		{"POST", profilesPath + "/prod/test", ""},
	} {
		wantProblem(t, ts.call(t, c.method, c.path, c.body, nil, cookie), http.StatusForbidden, CodeCSRFRejected, nil)
	}
}

const draftPath = profilesPath + "/test"

// draftBody is the create body without its id.
func draftBody(endpoint, credential, extra string) string {
	return strings.Replace(createBody(endpoint, credential, extra), `"id":"prod",`, "", 1)
}

func TestAdminProfilesTestDraft(t *testing.T) {
	ts := newTestServer(t)
	cookie, csrf := ts.signIn(t, "root")
	endpoint, credential := ts.fakeEndpoint(t)
	w := ts.send(t, "POST", draftPath, draftBody(endpoint, credential, ""), cookie, csrf)
	var got map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	want := map[string]any{"ok": true, "code": "PROFILE_OK", "namespaces": []any{"checkout", "payments", "zeta"}, "capabilities": []any{
		"namespace_usage", "workload_usage", "requests_limits", "hpa_history", "workloads_without_hpa",
		"ip_node_mapping", "nodepool_discovery", "node_utilization", "node_autoscaling",
	}}
	if w.Code != 200 || !reflect.DeepEqual(got, want) {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
	list := ts.call(t, "GET", profilesPath, "", nil, cookie)
	if list.Code != 200 || !strings.Contains(list.Body.String(), `"profiles":[]`) {
		t.Fatalf("list %d %s", list.Code, list.Body.String())
	}
}

func TestAdminProfilesTestDraftOrder(t *testing.T) {
	ts := newTestServer(t)
	endpoint, credential := ts.fakeEndpoint(t)
	good := draftBody(endpoint, credential, "")
	wantProblem(t, ts.call(t, "POST", draftPath, good, nil), 401, CodeUnauthenticated, nil)

	reader, readerCSRF := ts.signIn(t, "eve")
	wantProblem(t, ts.send(t, "POST", draftPath, good, reader, readerCSRF), 403, CodeForbidden, nil)

	cookie, csrf := ts.signIn(t, "root")
	wantProblem(t, ts.call(t, "POST", draftPath, good, nil, cookie), 403, CodeCSRFRejected, nil)

	huge := draftBody(endpoint, credential, `,"display_name_pad":"`+strings.Repeat("x", 65*1024)+`"`)
	wantProblem(t, ts.send(t, "POST", draftPath, huge, cookie, csrf), 400, CodeRequestInvalid, map[string]string{"": "too_long"})
	wantProblem(t, ts.send(t, "POST", draftPath, draftBody(endpoint, credential, `,"x":1`), cookie, csrf), 400, CodeRequestInvalid, map[string]string{"x": "unknown"})
	wantProblem(t, ts.send(t, "POST", draftPath, draftBody(endpoint, credential, `,"endpoint_type":"prometheus"`), cookie, csrf), 400, CodeRequestInvalid, map[string]string{"endpoint_type": "unknown"})
	noName := strings.Replace(good, `"display_name":"Production",`, "", 1)
	wantProblem(t, ts.send(t, "POST", draftPath, noName, cookie, csrf), 400, CodeRequestInvalid, map[string]string{"display_name": "missing"})
}

func TestAdminProfilesTestDraftRefused(t *testing.T) {
	ts := newTestServer(t)
	cookie, csrf := ts.signIn(t, "root")
	endpoint, credential := ts.fakeEndpoint(t)
	body := strings.Replace(draftBody(endpoint, credential, ""), `"cluster_name":"prod"`, `"cluster_name":" "`, 1)
	wantProblem(t, ts.send(t, "POST", draftPath, body, cookie, csrf), 422, CodeProfileDocumentInvalid, map[string]string{"cluster_name": "empty"})
	wantProblem(t, ts.send(t, "POST", draftPath, draftBody("http://169.254.169.254/api/v1", "", ""), cookie, csrf), 422, CodeEndpointAddressRejected, map[string]string{"endpoint_url": "invalid"})
}

func TestAdminProfilesNamedTest(t *testing.T) {
	ts := newTestServer(t)
	cookie, csrf := ts.signIn(t, "root")
	endpoint, credential := ts.fakeEndpoint(t)
	body := strings.Replace(createBody(endpoint, credential, ""), `"id":"prod"`, `"id":"test"`, 1)
	if w := ts.send(t, "POST", profilesPath, body, cookie, csrf); w.Code != 201 {
		t.Fatalf("create %d %s", w.Code, w.Body.String())
	}
	if w := ts.call(t, "GET", profilesPath+"/test", "", nil, cookie); w.Code != 200 {
		t.Fatalf("get %d %s", w.Code, w.Body.String())
	}
	got := decodeProfile(t, ts.call(t, "GET", profilesPath+"/test", "", nil, cookie))
	patch := `{"base_revision_id":"` + got["revision_id"].(string) + `","display_name":"Renamed"}`
	w := ts.send(t, "PATCH", profilesPath+"/test", patch, cookie, csrf)
	if w.Code != 200 || decodeProfile(t, w)["display_name"] != "Renamed" {
		t.Fatalf("patch %d %s", w.Code, w.Body.String())
	}
	if w := ts.send(t, "DELETE", profilesPath+"/test", "", cookie, csrf); w.Code != 204 {
		t.Fatalf("delete %d %s", w.Code, w.Body.String())
	}
}

func TestAdminProfilesPatchJudgesBodyBeforeID(t *testing.T) {
	ts := newTestServer(t)
	cookie, csrf := ts.signIn(t, "root")
	path := profilesPath + "/Bad_Id"
	base := `"base_revision_id":"` + uuid.NewString() + `"`
	wantProblem(t, ts.send(t, "PATCH", path, `{`+base+`,"nope":1}`, cookie, csrf), 400, CodeRequestInvalid, map[string]string{"nope": "unknown"})
	wantProblem(t, ts.send(t, "PATCH", path, `{`+base+`}`, cookie, csrf), 404, CodeProfileNotFound, nil)
}

const namespacesURL = readerProfilesURL + "/%s/namespaces"

// createNamespaced stores the profile id over a fake source that answers the
// namespace listing as given, with the allowed namespaces as a JSON list.
func (ts *testServer) createNamespaced(t *testing.T, id, allowed string, active bool, answer promtest.Request) {
	t.Helper()
	cookie, csrf := ts.signIn(t, "root")
	endpoint, credential := ts.fakeEndpointWith(t, answer)
	body := `{"id":"` + id + `","display_name":"Profile","endpoint_url":"` + endpoint + `",` +
		`"cluster_name":"prod","allowed_namespaces":` + allowed + `,"active":` + strconv.FormatBool(active)
	if credential != "" {
		body += `,"credential_file":"` + credential + `"`
	}
	if w := ts.send(t, "POST", profilesPath, body+"}", cookie, csrf); w.Code != 201 {
		t.Fatalf("create: %d %s", w.Code, w.Body.String())
	}
}

func (ts *testServer) listNamespaces(t *testing.T, id string, cookie *http.Cookie) *httptest.ResponseRecorder {
	t.Helper()
	path := fmt.Sprintf(namespacesURL, id)
	if cookie == nil {
		return ts.call(t, "GET", path, "", nil)
	}
	return ts.call(t, "GET", path, "", nil, cookie)
}

func TestProfileNamespacesRefusals(t *testing.T) {
	ts := newTestServer(t)
	answer := namespacesAnswer(200, "payments")
	ts.createNamespaced(t, "prod", `[]`, true, answer)
	ts.createNamespaced(t, "idle", `[]`, false, answer)
	wantProblem(t, ts.listNamespaces(t, "prod", nil), 401, CodeUnauthenticated, nil)
	eve, _ := ts.signIn(t, "eve")
	wantProblem(t, ts.listNamespaces(t, "prod", eve), 403, CodeForbidden, nil)
	root, _ := ts.signIn(t, "root")
	wantProblem(t, ts.listNamespaces(t, "absent", root), 404, CodeProfileNotFound, nil)
	wantProblem(t, ts.listNamespaces(t, "Bad%20Id", root), 404, CodeProfileNotFound, nil)
	wantProblem(t, ts.listNamespaces(t, "idle", root), 409, CodeProfileInactive, nil)
}

func TestProfileNamespacesAnswers(t *testing.T) {
	cases := []struct {
		name    string
		allowed string
		answer  promtest.Request
		want    ProfileNamespaces
	}{
		{"limited", `["payments","zeta","absent"]`, namespacesAnswer(200, "zeta", "checkout", "payments"),
			ProfileNamespaces{Code: "PROFILE_OK", Namespaces: []string{"payments", "zeta"}, Limited: true}},
		{"unlimited", `[]`, namespacesAnswer(200, "zeta", "checkout", "payments"),
			ProfileNamespaces{Code: "PROFILE_OK", Namespaces: []string{"checkout", "payments", "zeta"}, Limited: false}},
		{"failing", `["payments"]`, namespacesAnswer(401),
			ProfileNamespaces{Code: "ENDPOINT_AUTH_REJECTED", Namespaces: []string{}, Limited: true}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ts := newTestServer(t)
			ts.createNamespaced(t, "prod", c.allowed, true, c.answer)
			before := ts.count(t, countAudits)
			cookie, _ := ts.signIn(t, "root")
			w := ts.listNamespaces(t, "prod", cookie)
			if w.Code != 200 {
				t.Fatalf("%d %s", w.Code, w.Body.String())
			}
			var got ProfileNamespaces
			if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, c.want) {
				t.Fatalf("got %+v", got)
			}
			if strings.Contains(w.Body.String(), "http") {
				t.Fatalf("body holds a URL: %s", w.Body.String())
			}
			// Signing in writes its own audit row; the read writes none.
			if after := ts.count(t, countAudits); after != before+1 {
				t.Fatalf("audit rows %d, want %d", after, before+1)
			}
		})
	}
}

func TestProfileNamespacesForAnOperator(t *testing.T) {
	ts := newTestServer(t)
	ts.createNamespaced(t, "prod", `[]`, true, namespacesAnswer(200, "payments"))
	ops := ts.signInAs(t, seed(t, ts.pool, ts.now, "ops", opsPassword, auth.RoleOperator), opsPassword)
	w := ts.listNamespaces(t, "prod", ops.cookie)
	if w.Code != 200 {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
}
