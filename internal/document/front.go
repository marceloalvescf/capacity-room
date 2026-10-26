package document

import (
	"fmt"

	"github.com/marceloalvescf/capacity-room/internal/catalog"
	"github.com/marceloalvescf/capacity-room/internal/discovery"
	"github.com/marceloalvescf/capacity-room/internal/snapshot"
)

// legendRow is one row of the status-legend table: the status mark and the
// catalog key of its meaning.
type legendRow struct {
	status string
	key    catalog.Key
}

// legendOrder is the order the original prints the status legend in:
// healthy, attention, critical, unavailable, undefined. This is not the
// order of template/capacity/validate.typ's own allowed-statuses list.
var legendOrder = []legendRow{
	{"healthy", catalog.FrontLegendHealthy},
	{"attention", catalog.FrontLegendAttention},
	{"critical", catalog.FrontLegendCritical},
	{"unavailable", catalog.FrontLegendUnavailable},
	{"undefined", catalog.FrontLegendUndefined},
}

// statusWord is the legend's word for status: the word a status mark stands
// for when its run's text is a value rather than that word.
func statusWord(b *builder, status string) string {
	for _, row := range legendOrder {
		if row.status == status {
			return b.text(row.key, nil)
		}
	}
	if b.err == nil {
		b.err = fmt.Errorf("document: no legend word for status %q", status)
	}
	return ""
}

// profileKeys word the profile the document was built with.
var profileKeys = map[string]catalog.Key{
	"detailed":  catalog.FrontMetadataProfileDetailed,
	"executive": catalog.FrontMetadataProfileExecutive,
}

// buildFront assembles the opening page: an outline, the report metadata,
// the status legend, and the data-quality counts.
func buildFront(b *builder, s snapshot.Snapshot, profile string) Front {
	return Front{Blocks: []Block{
		Outline{Title: b.text(catalog.FrontOutlineTitle, nil), Depth: 2},
		frontHeading(b, catalog.FrontMetadataHeading),
		metadataTable(b, s, profile),
		frontHeading(b, catalog.FrontLegendHeading),
		legendTable(b),
		frontHeading(b, catalog.FrontQualityHeading),
		qualityList(b, s),
	}}
}

// frontHeading returns an unnumbered, level-2 heading holding one plain
// run: every heading of the opening page is one of these.
func frontHeading(b *builder, key catalog.Key) Heading {
	return Heading{Level: 2, Numbered: false, Runs: []Run{Text(b.text(key, nil))}}
}

// metadataTable builds the "report-metadata" table: six rows always
// present, and the organization and classification rows only when the
// snapshot carries them. A name is an ident run, so that it keeps its
// letters and breaks only at its own hyphens.
func metadataTable(b *builder, s snapshot.Snapshot, profile string) Table {
	rows := [][][]Run{
		metadataRow(b, catalog.FrontMetadataEnvironment, []Run{Ident(s.Scope.ClusterName)}),
		metadataRow(b, catalog.FrontMetadataPeriod, []Run{Text(periodRange(b, s))}),
		metadataRow(b, catalog.FrontMetadataProfile, []Run{Text(b.text(profileKeys[profile], nil))}),
		metadataRow(b, catalog.FrontMetadataNamespaces, identList(s.Scope.Namespaces)),
		metadataRow(b, catalog.FrontMetadataExcludedContainers, excludedContainers(b, s)),
	}
	if s.Report.Organization != "" {
		rows = append(rows, metadataRow(b, catalog.FrontMetadataOrganization, []Run{Text(s.Report.Organization)}))
	}
	if s.Report.Classification != "" {
		rows = append(rows, metadataRow(b, catalog.FrontMetadataClassification, []Run{Text(s.Report.Classification)}))
	}

	return Table{
		ID:      "report-metadata",
		Headers: []string{b.text(catalog.FrontMetadataHeaderField, nil), b.text(catalog.FrontMetadataHeaderValue, nil)},
		Rows:    rows,
	}
}

// metadataRow builds one label/value row: the label a single text run from
// the catalog, the value the runs given.
func metadataRow(b *builder, key catalog.Key, value []Run) [][]Run {
	return [][]Run{{Text(b.text(key, nil))}, value}
}

// identList sets each name as an ident run, the names parted by a comma
// and a space in a text run.
func identList(names []string) []Run {
	runs := make([]Run, 0, 2*len(names))
	for i, name := range names {
		if i > 0 {
			runs = append(runs, Text(", "))
		}
		runs = append(runs, Ident(name))
	}
	return runs
}

// periodRange formats the analysis period's start and end by their own
// wall clock and offset, then fills them into front.metadata.period_range.
func periodRange(b *builder, s snapshot.Snapshot) string {
	start := b.cat.DateTime(b.time(s.Report.PeriodStart))
	end := b.cat.DateTime(b.time(s.Report.PeriodEnd))
	return b.text(catalog.FrontMetadataPeriodRange, catalog.Params{"Start": start, "End": end})
}

// excludedContainers lists the excluded container names as ident runs,
// or, when there is none, returns the catalog's fallback sentence.
func excludedContainers(b *builder, s snapshot.Snapshot) []Run {
	if len(s.Scope.ExcludedContainers) == 0 {
		return []Run{Text(b.text(catalog.FrontMetadataNoExcludedContainers, nil))}
	}
	return identList(s.Scope.ExcludedContainers)
}

// legendTable builds the five-row "status-legend" table, in legendOrder:
// the first cell of each row holds the status mark alone, with no label of
// its own.
func legendTable(b *builder) Table {
	rows := make([][][]Run, len(legendOrder))
	for i, row := range legendOrder {
		rows[i] = [][]Run{
			{Status(row.status)},
			{Text(b.text(row.key, nil))},
		}
	}
	return Table{
		ID:      "status-legend",
		Headers: []string{b.text(catalog.FrontLegendHeaderStatus, nil), b.text(catalog.FrontLegendHeaderMeaning, nil)},
		Rows:    rows,
	}
}

// qualityList counts every capability of the snapshot by effective state, always
// over all nine, at either floor, and builds the three
// bullet items of the opening page.
func qualityList(b *builder, s snapshot.Snapshot) List {
	var available, partial, unavailable int
	for _, capability := range effectiveCapabilities(s) {
		switch capability.State {
		case discovery.StateAvailable:
			available++
		case discovery.StatePartial:
			partial++
		case discovery.StateUnavailable:
			unavailable++
		}
	}

	return List{Items: [][]Run{
		qualityItem(b, catalog.FrontQualityAvailable, available),
		qualityItem(b, catalog.FrontQualityPartial, partial),
		qualityItem(b, catalog.FrontQualityUnavailable, unavailable),
	}}
}

// qualityItem builds one bullet: a strong label, then the count with one
// leading space.
func qualityItem(b *builder, key catalog.Key, count int) []Run {
	return []Run{Strong(b.text(key, nil)), Text(" " + b.cat.Integer(count))}
}
