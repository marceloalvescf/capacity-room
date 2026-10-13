package prom

import (
	"context"
	"encoding/json"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// Point is one sample of a series of a range answer. Time is the
// timestamp as the endpoint wrote it, in seconds. Valid is false when the
// value was NaN or infinite.
type Point struct {
	Time  float64
	Value float64
	Valid bool
}

// Series is one series of a range answer.
type Series struct {
	Labels map[string]string
	Points []Point
}

// QueryRange asks a range query between start and end, at the given step.
func (c *Client) QueryRange(ctx context.Context, query string, start, end time.Time, step string) ([]Series, error) {
	answered := c.answered
	data, err := runQuery(ctx, c.sleep, queryRangeSend(c.httpClient, c.root, c.token, query, start, end, step), &answered, c.foreignTransport, c.maxResponseBytes)
	c.answered = answered
	if err != nil {
		return nil, err
	}
	return parseMatrix(data)
}

// queryRangeSend builds the send function for one range query attempt.
func queryRangeSend(httpClient *http.Client, root, token, query string, start, end time.Time, step string) func(context.Context) (*http.Response, error) {
	return func(ctx context.Context) (*http.Response, error) {
		req, err := newQueryRangeRequest(ctx, root, token, query, start, end, step)
		if err != nil {
			return nil, err
		}
		return httpClient.Do(req)
	}
}

// newQueryRangeRequest builds the POST /query_range request: form-encoded
// body with query, start, end, and step. start and end are whole seconds
// in decimal digits, as newQueryRequest writes time; step is sent as
// given.
func newQueryRangeRequest(ctx context.Context, root, token, query string, start, end time.Time, step string) (*http.Request, error) {
	form := url.Values{}
	form.Set("query", query)
	form.Set("start", strconv.FormatInt(start.Unix(), 10))
	form.Set("end", strconv.FormatInt(end.Unix(), 10))
	form.Set("step", step)
	return newFormRequest(ctx, root, "/query_range", token, form)
}

// parseMatrix turns the "data" object, undecoded, of a successful Prometheus
// answer into series. Only a "matrix" result is accepted; a "vector" result, or
// any other, is refused as unsupported.
func parseMatrix(data json.RawMessage) ([]Series, error) {
	resultTypeRaw, resultRaw, ok := resultOf(data)
	if !ok {
		return nil, unsupportedResultType(resultTypeRaw)
	}
	resultType, _ := resultTypeRaw.(string)
	if resultType != "matrix" {
		return nil, unsupportedResultType(resultTypeRaw)
	}

	series := []Series{}
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

		valuesRaw, hasValues := item["values"]
		if !hasValues {
			return nil, &Error{Kind: KindQuery, Message: "Prometheus matrix result has no values"}
		}
		valuesList, ok := valuesRaw.([]interface{})
		if !ok {
			return nil, &Error{Kind: KindQuery, Message: "Prometheus matrix result has no values"}
		}

		points := make([]Point, 0, len(valuesList))
		for _, pairRaw := range valuesList {
			point, err := parsePoint(pairRaw)
			if err != nil {
				return nil, err
			}
			points = append(points, point)
		}

		series = append(series, Series{Labels: labels, Points: points})
	}
	return series, nil
}

// parsePoint reads one [timestamp, encoded] pair of a matrix result's
// "values" list. Both members are read: the timestamp must be a JSON
// number or a JSON string strconv.ParseFloat can read, and encoded is a
// JSON string, read by strconv.ParseFloat, which reads "1", "NaN", and
// "+Inf" alike, or a JSON number. Valid is false for a non-finite value, which means "no
// data" rather than a failure.
func parsePoint(raw interface{}) (Point, error) {
	invalid := &Error{Kind: KindQuery, Message: "Prometheus response contains an invalid sample"}

	pair, ok := raw.([]interface{})
	if !ok || len(pair) != 2 {
		return Point{}, invalid
	}
	when, ok := numberValue(pair[0])
	if !ok {
		return Point{}, invalid
	}
	value, ok := sampleValue(pair[1])
	if !ok {
		return Point{}, invalid
	}
	return Point{Time: when, Value: value, Valid: !math.IsNaN(value) && !math.IsInf(value, 0)}, nil
}
