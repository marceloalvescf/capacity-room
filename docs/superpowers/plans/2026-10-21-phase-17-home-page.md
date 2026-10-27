# Phase 17: Home Page Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** `/` becomes Home. A strip shows every active cluster by its latest report, and below it the chosen cluster's answer, built from the report overview's own parts. One new read route, `GET /api/v1/home`, feeds it.

**Architecture:**
- **Store.** One statement reads, for each active profile, its newest live report and its newest collected report that has not expired. A new index backs it.
- **Service and API.** The reports service wraps both reads as summaries, and the API serves them as `Home`.
- **Web.** The overview's parts take the view and a base path as props, so the Overview tab and Home share one implementation. Home adds a chip strip, an unseen mark stored per user in `localStorage`, and the answer states.

**Tech Stack:**
- Go with pgx and PostgreSQL 18; goose migrations; oapi-codegen.
- React and TypeScript, TanStack Query, openapi-fetch, react-router, i18next.
- Vitest and Playwright.

**Spec:** `docs/superpowers/specs/2026-10-21-home-page-design.md` (e40c7bc). Read it before any task.

## Global Constraints

- Branch `feat/home-page`. Every commit names its paths (`git add <paths>`, never `-A`).
- A subagent never runs Docker. The main thread gives the DSN for `postgres`-tagged tests.
- Release 2.8.0, a minor version. The version commit is separate and comes at the end, on the owner's word.
- No skipped or focused tests, in Go, Vitest, or Playwright.
- Strict CSP: no `style` prop, no `dangerouslySetInnerHTML`, no injected `<style>`.
- Every visible text, accessible name, and title comes from `web/src/locales/en.json` and `pt-BR.json`, with the same keys and parameters in both.
  - Technical terms stay English in both languages.
  - No banned term (`internal/catalog/banned-terms.json`).
- Every foreground and background drawn together is listed in `web/src/theme/pairs.ts`.
- Names and numbers use the mono face. Motion appears only under `motion-safe:`.
- Every timestamp in SQL is passed from Go. No statement calls `now()`.
- The released migrations 00001 to 00006 are not edited. The new one is `00007_reports_by_profile.sql`.
- Layers:
  - `internal/platform/store` (120) is below `profiles` (145), which is below `reports` (146), which is below `api` (150).
  - `reports` must not import `api`.
  - Nothing outside `cmd/` imports `cmd/`.
- The contract: `api/openapi.yaml` and `Routes()` match both ways. `make generate-check` stays clean.
- Comments say what code does. They never name a task, phase, or step.

## Review Focus

1. **A newer deleted report.** A profile's newest row is deleted, and an older live one exists. `newest` must be the older live row, never the deleted one. The store test is in Task 1.
2. **The report expires between `GET /home` and its view.** The view answers 410. The answer section says the report is gone, and Home fetches `/home` again. The Vitest is in Task 6.
3. **A report limited to some namespaces.** The meta line names those namespaces in mono, so the mark is not read as the whole cluster. The Vitest is in Task 6.
4. **`?cluster=` names a profile no longer active.** The first chip is chosen, with no error. The Vitest is in Task 6.
5. **`localStorage` throws on every call.** Home renders, draws no unseen dot, and throws nothing. The Vitest is in Task 5.

---

### Task 1: Store read of each profile's reports

**Files:**
- Create: `internal/platform/store/migrations/00007_reports_by_profile.sql`
- Modify: `internal/platform/store/reports.go` (next to `ListReports`, around line 455)
- Test: `internal/platform/store/report_home_test.go` (`//go:build postgres`, package `store_test`)

**Interfaces:**
- Produces:
  - `type ProfileReports struct { ProfileID string; Latest *ReportListItem; Newest *ReportListItem }`
  - `func LatestReportsByProfile(ctx context.Context, q Querier, profileIDs []string, now time.Time) ([]ProfileReports, error)`
  - It returns one entry per id given, in the order given. An entry has nil pointers when the profile has no matching row.
  - `Latest` is the newest row (by `created_at DESC, id DESC`) with `deleted_at IS NULL`, `status = 'collected'`, `expired_at IS NULL`, and `expires_at > now`.
  - `Newest` is the newest row with `deleted_at IS NULL`.
  - Both carry `OwnerUsername` and `PDFs`: reuse `listColumns`, `scanItem`, and one `attachPDFs` call over every item found.

