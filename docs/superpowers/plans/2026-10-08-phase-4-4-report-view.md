# Phase 4.4: The Report and Its Seven Tabs Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** A signed-in reader opens a collected report from its progress screen and reads it on the web in seven tabs (overview, findings, namespaces, workloads, autoscaling, nodes, data quality), worded as the PDF words it, in the interface language, with the PDFs one click away.

**Architecture:** The route `/reports/:id/:tab` renders a frame that reads `GET /reports/{id}` (the header and the PDF block) and `GET /reports/{id}/view?lang=` (the tabs), and hands the view to each tab through the router's outlet context. The view's closed sets become named enums in the contract, so the frontend gets their value lists from the generated types and a Go test holds them equal to the Go registries. One table component on TanStack Table serves the five tables; every state of a tab lives in the address. The frontend tests read views of golden snapshots that `make update-testdata` writes and a Go test keeps current.

**Tech Stack:** As in part 4.3, plus one dependency: `@tanstack/react-table` 9.2.6, exact. Go 1.27; React 19.3.0, React Router 8.4.0, TanStack Query 5.104.1, Radix UI 1.7.0, Tailwind CSS 4.3.3, react-i18next 17.0.16, openapi-typescript 7.13.0; Vitest with MSW; Playwright.

**Spec:** `docs/superpowers/specs/2026-09-27-k8s-capacity-report-design.md` as amended in `61b7fbb` (approved by the owner on 2026-10-08): section 12.2 and the new 12.2.1. The owner's choices are logged in `.cache/phase-4/part-4/brainstorm-decisions.md` (answers 1 to 12). The mockups they approved are under `.cache/phase-4/brainstorm/.superpowers/brainstorm/`: `401575-1791487146/content/q4-frame.html` (frame 4C), `…/q5-overview-5d-v2.html` (overview 5D), and in `106198-1791491599/content/`: `q7-tables.html` (6C without the code, top; tables 7B), `q8-sheet.html` (8B), `q9-tabs-v2.html` (the remaining tabs, rows across the full width). Their source is `.cache/phase-4/part-4/mock/src/`, and an implementer may read it for layout and classes; the words in this plan win over the mock's.

**Depends on:** `main` at `90c4f38` (release 0.4.0). Branch `feat/report-view`, which holds the host commits `8c18113`, `8dd855a`, `2e48112`, `56b6e4a` (F-141, `make gate` passed on `56b6e4a`) and the spec amendment `61b7fbb`.

## Global Constraints

Every constraint of the plans of phases 0 to 4.3 stands, and `AGENTS.md` binds every task. The ones this part leans on most:

- **Every command runs inside `nix develop`** from the repository root (Go 1.27, Node 26, typst 0.15.1, pdftotext, make). The host is NixOS: only `/bin/sh` and `/usr/bin/env` exist under `/bin` and `/usr/bin`.
- **No backward compatibility, and stored data is junk** (the owner, 2026-10-08): no screen state, code, or migration for a report of an earlier version; a refused view takes the general error path.
- **Every text the reader sees, and every accessible name, comes from `web/src/locales/{en,pt-BR}.json`** through `t()`. Each task carries its words in its own table; an implementer transcribes and never invents one. The report's own sentences (finding sentences and actions, the action plan's texts, value texts, reasons, remarks, situations, warnings, notices, the period text) come from the view unchanged.
- **Technical terms are English in both languages** (request, limit, workload, node, node pool, replica, autoscaling, recording rule, raw metrics); the vocabulary test enforces the banned terms of `internal/catalog/banned-terms.json` in both files, "node group" included.
- **Status is a shape, then a label, then a colour**, with `StatusMark`'s five shapes. A shape beside a value carries its status word for screen readers.
- **Names and numbers use the monospaced font.**
- **No inline style**: no `style` prop, no `dangerouslySetInnerHTML`, no `<style>` added at run time; the CSP stays `script-src 'self'` and `style-src 'self'`.
- **Contrast**: every new pair enters `web/src/theme/pairs.ts` (4.5:1 for text, 3:1 for a border or focus ring, both themes).
- **Every npm dependency has an exact version**; `npm ci` with no `--force`; `web/package-lock.json` committed.
- **The contract is `api/openapi.yaml`**; `make generate` writes both type files and `make generate-check` passes.
- **Golden files are written by `make update-testdata` alone**, run by the main thread; a test never writes one.
- **Commits:** `type(scope): description`, imperative, under 50 characters (each subject below is counted); stage with `git add <paths>`.
- **Comments say what the code does**; none names a task, step, ruling, brief, finding, or `.cache/` path.
- Scratch goes under `.cache/phase-4/part-4/`.

## Review Focus

Each line names an input the spec implies and no requirement spells out, and the test that pins it.

1. **A report with nothing in a section** (no finding, no HPA, no namespace, no warning). The tab shows its empty sentence, not an empty table or a broken list. Pinned in Task 3 (`overview empty plan`), Task 4 (`empty table`), Task 5 (`no findings`), Task 7 (`no warnings`).
2. **An address written by hand or kept from another report** (`?open=nope`, `?finding=99`, `?severity=bogus`, `?sort=nope.up`). The tab ignores what it does not know and opens with its defaults; nothing throws. Pinned in Task 4 (`stale address`), Task 5 (`stale finding`).
3. **A language switch in the middle of reading.** The view loads again in the other language and the address state stays: the same tab, the same opened row, the same filters. Pinned in Task 3 (`language switch keeps the place`).
4. **A missing number.** A null raw value shows its placeholder text muted, sorts after every number in both directions, and never breaks a cell. Pinned in Task 4 (`nulls sort last`).
5. **A report that is not readable as a report**: still running (409), expired (410), deleted (404), or failing otherwise (500). Running opens its progress screen; the others show their sentence, never a blank page. Pinned in Task 3 (`states`).

## Where this part sits

| Part | Delivers | Proof at the end |
|---|---|---|
| 4.1 Foundation | Toolchain, serving, theme, i18n, session, sign-in, account | Done (`2f52cd7`) |
| 4.2 Reports | List, form, progress, PDFs, delete | Done (`5090fb7`), released as 0.3.0 |
| 4.3 Calendar ranges | Ranges in the command, the snapshot `2.0`, the API, the picker | Done (`15083c8`), released as 0.4.0 |
| **4.4 The report (this plan)** | The seven tabs | In a real browser: a collected report opened from its progress screen, every tab passing axe and the CSP |
| 4.5 Administration | Profiles and users | An administrator creates a profile and a user |
| 4.6 End-to-end | Security, concurrency, PDF recovery specs | `make e2e` runs the four specs of spec 13.4 |

### Out of this part

| Left out | Reason |
|---|---|
| The focus after Generate PDF and Try again (F-136, part) | Declined by the owner on 2026-10-08 |
| The announcement ref of the progress page (F-136, part) | Deferred by the owner on 2026-10-08 |
| F-134, F-135, F-140, and the picker minors of E253 | The list and the form are not touched by this part |
| Any handling of a report stored by an earlier version | The owner's rule of 2026-10-08 |

## How to read this plan

As in parts 4.2 and 4.3: each task gives exact paths, interfaces, its words, and a table of tests with inputs and expected results. Write each test first, watch it fail, then implement.

Executors:
- **Subagent:** a Sonnet subagent in the main checkout, no worktree (E235). Every command inside `nix develop`. It may run `go`, `make build`, `make test`, `make lint`, `make generate`, `make generate-check`, `make web`, and `npm` inside `web/`. It never runs Docker, `make e2e`, or `make update-*`, and cannot declare a task finished. `make lint` is the only formatter check; no prettier or other formatter is run.
- **Main thread:** Docker, `make update-testdata`, judgment, wording, design, and the owner.

Rules for every brief:
- Tasks run in sequence. A reviewer reads a diff file and runs tests only on packages nobody edits.
- Stage with `git add <paths>`; never stash, restore, check out a path, or clean; never overwrite a file from git.
- A test of a change goes red first, on the code before the change.
- The brief appends this plan's rulings for its task, the Global Constraints, and the Review Focus rows its task owns.
- A shell loop left waiting for a test is found with `pgrep -fl "until grep"` and stopped.

