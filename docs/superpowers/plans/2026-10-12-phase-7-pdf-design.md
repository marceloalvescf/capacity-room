# Phase 7: PDF Design Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make the PDF answer the capacity question on page 1, mark only what is true, speak the web's words, and carry CapacityRoom's faces and mark, in grey.

**Architecture:** Go still decides every word and number. `internal/document` gains a `verdict` block and a `colophon` block, and Build puts the verdict first, then the opening page, then the executive section, then the evidence. The Typst layer sets Public Sans for words and JetBrains Mono for names, numbers, and status glyphs; per-table column faces replace the centring of every cell. Nothing on a page has colour.

**Tech Stack:** Go 1.27, Typst 0.15.1, the data-file contract of `template/capacity/validate.typ`, the catalog of `internal/catalog/locales/*.toml`.

**Spec:** the owner decisions of 2026-10-12 in `ROADMAP.md` (decisions table, last row), findings F-216 to F-220, and the critique `.impeccable/critique/2026-10-12T18-35-40Z__template-capacity.md`. Approved mocks: verdict page V1 at `.cache/pdf-mock/verdict/v1.typ` (render `v1-1.png`), colophon C2 at `.cache/pdf-mock/c2/`. The design of the product is `docs/superpowers/specs/2026-09-27-k8s-capacity-report-design.md`; this plan amends its PDF chapters where they bind the original's order, words, or look.

## Global Constraints

- Branch `feat/pdf-design`. Nothing is edited on `main`.
- Parity with the original binds **the numbers only**. The words follow the web; the order, the layout, and the faces are this project's.
- No page holds a coloured pixel (`TestGoldenPagesHaveNoColour`). Greys only: `luma(...)` and `black`.
- Faces: Public Sans 2.001 for words, JetBrains Mono 2.304 for names, numbers, and status glyphs. Static TTFs, SIL OFL 1.1, bundled under `template/graphite/assets/fonts/`, since `internal/typst/compile.go` passes `--ignore-system-fonts --ignore-embedded-fonts`.
- Status glyphs: healthy `●`, attention `▲`, critical `◆` (U+25C6, was `♦`), undefined `○`, unavailable `—`, set in JetBrains Mono. Grey (`luma(45%)`) for healthy, undefined, unavailable; black for attention and critical; critical bold.
- Every word on a page arrives in the data file. The template adds none and evaluates nothing as markup.
- Every catalog entry exists in `en` and `pt-BR` with the same parameters, in the form `{{.Name}}`; technical terms stay English in both languages; no em-dash inside an entry.
- A test never writes a golden file. `make update-testdata` and `make update-golden` do; the main thread runs them and reviews every changed file and page.
- Subagents never run Docker. `make test-template`, `make test-golden`, `make update-golden`, `make gate` stay on the main thread. A subagent runs the tagged tests natively: `go test -count=1 -tags=template ./template/ ./internal/typst/ ./internal/cli/` and `go test -count=1 -tags='template golden' ./template/ -run 'Golden|Snapshots'`.
- Every commit names its paths (`git add <paths>`). Commit subjects in conventional form, under 50 characters.
- No comment speaks of the process (no task, step, round, or path under `.cache/`).
- The generic layer `template/graphite` keeps its frozen surface: `graphite-report`, `callout`, `keep-together`, and the keys its README documents. Adding font files to its assets does not change that surface.

## Review Focus

1. **A report with autoscaling disabled or unknown.** The verdict shows the status word ("Disabled", "Unknown") in place of `current / max`, draws no gauge, and lists no facts. Test in Task 5 with `partial-data` or a snapshot whose `Summary.Autoscaling.Status` is not `enabled`.
2. **A report with no finding at all.** The answer strip shows "This report holds no findings." and no counts; pressure shows "Nothing needs attention."; the plan shows its empty sentence and no table. Test in Task 5 with a snapshot stripped of findings and recommendations.
3. **More than 60 nodes, or a maximum of 0 or null.** The gauge is drawn only for `0 < max`; above 60 nodes it draws three tint bands, not one per node, as the web's `bandLimit`. Test in Task 4 with fixtures `verdict.many-nodes.json` (max 120) and Task 5 with `max_nodes` null.
4. **The executive profile.** Its minimum is raised to attention; the verdict's totals still count every finding (they are `Summary` counts), and the plan lists only actions at or above the raised minimum. Test in Task 5.
5. **A long scope name** (`aks-agentpool-12345678-vmss000000`) in the pressure column and the plan. It breaks only at its own hyphens and never overflows its column. Covered by the golden pages of `edge-scaling` in Task 7, read by eye.

---

## File Structure

| File | Responsibility | Tasks |
|---|---|---|
| `template/graphite/assets/fonts/PublicSans-*.ttf`, `JetBrainsMono-*.ttf`, `OFL-PublicSans.txt`, `OFL-JetBrainsMono.txt` | the faces | 1 |
| `template/toolchain_test.go`, `template/embed_test.go` | the font files and licences exist and are embedded | 1 |
| `template/capacity/main.typ` | theme font, title band (cover), footer, block loop, sticky intro | 1, 6 |
| `template/capacity/runs.typ` | ident and number faces, status glyphs | 1 |
| `template/capacity/tables.typ` | per-table `faces` and sizes; rendering by face | 1 |
| `template/capacity_negative_test.go` | `capacityLayouts` gains faces; diagnostic count | 1, 4 |
| `internal/document/hpas.go` | HPA coverage callout kind | 2 |
| `internal/document/appendix.go` | capability state to status | 2 |
| `internal/document/nodepools.go` | node State cell mark | 2 |
| `web/src/features/report/quality-tab.tsx` (+ test) | unavailable capability mark on the web | 2 |
| `internal/catalog/locales/en.toml`, `pt-BR.toml`, `keys.go`, `format.go`, `guard_test.go` | wording | 3, 5, 6 |
| `internal/document/executive.go` | priority words, plan scope, no qualifier | 3 |
| `internal/goldentest/executive_text_test.go`, `internal/document/sections_test.go` | tests of the original's words, re-ruled | 3 |
| `internal/document/document.go` | `Verdict`, `Colophon` blocks; `Cover` fields | 4, 5, 6 |
| `template/capacity/validate.typ`, `blocks.typ`, `verdict.typ` (new), `cover.typ` | contract and drawing of the new blocks | 4, 5, 6 |
| `template/tests/capacity/fixtures/verdict.*.json`, `template/tests/capacity/negative/*` | fixtures | 4, 6 |
| `internal/document/verdict.go` (new) + `verdict_test.go` | the verdict builder | 5 |
| `internal/document/build.go`, `front.go` | order of the document; cover; front metadata | 5, 6 |
| `template/golden/snapshots.go`, `template/tests/golden/*` | golden pages | 7 |
| `AGENTS.md`, `DESIGN.md`, `PRODUCT.md`, `docs/operations.md` (if a PDF sentence there changes), `CHANGELOG.md` | records | 7 |

