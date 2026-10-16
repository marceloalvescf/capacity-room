package collect

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/marceloalvescf/capacity-room/internal/config"
	"github.com/marceloalvescf/capacity-room/internal/discovery"
	"github.com/marceloalvescf/capacity-room/internal/prom"
)

// workloadCapability builds the workload_usage capability with the given
// state and strategy.
func workloadCapability(state, strategy string) discovery.Capability {
	return discovery.Capability{
		Name:     "workload_usage",
		State:    state,
		Strategy: strategy,
		Evidence: []string{},
		Reasons:  []string{},
	}
}

// workloadTestInput builds the Input of a workload test: both capabilities
// Workloads needs, and the window of the original's tests, start 1, end 2,
// step "30s".
func workloadTestInput(scope discovery.Scope, cluster config.Cluster, analysis config.Analysis, workloadCap, resourceCap discovery.Capability) Input {
	return Input{
		Scope:        scope,
		Cluster:      cluster,
		Analysis:     analysis,
		Capabilities: []discovery.Capability{workloadCap, resourceCap},
		Window: Window{
			Start: time.Unix(1, 0),
			End:   time.Unix(2, 0),
			Step:  "30s",
		},
	}
}

// TestWorkloadRecordingQueriesInOrder transcribes the queries of the
// recording strategy of tests/test_workload_collector.py: seven texts, in
// order, the CPU text equal to section 1.3.a of original-collection.md,
// letter for letter.
func TestWorkloadRecordingQueriesInOrder(t *testing.T) {
	q := &scriptedQuerier{}
	cluster := config.Cluster{Name: "prod", Namespaces: []string{"payments"}, Label: "cluster", FilterMode: "required"}
	analysis := config.Analysis{ExcludedContainers: []string{}}
	in := workloadTestInput(testScope(true), cluster, analysis, workloadCapability(discovery.StateAvailable, discovery.StrategyRecordingRule), testCapability(discovery.StateAvailable, nil))

	if _, err := Workloads(context.Background(), q, in); err != nil {
		t.Fatalf("Workloads: %v", err)
	}

	want := []string{
		`sum by (namespace, workload, workload_type) (rate(container_cpu_usage_seconds_total{cluster="prod",namespace=~"(?:payments)",container!~"(?:|POD)"}[30s]) * on (cluster, namespace, pod) group_left(workload, workload_type) namespace_workload_pod:kube_pod_owner:relabel{cluster="prod",namespace=~"(?:payments)"})`,
		`sum by (namespace, workload, workload_type) (max by (cluster, namespace, pod, container) (container_memory_working_set_bytes{cluster="prod",namespace=~"(?:payments)",container!~"(?:|POD)"}) * on (cluster, namespace, pod) group_left(workload, workload_type) namespace_workload_pod:kube_pod_owner:relabel{cluster="prod",namespace=~"(?:payments)"})`,
		`sum by (namespace, workload, workload_type) (kube_pod_container_resource_requests{cluster="prod",resource="cpu",namespace=~"(?:payments)",container!~"(?:|POD)"} * on (cluster, namespace, pod) group_left(workload, workload_type) namespace_workload_pod:kube_pod_owner:relabel{cluster="prod",namespace=~"(?:payments)"})`,
		`sum by (namespace, workload, workload_type) (kube_pod_container_resource_requests{cluster="prod",resource="memory",namespace=~"(?:payments)",container!~"(?:|POD)"} * on (cluster, namespace, pod) group_left(workload, workload_type) namespace_workload_pod:kube_pod_owner:relabel{cluster="prod",namespace=~"(?:payments)"})`,
		`sum by (namespace, workload, workload_type) (kube_pod_container_resource_limits{cluster="prod",resource="cpu",namespace=~"(?:payments)",container!~"(?:|POD)"} * on (cluster, namespace, pod) group_left(workload, workload_type) namespace_workload_pod:kube_pod_owner:relabel{cluster="prod",namespace=~"(?:payments)"})`,
		`sum by (namespace, workload, workload_type) (kube_pod_container_resource_limits{cluster="prod",resource="memory",namespace=~"(?:payments)",container!~"(?:|POD)"} * on (cluster, namespace, pod) group_left(workload, workload_type) namespace_workload_pod:kube_pod_owner:relabel{cluster="prod",namespace=~"(?:payments)"})`,
		`count by (namespace, workload, workload_type) (namespace_workload_pod:kube_pod_owner:relabel{cluster="prod",namespace=~"(?:payments)"})`,
	}
	if len(q.queries) != len(want) {
		t.Fatalf("len(queries) = %d, want %d\nqueries: %v", len(q.queries), len(want), q.queries)
	}
	for i, w := range want {
		if q.queries[i] != w {
			t.Errorf("queries[%d] =\n%q, want\n%q", i, q.queries[i], w)
		}
	}
}

// TestWorkloadRecordingJoinWithoutScope covers a scope whose matcher is
// off: the join is on (namespace, pod), and no query holds a cluster
// matcher.
func TestWorkloadRecordingJoinWithoutScope(t *testing.T) {
	q := &scriptedQuerier{}
	in := workloadTestInput(testScope(false), defaultCluster(), defaultAnalysis(), workloadCapability(discovery.StateAvailable, discovery.StrategyRecordingRule), testCapability(discovery.StateUnavailable, []string{}))

	if _, err := Workloads(context.Background(), q, in); err != nil {
		t.Fatalf("Workloads: %v", err)
	}
	found := false
	for _, query := range q.queries {
		if strings.Contains(query, "on (namespace, pod)") {
			found = true
		}
		if strings.Contains(query, "cluster=") {
			t.Errorf("query %q holds a cluster matcher", query)
		}
	}
	if !found {
		t.Errorf("no query holds the join on (namespace, pod)\nqueries: %v", q.queries)
	}
}

