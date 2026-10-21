# AGENTS.md

This file guides LLMs and people working in this repository.

`capacity-room` generates Kubernetes capacity reports from a
Prometheus-compatible API, using metrics only. The design is in
`docs/superpowers/specs/2026-09-27-k8s-capacity-report-design.md`, and it is
the authority when this file and the code disagree with it.

## Environment

- Development happens on NixOS (x86_64). Go 1.27, Node 26, `typst` 0.15.1,
  `pdftotext`, GNU make, `govulncheck`, and `trivy` come from the devShell of `flake.nix`, whose
  `flake.lock` pins them; every command runs inside `nix develop`. No tool
  is taken from the host profile.
- Anything whose result must be exact runs in the Linux test image, through
  Docker. A native result never closes a gate.
- Docker is the host's system daemon (NixOS `virtualisation.docker`, the
  systemd unit `docker`), reached through the `docker` group. Before a Docker
  step, `systemctl is-active docker` prints `active`, `docker context show`
  prints `default`, and `docker info --format '{{.OperatingSystem}}'` starts
  with `NixOS`; anything else is the wrong engine. An agent never starts the
  daemon; when it is down, the owner starts it.
- Scratch files go under `.cache/`, never under `/tmp` on the host.
- `older/` holds the two original projects as read-only reference. It is
  never tracked and never edited.

## Commands

```bash
make web              # npm ci when the lock file changed, then the frontend build in web/dist
make build            # bin/capacity-room, with the frontend embedded
make test             # Go tests that need no service and no tool, then the frontend tests
make lint             # gofmt, go vet, golangci-lint, then tsc and ESLint over web/
make image            # runtime image and test image
make test-toolchain   # checks typst and pdftotext inside the test image
make test-template    # compiles the template and writes reports in the test image
make test-golden      # compares rendered pages with the golden pages
make test-chart       # helm lint, the chart's Go tests, and kubeconform in the test image
make generate         # writes the Go types and the TypeScript types of api/openapi.yaml
make generate-check   # fails when the committed generated file is stale
make postgres-up      # starts PostgreSQL 18 for the platform tests, through Compose
make postgres-down    # removes it, with its volume
make test-postgres    # postgres-up, the tests tagged postgres (worker and cli with typst), postgres-down
make e2e              # the Compose stack and the Playwright specs in a real browser
make gate             # everything above, from freshly built images
make update-golden    # rewrites the golden pages after an intended change
make update-testdata  # rewrites the golden snapshots and data files
make audit            # govulncheck, npm audit, and trivy over the image; needs the network and Docker
make push             # the runtime image to Docker Hub; outward, main thread, on the owner's word
```

`make audit` is not part of `make gate`, whose result must not change with a
public database. It runs at the review of a release and on its version
commit. trivy leaves out an advisory with no fixed version, and those named
in `deploy/trivy-ignore.yaml`, each with a statement and an expiry.

`make gate` writes one log per step under `.cache/gate/`. It builds the images
without cache and runs the tests without cached results.

Run one test while iterating:

```bash
go test ./internal/guard/ -run TestForbiddenModules -v
cd web && npx vitest run src/session/session.test.tsx
```

## Rules enforced by a guard

Each of these fails `make test` or `make image` when broken. The guards live
in `internal/guard`, except the Typst pin, which `internal/typst` and the
image build enforce.

- **No test skips.** `Skip`, `Skipf`, and `SkipNow` are forbidden in test
  files and in any file that imports `testing`. A test that needs a tool
  carries a build tag and fails when the tool is missing. A skipped gate is
  not a checked gate.
- **Every build tag is declared.** `guard.BuildTags` lists them. A tag that is
  not in the list fails the guard, so that no file hides from the scans. The
  legacy `// +build` form is forbidden; write `//go:build`.
- **Metrics only.** No Kubernetes or cloud client may appear in `go.mod`.
- **Imports point one way.** `guard.Layers` holds the table of layers. A new
  package must be added to that table. Nothing outside `cmd/` imports a
  package under `cmd/`, and the root package imports nothing of this module.
  An import of a package that the table does not hold is reported at the
  import, whatever imports it. An import made only by a test file is outside
  the rule: a test of a low package may use a higher one.
- **Reader text lives in the catalog.** A guard reads every string literal of
  the Go files under `internal/` and `cmd/`, test files left out. It reports
  a literal with a letter outside ASCII. In `internal/document` it also
  reports two words with only white space or percent signs between them, a word
  that starts with a capital letter followed by a lower-case one, and a
  format verb beside a word. An argument of `errors.New`, `fmt.Errorf`, or a
  function whose name ends in `Errorf` is a diagnostic and is left alone, and
  so is the name of a parameter in a `catalog.Params`. A literal that the
  guard takes wrongly for text carries the comment
  `// wording: not reader text, <reason>` at the end of its line.
- **The catalog agrees with itself.** Every key exists in both languages with
  the same parameters, and every code of the registries of
  `internal/discovery` has its entry. Every key is used: a key of a family,
  such as `state.available`, by being a code of its registry, and every other
  key by being named in `internal/document`. An entry uses
  the one template form `{{.Name}}`. No entry holds a banned term, and none
  holds an em-dash unless it is the em-dash alone.
- **The production binary holds no test support.** `internal/promtest`,
  `internal/goldentest`, `cmd/fake-prometheus`, `cmd/typst-switch`, and
  `cmd/update-testdata` are not among its dependencies, and `time/tzdata` is.
- **The Containerfiles share one Go base.** Every `FROM golang:` line of
  `deploy/Containerfile` and `deploy/worker-e2e.Containerfile` is equal.
- **Typst is pinned at exactly 0.15.1.** The toolchain probe refuses any other
  version, and the image build asserts it.
- **The released migrations are never edited.** A test pins the SHA-256 of
  each migration released with 1.0.0; a schema change adds a new file that
  carries the stored rows forward.
- **The operations guide equals the code.** Two tests compare the tables of
  `docs/operations.md`: every key of `platform.toml` both ways, and every
  failure code of a report or a PDF in the contract both ways.
- **The old name is gone.** No tracked file outside the history holds a name of
  1.x. `internal/guard/oldname.go` holds the patterns and the history: the
  plans, the earlier specs, `docs/decisions/`, `docs/reference/`,
  `testdata/python/`, `testdata/prometheus/`, and the changelog of 1.x.
- **One version.** `VERSION` holds it, and the newest changelog heading must
  equal it. The only level-two headings the changelog may hold are
  `## [Unreleased]` and `## [X.Y.Z] - YYYY-MM-DD`, and it holds no heading
  written as text over a line of dashes.

## Rules kept by review

No guard checks these. The reviewer of each task does.

- **From 1.0.0 on, an upgrade never loses or breaks stored data silently**
  (spec 15.1). The public surface is the API under `/api/v1` as
  `api/openapi.yaml` describes it, the commands with their flags,
  environment variables, and exit codes, the keys of `report.toml` and
  `platform.toml`, and the snapshot file (`schema_version` 2.0). Breaking it
  is a major version; adding to it is a minor one. Stored data is converted
  forward by a migration, and the code keeps one path, never a reader of an
  earlier shape; data that cannot be carried forward is dropped on purpose,
  named under `Removed`, with a major version.
