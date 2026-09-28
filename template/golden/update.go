package golden

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/marceloalvescf/k8s-capacity-report/internal/typst"
)

// renderFunc renders one snapshot into dir, compiling from root. It is the
// shape of golden.Render with the toolchain already bound, so update can be
// tested with a fake renderer that never touches a real typst binary.
type renderFunc func(ctx context.Context, root string, s Snapshot, dir string) error

// Update rewrites every golden page: it renders every entry of Snapshots
// into a scratch folder first, and only once every page has rendered
// successfully does it delete the existing "*.png" in goldenDir and copy
// the new pages in, so a failed render never empties goldenDir. It returns
// the names written (every rendered PNG, plus "SHA256SUMS" last), sorted,
// for a caller to report one per line.
func Update(ctx context.Context, tc typst.Toolchain, templateDir, goldenDir string) ([]string, error) {
	render := func(ctx context.Context, root string, s Snapshot, dir string) error {
		return Render(ctx, tc, root, s, dir)
	}
	return update(ctx, render, Snapshots, templateDir, goldenDir)
}

// update is Update with the renderer injected, so a test can replace the
// typst compile with a fake that writes small files.
//
// It renders every snapshot into a scratch folder (removed before
// returning, on every path), verifies the scratch folder holds exactly the
// files every snapshot's Files() names, and only then touches goldenDir:
// existing PNGs are deleted, the new pages are copied in, and SHA256SUMS is
// written. Nothing in goldenDir is touched before that point, so a render
// that fails, or a renderer that silently drops a page, leaves the
// existing golden pages exactly as they were.
func update(ctx context.Context, render renderFunc, snapshots []Snapshot, templateDir, goldenDir string) ([]string, error) {
	scratch, err := os.MkdirTemp("", "kcr-golden-*")
	if err != nil {
		return nil, fmt.Errorf("creating scratch folder: %w", err)
	}
	defer func() { _ = os.RemoveAll(scratch) }()

	for _, s := range snapshots {
		if err := render(ctx, templateDir, s, scratch); err != nil {
			return nil, err
		}
	}

	names, err := verifyRendered(scratch, snapshots)
	if err != nil {
		return nil, err
	}

	if err := os.MkdirAll(goldenDir, 0o755); err != nil {
		return nil, fmt.Errorf("creating %s: %w", goldenDir, err)
	}
	if err := removePNGs(goldenDir); err != nil {
		return nil, err
	}

	sums := make(map[string]string, len(names))
	for _, name := range names {
		data, err := os.ReadFile(filepath.Join(scratch, name))
		if err != nil {
			return nil, fmt.Errorf("reading rendered %s: %w", name, err)
		}
		dst := filepath.Join(goldenDir, name)
		if err := os.WriteFile(dst, data, 0o644); err != nil {
			return nil, fmt.Errorf("writing %s: %w", dst, err)
		}
		digest, err := Digest(dst)
		if err != nil {
			return nil, fmt.Errorf("digesting %s: %w", dst, err)
		}
		sums[name] = digest
	}

	sumsPath := filepath.Join(goldenDir, "SHA256SUMS")
	if err := WriteSums(sumsPath, sums); err != nil {
		return nil, fmt.Errorf("writing %s: %w", sumsPath, err)
	}

	written := append(append([]string(nil), names...), "SHA256SUMS")
	return written, nil
}

// verifyRendered checks that dir holds exactly the files every snapshot's
// Files() names: no fewer (a renderer that reports success without
// writing a page) and no more (a stray file that would otherwise reach
// goldenDir uninvited). It returns the expected names, sorted.
func verifyRendered(dir string, snapshots []Snapshot) ([]string, error) {
	expected := make(map[string]bool)
	names := make([]string, 0)
	for _, s := range snapshots {
		for _, name := range s.Files() {
			expected[name] = true
			names = append(names, name)
		}
	}
	sort.Strings(names)

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("reading scratch folder %s: %w", dir, err)
	}
	present := make(map[string]bool, len(entries))
	for _, e := range entries {
		present[e.Name()] = true
	}

	for _, name := range names {
		if !present[name] {
			return nil, fmt.Errorf("rendering: expected page %s was not written", name)
		}
	}
	for name := range present {
		if !expected[name] {
			return nil, fmt.Errorf("rendering: unexpected file %s in scratch folder", name)
		}
	}
	return names, nil
}

// removePNGs deletes every regular file directly inside dir whose name
// ends in ".png". It never deletes a folder, even one whose name happens
// to end in ".png", and never looks inside a subfolder. A dir that does
// not exist yet is not an error: Update must also work the first time,
// before template/tests/golden/ has ever been created.
func removePNGs(dir string) error {
	matches, err := filepath.Glob(filepath.Join(dir, "*.png"))
	if err != nil {
		return fmt.Errorf("listing %s: %w", dir, err)
	}
	for _, match := range matches {
		info, err := os.Lstat(match)
		if err != nil {
			return fmt.Errorf("checking %s: %w", match, err)
		}
		if info.IsDir() {
			continue
		}
		if err := os.Remove(match); err != nil {
			return fmt.Errorf("removing %s: %w", match, err)
		}
	}
	return nil
}
