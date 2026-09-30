package collect

import (
	"context"
	"sort"

	"github.com/marceloalvescf/k8s-capacity-report/internal/discovery"
	"github.com/marceloalvescf/k8s-capacity-report/internal/prom"
	"github.com/marceloalvescf/k8s-capacity-report/internal/resolve"
)

// The metric names the node collector queries, following
// metrics/names.py of the original.
const (
	kubeNodeInfo              = "kube_node_info"
	nodeUnameInfo             = "node_uname_info"
	kubeNodeLabels            = "kube_node_labels"
	kubeNodeStatusCapacity    = "kube_node_status_capacity"
	kubeNodeStatusAllocatable = "kube_node_status_allocatable"
	kubeNodeStatusCondition   = "kube_node_status_condition"
	kubeNodeSpecUnschedulable = "kube_node_spec_unschedulable"
	nodeCPURule               = "instance:node_cpu_utilisation:rate5m"
	nodeMemoryRule            = "instance:node_memory_utilisation:ratio"
	nodeCPUSeconds            = "node_cpu_seconds_total"
	nodeMemAvailable          = "node_memory_MemAvailable_bytes"
	nodeMemTotal              = "node_memory_MemTotal_bytes"
)

// The names Input.Capabilities carries the two node capabilities under; the
// third is requestsLimitsCapability.
const (
	nodeUtilizationCapability = "node_utilization"
	nodepoolCapability        = "nodepool_discovery"
)

// Node is one node of the scope.
type Node struct {
	Name                 string
	Pool                 string // "unknown" when undetermined
	PoolConfidence       string // exact, inferred, unknown
	CPUCapacity          Number
	MemoryCapacity       Number
	CPUAllocatable       Number
	MemoryAllocatable    Number
	CPUUsagePercent      Statistics
	MemoryUsagePercent   Statistics
	CPURequestPercent    Number // one ratio, requested over allocatable
	MemoryRequestPercent Number
	Ready                Flag
	Schedulable          Flag
}

// Pool is the nodes that share a pool name.
type Pool struct {
	Name       string
	Confidence string   // the lowest of its nodes: unknown < inferred < exact
	Nodes      []string // names, sorted
}

// nodeAnswers holds what the queries of the node collector returned.
type nodeAnswers struct {
	info, uname, labels               []prom.Sample
	capacityCPU, capacityMemory       []prom.Sample
	allocatableCPU, allocatableMemory []prom.Sample
	ready, unschedulable              []prom.Sample
	requestsCPU, requestsMemory       []prom.Sample
	usageCPU, usageMemory             []prom.Series
}

