//go:build template

package template

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"unicode"

	"github.com/marceloalvescf/k8s-capacity-report/internal/typst"
)

// capacityFixture returns the absolute path of a positive fixture under
// tests/capacity/fixtures/, given its stem (for example "detailed.pt-BR").
func capacityFixture(ws, stem string) string {
	return filepath.Join(ws, "tests", "capacity", "fixtures", stem+".json")
}

// TestCapacityFixturesCompileCleanly checks the five positive fixtures that
// have no test of their own dedicated to a compile failure: each must
// compile without a diagnostic and produce a non-empty PDF. detailed.pt-BR
// and detailed.en, identifiers.pt-BR, minimal.pt-BR, and literal.pt-BR are
// each also exercised, more specifically, by the tests below; literal.pt-BR
// is covered there instead of here since its own tests already assert a
// clean compile.
func TestCapacityFixturesCompileCleanly(t *testing.T) {
	ws := workspace(t)
	for _, stem := range []string{"base.pt-BR", "detailed.pt-BR", "detailed.en", "identifiers.pt-BR", "minimal.pt-BR"} {
		t.Run(stem, func(t *testing.T) {
			pdf, stderr, err := compileCapacityData(t, ws, capacityFixture(ws, stem))
			assertCleanCompile(t, stderr, err)
			assertNonEmptyPDF(t, pdf)
		})
	}
}

// fileDigest returns the SHA-256 of path, in hexadecimal.
func fileDigest(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// TestCapacityIsDeterministic compiles the same fixture twice and requires
// byte-identical PDFs: the report layer must never depend on wall-clock
// time, map iteration order, or anything else that could vary between two
// runs of the very same input.
func TestCapacityIsDeterministic(t *testing.T) {
	ws := workspace(t)
	fixture := capacityFixture(ws, "detailed.pt-BR")

	pdf1, stderr1, err1 := compileCapacityData(t, ws, fixture)
	assertCleanCompile(t, stderr1, err1)
	digest1 := fileDigest(t, pdf1)

	pdf2, stderr2, err2 := compileCapacityData(t, ws, fixture)
	assertCleanCompile(t, stderr2, err2)
	digest2 := fileDigest(t, pdf2)

	if digest1 != digest2 {
		t.Errorf("digest = %s, then %s; want the same fixture to compile identically every time", digest1, digest2)
	}
}

// collectIdents appends the `text` of every run object with `"kind":
// "ident"` found anywhere in v, walking the generic structure produced by
// json.Unmarshal into an interface{} (so it works on any block or run,
// including one nested inside "front" or a table cell, without needing to
// know the schema's shape).
func collectIdents(v any, out *[]string) {
	switch val := v.(type) {
	case map[string]any:
		if kind, ok := val["kind"].(string); ok && kind == "ident" {
			if text, ok := val["text"].(string); ok {
				*out = append(*out, text)
			}
		}
		for _, sub := range val {
			collectIdents(sub, out)
		}
	case []any:
		for _, sub := range val {
			collectIdents(sub, out)
		}
	}
}

// collectRunTexts appends the `text` of every run object found anywhere in
// v, whatever its `kind`. Unlike collectIdents, it does not filter by
// kind: an identifier can legitimately also reach the page through a
// plain `text` run (for example a namespace name repeated inside a
// comma-joined "Namespaces: checkout, billing-core, ..." cell), so a count
// of how often an identifier appears in the data has to include every run,
// not only its own `ident` runs.
func collectRunTexts(v any, out *[]string) {
	switch val := v.(type) {
	case map[string]any:
		if _, hasKind := val["kind"]; hasKind {
			if text, ok := val["text"].(string); ok {
				*out = append(*out, text)
			}
		}
		for _, sub := range val {
			collectRunTexts(sub, out)
		}
	case []any:
		for _, sub := range val {
			collectRunTexts(sub, out)
		}
	}
}

// stripWhitespace removes every Unicode whitespace character from s. Both
// sides of a comparison against a PDF text layer are stripped this way,
// since Typst's line breaking freely inserts spaces and newlines that have
// no counterpart in the source string.
func stripWhitespace(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) {
			return -1
		}
		return r
	}, s)
}

