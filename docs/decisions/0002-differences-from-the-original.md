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
