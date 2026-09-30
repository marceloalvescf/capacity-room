package collect

import (
	"context"
	"reflect"
	"testing"

	"github.com/marceloalvescf/k8s-capacity-report/internal/discovery"
	"github.com/marceloalvescf/k8s-capacity-report/internal/prom"
)

const nodeScope = `{cluster="prod"}`

const (
	nodeInfoQuery        = "kube_node_info" + nodeScope
	nodeUnameQuery       = "node_uname_info" + nodeScope
	nodeLabelsQuery      = "kube_node_labels" + nodeScope
	nodeCapCPUQuery      = `kube_node_status_capacity{cluster="prod",resource="cpu"}`
	nodeCapMemoryQuery   = `kube_node_status_capacity{cluster="prod",resource="memory"}`
	nodeAllocCPUQuery    = `kube_node_status_allocatable{cluster="prod",resource="cpu"}`
	nodeAllocMemoryQuery = `kube_node_status_allocatable{cluster="prod",resource="memory"}`
	nodeReadyQuery       = `kube_node_status_condition{cluster="prod",condition="Ready"}`
	nodeUnschedQuery     = "kube_node_spec_unschedulable" + nodeScope
	nodeReqCPUQuery      = `sum by (node, instance) (kube_pod_container_resource_requests{cluster="prod",resource="cpu"})`
	nodeReqMemoryQuery   = `sum by (node, instance) (kube_pod_container_resource_requests{cluster="prod",resource="memory"})`
	nodeUsageCPUQuery    = "instance:node_cpu_utilisation:rate5m" + nodeScope + " * 100"
	nodeUsageMemoryQuery = "instance:node_memory_utilisation:ratio" + nodeScope + " * 100"
)

func nodeCapability(name, state, strategy string, evidence, reasons []string) discovery.Capability {
	if evidence == nil {
		evidence = []string{}
	}
	if reasons == nil {
		reasons = []string{}
	}
	return discovery.Capability{Name: name, State: state, Strategy: strategy, Evidence: evidence, Reasons: reasons}
}

func nodeInput() Input {
	requests := testCapability(discovery.StateAvailable, nil)
	in := testInput(testScope(true), defaultCluster(), defaultAnalysis(), requests)
	in.Capabilities = []discovery.Capability{
		nodeCapability("node_utilization", discovery.StateAvailable, discovery.StrategyRecordingRule, nil, nil),
		nodeCapability("nodepool_discovery", discovery.StateAvailable, discovery.StrategyRawMetrics, nil, nil),
		requests,
	}
	return in
}

func withCapability(in Input, capability discovery.Capability) Input {
	capabilities := append([]discovery.Capability{}, in.Capabilities...)
	for i := range capabilities {
		if capabilities[i].Name == capability.Name {
			capabilities[i] = capability
		}
	}
	in.Capabilities = capabilities
	return in
}

func nodeSample(value float64, labels map[string]string) prom.Sample {
	return prom.Sample{Labels: labels, Value: value, Valid: true}
}

func byNode(name string) map[string]string { return map[string]string{"node": name} }

func usageSeries(labels map[string]string, values ...float64) prom.Series {
	points := make([]prom.Point, len(values))
	for i, value := range values {
		points[i] = prom.Point{Time: float64(i + 1), Value: value, Valid: true}
	}
	return prom.Series{Labels: labels, Points: points}
}

