package snapshot

import (
	"sort"
	"time"

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
}

// Build makes a snapshot from in and checks it.
func Build(in Input) (Snapshot, error) {
	loc, err := in.Config.Analysis.Location()
	if err != nil {
		return Snapshot{}, err
	}
	period, err := in.Config.Analysis.Duration()
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
			PeriodStart:      formatTime(periodStart(in.EmittedAt, loc, period), loc),
			PeriodEnd:        emittedAt,
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
			CollectionStartedAt:   formatTime(in.CollectionStarted, loc),
			CollectionCompletedAt: formatTime(in.CollectionCompleted, loc),
		},
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

// periodStart computes the wall clock of emittedAt in loc, less period, and
// lets time.Date normalise the result in loc. This is deliberately not
// emittedAt.Add(-period): the two disagree exactly on a day that crosses a
// change of the zone's offset, and the original counts wall-clock time, not
// elapsed duration.
func periodStart(emittedAt time.Time, loc *time.Location, period time.Duration) time.Time {
	local := emittedAt.In(loc)
	return time.Date(
		local.Year(), local.Month(), local.Day(),
		local.Hour(), local.Minute(), local.Second()-int(period.Seconds()),
		0, loc,
	)
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
