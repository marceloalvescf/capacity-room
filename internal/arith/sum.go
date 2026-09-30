// Package arith holds the arithmetic whose result must equal, as the same
// float64, what the original program computes.
package arith

import "math"

// Sum adds values as CPython's builtin sum adds floats: Neumaier's
// compensated summation, the compensation added at the end unless it is
// not finite, so that an overflowed sum stays infinite.
func Sum(values []float64) float64 {
	f, c := 0.0, 0.0
	for _, x := range values {
		t := f + x
		if math.Abs(f) >= math.Abs(x) {
			c += (f - t) + x
		} else {
			c += (x - t) + f
		}
		f = t
	}
	if c != 0 && !math.IsInf(c, 0) && !math.IsNaN(c) {
		f += c
	}
	return f
}

// Mean returns Sum(values) divided by the count. It panics on an empty list;
// a caller checks for one first.
func Mean(values []float64) float64 {
	if len(values) == 0 {
		panic("arith.Mean: empty list")
	}
	return Sum(values) / float64(len(values))
}
