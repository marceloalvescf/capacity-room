package collect

import (
	"math"
	"sort"

	"github.com/marceloalvescf/k8s-capacity-report/internal/arith"
	"github.com/marceloalvescf/k8s-capacity-report/internal/prom"
)

// The states a Statistics can hold.
const (
	StateValue       = "value"
	StateUnavailable = "unavailable"
	StateNoBase      = "no_base"
)

// Statistics is a series reduced to four numbers. The numbers mean
// something only when State is StateValue.
type Statistics struct {
	State   string
	Average float64
	Minimum float64
	Peak    float64
	Current float64
}

// Key is the values of the grouping labels of one row, in the order of the
// labels. A label a series does not have at all reads as unknownLabel; a
// label the series has with the empty text as its value reads as the
// empty text. A place beyond the number of labels given holds the empty
// text.
type Key [3]string

// unknownLabel is UNKNOWN_LABEL of the original: the default of
// PromSeries.label(name, UNKNOWN_LABEL), used only when the series has no
// value for the label at all.
const unknownLabel = "unknown"

// Lookup returns the statistics of key, or a Statistics of StateUnavailable
// when statistics holds no entry for it.
func Lookup(statistics map[Key]Statistics, key Key) Statistics {
	if value, ok := statistics[key]; ok {
		return value
	}
	return Statistics{State: StateUnavailable}
}

// keyOf reads the values of labels from source, in order, as dict.get(name,
// UNKNOWN_LABEL) reads them in the original: a label source has no entry
// for reads as unknownLabel, and a label source holds with the empty text
// stays the empty text.
func keyOf(source map[string]string, labels []string) Key {
	var key Key
	for i, label := range labels {
		if value, ok := source[label]; ok {
			key[i] = value
		} else {
			key[i] = unknownLabel
		}
	}
	return key
}

// isFinite reports whether value is neither NaN nor infinite.
func isFinite(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0)
}

// samplesByTimestamp groups the valid points of series by key and
// timestamp, following _samples_by_timestamp of the original: the points
// of several series that share one key and one timestamp are combined with
// combine, in the order the series stand in series; a combined value that
// ends up non-finite is left out; and a key left with no timestamp has no entry. A point whose
// timestamp is NaN or infinite is left out as well: the client itself now
// refuses such a timestamp before it reaches this package, but a caller of
// this package can still build a Point by hand with one, and a NaN
// timestamp can never be found again once used as a map key.
func samplesByTimestamp(series []prom.Series, labels []string, combine func(a, b float64) float64) map[Key]map[float64]float64 {
	grouped := make(map[Key]map[float64]float64)
	for _, item := range series {
		key := keyOf(item.Labels, labels)
		for _, point := range item.Points {
			if !point.Valid || !isFinite(point.Time) {
				continue
			}
			values := grouped[key]
			if values == nil {
				values = make(map[float64]float64)
				grouped[key] = values
			}
			if existing, ok := values[point.Time]; ok {
				values[point.Time] = combine(existing, point.Value)
			} else {
				values[point.Time] = point.Value
			}
		}
	}
	for key, values := range grouped {
		for timestamp, value := range values {
			if !isFinite(value) {
				delete(values, timestamp)
			}
		}
		if len(values) == 0 {
			delete(grouped, key)
		}
	}
	return grouped
}

// timestamped carries one value together with the timestamp it was read
// at, so that sorting by timestamp never needs a later, separate lookup by
// that same timestamp to recover the value.
type timestamped struct {
	timestamp float64
	value     float64
}

