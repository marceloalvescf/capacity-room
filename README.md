<p align="center">
  <img src="web/public/favicon.svg" width="72" height="72" alt="">
</p>

<h1 align="center">CapacityRoom</h1>

<p align="center">
  <strong>Kubernetes capacity reports from your metrics alone.</strong><br>
  A PDF and a web report of requests, limits, usage, HPAs, and nodes, read from
  a Prometheus-compatible API. No kubeconfig, no cluster access, no cloud
  credentials.
</p>

<p align="center">
  <a href="https://github.com/marceloalvescf/capacity-room/actions/workflows/ci.yml"><img alt="checks" src="https://img.shields.io/github/check-runs/marceloalvescf/capacity-room/main?nameFilter=checks&label=checks"></a>
  <a href="https://github.com/marceloalvescf/capacity-room/actions/workflows/ci.yml"><img alt="integration" src="https://img.shields.io/github/check-runs/marceloalvescf/capacity-room/main?nameFilter=integration&label=integration"></a>
  <a href="https://github.com/marceloalvescf/capacity-room/actions/workflows/ci.yml"><img alt="e2e" src="https://img.shields.io/github/check-runs/marceloalvescf/capacity-room/main?nameFilter=e2e&label=e2e"></a>
  <a href="https://github.com/marceloalvescf/capacity-room/actions/workflows/audit.yml"><img alt="audit" src="https://github.com/marceloalvescf/capacity-room/actions/workflows/audit.yml/badge.svg"></a>
  <br>
  <a href="https://github.com/marceloalvescf/capacity-room/releases/latest"><img alt="release" src="https://img.shields.io/github/v/release/marceloalvescf/capacity-room?sort=semver"></a>
  <a href="https://hub.docker.com/r/alvesmarcelocf/capacity-room"><img alt="image" src="https://img.shields.io/docker/v/alvesmarcelocf/capacity-room?sort=semver&label=image"></a>
  <a href="go.mod"><img alt="Go" src="https://img.shields.io/github/go-mod/go-version/marceloalvescf/capacity-room"></a>
  <a href="LICENSE"><img alt="licence" src="https://img.shields.io/badge/license-MIT-blue"></a>
</p>

<table>
  <tr>
    <td width="58%"><img src="docs/assets/web-overview.png" alt="The overview of a report in the web interface: the findings by severity, whether the cluster can grow, the pressure on each scope, and the action plan."></td>
    <td width="42%"><img src="template/tests/golden/command-complete-en-1.png" alt="The verdict page of a PDF report: the findings by severity, the pressure on each scope, the cluster autoscaling, and the first actions."></td>
  </tr>
</table>

`capacity-room` generates Kubernetes capacity reports from a
Prometheus-compatible API, such as Prometheus, Thanos Query Frontend,
VictoriaMetrics, or Cortex. It reads metrics only: it never uses a kubeconfig,
the Kubernetes API, or cloud credentials. Reports are produced as PDF through
Typst, in Portuguese (Brazil) or English, and as a web report on the platform.
The product is called CapacityRoom; the command and the image are named
`capacity-room`.

The full design is in
[docs/superpowers/specs/2026-09-27-k8s-capacity-report-design.md](docs/superpowers/specs/2026-09-27-k8s-capacity-report-design.md).

## Quick start

### One PDF, with Docker

The image `alvesmarcelocf/capacity-room` carries the command and Typst. Write
a configuration that names your endpoint, the cluster, and its namespaces,
then run the image over the folder that holds it:

```bash
cat > report.toml <<'EOF'
[metrics]
url = "https://prometheus.example.org"

[cluster]
name = "prod"
namespaces = ["payments", "checkout"]
EOF

docker run --rm --user "$(id -u):$(id -g)" -v "$PWD:/work" -w /work \
  alvesmarcelocf/capacity-room:4.1.0 \
  report --config report.toml --lang en --output report.pdf
```

