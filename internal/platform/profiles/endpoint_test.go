package profiles

import (
	"errors"
	"net/netip"
	"strings"
	"testing"

	"github.com/marceloalvescf/capacity-room/internal/platform/netpolicy"
)

func TestCanonicalEndpointValidatesEveryCandidate(t *testing.T) {
	open := netpolicy.Policy{}
	loop := netpolicy.Policy{Networks: []netip.Prefix{netip.MustParsePrefix("127.0.0.1/32")}}
	// 2035 characters: the root, its /query_range, and /api/v1/query fit in
	// 2048; /api/v1/query_range does not.
	long := "https://metrics.internal/" + strings.Repeat("a", 2010)
	if len(long) != 2035 {
		t.Fatalf("the long root has %d characters", len(long))
	}
	if _, err := open.ValidateURL(long); err != nil {
		t.Fatalf("the long root alone is refused: %v", err)
	}
	cases := []struct {
		policy netpolicy.Policy
		url    string
		want   string
		code   netpolicy.Code
	}{
		{open, "https://metrics.internal", "https://metrics.internal", ""},
		{open, "https://metrics.internal/api/v1", "https://metrics.internal/api/v1", ""},
		{open, "https://metrics.internal/api/v1/../x", "", netpolicy.CodeURLRejected},
		{open, "https://127.0.0.1:9090", "", netpolicy.CodeAddressRejected},
		{loop, "https://127.0.0.1:9090", "https://127.0.0.1:9090", ""},
		{open, long, "", netpolicy.CodeURLRejected},
	}
	for _, c := range cases {
		got, err := CanonicalEndpoint(c.policy, c.url)
		if c.code == "" {
			if err != nil || got != c.want {
				t.Errorf("%s: got %q, %v", c.url, got, err)
			}
			continue
		}
		var pe *netpolicy.Error
		if !errors.As(err, &pe) || pe.Code != c.code {
			t.Errorf("%s: err = %v, want %s", c.url, err, c.code)
		}
	}
}

func TestCanonicalEndpointForm(t *testing.T) {
	got, err := CanonicalEndpoint(netpolicy.Policy{}, "HTTPS://Metrics.Internal.:443/api/v1/")
	if err != nil || got != "https://metrics.internal/api/v1" {
		t.Fatalf("got %q, %v", got, err)
	}
}
