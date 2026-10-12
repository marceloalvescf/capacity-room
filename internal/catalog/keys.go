package catalog

import (
	"slices"
	"sort"

	"github.com/marceloalvescf/k8s-capacity-report/internal/analysis"
	"github.com/marceloalvescf/k8s-capacity-report/internal/collect"
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

	SectionsNamespaceLabel Key = "sections.namespace_label"

	SectionsNamespacesHeading           Key = "sections.namespaces.heading"
	SectionsNamespacesEmpty             Key = "sections.namespaces.empty"
	SectionsNamespacesBelowMinimum      Key = "sections.namespaces.below_minimum"
	SectionsNamespacesHeaderResource    Key = "sections.namespaces.header_resource"
	SectionsNamespacesHeaderAverage     Key = "sections.namespaces.header_average"
	SectionsNamespacesHeaderPeak        Key = "sections.namespaces.header_peak"
	SectionsNamespacesHeaderRequest     Key = "sections.namespaces.header_request"
	SectionsNamespacesHeaderLimit       Key = "sections.namespaces.header_limit"
	SectionsNamespacesHeaderPeakRequest Key = "sections.namespaces.header_peak_request"
	SectionsNamespacesHeaderPeakLimit   Key = "sections.namespaces.header_peak_limit"
	SectionsNamespacesRowCPU            Key = "sections.namespaces.row_cpu"
	SectionsNamespacesRowMemory         Key = "sections.namespaces.row_memory"

	SectionsWorkloadsHeading             Key = "sections.workloads.heading"
	SectionsWorkloadsEmpty               Key = "sections.workloads.empty"
	SectionsWorkloadsBelowMinimum        Key = "sections.workloads.below_minimum"
	SectionsWorkloadsHeaderWorkload      Key = "sections.workloads.header_workload"
	SectionsWorkloadsHeaderPods          Key = "sections.workloads.header_pods"
	SectionsWorkloadsHeaderCPUAverage    Key = "sections.workloads.header_cpu_average"
	SectionsWorkloadsHeaderCPUPeak       Key = "sections.workloads.header_cpu_peak"
	SectionsWorkloadsHeaderCPURequest    Key = "sections.workloads.header_cpu_request"
	SectionsWorkloadsHeaderCPULimit      Key = "sections.workloads.header_cpu_limit"
	SectionsWorkloadsHeaderMemoryAverage Key = "sections.workloads.header_memory_average"
	SectionsWorkloadsHeaderMemoryPeak    Key = "sections.workloads.header_memory_peak"
	SectionsWorkloadsHeaderMemoryRequest Key = "sections.workloads.header_memory_request"
	SectionsWorkloadsHeaderMemoryLimit   Key = "sections.workloads.header_memory_limit"

	PlaceholderUnavailable Key = "placeholder.unavailable"
	PlaceholderNoBase      Key = "placeholder.no_base"

	NotesHeading      Key = "notes.heading"
	NotesHeaderRemark Key = "notes.header_remark"

	SectionsHPAsHeading                   Key = "sections.hpas.heading"
	SectionsHPAsIntro                     Key = "sections.hpas.intro"
	SectionsHPAsObservedHeading           Key = "sections.hpas.observed_heading"
	SectionsHPAsUncoveredHeading          Key = "sections.hpas.uncovered_heading"
	SectionsHPAsHeaderHPA                 Key = "sections.hpas.header_hpa"
	SectionsHPAsHeaderSpecMin             Key = "sections.hpas.header_spec_min"
	SectionsHPAsHeaderSpecMax             Key = "sections.hpas.header_spec_max"
	SectionsHPAsHeaderCurrentAverage      Key = "sections.hpas.header_current_average"
	SectionsHPAsHeaderMinReached          Key = "sections.hpas.header_min_reached"
	SectionsHPAsHeaderMaxReached          Key = "sections.hpas.header_max_reached"
	SectionsHPAsHeaderSituation           Key = "sections.hpas.header_situation"
	SectionsHPAsHeaderKind                Key = "sections.hpas.header_kind"
	SectionsHPAsHeaderWorkload            Key = "sections.hpas.header_workload"
	SectionsHPAsHeaderPods                Key = "sections.hpas.header_pods"
	SectionsHPAsSituationMaxReached       Key = "sections.hpas.situation_max_reached"
	SectionsHPAsSituationPinned           Key = "sections.hpas.situation_pinned"
	SectionsHPAsSituationBelowMax         Key = "sections.hpas.situation_below_max"
	SectionsHPAsSituationNoHPA            Key = "sections.hpas.situation_no_hpa"
	SectionsHPAsSituationSpecificStrategy Key = "sections.hpas.situation_specific_strategy"
	SectionsHPAsEmptyNamespace            Key = "sections.hpas.empty_namespace"
	SectionsHPAsEmptyNamespacePartial     Key = "sections.hpas.empty_namespace_partial"
	SectionsHPAsNoteIncompleteTitle       Key = "sections.hpas.note_incomplete_title"
	SectionsHPAsNoteIncomplete            Key = "sections.hpas.note_incomplete"
	SectionsHPAsNotePartialTitle          Key = "sections.hpas.note_partial_title"
	SectionsHPAsNotePartial               Key = "sections.hpas.note_partial"

	SectionsNodepoolsHeading                 Key = "sections.nodepools.heading"
	SectionsNodepoolsSummaryHeading          Key = "sections.nodepools.summary_heading"
	SectionsNodepoolsProfileHeading          Key = "sections.nodepools.profile_heading"
	SectionsNodepoolsDetailHeading           Key = "sections.nodepools.detail_heading"
	SectionsNodepoolsPoolLabel               Key = "sections.nodepools.pool_label"
	SectionsNodepoolsHeaderPool              Key = "sections.nodepools.header_pool"
	SectionsNodepoolsHeaderNodes             Key = "sections.nodepools.header_nodes"
	SectionsNodepoolsHeaderCPUAverage        Key = "sections.nodepools.header_cpu_average"
	SectionsNodepoolsHeaderCPUPeak           Key = "sections.nodepools.header_cpu_peak"
	SectionsNodepoolsHeaderMemoryAverage     Key = "sections.nodepools.header_memory_average"
	SectionsNodepoolsHeaderMemoryPeak        Key = "sections.nodepools.header_memory_peak"
	SectionsNodepoolsHeaderConfidence        Key = "sections.nodepools.header_confidence"
	SectionsNodepoolsHeaderCPUAllocatable    Key = "sections.nodepools.header_cpu_allocatable"
	SectionsNodepoolsHeaderCPUUsed           Key = "sections.nodepools.header_cpu_used"
	SectionsNodepoolsHeaderCPURequested      Key = "sections.nodepools.header_cpu_requested"
	SectionsNodepoolsHeaderMemoryAllocatable Key = "sections.nodepools.header_memory_allocatable"
	SectionsNodepoolsHeaderMemoryUsed        Key = "sections.nodepools.header_memory_used"
	SectionsNodepoolsHeaderMemoryRequested   Key = "sections.nodepools.header_memory_requested"
	SectionsNodepoolsHeaderNode              Key = "sections.nodepools.header_node"
	SectionsNodepoolsHeaderCPUAvgPeak        Key = "sections.nodepools.header_cpu_avg_peak"
	SectionsNodepoolsHeaderMemoryAvgPeak     Key = "sections.nodepools.header_memory_avg_peak"
	SectionsNodepoolsHeaderReason            Key = "sections.nodepools.header_reason"
	SectionsNodepoolsHeaderState             Key = "sections.nodepools.header_state"
	SectionsNodepoolsEmptySummary            Key = "sections.nodepools.empty_summary"
	SectionsNodepoolsEmptyProfile            Key = "sections.nodepools.empty_profile"
	SectionsNodepoolsEmptyDetail             Key = "sections.nodepools.empty_detail"
	SectionsNodepoolsBelowMinimum            Key = "sections.nodepools.below_minimum"
	SectionsNodepoolsNoFinding               Key = "sections.nodepools.no_finding"

	NodeStateReady         Key = "node_state.ready"
	NodeStateNotReady      Key = "node_state.not_ready"
	NodeStateSchedulable   Key = "node_state.schedulable"
	NodeStateUnschedulable Key = "node_state.unschedulable"

	PlaceholderUndetermined Key = "placeholder.undetermined"
	PlaceholderEmptyCell    Key = "placeholder.empty_cell"

	ValueYes Key = "value.yes"
	ValueNo  Key = "value.no"

	SectionsExecutiveHeading                Key = "sections.executive.heading"
	SectionsExecutiveIntro                  Key = "sections.executive.intro"
	SectionsExecutiveCriticalHeading        Key = "sections.executive.critical_heading"
	SectionsExecutiveCriticalEmpty          Key = "sections.executive.critical_empty"
	SectionsExecutiveAttentionHeading       Key = "sections.executive.attention_heading"
	SectionsExecutiveAttentionEmpty         Key = "sections.executive.attention_empty"
	SectionsExecutiveHeaderNamespace        Key = "sections.executive.header_namespace"
	SectionsExecutiveHeaderWorkload         Key = "sections.executive.header_workload"
	SectionsExecutiveHeaderPods             Key = "sections.executive.header_pods"
	SectionsExecutiveHeaderReason           Key = "sections.executive.header_reason"
	SectionsExecutiveNamespacesHeading      Key = "sections.executive.namespaces_heading"
	SectionsExecutiveNamespacesEmpty        Key = "sections.executive.namespaces_empty"
	SectionsExecutiveHeaderCPULimit         Key = "sections.executive.header_cpu_limit"
	SectionsExecutiveHeaderMemoryLimit      Key = "sections.executive.header_memory_limit"
	SectionsExecutiveHeaderStatus           Key = "sections.executive.header_status"
	SectionsExecutiveHeaderPriority         Key = "sections.executive.header_priority"
	SectionsExecutiveScalingHeading         Key = "sections.executive.scaling_heading"
	SectionsExecutiveScalingEmpty           Key = "sections.executive.scaling_empty"
	SectionsExecutiveHeaderKind             Key = "sections.executive.header_kind"
	SectionsExecutiveHeaderReplicas         Key = "sections.executive.header_replicas"
	SectionsExecutiveHeaderSituation        Key = "sections.executive.header_situation"
	SectionsExecutiveReplicasObserved       Key = "sections.executive.replicas_observed"
	SectionsExecutiveActiveReplicasSingular Key = "sections.executive.active_replicas.singular"
	SectionsExecutiveActiveReplicasPlural   Key = "sections.executive.active_replicas.plural"
	SectionsExecutiveSituationHPAMax        Key = "sections.executive.situation_hpa_max"
	SectionsExecutiveTargetUnknown          Key = "sections.executive.target_unknown"
	SectionsExecutiveNodepoolsHeading       Key = "sections.executive.nodepools_heading"
	SectionsExecutiveHeaderPool             Key = "sections.executive.header_pool"
	SectionsExecutiveHeaderPoolSituation    Key = "sections.executive.header_pool_situation"
	SectionsExecutiveHeaderRemark           Key = "sections.executive.header_remark"
	SectionsExecutivePoolStable             Key = "sections.executive.pool_stable"
	SectionsExecutivePoolAttention          Key = "sections.executive.pool_attention"
	SectionsExecutivePoolCritical           Key = "sections.executive.pool_critical"
	SectionsExecutiveActionsHeading         Key = "sections.executive.actions_heading"
	SectionsExecutiveActionsEmpty           Key = "sections.executive.actions_empty"
	SectionsExecutiveHeaderOrder            Key = "sections.executive.header_order"
	SectionsExecutiveHeaderAction           Key = "sections.executive.header_action"
	SectionsExecutiveHeaderScope            Key = "sections.executive.header_scope"
	SectionsExecutiveLimitationsHeading     Key = "sections.executive.limitations_heading"
	SectionsExecutiveLimitationsEmpty       Key = "sections.executive.limitations_empty"
	SectionsExecutiveHeaderCapability       Key = "sections.executive.header_capability"
	SectionsExecutiveHeaderCoverage         Key = "sections.executive.header_coverage"
	SectionsExecutiveInterpretation         Key = "sections.executive.interpretation"
	SectionsExecutiveReasonAllMissing       Key = "sections.executive.reason_all_missing"
	SectionsExecutiveRiskStatusBoth         Key = "sections.executive.risk_status_both"

	PeriodDaySingular    Key = "period.day.singular"
	PeriodDayPlural      Key = "period.day.plural"
	PeriodHourSingular   Key = "period.hour.singular"
	PeriodHourPlural     Key = "period.hour.plural"
	PeriodMinuteSingular Key = "period.minute.singular"
	PeriodMinutePlural   Key = "period.minute.plural"
	PeriodSecondSingular Key = "period.second.singular"
	PeriodSecondPlural   Key = "period.second.plural"
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
	SectionsNamespaceLabel,
	SectionsNamespacesHeading, SectionsNamespacesEmpty, SectionsNamespacesBelowMinimum, SectionsNamespacesHeaderResource,
	SectionsNamespacesHeaderAverage, SectionsNamespacesHeaderPeak, SectionsNamespacesHeaderRequest,
	SectionsNamespacesHeaderLimit, SectionsNamespacesHeaderPeakRequest, SectionsNamespacesHeaderPeakLimit,
	SectionsNamespacesRowCPU, SectionsNamespacesRowMemory,
	SectionsWorkloadsHeading, SectionsWorkloadsEmpty, SectionsWorkloadsBelowMinimum, SectionsWorkloadsHeaderWorkload,
	SectionsWorkloadsHeaderPods, SectionsWorkloadsHeaderCPUAverage, SectionsWorkloadsHeaderCPUPeak,
	SectionsWorkloadsHeaderCPURequest, SectionsWorkloadsHeaderCPULimit, SectionsWorkloadsHeaderMemoryAverage,
	SectionsWorkloadsHeaderMemoryPeak, SectionsWorkloadsHeaderMemoryRequest, SectionsWorkloadsHeaderMemoryLimit,
	PlaceholderUnavailable, PlaceholderNoBase,
	SectionsHPAsHeading, SectionsHPAsIntro, SectionsHPAsObservedHeading, SectionsHPAsUncoveredHeading,
	SectionsHPAsHeaderHPA, SectionsHPAsHeaderSpecMin, SectionsHPAsHeaderSpecMax,
	SectionsHPAsHeaderCurrentAverage, SectionsHPAsHeaderMinReached, SectionsHPAsHeaderMaxReached,
	SectionsHPAsHeaderSituation, SectionsHPAsHeaderKind, SectionsHPAsHeaderWorkload,
	SectionsHPAsHeaderPods, SectionsHPAsSituationMaxReached, SectionsHPAsSituationPinned,
	SectionsHPAsSituationBelowMax, SectionsHPAsSituationNoHPA, SectionsHPAsSituationSpecificStrategy,
	SectionsHPAsEmptyNamespace, SectionsHPAsEmptyNamespacePartial, SectionsHPAsNoteIncompleteTitle,
	SectionsHPAsNoteIncomplete, SectionsHPAsNotePartialTitle, SectionsHPAsNotePartial,
	SectionsNodepoolsHeading, SectionsNodepoolsSummaryHeading, SectionsNodepoolsProfileHeading,
	SectionsNodepoolsDetailHeading, SectionsNodepoolsPoolLabel, SectionsNodepoolsHeaderPool,
	SectionsNodepoolsHeaderNodes, SectionsNodepoolsHeaderCPUAverage, SectionsNodepoolsHeaderCPUPeak,
	SectionsNodepoolsHeaderMemoryAverage, SectionsNodepoolsHeaderMemoryPeak,
	SectionsNodepoolsHeaderConfidence, SectionsNodepoolsHeaderCPUAllocatable,
	SectionsNodepoolsHeaderCPUUsed, SectionsNodepoolsHeaderCPURequested,
	SectionsNodepoolsHeaderMemoryAllocatable, SectionsNodepoolsHeaderMemoryUsed,
	SectionsNodepoolsHeaderMemoryRequested, SectionsNodepoolsHeaderNode,
	SectionsNodepoolsHeaderCPUAvgPeak, SectionsNodepoolsHeaderMemoryAvgPeak,
	SectionsNodepoolsHeaderReason, SectionsNodepoolsHeaderState, SectionsNodepoolsEmptySummary,
	SectionsNodepoolsEmptyProfile, SectionsNodepoolsEmptyDetail, SectionsNodepoolsBelowMinimum, SectionsNodepoolsNoFinding,
	NodeStateReady, NodeStateNotReady, NodeStateSchedulable, NodeStateUnschedulable,
	PlaceholderUndetermined, PlaceholderEmptyCell,
	ValueYes, ValueNo,
	NotesHeading, NotesHeaderRemark,
	SectionsExecutiveHeading, SectionsExecutiveIntro, SectionsExecutiveCriticalHeading,
	SectionsExecutiveCriticalEmpty, SectionsExecutiveAttentionHeading,
	SectionsExecutiveAttentionEmpty, SectionsExecutiveHeaderNamespace,
	SectionsExecutiveHeaderWorkload, SectionsExecutiveHeaderPods,
	SectionsExecutiveHeaderReason, SectionsExecutiveNamespacesHeading,
	SectionsExecutiveNamespacesEmpty, SectionsExecutiveHeaderCPULimit,
	SectionsExecutiveHeaderMemoryLimit, SectionsExecutiveHeaderStatus,
	SectionsExecutiveHeaderPriority, SectionsExecutiveScalingHeading,
	SectionsExecutiveScalingEmpty, SectionsExecutiveHeaderKind,
	SectionsExecutiveHeaderReplicas, SectionsExecutiveHeaderSituation,
	SectionsExecutiveReplicasObserved, SectionsExecutiveActiveReplicasSingular,
	SectionsExecutiveActiveReplicasPlural, SectionsExecutiveSituationHPAMax,
	SectionsExecutiveTargetUnknown, SectionsExecutiveNodepoolsHeading,
	SectionsExecutiveHeaderPool, SectionsExecutiveHeaderPoolSituation,
	SectionsExecutiveHeaderRemark, SectionsExecutivePoolStable,
	SectionsExecutivePoolAttention, SectionsExecutivePoolCritical,
	SectionsExecutiveActionsHeading, SectionsExecutiveActionsEmpty,
	SectionsExecutiveHeaderOrder, SectionsExecutiveHeaderAction,
	SectionsExecutiveHeaderScope,
	SectionsExecutiveLimitationsHeading, SectionsExecutiveLimitationsEmpty,
	SectionsExecutiveHeaderCapability, SectionsExecutiveHeaderCoverage,
	SectionsExecutiveInterpretation, SectionsExecutiveReasonAllMissing,
	SectionsExecutiveRiskStatusBoth,
	PeriodDaySingular, PeriodDayPlural, PeriodHourSingular, PeriodHourPlural,
	PeriodMinuteSingular, PeriodMinutePlural, PeriodSecondSingular, PeriodSecondPlural,
}