- **`docs/operations.md` is the operator's full document, and the chart's
  README (`deploy/helm/capacity-room/README.md`) is its quick start, links to
  it, and changes with it in the same commit. `docs/security.md` records the threat model, the controls, and each
  review.** A change to a setting, a failure code, a command, or the Compose
  file updates the guide in the same commit; a new control or a new
  attacker updates the security document.

- **Diagnostics are in English.** This covers errors, logs, and help text.
- **Technical terms are English in both languages.** A Portuguese sentence
  writes request, limit, workload, node, node pool, label, replica,
  autoscaling, recording rule, raw metrics, and throttling as English words,
  masculine, with an English plural. Only a common noun is Portuguese. The
  guard bans the translations; the reviewer judges the border cases against
  the table of spec 9.3.
- **Nothing else declares a version** without a test that compares it with
  `VERSION`.
- **The sentences of the API error envelope are diagnostics.** They are
  fixed English sentences chosen by code, held in one table in
  `internal/platform/api/errors.go`, for API clients; a reader in a browser
  sees the frontend's translation of the code, never this table. They are
  not catalog entries.
- **The wording guard reads text, not meaning.** It lets through one word
  alone, words parted by digits or a slash, and a sentence joined from parts.
  The reviewer of a task reads the string literals of `internal/document`,
  and the golden data files show every text that reaches a page.
- **No comment speaks of the process.** A comment says what the code does. It
  names no task, step, round, ruling, brief, or role, and no path under
  `.cache/`. It may name a section of the design and the original program.
- **A ratio is worked out sample by sample.** Consumption over request, and
  over limit, is computed at each timestamp and then reduced. It is never
  the ratio of two averages, which gives other numbers than the original.
- **The numbers equal the original exactly.** Two numbers are equal when
  they are the same `float64`. A test of the comparison has no tolerance,
  and an expected number in a test is a literal, never the expression of
  the code written again.
- **The executive profile raises the minimum severity to at least attention
  for the whole document, the appendix included; the detailed profile raises
  it for the executive summary alone and never for the appendix.**

Build tags in use:

| Tag | Needs | Runs in |
|---|---|---|
| `toolchain` | `typst`, `pdftotext` | `make test-toolchain` |
| `template` | `typst`, `pdftotext` | `make test-template`, which also runs `internal/cli` |
| `golden` | `typst`, with `template` | `make test-golden` |
| `postgres` | PostgreSQL, through `CAPACITYROOM_TEST_DATABASE_DSN` | `make test-postgres`, over `internal/platform/...` but the worker |
| `postgres` with `template` | PostgreSQL and `typst` | `make test-postgres`, over `internal/platform/worker` and `internal/cli`, so each of their postgres tests runs once |
| `helm` | `helm`, `kubeconform` | `make test-chart` |

With the native tools installed, a subagent runs the tagged tests itself:

```bash
go test -count=1 -tags=template ./template/ ./internal/typst/ ./internal/cli/
go test -count=1 -tags='template golden' ./template/ -run 'Golden|Snapshots'
```

## The layers

`guard.Layers` holds this table. A package imports only packages of a lower
rank, or of its own layer.

| Rank | Packages |
|---|---|
| 0 | `internal/guard`, `internal/arith`, `internal/typst`, `internal/redact`, `internal/promtest`, `template`, `web` |
| 10 | `template/golden` |
| 20 | `internal/config` |
| 30 | `internal/prom` |
| 40 | `internal/discovery` |
| 50 | `internal/resolve` |
| 55 | `internal/collect` |
| 60 | `internal/analysis` |
| 70 | `internal/snapshot` |
| 75 | `internal/pipeline` |
| 80 | `internal/catalog` |
| 90 | `internal/document` |
| 95 | `internal/goldentest` |
| 100 | `internal/render` |
| 105 | `internal/platform/netpolicy` |
| 110 | `internal/platform/settings` |
| 120 | `internal/platform/store` |
| 125 | `internal/platform/storetest` |
| 128 | `internal/platform/health` |
| 130 | `internal/platform/audit` |
| 140 | `internal/platform/auth` |
| 145 | `internal/platform/profiles` |
| 146 | `internal/platform/reports` |
| 150 | `internal/platform/api` |
| 160 | `internal/platform/worker` |
| 200 | `internal/cli` |
| 210 | `internal/chart` |

The snapshot is the only handover. Everything after `snapshot.Build` reads
the snapshot and nothing else. The platform packages sit above the core and
below the command; the core never imports `internal/platform/`.

## The command

- **One redactor for the whole run.** `cli.Run` makes it on its first line
  and wraps both streams with it. Nothing below `Run` sees a raw stream. A
  secret is added to it as soon as it is known: the value of `--token` before
  the flags are read, the token of the configuration as soon as it is loaded.
  No function passes a list of secrets. A secret is matched as it is and in
  its `%q`-escaped and JSON-escaped forms.
- **One message, one write.** The redactor reads each write as one text, so a
  line is built whole and written once.
- **A log value is redacted before it is formatted.** Every logger of
  `serve` and `worker`, River's included, comes from `newLogger`, whose
  handler is `redact.Handler` over the text handler: at each record it
  redacts the message and every attribute, those bound by `With` and inside
  groups included, with the secrets known when the line is written, since
  the text handler escapes a quote or a backslash and the stream redactor
  matches literal text. A test reads the Go files of `internal/cli` and
  `internal/platform` and fails on any other handler, default logger, or
  `log` call, the two `io.Discard` handlers of the API excepted. The
  `ErrorLog` of every HTTP server writes through that logger too.
- **The assignment pattern keeps the structure around a secret.** A `token`,
  `api_key`, or `access_token` value in quotes is replaced inside its quotes,
  which are kept; a bare value runs to white space, `,`, `;`, a quote, `}`,
  `]`, or `)`, and an escaped quote belongs to it; `[redacted]` followed by
  value characters is redacted whole. A secret holding a stop character is
  cut there by the pattern; only a literal secret of the redactor removes
  the rest.
- **No message repeats an argument that was not understood.** It may be a
  secret typed at the wrong place. A path that the user gave is repeated on
  purpose, and so is a value read from a file that the user named. A flag
  error names only a flag the program defines, through one flag-error
  function on the root command, never the text the user typed.
- **A password typed on the host enters the redactor once it passes the
  password policy,** and the server secret as soon as `serve` reads it. A
  value shorter than the policy cannot be told from ordinary text.
- **The program has the commands the design names, and no other.** The
  completion command that the command-line library adds is turned off. A
  test names the commands it expects and runs each with a stray argument.
- **Every failure prints one line**, `error: <message>`, on standard error,
  and nothing on standard output.
- **Output files are written beside their target and renamed.** They follow
  the umask, which is never read or set. PDF bytes are written only when
  they start as a PDF does.
- **A token is at least 8 characters long.** A shorter one cannot be told
  from ordinary text, and is refused before it reaches the redactor.
- **The snapshot file has one reading.** The reader walks the file against
  the contract, letter for letter, and fills the snapshot from what it
  walked. It names a wrong schema version before anything else. The schema
  is `2.0`, with no `report.period`: a snapshot `1.0` is named and refused,
  with no reader of it. Its period ends at or before its emission.
