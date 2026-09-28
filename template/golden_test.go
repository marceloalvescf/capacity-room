//go:build template && golden

package template

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/marceloalvescf/k8s-capacity-report/template/golden"
)

// goldenRender holds the result of rendering every snapshot exactly once
// for this test binary. The bytes of every rendered page are copied into
// memory while the rendering test's own t.TempDir() is still alive, so
// later tests (run from a different *testing.T, whose own t.TempDir() the
// first test knows nothing about) can still use them after that directory
// has been cleaned up.
type goldenRender struct {
	pages map[string][]byte // PNG name, e.g. "complete-1.png", to its bytes
	err   error
}

var (
	goldenOnce   sync.Once
	goldenResult goldenRender
)

// renderGolden renders every entry of golden.Snapshots exactly once for
// the whole test binary, into a t.TempDir() owned by whichever test calls
// it first, and returns the shared result every test reads.
func renderGolden(t *testing.T) goldenRender {
	t.Helper()
	goldenOnce.Do(func() {
		tc := toolchain(t)
		root := workspace(t)
		dir := t.TempDir()

		pages := make(map[string][]byte)
		for _, s := range golden.Snapshots {
			if err := golden.Render(context.Background(), tc, root, s, dir); err != nil {
				goldenResult.err = fmt.Errorf("rendering %s: %w", s.Name, err)
				return
			}
			for _, name := range s.Files() {
				data, err := os.ReadFile(filepath.Join(dir, name))
				if err != nil {
					goldenResult.err = fmt.Errorf("reading rendered %s: %w", name, err)
					return
				}
				pages[name] = data
			}
		}
		goldenResult.pages = pages
	})
	if goldenResult.err != nil {
		t.Fatalf("rendering golden pages: %v", goldenResult.err)
	}
	return goldenResult
}

// writeRendered copies a page's bytes into the calling test's own
// t.TempDir(), so functions that take a path (golden.Digest, golden.
// HasColour, golden.WriteDifference) can be called on it.
func writeRendered(t *testing.T, name string, data []byte) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("writing %s: %v", path, err)
	}
	return path
}

// goldenDir is template/tests/golden/ in the source tree, where the
// reviewed golden pages and their SHA256SUMS manifest are committed.
func goldenDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(moduleRoot(t), "template", "tests", "golden")
}

