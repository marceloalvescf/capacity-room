package prom

import (
	"context"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/marceloalvescf/k8s-capacity-report/internal/config"
)

const emptySuccessVector = `{"status":"success","data":{"resultType":"vector","result":[]}}`

// recordingSleep returns a Sleep function that records each wait it is
// asked for and returns at once, and a pointer to the slice it records
// into.
func recordingSleep() (func(context.Context, time.Duration) error, *[]time.Duration) {
	waits := &[]time.Duration{}
	return func(_ context.Context, d time.Duration) error {
		*waits = append(*waits, d)
		return nil
	}, waits
}

// baseOptions builds the Options every test starts from: an explicit
// "prometheus" endpoint type (so Candidates never guesses), TLS verified,
// and a generous timeout. Tests override what they need to.
func baseOptions(rawURL string) Options {
	return Options{
		Metrics: config.Metrics{
			URL:            rawURL,
			EndpointType:   "prometheus",
			TLSVerify:      true,
			TimeoutSeconds: 5,
		},
	}
}

// assertPromError fails the test unless err is a *prom.Error with the given
// Kind and Message.
func assertPromError(t *testing.T, err error, wantKind Kind, wantMessage string) {
	t.Helper()
	var promErr *Error
	if !errors.As(err, &promErr) {
		t.Fatalf("err = %v (%T), want a *prom.Error", err, err)
	}
	if promErr.Kind != wantKind {
		t.Errorf("Kind = %v, want %v", promErr.Kind, wantKind)
	}
	if promErr.Message != wantMessage {
		t.Errorf("Message = %q, want %q", promErr.Message, wantMessage)
	}
}

// hostOf returns the host:port component of a URL, for checking that an
// error message never repeats it.
func hostOf(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	return parsed.Host
}

func TestConnectSendsProbeAsForm(t *testing.T) {
	var gotMethod, gotPath, gotRawQuery, gotBody, gotAccept string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotRawQuery = r.URL.RawQuery
		body, _ := io.ReadAll(r.Body)
		gotBody = string(body)
		gotAccept = r.Header.Get("Accept")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(emptySuccessVector))
	}))
	defer server.Close()

	sleep, _ := recordingSleep()
	opts := baseOptions(server.URL + "/api/v1")
	opts.Sleep = sleep
	if _, err := Connect(context.Background(), opts); err != nil {
		t.Fatalf("Connect: %v", err)
	}

	if gotMethod != http.MethodPost {
		t.Errorf("method = %q, want POST", gotMethod)
	}
	if gotPath != "/api/v1/query" {
		t.Errorf("path = %q, want /api/v1/query", gotPath)
	}
	if gotRawQuery != "" {
		t.Errorf("raw query = %q, want none (no ? in the address)", gotRawQuery)
	}
	if gotBody != "query=vector%281%29" {
		t.Errorf("body = %q, want query=vector%%281%%29", gotBody)
	}
	if gotAccept != "application/json" {
		t.Errorf("Accept = %q, want application/json", gotAccept)
	}
}

func TestConnectSendsBearerToken(t *testing.T) {
	var gotAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(emptySuccessVector))
	}))
	defer server.Close()

	sleep, _ := recordingSleep()
	opts := baseOptions(server.URL + "/api/v1")
	opts.Sleep = sleep
	opts.Token = "abc"
	if _, err := Connect(context.Background(), opts); err != nil {
		t.Fatalf("Connect: %v", err)
	}

	if gotAuth != "Bearer abc" {
		t.Errorf("Authorization = %q, want %q", gotAuth, "Bearer abc")
	}
}

func TestConnectSendsNoAuthorizationWithoutToken(t *testing.T) {
	authSeen := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "" {
			authSeen = true
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(emptySuccessVector))
	}))
	defer server.Close()

	sleep, _ := recordingSleep()
	opts := baseOptions(server.URL + "/api/v1")
	opts.Sleep = sleep
	if _, err := Connect(context.Background(), opts); err != nil {
		t.Fatalf("Connect: %v", err)
	}

	if authSeen {
		t.Error("Authorization header was sent with an empty token")
	}
}

func TestConnectFallsToSecondCandidate(t *testing.T) {
	cases := []struct {
		name   string
		status int
		body   string
	}{
		{"404", http.StatusNotFound, ""},
		{"405", http.StatusMethodNotAllowed, ""},
		{"html", http.StatusOK, "<html></html>"},
		{"empty array", http.StatusOK, "[]"},
		{"object without status", http.StatusOK, `{"data":{}}`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/api/v1/query" {
					w.Header().Set("Content-Type", "application/json")
					_, _ = w.Write([]byte(emptySuccessVector))
					return
				}
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			}))
			defer server.Close()

			sleep, _ := recordingSleep()
			opts := baseOptions(server.URL)
			opts.Sleep = sleep
			client, err := Connect(context.Background(), opts)
			if err != nil {
				t.Fatalf("Connect: %v", err)
			}
			if want := server.URL + "/api/v1"; client.Root() != want {
				t.Errorf("Root() = %q, want %q", client.Root(), want)
			}
		})
	}
}

