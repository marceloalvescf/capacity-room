package api

import (
	"net"
	"net/http"
	"net/netip"
	"strings"
)

// ClientAddress returns the canonical form of the host of r.RemoteAddr,
// or, when that host is inside one of trusted, of the last address of
// X-Forwarded-For. An entry that is not an address gives the peer. A peer
// that is not an address is returned as it is.
func ClientAddress(r *http.Request, trusted []netip.Prefix) string {
	peer := r.RemoteAddr
	if host, _, err := net.SplitHostPort(peer); err == nil {
		peer = host
	}
	addr, err := netip.ParseAddr(peer)
	if err != nil {
		return peer
	}
	addr = addr.Unmap()
	for _, prefix := range trusted {
		if prefix.Contains(addr) {
			parts := strings.Split(strings.Join(r.Header.Values("X-Forwarded-For"), ","), ",")
			if last, err := netip.ParseAddr(strings.TrimSpace(parts[len(parts)-1])); err == nil {
				return last.Unmap().String()
			}
			break
		}
	}
	return addr.String()
}
