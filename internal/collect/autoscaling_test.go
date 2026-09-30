package collect

import (
	"context"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/marceloalvescf/k8s-capacity-report/internal/discovery"
	"github.com/marceloalvescf/k8s-capacity-report/internal/prom"
)

const (
	metricGroups      = "cluster_autoscaler_node_groups_count"
	metricNodes       = "cluster_autoscaler_nodes_count"
	metricMin         = "cluster_autoscaler_node_group_min_nodes_count"
	metricMax         = "cluster_autoscaler_node_group_max_nodes_count"
	metricScaled      = "cluster_autoscaler_scaled_up_nodes_total"
	metricUnschedPods = "cluster_autoscaler_unschedulable_pods_count"
)

func autoscalingInput(mode string) Input {
	analysis := defaultAnalysis()
	analysis.NodeAutoscaling = mode
	in := testInput(testScope(true), defaultCluster(), analysis, testCapability(discovery.StateAvailable, nil))
	in.Window = Window{Start: time.Unix(1000, 0), End: time.Unix(1000+86400, 0), Step: "5m"}
	return in
}

// autoscalingQuerier answers each range query with the series of the metric
// its text names.
func autoscalingQuerier(answers map[string][]prom.Series) *scriptedQuerier {
	return &scriptedQuerier{rangeFunc: func(query string) ([]prom.Series, error) {
		for metric, series := range answers {
			if strings.Contains(query, metric) {
				return series, nil
			}
		}
		return nil, nil
	}}
}

func collectAutoscaling(t *testing.T, in Input, answers map[string][]prom.Series) (AutoscalingEvidence, *scriptedQuerier) {
	t.Helper()
	q := autoscalingQuerier(answers)
	evidence, err := Autoscaling(context.Background(), q, in)
	if err != nil {
		t.Fatalf("Autoscaling: %v", err)
	}
	return evidence, q
}

func grouped(group string, values ...float64) prom.Series {
	return usageSeries(map[string]string{"node_group": group}, values...)
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
	want := []string{
		`cluster_autoscaler_node_groups_count{cluster="prod"}`,
		`cluster_autoscaler_nodes_count{cluster="prod"}`,
		`cluster_autoscaler_node_group_min_nodes_count{cluster="prod"}`,
		`cluster_autoscaler_node_group_max_nodes_count{cluster="prod"}`,
		`increase(cluster_autoscaler_scaled_up_nodes_total{cluster="prod"}[86400s])`,
		`cluster_autoscaler_unschedulable_pods_count{cluster="prod"}`,
	}
	if !reflect.DeepEqual(q.queries, want) {
		t.Errorf("queries =\n%q\nwant\n%q", q.queries, want)
	}
}

func TestAutoscalingWindowAtLeastOneSecond(t *testing.T) {
	in := autoscalingInput("auto")
	in.Window = Window{Start: time.Unix(500, 0), End: time.Unix(500, 0), Step: "5m"}
	_, q := collectAutoscaling(t, in, nil)
	want := `increase(cluster_autoscaler_scaled_up_nodes_total{cluster="prod"}[1s])`
	if q.queries[4] != want {
		t.Errorf("query 5 = %q, want %q", q.queries[4], want)
	}
}

func TestAutoscalingFromTheCompleteScenario(t *testing.T) {
	answers := map[string][]prom.Series{
		metricGroups:      {grouped("pool-a", 1, 2)},
		metricNodes:       {grouped("pool-a", 2, 3)},
		metricMin:         {grouped("pool-a", 1, 1)},
		metricMax:         {grouped("pool-a", 3, 3)},
		metricScaled:      {grouped("pool-a", 0, 1)},
		metricUnschedPods: {grouped("pool-a", 0, 1)},
	}
	evidence, _ := collectAutoscaling(t, autoscalingInput("auto"), answers)
	want := AutoscalingEvidence{
		Status: AutoscalingEnabled, Source: discovery.StrategyRawMetrics, Confidence: discovery.ConfidenceInferred,
		ManagedNodeGroups: known(2), CurrentNodes: known(3), MinNodes: known(1), MaxNodes: known(3),
		ScaleEvents: known(1), UnschedulablePods: known(1),
	}
	if evidence != want {
		t.Errorf("evidence = %+v, want %+v", evidence, want)
	}
}