// TestWorkloadRecordingJoinUsesTheScopeLabel covers a scope labelled
// "k8s_cluster": the join is on (k8s_cluster, namespace, pod).
func TestWorkloadRecordingJoinUsesTheScopeLabel(t *testing.T) {
	q := &scriptedQuerier{}
	scope := discovery.Scope{
		ClusterName:      "prod",
		ClusterLabel:     "k8s_cluster",
		FilterMode:       "required",
		MatcherEnabled:   true,
		Confidence:       discovery.ConfidenceExact,
		DiscoveredValues: []string{"prod"},
	}
	in := workloadTestInput(scope, defaultCluster(), defaultAnalysis(), workloadCapability(discovery.StateAvailable, discovery.StrategyRecordingRule), testCapability(discovery.StateUnavailable, []string{}))

	if _, err := Workloads(context.Background(), q, in); err != nil {
		t.Fatalf("Workloads: %v", err)
	}
	found := false
	for _, query := range q.queries {
		if strings.Contains(query, "on (k8s_cluster, namespace, pod)") {
			found = true
		}
	}
	if !found {
		t.Errorf("no query holds the join on (k8s_cluster, namespace, pod)\nqueries: %v", q.queries)
	}
}

// TestWorkloadRawQueriesInOrder transcribes
// test_raw_owner_fallback_uses_only_controllers_and_is_partial: eight
// queries, both owner queries holding owner_is_controller="true", and a
// partial state.
func TestWorkloadRawQueriesInOrder(t *testing.T) {
	q := &scriptedQuerier{}
	in := workloadTestInput(testScope(true), defaultCluster(), defaultAnalysis(), workloadCapability(discovery.StateAvailable, discovery.StrategyRawMetrics), testCapability(discovery.StateAvailable, nil))

	result, err := Workloads(context.Background(), q, in)
	if err != nil {
		t.Fatalf("Workloads: %v", err)
	}
	if len(q.queries) != 8 {
		t.Fatalf("len(queries) = %d, want 8\nqueries: %v", len(q.queries), q.queries)
	}
	if !strings.Contains(q.queries[0], "kube_pod_owner") {
		t.Errorf("queries[0] = %q, want it to hold kube_pod_owner", q.queries[0])
	}
	if !strings.Contains(q.queries[1], "kube_replicaset_owner") {
		t.Errorf("queries[1] = %q, want it to hold kube_replicaset_owner", q.queries[1])
	}
	for i := 0; i < 2; i++ {
		if !strings.Contains(q.queries[i], `owner_is_controller="true"`) {
			t.Errorf(`queries[%d] = %q, want it to hold owner_is_controller="true"`, i, q.queries[i])
		}
	}
	names := []string{
		"container_cpu_usage_seconds_total", "container_memory_working_set_bytes",
		"kube_pod_container_resource_requests", "kube_pod_container_resource_requests",
		"kube_pod_container_resource_limits", "kube_pod_container_resource_limits",
	}
	for i, name := range names {
		if !strings.Contains(q.queries[i+2], name) {
			t.Errorf("queries[%d] = %q, want it to hold %q", i+2, q.queries[i+2], name)
		}
	}
	if result.State != discovery.StatePartial {
		t.Errorf("State = %q, want %q", result.State, discovery.StatePartial)
	}
}

// TestWorkloadRawKeepsUnownedPod transcribes
// test_raw_metrics_preserve_unowned_pods_as_per_pod_data: a pod the owner
// queries do not name becomes a row of kind Pod under its own name, with
// no known pod count.
func TestWorkloadRawKeepsUnownedPod(t *testing.T) {
	orphanSeries := []prom.Series{{
		Labels: map[string]string{"namespace": "payments", "pod": "orphan-123"},
		Points: []prom.Point{{Time: 1, Value: 1, Valid: true}, {Time: 2, Value: 2, Valid: true}},
	}}
	q := &scriptedQuerier{rangeFunc: rangeTable(
		rangeCase{[]string{"container_cpu_usage_seconds_total"}, orphanSeries},
		rangeCase{[]string{"container_memory_working_set_bytes"}, orphanSeries},
	)}
	in := workloadTestInput(testScope(true), defaultCluster(), defaultAnalysis(), workloadCapability(discovery.StateAvailable, discovery.StrategyRawMetrics), testCapability(discovery.StateUnavailable, []string{}))

	result, err := Workloads(context.Background(), q, in)
	if err != nil {
		t.Fatalf("Workloads: %v", err)
	}
	if len(result.Rows) != 1 {
		t.Fatalf("len(Rows) = %d, want 1", len(result.Rows))
	}
	row := result.Rows[0]
	if row.Namespace != "payments" || row.Kind != "Pod" || row.Name != "orphan-123" {
		t.Errorf("row = %+v, want payments/Pod/orphan-123", row)
	}
	if row.PodsKnown {
		t.Errorf("PodsKnown = true, want false")
	}
}

