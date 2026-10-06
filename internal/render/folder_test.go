package render

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func assertEmpty(t *testing.T, dir string) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("%s holds %d entries, want none", dir, len(entries))
	}
}

func TestWorkFolderIsRemovedOnSuccess(t *testing.T) {
	parent := t.TempDir()
	if err := inWorkFolder(parent, func(string) error { return nil }); err != nil {
		t.Fatalf("inWorkFolder: %v", err)
	}
	assertEmpty(t, parent)
}

func TestWorkFolderIsRemovedOnError(t *testing.T) {
	parent := t.TempDir()
	boom := errors.New("boom")
	if err := inWorkFolder(parent, func(string) error { return boom }); !errors.Is(err, boom) {
		t.Fatalf("err = %v, want %v", err, boom)
	}
	assertEmpty(t, parent)
}

func TestWorkFolderIsRemovedOnPanic(t *testing.T) {
	parent := t.TempDir()
	recovered := func() (r any) {
		defer func() { r = recover() }()
		_ = inWorkFolder(parent, func(dir string) error {
			if err := os.WriteFile(filepath.Join(dir, "x"), []byte("y"), 0o644); err != nil {
				t.Fatalf("WriteFile: %v", err)
			}
			panic("boom")
		})
		return nil
	}()
	if recovered != "boom" {
		t.Fatalf("recovered = %v, want the panic to reach the caller", recovered)
	}
	assertEmpty(t, parent)
}
