package collect

import (
	"context"
	"sort"
	"strings"

	"github.com/marceloalvescf/capacity-room/internal/discovery"
	"github.com/marceloalvescf/capacity-room/internal/prom"
)

// The metric names the namespace collector queries, following
// metrics/names.py of the original.
const (
	containerCPUUsageSecondsTotal    = "container_cpu_usage_seconds_total"
	containerMemoryWorkingSetBytes   = "container_memory_working_set_bytes"
	kubePodContainerResourceRequests = "kube_pod_container_resource_requests"
	kubePodContainerResourceLimits   = "kube_pod_container_resource_limits"
	kubePodStatusPhase               = "kube_pod_status_phase"
)

// requestsLimitsCapability is the name Input.Capabilities carries the
// requests-and-limits capability under.
const requestsLimitsCapability = "requests_limits"

// Namespace is the row of one namespace.
type Namespace struct {
	Name                string
	Pods                int
	PodsKnown           bool
	Approximate         bool
	RequestsLimitsState string // a state of discovery
	Resources           Resources
}

// Namespaces follows NamespaceCollector.collect of
// collectors/namespaces.py: it sends the seven queries of the original, in
// order, reduces the answers, and returns one row for each namespace of
// in.Cluster, sorted and deduplicated.
func Namespaces(ctx context.Context, q Querier, in Input) (Result[Namespace], error) {
	capability, err := findCapability(in.Capabilities, requestsLimitsCapability)
	if err != nil {
		return Result[Namespace]{}, err
	}

	cpuSeries, err := q.QueryRange(ctx, namespaceCPUQuery(in), in.Window.Start, in.Window.End, in.Window.Step)
	if err != nil {
		return Result[Namespace]{}, err
	}
	cpuStats := Reduce(cpuSeries, []string{"namespace"})

	memorySeries, err := q.QueryRange(ctx, namespaceMemoryQuery(in), in.Window.Start, in.Window.End, in.Window.Step)
	if err != nil {
		return Result[Namespace]{}, err
	}
	memoryStats := Reduce(memorySeries, []string{"namespace"})

	legacy := usesLegacyResourceNames(capability.Evidence)
	resourceSeries, err := namespaceResourceSeries(ctx, q, in, capability, legacy)
	if err != nil {
		return Result[Namespace]{}, err
	}
	cpuRequests := Reduce(resourceSeries[0], []string{"namespace"})
	memoryRequests := Reduce(resourceSeries[1], []string{"namespace"})
	cpuLimits := Reduce(resourceSeries[2], []string{"namespace"})
	memoryLimits := Reduce(resourceSeries[3], []string{"namespace"})

	cpuRequestPercent, cpuRequestApprox := Ratios(cpuSeries, resourceSeries[0], []string{"namespace"}, capability.State)
	memoryRequestPercent, memoryRequestApprox := Ratios(memorySeries, resourceSeries[1], []string{"namespace"}, capability.State)
	cpuLimitPercent, cpuLimitApprox := Ratios(cpuSeries, resourceSeries[2], []string{"namespace"}, capability.State)
	memoryLimitPercent, memoryLimitApprox := Ratios(memorySeries, resourceSeries[3], []string{"namespace"}, capability.State)

	podSamples, err := q.Query(ctx, namespacePodQuery(in), in.Window.End)
	if err != nil {
		return Result[Namespace]{}, err
	}
	totals := podTotals(podSamples)

	namespaces := uniqueSorted(in.Cluster.Namespaces)
	rows := make([]Namespace, 0, len(namespaces))
	for _, namespace := range namespaces {
		key := Key{namespace}
		total, known := totals[namespace]
		pods := 0
		if known {
			pods = int(total)
		}
		approximate := cpuRequestApprox[key] || memoryRequestApprox[key] || cpuLimitApprox[key] || memoryLimitApprox[key]
		rows = append(rows, Namespace{
			Name:                namespace,
			Pods:                pods,
			PodsKnown:           known,
			Approximate:         approximate,
			RequestsLimitsState: capability.State,
			Resources: Resources{
				CPUUsage:             Lookup(cpuStats, key),
				MemoryUsage:          Lookup(memoryStats, key),
				CPURequests:          Lookup(cpuRequests, key),
				MemoryRequests:       Lookup(memoryRequests, key),
				CPULimits:            Lookup(cpuLimits, key),
				MemoryLimits:         Lookup(memoryLimits, key),
				CPURequestPercent:    Lookup(cpuRequestPercent, key),
				MemoryRequestPercent: Lookup(memoryRequestPercent, key),
				CPULimitPercent:      Lookup(cpuLimitPercent, key),
				MemoryLimitPercent:   Lookup(memoryLimitPercent, key),
			},
		})
	}

	usageFamilies := 0
	if len(cpuStats) > 0 {
		usageFamilies++
	}
	if len(memoryStats) > 0 {
		usageFamilies++
	}

	var state string
	switch {
	case usageFamilies == 0:
		state = discovery.StateUnavailable
	case usageFamilies == 2 && capability.State == discovery.StateAvailable:
		state = discovery.StateAvailable
	default:
		state = discovery.StatePartial
	}

	warnings := []Message{}
	if usageFamilies < 2 {
		warnings = append(warnings, Message{Code: WarningNamespaceUsageIncomplete, Params: map[string]float64{}})
	}
	switch capability.State {
	case discovery.StatePartial:
		warnings = append(warnings, Message{Code: WarningNamespaceRequestsLimitsIncomplete, Params: map[string]float64{}})
	case discovery.StateUnavailable:
		warnings = append(warnings, Message{Code: WarningNamespaceRequestsLimitsUnavailable, Params: map[string]float64{}})
	}

	return Result[Namespace]{
		State:    state,
		Strategy: discovery.StrategyRawMetrics,
		Rows:     rows,
		Warnings: warnings,
		Notices:  []Message{},
	}, nil
}

