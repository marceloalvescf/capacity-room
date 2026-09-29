package prom

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

const emptySuccessMatrix = `{"status":"success","data":{"resultType":"matrix","result":[]}}`

// TestQueryRangeSendsTheForm checks the request QueryRange sends: a POST
// to a path ending in /query_range, form-encoded, with the four fields,
// and no query string on the address.
func TestQueryRangeSendsTheForm(t *testing.T) {
	var calls int32
	var gotMethod, gotPath, gotRawQuery, gotBody string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&calls, 1)
		w.Header().Set("Content-Type", "application/json")
		if n == 1 {
			_, _ = w.Write([]byte(emptySuccessVector))
			return
		}
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotRawQuery = r.URL.RawQuery
		body, _ := io.ReadAll(r.Body)
		gotBody = string(body)
		_, _ = w.Write([]byte(emptySuccessMatrix))
	}))
	defer server.Close()

	sleep, _ := recordingSleep()
	opts := baseOptions(server.URL + "/api/v1")
	opts.Sleep = sleep
	client, err := Connect(context.Background(), opts)
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}

	start := time.Unix(1784050200, 0)
	end := time.Unix(1784136600, 0)
	if _, err := client.QueryRange(context.Background(), "up", start, end, "5m"); err != nil {
		t.Fatalf("QueryRange: %v", err)
	}

	if gotMethod != http.MethodPost {
		t.Errorf("method = %q, want POST", gotMethod)
	}
	if !strings.HasSuffix(gotPath, "/query_range") {
		t.Errorf("path = %q, want it to end in /query_range", gotPath)
	}
	if gotRawQuery != "" {
		t.Errorf("raw query = %q, want none (no ? in the address)", gotRawQuery)
	}
	form, err := url.ParseQuery(gotBody)
	if err != nil {
		t.Fatalf("ParseQuery(%q): %v", gotBody, err)
	}
	if got := form.Get("query"); got != "up" {
		t.Errorf("query = %q, want up", got)
	}
	if got := form.Get("start"); got != "1784050200" {
		t.Errorf("start = %q, want 1784050200", got)
	}
	if got := form.Get("end"); got != "1784136600" {
		t.Errorf("end = %q, want 1784136600", got)
	}
	if got := form.Get("step"); got != "5m" {
		t.Errorf("step = %q, want 5m", got)
	}
}

// TestQueryRangeReadsAMatrix checks that a matrix answer of two series of
// two points each is turned into Series with labels and points in order.
func TestQueryRangeReadsAMatrix(t *testing.T) {
	const body = `{"status":"success","data":{"resultType":"matrix","result":[
		{"metric":{"pod":"a"},"values":[[1000,"1"],[1010,"2"]]},
		{"metric":{"pod":"b"},"values":[[1000,"3"],[1010,"4"]]}
	]}}`
	client := rangeClient(t, body)

	got, err := client.QueryRange(context.Background(), "up", time.Unix(1000, 0), time.Unix(1010, 0), "10s")
	if err != nil {
		t.Fatalf("QueryRange: %v", err)
	}

	want := []Series{
		{
			Labels: map[string]string{"pod": "a"},
			Points: []Point{
				{Time: 1000, Value: 1, Valid: true},
				{Time: 1010, Value: 2, Valid: true},
			},
		},
		{
			Labels: map[string]string{"pod": "b"},
			Points: []Point{
				{Time: 1000, Value: 3, Valid: true},
				{Time: 1010, Value: 4, Valid: true},
			},
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("QueryRange() = %+v, want %+v", got, want)
	}
}

// TestQueryRangeMarksNonFinite checks that NaN, +Inf, and -Inf values give
// a point with Valid false, and an ordinary value gives Valid true.
func TestQueryRangeMarksNonFinite(t *testing.T) {
	const body = `{"status":"success","data":{"resultType":"matrix","result":[
		{"metric":{},"values":[[1,"NaN"],[2,"+Inf"],[3,"-Inf"],[4,"0"]]}
	]}}`
	client := rangeClient(t, body)

	got, err := client.QueryRange(context.Background(), "up", time.Unix(1, 0), time.Unix(4, 0), "1s")
	if err != nil {
		t.Fatalf("QueryRange: %v", err)
	}
	if len(got) != 1 || len(got[0].Points) != 4 {
		t.Fatalf("QueryRange() = %+v, want one series of four points", got)
	}
	points := got[0].Points
	for i := 0; i < 3; i++ {
		if points[i].Valid {
			t.Errorf("points[%d].Valid = true, want false", i)
		}
	}
	if !points[3].Valid {
		t.Error("points[3].Valid = false, want true")
	}
	if points[3].Value != 0 {
		t.Errorf("points[3].Value = %v, want 0", points[3].Value)
	}
}

// TestQueryRangeRefusesAVector checks that QueryRange accepts a matrix
// result only: a vector answer is refused, naming the result type it got.
func TestQueryRangeRefusesAVector(t *testing.T) {
	client := rangeClient(t, emptySuccessVector)

	_, err := client.QueryRange(context.Background(), "up", time.Unix(1, 0), time.Unix(2, 0), "1s")
	assertPromError(t, err, KindQuery, "unsupported Prometheus result type: vector")
}

// TestQueryRangeRefusesBadPoint checks that a point of one element, and a
// point whose value is a JSON number rather than a text, are both refused.
func TestQueryRangeRefusesBadPoint(t *testing.T) {
	cases := []struct {
		name string
		body string
	}{
		{
			name: "one element",
			body: `{"status":"success","data":{"resultType":"matrix","result":[
				{"metric":{},"values":[[1]]}
			]}}`,
		},
		{
			name: "value is a number",
			body: `{"status":"success","data":{"resultType":"matrix","result":[
				{"metric":{},"values":[[1,2]]}
			]}}`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			client := rangeClient(t, tc.body)
			_, err := client.QueryRange(context.Background(), "up", time.Unix(1, 0), time.Unix(2, 0), "1s")
			var promErr *Error
			if !errors.As(err, &promErr) || promErr.Kind != KindQuery {
				t.Fatalf("err = %v, want a *prom.Error with Kind KindQuery", err)
			}
		})
	}
}

