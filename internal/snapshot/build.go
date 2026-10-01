package snapshot

import (
	"sort"
	"time"

	"github.com/marceloalvescf/k8s-capacity-report/internal/analysis"
	"github.com/marceloalvescf/k8s-capacity-report/internal/collect"
	"github.com/marceloalvescf/k8s-capacity-report/internal/config"
	"github.com/marceloalvescf/k8s-capacity-report/internal/discovery"
)

// Input is everything Build needs to assemble a Snapshot. Config supplies
// the settings the snapshot is allowed to keep; the secrets of Config
// (the metrics URL, the token environment name, the CA bundle path, the
// output directory) are never read.
type Input struct {
	ID                  string
	GeneratorVersion    string
	EmittedAt           time.Time
	Config              config.Config
	EndpointKind        string // the kind that answered: "prometheus" or "grafana"
	Scope               discovery.Scope
	Capabilities        []discovery.Capability
	CollectionStarted   time.Time
	CollectionCompleted time.Time
	Window              collect.Window
	Collected           collect.Collected
	Findings            []analysis.Finding
}

// Build makes a snapshot from in and checks it.
func Build(in Input) (Snapshot, error) {
	loc, err := in.Config.Analysis.Location()
	if err != nil {
		return Snapshot{}, err
	}

	emittedAt := formatTime(in.EmittedAt, loc)

	namespaces := buildNamespaces(in.Collected.Namespaces.Rows)
	workloads := buildWorkloads(in.Collected.Workloads.Rows, in.Collected.Coverage.Rows, in.Collected.HPAs.Rows)
	hpas := buildHPAs(in.Collected.HPAs.Rows)
	nodes := buildNodes(in.Collected.Nodes.Rows)
	nodepools := buildNodepools(collect.Pools(in.Collected.Nodes.Rows))
	findings := buildFindings(in.Findings)

	snap := Snapshot{
		SchemaVersion: SchemaVersion,
		Report: Report{
			ID:               in.ID,
			GeneratorVersion: in.GeneratorVersion,
			EmittedAt:        emittedAt,
			Period:           in.Config.Analysis.Period,
			PeriodStart:      formatTime(in.Window.Start, loc),
			PeriodEnd:        formatTime(in.Window.End, loc),
			Timezone:         in.Config.Analysis.Timezone,
			Profile:          in.Config.Report.Profile,
			MinSeverity:      in.Config.Report.MinSeverity,
			Organization:     in.Config.Report.Organization,
			Classification:   in.Config.Report.Classification,
			Author:           in.Config.Report.Author,
			Thresholds: Thresholds{
				WarningPercent:           in.Config.Thresholds.WarningPercent,
				CriticalPercent:          in.Config.Thresholds.CriticalPercent,
				LowUtilizationPercent:    in.Config.Thresholds.LowUtilizationPercent,
				AutoscalerNearMaxPercent: in.Config.Thresholds.AutoscalerNearMaxPercent,
				NodepoolSpreadPoints:     in.Config.Thresholds.NodepoolSpreadPoints,
				BurstyGapPercent:         in.Config.Thresholds.BurstyGapPercent,
				HPAReplicaDivergence:     in.Config.Thresholds.HPAReplicaDivergence,
			},
			Settings: Settings{
				EndpointType:    in.EndpointKind,
				TLSVerify:       in.Config.Metrics.TLSVerify,
				TimeoutSeconds:  in.Config.Metrics.TimeoutSeconds,
				NodeAutoscaling: in.Config.Analysis.NodeAutoscaling,
				NodepoolLabel:   in.Config.Cluster.NodepoolLabel,
			},
		},
		Scope: Scope{
			ClusterName:        in.Scope.ClusterName,
			ClusterLabel:       in.Scope.ClusterLabel,
			FilterMode:         in.Scope.FilterMode,
			MatcherEnabled:     in.Scope.MatcherEnabled,
			Confidence:         in.Scope.Confidence,
			DiscoveredValues:   sortedCopy(in.Scope.DiscoveredValues),
			Namespaces:         sortedCopy(in.Config.Cluster.Namespaces),
			ExcludedContainers: sortedCopy(in.Config.Analysis.ExcludedContainers),
		},
		Quality: Quality{
			Capabilities:          buildCapabilities(in.Capabilities),
			Warnings:              buildMessages(in.Collected.Warnings()),
			Notices:               buildMessages(in.Collected.Notices()),
			CollectionStartedAt:   formatTime(in.CollectionStarted, loc),
			CollectionCompletedAt: formatTime(in.CollectionCompleted, loc),
		},
		Summary:    buildSummary(namespaces, workloads, hpas, nodes, nodepools, findings, in.Collected.Autoscaling),
		Namespaces: namespaces,
		Workloads:  workloads,
		HPAs:       hpas,
		Nodes:      nodes,
		Nodepools:  nodepools,
		Findings:   findings,
	}

	if err := snap.Check(); err != nil {
		return Snapshot{}, err
	}
	return snap, nil
}

