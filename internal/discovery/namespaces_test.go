package discovery

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/marceloalvescf/capacity-room/internal/prom"
)

func namespaceSeries(names ...string) []prom.Sample {
	samples := make([]prom.Sample, 0, len(names))
	for _, name := range names {
		samples = append(samples, series(map[string]string{"namespace": name}))
	}
	return samples
}

func TestListNamespacesQueryText(t *testing.T) {
	at := time.Unix(1700000000, 0)
	tests := []struct {
		name     string
		scope    Scope
		lookback time.Duration
		want     string
	}{
		{"matcher on", Scope{ClusterName: "prod", ClusterLabel: "cluster", MatcherEnabled: true}, 0,
			`count by (namespace) (kube_pod_status_phase{cluster="prod"})`},
		{"matcher off", Scope{ClusterName: "prod", ClusterLabel: "cluster"}, 0,
			`count by (namespace) (kube_pod_status_phase)`},
		{"lookback of a week", Scope{ClusterName: "prod", ClusterLabel: "cluster", MatcherEnabled: true}, 7 * 24 * time.Hour,
			`count by (namespace) (max_over_time(kube_pod_status_phase{cluster="prod"}[604800s]))`},
		{"lookback under a second", Scope{}, 999 * time.Millisecond,
			`count by (namespace) (kube_pod_status_phase)`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			q := &fakeQuerier{}
			if _, err := ListNamespaces(context.Background(), q, tt.scope, at, tt.lookback); err != nil {
				t.Fatalf("ListNamespaces: %v", err)
			}
			if len(q.queries) != 1 || q.queries[0].query != tt.want {
				t.Fatalf("queries = %v, want [%q]", q.queries, tt.want)
			}
			if !q.queries[0].at.Equal(at) {
				t.Errorf("at = %v, want %v", q.queries[0].at, at)
			}
		})
	}
}

func TestListNamespacesSortsDedupesAndFilters(t *testing.T) {
	query := `count by (namespace) (kube_pod_status_phase)`
	q := &fakeQuerier{answers: map[string][]prom.Sample{
		query: namespaceSeries("payments", "api", "payments", "Bad_Name", "-lead", "trail-", "", "a1"),
	}}

	got, err := ListNamespaces(context.Background(), q, Scope{}, time.Now(), 0)
	if err != nil {
		t.Fatalf("ListNamespaces: %v", err)
	}
	want := []string{"a1", "api", "payments"}
	if !slices.Equal(got, want) {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestListNamespacesEmptyAnswerIsNotNil(t *testing.T) {
	got, err := ListNamespaces(context.Background(), &fakeQuerier{}, Scope{}, time.Now(), 0)
	if err != nil {
		t.Fatalf("ListNamespaces: %v", err)
	}
	if got == nil || len(got) != 0 {
		t.Fatalf("got %#v, want []string{}", got)
	}
}

func TestListNamespacesDropsAllInvalid(t *testing.T) {
	q := &fakeQuerier{answers: map[string][]prom.Sample{
		`count by (namespace) (kube_pod_status_phase)`: namespaceSeries("UPPER"),
	}}
	got, err := ListNamespaces(context.Background(), q, Scope{}, time.Now(), 0)
	if err != nil || got == nil || len(got) != 0 {
		t.Fatalf("got %#v, %v, want []string{}, nil", got, err)
	}
}

func TestListNamespacesPassesTheErrorThrough(t *testing.T) {
	boom := errors.New("boom")
	got, err := ListNamespaces(context.Background(), &fakeQuerier{err: boom}, Scope{}, time.Now(), 0)
	if err != boom {
		t.Fatalf("err = %v, want %v", err, boom)
	}
	if got != nil {
		t.Fatalf("got %#v, want nil", got)
	}
}
