package document

import (
	"slices"
	"strings"

	"github.com/marceloalvescf/k8s-capacity-report/internal/catalog"
	"github.com/marceloalvescf/k8s-capacity-report/internal/discovery"
	"github.com/marceloalvescf/k8s-capacity-report/internal/snapshot"
)

// deploymentKind is the one workload kind a missing HPA is a finding for;
// any other kind that lacks an HPA scales by a strategy of its own.
const deploymentKind = "Deployment" // wording: not reader text, a workload kind name

// buildHPAsSection assembles section 3: a numbered heading, its
// introduction, a note when the HPA data is incomplete, and then, for every
// namespace of the scope, the HPAs observed there and the workloads no HPA
// targets. A namespace with neither gets one paragraph. This section
// applies no minimum severity.
func buildHPAsSection(b *builder, s snapshot.Snapshot) []Block {
	period := b.period(b.time(s.Report.PeriodStart), b.time(s.Report.PeriodEnd))
	blocks := []Block{
		Heading{Level: 1, Numbered: true, Runs: []Run{Text(b.text(catalog.SectionsHPAsHeading, nil))}},
		Paragraph{Runs: []Run{Text(b.text(catalog.SectionsHPAsIntro, catalog.Params{"Period": period}))}},
	}

	note := hpaNote(b, s)
	if note != nil {
		blocks = append(blocks, *note)
	}

	for _, namespace := range s.Scope.Namespaces {
		hpas := observedHPAs(s.HPAs, namespace)
		uncovered := uncoveredWorkloads(s.Workloads, namespace)

		blocks = append(blocks, namespaceHeading(b, namespace))
		if len(hpas) == 0 && len(uncovered) == 0 {
			key := catalog.SectionsHPAsEmptyNamespace
			if note != nil {
				key = catalog.SectionsHPAsEmptyNamespacePartial
			}
			blocks = append(blocks, Paragraph{Runs: []Run{Text(b.text(key, nil))}})
			continue
		}
		if len(hpas) > 0 {
			blocks = append(blocks, tableHeading(b, catalog.SectionsHPAsObservedHeading), hpaTable(b, hpas))
		}
		if len(uncovered) > 0 {
			blocks = append(blocks, tableHeading(b, catalog.SectionsHPAsUncoveredHeading), uncoveredTable(b, uncovered))
		}
	}
	return blocks
}

// tableHeading returns the unnumbered level-3 heading above a table.
func tableHeading(b *builder, key catalog.Key) Heading {
	return Heading{Level: 3, Numbered: false, Runs: []Run{Text(b.text(key, nil))}}
}

// hpaNote returns the callout that warns the HPA data is not whole, or nil
// when it is: incomplete when either capability is unavailable, partial
// when either is partial.
func hpaNote(b *builder, s snapshot.Snapshot) *Callout {
	incomplete, partial := false, false
	for _, c := range effectiveCapabilities(s) {
		if c.Name != "hpa_history" && c.Name != "workloads_without_hpa" {
			continue
		}
		switch c.State {
		case discovery.StateUnavailable:
			incomplete = true
		case discovery.StatePartial:
			partial = true
		}
	}

	switch {
	case incomplete:
		return &Callout{
			Kind:  "warning",
			Title: []Run{Text(b.text(catalog.SectionsHPAsNoteIncompleteTitle, nil))},
			Runs:  []Run{Text(b.text(catalog.SectionsHPAsNoteIncomplete, nil))},
		}
	case partial:
		return &Callout{
			Kind:  "warning",
			Title: []Run{Text(b.text(catalog.SectionsHPAsNotePartialTitle, nil))},
			Runs:  []Run{Text(b.text(catalog.SectionsHPAsNotePartial, nil))},
		}
	}
	return nil
}

// observedHPAs returns the HPAs of namespace by name, leaving out those
// whose peak is a known zero: an HPA scaled to zero. An HPA whose peak is
// unknown stays.
func observedHPAs(hpas []snapshot.HPA, namespace string) []snapshot.HPA {
	var out []snapshot.HPA
	for _, h := range hpas {
		if h.Namespace != namespace {
			continue
		}
		if h.Current.State == stateValue && h.Current.Peak != nil && *h.Current.Peak == 0 {
			continue
		}
		out = append(out, h)
	}
	slices.SortStableFunc(out, func(a, c snapshot.HPA) int { return strings.Compare(a.Name, c.Name) })
	return out
}