// TestWorkloadRawCountsPodsFromOwners covers three pods the owner queries
// give to one deployment: the row's pod count is 3, and no query asks
// "count by" for it.
func TestWorkloadRawCountsPodsFromOwners(t *testing.T) {
	podOwners := []prom.Sample{
		{Labels: map[string]string{"namespace": "payments", "pod": "web-1", "owner_kind": "ReplicaSet", "owner_name": "web-abc"}, Value: 1, Valid: true},
		{Labels: map[string]string{"namespace": "payments", "pod": "web-2", "owner_kind": "ReplicaSet", "owner_name": "web-abc"}, Value: 1, Valid: true},
		{Labels: map[string]string{"namespace": "payments", "pod": "web-3", "owner_kind": "ReplicaSet", "owner_name": "web-abc"}, Value: 1, Valid: true},
	}
	replicaOwners := []prom.Sample{
		{Labels: map[string]string{"namespace": "payments", "replicaset": "web-abc", "owner_kind": "Deployment", "owner_name": "web"}, Value: 1, Valid: true},
	}
	usage := []prom.Series{{
		Labels: map[string]string{"namespace": "payments", "pod": "web-1"},
		Points: []prom.Point{{Time: 1, Value: 1, Valid: true}},
	}}
	q := &scriptedQuerier{
		queryFunc: func(query string) ([]prom.Sample, error) {
			switch {
			case strings.Contains(query, "kube_pod_owner"):
				return podOwners, nil
			case strings.Contains(query, "kube_replicaset_owner"):
				return replicaOwners, nil
			}
			return nil, nil
		},
		rangeFunc: rangeTable(rangeCase{[]string{"container_cpu_usage_seconds_total"}, usage}),
	}
	in := workloadTestInput(testScope(true), defaultCluster(), defaultAnalysis(), workloadCapability(discovery.StateAvailable, discovery.StrategyRawMetrics), testCapability(discovery.StateUnavailable, []string{}))

	result, err := Workloads(context.Background(), q, in)
	if err != nil {
		t.Fatalf("Workloads: %v", err)
	}
	if len(result.Rows) != 1 {
		t.Fatalf("len(Rows) = %d, want 1", len(result.Rows))
	}
	row := result.Rows[0]
	if row.Pods != 3 || !row.PodsKnown {
		t.Errorf("Pods = %d, PodsKnown = %v, want 3, true", row.Pods, row.PodsKnown)
	}
	for _, query := range q.queries {
		if strings.Contains(query, "count by") {
			t.Errorf("query %q holds a count by; the raw strategy sends none", query)
		}
	}
}

// TestWorkloadRawAlwaysWarnsOfHistory covers the raw strategy's first
// warning: it always names the current association between pods and
// workloads.
func TestWorkloadRawAlwaysWarnsOfHistory(t *testing.T) {
	q := &scriptedQuerier{}
	in := workloadTestInput(testScope(true), defaultCluster(), defaultAnalysis(), workloadCapability(discovery.StateAvailable, discovery.StrategyRawMetrics), testCapability(discovery.StateUnavailable, []string{}))

	result, err := Workloads(context.Background(), q, in)
	if err != nil {
		t.Fatalf("Workloads: %v", err)
	}
	if len(result.Warnings) == 0 || result.Warnings[0].Code != WarningWorkloadHistoryCurrentAssociation {
		t.Fatalf("Warnings[0] = %v, want %q first", result.Warnings, WarningWorkloadHistoryCurrentAssociation)
	}
}

// TestWorkloadUnavailableSendsNothing transcribes
// test_unavailable_workload_capability_makes_no_queries: no query, state
// unavailable, no row.
func TestWorkloadUnavailableSendsNothing(t *testing.T) {
	q := &scriptedQuerier{}
	in := workloadTestInput(testScope(true), defaultCluster(), defaultAnalysis(), workloadCapability(discovery.StateUnavailable, discovery.StrategyNone), testCapability(discovery.StateAvailable, nil))

	result, err := Workloads(context.Background(), q, in)
	if err != nil {
		t.Fatalf("Workloads: %v", err)
	}
	if len(q.queries) != 0 {
		t.Fatalf("len(queries) = %d, want 0\nqueries: %v", len(q.queries), q.queries)
	}
	if result.State != discovery.StateUnavailable {
		t.Errorf("State = %q, want %q", result.State, discovery.StateUnavailable)
	}
	if len(result.Rows) != 0 {
		t.Errorf("len(Rows) = %d, want 0", len(result.Rows))
	}
}

// TestWorkloadSkipsRequestsAndLimits covers requests_limits unavailable:
// three queries in the recording strategy, four in the raw one, none of
// them holding a requests or limits metric.
func TestWorkloadSkipsRequestsAndLimits(t *testing.T) {
	for _, tc := range []struct {
		name     string
		strategy string
		want     int
	}{
		{"recording", discovery.StrategyRecordingRule, 3},
		{"raw", discovery.StrategyRawMetrics, 4},
	} {
		t.Run(tc.name, func(t *testing.T) {
			q := &scriptedQuerier{}
			in := workloadTestInput(testScope(true), defaultCluster(), defaultAnalysis(), workloadCapability(discovery.StateAvailable, tc.strategy), testCapability(discovery.StateUnavailable, []string{}))

			if _, err := Workloads(context.Background(), q, in); err != nil {
				t.Fatalf("Workloads: %v", err)
			}
			if len(q.queries) != tc.want {
				t.Fatalf("len(queries) = %d, want %d\nqueries: %v", len(q.queries), tc.want, q.queries)
			}
			for _, query := range q.queries {
				if strings.Contains(query, "resource_requests") || strings.Contains(query, "resource_limits") {
					t.Errorf("query %q holds a resource requests or limits metric", query)
				}
			}
		})
	}
}

// workloadConsumptionQuerier answers the recording strategy's pod count
// with two workloads, "has-usage" and "no-usage", and its CPU usage with a
// series for "has-usage" alone.
func workloadConsumptionQuerier() *scriptedQuerier {
	podCount := []prom.Sample{
		{Labels: map[string]string{"namespace": "payments", "workload": "has-usage", "workload_type": "Deployment"}, Value: 1, Valid: true},
		{Labels: map[string]string{"namespace": "payments", "workload": "no-usage", "workload_type": "Deployment"}, Value: 1, Valid: true},
	}
	usage := []prom.Series{{
		Labels: map[string]string{"namespace": "payments", "workload": "has-usage", "workload_type": "Deployment"},
		Points: []prom.Point{{Time: 1, Value: 1, Valid: true}},
	}}
	return &scriptedQuerier{
		queryFunc: func(string) ([]prom.Sample, error) { return podCount, nil },
		rangeFunc: rangeTable(rangeCase{[]string{"container_cpu_usage_seconds_total"}, usage}),
	}
}

