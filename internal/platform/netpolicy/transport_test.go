package netpolicy

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/marceloalvescf/k8s-capacity-report/internal/config"
	"github.com/marceloalvescf/k8s-capacity-report/internal/prom"
	"github.com/marceloalvescf/k8s-capacity-report/internal/promtest"
)

var loopbackPolicy = Policy{Networks: []netip.Prefix{netip.MustParsePrefix("127.0.0.1/32")}}

// countingResolver maps every name to the loopback and counts its calls.
func countingResolver(calls *atomic.Int32) Resolver {
	return func(context.Context, string) ([]netip.Addr, error) {
		calls.Add(1)
		return []netip.Addr{netip.MustParseAddr("127.0.0.1")}, nil
	}
}

func serverPort(t *testing.T, s *httptest.Server) string {
	t.Helper()
	u, err := url.Parse(s.URL)
	if err != nil {
		t.Fatal(err)
	}
	return u.Port()
}

func wantCode(t *testing.T, err error, code Code) {
	t.Helper()
	var pe *Error
	if !errors.As(err, &pe) {
		t.Fatalf("err = %v, want *Error with code %s", err, code)
	}
	if pe.Code != code {
		t.Fatalf("code = %s, want %s", pe.Code, code)
	}
}

func TestTransportPostsOverLoopback(t *testing.T) {
	var host, body string
	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		host = r.Host
		raw, _ := io.ReadAll(r.Body)
		body = string(raw)
	}))
	defer server.Close()
	port := serverPort(t, server)
	var calls atomic.Int32
	tr := &Transport{Policy: loopbackPolicy, Resolve: countingResolver(&calls)}

	req, _ := http.NewRequest(http.MethodPost, "http://metrics.test:"+port+"/api/v1/query", strings.NewReader("query=up"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := tr.RoundTrip(req)
	if err != nil {
		t.Fatalf("RoundTrip: %v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}
	if want := "metrics.test:" + port; host != want {
		t.Errorf("Host = %q, want %q", host, want)
	}
	if body != "query=up" {
		t.Errorf("body = %q, want %q", body, "query=up")
	}
	if calls.Load() != 1 {
		t.Errorf("resolver calls = %d, want 1", calls.Load())
	}
}

func TestTransportNeverResolvesALiteral(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	defer server.Close()
	tr := &Transport{Policy: loopbackPolicy, Resolve: func(context.Context, string) ([]netip.Addr, error) {
		t.Error("the resolver was called for a literal")
		return nil, errors.New("must not be called")
	}}
	req, _ := http.NewRequest(http.MethodGet, "http://127.0.0.1:"+serverPort(t, server)+"/x", nil)
	resp, err := tr.RoundTrip(req)
	if err != nil {
		t.Fatalf("RoundTrip: %v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}
}

func TestTransportRefusesAnUnlistedAddress(t *testing.T) {
	var seen atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { seen.Add(1) }))
	defer server.Close()
	var calls atomic.Int32
	tr := &Transport{
		Policy:  Policy{Networks: []netip.Prefix{netip.MustParsePrefix("10.0.0.0/8")}},
		Resolve: countingResolver(&calls),
	}
	req, _ := http.NewRequest(http.MethodGet, "http://metrics.test:"+serverPort(t, server)+"/x", nil)
	_, err := tr.RoundTrip(req)
	wantCode(t, err, CodeAddressRejected)
	if seen.Load() != 0 {
		t.Errorf("server saw %d requests, want 0", seen.Load())
	}
}

// closeRecorder records that its connection was closed.
type closeRecorder struct {
	net.Conn
	closed atomic.Bool
}

func (c *closeRecorder) Close() error {
	c.closed.Store(true)
	return c.Conn.Close()
}

func TestTransportRefusesARebindingPeer(t *testing.T) {
	var first, second atomic.Int32
	firstServer := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { first.Add(1) }))
	defer firstServer.Close()
	secondServer := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { second.Add(1) }))
	defer secondServer.Close()

	// The dialer ignores the address and connects to the second server, and
	// the connection reports a peer other than the address dialled.
	var recorder *closeRecorder
	tr := &Transport{
		Policy:  loopbackPolicy,
		Resolve: countingResolver(new(atomic.Int32)),
		Dialer: dialerFunc(func(ctx context.Context, network, _ string) (net.Conn, error) {
			conn, err := (&net.Dialer{}).DialContext(ctx, network, secondServer.Listener.Addr().String())
			if err != nil {
				return nil, err
			}
			recorder = &closeRecorder{Conn: &fakePeerConn{Conn: conn}}
			return recorder, nil
		}),
	}
	req, _ := http.NewRequest(http.MethodGet, "http://metrics.test:"+serverPort(t, firstServer)+"/x", nil)
	_, err := tr.RoundTrip(req)
	wantCode(t, err, CodeConnectionRejected)
	if recorder == nil || !recorder.closed.Load() {
		t.Error("the connection was not closed")
	}
	if first.Load() != 0 || second.Load() != 0 {
		t.Errorf("requests seen = %d and %d, want 0 and 0", first.Load(), second.Load())
	}
}

