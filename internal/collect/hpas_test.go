package collect

import (
	"context"
	"reflect"
	"testing"

	"github.com/marceloalvescf/k8s-capacity-report/internal/discovery"
	"github.com/marceloalvescf/k8s-capacity-report/internal/prom"
)

const (
	hpaInfoMetric    = "kube_horizontalpodautoscaler_info"
	hpaMinMetric     = "kube_horizontalpodautoscaler_spec_min_replicas"
	hpaMaxMetric     = "kube_horizontalpodautoscaler_spec_max_replicas"
	hpaCurrentMetric = "kube_horizontalpodautoscaler_status_current_replicas"
	hpaDesiredMetric = "kube_horizontalpodautoscaler_status_desired_replicas"
)

const hpaSelector = `{cluster="prod",namespace=~"(?:payments)"}`

func hpaCapability(name, state string, evidence, reasons []string) discovery.Capability {
	strategy := discovery.StrategyRawMetrics
	if state == discovery.StateUnavailable {
		strategy = discovery.StrategyNone
	}
	if evidence == nil {
		evidence = []string{}
	}
	if reasons == nil {
		reasons = []string{}
	}
	return discovery.Capability{Name: name, State: state, Strategy: strategy, Evidence: evidence, Reasons: reasons}
}

var labelsEvidence = []string{"kube_deployment_labels", "kube_statefulset_labels"}

func hpaInput(history, coverage discovery.Capability) Input {
	in := testInput(testScope(true), defaultCluster(), defaultAnalysis(), history)
	in.Capabilities = []discovery.Capability{history, coverage}
	return in
}

func availableHPAInput() Input {
	return hpaInput(
		hpaCapability("hpa_history", discovery.StateAvailable, nil, nil),
		hpaCapability("workloads_without_hpa", discovery.StateAvailable, labelsEvidence, nil),
	)
}

func hpaSample(labels map[string]string, value float64) prom.Sample {
	return prom.Sample{Labels: labels, Value: value, Valid: true}
}

func hpaLabels(name string) map[string]string {
	return map[string]string{"namespace": "payments", "horizontalpodautoscaler": name}
}

func infoSample(name, kind, target string) prom.Sample {
	labels := hpaLabels(name)
	labels["scaletargetref_kind"] = kind
	labels["scaletargetref_name"] = target
	return hpaSample(labels, 1)
}

func hpaRange(name string, values ...float64) prom.Series {
	points := make([]prom.Point, len(values))
	for i, value := range values {
		points[i] = prom.Point{Time: float64(i + 1), Value: value, Valid: true}
	}
	return prom.Series{Labels: hpaLabels(name), Points: points}
}

func workloadSample(label, name string, value float64) prom.Sample {
	return hpaSample(map[string]string{"namespace": "payments", label: name}, value)
}

// hpaQuerier answers by metric name: the query text starts with it.
func hpaQuerier(instant map[string][]prom.Sample, ranges map[string][]prom.Series) *scriptedQuerier {
	return &scriptedQuerier{
		queryFunc: func(query string) ([]prom.Sample, error) {
			for metric, samples := range instant {
				if len(query) >= len(metric) && query[:len(metric)] == metric && query[len(metric)] == '{' {
					return samples, nil
				}
			}
			return nil, nil
		},
		rangeFunc: func(query string) ([]prom.Series, error) {
			for metric, series := range ranges {
				if len(query) >= len(metric) && query[:len(metric)] == metric && query[len(metric)] == '{' {
					return series, nil
				}
			}
			return nil, nil
		},
	}
}

func completeHPAQuerier() *scriptedQuerier {
	return hpaQuerier(
		map[string][]prom.Sample{
			hpaInfoMetric: {infoSample("payments-web", "Deployment", "payments-web")},
			hpaMinMetric:  {hpaSample(hpaLabels("payments-web"), 1)},
			hpaMaxMetric:  {hpaSample(hpaLabels("payments-web"), 4)},
		},
		map[string][]prom.Series{
			hpaCurrentMetric: {hpaRange("payments-web", 2, 3)},
			hpaDesiredMetric: {hpaRange("payments-web", 3, 4)},
		},
	)
}

