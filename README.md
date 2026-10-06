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
  history, the resource usage by node pool, the executive summary, the
  technical quality appendix, and the interpretation notes of the
  collection. The `executive` profile writes the cover, the opening page,
  the executive summary, and the appendix.
- The command `serve`, the backend of the web platform: sign-in with local
  accounts in three roles, sessions, the reader's own account, and the
  administration of users and of metric-source profiles, over an HTTP API
  described in [api/openapi.yaml](api/openapi.yaml). The report routes and
  the frontend are not there yet.
- The command `worker`, which runs the report jobs of the platform: it
  collects a requested report through the outbound policy, stores its
  snapshot, writes one PDF per language, and expires and removes reports
  after their retention.
- The command `admin`, with `migrate`, `bootstrap-admin`, `probe`, and
  `import-profiles`.
- A container image that carries the binary and Typst 0.15.1.
- The report template, in two layers under [template/](template/): a generic
  page shell and the layout of the capacity report.
- Guards that protect the structure of the repository, and golden files that
  pin the snapshot, the data file, and the rendered pages.

## Requirements

- Go 1.27
- Docker with Buildx
- For `serve`, `worker`, and `admin`: PostgreSQL 18
- For `worker`: `typst` 0.15.1, as for a PDF
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
`--report-profile`, `--min-severity`, and `--output`, and the variables
`KCR_LANGUAGE`, `KCR_REPORT_PROFILE`, `KCR_MIN_SEVERITY`, and
`KCR_OUTPUT_DIRECTORY`. The profile and the minimum severity come from the
flag, else from the variable, else from the stored snapshot.

Without `--output`, the PDF is written to `report.output_directory` under the
name `report-resources-<cluster>-<language>-<date>-<time>.pdf`. When the
minimum severity is not `all`, it comes after the cluster:
`report-resources-<cluster>-<severity>-<language>-<date>-<time>.pdf`.

The report profile is `detailed`, the default, or `executive`. Set it with
`report.profile`, `--report-profile`, or `KCR_REPORT_PROFILE`. The `executive`
profile writes the cover, the opening page, the executive summary as its only
numbered section, 1, and the appendix as section 2. Its minimum severity is
raised to at least attention for the whole report, the appendix included. Its
PDF is named
`report-resources-<cluster>-executive-<language>-<date>-<time>.pdf`: the
suffix `-executive` takes the place of the severity.

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

The detailed report holds five sections, numbered 1 to 5, and the appendix, 6:

- **Capacity by namespace** (section 1). For each namespace, the average and
  peak usage of CPU and memory, its requests and limits, and the two ratios
  of the peak to the request and to the limit.
- **Resources by workload** (section 2). For each workload, its pods, the
  average and peak usage of CPU and memory, and the four ratios of usage to
  request and to limit.
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
- **Executive summary** (section 5). The workloads with critical priority and
  those with high priority; the namespaces with workloads at risk; the
  workloads at their replica maximum and without an HPA; the situation of the
  node pools; the prioritized action plan; and the material limitations. Its
  rows start at the severity attention, whatever the minimum.

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

## Running the platform

`serve` needs a deployment file and the database DSN in the environment.
The file holds no secret: the server secret, which keys the sign-in
throttle, is a file of at least 32 random bytes that the file names.

```toml
session_ttl_seconds = 43200
session_idle_seconds = 3600
server_secret_path = "/etc/kcr/server-secret"
trusted_proxies = []
allowed_endpoint_hostnames = ["prometheus.example.invalid"]
allowed_endpoint_cidrs = ["10.0.0.0/8"]
credential_dir = "/run/secrets/metrics"
ca_bundle_dir = "/run/config/ca"
```

The two allowlists are the outbound policy: every address a metric source
resolves to must be allowed by host name or by range, and both lists are
empty by default, so nothing is reachable until one entry names it. The
loopback, link-local, multicast, and metadata addresses are refused unless
an entry names that single address. The two folders hold the credential
files and the CA bundles a profile names by file name; they are read when a
profile is tested or a report runs, never when a profile is written.

```bash
head -c 32 /dev/urandom > /etc/kcr/server-secret
export KCR_DATABASE_DSN='postgres://kcr:...@db:5432/kcr'

# The first administrator, once; the password is one line on standard input.
printf '%s\n' 'a password of twelve or more' | KCR_BOOTSTRAP_MODE=1 \
  bin/k8s-capacity-report admin bootstrap-admin --username admin --password-stdin --config platform.toml

# The schema, applied under a database lock. serve does the same before it listens.
bin/k8s-capacity-report admin migrate --config platform.toml

bin/k8s-capacity-report serve --config platform.toml --listen :8080

# A health check without curl, for a container.
bin/k8s-capacity-report admin probe http://127.0.0.1:8080/health/ready
```

