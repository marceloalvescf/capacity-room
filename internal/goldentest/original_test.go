package goldentest

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/marceloalvescf/k8s-capacity-report/internal/analysis"
	"github.com/marceloalvescf/k8s-capacity-report/internal/catalog"
	"github.com/marceloalvescf/k8s-capacity-report/internal/collect"
	"github.com/marceloalvescf/k8s-capacity-report/internal/discovery"
	"github.com/marceloalvescf/k8s-capacity-report/internal/document"
	"github.com/marceloalvescf/k8s-capacity-report/internal/promtest"
	"github.com/marceloalvescf/k8s-capacity-report/internal/snapshot"
)

// pythonStatistics is one of the ten statistics of a recorded row: four
// numbers, each absent written as null, and no_base.
type pythonStatistics struct {
	Average *float64 `json:"average"`
	Minimum *float64 `json:"minimum"`
	Peak    *float64 `json:"peak"`
	Current *float64 `json:"current"`
	NoBase  bool     `json:"no_base"`
}

// pythonNamespaceRow is one row of the "namespaces" list of a recorded
// file.
type pythonNamespaceRow struct {
	Namespace            string           `json:"namespace"`
	ActivePods           *float64         `json:"active_pods"`
	ApproximateRatios    bool             `json:"approximate_ratios"`
	RequestsLimitsStatus string           `json:"requests_limits_status"`
	CPUUsage             pythonStatistics `json:"cpu_usage"`
	MemoryUsage          pythonStatistics `json:"memory_usage"`
	CPURequests          pythonStatistics `json:"cpu_requests"`
	MemoryRequests       pythonStatistics `json:"memory_requests"`
	CPULimits            pythonStatistics `json:"cpu_limits"`
	MemoryLimits         pythonStatistics `json:"memory_limits"`
	CPURequestPercent    pythonStatistics `json:"cpu_request_percent"`
	MemoryRequestPercent pythonStatistics `json:"memory_request_percent"`
	CPULimitPercent      pythonStatistics `json:"cpu_limit_percent"`
	MemoryLimitPercent   pythonStatistics `json:"memory_limit_percent"`
}

// pythonWorkloadRow is one row of the "workloads" list of a recorded file.
type pythonWorkloadRow struct {
	Namespace            string           `json:"namespace"`
	Kind                 string           `json:"kind"`
	Name                 string           `json:"name"`
	ActivePods           *float64         `json:"active_pods"`
	ApproximateRatios    bool             `json:"approximate_ratios"`
	RequestsLimitsStatus string           `json:"requests_limits_status"`
	CPUUsage             pythonStatistics `json:"cpu_usage"`
	MemoryUsage          pythonStatistics `json:"memory_usage"`
	CPURequests          pythonStatistics `json:"cpu_requests"`
	MemoryRequests       pythonStatistics `json:"memory_requests"`
	CPULimits            pythonStatistics `json:"cpu_limits"`
	MemoryLimits         pythonStatistics `json:"memory_limits"`
	CPURequestPercent    pythonStatistics `json:"cpu_request_percent"`
	MemoryRequestPercent pythonStatistics `json:"memory_request_percent"`
	CPULimitPercent      pythonStatistics `json:"cpu_limit_percent"`
	MemoryLimitPercent   pythonStatistics `json:"memory_limit_percent"`
}

// pythonEvidencePair is one named value of a recorded finding's evidence:
// a number, a boolean, or a text. The pair "approximate" is not compared
// (compareEvidence leaves the pair out and compares the finding's
// confidence instead).
type pythonEvidencePair struct {
	Name  string
	Value analysis.Value
}

// UnmarshalJSON reads a pair written as a two-element JSON array: a name,
// then a number, a boolean, or a text.
func (p *pythonEvidencePair) UnmarshalJSON(data []byte) error {
	var raw []json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	if len(raw) != 2 {
		return fmt.Errorf("evidence pair holds %d elements, want 2", len(raw))
	}
	if err := json.Unmarshal(raw[0], &p.Name); err != nil {
		return fmt.Errorf("evidence pair name: %w", err)
	}
	var number float64
	if err := json.Unmarshal(raw[1], &number); err == nil {
		p.Value = analysis.Number(number)
		return nil
	}
	var flag bool
	if err := json.Unmarshal(raw[1], &flag); err == nil {
		p.Value = analysis.Boolean(flag)
		return nil
	}
	var text string
	if err := json.Unmarshal(raw[1], &text); err != nil {
		return fmt.Errorf("evidence pair value is neither a number, a boolean, nor a text: %w", err)
	}
	p.Value = analysis.Text(text)
	return nil
}

// pythonFinding is one entry of the "findings" list of a recorded file.
type pythonFinding struct {
	Code       string               `json:"code"`
	Severity   string               `json:"severity"`
	ScopeType  string               `json:"scope_type"`
	ScopeKind  string               `json:"scope_kind"`
	ScopeName  string               `json:"scope_name"`
	Confidence string               `json:"confidence"`
	Evidence   []pythonEvidencePair `json:"evidence"`
}

// pythonHPA is one row of the "hpas" list of a recorded file.
type pythonHPA struct {
	Namespace  string           `json:"namespace"`
	Name       string           `json:"name"`
	TargetKind string           `json:"target_kind"`
	TargetName string           `json:"target_name"`
	SpecMin    *float64         `json:"spec_min"`
	SpecMax    *float64         `json:"spec_max"`
	Current    pythonStatistics `json:"current"`
	Desired    pythonStatistics `json:"desired"`
}

// pythonWorkloadWithoutHPA is one row of the "workloads_without_hpa" list
// of a recorded file.
type pythonWorkloadWithoutHPA struct {
	Namespace string `json:"namespace"`
	Kind      string `json:"kind"`
	Name      string `json:"name"`
}

// pythonNode is one row of the "nodes" list of a recorded file.
type pythonNode struct {
	Name                 string           `json:"name"`
	Pool                 string           `json:"pool"`
	PoolConfidence       string           `json:"pool_confidence"`
	CPUCapacity          *float64         `json:"cpu_capacity"`
	MemoryCapacity       *float64         `json:"memory_capacity"`
	CPUAllocatable       *float64         `json:"cpu_allocatable"`
	MemoryAllocatable    *float64         `json:"memory_allocatable"`
	CPUUsagePercent      pythonStatistics `json:"cpu_usage_percent"`
	MemoryUsagePercent   pythonStatistics `json:"memory_usage_percent"`
	CPURequestPercent    *float64         `json:"cpu_request_percent"`
	MemoryRequestPercent *float64         `json:"memory_request_percent"`
	Ready                *bool            `json:"ready"`
	Schedulable          *bool            `json:"schedulable"`
}

// pythonNodepool is one row of the "nodepools" list of a recorded file.
type pythonNodepool struct {
	Name       string   `json:"name"`
	Confidence string   `json:"confidence"`
	Nodes      []string `json:"nodes"`
}

// pythonAutoscaling is the "autoscaling" object of a recorded file.
type pythonAutoscaling struct {
	Status            string   `json:"status"`
	Source            string   `json:"source"`
	Confidence        string   `json:"confidence"`
	ManagedNodeGroups *float64 `json:"managed_node_groups"`
	CurrentNodes      *float64 `json:"current_nodes"`
	MinNodes          *float64 `json:"min_nodes"`
	MaxNodes          *float64 `json:"max_nodes"`
	ScaleEvents       *float64 `json:"scale_events"`
	UnschedulablePods *float64 `json:"unschedulable_pods"`
}

// pythonRecommendation is one recommendation of the original. Its action
// sentence is kept for reference and never compared.
type pythonRecommendation struct {
	Code       string   `json:"code"`
	Severity   string   `json:"severity"`
	ScopeType  string   `json:"scope_type"`
	Scopes     []string `json:"scopes"`
	Confidence string   `json:"confidence"`
	Action     string   `json:"action"`
}

// pythonSummary holds the eleven counts of the summary of the original.
type pythonSummary struct {
	Namespaces          float64 `json:"namespaces"`
	Workloads           float64 `json:"workloads"`
	HPAs                float64 `json:"hpas"`
	Nodes               float64 `json:"nodes"`
	Nodepools           float64 `json:"nodepools"`
	Findings            float64 `json:"findings"`
	Recommendations     float64 `json:"recommendations"`
	Healthy             float64 `json:"healthy"`
	Attention           float64 `json:"attention"`
	Critical            float64 `json:"critical"`
	WorkloadsWithoutHPA float64 `json:"workloads_without_hpa"`
}

// pythonCoverageRow is one row of the workload coverage of the original.
type pythonCoverageRow struct {
	Namespace     string `json:"namespace"`
	Kind          string `json:"kind"`
	Name          string `json:"name"`
	ObservedUsage bool   `json:"observed_usage"`
	HPACoverage   string `json:"hpa_coverage"`
}

// pythonRun is one run of a cell of an executive table.
type pythonRun struct {
	Kind   string `json:"kind"`
	Status string `json:"status"`
	Text   string `json:"text"`
}

// pythonExecutive holds the rows of the seven tables of the executive
// body at one effective minimum, and the kind of its note of the HPA data.
type pythonExecutive struct {
	CriticalWorkloads   [][][]pythonRun `json:"critical_workloads"`
	AttentionWorkloads  [][][]pythonRun `json:"attention_workloads"`
	NamespacesAtRisk    [][][]pythonRun `json:"namespaces_at_risk"`
	ReplicaCeiling      [][][]pythonRun `json:"replica_ceiling"`
	HPANote             string          `json:"hpa_note"`
	NodepoolStatus      [][][]pythonRun `json:"nodepool_status"`
	ActionPlan          [][][]pythonRun `json:"action_plan"`
	MaterialLimitations [][][]pythonRun `json:"material_limitations"`
}

// pythonRecording is one decoded testdata/python/<scenario>.json file.
type pythonRecording struct {
	Scenario              string                     `json:"scenario"`
	Source                string                     `json:"source"`
	PeriodStart           int64                      `json:"period_start"`
	PeriodEnd             int64                      `json:"period_end"`
	Step                  string                     `json:"step"`
	NamespaceCollection   string                     `json:"namespace_collection"`
	WorkloadCollection    string                     `json:"workload_collection"`
	HPACollection         string                     `json:"hpa_collection"`
	HPACoverageCollection string                     `json:"hpa_coverage_collection"`
	NodeCollection        string                     `json:"node_collection"`
	AutoscalingStatus     string                     `json:"autoscaling_status"`
	Namespaces            []pythonNamespaceRow       `json:"namespaces"`
	Workloads             []pythonWorkloadRow        `json:"workloads"`
	HPAs                  []pythonHPA                `json:"hpas"`
	WorkloadsWithoutHPA   []pythonWorkloadWithoutHPA `json:"workloads_without_hpa"`
	Nodes                 []pythonNode               `json:"nodes"`
	Nodepools             []pythonNodepool           `json:"nodepools"`
	Autoscaling           pythonAutoscaling          `json:"autoscaling"`
	NodePressure          bool                       `json:"node_pressure"`
	Findings              []pythonFinding            `json:"findings"`
	Recommendations       []pythonRecommendation     `json:"recommendations"`
	Summary               pythonSummary              `json:"summary"`
	WorkloadCoverage      []pythonCoverageRow        `json:"workload_coverage"`
	Executive             map[string]pythonExecutive `json:"executive"`
	Warnings              []string                   `json:"warnings"`
	Notices               []string                   `json:"notices"`
}

// pythonRecordingPath returns the place of a scenario's recorded file of
// the original.
func pythonRecordingPath(scenario string) (string, error) {
	root, err := moduleRoot()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "testdata", "python", scenario+".json"), nil
}

// pythonQueriesPath returns the place of a scenario's recorded file of the
// queries of the original.
func pythonQueriesPath(scenario string) (string, error) {
	root, err := moduleRoot()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "testdata", "python", scenario+".queries.txt"), nil
}

// loadPythonRecording reads and decodes the recorded file at path.
func loadPythonRecording(path string) (pythonRecording, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return pythonRecording{}, fmt.Errorf("reading %s: %w", path, err)
	}
	var recording pythonRecording
	if err := json.Unmarshal(data, &recording); err != nil {
		return pythonRecording{}, fmt.Errorf("decoding %s: %w", path, err)
	}
	return recording, nil
}

// floatPtr returns a pointer to a copy of v.
func floatPtr(v float64) *float64 {
	return &v
}

// pythonStatisticState follows the rule of the recorded file: no_base true
// gives no_base; four absent numbers with no_base false give unavailable;
// anything else gives value.
func pythonStatisticState(s pythonStatistics) string {
	if s.NoBase {
		return collect.StateNoBase
	}
	if s.Average == nil && s.Minimum == nil && s.Peak == nil && s.Current == nil {
		return collect.StateUnavailable
	}
	return collect.StateValue
}

// comparePointer compares two optional numbers by ==, with no tolerance,
// and reports a difference naming label, field, and both values.
func comparePointer(label, field string, got, want *float64) []string {
	switch {
	case got == nil && want == nil:
		return nil
	case got == nil || want == nil:
		return []string{fmt.Sprintf("%s: %s: got %s, want %s", label, field, floatText(got), floatText(want))}
	case *got != *want:
		return []string{fmt.Sprintf("%s: %s: got %v, want %v", label, field, *got, *want)}
	default:
		return nil
	}
}

// floatText writes p as "null" when it is nil, and its number otherwise.
func floatText(p *float64) string {
	if p == nil {
		return "null"
	}
	return fmt.Sprintf("%v", *p)
}

