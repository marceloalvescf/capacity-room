# Phase 11: Web Minors Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Close the web minors the owner ruled on 2026-10-13: the report list tells its reports apart (cluster, worst severity, language, the language of each PDF) and stops losing state, the new report form says what it dropped and keeps its key and its lock honest, the progress page counts from the start and deletes what the list deletes, the report screens get one title wording, one h1, and an explained confidence, and a tooltip sits inside the landmark of its button.

**Architecture:** One backend thread (F-205): a new migration adds two nullable columns to `reports`, filled by the collect handler in the transaction that stores the snapshot, from a helper of `internal/document` that reuses the view's own filter, and exposed as two nullable fields of `ReportSummary`. Every other change is in `web/`: React components, the API client, the locale files, and the e2e specs. The tasks share the locale files, so they run in sequence.

**Tech Stack:** Go 1.27, PostgreSQL 18 with goose, OpenAPI 3.0.3 with oapi-codegen and openapi-typescript, React 19 with Radix UI, TanStack Query, react-i18next, Vitest with msw, Playwright with axe.

**Spec:** `docs/superpowers/specs/2026-09-27-k8s-capacity-report-design.md` (sections 11.10 and 12), `AGENTS.md`, the owner decisions of 2026-10-13 in `ROADMAP.md` (phase 11 row: F-134, F-135, F-136, F-140, F-143, F-146, F-175, F-177, F-180, F-181, F-205), and the decisions D1 to D14 below, which the owner rules with this plan.

## Decisions for the owner

The plan is written on the recommended option of each decision. No brief is cut until the owner has ruled them; a ruling other than the recommendation amends the task it names before that task's brief is cut. Every visual result is shown on real screenshots at the look of Task 10 before the phase closes.

| ID | Item | Recommendation | Alternative | Task |
|---|---|---|---|---|
| D1 | F-205: what "worst severity" stores | The worst severity among the findings the report shows: its minimum severity applied, raised to attention for the executive profile, exactly the filter of the view. `healthy` when it shows none. So the list never names a severity the report hides. | The worst over every stored finding, whatever the minimum. | 1 |
| D2 | F-205: the two new list columns | "Cluster" (the name, mono) and "Worst severity" / "Pior severidade" (the status mark and its word), placed after Profile; a dash for a report collected before 1.6.0 and for one not collected yet. | One column "Cluster" with the mark beside the name. | 5 |
| D3 | F-134 point 4: the PDF column | Every PDF of the report, one per line, in language order: the language code in mono (`en`, `pt-BR`), then its mark and its status word. A collected report with none reads "Not generated". The spec sentence "the PDF of the interface language" changes with it. | The PDF of the interface language, as today, with its code before the mark. | 5 |
| D4 | F-175: the language of each row | The second line of the Profile cell ends with the report's language name: "payments · Last 24 hours · English". | A column of its own. | 5 |
| D5 | F-134 point 3: the names of Open, Delete, and the checkbox | "Open the report on {{profile}}, requested {{time}}", "Delete the report on {{profile}}, requested {{time}}", "Select the report on {{profile}}, requested {{time}}", with the time to the second (`2026-10-13 22:08:15`); pt-BR "Abrir o relatório de {{profile}}, solicitado em {{time}}", "Excluir o relatório de …", "Selecionar o relatório de …". Each starts with the visible word, so the name holds the label. The visible words stay "Open" and "Delete". | Time to the minute, owner's username added. | 5 |
| D6 | F-134 points 1 and 2: loading and the profile names | While the reports or the profile names load, the page shows "Loading reports…" / "Carregando relatórios…" as a status line under the heading, and no table, so no raw id flashes. When the profile names fail, a notice above the table, "The profile names could not be loaded, so the list shows their ids." / "Não foi possível carregar os nomes dos perfis; a lista mostra os ids.", with the retry button the list already has. | A skeleton table. | 5 |
| D7 | F-134 point 5: the request timeout | Every request of the API client fails when no answer has come 30 s after it was sent. The message is "The server did not answer in time." / "O servidor não respondeu a tempo." A timed-out request counts as unanswered: the new report form keeps its key, so a resend replays. | A timeout on the delete requests alone. | 4 |
| D8 | F-134 point 6: a bulk delete interrupted part way | The reports that went are announced as today, the error shows in the alert, and every report that was not deleted (refused, or in a chunk that failed or was never sent) stays selected. | Stop and show the error alone, as today. | 5 |
| D9 | F-135 point 1: what `?from=` says when it drops a value | A notice above the form for each drop: "The profile of that report is no longer available, so the form starts from {{profile}}." / "O perfil daquele relatório não está mais disponível; o formulário começa pelo perfil {{profile}}."; "Namespaces the profile no longer allows were left out: {{names}}." / "Namespaces que o perfil não permite mais ficaram de fora: {{names}}."; and, the same silent drop of a range on a profile that now restricts its periods, "The profile no longer permits that report's period, so the form starts from {{period}}." / "O perfil não permite mais o período daquele relatório; o formulário começa por {{period}}." The third is beyond the ruled points; strike it if unwanted. | The first two only. | 6 |
| D10 | F-136 point 2: the elapsed time while queued | No elapsed line while the report is queued (the steps already say so); from `started_at` once it runs. | "Waiting for {{time}}" while queued. | 8 |
| D11 | F-177: the title | "Report · {{profile}}" / "Relatório · {{profile}}", on the report and the progress page, as the triage recommended. | "{{profile}} report" / "Relatório do perfil {{profile}}". | 9 |
| D12 | F-177: a collection under one second | "in under 1 s" / "em menos de 1 s" on the data quality tab; every other duration as today (`1:05`). | "in 0:00", as today. | 9 |
| D13 | F-177: Confidence explained | Text, not a tooltip: the triage proposed a tooltip on the header, but a tooltip needs a focusable trigger and the header is not one. A line under the action plan, "Confidence: exact, read from the metrics; inferred, deduced from approximate or partial evidence; unknown, not established by the metrics." / "Confiança: exata, lida das métricas; inferida, deduzida de evidência aproximada ou parcial; desconhecida, não estabelecida pelas métricas."; and the finding's detail line takes its meaning: "Confidence: Inferred. Deduced from approximate or partial evidence." / "Confiança: Inferida. Deduzida de evidência aproximada ou parcial." | An info icon button beside the header, with the text in its tooltip. | 9 |
| D14 | F-181: the flip at 1024 px | Close it as checked. Measured on the real build at 1024×768 (`.cache/phase-11/look/before/`): with 1, 8, or 15 namespaces the picker opens below its trigger; with 30, the trigger sits at y 645 and the picker opens upward over the namespaces, inside the window, never over the title. The flip is Radix's collision rule and keeps the whole picker on screen. | Scroll the trigger to the top of the window on open, so the picker always opens below. | 10 |

