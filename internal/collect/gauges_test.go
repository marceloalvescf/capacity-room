package collect

import (
	"math"
	"testing"

	"github.com/marceloalvescf/k8s-capacity-report/internal/prom"
)

func gaugeSeries(name string, points ...prom.Point) prom.Series {
	return prom.Series{Labels: map[string]string{"namespace": name}, Points: points}
}

func TestReduceMaxKeepsTheLargerSample(t *testing.T) {
	series := []prom.Series{
		gaugeSeries("a", prom.Point{Time: 1, Value: 2, Valid: true}),
		gaugeSeries("a", prom.Point{Time: 1, Value: 5, Valid: true}),
	}
	got := ReduceMax(series, []string{"namespace"})
	checkStatistics(t, got[Key{"a"}], Statistics{State: StateValue, Average: 5, Minimum: 5, Peak: 5, Current: 5})
}

func TestReduceMaxAverageIsCompensated(t *testing.T) {
	series := []prom.Series{gaugeSeries("a",
		prom.Point{Time: 1, Value: 3.3, Valid: true},
		prom.Point{Time: 2, Value: 1.1, Valid: true},
		prom.Point{Time: 3, Value: 2.2, Valid: true},
	)}
	got := ReduceMax(series, []string{"namespace"})
	if got[Key{"a"}].Average != 2.1999999999999997 {
		t.Errorf("Average = %v, want 2.1999999999999997", got[Key{"a"}].Average)
	}
}

func TestReduceMaxDropsAKeyOfInvalidSamples(t *testing.T) {
	series := []prom.Series{gaugeSeries("a", prom.Point{Time: 1, Value: math.NaN(), Valid: false})}
	if got := ReduceMax(series, []string{"namespace"}); len(got) != 0 {
		t.Errorf("got %v, want no key", got)
	}
}

func sample(name string, value float64, valid bool) prom.Sample {
	return prom.Sample{Labels: map[string]string{"namespace": name}, Value: value, Valid: valid}
}

func TestInstantValuesReadsOnePerKey(t *testing.T) {
	values, ambiguous := InstantValues([]prom.Sample{sample("a", 1, true), sample("b", 4, true)}, 10, []string{"namespace"})
	if len(values) != 2 || values[Key{"a"}] != 1 || values[Key{"b"}] != 4 {
		t.Errorf("values = %v, want a=1 b=4", values)
	}
	if len(ambiguous) != 0 {
		t.Errorf("ambiguous = %v, want none", ambiguous)
	}
}

func TestInstantValuesMarksAmbiguity(t *testing.T) {
	values, ambiguous := InstantValues([]prom.Sample{sample("a", 4, true), sample("a", 6, true)}, 10, []string{"namespace"})
	if _, ok := values[Key{"a"}]; ok {
		t.Errorf("values = %v, want a absent", values)
	}
	if !ambiguous[Key{"a"}] {
		t.Errorf("ambiguous = %v, want a present", ambiguous)
	}
}

func TestInstantValuesIgnoresEqualDuplicates(t *testing.T) {
	values, ambiguous := InstantValues([]prom.Sample{sample("a", 4, true), sample("a", 4, true)}, 10, []string{"namespace"})
	if values[Key{"a"}] != 4 || len(values) != 1 {
		t.Errorf("values = %v, want a=4", values)
	}
	if len(ambiguous) != 0 {
		t.Errorf("ambiguous = %v, want none", ambiguous)
	}
}

func TestInstantValuesSkipsInvalid(t *testing.T) {
	values, ambiguous := InstantValues([]prom.Sample{sample("a", math.NaN(), false), sample("a", 3, false)}, 10, []string{"namespace"})
	if values == nil || ambiguous == nil {
		t.Fatalf("values = %v, ambiguous = %v, want non-nil maps", values, ambiguous)
	}
	if len(values) != 0 || len(ambiguous) != 0 {
		t.Errorf("values = %v, ambiguous = %v, want both empty", values, ambiguous)
	}
}

func TestInstantValuesReadsAbsentLabelAsUnknown(t *testing.T) {
	values, _ := InstantValues([]prom.Sample{{Labels: map[string]string{}, Value: 2, Valid: true}}, 10, []string{"namespace"})
	if values[Key{"unknown"}] != 2 {
		t.Errorf("values = %v, want unknown=2", values)
	}
}

func TestPercent(t *testing.T) {
	known := func(v float64) Number { return Number{Value: v, Known: true} }
	cases := []struct {
		name string
		n, d Number
		want Number
	}{
		{"ratio", known(3.6), known(4), Number{Value: 90, Known: true}},
		{"zero denominator", known(1), known(0), Number{}},
		{"unknown denominator", known(1), Number{}, Number{}},
		{"unknown numerator", Number{}, known(1), Number{}},
		{"infinite numerator", known(math.Inf(1)), known(1), Number{}},
		{"negative denominator", known(1), known(-2), Number{}},
	}
	for _, c := range cases {
		if got := Percent(c.n, c.d); got != c.want {
			t.Errorf("%s: Percent = %v, want %v", c.name, got, c.want)
		}
	}
}

func TestReduceMaxKeepsTheFirstOfTwoZeros(t *testing.T) {
	negativeZero := math.Copysign(0, -1)
	cases := []struct {
		name        string
		first, last float64
		wantNegZero bool
	}{
		{"positive then negative", 0, negativeZero, false},
		{"negative then positive", negativeZero, 0, true},
	}
	for _, c := range cases {
		series := []prom.Series{
			gaugeSeries("a", prom.Point{Time: 1, Value: c.first, Valid: true}),
			gaugeSeries("a", prom.Point{Time: 1, Value: c.last, Valid: true}),
		}
		got := ReduceMax(series, []string{"namespace"})[Key{"a"}].Peak
		if got != 0 || math.Signbit(got) != c.wantNegZero {
			t.Errorf("%s: peak = %v with sign bit %v, want sign bit %v", c.name, got, math.Signbit(got), c.wantNegZero)
		}
	}
}
