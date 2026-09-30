package collect

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/marceloalvescf/k8s-capacity-report/internal/discovery"
	"github.com/marceloalvescf/k8s-capacity-report/internal/prom"
	"github.com/marceloalvescf/k8s-capacity-report/internal/resolve"
)

// The metric names the raw-metrics workload strategy queries, and the
// recording rule that joins usage to a workload for the other strategy,
// following metrics/names.py of the original.
const (
	kubePodOwner                            = "kube_pod_owner"
	kubeReplicasetOwner                     = "kube_replicaset_owner"
	namespaceWorkloadPodKubePodOwnerRelabel = "namespace_workload_pod:kube_pod_owner:relabel"
)

// workloadUsageCapability is the name Input.Capabilities carries the
// workload-usage capability under.
const workloadUsageCapability = "workload_usage"

// workloadKeyLabels are WORKLOAD_KEY_LABELS of the original: the labels a
// workload row is keyed by, in the order Key holds them.
var workloadKeyLabels = []string{"namespace", "workload", "workload_type"}

// excludedWorkloadKind is the one workload kind the inventory leaves out,
// EXCLUDED_WORKLOAD_KINDS of the original: a Job is one finite execution,
// so a frequent CronJob would fill the inventory with short-lived rows
// competing against long-running workloads. Its usage still reaches the
// namespace totals.
const excludedWorkloadKind = "Job"

// Workload is the row of one workload.
type Workload struct {
	Namespace           string
	Kind                string
	Name                string
	Pods                int
	PodsKnown           bool
	Approximate         bool
	RequestsLimitsState string
	Resources           Resources
}

// Workloads follows WorkloadCollector.collect of collectors/workloads.py:
// it sends the queries of the strategy the workload_usage capability
// names, and returns one row for each workload the answers and the
// ownership records name, sorted, a Job excluded.
func Workloads(ctx context.Context, q Querier, in Input) (Result[Workload], error) {
	workloadCap, err := findCapability(in.Capabilities, workloadUsageCapability)
	if err != nil {
		return Result[Workload]{}, err
	}
	resourceCap, err := findCapability(in.Capabilities, requestsLimitsCapability)
	if err != nil {
		return Result[Workload]{}, err
	}

	if workloadCap.State == discovery.StateUnavailable || workloadCap.Strategy == discovery.StrategyNone {
		return emptyWorkloadResult(workloadCap.Strategy), nil
	}

	switch workloadCap.Strategy {
	case discovery.StrategyRecordingRule:
		return collectRecordingWorkloads(ctx, q, in, workloadCap, resourceCap)
	case discovery.StrategyRawMetrics:
		return collectRawWorkloads(ctx, q, in, workloadCap, resourceCap)
	default:
		return emptyWorkloadResult(workloadCap.Strategy), nil
	}
}

// emptyWorkloadResult is the result of a workload_usage capability that
// sends no query: unavailable, the capability's own strategy, and nothing
// else.
func emptyWorkloadResult(strategy string) Result[Workload] {
	return Result[Workload]{
		State:    discovery.StateUnavailable,
		Strategy: strategy,
		Rows:     []Workload{},
		Warnings: []Message{},
		Notices:  []Message{},
	}
}

