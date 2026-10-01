package collect

import (
	"github.com/marceloalvescf/k8s-capacity-report/internal/prom"
)

// Number is a value that may be unknown.
type Number struct {
	Value float64
	Known bool
}

// Flag is a boolean that may be unknown.
type Flag struct {
	Value bool
	Known bool
}

// ReduceMax is Reduce with the maximum in place of the sum where two series
// share a key and a timestamp. It is the gauge aggregation of the original,
// aggregate_gauge_statistics.
func ReduceMax(series []prom.Series, labels []string) map[Key]Statistics {
	return reduceWith(series, labels, func(a, b float64) float64 {
		if b > a {
			return b
		}
		return a
	})
}

// InstantValues reads the value of each key from instant samples, following
// _instant_values of the original. An instant answer keeps no timestamp, so
// every sample stands at the same instant and the latest timestamp of a key
// is that instant. A key with more than one distinct value there is
// ambiguous: it is absent from values and present in ambiguous. A sample
// that is not valid is skipped.
func InstantValues(samples []prom.Sample, _ float64, labels []string) (values map[Key]float64, ambiguous map[Key]bool) {
	distinct := make(map[Key]map[float64]bool)
	for _, sample := range samples {
		if !sample.Valid || !isFinite(sample.Value) {
			continue
		}
		key := keyOf(sample.Labels, labels)
		if distinct[key] == nil {
			distinct[key] = make(map[float64]bool)
		}
		distinct[key][sample.Value] = true
	}
	values = make(map[Key]float64)
	ambiguous = make(map[Key]bool)
	for key, set := range distinct {
		if len(set) > 1 {
			ambiguous[key] = true
			continue
		}
		for value := range set {
			values[key] = value
		}
	}
	return values, ambiguous
}

// Percent returns numerator over denominator times 100, evaluated in that
// order, and an unknown Number when either is unknown or not finite or the
// denominator is not above zero.
func Percent(numerator, denominator Number) Number {
	if !numerator.Known || !denominator.Known {
		return Number{}
	}
	value, ok := percent(numerator.Value, denominator.Value)
	if !ok {
		return Number{}
	}
	return Number{Value: value, Known: true}
}