Tasks share files, so they run **in sequence**, never in parallel.

---

### Task 1: Faces, glyphs, and table columns

Delivers F-219. Template-only, plus the tests that pin the template.

**Files:**
- Create: `template/graphite/assets/fonts/PublicSans-{Regular,Italic,SemiBold,SemiBoldItalic,Bold,BoldItalic}.ttf`, `JetBrainsMono-{Regular,Italic,SemiBold,Bold}.ttf`, `OFL-PublicSans.txt`, `OFL-JetBrainsMono.txt`
- Modify: `template/toolchain_test.go:13-35`, `template/embed_test.go` (`TestFontLicenceIsEmbedded` near :576), `template/capacity/main.typ:41-53`, `template/capacity/runs.typ`, `template/capacity/tables.typ`, `template/capacity_negative_test.go` (`capacityLayouts` ~:470-486 and `TestCapacityTableLayoutsMatchThePlan` :487)

**Interfaces:**
- Produces: in `runs.typ`, `#let mono = "JetBrains Mono"`, `#let status-symbols` with `critical: "◆"`, and `#let status-glyph(status)` returning the glyph as content in mono with the status fill and weight (no label). Task 4 imports both.
- Produces: in `tables.typ`, every layout entry has `faces: (...)`, one per column, each one of `"name"`, `"num"`, `"text"`, `"mark"`.

- [ ] **Step 1: Copy the font files.** From nixpkgs (the pinned registry of this host), copy the static TTFs and the licences:

```bash
PS=$(nix build --no-link --print-out-paths nixpkgs#public-sans)
JB=$(nix build --no-link --print-out-paths nixpkgs#jetbrains-mono)
D=template/graphite/assets/fonts
for w in Regular Italic SemiBold SemiBoldItalic Bold BoldItalic; do install -m 0644 "$PS/share/fonts/truetype/PublicSans-$w.ttf" "$D/"; done
for w in Regular Italic SemiBold Bold; do install -m 0644 "$JB/share/fonts/truetype/JetBrainsMono-$w.ttf" "$D/"; done
```

The licences are not in those store paths: take the OFL text from each project's source release (`https://github.com/uswds/public-sans/blob/v2.001/LICENSE.md` and `https://github.com/JetBrains/JetBrainsMono/blob/v2.304/OFL.txt`), saved as `OFL-PublicSans.txt` and `OFL-JetBrainsMono.txt`, each with the copyright line of its font. Verify with `typst fonts --ignore-system-fonts --font-path template/graphite/assets/fonts` (inside `nix develop`): the list holds `JetBrains Mono`, `Liberation Sans`, `Public Sans`.

- [ ] **Step 2: Write the failing font tests.** In `template/toolchain_test.go`, rename `liberationSansFontFiles` to `fontFiles` and list all fourteen TTFs; add `TestTypstListsTheThreeFaces` (tag `toolchain`, beside `TestTypstListsLiberationSans`, which it replaces) that runs `typst fonts --ignore-system-fonts --font-path <dir>` and requires exactly the lines `JetBrains Mono`, `Liberation Sans`, `Public Sans` among its output. In `embed_test.go`, extend the licence test to require `OFL-PublicSans.txt` and `OFL-JetBrainsMono.txt` in the embedded tree beside the Liberation licence.

Run: `go test -count=1 ./template/ -run 'Font|Licence'` → FAIL before Step 1 is in place, PASS after.

- [ ] **Step 3: Set the faces.** In `template/capacity/main.typ`, pass `font: "Public Sans"` inside the `theme:` dictionary given to `graphite-report` (the key exists in `template/graphite/src/config.typ:12`; the graphite default stays Liberation Sans for its own examples). In `runs.typ`:

```typst
#let mono = "JetBrains Mono"

#let status-symbols = (
  healthy: "●",
  attention: "▲",
  critical: "◆",
  undefined: "○",
  unavailable: "—",
)

#let status-fill(status) = if status in ("healthy", "unavailable", "undefined") { luma(45%) } else { black }

// status-glyph sets the mark of a status in the monospaced face, which
// holds all five glyphs; the face of the words holds none of them.
#let status-glyph(status) = text(
  font: mono,
  fill: status-fill(status),
  weight: if status == "critical" { "bold" } else { "regular" },
  status-symbols.at(status),
)

#let run-to-content(run) = {
  if run.kind == "text" {
    text(run.text)
  } else if run.kind == "strong" {
    text(weight: "bold", run.text)
  } else if run.kind == "ident" {
    text(font: mono, size: 0.92em, hyphenate: false, lang: "en", run.text)
  } else if run.kind == "placeholder" {
    text(run.text)
  } else {
    let weight = if run.status == "critical" { "bold" } else { "regular" }
    box(text(fill: status-fill(run.status), weight: weight)[
      #status-glyph(run.status)#if run.text != none { h(2pt); text(run.text) }
    ])
  }
}
```

Keep the header comment of the file but drop the sentence that says it reproduces the original's runs; say instead that it draws the runs of the data file.

- [ ] **Step 4: Faces per column.** In `tables.typ`, give every layout a `faces` array and raise the floor of `size` to 6.8pt. Use exactly this table (columns and inset unchanged unless named):

| id | faces | size |
|---|---|---|
| report-metadata | text, text | 8.5pt |
| status-legend | mark, text | 8.5pt |
| namespace-capacity | text, num, num, num, num, num, num | 7.1pt |
| workload-resources | name, num, num, num, num, num, num, num, num, num | 6.8pt |
| hpa-observed | name, num, num, num, num, num, text | 7pt |
| workloads-without-hpa | text, name, num, text | 7.4pt |
| nodepool-summary | name, num, num, num, num, num, text | 7.1pt |
| nodepool-profile | name, num, num, num, num, num, num | 6.8pt |
| node-detail | name, num, num, text, text | 7.1pt |
| priority-workloads | name, name, num, text | 7.2pt |
| namespaces-at-risk | name, num, num, text, text, mark | 7.2pt |
| replica-ceiling | name, name, text, num, text | 7.2pt |
| nodepool-status | name, mark, text | 7.3pt |
| action-plan | num, mark, text, name | 7.2pt |
| material-limitations | text, mark, text | 7.5pt |
| quality-appendix | text, mark, text, text, text | 7pt |
| interpretation-notes | text | 7.5pt |