func TestConnectStopsOnOtherErrors(t *testing.T) {
	cases := []struct {
		name     string
		status   int
		wantKind Kind
		wantMsg  string
	}{
		{"401", http.StatusUnauthorized, KindAuthentication, "metrics endpoint rejected authentication"},
		{"403", http.StatusForbidden, KindAuthorization, "metrics endpoint rejected authorization"},
		{"302", http.StatusFound, KindQuery, "metrics endpoint returned HTTP 302"},
		{"418", http.StatusTeapot, KindQuery, "metrics endpoint returned HTTP 418"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var secondHits int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/api/v1/query" {
					atomic.AddInt32(&secondHits, 1)
					w.Header().Set("Content-Type", "application/json")
					_, _ = w.Write([]byte(emptySuccessVector))
					return
				}
				w.WriteHeader(tc.status)
			}))
			defer server.Close()

			sleep, _ := recordingSleep()
			opts := baseOptions(server.URL)
			opts.Sleep = sleep
			_, err := Connect(context.Background(), opts)
			assertPromError(t, err, tc.wantKind, tc.wantMsg)
			if secondHits != 0 {
				t.Errorf("second candidate received %d requests, want 0", secondHits)
			}
		})
	}
}

func TestConnectReportsNoCandidate(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	sleep, _ := recordingSleep()
	opts := baseOptions(server.URL)
	opts.Sleep = sleep
	_, err := Connect(context.Background(), opts)
	assertPromError(t, err, KindEndpointNotFound, "no candidate endpoint answered the Prometheus API")
}

func TestConnectNamesAnAnswerThatIsNotPrometheus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte("<html><body>Sign in</body></html>"))
	}))
	defer server.Close()

	sleep, _ := recordingSleep()
	opts := baseOptions(server.URL)
	opts.Sleep = sleep
	_, err := Connect(context.Background(), opts)
	if err == nil {
		t.Fatal("Connect: want an error, got nil")
	}
	if err.Error() != "no candidate endpoint answered the Prometheus API" {
		t.Errorf("message = %q, want the exact sentinel", err.Error())
	}
	if strings.Contains(err.Error(), "invalid character") {
		t.Errorf("message leaks the raw JSON parse error: %q", err.Error())
	}
}

func TestQueryAfterConnectIsMidCollection(t *testing.T) {
	cases := []struct {
		name    string
		status  int
		body    string
		wantMsg string
	}{
		{"404", http.StatusNotFound, "", "Prometheus API path was not found mid-collection"},
		{"405", http.StatusMethodNotAllowed, "", "Prometheus API path rejected the query method mid-collection"},
		{"html", http.StatusOK, "<html></html>", "metrics endpoint did not answer the Prometheus API mid-collection"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var calls int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				n := atomic.AddInt32(&calls, 1)
				if n == 1 {
					w.Header().Set("Content-Type", "application/json")
					_, _ = w.Write([]byte(emptySuccessVector))
					return
				}
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			}))
			defer server.Close()

			sleep, _ := recordingSleep()
			opts := baseOptions(server.URL + "/api/v1")
			opts.Sleep = sleep
			client, err := Connect(context.Background(), opts)
			if err != nil {
				t.Fatalf("Connect: %v", err)
			}

			_, err = client.Query(context.Background(), "up", time.Unix(1000, 0))
			assertPromError(t, err, KindMetrics, tc.wantMsg)
		})
	}
}

func TestQuerySendsTimeInSeconds(t *testing.T) {
	var calls int32
	var gotBody string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&calls, 1)
		w.Header().Set("Content-Type", "application/json")
		if n == 1 {
			_, _ = w.Write([]byte(emptySuccessVector))
			return
		}
		body, _ := io.ReadAll(r.Body)
		gotBody = string(body)
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

	loc := time.FixedZone("", -3*3600)
	at := time.Date(2026, 7, 15, 14, 30, 0, 700_000_000, loc)
	if at.Unix() != 1784136600 {
		t.Fatalf("test fixture is wrong: at.Unix() = %d, want 1784136600", at.Unix())
	}

	if _, err := client.Query(context.Background(), "up", at); err != nil {
		t.Fatalf("Query: %v", err)
	}
	if !strings.Contains(gotBody, "time=1784136600") {
		t.Errorf("body = %q, want it to hold time=1784136600", gotBody)
	}
}