func TestAutoscalingSumsGroups(t *testing.T) {
	answers := map[string][]prom.Series{metricNodes: {grouped("a", 2, 3), grouped("b", 1, 1)}}
	evidence, _ := collectAutoscaling(t, autoscalingInput("auto"), answers)
	if evidence.CurrentNodes != known(4) {
		t.Errorf("CurrentNodes = %+v, want 4", evidence.CurrentNodes)
	}
}

func TestAutoscalingIgnoresUngroupedBesideGrouped(t *testing.T) {
	answers := map[string][]prom.Series{metricNodes: {
		grouped("a", 2, 2),
		usageSeries(map[string]string{}, 9, 9),
	}}
	evidence, _ := collectAutoscaling(t, autoscalingInput("auto"), answers)
	if evidence.CurrentNodes != known(2) {
		t.Errorf("CurrentNodes = %+v, want 2", evidence.CurrentNodes)
	}
}

func TestAutoscalingUnschedulableUsesOneKeyOfGroupedSeries(t *testing.T) {
	answers := map[string][]prom.Series{metricUnschedPods: {grouped("a", 1, 2), grouped("b", 4, 3)}}
	evidence, _ := collectAutoscaling(t, autoscalingInput("auto"), answers)
	if evidence.UnschedulablePods != known(3) {
		t.Errorf("UnschedulablePods = %+v, want 3, the maximum at the last timestamp", evidence.UnschedulablePods)
	}
}

func TestAutoscalingGroupLabelOrder(t *testing.T) {
	answers := map[string][]prom.Series{metricNodes: {
		usageSeries(map[string]string{"node_group": "", "nodegroup": "a"}, 2, 2),
		usageSeries(map[string]string{"node_group_id": "b", "node_group_name": "c"}, 5, 5),
	}}
	evidence, _ := collectAutoscaling(t, autoscalingInput("auto"), answers)
	if evidence.CurrentNodes != known(7) {
		t.Errorf("CurrentNodes = %+v, want 7", evidence.CurrentNodes)
	}
}

func TestAutoscalingMergesUngroupedByMax(t *testing.T) {
	answers := map[string][]prom.Series{metricNodes: {
		usageSeries(map[string]string{}, 2, 3),
		usageSeries(map[string]string{}, 5, 1),
	}}
	evidence, _ := collectAutoscaling(t, autoscalingInput("auto"), answers)
	if evidence.CurrentNodes != known(3) {
		t.Errorf("CurrentNodes = %+v, want 3", evidence.CurrentNodes)
	}
}

func TestAutoscalingTruncates(t *testing.T) {
	answers := map[string][]prom.Series{
		metricNodes:  {usageSeries(map[string]string{}, 2.9)},
		metricScaled: {usageSeries(map[string]string{}, 2.9)},
	}
	evidence, _ := collectAutoscaling(t, autoscalingInput("auto"), answers)
	if evidence.CurrentNodes != known(2) {
		t.Errorf("CurrentNodes = %+v, want 2", evidence.CurrentNodes)
	}
	if evidence.ScaleEvents != known(2.9) {
		t.Errorf("ScaleEvents = %+v, want 2.9", evidence.ScaleEvents)
	}
}

func TestAutoscalingUnknown(t *testing.T) {
	evidence, _ := collectAutoscaling(t, autoscalingInput("auto"), nil)
	want := AutoscalingEvidence{Status: AutoscalingUnknown, Source: discovery.StrategyNone, Confidence: discovery.ConfidenceUnknown}
	if evidence != want {
		t.Errorf("evidence = %+v, want %+v", evidence, want)
	}
}

func TestAutoscalingOneMetricIsEnough(t *testing.T) {
	answers := map[string][]prom.Series{metricUnschedPods: {usageSeries(map[string]string{}, 4)}}
	evidence, _ := collectAutoscaling(t, autoscalingInput("auto"), answers)
	want := AutoscalingEvidence{
		Status: AutoscalingEnabled, Source: discovery.StrategyRawMetrics, Confidence: discovery.ConfidenceInferred,
		UnschedulablePods: known(4),
	}
	if evidence != want {
		t.Errorf("evidence = %+v, want %+v", evidence, want)
	}
}
