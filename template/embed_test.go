package template

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
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

func TestUnpackIntoExistingFolder(t *testing.T) {
	dir := t.TempDir()
	if err := Unpack(dir); err != nil {
		t.Fatalf("first Unpack() error = %v", err)
	}
	if err := Unpack(dir); err != nil {
		t.Fatalf("second Unpack() error = %v", err)
	}
	assertUnpackedMatchesEmbedded(t, dir)
}

// TestEmbeddedFilesMatchDisk compares every embedded file against the file
// on disk at the same relative path. `go test` runs with the package
// directory as its working directory, so that relative path already points
// at the source tree; no module-root lookup is needed here.
func TestEmbeddedFilesMatchDisk(t *testing.T) {
	walkEmbeddedFiles(t, func(path string) error {
		want, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("reading source file %s: %v", path, err)
		}
		got, err := embedded.ReadFile(path)
		if err != nil {
			return err
		}
		if !bytes.Equal(got, want) {
			t.Fatalf("embedded %s does not match the file on disk", path)
		}
		return nil
	})
}

func TestFontLicenceIsEmbedded(t *testing.T) {
	if _, err := embedded.ReadFile("graphite/assets/fonts/LICENSE"); err != nil {
		t.Fatalf("reading embedded font licence: %v", err)
	}
}