// TestWorkloadOmitsWithoutConsumption covers workload_usage available with
// one of two workloads holding no observed consumption: one row, and the
// notice of the omission with count 1.
func TestWorkloadOmitsWithoutConsumption(t *testing.T) {
	q := workloadConsumptionQuerier()
	in := workloadTestInput(testScope(true), defaultCluster(), defaultAnalysis(), workloadCapability(discovery.StateAvailable, discovery.StrategyRecordingRule), testCapability(discovery.StateUnavailable, []string{}))

	result, err := Workloads(context.Background(), q, in)
	if err != nil {
		t.Fatalf("Workloads: %v", err)
	}
	if len(result.Rows) != 1 || result.Rows[0].Name != "has-usage" {
		t.Fatalf("Rows = %+v, want one row named has-usage", result.Rows)
	}
	if len(result.Notices) != 1 || result.Notices[0].Code != NoticeWorkloadsOmittedNoUsage || result.Notices[0].Params["count"] != 1 {
		t.Fatalf("Notices = %+v, want one WORKLOADS_OMITTED_NO_USAGE with count 1", result.Notices)
	}
}

// TestWorkloadPartialKeepsWithoutConsumption covers workload_usage partial
// with the same answers: both rows stay, and no notice is added.
func TestWorkloadPartialKeepsWithoutConsumption(t *testing.T) {
	q := workloadConsumptionQuerier()
	in := workloadTestInput(testScope(true), defaultCluster(), defaultAnalysis(), workloadCapability(discovery.StatePartial, discovery.StrategyRecordingRule), testCapability(discovery.StateUnavailable, []string{}))

	result, err := Workloads(context.Background(), q, in)
	if err != nil {
		t.Fatalf("Workloads: %v", err)
	}
	if len(result.Rows) != 2 {
		t.Fatalf("len(Rows) = %d, want 2", len(result.Rows))
	}
	if len(result.Notices) != 0 {
		t.Errorf("Notices = %v, want none", result.Notices)
	}
}

// TestWorkloadOmitsJobs transcribes the three Job readers of the original:
// a recording answer with one Job, one with two, and a raw answer with
// one. No row of kind Job survives, and the notice names the count of
// each.
func TestWorkloadOmitsJobs(t *testing.T) {
	for _, tc := range []struct {
		name      string
		strategy  string
		queryFunc func(string) ([]prom.Sample, error)
		rangeFunc func(string) ([]prom.Series, error)
		wantCount float64
	}{
		{
			name:     "one job, recording",
			strategy: discovery.StrategyRecordingRule,
			queryFunc: func(string) ([]prom.Sample, error) {
				return []prom.Sample{
					{Labels: map[string]string{"namespace": "payments", "workload": "api", "workload_type": "Deployment"}, Value: 1, Valid: true},
					{Labels: map[string]string{"namespace": "payments", "workload": "backup-28123456", "workload_type": "Job"}, Value: 1, Valid: true},
				}, nil
			},
			rangeFunc: func(query string) ([]prom.Series, error) {
				if !strings.Contains(query, "container_cpu_usage_seconds_total") {
					return nil, nil
				}
				return []prom.Series{
					{Labels: map[string]string{"namespace": "payments", "workload": "api", "workload_type": "Deployment"}, Points: []prom.Point{{Time: 1, Value: 1, Valid: true}}},
					{Labels: map[string]string{"namespace": "payments", "workload": "backup-28123456", "workload_type": "Job"}, Points: []prom.Point{{Time: 1, Value: 1, Valid: true}}},
				}, nil
			},
			wantCount: 1,
		},
		{
			name:     "two jobs, recording",
			strategy: discovery.StrategyRecordingRule,
			queryFunc: func(string) ([]prom.Sample, error) {
				return []prom.Sample{
					{Labels: map[string]string{"namespace": "payments", "workload": "api", "workload_type": "Deployment"}, Value: 1, Valid: true},
					{Labels: map[string]string{"namespace": "payments", "workload": "backup-28123456", "workload_type": "Job"}, Value: 1, Valid: true},
					{Labels: map[string]string{"namespace": "payments", "workload": "cleanup-28123456", "workload_type": "Job"}, Value: 1, Valid: true},
				}, nil
			},
			rangeFunc: func(query string) ([]prom.Series, error) {
				if !strings.Contains(query, "container_cpu_usage_seconds_total") {
					return nil, nil
				}
				return []prom.Series{
					{Labels: map[string]string{"namespace": "payments", "workload": "api", "workload_type": "Deployment"}, Points: []prom.Point{{Time: 1, Value: 1, Valid: true}}},
					{Labels: map[string]string{"namespace": "payments", "workload": "backup-28123456", "workload_type": "Job"}, Points: []prom.Point{{Time: 1, Value: 1, Valid: true}}},
					{Labels: map[string]string{"namespace": "payments", "workload": "cleanup-28123456", "workload_type": "Job"}, Points: []prom.Point{{Time: 1, Value: 1, Valid: true}}},
				}, nil
			},
			wantCount: 2,
		},
		{
			name:     "one job, raw",
			strategy: discovery.StrategyRawMetrics,
			queryFunc: func(query string) ([]prom.Sample, error) {
				switch {
				case strings.Contains(query, "kube_pod_owner"):
					return []prom.Sample{
						{Labels: map[string]string{"namespace": "payments", "pod": "api-1", "owner_kind": "ReplicaSet", "owner_name": "api-abc"}, Value: 1, Valid: true},
						{Labels: map[string]string{"namespace": "payments", "pod": "backup-1", "owner_kind": "Job", "owner_name": "backup-28123456"}, Value: 1, Valid: true},
					}, nil
				case strings.Contains(query, "kube_replicaset_owner"):
					return []prom.Sample{
						{Labels: map[string]string{"namespace": "payments", "replicaset": "api-abc", "owner_kind": "Deployment", "owner_name": "api"}, Value: 1, Valid: true},
					}, nil
				}
				return nil, nil
			},
			rangeFunc: func(query string) ([]prom.Series, error) {
				if !strings.Contains(query, "container_cpu_usage_seconds_total") {
					return nil, nil
				}
				return []prom.Series{
					{Labels: map[string]string{"namespace": "payments", "pod": "api-1"}, Points: []prom.Point{{Time: 1, Value: 1, Valid: true}}},
					{Labels: map[string]string{"namespace": "payments", "pod": "backup-1"}, Points: []prom.Point{{Time: 1, Value: 1, Valid: true}}},
				}, nil
			},
			wantCount: 1,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			q := &scriptedQuerier{queryFunc: tc.queryFunc, rangeFunc: tc.rangeFunc}
			in := workloadTestInput(testScope(true), defaultCluster(), defaultAnalysis(), workloadCapability(discovery.StateAvailable, tc.strategy), testCapability(discovery.StateUnavailable, []string{}))

			result, err := Workloads(context.Background(), q, in)
			if err != nil {
				t.Fatalf("Workloads: %v", err)
			}
			for _, row := range result.Rows {
				if row.Kind == "Job" {
					t.Errorf("Rows holds a Job: %+v", row)
				}
			}
			if len(result.Notices) != 1 || result.Notices[0].Code != NoticeJobsOmitted || result.Notices[0].Params["count"] != tc.wantCount {
				t.Fatalf("Notices = %+v, want one JOBS_OMITTED with count %v", result.Notices, tc.wantCount)
			}
		})
	}
}

