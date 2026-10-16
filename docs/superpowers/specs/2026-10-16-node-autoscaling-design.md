# Phase 14: node autoscaling from kube-state-metrics

Date: 2026-10-16. Status: approved in conversation by the owner, written for review.

## Purpose

The report tells whether the nodes of the cluster scale on their own. Today it
knows only from the metrics of the cluster autoscaler (`cluster_autoscaler_*`),
which the original program asked for. On AKS the cluster autoscaler runs in the
managed control plane, and a Prometheus inside the cluster cannot scrape it
(Microsoft's FAQ of control plane metrics). On OKE the add-on runs in
`kube-system` but nobody scrapes it unless the operator adds a scrape. So on the
clusters the owner reports on, every report says that the state of node
autoscaling could not be determined: a finding of attention, a partial
capability, and a line in the data quality appendix.

This phase reads node autoscaling from kube-state-metrics alone, as
kube-prometheus-stack ships it, so that a report on AKS or OKE says enabled or
disabled. It asks nothing of the operator. It also pins that a report PDF
holds no date, so a render is the same bytes at any time (F-269). The phase
ends as release 2.4.0.

## Decisions of the owner (2026-10-15 and 2026-10-16)

- Node autoscaling means the cluster autoscaler and the nodes it adds, not the
  HPAs.
- The program reads only what kube-prometheus-stack provides. Scraping the
  cluster autoscaler, an exporter of its status ConfigMap, and Azure Monitor
  managed Prometheus are ruled out. The six `cluster_autoscaler_*` queries and
  their probe leave the program.
- Dropping the section of node autoscaling from the report was weighed and
  declined: it removes public surface, so it is a major version, and the
  snapshot schema would change.
- Option A for what kube-state-metrics cannot give: the findings
  `NODE_AUTOSCALING_NEAR_MAX`, `NODE_AUTOSCALING_AT_MIN`, and
  `NODE_AUTOSCALING_BOUNDS_UNKNOWN`, the fields `min_nodes`, `max_nodes`, and
  `managed_node_groups`, and the gauge of nodes against the maximum stay in the
  contract and are never produced. The snapshot keeps schema `2.0`, a stored
  report still loads, and nothing is migrated.
- The operator configures nothing for this. The stock kube-prometheus-stack
  collects ConfigMaps, and the guide says so in a paragraph.
- The real-cluster checks run on an AKS cluster alone; the OKE clusters are
  reachable only from the owner's work laptop.
- F-269 joins the phase (section 9). The owner first took a fixed creation
  timestamp, the snapshot's emission; a measure during the work showed a
  report PDF holds no date at all, so the fix moved to the test (2026-10-16).

## 1. The signals

The cluster autoscaler writes its status to the ConfigMap
`kube-system/cluster-autoscaler-status`. Writing it is the autoscaler's
default (`--write-status-configmap`, name `cluster-autoscaler-status`); AKS
documents the ConfigMap for its managed autoscaler, and the OKE add-on writes
it by default (`writeStatusConfigmap: true`). The autoscaler rewrites it at
every loop, so its resource version keeps changing while it runs.

kube-state-metrics exports, for every ConfigMap, with no allowlist:

| Metric | Stability | Tells |
|---|---|---|
| `kube_configmap_info{namespace, configmap}` | STABLE | the ConfigMap exists |
| `kube_configmap_metadata_resource_version{namespace, configmap}` | EXPERIMENTAL | its resource version, as the value |

Its `configmaps` collector is on by default, in kube-state-metrics and in the
kube-state-metrics chart that kube-prometheus-stack uses, which does not
override the list. The program reads neither the data nor the labels nor the
annotations of the ConfigMap, so its format does not matter and no
`--metric-*-allowlist` is needed.

The other numbers come from metrics that kube-prometheus-stack collects by
default: `kube_node_info` (the nodes), `kube_node_created` (the creation time
of a node, as the value), and `kube_pod_status_unschedulable` (a pod the
scheduler cannot place).

## 2. The probe and the capability

The preflight keeps nine probes and the same order. The ninth, the last
request of a preflight, becomes `count(kube_configmap_info)`, with the scope's
cluster matcher when it is on, at the end of the window. It asks whether
kube-state-metrics collects ConfigMaps at all, not whether the autoscaler
runs; the collector answers that.

The capability `node_autoscaling` is:

- `available`, `configured`, when the configuration states `enabled` or
  `disabled`, as today;
- `available`, `raw_metrics`, evidence `kube_configmap_info`, when the probe
  finds a ConfigMap;
- `partial`, `none`, reason `NODE_AUTOSCALING_UNDETERMINED`, otherwise.

`--check` still sends no query of a collector.

## 3. The collector

The collector keeps its place, last of the five, and sends nothing when the
configuration states the mode, or when the probe found no ConfigMap.
Otherwise it sends five instant queries at the end of the window, in this
order, each with the scope's matchers (`W` is the window's length in whole
seconds, at least 1, as the span of the `increase` of today; `S` is the
larger of `W` and 300, the lookback of an instant selector, so that a window
of a few minutes still holds samples of the ConfigMap):

