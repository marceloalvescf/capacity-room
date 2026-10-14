package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"testing"
	"time"

	"github.com/marceloalvescf/capacity-room/internal/platform/store"
)

func TestThrottleKeyIsHMACWithNUL(t *testing.T) {
	m := hmac.New(sha256.New, []byte("k"))
	m.Write([]byte("admin\x0010.0.0.1"))
	want := m.Sum(nil)
	th := Throttle{Secret: []byte("k")}
	if got := th.Key("Admin", "10.0.0.1"); !hmac.Equal(got, want) {
		t.Fatalf("got %x, want %x", got, want)
	}
	if hmac.Equal(th.Key("adm", "in\x0010.0.0.1"), want) {
		t.Fatal("key is ambiguous across the separator")
	}
}

func TestDecide(t *testing.T) {
	now := time.Date(2026, 10, 5, 12, 0, 0, 0, time.UTC)
	at := func(d time.Duration) *store.Throttle {
		b := now.Add(d)
		return &store.Throttle{BlockedUntil: &b}
	}
	cases := []struct {
		name  string
		row   *store.Throttle
		retry int
		block bool
	}{
		{"nil row", nil, 0, false},
		{"no block", &store.Throttle{}, 0, false},
		{"half second", at(500 * time.Millisecond), 1, true},
		{"299.2 seconds", at(299200 * time.Millisecond), 300, true},
		{"behind", at(-time.Second), 0, false},
	}
	for _, c := range cases {
		retry, blocked := Decide(c.row, now)
		if blocked != c.block || (blocked && retry != c.retry) {
			t.Errorf("%s: got %d, %v", c.name, retry, blocked)
		}
	}
}

func TestThrottleKeyGroupsAnIPv6ClientByItsSlash64(t *testing.T) {
	th := Throttle{Secret: []byte("k")}
	if !hmac.Equal(th.Key("admin", "2001:db8:1:2::1"), th.Key("admin", "2001:db8:1:2:ffff::9")) {
		t.Fatal("two addresses of one /64 have two keys")
	}
	if hmac.Equal(th.Key("admin", "2001:db8:1:2::1"), th.Key("admin", "2001:db8:1:3::1")) {
		t.Fatal("two /64 prefixes share a key")
	}
	if hmac.Equal(th.Key("admin", "192.0.2.1"), th.Key("admin", "192.0.2.2")) {
		t.Fatal("two IPv4 addresses share a key")
	}
}

func TestClientKeyUnmapsIPv4MappedAddresses(t *testing.T) {
	if got := clientKey("::ffff:192.0.2.7"); got != "192.0.2.7" {
		t.Fatalf("mapped = %q", got)
	}
	if clientKey("::ffff:192.0.2.7") == clientKey("::ffff:192.0.2.8") {
		t.Fatal("two mapped addresses share a key")
	}
	if clientKey("::ffff:192.0.2.7") != clientKey("192.0.2.7") {
		t.Fatal("the two forms of one address differ")
	}
	if got := clientKey("2001:db8:1:2:3:4:5:6"); got != "2001:db8:1:2::/64" {
		t.Fatalf("ipv6 = %q", got)
	}
}
