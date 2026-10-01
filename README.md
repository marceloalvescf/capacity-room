# k8s-capacity-report

`k8s-capacity-report` generates Kubernetes capacity reports from a
Prometheus-compatible API. It reads metrics only: it never uses a kubeconfig,
the Kubernetes API, or cloud credentials. Reports are produced as PDF through
Typst, in Portuguese (Brazil) or English.

The project is under construction. This file documents what exists today; the
full design is in
[docs/superpowers/specs/2026-09-27-k8s-capacity-report-design.md](docs/superpowers/specs/2026-09-27-k8s-capacity-report-design.md).

## What exists today

- The command `report`. It checks a Prometheus-compatible endpoint, stores
  what it found in a snapshot, and writes a PDF.
- The PDF holds the cover, the report metadata, the status legend, the
  capacity of each namespace, the resources of each workload, the HPA
  history, the resource usage by node pool, the data quality appendix, and
  the interpretation notes of the collection.
- A container image that carries the binary and Typst 0.15.1.
- The report template, in two layers under [template/](template/): a generic
  page shell and the layout of the capacity report.
- Guards that protect the structure of the repository, and golden files that
  pin the snapshot, the data file, and the rendered pages.

## Requirements

- Go 1.27
- Docker with Buildx
- To write a PDF outside the container: `typst` 0.15.1 on `PATH`, or its path
  in `KCR_TYPST`. Any other version is refused.

## Generating a report

### Configuration

The smallest configuration names the endpoint, the cluster, and the
namespaces:

```toml
[metrics]
url = "https://prometheus.example.org"
token_env = "KCR_TOKEN"

[cluster]
name = "prod"
namespaces = ["payments", "checkout"]
```

Settings are read from the file, then from the environment, then from the
command line. The later one wins. An unknown section or key is an error.
Section 5.2 of the design lists every key, its default, and its rule.

The token is never written in the file. `metrics.token_env` names the
environment variable that holds it, or `--token` gives it. When the named
variable is unset or empty, the run stops. A token shorter than 8 characters
is refused: a text that short cannot be told from ordinary words, so it could
not be removed from the messages the program prints.

### The four modes

```bash
# Check the endpoint, the cluster scope, and the metrics it offers.
k8s-capacity-report report --config report.toml --check

# Write a PDF.
k8s-capacity-report report --config report.toml --lang en --output report.pdf

# Store what was found, without writing a PDF. Needs no typst.
k8s-capacity-report report --config report.toml --snapshot cluster.json

# Write a PDF from a stored snapshot. Makes no request to the endpoint.
k8s-capacity-report report --from-snapshot cluster.json --lang pt-BR --output report.pdf
```

The modes exclude each other. One collection gives a PDF in both languages
through `--snapshot` followed by `--from-snapshot`, once for each language.

`--from-snapshot` reads no configuration file. It takes `--lang`,
`--min-severity`, and `--output`, and the variables `KCR_LANGUAGE`,
`KCR_MIN_SEVERITY`, and `KCR_OUTPUT_DIRECTORY`.

Without `--output`, the PDF is written to `report.output_directory` under the
name `report-resources-<cluster>-<language>-<date>-<time>.pdf`. When the
minimum severity is not `all`, it comes after the cluster:
`report-resources-<cluster>-<severity>-<language>-<date>-<time>.pdf`.

Add `--verbose` to any mode for a few more lines about the run.

### What a run collects

A run that writes a PDF or a snapshot asks the endpoint about the period
`analysis.period`, which ends when the run starts. For each namespace of
`cluster.namespaces`, and for each workload in them, it reads the consumption
of CPU and memory, the requests, and the limits, and reduces each to an
average, a minimum, a peak, and the last value. It also reads the HPAs of
those workloads, the coverage of each workload by an HPA, the nodes and the
node pools with their capacity and usage, and the cluster autoscaling.

- **The report judges the peak.** The columns of percentages show the
  highest ratio of consumption to request, and to limit, that the period
  saw. A ratio is worked out sample by sample; it is never the ratio of two
  averages.
- **Three things can stand where a number is expected.** A number; the mark
  of data that is unavailable, when the value could not be measured; and the
  mark of no base, when consumption was measured and no request or limit
  exists to compare it with.
- **The snapshot holds every section.** Besides the namespaces and the
  workloads, it holds the summary (the counts and the evidence of the
  cluster autoscaling), the HPAs, the nodes, the node pools, and, on each
  workload, its HPA coverage: covered, uncovered, or unknown. A workload
  that only the coverage found is a row with no observed usage.
- **A Job is left out of the list of workloads**, and so is a workload that
  consumed nothing in the period. What they consumed still counts in the
  total of their namespace. The interpretation notes, after the quality
  appendix, say how many were left out.
- **`--check` collects nothing.** It sends the requests of the preflight
  and stops.

The report holds four sections, numbered 1 to 4, and the appendix, 5:

- **HPA history** (section 3). For each namespace, the HPAs that were
  observed, with their specs, the current average, the minimum and the
  maximum they reached, and their situation; the workloads without an HPA,
  with their pods and their situation; and a note when the HPA data is
  incomplete or partial.
- **Resource usage by node pool** (section 4). A summary of each pool, with
  its average and peak usage and the confidence in the identity of the pool;
  an aggregate profile of the allocatable, used, and requested CPU and memory
  of each node; and the detail of each node, with its usage, the reason of
  its state, and whether it is ready and schedulable.

After the lines of the capabilities, a run that collects prints six lines:
`namespace collection`, `workload collection`, `hpa collection`,
`hpa coverage collection`, and `node collection`, each with a state
(`available`, `partial`, or `unavailable`), such as
`namespace collection: available`, and `autoscaling collection`, with
`enabled`, `disabled`, or `unknown`.

Four rules stop a run:

- An answer of the endpoint larger than 256 MiB is refused, with exit
  code 3.
- A namespace of `cluster.namespaces` that holds a slash is refused, with
  exit code 2; a workload that the endpoint answers with a slash in its
  namespace, its name, or its kind stops the run with exit code 3. No name
  of Kubernetes holds a slash, so such a name is a fault of the endpoint.
- A period that is empty stops the run with exit code 2, before any request.
  This happens only in a time zone that changes its offset, when the period
  is shorter than the hour that the change skips and the run starts inside
  the hour after it. The message names the period and the zone; a longer
  period avoids it.
- No collector returns a row with a state other than unavailable, and the
  state of the cluster autoscaling is unknown: the run stops with
  `error: no useful report sections were collected` and exit code 5, the
  code of the insufficient capabilities of the preflight.

### Exit codes

| Code | Meaning |
|---|---|
| 0 | Success |
| 1 | The run was interrupted, or the program failed in a way it did not expect |
| 2 | Configuration: the file, a flag, a snapshot file that cannot be read |
| 3 | Metrics: the endpoint could not be reached or did not answer as expected |
| 4 | Cluster scope: the cluster named is not the one the endpoint shows |
| 5 | Insufficient capabilities: the endpoint offers nothing a report could be made of |
| 6 | Output: the output path, a missing or wrong `typst`, or a compile that gave no PDF |

A failure prints one line on standard error, which starts with `error: `, and
nothing on standard output.

### Trying it without a Prometheus

The repository holds eight scenarios and a server that answers from them.
Six are recorded answers, in `testdata/prometheus/`. The other two were
written for this project, in `testdata/prometheus-own/`: `edge-values` holds
values at the edge, such as a sample that is not a number, a request of zero,
and a workload with no request at all; `edge-scaling` holds the edges of the
HPAs and of the nodes. Each answer is given once, so the server
is started again for each run that collects.

```bash
make build
go build -o bin/fake-prometheus ./cmd/fake-prometheus

bin/fake-prometheus --fixture testdata/prometheus/direct-complete.json \
  --listen 127.0.0.1:9090 &

cat > .cache/report.toml <<'EOF'
[metrics]
url = "http://127.0.0.1:9090/api/v1"

[cluster]
name = "prod"
namespaces = ["payments"]
EOF

bin/k8s-capacity-report report --config .cache/report.toml --check
kill %1
```

## Commands

```bash
make build            # bin/k8s-capacity-report
make test             # Go tests that need no service and no tool
make lint             # gofmt, go vet, golangci-lint
make image            # runtime image and test image
make test-toolchain   # checks typst and pdftotext inside the test image
make test-template    # compiles the template and writes reports in the test image
make test-golden      # compares rendered pages with the golden pages
make gate             # everything above, from freshly built images
make update-golden    # rewrites the golden pages after an intended change
make update-testdata  # rewrites the golden snapshots and data files
```

## Documents

| File | Content |
|---|---|
| [AGENTS.md](AGENTS.md) | Working agreement: environment, gates, phases, versioning |
| [CHANGELOG.md](CHANGELOG.md) | What changed in each version |
| [docs/superpowers/specs/](docs/superpowers/specs/) | Design |
| [docs/superpowers/plans/](docs/superpowers/plans/) | Implementation plans, one per phase |
| [docs/decisions/](docs/decisions/) | Decisions taken from measurements |
| [docs/reference/](docs/reference/) | Documents and inventories of the two original projects |
| [template/graphite/README.md](template/graphite/README.md) | The generic template package |
| [testdata/prometheus/README.md](testdata/prometheus/README.md) | The six recorded scenarios |
| [testdata/prometheus-own/README.md](testdata/prometheus-own/README.md) | The two scenarios written for this project |
| [testdata/python/README.md](testdata/python/README.md) | The numbers and queries of the original, for the eight scenarios |

## Licence

The code is under the MIT licence, in [LICENSE](LICENSE). The Liberation Sans
fonts under `template/graphite/assets/fonts` are under the SIL Open Font
License, kept beside them.