// Confidences, ReasonFamilies, and ReasonResources are the values the key
// functions Confidence and Reason take.
var (
	Confidences     = []string{"exact", "inferred", "unknown"}
	ReasonFamilies  = []string{"usage_peak", "usage_average", "request_high", "request_peak", "request_average", "limit_peak", "limit_average", "request_missing", "limit_missing", "bursty"}
	ReasonResources = []string{"cpu", "memory"}
)

// Priorities lists the priorities the executive body prints: attention and
// critical. RiskSeverities lists the two severities a namespace at risk
// counts. ScopeSummaries lists the kinds of a scope list of more than
// three names.
var (
	Priorities     = []string{"attention", "critical"}
	RiskSeverities = []string{"critical", "attention"}
	ScopeSummaries = []string{"namespace", "workload", "hpa", "deployment"}
)

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

// Warning returns the key of warning code, one of collect.WarningCodes.
func Warning(code string) Key { return Key("warning." + code) }

// Notice returns the key of notice code, one of collect.NoticeCodes, for
// a count: the entry "singular" for a count of 1, "plural" for any other.
func Notice(code string, count float64) Key {
	if count == 1 {
		return Key("notice." + code + ".singular")
	}
	return Key("notice." + code + ".plural")
}

// Confidence returns the key that names confidence, one of Confidences.
func Confidence(confidence string) Key { return Key("confidence." + confidence) }