Sign-in is a cookie session: `GET /api/v1/auth/session` gives an anonymous
caller a sign-in CSRF token, `POST /api/v1/auth/login` with that token in
`X-CSRF-Token` signs in and sets `kcr_session`, and every state-changing
request after it carries the session's `csrf_token` in the same header. Five
failed sign-ins in five minutes block the account and address pair for five
minutes. An administrator manages users under `/api/v1/admin/users`; every
signed-in reader changes their own password and language at
`/api/v1/account`. Every failure answers one JSON shape,
`{"error":{"code","message","fields"}}`.

A profile describes one metric source. Every signed-in reader lists the
active ones at `GET /api/v1/profiles`, as summaries. An administrator
lists, creates, reads, revises, and deletes them under
`/api/v1/admin/profiles` and `/api/v1/admin/profiles/{id}`; every change is
a new immutable revision, a revise names the revision it was read at and
answers `PROFILE_REVISION_STALE` when another change came first, and a
delete keeps every revision. `POST /api/v1/admin/profiles/{id}/test` runs
the preflight of a report against the source through the outbound policy
and answers `{"ok","code","capabilities"}`, never a URL, a name, an
address, or the text of a failure.

`admin import-profiles` creates or revises profiles from a TOML file and is
repeatable: a profile the file names and the database does not is created,
one whose document differs is revised, one that is equal is left as it is,
and one the file does not name is left alone. The whole file is validated
before anything is written, and an unknown key, an inline credential, or a
repeated id refuses it.

```toml
[[profiles]]
id = "prod"
display_name = "Production"
endpoint_url = "https://prometheus.example.invalid/api/v1"
cluster_name = "prod"
allowed_namespaces = ["payments", "checkout"]
credential_file = "prometheus-bearer"
active = true

[[profiles]]
id = "stage"
display_name = "Staging"
endpoint_url = "https://prometheus.example.invalid/api/v1"
cluster_name = "stage"
allowed_namespaces = ["payments"]
timeout_seconds = 30
permitted_periods = ["24h", "7d"]
default_period = "7d"

[profiles.thresholds]
warning_percent = 60
```

```bash
bin/k8s-capacity-report admin import-profiles --file profiles.toml --config platform.toml
```

`worker` reads the same deployment file and DSN. It applies the schema
under the same lock as `serve`, so the two start in any order, answers
`GET /health/live` and `GET /health/ready` on `--health-listen` (default
`:8091`), and runs the report jobs through River, the job queue that lives
in the same database. On a stop signal it takes no new job, waits
`worker_drain_seconds` for the running ones, then cancels them; a cancelled
job returns to the queue and runs again from the start.

```bash
bin/k8s-capacity-report worker --config platform.toml --health-listen :8091
```

These keys of the deployment file govern the reports; each shows its
default and its bounds.

| Key | Default | Bounds | Meaning |
|---|---|---|---|
| `worker_slots` | 2 | 1 to 16 | Report jobs one worker runs at once |
| `worker_drain_seconds` | 30 | 1 to 3600 | Wait for running jobs at a stop, before they are cancelled |
| `collect_timeout_seconds` | 900 | 60 to 3600 | Longest collection |
| `render_timeout_seconds` | 300 | 10 to 3600 | Longest compilation of one PDF |
| `job_max_attempts` | 3 | 1 to 10 | Attempts of a job whose source was unreachable or that failed inside |
| `report_retention_days` | 7 | 1 to 365 | Days a report keeps its snapshot and PDFs |
| `expired_report_days` | 30 | 1 to 365 | Days an expired report is kept as a record before it is removed |
| `admission_limit` | 20 | 1 to 1000 | Reports queued or running at once |
| `admission_profile_limit` | 5 | 1 to `admission_limit` | Reports queued or running at once for one profile |
| `admission_retry_seconds` | 5 | 1 to 300 | The wait a refused request is told to keep |

A report is collected once, stored as a snapshot, and compiled into one
PDF for each language. A collection whose source cannot be reached is tried
again, up to `job_max_attempts`; any other failure is final and stored as a
code. After `report_retention_days` the snapshot and the PDFs are deleted
and the report is kept as expired; after `expired_report_days` more it is
removed. Once a minute the worker also removes sign-in records whose block
is over and sessions that ended longer than a session lifetime ago.

`serve`, `worker`, and `admin` exit with 0, with 1 for a failure at run time
(the database, the listener, Typst for `worker`, jobs that did not stop
after the drain, a refusal of `bootstrap-admin`), and with 2 for the
arguments or the settings file.

## Commands

```bash
make build            # bin/k8s-capacity-report
make test             # Go tests that need no service and no tool
make lint             # gofmt, go vet, golangci-lint
make image            # runtime image and test image
make test-toolchain   # checks typst and pdftotext inside the test image
make test-template    # compiles the template and writes reports in the test image
make test-golden      # compares rendered pages with the golden pages
make generate         # writes the Go types of api/openapi.yaml
make generate-check   # fails when the committed generated file is stale
make postgres-up      # starts PostgreSQL 18 for the platform tests, through Compose
make postgres-down    # removes it, with its volume
make test-postgres    # postgres-up, the tests tagged postgres (and the worker's with typst), postgres-down
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
