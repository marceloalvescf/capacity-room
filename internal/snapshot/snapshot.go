// Package snapshot defines the stored contract of a report run: one JSON
// document that everything after it, the report document and the PDF,
// reads and nothing else. The same snapshot gives a report in Portuguese or
// in English, so it holds codes and numbers, never a sentence.
//
// This part of the project fills eleven of its sections: report, scope,
// quality, summary, namespaces, workloads, HPAs, nodes, nodepools, and
// findings. A later part adds recommendations (spec 8).
package snapshot

// SchemaVersion is the only schema version this package writes, and the
// only one Decode accepts.
const SchemaVersion = "1.0"

// Snapshot is the stored contract of one report run.
type Snapshot struct {
	SchemaVersion string      `json:"schema_version"`
	Report        Report      `json:"report"`
	Scope         Scope       `json:"scope"`
	Quality       Quality     `json:"quality"`
	Summary       Summary     `json:"summary"`
	Namespaces    []Namespace `json:"namespaces"` // sorted, never nil
	Workloads     []Workload  `json:"workloads"`  // sorted, never nil
	HPAs          []HPA       `json:"hpas"`       // sorted, never nil
	Nodes         []Node      `json:"nodes"`      // sorted, never nil
	Nodepools     []NodePool  `json:"nodepools"`  // sorted, never nil
	Findings      []Finding   `json:"findings"`   // sorted, never nil
}

// Report identifies the run and carries the settings that shaped it: the
// requested profile and minimum severity, the organization line, and the
// numbers the analysis compared against. It never carries an address, a
// host name, a token, or a path.
type Report struct {
	ID               string     `json:"id"`
	GeneratorVersion string     `json:"generator_version"`
	EmittedAt        string     `json:"emitted_at"` // RFC 3339, with offset, cut to whole seconds
	Period           string     `json:"period"`
	PeriodStart      string     `json:"period_start"`
	PeriodEnd        string     `json:"period_end"`
	Timezone         string     `json:"timezone"`
	Profile          string     `json:"profile"`
	MinSeverity      string     `json:"min_severity"`
	Organization     string     `json:"organization"`
	Classification   string     `json:"classification"`
	Author           string     `json:"author"`
	Thresholds       Thresholds `json:"thresholds"`
	Settings         Settings   `json:"settings"`
}

// Thresholds holds the seven numeric limits that decide a finding's
// severity.
type Thresholds struct {
	WarningPercent           float64 `json:"warning_percent"`
	CriticalPercent          float64 `json:"critical_percent"`
	LowUtilizationPercent    float64 `json:"low_utilization_percent"`
	AutoscalerNearMaxPercent float64 `json:"autoscaler_near_max_percent"`
	NodepoolSpreadPoints     float64 `json:"nodepool_spread_points"`
	BurstyGapPercent         float64 `json:"bursty_gap_percent"`
	HPAReplicaDivergence     float64 `json:"hpa_replica_divergence"`
}

// Settings holds the non-secret settings of the run: never an address, a
// token, or a path.
type Settings struct {
	EndpointType    string  `json:"endpoint_type"` // the kind that answered: prometheus or grafana, never auto
	TLSVerify       bool    `json:"tls_verify"`
	TimeoutSeconds  float64 `json:"timeout_seconds"`
	NodeAutoscaling string  `json:"node_autoscaling"`
	NodepoolLabel   string  `json:"nodepool_label"`
}

// Scope is the stored form of the resolved cluster scope: which values of
// the cluster label are visible, whether a matcher is added, and the
// namespaces and containers the analysis covers or excludes. It looks like
// discovery.Scope, and is deliberately a type of its own: a change to
// discovery must never silently change a stored file.
type Scope struct {
	ClusterName        string   `json:"cluster_name"`
	ClusterLabel       string   `json:"cluster_label"`
	FilterMode         string   `json:"filter_mode"`
	MatcherEnabled     bool     `json:"matcher_enabled"`
	Confidence         string   `json:"confidence"`
	DiscoveredValues   []string `json:"discovered_values"`   // never nil
	Namespaces         []string `json:"namespaces"`          // sorted, never nil
	ExcludedContainers []string `json:"excluded_containers"` // sorted, never nil
}

