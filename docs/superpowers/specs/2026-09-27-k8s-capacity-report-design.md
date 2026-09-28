# k8s-capacity-report: design

Date: 2026-09-27
Status: approved by the owner on 2026-09-27

## 1. Purpose

`k8s-capacity-report` generates Kubernetes capacity reports from a
Prometheus-compatible API. It reads metrics only. It never uses a kubeconfig,
the Kubernetes API, or cloud credentials.

It replaces two projects with one repository:

- `kubernetes-report` 1.11.0, a Python collector, analyser, and web platform;
- `graphite-report` 1.0.0, a Typst package for monochrome corporate reports.

The original Python source is kept at `older/kubernetes-report/` as read-only
reference. That folder is never tracked by Git.

### Goals

1. Written in Go.
2. One repository for collector, BFF, frontend, and report template.
3. A web frontend served by a BFF (backend for frontend).
4. Portuguese (Brazil) and English, chosen by the reader.
5. Every key point of both originals is kept.

### Non-goals

- Kubernetes manifests. They are a later phase and start only when the owner
  asks for them.
- Single sign-on.
- Scheduled reports. Every report is generated on demand.
- Reading or migrating data written by the Python platform.
- Charts, Markdown conversion, DOCX output, or any organisation's branding.
- Publication of the template to Typst Universe.
- Nix files. The owner adapts the project if it moves to a NixOS host.

## 2. Decisions

| Topic | Decision |
|---|---|
| Language | Go 1.27 |
| Repository | One, branch `main` |
| Binary | One, named `k8s-capacity-report`, with commands `report`, `serve`, `worker`, `admin` |
| PDF | Typst, pinned at exactly 0.15.1, template inside the repository |
| Template input | A JSON data file, not generated Typst source |
| Report language | Chosen by the reader for interface, report view, and PDF |
| Wording | Stored nowhere in the snapshot; produced from one Go catalog |
| Sign-in | Local accounts with roles administrator, operator, reader |
| Report visibility | Every signed-in user sees every report |
| Frontend | React, TypeScript, Vite; static files served by the BFF |
| API contract | One OpenAPI file; Go and TypeScript types generated from it |
| Database | PostgreSQL holds data, job queue, snapshots, and PDFs |
| Queue | River, on PostgreSQL |
| Packaging | Docker Compose |
| Development | macOS with native Go; exact-output checks run in the Linux image |
| Documents | `README.md`, `AGENTS.md`, `CHANGELOG.md`, `ROADMAP.md` in English |

## 3. Deliberate differences from the originals

| Original | This project | Reason |
|---|---|---|
| Sentences written once in Portuguese and stored in the snapshot | Snapshot stores codes and numbers; sentences come from a catalog | Two languages from one stored report |
| Python generates Typst source and escapes every value | Go writes a data file; the template inserts values as text | Removes a whole class of escaping defects |
| Template installed as `@local/graphite-report:1.0.0` through a script that creates links | Template imported by relative path inside the repository; its folder stays a valid Typst package | No install step, no version string repeated across fixtures |
| One template diagnostic written in Portuguese | Every diagnostic in English | Diagnostics are for operators and developers |
| Typst 0.15.0 | Typst 0.15.1 | Owner's decision |
| PDFs in S3-compatible object storage | PDFs in PostgreSQL | One store, one backup, no storage credentials |
| File profiles override database profiles | Database profiles only, plus `admin import-profiles` | One source, no precedence rule |
| A credential may be pasted into a profile and is encrypted with a keyring | A profile only names a credential file mounted into the worker | No secret is ever stored in the database, so no keyring is needed |
| Settings editable by an administrator in the interface | Settings come from the deployment file only | One source |
| Roles `admin` and `user`; a report is visible to its owner and to administrators | Three roles; every report visible to every signed-in user | Owner's decision |
| Report author hard-coded as "Time SRE" | `report.author` is a configuration value | The tool is generic |
| Web labels copied by hand from the document labels, already drifted | The BFF sends the report already worded | One place for every label |
| Version repeated in seven files | One `VERSION` file | Nothing to keep in step |
| Tests skip when a tool is missing | Tests fail when a tool is missing | A skipped gate is not a checked gate |
| Test harness and tooling in Python | Everything in Go, frontend in TypeScript | One toolchain per side |

Accepted consequences:

- Profiles owned by the deployment lose their read-only guarantee. After
  import, an administrator can edit them in the interface.
- The web view of an old report follows the current catalog wording. A PDF
  already compiled keeps the wording it was compiled with.

## 4. Architecture

### 4.1 Repository layout

```text
cmd/k8s-capacity-report/   entry point; wires the commands
cmd/fake-prometheus/       test fixture server; never in the production image
internal/
  config/      file, then environment, then flags
  prom/        HTTP client, endpoint candidates, selectors, escaping
  discovery/   cluster scope and capability discovery
  collect/     namespaces, workloads, nodes, autoscalers, cluster autoscaling
  resolve/     workload, node, and node pool identities
  analysis/    findings, recommendations, finding registry
  snapshot/    stored contract, version 1.0
  catalog/     Portuguese and English wording and formats
  document/    snapshot + language + profile -> report.json
  typst/       toolchain probe and compile
  redact/      secret removal from text
  platform/
    api/       generated interfaces and handlers
    auth/      accounts, sessions, CSRF, throttle
    audit/     append-only event log
    netpolicy/ outbound request policy
    profiles/  metric sources and their revisions
    reports/   requests, state, admission, idempotency
    worker/    job handlers
    store/     PostgreSQL access and migrations
    retention/ expiry
api/openapi.yaml           the BFF contract
template/
  graphite/    generic package: typst.toml, lib.typ, src/, assets/fonts/,
               examples/, starter, README.md
  capacity/    layout of this report
web/           React application
deploy/        Containerfile and Compose files
testdata/      Prometheus scenarios, golden files
docs/reference/            original documents and inventories of their source
docs/superpowers/          specs and plans
VERSION
Makefile
```

