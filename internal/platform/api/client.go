package api

import (
	"net"
	"net/http"
	"net/netip"
	"strings"
)

// ClientAddress returns the host of r.RemoteAddr, or, when that host is
// inside one of trusted, the last address of X-Forwarded-For.
func ClientAddress(r *http.Request, trusted []netip.Prefix) string {
	peer := r.RemoteAddr
	if host, _, err := net.SplitHostPort(peer); err == nil {
		peer = host
	}
	addr, err := netip.ParseAddr(peer)
	if err != nil {
		return peer
	}
	for _, prefix := range trusted {
		if prefix.Contains(addr.Unmap()) {
			parts := strings.Split(strings.Join(r.Header.Values("X-Forwarded-For"), ","), ",")
			if last := strings.TrimSpace(parts[len(parts)-1]); last != "" {
				return last
			}
			break
		}
	}
	return peer
}
