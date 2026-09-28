# Kubernetes Capacity Report

`kube-capacity-report` is a metrics-only tool for generating Kubernetes capacity reports from a Prometheus-compatible API. It is provider-independent and does not use kubeconfig, ServiceAccounts, Kubernetes APIs, or cloud credentials, in either the CLI or the optional web platform described below.

The CLI writes a native `.typ` file only and compiles nothing; Typst compilation is intentionally out of scope for the CLI. The optional web platform compiles PDFs itself, inside its own container image, using the same pinned Typst binary and `graphite-report` package the CLI's own `typst`-marked tests exercise.

## Requirements

- Python 3.11 or newer.
- A Prometheus-compatible HTTP API.
- An optional Bearer token, only when the selected endpoint requires it.
- No Kubernetes or cloud API access.
- No Typst executable is required to generate the `.typ` source.

## Installation

Install the package:

```bash
python -m pip install .
```

Install it in editable mode for development and tests:

```bash
python -m pip install -e .[test]
```

## Configuration

The CLI reads TOML first, then environment variables, then command-line overrides.

```toml
[metrics]
url = "https://grafana.example/api/datasources/uid/thanos/resources/api/v1"
endpoint_type = "auto"
token_env = "GRAFANA_TOKEN"
tls_verify = true
ca_bundle = ""
timeout_seconds = 90

[cluster]
name = "production-east"
label = "cluster"
filter_mode = "required"
namespaces = ["ecommerce", "payments"]
nodepool_label = ""

[analysis]
period = "24h"
timezone = "America/Sao_Paulo"
node_autoscaling = "auto"
excluded_containers = []

[thresholds]
warning_percent = 70
critical_percent = 85
low_utilization_percent = 30
autoscaler_near_max_percent = 80
bursty_gap_percent = 100
nodepool_spread_points = 30
hpa_replica_divergence = 1

[report]
output_directory = "."
profile = "detailed"
min_severity = "all"
organization = ""
classification = ""
graphite_package = "@local/graphite-report:1.0.0"
```

Rules that matter in practice:

- `cluster.name` is required.
- `cluster.label` defaults to `cluster`.
- `cluster.filter_mode` defaults to `required`.
- TLS verification is enabled by default.
- `--insecure` disables TLS verification explicitly.
- The token is optional unless the endpoint policy requires it.
- Sidecars are included by default; only empty container names and `POD` are excluded automatically.
- `metrics.token` is rejected in the file. Point `metrics.token_env` at an environment variable, or pass `--token`, which is held in memory for the run and never written back to configuration.
- An invalid TOML file fails with a message naming the line and column, and exit code 2.
- Not every threshold is a percentage. `nodepool_spread_points` is a spread in percentage points between the nodes of one pool, `bursty_gap_percent` is a relative variation that may exceed 100, and `hpa_replica_divergence` is a replica count: the absolute gap between the replicas observed and the replicas desired that raises a finding. Only the plain percentages are validated against the 0..100 range.

## Endpoints

Three endpoint styles are supported:

- Direct Prometheus or Thanos Query API.
- Grafana datasource proxy to a Prometheus-compatible backend.
- Any other HTTP API root that speaks the Prometheus `/query` and `/query_range` contract.

Examples:

```bash
kube-capacity-report --config report.toml
kube-capacity-report --config report.toml --endpoint-type prometheus
kube-capacity-report --config report.toml --endpoint-type grafana --token "$GRAFANA_TOKEN"
```

The URL may point either at the API root or at the server root. When it points at the server root, `/api/v1` is appended and probed automatically, so `http://localhost:9000` and `http://localhost:9000/api/v1` both work. A path that answers with something other than a Prometheus response is treated as the wrong candidate and skipped rather than aborting the run.

Every query travels as `POST` with a form-encoded body, on both `/query` and `/query_range`: a cluster with many namespaces builds a selector long enough for a proxy to answer `414` to the same query sent in the URL. There is no fallback to `GET`. A path that refuses the verb with `405` is the wrong candidate, like any other path that does not speak the Prometheus API, so an endpoint or proxy that accepts only `GET` cannot serve the report.

Token requirements depend on the endpoint, not on the report itself.

## Usage

Default report:

```bash
kube-capacity-report --config report.toml
```

Focused report:

```bash
kube-capacity-report --config report.toml --min-severity attention
```

Executive profile:

```bash
kube-capacity-report --config report.toml --report-profile executive
```

Explicit output path:

```bash
kube-capacity-report --config report.toml --output report.typ
```

Preflight only:

```bash
kube-capacity-report --config report.toml --check
```

Useful CLI flags:

- `--min-severity all|attention|critical`
- `--report-profile detailed|executive`
- `--check`
- `--output report.typ`
- `--cluster`, `--cluster-label`, `--cluster-filter`

## Web platform

An optional on-demand web platform sits on top of the same metrics-only collector: it lets an authenticated operator generate a namespace-subset capacity report from a browser, watch progress, read the result as interactive HTML, and download a PDF. It carries the same invariant as the CLI -- no Kubernetes API access, no kubeconfig, no cloud credentials, metrics-only visibility -- and adds nothing that changes it.

Install it as an extra, kept separate from the base CLI so a plain `python -m pip install .` stays dependency-light:

```bash
python -m pip install '.[platform]'
```

That extra adds a second console script alongside the unchanged `kube-capacity-report`:

- `kube-capacity-report` -- the CLI, unchanged.
- `kube-capacity-report-platform` -- the platform's operator command line (`bootstrap-admin`, `generate-key`, `rotate-secrets`, `migrate`, `worker`, `probe-web`, `probe-worker`).

### Compose quick start

The platform's first supported packaging is Docker Compose (`deploy/compose.yaml`): four services -- `web`, `worker`, `postgres`, `minio` -- plus a fake Prometheus fixture that only exists in the integration overlay, never in production. The overlay ships three active profiles against that fixture -- `Integration`, `Integration (secondary)`, and the wider `Integration (wide)` (two namespaces, five workloads, two HPAs, three nodes), which must be generated with both its namespaces selected since the fixture answers every series in a matched entry regardless of the namespace selector. Building the image needs a checkout of the `graphite-report` package next to this repository -- Compose's build context defaults to `../../graphite-report`, a sibling directory of the repository root; a worktree or any other layout needs `GRAPHITE_REPORT_CONTEXT=<path>` set in `deploy/.env` pointing at it. To start it:

```bash
cp deploy/.env.example deploy/.env               # fill in the blanks
cp config/platform.example.toml deploy/platform.toml
mkdir -p deploy/secrets/metrics deploy/ca
printf '%s' '<MINIO_ROOT_USER from deploy/.env>' > deploy/secrets/metrics/s3-access-key        # replace with the real value
printf '%s' '<MINIO_ROOT_PASSWORD from deploy/.env>' > deploy/secrets/metrics/s3-secret-key    # replace with the real value
docker compose -f deploy/compose.yaml build web
docker run --rm --user 0 --volume "$PWD/deploy/secrets:/out" \
  --entrypoint sh kube-capacity-report:local -c \
  'python -m kube_capacity_report.platform.commands generate-key --keyring /out/kcr-keyring.json --key-id 2026-09 \
   && chown 10001:10001 /out/kcr-keyring.json /out/metrics/s3-access-key /out/metrics/s3-secret-key \
   && chmod 0400 /out/metrics/s3-access-key /out/metrics/s3-secret-key'
docker compose -f deploy/compose.yaml up -d
printf '%s\n' 'the admin password' | docker compose -f deploy/compose.yaml run --rm -T \
  -e KCR_BOOTSTRAP_MODE=1 web bootstrap-admin --username admin --password-stdin
```

The keyring step is a plain `docker run`, not `docker compose run`, because the `web` service's `cap_drop`/`read_only` settings and its read-only `./secrets` mount leave `compose run` unable to write the file even as root; a plain container keeps Docker's default capabilities, so it can write into the bind mount, `chown` the keyring and the two metrics credential files to the image's UID (10001), and lock the credential files down to mode 0400 -- a wrong value in either credential file is not rejected at startup, since boto3 builds its client without a network call: the `web` service fails only at the first PDF download, while the `worker` reports it at its next readiness poll (every 15 s) as `pdf_storage: degraded` in `/ready`'s JSON body plus a `PDF_STORAGE_DEGRADED` log line (its readiness HTTP status stays 200 by design). If `deploy/.env` sets `KCR_IMAGE_TAG`, use that tag in place of `local` in the `docker run` command above -- plain `docker run` does not read `deploy/.env`. The first `up` migrates the schema through the `web` verb (a startup coordinator, Compose only) before Uvicorn starts; `bootstrap-admin` is a one-off, gated by `KCR_BOOTSTRAP_MODE=1`, and is the only way an installation gets its first administrator. Full detail -- including the Kubernetes equivalent -- lives in `docs/web-platform-operations.md`.

### Profiles, retention, and PDF

