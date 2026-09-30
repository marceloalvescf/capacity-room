package prom

import (
	"context"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sync/atomic"
	"testing"
	"time"
)

// connectedClient starts a server that answers the probe with an empty
// success vector, then whatever bodyFn returns for every later request,
// and returns a Client already bound to it.
func connectedClient(t *testing.T, bodyFn func(n int32) (int, string)) *Client {
	t.Helper()
	var calls int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		n := atomic.AddInt32(&calls, 1)
		w.Header().Set("Content-Type", "application/json")
		if n == 1 {
			_, _ = w.Write([]byte(emptySuccessVector))
			return
		}
		status, body := bodyFn(n)
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(server.Close)

	sleep, _ := recordingSleep()
	opts := baseOptions(server.URL + "/api/v1")
	opts.Sleep = sleep
	client, err := Connect(context.Background(), opts)
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	return client
}

func TestQueryReadsSamples(t *testing.T) {
	body := `{"status":"success","data":{"resultType":"vector","result":[
		{"metric":{"cluster":"prod"},"value":[1000,"2"]},
		{"metric":{"cluster":"dev"},"value":[1000,"3"]}
	]}}`
	client := connectedClient(t, func(int32) (int, string) { return http.StatusOK, body })

	samples, err := client.Query(context.Background(), "up", time.Unix(1000, 0))
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(samples) != 2 {
		t.Fatalf("len(samples) = %d, want 2", len(samples))
	}

	want := []Sample{
		{Labels: map[string]string{"cluster": "prod"}, Value: 2, Valid: true},
		{Labels: map[string]string{"cluster": "dev"}, Value: 3, Valid: true},
	}
	if !reflect.DeepEqual(samples, want) {
		t.Errorf("samples = %#v, want %#v", samples, want)
	}
}

func TestQueryTreatsNonFiniteAsNoData(t *testing.T) {
	cases := []string{"NaN", "+Inf", "-Inf", "Inf"}

	for _, encoded := range cases {
		t.Run(encoded, func(t *testing.T) {
			body := fmt.Sprintf(`{"status":"success","data":{"resultType":"vector","result":[{"metric":{},"value":[1000,%q]}]}}`, encoded)
			client := connectedClient(t, func(int32) (int, string) { return http.StatusOK, body })

			samples, err := client.Query(context.Background(), "up", time.Unix(1000, 0))
			if err != nil {
				t.Fatalf("Query: %v", err)
			}
			if len(samples) != 1 {
				t.Fatalf("len(samples) = %d, want 1", len(samples))
			}
			if samples[0].Valid {
				t.Errorf("Valid = true for %q, want false", encoded)
			}
		})
	}
}

func TestQueryKeepsMeasuredZero(t *testing.T) {
	body := `{"status":"success","data":{"resultType":"vector","result":[{"metric":{},"value":[1000,"0"]}]}}`
	client := connectedClient(t, func(int32) (int, string) { return http.StatusOK, body })

	samples, err := client.Query(context.Background(), "up", time.Unix(1000, 0))
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(samples) != 1 {
		t.Fatalf("len(samples) = %d, want 1", len(samples))
	}
	if !samples[0].Valid {
		t.Error("Valid = false, want true")
	}
	if samples[0].Value != 0 {
		t.Errorf("Value = %v, want 0", samples[0].Value)
	}
	if math.Signbit(samples[0].Value) {
		t.Error("Value is negative zero, want positive")
	}
}