// Quality holds what the run could tell about its own coverage: the nine
// capabilities, the warnings and notices the collectors raised, and when
// collection started and completed.
type Quality struct {
	Capabilities          []Capability `json:"capabilities"`
	Warnings              []Message    `json:"warnings"` // never nil
	Notices               []Message    `json:"notices"`  // never nil
	CollectionStartedAt   string       `json:"collection_started_at"`
	CollectionCompletedAt string       `json:"collection_completed_at"`
}

// Capability is the stored form of one of the nine abilities the report
// needs. It looks like discovery.Capability, and is deliberately a type of
// its own, for the same reason Scope is.
type Capability struct {
	Name     string   `json:"name"`
	State    string   `json:"state"`
	Strategy string   `json:"strategy"`
	Evidence []string `json:"evidence"` // never nil
	Reasons  []string `json:"reasons"`  // reason codes, never nil
}

// Message is a warning or a notice: a code and its named numbers, spec 8.2.
type Message struct {
	Code   string             `json:"code"`
	Params map[string]float64 `json:"params"` // never nil
}

// Statistics is a series reduced to four numbers, spec 8.1. A number is nil
// when the state is not value.
type Statistics struct {
	State   string   `json:"state"`
	Average *float64 `json:"average"`
	Minimum *float64 `json:"minimum"`
	Peak    *float64 `json:"peak"`
	Current *float64 `json:"current"`
}

// Namespace is the row of one namespace, spec 8, "namespaces".
type Namespace struct {
	Name                 string     `json:"name"`
	Pods                 *float64   `json:"pods"` // nil when unknown
	Approximate          bool       `json:"approximate"`
	CPUUsage             Statistics `json:"cpu_usage"`
	MemoryUsage          Statistics `json:"memory_usage"`
	CPURequests          Statistics `json:"cpu_requests"`
	MemoryRequests       Statistics `json:"memory_requests"`
	CPULimits            Statistics `json:"cpu_limits"`
	MemoryLimits         Statistics `json:"memory_limits"`
	CPURequestPercent    Statistics `json:"cpu_request_percent"`
	MemoryRequestPercent Statistics `json:"memory_request_percent"`
	CPULimitPercent      Statistics `json:"cpu_limit_percent"`
	MemoryLimitPercent   Statistics `json:"memory_limit_percent"`
}

// The three values of Workload.HPACoverage.
const (
	CoverageCovered   = "covered"   // an HPA targets the workload
	CoverageUncovered = "uncovered" // the coverage inventory lists it and no HPA targets it
	CoverageUnknown   = "unknown"   // the run could not tell
)

// validCoverages lists the values of Workload.HPACoverage.
var validCoverages = []string{CoverageCovered, CoverageUncovered, CoverageUnknown}

// Summary holds the counts of the run and its autoscaling, spec 8.
type Summary struct {
	Namespaces          float64     `json:"namespaces"`
	Workloads           float64     `json:"workloads"` // rows with observed usage only
	HPAs                float64     `json:"hpas"`
	Nodes               float64     `json:"nodes"`
	Nodepools           float64     `json:"nodepools"`
	Findings            float64     `json:"findings"`
	Healthy             float64     `json:"healthy"`
	Attention           float64     `json:"attention"`
	Critical            float64     `json:"critical"`
	WorkloadsWithoutHPA float64     `json:"workloads_without_hpa"` // rows whose hpa_coverage is uncovered
	Autoscaling         Autoscaling `json:"autoscaling"`
}

// Autoscaling is what the run learned about the autoscaling of the nodes.
// A count is nil when the run did not learn it.
type Autoscaling struct {
	Status            string   `json:"status"` // enabled, disabled, unknown
	Source            string   `json:"source"` // configured, raw_metrics, none
	Confidence        string   `json:"confidence"`
	ManagedNodeGroups *float64 `json:"managed_node_groups"`
	CurrentNodes      *float64 `json:"current_nodes"`
	MinNodes          *float64 `json:"min_nodes"`
	MaxNodes          *float64 `json:"max_nodes"`
	ScaleEvents       *float64 `json:"scale_events"`
	UnschedulablePods *float64 `json:"unschedulable_pods"`
}

