# Phase 10: Report Truth Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make every mark of the report agree with what it marks: node readiness and schedulability become findings, an HPA's situation carries its worst finding, the limit sentences stop predicting a risk the severity denies, node utilization reads Partial while its evidence is incomplete, and the page stops breaking a number from its unit or a mark from its label.

**Architecture:** Go decides, the template and the web draw. Two new analysis findings (`NODE_NOT_READY`, `NODE_UNSCHEDULABLE`) enter the registry and both catalogs; the view gives the node's state a status from them, so the web's one exception to "a tint follows a status the view gives" leaves AGENTS.md. The HPA situation helper reads the HPA's findings. The catalog's formatters join a number and its unit with a no-break space and write every date in ISO form. The comparison with the recorded original learns a table of ruled differences, backed by a decision record.

**Tech Stack:** Go 1.27, Typst 0.15.1, React and TypeScript (Vitest), OpenAPI 3 with oapi-codegen and openapi-typescript.

**Spec:** `docs/superpowers/specs/2026-09-27-k8s-capacity-report-design.md`, `AGENTS.md`, the owner decisions of 2026-10-13 in `ROADMAP.md` (phase 10: F-199, F-178, F-207, F-106, F-200, F-201, F-107, F-12, and the "No remark" part of F-177), and the wording sheet `.cache/phase-10/wording.md` (W1 to W8).

## Owner rulings this plan depends on

The plan is written on the recommended option of each open entry of the wording sheet: W1 and W2 as proposed, W4 A, W5 A, W7 A, W8 A. W3 and W6 are ruled. No brief is cut until the owner has ruled the sheet; a ruling other than the recommended one amends the task that names it (Task 3 for W7, Task 6 for W1 and W2, Task 8 for W8, Task 9 for W4 and W5) before its brief.

## Global Constraints

- Branch `feat/report-truth`, opened from `382fc95`; nothing is edited on `main`.
- Go decides what is said; the template and the web decide how it looks. The web applies no threshold and derives no status the view does not give.
- Every reader text comes from the catalog (Go) or the locale files (web); both catalog languages and both locale files hold the same keys with the same parameters, and every key is used.
- Technical terms stay English in both languages (request, limit, node, workload, cordon, HPA); only common nouns are Portuguese.
- Numbers equal the original exactly: no change in this phase moves a number of `testdata/python`. A finding the project writes and the original does not is a ruled difference, listed in `docs/decisions/0002-differences-from-the-original.md` and in the comparison's table; nothing else is excused.
- The public surface changes only by addition (minor version 1.5.0): `ViewNode` gains `state_status`; an empty string becomes a value of `ViewFinding.action` and `ViewCapability.remark`. The snapshot stays `schema_version` 2.0: the two new codes are new registry entries, and a snapshot written before them still reads.
- A snapshot stored before this phase has no readiness finding; its view draws the node's state unmarked and builds without error. Findings are the analysis of their run; no migration rewrites them.
- No test skips, no focused test; an expected number in a test is a literal.
- A comment says what the code does; it names no task, step, finding, phase, or ruling.
- Subagents never run Docker. `make update-testdata`, `make update-golden`, `make test-golden`, and `make gate` run on the main thread.
- Golden data and golden pages: after each task that moves output, the main thread runs `make update-testdata`, reads the diff of every changed file and explains each changed value, and commits the data with the task. `make update-golden` runs once, in Task 10, and every changed page is opened and explained before it is committed. Until then `make test-golden` is expected to fail on the pages the earlier tasks move.
- An implementer's suite passes except the comparisons with golden data files (`TestGoldenViewFiles`, the snapshot and data-file comparisons of `internal/goldentest`); the report names each failing golden test and confirms, from its diff output, that it differs only where the task says.
- Commits name their paths (`git add <paths>`), conventional format, subject under 50 characters.

## Review Focus

1. **A node whose readiness is unknown** (`orphan` of `edge-scaling`: ready and schedulable both null) gets no readiness finding, and its state reads unavailable on the PDF and the web, as today. Pinned by Task 6 (analysis) and Task 7 (view and PDF).
2. **A node that is ready but cordoned** gets `NODE_UNSCHEDULABLE` alone, and its state is marked attention on both surfaces. Pinned by Task 6 and Task 7.
3. **The executive profile** (minimum raised to attention) now lists a not-ready node in its node tables, and neither surface's action plan holds a `NODE_` code, as for every other node finding. Pinned by Task 7.
4. **A snapshot stored before this phase** (a not-ready node with no readiness finding) builds its view and its PDF without error, the state unmarked. Pinned by Task 7.
5. **The comparison with the original still fails on a stray difference.** The ruled table removes findings by code alone; a ruled code that no scenario writes fails a test; `TestComparisonCanFail` still fails on a changed number. Pinned by Task 6.