// cellSplit divides one line of pdftotext -layout output into columns: two
// or more consecutive spaces is the column separator that mode uses to
// keep a table's columns aligned.
var cellSplit = regexp.MustCompile(`\s{2,}`)

// leadingCellStream concatenates the FIRST column of every non-blank line
// of text, in order, with no separator. It exists because a wide table
// row, wrapped by pdftotext -layout onto a second physical line, places
// that continuation at the very start of the next line — but stripping
// whitespace from the whole text naively concatenates the *entire* first
// line (every column) before the second, so the neighbouring columns'
// text lands in the middle of the wrapped value. Concatenating only each
// line's first column sidesteps that: nothing else can land between a
// value and its own continuation, since neither ever holds more than the
// first column's text. It is deliberately a second, narrower view of the
// text, used alongside (never instead of) stripWhitespace(pdfText(...))
// on the whole document, which still finds every identifier that does not
// happen to sit in the first column of a wrapped row.
func leadingCellStream(text string) string {
	var b strings.Builder
	for _, line := range strings.Split(text, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		b.WriteString(stripWhitespace(cellSplit.Split(trimmed, 2)[0]))
	}
	return b.String()
}

// TestCapacityIdentifiersSurviveTheTextLayer proves every `ident` run of
// detailed.pt-BR and detailed.en reaches the PDF's text layer intact: an
// identifier that vanished, or was mangled by hyphenation, would be a
// silent content loss that no compile error would ever catch.
//
// It counts occurrences rather than merely checking presence, because
// these fixtures' metadata table prints a comma-joined
// "checkout, billing-core, gateway-api" as plain text: every one of those
// three names would still be "found somewhere" even if every `ident` run
// printed nothing at all, which would make a presence-only check pass for
// the wrong reason. Counting how many times each identifier occurs in the
// data that reaches the page, and requiring at least that many occurrences
// in the rendered text, does not have that blind spot: if only the
// `ident`-run occurrences vanish, the text-layer count drops below the
// data-side count.
func TestCapacityIdentifiersSurviveTheTextLayer(t *testing.T) {
	ws := workspace(t)
	for _, stem := range []string{"detailed.pt-BR", "detailed.en"} {
		t.Run(stem, func(t *testing.T) {
			path := capacityFixture(ws, stem)
			raw, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("reading %s: %v", path, err)
			}
			var doc map[string]any
			if err := json.Unmarshal(raw, &doc); err != nil {
				t.Fatalf("parsing %s: %v", path, err)
			}

			var idents []string
			collectIdents(doc["blocks"], &idents)
			if len(idents) == 0 {
				t.Fatalf("%s: found no ident runs; the fixture or the collector is broken", stem)
			}

			// The data-side corpus: every run's text, of whatever kind,
			// from every block (including one nested inside "front", a
			// table cell, or a list item), plus cover.cluster, joined
			// with a NUL between consecutive strings so an occurrence can
			// never be formed by splicing the end of one string to the
			// start of the next.
			var runTexts []string
			collectRunTexts(doc["blocks"], &runTexts)
			if cover, ok := doc["cover"].(map[string]any); ok {
				if cluster, ok := cover["cluster"].(string); ok {
					runTexts = append(runTexts, cluster)
				}
			}
			data := strings.Join(runTexts, "\x00")

			pdf, stderr, err := compileCapacityData(t, ws, path)
			assertCleanCompile(t, stderr, err)
			pageText := pdfText(t, pdf)
			whole := stripWhitespace(pageText)
			// leadingCellStream is also counted, and the larger of the two
			// counts is kept: a wrapped identifier's second half can only
			// be recovered through leadingCellStream (see its doc
			// comment), while an identifier repeated across several
			// columns is only counted correctly by the whole-text strip
			// (leadingCellStream sees just the first column). Today no
			// identifier's occurrences are split across both situations
			// at once, so the larger count is always the accurate one.
			leading := leadingCellStream(pageText)

			seen := make(map[string]bool, len(idents))
			for _, ident := range idents {
				if seen[ident] {
					continue
				}
				seen[ident] = true

				dataCount := strings.Count(data, ident)
				textCount := strings.Count(whole, ident)
				if lc := strings.Count(leading, ident); lc > textCount {
					textCount = lc
				}
				if textCount < dataCount {
					t.Errorf("%s: ident %q occurs %d time(s) in the text layer, want at least %d (its count in the data)",
						stem, ident, textCount, dataCount)
				}
			}
		})
	}
}

