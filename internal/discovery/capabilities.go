package discovery

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/marceloalvescf/k8s-capacity-report/internal/config"
	"github.com/marceloalvescf/k8s-capacity-report/internal/prom"
)

// The metric names the probes ask for, kept local to this file: a name
// written twice can diverge in one of them, and the query breaks in
// silence, so every probe below reads its name from here, once.
const (
	metricContainerCPUUsageSecondsTotal             = "container_cpu_usage_seconds_total"
	metricContainerMemoryWorkingSetBytes            = "container_memory_working_set_bytes"
	metricInstanceNodeCPUUtilisationRate5m          = "instance:node_cpu_utilisation:rate5m"
	metricInstanceNodeMemoryUtilisationRatio        = "instance:node_memory_utilisation:ratio"
	metricKubeDeploymentLabels                      = "kube_deployment_labels"
	metricKubeDeploymentSpecReplicas                = "kube_deployment_spec_replicas"
	metricKubeHorizontalPodAutoscalerInfo           = "kube_horizontalpodautoscaler_info"
	metricKubeHorizontalPodAutoscalerSpecMax        = "kube_horizontalpodautoscaler_spec_max_replicas"
	metricKubeHorizontalPodAutoscalerSpecMin        = "kube_horizontalpodautoscaler_spec_min_replicas"
	metricKubeHorizontalPodAutoscalerStatusCurrent  = "kube_horizontalpodautoscaler_status_current_replicas"
	metricKubeHorizontalPodAutoscalerStatusDesired  = "kube_horizontalpodautoscaler_status_desired_replicas"
	metricKubeNodeInfo                              = "kube_node_info"
	metricKubeNodeLabels                            = "kube_node_labels"
	metricKubePodContainerResourceLimits            = "kube_pod_container_resource_limits"
	metricKubePodContainerResourceLimitsCPUCores    = "kube_pod_container_resource_limits_cpu_cores"
	metricKubePodContainerResourceLimitsMemoryBytes = "kube_pod_container_resource_limits_memory_bytes"
	metricKubePodContainerResourceRequests          = "kube_pod_container_resource_requests"
	metricKubePodContainerResourceRequestsCPUCores  = "kube_pod_container_resource_requests_cpu_cores"
	metricKubePodContainerResourceRequestsMemBytes  = "kube_pod_container_resource_requests_memory_bytes"
	metricKubePodOwner                              = "kube_pod_owner"
	metricKubeStatefulSetLabels                     = "kube_statefulset_labels"
	metricKubeStatefulSetReplicas                   = "kube_statefulset_replicas"
	metricNamespaceWorkloadPodKubePodOwnerRelabel   = "namespace_workload_pod:kube_pod_owner:relabel"
	metricNodeCPUSecondsTotal                       = "node_cpu_seconds_total"
	metricNodeMemoryMemAvailableBytes               = "node_memory_MemAvailable_bytes"
	metricNodeMemoryMemTotalBytes                   = "node_memory_MemTotal_bytes"
)

// ErrInsufficient is returned by CheckSufficient. The command maps it to
// exit code 5.
var ErrInsufficient = errors.New("all analytical capabilities are unavailable")

// prober asks the nine capability queries in the fixed order of the
// original, against one scope and one instant, and remembers the first
// query error so that no probe after it is asked.
type prober struct {
	ctx   context.Context
	q     Querier
	scope Scope
	at    time.Time
	err   error
}

// has runs `count(<metric><matchers>)`, where matchers carries the scope's
// matcher plus extra, and reports whether any returned sample is valid and
// above zero. Once a query has failed, has stops asking and always answers
// false, so that Detect issues no probe after the one that failed.
func (p *prober) has(metric string, extra map[string]string) bool {
	if p.err != nil {
		return false
	}
	equal := p.scope.Matchers()
	for name, value := range extra {
		equal[name] = value
	}
	query := fmt.Sprintf("count(%s)", prom.Selector{Metric: metric, Equal: equal}.String())
	samples, err := p.q.Query(p.ctx, query, p.at)
	if err != nil {
		p.err = err
		return false
	}
	for _, sample := range samples {
		if sample.Valid && sample.Value > 0 {
			return true
		}
	}
	return false
}