Render by face:

```typst
// A face says how a column reads: a name or a text from the left, a
// number from the right in the monospaced face, a mark from the left.
#let face-align = (name: left, num: right, text: left, mark: left)

#let render-table(id, headers, rows) = {
  let layout = layouts.at(id)
  let faces = layout.faces
  block(width: 100%)[
    #set text(size: layout.size)
    #set par(justify: false, leading: 0.45em, spacing: 0.4em)
    #set table(inset: (x: 3pt, y: layout.inset-y), stroke: (x, y) => (
      top: if y == 0 { 0.8pt + black } else { 0.5pt + luma(78%) },
      bottom: 0.5pt + luma(78%),
    ))
    #let header-cells = headers.enumerate().map(((i, item)) => table.cell(
      align: face-align.at(faces.at(i)) + horizon,
      text(weight: "semibold", item),
    ))
    #let body = rows.map(row => row.enumerate().map(((i, cell)) => table.cell(
      align: face-align.at(faces.at(i)) + horizon,
      if faces.at(i) == "num" { text(font: "JetBrains Mono", cell) } else { cell },
    ))).flatten()
    #table(
      columns: layout.columns,
      table.header(repeat: true, ..header-cells),
      ..body,
    )
  ]
}
```

The rules are horizontal hairlines with a black rule over the header, as in the approved mock; there are no vertical rules and no header fill.

- [ ] **Step 5: Update the layout test.** In `capacity_negative_test.go`, add a `faces []string` field to each `capacityLayouts` entry with the values of the table above and the new sizes; make `TestCapacityTableLayoutsMatchThePlan` read `faces` from `tables.typ` the way it reads `size`, and require `len(faces) == len(columns)` and every face in `{name,num,text,mark}`.

Run: `go test -count=1 -tags=template ./template/ -run 'Layout|Capacity'` → PASS. The golden comparison fails now by design (`TestGoldenPagesMatch`); do not run `make update-golden` in this task.

- [ ] **Step 6: Commit**

```bash
git add template/graphite/assets/fonts template/toolchain_test.go template/embed_test.go \
  template/capacity/main.typ template/capacity/runs.typ template/capacity/tables.typ \
  template/capacity_negative_test.go
git commit -m "feat(pdf): set the faces and the column faces"
```

---

### Task 2: Marks that tell the truth

Delivers F-217.

**Files:**
- Modify: `internal/document/hpas.go:79-90`, `internal/document/appendix.go:39-60`, `internal/document/nodepools.go` (node-detail State cell), `web/src/features/report/quality-tab.tsx:13`, `web/src/features/report/quality-tab.test.tsx:47`
- Test: `internal/document/hpas_test.go`, `internal/document/appendix_test.go`, `internal/document/nodepools_test.go`

**Interfaces:**
- Produces: `appendixStatus(state discovery.State) string` returns `"healthy"`, `"attention"`, or `"unavailable"`.

- [ ] **Step 1: Write the failing tests.**
  - `hpas_test.go`: the coverage callout of a partial and of an incomplete HPA capability has `Kind == "warning"` (today `"note"`, whose marker is the healthy shape).
  - `appendix_test.go`: `appendixStatus(discovery.StateUnavailable) == "unavailable"`; at minimum severity `critical`, `passesSeverity` keeps an unavailable row and drops partial and available ones (the minimum keeps its meaning: unavailable data is still the row a reader must see).
  - `nodepools_test.go`, against the `edge-scaling` snapshot: the State cell of `pool-a-2` (Ready false, Schedulable false) is `[StatusText("attention", "Não pronto; Não agendável")]` in pt-BR; the State cell of `orphan` (both unknown) is `[StatusText("unavailable", "Indisponível; Indisponível")]`; a Ready and Schedulable node keeps a plain text cell. This is decision 1A of 2026-10-11, now in both renderings.

Run: `go test -count=1 ./internal/document/ -run 'Callout|Appendix|NodeDetail'` → FAIL.

- [ ] **Step 2: Implement.**
  - `hpas.go`: both returns build `&Callout{Kind: "warning", ...}`.
  - `appendix.go`: `case discovery.StateUnavailable: return "unavailable"`. In `passesSeverity`, the `"critical"` branch keeps `state == discovery.StateUnavailable` as it does today; only the drawn status changes. Rewrite the comment at :39 to say unavailable is drawn as unavailable.
  - `nodepools.go`: in the node-detail row builder, wrap the State text in `StatusText("attention", text)` when `Ready` is false, in `StatusText("unavailable", text)` when `Ready` is nil, else keep `Text(text)`.
  - Web: `quality-tab.tsx:13` maps `unavailable: "unavailable"`; the test at `quality-tab.test.tsx:47` is renamed "an unavailable capability is marked unavailable" and expects the dash shape and the word for screen readers.

Run: `go test -count=1 ./internal/document/` and `cd web && npx vitest run src/features/report/quality-tab.test.tsx` → PASS.

- [ ] **Step 3: Commit**

```bash
git add internal/document/hpas.go internal/document/hpas_test.go internal/document/appendix.go \
  internal/document/appendix_test.go internal/document/nodepools.go internal/document/nodepools_test.go \
  web/src/features/report/quality-tab.tsx web/src/features/report/quality-tab.test.tsx
git commit -m "fix(pdf): mark only what is true"
```

---

### Task 3: One wording with the web

Delivers F-218. Catalog wording is decided here, on the main thread's word; an implementer copies it letter for letter.

**Files:**
- Modify: `internal/catalog/locales/en.toml`, `pt-BR.toml`, `internal/catalog/keys.go`, `internal/catalog/format.go:48-55`, `internal/catalog/guard_test.go` (`wordingTable` :112, priority rows :298 and :560), `internal/catalog/format_test.go:208-211`, `internal/document/executive.go:533-611`, `internal/goldentest/executive_text_test.go:31-372`, `internal/document/sections_test.go:533-630`, `internal/document/executive_test.go`

**Interfaces:**
- Produces: `recommendationScope(b *builder, r snapshot.Recommendation) Run` returns the full scopes (up to three) joined by `", "` as one `Ident`, or the count summary placeholder above three. The view's `ScopeText` reads it unchanged.
- Removes: the key `sections.executive.action_qualified`, the family `ScopeQualifier` and its keys, and `catalog.ScopeQualifiers`.

