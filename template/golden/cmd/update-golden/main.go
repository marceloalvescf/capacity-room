// Command update-golden rewrites every golden page under
// template/tests/golden/. It is never run by a test; run it deliberately,
// for an intended visual change, and review every page it writes before
// committing.
package main

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/marceloalvescf/k8s-capacity-report/internal/typst"
	"github.com/marceloalvescf/k8s-capacity-report/template/golden"
)

// moduleRoot walks up from the working folder to the first folder holding
// go.mod, the same way template/harness_test.go and internal/guard do; it
// is duplicated here because it is one of a handful of lines and package
// main cannot usefully export it to import instead.
func moduleRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("go.mod not found above %s", dir)
		}
		dir = parent
	}
}

// copyTemplate copies src (the template folder) into dst, leaving out every
// ".go" file. The compile root must not contain Go source: the source tree
// is otherwise mounted read-only when this command runs in the test image,
// and Typst has no use for it regardless.
func copyTemplate(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		if filepath.Ext(path) == ".go" {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o644)
	})
}

func run(ctx context.Context) error {
	root, err := moduleRoot()
	if err != nil {
		return err
	}
	templateSrc := filepath.Join(root, "template")
	goldenDir := filepath.Join(templateSrc, "tests", "golden")

	scratch, err := os.MkdirTemp("", "kcr-update-golden-*")
	if err != nil {
		return fmt.Errorf("creating scratch folder: %w", err)
	}
	defer func() { _ = os.RemoveAll(scratch) }()

	if err := copyTemplate(templateSrc, scratch); err != nil {
		return fmt.Errorf("copying template to %s: %w", scratch, err)
	}

	tc, err := typst.Locate(os.Getenv, exec.LookPath)
	if err != nil {
		return err
	}
	if err := tc.Probe(ctx); err != nil {
		return err
	}

	if err := os.MkdirAll(goldenDir, 0o755); err != nil {
		return fmt.Errorf("creating %s: %w", goldenDir, err)
	}

	written, err := golden.Update(ctx, tc, scratch, goldenDir)
	if err != nil {
		return err
	}
	for _, name := range written {
		fmt.Println(name)
	}
	return nil
}

func main() {
	if err := run(context.Background()); err != nil {
		fmt.Fprintln(os.Stderr, "update-golden:", err)
		os.Exit(1)
	}
}
