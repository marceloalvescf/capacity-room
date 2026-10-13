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
// and QueryRange do, and returns the error and the parsed result.
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
	long := `{"status":"success","data":{"resultType":"vector","result":[` +
		strings.Repeat(`{"metric":{"a":"b"},"value":[1,"1"]},`, 10000) + `{"metric":`
	cases := []struct {
		name, body, parser string
		kind               Kind
		message            string
	}{
		{"not JSON", `{"status":`, "vector", KindMetrics, notAPI},
		{"truncated tail of a long answer", long, "vector", KindMetrics, notAPI},
		{"array, not object", `[]`, "vector", KindMetrics, notAPI},
		{"no status", `{"data":{}}`, "vector", KindMetrics, notAPI},
		{"status error with errorType", `{"status":"error","errorType":"timeout"}`, "vector", KindQuery, "Prometheus query failed with type timeout"},
		{"status error without errorType", `{"status":"error"}`, "vector", KindQuery, "Prometheus query failed with type unknown"},
		{"status a number", `{"status":1,"data":{}}`, "vector", KindQuery, "Prometheus query failed with type unknown"},
		{"data null", `{"status":"success","data":null}`, "vector", KindQuery, noData},
		{"data an array", `{"status":"success","data":[]}`, "vector", KindQuery, noData},
		{"data absent", `{"status":"success"}`, "vector", KindQuery, noData},
		{"result null", `{"status":"success","data":{"resultType":"vector","result":null}}`, "vector", -1, ""},
		{"result absent", `{"status":"success","data":{"resultType":"vector"}}`, "vector", -1, ""},
		{"result an object", `{"status":"success","data":{"resultType":"vector","result":{}}}`, "vector", KindQuery, "unsupported Prometheus result type: vector"},
		{"resultType a number", `{"status":"success","data":{"resultType":3,"result":[]}}`, "vector", KindQuery, "unsupported Prometheus result type: 3"},
		{"matrix asked of a vector", `{"status":"success","data":{"resultType":"matrix","result":[]}}`, "vector", KindQuery, "unsupported Prometheus result type: matrix"},
		{"item not an object", `{"status":"success","data":{"resultType":"vector","result":[1]}}`, "vector", KindQuery, badItem},
		{"metric null", `{"status":"success","data":{"resultType":"vector","result":[{"metric":null,"value":[1,"2"]}]}}`, "vector", -2, "XX"},
		{"metric with a number label", `{"status":"success","data":{"resultType":"vector","result":[{"metric":{"a":1},"value":[1,"2"]}]}}`, "vector", KindQuery, "Prometheus response contains invalid labels"},
		{"repeated key, last wins", `{"status":"error","status":"success","data":{"resultType":"vector","result":[]}}`, "vector", -1, ""},
		{"repeated result, last wins", `{"status":"success","data":{"resultType":"vector","result":[1],"result":[]}}`, "vector", -1, ""},
		{"matrix without values", `{"status":"success","data":{"resultType":"matrix","result":[{"metric":{}}]}}`, "matrix", KindQuery, "Prometheus matrix result has no values"},
		{"matrix values null", `{"status":"success","data":{"resultType":"matrix","result":[{"metric":{},"values":null}]}}`, "matrix", KindQuery, "Prometheus matrix result has no values"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := answerOutcome(c.body, c.parser)
			if c.kind < 0 {
				if err != nil {
					t.Fatalf("want success, got %v", err)
				}
				if c.kind == -1 && !reflect.DeepEqual(got, []Sample{}) {
					t.Fatalf("want an empty result, got %#v", got)
				}
				if c.kind == -2 && !reflect.DeepEqual(got, []Sample{{Labels: map[string]string{}, Value: 2, Valid: true}}) {
					t.Fatalf("got %#v", got)
				}
				return
			}
			var e *Error
			if !errors.As(err, &e) {
				t.Logf("OUTCOME no error: %#v", got)
				t.Fatalf("want error %q, got %v", c.message, err)
			}
			if e.Kind != c.kind || e.Message != c.message {
				t.Logf("OUTCOME kind=%d message=%q", e.Kind, e.Message)
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