// buildCapabilities copies each discovery.Capability into the snapshot's
// own Capability type, field by field, turning a nil Evidence or Reasons
// into an empty list.
func buildCapabilities(capabilities []discovery.Capability) []Capability {
	out := make([]Capability, len(capabilities))
	for i, c := range capabilities {
		out[i] = Capability{
			Name:     c.Name,
			State:    c.State,
			Strategy: c.Strategy,
			Evidence: nonNilStrings(c.Evidence),
			Reasons:  nonNilStrings(c.Reasons),
		}
	}
	return out
}

// formatTime writes t in loc as RFC 3339 with the offset, cut to whole
// seconds.
func formatTime(t time.Time, loc *time.Location) string {
	return t.In(loc).Truncate(time.Second).Format(time.RFC3339)
}

// buildMessages copies each collect.Message into the snapshot's own
// Message type, field by field, turning a nil Params into an empty object.
func buildMessages(messages []collect.Message) []Message {
	out := make([]Message, len(messages))
	for i, m := range messages {
		out[i] = Message{Code: m.Code, Params: buildParams(m.Params)}
	}
	return out
}

// buildParams copies params into a map that is never nil, so that an empty
// object of parameters is written as {}, never null.
func buildParams(params map[string]float64) map[string]float64 {
	out := make(map[string]float64, len(params))
	for k, v := range params {
		out[k] = v
	}
	return out
}

// buildStatistics copies a collect.Statistics into the snapshot's own
// Statistics type: the four numbers are set only when the state is
// collect.StateValue, and are nil otherwise.
func buildStatistics(s collect.Statistics) Statistics {
	if s.State != collect.StateValue {
		return Statistics{State: s.State}
	}
	return Statistics{
		State:   s.State,
		Average: ptr(s.Average),
		Minimum: ptr(s.Minimum),
		Peak:    ptr(s.Peak),
		Current: ptr(s.Current),
	}
}

// ptr returns a pointer to a copy of v.
func ptr(v float64) *float64 {
	return &v
}

// buildPods returns nil when pods is not known, and a pointer to its value
// otherwise.
func buildPods(pods int, known bool) *float64 {
	if !known {
		return nil
	}
	value := float64(pods)
	return &value
}

// buildNamespaces copies each collect.Namespace into the snapshot's own
// Namespace type, field by field.
func buildNamespaces(rows []collect.Namespace) []Namespace {
	out := make([]Namespace, len(rows))
	for i, row := range rows {
		out[i] = Namespace{
			Name:                 row.Name,
			Pods:                 buildPods(row.Pods, row.PodsKnown),
			Approximate:          row.Approximate,
			CPUUsage:             buildStatistics(row.Resources.CPUUsage),
			MemoryUsage:          buildStatistics(row.Resources.MemoryUsage),
			CPURequests:          buildStatistics(row.Resources.CPURequests),
			MemoryRequests:       buildStatistics(row.Resources.MemoryRequests),
			CPULimits:            buildStatistics(row.Resources.CPULimits),
			MemoryLimits:         buildStatistics(row.Resources.MemoryLimits),
			CPURequestPercent:    buildStatistics(row.Resources.CPURequestPercent),
			MemoryRequestPercent: buildStatistics(row.Resources.MemoryRequestPercent),
			CPULimitPercent:      buildStatistics(row.Resources.CPULimitPercent),
			MemoryLimitPercent:   buildStatistics(row.Resources.MemoryLimitPercent),
		}
	}
	return out
}

// workloadKey identifies a workload by its namespace, name, and kind.
type workloadKey struct {
	namespace, name, kind string
}