// TestQueryRangeRetriesOn503 checks that QueryRange retries a 503 the same
// way Query does, and returns the matrix of the answer that finally comes.
func TestQueryRangeRetriesOn503(t *testing.T) {
	var probed int32
	var rangeCalls int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/query_range") {
			n := atomic.AddInt32(&rangeCalls, 1)
			if n <= 2 {
				w.WriteHeader(http.StatusServiceUnavailable)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(emptySuccessMatrix))
			return
		}
		atomic.AddInt32(&probed, 1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(emptySuccessVector))
	}))
	defer server.Close()

	sleep, _ := recordingSleep()
	opts := baseOptions(server.URL + "/api/v1")
	opts.Sleep = sleep
	client, err := Connect(context.Background(), opts)
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}

	got, err := client.QueryRange(context.Background(), "up", time.Unix(1, 0), time.Unix(2, 0), "1s")
	if err != nil {
		t.Fatalf("QueryRange: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("QueryRange() = %+v, want an empty matrix", got)
	}
	if rangeCalls != 3 {
		t.Errorf("requests to /query_range = %d, want 3", rangeCalls)
	}
}

// TestQueryRangeKeepsTheRefusalOfTheEndpoint checks that a 422 answer
// whose body is a Prometheus error keeps the errorType and error text of
// the body in the message, and is not tried again.
func TestQueryRangeKeepsTheRefusalOfTheEndpoint(t *testing.T) {
	var rangeCalls int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/query_range") {
			atomic.AddInt32(&rangeCalls, 1)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnprocessableEntity)
			_, _ = w.Write([]byte(`{"status":"error","errorType":"execution","error":"exceeded maximum resolution of 11,000 points per timeseries"}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(emptySuccessVector))
	}))
	defer server.Close()

	sleep, _ := recordingSleep()
	opts := baseOptions(server.URL + "/api/v1")
	opts.Sleep = sleep
	client, err := Connect(context.Background(), opts)
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}

	_, err = client.QueryRange(context.Background(), "up", time.Unix(1, 0), time.Unix(2, 0), "1s")
	var promErr *Error
	if !errors.As(err, &promErr) {
		t.Fatalf("err = %v, want a *prom.Error", err)
	}
	if promErr.Kind != KindQuery {
		t.Errorf("Kind = %v, want KindQuery", promErr.Kind)
	}
	if !strings.Contains(promErr.Message, "execution") {
		t.Errorf("message %q does not hold the errorType", promErr.Message)
	}
	if !strings.Contains(promErr.Message, "exceeded maximum resolution of 11,000 points per timeseries") {
		t.Errorf("message %q does not hold the error text", promErr.Message)
	}
	if rangeCalls != 1 {
		t.Errorf("requests to /query_range = %d, want 1", rangeCalls)
	}
}

// rangeClient builds a client whose second answer onward (the probe
// always gets an empty vector) is body, for a test that only cares about
// one QueryRange call.
func rangeClient(t *testing.T, body string) *Client {
	t.Helper()
	var calls int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		n := atomic.AddInt32(&calls, 1)
		w.Header().Set("Content-Type", "application/json")
		if n == 1 {
			_, _ = w.Write([]byte(emptySuccessVector))
			return
		}
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
