# Recorded answers of the original tool

These files hold what `kubernetes-report` 1.11.0, the Python tool that this
project ports, computes and asks for each scenario of the tests. The tests
compare the numbers and the queries of this project with them, number by
number and query by query.

Do not edit a file here. A file is replaced only by recording the original
again, whole, and every changed number is then explained before it is
committed.

## What was recorded

| File | Holds |
|---|---|
| `<scenario>.json` | The namespaces, the workloads, the HPAs, the workloads without an HPA, the nodes, the node pools, the cluster autoscaling evidence, the node pressure, every finding, the recommendations, the summary counts, the coverage of every workload row, the rows of the seven tables of the executive body at the two effective minimums, and the warnings and notices of the five collectors |
| `<scenario>.queries.txt` | Every request the original sends for the scenario, in order, one line for each: the path, a tab, the query |

There is one pair of files for each of the seven scenarios: the five of
`testdata/prometheus/` and `edge-values` and `edge-scaling` of
`testdata/prometheus-own/`.

In a `.json` file:

- `period_start`, `period_end`, and `step` are the window of the range
  queries, in seconds since the epoch, and the step.
- `namespace_collection`, `workload_collection`, `hpa_collection`,
  `hpa_coverage_collection`, and `node_collection` are the states that the
  five collectors report, and `autoscaling_status` is the status of the
  cluster autoscaling evidence: `enabled`, `disabled`, or `unknown`.
- A statistic holds `average`, `minimum`, `peak`, and `current`, each a
  number or `null`, and `no_base`. Four `null` with `no_base` false mean that
  the value could not be measured. `no_base` true means that the base of the
  ratio does not exist.
- An HPA holds its target kind and name, its two specs, each a number or
  `null`, and the statistics of its current and desired replicas.
- A node holds its pool and the confidence of the pool, four capacities,
  each a number or `null`, the two usage statistics, and the two request
  percents, each the `average` of the original's statistic or `null`, and
  `ready` and `schedulable`, each `true`, `false`, or `null`.
- A node pool holds its name, its confidence, and the names of its nodes.
- The cluster autoscaling evidence holds its status, source, and
  confidence, and six numbers, each a number or `null`.
- `node_pressure` is what the original computes from the findings of the
  nodes and of the node pools.
- The evidence of a finding is a list of pairs, a name and a value. A value
  is a number, `true` or `false`, or a text, as the original writes it. The
  pair `approximate` is a mark of the original that this project does not
  store: the confidence `inferred` of the finding says the same.
- A recommendation holds its code, severity, scope type, scopes (the
  distinct scope names, sorted), confidence, and the action sentence of the
  original, in Portuguese, which this project does not store and does not
  compare.
- `summary` holds the eleven counts the mapper of the original writes:
  namespaces, workloads, hpas, nodes, nodepools, findings, recommendations,
  healthy, attention, critical, workloads_without_hpa.
- `workload_coverage` holds every row the mapper of the original publishes,
  measured and coverage-only, with `observed_usage` and `hpa_coverage`.
- `executive` holds, under `attention` and `critical` (the two effective
  minimums of the executive body), the rows of its seven tables as the row
  functions of the original's renderer give them, each cell a list of runs
  read from the Typst snippets: a run is `ident`, `placeholder`, `text`,
  or `status` with its status kind, and its text as the original writes
  it, in Portuguese; `hpa_note` is `incomplete`, `partial`, or empty.
- The warnings and notices are the sentences of the original, in Portuguese,
  in the order namespace, workload, hpa, hpa coverage, node, each once.
- A number is written as Python writes it, which is the shortest text that
  reads back as the same number.

## How it was recorded

- Date: 2026-10-01. The query lists were recorded again on that date and
  did not change from the recordings of 2026-09-29 and 2026-09-30.
- Source: `kubernetes-report` 1.11.0, imported from its source folder.
- Python 3.14.7 with `requests` 2.34.2, in an environment of its own.
- The answers were served by `cmd/fake-prometheus` of this project, built
  from the commit 156b9a6 of the branch of part 2.4, so that both tools read the same answers from
  the same server.
- The input is the fixed input of the golden files of this project: cluster
  `prod`, namespace `payments`, excluded container `istio-proxy`, period
  `24h`, time zone `America/Sao_Paulo`, emission time 2026-07-15 14:30 of
  that zone, thresholds by default. The endpoint type, the token, and the
  node autoscaling are those of each scenario, as `internal/promtest` lists
  them.
- The numbers were read from the result of `preflight_report`,
  `collect_evidence`, and `analyze_evidence` of the module `collection`; the
  summary and the coverage from `map_summary` and `map_workloads` of the
  module `snapshots.mapper_v1`; the executive rows from the row functions of
  the modules `renderers.workloads`, `renderers.nodes`, and
  `renderers.capabilities`, with the options of the executive profile. The
  requests were read by a wrapper around the method of the client that sends
  a request.
- The records of the mapper subclass a base class of the library `pydantic`,
  which the environment does not hold; a stand-in of twenty lines that fills
  the annotated fields of a record from its keyword arguments and class
  defaults stood for it, which is all the mapper uses of the library.

The script and its environment are not tracked. Nothing of the original was
changed.

## SHA-256

```text
90e491c7e2cb4132d5ef9774b515f636bf68b2e759275f3e2797da1aa1dcea79  direct-complete.json
1fddbeab42ee6f9ca2b147f8ebe18d7f32f61c17ef4adfee84337e4a08947b6a  edge-scaling.json
3bbd3804fda091d7efe9f97269b21dbfb79cca694b31a1779aa7805dbc968910  edge-values.json
7f4eafc98755e3f9c0958974fb0530c43d8fd6044b962f9779ff4b3a3d46a720  partial-data.json
fba40069ebeced6521d48a677e721b4e51b368b5619604c22cf0238f4f961cf1  raw-fallback.json
9b61e84fd168ae9748f25bf912e4a192fd5106c0dff7b3e2595a6265a1d7a9e0  thanos-multicluster.json
0c6c75bc9e1e07f58899ae6bca71a427d89d5b71c4ab621284bd8c9208c53c72  value-edge-cases.json
51ea605045169cf7397bef8c6887d931ad86fb4c0d7c03b639645f68cd33a540  direct-complete.queries.txt
f552ed8b6bb6b27dec20c03c3e7bc5a2db2292b3f3d32c06f16e79dec02bedaa  edge-scaling.queries.txt
51ea605045169cf7397bef8c6887d931ad86fb4c0d7c03b639645f68cd33a540  edge-values.queries.txt
664f32e829cad22af13e9802cf1c59cef455b2b74b7ff8a88a34d1a461796787  partial-data.queries.txt
200606fc6c00cb8b0feafc7e246a34dce5ffaa203fb779c43973f19e4f63a6dc  raw-fallback.queries.txt
51ea605045169cf7397bef8c6887d931ad86fb4c0d7c03b639645f68cd33a540  thanos-multicluster.queries.txt
9c09945181c35fdab4eceb1b4fc642823c11d61df6d2c76603af4ebfd99fbb17  value-edge-cases.queries.txt
```
