package collect

import (
	"context"
	"sort"

	"github.com/marceloalvescf/capacity-room/internal/discovery"
	"github.com/marceloalvescf/capacity-room/internal/prom"
)

// The metric names the HPA collector queries, following metrics/names.py
// of the original.
const (
	kubeHPAInfo               = "kube_horizontalpodautoscaler_info"
	kubeHPASpecMinReplicas    = "kube_horizontalpodautoscaler_spec_min_replicas"
	kubeHPASpecMaxReplicas    = "kube_horizontalpodautoscaler_spec_max_replicas"
	kubeHPACurrentReplicas    = "kube_horizontalpodautoscaler_status_current_replicas"
	kubeHPADesiredReplicas    = "kube_horizontalpodautoscaler_status_desired_replicas"
	kubeDeploymentLabels      = "kube_deployment_labels"
	kubeDeploymentSpecReplica = "kube_deployment_spec_replicas"
	kubeStatefulSetLabels     = "kube_statefulset_labels"
	kubeStatefulSetReplicas   = "kube_statefulset_replicas"
)

// The names Input.Capabilities carries the two HPA capabilities under.
const (
	hpaHistoryCapability          = "hpa_history"
	workloadsWithoutHPACapability = "workloads_without_hpa"
)

// HPA is the history of one HorizontalPodAutoscaler.
type HPA struct {
	Namespace, Name        string
	TargetKind, TargetName string // empty when unknown or ambiguous
	SpecMin, SpecMax       Number // unknown when absent or ambiguous
	Current, Desired       Statistics
}

// WorkloadRef names a workload of the coverage inventory.
type WorkloadRef struct{ Namespace, Kind, Name string }

// targetRef is the workload an HPA scales.
type targetRef struct{ kind, name string }

// HPAs follows HpaCollector.collect and workloads_without_hpa of
// collectors/hpa.py: the history first, then the coverage inventory. The
// info query is sent once, by the first of them that runs, and both read
// its answer.
func HPAs(ctx context.Context, q Querier, in Input) (history Result[HPA], coverage Result[WorkloadRef], err error) {
	historyCap, err := findCapability(in.Capabilities, hpaHistoryCapability)
	if err != nil {
		return history, coverage, err
	}
	coverageCap, err := findCapability(in.Capabilities, workloadsWithoutHPACapability)
	if err != nil {
		return history, coverage, err
	}

	var info []prom.Sample
	infoSent := false
	sendInfo := func() error {
		if infoSent {
			return nil
		}
		samples, err := q.Query(ctx, hpaQuery(kubeHPAInfo, in), in.Window.End)
		if err != nil {
			return err
		}
		info, infoSent = samples, true
		return nil
	}

	if historyCap.State == discovery.StateUnavailable {
		history = unavailableResult[HPA](historyCap)
	} else {
		if err := sendInfo(); err != nil {
			return history, coverage, err
		}
		history, err = collectHPAHistory(ctx, q, in, historyCap, info)
		if err != nil {
			return history, coverage, err
		}
	}

	if coverageCap.State == discovery.StateUnavailable {
		return history, unavailableResult[WorkloadRef](coverageCap), nil
	}
	if err := sendInfo(); err != nil {
		return history, coverage, err
	}
	coverage, err = collectCoverage(ctx, q, in, coverageCap, info)
	return history, coverage, err
}

// hpaQuery is a bare metric with the scope matchers and the namespaces.
func hpaQuery(name string, in Input) string {
	return prom.Selector{
		Metric: name,
		Equal:  in.Scope.Matchers(),
		Regex:  map[string][]string{"namespace": in.Cluster.Namespaces},
	}.String()
}

// reasonWarnings turns the reason codes of a capability into warnings.
func reasonWarnings(capability discovery.Capability) []Message {
	warnings := []Message{}
	for _, reason := range capability.Reasons {
		warnings = append(warnings, Message{Code: reason, Params: map[string]float64{}})
	}
	return warnings
}