// hasAutoscaler runs the node_autoscaling probe: a single regex-name
// selector, with the scope's matcher appended as a second matcher when it
// is on. It is built by hand because it names no exact metric, only a
// family.
func (p *prober) hasAutoscaler() bool {
	if p.err != nil {
		return false
	}
	selector := `{__name__=~"cluster_autoscaler_.+"}`
	if p.scope.MatcherEnabled {
		selector = fmt.Sprintf(`{__name__=~"cluster_autoscaler_.+",%s="%s"}`, p.scope.ClusterLabel, prom.Quote(p.scope.ClusterName))
	}
	query := fmt.Sprintf("count(%s)", selector)
	samples, err := p.q.Query(p.ctx, query, p.at)
	if err != nil {
		p.err = err
		return false
	}
	for _, sample := range samples {
		if sample.Valid && sample.Value > 0 {
			return true
		}
	}
	return false
}

// Detect returns the nine capabilities, in the order of Names. The probes
// are asked in the order of the original, with its short cuts kept: a probe
// the original would not ask in a given case is not asked here either. Once
// a query fails, Detect asks no further probe and returns that error.
func Detect(ctx context.Context, q Querier, scope Scope, analysis config.Analysis, at time.Time) ([]Capability, error) {
	p := &prober{ctx: ctx, q: q, scope: scope, at: at}

	cpuUsage := p.has(metricContainerCPUUsageSecondsTotal, nil)
	memoryUsage := p.has(metricContainerMemoryWorkingSetBytes, nil)
	workloadRule := p.has(metricNamespaceWorkloadPodKubePodOwnerRelabel, nil)
	workloadRaw := p.has(metricKubePodOwner, nil)

	unifiedNames := []string{
		metricKubePodContainerResourceRequests, metricKubePodContainerResourceRequests,
		metricKubePodContainerResourceLimits, metricKubePodContainerResourceLimits,
	}
	unifiedResources := []string{"cpu", "memory", "cpu", "memory"}
	unifiedPresence := make([]bool, len(unifiedNames))
	for i := range unifiedNames {
		unifiedPresence[i] = p.has(unifiedNames[i], map[string]string{"resource": unifiedResources[i]})
	}

	legacyNames := []string{
		metricKubePodContainerResourceRequestsCPUCores, metricKubePodContainerResourceRequestsMemBytes,
		metricKubePodContainerResourceLimitsCPUCores, metricKubePodContainerResourceLimitsMemoryBytes,
	}
	legacyPresence := make([]bool, len(legacyNames))
	for i, name := range legacyNames {
		legacyPresence[i] = p.has(name, nil)
	}

	hpaNames := []string{
		metricKubeHorizontalPodAutoscalerInfo, metricKubeHorizontalPodAutoscalerSpecMin,
		metricKubeHorizontalPodAutoscalerSpecMax, metricKubeHorizontalPodAutoscalerStatusCurrent,
		metricKubeHorizontalPodAutoscalerStatusDesired,
	}
	hpaPresence := make([]bool, len(hpaNames))
	for i, name := range hpaNames {
		hpaPresence[i] = p.has(name, nil)
	}

	deploymentLabels := p.has(metricKubeDeploymentLabels, nil)
	deploymentReplicas := !deploymentLabels && p.has(metricKubeDeploymentSpecReplicas, nil)
	statefulsetLabels := p.has(metricKubeStatefulSetLabels, nil)
	statefulsetReplicas := !statefulsetLabels && p.has(metricKubeStatefulSetReplicas, nil)

	nodeInfo := p.has(metricKubeNodeInfo, nil)
	nodeLabels := p.has(metricKubeNodeLabels, nil)

	nodeRules := p.has(metricInstanceNodeCPUUtilisationRate5m, nil) && p.has(metricInstanceNodeMemoryUtilisationRatio, nil)
	nodeRaw := p.has(metricNodeCPUSecondsTotal, nil) &&
		p.has(metricNodeMemoryMemAvailableBytes, nil) &&
		p.has(metricNodeMemoryMemTotalBytes, nil)

	autoscalerMetrics := p.hasAutoscaler()

	if p.err != nil {
		return nil, p.err
	}

	return []Capability{
		namespaceUsageCapability(cpuUsage, memoryUsage),
		workloadUsageCapability(cpuUsage, memoryUsage, workloadRule, workloadRaw),
		requestsLimitsCapability(unifiedNames, unifiedPresence, legacyNames, legacyPresence),
		hpaHistoryCapability(hpaNames, hpaPresence),
		workloadsWithoutHPACapability(deploymentLabels, deploymentReplicas, statefulsetLabels, statefulsetReplicas, hpaPresence[0]),
		ipNodeMappingCapability(nodeInfo, nodeRules, nodeRaw),
		nodepoolDiscoveryCapability(nodeLabels, nodeInfo),
		nodeUtilizationCapability(nodeRules, nodeRaw),
		nodeAutoscalingCapability(analysis.NodeAutoscaling, autoscalerMetrics),
	}, nil
}

