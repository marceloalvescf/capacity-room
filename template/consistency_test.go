//go:build template

package template

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	kcr "github.com/marceloalvescf/k8s-capacity-report"
	"github.com/marceloalvescf/k8s-capacity-report/internal/typst"
)

var packageIdentifierPattern = regexp.MustCompile(`@local/graphite-report:(\d+\.\d+\.\d+)`)

func TestManifestVersionEqualsVersionFile(t *testing.T) {
	manifest := readManifest(t, filepath.Join(moduleRoot(t), "template", "graphite", "typst.toml"))
	if manifest.Package.Version != kcr.Version() {
		t.Fatalf("typst.toml package.version = %q, want %q (kcr.Version())", manifest.Package.Version, kcr.Version())
	}
}

func TestManifestCompilerIsThePinnedVersion(t *testing.T) {
	manifest := readManifest(t, filepath.Join(moduleRoot(t), "template", "graphite", "typst.toml"))
	if manifest.Package.Compiler != typst.RequiredVersion {
		t.Fatalf("typst.toml package.compiler = %q, want %q (typst.RequiredVersion)", manifest.Package.Compiler, typst.RequiredVersion)
	}
}

// templateFiles returns every file under template/ in the source tree
// (not a workspace copy) whose extension is one of exts.
func templateFiles(t *testing.T, exts map[string]bool) []string {
	t.Helper()
	root := filepath.Join(moduleRoot(t), "template")
	var files []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if exts[filepath.Ext(path)] {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking %s: %v", root, err)
	}
	return files
}

func TestEveryPackageIdentifierUsesTheVersion(t *testing.T) {
	files := templateFiles(t, map[string]bool{".typ": true, ".toml": true, ".md": true})
	version := kcr.Version()
	found := false
	for _, f := range files {
		content, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("reading %s: %v", f, err)
		}
		for _, m := range packageIdentifierPattern.FindAllStringSubmatch(string(content), -1) {
			found = true
			if m[1] != version {
				t.Errorf("%s: identifier uses version %q, want %q", f, m[1], version)
			}
		}
	}
	if !found {
		t.Fatalf("no @local/graphite-report:X.Y.Z identifier found under template/")
	}
}

// stripLineComments removes everything from "//" to the end of each line,
// so a mention of eval( inside a comment does not count as real usage.
func stripLineComments(content string) string {
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		if idx := strings.Index(line, "//"); idx >= 0 {
			lines[i] = line[:idx]
		}
	}
	return strings.Join(lines, "\n")
}

func TestNoEvalInTemplate(t *testing.T) {
	files := templateFiles(t, map[string]bool{".typ": true})
	if len(files) == 0 {
		t.Fatalf("no .typ files found under template/")
	}
	for _, f := range files {
		raw, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("reading %s: %v", f, err)
		}
		if strings.Contains(stripLineComments(string(raw)), "eval(") {
			t.Errorf("%s contains eval(", f)
		}
	}
}