### 4.2 Commands

| Command | Purpose | Needs |
|---|---|---|
| `report` | Collect and write a PDF or a snapshot | Prometheus; `typst` for PDF |
| `serve` | The BFF: API, sign-in, static frontend | PostgreSQL |
| `worker` | Run report jobs and PDF compiles | PostgreSQL, Prometheus, `typst` |
| `admin migrate` | Apply database migrations | PostgreSQL |
| `admin bootstrap-admin` | Create the first administrator | PostgreSQL |
| `admin import-profiles` | Load profiles from a TOML file | PostgreSQL |
| `admin probe` | Check the health address of `serve` or `worker` | Nothing |

### 4.3 Data flow

```text
Standalone
  config -> prom -> discovery -> collect + resolve -> analysis
         -> snapshot -> document (language, profile) -> typst -> PDF

Platform
  browser -> BFF -> report row + job, one transaction
  worker:  profile revision -> collection and analysis -> snapshot, stored
           -> one PDF per language, each stored with its own status
  reading: BFF turns snapshot + reader's language into the report view
```

### 4.4 Structural rules

- **Imports point one way.** Each package in the flow may import the ones
  before it. `platform/` uses the core; the core never imports `platform/`.
- **The snapshot is the only handover.** Everything after `analysis/` reads the
  snapshot and nothing else.
- **Wording lives only in `catalog/`.** No other package contains a sentence
  meant for a reader.
- **Metrics only.** No Kubernetes or cloud client may appear in `go.mod`.
- **Two template layers.** `graphite/` is generic and carries no brand.
  `capacity/` holds everything specific to this report.

Tests enforce the first, third, and fourth rules (section 13).

## 5. Standalone command

### 5.1 Usage

```bash
k8s-capacity-report report --config report.toml --lang en --output report.pdf
k8s-capacity-report report --config report.toml --check
k8s-capacity-report report --config report.toml --snapshot cluster.json
k8s-capacity-report report --from-snapshot cluster.json --lang pt-BR --output report.pdf
```

- `--check` runs the preflight only: endpoint, cluster scope, capabilities.
- `--snapshot` writes the snapshot and needs no `typst`.
- `--from-snapshot` makes no Prometheus call.
- A PDF needs `typst` 0.15.1 on `PATH`, or the path in `KCR_TYPST`. Any other
  version is an error.
- Output files are written atomically.

### 5.2 Configuration

Order of precedence: TOML file, then environment variables, then flags.
Unknown sections or keys are errors. An invalid file fails with its line and
column.

| Key | Type | Default | Rule |
|---|---|---|---|
| `metrics.url` | string | required | Absolute `http` or `https` URL; no user info, query, or fragment |
| `metrics.endpoint_type` | string | `auto` | `auto`, `prometheus`, `grafana` |
| `metrics.token_env` | string | empty | Name of the variable holding the token |
| `metrics.tls_verify` | boolean | `true` | |
| `metrics.ca_bundle` | path | empty | Must exist when set |
| `metrics.timeout_seconds` | number | `90` | Greater than zero |
| `cluster.name` | string | required | |
| `cluster.namespaces` | list | required | At least one |
| `cluster.label` | string | `cluster` | Valid Prometheus label name |
| `cluster.filter_mode` | string | `required` | `required`, `auto`, `none` |
| `cluster.nodepool_label` | string | empty | Valid label name when set |
| `analysis.period` | string | `24h` | `^[1-9][0-9]*[mhd]$` |
| `analysis.timezone` | string | `America/Sao_Paulo` | IANA zone |
| `analysis.node_autoscaling` | string | `auto` | `auto`, `enabled`, `disabled` |
| `analysis.excluded_containers` | list | empty | |
| `thresholds.warning_percent` | number | `70` | 0 to 100, below critical |
| `thresholds.critical_percent` | number | `85` | 0 to 100 |
| `thresholds.low_utilization_percent` | number | `30` | 0 to 100 |
| `thresholds.autoscaler_near_max_percent` | number | `80` | 0 to 100 |
| `thresholds.nodepool_spread_points` | number | `30` | 0 to 100, percentage points |
| `thresholds.bursty_gap_percent` | number | `100` | Zero or more, no upper limit |
| `thresholds.hpa_replica_divergence` | number | `1` | Zero or more, a replica count |
| `report.profile` | string | `detailed` | `detailed`, `executive` |
| `report.min_severity` | string | `all` | `all`, `attention`, `critical` |
| `report.language` | string | `pt-BR` | `pt-BR`, `en` |
| `report.organization` | string | empty | |
| `report.classification` | string | empty | |
| `report.author` | string | empty | |
| `report.output_directory` | path | `.` | Must exist and be writable |

`metrics.token` in the file is rejected. The token comes from the variable
named by `metrics.token_env` or from `--token`, and is held in memory only.

