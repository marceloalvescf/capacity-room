package prom

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/marceloalvescf/k8s-capacity-report/internal/config"
)

// attempts is how many times one query is tried before it is given up on.
const attempts = 3

// maxRetryAfter is the longest wait honoured between attempts, whatever a
// Retry-After header asks for.
const maxRetryAfter = 60 * time.Second

// Kind says which class of failure an Error is.
type Kind int

// The kinds an Error carries.
const (
	KindMetrics          Kind = iota // the endpoint failed or answered badly
	KindAuthentication               // HTTP 401
	KindAuthorization                // HTTP 403
	KindTLS                          // a TLS handshake or certificate failure
	KindEndpointNotFound             // no candidate answered as a Prometheus API
	KindQuery                        // the API refused or garbled one query
)

// Error is a failure of this package. Its Message never holds the address
// of the endpoint or a token.
type Error struct {
	Kind    Kind
	Status  int // zero when there was no answer
	Message string
}

// Error returns the message.
func (e *Error) Error() string { return e.Message }

// Options configures Connect.
type Options struct {
	Metrics config.Metrics
	Token   string
	// Sleep waits between attempts. Nil means a real wait that ends early
	// when the context ends.
	Sleep func(ctx context.Context, d time.Duration) error
	// Transport replaces the transport built from Metrics. Phase 3 uses it
	// for the outbound policy. Nil means the built one.
	Transport http.RoundTripper
}

// Client asks a Prometheus-compatible API, bound to the root Connect found.
type Client struct {
	httpClient *http.Client
	root       string
	kind       string
	token      string
	sleep      func(ctx context.Context, d time.Duration) error
	answered   bool
}

// Root returns the API root that answered.
func (c *Client) Root() string { return c.root }

// Kind returns the endpoint kind, "prometheus" or "grafana".
func (c *Client) Kind() string { return c.kind }

// Connect tries each candidate root with the probe `vector(1)` and returns
// a client bound to the first root that answers as a Prometheus API.
func Connect(ctx context.Context, opts Options) (*Client, error) {
	httpClient, err := buildHTTPClient(opts)
	if err != nil {
		return nil, err
	}

	roots, kind := Candidates(opts.Metrics.URL, opts.Metrics.EndpointType)

	for _, root := range roots {
		answered := false
		_, err := runQuery(ctx, httpClient, root, opts.Token, opts.Sleep, "vector(1)", nil, &answered)
		if err == nil {
			return &Client{
				httpClient: httpClient,
				root:       root,
				kind:       kind,
				token:      opts.Token,
				sleep:      opts.Sleep,
				answered:   true,
			}, nil
		}

		var promErr *Error
		if errors.As(err, &promErr) && promErr.Kind == KindEndpointNotFound {
			continue
		}
		return nil, err
	}

	return nil, &Error{Kind: KindEndpointNotFound, Message: "no candidate endpoint answered the Prometheus API"}
}

// Query asks an instant query at the given time.
func (c *Client) Query(ctx context.Context, query string, at time.Time) ([]Sample, error) {
	answered := c.answered
	samples, err := runQuery(ctx, c.httpClient, c.root, c.token, c.sleep, query, &at, &answered)
	c.answered = answered
	return samples, err
}

// buildHTTPClient builds the client Connect and Query share: no proxy, no
// followed redirect, and the timeout of Metrics.TimeoutSeconds covering one
// attempt, headers and body.
func buildHTTPClient(opts Options) (*http.Client, error) {
	transport := opts.Transport
	if transport == nil {
		built, err := buildTransport(opts.Metrics)
		if err != nil {
			return nil, err
		}
		transport = built
	}
	return &http.Client{
		Transport: transport,
		Timeout:   secondsToDuration(opts.Metrics.TimeoutSeconds),
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}, nil
}

// buildTransport builds the transport a Client uses when Options.Transport
// is nil: no proxy, whatever the environment holds, and TLS trust from
// Metrics.CABundle when it is set, the system roots otherwise, with
// Metrics.TLSVerify false turning the check off.
func buildTransport(m config.Metrics) (*http.Transport, error) {
	tlsConfig := &tls.Config{}
	if !m.TLSVerify {
		tlsConfig.InsecureSkipVerify = true
	}
	if m.CABundle != "" {
		pem, err := os.ReadFile(m.CABundle)
		if err != nil {
			return nil, fmt.Errorf("reading the CA bundle: %w", err)
		}
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(pem) {
			return nil, errors.New("the CA bundle holds no usable certificate")
		}
		tlsConfig.RootCAs = pool
	}
	return &http.Transport{
		Proxy:           nil,
		TLSClientConfig: tlsConfig,
		// Every attempt is its own connection. Reusing a pooled one would
		// let a connection a server closed while idle surface as its own
		// distinct net/http error, one classified as neither a connection
		// failure nor a TLS failure, and whose text holds the address.
		DisableKeepAlives: true,
	}, nil
}