The search of the web tables reads names, words, sentences, and remarks, never a value with a unit (checked at `382fc95`: `data-table.tsx:108`, `findings-tab.tsx:131`, `workloads-tab.tsx:59`, and every tab's `searchText`), so the no-break space of Task 1 changes no search.

---

## File Structure

| File | Responsibility | Task |
|---|---|---|
| `internal/catalog/format.go`, `format_test.go` | the unit space; ISO dates | 1, 3 |
| `template/capacity/runs.typ`, `template/capacity/tables.typ` | a mark and its label on one line; column widths | 1, 2 |
| `template/golden/snapshots.go` | what the moved pages protect | 2, 10 |
| `internal/analysis/resources.go`, `resources_test.go` | the CPU variation floor | 4 |
| `docs/decisions/0002-differences-from-the-original.md` (new) | the ruled differences from the original | 4, 6 |
| `internal/document/appendix.go`, `appendix_test.go` | node utilization weakened; the empty remark | 5, 9 |
| `internal/analysis/nodes.go`, `registry.go`, `nodes_test.go`, `registry_test.go` | the readiness findings | 6 |
| `internal/catalog/keys.go`, `locales/en.toml`, `locales/pt-BR.toml`, `guard_test.go`, `keys_test.go` | the new families; the reworded entries | 6, 9 |
| `internal/document/reasons.go` | the reason fragment of the new codes | 6 |
| `internal/goldentest/original_test.go` | the table of ruled codes | 6 |
| `internal/document/nodepools.go`, `view.go`, `view_test.go`, `nodepools_test.go` | the node's state status | 7 |
| `api/openapi.yaml`, `web/src/api/schema.gen.ts`, `internal/platform/api/view_contract_test.go` | `ViewNode.state_status` | 7 |
| `web/src/features/report/nodes-tab.tsx`, `autoscaling-nodes.test.tsx` | the State column from the view | 7 |
| `internal/document/hpas.go`, `hpas_test.go` | the HPA situation from its findings | 8 |
| `internal/document/view.go`, `web/src/features/report/findings-tab.tsx` | the healthy action | 9 |
| `AGENTS.md`, `DESIGN.md`, `CHANGELOG.md` | records | 7, 10 |

Tasks run in sequence: they share the catalog, `view.go`, the golden data, and the guard tables.

---

### Task 1: A number and its unit stay together (F-107)

**Files:**
- Modify: `internal/catalog/format.go:48-75` (`CPU`, `Memory`) and `:94-118` (`Period`)
- Test: `internal/catalog/format_test.go` (`TestCPU`, `TestMemory`, `TestPeriod`), `internal/document/sections_test.go:133`, and every other Go test that expects a unit after a plain space (`grep -rn '" mCPU\| mCPU"\| MiB"\| GiB"\| KiB"\| core"\| cores"\| hours"\| horas"\| days"\| dias"' --include=*_test.go internal cmd template`)

**Interfaces:**
- Produces: `CPU`, `Memory`, and `Period` write U+00A0 between the number and the unit; `Percent` is unchanged (it has no space).

- [ ] **Step 1: Change the expected strings in the tests**

In `format_test.go`, every expected CPU, Memory, and Period string takes ` ` in place of the space before its unit, for example:

```go
{0.3, "300 mCPU"},
{1, "1,00 core"},   // the pt-BR case of the table, as written there today
{1536, "1,50 KiB"},
```

Keep each table's existing inputs; change only the separator. Do the same in `sections_test.go:133` and in every test the grep above finds.

- [ ] **Step 2: Run the tests to see them fail**

Run: `go test ./internal/catalog/ ./internal/document/ -run 'TestCPU|TestMemory|TestPeriod|Capacity' -count=1`
Expected: FAIL, the formatter still writes a plain space.

- [ ] **Step 3: Write the unit space**

In `format.go`, above `CPU`:

```go
// unitSpace joins a number and its unit: a no-break space, so that a line
// never breaks between them.
const unitSpace = " "
```

Replace `" mCPU"`, `" core"`, `" cores"` with `unitSpace + "mCPU"`, `unitSpace + "core"`, `unitSpace + "cores"`; in `Memory`, `+ " " + unit` with `+ unitSpace + unit`; in `Period`, `+ " " + word` with `+ unitSpace + word`. Update the doc comments of the three functions to say "followed by a no-break space and the unit".

- [ ] **Step 4: Run the package tests**

Run: `go test ./internal/catalog/ ./internal/document/ ./internal/goldentest/ -count=1`
Expected: PASS except the golden-file comparisons. Then run the template-tagged tests natively, since `pdftotext` reads the no-break space as U+00A0:
`go test -count=1 -tags=template ./template/ ./internal/typst/ ./internal/cli/`
Expected: PASS, or failures only where a test expects the old space; fix those expectations the same way.

- [ ] **Step 5: Commit**

```bash
git add internal/catalog/format.go internal/catalog/format_test.go internal/document/sections_test.go <other test files changed>
git commit -m "fix(catalog): keep a number and its unit together"
```

- [ ] **Step 6 (main thread): Golden data and a render check**

Run `make update-testdata`, read the diff of every changed file (only the separator before a unit may change), and commit them: `git commit -m "test(testdata): unit space"` with the changed paths named. Render the command fixtures of `direct-complete` (en and pt-BR) natively at 144 PPI into `.cache/phase-10/render/` with the arguments the golden test uses, and look at the ten-column workload table and the namespace table. A value that now overflows its cell into the next is fixed in the same task by widening that column in `template/capacity/tables.typ` (the column's `fr` share, the others unchanged), with a fix brief to the implementer.

---

### Task 2: A status mark and its label stay on one line (F-12)

Reproduced at `382fc95` on the golden page `capacity-pt-5`: in the quality appendix the Coverage cell of "Escalonamento automático de nós" sets the critical mark on one line and the bold "Indisponível" under it.

**Files:**
- Modify: `template/capacity/runs.typ:49-53` (the status run), and `template/capacity/tables.typ` only if a label cannot fit its column even on a line of its own
- Modify: `template/golden/snapshots.go` (the `Protects` of `capacity-pt`)

**Interfaces:**
- Consumes: the status run `(kind: "status", status, text, alt)`; its contract does not change.
- Produces: a status run whose mark and first word of its label always share a line; a label longer than its column wraps after its first word, never before it.

- [ ] **Step 1: Render the reproduction**

Find the arguments of the golden render in `template/golden/` (the code that writes the PNGs for `make update-golden`) and render `capacity-pt` natively, the same way, into `.cache/phase-10/f12/before/`. Confirm page 5 shows the mark above "Indisponível". Then render every golden snapshot of the list into `.cache/phase-10/f12/before/` so the next step can compare them all.

- [ ] **Step 2: Bind the mark to the label**

In `runs.typ`, the status run joins mark and label with a gap through which no line may break. Write it so that the mark, the gap, and the label's first word form one unbreakable unit and the rest of the label may still wrap, for example by placing a word joiner on each side of the gap:

```typst
box(text(fill: status-fill(run.status), weight: weight)[
  #status-mark(run.status, run.at("alt", default: none))#if run.text != none { sym.wj; h(2pt); sym.wj; text(run.text) }
])
```

If Typst still breaks there, set the mark and the label's first word inside one inner `box` and leave the remaining words outside it. Keep the mark a PDF artifact as it is today (its `alt` handling is unchanged).

- [ ] **Step 3: Render again and compare**

Render every golden snapshot into `.cache/phase-10/f12/after/` and compare each page's SHA-256 with `before/`. The changed pages must be exactly those that held a label wrapped under its mark; open each changed page and confirm the mark and its label share a line. If "Indisponível" in bold still cannot fit the appendix's Coverage column on one line, widen that column in `tables.typ` (the `quality-appendix` layout, and `material-limitations` if it shows the same) by the smallest share that fits, and say so in the report.

- [ ] **Step 4: Record what the page protects**

Add to the `Protects` of `capacity-pt` in `snapshots.go`: "a status mark and the first word of its label on one line in the narrow Coverage column of the appendix".

- [ ] **Step 5: Run the template tests**

Run: `go test -count=1 -tags=template ./template/` and `go test ./template/golden/ -count=1`
Expected: PASS (the golden comparison itself is not run here; it fails until Task 10 on the moved pages).

- [ ] **Step 6: Commit**

```bash
git add template/capacity/runs.typ template/golden/snapshots.go <tables.typ if changed>
git commit -m "fix(template): keep a status mark with its label"
```

The report lists the pages that changed between `before/` and `after/`.

---

### Task 3: Every date is ISO (F-201, W7 A)

**Files:**
- Modify: `internal/catalog/format.go:11-19` (`DateTime`)
- Test: `internal/catalog/format_test.go` (`TestDateTime`), and every Go or web test that expects a date as `MM/DD/YYYY` or `DD/MM/YYYY` (`grep -rnE '[0-9]{2}/[0-9]{2}/20[0-9]{2}' --include=*_test.go internal cmd template; grep -rnE '[0-9]{2}/[0-9]{2}/20[0-9]{2}' web/src`)

**Interfaces:**
- Produces: `(*Catalog).DateTime(t)` returns `t.Format("2006-01-02 15:04")` in both languages. Its callers are unchanged: the period range (`front.go:123`, the PDF metadata, the cover, and the view's `period_text`), the cover's issued date and the colophon (`build.go:181`, `:136`), and the view's `emitted_at_text` (`view.go:404`).

- [ ] **Step 1: Write the failing test**

`TestDateTime` expects, for `time.Date(2026, 7, 15, 14, 30, 0, 0, time.UTC)`, `"2026-07-15 14:30"` from both the English and the Portuguese catalog. Change every other expectation the grep finds to the ISO form.

- [ ] **Step 2: Run it to see it fail**

Run: `go test ./internal/catalog/ -run TestDateTime -count=1`
Expected: FAIL, got `07/15/2026 14:30` and `15/07/2026 14:30`.

- [ ] **Step 3: Write ISO**

```go
// DateTime writes the wall time of t as an ISO date and a time of day,
// 2006-01-02 15:04, in both languages.
func (c *Catalog) DateTime(t time.Time) string {
	return t.Format("2006-01-02 15:04")
}
```

- [ ] **Step 4: Run the tests**

Run: `go test ./internal/... -count=1` and `cd web && npx vitest run`
Expected: PASS except the golden-file comparisons.

- [ ] **Step 5: Commit**

```bash
git add internal/catalog/format.go internal/catalog/format_test.go <other changed tests>
git commit -m "feat(catalog): write every date in ISO form"
```

- [ ] **Step 6 (main thread):** `make update-testdata`; read every changed file's diff (only dates may change); commit `test(testdata): ISO dates`.

---

### Task 4: CPU variation is judged from 1 mCPU (F-106)

**Files:**
- Modify: `internal/analysis/resources.go:93` (the guard of the `_BURSTY` finding)
- Test: `internal/analysis/resources_test.go` (beside `TestBurstyBoundary` at `:170`)
- Create: `docs/decisions/0002-differences-from-the-original.md`

**Interfaces:**
- Produces: `NAMESPACE_CPU_BURSTY` and `WORKLOAD_CPU_BURSTY` are judged only when the CPU average is at least 0.001 cores; the memory codes keep `Average > 0`. No recorded scenario has a CPU average below 1 mCPU (checked at `382fc95`: the lowest is 0.1 cores in `edge-values`), so no number of `testdata/python` and no golden file moves.

- [ ] **Step 1: Write the failing test**

```go
// TestCPUVariationNeedsOneMillicore pins the floor of the CPU variation:
// an average below 1 mCPU is not judged, an average of exactly 1 mCPU is,
// and memory keeps no floor.
func TestCPUVariationNeedsOneMillicore(t *testing.T) {
	cases := []struct {
		name     string
		resource string
		average  float64
		peak     float64
		want     bool
	}{
		{"cpu below one millicore", "cpu", 0.0009, 0.5, false},
		{"cpu at one millicore", "cpu", 0.001, 0.5, true},
		{"memory below one byte keeps being judged", "memory", 0.0009, 0.5, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// Build usage with State collect.StateValue, Average c.average,
			// Peak c.peak, and call resourceFindings the way
			// TestBurstyBoundary does, with default thresholds.
			// got := any finding whose Code ends in "_BURSTY".
			// if got != c.want { t.Errorf(...) }
		})
	}
}
```

Write the body after the model of `TestBurstyBoundary` in the same file, with the same helper it uses to call `resourceFindings` and the same thresholds; the comments above are the assertion to write, not text to keep.

- [ ] **Step 2: Run it to see it fail**

Run: `go test ./internal/analysis/ -run TestCPUVariationNeedsOneMillicore -count=1 -v`
Expected: FAIL on "cpu below one millicore" (a `_BURSTY` finding is written today).

- [ ] **Step 3: Write the floor**

In `resources.go`, above `resourceFindings`:

```go
// minimumCPUAverage is the CPU average, in cores, below which the variation
// of CPU usage is not judged: one millicore, under which the gap between
// the peak and the average is a ratio of rounding noise.
const minimumCPUAverage = 0.001
```

and replace the guard at `:93`:

```go
	judged := usage.Average > 0
	if resource == "cpu" {
		judged = usage.Average >= minimumCPUAverage
	}
	if usage.State == collect.StateValue && judged {
```

Update the function's header comment where it describes BURSTY to name the floor.

- [ ] **Step 4: Write the decision record**

Create `docs/decisions/0002-differences-from-the-original.md`:

```markdown
# 0002: Findings that differ from the original

Date: 2026-10-13
Status: decided

## Question

The numbers of this project equal those of kubernetes-report 1.11.0, and
`internal/goldentest` compares them with the recorded runs of
`testdata/python` on every test run. Some findings of the original mislead
a reader. Which differences are kept, and how does the comparison know them?

## Decision

Parity binds the numbers. A finding may differ from the original's only
by a decision recorded here. The comparison removes the findings of a
ruled code before it compares, and fails when a ruled code is written by
no recorded scenario.

### CPU variation from one millicore

`NAMESPACE_CPU_BURSTY` and `WORKLOAD_CPU_BURSTY` are judged only when the
CPU average is at least 1 mCPU. The original judges every average above
zero, and a real cluster showed variations of 18813 % and 28800 % on
averages that round to 0 mCPU. No recorded scenario has a CPU average
below 1 mCPU, so the comparison removes nothing for this decision. The
memory codes are unchanged.
```

- [ ] **Step 5: Run the tests**

Run: `go test ./internal/analysis/ ./internal/goldentest/ -count=1`
Expected: PASS, golden comparisons included (nothing moves).

- [ ] **Step 6: Commit**

```bash
git add internal/analysis/resources.go internal/analysis/resources_test.go docs/decisions/0002-differences-from-the-original.md
git commit -m "fix(analysis): judge CPU variation from 1 mCPU"
```

---

### Task 5: Node utilization reads Partial while its evidence is incomplete (F-200)

**Files:**
- Modify: `internal/document/appendix.go:92-118` (`ambiguityWarnings`, `effectiveCapabilities`)
- Test: `internal/document/appendix_test.go:115-170` (the table of weakened capabilities)

**Interfaces:**
- Consumes: `collect.WarningNodeCapacityEvidenceIncomplete` (`NODE_CAPACITY_EVIDENCE_INCOMPLETE`).
- Produces: `effectiveCapabilities` also turns an available `node_utilization` partial while that warning stands, its code the one reason. Every caller picks it up: the appendix, the executive limitations (`executive.go:611`), the quality list counts (`front.go:163`), and the view's quality (`view.go:689`). The web needs no change: `partial` is already in `ViewCapabilityState` and drawn by `quality-tab.tsx`. Only `edge-scaling` raises the warning today.

- [ ] **Step 1: Write the failing test**

Add a row to the table of `appendix_test.go` beside `{"ambiguous spec turns hpa_history partial", collect.WarningHPAAmbiguousSpec, 3}`:

```go
{"incomplete node evidence turns node_utilization partial", collect.WarningNodeCapacityEvidenceIncomplete, 7},
```

with the same assertions as the existing rows (the capability at that index is partial, its one reason is the warning's code, the snapshot is untouched). Confirm index 7 is `node_utilization` in the test's capability list and correct it if the list differs.

- [ ] **Step 2: Run it to see it fail**

Run: `go test ./internal/document/ -run Appendix -count=1 -v`
Expected: FAIL, `node_utilization` stays available.

- [ ] **Step 3: Weaken node utilization**

Rename `ambiguityWarnings` to `weakeningWarnings` and add the entry:

```go
// weakeningWarnings names, for each capability that a warning weakens, the
// warning that turns it partial: an ambiguous HPA weakens the HPA history
// and the workloads without HPA, and incomplete node capacity evidence
// weakens the node utilization.
var weakeningWarnings = map[string]string{
	"hpa_history":           collect.WarningHPAAmbiguousSpec,
	"workloads_without_hpa": collect.WarningHPAAmbiguousTarget,
	"node_utilization":      collect.WarningNodeCapacityEvidenceIncomplete,
}
```

Update the comment of `effectiveCapabilities` to name the third pair.

- [ ] **Step 4: Run the tests**

Run: `go test ./internal/document/ -count=1`
Expected: PASS except the golden-file comparisons, which differ only for `edge-scaling`.

- [ ] **Step 5: Commit**

```bash
git add internal/document/appendix.go internal/document/appendix_test.go
git commit -m "fix(document): node utilization partial on warning"
```

- [ ] **Step 6 (main thread):** `make update-testdata`; the diff touches `edge-scaling` alone (the appendix row, the limitations, the quality counts, the view's capability); commit `test(testdata): partial node utilization`.

---

### Task 6: Readiness and schedulability are findings (F-199, W1, W2)

**Files:**
- Modify: `internal/analysis/nodes.go:80-145` (`Nodes` and its header comment), `internal/analysis/registry.go:236-243` (two definitions after `NODE_LOW_UTILIZATION`)
- Modify: `internal/catalog/keys.go:487` (`ObservationFamilies`), `internal/catalog/locales/en.toml` and `pt-BR.toml` (two families after `[finding.node_low_utilization]`)
- Modify: `internal/document/reasons.go:81-86` (two cases)
- Modify: `internal/goldentest/original_test.go` (the ruled codes)
- Modify: `docs/decisions/0002-differences-from-the-original.md` (a second section)
- Test: `internal/analysis/nodes_test.go`, `internal/analysis/registry_test.go` (count 66 to 68, the definition table at `:138`, the confidence map at `:333`), `internal/catalog/guard_test.go` (the wording tables at `:1940` and `:1969`), `internal/catalog/keys_test.go`, `internal/document/reasons_test.go`, `internal/goldentest/original_test.go`

**Interfaces:**
- Consumes: `collect.Node.Ready` and `collect.Node.Schedulable`, each a `collect.Flag{Value, Known bool}` read at the end of the window.
- Produces: findings `NODE_NOT_READY` (family `node_not_ready`) and `NODE_UNSCHEDULABLE` (family `node_unschedulable`), severity attention, scope node, no evidence, confidence exact. `analysis.Registry` holds 68 codes. Task 7 relies on the two code strings.

- [ ] **Step 1: Write the failing analysis test**

In `nodes_test.go`:

```go
// TestNodeReadinessFindings pins the two readiness findings: a known false
// readiness or schedulability is a finding of attention, a known true one
// is none, and an unknown one is none.
func TestNodeReadinessFindings(t *testing.T) {
	cases := []struct {
		name        string
		ready       collect.Flag
		schedulable collect.Flag
		want        []string
	}{
		{"ready and schedulable", collect.Flag{Value: true, Known: true}, collect.Flag{Value: true, Known: true}, nil},
		{"not ready", collect.Flag{Value: false, Known: true}, collect.Flag{Value: true, Known: true}, []string{"NODE_NOT_READY"}},
		{"cordoned", collect.Flag{Value: true, Known: true}, collect.Flag{Value: false, Known: true}, []string{"NODE_UNSCHEDULABLE"}},
		{"not ready and cordoned", collect.Flag{Value: false, Known: true}, collect.Flag{Value: false, Known: true}, []string{"NODE_NOT_READY", "NODE_UNSCHEDULABLE"}},
		{"unknown", collect.Flag{}, collect.Flag{}, nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			node := collect.Node{Name: "n1", Ready: c.ready, Schedulable: c.schedulable}
			var got []string
			for _, f := range Nodes([]collect.Node{node}, config.DefaultThresholds()) {
				if f.Code != "NODE_NOT_READY" && f.Code != "NODE_UNSCHEDULABLE" {
					continue
				}
				got = append(got, f.Code)
				if f.Severity != SeverityAttention || f.ScopeType != ScopeNode || f.ScopeName != "n1" ||
					f.Confidence != discovery.ConfidenceExact || len(f.Evidence) != 0 {
					t.Errorf("finding %+v, want attention, node n1, exact, no evidence", f)
				}
			}
			if !slices.Equal(got, c.want) {
				t.Errorf("codes = %v, want %v", got, c.want)
			}
		})
	}
}
```

Use the thresholds constructor the file already uses if it is not `config.DefaultThresholds()`, and the node fixture helper of the file if it builds nodes another way (the usage fields left unknown, so no usage finding interferes). Update `TestRegistryHoldsSixtySixCodes` to `TestRegistryHoldsSixtyEightCodes` with 68, and add the two rows to its definition table and confidence map (`exact`).

- [ ] **Step 2: Run it to see it fail**

Run: `go test ./internal/analysis/ -count=1`
Expected: FAIL, no readiness finding and 66 codes.

- [ ] **Step 3: Register and write the findings**

In `registry.go`, right after the `NODE_LOW_UTILIZATION` definition:

```go
		Definition{Code: "NODE_NOT_READY", Family: "node_not_ready", Severities: []string{SeverityAttention}, ScopeType: ScopeNode, Params: map[string]string{},
			Evidence: []EvidenceSpec{}, Confidence: fixedExact},
		Definition{Code: "NODE_UNSCHEDULABLE", Family: "node_unschedulable", Severities: []string{SeverityAttention}, ScopeType: ScopeNode, Params: map[string]string{},
			Evidence: []EvidenceSpec{}, Confidence: fixedExact},
```

In `nodes.go`, inside the loop over `ordered`, after the `NODE_LOW_UTILIZATION` block:

```go
		if node.Ready.Known && !node.Ready.Value {
			findings = append(findings, Finding{
				Code:       "NODE_NOT_READY",
				Severity:   SeverityAttention,
				ScopeType:  ScopeNode,
				ScopeName:  node.Name,
				Evidence:   []Evidence{},
				Confidence: discovery.ConfidenceExact,
			})
		}
		if node.Schedulable.Known && !node.Schedulable.Value {
			findings = append(findings, Finding{
				Code:       "NODE_UNSCHEDULABLE",
				Severity:   SeverityAttention,
				ScopeType:  ScopeNode,
				ScopeName:  node.Name,
				Evidence:   []Evidence{},
				Confidence: discovery.ConfidenceExact,
			})
		}
```

Extend the header comment of `Nodes` with the two codes, last in each node's order.

- [ ] **Step 4: Write the catalog entries (W1, W2)**

Add `"node_not_ready", "node_unschedulable"` to `ObservationFamilies` in `keys.go`. In `en.toml`, after `[finding.node_low_utilization]`:

```toml
[finding.node_not_ready]
summary = "The node was not ready at the end of the period."
action = "Check the conditions of the node and move its workloads if it does not recover."
observation = "Not ready"

[finding.node_unschedulable]
summary = "The node was unschedulable at the end of the period."
action = "Confirm that the node was cordoned on purpose and plan its return or its removal."
observation = "Unschedulable"
```

In `pt-BR.toml`, at the same place:

```toml
[finding.node_not_ready]
summary = "O node não estava pronto no fim do período."
action = "Verificar as condições do node e mover os workloads se ele não se recuperar."
observation = "Não pronto"

[finding.node_unschedulable]
summary = "O node estava marcado como não agendável no fim do período."
action = "Confirmar que o cordon do node foi intencional e planejar o retorno ou a remoção dele."
observation = "Não agendável"
```

Add the matching rows to the verbatim wording tables of `guard_test.go` (summary and action at `:1940`, observation at `:1969`) and the new keys to `keys_test.go`. In `reasons.go`, add beside the two existing cases:

```go
	case "NODE_NOT_READY":
		return strings.TrimRight(b.text(catalog.FindingSummary("node_not_ready", ""), nil), ".")
	case "NODE_UNSCHEDULABLE":
		return strings.TrimRight(b.text(catalog.FindingSummary("node_unschedulable", ""), nil), ".")
```

with a row for each in `reasons_test.go`.

- [ ] **Step 5: Teach the comparison the ruled codes**

In `original_test.go`:

```go
// ruledCodes holds the finding codes this project writes and the original
// does not, each a decision of
// docs/decisions/0002-differences-from-the-original.md.
var ruledCodes = []string{"NODE_NOT_READY", "NODE_UNSCHEDULABLE"}

// withoutRuledCodes returns a copy of snap without the findings and the
// recommendations of ruledCodes, its summary counts lowered by what it
// removed, and the number of findings removed for each ruled code. snap
// itself is left as it is.
func withoutRuledCodes(snap snapshot.Snapshot) (snapshot.Snapshot, map[string]int)
```

The copy clones `Findings` and `Recommendations` before filtering; `Summary.Findings` and `Summary.Recommendations` drop by the number removed, and each severity count by the removed findings of that severity. `TestNumbersEqualTheOriginal` passes `withoutRuledCodes(out.Snapshot)`'s snapshot to `compareNumbers`, so every part of the comparison that is built from the snapshot (the findings, the recommendations, the summary, the executive rows) sees the original's set. Add:

```go
// TestRuledCodesAreWritten fails when a ruled code is written by no
// recorded scenario, so that the table cannot outlive its decision.
func TestRuledCodesAreWritten(t *testing.T)
```

which runs every scenario of `promtest.Scenarios`, sums the removed counts, and reports each ruled code with a sum of zero. And a test that `edge-scaling` writes `NODE_NOT_READY` and `NODE_UNSCHEDULABLE` for `pool-a-2` and no readiness finding for `orphan` (ready and schedulable unknown), run through `run()` like the other comparisons. If any other part of the comparison still differs after the filter (a pressure, an executive row, a count not listed here), stop and report: it is a difference this plan does not rule.

- [ ] **Step 6: Record the decision**

Append to `docs/decisions/0002-differences-from-the-original.md`:

```markdown
### Node readiness and schedulability

`NODE_NOT_READY` and `NODE_UNSCHEDULABLE`, of attention, mark a node whose
Ready condition was false, or that was cordoned, at the end of the period.
The original reads both states and writes them in its node table, but
writes no finding, so a node that was not ready read as healthy. The
recorded scenario `edge-scaling` holds one such node, `pool-a-2`; the
comparison removes its two findings and their two recommendations, and
lowers the counts of findings, recommendations, and attention by two.
```

- [ ] **Step 7: Run the tests**

Run: `go test ./internal/... -count=1`
Expected: PASS except the golden-file comparisons, which differ only for `edge-scaling` (and its executive data files).

- [ ] **Step 8: Commit**

```bash
git add internal/analysis/nodes.go internal/analysis/registry.go internal/analysis/nodes_test.go internal/analysis/registry_test.go internal/catalog/keys.go internal/catalog/keys_test.go internal/catalog/guard_test.go internal/catalog/locales/en.toml internal/catalog/locales/pt-BR.toml internal/document/reasons.go internal/document/reasons_test.go internal/goldentest/original_test.go docs/decisions/0002-differences-from-the-original.md
git commit -m "feat(analysis): find nodes not ready or cordoned"
```

- [ ] **Step 9 (main thread):** `make update-testdata`; in `edge-scaling` the two findings, their recommendations, the summary (findings 28, recommendations 21, attention 12), the node `pool-a-2` in the executive node tables, its reason cell, and the node pressure lead may change, nothing else; commit `test(testdata): node readiness findings`.

---

### Task 7: The node's state takes the status the view gives (F-199)

**Files:**
- Modify: `internal/document/nodepools.go:315-326` (`nodeState`), `internal/document/view.go:193-210` (`ViewNode`) and `:637-660` (`nodes`)
- Modify: `api/openapi.yaml` (`ViewNode`), then `make generate` (`web/src/api/schema.gen.ts`)
- Modify: `web/src/features/report/nodes-tab.tsx:64-75` (`NodeState`)
- Modify: `AGENTS.md:636-642`, `DESIGN.md:235`
- Test: `internal/document/nodepools_test.go`, `internal/document/view_test.go`, `web/src/features/report/autoscaling-nodes.test.tsx:130-138,222-264`

**Interfaces:**
- Consumes: the codes `NODE_NOT_READY` and `NODE_UNSCHEDULABLE` from Task 6.
- Produces: `nodeStateStatus(n snapshot.Node, findings []snapshot.Finding) string`, which returns `"unavailable"` when `n.Ready` is nil, else the worst severity of the node's readiness findings, else `""`. `ViewNode.StateStatus *string` (`json:"state_status"`), `nil` for `""`. The PDF's state cell and the view both read it.

- [ ] **Step 1: Write the failing Go tests**

In `view_test.go`, a table test over `nodeStateStatus` and the view's `state_status`:

| Node | Findings | want |
|---|---|---|
| ready nil, schedulable nil | none | `unavailable` |
| ready false, schedulable true | `NODE_NOT_READY` (attention) | `attention` |
| ready true, schedulable false | `NODE_UNSCHEDULABLE` (attention) | `attention` |
| ready true, schedulable true | none | `""` (view: `null`) |
| ready false, schedulable true | none, as in a snapshot stored before this phase | `""` (view: `null`), and the view builds without error |

In `nodepools_test.go`, the PDF state cell of the second and third rows is a status run of attention with the state text ("Not ready; Schedulable", "Ready; Unschedulable"), the fourth and fifth a plain text run, the first a status run of unavailable, all with the texts of today. Add one test that the executive document of a snapshot holding a `NODE_NOT_READY` node lists that node in its node table and holds no `NODE_` code in its action plan, and that the view's recommendations hold none either.

- [ ] **Step 2: Run them to see them fail**

Run: `go test ./internal/document/ -count=1 -run 'NodeState|Executive'`
Expected: FAIL, `nodeStateStatus` is undefined.

- [ ] **Step 3: Write the helper and use it on both surfaces**

In `nodepools.go`:

```go
// readinessCodes are the findings of a node's state.
var readinessCodes = []string{"NODE_NOT_READY", "NODE_UNSCHEDULABLE"}

// nodeStateStatus is the status of a node's state: unavailable when its
// readiness is unknown, else the worst severity of its readiness findings,
// else none ("").
func nodeStateStatus(n snapshot.Node, findings []snapshot.Finding) string {
	if n.Ready == nil {
		return "unavailable"
	}
	status := ""
	for _, f := range findings {
		if f.ScopeType != analysis.ScopeNode || f.ScopeName != n.Name || !slices.Contains(readinessCodes, f.Code) {
			continue
		}
		if status == "" || analysis.Rank(f.Severity) > analysis.Rank(status) {
			status = f.Severity
		}
	}
	return status
}
```

`nodeState(b, n, findings)` builds the same text as today and returns `StatusValue(status, text, statusWord(b, status))` when `nodeStateStatus` is not `""`, else `Text(text)`; pass the snapshot's findings from its caller `nodeRow`. In `view.go` add to `ViewNode`, after `Schedulable`:

```go
	// StateStatus is the mark of the state: unavailable when readiness is
	// unknown, else the severity of the node's readiness findings; nil when
	// the state carries no mark.
	StateStatus *string `json:"state_status"`
```

and set it in `nodes()` from `nodeStateStatus(n, s.Findings)` through the file's `mark` helper, `nil` for `""`.

- [ ] **Step 4: Add the field to the contract**

In `api/openapi.yaml`, `ViewNode` gains `state_status`, required and nullable, written like its `reason_status` (same `$ref` or enum, same nullability). Run `make generate`. Run `go test ./internal/platform/api/ -run TestViewMatchesTheContract -count=1`; expected PASS.

- [ ] **Step 5: Write the failing web test**

In `autoscaling-nodes.test.tsx`, the mock rows carry `state_status`: the not-ready node `"attention"`, a ready and cordoned node `"attention"`, the unknown node `"unavailable"`, a ready node `null`. Assert the cordoned node's state cell reads `/^AttentionReady · Unschedulable$/` with the attention mark, and that a row whose `ready` is `false` but whose `state_status` is `null` draws no mark (the web follows the view, not the data). Run `cd web && npx vitest run src/features/report/autoscaling-nodes.test.tsx`; expected FAIL.

- [ ] **Step 6: Draw the State column from the view**

In `nodes-tab.tsx`, `NodeState` draws `<StatusMark status={row.state_status} label={stateText(row, t)} withStatus />` when `row.state_status` is not null, and the plain text otherwise; it no longer reads `row.ready` to choose a mark. Run the web tests and `make lint`; expected PASS.

- [ ] **Step 7: Retire the exception from the records**

In `AGENTS.md` (the bullet "A tint follows a status the view gives"), replace "The one exception is the State column of the nodes, which marks Not ready as attention and Unknown as unavailable though the view gives no status for readiness, an owner decision of 2026-10-11 (option 1A)." with "The State column of the nodes takes the status the view gives the node's state: the severity of its readiness findings, or unavailable when readiness is unknown." In `DESIGN.md:235`, replace the sentence of the exception with the same rule in the file's voice.

- [ ] **Step 8: Commit**

```bash
git add internal/document/nodepools.go internal/document/nodepools_test.go internal/document/view.go internal/document/view_test.go api/openapi.yaml web/src/api/schema.gen.ts web/src/features/report/nodes-tab.tsx web/src/features/report/autoscaling-nodes.test.tsx AGENTS.md DESIGN.md
git commit -m "feat(view): give the node state its status"
```

- [ ] **Step 9 (main thread):** `make update-testdata`; the diff adds `state_status` to every node of the golden views and marks `pool-a-2` attention, `orphan` unavailable; the PDF data of `edge-scaling` marks `pool-a-2`'s state from its findings; commit `test(testdata): node state status`.

---

### Task 8: An HPA's situation carries its worst finding (F-178, W8 A)

Reproduced at `382fc95` on the golden view `web/tests/fixtures/view/edge-scaling.en.json`: `payments/idle` reads "Below the maximum" with `situation_status` `healthy` while the view's findings hold `HPA_REPLICA_DIVERGENCE` of attention for `payments/idle`.

**Files:**
- Modify: `internal/document/hpas.go:181-202` (`hpaSituation`, `hpaSituationText`) and their callers in `hpas.go` and `executive.go`, `internal/document/view.go:617-634` (`hpas`)
- Test: `internal/document/hpas_test.go`, `internal/document/view_test.go`

**Interfaces:**
- Produces: `hpaSituationText(b *builder, h snapshot.HPA, findings []snapshot.Finding) (status, text string)`; `hpaSituation(b, h, findings) Run`. The text is the situation as today; the status is the worse of the situation's status and the HPA's worst finding, where an unavailable or healthy situation yields to an attention or critical finding.

- [ ] **Step 1: Write the failing test on the golden snapshot**

In `view_test.go`, load `testdata/snapshot/edge-scaling.json` the way the file's other golden-snapshot tests do, build the English view, and assert that the HPA `payments/idle` has `Situation` "Below the maximum" and `SituationStatus` `attention`, and that `payments/web` keeps "Maximum reached" and `critical`. In `hpas_test.go`, a table over `hpaSituationText`:

| Situation | Worst finding of the HPA | want status |
|---|---|---|
| below the maximum (healthy) | none | healthy |
| below the maximum (healthy) | `HPA_REPLICA_DIVERGENCE` (attention) | attention |
| pinned (healthy) | `HPA_PINNED_REPLICAS` (healthy) | healthy |
| maximum reached (critical) | `HPA_REPLICA_DIVERGENCE` (attention) | critical |
| unavailable | `HPA_REPLICA_DIVERGENCE` (attention) | attention |
| unavailable | none | unavailable |

The finding's scope is `analysis.ScopeHPA` with the name `namespace/name`; a finding of another HPA of the same name in another namespace must not count (one more row).

- [ ] **Step 2: Run them to see them fail**

Run: `go test ./internal/document/ -run 'HPASituation|Idle' -count=1 -v`
Expected: FAIL, `payments/idle` reads healthy.

- [ ] **Step 3: Read the findings**

```go
// hpaSituationText is the status and the text of hpaSituation: the text
// says where the replicas stood against the maximum; the status is the
// worse of that situation and the HPA's worst finding, an unavailable or
// healthy situation yielding to a finding of attention or critical.
func hpaSituationText(b *builder, h snapshot.HPA, findings []snapshot.Finding) (status, text string) {
	status, text = hpaReplicaSituation(b, h)
	worst := scopeSeverity(findings, analysis.ScopeHPA, h.Namespace+"/"+h.Name, "")
	if worst != analysis.SeverityHealthy && (status == "unavailable" || analysis.Rank(worst) > analysis.Rank(status)) {
		status = worst
	}
	return status, text
}
```

where `hpaReplicaSituation` is today's body of `hpaSituationText`, unchanged. Pass the snapshot's findings from every caller (`grep -n 'hpaSituation' internal/document/*.go`).

- [ ] **Step 4: Run the tests**

Run: `go test ./internal/document/ -count=1`
Expected: PASS except the golden-file comparisons: every scenario whose HPA holds a finding worse than its situation moves (`payments/idle` of `edge-scaling`; every `payments/payments-web` whose situation is below its divergence).

- [ ] **Step 5: Commit**

```bash
git add internal/document/hpas.go internal/document/hpas_test.go internal/document/view.go internal/document/view_test.go <executive.go if a caller>
git commit -m "fix(document): mark an HPA with its worst finding"
```

- [ ] **Step 6 (main thread):** `make update-testdata`; read every changed situation mark and name its finding; commit `test(testdata): HPA situation status`.

---

### Task 9: The catalog's wording (F-207, W3; F-201, W4, W5; F-177, W6)

**Files:**
- Modify: `internal/catalog/locales/en.toml:324-331,442`, `pt-BR.toml:324-331,442`, `en.toml:64`, `pt-BR.toml:64`
- Modify: `internal/catalog/keys.go:72` (`AppendixNoRemark` removed), `internal/catalog/guard_test.go` (the wording tables)
- Modify: `internal/document/appendix.go:80-125` (`appendixRemark` and the appendix row), `internal/document/executive.go:611-620`, `internal/document/view.go:463-471`
- Modify: `web/src/features/report/findings-tab.tsx:93-96`
- Test: `internal/document/appendix_test.go`, `internal/document/view_test.go`, `web/src/features/report/findings-tab.test.tsx`

**Interfaces:**
- Produces: `appendixRemark(b, reasons) string` returns `""` for no reasons; the PDF appendix and limitations draw an empty cell (`[]Run{}`, serialized as `[]`) for it; `ViewCapability.Remark` is `""`. `ViewFinding.Action` is `""` for a healthy finding. The web hides the Action section of a finding whose action is `""`.

- [ ] **Step 1: Write the failing tests**

In `guard_test.go`, change the verbatim rows to the new wording:
- `resource_limit_usage.cpu` summary: en "CPU usage against the limit.", pt-BR "Uso de CPU em relação ao limit."
- `resource_limit_usage.memory` summary: en "Memory usage against the limit.", pt-BR "Uso de memória em relação ao limit."
- `autoscaling_near_max` summary: en "The number of nodes reached or came close to the observed maximum.", pt-BR "O número de nodes atingiu ou ficou próximo do máximo observado."
- the rows of `appendix.no_remark` removed.

In `appendix_test.go`: a capability with no reason gives an empty cell in the appendix row and `""` from `appendixRemark`. In `view_test.go`: a healthy finding's `Action` is `""`, an attention finding's is its catalog action, and a capability with no reason has `Remark` `""`. In `findings-tab.test.tsx`: a healthy finding with `action: ""` opens with no "Action" heading; an attention finding shows it.

- [ ] **Step 2: Run them to see them fail**

Run: `go test ./internal/catalog/ ./internal/document/ -count=1` and `cd web && npx vitest run src/features/report/findings-tab.test.tsx`
Expected: FAIL on every changed row.

- [ ] **Step 3: Write the wording**

`en.toml`:

```toml
[finding.resource_limit_usage.cpu]
summary = "CPU usage against the limit."
...
[finding.resource_limit_usage.memory]
summary = "Memory usage against the limit."
...
[finding.autoscaling_near_max]
summary = "The number of nodes reached or came close to the observed maximum."
```

`pt-BR.toml`:

```toml
[finding.resource_limit_usage.cpu]
summary = "Uso de CPU em relação ao limit."
...
[finding.resource_limit_usage.memory]
summary = "Uso de memória em relação ao limit."
...
[finding.autoscaling_near_max]
summary = "O número de nodes atingiu ou ficou próximo do máximo observado."
```

The actions of these families stay. Remove `no_remark` from `[appendix]` in both files and `AppendixNoRemark` from `keys.go`.

- [ ] **Step 4: Write the empty remark and the healthy action**

`appendixRemark` returns `""` when `reasons` is empty; its comment says so. The appendix row (`appendix.go:86`) and the limitations row (`executive.go:615`) use a cell helper:

```go
// remarkCell is the remark of a capability as a table cell: empty when the
// capability has no reason.
func remarkCell(b *builder, reasons []string) []Run {
	if len(reasons) == 0 {
		return []Run{}
	}
	return []Run{Text(appendixRemark(b, reasons))}
}
```

In `view.go` (the findings, `:463-471`), the action is `""` when `f.Severity == analysis.SeverityHealthy`, else the catalog action as today. Confirm that `ViewFinding.action` and `ViewCapability.remark` in `api/openapi.yaml` carry no `minLength`; if one does, remove it and run `make generate`. In `findings-tab.tsx`, render the Action section only when `finding.action !== ""`.

- [ ] **Step 5: Run the tests**

Run: `go test ./internal/... -count=1`, `go test -count=1 -tags=template ./template/`, `cd web && npx vitest run`, `make lint`
Expected: PASS except the golden-file comparisons.

- [ ] **Step 6: Commit**

```bash
git add internal/catalog/locales/en.toml internal/catalog/locales/pt-BR.toml internal/catalog/keys.go internal/catalog/guard_test.go internal/document/appendix.go internal/document/appendix_test.go internal/document/executive.go internal/document/view.go internal/document/view_test.go web/src/features/report/findings-tab.tsx web/src/features/report/findings-tab.test.tsx <api and generated files if changed>
git commit -m "feat(catalog): neutral limit and plain remarks"
```

- [ ] **Step 7 (main thread):** `make update-testdata`; read every changed sentence, remark, and action; commit `test(testdata): report wording`.

---

### Task 10 (main thread): Golden pages, the look, and the records

- [ ] **Step 1: Golden pages.** Run `make update-golden`. Open every changed page and name its cause from Tasks 1 to 9 (unit space, mark and label, ISO dates, partial node utilization, readiness findings, node state marks, HPA marks, wording, empty remarks). A page that changed for no named cause is a defect. Update the `Protects` of each snapshot whose protected behaviour changed (for example `command-scaling-pt`: "a node whose state is unavailable twice" now sits beside a node marked attention by its readiness findings). Run `make test-golden`; expected PASS. Commit `test(golden): report truth pages`.
- [ ] **Step 2: The look.** Show the owner, as real screenshots on one page: the changed PDF pages before and after (the appendix of `capacity-pt`, the node and HPA tables and the appendix of `command-scaling-pt` and `-en`, a workload table of `command-complete-en`), and the web's nodes, autoscaling, findings, and quality tabs of `edge-scaling` in both themes at 1366×768. Small deviations from the rulings are ruled on the main thread and listed.
- [ ] **Step 3: Records.** `CHANGELOG.md` under `## [Unreleased]`:
  - `### Added`: "A node that was not ready, or was cordoned, at the end of the period now has a finding of attention, and the State column marks it." / "The report view marks the state of each node."
  - `### Changed`: "An HPA's situation carries the mark of its worst finding." / "Node utilization reads Partial while the node capacity evidence is incomplete." / "The limit findings describe the usage against the limit, with no risk predicted beside a healthy mark." / "The node autoscaling finding says when the cluster reached its maximum." / "Every date of the report is written as an ISO date and time." / "A healthy finding shows no action, and a capability with no remark shows an empty cell." / "CPU variation is judged only from an average of 1 mCPU."
  - `### Fixed`: "A number and its unit no longer split across two lines." / "A status mark no longer sits on a line above its label."
  State the omissions in the closing report: the decision record, the comparison's table, and the tests are left out as internal.
- [ ] **Step 4: ROADMAP.** Mark F-199, F-178, F-207, F-106, F-200, F-201, F-107, F-12 DONE with their commits, and F-177 PARTIAL (the No remark part done); record every finding of the phase.
- [ ] **Step 5: Gate and review.** Run `make gate` and read its summary line; run the adversarial whole-branch review over a diff file with one fix wave; read ROADMAP and CHANGELOG back; merge into `main` by fast-forward on the owner's word, with the version 1.5.0 and its tag only on the owner's word.