// TestQueryRefusesAMatrix checks that Query accepts a vector result only:
// a matrix answer is refused, naming the result type it got.
func TestQueryRefusesAMatrix(t *testing.T) {
	var calls int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		n := atomic.AddInt32(&calls, 1)
		w.Header().Set("Content-Type", "application/json")
		if n == 1 {
			_, _ = w.Write([]byte(emptySuccessVector))
			return
		}
		_, _ = w.Write([]byte(`{"status":"success","data":{"resultType":"matrix","result":[]}}`))
	}))
	defer server.Close()

	sleep, _ := recordingSleep()
	opts := baseOptions(server.URL + "/api/v1")
	opts.Sleep = sleep
	client, err := Connect(context.Background(), opts)
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}

	_, err = client.Query(context.Background(), "up", time.Unix(1000, 0))
	assertPromError(t, err, KindQuery, "unsupported Prometheus result type: matrix")
}

// TestAnswerAboveTheLimitIsRefused checks that a body over the client's
// limit is refused at once, without being parsed or tried again.
func TestAnswerAboveTheLimitIsRefused(t *testing.T) {
	var calls int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		n := atomic.AddInt32(&calls, 1)
		w.Header().Set("Content-Type", "application/json")
		if n == 1 {
			_, _ = w.Write([]byte(emptySuccessVector))
			return
		}
		_, _ = w.Write([]byte(emptySuccessVector + "  "))
	}))
	defer server.Close()

	sleep, _ := recordingSleep()
	opts := baseOptions(server.URL + "/api/v1")
	opts.Sleep = sleep
	client, err := Connect(context.Background(), opts)
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	client.maxResponseBytes = 64

	_, err = client.Query(context.Background(), "up", time.Unix(1000, 0))
	assertPromError(t, err, KindMetrics, "metrics endpoint answered with more than 256 MiB")
	if calls != 2 {
		t.Errorf("requests = %d, want 2 (the connect probe, and the one query attempt)", calls)
	}
}

// TestAnswerAtTheLimitIsRead checks that a body of exactly the client's
// limit is read and parsed normally.
func TestAnswerAtTheLimitIsRead(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
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
	client.maxResponseBytes = int64(len(emptySuccessVector))

	if _, err := client.Query(context.Background(), "up", time.Unix(1000, 0)); err != nil {
		t.Fatalf("Query: %v", err)
	}
}

func TestQueryNeverFollowsRedirect(t *testing.T) {
	var secondHits int32
	second := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&secondHits, 1)
	}))
	defer second.Close()

	var calls int32
	first := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		n := atomic.AddInt32(&calls, 1)
		if n == 1 {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(emptySuccessVector))
			return
		}
		w.Header().Set("Location", second.URL)
		w.WriteHeader(http.StatusFound)
	}))
	defer first.Close()

	sleep, _ := recordingSleep()
	opts := baseOptions(first.URL + "/api/v1")
	opts.Sleep = sleep
	client, err := Connect(context.Background(), opts)
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}

	_, err = client.Query(context.Background(), "up", time.Unix(1000, 0))
	assertPromError(t, err, KindQuery, "metrics endpoint returned HTTP 302")
	if secondHits != 0 {
		t.Errorf("redirect target received %d requests, want 0", secondHits)
	}
}

func TestRetryOnServerError(t *testing.T) {
	var calls int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		n := atomic.AddInt32(&calls, 1)
		if n <= 2 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(emptySuccessVector))
	}))
	defer server.Close()

	sleep, waits := recordingSleep()
	opts := baseOptions(server.URL + "/api/v1")
	opts.Sleep = sleep
	if _, err := Connect(context.Background(), opts); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	want := []time.Duration{time.Second, 2 * time.Second}
	if !reflect.DeepEqual(*waits, want) {
		t.Errorf("waits = %v, want %v", *waits, want)
	}
}

func TestRetryGivesUpAfterThree(t *testing.T) {
	var calls int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	sleep, waits := recordingSleep()
	opts := baseOptions(server.URL + "/api/v1")
	opts.Sleep = sleep
	_, err := Connect(context.Background(), opts)
	assertPromError(t, err, KindMetrics, "metrics endpoint returned HTTP 503")
	if calls != 3 {
		t.Errorf("requests = %d, want 3", calls)
	}
	want := []time.Duration{time.Second, 2 * time.Second}
	if !reflect.DeepEqual(*waits, want) {
		t.Errorf("waits = %v, want %v", *waits, want)
	}
}