// collectRecordingWorkloads follows _collect_recording of the original: six
// range queries, each joining usage or a resource to ownership inside the
// query text, then one instant query counting active pods per workload.
func collectRecordingWorkloads(ctx context.Context, q Querier, in Input, workloadCap, resourceCap discovery.Capability) (Result[Workload], error) {
	legacy := usesLegacyResourceNames(resourceCap.Evidence)

	cpuRaw, err := q.QueryRange(ctx, workloadJoinedExpression(containerCPUUsageSecondsTotal, "", true, in), in.Window.Start, in.Window.End, in.Window.Step)
	if err != nil {
		return Result[Workload]{}, err
	}
	cpuSeries := normalizeWorkloadType(cpuRaw)

	memoryRaw, err := q.QueryRange(ctx, workloadJoinedExpression(containerMemoryWorkingSetBytes, "", false, in), in.Window.Start, in.Window.End, in.Window.Step)
	if err != nil {
		return Result[Workload]{}, err
	}
	memorySeries := normalizeWorkloadType(memoryRaw)

	resourceRaw, err := workloadResourceSeries(ctx, q, in, resourceCap, legacy, func(name, resourceKind string) string {
		return workloadJoinedExpression(name, resourceKind, false, in)
	})
	if err != nil {
		return Result[Workload]{}, err
	}
	cpuRequests := normalizeWorkloadType(resourceRaw[0])
	memoryRequests := normalizeWorkloadType(resourceRaw[1])
	cpuLimits := normalizeWorkloadType(resourceRaw[2])
	memoryLimits := normalizeWorkloadType(resourceRaw[3])

	statistics, approximate := workloadStatisticsAndRatios(cpuSeries, memorySeries, cpuRequests, memoryRequests, cpuLimits, memoryLimits, resourceCap.State)

	podSamples, err := q.Query(ctx, workloadPodCountQuery(in), in.Window.End)
	if err != nil {
		return Result[Workload]{}, err
	}
	podSeries := normalizeWorkloadType(instantSeries(podSamples, float64(in.Window.End.Unix())))
	podStatistics := Reduce(podSeries, workloadKeyLabels)
	activePods := make(map[Key]int, len(podStatistics))
	for key, stat := range podStatistics {
		activePods[key] = int(stat.Current)
	}

	usageFamilies := 0
	if len(statistics["cpu_usage"]) > 0 {
		usageFamilies++
	}
	if len(statistics["memory_usage"]) > 0 {
		usageFamilies++
	}

	var state string
	switch {
	case usageFamilies == 0:
		state = discovery.StateUnavailable
	case usageFamilies == 2 && resourceCap.State == discovery.StateAvailable:
		state = discovery.StateAvailable
	default:
		state = discovery.StatePartial
	}

	warnings := []Message{}
	if usageFamilies < 2 {
		warnings = append(warnings, Message{Code: WarningWorkloadUsageIncomplete, Params: map[string]float64{}})
	}
	warnings = append(warnings, workloadResourceWarning(resourceCap.State)...)

	if err := checkWorkloadKeys(statistics, activePods); err != nil {
		return Result[Workload]{}, err
	}
	rows, omittedNoUsage, omittedJobs := assembleWorkloads(statistics, activePods, approximate, workloadCap.State, resourceCap.State)

	return Result[Workload]{
		State:    state,
		Strategy: discovery.StrategyRecordingRule,
		Rows:     rows,
		Warnings: warnings,
		Notices:  workloadOmissionNotices(omittedNoUsage, omittedJobs),
	}, nil
}