F-180 closes on the 1024×768 overview of the same folder: the answer strip, the data-quality notice, the gauge, and the pressure by scope stand above the fold, and the action-plan rows open their finding since 6edf691. It is shown to the owner at the look and closed there.

## Global Constraints

- Branch `feat/web-minors`, opened from `4605fcf`; nothing is edited on `main`.
- The public surface changes only by addition (minor version 1.6.0): `ReportSummary` (and so `Report`) gains `worst_severity` and `cluster_name`, both required and nullable; a new schema `Severity`. The released migrations are never edited; `00005_report_summary.sql` is new. No stored row is rewritten: a report collected before the upgrade keeps both columns null, and the screens show a dash (spec 15.1: nothing is lost, nothing is backfilled).
- Every reader text comes from `web/src/locales/en.json` and `pt-BR.json`, both holding the same keys with the same parameters; every key is used; a family of keys is declared in `web/src/i18n/families.ts`. Technical terms stay English in both languages (namespace, cluster, request, limit); only common nouns are Portuguese.
- The web applies no threshold and derives no status the view or the API does not give: the worst severity is stored by Go and drawn by the list.
- Status is a shape, then a label, then a colour, drawn by `StatusMark`. Names and numbers use the monospaced font. No inline style. Motion only under `motion-safe:`.
- No test skips, no focused test. Each new test is seen failing first; when an implementer writes code first, the report shows a mutation that makes each new test fail.
- A comment says what the code does; it names no task, step, finding, phase, decision, or ruling.
- Subagents never run Docker. `make postgres-up` runs on the main thread before Task 1, which gives the DSN; `make test-postgres`, `make e2e`, and `make gate` run on the main thread.
- No golden page and no golden data file changes in this phase: nothing in it moves the snapshot, the view, or the PDF. A task whose diff changes a file under `template/tests/golden/`, `testdata/`, `template/tests/capacity/fixtures/`, or `web/tests/fixtures/view/` stops and reports.
- Commits name their paths (`git add <paths>`), conventional format, subject under 50 characters.

## Review Focus

1. **A report collected before 1.6.0** has both columns null: `GET /reports` and `GET /reports/{id}` answer 200 with `worst_severity: null` and `cluster_name: null`, and the list draws a dash in both cells. Pinned by Task 1 (API, postgres) and Task 5 (list).
2. **An executive report whose findings are all healthy** stores `healthy`, never the raw worst of a lower severity the report hides; and a detailed report stores the same worst its view shows. Pinned by Task 1, which compares the helper with `BuildView` over every golden snapshot, in both profiles.
3. **An expired report keeps its two values** after the expiry step deletes its snapshot and PDF rows. Pinned by Task 1 (store, postgres).
4. **A tooltip of a button inside a body-portalled popover** (the calendar button of the period picker) still opens, in the body, while every shell and table button's tooltip opens inside its landmark. Pinned by Task 3.
5. **A request that times out while sending a new report** keeps the idempotency key, so the resend replays the same body. Pinned by Task 4.

The bulk delete whose first chunk fails announces nothing gone and keeps every id selected (Task 5); the progress page after an id change announces nothing about the report it left (Task 8).

---

## File Structure

| File | Responsibility | Task |
|---|---|---|
| `internal/platform/store/migrations/00005_report_summary.sql` (new) | the two columns and their checks | 1 |
| `internal/platform/store/reports.go`, `migrate_test.go`, `report_queries_test.go`, `report_read_test.go` | write and read the columns | 1 |
| `internal/document/worst.go`, `worst_test.go` (new) | the worst severity the view shows | 1 |
| `internal/platform/worker/collect.go`, `collect_test.go` | store the two values with the collection | 1 |
| `internal/platform/reports/read.go`, `internal/platform/api/reports.go`, `reports_test.go`, `contract_test.go` | carry them to the API | 1 |
| `api/openapi.yaml`, `internal/platform/api/openapi.gen.go`, `web/src/api/schema.gen.ts`, `web/tests/msw.ts` | the contract and its generated types | 1, 2 |
| `internal/platform/api/view_contract_test.go` | the walker reads a one-entry `allOf` | 2 |
| `web/src/app/app.tsx`, `web/src/components/ui/icon-button.tsx`, `web/tests/render.tsx`, `web/e2e/security.spec.ts` | one tooltip provider; the tooltip in its landmark | 3 |
| `web/src/api/client.ts`, `client.test.ts`, `errors.ts` | the request timeout | 4 |
| `web/src/features/reports/reports-page.tsx`, `api.ts`, `reports-page.test.tsx`, `web/src/lib/format.ts`, `web/e2e/reports.spec.ts` | the list | 5 |
| `web/src/features/reports/generate-page.tsx`, `generate-page.test.tsx`, `web/src/components/ui/select.tsx`, `ui.test.tsx` | the form | 6 |
| `web/src/features/reports/period-picker.tsx`, `period-picker.test.tsx` | the quick-range highlight | 7 |
| `web/src/features/reports/progress-page.tsx`, `progress-page.test.tsx`, `web/src/app/router.tsx` | the progress page | 8 |
| `web/src/features/report/report-page.tsx`, `report-page.test.tsx`, `findings-tab.tsx`, `findings-tab.test.tsx`, `overview-tab.tsx`, `overview-tab.test.tsx`, `quality-tab.tsx`, `quality-tab.test.tsx`, `web/src/app/not-found.tsx` | the report screens | 9 |
| `web/src/locales/en.json`, `pt-BR.json`, `web/src/i18n/families.ts` | every new text | 3 to 9 |
| `docs/superpowers/specs/2026-09-27-k8s-capacity-report-design.md`, `AGENTS.md`, `DESIGN.md`, `CHANGELOG.md` | records | 1, 3, 5, 8, 10 |

Every task runs after the one before it: they share the locale files, the contract, or the test helpers.

---

### Task 1: The worst severity and the cluster, stored at collection (F-205)

