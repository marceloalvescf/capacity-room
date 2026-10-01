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
| `<scenario>.json` | The namespaces, the workloads, the HPAs, the workloads without an HPA, the nodes, the node pools, the cluster autoscaling evidence, the node pressure, every finding, and the warnings and notices of the five collectors |
| `<scenario>.queries.txt` | Every request the original sends for the scenario, in order, one line for each: the path, a tab, the query |

There is one pair of files for each of the eight scenarios: the six of
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
- The warnings and notices are the sentences of the original, in Portuguese,
  in the order namespace, workload, hpa, hpa coverage, node, each once.
- A number is written as Python writes it, which is the shortest text that
  reads back as the same number.

## How it was recorded

- Date: 2026-09-30. The query lists were recorded again on that date and
  did not change from the recording of 2026-09-29.
- Source: `kubernetes-report` 1.11.0, imported from its source folder.
- Python 3.14.7 with `requests` 2.34.2, in an environment of its own.
- The answers were served by `cmd/fake-prometheus` of this project, built
  from the commit `e6c532e`, so that both tools read the same answers from
  the same server.
- The input is the fixed input of the golden files of this project: cluster
  `prod`, namespace `payments`, excluded container `istio-proxy`, period
  `24h`, time zone `America/Sao_Paulo`, emission time 2026-07-15 14:30 of
  that zone, thresholds by default. The endpoint type, the token, and the
  node autoscaling are those of each scenario, as `internal/promtest` lists
  them.
- The numbers were read from the result of `preflight_report`,
  `collect_evidence`, and `analyze_evidence` of the module `collection`. The
  requests were read by a wrapper around the method of the client that sends
  a request.

The script and its environment are not tracked. Nothing of the original was
changed.

## SHA-256

```text
0b8c536e6540f1f78be730180a185977cab053049a1665a0df44e5635735f483  direct-complete.json
0dcd39ce6b68ed24069fb24c0344983073a24850b2d7e8390937404ef03bdf3a  edge-scaling.json
2119d8db5ae47393789134b5a9c529e78f59a39a165d1aa735256a83b676665b  edge-values.json
af6dacd88fa5f7bf4621b53c92243a62c99b3d17b38bcd0a82c6aa3b0134363d  grafana-authenticated.json
f7248d344bc709270d7d9d14a43eaf6e57908e4ece8e872f29801d460b28cff6  partial-data.json
7d1dd022702a34059a1e9e16b08c4f73c48da1d43aba33dbb796116bd1ceed5a  raw-fallback.json
1406129bc402d2d494b9efc6fc5405ecae2fce1c3729025c7ece2921908ddc0d  thanos-multicluster.json
79fc52777ea677af285cc572722cb8becf96b78bf9840a39b997fb02a77e132b  value-edge-cases.json
51ea605045169cf7397bef8c6887d931ad86fb4c0d7c03b639645f68cd33a540  direct-complete.queries.txt
f552ed8b6bb6b27dec20c03c3e7bc5a2db2292b3f3d32c06f16e79dec02bedaa  edge-scaling.queries.txt
51ea605045169cf7397bef8c6887d931ad86fb4c0d7c03b639645f68cd33a540  edge-values.queries.txt
d538e9fc978df2e9fb8789da1ec1d713b1c9593bbd429e82b7dcc1475870982d  grafana-authenticated.queries.txt
664f32e829cad22af13e9802cf1c59cef455b2b74b7ff8a88a34d1a461796787  partial-data.queries.txt
200606fc6c00cb8b0feafc7e246a34dce5ffaa203fb779c43973f19e4f63a6dc  raw-fallback.queries.txt
51ea605045169cf7397bef8c6887d931ad86fb4c0d7c03b639645f68cd33a540  thanos-multicluster.queries.txt
9c09945181c35fdab4eceb1b4fc642823c11d61df6d2c76603af4ebfd99fbb17  value-edge-cases.queries.txt
```