// compareStatistic compares one of the ten statistics of a row: the state
// the recorded numbers imply, then the four numbers themselves, field by
// field, so that a statistic holding some numbers and some null is judged
// the same way whichever ones are absent.
func compareStatistic(label, field string, got snapshot.Statistics, want pythonStatistics) []string {
	var diffs []string
	if wantState := pythonStatisticState(want); got.State != wantState {
		diffs = append(diffs, fmt.Sprintf("%s: %s.state: got %q, want %q", label, field, got.State, wantState))
	}
	diffs = append(diffs, comparePointer(label, field+".average", got.Average, want.Average)...)
	diffs = append(diffs, comparePointer(label, field+".minimum", got.Minimum, want.Minimum)...)
	diffs = append(diffs, comparePointer(label, field+".peak", got.Peak, want.Peak)...)
	diffs = append(diffs, comparePointer(label, field+".current", got.Current, want.Current)...)
	return diffs
}

// compareTenStatistics compares the ten statistics a namespace or a
// workload row carries, in the order the snapshot lists them.
func compareTenStatistics(label string, got snapshot.Namespace, want pythonNamespaceRow) []string {
	var diffs []string
	diffs = append(diffs, compareStatistic(label, "cpu_usage", got.CPUUsage, want.CPUUsage)...)
	diffs = append(diffs, compareStatistic(label, "memory_usage", got.MemoryUsage, want.MemoryUsage)...)
	diffs = append(diffs, compareStatistic(label, "cpu_requests", got.CPURequests, want.CPURequests)...)
	diffs = append(diffs, compareStatistic(label, "memory_requests", got.MemoryRequests, want.MemoryRequests)...)
	diffs = append(diffs, compareStatistic(label, "cpu_limits", got.CPULimits, want.CPULimits)...)
	diffs = append(diffs, compareStatistic(label, "memory_limits", got.MemoryLimits, want.MemoryLimits)...)
	diffs = append(diffs, compareStatistic(label, "cpu_request_percent", got.CPURequestPercent, want.CPURequestPercent)...)
	diffs = append(diffs, compareStatistic(label, "memory_request_percent", got.MemoryRequestPercent, want.MemoryRequestPercent)...)
	diffs = append(diffs, compareStatistic(label, "cpu_limit_percent", got.CPULimitPercent, want.CPULimitPercent)...)
	diffs = append(diffs, compareStatistic(label, "memory_limit_percent", got.MemoryLimitPercent, want.MemoryLimitPercent)...)
	return diffs
}

// compareWorkloadStatistics is compareTenStatistics for a workload row: the
// two row types carry the same ten statistics, under different Go types.
func compareWorkloadStatistics(label string, got snapshot.Workload, want pythonWorkloadRow) []string {
	asNamespace := snapshot.Namespace{
		CPUUsage: got.CPUUsage, MemoryUsage: got.MemoryUsage,
		CPURequests: got.CPURequests, MemoryRequests: got.MemoryRequests,
		CPULimits: got.CPULimits, MemoryLimits: got.MemoryLimits,
		CPURequestPercent: got.CPURequestPercent, MemoryRequestPercent: got.MemoryRequestPercent,
		CPULimitPercent: got.CPULimitPercent, MemoryLimitPercent: got.MemoryLimitPercent,
	}
	asPythonRow := pythonNamespaceRow{
		CPUUsage: want.CPUUsage, MemoryUsage: want.MemoryUsage,
		CPURequests: want.CPURequests, MemoryRequests: want.MemoryRequests,
		CPULimits: want.CPULimits, MemoryLimits: want.MemoryLimits,
		CPURequestPercent: want.CPURequestPercent, MemoryRequestPercent: want.MemoryRequestPercent,
		CPULimitPercent: want.CPULimitPercent, MemoryLimitPercent: want.MemoryLimitPercent,
	}
	return compareTenStatistics(label, asNamespace, asPythonRow)
}

// namespaceLabel names a namespace row for a difference message.
func namespaceLabel(scenario, name string) string {
	return fmt.Sprintf("%s: namespace %s", scenario, name)
}

// compareNamespaceRow compares one namespace row, field by field: pods,
// approximate, the requests_limits_status of the collector that produced
// the row (not stored in the snapshot itself), and the ten statistics.
func compareNamespaceRow(scenario string, got snapshot.Namespace, requestsLimitsState string, want pythonNamespaceRow) []string {
	label := namespaceLabel(scenario, got.Name)
	var diffs []string
	diffs = append(diffs, comparePointer(label, "pods", got.Pods, want.ActivePods)...)
	if got.Approximate != want.ApproximateRatios {
		diffs = append(diffs, fmt.Sprintf("%s: approximate: got %v, want %v", label, got.Approximate, want.ApproximateRatios))
	}
	if requestsLimitsState != want.RequestsLimitsStatus {
		diffs = append(diffs, fmt.Sprintf("%s: requests_limits_status: got %q, want %q", label, requestsLimitsState, want.RequestsLimitsStatus))
	}
	diffs = append(diffs, compareTenStatistics(label, got, want)...)
	return diffs
}

// workloadLabel names a workload row for a difference message.
func workloadLabel(scenario, namespace, kind, name string) string {
	return fmt.Sprintf("%s: workload %s/%s (%s)", scenario, namespace, name, kind)
}

// compareWorkloadRow compares one workload row, field by field: pods,
// approximate, the requests_limits_status of the collector that produced
// the row (not stored in the snapshot itself), and the ten statistics.
func compareWorkloadRow(scenario string, got snapshot.Workload, requestsLimitsState string, want pythonWorkloadRow) []string {
	label := workloadLabel(scenario, got.Namespace, got.Kind, got.Name)
	var diffs []string
	diffs = append(diffs, comparePointer(label, "pods", got.Pods, want.ActivePods)...)
	if got.Approximate != want.ApproximateRatios {
		diffs = append(diffs, fmt.Sprintf("%s: approximate: got %v, want %v", label, got.Approximate, want.ApproximateRatios))
	}
	if requestsLimitsState != want.RequestsLimitsStatus {
		diffs = append(diffs, fmt.Sprintf("%s: requests_limits_status: got %q, want %q", label, requestsLimitsState, want.RequestsLimitsStatus))
	}
	diffs = append(diffs, compareWorkloadStatistics(label, got, want)...)
	return diffs
}

// compareNamespaces matches the namespaces of a snapshot with those of a
// recorded file by name, and compares each pair found on both sides.
// collectedRows is collect.Collected.Namespaces.Rows of the same run: it
// holds the same rows as got, in the same order, since buildNamespaces
// (internal/snapshot/build.go) copies them across one for one with no
// filtering or reordering, so collectedRows[i] is the row got[i] was built
// from.
func compareNamespaces(scenario string, got []snapshot.Namespace, collectedRows []collect.Namespace, want []pythonNamespaceRow) []string {
	if len(got) != len(collectedRows) {
		return []string{fmt.Sprintf("%s: namespaces: the snapshot holds %d rows, the collector %d", scenario, len(got), len(collectedRows))}
	}

	var diffs []string
	wantByName := make(map[string]pythonNamespaceRow, len(want))
	for _, w := range want {
		wantByName[w.Namespace] = w
	}
	seen := make(map[string]bool, len(got))
	for i, g := range got {
		seen[g.Name] = true
		w, ok := wantByName[g.Name]
		if !ok {
			diffs = append(diffs, fmt.Sprintf("%s: present in the project, missing from the original", namespaceLabel(scenario, g.Name)))
			continue
		}
		diffs = append(diffs, compareNamespaceRow(scenario, g, collectedRows[i].RequestsLimitsState, w)...)
	}
	for _, w := range want {
		if !seen[w.Namespace] {
			diffs = append(diffs, fmt.Sprintf("%s: present in the original, missing from the project", namespaceLabel(scenario, w.Namespace)))
		}
	}
	gotNames := make([]string, len(got))
	for i, g := range got {
		gotNames[i] = g.Name
	}
	wantNames := make([]string, len(want))
	for i, w := range want {
		wantNames[i] = w.Namespace
	}
	return append(diffs, compareKeyOrder(scenario, "namespaces", gotNames, wantNames)...)
}

// compareKeyOrder compares the sequence of keys of a list of the snapshot
// with that of the recording, on the keys both hold: a key held by one
// side only is reported by the comparison by key, and is left out here.
func compareKeyOrder[K comparable](scenario, list string, got, want []K) []string {
	common := func(keys, other []K) []K {
		var kept []K
		for _, k := range keys {
			if slices.Contains(other, k) {
				kept = append(kept, k)
			}
		}
		return kept
	}
	gotKept, wantKept := common(got, want), common(want, got)
	if slices.Equal(gotKept, wantKept) {
		return nil
	}
	return []string{fmt.Sprintf("%s: %s: the order differs from the original: got %v, want %v", scenario, list, gotKept, wantKept)}
}

// workloadRowKey identifies a workload row by namespace, kind, and name.
type workloadRowKey struct{ Namespace, Kind, Name string }

// compareWorkloads matches the workloads of a snapshot with those of a
// recorded file by namespace, kind, and name, and compares each pair found
// on both sides. collectedRows is collect.Collected.Workloads.Rows of the
// same run: it holds the same rows as the observed rows of got, in the same
// order, since buildWorkloads (internal/snapshot/build.go) copies them
// across one for one and only adds rows of the coverage inventory, which
// have no observed usage and are left out here, so collectedRows[i] is the
// row the i-th observed row of got was built from.
func compareWorkloads(scenario string, got []snapshot.Workload, collectedRows []collect.Workload, want []pythonWorkloadRow) []string {
	observed := make([]snapshot.Workload, 0, len(got))
	for _, row := range got {
		if row.ObservedUsage {
			observed = append(observed, row)
		}
	}
	got = observed
	if len(got) != len(collectedRows) {
		return []string{fmt.Sprintf("%s: workloads: the snapshot holds %d rows, the collector %d", scenario, len(got), len(collectedRows))}
	}

	var diffs []string
	wantByKey := make(map[workloadRowKey]pythonWorkloadRow, len(want))
	for _, w := range want {
		wantByKey[workloadRowKey{w.Namespace, w.Kind, w.Name}] = w
	}
	seen := make(map[workloadRowKey]bool, len(got))
	for i, g := range got {
		key := workloadRowKey{g.Namespace, g.Kind, g.Name}
		seen[key] = true
		w, ok := wantByKey[key]
		if !ok {
			diffs = append(diffs, fmt.Sprintf("%s: present in the project, missing from the original", workloadLabel(scenario, g.Namespace, g.Kind, g.Name)))
			continue
		}
		diffs = append(diffs, compareWorkloadRow(scenario, g, collectedRows[i].RequestsLimitsState, w)...)
	}
	for _, w := range want {
		key := workloadRowKey{w.Namespace, w.Kind, w.Name}
		if !seen[key] {
			diffs = append(diffs, fmt.Sprintf("%s: present in the original, missing from the project", workloadLabel(scenario, w.Namespace, w.Kind, w.Name)))
		}
	}
	gotKeys := make([]workloadRowKey, len(got))
	for i, g := range got {
		gotKeys[i] = workloadRowKey{g.Namespace, g.Kind, g.Name}
	}
	wantKeys := make([]workloadRowKey, len(want))
	for i, w := range want {
		wantKeys[i] = workloadRowKey{w.Namespace, w.Kind, w.Name}
	}
	return append(diffs, compareKeyOrder(scenario, "workloads", gotKeys, wantKeys)...)
}

// findingKey identifies a finding by code, scope type, scope name, and
// scope kind, the keying the recorded file's set of findings is compared by.
type findingKey struct{ Code, ScopeType, ScopeName, ScopeKind string }

// findingOrderKey identifies a finding by code, scope type, and scope name:
// the keying of the order, which leaves the scope kind out because the
// original sorts without it.
type findingOrderKey struct{ Code, ScopeType, ScopeName string }

// findingLabel names a finding for a difference message.
func findingLabel(scenario string, key findingKey) string {
	return fmt.Sprintf("%s: finding %s %s %s (%s)", scenario, key.Code, key.ScopeType, key.ScopeName, key.ScopeKind)
}

// describeValue names the kind of v and what it holds.
func describeValue(v analysis.Value) string {
	switch v.Kind {
	case analysis.KindNumber:
		return fmt.Sprintf("number %v", v.Number)
	case analysis.KindBoolean:
		return fmt.Sprintf("boolean %v", v.Boolean)
	default:
		return fmt.Sprintf("%s %q", v.Kind, v.Text)
	}
}

// compareEvidence compares the evidence of a matched finding. The pair
// "approximate" of the recorded evidence is left out: it is compared with
// the confidence of the finding instead, by the ordinary comparison of
// that field.
func compareEvidence(label string, got []snapshot.Evidence, want []pythonEvidencePair) []string {
	filtered := make([]pythonEvidencePair, 0, len(want))
	for _, w := range want {
		if w.Name == "approximate" {
			continue
		}
		filtered = append(filtered, w)
	}
	if len(got) != len(filtered) {
		return []string{fmt.Sprintf("%s: evidence: got %d entries, want %d", label, len(got), len(filtered))}
	}
	var diffs []string
	for i := range got {
		if got[i].Name != filtered[i].Name {
			diffs = append(diffs, fmt.Sprintf("%s: evidence[%d].name: got %q, want %q", label, i, got[i].Name, filtered[i].Name))
			continue
		}
		gotValue := analysis.Value{Kind: analysis.ValueKind(got[i].Value.Kind), Number: got[i].Value.Number, Boolean: got[i].Value.Boolean, Text: got[i].Value.Text}
		if gotValue != filtered[i].Value {
			diffs = append(diffs, fmt.Sprintf("%s: evidence[%d].%s: got %s, want %s", label, i, got[i].Name, describeValue(gotValue), describeValue(filtered[i].Value)))
		}
	}
	return diffs
}