- **The probes keep the order of the original**, short cuts included, except
  that the ninth probe asks `count(kube_configmap_info)` in place of the
  original's probe of the cluster autoscaler (decision 0002). The fake
  server answers a query with the first recorded answer whose text is
  part of the query, so another order consumes the wrong answers.
- **The collectors keep the order of the original too:** the namespaces,
  then the workloads, then the HPAs and their coverage, then the nodes, then
  the cluster autoscaling, and inside each the queries in its order. The text of
  every query equals the original letter for letter, except the node
  autoscaling probe and collector, which read kube-state-metrics (decision
  0002); the comparison with the original leaves them out.
- **`--check` is the preflight and nothing else.** It sends no query of a
  collector, and the requests it sends are the first requests of a run.
- **A run with nothing to report stops with exit code 5.** When the five
  collectors return no row in a state other than unavailable and the status
  of the cluster autoscaling is unknown, the run stops after the collectors
  and before the clock is read the second time.
- **The period is judged before the first request.** A period whose start is
  not before its end stops the run with exit code 2, and so does a range
  that ends after the run starts (`collect.ErrRangeAfterRun`). A range is
  `analysis.start` and `analysis.end`, two wall times `YYYY-MM-DD HH:MM` read
  in the zone by `config.ParseWallTime`, the one parser of the program; a
  time the zone skips or repeats is refused. A length and a range exclude
  each other within a source, and a later source that sets one clears the
  other.
- **The clock is read twice in a run that collects:** for the emission time,
  which is also the start of the collection, and for its end.

## The platform

`serve` is the web platform's backend, `worker` runs its report jobs, and
`admin` holds its operator commands. All three read one TOML deployment file, `--config` (default `platform.toml`),
and the database DSN from `CAPACITYROOM_DATABASE_DSN` alone. The DSN enters the
redactor the moment the settings are loaded, before the pool opens.

- **The deployment file refuses what it does not know,** as `report.toml`
  does. Its keys today: `session_ttl_seconds`, `session_idle_seconds`,
  `server_secret_path` (a file of at least 32 random bytes, which keys the
  sign-in throttle), `trusted_proxies` (CIDRs whose `X-Forwarded-For`
  is believed; empty by default), `allowed_endpoint_hostnames` and
  `allowed_endpoint_cidrs` (the two allowlists of the outbound policy;
  empty by default, so no metric source is reachable until one is listed;
  a CIDR that covers a metadata address with more than that one address is
  refused), and `credential_dir` and `ca_bundle_dir` (the folders the
  reference files are read from, absolute, defaults `/run/secrets/metrics`
  and `/run/config/ca`, not read at load, and refused when they hold the
  deployment file or the server secret), and the ten keys of the worker
  and the reports: `worker_slots`, `worker_drain_seconds`,
  `collect_timeout_seconds`, `render_timeout_seconds`, `job_max_attempts`,
  `report_retention_days`, `expired_report_days`, `admission_limit`,
  `admission_profile_limit` (not above `admission_limit`; when omitted, 5
  or `admission_limit` if smaller), and
  `admission_retry_seconds`, with no environment override.
- **The schema is applied under a lock.** The migrations are SQL files
  embedded under `internal/platform/store/migrations/`, applied by goose
  under the PostgreSQL session lock `72427301`, by `serve` before it listens,
  by `worker` before it starts, and by `admin migrate`. Every timestamp of
  this project's tables is passed from Go; no statement of this project
  calls `now()`. River's own tables, created by `00003_river.sql`, use
  `now()` in River's SQL; that is the one exception. Every foreign key is `ON DELETE RESTRICT`; deletion is
  deactivation. The audit table refuses an update, a delete, and a truncate
  by triggers.
- **A change of role or active flag takes the table lock first,** so the
  last-administrator rule reads a count that cannot race; a password or
  language change writes its column alone, and a password write is a
  compare-and-set on the hash it verified.
- **A test gets a fresh database.** `storetest.New(t)` creates
  `capacityroom_test_<16 hex>` on the server of `CAPACITYROOM_TEST_DATABASE_DSN`, migrates
  it, and drops it when the test ends; `NewEmpty(t)` skips the migration. A
  tagged test fails when the variable is unset.
- **The contract is `api/openapi.yaml`.** `make generate` writes the Go types
  of its schemas; the handlers are written by hand on `net/http` from one
  route table, and a test compares the table with the file both ways. A
  route enters the file with the task that adds it.
- **The checks of a route run in one order:** session (401), role (403),
  CSRF (403), body or query (400), then the service, which judges the id.
  The role is read from the database on every request. A body is at most
  64 KiB, with no unknown, repeated, or wrongly typed key.
  `GET /auth/session` first refuses a request whose `Sec-Fetch-Site` is
  present and neither `same-origin` nor `none`, with 403 `CSRF_REJECTED`.
- **The report routes are those of spec 11.10:** `POST /reports`,
  `GET /reports` (a keyset page on creation time and id), `GET /reports/{id}`,
  its `/view?lang=`, `/snapshot`, and `/pdf?lang=`, `POST
  /reports/{id}/pdfs`, `DELETE /reports/{id}`, and `POST
  /reports/delete` (up to 200 ids, one outcome each, in the order sent),
  and `GET /home`, a read for every signed-in user that answers, for each
  active profile, its newest report and its newest collected report that has
  not expired.
  Every signed-in user reads every report that is not deleted; an operator
  or an administrator requests reports and PDFs; an administrator deletes
  any report, an operator its own.
- **A request names a length or a range, never both.** `POST /reports`
  takes `period`, or `start` and `end` as wall times in the resolved zone.
  The syntax is judged first (`invalid`, `missing`, `conflict`); then, once
  the zone is resolved, the range against the zone and the request's clock
  (`invalid` for a time the zone skips or repeats, `order`, `future`), all
  400; then the permits: a profile with `permitted_periods` refuses a range
  with 422 `not_permitted` on `start`. A stored range reaches the window as
  given.
- **A request names its language, and the worker makes that PDF alone.**
  `POST /reports` requires `language`, stored with the selection, so a key
  reused with another language is a conflict. The collect handler queues
  one render, for that language. A stored request without a valid language
  fails its collection with `INTERNAL_FAILURE`, and `GET /reports/{id}`
  answers 500 for it; there is no fallback, and `GET /reports` lists such a
  report, or one whose stored request cannot be decoded, with a null
  selection. `POST /reports/{id}/pdfs`
  makes the PDF of a language under the report's row lock: no row inserts a
  pending one with a render job, a failed row is compiled again, a pending
  row changes nothing, and a ready row answers `PDF_ALREADY_READY`; each
  path that queues sets the phase to `rendering` and writes
  `report.pdf_request`.
- **A report answers 404 once deleted or purged, and 410 from its expiry
  until its purge,** except to a delete: an expired report is deleted like
  any other, as spec 12.2 offers it. Expired means `expires_at` at or before now or
  `expired_at` set, judged with one reading of the clock per request; the
  maintenance job's clock may run ahead of the API's.