// completeNodeAnswers answers the thirteen queries for one node, node-1.
func completeNodeAnswers() (map[string][]prom.Sample, map[string][]prom.Series) {
	instant := map[string][]prom.Sample{
		nodeInfoQuery:        {nodeSample(1, map[string]string{"node": "node-1", "internal_ip": "10.0.0.1", "hostname": "node-1", "provider_id": "p://1"})},
		nodeUnameQuery:       {nodeSample(1, map[string]string{"node": "node-1", "instance": "10.0.0.1:9100", "nodename": "node-1"})},
		nodeLabelsQuery:      {nodeSample(1, map[string]string{"node": "node-1", "label_agentpool": "pool-a"})},
		nodeCapCPUQuery:      {nodeSample(4, byNode("node-1"))},
		nodeCapMemoryQuery:   {nodeSample(8, byNode("node-1"))},
		nodeAllocCPUQuery:    {nodeSample(4, byNode("node-1"))},
		nodeAllocMemoryQuery: {nodeSample(8, byNode("node-1"))},
		nodeReadyQuery:       {nodeSample(1, map[string]string{"node": "node-1", "status": "true"})},
		nodeUnschedQuery:     {nodeSample(0, byNode("node-1"))},
		nodeReqCPUQuery:      {nodeSample(1, map[string]string{"node": "node-1", "instance": "10.0.0.1:9100"})},
		nodeReqMemoryQuery:   {nodeSample(2, map[string]string{"node": "node-1", "instance": "10.0.0.1:9100"})},
	}
	ranges := map[string][]prom.Series{
		nodeUsageCPUQuery:    {usageSeries(map[string]string{"instance": "10.0.0.1:9100", "node": "node-1"}, 55, 60)},
		nodeUsageMemoryQuery: {usageSeries(map[string]string{"instance": "10.0.0.1:9100", "node": "node-1"}, 60, 65)},
	}
	return instant, ranges
}

func nodeQuerier(instant map[string][]prom.Sample, ranges map[string][]prom.Series) *scriptedQuerier {
	return &scriptedQuerier{
		queryFunc: func(query string) ([]prom.Sample, error) { return instant[query], nil },
		rangeFunc: func(query string) ([]prom.Series, error) { return ranges[query], nil },
	}
}

func collectNodes(t *testing.T, in Input, instant map[string][]prom.Sample, ranges map[string][]prom.Series) (Result[Node], *scriptedQuerier) {
	t.Helper()
	q := nodeQuerier(instant, ranges)
	result, err := Nodes(context.Background(), q, in)
	if err != nil {
		t.Fatalf("Nodes: %v", err)
	}
	return result, q
}

func rowNamed(t *testing.T, result Result[Node], name string) Node {
	t.Helper()
	for _, row := range result.Rows {
		if row.Name == name {
			return row
		}
	}
	t.Fatalf("no row %q in %+v", name, result.Rows)
	return Node{}
}

func warningCodes(warnings []Message) []string {
	codes := []string{}
	for _, warning := range warnings {
		codes = append(codes, warning.Code)
	}
	return codes
}

func TestNodeQueriesInOrder(t *testing.T) {
	instant, ranges := completeNodeAnswers()
	_, q := collectNodes(t, nodeInput(), instant, ranges)
	want := []string{
		`kube_node_info{cluster="prod"}`,
		`node_uname_info{cluster="prod"}`,
		`kube_node_labels{cluster="prod"}`,
		`kube_node_status_capacity{cluster="prod",resource="cpu"}`,
		`kube_node_status_capacity{cluster="prod",resource="memory"}`,
		`kube_node_status_allocatable{cluster="prod",resource="cpu"}`,
		`kube_node_status_allocatable{cluster="prod",resource="memory"}`,
		`kube_node_status_condition{cluster="prod",condition="Ready"}`,
		`kube_node_spec_unschedulable{cluster="prod"}`,
		`sum by (node, instance) (kube_pod_container_resource_requests{cluster="prod",resource="cpu"})`,
		`sum by (node, instance) (kube_pod_container_resource_requests{cluster="prod",resource="memory"})`,
		`instance:node_cpu_utilisation:rate5m{cluster="prod"} * 100`,
		`instance:node_memory_utilisation:ratio{cluster="prod"} * 100`,
	}
	if !reflect.DeepEqual(q.queries, want) {
		t.Errorf("queries =\n%q\nwant\n%q", q.queries, want)
	}
}

