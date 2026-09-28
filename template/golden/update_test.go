package golden

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

func TestRemovePNGsDeletesOnlyPNGFiles(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"a-1.png", "b-1.png", "keep.txt"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0o644); err != nil {
			t.Fatalf("writing %s: %v", name, err)
		}
	}

	if err := removePNGs(dir); err != nil {
		t.Fatalf("removePNGs() error = %v", err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("reading %s: %v", dir, err)
	}
	if len(entries) != 1 || entries[0].Name() != "keep.txt" {
		t.Fatalf("directory after removePNGs() = %v, want only keep.txt", entries)
	}
}

func TestRemovePNGsToleratesMissingDir(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "does-not-exist")
	if err := removePNGs(dir); err != nil {
		t.Fatalf("removePNGs() error = %v, want nil for a missing folder", err)
	}
}

// TestRemovePNGsLeavesDirectoriesAlone proves the "never deletes a folder"
// rule even in the contrived case of a directory whose own name ends in
// ".png" and so matches the glob pattern.
func TestRemovePNGsLeavesDirectoriesAlone(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "looks-like-a-page.png")
	if err := os.MkdirAll(filepath.Join(sub, "inside"), 0o755); err != nil {
		t.Fatalf("creating %s: %v", sub, err)
	}

	if err := removePNGs(dir); err != nil {
		t.Fatalf("removePNGs() error = %v", err)
	}

	if _, err := os.Stat(filepath.Join(sub, "inside")); err != nil {
		t.Fatalf("stat %s: %v, want the directory (and its content) left alone", sub, err)
	}
}

// fakeRender returns a renderFunc that writes deterministic content for
// each of s.Files() into dir, unless name (a snapshot name) is in fail,
// in which case it returns an error instead, and unless name is in
// dropPage, in which case it returns success without writing any file at
// all, simulating a compiler that silently produced nothing.
func fakeRender(content map[string][]byte, fail map[string]bool, drop map[string]bool) renderFunc {
	return func(_ context.Context, _ string, s Snapshot, dir string) error {
		if fail[s.Name] {
			return errors.New("fake render failure for " + s.Name)
		}
		if drop[s.Name] {
			return nil
		}
		for _, name := range s.Files() {
			data := content[name]
			if data == nil {
				data = []byte(name) // deterministic default content
			}
			if err := os.WriteFile(filepath.Join(dir, name), data, 0o644); err != nil {
				return err
			}
		}
		return nil
	}
}

func digestOf(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func readFile(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	return data
}

func TestUpdateKeepsGoldenPagesWhenARenderFails(t *testing.T) {
	goldenDir := t.TempDir()
	files := map[string][]byte{
		"a-1.png":    []byte("stale-a"),
		"b-1.png":    []byte("stale-b"),
		"SHA256SUMS": []byte(digestOf([]byte("stale-a")) + "  a-1.png\n" + digestOf([]byte("stale-b")) + "  b-1.png\n"),
	}
	for name, data := range files {
		if err := os.WriteFile(filepath.Join(goldenDir, name), data, 0o644); err != nil {
			t.Fatalf("writing %s: %v", name, err)
		}
	}

	snapshots := []Snapshot{
		{Name: "a", Pages: []int{1}},
		{Name: "b", Pages: []int{1}}, // fails
	}
	render := fakeRender(nil, map[string]bool{"b": true}, nil)

	written, err := update(context.Background(), render, snapshots, t.TempDir(), goldenDir)
	if err == nil {
		t.Fatal("update() error = nil, want an error from the failing render")
	}
	if written != nil {
		t.Fatalf("update() names = %v, want nil on error", written)
	}

	for name, want := range files {
		got := readFile(t, filepath.Join(goldenDir, name))
		if string(got) != string(want) {
			t.Errorf("%s changed after a failed update: got %q, want %q", name, got, want)
		}
	}
}

func TestUpdateReplacesGoldenPagesOnSuccess(t *testing.T) {
	goldenDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(goldenDir, "old-1.png"), []byte("old"), 0o644); err != nil {
		t.Fatalf("writing old-1.png: %v", err)
	}

	snapshots := []Snapshot{
		{Name: "x", Pages: []int{1}},
		{Name: "y", Pages: []int{1}},
	}
	content := map[string][]byte{
		"x-1.png": []byte("content-x"),
		"y-1.png": []byte("content-y"),
	}
	render := fakeRender(content, nil, nil)

	written, err := update(context.Background(), render, snapshots, t.TempDir(), goldenDir)
	if err != nil {
		t.Fatalf("update() error = %v", err)
	}

	wantWritten := []string{"x-1.png", "y-1.png", "SHA256SUMS"}
	if len(written) != len(wantWritten) {
		t.Fatalf("update() names = %v, want %v", written, wantWritten)
	}
	if written[len(written)-1] != "SHA256SUMS" {
		t.Fatalf("update() names = %v, want SHA256SUMS last", written)
	}
	sortedGot := append([]string(nil), written[:len(written)-1]...)
	sort.Strings(sortedGot)
	if sortedGot[0] != "x-1.png" || sortedGot[1] != "y-1.png" {
		t.Fatalf("update() names (excluding SHA256SUMS) = %v, want x-1.png and y-1.png", sortedGot)
	}

	if _, err := os.Stat(filepath.Join(goldenDir, "old-1.png")); !os.IsNotExist(err) {
		t.Fatalf("old-1.png still present after update(): err = %v", err)
	}

	for name, want := range content {
		got := readFile(t, filepath.Join(goldenDir, name))
		if string(got) != string(want) {
			t.Errorf("%s = %q, want %q", name, got, want)
		}
	}

	sums, err := ReadSums(filepath.Join(goldenDir, "SHA256SUMS"))
	if err != nil {
		t.Fatalf("ReadSums() error = %v", err)
	}
	if len(sums) != len(content) {
		t.Fatalf("SHA256SUMS has %d entries, want %d", len(sums), len(content))
	}
	for name, data := range content {
		want := digestOf(data)
		if sums[name] != want {
			t.Errorf("SHA256SUMS[%q] = %q, want %q", name, sums[name], want)
		}
	}
}

