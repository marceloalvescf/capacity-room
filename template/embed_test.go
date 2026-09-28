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

// TestUnpackRefusesFolderThatIsNotEmpty proves Unpack never reuses a folder
// that already holds something: a stale file from a previous run, or a
// planted symbolic link, must never be written through. It leaves the
// folder exactly as it found it.
func TestUnpackRefusesFolderThatIsNotEmpty(t *testing.T) {
	dir := t.TempDir()
	existing := filepath.Join(dir, "already-here.txt")
	if err := os.WriteFile(existing, []byte("leftover"), 0o644); err != nil {
		t.Fatalf("writing %s: %v", existing, err)
	}

	err := Unpack(dir)
	if err == nil || !strings.Contains(err.Error(), "is not empty") {
		t.Fatalf("Unpack() error = %v, want it to contain %q", err, "is not empty")
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("reading %s: %v", dir, err)
	}
	if len(entries) != 1 || entries[0].Name() != "already-here.txt" {
		t.Fatalf("dir entries = %v, want exactly the one pre-existing file", entries)
	}
	got, err := os.ReadFile(existing)
	if err != nil {
		t.Fatalf("reading %s: %v", existing, err)
	}
	if string(got) != "leftover" {
		t.Fatalf("%s = %q, want it unchanged", existing, got)
	}
}

// TestUnpackNeverWritesThroughSymbolicLink proves Unpack refuses a folder
// that holds a symbolic link rather than following it: a link planted at a
// path the embedded tree occupies (here, "graphite") must never cause a
// write outside the target folder.
func TestUnpackNeverWritesThroughSymbolicLink(t *testing.T) {
	dir := t.TempDir()
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(dir, "graphite")); err != nil {
		t.Fatalf("creating symbolic link: %v", err)
	}

	if err := Unpack(dir); err == nil {
		t.Fatalf("Unpack() error = nil, want an error")
	}

	entries, err := os.ReadDir(outside)
	if err != nil {
		t.Fatalf("reading %s: %v", outside, err)
	}
	if len(entries) != 0 {
		t.Fatalf("outside folder entries = %v, want it to stay empty", entries)
	}
}

// TestUnpackRefusesFolderThatIsASymbolicLink proves Unpack checks dir
// itself with os.Lstat rather than os.Stat: a symbolic link planted at dir
// must never be followed, whatever it points at, because os.Stat would
// resolve it and let the embedded template be written through the link
// into an entirely different location.
func TestUnpackRefusesFolderThatIsASymbolicLink(t *testing.T) {
	const wantErr = "is a symbolic link"

	t.Run("link to an empty folder", func(t *testing.T) {
		parent := t.TempDir()
		target := filepath.Join(parent, "empty-target")
		if err := os.Mkdir(target, 0o755); err != nil {
			t.Fatalf("creating %s: %v", target, err)
		}
		link := filepath.Join(parent, "link")
		if err := os.Symlink(target, link); err != nil {
			t.Fatalf("creating symbolic link: %v", err)
		}

		err := Unpack(link)
		if err == nil || !strings.Contains(err.Error(), wantErr) {
			t.Fatalf("Unpack() error = %v, want it to contain %q", err, wantErr)
		}

		entries, err := os.ReadDir(target)
		if err != nil {
			t.Fatalf("reading %s: %v", target, err)
		}
		if len(entries) != 0 {
			t.Fatalf("target folder entries = %v, want it to stay empty", entries)
		}
	})

	t.Run("link to a non-empty folder", func(t *testing.T) {
		parent := t.TempDir()
		target := filepath.Join(parent, "full-target")
		if err := os.Mkdir(target, 0o755); err != nil {
			t.Fatalf("creating %s: %v", target, err)
		}
		existing := filepath.Join(target, "already-here.txt")
		if err := os.WriteFile(existing, []byte("leftover"), 0o644); err != nil {
			t.Fatalf("writing %s: %v", existing, err)
		}
		link := filepath.Join(parent, "link")
		if err := os.Symlink(target, link); err != nil {
			t.Fatalf("creating symbolic link: %v", err)
		}

		err := Unpack(link)
		if err == nil || !strings.Contains(err.Error(), wantErr) {
			t.Fatalf("Unpack() error = %v, want it to contain %q", err, wantErr)
		}

		got, err := os.ReadFile(existing)
		if err != nil {
			t.Fatalf("reading %s: %v", existing, err)
		}
		if string(got) != "leftover" {
			t.Fatalf("%s = %q, want it unchanged", existing, got)
		}
	})

	t.Run("link to a file", func(t *testing.T) {
		parent := t.TempDir()
		target := filepath.Join(parent, "a-file")
		if err := os.WriteFile(target, []byte("a plain file"), 0o644); err != nil {
			t.Fatalf("writing %s: %v", target, err)
		}
		link := filepath.Join(parent, "link")
		if err := os.Symlink(target, link); err != nil {
			t.Fatalf("creating symbolic link: %v", err)
		}

		err := Unpack(link)
		if err == nil || !strings.Contains(err.Error(), wantErr) {
			t.Fatalf("Unpack() error = %v, want it to contain %q", err, wantErr)
		}

		got, err := os.ReadFile(target)
		if err != nil {
			t.Fatalf("reading %s: %v", target, err)
		}
		if string(got) != "a plain file" {
			t.Fatalf("%s = %q, want it unchanged", target, got)
		}
	})

	t.Run("dangling link", func(t *testing.T) {
		parent := t.TempDir()
		missing := filepath.Join(parent, "does-not-exist")
		link := filepath.Join(parent, "link")
		if err := os.Symlink(missing, link); err != nil {
			t.Fatalf("creating symbolic link: %v", err)
		}

		err := Unpack(link)
		if err == nil || !strings.Contains(err.Error(), wantErr) {
			t.Fatalf("Unpack() error = %v, want it to contain %q", err, wantErr)
		}

		if _, err := os.Lstat(missing); !os.IsNotExist(err) {
			t.Fatalf("Lstat(%s) error = %v, want it to still report missing", missing, err)
		}
	})
}

