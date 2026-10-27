# Phase 3.3: River, the Worker, Storage, Retention, and the Audit of Reports Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** An operator's report request, made through a service, is stored with the profile revision in force, an idempotency key, and an admission check, and is queued in the same transaction; the `worker` command runs it through the shared preflight and collectors behind the outbound policy, stores the snapshot, compiles one PDF per language from it, and expires the report after its retention; the snapshot's shape is frozen at `schema_version` 1.0 and the reader refuses what `Build` never writes; the deferred findings F-41, F-89, F-93, F-102, F-109, F-115, F-116, and F-117 close.

**Architecture:** Two core packages are cut from the `report` command so the command and the worker share them: `internal/pipeline` (preflight, collectors, analysis, snapshot) and `internal/render` (document and compile, PDF bytes). Three platform packages join: `internal/platform/health` (the two health routes, shared by `serve` and `worker`), `internal/platform/reports` (the request, the PDF retry, the job arguments), and `internal/platform/worker` (the River client and the three job handlers: collect, render, maintenance). River owns queuing, leasing, retries, and the periodic schedule; its schema is a goose migration applied under the lock of 3.1. Every handler writes the report row and completes its River job in one transaction, and every write of a running report is a compare-and-set on the attempt.

**Tech Stack:** Go 1.27; River v0.49.0 (`github.com/riverqueue/river`, `riverdriver`, `riverdriver/riverpgxv5`, `rivershared`, `rivertype`, all pinned to the same version) over the existing pgx v5.11.0 pool; `encoding/json/v2` of the standard library for the duplicate-key refusal; PostgreSQL 18 through the harness of 3.1; Typst 0.15.1; the fake Prometheus of `internal/promtest`.