- [ ] **Step 1: Write the failing tests.**
  - `format_test.go`: `CPU(1) == "1 core"`, `CPU(-1) == "-1 core"`, `CPU(1.004) == "1 core"` (rounds to 1), `CPU(2) == "2 cores"`, `CPU(0.5) == "500 mCPU"` in English; in pt-BR `CPU(1) == "1 core"`, `CPU(2,5) == "2,5 cores"`.
  - `executive_test.go`: the action-plan row of `payments/web` has its scope cell `[Ident("payments/web")]` and its action cell the plain action, with no parenthesised qualifier; its priority cell is `StatusText("critical", "Critical")` in English and `StatusText("critical", "Crítico")` in pt-BR; an attention row reads `"Attention"` / `"Atenção"`.
  - `guard_test.go`: update `wordingTable` and the priority rows to the table below; the tests fail until the toml files change.

- [ ] **Step 2: Change the entries.** Exactly these, both languages (keys not listed keep their wording):

| key | en | pt-BR |
|---|---|---|
| `priority.attention` | Attention | Atenção |
| `priority.critical` | Critical | Crítico |
| `front.legend.attention` | Attention | Atenção |
| `front.legend.critical` | Critical | Crítico |
| `sections.executive.critical_heading` | Critical workloads | Workloads críticos |
| `sections.executive.attention_heading` | Workloads needing attention | Workloads que pedem atenção |
| `risk_status.critical.singular` / `.plural` | Critical 1 / Critical {{.Count}} | Crítico 1 / Crítico {{.Count}} |
| `risk_status.attention.singular` / `.plural` | Attention 1 / Attention {{.Count}} | Atenção 1 / Atenção {{.Count}} |
| `appendix.empty` | No capability of the data reaches the minimum severity of this report. | Nenhuma capacidade dos dados atinge a severidade mínima deste relatório. |
| `sections.namespaces.empty` | No namespace reaches the minimum severity of this report. | Nenhum namespace atinge a severidade mínima deste relatório. |
| `sections.workloads.empty` | No workload reaches the minimum severity of this report. | Nenhum workload atinge a severidade mínima deste relatório. |
| `sections.nodepools.empty_detail` | No node reaches the minimum severity of this report. | Nenhum node atinge a severidade mínima deste relatório. |

Before writing the four empty sentences, confirm in `sections.go:141,216`, `nodepools.go:290`, and `appendix.go:21` that each is reached only when rows exist and all fall below the minimum, or when there are none; if a key is also reached with no data at all, stop and report, since the sentence must stay true in that case.

Remove `sections.executive.action_qualified` and the `[scope_qualifier]` table from both files, their constants and family function from `keys.go`, and their rows from `wordingTable`.

- [ ] **Step 3: Implement.**
  - `format.go` `CPU`: format with `c.Decimal(cores, 2)`; when the result equals `c.Decimal(1, 2)` or `c.Decimal(-1, 2)`, append `" core"`, else `" cores"`.
  - `executive.go` `actionPlan`: drop `counts` and the qualifier branch; the action cell is `Text(actions[i])`. `recommendationScope`: for `len(r.Scopes) <= 3` return `Ident(strings.Join(r.Scopes, ", "))`; the summary branch is unchanged. Update its comment.
- [ ] **Step 4: Re-rule the tests of the original's words.** Parity binds the numbers only. In `internal/goldentest/executive_text_test.go`, remove from `originalExecutiveTexts` every entry whose key's wording changed in Step 2 (and the qualifier entries), and change the doc comment at the top of the table to say it pins the original's Portuguese for the entries whose wording this project keeps. In `internal/document/sections_test.go`, `parityTexts` maps the original's words to catalog keys and `withCatalogTexts` swaps them for the current catalog text; remove the removed keys and keep the rest. `TestNumbersEqualTheOriginal` and the rest of `original_test.go` stay untouched: they are the parity that still binds.

Run: `go test -count=1 ./internal/catalog/ ./internal/document/ ./internal/goldentest/` → PASS. Then `make update-testdata` is **not** run by a subagent here; the golden data files change in Task 7.

- [ ] **Step 5: Commit**

```bash
git add internal/catalog internal/document/executive.go internal/document/executive_test.go \
  internal/document/sections_test.go internal/goldentest/executive_text_test.go
git commit -m "feat(pdf): speak the web's words"
```

---

### Task 4: The verdict block in the contract

Adds the `verdict` block to the data file: its Go type, its check, and its drawing. Build does not emit it yet.

**Files:**
- Modify: `internal/document/document.go` (new types after `Callout`), `template/capacity/validate.typ`, `template/capacity/blocks.typ`, `template/capacity_negative_test.go:274` (diagnostic count 21 → 22)
- Create: `template/capacity/verdict.typ`, `template/tests/capacity/fixtures/verdict.en.json`, `verdict.many-nodes.json`, `verdict.disabled.json`, `verdict.empty.json`, negative fixtures under `template/tests/capacity/negative/`
- Test: `internal/document/document_test.go`, `template/capacity_test.go`, `template/capacity_negative_test.go`

**Interfaces:**
- Produces (Go, `document.go`):

