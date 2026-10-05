package api

import (
	"net/http"
	"net/netip"
	"testing"
)

func TestClientAddress(t *testing.T) {
	proxies := []netip.Prefix{netip.MustParsePrefix("10.0.0.0/8")}
	cases := []struct {
		name    string
		trusted []netip.Prefix
		peer    string
		xff     string
		want    string
	}{
		{"UsesRemoteAddr", nil, "203.0.113.5:4321", "10.0.0.1", "203.0.113.5"},
		{"IgnoresForwardedFromUntrustedPeer", proxies, "203.0.113.5:4321", "1.2.3.4", "203.0.113.5"},
		{"TrustsListedProxy", proxies, "10.1.1.1:80", "203.0.113.9", "203.0.113.9"},
		{"TrustsListedProxyLast", proxies, "10.1.1.1:80", "1.1.1.1, 203.0.113.9", "203.0.113.9"},
		{"TrustsListedProxyNoHeader", proxies, "10.1.1.1:80", "", "10.1.1.1"},
		{"IPv6", nil, "[::1]:5", "", "::1"},
		{"NoPort", nil, "unix", "", "unix"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := &http.Request{RemoteAddr: c.peer, Header: http.Header{}}
			if c.xff != "" {
				r.Header.Set("X-Forwarded-For", c.xff)
			}
			if got := ClientAddress(r, c.trusted); got != c.want {
				t.Fatalf("got %q, want %q", got, c.want)
			}
		})
	}
}