type dialerFunc func(ctx context.Context, network, address string) (net.Conn, error)

func (f dialerFunc) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	return f(ctx, network, address)
}

// fakePeerConn reports a peer other than the one it is connected to.
type fakePeerConn struct{ net.Conn }

func (c *fakePeerConn) RemoteAddr() net.Addr {
	return &net.TCPAddr{IP: net.ParseIP("127.0.0.2"), Port: 1}
}

func TestTransportRefusesEveryRedirect(t *testing.T) {
	for _, status := range []int{301, 302, 303, 307, 308} {
		t.Run(strconv.Itoa(status), func(t *testing.T) {
			var seen atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				seen.Add(1)
				http.Redirect(w, r, "/elsewhere", status)
			}))
			defer server.Close()
			tr := &Transport{Policy: loopbackPolicy, Resolve: countingResolver(new(atomic.Int32))}
			req, _ := http.NewRequest(http.MethodGet, "http://metrics.test:"+serverPort(t, server)+"/x", nil)
			resp, err := tr.RoundTrip(req)
			wantCode(t, err, CodeRedirectRejected)
			if resp != nil {
				t.Error("a response came back with the refusal")
			}
			if seen.Load() != 1 {
				t.Errorf("server saw %d requests, want 1", seen.Load())
			}
		})
	}
}

func TestTransportRefusesAnInvalidURL(t *testing.T) {
	var seen, calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { seen.Add(1) }))
	defer server.Close()
	tr := &Transport{Policy: loopbackPolicy, Resolve: countingResolver(&calls)}
	req, _ := http.NewRequest(http.MethodGet, "http://metrics.test:"+serverPort(t, server)+"/../x", nil)
	_, err := tr.RoundTrip(req)
	wantCode(t, err, CodeURLRejected)
	if calls.Load() != 0 || seen.Load() != 0 {
		t.Errorf("resolutions = %d, requests = %d, want 0 and 0", calls.Load(), seen.Load())
	}
}

func TestTransportResolvesPerRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	defer server.Close()
	var calls atomic.Int32
	tr := &Transport{Policy: loopbackPolicy, Resolve: countingResolver(&calls)}
	for i := 0; i < 2; i++ {
		req, _ := http.NewRequest(http.MethodGet, "http://metrics.test:"+serverPort(t, server)+"/x", nil)
		resp, err := tr.RoundTrip(req)
		if err != nil {
			t.Fatalf("RoundTrip %d: %v", i, err)
		}
		_ = resp.Body.Close()
	}
	if calls.Load() != 2 {
		t.Errorf("resolver calls = %d, want 2", calls.Load())
	}
}

func tlsServer(t *testing.T) (*httptest.Server, *x509.CertPool) {
	t.Helper()
	server := httptest.NewTLSServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	t.Cleanup(server.Close)
	pool := x509.NewCertPool()
	pool.AddCert(server.Certificate())
	return server, pool
}