// compareFindings compares the findings of a snapshot with those of a
// recorded file: first as sets keyed by code, scope type, scope name, and
// scope kind,
// so that a missing finding is named as missing and not as a shift of
// every finding after it; then, for a key found on both sides, severity,
// confidence, and evidence; then the order of the two lists of keys, with
// the scope kind left out.
func compareFindings(scenario string, got []snapshot.Finding, want []pythonFinding) []string {
	var diffs []string

	gotByKey := make(map[findingKey]snapshot.Finding, len(got))
	gotOrder := make([]findingKey, len(got))
	for i, f := range got {
		key := findingKey{f.Code, f.ScopeType, f.ScopeName, f.ScopeKind}
		gotByKey[key] = f
		gotOrder[i] = key
	}
	wantByKey := make(map[findingKey]pythonFinding, len(want))
	wantOrder := make([]findingKey, len(want))
	for i, f := range want {
		key := findingKey{f.Code, f.ScopeType, f.ScopeName, f.ScopeKind}
		wantByKey[key] = f
		wantOrder[i] = key
	}

	for _, key := range gotOrder {
		if _, ok := wantByKey[key]; !ok {
			diffs = append(diffs, fmt.Sprintf("%s: present in the project, missing from the original", findingLabel(scenario, key)))
		}
	}
	for _, key := range wantOrder {
		if _, ok := gotByKey[key]; !ok {
			diffs = append(diffs, fmt.Sprintf("%s: present in the original, missing from the project", findingLabel(scenario, key)))
		}
	}

	for _, key := range gotOrder {
		gf, gok := gotByKey[key]
		wf, wok := wantByKey[key]
		if !gok || !wok {
			continue
		}
		label := findingLabel(scenario, key)
		if gf.Severity != wf.Severity {
			diffs = append(diffs, fmt.Sprintf("%s: severity: got %q, want %q", label, gf.Severity, wf.Severity))
		}
		if gf.Confidence != wf.Confidence {
			diffs = append(diffs, fmt.Sprintf("%s: confidence: got %q, want %q", label, gf.Confidence, wf.Confidence))
		}
		diffs = append(diffs, compareEvidence(label, gf.Evidence, wf.Evidence)...)
	}

	if sameKeySet(gotOrder, wantOrder) {
		gotSequence, wantSequence := orderKeys(gotOrder), orderKeys(wantOrder)
		for i := range gotSequence {
			if gotSequence[i] != wantSequence[i] {
				diffs = append(diffs, fmt.Sprintf("%s: findings[%d]: the order differs from the original: got %v, want %v", scenario, i, gotSequence[i], wantSequence[i]))
				break
			}
		}
	}

	return diffs
}

// recommendationKey identifies a recommendation: its code and scope type.
type recommendationKey struct{ Code, ScopeType string }

// compareRecommendations compares the recommendations of the snapshot
// with those of the original, matched by code and scope type: the
// severity, the scopes, and the confidence of each, the entries present on
// one side only, and, when both sides hold the same entries, their order.
func compareRecommendations(scenario string, got []snapshot.Recommendation, want []pythonRecommendation) []string {
	var diffs []string
	label := func(k recommendationKey) string {
		return fmt.Sprintf("%s: recommendation %s (%s)", scenario, k.Code, k.ScopeType)
	}

	gotByKey := make(map[recommendationKey]snapshot.Recommendation, len(got))
	gotOrder := make([]recommendationKey, len(got))
	for i, r := range got {
		key := recommendationKey{r.Code, r.ScopeType}
		gotByKey[key] = r
		gotOrder[i] = key
	}
	wantByKey := make(map[recommendationKey]pythonRecommendation, len(want))
	wantOrder := make([]recommendationKey, len(want))
	for i, r := range want {
		key := recommendationKey{r.Code, r.ScopeType}
		wantByKey[key] = r
		wantOrder[i] = key
	}

	for _, key := range gotOrder {
		if _, ok := wantByKey[key]; !ok {
			diffs = append(diffs, fmt.Sprintf("%s: present in the project, missing from the original", label(key)))
		}
	}
	for _, key := range wantOrder {
		if _, ok := gotByKey[key]; !ok {
			diffs = append(diffs, fmt.Sprintf("%s: present in the original, missing from the project", label(key)))
		}
	}
	for _, key := range gotOrder {
		g, gok := gotByKey[key]
		w, wok := wantByKey[key]
		if !gok || !wok {
			continue
		}
		if g.Severity != w.Severity {
			diffs = append(diffs, fmt.Sprintf("%s: severity: got %q, want %q", label(key), g.Severity, w.Severity))
		}
		if !slices.Equal(g.Scopes, w.Scopes) {
			diffs = append(diffs, fmt.Sprintf("%s: scopes: got %v, want %v", label(key), g.Scopes, w.Scopes))
		}
		if g.Confidence != w.Confidence {
			diffs = append(diffs, fmt.Sprintf("%s: confidence: got %q, want %q", label(key), g.Confidence, w.Confidence))
		}
	}
	if len(gotOrder) == len(wantOrder) && len(gotByKey) == len(wantByKey) && !slices.ContainsFunc(gotOrder, func(k recommendationKey) bool {
		_, ok := wantByKey[k]
		return !ok
	}) {
		for i := range gotOrder {
			if gotOrder[i] != wantOrder[i] {
				diffs = append(diffs, fmt.Sprintf("%s: recommendations[%d]: the order differs from the original: got %v, want %v", scenario, i, gotOrder[i], wantOrder[i]))
				break
			}
		}
	}
	return diffs
}

// compareSummary compares the eleven counts of the summary.
func compareSummary(scenario string, got snapshot.Summary, want pythonSummary) []string {
	var diffs []string
	counts := []struct {
		name      string
		got, want float64
	}{
		{"namespaces", got.Namespaces, want.Namespaces},
		{"workloads", got.Workloads, want.Workloads},
		{"hpas", got.HPAs, want.HPAs},
		{"nodes", got.Nodes, want.Nodes},
		{"nodepools", got.Nodepools, want.Nodepools},
		{"findings", got.Findings, want.Findings},
		{"recommendations", got.Recommendations, want.Recommendations},
		{"healthy", got.Healthy, want.Healthy},
		{"attention", got.Attention, want.Attention},
		{"critical", got.Critical, want.Critical},
		{"workloads_without_hpa", got.WorkloadsWithoutHPA, want.WorkloadsWithoutHPA},
	}
	for _, c := range counts {
		if c.got != c.want {
			diffs = append(diffs, fmt.Sprintf("%s: summary.%s: got %v, want %v", scenario, c.name, c.got, c.want))
		}
	}
	return diffs
}

// coverageKey identifies a workload row of the coverage.
type coverageKey struct{ Namespace, Kind, Name string }

// compareCoverage compares every workload row of the snapshot, those with
// observed usage and those only in the coverage, with the rows of the
// original, matched by namespace, kind, and name: observed_usage and
// hpa_coverage, the rows on one side only, and the order of the keys.
func compareCoverage(scenario string, got []snapshot.Workload, want []pythonCoverageRow) []string {
	var diffs []string
	label := func(k coverageKey) string {
		return fmt.Sprintf("%s: coverage %s/%s/%s", scenario, k.Namespace, k.Kind, k.Name)
	}

	gotByKey := make(map[coverageKey]snapshot.Workload, len(got))
	gotOrder := make([]coverageKey, len(got))
	for i, w := range got {
		key := coverageKey{w.Namespace, w.Kind, w.Name}
		gotByKey[key] = w
		gotOrder[i] = key
	}
	wantByKey := make(map[coverageKey]pythonCoverageRow, len(want))
	wantOrder := make([]coverageKey, len(want))
	for i, w := range want {
		key := coverageKey{w.Namespace, w.Kind, w.Name}
		wantByKey[key] = w
		wantOrder[i] = key
	}

	for _, key := range gotOrder {
		if _, ok := wantByKey[key]; !ok {
			diffs = append(diffs, fmt.Sprintf("%s: present in the project, missing from the original", label(key)))
		}
	}
	for _, key := range wantOrder {
		if _, ok := gotByKey[key]; !ok {
			diffs = append(diffs, fmt.Sprintf("%s: present in the original, missing from the project", label(key)))
		}
	}
	for _, key := range gotOrder {
		g, gok := gotByKey[key]
		w, wok := wantByKey[key]
		if !gok || !wok {
			continue
		}
		if g.ObservedUsage != w.ObservedUsage {
			diffs = append(diffs, fmt.Sprintf("%s: observed_usage: got %v, want %v", label(key), g.ObservedUsage, w.ObservedUsage))
		}
		if g.HPACoverage != w.HPACoverage {
			diffs = append(diffs, fmt.Sprintf("%s: hpa_coverage: got %q, want %q", label(key), g.HPACoverage, w.HPACoverage))
		}
	}
	if len(gotOrder) == len(wantOrder) && len(gotByKey) == len(wantByKey) && !slices.ContainsFunc(gotOrder, func(k coverageKey) bool {
		_, ok := wantByKey[k]
		return !ok
	}) {
		for i := range gotOrder {
			if gotOrder[i] != wantOrder[i] {
				diffs = append(diffs, fmt.Sprintf("%s: coverage[%d]: the order differs from the original: got %v, want %v", scenario, i, gotOrder[i], wantOrder[i]))
				break
			}
		}
	}
	return diffs
}

