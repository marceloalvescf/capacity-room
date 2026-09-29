package goldentest

import (
	"context"
	"strings"
	"testing"
)

// edgeAnswer names, by the path of the request and a fragment its own query
// text alone holds, one of the fourteen answers edge-values replaces: the
// two instant queries of the collection phase and the twelve range queries
// the namespace and the workload collector each send for CPU usage, memory
// usage, requests, and limits.
type edgeAnswer struct {
	name, path, fragment string
}

var edgeAnswers = []edgeAnswer{
	{"namespace pods", "/api/v1/query", "kube_pod_status_phase"},
	{"workload pods", "/api/v1/query", "count by (namespace, workload, workload_type) (namespace_workload_pod:kube_pod_owner:relabel"},
	{"namespace cpu usage", "/api/v1/query_range", "sum by (namespace) (rate(container_cpu_usage_seconds_total{"},
	{"namespace memory usage", "/api/v1/query_range", "sum by (namespace) (max by (namespace, pod, container) (container_memory_working_set_bytes{"},
	{"namespace cpu requests", "/api/v1/query_range", `sum by (namespace) (kube_pod_container_resource_requests{cluster="prod",resource="cpu"`},
	{"namespace memory requests", "/api/v1/query_range", `sum by (namespace) (kube_pod_container_resource_requests{cluster="prod",resource="memory"`},
	{"namespace cpu limits", "/api/v1/query_range", `sum by (namespace) (kube_pod_container_resource_limits{cluster="prod",resource="cpu"`},
	{"namespace memory limits", "/api/v1/query_range", `sum by (namespace) (kube_pod_container_resource_limits{cluster="prod",resource="memory"`},
	{"workload cpu usage", "/api/v1/query_range", "sum by (namespace, workload, workload_type) (rate(container_cpu_usage_seconds_total{"},
	{"workload memory usage", "/api/v1/query_range", "sum by (namespace, workload, workload_type) (max by (cluster, namespace, pod, container) (container_memory_working_set_bytes{"},
	{"workload cpu requests", "/api/v1/query_range", `sum by (namespace, workload, workload_type) (kube_pod_container_resource_requests{cluster="prod",resource="cpu"`},
	{"workload memory requests", "/api/v1/query_range", `sum by (namespace, workload, workload_type) (kube_pod_container_resource_requests{cluster="prod",resource="memory"`},
	{"workload cpu limits", "/api/v1/query_range", `sum by (namespace, workload, workload_type) (kube_pod_container_resource_limits{cluster="prod",resource="cpu"`},
	{"workload memory limits", "/api/v1/query_range", `sum by (namespace, workload, workload_type) (kube_pod_container_resource_limits{cluster="prod",resource="memory"`},
}

// TestEdgeScenarioLeavesNoAnswerUnused runs edge-values through the whole
// chain and checks that each of the fourteen answers it replaces was asked
// exactly once. promtest.Server tells how many scripted requests remain
// unconsumed only as a total count, and a scenario's own capability probes
// leave several of its other, never-asked variants above zero on every
// scenario; so this checks the fourteen themselves, by path and by a
// fragment of the query text real enough to identify each one, against what
// the server actually saw.
func TestEdgeScenarioLeavesNoAnswerUnused(t *testing.T) {
	scenario := scenarioByName(t, "edge-values")

	_, fake, err := run(context.Background(), scenario)
	if err != nil {
		t.Fatalf("run(%s): %v", scenario.Name, err)
	}
	seen := fake.Seen()

	for _, want := range edgeAnswers {
		count := 0
		for _, s := range seen {
			if s.Matched && s.Path == want.path && strings.Contains(s.Query, want.fragment) {
				count++
			}
		}
		if count != 1 {
			t.Errorf("%s: matched %d times among the seen requests, want 1", want.name, count)
		}
	}
}
