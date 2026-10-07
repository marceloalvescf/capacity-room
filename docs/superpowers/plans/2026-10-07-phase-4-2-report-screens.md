# Phase 4.2: The Report Screens Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** An operator requests a report from the browser, watches it run, downloads its PDF in the language of the request, asks for the other language, and deletes it; a reader follows and downloads. Before that, a request names the language of its report, the worker makes that one PDF, the other is made on request, and `serve` no longer fails its stop on a stalled download (F-128).

**Architecture:** The backend changes are small and sit where they already live: `reports.Selection` gains `language`, the collect handler queues one render, and `Service.RequestPDF` replaces `RetryPDF` behind `POST /reports/{id}/pdfs`. The frontend adds three screens under `web/src/features/reports/` that share one module of queries (`api.ts`), built on a few new primitives in `web/src/components/ui/`. The integration stack gains the fake Prometheus, built from its own stage of the Containerfile, and one fixture profile, so Playwright requests a real report.

**Tech Stack:** As in part 4.1, with no new dependency: Go 1.27; React 19.3.0, React Router 8.4.0, TanStack Query 5.104.1, React Hook Form 7.89.0 with Zod 4.6.5, Radix UI 1.7.0 (`Popover`, `AlertDialog`), Tailwind CSS 4.3.3, react-i18next 17.0.16; Vitest 5.0.3 with MSW 3.0.2; Playwright 1.63.0.

**Spec:** `docs/superpowers/specs/2026-09-27-k8s-capacity-report-design.md` as amended in `10416ec` (approved by the owner on 2026-10-07): sections 4.3, 11.1, 11.3, 11.7, 11.10, 12.2, 12.3, 12.4, 12.5, 12.6, 13.4, 13.5, 14.2. The decisions behind the amendment, with the mockups the owner chose, are in the companion pages under `.cache/phase-4/brainstorm/.superpowers/brainstorm/9007-1791377823/content/` (`section-1.html` is the screens as approved; `section-3.html` the build; `section-4.html` the browser test). The original frontend, read-only, is `older/kubernetes-report/frontend/src/features/{history,generate,reports}/`.

**Depends on:** Part 4.1, merged into `main` at `aa55455` and pushed on 2026-10-07. Branch `feat/report-screens`, opened from `aa55455`.

## Global Constraints

Every constraint of the plans of phases 0 to 4.1 stands, and `AGENTS.md` binds every task. The ones this part leans on most:

- **No backward compatibility** (the owner's rule): a renamed route, code, or audit action leaves no alias; a stored request without a language gets no fallback (R182).
- **Every text the reader sees, and every accessible name, comes from `web/src/locales/{en,pt-BR}.json`.** Task 4 carries every word of this part; an implementer transcribes and never invents one. A missing text is a stop-and-report.
- **Technical terms are English in both languages**, and no locale entry holds a term of `internal/catalog/banned-terms.json` (in Portuguese: `limite`, `nó`, `nós`, `solicitação`, among others).
- **No inline style**, no `<style>` added at run time; the CSP stays `script-src 'self'` and `style-src 'self'`, with no hash added in this part (R189).
- **Status is a shape, then a label, then a colour**, with the PDF's five shapes (spec 12.2 as amended; the single circle waits as F-130).
- **Names and numbers use the monospaced font**; motion only under `motion-safe:`.
- **Contrast:** text pairs at least 4.5:1, a large heading, an input border, and a focus ring at least 3:1, in both themes; every new pair enters `web/src/theme/pairs.ts`.
- **A ratio of anything is not computed here**; the screens show what the API answers.
- **Commits:** `type(scope): description`, imperative, under 50 characters; every subject below is counted. Stage with `git add <paths>`.
- **Diagnostics are English**; comments never speak of the process.
- Scratch goes under `.cache/phase-4/part-2/`.

## Review Focus

Each line names an input the spec implies and no requirement spells out, and the test that pins it.

1. **A report deleted elsewhere while its progress page is open.** The next check answers 404: the page shows `progress.notFound`, stops checking, and offers the way back to the list. Pinned in Task 7 (`a report deleted elsewhere`).
2. **A double click on "Request report", or a resend after a network failure.** One report is created: the button is disabled while the request is in flight, and a resend with unchanged values carries the same `Idempotency-Key`, which the server replays. Pinned in Task 6 (`one report per attempt`).
3. **A typed period at the edge:** `0h`, `01h`, `1H`, `1.5h`, `36`, `h`, an empty value, and ` 36h ` with spaces. Only ` 36h ` is accepted, trimmed to `36h`; the others show `period.invalid` and Apply stays disabled. Pinned in Task 6 (`period grammar`).
4. **The interface language switched on the progress page.** The PDF block follows at once: the PDF of the new language comes first, the other below. Pinned in Task 7 (`the PDF block follows the language`).
5. **"Load more" while the list checks by itself.** A check refetches every loaded page and keeps them; no row is lost or repeated, and the selection keeps only rows still shown. Pinned in Task 5 (`polling keeps the loaded pages`).

## Where this part sits

| Part | Delivers | Proof at the end |
|---|---|---|
| 4.1 Foundation | Toolchain, serving, theme, i18n, session, sign-in, account | Done (`aa55455`) |
| **4.2 Reports (this plan)** | Report list, new report form, progress with the PDFs, delete; a request names its language; the other language on request; F-128, F-129 | In a real browser: a report requested, followed, downloaded, its other language generated, and deleted; a reader downloads |
| 4.3 Calendar ranges (F-131) | A period with a start and an end chosen on a calendar | Owner's decision of 2026-10-07: the part right after 4.2 |
| 4.4 The report | The seven tabs; the PDF block moves to the report header | Every tab of a golden scenario in both languages |
| 4.5 Administration | Profiles and users | An administrator creates a profile and a user from the browser |
| 4.6 End-to-end | Security, concurrency, and PDF recovery specs | `make e2e` runs the four specs of spec 13.4 |

### Out of this part

| Left out | Arrives in | Reason |
|---|---|---|
| Calendar ranges | 4.3 | F-131, the owner's order |
| Filters on the report list | none | The owner chose no filter (2026-10-07) |
| Undo after a delete | none | The owner chose the dialog alone |
| One circle for every status | later refinement | F-130: web and PDF change together |
| TanStack Table and sorting | 4.4 | Nothing sorts in this part |
| The compile-failure switch in the integration stack | 4.6 | Only the PDF recovery spec needs it |

## How to read this plan

As in part 4.1: each task gives exact paths, signatures, and a table of tests with inputs and expected results. Write each test first, watch it fail, then implement.

Executors:
- **Subagent:** a Sonnet subagent. It may run `go`, `make build`, `make test`, `make lint`, `make generate`, `make generate-check`, `make web`, `npm` inside `web/`, and the tagged Go tests against the PostgreSQL the main thread started (`KCR_TEST_DATABASE_DSN`). It never runs Docker, `make e2e`, or `make update-*`, and cannot declare a task finished.
- **Main thread:** Docker, judgment, wording, design, and the owner.

Rules for every brief, as in part 4.1:
- Tasks run in sequence unless this plan says otherwise. A reviewer reads a diff file and runs tests only on packages nobody edits.
- Stage with `git add <paths>`; never stash, restore, check out a path, or clean.
- A test of a change goes red first, on the code before the change.
- The harness creates worktrees from `main`: the implementer's first step fast-forwards its worktree to `feat/report-screens`.
- Kill the process that holds the port, not its parent; run the binary by hand before the review.
- A stopped gate is stopped whole (ROADMAP, learned in 4.1).

**Order:**
- Task 1, then Task 2: both edit `internal/platform/reports`, `internal/platform/api`, and `api/openapi.yaml`.
- Task 3 follows Task 2 (it edits `internal/platform/api/middleware.go` and `reports_test.go`).
- Task 4 follows Task 2: it needs the generated `schema.gen.ts` with `language` and `/reports/{id}/pdfs`. It may run beside Task 3, which touches no file of `web/`.
- Tasks 5, 6, 7 follow Task 4, in that order: each adds its route and its screen to `router.tsx` and `tests/screens.test.tsx`.
- Task 8 follows Task 7. Task 9 (combined run, owner's look) and Task 10 (close) end the part.

## Rulings made on the owner's behalf

Each ruling is a decision the amended spec leaves open: decision, grounds, cost if wrong. The numbering continues from R180.

| ID | Ruling |
|---|---|
| R181 | `reports.Selection` gains `Language *string \`json:"language"\`` and `reports.Resolved` gains `Language string \`json:"language"\``. `checkSyntax` answers `language: missing` when it is nil and `language: invalid` when it is not `pt-BR` or `en`. `requestReport` passes `[]string{"language"}` as required to `DecodeBody`, so a missing key is refused there first. The language is stored in `reports.request` with the rest of the selection, so no migration is needed. The contract's `ReportRequest` gains `language` (required, `Language`), and `ReportSelection` gains `language` (required). The digest of a request is `json.Marshal(Selection)`, so a key reused with another language is a conflict with no further code. Grounds: spec 11.7 as amended; `reports.request` is jsonb (`00004_reports.sql`). Cost if wrong: none found. |
| R182 | The collect handler queues one render and inserts one pending PDF row, for `Resolved.Language`. A stored request whose language is not `pt-BR` or `en` (a report queued before this part) fails the collection with `INTERNAL_FAILURE`, logged with the report's id; there is no fallback. Grounds: the owner's rule against compatibility layers. Cost if wrong: a report queued across the upgrade fails and is requested again. |
| R183 | `Service.RequestPDF(ctx, actor, reportID, language) error` replaces `RetryPDF`. Under the report's row lock: no row inserts one pending row with a new render job; a failed row is retried as today; a pending row is accepted and changes nothing; a ready row is `PDF_ALREADY_READY` (409, "This PDF is already ready."). Every path that queues sets the phase to `rendering` and appends `report.pdf_request` with `{"language"}`. `PDF_RETRY_NOT_ALLOWED`, `report.pdf_retry`, `RetryPDF`, `PDFRetryRequest`, `PDFRetryAccepted`, and `/pdf-retries` leave the code, the contract, and the locales. The route is `POST /reports/{id}/pdfs`, operationId `requestReportPDF`, body `PDFRequest {language}`, answer 202 `PDFRequestAccepted {language, status: pending}`. Grounds: spec 11.7 and 11.10 as amended. Cost if wrong: none found. |
| R184 | F-128. `serve`'s stop calls `Shutdown` with 30 seconds. When that runs out, it calls `Close`, logs one warning `serve: connections closed before they finished: N` (N counted by a `ConnState` hook: connections not yet `StateClosed` or `StateHijacked`), and returns nil, so the exit is 0. The timeout is a parameter of the stop function, 30 seconds from the command, so a test runs it in 200 ms. Grounds: spec 11.1 as amended. Cost if wrong: none found. |
| R185 | F-129. `TestStoredSnapshotChangedUnderItsDigestAnswers500` changes one byte that keeps the JSON valid (a digit of a number, chosen by the test and asserted to parse), so both halves prove the digest check. The debug line of a request cancelled by its client gains `"error", err`. Grounds: the finding. Cost if wrong: none found. |
| R186 | `ApiFailure` gains `readonly fields: Readonly<Record<string, string>>`, read from `error.fields` (empty when absent). The form marks a field from `REQUEST_INVALID` and `REPORT_SELECTION_NOT_PERMITTED` with `validation.invalid` or `validation.notPermitted`; a field the form does not show falls back to the alert of the code. Grounds: spec 12.3 (the profile's limits); the original mapped server fields. Cost if wrong: none found. |
| R187 | Dates show as `YYYY-MM-DD HH:mm` in the browser's time zone, in the monospaced font, in both languages; elapsed time as `m:ss` under an hour and `h:mm:ss` from an hour. One module, `web/src/lib/format.ts`, holds both. Grounds: spec 12.5 (numbers mono); the mockups the owner approved. Cost if wrong: one module. |
| R188 | A period's words. The six quick ranges have fixed keys (`period.quick.1h` and the others). A value equal to a quick range shows its words; any other value shows `period.custom.value` with `{{duration}}` from `Intl.NumberFormat(language, {style: "unit", unit: "minute" \| "hour" \| "day", unitDisplay: "long"})`, which agrees in number in both languages. Grounds: Portuguese "últimos/últimas" agrees in gender with the unit, which a single sentence cannot; the phrase chosen ("… até agora") needs no agreement. Cost if wrong: two keys. |
| R189 | The selects are the browser's own `<select>`, styled; no Radix Select, so no style hash enters the CSP. The time zones are the profile's `permitted_timezones`, or, when that list is empty, `Intl.supportedValuesOf("timeZone")` with the profile's default added when the browser lacks it. Popover and AlertDialog join `no-style.test.tsx`. Grounds: spec 12.3 as amended; R172. Cost if wrong: none found. |
| R190 | Checking. The list refetches every 5 seconds while a loaded row is `queued` or `running` or has a `pending` PDF, and not otherwise (`refetchInterval` returns `false`). The progress page refetches after 1.5 s for the first 20 checks, 3 s for the next 20, then 6 s, while the report is queued or running or has a pending PDF; a 404 or 410 stops it. Grounds: spec 11.7 as amended; the original's backoff. Cost if wrong: none found. |
| R191 | Who sees what. A checkbox and Delete appear on a row only when the principal may delete it: an administrator any, an operator their own (`owner_id === principal.id`), a reader none. Generate PDF and Try again appear for operators and administrators. A bulk outcome `forbidden` keeps those rows selected and shows `reports.deleteForbidden`; `deleted`, `not_found`, and `expired` count as gone. A bulk selection of one row opens the one-report dialog. Grounds: spec 11.3; the original. Cost if wrong: none found. |
| R192 | The form starts with no namespace checked and requires one (`validation.namespaces`); the language starts on the interface language; the four options start on the profile's defaults, and choosing another profile resets namespaces and options. `?from=<id>` loads that report and fills profile, namespaces, language, and options from its selection, then behaves as a fresh form. Grounds: the original; spec 12.3. Cost if wrong: none found. |
| R193 | The idempotency key is `crypto.randomUUID()`, made when the form opens. It is renewed when a value changes after an attempt reached the server (any answer); an attempt that never got an answer keeps it, so a resend replays. `IDEMPOTENCY_KEY_REUSED` renews it and shows the code's alert. Grounds: spec 12.3 as amended; the original. Cost if wrong: none found. |
| R194 | `StatusMark` gains `label?: string`, printed in place of `status.<status>`. The marks: `queued`, `running`, a `pending` PDF: `undefined`; `collected`, a `ready` PDF: `healthy`; `failed` (report or PDF): `critical`; an expired report: `unavailable`. One function, `reportMark(summary)` and `pdfMark(state)`, in `features/reports/marks.ts`. Grounds: spec 12.2 as amended. Cost if wrong: one module. |
| R195 | `Button` gains the variant `danger`: `bg-critical text-surface border-critical`. The pair `text("surface", "critical")` enters `pairs.ts`. Grounds: the dialog the owner chose has a red Delete. Cost if wrong: one pair. |
| R196 | The integration stack. `deploy/Containerfile` gains the stage `fake-prometheus`: built from the `build` stage with `go build ./cmd/fake-prometheus`, copied onto the same distroless base, tagged `k8s-capacity-report-fake-prometheus:$(VERSION)` by `make e2e`, never by `make image`. `compose.integration.yaml` gains the service `fake-prometheus` (`--fixture /fixtures/direct-complete.json --listen :9090`, `testdata/prometheus/` mounted read-only) and a one-shot `profiles` service running `admin import-profiles --file /etc/kcr/profiles.toml`, run by `make e2e` after `bootstrap`. `deploy/platform/profiles.toml` holds one profile (`id = "e2e-prod"`, `display_name = "Production"`, `endpoint_url = "http://fake-prometheus:9090"`, `endpoint_type = "prometheus"`, `cluster_name = "prod"`, `allowed_namespaces = ["payments"]`, `active = true`), and `deploy/platform/platform.toml` gains `allowed_endpoint_hostnames = ["fake-prometheus"]`. The spec creates its operator and reader through the admin API. Grounds: spec 14.2 ("adds the fake Prometheus, fixture profiles"); the profile of `internal/cli/api_end_to_end_test.go`. Cost if wrong: none found. |
| R197 | The CSP and axe guard of `e2e/sign-in.spec.ts` moves to `e2e/fixtures.ts`, exporting `test` and `expectNoViolations(page)`; both specs import it. Grounds: two specs share it. Cost if wrong: none found. |

## Findings this plan closes

| Finding | How |
|---|---|
| F-128 | Task 3 (R184) |
| F-129 | Task 3 (R185) |

## File structure

| Path | Responsibility |
|---|---|
| `internal/platform/reports/{service.go,request.go,read.go}` and tests; `internal/platform/worker/collect.go` and tests; `internal/platform/api/{reports.go,routes.go,errors.go}` and tests; `api/openapi.yaml`; generated files | The language of a request; one PDF (Task 1) |
| `internal/platform/reports/{service.go}`, `retry_test.go` → `pdfs_test.go`; `internal/platform/store/reports.go`; `internal/platform/api/*`; `api/openapi.yaml` | `RequestPDF` and `POST /reports/{id}/pdfs` (Task 2) |
| `internal/cli/serve.go`, `serve_test.go`; `internal/platform/api/middleware.go`, `reports_test.go` | F-128 and F-129 (Task 3) |
| `web/src/api/client.ts`; `web/src/components/ui/{button,select,checkbox,segmented,confirm-dialog,popover}.tsx`; `web/src/components/status-mark.tsx`; `web/src/lib/format.ts`; `web/src/i18n/families.ts`; `web/src/locales/*.json`; `web/src/theme/pairs.ts`; `web/src/features/reports/{api.ts,marks.ts,period.ts}`; `web/src/app/{router.tsx,shell.tsx}`; `web/tests/msw.ts` | The shared pieces and every word (Task 4) |
| `web/src/features/reports/{reports-page.tsx,delete-dialog.tsx}` and tests | The list and the delete dialog (Task 5) |
| `web/src/features/reports/{generate-page.tsx,period-picker.tsx}` and tests | The form and the period picker (Task 6) |
| `web/src/features/reports/{progress-page.tsx,pdf-block.tsx}` and tests | The progress screen (Task 7) |
| `deploy/Containerfile`, `deploy/compose.integration.yaml`, `deploy/platform/{platform.toml,profiles.toml}`, `Makefile`, `web/e2e/{fixtures.ts,reports.spec.ts,sign-in.spec.ts}` | The integration stack and the spec (Task 8) |
| `AGENTS.md`, `README.md`, `CHANGELOG.md` | The documents (Task 10) |

---

### Task 0: Open the part

**Executor:** Main thread.

- [x] **Step 1:** `ROADMAP.md` and `CHANGELOG.md` read and true; `main` at `aa55455`, pushed.
- [x] **Step 2:** `git switch -c feat/report-screens`.
- [x] **Step 3:** The spec amendment, `10416ec`, approved by the owner on 2026-10-07.
- [ ] **Step 4:** `ROADMAP.md`: a section "Phase 4.2" with P42-0 to P42-10, all `OPEN`, and the rulings R181 to R197; the summary line names this plan.
- [ ] **Step 5:** Commit the plan: `docs(plan): add the plan of part 4.2` (36 characters).

**Done when:** the plan is committed and approved by the owner.

---

### Task 1: The language of a request, and one PDF (R181, R182)

**Executor:** Subagent, against the running PostgreSQL.

**Files:**
- Modify: `internal/platform/reports/service.go` (`Selection`, `Resolved`), `internal/platform/reports/request.go` (`checkSyntax`, `resolve`), `internal/platform/worker/collect.go` (the loop near line 266), `internal/platform/api/reports.go` (`requestReport`, `reportOf`), `api/openapi.yaml` (`ReportRequest` near line 1099, `ReportSelection` near line 1082)
- Regenerate: `internal/platform/api/openapi.gen.go`, `web/src/api/schema.gen.ts` (`make generate`)
- Test: `internal/platform/reports/request_test.go`, `internal/platform/worker/collect_test.go`, `internal/platform/api/reports_test.go`, and every existing test that builds a `Selection` or posts `/reports` (each gains `language`)

**Interfaces:**
- Produces: `Selection.Language *string`, `Resolved.Language string`; `Report.selection.language` in the contract and in `schema.gen.ts`; `POST /reports` requires `language`.

- [ ] **Step 1: Write the failing tests**

| Test | Input | Expected |
|---|---|---|
| `TestRequestNeedsALanguage` | a valid selection with `Language` nil | `REQUEST_INVALID`, fields `{"language": "missing"}`; no row |
| `TestRequestRefusesAnUnknownLanguage` | `Language` = `"es"` | `REQUEST_INVALID`, fields `{"language": "invalid"}` |
| `TestRequestStoresTheLanguage` | `Language` = `"en"` | `GET` of the report: `Selection.Language == "en"` |
| `TestKeyReusedWithAnotherLanguage` | one key, first `en`, then `pt-BR`, same other values | the second is `IDEMPOTENCY_KEY_REUSED` |
| `TestCollectQueuesOnePDF` (`worker`) | a report requested in `en`, collected over `direct-complete` | exactly one `report_pdfs` row, language `en`, `pending`; exactly one render job |
| `TestCollectFailsAStoredRequestWithoutLanguage` (`worker`) | a report row whose `request` JSON lacks `language` | the report fails with `INTERNAL_FAILURE`; no PDF row |
| `TestPostReportWithoutLanguage` (`api`) | `POST /reports` body without `language` | 400 `REQUEST_INVALID`, fields `{"language": "missing"}` |
| `TestReportShowsItsLanguage` (`api`) | `GET /reports/{id}` of a report in `pt-BR` | `selection.language == "pt-BR"` |

- [ ] **Step 2:** `go test -count=1 -tags=postgres ./internal/platform/reports/ ./internal/platform/worker/ ./internal/platform/api/ -run 'Language|OnePDF|WithoutLanguage|ItsLanguage|KeyReused'`. Expected: FAIL to compile, then each for its reason.
- [ ] **Step 3:** Implement; then `make generate`.
- [ ] **Step 4:** `make test`, `make lint`, `make generate-check`, and `go test -count=1 -tags=postgres ./internal/platform/... ./internal/cli/...` (with `-tags='postgres template'` for `./internal/platform/worker ./internal/cli`). All pass.
- [ ] **Step 5:** Commit `feat(reports): name the language of a request` (45 characters).

**Done when:** every row passes, `make generate-check` is clean, and no test builds a `Selection` without a language.

---

### Task 2: The PDF of a language, on request (R183)

**Executor:** Subagent, against the running PostgreSQL. Follows Task 1.

**Files:**
- Modify: `internal/platform/reports/service.go` (`RequestPDF` replaces `RetryPDF`; `CodePDFAlreadyReady = "PDF_ALREADY_READY"` replaces `CodePDFRetryNotAllowed`; `ActionReportPDFRequest = "report.pdf_request"` replaces `ActionReportPDFRetry`), `internal/platform/api/{routes.go,reports.go,errors.go}`, `api/openapi.yaml` (the path block near line 749, `PDFRetryRequest`/`PDFRetryAccepted` near 1246, `ErrorCode` near 1308), `web/src/locales/{en,pt-BR}.json` (the `error` group: remove `PDF_RETRY_NOT_ALLOWED`, add `PDF_ALREADY_READY`, words in Task 4's table)
- Rename: `internal/platform/reports/retry_test.go` → `pdfs_test.go`
- Regenerate: both generated files

**Interfaces:**

```go
// RequestPDF makes the PDF of one language of a collected report: it creates
// the PDF when it was never asked for, compiles it again when it failed, and
// changes nothing while it is pending.
func (s *Service) RequestPDF(ctx context.Context, actor auth.Principal, reportID uuid.UUID, language string) error
```

- `problems[CodePDFAlreadyReady] = {409, "This PDF is already ready."}`.

- [ ] **Step 1: Write the failing tests**

| Test | Input | Expected |
|---|---|---|
| `TestRequestPDFCreatesAMissingLanguage` | collected report in `en`; `RequestPDF(pt-BR)` | a `pt-BR` row `pending`, one new render job, phase `rendering`, audit `report.pdf_request {"language":"pt-BR"}` |
| `TestRequestPDFRetriesAFailedOne` | `en` row `failed` | the row `pending`, a new job, phase `rendering`, one audit row |
| `TestRequestPDFWhilePending` | `en` row `pending` | nil; no new job; no audit row |
| `TestRequestPDFWhenReady` | `en` row `ready` | `PDF_ALREADY_READY` |
| `TestRequestPDFNeedsACollectedReport` | report `running`; report expired; report deleted | `SNAPSHOT_NOT_READY`; `REPORT_EXPIRED`; `REPORT_NOT_FOUND` |
| `TestRequestPDFNeedsAnOperator` | a reader | `FORBIDDEN` |
| `TestPostPDFsRoute` (`api`) | `POST /reports/{id}/pdfs {"language":"pt-BR"}` as an operator | 202 `{"language":"pt-BR","status":"pending"}` |
| `TestPostPDFsWhenReady` (`api`) | same, row ready | 409 `PDF_ALREADY_READY` |
| `TestRoutesMatchTheContract`, `TestContractNamesEveryCode` (existing) | the renamed route and code | pass both ways; `/pdf-retries` and `PDF_RETRY_NOT_ALLOWED` exist nowhere (`grep -rn` in `internal/ api/ web/src` returns nothing) |

- [ ] **Step 2:** Run the new tests; expected FAIL.
- [ ] **Step 3:** Implement; `make generate`.
- [ ] **Step 4:** `make test`, `make lint`, `make generate-check`, the tagged tests as in Task 1. All pass.
- [ ] **Step 5:** Commit `feat(reports): make a PDF on request` (36 characters).

**Done when:** every row passes and the grep of the old names is empty.

---

### Task 3: A stalled download no longer fails the stop; two minors (R184, R185; F-128, F-129)

**Executor:** Subagent. Follows Task 2. May run beside Task 4.

**Files:**
- Modify: `internal/cli/serve.go` (the stop near lines 95-108; `newHTTPServer` near 170 gains the `ConnState` hook), `internal/platform/api/middleware.go` (near line 21)
- Test: `internal/cli/serve_test.go`, `internal/platform/api/reports_test.go` (near line 817)

**Interfaces:**

```go
// stopServer shuts httpServer down within timeout; connections still open
// then are closed and counted in one warning. It fails only when the
// listener cannot be closed.
func stopServer(ctx context.Context, httpServer *http.Server, conns *connCount, timeout time.Duration, logger *slog.Logger) error
```

`connCount` tracks open connections through `http.Server.ConnState`; `runServe` passes `shutdownTimeout` (30 s).

- [ ] **Step 1: Write the failing tests**

| Test | Input | Expected |
|---|---|---|
| `TestStopClosesAStalledDownload` | a handler that writes 1 MiB to a client that never reads; `stopServer` with 200 ms | returns nil within 1 s; the log holds `serve: connections closed before they finished: 1` |
| `TestStopWithNothingOpen` | no connection | returns nil; no warning line |
| `TestServeListensAndStops` (existing) | — | still passes |
| `TestStoredSnapshotChangedUnderItsDigestAnswers500` (changed) | the golden snapshot with one digit changed; the test asserts the changed body is valid JSON | `/snapshot` and `/view?lang=en` answer 500 `INTERNAL_ERROR`; with the digest check removed by a scratch edit, the view half fails |
| `TestCanceledRequestLogsItsError` | a request whose context is cancelled by the client, at debug level | the line holds `error=` |

- [ ] **Step 2:** Run them; `TestStopClosesAStalledDownload` fails against today's code (the stop returns `cannot stop the server`).
- [ ] **Step 3:** Implement.
- [ ] **Step 4:** `make test`, `make lint`, and `go test -count=1 -tags='postgres template' ./internal/cli/` and `-tags=postgres ./internal/platform/api/`. All pass.
- [ ] **Step 5:** Commit `fix(serve): close stalled connections on stop` (45 characters).

**Done when:** every row passes, and the scratch mutation of the digest check is reported red for the view half.

---

### Task 4: The shared pieces and every word (R186 to R189, R194, R195)

**Executor:** Subagent. Follows Task 2 (it needs the generated schema).

**Files:**
- Modify: `web/src/api/client.ts` (`ApiFailure.fields`), `web/src/components/status-mark.tsx` (`label`), `web/src/components/ui/button.tsx` (`danger`), `web/src/features/account/account-page.tsx` (uses `Segmented`), `web/src/i18n/families.ts`, `web/src/locales/{en,pt-BR}.json`, `web/src/theme/pairs.ts`, `web/src/app/router.tsx` (`/` opens `/reports`), `web/src/app/shell.tsx` (the nav), `web/src/components/no-style.test.tsx`, `web/tests/msw.ts` (report and profile handlers)
- Create: `web/src/components/ui/{select.tsx,checkbox.tsx,segmented.tsx,confirm-dialog.tsx,popover.tsx}`, `web/src/lib/format.ts`, `web/src/features/reports/{api.ts,marks.ts,period.ts}` and their tests

**Interfaces:**

```ts
// components/ui
export function Select(props: SelectHTMLAttributes<HTMLSelectElement> & { label: string; error?: string; hint?: string }): JSX.Element;
export function Checkbox(props: InputHTMLAttributes<HTMLInputElement> & { label: string; indeterminate?: boolean }): JSX.Element;
export function Segmented<T extends string>(props: { legend: string; name: string; value: T; options: readonly { value: T; label: string; lang?: string }[]; onChange: (value: T) => void }): JSX.Element;
export function ConfirmDialog(props: { open: boolean; title: string; body: string; confirm: string; cancel: string; onConfirm: () => void; onOpenChange: (open: boolean) => void }): JSX.Element; // Radix AlertDialog; Cancel holds the focus first; confirm is a `danger` Button
export const Popover: typeof import("radix-ui").Popover.Root; export function PopoverContent(props: ComponentProps<typeof import("radix-ui").Popover.Content>): JSX.Element;

// lib/format.ts (R187)
export function formatTime(iso: string): string;          // "2026-10-07 14:02", browser time zone
export function formatDate(iso: string): string;          // "2026-11-06"
export function formatElapsed(ms: number): string;        // "0:42", "1:02:05"

// features/reports/period.ts (R188)
export const quickRanges = ["1h", "6h", "24h", "7d", "15d", "30d"] as const;
export function parsePeriod(input: string): string | null; // trims; ^[1-9][0-9]*[mhd]$ or null
export function periodText(t: TFunction, language: Language, period: string): string;

// features/reports/marks.ts (R194)
export function reportMark(report: { status: ReportStatus; expired: boolean }): Status;
export function pdfMark(status: PDFStatus): Status;

// features/reports/api.ts
export const reportsKey: readonly ["reports"];
export function useReports(): UseInfiniteQueryResult<InfiniteData<ReportPage>>;      // GET /reports, limit 50, refetchInterval per R190
export function useReport(id: string): UseQueryResult<Report>;  // GET /reports/{id}, backoff per R190
export function useProfiles(): UseQueryResult<ProfileSummary[]>;
export function isActive(report: { status: ReportStatus; pdfs: ReportPDFState[] }): boolean;
export function canDelete(principal: Principal, report: { owner_id: string }): boolean; // R191
```

`api.ts` also exports the mutations `useRequestReport()`, `useRequestPDF(id)`, `useDeleteReport()`, `useDeleteReports()`; each invalidates `reportsKey` on success.

**The wording.** Transcribe exactly. Keys in the `error` group are already present except `PDF_ALREADY_READY` (added in Task 2).

| Key | en | pt-BR |
|---|---|---|
| `nav.reports` | Reports | Relatórios |
| `nav.generate` | New report | Novo relatório |
| `reports.title` | Reports | Relatórios |
| `reports.updating` | Updating while a report runs | Atualizando enquanto há relatórios em andamento |
| `reports.column.requested` | Requested | Criado em |
| `reports.column.profile` | Profile | Perfil |
| `reports.column.status` | Status | Status |
| `reports.column.pdf` | PDF | PDF |
| `reports.column.keptUntil` | Kept until | Disponível até |
| `reports.open` | Open | Abrir |
| `reports.delete` | Delete | Excluir |
| `reports.select` | Select the report of {{time}} | Selecionar o relatório de {{time}} |
| `reports.selectAll` | Select all | Selecionar todos |
| `reports.selected` | Selected: {{count}} | Selecionados: {{count}} |
| `reports.deleteSelected` | Delete selected | Excluir selecionados |
| `reports.clearSelection` | Clear selection | Limpar seleção |
| `reports.loadMore` | Load more | Carregar mais |
| `reports.empty` | No reports yet. | Nenhum relatório ainda. |
| `reports.expired` | Expired | Expirado |
| `reports.notGenerated` | Not generated | Não gerado |
| `reports.loadFailed` | The reports could not be loaded. | Não foi possível carregar os relatórios. |
| `reports.retry` | Try again | Tentar de novo |
| `reports.deleteForbidden` | Some reports could not be deleted: your role does not allow it. | Alguns relatórios não puderam ser excluídos: seu papel não permite. |
| `delete.oneTitle` | Delete this report? | Excluir este relatório? |
| `delete.oneBody` | {{profile}} · {{time}}, by {{username}}. Its data and its PDFs are removed. This cannot be undone. | {{profile}} · {{time}}, por {{username}}. Os dados e os PDFs são removidos. Esta ação não pode ser desfeita. |
| `delete.manyTitle` | Delete {{count}} reports? | Excluir {{count}} relatórios? |
| `delete.manyBody` | Their data and their PDFs are removed. This cannot be undone. | Os dados e os PDFs são removidos. Esta ação não pode ser desfeita. |
| `delete.confirm` | Delete | Excluir |
| `delete.cancel` | Cancel | Cancelar |
| `generate.title` | New report | Novo relatório |
| `generate.profile` | Profile | Perfil |
| `generate.namespaces` | Namespaces | Namespaces |
| `generate.language` | Report language | Idioma do relatório |
| `generate.options` | Options | Opções |
| `generate.change` | Change | Alterar |
| `generate.period` | Period | Período |
| `generate.timezone` | Time zone | Fuso horário |
| `generate.reportProfile` | Report profile | Perfil do relatório |
| `generate.minSeverity` | Minimum severity | Severidade mínima |
| `generate.submit` | Request report | Gerar relatório |
| `generate.submitting` | Requesting… | Gerando… |
| `generate.noProfiles` | No profile is available. An administrator creates profiles. | Nenhum perfil disponível. Um administrador cria os perfis. |
| `generate.profilesFailed` | The profiles could not be loaded. | Não foi possível carregar os perfis. |
| `generate.profileChanged` | The profile changed. Its choices were loaded again. | O perfil mudou. Suas opções foram carregadas de novo. |
| `generate.longPeriod` | Periods over 15 days read older data through Thanos and can raise CPU and memory use. | Períodos acima de 15 dias leem dados antigos pelo Thanos e podem elevar o uso de CPU e memória. |
| `period.custom.title` | Custom | Personalizado |
| `period.custom.label` | Length | Duração |
| `period.custom.hint` | A number and a unit: m minutes, h hours, d days. For example 90m, 36h, 10d. | Um número e uma unidade: m minutos, h horas, d dias. Por exemplo 90m, 36h, 10d. |
| `period.custom.value` | Last {{duration}} | {{duration}} até agora |
| `period.apply` | Apply | Aplicar |
| `period.invalid` | Write a number followed by m, h, or d. | Escreva um número seguido de m, h ou d. |
| `period.restricted` | This profile permits only these periods. | Este perfil permite apenas estes períodos. |
| `period.quick.1h` | Last hour | Última hora |
| `period.quick.6h` | Last 6 hours | Últimas 6 horas |
| `period.quick.24h` | Last 24 hours | Últimas 24 horas |
| `period.quick.7d` | Last 7 days | Últimos 7 dias |
| `period.quick.15d` | Last 15 days | Últimos 15 dias |
| `period.quick.30d` | Last 30 days | Últimos 30 dias |
| `progress.screen` | Report | Relatório |
| `progress.title` | Report on {{profile}} | Relatório de {{profile}} |
| `progress.requested` | Requested {{time}} by {{username}} | Criado em {{time}} por {{username}} |
| `progress.keptUntil` | Kept until {{date}} | Disponível até {{date}} |
| `progress.steps` | Progress | Progresso |
| `progress.elapsed` | Elapsed {{time}} | Tempo decorrido {{time}} |
| `progress.closeable` | You can close this page; the report keeps running. | Você pode fechar esta página; o relatório continua sendo gerado. |
| `progress.pdf` | PDF | PDF |
| `progress.download` | Download PDF | Baixar PDF |
| `progress.generatePdf` | Generate PDF | Gerar PDF |
| `progress.retryPdf` | Try again | Tentar de novo |
| `progress.deleteReport` | Delete report | Excluir relatório |
| `progress.code` | Code {{code}} | Código {{code}} |
| `progress.requestAgain` | Request again with these values | Gerar de novo com estes valores |
| `progress.back` | Back to reports | Voltar aos relatórios |
| `progress.notFound` | This report does not exist, or was deleted. | Este relatório não existe ou foi excluído. |
| `progress.expired` | This report has expired; its data and PDFs were removed. | Este relatório expirou; seus dados e PDFs foram removidos. |
| `progress.announce.collected` | The report was collected. | O relatório foi coletado. |
| `progress.announce.failed` | The report failed. | O relatório falhou. |
| `progress.announce.pdfReady` | The PDF in {{language}} is ready. | O PDF em {{language}} está pronto. |
| `validation.namespaces` | Choose at least one namespace. | Escolha pelo menos um namespace. |
| `validation.notPermitted` | The profile does not permit this value. | O perfil não permite este valor. |
| `validation.invalid` | This value is not valid. | Este valor não é válido. |
| `error.PDF_ALREADY_READY` | This PDF is already ready. | Este PDF já está pronto. |

The families, each equal to its generated enum (`families.ts`):

| Family | en | pt-BR |
|---|---|---|
| `reportStatus` | queued: Queued; running: Running; collected: Collected; failed: Failed | queued: Na fila; running: Em execução; collected: Coletado; failed: Falhou |
| `reportPhase` | waiting: Waiting; preflight: Checking the source; collecting: Collecting; analyzing: Analyzing; snapshotting: Snapshot; rendering: PDF; complete: Complete | waiting: Aguardando; preflight: Verificando a fonte; collecting: Coletando; analyzing: Analisando; snapshotting: Snapshot; rendering: PDF; complete: Concluído |
| `pdfStatus` | pending: Preparing; ready: Ready; failed: Failed | pending: Em preparação; ready: Pronto; failed: Falhou |
| `reportProfile` | detailed: Detailed; executive: Executive | detailed: Detalhado; executive: Executivo |
| `minSeverity` | all: All; attention: Attention and critical; critical: Critical only | all: Todas; attention: Atenção e crítico; critical: Somente crítico |
| `pdfFailure` | PDF_COMPILE_FAILED: The PDF could not be compiled.; PDF_TOO_LARGE: The PDF is larger than allowed.; PDF_ATTEMPTS_EXHAUSTED: The PDF failed after every attempt.; INTERNAL_FAILURE: Something went wrong while making the PDF. | PDF_COMPILE_FAILED: Não foi possível compilar o PDF.; PDF_TOO_LARGE: O PDF passa do tamanho permitido.; PDF_ATTEMPTS_EXHAUSTED: O PDF falhou em todas as tentativas.; INTERNAL_FAILURE: Algo deu errado ao gerar o PDF. |

`reportFailure`, one sentence per code:

| Code | en | pt-BR |
|---|---|---|
| ENDPOINT_URL_REJECTED | The address of the metric source is not allowed. | O endereço da fonte de métricas não é permitido. |
| ENDPOINT_ADDRESS_REJECTED | The metric source resolves to an address that is not allowed. | A fonte de métricas aponta para um endereço que não é permitido. |
| ENDPOINT_RESOLUTION_FAILED | The name of the metric source could not be resolved. | Não foi possível resolver o nome da fonte de métricas. |
| ENDPOINT_CONNECTION_REJECTED | The connection to the metric source was refused. | A conexão com a fonte de métricas foi recusada. |
| ENDPOINT_REDIRECT_REJECTED | The metric source answered with a redirect, which is not allowed. | A fonte de métricas respondeu com um redirecionamento, que não é permitido. |
| PROFILE_CREDENTIAL_UNAVAILABLE | The credential file of the profile could not be read. | Não foi possível ler o arquivo de credencial do perfil. |
| PROFILE_CA_BUNDLE_UNAVAILABLE | The CA bundle of the profile could not be read. | Não foi possível ler o CA bundle do perfil. |
| ENDPOINT_TLS_FAILED | The TLS connection to the metric source failed. | A conexão TLS com a fonte de métricas falhou. |
| ENDPOINT_AUTH_REJECTED | The metric source refused the credential. | A fonte de métricas recusou a credencial. |
| ENDPOINT_API_NOT_FOUND | No Prometheus API was found at the address of the metric source. | Nenhuma API do Prometheus foi encontrada no endereço da fonte de métricas. |
| ENDPOINT_UNREACHABLE | The metric source did not answer in time. | A fonte de métricas não respondeu a tempo. |
| CLUSTER_SCOPE_NOT_FOUND | The cluster of the profile was not found in the metrics. | O cluster do perfil não foi encontrado nas métricas. |
| CAPABILITIES_INSUFFICIENT | The metric source lacks the metrics a report needs. | A fonte de métricas não tem as métricas de que um relatório precisa. |
| NOTHING_TO_REPORT | The metrics held nothing to report for this selection. | As métricas não tinham nada a relatar para esta seleção. |
| SNAPSHOT_TOO_LARGE | The report's data is larger than allowed. | Os dados do relatório passam do tamanho permitido. |
| COLLECTION_TIMEOUT | The collection took longer than allowed. | A coleta levou mais tempo do que o permitido. |
| JOB_ATTEMPTS_EXHAUSTED | The report failed after every attempt. | O relatório falhou em todas as tentativas. |
| INTERNAL_FAILURE | Something went wrong while making the report. | Algo deu errado ao gerar o relatório. |

**The pair:** `text("surface", "critical")` (R195).

- [ ] **Step 1: Write the failing tests**

| Test | Input | Expected |
|---|---|---|
| `client keeps the fields of an error` | a 422 with `fields {"period":"not_permitted"}` | `ApiFailure.fields.period === "not_permitted"`; an error without `fields` gives `{}` |
| `status mark prints its label` | `<StatusMark status="undefined" label="Running" />` | the text "Running"; the outlined circle |
| `segmented control` | arrow keys over two options | the checked value moves and `onChange` fires; the account screen's tests still pass |
| `select and checkbox` | a `Select` with an `error`; a `Checkbox` with `indeterminate` | `aria-invalid` and the error wired by `aria-describedby`; the input's `indeterminate` is true |
| `confirm dialog` | open it | role `alertdialog`, the title as its name, the focus on Cancel; Escape closes it |
| `no style from the popover and the dialog` (`no-style.test.tsx`) | open and close a `Popover` and a `ConfirmDialog` | no `<style>` added |
| `period grammar` | `36h`, ` 36h `, `0h`, `01h`, `1H`, `1.5h`, `36`, `h`, `""` | `36h`, `36h`, then `null` for each other |
| `period words` | `7d` in en; `36h` in en and pt-BR; `1h` in pt-BR | "Last 7 days"; "Last 36 hours", "36 horas até agora"; "Última hora" |
| `format` | `2026-10-07T17:02:00Z` with the time zone `America/Sao_Paulo`; 42 000 ms; 3 725 000 ms | "2026-10-07 14:02"; "0:42"; "1:02:05" |
| `marks` | each status, an expired report, each PDF status | the mapping of R194 |
| `can delete` | administrator, operator on own and other report, reader | true, true, false, false |
| `families are exact`, `every key is used`, `same keys, same parameters`, vocabulary (existing) | the new keys | pass |
| contrast (existing) | the new pair | passes in both themes |

- [ ] **Step 2:** `cd web && npx vitest run`; expected FAIL for the new rows.
- [ ] **Step 3:** Implement. The sidebar lists Reports, then New report (operators and administrators), then Account; `/` opens `/reports`. The routes `/reports`, `/generate`, `/jobs/:id` are added by Tasks 5, 6, 7.
- [ ] **Step 4:** `make test`, `make lint`. All pass.
- [ ] **Step 5:** Commit `feat(web): add the pieces of the report screens` (47 characters).

**Done when:** every row passes and every key of the tables is in both locale files, letter for letter.

---

### Task 5: The report list and the delete dialog (R190, R191)

**Executor:** Subagent. Follows Task 4.

**Files:**
- Create: `web/src/features/reports/{reports-page.tsx,delete-dialog.tsx,reports-page.test.tsx}`
- Modify: `web/src/app/router.tsx` (`/reports`), `web/tests/screens.test.tsx` (the screen "reports" in both themes and languages)

**What it shows:** spec 12.2 as amended and `section-1.html`, frames 1, 2, 3, 9: the title with `reports.updating` while a row is active, "New report" for operators and administrators; the table (`Checkbox` column per R191, Requested in mono, Profile as the profile's `display_name` or its id when absent from `GET /profiles`, Status by `reportMark`, the interface language's PDF by `pdfMark` or `reports.notGenerated` or "—" before collection, Kept until, Open and Delete); the bulk bar while a row is selected; "Load more" while `next_cursor` is set; `reports.empty` with the button; `reports.loadFailed` with `reports.retry` when nothing loaded, an alert above the rows when a later check failed.

- [ ] **Step 1: Write the failing tests**

| Test | Input | Expected |
|---|---|---|
| `newest first, load more` | two pages of the mock | 50 rows, then "Load more" shows 60; the button leaves at the last page |
| `polling keeps the loaded pages` (Review Focus 5) | two pages loaded, one row running; advance 5 s with the row now collected | 60 rows, no duplicate id, the row shows Collected; with no active row, no request after 10 s more |
| `a reader` | role reader | no checkbox, no Delete, no New report |
| `an operator deletes only their own` | rows of two owners | checkbox and Delete on own rows only |
| `delete one` | Delete, then Delete in the dialog | `DELETE /reports/{id}`; the dialog named the profile, the time, the owner; the list reloads |
| `bulk delete` | three selected; outcomes deleted, not_found, forbidden | `POST /reports/delete` with the three ids in order; the forbidden row stays selected; `reports.deleteForbidden` shows |
| `bulk of one` | one selected, Delete selected | the one-report dialog |
| `cancel keeps everything` | Cancel in the dialog | no request; selection kept |
| `expired row` | `expired: true` | the dash mark and `reports.expired`; no Open; Delete for who may |
| `empty` | no report, as operator | `reports.empty` and the New report button |
| screens (existing test, new entry) | `/reports` in en and pt-BR, light and dark | no axe finding, no banned term, the title |

- [ ] **Step 2:** Run; expected FAIL.
- [ ] **Step 3:** Implement.
- [ ] **Step 4:** `make test`, `make lint`.
- [ ] **Step 5:** Commit `feat(web): add the report list` (30 characters).

---

### Task 6: The new report form and the period picker (R186, R188, R189, R192, R193)

**Executor:** Subagent. Follows Task 5.

**Files:**
- Create: `web/src/features/reports/{generate-page.tsx,period-picker.tsx,generate-page.test.tsx,period-picker.test.tsx}`
- Modify: `web/src/app/router.tsx` (`/generate`, operators and administrators; a reader gets the not-found screen), `web/tests/screens.test.tsx`

**What it shows:** spec 12.3 as amended, `section-1.html` frame 4, and `q3b-period.html`: Profile (`Select`), Namespaces (`Checkbox` per allowed namespace, mono), Report language (`Segmented`), the options line (`generate.options`, the four values in words, `generate.change`), which opens Period (the picker: a button that opens a `Popover` with the quick ranges on the right and the custom period on the left, or only the permitted periods with `period.restricted`), Time zone, Report profile, Minimum severity (`Select`s). `generate.longPeriod` above 15 days. Submit goes to `/jobs/<id>`.

- [ ] **Step 1: Write the failing tests**

| Test | Input | Expected |
|---|---|---|
| `defaults from the profile` | choose a profile | the options line shows its defaults; no namespace checked; the language is the interface's |
| `another profile resets` | change profile after checking a namespace | namespaces cleared, options at the new defaults |
| `needs a namespace` | submit with none | `validation.namespaces`; no request |
| `one report per attempt` (Review Focus 2) | double click; then a network failure and a resend; then a value changed and a send | one `POST` for the double click; the resend carries the same `Idempotency-Key`; after the change, a new key |
| `period grammar` (Review Focus 3) | type each input of Task 4's grammar row and press Apply | only ` 36h ` is applied, as `36h`; the others show `period.invalid`, Apply disabled |
| `restricted periods` | `permitted_periods ["24h","7d"]` | only those two, `period.restricted`, no custom |
| `long period` | `30d` | `generate.longPeriod` shows; at `15d` it does not |
| `time zones` | `permitted_timezones []` | the browser's list, the default first selected |
| `server fields` | 422 `REPORT_SELECTION_NOT_PERMITTED`, fields `{"period":"not_permitted"}` | the period marked `validation.notPermitted`, the options opened |
| `profile changed` | 409 `PROFILE_INACTIVE` | `generate.profileChanged`; profiles reloaded |
| `admission` | 429 `ADMISSION_GLOBAL_LIMIT`, Retry-After 30 | the code's sentence with 30 |
| `from a failed report` | `/generate?from=<id>` | every value filled from the report's selection, the language included |
| `no profile` | `GET /profiles` empty | `generate.noProfiles`, no form |
| `a reader` | `/generate` as reader | the not-found screen |
| screens (existing, new entry) | `/generate` in both languages and themes | passes |

- [ ] **Step 2:** Run; expected FAIL.
- [ ] **Step 3:** Implement.
- [ ] **Step 4:** `make test`, `make lint`.
- [ ] **Step 5:** Commit `feat(web): add the new report form` (34 characters).

---

### Task 7: The progress screen and its PDFs (R190, R191, R194)

**Executor:** Subagent. Follows Task 6.

**Files:**
- Create: `web/src/features/reports/{progress-page.tsx,pdf-block.tsx,progress-page.test.tsx}`
- Modify: `web/src/app/router.tsx` (`/jobs/:id`), `web/tests/screens.test.tsx`

**What it shows:** spec 12.2 as amended and `section-1.html` frames 5 to 8: `progress.title` with the profile's name, `progress.requested` and the status mark; running: the seven steps (`reportPhase`), done ones filled, the current one marked, `progress.elapsed`, `progress.closeable`; collected: `progress.keptUntil`, the PDF block (the interface language's PDF first: its mark and Download, a plain link to `/api/v1/reports/{id}/pdf?lang=`, or Try again when failed with the `pdfFailure` sentence; then the other language: `reports.notGenerated` with Generate PDF, its mark while pending, or its download), Delete report; failed: an error alert with the `reportFailure` sentence and `progress.code`, then Back to reports and, for an operator, Request again with these values. Each change of status and each PDF turning ready is announced once through `useAnnounce`.

- [ ] **Step 1: Write the failing tests**

| Test | Input | Expected |
|---|---|---|
| `steps advance` | phases waiting, collecting, then collected | the current step moves; the request schedule is 1.5 s, then 3 s after 20 checks, then 6 s after 40 (fake timers) |
| `stops when final` | collected with no pending PDF; failed | no request after 10 s more |
| `the PDF block follows the language` (Review Focus 4) | collected, `en` ready, `pt-BR` absent; switch the interface to pt-BR | before: English first with Download; after: Português first with `reports.notGenerated` and Generate PDF |
| `generate the other language` | Generate PDF | `POST /reports/{id}/pdfs {"language": …}`; the row shows Preparing; checking resumes until Ready, announced once |
| `retry a failed PDF` | `en` failed with `PDF_COMPILE_FAILED` | its sentence and Try again; the same route |
| `a reader` | role reader | downloads only: no Generate, no Try again, no Delete |
| `failed report` | `ENDPOINT_UNREACHABLE` | the sentence, `Code ENDPOINT_UNREACHABLE`, Back; Request again opens `/generate?from=<id>` for an operator only |
| `a report deleted elsewhere` (Review Focus 1) | a check answers 404 | `progress.notFound`, Back; no further request |
| `expired` | a check answers 410 | `progress.expired` |
| `delete from here` | Delete report, confirm | `DELETE`, then `/reports` |
| `announcements` | running → collected → PDF ready | the live region reads `progress.announce.collected`, then `progress.announce.pdfReady`, once each |
| screens (existing, new entry) | `/jobs/<id>` collected, both languages and themes | passes |

- [ ] **Step 2:** Run; expected FAIL.
- [ ] **Step 3:** Implement.
- [ ] **Step 4:** `make test`, `make lint`.
- [ ] **Step 5:** Commit `feat(web): add the progress screen` (34 characters).

---

### Task 8: The integration stack and the reports spec (R196, R197)

**Executor:** Subagent writes the files; the main thread runs `make e2e` and owns the result.

**Files:**
- Modify: `deploy/Containerfile` (stage `fake-prometheus`), `deploy/compose.integration.yaml` (services `fake-prometheus`, `profiles`; `serve` and `worker` depend on `fake-prometheus`), `deploy/platform/platform.toml` (`allowed_endpoint_hostnames`), `deploy/platform/README.md` (names `profiles.toml`), `Makefile` (`e2e` builds the stage and runs `profiles` after `bootstrap`), `web/e2e/sign-in.spec.ts` (imports the fixture)
- Create: `deploy/platform/profiles.toml`, `web/e2e/fixtures.ts`, `web/e2e/reports.spec.ts`

**The spec `reports.spec.ts`**, serial, in English, every page through `expectNoViolations` and the CSP guard:
1. The administrator creates `e2e-operator` (operator) and `e2e-reader` (reader) through `POST /admin/users`, with fixed passwords in the spec.
2. The operator signs in: `/reports` is empty; New report; profile Production, namespace `payments`, the language English; Request report.
3. The progress page reaches Collected by itself (timeout 60 s); the English PDF is Ready; its download's first bytes are `%PDF-`.
4. Generate PDF for Português (Brasil); it reaches Ready.
5. Reports: the row shows Collected and the English PDF Ready, without a reload.
6. In a second context, the reader signs in: no New report, no checkbox, no Delete; `/generate` shows not found; the progress page offers the two downloads and nothing else.
7. Back as the operator: Delete in the row, the dialog names the report, Delete; the list is empty.

- [ ] **Step 1 (subagent):** Write the files; `make lint` passes (ESLint covers `e2e/`); `docker compose -f deploy/compose.yaml -f deploy/compose.integration.yaml config` is not run by the subagent.
- [ ] **Step 2 (main thread):** `make e2e`; both specs pass. Prove the guards: a scratch edit that hides Generate PDF from operators fails step 4 (`.cache/phase-4/part-2/e2e/`), reverted.
- [ ] **Step 3:** Commit `test(e2e): request a report in the browser` (42 characters).

**Done when:** `make e2e` passes twice in a row from a clean stack.

---

### Task 9: The combined run and the owner's look

**Executor:** Main thread.

- [ ] **Step 1:** `make test`, `make lint`, `make generate-check`, `make test-postgres`, `make image`, `make check-runtime`, `make e2e` over the combined branch; logs under `.cache/phase-4/part-2/combined/`.
- [ ] **Step 2:** Screenshots of the real build (list, form with the picker open, progress running, collected, failed, delete dialog; light and dark; en and pt-BR) on the companion page, opened for the owner. A change the owner asks for is made and the screenshots taken again.

**Done when:** every command passes and the owner has seen the screens.

---

### Task 10: The documents, review, gate, close

**Executor:** Main thread.

- [ ] **Step 1: `AGENTS.md`.** "The interface" gains: the report screens' checking rules (R190), who sees what (R191), the idempotency key (R193), the native selects and no style hash (R189), the period's words (R188). "The platform" changes: a request names its language; one PDF; `POST /reports/{id}/pdfs`; `serve`'s stop (R184). `make e2e` builds the fake Prometheus stage.
- [ ] **Step 2: `README.md`.** The interface now requests and follows reports; the API section names `language` and `/pdfs`.
- [ ] **Step 3: `CHANGELOG.md`, under Unreleased.**
  - `### Added`: the report list, with bulk delete; the new report form, with quick and typed periods; the progress page, where the PDF is downloaded and the other language generated.
  - `### Changed`: a report request names its language, and only that PDF is made at first; the other is made on request.
  - `### Removed`: the PDF retry route, replaced by the PDF request route.
  - `### Fixed`: stopping `serve` no longer fails while a download is stalled.
  - Omitted, and said so: the fake Prometheus stage and fixture profile, the test fixes of F-129.
- [ ] **Step 4: The adversarial review of the branch, by areas**, probes under `.cache/phase-4/part-2/review/probe/`: (a) the request, the PDF request, and the worker's one PDF; (b) the stop of `serve`; (c) the three screens' data: checking, idempotency, roles, errors; (d) wording, contrast, accessibility. One fix wave, then one re-review.
- [ ] **Step 5:** `make gate` with logs under `.cache/phase-4/part-2/gate/`; the named tests of Tasks 1 to 3 run by name on the same commit.
- [ ] **Step 6: Close.** `ROADMAP.md` (P42 items, F-128, F-129); the closing report `.cache/phase-4/part-2/closing-report.md`; the owner's word on the merge and the push. The version is the owner's call: this part changes the API (a required field, a renamed route), which is a minor version by the changelog's rules.

**Done when:** the gate passes from images built without cache, the review is clean, and the owner has the closing report.
