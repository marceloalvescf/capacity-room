package collect

import (
	"math"
	"testing"

	"github.com/marceloalvescf/k8s-capacity-report/internal/prom"
)

// checkStatistics compares got against want field by field, naming the
// field that differs.
func checkStatistics(t *testing.T, got, want Statistics) {
	t.Helper()
	if got.State != want.State {
		t.Errorf("State = %q, want %q", got.State, want.State)
	}
	if got.Average != want.Average {
		t.Errorf("Average = %v, want %v", got.Average, want.Average)
	}
	if got.Minimum != want.Minimum {
		t.Errorf("Minimum = %v, want %v", got.Minimum, want.Minimum)
	}
	if got.Peak != want.Peak {
		t.Errorf("Peak = %v, want %v", got.Peak, want.Peak)
	}
	if got.Current != want.Current {
		t.Errorf("Current = %v, want %v", got.Current, want.Current)
	}
}

// TestReduceKeepsZeroAndSkipsNonFinite transcribes
// test_aggregate_statistics_keeps_zero_and_skips_non_finite_or_missing of
// tests/test_collector_common.py.
func TestReduceKeepsZeroAndSkipsNonFinite(t *testing.T) {
	series := []prom.Series{
		{
			Labels: map[string]string{"namespace": "payments"},
			Points: []prom.Point{
				{Time: 1, Valid: false},
				{Time: 5, Value: 2.0, Valid: true},
				{Time: 3, Value: math.NaN(), Valid: false},
				{Time: 4, Value: math.Inf(1), Valid: false},
				{Time: 2, Value: 0.0, Valid: true},
			},
		},
		{
			Labels: map[string]string{"namespace": "payments"},
			Points: []prom.Point{
				{Time: 5, Value: 3.0, Valid: true},
			},
		},
	}

	got := Reduce(series, []string{"namespace"})
	stats := Lookup(got, Key{"payments"})

	checkStatistics(t, stats, Statistics{
		State:   StateValue,
		Average: 2.5,
		Minimum: 0.0,
		Peak:    5.0,
		Current: 5.0,
	})
}

// TestRatiosAlignSamples transcribes
// test_aggregate_percentages_aligns_samples_instead_of_dividing_aggregates
// of tests/test_collector_common.py.
func TestRatiosAlignSamples(t *testing.T) {
	usage := []prom.Series{
		{
			Labels: map[string]string{"namespace": "payments"},
			Points: []prom.Point{
				{Time: 1, Value: 50, Valid: true},
				{Time: 2, Value: 100, Valid: true},
			},
		},
	}
	requests := []prom.Series{
		{
			Labels: map[string]string{"namespace": "payments"},
			Points: []prom.Point{
				{Time: 1, Value: 100, Valid: true},
				{Time: 2, Value: 1000, Valid: true},
			},
		},
	}

	ratios, approximate := Ratios(usage, requests, []string{"namespace"}, "available")
	stats := Lookup(ratios, Key{"payments"})

	if stats.Average != 30.0 {
		t.Errorf("Average = %v, want %v", stats.Average, 30.0)
	}
	if stats.Peak != 50.0 {
		t.Errorf("Peak = %v, want %v", stats.Peak, 50.0)
	}
	if stats.Current != 10.0 {
		t.Errorf("Current = %v, want %v", stats.Current, 10.0)
	}
	if len(approximate) != 0 {
		t.Errorf("approximate = %v, want empty", approximate)
	}
}

// TestRatiosFallBackToLatestBase transcribes
// test_aggregate_percentages_falls_back_to_latest_denominator of
// tests/test_collector_common.py.
func TestRatiosFallBackToLatestBase(t *testing.T) {
	usage := []prom.Series{
		{
			Labels: map[string]string{"namespace": "payments"},
			Points: []prom.Point{
				{Time: 1, Value: 50, Valid: true},
				{Time: 2, Value: 100, Valid: true},
			},
		},
	}
	requests := []prom.Series{
		{
			Labels: map[string]string{"namespace": "payments"},
			Points: []prom.Point{
				{Time: 1, Value: 100, Valid: true},
			},
		},
	}

	ratios, approximate := Ratios(usage, requests, []string{"namespace"}, "available")
	stats := Lookup(ratios, Key{"payments"})

	if stats.Current != 100.0 {
		t.Errorf("Current = %v, want %v", stats.Current, 100.0)
	}
	if !approximate[Key{"payments"}] {
		t.Errorf("approximate[%v] = false, want true", Key{"payments"})
	}
	if len(approximate) != 1 {
		t.Errorf("approximate = %v, want exactly one key", approximate)
	}
}