// sortedNames returns the keys of pages, sorted, so iteration order is
// deterministic across runs.
func sortedNames(pages map[string][]byte) []string {
	names := make([]string, 0, len(pages))
	for name := range pages {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// TestGoldenManifestMatchesFiles checks template/tests/golden/'s own
// internal consistency: the names SHA256SUMS lists are exactly the PNG
// files present, and each recorded digest matches the file on disk. It
// does not depend on a fresh render, so it fails on its own, with a clear
// message, before any golden page has ever been committed.
func TestGoldenManifestMatchesFiles(t *testing.T) {
	renderGolden(t) // keep all four tests reading the one shared render pass

	dir := goldenDir(t)
	sums, err := golden.ReadSums(filepath.Join(dir, "SHA256SUMS"))
	if err != nil {
		t.Fatalf("reading golden manifest: %v; run `make update-golden` to create it", err)
	}
	if len(sums) == 0 {
		t.Fatalf("golden manifest %s/SHA256SUMS lists no page; run `make update-golden` to create it", dir)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("reading golden directory %s: %v; run `make update-golden` to create it", dir, err)
	}
	var files []string
	for _, e := range entries {
		if !e.IsDir() && filepath.Ext(e.Name()) == ".png" {
			files = append(files, e.Name())
		}
	}
	sort.Strings(files)

	names := make([]string, 0, len(sums))
	for name := range sums {
		names = append(names, name)
	}
	sort.Strings(names)

	if len(names) != len(files) {
		t.Fatalf("SHA256SUMS names %v do not match PNG files present %v", names, files)
	}
	for i := range names {
		if names[i] != files[i] {
			t.Fatalf("SHA256SUMS names %v do not match PNG files present %v", names, files)
		}
	}

	for _, name := range names {
		got, err := golden.Digest(filepath.Join(dir, name))
		if err != nil {
			t.Errorf("digesting %s: %v", name, err)
			continue
		}
		if got != sums[name] {
			t.Errorf("%s: digest on disk is %s, SHA256SUMS says %s", name, got, sums[name])
		}
	}
}

// TestGoldenCountMatchesSnapshots checks that the number of golden PNG
// files equals the number of pages golden.Snapshots declares, so a
// snapshot added to the list without ever running update-golden is caught
// even before any digest is compared.
func TestGoldenCountMatchesSnapshots(t *testing.T) {
	renderGolden(t)

	dir := goldenDir(t)
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("reading golden directory %s: %v; run `make update-golden` to create it", dir, err)
	}
	count := 0
	for _, e := range entries {
		if !e.IsDir() && filepath.Ext(e.Name()) == ".png" {
			count++
		}
	}

	want := 0
	for _, s := range golden.Snapshots {
		want += len(s.Pages)
	}

	if count != want {
		t.Fatalf("golden directory %s has %d PNG files, want %d (the sum of golden.Snapshots pages)", dir, count, want)
	}
}

// TestGoldenPagesMatch renders every snapshot and compares each page's
// digest against the one recorded for it in SHA256SUMS. On a mismatch, if
// KCR_GOLDEN_OUT is set, it writes the actual page and a difference image
// under it and names both paths in the failure; otherwise the failure says
// to set the variable.
func TestGoldenPagesMatch(t *testing.T) {
	render := renderGolden(t)

	dir := goldenDir(t)
	sums, err := golden.ReadSums(filepath.Join(dir, "SHA256SUMS"))
	if err != nil {
		t.Fatalf("reading golden manifest: %v; run `make update-golden` to create it", err)
	}

	out := os.Getenv("KCR_GOLDEN_OUT")

	for _, name := range sortedNames(render.pages) {
		actualPath := writeRendered(t, name, render.pages[name])
		actualDigest, err := golden.Digest(actualPath)
		if err != nil {
			t.Errorf("digesting rendered %s: %v", name, err)
			continue
		}

		want, ok := sums[name]
		if !ok {
			t.Errorf("%s: rendered, but not recorded in %s/SHA256SUMS", name, dir)
			continue
		}
		if actualDigest == want {
			continue
		}

		if out == "" {
			t.Errorf("%s: rendered digest %s does not match golden digest %s; "+
				"set KCR_GOLDEN_OUT to a folder to save the actual page and a difference image",
				name, actualDigest, want)
			continue
		}

		actualOut := filepath.Join(out, "actual", name)
		diffOut := filepath.Join(out, "diff", name)
		if err := os.MkdirAll(filepath.Dir(actualOut), 0o755); err != nil {
			t.Fatalf("creating %s: %v", filepath.Dir(actualOut), err)
		}
		if err := os.MkdirAll(filepath.Dir(diffOut), 0o755); err != nil {
			t.Fatalf("creating %s: %v", filepath.Dir(diffOut), err)
		}
		if err := os.WriteFile(actualOut, render.pages[name], 0o644); err != nil {
			t.Fatalf("writing %s: %v", actualOut, err)
		}
		if err := golden.WriteDifference(filepath.Join(dir, name), actualPath, diffOut); err != nil {
			t.Errorf("%s: rendered digest %s does not match golden digest %s; "+
				"writing difference image: %v", name, actualDigest, want, err)
			continue
		}
		t.Errorf("%s: rendered digest %s does not match golden digest %s; "+
			"actual page written to %s, difference image written to %s",
			name, actualDigest, want, actualOut, diffOut)
	}
}

// pdfPageCount returns the page count of the PDF at path, read with
// pdfinfo (installed alongside pdftotext, per the harness's own
// requirements). It fails the test, never skips it, when pdfinfo is
// missing, exits non-zero, or its output has no "Pages:" line.
func pdfPageCount(t *testing.T, path string) int {
	t.Helper()
	out, err := exec.Command("pdfinfo", path).Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			t.Fatalf("pdfinfo %s: %v, stderr: %s", path, exitErr, exitErr.Stderr)
		}
		t.Fatalf("pdfinfo %s: %v", path, err)
	}
	for _, line := range strings.Split(string(out), "\n") {
		if !strings.HasPrefix(line, "Pages:") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) != 2 {
			t.Fatalf("pdfinfo %s: unexpected \"Pages:\" line %q", path, line)
		}
		n, err := strconv.Atoi(fields[1])
		if err != nil {
			t.Fatalf("pdfinfo %s: parsing page count %q: %v", path, fields[1], err)
		}
		return n
	}
	t.Fatalf("pdfinfo %s: output has no \"Pages:\" line", path)
	return 0
}