func TestUpdateFailsWhenARenderedPageIsMissing(t *testing.T) {
	goldenDir := t.TempDir()
	kept := []byte("kept")
	if err := os.WriteFile(filepath.Join(goldenDir, "keep-1.png"), kept, 0o644); err != nil {
		t.Fatalf("writing keep-1.png: %v", err)
	}

	snapshots := []Snapshot{
		{Name: "keep", Pages: []int{1}},
		{Name: "ghost", Pages: []int{1}}, // "renders" successfully but writes nothing
	}
	render := fakeRender(nil, nil, map[string]bool{"ghost": true})

	written, err := update(context.Background(), render, snapshots, t.TempDir(), goldenDir)
	if err == nil {
		t.Fatal("update() error = nil, want an error naming the missing file")
	}
	if !strings.Contains(err.Error(), "ghost-1.png") {
		t.Fatalf("update() error = %q, want it to name ghost-1.png", err)
	}
	if written != nil {
		t.Fatalf("update() names = %v, want nil on error", written)
	}

	got := readFile(t, filepath.Join(goldenDir, "keep-1.png"))
	if string(got) != string(kept) {
		t.Fatalf("keep-1.png changed: got %q, want %q", got, kept)
	}
	if _, err := os.Stat(filepath.Join(goldenDir, "SHA256SUMS")); !os.IsNotExist(err) {
		t.Fatalf("SHA256SUMS was written despite the missing rendered page")
	}
}

func TestUpdateLeavesOtherFilesAlone(t *testing.T) {
	goldenDir := t.TempDir()
	notes := []byte("do not touch")
	if err := os.WriteFile(filepath.Join(goldenDir, "notes.txt"), notes, 0o644); err != nil {
		t.Fatalf("writing notes.txt: %v", err)
	}
	nested := filepath.Join(goldenDir, "keep")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatalf("creating %s: %v", nested, err)
	}
	nestedPNG := []byte("nested")
	if err := os.WriteFile(filepath.Join(nested, "x.png"), nestedPNG, 0o644); err != nil {
		t.Fatalf("writing keep/x.png: %v", err)
	}

	snapshots := []Snapshot{{Name: "a", Pages: []int{1}}}
	render := fakeRender(nil, nil, nil)

	if _, err := update(context.Background(), render, snapshots, t.TempDir(), goldenDir); err != nil {
		t.Fatalf("update() error = %v", err)
	}

	got := readFile(t, filepath.Join(goldenDir, "notes.txt"))
	if string(got) != string(notes) {
		t.Fatalf("notes.txt changed: got %q, want %q", got, notes)
	}
	got = readFile(t, filepath.Join(nested, "x.png"))
	if string(got) != string(nestedPNG) {
		t.Fatalf("keep/x.png changed: got %q, want %q", got, nestedPNG)
	}
}

// TestUpdateRemovesItsScratchFolder points TMPDIR at a folder this test
// owns, so it can count what update() leaves behind in the directory
// os.MkdirTemp allocates from, both when the render succeeds and when it
// fails. Either way, the scratch folder update() creates must be gone by
// the time it returns.
func TestUpdateRemovesItsScratchFolder(t *testing.T) {
	tmpParent := filepath.Join(t.TempDir(), "tmp-parent")
	if err := os.MkdirAll(tmpParent, 0o755); err != nil {
		t.Fatalf("creating %s: %v", tmpParent, err)
	}
	t.Setenv("TMPDIR", tmpParent)

	entryCount := func() int {
		entries, err := os.ReadDir(tmpParent)
		if err != nil {
			t.Fatalf("reading %s: %v", tmpParent, err)
		}
		return len(entries)
	}

	snapshots := []Snapshot{{Name: "a", Pages: []int{1}}}

	// Successful run.
	if before := entryCount(); before != 0 {
		t.Fatalf("tmp parent has %d entries before the successful run, want 0", before)
	}
	if _, err := update(context.Background(), fakeRender(nil, nil, nil), snapshots, t.TempDir(), t.TempDir()); err != nil {
		t.Fatalf("update() error = %v", err)
	}
	if after := entryCount(); after != 0 {
		t.Fatalf("tmp parent has %d entries after the successful run, want 0 (scratch folder left behind)", after)
	}

	// Failing run.
	failing := fakeRender(nil, map[string]bool{"a": true}, nil)
	if _, err := update(context.Background(), failing, snapshots, t.TempDir(), t.TempDir()); err == nil {
		t.Fatal("update() error = nil, want an error from the failing render")
	}
	if after := entryCount(); after != 0 {
		t.Fatalf("tmp parent has %d entries after the failing run, want 0 (scratch folder left behind)", after)
	}
}