// buildWorkloads makes the rows of the workloads section: one row for each
// measured workload and, after sorting by namespace, name, and kind, one
// row for each workload of the coverage inventory that no measured row
// matches. A measured row is uncovered when the inventory lists it, covered
// when an HPA targets it, and unknown otherwise.
func buildWorkloads(measured []collect.Workload, inventory []collect.WorkloadRef, hpas []collect.HPA) []Workload {
	uncovered := make(map[workloadKey]bool, len(inventory))
	for _, ref := range inventory {
		uncovered[workloadKey{ref.Namespace, ref.Name, ref.Kind}] = true
	}
	targeted := make(map[workloadKey]bool, len(hpas))
	for _, hpa := range hpas {
		if hpa.TargetKind != "" && hpa.TargetName != "" {
			targeted[workloadKey{hpa.Namespace, hpa.TargetName, hpa.TargetKind}] = true
		}
	}

	out := make([]Workload, 0, len(measured)+len(inventory))
	seen := make(map[workloadKey]bool, len(measured))
	for _, row := range measured {
		key := workloadKey{row.Namespace, row.Name, row.Kind}
		seen[key] = true
		coverage := CoverageUnknown
		switch {
		case uncovered[key]:
			coverage = CoverageUncovered
		case targeted[key]:
			coverage = CoverageCovered
		}
		out = append(out, Workload{
			Namespace:            row.Namespace,
			Kind:                 row.Kind,
			Name:                 row.Name,
			ObservedUsage:        true,
			HPACoverage:          coverage,
			Pods:                 buildPods(row.Pods, row.PodsKnown),
			Approximate:          row.Approximate,
			CPUUsage:             buildStatistics(row.Resources.CPUUsage),
			MemoryUsage:          buildStatistics(row.Resources.MemoryUsage),
			CPURequests:          buildStatistics(row.Resources.CPURequests),
			MemoryRequests:       buildStatistics(row.Resources.MemoryRequests),
			CPULimits:            buildStatistics(row.Resources.CPULimits),
			MemoryLimits:         buildStatistics(row.Resources.MemoryLimits),
			CPURequestPercent:    buildStatistics(row.Resources.CPURequestPercent),
			MemoryRequestPercent: buildStatistics(row.Resources.MemoryRequestPercent),
			CPULimitPercent:      buildStatistics(row.Resources.CPULimitPercent),
			MemoryLimitPercent:   buildStatistics(row.Resources.MemoryLimitPercent),
		})
	}

	unavailable := Statistics{State: collect.StateUnavailable}
	for _, ref := range inventory {
		key := workloadKey{ref.Namespace, ref.Name, ref.Kind}
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, Workload{
			Namespace:            ref.Namespace,
			Kind:                 ref.Kind,
			Name:                 ref.Name,
			HPACoverage:          CoverageUncovered,
			CPUUsage:             unavailable,
			MemoryUsage:          unavailable,
			CPURequests:          unavailable,
			MemoryRequests:       unavailable,
			CPULimits:            unavailable,
			MemoryLimits:         unavailable,
			CPURequestPercent:    unavailable,
			MemoryRequestPercent: unavailable,
			CPULimitPercent:      unavailable,
			MemoryLimitPercent:   unavailable,
		})
	}

	sort.SliceStable(out, func(i, j int) bool { return workloadLess(out[i], out[j]) })
	return out
}

// buildNumber returns nil when n is not known, and a pointer to its value
// otherwise.
func buildNumber(n collect.Number) *float64 {
	if !n.Known {
		return nil
	}
	return ptr(n.Value)
}

// buildFlag returns nil when f is not known, and a pointer to its value
// otherwise.
func buildFlag(f collect.Flag) *bool {
	if !f.Known {
		return nil
	}
	value := f.Value
	return &value
}

// buildHPAs copies each collect.HPA into the snapshot's own HPA type, field
// by field.
func buildHPAs(rows []collect.HPA) []HPA {
	out := make([]HPA, len(rows))
	for i, row := range rows {
		out[i] = HPA{
			Namespace:  row.Namespace,
			Name:       row.Name,
			TargetKind: row.TargetKind,
			TargetName: row.TargetName,
			SpecMin:    buildNumber(row.SpecMin),
			SpecMax:    buildNumber(row.SpecMax),
			Current:    buildStatistics(row.Current),
			Desired:    buildStatistics(row.Desired),
		}
	}
	return out
}

