package profiles

import (
	"errors"
	"net/netip"
	"testing"

	"github.com/marceloalvescf/k8s-capacity-report/internal/platform/netpolicy"
)

func TestCanonicalEndpointValidatesEveryCandidate(t *testing.T) {
	open := netpolicy.Policy{}
	loop := netpolicy.Policy{Networks: []netip.Prefix{netip.MustParsePrefix("127.0.0.1/32")}}
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
	}
	for _, c := range cases {
		got, err := CanonicalEndpoint(c.policy, c.url, "auto")
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

func TestCanonicalEndpointGrafanaNeedsDatasource(t *testing.T) {
	var pe *netpolicy.Error
	_, err := CanonicalEndpoint(netpolicy.Policy{}, "https://grafana.internal", "grafana")
	if !errors.As(err, &pe) || pe.Code != netpolicy.CodeURLRejected {
		t.Fatalf("err = %v", err)
	}
	got, err := CanonicalEndpoint(netpolicy.Policy{}, "https://grafana.internal/api/datasources/proxy/1", "grafana")
	if err != nil || got != "https://grafana.internal/api/datasources/proxy/1" {
		t.Fatalf("got %q, %v", got, err)
	}
}

func TestCanonicalEndpointForm(t *testing.T) {
	got, err := CanonicalEndpoint(netpolicy.Policy{}, "HTTPS://Metrics.Internal.:443/api/v1/", "auto")
	if err != nil || got != "https://metrics.internal/api/v1" {
		t.Fatalf("got %q, %v", got, err)
	}
}