func TestRetryHonoursRetryAfter(t *testing.T) {
	var calls int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		n := atomic.AddInt32(&calls, 1)
		if n == 1 {
			w.Header().Set("Retry-After", "7")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(emptySuccessVector))
	}))
	defer server.Close()

	sleep, waits := recordingSleep()
	opts := baseOptions(server.URL + "/api/v1")
	opts.Sleep = sleep
	if _, err := Connect(context.Background(), opts); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	want := []time.Duration{7 * time.Second}
	if !reflect.DeepEqual(*waits, want) {
		t.Errorf("waits = %v, want %v", *waits, want)
	}
}

func TestRetryCapsRetryAfter(t *testing.T) {
	var calls int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		n := atomic.AddInt32(&calls, 1)
		if n == 1 {
			w.Header().Set("Retry-After", "3600")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(emptySuccessVector))
	}))
	defer server.Close()

	sleep, waits := recordingSleep()
	opts := baseOptions(server.URL + "/api/v1")
	opts.Sleep = sleep
	if _, err := Connect(context.Background(), opts); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	want := []time.Duration{60 * time.Second}
	if !reflect.DeepEqual(*waits, want) {
		t.Errorf("waits = %v, want %v", *waits, want)
	}
}

func TestRetryIgnoresOtherRetryAfter(t *testing.T) {
	headers := []string{"1.5", "-1", "Wed, 21 Oct 2026 07:28:00 GMT"}

	for _, header := range headers {
		t.Run(header, func(t *testing.T) {
			var calls int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				n := atomic.AddInt32(&calls, 1)
				if n == 1 {
					w.Header().Set("Retry-After", header)
					w.WriteHeader(http.StatusTooManyRequests)
					return
				}
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(emptySuccessVector))
			}))
			defer server.Close()

			sleep, waits := recordingSleep()
			opts := baseOptions(server.URL + "/api/v1")
			opts.Sleep = sleep
			if _, err := Connect(context.Background(), opts); err != nil {
				t.Fatalf("Connect: %v", err)
			}
			want := []time.Duration{time.Second}
			if !reflect.DeepEqual(*waits, want) {
				t.Errorf("waits = %v, want %v", *waits, want)
			}
		})
	}
}

// closingListener accepts every connection and closes it immediately, to
// stand in for a metrics endpoint that never answers.
func closingListener(t *testing.T) net.Listener {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen: %v", err)
	}
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			_ = conn.Close()
		}
	}()
	t.Cleanup(func() { _ = listener.Close() })
	return listener
}

func TestRetryOnConnectionFailure(t *testing.T) {
	listener := closingListener(t)

	sleep, waits := recordingSleep()
	opts := baseOptions("http://" + listener.Addr().String() + "/api/v1")
	opts.Sleep = sleep
	opts.Metrics.TimeoutSeconds = 2
	_, err := Connect(context.Background(), opts)
	assertPromError(t, err, KindMetrics, "could not connect to the metrics endpoint")
	want := []time.Duration{time.Second, 2 * time.Second}
	if !reflect.DeepEqual(*waits, want) {
		t.Errorf("waits = %v, want %v", *waits, want)
	}
}

func TestNoRetryOnClientError(t *testing.T) {
	codes := []int{http.StatusBadRequest, http.StatusUnauthorized, http.StatusForbidden, http.StatusNotFound}

	for _, code := range codes {
		t.Run(strconv.Itoa(code), func(t *testing.T) {
			var calls int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				atomic.AddInt32(&calls, 1)
				w.WriteHeader(code)
			}))
			defer server.Close()

			sleep, waits := recordingSleep()
			opts := baseOptions(server.URL + "/api/v1")
			opts.Sleep = sleep
			_, err := Connect(context.Background(), opts)
			if err == nil {
				t.Fatal("Connect: want an error, got nil")
			}
			if calls != 1 {
				t.Errorf("requests = %d, want 1", calls)
			}
			if len(*waits) != 0 {
				t.Errorf("waits = %v, want none", *waits)
			}
		})
	}
}

func TestSleepEndsWithContext(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	opts := baseOptions(server.URL + "/api/v1")
	// opts.Sleep is left nil: a real wait, ended early by the context.

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	start := time.Now()
	_, err := Connect(ctx, opts)
	elapsed := time.Since(start)

	if elapsed > time.Second {
		t.Fatalf("Connect took %v, want under 1s", elapsed)
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("err = %v, want it to wrap context.DeadlineExceeded", err)
	}
}

func TestTimeoutCoversTheBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}
		time.Sleep(2 * time.Second)
		_, _ = w.Write([]byte(emptySuccessVector))
	}))
	defer server.Close()

	sleep, _ := recordingSleep()
	opts := baseOptions(server.URL + "/api/v1")
	opts.Sleep = sleep
	opts.Metrics.TimeoutSeconds = 0.2

	start := time.Now()
	_, err := Connect(context.Background(), opts)
	elapsed := time.Since(start)

	assertPromError(t, err, KindMetrics, errCouldNotConnect)
	if elapsed > 5*time.Second {
		t.Fatalf("Connect took %v, want it to fail quickly rather than hang", elapsed)
	}
}