// buildNodes copies each collect.Node into the snapshot's own Node type,
// field by field.
func buildNodes(rows []collect.Node) []Node {
	out := make([]Node, len(rows))
	for i, row := range rows {
		out[i] = Node{
			Name:                 row.Name,
			Pool:                 row.Pool,
			PoolConfidence:       row.PoolConfidence,
			CPUCapacity:          buildNumber(row.CPUCapacity),
			MemoryCapacity:       buildNumber(row.MemoryCapacity),
			CPUAllocatable:       buildNumber(row.CPUAllocatable),
			MemoryAllocatable:    buildNumber(row.MemoryAllocatable),
			CPUUsagePercent:      buildStatistics(row.CPUUsagePercent),
			MemoryUsagePercent:   buildStatistics(row.MemoryUsagePercent),
			CPURequestPercent:    buildNumber(row.CPURequestPercent),
			MemoryRequestPercent: buildNumber(row.MemoryRequestPercent),
			Ready:                buildFlag(row.Ready),
			Schedulable:          buildFlag(row.Schedulable),
		}
	}
	return out
}

// buildNodepools copies each collect.Pool into the snapshot's own NodePool
// type, field by field.
func buildNodepools(pools []collect.Pool) []NodePool {
	out := make([]NodePool, len(pools))
	for i, pool := range pools {
		out[i] = NodePool{Name: pool.Name, Confidence: pool.Confidence, Nodes: nonNilStrings(pool.Nodes)}
	}
	return out
}

// buildSummary counts the lists of the snapshot and copies the evidence of
// the autoscaling.
func buildSummary(namespaces []Namespace, workloads []Workload, hpas []HPA, nodes []Node, nodepools []NodePool, findings []Finding, autoscaling collect.AutoscalingEvidence) Summary {
	summary := Summary{
		Namespaces: float64(len(namespaces)),
		HPAs:       float64(len(hpas)),
		Nodes:      float64(len(nodes)),
		Nodepools:  float64(len(nodepools)),
		Findings:   float64(len(findings)),
		Autoscaling: Autoscaling{
			Status:            autoscaling.Status,
			Source:            autoscaling.Source,
			Confidence:        autoscaling.Confidence,
			ManagedNodeGroups: buildNumber(autoscaling.ManagedNodeGroups),
			CurrentNodes:      buildNumber(autoscaling.CurrentNodes),
			MinNodes:          buildNumber(autoscaling.MinNodes),
			MaxNodes:          buildNumber(autoscaling.MaxNodes),
			ScaleEvents:       buildNumber(autoscaling.ScaleEvents),
			UnschedulablePods: buildNumber(autoscaling.UnschedulablePods),
		},
	}
	for _, workload := range workloads {
		if workload.ObservedUsage {
			summary.Workloads++
		}
		if workload.HPACoverage == CoverageUncovered {
			summary.WorkloadsWithoutHPA++
		}
	}
	for _, finding := range findings {
		switch finding.Severity {
		case analysis.SeverityHealthy:
			summary.Healthy++
		case analysis.SeverityAttention:
			summary.Attention++
		case analysis.SeverityCritical:
			summary.Critical++
		}
	}
	return summary
}

// buildFindings copies each analysis.Finding into the snapshot's own
// Finding type, field by field.
func buildFindings(findings []analysis.Finding) []Finding {
	out := make([]Finding, len(findings))
	for i, f := range findings {
		out[i] = Finding{
			Code:       f.Code,
			Severity:   f.Severity,
			ScopeType:  f.ScopeType,
			ScopeKind:  f.ScopeKind,
			ScopeName:  f.ScopeName,
			Evidence:   buildEvidence(f.Evidence),
			Confidence: f.Confidence,
		}
	}
	return out
}

// buildEvidence copies each analysis.Evidence into the snapshot's own
// Evidence type, field by field, never nil.
func buildEvidence(evidence []analysis.Evidence) []Evidence {
	out := make([]Evidence, len(evidence))
	for i, e := range evidence {
		out[i] = Evidence{Name: e.Name, Value: Value{Kind: string(e.Value.Kind), Number: e.Value.Number, Boolean: e.Value.Boolean, Text: e.Value.Text}}
	}
	return out
}

// sortedCopy returns a sorted copy of values, never nil.
func sortedCopy(values []string) []string {
	out := append([]string{}, values...)
	sort.Strings(out)
	return out
}

// nonNilStrings returns values, or an empty list in place of nil. Never
// null is written by encoding/json for the empty list this returns.
func nonNilStrings(values []string) []string {
	if values == nil {
		return []string{}
	}
	return values
}
