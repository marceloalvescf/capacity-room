//go:build postgres

package worker

import (
	"context"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/riverqueue/river/rivertype"

	"github.com/marceloalvescf/capacity-room/internal/platform/profiles"
	"github.com/marceloalvescf/capacity-room/internal/platform/reports"
	"github.com/marceloalvescf/capacity-room/internal/platform/store"
	"github.com/marceloalvescf/capacity-room/internal/promtest"
	"github.com/marceloalvescf/capacity-room/internal/snapshot"
)

const discoveryPrefix = "count by (namespace) (max_over_time(kube_pod_status_phase"

// discoveryEnv serves direct-complete, answers the namespace discovery query
// with discovered, records whether it was asked, and adds a profile whose
// allowed list is allowed.
func discoveryEnv(t *testing.T, allowed, discovered []string) (*env, func() bool) {
	t.Helper()
	e := newEnv(t, nil)
	fixture := scenarioFixture(t)
	fake := promtest.NewServer(fixture)
	var mu sync.Mutex
	asked := false
	endpoint := serve(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		if strings.HasPrefix(r.PostFormValue("query"), discoveryPrefix) {
			mu.Lock()
			asked = true
			mu.Unlock()
			var result []string
			for _, name := range discovered {
				result = append(result, fmt.Sprintf(`{"metric":{"namespace":%q},"value":[1784140200,"1"]}`, name))
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = fmt.Fprintf(w, `{"status":"success","data":{"resultType":"vector","result":[%s]}}`, strings.Join(result, ","))
			return
		}
		fake.ServeHTTP(w, r)
	}))
	cred := ""
	if token := tokenOf(fixture); token != "" {
		e.writeCredential("token", token+"\n")
		cred = "token"
	}
	d := profiles.Defaults()
	d.DisplayName = "Production"
	d.Active = true
	d.EndpointURL = endpoint
	d.ClusterName = "prod"
	d.AllowedNamespaces = allowed
	d.CredentialFile = cred
	if _, err := e.profiles.Create(context.Background(), e.root, "prod", d); err != nil {
		t.Fatal(err)
	}
	return e, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return asked
	}
}

func collectedNamespaces(t *testing.T, e *env, namespaces []string) []string {
	t.Helper()
	e.start(collectHooks{})
	acc, err := e.reports.Request(context.Background(), e.op, "key-"+t.Name(),
		reports.Selection{ProfileID: "prod", Namespaces: namespaces, Language: ptrTo(e.language())})
	if err != nil {
		t.Fatal(err)
	}
	report := e.report(acc.ID)
	e.waitJob(report.CollectJobID, rivertype.JobStateCompleted)
	if got := e.report(report.ID); got.Status != "collected" {
		t.Fatalf("report %+v", got)
	}
	_, body, _, err := store.GetSnapshot(context.Background(), e.pool, report.ID)
	if err != nil {
		t.Fatal(err)
	}
	snap, err := snapshot.Decode(body)
	if err != nil {
		t.Fatal(err)
	}
	return snap.Scope.Namespaces
}

func TestCollectRunsTheRequestedNamespaces(t *testing.T) {
	e, asked := discoveryEnv(t, []string{"payments", "billing"}, []string{"other"})
	got := collectedNamespaces(t, e, []string{"payments"})
	if !slices.Equal(got, []string{"payments"}) || asked() {
		t.Fatalf("namespaces = %q, discovery asked = %v; want [payments], false", got, asked())
	}
}

func TestCollectRunsTheAllowedNamespacesForANullSelection(t *testing.T) {
	e, asked := discoveryEnv(t, []string{"payments"}, []string{"other"})
	got := collectedNamespaces(t, e, nil)
	if !slices.Equal(got, []string{"payments"}) || asked() {
		t.Fatalf("namespaces = %q, discovery asked = %v; want [payments], false", got, asked())
	}
}

func TestCollectDiscoversForANullSelectionAndNoLimit(t *testing.T) {
	e, asked := discoveryEnv(t, []string{}, []string{"payments"})
	got := collectedNamespaces(t, e, nil)
	if !slices.Equal(got, []string{"payments"}) || !asked() {
		t.Fatalf("namespaces = %q, discovery asked = %v; want [payments], true", got, asked())
	}
}
