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

	"github.com/marceloalvescf/k8s-capacity-report/internal/collect"
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
// The pair "approximate" carries a boolean rather than a number; its value
// is not compared (compareEvidence leaves the pair out and compares the
// finding's confidence instead), so UnmarshalJSON only checks that it
// reads as one or the other.
type pythonEvidencePair struct {
	Name   string
	Number float64
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
	var flag bool
	if err := json.Unmarshal(raw[1], &flag); err != nil {
		return fmt.Errorf("evidence pair value is neither a number nor a boolean: %w", err)
	}
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
	return diffs
}

// workloadRowKey identifies a workload row by namespace, kind, and name.
type workloadRowKey struct{ Namespace, Kind, Name string }

// compareWorkloads matches the workloads of a snapshot with those of a
// recorded file by namespace, kind, and name, and compares each pair found
// on both sides. collectedRows is collect.Collected.Workloads.Rows of the
// same run: it holds the same rows as got, in the same order, since
// buildWorkloads (internal/snapshot/build.go) copies them across one for
// one with no filtering or reordering, so collectedRows[i] is the row
// got[i] was built from.
func compareWorkloads(scenario string, got []snapshot.Workload, collectedRows []collect.Workload, want []pythonWorkloadRow) []string {
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
		if gf.ScopeType != wf.ScopeType {
			diffs = append(diffs, fmt.Sprintf("%s: scope_type: got %q, want %q", label, gf.ScopeType, wf.ScopeType))
		}
		if gf.Confidence != wf.Confidence {
			diffs = append(diffs, fmt.Sprintf("%s: confidence: got %q, want %q", label, gf.Confidence, wf.Confidence))
		}
		diffs = append(diffs, compareEvidence(label, gf.Evidence, wf.Evidence)...)
	}

	if sameKeySet(gotOrder, wantOrder) && !slices.Equal(gotOrder, wantOrder) {
		for i := range gotOrder {
			if gotOrder[i] != wantOrder[i] {
				diffs = append(diffs, fmt.Sprintf("%s: findings[%d]: the order differs from the original: got %v, want %v", scenario, i, gotOrder[i], wantOrder[i]))
				break
			}
		}
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
	diffs = append(diffs, compareFindings(scenario, snap.Findings, want.Findings)...)

	gotWarnings, gotNotices, err := originalTexts(snap.Quality.Warnings, snap.Quality.Notices)
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

// collectorQueryFloor is the fewest lines the project's own golden queries
// file can hold for any scenario today: the fourteen queries the
// namespace collector and the workload collector send between them, and
// at least one query of the preflight that precedes them. A list shorter
// than this is never a real run: it is empty, or holds only the
// preflight.
const collectorQueryFloor = 14 + 1

// TestQueriesBeginTheListOfTheOriginal checks, for each of the seven
// scenarios, that the project's own golden queries file holds at least
// collectorQueryFloor lines, that every one of them equals the line of
// the same number of the recorded queries file of the original, and that
// the recorded list is strictly longer.
func TestQueriesBeginTheListOfTheOriginal(t *testing.T) {
	for _, scenario := range promtest.Scenarios {
		t.Run(scenario.Name, func(t *testing.T) {
			projectLines := queriesLines(t, scenario)
			if len(projectLines) < collectorQueryFloor {
				t.Fatalf("%s: the project's own list holds %d lines; the floor is the fourteen queries of the collectors and at least one of the preflight, %d",
					scenario.Name, len(projectLines), collectorQueryFloor)
			}

			originalPath, err := pythonQueriesPath(scenario.Name)
			if err != nil {
				t.Fatalf("pythonQueriesPath(%s): %v", scenario.Name, err)
			}
			originalLines := readQueriesLines(t, originalPath)

			if len(originalLines) < len(projectLines) {
				t.Fatalf("%s: the original holds %d lines, shorter than the project's %d", scenario.Name, len(originalLines), len(projectLines))
			}
			// The original goes on to query the autoscalers, the nodes,
			// and the cluster autoscaling after the namespaces and the
			// workloads, which this project does not send yet, so its
			// list is always strictly longer; this assertion changes once
			// the project sends those queries too.
			if len(originalLines) <= len(projectLines) {
				t.Errorf("%s: the original holds %d lines, not more than the project's %d", scenario.Name, len(originalLines), len(projectLines))
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
		key := findingKey{removed.Code, removed.ScopeName, removed.ScopeKind}
		checkExact(t, want, []string{
			findingLabel(scenario.Name, key) + ": present in the project, missing from the original",
		})
	})

	t.Run("the code of a finding changed", func(t *testing.T) {
		want := fresh(t)
		original := want.Findings[0]
		want.Findings[0].Code = original.Code + "_X"
		originalKey := findingKey{original.Code, original.ScopeName, original.ScopeKind}
		changedKey := findingKey{original.Code + "_X", original.ScopeName, original.ScopeKind}
		checkExact(t, want, []string{
			findingLabel(scenario.Name, originalKey) + ": present in the project, missing from the original",
			findingLabel(scenario.Name, changedKey) + ": present in the original, missing from the project",
		})
	})

	t.Run("a number of the evidence of a finding changed", func(t *testing.T) {
		want := fresh(t)
		f := want.Findings[0]
		key := findingKey{f.Code, f.ScopeName, f.ScopeKind}
		want.Findings[0].Evidence[0].Number = 56
		checkExact(t, want, []string{
			findingLabel(scenario.Name, key) + ": evidence[0].average_percent: got 55, want 56",
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
		key := findingKey{f.Code, f.ScopeName, f.ScopeKind}
		want.Findings[idx].Confidence = "exact"
		checkExact(t, want, []string{
			findingLabel(scenario.Name, key) + `: confidence: got "inferred", want "exact"`,
		})
	})

	t.Run("the scope type of a finding changed", func(t *testing.T) {
		want := fresh(t)
		f := want.Findings[2]
		if f.ScopeType != "namespace" {
			t.Fatalf("edge-values findings[2] has scope_type %q, want %q", f.ScopeType, "namespace")
		}
		key := findingKey{f.Code, f.ScopeName, f.ScopeKind}
		want.Findings[2].ScopeType = "workload"
		checkExact(t, want, []string{
			findingLabel(scenario.Name, key) + `: scope_type: got "namespace", want "workload"`,
		})
	})

	t.Run("two findings exchanged", func(t *testing.T) {
		want := fresh(t)
		first, second := want.Findings[0], want.Findings[1]
		want.Findings[0], want.Findings[1] = second, first
		firstKey := findingKey{first.Code, first.ScopeName, first.ScopeKind}
		secondKey := findingKey{second.Code, second.ScopeName, second.ScopeKind}
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
}

// TestOriginalSentencesCoverEveryCode wants a sentence of the original for
// every warning code and for both forms of every notice code, so that the
// comparison with the recorded files never passes by comparing nothing.
func TestOriginalSentencesCoverEveryCode(t *testing.T) {
	for _, code := range collect.WarningCodes {
		if _, err := originalWarningText(code); err != nil {
			t.Errorf("warning %s: %v", code, err)
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