func TestHPAQueriesInOrder(t *testing.T) {
	q := completeHPAQuerier()
	if _, _, err := HPAs(context.Background(), q, availableHPAInput()); err != nil {
		t.Fatalf("HPAs: %v", err)
	}
	want := []string{
		hpaInfoMetric + hpaSelector,
		hpaMinMetric + hpaSelector,
		hpaMaxMetric + hpaSelector,
		hpaCurrentMetric + hpaSelector,
		hpaDesiredMetric + hpaSelector,
		"kube_deployment_labels" + hpaSelector,
		"kube_statefulset_labels" + hpaSelector,
	}
	if !reflect.DeepEqual(q.queries, want) {
		t.Errorf("queries =\n%q\nwant\n%q", q.queries, want)
	}
}

func TestHPAQueriesKeepConfiguredNamespaceOrder(t *testing.T) {
	q := completeHPAQuerier()
	in := availableHPAInput()
	in.Cluster.Namespaces = []string{"payments", "billing"}
	if _, _, err := HPAs(context.Background(), q, in); err != nil {
		t.Fatalf("HPAs: %v", err)
	}
	want := hpaInfoMetric + `{cluster="prod",namespace=~"(?:payments|billing)"}`
	if q.queries[0] != want {
		t.Errorf("queries[0] = %q, want %q", q.queries[0], want)
	}
}

func TestHPAUnavailableSendsNothing(t *testing.T) {
	q := completeHPAQuerier()
	in := hpaInput(
		hpaCapability("hpa_history", discovery.StateUnavailable, nil, []string{"HPA_HISTORY_INCOMPLETE"}),
		hpaCapability("workloads_without_hpa", discovery.StateAvailable, labelsEvidence, nil),
	)
	history, _, err := HPAs(context.Background(), q, in)
	if err != nil {
		t.Fatalf("HPAs: %v", err)
	}
	if history.State != discovery.StateUnavailable || history.Strategy != discovery.StrategyNone {
		t.Errorf("history state, strategy = %q, %q, want unavailable, none", history.State, history.Strategy)
	}
	if history.Rows == nil || len(history.Rows) != 0 || history.Notices == nil || len(history.Notices) != 0 {
		t.Errorf("history rows, notices = %v, %v, want empty non-nil", history.Rows, history.Notices)
	}
	wantWarnings := []Message{{Code: "HPA_HISTORY_INCOMPLETE", Params: map[string]float64{}}}
	if !reflect.DeepEqual(history.Warnings, wantWarnings) {
		t.Errorf("history warnings = %v, want %v", history.Warnings, wantWarnings)
	}
	want := []string{
		hpaInfoMetric + hpaSelector,
		"kube_deployment_labels" + hpaSelector,
		"kube_statefulset_labels" + hpaSelector,
	}
	if !reflect.DeepEqual(q.queries, want) {
		t.Errorf("queries =\n%q\nwant\n%q", q.queries, want)
	}
}

func TestHPAWarningsHoldTheReasons(t *testing.T) {
	q := completeHPAQuerier()
	q.queryFunc = func(query string) ([]prom.Sample, error) {
		if len(query) > len(hpaMaxMetric) && query[:len(hpaMaxMetric)] == hpaMaxMetric {
			return []prom.Sample{hpaSample(hpaLabels("payments-web"), 4), hpaSample(hpaLabels("payments-web"), 6)}, nil
		}
		return nil, nil
	}
	in := hpaInput(
		hpaCapability("hpa_history", discovery.StatePartial, nil, []string{"HPA_HISTORY_INCOMPLETE"}),
		hpaCapability("workloads_without_hpa", discovery.StateUnavailable, nil, nil),
	)
	history, _, err := HPAs(context.Background(), q, in)
	if err != nil {
		t.Fatalf("HPAs: %v", err)
	}
	want := []Message{
		{Code: "HPA_HISTORY_INCOMPLETE", Params: map[string]float64{}},
		{Code: "HPA_AMBIGUOUS_SPEC", Params: map[string]float64{}},
	}
	if !reflect.DeepEqual(history.Warnings, want) {
		t.Errorf("warnings = %v, want %v", history.Warnings, want)
	}
}