// statisticsOf reduces values, keyed by timestamp, to a Statistics,
// following _statistics of the original: the average is the compensated
// sum of the values divided by their count, the values added in the order of
// their timestamps; the current value is the one of the largest timestamp. Each
// value is carried through the sort together with its timestamp, and never
// looked up again by it: a map lookup keyed by a timestamp read back out of
// the same map is exactly what let a NaN timestamp stand for a real value
// and return zero instead (a map key never equals itself under NaN).
func statisticsOf(values map[float64]float64) Statistics {
	pairs := make([]timestamped, 0, len(values))
	for timestamp, value := range values {
		pairs = append(pairs, timestamped{timestamp: timestamp, value: value})
	}
	sort.Slice(pairs, func(i, j int) bool { return pairs[i].timestamp < pairs[j].timestamp })

	sorted := make([]float64, len(pairs))
	minimum := pairs[0].value
	peak := minimum
	for i, pair := range pairs {
		sorted[i] = pair.value
		if pair.value < minimum {
			minimum = pair.value
		}
		if pair.value > peak {
			peak = pair.value
		}
	}

	return Statistics{
		State:   StateValue,
		Average: arith.Mean(sorted),
		Minimum: minimum,
		Peak:    peak,
		Current: pairs[len(pairs)-1].value,
	}
}

// Reduce follows aggregate_statistics of the original: series is grouped
// by the values of labels and by timestamp with samplesByTimestamp, and
// each key left with a point is reduced with statisticsOf.
func Reduce(series []prom.Series, labels []string) map[Key]Statistics {
	return reduceWith(series, labels, add)
}

// add is the combination of Reduce.
func add(a, b float64) float64 { return a + b }

// reduceWith groups series with samplesByTimestamp, combining points of one
// key and timestamp with combine, and reduces each key with statisticsOf.
func reduceWith(series []prom.Series, labels []string, combine func(a, b float64) float64) map[Key]Statistics {
	grouped := samplesByTimestamp(series, labels, combine)
	result := make(map[Key]Statistics, len(grouped))
	for key, values := range grouped {
		result[key] = statisticsOf(values)
	}
	return result
}

// percent divides numerator by denominator and scales the result to a
// percentage. A denominator that is absent, zero, or negative gives no
// ratio, and neither does a numerator or denominator that is not finite.
func percent(numerator, denominator float64) (float64, bool) {
	if !isFinite(numerator) || !isFinite(denominator) || denominator <= 0 {
		return 0, false
	}
	return numerator / denominator * 100.0, true
}

// latestOf returns the value of the largest timestamp of values, and
// whether values held any timestamp at all.
func latestOf(values map[float64]float64) (float64, bool) {
	var latestTimestamp, latestValue float64
	found := false
	for timestamp, value := range values {
		if !found || timestamp > latestTimestamp {
			latestTimestamp = timestamp
			latestValue = value
			found = true
		}
	}
	return latestValue, found
}

// Ratios follows aggregate_percentages and percent of the original, line
// by line: usage and base are each grouped by key and timestamp with
// samplesByTimestamp; for each key of usage, the base of a timestamp the
// base lacks falls back to the base of its latest timestamp, and marks the
// key as approximate; a key left with no ratio at all reads as
// StateNoBase when the base is zero at some timestamp of a finite usage,
// whatever baseState is, or when the key has usage, the base has no point
// for it, and baseState is "available"; otherwise the key has no entry.
func Ratios(usage, base []prom.Series, labels []string, baseState string) (map[Key]Statistics, map[Key]bool) {
	usageValues := samplesByTimestamp(usage, labels, add)
	baseValues := samplesByTimestamp(base, labels, add)

	ratios := make(map[Key]Statistics)
	approximate := make(map[Key]bool)

	for key, values := range usageValues {
		availableBase := baseValues[key]
		latestBase, hasLatestBase := latestOf(availableBase)

		noBase := baseState == "available" && len(values) > 0 && len(availableBase) == 0

		percentages := make(map[float64]float64)
		for timestamp, value := range values {
			baseValue, exact := availableBase[timestamp]
			fallback := !exact
			denominator := baseValue
			hasDenominator := exact
			if !exact && hasLatestBase {
				denominator = latestBase
				hasDenominator = true
			}
			if !hasDenominator {
				continue
			}

			ratio, ok := percent(value, denominator)
			if !ok {
				if denominator == 0 && isFinite(value) {
					noBase = true
				}
				continue
			}
			percentages[timestamp] = ratio
			if fallback {
				approximate[key] = true
			}
		}

		if len(percentages) > 0 {
			ratios[key] = statisticsOf(percentages)
		} else if noBase {
			ratios[key] = Statistics{State: StateNoBase}
		}
	}
	return ratios, approximate
}
