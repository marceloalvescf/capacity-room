# Changelog

All notable changes to this project are recorded in this file. The format
follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and the
project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- The `report` command checks a Prometheus-compatible endpoint, the cluster
  scope, and the metrics the endpoint offers, with `--check`.
- The `report` command writes a PDF with the cover, the report metadata, the
  status legend, and the data quality appendix, in Portuguese (Brazil) or
  English.
- The `report` command stores what it found in a snapshot file with
  `--snapshot`, and writes a PDF from that file with `--from-snapshot`,
  without any request to the endpoint.
- The configuration is read from a TOML file, then from the environment, then
  from the command line.
- The bearer token is taken from an environment variable or from `--token`,
  and is removed from every message the program prints.
- A bearer token shorter than 8 characters is refused, because a text that
  short cannot be told from ordinary words in a message.
- The report holds the capacity of each namespace and the resources of each
  workload: average and peak consumption of CPU and memory, the requests and
  the limits, and the peak of consumption judged against both.
- The report lists the interpretation notes of the collection after the data
  quality appendix.
- The snapshot file holds the namespaces, the workloads, the findings, the
  warnings, and the notices.
- A run that collects prints one line for the collection of the namespaces
  and one for the collection of the workloads.
- An answer of the endpoint larger than 256 MiB stops the run.
- A period that is empty, because the offset of the time zone changes inside
  it, stops the run with a message that names the period and the zone.
- A namespace of the configuration, or a workload that the endpoint answers,
  whose name holds a slash stops the run, because no name of Kubernetes
  holds one.
- The report holds the history of the HPAs of each namespace and the
  workloads without one, with the situation of each against its configured
  maximum.
- The report holds the usage of the node pools, a profile of allocatable,
  used, and requested resources per node, and the detail of each node with
  the reason of its status.
- The snapshot file holds the HPAs, the nodes, the node pools, the coverage
  of each workload, the cluster autoscaling evidence, and the counts of the
  report.
- A run that collects prints one line for each of the five collections and
  one for the cluster autoscaling.
- A run whose collectors return nothing, with the state of the cluster
  autoscaling unknown, stops with exit code 5.
- The report holds an executive summary, section 5: the workloads with
  critical and high priority, the namespaces with a workload at risk, the
  workloads at their replica maximum and without HPA, the situation of the
  node pools, the prioritized action plan, and the material limitations.
- The `executive` report profile writes the opening page, the executive
  summary, and the quality appendix, with the minimum severity raised to at
  least attention; the default name of its PDF ends in `-executive` in place
  of the severity.
- The snapshot file holds the recommendations and their count.

### Changed

- The Portuguese report writes the technical terms of Kubernetes in English:
  request, limit, workload, node, node pool, label, and autoscaling.

### Fixed

- The average of a series equals the original's when the series has more
  than two samples.
- A threshold or a timeout that is not a finite number is refused by the
  configuration.
- The host of an IPv6 instance keeps the letter case of its zone when it is
  matched to a node.
- A name in the report metadata, such as a namespace, breaks only at its
  own hyphens, in Portuguese too, and never starts a line with a hyphen.