// runsOf reads the runs of a built cell through their JSON form, which is
// the one place that names a run's kind, status, and text.
func runsOf(runs []document.Run) ([]pythonRun, error) {
	data, err := json.Marshal(runs)
	if err != nil {
		return nil, err
	}
	var out []pythonRun
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// executiveSubsection is one of the seven subsections of the executive
// body: the recorded name, the key of its heading, and the rows and the
// note the original recorded for it.
type executiveSubsection struct {
	name    string
	heading catalog.Key
	rows    func(pythonExecutive) [][][]pythonRun
}

var executiveSubsections = []executiveSubsection{
	{"critical_workloads", catalog.SectionsExecutiveCriticalHeading, func(e pythonExecutive) [][][]pythonRun { return e.CriticalWorkloads }},
	{"attention_workloads", catalog.SectionsExecutiveAttentionHeading, func(e pythonExecutive) [][][]pythonRun { return e.AttentionWorkloads }},
	{"namespaces_at_risk", catalog.SectionsExecutiveNamespacesHeading, func(e pythonExecutive) [][][]pythonRun { return e.NamespacesAtRisk }},
	{"replica_ceiling", catalog.SectionsExecutiveScalingHeading, func(e pythonExecutive) [][][]pythonRun { return e.ReplicaCeiling }},
	{"nodepool_status", catalog.SectionsExecutiveNodepoolsHeading, func(e pythonExecutive) [][][]pythonRun { return e.NodepoolStatus }},
	{"action_plan", catalog.SectionsExecutiveActionsHeading, func(e pythonExecutive) [][][]pythonRun { return e.ActionPlan }},
	{"material_limitations", catalog.SectionsExecutiveLimitationsHeading, func(e pythonExecutive) [][][]pythonRun { return e.MaterialLimitations }},
}

// headingText returns the text of a heading block, or false when the
// block is not a heading of the level.
func headingText(block document.Block, level int) (string, bool) {
	h, ok := block.(document.Heading)
	if !ok || h.Level != level {
		return "", false
	}
	runs, err := runsOf(h.Runs)
	if err != nil {
		return "", false
	}
	var text strings.Builder
	for _, r := range runs {
		text.WriteString(r.Text)
	}
	return text.String(), true
}

// compareExecutive builds the Portuguese executive document of the
// snapshot at each of the two minimums the original recorded, and compares
// the rows of its seven tables with the recorded ones: the count of rows,
// and for every cell the count of runs and, for every run, its kind, its
// status, and its text as compareRun says. The note of the HPA data is
// compared by the title of the callout that follows the replica ceiling.
func compareExecutive(scenario string, snap snapshot.Snapshot, want map[string]pythonExecutive) []string {
	var diffs []string
	cat, err := catalog.Load(catalog.PortugueseBrazil)
	if err != nil {
		return []string{fmt.Sprintf("%s: loading the catalog: %v", scenario, err)}
	}
	indexes, err := executiveIndexes()
	if err != nil {
		return []string{fmt.Sprintf("%s: reading the catalog entries: %v", scenario, err)}
	}
	title := func(key catalog.Key) string {
		text, err := cat.Text(key, nil)
		if err != nil {
			diffs = append(diffs, fmt.Sprintf("%s: reading the catalog text %s: %v", scenario, key, err))
		}
		return text
	}
	executiveHeading := title(catalog.SectionsExecutiveHeading)
	incompleteTitle := title(catalog.SectionsHPAsNoteIncompleteTitle)
	partialTitle := title(catalog.SectionsHPAsNotePartialTitle)

	for _, minimum := range []string{"attention", "critical"} {
		recorded, ok := want[minimum]
		if !ok {
			diffs = append(diffs, fmt.Sprintf("%s: executive.%s: missing from the original", scenario, minimum))
			continue
		}
		doc, err := document.Build(snap, cat, document.Options{Profile: "executive", MinSeverity: minimum})
		if err != nil {
			diffs = append(diffs, fmt.Sprintf("%s: executive.%s: building the document: %v", scenario, minimum, err))
			continue
		}

		pos := slices.IndexFunc(doc.Blocks, func(b document.Block) bool {
			text, ok := headingText(b, 1)
			return ok && text == executiveHeading
		})
		if pos < 0 {
			diffs = append(diffs, fmt.Sprintf("%s: executive.%s: the document holds no executive heading", scenario, minimum))
			continue
		}

		note := ""
		for _, sub := range executiveSubsections {
			label := fmt.Sprintf("%s: executive.%s.%s", scenario, minimum, sub.name)
			wantText := title(sub.heading)
			next := slices.IndexFunc(doc.Blocks[pos+1:], func(b document.Block) bool {
				text, ok := headingText(b, 2)
				return ok && text == wantText
			})
			if next < 0 || pos+2+next >= len(doc.Blocks) {
				diffs = append(diffs, fmt.Sprintf("%s: the document holds no subsection with the heading %q", label, wantText))
				continue
			}
			pos += 1 + next
			var rows [][][]document.Run
			if table, ok := doc.Blocks[pos+1].(document.Table); ok {
				rows = table.Rows
			}
			diffs = append(diffs, compareExecutiveRows(label, indexes, rows, originalPlanCells(sub.name, rows, originalLimitationMarks(sub.name, sub.rows(recorded))))...)

			if sub.name == "replica_ceiling" && pos+2 < len(doc.Blocks) {
				if callout, ok := doc.Blocks[pos+2].(document.Callout); ok {
					runs, err := runsOf(callout.Title)
					switch {
					case err != nil:
						diffs = append(diffs, fmt.Sprintf("%s: reading the note: %v", label, err))
					case len(runs) == 1 && runs[0].Text == incompleteTitle:
						note = "incomplete"
					case len(runs) == 1 && runs[0].Text == partialTitle:
						note = "partial"
					default:
						diffs = append(diffs, fmt.Sprintf("%s: the note has an unknown title", label))
					}
				}
			}
		}
		if note != recorded.HPANote {
			diffs = append(diffs, fmt.Sprintf("%s: executive.%s: hpa_note: got %q, want %q", scenario, minimum, note, recorded.HPANote))
		}
	}
	return diffs
}

// originalLimitationMarks returns the recorded rows of a table with the mark
// of the coverage cell of the material limitations read as this project
// draws it: the original drew an unavailable capability with the critical
// mark; this project draws it with the unavailable mark, the legend's
// meaning for missing data. Every other status stays as recorded, and
// the rows given are left as they are.
func originalLimitationMarks(table string, rows [][][]pythonRun) [][][]pythonRun {
	if table != "material_limitations" {
		return rows
	}
	out := make([][][]pythonRun, len(rows))
	for r, row := range rows {
		out[r] = make([][]pythonRun, len(row))
		for c, cell := range row {
			out[r][c] = slices.Clone(cell)
			if c != 1 {
				continue
			}
			for i := range out[r][c] {
				if out[r][c][i].Status == "critical" {
					out[r][c][i].Status = "unavailable"
				}
			}
		}
	}
	return out
}

// originalQualifiers are the four qualifier words the original wrote in
// parentheses after an action, as testdata/python records them.
var originalQualifiers = []string{"namespaces", "cargas de trabalho", "HPAs", "cluster"}

// originalPlanCells returns the recorded rows of the action plan with two
// differences of wording read as this project writes them. The original
// wrote a parenthesised qualifier after an action that more than one row
// shares, and named a scope by the part of its name after the last slash;
// this project writes the action alone and each scope by its full name. The
// qualifier is stripped from the recorded action when it is one of the four
// words the original recorded (originalQualifiers), and no other
// parenthesis; a recorded scope that equals ours, or equals the part after
// the last slash of ours, scope by scope and in order, is read as ours; the
// word of the scope type that starts our cell is read as the recorded cell's
// own. A scope
// that names another object, a different count of scopes, and every other
// cell, status, row, and number are left as recorded. The rows given are
// left as they are.
func originalPlanCells(table string, got [][][]document.Run, want [][][]pythonRun) [][][]pythonRun {
	if table != "action_plan" {
		return want
	}
	out := make([][][]pythonRun, len(want))
	for r, row := range want {
		out[r] = make([][]pythonRun, len(row))
		for c, cell := range row {
			out[r][c] = slices.Clone(cell)
			if len(out[r][c]) != 1 {
				continue
			}
			switch c {
			case 2:
				text := out[r][c][0].Text
				for _, qualifier := range originalQualifiers {
					text = strings.TrimSuffix(text, " ("+qualifier+")")
				}
				out[r][c][0].Text = text
			case 3:
				if r >= len(got) || len(got[r]) <= 3 {
					continue
				}
				runs, err := runsOf(got[r][3])
				if err != nil {
					continue
				}
				// This project starts the cell with the word of the scope
				// type, a text run the original did not write; the word is
				// carried over and the names are compared as before.
				if len(runs) == 2 && runs[0].Kind == "text" {
					out[r][c] = append([]pythonRun{runs[0]}, out[r][c]...)
					runs = runs[1:]
				}
				if len(runs) != 1 {
					continue
				}
				last := len(out[r][c]) - 1
				ours, theirs := strings.Split(runs[0].Text, ", "), strings.Split(out[r][c][last].Text, ", ")
				if len(ours) != len(theirs) {
					continue
				}
				same := true
				for i := range ours {
					if theirs[i] != ours[i] && theirs[i] != ours[i][strings.LastIndex(ours[i], "/")+1:] {
						same = false
					}
				}
				if same {
					out[r][c][last].Text = runs[0].Text
				}
			}
		}
	}
	return out
}

// compareExecutiveRows compares the rows of one table with the recorded
// ones, as compareExecutive describes. indexes holds the index of the
// original's texts and the reverse index of the catalog.
func compareExecutiveRows(label string, indexes [2]textIndex, got [][][]document.Run, want [][][]pythonRun) []string {
	var diffs []string
	if len(got) != len(want) {
		return []string{fmt.Sprintf("%s: rows: got %d, want %d", label, len(got), len(want))}
	}
	for r := range got {
		if len(got[r]) != len(want[r]) {
			diffs = append(diffs, fmt.Sprintf("%s: row %d: cells: got %d, want %d", label, r, len(got[r]), len(want[r])))
			continue
		}
		for c := range got[r] {
			cellLabel := fmt.Sprintf("%s: row %d, cell %d", label, r, c)
			runs, err := runsOf(got[r][c])
			if err != nil {
				diffs = append(diffs, fmt.Sprintf("%s: reading the runs: %v", cellLabel, err))
				continue
			}
			if len(runs) != len(want[r][c]) {
				diffs = append(diffs, fmt.Sprintf("%s: runs: got %d, want %d", cellLabel, len(runs), len(want[r][c])))
				continue
			}
			for i := range runs {
				diffs = append(diffs, compareRun(fmt.Sprintf("%s, run %d", cellLabel, i), indexes, runs[i], want[r][c][i])...)
			}
		}
	}
	return diffs
}

// compareRun compares one run: the kind, the status, and the text. An
// ident is compared letter for letter. Any other run is compared by the
// catalog keys behind its text: the original's text is read through
// originalExecutiveTexts, the program's through the reverse index of the
// Portuguese catalog. When both have a reading, they must share one, keys
// and parameters alike; when neither has one, the texts must be equal
// letter for letter; when only one has a reading, they differ.
func compareRun(label string, indexes [2]textIndex, got, want pythonRun) []string {
	var diffs []string
	if got.Kind != want.Kind {
		diffs = append(diffs, fmt.Sprintf("%s: kind: got %q, want %q", label, got.Kind, want.Kind))
	}
	if got.Status != want.Status {
		diffs = append(diffs, fmt.Sprintf("%s: status: got %q, want %q", label, got.Status, want.Status))
	}
	if want.Kind == "ident" {
		if got.Text != want.Text {
			diffs = append(diffs, fmt.Sprintf("%s: text: got %q, want %q", label, got.Text, want.Text))
		}
		return diffs
	}
	wantForms, gotForms := indexes[0].resolve(want.Text), indexes[1].resolve(got.Text)
	switch {
	case len(gotForms) == 0 && len(wantForms) == 0:
		if got.Text != want.Text {
			diffs = append(diffs, fmt.Sprintf("%s: text: got %q, want %q", label, got.Text, want.Text))
		}
	case len(wantForms) == 0:
		diffs = append(diffs, fmt.Sprintf("%s: text: got %q as %s, want %q with no catalog key", label, got.Text, strings.Join(gotForms, " or "), want.Text))
	case len(gotForms) == 0:
		diffs = append(diffs, fmt.Sprintf("%s: text: got %q with no catalog key, want %q as %s", label, got.Text, want.Text, strings.Join(wantForms, " or ")))
	case !slices.ContainsFunc(gotForms, func(form string) bool { return slices.Contains(wantForms, form) }):
		diffs = append(diffs, fmt.Sprintf("%s: text: got %q as %s, want %q as %s", label, got.Text, strings.Join(gotForms, " or "), want.Text, strings.Join(wantForms, " or ")))
	}
	return diffs
}

// orderKeys drops the scope kind of every key.
func orderKeys(keys []findingKey) []findingOrderKey {
	out := make([]findingOrderKey, len(keys))
	for i, key := range keys {
		out[i] = findingOrderKey{key.Code, key.ScopeType, key.ScopeName}
	}
	return out
}

// sameKeySet reports whether a and b hold the same keys the same number of
// times, in any order.
func sameKeySet(a, b []findingKey) bool {
	if len(a) != len(b) {
		return false
	}
	counts := make(map[findingKey]int, len(a))
	for _, key := range a {
		counts[key]++
	}
	for _, key := range b {
		counts[key]--
	}
	for _, count := range counts {
		if count != 0 {
			return false
		}
	}
	return true
}

// originalWarnings holds the sentence the original writes for each warning
// code, in Portuguese, letter for letter. The recorded files of
// testdata/python/ hold these sentences, and the comparison renders them
// from the codes of the snapshot rather than from the catalog, whose
// Portuguese follows the vocabulary of spec 9.3 and not the original.
var originalWarnings = map[string]string{
	collect.WarningNamespaceUsageIncomplete:           "As métricas de uso dos namespaces estão incompletas.",
	collect.WarningNamespaceRequestsLimitsIncomplete:  "As métricas de solicitações e limites dos namespaces estão incompletas.",
	collect.WarningNamespaceRequestsLimitsUnavailable: "As métricas de solicitações e limites dos namespaces estão indisponíveis.",
	collect.WarningWorkloadUsageIncomplete:            "As métricas de uso das cargas de trabalho estão incompletas.",
	collect.WarningWorkloadRequestsLimitsIncomplete:   "As métricas de solicitações e limites das cargas de trabalho estão incompletas.",
	collect.WarningWorkloadRequestsLimitsUnavailable:  "As métricas de solicitações e limites das cargas de trabalho estão indisponíveis.",
	collect.WarningWorkloadHistoryCurrentAssociation:  "O histórico usa a associação atual entre pods ativos e cargas de trabalho.",
	collect.WarningHPAAmbiguousSpec:                   "Valores ambíguos de configuração ou alvo de HPA foram omitidos.",
	collect.WarningHPAAmbiguousTarget:                 "Alvos ambíguos de HPA impedem uma cobertura confiável das cargas de trabalho.",
	collect.WarningNodeIdentityUnavailable:            "As métricas de identidade dos nós estão indisponíveis.",
	collect.WarningNodeCapacityEvidenceIncomplete:     "As evidências de capacidade dos nós estão incompletas.",

	// The reasons a capability carries, which the original adds to the
	// warnings as they stand.
	"METRICS_NOT_FOUND":                "As métricas necessárias não foram encontradas.",
	"CPU_USAGE_METRIC_NOT_FOUND":       "A métrica de uso de CPU não foi encontrada.",
	"MEMORY_USAGE_METRIC_NOT_FOUND":    "A métrica de uso de memória não foi encontrada.",
	"USAGE_METRICS_NOT_FOUND":          "As métricas de uso de CPU e memória não foram encontradas.",
	"WORKLOAD_HISTORY_INEXACT":         "A associação histórica entre pods e cargas de trabalho não pôde ser reconstruída com exatidão.",
	"WORKLOAD_ASSOCIATION_UNAVAILABLE": "A associação por carga de trabalho não está disponível; a cobertura está limitada a pods.",
	"REQUESTS_LIMITS_INCOMPLETE":       "A cobertura de solicitações e limites está incompleta para CPU ou memória.",
	"LEGACY_METRICS_PARTIAL":           "As métricas legadas oferecem cobertura parcial de recursos.",
	"HPA_HISTORY_INCOMPLETE":           "A cobertura histórica dos HPAs está incompleta.",
	"WORKLOAD_INVENTORY_INCOMPLETE":    "O inventário de cargas de trabalho ou alvos de HPA está incompleto.",
	"NODE_ADDRESS_MAPPING_INCOMPLETE":  "A associação entre endereços e nós pode estar incompleta.",
	"NODEPOOL_LABELS_NOT_FOUND":        "Os rótulos de pools de nós não foram encontrados.",
	"NODE_AUTOSCALING_UNDETERMINED":    "O estado do escalonamento automático de nós não pôde ser determinado.",
}

// originalNotice holds the two forms of a notice of the original: the
// sentence for a count of one, and the sentence for any other count with
// %s standing for the count.
type originalNotice struct {
	singular string
	plural   string
}

var originalNotices = map[string]originalNotice{
	collect.NoticeWorkloadsOmittedNoUsage: {
		singular: "1 carga de trabalho deixada de fora por não ter consumo observado no período.",
		plural:   "%s cargas de trabalho deixadas de fora por não terem consumo observado no período.",
	},
	collect.NoticeJobsOmitted: {
		singular: "1 Job foi deixado de fora do inventário de cargas de trabalho; o consumo continua contabilizado no total do namespace.",
		plural:   "%s Jobs foram deixados de fora do inventário de cargas de trabalho; o consumo continua contabilizado nos totais dos namespaces.",
	},
	collect.NoticeWorkloadsOmittedScaleZero: {
		singular: "1 carga deixada de fora do inventário de cobertura por estar com escala zero.",
		plural:   "%s cargas deixadas de fora do inventário de cobertura por estarem com escala zero.",
	},
}

// originalWarningText returns the sentence of the original for a warning
// code, or an error naming a code the table does not hold.
func originalWarningText(code string) (string, error) {
	text, ok := originalWarnings[code]
	if !ok {
		return "", fmt.Errorf("no sentence of the original for warning %s", code)
	}
	return text, nil
}

// originalNoticeText returns the sentence of the original for a notice code
// and a count: the singular form for one, otherwise the plural form with
// the count written as Python writes an integer.
func originalNoticeText(code string, count float64) (string, error) {
	notice, ok := originalNotices[code]
	if !ok {
		return "", fmt.Errorf("no sentence of the original for notice %s", code)
	}
	if count == 1 {
		return notice.singular, nil
	}
	return fmt.Sprintf(notice.plural, strconv.FormatFloat(count, 'f', -1, 64)), nil
}

// originalTexts renders the warnings and notices of a snapshot as the
// original would have written them, in the order of the snapshot.
func originalTexts(warnings, notices []snapshot.Message) ([]string, []string, error) {
	warningTexts := make([]string, len(warnings))
	for i, w := range warnings {
		text, err := originalWarningText(w.Code)
		if err != nil {
			return nil, nil, err
		}
		warningTexts[i] = text
	}
	noticeTexts := make([]string, len(notices))
	for i, n := range notices {
		text, err := originalNoticeText(n.Code, n.Params["count"])
		if err != nil {
			return nil, nil, err
		}
		noticeTexts[i] = text
	}
	return warningTexts, noticeTexts, nil
}

// compareStrings compares two lists of sentences position by position,
// reporting a difference at every index held by only one of the lists, or
// held by both with a different sentence.
func compareStrings(scenario, label string, got, want []string) []string {
	var diffs []string
	n := len(got)
	if len(want) > n {
		n = len(want)
	}
	for i := 0; i < n; i++ {
		var g, w string
		gok, wok := i < len(got), i < len(want)
		if gok {
			g = got[i]
		}
		if wok {
			w = want[i]
		}
		if !gok || !wok || g != w {
			diffs = append(diffs, fmt.Sprintf("%s: %s[%d]: got %q, want %q", scenario, label, i, g, w))
		}
	}
	return diffs
}

// hpaLabel names an HPA row for a difference message.
func hpaLabel(scenario, namespace, name string) string {
	return fmt.Sprintf("%s: hpa %s/%s", scenario, namespace, name)
}

// compareHPAs matches the HPAs of a snapshot with those of a recorded file
// by namespace and name, and compares each pair found on both sides: the
// target, the two specs, and the two statistics.
func compareHPAs(scenario string, got []snapshot.HPA, want []pythonHPA) []string {
	var diffs []string
	type key struct{ Namespace, Name string }
	wantByKey := make(map[key]pythonHPA, len(want))
	for _, w := range want {
		wantByKey[key{w.Namespace, w.Name}] = w
	}
	seen := make(map[key]bool, len(got))
	for _, g := range got {
		k := key{g.Namespace, g.Name}
		seen[k] = true
		label := hpaLabel(scenario, g.Namespace, g.Name)
		w, ok := wantByKey[k]
		if !ok {
			diffs = append(diffs, fmt.Sprintf("%s: present in the project, missing from the original", label))
			continue
		}
		if g.TargetKind != w.TargetKind {
			diffs = append(diffs, fmt.Sprintf("%s: target_kind: got %q, want %q", label, g.TargetKind, w.TargetKind))
		}
		if g.TargetName != w.TargetName {
			diffs = append(diffs, fmt.Sprintf("%s: target_name: got %q, want %q", label, g.TargetName, w.TargetName))
		}
		diffs = append(diffs, comparePointer(label, "spec_min", g.SpecMin, w.SpecMin)...)
		diffs = append(diffs, comparePointer(label, "spec_max", g.SpecMax, w.SpecMax)...)
		diffs = append(diffs, compareStatistic(label, "current", g.Current, w.Current)...)
		diffs = append(diffs, compareStatistic(label, "desired", g.Desired, w.Desired)...)
	}
	for _, w := range want {
		if !seen[key{w.Namespace, w.Name}] {
			diffs = append(diffs, fmt.Sprintf("%s: present in the original, missing from the project", hpaLabel(scenario, w.Namespace, w.Name)))
		}
	}
	gotKeys := make([]key, len(got))
	for i, g := range got {
		gotKeys[i] = key{g.Namespace, g.Name}
	}
	wantKeys := make([]key, len(want))
	for i, w := range want {
		wantKeys[i] = key{w.Namespace, w.Name}
	}
	return append(diffs, compareKeyOrder(scenario, "hpas", gotKeys, wantKeys)...)
}

// compareUncovered compares the workloads of the snapshot whose HPA
// coverage is uncovered with the recorded workloads_without_hpa, position
// by position: namespace, kind, and name.
func compareUncovered(scenario string, got []snapshot.Workload, want []pythonWorkloadWithoutHPA) []string {
	var uncovered []snapshot.Workload
	for _, row := range got {
		if row.HPACoverage == snapshot.CoverageUncovered {
			uncovered = append(uncovered, row)
		}
	}
	var diffs []string
	for i := 0; i < len(uncovered) || i < len(want); i++ {
		label := fmt.Sprintf("%s: workloads_without_hpa[%d]", scenario, i)
		switch {
		case i >= len(want):
			diffs = append(diffs, fmt.Sprintf("%s: present in the project, missing from the original: %s/%s (%s)", label, uncovered[i].Namespace, uncovered[i].Name, uncovered[i].Kind))
		case i >= len(uncovered):
			diffs = append(diffs, fmt.Sprintf("%s: present in the original, missing from the project: %s/%s (%s)", label, want[i].Namespace, want[i].Name, want[i].Kind))
		default:
			g, w := uncovered[i], want[i]
			if g.Namespace != w.Namespace {
				diffs = append(diffs, fmt.Sprintf("%s: namespace: got %q, want %q", label, g.Namespace, w.Namespace))
			}
			if g.Kind != w.Kind {
				diffs = append(diffs, fmt.Sprintf("%s: kind: got %q, want %q", label, g.Kind, w.Kind))
			}
			if g.Name != w.Name {
				diffs = append(diffs, fmt.Sprintf("%s: name: got %q, want %q", label, g.Name, w.Name))
			}
		}
	}
	return diffs
}

// nodeLabel names a node row for a difference message.
func nodeLabel(scenario, name string) string {
	return fmt.Sprintf("%s: node %s", scenario, name)
}

// boolText writes p as "null" when it is nil, and its value otherwise.
func boolText(p *bool) string {
	if p == nil {
		return "null"
	}
	return strconv.FormatBool(*p)
}

// compareBoolPointer compares two optional booleans and reports a
// difference naming label, field, and both values.
func compareBoolPointer(label, field string, got, want *bool) []string {
	if (got == nil) == (want == nil) && (got == nil || *got == *want) {
		return nil
	}
	return []string{fmt.Sprintf("%s: %s: got %s, want %s", label, field, boolText(got), boolText(want))}
}

// compareNodes matches the nodes of a snapshot with those of a recorded
// file by name, and compares every field of each pair found on both sides.
func compareNodes(scenario string, got []snapshot.Node, want []pythonNode) []string {
	var diffs []string
	wantByName := make(map[string]pythonNode, len(want))
	for _, w := range want {
		wantByName[w.Name] = w
	}
	seen := make(map[string]bool, len(got))
	for _, g := range got {
		seen[g.Name] = true
		label := nodeLabel(scenario, g.Name)
		w, ok := wantByName[g.Name]
		if !ok {
			diffs = append(diffs, fmt.Sprintf("%s: present in the project, missing from the original", label))
			continue
		}
		if g.Pool != w.Pool {
			diffs = append(diffs, fmt.Sprintf("%s: pool: got %q, want %q", label, g.Pool, w.Pool))
		}
		if g.PoolConfidence != w.PoolConfidence {
			diffs = append(diffs, fmt.Sprintf("%s: pool_confidence: got %q, want %q", label, g.PoolConfidence, w.PoolConfidence))
		}
		diffs = append(diffs, comparePointer(label, "cpu_capacity", g.CPUCapacity, w.CPUCapacity)...)
		diffs = append(diffs, comparePointer(label, "memory_capacity", g.MemoryCapacity, w.MemoryCapacity)...)
		diffs = append(diffs, comparePointer(label, "cpu_allocatable", g.CPUAllocatable, w.CPUAllocatable)...)
		diffs = append(diffs, comparePointer(label, "memory_allocatable", g.MemoryAllocatable, w.MemoryAllocatable)...)
		diffs = append(diffs, compareStatistic(label, "cpu_usage_percent", g.CPUUsagePercent, w.CPUUsagePercent)...)
		diffs = append(diffs, compareStatistic(label, "memory_usage_percent", g.MemoryUsagePercent, w.MemoryUsagePercent)...)
		diffs = append(diffs, comparePointer(label, "cpu_request_percent", g.CPURequestPercent, w.CPURequestPercent)...)
		diffs = append(diffs, comparePointer(label, "memory_request_percent", g.MemoryRequestPercent, w.MemoryRequestPercent)...)
		diffs = append(diffs, compareBoolPointer(label, "ready", g.Ready, w.Ready)...)
		diffs = append(diffs, compareBoolPointer(label, "schedulable", g.Schedulable, w.Schedulable)...)
	}
	for _, w := range want {
		if !seen[w.Name] {
			diffs = append(diffs, fmt.Sprintf("%s: present in the original, missing from the project", nodeLabel(scenario, w.Name)))
		}
	}
	gotNames := make([]string, len(got))
	for i, g := range got {
		gotNames[i] = g.Name
	}
	wantNames := make([]string, len(want))
	for i, w := range want {
		wantNames[i] = w.Name
	}
	return append(diffs, compareKeyOrder(scenario, "nodes", gotNames, wantNames)...)
}

// nodepoolLabel names a node pool row for a difference message.
func nodepoolLabel(scenario, name string) string {
	return fmt.Sprintf("%s: nodepool %s", scenario, name)
}

// compareNodepools matches the pools of a snapshot with those of a
// recorded file by name, and compares the confidence and the node names,
// in order, of each pair found on both sides.
func compareNodepools(scenario string, got []snapshot.NodePool, want []pythonNodepool) []string {
	var diffs []string
	wantByName := make(map[string]pythonNodepool, len(want))
	for _, w := range want {
		wantByName[w.Name] = w
	}
	seen := make(map[string]bool, len(got))
	for _, g := range got {
		seen[g.Name] = true
		label := nodepoolLabel(scenario, g.Name)
		w, ok := wantByName[g.Name]
		if !ok {
			diffs = append(diffs, fmt.Sprintf("%s: present in the project, missing from the original", label))
			continue
		}
		if g.Confidence != w.Confidence {
			diffs = append(diffs, fmt.Sprintf("%s: confidence: got %q, want %q", label, g.Confidence, w.Confidence))
		}
		diffs = append(diffs, compareStrings(label, "nodes", g.Nodes, w.Nodes)...)
	}
	for _, w := range want {
		if !seen[w.Name] {
			diffs = append(diffs, fmt.Sprintf("%s: present in the original, missing from the project", nodepoolLabel(scenario, w.Name)))
		}
	}
	gotNames := make([]string, len(got))
	for i, g := range got {
		gotNames[i] = g.Name
	}
	wantNames := make([]string, len(want))
	for i, w := range want {
		wantNames[i] = w.Name
	}
	return append(diffs, compareKeyOrder(scenario, "nodepools", gotNames, wantNames)...)
}

// compareAutoscaling compares the autoscaling of the snapshot summary with
// the recorded one, field by field.
func compareAutoscaling(scenario string, got snapshot.Autoscaling, want pythonAutoscaling) []string {
	label := scenario + ": autoscaling"
	var diffs []string
	if got.Status != want.Status {
		diffs = append(diffs, fmt.Sprintf("%s: status: got %q, want %q", label, got.Status, want.Status))
	}
	if got.Source != want.Source {
		diffs = append(diffs, fmt.Sprintf("%s: source: got %q, want %q", label, got.Source, want.Source))
	}
	if got.Confidence != want.Confidence {
		diffs = append(diffs, fmt.Sprintf("%s: confidence: got %q, want %q", label, got.Confidence, want.Confidence))
	}
	diffs = append(diffs, comparePointer(label, "managed_node_groups", got.ManagedNodeGroups, want.ManagedNodeGroups)...)
	diffs = append(diffs, comparePointer(label, "current_nodes", got.CurrentNodes, want.CurrentNodes)...)
	diffs = append(diffs, comparePointer(label, "min_nodes", got.MinNodes, want.MinNodes)...)
	diffs = append(diffs, comparePointer(label, "max_nodes", got.MaxNodes, want.MaxNodes)...)
	diffs = append(diffs, comparePointer(label, "scale_events", got.ScaleEvents, want.ScaleEvents)...)
	diffs = append(diffs, comparePointer(label, "unschedulable_pods", got.UnschedulablePods, want.UnschedulablePods)...)
	return diffs
}

// nodePressure reports whether any finding of scope node or nodepool is at
// attention or above.
func nodePressure(findings []snapshot.Finding) bool {
	for _, f := range findings {
		if (f.ScopeType == analysis.ScopeNode || f.ScopeType == analysis.ScopeNodepool) && f.Severity != analysis.SeverityHealthy {
			return true
		}
	}
	return false
}

// compareCollectionStates compares the states of the five collectors and
// the status of the cluster autoscaling evidence with the recorded ones.
func compareCollectionStates(scenario string, collected collect.Collected, want pythonRecording) []string {
	var diffs []string
	for _, c := range []struct{ name, got, want string }{
		{"namespace_collection", collected.Namespaces.State, want.NamespaceCollection},
		{"workload_collection", collected.Workloads.State, want.WorkloadCollection},
		{"hpa_collection", collected.HPAs.State, want.HPACollection},
		{"hpa_coverage_collection", collected.Coverage.State, want.HPACoverageCollection},
		{"node_collection", collected.Nodes.State, want.NodeCollection},
		{"autoscaling_status", collected.Autoscaling.Status, want.AutoscalingStatus},
	} {
		if c.got != c.want {
			diffs = append(diffs, fmt.Sprintf("%s: %s: got %q, want %q", scenario, c.name, c.got, c.want))
		}
	}
	return diffs
}

// compareNumbers compares snap and collected, the snapshot and the
// collectors' own result of one run of scenario, with want, a decoded
// recorded file of the original, and returns every difference found: a
// namespace row, a workload row, a finding, a warning, or a notice present
// on one side and not the other, or a field of a matched row or finding
// that disagrees, each naming the scenario, the row or finding, and the
// field, with both values.
func compareNumbers(scenario string, snap snapshot.Snapshot, collected collect.Collected, want pythonRecording) []string {
	var diffs []string
	diffs = append(diffs, compareNamespaces(scenario, snap.Namespaces, collected.Namespaces.Rows, want.Namespaces)...)
	diffs = append(diffs, compareWorkloads(scenario, snap.Workloads, collected.Workloads.Rows, want.Workloads)...)
	diffs = append(diffs, compareHPAs(scenario, snap.HPAs, want.HPAs)...)
	diffs = append(diffs, compareUncovered(scenario, snap.Workloads, want.WorkloadsWithoutHPA)...)
	diffs = append(diffs, compareNodes(scenario, snap.Nodes, want.Nodes)...)
	diffs = append(diffs, compareNodepools(scenario, snap.Nodepools, want.Nodepools)...)
	diffs = append(diffs, compareAutoscaling(scenario, snap.Summary.Autoscaling, want.Autoscaling)...)
	if got := nodePressure(snap.Findings); got != want.NodePressure {
		diffs = append(diffs, fmt.Sprintf("%s: node_pressure: got %v, want %v", scenario, got, want.NodePressure))
	}
	diffs = append(diffs, compareCollectionStates(scenario, collected, want)...)
	diffs = append(diffs, compareFindings(scenario, snap.Findings, want.Findings)...)
	diffs = append(diffs, compareRecommendations(scenario, snap.Recommendations, want.Recommendations)...)
	diffs = append(diffs, compareSummary(scenario, snap.Summary, want.Summary)...)
	diffs = append(diffs, compareCoverage(scenario, snap.Workloads, want.WorkloadCoverage)...)
	diffs = append(diffs, compareExecutive(scenario, snap, want.Executive)...)

	gotWarnings, gotNotices, err := originalTexts(snap.Quality.Warnings, snap.Quality.Notices)
	if err != nil {
		diffs = append(diffs, fmt.Sprintf("%s: rendering warnings and notices: %v", scenario, err))
		return diffs
	}
	diffs = append(diffs, compareStrings(scenario, "warnings", gotWarnings, want.Warnings)...)
	diffs = append(diffs, compareStrings(scenario, "notices", gotNotices, want.Notices)...)
	return diffs
}

// TestNumbersEqualTheOriginal runs each of the eight scenarios through
// Snapshot and compares the result with its recorded file of the original,
// row by row and number by number.
func TestNumbersEqualTheOriginal(t *testing.T) {
	for _, scenario := range promtest.Scenarios {
		t.Run(scenario.Name, func(t *testing.T) {
			path, err := pythonRecordingPath(scenario.Name)
			if err != nil {
				t.Fatalf("pythonRecordingPath(%s): %v", scenario.Name, err)
			}
			want, err := loadPythonRecording(path)
			if err != nil {
				t.Fatalf("loadPythonRecording(%s): %v", scenario.Name, err)
			}

			out, _, err := run(context.Background(), scenario)
			if err != nil {
				t.Fatalf("run(%s): %v", scenario.Name, err)
			}

			if diffs := compareNumbers(scenario.Name, out.Snapshot, out.Collected, want); len(diffs) > 0 {
				t.Fatalf("%s differs from the original:\n%s", scenario.Name, strings.Join(diffs, "\n"))
			}
		})
	}
}

// TestCollectionStatesEqualTheOriginal checks, for each of the eight
// scenarios, that the states of the five collectors equal the five
// collection states of the recorded file, and that the status of the
// cluster autoscaling evidence equals autoscaling_status.
func TestCollectionStatesEqualTheOriginal(t *testing.T) {
	for _, scenario := range promtest.Scenarios {
		t.Run(scenario.Name, func(t *testing.T) {
			path, err := pythonRecordingPath(scenario.Name)
			if err != nil {
				t.Fatalf("pythonRecordingPath(%s): %v", scenario.Name, err)
			}
			want, err := loadPythonRecording(path)
			if err != nil {
				t.Fatalf("loadPythonRecording(%s): %v", scenario.Name, err)
			}

			out, _, err := run(context.Background(), scenario)
			if err != nil {
				t.Fatalf("run(%s): %v", scenario.Name, err)
			}

			if out.Collected.Namespaces.State != want.NamespaceCollection {
				t.Errorf("%s: namespace collector state: got %q, want %q", scenario.Name, out.Collected.Namespaces.State, want.NamespaceCollection)
			}
			if out.Collected.Workloads.State != want.WorkloadCollection {
				t.Errorf("%s: workload collector state: got %q, want %q", scenario.Name, out.Collected.Workloads.State, want.WorkloadCollection)
			}
			if out.Collected.HPAs.State != want.HPACollection {
				t.Errorf("%s: hpa collector state: got %q, want %q", scenario.Name, out.Collected.HPAs.State, want.HPACollection)
			}
			if out.Collected.Coverage.State != want.HPACoverageCollection {
				t.Errorf("%s: hpa coverage collector state: got %q, want %q", scenario.Name, out.Collected.Coverage.State, want.HPACoverageCollection)
			}
			if out.Collected.Nodes.State != want.NodeCollection {
				t.Errorf("%s: node collector state: got %q, want %q", scenario.Name, out.Collected.Nodes.State, want.NodeCollection)
			}
			if out.Collected.Autoscaling.Status != want.AutoscalingStatus {
				t.Errorf("%s: autoscaling status: got %q, want %q", scenario.Name, out.Collected.Autoscaling.Status, want.AutoscalingStatus)
			}
		})
	}
}

// TestWindowEqualsTheOriginal checks, for each of the eight scenarios,
// that the start and the end of the query window, in seconds since the
// epoch, equal period_start and period_end of the recorded file, and that
// the step collect.Step gives for the period equals its step.
func TestWindowEqualsTheOriginal(t *testing.T) {
	for _, scenario := range promtest.Scenarios {
		t.Run(scenario.Name, func(t *testing.T) {
			path, err := pythonRecordingPath(scenario.Name)
			if err != nil {
				t.Fatalf("pythonRecordingPath(%s): %v", scenario.Name, err)
			}
			want, err := loadPythonRecording(path)
			if err != nil {
				t.Fatalf("loadPythonRecording(%s): %v", scenario.Name, err)
			}

			emittedAt, err := fixedEmittedAt()
			if err != nil {
				t.Fatalf("fixedEmittedAt: %v", err)
			}
			cfg := fixedConfig(scenario, "")
			window, err := collect.NewWindow(cfg.Analysis, emittedAt)
			if err != nil {
				t.Fatalf("NewWindow(%s): %v", scenario.Name, err)
			}
			period, err := cfg.Analysis.Duration()
			if err != nil {
				t.Fatalf("Duration(%s): %v", scenario.Name, err)
			}

			if got := window.Start.Unix(); got != want.PeriodStart {
				t.Errorf("%s: period start: got %d, want %d", scenario.Name, got, want.PeriodStart)
			}
			if got := window.End.Unix(); got != want.PeriodEnd {
				t.Errorf("%s: period end: got %d, want %d", scenario.Name, got, want.PeriodEnd)
			}
			if got := collect.Step(period); got != want.Step {
				t.Errorf("%s: step: got %q, want %q", scenario.Name, got, want.Step)
			}
		})
	}
}

// readQueriesLines reads the file at path and splits it into lines, one
// per request, the way queriesLines reads a project's own golden file.
func readQueriesLines(t *testing.T, path string) []string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	return strings.Split(strings.TrimRight(string(data), "\n"), "\n")
}