- [ ] **Step 1: Write the migration.** It is a goose file with `-- +goose Up`:

```sql
CREATE INDEX ix_reports_by_profile ON reports (profile_id, created_at DESC, id DESC) WHERE deleted_at IS NULL;
```

The Down section drops it.

- [ ] **Step 2: Write the failing tests** in `report_home_test.go`. Use `storetest.New`, `newReportFixture`, `mustInsertReport`, and `collectedReport` from the existing store tests.
  - `TestLatestReportsByProfileGivesBothReports`. A collected report at T1 and a queued one at T2 > T1: `Latest.ID` is the T1 row and `Newest.ID` is the T2 row.
  - `TestLatestReportsByProfileLeavesDeletedOut`. A collected report at T1, and a report at T2 with `deleted_at` set: `Newest.ID` and `Latest.ID` are both the T1 row (Review Focus 1).
  - `TestLatestReportsByProfileLeavesExpiredOutOfLatest`. A collected report whose `expires_at` equals `now`, and another with `expired_at` set: `Latest` is nil, and `Newest` is the newer of the two.
  - `TestLatestReportsByProfileKeepsTheGivenOrder`. Ids `["b","a","none"]` give three entries in that order. The `"none"` entry has both pointers nil.
  - `TestLatestReportsByProfileUsesTheIndex`. Copy the tracer and EXPLAIN pattern of `TestListReportsUsesTheIndex` (`report_read_test.go:192`): 10000 rows across 20 profiles, then `ANALYZE`. The plan text contains `ix_reports_by_profile`.

- [ ] **Step 3: Run the tests and see them fail.**

Run: `CAPACITYROOM_TEST_DATABASE_DSN=<dsn> go test -count=1 -tags=postgres ./internal/platform/store/ -run LatestReportsByProfile -v`

Expected: FAIL, undefined `store.LatestReportsByProfile`.

- [ ] **Step 4: Implement `LatestReportsByProfile`.** Use one statement over `unnest($1::text[]) WITH ORDINALITY` with two `LEFT JOIN LATERAL (... LIMIT 1)` subqueries, so each subquery walks `ix_reports_by_profile`. Order by ordinality. Scan both sides; a nil id means no row.

- [ ] **Step 5: Run the tests and see them pass.** Then run the whole package with the tag: `go test -count=1 -tags=postgres ./internal/platform/store/`. Expected: PASS.

- [ ] **Step 6: Commit.**

```bash
git add internal/platform/store/migrations/00007_reports_by_profile.sql internal/platform/store/reports.go internal/platform/store/report_home_test.go
git commit -m "feat(store): read each profile's latest reports"
```

### Task 2: Reports service `Home`

**Files:**
- Modify: `internal/platform/reports/read.go`
- Test: `internal/platform/reports/home_test.go` (`//go:build postgres`)

**Interfaces:**
- Consumes: `store.LatestReportsByProfile` (Task 1), and `profiles.Summary` (fields `ID` and `DisplayName`).
- Produces:
  - `type HomeCluster struct { ProfileID, DisplayName string; Latest, Newest *Summary }`
  - `func (s *Service) Home(ctx context.Context, _ auth.Principal, active []profiles.Summary) ([]HomeCluster, error)`
  - It reads `s.now()` once, passes it to the store, and builds each side with `summaryOf(item, now)`. The output keeps the order of `active`.

- [ ] **Step 1: Write the failing tests.**
  - `TestHomeKeepsTheActiveOrder`. Two summaries `{ID:"z"}` and `{ID:"a"}` give clusters `z` then `a`, with `DisplayName` copied.
  - `TestHomeReadsTheClockOnce`. `Now` counts its calls and the count is 1.
  - `TestHomeMarksAnExpiredNewest`. A collected report whose `expires_at` is before the fixed `Now` gives `Newest.Expired == true` and `Latest == nil`.

- [ ] **Step 2: Run the tests and see them fail.** Run `go test -count=1 -tags=postgres ./internal/platform/reports/ -run Home -v`. Expected: FAIL.

- [ ] **Step 3: Implement `Home`.**

- [ ] **Step 4: Run the tests and see them pass.** Then run the whole package with the tags `postgres template` (the worker runs under both; the reports package runs under `postgres` alone). Expected: PASS.