// hangingServer starts a server whose handler never answers on its own. It
// waits for the request's own context to end (the client gave up), but
// bounds the wait: detecting that a client dropped its side of the
// connection is not instant, and httptest.Server.Close blocks until every
// handler has returned, so an unbounded wait here would leave a slow
// detection holding the whole test open rather than the 50ms the tests
// that use this actually need.
func hangingServer(t *testing.T) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-time.After(300 * time.Millisecond):
		}
	}))
	t.Cleanup(server.Close)
	return server
}

// TestCancelledContextCarriesNoAddress checks that a context the caller
// cancels mid-request — not merely one whose deadline passed — is reported
// without the endpoint's address anywhere in the message.
func TestCancelledContextCarriesNoAddress(t *testing.T) {
	server := hangingServer(t)

	opts := baseOptions(server.URL + "/api/v1")
	opts.Token = "s3cr3t-value"
	// opts.Sleep left nil: no retry is expected, so it is never called.

	ctx, cancel := context.WithCancel(context.Background())
	time.AfterFunc(50*time.Millisecond, cancel)

	_, err := Connect(ctx, opts)
	if !errors.Is(err, context.Canceled) {
		t.Errorf("err = %v, want it to wrap context.Canceled", err)
	}
	host := hostOf(server.URL)
	if strings.Contains(err.Error(), host) {
		t.Errorf("message %q holds the host %q", err.Error(), host)
	}
	if strings.Contains(err.Error(), opts.Token) {
		t.Errorf("message %q holds the token", err.Error())
	}
}

// TestDeadlineOfTheCallCarriesNoAddress is the same, but with a context
// deadline rather than an explicit cancel, and a per-attempt timeout far
// longer than that deadline, so the caller's own context is what ends the
// call.
func TestDeadlineOfTheCallCarriesNoAddress(t *testing.T) {
	server := hangingServer(t)

	opts := baseOptions(server.URL + "/api/v1")
	opts.Metrics.TimeoutSeconds = 30

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err := Connect(ctx, opts)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("err = %v, want it to wrap context.DeadlineExceeded", err)
	}
	if strings.Contains(err.Error(), hostOf(server.URL)) {
		t.Errorf("message %q holds the host", err.Error())
	}
}

// TestTimeoutOfOneAttemptIsRetried checks that a per-attempt timeout
// (Metrics.TimeoutSeconds, not the caller's own context) is a plain,
// retried connection failure: the caller's context here has no deadline at
// all, so only the three attempts' own budgets ever expire.
func TestTimeoutOfOneAttemptIsRetried(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-time.After(500 * time.Millisecond):
		}
	}))
	defer server.Close()

	sleep, waits := recordingSleep()
	opts := baseOptions(server.URL + "/api/v1")
	opts.Sleep = sleep
	opts.Metrics.TimeoutSeconds = 0.1

	_, err := Connect(context.Background(), opts)
	assertPromError(t, err, KindMetrics, errCouldNotConnect)
	want := []time.Duration{time.Second, 2 * time.Second}
	if !reflect.DeepEqual(*waits, want) {
		t.Errorf("waits = %v, want %v", *waits, want)
	}
}

// TestBodyReadFailureIsClassified checks that a failure reading the
// response body — not sending the request — goes through the same
// classification as a connection failure, and is retried the same way.
// The brief's own text says the TLS equivalent is not needed: a body-read
// failure already exercises the shared classifier; a TLS handshake failure
// happens before any body exists to read.
func TestBodyReadFailureIsClassified(t *testing.T) {
	cases := []struct {
		name    string
		handler http.HandlerFunc
	}{
		{
			name: "connection closes mid-body",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				hijacker, ok := w.(http.Hijacker)
				if !ok {
					http.Error(w, "no hijack support", http.StatusInternalServerError)
					return
				}
				conn, buf, err := hijacker.Hijack()
				if err != nil {
					return
				}
				defer func() { _ = conn.Close() }()
				// Promise 100 bytes, deliver a few, then close: the client
				// reads a truncated body and fails before finishing it.
				_, _ = buf.WriteString("HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: 100\r\n\r\n{\"stat")
				_ = buf.Flush()
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(tc.handler)
			defer server.Close()

			sleep, waits := recordingSleep()
			opts := baseOptions(server.URL + "/api/v1")
			opts.Sleep = sleep

			_, err := Connect(context.Background(), opts)
			assertPromError(t, err, KindMetrics, errCouldNotConnect)
			want := []time.Duration{time.Second, 2 * time.Second}
			if !reflect.DeepEqual(*waits, want) {
				t.Errorf("waits = %v, want %v", *waits, want)
			}
		})
	}
}