// Nodes follows NodeCollector.collect of collectors/nodes.py: the queries in
// its order, the identities of the nodes, and one row per node.
func Nodes(ctx context.Context, q Querier, in Input) (Result[Node], error) {
	utilization, err := findCapability(in.Capabilities, nodeUtilizationCapability)
	if err != nil {
		return Result[Node]{}, err
	}
	pools, err := findCapability(in.Capabilities, nodepoolCapability)
	if err != nil {
		return Result[Node]{}, err
	}
	requests, err := findCapability(in.Capabilities, requestsLimitsCapability)
	if err != nil {
		return Result[Node]{}, err
	}
	answers, err := askNodes(ctx, q, in, utilization, pools, requests)
	if err != nil {
		return Result[Node]{}, err
	}

	at := float64(in.Window.End.Unix())
	identities, outside := nodeIdentities(answers)
	if len(identities) == 0 {
		return Result[Node]{
			State:    discovery.StateUnavailable,
			Strategy: utilization.Strategy,
			Rows:     []Node{},
			Warnings: []Message{{Code: WarningNodeIdentityUnavailable, Params: map[string]float64{}}},
			Notices:  []Message{},
		}, nil
	}
	identities = enrichHostnames(identities, answers.uname)
	index := resolve.NewNodeIndex(identities)

	labelsByNode := nodeLabels(answers.labels)
	shared := resolve.SharedNameLabels(labelsByNode)
	byNode := []string{"node"}
	capacityCPU := ReduceMax(instantSeries(withNode(answers.capacityCPU), at), byNode)
	capacityMemory := ReduceMax(instantSeries(withNode(answers.capacityMemory), at), byNode)
	allocatableCPU := ReduceMax(instantSeries(withNode(answers.allocatableCPU), at), byNode)
	allocatableMemory := ReduceMax(instantSeries(withNode(answers.allocatableMemory), at), byNode)
	requestedCPU := ReduceMax(instantSeries(withNode(answers.requestsCPU), at), byNode)
	requestedMemory := ReduceMax(instantSeries(withNode(answers.requestsMemory), at), byNode)
	unschedulable := ReduceMax(instantSeries(withNode(answers.unschedulable), at), byNode)
	usageCPU := resolvedStatistics(answers.usageCPU, index)
	usageMemory := resolvedStatistics(answers.usageMemory, index)
	ready := readyByNode(answers.ready, at)

	incomplete := outside
	for _, capability := range []discovery.Capability{utilization, pools, requests} {
		if capability.State != discovery.StateAvailable {
			incomplete = true
		}
	}

	rows := make([]Node, 0, len(identities))
	for _, identity := range identities {
		name := identity.Name
		key := Key{name}
		pool, _, confidence := resolve.NodePool(name, labelsByNode[name], in.Cluster.NodepoolLabel, shared)
		row := Node{
			Name:               name,
			Pool:               pool,
			PoolConfidence:     confidence,
			CPUCapacity:        currentOf(capacityCPU, key),
			MemoryCapacity:     currentOf(capacityMemory, key),
			CPUAllocatable:     currentOf(allocatableCPU, key),
			MemoryAllocatable:  currentOf(allocatableMemory, key),
			CPUUsagePercent:    Lookup(usageCPU, key),
			MemoryUsagePercent: Lookup(usageMemory, key),
			Ready:              ready[name],
		}
		row.CPURequestPercent = Percent(currentOf(requestedCPU, key), row.CPUAllocatable)
		row.MemoryRequestPercent = Percent(currentOf(requestedMemory, key), row.MemoryAllocatable)
		if value := currentOf(unschedulable, key); value.Known {
			row.Schedulable = Flag{Value: value.Value == 0, Known: true}
		}
		rows = append(rows, row)

		if confidence == resolve.ConfidenceUnknown ||
			!row.CPUCapacity.Known || !row.MemoryCapacity.Known ||
			!row.CPUAllocatable.Known || !row.MemoryAllocatable.Known ||
			row.CPUUsagePercent.State != StateValue || row.MemoryUsagePercent.State != StateValue ||
			(row.CPUAllocatable.Value > 0 && !row.CPURequestPercent.Known) ||
			(row.MemoryAllocatable.Value > 0 && !row.MemoryRequestPercent.Known) {
			incomplete = true
		}
	}

	warnings := dedupMessages(reasonWarnings(utilization), reasonWarnings(pools), reasonWarnings(requests))
	if incomplete && len(warnings) == 0 {
		warnings = []Message{{Code: WarningNodeCapacityEvidenceIncomplete, Params: map[string]float64{}}}
	}
	state := discovery.StateAvailable
	if incomplete {
		state = discovery.StatePartial
	}
	return Result[Node]{
		State:    state,
		Strategy: utilization.Strategy,
		Rows:     rows,
		Warnings: warnings,
		Notices:  []Message{},
	}, nil
}

// nodeQuery is the selector of a node metric: the scope matchers and the
// extra matchers, merged and sorted by label name.
func nodeQuery(metric string, in Input, extra map[string]string) string {
	equal := make(map[string]string)
	for name, value := range in.Scope.Matchers() {
		equal[name] = value
	}
	for name, value := range extra {
		equal[name] = value
	}
	return prom.Selector{Metric: metric, Equal: equal}.String()
}