func TestHPAPartialRunsEveryQuery(t *testing.T) {
	q := completeHPAQuerier()
	in := hpaInput(
		hpaCapability("hpa_history", discovery.StatePartial, nil, []string{"HPA_HISTORY_INCOMPLETE"}),
		hpaCapability("workloads_without_hpa", discovery.StateUnavailable, nil, nil),
	)
	history, _, err := HPAs(context.Background(), q, in)
	if err != nil {
		t.Fatalf("HPAs: %v", err)
	}
	if len(q.queries) != 5 {
		t.Errorf("len(queries) = %d, want 5: %q", len(q.queries), q.queries)
	}
	if history.State != discovery.StatePartial || history.Strategy != discovery.StrategyRawMetrics {
		t.Errorf("state, strategy = %q, %q, want partial, raw_metrics", history.State, history.Strategy)
	}
}

func TestHPARowFromTheCompleteScenario(t *testing.T) {
	history, coverage, err := HPAs(context.Background(), completeHPAQuerier(), availableHPAInput())
	if err != nil {
		t.Fatalf("HPAs: %v", err)
	}
	if len(history.Rows) != 1 {
		t.Fatalf("rows = %v, want one", history.Rows)
	}
	row := history.Rows[0]
	if row.Namespace != "payments" || row.Name != "payments-web" || row.TargetKind != "Deployment" || row.TargetName != "payments-web" {
		t.Errorf("identity = %+v", row)
	}
	if row.SpecMin != (Number{Value: 1, Known: true}) || row.SpecMax != (Number{Value: 4, Known: true}) {
		t.Errorf("specs = %+v, %+v, want 1, 4", row.SpecMin, row.SpecMax)
	}
	checkStatistics(t, row.Current, Statistics{State: StateValue, Average: 2.5, Minimum: 2, Peak: 3, Current: 3})
	checkStatistics(t, row.Desired, Statistics{State: StateValue, Average: 3.5, Minimum: 3, Peak: 4, Current: 4})
	if history.State != discovery.StateAvailable || len(history.Warnings) != 0 {
		t.Errorf("state, warnings = %q, %v, want available, none", history.State, history.Warnings)
	}
	if len(coverage.Rows) != 0 || coverage.State != discovery.StateAvailable {
		t.Errorf("coverage = %+v, want available and no rows", coverage)
	}
}

func TestHPAAmbiguousSpec(t *testing.T) {
	q := completeHPAQuerier()
	q.queryFunc = func(query string) ([]prom.Sample, error) {
		if len(query) > len(hpaMaxMetric) && query[:len(hpaMaxMetric)] == hpaMaxMetric {
			return []prom.Sample{hpaSample(hpaLabels("payments-web"), 4), hpaSample(hpaLabels("payments-web"), 6)}, nil
		}
		return nil, nil
	}
	history, _, err := HPAs(context.Background(), q, availableHPAInput())
	if err != nil {
		t.Fatalf("HPAs: %v", err)
	}
	if len(history.Rows) != 1 || history.Rows[0].SpecMax.Known {
		t.Fatalf("rows = %+v, want one row with unknown SpecMax", history.Rows)
	}
	if history.State != discovery.StatePartial {
		t.Errorf("state = %q, want partial", history.State)
	}
	want := []Message{{Code: "HPA_AMBIGUOUS_SPEC", Params: map[string]float64{}}}
	if !reflect.DeepEqual(history.Warnings, want) {
		t.Errorf("warnings = %v, want %v", history.Warnings, want)
	}
}