// TestKeepAlivesStayOn checks that the built transport leaves keep-alives
// on: a report run asks about 70 questions of one endpoint, and each
// deserves to reuse a connection rather than pay a fresh handshake.
func TestKeepAlivesStayOn(t *testing.T) {
	transport, err := buildTransport(config.Metrics{TLSVerify: true, TimeoutSeconds: 5})
	if err != nil {
		t.Fatalf("buildTransport: %v", err)
	}
	if transport.DisableKeepAlives {
		t.Error("DisableKeepAlives is true, want false")
	}
}

// TestConnectionIsReused checks, end to end, that a bound Client's
// sequential requests actually share a connection rather than each paying
// a fresh handshake: fewer distinct remote addresses (as the server sees
// them) than requests.
func TestConnectionIsReused(t *testing.T) {
	var mu sync.Mutex
	remoteAddrs := map[string]bool{}
	var calls int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		remoteAddrs[r.RemoteAddr] = true
		mu.Unlock()
		atomic.AddInt32(&calls, 1)
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

	for i := 0; i < 3; i++ {
		if _, err := client.Query(context.Background(), "up", time.Unix(1000, 0)); err != nil {
			t.Fatalf("Query %d: %v", i, err)
		}
	}

	mu.Lock()
	distinct := len(remoteAddrs)
	mu.Unlock()
	if int(calls) != 4 {
		t.Fatalf("requests = %d, want 4", calls)
	}
	if distinct >= int(calls) {
		t.Errorf("distinct remote addresses = %d, want fewer than %d requests", distinct, calls)
	}
}

func TestTLSRejectsUnknownAuthority(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(emptySuccessVector))
	}))
	defer server.Close()

	sleep, _ := recordingSleep()
	opts := baseOptions(server.URL + "/api/v1")
	opts.Sleep = sleep
	_, err := Connect(context.Background(), opts)
	assertPromError(t, err, KindTLS, "TLS validation failed")
}

// writeCertBundle writes cert as a PEM bundle inside t's temporary
// directory and returns its path.
func writeCertBundle(t *testing.T, cert *x509.Certificate) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "ca.pem")
	block := &pem.Block{Type: "CERTIFICATE", Bytes: cert.Raw}
	if err := os.WriteFile(path, pem.EncodeToMemory(block), 0o600); err != nil {
		t.Fatalf("writing bundle: %v", err)
	}
	return path
}

func TestTLSAcceptsBundle(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(emptySuccessVector))
	}))
	defer server.Close()

	bundle := writeCertBundle(t, server.Certificate())

	sleep, _ := recordingSleep()
	opts := baseOptions(server.URL + "/api/v1")
	opts.Sleep = sleep
	opts.Metrics.CABundle = bundle
	if _, err := Connect(context.Background(), opts); err != nil {
		t.Fatalf("Connect: %v", err)
	}
}

func TestTLSInsecureSkipsCheck(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(emptySuccessVector))
	}))
	defer server.Close()

	sleep, _ := recordingSleep()
	opts := baseOptions(server.URL + "/api/v1")
	opts.Sleep = sleep
	opts.Metrics.TLSVerify = false
	if _, err := Connect(context.Background(), opts); err != nil {
		t.Fatalf("Connect: %v", err)
	}
}

func TestTransportUsesNoProxy(t *testing.T) {
	transport, err := buildTransport(config.Metrics{TLSVerify: true, TimeoutSeconds: 5})
	if err != nil {
		t.Fatalf("buildTransport: %v", err)
	}
	if transport.Proxy != nil {
		t.Error("Proxy is set, want nil")
	}
}

// foreignTransportErr stands in for an error a Phase 3 outbound policy
// transport might return, one this package must never try to interpret.
type foreignTransportErr struct{ reason string }

func (e foreignTransportErr) Error() string { return "policy refused: " + e.reason }

// countingRoundTripper always fails with err and counts its calls.
type countingRoundTripper struct {
	calls int32
	err   error
}

func (c *countingRoundTripper) RoundTrip(*http.Request) (*http.Response, error) {
	atomic.AddInt32(&c.calls, 1)
	return nil, c.err
}

func TestForeignTransportErrorIsReturnedOnce(t *testing.T) {
	sentinel := foreignTransportErr{reason: "denied"}
	transport := &countingRoundTripper{err: sentinel}

	sleep, waits := recordingSleep()
	opts := baseOptions("https://prom.example/api/v1")
	opts.Sleep = sleep
	opts.Transport = transport

	_, err := Connect(context.Background(), opts)
	if !errors.Is(err, sentinel) {
		t.Errorf("err = %v, want it to be sentinel by errors.Is", err)
	}
	if err.Error() != sentinel.Error() {
		t.Errorf("err.Error() = %q, want exactly %q (no Post \"...\": prefix)", err.Error(), sentinel.Error())
	}
	if transport.calls != 1 {
		t.Errorf("calls = %d, want 1", transport.calls)
	}
	if len(*waits) != 0 {
		t.Errorf("waits = %v, want none", *waits)
	}
}