// TestQueriesEqualTheOriginal checks, for each of the eight scenarios,
// that the project's own golden queries file equals the recorded queries
// file of the original line for line, whole, and that neither is longer
// than the other.
func TestQueriesEqualTheOriginal(t *testing.T) {
	for _, scenario := range promtest.Scenarios {
		t.Run(scenario.Name, func(t *testing.T) {
			projectLines := queriesLines(t, scenario)
			originalPath, err := pythonQueriesPath(scenario.Name)
			if err != nil {
				t.Fatalf("pythonQueriesPath(%s): %v", scenario.Name, err)
			}
			originalLines := readQueriesLines(t, originalPath)

			if len(projectLines) != len(originalLines) {
				t.Errorf("%s: the project's list holds %d lines, the original's %d", scenario.Name, len(projectLines), len(originalLines))
			}
			for i := 0; i < len(projectLines) && i < len(originalLines); i++ {
				if projectLines[i] != originalLines[i] {
					t.Errorf("%s: query line %d: got %q, want %q", scenario.Name, i+1, projectLines[i], originalLines[i])
				}
			}
		})
	}
}

// sha256LineRe matches one line of the SHA-256 block of
// testdata/python/README.md: a lower-case hex digest, two spaces, and a
// file name.
var sha256LineRe = regexp.MustCompile(`^([0-9a-f]{64})  (\S+)$`)