Endpoints, credentials, and thresholds are configured as profiles: a **file profile** shipped inactive in the mounted `platform.toml` (`config/platform.example.toml`), or a **database profile** an administrator creates through the API; a file profile always wins over a database profile with the same id. Credentials and CA bundles are never inline -- a profile carries a named reference (`credential_reference`, `ca_bundle_reference`) resolved at execution time under `credential_reference_dir`/`ca_reference_dir`, so a copy of the configuration discloses no secret.

By default report retention is **seven days**, the queue runs **two local worker slots**, and there is **no 15-day cap** on how far back a report's period can look -- only a warning in the generate form for a lookback long enough to hit Thanos Store Gateway's slower path. A generated report has two independent artifacts: an HTML snapshot, available as soon as collection succeeds, and a PDF compiled from that same snapshot. A failed PDF compile never invalidates the HTML, and **PDF retry** replays the stored snapshot through the Typst toolchain with zero Prometheus calls -- collection never happens twice for one job. There are no schedules: every report is generated on demand.

Operator procedures (backup/restore, key rotation, session revocation, worker drain, lease reaping, retention reconciliation, scaling, rollback, and the Kubernetes packaging under `deploy/kubernetes/base`) are in `docs/web-platform-operations.md`. Trust boundaries, authentication, secrets, and SSRF controls are in `docs/web-platform-security.md`. The release qualification procedure is in `docs/web-platform-release-checklist.md`.

## Typst output

The generated document uses `graphite-report` for the A4 page shell and keeps
all report-specific editorial helpers in the Python renderer. It contains:

- a centered opening page with environment, author, and emission date;
- a second page with outline, metadata, status legend, and data-quality summary;
- generic recurring header, classification footer, and page numbering;
- topic-specific tables with human-readable CPU, memory, count, and percentage values;
- an HPA history section listing, per namespace, the configured minimum and maximum, the average, minimum and peak replicas observed, and whether each autoscaler reached its ceiling;
- an executive table consolidating workloads that hit their replica ceiling with Deployments that have no autoscaler;
- a compact executive summary and a traceable technical quality appendix.

An autoscaler configured with the same minimum and maximum is reported as pinned rather than as
having reached its ceiling, since it cannot scale by design.

Identifiers are emitted verbatim, so namespace, workload, and node names can be copied out of the
generated document and pasted into other tools.

Reader-facing report text is Portuguese (Brazil) and uses one vocabulary everywhere. Findings,
recommendations, and data-quality reasons, warnings, and notices are written once, in their final
wording, by the code that produces them; the Typst document and the web platform print the same
sentence, and `--verbose` prints the same warnings and notices on the command line. The vocabulary
is *solicitação* (request), *limite* (limit), *carga de trabalho* (workload), *pool de nós* (node
pool), *dimensionamento* (right-sizing), *margem* (headroom), *escalonamento automático*
(autoscaling), and *limitação de CPU* (throttling). Kubernetes names such as `Deployment`,
`StatefulSet`, `HPA`, `OOMKill`, and namespace stay as they are, and so do finding codes. A node
pool that could not be identified is shown as *Indeterminado*.

The generated document never prints a metric or recording-rule name: its technical appendix
describes the coverage of each capability in prose. The web platform's data quality tab lists the
evidence behind each capability, which is usually a list of metric and recording-rule names, and
`--verbose` prints the identifier-shaped entries of that evidence as query families.

## Data quality

The report keeps these states distinct:

- `available`
- `partial`
- `unavailable`

It also preserves the chosen strategy:

- recording-rule data when available
- raw-metric fallback when needed
- configured autoscaling state when supplied
- unknown autoscaling when no evidence exists

A value in the report is a number or one of two reader states, and the three are never merged:

- a measured zero is printed as zero;
- *Indisponível* (legend: "Dado indisponível") means the value could not be measured, because the
  metric is missing or its capability is partial or unavailable;
- *Não definido* in the document and *Sem base* in the web tables (legend: "Sem base de cálculo")
  means the measurement exists and the base of the ratio does not, over the whole period: the
  percentage of a request or of a limit for a workload or a namespace that has none configured,
  or whose configured value was zero throughout. The collector never assigns this state to a
  node. When the base exists for part of the period, the percentage is computed from that part
  and this state is not shown. When the request or limit series is absent altogether, the state
  is reported only if those metrics are themselves available, so a missing scrape is never
  blamed on configuration.

Workload inventory is discovered from the `kube_*_labels` metric families when the cluster
publishes them, and from the `kube_*_replicas` families otherwise. Many kube-state-metrics
deployments do not expose the label families without an explicit allowlist.

A workload with no observed consumption during the period is omitted from the report, including a
Deployment, StatefulSet, or DaemonSet scaled to zero replicas and a CronJob that did not run within
the window. Because the report is a capacity survey based on real resource usage, a workload at zero
has nothing to report and would only compete for attention with workloads that are consuming
resources. Namespace-level resource consumption is unaffected and continues to include everything
measured.

