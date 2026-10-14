//go:build template

// This file exercises `report` end to end with the real typst compiler and
// the real pdftotext, from every scenario of promtest.Scenarios: through
// the real client, the real template, and the real compiler, to a PDF
// whose text is read back and checked. It needs no Docker; the native
// toolchain the Makefile's test-template target also expects is enough.
package cli

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
	"unicode"

	"github.com/marceloalvescf/capacity-room/internal/catalog"
	"github.com/marceloalvescf/capacity-room/internal/discovery"
	"github.com/marceloalvescf/capacity-room/internal/goldentest"
	"github.com/marceloalvescf/capacity-room/internal/promtest"
	"github.com/marceloalvescf/capacity-room/internal/snapshot"
)

// realEnv is testEnv with Getenv and LookPath replaced by the real process
// environment and PATH, the way typst.Locate finds the compiler in
// production (by CAPACITYROOM_TYPST, or on PATH); every other field keeps the fixed
// test value testEnv gives it: the clock, the report identifier, a fresh
// TempDir, and the version.
func realEnv(t *testing.T) Env {
	t.Helper()
	env := testEnv(t)
	env.Getenv = os.Getenv
	env.LookPath = exec.LookPath
	return env
}

// pdfText returns the whole text layer of pdf through pdftotext -raw, page
// breaks included as form feeds. -raw keeps the order the PDF's own content
// stream writes a page's text runs in, rather than reconstructing a
// reading order from their positions on the page; -layout does the latter
// row by row, and even plain mode (pdftotext's default) reorders text into
// paragraph-like blocks, so both can place a neighbouring table column's
// own text between the two halves of a cell that wrapped onto a second
// physical line — confirmed directly against this file's own PDFs: the
// pt-BR wording of TestRealReportShowsPartialData wraps its reason
// sentence exactly this way under both of those modes, while -raw keeps it
// contiguous once whitespace is stripped for a comparison. It fails the
// test (never skips it) when pdftotext is missing or exits non-zero: a
// machine without it does not get a free pass.
func pdfText(t *testing.T, pdf string) string {
	t.Helper()
	out, err := exec.Command("pdftotext", "-raw", pdf, "-").Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			t.Fatalf("pdftotext -raw %s -: %v, stderr: %s", pdf, exitErr, exitErr.Stderr)
		}
		t.Fatalf("pdftotext -raw %s -: %v", pdf, err)
	}
	return string(out)
}

// stripWhitespace removes every Unicode whitespace character from s, so a
// value pdftotext wrapped onto a second physical line inside its own table
// cell still matches contiguously. It is a narrower duplicate of
// template/capacity_test.go's helper of the same name, kept here because a
// Go test helper cannot be imported across packages.
func stripWhitespace(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) {
			return -1
		}
		return r
	}, s)
}

// containsText reports whether text holds want once both are stripped of
// whitespace: the shape every check in this file uses for a sentence or a
// name that might have wrapped, as opposed to a raw strings.Contains, which
// a wrapped value would fail even though it reached the page intact.
func containsText(text, want string) bool {
	return strings.Contains(stripWhitespace(text), stripWhitespace(want))
}

// requirePDF fails the test unless path names a file that starts with the
// five bytes every PDF does.
func requirePDF(t *testing.T, path string) {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	if !strings.HasPrefix(string(raw), "%PDF-") {
		n := len(raw)
		if n > 16 {
			n = 16
		}
		t.Fatalf("%s does not start with %%PDF-: %q", path, raw[:n])
	}
}