Environment variables keep the `KCR_` prefix of the original:
`KCR_METRICS_URL`, `KCR_ENDPOINT_TYPE`, `KCR_CLUSTER`, `KCR_CLUSTER_LABEL`,
`KCR_CLUSTER_FILTER`, `KCR_NAMESPACES`, `KCR_PERIOD`, `KCR_TIMEZONE`,
`KCR_NODE_AUTOSCALING`, `KCR_MIN_SEVERITY`, `KCR_REPORT_PROFILE`,
`KCR_OUTPUT_DIRECTORY`, plus the new `KCR_LANGUAGE` and `KCR_TYPST`.

Flags: `--config`, `--metrics-url`, `--endpoint-type`, `--token`, `--insecure`,
`--cluster`, `--cluster-label`, `--cluster-filter`, `--namespaces`, `--period`,
`--timezone`, `--node-autoscaling`, `--min-severity`, `--report-profile`,
`--lang`, `--output`, `--snapshot`, `--from-snapshot`, `--check`, `--verbose`.

### 5.3 Exit codes

| Code | Meaning |
|---|---|
| 0 | Success |
| 2 | Configuration error |
| 3 | Metrics error |
| 4 | Cluster scope error |
| 5 | Insufficient capabilities |
| 6 | Output error, including a missing or wrong `typst` |

### 5.4 Redaction

Every message printed or logged passes through `redact/`, in this order:
literal secrets; quoted `authorization` values; bare `authorization` lines;
`token=`, `access_token=`, and `api_key=` assignments; URL query strings; URL
user info. A recovered panic is redacted before it is printed.

## 6. Collection

Behaviour is ported from the Python source, which is the reference for every
detail not repeated here. `docs/reference/inventory/collection.md` holds
the inventory taken on 2026-09-27.

### 6.1 Prometheus client

- Every query is `POST` with a form-encoded body, on `/query` and
  `/query_range`. There is no fallback to `GET`.
- Timeout is 90 seconds per request. Three attempts, with exponential backoff,
  on connection failure, 429, and 5xx. `Retry-After` is honoured up to 60
  seconds.
- Proxy variables from the environment are ignored.
- Redirects are never followed.
- Candidate roots are the given URL and the URL plus `/api/v1`. A candidate
  that answers 404, 405, or a body that is not a Prometheus response is the
  wrong candidate and is skipped. Any other error stops the run.
- Once a candidate has answered correctly, a later malformed answer is a
  metrics error, not a wrong candidate.
- The probe query is `vector(1)`.
- A sample that is `NaN` or infinite is "no data". It is never zero.

### 6.2 Cluster scope

The three filter modes `required`, `auto`, and `none` behave as in the
original, including the errors raised when several cluster values are visible.

### 6.3 Capabilities

Nine capabilities, in this order: `namespace_usage`, `workload_usage`,
`requests_limits`, `hpa_history`, `workloads_without_hpa`, `ip_node_mapping`,
`nodepool_discovery`, `node_utilization`, `node_autoscaling`.

- State is `available`, `partial`, or `unavailable`.
- Strategy is `recording_rule`, `raw_metrics`, `configured`, or `none`.
- Recording rules are preferred; raw metrics are the fallback.
- Evidence lists the metric names found. Collectors read the evidence to
  choose between unified and legacy metric names.

### 6.4 Collectors and resolvers

- The 17 query shapes of the original are ported exactly, including label
  order inside selectors and the two-step escaping of regular expressions.
- The range step comes from a fixed table: up to 30 minutes `30s`, up to 6
  hours `2m`, up to 24 hours `5m`, up to 7 days `15m`, otherwise `1h`. No
  alignment is added.
- The period start is the emission time minus the period, with no
  daylight-saving correction, as in the original.
- Time zone data is embedded in the binary.
- Node pool identity follows the original order: configured label, known
  provider labels, AKS node name, OKE node name, shared name label, unknown.
- A workload with no observed consumption is left out of the inventory. Its
  consumption still counts in the namespace totals.
- Every output list is sorted, and warnings and notices keep first-seen order
  without duplicates, so that the same input always gives the same snapshot.

## 7. Analysis

`docs/reference/inventory/analysis.md` holds the inventory.

### 7.1 Findings

- The 66 finding codes of the original are kept unchanged.
- A finding carries: code, severity, scope type, scope kind, scope name,
  evidence as named values, and confidence. It carries no sentence.
- Trigger conditions and the choice of number (average or peak) are ported
  exactly.
- A finding is emitted at `healthy` severity when usage is below the warning
  threshold. The original does this and its tests do not check it; this
  project adds the test.

### 7.2 Finding registry

One Go table maps each code to:

- its message family, such as `resource_request_usage`;
- its fixed parameters, such as `resource = cpu`;
- the evidence names it must carry.

The registry is the single list of valid codes. The catalog guards in section
13 read it.

### 7.3 Severity and confidence

- Severity: `healthy`, `attention`, `critical`, in that order.
- Confidence: `exact`, `inferred`, `unknown`.
- The executive profile raises the minimum severity to `attention`.
- Minimum severity is applied when a report is shown or compiled, never when
  it is stored.

### 7.4 Recommendations

Recommendations have no wording of their own. They group the findings above
`healthy` by code, merge their scopes, take the highest severity and the most
exact confidence, and are ordered by severity, confidence, scope, and code.

## 8. Snapshot

The snapshot is one JSON document, `schema_version` `"1.0"`. It is a new
format and does not read snapshots written by the Python platform.