// Workload is the row of one workload, spec 8, "workloads". A row whose
// ObservedUsage is false names a workload that only the coverage inventory
// lists: it has no pods and no number.
type Workload struct {
	Namespace            string     `json:"namespace"`
	Kind                 string     `json:"kind"`
	Name                 string     `json:"name"`
	ObservedUsage        bool       `json:"observed_usage"`
	HPACoverage          string     `json:"hpa_coverage"`
	Pods                 *float64   `json:"pods"` // nil when unknown
	Approximate          bool       `json:"approximate"`
	CPUUsage             Statistics `json:"cpu_usage"`
	MemoryUsage          Statistics `json:"memory_usage"`
	CPURequests          Statistics `json:"cpu_requests"`
	MemoryRequests       Statistics `json:"memory_requests"`
	CPULimits            Statistics `json:"cpu_limits"`
	MemoryLimits         Statistics `json:"memory_limits"`
	CPURequestPercent    Statistics `json:"cpu_request_percent"`
	MemoryRequestPercent Statistics `json:"memory_request_percent"`
	CPULimitPercent      Statistics `json:"cpu_limit_percent"`
	MemoryLimitPercent   Statistics `json:"memory_limit_percent"`
}

// HPA is the row of one horizontal pod autoscaler, spec 8, "hpas". Its
// target is empty when the run could not tell it.
type HPA struct {
	Namespace  string     `json:"namespace"`
	Name       string     `json:"name"`
	TargetKind string     `json:"target_kind"`
	TargetName string     `json:"target_name"`
	SpecMin    *float64   `json:"spec_min"`
	SpecMax    *float64   `json:"spec_max"`
	Current    Statistics `json:"current"`
	Desired    Statistics `json:"desired"`
}

// Node is the row of one node, spec 8, "nodes".
type Node struct {
	Name                 string     `json:"name"`
	Pool                 string     `json:"pool"`
	PoolConfidence       string     `json:"pool_confidence"`
	CPUCapacity          *float64   `json:"cpu_capacity"`
	MemoryCapacity       *float64   `json:"memory_capacity"`
	CPUAllocatable       *float64   `json:"cpu_allocatable"`
	MemoryAllocatable    *float64   `json:"memory_allocatable"`
	CPUUsagePercent      Statistics `json:"cpu_usage_percent"`
	MemoryUsagePercent   Statistics `json:"memory_usage_percent"`
	CPURequestPercent    *float64   `json:"cpu_request_percent"`
	MemoryRequestPercent *float64   `json:"memory_request_percent"`
	Ready                *bool      `json:"ready"`
	Schedulable          *bool      `json:"schedulable"`
}

// NodePool is the row of one node pool, spec 8, "nodepools".
type NodePool struct {
	Name       string   `json:"name"`
	Confidence string   `json:"confidence"` // the lowest of its nodes
	Nodes      []string `json:"nodes"`      // names, sorted, never nil
}

// Value is the value of an evidence as the snapshot writes it: a JSON
// number, boolean, or string, one of the three.
type Value struct {
	Kind    string // "number", "boolean", "text"
	Number  float64
	Boolean bool
	Text    string
}

// Evidence is one named value of a finding.
type Evidence struct {
	Name  string `json:"name"`
	Value Value  `json:"value"`
}

// Finding is one judgment of the analysis, spec 7.1. It carries no
// sentence.
type Finding struct {
	Code       string     `json:"code"`
	Severity   string     `json:"severity"`
	ScopeType  string     `json:"scope_type"`
	ScopeKind  string     `json:"scope_kind"`
	ScopeName  string     `json:"scope_name"`
	Evidence   []Evidence `json:"evidence"` // never nil
	Confidence string     `json:"confidence"`
}
