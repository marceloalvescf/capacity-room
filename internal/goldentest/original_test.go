package goldentest

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/marceloalvescf/k8s-capacity-report/internal/catalog"
	"github.com/marceloalvescf/k8s-capacity-report/internal/collect"
	"github.com/marceloalvescf/k8s-capacity-report/internal/discovery"
	"github.com/marceloalvescf/k8s-capacity-report/internal/prom"
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

// pythonEvidencePair is one named number of a recorded finding's evidence.
// The pair "approximate" carries a boolean rather than a number, so both
// are read, and IsBool says which one is meaningful.
type pythonEvidencePair struct {
	Name   string
	Number float64
	Bool   bool
	IsBool bool
}

// UnmarshalJSON reads a pair written as a two-element JSON array: a name,
// then a number or a boolean.
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
	if err := json.Unmarshal(raw[1], &p.Number); err == nil {
		return nil
	}
	if err := json.Unmarshal(raw[1], &p.Bool); err != nil {
		return fmt.Errorf("evidence pair value is neither a number nor a boolean: %w", err)
	}
	p.IsBool = true
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

// pythonRecording is one decoded testdata/python/<scenario>.json file.
type pythonRecording struct {
	Scenario            string               `json:"scenario"`
	Source              string               `json:"source"`
	PeriodStart         int64                `json:"period_start"`
	PeriodEnd           int64                `json:"period_end"`
	Step                string               `json:"step"`
	NamespaceCollection string               `json:"namespace_collection"`
	WorkloadCollection  string               `json:"workload_collection"`
	Namespaces          []pythonNamespaceRow `json:"namespaces"`
	Workloads           []pythonWorkloadRow  `json:"workloads"`
	Findings            []pythonFinding      `json:"findings"`
	Warnings            []string             `json:"warnings"`
	Notices             []string             `json:"notices"`
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

// compareNamespaceRow compares one namespace row, field by field. The
// snapshot does not store requests_limits_status, so it is not compared
// here.
func compareNamespaceRow(scenario string, got snapshot.Namespace, want pythonNamespaceRow) []string {
	label := fmt.Sprintf("%s: namespace %s", scenario, got.Name)
	var diffs []string
	diffs = append(diffs, comparePointer(label, "pods", got.Pods, want.ActivePods)...)
	if got.Approximate != want.ApproximateRatios {
		diffs = append(diffs, fmt.Sprintf("%s: approximate: got %v, want %v", label, got.Approximate, want.ApproximateRatios))
	}
	diffs = append(diffs, compareTenStatistics(label, got, want)...)
	return diffs
}

// workloadLabel names a workload row for a difference message.
func workloadLabel(scenario, namespace, kind, name string) string {
	return fmt.Sprintf("%s: workload %s/%s (%s)", scenario, namespace, name, kind)
}

// compareWorkloadRow compares one workload row, field by field. The
// snapshot does not store requests_limits_status, so it is not compared
// here.
func compareWorkloadRow(scenario string, got snapshot.Workload, want pythonWorkloadRow) []string {
	label := workloadLabel(scenario, got.Namespace, got.Kind, got.Name)
	var diffs []string
	diffs = append(diffs, comparePointer(label, "pods", got.Pods, want.ActivePods)...)
	if got.Approximate != want.ApproximateRatios {
		diffs = append(diffs, fmt.Sprintf("%s: approximate: got %v, want %v", label, got.Approximate, want.ApproximateRatios))
	}
	diffs = append(diffs, compareWorkloadStatistics(label, got, want)...)
	return diffs
}

// compareNamespaces matches the namespaces of a snapshot with those of a
// recorded file by name, and compares each pair found on both sides.
func compareNamespaces(scenario string, got []snapshot.Namespace, want []pythonNamespaceRow) []string {
	var diffs []string
	wantByName := make(map[string]pythonNamespaceRow, len(want))
	for _, w := range want {
		wantByName[w.Namespace] = w
	}
	seen := make(map[string]bool, len(got))
	for _, g := range got {
		seen[g.Name] = true
		w, ok := wantByName[g.Name]
		if !ok {
			diffs = append(diffs, fmt.Sprintf("%s: namespace %s: present in the project, missing from the original", scenario, g.Name))
			continue
		}
		diffs = append(diffs, compareNamespaceRow(scenario, g, w)...)
	}
	for _, w := range want {
		if !seen[w.Namespace] {
			diffs = append(diffs, fmt.Sprintf("%s: namespace %s: present in the original, missing from the project", scenario, w.Namespace))
		}
	}
	return diffs
}

// workloadRowKey identifies a workload row by namespace, kind, and name.
type workloadRowKey struct{ Namespace, Kind, Name string }

// compareWorkloads matches the workloads of a snapshot with those of a
// recorded file by namespace, kind, and name, and compares each pair found
// on both sides.
func compareWorkloads(scenario string, got []snapshot.Workload, want []pythonWorkloadRow) []string {
	var diffs []string
	wantByKey := make(map[workloadRowKey]pythonWorkloadRow, len(want))
	for _, w := range want {
		wantByKey[workloadRowKey{w.Namespace, w.Kind, w.Name}] = w
	}
	seen := make(map[workloadRowKey]bool, len(got))
	for _, g := range got {
		key := workloadRowKey{g.Namespace, g.Kind, g.Name}
		seen[key] = true
		w, ok := wantByKey[key]
		if !ok {
			diffs = append(diffs, fmt.Sprintf("%s: present in the project, missing from the original", workloadLabel(scenario, g.Namespace, g.Kind, g.Name)))
			continue
		}
		diffs = append(diffs, compareWorkloadRow(scenario, g, w)...)
	}
	for _, w := range want {
		key := workloadRowKey{w.Namespace, w.Kind, w.Name}
		if !seen[key] {
			diffs = append(diffs, fmt.Sprintf("%s: present in the original, missing from the project", workloadLabel(scenario, w.Namespace, w.Kind, w.Name)))
		}
	}
	return diffs
}

// findingKey identifies a finding by code, scope name, and scope kind, the
// keying the recorded file's set of findings is compared by.
type findingKey struct{ Code, ScopeName, ScopeKind string }

// findingLabel names a finding for a difference message.
func findingLabel(scenario string, key findingKey) string {
	return fmt.Sprintf("%s: finding %s %s (%s)", scenario, key.Code, key.ScopeName, key.ScopeKind)
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
		if got[i].Value != filtered[i].Number {
			diffs = append(diffs, fmt.Sprintf("%s: evidence[%d].%s: got %v, want %v", label, i, got[i].Name, got[i].Value, filtered[i].Number))
		}
	}
	return diffs
}

