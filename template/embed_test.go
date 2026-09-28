package template

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"testing/fstest"
)

// walkEmbeddedFiles calls fn for every regular file the package embeds,
// skipping directory entries.
func walkEmbeddedFiles(t *testing.T, fn func(path string) error) {
	t.Helper()
	err := fs.WalkDir(embedded, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		return fn(path)
	})
	if err != nil {
		t.Fatalf("walking embedded files: %v", err)
	}
}

// assertUnpackedMatchesEmbedded checks that every embedded file exists
// under dir with the same bytes it holds in the embed.FS.
func assertUnpackedMatchesEmbedded(t *testing.T, dir string) {
	t.Helper()
	walkEmbeddedFiles(t, func(path string) error {
		want, err := embedded.ReadFile(path)
		if err != nil {
			return err
		}
		got, err := os.ReadFile(filepath.Join(dir, path))
		if err != nil {
			t.Fatalf("reading unpacked %s: %v", path, err)
		}
		if !bytes.Equal(got, want) {
			t.Fatalf("unpacked %s does not match the embedded bytes", path)
		}
		return nil
	})
}

func TestUnpackWritesEveryEmbeddedFile(t *testing.T) {
	dir := t.TempDir()
	if err := Unpack(dir); err != nil {
		t.Fatalf("Unpack() error = %v", err)
	}
	assertUnpackedMatchesEmbedded(t, dir)
}

// TestUnpackIntoExistingFolder proves the second Unpack actually overwrites
// and does not merely find its files already present. Calling Unpack twice
// with nothing changed in between would pass even if Unpack skipped files
// that already exist, so this test disturbs the folder first: it corrupts
// one unpacked file and adds one file the template does not have.
func TestUnpackIntoExistingFolder(t *testing.T) {
	dir := t.TempDir()
	if err := Unpack(dir); err != nil {
		t.Fatalf("first Unpack() error = %v", err)
	}

	var target string
	walkEmbeddedFiles(t, func(path string) error {
		if target == "" {
			target = path
		}
		return nil
	})
	if target == "" {
		t.Fatalf("no embedded file found to overwrite")
	}
	overwritten := filepath.Join(dir, target)
	if err := os.WriteFile(overwritten, []byte("not the real content"), 0o644); err != nil {
		t.Fatalf("overwriting %s: %v", overwritten, err)
	}

	extra := filepath.Join(dir, "extra-file-not-in-template.txt")
	if err := os.WriteFile(extra, []byte("leftover from a previous run"), 0o644); err != nil {
		t.Fatalf("writing %s: %v", extra, err)
	}

	if err := Unpack(dir); err != nil {
		t.Fatalf("second Unpack() error = %v", err)
	}

	// The corrupted file must be restored to the embedded bytes.
	assertUnpackedMatchesEmbedded(t, dir)

	// The extra file must still be there: Unpack writes what it knows
	// about, it does not delete what it does not.
	if _, err := os.Stat(extra); err != nil {
		t.Fatalf("extra file %s should still exist after Unpack: %v", extra, err)
	}
}

// embedRootsFromSource reads the //go:embed line out of the given Go
// source file and returns its space-separated patterns, in order. Reading
// the real directive as text (rather than hard-coding a second copy of the
// pattern list here) keeps the disk-side scan below from silently drifting
// out of sync with what embed.go actually embeds.
func embedRootsFromSource(path string) ([]string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	for _, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "//go:embed") {
			continue
		}
		roots := strings.Fields(strings.TrimPrefix(line, "//go:embed"))
		if len(roots) == 0 {
			return nil, fmt.Errorf("%s: //go:embed line has no pattern", path)
		}
		return roots, nil
	}
	return nil, fmt.Errorf("%s: no //go:embed line found", path)
}