```go
// Verdict is the first page: the answer, whether the cluster can grow,
// where it runs short, the first actions, and the legend.
type Verdict struct {
	Totals   VerdictTotals
	Quality  *VerdictQuality // nil when the data carries no warning
	Growth   VerdictGrowth
	Pressure VerdictPressure
	Plan     VerdictPlan
	Legend   []VerdictLegendItem // the five statuses, in legendOrder
}

// VerdictTotals holds the three counts, critical first, or the sentence
// that says there is no finding (then Items is empty).
type VerdictTotals struct {
	Items []VerdictCount
	None  *string
}

// VerdictCount is one status, its word, and its count as text.
type VerdictCount struct{ Status, Label, Count string }

// VerdictQuality is the data-quality line: its label, the number of
// warnings, and the first three in words.
type VerdictQuality struct {
	Label string
	Count string
	Items []string
	More  *string // "+N" when more than three
}

// VerdictGrowth is whether the cluster can grow.
type VerdictGrowth struct {
	Title   string
	Current *string       // nil unless the autoscaling is enabled
	Max     *string       // nil unless the autoscaling is enabled
	State   *string       // the status word when it is not enabled
	Gauge   *VerdictGauge // nil without a positive maximum
	Caption *string       // "Nodes · current / maximum" with a gauge
	Finding *VerdictFinding
	Warning *string // the node-evidence warning, when the data has it
	Facts   []VerdictFact
}

// VerdictGauge is what the depth gauge draws: whole nodes.
type VerdictGauge struct {
	Current int
	Max     int
	Min     *int
	Hazard  bool // the last band is hatched when full and the cluster has a finding above healthy
}

// VerdictFinding is a finding drawn with its mark and its sentence.
type VerdictFinding struct{ Status, Label, Sentence string }

// VerdictFact is a label and its value, such as the minimum nodes.
type VerdictFact struct{ Label, Value string }

// VerdictPressure lists the scopes with a finding above healthy, worst
// first, or the sentence that says none has one (then Rows is empty).
type VerdictPressure struct {
	Title string
	Rows  []VerdictPressureRow
	Empty *string
}

// VerdictPressureRow is one scope: its worst status, its name, the first
// finding of that status, and how many of its rows reach each status.
type VerdictPressureRow struct {
	Status   string
	Scope    string
	Sentence string
	Name     []Run         // ident, or placeholder for an unknown pool; empty when none
	Peak     *VerdictFact  // the peak percent of the lead finding, when it has one
	Counts   []VerdictCount
}

// VerdictPlan is the first actions of the plan, as a table of id
// "verdict-plan", and the line that says how many there are in all.
type VerdictPlan struct {
	Title string
	Table *Table  // nil when the plan is empty
	Empty *string // the sentence of an empty plan
	More  *string // "5 of 10 actions. The full plan is in the executive summary."
}

// VerdictLegendItem is one status and its word.
type VerdictLegendItem struct{ Status, Label string }
```

`Verdict.MarshalJSON` writes, with `"type": "verdict"` first, the keys `totals`, `quality`, `growth`, `pressure`, `plan`, `legend` in snake_case (`items`, `none`, `label`, `count`, `more`, `title`, `current`, `max`, `state`, `gauge`, `caption`, `finding`, `warning`, `facts`, `min`, `hazard`, `status`, `sentence`, `value`, `rows`, `empty`, `scope`, `name`, `peak`, `counts`, `table`), every optional field as `null` when nil, every slice as `[]` when empty, through `marshalNoEscape`. The nested `table` is the existing `Table` encoding.

- Produces (Typst): `verdict.typ` exports `#let build-verdict(entry)`; `validate.typ` gains `validate-verdict-block(place, block)`; the new diagnostic `misplaced-verdict: "block \"verdict\" is allowed only as the first block"`.

- [ ] **Step 1: Write the failing Go test.** `document_test.go`: encode a `Verdict` with every optional field nil and every slice empty, and compare with a literal JSON string; encode one with every field set and compare with a second literal. Keys in the order above.

- [ ] **Step 2: Implement the Go types and `MarshalJSON`.** Use one anonymous struct per level, as `Table` and `Callout` do, so each key's place is fixed.

- [ ] **Step 3: Write the fixtures.** `verdict.en.json` is a whole data file (schema "1") whose `blocks` are `[verdict, pagebreak, front, ...]` copied from `fixtures/command/edge-scaling.en.json`, with the verdict filled from the edge-scaling view (totals 9, 10, 7; growth current "3", max "3", gauge `{current:3,max:3,min:2,hazard:true}`, finding attention "The number of nodes is close to the observed maximum.", warning "The capacity evidence of the nodes is incomplete.", facts Minimum nodes 2, Unschedulable pods 2, Scale events 1; pressure the five rows of the V1 mock; plan the first five rows of the edge-scaling plan with `more` "5 of 10 actions. The full plan is in the executive summary."). `verdict.many-nodes.json` changes the gauge to `{current:95,max:120,min:10,hazard:false}`. `verdict.disabled.json` sets `current`, `max`, `gauge`, `caption` to null, `state` "Disabled", `facts` `[]`. `verdict.empty.json` sets `totals.items` `[]` with `none` "This report holds no findings.", `quality` null, `pressure.rows` `[]` with `empty` "Nothing needs attention.", `plan.table` null with `empty` "No prioritized action was identified.".

Negative fixtures, each `<name>.<case>.json` with its `.expect` holding the exact message:
- `misplaced-verdict.json`: a verdict at `blocks[2]`.
- `unknown-key.verdict.json`: `growth.extra`.
- `missing-key.verdict-gauge.json`: `growth.gauge` without `hazard`.
- `wrong-type.verdict-gauge.json`: `growth.gauge.max` as `"3"`.
- `wrong-type.verdict-gauge-range.json`: `growth.gauge` with `max: 0`; the message says `growth.gauge.max` must be a positive integer.
- `unknown-status.verdict.json`: a pressure row with status `"bad"`.