// TestWorkloadOmitsAJobSeenOnlyInTheCount transcribes
// test_unobserved_job_is_disclosed_once_without_consumption_omission: a Job
// named only by the pod count, never by a usage answer, is disclosed once,
// by the Job notice alone.
func TestWorkloadOmitsAJobSeenOnlyInTheCount(t *testing.T) {
	podCount := []prom.Sample{
		{Labels: map[string]string{"namespace": "payments", "workload": "api", "workload_type": "Deployment"}, Value: 1, Valid: true},
		{Labels: map[string]string{"namespace": "payments", "workload": "backup-28123456", "workload_type": "Job"}, Value: 1, Valid: true},
	}
	usage := []prom.Series{{
		Labels: map[string]string{"namespace": "payments", "workload": "api", "workload_type": "Deployment"},
		Points: []prom.Point{{Time: 1, Value: 1, Valid: true}},
	}}
	q := &scriptedQuerier{
		queryFunc: func(string) ([]prom.Sample, error) { return podCount, nil },
		rangeFunc: rangeTable(rangeCase{[]string{"container_cpu_usage_seconds_total"}, usage}),
	}
	in := workloadTestInput(testScope(true), defaultCluster(), defaultAnalysis(), workloadCapability(discovery.StateAvailable, discovery.StrategyRecordingRule), testCapability(discovery.StateUnavailable, []string{}))

	result, err := Workloads(context.Background(), q, in)
	if err != nil {
		t.Fatalf("Workloads: %v", err)
	}
	if len(result.Notices) != 1 || result.Notices[0].Code != NoticeJobsOmitted || result.Notices[0].Params["count"] != 1 {
		t.Fatalf("Notices = %+v, want one JOBS_OMITTED with count 1", result.Notices)
	}
}

// TestWorkloadKindIsCanonical transcribes the two readers of lower-case
// kinds: a lower-case "statefulset" gives a row of kind StatefulSet that
// finds its pod count, and a lower-case "job" is still omitted.
func TestWorkloadKindIsCanonical(t *testing.T) {
	podCount := []prom.Sample{
		{Labels: map[string]string{"namespace": "payments", "workload": "api", "workload_type": "statefulset"}, Value: 3, Valid: true},
		{Labels: map[string]string{"namespace": "payments", "workload": "backup-28123456", "workload_type": "job"}, Value: 1, Valid: true},
	}
	usage := []prom.Series{
		{Labels: map[string]string{"namespace": "payments", "workload": "api", "workload_type": "statefulset"}, Points: []prom.Point{{Time: 1, Value: 1, Valid: true}}},
		{Labels: map[string]string{"namespace": "payments", "workload": "backup-28123456", "workload_type": "job"}, Points: []prom.Point{{Time: 1, Value: 1, Valid: true}}},
	}
	q := &scriptedQuerier{
		queryFunc: func(string) ([]prom.Sample, error) { return podCount, nil },
		rangeFunc: rangeTable(rangeCase{[]string{"container_cpu_usage_seconds_total"}, usage}),
	}
	in := workloadTestInput(testScope(true), defaultCluster(), defaultAnalysis(), workloadCapability(discovery.StateAvailable, discovery.StrategyRecordingRule), testCapability(discovery.StateUnavailable, []string{}))

	result, err := Workloads(context.Background(), q, in)
	if err != nil {
		t.Fatalf("Workloads: %v", err)
	}
	if len(result.Rows) != 1 {
		t.Fatalf("len(Rows) = %d, want 1", len(result.Rows))
	}
	row := result.Rows[0]
	if row.Kind != "StatefulSet" || row.Pods != 3 || !row.PodsKnown {
		t.Errorf("row = %+v, want kind StatefulSet, Pods 3, PodsKnown true", row)
	}
}

