//go:build template

package template

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/BurntSushi/toml"

	"github.com/marceloalvescf/capacity-room/internal/typst"
)

// packageManifest mirrors the subset of typst.toml this harness relies on.
type packageManifest struct {
	Package struct {
		Version  string   `toml:"version"`
		Compiler string   `toml:"compiler"`
		Exclude  []string `toml:"exclude"`
	} `toml:"package"`
}

func readManifest(t *testing.T, path string) packageManifest {
	t.Helper()
	var manifest packageManifest
	if _, err := toml.DecodeFile(path, &manifest); err != nil {
		t.Fatalf("decoding %s: %v", path, err)
	}
	return manifest
}

// excludeTopLevel returns a copyTree skip function that leaves out any
// entry whose path segment directly under src matches a name in exclude,
// the same "exclude" a real Typst package installer honours.
func excludeTopLevel(src string, exclude []string) func(path string) bool {
	excluded := make(map[string]bool, len(exclude))
	for _, name := range exclude {
		excluded[name] = true
	}
	return func(path string) bool {
		rel, err := filepath.Rel(src, path)
		if err != nil || rel == "." {
			return false
		}
		top := strings.SplitN(rel, string(filepath.Separator), 2)[0]
		return excluded[top]
	}
}

// TestPackageInstallsByCopy proves the package can be consumed the way its
// own README documents: copied (minus its excluded entries) into a local
// package cache and imported by @local/graphite-report:<version>, with no
// installer beyond a plain file copy.
func TestPackageInstallsByCopy(t *testing.T) {
	ws := workspace(t)
	graphiteDir := filepath.Join(ws, "graphite")
	manifest := readManifest(t, filepath.Join(graphiteDir, "typst.toml"))

	temp := t.TempDir()
	pkgRoot := filepath.Join(temp, "pkgs")
	pkgDir := filepath.Join(pkgRoot, "local", "graphite-report", manifest.Package.Version)
	copyTree(t, graphiteDir, pkgDir, excludeTopLevel(graphiteDir, manifest.Package.Exclude))

	docDir := filepath.Join(temp, "doc")
	if err := os.MkdirAll(docDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(%s): %v", docDir, err)
	}
	mainSrc := filepath.Join(graphiteDir, "template", "main.typ")
	mainData, err := os.ReadFile(mainSrc)
	if err != nil {
		t.Fatalf("reading %s: %v", mainSrc, err)
	}
	docMain := filepath.Join(docDir, "main.typ")
	if err := os.WriteFile(docMain, mainData, 0o644); err != nil {
		t.Fatalf("writing %s: %v", docMain, err)
	}

	tc := toolchain(t)
	output := filepath.Join(temp, "out.pdf")
	job := typst.Job{
		Root:        docDir,
		FontPath:    filepath.Join(pkgDir, "assets", "fonts"),
		Source:      docMain,
		Output:      output,
		Format:      typst.PDF,
		PackagePath: pkgRoot,
	}
	stderr, err := tc.Compile(context.Background(), job)
	assertCleanCompile(t, stderr, err)

	text := pdfText(t, output)
	want := "Relatório corporativo"
	if !strings.Contains(text, want) {
		t.Fatalf("pdf text = %q, want it to contain %q", text, want)
	}
}