func TestNodeQueriesRaw(t *testing.T) {
	in := withCapability(nodeInput(), nodeCapability("node_utilization", discovery.StateAvailable, discovery.StrategyRawMetrics, nil, nil))
	in.Window.Step = "5m"
	_, q := collectNodes(t, in, nil, nil)
	want := []string{
		`100 - avg by(instance) (rate(node_cpu_seconds_total{cluster="prod",mode="idle"}[5m])) * 100`,
		`(1 - node_memory_MemAvailable_bytes{cluster="prod"} / node_memory_MemTotal_bytes{cluster="prod"}) * 100`,
	}
	if !reflect.DeepEqual(q.queries[11:], want) {
		t.Errorf("range queries = %q, want %q", q.queries[11:], want)
	}
	if len(q.queries) != 13 {
		t.Errorf("len(queries) = %d, want 13", len(q.queries))
	}
}

func TestNodeQueriesWithoutScopeMatcher(t *testing.T) {
	in := nodeInput()
	in.Scope = testScope(false)
	_, q := collectNodes(t, in, nil, nil)
	if q.queries[0] != "kube_node_info" || q.queries[3] != `kube_node_status_capacity{resource="cpu"}` {
		t.Errorf("queries = %q", q.queries)
	}
}

func TestNodeQueriesLegacyRequestNames(t *testing.T) {
	in := withCapability(nodeInput(), testCapability(discovery.StateAvailable, []string{"kube_pod_container_resource_requests_memory_bytes"}))
	_, q := collectNodes(t, in, nil, nil)
	want := `sum by (node, instance) (kube_pod_container_resource_requests_memory_bytes{cluster="prod"})`
	if len(q.queries) != 12 || q.queries[9] != want {
		t.Errorf("queries = %q, want the one legacy memory request query at index 9", q.queries)
	}
}

func TestNodeQueriesSkipWhatIsUnavailable(t *testing.T) {
	in := nodeInput()
	in = withCapability(in, nodeCapability("nodepool_discovery", discovery.StateUnavailable, discovery.StrategyNone, nil, []string{"NODEPOOL_LABELS_NOT_FOUND"}))
	in = withCapability(in, testCapability(discovery.StateUnavailable, nil))
	in = withCapability(in, nodeCapability("node_utilization", discovery.StateUnavailable, discovery.StrategyNone, nil, nil))
	_, q := collectNodes(t, in, nil, nil)
	want := []string{
		nodeInfoQuery, nodeUnameQuery,
		nodeCapCPUQuery, nodeCapMemoryQuery, nodeAllocCPUQuery, nodeAllocMemoryQuery,
		nodeReadyQuery, nodeUnschedQuery,
	}
	if !reflect.DeepEqual(q.queries, want) {
		t.Errorf("queries =\n%q\nwant\n%q", q.queries, want)
	}
}

func TestNodeRowFromTheCompleteScenario(t *testing.T) {
	instant, ranges := completeNodeAnswers()
	result, _ := collectNodes(t, nodeInput(), instant, ranges)
	if result.State != discovery.StateAvailable || result.Strategy != discovery.StrategyRecordingRule {
		t.Errorf("state, strategy = %q, %q", result.State, result.Strategy)
	}
	if len(result.Warnings) != 0 || len(result.Notices) != 0 {
		t.Errorf("warnings, notices = %v, %v, want none", result.Warnings, result.Notices)
	}
	want := Node{
		Name: "node-1", Pool: "pool-a", PoolConfidence: "exact",
		CPUCapacity: Number{4, true}, MemoryCapacity: Number{8, true},
		CPUAllocatable: Number{4, true}, MemoryAllocatable: Number{8, true},
		CPUUsagePercent:      Statistics{State: StateValue, Average: 57.5, Minimum: 55, Peak: 60, Current: 60},
		MemoryUsagePercent:   Statistics{State: StateValue, Average: 62.5, Minimum: 60, Peak: 65, Current: 65},
		CPURequestPercent:    Number{25, true},
		MemoryRequestPercent: Number{25, true},
		Ready:                Flag{true, true},
		Schedulable:          Flag{true, true},
	}
	if len(result.Rows) != 1 || result.Rows[0] != want {
		t.Errorf("rows = %+v, want [%+v]", result.Rows, want)
	}
}

