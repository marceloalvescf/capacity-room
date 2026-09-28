package catalog

import (
	"sort"

	"github.com/marceloalvescf/k8s-capacity-report/internal/discovery"
)

// Key names one entry of the catalog.
type Key string

// The keys of the plain wording table: one constant for each of its rows,
// named after the key in upper camel case.
const (
	LabelsNote    Key = "labels.note"
	LabelsInfo    Key = "labels.info"
	LabelsWarning Key = "labels.warning"

	CoverTitle        Key = "cover.title"
	CoverClusterLabel Key = "cover.cluster_label"
	CoverAuthorLabel  Key = "cover.author_label"
	CoverIssuedLabel  Key = "cover.issued_label"

	FooterPageLabel Key = "footer.page_label"

	FrontOutlineTitle Key = "front.outline_title"

	FrontMetadataHeading              Key = "front.metadata.heading"
	FrontMetadataHeaderField          Key = "front.metadata.header_field"
	FrontMetadataHeaderValue          Key = "front.metadata.header_value"
	FrontMetadataEnvironment          Key = "front.metadata.environment"
	FrontMetadataPeriod               Key = "front.metadata.period"
	FrontMetadataPeriodRange          Key = "front.metadata.period_range"
	FrontMetadataNamespaces           Key = "front.metadata.namespaces"
	FrontMetadataExcludedContainers   Key = "front.metadata.excluded_containers"
	FrontMetadataNoExcludedContainers Key = "front.metadata.no_excluded_containers"
	FrontMetadataOrganization         Key = "front.metadata.organization"
	FrontMetadataClassification       Key = "front.metadata.classification"

	FrontLegendHeading       Key = "front.legend.heading"
	FrontLegendHeaderStatus  Key = "front.legend.header_status"
	FrontLegendHeaderMeaning Key = "front.legend.header_meaning"
	FrontLegendHealthy       Key = "front.legend.healthy"
	FrontLegendAttention     Key = "front.legend.attention"
	FrontLegendCritical      Key = "front.legend.critical"
	FrontLegendUnavailable   Key = "front.legend.unavailable"
	FrontLegendUndefined     Key = "front.legend.undefined"

	FrontQualityHeading     Key = "front.quality.heading"
	FrontQualityAvailable   Key = "front.quality.available"
	FrontQualityPartial     Key = "front.quality.partial"
	FrontQualityUnavailable Key = "front.quality.unavailable"

	AppendixHeading          Key = "appendix.heading"
	AppendixHeaderCapability Key = "appendix.header_capability"
	AppendixHeaderCoverage   Key = "appendix.header_coverage"
	AppendixHeaderSource     Key = "appendix.header_source"
	AppendixHeaderRemark     Key = "appendix.header_remark"
	AppendixHeaderEvidence   Key = "appendix.header_evidence"
	AppendixNoRemark         Key = "appendix.no_remark"
	AppendixEmpty            Key = "appendix.empty"

	StateAvailable   Key = "state.available"
	StatePartial     Key = "state.partial"
	StateUnavailable Key = "state.unavailable"

	StrategyRecordingRule Key = "strategy.recording_rule"
	StrategyRawMetrics    Key = "strategy.raw_metrics"
	StrategyConfigured    Key = "strategy.configured"
	StrategyNone          Key = "strategy.none"

	CapabilityNameNamespaceUsage      Key = "capability.name.namespace_usage"
	CapabilityNameWorkloadUsage       Key = "capability.name.workload_usage"
	CapabilityNameRequestsLimits      Key = "capability.name.requests_limits"
	CapabilityNameHPAHistory          Key = "capability.name.hpa_history"
	CapabilityNameWorkloadsWithoutHPA Key = "capability.name.workloads_without_hpa"
	CapabilityNameIPNodeMapping       Key = "capability.name.ip_node_mapping"
	CapabilityNameNodepoolDiscovery   Key = "capability.name.nodepool_discovery"
	CapabilityNameNodeUtilization     Key = "capability.name.node_utilization"
	CapabilityNameNodeAutoscaling     Key = "capability.name.node_autoscaling"

	CapabilityReasonMetricsNotFound                Key = "capability.reason.METRICS_NOT_FOUND"
	CapabilityReasonCPUUsageMetricNotFound         Key = "capability.reason.CPU_USAGE_METRIC_NOT_FOUND"
	CapabilityReasonMemoryUsageMetricNotFound      Key = "capability.reason.MEMORY_USAGE_METRIC_NOT_FOUND"
	CapabilityReasonUsageMetricsNotFound           Key = "capability.reason.USAGE_METRICS_NOT_FOUND"
	CapabilityReasonWorkloadHistoryInexact         Key = "capability.reason.WORKLOAD_HISTORY_INEXACT"
	CapabilityReasonWorkloadAssociationUnavailable Key = "capability.reason.WORKLOAD_ASSOCIATION_UNAVAILABLE"
	CapabilityReasonRequestsLimitsIncomplete       Key = "capability.reason.REQUESTS_LIMITS_INCOMPLETE"
	CapabilityReasonLegacyMetricsPartial           Key = "capability.reason.LEGACY_METRICS_PARTIAL"
	CapabilityReasonHPAHistoryIncomplete           Key = "capability.reason.HPA_HISTORY_INCOMPLETE"
	CapabilityReasonWorkloadInventoryIncomplete    Key = "capability.reason.WORKLOAD_INVENTORY_INCOMPLETE"
	CapabilityReasonNodeAddressMappingIncomplete   Key = "capability.reason.NODE_ADDRESS_MAPPING_INCOMPLETE"
	CapabilityReasonNodepoolLabelsNotFound         Key = "capability.reason.NODEPOOL_LABELS_NOT_FOUND"
	CapabilityReasonNodeAutoscalingUndetermined    Key = "capability.reason.NODE_AUTOSCALING_UNDETERMINED"
)

