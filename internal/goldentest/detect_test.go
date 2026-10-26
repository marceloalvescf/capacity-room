package goldentest

import (
	"context"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/marceloalvescf/capacity-room/internal/collect"
	"github.com/marceloalvescf/capacity-room/internal/config"
	"github.com/marceloalvescf/capacity-room/internal/discovery"
	"github.com/marceloalvescf/capacity-room/internal/prom"
	"github.com/marceloalvescf/capacity-room/internal/snapshot"
)

// detectQueries lists, by the exact text Detect sends when the scope's
// matcher is off, every query the nine capability probes can issue. It is
// the same list TestDetectReturnsNineInOrder (internal/discovery) answers
// with "present" for every entry; this file answers every combination of
// "present" and "absent" that detectSets builds instead, so that every
// shape a capability can take is exercised, not only the all-present case
// that package already covers.
var detectQueries = []string{
	`count(container_cpu_usage_seconds_total)`,
	`count(container_memory_working_set_bytes)`,
	`count(namespace_workload_pod:kube_pod_owner:relabel)`,
	`count(kube_pod_owner)`,
	`count(kube_pod_container_resource_requests{resource="cpu"})`,
	`count(kube_pod_container_resource_requests{resource="memory"})`,
	`count(kube_pod_container_resource_limits{resource="cpu"})`,
	`count(kube_pod_container_resource_limits{resource="memory"})`,
	`count(kube_pod_container_resource_requests_cpu_cores)`,
	`count(kube_pod_container_resource_requests_memory_bytes)`,
	`count(kube_pod_container_resource_limits_cpu_cores)`,
	`count(kube_pod_container_resource_limits_memory_bytes)`,
	`count(kube_horizontalpodautoscaler_info)`,
	`count(kube_horizontalpodautoscaler_spec_min_replicas)`,
	`count(kube_horizontalpodautoscaler_spec_max_replicas)`,
	`count(kube_horizontalpodautoscaler_status_current_replicas)`,
	`count(kube_horizontalpodautoscaler_status_desired_replicas)`,
	`count(kube_deployment_labels)`,
	`count(kube_deployment_spec_replicas)`,
	`count(kube_statefulset_labels)`,
	`count(kube_statefulset_replicas)`,
	`count(kube_node_info)`,
	`count(kube_node_labels)`,
	`count(instance:node_cpu_utilisation:rate5m)`,
	`count(instance:node_memory_utilisation:ratio)`,
	`count(node_cpu_seconds_total)`,
	`count(node_memory_MemAvailable_bytes)`,
	`count(node_memory_MemTotal_bytes)`,
	`count(kube_configmap_info)`,
}

// detectQuerier answers every query in present with one valid sample above
// zero, and every other query with none, matching how a real endpoint
// answers "count(metric)" when the metric is, or is not, scraped.
type detectQuerier struct {
	present map[string]bool
}

func (q detectQuerier) Query(_ context.Context, query string, _ time.Time) ([]prom.Sample, error) {
	if q.present[query] {
		return []prom.Sample{{Value: 1, Valid: true}}, nil
	}
	return nil, nil
}

// allDetectQueries returns a set holding every query of detectQueries.
func allDetectQueries() map[string]bool {
	set := make(map[string]bool, len(detectQueries))
	for _, q := range detectQueries {
		set[q] = true
	}
	return set
}

// detectSets returns every set of "present" queries
// TestEverythingDetectGivesPassesCheck runs: all present; none present;
// each single query absent with the rest present; each single query
// present with the rest absent; each pair of queries absent with the rest
// present; and each pair of queries present with the rest absent. A single
// change alone cannot reach every shape a capability takes: workload_usage
// with raw ownership and exactly one usage metric, for instance, needs the
// association metric and one usage metric changed together, which only a
// pair of changes can produce.
func detectSets() []map[string]bool {
	sets := []map[string]bool{allDetectQueries(), {}}

	for i := range detectQueries {
		absentOne := allDetectQueries()
		delete(absentOne, detectQueries[i])
		sets = append(sets, absentOne)

		sets = append(sets, map[string]bool{detectQueries[i]: true})
	}

	for i := range detectQueries {
		for j := i + 1; j < len(detectQueries); j++ {
			absentPair := allDetectQueries()
			delete(absentPair, detectQueries[i])
			delete(absentPair, detectQueries[j])
			sets = append(sets, absentPair)

			sets = append(sets, map[string]bool{detectQueries[i]: true, detectQueries[j]: true})
		}
	}
	return sets
}

// capabilityShape is one distinct combination of name, state, strategy, and
// reasons a capability from Detect can take. reasons is kept in the order
// Detect gives it: a repeated reason, or the same two reasons in the other
// order, is a different shape.
type capabilityShape struct {
	name, state, strategy string
	reasons               []string
}

// shapeKey turns a capabilityShape into a string safe to use as a map key
// (capabilityShape itself holds a slice, which Go maps cannot key on).
func shapeKey(s capabilityShape) string {
	return s.name + "\x00" + s.state + "\x00" + s.strategy + "\x00" + strings.Join(s.reasons, "\x00")
}

// requiredShape is one shape TestEverythingDetectGivesPassesCheck requires
// to have been seen at least once. An empty strategy matches any strategy:
// requests_limits's LEGACY_METRICS_PARTIAL row cares about the reason, not
// the strategy that carries it.
type requiredShape struct {
	name, state, strategy string
	reasons               []string
}

func (r requiredShape) matches(s capabilityShape) bool {
	if s.name != r.name || s.state != r.state {
		return false
	}
	if r.strategy != "" && s.strategy != r.strategy {
		return false
	}
	return slices.Equal(s.reasons, r.reasons)
}

