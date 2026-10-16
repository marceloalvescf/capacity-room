package collect

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/marceloalvescf/capacity-room/internal/discovery"
	"github.com/marceloalvescf/capacity-room/internal/prom"
)

const (
	queryInfo          = `max_over_time(kube_configmap_info{cluster="prod",configmap="cluster-autoscaler-status",namespace="kube-system"}[86400s])`
	queryWrites        = `changes(kube_configmap_metadata_resource_version{cluster="prod",configmap="cluster-autoscaler-status",namespace="kube-system"}[86400s])`
	queryNodes         = `count(count by (node) (kube_node_info{cluster="prod"}))`
	queryAdded         = `count(count by (node) (max_over_time(kube_node_created{cluster="prod"}[86400s]) > time() - 86400))`
	queryUnschedulable = `count(count by (namespace, pod) (kube_pod_status_unschedulable{cluster="prod"} == 1))`
)

func autoscalingInput(mode string) Input {
	analysis := defaultAnalysis()
	analysis.NodeAutoscaling = mode
	capability := discovery.Capability{
		Name: "node_autoscaling", State: discovery.StateAvailable, Strategy: discovery.StrategyRawMetrics,
		Evidence: []string{"kube_configmap_info"}, Reasons: []string{},
	}
	in := testInput(testScope(true), defaultCluster(), analysis, capability)
	in.Window = Window{Start: time.Unix(1000, 0), End: time.Unix(1000+86400, 0), Step: "5m"}
	return in
}

// autoscalingInputWithout is the input of a run whose probe found no
// ConfigMap.
func autoscalingInputWithout() Input {
	in := autoscalingInput("auto")
	in.Capabilities = []discovery.Capability{{
		Name: "node_autoscaling", State: discovery.StatePartial, Strategy: discovery.StrategyNone,
		Evidence: []string{}, Reasons: []string{"NODE_AUTOSCALING_UNDETERMINED"},
	}}
	return in
}

// autoscalingQuerier answers each instant query with the samples its text
// maps to, and nothing else.
func autoscalingQuerier(answers map[string][]prom.Sample) *scriptedQuerier {
	return &scriptedQuerier{queryFunc: func(query string) ([]prom.Sample, error) {
		return answers[query], nil
	}}
}

func samples(values ...float64) []prom.Sample {
	out := make([]prom.Sample, len(values))
	for i, v := range values {
		out[i] = prom.Sample{Value: v, Valid: true}
	}
	return out
}

func collectAutoscaling(t *testing.T, in Input, answers map[string][]prom.Sample) (AutoscalingEvidence, *scriptedQuerier) {
	t.Helper()
	q := autoscalingQuerier(answers)
	evidence, err := Autoscaling(context.Background(), q, in)
	if err != nil {
		t.Fatalf("Autoscaling: %v", err)
	}
	return evidence, q
}

func known(value float64) Number { return Number{Value: value, Known: true} }

func TestAutoscalingConfiguredSendsNothing(t *testing.T) {
	evidence, q := collectAutoscaling(t, autoscalingInput("disabled"), nil)
	want := AutoscalingEvidence{Status: AutoscalingDisabled, Source: discovery.StrategyConfigured, Confidence: discovery.ConfidenceExact}
	if evidence != want {
		t.Errorf("evidence = %+v, want %+v", evidence, want)
	}
	if len(q.queries) != 0 {
		t.Errorf("queries = %q, want none", q.queries)
	}
	evidence, q = collectAutoscaling(t, autoscalingInput("enabled"), nil)
	if evidence.Status != AutoscalingEnabled || evidence.Source != discovery.StrategyConfigured || len(q.queries) != 0 {
		t.Errorf("enabled: evidence = %+v, queries = %q", evidence, q.queries)
	}
}

func TestAutoscalingQueriesInOrder(t *testing.T) {
	_, q := collectAutoscaling(t, autoscalingInput("auto"), nil)
	want := []string{queryInfo, queryWrites, queryNodes, queryAdded, queryUnschedulable}
	if !reflect.DeepEqual(q.queries, want) {
		t.Errorf("queries =\n%q\nwant\n%q", q.queries, want)
	}
}

func TestAutoscalingWindowAtLeastOneSecond(t *testing.T) {
	in := autoscalingInput("auto")
	in.Window = Window{Start: time.Unix(500, 0), End: time.Unix(500, 0), Step: "5m"}
	_, q := collectAutoscaling(t, in, nil)
	if want := `changes(kube_configmap_metadata_resource_version{cluster="prod",configmap="cluster-autoscaler-status",namespace="kube-system"}[1s])`; q.queries[1] != want {
		t.Errorf("query 2 = %q, want %q", q.queries[1], want)
	}
	if want := `count(count by (node) (max_over_time(kube_node_created{cluster="prod"}[1s]) > time() - 1))`; q.queries[3] != want {
		t.Errorf("query 4 = %q, want %q", q.queries[3], want)
	}
}