Add `--check` in place of `--output` to see what the endpoint offers before a
full run. An endpoint that needs a bearer token reads it from a variable that
`metrics.token_env` names, passed with `-e`; see
[Configuration](#configuration).

### The web platform, with Docker Compose

The platform puts the reports on the web: sign-in with three roles, the
metric sources as profiles, reports requested from the browser, their PDFs in
both languages, and a report screen of seven tabs.
[Section 2 of the operations guide](docs/operations.md#2-install) installs it
with `deploy/compose.yaml`, PostgreSQL 18, and a TLS proxy of your own.

### On Kubernetes, with Helm

The chart is published as an OCI chart. Get its folder, which holds
`values.yaml` and the examples, and follow
[its quick start](deploy/helm/capacity-room/README.md):

```bash
helm pull oci://ghcr.io/marceloalvescf/charts/capacity-room --version 4.1.0 --untar
```

## What it does

- The command `report`. It checks a Prometheus-compatible endpoint, stores
  what it found in a snapshot, and writes a PDF.
- The PDF opens on a verdict page: the findings by severity, whether the
  cluster can grow, the pressure on each scope, and the first actions. Then
  come the opening page, the executive summary, the capacity of each
  namespace, the resources of each workload, the HPA history, the resource
  usage by node pool, and an appendix with the technical quality of the data
  and the interpretation notes. The `executive` profile writes the verdict
  page, the opening page, the executive summary, and the appendix.
- The command `serve`, the backend of the web platform: sign-in with local
  accounts in three roles, sessions, the reader's own account, the
  administration of users and of metric-source profiles, and the reports,
  over an HTTP API described in [api/openapi.yaml](api/openapi.yaml).
- The web interface, served by `serve`, in Portuguese (Brazil) and English,
  in a light and a dark theme: a home page with the latest report of each
  profile; an account screen where a reader changes their password and
  language; the list of reports, with bulk delete; the form that requests a
  report, with quick, calendar, and typed periods; and the progress page of a
  report, where its PDFs are downloaded and the PDF of the other language is
  generated. A collected report opens on a screen of seven tabs: overview,
  findings, namespaces, workloads, autoscaling, nodes, and data quality. It
  is worded as its PDF, and the PDFs are one click away. An administrator
  manages the users (create, change the role, the language, or the password,
  deactivate, reactivate) and the metric-source profiles (list, create, edit,
  delete, and test a profile's connection, saved or before saving it).
- The command `worker`, which runs the report jobs of the platform: it
  collects a requested report through the outbound policy, stores its
  snapshot, writes the PDF of the language it was requested in, writes the
  other language's PDF when someone asks for it, and expires and removes
  reports after their retention.
- The command `admin`, with `migrate`, `bootstrap-admin`, `reset-password`,
  `probe`, and `import-profiles`.
- A container image that carries the binary and Typst 0.15.1, and a Helm
  chart that runs the platform on Kubernetes.

## Generating a report

### Configuration

The smallest configuration names the endpoint, the cluster, and the
namespaces:

```toml
[metrics]
url = "https://prometheus.example.org"
token_env = "CAPACITYROOM_TOKEN"

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

A report covers the last 24 hours by default. `analysis.period` (`--period`,
`CAPACITYROOM_PERIOD`) sets another length, such as `7d`, ending when the run starts.
`analysis.start` and `analysis.end` (`--start` and `--end`, `CAPACITYROOM_START` and
`CAPACITYROOM_END`) set a calendar range instead, such as `2026-09-01 00:00` to
`2026-10-01 00:00`. Each is a date and a time to the minute, read in
`analysis.timezone`. The start must come before the end, and the end may not
be after the run starts. A time that does not exist in the zone, or occurs
twice in it, because the clock moves there, is refused. A length and a range
exclude each other: one source may not set both, and a later source that sets
one replaces the other.

### The four modes

```bash
# Check the endpoint, the cluster scope, and the metrics it offers.
capacity-room report --config report.toml --check

# Write a PDF.
capacity-room report --config report.toml --lang en --output report.pdf

# Store what was found, without writing a PDF. Needs no typst.
capacity-room report --config report.toml --snapshot cluster.json

# Write a PDF of a calendar range that has ended.
capacity-room report --config report.toml --start "2026-09-01 00:00" --end "2026-10-01 00:00" --output september.pdf

# Write a PDF from a stored snapshot. Makes no request to the endpoint.
capacity-room report --from-snapshot cluster.json --lang pt-BR --output report.pdf
```

The modes exclude each other. One collection gives a PDF in both languages
through `--snapshot` followed by `--from-snapshot`, once for each language.

`--from-snapshot` reads no configuration file. It takes `--lang`,
`--report-profile`, and `--output`, and the variables `CAPACITYROOM_LANGUAGE`,
`CAPACITYROOM_REPORT_PROFILE`, and `CAPACITYROOM_OUTPUT_DIRECTORY`. The profile
comes from the flag, else from the variable, else from the stored snapshot. A
snapshot written by 2.x or 3.x is refused; section 8 of
[`docs/operations.md`](docs/operations.md) shows how to convert one.

Without `--output`, the PDF is written to `report.output_directory` under the
name `report-resources-<cluster>-<language>-<date>-<time>.pdf`.

The report profile is `detailed`, the default, or `executive`. Set it with
`report.profile`, `--report-profile`, or `CAPACITYROOM_REPORT_PROFILE`. The `executive`
profile writes the verdict page, the opening page, the executive summary as its only
numbered section, 1, and the appendix as section 2. It shows the critical and
attention findings alone, in every count and every section, the appendix
included; the detailed profile shows every finding. Its PDF is named
`report-resources-<cluster>-executive-<language>-<date>-<time>.pdf`.

Add `--verbose` to any mode for a few more lines about the run.

### What a run collects

A run that writes a PDF or a snapshot asks the endpoint about its period: the
length `analysis.period`, which ends when the run starts, or the range from
`analysis.start` to `analysis.end`. For each namespace of
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
  cluster autoscaling), the HPAs, the nodes, the node pools, the
  recommendations, and, on each workload, its HPA coverage: covered,
  uncovered, or unknown. The recommendations are the findings above healthy,
  merged by code, with their severity, scopes, and confidence. A snapshot
  file whose recommendations disagree with its findings is refused. A workload
  that only the coverage found is a row with no observed usage.
- **A Job is left out of the list of workloads**, and so is a workload that
  consumed nothing in the period. What they consumed still counts in the
  total of their namespace. The interpretation notes, after the quality
  appendix, say how many were left out.
- **`--check` collects nothing.** It sends the requests of the preflight
  and stops.

The PDF opens on a verdict page, which gives the findings by severity, whether
the cluster can grow, the pressure on each scope, and the first actions, and
then the opening page. The detailed report holds five sections, numbered 1 to
5, and the appendix, 6; its words are those of the web app. A line that names
the program and the date closes the document.

- **Executive summary** (section 1). The critical workloads and those needing
  attention; the namespaces with workloads at risk; the workloads at their
  replica maximum and without an HPA; the situation of the node pools; the
  prioritized action plan, which names each scope with its kind; and the
  material limitations. Its rows start at the severity attention.
- **Capacity by namespace** (section 2). For each namespace, the average and
  peak usage of CPU and memory, its requests and limits, and the two ratios
  of the peak to the request and to the limit.
- **Resources by workload** (section 3). For each workload, its pods, the
  average and peak usage of CPU and memory, and the four ratios of usage to
  request and to limit.
- **HPA history** (section 4). For each namespace, the HPAs that were
  observed, with their specs, the current average, the minimum and the
  maximum they reached, and their situation; the workloads without an HPA,
  with their pods and their situation; and a note when the HPA data is
  incomplete or partial.
- **Resource usage by node pool** (section 5). A summary of each pool, with
  its average and peak usage and the confidence in the identity of the pool;
  an aggregate profile of the allocatable, used, and requested CPU and memory
  of each node; and the detail of each node, with its usage, the reason of
  its state, and whether it is ready and schedulable.

The appendix, 6, holds the technical quality of the data and the
interpretation notes.

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

## Running the platform

The platform puts the reports on the web: `serve` holds the interface and the
API, `worker` collects the reports and compiles their PDFs, and PostgreSQL
keeps everything, run together by `deploy/compose.yaml` behind a TLS proxy
of your own. Upgrading to 3.0.0 or to 4.0.0 carries the stored reports
forward and cannot be undone: back up the database first.

- [`docs/operations.md`](docs/operations.md) is the guide to install,
  configure, back up, upgrade, and troubleshoot it. Section 12 runs it on
  Kubernetes.
- [`deploy/helm/capacity-room/README.md`](deploy/helm/capacity-room/README.md)
  is the quick start of the Helm chart.
- [`docs/security.md`](docs/security.md) describes what it defends against,
  and [`SECURITY.md`](SECURITY.md) how to report a vulnerability.

`deploy/compose.integration.yaml` and the files of `deploy/platform/` serve
`make e2e` alone, and never a deployment.

## Development

### Requirements

The Nix flake's development shell holds every tool at its pinned version:
run each command inside `nix develop`. Without Nix, you need:

- Go 1.27
- Node 26 with npm, to build the interface
- Docker with Buildx and the Compose plugin
- `typst` 0.15.1 and `pdftotext`, for the tests that compile a PDF. Any other
  version of `typst` is refused; `CAPACITYROOM_TYPST` may give its path.
- For `serve`, `worker`, and `admin`: PostgreSQL 18

### Commands

```bash
make web              # the interface, built into web/dist
make build            # bin/capacity-room, with the interface embedded
make test             # Go tests that need no service and no tool, and the interface's tests
make lint             # gofmt, go vet, golangci-lint, and tsc and ESLint over web/
make image            # runtime image and test image
make test-toolchain   # checks typst and pdftotext inside the test image
make test-template    # compiles the template and writes reports in the test image
make test-golden      # compares rendered pages with the golden pages
make test-chart       # helm lint, the chart's tests, and kubeconform in the test image
make check-runtime    # runs the runtime image as the release ships it
make generate         # writes the Go and TypeScript types of api/openapi.yaml
make generate-check   # fails when the committed generated file is stale
make postgres-up      # starts PostgreSQL 18 for the platform tests, through Compose
make postgres-down    # removes it, with its volume
make test-postgres    # postgres-up, the tests tagged postgres (and the worker's with typst), postgres-down
make e2e              # the Compose stack and the browser test of the interface
make gate             # every check above, from freshly built images
make audit            # the scanners of the Go modules, the interface's dependencies, and the image
make update-golden    # rewrites the golden pages after an intended change
make update-testdata  # rewrites the golden snapshots and data files
```

CI runs the same targets on every pull request and push to `main`. A pushed
version tag runs `make gate` and `make audit` again, then publishes the image,
the chart, and a draft GitHub release.

### Trying it without a Prometheus

The repository holds seven scenarios and a server that answers from them.
Five are recorded answers, in `testdata/prometheus/`, with the answers to the
queries the original never sent in `testdata/prometheus-added/`. The other
two were written for this project, in `testdata/prometheus-own/`:
`edge-values` holds values at the edge, such as a sample that is not a
number, a request of zero, and a workload with no request at all;
`edge-scaling` holds the edges of the HPAs and of the nodes. Each answer is
given once, so the server is started again for each run that collects.

```bash
make build
go build -o bin/fake-prometheus ./cmd/fake-prometheus

bin/fake-prometheus --fixture testdata/prometheus/direct-complete.json \
  --added testdata/prometheus-added/direct-complete.json \
  --listen 127.0.0.1:9090 &

mkdir -p .cache
cat > .cache/report.toml <<'EOF'
[metrics]
url = "http://127.0.0.1:9090/api/v1"

[cluster]
name = "prod"
namespaces = ["payments"]
EOF

bin/capacity-room report --config .cache/report.toml --check
kill %1
```

### Working on the interface

Vite serves the interface with live reload and passes `/api` and `/health`
to `serve` on port 8080. `serve` and `worker` read their settings from a
`platform.toml`, whose keys section 3 of the operations guide lists; the one
below lets them reach a fake Prometheus on the same host.

```bash
make postgres-up
export CAPACITYROOM_DATABASE_DSN='postgres://capacityroom_test:capacityroom_test_only@127.0.0.1:55432/capacityroom_test?sslmode=disable'

mkdir -p .cache/dev
head -c 32 /dev/urandom > .cache/dev/server-secret
cat > .cache/dev/platform.toml <<EOF
server_secret_path = "$PWD/.cache/dev/server-secret"
allowed_endpoint_cidrs = ["127.0.0.1/32"]
EOF

make build
printf 'a-local-password-1\n' | CAPACITYROOM_BOOTSTRAP_MODE=1 \
  bin/capacity-room admin bootstrap-admin --username admin --password-stdin --config .cache/dev/platform.toml
bin/capacity-room serve --config .cache/dev/platform.toml --listen 127.0.0.1:8080 &
bin/capacity-room worker --config .cache/dev/platform.toml &
cd web && npm run dev    # http://localhost:5173, sign in as admin
```

## Documents

| File | Content |
|---|---|
| [CHANGELOG.md](CHANGELOG.md) | What changed in each version |
| [docs/operations.md](docs/operations.md) | The operations guide of the platform |
| [docs/security.md](docs/security.md) | The threat model, the controls, and the security review |
| [SECURITY.md](SECURITY.md) | How to report a vulnerability |
| [deploy/helm/capacity-room/README.md](deploy/helm/capacity-room/README.md) | The quick start of the Helm chart |
| [AGENTS.md](AGENTS.md) | Working agreement: environment, gates, phases, versioning |
| [docs/superpowers/specs/](docs/superpowers/specs/) | Design |
| [docs/superpowers/plans/](docs/superpowers/plans/) | Implementation plans, one per phase |
| [docs/decisions/](docs/decisions/) | Decisions taken from measurements |
| [docs/reference/](docs/reference/) | Documents and inventories of the two original projects |
| [template/graphite/README.md](template/graphite/README.md) | The generic template package |
| [testdata/prometheus/README.md](testdata/prometheus/README.md) | The five recorded scenarios |
| [testdata/prometheus-added/README.md](testdata/prometheus-added/README.md) | The answers the original never asked for |
| [testdata/prometheus-own/README.md](testdata/prometheus-own/README.md) | The two scenarios written for this project |
| [testdata/python/README.md](testdata/python/README.md) | The numbers and queries of the original, for the seven scenarios |

## Licence

The code is under the MIT licence, in [LICENSE](LICENSE). The fonts under
`template/graphite/assets/fonts`, Public Sans, JetBrains Mono, and Liberation
Sans, are under the SIL Open Font License, each kept beside them.
