package netpolicy

import (
	"context"
	"crypto/tls"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"sync"
)

// ContextDialer opens a connection. *net.Dialer is one.
type ContextDialer interface {
	DialContext(ctx context.Context, network, address string) (net.Conn, error)
}

// Transport is the http.RoundTripper of every request to a metric source:
// it validates the URL, resolves the host again, dials the first allowed
// address as a literal, compares the peer, refuses a redirect, and closes
// the connection after the body. It uses no proxy and keeps no connection.
type Transport struct {
	Policy  Policy
	Resolve Resolver      // nil means SystemResolver
	TLS     *tls.Config   // ServerName is set per request from the origin's host
	Dialer  ContextDialer // nil means a dialer with no timeout; the client's timeout bounds the request
}

// RoundTrip sends r under the policy. The body of r is closed on every
// return, by the inner transport once it has taken the body, and here on a
// return before that.
func (t *Transport) RoundTrip(r *http.Request) (*http.Response, error) {
	handed := false
	defer func() {
		if !handed && r.Body != nil {
			_ = r.Body.Close()
		}
	}()
	origin, err := t.Policy.ValidateURL(r.URL.String())
	if err != nil {
		return nil, err
	}
	resolve := t.Resolve
	if resolve == nil {
		resolve = SystemResolver
	}
	addrs, err := t.Policy.Resolve(r.Context(), origin, resolve)
	if err != nil {
		return nil, err
	}
	addr := addrs[0]

	var dialer ContextDialer = &net.Dialer{}
	if t.Dialer != nil {
		dialer = t.Dialer
	}
	var conn net.Conn
	conn, err = dialer.DialContext(r.Context(), "tcp", net.JoinHostPort(addr.String(), strconv.Itoa(origin.Port)))
	if err != nil {
		return nil, err
	}

	tcp, ok := conn.RemoteAddr().(*net.TCPAddr)
	if !ok || canonical(tcp.AddrPort().Addr()) != addr {
		_ = conn.Close()
		return nil, refuse(CodeConnectionRejected)
	}

	transport := &http.Transport{Proxy: nil, DisableKeepAlives: true, ForceAttemptHTTP2: false}
	if origin.Scheme == "https" {
		cfg := &tls.Config{}
		if t.TLS != nil {
			cfg = t.TLS.Clone()
		}
		// An IP literal as ServerName sends no SNI and is checked against the
		// IP addresses of the certificate.
		cfg.ServerName = origin.Host
		secured := tls.Client(conn, cfg)
		if err := secured.HandshakeContext(r.Context()); err != nil {
			_ = conn.Close()
			return nil, err
		}
		conn = secured
		transport.DialTLSContext = func(context.Context, string, string) (net.Conn, error) { return secured, nil }
	} else {
		transport.DialContext = func(context.Context, string, string) (net.Conn, error) { return conn, nil }
	}

	target, err := url.Parse(origin.Scheme + "://" + origin.Authority() + origin.Path)
	if err != nil {
		_ = conn.Close()
		return nil, refuse(CodeURLRejected)
	}
	out := r.Clone(r.Context())
	out.URL = target
	out.Host = origin.Authority()

	handed = true
	resp, err := transport.RoundTrip(out)
	if err != nil {
		transport.CloseIdleConnections()
		_ = conn.Close()
		return nil, err
	}
	if resp.StatusCode >= 300 && resp.StatusCode <= 399 {
		_ = resp.Body.Close()
		transport.CloseIdleConnections()
		_ = conn.Close()
		return nil, refuse(CodeRedirectRejected)
	}
	resp.Body = &closingBody{ReadCloser: resp.Body, release: func() {
		transport.CloseIdleConnections()
		_ = conn.Close()
	}}
	return resp, nil
}

// closingBody releases the connection when the body is read to its end or
// closed.
type closingBody struct {
	io.ReadCloser
	once    sync.Once
	release func()
}

func (b *closingBody) Read(p []byte) (int, error) {
	n, err := b.ReadCloser.Read(p)
	if err == io.EOF {
		b.once.Do(b.release)
	}
	return n, err
}

func (b *closingBody) Close() error {
	err := b.ReadCloser.Close()
	b.once.Do(b.release)
	return err
}