// namespaceUsageCapability reports whether container CPU and memory usage
// are visible.
func namespaceUsageCapability(cpuUsage, memoryUsage bool) Capability {
	evidence := usagePresent(cpuUsage, memoryUsage)
	switch {
	case cpuUsage && memoryUsage:
		return Capability{Name: "namespace_usage", State: StateAvailable, Strategy: StrategyRawMetrics, Evidence: evidence, Reasons: []string{}}
	case cpuUsage || memoryUsage:
		return Capability{Name: "namespace_usage", State: StatePartial, Strategy: StrategyRawMetrics, Evidence: evidence, Reasons: []string{missingUsageReason(cpuUsage)}}
	default:
		return Capability{Name: "namespace_usage", State: StateUnavailable, Strategy: StrategyNone, Evidence: []string{}, Reasons: []string{reasonUsageMetricsNotFound}}
	}
}

// workloadUsageCapability reports whether container usage can be attributed
// to a workload rather than only a pod.
func workloadUsageCapability(cpuUsage, memoryUsage, workloadRule, workloadRaw bool) Capability {
	usageEvidence := usagePresent(cpuUsage, memoryUsage)

	if len(usageEvidence) == 0 {
		evidence := []string{}
		if workloadRule {
			evidence = append(evidence, metricNamespaceWorkloadPodKubePodOwnerRelabel)
		}
		if workloadRaw {
			evidence = append(evidence, metricKubePodOwner)
		}
		return Capability{Name: "workload_usage", State: StateUnavailable, Strategy: StrategyNone, Evidence: evidence, Reasons: []string{reasonUsageMetricsNotFound}}
	}

	usageComplete := cpuUsage && memoryUsage

	if workloadRule {
		evidence := append([]string{metricNamespaceWorkloadPodKubePodOwnerRelabel}, usageEvidence...)
		if usageComplete {
			return Capability{Name: "workload_usage", State: StateAvailable, Strategy: StrategyRecordingRule, Evidence: evidence, Reasons: []string{}}
		}
		return Capability{Name: "workload_usage", State: StatePartial, Strategy: StrategyRecordingRule, Evidence: evidence, Reasons: []string{missingUsageReason(cpuUsage)}}
	}

	if workloadRaw {
		evidence := append([]string{metricKubePodOwner}, usageEvidence...)
		reasons := []string{reasonWorkloadHistoryInexact}
		if !usageComplete {
			reasons = append(reasons, missingUsageReason(cpuUsage))
		}
		return Capability{Name: "workload_usage", State: StatePartial, Strategy: StrategyRawMetrics, Evidence: evidence, Reasons: reasons}
	}

	return Capability{Name: "workload_usage", State: StatePartial, Strategy: StrategyRawMetrics, Evidence: usageEvidence, Reasons: []string{reasonWorkloadAssociationUnavailable}}
}

