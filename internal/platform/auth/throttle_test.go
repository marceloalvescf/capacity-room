package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"testing"
	"time"

	"github.com/marceloalvescf/k8s-capacity-report/internal/platform/store"
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