func TestHPAAmbiguousTarget(t *testing.T) {
	q := hpaQuerier(
		map[string][]prom.Sample{
			hpaInfoMetric: {
				infoSample("payments-web", "Deployment", "payments-web"),
				infoSample("payments-web", "Deployment", "payments-api"),
			},
			"kube_deployment_labels": {workloadSample("deployment", "payments-web", 1), workloadSample("deployment", "payments-api", 1)},
		},
		nil,
	)
	history, coverage, err := HPAs(context.Background(), q, availableHPAInput())
	if err != nil {
		t.Fatalf("HPAs: %v", err)
	}
	if len(history.Rows) != 1 || history.Rows[0].TargetKind != "" || history.Rows[0].TargetName != "" {
		t.Errorf("history rows = %+v, want one row with an empty target", history.Rows)
	}
	if history.State != discovery.StatePartial {
		t.Errorf("history state = %q, want partial", history.State)
	}
	if len(coverage.Rows) != 0 {
		t.Errorf("coverage rows = %v, want none", coverage.Rows)
	}
	if coverage.State != discovery.StatePartial {
		t.Errorf("coverage state = %q, want partial", coverage.State)
	}
	want := []Message{{Code: "HPA_AMBIGUOUS_TARGET", Params: map[string]float64{}}}
	if !reflect.DeepEqual(coverage.Warnings, want) {
		t.Errorf("coverage warnings = %v, want %v", coverage.Warnings, want)
	}
}

func TestHPAWithoutSpecsStillARow(t *testing.T) {
	q := hpaQuerier(nil, map[string][]prom.Series{hpaCurrentMetric: {hpaRange("payments-web", 2)}})
	history, _, err := HPAs(context.Background(), q, availableHPAInput())
	if err != nil {
		t.Fatalf("HPAs: %v", err)
	}
	if len(history.Rows) != 1 {
		t.Fatalf("rows = %+v, want one", history.Rows)
	}
	row := history.Rows[0]
	if row.SpecMin.Known || row.SpecMax.Known {
		t.Errorf("specs = %+v, %+v, want unknown", row.SpecMin, row.SpecMax)
	}
	if row.Desired.State != StateUnavailable {
		t.Errorf("Desired.State = %q, want unavailable", row.Desired.State)
	}
	if row.Current.State != StateValue {
		t.Errorf("Current.State = %q, want value", row.Current.State)
	}
	if row.TargetKind != "" || row.TargetName != "" {
		t.Errorf("target = %q/%q, want empty", row.TargetKind, row.TargetName)
	}
}

func coverageRows(t *testing.T, q Querier, in Input) Result[WorkloadRef] {
	t.Helper()
	_, coverage, err := HPAs(context.Background(), q, in)
	if err != nil {
		t.Fatalf("HPAs: %v", err)
	}
	return coverage
}

func TestCoverageLabelsFamily(t *testing.T) {
	q := hpaQuerier(map[string][]prom.Sample{
		hpaInfoMetric:             {infoSample("web", "Deployment", "web")},
		"kube_deployment_labels":  {workloadSample("deployment", "web", 1), workloadSample("deployment", "api", 1)},
		"kube_statefulset_labels": {workloadSample("statefulset", "queue", 1)},
	}, nil)
	coverage := coverageRows(t, q, availableHPAInput())
	want := []WorkloadRef{
		{Namespace: "payments", Kind: "Deployment", Name: "api"},
		{Namespace: "payments", Kind: "StatefulSet", Name: "queue"},
	}
	if !reflect.DeepEqual(coverage.Rows, want) {
		t.Errorf("rows = %v, want %v", coverage.Rows, want)
	}
	if len(coverage.Notices) != 0 || coverage.Notices == nil {
		t.Errorf("notices = %v, want empty non-nil", coverage.Notices)
	}
	if coverage.State != discovery.StateAvailable || coverage.Strategy != discovery.StrategyRawMetrics {
		t.Errorf("state, strategy = %q, %q", coverage.State, coverage.Strategy)
	}
}

