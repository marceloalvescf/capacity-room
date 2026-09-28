package document

import (
	"strings"

	"github.com/marceloalvescf/k8s-capacity-report/internal/catalog"
	"github.com/marceloalvescf/k8s-capacity-report/internal/discovery"
	"github.com/marceloalvescf/k8s-capacity-report/internal/snapshot"
)

// buildAppendix assembles the numbered appendix heading and either the
// "quality-appendix" table or its fallback paragraph, when no row of it
// passes the minimum severity.
func buildAppendix(b *builder, s snapshot.Snapshot, severity string) []Block {
	heading := Heading{Level: 1, Numbered: true, Runs: []Run{Text(b.text(catalog.AppendixHeading, nil))}}

	rows := appendixRows(b, s, severity)
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
// healthy, partial is attention, unavailable is critical.
func appendixStatus(state string) string {
	switch state {
	case discovery.StatePartial:
		return "attention"
	case discovery.StateUnavailable:
		return "critical"
	default:
		return "healthy"
	}
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
func appendixRows(b *builder, s snapshot.Snapshot, severity string) [][][]Run {
	var rows [][][]Run
	for _, c := range s.Quality.Capabilities {
		if !passesSeverity(c.State, severity) {
			continue
		}
		rows = append(rows, [][]Run{
			{Text(b.text(catalog.CapabilityName(c.Name), nil))},
			{StatusText(appendixStatus(c.State), b.text(catalog.State(c.State), nil))},
			{Text(b.text(catalog.Strategy(c.Strategy), nil))},
			{Text(appendixRemark(b, c.Reasons))},
			{Text(b.text(catalog.CapabilityEvidence(c.Name, c.State), nil))},
		})
	}
	return rows
}

// appendixRemark joins the sentences of every reason code with one space,
// or, when there is none, returns the catalog's "no remark" sentence.
func appendixRemark(b *builder, reasons []string) string {
	if len(reasons) == 0 {
		return b.text(catalog.AppendixNoRemark, nil)
	}
	sentences := make([]string, len(reasons))
	for i, code := range reasons {
		sentences[i] = b.text(catalog.CapabilityReason(code), nil)
	}
	return strings.Join(sentences, " ")
}