```text
report        id, generator version, emission time, period start and end,
              time zone, thresholds (all seven), non-secret settings,
              organization, classification, author,
              requested profile and minimum severity
scope         cluster name and label, filter mode, matcher state, confidence,
              discovered values, namespaces selected and excluded,
              excluded containers
quality       capabilities, warnings, notices, collection start and end
summary       counts, cluster autoscaling evidence
namespaces    per namespace: pods, statistics, approximate flag
workloads     per workload: kind, pods, statistics, autoscaler coverage
hpas          per autoscaler: target, limits, current and desired statistics
nodes         per node: pool, pool confidence, capacity, statistics, state
nodepools     per pool: confidence, node names
findings      section 7.1
recommendations  section 7.4
```

### 8.1 Statistics

```json
{ "state": "value", "average": 0.42, "minimum": 0.0, "peak": 0.9, "current": 0.4 }
{ "state": "unavailable", "average": null, "minimum": null, "peak": null, "current": null }
{ "state": "no_base", "average": null, "minimum": null, "peak": null, "current": null }
```

- `value`: at least one number is present. A measured zero is a number.
- `unavailable`: the value could not be measured.
- `no_base`: the measurement exists but the base of the ratio does not, over
  the whole period.
- Approximation is a flag on the namespace or workload row.

### 8.2 Warnings, notices, and capability reasons

Each is a code with named parameters, for example
`{ "code": "WORKLOADS_OMITTED_NO_USAGE", "params": { "count": 3 } }`. A
capability carries a list of reason codes, because the original can join two
reasons.

### 8.3 Rules

- A stored snapshot is never rewritten and never narrowed.
- Internal addresses, host names, provider identifiers, endpoint addresses,
  and tokens never enter the snapshot.
- The writer checks integrity: no repeated identity, no workload that is both
  an autoscaler target and listed as having none, no pool naming an absent
  node.
- The snapshot is stored with its SHA-256 digest. The digest is checked again
  before each PDF compile.
- The schema version changes only when the shape changes. A code is never
  reused for another meaning and never removed while its schema version is
  supported.

## 9. Catalog

### 9.1 Content

One TOML file per language, embedded in the binary: `pt-BR.toml` and
`en.toml`. The library is go-i18n, which provides named parameters and plural
forms.

The catalog holds every reader-facing string of the report:

- finding sentences and action sentences, by message family;
- warnings, notices, and capability reasons;
- section headings, table headers, and the status legend;
- fallback sentences for empty sections;
- short reason fragments used inside table cells;
- labels for severities, states, strategies, confidences, and capabilities;
- placeholders: unavailable, no base, undetermined pool, unidentified target;
- cover and footer text.

This is a few hundred entries per language.

### 9.2 Formats

Formats belong to the language and are implemented once in Go.

| Value | Portuguese | English |
|---|---|---|
| Decimal | `3,5` | `3.5` |
| Percent | `78,6%` | `78.6%` |
| CPU below one core | `500 mCPU` | `500 mCPU` |
| CPU | `3,5 cores` | `3.5 cores` |
| Memory | `1,25 GiB` | `1.25 GiB` |
| Date and time | `15/07/2026 14:30` | `07/15/2026 14:30` |
| Period | `24 horas` | `24 hours` |

Rounding follows the original: one decimal for percentages and millicores,
two for cores and memory, trailing zeros removed.

### 9.3 Vocabulary

| Concept | Portuguese | English |
|---|---|---|
| request | solicitação | request |
| limit | limite | limit |
| workload | carga de trabalho | workload |
| node pool | pool de nós | node pool |
| right-sizing | dimensionamento | right-sizing |
| headroom | margem | headroom |
| autoscaling | escalonamento automático | autoscaling |
| throttling | limitação de CPU | CPU throttling |
| label | rótulo | label |

- Kubernetes names (`Deployment`, `StatefulSet`, `DaemonSet`, `Job`,
  `CronJob`, `Pod`, namespace, `HPA`, `OOMKill`) and finding codes stay as
  they are in both languages.
- The Portuguese sentences are carried over unchanged from the original.
- No em-dash inside a sentence. The em-dash alone is the mark for an empty
  table cell.
- The node pool marker `unknown` is never shown raw. It reads *Indeterminado*
  or *Undetermined*.
- Where grammar differs by resource, the catalog has one entry per resource.
  A noun is never pasted into a sentence.

### 9.4 Who renders

Only Go turns codes into sentences. The frontend has its own small catalog
for the interface alone: navigation, forms, buttons, and error messages by
error code.

Operator diagnostics, meaning logs and command-line errors, are in English
and are not in the catalog.

### 9.5 Versioning

Changing a sentence in either language is a visible change and calls for a
minor version.

## 10. Document and template

### 10.1 Division of work

Go decides what is said. The template decides how it looks.

- `document/` builds `report.json` from a snapshot, a language, a profile, and
  a minimum severity.
- `template/capacity/` reads `report.json` and lays it out.
- `template/graphite/` provides the page shell.

### 10.2 Data file

`report.json` is a list of blocks:

| Block | Content |
|---|---|
| `front` | The opening matter, set in smaller type; holds other blocks |
| `outline` | Title and depth |
| `heading` | Level, text, and whether it is numbered |
| `paragraph` | Runs of text |
| `list` | Items, each made of runs |
| `table` | Table identifier, header texts, rows of cells |
| `callout` | Kind, optional title, runs of text |
| `pagebreak` | Weak or not |

