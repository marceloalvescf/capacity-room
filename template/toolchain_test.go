//go:build template

package template

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

var fontFiles = []string{
	"LiberationSans-Regular.ttf",
	"LiberationSans-Bold.ttf",
	"LiberationSans-Italic.ttf",
	"LiberationSans-BoldItalic.ttf",
	"PublicSans-Regular.ttf",
	"PublicSans-Italic.ttf",
	"PublicSans-SemiBold.ttf",
	"PublicSans-SemiBoldItalic.ttf",
	"PublicSans-Bold.ttf",
	"PublicSans-BoldItalic.ttf",
	"JetBrainsMono-Regular.ttf",
	"JetBrainsMono-Italic.ttf",
	"JetBrainsMono-SemiBold.ttf",
	"JetBrainsMono-Bold.ttf",
}

func TestFontFacesExist(t *testing.T) {
	ws := workspace(t)
	fontDir := FontPath(ws)
	for _, name := range fontFiles {
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

func TestTypstListsTheThreeFaces(t *testing.T) {
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

	lines := map[string]bool{}
	for _, line := range strings.Split(string(out), "\n") {
		lines[strings.TrimSpace(line)] = true
	}
	for _, want := range []string{"JetBrains Mono", "Liberation Sans", "Public Sans"} {
		if !lines[want] {
			t.Errorf("typst fonts output = %q, want the line %q", out, want)
		}
	}
}
