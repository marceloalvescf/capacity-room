package prom

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/marceloalvescf/capacity-room/internal/config"
)

// attempts is how many times one query is tried before it is given up on.
const attempts = 3

// maxRetryAfter is the longest wait honoured between attempts, whatever a
// Retry-After header asks for.
const maxRetryAfter = 60 * time.Second

// errCouldNotConnect is the fixed message of a connection failure or a
// timeout, once every attempt has been used.
const errCouldNotConnect = "could not connect to the metrics endpoint"

// MaxResponseBytes is the largest answer the client reads.
const MaxResponseBytes = 256 << 20

// errAnswerTooLarge is the fixed message of a body over the client's
// limit, whatever that limit actually is.
const errAnswerTooLarge = "metrics endpoint answered with more than 256 MiB"

// midCollectionSuffix marks a failure that came after a good probe: the
// candidate was the right one, something just went wrong along the way.
const midCollectionSuffix = " mid-collection"

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
	// Transport replaces the transport built from Metrics, for a caller
	// that enforces its own outbound policy. Nil means the built one.
	Transport http.RoundTripper
}

// Client asks a Prometheus-compatible API, bound to the root Connect found.
type Client struct {
	httpClient *http.Client
	root       string
	token      string
	sleep      func(ctx context.Context, d time.Duration) error
	answered   bool
	// foreignTransport is true when the caller supplied Options.Transport.
	// Only an error of that transport which carries the PolicyRefusal marker
	// stops at once and is returned as it is; any other transport error is
	// a connection failure and is tried again.
	foreignTransport bool
	// maxResponseBytes is the largest answer this client reads. It is
	// MaxResponseBytes, and only a test of this package sets it smaller.
	maxResponseBytes int64
}

// Root returns the API root that answered.
func (c *Client) Root() string { return c.root }