// compareFindings compares the findings of a snapshot with those of a
// recorded file: first as sets keyed by code, scope name, and scope kind,
// so that a missing finding is named as missing and not as a shift of
// every finding after it; then, for a key found on both sides, severity,
// confidence, and evidence; then the order of the two lists of keys.
func compareFindings(scenario string, got []snapshot.Finding, want []pythonFinding) []string {
	var diffs []string

	gotByKey := make(map[findingKey]snapshot.Finding, len(got))
	gotOrder := make([]findingKey, len(got))
	for i, f := range got {
		key := findingKey{f.Code, f.ScopeName, f.ScopeKind}
		gotByKey[key] = f
		gotOrder[i] = key
	}
	wantByKey := make(map[findingKey]pythonFinding, len(want))
	wantOrder := make([]findingKey, len(want))
	for i, f := range want {
		key := findingKey{f.Code, f.ScopeName, f.ScopeKind}
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

	if sameKeySet(gotOrder, wantOrder) && !slices.Equal(gotOrder, wantOrder) {
		diffs = append(diffs, fmt.Sprintf("%s: findings: the order differs from the original: got %v, want %v", scenario, gotOrder, wantOrder))
	}

	return diffs
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

// catalogTexts renders the sentences of the snapshot's warnings and
// notices with cat, following buildNotes of internal/document: each
// warning's code with no parameter, each notice's code and count, the
// parameter Count filled in for the entry that takes it.
func catalogTexts(cat *catalog.Catalog, warnings, notices []snapshot.Message) ([]string, []string, error) {
	warningTexts := make([]string, len(warnings))
	for i, w := range warnings {
		text, err := cat.Text(catalog.Warning(w.Code), nil)
		if err != nil {
			return nil, nil, fmt.Errorf("warning %s: %w", w.Code, err)
		}
		warningTexts[i] = text
	}
	noticeTexts := make([]string, len(notices))
	for i, n := range notices {
		count := n.Params["count"]
		var params catalog.Params
		if count != 1 {
			params = catalog.Params{"Count": cat.Count(count)}
		}
		text, err := cat.Text(catalog.Notice(n.Code, count), params)
		if err != nil {
			return nil, nil, fmt.Errorf("notice %s: %w", n.Code, err)
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

// compareNumbers compares snap, a decoded snapshot of scenario, with want,
// a decoded recorded file of the original, and returns every difference
// found: a namespace row, a workload row, a finding, a warning, or a
// notice present on one side and not the other, or a field of a matched
// row or finding that disagrees, each naming the scenario, the row or
// finding, and the field, with both values.
func compareNumbers(scenario string, snap snapshot.Snapshot, want pythonRecording) []string {
	cat, err := catalog.Load(catalog.PortugueseBrazil)
	if err != nil {
		return []string{fmt.Sprintf("%s: loading the pt-BR catalog: %v", scenario, err)}
	}

	var diffs []string
	diffs = append(diffs, compareNamespaces(scenario, snap.Namespaces, want.Namespaces)...)
	diffs = append(diffs, compareWorkloads(scenario, snap.Workloads, want.Workloads)...)
	diffs = append(diffs, compareFindings(scenario, snap.Findings, want.Findings)...)

	gotWarnings, gotNotices, err := catalogTexts(cat, snap.Quality.Warnings, snap.Quality.Notices)
	if err != nil {
		diffs = append(diffs, fmt.Sprintf("%s: rendering warnings and notices: %v", scenario, err))
		return diffs
	}
	diffs = append(diffs, compareStrings(scenario, "warnings", gotWarnings, want.Warnings)...)
	diffs = append(diffs, compareStrings(scenario, "notices", gotNotices, want.Notices)...)
	return diffs
}

// TestNumbersEqualTheOriginal runs each of the seven scenarios through
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

			encoded, err := Snapshot(context.Background(), scenario)
			if err != nil {
				t.Fatalf("Snapshot(%s): %v", scenario.Name, err)
			}
			snap, err := snapshot.Decode(encoded)
			if err != nil {
				t.Fatalf("Decode(%s): %v", scenario.Name, err)
			}

			if diffs := compareNumbers(scenario.Name, snap, want); len(diffs) > 0 {
				t.Fatalf("%s differs from the original:\n%s", scenario.Name, strings.Join(diffs, "\n"))
			}
		})
	}
}

// collectScenario runs scenario's fixture in this process through
// connection, scope resolution, capability detection, and collection, with
// the same fixed input Snapshot uses, and returns what the two collectors
// found.
func collectScenario(ctx context.Context, scenario promtest.Scenario) (collect.Collected, error) {
	path, err := promtest.Path(scenario.Name)
	if err != nil {
		return collect.Collected{}, fmt.Errorf("%s: locating fixture: %w", scenario.Name, err)
	}
	fixture, err := promtest.Load(path)
	if err != nil {
		return collect.Collected{}, fmt.Errorf("%s: loading fixture: %w", scenario.Name, err)
	}

	fake := promtest.NewServer(fixture)
	server := httptest.NewServer(fake)
	defer server.Close()

	emittedAt, err := fixedEmittedAt()
	if err != nil {
		return collect.Collected{}, fmt.Errorf("%s: %w", scenario.Name, err)
	}
	cfg := fixedConfig(scenario, server.URL)

	window, err := collect.NewWindow(cfg.Analysis, emittedAt)
	if err != nil {
		return collect.Collected{}, fmt.Errorf("%s: building the window: %w", scenario.Name, err)
	}

	client, err := prom.Connect(ctx, prom.Options{Metrics: cfg.Metrics, Token: scenario.Token})
	if err != nil {
		return collect.Collected{}, fmt.Errorf("%s: connecting: %w", scenario.Name, err)
	}

	scope, err := discovery.ResolveScope(ctx, client, cfg.Cluster, window.End)
	if err != nil {
		return collect.Collected{}, fmt.Errorf("%s: resolving the scope: %w", scenario.Name, err)
	}

	capabilities, err := discovery.Detect(ctx, client, scope, cfg.Analysis, window.End)
	if err != nil {
		return collect.Collected{}, fmt.Errorf("%s: detecting the capabilities: %w", scenario.Name, err)
	}

	if err := discovery.CheckSufficient(capabilities); err != nil {
		return collect.Collected{}, fmt.Errorf("%s: checking the capabilities: %w", scenario.Name, err)
	}

	return collect.All(ctx, client, collect.Input{
		Scope:        scope,
		Cluster:      cfg.Cluster,
		Analysis:     cfg.Analysis,
		Capabilities: capabilities,
		Window:       window,
	})
}

// TestCollectionStatesEqualTheOriginal checks, for each of the seven
// scenarios, that the state of the namespace collector and of the workload
// collector equal namespace_collection and workload_collection of the
// recorded file.
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

			collected, err := collectScenario(context.Background(), scenario)
			if err != nil {
				t.Fatalf("collectScenario(%s): %v", scenario.Name, err)
			}

			if collected.Namespaces.State != want.NamespaceCollection {
				t.Errorf("%s: namespace collector state: got %q, want %q", scenario.Name, collected.Namespaces.State, want.NamespaceCollection)
			}
			if collected.Workloads.State != want.WorkloadCollection {
				t.Errorf("%s: workload collector state: got %q, want %q", scenario.Name, collected.Workloads.State, want.WorkloadCollection)
			}
		})
	}
}