// readSHA256Table reads the fenced block that follows "## SHA-256" in the
// README at path and returns the digest recorded for every file name it
// lists.
func readSHA256Table(t *testing.T, path string) map[string]string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}

	hashes := make(map[string]string)
	inBlock := false
	for _, line := range strings.Split(string(data), "\n") {
		switch {
		case line == "```text":
			inBlock = true
		case line == "```" && inBlock:
			inBlock = false
		case inBlock:
			m := sha256LineRe.FindStringSubmatch(line)
			if m == nil {
				t.Fatalf("%s: line %q inside the SHA-256 block does not hold a digest and a file name", path, line)
			}
			hashes[m[2]] = m[1]
		}
	}
	if len(hashes) == 0 {
		t.Fatalf("%s: found no SHA-256 block", path)
	}
	return hashes
}

// TestRecordedFilesAreUntouched checks that every file of
// testdata/python/, other than README.md itself, has the SHA-256 the
// README records for it, that every file the README names is present, and
// that no file of the folder is left unnamed.
func TestRecordedFilesAreUntouched(t *testing.T) {
	root, err := moduleRoot()
	if err != nil {
		t.Fatalf("moduleRoot: %v", err)
	}
	dir := filepath.Join(root, "testdata", "python")

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("reading %s: %v", dir, err)
	}
	present := make(map[string]bool, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || entry.Name() == "README.md" {
			continue
		}
		present[entry.Name()] = true
	}

	hashes := readSHA256Table(t, filepath.Join(dir, "README.md"))

	for name := range hashes {
		if !present[name] {
			t.Errorf("README.md names %s, which testdata/python/ does not hold", name)
		}
	}
	for name := range present {
		if _, ok := hashes[name]; !ok {
			t.Errorf("testdata/python/ holds %s, which README.md does not name", name)
		}
	}
	for name, want := range hashes {
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Errorf("reading %s: %v", name, err)
			continue
		}
		sum := sha256.Sum256(data)
		if got := hex.EncodeToString(sum[:]); got != want {
			t.Errorf("%s: sha-256 is %s, want %s", name, got, want)
		}
	}
}