When an autoscaler target cannot be resolved unambiguously, only the affected namespace loses its
coverage result; the remaining namespaces are reported normally and the partial coverage is
disclosed in the report.

Nodepool confidence can be exact, inferred, or unknown. Historical ratios may be approximate when only fallback data is available.

## Security

- TLS verification is on by default.
- A CA bundle can be provided when the endpoint uses a private CA.
- `--insecure` is explicit and should be used only when you accept the risk.
- Bearer tokens and Authorization headers are redacted from handled errors, diagnostic output, and generated reports.
- An unexpected failure can still print a Python traceback, which does not pass through the redactor. Tokens travel in request headers, not in URLs, so they are not part of a traceback, but treat traceback output as unfiltered.
- Secrets are not persisted by the tool.

## Development and tests

Run the test suite with:

```bash
docker compose -f deploy/compose.test.yaml up -d --wait
TEST_POSTGRES_DSN='postgresql+psycopg://kcr_test:kcr_test_only@127.0.0.1:55432/kcr_test' \
  python -m pytest -q
```

The web platform's tests carry the `postgres` marker and need that PostgreSQL server: without
`TEST_POSTGRES_DSN` they fail rather than skip. `python -m pytest -q -m "not postgres"` runs
everything else with no service at all.

The frontend has its own checks, run from `frontend/` after `npm ci`: `npm test` (Vitest),
`npm run build` (type check and production build), and `npm run e2e` (Playwright against a running
Compose stack; `KCR_E2E_CHROMIUM` points it at a system Chromium instead of a downloaded one).

Two guards keep the report vocabulary from drifting. `tests/test_vocabulary.py` reads every string
literal in `collectors/`, `metrics/` (capability discovery included), `analysis/`, and `models.py`,
and fails on one that is neither final reader text nor declared internal; it also scans the
goldens.
`frontend/tests/vocabulary.test.tsx` renders every report tab and the profile form and scans the
visible text and the accessible names.

Six checks carry the `typst` marker and verify the generated document itself: one compiles every
golden against a stub of the package's public API; one asserts that a control character reaches the
PDF text layer as the character itself rather than as a literal escape sequence; one compiles
against the real package and asserts that identifiers survive the PDF text layer unchanged; one
reads the package's callout markers and asserts the copy this repository keeps of them is still
accurate, so the status glyphs cannot start contradicting a marker unnoticed; one compiles a stored
report snapshot through the platform's own pinned-toolchain path -- `PdfToolchain.probe` then
`compile_snapshot_pdf` -- against the real binary and package, and reads the PDF text layer back;
one compiles an identifier long enough to wrap inside a narrow table cell and asserts the PDF text
layer never repeats the hyphen at the line break, with a control rendering (no `lang: "en"`) that
must reproduce the defect, or the main assertion would be meaningless.
Each skips, with an
explicit reason, when `typst`, `pdftotext`, or `GRAPHITE_ROOT` is unavailable -- and a skipped check
is not an executed check, so confirm the reported count and not merely the absence of failures. Run
them before closing a release:

```bash
export GRAPHITE_ROOT=/path/to/graphite-report
PROJECT_PYTHON="$(command -v python)"   # the interpreter this package is installed in
nix shell nixpkgs#poppler-utils --command \
  nix develop "path:$GRAPHITE_ROOT" --command \
  "$PROJECT_PYTHON" -m pytest -m typst -q -rs
```

`typst` comes from the sibling package's devshell and `pdftotext` from `poppler-utils`, which
neither devshell ships, hence the two nested wrappers; without `pdftotext` four of the six checks
skip. The devshell also puts its own `python` first on `PATH`, one without this package or its
dependencies, which is why the command names the project's interpreter instead of calling
`python`.

The repository keeps the original AKS/OKE scripts and local Nix Flakes, `.direnv`, and `.envrc` untracked as development references only.

`AGENTS.md` documents the working agreement: the correction roadmap, the per-branch cycle, and the
changelog and versioning rules. `CHANGELOG.md` records what shipped in each version. `ROADMAP.md`
tracks open findings and is deliberately untracked.

## Limitations

- Metrics-only visibility means neither the CLI nor the web platform can inspect Kubernetes objects directly.
- The CLI performs no PDF generation, chart generation, or provider-specific inference; it writes `.typ` source only. The web platform compiles PDFs itself, from the same kind of collected data, inside its own container image.
- Some historical ratios depend on the retained metric window exposed by Prometheus.