A cell or paragraph is a list of runs. Each run has a kind:

| Kind | Rendering |
|---|---|
| `text` | Plain prose in the document language |
| `strong` | Prose in bold, for a label inside a sentence |
| `ident` | A real name: slightly smaller, no hyphenation, tagged as English |
| `placeholder` | A prose stand-in such as *Indeterminado*, in the document language |
| `status` | A status mark followed by its label |

The cover and the footer are not blocks. They are top-level parts of the data
file, because the generic layer receives them as arguments.

A heading that is not numbered is also left out of the outline. The original
numbers its sections and leaves unnumbered the headings that repeat per
namespace or per node pool.

Rules:

- A table names its identifier, for example `workload-resources`. Column
  widths, font size, and cell padding for each identifier live in the
  template. An unknown identifier stops the compile.
- The template never evaluates a value as markup. A test forbids `eval` in
  the template.
- `report.json` also carries the theme values that depend on language,
  `lang` and `region`, which drive hyphenation. Dates arrive already
  formatted, as text.
- The data file is strict in both directions: an unknown key is an error, and
  so is a field that no page uses. A field enters the contract with the
  feature that prints it.

### 10.3 Content

The detailed and executive profiles keep the section order, tables, columns,
sort orders, and fallback sentences of the original.
`docs/reference/inventory/document-frontend.md` holds the inventory.

- Status marks keep their shapes: healthy `●`, attention `▲`, critical `♦`,
  no base `○`, unavailable `—`. Meaning never depends on colour.
- The cover shows the author only when `report.author` is set.
- The footer starts on the second page and shows the classification, when
  set, and the page number.
- The technical appendix is prose and never prints a metric name.

### 10.4 Generic layer

`template/graphite/` is a port of `graphite-report` 1.0.0. The source is small,
nine Typst files, and is carried over with its structure intact.
`docs/reference/inventory/template.md` holds the inventory.

What stays:

- three public functions: `graphite-report`, `callout`, `keep-together`;
- every argument, default, label key, and theme key of version 1.0.0;
- `labels` and `theme` accept only the documented keys, and an unknown key is
  an error that names the key and lists the accepted ones;
- the report configuration is validated in one place, and each component
  validates its own arguments;
- every error message starts with `graphite-report:`;
- `callout` used outside a `graphite-report` document stops the compile;
- a callout does not split across pages unless `breakable: true` is passed;
- the compact opening, where a field that is not given takes no space;
- no brand, logo, or organisation name.

What changes:

- the one diagnostic written in Portuguese is rewritten in English;
- the manifest declares compiler 0.15.1;
- the manifest version equals `VERSION`, and a test checks it;
- examples and fixtures import the package by relative path.

The folder remains a valid Typst package, so it can still be used for other
reports. Installing it is a plain copy into the local package folder. One test
proves it: it copies the folder to a scratch package path and compiles the
starter document through the package name.

The layer keeps its Portuguese default labels. `capacity/` always passes the
language keys and every label that can reach the page, which are the three
that a callout without a title prints. A test confirms none of them is left
to the default.

Known limits, carried over and documented in the template README:

- text that does not fit in a table cell overflows without any warning;
- a small table can split at the end of a page unless wrapped in
  `keep-together`;
- header and footer content is not exposed to assistive technology, so
  anything essential also appears in the body.

### 10.5 Compile

```text
typst compile --root <work> --font-path <work>/graphite/assets/fonts \
  --ignore-system-fonts --ignore-embedded-fonts \
  --input data=/report.json <work>/capacity/main.typ <out>.pdf
```

- The template and fonts are embedded in the binary and unpacked into a work
  folder, which is the compile root. `report.json` is written there and named
  by a path relative to that root.
- `typst` is called by absolute path with an empty `PATH`.
- The version is checked before the first compile. A mismatch is a failure.
- The document date comes from the snapshot.
- A compiler warning is a failure.

### 10.6 Licences

| Content | Licence |
|---|---|
| Code and template | MIT |
| Liberation Sans fonts | SIL Open Font License 1.1 |

The font licence text stays beside the fonts, is embedded with them, and is
present in the container image.

## 11. Web platform

`docs/reference/inventory/platform.md` holds the inventory of the original.

### 11.1 Services

| Service | Runs | Notes |
|---|---|---|
| `serve` | `k8s-capacity-report serve` | BFF on port 8080 |
| `worker` | `k8s-capacity-report worker` | Two slots by default; health on port 8091 |
| `postgres` | PostgreSQL | |

### 11.2 Deployment settings

One TOML file mounted into `serve` and `worker`, plus `KCR_DATABASE_DSN` in the
environment. The file sets:

- session lifetimes, retention days, worker slots, lease and drain times;
- admission limits;
- allowed endpoint host names and address ranges;
- the folders holding credential files and CA bundles;
- the path of the server secret.

The server secret is a file of random bytes. It keys the login throttle, so
that the throttle table never lists attempted user names in readable form.

### 11.3 Accounts and roles

| Action | Administrator | Operator | Reader |
|---|---|---|---|
| Manage users | Yes | No | No |
| Manage and test profiles | Yes | No | No |
| Request a report, retry a PDF | Yes | Yes | No |
| Delete a report | Yes | Own requests | No |
| Open reports, download PDFs and snapshots | Yes | Yes | Yes |
| Change own password and language | Yes | Yes | Yes |