// TestComparisonCanFail proves, on the edge-values scenario, that
// compareNumbers reports each of a list of single changes made to a copy
// of the recorded file held in memory, and that it reports exactly the
// differences the change makes, in order, and nothing beside them.
func TestComparisonCanFail(t *testing.T) {
	scenario := scenarioByName(t, "edge-values")

	out, _, err := run(context.Background(), scenario)
	if err != nil {
		t.Fatalf("run(%s): %v", scenario.Name, err)
	}
	snap, collected := out.Snapshot, out.Collected

	path, err := pythonRecordingPath(scenario.Name)
	if err != nil {
		t.Fatalf("pythonRecordingPath(%s): %v", scenario.Name, err)
	}

	fresh := func(t *testing.T) pythonRecording {
		t.Helper()
		want, err := loadPythonRecording(path)
		if err != nil {
			t.Fatalf("loadPythonRecording(%s): %v", path, err)
		}
		return want
	}

	// checkExact requires compareNumbers to report exactly want, in order,
	// and nothing else.
	checkExact := func(t *testing.T, recording pythonRecording, want []string) {
		t.Helper()
		got := compareNumbers(scenario.Name, snap, collected, recording)
		if !slices.Equal(got, want) {
			t.Errorf("differences:\n got:  %s\n want: %s", strings.Join(got, "\n       "), strings.Join(want, "\n       "))
		}
	}

	t.Run("a number of a statistic changed", func(t *testing.T) {
		want := fresh(t)
		want.Namespaces[0].MemoryUsage.Average = floatPtr(106)
		checkExact(t, want, []string{
			namespaceLabel(scenario.Name, "payments") + ": memory_usage.average: got 105, want 106",
		})
	})

	t.Run("a statistic changed from a number to null", func(t *testing.T) {
		want := fresh(t)
		want.Namespaces[0].MemoryUsage.Average = nil
		checkExact(t, want, []string{
			namespaceLabel(scenario.Name, "payments") + ": memory_usage.average: got 105, want null",
		})
	})

	t.Run("no_base changed from true to false", func(t *testing.T) {
		want := fresh(t)
		want.Namespaces[0].CPURequestPercent.NoBase = false
		checkExact(t, want, []string{
			namespaceLabel(scenario.Name, "payments") + `: cpu_request_percent.state: got "no_base", want "unavailable"`,
		})
	})

	t.Run("a pod count changed", func(t *testing.T) {
		want := fresh(t)
		*want.Namespaces[0].ActivePods = 99
		checkExact(t, want, []string{
			namespaceLabel(scenario.Name, "payments") + ": pods: got 3, want 99",
		})
	})

	t.Run("approximate_ratios changed", func(t *testing.T) {
		want := fresh(t)
		want.Namespaces[0].ApproximateRatios = !want.Namespaces[0].ApproximateRatios
		checkExact(t, want, []string{
			namespaceLabel(scenario.Name, "payments") + ": approximate: got true, want false",
		})
	})

	t.Run("requests_limits_status changed", func(t *testing.T) {
		want := fresh(t)
		want.Workloads[0].RequestsLimitsStatus = "partial"
		checkExact(t, want, []string{
			workloadLabel(scenario.Name, "payments", "Deployment", "api") + `: requests_limits_status: got "available", want "partial"`,
		})
	})

	t.Run("a workload removed", func(t *testing.T) {
		want := fresh(t)
		want.Workloads = want.Workloads[:1]
		checkExact(t, want, []string{
			workloadLabel(scenario.Name, "payments", "StatefulSet", "queue") + ": present in the project, missing from the original",
		})
	})

	t.Run("a finding removed", func(t *testing.T) {
		want := fresh(t)
		removed := want.Findings[0]
		want.Findings = append([]pythonFinding{}, want.Findings[1:]...)
		key := findingKey{removed.Code, removed.ScopeType, removed.ScopeName, removed.ScopeKind}
		checkExact(t, want, []string{
			findingLabel(scenario.Name, key) + ": present in the project, missing from the original",
		})
	})

	t.Run("the code of a finding changed", func(t *testing.T) {
		want := fresh(t)
		original := want.Findings[0]
		want.Findings[0].Code = original.Code + "_X"
		originalKey := findingKey{original.Code, original.ScopeType, original.ScopeName, original.ScopeKind}
		changedKey := findingKey{original.Code + "_X", original.ScopeType, original.ScopeName, original.ScopeKind}
		checkExact(t, want, []string{
			findingLabel(scenario.Name, originalKey) + ": present in the project, missing from the original",
			findingLabel(scenario.Name, changedKey) + ": present in the original, missing from the project",
		})
	})

	t.Run("a number of the evidence of a finding changed", func(t *testing.T) {
		want := fresh(t)
		f := want.Findings[0]
		key := findingKey{f.Code, f.ScopeType, f.ScopeName, f.ScopeKind}
		want.Findings[0].Evidence[0].Value = analysis.Number(56)
		checkExact(t, want, []string{
			findingLabel(scenario.Name, key) + ": evidence[0].average_percent: got number 55, want number 56",
		})
	})

	t.Run("the confidence of a finding changed", func(t *testing.T) {
		want := fresh(t)
		idx := -1
		for i := range want.Findings {
			if want.Findings[i].Confidence == "inferred" {
				idx = i
				break
			}
		}
		if idx < 0 {
			t.Fatal("edge-values holds no finding of confidence inferred to change")
		}
		f := want.Findings[idx]
		key := findingKey{f.Code, f.ScopeType, f.ScopeName, f.ScopeKind}
		want.Findings[idx].Confidence = "exact"
		checkExact(t, want, []string{
			findingLabel(scenario.Name, key) + `: confidence: got "inferred", want "exact"`,
		})
	})

	t.Run("the scope type of a finding changed", func(t *testing.T) {
		want := fresh(t)
		idx := -1
		for i := range want.Findings {
			if want.Findings[i].ScopeType == "namespace" {
				idx = i
				break
			}
		}
		if idx < 0 {
			t.Fatal("edge-values holds no finding of scope type namespace to change")
		}
		f := want.Findings[idx]
		key := findingKey{f.Code, f.ScopeType, f.ScopeName, f.ScopeKind}
		changedKey := findingKey{f.Code, "workload", f.ScopeName, f.ScopeKind}
		want.Findings[idx].ScopeType = "workload"
		checkExact(t, want, []string{
			findingLabel(scenario.Name, key) + ": present in the project, missing from the original",
			findingLabel(scenario.Name, changedKey) + ": present in the original, missing from the project",
		})
	})

	t.Run("two findings exchanged", func(t *testing.T) {
		want := fresh(t)
		first, second := want.Findings[0], want.Findings[1]
		want.Findings[0], want.Findings[1] = second, first
		firstKey := findingOrderKey{first.Code, first.ScopeType, first.ScopeName}
		secondKey := findingOrderKey{second.Code, second.ScopeType, second.ScopeName}
		checkExact(t, want, []string{
			fmt.Sprintf("%s: findings[0]: the order differs from the original: got %v, want %v", scenario.Name, firstKey, secondKey),
		})
	})

	t.Run("a notice removed", func(t *testing.T) {
		want := fresh(t)
		removedText := want.Notices[1]
		want.Notices = want.Notices[:1]
		checkExact(t, want, []string{
			fmt.Sprintf("%s: notices[1]: got %q, want %q", scenario.Name, removedText, ""),
		})
	})

	t.Run("one letter of a notice changed", func(t *testing.T) {
		want := fresh(t)
		original := want.Notices[0]
		mutated := strings.Replace(original, "carga", "xarga", 1)
		want.Notices[0] = mutated
		checkExact(t, want, []string{
			fmt.Sprintf("%s: notices[0]: got %q, want %q", scenario.Name, original, mutated),
		})
	})

	scaling := scenarioByName(t, "edge-scaling")
	scalingOut, _, err := run(context.Background(), scaling)
	if err != nil {
		t.Fatalf("run(%s): %v", scaling.Name, err)
	}
	scalingPath, err := pythonRecordingPath(scaling.Name)
	if err != nil {
		t.Fatalf("pythonRecordingPath(%s): %v", scaling.Name, err)
	}
	freshScaling := func(t *testing.T) pythonRecording {
		t.Helper()
		want, err := loadPythonRecording(scalingPath)
		if err != nil {
			t.Fatalf("loadPythonRecording(%s): %v", scalingPath, err)
		}
		return want
	}
	checkScaling := func(t *testing.T, recording pythonRecording, want []string) {
		t.Helper()
		got := compareNumbers(scaling.Name, scalingOut.Snapshot, scalingOut.Collected, recording)
		if !slices.Equal(got, want) {
			t.Errorf("differences:\n got:  %s\n want: %s", strings.Join(got, "\n       "), strings.Join(want, "\n       "))
		}
	}

	t.Run("a number of an hpa changed", func(t *testing.T) {
		want := freshScaling(t)
		idx := slices.IndexFunc(want.HPAs, func(h pythonHPA) bool { return h.Name == "web" })
		if idx < 0 {
			t.Fatal("edge-scaling holds no HPA named web")
		}
		original := *want.HPAs[idx].Current.Peak
		want.HPAs[idx].Current.Peak = floatPtr(original + 1)
		checkScaling(t, want, []string{
			hpaLabel(scaling.Name, "payments", "web") + fmt.Sprintf(": current.peak: got %v, want %v", original, original+1),
		})
	})

	t.Run("two hpas exchanged", func(t *testing.T) {
		want := freshScaling(t)
		if len(want.HPAs) < 2 {
			t.Fatal("edge-scaling holds fewer than two HPAs")
		}
		type hpaKey struct{ Namespace, Name string }
		var original, exchanged []hpaKey
		for _, h := range want.HPAs {
			original = append(original, hpaKey{h.Namespace, h.Name})
		}
		want.HPAs[0], want.HPAs[1] = want.HPAs[1], want.HPAs[0]
		for _, h := range want.HPAs {
			exchanged = append(exchanged, hpaKey{h.Namespace, h.Name})
		}
		checkScaling(t, want, []string{
			fmt.Sprintf("%s: hpas: the order differs from the original: got %v, want %v", scaling.Name, original, exchanged),
		})
	})

	t.Run("the spec of an hpa changed to null", func(t *testing.T) {
		want := freshScaling(t)
		idx := slices.IndexFunc(want.HPAs, func(h pythonHPA) bool { return h.Name == "fixed" })
		if idx < 0 {
			t.Fatal("edge-scaling holds no HPA named fixed")
		}
		want.HPAs[idx].SpecMax = nil
		checkScaling(t, want, []string{
			hpaLabel(scaling.Name, "payments", "fixed") + ": spec_max: got 3, want null",
		})
	})

	t.Run("a workload without an hpa changed", func(t *testing.T) {
		want := freshScaling(t)
		want.WorkloadsWithoutHPA[1].Kind = "Deployment"
		checkScaling(t, want, []string{
			scaling.Name + `: workloads_without_hpa[1]: kind: got "StatefulSet", want "Deployment"`,
		})
	})

	t.Run("a number of a node changed", func(t *testing.T) {
		want := freshScaling(t)
		idx := slices.IndexFunc(want.Nodes, func(n pythonNode) bool { return n.Name == "pool-a-1" })
		if idx < 0 {
			t.Fatal("edge-scaling holds no node named pool-a-1")
		}
		original := *want.Nodes[idx].CPURequestPercent
		want.Nodes[idx].CPURequestPercent = floatPtr(original + 1)
		checkScaling(t, want, []string{
			nodeLabel(scaling.Name, "pool-a-1") + fmt.Sprintf(": cpu_request_percent: got %v, want %v", original, original+1),
		})
	})

	t.Run("the readiness of a node changed", func(t *testing.T) {
		want := freshScaling(t)
		idx := slices.IndexFunc(want.Nodes, func(n pythonNode) bool { return n.Name == "pool-a-1" })
		if idx < 0 {
			t.Fatal("edge-scaling holds no node named pool-a-1")
		}
		flipped := want.Nodes[idx].Ready == nil || !*want.Nodes[idx].Ready
		want.Nodes[idx].Ready = &flipped
		got := compareNumbers(scaling.Name, scalingOut.Snapshot, scalingOut.Collected, want)
		if len(got) != 1 || !strings.HasPrefix(got[0], nodeLabel(scaling.Name, "pool-a-1")+": ready: ") {
			t.Errorf("differences: %q, want one difference naming node pool-a-1 and ready", got)
		}
	})

	t.Run("a nodepool changed", func(t *testing.T) {
		want := freshScaling(t)
		want.Nodepools[1].Nodes = want.Nodepools[1].Nodes[:1]
		checkScaling(t, want, []string{
			fmt.Sprintf("%s: nodes[1]: got %q, want %q", nodepoolLabel(scaling.Name, "pool-a"), "pool-a-2", ""),
		})
	})

	t.Run("a number of the autoscaling changed", func(t *testing.T) {
		want := freshScaling(t)
		want.Autoscaling.MaxNodes = floatPtr(4)
		checkScaling(t, want, []string{
			scaling.Name + ": autoscaling: max_nodes: got 3, want 4",
		})
	})

	t.Run("node pressure changed", func(t *testing.T) {
		want := freshScaling(t)
		want.NodePressure = false
		checkScaling(t, want, []string{
			scaling.Name + ": node_pressure: got true, want false",
		})
	})

	t.Run("a collection state changed", func(t *testing.T) {
		want := freshScaling(t)
		want.NodeCollection = "available"
		checkScaling(t, want, []string{
			scaling.Name + `: node_collection: got "partial", want "available"`,
		})
	})

	t.Run("a text of an evidence changed", func(t *testing.T) {
		want := freshScaling(t)
		idx := slices.IndexFunc(want.Findings, func(f pythonFinding) bool { return f.Code == "NODE_OVERREQUESTED" })
		if idx < 0 {
			t.Fatal("edge-scaling holds no finding NODE_OVERREQUESTED")
		}
		f := want.Findings[idx]
		key := findingKey{f.Code, f.ScopeType, f.ScopeName, f.ScopeKind}
		want.Findings[idx].Evidence[0].Value = analysis.Text("memory")
		checkScaling(t, want, []string{
			findingLabel(scaling.Name, key) + `: evidence[0].resources: got text "cpu", want text "memory"`,
		})
	})

	t.Run("a scope of a recommendation renamed", func(t *testing.T) {
		want := freshScaling(t)
		idx := slices.IndexFunc(want.Recommendations, func(r pythonRecommendation) bool {
			return r.Code == "WORKLOAD_MEMORY_LIMIT_CRITICAL"
		})
		if idx < 0 {
			t.Fatal("edge-scaling holds no recommendation WORKLOAD_MEMORY_LIMIT_CRITICAL")
		}
		want.Recommendations[idx].Scopes = []string{"payments/other"}
		checkScaling(t, want, []string{
			scaling.Name + ": recommendation WORKLOAD_MEMORY_LIMIT_CRITICAL (workload): scopes: got [payments/payments-web], want [payments/other]",
		})
	})

	t.Run("a count of the summary changed", func(t *testing.T) {
		want := freshScaling(t)
		want.Summary.Recommendations = 20
		checkScaling(t, want, []string{
			scaling.Name + ": summary.recommendations: got 19, want 20",
		})
	})

	t.Run("a coverage of a row changed", func(t *testing.T) {
		want := freshScaling(t)
		idx := slices.IndexFunc(want.WorkloadCoverage, func(r pythonCoverageRow) bool { return r.Name == "api" })
		if idx < 0 {
			t.Fatal("edge-scaling holds no coverage row named api")
		}
		want.WorkloadCoverage[idx].HPACoverage = "covered"
		checkScaling(t, want, []string{
			scaling.Name + ": coverage payments/Deployment/api: hpa_coverage: got \"uncovered\", want \"covered\"",
		})
	})

	t.Run("an ident of the action plan renamed", func(t *testing.T) {
		want := freshScaling(t)
		plan := want.Executive["attention"].ActionPlan
		plan[0][3][0].Text = "other"
		checkScaling(t, want, []string{
			scaling.Name + ": executive.attention.action_plan: row 0, cell 3, run 1: text: got \"payments/web\", want \"other\"",
		})
	})

	t.Run("a number of a replicas cell changed", func(t *testing.T) {
		want := freshScaling(t)
		rows := want.Executive["attention"].ReplicaCeiling
		rows[0][3][0].Text = "5 observadas / máximo 4"
		checkScaling(t, want, []string{
			scaling.Name + `: executive.attention.replica_ceiling: row 0, cell 3, run 0: text: got "4 observados / máximo 4" as sections.executive.replicas_observed(Max="4", Peak="4"), want "5 observadas / máximo 4" as sections.executive.replicas_observed(Max="4", Peak="5")`,
		})
	})

	t.Run("the note of the hpa data changed", func(t *testing.T) {
		want := freshScaling(t)
		e := want.Executive["attention"]
		e.HPANote = "incomplete"
		want.Executive["attention"] = e
		checkScaling(t, want, []string{
			scaling.Name + `: executive.attention: hpa_note: got "partial", want "incomplete"`,
		})
	})

	t.Run("the two minimums swapped", func(t *testing.T) {
		want := freshScaling(t)
		want.Executive["attention"], want.Executive["critical"] = want.Executive["critical"], want.Executive["attention"]
		checkScaling(t, want, []string{
			scaling.Name + `: executive.attention.replica_ceiling: rows: got 2, want 1`,
			scaling.Name + `: executive.attention.action_plan: rows: got 10, want 7`,
			scaling.Name + `: executive.critical.replica_ceiling: rows: got 1, want 2`,
			scaling.Name + `: executive.critical.action_plan: rows: got 7, want 10`,
		})
	})

	t.Run("two recommendations swapped", func(t *testing.T) {
		want := freshScaling(t)
		want.Recommendations[0], want.Recommendations[1] = want.Recommendations[1], want.Recommendations[0]
		checkScaling(t, want, []string{
			scaling.Name + `: recommendations[0]: the order differs from the original: got {NODE_CPU_USAGE_CRITICAL node}, want {WORKLOAD_MEMORY_LIMIT_CRITICAL workload}`,
		})
	})

	t.Run("the kind of a run changed", func(t *testing.T) {
		want := freshScaling(t)
		want.Executive["attention"].ActionPlan[0][0][0].Kind = "placeholder"
		checkScaling(t, want, []string{
			scaling.Name + `: executive.attention.action_plan: row 0, cell 0, run 0: kind: got "text", want "placeholder"`,
		})
	})

	t.Run("the status of a run changed", func(t *testing.T) {
		want := freshScaling(t)
		want.Executive["attention"].NodepoolStatus[0][1][0].Status = "attention"
		checkScaling(t, want, []string{
			scaling.Name + `: executive.attention.nodepool_status: row 0, cell 1, run 0: status: got "critical", want "attention"`,
		})
	})

	t.Run("the executive entry of a minimum removed", func(t *testing.T) {
		want := freshScaling(t)
		delete(want.Executive, "critical")
		checkScaling(t, want, []string{
			scaling.Name + `: executive.critical: missing from the original`,
		})
	})

	t.Run("a row removed from a table", func(t *testing.T) {
		want := freshScaling(t)
		e := want.Executive["attention"]
		e.NodepoolStatus = e.NodepoolStatus[:2]
		want.Executive["attention"] = e
		checkScaling(t, want, []string{
			scaling.Name + `: executive.attention.nodepool_status: rows: got 3, want 2`,
		})
	})

	t.Run("the actions of two plan rows swapped", func(t *testing.T) {
		want := freshScaling(t)
		plan := want.Executive["attention"].ActionPlan
		plan[0][2], plan[1][2] = plan[1][2], plan[0][2]
		checkScaling(t, want, []string{
			scaling.Name + `: executive.attention.action_plan: row 0, cell 2, run 0: text: got "Revisar o máximo configurado e a capacidade do workload." as finding.hpa_max_reached.action or front.metadata.period_range(End="capacidade do workload.", Start="Revisar o máximo configurado e"), want "Revisar o limite de CPU e o perfil de consumo observado." as finding.resource_limit_usage.cpu.action`,
			scaling.Name + `: executive.attention.action_plan: row 1, cell 2, run 0: text: got "Revisar o limit de CPU e o perfil de consumo observado." as finding.resource_limit_usage.cpu.action, want "Revisar o limite máximo e a capacidade da carga de trabalho." as finding.hpa_max_reached.action`,
		})
	})

	t.Run("a recorded scope names another object", func(t *testing.T) {
		want := freshScaling(t)
		want.Executive["attention"].ActionPlan[2][3][0].Text = "payments-api"
		checkScaling(t, want, []string{scaling.Name + `: executive.attention.action_plan: row 2, cell 3, run 1: text: got "payments", want "payments-api"`})
	})

	t.Run("an unknown trailing parenthesis is not stripped", func(t *testing.T) {
		want := freshScaling(t)
		want.Executive["attention"].ActionPlan[1][2][0].Text += " (x)"
		checkScaling(t, want, []string{scaling.Name + `: executive.attention.action_plan: row 1, cell 2, run 0: text: got "Revisar o limit de CPU e o perfil de consumo observado." as finding.resource_limit_usage.cpu.action, want "Revisar o limite de CPU e o perfil de consumo observado. (x)" with no catalog key`})
	})

	t.Run("a recorded action differs after its qualifier is stripped", func(t *testing.T) {
		want := freshScaling(t)
		want.Executive["attention"].ActionPlan[2][2][0].Text = "Revisar outra coisa. (namespaces)"
		checkScaling(t, want, []string{scaling.Name + `: executive.attention.action_plan: row 2, cell 2, run 0: text: got "Revisar o request de CPU e o consumo observado." as finding.resource_request_usage.cpu.action, want "Revisar outra coisa." with no catalog key`})
	})

	t.Run("a kind of the replica ceiling changed", func(t *testing.T) {
		want := freshScaling(t)
		want.Executive["attention"].ReplicaCeiling[0][2][0].Text = "StatefulSet"
		checkScaling(t, want, []string{
			scaling.Name + `: executive.attention.replica_ceiling: row 0, cell 2, run 0: text: got "Deployment", want "StatefulSet"`,
		})
	})

	t.Run("the mark of a limitation coverage cell changed", func(t *testing.T) {
		want := freshScaling(t)
		want.Executive["attention"].MaterialLimitations[1][1][0].Status = "healthy"
		checkScaling(t, want, []string{
			scaling.Name + `: executive.attention.material_limitations: row 1, cell 1, run 0: status: got "attention", want "healthy"`,
		})
	})

	t.Run("a critical mark in a limitation coverage cell reads as unavailable", func(t *testing.T) {
		want := freshScaling(t)
		want.Executive["attention"].MaterialLimitations[1][1][0].Status = "critical"
		checkScaling(t, want, []string{
			scaling.Name + `: executive.attention.material_limitations: row 1, cell 1, run 0: status: got "attention", want "unavailable"`,
		})
	})

	t.Run("two limitation labels swapped", func(t *testing.T) {
		want := freshScaling(t)
		rows := want.Executive["attention"].MaterialLimitations
		rows[0][0], rows[1][0] = rows[1][0], rows[0][0]
		checkScaling(t, want, []string{
			scaling.Name + `: executive.attention.material_limitations: row 0, cell 0, run 0: text: got "Histórico de HPA" as capability.name.hpa_history or sections.hpas.heading, want "Interpretação" as sections.executive.interpretation`,
			scaling.Name + `: executive.attention.material_limitations: row 1, cell 0, run 0: text: got "Interpretação" as front.legend.header_meaning or sections.executive.interpretation, want "Histórico de HPA" as capability.name.hpa_history`,
		})
	})
}

