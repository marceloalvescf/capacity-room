package profiles

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/marceloalvescf/k8s-capacity-report/internal/discovery"
	"github.com/marceloalvescf/k8s-capacity-report/internal/platform/netpolicy"
	"github.com/marceloalvescf/k8s-capacity-report/internal/prom"
)

func TestClassify(t *testing.T) {
	cases := []struct {
		name string
		err  error
		code string
		ok   bool
	}{
		{"policy", fmt.Errorf("wrapped: %w", &netpolicy.Error{Code: netpolicy.CodeAddressRejected}), "ENDPOINT_ADDRESS_REJECTED", true},
		{"credential", &SourceError{Code: CodeCredentialUnavailable}, "PROFILE_CREDENTIAL_UNAVAILABLE", true},
		{"ca bundle", &SourceError{Code: CodeCABundleUnavailable}, "PROFILE_CA_BUNDLE_UNAVAILABLE", true},
		{"tls", &prom.Error{Kind: prom.KindTLS}, "ENDPOINT_TLS_FAILED", true},
		{"authentication", &prom.Error{Kind: prom.KindAuthentication}, "ENDPOINT_AUTH_REJECTED", true},
		{"authorization", &prom.Error{Kind: prom.KindAuthorization}, "ENDPOINT_AUTH_REJECTED", true},
		{"not found", &prom.Error{Kind: prom.KindEndpointNotFound}, "ENDPOINT_API_NOT_FOUND", true},
		{"other kind", &prom.Error{Kind: prom.KindMetrics}, "ENDPOINT_UNREACHABLE", true},
		{"scope", &discovery.ScopeError{Message: "x"}, "CLUSTER_SCOPE_NOT_FOUND", true},
		{"insufficient", fmt.Errorf("w: %w", discovery.ErrInsufficient), "CAPABILITIES_INSUFFICIENT", true},
		{"deadline", fmt.Errorf("w: %w", context.DeadlineExceeded), "ENDPOINT_UNREACHABLE", true},
		{"cancelled", context.Canceled, "ENDPOINT_UNREACHABLE", true},
		{"unknown", errors.New("boom"), "", false},
		{"policy over source", errors.Join(&SourceError{Code: CodeCredentialUnavailable}, &netpolicy.Error{Code: netpolicy.CodeURLRejected}), "ENDPOINT_URL_REJECTED", true},
		{"source over prom", errors.Join(&prom.Error{Kind: prom.KindTLS}, &SourceError{Code: CodeCABundleUnavailable}), "PROFILE_CA_BUNDLE_UNAVAILABLE", true},
		{"prom over scope", errors.Join(&discovery.ScopeError{Message: "x"}, &prom.Error{Kind: prom.KindTLS}), "ENDPOINT_TLS_FAILED", true},
		{"scope over insufficient", errors.Join(discovery.ErrInsufficient, &discovery.ScopeError{Message: "x"}), "CLUSTER_SCOPE_NOT_FOUND", true},
		{"insufficient over context", errors.Join(context.Canceled, discovery.ErrInsufficient), "CAPABILITIES_INSUFFICIENT", true},
		{"policy over context", errors.Join(context.DeadlineExceeded, &netpolicy.Error{Code: netpolicy.CodeResolutionFailed}), "ENDPOINT_RESOLUTION_FAILED", true},
	}
	for _, c := range cases {
		code, ok := Classify(c.err)
		if code != c.code || ok != c.ok {
			t.Errorf("%s: Classify = %q, %v; want %q, %v", c.name, code, ok, c.code, c.ok)
		}
	}
}

func TestSourceErrorCarriesOnlyTheCode(t *testing.T) {
	if got := (&SourceError{Code: CodeCredentialUnavailable}).Error(); got != "PROFILE_CREDENTIAL_UNAVAILABLE" {
		t.Fatalf("got %q", got)
	}
}
