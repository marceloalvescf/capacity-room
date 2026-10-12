package document

import (
	"fmt"
	"slices"
	"time"

	"github.com/marceloalvescf/k8s-capacity-report/internal/catalog"
	"github.com/marceloalvescf/k8s-capacity-report/internal/snapshot"
)

// severities lists the minimum severities Build accepts. internal/snapshot
// keeps the same list, unexported: a stored snapshot must be able to judge
// a file no configuration of this version produced, so this copy is
// deliberate, not a shortcut.
var severities = []string{"all", "attention", "critical"}

// builder collects the first error Build meets, the way bufio.Scanner
// keeps the first error of a read: every call after that one is a no-op.
// A hundred calls to the catalog need no error check of their own this
// way, and Build never returns a document with an empty text in it.
type builder struct {
	cat *catalog.Catalog
	err error
}

// text looks up key, filled with params, and keeps the first error.
func (b *builder) text(key catalog.Key, params catalog.Params) string {
	if b.err != nil {
		return ""
	}
	text, err := b.cat.Text(key, params)
	if err != nil {
		b.err = err
		return ""
	}
	return text
}

// time parses one of the snapshot's own time fields and keeps the first
// error. A value that failed Check would already have been refused before
// Build calls this; it is checked again here only because a builder method
// must never assume the invariant of a package it does not own.
func (b *builder) time(value string) time.Time {
	if b.err != nil {
		return time.Time{}
	}
	t, err := snapshot.Time(value)
	if err != nil {
		b.err = err
	}
	return t
}

// period writes the length between start and end, and keeps the first
// error.
func (b *builder) period(start, end time.Time) string {
	if b.err != nil {
		return ""
	}
	text, err := b.cat.Period(start, end)
	if err != nil {
		b.err = err
		return ""
	}
	return text
}

// resolve checks the snapshot and returns the profile and the minimum
// severity a document of it is built with: the options, or what the
// snapshot stores where an option is empty.
func resolve(s snapshot.Snapshot, opts Options) (profile, severity string, err error) {
	if err := s.Check(); err != nil {
		return "", "", err
	}

	profile = opts.Profile
	if profile == "" {
		profile = s.Report.Profile
	}
	if profile != "detailed" && profile != "executive" {
		return "", "", fmt.Errorf("unknown report profile %q", profile)
	}

	severity = opts.MinSeverity
	if severity == "" {
		severity = s.Report.MinSeverity
	}
	if !slices.Contains(severities, severity) {
		return "", "", fmt.Errorf("unknown minimum severity %q", severity)
	}
	return profile, severity, nil
}

// Build assembles the document of a snapshot: a cover, the verdict, the
// opening page, the executive section, and then, for the detailed profile,
// the four sections of the evidence, and a quality appendix with its notes.
// The executive profile keeps the cover, the verdict, the opening page, the
// executive section, and the appendix with its notes, and raises the
// minimum severity to at least attention for the whole document; the
// detailed profile raises it for the executive section alone.
func Build(s snapshot.Snapshot, c *catalog.Catalog, opts Options) (Document, error) {
	profile, severity, err := resolve(s, opts)
	if err != nil {
		return Document{}, err
	}

	b := &builder{cat: c}
	thresholds := thresholdsFromSnapshot(s.Report.Thresholds)

	shownSeverity := severity
	if profile == "executive" {
		shownSeverity = raiseToAttention(severity)
	}
	front := buildFront(b, s, profile, shownSeverity)
	blocks := []Block{buildVerdict(b, s, shownSeverity), PageBreak{Weak: false}, front, PageBreak{Weak: false}}
	blocks = append(blocks, buildExecutiveSection(b, s, thresholds, severity, profile == "executive")...)
	if profile == "detailed" {
		blocks = append(blocks, buildNamespacesSection(b, s.Namespaces, s.Findings, thresholds, severity)...)
		blocks = append(blocks, buildWorkloadsSection(b, s.Workloads, s.Findings, thresholds, severity)...)
		blocks = append(blocks, buildHPAsSection(b, s)...)
		blocks = append(blocks, buildNodepoolsSection(b, s, thresholds, severity)...)
	}
	appendixSeverity := severity
	if profile == "executive" {
		appendixSeverity = raiseToAttention(severity)
	}
	appendix := buildAppendix(b, s, appendixSeverity)
	if slices.ContainsFunc(appendix, func(block Block) bool { _, ok := block.(Table); return ok }) {
		blocks = append(blocks, PageBreak{Weak: true})
	}
	blocks = append(blocks, appendix...)
	blocks = append(blocks, buildNotes(b, s)...)

	cover := buildCover(b, s, profile)
	blocks = append(blocks, Colophon{Text: b.text(catalog.ColophonText, catalog.Params{"Issued": cover.Issued})})

	doc := Document{
		Schema:   Schema,
		Language: string(c.Language()),
		Theme:    buildTheme(c),
		Labels:   buildLabels(b),
		Cover:    cover,
		Footer:   buildFooter(b, s),
		Blocks:   blocks,
	}

	if b.err != nil {
		return Document{}, b.err
	}
	return doc, nil
}

// buildTheme reads the language and region codes the catalog carries for
// its own language.
func buildTheme(c *catalog.Catalog) Theme {
	lang, region := c.Theme()
	return Theme{Lang: lang, Region: region}
}

// buildLabels reads the three callout labels of the wording table.
func buildLabels(b *builder) Labels {
	return Labels{
		Note:    b.text(catalog.LabelsNote, nil),
		Info:    b.text(catalog.LabelsInfo, nil),
		Warning: b.text(catalog.LabelsWarning, nil),
	}
}

// buildCover reads the nine fields of the title block: the cluster name
// as the snapshot stored it, the emission time by its own wall clock, the
// analysis period, the profile word, and the author, null when the snapshot
// carries none.
func buildCover(b *builder, s snapshot.Snapshot, profile string) Cover {
	cover := Cover{
		Title:        b.text(catalog.CoverTitle, nil),
		ClusterLabel: b.text(catalog.CoverClusterLabel, nil),
		Cluster:      s.Scope.ClusterName,
		AuthorLabel:  b.text(catalog.CoverAuthorLabel, nil),
		IssuedLabel:  b.text(catalog.CoverIssuedLabel, nil),
		Issued:       b.cat.DateTime(b.time(s.Report.EmittedAt)),
		Period:       periodRange(b, s),
		Profile:      b.text(profileKeys[profile], nil),
	}
	if s.Report.Author != "" {
		author := s.Report.Author
		cover.Author = &author
	}
	return cover
}

// buildFooter reads the running footer: the classification, null when the
// snapshot carries none, and the page label.
func buildFooter(b *builder, s snapshot.Snapshot) Footer {
	footer := Footer{PageLabel: b.text(catalog.FooterPageLabel, nil)}
	if s.Report.Classification != "" {
		classification := s.Report.Classification
		footer.Classification = &classification
	}
	return footer
}