// TestWorkloadWithASlashStopsTheCollector covers a workload whose
// namespace, name, or kind holds a slash, in either strategy: the
// collector stops and returns a *prom.Error of kind KindQuery instead of
// keying a row by it, since a name of Kubernetes never holds a slash and
// such a label is a fault of the endpoint or of the configuration.
func TestWorkloadWithASlashStopsTheCollector(t *testing.T) {
	assertSlashError := func(t *testing.T, err error) {
		t.Helper()
		var promErr *prom.Error
		if !errors.As(err, &promErr) || promErr.Kind != prom.KindQuery {
			t.Fatalf("err = %v, want a *prom.Error with Kind KindQuery", err)
		}
		if !strings.Contains(promErr.Message, "a workload name that holds a slash") {
			t.Errorf("message %q does not say a workload name holds a slash", promErr.Message)
		}
	}

	t.Run("name, recording strategy", func(t *testing.T) {
		usage := []prom.Series{{
			Labels: map[string]string{"namespace": "payments", "workload": "b/c", "workload_type": "Deployment"},
			Points: []prom.Point{{Time: 1, Value: 1, Valid: true}},
		}}
		q := &scriptedQuerier{rangeFunc: rangeTable(rangeCase{[]string{"container_cpu_usage_seconds_total"}, usage})}
		in := workloadTestInput(testScope(true), defaultCluster(), defaultAnalysis(), workloadCapability(discovery.StateAvailable, discovery.StrategyRecordingRule), testCapability(discovery.StateUnavailable, []string{}))

		_, err := Workloads(context.Background(), q, in)
		assertSlashError(t, err)
		if !strings.Contains(err.Error(), `"b/c"`) {
			t.Errorf("message %q does not name the workload", err.Error())
		}
	})

	t.Run("name, raw strategy", func(t *testing.T) {
		podOwners := []prom.Sample{
			{Labels: map[string]string{"namespace": "payments", "pod": "web-1", "owner_kind": "ReplicaSet", "owner_name": "web-abc"}, Value: 1, Valid: true},
		}
		replicaOwners := []prom.Sample{
			{Labels: map[string]string{"namespace": "payments", "replicaset": "web-abc", "owner_kind": "Deployment", "owner_name": "b/c"}, Value: 1, Valid: true},
		}
		q := &scriptedQuerier{
			queryFunc: func(query string) ([]prom.Sample, error) {
				switch {
				case strings.Contains(query, "kube_pod_owner"):
					return podOwners, nil
				case strings.Contains(query, "kube_replicaset_owner"):
					return replicaOwners, nil
				}
				return nil, nil
			},
		}
		in := workloadTestInput(testScope(true), defaultCluster(), defaultAnalysis(), workloadCapability(discovery.StateAvailable, discovery.StrategyRawMetrics), testCapability(discovery.StateUnavailable, []string{}))

		_, err := Workloads(context.Background(), q, in)
		assertSlashError(t, err)
		if !strings.Contains(err.Error(), `"b/c"`) {
			t.Errorf("message %q does not name the workload", err.Error())
		}
	})

	t.Run("kind", func(t *testing.T) {
		usage := []prom.Series{{
			Labels: map[string]string{"namespace": "payments", "workload": "api", "workload_type": "apps/Deployment"},
			Points: []prom.Point{{Time: 1, Value: 1, Valid: true}},
		}}
		q := &scriptedQuerier{rangeFunc: rangeTable(rangeCase{[]string{"container_cpu_usage_seconds_total"}, usage})}
		in := workloadTestInput(testScope(true), defaultCluster(), defaultAnalysis(), workloadCapability(discovery.StateAvailable, discovery.StrategyRecordingRule), testCapability(discovery.StateUnavailable, []string{}))

		_, err := Workloads(context.Background(), q, in)
		assertSlashError(t, err)
		if !strings.Contains(err.Error(), `"apps/Deployment"`) {
			t.Errorf("message %q does not name the kind", err.Error())
		}
	})
}

// TestWorkloadNoBase transcribes
// test_available_request_family_without_workload_series_renders_not_defined
// at the collector: usage present, requests_limits available, the request
// series empty; the request ratios of the workload read no_base.
func TestWorkloadNoBase(t *testing.T) {
	usage := []prom.Series{{
		Labels: map[string]string{"namespace": "payments", "workload": "api", "workload_type": "Deployment"},
		Points: []prom.Point{{Time: 1, Value: 1, Valid: true}},
	}}
	q := &scriptedQuerier{rangeFunc: rangeTable(
		rangeCase{[]string{"container_cpu_usage_seconds_total"}, usage},
		rangeCase{[]string{"container_memory_working_set_bytes"}, usage},
	)}
	in := workloadTestInput(testScope(true), defaultCluster(), defaultAnalysis(), workloadCapability(discovery.StateAvailable, discovery.StrategyRecordingRule), testCapability(discovery.StateAvailable, nil))

	result, err := Workloads(context.Background(), q, in)
	if err != nil {
		t.Fatalf("Workloads: %v", err)
	}
	if len(result.Rows) != 1 {
		t.Fatalf("len(Rows) = %d, want 1", len(result.Rows))
	}
	if result.Rows[0].Resources.CPURequestPercent.State != StateNoBase {
		t.Errorf("CPURequestPercent.State = %q, want %q", result.Rows[0].Resources.CPURequestPercent.State, StateNoBase)
	}
}