// TestCapacityIdentDoesNotRepeatTheHyphen checks identifiers.pt-BR's row 1,
// an `ident` run: with `hyphenate: false`, Typst may still wrap the string
// at its own hyphen, but must not insert an additional one there.
func TestCapacityIdentDoesNotRepeatTheHyphen(t *testing.T) {
	ws := workspace(t)
	pdf, stderr, err := compileCapacityData(t, ws, capacityFixture(ws, "identifiers.pt-BR"))
	assertCleanCompile(t, stderr, err)
	// leadingCellStream, not a whole-text strip: this table has several
	// columns, and row 1's identifier wraps onto a second physical line,
	// so a whole-text strip would splice the neighbouring columns' text
	// in between the two halves. See leadingCellStream's doc comment.
	leading := leadingCellStream(pdfText(t, pdf))

	want := "aks-generalpool-29715314-vmss000001"
	if !strings.Contains(leading, want) {
		t.Errorf("text (whitespace removed) does not contain %q; the ident run must wrap, if at all, without adding a hyphen", want)
	}
}

// TestCapacityControlRepeatsTheHyphen checks identifiers.pt-BR's row 2, the
// same identifier as a plain `text` run: ordinary Portuguese hyphenation
// applies, and is expected to break right after the identifier's own
// hyphen, producing a doubled hyphen once whitespace is removed. This is
// the control for TestCapacityIdentDoesNotRepeatTheHyphen: if Typst never
// wrapped this identifier at all, that test would pass for the wrong
// reason, and this one catches it.
func TestCapacityControlRepeatsTheHyphen(t *testing.T) {
	ws := workspace(t)
	pdf, stderr, err := compileCapacityData(t, ws, capacityFixture(ws, "identifiers.pt-BR"))
	assertCleanCompile(t, stderr, err)
	leading := leadingCellStream(pdfText(t, pdf))

	if !strings.Contains(leading, "--") {
		t.Fatalf("text (whitespace removed) does not contain a doubled hyphen; " +
			"row 2's plain-text identifier was expected to hyphenate right after " +
			"its own hyphen. If this fails, TestCapacityIdentDoesNotRepeatTheHyphen " +
			"proves nothing, because Typst may simply not be wrapping the " +
			"identifier at all")
	}
}

// literalStrings holds every string of literal.pt-BR.json that must print
// exactly as written, i.e. with no character interpreted as Typst markup.
// It excludes the two strings that hold a real control character (a
// newline and a tab): those are checked instead, for the opposite property
// (not being turned into an escape sequence), by
// TestCapacityControlCharactersAreNotEscaped.
//
// It checks "ação çedilha ñ ü" but not the "日本語" suffix of that same
// fixture string: template/graphite/assets/fonts ships only Liberation
// Sans, a Latin-only family, so those three CJK characters have no glyph
// in the report's font at all and render as empty boxes (confirmed by
// rendering this fixture to PNG at 300 PPI). That is a font-coverage gap,
// not markup being interpreted, so it is out of this test's scope — but
// see the report for this task, which flags it for the controller. The
// limit is documented in template/graphite/README.md under "Known
// limitations".
var literalStrings = []string{
	"#let x = 1",
	"*negrito*",
	"_sublinhado_",
	"$x^2$",
	"<rotulo>",
	"@referencia",
	"`codigo`",
	"[colchetes]",
	"// comentario",
	"\\n literal", // a real backslash followed by the letter n, not a control character
	"https://example.com/a?b=c&d=e",
	"ação çedilha ñ ü",
}

// TestCapacityPrintsMarkupLiterally proves that every string of
// literal.pt-BR.json which looks like Typst markup, a URL, or non-ASCII
// text reaches the page unparsed and unmangled: the report layer only ever
// wraps a run's `text` in `text(...)`, never treats it as markup.
func TestCapacityPrintsMarkupLiterally(t *testing.T) {
	ws := workspace(t)
	pdf, stderr, err := compileCapacityData(t, ws, capacityFixture(ws, "literal.pt-BR"))
	assertCleanCompile(t, stderr, err)
	assertNonEmptyPDF(t, pdf)

	text := pdfText(t, pdf)
	for _, s := range literalStrings {
		if !strings.Contains(text, s) {
			t.Errorf("text does not contain %q", s)
		}
	}
}

