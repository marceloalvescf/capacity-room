// Package resolve turns ownership records of pods into workload identities.
package resolve

import "strings"

// Owner is one ownership record: child is owned by the object of
// OwnerKind and OwnerName. OwnerKind and OwnerName are empty when the
// child has no controller.
type Owner struct {
	Namespace string
	Child     string
	OwnerKind string
	OwnerName string
}

// Identity names a workload.
type Identity struct {
	Namespace string
	Kind      string
	Name      string
}

// Pod names a pod.
type Pod struct {
	Namespace string
	Name      string
}

// The workload kinds this package names directly. KindPod is not written
// by Workloads: it names a pod that no ownership record names, for a
// collector to use. KindStandalonePod names a pod with no controller.
const (
	KindJob           = "Job"
	KindPod           = "Pod"
	KindStandalonePod = "StandalonePod"
)

// Workloads resolves every pod owner to the workload identity that owns it.
// A pod with no owner kind or name stands alone under its own name. A pod
// owned by a ReplicaSet takes the owner of that replica set, found among
// replicaSets by namespace and name, when that owner has a kind and a name;
// otherwise it keeps the replica set. Any other owner is kept as given, one
// step and no more. When two records name the same pod, the last one wins.
func Workloads(pods, replicaSets []Owner) map[Pod]Identity {
	parents := make(map[Pod]Owner, len(replicaSets))
	for _, owner := range replicaSets {
		parents[Pod{Namespace: owner.Namespace, Name: owner.Child}] = owner
	}

	resolved := make(map[Pod]Identity, len(pods))
	for _, pod := range pods {
		kind, name := pod.OwnerKind, pod.OwnerName
		switch {
		case kind == "" || name == "":
			kind, name = KindStandalonePod, pod.Child
		case kind == "ReplicaSet":
			if parent, ok := parents[Pod{Namespace: pod.Namespace, Name: name}]; ok {
				if parent.OwnerKind != "" && parent.OwnerName != "" {
					kind, name = parent.OwnerKind, parent.OwnerName
				}
			}
		}
		resolved[Pod{Namespace: pod.Namespace, Name: pod.Child}] = Identity{
			Namespace: pod.Namespace,
			Kind:      kind,
			Name:      name,
		}
	}
	return resolved
}

// canonicalWorkloadKinds maps the lower-case form of each known workload
// kind to its canonical casing.
var canonicalWorkloadKinds = map[string]string{
	"deployment":            "Deployment",
	"statefulset":           "StatefulSet",
	"daemonset":             "DaemonSet",
	"replicaset":            "ReplicaSet",
	"replicationcontroller": "ReplicationController",
	"job":                   KindJob,
	"cronjob":               "CronJob",
	"pod":                   KindPod,
	"standalonepod":         KindStandalonePod,
}

// CanonicalKind returns the canonical Kubernetes casing for a known
// workload kind. A value outside the nine known kinds, the empty string
// included, is returned as given.
func CanonicalKind(kind string) string {
	if canonical, ok := canonicalWorkloadKinds[strings.ToLower(kind)]; ok {
		return canonical
	}
	return kind
}