// TestWorkloadNumbersOfRawFallback transcribes the answers of the scenario
// "raw-fallback" (testdata/prometheus/raw-fallback.json, requests 32, 33,
// and 57 to 62, by their 0-based index) and asserts the whole row against
// the numbers the original recorded for it in
// testdata/python/raw-fallback.json, workloads[0]. The scenario file
// itself is not read here; its numbers are transcribed above.
func TestWorkloadNumbersOfRawFallback(t *testing.T) {
	const t1, t2 = 1784050200, 1784093400

	podOwners := []prom.Sample{{
		Labels: map[string]string{"namespace": "payments", "pod": "payments-web-abc", "owner_kind": "ReplicaSet", "owner_name": "payments-web-abc"},
		Value:  1, Valid: true,
	}}
	replicaOwners := []prom.Sample{{
		Labels: map[string]string{"namespace": "payments", "replicaset": "payments-web-abc", "owner_kind": "Deployment", "owner_name": "payments-web"},
		Value:  1, Valid: true,
	}}
	series := func(value1, value2 float64) []prom.Series {
		return []prom.Series{{
			Labels: map[string]string{"namespace": "payments", "pod": "payments-web-abc"},
			Points: []prom.Point{
				{Time: t1, Value: value1, Valid: true},
				{Time: t2, Value: value2, Valid: true},
			},
		}}
	}
	cpuUsage := series(0.2, 0.4)
	memoryUsage := series(100, 120)
	cpuRequests := series(0.5, 0.6)
	memoryRequests := series(0.5, 0.6)
	cpuLimits := series(1.0, 1.2)
	memoryLimits := series(1.0, 1.2)

	q := &scriptedQuerier{
		queryFunc: func(query string) ([]prom.Sample, error) {
			switch {
			case strings.Contains(query, "kube_pod_owner"):
				return podOwners, nil
			case strings.Contains(query, "kube_replicaset_owner"):
				return replicaOwners, nil
			}
			return nil, nil
		},
		rangeFunc: rangeTable(
			rangeCase{[]string{"container_cpu_usage_seconds_total"}, cpuUsage},
			rangeCase{[]string{"container_memory_working_set_bytes"}, memoryUsage},
			rangeCase{[]string{"kube_pod_container_resource_requests", `resource="cpu"`}, cpuRequests},
			rangeCase{[]string{"kube_pod_container_resource_requests", `resource="memory"`}, memoryRequests},
			rangeCase{[]string{"kube_pod_container_resource_limits", `resource="cpu"`}, cpuLimits},
			rangeCase{[]string{"kube_pod_container_resource_limits", `resource="memory"`}, memoryLimits},
		),
	}
	in := workloadTestInput(testScope(true), defaultCluster(), defaultAnalysis(), workloadCapability(discovery.StateAvailable, discovery.StrategyRawMetrics), testCapability(discovery.StateAvailable, nil))

	result, err := Workloads(context.Background(), q, in)
	if err != nil {
		t.Fatalf("Workloads: %v", err)
	}
	if len(result.Rows) != 1 {
		t.Fatalf("len(Rows) = %d, want 1", len(result.Rows))
	}
	row := result.Rows[0]
	if row.Namespace != "payments" || row.Kind != "Deployment" || row.Name != "payments-web" {
		t.Fatalf("row = %+v, want payments/Deployment/payments-web", row)
	}
	if row.Pods != 1 || !row.PodsKnown {
		t.Errorf("Pods = %d, PodsKnown = %v, want 1, true", row.Pods, row.PodsKnown)
	}
	if row.Approximate {
		t.Errorf("Approximate = true, want false")
	}

	res := row.Resources
	checkStatistics(t, res.CPUUsage, Statistics{State: StateValue, Average: 0.30000000000000004, Minimum: 0.2, Peak: 0.4, Current: 0.4})
	checkStatistics(t, res.MemoryUsage, Statistics{State: StateValue, Average: 110.0, Minimum: 100.0, Peak: 120.0, Current: 120.0})
	checkStatistics(t, res.CPURequests, Statistics{State: StateValue, Average: 0.55, Minimum: 0.5, Peak: 0.6, Current: 0.6})
	checkStatistics(t, res.MemoryRequests, Statistics{State: StateValue, Average: 0.55, Minimum: 0.5, Peak: 0.6, Current: 0.6})
	checkStatistics(t, res.CPULimits, Statistics{State: StateValue, Average: 1.1, Minimum: 1.0, Peak: 1.2, Current: 1.2})
	checkStatistics(t, res.MemoryLimits, Statistics{State: StateValue, Average: 1.1, Minimum: 1.0, Peak: 1.2, Current: 1.2})
	checkStatistics(t, res.CPURequestPercent, Statistics{State: StateValue, Average: 53.333333333333336, Minimum: 40.0, Peak: 66.66666666666667, Current: 66.66666666666667})
	checkStatistics(t, res.MemoryRequestPercent, Statistics{State: StateValue, Average: 20000.0, Minimum: 20000.0, Peak: 20000.0, Current: 20000.0})
	checkStatistics(t, res.CPULimitPercent, Statistics{State: StateValue, Average: 26.666666666666668, Minimum: 20.0, Peak: 33.333333333333336, Current: 33.333333333333336})
	checkStatistics(t, res.MemoryLimitPercent, Statistics{State: StateValue, Average: 10000.0, Minimum: 10000.0, Peak: 10000.0, Current: 10000.0})
}

// TestWorkloadRowsAreSorted covers workloads in two namespaces, two of one
// name and two kinds: the rows are sorted by namespace, then name, then
// kind.
func TestWorkloadRowsAreSorted(t *testing.T) {
	podCount := []prom.Sample{
		{Labels: map[string]string{"namespace": "b", "workload": "svc", "workload_type": "Deployment"}, Value: 1, Valid: true},
		{Labels: map[string]string{"namespace": "a", "workload": "svc", "workload_type": "Deployment"}, Value: 1, Valid: true},
		{Labels: map[string]string{"namespace": "a", "workload": "svc", "workload_type": "StatefulSet"}, Value: 1, Valid: true},
		{Labels: map[string]string{"namespace": "a", "workload": "api", "workload_type": "Deployment"}, Value: 1, Valid: true},
	}
	q := &scriptedQuerier{queryFunc: func(string) ([]prom.Sample, error) { return podCount, nil }}
	cluster := config.Cluster{Name: "prod", Namespaces: []string{"a", "b"}, Label: "cluster", FilterMode: "required"}
	in := workloadTestInput(testScope(true), cluster, defaultAnalysis(), workloadCapability(discovery.StatePartial, discovery.StrategyRecordingRule), testCapability(discovery.StateUnavailable, []string{}))

	result, err := Workloads(context.Background(), q, in)
	if err != nil {
		t.Fatalf("Workloads: %v", err)
	}
	want := []Workload{
		{Namespace: "a", Kind: "Deployment", Name: "api"},
		{Namespace: "a", Kind: "Deployment", Name: "svc"},
		{Namespace: "a", Kind: "StatefulSet", Name: "svc"},
		{Namespace: "b", Kind: "Deployment", Name: "svc"},
	}
	if len(result.Rows) != len(want) {
		t.Fatalf("len(Rows) = %d, want %d\nRows: %+v", len(result.Rows), len(want), result.Rows)
	}
	for i, w := range want {
		got := result.Rows[i]
		if got.Namespace != w.Namespace || got.Name != w.Name || got.Kind != w.Kind {
			t.Errorf("Rows[%d] = %s/%s/%s, want %s/%s/%s", i, got.Namespace, got.Name, got.Kind, w.Namespace, w.Name, w.Kind)
		}
	}
}