- [ ] **Step 5: Commit.** `git add` both files, then `git commit -m "feat(reports): serve each profile's latest reports"`.

### Task 3: The route `GET /api/v1/home` and its contract

**Files:**
- Modify: `api/openapi.yaml`. Add the path `/home` near `/profiles`, and the schemas `Home` and `HomeCluster` exactly as the spec section "The route `GET /home`" gives them.
- Modify: `internal/platform/api/routes.go`. Add `{"GET", "/api/v1/home", s.readerRoute(s.home)}`.
- Create: `internal/platform/api/home.go`
- Regenerate: `internal/platform/api/openapi.gen.go` and `web/src/api/schema.gen.ts` (`make generate`).
- Test: `internal/platform/api/home_test.go` (`//go:build postgres`, following the existing handler tests of `reports_test.go`).
- Modify: `AGENTS.md`. In "The platform", where it lists the report routes, add `GET /home` as a read for every signed-in user.

**Interfaces:**
- Consumes: `Profiles.ListActive`, `Reports.Home` (Task 2), and `reportSummaryOf`.
- Produces:
  - `func (s *Server) home(w http.ResponseWriter, r *http.Request, a auth.Authenticated)`. It calls `ListActive`, then `Reports.Home`, maps with `reportSummaryOf`, and writes `Home` with `writeJSON(w, http.StatusOK, out)`. Errors go through `s.fail`.
  - The generated TypeScript types `Schemas["Home"]` and `Schemas["HomeCluster"]`.

- [ ] **Step 1: Write the failing tests.**
  - `TestHomeNeedsASession`: no cookie gives 401 `UNAUTHENTICATED`.
  - `TestHomeAnswersEveryActiveProfile`: a reader session, one active and one inactive profile, and one collected report. The body has one cluster with `latest.id` equal to that report and `newest.id` equal to the same.
  - `TestRoutesMatchTheContract` fails until the yaml is added.

- [ ] **Step 2: Run the tests and see them fail.** Run `go test -count=1 -tags=postgres ./internal/platform/api/ -run 'Home|Contract' -v`. Expected: FAIL.

- [ ] **Step 3: Add the yaml.** Then run `make generate`, and add the route and the handler.

- [ ] **Step 4: Run the tests and see them pass.** Run them again, then `make test`, `make lint`, and `make generate-check`. Expected: PASS and a clean diff check.

- [ ] **Step 5: Commit.** `git add api/openapi.yaml internal/platform/api/routes.go internal/platform/api/home.go internal/platform/api/home_test.go internal/platform/api/openapi.gen.go web/src/api/schema.gen.ts AGENTS.md`, then `git commit -m "feat(api): add GET /home"`.

### Task 4: The overview's parts take their view and base

**Files:**
- Create: `web/src/features/report/overview-parts.tsx`. Move `GrowthPair`, `AnswerStrip`, `QualityNotice`, `PlanAction`, `scopeLabel`, and `Plan` here from `overview-tab.tsx`.
- Modify: `web/src/features/report/overview-tab.tsx`, `findings-chart.tsx`, and `findings-tab.tsx`.
- Test: `web/src/features/report/overview-parts.test.tsx`. Existing tests stay green.

**Interfaces:**
- Produces:
  - `type PartProps = { view: ReportView; base: string }`. `base` is `/reports/<id>`, with no trailing slash.
  - `export function AnswerStrip(props: PartProps)`
  - `export function QualityNotice(props: PartProps)`
  - `export function GrowthPair(props: PartProps)`
  - `export function Plan(props: PartProps & { limit?: number })`. With `limit`, it shows the first `limit` rows and then a link with `t("home.plan.all")` to `${base}/overview`.
  - `FindingsChart` props become `{ view: ReportView; base: string; linkSegments: boolean; under?: ReactNode }`.
- Every link is absolute:
  - `${base}/findings?severity=${s}`
  - `${base}/findings`
  - `${base}/quality`
  - `${base}/autoscaling`
  - `${base}/findings?scope=${scope_type}&finding=${finding}`
- `OverviewTab` and `FindingsTab` build `base` from the route's `id` param and pass `useView()`.