// TestRealReportFromEachScenario compiles a real PDF, in each language, from
// every scenario of promtest.Scenarios, and checks that the cover title and
// the nine capability names of that language, taken from the catalog by
// their keys, and the cluster name every scenario config carries ("prod"),
// all reach the text layer.
func TestRealReportFromEachScenario(t *testing.T) {
	for _, scenario := range promtest.Scenarios {
		for _, lang := range catalog.Languages() {
			t.Run(scenario.Name+"/"+string(lang), func(t *testing.T) {
				server, _, scenario := startScenario(t, scenario.Name)
				defer server.Close()

				outDir := t.TempDir()
				cfgPath := writeConfig(t, scenarioConfig(server, scenario, outDir))
				outPath := filepath.Join(outDir, "r.pdf")

				env := realEnv(t)
				args := scenarioArgs(cfgPath, scenario, "--lang", string(lang), "--output", outPath)
				_, stderr, code := runArgs(context.Background(), env, args)
				if code != 0 {
					t.Fatalf("code = %d, want 0; stderr = %q", code, stderr)
				}
				requirePDF(t, outPath)

				text := pdfText(t, outPath)
				if !containsText(text, "prod") {
					t.Errorf("text does not contain %q", "prod")
				}

				cat, err := catalog.Load(lang)
				if err != nil {
					t.Fatalf("catalog.Load(%s): %v", lang, err)
				}
				title, err := cat.Text(catalog.CoverTitle, nil)
				if err != nil {
					t.Fatalf("CoverTitle text: %v", err)
				}
				if !containsText(text, title) {
					t.Errorf("text does not contain the cover title %q", title)
				}
				for _, name := range discovery.Names {
					capText, err := cat.Text(catalog.CapabilityName(name), nil)
					if err != nil {
						t.Fatalf("capability name text for %s: %v", name, err)
					}
					if !containsText(text, capText) {
						t.Errorf("text does not contain the capability name %q", capText)
					}
				}
			})
		}
	}
}

// TestRealReportPageCount checks that direct-complete, whose fixture backs
// every one of the nine capabilities and gives payments-web its own rows,
// renders as exactly five pages in the detailed profile: the verdict; the
// opening page; the page of section 1, the executive summary; the page of
// sections 2 to 5; and the quality appendix, with its interpretation notes.
// The executive profile drops sections 2 to 5, so it renders as four pages:
// the verdict, the opening page, the executive summary, and the appendix.
func TestRealReportPageCount(t *testing.T) {
	server, _, scenario := startScenario(t, "direct-complete")
	defer server.Close()

	outDir := t.TempDir()
	cfgPath := writeConfig(t, scenarioConfig(server, scenario, outDir))
	outPath := filepath.Join(outDir, "r.pdf")

	env := realEnv(t)
	_, stderr, code := runArgs(context.Background(), env, scenarioArgs(cfgPath, scenario, "--output", outPath))
	if code != 0 {
		t.Fatalf("code = %d, want 0; stderr = %q", code, stderr)
	}

	text := pdfText(t, outPath)
	if got := strings.Count(text, "\f"); got != 5 {
		t.Fatalf("form feed count = %d, want 5 (the verdict, the opening page, the executive summary, sections 2 to 5, and the quality appendix)", got)
	}
}

// TestRealReportShowsPartialData checks, in both languages, that
// partial-data shows both the reason sentence of a capability whose
// metrics were never found and the coverage status of a capability in the
// unavailable state, each read from the catalog by key rather than
// re-typed, the way TestRealReportFromEachScenario reads the cover title
// and the capability names above it.
func TestRealReportShowsPartialData(t *testing.T) {
	for _, lang := range catalog.Languages() {
		t.Run(string(lang), func(t *testing.T) {
			server, _, scenario := startScenario(t, "partial-data")
			defer server.Close()

			outDir := t.TempDir()
			cfgPath := writeConfig(t, scenarioConfig(server, scenario, outDir))
			outPath := filepath.Join(outDir, "r.pdf")

			env := realEnv(t)
			args := scenarioArgs(cfgPath, scenario, "--lang", string(lang), "--output", outPath)
			_, stderr, code := runArgs(context.Background(), env, args)
			if code != 0 {
				t.Fatalf("code = %d, want 0; stderr = %q", code, stderr)
			}

			cat, err := catalog.Load(lang)
			if err != nil {
				t.Fatalf("catalog.Load(%s): %v", lang, err)
			}
			reason, err := cat.Text(catalog.CapabilityReasonMetricsNotFound, nil)
			if err != nil {
				t.Fatalf("METRICS_NOT_FOUND text: %v", err)
			}
			unavailable, err := cat.Text(catalog.State(discovery.StateUnavailable), nil)
			if err != nil {
				t.Fatalf("state.unavailable text: %v", err)
			}

			text := pdfText(t, outPath)
			for _, want := range []string{reason, unavailable} {
				if !containsText(text, want) {
					t.Errorf("text does not contain %q", want)
				}
			}
		})
	}
}

