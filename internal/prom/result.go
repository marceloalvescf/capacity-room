package prom

import (
	"fmt"
	"math"
	"strconv"
)

// Sample is one series of an instant answer. Valid is false when the value
// was NaN or infinite, which means "no data".
type Sample struct {
	Labels map[string]string
	Value  float64
	Valid  bool
}

// parseVector turns the "data" object of a successful Prometheus answer
// into samples. Only a "vector" result is accepted; a matrix result is
// refused as unsupported, because nothing in this package issues a range
// query yet.
func parseVector(data map[string]interface{}) ([]Sample, error) {
	resultTypeRaw := data["resultType"]
	resultType, _ := resultTypeRaw.(string)

	resultRaw := data["result"]
	var resultList []interface{}
	if resultRaw != nil {
		list, ok := resultRaw.([]interface{})
		if !ok {
			return nil, unsupportedResultType(resultTypeRaw)
		}
		resultList = list
	}
	if resultType != "vector" {
		return nil, unsupportedResultType(resultTypeRaw)
	}

	samples := make([]Sample, 0, len(resultList))
	for _, itemRaw := range resultList {
		item, ok := itemRaw.(map[string]interface{})
		if !ok {
			return nil, &Error{Kind: KindQuery, Message: "Prometheus response contains an invalid result item"}
		}

		labels, err := parseLabels(item["metric"])
		if err != nil {
			return nil, err
		}

		valueRaw, hasValue := item["value"]
		if !hasValue {
			return nil, &Error{Kind: KindQuery, Message: "Prometheus vector result has no value"}
		}
		value, valid, err := parseSampleValue(valueRaw)
		if err != nil {
			return nil, err
		}

		samples = append(samples, Sample{Labels: labels, Value: value, Valid: valid})
	}
	return samples, nil
}

// unsupportedResultType builds the fixed message for a resultType Prometheus
// answered that this client does not accept.
func unsupportedResultType(resultType interface{}) error {
	return &Error{Kind: KindQuery, Message: fmt.Sprintf("unsupported Prometheus result type: %s", resultTypeText(resultType))}
}

// resultTypeText renders a resultType value for the unsupported-type
// message, whatever JSON shape it arrived in.
func resultTypeText(v interface{}) string {
	if s, ok := v.(string); ok {
		return s
	}
	return fmt.Sprintf("%v", v)
}

// parseLabels reads the "metric" object of a result item: an object whose
// values are all strings, or absent (which means no labels).
func parseLabels(raw interface{}) (map[string]string, error) {
	if raw == nil {
		return map[string]string{}, nil
	}
	obj, ok := raw.(map[string]interface{})
	if !ok {
		return nil, &Error{Kind: KindQuery, Message: "Prometheus response contains invalid labels"}
	}
	labels := make(map[string]string, len(obj))
	for key, value := range obj {
		s, ok := value.(string)
		if !ok {
			return nil, &Error{Kind: KindQuery, Message: "Prometheus response contains invalid labels"}
		}
		labels[key] = s
	}
	return labels, nil
}

// parseSampleValue reads a vector result's "value" pair: [timestamp,
// encoded]. Both members are read: the timestamp must be a JSON number or
// a JSON string strconv.ParseFloat can read, and encoded is a JSON string,
// read by strconv.ParseFloat, which reads "1", "NaN", and "+Inf" alike.
// valid is false for a non-finite value, which means "no data" rather than
// a failure. The parsed timestamp itself is not returned: Sample carries no
// timestamp field, since an instant query has only the one moment it was
// asked for.
func parseSampleValue(raw interface{}) (value float64, valid bool, err error) {
	invalid := &Error{Kind: KindQuery, Message: "Prometheus response contains an invalid sample"}

	pair, ok := raw.([]interface{})
	if !ok || len(pair) != 2 {
		return 0, false, invalid
	}
	if !isReadableSampleNumber(pair[0]) {
		return 0, false, invalid
	}
	encoded, ok := pair[1].(string)
	if !ok {
		return 0, false, invalid
	}
	value, err = strconv.ParseFloat(encoded, 64)
	if err != nil {
		return 0, false, invalid
	}
	return value, !math.IsNaN(value) && !math.IsInf(value, 0), nil
}

// isReadableSampleNumber reports whether v is a JSON number (already a
// float64, however encoding/json decoded it) or a JSON string
// strconv.ParseFloat can read — the two shapes a sample's timestamp or
// value member is allowed to arrive in.
func isReadableSampleNumber(v interface{}) bool {
	switch value := v.(type) {
	case float64:
		return true
	case string:
		_, err := strconv.ParseFloat(value, 64)
		return err == nil
	default:
		return false
	}
}