func TestNodeResolvesByInstance(t *testing.T) {
	instant, ranges := completeNodeAnswers()
	ranges[nodeUsageCPUQuery] = []prom.Series{usageSeries(map[string]string{"instance": "10.0.0.1:9100"}, 30, 50)}
	result, _ := collectNodes(t, nodeInput(), instant, ranges)
	got := rowNamed(t, result, "node-1").CPUUsagePercent
	want := Statistics{State: StateValue, Average: 40, Minimum: 30, Peak: 50, Current: 50}
	if got != want {
		t.Errorf("CPUUsagePercent = %+v, want %+v", got, want)
	}
}

func TestNodeDropsAnUnresolvedSeries(t *testing.T) {
	instant, ranges := completeNodeAnswers()
	instant[nodeInfoQuery] = []prom.Sample{
		nodeSample(1, map[string]string{"node": "node-1", "internal_ip": "10.0.0.1"}),
		nodeSample(1, map[string]string{"node": "node-2", "internal_ip": "10.0.0.1"}),
	}
	instant[nodeUnameQuery] = nil
	ranges[nodeUsageCPUQuery] = []prom.Series{usageSeries(map[string]string{"instance": "10.0.0.1:9100"}, 30, 50)}
	ranges[nodeUsageMemoryQuery] = nil
	result, _ := collectNodes(t, nodeInput(), instant, ranges)
	if len(result.Rows) != 2 {
		t.Fatalf("rows = %+v, want node-1 and node-2", result.Rows)
	}
	for _, row := range result.Rows {
		if row.CPUUsagePercent.State != StateUnavailable {
			t.Errorf("%s CPUUsagePercent = %+v, want unavailable", row.Name, row.CPUUsagePercent)
		}
	}
}

func TestNodeKnownByRequestsOnly(t *testing.T) {
	instant, ranges := completeNodeAnswers()
	instant[nodeReqCPUQuery] = append(instant[nodeReqCPUQuery], nodeSample(1, byNode("orphan")))
	result, _ := collectNodes(t, nodeInput(), instant, ranges)
	orphan := rowNamed(t, result, "orphan")
	if orphan.Pool != "unknown" || orphan.PoolConfidence != "unknown" || orphan.CPUCapacity.Known {
		t.Errorf("orphan = %+v", orphan)
	}
	if result.State != discovery.StatePartial {
		t.Errorf("State = %q, want partial", result.State)
	}
	if got := warningCodes(result.Warnings); !reflect.DeepEqual(got, []string{WarningNodeCapacityEvidenceIncomplete}) {
		t.Errorf("warnings = %v", got)
	}
}

func TestNodeWarningsHoldTheReasons(t *testing.T) {
	instant, ranges := completeNodeAnswers()
	in := withCapability(nodeInput(), nodeCapability("nodepool_discovery", discovery.StatePartial, discovery.StrategyRawMetrics, nil, []string{"NODEPOOL_LABELS_NOT_FOUND"}))
	in = withCapability(in, nodeCapability("requests_limits", discovery.StatePartial, discovery.StrategyRawMetrics, []string{"kube_pod_container_resource_requests"}, []string{"NODEPOOL_LABELS_NOT_FOUND"}))
	result, _ := collectNodes(t, in, instant, ranges)
	if result.State != discovery.StatePartial {
		t.Errorf("State = %q, want partial", result.State)
	}
	if got := warningCodes(result.Warnings); !reflect.DeepEqual(got, []string{"NODEPOOL_LABELS_NOT_FOUND"}) {
		t.Errorf("warnings = %v, want the reason once", got)
	}
}

