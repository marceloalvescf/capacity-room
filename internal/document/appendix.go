package document

import (
	"slices"
	"strings"

	"github.com/marceloalvescf/capacity-room/internal/catalog"
	"github.com/marceloalvescf/capacity-room/internal/collect"
	"github.com/marceloalvescf/capacity-room/internal/discovery"
	"github.com/marceloalvescf/capacity-room/internal/snapshot"
)

// buildAppendix assembles the numbered appendix heading and either the
// "quality-appendix" table or its fallback paragraph, when no row of it
// passes the minimum severity.
func buildAppendix(b *builder, s snapshot.Snapshot, severity string) []Block {
	heading := Heading{Level: 1, Numbered: true, Runs: []Run{Text(b.text(catalog.AppendixHeading, nil))}}

	rows := appendixRows(b, effectiveCapabilities(s), severity)
	if len(rows) == 0 {
		return []Block{heading, Paragraph{Runs: []Run{Text(b.text(catalog.AppendixEmpty, nil))}}}
	}

	table := Table{
		ID: "quality-appendix",
		Headers: []string{
			b.text(catalog.AppendixHeaderCapability, nil),
			b.text(catalog.AppendixHeaderCoverage, nil),
			b.text(catalog.AppendixHeaderSource, nil),
			b.text(catalog.AppendixHeaderRemark, nil),
			b.text(catalog.AppendixHeaderEvidence, nil),
		},
		Rows: rows,
	}
	return []Block{heading, table}
}

// appendixStatus is the status mark of a capability's state: available is
// healthy, partial is attention, and unavailable is drawn as unavailable: a
// capability that could not be read is missing data, not a finding.
func appendixStatus(state string) string {
	switch state {
	case discovery.StatePartial:
		return "attention"
	case discovery.StateUnavailable:
		return "unavailable"
	default:
		return "healthy"
	}
}

// capabilityState is the mark of a capability's state: the state's word as
// its text and the legend's word for its status as its alternate text.
func capabilityState(b *builder, state string) Run {
	status := appendixStatus(state)
	return StatusValue(status, b.text(catalog.State(state), nil), statusWord(b, status))
}

// passesSeverity reports whether a capability's state clears the minimum
// severity: "all" keeps every row, "attention" keeps partial and
// unavailable, "critical" keeps unavailable. It does not touch the three
// counts of the opening page, which count every capability regardless.
func passesSeverity(state, severity string) bool {
	switch severity {
	case "attention":
		return state == discovery.StatePartial || state == discovery.StateUnavailable
	case "critical":
		return state == discovery.StateUnavailable
	default: // "all"
		return true
	}
}

// appendixRows builds one row per capability that passes severity, in the
// order the snapshot stores them.
func appendixRows(b *builder, capabilities []snapshot.Capability, severity string) [][][]Run {
	var rows [][][]Run
	for _, c := range capabilities {
		if !passesSeverity(c.State, severity) {
			continue
		}
		rows = append(rows, [][]Run{
			{Text(b.text(catalog.CapabilityName(c.Name), nil))},
			{capabilityState(b, c.State)},
			{Text(b.text(catalog.Strategy(c.Strategy), nil))},
			remarkCell(b, c.Reasons),
			{Text(b.text(catalog.CapabilityEvidence(c.Name, c.State), nil))},
		})
	}
	return rows
}

// weakeningWarnings names, for each capability that a warning weakens, the
// warning that turns it partial: an ambiguous HPA weakens the HPA history
// and the workloads without HPA, and incomplete node capacity evidence
// weakens the node utilization.
var weakeningWarnings = map[string]string{
	"hpa_history":           collect.WarningHPAAmbiguousSpec,
	"workloads_without_hpa": collect.WarningHPAAmbiguousTarget,
	"node_utilization":      collect.WarningNodeCapacityEvidenceIncomplete,
}

// effectiveCapabilities returns the capabilities of the snapshot with an
// available capability turned partial when the warning that weakens it is
// present (HPA_AMBIGUOUS_SPEC for hpa_history, HPA_AMBIGUOUS_TARGET for
// workloads_without_hpa, NODE_CAPACITY_EVIDENCE_INCOMPLETE for
// node_utilization), the warning's code standing as the one reason.
// The snapshot is left as it is.
func effectiveCapabilities(s snapshot.Snapshot) []snapshot.Capability {
	out := slices.Clone(s.Quality.Capabilities)
	for i, c := range out {
		code, weakened := weakeningWarnings[c.Name]
		if !weakened || c.State != discovery.StateAvailable {
			continue
		}
		if slices.ContainsFunc(s.Quality.Warnings, func(w snapshot.Message) bool { return w.Code == code }) {
			out[i].State = discovery.StatePartial
			out[i].Reasons = []string{code}
		}
	}
	return out
}

// appendixRemark joins the sentences of every reason code with one space,
// or, when there is none, returns the empty string.
func appendixRemark(b *builder, reasons []string) string {
	if len(reasons) == 0 {
		return ""
	}
	sentences := make([]string, len(reasons))
	for i, code := range reasons {
		if slices.Contains(collect.WarningCodes, code) {
			sentences[i] = b.text(catalog.Warning(code), nil)
			continue
		}
		sentences[i] = b.text(catalog.CapabilityReason(code), nil)
	}
	return strings.Join(sentences, " ")
}

// warningText writes a warning: the sentence of its reason code when the
// warning is one, else the sentence of the warning.
func warningText(b *builder, code string) string {
	if slices.Contains(discovery.ReasonCodes, code) {
		return b.text(catalog.CapabilityReason(code), nil)
	}
	return b.text(catalog.Warning(code), nil)
}

// noticeText writes a notice: the entry of its code for its count, the
// count filled in when it is not one.
func noticeText(b *builder, n snapshot.Message) string {
	count := n.Params["count"]
	var params catalog.Params
	if count != 1 {
		params = catalog.Params{"Count": b.cat.Count(count)}
	}
	return b.text(catalog.Notice(n.Code, count), params)
}

// buildNotes assembles the interpretation notes that follow the appendix:
// one row for every warning, then every notice, of the snapshot, each text
// written once; or no block at all when there is neither. The minimum
// severity does not filter these rows.
func buildNotes(b *builder, s snapshot.Snapshot) []Block {
	var rows [][][]Run
	seen := make(map[string]bool)
	add := func(text string) {
		if seen[text] {
			return
		}
		seen[text] = true
		rows = append(rows, [][]Run{{Text(text)}})
	}

	for _, w := range s.Quality.Warnings {
		add(warningText(b, w.Code))
	}
	for _, n := range s.Quality.Notices {
		add(noticeText(b, n))
	}

	if len(rows) == 0 {
		return nil
	}
	return []Block{
		Heading{Level: 2, Numbered: false, Runs: []Run{Text(b.text(catalog.NotesHeading, nil))}},
		Table{ID: "interpretation-notes", Headers: []string{b.text(catalog.NotesHeaderRemark, nil)}, Rows: rows},
	}
}

// remarkCell is the remark of a capability as a table cell: empty when the
// capability has no reason.
func remarkCell(b *builder, reasons []string) []Run {
	if len(reasons) == 0 {
		return []Run{}
	}
	return []Run{Text(appendixRemark(b, reasons))}
}
