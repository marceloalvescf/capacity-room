package cli

import (
	"net/netip"
	"testing"

	"github.com/marceloalvescf/k8s-capacity-report/internal/platform/settings"
)

func TestOutboundNotice(t *testing.T) {
	cases := []struct {
		name   string
		loaded settings.Settings
		want   string
	}{
		{"both empty", settings.Settings{}, "serve: the outbound allowlists are empty; no metric source is reachable"},
		{"a host name", settings.Settings{AllowedEndpointHostnames: []string{"metrics.example"}}, ""},
		{"a network", settings.Settings{AllowedEndpointCIDRs: []netip.Prefix{netip.MustParsePrefix("10.0.0.0/8")}}, ""},
	}
	for _, c := range cases {
		if got := outboundNotice(c.loaded); got != c.want {
			t.Errorf("%s: outboundNotice = %q, want %q", c.name, got, c.want)
		}
	}
}