- [ ] **Step 1: Write the failing tests** in `overview-parts.test.tsx`, rendering with the golden view `web/tests/fixtures/view/direct-complete.en.json`:
  - `AnswerStrip` with `base="/reports/abc"` has a link with href `/reports/abc/findings?severity=critical`.
  - `Plan` with `limit={3}` renders 3 rows and the "all" link to `/reports/abc/overview`.
  - `FindingsChart` with `linkSegments={false}` links a scope's sentence to `/reports/abc/findings?scope=namespace&finding=0`.

- [ ] **Step 2: Run the tests and see them fail.** Run `cd web && npx vitest run src/features/report/overview-parts.test.tsx`. Expected: FAIL.

- [ ] **Step 3: Move and re-type the parts.** Update the two callers.

- [ ] **Step 4: Run the tests and see them pass.** Run `cd web && npx vitest run src/features/report/`. Every existing report test passes unchanged. Expected: PASS.

- [ ] **Step 5: Commit.** `git commit -m "refactor(web): share the overview's parts"`, with the five paths.

### Task 5: Home's data, chip rules, age, and the unseen store

**Files:**
- Create:
  - `web/src/features/home/api.ts`
  - `web/src/features/home/clusters.ts`
  - `web/src/features/home/seen.ts`
- Modify: `web/src/lib/format.ts`
- Test:
  - `web/src/features/home/clusters.test.ts`
  - `web/src/features/home/seen.test.ts`
  - `web/src/lib/format.test.ts`

**Interfaces:**
- Consumes: `Schemas["HomeCluster"]` (Task 3), `isActive` from `@/features/reports/api`, and `readStored` and `writeStored` from `@/lib/storage`.
- Produces:
  - In `api.ts`:
    - `export const homeKey = ["home"] as const`
    - `export function useHome()`. It is a `useQuery` on `api.GET("/home")` with `refetchInterval: q => q.state.data?.clusters.some(c => c.newest !== null && isActive(c.newest)) ? 5000 : false`.
  - In `clusters.ts`:
    - `export type ChipLine = { kind: "age"; at: string } | { kind: "step"; step: "queued" | "collecting" | "rendering" } | { kind: "failed" } | { kind: "expired" } | { kind: "none" }`
    - `export function chipOf(c: HomeCluster): { status: Status; line: ChipLine }`. It follows the spec's table of chip marks and the rules under it.
      - In flight takes precedence.
      - The step comes from `newest`: queued is `queued`, running is `collecting`, and collected with a pending PDF is `rendering`.
      - A failed newest gives `failed` only when `latest` is null.
    - `export function orderClusters(cs: HomeCluster[]): HomeCluster[]`. The rank is critical, attention, healthy, unavailable, undefined; then `display_name` by `localeCompare`.
  - In `seen.ts`:
    - `export function unseen(userId: string, c: HomeCluster): boolean`
    - `export function markSeen(userId: string, profileId: string, reportId: string): void`
    - The key is `capacityroom-home-seen:${userId}`, holding a JSON map. A profile with no entry is recorded and reported as seen. Storage failure reads as seen and writes nothing.
  - In `format.ts`: `export function formatAge(iso: string, now: number, language: string): string`, through `Intl.RelativeTimeFormat(language, { numeric: "always", style: "short" })`. The unit is the largest whole unit among minute, hour, day, and week; under one minute it is 0 minutes.

- [ ] **Step 1: Write the failing tests.**
  - `clusters.test.ts` has one case per row of the spec's table of chip marks:
    - live critical gives `{status:"critical", line:{kind:"age"}}`;
    - a null `worst_severity` gives `unavailable`;
    - only expired gives `unavailable` with `expired`;
    - none gives `undefined` with `none`;
    - running over live attention gives `attention` with step `collecting`;
    - a failed newest and no latest gives `failed`;
    - a failed newest over live healthy gives `age`.
  - `clusters.test.ts` also checks the order: names `b-crit`, `a-ok`, and `c-crit` come out as `c-crit`, `b-crit`, `a-ok` when the display names sort that way. Write literal names that make the expected order unambiguous.
  - `seen.test.ts`:
    - the first read records the report and returns false;
    - a changed `latest.id` returns true;
    - after `markSeen` it is false again;
    - two user ids do not share entries;
    - with `localStorage.getItem` and `setItem` throwing, the result is false and nothing throws (Review Focus 5).
  - `format.test.ts`: `formatAge("2026-10-21T12:00:00Z", Date.parse("2026-10-21T14:30:00Z"), "en")` returns `"2 hr. ago"`, and `"pt-BR"` gives Intl's own output for 2 hours. Assert both against literals copied from a run of Node 26's Intl.

