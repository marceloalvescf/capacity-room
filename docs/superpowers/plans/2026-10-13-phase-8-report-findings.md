# Phase 8: Report Findings Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Close the four open findings of phase 7: one name for an unknown node pool (F-225), an action scope that names its kind once (F-227), status marks a screen reader skips for the word beside them (F-222), and an overview that draws a verdict the view gives instead of computing it (F-221).

**Architecture:** Go decides, the template and the web draw. F-225 is a wording change of the web's locale files. F-227 drops the kind word where the scope summary already names it, in Go and on the web. F-222 marks every status glyph of the PDF as a PDF artifact, so the page keeps its pixels and a screen reader reads the word that stands beside the mark. F-221 splits the verdict of `internal/document` into a decision layer (indices, codes, counts) and the PDF's wording layer; the view gains a `verdict` field from the decision layer, `ViewRecommendation` gains the index of its finding, and the web overview drops its own copies of the rules.

**Tech Stack:** Go 1.27, Typst 0.15.1, React and TypeScript (Vitest), OpenAPI 3 with oapi-codegen and openapi-typescript.

**Spec:** `docs/superpowers/specs/2026-09-27-k8s-capacity-report-design.md` (sections 11.10 and 12), `AGENTS.md`, and the owner decisions of 2026-10-13 in `ROADMAP.md` (phase 8, F-221, F-222, F-225, F-227).

## Global Constraints

- Branch `feat/report-findings`; nothing is edited on `main`.
- Go decides what is said; the template and the web decide how it looks. The web shows no number and applies no rule of the verdict that the view does not give.
- The PDF of every scenario stays byte-for-byte the same, except where a task names the change: no golden page changes in this phase, and `make test-golden` passes untouched.
- Every reader text comes from the catalog (Go) or the locale files (web); both locale files hold the same keys, and every key is used.
- An unknown node pool reads "Undetermined" in English and "Indeterminado" in Portuguese, on the PDF and on the web.
- The public API changes only by addition (a minor version): `ReportView` gains `verdict`, `ViewRecommendation` gains `finding`. No field is removed or changes its type.
- Every new View schema is excluded from the Go generation in `api/oapi-codegen.yaml`, and `TestViewMatchesTheContract` walks it both ways.
- No test skips, no focused test; an expected number in a test is a literal.
- A comment says what the code does; it names no task, step, finding, or phase.
- Subagents never run Docker; `make update-testdata`, `make test-golden`, and `make gate` run on the main thread.
- Commits name their paths (`git add <paths>`), conventional format, subject under 50 characters.

## Review Focus

1. **The indices of the verdict point into the view's own arrays.** `verdict.cluster_finding`, each `pressure[].finding`, and `recommendations[].finding` index `view.findings`; `verdict.node_evidence_warning` indexes `view.quality.warnings`. An executive report (minimum raised to attention) and a detailed report with minimum `all` must both give indices that land on the finding the PDF's verdict words. Pinned by the index test of Task 4.
2. **A report without autoscaling, without a maximum, or without a current count** has `verdict.gauge` null and the overview draws no gauge, as the PDF does. Pinned by Task 4 (Go) and Task 5 (web).
3. **The PDF does not move.** The split of `buildVerdict` must leave every data file and golden page as it was. Pinned by the unchanged `TestVerdict*` tests and `make test-golden` in Task 4.
4. **An action whose finding is filtered out or absent** keeps `finding: null` and the web shows it as plain text. Pinned by Task 4 (Go) and Task 5 (web).
5. **A plan scope of exactly three objects** still prints the kind and the three names; four or more print the summary alone. Pinned by Task 2 on both sides.

---

## File Structure

