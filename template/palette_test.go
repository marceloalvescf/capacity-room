package template

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// palettePairs binds each colour of capacity/palette.typ to the custom
// property of the web theme that it must equal.
var palettePairs = []struct{ typst, css string }{
	{"critical", "--critical"},
	{"attention", "--attention"},
	{"healthy", "--healthy"},
	{"neutral", "--neutral"},
	{"critical-surface", "--critical-surface"},
	{"attention-surface", "--attention-surface"},
}

func TestPaletteEqualsTheWebTheme(t *testing.T) {
	palette, err := os.ReadFile("capacity/palette.typ")
	if err != nil {
		t.Fatalf("reading palette.typ: %v", err)
	}
	css, err := os.ReadFile("../web/src/theme/theme.css")
	if err != nil {
		t.Fatalf("reading theme.css: %v", err)
	}
	_, root, ok := strings.Cut(string(css), ":root {")
	if !ok {
		t.Fatal("theme.css holds no :root block")
	}
	root, _, _ = strings.Cut(root, "}")

	for _, pair := range palettePairs {
		typ := regexp.MustCompile(`(?m)^#let ` + regexp.QuoteMeta(pair.typst) + ` = rgb\("(#[0-9a-f]{6})"\)`).FindSubmatch(palette)
		if typ == nil {
			t.Errorf("palette.typ does not define %s as rgb(\"#rrggbb\")", pair.typst)
			continue
		}
		web := regexp.MustCompile(`(?m)^\s*` + regexp.QuoteMeta(pair.css) + `:\s*(#[0-9a-f]{6});`).FindStringSubmatch(root)
		if web == nil {
			t.Errorf("the :root block of theme.css does not define %s", pair.css)
			continue
		}
		if string(typ[1]) != web[1] {
			t.Errorf("%s = %s in palette.typ, but %s = %s in theme.css", pair.typst, typ[1], pair.css, web[1])
		}
	}
}

var colourLiteral = regexp.MustCompile(
	`rgb\(|cmyk\(|color\.|\b(red|green|blue|yellow|orange|purple|aqua|teal|eastern|navy|maroon|fuchsia|olive|lime|silver|gray)\b`)

func TestNoColourOutsideThePalette(t *testing.T) {
	err := filepath.WalkDir(".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || filepath.Ext(path) != ".typ" || path == filepath.Join("capacity", "palette.typ") || strings.HasPrefix(path, "tests"+string(filepath.Separator)) {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for i, line := range strings.Split(string(data), "\n") {
			if hit := colourLiteral.FindString(line); hit != "" {
				t.Errorf("%s:%d holds the colour literal %q; colours live in capacity/palette.typ", path, i+1, hit)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
