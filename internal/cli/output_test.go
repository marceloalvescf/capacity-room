package cli

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSlug(t *testing.T) {
	tests := []struct{ in, want string }{
		{"prod", "prod"},
		{"Prod West_1", "prod-west-1"},
		{"---", "cluster"},
		{"", "cluster"},
		{"a__b--c", "a-b-c"},
		{"-leading and trailing-", "leading-and-trailing"},
	}
	for _, tt := range tests {
		if got := slug(tt.in); got != tt.want {
			t.Errorf("slug(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

// TestDefaultOutputName covers the same five rows as TestPDFModeDefaultName,
// directly against the naming algorithm, with no network dependency: the
// emission time is fixed at 2026-07-15 14:30:00, the same instant
// internal/goldentest.fixedConfig uses.
func TestDefaultOutputName(t *testing.T) {
	emittedAt := time.Date(2026, time.July, 15, 14, 30, 0, 0, time.UTC)

	tests := []struct {
		name                           string
		cluster, minSeverity, language string
		want                           string
	}{
		{"cluster prod", "prod", "all", "en", "report-resources-prod-en-20260715-143000.pdf"},
		{"cluster with punctuation", "Prod West_1", "all", "en", "report-resources-prod-west-1-en-20260715-143000.pdf"},
		{"cluster all punctuation", "---", "all", "en", "report-resources-cluster-en-20260715-143000.pdf"},
		{"minimum severity", "prod", "attention", "en", "report-resources-prod-attention-en-20260715-143000.pdf"},
		{"language pt-BR", "prod", "all", "pt-BR", "report-resources-prod-pt-br-20260715-143000.pdf"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := defaultOutputName(tt.cluster, tt.minSeverity, tt.language, emittedAt)
			if got != tt.want {
				t.Errorf("defaultOutputName(%q, %q, %q) = %q, want %q", tt.cluster, tt.minSeverity, tt.language, got, tt.want)
			}
		})
	}
}

func TestCheckOutputPathSuffix(t *testing.T) {
	if err := checkOutputPath(filepath.Join(t.TempDir(), "r.txt")); err == nil {
		t.Fatal("checkOutputPath(*.txt) = nil, want an error")
	} else if want := "output path must end in .pdf"; err.Error() != want {
		t.Fatalf("checkOutputPath error = %q, want %q", err.Error(), want)
	}
}

func TestCheckOutputPathMissingDirectory(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing", "r.pdf")
	err := checkOutputPath(path)
	if err == nil {
		t.Fatal("checkOutputPath(missing dir) = nil, want an error")
	}
	dir := filepath.Dir(path)
	if want := "output directory must exist and be writable: " + dir; err.Error() != want {
		t.Fatalf("checkOutputPath error = %q, want %q", err.Error(), want)
	}
}

func TestCheckOutputPathIsAFolder(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "r.pdf")
	if err := os.Mkdir(path, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", path, err)
	}
	err := checkOutputPath(path)
	if err == nil {
		t.Fatal("checkOutputPath(folder) = nil, want an error")
	}
	if want := "output path is a folder: " + path; err.Error() != want {
		t.Fatalf("checkOutputPath error = %q, want %q", err.Error(), want)
	}
}

func TestCheckSnapshotOutputPathSuffix(t *testing.T) {
	err := checkSnapshotOutputPath(filepath.Join(t.TempDir(), "s.txt"))
	if err == nil {
		t.Fatal("checkSnapshotOutputPath(*.txt) = nil, want an error")
	}
	if want := "snapshot path must end in .json"; err.Error() != want {
		t.Fatalf("checkSnapshotOutputPath error = %q, want %q", err.Error(), want)
	}
}

func TestWriteAtomicFileReplacesAnExistingFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "out.json")
	if err := os.WriteFile(path, []byte("old"), 0o644); err != nil {
		t.Fatalf("writing old file: %v", err)
	}

	if err := writeAtomicFile(path, []byte("new")); err != nil {
		t.Fatalf("writeAtomicFile: %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	if string(got) != "new" {
		t.Fatalf("content = %q, want %q", got, "new")
	}
	assertNoTempFilesBeside(t, dir)
}

// TestPDFModeFollowsWhatTheCompilerLeft tests settlePDFMode directly: it
// creates a file, sets its mode by hand with os.Chmod (never subject to the
// umask, so the test controls the exact starting mode), and checks what
// settlePDFMode leaves it at.
func TestPDFModeFollowsWhatTheCompilerLeft(t *testing.T) {
	tests := []struct {
		name        string
		start, want os.FileMode
	}{
		{"0o666", 0o666, 0o644},
		{"0o644", 0o644, 0o644},
		{"0o600", 0o600, 0o600},
		{"0o640", 0o640, 0o640},
		{"0o755", 0o755, 0o644},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "r.pdf")
			if err := os.WriteFile(path, []byte("data"), 0o644); err != nil {
				t.Fatalf("writing %s: %v", path, err)
			}
			if err := os.Chmod(path, tt.start); err != nil {
				t.Fatalf("chmod %s to %v: %v", path, tt.start, err)
			}

			if err := settlePDFMode(path); err != nil {
				t.Fatalf("settlePDFMode: %v", err)
			}

			info, err := os.Stat(path)
			if err != nil {
				t.Fatalf("stat: %v", err)
			}
			if got := info.Mode().Perm(); got != tt.want {
				t.Fatalf("mode = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestWriteAtomicFileHasTheExpectedMode(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "out.json")
	if err := writeAtomicFile(path, []byte("data")); err != nil {
		t.Fatalf("writeAtomicFile: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if got, want := info.Mode().Perm(), expected0644Mode(t); got != want {
		t.Fatalf("mode = %v, want %v", got, want)
	}
}

func TestWriteAtomicFileLeavesNoTempFileWhenRenameFails(t *testing.T) {
	dir := t.TempDir()
	// path itself is an existing folder, so the final os.Rename fails after
	// the temporary file has already been reserved and written; that
	// temporary file must still be cleaned up.
	path := filepath.Join(dir, "out.json")
	if err := os.Mkdir(path, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", path, err)
	}

	err := writeAtomicFile(path, []byte("data"))
	if err == nil {
		t.Fatal("writeAtomicFile onto an existing folder = nil error, want one")
	}
	assertNoTempFilesBeside(t, dir)
}

// TestWritableCheckReportsAProbeItCannotRemove forces checkWritableTarget's
// clean-up of its own probe file to fail, through the private seam
// removeProbeFile, restored with t.Cleanup so no other test in the package
// ever sees the replacement.
func TestWritableCheckReportsAProbeItCannotRemove(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "r.pdf")

	boom := errors.New("boom")
	old := removeProbeFile
	removeProbeFile = func(string) error { return boom }
	t.Cleanup(func() { removeProbeFile = old })

	err := checkOutputPath(path)
	if err == nil {
		t.Fatal("checkOutputPath() = nil, want an error")
	}
	if !strings.Contains(err.Error(), "output directory holds a probe file that could not be removed") {
		t.Fatalf("error = %q, want it to mention the probe file", err.Error())
	}
	if !strings.Contains(err.Error(), "boom") {
		t.Fatalf("error = %q, want it to name the cause", err.Error())
	}
}

// expected0644Mode returns the mode a file created with 0o644 will actually
// get on this machine, right now: 0o644 with every bit the process umask
// clears also cleared. It measures the umask indirectly, by creating a
// throwaway file with 0o666 and reading back what survived, rather than
// reading or setting the umask itself (umask has no portable read-only
// accessor in Go: reading it means setting it, if only for an instant,
// which would race any other goroutine creating a file at the same time).
// Since 0o644's bits are a subset of 0o666's, ANDing the measured mode with
// 0o644 gives exactly "0o644 less the same umask", for any umask.
func expected0644Mode(t *testing.T) os.FileMode {
	t.Helper()
	path := filepath.Join(t.TempDir(), "umask-probe")
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o666)
	if err != nil {
		t.Fatalf("probing the umask: %v", err)
	}
	defer func() { _ = f.Close() }()
	info, err := f.Stat()
	if err != nil {
		t.Fatalf("stat of the umask probe: %v", err)
	}
	return os.FileMode(0o644) & info.Mode().Perm()
}

// assertNoTempFilesBeside fails the test if dir holds any ".*.tmp" entry.
func assertNoTempFilesBeside(t *testing.T, dir string) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("reading %s: %v", dir, err)
	}
	for _, e := range entries {
		name := e.Name()
		if len(name) > 0 && name[0] == '.' {
			t.Errorf("%s left a temporary file behind: %s", dir, name)
		}
	}
}
