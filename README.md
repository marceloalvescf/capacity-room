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
- The PDF holds the cover, the report metadata, the status legend, and the
  data quality appendix. **The capacity sections are not built yet**: the
  report says which metrics the endpoint offers, and not yet how much of the
  cluster is in use.
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

The repository holds six recorded scenarios and a server that answers from
them. Each recorded answer is given once, so the server is started again for
each run that collects.

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

## Licence

The code is under the MIT licence, in [LICENSE](LICENSE). The Liberation Sans
fonts under `template/graphite/assets/fonts` are under the SIL Open Font
License, kept beside them.