// TestUnpackAcceptsParentThatIsASymbolicLink proves Unpack checks only the
// final path component of dir: a parent folder reached through a symbolic
// link, such as /tmp on macOS, is accepted rather than refused.
func TestUnpackAcceptsParentThatIsASymbolicLink(t *testing.T) {
	parent := t.TempDir()
	real := filepath.Join(parent, "real")
	if err := os.Mkdir(real, 0o755); err != nil {
		t.Fatalf("creating %s: %v", real, err)
	}
	alias := filepath.Join(parent, "alias")
	if err := os.Symlink(real, alias); err != nil {
		t.Fatalf("creating symbolic link: %v", err)
	}

	dir := filepath.Join(alias, "work")
	if err := Unpack(dir); err != nil {
		t.Fatalf("Unpack() error = %v", err)
	}
	assertUnpackedMatchesEmbedded(t, dir)
}

// TestUnpackRefusesPathThatIsAFile proves Unpack refuses a dir argument
// that names an existing regular file instead of a folder, and leaves that
// file untouched.
func TestUnpackRefusesPathThatIsAFile(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "not-a-folder")
	if err := os.WriteFile(target, []byte("a plain file"), 0o644); err != nil {
		t.Fatalf("writing %s: %v", target, err)
	}

	err := Unpack(target)
	if err == nil || !strings.Contains(err.Error(), "is not a folder") {
		t.Fatalf("Unpack() error = %v, want it to contain %q", err, "is not a folder")
	}

	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("reading %s: %v", target, err)
	}
	if string(got) != "a plain file" {
		t.Fatalf("%s = %q, want it unchanged", target, got)
	}
}

// TestUnpackIntoEmptyExistingFolder proves an empty pre-existing folder is
// accepted: Unpack does not require dir to be absent.
func TestUnpackIntoEmptyExistingFolder(t *testing.T) {
	dir := t.TempDir()
	if err := Unpack(dir); err != nil {
		t.Fatalf("Unpack() error = %v", err)
	}
	assertUnpackedMatchesEmbedded(t, dir)
}

// TestUnpackCreatesMissingFolder proves Unpack creates dir, with any
// missing parents, when nothing exists at that path yet.
func TestUnpackCreatesMissingFolder(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "a", "b", "c")
	if err := Unpack(dir); err != nil {
		t.Fatalf("Unpack() error = %v", err)
	}
	assertUnpackedMatchesEmbedded(t, dir)
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