// wordingKeys lists every constant above, in the order of the wording
// table, so that Keys can enumerate them.
var wordingKeys = []Key{
	LabelsNote, LabelsInfo, LabelsWarning,
	CoverTitle, CoverClusterLabel, CoverAuthorLabel, CoverIssuedLabel,
	FooterPageLabel,
	FrontOutlineTitle,
	FrontMetadataHeading, FrontMetadataHeaderField, FrontMetadataHeaderValue,
	FrontMetadataEnvironment, FrontMetadataPeriod, FrontMetadataPeriodRange,
	FrontMetadataNamespaces, FrontMetadataExcludedContainers,
	FrontMetadataNoExcludedContainers, FrontMetadataOrganization,
	FrontMetadataClassification,
	FrontLegendHeading, FrontLegendHeaderStatus, FrontLegendHeaderMeaning,
	FrontLegendHealthy, FrontLegendAttention, FrontLegendCritical,
	FrontLegendUnavailable, FrontLegendUndefined,
	FrontQualityHeading, FrontQualityAvailable, FrontQualityPartial,
	FrontQualityUnavailable,
	AppendixHeading, AppendixHeaderCapability, AppendixHeaderCoverage,
	AppendixHeaderSource, AppendixHeaderRemark, AppendixHeaderEvidence,
	AppendixNoRemark, AppendixEmpty,
	StateAvailable, StatePartial, StateUnavailable,
	StrategyRecordingRule, StrategyRawMetrics, StrategyConfigured, StrategyNone,
	CapabilityNameNamespaceUsage, CapabilityNameWorkloadUsage,
	CapabilityNameRequestsLimits, CapabilityNameHPAHistory,
	CapabilityNameWorkloadsWithoutHPA, CapabilityNameIPNodeMapping,
	CapabilityNameNodepoolDiscovery, CapabilityNameNodeUtilization,
	CapabilityNameNodeAutoscaling,
	CapabilityReasonMetricsNotFound, CapabilityReasonCPUUsageMetricNotFound,
	CapabilityReasonMemoryUsageMetricNotFound, CapabilityReasonUsageMetricsNotFound,
	CapabilityReasonWorkloadHistoryInexact, CapabilityReasonWorkloadAssociationUnavailable,
	CapabilityReasonRequestsLimitsIncomplete, CapabilityReasonLegacyMetricsPartial,
	CapabilityReasonHPAHistoryIncomplete, CapabilityReasonWorkloadInventoryIncomplete,
	CapabilityReasonNodeAddressMappingIncomplete, CapabilityReasonNodepoolLabelsNotFound,
	CapabilityReasonNodeAutoscalingUndetermined,
}

// CapabilityName returns the key that names capability name, one of
// discovery.Names.
func CapabilityName(name string) Key { return Key("capability.name." + name) }

// CapabilityReason returns the key of reason code code, one of
// discovery.ReasonCodes.
func CapabilityReason(code string) Key { return Key("capability.reason." + code) }

// CapabilityEvidence returns the key of the evidence sentence of capability
// name, one of discovery.Names, in state, one of discovery.States.
func CapabilityEvidence(name, state string) Key {
	return Key("capability.evidence." + name + "." + state)
}

// State returns the key that names state, one of discovery.States.
func State(state string) Key { return Key("state." + state) }

// Strategy returns the key that names strategy, one of discovery.Strategies.
func Strategy(strategy string) Key { return Key("strategy." + strategy) }

// Keys returns every valid key, sorted: the constants of this file and the
// keys made from the registries of discovery, which is only the capability
// evidence sentences — every other family (capability names, capability
// reasons, states, strategies) already has a row, and so a constant, in the
// wording table.
func Keys() []Key {
	keys := make([]Key, 0, len(wordingKeys)+len(discovery.Names)*len(discovery.States))
	keys = append(keys, wordingKeys...)
	for _, name := range discovery.Names {
		for _, state := range discovery.States {
			keys = append(keys, CapabilityEvidence(name, state))
		}
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })
	return keys
}