**Files:**
- Create: `internal/platform/store/migrations/00005_report_summary.sql`, `internal/document/worst.go`, `internal/document/worst_test.go`
- Modify: `internal/platform/store/reports.go` (`Report` struct, `reportColumns` :57, `scanReportAfter` :81, `CollectReport` :222, `listColumns` :386, `scanItem` :389), `internal/platform/worker/collect.go:263-280`, `internal/platform/reports/read.go:22-35` and `summaryOf` :113, `internal/platform/api/reports.go:114-140` (`reportSummaryOf`, `reportOf`), `api/openapi.yaml` (`ReportSummary` :1214, new `Severity`), `web/tests/msw.ts` (the `report()` factory), the spec sentences of section 11.10 that list the fields of a report summary (find them with `grep -n 'ReportSummary\|owner_username' docs/superpowers/specs/2026-09-27-k8s-capacity-report-design.md`)
- Generate: `internal/platform/api/openapi.gen.go`, `web/src/api/schema.gen.ts` (`make generate`)
- Test: `internal/platform/store/migrate_test.go` (:59, :234, :380-401), `report_queries_test.go`, `report_read_test.go`, `internal/platform/worker/collect_test.go` (`TestCollectStoresTheSnapshot` :441), `internal/platform/api/reports_test.go`, `internal/platform/api/contract_test.go`

**Interfaces:**
- Produces: `document.WorstSeverity(s snapshot.Snapshot) (string, error)`; `store.CollectReport(ctx, q, id, attempt, now, summary store.CollectedSummary) error` with `type CollectedSummary struct { WorstSeverity, ClusterName string }`; `store.Report.WorstSeverity *string`, `store.Report.ClusterName *string`; `reports.Summary.WorstSeverity *string`, `reports.Summary.ClusterName *string`; the contract fields `ReportSummary.worst_severity` (`Severity`, nullable) and `ReportSummary.cluster_name` (string, nullable), so the TypeScript type `Schemas["ReportSummary"]` holds `worst_severity: "healthy" | "attention" | "critical" | null` and `cluster_name: string | null`.

The main thread runs `make postgres-up` before this brief and gives the DSN; the tagged tests run with `KCR_TEST_DATABASE_DSN` set.

- [ ] **Step 1: The helper's test**