- **Authorization runs before any conditional answer.** The view, the
  snapshot, and the PDF are read whole in one statement after the service
  allowed the reader, carry a strong `ETag` (the stored SHA-256, or that of
  the view's bytes) and `Cache-Control: private, no-cache`, and are answered
  through `http.ServeContent`, whose 304, 206, 412, and 416 the contract
  names. No 304, 409, or 410 reaches a caller who could not read the report.
  At most four downloads read and write a body at once; a waiting request
  holds no body, and the write of an answer has two minutes.
- **A body is read through the download pool.** `store.OpenDownloads` opens
  a pool of `store.DownloadConns` connections, four, that keeps no idle
  connection: every released one is destroyed, so the memory of a large
  `bytea` leaves with it. The snapshot and the PDF are read through it, and
  the view through the snapshot; every other read stays on the main pool.
  The download slots of the API equal `store.DownloadConns`. `serve` opens
  the pool after the main one and closes it before; `worker` never opens it.
- **The view is a second product of `internal/document`.** `BuildView`
  words a snapshot for the web from the helpers the PDF uses, so every text
  is the PDF's and the rows kept are the PDF's (the action plan's
  recommendations through one helper); a closed set travels as its code,
  and the mark the PDF prints beside a text travels as its status. A
  node's request share carries the severity of the node's request finding
  for that resource, a status the PDF states as that finding rather than
  beside the share. Its `verdict` holds the decisions of the PDF's verdict
  page, made once by `decideVerdict`, as indices into the view's own lists
  (`findings`, `quality.warnings`), and each recommendation names its
  finding the same way.
  The contract describes it as `ReportView`, excluded from the Go
  generation, and `TestViewMatchesTheContract` walks the view of every
  golden snapshot against that schema both ways.
- **A delete marks.** `deleted_at` is set where it is null, with
  `report.delete` in the same transaction; from the mark the report answers
  404 everywhere and a replay of its key answers `REPORT_NOT_FOUND` until
  the sweep, after which the key is free, as after the purge. A bulk
  delete marks in id order, so two bulk deletes cannot deadlock. A replay
  reads its key and its report in one statement.
- **`serve`'s stop is bounded.** It waits 30 seconds for the requests in
  flight, then closes the connections still serving a request, counted by a
  `ConnState` hook, logs one warning with their number, and exits 0; a second
  signal ends the process at once; it fails only when the listener cannot be
  closed.
- **`serve`, `worker`, and `admin` exit with 0, 1, or 2:** 1 for a failure
  at run time (the database, the listener, Typst for `worker`, jobs that did
  not stop after the drain, a refusal of `bootstrap-admin`), 2 for the
  arguments or the settings file.
- **`admin bootstrap-admin` runs only with `CAPACITYROOM_BOOTSTRAP_MODE=1`,** reads
  the password from standard input alone, and refuses when any user exists.
- **`admin reset-password` runs only with `CAPACITYROOM_BOOTSTRAP_MODE=1`,** reads
  the new password from standard input alone, refuses an unknown or
  deactivated user, revokes every session of the user, writes
  `user.password_reset` with no actor, and changes neither the role nor the
  active flag.
- **The Compose network has a fixed subnet,** `172.31.213.0/24`, so a TLS
  proxy on the host reaches `serve` from `172.31.213.1`, which
  `trusted_proxies` names; the e2e project takes `172.31.214.0/24`. The
  sign-in throttle keys a client by its canonical address, and an IPv6
  client by its /64.
- **A reference file is opened once, inside its folder** (`os.Root`,
  non-blocking), and checked on the open file. A render attempt claims its
  PDF row with its job and attempt, in a transaction of its own, as a
  collect attempt claims its report.
- **Every request to a metric source passes the outbound policy** of
  design 11.5, in `internal/platform/netpolicy`: only `http` and `https`,
  with no user info, query, fragment, control character, or dot segment; the
  unspecified, loopback, link-local, multicast, reserved, and IPv4-embedding
  IPv6 classes and the two metadata addresses are refused, and only an
  allowlist network of exactly one address lifts a refusal, for that
  address alone; every resolved address must be allowed by host name or by
  range, and one bad answer refuses the whole set; the name is resolved
  again before each request, the connection goes to the literal address,
  and the peer is compared after connecting; any redirect is refused; proxy
  variables are ignored. The fake Prometheus of the tests is admitted by
  the one-address entry `127.0.0.1/32`. The transport keeps no connection
  between requests, and the built transport of `internal/prom` is never
  used by the platform. An error of the policy carries a code and never a
  URL, a host, or an address.
- **A profile names its credential and its CA bundle by file name.** A name
  matches `^[a-z0-9](?:[a-z0-9._-]{0,61}[a-z0-9])?$` and holds no `..`; the
  file is read under its folder after symbolic links are followed, as a
  regular file of at most 1 MiB; a resolution error carries a code and
  never a path, a name, or content. A file is resolved only by the profile
  tests (saved and draft) and by the worker at execution; never at create,
  revise, or import. A credential is UTF-8 with one trailing line break
  removed and at least 8 characters, as on the command line.
- **A draft profile is tested before it is saved.** `POST
  /admin/profiles/test` takes a document (and the id, when it has one),
  checks it as a create does (422 for the document or a statically refused
  endpoint), and runs the saved test's preflight through the outbound
  policy; it reads and writes no profile row. It writes one audit row
  `profile.test` with `{"code","draft":true}`, targeting the id when one is
  given. `ProfileTestCode` is a contract enum held equal to
  `profiles.TestCodes` in order.
- **Every revision of a profile is immutable** (a trigger refuses an update,
  a delete, and a truncate), and `profiles.current_revision_id` pins the
  current one, with a deferred constraint that it names a revision of the
  same profile. A revision number is computed inside the insert. A revise
  names `base_revision_id` and answers `PROFILE_REVISION_STALE` when it is
  not the current revision; a revise that changes nothing writes nothing;
  the fingerprint chains the parent revision, so A, B, A is three
  revisions. A delete clears the pointer and keeps every revision, and the
  id may be created again, continuing the count. The endpoint is stored in
  the canonical form of the policy, checked for every candidate root and
  its `/query` and `/query_range` before anything is written.
- **`admin import-profiles --file <path>`** reads `[[profiles]]` tables,
  refuses an unknown key (an inline credential key among them) and a
  repeated id, validates every document and every endpoint before any
  write, then creates or revises in one transaction with no actor, and
  prints `created`, `revised`, `recreated`, or `unchanged` per profile; a
  profile deleted through the API is recreated, continuing its revision
  count, and audited `profile.recreate`. Profiles the file does not name are
  left alone. A refused file is exit 2 before the pool opens.

### The worker and the reports

- **River is the queue, and its schema is a goose migration.**
  `00003_river.sql` holds River's SQL of its versions 2 to 8 as the library
  gives it, and a test compares the file with the library version by
  version, so an upgrade of River fails until a new migration holds the new
  versions; a River version that adds an enum value and one that uses it go
  in two files. No `riverdatabasesql`, no `rivertest`.
- **A report is `queued`, `running`, `collected`, or `failed`,** with a
  phase bound to it by a check: `rendering` means a PDF row is `pending`,
  `complete` means none is. A request takes the transaction advisory lock
  `72427302` before its idempotency row and its admission counts, and
  inserts the River job, the report carrying the job's id, the idempotency
  row, and the audit row in one transaction.
- **The attempt owns a running report.** A collect attempt claims the report
  with its River attempt and never lowers it; every later write of the
  report names that attempt and the status `running`, and writes nothing
  when either differs. A PDF row is written only from `pending`. Every
  handler writes its outcome and completes its River job in one
  transaction. A stale attempt reaches River as `<kind>: STALE_ATTEMPT`.
- **No stored text carries a failure's words.** A report or a PDF row
  stores a code; a River job's error is `<kind>: <CODE>`; the cause goes to
  the log through the run's redactor. A credential the worker reads enters
  the redactor before it is used.
- **Only an unreachable source, a name that did not resolve, and an internal
  failure are retried,** up to `job_max_attempts`; a deadline and every other
  code are stored at once.
- **A success write keeps its budget.** The transaction that stores a
  collection or a PDF runs free of the job's cancel, until the later of the
  job's deadline and now plus ten seconds; any error of it that is not a
  stale attempt is `INTERNAL_FAILURE`, never a code of the source or a
  deadline. A failure write keeps its ten seconds.
- **A deleted report is never claimed and never rendered.** A queued one
  completes its job writing nothing; a running one finishes its attempt;
  reconcile and expiry leave deleted reports alone.
- **A snapshot is stored as the bytes `snapshot.Encode` wrote,** in `bytea`,
  at most 32 MiB, with its SHA-256 and size, never as `jsonb`; a PDF at most
  64 MiB. A render recomputes the snapshot's SHA-256 before decoding it.
- **The maintenance job runs every minute** on the elected River leader, in
  six steps of their own: reconcile (a report or PDF row whose River job is
  gone or final fails `JOB_ATTEMPTS_EXHAUSTED` or `PDF_ATTEMPTS_EXHAUSTED`),
  sweep (a deleted report that is not running, or is running with its
  collect job gone or final, goes with its rows, `report.purge
  {"cause":"deleted"}`), expire (after `report_retention_days` the snapshot
  and PDF rows go and the report is kept as expired), purge
  (`expired_report_days` after it was marked expired the report goes,
  `report.purge {"cause":"expired"}`), the sign-in throttle rows whose
  block is over, and the sessions ended longer than a session lifetime ago.
- **The `worker` stop is bounded:** it takes no new job, waits
  `worker_drain_seconds`, cancels the rest (River returns them with the
  attempt restored), waits once more, and then exits 1 with `jobs did not
  stop`; readiness is false from the moment the stop begins. A second signal
  ends the process at once.

### Golden snapshots and data files

- `testdata/prometheus-added/` holds, for each scenario, the answers to the
  queries the original never sent, read before the recorded answers by
  `promtest.LoadScenario`; its README pins each file by SHA-256 and says what
  each answer exercises.
- `testdata/snapshot/` holds one snapshot for each scenario, and
  `testdata/queries/` the queries the program sent for it, in order. The
  recorded answers match a query by a part of its text, so only the list of
  queries can show that a probe asks what it should.
  `template/tests/capacity/fixtures/command/` holds the data files made from
  them, which are also fixtures of the template and sources of golden pages.
- `promtest.RangeScenarios` holds `direct-complete-range`: the fixture of
  `direct-complete` over a range of 24 hours that ended the day before the
  fixed emission. It has a snapshot and a list of queries, and no data file
  and no golden page. It stays outside `promtest.Scenarios`, so every
  comparison with the original walks the eight it always walked.
- A test never writes one. `make update-testdata` does. It needs no image.
- After `make update-testdata`, read every file that changed, whole, before
  committing. A value that cannot be explained is a defect.
- The files of `testdata/prometheus/` are the recorded answers of the
  original, byte for byte. They are never edited.
- `testdata/prometheus-own/` holds the scenarios that this project wrote,
  for values that no recorded answer holds. It holds two scenarios,
  `edge-values` and `edge-scaling`. Its README says what each
  answer exercises. A file there is not edited once its recorded numbers
  exist.
- `testdata/python/` holds what the original computes and asks for each of
  the eight scenarios: its numbers for every section the program computes,
  and its queries in order. The recording script lives under
  `.cache/phase-2/part-4/python/`. It also records the recommendations, the
  summary counts, the coverage of every row, and the executive rows, with a
  stand-in for pydantic that its README describes. A test compares this
  project with them on every run. They are never edited; they are replaced
  only by recording the original again, and its README says how.
- `template/tests/capacity/fixtures/command/` holds four executive data
  files, `<scenario>.executive.<lang>.json` for `direct-complete` and
  `edge-scaling`, written by `make update-testdata` like the others. The
  snapshots `command-executive-pt` and `command-executive-en` take pages
  from them.
- `web/tests/fixtures/view/<scenario>.<lang>.json` holds the golden views
  for `edge-scaling` and `direct-complete`, in both languages, written by
  `make update-testdata`. `TestGoldenViewFiles` compares them, a guard
  refuses a stray file in the folder, and the frontend's tests read them.
- A test that walks the scenarios or reads a list of queries counts what it
  means to count. A filter that matched one thing when it was written
  matches more when the program asks more.

## The interface

`web/` holds the frontend: React and TypeScript, built by Vite into
`web/dist`, embedded in the binary by the Go package `web`, and served by
`serve`. Spec 12 is its design.

- **Every text the reader sees comes from the locale files,**
  `web/src/locales/en.json` and `pt-BR.json`, through `t()`; so does every
  accessible name, a tooltip, and the page title. The two files hold the same
  keys with the same parameters, every key is used, every error code of the
  contract has its `error.<CODE>`, and a family of keys, such as `role.*`,
  is declared in `web/src/i18n/families.ts`. A Zod check carries a locale
  key as its message, never Zod's own text. ESLint refuses JSX text that
  holds a letter; it does not see a string in braces, which review catches.
- **The banned terms are one file,** `internal/catalog/banned-terms.json`,
  read by the Go guard and by the frontend's vocabulary test with the same
  matching. The wording rule of the report holds: technical terms are
  English in both languages.
- **A colour is a token pair.** `web/src/theme/theme.css` defines the tokens
  of both themes, and `web/src/theme/pairs.ts` lists every foreground and
  background drawn together. The contrast test requires 4.5:1 for text, and
  3:1 for a large heading and for an input border or a focus ring against
  its surface, in both themes. A new pair on the screen enters the list.
- **No inline style.** No `style` prop, no `dangerouslySetInnerHTML`, and no
  `<style>` element added at run time; a test opens the menu, the
  tooltip, the popover, the alert dialog, and the dialog, the five Radix
  layers in use,
  and finds none. A new layer joins
  that test. The CSP stays `script-src 'self'` and `style-src 'self'`
  until a style hash enters with its test. Zod runs with its JIT off,
  because its probe of `new Function` breaks the CSP.
- **Status is a shape, then a label, then a colour,** with the five shapes of
  the PDF, drawn by `StatusMark`.
- **Names and numbers use the monospaced font,** and motion appears only
  under `motion-safe:`.
- **The look is recorded in `DESIGN.md`,** derived from the built code; a
  change of a token, a face, or a component rule updates it in the same
  commit. The product is CapacityRoom. The command, the Go module, the images, and
  the deploy names are `capacity-room`; environment variables start with
  `CAPACITYROOM_`; a lowercase identifier starts with `capacityroom_` or
  `capacityroom-`.
- **A tint follows a status the view gives.** The tint of a peak cell (by
  the worse of the statuses of its peak and its share) and the colour of a
  proof bar (the severity of its finding; for the nodes of the cluster, the
  hazard the view gives the gauge) follow a status the view gives, never a
  threshold of the web app. A proof bar draws only the view's numbers
  against the reference its evidence names: 100 for a percent, the maximum
  of a count, a range from its lowest value. The State column of the nodes
  takes the status the view gives the node's state: the severity of its
  readiness findings, or unavailable when readiness is unknown. A status
  colour sits on its shape and on a proof bar, never on words. Filled
  actions use the `action` tokens; `accent` is for links and focus. One
  shadow, `shadow-float`, lifts a floating layer (menu, popover, dialog);
  nothing else casts one, and a modal dims the page through the `scrim`
  token.
- **An icon button keeps its name.** `IconButton` puts the name in
  `aria-label` and shows it in a tooltip on hover and on focus. Its tooltip is
  drawn inside the landmark that holds the button, or in the body when no
  landmark does, and the app has one tooltip provider.
- **The static handler answers what no route answers,** for `GET` and `HEAD`
  only, and never under `/api` or `/health/`: the built file at the cleaned
  path, a plain 404 for a missing file under `/assets/`, and `index.html`
  for any other path. A file under `/assets/` is cached for a year as
  immutable; every other file is `no-cache`. The content type comes from the
  package's own table, and a test refuses a built file whose extension it
  lacks.
- **The session follows spec 12.6.** Every state-changing request carries
  the CSRF token of the current session. A `CSRF_REJECTED` loads the session
  again and repeats the request once. A `401` whose code is
  `INVALID_CREDENTIALS` is a form error; every other `401` clears the query
  cache and opens sign-in with `ended=1` and the current path in `next`,
  except on sign-out, where it means the reader is already out. `next` is
  followed only when it is a path that starts with a single `/`.
- **Every request of the API client has a time bound.** It fails with
  `error.timeout` when no answer has come 30 seconds after it was sent; the
  two profile tests and the report view allow 150 seconds, from one table in
  `web/src/api/client.ts`. A timed-out request counts as unanswered, so a
  resend replays.
- **The language and the theme survive refused storage.** Before sign-in
  the language is the stored choice or the browser's; after it, the
  account's, and the stored choice is removed. A refusal of `localStorage`
  keeps a choice for the page's lifetime and throws nothing.
- **No skipped or focused test.** ESLint refuses `skip`, `only`, `todo`, and
  `fixme` in Vitest and in Playwright, as the Go guard refuses a skip.
- **Every npm dependency has an exact version,** with no `^` or `~`, and
  `web/package-lock.json` is committed. `npm ci` installs it with no
  `--force` and no `--legacy-peer-deps`.
- **The frontend's types come from the contract.** `make generate` writes
  `web/src/api/schema.gen.ts` from `api/openapi.yaml`, and
  `make generate-check` fails when the committed file is stale.
- **The built version equals `VERSION`.** Vite reads `../VERSION` into the
  meta `capacityroom-version` of `index.html`, and a Go test compares the embedded
  file with it.
- **`make e2e` runs the real stack.** `deploy/compose.yaml` holds
  `postgres`, `serve`, and `worker`, and requires `CAPACITYROOM_VERSION`,
  `CAPACITYROOM_CONFIG_DIR`, and `CAPACITYROOM_POSTGRES_PASSWORD`, with no default.
  `deploy/compose.integration.yaml` adds the one-shot `migrate` and
  `bootstrap` services and the Playwright service, which shares the network
  of `serve`, and the fake Prometheus, built by `make e2e` from the
  `fake-prometheus` stage of `deploy/Containerfile` and never by
  `make image`, with the one-shot `profiles` service that imports
  `deploy/platform/profiles.toml`. Each fake serves its scenario once, so
  each spec that collects has a fake of its own and runs one collection,
  and Playwright never retries: `fake-prometheus` serves the reports spec,
  and `fake-recovery`, which waits 150 ms before each answer so a
  collection lasts about 10 s, serves the recovery spec through the profile
  `e2e-recovery`, whose credential is the file
  `deploy/platform/credentials/e2e-recovery`. A fake is healthy once it
  answers `/-/ready`, which spends no recorded answer, and `serve` and
  `worker` wait for both. A spec that needs a report in flight and no data
  uses a profile at `fake-prometheus:9091`, where nothing listens, as the
  concurrency spec and the administration spec's draft tests do. The target
  stops the fakes and fails unless each log holds `remaining: 8`, the HPA
  answer of `direct-complete` that no run asks and the seven recorded answers
  of the cluster autoscaler, pinned by
  `TestDirectCompleteLeavesTheRecordedAnswerUnused`; the fakes read
  `testdata/prometheus-added/direct-complete.json` through `--added`.
- **The test worker's typst is a switch.** `make e2e` builds
  `deploy/worker-e2e.Containerfile` on the runtime image as
  `capacity-room-worker-e2e`, whose worker runs `cmd/typst-switch`
  through `CAPACITYROOM_TYPST`: it passes every call to typst, unless
  `/switch/fail` exists, which makes every compile fail. The Playwright
  service writes the volume `switch`, and the worker reads it.
  `make gate` builds the fakes and the test worker with no cache.
- **No secret reaches the stack's logs.** Every password under `web/e2e/`
  is declared through `secret()`, which records it in `/results/secrets.txt`.
  After the stack stops, the target fails when no secret was recorded, when
  the list of secrets cannot be built, or when a recorded secret, the
  administrator's password, the credential, or the database password
  appears in the logs of `serve`, `worker`, or the one-shot services. It
  never prints a secret.
- **The specs are independent of order.** Playwright runs them in name
  order: admin, concurrency, recovery, reports, security, sign-in. Each
  names its users and profiles with its own prefix (`e2e-s-`, `e2e-c-`,
  `e2e-r-`), finds its reports by id, and undoes in a `finally` what it
  changes for everyone: the recovery spec turns the switch off, and the
  concurrency spec leaves `e2e-admin` the one active administrator. Every
  spec fails on any CSP violation and on any axe finding, except the
  security check that injects one. The fixtures of `deploy/platform/` serve
  this stack alone.
- **The report screen is `/reports/:id/:tab`,** with seven tabs: overview,
  findings, namespaces, workloads, autoscaling, nodes, and quality. Its frame
  reads `GET /reports/{id}` and `GET /reports/{id}/view?lang=`. The view has
  its own query key and stays on screen across a language switch for the same
  report only. A later failed fetch of either never replaces a screen that is
  held, except a 404 or a 410 of either, or a 409 of the view, which opens
  the progress page.
- **The overview opens on its answer, built from the view alone.** First the
  answer strip: the severity totals and any data-quality warnings. Then the
  verdict band, which comes from `view.verdict` and keeps no rule of the web
  app: whether the cluster can grow (its nodes against the autoscaler's
  maximum as a proof bar, the worst cluster finding, and the node-evidence
  warning when the report has it) and the pressure of each scope, the scopes
  worst first (its worst severity, the first finding of that severity,
  linked to the findings tab). Then the action plan, whose rows open the
  finding that `recommendation.finding` names. The scope of a row names each
  scope by its full name, as the PDF does, and its kind too only when the
  action has three scopes or fewer; above three, the summary alone. The
  overview shows no number the view does not hold; a count it shows is a
  count of rows, findings, or warnings.
- **Home answers from two reads.** It reads `GET /home` and the chosen
  report's view. A chip's mark is the `worst_severity` of its latest report
  that is collected and has not expired, never a rule of the web app. A
  report that covers some namespaces says so in the meta line.
- **The overview's parts have one implementation,** used by the Overview tab
  and by Home, with links built from the report's base path.
- **A row with no finding says so.** The view gives such a row the status
  healthy; the tables show "No finding" with no mark, as the PDF does, and
  rank it after the healthy rows.
- **The closed sets of the view are contract enums.** The `View*` enums of
  `api/openapi.yaml` are excluded from the Go generation and held equal to
  the Go registries by `TestViewEnumsEqualTheRegistries`, so the frontend's
  families come from the generated lists.
- **The state of a tab lives in the address.** `q`, `sort`, `open`,
  `severity`, `scope`, and `finding` are written with replace, and an unknown
  value is ignored.
- **`DataTable` is the one table of the app,** in `web/src/components/`, on
  TanStack Table: the report's tabs, the users, and the profiles. Opening a
  row (by a click or by `open` in the address) reveals it through
  `revealRows`: the row and its opened row scroll between the sticky header
  and the bottom of the window, or, when the pair is taller than the window,
  the row's top settles under the header; closing never scrolls. Cells wrap
  and nothing scrolls sideways. A `DataTable` with no `renderOpened`, as the
  Node pools table, has no expander column and ignores `open`. The e2e checks the fit with `expectFits` at
  1366×768, 1200×1000, and 1024×768, on a users table made longer than the
  window.
- **The administration is for administrators alone.** `/users`,
  `/profiles`, `/profiles/new`, and `/profiles/:id/edit` sit behind
  `RequireAdministrator` and answer the not-found screen to anyone else;
  their navigation entries show only to an administrator. Rows open in
  place; New user is a `Dialog`.
- **The profile form guards its changes.** While it has changes, a
  navigation in the app asks first (`useBlocker`) and `beforeunload` warns;
  save and cancel release the guard, and a navigation to sign-in (sign-out,
  an ended session) always passes. A refusal of the API lands on its field,
  opening a folded section, and a field the form lacks shows in the form's
  alert; a mark the API set leaves its field when the field changes, and a
  new test or save clears every earlier one. A test result is cleared by
  the first change after it.
- **A mark that the PDF prints beside a value is drawn with its shape,** and,
  where its label is not the severity, with the status word for screen
  readers (`StatusMark` with `withStatus`).
- **The content of every Radix layer sits at `z-50`,** above the sticky
  header of the table.
- **The report screens check only while something moves.** The list
  refetches every loaded page every 5 seconds while a loaded row is queued,
  running, or has a pending PDF, and not otherwise. The progress page
  checks after 1.5 s for its first 20 answers, 3 s for the next 20, then
  6 s, while the report is queued, running, or has a pending PDF, and counts
  again from the answer that shows the report active again, as after Generate
  PDF; a 404 or 410 stops it.
- **Home checks only while something moves,** the rule of the Reports list.
- **The screens show what the role allows.** A checkbox and Delete appear
  on a row only for an administrator, or for an operator on their own
  report; Generate PDF, Try again, and New report only for operators and
  administrators. A bulk outcome `forbidden` keeps its row selected.
- **One idempotency key per attempt.** The form makes a key when it opens
  and renews it when a value changes after an attempt got an answer, or on
  `IDEMPOTENCY_KEY_REUSED`; an attempt that got no answer keeps it, so a
  resend replays.
- **A select is the browser's own `<select>`,** styled, so no style hash
  enters the CSP.
- **A period shows its words.** The six quick ranges have their own keys;
  any other value reads `period.custom.value`, with the duration from
  `Intl.NumberFormat` in long units, so it agrees in number in both
  languages.
- **The period picker holds one of three kinds of value:** a length, a
  calendar name, or a typed range of two wall times. A calendar name is
  resolved in the form's zone when the request is sent, never when it is
  picked; weeks start on Monday in both languages; an empty resolved range
  sends nothing and shows `period.range.order` on the period. The resolved
  range of an attempt is kept until a value changes or the key renews, so a
  resend replays. The calendar is styled through `classNames` and
  `modifiersClassNames` alone, never through the library's stylesheet.

## The template

`template/graphite/` is the generic page shell, a port of `graphite-report`
1.0.0 that stays a valid Typst package. `template/capacity/` is the layout of
the capacity report. Go decides what is said; the template decides how it
looks.

- The report layer reads one JSON data file. Its contract is in the plan of
  phase 1, and `template/capacity/validate.typ` enforces it.
- `template.Unpack` writes only into a fresh folder: one that does not exist,
  or exists and is empty, and is not a symbolic link. The caller creates a new
  folder for each run and discards it afterwards.
- Every word on the page arrives in the data file. The template adds no
  wording of its own and never evaluates a value as markup. `eval` is
  forbidden, and a test scans for it.
- Every diagnostic of the generic layer starts with `graphite-report: `, and
  every diagnostic of the report layer with `capacity-report: `. All of the
  report layer's diagnostics live in one dictionary in `validate.typ`, and
  each has a fixture that triggers it. A data file that is not valid JSON
  stops with Typst's own message, because Typst cannot catch a parse
  error; the command that writes the file answers for its syntax.
- The report is set in three faces, all shipped in `template/graphite/assets/fonts/`:
  Public Sans 2.001 for words, JetBrains Mono 2.304 for every name, number,
  and status glyph, and Liberation Sans for the generic layer's own default.
  The five status glyphs (the diamond of critical, the triangle of attention,
  the circle of healthy, the empty circle of no base, the dash of unavailable)
  are set in JetBrains Mono, which holds them all; critical is bold. A status
  glyph is a PDF artifact when its word stands beside it; otherwise it
  carries that word as alternate text, which the data file gives as the
  run's `alt`.
- A table column has a face: `name` and `num` and `mark` and `text`, held with
  its width in `template/capacity/tables.typ`. A number is set in mono and
  aligned right, a name and a text and a mark from the left.
- The data file that the command writes opens with the verdict block: the
  answer strip, the node gauge, the pressure of each scope, the head of the
  action plan, and the legend, under the title band that the graphite title
  draws. The front matter follows it. It closes with the colophon block: a
  rule, the load-line disc, and the line naming the program.
- The report layer has 23 diagnostics in `validate.typ`.
- A page uses only the colours of `template/capacity/palette.typ`, the light
  status colours of the web, which a test holds equal; words and numbers stay
  in ink.
- The public surface of the generic layer is frozen: `graphite-report`,
  `callout`, `keep-together`, with the keys its README documents.

### Golden pages

- `template/golden/snapshots.go` is the one list of golden pages. Each entry
  states the one behaviour it alone protects.
- A snapshot covers every page of its document, or says in `Omits` which
  pages it leaves out and why. A test compares the list with the page count.
- Pages are rendered at 144 PPI and compared by SHA-256.
- A test never writes a golden page. `make update-golden` does, and only for
  an intended visual change.
- After `make update-golden`, open every page that changed and review it
  before committing. A page that changed without a reason you can name is a
  defect, not a new golden page.
- The snapshot `capacity-parity` pins the numbers of the original report:
  every value of its tables reaches its cell unchanged. Parity binds the
  numbers alone; the words follow the web app, and the order of the
  sections, the layout, and the faces are this project's (decided
  2026-10-12). A change to a number needs a decision recorded in
  `ROADMAP.md`; a change to the page margins, one inch at the top and the
  sides and half an inch at the bottom, needs a new golden page.