// Reason returns the key of the reason of a node finding for family, one of
// ReasonFamilies, and resource, one of ReasonResources.
func Reason(family, resource string) Key { return Key("reason." + family + "." + resource) }

// Priority returns the key of the label of priority severity, one of
// Priorities: priority.<severity>.
func Priority(severity string) Key { return Key("priority." + severity) }

// RiskCount returns the key of the count of workloads at severity, one of
// RiskSeverities: risk_status.<severity>.singular for a count of 1,
// risk_status.<severity>.plural for any other count.
func RiskCount(severity string, count float64) Key {
	if count == 1 {
		return Key("risk_status." + severity + ".singular")
	}
	return Key("risk_status." + severity + ".plural")
}

// ScopeSummary returns the key of the summary of a scope list of kind, one
// of ScopeSummaries: scope_summary.<kind>.
func ScopeSummary(kind string) Key { return Key("scope_summary." + kind) }

// ObservationFamilies lists the families that carry an observation: the
// families of the codes of the node and nodepool scopes.
var ObservationFamilies = []string{"node_usage", "node_request_high", "node_overrequested", "node_low_utilization", "nodepool_uneven", "nodepool_unknown"}

// findingKey returns finding.<family>[.<resource>].<part>.
func findingKey(family, resource, part string) Key {
	if resource == "" {
		return Key("finding." + family + "." + part)
	}
	return Key("finding." + family + "." + resource + "." + part)
}