- The first administrator is created by `admin bootstrap-admin`. The password
  is read from standard input only. The command needs `KCR_BOOTSTRAP_MODE=1`
  and refuses when any user exists.
- Deleting a user deactivates the account.
- The role is read from the database on every request.

### 11.4 Authentication

| Control | Rule |
|---|---|
| Password hash | Argon2id; rehashed on sign-in when parameters change |
| Password policy | 12 to 1024 characters |
| Session token | 256 random bits; only its SHA-256 digest is stored |
| Session lifetime | 12 hours absolute, 1 hour idle |
| Cookie | `Secure`, `HttpOnly`, `SameSite=Lax`, `Path=/api` |
| Sign-in | Always issues a new session and revokes the one presented |
| Revocation | Role change, deactivation, and password change revoke every session of the user |
| Sign-in CSRF | Cookie and header must match, checked before credentials are read |
| Request CSRF | Token bound to the session row |
| Throttle | 5 failures in 300 seconds block for 300 seconds, keyed by HMAC of user name and client address |
| Unknown user | Costs one Argon2 verification against a dummy hash |
| Comparisons | Constant time |

### 11.5 Outbound request policy

Every request to a metric source passes this policy, ported in full from the
original:

1. Only `http` and `https`. No user info, query, fragment, control character,
   or dot segment.
2. These addresses are refused: unspecified, loopback, link-local, multicast,
   reserved, the IPv6 forms that embed IPv4 (NAT64, 6to4, Teredo), and the
   cloud metadata addresses. Only an allowed range naming exactly one address
   can lift a refusal.
3. Every resolved address must be allowed by host name or by range. One bad
   answer refuses the whole set.
4. The name is resolved again before each request. The connection goes to the
   literal address, and the peer address is compared after connecting.
5. Any redirect is refused.
6. Proxy variables are ignored.

### 11.6 Profiles

A profile describes one metric source.

| Group | Fields |
|---|---|
| Identity | Identifier, display name, active flag |
| Endpoint | Address, type, timeout, TLS verification |
| References | Credential file name, CA bundle file name |
| Cluster | Name, label, filter mode, node pool label, node autoscaling |
| Namespaces | Allowed namespaces, excluded containers |
| Choices | Permitted and default period, time zone, report profile, minimum severity |
| Thresholds | The seven thresholds |

- Every change adds a new revision. A revision is never updated or deleted,
  and a database trigger enforces it. A report keeps the revision it was
  requested with.
- An edit names the revision it started from and fails when that revision is
  no longer current.
- A reference is a file name. The worker reads the file when the job runs,
  outside any database transaction, from a folder it cannot leave. Files are
  limited to 1 MiB.
- The address is checked against the outbound policy for every candidate
  root.
- `admin import-profiles` creates or revises profiles from a TOML file and is
  repeatable.

### 11.7 Reports and jobs

```text
status   queued -> running -> collected | failed
phase    waiting -> preflight -> collecting -> analyzing -> snapshotting
         -> rendering -> complete
pdf      one row per language: pending -> ready | failed
```

- The report becomes `collected` in the same transaction that stores the
  snapshot and creates the two pending PDF rows. From then on the web view is
  available.
- A failed compile never changes the report status or the other language.
- A PDF retry compiles from the stored snapshot. Collection runs once per
  report.
- River runs the jobs. It keeps its own tables, picks up jobs abandoned by a
  crashed worker, and retries with backoff up to a limit.
- The report row holds the state the interface shows. The job handler updates
  it and completes the job in one transaction.
- A worker asked to stop takes no new job and finishes the current ones
  within the drain time.
- Admission limits apply to queued and running reports, in total and per
  profile. Over the limit, the BFF answers 429 with `Retry-After`.
- A request carries an `Idempotency-Key`. The same key and body return the
  same report. The same key with another body is a conflict.
- The browser asks for progress every 1.5 seconds, slowing to 6 seconds.

### 11.8 Storage and retention

- Snapshots and PDFs are stored in PostgreSQL with digest and size.
- Limits: 32 MiB per snapshot, 64 MiB per PDF. Larger output fails the job.
- Reports expire after seven days by default. Expiry deletes the snapshot and
  PDFs and marks the report expired, in one transaction.
- An expired report answers 410 for 30 days, then the row is deleted.
- Expiry runs every minute as a periodic job.
- There is no limit on the lookback period. The form warns above 15 days.

### 11.9 Audit

Every sign-in, sign-out, and change to users, profiles, and reports adds one
row to an append-only table: actor, time, action, target, outcome, and
metadata.

- The metadata writer refuses a key that looks like a secret (`password`,
  `token`, `cookie`, `hash`, `digest`, `secret`, `dsn`) and any value that is
  not a simple one. It records which fields changed, never their values.
- A throttled sign-in attempt writes no row.
- No API reads the table. Operators read it with SQL.

### 11.10 API

All routes are under `/api/v1` and are described in `api/openapi.yaml`.

