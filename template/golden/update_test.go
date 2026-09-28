package golden

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/marceloalvescf/k8s-capacity-report/internal/typst"
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

// TestUpdateClearsExistingPNGsBeforeFailing exercises Update's ordering
// without a real typst binary: it points the toolchain at a path that
// cannot possibly be typst, which fails Render deterministically the same
// way whether or not typst is installed on this machine. It proves that
// existing PNGs are cleared before the (failing) render is attempted, and
// that a file that is not a PNG is left alone.
func TestUpdateClearsExistingPNGsBeforeFailing(t *testing.T) {
	goldenDir := t.TempDir()
	stalePNG := filepath.Join(goldenDir, "old-1.png")
	keep := filepath.Join(goldenDir, "keep.txt")
	if err := os.WriteFile(stalePNG, []byte("stale"), 0o644); err != nil {
		t.Fatalf("writing %s: %v", stalePNG, err)
	}
	if err := os.WriteFile(keep, []byte("keep"), 0o644); err != nil {
		t.Fatalf("writing %s: %v", keep, err)
	}

	templateDir := t.TempDir()
	fakeBinary := filepath.Join(t.TempDir(), "not-typst")
	tc := typst.Toolchain{Binary: fakeBinary}

	written, err := Update(context.Background(), tc, templateDir, goldenDir)
	if err == nil {
		t.Fatalf("Update() error = nil, want an error since %s does not exist", fakeBinary)
	}
	if written != nil {
		t.Fatalf("Update() names = %v, want nil on error", written)
	}

	if _, err := os.Stat(stalePNG); !os.IsNotExist(err) {
		t.Fatalf("stat %s: err = %v, want the stale PNG removed before the failing render", stalePNG, err)
	}
	if _, err := os.Stat(keep); err != nil {
		t.Fatalf("stat %s: %v, want the non-PNG file left alone", keep, err)
	}
	if _, err := os.Stat(filepath.Join(goldenDir, "SHA256SUMS")); !os.IsNotExist(err) {
		t.Fatalf("SHA256SUMS was written despite the failing render")
	}
}