// requestsLimitsCapability reports whether the report can see requests and
// limits, preferring the unified metrics over the legacy ones.
func requestsLimitsCapability(unifiedNames []string, unifiedPresence []bool, legacyNames []string, legacyPresence []bool) Capability {
	switch {
	case allTrue(unifiedPresence):
		return Capability{
			Name: "requests_limits", State: StateAvailable, Strategy: StrategyRawMetrics,
			Evidence: []string{metricKubePodContainerResourceRequests, metricKubePodContainerResourceLimits},
			Reasons:  []string{},
		}
	case allTrue(legacyPresence):
		return Capability{
			Name: "requests_limits", State: StateAvailable, Strategy: StrategyRawMetrics,
			Evidence: append([]string{}, legacyNames...),
			Reasons:  []string{},
		}
	case anyTrue(unifiedPresence):
		return Capability{
			Name: "requests_limits", State: StatePartial, Strategy: StrategyRawMetrics,
			Evidence: dedupPresent(unifiedNames, unifiedPresence),
			Reasons:  []string{reasonRequestsLimitsIncomplete},
		}
	case anyTrue(legacyPresence):
		return Capability{
			Name: "requests_limits", State: StatePartial, Strategy: StrategyRawMetrics,
			Evidence: presentNames(legacyNames, legacyPresence),
			Reasons:  []string{reasonLegacyMetricsPartial},
		}
	default:
		return Capability{Name: "requests_limits", State: StateUnavailable, Strategy: StrategyNone, Evidence: []string{}, Reasons: []string{reasonMetricsNotFound}}
	}
}

// hpaHistoryCapability reports whether the five HPA metrics are visible.
func hpaHistoryCapability(names []string, presence []bool) Capability {
	switch {
	case allTrue(presence):
		return Capability{Name: "hpa_history", State: StateAvailable, Strategy: StrategyRawMetrics, Evidence: append([]string{}, names...), Reasons: []string{}}
	case anyTrue(presence):
		return Capability{Name: "hpa_history", State: StatePartial, Strategy: StrategyRawMetrics, Evidence: presentNames(names, presence), Reasons: []string{reasonHPAHistoryIncomplete}}
	default:
		return Capability{Name: "hpa_history", State: StateUnavailable, Strategy: StrategyNone, Evidence: []string{}, Reasons: []string{reasonMetricsNotFound}}
	}
}

// workloadsWithoutHPACapability reports whether the report can tell which
// workloads have no HPA target.
func workloadsWithoutHPACapability(deploymentLabels, deploymentReplicas, statefulsetLabels, statefulsetReplicas, hpaInfo bool) Capability {
	names := []string{metricKubeDeploymentLabels, metricKubeDeploymentSpecReplicas, metricKubeStatefulSetLabels, metricKubeStatefulSetReplicas, metricKubeHorizontalPodAutoscalerInfo}
	presence := []bool{deploymentLabels, deploymentReplicas, statefulsetLabels, statefulsetReplicas, hpaInfo}
	evidence := presentNames(names, presence)

	deploymentInventory := deploymentLabels || deploymentReplicas
	statefulsetInventory := statefulsetLabels || statefulsetReplicas

	switch {
	case deploymentInventory && statefulsetInventory && hpaInfo:
		return Capability{Name: "workloads_without_hpa", State: StateAvailable, Strategy: StrategyRawMetrics, Evidence: evidence, Reasons: []string{}}
	case len(evidence) > 0:
		return Capability{Name: "workloads_without_hpa", State: StatePartial, Strategy: StrategyRawMetrics, Evidence: evidence, Reasons: []string{reasonWorkloadInventoryIncomplete}}
	default:
		return Capability{Name: "workloads_without_hpa", State: StateUnavailable, Strategy: StrategyNone, Evidence: []string{}, Reasons: []string{reasonMetricsNotFound}}
	}
}