1. whether the status ConfigMap appeared in the window:
   `max_over_time(kube_configmap_info{configmap="cluster-autoscaler-status"}[Ss])`;
2. how often it was written:
   `changes(kube_configmap_metadata_resource_version{configmap="cluster-autoscaler-status"}[Ss])`;
3. the nodes at the end: `count(count by (node) (kube_node_info))`;
4. the nodes added in the window, each once, a node removed before the end
   included: `count(count by (node) (max_over_time(kube_node_created[Ws]) > time() - W))`;
5. the unschedulable pods at the end:
   `count(count by (namespace, pod) (kube_pod_status_unschedulable == 1))`.

The ConfigMap is selected by its name alone: the cluster autoscaler writes
it in kube-system on AKS and in the OKE add-on, and in its own namespace when
installed by hand (amended 2026-10-16 after the whole-branch review).

Each query is instant because a window aggregate is wanted at the end
alone: a range of `count(... > ...)` has gaps, and its last point would be
an earlier step. The inner `count by` counts a node or a pod once when two
replicas of kube-state-metrics export it, and an answer of several samples
(one per replica) reads as the largest.

**The status.** With the mode `auto` and a ConfigMap found by the probe:

- `enabled` when the status ConfigMap was written during the window (query 2
  above zero), or, when kube-state-metrics exports no resource version for
  it (query 2 empty), when it appeared in the window (query 1);
- `disabled` otherwise: the ConfigMap never appeared in the window, or it
  appeared and nobody wrote it, the mark of an autoscaler that was turned
  off and left its ConfigMap behind.

With no ConfigMap found by the probe, the status is `unknown`, with no
count. The source is `raw_metrics` and the confidence `inferred` for both
enabled and disabled; the reader of the snapshot accepts the new
combination disabled, `raw_metrics`, `inferred`, an addition to schema
`2.0`. A configured mode keeps the source `configured` and the confidence
`exact`.

**The evidence**, for enabled and disabled alike: `current_nodes` is query
3, `scale_events` query 4, and `unschedulable_pods` query 5. When query 3
answers, an empty query 4 or 5 is zero; when it does not, all three are
unknown. `managed_node_groups`, `min_nodes`, and `max_nodes` are always
unknown. The meaning of `scale_events` holds: the original counted the
nodes the autoscaler added (`cluster_autoscaler_scaled_up_nodes_total`), and
this counts the nodes added, by whatever added them.

The exit code 5 rule is unchanged: a run whose five collectors give no row and
whose autoscaling status is `unknown` stops.

## 4. Findings and wording

The analysis produces one of four findings for node autoscaling:
`NODE_AUTOSCALING_UNKNOWN`, `NODE_AUTOSCALING_DISABLED`,
`NODE_AUTOSCALING_DISABLED_PRESSURE`, and `NODE_AUTOSCALING_ENABLED`. An
enabled status always gives `NODE_AUTOSCALING_ENABLED`; the branches of near
maximum, at minimum, and bounds unknown leave the analysis. Their three codes
stay in the registry and in both catalogs, so a stored report that holds one
still loads and still renders its PDF.

The wording that is no longer true changes, in both languages:

| Key | Today (en) | Proposed (en) | Proposed (pt-BR) |
|---|---|---|---|
| `finding.autoscaling_disabled.summary` | Node autoscaling is configured as disabled. | Node autoscaling is disabled. | O autoscaling de nodes está desabilitado. |
| `finding.autoscaling_enabled.summary` | Node autoscaling enabled with observed headroom. | Node autoscaling is enabled. | O autoscaling de nodes está habilitado. |
| `finding.autoscaling_enabled.action` | Keep following the bounds and the scaling events. | Keep following the nodes added and the unschedulable pods. | Manter acompanhamento dos nodes adicionados e dos pods não agendáveis. |
| `finding.autoscaling_unknown.action` | Confirm operationally whether cluster autoscaling is enabled. | Check that kube-state-metrics collects ConfigMaps, or state the node autoscaling in the configuration. | Verificar se o kube-state-metrics coleta ConfigMaps, ou informar o autoscaling de nodes na configuração. |
| `verdict.growth.scale_events` and the web's `autoscaling.scaleEvents` | Scale events | Nodes added | Nodes adicionados |
| `verdict.growth.state.enabled` (new) | | Enabled | Ativo |
| `verdict.growth.current_nodes` (new) | | Current nodes | Nodes atuais |

The disabled sentence holds for a configured mode too. The `NODE_AUTOSCALING_UNDETERMINED`
reason keeps its sentence: it now appears only when ConfigMaps are not
collected.

## 5. The report shows what is known

A number that is unknown is left out, never drawn as the word Unavailable
or as a hyphen, so a report from kube-state-metrics shows no empty maximum,
minimum, or count of node groups. A stored report that holds them still
shows them.

- **The verdict.** The PDF shows node autoscaling in its verdict alone (page
  1, in both profiles); it has no section of its own. The web shows it in the
  overview. The pair current against maximum, with the gauge, is drawn when
  the status is enabled and both numbers are known, followed by the facts
  that are known among the minimum, the unschedulable pods, and the nodes
  added, as today. With an enabled status and no maximum: the state word
  Enabled, then the facts that are known among the current nodes, the
  unschedulable pods, and the nodes added. A disabled or unknown status shows
  its state word alone, as today. The template needs no change: its block
  already draws the state word when there is no current count.
- **The autoscaling tab** of the web: the status, the source, and the
  confidence, then only the counts that are known.
- The view carries the same numbers as today, null where unknown; the
  contract does not change.

## 6. Parity, fixtures, and tests

**A deliberate difference from the original.** The original asks the six
`cluster_autoscaler_*` queries and the regex probe. This program no longer
does, so the rules that the text of every query equals the original and that
the probes keep the original's order gain one named exception: the node
autoscaling probe and collector. `TestQueriesEqualTheOriginal` and the
comparison of the autoscaling evidence with `testdata/python` record the
difference rather than compare it: a section of
`docs/decisions/0002-differences-from-the-original.md` names it, and the
comparison leaves the node autoscaling queries, the autoscaling evidence,
and the `NODE_AUTOSCALING_*` findings out on both sides, as it leaves out
the readiness findings of that decision. The rest of the list still equals
the original letter for letter.

**Recorded answers stay untouched.** The files of `testdata/prometheus/` are
never edited. The answers to the new queries for each scenario live in a new
folder, `testdata/prometheus-added/<scenario>.json`, in the format of the
recorded files, with a README that says what each answer exercises. The fake
server and every loader read the recorded file and its added file as one
scenario, the added answers first, so that a recorded answer never takes a
new query. The answers of the seven old autoscaler requests stay in the
recorded files and go unused, so the count of unused answers that `make e2e`
checks becomes 8 for `direct-complete` (`remaining: 1` today), pinned by its
test.

**The scenarios cover every status:**

| Status | Covered by |
|---|---|
| enabled, ConfigMap written in the window | `direct-complete`, `grafana-authenticated`, `thanos-multicluster` (with the cluster matcher) |
| enabled, no resource version exported, ConfigMap present | `raw-fallback` |
| disabled, no ConfigMap, empty answers read as zero | `edge-values` |
| disabled with node pressure, ConfigMap left unwritten | `edge-scaling` |
| unknown, no ConfigMap collected | `partial-data` |
| configured | `value-edge-cases`, unchanged |

The golden snapshots, query lists, data files, views, and pages are rewritten
by `make update-testdata` and `make update-golden`, and every changed file is
read before it is committed. F-270 closes with the old queries.

## 7. Documents