// uncoveredWorkloads returns the rows of namespace that no HPA targets, by
// kind and then name.
func uncoveredWorkloads(workloads []snapshot.Workload, namespace string) []snapshot.Workload {
	var out []snapshot.Workload
	for _, w := range workloads {
		if w.Namespace == namespace && w.HPACoverage == snapshot.CoverageUncovered {
			out = append(out, w)
		}
	}
	slices.SortStableFunc(out, func(a, c snapshot.Workload) int {
		if n := strings.Compare(a.Kind, c.Kind); n != 0 {
			return n
		}
		return strings.Compare(a.Name, c.Name)
	})
	return out
}

// hpaTable builds one "hpa-observed" table, one row for every HPA.
func hpaTable(b *builder, hpas []snapshot.HPA) Table {
	rows := make([][][]Run, len(hpas))
	for i, h := range hpas {
		rows[i] = [][]Run{
			{Ident(h.Name)},
			{specRun(b, h.SpecMin)},
			{specRun(b, h.SpecMax)},
			{quantityRun(b, h.Current, h.Current.Average, func(v float64) string { return b.cat.Decimal(v, 1) })},
			{quantityRun(b, h.Current, h.Current.Minimum, b.cat.Count)},
			{quantityRun(b, h.Current, h.Current.Peak, b.cat.Count)},
			{hpaSituation(b, h)},
		}
	}
	return Table{
		ID: "hpa-observed",
		Headers: []string{
			b.text(catalog.SectionsHPAsHeaderHPA, nil),
			b.text(catalog.SectionsHPAsHeaderSpecMin, nil),
			b.text(catalog.SectionsHPAsHeaderSpecMax, nil),
			b.text(catalog.SectionsHPAsHeaderCurrentAverage, nil),
			b.text(catalog.SectionsHPAsHeaderMinReached, nil),
			b.text(catalog.SectionsHPAsHeaderMaxReached, nil),
			b.text(catalog.SectionsHPAsHeaderSituation, nil),
		},
		Rows: rows,
	}
}

// specRun writes a configured replica bound by Count, or the unavailable
// placeholder when it is unknown.
func specRun(b *builder, spec *float64) Run {
	return Text(podsText(b, spec))
}

// hpaAtMax reports whether an HPA reached its maximum: the maximum and the
// peak are known, the HPA is not pinned, and the peak is at or above the
// maximum.
func hpaAtMax(h snapshot.HPA) bool {
	if h.SpecMax == nil || h.Current.State != stateValue || h.Current.Peak == nil {
		return false
	}
	pinned := h.SpecMin != nil && *h.SpecMin == *h.SpecMax
	return !pinned && *h.Current.Peak >= *h.SpecMax
}

// hpaSituation judges an HPA, the first case that holds: the maximum was
// reached when the maximum and the peak are known and the HPA is not
// pinned; pinned when both bounds are known and equal; below the maximum
// when the maximum and the peak are known; unavailable otherwise.
func hpaSituation(b *builder, h snapshot.HPA) Run {
	status, text := hpaSituationText(b, h)
	return StatusText(status, text)
}

// hpaSituationText is the status and the text of hpaSituation.
func hpaSituationText(b *builder, h snapshot.HPA) (status, text string) {
	maxKnown := h.SpecMax != nil
	peakKnown := h.Current.State == stateValue && h.Current.Peak != nil
	pinned := h.SpecMin != nil && maxKnown && *h.SpecMin == *h.SpecMax

	switch {
	case hpaAtMax(h):
		return "critical", b.text(catalog.SectionsHPAsSituationMaxReached, nil)
	case pinned:
		return "healthy", b.text(catalog.SectionsHPAsSituationPinned, nil)
	case maxKnown && peakKnown:
		return "healthy", b.text(catalog.SectionsHPAsSituationBelowMax, nil)
	default:
		return "unavailable", b.text(catalog.PlaceholderUnavailable, nil)
	}
}

// uncoveredTable builds one "workloads-without-hpa" table, one row for every
// workload.
func uncoveredTable(b *builder, workloads []snapshot.Workload) Table {
	rows := make([][][]Run, len(workloads))
	for i, w := range workloads {
		situation := StatusText("unavailable", b.text(catalog.SectionsHPAsSituationSpecificStrategy, nil))
		if w.Kind == deploymentKind {
			situation = StatusText("attention", b.text(catalog.SectionsHPAsSituationNoHPA, nil))
		}
		rows[i] = [][]Run{
			{Text(w.Kind)},
			{Ident(w.Name)},
			{podsRun(b, w.Pods)},
			{situation},
		}
	}
	return Table{
		ID: "workloads-without-hpa",
		Headers: []string{
			b.text(catalog.SectionsHPAsHeaderKind, nil),
			b.text(catalog.SectionsHPAsHeaderWorkload, nil),
			b.text(catalog.SectionsHPAsHeaderPods, nil),
			b.text(catalog.SectionsHPAsHeaderSituation, nil),
		},
		Rows: rows,
	}
}