// FindingSummary returns the key of the summary of the findings of
// family, one of the families of analysis.Registry, for resource, the
// "resource" parameter of the definition, or "" when it has none:
// finding.<family>.<resource>.summary, or finding.<family>.summary.
func FindingSummary(family, resource string) Key { return findingKey(family, resource, "summary") }

// FindingAction returns the key of the action of the same family and
// resource: finding.<family>[.<resource>].action.
func FindingAction(family, resource string) Key { return findingKey(family, resource, "action") }

// FindingObservation returns the key of the short observation the
// executive pool table prints for the family, one of ObservationFamilies,
// and resource: finding.<family>[.<resource>].observation.
func FindingObservation(family, resource string) Key {
	return findingKey(family, resource, "observation")
}

// FindingPair is a family of findings and the resource of its definition,
// empty when the family has none.
type FindingPair struct{ Family, Resource string }

// FindingPairs returns the distinct pairs of family and resource of
// analysis.Registry, in the order of the registry.
func FindingPairs() []FindingPair {
	var pairs []FindingPair
	for _, definition := range analysis.Registry {
		pair := FindingPair{definition.Family, definition.Params["resource"]}
		if !slices.Contains(pairs, pair) {
			pairs = append(pairs, pair)
		}
	}
	return pairs
}

