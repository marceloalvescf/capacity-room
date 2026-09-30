package arith

import (
	"math"
	"strings"
	"testing"
)

func tenTenths() []float64 {
	values := make([]float64, 10)
	for i := range values {
		values[i] = 0.1
	}
	return values
}

func TestSumMatchesPython(t *testing.T) {
	tests := []struct {
		name   string
		values []float64
		want   float64
	}{
		{"three decimals", []float64{3.3, 1.1, 2.2}, 6.6},
		{"cancelling", []float64{1e16, 1, -1e16}, 1},
		{"ten tenths", tenTenths(), 1},
		{"small tail", []float64{1, 1e-16, 1e-16, 1e-16}, 1.0000000000000002},
		{"overflow", []float64{1e308, 1e308, -1e308}, math.Inf(1)},
		{"empty", []float64{}, 0},
	}
	for _, test := range tests {
		if got := Sum(test.values); got != test.want {
			t.Errorf("%s: Sum = %v, want %v", test.name, got, test.want)
		}
	}
}

func TestMeanMatchesPython(t *testing.T) {
	tests := []struct {
		name   string
		values []float64
		want   float64
	}{
		{"three decimals", []float64{3.3, 1.1, 2.2}, 2.1999999999999997},
		{"cancelling", []float64{1e16, 1, -1e16}, 0.3333333333333333},
		{"ten tenths", tenTenths(), 0.1},
		{"small tail", []float64{1, 1e-16, 1e-16, 1e-16}, 0.25000000000000006},
	}
	for _, test := range tests {
		if got := Mean(test.values); got != test.want {
			t.Errorf("%s: Mean = %v, want %v", test.name, got, test.want)
		}
	}
}

func TestMeanOfNothingPanics(t *testing.T) {
	defer func() {
		got := recover()
		message, ok := got.(string)
		if !ok || !strings.Contains(message, "Mean") {
			t.Fatalf("panic = %v, want a message holding Mean", got)
		}
	}()
	Mean(nil)
}

func TestSumDiffersFromAPlainLoop(t *testing.T) {
	values := []float64{3.3, 1.1, 2.2}
	plain := 0.0
	for _, value := range values {
		plain += value
	}
	if Sum(values) == plain {
		t.Fatalf("Sum and a plain loop agree on %v", plain)
	}
}
