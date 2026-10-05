package netpolicy

import (
	"net/netip"
	"testing"
)

func TestBlockedClasses(t *testing.T) {
	for _, text := range []string{
		"127.0.0.1", "::1", "169.254.169.254", "fe80::1", "224.0.0.1", "0.0.0.0",
		"::ffff:127.0.0.1", "fd00:ec2::254", "64:ff9b::7f00:1", "2002:7f00:1::",
		"2001:0:1::1", "255.255.255.255", "::",
	} {
		if !Blocked(netip.MustParseAddr(text)) {
			t.Errorf("%s not blocked", text)
		}
	}
}

func TestNotBlocked(t *testing.T) {
	for _, text := range []string{
		"10.0.0.1", "172.16.0.1", "192.168.1.1", "100.64.0.1", "fc00::1", "8.8.8.8", "::ffff:10.20.0.4",
	} {
		if Blocked(netip.MustParseAddr(text)) {
			t.Errorf("%s blocked", text)
		}
	}
}

func TestClassPermittedLiftsOneAddress(t *testing.T) {
	lifted := Policy{Networks: prefixes(t, "169.254.169.254/32")}
	if !lifted.ClassPermitted(netip.MustParseAddr("169.254.169.254")) {
		t.Error("the listed address is not permitted")
	}
	if lifted.ClassPermitted(netip.MustParseAddr("169.254.169.253")) {
		t.Error("a neighbour address is permitted")
	}
	byName := Policy{Hostnames: []string{"metrics.internal"}}
	if byName.ClassPermitted(netip.MustParseAddr("127.0.0.1")) {
		t.Error("a hostname entry lifted a class")
	}
}

func TestAddressAllowed(t *testing.T) {
	cases := []struct {
		policy Policy
		addr   string
		host   string
		want   bool
	}{
		{Policy{Hostnames: []string{"h"}}, "10.1.1.1", "h", true},
		{Policy{Hostnames: []string{"h"}}, "10.1.1.1", "other", false},
		{Policy{Networks: prefixes(t, "10.0.0.0/8")}, "10.1.1.1", "other", true},
		{Policy{Networks: prefixes(t, "10.0.0.0/8")}, "11.0.0.1", "other", false},
		{Policy{}, "10.1.1.1", "h", false},
	}
	for _, c := range cases {
		if got := c.policy.AddressAllowed(netip.MustParseAddr(c.addr), c.host); got != c.want {
			t.Errorf("%+v (%s, %s): %v, want %v", c.policy, c.addr, c.host, got, c.want)
		}
	}
}