// treeFiles walks every root in roots within fsys and returns a map from
// path to file bytes. A root may name a single file or a directory.
func treeFiles(fsys fs.FS, roots []string) (map[string][]byte, error) {
	files := make(map[string][]byte)
	for _, root := range roots {
		err := fs.WalkDir(fsys, root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				return nil
			}
			data, err := fs.ReadFile(fsys, path)
			if err != nil {
				return err
			}
			files[path] = data
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	return files, nil
}

// compareTrees reports every difference between disk and embedded over the
// given roots: a file present under only one of the two, or present under
// both with different bytes. //go:embed silently leaves out any file whose
// name starts with "." or "_", so comparing only the embedded side against
// disk (as TestEmbeddedFilesMatchDisk once did) can never catch a file disk
// has that the embed directive dropped; walking disk too closes that gap.
func compareTrees(disk, embedded fs.FS, roots []string) ([]string, error) {
	diskFiles, err := treeFiles(disk, roots)
	if err != nil {
		return nil, err
	}
	embeddedFiles, err := treeFiles(embedded, roots)
	if err != nil {
		return nil, err
	}

	var diffs []string
	for path, want := range diskFiles {
		got, ok := embeddedFiles[path]
		if !ok {
			diffs = append(diffs, fmt.Sprintf("%s: present on disk, missing from the embedded tree", path))
			continue
		}
		if !bytes.Equal(got, want) {
			diffs = append(diffs, fmt.Sprintf("%s: embedded bytes differ from disk", path))
		}
	}
	for path := range embeddedFiles {
		if _, ok := diskFiles[path]; !ok {
			diffs = append(diffs, fmt.Sprintf("%s: present in the embedded tree, missing from disk", path))
		}
	}
	sort.Strings(diffs)
	return diffs, nil
}

// TestEmbeddedFilesMatchDisk compares the embedded tree against the source
// tree both ways: every file embed.go's patterns embed must match disk, and
// every file those patterns find on disk must be embedded. `go test` runs
// with the package directory as its working directory, so a plain relative
// path already reaches the source tree; no module-root lookup is needed.
func TestEmbeddedFilesMatchDisk(t *testing.T) {
	roots, err := embedRootsFromSource("embed.go")
	if err != nil {
		t.Fatalf("reading embed patterns: %v", err)
	}
	diffs, err := compareTrees(os.DirFS("."), embedded, roots)
	if err != nil {
		t.Fatalf("comparing embedded tree with disk: %v", err)
	}
	for _, diff := range diffs {
		t.Error(diff)
	}
}

func TestFontLicenceIsEmbedded(t *testing.T) {
	if _, err := embedded.ReadFile("graphite/assets/fonts/LICENSE"); err != nil {
		t.Fatalf("reading embedded font licence: %v", err)
	}
}

func TestEmbedPatternsAreReadFromSource(t *testing.T) {
	roots, err := embedRootsFromSource("embed.go")
	if err != nil {
		t.Fatalf("embedRootsFromSource() error = %v", err)
	}
	for _, want := range []string{"graphite/lib.typ", "graphite/src", "graphite/assets", "capacity"} {
		found := false
		for _, root := range roots {
			if root == want {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("embed patterns = %v, want it to include %q", roots, want)
		}
	}
}

func TestCompareTreesEqual(t *testing.T) {
	disk := fstest.MapFS{
		"src/a.typ": &fstest.MapFile{Data: []byte("hello")},
	}
	embedded := fstest.MapFS{
		"src/a.typ": &fstest.MapFile{Data: []byte("hello")},
	}

	diffs, err := compareTrees(disk, embedded, []string{"src"})
	if err != nil {
		t.Fatalf("compareTrees() error = %v", err)
	}
	if len(diffs) != 0 {
		t.Fatalf("compareTrees() = %v, want no differences", diffs)
	}
}

func TestCompareTreesReportsFileMissingFromEmbedded(t *testing.T) {
	disk := fstest.MapFS{
		"src/a.typ":       &fstest.MapFile{Data: []byte("hello")},
		"src/_hidden.typ": &fstest.MapFile{Data: []byte("secret")},
	}
	embedded := fstest.MapFS{
		"src/a.typ": &fstest.MapFile{Data: []byte("hello")},
	}

	diffs, err := compareTrees(disk, embedded, []string{"src"})
	if err != nil {
		t.Fatalf("compareTrees() error = %v", err)
	}
	if len(diffs) != 1 || !strings.Contains(diffs[0], "src/_hidden.typ") || !strings.Contains(diffs[0], "embedded") {
		t.Fatalf("compareTrees() = %v, want one difference naming src/_hidden.typ and the embedded side", diffs)
	}
}

func TestCompareTreesReportsFileMissingFromDisk(t *testing.T) {
	disk := fstest.MapFS{
		"src/a.typ": &fstest.MapFile{Data: []byte("hello")},
	}
	embedded := fstest.MapFS{
		"src/a.typ":     &fstest.MapFile{Data: []byte("hello")},
		"src/extra.typ": &fstest.MapFile{Data: []byte("extra")},
	}

	diffs, err := compareTrees(disk, embedded, []string{"src"})
	if err != nil {
		t.Fatalf("compareTrees() error = %v", err)
	}
	if len(diffs) != 1 || !strings.Contains(diffs[0], "src/extra.typ") || !strings.Contains(diffs[0], "disk") {
		t.Fatalf("compareTrees() = %v, want one difference naming src/extra.typ and the disk side", diffs)
	}
}

func TestCompareTreesReportsDifferentBytes(t *testing.T) {
	disk := fstest.MapFS{
		"src/a.typ": &fstest.MapFile{Data: []byte("hello")},
	}
	embedded := fstest.MapFS{
		"src/a.typ": &fstest.MapFile{Data: []byte("goodbye")},
	}

	diffs, err := compareTrees(disk, embedded, []string{"src"})
	if err != nil {
		t.Fatalf("compareTrees() error = %v", err)
	}
	if len(diffs) != 1 || !strings.Contains(diffs[0], "src/a.typ") {
		t.Fatalf("compareTrees() = %v, want one difference naming src/a.typ", diffs)
	}
}
