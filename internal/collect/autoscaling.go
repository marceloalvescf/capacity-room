package collect

import (
	"context"
	"maps"
	"strconv"

	"github.com/marceloalvescf/capacity-room/internal/discovery"
	"github.com/marceloalvescf/capacity-room/internal/prom"
)

// The states of the cluster autoscaling.
const (
	AutoscalingEnabled  = "enabled"
	AutoscalingDisabled = "disabled"
	AutoscalingUnknown  = "unknown"
)

// AutoscalingEvidence is what the cluster autoscaling of the scope is known
// to be.
type AutoscalingEvidence struct {
	Status     string // enabled, disabled, unknown
	Source     string // a discovery strategy: configured, raw_metrics, none
	Confidence string // exact, inferred, unknown

	ManagedNodeGroups, CurrentNodes, MinNodes, MaxNodes Number // whole numbers, truncated
	ScaleEvents, UnschedulablePods                      Number
}

// statusConfigMap names the ConfigMap the cluster autoscaler writes its
// status to: by default in kube-system (AKS, the OKE add-on), and in its own
// namespace when installed by hand, so the name alone selects it.
var statusConfigMap = map[string]string{"configmap": "cluster-autoscaler-status"}

// minStatusLookback is the shortest span the status queries look back, in
// seconds: 300 s is the lookback of an instant selector in Prometheus, so a
// window of a few minutes still holds samples of the ConfigMap.
const minStatusLookback = 300

// Autoscaling reads the node autoscaling of the scope from kube-state-metrics
// (design, node autoscaling): enabled when the status ConfigMap of the
// cluster autoscaler was written during the window, or appeared in it when
// no resource version is exported; disabled otherwise. With the mode
// configured, or with no ConfigMap collected at all, it sends nothing.
func Autoscaling(ctx context.Context, q Querier, in Input) (AutoscalingEvidence, error) {
	if mode := in.Analysis.NodeAutoscaling; mode == AutoscalingEnabled || mode == AutoscalingDisabled {
		return AutoscalingEvidence{Status: mode, Source: discovery.StrategyConfigured, Confidence: discovery.ConfidenceExact}, nil
	}
	capability, err := findCapability(in.Capabilities, "node_autoscaling")
	if err != nil {
		return AutoscalingEvidence{}, err
	}
	if capability.State != discovery.StateAvailable {
		return AutoscalingEvidence{Status: AutoscalingUnknown, Source: discovery.StrategyNone, Confidence: discovery.ConfidenceUnknown}, nil
	}

	length := max(1, in.Window.End.Unix()-in.Window.Start.Unix())
	seconds := strconv.FormatInt(length, 10)
	lookback := strconv.FormatInt(max(length, minStatusLookback), 10)
	scope := in.Scope.Matchers()
	configMap := maps.Clone(statusConfigMap)
	maps.Copy(configMap, scope)
	queries := []string{
		"max_over_time(" + prom.Selector{Metric: "kube_configmap_info", Equal: configMap}.String() + "[" + lookback + "s])",
		"changes(" + prom.Selector{Metric: "kube_configmap_metadata_resource_version", Equal: configMap}.String() + "[" + lookback + "s])",
		"count(count by (node) (" + prom.Selector{Metric: "kube_node_info", Equal: scope}.String() + "))",
		"count(count by (node) (max_over_time(" + prom.Selector{Metric: "kube_node_created", Equal: scope}.String() + "[" + seconds + "s]) > time() - " + seconds + "))",
		"count(count by (namespace, pod) (" + prom.Selector{Metric: "kube_pod_status_unschedulable", Equal: scope}.String() + " == 1))",
	}
	var values [5]Number
	for i, query := range queries {
		answer, err := q.Query(ctx, query, in.Window.End)
		if err != nil {
			return AutoscalingEvidence{}, err
		}
		values[i] = largest(answer)
	}
	info, writes, nodes, added, unschedulable := values[0], values[1], values[2], values[3], values[4]

	status := AutoscalingDisabled
	if (writes.Known && writes.Value > 0) || (!writes.Known && info.Known) {
		status = AutoscalingEnabled
	}
	if !nodes.Known {
		added, unschedulable = Number{}, Number{}
	} else {
		added, unschedulable = orZero(added), orZero(unschedulable)
	}
	return AutoscalingEvidence{
		Status:            status,
		Source:            discovery.StrategyRawMetrics,
		Confidence:        discovery.ConfidenceInferred,
		CurrentNodes:      nodes,
		ScaleEvents:       added,
		UnschedulablePods: unschedulable,
	}, nil
}

// largest returns the largest valid, finite value of an instant answer, or
// an unknown number when it holds none: each replica of kube-state-metrics
// answers with a sample of its own.
func largest(samples []prom.Sample) Number {
	var n Number
	for _, s := range samples {
		if s.Valid && isFinite(s.Value) && (!n.Known || s.Value > n.Value) {
			n = Number{Value: s.Value, Known: true}
		}
	}
	return n
}

// orZero reads an empty answer as zero: once kube-state-metrics answered for
// the nodes, no created node or no unschedulable pod is a count of none.
func orZero(n Number) Number {
	if n.Known {
		return n
	}
	return Number{Value: 0, Known: true}
}