// collectRawWorkloads follows _collect_raw of the original: two instant
// ownership queries build the ownership map client-side, then six range
// queries grouped only by namespace and pod are relabeled to a workload
// with it.
func collectRawWorkloads(ctx context.Context, q Querier, in Input, workloadCap, resourceCap discovery.Capability) (Result[Workload], error) {
	legacy := usesLegacyResourceNames(resourceCap.Evidence)

	podOwnerSamples, err := q.Query(ctx, workloadOwnerQuery(kubePodOwner, in), in.Window.End)
	if err != nil {
		return Result[Workload]{}, err
	}
	replicaSetOwnerSamples, err := q.Query(ctx, workloadOwnerQuery(kubeReplicasetOwner, in), in.Window.End)
	if err != nil {
		return Result[Workload]{}, err
	}
	ownership := resolve.Workloads(
		workloadOwnerRecords(podOwnerSamples, "pod"),
		workloadOwnerRecords(replicaSetOwnerSamples, "replicaset"),
	)

	cpuRaw, err := q.QueryRange(ctx, workloadRawExpression(containerCPUUsageSecondsTotal, "", true, in), in.Window.Start, in.Window.End, in.Window.Step)
	if err != nil {
		return Result[Workload]{}, err
	}
	memoryRaw, err := q.QueryRange(ctx, workloadRawExpression(containerMemoryWorkingSetBytes, "", false, in), in.Window.Start, in.Window.End, in.Window.Step)
	if err != nil {
		return Result[Workload]{}, err
	}
	resourceRaw, err := workloadResourceSeries(ctx, q, in, resourceCap, legacy, func(name, resourceKind string) string {
		return workloadRawExpression(name, resourceKind, false, in)
	})
	if err != nil {
		return Result[Workload]{}, err
	}

	cpuSeries := workloadRelabelRaw(cpuRaw, ownership)
	memorySeries := workloadRelabelRaw(memoryRaw, ownership)
	cpuRequests := workloadRelabelRaw(resourceRaw[0], ownership)
	memoryRequests := workloadRelabelRaw(resourceRaw[1], ownership)
	cpuLimits := workloadRelabelRaw(resourceRaw[2], ownership)
	memoryLimits := workloadRelabelRaw(resourceRaw[3], ownership)

	statistics, approximate := workloadStatisticsAndRatios(cpuSeries, memorySeries, cpuRequests, memoryRequests, cpuLimits, memoryLimits, resourceCap.State)

	activePods := make(map[Key]int)
	for _, identity := range ownership {
		key := Key{identity.Namespace, identity.Name, canonicalOrUnknown(identity.Kind)}
		activePods[key]++
	}

	warnings := []Message{{Code: WarningWorkloadHistoryCurrentAssociation, Params: map[string]float64{}}}
	warnings = append(warnings, workloadResourceWarning(resourceCap.State)...)

	if err := checkWorkloadKeys(statistics, activePods); err != nil {
		return Result[Workload]{}, err
	}
	rows, omittedNoUsage, omittedJobs := assembleWorkloads(statistics, activePods, approximate, workloadCap.State, resourceCap.State)

	return Result[Workload]{
		State:    discovery.StatePartial,
		Strategy: discovery.StrategyRawMetrics,
		Rows:     rows,
		Warnings: warnings,
		Notices:  workloadOmissionNotices(omittedNoUsage, omittedJobs),
	}, nil
}

// workloadStatisticsAndRatios follows _statistics_and_ratios of the
// original, shared by both strategies: the six families are reduced by
// workloadKeyLabels, then the four percent families are computed from
// usage over their matching resource family.
func workloadStatisticsAndRatios(cpuUsage, memoryUsage, cpuRequests, memoryRequests, cpuLimits, memoryLimits []prom.Series, resourceState string) (map[string]map[Key]Statistics, map[Key]bool) {
	statistics := map[string]map[Key]Statistics{
		"cpu_usage":       Reduce(cpuUsage, workloadKeyLabels),
		"memory_usage":    Reduce(memoryUsage, workloadKeyLabels),
		"cpu_requests":    Reduce(cpuRequests, workloadKeyLabels),
		"memory_requests": Reduce(memoryRequests, workloadKeyLabels),
		"cpu_limits":      Reduce(cpuLimits, workloadKeyLabels),
		"memory_limits":   Reduce(memoryLimits, workloadKeyLabels),
	}

	cpuRequestPercent, cpuRequestApprox := Ratios(cpuUsage, cpuRequests, workloadKeyLabels, resourceState)
	memoryRequestPercent, memoryRequestApprox := Ratios(memoryUsage, memoryRequests, workloadKeyLabels, resourceState)
	cpuLimitPercent, cpuLimitApprox := Ratios(cpuUsage, cpuLimits, workloadKeyLabels, resourceState)
	memoryLimitPercent, memoryLimitApprox := Ratios(memoryUsage, memoryLimits, workloadKeyLabels, resourceState)
	statistics["cpu_request_percent"] = cpuRequestPercent
	statistics["memory_request_percent"] = memoryRequestPercent
	statistics["cpu_limit_percent"] = cpuLimitPercent
	statistics["memory_limit_percent"] = memoryLimitPercent

	approximate := mergeApprox(cpuRequestApprox, memoryRequestApprox, cpuLimitApprox, memoryLimitApprox)
	return statistics, approximate
}