**Order:** Task 1 → main thread's `make update-testdata` and its commit → Task 2 → Task 3 → Task 4 → Task 5 → Task 6 → Task 7 → Task 8 → Task 9 → Task 10. Tasks 3 to 8 share the locale files, `families.ts`, `pairs.ts`, the router, and `tests/screens.test.tsx`, so none runs beside another.

## Rulings made on the owner's behalf

Each ruling is a decision the amended spec leaves open: decision, grounds, cost if wrong. The numbering continues from R212.

| ID | Ruling |
|---|---|
| R213 | The view's closed sets enter the contract as named enums: `ViewSeverity` [healthy, attention, critical], `ViewScopeType` [namespace, workload, hpa, node, nodepool, cluster], `ViewConfidence` [exact, inferred, unknown], `ViewHPACoverage` [covered, uncovered, unknown], `ViewAutoscalingStatus` [enabled, disabled, unknown], `ViewAutoscalingSource` [configured, raw_metrics, none], `ViewStrategy` [recording_rule, raw_metrics, configured, none], `ViewCapabilityName` (the nine of `discovery.Names`, in its order), `ViewCapabilityState` [available, partial, unavailable], `ViewEvidenceName` (the evidence names of `analysis.Registry`, sorted). The fields that carry them take a `$ref`: `ViewFinding.severity`, `.scope_type`, `.confidence`; `ViewEvidence.name`; `ViewRecommendation.severity`, `.scope_type`, `.confidence`; `ViewNamespace.status`; `ViewWorkload.status`, `.hpa_coverage`; `ViewNode.status`, `.pool_confidence`; `ViewNodepool.confidence`; `ViewAutoscaling.status`, `.source`, `.confidence`; `ViewCapability.name`, `.state`, `.strategy`. `ReportView.min_severity` takes `MinSeverity` and `ViewReport.profile` takes `ReportProfile`. The ten new schemas join `exclude-schemas` in `api/oapi-codegen.yaml`. `internal/snapshot` exports its unexported lists as `snapshot.Coverages`, `snapshot.AutoscalingStatuses`, `snapshot.AutoscalingSources` (same values, same order). `TestViewEnumsEqualTheRegistries` (in `internal/platform/api/view_contract_test.go`) reads each enum from the contract and compares it, as a set, with its Go list: `analysis.Severities`; the set of `Definition.ScopeType` over `analysis.Registry`; `discovery.ConfidenceExact`/`Inferred`/`Unknown`; `snapshot.Coverages`; `snapshot.AutoscalingStatuses`; `snapshot.AutoscalingSources`; `discovery.Strategies`; `discovery.Names`; `discovery.States`; the union of `Definition.Evidence[].Name` over `analysis.Registry`. Grounds: spec 12.4 ("every closed set travels as its code, which a client words") and AGENTS ("the frontend's types come from the contract"); the walker of `TestViewMatchesTheContract` already rejects a value outside an enum, so both directions are held. Cost if wrong: contract text only; no handler changes. |
| R214 | Golden views. `internal/goldentest` gains `var ViewScenarios = []string{"edge-scaling", "direct-complete"}`, `func ViewFile(snap []byte, lang catalog.Language) ([]byte, error)` (decode, `document.BuildView`, `document.EncodeView`), and `func ViewFilePath(scenario string, lang catalog.Language) (string, error)` returning `web/tests/fixtures/view/<scenario>.<lang>.json` under the module root. `cmd/update-testdata` writes them for both languages after the data files; `TestGoldenViewFiles` compares each with `compareGolden`. Grounds: the frontend tests then read the exact bytes the API serves for a golden snapshot. Cost if wrong: four files. |
| R215 | The frame. The route `reports/:id` (a sibling of `reports`) renders `ReportPage`; its children are `index` → `<Navigate to="overview" replace />`, one child per tab as its task adds it, and `*` → `NotFound`. `ReportPage` reads `useReport(id)` and `useReportView(id, language)` (key `[...reportsKey, id, "view", language]`, no polling), sets `useScreenTitle(t("progress.screen"))`, and renders: the title `progress.title` with the profile's display name (from `useProfiles`, falling back to the id, as the progress screen does); one line of facts: `report.cluster` with the cluster name in mono, the view's `period_text`, `reportProfile.<profile>`, and `progress.requested` with `formatTime(created_at)` and the owner; a `<nav aria-label={t("report.tabs")}>` of seven `NavLink`s (active: `aria-current="page"`, the accent underline); and at the end of the tab bar the PDF button (R216). The tab receives the view through `useOutletContext`, read by `useView(): ReportView`. States: while either query loads, the title alone; the view answers 409 → `navigate("/jobs/<id>", { replace: true })`; 404 or 410 from either query → the progress screen's `Gone` with `progress.notFound` / `progress.expired` (exported from `progress-page.tsx` for reuse); any other failure → `Alert` with `errorText` and a link `progress.back` to the list. Grounds: spec 12.2.1; the progress screen's own handling. Cost if wrong: one component. |
| R216 | The PDF button. A `Popover` whose trigger is a `Button` (`progress.pdf`, a `CaretDownIcon`) at the end of the tab bar, and whose content is the progress screen's `PdfBlock` unchanged (`canRequest` for operators and administrators), 26rem wide, aligned to the end. `useReport`'s polling keeps a pending PDF moving. Grounds: frame 4C. Cost if wrong: none found. |
| R217 | The progress screen of a collected report that has not expired shows a link `report.open` to `/reports/<id>/overview`, styled as the primary button, in the line under the title, for every role. Grounds: spec 12.2 as amended; the owner's answer B. Cost if wrong: one link. |
| R218 | The overview. Three tiles (critical, attention, healthy; `StatusMark` with `status.<severity>`; the count `summary.<severity>.text` in mono, large; a link `report.overview.seeFindings` to `../findings?severity=<severity>`). Then the inventory, a `dl` labelled `report.overview.inventory`: namespaces, workloads, HPAs, workloads without HPA, nodes, node pools, each number above its name (`flex-col-reverse`), the six numbers on one line. Then `report.plan.title` and a table of `view.recommendations` in their order: `#` (index + 1, mono), priority (`StatusMark` with `status.<severity>`), action, scope (`scope_text`, mono, no wrap), confidence (`confidence.<code>`); none → `report.plan.empty`. Last, a `<details>` closed, summary `report.about.title`, holding a `dl`: time zone (mono), namespaces (joined with ", ", mono), minimum severity (`minSeverity.<code>`), issued (`emitted_at_text`), generator version (mono). Grounds: overview 5D with the aligned inventory. Cost if wrong: one component. |
| R219 | The table. `web/src/features/report/data-table.tsx` exports `DataTable<Row>` on `@tanstack/react-table` 9.2.6 (exact, in `dependencies`): props `label` (the table's accessible name), `searchLabel`, `rows`, `rowKey(row): string`, `rowName(row): string`, `columns: ReportColumn<Row>[]`, `searchText(row): string`, `renderOpened(row): ReactNode`. Above the table: a search field (`Field` with `searchLabel`, a magnifier, matching the lower-cased `searchText` by substring) and `report.table.rows` with the count of rows shown. The table: the first column is the open control, an `IconButton` with `report.table.open` / `report.table.close` and `{name}`, `aria-expanded`; a header is a button when its column sorts, with `aria-sort` on the `th`; the `thead` is sticky at the top of the page; there is no scroll box of the table's own. One row opens at a time, in place, in a row whose single cell spans every column, on `bg-bg`, the opened row itself on `bg-sidebar-active`. The address holds `q`, `sort` (`<columnId>.asc` or `.desc`), and `open` (the row key), written with `replace: true`; an unknown column, direction, or key is ignored. The default order is status by rank (critical, attention, healthy) then the name, ascending. A `sortValue` of `null` sorts after every value in both directions. No rows → `report.table.empty`; no row matches → `report.table.noMatch`. Grounds: spec 12.2.1; tables 7B and rows 8B; spec 12.1 names TanStack Table. Cost if wrong: one component. |
| R220 | The pieces of a row, `web/src/features/report/pieces.tsx`: `ValueText({ value })` (the shape of `value.status` when not null, an `sr-only` `status.<status>`, then `value.text`; muted when `value.raw` is null); `PeakCell({ peak, share, word })` (the peak's `ValueText` in mono, under it in small muted text the share's shape and `report.share.request` or `report.share.requested` with `{value: share.text}`, or the share's text alone when its raw is null); `StatGrid({ title, rows })` (a table: an optional bold title in the first header cell, then `report.stat.average`, `.minimum`, `.peak`, `.current`; one row per `{ label, stat }`, each cell a `ValueText`, mono, right-aligned, across the full width); `Facts({ items })` (a `dl` in a grid of up to four, label over value); `Counts({ label, items })` (the overview's inventory style, number above name, on one line where it fits); `RowFindings({ match })` (heading `report.tab.findings`, then each finding of `view.findings` that `match` keeps, in view order: its `StatusMark` and sentence, the whole item a link to `../findings?finding=<index>`; nothing when none). An opened row holds its pieces full width, stacked with `gap-4`. Grounds: answer 11 (rows across the full width, each with its own findings). Cost if wrong: one file. |
| R221 | Matching a finding to a row: a namespace `scope_type === "namespace" && scope_name === name`; a workload `scope_type === "workload" && scope_kind === kind && scope_name === namespace + "/" + name`; an HPA `scope_type === "hpa" && scope_name === namespace + "/" + name`; a node `scope_type === "node" && scope_name === name`. Grounds: the view's scope names (`payments/payments-web`, `payments/web`). Cost if wrong: a finding missing from a row, still in the findings tab. |
| R222 | The findings tab. Address: `severity` (a comma list of `viewSeverityValues`; absent means all three; present and empty means none; unknown values dropped), `scope` (a `viewScopeTypeValues` value or absent), `q`, `finding` (an index into `view.findings`). Above: a search (`report.findings.search`, matching the lower-cased scope name, sentence, and action), a group of three toggle buttons labelled `report.findings.severity` (each `aria-pressed`, `StatusMark` with `status.<s>`, the count of `view.findings` of that severity in mono), and a `Select` `report.findings.scope` (`report.findings.allScopes`, then `scope.<type>` for each type present in the view). Below, two columns: the list, grouped under one heading per severity in the order critical, attention, healthy, each heading its `StatusMark` and count, each item a button (`aria-pressed` when selected) with the mark, `scope.<type>`, the scope name in mono, and the sentence on one line; beside it the selected finding: the mark, `scope.<type>`, `report.confidence` with `confidence.<code>`, the scope name in mono as heading, `report.findings.observed` and the sentence, `report.findings.action` and the action, `report.findings.evidence` and a `dl` of `evidence.<name>` and its text in mono. No code. The selected finding is the one of `finding` when it is shown; otherwise the first shown, without writing the address. A scope name is shown as `report.node.unknownPool` when the scope is a node pool named `unknown`, and not shown when the scope is `cluster`. No findings → `report.findings.none`; none shown → `report.findings.noMatch`. Grounds: spec 12.2.1; 6C without the code. Cost if wrong: one component. |
| R223 | The tabs of tables. Namespaces: name (mono), status, pods (right, mono), CPU peak (`cpu_usage.peak`, share `cpu_request_percent.peak`, `request`), memory peak (likewise); opened: `StatGrid` CPU (usage, request, limit, % request, % limit), `StatGrid` memory, `RowFindings`. Workloads (only `observed_usage`): name (mono, under it the kind and the namespace in mono), status, pods, CPU peak, memory peak; opened: `Facts` (pods; HPA `hpaCoverage.<code>`; values `report.facts.approximate` when `approximate`, else `report.facts.measured`), the two `StatGrid`s, `RowFindings`. HPAs: name (mono, under it the target kind, the target name and the namespace), situation (`StatusMark` with the view's `situation` when `situation_status` is not null, else the text), min / max (`spec_min.text` / `spec_max.text`, mono, right), peak replicas (`current.peak`); opened: `StatGrid` (current replicas `current`, desired replicas `desired`), `RowFindings`. Nodes: name (mono, under it the pool in mono, or `report.node.unknownPool` as plain text), status, state (`report.node.ready`/`.notReady` · `.schedulable`/`.unschedulable`, or `report.node.unknownState` when `ready` is null; attention colour when not ready), CPU peak (`cpu_usage_percent.peak`, share `cpu_request_percent`, `requested`), memory peak (likewise); opened: the name with the copy control (R224) and the reason (`StatusMark` with the reason when `reason_status` is not null, else the text), `Facts` (CPU capacity, CPU allocatable, memory capacity, memory allocatable), `StatGrid` (CPU %, memory %), `RowFindings`. Search text: the name with what places it. Grounds: tables 7B, rows 8B, the remaining tabs as approved. Cost if wrong: columns only. |
| R224 | The copy control: an `IconButton` (`CopyIcon`, `report.copy` with `{name}`) beside the name of a node in its opened row and of a node pool in the pools table; it calls `navigator.clipboard.writeText(name)` and announces `report.copied` with `{name}` through `useAnnounce`; a refusal announces `report.copyFailed`. The unknown pool has none. Grounds: spec 12.2.1. Cost if wrong: one control. |
| R225 | Autoscaling: a heading `report.autoscaling.title`, beside it `autoscalingStatus.<status>`, `report.autoscaling.source` with `strategy.<source>`, and `report.confidence` with `confidence.<code>`; then `Counts` (current, minimum, maximum nodes; managed node pools; scale events; unschedulable pods; workloads without HPA from `summary`); then `report.hpas.title` and the HPA table. Nodes: the node table, then `report.pools.title` and a plain table (no search, no opening) of `view.nodepools`: name (mono, with the copy control; `report.node.unknownPool` for `unknown`), `confidence.<code>`, the node count in mono followed by the names in small mono. Grounds: the remaining tabs as approved. Cost if wrong: layout only. |
| R226 | Data quality: `report.quality.collected` with `start` and `end` from `formatTime` and `elapsed` from `formatElapsed` of the difference; then the capabilities table (`DataTable`): name `capability.<name>`, state (`StatusMark`: available → `healthy`, partial → `attention`, unavailable → `unavailable`, labelled `capabilityState.<state>`), source `strategy.<strategy>`, remark (the view's text, muted); opened: `report.quality.metrics` and the evidence list, one metric per line, mono; default order the view's (no status rank). Then two columns: `report.quality.warnings` and its messages (each an attention `StatusMark` labelled with the text), `report.quality.notices` and its messages; an empty list shows `report.quality.noWarnings` / `report.quality.noNotices`. Grounds: spec 12.2.1; "the only place metric names appear". Cost if wrong: layout only. |
| R227 | Families: `scope` (`viewScopeTypeValues`), `confidence` (`viewConfidenceValues`), `evidence` (`viewEvidenceNameValues`), `hpaCoverage` (`viewHPACoverageValues`), `autoscalingStatus` (`viewAutoscalingStatusValues`), `strategy` (`viewStrategyValues`), `capability` (`viewCapabilityNameValues`), `capabilityState` (`viewCapabilityStateValues`), `report.tab` (`reportTabs`). The generator's names are read from `schema.gen.ts` after Task 1; a different spelling is used as generated. Grounds: AGENTS (a family is declared). Cost if wrong: none found. |
| R228 | Contrast: the opened row's `bg-sidebar-active` carries text, so `pairs.ts` gains `healthy`, `attention`, `critical`, `neutral`, `muted-fg`, and `fg` on `sidebar-active`, and `accent` on `sidebar-active` at 3:1 (the focus ring). The selected finding uses the same background. If a pair fails, the task stops and reports; the main thread chooses the background. Grounds: AGENTS (a new pair enters the list). Cost if wrong: a background colour. |
| R229 | No inline style on the new surface: `no-style.test.tsx` renders the report on each tab with a row opened and the PDF popover open, and finds no `style` attribute outside Radix content and no `<style>` added. `tests/screens.test.tsx` gains the seven tabs (both languages, both themes, axe and vocabulary). Grounds: AGENTS; the parked scope minor of E253. Cost if wrong: none found. |
| R230 | The reports spec: after the progress page reaches Collected, the operator follows `report.open`, sees the overview, visits the seven tabs in order (each: its heading or table visible, `expectNoViolations`), opens the first workload row, and opens the PDF popover. F-137 (a): the reader's context is opened inside the `try` whose `finally` closes it. F-137 (b): `make e2e` reads the fake's log after the stack stops and fails unless it holds the line `remaining: 0`. Grounds: spec 13.4; the owner's answer on question 3. Cost if wrong: one spec step. |
| R231 | Version 0.5.0 is proposed at the close: a new screen. Released only on the owner's word. Grounds: AGENTS (a new feature is a minor version). Cost if wrong: the owner's call. |

## Findings this plan closes

| Finding | How |
|---|---|
| F-137 (a) and (b) | Task 9 |
| E253, the no-style scope minor | Task 8 |
| F-141 | Already closed on this branch (`8dd855a`, `2e48112`, `56b6e4a`) |

## File structure

| Path | Responsibility |
|---|---|
| `api/openapi.yaml`, `api/oapi-codegen.yaml`, `internal/snapshot/{snapshot.go,codec.go}`, `internal/platform/api/view_contract_test.go`, `web/src/api/schema.gen.ts`, `internal/platform/api/openapi.gen.go`; `internal/goldentest/goldentest.go`, `internal/goldentest/golden_test.go`, `cmd/update-testdata/main.go` | The view's enums and the golden views (Task 1) |
| `web/tests/fixtures/view/{edge-scaling,direct-complete}.{en,pt-BR}.json` | Written by `make update-testdata` (main thread, after Task 1) |
| `web/package.json`, `web/package-lock.json` | `@tanstack/react-table` 9.2.6 (Task 4) |
| `web/src/features/report/api.ts` | `useReportView`, the view's types |
| `web/src/features/report/tabs.ts` | `reportTabs` |
| `web/src/features/report/report-page.tsx` | The frame, the states, the PDF button, `useView` |
| `web/src/features/report/overview-tab.tsx` | The overview |
| `web/src/features/report/pieces.tsx` | `ValueText`, `PeakCell`, `StatGrid`, `Facts`, `Counts`, `RowFindings`, the matchers of R221 |
| `web/src/features/report/data-table.tsx` | `DataTable` |
| `web/src/features/report/{namespaces-tab,workloads-tab}.tsx` | Task 4 |
| `web/src/features/report/findings-tab.tsx` | Task 5 |
| `web/src/features/report/{autoscaling-tab,nodes-tab,copy-name}.tsx` | Task 6 |
| `web/src/features/report/quality-tab.tsx` | Task 7 |
| `web/src/features/reports/progress-page.tsx` | `Gone` exported; the link of R217 |
| `web/src/app/router.tsx`, `web/src/locales/*.json`, `web/src/i18n/families.ts`, `web/src/theme/pairs.ts`, `web/tests/msw.ts`, `web/tests/screens.test.tsx`, `web/src/components/no-style.test.tsx` | Shared; each task adds its part |
| `web/e2e/reports.spec.ts`, `Makefile` (`e2e`) | Task 9 |
| `AGENTS.md`, `README.md`, `CHANGELOG.md` | Task 10 |

---

### Task 0: Open the part

**Executor:** Main thread.

- [x] **Step 1:** `ROADMAP.md` and `CHANGELOG.md` read and true; `main` at `90c4f38`, pushed; tag `v0.4.0` local.
- [x] **Step 2:** `git switch -c feat/report-view`; the host commits; `make gate` passed on `56b6e4a` (`.cache/nixos-gate/`).
- [x] **Step 3:** The brainstorm (answers 1 to 12) and the spec amendment `61b7fbb`, approved by the owner on 2026-10-08.
- [ ] **Step 4:** `ROADMAP.md`: a section "Phase 4.4" with P44-0 to P44-10 `OPEN`, the rulings R213 to R231 named, the host commits under P44-0; the plan's commit named in P44-0.

---

### Task 1: The view's enums and the golden views (R213, R214)

**Executor:** Subagent; then the main thread runs `make update-testdata` and commits the four files (`test(web): add the golden views`, 31 characters).

**Files:**
- Modify: `api/openapi.yaml` (the ten schemas and the `$ref`s of R213), `api/oapi-codegen.yaml` (exclude the ten), `internal/snapshot/snapshot.go` (`Coverages`), `internal/snapshot/codec.go` (`AutoscalingStatuses`, `AutoscalingSources`, and their uses), `internal/goldentest/goldentest.go`, `cmd/update-testdata/main.go`
- Generated: `web/src/api/schema.gen.ts`, `internal/platform/api/openapi.gen.go` (`make generate`)
- Test: `internal/platform/api/view_contract_test.go`, `internal/goldentest/golden_test.go`

**Interfaces:**

```go
// internal/snapshot
var Coverages = []string{CoverageCovered, CoverageUncovered, CoverageUnknown}
var AutoscalingStatuses = []string{collect.AutoscalingEnabled, collect.AutoscalingDisabled, collect.AutoscalingUnknown}
var AutoscalingSources = []string{discovery.StrategyConfigured, discovery.StrategyRawMetrics, discovery.StrategyNone}

// internal/goldentest
var ViewScenarios = []string{"edge-scaling", "direct-complete"}
func ViewFile(snap []byte, lang catalog.Language) ([]byte, error)
func ViewFilePath(scenario string, lang catalog.Language) (string, error) // web/tests/fixtures/view/<scenario>.<lang>.json
```

Produces for the frontend (names as `openapi-typescript --enum-values` writes them; R227): `viewSeverityValues`, `viewScopeTypeValues`, `viewConfidenceValues`, `viewHPACoverageValues`, `viewAutoscalingStatusValues`, `viewAutoscalingSourceValues`, `viewStrategyValues`, `viewCapabilityNameValues`, `viewCapabilityStateValues`, `viewEvidenceNameValues`.

- [ ] **Step 1: Write the failing tests**

| Test | Input | Expected |
|---|---|---|
| `TestViewEnumsEqualTheRegistries` | each of the ten enums read from `api/openapi.yaml` | equal as sets to its Go list (R213); the failure names the enum and the values missing on each side |
| `TestViewMatchesTheContract` (existing) | every golden snapshot, both languages | passes with the enums in place; a scratch edit that removes `cluster` from `ViewScopeType` makes it fail (seen, then reverted) |
| `TestGoldenViewFiles` | the two scenarios, both languages | each file equals `ViewFile` of the golden snapshot; a missing file fails naming `make update-testdata` |

- [ ] **Step 2:** `go test ./internal/platform/api/ ./internal/goldentest/ -run 'ViewEnums|ViewMatches|GoldenViewFiles'`. Expected: FAIL.
- [ ] **Step 3:** Implement; `make generate`.
- [ ] **Step 4:** `make lint`, `make test`, `make generate-check`. All pass except `TestGoldenViewFiles`, which waits for the main thread's `make update-testdata`; the report says so.
- [ ] **Step 5:** Commit `feat(api): enumerate the view's closed sets` (43 characters).
- [ ] **Step 6 (main thread):** `make update-testdata`; read the four files whole; `make test`; commit them.

**Done when:** every row passes after the main thread's commit, and `make generate-check` is clean.

---

### Task 2: The frame, the states, the PDF button, the way in (R215, R216, R217)

**Executor:** Subagent. Follows Task 1.

**Files:**
- Create: `web/src/features/report/{api.ts,tabs.ts,report-page.tsx,report-page.test.tsx}`
- Modify: `web/src/app/router.tsx`, `web/src/features/reports/progress-page.tsx` (export `Gone`; the link), `web/src/features/reports/progress-page.test.tsx`, `web/tests/msw.ts` (R215's view handler), `web/src/locales/{en,pt-BR}.json`, `web/src/i18n/families.ts` (`report.tab`)

**Interfaces:**

```ts
// api.ts
export type ReportView = Schemas["ReportView"];
export type ViewFinding = Schemas["ViewFinding"]; // and ViewValue, ViewStatistic, ViewNamespace, ViewWorkload, ViewHPA, ViewNode, ViewNodepool, ViewCapability, ViewMessage
export function useReportView(id: string, language: Language): UseQueryResult<ReportView>;
// tabs.ts
export const reportTabs = ["overview", "findings", "namespaces", "workloads", "autoscaling", "nodes", "quality"] as const;
export type ReportTab = (typeof reportTabs)[number];
// report-page.tsx
export function ReportPage(): JSX.Element;
export function useView(): ReportView;
// progress-page.tsx
export function Gone({ message }: { message: string }): JSX.Element;
// tests/msw.ts: Mock gains views: Map<string, ReportView> keyed `${id}:${lang}`, and viewFailure: Map<string, { status: number; code: ErrorCode }>;
// GET /reports/:id/view answers the failure when set, else the view, else 404 REPORT_NOT_FOUND.
```

In this task the router's tab children are `index` → `overview` and `overview` → a component that renders `report.tab.overview` as an `h2`; Task 3 replaces it.

**The words** (both files):

| Key | English | Português (Brasil) |
|---|---|---|
| `report.tabs` | Report sections | Seções do relatório |
| `report.tab.overview` | Overview | Visão geral |
| `report.tab.findings` | Findings | Achados |
| `report.tab.namespaces` | Namespaces | Namespaces |
| `report.tab.workloads` | Workloads | Workloads |
| `report.tab.autoscaling` | Autoscaling | Autoscaling |
| `report.tab.nodes` | Nodes | Nodes |
| `report.tab.quality` | Data quality | Qualidade dos dados |
| `report.cluster` | Cluster | Cluster |
| `report.open` | Open report | Abrir relatório |

- [ ] **Step 1: Write the failing tests** (`report-page.test.tsx`, the golden view `edge-scaling` through MSW)

| Test | Input | Expected |
|---|---|---|
| `header facts` | `/reports/<id>/overview`, en | heading `Report on Production`; `Cluster` and `prod` (mono); the view's `period_text`; `Detailed`; `Requested … by …`; no time zone |
| `seven tabs` | the same | a nav named `report.tabs` with seven links in order; Overview `aria-current="page"`; each link's `href` ends in its tab |
| `index opens the overview` | `/reports/<id>` | location `/reports/<id>/overview` |
| `unknown tab` | `/reports/<id>/nope` | the not-found heading inside the shell |
| `pdf button` | open it as an operator; then as a reader | the popover holds the PDF block, interface language first; the operator sees Generate PDF for the missing language, the reader sees downloads alone |
| `states` (Review Focus 5) | view 409 `SNAPSHOT_NOT_READY`; 410; 404; 500 | location `/jobs/<id>`; `progress.expired`; `progress.notFound`; `error.INTERNAL_ERROR` text and a link to `/reports` |
| `language` | switch to pt-BR | a request for the view with `lang=pt-BR`; Portuguese tab names |
| `open report link` (`progress-page.test.tsx`) | a collected report; a running one; an expired collected one | the link to `/reports/<id>/overview`; none; none |
| locale tests (existing) | both files | same keys, same parameters, banned terms absent, every key used |

- [ ] **Step 2:** `cd web && npx vitest run src/features/report src/features/reports/progress-page.test.tsx`. Expected: FAIL.
- [ ] **Step 3:** Implement R215 to R217; add the words and the family.
- [ ] **Step 4:** `make lint`, `make test`. All pass.
- [ ] **Step 5:** Commit `feat(web): frame the report and its tabs` (40 characters).

---

### Task 3: The overview (R218, R220)

**Executor:** Subagent. Follows Task 2.

**Files:**
- Create: `web/src/features/report/{overview-tab.tsx,overview-tab.test.tsx,pieces.tsx}`
- Modify: `web/src/app/router.tsx` (the `overview` child), `web/src/locales/{en,pt-BR}.json`, `web/src/i18n/families.ts` (`confidence`), `web/tests/screens.test.tsx` (the overview, both languages and themes)

**Interfaces:** R220's pieces; this task writes `Counts`, `ValueText`, and the module; `PeakCell`, `StatGrid`, `Facts`, and `RowFindings` follow in Task 4.

```ts
export function ValueText({ value }: { value: ViewValue }): JSX.Element;
export function Counts({ label, items }: { label: string; items: { label: string; value: ViewValue }[] }): JSX.Element;
```

**The words** (both files):

| Key | English | Português (Brasil) |
|---|---|---|
| `report.overview.seeFindings` | See findings | Ver achados |
| `report.overview.inventory` | Inventory | Inventário |
| `report.inventory.namespaces` | Namespaces | Namespaces |
| `report.inventory.workloads` | Workloads | Workloads |
| `report.inventory.hpas` | HPAs | HPAs |
| `report.inventory.workloadsWithoutHpa` | Workloads without HPA | Workloads sem HPA |
| `report.inventory.nodes` | Nodes | Nodes |
| `report.inventory.nodepools` | Node pools | Node pools |
| `report.plan.title` | Prioritized action plan | Plano de ação priorizado |
| `report.plan.order` | # | # |
| `report.plan.priority` | Priority | Prioridade |
| `report.plan.action` | Action | Ação |
| `report.plan.scope` | Scope | Escopo |
| `report.plan.confidence` | Confidence | Confiança |
| `report.plan.empty` | No prioritized action was identified. | Nenhuma ação priorizada foi identificada. |
| `report.about.title` | About this report | Sobre este relatório |
| `report.about.timezone` | Time zone | Fuso horário |
| `report.about.namespaces` | Namespaces | Namespaces |
| `report.about.minSeverity` | Minimum severity | Severidade mínima |
| `report.about.issued` | Issued | Emitido em |
| `report.about.version` | Generator version | Versão do gerador |
| `confidence.exact` | Exact | Exata |
| `confidence.inferred` | Inferred | Inferida |
| `confidence.unknown` | Unknown | Desconhecida |

- [ ] **Step 1: Write the failing tests** (`overview-tab.test.tsx`, the golden view `edge-scaling`)

| Test | Input | Expected |
|---|---|---|
| `severity tiles` | en | three tiles in the order critical, attention, healthy, with `9`, `10`, `7`; each link's `href` ends in `findings?severity=<severity>` |
| `inventory` | en | six counts in order with `1`, `1`, `4`, `2`, `4`, `3`; each number precedes its name in the DOM order of its item |
| `action plan` | en | rows equal to `view.recommendations` in order; row 1: `1`, Critical, its action, `web`, Exact |
| `overview empty plan` (Review Focus 1) | the view with `recommendations: []` | `report.plan.empty`; no table |
| `about folded` | en; then click the summary | the `details` is closed; then shows `America/Sao_Paulo`, `payments`, `All`, the issue text, `0.0.0-golden` |
| `language switch keeps the place` (Review Focus 3) | on `/reports/<id>/overview`, switch to pt-BR | location unchanged; `Plano de ação priorizado`; the plan's actions are the pt-BR view's |
| screens (new entries) | the overview, both languages and themes | axe and vocabulary pass |

- [ ] **Step 2:** Run; expected FAIL.
- [ ] **Step 3:** Implement R218 and the pieces it uses.
- [ ] **Step 4:** `make lint`, `make test`. All pass.
- [ ] **Step 5:** Commit `feat(web): show the report overview` (35 characters).

---

### Task 4: The table, namespaces, and workloads (R219, R220, R221, R223, R228)

**Executor:** Subagent. Follows Task 3.

**Files:**
- Modify: `web/package.json`, `web/package-lock.json` (`npm install --save-exact @tanstack/react-table@9.2.6`), `web/src/features/report/pieces.tsx` (`PeakCell`, `StatGrid`, `Facts`, `RowFindings`, the matchers), `web/src/app/router.tsx`, `web/src/locales/{en,pt-BR}.json`, `web/src/i18n/families.ts` (`hpaCoverage`), `web/src/theme/pairs.ts` (R228), `web/tests/screens.test.tsx`
- Create: `web/src/features/report/{data-table.tsx,data-table.test.tsx,namespaces-tab.tsx,workloads-tab.tsx,tables.test.tsx}`

**Interfaces:**

```ts
export interface ReportColumn<Row> {
  id: string;
  header: string;
  cell: (row: Row) => ReactNode;
  sortValue?: (row: Row) => number | string | null; // a column without it does not sort
  align?: "start" | "end";
}
export function DataTable<Row>(props: {
  label: string;
  searchLabel: string;
  rows: readonly Row[];
  rowKey: (row: Row) => string;
  rowName: (row: Row) => string;
  columns: ReportColumn<Row>[];
  searchText: (row: Row) => string;
  renderOpened: (row: Row) => ReactNode;
  defaultOrder?: "status" | "view"; // "status": by severity rank then name (R219); "view": the view's order (R226)
  status?: (row: Row) => string; // the row's ViewSeverity, for the default order
}): JSX.Element;
export function PeakCell(props: { peak: ViewValue; share: ViewValue; word: "request" | "requested" }): JSX.Element;
export function StatGrid(props: { title?: string; rows: { label: string; stat: ViewStatistic }[] }): JSX.Element;
export function Facts(props: { items: { label: string; value: ReactNode }[] }): JSX.Element;
export function RowFindings(props: { match: (finding: ViewFinding) => boolean }): JSX.Element | null;
export const matchNamespace: (row: ViewNamespace) => (f: ViewFinding) => boolean; // and matchWorkload, matchHPA, matchNode (R221)
```

**The words** (both files):

| Key | English | Português (Brasil) |
|---|---|---|
| `report.table.rows` | Rows: {{count}} | Linhas: {{count}} |
| `report.table.empty` | No rows in this report. | Nenhuma linha neste relatório. |
| `report.table.noMatch` | No row matches the search. | Nenhuma linha corresponde à busca. |
| `report.table.open` | Show the details of {{name}} | Mostrar os detalhes de {{name}} |
| `report.table.close` | Hide the details of {{name}} | Ocultar os detalhes de {{name}} |
| `report.search.namespaces` | Search namespaces | Buscar namespaces |
| `report.search.workloads` | Search workloads | Buscar workloads |
| `report.column.namespace` | Namespace | Namespace |
| `report.column.workload` | Workload | Workload |
| `report.column.status` | Status | Status |
| `report.column.pods` | Pods | Pods |
| `report.column.cpuPeak` | CPU · Peak | CPU · Pico |
| `report.column.memoryPeak` | Memory · Peak | Memória · Pico |
| `report.share.request` | {{value}} of request | {{value}} do request |
| `report.share.requested` | {{value}} requested | {{value}} em request |
| `report.stat.average` | Average | Média |
| `report.stat.minimum` | Minimum | Mínimo |
| `report.stat.peak` | Peak | Pico |
| `report.stat.current` | Current | Atual |
| `report.stat.cpu` | CPU | CPU |
| `report.stat.memory` | Memory | Memória |
| `report.stat.usage` | Usage | Uso |
| `report.stat.request` | Request | Request |
| `report.stat.limit` | Limit | Limit |
| `report.stat.requestPercent` | % Request | % Request |
| `report.stat.limitPercent` | % Limit | % Limit |
| `report.facts.pods` | Pods | Pods |
| `report.facts.hpa` | HPA | HPA |
| `report.facts.values` | Values | Valores |
| `report.facts.measured` | Measured | Medidos |
| `report.facts.approximate` | Approximate | Aproximados |
| `hpaCoverage.covered` | Covered by an HPA | Coberto por um HPA |
| `hpaCoverage.uncovered` | No HPA | Sem HPA |
| `hpaCoverage.unknown` | Unknown | Desconhecida |

`StatGrid`'s title cell for CPU is `report.stat.cpu`, for memory `report.stat.memory`; its row labels are `report.stat.usage`, `.request`, `.limit`, `.requestPercent`, `.limitPercent`.

- [ ] **Step 1: Write the failing tests** (`data-table.test.tsx` on a small table of fixed rows; `tables.test.tsx` on the golden views)

| Test | Input | Expected |
|---|---|---|
| `default order` | rows healthy `b`, critical `c`, attention `a`, critical `a` | critical `a`, critical `c`, attention `a`, healthy `b` |
| `sort by a column` | click the CPU header twice | ascending then descending; `aria-sort` follows; the address holds `sort=cpu.asc` then `sort=cpu.desc` |
| `nulls sort last` (Review Focus 4) | raws 3, null, 1 in both directions | `1, 3, null` then `3, 1, null`; the null cell shows the view's placeholder text, muted |
| `search` | type `WEB` | only rows whose search text holds `web`; `Rows: N` follows; the address holds `q=WEB` |
| `no match` and `empty table` (Review Focus 1) | a search matching nothing; no rows | `report.table.noMatch`; `report.table.empty` |
| `open one row` | open row 1, then row 2 | one opened row at a time; `aria-expanded` follows; the address holds `open=<key>`; the opened cell spans every column |
| `stale address` (Review Focus 2) | `?open=nope&sort=nope.up&q=` | the default order; no row opened; no error |
| `address restores` | `?q=api&sort=pods.desc&open=<key>` | that search, that order, that row opened |
| `language switch keeps the opened row` (Review Focus 3) | the workloads tab with `payments-web` opened and `q=pay`, switch to pt-BR | location unchanged; the row still opened, its grid in Portuguese |
| `workloads columns` | the workloads tab, `edge-scaling`, en | only the workloads with `observed_usage`; `payments-web` with `Deployment` and `payments`, Critical, `1`, `400 mCPU` with the attention shape and `80% of request`, `120 B` with the critical shape and `24000% of request` |
| `workload opened` | open `payments-web` | Facts (Pods 1, Covered by an HPA, Measured); the CPU grid's rows and the four columns, `% Request` peak `80%` with the attention shape; its four findings, each a link whose `href` ends in `findings?finding=<index>` |
| `namespaces` | the namespaces tab, open `payments` | its row and its two grids and its findings |
| screens (new entries) | the two tabs with a row opened, both languages and themes | axe and vocabulary pass |
| contrast (existing test) | the new pairs of R228 | pass in both themes, or the task stops and reports |

- [ ] **Step 2:** Run; expected FAIL.
- [ ] **Step 3:** Install the dependency; implement R219 to R221 and the two tabs of R223; add the pairs and the words.
- [ ] **Step 4:** `make lint`, `make test`. All pass.
- [ ] **Step 5:** Commit `feat(web): show namespaces and workloads` (40 characters).

---

### Task 5: The findings (R222)

**Executor:** Subagent. Follows Task 4.

**Files:**
- Create: `web/src/features/report/{findings-tab.tsx,findings-tab.test.tsx}`
- Modify: `web/src/app/router.tsx`, `web/src/locales/{en,pt-BR}.json`, `web/src/i18n/families.ts` (`scope`, `evidence`), `web/tests/screens.test.tsx`

**The words** (both files):

| Key | English | Português (Brasil) |
|---|---|---|
| `report.findings.search` | Search findings | Buscar achados |
| `report.findings.severity` | Severity | Severidade |
| `report.findings.scope` | Scope | Escopo |
| `report.findings.allScopes` | All scopes | Todos os escopos |
| `report.findings.none` | This report holds no findings. | Este relatório não tem achados. |
| `report.findings.noMatch` | No finding matches these filters. | Nenhum achado corresponde a estes filtros. |
| `report.findings.observed` | Observed | Observado |
| `report.findings.action` | What to do | O que fazer |
| `report.findings.evidence` | Evidence | Evidências |
| `report.confidence` | Confidence: {{value}} | Confiança: {{value}} |
| `report.node.unknownPool` | Unknown pool | Node pool desconhecido |
| `scope.namespace` | Namespace | Namespace |
| `scope.workload` | Workload | Workload |
| `scope.hpa` | HPA | HPA |
| `scope.node` | Node | Node |
| `scope.nodepool` | Node pool | Node pool |
| `scope.cluster` | Cluster | Cluster |
| `evidence.affected_nodes` | Affected nodes | Nodes afetados |
| `evidence.average` | Average | Média |
| `evidence.average_difference` | Average difference | Diferença média |
| `evidence.average_percent` | Average | Média |
| `evidence.current_average` | Current replicas, average | Replicas atuais, média |
| `evidence.current_nodes` | Current nodes | Nodes atuais |
| `evidence.current_peak` | Current replicas, peak | Replicas atuais, pico |
| `evidence.gap_percent` | Gap between peak and average | Distância entre pico e média |
| `evidence.max_nodes` | Maximum nodes | Máximo de nodes |
| `evidence.maximum_average_percent` | Highest average | Maior média |
| `evidence.maximum_peak_percent` | Highest peak | Maior pico |
| `evidence.min_nodes` | Minimum nodes | Mínimo de nodes |
| `evidence.minimum_peak_percent` | Lowest peak | Menor pico |
| `evidence.node_pressure` | Node pressure | Pressão no node |
| `evidence.peak` | Peak | Pico |
| `evidence.peak_percent` | Peak | Pico |
| `evidence.percent` | Share | Percentual |
| `evidence.resources` | Resources | Recursos |
| `evidence.spec_max` | Maximum replicas | Máximo de replicas |
| `evidence.spec_min` | Minimum replicas | Mínimo de replicas |
| `evidence.spread_percent` | Spread | Dispersão |

The `evidence` family equals `viewEvidenceNameValues`; when the generated list holds a name this table lacks, the task stops and reports it, and the main thread words it.

- [ ] **Step 1: Write the failing tests** (`findings-tab.test.tsx`, the golden view `edge-scaling`)

| Test | Input | Expected |
|---|---|---|
| `grouped, critical first` | en | three headings in the order critical, attention, healthy, with their counts; items in view order inside each |
| `chips` | press Healthy | Healthy `aria-pressed="false"`; no healthy item; the address holds `severity=critical,attention` |
| `see findings arrives` | `?severity=critical` | only the critical chip pressed; only critical items |
| `scope select` | choose Node | only node findings; the address holds `scope=node`; the options are All scopes and the scope types present |
| `search` | type `oomkill` | only the findings whose sentence, action, or scope name holds it |
| `default selection` | no `finding` in the address | the first shown finding is shown whole; the address is not written |
| `select one` | click the third item | `aria-pressed` on it; the detail shows its scope name, sentence, action, evidence with `evidence.<name>`; the address holds `finding=<index>`; no code anywhere |
| `stale finding` (Review Focus 2) | `?finding=99&severity=bogus` | all chips pressed; the first finding shown whole; no error |
| `no findings` (Review Focus 1) | a view with `findings: []` | `report.findings.none` |
| `no match` | every chip off | `report.findings.noMatch` |
| `unknown pool and cluster scopes` | the node pool finding named `unknown`; the cluster finding | `Unknown pool` as plain text; the cluster finding shows `Cluster` and no name |
| `from a row` | `?finding=<index of a healthy finding>`, no `severity` | every chip pressed; that finding shown whole |
| screens (new entries) | the tab with a finding selected, both languages and themes | axe and vocabulary pass |

- [ ] **Step 2:** Run; expected FAIL.
- [ ] **Step 3:** Implement R222; add the words and the families.
- [ ] **Step 4:** `make lint`, `make test`. All pass.
- [ ] **Step 5:** Commit `feat(web): show the findings of a report` (40 characters).

---

### Task 6: Autoscaling and nodes (R223, R224, R225)

**Executor:** Subagent. Follows Task 5.

**Files:**
- Create: `web/src/features/report/{autoscaling-tab.tsx,nodes-tab.tsx,copy-name.tsx,autoscaling-nodes.test.tsx}`
- Modify: `web/src/app/router.tsx`, `web/src/locales/{en,pt-BR}.json`, `web/src/i18n/families.ts` (`autoscalingStatus`, `strategy`), `web/tests/screens.test.tsx`

**Interfaces:**

```ts
export function CopyName({ name }: { name: string }): JSX.Element; // R224
```

**The words** (both files):

| Key | English | Português (Brasil) |
|---|---|---|
| `report.autoscaling.title` | Cluster autoscaling | Autoscaling do cluster |
| `report.autoscaling.source` | Source: {{value}} | Fonte: {{value}} |
| `report.autoscaling.currentNodes` | Current nodes | Nodes atuais |
| `report.autoscaling.minNodes` | Minimum nodes | Mínimo de nodes |
| `report.autoscaling.maxNodes` | Maximum nodes | Máximo de nodes |
| `report.autoscaling.managedPools` | Managed node pools | Node pools gerenciados |
| `report.autoscaling.scaleEvents` | Scale events | Eventos de escala |
| `report.autoscaling.unschedulablePods` | Unschedulable pods | Pods não agendáveis |
| `report.hpas.title` | HPAs | HPAs |
| `report.search.hpas` | Search HPAs | Buscar HPAs |
| `report.search.nodes` | Search nodes | Buscar nodes |
| `report.column.hpa` | HPA | HPA |
| `report.column.situation` | Situation | Situação |
| `report.column.replicaRange` | Min / Max | Mín. / Máx. |
| `report.column.replicasPeak` | Replicas · Peak | Replicas · Pico |
| `report.column.node` | Node | Node |
| `report.column.state` | State | Estado |
| `report.column.nodepool` | Node pool | Node pool |
| `report.column.confidence` | Confidence | Confiança |
| `report.column.nodes` | Nodes | Nodes |
| `report.stat.currentReplicas` | Current replicas | Replicas atuais |
| `report.stat.desiredReplicas` | Desired replicas | Replicas desejadas |
| `report.stat.cpuPercent` | CPU % | CPU % |
| `report.stat.memoryPercent` | Memory % | Memória % |
| `report.facts.cpuCapacity` | CPU capacity | Capacidade de CPU |
| `report.facts.cpuAllocatable` | CPU allocatable | CPU alocável |
| `report.facts.memoryCapacity` | Memory capacity | Capacidade de memória |
| `report.facts.memoryAllocatable` | Memory allocatable | Memória alocável |
| `report.node.ready` | Ready | Pronto |
| `report.node.notReady` | Not ready | Não pronto |
| `report.node.schedulable` | Schedulable | Agendável |
| `report.node.unschedulable` | Unschedulable | Não agendável |
| `report.node.unknownState` | Unknown | Desconhecido |
| `report.pools.title` | Node pools | Node pools |
| `report.copy` | Copy {{name}} | Copiar {{name}} |
| `report.copied` | Copied: {{name}} | Copiado: {{name}} |
| `report.copyFailed` | The name could not be copied. | Não foi possível copiar o nome. |
| `autoscalingStatus.enabled` | Enabled | Ativo |
| `autoscalingStatus.disabled` | Disabled | Inativo |
| `autoscalingStatus.unknown` | Unknown | Desconhecido |
| `strategy.recording_rule` | Recording rule | Recording rule |
| `strategy.raw_metrics` | Raw metrics | Raw metrics |
| `strategy.configured` | Configuration | Configuração |
| `strategy.none` | No source | Sem fonte |

The node state reads `{ready} · {schedulable}` joined in the component with the two words; no key holds the separator.

- [ ] **Step 1: Write the failing tests** (`autoscaling-nodes.test.tsx`, the golden view `edge-scaling`)

| Test | Input | Expected |
|---|---|---|
| `cluster line and counts` | the autoscaling tab, en | `Enabled`, `Source: Raw metrics`, `Confidence: Inferred`; seven counts `3, 2, 3, 2, 1, 2, 2` in R225's order |
| `hpa rows` | the same | four HPAs; `web` with `Deployment payments-web · payments`, `Maximum reached` with the critical shape, `2 / 4`, `4`; `dup` with `1 / Unavailable` |
| `hpa opened` | open `web` | a grid of current and desired replicas; its finding, a link to the findings |
| `node rows` | the nodes tab | `orphan` with `Unknown pool` as plain text and `Unknown` state; `pool-a-2` `Not ready · Unschedulable` in the attention colour; `95%` with `25% requested` |
| `node opened` | open the first node | the name with the copy control, the reason with its mark; capacity and allocatable; the CPU % and memory % grid; its findings |
| `copy` | press the copy control; then with `writeText` rejecting | `writeText("aks-agentpool-12345678-vmss000000")` and `Copied: …` announced; `report.copyFailed` announced |
| `pools` | the pools table | `agentpool` and `pool-a` with a copy control; `Unknown pool` with none; counts `1, 2, 1` with the names |
| screens (new entries) | the two tabs with a row opened, both languages and themes | axe and vocabulary pass |

- [ ] **Step 2:** Run; expected FAIL.
- [ ] **Step 3:** Implement; add the words and the families.
- [ ] **Step 4:** `make lint`, `make test`. All pass.
- [ ] **Step 5:** Commit `feat(web): show autoscaling and nodes` (37 characters).

---

### Task 7: Data quality (R226)

**Executor:** Subagent. Follows Task 6.

**Files:**
- Create: `web/src/features/report/{quality-tab.tsx,quality-tab.test.tsx}`
- Modify: `web/src/app/router.tsx`, `web/src/locales/{en,pt-BR}.json`, `web/src/i18n/families.ts` (`capability`, `capabilityState`), `web/tests/screens.test.tsx`

**The words** (both files):

| Key | English | Português (Brasil) |
|---|---|---|
| `report.quality.collected` | Collected from {{start}} to {{end}}, in {{elapsed}} | Coletado de {{start}} a {{end}}, em {{elapsed}} |
| `report.quality.metrics` | Metrics read | Métricas lidas |
| `report.quality.warnings` | Warnings | Avisos |
| `report.quality.notices` | Notices | Notas |
| `report.quality.noWarnings` | No warning. | Nenhum aviso. |
| `report.quality.noNotices` | No notice. | Nenhuma nota. |
| `report.search.capabilities` | Search capabilities | Buscar capacidades |
| `report.column.capability` | Capability | Capacidade |
| `report.column.source` | Source | Fonte |
| `report.column.remark` | Remark | Observação |
| `capability.namespace_usage` | Usage by namespace | Uso por namespace |
| `capability.workload_usage` | Usage by workload | Uso por workload |
| `capability.requests_limits` | Requests and limits | Requests e limits |
| `capability.hpa_history` | HPA history | Histórico de HPA |
| `capability.workloads_without_hpa` | Coverage of workloads without HPA | Cobertura de workloads sem HPA |
| `capability.ip_node_mapping` | Mapping between address and node | Associação entre endereço e node |
| `capability.nodepool_discovery` | Identification of node pools | Identificação de node pools |
| `capability.node_utilization` | Node utilization | Utilização dos nodes |
| `capability.node_autoscaling` | Node autoscaling | Autoscaling de nodes |
| `capabilityState.available` | Available | Disponível |
| `capabilityState.partial` | Partial | Parcial |
| `capabilityState.unavailable` | Unavailable | Indisponível |

The `report.column.state` key of Task 6 heads the state column.

- [ ] **Step 1: Write the failing tests** (`quality-tab.test.tsx`, the golden view `edge-scaling`)

| Test | Input | Expected |
|---|---|---|
| `collection line` | en, `TZ=UTC` | `Collected from 2026-07-15 17:30 to 2026-07-15 17:30, in 0:42` |
| `capabilities in view order` | the same | nine rows in the view's order; `HPA history` with the attention shape and `Partial`, `Raw metrics`, its remark |
| `metric names only here` | open `HPA history` | its five metric names, mono; no metric name appears in any other tab's text (render each tab; none holds `kube_`) |
| `warnings and notices` | the same | two warnings with the attention shape; one notice |
| `no warnings` (Review Focus 1) | a view with `warnings: []` and `notices: []` | `No warning.` and `No notice.` |
| screens (new entries) | the tab with a capability opened, both languages and themes | axe and vocabulary pass |

- [ ] **Step 2:** Run; expected FAIL.
- [ ] **Step 3:** Implement; add the words and the families.
- [ ] **Step 4:** `make lint`, `make test`. All pass, `every key is used` included.
- [ ] **Step 5:** Commit `feat(web): show the data quality` (32 characters).

---

### Task 8: No inline style on the report (R229)

**Executor:** Subagent. Follows Task 7.

**Files:** Modify `web/src/components/no-style.test.tsx`.

- [ ] **Step 1: Write the test**

| Test | Input | Expected |
|---|---|---|
| `the report writes no style` | for each of the seven tabs: render it on the golden view, open a row where the tab has a table, select a finding on the findings tab, open the PDF popover on the overview | no element outside `[data-radix-popper-content-wrapper]` has a `style` attribute; no `<style>` was added |

- [ ] **Step 2:** Prove it bites: a scratch `style={{ color: "red" }}` on the overview's heading makes it fail; reverted.
- [ ] **Step 3:** `make lint`, `make test`. All pass.
- [ ] **Step 4:** Commit `test(web): find no style on the report` (38 characters).

---

### Task 9: The browser (R230)

**Executor:** Subagent writes the change; the main thread runs `make e2e`.

**Files:** Modify `web/e2e/reports.spec.ts`, `Makefile` (the `e2e` target).

- [ ] **Step 1 (subagent):** In the spec, after "the progress page reaches Collected and the English PDF": a step "the operator reads the report" that follows `report.open`, sees the overview's plan heading, visits the seven tabs through the nav (each: a heading or table of the tab visible, `expectNoViolations`), opens the first row of the workloads table, and opens the PDF popover (`expectNoViolations`). Move `browser.newContext` and `watchPolicy` of the reader into the `try` whose `finally` closes the context. In the `e2e` target, after `up --abort-on-container-exit …` and before `down`, read `$(E2E_COMPOSE) logs --no-log-prefix fake-prometheus` and fail the target unless a line equals `remaining: 0`; the target's status is the first failure. `make lint` passes.
- [ ] **Step 2 (main thread):** the Docker checks of AGENTS.md; `make e2e` passes. Prove the fake check bites: a scratch change of the expected line to `remaining: 9` makes the target fail after the spec passes (log under `.cache/phase-4/part-4/e2e/`), reverted.
- [ ] **Step 3:** Commit `test(e2e): read the report in seven tabs` (40 characters).

**Done when:** `make e2e` passes twice in a row from a clean stack.

---

### Task 10: The combined run, the owner's look, documents, review, gate, close

**Executor:** Main thread.

- [ ] **Step 1:** `make test`, `make lint`, `make generate-check`, `make test-postgres`, `make image`, `make check-runtime`, `make e2e` over the combined branch; logs under `.cache/phase-4/part-4/combined/`.
- [ ] **Step 2: The owner's look.** The look stack (`.cache/phase-4/part-4/look/stack/up.sh`) with a collected report; screenshots of the real build at 1200×1000 (every tab, a row opened, the findings with a filter, the PDF popover open; light and dark; en and pt-BR) on the companion page beside the mockups the owner approved, opened. Deviations from an approved mockup that are small are ruled by the main thread and listed; a change the owner asks for is made and the screens taken again.
- [ ] **Step 3: `AGENTS.md`.** "The interface": the report route and its tabs, the view's closed sets as contract enums, the state of a tab in the address, `DataTable`. "Golden snapshots and data files": the golden views under `web/tests/fixtures/view/`, written by `make update-testdata`.
- [ ] **Step 4: `README.md`.** The web section names the report screen.
- [ ] **Step 5: `CHANGELOG.md`, under Unreleased.** `### Added`: a collected report opens from its progress page in seven tabs (overview, findings, namespaces, workloads, autoscaling, nodes, data quality), worded as its PDF, with its PDFs one click away. `### Fixed`: the development tools run on NixOS. Omitted, and said so: the contract enums, the golden views, the tests, the end-to-end changes.
- [ ] **Step 6: The adversarial review of the branch, by areas**, probes under `.cache/phase-4/part-4/review/probe/`: (a) the contract enums, the golden views, the host fixes; (b) the frame, the states, the address, the table; (c) the seven tabs against the spec and the view; (d) wording, contrast, accessibility, no inline style. One fix wave, then one re-review.
- [ ] **Step 7:** `make gate` with logs under `.cache/phase-4/part-4/gate/`.
- [ ] **Step 8: Close.** `ROADMAP.md` (P44 items, F-137, the E253 scope minor); the closing report `.cache/phase-4/part-4/closing-report.md`; the owner's word on the merge, the push, and version 0.5.0 (R231), each naming its target.

**Done when:** the gate passes from images built without cache, the review is clean, and the owner has the closing report.