// secondsToDuration turns a whole-or-fractional number of seconds into a
// time.Duration.
func secondsToDuration(seconds float64) time.Duration {
	return time.Duration(seconds * float64(time.Second))
}

// runQuery runs one query to completion, retrying as the brief's table of
// failures describes. answered tracks whether this client (or this
// candidate, during Connect) has ever parsed a full, successful answer; it
// is set to true the moment that happens, which turns a later 404, 405, or
// unparsable body from a "wrong candidate" signal into a mid-collection
// failure.
func runQuery(ctx context.Context, httpClient *http.Client, root, token string, sleep func(context.Context, time.Duration) error, query string, at *time.Time, answered *bool) ([]Sample, error) {
	for attempt := 0; attempt < attempts; attempt++ {
		resp, err := postQuery(ctx, httpClient, root, token, query, at)
		if err != nil {
			if isTLSFailure(err) {
				return nil, &Error{Kind: KindTLS, Message: "TLS validation failed"}
			}
			if !isConnectionFailure(err) {
				// Foreign: Options.Transport failed with its own error,
				// which is neither a connection failure, a timeout, nor a
				// TLS failure. It is returned as it is, after one call.
				return nil, err
			}
			if attempt == attempts-1 {
				return nil, &Error{Kind: KindMetrics, Message: withMidCollection("could not connect to the metrics endpoint", *answered)}
			}
			if waitErr := wait(ctx, sleep, retryDelay(attempt)); waitErr != nil {
				return nil, waitErr
			}
			continue
		}

		body, readErr := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if readErr != nil {
			if attempt == attempts-1 {
				return nil, &Error{Kind: KindMetrics, Message: withMidCollection("could not connect to the metrics endpoint", *answered)}
			}
			if waitErr := wait(ctx, sleep, retryDelay(attempt)); waitErr != nil {
				return nil, waitErr
			}
			continue
		}

		switch {
		case resp.StatusCode == http.StatusUnauthorized:
			return nil, &Error{Kind: KindAuthentication, Status: resp.StatusCode, Message: "metrics endpoint rejected authentication"}

		case resp.StatusCode == http.StatusForbidden:
			return nil, &Error{Kind: KindAuthorization, Status: resp.StatusCode, Message: "metrics endpoint rejected authorization"}

		case resp.StatusCode == http.StatusNotFound:
			return nil, endpointError("Prometheus API path was not found", *answered)

		case resp.StatusCode == http.StatusMethodNotAllowed:
			return nil, endpointError("Prometheus API path rejected the query method", *answered)

		case resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= http.StatusInternalServerError:
			if attempt == attempts-1 {
				return nil, &Error{Kind: KindMetrics, Status: resp.StatusCode, Message: fmt.Sprintf("metrics endpoint returned HTTP %d", resp.StatusCode)}
			}
			delay := retryAfterDelay(resp.Header.Get("Retry-After"), retryDelay(attempt))
			if waitErr := wait(ctx, sleep, delay); waitErr != nil {
				return nil, waitErr
			}
			continue

		case resp.StatusCode < 200 || resp.StatusCode > 299:
			return nil, &Error{Kind: KindQuery, Status: resp.StatusCode, Message: fmt.Sprintf("metrics endpoint returned HTTP %d", resp.StatusCode)}
		}

		data, err := decodeAPIResponse(body, *answered)
		if err != nil {
			return nil, err
		}
		*answered = true
		return parseVector(data)
	}

	// Every branch above either returns or continues, and the branches that
	// continue always return instead on the last attempt, so this line is
	// never reached; it satisfies the compiler's need for a final return.
	return nil, &Error{Kind: KindMetrics, Message: withMidCollection("could not connect to the metrics endpoint", *answered)}
}

// postQuery sends the POST /query request: form-encoded body, no query
// string on the address, Accept: application/json, and a bearer token when
// one is given. at nil means the probe, which sends no time field.
func postQuery(ctx context.Context, httpClient *http.Client, root, token, query string, at *time.Time) (*http.Response, error) {
	form := url.Values{}
	form.Set("query", query)
	if at != nil {
		form.Set("time", strconv.FormatInt(at.Unix(), 10))
	}

	target := strings.TrimSuffix(root, "/") + "/query"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, target, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	return httpClient.Do(req)
}