// checkWorkloadKeys refuses a slash in the namespace, name, or kind of any
// key statistics or activePods holds, once the kind is already canonical
// and before assembleWorkloads keys a single row by it: a name of
// Kubernetes never holds a slash, so such a label is a fault of the
// endpoint or of the configuration, not a workload to report on.
func checkWorkloadKeys(statistics map[string]map[Key]Statistics, activePods map[Key]int) error {
	checked := make(map[Key]bool)
	check := func(key Key) error {
		if checked[key] {
			return nil
		}
		checked[key] = true
		namespace, name, kind := key[0], key[1], key[2]
		if strings.Contains(namespace, "/") || strings.Contains(name, "/") || strings.Contains(kind, "/") {
			return &prom.Error{Kind: prom.KindQuery, Message: fmt.Sprintf(
				"the endpoint answered with a workload name that holds a slash: namespace %q, name %q, kind %q",
				namespace, name, kind)}
		}
		return nil
	}
	for key := range activePods {
		if err := check(key); err != nil {
			return err
		}
	}
	for _, family := range statistics {
		for key := range family {
			if err := check(key); err != nil {
				return err
			}
		}
	}
	return nil
}

// assembleWorkloads follows _assemble of the original: the keys of
// activePods and of every family of statistics are joined, a key of the
// excluded kind is taken out and counted, and, when workloadState is
// available, a key with no statistic of CPU usage or memory usage is taken
// out and counted too. The remaining keys become rows, sorted by
// namespace, then name, then kind.
func assembleWorkloads(statistics map[string]map[Key]Statistics, activePods map[Key]int, approximate map[Key]bool, workloadState, resourceState string) (rows []Workload, omittedWithoutConsumption, omittedJobs int) {
	keySet := make(map[Key]bool)
	for key := range activePods {
		keySet[key] = true
	}
	for _, family := range statistics {
		for key := range family {
			keySet[key] = true
		}
	}

	for key := range keySet {
		if key[2] == excludedWorkloadKind {
			delete(keySet, key)
			omittedJobs++
		}
	}

	if workloadState == discovery.StateAvailable {
		observed := make(map[Key]bool)
		for _, name := range [2]string{"cpu_usage", "memory_usage"} {
			for key := range statistics[name] {
				observed[key] = true
			}
		}
		for key := range keySet {
			if !observed[key] {
				delete(keySet, key)
				omittedWithoutConsumption++
			}
		}
	}

	keys := make([]Key, 0, len(keySet))
	for key := range keySet {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i][0] != keys[j][0] {
			return keys[i][0] < keys[j][0]
		}
		if keys[i][1] != keys[j][1] {
			return keys[i][1] < keys[j][1]
		}
		return keys[i][2] < keys[j][2]
	})

	rows = make([]Workload, 0, len(keys))
	for _, key := range keys {
		pods, known := activePods[key]
		rows = append(rows, Workload{
			Namespace:           key[0],
			Kind:                key[2],
			Name:                key[1],
			Pods:                pods,
			PodsKnown:           known,
			Approximate:         approximate[key],
			RequestsLimitsState: resourceState,
			Resources: Resources{
				CPUUsage:             Lookup(statistics["cpu_usage"], key),
				MemoryUsage:          Lookup(statistics["memory_usage"], key),
				CPURequests:          Lookup(statistics["cpu_requests"], key),
				MemoryRequests:       Lookup(statistics["memory_requests"], key),
				CPULimits:            Lookup(statistics["cpu_limits"], key),
				MemoryLimits:         Lookup(statistics["memory_limits"], key),
				CPURequestPercent:    Lookup(statistics["cpu_request_percent"], key),
				MemoryRequestPercent: Lookup(statistics["memory_request_percent"], key),
				CPULimitPercent:      Lookup(statistics["cpu_limit_percent"], key),
				MemoryLimitPercent:   Lookup(statistics["memory_limit_percent"], key),
			},
		})
	}
	return rows, omittedWithoutConsumption, omittedJobs
}