// TestAllRunsNamespacesFirst covers a run where requests_limits and
// workload_usage are available and the HPA and node discovery capabilities
// are not. The twenty-nine queries are: seven of the namespace collector,
// seven of the workload collector, none of the HPA collector (both capabilities
// unavailable), ten of the node collector (the eight of identity, capacity,
// allocatable, ready and unschedulable, and the two requests queries), and five
// of the autoscaling collector.
func TestAllRunsNamespacesFirst(t *testing.T) {
	capabilities := []discovery.Capability{
		testCapability(discovery.StateAvailable, nil),
		workloadCapability(discovery.StateAvailable, discovery.StrategyRecordingRule),
		hpaCapability("hpa_history", discovery.StateUnavailable, nil, nil),
		hpaCapability("workloads_without_hpa", discovery.StateUnavailable, nil, nil),
		nodeCapability("node_utilization", discovery.StateUnavailable, discovery.StrategyNone, nil, nil),
		nodeCapability("nodepool_discovery", discovery.StateUnavailable, discovery.StrategyNone, nil, nil),
		nodeCapability("node_autoscaling", discovery.StateAvailable, discovery.StrategyRawMetrics, []string{"kube_configmap_info"}, nil),
	}
	in := Input{
		Scope: testScope(true), Cluster: defaultCluster(), Analysis: defaultAnalysis(), Capabilities: capabilities,
		Window: Window{Start: time.Unix(1, 0), End: time.Unix(2, 0), Step: "30s"},
	}
	ctx := context.Background()

	nsQuerier := &scriptedQuerier{}
	if _, err := Namespaces(ctx, nsQuerier, in); err != nil {
		t.Fatalf("Namespaces: %v", err)
	}
	workloadQuerier := &scriptedQuerier{}
	if _, err := Workloads(ctx, workloadQuerier, in); err != nil {
		t.Fatalf("Workloads: %v", err)
	}
	nodeQuerier := &scriptedQuerier{}
	if _, err := Nodes(ctx, nodeQuerier, in); err != nil {
		t.Fatalf("Nodes: %v", err)
	}
	autoscalingQuerier := &scriptedQuerier{}
	if _, err := Autoscaling(ctx, autoscalingQuerier, in); err != nil {
		t.Fatalf("Autoscaling: %v", err)
	}
	for name, got := range map[string]int{
		"namespaces": len(nsQuerier.queries), "workloads": len(workloadQuerier.queries),
		"nodes": len(nodeQuerier.queries), "autoscaling": len(autoscalingQuerier.queries),
	} {
		want := map[string]int{"namespaces": 7, "workloads": 7, "nodes": 10, "autoscaling": 5}[name]
		if got != want {
			t.Fatalf("the %s collector alone sent %d queries, want %d", name, got, want)
		}
	}

	allQuerier := &scriptedQuerier{}
	collected, err := All(ctx, allQuerier, in)
	if err != nil {
		t.Fatalf("All: %v", err)
	}
	var want []string
	for _, group := range [][]string{nsQuerier.queries, workloadQuerier.queries, nodeQuerier.queries, autoscalingQuerier.queries} {
		want = append(want, group...)
	}
	if len(allQuerier.queries) != 29 {
		t.Fatalf("len(queries) = %d, want 29\nqueries: %v", len(allQuerier.queries), allQuerier.queries)
	}
	for i := range want {
		if allQuerier.queries[i] != want[i] {
			t.Errorf("queries[%d] =\n%q, want\n%q", i, allQuerier.queries[i], want[i])
		}
	}
	if collected.Namespaces.State == "" || collected.Workloads.State == "" {
		t.Errorf("Collected = %+v, want both states set", collected)
	}
}

// TestAllJoinsMessagesOnce covers both collectors warning of their own
// state: the joined warnings hold the codes of both, then the one warning of
// the node collector (the HPA capabilities carry no reason, so the HPA
// results add none), in the order of the collectors, none twice.
func TestAllJoinsMessagesOnce(t *testing.T) {
	cluster := defaultCluster()
	analysis := defaultAnalysis()
	scope := testScope(true)
	capabilities := []discovery.Capability{
		testCapability(discovery.StateUnavailable, []string{}),
		workloadCapability(discovery.StateAvailable, discovery.StrategyRecordingRule),
		hpaCapability("hpa_history", discovery.StateUnavailable, nil, nil),
		hpaCapability("workloads_without_hpa", discovery.StateUnavailable, nil, nil),
		nodeCapability("node_utilization", discovery.StateUnavailable, discovery.StrategyNone, nil, nil),
		nodeCapability("nodepool_discovery", discovery.StateUnavailable, discovery.StrategyNone, nil, nil),
		nodeCapability("node_autoscaling", discovery.StatePartial, discovery.StrategyNone, nil, []string{"NODE_AUTOSCALING_UNDETERMINED"}),
	}
	in := Input{
		Scope: scope, Cluster: cluster, Analysis: analysis, Capabilities: capabilities,
		Window: Window{Start: time.Unix(1, 0), End: time.Unix(2, 0), Step: "30s"},
	}

	q := &scriptedQuerier{}
	collected, err := All(context.Background(), q, in)
	if err != nil {
		t.Fatalf("All: %v", err)
	}
	wantCodes := []string{
		WarningNamespaceUsageIncomplete,
		WarningNamespaceRequestsLimitsUnavailable,
		WarningWorkloadUsageIncomplete,
		WarningWorkloadRequestsLimitsUnavailable,
		WarningNodeIdentityUnavailable,
	}
	warnings := collected.Warnings()
	if len(warnings) != len(wantCodes) {
		t.Fatalf("Warnings = %+v, want %d entries", warnings, len(wantCodes))
	}
	for i, code := range wantCodes {
		if warnings[i].Code != code {
			t.Errorf("Warnings[%d].Code = %q, want %q", i, warnings[i].Code, code)
		}
	}
}
