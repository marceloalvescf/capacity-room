package collect

import (
	"context"
	"math"
	"slices"
	"strconv"

	"github.com/marceloalvescf/k8s-capacity-report/internal/arith"
	"github.com/marceloalvescf/k8s-capacity-report/internal/discovery"
	"github.com/marceloalvescf/k8s-capacity-report/internal/prom"
)

// The states of the cluster autoscaling.
const (
	AutoscalingEnabled  = "enabled"
	AutoscalingDisabled = "disabled"
	AutoscalingUnknown  = "unknown"
)

// The metrics of the cluster autoscaler, following AUTOSCALER_METRICS of
// collectors/autoscaling.py.
const (
	autoscalerNodeGroups        = "cluster_autoscaler_node_groups_count"
	autoscalerNodes             = "cluster_autoscaler_nodes_count"
	autoscalerMinNodes          = "cluster_autoscaler_node_group_min_nodes_count"
	autoscalerMaxNodes          = "cluster_autoscaler_node_group_max_nodes_count"
	autoscalerScaledUp          = "cluster_autoscaler_scaled_up_nodes_total"
	autoscalerUnschedulablePods = "cluster_autoscaler_unschedulable_pods_count"
)

// autoscalerGroupLabels are the labels that name a node group, in the order
// the first non-empty one wins.
var autoscalerGroupLabels = []string{"node_group", "nodegroup", "node_group_name", "node_group_id"}

// autoscalerGroupKey is the one label the series of a group are reduced by.
const autoscalerGroupKey = "autoscaler_group"

// AutoscalingEvidence is what the cluster autoscaling of the scope is known
// to be.
type AutoscalingEvidence struct {
	Status     string // enabled, disabled, unknown
	Source     string // a discovery strategy: configured, raw_metrics, none
	Confidence string // exact, inferred, unknown

	ManagedNodeGroups, CurrentNodes, MinNodes, MaxNodes Number // whole numbers, truncated
	ScaleEvents, UnschedulablePods                      Number
}

// Autoscaling follows AutoscalingCollector.collect of
// collectors/autoscaling.py. With the mode configured as enabled or disabled
// it sends nothing.
func Autoscaling(ctx context.Context, q Querier, in Input) (AutoscalingEvidence, error) {
	if mode := in.Analysis.NodeAutoscaling; mode == AutoscalingEnabled || mode == AutoscalingDisabled {
		return AutoscalingEvidence{Status: mode, Source: discovery.StrategyConfigured, Confidence: discovery.ConfidenceExact}, nil
	}

	seconds := max(1, in.Window.End.Unix()-in.Window.Start.Unix())
	metrics := []struct {
		name string
		sum  bool
	}{
		{autoscalerNodeGroups, true},
		{autoscalerNodes, true},
		{autoscalerMinNodes, true},
		{autoscalerMaxNodes, true},
		{autoscalerScaledUp, true},
		{autoscalerUnschedulablePods, false},
	}
	var values [6]Number
	for i, metric := range metrics {
		query := prom.Selector{Metric: metric.name, Equal: in.Scope.Matchers()}.String()
		if metric.name == autoscalerScaledUp {
			query = "increase(" + query + "[" + strconv.FormatInt(seconds, 10) + "s])"
		}
		series, err := q.QueryRange(ctx, query, in.Window.Start, in.Window.End, in.Window.Step)
		if err != nil {
			return AutoscalingEvidence{}, err
		}
		values[i] = observedValue(series, metric.sum)
	}

	anyKnown := false
	for _, value := range values {
		anyKnown = anyKnown || value.Known
	}
	if !anyKnown {
		return AutoscalingEvidence{Status: AutoscalingUnknown, Source: discovery.StrategyNone, Confidence: discovery.ConfidenceUnknown}, nil
	}
	return AutoscalingEvidence{
		Status:            AutoscalingEnabled,
		Source:            discovery.StrategyRawMetrics,
		Confidence:        discovery.ConfidenceInferred,
		ManagedNodeGroups: truncated(values[0]),
		CurrentNodes:      truncated(values[1]),
		MinNodes:          truncated(values[2]),
		MaxNodes:          truncated(values[3]),
		ScaleEvents:       values[4],
		UnschedulablePods: values[5],
	}, nil
}

// truncated drops the fraction of a known number, toward zero. The sum with
// zero turns a negative zero into a positive one, as int() does.
func truncated(n Number) Number {
	if n.Known {
		n.Value = math.Trunc(n.Value) + 0
	}
	return n
}

// observedValue follows _observed_value of the original. The group of a
// series is its first non-empty group label. With grouped series and sum,
// the current of each group is taken and the currents are added in the order
// the groups first hold a sample; without sum, the grouped series alone are merged
// into one. With no grouped series, every series is merged into one. The
// merge takes the maximum per timestamp.
func observedValue(series []prom.Series, sum bool) Number {
	var groupedSeries []prom.Series
	var order []string
	for _, item := range series {
		group := groupOf(item.Labels)
		if group == "" {
			continue
		}
		groupedSeries = append(groupedSeries, prom.Series{Labels: map[string]string{autoscalerGroupKey: group}, Points: item.Points})
		if !hasFinitePoint(item) || slices.Contains(order, group) {
			continue
		}
		order = append(order, group)
	}

	if len(groupedSeries) > 0 && sum {
		reduced := ReduceMax(groupedSeries, []string{autoscalerGroupKey})
		var currents []float64
		for _, group := range order {
			if current := currentOf(reduced, Key{group}); current.Known {
				currents = append(currents, current.Value)
			}
		}
		if len(currents) == 0 {
			return Number{}
		}
		return Number{Value: arith.Sum(currents), Known: true}
	}
	if len(groupedSeries) > 0 {
		series = groupedSeries
	}
	for _, statistics := range ReduceMax(series, nil) {
		if statistics.State == StateValue {
			return Number{Value: statistics.Current, Known: true}
		}
	}
	return Number{}
}

// hasFinitePoint reports whether the series holds a point that the reduction
// keeps: a valid one, with a finite time and a finite value.
func hasFinitePoint(series prom.Series) bool {
	return slices.ContainsFunc(series.Points, func(p prom.Point) bool {
		return p.Valid && isFinite(p.Time) && isFinite(p.Value)
	})
}

// groupOf returns the first non-empty node group label, or the empty text.
func groupOf(labels map[string]string) string {
	for _, label := range autoscalerGroupLabels {
		if labels[label] != "" {
			return labels[label]
		}
	}
	return ""
}
