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
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"unicode"

	"github.com/marceloalvescf/k8s-capacity-report/internal/catalog"
	"github.com/marceloalvescf/k8s-capacity-report/internal/discovery"
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

// TestRealReportHasTwoPages checks that direct-complete, whose fixture backs
// every one of the nine capabilities, renders as exactly two pages: two form
// feeds in the text layer, one at the end of each page, so the two page
// breaks in a row leave no empty page between the cover and the appendix.
func TestRealReportHasTwoPages(t *testing.T) {
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
	if got := strings.Count(text, "\f"); got != 2 {
		t.Fatalf("form feed count = %d, want 2", got)
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