// TestCapacityControlCharactersAreNotEscaped proves that a real newline or
// tab byte inside a JSON string is never turned into the two-character
// escape sequence "\n" or "\t" on the page.
//
// literal.pt-BR.json also holds the separate string "\n literal" (a real
// backslash followed by the letter n, not a control character), which
// TestCapacityPrintsMarkupLiterally proves prints exactly as written -
// meaning the literal two characters "\" and "n" legitimately appear
// elsewhere in this same document. That occurrence is removed from the
// text before the checks below, so it can never be mistaken for the bug
// this test exists to catch.
func TestCapacityControlCharactersAreNotEscaped(t *testing.T) {
	ws := workspace(t)
	pdf, stderr, err := compileCapacityData(t, ws, capacityFixture(ws, "literal.pt-BR"))
	assertCleanCompile(t, stderr, err)

	text := pdfText(t, pdf)
	if !strings.Contains(text, "linha um") {
		t.Errorf("text does not contain %q", "linha um")
	}
	if !strings.Contains(text, "linha dois") {
		t.Errorf("text does not contain %q", "linha dois")
	}

	// The real tab of "tab\tulado" is not assumed to lay out as anything
	// in particular: pdftotext -layout was checked by hand and prints it
	// as nothing at all ("tabulado", confirmed byte for byte with `xxd`),
	// but a real tab printing as plain white space would be just as
	// correct. Stripping all white space first makes the assertion hold
	// either way.
	if !strings.Contains(stripWhitespace(text), "tabulado") {
		t.Errorf("text (whitespace removed) does not contain %q", "tabulado")
	}

	scrubbed := strings.ReplaceAll(text, "\\n literal", "")
	if strings.Contains(scrubbed, "\\n") {
		t.Errorf("text contains the escape sequence %q; a real newline must not be turned into it", "\\n")
	}
	if strings.Contains(scrubbed, "\\t") {
		t.Errorf("text contains the escape sequence %q; a real tab must not be turned into it", "\\t")
	}
}

// TestCapacityMinimalCoverOmitsAuthor checks minimal.pt-BR, whose
// cover.author is null: the author line must take up no space at all, not
// merely print empty. The absence check alone would pass just as well on a
// blank page, so it is paired with two positive checks — the cover title
// and the issue label, both from the fixture — that only pass if the
// cover actually rendered.
func TestCapacityMinimalCoverOmitsAuthor(t *testing.T) {
	ws := workspace(t)
	pdf, stderr, err := compileCapacityData(t, ws, capacityFixture(ws, "minimal.pt-BR"))
	assertCleanCompile(t, stderr, err)

	page1 := pdfTextPage(t, pdf, 1)
	if !strings.Contains(page1, "LEVANTAMENTO DE CAPACIDADE") {
		t.Errorf("page 1 text does not contain %q", "LEVANTAMENTO DE CAPACIDADE")
	}
	if !strings.Contains(page1, "EMITIDO EM:") {
		t.Errorf("page 1 text does not contain %q", "EMITIDO EM:")
	}
	if strings.Contains(page1, "ELABORADO POR") {
		t.Errorf("page 1 text contains %q, want it omitted when cover.author is null", "ELABORADO POR")
	}
}

// TestCapacityFooterWithoutClassification checks minimal.pt-BR, whose
// footer.classification is null: page 2 (the first page with a footer at
// all) must show the page label and number with no leading "classification
// · " and no stray separator. The "Página 2" check is the positive that
// pairs with the "no ·" absence check below it, on the same page: it
// proves the footer is there at all, not merely that this one separator
// is missing from a blank page.
func TestCapacityFooterWithoutClassification(t *testing.T) {
	ws := workspace(t)
	pdf, stderr, err := compileCapacityData(t, ws, capacityFixture(ws, "minimal.pt-BR"))
	assertCleanCompile(t, stderr, err)

	page2 := pdfTextPage(t, pdf, 2)
	if !strings.Contains(page2, "Página 2") {
		t.Errorf("page 2 text does not contain %q", "Página 2")
	}
	if strings.Contains(page2, "·") {
		t.Errorf("page 2 text contains %q, want it omitted when footer.classification is null", "·")
	}
}