// ipNodeMappingCapability reports whether pod IPs can be mapped to a node.
func ipNodeMappingCapability(nodeInfo, nodeRules, nodeRaw bool) Capability {
	switch {
	case nodeInfo:
		return Capability{Name: "ip_node_mapping", State: StateAvailable, Strategy: StrategyRawMetrics, Evidence: []string{metricKubeNodeInfo}, Reasons: []string{}}
	case nodeRules:
		return Capability{
			Name: "ip_node_mapping", State: StatePartial, Strategy: StrategyRawMetrics,
			Evidence: []string{metricInstanceNodeCPUUtilisationRate5m, metricInstanceNodeMemoryUtilisationRatio},
			Reasons:  []string{reasonNodeAddressMappingIncomplete},
		}
	case nodeRaw:
		return Capability{
			Name: "ip_node_mapping", State: StatePartial, Strategy: StrategyRawMetrics,
			Evidence: []string{metricNodeCPUSecondsTotal, metricNodeMemoryMemAvailableBytes, metricNodeMemoryMemTotalBytes},
			Reasons:  []string{reasonNodeAddressMappingIncomplete},
		}
	default:
		return Capability{Name: "ip_node_mapping", State: StateUnavailable, Strategy: StrategyNone, Evidence: []string{}, Reasons: []string{reasonMetricsNotFound}}
	}
}

// nodepoolDiscoveryCapability reports whether node pools can be told apart.
func nodepoolDiscoveryCapability(nodeLabels, nodeInfo bool) Capability {
	switch {
	case nodeLabels:
		return Capability{Name: "nodepool_discovery", State: StateAvailable, Strategy: StrategyRawMetrics, Evidence: []string{metricKubeNodeLabels}, Reasons: []string{}}
	case nodeInfo:
		return Capability{Name: "nodepool_discovery", State: StatePartial, Strategy: StrategyRawMetrics, Evidence: []string{metricKubeNodeInfo}, Reasons: []string{reasonNodepoolLabelsNotFound}}
	default:
		return Capability{Name: "nodepool_discovery", State: StateUnavailable, Strategy: StrategyNone, Evidence: []string{}, Reasons: []string{reasonMetricsNotFound}}
	}
}

// nodeUtilizationCapability reports whether node CPU and memory
// utilisation can be measured, preferring the recording rules.
func nodeUtilizationCapability(nodeRules, nodeRaw bool) Capability {
	switch {
	case nodeRules:
		return Capability{
			Name: "node_utilization", State: StateAvailable, Strategy: StrategyRecordingRule,
			Evidence: []string{metricInstanceNodeCPUUtilisationRate5m, metricInstanceNodeMemoryUtilisationRatio},
			Reasons:  []string{},
		}
	case nodeRaw:
		return Capability{
			Name: "node_utilization", State: StateAvailable, Strategy: StrategyRawMetrics,
			Evidence: []string{metricNodeCPUSecondsTotal, metricNodeMemoryMemAvailableBytes, metricNodeMemoryMemTotalBytes},
			Reasons:  []string{},
		}
	default:
		return Capability{Name: "node_utilization", State: StateUnavailable, Strategy: StrategyNone, Evidence: []string{}, Reasons: []string{reasonMetricsNotFound}}
	}
}

// nodeAutoscalingCapability reports whether node autoscaling activity can
// be measured. A configured answer never depends on the probe, even though
// the probe is always asked.
func nodeAutoscalingCapability(configured string, autoscalerMetrics bool) Capability {
	if configured == "enabled" || configured == "disabled" {
		return Capability{Name: "node_autoscaling", State: StateAvailable, Strategy: StrategyConfigured, Evidence: []string{}, Reasons: []string{}}
	}
	if autoscalerMetrics {
		return Capability{Name: "node_autoscaling", State: StateAvailable, Strategy: StrategyRawMetrics, Evidence: []string{"cluster_autoscaler_*"}, Reasons: []string{}}
	}
	return Capability{Name: "node_autoscaling", State: StatePartial, Strategy: StrategyNone, Evidence: []string{}, Reasons: []string{reasonNodeAutoscalingUndetermined}}
}

