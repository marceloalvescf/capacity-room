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
		Namespaces: buildNamespaces(in.Collected.Namespaces.Rows),
		Workloads:  buildWorkloads(in.Collected.Workloads.Rows),
		Findings:   buildFindings(in.Findings),
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

// buildWorkloads copies each collect.Workload into the snapshot's own
// Workload type, field by field.
func buildWorkloads(rows []collect.Workload) []Workload {
	out := make([]Workload, len(rows))
	for i, row := range rows {
		out[i] = Workload{
			Namespace:            row.Namespace,
			Kind:                 row.Kind,
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
