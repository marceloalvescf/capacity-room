//go:build template

package template

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

var liberationSansFontFiles = []string{
	"LiberationSans-Regular.ttf",
	"LiberationSans-Bold.ttf",
	"LiberationSans-Italic.ttf",
	"LiberationSans-BoldItalic.ttf",
}

func TestFontFacesExist(t *testing.T) {
	ws := workspace(t)
	fontDir := FontPath(ws)
	for _, name := range liberationSansFontFiles {
		path := filepath.Join(fontDir, name)
		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("stat %s: %v", path, err)
		}
		if info.Size() == 0 {
			t.Fatalf("%s is empty", path)
		}
	}
}

func TestTypstListsLiberationSans(t *testing.T) {
	tc := toolchain(t)
	ws := workspace(t)

	cmd := exec.Command(tc.Binary, "fonts",
		"--font-path", FontPath(ws),
		"--ignore-system-fonts",
		"--ignore-embedded-fonts",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("typst fonts: %v\n%s", err, out)
	}

	for _, line := range strings.Split(string(out), "\n") {
		if strings.HasPrefix(line, "Liberation Sans") {
			return
		}
	}
	t.Fatalf("typst fonts output = %q, want a line starting with %q", out, "Liberation Sans")
}
