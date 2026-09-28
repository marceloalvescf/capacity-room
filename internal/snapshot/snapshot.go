// Package snapshot defines the stored contract of a report run: one JSON
// document that everything after it, the report document and the PDF,
// reads and nothing else. The same snapshot gives a report in Portuguese or
// in English, so it holds codes and numbers, never a sentence.
//
// This part of the project fills three of its sections: report, scope, and
// quality. Later parts add summary, namespaces, workloads, HPAs, nodes,
// nodepools, findings, and recommendations (spec 8).
package snapshot

// SchemaVersion is the only schema version this package writes, and the
// only one Decode accepts.
const SchemaVersion = "1.0"

// Snapshot is the stored contract of one report run.
type Snapshot struct {
	SchemaVersion string  `json:"schema_version"`
	Report        Report  `json:"report"`
	Scope         Scope   `json:"scope"`
	Quality       Quality `json:"quality"`
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
// capabilities, and when collection started and completed.
type Quality struct {
	Capabilities          []Capability `json:"capabilities"`
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
