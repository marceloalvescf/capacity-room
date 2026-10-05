package netpolicy

import (
	"context"
	"net"
	"net/netip"
)

// Resolver answers the addresses of a host, in order.
type Resolver func(ctx context.Context, host string) ([]netip.Addr, error)

// SystemResolver uses net.DefaultResolver.
func SystemResolver(ctx context.Context, host string) ([]netip.Addr, error) {
	return net.DefaultResolver.LookupNetIP(ctx, "ip", host)
}

// Resolve returns the addresses of o that the policy allows, canonical and
// without repeats, in the resolver's order: the literal alone, or every answer
// of the resolver. One refused answer refuses all of them.
func (p Policy) Resolve(ctx context.Context, o Origin, resolve Resolver) ([]netip.Addr, error) {
	var candidates []netip.Addr
	if o.Literal {
		addr, err := netip.ParseAddr(o.Host)
		if err != nil {
			return nil, refuse(CodeURLRejected)
		}
		candidates = []netip.Addr{canonical(addr)}
	} else {
		answers, err := resolve(ctx, o.Host)
		if err != nil || len(answers) == 0 {
			return nil, refuse(CodeResolutionFailed)
		}
		seen := make(map[netip.Addr]bool, len(answers))
		for _, answer := range answers {
			addr := canonical(answer)
			if !seen[addr] {
				seen[addr] = true
				candidates = append(candidates, addr)
			}
		}
	}
	for _, addr := range candidates {
		if !p.AddressAllowed(addr, o.Host) {
			return nil, refuse(CodeAddressRejected)
		}
	}
	return candidates, nil
}