func TestNodeNoIdentity(t *testing.T) {
	result, _ := collectNodes(t, nodeInput(), nil, nil)
	if result.State != discovery.StateUnavailable || result.Strategy != discovery.StrategyRecordingRule {
		t.Errorf("state, strategy = %q, %q", result.State, result.Strategy)
	}
	if got := warningCodes(result.Warnings); !reflect.DeepEqual(got, []string{WarningNodeIdentityUnavailable}) {
		t.Errorf("warnings = %v", got)
	}
	if result.Rows == nil || len(result.Rows) != 0 || result.Notices == nil || len(result.Notices) != 0 {
		t.Errorf("rows, notices = %v, %v, want empty and not nil", result.Rows, result.Notices)
	}
}

func TestNodeReadyStates(t *testing.T) {
	instant, ranges := completeNodeAnswers()
	status := func(node, state string, value float64) prom.Sample {
		return nodeSample(value, map[string]string{"node": node, "status": state})
	}
	instant[nodeReadyQuery] = []prom.Sample{
		status("a", "true", 1), status("a", "false", 0), status("a", "unknown", 0),
		status("b", "false", 1), status("b", "true", 0),
		status("c", "unknown", 1),
		status("d", "true", 0),
		status("e", "true", 1), status("e", "false", 1),
	}
	result, _ := collectNodes(t, nodeInput(), instant, ranges)
	want := map[string]Flag{"a": {true, true}, "b": {false, true}, "c": {}, "d": {}, "e": {}}
	for name, flag := range want {
		if got := rowNamed(t, result, name).Ready; got != flag {
			t.Errorf("%s Ready = %+v, want %+v", name, got, flag)
		}
	}
}

func TestNodeSchedulable(t *testing.T) {
	instant, ranges := completeNodeAnswers()
	instant[nodeUnschedQuery] = []prom.Sample{nodeSample(0, byNode("a")), nodeSample(1, byNode("b"))}
	instant[nodeCapCPUQuery] = append(instant[nodeCapCPUQuery], nodeSample(2, byNode("c")))
	result, _ := collectNodes(t, nodeInput(), instant, ranges)
	want := map[string]Flag{"a": {true, true}, "b": {false, true}, "c": {}}
	for name, flag := range want {
		if got := rowNamed(t, result, name).Schedulable; got != flag {
			t.Errorf("%s Schedulable = %+v, want %+v", name, got, flag)
		}
	}
}

func TestNodeRequestPercent(t *testing.T) {
	instant, ranges := completeNodeAnswers()
	instant[nodeReqCPUQuery] = []prom.Sample{nodeSample(3.6, byNode("node-1"))}
	instant[nodeAllocMemoryQuery] = []prom.Sample{nodeSample(0, byNode("node-1"))}
	result, _ := collectNodes(t, nodeInput(), instant, ranges)
	row := rowNamed(t, result, "node-1")
	if row.CPURequestPercent != (Number{90, true}) {
		t.Errorf("CPURequestPercent = %+v, want 90", row.CPURequestPercent)
	}
	if row.MemoryRequestPercent.Known {
		t.Errorf("MemoryRequestPercent = %+v, want unknown", row.MemoryRequestPercent)
	}
	if result.State != discovery.StateAvailable {
		t.Errorf("State = %q: an allocatable of zero needs no request percent", result.State)
	}
}

func TestNodeRequestsTakeTheMaxOfTheSeriesOfANode(t *testing.T) {
	instant, ranges := completeNodeAnswers()
	instant[nodeReqCPUQuery] = []prom.Sample{
		nodeSample(1, map[string]string{"node": "node-1", "instance": "a"}),
		nodeSample(2, map[string]string{"node": "node-1", "instance": "b"}),
	}
	result, _ := collectNodes(t, nodeInput(), instant, ranges)
	if got := rowNamed(t, result, "node-1").CPURequestPercent; got != (Number{50, true}) {
		t.Errorf("CPURequestPercent = %+v, want 50", got)
	}
}

