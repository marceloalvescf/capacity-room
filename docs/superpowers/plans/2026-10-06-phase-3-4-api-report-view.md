# Phase 3.4: The Report API, the Report View, and the Contract Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Every report route of spec 11.10 answers through `serve`: an operator requests a report with an idempotency key and is refused with 429 and `Retry-After` over the admission limits; every signed-in reader lists the reports by page, reads one, opens it worded in a language (the report view of spec 11.11), and downloads its snapshot and its PDFs with an `ETag`; an operator retries a PDF and deletes a report, one or up to 200, and the maintenance job sweeps what was deleted; an expired report answers 410 until its purge and 404 after. The findings F-118 to F-122, F-124, and F-125 close first.

**Architecture:** No new package. The report view is a second product of `internal/document`, beside the Typst data file, built from the same snapshot by the same helpers so that the view says what the PDF says. The reports service of `internal/platform/reports` gains the reads, the delete, and the bulk delete; the store gains the list, the reads of the bodies, the mark, and the sweep; `internal/platform/api` gains the report routes, the codes, and the conditional answers through `http.ServeContent`; `serve` builds the service with an insert-only River client. A deleted report is marked and hidden at once; the maintenance job removes its rows once no attempt runs it.

**Tech Stack:** Go 1.27 and the modules already in `go.mod`; no new module. River v0.49.0 (insert-only client in `serve`). PostgreSQL 18 through the harness of 3.1. Typst 0.15.1 for the proof. `oapi-codegen` v2.8.0 through `make generate`.