**Spec:** `docs/superpowers/specs/2026-09-27-k8s-capacity-report-design.md`, sections 3, 4.2, 4.3, 8, 11.1, 11.2, 11.7, 11.8, 11.9, 13.2, 15. The inventory of the original is `docs/reference/inventory/platform.md`. The research of this part, read in the session that wrote the plan, is under `.cache/phase-3/part-3/research/`: `original-worker.md` (the original's queue, states, storage, retention, admission, idempotency, audit, with file and line), `current-code.md` (the code of this repository at `6303688`, with file and line), `river.md` (River v0.49.0, with module-cache file and line, behaviour verified against PostgreSQL 18.3). The decisions of the owner are in `ROADMAP.md`.

**Depends on:** Part 3.2, merged into `main` at `6303688` and pushed on 2026-10-06.

## Where this part sits

| Part | Delivers | Proof at the end |
|---|---|---|
| 3.1, merged | Settings, store, audit, accounts, sessions, the API skeleton, `serve`, `admin` | An administrator bootstrapped from the command line signs in and creates an operator |
| 3.2, merged | Profiles and revisions, the reference files, the outbound policy, the profile routes and test, `admin import-profiles` | A profile created through the API is tested against the fake Prometheus admitted by the policy |
| **3.3 River, worker, storage, retention, audit (this plan)** | The shared pipeline and render, the snapshot frozen, the report tables and River's, the reports service (request, retry), the collect, render, and maintenance jobs, the `worker` command, the shared health routes; F-41, F-89, F-93, F-102, F-109, F-115, F-116, F-117 | A report requested through the service is collected by the worker against the fake Prometheus, its snapshot equals what the pipeline writes for the same inputs, both PDFs equal what the render writes for that snapshot, and the report expires and is purged on the clock |
| 3.4 API, report view, OpenAPI contract | The report routes, the report view, pagination, ETags, delete, the 429 and the 410 answers, the reports service wired into `serve` | A report requested and downloaded through the API, as spec 16 asks |

### Out of this part

| Left out | Arrives in | Reason |
|---|---|---|
| Every report route of spec 11.10, the report view (11.11), list, read of a snapshot or a PDF, ETags, the mapping of the service's codes to statuses and sentences | 3.4 | The owner's split of 2026-10-05: routes in 3.4 |
| Delete of a report (`DELETE /reports/{id}`, `POST /reports/delete`) and its sweep | 3.4 | It is a route; the maintenance job of this part gains the sweep with it |
| The reports service built by `serve` | 3.4 | No route of this part reads it; 3.4 wires it with a test on the handler `serve` builds (the rule of E127) |
| A Compose file of the platform (`serve`, `worker`, `postgres`) | 5 | Spec 14.2; the operations guide of the release |
| Running limits (per profile or global) apart from the slots | none | Spec 11.7 names admission limits only; the slots bound what runs (R123) |
| An administrator-editable settings row (the original's `platform_settings` and `[overrides]`) | none | Spec 11.2 puts every value of this part in the deployment file (R133) |

## How to read this plan

The owner's orchestration rule applies: the plan states what each task must deliver and how it is proven, and the implementer writes the code. Each task gives exact paths, exact signatures, and a table of tests with inputs and expected results. Write each test first, watch it fail, then implement.

Executors:

- **Subagent**: a Sonnet subagent. It may run `go`, `make build`, `make test`, `make lint`, `make generate`, `make generate-check`, and the tagged tests against the PostgreSQL the main thread started (`KCR_TEST_DATABASE_DSN`), with `typst` and `pdftotext` installed natively for the tag `template`. It never starts or stops Docker, never runs `make update-*`, and cannot declare a task finished.
- **Main thread**: needs Docker, judgment, API design, wording, or the owner.

Rules for every subagent brief, as in the plans of 3.1 and 3.2: tasks in sequence unless this plan says they may run side by side; a reviewer reads a diff file and runs tests only on packages nobody edits; `git add <paths>`; no stash, restore, checkout of a path, or clean; a test of a change goes red first, test by test, on the code before the change; a brief that says "report every X" lists what must not match; a decision flagged at the end of a report has already been made; the main thread commits before dispatching and computes before dispatching. Two rules learned in 3.2 bind every task of this part: **after a migration, every package with tagged tests runs** (`make test-postgres` over `./internal/platform/... ./internal/cli/...`, not the task's package), and **a service is proven through what the command builds** (Task 11 tests the River client `worker` builds, not one the test assembles).

Order: Tasks 1, 2, and 3 touch no common file and may run side by side, each in its own worktree. Task 4 follows Task 3 (both edit `internal/platform/settings/settings.go`). Tasks 5 to 12 run in sequence: they share the store, the migrations, and the worker package.

Reference material, read-only and never tracked:

- `older/kubernetes-report/kube_capacity_report/platform/`: `queue.py`, `worker.py`, `execution.py`, `jobs.py`, `retention.py`, `repositories.py`, `worker_health.py`, `settings.py`; `older/kubernetes-report/alembic/versions/20260904_0001_platform.py`.
- `.cache/phase-3/part-3/research/original-worker.md`: the facts of the original with file and line. A brief quotes the lines it needs; an implementer that finds the brief and the original in disagreement stops and reports.
- `.cache/phase-3/part-3/research/river.md`: the River facts. The verification programs are under `.cache/phase-3/part-3/pg-verify/`.

## Global Constraints

Every constraint of the plans of phases 0 to 3.2 stands. The ones below are added or are the ones this part leans on most.

- New modules: the five River modules at v0.49.0 and what they require (the four `github.com/tidwall/*` modules; `golang.org/x/text` to v0.42.0 and `golang.org/x/sync` to v0.23.0). No `riverdatabasesql` (it requires `lib/pq`), no `rivertest` (it requires testify, goleak, and a second YAML module). The guard of forbidden modules passes over the new graph.
- The core never imports `internal/platform/`. `internal/pipeline` ranks 75 and `internal/render` 100; `internal/platform/health` 128, `internal/platform/reports` 146, `internal/platform/worker` 160 (R136).
- **Every timestamp of this project's tables is passed from Go.** River's own tables use `now()` in their SQL and their defaults; that is River's, and it is the one exception, written in `AGENTS.md` (R120).
- **A snapshot is stored as the bytes `snapshot.Encode` wrote**, in `bytea`, with its SHA-256 in lower-case hex and its size; never as `jsonb` (R127). A PDF likewise.
- **No stored text carries a failure's words.** A report or a PDF row stores a code from the table of R125; a River job's error is `"<kind>: <CODE>"`; a log line carries the cause through the run's redactor; a credential read by the worker or by the profile test is added to the redactor before it is used (R141).
- Every write of a running report names the attempt it was claimed with, and writes nothing when the attempt differs (R124). Every handler completes its River job with `river.JobCompleteTx` in the transaction that writes the outcome.
- The checks of a future route keep the order session, role, CSRF, body; the service of this part checks the role it is given.
- No test skips. Tagged tests carry `//go:build postgres`, and a test that also needs Typst carries `//go:build postgres && template`. Comments never name a task, plan, or phase. Diagnostics in English. Every literal ASCII.
- Scratch under `.cache/phase-3/part-3/`. Commits `type(scope): description`, imperative, under 50 characters; each subject below is counted.

## Review Focus

Each line names an input the spec implies and does not describe, and the task whose tests pin it.

1. **A hung attempt that wakes after River rescued its job and a second attempt claimed it.** The old attempt writes nothing to the report, the snapshot, or the PDF rows, and does not complete the job the second attempt holds. Pinned in Task 8 (`TestCollectStaleAttemptWritesNothing`).
2. **Both languages finishing in the same instant.** The report reaches phase `complete` exactly once, with both rows `ready`. Pinned in Task 9 (`TestRenderBothLanguagesCompleteOnce`, 20 runs).
3. **Twenty requests with one idempotency key and one body, sent at once.** One report, one River job, one audit row; the other nineteen answer the same report with `Replayed`. The same key with another body answers `IDEMPOTENCY_KEY_REUSED` and writes nothing. Pinned in Task 7 (`TestRequestSameKeyConcurrently`).
4. **Thirty requests at once against an admission limit of five.** Exactly five are accepted and twenty-five are refused with `ADMISSION_GLOBAL_LIMIT`. Pinned in Task 7 (`TestAdmissionConcurrentExactlyLimit`).
5. **A worker stopped in the middle of a collection whose drain is shorter than the job.** The job returns to River `available` with its attempt restored, the report is not failed, and the next attempt collects it. Pinned in Task 11 (`TestWorkerStopReturnsTheJob`).
6. **A job River gives up on.** A job rescued at its last attempt, a handler that panics on its last attempt, or a job the cleaner deleted leaves no handler to write the report. The maintenance job fails the report `JOB_ATTEMPTS_EXHAUSTED`, or the PDF row `PDF_ATTEMPTS_EXHAUSTED`, so no report stays `running` or `rendering` forever, counted by admission and never expired (R142). Pinned in Task 10 (`TestReconcile*`, `TestPanicAtTheLastAttemptIsReconciled`).
7. **An attempt that blocks before its claim and is rescued.** A late claim of attempt 1 after attempt 2 claimed the report is refused, so attempt 1 never owns the report again and never completes the job attempt 2 holds (R124). Pinned in Task 6 (`TestClaimRefusesAnOlderAttempt`) and Task 8 (`TestCollectLateClaimLoses`).

## Rulings made on the owner's behalf

Each is a decision the spec leaves open, written as decision, reason, and cost if wrong. The owner may overturn any of them when reviewing this plan. The numbering continues that of part 3.2.

| ID | Ruling |
|---|---|
| R119 | Ruling: River v0.49.0, its five modules pinned to the one version, the pgx driver over the existing `*pgxpool.Pool`; no `riverdatabasesql`, no `rivertest`; tests run a real client with `Config.TestOnly` against `storetest` — spec 11.7 names River; `river.md` section 1 — River is 0.x, and an upgrade is a task of its own with its migration file |
| R120 | Ruling: River's schema is the goose migration `00003_river.sql`, holding River's SQL of its versions 2 to 8 as `rivermigrate.New(riverpgxv5.New(nil), nil).AllVersions()` gives it, each version between `-- +goose StatementBegin` and `-- +goose StatementEnd`, and the Down in reverse; version 1 (`river_migration`) is left out, since goose keeps the one history; a test compares the file with the library, version by version; River's SQL calls `now()` and that exception is written in `AGENTS.md` — one migration history under the lock `72427301`; `river.md` section 2 measured Up, Down, Up on PostgreSQL 18.3 under that lock — a River upgrade that adds an enum value and uses it must be split in two goose files |
| R121 | Ruling: the schema of reports is `00004_reports.sql`: `reports` (`id uuid`, `owner_id` → users, `profile_id` → profiles, `profile_revision_id` → profile_revisions, `request jsonb` holding the resolved selection, `status`, `phase`, `attempt integer`, `failure_code text` null, `created_at`, `started_at`, `collected_at`, `completed_at`, `expires_at`, `expired_at`, `updated_at`), `report_snapshots` (`report_id` primary key, `schema_version`, `body bytea`, `sha256`, `size_bytes`, `created_at`; a trigger refuses `UPDATE`), `report_pdfs` (`report_id`, `language`, `status`, `failure_code`, `body bytea`, `sha256`, `size_bytes`, `updated_at`; primary key `(report_id, language)`), `report_idempotency_keys` (`owner_id`, `key`, `request_sha256`, `report_id`, `created_at`; primary key `(owner_id, key)`); every foreign key `ON DELETE RESTRICT`; checks quoted in Task 5 — spec 11.7, 11.8; the original's `jobs`, `snapshots`, `pdf_artifacts`, `idempotency_keys` without the lease columns River replaces and without the S3 columns — none |
| R122 | Ruling: `status` is `queued`, `running`, `collected`, `failed`; `phase` is `waiting`, `preflight`, `collecting`, `analyzing`, `snapshotting`, `rendering`, `complete`; a check binds them: `queued` with `waiting`; `running` with `preflight` to `snapshotting`; `collected` with `rendering` or `complete`; `failed` with `complete`; `failure_code` is set exactly when `failed`; `rendering` means a PDF row is `pending`, `complete` means none is — spec 11.7's three lines; the original's `succeeded` is the design's `collected`, its unused `uploading_pdf` goes — none |
| R123 | Ruling: three job kinds, `collect` (`{"report_id"}`), `render` (`{"report_id","language"}`), `maintenance` (`{}`); the queue `reports` runs `worker_slots` workers, a render job at priority 1 and a collect job at priority 2, so a collected report's PDFs pass before the next collection; the queue `maintenance` runs one worker; `maintenance` is a periodic job every minute, `RunOnStart`, enqueued by River's leader alone; `MaxAttempts` is `job_max_attempts`, set at insert through the job's insert options (River applies the inserting client's value, `river.md` section 3); River's default retry policy; `JobTimeout` per kind is `collect_timeout_seconds` and `render_timeout_seconds` (River's default of one minute would kill a collection); `RescueStuckJobsAfter` is the larger timeout plus one minute; the reindexer is off (`river.NeverSchedule()`); the cleaner keeps River's defaults; no unique options, since the report rows decide what runs — spec 11.7; the original alternated collection and PDF claims (`worker.py:426-472`) — a queue full of renders delays a collection by at most two compiles per collected report |
| R124 | Ruling (amended by the owner on 2026-10-06): a collect attempt claims the report by writing `status running`, `phase preflight`, `attempt = job.Attempt` in one transaction, from `queued`, or from `running` only when the stored attempt is at most `job.Attempt` (equal for the run after a soft stop, which River gives the same attempt number), so a late claim of an older attempt is `ErrStaleAttempt`; every later write of that report is `UPDATE ... WHERE id = $1 AND attempt = $2 AND status = 'running'`; a write that matches no row rolls its transaction back, completes nothing, and returns `worker.ErrStaleAttempt`; a render handler locks the report `FOR UPDATE` before it reads or writes a PDF row — `river.md` section 4: `JobCompleteTx` compares on `state = 'running'` only, and a rescued job is `running` again under its next attempt, so the attempt is the owner; the original's compare-and-owner (`repositories.py:1409-1500`) — a hung handler that ignores its context returns an error River records against the job after the next attempt finished, which shows in `river_job.errors` and changes nothing of the report |
| R125 | Ruling (amended by the owner on 2026-10-06): the codes a report stores are those of the profile test (R108) for the preflight and the references, plus `NOTHING_TO_REPORT` (`collect.ErrNoUsefulSections`), `SNAPSHOT_TOO_LARGE`, `COLLECTION_TIMEOUT` (the job's deadline), `JOB_ATTEMPTS_EXHAUSTED` (R142), and `INTERNAL_FAILURE`; the codes a PDF row stores are `PDF_COMPILE_FAILED` (the render deadline included), `PDF_TOO_LARGE`, `PDF_ATTEMPTS_EXHAUSTED` (R142), `INTERNAL_FAILURE`; only `ENDPOINT_UNREACHABLE` and `INTERNAL_FAILURE` are retried by River while attempts remain, and stored on the last; a deadline is never retried, since an overrun repeats; every other code is stored at once and the job completes — spec 11.7 "retries with backoff up to a limit"; the original retried nothing reported (`execution.py:365-390`), which the spec changes for transient failures; spec 11.10: the API maps a code to a sentence, and the service stores no sentence — an unreachable source costs three attempts and River's backoff (1 s, 16 s) before the report fails |
| R126 | Ruling: a collect attempt reads the pinned revision whatever its `active` flag, reads the credential and the CA bundle at that moment outside any transaction, builds the configuration with `Document.RunConfig` and the stored selection, reads the clock once for the emission time (the window ends there) and once after the collectors, as the command does — the original runs a deactivated profile's queued job (`profiles.py:1219-1233`) and anchors the window when the worker runs (`collection.py:210-237`) — a report queued for long covers a later window than the moment it was requested |
| R127 | Ruling: the snapshot is stored as the bytes of `snapshot.Encode`, at most 32 MiB (33 554 432 bytes), else the report fails `SNAPSHOT_TOO_LARGE` and nothing is stored; a PDF is stored as the bytes of the compile, at most 64 MiB (67 108 864 bytes), else its row fails `PDF_TOO_LARGE` and the other language is untouched; each with SHA-256 in lower-case hex and the size; a render recomputes the snapshot's SHA-256 and fails `INTERNAL_FAILURE` on a mismatch — spec 11.8; `jsonb` reorders keys and drops a repeated one, which defeats F-41 and the byte comparison of the goldens (`current-code.md`, risks) — none |
| R128 | Ruling: `expires_at` is fixed at the request, `created_at + report_retention_days`; the maintenance job expires a report that is `collected` or `failed` with `expires_at <= now` and `expired_at` null by deleting its snapshot and PDF rows and setting `expired_at = now`, in one transaction per batch of at most 100, oldest first, rows taken `FOR UPDATE SKIP LOCKED`; a `queued` or `running` report past its deadline expires once it is terminal; a report whose `expires_at + expired_report_days <= now` and whose `expired_at` is set is purged (its idempotency row, then its row), batch 100; from `expires_at` the API of 3.4 answers 410 until the purge, then 404 — spec 11.8 (the row is marked expired and kept 30 days); the original measured the 410 window from the deadline (`jobs.py:375-400`) — none |
| R129 | Ruling: the maintenance job also deletes, at most 1000 rows each per run, a `login_rate_limits` row whose window and block are both over, and a session revoked or expired for longer than `session_ttl_seconds`, first clearing `rotated_from_id` of the sessions that point at one being deleted; neither purge writes an audit row — F-109 (`ROADMAP.md`); the original never purged them (`original-worker.md` 5.6) — a session row older than the ttl is gone, so a stolen token of that age is unknown rather than recognised as rotated |
| R130 | Ruling: `reports.Service.Request` checks the role (operator or administrator), the idempotency key (`^[A-Za-z0-9._:-]{1,128}$`), and the syntax of the selection before any transaction; then, in one transaction: `SELECT pg_advisory_xact_lock(72427302)` (amended by the owner on 2026-10-06, in place of a lock of the table: only a request raises the count of queued and running reports, so serialising the requests keeps admission exact, and the writes of the worker and of the maintenance job never wait behind a request; the key is `store.ReportRequestLockID`, the migration lock plus one), the idempotency row (same SHA-256 of the canonical request: the stored report, `Replayed`, no admission, no audit; another SHA-256: `IDEMPOTENCY_KEY_REUSED`), the profile row `FOR SHARE` and its current revision (absent or deleted: `PROFILE_NOT_FOUND`; inactive: `PROFILE_INACTIVE`), the selection judged against the revision (`REPORT_SELECTION_NOT_PERMITTED` with fields), admission (queued plus running, global then for the profile: `ADMISSION_GLOBAL_LIMIT`, `ADMISSION_PROFILE_LIMIT`, each with `admission_retry_seconds`), then the River `collect` job by `InsertTx`, the report row carrying the job's id, the idempotency row, and the audit row — spec 11.7; the original's order (`jobs.py:270-321`) with one advisory lock in place of its two; the original re-resolved the profile before a replay, which made a replay fail after a revision; the spec says the same key and body return the same report — every request waits for the one before it, which is microseconds at the admission limits of spec 11.7 |
| R131 | Ruling: `reports.Service.RetryPDF` lets an operator or an administrator retry one language of any report: the report `FOR UPDATE`; absent: `REPORT_NOT_FOUND`; `expires_at <= now`: `REPORT_EXPIRED`; not `collected`: `SNAPSHOT_NOT_READY`; the row `pending`: nothing written, accepted; `ready`: `PDF_RETRY_NOT_ALLOWED`; `failed`: a `render` job by `InsertTx`, the row back to `pending` with its code cleared and the new job's id, the report to phase `rendering` with `completed_at` null, the audit row — spec 11.10 (operator); the original's matrix (`jobs.py:485-521`) for one language — none |
| R132 | Ruling: the audit actions of reports, target type `report`: `report.submit` (`{"profile_id","revision_id"}`), `report.collect` (actor null; `{"code"}`, outcome `failed` when it failed), `report.render` (actor null; `{"language","code"}`, outcome as before), `report.pdf_retry` (`{"language"}`), `report.expire` (actor null; `{}`), `report.purge` (actor null; `{}`); `profile.recreate` joins the profile actions (R138) — spec 11.9 "every change to reports"; the original wrote no row for collection or render outcomes, which the spec's rule covers — three more rows per report |
| R133 | Ruling: the deployment file gains `worker_slots` (2, 1 to 16), `worker_drain_seconds` (30, 1 to 3600), `collect_timeout_seconds` (900, 60 to 3600), `render_timeout_seconds` (300, 10 to 3600), `job_max_attempts` (3, 1 to 10), `report_retention_days` (7, 1 to 365), `expired_report_days` (30, 1 to 365), `admission_limit` (20, 1 to 1000), `admission_profile_limit` (5, 1 to 1000, not above `admission_limit`), `admission_retry_seconds` (5, 1 to 300); no lease, heartbeat, poll, or stale key, since River owns them; no environment override — spec 11.2; the original's names and bounds where they mean the same (`original-worker.md` section 9) — none |
| R134 | Ruling: `worker [--config platform.toml] [--health-listen :8091]` loads the settings (exit 2), locates and probes Typst (exit 1), opens the pool with at least `worker_slots + 8` connections (exit 1), applies the migrations under the lock (exit 1), listens for health (exit 1), starts the River client, logs one line, and on a signal stops taking jobs, waits `worker_drain_seconds` for the running ones, cancels what remains (River makes it `available` with its attempt restored), and exits 0; a handler that has not returned `worker_drain_seconds` after the cancel is abandoned: the command prints `error: jobs did not stop` and exits 1, and River's rescuer recovers the job (amended by the owner on 2026-10-06) — spec 4.2, 11.1, 11.7; `river.md` sections 6 and 9 (LISTEN holds one connection; a stopped job's context returns it); the exit codes of `serve` — a worker migrates as `serve` does, under the same lock, so the two start in any order |
| R135 | Ruling: `internal/platform/health` holds the two routes `GET /health/live` and `GET /health/ready`; `ready` runs a list of named checks, each `func(ctx) bool`, within two seconds, and answers 200 `{"status":"ready","checks":{...}}` or 503 `{"status":"not_ready",...}`; `serve` gives `database` and `migrations`, as today; `worker` adds `worker` (the client started and not stopping) — spec 11.1; the original's worker health (`worker_health.py`) without its keyring and S3 checks — none |
| R136 | Ruling: `internal/pipeline.Collect` runs preflight, collectors, the useful check, the second clock read, the id, analysis, and `snapshot.Build`; `internal/render.Document` makes `report.json` from a snapshot and a language, `render.Compile` makes the PDF bytes from it and refuses bytes that do not start with `%PDF-`; the command calls them and writes the PDF with its atomic write, and its output, exit codes, and order of clock reads do not change — spec 4.3 names the same steps for the command and the worker; `current-code.md` 1.3 — the command holds the PDF in memory before writing it, which the 64 MiB cap bounds for the worker and nothing bounds for the command |
| R137 | Ruling: `profiles.Service.Source(d)` returns the Prometheus options through the policy transport, reading the credential and the CA bundle; `profiles.Service.Revision(ctx, id)` returns a pinned revision; `profiles.Service.Reveal` is called with every credential read; `profiles.Classify` maps an error to a code of R108; the profile test calls them and is bounded by `min(3 × timeout_seconds, 120 s)`; the host name rule is `netpolicy.ValidHostname`, the one pattern plus at most 253 characters, used by the policy and by the settings — F-117; one place for the source of a run, so the test and the worker cannot drift (`current-code.md` 1.3) — a slow source that needs more than two minutes for its preflight fails the test as `ENDPOINT_UNREACHABLE` and still runs as a report |
| R138 | Ruling: `admin import-profiles` re-creates a profile deleted through the API, prints `recreated <id>`, and writes `profile.recreate` with the metadata of `profile.create` — the owner's ruling of 2026-10-06 on F-115 — none |
| R139 | Ruling: the blocked classes gain `::/96` (IPv4-compatible), `::ffff:0:0:0/96` (IPv4-translated), and `64:ff9b:1::/48` (local-use NAT64); `::ffff:0:0/96`, the IPv4-mapped prefix, is not added, since every address is unmapped before it is judged — the owner's ruling of 2026-10-06 on F-116; the notation of `ROADMAP.md` read with `classes.go` (`current-code.md` 4.5) — an allowlist entry inside these prefixes lifts them only one address at a time |
| R140 | Ruling: the snapshot reader decodes with `encoding/json/v2`, which refuses a repeated key (F-41) and a text that is not UTF-8; `Check` gains the five rules of F-89 and a confidence rule per code (F-102); a test pins the hash of the contract's key paths, so a change of shape fails until `SchemaVersion` is raised (F-93); no golden snapshot changes — the owner's decisions of 2026-10-05 — a snapshot written by hand with text that is not UTF-8 is refused where it was read before |
| R141 | Ruling: a handler returns to River only `fmt.Errorf("<kind>: %s", code)`; the cause goes to the worker's log through the run's redactor; `profiles.Service.Reveal` adds each credential to that redactor before the credential is used, in `worker`, `serve`, and `admin` alike; `serve` and `admin` are wired in the task that adds `Reveal`, with a test on the handler `serve` builds (amended by the owner on 2026-10-06) — `AGENTS.md` (one redactor, a secret added as soon as it is known); `river.md` section 9 (River stores job errors in `river_job`) — none |
| R142 | Ruling (added by the owner on 2026-10-06): `reports.collect_job_id` and `report_pdfs.job_id` hold the id of the River job that runs them, set in the transaction that inserts the job; the maintenance job runs a reconcile step first: a `queued` or `running` report whose collect job is absent from `river_job` or is `discarded`, `cancelled`, or `completed` is `failed`, `complete`, `JOB_ATTEMPTS_EXHAUSTED`, with a `report.collect` failed row; a `pending` PDF row whose job is absent or in one of those states is `failed`, `PDF_ATTEMPTS_EXHAUSTED`, with a `report.render` failed row, and its report goes to `complete` when no row is `pending`; rows taken `FOR UPDATE SKIP LOCKED`, batch 100; the store reads `river_job` by id for it, the one query of this project over River's tables — River discards a job rescued at its last attempt, a job whose handler panicked on its last attempt, and a job whose stale attempt failed it, and none of those leaves a handler to write the report; the original's reaper codes (`queue.py:66-82, 409-445`) — a report fails up to a minute after River gave up on it |

## Findings this plan closes

| Finding | How |
|---|---|
| F-41 | Task 1 (R140) |
| F-89 | Task 1 (R140) |
| F-93 | Task 1 (R140) |
| F-102 | Task 1 (R140) |
| F-109 | Task 10 (R129) |
| F-115 | Task 3 (R138) |
| F-116 | Task 3 (R139) |
| F-117 | Task 3 (R137); the worker's own ceilings in Task 4 (R133) |

## File structure

| Path | Responsibility in this part |
|---|---|
| `internal/snapshot/codec.go`, `codec_test.go`; `internal/analysis/registry.go`, `registry_test.go` | The freeze (Task 1) |
| `internal/guard/imports.go`, `imports_test.go` | The five new ranks (Task 2) |
| `internal/pipeline/`, `internal/render/` | The shared run and render (Task 2) |
| `internal/cli/run.go`, `output.go` | Call the shared run and render (Task 2) |
| `internal/platform/netpolicy/classes.go`, `url.go`; `internal/platform/settings/settings.go` (host names); `internal/platform/profiles/source.go`, `test.go`, `importer.go`, `service.go`; `internal/cli/serve.go`, `admin.go` (`Reveal`) | F-115, F-116, F-117, the shared source, the redaction of a credential (Task 3) |
| `internal/platform/settings/settings.go` | The ten keys (Task 4) |
| `go.mod`, `go.sum`; `internal/platform/store/migrations/00003_river.sql`, `00004_reports.sql`; `internal/platform/store/river_test.go`, `migrate_test.go`; `internal/cli/admin_test.go` | The schema (Task 5) |
| `internal/platform/store/reports.go`, `reports_test.go`, `retention.go`, `retention_test.go` | The queries (Task 6, Task 10) |
| `internal/platform/reports/` | The request, the retry, the job arguments, the queue (Task 7) |
| `internal/platform/worker/collect.go`, `render.go`, `maintenance.go`, `client.go` and tests | The handlers and the client (Tasks 8 to 11) |
| `internal/platform/health/`; `internal/platform/api/health.go`, `server.go` | The shared health routes (Task 11) |
| `internal/cli/worker.go`, `worker_test.go`, `run.go`, `root_test.go` | The `worker` command (Task 11) |
| `Makefile` | `test-postgres` also runs the worker's tests tagged `postgres template` (Task 9) |
| `AGENTS.md`, `README.md`, `CHANGELOG.md` | The documents (Task 12) |

---

### Task 0: Open the part

**Executor:** Main thread.

- [x] **Step 1:** `main` at `6303688`, clean, pushed; `ROADMAP.md` and `CHANGELOG.md` read and true.
- [x] **Step 2:** `git switch -c feat/platform-worker`.
- [ ] **Step 3:** `ROADMAP.md`: the summary line of 3.3 names this plan; a section "Phase 3.3" with P33-0 to P33-12, all `OPEN`; the rulings R119 to R142.
- [ ] **Step 4:** Commit the plan: `docs(plan): add the plan of part 3.3` (36 characters).

**Done when:** the plan is committed on the branch and `ROADMAP.md` holds the part.

---

### Task 1: The snapshot frozen (R140; F-41, F-89, F-93, F-102)

**Executor:** Subagent. May run beside Tasks 2 and 3.

**Files:**
- Modify: `internal/snapshot/codec.go` (the first decode in `Decode`, codec.go:125-166; `checkStatistics` 874-909; `checkFindingRegistry` 1012-1045; a rule after `checkFindingScope`; `checkAutoscaling` 1305; `checkHPARows` 1363-1403; `checkWorkloadCoverage` 1512-1541), `codec_test.go`
- Modify: `internal/analysis/registry.go` (`Definition` gains the confidence rule; `buildRegistry`), `registry_test.go`

**Interfaces:**

```go
// analysis
// ConfidenceRule says which confidence a finding of a code may carry.
type ConfidenceRule struct {
	Fixed []string // the confidences allowed whatever the row; empty when a source below decides
	// Exactly one of these is set when Fixed is empty.
	FromApproximate bool // exact when the row is not approximate, inferred when it is
	FromPool        bool // equal to nodepools[].confidence of the pool in scope
	FromAutoscaling bool // equal to summary.autoscaling.confidence
}

type Definition struct {
	// ... the fields of today ...
	Confidence ConfidenceRule
}
```

The rule per code is the one `analysis` writes, read from the code (`current-code.md` 2.5): `exact` alone for the `*_REQUEST_MISSING`, `*_LIMIT_MISSING`, `*_BURSTY`, `HPA_PINNED_REPLICAS`, `HPA_MAX_REPLICAS_REACHED`, `HPA_NEAR_MAX_REPLICAS`, `DEPLOYMENT_WITHOUT_HPA`, `STATEFULSET_WITHOUT_HPA_INFO`, `NODE_*_USAGE_*`, `NODE_*_REQUEST_HIGH`, `NODE_LOW_UTILIZATION`; `FromApproximate` for the six `*_REQUEST_{HEALTHY,ATTENTION,CRITICAL}`, the six `*_LIMIT_{...}`, and the `*_OVERREQUESTED` of namespaces and workloads; `inferred` alone for `HPA_REPLICA_DIVERGENCE` and `NODE_OVERREQUESTED`; `unknown` alone for `NODEPOOL_UNKNOWN` and `NODE_AUTOSCALING_UNKNOWN`; `FromPool` for `NODEPOOL_*_UNEVEN`; `FromAutoscaling` for every other `NODE_AUTOSCALING_*`. The implementer reads each `analysis` file the list cites and stops and reports a code whose writer disagrees. A test walks the registry and fails for a definition with no rule.

Messages, each exact:

| Rule | Message |
|---|---|
| F-41, a repeated key at any depth | `snapshot repeats the key <path>`, `<path>` written as the messages of `Check` write one: `quality.warnings[0].code` (from the JSON pointer of the `jsontext` error) |
| F-89, covered with no targeting HPA | `workloads[<i>] (<ns>/<name>) is covered and no HPA targets it` |
| F-89, autoscaling shape | after the two existing messages of `checkAutoscaling`: `summary.autoscaling holds status <s>, source <src>, and confidence <c>, which analysis never writes`; the three shapes allowed are (`enabled` or `disabled`, `configured`, `exact`), (`unknown`, `none`, `unknown`), (`enabled`, `raw_metrics`, `inferred`) |
| F-89, a count beside a source that writes none | `summary.autoscaling with source <src> holds a count`, for `configured` and `none` |
| F-89, a status beside a contrary finding | `findings[<i>] is <code>, which the autoscaling status <s> never gives`; `unknown` gives only `NODE_AUTOSCALING_UNKNOWN`, `disabled` only `NODE_AUTOSCALING_DISABLED` and `NODE_AUTOSCALING_DISABLED_PRESSURE`, `enabled` only `NODE_AUTOSCALING_NEAR_MAX`, `NODE_AUTOSCALING_BOUNDS_UNKNOWN`, `NODE_AUTOSCALING_AT_MIN`, `NODE_AUTOSCALING_ENABLED` (read `internal/analysis/autoscaling.go:27-66`; stop and report a difference) |
| F-89, a value state missing a number | `<path> has the state value and lacks a number`, `<path>` as `checkStatistics` already writes it |
| F-89, `spec_min` above `spec_max` | `hpas[<i>].spec_min is greater than hpas[<i>].spec_max` |
| F-102 | `findings[<i>].confidence <c> does not match the definition of <code>` |
| F-93 | the failure of `TestSchemaShapeIsFrozen`: `the shape of schema 1.0 changed; raise snapshot.SchemaVersion and record the new shape` |

The decode keeps every check of today and its order: the version is still named before any key (F-44). The duplicate-name error of `jsontext` carries the JSON pointer of the object that holds the repeated name (`/findings/0`), and the name in its text; the path of the message is built from both (`findings[0].code`). `encoding/json/v2` decodes into `any` with its default options (a repeated name refused; invalid UTF-8 refused); the trailing-data check stays. A negative autoscaler count stays legal (`TestCheckKeepsANegativeAutoscalerCount`).

**Tests:**

| Test | Input | Expected |
|---|---|---|
| `TestDecodeRefusesARepeatedKey` | `direct-complete` with `"schema_version"` written twice at the top; with `"profile"` written twice inside `report`; with `"code"` written twice inside `findings[0]` | `snapshot repeats the key schema_version`; `snapshot repeats the key report.profile`; `snapshot repeats the key findings[0].code` |
| `TestDecodeRefusesTextThatIsNotUTF8` | `direct-complete` with a byte `0xff` inside a string | an error starting `snapshot` (the exact text pinned by the test from the run) |
| `TestCheckRefusesACoveredWorkloadNoHPATargets` | `direct-complete`, one covered workload whose HPA is removed | the message |
| `TestCheckRefusesAnAutoscalingShape` | each of: `configured` with confidence `inferred`; `raw_metrics` with status `disabled`; `configured` with `nodes_min = 1`; `none` with `nodes_max = 1` | the shape message for the first two, the count message for the last two |
| `TestCheckRefusesAContraryAutoscalingFinding` | `edge-scaling` with status `disabled` and its `NODE_AUTOSCALING_ENABLED` finding kept | the message |
| `TestCheckRefusesAValueMissingANumber` | a `value` statistic with `peak` null | the message |
| `TestCheckRefusesSpecMinAboveSpecMax` | an HPA with `spec_min = 5`, `spec_max = 2` | the message |
| `TestCheckRefusesAConfidenceTheCodeNeverTakes` | a `WORKLOAD_CPU_REQUEST_MISSING` with `inferred`; a `WORKLOAD_CPU_REQUEST_HEALTHY` on a non-approximate row with `inferred`, findings and recommendations changed together | the F-102 message, twice |
| `TestEveryDefinitionHasAConfidenceRule` | the registry | every code has `Fixed` or exactly one source |
| `TestSchemaShapeIsFrozen` | SHA-256 of the sorted key paths of the reflection walk, joined by `\n` | equals a literal recorded in the test when it is written |
| `TestGoldenSnapshotsAreUnchanged` and every test of `internal/snapshot`, `internal/document`, `internal/goldentest`, `internal/cli` | unchanged | pass; no file of `testdata/` changes |

- [ ] Steps: tests red, implement, `make lint`, `make test`, `go test -count=1 -tags=template ./internal/cli/`; commit `fix(snapshot): refuse what Build never writes` (45 characters).

**Done when:** every new test passes, no golden file changed, and `git diff --stat` touches only the four files.

---

### Task 2: The ranks, the shared run, and the shared render (R136)

**Executor:** Subagent. May run beside Tasks 1 and 3.

**Files:**
- Modify: `internal/guard/imports.go` (`Layers`: `internal/pipeline` 75, `internal/render` 100, `internal/platform/health` 128, `internal/platform/reports` 146, `internal/platform/worker` 160), `imports_test.go` (the lists of this part; the count becomes 31)
- Create: `internal/platform/health/doc.go`, `internal/platform/reports/doc.go`, `internal/platform/worker/doc.go` (a package clause and one sentence each)
- Create: `internal/pipeline/pipeline.go`, `pipeline_test.go`
- Create: `internal/render/render.go`, `render_test.go` (`//go:build template` for the compile tests)
- Modify: `internal/cli/run.go` (`runSnapshot` and `runPDF` call `pipeline.Collect`; `runPDF` and `runFromSnapshot` call `render.Document` and `render.Compile`; `compileToPath`, `withWorkFolder`, `checkPDFMagic`, `settlePDFMode` go when nothing calls them), `internal/cli/output.go` if a helper moves

**Interfaces:**

```go
package pipeline

// Run is one collection: the source, the configuration, the window judged
// before the first request, and the two things a run reads at its end.
type Run struct {
	Options   prom.Options
	Config    config.Config
	Window    collect.Window
	EmittedAt time.Time                // the first clock read, taken by the caller
	Now       func() time.Time         // read once, after the useful check
	NewID     func() (string, error)   // called once, after that read
	Version   string
	Step      func(ctx context.Context, s Step) error // nil: no hook; an error stops the run and is returned
}

type Step string

const (
	StepCollecting   Step = "collecting"   // after the preflight
	StepAnalyzing    Step = "analyzing"    // after the useful check
	StepSnapshotting Step = "snapshotting" // after analysis
)

// Result is what the command prints from and what the worker stores.
type Result struct {
	Snapshot     snapshot.Snapshot
	Capabilities []discovery.Capability
	Collected    collect.Collected
}

// Collect runs the preflight, the collectors in their order, the useful
// check, reads Now, takes the id, analyses, and builds the snapshot. It
// returns the errors of discovery, prom, and collect unchanged.
func Collect(ctx context.Context, r Run) (Result, error)
```

```go
package render

// MaxPDFBytes is the largest PDF the platform stores.
const MaxPDFBytes = 64 << 20

// ErrNotPDF is returned when the compiler wrote bytes that do not start as a PDF does.
var ErrNotPDF = errors.New("the compiler wrote a file that is not a PDF")

// Document makes report.json from the snapshot in the language, narrowed by opts.
func Document(s snapshot.Snapshot, lang catalog.Language, opts document.Options) ([]byte, error)

// Compile unpacks the template into a fresh folder under tempDir, compiles
// data, and returns the PDF bytes; the folder is removed before it returns.
func Compile(ctx context.Context, tc typst.Toolchain, tempDir string, data []byte) ([]byte, error)
```

The command keeps every exit code: a `render.Compile` error is exit 6, as `compileToPath` was; a `render.Document` error keeps the code `document.Build` gave. The PDF bytes are written with `writeAtomicFile`, which follows the umask as today. `tempDir` is the folder the command passes to its work folder today. The order inside `Collect` is: preflight, `StepCollecting`, collectors, useful check, `Now`, `NewID`, `StepAnalyzing`, analysis, `StepSnapshotting`, `snapshot.Build`. The step hook is called at those three points and nowhere else; the command passes nil.

**Tests:**

| Test | Input | Expected |
|---|---|---|
| `TestLayersHoldEveryPackageOfThisPart`, `TestLayersHoldThePlatformPackages` | the five ranks | pass, 31 entries |
| `TestCollectEqualsTheGoldenSnapshot` (pipeline) | `promtest` server of `direct-complete`, the configuration and fixed clock and id that `goldentest.Snapshot` uses for it (read `internal/goldentest/goldentest.go:198`) | `snapshot.Encode` of the result equals `testdata/snapshot/direct-complete.json` byte for byte |
| `TestCollectCallsTheStepsInOrder` (pipeline) | the same, a hook recording its calls | `collecting`, `analyzing`, `snapshotting` |
| `TestCollectStopsOnAStepError` (pipeline) | a hook that fails at `analyzing` | that error; `Now` read once and `NewID` called once (both precede the step), `snapshot.Build` never reached |
| `TestCollectReadsTheClockOnceAndTheIDOnce` (pipeline) | counting `Now` and `NewID` | one each |
| `TestCollectReturnsNothingToReport` (pipeline) | a scenario whose collectors return no row (the fake server's empty vectors, as `internal/cli`'s exit-5 test builds it) | `collect.ErrNoUsefulSections`; `Now` and `NewID` never called |
| `TestCompileReturnsAPDF` (render, `template`) | `render.Document` of `direct-complete` in `en` | bytes starting `%PDF-`; the temporary folder is gone |
| `TestCompileRefusesBytesThatAreNotAPDF` (render, `template`) | a toolchain whose binary is a script writing `hello` to the output | `ErrNotPDF` |
| `TestDocumentEqualsTheDataFile` (render) | `direct-complete`, `pt-BR` and `en`, empty options | equals `template/tests/capacity/fixtures/command/direct-complete.<lang>.json` |
| every test of `internal/cli`, untagged and with `template` | unchanged | pass |

- [ ] Steps: tests red, implement, `make lint`, `make test`, `go test -count=1 -tags=template ./template/ ./internal/typst/ ./internal/cli/ ./internal/render/`; commit `refactor(cli): share the run and the render` (43 characters).

**Done when:** the suites pass, `internal/cli/run.go` holds no copy of the collect block, and `compileToPath` is gone.

---

### Task 3: The policy, the profiles, and the shared source (R137, R138, R139, R141; F-115, F-116, F-117)

**Executor:** Subagent, against the running server. May run beside Tasks 1 and 2.

**Files:**
- Modify: `internal/platform/netpolicy/classes.go` (three prefixes), `classes_test.go`, `url.go` (`ValidHostname`), `url_test.go`
- Modify: `internal/platform/settings/settings.go` (`parseHostnames` calls `netpolicy.ValidHostname`; `hostnamePattern` goes), `settings_test.go`
- Create: `internal/platform/profiles/source.go`, `source_test.go`
- Modify: `internal/platform/profiles/test.go` (calls `Source`, `Classify`; the ceiling), `test_test.go`, `service.go` (`Reveal`, `Revision`, `ActionProfileRecreate`), `importer.go` (`OutcomeRecreated`), `importer_test.go`
- Modify: `internal/cli/admin_test.go` (`TestAdminImportProfilesCommand` gains the recreated case)
- Modify: `internal/cli/serve.go` and `internal/cli/admin.go` (every `profiles.Service` they build sets `Reveal` to the run's redactor's add function, R141), `internal/cli/serve_test.go` or the tagged test file of `serve` (`TestServeRevealsACredential`)

**Interfaces:**

```go
// netpolicy
// ValidHostname reports whether host is a lower-case ASCII host name of at
// most 253 characters, each label 1 to 63 characters.
func ValidHostname(host string) bool

// profiles
type Service struct {
	// ... the fields of 3.2 ...
	Reveal func(secret string) // called with every credential read, before it is used; nil does nothing
}

// Revision is one revision as a run reads it.
type Revision struct {
	ID        uuid.UUID
	ProfileID string
	Number    int
	Active    bool
	Document  Document
}

// Revision reads the revision with id, whatever its profile's pointer. ErrNotFound when absent.
func (s *Service) Revision(ctx context.Context, id uuid.UUID) (Revision, error)

// SourceError is a reference that could not be read; Code is
// PROFILE_CREDENTIAL_UNAVAILABLE or PROFILE_CA_BUNDLE_UNAVAILABLE.
type SourceError struct{ Code string }

// Source reads the credential and the CA bundle of d and returns the
// options of the Prometheus client through the policy transport, with the
// metrics of d.
func (s *Service) Source(d Document, metrics config.Metrics) (prom.Options, error)

// Classify maps an error of a run to a code of R108 and reports whether it knew it.
func Classify(err error) (string, bool)

const ActionProfileRecreate = "profile.recreate"
const OutcomeRecreated = "recreated"
```

`runTest` calls `Source`, `Classify`, and bounds its context with `min(3 × timeout_seconds, 120 s)`; its codes and audit rows do not change. `Classify` keeps today's order (`*netpolicy.Error`, `*SourceError`, the `prom` kinds, `*discovery.ScopeError`, `discovery.ErrInsufficient`, a done context as `ENDPOINT_UNREACHABLE`) and returns false for any other error. The importer tells a profile that never existed (`GetOrCreateProfile` inserted the row) from one whose pointer is cleared and which has revisions (`LatestProfileRevision` finds one): the second is `recreated`, audited `profile.recreate` with the metadata of `profile.create`, and continues the revision count.

**Tests:**

| Test | Input | Expected |
|---|---|---|
| `TestBlockedClasses` gains | `::10.0.0.1`, `::ffff:0:10.0.0.1`, `64:ff9b:1::a00:1` | blocked |
| `TestNotBlocked` keeps | `::ffff:10.20.0.4` (unmapped to an IPv4 private address the classes do not block), `fc00::1` | not blocked |
| `TestValidHostname` | a name of 253 characters in labels of 63; of 254; a label of 64; `a..b`; `A.b` | true; false; false; false; false |
| `TestLoadHostnamesRefused` gains | a 254-character entry | `allowed_endpoint_hostnames entry 1 is not a host name` |
| `TestSourceReadsTheReferences` (postgres) | a credential file and a CA bundle in the fixture folders | options with the token, a `*netpolicy.Transport` whose TLS pool holds the bundle; `Reveal` called once with the token |
| `TestSourceRefusesAMissingCredential` | the name of an absent file | `*SourceError{PROFILE_CREDENTIAL_UNAVAILABLE}`; `Reveal` never called |
| `TestRevisionReadsAnInactiveOne` | a profile revised to inactive, the first revision's id | the first revision, `Active` true; the second's id gives `Active` false |
| `TestTestCeiling` | a document with `timeout_seconds = 600`, a server that never answers, a fake clock observing the deadline | the context's deadline is 120 s after the start |
| `TestImportRecreatesADeletedProfile` | create through the service, delete, import the same document | outcome `recreated`; one `profile.recreate` row with no actor; revision 2 |
| `TestAdminImportProfilesCommand` gains | the same through the command | the line `recreated prod` |
| `TestServeRevealsACredential` (cli, postgres) | a profile test through the handler `serve` builds (`buildServer`), with a credential file | the credential is in the run's redactor: a write of it to the redacted standard error comes out masked |
| every test of `netpolicy`, `settings`, `profiles`, `cli` with and without `postgres` | unchanged | pass |

- [ ] Steps: tests red, implement, `make lint`, `make test`, `make test-postgres` scope run by the subagent: `KCR_TEST_DATABASE_DSN=... go test -count=1 -tags=postgres ./internal/platform/... ./internal/cli/...`; commit `fix(profiles): share the source of a run` (40 characters).

**Done when:** the suites pass, `settings` holds no host name pattern of its own, and every `profiles.Service` that `serve` and `admin` build sets `Reveal`.

---

### Task 4: The settings of the worker and the reports (R133)

**Executor:** Subagent. After Task 3.

**Files:**
- Modify: `internal/platform/settings/settings.go`, `settings_test.go`

**Interfaces:**

```go
type Settings struct {
	// ... the fields of 3.1 and 3.2 ...
	WorkerSlots           int           // worker_slots
	WorkerDrain           time.Duration // worker_drain_seconds
	CollectTimeout        time.Duration // collect_timeout_seconds
	RenderTimeout         time.Duration // render_timeout_seconds
	JobMaxAttempts        int           // job_max_attempts
	ReportRetention       time.Duration // report_retention_days, in days
	ExpiredReportKept     time.Duration // expired_report_days, in days
	AdmissionLimit        int           // admission_limit
	AdmissionProfileLimit int           // admission_profile_limit
	AdmissionRetry        time.Duration // admission_retry_seconds
}
```

Messages, each in the style of `errTTLRange`: `worker_slots must be between 1 and 16`, `worker_drain_seconds must be between 1 and 3600`, `collect_timeout_seconds must be between 60 and 3600`, `render_timeout_seconds must be between 10 and 3600`, `job_max_attempts must be between 1 and 10`, `report_retention_days must be between 1 and 365`, `expired_report_days must be between 1 and 365`, `admission_limit must be between 1 and 1000`, `admission_profile_limit must be between 1 and 1000`, `admission_profile_limit must not exceed admission_limit`, `admission_retry_seconds must be between 1 and 300`. A value of the wrong type keeps today's message.

**Tests:**

| Test | Input | Expected |
|---|---|---|
| `TestLoadWorkerDefaults` | no new key | 2, 30 s, 900 s, 300 s, 3, 7 days, 30 days, 20, 5, 5 s |
| `TestLoadWorkerRanges` | each key at its lower bound minus one and its upper bound plus one | the key's message, twice each |
| `TestLoadWorkerBounds` | each key at its two bounds | accepted |
| `TestLoadAdmissionProfileAboveGlobal` | `admission_limit = 3`, `admission_profile_limit = 4` | `admission_profile_limit must not exceed admission_limit` |
| `TestLoadRefusesUnknownKey` | `worker_slot = 2` | `<path>: unknown key worker_slot` |

- [ ] Steps: tests red, implement, `make lint`, `make test`; commit `feat(settings): add the worker and report keys` (46 characters).

**Done when:** the tests pass.

---

### Task 5: River and the report tables (R119, R120, R121, R122, R142)

**Executor:** Subagent, against the running server.

**Files:**
- Modify: `go.mod`, `go.sum` (`go get github.com/riverqueue/river@v0.49.0 github.com/riverqueue/river/riverdriver/riverpgxv5@v0.49.0 github.com/riverqueue/river/rivertype@v0.49.0`, then `go mod tidy`)
- Create: `internal/platform/store/migrations/00003_river.sql` (generated once by a scratch program under `.cache/phase-3/part-3/river-sql/`, never by a tracked one)
- Create: `internal/platform/store/migrations/00004_reports.sql`
- Create: `internal/platform/store/river_test.go`
- Modify: `internal/platform/store/migrate_test.go` (versions 2 → 4 at lines 50, 227, 361; the down-and-up test walks 4 → 3 → 2 and back), `internal/cli/admin_test.go:120` (`migrate: schema at version 4`)

`00004_reports.sql`, the Up (the Down drops the triggers, the tables in the order `report_idempotency_keys`, `report_pdfs`, `report_snapshots`, `reports`, then the function):

```sql
CREATE TABLE reports (
	id uuid NOT NULL,
	owner_id uuid NOT NULL,
	profile_id text NOT NULL,
	profile_revision_id uuid NOT NULL,
	request jsonb NOT NULL,
	status text NOT NULL,
	phase text NOT NULL,
	attempt integer NOT NULL DEFAULT 0,
	failure_code text,
	collect_job_id bigint NOT NULL,
	created_at timestamptz NOT NULL,
	started_at timestamptz,
	collected_at timestamptz,
	completed_at timestamptz,
	expires_at timestamptz NOT NULL,
	expired_at timestamptz,
	updated_at timestamptz NOT NULL,
	CONSTRAINT pk_reports PRIMARY KEY (id),
	CONSTRAINT fk_reports_owner FOREIGN KEY (owner_id) REFERENCES users (id) ON DELETE RESTRICT,
	CONSTRAINT fk_reports_profile FOREIGN KEY (profile_id) REFERENCES profiles (id) ON DELETE RESTRICT,
	CONSTRAINT fk_reports_revision FOREIGN KEY (profile_revision_id) REFERENCES profile_revisions (id) ON DELETE RESTRICT,
	CONSTRAINT ck_reports_status CHECK (status IN ('queued', 'running', 'collected', 'failed')),
	CONSTRAINT ck_reports_phase CHECK (
		(status = 'queued' AND phase = 'waiting')
		OR (status = 'running' AND phase IN ('preflight', 'collecting', 'analyzing', 'snapshotting'))
		OR (status = 'collected' AND phase IN ('rendering', 'complete'))
		OR (status = 'failed' AND phase = 'complete')),
	CONSTRAINT ck_reports_failure CHECK ((status = 'failed') = (failure_code IS NOT NULL)),
	CONSTRAINT ck_reports_attempt CHECK (attempt >= 0)
);
CREATE INDEX ix_reports_active_by_profile ON reports (profile_id) WHERE status IN ('queued', 'running');
CREATE INDEX ix_reports_due_for_expiry ON reports (expires_at, id) WHERE expired_at IS NULL;
CREATE INDEX ix_reports_due_for_purge ON reports (expires_at, id) WHERE expired_at IS NOT NULL;

CREATE TABLE report_snapshots (
	report_id uuid NOT NULL,
	schema_version text NOT NULL,
	body bytea NOT NULL,
	sha256 text NOT NULL,
	size_bytes bigint NOT NULL,
	created_at timestamptz NOT NULL,
	CONSTRAINT pk_report_snapshots PRIMARY KEY (report_id),
	CONSTRAINT fk_report_snapshots_report FOREIGN KEY (report_id) REFERENCES reports (id) ON DELETE RESTRICT,
	CONSTRAINT ck_report_snapshots_sha256 CHECK (sha256 ~ '^[0-9a-f]{64}$'),
	CONSTRAINT ck_report_snapshots_size CHECK (size_bytes = octet_length(body) AND size_bytes <= 33554432)
);

CREATE TABLE report_pdfs (
	report_id uuid NOT NULL,
	language text NOT NULL,
	status text NOT NULL,
	failure_code text,
	job_id bigint NOT NULL,
	body bytea,
	sha256 text,
	size_bytes bigint,
	updated_at timestamptz NOT NULL,
	CONSTRAINT pk_report_pdfs PRIMARY KEY (report_id, language),
	CONSTRAINT fk_report_pdfs_report FOREIGN KEY (report_id) REFERENCES reports (id) ON DELETE RESTRICT,
	CONSTRAINT ck_report_pdfs_language CHECK (language IN ('pt-BR', 'en')),
	CONSTRAINT ck_report_pdfs_status CHECK (status IN ('pending', 'ready', 'failed')),
	CONSTRAINT ck_report_pdfs_ready CHECK ((status = 'ready') = (body IS NOT NULL AND sha256 IS NOT NULL AND size_bytes IS NOT NULL)),
	CONSTRAINT ck_report_pdfs_failure CHECK ((status = 'failed') = (failure_code IS NOT NULL)),
	CONSTRAINT ck_report_pdfs_sha256 CHECK (sha256 IS NULL OR sha256 ~ '^[0-9a-f]{64}$'),
	CONSTRAINT ck_report_pdfs_size CHECK (size_bytes IS NULL OR (size_bytes = octet_length(body) AND size_bytes <= 67108864))
);

CREATE TABLE report_idempotency_keys (
	owner_id uuid NOT NULL,
	key text NOT NULL,
	request_sha256 text NOT NULL,
	report_id uuid NOT NULL,
	created_at timestamptz NOT NULL,
	CONSTRAINT pk_report_idempotency_keys PRIMARY KEY (owner_id, key),
	CONSTRAINT fk_report_idempotency_keys_owner FOREIGN KEY (owner_id) REFERENCES users (id) ON DELETE RESTRICT,
	CONSTRAINT fk_report_idempotency_keys_report FOREIGN KEY (report_id) REFERENCES reports (id) ON DELETE RESTRICT,
	CONSTRAINT uq_report_idempotency_keys_report UNIQUE (report_id)
);

-- +goose StatementBegin
CREATE FUNCTION reject_report_snapshot_update() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
	RAISE EXCEPTION 'report_snapshots rows are immutable' USING ERRCODE = 'integrity_constraint_violation';
END;
$$;
-- +goose StatementEnd
CREATE TRIGGER report_snapshots_are_immutable BEFORE UPDATE ON report_snapshots
	FOR EACH ROW EXECUTE FUNCTION reject_report_snapshot_update();
```

The implementer matches the spelling of the triggers and constraints of `00002_profiles.sql` where this text differs from it, and reports the difference. `collect_job_id` and `job_id` are the `river_job.id` of the job that runs the row (R142); they carry no foreign key, since River's cleaner deletes finished jobs.

**Tests:**

| Test | Input | Expected |
|---|---|---|
| `TestRiverMigrationMatchesTheLibrary` (store, postgres not needed: untagged) | the embedded `00003_river.sql` split at its `StatementBegin` markers; `rivermigrate.New(riverpgxv5.New(nil), nil).AllVersions()` | the Up blocks equal versions 2 to 8 of the library and the Down blocks their reverse, each compared after trimming white space at both ends |
| `TestMigrateIsIdempotent`, `TestMigrateToVersionFour`, `TestMigrateDownAndUp` | as today, version 4 | pass |
| `TestRiverTablesExist` | a migrated database | `river_job`, `river_leader` (unlogged), `river_queue`, `river_notification`, the enum `river_job_state`, and the function `river_job_state_in_bitmask` exist; `river_client`, `river_client_queue` (created by version 5, dropped by 7), and `river_migration` do not |
| `TestReportChecks` | inserts breaking each check: status `done`; `queued` with `preflight`; `failed` without a code; `collected` with a code; a PDF `ready` without a body; a snapshot of 64 bytes declaring 65; a PDF language `es` | SQLSTATE `23514` each |
| `TestReportSnapshotsAreImmutable` | `UPDATE report_snapshots` | SQLSTATE `23000` |
| `TestReportForeignKeysRestrict` | delete a user who owns a report | SQLSTATE `23503` |
| `TestMigrateHoldsTheLock` (from 3.1) | unchanged | pass |
| every tagged test of `internal/platform/...` and `internal/cli/...` | unchanged except the four version literals | pass |

- [ ] Steps: generate the River file with the scratch program, write the reports file, tests red, implement, `make lint`, `make test`, the full tagged run; commit `feat(store): add River and the report tables` (44 characters).

**Done when:** the full tagged run passes twice, and `go list -m all` shows no module the forbidden-module guard names.

---

### Task 6: The queries of reports (R121, R122, R124, R127, R128, R130, R142)

**Executor:** Subagent, against the running server.

**Files:**
- Create: `internal/platform/store/reports.go`, `reports_test.go`

**Interfaces:**

```go
type Report struct {
	ID                uuid.UUID
	OwnerID           uuid.UUID
	ProfileID         string
	ProfileRevisionID uuid.UUID
	Request           []byte // jsonb
	Status, Phase     string
	Attempt           int
	FailureCode       *string
	CollectJobID      int64
	CreatedAt         time.Time
	StartedAt, CollectedAt, CompletedAt *time.Time
	ExpiresAt         time.Time
	ExpiredAt         *time.Time
	UpdatedAt         time.Time
}

type ReportPDF struct {
	ReportID    uuid.UUID
	Language    string
	Status      string
	FailureCode *string
	JobID       int64
	SHA256      *string
	SizeBytes   *int64
	UpdatedAt   time.Time
}

// ErrStaleAttempt is returned by a write of a running report whose attempt
// or status no longer matches.
var ErrStaleAttempt = errors.New("report attempt is stale")

// ReportRequestLockID is the transaction advisory lock that serialises report requests (R130).
const ReportRequestLockID = 72427302

func LockReportRequests(ctx context.Context, tx pgx.Tx) error // SELECT pg_advisory_xact_lock(ReportRequestLockID)
func InsertReport(ctx context.Context, q Querier, r Report) error
func GetReport(ctx context.Context, q Querier, id uuid.UUID) (Report, error)            // ErrNotFound
func GetReportForUpdate(ctx context.Context, tx pgx.Tx, id uuid.UUID) (Report, error)   // ErrNotFound
func CountActiveReports(ctx context.Context, q Querier, profileID string) (global, profile int, err error) // queued and running
func ClaimReport(ctx context.Context, tx pgx.Tx, id uuid.UUID, attempt int, now time.Time) error
	// from queued, or from running when the stored attempt <= attempt (R124): running, preflight, attempt,
	// started_at = COALESCE(started_at, now); ErrStaleAttempt otherwise
func AdvanceReport(ctx context.Context, q Querier, id uuid.UUID, attempt int, phase string, now time.Time) error // ErrStaleAttempt
func RequeueReport(ctx context.Context, q Querier, id uuid.UUID, attempt int, now time.Time) error             // queued, waiting; ErrStaleAttempt
func FailReport(ctx context.Context, q Querier, id uuid.UUID, attempt int, code string, now time.Time) error   // failed, complete, completed_at; ErrStaleAttempt
func CollectReport(ctx context.Context, q Querier, id uuid.UUID, attempt int, now time.Time) error             // collected, rendering, collected_at; ErrStaleAttempt
func InsertSnapshot(ctx context.Context, q Querier, reportID uuid.UUID, schemaVersion string, body []byte, now time.Time) error // computes sha256 and size
func GetSnapshot(ctx context.Context, q Querier, reportID uuid.UUID) (schemaVersion string, body []byte, sha256 string, err error)
func InsertPendingPDF(ctx context.Context, q Querier, reportID uuid.UUID, language string, jobID int64, now time.Time) error
func GetPDF(ctx context.Context, q Querier, reportID uuid.UUID, language string) (ReportPDF, error)
func FinishPDF(ctx context.Context, q Querier, reportID uuid.UUID, language string, body []byte, now time.Time) error   // pending → ready; ErrStaleAttempt when not pending
func FailPDF(ctx context.Context, q Querier, reportID uuid.UUID, language string, code string, now time.Time) error     // pending → failed; ErrStaleAttempt when not pending
func RetryPDF(ctx context.Context, q Querier, reportID uuid.UUID, language string, jobID int64, now time.Time) error   // failed → pending, code cleared, job_id set
func CountPendingPDFs(ctx context.Context, q Querier, reportID uuid.UUID) (int, error)
func SetReportPhase(ctx context.Context, q Querier, id uuid.UUID, phase string, completedAt *time.Time, now time.Time) error // collected reports only
func GetIdempotencyKey(ctx context.Context, q Querier, ownerID uuid.UUID, key string) (requestSHA256 string, reportID uuid.UUID, err error) // ErrNotFound
func InsertIdempotencyKey(ctx context.Context, q Querier, ownerID uuid.UUID, key, requestSHA256 string, reportID uuid.UUID, now time.Time) error
```

The SHA-256 is computed in Go (`crypto/sha256`, lower-case hex), never by a statement. No statement calls `now()`.

**Tests:**

| Test | Input | Expected |
|---|---|---|
| `TestInsertAndGetReport` | a report of a fixture user and profile | equal after the round trip, times in UTC |
| `TestClaimReportSetsTheAttempt` | queued; claim with 1; claim again with 2 | running, preflight, attempt 1, `started_at` set; then attempt 2 with `started_at` unchanged |
| `TestWritesOfAStaleAttempt` | claimed with 2; `AdvanceReport`, `FailReport`, `CollectReport`, `RequeueReport` with 1 | `ErrStaleAttempt` each; the row unchanged |
| `TestClaimRefusesATerminalReport` | collected; failed | `ErrStaleAttempt` both |
| `TestClaimRefusesAnOlderAttempt` | queued; claim with 2; claim with 1; claim with 2 again | the claim with 1 is `ErrStaleAttempt` and leaves attempt 2; the second claim with 2 succeeds (the run after a soft stop) |
| `TestCountActiveReports` | 2 queued and 1 running of `prod`, 1 queued of `stage`, 1 collected and 1 failed of `prod` | global 4, `prod` 3 |
| `TestInsertSnapshotDigest` | body `{"a":1}\n` | sha256 `e346432021b04179518d9614f3560ccd71354a4ee101ddcb893d6959a9d6301c` and size 8 |
| `TestPDFTransitions` | pending → ready; ready → `FinishPDF` again; pending → failed → retry → pending | ready with sha256 and size; `ErrStaleAttempt`; pending with no code |
| `TestLockReportRequestsSerialises` | tx1 locks; tx2 locks | tx2 waits until tx1 commits (the `expectBlocksUntilCommit` helper of 3.1) |
| `TestLockReportRequestsLeavesWritesFree` | tx1 locks; tx2 runs `AdvanceReport` of a running report and commits | tx2 does not wait |
| `TestIdempotencyKeyPerOwner` | the same key for two owners | two rows; the second insert for one owner violates the primary key |

- [ ] Steps: tests red, implement, `make lint`, the tagged run of `./internal/platform/store/`; commit `feat(store): add the queries of reports` (39 characters).

**Done when:** the tests pass twice.

---

### Task 7: The reports service: request and retry (R123, R130, R131, R132)

**Executor:** Subagent, against the running server.

**Files:**
- Create: `internal/platform/reports/service.go`, `request.go`, `jobs.go`, `queue.go`, and tests

**Interfaces:**

```go
package reports

// CollectArgs and RenderArgs are the River jobs of a report; their Kind is
// the routing key and never changes.
type CollectArgs struct {
	ReportID uuid.UUID `json:"report_id"`
}
func (CollectArgs) Kind() string { return "collect" }

type RenderArgs struct {
	ReportID uuid.UUID `json:"report_id"`
	Language string    `json:"language"`
}
func (RenderArgs) Kind() string { return "render" }

// InsertOpts of each kind: queue "reports", priority 2 for collect and 1 for render,
// MaxAttempts from the settings.
func CollectInsertOpts(s settings.Settings) *river.InsertOpts
func RenderInsertOpts(s settings.Settings) *river.InsertOpts

// NewQueue returns a River client that only inserts.
func NewQueue(pool *pgxpool.Pool, logger *slog.Logger) (*river.Client[pgx.Tx], error)

type Service struct {
	Pool     *pgxpool.Pool
	Queue    *river.Client[pgx.Tx]
	Profiles *profiles.Service
	Settings settings.Settings
	Now      func() time.Time
	NewID    func() uuid.UUID
}

// Selection is a request as the operator sends it; a nil field takes the profile's default.
type Selection struct {
	ProfileID     string   `json:"profile_id"`
	Namespaces    []string `json:"namespaces"`
	Period        *string  `json:"period"`
	Timezone      *string  `json:"timezone"`
	ReportProfile *string  `json:"report_profile"`
	MinSeverity   *string  `json:"min_severity"`
}

// Resolved is the selection a report stores and the worker runs.
type Resolved struct {
	Namespaces    []string `json:"namespaces"`
	Period        string   `json:"period"`
	Timezone      string   `json:"timezone"`
	ReportProfile string   `json:"report_profile"`
	MinSeverity   string   `json:"min_severity"`
}

type Accepted struct {
	ID        uuid.UUID
	Status    string
	Phase     string
	ExpiresAt time.Time
	Replayed  bool
}

// Error is a refusal; Fields map a field of the selection to one of
// missing, invalid, repeated, empty, not_permitted.
type Error struct {
	Code       string
	Fields     map[string]string
	RetryAfter time.Duration // set for the two admission codes
}

func (s *Service) Request(ctx context.Context, actor auth.Principal, key string, sel Selection) (Accepted, error)
func (s *Service) RetryPDF(ctx context.Context, actor auth.Principal, reportID uuid.UUID, language string) error

const (
	ActionReportSubmit   = "report.submit"
	ActionReportCollect  = "report.collect"
	ActionReportRender   = "report.render"
	ActionReportPDFRetry = "report.pdf_retry"
	ActionReportExpire   = "report.expire"
	ActionReportPurge    = "report.purge"
)
```

Codes of `Error`: `IDEMPOTENCY_KEY_INVALID`, `IDEMPOTENCY_KEY_REUSED`, `REQUEST_INVALID` (with fields), `PROFILE_NOT_FOUND`, `PROFILE_INACTIVE`, `REPORT_SELECTION_NOT_PERMITTED` (with fields), `ADMISSION_GLOBAL_LIMIT`, `ADMISSION_PROFILE_LIMIT`, `REPORT_NOT_FOUND`, `REPORT_EXPIRED`, `SNAPSHOT_NOT_READY`, `PDF_RETRY_NOT_ALLOWED`; a principal without the role gets `auth.ErrForbidden`, as `profiles` does.

The syntax of a selection, before any transaction: `profile_id` matches the profile id rule of 3.2; `namespaces` is not empty (`empty`), each matches the namespace pattern of `profiles` (`invalid`), none repeats (`repeated`); `period` passes `config.ValidatePeriod`, `timezone` `config.ValidateTimezone`, `report_profile` is `detailed` or `executive`, `min_severity` is `all`, `attention`, or `critical` (`invalid`). Against the revision: every namespace is in `allowed_namespaces`; each of the four, resolved through `Document.ResolvedDefaults` when nil, is in its permitted list when that list is not empty (`not_permitted`). The request SHA-256 is that of `json.Marshal(sel)` (the field order of the struct; a nil field is `null`), lower-case hex. `Resolved.Namespaces` keep the operator's order.

**Tests:**

| Test | Input | Expected |
|---|---|---|
| `TestRequestStoresAndQueues` | an operator, key `k1`, a valid selection | `Accepted` queued, waiting, `expires_at = now + 7 days`; one report row with the current revision and `collect_job_id` equal to the job's id; one idempotency row; one `report.submit` row with `profile_id` and `revision_id`; one River `collect` job with the report id, queue `reports`, priority 2, `max_attempts` 3 |
| `TestRequestReplays` | the same key and body twice | the same id, `Replayed` true the second time; one row of each kind |
| `TestRequestReplayWhileTheQueueIsFull` | the limit reached after the first request | the replay is accepted |
| `TestRequestSameKeyAnotherBody` | the same key, another namespace | `IDEMPOTENCY_KEY_REUSED`; nothing written |
| `TestRequestSameKeyConcurrently` | 20 goroutines, one key, one body | 1 report, 1 job, 1 audit row; 19 `Replayed` |
| `TestRequestKeyRules` | empty; 129 characters; `a b`; 128 of `[A-Za-z0-9._:-]` | invalid; invalid; invalid; accepted |
| `TestRequestSyntax` | the table of the syntax rules, one case per word | `REQUEST_INVALID` with the one field and word |
| `TestRequestProfileStates` | absent id; a deleted profile; an inactive current revision | `PROFILE_NOT_FOUND`; `PROFILE_NOT_FOUND`; `PROFILE_INACTIVE` |
| `TestRequestSelectionNotPermitted` | a namespace outside `allowed_namespaces`; `period = "7d"` with permitted `["24h"]`; nil period with permitted `["7d"]` and default `7d` | `{"namespaces":"not_permitted"}`; `{"period":"not_permitted"}`; accepted with period `7d` |
| `TestAdmissionLimits` | limits 2 and 1; two profiles | second request of `prod` `ADMISSION_PROFILE_LIMIT`; third overall `ADMISSION_GLOBAL_LIMIT`; `RetryAfter` 5 s; a collected or failed report never counts |
| `TestAdmissionConcurrentExactlyLimit` | limit 5, profile limit 5, 30 goroutines with distinct keys | 5 accepted, 25 `ADMISSION_GLOBAL_LIMIT` |
| `TestRequestRefusesAReader` | a reader | `auth.ErrForbidden`; nothing written |
| `TestRequestRollsBackOnAQueueFailure` | a closed queue client | an error; no report, no idempotency row, no audit row |
| `TestRequestTakesTheRequestLock` | a test transaction holding `store.LockReportRequests`; one request | the request waits until that transaction commits |
| `TestRetryPDFMatrix` | absent; expired; queued; collected with the row pending; ready; failed | `REPORT_NOT_FOUND`; `REPORT_EXPIRED`; `SNAPSHOT_NOT_READY`; nil with nothing written; `PDF_RETRY_NOT_ALLOWED`; nil with the row pending and its `job_id` the new job's, the report phase `rendering` and `completed_at` null, one `render` job at priority 1, one `report.pdf_retry` row with the language |
| `TestRetryPDFTwiceAtOnce` | 10 goroutines on one failed row | one job and one audit row; the others nil |

- [ ] Steps: tests red, implement, `make lint`, the tagged run of `./internal/platform/...`; commit `feat(reports): add the request and the retry` (44 characters).

**Done when:** the tests pass twice.

---

### Task 8: The collect job (R124, R125, R126, R127, R141)

**Executor:** Subagent, against the running server.

**Files:**
- Create: `internal/platform/worker/collect.go`, `collect_test.go`, `worker.go` (the shared dependencies)

**Interfaces:**

```go
package worker

// Deps are what every handler of the worker reads.
type Deps struct {
	Pool      *pgxpool.Pool
	Profiles  *profiles.Service
	Settings  settings.Settings
	Toolchain typst.Toolchain
	TempDir   string
	Version   string
	Now       func() time.Time
	Logger    *slog.Logger // writes through the run's redactor
}

// ErrStaleAttempt is store.ErrStaleAttempt, returned when a newer attempt owns the report.
var ErrStaleAttempt = store.ErrStaleAttempt

type CollectWorker struct {
	river.WorkerDefaults[reports.CollectArgs]
	Deps Deps
}

func (w *CollectWorker) Timeout(*river.Job[reports.CollectArgs]) time.Duration // Settings.CollectTimeout
func (w *CollectWorker) Work(ctx context.Context, job *river.Job[reports.CollectArgs]) error
```

`Work`, in order:

1. One transaction: `GetReportForUpdate`; a report that is `collected`, `failed`, or expired completes the job and returns nil; else `ClaimReport(id, job.Attempt, now)`; commit.
2. `Profiles.Revision(report.ProfileRevisionID)`; decode `report.Request` as `reports.Resolved`; `cfg := rev.Document.RunConfig(r.Period, r.Timezone, r.ReportProfile, r.MinSeverity, r.Namespaces)`, where `r` is the decoded `Resolved`; `Profiles.Source(rev.Document, cfg.Metrics)`.
3. `emittedAt := Now()`; `collect.NewWindow(cfg.Analysis, emittedAt)`; `pipeline.Collect` with `NewID` returning the report id, `Version`, and a `Step` that calls `AdvanceReport` (a `ErrStaleAttempt` stops the run).
4. `snapshot.Encode`; over 32 MiB is `SNAPSHOT_TOO_LARGE`.
5. One transaction: `CollectReport`; `InsertSnapshot`; for each language of `catalog.Languages()`, a `render` job by `InsertTx` with `reports.RenderInsertOpts` and `InsertPendingPDF` with that job's id; `report.collect` succeeded with metadata `{}`; `river.JobCompleteTx`; commit; return nil.

A failure is classified: `profiles.Classify`, then `collect.ErrNoUsefulSections` as `NOTHING_TO_REPORT`, a `context.DeadlineExceeded` of the job's context as `COLLECTION_TIMEOUT`, anything else `INTERNAL_FAILURE`. A `context.Canceled` with no deadline passed (the worker is stopping) returns `ctx.Err()` with nothing written, so River restores the attempt. A retried code with `job.Attempt < job.MaxAttempts`: `RequeueReport` in a transaction under `context.WithoutCancel`, log, return `fmt.Errorf("collect: %s", code)`. Any other: one transaction under `context.WithoutCancel`: `FailReport`, `report.collect` failed with `{"code"}`, `river.JobCompleteTx`, commit, return nil. `ErrStaleAttempt` at any write: roll back, log, return it. Every log line names the report id and the code and carries the cause through `Logger`; no cause reaches a row or River.

**Tests.** `river.JobCompleteTx` works only inside `Work` with the context River passes (`river.md` section 4), so every test runs a real River client: a helper of the test builds one with `TestOnly`, the queue `reports` with one worker, and the workers this task has; the job is inserted by `reports.Service.Request`, and the test waits, at most 30 s, for the job's state through `client.JobGet`. Three unexported test hooks, `beforeClaim` (called before the claim's transaction), `afterClaim`, and `beforeCollected` (called after the snapshot is encoded, before its transaction), hold an attempt where a test needs it, as E111 did in 3.2.