- [ ] **Step 4: Write `validate-verdict-block`.** Check every object with `check-exact-keys` and every leaf with the `expect-*` helpers; statuses with `allowed-statuses`; `name` with `validate-run` per run (an empty `name` is legal); `gauge.current`, `gauge.max`, `gauge.min` integers with `0 <= current`, `0 < max`, `min == none or 0 <= min <= max`, else `wrong-type`; `plan.table` either `none` or a table checked by `validate-table-block` with id `"verdict-plan"`. Add `"verdict"` to `allowed-top-block-types`; in `validate-block`, fail `misplaced-verdict` when a verdict is not at index 0, and allow `front` at index 0 or at index 2 when `blocks[0]` is a verdict (change `misplaced-front`'s check to that rule). Add the `verdict-plan` layout to `tables.typ`: columns `(0.3, 1, 4, 2.2)` in fractions, as every layout, faces `(num, mark, text, name)`, size 8pt, inset-y 4pt. Add the layout to `capacityLayouts`. Update the diagnostic count at `capacity_negative_test.go:274` to 22.

- [ ] **Step 5: Draw it.** `verdict.typ`, built from the approved mock `.cache/pdf-mock/verdict/v1.typ` (read it whole first), reading every word from `entry`:
  - the answer strip: the three counts at 13pt, the label in Public Sans, the count in JetBrains Mono, the worst present status in bold (critical bold always); when `totals.none` is set, that sentence at 13pt alone;
  - the quality line when `quality` is set: attention glyph, bold label, count in mono, the items in `luma(35%)` joined by `" · "`, then `more`;
  - a grid of columns `(0.9fr, 1.5fr)`, gutter 18pt: left the growth (eyebrow title: 7pt semibold, tracking 0.08em, upper-cased by the template, `luma(35%)`; the figure `current / max` in JetBrains Mono 30pt semibold, tracking −0.04em, with `h(0.12em)` around the slash; or the `state` word at 20pt semibold; the gauge; the caption at 7.5pt; the finding's glyph and bold label, then its sentence; the warning with an attention glyph; the facts in `luma(35%)` as `label value` joined by `" · "`), right the pressure (eyebrow title; one hairline-topped row per item: a 7.8em column holding the glyph and the semibold scope, then the sentence, the name, the peak label and value in `luma(35%)`, and the counts as glyph, word, mono count, joined by `" · "`; or `empty`);
  - the plan: a semibold 11pt title, the table through `render-table`, then `more` at 7.5pt in `luma(35%)`; or `empty`;
  - the legend line at 7.5pt: glyph and word per item, parted by 6pt.

  The gauge, from the mock's `gauge` function: width of its column, band height 16pt; when `max <= 60`, one band per node, filled bands tinted by thirds with `luma(93%)`, `luma(85%)`, `luma(74%)`, empty bands white, hairline dividers at `0.4pt + luma(55%)`; when `max > 60`, three bands over the thirds of the scale, each filled to the share of `current` that falls in it; the last band hatched in black over `luma(74%)` when `hazard` is true and `current >= max`; a dashed black line at `min` when `0 < min < max`; an outline at `0.9pt + black`; soundings under the scale in JetBrains Mono italic 5.5pt `luma(35%)`, every node when `max <= 10`, else at most ten evenly spaced, always including 0 and `max`. The soundings are numbers the template writes from the integers it was given; they are not words.

  In `blocks.typ`, dispatch `entry.type == "verdict"` to `build-verdict(entry)`, before the final `else`.

- [ ] **Step 6: Run.** `go test -count=1 ./internal/document/` and `go test -count=1 -tags=template ./template/ -run 'Capacity|Negative|Diagnostic'` → PASS. Compile the four positive fixtures natively and read each page at 144 PPI; the `verdict.en.json` page must match the approved mock in layout and differ only in the bands the data gives.

- [ ] **Step 7: Commit**

```bash
git add internal/document/document.go internal/document/document_test.go \
  template/capacity/verdict.typ template/capacity/validate.typ template/capacity/blocks.typ \
  template/capacity/tables.typ template/capacity_negative_test.go \
  template/tests/capacity/fixtures/verdict.*.json template/tests/capacity/negative
git commit -m "feat(pdf): add the verdict block"
```

---

### Task 5: Build the verdict, answer first

Delivers F-216.

**Files:**
- Create: `internal/document/verdict.go`, `internal/document/verdict_test.go`
- Modify: `internal/document/build.go:95-141`, `internal/document/front.go`, `internal/catalog/locales/*.toml`, `internal/catalog/keys.go`, `internal/catalog/guard_test.go`, `internal/document/sections_test.go:295-330` (`TestBlocksFollowTheOrderOfTheOriginal`, renamed), `internal/document/build_test.go`

**Interfaces:**
- Consumes: the `Verdict` types of Task 4; `planRecommendations(s, minRank)`, `recommendationScope`, `executiveMinimum`, `legendOrder`, `effectiveCapabilities`.
- Produces: `func buildVerdict(b *builder, s snapshot.Snapshot, severity string) Verdict`.

- [ ] **Step 1: Add the catalog entries.** The words are the web's (`web/src/locales/*.json`), so one wording holds:

| key | en | pt-BR |
|---|---|---|
| `verdict.none` | This report holds no findings. | Este relatório não tem achados. |
| `verdict.quality` | Data quality | Qualidade dos dados |
| `verdict.more_warnings` | +{{.Count}} | +{{.Count}} |
| `verdict.growth.title` | Cluster autoscaling | Autoscaling do cluster |
| `verdict.growth.caption` | Nodes · current / maximum | Nodes · atual / máximo |
| `verdict.growth.min_nodes` | Minimum nodes | Mínimo de nodes |
| `verdict.growth.unschedulable_pods` | Unschedulable pods | Pods não agendáveis |
| `verdict.growth.scale_events` | Scale events | Eventos de escala |
| `verdict.growth.state.disabled` | Disabled | Inativo |
| `verdict.growth.state.unknown` | Unknown | Desconhecido |
| `verdict.pressure.title` | Pressure by scope | Pressão por escopo |
| `verdict.pressure.empty` | Nothing needs attention. | Nada pede atenção. |
| `verdict.pressure.scope.node` | Nodes | Nodes |
| `verdict.pressure.scope.nodepool` | Node pools | Node pools |
| `verdict.pressure.scope.namespace` | Namespaces | Namespaces |
| `verdict.pressure.scope.workload` | Workloads | Workloads |
| `verdict.pressure.scope.hpa` | HPAs | HPAs |
| `verdict.pressure.peak` | Peak | Pico |
| `verdict.plan.title` | Prioritized action plan | Plano de ação priorizado |
| `verdict.plan.order` | # | # |
| `verdict.plan.priority` | Priority | Prioridade |
| `verdict.plan.action` | Action | Ação |
| `verdict.plan.scope` | Scope | Escopo |
| `verdict.plan.empty` | No prioritized action was identified. | Nenhuma ação priorizada foi identificada. |
| `verdict.plan.more` | {{.Shown}} of {{.Total}} actions. The full plan is in the executive summary. | {{.Shown}} de {{.Total}} ações. O plano completo está no resumo executivo. |
| `status.healthy` | Healthy | Saudável |
| `status.attention` | Attention | Atenção |
| `status.critical` | Critical | Crítico |

`verdict.growth.state.*` and `verdict.pressure.scope.*` are families (`keys.go` functions, like `catalog.Priority`). Add every key to `keys.go`, `wordingKeys`, `Keys()`, and `wordingTable`. If a status word already exists as a family (grep `"status\."` in `keys.go`), reuse it instead of adding `status.*`.

- [ ] **Step 2: Write the failing tests** in `verdict_test.go`, each against a real snapshot of `testdata/snapshot/` loaded the way `executive_test.go` loads one, in English, with literals:
  - `edge-scaling`: totals `{critical "Critical" "9"}, {attention "Attention" "10"}, {healthy "Healthy" "7"}`; quality count `"2"`, items `["Ambiguous HPA configuration or target values were omitted.", "The capacity evidence of the nodes is incomplete."]` (copy the exact texts from `web/tests/fixtures/view/edge-scaling.en.json` `quality.warnings[].text`), `More` nil; growth current `"3"`, max `"3"`, gauge `{3, 3, 2, true}`, finding `{attention, "Attention", "The number of nodes is close to the observed maximum."}`, warning the evidence text, facts `[{Minimum nodes, 2}, {Unschedulable pods, 2}, {Scale events, 1}]`; pressure scopes in order `Nodes, Namespaces, Workloads, HPAs, Node pools` with statuses `critical, critical, critical, critical, attention` and their sentences and names as the web overview of the same view shows them (`.cache/critique/fourth/A/shots/1024-light-en-tab-overview.png` and the view fixture); plan 5 rows, first row `["1", StatusText(critical, "Critical"), "Review the configured maximum and the capacity of the workload.", Ident("payments/web")]`, `More` `"5 of 10 actions. The full plan is in the executive summary."`.
  - `direct-complete`: totals 4, 4, 5; quality nil; gauge `{3, 3, 1, true}`; plan `More` `"5 of 7 actions. The full plan is in the executive summary."`.
  - Autoscaling not enabled (take `edge-scaling`, set `s.Summary.Autoscaling.Status = "disabled"`): current, max, gauge, caption nil; state `"Disabled"`; facts empty.
  - `max_nodes` nil with status enabled: current set, max nil, gauge nil, caption nil.
  - No findings (clear `s.Findings`, `s.Recommendations`, set the three `Summary` counts to 0): totals items empty, none `"This report holds no findings."`; pressure empty `"Nothing needs attention."`; plan table nil, empty `"No prioritized action was identified."`.
  - Executive profile of `edge-scaling` with minimum `all`: the plan holds only attention and critical rows (minimum raised), totals unchanged.

Run: `go test -count=1 ./internal/document/ -run Verdict` → FAIL.

- [ ] **Step 3: Implement `verdict.go`.** Port the web's rules exactly (they are the reference; read `web/src/features/report/overview-tab.tsx:13-184` and `cluster-verdict.tsx` whole):
  - Totals from `s.Summary.Critical`, `.Attention`, `.Healthy`, formatted with `b.cat.Integer`; when all three are 0, `None`.
  - Quality from the effective warnings the view lists (`viewBuilder` quality warnings: reuse the helper the view uses, do not re-derive); first three texts; `More` `+N` when more.
  - Growth: `enabled := s.Summary.Autoscaling.Status == "enabled"`. When enabled, `Current`, `Max` from `CurrentNodes`, `MaxNodes` with `b.cat.Integer` (nil when the value is nil); `Gauge` when both are non-nil and `max > 0`, rounding to whole nodes as the web does (`depth-gauge.tsx`), `Min` from `MinNodes`, `Hazard` when the worst cluster finding is above healthy; `Caption` with a gauge; `Facts` the three labels with `b.cat.Integer` of `MinNodes`, `UnschedulablePods`, `ScaleEvents`, an absent value written as the catalog's unavailable placeholder used by the view. When not enabled, `State` from `verdict.growth.state.<status>`. `Finding` is the worst finding of scope type `cluster`, the first of the worst severity (web `worstClusterFinding`), with its sentence through the helper the view uses for `ViewFinding.Sentence`. `Warning` is the text of warning `NODE_CAPACITY_EVIDENCE_INCOMPLETE` when present.
  - Pressure: for scope types `node, nodepool, namespace, workload, hpa`, the web's `summarise`: findings of that type grouped by the key the web's `findingKey` uses (a workload by kind too: port it from `pieces.tsx`), the worst per group, the worst over groups, the lead the first finding of that worst; skip a type whose worst is healthy; counts of groups at critical and attention, only those above 0; rows sorted by worst, critical first, stable in the type order. `Name`: `Ident(scope name)`, or `Placeholder(<the unknown-pool word>)` for an unknown pool as the PDF already writes it; `Peak` from the lead's `peak_percent` evidence text.
  - Plan: `planRecommendations(s, executiveMinimum(severity))`; the first five as rows of table id `verdict-plan` with the four headers; `More` when there are more than five, else nil.
  - Legend: `legendOrder` with `front.legend.*` words.

  Add a comment at the top of `verdict.go` naming the web overview as the other rendering of the same rules (by component, not by path to `.cache`).

- [ ] **Step 4: Put the answer first.** In `Build`:

```go
front := buildFront(b, s)
blocks := []Block{buildVerdict(b, s, severity), PageBreak{Weak: false}, front, PageBreak{Weak: false}}
blocks = append(blocks, buildExecutiveSection(b, s, thresholds, severity, profile == "executive")...)
if profile == "detailed" {
	blocks = append(blocks, buildNamespacesSection(b, s.Namespaces, s.Findings, thresholds, severity)...)
	blocks = append(blocks, buildWorkloadsSection(b, s.Workloads, s.Findings, thresholds, severity)...)
	blocks = append(blocks, buildHPAsSection(b, s)...)
	blocks = append(blocks, buildNodepoolsSection(b, s, thresholds, severity)...)
}
```

then the appendix and notes as today (Task 6 changes the page break before the appendix). Update the doc comment of `Build`. In `front.go`, the metadata table gains two rows after the period: profile (`front.metadata.profile` with the words of `report.profile` if the catalog has them, else add `front.metadata.profile.detailed` = Detailed / Detalhado and `.executive` = Executive / Executivo) and minimum severity (`front.metadata.min_severity` = Minimum severity / Severidade mínima, values the words of the web's `minSeverity` options: All / Todas, Attention and critical / Atenção e crítico, Critical only / Somente crítico). The executive profile states that it raised the minimum: the row shows the raised value.

- [ ] **Step 5: Re-rule the order test.** Rename `TestBlocksFollowTheOrderOfTheOriginal` to `TestBlocksPutTheAnswerFirst` and set `wantTypes` to the new order: `"verdict", "pagebreak", "front", "pagebreak"`, then the executive section's types (as listed today at :307-310), then the four detailed sections (as listed today at :301-304), then the appendix. Every number of every row stays as `TestNumbersEqualTheOriginal` pins it.

Run: `go test -count=1 ./internal/...` → PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/document internal/catalog
git commit -m "feat(pdf): open on the verdict"
```

---

### Task 6: Title band, colophon, and page flow

Delivers F-220 and the C2 colophon.

**Files:**
- Modify: `internal/document/document.go` (`Cover`, new `Colophon`), `internal/document/build.go`, `template/capacity/cover.typ`, `template/capacity/main.typ`, `template/capacity/blocks.typ`, `template/capacity/validate.typ`, catalog files and guards, fixtures under `template/tests/capacity/fixtures/` (every hand-maintained fixture gains the new cover keys)

**Interfaces:**
- Produces: `Cover` gains `Period string` (`"period"`) and `Profile string` (`"profile"`); `type Colophon struct{ Text string }` with JSON `{"type":"colophon","text":...}`.

- [ ] **Step 1: Catalog.**

| key | en | pt-BR |
|---|---|---|
| `cover.title` | Capacity report | Levantamento de capacidade |
| `cover.cluster_label` | Cluster | Cluster |
| `cover.author_label` | Prepared by | Elaborado por |
| `cover.issued_label` | Issued on | Emitido em |
| `colophon.text` | Generated by k8s-capacity-report from a snapshot of Prometheus-compatible metrics. Issued on {{.Issued}}. | Gerado pelo k8s-capacity-report a partir de um snapshot de métricas compatíveis com o Prometheus. Emitido em {{.Issued}}. |

The colophon sentence is the owner's, letter for letter. `k8s-capacity-report` inside it is a name: if the wording guard reads it as two words parted by a hyphen, mark nothing in the toml; the guard reads Go literals, not the catalog.

- [ ] **Step 2: Failing tests.** `build_test.go`: the last block of every document is `Colophon{Text: "Generated by k8s-capacity-report from a snapshot of Prometheus-compatible metrics. Issued on 07/15/2026 14:30."}` for `direct-complete` in English; `Cover.Period` equals the metadata's period range text; `Cover.Profile` is `"Detailed"`. When the appendix is the fallback paragraph (the executive profile of `direct-complete`), no `PageBreak` stands before the appendix heading. Template: a negative fixture `misplaced-colophon.json` (a colophon not last) with a new diagnostic `misplaced-colophon: "block \"colophon\" is allowed only as the last block"` (count 23); `trailing-pagebreak` still holds.

- [ ] **Step 3: Implement.**
  - `build.go`: `Cover.Period = periodRange(b, s)`, `Cover.Profile` the profile word; append `Colophon{Text: b.text(catalog.ColophonText, catalog.Params{"Issued": <the cover's issued text>})}` last. Emit the `PageBreak{Weak: true}` before the appendix only when the appendix holds its table.
  - `cover.typ`: the title band of the approved mock V1: a grid `(1fr, auto)`, left the title at 18pt bold, then `cluster_label` and the cluster in mono, the period with every number in mono, the profile word; right, at 8pt `luma(35%)`, the author line when set and the issued line; then a 1.2pt black rule. The cover sits on page 1 above the verdict, as `graphite-report`'s `title`.
  - `blocks.typ`: `colophon` draws a 0.5pt `luma(78%)` rule, then at 7pt `luma(35%)` the load-line disc (from `.cache/pdf-mock/c2/capacity/main.typ`: a ring of radius s/3, stroke 0.075·s, crossed by a line of length 28/30·s, stroke 0.083·s, black, s = 8pt) and the text.
  - `validate.typ`: allow `colophon` in top blocks only, as the last block; check `text` a non-blank string; cover keys gain `period` and `profile`.
  - `main.typ`: in the block loop, a paragraph that directly follows a heading is placed in `block(sticky: true)`, so that a heading, its introduction, and the next block stay on one page.
- [ ] **Step 4: Run** `go test -count=1 ./internal/... ` and `go test -count=1 -tags=template ./template/` (golden comparison excepted) → PASS. Compile `fixtures/command/direct-complete.executive.en.json` regenerated natively (`go run ./cmd/update-testdata` writes it; the main thread reviews it in Task 7, the subagent only reads the PDF it compiles) and confirm the executive PDF has no page holding only the appendix fallback sentence.
- [ ] **Step 5: Commit**

```bash
git add internal/document internal/catalog template/capacity template/tests/capacity
git commit -m "feat(pdf): add the title band and colophon"
```

---

### Task 7 (main thread): Golden files, records, and the gate

- [ ] **Step 1:** `make update-testdata`; read every changed file whole. The numbers of every row equal those before this phase (diff the numeric runs only); every word that changed is in a catalog table of Tasks 3, 5, or 6.
- [ ] **Step 2:** In `template/golden/snapshots.go`, update `Pages` and `Omits` to the new page counts; add `capacity-verdict` (data `verdict.en.json`, protects: the verdict page and the gauge with its hatch) and `capacity-verdict-many` (`verdict.many-nodes.json`, protects: the gauge above sixty nodes); rewrite the `Protects` of `capacity-parity` to say it pins the original's numbers in this project's design.
- [ ] **Step 3:** `make update-golden`; open every page that changed. Reject a page with a clipped name, a number broken across lines, a heading alone at a page foot, a page holding one sentence, or anything not grey.
- [ ] **Step 4:** Records, same commit as the goldens:
  - `AGENTS.md`: rewrite the `capacity-parity` paragraph (parity binds the numbers; words follow the web; the order, layout, and faces are this project's, decided 2026-10-12); in **The template**, name the three faces, the status glyphs in mono, the column faces, the verdict and colophon blocks; in **The layers** nothing changes.
  - `DESIGN.md`: a section **The PDF** derived from the built template: faces, greys, glyphs, the gauge in black, the verdict page, the title band, the colophon, column faces.
  - `PRODUCT.md`, Brand Commitments: the PDF keeps the name `k8s-capacity-report`, carries the load-line disc in its colophon, and shares the faces of the web; it stays monochrome.
  - `docs/decisions/0001-golden-architecture.md`: note that the faces changed and that linux/amd64 was measured; re-measure arm64 before the next release if a machine is at hand, else record it as a finding.
  - `CHANGELOG.md` `[Unreleased]`: under Changed, one sentence each: the PDF opens on a verdict page; the PDF uses the faces and words of the web app; status marks in the PDF say only what is true; the PDF ends with a line naming the program that made it. Under Fixed: "1 core" is written in the singular.
- [ ] **Step 5:** `make gate`; then the adversarial review over the branch (`adversarial-reviewer`), fixes, and the owner's word on release 1.2.0.

---

## Self-review notes

- Spec coverage: F-216 Tasks 4-5; F-217 Task 2; F-218 Task 3; F-219 Task 1; F-220 Task 6; C2 colophon Task 6; V1 Tasks 4-6; faces and mark Tasks 1, 6; monochrome Global Constraints and Task 7 Step 3; parity re-ruled Tasks 3, 5, 7.
- Out of scope, recorded as findings: F-221, the web overview computes the verdict in TypeScript while the PDF computes it in Go; route the web through a `verdict` field of the view in a later phase. F-222, a status mark with no label in a PDF cell (the legend's Status column, a percentage cell) carries no word for a screen reader.