// TestCapacityFooterWithClassification checks detailed.pt-BR, whose
// footer.classification is set: page 2 must show it, the separator, the
// page label, and the number, in that order.
func TestCapacityFooterWithClassification(t *testing.T) {
	ws := workspace(t)
	pdf, stderr, err := compileCapacityData(t, ws, capacityFixture(ws, "detailed.pt-BR"))
	assertCleanCompile(t, stderr, err)

	page2 := pdfTextPage(t, pdf, 2)
	if !strings.Contains(page2, "Uso interno · Página 2") {
		t.Errorf("page 2 text does not contain %q", "Uso interno · Página 2")
	}
}

// TestCapacityFirstPageHasNoFooter checks detailed.pt-BR: the footer only
// starts on the second page (report-footer in main.typ gates on
// counter(page) > 1), so page 1 must show no page number at all. The
// absence check alone would pass on a blank page 1, or even if the footer
// never appeared anywhere, so it is paired with the cover title on page 1
// and the same footer text (proven present by
// TestCapacityFooterWithClassification) on page 2 — together proving the
// footer exists and starts on the second page, not merely that page 1
// lacks it.
func TestCapacityFirstPageHasNoFooter(t *testing.T) {
	ws := workspace(t)
	pdf, stderr, err := compileCapacityData(t, ws, capacityFixture(ws, "detailed.pt-BR"))
	assertCleanCompile(t, stderr, err)

	page1 := pdfTextPage(t, pdf, 1)
	if !strings.Contains(page1, "LEVANTAMENTO DE CAPACIDADE") {
		t.Errorf("page 1 text does not contain %q", "LEVANTAMENTO DE CAPACIDADE")
	}
	if strings.Contains(page1, "Página 1") {
		t.Errorf("page 1 text contains %q, want no footer on the first page", "Página 1")
	}

	page2 := pdfTextPage(t, pdf, 2)
	if !strings.Contains(page2, "Uso interno · Página 2") {
		t.Errorf("page 2 text does not contain %q, want the footer to start on the second page", "Uso interno · Página 2")
	}
}

// TestCapacityEnglishUsesEnglishLabels checks detailed.en: its labels,
// language, and date format must act together to keep every Portuguese
// word off the page, not just translate the ones a reviewer happens to
// look at.
func TestCapacityEnglishUsesEnglishLabels(t *testing.T) {
	ws := workspace(t)
	pdf, stderr, err := compileCapacityData(t, ws, capacityFixture(ws, "detailed.en"))
	assertCleanCompile(t, stderr, err)

	page2 := pdfTextPage(t, pdf, 2)
	if !strings.Contains(page2, "Page 2") {
		t.Errorf("page 2 text does not contain %q", "Page 2")
	}

	full := pdfText(t, pdf)
	if !strings.Contains(full, "Page") {
		t.Errorf("text does not contain %q, want the English page label somewhere in the document", "Page")
	}
	if strings.Contains(full, "Página") {
		t.Errorf("text contains %q, want the English fixture to never print the Portuguese page label", "Página")
	}
}

// balancedCall returns the text of a parenthesised argument list, given the
// text immediately following its opening "(" (already consumed by the
// caller). It reports an error if the parentheses never balance.
func balancedCall(afterOpenParen string) (string, error) {
	depth := 1
	for i, r := range afterOpenParen {
		switch r {
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				return afterOpenParen[:i], nil
			}
		}
	}
	return "", fmt.Errorf("unbalanced parentheses")
}