**Spec:** `docs/superpowers/specs/2026-09-27-k8s-capacity-report-design.md`, sections 4.3, 5.4, 11.3, 11.7, 11.8, 11.9, 11.10, 11.11, 11.12, 16. The research of this part is `.cache/phase-3/part-4/research/original-api.md` (the original's report routes, with file and line). The findings are in `ROADMAP.md`; the probe of F-118 and the latent gaps of F-119 to F-122 are in `.cache/phase-3/part-3/review/rereview.md` (N1 to N4) with their sources under `.cache/phase-3/part-3/review/probe/rereview-notes/`; the minors of F-125 are in `.cache/phase-3/part-3/review/ledger-minors-and-rulings.md`.

**Depends on:** Part 3.3, merged into `main` at `6ea960b` and pushed on 2026-10-06.

## Where this part sits

| Part | Delivers | Proof at the end |
|---|---|---|
| 3.1, merged | Settings, store, audit, accounts, sessions, the API skeleton, `serve`, `admin` | An administrator bootstrapped from the command line signs in and creates an operator |
| 3.2, merged | Profiles and revisions, the reference files, the outbound policy, the profile routes and test, `admin import-profiles` | A profile created through the API is tested against the fake Prometheus admitted by the policy |
| 3.3, merged | The shared pipeline and render, the frozen snapshot, River, the reports service (request, retry), the collect, render, and maintenance jobs, the `worker` command | A report requested through the service is collected, rendered, expired, and purged by the client `worker` builds |
| **3.4 API, report view, contract (this plan)** | F-118 to F-122, F-124, F-125; the report routes, the view, pagination, ETags, delete and its sweep, the 429 and 410 answers, the service wired into `serve` | A report requested through the API of the `serve` handler, run by the `worker` client, read, viewed in both languages, and downloaded through the API, as spec 16 asks |

### Out of this part

| Left out | Arrives in | Reason |
|---|---|---|
| Filters of the list (status, profile, owner) | 4, if a screen needs one | Spec 11.10 asks for a list by page; no screen of spec 12.2 names a filter |
| A cancel of a running collection on delete | none | R156: the attempt finishes and the sweep removes the row; the original's checkpoint cancel would add a code and a path for at most `collect_timeout_seconds` of work |
| Chunked streaming of a PDF from the database | none | R155: a body is at most 64 MiB and a real PDF is about 200 KiB; `http.ServeContent` streams from memory |
| F-123 (a stale render storing a failure code) | none | Deferred by the owner's flow on 2026-10-06 |
| F-110, F-111 (the CSRF token on the session route) | 4 | Notes for the frontend |
| A Compose file of the platform | 5 | Spec 14.2 |

## How to read this plan

The owner's orchestration rule applies: the plan states what each task must deliver and how it is proven, and the implementer writes the code. Each task gives exact paths, exact signatures, and a table of tests with inputs and expected results. Write each test first, watch it fail, then implement.

Executors:

- **Subagent**: a Sonnet subagent. It may run `go`, `make build`, `make test`, `make lint`, `make generate`, `make generate-check`, and the tagged tests against the PostgreSQL the main thread started (`KCR_TEST_DATABASE_DSN`), with `typst` and `pdftotext` installed natively for the tag `template`. It never starts or stops Docker, never runs `make update-*`, and cannot declare a task finished.
- **Main thread**: needs Docker, judgment, API design, wording, or the owner. Docker is OrbStack's engine (`docker context show` prints `orbstack`).

Rules for every subagent brief, as in the plans of 3.1 to 3.3: tasks in sequence unless this plan says they may run side by side; a reviewer reads a diff file and runs tests only on packages nobody edits; `git add <paths>`; no stash, restore, checkout of a path, or clean; a test of a change goes red first, test by test, on the code before the change; a brief that says "report every X" lists what must not match; a decision flagged at the end of a report has already been made; the main thread commits before dispatching and computes before dispatching. The rules learned in 3.1 to 3.3 bind every task:

- **After a migration, every package with tagged tests runs** (`make test-postgres` over `./internal/platform/... ./internal/cli/...`, and the worker's `postgres template` tests).
- **A service is proven through what the command builds** (Task 8 tests the handler `serve` builds; Task 10 runs the API of that handler and the client `worker` builds).
- **A brief for anything that locks or compares-and-sets names its concurrent test, and the test is shown red under a mutation.**
- **A ruling made on an implementer's report goes to the implementer with a fix request before the review.**
- **The harness creates worktrees from `main`:** the implementer's first step fast-forwards its worktree to `feat/platform-api`; a report is copied with `cp -n` to a new name.
- **A fix that moves a deadline needs a probe of the slow path; a redaction test uses a secret the formatter changes** (a quote and a backslash: `s3cr3t"t0ken\value-123`).
- **Run the binary by hand before the review.**

Order and parallelism: Tasks 1, 2, and 3 share no package and may run side by side, each in its own worktree. Task 4 follows them. Task 5 and Task 7 share no package and may run side by side; Task 6 follows Task 5; Task 8 follows Tasks 6 and 7; Task 9 may run beside Task 8 (it touches no file of `internal/platform/api`, `internal/platform/reports`, or `internal/cli`); Task 10 follows Task 8; Task 11 closes.

Reference material, read-only and never tracked:

- `older/kubernetes-report/kube_capacity_report/platform/api/reports.py`, `api/errors.py`, `api/report_query.py`, `jobs.py`, `retention.py`, `api_models.py`; `older/kubernetes-report/kube_capacity_report/snapshots/presentation_v1.py`.
- `.cache/phase-3/part-4/research/original-api.md`. A brief quotes the lines it needs; an implementer that finds the brief and the original in disagreement stops and reports.

## Global Constraints

Every constraint of the plans of phases 0 to 3.3 stands. The ones below are added or are the ones this part leans on most.

- No new module. No new package, so `guard.Layers` does not change.
- All routes are under `/api/v1` and are described in `api/openapi.yaml`; the route table and the file agree both ways; every operation has the `default` answer `Unexpected`; every code of the table of `errors.go` is named in the file.
- **The checks of a route run in one order:** session (401), role (403), CSRF (403), body or query (400), then the service. The role is read from the database on every request.
- **Authorization runs before any conditional answer.** No 304, 409, or 410 reaches a caller who could not read the report.
- **The sentences of the error envelope are diagnostics** in `internal/platform/api/errors.go`, one table, English, fixed per code. A stored failure code of a report or a PDF is a value of the answer, never a sentence.
- **Every timestamp of this project's tables is passed from Go.**
- **No stored text carries a failure's words** (3.3). No log value is formatted before it is redacted.
- Every catalog entry of this part is written by the main thread; the catalog guards hold (both languages, every key used, no banned term).
- No test skips. Tagged tests carry `//go:build postgres`, or `//go:build postgres && template` when they need Typst. Comments never name a task, plan, or phase. Diagnostics in English. Every literal ASCII.
- Scratch under `.cache/phase-3/part-4/`. Commits `type(scope): description`, imperative, under 50 characters; each subject below is counted.

## Review Focus

Each line names an input the spec implies and does not describe, and the task whose tests pin it.

1. **A delete that races the claim of its collection.** A report deleted while queued is never collected: the claim refuses it, the job completes writing nothing, and the sweep removes it; a report deleted while running finishes its attempt, queues no PDF that renders, and is swept once it is terminal. Pinned in Task 5 (`TestClaimRefusesADeletedReport`, `TestNeedsRenderSkipsADeletedReport`) and Task 6 (`TestDeleteWhileRunningIsSweptAfter`).
2. **A bulk delete of 200 ids by an operator that mixes its own, another operator's, expired, absent, already deleted, and malformed ids.** One answer, one outcome per id in the order sent, one transaction, one audit row per id actually marked; a repeated id or a 201st id refuses the whole body with 400. Pinned in Task 6 (`TestDeleteManyOutcomes`, `TestDeleteManyConcurrent`) and Task 8 (`TestBulkDeleteRoute`).
3. **A conditional request from someone who may not read the report, or for a report that is expired, deleted, or not ready.** No 304: the answer is 404, 410, or 409, as without the header; `If-None-Match` with a weak tag, a list, or `*` matches by the weak comparison of RFC 9110. Pinned in Task 8 (`TestConditionalAnswersAfterAuthorization`).
4. **A list read page by page while reports share a creation instant, and while a report of an earlier page is deleted.** No report repeats and none is skipped; the cursor of a deleted report still pages on; a cursor that is not ours answers 400 `CURSOR_INVALID`. Pinned in Task 5 (`TestListReportsKeysetOnEqualInstants`) and Task 8 (`TestListRoutePages`).
5. **A download while the maintenance job expires, sweeps, or purges the report.** The answer is a whole 200, a 404, or a 410, never a 500 and never a body cut short: the row and its body are read in one statement. Pinned in Task 5 (`TestReadBodiesAreOneStatement`) and Task 8 (`TestDownloadRacesTheSweep`).

## Rulings made on the owner's behalf

Each is a decision the spec leaves open, written as decision, reason, and cost if wrong. The owner may overturn any of them when reviewing this plan. The numbering continues that of part 3.3.

| ID | Ruling |
|---|---|
| R143 | Ruling (F-118): the success write of collect and of render runs over `context.WithoutCancel(ctx)` with the deadline at the later of the job's deadline and now plus the outcome timeout (`successContext`); the failure writes keep `outcomeContext`; an error of a success write whose own context is done (`wctx.Err() != nil`) is `INTERNAL_FAILURE`, decided before `classify` and `renderClassify` and never through `profiles.Classify`; the outcome timeout becomes a field of the workers' test hooks (default 10 s) so the slow path is probed in seconds — the re-review's N1 and its fix (`rereview.md`) — a success write may run until the job's deadline, so a stop waits for it up to the drain and the cancel, as before E158 |
| R144 | Ruling (F-119, F-120): the run's redaction of log values moves from `Redactor.ReplaceAttr` to `redact.Handler`, a `slog.Handler` that wraps the text handler and redacts at `Handle`: the message, and every attribute of the record and of `WithAttrs`, inside groups; a `WithAttrs` or `WithGroup` is recorded and replayed on the inner handler at each `Handle`, so a value bound before its secret was added is redacted by the secrets known when the line is written; a string is redacted as text; an error or a `fmt.Stringer` through `fmt.Sprint`, which prints `<nil>` for a nil receiver and recovers a panic as fmt does; a `[]byte` as its text; any other `KindAny` value through `fmt.Sprintf("%+v")`; the other kinds pass; `ReplaceAttr` is removed — the re-review's N2 and N3, and the rule that a value is redacted before it is formatted — every log line costs one replay of the bound attributes, which no route logs more than once |
| R145 | Ruling (F-124): the assignment pattern of spec 5.4 keeps its three names; a value that opens with a quote is redacted to its closing quote, which is kept (`token="[redacted]"`); any other value stops before white space, `,`, `;`, a quote, `}`, `]`, or `)` — the original's pattern swallowed the closing quote and brace of a structured line — a secret that holds one of those characters is cut there by the pattern, and only a literal secret of the redactor removes the rest; a deliberate difference from the original |
| R146 | Ruling (F-122): a test of `internal/cli` reads the Go files of `internal/cli` and `internal/platform/...`, test files left out, and fails when a call to `slog.NewTextHandler`, `slog.NewJSONHandler`, `slog.SetDefault`, `slog.Default`, or `log.Print*`/`log.Fatal*`/`log.Panic*` appears outside `internal/cli/logger.go`, except a handler whose writer is the literal `io.Discard` (the two of `internal/platform/api`, E87); `slog.New` over a handler it is given (River's `atLeast` in `worker/client.go`) is allowed; a second test runs `newLogger` with the secret of the rule above and finds it nowhere — a command that builds its own handler is the regression to stop, and the static scan finds it where a test of behaviour needs a running command — the scan reads call names, so a handler built through an alias of the package passes |
| R147 | Ruling (F-121 and the drift minor of Task 5 of 3.3): `readGoose` refuses any comment line that holds `+goose` in any case and spacing unless it is exactly one of the four annotations the file uses, refuses a statement outside a block, and refuses `-- +goose Down` anywhere but once after the last Up block; the annotation check runs over every file of `migrations/`, the comparison with River over `00003` alone — goose reads any `--` line that holds `+goose` (goose v3.28.0 `internal/sqlparser/parser.go:127,317-347`) — none |
| R148 | Ruling (F-125): of the 26 minors, 15 are fixed (Tasks 1, 3, 9) and 11 are closed as accepted, by the table of Task 9 — each was read against its code on 2026-10-06 — none |
| R149 | Ruling: the routes and their answers: `POST /reports` 202 `ReportAccepted` (a replay too); `GET /reports` 200 `ReportPage`; `GET /reports/{id}` 200 `Report`; `GET /reports/{id}/view?lang=` 200 `ReportView`; `GET /reports/{id}/snapshot` 200 `application/json`; `GET /reports/{id}/pdf?lang=` 200 `application/pdf`; `POST /reports/{id}/pdf-retries` 202 `PDFRetryAccepted`; `DELETE /reports/{id}` 204; `POST /reports/delete` 200 `ReportDeleteResults`; HEAD is not routed — spec 11.10; the original's statuses (`original-api.md`, section 1) — none |
| R150 | Ruling: every signed-in user reads every report that is not deleted (list, detail, view, snapshot, PDF); an operator or an administrator requests and retries; an administrator deletes any report, an operator only a report it requested (another's is 403 `FORBIDDEN`, since the operator can read it), a reader none (403) — spec 11.3 ("Open reports: Reader Yes"; "Delete a report: Operator: Own requests") — a reader sees who requested what, which the table of 11.3 allows |
| R151 | Ruling: the list is ordered by `created_at` descending, then `id` descending; `limit` 1 to 200, default 50; `cursor` is the base64url, without padding, of the creation instant in RFC 3339 with nanoseconds in UTC, a line feed, and the id; a cursor that does not decode, parse, or round-trip is 400 `CURSOR_INVALID`; the page is `{items, next_cursor}`, `next_cursor` null on the last page (the store reads `limit + 1`); deleted reports are left out, expired ones listed with `expired: true`; no filter — spec 11.10 ("a cursor on creation time and identifier"); the original's form (`original-api.md`, List) without its filters — a screen of phase 4 that needs a filter adds a query parameter |
| R152 | Ruling: `ReportSummary` is `{id, owner_id, owner_username, profile_id, status, phase, failure_code, pdfs: [{language, status, failure_code}], created_at, completed_at, expires_at, expired}`; `Report` (the detail) adds `profile_revision_id, attempt, started_at, collected_at, selection: {namespaces, period, timezone, report_profile, min_severity}`; `pdfs` is empty until the collection stores the rows; a code is an enum of the contract and the frontend words it — spec 11.10 ("status and progress"), 11.7 (phase), 12.2 (the PDF downloads in the header with their status); spec 11.10 (the frontend translates a code) — a reader sees the user name of the requester |
| R153 | Ruling: a report that is absent or deleted is 404 `REPORT_NOT_FOUND`; a report whose `expires_at` is at or before now is 410 `REPORT_EXPIRED` on the detail, the view, the snapshot, the PDF, the retry, and the single delete, until the purge, and 404 after; the list carries it with `expired: true`; a bulk delete answers it as the outcome `expired` — spec 11.8; R128; the original (`original-api.md`, 404 and 410) — none |
| R154 | Ruling: the codes of this part join the one table of `errors.go`: `IDEMPOTENCY_KEY_INVALID` 400 "The Idempotency-Key header is not valid.", `CURSOR_INVALID` 400 "The cursor is not valid.", `REPORT_NOT_FOUND` 404 "No such report was found.", `IDEMPOTENCY_KEY_REUSED` 409 "This idempotency key was already used for a different request.", `PROFILE_INACTIVE` 409 "This profile is not active.", `SNAPSHOT_NOT_READY` 409 "This report's data is not ready yet.", `PDF_NOT_READY` 409 "This report's PDF is not ready yet." (fields `status` and, when set, `failure_code`), `PDF_RETRY_NOT_ALLOWED` 409 "This report's PDF cannot be retried right now.", `REPORT_EXPIRED` 410 "This report has expired.", `REPORT_SELECTION_NOT_PERMITTED` 422 "This selection is not permitted by the profile." (fields from the service), `ADMISSION_GLOBAL_LIMIT` 429 "The platform is at its global report admission limit.", `ADMISSION_PROFILE_LIMIT` 429 "This profile is at its report admission limit." with `Retry-After` in whole seconds, rounded up, at least 1; `PROFILE_NOT_FOUND` keeps 404; a missing or malformed `Idempotency-Key` is `IDEMPOTENCY_KEY_INVALID` (the service judges both); a missing or invalid `lang` is 400 `REQUEST_INVALID {"lang": "missing" | "invalid"}` — the original's sentences where the code exists (`original-api.md`, Errors); spec 11.7 (429 with `Retry-After`) — none |
| R155 | Ruling: the snapshot route serves the stored bytes as they are, unnarrowed, `Content-Disposition: attachment; filename="report-<id>.json"`; the PDF route the stored bytes of one language, `filename="report-<id>-<lang>.pdf"`; the view is served inline; each answer of the three carries a strong `ETag`: the stored SHA-256 of the snapshot or of the PDF, and the SHA-256 of the view's bytes as sent (built on each request); `Cache-Control: private, no-cache` on their 200 and 304; the conditional answer, `Range`, and `Content-Length` are `http.ServeContent`'s, over the bytes read whole in one statement after authorization — spec 11.10 (ETags; the BFF streams the PDF; no link bypasses the check), 11.12 (a route sets its own cache rule); the stored snapshot is the input of `--from-snapshot` and the narrowing is the view's — a download holds its body in memory, at most 64 MiB a PDF |
| R156 | Ruling: a delete marks `reports.deleted_at = now` where it is null, in one transaction that writes `report.delete` (`{"status"}` at the mark, the actor); from the mark the report is 404 everywhere and a replay of its idempotency key is `REPORT_NOT_FOUND`; a second delete is 404; a queued deleted report is never claimed (the claim completes the job and writes nothing, as for an absent row); a running one finishes its attempt; a render skips a deleted report; reconcile and expiry leave deleted reports alone; admission counts them as before (a deleted queued report counts until the sweep, as in the original); the maintenance job sweeps, after reconcile, every deleted report whose status is not `running`, batch 100, oldest mark first, rows `FOR UPDATE SKIP LOCKED`: its PDF rows, snapshot, idempotency row, and row, with `report.purge {"cause": "deleted"}`; the purge of an expired report writes `report.purge {"cause": "expired"}` — spec 11.10 (delete), 11.9 (every change to reports); the original's mark and sweep (`original-api.md`, Delete) without its checkpoint cancel — a deleted running collection uses its slot until it ends, at most `collect_timeout_seconds` |
| R157 | Ruling: `00004_reports.sql` is edited in place: `reports.deleted_at timestamptz` null, the index `ix_reports_listing (created_at DESC, id DESC) WHERE deleted_at IS NULL`, the index `ix_reports_deleted (deleted_at, id) WHERE deleted_at IS NOT NULL` — no tag holds `00004` (E102, E157), and the owner's rule keeps no migration for compatibility — a database migrated from `main` before this part must be dropped and migrated again; the first tag of the platform freezes `00004` |
| R158 | Ruling: `POST /reports/{id}/pdf-retries` takes `{"language": "pt-BR" | "en"}` and answers 202 `{"language", "status": "pending"}`, a retry of a `pending` row included; the service's codes map by R154 — spec 11.10 ("retry one language"); R131 — none |
| R159 | Ruling: `POST /reports` takes the `Idempotency-Key` header and the body `ReportRequest`, the JSON of `reports.Selection` (`profile_id`, `namespaces`, `period`, `timezone`, `report_profile`, `min_severity`; absent or null takes the profile's default), and answers 202 `{"id", "status", "phase", "expires_at", "replayed"}` with the report's current status and phase — spec 11.7; the original's 202 — none |
| R160 | Ruling: the report view is `document.BuildView(s, c) (View, error)`, beside `Build`, from the same helpers: the text of every value is the text the PDF prints for the same quantity; a closed set (severity, status, state, strategy, confidence, scope type and kind, HPA coverage, autoscaling status and source) travels as its code and the frontend words it; a text the program composes (a sentence, an action, a reason, a remark, a period, a date, a formatted number, a warning, a notice) travels as text in the catalog's language; a number travels as `{"raw": number or null, "text"}`; the minimum is the snapshot's `min_severity`, raised to attention for the executive profile, as for the whole executive PDF; findings, recommendations, and the namespace, workload, and node rows below the minimum are left out, as the PDF leaves them out; HPAs, node pools, the summary counts, and the quality are whole; the shape is the table of Task 7 — spec 11.11; the original has no view (`original-api.md`) — the view and the PDF diverge if a helper is bypassed, which the test of Task 7 compares |
| R161 | Ruling: the contract describes the view under `components.schemas.ReportView` and its parts, excluded from the Go generation (the Go type is `document.View`, since the core cannot import the API); `TestViewMatchesTheContract` builds the view of every golden snapshot in both languages and walks its JSON against the schema both ways: every key named, every named key present, every JSON type the schema's, `$ref` followed, `nullable` honoured — spec 11.10 (all routes described in the file) — a schema written by hand can drift from the type, which the test stops |
| R162 | Ruling: `buildServer` returns `(*api.Server, error)` and builds `reports.Service` with `reports.NewQueue(pool, logger)`, the insert-only River client of 3.3, the settings, the clock, and `uuid.New`; `api.Server` gains `Reports *reports.Service`; a tagged test runs the handler `serve` builds — E127; R141 — none |
| R163 | Ruling: the proof of spec 16 is `TestReportThroughTheAPI` (`//go:build postgres && template`) in `internal/cli`: the handler of `buildServer`, the client of `buildWorker`, and the fake Prometheus admitted by `127.0.0.1/32`; an administrator bootstrapped, an operator created and signed in through the API, a profile imported, a report requested through `POST /reports`, polled through `GET /reports/{id}` until `complete`, read through the list, viewed in both languages, and both PDFs and the snapshot downloaded through the API and compared byte for byte with what `render` and `pipeline` write for the same inputs; then the binaries are run by hand — spec 16 ("a report requested and downloaded through the API") — none |

## Findings this plan closes

| Finding | How |
|---|---|
| F-118 | Task 1 (R143) |
| F-119, F-120 | Task 2 (R144) |
| F-122 | Task 2 (R146) |
| F-124 | Task 2 (R145) |
| F-121 | Task 3 (R147) |
| F-125 | Tasks 1, 3, and 9 (R148) |
| F-126 | Task 0 (the OrbStack instructions committed) and Task 11 (`make gate` on OrbStack) |

## File structure

| Path | Responsibility in this part |
|---|---|
| `internal/platform/worker/collect.go`, `render.go` and tests | F-118 and the render minors (Task 1) |
| `internal/redact/redact.go`, `handler.go` and tests; `internal/cli/logger.go`, `logger_test.go`; `internal/platform/worker/client.go` | The redacting handler, F-124, F-122 (Task 2) |
| `internal/platform/store/river_test.go`; `Makefile` | F-121, the split of `test-postgres` (Task 3) |
| `internal/platform/store/migrations/00004_reports.sql`; `internal/platform/store/reports.go`, `retention.go` and tests; `internal/platform/worker/collect.go`, `render.go`, `maintenance.go` (the deleted checks) | The schema and the queries of this part (Task 5) |
| `internal/platform/reports/read.go`, `delete.go`, `request.go`, `service.go` and tests; `internal/platform/worker/maintenance.go` | Reads, delete, bulk delete, the sweep step (Task 6) |
| `internal/document/view.go`, `view_test.go`; `internal/catalog/keys.go`, `locales/*.toml` | The report view (Task 7) |
| `api/openapi.yaml`, `internal/platform/api/openapi.gen.go`, `api/oapi-codegen.yaml` | The contract (Task 8, main thread) |
| `internal/platform/api/reports.go`, `errors.go`, `routes.go`, `server.go`, `view_contract_test.go` and tests; `internal/cli/serve.go`, `serve_wiring_test.go` | The routes and the wiring (Task 8) |
| the files of the table of Task 9 | The test minors (Task 9) |
| `internal/cli/api_end_to_end_test.go` | The proof (Task 10) |
| `AGENTS.md`, `README.md`, `CHANGELOG.md` | The documents (Task 11) |

---

### Task 0: Open the part

**Executor:** Main thread.

- [x] **Step 1:** `main` at `6ea960b`, pushed; `ROADMAP.md` and `CHANGELOG.md` read and true; F-126 recorded.
- [x] **Step 2:** `git switch -c feat/platform-api`.
- [ ] **Step 3:** Commit the OrbStack instructions (F-126): `git add AGENTS.md Makefile`; `docs: run Docker through OrbStack` (31 characters).
- [ ] **Step 4:** `ROADMAP.md`: the summary line of 3.4 names this plan; a section "Phase 3.4" with P34-0 to P34-11, all `OPEN`; the rulings R143 to R163; the owner's approval with its date.
- [ ] **Step 5:** Commit the plan: `docs(plan): add the plan of part 3.4` (36 characters).

**Done when:** the plan is committed on the branch, approved by the owner, and `ROADMAP.md` holds the part.

---

### Task 1: The success write keeps its budget (R143; F-118; F-125 render minors)

**Executor:** Subagent, against the running server. May run beside Tasks 2 and 3. Opus reviews (it moves a deadline).

**Files:**
- Modify: `internal/platform/worker/collect.go`, `render.go`
- Test: `internal/platform/worker/collect_test.go`, `render_test.go`

**Interfaces:**

```go
// successContext is the context of the transaction that stores a success:
// free of the job's cancel, with its deadline at the later of the job's
// deadline and now plus the outcome timeout.
func successContext(ctx context.Context, outcome time.Duration) (context.Context, context.CancelFunc)
```

- The hooks of `CollectWorker` and `RenderWorker` gain `outcomeTimeout time.Duration`; zero means `outcomeTimeout` (10 s). `outcomeContext` takes the duration too.
- The success branch of `collect` and of `Work` in render: `wctx, cancel := successContext(ctx, w.outcome())`; on an error, when `wctx.Err() != nil` the code is `INTERNAL_FAILURE` and the failure is written through `handle` with that code decided, without `classify`/`renderClassify`.

- [ ] **Step 1: Write the failing tests**

| Test | Input | Expected |
|---|---|---|
| `TestCollectSlowSuccessWriteKeepsItsBudget` | `CollectTimeout` 60 s, `JobMaxAttempts` 1, hook outcome timeout 1 s, `insideCollected` sleeps 2 s | report `collected`, phase `rendering`, no failure code (red today: `failed`, `ENDPOINT_UNREACHABLE`) |
| `TestCollectSuccessWritePastItsDeadlineIsInternal` | `CollectTimeout` 3 s, hook outcome timeout 1 s, `insideCollected` sleeps until 1 s after the job's deadline | report `failed`, `INTERNAL_FAILURE`, never `ENDPOINT_UNREACHABLE` or `COLLECTION_TIMEOUT`; the audit row `report.collect` failed with `{"code":"INTERNAL_FAILURE"}` |
| `TestRenderSlowSuccessWriteKeepsItsBudget` | `RenderTimeout` 60 s, hook outcome timeout 1 s, `beforeStore` passes and the store sleeps 2 s inside its transaction (a new hook `insideStore`) | the row `ready` |
| `TestRenderSuccessWritePastItsDeadlineIsInternal` | as above, sleeping past the job's deadline | the row stays `pending` while attempts remain (River records `render: INTERNAL_FAILURE`), and `failed` `INTERNAL_FAILURE` on the last attempt |
| `TestRenderSkipsAReportExpiredBetweenTransactions` (F-125, Task 9 of 3.3) | `beforeStore` sets `expired_at` and `expires_at` in the past | nothing written to the PDF row, the job completes, the log says `render skipped: report expired` |
| `TestRenderInternalFailureWithAttemptsLeft` (F-125) | the compile fails with an unclassified error, `JobMaxAttempts` 3, first attempt | the row stays `pending`; River's job `retryable` with the error `render: INTERNAL_FAILURE` |
| `TestNeedsRenderNamesEachSkip` (F-125) | report not collected; expired; row absent | three distinct log messages: `render skipped: report not collected`, `render skipped: report expired`, `render skipped: no PDF row` |

- [ ] **Step 2:** Run `go test -count=1 -tags='postgres template' ./internal/platform/worker/ -run 'Slow|PastItsDeadline|BetweenTransactions|AttemptsLeft|NamesEachSkip' -v`; expected FAIL, each for its reason.
- [ ] **Step 3:** Implement. **Probe the slow path by hand** as `rereview.md` N1 did: copy `.cache/phase-3/part-3/review/probe/rereview-notes/probe_k_test.go` into the package under a scratch name, run it against the new code, record the output in the report, and delete the copy before committing.
- [ ] **Step 4:** `make test`; `go test -count=1 -tags=postgres ./internal/platform/...`; `go test -count=1 -tags='postgres template' ./internal/platform/worker/`; all pass.
- [ ] **Step 5:** Commit `fix(worker): keep the budget of a success write` (47 characters).

**Done when:** the four deadline tests and the three render tests pass, the probe's report shows the slow write stored as `collected`, and the mutation that puts `outcomeContext` back on the success write turns `TestCollectSlowSuccessWriteKeepsItsBudget` red.

---

### Task 2: Redaction at the handler, the assignment pattern, the logger tie (R144, R145, R146; F-119, F-120, F-122, F-124)

**Executor:** Subagent. May run beside Tasks 1 and 3. Opus reviews (redaction).

**Files:**
- Create: `internal/redact/handler.go`, `handler_test.go`
- Modify: `internal/redact/redact.go` (remove `ReplaceAttr`; the pattern), `redact_test.go`; `internal/cli/logger.go`; `internal/platform/worker/client.go` if it names `ReplaceAttr`
- Modify: `internal/cli/logger_test.go` (the scan and the behaviour beside `TestLoggerRedactsBeforeFormatting`)

**Interfaces:**

```go
package redact

// Handler redacts the message and every attribute of a record, those bound
// by WithAttrs included, with the secrets the redactor knows when the
// record is handled, and passes the record to the inner handler.
type Handler struct { /* redactor, inner, recorded WithAttrs and WithGroup */ }

func NewHandler(r *Redactor, inner slog.Handler) *Handler
func (h *Handler) Enabled(ctx context.Context, level slog.Level) bool
func (h *Handler) Handle(ctx context.Context, rec slog.Record) error
func (h *Handler) WithAttrs(attrs []slog.Attr) slog.Handler
func (h *Handler) WithGroup(name string) slog.Handler
```

`cli.newLogger(w, redactor)` is `slog.New(redact.NewHandler(redactor, slog.NewTextHandler(w, nil)))`.

- [ ] **Step 1: Write the failing tests.** The secret of every test is `s3cr3t"t0ken\value-123`, added to the redactor after the logger is built unless the row says otherwise; every expectation is that the output holds neither the secret nor its escaped form `s3cr3t\"t0ken\\value-123`, and holds `[redacted]`.

| Test | Input | Expected |
|---|---|---|
| `TestHandlerRedactsEveryKind` | message; `slog.String`; an error; a `%w` wrapping; `errors.Join`; a Stringer; a `[]byte`; a `map[string]string`; a `[]string`; a struct with an unexported-name field `note`; a group holding a string | each redacted (red today: `[]byte`, map, slice, struct leak) |
| `TestHandlerRedactsBoundBeforeTheSecret` | `logger.With(slog.String("k", secret))` before `Add`, then `Info` | redacted (red today) |
| `TestHandlerReplaysGroupsInOrder` | `WithGroup("a").With("b", 1).WithGroup("c")`, `Info("m", "d", 2)` | the line equals what `slog.NewTextHandler` writes for the same calls: `a.b=1 a.c.d=2` |
| `TestHandlerTypedNilError` | `slog.Any("cause", (*probeErr)(nil))` | `cause=<nil>`, no panic (red today: panic) |
| `TestHandlerPanickingStringer` | a Stringer whose `String` panics | the line is written, with fmt's `%!v(PANIC=` form, no panic |
| `TestAssignmentKeepsTheStructure` | `v="{Token=abc123}"` through `Text` | `v="{Token=[redacted]}"` (red today: `}"` lost) |
| `TestAssignmentQuotedValue` | `token="a b c" next=1` | `token="[redacted]" next=1` |
| `TestAssignmentStopsAtDelimiters` | `api_key=abc]`, `access_token=abc)`, `token: abc,` | `api_key=[redacted]]`, `access_token=[redacted])`, `token=[redacted],` |
| `TestAssignmentLeavesOtherNames` | `x_token=abc`, `tokens=abc` | unchanged |
| `TestNoLoggerOutsideTheConstructor` | the Go files of `internal/cli` and `internal/platform` (`go/parser`, test files left out) | no call named in R146 outside `internal/cli/logger.go` and the two `io.Discard` handlers of `internal/platform/api`; a fixture file in `testdata` with `slog.New(slog.NewTextHandler(os.Stderr, nil))` is reported (the scan can fail) |
| `TestNewLoggerRedactsAnEscapedSecret` | `newLogger` over a buffer, the secret added, `Error("m", "cause", errors.New("x "+secret))` and the same through `With` bound before `Add` | no secret, no escaped secret |

- [ ] **Step 2:** Run the tests of `internal/redact` and `internal/cli`; expected FAIL as marked.
- [ ] **Step 3:** Implement. `spec 5.4` keeps its order (literal secrets first, then the patterns); only the value class of the assignment changes.
- [ ] **Step 4:** `make test`, `go test -race ./internal/redact/ ./internal/cli/`, `make lint`; the worker's tests that build a logger (`collect_test.go` copies the constructor today) call `newLogger`'s equivalent `slog.New(redact.NewHandler(...))`.
- [ ] **Step 5:** Commit `fix(redact): redact log values at the handler` (45 characters).

**Done when:** every row passes, the scan reports its fixture, and the mutation that returns `slog.New(slog.NewTextHandler(w, nil))` from `newLogger` turns `TestNewLoggerRedactsAnEscapedSecret` red.

---

### Task 3: The drift test reads what goose reads (R147; F-121; F-125)

**Executor:** Subagent. May run beside Tasks 1 and 2.

**Files:**
- Modify: `internal/platform/store/river_test.go`, `Makefile` (`test-postgres`)

- [ ] **Step 1: Write the failing tests** (in `river_test.go`, no tag needed for the reader; the existing comparison keeps its tag)

| Test | Input to `readGoose` | Expected |
|---|---|---|
| `TestReadGooseRefusesHiddenAnnotations` | `--+goose NO TRANSACTION`; `--  +GOOSE NO TRANSACTION`; `-- +goose envsub on` | each refused, naming the line (red today: accepted) |
| `TestReadGooseRefusesAStatementOutsideABlock` | `SELECT 1;` between two blocks | refused |
| `TestReadGooseRefusesAMisplacedDown` | `-- +goose Down` before the last Up block; twice | refused |
| `TestEveryMigrationHasOnlyKnownAnnotations` | every file of `migrations/` | passes |

- [ ] **Step 2:** Run; expected FAIL on the first three.
- [ ] **Step 3:** Implement. `Makefile`: the first `go test` of `test-postgres` runs every package of `./internal/platform/...` and `./internal/cli/...` except `internal/platform/worker` (`$$(go list ./internal/platform/... ./internal/cli/... | grep -v /internal/platform/worker$$)`), and the second runs `internal/platform/worker` once with `postgres template`, so its postgres tests run once (F-125, Task 9 of 3.3). Prove the split: the log of `make test-postgres` names `TestReportEndToEnd` once and `TestRiverMigrationMatchesTheLibrary` once (the main thread runs it).
- [ ] **Step 4:** `make test`; `go test -count=1 -tags=postgres ./internal/platform/store/`.
- [ ] **Step 5:** Commit `test(store): read goose annotations as goose does` (49 characters).

**Done when:** the three refusals pass and the probe of N4 (insert `--+goose NO TRANSACTION` after line 1 of `00003`) turns `TestEveryMigrationHasOnlyKnownAnnotations` red.

---

### Task 4: Combined run after the first round

**Executor:** Main thread.

- [ ] **Step 1:** Merge the three worktrees' commits onto the branch in order 1, 2, 3; `make test`; `make lint`; `make postgres-up`; `make test-postgres`; read in its log that each worker test runs once.

**Done when:** the full suite passes over the combined result.

---

### Task 5: The schema and the queries of this part (R151, R153, R155, R156, R157)

**Executor:** Subagent, against the running server. May run beside Task 7. Opus reviews (marks and claims under concurrency).

**Files:**
- Modify: `internal/platform/store/migrations/00004_reports.sql`, `internal/platform/store/reports.go`, `retention.go`, and their tests; `internal/platform/worker/render.go` (`needsRender`), `collect.go` (the claim's refusal)

**Interfaces:**

```go
package store

// Report gains DeletedAt *time.Time.

// ReportListItem is one row of the list: the report, its owner's user name,
// and its PDF rows without bodies, in language order.
type ReportListItem struct {
	Report
	OwnerUsername string
	PDFs          []ReportPDF
}

// ReportCursor is the position after which a page starts.
type ReportCursor struct {
	CreatedAt time.Time
	ID        uuid.UUID
}

// ListReports returns at most limit rows not deleted, newest first, after
// the cursor when one is given.
func ListReports(ctx context.Context, q Querier, after *ReportCursor, limit int) ([]ReportListItem, error)

// GetReportItem returns one report not deleted, with its owner's user name
// and PDF rows; ErrNotFound when absent or deleted.
func GetReportItem(ctx context.Context, q Querier, id uuid.UUID) (ReportListItem, error)

// ReadSnapshot returns, in one statement, the snapshot of a report that is
// collected, not deleted, and not expired at now, with the report's status
// and expiry: ErrNotFound when the report is absent or deleted.
func ReadSnapshot(ctx context.Context, q Querier, id uuid.UUID) (SnapshotRead, error)

// ReadPDF returns, in one statement, the PDF row of one language with its
// body, and the report's status and expiry; ErrNotFound when the report is
// absent or deleted.
func ReadPDF(ctx context.Context, q Querier, id uuid.UUID, language string) (PDFRead, error)

type SnapshotRead struct {
	Status    string
	ExpiresAt time.Time
	Body      []byte // nil unless collected and not expired
	SHA256    string
}

type PDFRead struct {
	Status    string // of the report
	ExpiresAt time.Time
	PDF       *ReportPDF // nil when the row is absent
	Body      []byte     // nil unless ready
}

// MarkReportDeleted sets deleted_at on a report not deleted and returns the
// status it had; ErrNotFound when absent or already deleted.
func MarkReportDeleted(ctx context.Context, tx pgx.Tx, id uuid.UUID, now time.Time) (status string, err error)

// SweepDeletedReports removes at most limit deleted reports whose status is
// not running, oldest mark first, skipping locked rows, with their PDF rows,
// snapshot, and idempotency row; it returns the ids removed.
func SweepDeletedReports(ctx context.Context, tx pgx.Tx, limit int) ([]uuid.UUID, error)
```

- `00004`: `deleted_at timestamptz` after `expired_at`; `ix_reports_listing` and `ix_reports_deleted` as R157; the Down unchanged in shape (the table drop removes them).
- `ClaimReport` matches `deleted_at IS NULL`; a refused claim of a deleted report returns `ErrNotFound`, which the collect handler already treats as an absent row (E148): it completes and writes nothing.
- `ReconcileReports`, `ReconcilePDFs`, and `ExpireDueReports` match `deleted_at IS NULL`.
- `needsRender` returns false for a report with `DeletedAt` set, with the log message `render skipped: report deleted`.
- `GetReport` and `GetReportForUpdate` still read deleted rows (the worker needs them); every read of the API goes through `GetReportItem`, `ListReports`, `ReadSnapshot`, `ReadPDF`.

- [ ] **Step 1: Write the failing tests**

| Test | Input | Expected |
|---|---|---|
| `TestListReportsNewestFirst` | five reports at distinct instants, one deleted | four, newest first, owner names and PDF rows filled |
| `TestListReportsKeysetOnEqualInstants` | 7 reports at one instant, pages of 3 | 3, 3, 1, ids descending, none repeated or skipped |
| `TestListReportsCursorOfADeletedRow` | page 1 ends at a report then deleted | page 2 starts after it |
| `TestListReportsUsesTheIndex` | `EXPLAIN` of the list query over 10 000 rows after `ANALYZE` | an index scan of `ix_reports_listing`, no sort node |
| `TestGetReportItemHidesDeleted` | a deleted report | `ErrNotFound` |
| `TestReadBodiesAreOneStatement` | a reader in a loop and a writer that expires and purges the report in another connection, 200 rounds | every read is a whole body with its SHA-256, or a state without a body, or `ErrNotFound`; never a body whose SHA-256 differs |
| `TestReadSnapshotStates` | queued; collected; expired (`expires_at <= now`); deleted | no body with status queued; body; no body with `ExpiresAt` in the past; `ErrNotFound` |
| `TestReadPDFStates` | row absent; pending; failed with code; ready; deleted | `PDF` nil; pending without body; failed with code; body; `ErrNotFound` |
| `TestMarkReportDeletedOnce` | mark twice | status then `ErrNotFound` |
| `TestMarkReportDeletedConcurrent` | 20 goroutines mark one report | exactly one returns a status |
| `TestClaimRefusesADeletedReport` | a queued report marked deleted, then a claim | `ErrNotFound`; the row unchanged |
| `TestSweepSkipsRunning` | deleted reports: queued, running, collected with PDFs, failed | three removed with their rows, the running one kept |
| `TestSweepSkipsLocked` | a deleted report locked `FOR UPDATE` by another transaction | not removed, no wait |
| `TestReconcileAndExpireLeaveDeletedAlone` | a deleted queued report whose job is gone; a deleted collected report past `expires_at` | reconcile and expiry touch neither |
| `TestNeedsRenderSkipsADeletedReport` | a collected report marked deleted with a pending row | the render completes its job and writes nothing |

- [ ] **Step 2:** Run `go test -count=1 -tags=postgres ./internal/platform/store/ -v -run 'List|GetReportItem|Read|Mark|Claim|Sweep|LeaveDeleted'` and the worker test; expected FAIL.
- [ ] **Step 3:** Implement. Show `TestMarkReportDeletedConcurrent` red under the mutation that drops `AND deleted_at IS NULL` from the mark.
- [ ] **Step 4:** After the migration, every package with tagged tests runs: `go test -count=1 -tags=postgres ./internal/platform/... ./internal/cli/...` and `go test -count=1 -tags='postgres template' ./internal/platform/worker/`.
- [ ] **Step 5:** Commit `feat(store): list, read, mark, and sweep reports` (47 characters).

**Done when:** every row passes and the tagged suites of every package pass.

---

### Task 6: The reports service: reads, delete, and the sweep (R150, R151, R152, R153, R156)

**Executor:** Subagent, against the running server. Opus reviews (delete under concurrency, the sweep).

**Files:**
- Create: `internal/platform/reports/read.go`, `delete.go` and tests
- Modify: `internal/platform/reports/request.go` (a replay of a deleted report), `service.go` (`RetryPDF` on a deleted report; the actions), `internal/platform/worker/maintenance.go` (the sweep step; the cause of the purge)

**Interfaces:**

```go
package reports

const ActionReportDelete = "report.delete"

// Summary is a report as the list and the detail show it.
type Summary struct {
	ID            uuid.UUID
	OwnerID       uuid.UUID
	OwnerUsername string
	ProfileID     string
	Status, Phase string
	FailureCode   *string
	PDFs          []PDFState
	CreatedAt     time.Time
	CompletedAt   *time.Time
	ExpiresAt     time.Time
	Expired       bool
}

type PDFState struct {
	Language    string
	Status      string
	FailureCode *string
}

// Detail is Summary with the request and the progress.
type Detail struct {
	Summary
	ProfileRevisionID uuid.UUID
	Attempt           int
	StartedAt         *time.Time
	CollectedAt       *time.Time
	Selection         Resolved
}

type Page struct {
	Items      []Summary
	NextCursor *string
}

// Download is a body to serve with its strong tag.
type Download struct {
	Body   []byte
	SHA256 string
}

func (s *Service) List(ctx context.Context, actor auth.Principal, cursor string, limit int) (Page, error)
func (s *Service) Get(ctx context.Context, actor auth.Principal, id uuid.UUID) (Detail, error)
func (s *Service) Snapshot(ctx context.Context, actor auth.Principal, id uuid.UUID) (Download, error)
func (s *Service) PDF(ctx context.Context, actor auth.Principal, id uuid.UUID, language string) (Download, error)
func (s *Service) Delete(ctx context.Context, actor auth.Principal, id uuid.UUID) error

type DeleteOutcome string

const (
	OutcomeDeleted   DeleteOutcome = "deleted"
	OutcomeNotFound  DeleteOutcome = "not_found"
	OutcomeExpired   DeleteOutcome = "expired"
	OutcomeForbidden DeleteOutcome = "forbidden"
)

type DeleteResult struct {
	ID      uuid.UUID
	Outcome DeleteOutcome
}

// DeleteMany marks up to 200 distinct reports in one transaction and
// answers one outcome per id, in the order given.
func (s *Service) DeleteMany(ctx context.Context, actor auth.Principal, ids []uuid.UUID) ([]DeleteResult, error)
```

Codes, as `*reports.Error`: `CURSOR_INVALID`, `REPORT_NOT_FOUND`, `REPORT_EXPIRED`, `SNAPSHOT_NOT_READY`, `PDF_NOT_READY` (fields `status`, and `failure_code` when set), `REQUEST_INVALID` (`{"language":"invalid"}`; `{"limit":"invalid"}` outside 1 to 200; `{"ids":"empty" | "too_many" | "repeated"}`); `auth.ErrForbidden` for a role or an operator's foreign report. `Get`, `Snapshot`, and `PDF` judge in this order: absent or deleted (404), expired at `s.now()` (410), not ready (409). `Delete` judges absent or deleted, then the role and the owner (403), then expired (410), then marks and audits; `DeleteMany` checks the role first (a reader gets `auth.ErrForbidden` for the whole call), then each id as `Delete` does with outcomes in place of errors, one transaction, one `now`. A replay in `Request` whose report is deleted is `REPORT_NOT_FOUND`. `RetryPDF` on a deleted report is `REPORT_NOT_FOUND`. The maintenance job runs `sweep` after `reconcile` and before `expire`, batch `reportBatch`, one `report.purge` per id with `{"cause":"deleted"}`; `purge` writes `{"cause":"expired"}`.

- [ ] **Step 1: Write the failing tests**

| Test | Input | Expected |
|---|---|---|
| `TestListPagesWithTheCursor` | 5 reports, limit 2 | pages of 2, 2, 1; `NextCursor` nil on the last |
| `TestListRefusesABadCursor` | `"!!"`; base64 of `"x"`; a valid instant with a bad id; a cursor whose instant has no zone | `CURSOR_INVALID` each |
| `TestListRefusesALimit` | 0; 201 | `REQUEST_INVALID {"limit":"invalid"}` |
| `TestListMarksExpired` | a report past `expires_at`, not purged | listed with `Expired` true |
| `TestReadsByEveryRole` | a reader, an operator, an administrator read a report another operator requested | `Get`, `Snapshot`, `PDF` succeed for each |
| `TestReadsJudgeInOrder` | absent; deleted; expired and deleted; expired; queued; failed | 404; 404; 404; 410; 409 `SNAPSHOT_NOT_READY`; 409 |
| `TestPDFNotReadyCarriesTheRow` | pending; failed `PDF_COMPILE_FAILED`; absent row of a collected report | `PDF_NOT_READY {"status":"pending"}`; `{"status":"failed","failure_code":"PDF_COMPILE_FAILED"}`; `{"status":"pending"}` |
| `TestDeleteByRole` | reader; operator on its own; operator on another's; administrator on another's | 403; marked; 403; marked |
| `TestDeleteJudgesInOrder` | absent; already deleted; expired (own) | 404; 404; 410, nothing written |
| `TestDeleteAudits` | an operator deletes its queued report | one `report.delete` row, actor the operator, `{"status":"queued"}` |
| `TestDeleteManyOutcomes` | an operator sends 200 ids: own, another's, expired own, absent, already deleted | outcomes in order: `deleted`, `forbidden`, `expired`, `not_found`, `not_found`; one audit row per `deleted` |
| `TestDeleteManyRefusesTheBody` | empty; 201 ids; a repeated id | `REQUEST_INVALID` with `empty`, `too_many`, `repeated`; nothing written |
| `TestDeleteManyConcurrent` | two administrators send the same 50 ids at once | every id marked once; 50 `report.delete` rows in all; each id `deleted` in exactly one answer and `not_found` in the other |
| `TestReplayOfADeletedReport` | a request, the report deleted, the same key and body again | `REPORT_NOT_FOUND`; no new report |
| `TestRetryOfADeletedReport` | a collected report deleted, then `RetryPDF` | `REPORT_NOT_FOUND` |
| `TestSweepStepRemovesDeleted` | maintenance run over deleted reports queued, failed, collected, and running | three removed with `report.purge {"cause":"deleted"}`; the running one kept; a second run after it fails removes it |
| `TestDeleteWhileRunningIsSweptAfter` | the collect hook `beforeCollected` deletes the report through the service; the attempt finishes | the report is `collected` and deleted; both render jobs complete writing nothing; the next maintenance run sweeps it |
| `TestPurgeNamesItsCause` | an expired report purged | `report.purge {"cause":"expired"}` |

- [ ] **Step 2:** Run `go test -count=1 -tags=postgres ./internal/platform/reports/ -v` and the worker tests; expected FAIL.
- [ ] **Step 3:** Implement. Show `TestDeleteManyConcurrent` red under the mutation that reads `deleted_at` in a separate statement before the mark.
- [ ] **Step 4:** `go test -count=1 -tags=postgres ./internal/platform/... ./internal/cli/...`; `go test -count=1 -tags='postgres template' ./internal/platform/worker/`; `make test`; `make lint`.
- [ ] **Step 5:** Commit `feat(reports): read, delete, and sweep reports` (45 characters).

**Done when:** every row passes; `TestReportEndToEnd` of 3.3 still passes with its audit order and the cause of the purge.

---

### Task 7: The report view (R160)

**Executor:** Main thread for Step 1 (two catalog entries); subagent for the rest. May run beside Task 5.

**Files:**
- Create: `internal/document/view.go`, `view_test.go`
- Modify: `internal/catalog/keys.go`, `internal/catalog/locales/en.toml`, `pt-BR.toml`

**Interfaces:**

```go
package document

// BuildView words a snapshot in the catalog's language for the web,
// narrowed as the PDF of the same report is.
func BuildView(s snapshot.Snapshot, c *catalog.Catalog) (View, error)

// EncodeView writes the view as JSON without HTML escaping.
func EncodeView(v View) ([]byte, error)
```

The shape (every list never nil; `Value` is `{"raw": number | null, "text": string}`; `Statistic` is `{"state", "average", "minimum", "peak", "current"}` of `Value`; the text of a missing number is the placeholder the PDF prints for that state):

| Key | Content |
|---|---|
| `language` | `pt-BR` or `en` |
| `min_severity` | the effective minimum |
| `report` | `id`, `cluster_name`, `profile`, `timezone`, `generator_version`, `namespaces` (list), `emitted_at` (stored text), `emitted_at_text` (`Catalog.DateTime`), `period_start`, `period_end`, `period_text` (as the opening page) |
| `summary` | `namespaces`, `workloads`, `hpas`, `nodes`, `nodepools`, `findings`, `recommendations`, `healthy`, `attention`, `critical`, `workloads_without_hpa` as `Value`; `autoscaling`: `status`, `source`, `confidence` codes and `managed_node_groups`, `current_nodes`, `min_nodes`, `max_nodes`, `scale_events`, `unschedulable_pods` as `Value` |
| `findings` | `code`, `severity`, `scope_type`, `scope_kind`, `scope_name`, `confidence`, `sentence` (the summary entry of the code), `action` (its action entry), `evidence`: `[{name, raw, text}]` where `raw` is the stored number, boolean, or text, and `text` is `Catalog.Percent` for a name ending in `percent`, `Catalog.Count` for another whole number, `Catalog.Decimal(v, 2)` for another number, `value.yes`/`value.no` for a boolean, the text as stored for a text |
| `recommendations` | `code`, `severity`, `scope_type`, `confidence`, `action` (as the action plan prints it, without the scope suffix), `scopes` (list), `scope_text` (the suffix the action plan prints) |
| `namespaces` | `name`, `status`, `pods` (`Value`), `approximate`, and the ten statistics of the snapshot row under their snapshot names, each a `Statistic` |
| `workloads` | `namespace`, `kind`, `name`, `status`, `observed_usage`, `hpa_coverage`, `pods`, `approximate`, the ten statistics, `reason` (the reason column of the PDF, empty when it prints none) |
| `hpas` | `namespace`, `name`, `target_kind`, `target_name`, `spec_min`, `spec_max` (`Value`), `current`, `desired` (`Statistic`), `situation` (the situation cell of the PDF) |
| `nodes` | `name`, `pool`, `pool_confidence`, `status`, `cpu_capacity`, `memory_capacity`, `cpu_allocatable`, `memory_allocatable`, `cpu_request_percent`, `memory_request_percent` (`Value`), `cpu_usage_percent`, `memory_usage_percent` (`Statistic`), `ready`, `schedulable` (boolean or null), `reason` (the reason cell of the PDF) |
| `nodepools` | `name`, `confidence`, `nodes` (list) |
| `quality` | `capabilities`: `[{name, state, strategy, evidence (list), remark}]` where `remark` is the appendix's remark; `warnings`, `notices`: `[{code, text}]` with the appendix's and the notes' texts; `collection_started_at`, `collection_completed_at` |

A row's `status` is the severity the PDF gives its scope (`scopeSeverity`), `healthy` when it has no finding.

- [ ] **Step 1 (main thread):** the entries `value.yes` ("yes" / "sim") and `value.no` ("no" / "não") in both languages, with their key constants; commit with the view (they are used only by it).
- [ ] **Step 2: Write the failing tests**

| Test | Input | Expected |
|---|---|---|
| `TestViewSaysWhatThePDFSays` | every golden snapshot of `testdata/snapshot/`, both languages | for every namespace, workload, HPA, and node the PDF prints, the text of each view value equals the text of the PDF's cell for the same quantity (the cells read from the `Document` that `Build` makes for the same snapshot); the reason and situation texts equal |
| `TestViewNarrowsAsThePDF` | the `edge-scaling` snapshot at `all`, `attention`, `critical`, and with the executive profile at `all` | the rows and findings kept are those the PDF keeps; the executive view keeps nothing below attention |
| `TestViewKeepsTheWholeOfTheRest` | as above | HPAs, node pools, summary counts, and quality equal at every minimum |
| `TestViewFindingTexts` | every code of the registry in a hand-made snapshot | a non-empty `sentence` and `action` in both languages; evidence texts by the rule of the shape |
| `TestViewMissingNumbers` | `edge-values` | every absent number has `raw` null and the placeholder the PDF prints |
| `TestViewIsDeterministic` | one snapshot built twice | equal bytes |
| `TestViewRefusesWhatCheckRefuses` | a snapshot that `Check` refuses | the same error |

- [ ] **Step 3:** Run `go test ./internal/document/ -run View -v`; expected FAIL.
- [ ] **Step 4:** Implement with the helpers of the package; no text is written twice. The catalog guards pass (`go test ./internal/catalog/`).
- [ ] **Step 5:** `make test`; `make lint`; commit `feat(document): word a snapshot for the web` (43 characters).

**Done when:** every row passes and the reviewer has read every string literal of `view.go`.

---

### Task 8: The report routes, the contract, and `serve` (R149 to R155, R158, R159, R161, R162)

**Executor:** Main thread for Step 1 (the contract); subagent for the rest, against the running server. Opus reviews (authorization before the conditional answers).

**Files:**
- Modify: `api/openapi.yaml`, `api/oapi-codegen.yaml`, `internal/platform/api/openapi.gen.go` (Step 1)
- Create: `internal/platform/api/reports.go`, `reports_test.go`, `view_contract_test.go`
- Modify: `internal/platform/api/errors.go`, `routes.go`, `server.go`, `contract_test.go` if needed; `internal/cli/serve.go`, `serve_wiring_test.go`

- [ ] **Step 1 (main thread):** write the nine operations of R149 in `api/openapi.yaml` (`requestReport`, `listReports`, `getReport`, `viewReport`, `getReportSnapshot`, `getReportPDF`, `retryReportPDF`, `deleteReport`, `deleteReports`), the parameters `ReportID`, `Lang`, `Cursor`, `Limit`, `IdempotencyKey`, the schemas `ReportRequest`, `ReportAccepted`, `ReportSummary`, `Report`, `ReportPage`, `ReportPDFState`, `ReportSelection`, `ReportStatus`, `ReportPhase`, `ReportFailureCode`, `PDFStatus`, `PDFFailureCode`, `PDFRetryRequest`, `PDFRetryAccepted`, `ReportDeleteRequest`, `ReportDeleteResults`, and `ReportView` with its parts (excluded from generation in `api/oapi-codegen.yaml`), every code of R154, `ETag`, `Cache-Control`, `Content-Disposition`, and `Retry-After` headers, and 304 on the three downloads; `make generate`; commit `docs(api): describe the report routes` (38 characters). `TestRoutesMatchTheContract` is red from this commit until Step 4, the one expected red window of the part.
- [ ] **Step 2: Write the failing tests**

| Test | Input | Expected |
|---|---|---|
| `TestReportRoutesCheckInOrder` | each route: no session; a reader on a write; a write without CSRF; a bad body or query | 401; 403; 403 `CSRF_REJECTED`; 400 |
| `TestRequestReportRoute` | an operator posts a valid body with a key; the same again; the same key with another body; no key; `bad key!` | 202 `replayed` false; 202 `replayed` true with the same id; 409 `IDEMPOTENCY_KEY_REUSED`; 400 `IDEMPOTENCY_KEY_INVALID`; 400 `IDEMPOTENCY_KEY_INVALID` |
| `TestAdmissionAnswers429` | `admission_limit` 1, `admission_retry_seconds` 5, two requests | 202 then 429 `ADMISSION_GLOBAL_LIMIT` with `Retry-After: 5` |
| `TestSelectionRefused` | a namespace the profile does not allow | 422 `REPORT_SELECTION_NOT_PERMITTED` with the service's fields |
| `TestListRoutePages` | 5 reports, `limit=2`, following `next_cursor`; `cursor=!!`; `limit=0` | 2, 2, 1 then `next_cursor` null; 400 `CURSOR_INVALID`; 400 `REQUEST_INVALID` |
| `TestReportRouteStatuses` | detail of absent, deleted, expired, queued | 404, 404, 410, 200 with the shape of `Report` |
| `TestViewRoute` | `lang=en`; `lang=pt-BR`; no `lang`; `lang=fr`; a queued report | 200 with `language` en and pt-BR; 400 `{"lang":"missing"}`; 400 `{"lang":"invalid"}`; 409 `SNAPSHOT_NOT_READY` |
| `TestDownloadsCarryTheirTags` | snapshot; PDF en | 200, `ETag` `"<stored sha256>"`, `Cache-Control: private, no-cache`, the `Content-Disposition` of R155, the stored bytes |
| `TestConditionalAnswersAfterAuthorization` | `If-None-Match` with the tag; `W/"<tag>"`; `"x", "<tag>"`; `*`; the tag on an expired report; the tag on a deleted report; no session | 304 ×4 with `ETag` and `Cache-Control`; 410; 404; 401 |
| `TestPDFNotReadyRoute` | a pending row | 409 `PDF_NOT_READY {"status":"pending"}` |
| `TestRetryRoute` | a failed row; a ready row; `{"language":"fr"}` | 202 `{"language":"en","status":"pending"}`; 409 `PDF_RETRY_NOT_ALLOWED`; 400 |
| `TestDeleteRoute` | own; another's by an operator; expired; absent | 204; 403; 410; 404 |
| `TestBulkDeleteRoute` | the mix of Review Focus 2; 201 ids; a repeated id | 200 with outcomes in order; 400; 400 |
| `TestDownloadRacesTheSweep` | GET of the PDF in a loop while the report is deleted and swept | each answer a whole 200 with the stored length, or 404; never 500 |
| `TestErrorTableNamesEveryCode` | the table of `errors.go` | holds every code the reports service returns, read from its constants |
| `TestViewMatchesTheContract` | every golden snapshot, both languages | the view's JSON walks the schema `ReportView` both ways (R161) |
| `TestServeBuildsTheReportRoutes` (`internal/cli`, `postgres`) | the handler of `buildServer`; an operator posts a request | 202, and a River `collect` job inserted for the report |

- [ ] **Step 3:** Run the tests of `internal/platform/api` and `internal/cli` with `-tags=postgres`; expected FAIL.
- [ ] **Step 4:** Implement. `Server` gains `Reports *reports.Service`; a nil `Reports` answers 500 `INTERNAL_ERROR` and is never nil in `serve`. `buildServer` returns `(*api.Server, error)`; `runServe` exits 1 when it fails. The routes read `lang` and `limit` with no unknown query parameter refused (a query is not a body). `Retry-After` is `ceil(RetryAfter.Seconds())`, at least 1.
- [ ] **Step 5:** Run the binary by hand: `serve` against the database of `make postgres-up`, a bootstrapped administrator, an operator, an imported profile; `curl` each route once, a 304 included; write the transcript to `.cache/phase-3/part-4/manual/routes.md`.
- [ ] **Step 6:** `make test`; `make lint`; `make generate-check`; `go test -count=1 -tags=postgres ./internal/platform/... ./internal/cli/...`.
- [ ] **Step 7:** Commit `feat(api): add the report routes` (32 characters).

**Done when:** every row passes, the contract test is green again, and the transcript shows every route answering as the contract says.

---

### Task 9: The test minors of 3.3 (R148; F-125)

**Executor:** Subagent. May run beside Task 8.

The 26 minors of `ledger-minors-and-rulings.md`, numbered in its order of appearance; the render minors are in Task 1, the drift and Makefile minors in Task 3.

| # | Minor | Ruling |
|---|---|---|
| 1 | `preflightRequests = 29` hard-coded | Fix: derive the count from the recorded probes of the scenario, with a comment naming where they come from |
| 2 | `newRun` copies goldentest's setup | Accept: the golden equality guards the copy |
| 3 | `imports.go` lists render (100) before goldentest (95) | Fix: order the table by rank |
| 4 | `render.MaxPDFBytes` not enforced in `Compile` | Accept: the worker enforces it; the command holds no cap by R136 |
| 5 | `LatestProfileRevision` read twice on the recreate path | Accept: one extra read of one row on an operator command |
| 6 | `createStep`'s bool parameter | Accept: one internal function with two callers |
| 7 | `TestSourceReadsTheReferences` checks `Hostnames != nil` | Fix: compare the whole `Policy` |
| 8 | `TestTestCeiling` uses the real clock | Fix: the fake clock of the brief |
| 9 | admin's `Reveal` wiring has no test | Accept: no admin path reads a credential |
| 10 | range-before-cross-key order implicit | Accept: both messages are true |
| 11 | worker range loop before the secret path checks | Accept: both messages are true |
| 12 | `TestDecodeRefusesTextThatIsNotUTF8` pins "after offset 412" | Fix: assert the message's form and that the offset is the first invalid byte of the file |
| 13 | `repeatedKeyPath` renders a numeric key as `[1]` | Fix: a numeric object key is a key (`.1`), an index alone is `[1]` |
| 14 | `shapePaths` ignores `supportedKey`'s ok | Fix: a field with no tag fails the hash test |
| 15 | two `schema_version` keys report the version | Accept: the version is named first by the rule of `AGENTS.md` |
| 16 | the drift test misses text outside blocks | Fixed in Task 3 |
| 17 | the `RetryPDF` rollback test uses a principal with no user row | Accept: a device that stands for any later failure |
| 18 | the negative control subtest has no comment | Fix: say it is a negative control |
| 19 | `TestCollectReadsTheClockForTheRun` tied to four reads | Accept: it pins the rule of the clock reads |
| 20 | `test-postgres` runs the worker's postgres tests twice | Fixed in Task 3 |
| 21 | no test of a report expired between the render transactions | Fixed in Task 1 |
| 22 | no render test of `INTERNAL_FAILURE` with attempts left | Fixed in Task 1 |
| 23 | `needsRender`'s skips share one message | Fixed in Task 1 |
| 24 | doc comments of `retention.go` wrapped irregularly | Fix: rewrap at the width of the package |
| 25 | `TestWorkerStopAbandonsAHungJob` waits 200 ms | Fix: poll the readiness flag until a deadline |
| 26 | the skip of `pool.Close` has no test | Accept: it needs a handler hung on a connection |

- [ ] **Step 1:** For each "Fix" of this task (1, 3, 7, 8, 12, 13, 14, 18, 24, 25): the change and, for a test that is strengthened, the mutation that it now catches and the old test did not, named in the report.
- [ ] **Step 2:** `make test`; `make lint`; the tagged suites of the packages touched.
- [ ] **Step 3:** Commit `test: settle the minors of the worker part` (42 characters).

**Done when:** the ten fixes are in, each strengthened test shown red under its mutation.

---

### Task 10: The proof through the API (R163)

**Executor:** Subagent for Step 1, against the running server; main thread for Step 2.

- [ ] **Step 1:** `TestReportThroughTheAPI` in `internal/cli/api_end_to_end_test.go` (`//go:build postgres && template`), as R163: a settable clock; the snapshot body equals `snapshot.Encode` of `pipeline.Collect` with the same inputs on a fresh fake server; each PDF equals `render.Compile` of `render.Document` of that snapshot; the view in each language equals `document.EncodeView` of `document.BuildView` of that snapshot; a second GET of each download with its tag answers 304; the operator deletes the report, the detail answers 404, and one maintenance run sweeps it; the audit rows in order are `report.submit`, `report.collect`, `report.render` twice, `report.delete`, `report.purge`. Commit `test(cli): prove a report through the API` (40 characters).
- [ ] **Step 2 (main thread):** run `serve` and `worker` by hand against the fake Prometheus, request a report with `curl`, wait for it, download both PDFs and open them, and record the transcript in `.cache/phase-3/part-4/manual/end-to-end.md`.

**Done when:** the test passes in `make test-postgres` and the PDFs downloaded by hand open.

---

### Task 11: The documents, review, gate, close

**Executor:** Main thread.

- [ ] **Step 1:** `AGENTS.md`: "The platform" gains the report routes and their order of checks, the rule of R153 (404, 410), the delete and the sweep (R156), the conditional answers after authorization (R155), the view in `internal/document` beside the data file (R160) and its contract test (R161), the redacting handler (R144) and the logger scan (R146); "The worker and the reports" gains the success write's budget (R143) and the sweep step; the commands list and the tag table checked against the Makefile.
- [ ] **Step 2:** `README.md`: the report routes, what each role may do, the 429, the 410 window.
- [ ] **Step 3:** `CHANGELOG.md` under Unreleased. `### Added`: an operator requests a report through the API with an idempotency key and is refused with a wait time over the admission limits; every signed-in reader lists the reports by page, reads one, opens it worded in Portuguese or English, and downloads its snapshot and its PDFs; an operator retries the PDF of one language; an operator deletes its own reports and an administrator any, one or up to 200 at once; a report expires after its retention and answers that it expired until it is removed; each report is collected, rendered in both languages, and expired by the worker (the items 3.3 left for this part). `### Changed`: the redaction of a `token=` value keeps the quote or brace that closes it. `### Fixed`: a slow database write after a finished collection no longer fails the report as an unreachable source. Omitted and said so in the report: the redacting handler's new reach (no production line logs those values), the drift test, the test minors, the Makefile split, the OrbStack instructions.
- [ ] **Step 4:** the adversarial review of the branch by areas, with probes under `.cache/phase-3/part-4/review/probe/`: (a) authorization, roles, and the conditional answers; (b) delete, the sweep, and their races with the worker; (c) the view against the PDF and the contract; (d) redaction and F-118. One fix wave; one re-review.
- [ ] **Step 5:** `make gate` on OrbStack; logs to `.cache/phase-3/part-4/gate/`; read that `TestReportThroughTheAPI`, `TestReportEndToEnd`, and `TestViewMatchesTheContract` are named in the logs.
- [ ] **Step 6:** `ROADMAP.md` updated, F-126 closed; the closing report `.cache/phase-3/part-4/closing-report.md`; the owner's word on the merge by fast-forward, the push, and the version.

**Done when:** the gate passes from images built without cache, the review is clean, and the owner has the closing report.
