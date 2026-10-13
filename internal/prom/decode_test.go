package prom

import (
	"errors"
	"math"
	"reflect"
	"runtime"
	"runtime/debug"
	"runtime/metrics"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// answerOutcome runs a body through decodeAPIResponse and a parser, as Query
// and QueryRange do, and returns the parsed result or the error.
func answerOutcome(body, parser string) (interface{}, error) {
	data, err := decodeAPIResponse([]byte(body), true)
	if err != nil {
		return nil, err
	}
	if parser == "matrix" {
		return parseMatrix(data)
	}
	return parseVector(data)
}

func TestAnswerShapesKeepTheirOutcome(t *testing.T) {
	const notAPI = "metrics endpoint did not answer the Prometheus API mid-collection"
	const noData = "Prometheus response has no data object"
	const badItem = "Prometheus response contains an invalid result item"
	const noValues = "Prometheus matrix result has no values"
	const badLabels = "Prometheus response contains invalid labels"
	long := `{"status":"success","data":{"resultType":"vector","result":[` +
		strings.Repeat(`{"metric":{"a":"b"},"value":[1,"1"]},`, 10000) + `{"metric":`
	vec := func(items string) string {
		return `{"status":"success","data":{"resultType":"vector","result":[` + items + `]}}`
	}
	cases := []struct {
		name, body, parser string
		kind               Kind   // the error kind, when an error is expected
		message            string // the error message, when an error is expected
		ok                 bool   // success is expected, with the result in want
		want               interface{}
	}{
		{name: "not JSON", body: `{"status":`, parser: "vector", kind: KindMetrics, message: notAPI},
		{name: "truncated tail of a long answer", body: long, parser: "vector", kind: KindMetrics, message: notAPI},
		{name: "array, not object", body: `[]`, parser: "vector", kind: KindMetrics, message: notAPI},
		{name: "no status", body: `{"data":{}}`, parser: "vector", kind: KindMetrics, message: notAPI},
		{name: "status error with errorType", body: `{"status":"error","errorType":"timeout"}`, parser: "vector", kind: KindQuery, message: "Prometheus query failed with type timeout"},
		{name: "status error without errorType", body: `{"status":"error"}`, parser: "vector", kind: KindQuery, message: "Prometheus query failed with type unknown"},
		{name: "status a number", body: `{"status":1,"data":{}}`, parser: "vector", kind: KindQuery, message: "Prometheus query failed with type unknown"},
		{name: "data null", body: `{"status":"success","data":null}`, parser: "vector", kind: KindQuery, message: noData},
		{name: "data an array", body: `{"status":"success","data":[]}`, parser: "vector", kind: KindQuery, message: noData},
		{name: "data absent", body: `{"status":"success"}`, parser: "vector", kind: KindQuery, message: noData},
		{name: "result null", body: `{"status":"success","data":{"resultType":"vector","result":null}}`, parser: "vector", ok: true, want: []Sample{}},
		{name: "result absent", body: `{"status":"success","data":{"resultType":"vector"}}`, parser: "vector", ok: true, want: []Sample{}},
		{name: "result an object", body: `{"status":"success","data":{"resultType":"vector","result":{}}}`, parser: "vector", kind: KindQuery, message: "unsupported Prometheus result type: vector"},
		{name: "resultType a number", body: `{"status":"success","data":{"resultType":3,"result":[]}}`, parser: "vector", kind: KindQuery, message: "unsupported Prometheus result type: 3"},
		{name: "matrix asked of a vector", body: `{"status":"success","data":{"resultType":"matrix","result":[]}}`, parser: "vector", kind: KindQuery, message: "unsupported Prometheus result type: matrix"},
		{name: "item not an object", body: vec(`1`), parser: "vector", kind: KindQuery, message: badItem},
		{name: "metric null", body: vec(`{"metric":null,"value":[1,"2"]}`), parser: "vector", ok: true, want: []Sample{{Labels: map[string]string{}, Value: 2, Valid: true}}},
		{name: "metric with a number label", body: vec(`{"metric":{"a":1},"value":[1,"2"]}`), parser: "vector", kind: KindQuery, message: badLabels},
		{name: "repeated key, last wins", body: `{"status":"error","status":"success","data":{"resultType":"vector","result":[]}}`, parser: "vector", ok: true, want: []Sample{}},
		{name: "repeated result, last wins", body: `{"status":"success","data":{"resultType":"vector","result":[1],"result":[]}}`, parser: "vector", ok: true, want: []Sample{}},
		{name: "matrix without values", body: `{"status":"success","data":{"resultType":"matrix","result":[{"metric":{}}]}}`, parser: "matrix", kind: KindQuery, message: noValues},
		{name: "matrix values null", body: `{"status":"success","data":{"resultType":"matrix","result":[{"metric":{},"values":null}]}}`, parser: "matrix", kind: KindQuery, message: noValues},

		{name: "overflow as a sample value", body: vec(`{"metric":{},"value":[1,1e999]}`), parser: "vector", kind: KindMetrics, message: notAPI},
		{name: "negative overflow as a sample value", body: vec(`{"metric":{},"value":[1,-1e999]}`), parser: "vector", kind: KindMetrics, message: notAPI},
		{name: "overflow as a timestamp", body: vec(`{"metric":{},"value":[1e999,"2"]}`), parser: "vector", kind: KindMetrics, message: notAPI},
		{name: "overflow as a label value", body: vec(`{"metric":{"a":1e999},"value":[1,"2"]}`), parser: "vector", kind: KindMetrics, message: notAPI},
		{name: "overflow as resultType", body: `{"status":"success","data":{"resultType":1e999,"result":[]}}`, parser: "vector", kind: KindMetrics, message: notAPI},
		{name: "overflow in an unused member", body: vec(`{"metric":{},"value":[1,"2"],"extra":1e999}`), parser: "vector", kind: KindMetrics, message: notAPI},
		{name: "overflow as status", body: `{"status":1e999,"data":{}}`, parser: "vector", kind: KindMetrics, message: notAPI},
		{name: "overflow in the second item after invalid labels", body: vec(`{"metric":{"a":1},"value":[1,"2"]},{"metric":{},"value":[1,1e999]}`), parser: "vector", kind: KindMetrics, message: notAPI},
		{name: "underflow as a sample value", body: vec(`{"metric":{},"value":[1,1e-999]}`), parser: "vector", ok: true, want: []Sample{{Labels: map[string]string{}, Value: 0, Valid: true}}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := answerOutcome(c.body, c.parser)
			if c.ok {
				if err != nil {
					t.Fatalf("want success, got %v", err)
				}
				if !reflect.DeepEqual(got, c.want) {
					t.Fatalf("got %#v, want %#v", got, c.want)
				}
				return
			}
			var e *Error
			if !errors.As(err, &e) {
				t.Fatalf("want error %q, got %v and %#v", c.message, err, got)
			}
			if e.Kind != c.kind || e.Message != c.message {
				t.Fatalf("got kind %d %q, want kind %d %q", e.Kind, e.Message, c.kind, c.message)
			}
		})
	}

	t.Run("matrix good", func(t *testing.T) {
		got, err := answerOutcome(`{"status":"success","data":{"resultType":"matrix","result":[{"metric":{"pod":"a"},"values":[[1,"1"],[2,"NaN"]]}]}}`, "matrix")
		if err != nil {
			t.Fatal(err)
		}
		series := got.([]Series)
		if len(series) != 1 || !reflect.DeepEqual(series[0].Labels, map[string]string{"pod": "a"}) || len(series[0].Points) != 2 {
			t.Fatalf("got %#v", series)
		}
		p := series[0].Points
		if !p[0].Valid || p[0].Value != 1 || p[1].Valid {
			t.Fatalf("got %#v", p)
		}
	})
	t.Run("vector good", func(t *testing.T) {
		got, err := answerOutcome(`{"status":"success","data":{"resultType":"vector","result":[{"metric":{"pod":"a"},"value":[1,"+Inf"]}]}}`, "vector")
		if err != nil {
			t.Fatal(err)
		}
		samples := got.([]Sample)
		if len(samples) != 1 || !reflect.DeepEqual(samples[0].Labels, map[string]string{"pod": "a"}) {
			t.Fatalf("got %#v", samples)
		}
		if samples[0].Valid || !math.IsInf(samples[0].Value, 1) {
			t.Fatalf("got %#v", samples[0])
		}
	})
}