// TestWindowEqualsTheOriginal checks, for each of the seven scenarios,
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

// TestQueriesBeginTheListOfTheOriginal checks, for each of the seven
// scenarios, that every line of the project's golden queries file equals
// the line of the same number of the recorded queries file of the
// original, and that the recorded list is at least as long.
func TestQueriesBeginTheListOfTheOriginal(t *testing.T) {
	for _, scenario := range promtest.Scenarios {
		t.Run(scenario.Name, func(t *testing.T) {
			projectLines := queriesLines(t, scenario)

			originalPath, err := pythonQueriesPath(scenario.Name)
			if err != nil {
				t.Fatalf("pythonQueriesPath(%s): %v", scenario.Name, err)
			}
			originalLines := readQueriesLines(t, originalPath)

			if len(originalLines) < len(projectLines) {
				t.Fatalf("%s: the original holds %d lines, shorter than the project's %d", scenario.Name, len(originalLines), len(projectLines))
			}
			for i, line := range projectLines {
				if originalLines[i] != line {
					t.Errorf("%s: query line %d: got %q, want %q", scenario.Name, i+1, line, originalLines[i])
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
// of the recorded file held in memory, and that a change touching one
// field of a row or a finding is reported as exactly that field, nothing
// beside it.
func TestComparisonCanFail(t *testing.T) {
	scenario := scenarioByName(t, "edge-values")

	encoded, err := Snapshot(context.Background(), scenario)
	if err != nil {
		t.Fatalf("Snapshot(%s): %v", scenario.Name, err)
	}
	snap, err := snapshot.Decode(encoded)
	if err != nil {
		t.Fatalf("Decode(%s): %v", scenario.Name, err)
	}

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

	// check requires compareNumbers to report a difference naming
	// contains, and fails when it reports none.
	check := func(t *testing.T, want pythonRecording, contains string) []string {
		t.Helper()
		diffs := compareNumbers(scenario.Name, snap, want)
		if len(diffs) == 0 {
			t.Fatalf("compareNumbers reported no difference, want one naming %q", contains)
		}
		joined := strings.Join(diffs, "\n")
		if !strings.Contains(joined, contains) {
			t.Errorf("differences do not name %q:\n%s", contains, joined)
		}
		return diffs
	}

	t.Run("a number of a statistic changed in its last digit", func(t *testing.T) {
		want := fresh(t)
		want.Namespaces[0].MemoryUsage.Average = floatPtr(math.Nextafter(*want.Namespaces[0].MemoryUsage.Average, math.Inf(1)))
		diffs := check(t, want, "namespace payments: memory_usage.average")
		if len(diffs) != 1 {
			t.Errorf("got %d differences, want 1:\n%s", len(diffs), strings.Join(diffs, "\n"))
		}
	})

	t.Run("a statistic changed from a number to null", func(t *testing.T) {
		want := fresh(t)
		want.Namespaces[0].MemoryUsage.Average = nil
		diffs := check(t, want, "namespace payments: memory_usage.average")
		if len(diffs) != 1 {
			t.Errorf("got %d differences, want 1:\n%s", len(diffs), strings.Join(diffs, "\n"))
		}
	})

	t.Run("no_base changed from true to false", func(t *testing.T) {
		want := fresh(t)
		want.Namespaces[0].CPURequestPercent.NoBase = false
		diffs := check(t, want, "namespace payments: cpu_request_percent.state")
		if len(diffs) != 1 {
			t.Errorf("got %d differences, want 1:\n%s", len(diffs), strings.Join(diffs, "\n"))
		}
	})

	t.Run("a pod count changed", func(t *testing.T) {
		want := fresh(t)
		*want.Namespaces[0].ActivePods = 99
		diffs := check(t, want, "namespace payments: pods")
		if len(diffs) != 1 {
			t.Errorf("got %d differences, want 1:\n%s", len(diffs), strings.Join(diffs, "\n"))
		}
	})

	t.Run("approximate_ratios changed", func(t *testing.T) {
		want := fresh(t)
		want.Namespaces[0].ApproximateRatios = !want.Namespaces[0].ApproximateRatios
		diffs := check(t, want, "namespace payments: approximate")
		if len(diffs) != 1 {
			t.Errorf("got %d differences, want 1:\n%s", len(diffs), strings.Join(diffs, "\n"))
		}
	})

	t.Run("a workload removed", func(t *testing.T) {
		want := fresh(t)
		want.Workloads = want.Workloads[:1]
		check(t, want, "workload payments/queue")
	})

	t.Run("a finding removed", func(t *testing.T) {
		want := fresh(t)
		removed := want.Findings[0].Code
		want.Findings = append([]pythonFinding{}, want.Findings[1:]...)
		check(t, want, removed)
	})

	t.Run("the code of a finding changed", func(t *testing.T) {
		want := fresh(t)
		original := want.Findings[0].Code
		want.Findings[0].Code = original + "_X"
		diffs := compareNumbers(scenario.Name, snap, want)
		joined := strings.Join(diffs, "\n")
		if !strings.Contains(joined, original+" ") || !strings.Contains(joined, "missing from the original") {
			t.Errorf("differences do not name %s missing from the original:\n%s", original, joined)
		}
		if !strings.Contains(joined, original+"_X") || !strings.Contains(joined, "missing from the project") {
			t.Errorf("differences do not name %s_X missing from the project:\n%s", original, joined)
		}
	})

	t.Run("a number of the evidence of a finding changed", func(t *testing.T) {
		want := fresh(t)
		want.Findings[0].Evidence[0].Number = math.Nextafter(want.Findings[0].Evidence[0].Number, math.Inf(1))
		diffs := check(t, want, "evidence[0]")
		if len(diffs) != 1 {
			t.Errorf("got %d differences, want 1:\n%s", len(diffs), strings.Join(diffs, "\n"))
		}
	})

	t.Run("the confidence of a finding changed", func(t *testing.T) {
		want := fresh(t)
		changed := false
		for i := range want.Findings {
			if want.Findings[i].Confidence == "inferred" {
				want.Findings[i].Confidence = "exact"
				changed = true
				break
			}
		}
		if !changed {
			t.Fatal("edge-values holds no finding of confidence inferred to change")
		}
		diffs := check(t, want, "confidence: got")
		if len(diffs) != 1 {
			t.Errorf("got %d differences, want 1:\n%s", len(diffs), strings.Join(diffs, "\n"))
		}
	})

	t.Run("two findings exchanged", func(t *testing.T) {
		want := fresh(t)
		want.Findings[0], want.Findings[1] = want.Findings[1], want.Findings[0]
		diffs := check(t, want, "the order differs from the original")
		if len(diffs) != 1 {
			t.Errorf("got %d differences, want 1:\n%s", len(diffs), strings.Join(diffs, "\n"))
		}
	})

	t.Run("a notice removed", func(t *testing.T) {
		want := fresh(t)
		want.Notices = want.Notices[:1]
		check(t, want, "notices[1]")
	})

	t.Run("one letter of a notice changed", func(t *testing.T) {
		want := fresh(t)
		want.Notices[0] = strings.Replace(want.Notices[0], "carga", "xarga", 1)
		diffs := check(t, want, "notices[0]")
		if len(diffs) != 1 {
			t.Errorf("got %d differences, want 1:\n%s", len(diffs), strings.Join(diffs, "\n"))
		}
	})
}
