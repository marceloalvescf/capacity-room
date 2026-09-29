// Package template embeds the generic report template — the Graphite
// package — and the capacity-report layer on top of it, and unpacks both
// onto disk for the report pipeline to compile.
package template

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

//go:embed graphite/typst.toml graphite/lib.typ graphite/src graphite/assets capacity
var embedded embed.FS

// Unpack writes the embedded template under dir, into a folder that is
// either missing or already empty. If dir does not exist, Unpack creates
// it, with any missing parents, mode 0o755. If dir exists and is an empty
// folder, Unpack writes into it. If dir exists and holds any entry, exists
// and is not a folder, or is itself a symbolic link — to a folder, to a
// file, or dangling — Unpack writes nothing and returns an error. Only the
// final path component of dir is checked for a symbolic link this way: a
// parent folder reached through one, such as /tmp on macOS, is accepted.
//
// This refusal is deliberate: writing into a folder that already holds
// something, or that is itself reached through a link, is how a previous
// run's leftovers or a planted link get written through. An empty folder
// holds no link to follow, so the caller is never asked to inspect one —
// dir itself excepted, which os.Lstat checks directly, without following it.
//
// If Unpack returns a non-nil error, the content of dir afterward is
// undefined: the caller must discard the folder rather than reuse it.
func Unpack(dir string) error {
	info, err := os.Lstat(dir)
	switch {
	case os.IsNotExist(err):
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	case err != nil:
		return fmt.Errorf("template: %w", err)
	case info.Mode()&os.ModeSymlink != 0:
		return fmt.Errorf("template: %s is a symbolic link", dir)
	case !info.IsDir():
		return fmt.Errorf("template: %s is not a folder", dir)
	default:
		entries, err := os.ReadDir(dir)
		if err != nil {
			return fmt.Errorf("template: %w", err)
		}
		if len(entries) != 0 {
			return fmt.Errorf("template: folder %s is not empty", dir)
		}
	}

	// fs.WalkDir visits a directory before any of its children, so every
	// parent folder is already created by the time a file inside it is
	// written.
	return fs.WalkDir(embedded, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		target := filepath.Join(dir, path)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		data, err := embedded.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o644)
	})
}

// FontPath returns the font folder inside an unpacked template.
func FontPath(dir string) string {
	return filepath.Join(dir, "graphite", "assets", "fonts")
}

// Entry returns the report entry point inside an unpacked template.
func Entry(dir string) string {
	return filepath.Join(dir, "capacity", "main.typ")
}