// TestQueryFailures covers the table of failures from the "status" row
// down: the failures that can only be seen once a query has actually
// parsed a JSON body, whether during the probe or a later query.
func TestQueryFailures(t *testing.T) {
	cases := []struct {
		name     string
		body     string
		wantKind Kind
		wantMsg  string
	}{
		{
			name:     "status is not success",
			body:     `{"status":"error","errorType":"bad_data"}`,
			wantKind: KindQuery,
			wantMsg:  "Prometheus query failed with type bad_data",
		},
		{
			name:     "status is not success, no errorType",
			body:     `{"status":"error"}`,
			wantKind: KindQuery,
			wantMsg:  "Prometheus query failed with type unknown",
		},
		{
			name:     "no data object",
			body:     `{"status":"success"}`,
			wantKind: KindQuery,
			wantMsg:  "Prometheus response has no data object",
		},
		{
			name:     "resultType is not vector",
			body:     `{"status":"success","data":{"resultType":"matrix","result":[]}}`,
			wantKind: KindQuery,
			wantMsg:  "unsupported Prometheus result type: matrix",
		},
		{
			name:     "result item is not an object",
			body:     `{"status":"success","data":{"resultType":"vector","result":["x"]}}`,
			wantKind: KindQuery,
			wantMsg:  "Prometheus response contains an invalid result item",
		},
		{
			name:     "metric is not an object of strings",
			body:     `{"status":"success","data":{"resultType":"vector","result":[{"metric":"x","value":[1,"2"]}]}}`,
			wantKind: KindQuery,
			wantMsg:  "Prometheus response contains invalid labels",
		},
		{
			name:     "metric has a non-string value",
			body:     `{"status":"success","data":{"resultType":"vector","result":[{"metric":{"cluster":1},"value":[1,"2"]}]}}`,
			wantKind: KindQuery,
			wantMsg:  "Prometheus response contains invalid labels",
		},
		{
			name:     "no value",
			body:     `{"status":"success","data":{"resultType":"vector","result":[{"metric":{}}]}}`,
			wantKind: KindQuery,
			wantMsg:  "Prometheus vector result has no value",
		},
		{
			name:     "value is not a pair",
			body:     `{"status":"success","data":{"resultType":"vector","result":[{"metric":{},"value":"bogus"}]}}`,
			wantKind: KindQuery,
			wantMsg:  "Prometheus response contains an invalid sample",
		},
		{
			name:     "a member cannot be read",
			body:     `{"status":"success","data":{"resultType":"vector","result":[{"metric":{},"value":[1000,"nope"]}]}}`,
			wantKind: KindQuery,
			wantMsg:  "Prometheus response contains an invalid sample",
		},
		{
			name:     "the first member is a non-numeric string",
			body:     `{"status":"success","data":{"resultType":"vector","result":[{"metric":{},"value":["nope","2"]}]}}`,
			wantKind: KindQuery,
			wantMsg:  "Prometheus response contains an invalid sample",
		},
		{
			name:     "the first member is an object",
			body:     `{"status":"success","data":{"resultType":"vector","result":[{"metric":{},"value":[{},"2"]}]}}`,
			wantKind: KindQuery,
			wantMsg:  "Prometheus response contains an invalid sample",
		},
		{
			name:     "the first member is null",
			body:     `{"status":"success","data":{"resultType":"vector","result":[{"metric":{},"value":[null,"2"]}]}}`,
			wantKind: KindQuery,
			wantMsg:  "Prometheus response contains an invalid sample",
		},
		{
			name:     "the first member is a boolean",
			body:     `{"status":"success","data":{"resultType":"vector","result":[{"metric":{},"value":[true,"2"]}]}}`,
			wantKind: KindQuery,
			wantMsg:  "Prometheus response contains an invalid sample",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			client := connectedClient(t, func(int32) (int, string) { return http.StatusOK, tc.body })
			_, err := client.Query(context.Background(), "up", time.Unix(1000, 0))
			assertPromError(t, err, tc.wantKind, tc.wantMsg)
		})
	}
}

// TestQueryAcceptsTimeAsNumberOrText checks that a sample's first member,
// the time, is read whether Prometheus sent it as a JSON number (whole or
// fractional) or as a JSON string strconv.ParseFloat can read.
func TestQueryAcceptsTimeAsNumberOrText(t *testing.T) {
	cases := []struct {
		name string
		body string
	}{
		{
			name: "whole number",
			body: `{"status":"success","data":{"resultType":"vector","result":[{"metric":{},"value":[1784136600,"2"]}]}}`,
		},
		{
			name: "fractional number",
			body: `{"status":"success","data":{"resultType":"vector","result":[{"metric":{},"value":[1784136600.5,"2"]}]}}`,
		},
		{
			name: "text",
			body: `{"status":"success","data":{"resultType":"vector","result":[{"metric":{},"value":["1784136600","2"]}]}}`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			client := connectedClient(t, func(int32) (int, string) { return http.StatusOK, tc.body })
			samples, err := client.Query(context.Background(), "up", time.Unix(1000, 0))
			if err != nil {
				t.Fatalf("Query: %v", err)
			}
			if len(samples) != 1 {
				t.Fatalf("len(samples) = %d, want 1", len(samples))
			}
			if !samples[0].Valid || samples[0].Value != 2 {
				t.Errorf("sample = %+v, want Valid true, Value 2", samples[0])
			}
		})
	}
}

// TestValueOutOfRangeIsNoData checks that a value too large for a
// float64, well formed but out of range, reads as no data in both readers
// rather than aborting the query, the same way the original's float()
// saturates to an infinity with no exception.
func TestValueOutOfRangeIsNoData(t *testing.T) {
	cases := []string{"1e400", "-1e400"}
	for _, encoded := range cases {
		t.Run(encoded, func(t *testing.T) {
			t.Run("vector", func(t *testing.T) {
				body := fmt.Sprintf(`{"status":"success","data":{"resultType":"vector","result":[{"metric":{},"value":[1000,%q]}]}}`, encoded)
				client := connectedClient(t, func(int32) (int, string) { return http.StatusOK, body })

				samples, err := client.Query(context.Background(), "up", time.Unix(1000, 0))
				if err != nil {
					t.Fatalf("Query: %v", err)
				}
				if len(samples) != 1 {
					t.Fatalf("len(samples) = %d, want 1", len(samples))
				}
				if samples[0].Valid {
					t.Errorf("Valid = true for %q, want false", encoded)
				}
			})
			t.Run("matrix", func(t *testing.T) {
				body := fmt.Sprintf(`{"status":"success","data":{"resultType":"matrix","result":[{"metric":{},"values":[[1000,%q]]}]}}`, encoded)
				client := rangeClient(t, body)

				got, err := client.QueryRange(context.Background(), "up", time.Unix(1, 0), time.Unix(2, 0), "1s")
				if err != nil {
					t.Fatalf("QueryRange: %v", err)
				}
				if len(got) != 1 || len(got[0].Points) != 1 {
					t.Fatalf("QueryRange() = %+v, want one series of one point", got)
				}
				if got[0].Points[0].Valid {
					t.Errorf("Valid = true for %q, want false", encoded)
				}
			})
		})
	}
}