func TestCoverageReplicasFamily(t *testing.T) {
	q := hpaQuerier(map[string][]prom.Sample{
		"kube_deployment_spec_replicas": {workloadSample("deployment", "web", 3), workloadSample("deployment", "batch", 0)},
	}, nil)
	in := hpaInput(
		hpaCapability("hpa_history", discovery.StateUnavailable, nil, nil),
		hpaCapability("workloads_without_hpa", discovery.StateAvailable, []string{"kube_deployment_spec_replicas"}, nil),
	)
	coverage := coverageRows(t, q, in)
	want := []WorkloadRef{{Namespace: "payments", Kind: "Deployment", Name: "web"}}
	if !reflect.DeepEqual(coverage.Rows, want) {
		t.Errorf("rows = %v, want %v", coverage.Rows, want)
	}
	wantNotices := []Message{{Code: "WORKLOADS_OMITTED_SCALE_ZERO", Params: map[string]float64{"count": 1}}}
	if !reflect.DeepEqual(coverage.Notices, wantNotices) {
		t.Errorf("notices = %v, want %v", coverage.Notices, wantNotices)
	}
	wantQueries := []string{hpaInfoMetric + hpaSelector, "kube_deployment_spec_replicas" + hpaSelector}
	if !reflect.DeepEqual(q.queries, wantQueries) {
		t.Errorf("queries = %q, want %q", q.queries, wantQueries)
	}
}

func TestCoverageKindIsCaseSensitive(t *testing.T) {
	q := hpaQuerier(map[string][]prom.Sample{
		hpaInfoMetric:            {infoSample("web", "deployment", "web")},
		"kube_deployment_labels": {workloadSample("deployment", "web", 1)},
	}, nil)
	coverage := coverageRows(t, q, availableHPAInput())
	want := []WorkloadRef{{Namespace: "payments", Kind: "Deployment", Name: "web"}}
	if !reflect.DeepEqual(coverage.Rows, want) {
		t.Errorf("rows = %v, want %v", coverage.Rows, want)
	}
}

func TestCoverageSortsByNamespaceNameKind(t *testing.T) {
	inNamespace := func(namespace, label, name string) prom.Sample {
		return hpaSample(map[string]string{"namespace": namespace, label: name}, 1)
	}
	q := hpaQuerier(map[string][]prom.Sample{
		"kube_deployment_labels":  {inNamespace("b", "deployment", "x"), inNamespace("a", "deployment", "x")},
		"kube_statefulset_labels": {inNamespace("a", "statefulset", "x")},
	}, nil)
	coverage := coverageRows(t, q, availableHPAInput())
	want := []WorkloadRef{
		{Namespace: "a", Kind: "Deployment", Name: "x"},
		{Namespace: "a", Kind: "StatefulSet", Name: "x"},
		{Namespace: "b", Kind: "Deployment", Name: "x"},
	}
	if !reflect.DeepEqual(coverage.Rows, want) {
		t.Errorf("rows = %v, want %v", coverage.Rows, want)
	}
}

func TestCoverageUnavailableSendsNothing(t *testing.T) {
	q := completeHPAQuerier()
	in := hpaInput(
		hpaCapability("hpa_history", discovery.StateUnavailable, nil, nil),
		hpaCapability("workloads_without_hpa", discovery.StateUnavailable, nil, []string{"WORKLOADS_WITHOUT_HPA_INCOMPLETE"}),
	)
	_, coverage, err := HPAs(context.Background(), q, in)
	if err != nil {
		t.Fatalf("HPAs: %v", err)
	}
	if len(q.queries) != 0 {
		t.Errorf("queries = %q, want none", q.queries)
	}
	if coverage.State != discovery.StateUnavailable || coverage.Strategy != discovery.StrategyNone {
		t.Errorf("state, strategy = %q, %q", coverage.State, coverage.Strategy)
	}
	if coverage.Rows == nil || coverage.Notices == nil || len(coverage.Warnings) != 1 {
		t.Errorf("coverage = %+v, want non-nil empty rows and notices, one warning", coverage)
	}
}

func replicasInput(evidence ...string) Input {
	return hpaInput(
		hpaCapability("hpa_history", discovery.StateUnavailable, nil, nil),
		hpaCapability("workloads_without_hpa", discovery.StateAvailable, evidence, nil),
	)
}

