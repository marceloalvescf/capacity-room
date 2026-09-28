package golden

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/marceloalvescf/k8s-capacity-report/internal/typst"
)

// Update rewrites every golden page: it deletes every existing "*.png" in
// goldenDir, renders every entry of Snapshots straight into goldenDir
// (compiling from templateDir, a compile root), and writes goldenDir's
// SHA256SUMS. It returns the names written (every rendered PNG, plus
// "SHA256SUMS"), sorted, for a caller to report one per line.
//
// Existing PNGs are removed before any render, rather than after, so that a
// snapshot renamed or dropped from Snapshots never leaves a stale page
// behind for TestGoldenManifestMatchesFiles to silently accept.
func Update(ctx context.Context, tc typst.Toolchain, templateDir, goldenDir string) ([]string, error) {
	if err := removePNGs(goldenDir); err != nil {
		return nil, err
	}

	sums := make(map[string]string)
	for _, s := range Snapshots {
		if err := Render(ctx, tc, templateDir, s, goldenDir); err != nil {
			return nil, err
		}
		for _, name := range s.Files() {
			digest, err := Digest(filepath.Join(goldenDir, name))
			if err != nil {
				return nil, fmt.Errorf("digesting %s: %w", name, err)
			}
			sums[name] = digest
		}
	}

	sumsPath := filepath.Join(goldenDir, "SHA256SUMS")
	if err := WriteSums(sumsPath, sums); err != nil {
		return nil, fmt.Errorf("writing %s: %w", sumsPath, err)
	}

	written := make([]string, 0, len(sums)+1)
	for name := range sums {
		written = append(written, name)
	}
	sort.Strings(written)
	written = append(written, "SHA256SUMS")
	return written, nil
}

// removePNGs deletes every "*.png" file directly inside dir. A dir that
// does not exist yet is not an error: update-golden must also work the
// first time, before template/tests/golden/ has ever been created.
func removePNGs(dir string) error {
	matches, err := filepath.Glob(filepath.Join(dir, "*.png"))
	if err != nil {
		return fmt.Errorf("listing %s: %w", dir, err)
	}
	for _, match := range matches {
		if err := os.Remove(match); err != nil {
			return fmt.Errorf("removing %s: %w", match, err)
		}
	}
	return nil
}
