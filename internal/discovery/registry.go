// Package discovery decides what a Prometheus-compatible endpoint can tell
// this report: which cluster values its metrics carry, and which of the
// nine capabilities it can back.
package discovery

// The confidences a Scope or a capability can carry: exact, when the
// discovered evidence names the value directly; inferred, when it was
// concluded from the absence of evidence; unknown, when there was none to
// look at.
const (
	ConfidenceExact    = "exact"
	ConfidenceInferred = "inferred"
	ConfidenceUnknown  = "unknown"
)

// The states a capability can carry.
const (
	StateAvailable   = "available"
	StatePartial     = "partial"
	StateUnavailable = "unavailable"
)

// The strategies a capability can rely on.
const (
	StrategyRecordingRule = "recording_rule"
	StrategyRawMetrics    = "raw_metrics"
	StrategyConfigured    = "configured"
	StrategyNone          = "none"
)

// Names lists the nine capabilities in their fixed order.
var Names = []string{
	"namespace_usage", "workload_usage", "requests_limits", "hpa_history",
	"workloads_without_hpa", "ip_node_mapping", "nodepool_discovery",
	"node_utilization", "node_autoscaling",
}

// States lists the three states a capability can carry.
var States = []string{StateAvailable, StatePartial, StateUnavailable}

// Strategies lists the four strategies a capability can rely on.
var Strategies = []string{
	StrategyRecordingRule, StrategyRawMetrics, StrategyConfigured, StrategyNone,
}

// The reason codes a capability can carry, one per sentence of the
// original.
const (
	reasonMetricsNotFound                = "METRICS_NOT_FOUND"
	reasonCPUUsageMetricNotFound         = "CPU_USAGE_METRIC_NOT_FOUND"
	reasonMemoryUsageMetricNotFound      = "MEMORY_USAGE_METRIC_NOT_FOUND"
	reasonUsageMetricsNotFound           = "USAGE_METRICS_NOT_FOUND"
	reasonWorkloadHistoryInexact         = "WORKLOAD_HISTORY_INEXACT"
	reasonWorkloadAssociationUnavailable = "WORKLOAD_ASSOCIATION_UNAVAILABLE"
	reasonRequestsLimitsIncomplete       = "REQUESTS_LIMITS_INCOMPLETE"
	reasonLegacyMetricsPartial           = "LEGACY_METRICS_PARTIAL"
	reasonHPAHistoryIncomplete           = "HPA_HISTORY_INCOMPLETE"
	reasonWorkloadInventoryIncomplete    = "WORKLOAD_INVENTORY_INCOMPLETE"
	reasonNodeAddressMappingIncomplete   = "NODE_ADDRESS_MAPPING_INCOMPLETE"
	reasonNodepoolLabelsNotFound         = "NODEPOOL_LABELS_NOT_FOUND"
	reasonNodeAutoscalingUndetermined    = "NODE_AUTOSCALING_UNDETERMINED"
)

// ReasonCodes lists every reason a capability can carry.
var ReasonCodes = []string{
	reasonMetricsNotFound,
	reasonCPUUsageMetricNotFound,
	reasonMemoryUsageMetricNotFound,
	reasonUsageMetricsNotFound,
	reasonWorkloadHistoryInexact,
	reasonWorkloadAssociationUnavailable,
	reasonRequestsLimitsIncomplete,
	reasonLegacyMetricsPartial,
	reasonHPAHistoryIncomplete,
	reasonWorkloadInventoryIncomplete,
	reasonNodeAddressMappingIncomplete,
	reasonNodepoolLabelsNotFound,
	reasonNodeAutoscalingUndetermined,
}

// Capability is the state of one of the nine abilities the report needs:
// what was found, how sure of it the discovery is, and, when it is short of
// available, the reason codes that explain why.
type Capability struct {
	Name     string
	State    string
	Strategy string
	Evidence []string // never nil
	Reasons  []string // reason codes, never nil
}