- Rendering was measured equal on linux/amd64, linux/arm64, and macOS in
  2026-09 (`docs/decisions/0001-golden-architecture.md`). The goldens of
  2026-10 are bound to linux/amd64, the platform of the gate and of
  production, by the owner's decision of 2026-10-13; another platform is
  measured only by a decision of its own. Measure again when the pinned
  Typst version changes.

## The chart

`deploy/helm/capacity-room/` runs the platform on Kubernetes. Spec
`docs/superpowers/specs/2026-10-14-kubernetes-deployment-design.md` is its
design, and section 12 of `docs/operations.md` is the operator's guide to it.

- **The chart lives in `deploy/helm/capacity-room/` and is not published.** An
  operator installs it from a checkout.
- **The chart's README is the quick start of section 12.** It links to
  `docs/operations.md` and changes with it in the same commit.
- **It creates no Secret, no PersistentVolumeClaim, and no NetworkPolicy.**
  The operator makes the Secrets and the ConfigMap it names, and the pull
  secret of a private registry.
- **Nothing is mounted under `/etc`.**
- **One version.** `Chart.yaml`'s `version` and `appVersion` equal `VERSION`,
  held by a test.
- **The schema follows the code.** The `platform` keys of `values.schema.json`,
  with the three chart-owned keys (`server_secret_path`, `credential_dir`,
  `ca_bundle_dir`), equal the settings keys, and its `profiles` keys equal
  the keys of the import file, both held by tests.
