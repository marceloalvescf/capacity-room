//go:build template

// This file exercises `report` end to end with the real typst compiler and
// the real pdftotext, from each of the six scenarios: through the real
// client, the real template, and the real compiler, to a PDF whose text is
// read back and checked. It needs no Docker; the native toolchain the
// Makefile's test-template target also expects is enough.
package cli

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"unicode"

	"github.com/marceloalvescf/k8s-capacity-report/internal/catalog"
	"github.com/marceloalvescf/k8s-capacity-report/internal/discovery"
	"github.com/marceloalvescf/k8s-capacity-report/internal/goldentest"
	"github.com/marceloalvescf/k8s-capacity-report/internal/promtest"
	"github.com/marceloalvescf/k8s-capacity-report/internal/snapshot"
)

// realEnv is testEnv with Getenv and LookPath replaced by the real process
// environment and PATH, the way typst.Locate finds the compiler in
// production (by KCR_TYPST, or on PATH); every other field keeps the fixed
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
// every one of the six scenarios, and checks that the cover title and the
// nine capability names of that language, taken from the catalog by their
// keys, and the cluster name every scenario config carries ("prod"), all
// reach the text layer.
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

// TestRealReportHasThreePages checks that direct-complete, whose fixture
// backs every one of the nine capabilities and gives payments-web its own
// rows, renders as exactly three pages: the opening page; the page of
// sections 1 and 2 (capacity by namespace, capacity by workload); and the
// quality appendix, with its interpretation notes.
func TestRealReportHasThreePages(t *testing.T) {
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
	if got := strings.Count(text, "\f"); got != 3 {
		t.Fatalf("form feed count = %d, want 3 (the opening page, sections 1 and 2, and the quality appendix)", got)
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

	text := pdfText(t, outPath)
	if !containsText(text, "payments-web") {
		t.Errorf("text does not contain %q", "payments-web")
	}
	if !containsText(text, heading) {
		t.Errorf("text does not contain %q", heading)
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

// TestRealReportTextOfEveryScenario checks, for every one of the seven
// scenarios and both languages, that the compiled PDF: has three pages;
// names the numbered heading of section 1, section 2, and the appendix
// in the outline, on page 1; carries the heading of section 1 and of
// section 2, each with its number, on page 2, and the heading of the
// appendix, with its number, on page 3; carries the name of every
// namespace and every workload the scenario's snapshot holds, as a word
// of its own, on page 2; carries the interpretation notes heading, after
// the appendix heading, on page 3, exactly when the snapshot holds a
// warning or a notice, and on no page otherwise; and, for edge-values,
// carries neither of the two workload names its snapshot omits, as a
// word of its own, on any page.
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
					if len(pages) != 3 {
						t.Fatalf("PDF has %d page(s), want 3", len(pages))
					}

					cat, err := catalog.Load(lang)
					if err != nil {
						t.Fatalf("catalog.Load(%s): %v", lang, err)
					}

					headings := []struct {
						number int
						key    catalog.Key
						page   int // 0-based: the page the heading itself, not the outline entry, belongs on
					}{
						{1, catalog.SectionsNamespacesHeading, 1},
						{2, catalog.SectionsWorkloadsHeading, 1},
						{3, catalog.AppendixHeading, 2},
					}

					// Page 1's own outline names all three, each with its
					// number, whether or not the heading it names still
					// reaches the page it promises: a separate assertion
					// from the one below, so a lost section and a lost
					// outline entry are never reported as the same fault.
					for _, h := range headings {
						want := numberedHeading(t, cat, h.number, h.key)
						if !containsText(pages[0], want) {
							t.Errorf("the outline on page 1 does not contain %q", want)
						}
					}

					// The heading itself, on the page it belongs on.
					for _, h := range headings {
						want := numberedHeading(t, cat, h.number, h.key)
						if !containsText(pages[h.page], want) {
							t.Errorf("page %d does not contain the heading %q", h.page+1, want)
						}
					}

					for _, ns := range snap.Namespaces {
						if !wordAppears(pages[1], ns.Name) {
							t.Errorf("page 2 does not contain the namespace name %q as a word of its own", ns.Name)
						}
					}
					for _, wl := range snap.Workloads {
						if !wordAppears(pages[1], wl.Name) {
							t.Errorf("page 2 does not contain the workload name %q as a word of its own", wl.Name)
						}
					}

					notesHeading, err := cat.Text(catalog.NotesHeading, nil)
					if err != nil {
						t.Fatalf("NotesHeading text: %v", err)
					}
					if wantNotes {
						appendixWant := numberedHeading(t, cat, 3, catalog.AppendixHeading)
						strippedPage3 := stripWhitespace(pages[2])
						appendixIdx := strings.Index(strippedPage3, stripWhitespace(appendixWant))
						notesIdx := strings.Index(strippedPage3, stripWhitespace(notesHeading))
						switch {
						case notesIdx == -1:
							t.Errorf("page 3 does not contain the interpretation notes heading %q, though the snapshot holds a warning or a notice", notesHeading)
						case appendixIdx != -1 && notesIdx < appendixIdx:
							t.Errorf("the interpretation notes heading %q appears before the appendix heading on page 3", notesHeading)
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