// CheckSufficient reports ErrInsufficient when the endpoint carries too
// little for the report to say anything: every capability other than
// node_autoscaling is unavailable, all eight of them, and node_autoscaling
// itself is not available. With nothing at all to collect there is nothing
// to report; with any one of the eight present, or with node_autoscaling
// available (even by configuration alone), the run goes on — the original
// does the same, and a run whose only evidence is the configured state of
// node autoscaling is exactly the case this keeps going.
//
// It first requires capabilities to hold exactly the nine capabilities of
// Names, in that order, each with one of the three known states — a list
// that is short, reordered, duplicated, or carries an unrecognised state is
// a caller error, reported as such, and never read as "sufficient" by a
// lookup that silently treats a missing or malformed entry as "not
// unavailable".
func CheckSufficient(capabilities []Capability) error {
	if err := checkCapabilityList(capabilities); err != nil {
		return err
	}

	byName := make(map[string]Capability, len(capabilities))
	for _, capability := range capabilities {
		byName[capability.Name] = capability
	}

	for _, name := range Names {
		if name == "node_autoscaling" {
			continue
		}
		if byName[name].State != StateUnavailable {
			return nil
		}
	}
	if byName["node_autoscaling"].State == StateAvailable {
		return nil
	}
	return ErrInsufficient
}

// checkCapabilityList requires capabilities to hold exactly the nine
// capabilities of Names, in that order, each carrying one of the three
// known states.
func checkCapabilityList(capabilities []Capability) error {
	names := make([]string, len(capabilities))
	for i, capability := range capabilities {
		names[i] = capability.Name
	}
	if !slices.Equal(names, Names) {
		found := "none"
		if len(names) > 0 {
			found = strings.Join(names, ", ")
		}
		return fmt.Errorf("capabilities must hold the nine capabilities in their order; found: %s", found)
	}
	for _, capability := range capabilities {
		if !slices.Contains(States, capability.State) {
			return fmt.Errorf("capability %q has an unknown state: %q", capability.Name, capability.State)
		}
	}
	return nil
}

// usagePresent lists the container usage metrics found, CPU before memory.
func usagePresent(cpuUsage, memoryUsage bool) []string {
	evidence := []string{}
	if cpuUsage {
		evidence = append(evidence, metricContainerCPUUsageSecondsTotal)
	}
	if memoryUsage {
		evidence = append(evidence, metricContainerMemoryWorkingSetBytes)
	}
	return evidence
}

// missingUsageReason names the one usage metric that is missing. It is
// only called where exactly one of cpuUsage and memoryUsage is false.
func missingUsageReason(cpuUsage bool) string {
	if !cpuUsage {
		return reasonCPUUsageMetricNotFound
	}
	return reasonMemoryUsageMetricNotFound
}

// presentNames returns the names whose presence is true, in the order
// given.
func presentNames(names []string, presence []bool) []string {
	out := []string{}
	for i, present := range presence {
		if present {
			out = append(out, names[i])
		}
	}
	return out
}

// dedupPresent returns the names whose presence is true, in the order
// given, keeping each distinct name once.
func dedupPresent(names []string, presence []bool) []string {
	out := []string{}
	seen := make(map[string]struct{}, len(names))
	for i, present := range presence {
		if !present {
			continue
		}
		if _, ok := seen[names[i]]; ok {
			continue
		}
		seen[names[i]] = struct{}{}
		out = append(out, names[i])
	}
	return out
}

// allTrue reports whether every value of values is true.
func allTrue(values []bool) bool {
	for _, v := range values {
		if !v {
			return false
		}
	}
	return true
}

// anyTrue reports whether at least one value of values is true.
func anyTrue(values []bool) bool {
	for _, v := range values {
		if v {
			return true
		}
	}
	return false
}