// snapshotInputs builds the same Inputs map job() (template/golden/
// render.go) would, so a direct compile here sees the document exactly as
// Render does.
func snapshotInputs(s golden.Snapshot) map[string]string {
	inputs := make(map[string]string, len(s.Inputs)+1)
	for k, v := range s.Inputs {
		inputs[k] = v
	}
	if s.Data != "" {
		inputs["data"] = "/" + s.Data
	}
	return inputs
}

// TestSnapshotsCoverEveryPage guards against a snapshot whose Pages
// undershoots its document's real page count without saying so: it
// compiles every snapshot's document in full (not limited to Pages) and
// reads the true page count with pdfinfo, independent of golden.Render
// entirely. When Omits is empty, Pages must list every page, 1 to n. When
// Omits is set, Pages must still be a strict, ascending subset of 1..n,
// naming no page above n — Omits itself is only ever read by a human, but
// its presence is what allows Pages to be incomplete at all.
func TestSnapshotsCoverEveryPage(t *testing.T) {
	ws := workspace(t)
	for _, s := range golden.Snapshots {
		t.Run(s.Name, func(t *testing.T) {
			pdf, stderr, err := compilePDF(t, ws, s.Source, snapshotInputs(s))
			assertCleanCompile(t, stderr, err)
			n := pdfPageCount(t, pdf)

			if s.Omits == "" {
				if len(s.Pages) != n {
					t.Fatalf("%s: Pages = %v lists %d page(s), but the document has %d page(s) and Omits is empty; "+
						"want Pages to list every page", s.Name, s.Pages, len(s.Pages), n)
				}
				for i, p := range s.Pages {
					if p != i+1 {
						t.Fatalf("%s: Pages = %v is not exactly 1..%d, and Omits is empty", s.Name, s.Pages, n)
					}
				}
				return
			}

			if len(s.Pages) >= n {
				t.Fatalf("%s: Pages = %v lists %d of the document's %d page(s), which is not a strict subset, "+
					"even though Omits is set to %q", s.Name, s.Pages, len(s.Pages), n, s.Omits)
			}
			for i, p := range s.Pages {
				if p < 1 || p > n {
					t.Fatalf("%s: Pages = %v names page %d, outside 1..%d (the document's page count)",
						s.Name, s.Pages, p, n)
				}
				if i > 0 && s.Pages[i-1] >= p {
					t.Fatalf("%s: Pages = %v is not strictly ascending", s.Name, s.Pages)
				}
			}
		})
	}
}

// TestGoldenPagesHaveNoColour checks every rendered page directly: it
// needs no golden page to compare against, so it is the one test of this
// file that can pass before template/tests/golden/ has ever been
// populated.
func TestGoldenPagesHaveNoColour(t *testing.T) {
	render := renderGolden(t)

	for _, name := range sortedNames(render.pages) {
		path := writeRendered(t, name, render.pages[name])
		hasColour, err := golden.HasColour(path)
		if err != nil {
			t.Errorf("checking colour of %s: %v", name, err)
			continue
		}
		if hasColour {
			t.Errorf("%s: contains a coloured pixel; no page may", name)
		}
	}
}
