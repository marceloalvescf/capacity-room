package profiles

import (
	"github.com/marceloalvescf/capacity-room/internal/platform/netpolicy"
	"github.com/marceloalvescf/capacity-room/internal/prom"
)

// CanonicalEndpoint applies the static rules of the policy to every
// candidate root of the endpoint and to the /query and /query_range of
// each, and returns the canonical URL of the endpoint. It resolves nothing.
func CanonicalEndpoint(p netpolicy.Policy, endpointURL string) (string, error) {
	origin, err := p.ValidateURL(endpointURL)
	if err != nil {
		return "", err
	}
	canonical := origin.URL()
	roots := prom.Candidates(canonical)
	for _, root := range roots {
		for _, u := range []string{root, root + "/query", root + "/query_range"} {
			if _, err := p.ValidateURL(u); err != nil {
				return "", err
			}
		}
	}
	return canonical, nil
}