// requiredShapes lists the shapes detectSets must reach at least once: the
// two ways raw ownership with exactly one usage metric present carries two
// reasons, one order for each missing metric; a capability whose only
// evidence is a legacy metric; and node autoscaling reported solely from
// configuration.
var requiredShapes = []requiredShape{
	{"workload_usage", discovery.StatePartial, discovery.StrategyRawMetrics,
		[]string{"WORKLOAD_HISTORY_INEXACT", "MEMORY_USAGE_METRIC_NOT_FOUND"}},
	{"workload_usage", discovery.StatePartial, discovery.StrategyRawMetrics,
		[]string{"WORKLOAD_HISTORY_INEXACT", "CPU_USAGE_METRIC_NOT_FOUND"}},
	{"requests_limits", discovery.StatePartial, "",
		[]string{"LEGACY_METRICS_PARTIAL"}},
	{"node_autoscaling", discovery.StateAvailable, discovery.StrategyConfigured,
		[]string{}},
}

// unavailableNamespaceRows returns one collect.Namespace for each of
// namespaces, sorted, with its ten statistics unavailable and its pod
// count unknown: a namespace row as it reads before any collector has run.
func unavailableNamespaceRows(namespaces []string) []collect.Namespace {
	sorted := append([]string{}, namespaces...)
	slices.Sort(sorted)

	unavailable := collect.Statistics{State: collect.StateUnavailable}
	rows := make([]collect.Namespace, len(sorted))
	for i, name := range sorted {
		rows[i] = collect.Namespace{
			Name: name,
			Resources: collect.Resources{
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
			},
		}
	}
	return rows
}

// TestEverythingDetectGivesPassesCheck proves two things about
// discovery.Detect's output: every shape of every capability it can
// produce passes snapshot.Check (through snapshot.Build, which calls it),
// and the sets this test runs actually reach the shapes that matter, not
// only the ones a lighter sweep would happen to hit. It runs detectSets,
// for node_autoscaling configured as auto, as enabled, and as disabled,
// through Detect and then through Build, collecting the distinct
// (name, state, strategy, reasons) shape of every capability it sees along
// the way. At the end, every shape of requiredShapes must have been seen,
// and every reason code of discovery.ReasonCodes must appear in some
// shape's reasons. Build also needs a window and a collection: neither
// changes with the capabilities under test, so both are fixed once, ahead
// of the sweep, from the harness's own emission time and configured
// namespaces.
func TestEverythingDetectGivesPassesCheck(t *testing.T) {
	emittedAt, err := fixedEmittedAt()
	if err != nil {
		t.Fatalf("fixedEmittedAt: %v", err)
	}

	scope := discovery.Scope{
		ClusterName:      "prod",
		ClusterLabel:     "cluster",
		FilterMode:       "auto",
		MatcherEnabled:   false,
		Confidence:       discovery.ConfidenceInferred,
		DiscoveredValues: []string{"prod"},
	}

	cfg := config.Default()
	cfg.Cluster.Name = scope.ClusterName
	cfg.Cluster.Namespaces = []string{"payments"}
	cfg.Cluster.Label = scope.ClusterLabel
	cfg.Cluster.FilterMode = scope.FilterMode

	window, err := collect.NewWindow(cfg.Analysis, emittedAt)
	if err != nil {
		t.Fatalf("NewWindow: %v", err)
	}
	collected := collect.Collected{
		Namespaces:  collect.Result[collect.Namespace]{Rows: unavailableNamespaceRows(cfg.Cluster.Namespaces)},
		Autoscaling: collect.AutoscalingEvidence{Status: collect.AutoscalingUnknown, Source: discovery.StrategyNone, Confidence: discovery.ConfidenceUnknown},
	}

	sets := detectSets()
	autoscalingValues := []string{"auto", "enabled", "disabled"}
	wantRan := len(autoscalingValues) * len(sets)

	shapes := make(map[string]capabilityShape)
	seenReasons := make(map[string]bool)

	ran := 0
	for _, autoscaling := range autoscalingValues {
		analysis := cfg.Analysis
		analysis.NodeAutoscaling = autoscaling

		for _, set := range sets {
			ran++

			capabilities, err := discovery.Detect(context.Background(), detectQuerier{present: set}, scope, analysis, emittedAt)
			if err != nil {
				t.Fatalf("Detect() error = %v (autoscaling=%s)", err, autoscaling)
			}

			runCfg := cfg
			runCfg.Analysis = analysis

			_, err = snapshot.Build(snapshot.Input{
				ID:                  "00000000-0000-4000-8000-000000000001",
				GeneratorVersion:    "0.0.0-test",
				EmittedAt:           emittedAt,
				Config:              runCfg,
				Scope:               scope,
				Capabilities:        capabilities,
				CollectionStarted:   emittedAt,
				CollectionCompleted: emittedAt.Add(time.Second),
				Window:              window,
				Collected:           collected,
			})
			if err != nil {
				t.Fatalf("Build() error = %v (autoscaling=%s, set=%v)", err, autoscaling, set)
			}

			for _, c := range capabilities {
				shape := capabilityShape{c.Name, c.State, c.Strategy, c.Reasons}
				shapes[shapeKey(shape)] = shape
				for _, r := range c.Reasons {
					seenReasons[r] = true
				}
			}
		}
	}

	t.Logf("ran %d sets, saw %d distinct capability shapes", ran, len(shapes))
	if ran != wantRan {
		t.Fatalf("ran %d sets, want %d", ran, wantRan)
	}

	for _, want := range requiredShapes {
		found := false
		for _, shape := range shapes {
			if want.matches(shape) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("no shape seen matches %+v", want)
		}
	}

	for _, code := range discovery.ReasonCodes {
		if !seenReasons[code] {
			t.Errorf("no shape seen carries reason %s", code)
		}
	}
}