| Route | Role | Purpose |
|---|---|---|
| `GET /auth/session` | None | Session state and sign-in CSRF token |
| `POST /auth/login` | None | Sign in |
| `POST /auth/logout` | Any | Sign out |
| `PATCH /account` | Any | Own password and language |
| `GET /profiles` | Any | Active profiles, summary only |
| `GET`, `POST /admin/profiles` | Administrator | List, create |
| `GET`, `PATCH`, `DELETE /admin/profiles/{id}` | Administrator | Read, revise, deactivate |
| `POST /admin/profiles/{id}/test` | Administrator | Preflight against the source |
| `GET`, `POST /admin/users` | Administrator | List, create |
| `GET`, `PATCH`, `DELETE /admin/users/{id}` | Administrator | Read, change, deactivate |
| `POST /reports` | Operator | Request a report |
| `GET /reports` | Any | List, by page |
| `GET /reports/{id}` | Any | Status and progress |
| `GET /reports/{id}/view?lang=` | Any | Report written in one language |
| `GET /reports/{id}/snapshot` | Any | Stored snapshot |
| `GET /reports/{id}/pdf?lang=` | Any | PDF in one language |
| `POST /reports/{id}/pdf-retries` | Operator | Retry one language |
| `DELETE /reports/{id}` | Operator | Delete |
| `POST /reports/delete` | Operator | Delete up to 200 |

"Operator" includes administrators. Health routes `/health/live` and
`/health/ready` are outside the contract and need no session.

- Lists use a cursor on creation time and identifier.
- Report view, snapshot, and PDF answers carry an `ETag`.
- PDFs are streamed by the BFF. No link bypasses the permission check.
- Errors have one shape: `{ "error": { "code", "message", "fields" } }`. The
  message is a fixed English sentence chosen by code, never text from an
  exception. The frontend shows its own translation of the code.

### 11.11 Report view

`GET /reports/{id}/view` returns the snapshot worded in the requested
language and narrowed by minimum severity. Each value carries the raw number,
for sorting, and the display text. Each finding carries its code, severity,
sentence, action, and formatted evidence.

### 11.12 Response headers

```text
Content-Security-Policy: default-src 'self'; script-src 'self';
  style-src 'self' [pinned hashes]; font-src 'self'; img-src 'self' data:;
  connect-src 'self'; object-src 'none'; base-uri 'none';
  frame-ancestors 'none'; form-action 'self'
X-Content-Type-Options: nosniff
Referrer-Policy: same-origin
X-Frame-Options: DENY
Cache-Control: no-store        (unless the route sets its own)
```

`'unsafe-inline'` is never used. The pinned hashes are those of the inline
styles that the component library itself injects. Phase 4 records each one
with a test that fails when the library changes it.

## 12. Frontend

### 12.1 Stack

React 19, React Router, TanStack Query, TanStack Table, React Hook Form with
Zod, Radix UI, Tailwind CSS 4, Phosphor icons, react-i18next, Vite, and
TypeScript. Fonts are Inter and Geist Mono, served by the BFF.

API types are generated from `api/openapi.yaml`.

### 12.2 Screens

| Route | Screen | Who |
|---|---|---|
| `/login` | Sign in | Everyone |
| `/reports` | Report list | Signed in |
| `/reports/:id/...` | Report, seven tabs | Signed in |
| `/generate` | New report | Operator |
| `/jobs/:id` | Progress | Signed in |
| `/account` | Password and language | Signed in |
| `/profiles`, `/profiles/new`, `/profiles/:id/edit` | Profiles | Administrator |
| `/users` | Users | Administrator |

Report tabs: overview, findings, namespaces, workloads, autoscaling, nodes,
data quality. The report header holds the PDF downloads: one per language,
each with its status and a retry button.

- Findings: list and detail pane, filters and selection kept in the address,
  free-text search, critical first.
- Tables: sortable and searchable, sticky header and first column, a detail
  sheet per row.
- Data quality is the only place metric names appear.
- The unknown node pool is shown as plain text without the copy control.

### 12.3 New report form

Profile, namespaces from the profile's allowed list, period, time zone,
report profile, and minimum severity. Choosing a profile loads its defaults.

### 12.4 Language

- The switch is in the header of every screen, including sign-in.
- Before sign-in, a browser set to Portuguese shows Portuguese; any other
  shows English. After sign-in the saved choice applies.
- Switching loads the report view again in the new language.

### 12.5 Design

- Restrained and neutral, with light and dark themes.
- Status is always a shape and a label, using the same shapes as the PDF.
- Names and numbers use the monospaced font.
- Contrast meets WCAG AA and a test checks every colour pair.
- Reduced motion is honoured. Live regions announce progress and results.
- No inline style.

## 13. Tests and gates

### 13.1 Rule

No test skips. A test that needs a tool is selected by a build tag and fails
when the tool is missing.

### 13.2 Go

| Group | Needs | Proves |
|---|---|---|
| Unit | Nothing | Each package |
| Prometheus scenarios | Nothing | Probing, `POST` only, tokens, TLS, both strategies |
| Snapshot goldens | Nothing | A scenario gives the same snapshot |
| Data file goldens | Nothing | A snapshot gives the same `report.json`, per language and profile |
| Catalog guards | Nothing | See below |
| Structure guards | Nothing | No Kubernetes or cloud client; imports point one way |
| Platform | PostgreSQL | Accounts, permissions, policy, queue, storage, retention, audit |

The six scenarios of the original are carried over: complete, Grafana with
token, several clusters, raw fallback, partial data, and value edge cases.

Catalog guards:

- every code in the registry has an entry in both languages;
- both entries use the same parameter names;
- no entry is unused;
- no banned term and no em-dash inside a sentence;
- no reader-facing string literal outside `catalog/`.

### 13.3 Template

Run in the Linux image with `typst` 0.15.1, the project fonts, and
`pdftotext`. The harness is Go.