- [ ] **Step 2: Run the tests and see them fail.** Run `cd web && npx vitest run src/features/home src/lib/format.test.ts`. Expected: FAIL.

- [ ] **Step 3: Implement the three modules and `formatAge`.**

- [ ] **Step 4: Run the tests and see them pass.** Expected: PASS.

- [ ] **Step 5: Commit.** `git commit -m "feat(web): add home data and chip rules"`, with the six paths.

### Task 6: The Home page, the route, and the navigation

**Files:**
- Create:
  - `web/src/features/home/home-page.tsx`
  - `web/src/features/home/cluster-strip.tsx`
  - `web/src/features/home/cluster-answer.tsx`
- Modify:
  - `web/src/app/router.tsx`: the index route renders `<HomePage />`.
  - `web/src/app/shell.tsx`: add `<NavLink to="/" end className={navLink}>{t("nav.home")}</NavLink>` as the first entry.
  - `web/src/features/report/report-page.tsx`: extract the meta line into `ReportMeta` in `web/src/features/report/report-meta.tsx`, with a `namespaces?: string[]` prop that only Home passes.
  - `web/src/locales/en.json` and `pt-BR.json`: `nav.home`, and a `home.*` group.
  - `web/src/theme/pairs.ts`: `line("fg", "surface")` for the chosen chip, and `line("accent", "surface")` and `line("accent", "bg")` for the dot, each only if not already listed.
  - `DESIGN.md`: the cluster chip, the unseen dot, and Home under Layout.
  - `PRODUCT.md`: Home under Capabilities.
  - `AGENTS.md`: the three rules under "Rules this adds to AGENTS.md" of the spec, in the section on the interface.
- Test: `web/src/features/home/home-page.test.tsx`

**Interfaces:**
- Consumes: `useHome`, `chipOf`, `orderClusters`, `unseen`, `markSeen`, `formatAge` (Task 5); the shared parts and `FindingsChart` (Task 4); `useReportView(id, language)` from `@/features/report/api`; `usePrincipal()`; `useScreenTitle`.
- Produces: `export function HomePage()`.

The behaviour follows the spec's sections "What the page shows", "When there is no answer", and "When there is no active profile":
- The chosen cluster is `?cluster`, written with `setSearchParams(..., { replace: true })`. An unknown one falls back to the first chip.
- The view language is the report screen's: `isLanguage(i18n.language) ? i18n.language : "en"`.
- The previous answer is held while the next view loads. Keep the last view in a ref. `useReportView` stays unchanged, because the report screen shares it.
- The page title comes from `useScreenTitle(t("home.title"))`.
- A 404, 409, or 410 of the view shows `home.answer.gone` and calls `queryClient.invalidateQueries({ queryKey: homeKey })`.
- Showing an answer calls `markSeen`.
- New report:
  - `/generate?from=<latest.id>` when `latest` exists;
  - otherwise `/generate?profile=<profile_id>`.
  - It is hidden when `principal.role === "reader"`.

- [ ] **Step 1: Write the failing tests** in `home-page.test.tsx`. Mock `/home` and `/reports/{id}/view` the way `reports-page.test.tsx` mocks the client, and use the golden view for the answer.
  - `"orders the chips worst first and chooses the first"`
  - `"chooses the cluster in the address"`
  - `"falls back to the first chip for an unknown cluster"` (Review Focus 4)
  - `"links the answer into the report"`: an `Open report` href of `/reports/<id>/overview`, and a severity link of `/reports/<id>/findings?severity=critical`.
  - `"names the report's namespaces"`: a selection with namespaces `["payments"]` shows `payments` in the meta line (Review Focus 3).
  - `"says a request is collecting, with Follow"`: the href is `/jobs/<newest.id>`.
  - `"says the newest request failed, above no report"`
  - `"offers a report request only to an operator"`: the reader sees no `New report` and no `Request a report`.
  - `"says the report is gone on a 410 and fetches home again"` (Review Focus 2)
  - `"shows the empty state to an administrator with a link to /profiles/new"`, and the other text to a reader.
  - `"draws the unseen dot with its accessible phrase"`
  - `"keeps the strip when the language changes"`: it switches i18n to `pt-BR` and expects a view request with `lang=pt-BR`.