// TestRatiosZeroBaseIsNoBase transcribes
// test_zero_denominator_is_reported_as_missing_base_not_as_unavailable of
// tests/test_collector_common.py.
func TestRatiosZeroBaseIsNoBase(t *testing.T) {
	usage := []prom.Series{
		{
			Labels: map[string]string{"namespace": "payments"},
			Points: []prom.Point{
				{Time: 1, Value: 50, Valid: true},
				{Time: 2, Value: 100, Valid: true},
			},
		},
	}
	requests := []prom.Series{
		{
			Labels: map[string]string{"namespace": "payments"},
			Points: []prom.Point{
				{Time: 1, Value: 0, Valid: true},
				{Time: 2, Value: 0, Valid: true},
			},
		},
	}

	ratios, _ := Ratios(usage, requests, []string{"namespace"}, "available")
	stats := Lookup(ratios, Key{"payments"})

	if stats.State != StateNoBase {
		t.Errorf("State = %q, want %q", stats.State, StateNoBase)
	}
}

// TestRatiosAbsentBaseFollowsTheState transcribes
// test_absent_denominator_is_missing_base_when_capability_is_available and
// test_absent_denominator_stays_unavailable_when_capability_is_ambiguous of
// tests/test_collector_common.py.
func TestRatiosAbsentBaseFollowsTheState(t *testing.T) {
	usage := []prom.Series{
		{
			Labels: map[string]string{"namespace": "payments"},
			Points: []prom.Point{
				{Time: 1, Value: 50, Valid: true},
			},
		},
	}

	tests := []struct {
		baseState string
		wantState string
		wantEntry bool
	}{
		{"available", StateNoBase, true},
		{"partial", "", false},
		{"unavailable", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.baseState, func(t *testing.T) {
			ratios, _ := Ratios(usage, nil, []string{"namespace"}, tt.baseState)
			stats, ok := ratios[Key{"payments"}]
			if ok != tt.wantEntry {
				t.Fatalf("ratios has entry = %v, want %v", ok, tt.wantEntry)
			}
			if tt.wantEntry && stats.State != tt.wantState {
				t.Errorf("State = %q, want %q", stats.State, tt.wantState)
			}
		})
	}
}

// TestReduceAddsSeriesOfOneKey covers two series of one key, values 1 and 2
// at one timestamp, reduced to a single point of 3.
func TestReduceAddsSeriesOfOneKey(t *testing.T) {
	series := []prom.Series{
		{
			Labels: map[string]string{"namespace": "payments"},
			Points: []prom.Point{{Time: 10, Value: 1, Valid: true}},
		},
		{
			Labels: map[string]string{"namespace": "payments"},
			Points: []prom.Point{{Time: 10, Value: 2, Valid: true}},
		},
	}

	got := Reduce(series, []string{"namespace"})
	stats := Lookup(got, Key{"payments"})

	checkStatistics(t, stats, Statistics{
		State:   StateValue,
		Average: 3,
		Minimum: 3,
		Peak:    3,
		Current: 3,
	})
}

// TestReduceDropsAKeyOfNaNOnly covers a series whose every point is not
// valid: the key has no entry.
func TestReduceDropsAKeyOfNaNOnly(t *testing.T) {
	series := []prom.Series{
		{
			Labels: map[string]string{"namespace": "payments"},
			Points: []prom.Point{
				{Time: 1, Valid: false},
				{Time: 2, Valid: false},
			},
		},
	}

	got := Reduce(series, []string{"namespace"})
	if _, ok := got[Key{"payments"}]; ok {
		t.Errorf("Reduce has an entry for %v, want none", Key{"payments"})
	}
}