// TestForeignTransportConnectionErrorIsRetried checks that a caller-supplied
// transport failing with an ordinary network error (here a *net.OpError) is
// not treated as foreign: it is retried like any other connection failure,
// and reported with this package's own fixed message.
func TestForeignTransportConnectionErrorIsRetried(t *testing.T) {
	networkErr := &net.OpError{Op: "dial", Net: "tcp", Err: errors.New("connection refused")}
	transport := &countingRoundTripper{err: networkErr}

	sleep, waits := recordingSleep()
	opts := baseOptions("https://prom.example/api/v1")
	opts.Sleep = sleep
	opts.Transport = transport

	_, err := Connect(context.Background(), opts)
	assertPromError(t, err, KindMetrics, errCouldNotConnect)
	if transport.calls != 3 {
		t.Errorf("calls = %d, want 3", transport.calls)
	}
	want := []time.Duration{time.Second, 2 * time.Second}
	if !reflect.DeepEqual(*waits, want) {
		t.Errorf("waits = %v, want %v", *waits, want)
	}
}

// TestUnknownTransportErrorOfBuiltTransportIsAConnectionFailure checks the
// classifying function directly: without a caller-supplied transport, any
// error at all — however unfamiliar — is a plain connection failure. This
// is the point of classifying by exclusion rather than by an ever-growing
// list of known errors.
func TestUnknownTransportErrorOfBuiltTransportIsAConnectionFailure(t *testing.T) {
	class, _ := classifyAttemptFailure(context.Background(), errors.New("something new"), false)
	if class != failureConnection {
		t.Errorf("class = %v, want failureConnection", class)
	}
}