- `docs/operations.md`: what node autoscaling needs (kube-state-metrics with
  its `configmaps` collector, on by default in kube-prometheus-stack), what
  each status means, and the two cases that need the configuration: a trimmed
  collector list, and an OKE add-on whose `statusConfigMapName` was changed.
- The design spec: the probe, the collector, the findings, and the exception
  to the parity of queries.
- AGENTS.md: the rule of the collectors' order and of query text names the
  exception.
- README.md: the lines on cluster autoscaling.
- CHANGELOG.md: under Changed, that node autoscaling is read from
  kube-state-metrics, so a report on a cluster whose autoscaler is managed
  (AKS) or not scraped (OKE) says enabled or disabled, and that the report
  no longer shows the autoscaler's minimum and maximum nodes or the gauge
  against the maximum.

## 8. Real clusters

Every step here touches a work cluster, so each waits for the owner's word
and names its target. Only read-only queries through the cluster's
Prometheus are sent, through a `kubectl port-forward`, as the run against a
real cluster of 2026-10-02 did. No name, address, or context of a work
cluster enters a tracked file; the record says "an AKS cluster".

Only an AKS cluster is reachable from the host this project is developed on.
The OKE clusters are reachable only from the owner's work laptop, so no step
of this phase runs on OKE; the owner may run the same queries there later,
and a contradiction found then is a finding of its own.

- **Before implementation**, on an AKS cluster, ideally with the autoscaler
  on: `count(kube_configmap_info)`, the status ConfigMap's
  `kube_configmap_info`, and `changes(kube_configmap_metadata_resource_version{...}[1h])`.
  A result that contradicts section 1 or the status rule of section 3 amends
  this spec before any code is written.
- **After implementation**, a `--check` and a report against the same
  cluster: the capability is available, and the status is enabled or
  disabled as the cluster is.

## 9. The dates of a PDF (F-269)

Typst writes a creation date into a PDF (`CreationDate`, `ModDate`, and the
XMP dates) unless the document sets `date: none`. With no
`--creation-timestamp` it takes `SOURCE_DATE_EPOCH`, and without that the
current second; the toolchain runs typst with an empty environment, so it
takes the current second. `TestRealCompileIsDeterministic` compiles a
document that sets no date, so it failed `make gate` whenever its two
compiles straddled a second.

The report's template sets `date: none` (`template/graphite/lib.typ`), so a
report PDF holds no date: compiles with any timestamp, or none, give the same
bytes (measured 2026-10-16). Nothing in the product changes. The determinism
test compiles a document that sets `date: none`, as the report does, twice
1.1 seconds apart, and finds equal bytes and no date; a test tagged
`template` does the same with a real report data file, so a change that gave
the report a date would fail it.

## 10. Version

A minor version, 2.4.0: a finding appears where it did not (enabled or
disabled in place of unknown), sentences of the report change, and the
verdict changes for a report with no maximum. No public surface is removed;
the snapshot stays at schema `2.0`.

## 11. Out of scope

- Karpenter, AKS node auto provisioning, and the Karpenter provider of OCI:
  their nodes are not added by the cluster autoscaler, and none writes the
  status ConfigMap. A cluster that scales only by Karpenter reads disabled;
  its configuration can state enabled.
- Node pool bounds from any source.
- A setting for the name or namespace of the status ConfigMap.
- Removing the three dormant findings and the dormant fields, which is a
  major version of its own.
- A check on an OKE cluster.

## Definition of done

- No query of the program names `cluster_autoscaler_`, and a test holds it.
- Each status of section 3 is produced by a scenario, and each of the four
  findings appears in a golden snapshot.
- A report from kube-state-metrics shows no unknown maximum, minimum, or count
  of node groups on the PDF or the web.
- A stored snapshot that holds `NODE_AUTOSCALING_BOUNDS_UNKNOWN`, near maximum,
  or at minimum still loads, views, and renders, held by a test.
- The recorded files of `testdata/prometheus/` and `testdata/python/` are
  unchanged.
- The guide, the design spec, AGENTS.md, the README, and the changelog say
  what section 7 lists.
- The real-cluster checks of section 8 pass on an AKS cluster, or the owner
  rules otherwise.
- A report PDF holds no date, and a report data file compiled twice 1.1
  seconds apart gives the same bytes, held by a test tagged `template`.
- `make gate` passes on the version commit.