// TestRealReportFromSnapshotIsTheSame checks that PDF mode and the two-step
// path (--snapshot, then --from-snapshot) of the same scenario, under the
// same fixed clock, identifier, and version, compile to byte-identical
// PDFs. Each collecting run gets its own server, since a scripted answer is
// used once.
func TestRealReportFromSnapshotIsTheSame(t *testing.T) {
	directPDF := func(t *testing.T) []byte {
		server, _, scenario := startScenario(t, "direct-complete")
		defer server.Close()
		outDir := t.TempDir()
		cfgPath := writeConfig(t, scenarioConfig(server, scenario, outDir))
		outPath := filepath.Join(outDir, "direct.pdf")

		env := realEnv(t)
		_, stderr, code := runArgs(context.Background(), env, scenarioArgs(cfgPath, scenario, "--output", outPath))
		if code != 0 {
			t.Fatalf("PDF mode: code = %d, want 0; stderr = %q", code, stderr)
		}
		data, err := os.ReadFile(outPath)
		if err != nil {
			t.Fatalf("reading %s: %v", outPath, err)
		}
		return data
	}

	twoStepPDF := func(t *testing.T) []byte {
		server, _, scenario := startScenario(t, "direct-complete")
		defer server.Close()
		snapDir := t.TempDir()
		cfgPath := writeConfig(t, scenarioConfig(server, scenario, snapDir))
		snapPath := filepath.Join(snapDir, "s.json")

		env := realEnv(t)
		_, stderr, code := runArgs(context.Background(), env, scenarioArgs(cfgPath, scenario, "--snapshot", snapPath))
		if code != 0 {
			t.Fatalf("snapshot mode: code = %d, want 0; stderr = %q", code, stderr)
		}

		outPath := filepath.Join(t.TempDir(), "from.pdf")
		_, stderr, code = runArgs(context.Background(), env, []string{"report", "--from-snapshot", snapPath, "--output", outPath})
		if code != 0 {
			t.Fatalf("from-snapshot mode: code = %d, want 0; stderr = %q", code, stderr)
		}
		data, err := os.ReadFile(outPath)
		if err != nil {
			t.Fatalf("reading %s: %v", outPath, err)
		}
		return data
	}

	a := directPDF(t)
	b := twoStepPDF(t)
	if sha256.Sum256(a) != sha256.Sum256(b) {
		t.Fatalf("PDF mode and snapshot-then-from-snapshot produced different documents")
	}
}

