# Answers added to the recorded scenarios

Each file holds answers to queries that the original program never sent, for
the scenario of the same name in `testdata/prometheus/` or
`testdata/prometheus-own/`, whose files are never edited.
`promtest.LoadScenario` reads a scenario as the answers of this folder
followed by the recorded answers, so a recorded answer never takes a query
meant for an added one. The format is the format of the recorded files.

## What each file answers

All but `partial-data` and `value-edge-cases` answer the probe of node
autoscaling and the five queries of its collector, from kube-state-metrics.

- `direct-complete.json` and
  `thanos-multicluster.json`: ConfigMaps are collected (41), the status
  ConfigMap of the cluster autoscaler exists and was written 120 times, one
  node, one node added, and one unschedulable pod. Enabled.
- `raw-fallback.json`: the ConfigMap exists and kube-state-metrics exports no
  resource version for it; one node, one node added, and no unschedulable
  pod. Enabled by presence.
- `edge-values.json`: ConfigMaps are collected (5), none is the status
  ConfigMap, and one node answers. The empty answers for nodes added and for
  unschedulable pods read as zero. Disabled.
- `edge-scaling.json`: the status ConfigMap exists and was written no time in
  the window, the autoscaler left behind; four nodes, two added, and two
  unschedulable pods. Disabled with node pressure.
- `partial-data.json`: no ConfigMap is collected, so the probe finds none and
  the collector sends nothing. Unknown.
- `value-edge-cases.json`: the probe finds ConfigMaps (41), and the mode is
  configured as disabled, so the collector sends nothing.

## SHA-256

```text
156fbf184ceddf9fdfbf423db03e51dde1cd0858fa2043712a422aa404270267  direct-complete.json
3fc4c0cda6822636d51380ff9f1181cf9a6694efb7b1c205c60e57b907fbfc29  edge-scaling.json
9a989bdc0faae70a6760f9239f1ca157ee6be3c9796d70566b15e36e84629c4d  edge-values.json
960321a97d2a4197e46dbb5966aa861639c26145c94610bccc287310b50679bc  partial-data.json
7f9ee96161d29a3aa92cb63733d665ca23ff2dfe42a830fef317a57559b9b456  raw-fallback.json
156fbf184ceddf9fdfbf423db03e51dde1cd0858fa2043712a422aa404270267  thanos-multicluster.json
cb0730b758a35287366784a8400024415dda9da8f4eb7215f8b8fea5bbcb4b3c  value-edge-cases.json
```
