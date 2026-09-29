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
| `<scenario>.json` | The namespaces, the workloads, the findings of namespaces and workloads, and the warnings and notices of the two collectors |
| `<scenario>.queries.txt` | Every request the original sends for the scenario, in order, one line for each: the path, a tab, the query |

There is one pair of files for each of the seven scenarios: the six of
`testdata/prometheus/` and `edge-values` of `testdata/prometheus-own/`.

In a `.json` file:

- `period_start`, `period_end`, and `step` are the window of the range
  queries, in seconds since the epoch, and the step.
- `namespace_collection` and `workload_collection` are the states that the
  two collectors report.
- A statistic holds `average`, `minimum`, `peak`, and `current`, each a
  number or `null`, and `no_base`. Four `null` with `no_base` false mean that
  the value could not be measured. `no_base` true means that the base of the
  ratio does not exist.
- The evidence of a finding is a list of pairs, a name and a value. The pair
  `approximate` is a mark of the original that this project does not store:
  the confidence `inferred` of the finding says the same.
- The warnings and notices are the sentences of the original, in Portuguese.
- A number is written as Python writes it, which is the shortest text that
  reads back as the same number.

The original sends more requests than this project does yet: after the
queries of the namespaces and of the workloads come those of the
autoscalers, the nodes, and the cluster autoscaling. The list of this
project for a scenario is the beginning of the list of the original.

## How it was recorded

- Date: 2026-09-29.
- Source: `kubernetes-report` 1.11.0, imported from its source folder.
- Python 3.14.7 with `requests` 2.34.2, in an environment of its own.
- The answers were served by `cmd/fake-prometheus` of this project, built
  from the commit `cf7bea0`, so that both tools read the same answers from
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
18f290a55767dd814cda16d14ef93c483eda5f9d38cdfb4f69924d3aabe33d59  direct-complete.json
a1be74c8d80ae404974fed5232aedbf5fb7defb8bac58f9af5287d1080aac7da  edge-values.json
9b808a1f8f9b5c3817cdcb638d7f866c2af8293bb5d5d3de346407f2b9d6ff66  grafana-authenticated.json
42d1c939dd7955646813223c92f027dbfdad9eabcd2b270cbbcfb3174403c311  partial-data.json
01f184b2d571773814e7d09ef47e681263d01f06fcf6c0ce6bec4aee8b9f7ce2  raw-fallback.json
b3512df844626e8260426ea89b72cd92f6cc53567c8b53110699282a26a89a33  thanos-multicluster.json
c465e4f885cd3f997badc85c9e9ce5f30ac954db428d27c717ac95cb69f43338  value-edge-cases.json
51ea605045169cf7397bef8c6887d931ad86fb4c0d7c03b639645f68cd33a540  direct-complete.queries.txt
51ea605045169cf7397bef8c6887d931ad86fb4c0d7c03b639645f68cd33a540  edge-values.queries.txt
d538e9fc978df2e9fb8789da1ec1d713b1c9593bbd429e82b7dcc1475870982d  grafana-authenticated.queries.txt
664f32e829cad22af13e9802cf1c59cef455b2b74b7ff8a88a34d1a461796787  partial-data.queries.txt
200606fc6c00cb8b0feafc7e246a34dce5ffaa203fb779c43973f19e4f63a6dc  raw-fallback.queries.txt
51ea605045169cf7397bef8c6887d931ad86fb4c0d7c03b639645f68cd33a540  thanos-multicluster.queries.txt
9c09945181c35fdab4eceb1b4fc642823c11d61df6d2c76603af4ebfd99fbb17  value-edge-cases.queries.txt
```