// TestRealReportInFolderWithSpace checks that a PDF run whose work folder
// sits under a path holding a space still succeeds, and that the folder is
// empty once the run is done.
func TestRealReportInFolderWithSpace(t *testing.T) {
	server, _, scenario := startScenario(t, "direct-complete")
	defer server.Close()

	outDir := t.TempDir()
	cfgPath := writeConfig(t, scenarioConfig(server, scenario, outDir))
	outPath := filepath.Join(outDir, "r.pdf")

	workDir := filepath.Join(t.TempDir(), "my work folder")
	if err := os.MkdirAll(workDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	env := realEnv(t)
	env.TempDir = workDir

	_, stderr, code := runArgs(context.Background(), env, scenarioArgs(cfgPath, scenario, "--output", outPath))
	if code != 0 {
		t.Fatalf("code = %d, want 0; stderr = %q", code, stderr)
	}
	assertTempDirEmpty(t, workDir)
}

// TestRealReportWithAVeryLongName checks that a cluster name of 400
// characters, rendered from a snapshot, still compiles: the template lets
// such a name overflow its column rather than refusing it, and typst never
// turns that overflow into a warning. It also confirms the work folder is
// removed as usual.
func TestRealReportWithAVeryLongName(t *testing.T) {
	longName := strings.Repeat("A", 400)
	snapPath := modifiedSnapshot(t, "direct-complete", func(s *snapshot.Snapshot) {
		s.Scope.ClusterName = longName
	})

	outPath := filepath.Join(t.TempDir(), "r.pdf")
	tempDir := t.TempDir()

	env := realEnv(t)
	env.TempDir = tempDir

	_, stderr, code := runArgs(context.Background(), env, []string{"report", "--from-snapshot", snapPath, "--lang", "en", "--output", outPath})
	if code != 0 {
		t.Fatalf("code = %d, want 0; stderr = %q", code, stderr)
	}
	assertTempDirEmpty(t, tempDir)
}

// TestRealReportPrintsNamesAsTheyAre checks that a cluster name built of
// punctuation typst would otherwise read as markup reaches the text layer
// character for character: the template never evaluates a value as markup.
func TestRealReportPrintsNamesAsTheyAre(t *testing.T) {
	const clusterName = `#prod *x* $y$ <a> @b [c]`
	snapPath := modifiedSnapshot(t, "direct-complete", func(s *snapshot.Snapshot) {
		s.Scope.ClusterName = clusterName
	})

	outPath := filepath.Join(t.TempDir(), "r.pdf")
	env := realEnv(t)
	_, stderr, code := runArgs(context.Background(), env, []string{"report", "--from-snapshot", snapPath, "--lang", "en", "--output", outPath})
	if code != 0 {
		t.Fatalf("code = %d, want 0; stderr = %q", code, stderr)
	}

	text := pdfText(t, outPath)
	if !strings.Contains(text, clusterName) {
		t.Fatalf("text does not contain %q, character for character", clusterName)
	}
}

// TestRealReportIsDeterministic runs the same collecting input through the
// real compiler twice, each against its own server, and checks the two
// PDFs are byte-identical.
func TestRealReportIsDeterministic(t *testing.T) {
	run := func(t *testing.T) []byte {
		server, _, scenario := startScenario(t, "direct-complete")
		defer server.Close()
		outDir := t.TempDir()
		cfgPath := writeConfig(t, scenarioConfig(server, scenario, outDir))
		outPath := filepath.Join(outDir, "r.pdf")

		env := realEnv(t)
		_, stderr, code := runArgs(context.Background(), env, scenarioArgs(cfgPath, scenario, "--output", outPath))
		if code != 0 {
			t.Fatalf("code = %d, want 0; stderr = %q", code, stderr)
		}
		data, err := os.ReadFile(outPath)
		if err != nil {
			t.Fatalf("reading %s: %v", outPath, err)
		}
		return data
	}

	first := run(t)
	second := run(t)
	if sha256.Sum256(first) != sha256.Sum256(second) {
		t.Fatalf("two runs of the same input produced different PDFs")
	}
}

// TestFromSnapshotPrintsTheSections builds a snapshot of this part with a
// --snapshot run against the recorded server, renders it with
// --from-snapshot, and checks the PDF text holds a workload name from the
// fixture and the namespaces section's own heading: it reads no golden
// file, which is still the shape of an earlier part.
func TestFromSnapshotPrintsTheSections(t *testing.T) {
	server, _, scenario := startScenario(t, "direct-complete")
	defer server.Close()

	outDir := t.TempDir()
	cfgPath := writeConfig(t, scenarioConfig(server, scenario, outDir))
	snapPath := filepath.Join(outDir, "s.json")

	env := realEnv(t)
	_, stderr, code := runArgs(context.Background(), env, scenarioArgs(cfgPath, scenario, "--snapshot", snapPath))
	if code != 0 {
		t.Fatalf("snapshot mode: code = %d, want 0; stderr = %q", code, stderr)
	}

	outPath := filepath.Join(t.TempDir(), "r.pdf")
	_, stderr, code = runArgs(context.Background(), env, []string{"report", "--from-snapshot", snapPath, "--lang", "en", "--output", outPath})
	if code != 0 {
		t.Fatalf("from-snapshot mode: code = %d, want 0; stderr = %q", code, stderr)
	}

	cat, err := catalog.Load(catalog.English)
	if err != nil {
		t.Fatalf("catalog.Load: %v", err)
	}
	heading, err := cat.Text(catalog.SectionsNamespacesHeading, nil)
	if err != nil {
		t.Fatalf("SectionsNamespacesHeading text: %v", err)
	}
	executive, err := cat.Text(catalog.SectionsExecutiveHeading, nil)
	if err != nil {
		t.Fatalf("SectionsExecutiveHeading text: %v", err)
	}

	text := pdfText(t, outPath)
	if !containsText(text, "payments-web") {
		t.Errorf("text does not contain %q", "payments-web")
	}
	if !containsText(text, heading) {
		t.Errorf("text does not contain %q", heading)
	}
	if !containsText(text, executive) {
		t.Errorf("text does not contain %q", executive)
	}
}

// TestRealReportExecutiveProfile checks, in both languages, that the
// executive profile prints the executive summary and the appendix and none
// of the headings of sections 2 to 5, and that the outline on page 2 lists
// the two sections it prints and no more.
func TestRealReportExecutiveProfile(t *testing.T) {
	for _, lang := range catalog.Languages() {
		t.Run(string(lang), func(t *testing.T) {
			server, _, scenario := startScenario(t, "direct-complete")
			defer server.Close()

			outDir := t.TempDir()
			cfgPath := writeConfig(t, scenarioConfig(server, scenario, outDir))
			outPath := filepath.Join(outDir, "r.pdf")
			args := scenarioArgs(cfgPath, scenario, "--lang", string(lang), "--report-profile", "executive", "--output", outPath)
			_, stderr, code := runArgs(context.Background(), realEnv(t), args)
			if code != 0 {
				t.Fatalf("code = %d, want 0; stderr = %q", code, stderr)
			}

			cat, err := catalog.Load(lang)
			if err != nil {
				t.Fatalf("catalog.Load(%s): %v", lang, err)
			}
			pages := pdfPages(pdfText(t, outPath))
			if len(pages) != 3 {
				t.Fatalf("PDF has %d page(s), want 3", len(pages))
			}

			for _, h := range []struct {
				number int
				key    catalog.Key
			}{
				{1, catalog.SectionsExecutiveHeading},
				{2, catalog.AppendixHeading},
			} {
				want := numberedHeading(t, cat, h.number, h.key)
				if !containsText(pages[1], want) {
					t.Errorf("the outline on page 2 does not contain %q", want)
				}
				if headingPage(pages, want) == -1 {
					t.Errorf("no page after the opening page contains the heading %q", want)
				}
			}

			for number, key := range map[int]catalog.Key{
				2: catalog.SectionsNamespacesHeading,
				3: catalog.SectionsWorkloadsHeading,
				4: catalog.SectionsHPAsHeading,
				5: catalog.SectionsNodepoolsHeading,
			} {
				// The title of section 4 is also a capability name of the
				// quality table, so only the numbered heading counts.
				text, err := cat.Text(key, nil)
				if err != nil {
					t.Fatalf("%s text: %v", key, err)
				}
				absent := fmt.Sprintf("%d %s", number, text)
				for i, page := range pages[2:] {
					if containsText(page, absent) {
						t.Errorf("page %d contains the heading %q of a section the executive profile leaves out", i+3, absent)
					}
				}
			}
		})
	}
}

// wordAppears reports whether text holds word as a word of its own: an
// occurrence whose neighboring character, on each side, is neither a
// letter, a digit, a hyphen, a dot, nor an underscore — or is absent,
// word being at the very start or the very end of text. A name of
// Kubernetes may itself hold a hyphen or a dot, so the ordinary regular
// expression word boundary, which treats both as the edge of a word, is
// not enough: it would count api as a word of its own inside
// api-gateway or my.api, neither of which it is.
func wordAppears(text, word string) bool {
	const notWordChar = `[^A-Za-z0-9_.\-]`
	re := regexp.MustCompile(`(?:^|` + notWordChar + `)` + regexp.QuoteMeta(word) + `(?:$|` + notWordChar + `)`)
	return re.MatchString(text)
}

// numberedHeading builds the text a numbered top-level heading of the
// report — section 1, section 2, or the appendix — prints as: the number,
// a space, and the catalog's own heading text.
func numberedHeading(t *testing.T, cat *catalog.Catalog, number int, key catalog.Key) string {
	t.Helper()
	text, err := cat.Text(key, nil)
	if err != nil {
		t.Fatalf("%s text: %v", key, err)
	}
	return fmt.Sprintf("%d %s", number, text)
}

// pdfPages splits text, the whole output of pdftotext -raw, into one
// string per page, on the form feed pdftotext writes between pages —
// and, as it also writes one after the last page, drops the empty page
// that trailing form feed would otherwise leave behind.
func pdfPages(text string) []string {
	pages := strings.Split(text, "\f")
	if len(pages) > 0 && pages[len(pages)-1] == "" {
		pages = pages[:len(pages)-1]
	}
	return pages
}

// TestWordAppears checks wordAppears' own table: api is a word of its
// own surrounded by ordinary text, but is not one as a bare substring of
// a longer word (capital, rapid), nor as the part of a Kubernetes name
// that a hyphen or a dot joins to something else (api-gateway, my.api).
func TestWordAppears(t *testing.T) {
	tests := []struct {
		name string
		text string
		want bool
	}{
		{"surrounded by ordinary text", "... api 4 ...", true},
		{"substring holding the letters before an extra one", "capital", false},
		{"prefix of a name joined by a hyphen", "api-gateway", false},
		{"suffix of a name joined by a dot", "my.api", false},
		{"substring in the middle of a word", "rapid", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := wordAppears(tt.text, "api"); got != tt.want {
				t.Errorf("wordAppears(%q, %q) = %v, want %v", tt.text, "api", got, tt.want)
			}
		})
	}
}