- **Every pod meets `restricted` and mounts no service account token.**
- **The hooks.** The admin Job is `post-install` alone, and the profiles Job
  is `post-install,post-upgrade`.
- **kubeconform reads only the schemas of `deploy/helm/schemas/`,** so a new
  kind needs its schema there.
- **The homelab's own files never enter the repository.** The chart's examples
  use `example.com` and placeholder names.

## Delegating to subagents

The main thread plans, judges, and runs the gates. Subagents implement.

- A subagent may run `go`, `npm` inside `web/`, `make web`, `make build`,
  `make test`, `make lint`, `make generate`, and `make generate-check`.
- A subagent must not run Docker, so `make image`, `make test-toolchain`,
  `make test-template`, `make test-golden`, `make update-golden`,
  `make postgres-up`, `make postgres-down`, `make test-postgres`,
  `make test-chart`, `make push`, `make e2e`, and `make gate` stay on the main thread. The main thread starts PostgreSQL
  with `make postgres-up` and gives the subagent the DSN; the subagent runs
  the tagged tests with `CAPACITYROOM_TEST_DATABASE_DSN` set.
- Two agents in one checkout break each other's builds when they share a Go
  package. Give a reviewer a diff file, and let it run tests only on packages
  nobody is editing.
- A subagent cannot declare a task or a phase finished.
- Interface design, API design, and catalog wording stay on the main thread.
- Tasks that share a file run in sequence. When in doubt, treat two tasks as
  sharing: an unneeded sequence costs time, a silent overwrite costs the work.
