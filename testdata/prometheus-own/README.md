# This project's own Prometheus scenarios

Neither file here is a recorded answer of the original Python tool. Each
was written by this project, and the original was then run against it so
that both tools read the same answers.

## `edge-values.json`

`edge-values.json` is not a recorded answer of the original Python tool. No
tool recorded it: this project wrote it, as a copy of the requests of
`testdata/prometheus/direct-complete.json` in which the answers of the
fourteen queries of the two collectors are set, one series with a sample
that is not a number, one with an
infinite sample, one base of zero, one base that lacks a timestamp, one
absent base, one workload with no consumption, one workload of kind `Job`,
and one workload type written in lower case.

Thirteen of the fourteen answers differ from those of `direct-complete`.
The fourteenth, the pods of the namespace, holds the same bytes, because
both scenarios answer `3`. The 56 other requests are those of
`direct-complete`, byte for byte.

The timestamps of every series of a range answer are `t1` = 1784050200 and
`t2` = 1784093400. Every capability stays available, with the recording
rule, exactly as `direct-complete` has it.

### Namespace `payments`

| Query | Series and points | What it exercises |
|---|---|---|
| CPU usage | one series: `t1` `0.5`, `t2` `NaN` | a sample that is not a number |
| memory usage | two series of the namespace: `t1` `100`, `t2` `+Inf`; and `t1` `50`, `t2` `60` | two series of one key; an infinite sample |
| requests of CPU | `t1` `0` | a base of zero |
| requests of memory | `t1` `200` | a base that lacks `t2` |
| limits of CPU | no series | an absent base |
| limits of memory | `t1` `300`, `t2` `300` | — |
| pods | `3`, as in `direct-complete` | — |

### Workloads, all of namespace `payments`

| Workload | Type as answered | CPU usage | Memory usage | Requests and limits | Pod count |
|---|---|---|---|---|---|
| `api` | `Deployment` | `0.2`, `0.9` | `100`, `120` | CPU requests `1`, `1`; CPU limits `1`, `1`; memory requests `400`, `400`; memory limits `800`, `800` | 4 |
| `queue` | `statefulset` | `0.1`, `0.1` | `10`, `10` | no series of any of the four | 2 |
| `idle` | `Deployment` | no series | no series | CPU requests `1`, `1` | 1 |
| `nightly` | `Job` | `0.3`, `0.3` | `30`, `30` | none | 1 |

## `edge-scaling.json`

Written by this project on 2026-09-30, as a copy of the requests of
`testdata/prometheus/direct-complete.json` in which the answers of the
HPAs, the coverage inventory, the nodes, and the cluster autoscaling are
set to the values at the edges of those collectors, and the CPU usage of
the namespace holds three points. It holds 70 requests: the probe of
`kube_deployment_labels` answers nothing, so a probe of
`kube_deployment_spec_replicas` follows it; the second answer of
`kube_horizontalpodautoscaler_info` of `direct-complete`, which no tool
asks, is left out. Every capability stays available, with the recording
rule for node utilisation, except `workloads_without_hpa`, which finds the
replicas family for the deployments.

The timestamps of a range answer are `t1` = 1784050200, `tm` = 1784071800,
and `t2` = 1784093400; an instant answer is at 1784136600.

### Namespace `payments`

The CPU usage series holds three points, `t1` `3.3`, `tm` `1.1`, `t2`
`2.2`: an average that a plain sum gets wrong. Everything else is as in
`direct-complete`.

### HPAs, namespace `payments`

| HPA | Target | Spec min | Spec max | Current | Desired | What it exercises |
|---|---|---|---|---|---|---|
| `web` | `Deployment`/`payments-web` | `2` | `4` | `2`, `4` | `2`, `4` | the maximum reached; the measured workload covered |
| `fixed` | `Deployment`/`fixed` | `3` | `3` | `3`, `3` | `3`, `3` | a pinned HPA |
| `idle` | `Deployment`/`idle` | `1` | `5` | `0`, `0` | `1`, `1` | scaled to zero, hidden from the table; a divergence of 1 |
| `dup` | `Deployment`/`dup` | `1` | two series, `4` and `6` | `1`, `2` | `1`, `2` | an ambiguous spec |

Coverage: `kube_deployment_spec_replicas` answers `payments-web` `3`,
`api` `2`, `batch` `0`; `kube_statefulset_labels` answers `queue`.

### Nodes

| Node | Info | Labels | Capacity cpu/memory | Allocatable | Ready | Unschedulable | Requests cpu/memory | CPU usage | Memory usage | What it exercises |
|---|---|---|---|---|---|---|---|---|---|---|
| `pool-a-1` | ip `10.0.0.1`, hostname `pool-a-1` | `label_agentpool` `pool-a` | `4` / `8e9` | `4` / `7e9` | `true` | `0` | `3.6` / `3.5e9` | `20`, `20` | `25`, `25` | a request high at critical, over requested, low utilisation |
| `pool-a-2` | ip `10.0.0.2` | `label_agentpool` `pool-a` | `4` / `8e9` | `4` / `7e9` | `false` | `1` | `1` / `1e9` | `55`, `60` | `50`, `55` | not ready, not schedulable; the uneven pool |
| `aks-agentpool-12345678-vmss000000` | ip `10.0.0.3` | none | `8` / `16e9` | `8` / `15e9` | `true` | `0` | `2` / `3e9` | `90`, `95`, by `instance` `10.0.0.3:9100` with no `node` label | `70`, `72`, likewise | the pool from the name; resolution by IP; a critical usage |
| `orphan` | none | none | `2` / `4e9` | none | none | none | `0.5` / none | none | none | a node known by other series only; the pool `unknown` |

`node_uname_info` names `pool-a-1` by its `node` label and the other two
by their instance and `nodename` only. The condition answer of `pool-a-1`
and `pool-a-2` holds the three statuses, one of them `1`.

### Cluster autoscaling, in `auto` mode

| Metric | Series |
|---|---|
| `node_groups_count` | `pool-a` `1`, `1`; `agentpool` `1`, `1` |
| `nodes_count` | `pool-a` `2`, `2`; `agentpool` `1`, `1` |
| `node_group_min_nodes_count` | `pool-a` `1`, `1`; `agentpool` `1`, `1` |
| `node_group_max_nodes_count` | `pool-a` `2`, `2`; `agentpool` `1`, `1` |
| `scaled_up_nodes_total` | `pool-a` `0`, `1` |
| `unschedulable_pods_count` | one series with no group label, `0`, `2` |

Do not edit a file here once its recorded numbers exist. The comparison with
the original tool depends on both tools reading the very same answers.

## SHA-256

```text
16f1b408e251371310b884349c1d98380e77ea6b780e28268b6c0b85bbf1c947  edge-scaling.json
53c1a5cb3e5c04c6623669993192ca4ab94162cfc26735082b9f1b264eb1c6e1  edge-values.json
```
