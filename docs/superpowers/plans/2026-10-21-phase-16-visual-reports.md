# Phase 16: visual reports Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Charts of the findings on the overview, the Findings tab, and the PDF's opening page; the PDF in the web's status colours; a signed-in reader following the browser's language; a range drawn as a range; the node names of a pool behind a popover.

**Architecture:** Go decides every count and order once (`decideVerdict`), and both products read it: the web view (`ViewVerdict.scopes`) and the PDF data file (the verdict's pressure rows). The web draws SVG with the status classes; the template draws in Typst with one palette file. The language becomes a pinned-or-browser choice on the account, carried by an additive API field.

**Tech Stack:** Go 1.27, PostgreSQL 18 (goose), Typst 0.15.1, React + TypeScript + Vite + Tailwind + Vitest, Radix, TanStack Table, Helm.

**Spec:** `docs/superpowers/specs/2026-10-20-visual-reports-design.md` (4b2b1b9, D7 in 5089f20). Approved mocks: `.cache/phase-16/look/dash.html` (overview `opt=B`, findings `opt=D`), `pdf6.html` (`mode=A`), `spread.html` (`opt=B`), `pools.html` (`opt=B`).

## Global Constraints

- Branch `feat/visual-reports`. Nothing is edited on `main`. Tasks run in sequence: they share `api/openapi.yaml`, the generated files, and the locale files.
- Counts and orders are decided in `internal/document`; the web and the template draw what they are given and count nothing.
- Every count of the chart is a count of findings, never of rows.
- Status colours, light theme, the PDF's only colours: critical `#be1e45`, attention `#8f5b00`, healthy `#24774a`, neutral `#5c5c5e`, critical surface `#fdedf0`, attention surface `#fdf4e3`. Words and numbers stay in ink, on the web and in the PDF.
- The release is 2.7.0, a minor version: the API only gains fields; nothing is removed. `ViewVerdict.pressure` stays in the contract, unread by the app.
- AGENTS.md rules hold: no inline style, no `dangerouslySetInnerHTML`; every reader text from the locale files (web) or the catalog (PDF), both languages, same parameters; technical terms in English in pt-BR (pt-BR "findings" is "achados", "nodes" stays "nodes"); no em-dash inside a catalog entry; no comment that speaks of the process; a test never writes a golden file.
- A subagent runs `go`, `npm` in `web/`, `make web`, `make build`, `make test`, `make lint`, `make generate`, `make generate-check`, and `make update-testdata`; never Docker. The main thread runs `make update-golden`, `make test-golden`, `make test-template`, `make test-postgres`, `make test-chart`, `make e2e`, `make gate`. Tagged tests a subagent may run natively: `go test -count=1 -tags=template ./template/ ./internal/typst/ ./internal/cli/` and `go test -count=1 -tags='template golden' ./template/ -run 'Golden|Snapshots'`; postgres tests with `CAPACITYROOM_TEST_DATABASE_DSN` the main thread provides.
- Every commit names its paths (`git add <paths>`); subjects `type(scope): description` under 50 characters.
- After `make update-testdata` or `make update-golden`, every changed file or page is read whole before the commit; a change that cannot be explained is a defect.

## Review Focus

1. A report whose view holds no finding at all (executive profile, every finding healthy): the overview, the Findings tab, and the PDF show "No finding" and the cluster card, not an empty donut or a division by zero. Pinned in Tasks 1, 3, and 4.
2. A spread finding whose lowest peak is 0%: still drawn as a range with both dots, never a fill from zero. Pinned in Task 5.
3. A pool with one node, and the pool named `unknown`: the popover opens, its heading reads, and the button's name has the count. Pinned in Task 6.
4. A browser whose first language is neither `pt` nor `en` (for example `fr-FR`), for an account that follows the browser: English, as before sign-in. Pinned in Task 7.
5. A body with `language_follows_browser: false`, or with both fields, on `PATCH /account` and `PATCH /admin/users/{id}`: 400 `REQUEST_INVALID` naming the field, nothing written, no audit row. Pinned in Task 7.

---

### Task 1: Scope counts in the verdict and the view

**Files:**
- Modify: `internal/document/verdict_facts.go` (`verdictFacts`, `decideVerdict`)
- Modify: `internal/document/view.go:736-785` (`ViewVerdict`, `viewBuilder.verdict`)
- Modify: `api/openapi.yaml` (`ViewVerdict` at :2256, new `ViewScope`)
- Test: `internal/document/verdict_facts_test.go` (create if absent; else the existing test file of `decideVerdict`)
- Regenerate: `web/src/api/schema.gen.ts`, `testdata/snapshot/`, `web/tests/fixtures/view/`, `template/tests/capacity/fixtures/command/` through `make generate` and `make update-testdata`

**Interfaces:**
- Produces: `type scopeFacts struct { ScopeType, Severity string; Lead, Critical, Attention, Healthy int }`; `verdictFacts.Scopes []scopeFacts`; `func decideScopes(findings []snapshot.Finding) []scopeFacts`; view JSON `verdict.scopes[]` with keys `scope_type`, `severity`, `finding`, `critical`, `attention`, `healthy`; Go `type ViewScope struct` with those json tags; TypeScript `components["schemas"]["ViewScope"]`.

- [ ] **Step 1: Write the failing test `TestDecideScopesCountsFindings`.** Build a findings slice in this order: node critical ×2, node attention ×1, node healthy ×1, workload critical ×1, workload attention ×4, workload healthy ×3, hpa attention ×5, namespace healthy ×2, cluster critical ×1 (distinct scope names per row, so rows and findings differ). Expect exactly, in order:

```go
want := []scopeFacts{
	{ScopeType: "node", Severity: "critical", Lead: 0, Critical: 2, Attention: 1, Healthy: 1},
	{ScopeType: "workload", Severity: "critical", Lead: 4, Critical: 1, Attention: 4, Healthy: 3},
	{ScopeType: "hpa", Severity: "attention", Lead: 12, Critical: 0, Attention: 5, Healthy: 0},
	{ScopeType: "namespace", Severity: "healthy", Lead: 17, Critical: 0, Attention: 0, Healthy: 2},
}
```

Add `TestDecideScopesOrdersByCounts` (two attention scopes, hpa with 3 attention and node with 5 attention: node first; two scopes with equal counts keep the order of `catalog.VerdictPressureScopes`) and `TestDecideScopesEmpty` (only cluster findings, and no findings: `len == 0`, never nil in the view, see Step 5).

- [ ] **Step 2: Run** `go test ./internal/document/ -run DecideScopes -v`. Expected: FAIL, `decideScopes` undefined.

- [ ] **Step 3: Implement `decideScopes`** in `verdict_facts.go`: one entry per scope type of `verdictScopeTypes` (cluster excluded) with at least one finding; counts are findings by severity; `Severity` the worst; `Lead` the index of the first finding of the worst severity; order by severity rank desc, then `Critical` desc, then `Attention` desc, then the position in `verdictScopeTypes` (stable sort). `decideVerdict` sets `Scopes` from the same `findings` it passes to `decidePressure`, so the minimum severity applies.

- [ ] **Step 4: Run** the three tests. Expected: PASS.

- [ ] **Step 5: Add the view field.** `ViewVerdict.Scopes []ViewScope \`json:"scopes"\``, mapped in `viewBuilder.verdict`, an empty slice (encoded `[]`) when there is none. In `api/openapi.yaml` add `ViewScope` (description: "One scope type other than the cluster with a finding in this view: its worst severity, its first finding of that severity, and its findings by severity."; required `[scope_type, severity, finding, critical, attention, healthy]`; `scope_type` `$ref ViewScopeType`, `severity` `$ref ViewSeverity`, the four others `integer`, `finding` described "Index in findings of the first finding of the severity.") and add `scopes` (array of `ViewScope`, "The scope types other than the cluster with a finding, the worst first.") to `ViewVerdict`'s properties and required list. `ViewScope` joins the View* schemas excluded from the Go generation, as its siblings are (`api/oapi-codegen.yaml`).

- [ ] **Step 6: Run** `make generate`, `go test ./internal/platform/api/ -run TestViewMatchesTheContract`, then `make update-testdata`. Expected: the contract test PASS; the changed golden views each gain `verdict.scopes` and nothing else. Read every changed file whole.

- [ ] **Step 7: Run** `make test`. Expected: PASS.

- [ ] **Step 8: Commit**

```bash
git add internal/document/verdict_facts.go internal/document/verdict_facts_test.go internal/document/view.go api/openapi.yaml api/oapi-codegen.yaml web/src/api/schema.gen.ts internal/platform/api/openapi.gen.go testdata/snapshot web/tests/fixtures/view template/tests/capacity/fixtures/command
git commit -m "feat(document): count the findings of each scope"
```

### Task 2: The PDF data file's chart rows

**Files:**
- Modify: `internal/document/document.go:215-290, 334, 396-410` (`Verdict`, `VerdictCount`, `VerdictPressure`, `VerdictPressureRow`, their `MarshalJSON`)
- Modify: `internal/document/verdict.go:27-205` (`buildVerdict`, `verdictCount`, `verdictPressure`, `pressureRow`)
- Modify: `internal/catalog/keys.go`, `internal/catalog/locales/en.toml`, `internal/catalog/locales/pt-BR.toml` (`[verdict.pressure]` and a new `[verdict.findings]`)
- Modify: `template/capacity/verdict.typ:161-189` (`build-pressure`, reading only the kept keys), `template/capacity/validate.typ:344-456`, and the fixtures of `template/tests/capacity/negative/` whose verdict keys change; `template/tests/capacity/fixtures/verdict*.json` hand-written fixtures
- Test: `internal/document/verdict_test.go` (the existing tests of `buildVerdict`)

**Interfaces:**
- Consumes: `verdictFacts.Scopes` (Task 1).
- Produces, the data file's verdict block:
  - each entry of `totals` gains `"value": <int>` beside `status`, `label`, `count`;
  - `pressure.rows[]` becomes `{ "status", "label", "sentence", "counts": [VerdictCount...], "bar": { "critical": int, "attention": int, "healthy": int, "scale": int } }`. `scope`, `name`, and `peak` leave the row;
  - `pressure.rows` is built from `Scopes`, in their order; `counts` holds one entry per severity the report shows (critical, attention, and healthy for the detailed profile; critical and attention for the executive one), zeros included; `bar.scale` is the largest row total;
  - a new `findings` block beside `pressure`: `{ "title": "Findings", "total_label": "findings" }` (pt-BR `"Achados"`, `"achados"`), keys `catalog.VerdictFindingsTitle`, `catalog.VerdictFindingsTotal`;
  - `pressure.empty` keeps its meaning: present when `rows` is empty.
- Go names: `type VerdictBar struct { Critical, Attention, Healthy, Scale int }`; `VerdictCount.Value int`; `Verdict.Findings VerdictFindings{Title, TotalLabel string}`.

- [ ] **Step 1: Write the failing tests** in `verdict_test.go`: `TestVerdictPressureRowsFollowTheScopes` (from the Task 1 findings set, detailed profile, en: rows labels `"Nodes"`, `"Workloads"`, `"HPAs"`, `"Namespaces"`; first row `bar == VerdictBar{Critical: 2, Attention: 1, Healthy: 1, Scale: 8}`; counts values `[2, 1, 1]`), `TestVerdictCountsOfTheExecutiveProfile` (executive: every row's counts has two entries, critical and attention), `TestVerdictTotalsCarryValues` (`totals[0].Value == 4`: the two critical findings of the nodes, the one of the workloads, and the one of the cluster), `TestVerdictFindingsWords` (en `"Findings"`/`"findings"`, pt-BR `"Achados"`/`"achados"`).

- [ ] **Step 2: Run** `go test ./internal/document/ -run Verdict -v`. Expected: FAIL to compile.

- [ ] **Step 3: Implement** the structs, their `MarshalJSON` (keys exactly as above), the builders, and the two catalog keys in `keys.go` (constants plus the registry) and both TOML files. Drop `VerdictPressurePeak` and any key the rows no longer use, so `TestEveryKeyIsUsed` stays true.

- [ ] **Step 4: Update `validate.typ`**: `check-exact-keys` for totals entries `("status", "label", "count", "value")`, rows `("status", "label", "sentence", "counts", "bar")`, bar `("critical", "attention", "healthy", "scale")` each `check-int` non-negative with `scale` positive when there are rows, and the `findings` block `("title", "total_label")`. Reuse the existing diagnostics (`missing-key`, `unknown-key`, `wrong-type`); the count stays 23. Update the negative fixtures that name a removed key and the hand-written positive fixtures.

- [ ] **Step 5: Run** the tests of Step 1, then `make update-testdata`, then `go test -count=1 -tags=template ./template/ ./internal/typst/ ./internal/cli/`. Expected: PASS. Before running them, `build-pressure` in `template/capacity/verdict.typ` stops reading `row.scope`, `row.name`, and `row.peak` (it sets the label, the sentence, and the count line only), so the template compiles until Task 3 draws the bars.

- [ ] **Step 6: Run** `make test`. Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/document internal/catalog template/capacity template/tests/capacity testdata web/tests/fixtures/view
git commit -m "feat(document): give the PDF the chart counts"
```

### Task 3: The PDF in colour, with the donut and the bars

**Files:**
- Create: `template/capacity/palette.typ` (the only file with colour literals)
- Modify: `template/capacity/runs.typ:7-31` (`status-fill`), `template/capacity/verdict.typ:8-203` (`gauge`, `build-pressure`, `build-verdict`, a new `donut`), `template/capacity/tables.typ` (peak cell tint), `template/capacity/blocks.typ`, `cover.typ`, `main.typ` only where they hold a grey that must move to the palette
- Modify: `template/golden_test.go:337-354` (remove `TestGoldenPagesHaveNoColour`)
- Create: `template/palette_test.go` (the palette and the colour-literal guard)
- Modify: `DESIGN.md` (section The PDF), `PRODUCT.md:73-74`, `AGENTS.md` (The template: "No page may contain a coloured pixel")

**Interfaces:**
- Consumes: the data file of Task 2.
- Produces: `palette.typ` exports `critical`, `attention`, `healthy`, `neutral`, `critical-surface`, `attention-surface` as `rgb("#...")` with the Global Constraints values, and `status-colour(status)` mapping `critical`, `attention`, `healthy` to their colour and `undefined`, `unavailable` to `neutral`.

- [ ] **Step 1: Write the failing tests** in `template/palette_test.go`: `TestPaletteEqualsTheWebTheme` reads `palette.typ` and the `:root {` block of `web/src/theme/theme.css` and asserts the six pairs equal (`critical`=`--critical`, `attention`=`--attention`, `healthy`=`--healthy`, `neutral`=`--neutral`, `critical-surface`=`--critical-surface`, `attention-surface`=`--attention-surface`); `TestNoColourOutsideThePalette` walks every `.typ` under `template/` except `palette.typ` and fails on `rgb(`, `cmyk(`, `color.`, or a named colour other than `black` and `white` (`red`, `green`, `blue`, `yellow`, `orange`, `purple`, `aqua`, `teal`, `eastern`, `navy`, `maroon`, `fuchsia`, `olive`, `lime`, `silver`, `gray`). Seed a temporary `rgb("#ff0000")` in `verdict.typ`, see it fail, remove it.

- [ ] **Step 2: Run** `go test ./template/ -run 'Palette|NoColour' -v`. Expected: FAIL, `palette.typ` missing.

- [ ] **Step 3: Implement the palette and the colouring**: `status-fill` returns `status-colour(status)` (critical stays bold); the gauge's filled bands take the hazard's colour (lighter thirds through `.lighten()` of that colour), its hatch stays; a table cell whose runs hold a `critical` or `attention` status run takes the matching surface as its fill (the worse when it holds both); every word and number keeps its current ink.

- [ ] **Step 4: Implement the chart** in `verdict.typ`: `donut(totals, total-label)` drawn from the `value` of each total (ring 140pt to 150pt wide in the mock, slices parted by a 2pt white gap, the sum in JetBrains Mono bold at the centre, `total-label` under it, then a legend: a swatch, the status glyph, the value, the label); `build-pressure` draws per row the status glyph and `label`, a bar of `bar.critical`, `bar.attention`, `bar.healthy` against `bar.scale` over a light track, `sentence` under it, and `counts` as a mono line of glyph and count; `build-verdict` puts the donut above the growth block in the left column. No word is added by the template. When `totals` sum to zero the donut is not drawn and `pressure.empty` shows, the growth block stays.

- [ ] **Step 5: Remove** `TestGoldenPagesHaveNoColour` and the `HasColour` helper if nothing else uses it.

- [ ] **Step 6: Run** `go test ./template/ -run 'Palette|NoColour' -v` and `go test -count=1 -tags=template ./template/ ./internal/typst/ ./internal/cli/`. Expected: PASS. (The golden pages are regenerated by the main thread in Task 8.)

- [ ] **Step 7: Rewrite the rule** in `AGENTS.md` (The template: the bullet "No page may contain a coloured pixel." becomes "A page uses only the colours of `template/capacity/palette.typ`, the light status colours of the web, which a test holds equal; words and numbers stay in ink."), `PRODUCT.md:73-74`, and `DESIGN.md`'s PDF section (the palette, the donut, the bars, the tinted peak cell).

- [ ] **Step 8: Commit**

```bash
git add template AGENTS.md PRODUCT.md DESIGN.md
git commit -m "feat(template): print the status colours and chart"
```

### Task 4: The web chart on the overview and the Findings tab

**Files:**
- Create: `web/src/features/report/findings-chart.tsx` (and `findings-chart.test.tsx`)
- Modify: `web/src/features/report/overview-tab.tsx:103-168` (`Pressure`, `PressurePair` leave), `web/src/features/report/findings-tab.tsx` (chart above the filters), `web/src/features/report/api.ts` (`ViewScope` type)
- Modify: `web/src/theme/pairs.ts`, `web/src/locales/en.json`, `web/src/locales/pt-BR.json`, `DESIGN.md`
- Test: `web/src/features/report/overview-tab.test.tsx`, `findings-tab.test.tsx`

**Interfaces:**
- Consumes: `ViewVerdict.scopes` (Task 1), `useView()`, `useAddress()`, `StatusMark`, `StatusShape`.
- Produces: `export function FindingsChart({ linkSegments }: { linkSegments: boolean })`, which renders the donut card (and, on the overview, the `GrowthPair` under it is placed by `OverviewTab`, not by the chart) and the scope card; `export function Donut({ critical, attention, healthy }: { critical: number; attention: number; healthy: number })`; `export function ScopeBar({ scope, scale, link }: { scope: ViewScope; scale: number; link: boolean })`.

- [ ] **Step 1: Write the failing tests**: on the golden view `edge-scaling.en.json`, the overview shows a donut whose centre text equals the sum of the answer strip's three counts and the word `findings`; one row per entry of `verdict.scopes`, in that order, each with its name, its worst finding's sentence as a link to `../findings?scope=<type>&finding=<index>`, and its three counts as text with `StatusMark`; the Findings tab shows the same chart above the search, and the critical segment of the first row is a link whose `href` ends with `?scope=<type>&severity=critical`; a view whose `findings` is empty (build it from the golden view with `findings: []`, `verdict.scopes: []`) shows `report.overview.noFinding`'s text and no `svg` of the chart; the answer strip and the growth card stay.

- [ ] **Step 2: Run** `cd web && npx vitest run src/features/report/`. Expected: FAIL.

- [ ] **Step 3: Implement** the chart as SVG, `aria-hidden`, fills through `fill-critical`, `fill-attention`, `fill-healthy`, `fill-track`, slices and segments parted by a 2px gap in `stroke-surface`; every count printed beside as text. The overview: answer strip, then a two-column grid (`lg`): left the donut card with `GrowthPair` under it, right the scope card ("Findings of each scope by severity, on one scale"); the scale is the largest row total. The Findings tab: the same two cards above the filters, segments as links that set `scope` and `severity` with replace (`useAddress().write`). New keys in both locale files: `report.chart.findings` ("findings" / "achados"), `report.chart.scopes` ("Findings of each scope by severity, on one scale" / "Achados de cada escopo por severidade, na mesma escala"), `report.chart.segment` ("{{count}} {{severity}} findings of {{scope}}" / "{{count}} achados {{severity}} de {{scope}}", the accessible name of a segment link), `report.overview.noFinding` reusing the existing "No finding" key if one exists under `report.*`. Pairs: `line(fg, "surface")` for `critical`, `attention`, `healthy`, `track` in `pairs.ts`.

- [ ] **Step 4: Run** the report tests, then `cd web && npx vitest run`. Expected: PASS, the contrast test included.

- [ ] **Step 5: Update** `DESIGN.md` (components: the findings chart, from the built code). Remove keys that no component uses any more (`report.overview.pressure*` that leave with `PressurePair`), so the "every key is used" test holds.

- [ ] **Step 6: Run** `make lint` and `make test`. Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add web/src DESIGN.md
git commit -m "feat(web): chart the findings of each scope"
```

### Task 5: The range bar

**Files:**
- Modify: `web/src/features/report/proof.ts` (`Proof`), `web/src/features/report/proof-bar.tsx` (`ProofBar`, `FindingProof`)
- Create: `web/src/features/report/range-bar.tsx`
- Test: `web/src/features/report/proof.test.ts`, `proof-bar.test.tsx`, `range-bar.test.tsx`

**Interfaces:**
- Produces: `type Proof = { kind: "range"; low: Evidence; high: Evidence; spread: Evidence | null } | { kind: "bar"; value: number; reference: number; count: boolean; lead: Evidence; rest: Evidence[] }`; `export function RangeBar({ low, high, spread, status }: { low: Evidence; high: Evidence; spread: Evidence | null; status: Status | null })`; `ProofBar` loses `from`.

- [ ] **Step 1: Write the failing tests**: `proofOf` returns `kind: "range"` for `minimum_peak_percent` 20 and `maximum_peak_percent` 60, and also for 0 and 60; `RangeBar` renders an open dot at 20% and a filled dot at 60% of a 0 to 100% track (`cx` as a share of the view box), labels with the evidence words and texts ("Lowest peak 20%", "Highest peak 60%", "Spread 40%" in en, from the existing `evidence.*` keys), scale labels 0%, 25%, 50%, 75%, 100%; for lowest 0 the open dot sits at 0 and no fill starts there; every other proof still renders `ProofBar` filling from zero.

- [ ] **Step 2: Run** `cd web && npx vitest run src/features/report/proof src/features/report/range-bar`. Expected: FAIL.

- [ ] **Step 3: Implement**: `FindingProof` renders `RangeBar` for a range and `ProofBar` otherwise; `RangeBar` is SVG, `aria-hidden`, coloured with the status classes, its numbers printed by `FindingProof` as today; the span between the dots at the status colour with lower opacity through a class, never a `style`.

- [ ] **Step 4: Run** `cd web && npx vitest run`, then `make lint`. Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add web/src/features/report
git commit -m "feat(web): draw a range as a range"
```

### Task 6: The node names of a pool behind a popover

**Files:**
- Modify: `web/src/components/data-table.tsx` (`renderOpened` optional; no expander column without it)
- Modify: `web/src/features/report/nodes-tab.tsx:73-127` (`Pools` on `DataTable`, the Nodes cell a popover)
- Modify: `web/src/components/no-style.test.tsx` (open the pool popover in "the report writes no style"), `web/src/locales/en.json`, `pt-BR.json`, `DESIGN.md`
- Test: `web/src/components/data-table.test.tsx`, `web/src/features/report/tables.test.tsx`

**Interfaces:**
- Consumes: `Popover`, `PopoverTrigger`, `PopoverContent` (`components/ui/popover.tsx`), `CopyName`, `DataTable`.
- Produces: `DataTable` accepts no `renderOpened`; then it renders no expander column and ignores `open` in the address.

- [ ] **Step 1: Write the failing tests**: a `DataTable` without `renderOpened` has no leading empty header cell and no expander button; the Node pools table of `edge-scaling.en.json` is a `DataTable` (sortable header buttons), its Nodes cell is a button whose accessible name is `report.pools.nodesOf` with the pool and the count ("Nodes of pool-a: 2"), whose visible text is the count; clicking it opens a popover with the heading "Nodes of pool-a", each name with a copy button, and a copy-all button (`report.pools.copyAll`, "Copy all names" / "Copiar todos os nomes"); Escape closes it; the pool `unknown` uses `report.node.unknownPool` in the heading; no node name is in the document before the click.

- [ ] **Step 2: Run** `cd web && npx vitest run src/components/data-table src/features/report/tables`. Expected: FAIL.

- [ ] **Step 3: Implement** the optional `renderOpened`, the `Pools` columns (pool name, status, confidence, nodes; sort values: name, status rank, confidence, count), and the cell: the count in mono and a chevron in a button, `PopoverContent` holding the heading (`label` style), a list of names in mono that scrolls past eight (a max height class), `CopyName` per name, and a copy-all button writing the names joined by newlines. New keys: `report.pools.nodesOf` ("Nodes of {{pool}}: {{count}}" / "Nodes de {{pool}}: {{count}}"), `report.pools.heading` ("Nodes of {{pool}}" / "Nodes de {{pool}}"), `report.pools.copyAll`. The visible button text is the count alone, since the locales hold no plural forms and the column says Nodes.

- [ ] **Step 4: Add** the pool popover to "the report writes no style" in `no-style.test.tsx`: open it on the Nodes tab and assert `styled()` is empty.

- [ ] **Step 5: Run** `cd web && npx vitest run`, `make lint`. Expected: PASS.

- [ ] **Step 6: Update** `DESIGN.md` (the count button and its popover) and `AGENTS.md` (The interface: DataTable may have no expander when its rows open nothing; the Node pools table is one).

- [ ] **Step 7: Commit**

```bash
git add web/src DESIGN.md AGENTS.md
git commit -m "feat(web): list a pool's nodes in a popover"
```

### Task 7: The browser's language after sign-in

**Files:**
- Create: `internal/platform/store/migrations/00006_language_follows_browser.sql`
- Modify: `internal/platform/store/users.go` (`User`, `userColumns`, `scanUser`, `InsertUser`, `UpdateUserFields`, new `UpdateLanguagePin`)
- Modify: `internal/platform/auth/session.go:20-25` (`Principal`), `internal/platform/auth/users.go:63-300` (`NewUser`, `UserChange`, `Create`, `Update`, `movedFields`), `internal/platform/auth/account.go:23-110` (`AccountChange`, `Change`)
- Modify: `internal/platform/api/account.go`, `internal/platform/api/users.go:12-95`, `internal/platform/api/auth.go:18`
- Modify: `api/openapi.yaml` (`Principal` :1466, `AccountChange` :1522, `User` :1534, `NewUser` :1566, `UserChange` :1580); regenerate
- Modify: `internal/cli/admin.go:91-240` (`--language` optional), `internal/cli/admin_test.go:141, 234, 270-280`
- Modify: `deploy/helm/capacity-room/values.yaml:97-98`, `templates/job-admin.yaml:37-38`, `values.schema.json:19`, `README.md:134`, `internal/chart/jobs_test.go:35, 67`; `docs/operations.md:210-222, 671-676`
- Modify: `web/src/i18n/i18n.ts`, `web/src/session/session-language.tsx`, `web/src/session/use-change-language.ts`, `web/src/app/language-menu.tsx`, `web/src/features/account/account-page.tsx:84-91`, `web/src/features/admin/users/user-row.tsx`, `new-user-dialog.tsx`, `api.ts`, the locale files
- Test: `internal/platform/store/users_test.go`, `internal/platform/auth/*_test.go`, `internal/platform/api/*_test.go` (postgres tag), `web/src/session/*.test.tsx`, `web/src/features/account/*.test.tsx`, `web/src/features/admin/users/*.test.tsx`

**Interfaces:**
- Produces: column `users.language_follows_browser boolean NOT NULL`; `store.User.LanguageFollowsBrowser bool`; `func UpdateLanguagePin(ctx context.Context, q Querier, id uuid.UUID, language string, follows bool, now time.Time) error` (writes `language`, `language_follows_browser`, `updated_at` alone); `auth.Principal.LanguageFollowsBrowser bool`; `auth.NewUser.Language` empty means follows the browser and stores `en`; `auth.UserChange.LanguageFollowsBrowser *bool` and `auth.AccountChange.LanguageFollowsBrowser *bool`; API field `language_follows_browser` (boolean, required in `Principal` and `User`, optional in `AccountChange` and `UserChange`); `NewUser.language` optional.

- [ ] **Step 1: Write the migration** `00006_language_follows_browser.sql`: `-- +goose Up` `ALTER TABLE users ADD COLUMN language_follows_browser boolean NOT NULL DEFAULT false;` then `ALTER TABLE users ALTER COLUMN language_follows_browser DROP DEFAULT;`; `-- +goose Down` drops the column. `TestReleasedMigrationsAreUnchanged` passes unedited.

- [ ] **Step 2: Write the failing tests** (postgres tag where they touch the database): an existing row migrated reads `LanguageFollowsBrowser == false`; `Create` with no language stores `en` and `true`; `PATCH /account {"language":"pt-BR"}` answers `language_follows_browser: false`; `PATCH /account {"language_follows_browser": true}` answers `true` and keeps `language`; `{"language_follows_browser": false}` answers 400 `REQUEST_INVALID` with `fields.language_follows_browser == "invalid"`; `{"language":"en","language_follows_browser":true}` answers 400 with `fields.language_follows_browser == "conflict"`; the same four on `PATCH /admin/users/{id}`; each unpin writes `account.language_changed` (account) or `user.update` with `changed.language_follows_browser: true` (admin), and a refused body writes no row; `POST /admin/users` without `language` answers 201 with `language: "en"` and `language_follows_browser: true`; `bootstrap-admin` without `--language` stores `en`, `true`, with `--language pt-BR` stores `pt-BR`, `false`, and `--language fr` still exits 2 with `error: --language must be pt-BR or en`.

- [ ] **Step 3: Run** `go test ./internal/platform/... ./internal/cli/` and, with the DSN, `go test -count=1 -tags=postgres ./internal/platform/store/ ./internal/platform/auth/ ./internal/platform/api/`. Expected: FAIL.

- [ ] **Step 4: Implement** the store, auth, and API changes. The conflict and the false value are checked after `DecodeBody`, in the handler, as a `Problem{Code: CodeRequestInvalid, Fields: ...}`; `conflict` is the field code for both fields at once (add it to the contract's description of field codes if the contract lists them). A language pin or unpin writes through `UpdateLanguagePin` alone, never through the table lock; `UpdateUserFields` keeps writing the flag it read, unchanged. Update `api/openapi.yaml`, then `make generate`.

- [ ] **Step 5: Implement** the CLI (`--language` default empty, help "Language of the administrator: pt-BR or en; when omitted, the administrator follows the browser's language."), the chart (`admin.language: ""`, `--language` passed only when set, the schema enum `["", "pt-BR", "en"]`, the README row's default empty), `jobs_test.go` (the default args hold no `--language`; `admin.language=en` adds it; `fr` still refused), and `docs/operations.md` (the flag and the value, in the bootstrap section and the admin Job description).

- [ ] **Step 6: Write the failing web tests**: signed in as a principal with `language_follows_browser: true` and `navigator.languages = ["pt-BR"]`, the app is in pt-BR; with `["fr-FR"]`, in en; with `false` and `language: "en"`, in en whatever the browser; the header menu shows the language in use and picking one sends `PATCH /account {language}`; the Account page has three choices, "Browser language" (`account.language.browser`, "Browser language" / "Idioma do navegador") sending `{language_follows_browser: true}`; the user row and the New user dialog offer the same choice, the dialog defaulting to it and omitting `language` when it is chosen.

- [ ] **Step 7: Implement** the web: `SessionLanguage` adopts `browserLanguage()` when the principal follows the browser, else `principal.language`; `useChangeLanguage` gains the unpin; the three screens as tested. List every other reader of `principal.language` and `User.language` (`grep`) and move each that means the reader's language to `i18n.language`; the report form already uses it.

- [ ] **Step 8: Run** `make test`, `make lint`, `make generate-check`, and the postgres tests. Expected: PASS.

- [ ] **Step 9: Commit**

```bash
git add internal/platform internal/cli internal/chart api deploy/helm/capacity-room docs/operations.md web/src
git commit -m "feat(platform): follow the browser's language"
```

### Task 8: Pages, changelog, and the gate (main thread)

**Files:**
- Modify: `template/tests/golden/` (by `make update-golden`), `CHANGELOG.md`, `ROADMAP.md`, `AGENTS.md` (only if a count or rule above still disagrees)

- [ ] **Step 1:** `make image`, `make update-golden`; open every changed page and check it against the 5A/6A mocks; a page that changed without a reason is a defect. `make test-golden`.
- [ ] **Step 2:** `make test-template`, `make test-chart`, `make test-postgres`, `make e2e`. Expected: PASS; the e2e admin spec still creates a user (its language choice now defaults to the browser).
- [ ] **Step 3:** Rebuild the demo images, collect the demo reports, and show the owner on the companion page the built overview, Findings tab, range bar, Node pools popover, Account language choice, and the PDF's opening page in both languages, light and dark.
- [ ] **Step 4:** `CHANGELOG.md` `[Unreleased]`: Added: the overview, the Findings tab, and the PDF's first page chart the findings of each scope; a signed-in reader follows the browser's language unless they pin one, and the Account page offers the browser's language; the node names of a pool open from their count. Changed: the PDF uses the status colours of the web app; the spread of a node pool is drawn as a range with its two ends named; `admin bootstrap-admin --language` and the chart's `admin.language` are optional, and a new administrator follows the browser's language.
- [ ] **Step 5:** Adversarial review of the whole branch (`adversarial-reviewer`), fixes, then `make gate` from images built without cache. Record every finding in `ROADMAP.md`.
- [ ] **Step 6:** The version commit 2.7.0 and the release wait for the owner's word naming each target.
