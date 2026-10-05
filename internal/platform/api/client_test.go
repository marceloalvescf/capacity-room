package api

import (
	"net/http"
	"net/netip"
	"testing"
)

var proxies = []netip.Prefix{netip.MustParsePrefix("10.0.0.0/8")}

func address(trusted []netip.Prefix, peer string, xff ...string) string {
	r := &http.Request{RemoteAddr: peer, Header: http.Header{}}
	for _, line := range xff {
		r.Header.Add("X-Forwarded-For", line)
	}
	return ClientAddress(r, trusted)
}

func expect(t *testing.T, got, want string) {
	t.Helper()
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestClientAddressUsesRemoteAddr(t *testing.T) {
	expect(t, address(nil, "203.0.113.5:4321", "10.0.0.1"), "203.0.113.5")
}

func TestClientAddressIgnoresForwardedFromUntrustedPeer(t *testing.T) {
	expect(t, address(proxies, "203.0.113.5:4321", "1.2.3.4"), "203.0.113.5")
}

func TestClientAddressTrustsListedProxy(t *testing.T) {
	expect(t, address(proxies, "10.1.1.1:80", "203.0.113.9"), "203.0.113.9")
	expect(t, address(proxies, "10.1.1.1:80", "1.1.1.1, 203.0.113.9"), "203.0.113.9")
	expect(t, address(proxies, "10.1.1.1:80", "1.1.1.1", "203.0.113.9"), "203.0.113.9")
	expect(t, address(proxies, "10.1.1.1:80"), "10.1.1.1")
}

func TestClientAddressIPv6(t *testing.T) {
	expect(t, address(nil, "[::1]:5"), "::1")
}

func TestClientAddressNoPort(t *testing.T) {
	expect(t, address(nil, "unix"), "unix")
}