// Connect tries each candidate root with the probe `vector(1)` and returns
// a client bound to the first root that answers as a Prometheus API.
func Connect(ctx context.Context, opts Options) (*Client, error) {
	httpClient, err := buildHTTPClient(opts)
	if err != nil {
		return nil, err
	}
	foreignTransport := opts.Transport != nil

	roots := Candidates(opts.Metrics.URL)

	for _, root := range roots {
		answered := false
		data, err := runQuery(ctx, opts.Sleep, probeSend(httpClient, root, opts.Token), &answered, foreignTransport, MaxResponseBytes)
		if err == nil {
			_, err = parseVector(data)
		}
		if err == nil {
			return &Client{
				httpClient:       httpClient,
				root:             root,
				token:            opts.Token,
				sleep:            opts.Sleep,
				answered:         true,
				foreignTransport: foreignTransport,
				maxResponseBytes: MaxResponseBytes,
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
	data, err := runQuery(ctx, c.sleep, querySend(c.httpClient, c.root, c.token, query, at), &answered, c.foreignTransport, c.maxResponseBytes)
	c.answered = answered
	if err != nil {
		return nil, err
	}
	return parseVector(data)
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
// is nil: no proxy, whatever the environment holds; TLS trust from
// Metrics.CABundle when it is set, the system roots otherwise; and
// keep-alives left on, since one report run asks a bound Client dozens of
// questions and each deserves to reuse the same connection rather than pay
// a fresh handshake.
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
	}, nil
}

// secondsToDuration turns a whole-or-fractional number of seconds into a
// time.Duration.
func secondsToDuration(seconds float64) time.Duration {
	return time.Duration(seconds * float64(time.Second))
}

// runQuery runs one query to completion, retrying a plain connection or
// timeout failure up to 3 attempts with a growing backoff, while a TLS
// failure, an error that carries the PolicyRefusal marker, or a cancelled
// context stops at once.
// answered tracks whether this client (or this candidate, during Connect)
// has ever parsed a full, successful answer; it is set to true the moment
// that happens, which turns a later 404, 405, or unparsable body from a
// "wrong candidate" signal into a mid-collection failure. send performs
// one attempt; Connect, Query, and QueryRange each give it a different
// request to send, and share everything else through this one function.
// The "data" object of a successful answer is returned for the caller to
// parse according to the result type it accepts.
func runQuery(
	ctx context.Context,
	sleep func(context.Context, time.Duration) error,
	send func(ctx context.Context) (*http.Response, error),
	answered *bool,
	foreignTransport bool,
	maxResponseBytes int64,
) (json.RawMessage, error) {
	for attempt := 0; attempt < attempts; attempt++ {
		resp, err := send(ctx)
		if err != nil {
			if done, failErr := attemptFailed(ctx, sleep, err, foreignTransport, *answered, attempt); done {
				return nil, failErr
			}
			continue
		}

		body, tooLarge, readErr := readLimitedBody(resp.Body, maxResponseBytes)
		_ = resp.Body.Close()
		if tooLarge {
			return nil, &Error{Kind: KindMetrics, Message: errAnswerTooLarge}
		}
		if readErr != nil {
			if done, failErr := attemptFailed(ctx, sleep, readErr, foreignTransport, *answered, attempt); done {
				return nil, failErr
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
				return nil, fmt.Errorf("metrics request stopped: %w", waitErr)
			}
			continue

		case resp.StatusCode == http.StatusBadRequest || resp.StatusCode == http.StatusUnprocessableEntity:
			return nil, &Error{Kind: KindQuery, Status: resp.StatusCode, Message: queryRefusalMessage(resp.StatusCode, body)}

		case resp.StatusCode < 200 || resp.StatusCode > 299:
			return nil, &Error{Kind: KindQuery, Status: resp.StatusCode, Message: fmt.Sprintf("metrics endpoint returned HTTP %d", resp.StatusCode)}
		}

		data, err := decodeAPIResponse(body, *answered)
		if err != nil {
			return nil, err
		}
		*answered = true
		return data, nil
	}

	// Every branch above either returns or continues, and the branches that
	// continue always return instead on the last attempt, so this line is
	// never reached; it satisfies the compiler's need for a final return.
	return nil, &Error{Kind: KindMetrics, Message: withMidCollection(errCouldNotConnect, *answered)}
}

// readLimitedBody reads at most limit+1 bytes of body. tooLarge is true
// when the body holds more than limit bytes, in which case data is nil and
// the body is left unread past that point: it is refused, not parsed.
func readLimitedBody(body io.Reader, limit int64) (data []byte, tooLarge bool, err error) {
	data, err = io.ReadAll(io.LimitReader(body, limit+1))
	if err != nil {
		return nil, false, err
	}
	if int64(len(data)) > limit {
		return nil, true, nil
	}
	return data, false, nil
}

// queryRefusalMessage builds the message for a 400 or 422 answer: the
// errorType and error text of a Prometheus error body, when the body holds
// both as strings, or the fixed "HTTP <code>" text otherwise.
func queryRefusalMessage(status int, body []byte) string {
	var parsed struct {
		ErrorType string `json:"errorType"`
		Error     string `json:"error"`
	}
	if err := json.Unmarshal(body, &parsed); err == nil && parsed.ErrorType != "" && parsed.Error != "" {
		return fmt.Sprintf("Prometheus query failed with type %s: %s", parsed.ErrorType, parsed.Error)
	}
	return fmt.Sprintf("metrics endpoint returned HTTP %d", status)
}

// attemptFailed classifies one attempt's failure — from sending the
// request or from reading its body, the same classification either way —
// and decides what runQuery does next. When done is true, runQuery returns
// err immediately. When done is false, the attempt is retried: a wait has
// already happened, unless this was the last attempt, in which case done
// is always true.
func attemptFailed(
	ctx context.Context,
	sleep func(context.Context, time.Duration) error,
	failure error,
	foreignTransport bool,
	answered bool,
	attempt int,
) (done bool, err error) {
	switch class, cause := classifyAttemptFailure(ctx, failure, foreignTransport); class {
	case failureStopped:
		return true, cause
	case failureTLS:
		return true, &Error{Kind: KindTLS, Message: "TLS validation failed"}
	case failureForeign:
		return true, cause
	}

	if attempt == attempts-1 {
		return true, &Error{Kind: KindMetrics, Message: withMidCollection(errCouldNotConnect, answered)}
	}
	if waitErr := wait(ctx, sleep, retryDelay(attempt)); waitErr != nil {
		return true, fmt.Errorf("metrics request stopped: %w", waitErr)
	}
	return false, nil
}

// probeSend builds the send function for Connect's probe: `vector(1)`,
// with no time field.
func probeSend(httpClient *http.Client, root, token string) func(context.Context) (*http.Response, error) {
	return func(ctx context.Context) (*http.Response, error) {
		req, err := newQueryRequest(ctx, root, token, "vector(1)", nil)
		if err != nil {
			return nil, err
		}
		return httpClient.Do(req)
	}
}

// querySend builds the send function for one instant query attempt.
func querySend(httpClient *http.Client, root, token, query string, at time.Time) func(context.Context) (*http.Response, error) {
	return func(ctx context.Context) (*http.Response, error) {
		req, err := newQueryRequest(ctx, root, token, query, &at)
		if err != nil {
			return nil, err
		}
		return httpClient.Do(req)
	}
}

// newQueryRequest builds the POST /query request: form-encoded body, no
// query string on the address, Accept: application/json, and a bearer
// token when one is given. at nil means the probe, which sends no time
// field.
func newQueryRequest(ctx context.Context, root, token, query string, at *time.Time) (*http.Request, error) {
	form := url.Values{}
	form.Set("query", query)
	if at != nil {
		form.Set("time", strconv.FormatInt(at.Unix(), 10))
	}
	return newFormRequest(ctx, root, "/query", token, form)
}

// newFormRequest builds one POST request against root+path: form-encoded
// body, no query string on the address, Accept: application/json, and a
// bearer token when one is given.
func newFormRequest(ctx context.Context, root, path, token string, form url.Values) (*http.Request, error) {
	target := strings.TrimSuffix(root, "/") + path
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, target, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	return req, nil
}

// decodeAPIResponse reads the JSON body of a 2xx answer through the
// "status" and "data" checks shared by every query, and returns the "data"
// object undecoded: the parsers decode it one series at a time, so no
// whole-answer tree is ever held in memory.
func decodeAPIResponse(body []byte, answered bool) (json.RawMessage, error) {
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(body, &obj); err != nil || obj == nil {
		return nil, endpointError("metrics endpoint did not answer the Prometheus API", answered)
	}
	if !numbersFitFloat64(body) {
		return nil, endpointError("metrics endpoint did not answer the Prometheus API", answered)
	}
	statusRaw, hasStatus := obj["status"]
	if !hasStatus {
		return nil, endpointError("metrics endpoint did not answer the Prometheus API", answered)
	}
	var status interface{}
	_ = json.Unmarshal(statusRaw, &status)
	if s, _ := status.(string); s != "success" {
		errType := "unknown"
		var et interface{}
		_ = json.Unmarshal(obj["errorType"], &et)
		if s, ok := et.(string); ok && s != "" {
			errType = s
		}
		return nil, &Error{Kind: KindQuery, Message: fmt.Sprintf("Prometheus query failed with type %s", errType)}
	}
	data := obj["data"]
	if !isJSONObject(data) {
		return nil, &Error{Kind: KindQuery, Message: "Prometheus response has no data object"}
	}
	return data, nil
}

// numbersFitFloat64 refuses what decoding the whole answer into interface{}
// refused: a number literal float64 cannot hold. It reads tokens only, so no
// tree is built, and it runs on a body already known to be valid JSON.
func numbersFitFloat64(body []byte) bool {
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.UseNumber()
	for {
		tok, err := dec.Token()
		if err != nil {
			return err == io.EOF
		}
		if n, ok := tok.(json.Number); ok {
			if _, err := strconv.ParseFloat(string(n), 64); err != nil {
				return false
			}
		}
	}
}

// isJSONObject reports whether raw, already validated JSON, is an object.
func isJSONObject(raw json.RawMessage) bool {
	trimmed := bytes.TrimLeft(raw, " \t\r\n")
	return len(trimmed) > 0 && trimmed[0] == '{'
}

// endpointError builds the error for a signal that, before any answer,
// means "try the next candidate", and after one, means a mid-collection
// failure. It is withMidCollection's only other caller, so the two never
// drift apart on what "after one" means.
func endpointError(message string, answered bool) *Error {
	if answered {
		return &Error{Kind: KindMetrics, Message: withMidCollection(message, answered)}
	}
	return &Error{Kind: KindEndpointNotFound, Message: message}
}

// withMidCollection appends the mid-collection suffix to message when
// answered is true.
func withMidCollection(message string, answered bool) string {
	if answered {
		return message + midCollectionSuffix
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

// failureClass is what classifyAttemptFailure sorts one attempt's error
// into.
type failureClass int

const (
	// failureConnection is everything classifyAttemptFailure did not sort
	// into one of the other three classes: retried, and after the last
	// attempt, reported as errCouldNotConnect.
	failureConnection failureClass = iota
	// failureStopped means the caller's own context ended. Not retried.
	failureStopped
	// failureTLS means a TLS handshake or certificate failure. Not
	// retried.
	failureTLS
	// failureForeign means the caller's own Options.Transport refused the
	// request with an error that carries the PolicyRefusal marker. Returned
	// as it is, after one call.
	failureForeign
)

// classifyAttemptFailure sorts err — from sending a request or from
// reading its body — into one of four classes, by exclusion rather than by
// an ever-growing list of known errors: has the caller's own context
// ended; is the cause a TLS failure; if the caller supplied its own
// transport, is the cause a refusal that carries the PolicyRefusal marker;
// and failing all of those, an ordinary connection failure, the same
// whether its exact shape is a timeout, a refused connection, a DNS failure,
// an answer that is not HTTP, or anything else net/http might produce.
//
// http.Client.Do wraps every transport error in a *url.Error, whose text
// holds the request's address; the cause inside it is unwrapped first, and
// classification and every message built from cause work only with that —
// the *url.Error itself is never returned or formatted. ctx is the context
// given to Connect or Query, not the timeout of one attempt: an attempt
// that merely used up its own share of Metrics.TimeoutSeconds is a plain
// connection failure, tried again: only ctx itself ending stops the
// retries outright.
//
// foreignTransport is true only when the caller supplied its own
// Options.Transport: the built-in transport's own errors are never treated
// as foreign, whatever shape they take.
func classifyAttemptFailure(ctx context.Context, err error, foreignTransport bool) (failureClass, error) {
	if ctxErr := ctx.Err(); ctxErr != nil {
		return failureStopped, fmt.Errorf("metrics request stopped: %w", ctxErr)
	}

	cause := unwrapURLError(err)

	if isTLSFailure(cause) {
		return failureTLS, nil
	}

	if foreignTransport && isRefusal(cause) {
		return failureForeign, cause
	}

	return failureConnection, nil
}

// unwrapURLError returns the cause inside a *url.Error — what
// http.Client.Do always wraps a transport failure in — or err itself when
// it is not one. A *url.Error's own text holds the request's address, so
// this package looks only at what is inside it.
func unwrapURLError(err error) error {
	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		return urlErr.Err
	}
	return err
}

// isRefusal reports whether cause is a refusal of the caller-supplied
// transport, an error that carries the PolicyRefusal marker. Any other error
// of a transport, a network problem or an answer that is not HTTP, is a
// failure to reach the source.
func isRefusal(cause error) bool {
	var refusal interface{ PolicyRefusal() }
	return errors.As(cause, &refusal)
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