| Test | Input | Expected |
|---|---|---|
| `TestCollectStoresTheSnapshot` | fake Prometheus of `direct-complete` on the loopback, policy `127.0.0.1/32`, fixed clock | report `collected`, phase `rendering`; snapshot body equals `snapshot.Encode` of `pipeline.Collect` run on a fresh server of the scenario with the same configuration, clock, and id; two PDF rows `pending`; two `render` jobs, priority 1; one `report.collect` succeeded; the job `completed` |
| `TestCollectPhases` | the same, a hook on `AdvanceReport` recording | `collecting`, `analyzing`, `snapshotting` |
| `TestCollectTerminalCodes` | an absent credential file; a server answering 401; policy without the loopback; a scenario with nothing to report | `PROFILE_CREDENTIAL_UNAVAILABLE`, `ENDPOINT_AUTH_REJECTED`, `ENDPOINT_ADDRESS_REJECTED`, `NOTHING_TO_REPORT`; each `failed`, `complete`, one `report.collect` failed with the code, the job `completed` on attempt 1 |
| `TestCollectRetriesAnUnreachableSource` | a closed port in the policy, `job_max_attempts = 2` | attempt 1: report `queued`, `waiting`, job `retryable` with error `collect: ENDPOINT_UNREACHABLE`; attempt 2: `failed` with that code |
| `TestCollectTimeout` | a server that holds every answer, `collect_timeout_seconds` overridden to 1 s through `Timeout`, attempts 3 | `failed`, `COLLECTION_TIMEOUT` on attempt 1, the job `completed` (a deadline is not retried, R125) |
| `TestCollectLateClaimLoses` | hold attempt 1 at `beforeClaim`; the test claims with attempt 2 through `store.ClaimReport`; release | attempt 1 returns `ErrStaleAttempt`; the report keeps attempt 2; the job not completed by attempt 1 |
| `TestCollectStaleAttemptWritesNothing` | hold attempt 1 at `beforeCollected`; the test calls `store.ClaimReport` with attempt 2, as a rescued second attempt would; release | `ErrStaleAttempt`; no snapshot, no PDF row, no audit row from attempt 1; the job not completed by attempt 1 |
| `TestCollectRunsAnInactiveRevision` | the profile deactivated after the request | `collected` |
| `TestCollectNeverStoresTheToken` | a credential `s3cr3t-t0ken-value`, an unreachable source, attempts 2 | the token appears in no column of `reports`, `report_snapshots`, `audit_events`, `river_job` (args and errors), and in no line of the logger's buffer |
| `TestCollectCanceledReturnsTheAttempt` | the client stopped with `StopAndCancel` while `afterClaim` holds the attempt | `ctx.Err()` returned; the report still `running`; no audit row |
| `TestCollectSkipsATerminalReport` | a report already `failed` | the job completes; nothing else written |