// askNodes sends the queries of the node collector in the order of the
// original and returns the answers. A query of an unavailable capability is
// not sent.
func askNodes(ctx context.Context, q Querier, in Input, utilization, pools, requests discovery.Capability) (nodeAnswers, error) {
	var a nodeAnswers
	instant := func(query string, into *[]prom.Sample) error {
		samples, err := q.Query(ctx, query, in.Window.End)
		*into = samples
		return err
	}
	cpu, memory := map[string]string{"resource": "cpu"}, map[string]string{"resource": "memory"}
	steps := []struct {
		skip   bool
		query  string
		target *[]prom.Sample
	}{
		{false, nodeQuery(kubeNodeInfo, in, nil), &a.info},
		{false, nodeQuery(nodeUnameInfo, in, nil), &a.uname},
		{pools.State == discovery.StateUnavailable, nodeQuery(kubeNodeLabels, in, nil), &a.labels},
		{false, nodeQuery(kubeNodeStatusCapacity, in, cpu), &a.capacityCPU},
		{false, nodeQuery(kubeNodeStatusCapacity, in, memory), &a.capacityMemory},
		{false, nodeQuery(kubeNodeStatusAllocatable, in, cpu), &a.allocatableCPU},
		{false, nodeQuery(kubeNodeStatusAllocatable, in, memory), &a.allocatableMemory},
		{false, nodeQuery(kubeNodeStatusCondition, in, map[string]string{"condition": "Ready"}), &a.ready},
		{false, nodeQuery(kubeNodeSpecUnschedulable, in, nil), &a.unschedulable},
	}
	for _, step := range steps {
		if step.skip {
			continue
		}
		if err := instant(step.query, step.target); err != nil {
			return a, err
		}
	}

	if requests.State != discovery.StateUnavailable {
		unified := holdsName(requests.Evidence, "kube_pod_container_resource_requests")
		for _, request := range []struct {
			kind, legacyName string
			target           *[]prom.Sample
		}{
			{"cpu", "kube_pod_container_resource_requests_cpu_cores", &a.requestsCPU},
			{"memory", "kube_pod_container_resource_requests_memory_bytes", &a.requestsMemory},
		} {
			metric, extra := "kube_pod_container_resource_requests", map[string]string{"resource": request.kind}
			if !unified {
				if !holdsName(requests.Evidence, request.legacyName) {
					continue
				}
				metric, extra = request.legacyName, nil
			}
			if err := instant("sum by (node, instance) ("+nodeQuery(metric, in, extra)+")", request.target); err != nil {
				return a, err
			}
		}
	}

	if utilization.State != discovery.StateUnavailable {
		var cpuQuery, memoryQuery string
		if utilization.Strategy == discovery.StrategyRecordingRule {
			cpuQuery = nodeQuery(nodeCPURule, in, nil) + " * 100"
			memoryQuery = nodeQuery(nodeMemoryRule, in, nil) + " * 100"
		} else {
			cpuQuery = "100 - avg by(instance) (rate(" + nodeQuery(nodeCPUSeconds, in, map[string]string{"mode": "idle"}) + "[" + in.Window.Step + "])) * 100"
			memoryQuery = "(1 - " + nodeQuery(nodeMemAvailable, in, nil) + " / " + nodeQuery(nodeMemTotal, in, nil) + ") * 100"
		}
		var err error
		if a.usageCPU, err = q.QueryRange(ctx, cpuQuery, in.Window.Start, in.Window.End, in.Window.Step); err != nil {
			return a, err
		}
		if a.usageMemory, err = q.QueryRange(ctx, memoryQuery, in.Window.Start, in.Window.End, in.Window.Step); err != nil {
			return a, err
		}
	}
	return a, nil
}

func holdsName(names []string, name string) bool {
	for _, candidate := range names {
		if candidate == name {
			return true
		}
	}
	return false
}

// finiteSample reports whether a sample is valid and finite.
func finiteSample(sample prom.Sample) bool { return sample.Valid && isFinite(sample.Value) }

