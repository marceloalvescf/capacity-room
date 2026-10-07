package goldentest

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/marceloalvescf/k8s-capacity-report/internal/promtest"
)

func rangeScenario(t *testing.T) promtest.Scenario {
	t.Helper()
	for _, s := range promtest.RangeScenarios {
		if s.Name == "direct-complete-range" {
			return s
		}
	}
	t.Fatal("no range scenario named direct-complete-range")
	return promtest.Scenario{}
}

// TestRangeScenarioSendsItsTimes proves that every request of the range
// scenario carries the instants of its range, the preflight's included.
func TestRangeScenarioSendsItsTimes(t *testing.T) {
	_, fake, err := run(context.Background(), rangeScenario(t))
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	var ranges, instants int
	for _, s := range fake.Seen() {
		switch {
		case strings.HasSuffix(s.Path, "/query_range"):
			ranges++
			if got := s.Form.Get("start"); got != "1783911600" {
				t.Errorf("start = %q, want 1783911600", got)
			}
			if got := s.Form.Get("end"); got != "1783998000" {
				t.Errorf("end = %q, want 1783998000", got)
			}
			if got := s.Form.Get("step"); got != "5m" {
				t.Errorf("step = %q, want 5m", got)
			}
		case strings.HasSuffix(s.Path, "/query") && s.Query == "vector(1)":
			// The connection probe asks about no instant, as the original's does.
			if s.Form.Has("time") {
				t.Errorf("the connection probe sends time = %q", s.Form.Get("time"))
			}
		case strings.HasSuffix(s.Path, "/query"):
			instants++
			if got := s.Form.Get("time"); got != "1783998000" {
				t.Errorf("time = %q, want 1783998000 for %q", got, s.Query)
			}
		}
	}
	if ranges == 0 || instants == 0 {
		t.Fatalf("saw %d range queries and %d instant queries, want both", ranges, instants)
	}
}

// TestRangeScenarioEqualsItsBase proves that the range changes the two
// instants of the period and nothing else.
func TestRangeScenarioEqualsItsBase(t *testing.T) {
	ctx := context.Background()
	base := scenarioByName(t, "direct-complete")
	ranged := rangeScenario(t)

	tree := func(s promtest.Scenario) map[string]any {
		data, err := Snapshot(ctx, s)
		if err != nil {
			t.Fatalf("Snapshot(%s): %v", s.Name, err)
		}
		var out map[string]any
		if err := json.Unmarshal(data, &out); err != nil {
			t.Fatalf("Unmarshal: %v", err)
		}
		report := out["report"].(map[string]any)
		if report["period_start"] == nil || report["period_end"] == nil {
			t.Fatalf("%s holds no period instants", s.Name)
		}
		delete(report, "period_start")
		delete(report, "period_end")
		return out
	}
	a, _ := json.Marshal(tree(base))
	b, _ := json.Marshal(tree(ranged))
	if !bytes.Equal(a, b) {
		t.Error("the snapshots differ in a field other than report.period_start and report.period_end")
	}

	baseSnap, _ := Snapshot(ctx, base)
	rangeSnap, _ := Snapshot(ctx, ranged)
	if bytes.Equal(baseSnap, rangeSnap) {
		t.Error("the period instants did not change")
	}

	baseQueries, err := Queries(ctx, base)
	if err != nil {
		t.Fatalf("Queries(base): %v", err)
	}
	rangeQueries, err := Queries(ctx, ranged)
	if err != nil {
		t.Fatalf("Queries(range): %v", err)
	}
	if !bytes.Equal(baseQueries, rangeQueries) {
		t.Error("the query lists differ")
	}
}

// TestOriginalComparisonsWalkEight proves that the list the comparisons with
// the original walk holds eight scenarios and none of them is a range.
func TestOriginalComparisonsWalkEight(t *testing.T) {
	if len(promtest.Scenarios) != 8 {
		t.Fatalf("len(promtest.Scenarios) = %d, want 8", len(promtest.Scenarios))
	}
	for _, s := range promtest.Scenarios {
		if s.Start != "" || s.End != "" {
			t.Errorf("%s holds a range", s.Name)
		}
	}
}