// unavailableResult is the result of a collector whose capability is
// unavailable: it keeps the capability's strategy and reasons.
func unavailableResult[T any](capability discovery.Capability) Result[T] {
	return Result[T]{
		State:    capability.State,
		Strategy: capability.Strategy,
		Rows:     []T{},
		Warnings: reasonWarnings(capability),
		Notices:  []Message{},
	}
}

// hpaTargets reads the target of each HPA from the info samples. A key
// with one distinct target is in targets; a key with more is ambiguous. A
// sample that is not valid is skipped.
func hpaTargets(samples []prom.Sample) (targets map[Key]targetRef, ambiguous map[Key]bool) {
	distinct := make(map[Key]map[targetRef]bool)
	for _, sample := range samples {
		if !sample.Valid || !isFinite(sample.Value) {
			continue
		}
		key := keyOf(sample.Labels, []string{"namespace", "horizontalpodautoscaler"})
		if distinct[key] == nil {
			distinct[key] = make(map[targetRef]bool)
		}
		distinct[key][targetRef{sample.Labels["scaletargetref_kind"], sample.Labels["scaletargetref_name"]}] = true
	}
	targets = make(map[Key]targetRef)
	ambiguous = make(map[Key]bool)
	for key, set := range distinct {
		if len(set) > 1 {
			ambiguous[key] = true
			continue
		}
		for target := range set {
			targets[key] = target
		}
	}
	return targets, ambiguous
}

func collectHPAHistory(ctx context.Context, q Querier, in Input, capability discovery.Capability, info []prom.Sample) (Result[HPA], error) {
	labels := []string{"namespace", "horizontalpodautoscaler"}
	at := float64(in.Window.End.Unix())

	minSamples, err := q.Query(ctx, hpaQuery(kubeHPASpecMinReplicas, in), in.Window.End)
	if err != nil {
		return Result[HPA]{}, err
	}
	maxSamples, err := q.Query(ctx, hpaQuery(kubeHPASpecMaxReplicas, in), in.Window.End)
	if err != nil {
		return Result[HPA]{}, err
	}
	currentSeries, err := q.QueryRange(ctx, hpaQuery(kubeHPACurrentReplicas, in), in.Window.Start, in.Window.End, in.Window.Step)
	if err != nil {
		return Result[HPA]{}, err
	}
	desiredSeries, err := q.QueryRange(ctx, hpaQuery(kubeHPADesiredReplicas, in), in.Window.Start, in.Window.End, in.Window.Step)
	if err != nil {
		return Result[HPA]{}, err
	}

	minimum, minimumAmbiguous := InstantValues(minSamples, at, labels)
	maximum, maximumAmbiguous := InstantValues(maxSamples, at, labels)
	current := ReduceMax(currentSeries, labels)
	desired := ReduceMax(desiredSeries, labels)
	targets, targetAmbiguous := hpaTargets(info)

	ambiguous := len(minimumAmbiguous) > 0 || len(maximumAmbiguous) > 0 || len(targetAmbiguous) > 0
	keys := make(map[Key]bool)
	for key := range targets {
		keys[key] = true
	}
	for key := range minimum {
		keys[key] = true
	}
	for key := range maximum {
		keys[key] = true
	}
	for key := range current {
		keys[key] = true
	}
	for key := range desired {
		keys[key] = true
	}
	for _, group := range []map[Key]bool{minimumAmbiguous, maximumAmbiguous, targetAmbiguous} {
		for key := range group {
			keys[key] = true
		}
	}

	rows := make([]HPA, 0, len(keys))
	for key := range keys {
		row := HPA{
			Namespace: key[0],
			Name:      key[1],
			Current:   Lookup(current, key),
			Desired:   Lookup(desired, key),
		}
		if target, ok := targets[key]; ok {
			row.TargetKind, row.TargetName = target.kind, target.name
		}
		if value, ok := minimum[key]; ok {
			row.SpecMin = Number{Value: value, Known: true}
		}
		if value, ok := maximum[key]; ok {
			row.SpecMax = Number{Value: value, Known: true}
		}
		rows = append(rows, row)
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Namespace != rows[j].Namespace {
			return rows[i].Namespace < rows[j].Namespace
		}
		return rows[i].Name < rows[j].Name
	})

	state := capability.State
	warnings := reasonWarnings(capability)
	if ambiguous {
		state = discovery.StatePartial
		warnings = append(warnings, Message{Code: WarningHPAAmbiguousSpec, Params: map[string]float64{}})
	}
	return Result[HPA]{
		State:    state,
		Strategy: discovery.StrategyRawMetrics,
		Rows:     rows,
		Warnings: warnings,
		Notices:  []Message{},
	}, nil
}