// withNode keeps the samples that carry a node label with a value.
func withNode(samples []prom.Sample) []prom.Sample {
	kept := []prom.Sample{}
	for _, sample := range samples {
		if sample.Labels["node"] != "" {
			kept = append(kept, sample)
		}
	}
	return kept
}

// currentOf reads the current value of a key as a Number.
func currentOf(statistics map[Key]Statistics, key Key) Number {
	if value, ok := statistics[key]; ok && value.State == StateValue {
		return Number{Value: value.Current, Known: true}
	}
	return Number{}
}

// nodeIdentities follows _identities and the merge of collect: a record per
// node of kube_node_info, with the internal IP and the hostname when the
// node has one value of each, and a bare record for each other node the
// answers name. The provider id of the original is never used to resolve
// anything and is not read. outside reports a node named only outside
// kube_node_info. The records are sorted by name.
func nodeIdentities(a nodeAnswers) (identities []resolve.NodeRecord, outside bool) {
	ips := make(map[string]map[string]bool)
	hostnames := make(map[string]map[string]bool)
	known := make(map[string]bool)
	add := func(sets map[string]map[string]bool, node, value string) {
		if value == "" {
			return
		}
		if sets[node] == nil {
			sets[node] = make(map[string]bool)
		}
		sets[node][value] = true
	}
	for _, sample := range a.info {
		node := sample.Labels["node"]
		if node == "" || !finiteSample(sample) {
			continue
		}
		known[node] = true
		add(ips, node, sample.Labels["internal_ip"])
		add(hostnames, node, sample.Labels["hostname"])
	}

	evidence := make(map[string]bool)
	for _, samples := range [][]prom.Sample{
		a.labels, a.capacityCPU, a.capacityMemory, a.allocatableCPU, a.allocatableMemory,
		a.ready, a.unschedulable, a.requestsCPU, a.requestsMemory,
	} {
		for _, sample := range samples {
			if node := sample.Labels["node"]; node != "" && finiteSample(sample) {
				evidence[node] = true
			}
		}
	}
	for _, series := range [][]prom.Series{a.usageCPU, a.usageMemory} {
		for _, one := range series {
			node := one.Labels["node"]
			if node == "" {
				continue
			}
			for _, point := range one.Points {
				if point.Valid && isFinite(point.Value) {
					evidence[node] = true
					break
				}
			}
		}
	}
	names := make(map[string]bool)
	for node := range known {
		names[node] = true
	}
	for node := range evidence {
		names[node] = true
		if !known[node] {
			outside = true
		}
	}

	single := func(set map[string]bool) string {
		if len(set) != 1 {
			return ""
		}
		for value := range set {
			return value
		}
		return ""
	}
	for node := range names {
		identities = append(identities, resolve.NodeRecord{
			Name:       node,
			InternalIP: single(ips[node]),
			Hostname:   single(hostnames[node]),
		})
	}
	sort.Slice(identities, func(i, j int) bool { return identities[i].Name < identities[j].Name })
	return identities, outside
}

// enrichHostnames follows _enrich_hostnames: a node without a hostname takes
// the nodename of node_uname_info when its series, resolved through an index
// of the identities, name exactly one distinct host.
func enrichHostnames(identities []resolve.NodeRecord, uname []prom.Sample) []resolve.NodeRecord {
	index := resolve.NewNodeIndex(identities)
	candidates := make(map[string]map[string]bool)
	for _, sample := range uname {
		if !finiteSample(sample) {
			continue
		}
		name, ok := index.Resolve(sample.Labels["node"], sample.Labels["instance"])
		host := resolve.InstanceHost(sample.Labels["nodename"])
		if !ok || host == "" {
			continue
		}
		if candidates[name] == nil {
			candidates[name] = make(map[string]bool)
		}
		candidates[name][host] = true
	}
	enriched := make([]resolve.NodeRecord, len(identities))
	for i, identity := range identities {
		if identity.Hostname == "" && len(candidates[identity.Name]) == 1 {
			for host := range candidates[identity.Name] {
				identity.Hostname = host
			}
		}
		enriched[i] = identity
	}
	return enriched
}

