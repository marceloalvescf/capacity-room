package collect

import (
	"math"
	"sort"

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
// labels. A label a series lacks, and a place beyond the number of labels
// given, hold the empty text.
type Key [3]string

// Lookup returns the statistics of key, or a Statistics of StateUnavailable
// when statistics holds no entry for it.
func Lookup(statistics map[Key]Statistics, key Key) Statistics {
	if value, ok := statistics[key]; ok {
		return value
	}
	return Statistics{State: StateUnavailable}
}

// keyOf reads the values of labels from source, in order. A label source
// lacks reads as the empty text, the zero value Key already holds.
func keyOf(source map[string]string, labels []string) Key {
	var key Key
	for i, label := range labels {
		key[i] = source[label]
	}
	return key
}

// isFinite reports whether value is neither NaN nor infinite.
func isFinite(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0)
}

// samplesByTimestamp groups the valid points of series by key and
// timestamp, following _samples_by_timestamp of the original: the points
// of several series that share one key and one timestamp are added, in the
// order the series stand in series; a sum that ends up non-finite is left
// out; and a key left with no timestamp has no entry.
func samplesByTimestamp(series []prom.Series, labels []string) map[Key]map[float64]float64 {
	grouped := make(map[Key]map[float64]float64)
	for _, item := range series {
		key := keyOf(item.Labels, labels)
		for _, point := range item.Points {
			if !point.Valid {
				continue
			}
			values := grouped[key]
			if values == nil {
				values = make(map[float64]float64)
				grouped[key] = values
			}
			if existing, ok := values[point.Time]; ok {
				values[point.Time] = existing + point.Value
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

// statisticsOf reduces values, keyed by timestamp, to a Statistics,
// following _statistics of the original: the average is the sum of the
// values divided by their count, the values added in the order of their
// timestamps; the current value is the one of the largest timestamp.
func statisticsOf(values map[float64]float64) Statistics {
	timestamps := make([]float64, 0, len(values))
	for timestamp := range values {
		timestamps = append(timestamps, timestamp)
	}
	sort.Float64s(timestamps)

	sum := 0.0
	minimum := values[timestamps[0]]
	peak := minimum
	for _, timestamp := range timestamps {
		value := values[timestamp]
		sum += value
		if value < minimum {
			minimum = value
		}
		if value > peak {
			peak = value
		}
	}

	return Statistics{
		State:   StateValue,
		Average: sum / float64(len(timestamps)),
		Minimum: minimum,
		Peak:    peak,
		Current: values[timestamps[len(timestamps)-1]],
	}
}

// Reduce follows aggregate_statistics of the original: series is grouped
// by the values of labels and by timestamp with samplesByTimestamp, and
// each key left with a point is reduced with statisticsOf.
func Reduce(series []prom.Series, labels []string) map[Key]Statistics {
	grouped := samplesByTimestamp(series, labels)
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
	usageValues := samplesByTimestamp(usage, labels)
	baseValues := samplesByTimestamp(base, labels)

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
