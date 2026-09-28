// Package template embeds the generic report template — the Graphite
// package — and unpacks it onto disk for the report pipeline to compile.
// Task 6 adds the capacity-report layer on top of it.
package template

import (
	"embed"
	"io/fs"
	"os"
	"path/filepath"
)

//go:embed graphite/typst.toml graphite/lib.typ graphite/src graphite/assets
var embedded embed.FS

// Unpack writes the embedded template under dir, creating it if needed.
// fs.WalkDir visits a directory before any of its children, so every
// parent folder is already created by the time a file inside it is
// written.
func Unpack(dir string) error {
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