// nodeLabels follows the labels of collect: per node, each label of
// kube_node_labels other than node that holds one value across the node's
// series.
func nodeLabels(samples []prom.Sample) map[string]map[string]string {
	values := make(map[string]map[string]map[string]bool)
	for _, sample := range samples {
		node := sample.Labels["node"]
		if node == "" || !finiteSample(sample) {
			continue
		}
		if values[node] == nil {
			values[node] = make(map[string]map[string]bool)
		}
		for label, value := range sample.Labels {
			if label == "node" || value == "" {
				continue
			}
			if values[node][label] == nil {
				values[node][label] = make(map[string]bool)
			}
			values[node][label][value] = true
		}
	}
	labels := make(map[string]map[string]string, len(values))
	for node, byLabel := range values {
		labels[node] = make(map[string]string)
		for label, set := range byLabel {
			if len(set) != 1 {
				continue
			}
			for value := range set {
				labels[node][label] = value
			}
		}
	}
	return labels
}

// resolvedStatistics follows _resolved_statistics: each series is resolved
// to a node through the index, a series that resolves to none is dropped,
// and the rest are reduced by node with the maximum.
func resolvedStatistics(series []prom.Series, index resolve.NodeIndex) map[Key]Statistics {
	resolved := make([]prom.Series, 0, len(series))
	for _, one := range series {
		name, ok := index.Resolve(one.Labels["node"], one.Labels["instance"])
		if !ok {
			continue
		}
		resolved = append(resolved, prom.Series{Labels: map[string]string{"node": name}, Points: one.Points})
	}
	return ReduceMax(resolved, []string{"node"})
}

// readyByNode follows _ready_by_node: a node is ready when its only active
// status is true, not ready when it is false, and unknown otherwise.
func readyByNode(samples []prom.Sample, at float64) map[string]Flag {
	withStatus := make([]prom.Sample, 0, len(samples))
	for _, sample := range samples {
		if sample.Labels["node"] != "" && sample.Labels["status"] != "" {
			withStatus = append(withStatus, sample)
		}
	}
	active := make(map[string]map[string]bool)
	for key, statistics := range ReduceMax(instantSeries(withStatus, at), []string{"node", "status"}) {
		if statistics.State != StateValue || statistics.Current <= 0 {
			continue
		}
		if active[key[0]] == nil {
			active[key[0]] = make(map[string]bool)
		}
		active[key[0]][key[1]] = true
	}
	ready := make(map[string]Flag, len(active))
	for node, statuses := range active {
		switch {
		case len(statuses) == 1 && statuses["true"]:
			ready[node] = Flag{Value: true, Known: true}
		case len(statuses) == 1 && statuses["false"]:
			ready[node] = Flag{Value: false, Known: true}
		}
	}
	return ready
}

// confidenceRank orders the confidence of a pool: unknown below inferred
// below exact.
var confidenceRank = map[string]int{
	resolve.ConfidenceUnknown:  0,
	resolve.ConfidenceInferred: 1,
	resolve.ConfidenceExact:    2,
}

// Pools follows group_nodepools: the nodes grouped by pool name, the pools
// sorted by name and the nodes of each by name, with the lowest confidence
// of its nodes.
func Pools(nodes []Node) []Pool {
	byName := make(map[string]*Pool)
	for _, node := range nodes {
		pool := byName[node.Pool]
		if pool == nil {
			pool = &Pool{Name: node.Pool, Confidence: node.PoolConfidence}
			byName[node.Pool] = pool
		} else if confidenceRank[node.PoolConfidence] < confidenceRank[pool.Confidence] {
			pool.Confidence = node.PoolConfidence
		}
		pool.Nodes = append(pool.Nodes, node.Name)
	}
	pools := make([]Pool, 0, len(byName))
	for _, pool := range byName {
		sort.Strings(pool.Nodes)
		pools = append(pools, *pool)
	}
	sort.Slice(pools, func(i, j int) bool { return pools[i].Name < pools[j].Name })
	return pools
}