func TestAutoscalingWithoutConfigMapsSendsNothing(t *testing.T) {
	evidence, q := collectAutoscaling(t, autoscalingInputWithout(), nil)
	want := AutoscalingEvidence{Status: AutoscalingUnknown, Source: discovery.StrategyNone, Confidence: discovery.ConfidenceUnknown}
	if evidence != want {
		t.Errorf("evidence = %+v, want %+v", evidence, want)
	}
	if len(q.queries) != 0 {
		t.Errorf("queries = %q, want none", q.queries)
	}
}

func TestAutoscalingEnabledWhenWritten(t *testing.T) {
	answers := map[string][]prom.Sample{
		queryInfo: samples(1), queryWrites: samples(118), queryNodes: samples(3),
		queryAdded: samples(1), queryUnschedulable: samples(2),
	}
	evidence, _ := collectAutoscaling(t, autoscalingInput("auto"), answers)
	want := AutoscalingEvidence{
		Status: AutoscalingEnabled, Source: discovery.StrategyRawMetrics, Confidence: discovery.ConfidenceInferred,
		CurrentNodes: known(3), ScaleEvents: known(1), UnschedulablePods: known(2),
	}
	if evidence != want {
		t.Errorf("evidence = %+v, want %+v", evidence, want)
	}
}

func TestAutoscalingReadsTheLargestReplica(t *testing.T) {
	answers := map[string][]prom.Sample{queryInfo: samples(1, 1), queryWrites: samples(0, 57), queryNodes: samples(3)}
	evidence, _ := collectAutoscaling(t, autoscalingInput("auto"), answers)
	if evidence.Status != AutoscalingEnabled {
		t.Errorf("status = %q, want enabled", evidence.Status)
	}
}

func TestAutoscalingDisabledWhenLeftBehind(t *testing.T) {
	answers := map[string][]prom.Sample{queryInfo: samples(1), queryWrites: samples(0), queryNodes: samples(4)}
	evidence, _ := collectAutoscaling(t, autoscalingInput("auto"), answers)
	want := AutoscalingEvidence{
		Status: AutoscalingDisabled, Source: discovery.StrategyRawMetrics, Confidence: discovery.ConfidenceInferred,
		CurrentNodes: known(4), ScaleEvents: known(0), UnschedulablePods: known(0),
	}
	if evidence != want {
		t.Errorf("evidence = %+v, want %+v", evidence, want)
	}
}

func TestAutoscalingDisabledWithoutTheConfigMap(t *testing.T) {
	answers := map[string][]prom.Sample{queryNodes: samples(1)}
	evidence, _ := collectAutoscaling(t, autoscalingInput("auto"), answers)
	want := AutoscalingEvidence{
		Status: AutoscalingDisabled, Source: discovery.StrategyRawMetrics, Confidence: discovery.ConfidenceInferred,
		CurrentNodes: known(1), ScaleEvents: known(0), UnschedulablePods: known(0),
	}
	if evidence != want {
		t.Errorf("evidence = %+v, want %+v", evidence, want)
	}
}

func TestAutoscalingEnabledByPresenceWithoutVersions(t *testing.T) {
	answers := map[string][]prom.Sample{queryInfo: samples(1), queryNodes: samples(1)}
	evidence, _ := collectAutoscaling(t, autoscalingInput("auto"), answers)
	if evidence.Status != AutoscalingEnabled {
		t.Errorf("status = %q, want enabled", evidence.Status)
	}
}

func TestAutoscalingUnknownCountsWithoutNodes(t *testing.T) {
	answers := map[string][]prom.Sample{
		queryInfo: samples(1), queryWrites: samples(5), queryAdded: samples(2), queryUnschedulable: samples(1),
	}
	evidence, _ := collectAutoscaling(t, autoscalingInput("auto"), answers)
	want := AutoscalingEvidence{Status: AutoscalingEnabled, Source: discovery.StrategyRawMetrics, Confidence: discovery.ConfidenceInferred}
	if evidence != want {
		t.Errorf("evidence = %+v, want %+v", evidence, want)
	}
}

func TestAutoscalingIgnoresInvalidSamples(t *testing.T) {
	answers := map[string][]prom.Sample{queryWrites: {{Value: 3, Valid: false}}, queryNodes: samples(1)}
	evidence, _ := collectAutoscaling(t, autoscalingInput("auto"), answers)
	if evidence.Status != AutoscalingDisabled {
		t.Errorf("status = %q, want disabled", evidence.Status)
	}
}
