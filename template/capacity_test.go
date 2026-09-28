//go:build template

package template

import (
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
func TestCapacityIdentifiersSurviveTheTextLayer(t *testing.T) {
	ws := workspace(t)
	for _, stem := range []string{"detailed.pt-BR", "detailed.en"} {
		t.Run(stem, func(t *testing.T) {
			path := capacityFixture(ws, stem)
			raw, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("reading %s: %v", path, err)
			}
			var doc any
			if err := json.Unmarshal(raw, &doc); err != nil {
				t.Fatalf("parsing %s: %v", path, err)
			}
			var idents []string
			collectIdents(doc, &idents)
			if len(idents) == 0 {
				t.Fatalf("%s: found no ident runs; the fixture or the collector is broken", stem)
			}

			pdf, stderr, err := compileCapacityData(t, ws, path)
			assertCleanCompile(t, stderr, err)
			pageText := pdfText(t, pdf)
			whole := stripWhitespace(pageText)
			leading := leadingCellStream(pageText)

			for _, ident := range idents {
				want := stripWhitespace(ident)
				if !strings.Contains(whole, want) && !strings.Contains(leading, want) {
					t.Errorf("%s: ident %q (whitespace removed) not found in the text layer", stem, ident)
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
// see the report for this task, which flags it for the controller.
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
// merely print empty.
func TestCapacityMinimalCoverOmitsAuthor(t *testing.T) {
	ws := workspace(t)
	pdf, stderr, err := compileCapacityData(t, ws, capacityFixture(ws, "minimal.pt-BR"))
	assertCleanCompile(t, stderr, err)

	page1 := pdfTextPage(t, pdf, 1)
	if strings.Contains(page1, "ELABORADO POR") {
		t.Errorf("page 1 text contains %q, want it omitted when cover.author is null", "ELABORADO POR")
	}
}

// TestCapacityFooterWithoutClassification checks minimal.pt-BR, whose
// footer.classification is null: page 2 (the first page with a footer at
// all) must show the page label and number with no leading "classification
// · " and no stray separator.
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
// counter(page) > 1), so page 1 must show no page number at all.
func TestCapacityFirstPageHasNoFooter(t *testing.T) {
	ws := workspace(t)
	pdf, stderr, err := compileCapacityData(t, ws, capacityFixture(ws, "detailed.pt-BR"))
	assertCleanCompile(t, stderr, err)

	page1 := pdfTextPage(t, pdf, 1)
	if strings.Contains(page1, "Página 1") {
		t.Errorf("page 1 text contains %q, want no footer on the first page", "Página 1")
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