// workloadOmissionNotices follows _omission_notices of the original: the
// notice of workloads left out for holding no observed consumption, then
// the notice of Jobs left out, each only when its count is above zero.
func workloadOmissionNotices(omittedWithoutConsumption, omittedJobs int) []Message {
	notices := []Message{}
	if omittedWithoutConsumption > 0 {
		notices = append(notices, Message{Code: NoticeWorkloadsOmittedNoUsage, Params: map[string]float64{"count": float64(omittedWithoutConsumption)}})
	}
	if omittedJobs > 0 {
		notices = append(notices, Message{Code: NoticeJobsOmitted, Params: map[string]float64{"count": float64(omittedJobs)}})
	}
	return notices
}

// workloadResourceWarning follows _resource_warning of the original: one
// warning when requests and limits are partial, a different one when they
// are unavailable, and none when they are available.
func workloadResourceWarning(state string) []Message {
	switch state {
	case discovery.StatePartial:
		return []Message{{Code: WarningWorkloadRequestsLimitsIncomplete, Params: map[string]float64{}}}
	case discovery.StateUnavailable:
		return []Message{{Code: WarningWorkloadRequestsLimitsUnavailable, Params: map[string]float64{}}}
	default:
		return []Message{}
	}
}

// mergeApprox unions the keys every map of maps holds as true.
func mergeApprox(maps ...map[Key]bool) map[Key]bool {
	merged := make(map[Key]bool)
	for _, m := range maps {
		for key, value := range m {
			if value {
				merged[key] = true
			}
		}
	}
	return merged
}

// workloadResourceSeries sends the four requests-and-limits queries, in
// order: requests of CPU, requests of memory, limits of CPU, limits of
// memory, built by expr. It sends nothing, and returns four nil series,
// when capability is unavailable.
func workloadResourceSeries(ctx context.Context, q Querier, in Input, capability discovery.Capability, legacy bool, expr func(name, resourceKind string) string) ([4][]prom.Series, error) {
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
		answer, err := q.QueryRange(ctx, expr(name, resourceKind), in.Window.Start, in.Window.End, in.Window.Step)
		if err != nil {
			return series, err
		}
		series[i] = answer
	}
	return series, nil
}

// workloadOwnershipQuery builds the ownership operand every recording-rule
// query joins to, and the query of the instant pod count: the ownership
// recording rule, matched on the scope and the namespaces of the cluster,
// with no matcher of containers.
func workloadOwnershipQuery(in Input) string {
	return prom.Selector{
		Metric: namespaceWorkloadPodKubePodOwnerRelabel,
		Equal:  in.Scope.Matchers(),
		Regex:  map[string][]string{"namespace": in.Cluster.Namespaces},
	}.String()
}

// workloadPodCountQuery builds the recording strategy's final query: the
// count of active pods per workload.
func workloadPodCountQuery(in Input) string {
	return "count by (namespace, workload, workload_type) (" + workloadOwnershipQuery(in) + ")"
}

// workloadJoinLabels is _join_labels of the original: the labels the usage
// or resource expression is joined to the ownership operand on, the
// scope's own label in place of "cluster" when its matcher is enabled.
func workloadJoinLabels(scope discovery.Scope) string {
	if scope.MatcherEnabled {
		return scope.ClusterLabel + ", namespace, pod"
	}
	return "namespace, pod"
}

// workloadJoinedExpression follows _joined of the original: the selector of
// name, rated when rate is true and deduplicated by container when it is
// the memory metric, summed by namespace, workload, and workload_type after
// being joined to the ownership operand.
func workloadJoinedExpression(name, resourceKind string, rate bool, in Input) string {
	source := containerMetric(name, resourceKind, in)
	if rate {
		source = "rate(" + source + "[" + in.Window.Step + "])"
	}
	join := workloadJoinLabels(in.Scope)
	source = deduplicateByContainer(source, name, join)
	return "sum by (namespace, workload, workload_type) (" + source +
		" * on (" + join + ") group_left(workload, workload_type) " +
		workloadOwnershipQuery(in) + ")"
}

// workloadRawExpression follows _raw_expression of the original: the
// selector of name, rated when rate is true and deduplicated by container
// when it is the memory metric, summed by namespace and pod, with no
// ownership join in the query text.
func workloadRawExpression(name, resourceKind string, rate bool, in Input) string {
	source := containerMetric(name, resourceKind, in)
	if rate {
		source = "rate(" + source + "[" + in.Window.Step + "])"
	}
	source = deduplicateByContainer(source, name, "namespace, pod")
	return "sum by (namespace, pod) (" + source + ")"
}

