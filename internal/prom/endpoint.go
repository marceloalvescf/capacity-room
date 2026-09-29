// Package prom is the only package of this module that talks HTTP: it
// carries a query to a Prometheus-compatible endpoint and turns the answer
// into samples, or into one of a fixed set of failures.
package prom

import (
	"net/url"
	"strings"
)

// Candidates returns the API roots to try, in order, and the endpoint
// kind, "prometheus" or "grafana". The address was validated by config.
//
// The kind is the given endpointType when it is not "auto". Otherwise it is
// "grafana" when the path holds "datasource" or "/proxy/", and
// "prometheus" otherwise. A second root, the first with "/api/v1"
// appended, is added unless the path already ends in "/api/v1" or holds
// "/api/v1/".
func Candidates(address, endpointType string) (roots []string, kind string) {
	normalized := normalizeRoot(address)
	path := pathOf(normalized)

	kind = endpointType
	if kind == "auto" {
		if strings.Contains(path, "datasource") || strings.Contains(path, "/proxy/") {
			kind = "grafana"
		} else {
			kind = "prometheus"
		}
	}

	candidates := []string{normalized}
	if !strings.HasSuffix(path, "/api/v1") && !strings.Contains(path, "/api/v1/") {
		candidates = append(candidates, normalized+"/api/v1")
	}

	return dedupe(candidates), kind
}

// normalizeRoot strips every trailing slash from the path of address,
// leaving the rest, and any slash that is not at the very end, untouched:
// the original strips every one of them too (Python's str.rstrip removes
// every trailing occurrence, not just one), and a single TrimSuffix call
// only ever removes the last.
func normalizeRoot(address string) string {
	parsed, err := url.Parse(address)
	if err != nil {
		return strings.TrimRight(address, "/")
	}
	parsed.Path = strings.TrimRight(parsed.Path, "/")
	return parsed.String()
}

// pathOf returns the path component of address, or "" when address does
// not parse.
func pathOf(address string) string {
	parsed, err := url.Parse(address)
	if err != nil {
		return ""
	}
	return parsed.Path
}

// dedupe returns items with a later repeat of an earlier value removed,
// keeping the first occurrence's position.
func dedupe(items []string) []string {
	seen := make(map[string]bool, len(items))
	result := make([]string, 0, len(items))
	for _, item := range items {
		if seen[item] {
			continue
		}
		seen[item] = true
		result = append(result, item)
	}
	return result
}