`internal/document/worst_test.go` walks every snapshot under `testdata/snapshot/` (the list the other document tests walk; reuse their loader). For each, in both report profiles (set the snapshot's report profile field to `detailed`, then `executive`, the field `resolve` reads), it compares `WorstSeverity(s)` with the worst `Severity` of `BuildView(s, en).Findings` by `analysis.Rank`, `healthy` when that list is empty. Add two literal cases: the worst of `edge-scaling` detailed, and the worst of one scenario whose findings in the executive profile are empty or healthy (find one by running the helper; if no scenario has none, build one by clearing `Findings` of a copy and expect `healthy`).

```go
func TestWorstSeverityIsTheWorstTheViewShows(t *testing.T) {
	for _, name := range goldenSnapshotNames(t) {
		for _, profile := range []string{"detailed", "executive"} {
			s := withProfile(loadSnapshot(t, name), profile)
			view, err := BuildView(s, catalogFor(t, "en"))
			if err != nil {
				t.Fatalf("%s %s: %v", name, profile, err)
			}
			want := analysis.SeverityHealthy
			for _, f := range view.Findings {
				if analysis.Rank(f.Severity) > analysis.Rank(want) {
					want = f.Severity
				}
			}
			got, err := WorstSeverity(s)
			if err != nil || got != want {
				t.Errorf("%s %s: WorstSeverity = %q, %v; the view shows %q", name, profile, got, err, want)
			}
		}
	}
}
```

Use the helper names the package's tests already have; `goldenSnapshotNames`, `withProfile`, `loadSnapshot`, and `catalogFor` stand for them.

- [ ] **Step 2: See it fail, then write the helper**

Run: `go test ./internal/document/ -run WorstSeverity -count=1` → FAIL (undefined).

`internal/document/worst.go`:

```go
package document

import (
	"github.com/marceloalvescf/k8s-capacity-report/internal/analysis"
	"github.com/marceloalvescf/k8s-capacity-report/internal/snapshot"
)

// WorstSeverity is the worst severity among the findings the report shows:
// the snapshot's minimum severity applied, raised to attention for the
// executive profile, as BuildView narrows them. It is healthy when the
// report shows no finding.
func WorstSeverity(s snapshot.Snapshot) (string, error) {
	profile, severity, err := resolve(s, Options{})
	if err != nil {
		return "", err
	}
	if profile == "executive" {
		severity = raiseToAttention(severity)
	}
	worst := analysis.SeverityHealthy
	for _, f := range shownFindings(s, severity) {
		if analysis.Rank(f.Severity) > analysis.Rank(worst) {
			worst = f.Severity
		}
	}
	return worst, nil
}
```

Use the module path of `go.mod`. Run the test → PASS.

- [ ] **Step 3: The migration and the store, tests first**

`00005_report_summary.sql`, in the style of `00004_reports.sql`:

```sql
-- +goose Up
ALTER TABLE reports
    ADD COLUMN worst_severity text,
    ADD COLUMN cluster_name text,
    ADD CONSTRAINT ck_reports_worst_severity
        CHECK (worst_severity IS NULL OR worst_severity IN ('healthy', 'attention', 'critical')),
    ADD CONSTRAINT ck_reports_summary_pair
        CHECK ((worst_severity IS NULL) = (cluster_name IS NULL)),
    ADD CONSTRAINT ck_reports_summary_collected
        CHECK (worst_severity IS NULL OR status = 'collected');

-- +goose Down
ALTER TABLE reports
    DROP CONSTRAINT ck_reports_summary_collected,
    DROP CONSTRAINT ck_reports_summary_pair,
    DROP CONSTRAINT ck_reports_worst_severity,
    DROP COLUMN cluster_name,
    DROP COLUMN worst_severity;
```

Tests to change or add (postgres tag):
- `migrate_test.go`: the three `version != 4` checks become 5; `TestMigrateDownAndUp` starts with a step from 5 to 4 that checks the column `worst_severity` is gone (add a small `columnExists(t, pool, table, column)` helper over `information_schema.columns`), then the existing steps to 3 and 2.
- `report_queries_test.go` (`TestOutcomesOfARunningReport` or a new `TestCollectReportStoresTheSummary`): `CollectReport` with `CollectedSummary{"attention", "prod"}` writes both columns; a collected row read back through `GetReport` and `ListReports` carries them; a direct `UPDATE reports SET worst_severity = 'attention'` on a `running` row fails with `ck_reports_summary_collected`.
- `report_read_test.go`: a collected report whose columns are null (a report collected before the upgrade) lists with both fields nil; after `ExpireDueReports` a collected report keeps both values.

Run `go test -count=1 -tags=postgres ./internal/platform/store/` → FAIL. Then change `store`: the two fields on `Report`, both column lists, both scans, and

```go
// CollectedSummary is what the list shows of a collection: the worst
// severity the report shows and the name of its cluster.
type CollectedSummary struct {
	WorstSeverity string
	ClusterName   string
}

func CollectReport(ctx context.Context, q Querier, id uuid.UUID, attempt int, now time.Time, summary CollectedSummary) error {
	return execGuarded(ctx, q, `
UPDATE reports SET status = 'collected', phase = 'rendering', collected_at = $3, updated_at = $3,
    worst_severity = $4, cluster_name = $5
WHERE id = $1 AND attempt = $2 AND status = 'running'`, id, attempt, now, summary.WorstSeverity, summary.ClusterName)
}
```

Run again → PASS. Fix every other caller of `CollectReport` the compiler names (tests included).

- [ ] **Step 4: The collect handler, test first**

`TestCollectStoresTheSnapshot` (`collect_test.go:441`) also reads the report row and expects the literal worst severity of its scenario and the cluster name of its profile; read the scenario's expected worst from `WorstSeverity` once, then write it as a literal. Run with the tags `postgres template` as `make test-postgres` does for the worker → FAIL.

In `collect.go`, after `snapshot.Encode` and before the transaction:

```go
	worst, err := document.WorstSeverity(result.Snapshot)
	if err != nil {
		return w.fail(...) // the path Encode's error takes, with INTERNAL_FAILURE
	}
	summary := store.CollectedSummary{WorstSeverity: worst, ClusterName: result.Snapshot.Scope.ClusterName}
```

and pass `summary` to `store.CollectReport` at :265. Follow exactly what the handler does when `snapshot.Encode` fails; the comment above the call says what the two values are. Run → PASS.

- [ ] **Step 5: The contract, the service, and the API, tests first**

In `api/openapi.yaml`, `ReportSummary` lists both new names in `required` and gains:

```yaml
        worst_severity:
          allOf:
            - $ref: "#/components/schemas/Severity"
          nullable: true
          description: >-
            The worst severity among the findings the report shows, its minimum
            severity applied; healthy when it shows none. Set when the collection
            is stored; null before, and for a report collected before 1.6.0.
        cluster_name:
          type: string
          nullable: true
          description: >-
            The cluster name of the snapshot. Set when the collection is stored;
            null before, and for a report collected before 1.6.0.
```

and `components.schemas` gains

```yaml
    Severity:
      type: string
      enum: [healthy, attention, critical]
```

Tests:
- `contract_test.go`: `TestSeverityEqualsTheRegistry` reads the `Severity` enum of the file and compares it with `analysis.Severities`, in order.
- `internal/platform/api/reports_test.go` (postgres): a collected report lists with `"worst_severity":"critical"` (or its literal) and `"cluster_name":"…"`; a report whose columns are null answers `null` for both, in the list and in `GET /reports/{id}`; a queued report answers `null` for both.

Run → FAIL. Then `make generate`, add both fields to `reports.Summary` and `summaryOf`, and set them in `reportSummaryOf` and `reportOf` (the generated type is a pointer to `Severity` and a `*string`). Add `worst_severity: null, cluster_name: null` to the `report()` factory of `web/tests/msw.ts` so the web suite still compiles. Amend the spec sentence of section 11.10 that lists a summary's fields, in the spec's own style.

Run: `go test ./... -count=1`, `KCR_TEST_DATABASE_DSN=… go test -count=1 -tags=postgres ./internal/platform/...` (the worker with `-tags='postgres template'`), `make generate-check`, `make lint`, `cd web && npx vitest run` → PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/platform/store/migrations/00005_report_summary.sql internal/platform/store/ internal/document/worst.go internal/document/worst_test.go internal/platform/worker/collect.go internal/platform/worker/collect_test.go internal/platform/reports/read.go internal/platform/api/ api/openapi.yaml web/src/api/schema.gen.ts web/tests/msw.ts docs/superpowers/specs/2026-09-27-k8s-capacity-report-design.md
git commit -m "feat(reports): store the worst severity and cluster"
```

The main thread then runs `make test-postgres` (Docker) and `make postgres-up` again after it.

---

### Task 2: The descriptions lost to `$ref` (F-143)

**Files:**
- Modify: `api/openapi.yaml` (`View.min_severity`, and the `severity` of `ViewNamespace`, `ViewWorkload`, `ViewNode`), `internal/platform/api/view_contract_test.go`
- Generate: `web/src/api/schema.gen.ts` if the generator's output changes

**Interfaces:**
- Consumes: the contract of Task 1.
- Produces: the walker of `TestViewMatchesTheContract` accepts a property written `allOf: [ $ref ]` with a sibling `description`, as `ReportSummary` writes its nullable fields.

OpenAPI 3.0 ignores the siblings of a `$ref`, so commit 1137957 dropped four descriptions when it turned the fields into enums. Restore them in the form the file already uses:

```yaml
        min_severity:
          allOf:
            - $ref: "#/components/schemas/MinSeverity"
          description: The effective minimum, raised to attention for the executive profile.
```

and, for the namespace, the workload, and the node, `allOf: [ $ref: ViewSeverity ]` with "The severity the PDF gives the namespace, healthy when it has no finding." (the workload, the node). The other dropped descriptions ("A code.", "detailed or executive.") say nothing the enum does not, and stay out.

- [ ] **Step 1: Teach the walker, test first.** Add a test in `view_contract_test.go` that the walker resolves a schema written as `allOf` with one `$ref` to that `$ref`, and refuses an `allOf` of two entries with an error naming the property. Run → FAIL. Implement it in the walker's resolve step. Run → PASS.
- [ ] **Step 2: Restore the four descriptions.** Run `go test ./internal/platform/api/ -count=1` (the contract tests), `make generate`, `make generate-check`, `make lint`, `cd web && npx vitest run` → PASS. If `schema.gen.ts` changed, read the diff: only comments and the shape of the four fields may change, never a type.
- [ ] **Step 3: Commit**

```bash
git add api/openapi.yaml internal/platform/api/view_contract_test.go web/src/api/schema.gen.ts
git commit -m "docs(api): restore the view's field descriptions"
```

---

### Task 3: One tooltip provider, each tooltip in its landmark (F-146, F-143)

**Files:**
- Modify: `web/src/app/app.tsx` (around the router, beside `QueryClientProvider`), `web/src/components/ui/icon-button.tsx`, `web/tests/render.tsx` and every test that renders a component holding an `IconButton` without it (`grep -rln 'IconButton\|PeriodPicker\|DataTable\|CopyName\|ThemeToggle\|LanguageMenu' web/src --include=*.test.tsx`), `web/e2e/security.spec.ts:54-58,186,199`, `AGENTS.md` ("An icon button keeps its name"), `DESIGN.md` (the icon button's rule)
- Test: `web/src/components/ui/pieces.test.tsx` or a new `icon-button.test.tsx`

**Interfaces:**
- Produces: `IconButton` keeps its props; the app renders one `Tooltip.Provider`; any test that renders an `IconButton` outside `renderApp` wraps it in `Tooltip.Provider` (export a `withTooltips` wrapper from `web/tests/render.tsx` and use it).

The tooltip's portal goes to the landmark that holds its button, so axe's `region` rule passes with a tooltip open. A button inside a body-portalled layer (the period picker's calendar button, inside its popover) has no landmark above it, and its tooltip stays in the body, as a popover's content does.

- [ ] **Step 1: The tests**

```tsx
it("draws the tooltip of a header button inside the header", async () => {
  await renderApp("/reports");
  const user = userEvent.setup();
  await user.tab(); // repeat until the theme button has focus, as the shell orders its controls
  const tooltip = await screen.findByRole("tooltip");
  expect(tooltip.closest("header")).not.toBeNull();
});

it("draws the tooltip of a button in the content inside main", async () => { /* a copy-name button of a report tab */ });

it("draws the tooltip of the picker's calendar button in the body", async () => {
  // open the period picker, switch to Range, focus a calendar button
  const tooltip = await screen.findByRole("tooltip");
  expect(tooltip.closest("main, header, nav, aside, footer")).toBeNull();
});
```

Write each with the helpers the neighbouring tests use. Run → the first two FAIL (the tooltip is in the body).

- [ ] **Step 2: The provider and the container**

In `app.tsx`, one `<Tooltip.Provider>` wraps the router. In `icon-button.tsx`:

```tsx
// A tooltip is drawn inside the landmark that holds its button, so that it
// belongs to a region of the page; a button outside every landmark (inside a
// layer portalled to the body) keeps the body.
const landmarks = "main, header, nav, aside, footer";

export function IconButton({ label, className, ...rest }: ComponentProps<typeof Button> & { label: string }) {
  const [container, setContainer] = useState<HTMLElement | null>(null);
  const place = useCallback((node: HTMLElement | null) => {
    setContainer(node?.closest<HTMLElement>(landmarks) ?? null);
  }, []);
  return (
    <Tooltip.Root>
      <Tooltip.Trigger asChild ref={place}>
        <Button aria-label={label} className={cn("w-9 px-0", className)} {...rest} />
      </Tooltip.Trigger>
      <Tooltip.Portal container={container}>
        <Tooltip.Content sideOffset={4} className="z-50 rounded-md bg-fg px-2 py-1 text-xs text-bg">
          {label}
        </Tooltip.Content>
      </Tooltip.Portal>
    </Tooltip.Root>
  );
}
```

If Radix's `Portal` treats `null` other than as "the body", pass `container ?? undefined`. Wrap the direct renders of the tests in `withTooltips`. Run `cd web && npx vitest run` and `make lint` → PASS.

- [ ] **Step 3: The e2e runs axe with a tooltip open**

In `security.spec.ts`, delete `closeLayers` and its comment; at both calls, assert the tooltip is open (`await expect(page.getByRole("tooltip")).toBeVisible()`) and run `expectNoViolations` with it open. The main thread runs `make e2e` after the task; the spec must fail on the code before Step 2 (the triage measured that: `region` on the popper wrapper) and pass after.

- [ ] **Step 4: Records.** `AGENTS.md`, "An icon button keeps its name": add "Its tooltip is drawn inside the landmark that holds the button, or in the body when no landmark does, and the app has one tooltip provider." `DESIGN.md`: the same rule in the icon button's entry.

- [ ] **Step 5: Commit**

```bash
git add web/src/app/app.tsx web/src/components/ui/icon-button.tsx web/tests/render.tsx web/e2e/security.spec.ts AGENTS.md DESIGN.md <the test files changed>
git commit -m "fix(web): draw a tooltip inside its landmark"
```

---

### Task 4: A request that gets no answer ends (F-134 point 5)

**Files:**
- Modify: `web/src/api/client.ts:91-99` (`dispatch`), `web/src/api/errors.ts:9-12`, `web/src/i18n/families.ts` (`error` family), `web/src/locales/en.json`, `pt-BR.json` (`error.timeout`)
- Test: `web/src/api/client.test.ts`, `web/src/features/reports/reports-page.test.tsx`, `web/src/features/reports/generate-page.test.tsx`

**Interfaces:**
- Produces: `export const requestTimeout = 30_000` in `client.ts`; a timed-out request rejects with a `DOMException` named `TimeoutError`, which `errorText` words as `error.timeout` and which is not an `ApiFailure` (so every caller treats it as unanswered).

- [ ] **Step 1: The tests**
  - `client.test.ts`: with fake timers, a handler that never answers (`() => new Promise(() => {})`); the call rejects with a `TimeoutError` once 30 000 ms pass, not at 29 999.
  - `reports-page.test.tsx`: a single delete whose `DELETE` never answers; after 30 s the dialog closes, the alert reads `error.timeout`, and the list is asked again.
  - `generate-page.test.tsx` (Review Focus 5): the first `POST /reports` never answers; after 30 s the form shows `error.timeout`; the resend carries the same `Idempotency-Key` and body.

  Run → FAIL.
- [ ] **Step 2: The timeout**

```ts
// A request with no answer this long after it was sent fails, so that no
// screen waits forever on a server that hangs. The timer is a plain timeout,
// so tests drive it with fake timers.
export const requestTimeout = 30_000;

async function dispatch(request: Request): Promise<Response> {
  // the CSRF header, as today
  const timer = new AbortController();
  const id = setTimeout(() => {
    timer.abort(new DOMException("no answer in time", "TimeoutError"));
  }, requestTimeout);
  try {
    return await globalThis.fetch(request, { signal: AbortSignal.any([request.signal, timer.signal]) });
  } finally {
    clearTimeout(id);
  }
}
```

The timeout covers the wait for the answer's head; the body that follows is read without it. In `errors.ts`, before the network case: a thrown value whose `name` is `TimeoutError` gives `t("error.timeout")`. Add `"timeout"` to the `error` family and the two locale entries of D7. Run → PASS; `make lint`.
- [ ] **Step 3: Commit**

```bash
git add web/src/api/client.ts web/src/api/client.test.ts web/src/api/errors.ts web/src/i18n/families.ts web/src/locales/en.json web/src/locales/pt-BR.json web/src/features/reports/reports-page.test.tsx web/src/features/reports/generate-page.test.tsx
git commit -m "fix(web): end a request that gets no answer"
```

---

### Task 5: The report list tells its reports apart (F-134, F-175, F-205)

**Files:**
- Modify: `web/src/features/reports/reports-page.tsx`, `web/src/features/reports/api.ts:157-179` (`useDeleteReports`), `web/src/lib/format.ts` (`formatTimeSeconds`), the locale files, `web/e2e/reports.spec.ts`, spec 12 ("Report list: … Columns: requested, profile, status, the PDF of the interface language, kept until.")
- Test: `web/src/features/reports/reports-page.test.tsx`, `web/src/lib/format.test.ts` (or the file that tests `formatTime`)

**Interfaces:**
- Consumes: `ReportSummary.worst_severity`, `ReportSummary.cluster_name` (Task 1); `error.timeout` (Task 4).
- Produces: `useDeleteReports().mutateAsync(ids)` resolves `{ results: ReportDeleteResult[]; failure: unknown }`, `failure` null when every chunk answered; `formatTimeSeconds(iso: string): string` writes `YYYY-MM-DD HH:MM:SS` in the browser's zone, as `formatTime` writes the minute.

- [ ] **Step 1: The tests** (each in the style of the file's tests, msw and `renderApp`)
  - loading: while `GET /profiles` is held, the page shows `reports.loading` and no table; no raw profile id is ever in the document.
  - profile names failed: `GET /profiles` answers 500; the notice `reports.profilesFailed` shows above the table, the cell shows the id in mono, and the retry asks `/profiles` again.
  - names: two reports of one profile created in the same minute, seconds apart; the two checkboxes, the two Open links, and the two Delete buttons have six different accessible names, each starting with its visible word, holding the profile and the time to the second.
  - PDF column: a report with `en` ready and `pt-BR` pending shows two lines, `en` then `pt-BR`, each with its mark and word; the interface language does not change which shows. Replace the test "the interface language's PDF" accordingly.
  - language: the Profile cell's second line ends with the report's language name (`language.en`), in both interface languages.
  - cluster and worst severity: a row with `cluster_name: "prod"` and `worst_severity: "critical"` shows `prod` in mono and the critical mark with its word; a row with both null shows a dash in each (Review Focus 1).
  - partial bulk delete: 250 selected; the first chunk answers 200 ids deleted, the second answers 500. The announcement counts 200, the alert shows the error, and the 50 unsent stay selected. A bulk whose first chunk fails announces nothing gone and keeps all 250 selected.

  Run → FAIL.
- [ ] **Step 2: The bulk delete keeps what it learned**

```ts
    mutationFn: async (ids: string[]) => {
      const results: Schemas["ReportDeleteResult"][] = [];
      for (let start = 0; start < ids.length; start += deleteChunk) {
        try {
          const { data } = await api.POST("/reports/delete", { body: { ids: ids.slice(start, start + deleteChunk) } });
          if (data === undefined) {
            throw new Error("the delete answer is empty");
          }
          results.push(...data.results);
        } catch (failure) {
          // The chunks already answered stand; the rest were not deleted.
          return { results, failure };
        }
      }
      return { results, failure: null as unknown };
    },
    onSettled: (outcome) => {
      settle(queryClient, (outcome?.results ?? []).filter((result) => result.outcome !== "forbidden").map((result) => result.id));
    },
```

In `confirm()` of the page, the bulk branch keeps selected the refused ids and every id with no result, and shows `errorText(t, toError(failure))` when `failure` is not null. It announces the count that went as today, except when a chunk failed and nothing went: then the alert alone speaks.
- [ ] **Step 3: Loading, names, and the cells.** While `reports.data` or `profiles.data` is undefined and neither has failed, render `<p role="status" className="text-sm text-muted-fg">{t("reports.loading")}</p>` in place of the table. When `profiles.isError`, render `<Alert tone="notice">` with `reports.profilesFailed` and the existing retry button (which refetches both). Give Open, Delete, and the checkbox the labels of D5 (`reports.openOne`, `reports.deleteOne`, `reports.select` with `{ profile, time }`, `time` from `formatTimeSeconds`). Add the columns of D2 after Profile (`reports.column.cluster`, `reports.column.worst`), the PDF lines of D3, and the language of D4 (`language.<code>`). Add every key in both locale files with the wording of the decisions.
- [ ] **Step 4: The e2e.** In `reports.spec.ts`, once the spec's report is collected, its row shows the cluster name of the profile the spec uses (read it from `deploy/platform/profiles.toml`) and the worst severity of its scenario as a literal word (read it from `web/tests/fixtures/view/direct-complete.en.json`, the worst `severity` of its `findings`).
- [ ] **Step 5: The spec.** Spec 12, report list: "Columns: requested, profile (with the namespaces, the period, and the language of the report), cluster, worst severity, status, every PDF with its language, kept until. A report collected before 1.6.0 shows a dash for its cluster and its worst severity."
- [ ] **Step 6: Run** `cd web && npx vitest run`, `make lint` → PASS. Commit:

```bash
git add web/src/features/reports/reports-page.tsx web/src/features/reports/api.ts web/src/features/reports/reports-page.test.tsx web/src/lib/format.ts <its test> web/src/locales/en.json web/src/locales/pt-BR.json web/e2e/reports.spec.ts docs/superpowers/specs/2026-09-27-k8s-capacity-report-design.md
git commit -m "feat(web): tell the reports of the list apart"
```

---

### Task 6: The form says what it dropped and keeps its key (F-135, F-140)

**Files:**
- Modify: `web/src/features/reports/generate-page.tsx` (`fromReport` :60-82, `FromReport` :490-507, the vanished-profile effect :152-159 and render path :168-172, the send lock :134, :241-294, :482, the `mark` path :209-237), `web/src/components/ui/select.tsx:14-32`, the locale files
- Test: `web/src/features/reports/generate-page.test.tsx`, `web/src/components/ui/ui.test.tsx`

**Interfaces:**
- Produces: `fromReport(source, profiles, fallback)` returns `{ values: FormValues; notices: FromNotice[] }` with `type FromNotice = { kind: "profileGone"; profile: string } | { kind: "namespacesDropped"; names: string[] } | { kind: "periodDropped"; period: PeriodValue }`.

- [ ] **Step 1: The tests**
  - `?from=` a report whose profile is gone: the form starts on the first profile and shows the notice of D9 naming it. A report with two namespaces of which one is no longer allowed: the notice names the dropped one, mono. A range report on a profile that now restricts periods: the period notice (D9, third line).
  - `REPORT_SELECTION_NOT_PERMITTED`: after the answer, `GET /profiles` is asked again and the form shows the profile's new allowed namespaces.
  - the send lock: render the form in a memory router whose `/jobs/:id` route has a loader that never settles, so the form stays after a success; the submit button stays disabled with `generate.submitting`, and a second click sends nothing.
  - F-140: `PROFILE_INACTIVE` answers the first send; the profiles reload without the chosen profile; the form resets to the first profile; a send at once, with no edit, carries a new key, and exactly two `POST /reports` reach the server (no `IDEMPOTENCY_KEY_REUSED` round).
  - `Select` in `ui.test.tsx`: a caller's `aria-describedby="x"` joins the hint and error ids (`"x hint-id error-id"`), and the computed `aria-invalid` is not overridden.

  Run → FAIL (the F-140 test fails on the key; the lock test on `disabled`).
- [ ] **Step 2: The vanished profile renews its key in an effect that runs**

```tsx
  const profile = profiles.find((candidate) => candidate.id === values.profileId);
  // A profile that left the list gives way to the first one; the request is a
  // new one, so it takes a new key.
  useEffect(() => {
    if (profile === undefined) {
      renewKey();
      setValues(defaultsOf(profiles[0] as ProfileSummary, values.language));
    }
  }, [profile, profiles, values.language]);
  if (profile === undefined) {
    return null;
  }
```

The render that finds no profile now commits `null`, so the effect sees it. Remove the old `vanished` effect and the `setValues` during render. If ESLint's hook rules ask for `renewKey` among the dependencies, make it stable with `useCallback` (it touches refs only) rather than silencing the rule; the empty-profiles case is the "no profile" screen and never reaches this effect, which a test already pins.
- [ ] **Step 3: The lock the button shows.** Keep the ref for the double click; add `const [locked, setLocked] = useState(false)` set with it and cleared with it on a failure; the button is `disabled={locked}` and reads `generate.submitting` while locked. Remove the success comment that described the old lock and write what the code does.
- [ ] **Step 4: The notices and the refetch.** `FromReport` renders each notice in an `Alert tone="notice"` above the form, names in mono. In the failure path, a `REPORT_SELECTION_NOT_PERMITTED` calls `reload()` (as `PROFILE_INACTIVE` does) and still marks its fields.
- [ ] **Step 5: `Select`.** Spread `rest` before the computed attributes, and join a caller's `aria-describedby` with the computed ids.
- [ ] **Step 6: Run** `cd web && npx vitest run`, `make lint` → PASS. Commit:

```bash
git add web/src/features/reports/generate-page.tsx web/src/features/reports/generate-page.test.tsx web/src/components/ui/select.tsx web/src/components/ui/ui.test.tsx web/src/locales/en.json web/src/locales/pt-BR.json
git commit -m "fix(web): say what a copied report dropped"
```

---

### Task 7: A quick range shows chosen only while it is (F-181)

**Files:**
- Modify: `web/src/features/reports/period-picker.tsx` (`aria-pressed` :408 and :424, `openChanged` :313-320, `switchMode` :322-325, the draft handlers)
- Test: `web/src/features/reports/period-picker.test.tsx`

**Interfaces:**
- Produces: a quick range has `aria-pressed="true"` only when it is the applied value, the popover's mode is the mode that value opens on (a length opens on Length; a calendar name and a range on Range), and the draft has not been edited since the popover opened.

- [ ] **Step 1: The tests.** Value `24h`: open; "Last 24 hours" is pressed; switch to Range: no quick range is pressed; switch back: it is pressed again. Value `yesterday`: open (on Range); "Yesterday" is pressed; type in From: none is pressed. Run → FAIL on the first switch.
- [ ] **Step 2: The rule.** Add `const [edited, setEdited] = useState(false)`, cleared when the popover opens and set by every change of a draft field; `pressed = matches && mode === modeOf(value) && !edited`, with `modeOf` the function `openChanged` already uses to choose the mode on open (extract it if it is inline). Run → PASS; `make lint`.
- [ ] **Step 3: Commit**

```bash
git add web/src/features/reports/period-picker.tsx web/src/features/reports/period-picker.test.tsx
git commit -m "fix(web): unmark the quick range in Range mode"
```

---

### Task 8: The progress page (F-136)

**Files:**
- Modify: `web/src/features/reports/progress-page.tsx` (the Delete condition :220, `<Elapsed since={data.created_at} />` :187, the announcement effect :80-95, `refetchInterval` :82-87), `web/src/app/router.tsx` (the `jobs/:id` route), spec 12 ("While the report runs, the steps of its phase and the elapsed time."), `AGENTS.md` ("The report screens check only while something moves")
- Test: `web/src/features/reports/progress-page.test.tsx`

**Interfaces:**
- Produces: `export function ProgressRoute()` in `progress-page.tsx`, the route's element, which renders `<ProgressPage key={id} />`.

- [ ] **Step 1: The tests**
  - Delete: an operator's own queued report, and a running one, offer Delete; a reader's does not (the list's rule, `canDelete` alone).
  - elapsed: a queued report (`started_at` null) shows no elapsed line; a running report whose `started_at` is 65 s before now, and `created_at` 10 minutes before, reads "Elapsed 1:05".
  - id change: on `/jobs/a` (running), follow a link to `/jobs/b` (collected); the live region announces nothing for `b`.
  - backoff: a report that ran through 45 checks (so the cadence reached 6 s), then collected; Generate PDF; the PDF stays pending; the next check comes 1.5 s after the answer that showed it pending, and the cadence counts again from there. The existing test "checks 19 times after 1.5 s, 20 after 3 s, then every 6 s" keeps its numbers.

  Run → FAIL.
- [ ] **Step 2: The changes**

```tsx
  // The checks count from the answer that showed the report active again, so
  // a PDF requested after a long run is followed as closely as a new report.
  const activeSince = useRef<number | null>(null);
  ...
    refetchInterval: (query) => {
      const current = query.state.data;
      if (ended(query.state.error) || current === undefined || !isActive(current)) {
        activeSince.current = null;
        return false;
      }
      activeSince.current ??= query.state.dataUpdateCount - 1;
      return nextCheck(query.state.dataUpdateCount - activeSince.current);
    },
```

The Delete condition becomes `canDelete(principal, data)`. `<Elapsed since={data.started_at} />` renders only when `started_at` is not null. The route renders `ProgressRoute`, so a new id mounts a new page and every ref starts over.
- [ ] **Step 3: Records.** Spec 12, progress: "While the report runs, the steps of its phase and the time elapsed since the collection started; while it is queued, the steps alone." AGENTS.md, the progress page's checks: add "and count again from the answer that shows the report active again, as after Generate PDF."
- [ ] **Step 4: Run** `cd web && npx vitest run`, `make lint` → PASS. Commit:

```bash
git add web/src/features/reports/progress-page.tsx web/src/features/reports/progress-page.test.tsx web/src/app/router.tsx docs/superpowers/specs/2026-09-27-k8s-capacity-report-design.md AGENTS.md
git commit -m "fix(web): follow the progress of the report shown"
```

---

### Task 9: The report screens (F-177, F-143)

**Files:**
- Modify: `web/src/features/report/report-page.tsx:73-78`, `web/src/features/reports/progress-page.tsx:169` (the title key `progress.title`), `web/src/features/report/quality-tab.tsx:71,104-108`, `web/src/features/report/overview-tab.tsx` (the action plan, around :293), `web/src/features/report/findings-tab.tsx:83` and :143-144, `web/src/app/not-found.tsx`, `web/src/app/router.tsx` (the `*` child of `reports/:id`), `web/src/i18n/families.ts` (`confidenceMeaning`), the locale files
- Test: `report-page.test.tsx` (the three 50 ms waits :198-200, :217-219, :249-251, and "unknown tab" :72-76), `quality-tab.test.tsx`, `overview-tab.test.tsx`, `findings-tab.test.tsx`, `progress-page.test.tsx`

**Interfaces:**
- Produces: `NotFound({ level }: { level?: 1 | 2 })`, `1` by default; the family `confidenceMeaning` keyed by `viewConfidenceValues`.

- [ ] **Step 1: The tests**
  - title (D11): the report page's and the progress page's h1 read `Report · Production` / `Relatório · Production`.
  - under 1 s (D12): equal start and end read "in under 1 s"; 65 s read "in 1:05".
  - confidence (D13): the action plan has the confidence line under its table; a finding's detail reads "Confidence: Inferred. Deduced from approximate or partial evidence."
  - one h1: `/reports/:id/nope` has exactly one h1 (the report's title) and an h2 `notFound.title`.
  - deep link: with `Element.prototype.scrollIntoView` mocked, `/reports/:id/findings?finding=5` scrolls the list entry of finding 5 into view once; a later click on another finding scrolls nothing.
  - the three 50 ms waits become waits on a condition: each test holds the answer it waits on with a deferred promise, waits until msw has the request (`await waitFor(() => expect(calls).toContain(…))`), asserts, then releases it. Each rewritten test still fails when the behaviour it protects is broken: show one mutation per test in the report.

  Run → FAIL.
- [ ] **Step 2: The changes.** The title key holds the wording of D11 in both files. In `quality-tab.tsx`, `elapsed < 1000 ? t("report.quality.underOneSecond") : formatElapsed(elapsed)`. The action plan gets `<p className="text-sm text-muted-fg">{t("report.plan.confidenceNote")}</p>` under its table; the finding's detail line becomes `t("report.confidence", { value })` followed by `t(\`confidenceMeaning.${finding.confidence}\`)`, joined as the locale entries of D13 show (`report.confidence` gains the period: "Confidence: {{value}}." with the meaning after it). `NotFound` takes `level` and renders `h2` when it is 2; the report's `*` child passes `level={2}`. In `findings-tab.tsx`, a layout effect that runs when the tab mounts with a `finding` in the address scrolls that entry into view (`block: "nearest"`, smooth only when `prefers-reduced-motion` is `no-preference`, as `revealRows` decides), and runs once.
- [ ] **Step 3: Run** `cd web && npx vitest run`, `make lint` → PASS. Commit:

```bash
git add web/src/features/report/ web/src/features/reports/progress-page.tsx web/src/features/reports/progress-page.test.tsx web/src/app/not-found.tsx web/src/app/router.tsx web/src/i18n/families.ts web/src/locales/en.json web/src/locales/pt-BR.json
git commit -m "fix(web): word the report title and confidence"
```

---

### Task 10: The look and the records (F-180, F-181, the close)

Main thread.

- [ ] **Step 1: The suite and Docker.** `make test`, `make lint`, `make generate-check`, `make test-postgres`, `make e2e` on the branch.
- [ ] **Step 2: The look.** On the look stack (`.cache/phase-10/look/stack/up.sh`, the binary rebuilt), shoot at 1366×768 and 1024×768, light and dark, en and pt-BR: the list with a collected report, a report whose two columns are set to null in the database (standing for one collected before 1.6.0), a queued report, and two PDFs; a form opened with `?from=` on a gone profile and on dropped namespaces; the picker in Range mode; the progress page queued and running; the report title; the findings tab with a deep-linked finding; the action plan's confidence line; the data quality line; a header tooltip open. Add the before-shots of `.cache/phase-11/look/before/` for F-180 and F-181 (D14). Publish them on a look page beside the images, and the owner rules there.
- [ ] **Step 3: Records.** `CHANGELOG.md` under `[Unreleased]`, one sentence per perceived change (Added: the cluster and the worst severity in the list, the language of each report and of each PDF, the notices of a copied report, confidence explained; Changed: the title, "under 1 s", the elapsed time from the start, Delete on the progress page, the quick-range highlight; Fixed: a request with no answer ends after 30 s, a bulk delete interrupted part way keeps what is left selected, the progress checks after a PDF request, a deep-linked finding scrolls into view, the profile ids no longer flash, the names of the row controls, the tooltips inside the page's regions). Name the omissions (tests, the descriptions of the contract). `ROADMAP.md`: every item with its commit; F-180 and F-181 closed on the look.
- [ ] **Step 4: The close** follows AGENTS.md, "Before merging": `make gate`, the whole-branch adversarial review with one fix wave, ROADMAP and CHANGELOG read back, merge by fast-forward, and version 1.6.0 on the owner's word.
