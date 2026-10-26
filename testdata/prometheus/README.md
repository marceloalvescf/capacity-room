# Prometheus scenarios

These five files are recorded answers of a Prometheus-compatible API. The tests
serve them through `internal/promtest`, and `cmd/fake-prometheus` serves one
of them over HTTP.

They were carried over byte for byte from `kubernetes-report` 1.11.0, folder
`tests/fixtures/prometheus/`. Every name and address in them is invented: the
clusters `prod` and `dev`, the namespace `payments`, the node `node-1` at
`10.0.0.1`.

Do not edit a file here. The golden snapshots depend on every byte, and the
comparison with the original tool depends on both tools reading the same
answers.

| File | Requests | Exercises |
|---|---|---|
| `direct-complete.json` | 70 | Every capability available, recording rules present |
| `thanos-multicluster.json` | 70 | Two values of the cluster label, `prod` and `dev` |
| `raw-fallback.json` | 73 | No recording rule; raw metrics and client-side joins |
| `partial-data.json` | 63 | Missing HPA, node, and autoscaler metrics |
| `value-edge-cases.json` | 70 | The same bytes as `direct-complete.json`; the original ran it with node autoscaling set to `disabled` |

## SHA-256

```text
114e04d41f69d77fceddd71433228fa16ff0d835cb73a3e1ee25384921b14f4f  direct-complete.json
1cbd8dc18187ac62c21a9e0d6beb21b300eb4770c0523911ad9968247d3454b4  partial-data.json
03e543dd89d3381318232cea8c66dad3e9ebe5d429dbf6e110e9ad31da388461  raw-fallback.json
f0ae19bf0b07f5446dd8dae717fba35f48388589e455999d272ac16f11e1bdcb  thanos-multicluster.json
114e04d41f69d77fceddd71433228fa16ff0d835cb73a3e1ee25384921b14f4f  value-edge-cases.json
```

Two further scenarios, with values at the edge that no answer of the original
tool holds, are not carried over from it and live in
`testdata/prometheus-own/`, described in its own README.