// TestValueBelowTheSmallestIsZero checks that a value too small for a
// float64 to represent reads as zero, valid, in both readers, the same
// value Python's float() gives for the same text.
func TestValueBelowTheSmallestIsZero(t *testing.T) {
	t.Run("vector", func(t *testing.T) {
		body := `{"status":"success","data":{"resultType":"vector","result":[{"metric":{},"value":[1000,"1e-400"]}]}}`
		client := connectedClient(t, func(int32) (int, string) { return http.StatusOK, body })

		samples, err := client.Query(context.Background(), "up", time.Unix(1000, 0))
		if err != nil {
			t.Fatalf("Query: %v", err)
		}
		if len(samples) != 1 {
			t.Fatalf("len(samples) = %d, want 1", len(samples))
		}
		if !samples[0].Valid {
			t.Error("Valid = false, want true")
		}
		if samples[0].Value != 0 {
			t.Errorf("Value = %v, want 0", samples[0].Value)
		}
	})
	t.Run("matrix", func(t *testing.T) {
		body := `{"status":"success","data":{"resultType":"matrix","result":[{"metric":{},"values":[[1000,"1e-400"]]}]}}`
		client := rangeClient(t, body)

		got, err := client.QueryRange(context.Background(), "up", time.Unix(1, 0), time.Unix(2, 0), "1s")
		if err != nil {
			t.Fatalf("QueryRange: %v", err)
		}
		if len(got) != 1 || len(got[0].Points) != 1 {
			t.Fatalf("QueryRange() = %+v, want one series of one point", got)
		}
		point := got[0].Points[0]
		if !point.Valid {
			t.Error("Valid = false, want true")
		}
		if point.Value != 0 {
			t.Errorf("Value = %v, want 0", point.Value)
		}
	})
}

// TestValueThatIsNoNumberIsStillRefused checks that a value that is not a
// number at all is still refused as an invalid sample in both readers,
// unlike a value that merely overflows.
func TestValueThatIsNoNumberIsStillRefused(t *testing.T) {
	cases := []string{"abc", "", "1e", "0x10"}
	for _, encoded := range cases {
		t.Run(encoded, func(t *testing.T) {
			t.Run("vector", func(t *testing.T) {
				body := fmt.Sprintf(`{"status":"success","data":{"resultType":"vector","result":[{"metric":{},"value":[1000,%q]}]}}`, encoded)
				client := connectedClient(t, func(int32) (int, string) { return http.StatusOK, body })

				_, err := client.Query(context.Background(), "up", time.Unix(1000, 0))
				assertPromError(t, err, KindQuery, "Prometheus response contains an invalid sample")
			})
			t.Run("matrix", func(t *testing.T) {
				body := fmt.Sprintf(`{"status":"success","data":{"resultType":"matrix","result":[{"metric":{},"values":[[1000,%q]]}]}}`, encoded)
				client := rangeClient(t, body)

				_, err := client.QueryRange(context.Background(), "up", time.Unix(1, 0), time.Unix(2, 0), "1s")
				assertPromError(t, err, KindQuery, "Prometheus response contains an invalid sample")
			})
		})
	}
}

// TestQueryRefusesANonFiniteTimestamp checks that a sample whose timestamp
// text reads as NaN or an infinity is refused as an invalid sample, rather
// than treated as a valid time.
func TestQueryRefusesANonFiniteTimestamp(t *testing.T) {
	cases := []string{"NaN", "+Inf", "-Inf", "Infinity"}
	for _, when := range cases {
		t.Run(when, func(t *testing.T) {
			body := fmt.Sprintf(`{"status":"success","data":{"resultType":"vector","result":[{"metric":{},"value":[%q,"9999"]}]}}`, when)
			client := connectedClient(t, func(int32) (int, string) { return http.StatusOK, body })

			_, err := client.Query(context.Background(), "up", time.Unix(1000, 0))
			assertPromError(t, err, KindQuery, "Prometheus response contains an invalid sample")
		})
	}
}
