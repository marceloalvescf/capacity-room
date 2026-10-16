# 0002: Findings that differ from the original

Date: 2026-10-13
Status: decided

## Question

The numbers of this project equal those of kubernetes-report 1.11.0, and
`internal/goldentest` compares them with the recorded runs of
`testdata/python` on every test run. Some findings of the original mislead
a reader. Which differences are kept, and how does the comparison know them?

## Decision

Parity binds the numbers. A finding may differ from the original's only
by a decision recorded here. The comparison removes the findings of a
ruled code before it compares, and fails when a ruled code is written by
no recorded scenario.

### CPU variation from one millicore

`NAMESPACE_CPU_BURSTY` and `WORKLOAD_CPU_BURSTY` are judged only when the
CPU average is at least 1 mCPU. The original judges every average above
zero, and a real cluster showed variations of 18813 % and 28800 % on
averages that round to 0 mCPU. No recorded scenario has a CPU average
below 1 mCPU, so the comparison removes nothing for this decision. The
memory codes are unchanged.

### Node utilization partial on incomplete evidence

The node utilization reads Partial while the warning
`NODE_CAPACITY_EVIDENCE_INCOMPLETE` stands, as the HPA history reads
Partial while an HPA is ambiguous. The original keeps it available beside
the warning. The recorded scenario `edge-scaling` raises the warning; its
executive material limitations list the warning as a row for the node
utilization where the original lists it as an interpretation, with the
same coverage and remark. The comparison reads the row as the
interpretation, only when the stored capability is available and the
warning stands.

### Node readiness and schedulability

`NODE_NOT_READY` and `NODE_UNSCHEDULABLE`, of attention, mark a node whose
Ready condition was false, or that was cordoned, at the end of the period.
The original reads both states and writes them in its node table, but
writes no finding, so a node that was not ready read as healthy. The
recorded scenario `edge-scaling` holds one such node, `pool-a-2`; the
comparison removes its two findings and their two recommendations, and
lowers the counts of findings, recommendations, and attention by two.

## Node autoscaling from kube-state-metrics

The original reads node autoscaling from the metrics of the cluster
autoscaler: a probe `count({__name__=~"cluster_autoscaler_.+"})` and six
range queries of `cluster_autoscaler_*`. A managed autoscaler, as on AKS,
exports none of them to a Prometheus inside the cluster, so the original
says on every such cluster that the state could not be determined. This
project reads the status ConfigMap of the cluster autoscaler and the nodes
from kube-state-metrics instead (design, node autoscaling, 2026-10-16).

The comparison with `testdata/python` therefore leaves out, on both sides:
the node autoscaling probe and the collector's queries in the lists of
queries; `summary.autoscaling` and the autoscaling collection state; and
every `NODE_AUTOSCALING_*` finding, with its recommendation, its executive
row, and its count in the severity totals; and the row of the node
autoscaling capability in the executive material limitations. Every other
query, number, and finding is still compared.
