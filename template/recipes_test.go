//go:build template

package template

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

type recipe struct {
	title string
	block string
}

// recipeHeadingPrefix marks a level-3 README heading that documents a
// recipe, as opposed to any other level-3 heading (e.g. the API sections).
const recipeHeadingPrefix = "### Recipe"

// isMarkdownHeading reports whether line is a level-2 or level-3 Markdown
// heading, the boundary a recipe's code sample search stops at.
func isMarkdownHeading(line string) bool {
	return strings.HasPrefix(line, "## ") || strings.HasPrefix(line, "### ")
}

// firstTypstBlock returns the content of the first ```typst fenced block in
// lines, without the fences, if any.
func firstTypstBlock(lines []string) (string, bool) {
	for i, line := range lines {
		if strings.TrimSpace(line) != "```typst" {
			continue
		}
		var block strings.Builder
		for _, inner := range lines[i+1:] {
			if strings.TrimSpace(inner) == "```" {
				return block.String(), true
			}
			block.WriteString(inner)
			block.WriteString("\n")
		}
		break
	}
	return "", false
}

// readmeRecipes extracts every "### Recipe" section of readmePath, in file
// order, pairing each with the first ```typst fenced block that appears
// before the next level-2 or level-3 heading.
func readmeRecipes(t *testing.T, readmePath string) []recipe {
	t.Helper()
	raw, err := os.ReadFile(readmePath)
	if err != nil {
		t.Fatalf("reading %s: %v", readmePath, err)
	}
	lines := strings.Split(string(raw), "\n")

	var recipes []recipe
	for i, line := range lines {
		if !strings.HasPrefix(line, recipeHeadingPrefix) {
			continue
		}
		title := strings.TrimSpace(strings.TrimPrefix(line, "### "))

		end := len(lines)
		for j := i + 1; j < len(lines); j++ {
			if isMarkdownHeading(lines[j]) {
				end = j
				break
			}
		}

		block, ok := firstTypstBlock(lines[i+1 : end])
		if !ok {
			t.Fatalf("recipe %q has no ```typst code block", title)
		}
		recipes = append(recipes, recipe{title: title, block: block})
	}
	return recipes
}

// recipeDocument builds a compilable document from a recipe's code block,
// prepending the import and the show rule the block itself omits.
func recipeDocument(block string) string {
	var doc strings.Builder
	if !strings.Contains(block, "#import") {
		doc.WriteString(`#import "/graphite/lib.typ": graphite-report, callout, keep-together` + "\n")
	}
	if !strings.Contains(block, "#show: graphite-report") {
		doc.WriteString(`#show: graphite-report.with(title: [Recipe from the README])` + "\n")
	}
	doc.WriteString(block)
	return doc.String()
}

func TestReadmeDeclaresTheKnownRecipes(t *testing.T) {
	readmePath := filepath.Join(moduleRoot(t), "template", "graphite", "README.md")
	recipes := readmeRecipes(t, readmePath)

	want := []string{
		"Recipe for dense tables",
		"Recipe: machine identifiers in tables",
		"Recipe: running elements from the second page",
	}
	if len(recipes) != len(want) {
		t.Fatalf("found %d recipes, want %d: %v", len(recipes), len(want), recipes)
	}
	for i, r := range recipes {
		if r.title != want[i] {
			t.Fatalf("recipe %d title = %q, want %q", i, r.title, want[i])
		}
	}
}

func TestEveryReadmeRecipeCompilesCleanly(t *testing.T) {
	ws := workspace(t)
	readmePath := filepath.Join(ws, "graphite", "README.md")
	recipes := readmeRecipes(t, readmePath)
	if len(recipes) == 0 {
		t.Fatalf("no recipes found in %s", readmePath)
	}

	for i, r := range recipes {
		source := filepath.Join(ws, "recipe-"+strconv.Itoa(i)+".typ")
		if err := os.WriteFile(source, []byte(recipeDocument(r.block)), 0o644); err != nil {
			t.Fatalf("writing recipe document: %v", err)
		}
		rel, err := filepath.Rel(ws, source)
		if err != nil {
			t.Fatalf("computing relative path: %v", err)
		}
		t.Run(r.title, func(t *testing.T) {
			_, stderr, err := compilePDF(t, ws, rel, nil)
			assertCleanCompile(t, stderr, err)
		})
	}
}