// decodeAPIResponse reads the JSON body of a 2xx answer through the
// "status" and "data" checks shared by every query.
func decodeAPIResponse(body []byte, answered bool) (map[string]interface{}, error) {
	var raw interface{}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, endpointError("metrics endpoint did not answer the Prometheus API", answered)
	}
	obj, ok := raw.(map[string]interface{})
	if !ok {
		return nil, endpointError("metrics endpoint did not answer the Prometheus API", answered)
	}
	statusRaw, hasStatus := obj["status"]
	if !hasStatus {
		return nil, endpointError("metrics endpoint did not answer the Prometheus API", answered)
	}
	if status, _ := statusRaw.(string); status != "success" {
		errType := "unknown"
		if et, ok := obj["errorType"].(string); ok && et != "" {
			errType = et
		}
		return nil, &Error{Kind: KindQuery, Message: fmt.Sprintf("Prometheus query failed with type %s", errType)}
	}
	data, ok := obj["data"].(map[string]interface{})
	if !ok {
		return nil, &Error{Kind: KindQuery, Message: "Prometheus response has no data object"}
	}
	return data, nil
}

// endpointError builds the error for a signal that, before any answer, means
// "try the next candidate", and after one, means a mid-collection failure.
func endpointError(message string, answered bool) *Error {
	if answered {
		return &Error{Kind: KindMetrics, Message: message + " mid-collection"}
	}
	return &Error{Kind: KindEndpointNotFound, Message: message}
}

// withMidCollection appends the mid-collection suffix to message when
// answered is true.
func withMidCollection(message string, answered bool) string {
	if answered {
		return message + " mid-collection"
	}
	return message
}

// retryDelay is the wait after the given zero-based attempt: 1s after the
// first, 2s after the second.
func retryDelay(attempt int) time.Duration {
	return time.Duration(1<<uint(attempt)) * time.Second //nolint:gosec // attempt is always 0 or 1 here
}

// retryAfterDelay reads a Retry-After header that holds only digits as a
// count of seconds, falling back to fallback otherwise, and caps the result
// at maxRetryAfter.
func retryAfterDelay(header string, fallback time.Duration) time.Duration {
	delay := fallback
	if isAllDigits(header) {
		if seconds, err := strconv.Atoi(header); err == nil {
			delay = time.Duration(seconds) * time.Second
		}
	}
	if delay > maxRetryAfter {
		delay = maxRetryAfter
	}
	return delay
}

// isAllDigits reports whether s is non-empty and holds only the digits 0-9.
func isAllDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// wait pauses for d, through sleep when it is given, or through a real
// timer that ends early when ctx ends.
func wait(ctx context.Context, sleep func(context.Context, time.Duration) error, d time.Duration) error {
	if sleep != nil {
		return sleep(ctx, d)
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// isTLSFailure reports whether err is a TLS handshake or certificate
// failure: a *tls.CertificateVerificationError, an
// x509.UnknownAuthorityError, an x509.HostnameError, an
// x509.CertificateInvalidError, or a tls.RecordHeaderError, found anywhere
// in err's chain.
func isTLSFailure(err error) bool {
	var certVerification *tls.CertificateVerificationError
	if errors.As(err, &certVerification) {
		return true
	}
	var unknownAuthority x509.UnknownAuthorityError
	if errors.As(err, &unknownAuthority) {
		return true
	}
	var hostname x509.HostnameError
	if errors.As(err, &hostname) {
		return true
	}
	var certInvalid x509.CertificateInvalidError
	if errors.As(err, &certInvalid) {
		return true
	}
	var recordHeader tls.RecordHeaderError
	return errors.As(err, &recordHeader)
}

// isConnectionFailure reports whether err is a connection failure or a
// timeout: a *net.OpError or *net.DNSError anywhere in its chain, a
// deadline that was exceeded, a peer that closed the connection without
// answering (io.EOF or io.ErrUnexpectedEOF), a net.Error that reports
// itself as a timeout, or net/http's own unexported signal for a pooled
// connection the peer closed while it sat idle (net/http gives no exported
// error for it to match with errors.Is, so its fixed text is matched
// instead; this package never repeats that text onward, since the message
// this function's caller then uses is one of this package's own fixed
// sentences). It is checked only after isTLSFailure, so a TLS cause is
// never mistaken for a plain connection failure.
func isConnectionFailure(err error) bool {
	var opErr *net.OpError
	if errors.As(err, &opErr) {
		return true
	}
	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) {
		return true
	}
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
		return true
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return true
	}
	if strings.Contains(err.Error(), "server closed idle connection") {
		return true
	}
	return false
}
