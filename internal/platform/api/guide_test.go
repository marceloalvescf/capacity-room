package api

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

const codesMarker = "<!-- table: failure-codes -->"

type guideCode struct {
	code     string
	storedOn string
}

// guideCodes returns the code and the "Stored on" cell of each data row of
// the table that follows the failure-codes marker.
func guideCodes(guide string) ([]guideCode, error) {
	_, after, found := strings.Cut(guide, codesMarker+"\n")
	if !found {
		return nil, fmt.Errorf("the failure-codes marker is missing from the guide")
	}
	var rows []guideCode
	for i, line := range strings.Split(after, "\n") {
		if !strings.HasPrefix(line, "|") {
			break
		}
		if i < 2 {
			continue
		}
		cells := strings.Split(line, "|")
		parts := strings.Split(cells[1], "`")
		if len(parts) < 3 || len(cells) < 3 {
			return nil, fmt.Errorf("row %d of the codes table is malformed", i)
		}
		rows = append(rows, guideCode{code: parts[1], storedOn: strings.TrimSpace(cells[2])})
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("the failure-codes table holds no row")
	}
	return rows, nil
}

func contractFailureCodes(t *testing.T) (report, pdf []string) {
	t.Helper()
	raw, err := os.ReadFile(contractPath(t))
	if err != nil {
		t.Fatalf("reading the contract: %v", err)
	}
	var doc struct {
		Components struct {
			Schemas struct {
				Report struct {
					Enum []string `yaml:"enum"`
				} `yaml:"ReportFailureCode"`
				PDF struct {
					Enum []string `yaml:"enum"`
				} `yaml:"PDFFailureCode"`
			} `yaml:"schemas"`
		} `yaml:"components"`
	}
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("parsing the contract: %v", err)
	}
	return doc.Components.Schemas.Report.Enum, doc.Components.Schemas.PDF.Enum
}

func setOf(items []string) map[string]bool {
	m := map[string]bool{}
	for _, s := range items {
		m[s] = true
	}
	return m
}

// codesDrift names, for each enum, the codes the guide lacks and the codes it
// adds.
func codesDrift(guide string, report, pdf []string) (string, error) {
	rows, err := guideCodes(guide)
	if err != nil {
		return "", err
	}
	var out []string
	for _, e := range []struct {
		name, word string
		codes      []string
	}{{"ReportFailureCode", "report", report}, {"PDFFailureCode", "PDF", pdf}} {
		inGuide := map[string]bool{}
		for _, r := range rows {
			for _, w := range strings.Fields(r.storedOn) {
				if w == e.word {
					inGuide[r.code] = true
				}
			}
		}
		missing := difference(setOf(e.codes), inGuide)
		extra := difference(inGuide, setOf(e.codes))
		if len(missing) > 0 || len(extra) > 0 {
			sort.Strings(missing)
			sort.Strings(extra)
			out = append(out, fmt.Sprintf("%s: missing from the guide %v, extra in the guide %v", e.name, missing, extra))
		}
	}
	return strings.Join(out, "; "), nil
}

func TestOperationsGuideNamesEveryFailureCode(t *testing.T) {
	raw, err := os.ReadFile("../../../docs/operations.md")
	if err != nil {
		t.Fatalf("reading the guide: %v", err)
	}
	guide := string(raw)
	report, pdf := contractFailureCodes(t)
	if len(report) == 0 || len(pdf) == 0 {
		t.Fatalf("the contract holds %d report codes and %d PDF codes", len(report), len(pdf))
	}

	drift, err := codesDrift(guide, report, pdf)
	if err != nil {
		t.Fatal(err)
	}
	if drift != "" {
		t.Fatalf("the guide and the contract differ: %s", drift)
	}

	t.Run("a removed row is seen", func(t *testing.T) {
		var kept []string
		for _, l := range strings.Split(guide, "\n") {
			if !strings.HasPrefix(l, "| `SNAPSHOT_TOO_LARGE` |") {
				kept = append(kept, l)
			}
		}
		drift, err := codesDrift(strings.Join(kept, "\n"), report, pdf)
		if err != nil || !strings.Contains(drift, "SNAPSHOT_TOO_LARGE") {
			t.Fatalf("drift = %q, err = %v; want SNAPSHOT_TOO_LARGE reported", drift, err)
		}
	})
	t.Run("an extra row is seen", func(t *testing.T) {
		mutated := strings.Replace(guide, "| `INTERNAL_FAILURE` |", "| `INVENTED_CODE` | report | no | x |\n| `INTERNAL_FAILURE` |", 1)
		drift, err := codesDrift(mutated, report, pdf)
		if err != nil || !strings.Contains(drift, "INVENTED_CODE") {
			t.Fatalf("drift = %q, err = %v; want INVENTED_CODE reported", drift, err)
		}
	})
	t.Run("a code moved from report to PDF is seen", func(t *testing.T) {
		mutated := strings.Replace(guide, "| `SNAPSHOT_TOO_LARGE` | report |", "| `SNAPSHOT_TOO_LARGE` | PDF |", 1)
		drift, err := codesDrift(mutated, report, pdf)
		if err != nil || !strings.Contains(drift, "SNAPSHOT_TOO_LARGE") {
			t.Fatalf("drift = %q, err = %v; want SNAPSHOT_TOO_LARGE reported", drift, err)
		}
	})
	t.Run("a removed marker is seen", func(t *testing.T) {
		if _, err := codesDrift(strings.Replace(guide, codesMarker, "", 1), report, pdf); err == nil {
			t.Fatal("a guide without the marker was accepted")
		}
	})
}