func TestTransportTLSWithMatchingName(t *testing.T) {
	server, pool := tlsServer(t)
	tr := &Transport{Policy: loopbackPolicy, Resolve: countingResolver(new(atomic.Int32)), TLS: &tls.Config{RootCAs: pool}}
	req, _ := http.NewRequest(http.MethodGet, "https://example.com:"+serverPort(t, server)+"/x", nil)
	resp, err := tr.RoundTrip(req)
	if err != nil {
		t.Fatalf("RoundTrip: %v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}
}

func TestTransportTLSNameMismatchFails(t *testing.T) {
	server, pool := tlsServer(t)
	tr := &Transport{Policy: loopbackPolicy, Resolve: countingResolver(new(atomic.Int32)), TLS: &tls.Config{RootCAs: pool}}
	req, _ := http.NewRequest(http.MethodGet, "https://other.test:"+serverPort(t, server)+"/x", nil)
	_, err := tr.RoundTrip(req)
	var verification *tls.CertificateVerificationError
	if !errors.As(err, &verification) {
		t.Fatalf("err = %v, want a *tls.CertificateVerificationError", err)
	}
	var pe *Error
	if errors.As(err, &pe) {
		t.Errorf("a TLS failure carries the policy code %s", pe.Code)
	}
}

func TestTransportTLSInsecureSkipVerify(t *testing.T) {
	server, _ := tlsServer(t)
	tr := &Transport{Policy: loopbackPolicy, Resolve: countingResolver(new(atomic.Int32)), TLS: &tls.Config{InsecureSkipVerify: true}}
	req, _ := http.NewRequest(http.MethodGet, "https://other.test:"+serverPort(t, server)+"/x", nil)
	resp, err := tr.RoundTrip(req)
	if err != nil {
		t.Fatalf("RoundTrip: %v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}
}

func TestTransportNoProxy(t *testing.T) {
	t.Setenv("HTTPS_PROXY", "http://127.0.0.1:1")
	t.Setenv("HTTP_PROXY", "http://127.0.0.1:1")
	t.Setenv("https_proxy", "http://127.0.0.1:1")
	t.Setenv("http_proxy", "http://127.0.0.1:1")
	t.Setenv("NO_PROXY", "")
	t.Setenv("no_proxy", "")
	server, pool := tlsServer(t)
	plain := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	defer plain.Close()
	tr := &Transport{Policy: loopbackPolicy, Resolve: countingResolver(new(atomic.Int32)), TLS: &tls.Config{RootCAs: pool}}
	for _, target := range []string{
		"https://example.com:" + serverPort(t, server) + "/x",
		"http://metrics.test:" + serverPort(t, plain) + "/x",
	} {
		req, _ := http.NewRequest(http.MethodGet, target, nil)
		resp, err := tr.RoundTrip(req)
		if err != nil {
			t.Fatalf("RoundTrip: %v", err)
		}
		_ = resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Errorf("status = %d, want 200", resp.StatusCode)
		}
	}
}

func TestTransportNoKeepAlive(t *testing.T) {
	var mu sync.Mutex
	news := 0
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	server.Config.ConnState = func(_ net.Conn, state http.ConnState) {
		if state == http.StateNew {
			mu.Lock()
			news++
			mu.Unlock()
		}
	}
	server.Start()
	defer server.Close()
	tr := &Transport{Policy: loopbackPolicy, Resolve: countingResolver(new(atomic.Int32))}
	for i := 0; i < 2; i++ {
		req, _ := http.NewRequest(http.MethodGet, "http://metrics.test:"+serverPort(t, server)+"/x", nil)
		resp, err := tr.RoundTrip(req)
		if err != nil {
			t.Fatalf("RoundTrip %d: %v", i, err)
		}
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}
	mu.Lock()
	defer mu.Unlock()
	if news != 2 {
		t.Errorf("connections = %d, want 2", news)
	}
}

func TestTransportThroughThePromClient(t *testing.T) {
	path, err := promtest.Path("direct-complete")
	if err != nil {
		t.Fatalf("Path: %v", err)
	}
	fixture, err := promtest.Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	server := httptest.NewServer(promtest.NewServer(fixture))
	defer server.Close()

	tr := &Transport{Policy: loopbackPolicy, Resolve: countingResolver(new(atomic.Int32))}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	client, err := prom.Connect(ctx, prom.Options{
		Metrics: config.Metrics{
			URL:            "http://metrics.test:" + serverPort(t, server) + "/api/v1",
			EndpointType:   "prometheus",
			TLSVerify:      true,
			TimeoutSeconds: 10,
		},
		Transport: tr,
	})
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	samples, err := client.Query(ctx, "kube_node_info", time.Unix(1700000000, 0))
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(samples) != 1 || samples[0].Value != 1 || samples[0].Labels["cluster"] != "prod" {
		t.Errorf("samples = %+v, want one sample of value 1 with cluster prod", samples)
	}

	// A refusal of the policy reaches the caller of Connect as the policy's
	// own error.
	refusing := &Transport{Policy: Policy{}, Resolve: countingResolver(new(atomic.Int32))}
	_, err = prom.Connect(ctx, prom.Options{
		Metrics: config.Metrics{
			URL:            "http://metrics.test:" + serverPort(t, server) + "/api/v1",
			EndpointType:   "prometheus",
			TLSVerify:      true,
			TimeoutSeconds: 10,
		},
		Transport: refusing,
	})
	wantCode(t, err, CodeAddressRejected)
}