| Group | Proves |
|---|---|
| Toolchain | Pinned version; the four font faces exist and `typst` lists them; the four fixed flags |
| Assertions | Configuration, theme, and components behave as documented |
| Positive | Every example, fixture, and the starter compile with no error and no warning |
| Negative | Bad input fails with the expected message, kept beside its fixture |
| Visual | Pages rendered at 144 PPI match the golden images |
| Monochrome | No page contains a coloured pixel, checked by reading the pixels |
| PDF text | Names survive unchanged; no repeated hyphen at a line break |
| Recipes | Every recipe in the template README compiles, and the list of recipes is pinned |
| Package | The folder installs by copy and compiles through its package name |

- The generic layer brings its 12 negative cases and its 10 snapshots of 13
  pages. Each snapshot states the one behaviour it alone protects.
- The report layer adds golden pages in both languages.
- One list declares the golden pages. Tests and the update command both read
  it.
- A snapshot covers every page of its document, or states which pages it
  leaves out and why. A test compares the list with the page count.
- Golden images are compared by SHA-256. When a page differs, the harness
  writes a difference image.
- A test never updates a golden image. `make update-golden` does, and every
  page is reviewed before it is committed.
- Phase 0 renders one page on `linux/amd64` and on `linux/arm64` and compares
  the bytes. If they are equal, the golden images serve both. If they differ,
  the template tests run on `linux/amd64` only.

### 13.4 Frontend

Unit and component tests, accessibility checks, type check, production build,
a vocabulary test in both languages over visible text and accessible names,
and end-to-end tests in a browser against the Compose stack: main path,
security, concurrency, and PDF recovery.

### 13.5 Commands

```bash
make test            # Go and frontend, no services
make test-postgres   # platform tests; starts PostgreSQL in Compose
make test-template   # template tests in the image
make e2e             # full stack with the fake Prometheus
make lint            # gofmt, go vet, golangci-lint, frontend lint
make generate        # code from openapi.yaml and SQL; fails if output changes
make gate            # all of the above from a freshly built image
```

## 14. Container image and Compose

### 14.1 Image

| Stage | Base | Output |
|---|---|---|
| Frontend | Node | Static files |
| Build | Go 1.27 | One static binary with frontend, template, and fonts embedded |
| Typst | `ghcr.io/typst/typst:0.15.1` | The `typst` binary |
| Runtime | Distroless static | Binary and `typst` |
| Test | Debian slim | Runtime content plus `pdftotext` |

- Runs as user 10001.
- The build compiles a real document as that user before the image is
  accepted.
- The `typst` version is checked at build and at start.

### 14.2 Compose

- `deploy/compose.yaml`: `serve`, `worker`, `postgres`.
- `deploy/compose.integration.yaml`: adds the fake Prometheus, fixture
  profiles, and a way to make a compile fail on purpose.
- `deploy/compose.test.yaml`: PostgreSQL alone, for `make test-postgres`.

`serve` and `worker` run with a read-only root file system, a temporary file
system for scratch, no added privileges, and every capability dropped. Health
checks use `admin probe`.

`serve` applies migrations under a database lock before it starts listening.

## 15. Version, changelog, and working rules

- `VERSION` holds the version. The binary, the image tag, and the frontend
  read it. A test checks that the newest `CHANGELOG.md` heading matches.
- `CHANGELOG.md` follows Keep a Changelog and Semantic Versioning. Each item
  is one sentence about what a user perceives.
- A defect fix is a patch. A change to when a finding appears, to a sentence,
  or to a visual default is a minor version.
- `ROADMAP.md` is untracked. It records every finding with a stable
  identifier, a source, and a phase.
- One branch per phase. Creating the branch is the first step of a phase.
- A phase is merged by fast-forward when its last item closes. Merging and
  raising the version are separate decisions. A tag is created only with a
  new version and only after `make gate` passes on that commit.
- Gate logs are kept under `.cache/release-X.Y.Z/`.
- The main thread plans, judges, and runs the gates. Subagents implement and
  cannot declare a phase finished. An adversarial review precedes each merge.
- Interface design, API design, and catalog wording stay on the main thread.
- Commits follow the conventional format.

## 16. Build order

| Phase | Delivers | Proof |
|---|---|---|
| 0. Foundation | Layout, `VERSION`, `Makefile`, image, guards | `make gate` passes on the empty project |
| 1. Template | Both layers, reading a hand-written `report.json` | PDFs in both languages; goldens reviewed |
| 2. Standalone command | Configuration, collection, analysis, snapshot, catalog, document | A PDF from each scenario, then from a real cluster |
| 3. Platform backend | Database, accounts, policy, profiles, queue, worker, API | A report requested and downloaded through the API |
| 4. Frontend | Every screen in both languages | End-to-end tests pass |
| 5. Release 1.0.0 | Security review, operations guide | Tag after the gate |

Each phase gets its own implementation plan, written when the phase before it
has closed.

## 17. Risks

| Risk | Answer |
|---|---|
| The English wording is new and has no earlier reviewer | The owner reviews the English catalog before phase 2 closes |
| Output differs from the Python tool on the same data | Phase 2 compares both tools on the six scenarios, number by number |
| PDFs enlarge the database | Size limits, seven-day retention, and a sizing note in the operations guide |
| Typst 0.15.1 renders differently from 0.15.0 | Every golden image is produced and reviewed under 0.15.1 |
| A rendering difference between macOS and Linux | Exact-output checks run only in the Linux image |
| A rendering difference between processor architectures | Measured in phase 0; the template tests are pinned to one architecture if needed |