// Keys returns every valid key, sorted: the constants of this file and the
// keys made from the registries of discovery, collect, and analysis. The
// registries of discovery give only the capability evidence sentences — every other
// family (capability names, capability reasons, states, strategies) already
// has a row, and so a constant, in the wording table. The registries of
// collect give both families of the second wording table: every warning
// code, and both entries of every notice code. Confidence and Reason give
// the confidences and the node finding reasons, and the pairs of analysis
// give, for each pair, the summary, the action, and, for a family of
// ObservationFamilies, the observation.
func Keys() []Key {
	keys := make([]Key, 0, len(wordingKeys)+len(discovery.Names)*len(discovery.States)+len(collect.WarningCodes)+len(collect.NoticeCodes)*2+len(Confidences)+len(ReasonFamilies)*len(ReasonResources))
	keys = append(keys, wordingKeys...)
	for _, name := range discovery.Names {
		for _, state := range discovery.States {
			keys = append(keys, CapabilityEvidence(name, state))
		}
	}
	for _, code := range collect.WarningCodes {
		keys = append(keys, Warning(code))
	}
	for _, code := range collect.NoticeCodes {
		keys = append(keys, Notice(code, 1))
		keys = append(keys, Notice(code, 2))
	}
	for _, confidence := range Confidences {
		keys = append(keys, Confidence(confidence))
	}
	for _, family := range ReasonFamilies {
		for _, resource := range ReasonResources {
			keys = append(keys, Reason(family, resource))
		}
	}
	for _, pair := range FindingPairs() {
		keys = append(keys, FindingSummary(pair.Family, pair.Resource), FindingAction(pair.Family, pair.Resource))
		if slices.Contains(ObservationFamilies, pair.Family) {
			keys = append(keys, FindingObservation(pair.Family, pair.Resource))
		}
	}
	for _, severity := range Priorities {
		keys = append(keys, Priority(severity))
	}
	for _, severity := range RiskSeverities {
		keys = append(keys, RiskCount(severity, 1), RiskCount(severity, 2))
	}
	for _, kind := range ScopeSummaries {
		keys = append(keys, ScopeSummary(kind))
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })
	return keys
}