// headingPage returns the 0-based index of the first page after the opening
// page, the second, whose text holds want, or -1 when none does. The opening
// page carries the outline, which names every heading without being its
// place.
func headingPage(pages []string, want string) int {
	for i := 2; i < len(pages); i++ {
		if containsText(pages[i], want) {
			return i
		}
	}
	return -1
}

// TestRealReportTextOfEveryScenario checks, for every one of the eight
// scenarios and both languages, that the compiled PDF: names the numbered
// headings of the six parts (executive summary 1, namespaces 2, workloads 3,
// HPA history 4, node pools 5, appendix 6) in the outline, on page 2; carries each of
// those headings, with its number, on a page after the second, in the order
// of the numbers; carries the name of every namespace and every workload
// the scenario's snapshot holds, as a word of its own, on the pages from
// section 2 through the start of section 4; carries the interpretation
// notes heading, after the appendix heading, exactly when the snapshot
// holds a warning or a notice, and on no page otherwise; and, for
// edge-values, carries neither of the two workload names its snapshot
// omits, as a word of its own, on any page.
//
// The names and the notes expectation come from
// testdata/snapshot/<scenario>.json, not from a second collecting run:
// TestSnapshotModeWritesTheGoldenFile (report_test.go) and
// TestGoldenFlowEqualsTheCommand (run_test.go) already keep that file
// equal to what a run of the command itself writes for the same scenario.
func TestRealReportTextOfEveryScenario(t *testing.T) {
	for _, scenario := range promtest.Scenarios {
		t.Run(scenario.Name, func(t *testing.T) {
			snapPath, err := goldentest.SnapshotPath(scenario.Name)
			if err != nil {
				t.Fatalf("SnapshotPath(%s): %v", scenario.Name, err)
			}
			raw, err := os.ReadFile(snapPath)
			if err != nil {
				t.Fatalf("reading %s: %v", snapPath, err)
			}
			snap, err := snapshot.Decode(raw)
			if err != nil {
				t.Fatalf("decoding %s: %v", snapPath, err)
			}
			wantNotes := len(snap.Quality.Warnings) > 0 || len(snap.Quality.Notices) > 0

			for _, lang := range catalog.Languages() {
				t.Run(string(lang), func(t *testing.T) {
					server, _, sc := startScenario(t, scenario.Name)
					defer server.Close()

					outDir := t.TempDir()
					cfgPath := writeConfig(t, scenarioConfig(server, sc, outDir))
					outPath := filepath.Join(outDir, "r.pdf")

					env := realEnv(t)
					args := scenarioArgs(cfgPath, sc, "--lang", string(lang), "--output", outPath)
					_, stderr, code := runArgs(context.Background(), env, args)
					if code != 0 {
						t.Fatalf("code = %d, want 0; stderr = %q", code, stderr)
					}

					pages := pdfPages(pdfText(t, outPath))
					if len(pages) < 3 {
						t.Fatalf("PDF has %d page(s), want at least 3", len(pages))
					}

					cat, err := catalog.Load(lang)
					if err != nil {
						t.Fatalf("catalog.Load(%s): %v", lang, err)
					}

					headings := []struct {
						number int
						key    catalog.Key
					}{
						{1, catalog.SectionsExecutiveHeading},
						{2, catalog.SectionsNamespacesHeading},
						{3, catalog.SectionsWorkloadsHeading},
						{4, catalog.SectionsHPAsHeading},
						{5, catalog.SectionsNodepoolsHeading},
						{6, catalog.AppendixHeading},
					}

					// Page 2's own outline names every heading, whether or
					// not the heading it names reaches a page: a separate
					// assertion from the one below, so a lost section and a
					// lost outline entry are never reported as the same
					// fault.
					for _, h := range headings {
						want := numberedHeading(t, cat, h.number, h.key)
						if !containsText(pages[1], want) {
							t.Errorf("the outline on page 2 does not contain %q", want)
						}
					}

					// The heading itself, on a later page, in the order of
					// the numbers.
					found := make([]int, len(headings))
					for i, h := range headings {
						want := numberedHeading(t, cat, h.number, h.key)
						found[i] = headingPage(pages, want)
						if found[i] == -1 {
							t.Errorf("no page after the opening page contains the heading %q", want)
						}
						if i > 0 && found[i] != -1 && found[i] < found[i-1] {
							t.Errorf("the heading %q is on page %d, before the heading of the part before it, on page %d", want, found[i]+1, found[i-1]+1)
						}
					}
					if slices.Contains(found, -1) {
						return
					}

					// The names of the rows: from the page of section 2
					// through the page where section 4 starts.
					rowPages := strings.Join(pages[found[1]:found[3]+1], "\f")
					for _, ns := range snap.Namespaces {
						if !wordAppears(rowPages, ns.Name) {
							t.Errorf("the pages of sections 2 and 3 do not contain the namespace name %q as a word of its own", ns.Name)
						}
					}
					for _, wl := range snap.Workloads {
						if !wordAppears(rowPages, wl.Name) {
							t.Errorf("the pages of sections 2 and 3 do not contain the workload name %q as a word of its own", wl.Name)
						}
					}

					notesHeading, err := cat.Text(catalog.NotesHeading, nil)
					if err != nil {
						t.Fatalf("NotesHeading text: %v", err)
					}
					if wantNotes {
						appendixWant := numberedHeading(t, cat, 6, catalog.AppendixHeading)
						strippedTail := stripWhitespace(strings.Join(pages[found[5]:], "\f"))
						appendixIdx := strings.Index(strippedTail, stripWhitespace(appendixWant))
						notesIdx := strings.Index(strippedTail, stripWhitespace(notesHeading))
						switch {
						case notesIdx == -1:
							t.Errorf("the appendix does not contain the interpretation notes heading %q, though the snapshot holds a warning or a notice", notesHeading)
						case appendixIdx != -1 && notesIdx < appendixIdx:
							t.Errorf("the interpretation notes heading %q appears before the appendix heading", notesHeading)
						}
					} else {
						for i, page := range pages {
							if containsText(page, notesHeading) {
								t.Errorf("page %d contains the interpretation notes heading %q, though the snapshot holds neither a warning nor a notice", i+1, notesHeading)
							}
						}
					}

					if scenario.Name == "edge-values" {
						for _, omitted := range []string{"idle", "nightly"} {
							for i, page := range pages {
								if wordAppears(page, omitted) {
									t.Errorf("page %d contains %q as a word of its own, though the snapshot omits that workload", i+1, omitted)
								}
							}
						}
					}
				})
			}
		})
	}
}

