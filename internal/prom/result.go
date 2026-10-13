package prom

import (
	"bytes"
	"encoding/json"
	"errors"
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

// parseVector turns the "data" object, undecoded, of a successful Prometheus
// answer into samples. Only a "vector" result is accepted; a "matrix" result, or
// any other, is refused as unsupported.
func parseVector(data json.RawMessage) ([]Sample, error) {
	resultTypeRaw, resultRaw, ok := resultOf(data)
	if !ok {
		return nil, unsupportedResultType(resultTypeRaw)
	}
	resultType, _ := resultTypeRaw.(string)
	if resultType != "vector" {
		return nil, unsupportedResultType(resultTypeRaw)
	}

	samples := []Sample{}
	dec := resultItems(resultRaw)
	for dec.More() {
		var itemRaw interface{}
		// decodeAPIResponse already refused every number float64 cannot hold,
		// so this decode of valid JSON cannot fail.
		_ = dec.Decode(&itemRaw)
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
// read by strconv.ParseFloat, which reads "1", "NaN", and "+Inf" alike, or
// a JSON number.
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
	value, ok = sampleValue(pair[1])
	if !ok {
		return 0, false, invalid
	}
	return value, !math.IsNaN(value) && !math.IsInf(value, 0), nil
}

// isReadableSampleNumber reports whether v is a JSON number (already a
// float64, however encoding/json decoded it) or a JSON string
// strconv.ParseFloat can read — the two shapes a sample's timestamp or
// value member is allowed to arrive in.
func isReadableSampleNumber(v interface{}) bool {
	_, ok := numberValue(v)
	return ok
}

// numberValue reads v as a JSON number (already a float64, however
// encoding/json decoded it) or a JSON string strconv.ParseFloat can read.
// A timestamp that is NaN or infinite is refused here: it is never a value
// a real Prometheus endpoint emits, and it cannot serve as the key it
// becomes once a caller groups points by it.
func numberValue(v interface{}) (float64, bool) {
	switch value := v.(type) {
	case float64:
		return value, isFinite(value)
	case string:
		f, err := strconv.ParseFloat(value, 64)
		return f, err == nil && isFinite(f)
	default:
		return 0, false
	}
}

// isFinite reports whether value is neither NaN nor infinite.
func isFinite(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0)
}

// sampleValue reads a sample's value member: a JSON string read by
// readEncodedValue, or a JSON number, already a float64, taken as it is, as
// the original's float() takes either.
func sampleValue(v interface{}) (float64, bool) {
	switch value := v.(type) {
	case string:
		return readEncodedValue(value)
	case float64:
		return value, true
	default:
		return 0, false
	}
}

// readEncodedValue reads the text of a sample's value member: a number
// strconv.ParseFloat can read. A value that is well formed but too large
// or too small for float64's range still parses, to an infinity or to
// zero, the same way the original's float() does with no exception; only
// a text that is not a number at all is refused.
func readEncodedValue(encoded string) (float64, bool) {
	value, err := strconv.ParseFloat(encoded, 64)
	if err == nil {
		return value, true
	}
	var numErr *strconv.NumError
	if errors.As(err, &numErr) && numErr.Err == strconv.ErrRange {
		return value, true
	}
	return 0, false
}

// resultOf reads the "resultType" and "result" members of a data object.
// result is the raw bytes of the array, nil when the member is absent or
// null; ok is false when it is present and not an array.
func resultOf(data json.RawMessage) (resultType interface{}, result json.RawMessage, ok bool) {
	var members map[string]json.RawMessage
	_ = json.Unmarshal(data, &members)
	// The data object is valid JSON whose numbers float64 holds, so these
	// decodes cannot fail.
	_ = json.Unmarshal(members["resultType"], &resultType)
	raw, present := members["result"]
	if !present || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return resultType, nil, true
	}
	if !bytes.HasPrefix(bytes.TrimSpace(raw), []byte("[")) {
		return resultType, nil, false
	}
	return resultType, raw, true
}

// resultItems returns a decoder positioned inside the array of a result, so
// the caller reads one element at a time; a nil result has no element.
func resultItems(result json.RawMessage) *json.Decoder {
	dec := json.NewDecoder(bytes.NewReader(result))
	if result != nil {
		// The array was validated with the whole body, so its opening
		// bracket is there.
		_, _ = dec.Token()
	}
	return dec
}