func TestNodePoolFromTheName(t *testing.T) {
	instant, ranges := completeNodeAnswers()
	name := "aks-agentpool-12345678-vmss000000"
	instant[nodeInfoQuery] = []prom.Sample{nodeSample(1, byNode(name))}
	instant[nodeLabelsQuery] = nil
	result, _ := collectNodes(t, nodeInput(), instant, ranges)
	row := rowNamed(t, result, name)
	if row.Pool != "agentpool" || row.PoolConfidence != "inferred" {
		t.Errorf("pool = %q, %q", row.Pool, row.PoolConfidence)
	}
}

func TestNodePoolFromTheConfiguredLabel(t *testing.T) {
	instant, ranges := completeNodeAnswers()
	instant[nodeLabelsQuery] = []prom.Sample{
		nodeSample(1, map[string]string{"node": "node-1", "label_team": "blue", "label_agentpool": "pool-a"}),
		nodeSample(1, map[string]string{"node": "node-1", "label_team": "blue", "label_agentpool": "pool-b"}),
	}
	in := nodeInput()
	in.Cluster.NodepoolLabel = "label_team"
	result, _ := collectNodes(t, in, instant, ranges)
	row := rowNamed(t, result, "node-1")
	if row.Pool != "blue" || row.PoolConfidence != "exact" {
		t.Errorf("pool = %q, %q, want blue exact", row.Pool, row.PoolConfidence)
	}
}

func TestNodeLabelWithTwoValuesIsDropped(t *testing.T) {
	instant, ranges := completeNodeAnswers()
	instant[nodeLabelsQuery] = []prom.Sample{
		nodeSample(1, map[string]string{"node": "node-1", "label_agentpool": "pool-a"}),
		nodeSample(1, map[string]string{"node": "node-1", "label_agentpool": "pool-b"}),
	}
	result, _ := collectNodes(t, nodeInput(), instant, ranges)
	if got := rowNamed(t, result, "node-1").PoolConfidence; got != "unknown" {
		t.Errorf("PoolConfidence = %q, want unknown", got)
	}
	if result.State != discovery.StatePartial {
		t.Errorf("State = %q, want partial", result.State)
	}
}

func TestNodeSharedNameLabel(t *testing.T) {
	instant, ranges := completeNodeAnswers()
	instant[nodeInfoQuery] = []prom.Sample{nodeSample(1, byNode("x")), nodeSample(1, byNode("y"))}
	instant[nodeLabelsQuery] = []prom.Sample{
		nodeSample(1, map[string]string{"node": "x", "label_name": "batch"}),
		nodeSample(1, map[string]string{"node": "y", "label_name": "batch"}),
	}
	result, _ := collectNodes(t, nodeInput(), instant, ranges)
	if row := rowNamed(t, result, "x"); row.Pool != "batch" || row.PoolConfidence != "inferred" {
		t.Errorf("x pool = %q, %q", row.Pool, row.PoolConfidence)
	}
}

func TestNodeHostnameLowered(t *testing.T) {
	instant, ranges := completeNodeAnswers()
	instant[nodeInfoQuery] = []prom.Sample{nodeSample(1, map[string]string{"node": "worker-a", "internal_ip": "10.0.0.9"})}
	instant[nodeUnameQuery] = []prom.Sample{nodeSample(1, map[string]string{"node": "worker-a", "nodename": "Node-1"})}
	ranges[nodeUsageCPUQuery] = []prom.Series{usageSeries(map[string]string{"instance": "node-1:9100"}, 30, 50)}
	result, _ := collectNodes(t, nodeInput(), instant, ranges)
	if got := rowNamed(t, result, "worker-a").CPUUsagePercent.Peak; got != 50 {
		t.Errorf("Peak = %v, want 50: the lower-cased hostname resolves the series", got)
	}
}

