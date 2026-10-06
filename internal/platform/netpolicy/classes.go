package netpolicy

import "net/netip"

var blockedNetworks = mustPrefixes(
	"0.0.0.0/8", "127.0.0.0/8", "169.254.0.0/16", "224.0.0.0/4", "240.0.0.0/4",
	"::/128", "::1/128", "fe80::/10", "ff00::/8", "64:ff9b::/96", "2002::/16", "2001::/32",
	"::/96", "::ffff:0:0:0/96", "64:ff9b:1::/48",
)

var blockedAddresses = []netip.Addr{
	netip.MustParseAddr("169.254.169.254"),
	netip.MustParseAddr("fd00:ec2::254"),
}

func mustPrefixes(texts ...string) []netip.Prefix {
	out := make([]netip.Prefix, len(texts))
	for i, text := range texts {
		out[i] = netip.MustParsePrefix(text)
	}
	return out
}

// canonical unmaps an IPv4-mapped address and drops the zone.
func canonical(addr netip.Addr) netip.Addr {
	return addr.Unmap().WithZone("")
}

// Blocked reports whether addr falls in a refused class, after unmapping an
// IPv4-mapped address.
func Blocked(addr netip.Addr) bool {
	addr = canonical(addr)
	if !addr.IsValid() {
		return true
	}
	for _, blocked := range blockedAddresses {
		if addr == blocked {
			return true
		}
	}
	for _, network := range blockedNetworks {
		if network.Contains(addr) {
			return true
		}
	}
	return false
}

// Policy is the allowlist of a deployment.
type Policy struct {
	Hostnames []string
	Networks  []netip.Prefix
}

// ClassPermitted reports whether addr is outside every refused class, or
// inside a network of the allowlist that holds exactly one address.
func (p Policy) ClassPermitted(addr netip.Addr) bool {
	addr = canonical(addr)
	if !Blocked(addr) {
		return true
	}
	for _, network := range p.Networks {
		if network.Bits() == network.Addr().BitLen() && network.Contains(addr) {
			return true
		}
	}
	return false
}

// AddressAllowed reports whether addr may be connected to for host: its class
// is permitted and either host is in Hostnames or addr is inside a network of
// the allowlist.
func (p Policy) AddressAllowed(addr netip.Addr, host string) bool {
	addr = canonical(addr)
	if !p.ClassPermitted(addr) {
		return false
	}
	for _, name := range p.Hostnames {
		if name == host {
			return true
		}
	}
	for _, network := range p.Networks {
		if network.Contains(addr) {
			return true
		}
	}
	return false
}
