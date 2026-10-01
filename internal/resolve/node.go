package resolve

import (
	"net/netip"
	"strings"
)

// tabsAndBreaks removes the characters urlsplit deletes from its input.
var tabsAndBreaks = strings.NewReplacer("\t", "", "\r", "", "\n", "")

// InstanceHost returns the host of an instance label, as the original does
// with the host of "//" plus the instance: the port, IPv6 brackets, and any
// user information removed, the host in lower case but for the zone of an
// IPv6 address, which keeps its case; the text itself when it
// has no host part; the empty text for an empty instance or a malformed one.
func InstanceHost(instance string) string {
	if instance == "" {
		return ""
	}
	netloc := tabsAndBreaks.Replace(instance)
	if i := strings.IndexAny(netloc, "/?#"); i >= 0 {
		netloc = netloc[:i]
	}
	open, closed := strings.Contains(netloc, "["), strings.Contains(netloc, "]")
	if open != closed {
		return ""
	}
	if i := strings.LastIndex(netloc, "@"); i >= 0 {
		netloc = netloc[i+1:]
	}
	var host string
	if open {
		_, rest, _ := strings.Cut(netloc, "[")
		var after string
		host, after, _ = strings.Cut(rest, "]")
		if after != "" && after[0] != ':' {
			return ""
		}
		if addr, err := netip.ParseAddr(host); (err != nil || !addr.Is6()) && !strings.HasPrefix(host, "v") && !strings.HasPrefix(host, "V") {
			return ""
		}
	} else {
		host, _, _ = strings.Cut(netloc, ":")
	}
	if host == "" {
		return instance
	}
	addr, zone, hasZone := strings.Cut(host, "%")
	if hasZone {
		return strings.ToLower(addr) + "%" + zone
	}
	return strings.ToLower(host)
}

// NodeRecord is what the index knows of a node.
type NodeRecord struct {
	Name       string
	InternalIP string
	Hostname   string
}

// NodeIndex resolves a series to a node by its node label, then by the host
// of its instance label against the internal IP, the hostname, and the
// name, in that order. A value that two nodes share resolves to no node.
type NodeIndex struct {
	byName     map[string]NodeRecord
	byIP       map[string]NodeRecord
	byHostname map[string]NodeRecord
}

// NewNodeIndex indexes nodes by name, internal IP, and hostname. Empty
// values are not indexed.
func NewNodeIndex(nodes []NodeRecord) NodeIndex {
	return NodeIndex{
		byName:     uniqueIndex(nodes, func(n NodeRecord) string { return n.Name }),
		byIP:       uniqueIndex(nodes, func(n NodeRecord) string { return n.InternalIP }),
		byHostname: uniqueIndex(nodes, func(n NodeRecord) string { return n.Hostname }),
	}
}

// uniqueIndex maps each value to its node. A value seen on a second,
// different node is removed and remembered, so that a third node with it is
// skipped too.
func uniqueIndex(nodes []NodeRecord, value func(NodeRecord) string) map[string]NodeRecord {
	index := make(map[string]NodeRecord)
	ambiguous := make(map[string]bool)
	for _, node := range nodes {
		v := value(node)
		if v == "" || ambiguous[v] {
			continue
		}
		previous, seen := index[v]
		switch {
		case !seen:
			index[v] = node
		case previous != node:
			delete(index, v)
			ambiguous[v] = true
		}
	}
	return index
}

// Resolve returns the name of the node of a series: the node label as
// given against the names, else the host of the instance against the IPs,
// the hostnames, and the names, in that order.
func (x NodeIndex) Resolve(nodeLabel, instance string) (name string, ok bool) {
	if node, found := x.byName[nodeLabel]; found {
		return node.Name, true
	}
	host := InstanceHost(instance)
	for _, index := range []map[string]NodeRecord{x.byIP, x.byHostname, x.byName} {
		if node, found := index[host]; found {
			return node.Name, true
		}
	}
	return "", false
}