// namespaceResourceSeries sends the four requests-and-limits queries, in
// order: requests of CPU, requests of memory, limits of CPU, limits of
// memory. It sends nothing, and returns four nil series, when capability
// is unavailable.
func namespaceResourceSeries(ctx context.Context, q Querier, in Input, capability discovery.Capability, legacy bool) ([4][]prom.Series, error) {
	var series [4][]prom.Series
	if capability.State == discovery.StateUnavailable {
		return series, nil
	}
	families := [4]struct{ metric, kind string }{
		{kubePodContainerResourceRequests, "cpu"},
		{kubePodContainerResourceRequests, "memory"},
		{kubePodContainerResourceLimits, "cpu"},
		{kubePodContainerResourceLimits, "memory"},
	}
	for i, family := range families {
		name, resourceKind := resourceMetricName(family.metric, family.kind, legacy)
		answer, err := q.QueryRange(ctx, namespaceResourceQuery(name, resourceKind, in), in.Window.Start, in.Window.End, in.Window.Step)
		if err != nil {
			return series, err
		}
		series[i] = answer
	}
	return series, nil
}

// usesLegacyResourceNames reports whether evidence names a legacy metric:
// one whose name ends in "_cpu_cores" or "_memory_bytes".
func usesLegacyResourceNames(evidence []string) bool {
	for _, name := range evidence {
		if strings.HasSuffix(name, "_cpu_cores") || strings.HasSuffix(name, "_memory_bytes") {
			return true
		}
	}
	return false
}

// resourceMetricName follows the resource() closure of the original: the
// unified metric and its resource kind unless legacy is true, in which
// case the legacy name carries the kind's suffix and no resource matcher.
func resourceMetricName(metric, kind string, legacy bool) (name, resourceKind string) {
	if !legacy {
		return metric, kind
	}
	suffix := "cpu_cores"
	if kind == "memory" {
		suffix = "memory_bytes"
	}
	return metric + "_" + suffix, ""
}

// namespaceCPUQuery builds the first query: the rate of CPU usage, summed
// by namespace.
func namespaceCPUQuery(in Input) string {
	return "sum by (namespace) (rate(" + containerMetric(containerCPUUsageSecondsTotal, "", in) + "[" + in.Window.Step + "]))"
}

// namespaceMemoryQuery builds the second query: memory usage deduplicated
// by container, then summed by namespace, following
// deduplicate_by_container of the original.
func namespaceMemoryQuery(in Input) string {
	return "sum by (namespace) (max by (namespace, pod, container) (" + containerMetric(containerMemoryWorkingSetBytes, "", in) + "))"
}

// namespaceResourceQuery builds one of the third through sixth queries:
// name, matched on resourceKind when it is not empty, summed by
// namespace.
func namespaceResourceQuery(name, resourceKind string, in Input) string {
	return "sum by (namespace) (" + containerMetric(name, resourceKind, in) + ")"
}

// namespacePodQuery builds the seventh query: the count of running pods,
// summed by namespace. It carries no matcher of containers, unlike the
// first six.
func namespacePodQuery(in Input) string {
	equal := in.Scope.Matchers()
	equal["phase"] = "Running"
	selector := prom.Selector{
		Metric: kubePodStatusPhase,
		Equal:  equal,
		Regex:  map[string][]string{"namespace": in.Cluster.Namespaces},
	}.String()
	return "sum by (namespace) (" + selector + " == 1)"
}

// containerMetric builds the selector text of one of the first six
// queries: the scope's matcher, the resource matcher (left out when
// resourceKind is empty), the namespaces of the cluster in their
// configured order, and the excluded containers of the analysis, always
// preceded by the empty text and "POD".
func containerMetric(name, resourceKind string, in Input) string {
	equal := in.Scope.Matchers()
	if resourceKind != "" {
		equal["resource"] = resourceKind
	}
	excluded := append([]string{"", "POD"}, in.Analysis.ExcludedContainers...)
	return prom.Selector{
		Metric:   name,
		Equal:    equal,
		Regex:    map[string][]string{"namespace": in.Cluster.Namespaces},
		NotRegex: map[string][]string{"container": excluded},
	}.String()
}

// podTotals sums the valid values of samples by their namespace label, in
// the order the answer gave them, following aggregate_statistics of the
// original applied to one instant. A sample without the label "namespace"
// is keyed as unknownLabel. A namespace absent from the result never had a
// valid sample.
func podTotals(samples []prom.Sample) map[string]float64 {
	totals := make(map[string]float64)
	for _, sample := range samples {
		if !sample.Valid {
			continue
		}
		namespace, ok := sample.Labels["namespace"]
		if !ok {
			namespace = unknownLabel
		}
		totals[namespace] += sample.Value
	}
	return totals
}

// uniqueSorted returns the distinct values of values, sorted.
func uniqueSorted(values []string) []string {
	seen := make(map[string]bool, len(values))
	out := make([]string, 0, len(values))
	for _, v := range values {
		if seen[v] {
			continue
		}
		seen[v] = true
		out = append(out, v)
	}
	sort.Strings(out)
	return out
}