| File | Responsibility | Task |
|---|---|---|
| `web/src/locales/en.json`, `web/src/locales/pt-BR.json` | the word of an unknown pool | 1 |
| `web/src/features/report/*.test.tsx` | tests that name the unknown pool | 1 |
| `internal/document/executive.go` | the scope cell of an action row | 2 |
| `internal/document/executive_test.go` (or the test file that holds `actionRows` tests) | the scope cell with four objects | 2 |
| `web/src/features/report/overview-tab.tsx` | the plan's scope cell, then the overview from the verdict | 2, 5 |
| `template/capacity/runs.typ` | the status glyph as a PDF artifact | 3 |
| `template/artifact_test.go` (new, tag `template`) | the glyph is an artifact and draws the same pixels | 3 |
| `internal/document/verdict_facts.go` (new) | the decisions of the verdict: indices, codes, counts | 4 |
| `internal/document/verdict.go` | the PDF's words of those decisions | 4 |
| `internal/document/view.go` | `View.Verdict`, `ViewRecommendation.Finding` | 4 |
| `api/openapi.yaml`, `api/oapi-codegen.yaml` | `ViewVerdict`, `ViewGauge`, `ViewPressure`, `ViewHazard`, `finding` | 4 |
| `internal/platform/api/view_contract_test.go` | the new schemas, enum, and field refs | 4 |
| `web/tests/fixtures/view/*.json` | the golden views with the verdict | 4 |
| `web/src/api/schema.gen.ts` | generated | 4 |
| `web/src/features/report/cluster-verdict.tsx`, `depth-gauge.tsx`, `pieces.tsx`, `api.ts` | drawn from the verdict | 5 |
| `AGENTS.md`, `DESIGN.md`, `CHANGELOG.md` | records | 6 |

Tasks run in sequence: 2 and 5 share `overview-tab.tsx`, 1 and 5 share its test, and 4 feeds 5.

---

### Task 1: An unknown node pool reads "Undetermined" on the web (F-225)

**Files:**
- Modify: `web/src/locales/en.json:448`, `web/src/locales/pt-BR.json:448`
- Test: `web/src/features/report/overview-tab.test.tsx:178`, `web/src/features/report/autoscaling-nodes.test.tsx:135,318`, `web/src/features/report/findings-tab.test.tsx:161,287-293`

**Interfaces:**
- Consumes: the key `report.node.unknownPool`, used by `overview-tab.tsx:119`, `findings-tab.tsx:45,72,130`, `nodes-tab.tsx:105,138,152`. The key keeps its name.
- Produces: nothing new.

- [ ] **Step 1: Change the tests to the new word**

In every test listed above, replace the string `"Unknown pool"` with `"Undetermined"` (the search text typed in `findings-tab.test.tsx:161` included). Add one Portuguese assertion to `overview-tab.test.tsx` beside the English unknown-pool test, rendering the `edge-scaling.pt-BR.json` view the way the file's other pt-BR test does, and expecting the pressure line of node pools to show `Indeterminado`.

- [ ] **Step 2: Run the tests to see them fail**

Run: `cd web && npx vitest run src/features/report/overview-tab.test.tsx src/features/report/autoscaling-nodes.test.tsx src/features/report/findings-tab.test.tsx`
Expected: FAIL, the screens still read "Unknown pool" and "Node pool desconhecido".

- [ ] **Step 3: Change the two locale values**

`web/src/locales/en.json:448`: `"unknownPool": "Undetermined",`
`web/src/locales/pt-BR.json:448`: `"unknownPool": "Indeterminado",`

- [ ] **Step 4: Run the tests to see them pass**

Run: `cd web && npx vitest run`
Expected: PASS, every file.

- [ ] **Step 5: Commit**

```bash
git add web/src/locales/en.json web/src/locales/pt-BR.json web/src/features/report/overview-tab.test.tsx web/src/features/report/autoscaling-nodes.test.tsx web/src/features/report/findings-tab.test.tsx
git commit -m "fix(web): name an unknown pool as the PDF does"
```

---

### Task 2: An action scope names its kind once (F-227)

**Files:**
- Modify: `internal/document/executive.go:565` (inside `actionRows`, executive.go:546-569)
- Modify: `web/src/features/report/overview-tab.tsx:342-347` (the scope cell of `Plan`)
- Test: the Go test file that covers `actionRows` or `recommendationScope` (find with `grep -rn 'recommendationScope\|actionRows' internal/document/*_test.go`; add to it), and `web/src/features/report/overview-tab.test.tsx`