// TestOriginalSentencesCoverEveryCode wants a sentence of the original for
// every warning code, every reason code, and both forms of every notice
// code, so that the comparison with the recorded files never passes by
// comparing nothing.
func TestOriginalSentencesCoverEveryCode(t *testing.T) {
	for _, code := range collect.WarningCodes {
		if _, err := originalWarningText(code); err != nil {
			t.Errorf("warning %s: %v", code, err)
		}
	}
	for _, code := range discovery.ReasonCodes {
		if _, err := originalWarningText(code); err != nil {
			t.Errorf("reason %s: %v", code, err)
		}
	}
	for _, code := range collect.NoticeCodes {
		for _, count := range []float64{1, 2} {
			if _, err := originalNoticeText(code, count); err != nil {
				t.Errorf("notice %s with count %v: %v", code, count, err)
			}
		}
	}
	if _, err := originalWarningText("NO_SUCH_WARNING"); err == nil {
		t.Error("originalWarningText(NO_SUCH_WARNING) = nil error, want one")
	}
	if _, err := originalNoticeText("NO_SUCH_NOTICE", 1); err == nil {
		t.Error("originalNoticeText(NO_SUCH_NOTICE, 1) = nil error, want one")
	}
}

// TestOriginalNoticeTextWritesTheCount pins the singular form for a count
// of one and the plural form with the count written as Python writes an
// integer for any other count.
func TestOriginalNoticeTextWritesTheCount(t *testing.T) {
	tests := []struct {
		code  string
		count float64
		want  string
	}{
		{collect.NoticeWorkloadsOmittedNoUsage, 1, "1 carga de trabalho deixada de fora por não ter consumo observado no período."},
		{collect.NoticeWorkloadsOmittedNoUsage, 3, "3 cargas de trabalho deixadas de fora por não terem consumo observado no período."},
		{collect.NoticeJobsOmitted, 1, "1 Job foi deixado de fora do inventário de cargas de trabalho; o consumo continua contabilizado no total do namespace."},
		{collect.NoticeJobsOmitted, 12, "12 Jobs foram deixados de fora do inventário de cargas de trabalho; o consumo continua contabilizado nos totais dos namespaces."},
	}
	for _, tt := range tests {
		got, err := originalNoticeText(tt.code, tt.count)
		if err != nil {
			t.Fatalf("originalNoticeText(%s, %v): %v", tt.code, tt.count, err)
		}
		if got != tt.want {
			t.Errorf("originalNoticeText(%s, %v) = %q, want %q", tt.code, tt.count, got, tt.want)
		}
	}
}

func TestPythonEvidenceReadsThreeKinds(t *testing.T) {
	cases := []struct {
		in   string
		want pythonEvidencePair
	}{
		{`["node_pressure", true]`, pythonEvidencePair{"node_pressure", analysis.Boolean(true)}},
		{`["resources", "cpu,memory"]`, pythonEvidencePair{"resources", analysis.Text("cpu,memory")}},
		{`["affected_nodes", 1]`, pythonEvidencePair{"affected_nodes", analysis.Number(1)}},
		{`["average", 1.5]`, pythonEvidencePair{"average", analysis.Number(1.5)}},
	}
	for _, c := range cases {
		var got pythonEvidencePair
		if err := json.Unmarshal([]byte(c.in), &got); err != nil {
			t.Errorf("Unmarshal(%s) error = %v", c.in, err)
			continue
		}
		if got != c.want {
			t.Errorf("Unmarshal(%s) = %+v, want %+v", c.in, got, c.want)
		}
	}
}