func countNotice(count float64) []Message {
	return []Message{{Code: "WORKLOADS_OMITTED_SCALE_ZERO", Params: map[string]float64{"count": count}}}
}

func TestCoverageCountsZeroScaleInAnAmbiguousNamespace(t *testing.T) {
	q := hpaQuerier(map[string][]prom.Sample{
		hpaInfoMetric: {
			infoSample("web", "Deployment", "web"),
			infoSample("web", "Deployment", "api"),
		},
		"kube_deployment_spec_replicas": {workloadSample("deployment", "web", 3), workloadSample("deployment", "batch", 0)},
	}, nil)
	coverage := coverageRows(t, q, replicasInput("kube_deployment_spec_replicas"))
	if len(coverage.Rows) != 0 {
		t.Errorf("rows = %v, want none", coverage.Rows)
	}
	if !reflect.DeepEqual(coverage.Notices, countNotice(1)) {
		t.Errorf("notices = %v, want %v", coverage.Notices, countNotice(1))
	}
}

func TestCoverageAmbiguousInventoryKeyIsNeitherKeptNorCounted(t *testing.T) {
	q := hpaQuerier(map[string][]prom.Sample{
		"kube_deployment_spec_replicas": {
			workloadSample("deployment", "dup", 0),
			workloadSample("deployment", "dup", 2),
			workloadSample("deployment", "batch", 0),
			workloadSample("deployment", "web", 3),
		},
	}, nil)
	coverage := coverageRows(t, q, replicasInput("kube_deployment_spec_replicas"))
	want := []WorkloadRef{{Namespace: "payments", Kind: "Deployment", Name: "web"}}
	if !reflect.DeepEqual(coverage.Rows, want) {
		t.Errorf("rows = %v, want %v", coverage.Rows, want)
	}
	if !reflect.DeepEqual(coverage.Notices, countNotice(1)) {
		t.Errorf("notices = %v, want %v", coverage.Notices, countNotice(1))
	}
}

func TestCoverageStatefulSetReplicasFamily(t *testing.T) {
	q := hpaQuerier(map[string][]prom.Sample{
		"kube_statefulset_replicas": {workloadSample("statefulset", "queue", 2), workloadSample("statefulset", "idle", 0)},
	}, nil)
	coverage := coverageRows(t, q, replicasInput("kube_statefulset_replicas"))
	want := []WorkloadRef{{Namespace: "payments", Kind: "StatefulSet", Name: "queue"}}
	if !reflect.DeepEqual(coverage.Rows, want) {
		t.Errorf("rows = %v, want %v", coverage.Rows, want)
	}
	if !reflect.DeepEqual(coverage.Notices, countNotice(1)) {
		t.Errorf("notices = %v, want %v", coverage.Notices, countNotice(1))
	}
	wantQueries := []string{hpaInfoMetric + hpaSelector, "kube_statefulset_replicas" + hpaSelector}
	if !reflect.DeepEqual(q.queries, wantQueries) {
		t.Errorf("queries = %q, want %q", q.queries, wantQueries)
	}
}

func TestCoveragePrefersLabelsOverReplicas(t *testing.T) {
	q := hpaQuerier(map[string][]prom.Sample{
		"kube_deployment_labels":        {workloadSample("deployment", "batch", 1)},
		"kube_deployment_spec_replicas": {workloadSample("deployment", "web", 3)},
	}, nil)
	coverage := coverageRows(t, q, replicasInput("kube_deployment_spec_replicas", "kube_deployment_labels"))
	want := []WorkloadRef{{Namespace: "payments", Kind: "Deployment", Name: "batch"}}
	if !reflect.DeepEqual(coverage.Rows, want) {
		t.Errorf("rows = %v, want %v", coverage.Rows, want)
	}
	wantQueries := []string{hpaInfoMetric + hpaSelector, "kube_deployment_labels" + hpaSelector}
	if !reflect.DeepEqual(q.queries, wantQueries) {
		t.Errorf("queries = %q, want %q", q.queries, wantQueries)
	}
}