**Interfaces:**
- Consumes: `recommendationScope(b *builder, r snapshot.Recommendation) Run` (executive.go:574), unchanged.
- Produces: nothing new.

Today a row writes `catalog.Scope(r.ScopeType) + " "` and then `recommendationScope`, which for more than three scopes is a summary that already names the kind ("13 workloads identified"), so the cell reads "Workload 13 workloads identified". With more than three scopes the cell is the summary alone; with three or fewer it stays the kind word and the names.

- [ ] **Step 1: Write the failing Go test**

Build a snapshot from the `edge-scaling` fixture (the same helper the file's other tests use), give one recommendation four workload scopes (e.g. `ns/a`, `ns/b`, `ns/c`, `ns/d`) and another exactly three, build the English document, and find the action rows. Assert:
- the row with four scopes has a scope cell of exactly one run, a placeholder whose text is `4 workloads identified`;
- the row with three scopes has a scope cell of two runs: text `Workload ` and ident `ns/a, ns/b, ns/c`.

Check the kind word of the catalog (`[scope] workload` in `internal/catalog/locales/en.toml:515-521`) and write the literal it holds.

- [ ] **Step 2: Run it to see it fail**

Run: `go test ./internal/document/ -run <the new test's name> -v`
Expected: FAIL, the four-scope cell has two runs, `Workload ` and the summary.

- [ ] **Step 3: Write the Go change**

In `actionRows`, replace the scope cell with:

```go
			scopeCell(b, r),
```

and add, beside `recommendationScope`:

```go
// scopeCell writes the scope of an action: the word of its kind and the
// names, or, with more than three scopes, the summary alone, which names
// the kind itself.
func scopeCell(b *builder, r snapshot.Recommendation) []Run {
	if len(r.Scopes) > 3 {
		return []Run{recommendationScope(b, r)}
	}
	return []Run{Text(b.text(catalog.Scope(r.ScopeType), nil) + " "), recommendationScope(b, r)}
}
```

- [ ] **Step 4: Run the Go tests**

Run: `go test ./internal/document/ ./internal/goldentest/ ./internal/catalog/`
Expected: PASS. No golden data file holds a summary (checked: no fixture under `template/tests/capacity/fixtures/` or `testdata/` has one), so nothing else moves.

- [ ] **Step 5: Write the failing web test**

In `overview-tab.test.tsx`, take the `edge-scaling.en.json` view, replace its first recommendation's `scopes` with four names and its `scope_text` with `"4 workloads identified"`, render the overview, and assert that the scope cell of that row reads exactly `4 workloads identified` (no `Workload` word before it). Keep the existing test that pins the muted kind word and the mono names for three or fewer.

- [ ] **Step 6: Run it to see it fail**

Run: `cd web && npx vitest run src/features/report/overview-tab.test.tsx`
Expected: FAIL, the cell reads `Workload 4 workloads identified`.

- [ ] **Step 7: Write the web change**

In `Plan`, the scope cell becomes:

```tsx
                  <td className="px-3 py-2">
                    {recommendation.scopes.length <= 3 ? (
                      <>
                        <span className="text-muted-fg">{t(`scope.${recommendation.scope_type}`)}</span>{" "}
                        <span className="font-mono [overflow-wrap:anywhere]">{scopeLabel(recommendation)}</span>
                      </>
                    ) : (
                      scopeLabel(recommendation)
                    )}
                  </td>
```

- [ ] **Step 8: Run the tests**

Run: `cd web && npx vitest run && npx tsc --noEmit && npx eslint --max-warnings 0 .`
Expected: PASS.

- [ ] **Step 9: Commit**

```bash
git add internal/document/executive.go internal/document/<test file> web/src/features/report/overview-tab.tsx web/src/features/report/overview-tab.test.tsx
git commit -m "fix(report): name an action scope's kind once"
```

---

### Task 3: A status mark of the PDF is an artifact (F-222)

**Files:**
- Modify: `template/capacity/runs.typ:21-26` (`status-glyph`)
- Create: `template/artifact_test.go` (build tag `template`)

**Interfaces:**
- Consumes: the Typst element `pdf.artifact` of Typst 0.15.1 (measured: wrapping a boxed glyph in it leaves the 144 PPI PNG byte-identical and adds `/Artifact` marked content to the page stream).
- Produces: nothing for later tasks.

Every status glyph stands beside its word, except the legend's Status column, whose word is in the Meaning column of the same row. As an artifact the glyph is decoration: a screen reader reads the word and skips the shape. The data file and its contract do not change.

- [ ] **Step 1: Write the failing test**

`template/artifact_test.go`, tag `//go:build template`, package of the template tests. It compiles two minimal documents with the toolchain the other template tests use (`typst.Locate(os.Getenv, exec.LookPath)`, the same root, font path, and flags as `harness_test.go`): one that imports `capacity/runs.typ` and draws one status run `(kind: "status", status: "critical", text: none)` on an otherwise empty page, and one that draws a text run `(kind: "text", text: "x")` instead. For each PDF it inflates every `FlateDecode` stream (`compress/zlib` over the bytes between `stream\r?\n` and `endstream`) and counts the occurrences of `/Artifact BMC` and `/Artifact <<` (marked-content openings). Assert that the status document has exactly one more artifact opening than the text document. Compile both again to PNG at 144 PPI and assert nothing about them here; the golden pages pin the pixels.

- [ ] **Step 2: Run it to see it fail**

Run: `go test -count=1 -tags=template ./template/ -run TestStatusGlyphIsAnArtifact -v`
Expected: FAIL, the two counts are equal.

- [ ] **Step 3: Write the change**

```typst
// status-glyph sets the mark of a status in the monospaced face, which
// holds all five glyphs; the face of the words holds none of them. The mark
// is an artifact of the PDF: the word beside it, in its run or in the next
// cell of the legend, is what a screen reader reads.
#let status-glyph(status) = pdf.artifact(text(
  font: mono,
  fill: status-fill(status),
  weight: if status == "critical" { "bold" } else { "regular" },
  status-symbols.at(status),
))
```

- [ ] **Step 4: Run the template tests**

Run: `go test -count=1 -tags=template ./template/ ./internal/typst/ ./internal/cli/`
Expected: PASS.

- [ ] **Step 5: Main thread: prove the pixels did not move**

Run (main thread, Docker): `make test-golden`
Expected: PASS with no golden page changed. A failure means the artifact moved a pixel; stop and report, do not run `make update-golden`.

- [ ] **Step 6: Commit**

```bash
git add template/capacity/runs.typ template/artifact_test.go
git commit -m "fix(pdf): let screen readers skip status marks"
```

---

### Task 4: The view gives the verdict (F-221, Go and contract)

**Files:**
- Create: `internal/document/verdict_facts.go`
- Modify: `internal/document/verdict.go` (`buildVerdict`, `verdictGrowth`, `depthGauge`, `verdictPressure`, `pressureRow`)
- Modify: `internal/document/view.go` (`View`, `ViewRecommendation`, `buildView`, `vb.recommendations`)
- Modify: `api/openapi.yaml` (`ReportView` at 1880-1941, `ViewRecommendation` at 2195), `api/oapi-codegen.yaml` (`exclude-schemas`)
- Modify: `internal/platform/api/view_contract_test.go` (`TestViewEnumsEqualTheRegistries` at 306, `TestViewFieldsTakeTheirEnums` at 382)
- Test: `internal/document/view_test.go`, `internal/document/verdict_test.go` (unchanged; must still pass)
- Regenerate: `web/src/api/schema.gen.ts` (`make generate`), `web/tests/fixtures/view/*.json` (`make update-testdata`, main thread)

**Interfaces:**
- Consumes: `worstClusterFinding`, `keyOf`, `scopeKey`, `verdictScopeTypes`, `planRecommendations`, `executiveMinimum`, `minimumRank` (all in `internal/document`).
- Produces, Go:

```go
// verdictFacts are the decisions of the verdict, before any word: the
// indices point into the findings at or above the minimum and into the
// warnings of the data.
type verdictFacts struct {
	ClusterFinding  int // -1 when the cluster has no finding
	EvidenceWarning int // -1 when the evidence of the nodes is complete
	Gauge           *gaugeFacts
	Pressure        []pressureFacts
}

type gaugeFacts struct {
	Current, Max int
	Min          *int
	Hazard       string // "", "attention", or "critical"
}

type pressureFacts struct {
	ScopeType, Severity string
	Lead                int // index of the first finding of Severity in the type
	Critical, Attention int // rows that reach each severity
}

func shownFindings(s snapshot.Snapshot, severity string) []snapshot.Finding
func decideVerdict(s snapshot.Snapshot, findings []snapshot.Finding) verdictFacts
```

- Produces, JSON (the view):

```json
"verdict": {
  "cluster_finding": 0,
  "node_evidence_warning": null,
  "gauge": { "current": 3, "max": 5, "min": 1, "hazard": "critical" },
  "pressure": [
    { "scope_type": "node", "severity": "critical", "finding": 2, "critical": 1, "attention": 2 }
  ]
}
```

and `"finding": 4` (or `null`) on each recommendation. Go structs:

```go
type ViewVerdict struct {
	ClusterFinding      *int           `json:"cluster_finding"`
	NodeEvidenceWarning *int           `json:"node_evidence_warning"`
	Gauge               *ViewGauge     `json:"gauge"`
	Pressure            []ViewPressure `json:"pressure"`
}

type ViewGauge struct {
	Current int     `json:"current"`
	Max     int     `json:"max"`
	Min     *int    `json:"min"`
	Hazard  *string `json:"hazard"`
}

type ViewPressure struct {
	ScopeType string `json:"scope_type"`
	Severity  string `json:"severity"`
	Finding   int    `json:"finding"`
	Critical  int    `json:"critical"`
	Attention int    `json:"attention"`
}
```

`View` gains `Verdict ViewVerdict \`json:"verdict"\`` after `Recommendations`; `ViewRecommendation` gains `Finding *int \`json:"finding"\`` after `ScopeText`.

The decisions stay where they are and move into `decideVerdict` unchanged:
- `ClusterFinding`: the index of `worstClusterFinding(findings)` in `findings` (return the index instead of a pointer, or find it).
- `EvidenceWarning`: the index of the first warning with code `collect.WarningNodeCapacityEvidenceIncomplete` in `s.Quality.Warnings`.
- `Gauge`: only when `Autoscaling.Status` is enabled, current and max are present, and max > 0; the rounding, clamping, and minimum rule of `depthGauge`; `Hazard` is the cluster finding's severity when it is above healthy, else "".
- `Pressure`: the rows of `pressureRow` in the order of `verdictPressure`, with `Lead` the index into `findings` of the lead finding and the counts as integers.

`buildVerdict` calls `shownFindings` and `decideVerdict` once and words from the facts: `VerdictGauge.Hazard` is `facts.Gauge.Hazard != ""`; the pressure row's sentence, name, and peak come from `findings[row.Lead]`. Nothing the PDF prints changes.

The finding of an action is the first finding in the view's findings whose code, scope type, and scope name equal the recommendation's code, scope type, and first scope; `nil` when there is none or the action has no scope. This is the rule of the web's `matchRecommendation` (`web/src/features/report/pieces.tsx:258`); read it and carry it to Go exactly, then name it `recommendationFinding(r snapshot.Recommendation, findings []snapshot.Finding) *int`.

`buildView` computes `findings := shownFindings(s, severity)` once, keeps `vb.findings` over it, and fills `Verdict` from `decideVerdict(s, findings)`. The view's findings are exactly that list, so every index lands on its finding.

- [ ] **Step 1: Write the failing view tests**

In `view_test.go`, add:
- `TestViewVerdictPointsAtWhatThePDFWords`: for every golden snapshot (`testdata/snapshot/*.json`, the same loop as `TestViewSaysWhatThePDFSays`), in English, build the view and the document. Assert that `view.Findings[*view.Verdict.ClusterFinding].Sentence` equals the PDF verdict's `Growth.Finding.Sentence` (both nil together); that `len(view.Verdict.Pressure)` equals `len(pdf.Pressure.Rows)` and, row by row, `view.Findings[p.Finding].Sentence` equals the row's sentence, `p.Severity` equals the row's status, and the counts equal the numbers of the row's `Counts`; that the gauge is nil together with the PDF's gauge, and otherwise current, max, and min are equal and `Hazard != nil` equals the PDF's `Hazard`; that `NodeEvidenceWarning` points at a warning whose text equals the PDF's `Growth.Warning`.
- `TestViewVerdictOfEdgeScaling`: the literal verdict of `edge-scaling` in English, written out field by field from the PDF's `TestVerdictOfEdgeScalingGrowth` and `TestVerdictOfEdgeScalingPressure` values (read them; copy the numbers as literals).
- `TestViewVerdictOfTheExecutiveProfile`: the snapshot of `TestVerdictOfTheExecutiveProfileRaisesTheMinimum`, through `BuildView`: every index lands inside `view.Findings` and none points at a healthy finding.
- `TestViewVerdictWithoutAutoscaling`: the snapshot of `TestVerdictWithoutAutoscaling`: `Gauge` is nil.
- `TestViewRecommendationFinding`: in `edge-scaling`, each recommendation's `Finding` is nil or points at a finding with the same code, scope type, and the first scope as name; one recommendation whose finding is absent from the view gives nil.

Extend `TestViewListsAreNeverNull` to `view.Verdict.Pressure`.

- [ ] **Step 2: Run them to see them fail**

Run: `go test ./internal/document/ -run 'TestViewVerdict|TestViewRecommendationFinding|TestViewListsAreNeverNull' -v`
Expected: FAIL to compile (`view.Verdict` undefined).

- [ ] **Step 3: Write `verdict_facts.go` and the view fields**

Move the decisions into `decideVerdict` as described, change `buildVerdict` to word from the facts, add the view structs and fields, and fill them in `buildView` and `vb.recommendations` (pass the shown findings to it).

- [ ] **Step 4: Run the document tests**

Run: `go test ./internal/document/ ./internal/goldentest/`
Expected: every new test passes; every `TestVerdict*` test passes unchanged; `TestGoldenViewFiles` FAILS (the golden views lack `verdict` and `finding`). Nothing else fails. A failure of a data-file or PDF comparison means the PDF moved: fix it before going on.

- [ ] **Step 5: Write the contract**

In `api/openapi.yaml`:
- `ReportView`: add `verdict` to `properties` and `required`, `$ref: '#/components/schemas/ViewVerdict'`.
- `ViewRecommendation`: add `finding` to `properties` and `required`, `type: [integer, "null"]` written the way the file writes its other nullable integers, with a description: "Index in findings of the finding the action comes from; null when none is shown."
- `ViewVerdict`: object, `additionalProperties: false`, required `cluster_finding`, `node_evidence_warning`, `gauge`, `pressure`. `cluster_finding`: nullable integer, "Index in findings of the worst finding of the cluster." `node_evidence_warning`: nullable integer, "Index in quality.warnings of the warning that the evidence of the nodes is incomplete." `gauge`: `ViewGauge` or null. `pressure`: array of `ViewPressure`, "The scope types under pressure, the worst first."
- `ViewGauge`: object, `additionalProperties: false`, required `current`, `max`, `min`, `hazard`; integers, `min` nullable, `hazard` `$ref: '#/components/schemas/ViewHazard'`.
- `ViewHazard`: the nullable enum `critical`, `attention`, `null`, written as `ViewMark` writes its null.
- `ViewPressure`: object, `additionalProperties: false`, required `scope_type` (`ViewScopeType`), `severity` (`ViewSeverity`), `finding`, `critical`, `attention` (integers).

Copy the style of the neighbouring schemas (descriptions, `additionalProperties`, how a nullable `$ref` is written). Add `ViewVerdict`, `ViewGauge`, `ViewPressure`, and `ViewHazard` to `exclude-schemas` in `api/oapi-codegen.yaml`.

In `view_contract_test.go`: add `ViewHazard` to `TestViewEnumsEqualTheRegistries` against a Go registry `document.GaugeHazards = []string{analysis.SeverityCritical, analysis.SeverityAttention}` (declare it in `verdict_facts.go` and use it in `decideVerdict`), and add `ViewPressure.scope_type → ViewScopeType`, `ViewPressure.severity → ViewSeverity`, `ViewGauge.hazard → ViewHazard` to `TestViewFieldsTakeTheirEnums`.

- [ ] **Step 6: Generate and run the contract tests**

Run: `make generate && make generate-check && go test ./internal/platform/api/ -run 'TestView|TestWalker' -v`
Expected: PASS; `TestViewMatchesTheContract` walks the verdict of every snapshot in both languages.

- [ ] **Step 7: Main thread: regenerate the golden views**

Run (main thread): `make update-testdata`, then read every changed file whole. Expected changes: the four files of `web/tests/fixtures/view/` gain `verdict` and `finding`, and nothing else in `testdata/` or `template/tests/capacity/fixtures/` changes. Check by hand in `edge-scaling.en.json` that each index lands on the finding the PDF's verdict page words.

- [ ] **Step 8: Run the suite**

Run: `make test && make lint`
Expected: PASS. The web compiles against the new type without using it yet.

- [ ] **Step 9: Main thread: prove the PDF did not move**

Run (main thread, Docker): `make test-golden`
Expected: PASS, no golden page changed.

- [ ] **Step 10: Commit**

```bash
git add internal/document/verdict_facts.go internal/document/verdict.go internal/document/view.go internal/document/view_test.go api/openapi.yaml api/oapi-codegen.yaml internal/platform/api/view_contract_test.go web/src/api/schema.gen.ts web/tests/fixtures/view/
git commit -m "feat(api): give the overview's verdict in the view"
```

---

### Task 5: The overview draws the verdict (F-221, web)

**Files:**
- Modify: `web/src/features/report/overview-tab.tsx` (`Growth`, `summarise`, `Pressure`, `PressureLine`, `PlanAction`, the helpers `worstOf`, `rank`, `scopeTypes`)
- Modify: `web/src/features/report/cluster-verdict.tsx` (drop `worstClusterFinding`)
- Modify: `web/src/features/report/depth-gauge.tsx` (props from `ViewGauge`)
- Modify: `web/src/features/report/pieces.tsx` (drop `matchRecommendation` if nothing else uses it)
- Modify: `web/src/features/report/api.ts` (re-export `ViewVerdict`, `ViewGauge`, `ViewPressure`)
- Test: `overview-tab.test.tsx`, `depth-gauge.test.tsx`, `autoscaling-nodes.test.tsx`

**Interfaces:**
- Consumes: `view.verdict` and `recommendation.finding` from Task 4; types `Schemas["ViewVerdict"]`, `Schemas["ViewGauge"]`, `Schemas["ViewPressure"]`.
- Produces: `DepthGauge({ gauge }: { gauge: ViewGauge })`; `ClusterVerdict()` reads `verdict.cluster_finding` and `verdict.node_evidence_warning`.

What stays a web decision: the layout, the links (`../findings?scope=<type>&finding=<index>`, `../findings?severity=<s>` gated by `min_severity`), the bold worst total, the first three quality warnings with `+N`, and the full plan table (the PDF's page holds five; the screen holds all). What leaves: `worstClusterFinding`, `worstOf`, `summarise`, the scope order and the rank sort of the pressure, the gauge's rounding and minimum rule and its hazard choice, and `matchRecommendation`.

- [ ] **Step 1: Change the tests to the verdict**

- `depth-gauge.test.tsx`: render `DepthGauge` from `gauge` objects (`{ current, max, min, hazard }`, integers) instead of `current/max/min/severity`; keep each existing case's expectation, with `min` already rounded.
- `overview-tab.test.tsx`: add a test that edits the `edge-scaling.en.json` verdict (e.g. swaps the order of two pressure rows, or points `cluster_finding` at another finding) and asserts the overview follows the view, not its own rule: the lines come in the view's order, and the cluster sentence is the sentence of the finding the index names. Add a test where `verdict.gauge` is null while autoscaling is enabled: no gauge is drawn. Add a test where a recommendation's `finding` is null: its action is plain text; and where it is an index, the link carries that index.
- Keep every existing overview test; their expectations hold, since the golden views carry the same answers.

- [ ] **Step 2: Run them to see them fail**

Run: `cd web && npx vitest run src/features/report/`
Expected: FAIL on the new tests and on `depth-gauge.test.tsx`.

- [ ] **Step 3: Draw from the verdict**

- `Growth`: `const { gauge } = verdict;` and `{gauge === null ? null : <DepthGauge gauge={gauge} />}`; drop the raw-min and hazard computation.
- `DepthGauge`: takes `gauge: ViewGauge`; its level, max, min, and hazard colour come from the fields as given, with no rounding or clamping of its own.
- `ClusterVerdict`: the finding is `verdict.cluster_finding === null ? null : findings[verdict.cluster_finding]`; the warning is `verdict.node_evidence_warning === null ? null : quality.warnings[verdict.node_evidence_warning]`.
- `Pressure`: maps `verdict.pressure` in order; each line takes `findings[p.finding]` for its sentence, name (through `scopeName`), and peak, `p.severity` for its mark, `p.scope_type` for its word and link, and `p.critical` / `p.attention` for its counts (each shown only above zero).
- `PlanAction`: `recommendation.finding === null` gives plain text; otherwise the link uses `recommendation.finding`.
- Remove the helpers that no longer have a caller, and any locale key left unused (`src/i18n/locales.test.ts` names it).

- [ ] **Step 4: Run the web checks**

Run: `cd web && npx vitest run && npx tsc --noEmit && npx eslint --max-warnings 0 .`
Expected: PASS.

- [ ] **Step 5: Run the suite**

Run: `make test && make lint`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add web/src/features/report/ web/src/locales/en.json web/src/locales/pt-BR.json
git commit -m "refactor(web): draw the overview from the verdict"
```

---

### Task 6: Records and gate (main thread)

**Files:**
- Modify: `AGENTS.md` (the overview rule under "The interface"; the stale "where the PDF prints the short name"; the view rule naming `verdict` and `finding`; the unknown pool word)
- Modify: `DESIGN.md` (the plan's scope cell; the unknown pool word, if named)
- Modify: `CHANGELOG.md` (`[Unreleased]`)
- Modify: `ROADMAP.md` (untracked: P8-1 to P8-4, F-221, F-222, F-225, F-227)

- [ ] **Step 1: Update `AGENTS.md`.** The overview item says the verdict band and the pressure come from `view.verdict` and the plan's links from `recommendation.finding`; "where the PDF prints the short name" goes (both print the full name); the view item names the verdict as the decisions of the PDF's verdict page; the template item notes that a status glyph is a PDF artifact.
- [ ] **Step 2: Update `DESIGN.md`** where it describes the plan's scope cell (DESIGN.md:328) and the unknown pool.
- [ ] **Step 3: Write the changelog** under `[Unreleased]`:

```markdown
### Added

- The report view of the API gives the verdict of the overview: the worst finding of the cluster, the node gauge, the pressure on each scope, and the finding each action comes from.

### Changed

- The web app names a node pool it could not identify "Undetermined", as the PDF does.
- The node gauge of the web app marks the minimum in whole nodes, as the PDF does.

### Fixed

- An action that covers more than three scopes no longer repeats their kind, in the PDF and in the web app.
- Screen readers skip the status marks of the PDF and read the word beside them.
```

- [ ] **Step 4: Run the adversarial review** over `git diff main...feat/report-findings`, through the `adversarial-reviewer` agent with the diff in a file; record every finding in `ROADMAP.md`, fix what it confirms.
- [ ] **Step 5: Run `make gate`** (main thread) and read its result.
- [ ] **Step 6: Update `ROADMAP.md`:** P8-1 to P8-4 and the four findings `DONE` with their commits; the phase line with the gate's commit.
- [ ] **Step 7: Commit** `AGENTS.md`, `DESIGN.md`, `CHANGELOG.md` with `docs: record the report findings`.

The phase changes sentences of the report and adds to the API: by the changelog rules it is the minor version 1.3.0, released on the owner's word.