// TestCapacityPassesEveryLabel reads capacity/main.typ as text and checks
// the call to graphite-report.with(...): it must pass both `labels` and
// `theme`, and theme's three keys (`lang`, `region`, `date-format`) are
// written inline there. main.typ does not build `labels` inline, though:
// it assembles the seven label keys into `report-labels` first, and passes
// that variable, so those seven keys are checked against the text of the
// file that precedes the call instead of the call's own argument list.
func TestCapacityPassesEveryLabel(t *testing.T) {
	ws := workspace(t)
	raw, err := os.ReadFile(filepath.Join(ws, "capacity", "main.typ"))
	if err != nil {
		t.Fatalf("reading main.typ: %v", err)
	}
	source := string(raw)

	const marker = "graphite-report.with("
	markerAt := strings.Index(source, marker)
	if markerAt == -1 {
		t.Fatalf("main.typ does not call %q", marker)
	}
	call, err := balancedCall(source[markerAt+len(marker):])
	if err != nil {
		t.Fatalf("main.typ: %v", err)
	}

	for _, arg := range []string{"labels", "theme"} {
		if !strings.Contains(call, arg) {
			t.Errorf("graphite-report.with(...) does not pass an argument named %q", arg)
		}
	}
	for _, key := range []string{"lang", "region", "date-format"} {
		if !strings.Contains(call, key) {
			t.Errorf("graphite-report.with(...)'s theme does not set %q", key)
		}
	}

	before := source[:markerAt]
	for _, key := range []string{"author", "date", "classification", "outline", "note", "info", "warning"} {
		if !strings.Contains(before, key+":") {
			t.Errorf("main.typ does not build a %q label before calling graphite-report.with(...)", key)
		}
	}
}

// TestCapacityCompilesFromUnpackedTemplate proves that the embedded copy
// of the template — the one Unpack() writes to disk, and the one
// production actually compiles from — produces the exact same PDF as the
// source tree. TestEmbeddedFilesMatchDisk (embed_test.go) already proves
// the embedded bytes equal the source tree byte for byte, but that alone
// would not catch Unpack() itself mishandling a file on the way to disk
// (wrong relative path, a missing parent folder, truncated content): only
// an independent compile-and-compare of the two catches that.
func TestCapacityCompilesFromUnpackedTemplate(t *testing.T) {
	fixture := filepath.Join(moduleRoot(t), "template", "tests", "capacity", "fixtures", "detailed.pt-BR.json")
	data, err := os.ReadFile(fixture)
	if err != nil {
		t.Fatalf("reading %s: %v", fixture, err)
	}

	// compilePDF (the harness helper every other test in this file uses)
	// takes a source path relative to its root and joins it itself, but
	// Entry(dir) returns an already-absolute path, so it cannot be handed
	// to compilePDF without being joined onto the root a second time. The
	// job is built directly here instead, using Entry and FontPath exactly
	// as a real caller of Unpack would.
	unpacked := t.TempDir()
	if err := Unpack(unpacked); err != nil {
		t.Fatalf("Unpack() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(unpacked, "report.json"), data, 0o644); err != nil {
		t.Fatalf("writing report.json: %v", err)
	}

	tc := toolchain(t)
	unpackedOutput := filepath.Join(t.TempDir(), "unpacked.pdf")
	unpackedJob := typst.Job{
		Root:     unpacked,
		FontPath: FontPath(unpacked),
		Source:   Entry(unpacked),
		Output:   unpackedOutput,
		Format:   typst.PDF,
		Inputs:   map[string]string{"data": "/report.json"},
	}
	unpackedStderr, err := tc.Compile(context.Background(), unpackedJob)
	assertCleanCompile(t, unpackedStderr, err)

	ws := workspace(t)
	sourcePDF, sourceStderr, err := compileCapacityData(t, ws, fixture)
	assertCleanCompile(t, sourceStderr, err)

	unpackedDigest := fileDigest(t, unpackedOutput)
	sourceDigest := fileDigest(t, sourcePDF)
	if unpackedDigest != sourceDigest {
		t.Errorf("unpacked-template digest = %s, source-tree digest = %s, want equal", unpackedDigest, sourceDigest)
	}
}

// pdfTextPage returns one page's text through pdftotext -f <n> -l <n>
// -layout, so a footer or cover assertion reads only the page it names
// instead of the whole document.
func pdfTextPage(t *testing.T, pdf string, page int) string {
	t.Helper()
	p := strconv.Itoa(page)
	out, err := exec.Command("pdftotext", "-f", p, "-l", p, "-layout", pdf, "-").Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			t.Fatalf("pdftotext -f %s -l %s -layout %s -: %v, stderr: %s", p, p, pdf, exitErr, exitErr.Stderr)
		}
		t.Fatalf("pdftotext -f %s -l %s -layout %s -: %v", p, p, pdf, err)
	}
	return string(out)
}