- [ ] **Step 2: Run the tests and see them fail.** Run `cd web && npx vitest run src/features/home/home-page.test.tsx`. Expected: FAIL.

- [ ] **Step 3: Build the three components.** Then wire the router, the navigation, `ReportMeta`, the locales, the pairs, and the documents. The chip is a `<Link>` with `aria-current` when chosen. The dot is a 6px accent element at top right, with `aria-hidden`, plus a `sr-only` `t("home.chip.unseen")`.

- [ ] **Step 4: Run the tests and see them pass.** Run `cd web && npx vitest run`, then `make lint`. The locale, vocabulary, contrast, and inline-style tests all pass. Expected: PASS.

- [ ] **Step 5: Commit.** `git commit -m "feat(web): add the home page"`, naming every path above.

### Task 7: The form opens on a named profile

**Files:**
- Modify: `web/src/features/reports/generate-page.tsx` (around lines 583 to 613)
- Test: `web/src/features/reports/generate-page.test.tsx`

**Interfaces:**
- `GeneratePage` reads `params.get("profile")`.
- When `from` is null and the id is in `useProfiles()`, `GenerateForm` starts from `defaultsOf(thatProfile, language)`.
- Otherwise it behaves as today. `from` wins.

- [ ] **Step 1: Write the failing tests.**
  - `"opens on the profile in the address"`: the profile select shows the second profile.
  - `"ignores an unknown profile"`: the first profile is chosen and no notice shows.
  - `"prefers from over profile"`.

- [ ] **Step 2: Run the tests and see them fail.** Run `cd web && npx vitest run src/features/reports/generate-page.test.tsx -t profile`. Expected: FAIL.

- [ ] **Step 3: Implement.**

- [ ] **Step 4: Run the tests and see them pass.** Expected: PASS.

- [ ] **Step 5: Commit.** `git commit -m "feat(web): open the form on a named profile"`.

### Task 8: End to end

**Files:**
- Modify: `web/e2e/reports.spec.ts`. After its one collection, open `/` as the operator. The chip of `e2e-prod` (its cluster name) shows the report's mark, and the answer strip shows the same totals as the overview.
- Create: `web/e2e/home.spec.ts`. Prefix `e2e-h-`; no collection.
  - An administrator creates an operator and a reader.
  - The operator sees the chips of the active profiles. `e2e-recovery` is "No report yet" until the recovery spec runs, so the spec must accept a chip of either state and assert only what holds in any order. Choose a profile created by this spec through the admin API (`e2e-h-empty`, pointed at `fake-prometheus:9091`) for the "No report yet" and Request assertions.
  - The reader sees no Request.
  - In `finally`, the spec deactivates `e2e-h-empty`.
  - It runs the CSP and axe checks.
- Modify: `AGENTS.md`. Add `e2e-h-` to the list of spec prefixes, and `home` to the specs in name order.
- Check: the admin spec counts no profiles in a way that an extra `e2e-h-` profile would break. If it does, stop and report.

- [ ] **Step 1: Write both changes.**

- [ ] **Step 2: Main thread runs `make e2e`.** Expected: every spec passes, with the `remaining: 8` check intact.

- [ ] **Step 3: Commit.** `git commit -m "test(web): cover the home page end to end"`.

### Task 9: Changelog, look check, review, gate (main thread)

- [ ] **Step 1: Add the changelog lines.** Under `## [Unreleased]`, `### Added`:
  - "The home page shows every cluster by its latest report, and answers the chosen cluster with its totals, its pressure, its growth, and what to do first."
  - "A report can be requested for a cluster straight from the home page."
- [ ] **Step 2: Look check.** Built Home in light and dark, at 1366×768 and 1024×768, beside `.impeccable/mocks/decision/home/dossier.html`, opened on the companion page for the owner.
- [ ] **Step 3: Run the whole-branch adversarial review** (`adversarial-reviewer` agent) on `git diff main...feat/home-page`. Fix its findings and log them in ROADMAP.
- [ ] **Step 4: Run `make gate`.** Logs go to `.cache/phase-17/`.
- [ ] **Step 5: Version commit 2.8.0 on the owner's word.** It covers `VERSION`, the changelog heading, the chart's `version` and `appVersion`, and every other file held equal to `VERSION`. Then gate and audit, as the release flow sets out.