// TestAnswerMemoryStaysBounded decodes a 32 MiB instant answer and samples
// the live heap while it does: the peak growth stays a small multiple of the
// body, where a whole-answer tree grew to about thirty times it.
func TestAnswerMemoryStaysBounded(t *testing.T) {
	var b strings.Builder
	b.WriteString(`{"status":"success","data":{"resultType":"vector","result":[`)
	for i := 0; b.Len() < 32<<20; i++ {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString(`{"metric":{"a":"b"},"value":[1,"1"]}`)
	}
	b.WriteString(`]}}`)
	body := []byte(b.String())

	defer debug.SetGCPercent(debug.SetGCPercent(10))
	runtime.GC()
	sample := []metrics.Sample{{Name: "/memory/classes/heap/objects:bytes"}}
	metrics.Read(sample)
	base := sample[0].Value.Uint64()

	var peak atomic.Uint64
	done := make(chan struct{})
	go func() {
		s := []metrics.Sample{{Name: "/memory/classes/heap/objects:bytes"}}
		for {
			select {
			case <-done:
				return
			default:
			}
			metrics.Read(s)
			if v := s[0].Value.Uint64(); v > peak.Load() {
				peak.Store(v)
			}
			time.Sleep(time.Millisecond)
		}
	}()
	data, err := decodeAPIResponse(body, true)
	var samples []Sample
	if err == nil {
		samples, err = parseVector(data)
	}
	close(done)
	if err != nil {
		t.Fatal(err)
	}
	runtime.GC()
	metrics.Read(sample)
	kept := sample[0].Value.Uint64()
	runtime.KeepAlive(samples)
	if kept >= peak.Load() {
		t.Fatalf("the sampler saw no point above what the result keeps (%d MiB); nothing was measured", kept>>20)
	}
	overhead := peak.Load() - kept
	t.Logf("body %d MiB, peak heap %d MiB above the start, %d MiB above what the result keeps, samples %d",
		len(body)>>20, (peak.Load()-base)>>20, overhead>>20, len(samples))
	if overhead > 6*uint64(len(body)) {
		t.Fatalf("decoding held %d MiB beyond its result for a body of %d MiB", overhead>>20, len(body)>>20)
	}
}
