package profiles

import (
	"strings"

	"github.com/marceloalvescf/k8s-capacity-report/internal/platform/netpolicy"
	"github.com/marceloalvescf/k8s-capacity-report/internal/prom"
)

// CanonicalEndpoint applies the static rules of the policy to every
// candidate root of the endpoint and to the /query and /query_range of
// each, and returns the canonical URL of the endpoint. It resolves nothing.
func CanonicalEndpoint(p netpolicy.Policy, endpointURL, endpointType string) (string, error) {
	origin, err := p.ValidateURL(endpointURL)
	if err != nil {
		return "", err
	}
	canonical := origin.URL()
	roots, kind := prom.Candidates(canonical, endpointType)
	if kind == "grafana" && !strings.Contains(origin.Path, "datasource") && !strings.Contains(origin.Path, "proxy") {
		return "", &netpolicy.Error{Code: netpolicy.CodeURLRejected}
	}
	for _, root := range roots {
		for _, u := range []string{root, root + "/query", root + "/query_range"} {
			if _, err := p.ValidateURL(u); err != nil {
				return "", err
			}
		}
	}
	return canonical, nil
}
