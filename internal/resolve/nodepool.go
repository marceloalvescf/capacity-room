package resolve

import (
	"regexp"
	"strings"
)

// The confidence of a finding. The values equal the ones of the discovery
// package.
const (
	ConfidenceExact    = "exact"
	ConfidenceInferred = "inferred"
	ConfidenceUnknown  = "unknown"
)

// PoolUnknown is the pool of a node that no step identifies.
const PoolUnknown = "unknown"

// KnownPoolLabels are the labels tried after the configured one, in order.
var KnownPoolLabels = []string{
	"label_kubernetes_azure_com_agentpool",
	"label_agentpool",
	"label_cloud_google_com_gke_nodepool",
	"label_eks_amazonaws_com_nodegroup",
}

// nameLabel is not a provider pool label: on many clusters it carries the
// name of the node itself. It names a pool only when more than one node
// carries the same value.
const nameLabel = "label_name"

var aksName = regexp.MustCompile(`^aks-([a-z0-9]+?)-[0-9]+-vmss`)

// SharedNameLabels returns the values of label_name that more than one node
// carries.
func SharedNameLabels(labelsByNode map[string]map[string]string) map[string]bool {
	counts := make(map[string]int)
	for _, labels := range labelsByNode {
		if v := labels[nameLabel]; v != "" {
			counts[v]++
		}
	}
	shared := make(map[string]bool)
	for v, n := range counts {
		if n > 1 {
			shared[v] = true
		}
	}
	return shared
}

// NodePool returns the pool of a node, where it came from, and the
// confidence. The steps run in order: the configured label, the known
// labels, the AKS node name, the OKE node name, a shared label_name, and
// last the unknown pool.
func NodePool(nodeName string, labels map[string]string, configuredLabel string, sharedNames map[string]bool) (pool, source, confidence string) {
	if configuredLabel != "" && labels[configuredLabel] != "" {
		return labels[configuredLabel], configuredLabel, ConfidenceExact
	}
	for _, label := range KnownPoolLabels {
		if labels[label] != "" {
			return labels[label], label, ConfidenceExact
		}
	}
	if m := aksName.FindStringSubmatch(nodeName); m != nil {
		return m[1], "aks-node-name", ConfidenceInferred
	}
	if parts := strings.Split(nodeName, "-"); len(parts) >= 4 && parts[0] == "oke" {
		return "pool-" + parts[2], "oke-node-name", ConfidenceInferred
	}
	if v := labels[nameLabel]; v != "" && sharedNames[v] {
		return v, nameLabel, ConfidenceInferred
	}
	return PoolUnknown, "none", ConfidenceUnknown
}