// inventory names the metric that lists one kind of workload and the label
// that holds its name. A replicas metric lists a workload with its scale.
type inventory struct {
	metric   string
	replicas bool
	label    string
	kind     string
}

// inventoryOf picks the labels metric when the capability's evidence holds
// it, else the replicas metric, else none.
func inventoryOf(capability discovery.Capability, labels, replicas, label, kind string) (inventory, bool) {
	for _, name := range []string{labels, replicas} {
		for _, evidence := range capability.Evidence {
			if evidence == name {
				return inventory{metric: name, replicas: name == replicas, label: label, kind: kind}, true
			}
		}
	}
	return inventory{}, false
}

func collectCoverage(ctx context.Context, q Querier, in Input, capability discovery.Capability, info []prom.Sample) (Result[WorkloadRef], error) {
	targets, targetAmbiguous := hpaTargets(info)
	ambiguousNamespaces := make(map[string]bool)
	for key := range targetAmbiguous {
		ambiguousNamespaces[key[0]] = true
	}
	covered := make(map[WorkloadRef]bool)
	for key, target := range targets {
		covered[WorkloadRef{Namespace: key[0], Kind: target.kind, Name: target.name}] = true
	}

	at := float64(in.Window.End.Unix())
	identities := make(map[WorkloadRef]bool)
	omitted := 0
	for _, inv := range []struct {
		labels, replicas, label, kind string
	}{
		{kubeDeploymentLabels, kubeDeploymentSpecReplica, "deployment", "Deployment"},
		{kubeStatefulSetLabels, kubeStatefulSetReplicas, "statefulset", "StatefulSet"},
	} {
		source, ok := inventoryOf(capability, inv.labels, inv.replicas, inv.label, inv.kind)
		if !ok {
			continue
		}
		samples, err := q.Query(ctx, hpaQuery(source.metric, in), in.Window.End)
		if err != nil {
			return Result[WorkloadRef]{}, err
		}
		values, _ := InstantValues(samples, at, []string{"namespace", source.label})
		for key, value := range values {
			if source.replicas && value == 0 {
				omitted++
				continue
			}
			identities[WorkloadRef{Namespace: key[0], Kind: source.kind, Name: key[1]}] = true
		}
	}

	rows := []WorkloadRef{}
	for ref := range identities {
		if !covered[ref] && !ambiguousNamespaces[ref.Namespace] {
			rows = append(rows, ref)
		}
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Namespace != rows[j].Namespace {
			return rows[i].Namespace < rows[j].Namespace
		}
		if rows[i].Name != rows[j].Name {
			return rows[i].Name < rows[j].Name
		}
		return rows[i].Kind < rows[j].Kind
	})

	state := capability.State
	warnings := reasonWarnings(capability)
	if len(targetAmbiguous) > 0 {
		state = discovery.StatePartial
		warnings = append(warnings, Message{Code: WarningHPAAmbiguousTarget, Params: map[string]float64{}})
	}
	notices := []Message{}
	if omitted > 0 {
		notices = append(notices, Message{Code: NoticeWorkloadsOmittedScaleZero, Params: map[string]float64{"count": float64(omitted)}})
	}
	return Result[WorkloadRef]{
		State:    state,
		Strategy: discovery.StrategyRawMetrics,
		Rows:     rows,
		Warnings: warnings,
		Notices:  notices,
	}, nil
}