- Every commit names its paths: `git add <paths>`, never `git add -A` and
  never `git commit -a`.
- A subagent never runs `git stash`, `git restore`, `git checkout` of a
  path, or `git clean`. The checkout holds changes of the main thread that
  are not its own.
- A brief is cut from the plan. When an implementer finds that the brief and
  the original disagree, it stops and reports; the main thread runs the
  original, rules, corrects the plan, and cuts the brief again.
- After a round of parallel work, the full suite runs once over the combined
  result.
- An adversarial review runs over the whole branch before a phase is merged.

## Phases and branches

Work is organised in phases, one implementation plan per phase, under
`docs/superpowers/plans/`.

1. **Before opening a phase,** read `ROADMAP.md` and `CHANGELOG.md` and
   confirm they describe the real state. Correct them first.
2. **Opening a phase is creating its branch.** Nothing is edited on `main`.
   Confirm with `git branch --show-current` before touching a file.
3. **Integration is by phase, not by item.** The branch is merged into `main`
   by fast-forward when the last item of the phase closes.
4. **Before merging:** run `make gate`; update `ROADMAP.md`; review
   `CHANGELOG.md`; read both files back to confirm they changed.

## ROADMAP.md

`ROADMAP.md` sits at the repository root and is not tracked. It is a working
tool, not a deliverable. If it does not exist, create it with this structure
before starting any work:

- **How to use** and the **legend**. Status: `DONE`, `OPEN`, `PARTIAL`,
  `DEFERRED`, `BLOCKED`, `REMOVED`. Source: `PLAN` item of an implementation
  plan, `REV` code review, `PROD` real execution, `JUDGE` review of an
  agent's work.
- A summary table with one line per phase.
- One section per phase. Each item has a stable identifier, a source, a
  one-line description, and `file:line`.
- **Findings log**, where every new finding is recorded before it receives a
  phase.
- **Learned process rules**, so that an orchestration mistake is not repeated.

Rules:

- Every finding goes into `ROADMAP.md`, wherever it appeared. A finding
  without a phase becomes invisible debt.
- A `DONE` item names the commit that closed it. A `DONE` item with no test
  does not count.
- A finding from real execution takes precedence over one from reading code.
- Identifiers are stable and never reused.
- `REMOVED` is for an item whose target left the project.

## Changelog and version

`CHANGELOG.md` is tracked. It follows Keep a Changelog 1.1.0 and Semantic
Versioning.

- Entries are grouped under `### Added`, `### Changed`, `### Deprecated`,
  `### Removed`, `### Fixed`, and `### Security`. A group appears only when it
  has items.
- A version heading is `## [X.Y.Z] - YYYY-MM-DD`. A guard rejects any other
  form.
- Each item is one sentence about what a user perceives. No function names,
  no commit hashes, no internal terms.
- Work with no effect for users is left out: tests, internal documents,
  repository settings. State each omission when reporting, because a silent
  omission cannot be told from forgetting.
- Merging and raising the version are separate decisions. A phase made only
  of tests or internal work is merged with no new version and no tag.
- The newest version heading equals `VERSION`. A guard checks it.
- A defect fix is a patch. A change to when a finding appears, to a sentence
  of the report, or to a visual default is a minor version.
- A tag is `vX.Y.Z`, annotated, and created only after `make gate` passes on
  that commit. Its logs are kept under `.cache/release-X.Y.Z/`.

## Commits

Conventional format, `type(scope): description`, imperative, subject under 50
characters. Types: `feat`, `fix`, `docs`, `style`, `refactor`, `perf`, `test`,
`chore`, `ci`, `build`. The body, when needed, uses `*` bullets.