// TestRealReportHoldsSectionsThreeAndFour checks, for every scenario and
// both languages, that the headings of section 3 (HPA history) and of
// section 4 (node pools) each appear once in the text of the pages after
// the opening page.
func TestRealReportHoldsSectionsThreeAndFour(t *testing.T) {
	for _, scenario := range promtest.Scenarios {
		t.Run(scenario.Name, func(t *testing.T) {
			for _, lang := range catalog.Languages() {
				t.Run(string(lang), func(t *testing.T) {
					server, _, sc := startScenario(t, scenario.Name)
					defer server.Close()

					outDir := t.TempDir()
					cfgPath := writeConfig(t, scenarioConfig(server, sc, outDir))
					outPath := filepath.Join(outDir, "r.pdf")
					args := scenarioArgs(cfgPath, sc, "--lang", string(lang), "--output", outPath)
					_, stderr, code := runArgs(context.Background(), realEnv(t), args)
					if code != 0 {
						t.Fatalf("code = %d, want 0; stderr = %q", code, stderr)
					}

					cat, err := catalog.Load(lang)
					if err != nil {
						t.Fatalf("catalog.Load(%s): %v", lang, err)
					}
					pages := pdfPages(pdfText(t, outPath))
					body := stripWhitespace(strings.Join(pages[2:], "\f"))
					for _, h := range []struct {
						number int
						key    catalog.Key
					}{
						{4, catalog.SectionsHPAsHeading},
						{5, catalog.SectionsNodepoolsHeading},
					} {
						want := stripWhitespace(numberedHeading(t, cat, h.number, h.key))
						if got := strings.Count(body, want); got != 1 {
							t.Errorf("the heading %q appears %d times after the opening page, want 1", want, got)
						}
					}
				})
			}
		})
	}
}
