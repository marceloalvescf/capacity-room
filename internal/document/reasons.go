package document

import (
	"fmt"
	"slices"
	"strings"

	"github.com/marceloalvescf/capacity-room/internal/catalog"
	"github.com/marceloalvescf/capacity-room/internal/snapshot"
)

// evidenceNumber returns the number a finding carries under name.
func evidenceNumber(f snapshot.Finding, name string) (float64, bool) {
	for _, e := range f.Evidence {
		if e.Name == name && e.Value.Kind == "number" {
			return e.Value.Number, true
		}
	}
	return 0, false
}

// reasonFragment writes the short reason of a finding, as the original's
// _finding_reason_fragment: the branches below are tried in order and the
// first that holds with its number present wins; a finding that reaches
// none keeps the first error of the builder.
func reasonFragment(b *builder, f snapshot.Finding) string {
	resource := "cpu"
	if strings.Contains(f.Code, "_MEMORY_") {
		resource = "memory"
	}
	percentText := func(family, name string) (string, bool) {
		value, ok := evidenceNumber(f, name)
		if !ok {
			return "", false
		}
		return b.text(catalog.Reason(family, resource), catalog.Params{"Percent": b.cat.Percent(value)}), true
	}
	// first returns the text of the first (family, evidence name) pair that
	// holds a number.
	first := func(pairs ...[2]string) (string, bool) {
		for _, pair := range pairs {
			if text, ok := percentText(pair[0], pair[1]); ok {
				return text, true
			}
		}
		return "", false
	}

	switch {
	case strings.HasSuffix(f.Code, "_REQUEST_MISSING"):
		return b.text(catalog.Reason("request_missing", resource), nil)
	case strings.HasSuffix(f.Code, "_LIMIT_MISSING"):
		return b.text(catalog.Reason("limit_missing", resource), nil)
	}

	var text string
	var ok bool
	if strings.HasSuffix(f.Code, "_REQUEST_HIGH") {
		text, ok = first([2]string{"request_high", "percent"})
	}
	if !ok && strings.Contains(f.Code, "_REQUEST_") {
		text, ok = first([2]string{"request_peak", "peak_percent"}, [2]string{"request_average", "average_percent"})
	}
	if !ok && strings.Contains(f.Code, "_LIMIT_") {
		text, ok = first([2]string{"limit_peak", "peak_percent"}, [2]string{"limit_average", "average_percent"})
	}
	if !ok && strings.Contains(f.Code, "_USAGE_") {
		text, ok = first([2]string{"usage_peak", "peak_percent"}, [2]string{"usage_average", "average_percent"})
	}
	if !ok && strings.HasSuffix(f.Code, "_OVERREQUESTED") {
		text, ok = first([2]string{"request_average", "average_percent"})
	}
	if !ok && strings.HasSuffix(f.Code, "_BURSTY") {
		text, ok = first([2]string{"bursty", "gap_percent"})
	}
	if ok {
		return text
	}

	switch f.Code {
	case "NODE_OVERREQUESTED":
		return strings.TrimRight(b.text(catalog.FindingSummary("node_overrequested", ""), nil), ".")
	case "NODE_LOW_UTILIZATION":
		return strings.TrimRight(b.text(catalog.FindingSummary("node_low_utilization", ""), nil), ".")
	case "NODE_NOT_READY":
		return strings.TrimRight(b.text(catalog.FindingSummary("node_not_ready", ""), nil), ".")
	case "NODE_UNSCHEDULABLE":
		return strings.TrimRight(b.text(catalog.FindingSummary("node_unschedulable", ""), nil), ".")
	}
	if b.err == nil {
		b.err = fmt.Errorf("finding %s has no reason text", f.Code)
	}
	return ""
}

// allMissing lists the four codes that, present together on one workload,
// collapse into one sentence.
var allMissing = []string{
	"WORKLOAD_CPU_REQUEST_MISSING",
	"WORKLOAD_CPU_LIMIT_MISSING",
	"WORKLOAD_MEMORY_REQUEST_MISSING",
	"WORKLOAD_MEMORY_LIMIT_MISSING",
}

// workloadReason joins the fragments of the findings of one workload, in
// the order given, each once, with a semicolon. When the four missing codes
// of a workload are all present, one sentence for them comes first and
// those four give no fragment of their own.
func workloadReason(b *builder, findings []snapshot.Finding) string {
	collapsed := true
	for _, code := range allMissing {
		if !slices.ContainsFunc(findings, func(f snapshot.Finding) bool { return f.Code == code }) {
			collapsed = false
			break
		}
	}

	var fragments []string
	add := func(fragment string) {
		if !slices.Contains(fragments, fragment) {
			fragments = append(fragments, fragment)
		}
	}
	if collapsed {
		add(b.text(catalog.SectionsExecutiveReasonAllMissing, nil))
	}
	for _, f := range findings {
		if collapsed && slices.Contains(allMissing, f.Code) {
			continue
		}
		add(reasonFragment(b, f))
	}
	return strings.Join(fragments, "; ")
}
