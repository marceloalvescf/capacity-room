package discovery

import (
	"context"
	"fmt"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/marceloalvescf/capacity-room/internal/config"
	"github.com/marceloalvescf/capacity-room/internal/prom"
)

// Querier is the part of the client that discovery needs.
type Querier interface {
	Query(ctx context.Context, query string, at time.Time) ([]prom.Sample, error)
}

// Scope is the resolved visibility of the configured cluster: which values
// of the cluster label are visible on the endpoint, and whether the
// selectors this run builds must carry a matcher for it.
type Scope struct {
	ClusterName      string
	ClusterLabel     string
	FilterMode       string
	MatcherEnabled   bool
	Confidence       string
	DiscoveredValues []string // sorted, never nil
}

// Matchers returns the label and value to add to a selector: one entry
// when the matcher is on, none when it is off.
func (s Scope) Matchers() map[string]string {
	if !s.MatcherEnabled {
		return map[string]string{}
	}
	return map[string]string{s.ClusterLabel: s.ClusterName}
}

// ScopeError is a cluster scope failure. The command maps it to exit
// code 4.
type ScopeError struct{ Message string }

// Error returns the message.
func (e *ScopeError) Error() string { return e.Message }

// ResolveScope decides whether cluster.Name is visible in cluster.Label on
// the endpoint q queries, and whether later queries must carry a matcher
// for it. An error q returns is returned as it is.
func ResolveScope(ctx context.Context, q Querier, cluster config.Cluster, at time.Time) (Scope, error) {
	values, unlabeled, err := labelState(ctx, q, cluster.Label, "kube_node_info", at)
	if err != nil {
		return Scope{}, err
	}
	if len(values) == 0 {
		values, unlabeled, err = labelState(ctx, q, cluster.Label, "up", at)
		if err != nil {
			return Scope{}, err
		}
	}

	switch cluster.FilterMode {
	case "required":
		return requiredScope(cluster, values)
	case "none":
		return noneScope(cluster, values, unlabeled)
	default: // "auto"
		return autoScope(cluster, values)
	}
}

// labelState runs `count by (label) (selector)` at at and reports the sorted,
// deduplicated, non-empty values of label across the returned series, and
// whether any returned series carried no value for label. A series counts
// toward both whatever its sample value is.
func labelState(ctx context.Context, q Querier, label, selector string, at time.Time) (values []string, unlabeled bool, err error) {
	query := fmt.Sprintf("count by (%s) (%s)", label, selector)
	samples, err := q.Query(ctx, query, at)
	if err != nil {
		return nil, false, err
	}

	seen := make(map[string]struct{}, len(samples))
	for _, sample := range samples {
		value := sample.Labels[label]
		if value == "" {
			unlabeled = true
			continue
		}
		seen[value] = struct{}{}
	}

	values = make([]string, 0, len(seen))
	for value := range seen {
		values = append(values, value)
	}
	sort.Strings(values)
	return values, unlabeled, nil
}

// requiredScope resolves a "required" filter mode: cluster.Name must be
// among values.
func requiredScope(cluster config.Cluster, values []string) (Scope, error) {
	if !slices.Contains(values, cluster.Name) {
		return Scope{}, &ScopeError{Message: fmt.Sprintf(
			"cluster %q was not found in label %q; visible values: %s",
			cluster.Name, cluster.Label, joinOrNone(values),
		)}
	}
	return Scope{
		ClusterName:      cluster.Name,
		ClusterLabel:     cluster.Label,
		FilterMode:       cluster.FilterMode,
		MatcherEnabled:   true,
		Confidence:       ConfidenceExact,
		DiscoveredValues: values,
	}, nil
}

// noneScope resolves a "none" filter mode: no matcher is added, but the
// endpoint must show at most one cluster group, and never a mix of labeled
// and unlabeled ones.
func noneScope(cluster config.Cluster, values []string, unlabeled bool) (Scope, error) {
	if len(values) > 0 && unlabeled {
		return Scope{}, &ScopeError{Message: "labeled and unlabeled cluster groups are visible while filter mode is none"}
	}
	if len(values) > 1 {
		return Scope{}, &ScopeError{Message: "multiple cluster values are visible while filter mode is none"}
	}
	return Scope{
		ClusterName:      cluster.Name,
		ClusterLabel:     cluster.Label,
		FilterMode:       cluster.FilterMode,
		MatcherEnabled:   false,
		Confidence:       ConfidenceExact,
		DiscoveredValues: values,
	}, nil
}

// autoScope resolves an "auto" filter mode: the matcher turns on when
// cluster.Name is visible, off with an inferred confidence when nothing is,
// and the mode fails when only other clusters are visible.
func autoScope(cluster config.Cluster, values []string) (Scope, error) {
	if slices.Contains(values, cluster.Name) {
		return Scope{
			ClusterName:      cluster.Name,
			ClusterLabel:     cluster.Label,
			FilterMode:       cluster.FilterMode,
			MatcherEnabled:   true,
			Confidence:       ConfidenceExact,
			DiscoveredValues: values,
		}, nil
	}
	if len(values) > 0 {
		return Scope{}, &ScopeError{Message: fmt.Sprintf(
			"cluster %q was not found in visible label values: %s",
			cluster.Name, strings.Join(values, ", "),
		)}
	}
	return Scope{
		ClusterName:      cluster.Name,
		ClusterLabel:     cluster.Label,
		FilterMode:       cluster.FilterMode,
		MatcherEnabled:   false,
		Confidence:       ConfidenceInferred,
		DiscoveredValues: values,
	}, nil
}

// joinOrNone joins values with ", ", or answers "none" when there are none.
func joinOrNone(values []string) string {
	if len(values) == 0 {
		return "none"
	}
	return strings.Join(values, ", ")
}

// namespacePattern is the Kubernetes DNS-1123 label a namespace name must
// match.
var namespacePattern = regexp.MustCompile(`^[a-z0-9](?:[-a-z0-9]{0,61}[a-z0-9])?$`)

// ListNamespaces returns the namespaces that have pods in scope: the sorted,
// deduplicated values of the namespace label of kube_pod_status_phase that
// are valid Kubernetes names. A lookback of a second or more reads every
// series seen in the lookback that ends at at, so a namespace whose pods went
// away during a period is still found; a shorter one reads the series at at
// alone. It never returns nil. An error q returns is returned as it is.
func ListNamespaces(ctx context.Context, q Querier, scope Scope, at time.Time, lookback time.Duration) ([]string, error) {
	selector := prom.Selector{Metric: "kube_pod_status_phase", Equal: scope.Matchers()}.String()
	if seconds := int64(lookback / time.Second); seconds > 0 {
		selector = "max_over_time(" + selector + "[" + strconv.FormatInt(seconds, 10) + "s])"
	}
	values, _, err := labelState(ctx, q, "namespace", selector, at)
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(values))
	for _, value := range values {
		if namespacePattern.MatchString(value) {
			names = append(names, value)
		}
	}
	return names, nil
}