func TestNodeHostnameNeedsOneCandidate(t *testing.T) {
	instant, ranges := completeNodeAnswers()
	instant[nodeInfoQuery] = []prom.Sample{nodeSample(1, map[string]string{"node": "worker-a", "internal_ip": "10.0.0.9"})}
	instant[nodeUnameQuery] = []prom.Sample{
		nodeSample(1, map[string]string{"node": "worker-a", "nodename": "one"}),
		nodeSample(1, map[string]string{"node": "worker-a", "nodename": "two"}),
	}
	ranges[nodeUsageCPUQuery] = []prom.Series{usageSeries(map[string]string{"instance": "one:9100"}, 30, 50)}
	result, _ := collectNodes(t, nodeInput(), instant, ranges)
	if got := rowNamed(t, result, "worker-a").CPUUsagePercent.State; got != StateUnavailable {
		t.Errorf("State = %q, want unavailable: two candidates give no hostname", got)
	}
}

func TestNodeMaxOfSeriesOfOneNode(t *testing.T) {
	instant, ranges := completeNodeAnswers()
	ranges[nodeUsageCPUQuery] = []prom.Series{
		usageSeries(map[string]string{"instance": "10.0.0.1:9100", "node": "node-1"}, 10, 40),
		usageSeries(map[string]string{"instance": "10.0.0.1:9101"}, 20, 30),
	}
	result, _ := collectNodes(t, nodeInput(), instant, ranges)
	want := Statistics{State: StateValue, Average: 30, Minimum: 20, Peak: 40, Current: 40}
	if got := rowNamed(t, result, "node-1").CPUUsagePercent; got != want {
		t.Errorf("CPUUsagePercent = %+v, want %+v", got, want)
	}
}

func TestNodeRowsSortedAndIdentityAttributesWithTwoValuesAreEmpty(t *testing.T) {
	instant, ranges := completeNodeAnswers()
	instant[nodeInfoQuery] = []prom.Sample{
		nodeSample(1, map[string]string{"node": "b", "internal_ip": "10.0.0.2"}),
		nodeSample(1, map[string]string{"node": "a", "internal_ip": "10.0.0.1"}),
		nodeSample(1, map[string]string{"node": "a", "internal_ip": "10.0.0.3"}),
	}
	ranges[nodeUsageCPUQuery] = []prom.Series{usageSeries(map[string]string{"instance": "10.0.0.1:9100"}, 30, 50)}
	result, _ := collectNodes(t, nodeInput(), instant, ranges)
	names := []string{}
	for _, row := range result.Rows {
		names = append(names, row.Name)
	}
	if !reflect.DeepEqual(names, []string{"a", "b", "node-1"}) {
		t.Errorf("names = %v", names)
	}
	if got := rowNamed(t, result, "a").CPUUsagePercent.State; got != StateUnavailable {
		t.Errorf("a has two IPs, so none: State = %q", got)
	}
}

func TestPools(t *testing.T) {
	nodes := []Node{
		{Name: "n4", Pool: "unknown", PoolConfidence: "unknown"},
		{Name: "n2", Pool: "pool-b", PoolConfidence: "exact"},
		{Name: "n3", Pool: "pool-a", PoolConfidence: "inferred"},
		{Name: "n1", Pool: "pool-a", PoolConfidence: "exact"},
		{Name: "n0", Pool: "pool-b", PoolConfidence: "exact"},
	}
	want := []Pool{
		{Name: "pool-a", Confidence: "inferred", Nodes: []string{"n1", "n3"}},
		{Name: "pool-b", Confidence: "exact", Nodes: []string{"n0", "n2"}},
		{Name: "unknown", Confidence: "unknown", Nodes: []string{"n4"}},
	}
	if got := Pools(nodes); !reflect.DeepEqual(got, want) {
		t.Errorf("Pools = %+v, want %+v", got, want)
	}
	if got := Pools(nil); got == nil || len(got) != 0 {
		t.Errorf("Pools(nil) = %#v, want empty and not nil", got)
	}
}