// deduplicateByContainer follows deduplicate_by_container of common.py: it
// wraps expression in "max by (grouping, container) (...)" only for the
// memory metric, which cAdvisor can expose more than once in a scrape.
func deduplicateByContainer(expression, metric, grouping string) string {
	if metric != containerMemoryWorkingSetBytes {
		return expression
	}
	return "max by (" + grouping + ", container) (" + expression + ")"
}

// workloadOwnerQuery builds one of the raw strategy's two ownership
// queries: metric matched on the scope, owner_is_controller="true", and
// the namespaces of the cluster.
func workloadOwnerQuery(metric string, in Input) string {
	equal := in.Scope.Matchers()
	equal["owner_is_controller"] = "true"
	return prom.Selector{
		Metric: metric,
		Equal:  equal,
		Regex:  map[string][]string{"namespace": in.Cluster.Namespaces},
	}.String()
}

// workloadOwnerRecords reads samples into ownership records, taking the
// child's name from the label named childLabel: "pod" for a pod-owner
// answer, "replicaset" for a replica-set-owner answer.
func workloadOwnerRecords(samples []prom.Sample, childLabel string) []resolve.Owner {
	owners := make([]resolve.Owner, len(samples))
	for i, sample := range samples {
		owners[i] = resolve.Owner{
			Namespace: sample.Labels["namespace"],
			Child:     sample.Labels[childLabel],
			OwnerKind: sample.Labels["owner_kind"],
			OwnerName: sample.Labels["owner_name"],
		}
	}
	return owners
}

// workloadRelabelRaw follows _relabel_raw of the original: each series
// takes the identity ownership names for its pod, or, when ownership names
// none, a workload of kind Pod under the pod's own name.
func workloadRelabelRaw(series []prom.Series, ownership map[resolve.Pod]resolve.Identity) []prom.Series {
	relabeled := make([]prom.Series, len(series))
	for i, item := range series {
		pod := resolve.Pod{Namespace: item.Labels["namespace"], Name: item.Labels["pod"]}
		identity, ok := ownership[pod]
		if !ok {
			identity = resolve.Identity{Namespace: pod.Namespace, Kind: resolve.KindPod, Name: pod.Name}
		}
		relabeled[i] = prom.Series{
			Labels: map[string]string{
				"namespace":     identity.Namespace,
				"workload":      identity.Name,
				"workload_type": canonicalOrUnknown(identity.Kind),
			},
			Points: item.Points,
		}
	}
	return relabeled
}

// normalizeWorkloadType follows _normalize_workloads of the original: the
// label workload_type of every series is replaced by its canonical kind,
// an absent or empty value read as Unknown first.
func normalizeWorkloadType(series []prom.Series) []prom.Series {
	normalized := make([]prom.Series, len(series))
	for i, item := range series {
		labels := make(map[string]string, len(item.Labels))
		for key, value := range item.Labels {
			labels[key] = value
		}
		labels["workload_type"] = canonicalOrUnknown(labels["workload_type"])
		normalized[i] = prom.Series{Labels: labels, Points: item.Points}
	}
	return normalized
}

// canonicalOrUnknown returns resolve.CanonicalKind of kind, kind read as
// "Unknown" first when it is empty.
func canonicalOrUnknown(kind string) string {
	if kind == "" {
		kind = "Unknown"
	}
	return resolve.CanonicalKind(kind)
}

// instantSeries turns the samples of an instant answer into series of one
// point each, at time at, so the answer can be reduced with the same
// normalizeWorkloadType and Reduce a range answer is.
func instantSeries(samples []prom.Sample, at float64) []prom.Series {
	series := make([]prom.Series, len(samples))
	for i, sample := range samples {
		series[i] = prom.Series{
			Labels: sample.Labels,
			Points: []prom.Point{{Time: at, Value: sample.Value, Valid: sample.Valid}},
		}
	}
	return series
}