- [ ] Steps: tests red, implement, `make lint`, the tagged run of `./internal/platform/...`; commit `feat(worker): add the collect job` (33 characters).

**Done when:** the tests pass twice.

---

### Task 9: The render job (R124, R125, R127)

**Executor:** Subagent, against the running server, with Typst.

**Files:**
- Create: `internal/platform/worker/render.go`, `render_test.go` (`//go:build postgres && template`)
- Modify: `Makefile` (`test-postgres` runs, after its present command, `go test $(TEST_FLAGS) -tags='postgres template' ./internal/platform/worker/...` with the same DSN, and keeps `postgres-down` and the exit status of the first failure)

**Interfaces:**

```go
type RenderWorker struct {
	river.WorkerDefaults[reports.RenderArgs]
	Deps Deps
}

func (w *RenderWorker) Timeout(*river.Job[reports.RenderArgs]) time.Duration // Settings.RenderTimeout
func (w *RenderWorker) Work(ctx context.Context, job *river.Job[reports.RenderArgs]) error
```

`Work`, in order: one transaction: `GetReportForUpdate`; expired, not `collected`, or the PDF row not `pending` completes the job and returns nil; `GetSnapshot`; commit. Recompute SHA-256 (a mismatch is `INTERNAL_FAILURE`); `snapshot.Decode`; `render.Document(snap, lang, document.Options{})`; `render.Compile(ctx, Toolchain, TempDir, data)` (an error that is not the context's is `PDF_COMPILE_FAILED`); over `render.MaxPDFBytes` is `PDF_TOO_LARGE`. One transaction: `GetReportForUpdate` again (expired now: complete the job, write nothing); `FinishPDF` or `FailPDF`; when `CountPendingPDFs` is 0, `SetReportPhase(complete, now)`; `report.render` with `{"language"}` or `{"language","code"}`; `river.JobCompleteTx`; commit. `INTERNAL_FAILURE` is retried as in Task 8 (nothing to requeue: the row stays `pending`); the context's deadline is stored at once as `PDF_COMPILE_FAILED` (R125); `context.Canceled` returns with nothing written.

**Tests:**

| Test | Input | Expected |
|---|---|---|
| `TestRenderStoresThePDF` | a report collected by Task 8's handler from `direct-complete`; the `en` job | row `ready`; body equals `render.Compile(render.Document(decoded snapshot, en, {}))` run in the test; sha256 and size; report phase still `rendering` |
| `TestRenderCompletesTheReport` | both jobs, one after the other | phase `complete`, `completed_at` set, two `report.render` rows |
| `TestRenderBothLanguagesCompleteOnce` | both jobs at once on a client with two workers, 20 runs | every run ends with both rows `ready`, phase `complete`, and `completed_at` set; no run leaves the report in `rendering` with no row `pending` |
| `TestRenderCompileFailure` | a toolchain whose binary exits 1 | row `failed`, `PDF_COMPILE_FAILED`; the other language untouched; with it already `ready`, phase `complete` |
| `TestRenderTimeout` | a toolchain script that sleeps past `render_timeout_seconds` overridden to 1 s through `Timeout`, attempts 3 | row `failed`, `PDF_COMPILE_FAILED` on attempt 1, the job `completed` |
| `TestRenderTooLarge` | a toolchain script writing `%PDF-` followed by 64 MiB plus one byte | `PDF_TOO_LARGE` |
| `TestRenderDigestMismatch` | the snapshot body altered by a test-only statement after the trigger is disabled in the test's own database | `INTERNAL_FAILURE` after the last attempt |
| `TestRenderSkipsAnExpiredReport` | `expired_at` set before the job runs | the job completes; no row written |
| `TestRenderAfterARetry` | a failed row retried through `reports.Service.RetryPDF` | `ready`; phase `complete` |

- [ ] Steps: tests red, implement, `make lint`, `KCR_TEST_DATABASE_DSN=... go test -count=1 -tags='postgres template' ./internal/platform/worker/`; commit `feat(worker): add the render job` (32 characters).

**Done when:** the tests pass twice and the Makefile change is read by the main thread.

---

### Task 10: The maintenance job: reconcile, expiry, purge, and the old rows (R128, R129, R132, R142)

**Executor:** Subagent, against the running server.

**Files:**
- Create: `internal/platform/store/retention.go`, `retention_test.go`
- Create: `internal/platform/worker/maintenance.go`, `maintenance_test.go`

**Interfaces:**

```go
// store
func ReconcileReports(ctx context.Context, tx pgx.Tx, now time.Time, limit int) ([]uuid.UUID, error)
	// queued or running, collect_job_id absent from river_job or discarded, cancelled, completed;
	// FOR UPDATE SKIP LOCKED; failed, complete, JOB_ATTEMPTS_EXHAUSTED, completed_at, updated_at
func ReconcilePDFs(ctx context.Context, tx pgx.Tx, now time.Time, limit int) ([]PDFKey, error)
	// pending, job_id absent from river_job or discarded, cancelled, completed; the report locked
	// FOR UPDATE SKIP LOCKED first; failed, PDF_ATTEMPTS_EXHAUSTED; the report to complete
	// with completed_at when no row is pending
type PDFKey struct {
	ReportID uuid.UUID
	Language string
}
func ExpireDueReports(ctx context.Context, tx pgx.Tx, now time.Time, limit int) ([]uuid.UUID, error)
	// collected or failed, expires_at <= now, expired_at null; oldest first; FOR UPDATE SKIP LOCKED;
	// deletes their snapshot and PDF rows; sets expired_at and updated_at
func PurgeExpiredReports(ctx context.Context, tx pgx.Tx, now time.Time, kept time.Duration, limit int) ([]uuid.UUID, error)
	// expired_at set, expires_at + kept <= now; deletes the idempotency rows, then the reports
func PurgeThrottle(ctx context.Context, q Querier, now time.Time, window time.Duration, limit int) (int64, error)
func PurgeSessions(ctx context.Context, tx pgx.Tx, now time.Time, ttl time.Duration, limit int) (int64, error)

// worker
type MaintenanceArgs struct{}
func (MaintenanceArgs) Kind() string { return "maintenance" }

type MaintenanceWorker struct {
	river.WorkerDefaults[MaintenanceArgs]
	Deps Deps
}
func (w *MaintenanceWorker) Work(ctx context.Context, job *river.Job[MaintenanceArgs]) error
```

`Work` reads the clock once and runs five steps, each in its own transaction, in order: reconcile (100 reports, then 100 PDF rows, R142), with one `report.collect` failed row `{"code":"JOB_ATTEMPTS_EXHAUSTED"}` per report and one `report.render` failed row `{"language","code":"PDF_ATTEMPTS_EXHAUSTED"}` per PDF row; expire (100), with one `report.expire` row per report; purge (100), with one `report.purge` row per report; the throttle (1000), with the window of the sign-in throttle of `internal/platform/auth` as one constant shared with it; the sessions (1000). A failed step is logged and the next one still runs; the job returns the first error. Before writing the session query the implementer reads every use of `rotated_from_id` and stops and reports if a reader would mistake a purged row for a fresh one.

**Tests:**

| Test | Input | Expected |
|---|---|---|
| `TestReconcileReports` | a running report whose job a test statement sets `discarded`; one whose job is `cancelled`; one whose job row is deleted; one queued whose job is `available`; one running whose job is `running`; one `retryable`; one `scheduled` | the first three `failed`, `JOB_ATTEMPTS_EXHAUSTED`; the other four untouched |
| `TestReconcilePDFs` | a collected report in `rendering` with the `en` row `pending` and its job `discarded`, the `pt-BR` row `ready`; another with one row `pending` on a `discarded` job and the other `pending` on an `available` one | the first: `en` `failed` with `PDF_ATTEMPTS_EXHAUSTED`, the report `complete` with `completed_at`; the second: one row failed, the report still `rendering` |
| `TestReconcileSkipsALockedReport` | tx1 holds a due report `FOR UPDATE` | that report left, the others reconciled |
| `TestPanicAtTheLastAttemptIsReconciled` | a collect handler whose `afterClaim` hook panics, `job_max_attempts = 1`, a real client; then one maintenance run | the job `discarded`; the report `failed`, `JOB_ATTEMPTS_EXHAUSTED`, one `report.collect` failed row; the active count of the profile back to 0 |
| `TestExpireDueReports` | a collected report with PDFs at `expires_at`; one a second later; one running past its deadline; one already expired | the first expired (snapshot and PDF rows gone, `expired_at = now`); the others untouched |
| `TestExpireBatchOrder` | 3 due, limit 2 | the two oldest |
| `TestExpireSkipsALockedRow` | tx1 holds one due row `FOR UPDATE`; expire | the other rows expire; the locked one is left |
| `TestPurgeExpiredReports` | expired at `expires_at`, `kept` 30 days; now at 30 days minus a second, then at 30 days | untouched; then the row and its idempotency row gone |
| `TestPurgeThrottle` | a row whose window ended with no block; one blocked until later; one in its window | the first deleted only |
| `TestPurgeSessions` | revoked longer than the ttl ago; revoked a second short of it; expired longer ago; live; a live one rotated from a purged one | the first and third deleted; the live rotated one keeps its row with `rotated_from_id` null |
| `TestMaintenanceRunsEveryStep` | one of each kind due | the five effects; one `report.collect` failed, one `report.expire`, and one `report.purge`, each with no actor |
| `TestMaintenanceContinuesAfterAFailure` | the throttle step failing (a closed connection injected for it) | the session step still runs; the error returned |
| `TestExpiryBeforeRenderWritesNoPDF` | a render job held after its first transaction; expire the report; release | no PDF row; the job completed |

- [ ] Steps: tests red, implement, `make lint`, the tagged run of `./internal/platform/...`; commit `feat(worker): add the maintenance job` (37 characters).

**Done when:** the tests pass twice.

---

### Task 11: The health routes, the client, and the `worker` command (R123, R134, R135, R141)

**Executor:** Subagent, against the running server.

**Files:**
- Create: `internal/platform/health/health.go`, `health_test.go`
- Modify: `internal/platform/api/health.go` (goes), `server.go` (mounts `health.Handler`), `api_test.go` if it names the methods
- Create: `internal/platform/worker/client.go`, `client_test.go`
- Create: `internal/cli/worker.go`, `worker_test.go`
- Modify: `internal/cli/run.go` (registers `worker`), `root_test.go` (the seven lists); the `profiles.Service` that `worker` builds sets `Reveal` as `serve` and `admin` do since Task 3

**Interfaces:**

```go
package health

type Check struct {
	Name string
	Run  func(ctx context.Context) bool
}

// Handler answers GET /health/live with 200 {"status":"live"} and GET
// /health/ready with 200 or 503 and the result of every check, run within
// two seconds; any other path is 404 {"status":"not_found"}.
func Handler(checks []Check) http.Handler

func Database(pool *pgxpool.Pool) Check   // SELECT 1
func Migrations(pool *pgxpool.Pool) Check // store.HasPending is false
```

```go
package worker

type Config struct {
	Deps   Deps
	Slots  int
	Test   bool // Config.TestOnly of River
}

// NewClient returns the River client of the worker: the queue reports with
// Slots workers, the queue maintenance with one, the three workers, the
// periodic maintenance job every minute, the timeouts, the rescue after the
// larger timeout plus one minute, the reindexer off, the logger of Deps.
func NewClient(c Config) (*river.Client[pgx.Tx], error)

// Running is a health check: the client started and has not begun to stop.
func Running(client *river.Client[pgx.Tx]) health.Check
```

`internal/cli/worker.go` builds the worker through one function, `buildWorker(pool, loaded, tc, env, logger) (*river.Client[pgx.Tx], error)`, which the command and its wiring test both call. The command follows R134; its log lines are `worker: started with <n> slots`, `worker: stopping`, `worker: stopped`. The pool's `MaxConns` is the larger of the DSN's value and `worker_slots + 8`.

**Tests:**

| Test | Input | Expected |
|---|---|---|
| `TestHealthLive`, `TestHealthReady`, `TestHealthNotFound` (health) | checks returning true and false | 200 live; 200 ready with both checks; 503 `not_ready` with the false one named; 404 |
| `TestHealthReadyBound` | a check that blocks | 503 within two seconds and a half |
| every test of `internal/platform/api` | unchanged bodies of the two routes | pass |
| `TestWorkerWiresTheJobs` (cli, postgres) | `buildWorker` with the fixture settings | the client holds the kinds `collect`, `render`, `maintenance`, the queue `reports` with `worker_slots` workers and `maintenance` with 1; inserting a `collect` job for a requested report and starting the client collects it against the fake Prometheus (the proof that the command's client runs what the service queues) |
| `TestWorkerStopReturnsTheJob` (cli, postgres) | a fake Prometheus that holds its answers; `worker_drain_seconds = 1`; cancel the command's context during the collectors | exit 0 within 3 s; the job `available`, attempt restored; the report `running`; a second run collects it |
| `TestWorkerCommandErrors` (cli) | a bad settings file; `KCR_TYPST` pointing at nothing; an unreachable database | exit 2; exit 1; exit 1, each one line `error: ...` and nothing on standard output |
| `TestWorkerHealthAnswers` (cli, postgres) | a running command on a free port | `/health/ready` 200 with `database`, `migrations`, `worker` |
| `TestWorkerStopAbandonsAHungJob` (cli, postgres) | a collect handler hook that ignores its context; `worker_drain_seconds = 1`; cancel the command's context | exit 1 within 4 s with the one line `error: jobs did not stop`; nothing on standard output |
| `TestWorkerRevealsACredential` (cli, postgres) | a collection through the client `buildWorker` returns, with a credential file | the credential is in the run's redactor |
| `TestCommandsAreTheDesignOnes` and the other lists of `root_test.go` | `worker` added | pass |

- [ ] Steps: tests red, implement, `make lint`, the full tagged run (`./internal/platform/... ./internal/cli/...`, and `postgres template` over the worker); commit `feat(cli): add the worker command` (33 characters).

**Done when:** the full tagged run passes twice and a run of `bin/k8s-capacity-report worker --help` lists its two flags.

---

### Task 12: The proof, the documents, review, gate, close

**Executor:** Subagent for Step 1; main thread for the rest.

- [ ] **Step 1 (subagent):** `TestReportEndToEnd` in `internal/platform/worker/end_to_end_test.go` (`//go:build postgres && template`): an operator requests a report of `direct-complete` through `reports.Service.Request`; the client of `worker.NewClient` runs in the test process against the fake Prometheus admitted by `127.0.0.1/32`, with a settable clock; the test waits, at most 60 s, until the report is `collected` with phase `complete`; the snapshot body equals `snapshot.Encode` of `pipeline.Collect` run with the same inputs on a fresh server; each PDF equals `render.Compile` of `render.Document` of that snapshot; the clock moves to `expires_at` and one maintenance run expires the report (no snapshot, no PDF row, `expired_at` set); the clock moves 30 days further and one run purges it; the audit rows, in order, are `report.submit`, `report.collect`, `report.render` twice, `report.expire`, `report.purge`. Commit `test(worker): prove a report end to end` (39 characters).
- [ ] **Step 2:** `AGENTS.md`: the layer table complete (it lacks 105 and 145 today, and gains 75, 100, 128, 146, 160); the command list gains `worker`; "The platform" gains River (the goose file, the drift test, the `now()` of River's tables as the one exception), the report states, the attempt rule (a claim never lowers the attempt), the request lock `72427302`, the reconcile step and its two codes, the stored bytes and caps, retention, the deployment keys of this part; the rule "a file that the compiler wrote is renamed only when it starts as a PDF does" becomes "PDF bytes are written only when they start as a PDF does"; the build tag table gains `postgres template` over the worker.
- [ ] **Step 3:** `README.md`: the `worker` command, its flags and health, the ten keys, the retention.
- [ ] **Step 4:** `CHANGELOG.md`, `### Added` under Unreleased: the `worker` command, which runs the report jobs with two slots by default and answers health on port 8091; sign-in records whose block is over and sessions ended longer than their lifetime ago are removed. `### Changed`: `admin import-profiles` reports `recreated` for a profile deleted through the API; three IPv6 forms that embed an IPv4 address are refused by the outbound policy; the profile test gives up after two minutes. `### Fixed`: nothing. Omitted and said so in the report: what a user sees of a report (the request, admission, idempotency, one PDF per language, retention and expiry), which reaches users with the routes of 3.4 and enters its changelog; the snapshot freeze (no user reads a snapshot file of another shape); the shared run and render (no behaviour changes); the 253-character cap of a host name (such a name never resolved).
- [ ] **Step 5:** the adversarial review of the branch (`.cache/phase-3/part-3/review/`), with the attempt rule, the transactions with River, admission and idempotency under concurrency, expiry against rendering, and the redaction as its focus, probes under `review/probe/`; one fix wave; one re-review.
- [ ] **Step 6:** `make gate`; logs to `.cache/phase-3/part-3/gate/`; read that `TestReportEndToEnd` is named in the log of `test-postgres`.
- [ ] **Step 7:** `ROADMAP.md` updated; the closing report; the owner's word to merge by fast-forward, no version, no tag.

**Done when:** the gate passes from images built without cache, the review is clean, and the owner has the closing report.
