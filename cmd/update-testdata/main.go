// Command update-testdata rewrites the golden snapshot files under
// testdata/snapshot/ and the golden data files under
// template/tests/capacity/fixtures/command/. It is never run by a test; run
// it deliberately, read every file it changes, and review the change
// before committing it.
package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/marceloalvescf/k8s-capacity-report/internal/catalog"
	"github.com/marceloalvescf/k8s-capacity-report/internal/goldentest"
	"github.com/marceloalvescf/k8s-capacity-report/internal/promtest"
)

// moduleRoot walks up from the working directory to the first folder
// holding go.mod, the same way internal/goldentest does; it is duplicated
// here because it is a handful of lines and package main cannot usefully
// export it to import instead.
func moduleRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("getting working directory: %w", err)
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

// writeIfChanged writes data to path only when it differs from the file
// already there, through a temporary file beside path and a rename, so
// that a run that fails half way leaves no half file. It prints "written"
// or "unchanged" followed by path relative to root.
func writeIfChanged(root, path string, data []byte) error {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return fmt.Errorf("relating %s to %s: %w", path, root, err)
	}

	if existing, err := os.ReadFile(path); err == nil {
		if string(existing) == string(data) {
			fmt.Println("unchanged", rel)
			return nil
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("reading %s: %w", path, err)
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("creating %s: %w", dir, err)
	}

	tmp, err := os.CreateTemp(dir, ".update-testdata-*")
	if err != nil {
		return fmt.Errorf("creating a temporary file beside %s: %w", path, err)
	}
	tmpPath := tmp.Name()
	defer func() { _ = os.Remove(tmpPath) }()

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("writing %s: %w", tmpPath, err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("closing %s: %w", tmpPath, err)
	}
	if err := os.Chmod(tmpPath, 0o644); err != nil {
		return fmt.Errorf("setting permissions of %s: %w", tmpPath, err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("renaming %s to %s: %w", tmpPath, path, err)
	}

	fmt.Println("written", rel)
	return nil
}

func run(ctx context.Context) error {
	root, err := moduleRoot()
	if err != nil {
		return err
	}

	snapshots := make(map[string][]byte, len(promtest.Scenarios))
	for _, scenario := range promtest.Scenarios {
		data, err := goldentest.Snapshot(ctx, scenario)
		if err != nil {
			return err
		}
		snapshots[scenario.Name] = data

		path, err := goldentest.SnapshotPath(scenario.Name)
		if err != nil {
			return err
		}
		if err := writeIfChanged(root, path, data); err != nil {
			return err
		}

		queries, err := goldentest.Queries(ctx, scenario)
		if err != nil {
			return err
		}
		queriesPath, err := goldentest.QueriesPath(scenario.Name)
		if err != nil {
			return err
		}
		if err := writeIfChanged(root, queriesPath, queries); err != nil {
			return err
		}
	}

	kinds := []struct {
		profile   string
		scenarios []string
		path      func(scenario string, lang catalog.Language) (string, error)
	}{
		{"", goldentest.DataFileScenarios, goldentest.DataFilePath},
		{"executive", goldentest.ExecutiveDataFileScenarios, goldentest.ExecutiveDataFilePath},
	}
	for _, kind := range kinds {
		for _, name := range kind.scenarios {
			for _, lang := range catalog.Languages() {
				doc, err := goldentest.DataFile(snapshots[name], lang, kind.profile)
				if err != nil {
					return fmt.Errorf("%s (%s): %w", name, lang, err)
				}

				path, err := kind.path(name, lang)
				if err != nil {
					return err
				}
				if err := writeIfChanged(root, path, doc); err != nil {
					return err
				}
			}
		}
	}

	return nil
}

func main() {
	if err := run(context.Background()); err != nil {
		fmt.Fprintln(os.Stderr, "update-testdata:", err)
		os.Exit(1)
	}
}
