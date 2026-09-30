package analysis

import (
	"testing"

	"github.com/marceloalvescf/k8s-capacity-report/internal/collect"
	"github.com/marceloalvescf/k8s-capacity-report/internal/discovery"
)

func known(v float64) collect.Number { return collect.Number{Value: v, Known: true} }

// testNode builds a node of the pool "p" with exact confidence and no
// numbers; a test sets the fields it cares about.
func testNode(name string) collect.Node {
	return collect.Node{Name: name, Pool: "p", PoolConfidence: discovery.ConfidenceExact}
}

// numberEvidence reads the number evidence of a finding, in order, as
// name and value pairs.
func numberEvidence(t *testing.T, f Finding) map[string]float64 {
	t.Helper()
	out := map[string]float64{}
	for _, e := range f.Evidence {
		if e.Value.Kind != KindNumber {
			t.Fatalf("%s: evidence %s is %q, want number", f.Code, e.Name, e.Value.Kind)
		}
		out[e.Name] = e.Value.Number
	}
	return out
}

func evidenceList(f Finding) []string {
	names := []string{}
	for _, e := range f.Evidence {
		names = append(names, e.Name)
	}
	return names
}

func byCode(findings []Finding, code string) []Finding {
	var out []Finding
	for _, f := range findings {
		if f.Code == code {
			out = append(out, f)
		}
	}
	return out
}

// only fails unless findings holds exactly one finding, and returns it.
func only(t *testing.T, findings []Finding) Finding {
	t.Helper()
	if len(findings) != 1 {
		t.Fatalf("got %d findings, want 1: %+v", len(findings), findings)
	}
	return findings[0]
}

func codesOf(findings []Finding) []string {
	out := []string{}
	for _, f := range findings {
		out = append(out, f.Code)
	}
	return out
}