// TestReduceCurrentIsTheLastTimestamp covers points given out of order of
// time: the current value is the one of the largest timestamp, and the
// average counts each point once, whatever order they arrived in.
func TestReduceCurrentIsTheLastTimestamp(t *testing.T) {
	series := []prom.Series{
		{
			Labels: map[string]string{"namespace": "payments"},
			Points: []prom.Point{
				{Time: 5, Value: 100, Valid: true},
				{Time: 1, Value: 10, Valid: true},
				{Time: 3, Value: 30, Valid: true},
			},
		},
	}

	got := Reduce(series, []string{"namespace"})
	stats := Lookup(got, Key{"payments"})

	wantAverage := (100.0 + 10.0 + 30.0) / 3.0
	checkStatistics(t, stats, Statistics{
		State:   StateValue,
		Average: wantAverage,
		Minimum: 10,
		Peak:    100,
		Current: 100,
	})
}

// TestReduceUsesThreeLabels covers labels namespace, workload, and
// workload_type: the key holds all three values.
func TestReduceUsesThreeLabels(t *testing.T) {
	series := []prom.Series{
		{
			Labels: map[string]string{
				"namespace":     "ns1",
				"workload":      "api",
				"workload_type": "Deployment",
			},
			Points: []prom.Point{{Time: 1, Value: 1, Valid: true}},
		},
	}

	got := Reduce(series, []string{"namespace", "workload", "workload_type"})
	if _, ok := got[Key{"ns1", "api", "Deployment"}]; !ok {
		t.Errorf("Reduce has no entry for %v", Key{"ns1", "api", "Deployment"})
	}
}

// TestRatiosNegativeBaseGivesNoRatio covers a base of -1: no ratio for
// that timestamp, and no entry at all since it is the only timestamp.
func TestRatiosNegativeBaseGivesNoRatio(t *testing.T) {
	usage := []prom.Series{
		{
			Labels: map[string]string{"namespace": "payments"},
			Points: []prom.Point{{Time: 1, Value: 50, Valid: true}},
		},
	}
	requests := []prom.Series{
		{
			Labels: map[string]string{"namespace": "payments"},
			Points: []prom.Point{{Time: 1, Value: -1, Valid: true}},
		},
	}

	ratios, approximate := Ratios(usage, requests, []string{"namespace"}, "available")
	if _, ok := ratios[Key{"payments"}]; ok {
		t.Errorf("ratios has an entry for %v, want none", Key{"payments"})
	}
	if len(approximate) != 0 {
		t.Errorf("approximate = %v, want empty", approximate)
	}
}

// TestRatiosScenarioNumbers covers usage 0.2, 0.4 against base 0.5, 0.5 at
// the same two timestamps: the ratios are computed sample by sample, not
// as the ratio of the sums.
func TestRatiosScenarioNumbers(t *testing.T) {
	usage := []prom.Series{
		{
			Labels: map[string]string{"namespace": "payments"},
			Points: []prom.Point{
				{Time: 1, Value: 0.2, Valid: true},
				{Time: 2, Value: 0.4, Valid: true},
			},
		},
	}
	requests := []prom.Series{
		{
			Labels: map[string]string{"namespace": "payments"},
			Points: []prom.Point{
				{Time: 1, Value: 0.5, Valid: true},
				{Time: 2, Value: 0.5, Valid: true},
			},
		},
	}

	ratio1 := 0.2 / 0.5 * 100.0
	ratio2 := 0.4 / 0.5 * 100.0
	wantMinimum := math.Min(ratio1, ratio2)
	wantPeak := math.Max(ratio1, ratio2)
	wantAverage := (ratio1 + ratio2) / 2.0

	ratios, approximate := Ratios(usage, requests, []string{"namespace"}, "available")
	stats := Lookup(ratios, Key{"payments"})

	if wantPeak != 80.0 {
		t.Fatalf("test setup: wantPeak = %v, want %v", wantPeak, 80.0)
	}
	checkStatistics(t, stats, Statistics{
		State:   StateValue,
		Average: wantAverage,
		Minimum: wantMinimum,
		Peak:    wantPeak,
		Current: ratio2,
	})
	if approximate[Key{"payments"}] {
		t.Errorf("approximate[%v] = true, want false", Key{"payments"})
	}
}