// TestErrorsNeverRepeatTheAddressOrToken walks a representative case of
// every classified failure (TLS excepted, checked separately since it
// needs its own server type) with a token, and checks that no message
// holds it or the host of the server. net/http's own errors hold the
// address in their text; this package's fixed sentences must not repeat
// what those errors already exposed.
func TestErrorsNeverRepeatTheAddressOrToken(t *testing.T) {
	const token = "s3cr3t-value"

	connectWithStatus := func(t *testing.T, status int, body string) (string, error) {
		t.Helper()
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(status)
			_, _ = w.Write([]byte(body))
		}))
		defer server.Close()
		sleep, _ := recordingSleep()
		opts := baseOptions(server.URL + "/api/v1")
		opts.Sleep = sleep
		opts.Token = token
		_, err := Connect(context.Background(), opts)
		return hostOf(server.URL), err
	}

	queryAfterConnect := func(t *testing.T, status int, body string) (string, error) {
		t.Helper()
		var calls int32
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			n := atomic.AddInt32(&calls, 1)
			if n == 1 {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(emptySuccessVector))
				return
			}
			w.WriteHeader(status)
			_, _ = w.Write([]byte(body))
		}))
		defer server.Close()
		sleep, _ := recordingSleep()
		opts := baseOptions(server.URL + "/api/v1")
		opts.Sleep = sleep
		opts.Token = token
		client, err := Connect(context.Background(), opts)
		if err != nil {
			t.Fatalf("Connect: %v", err)
		}
		_, err = client.Query(context.Background(), "up", time.Unix(1000, 0))
		return hostOf(server.URL), err
	}

	scenarios := []struct {
		name string
		run  func(t *testing.T) (string, error)
	}{
		{"connection failure", func(t *testing.T) (string, error) {
			listener := closingListener(t)
			sleep, _ := recordingSleep()
			opts := baseOptions("http://" + listener.Addr().String() + "/api/v1")
			opts.Sleep = sleep
			opts.Token = token
			_, err := Connect(context.Background(), opts)
			return listener.Addr().String(), err
		}},
		{"401", func(t *testing.T) (string, error) { return connectWithStatus(t, http.StatusUnauthorized, "") }},
		{"403", func(t *testing.T) (string, error) { return connectWithStatus(t, http.StatusForbidden, "") }},
		{"no candidate (404 exhausted)", func(t *testing.T) (string, error) {
			return connectWithStatus(t, http.StatusNotFound, "")
		}},
		{"429 exhausted", func(t *testing.T) (string, error) {
			return connectWithStatus(t, http.StatusTooManyRequests, "")
		}},
		{"500 exhausted", func(t *testing.T) (string, error) {
			return connectWithStatus(t, http.StatusInternalServerError, "")
		}},
		{"other status", func(t *testing.T) (string, error) { return connectWithStatus(t, http.StatusTeapot, "") }},
		{"not prometheus", func(t *testing.T) (string, error) {
			return connectWithStatus(t, http.StatusOK, "<html></html>")
		}},
		{"mid-collection 404", func(t *testing.T) (string, error) {
			return queryAfterConnect(t, http.StatusNotFound, "")
		}},
		{"mid-collection 405", func(t *testing.T) (string, error) {
			return queryAfterConnect(t, http.StatusMethodNotAllowed, "")
		}},
		{"mid-collection not JSON", func(t *testing.T) (string, error) {
			return queryAfterConnect(t, http.StatusOK, "<html></html>")
		}},
		{"query failed status", func(t *testing.T) (string, error) {
			return queryAfterConnect(t, http.StatusOK, `{"status":"error","errorType":"bad_data"}`)
		}},
		{"no data object", func(t *testing.T) (string, error) {
			return queryAfterConnect(t, http.StatusOK, `{"status":"success"}`)
		}},
		{"bad resultType", func(t *testing.T) (string, error) {
			return queryAfterConnect(t, http.StatusOK, `{"status":"success","data":{"resultType":"matrix","result":[]}}`)
		}},
		{"bad result item", func(t *testing.T) (string, error) {
			return queryAfterConnect(t, http.StatusOK, `{"status":"success","data":{"resultType":"vector","result":["x"]}}`)
		}},
		{"bad labels", func(t *testing.T) (string, error) {
			return queryAfterConnect(t, http.StatusOK,
				`{"status":"success","data":{"resultType":"vector","result":[{"metric":"x","value":[1,"2"]}]}}`)
		}},
		{"no value", func(t *testing.T) (string, error) {
			return queryAfterConnect(t, http.StatusOK, `{"status":"success","data":{"resultType":"vector","result":[{"metric":{}}]}}`)
		}},
		{"invalid sample", func(t *testing.T) (string, error) {
			return queryAfterConnect(t, http.StatusOK,
				`{"status":"success","data":{"resultType":"vector","result":[{"metric":{},"value":[1,"nope"]}]}}`)
		}},
		{"cancelled context", func(t *testing.T) (string, error) {
			server := hangingServer(t)
			opts := baseOptions(server.URL + "/api/v1")
			opts.Token = token
			ctx, cancel := context.WithCancel(context.Background())
			time.AfterFunc(50*time.Millisecond, cancel)
			_, err := Connect(ctx, opts)
			return hostOf(server.URL), err
		}},
		{"deadline", func(t *testing.T) (string, error) {
			server := hangingServer(t)
			opts := baseOptions(server.URL + "/api/v1")
			opts.Token = token
			opts.Metrics.TimeoutSeconds = 30
			ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
			defer cancel()
			_, err := Connect(ctx, opts)
			return hostOf(server.URL), err
		}},
		{"refused connection to a closed port", func(*testing.T) (string, error) {
			sleep, _ := recordingSleep()
			opts := baseOptions("http://127.0.0.1:1/api/v1")
			opts.Sleep = sleep
			opts.Token = token
			_, err := Connect(context.Background(), opts)
			return "127.0.0.1:1", err
		}},
		{"a DNS name that does not resolve", func(*testing.T) (string, error) {
			sleep, _ := recordingSleep()
			opts := baseOptions("http://no-such-host.invalid/api/v1")
			opts.Sleep = sleep
			opts.Token = token
			_, err := Connect(context.Background(), opts)
			return "no-such-host.invalid", err
		}},
	}

	for _, sc := range scenarios {
		t.Run(sc.name, func(t *testing.T) {
			host, err := sc.run(t)
			if err == nil {
				t.Fatal("want an error")
			}
			msg := err.Error()
			if strings.Contains(msg, token) {
				t.Errorf("message %q holds the token", msg)
			}
			if host != "" && strings.Contains(msg, host) {
				t.Errorf("message %q holds the host %q", msg, host)
			}
		})
	}

	t.Run("tls", func(t *testing.T) {
		server := httptest.NewTLSServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
		defer server.Close()
		sleep, _ := recordingSleep()
		opts := baseOptions(server.URL + "/api/v1")
		opts.Sleep = sleep
		opts.Token = token
		_, err := Connect(context.Background(), opts)
		if err == nil {
			t.Fatal("want an error")
		}
		if strings.Contains(err.Error(), token) {
			t.Errorf("message %q holds the token", err.Error())
		}
		if strings.Contains(err.Error(), hostOf(server.URL)) {
			t.Errorf("message %q holds the host", err.Error())
		}
	})
}
