# Phase 2.4: Executive Profile and Proofs Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** `k8s-capacity-report report` prints the executive summary as section 5 of the detailed report and as the body of the executive profile, stores the recommendations and their count in the snapshot, completing schema `1.0`, carries the sentences of the 66 finding codes in the catalog of both languages, and proves against the Python tool, on eight scenarios, the recommendations, the summary counts, the coverage of every row, and the rows of the seven executive tables.

**Architecture:** The last thread of phase 2 through the layers of spec 4.3: the analysis gains `Recommendations`, the snapshot gains `recommendations` and `summary.recommendations` and recomputes both when it reads a file, the catalog gains the finding sentences by family and resource and the wording of the executive body, the document gains the executive body (seven tables that the template already lays out) and the executive profile, and the command accepts that profile. Four findings of part 2.3 close on the way (F-83, F-85, F-88, F-92), and the run against a real cluster takes up Task 17 of part 2.1, F-35, and F-66.

**Tech Stack:** Go 1.27 and the modules already in `go.mod`. No new module. Typst 0.15.1, `pdftotext`, Docker. Python 3.14.7 in the environment of part 2.3, linked from `.cache/phase-2/part-4/python/`, on the main thread, to record the original again (Task 8).

**Spec:** `docs/superpowers/specs/2026-09-27-k8s-capacity-report-design.md`, sections 5.1 to 5.3, 7.3, 7.4, 8, 9.1 to 9.5, 10.2, 10.3, 13.2, 13.3, 15, 16, and 17. The decisions of the owner are in `ROADMAP.md`.

**Depends on:** Part 2.3, merged into `main` at `afaced6`.

## Where this part sits

| Part | Delivers | PDF at the end |
|---|---|---|
| 2.1 Spine, merged | Configuration, redaction, client, discovery, the fake server, the catalog mechanism, the four modes of `report` | Cover, opening page, quality appendix |
| 2.2 Namespaces and workloads, merged | Range queries, two collectors, workload identity, 40 codes, five formats, the comparison with the Python tool | Sections 1 and 2, interpretation notes |
| 2.2b The wording rule, merged | Technical terms in English in both languages; the guard that keeps it | The same pages |
| 2.3 Autoscalers and nodes, merged | Three collectors, two resolvers, 26 codes, `summary`, `hpas`, `nodes`, `nodepools`, coverage, the last queries | Sections 3 and 4 |
| **2.4 Executive profile and proofs (this plan)** | Recommendations, the finding sentences, the executive body as section 5, the executive profile, the complete catalog, the proofs against the original, a real cluster | Both profiles, six sections |

### Out of this part

| Left out | Arrives in | Reason |
|---|---|---|
| F-41, F-89: a snapshot read twice in two ways; contradictory rows `Build` never writes | 3 | They matter when snapshots are stored and served |
| F-93: `schema_version` unchanged while the shape changed | 5 | The first release fixes the schema (R29) |
| A change to the template | none | The seven tables of the executive body exist in `template/capacity/tables.typ:47-75` since phase 1, and the parity pages must stay equal to the original's |
| The web view of a finding's sentence | 4 | The sentences enter the catalog now; only the action plan and the node reasons print one |

## How to read this plan

The owner's orchestration rule applies: the plan states what each task must
deliver and how it is proven, and the implementer writes the code. Each task
gives exact paths, exact signatures, and a table of tests with inputs and
expected results. Write each test first, watch it fail, then implement.

Executors:

- **Subagent**: a Sonnet subagent. It may run `go`, `make build`, `make test`,
  and `make lint`, and the tagged tests with the native tools. It must not
  run Docker, must not run `make update-testdata` or `make update-golden`,
  and must not declare a task finished.
- **Main thread**: needs Docker, judgment, wording, page review, the
  recording environment, or the owner.

Rules for every subagent brief:

- Tasks run in sequence. Two agents never share the checkout.
- A reviewer receives a diff file under `.cache/phase-2/part-4/` and runs
  tests only on packages nobody is editing.
- Every commit names its paths: `git add <paths>`, never `git add -A`.
- A subagent never runs `git stash`, `git restore`, `git checkout` of a
  path, or `git clean`.
- A test of a change goes red on the code before the change, test by test,
  and the brief asks for the red run.
- A brief that says "report every X" also lists, as tests, the cases that
  must not match.
- A subagent has no `ReportFindings` tool. A reviewer writes its findings to
  a file under `.cache/phase-2/part-4/` and repeats them in its last message.
- A decision flagged at the end of a report has already been made. A brief
  that says "stop and report" means stop before deciding.
- When a brief lists the tests that fail after a change, it names every
  reader of the artefact that changes, found by grep, not the one test that
  owns it. `.cache/phase-2/part-4/research/go-interfaces.md` section 10
  holds the list of readers at `afaced6`.
- The main thread commits its own changes before it dispatches.

Reference material, read-only and never tracked:

- `older/kubernetes-report/kube_capacity_report/`: the Python source, the
  reference for every detail of behaviour that this plan does not repeat.
  Inside `older/`, nobody reads `.env`, `.envrc`, `report.toml`, anything
  under `deploy/secrets`, `.venv`, `.git`, `.worktrees`, `build`, or
  `node_modules`.
- `.cache/phase-2/part-4/research/original-executive.md` (the eight
  sections and seven tables of the executive body, with the rows the
  original gave on the eight scenarios), `original-recommendations.md`
  (`build_recommendations`, the summary counts, the coverage, the action
  plan), `original-sentences.md` (the 66 summaries and actions, the
  fragments, the observations, the small tables), and `go-interfaces.md`
  (the Go side at `afaced6`, with file and line): the facts extracted for
  this part. A brief quotes the lines it needs; an implementer that finds
  the brief and the original in disagreement stops and reports.
- `.cache/phase-2/part-4/python/out/<scenario>.json`: what the original
  computes for each scenario, including the executive rows at the two
  effective minimums, read on 2026-09-30. Task 8 records the same again
  into `testdata/python/`.

## Global Constraints

Every constraint of the plans of parts 2.1 to 2.3 stands. The ones below are
added or are the ones this part leans on most.

- Go 1.27. Module `github.com/marceloalvescf/k8s-capacity-report`. No new
  module enters `go.mod`. No new package; `guard.Layers` is not touched.
- Metrics only. Imports point one way. The snapshot is the only handover:
  everything after `snapshot.Build` reads the snapshot and nothing else.
- The queries of every scenario stay the original's whole, letter for
  letter; this part sends no new query.
- The numbers equal the original exactly: the same `float64`, no tolerance.
  An expected number in a test is a literal.
- The 66 finding codes are the strings of the original, unchanged. A
  finding carries no sentence; a recommendation carries no sentence.
- The snapshot holds codes and numbers. It holds no address, no host name,
  no provider identifier, and no token. A stored snapshot is never
  narrowed: the minimum severity and the profile are applied when a
  document is built, never when a snapshot is written.
- Every output list is sorted or in a stated order: the recommendations in
  the order of the original's analysis (R63).
- Wording lives only in `internal/catalog`. Every technical term is English
  in both languages; only a common noun is Portuguese (spec 9.3): request,
  limit, workload, node, node pool, replica, autoscaling, throttling are
  English words, masculine in Portuguese, with an English plural. A noun is
  never pasted into a sentence: one entry per resource. Kubernetes names
  (`Deployment`, `StatefulSet`, `HPA`, `OOMKill`) stay as they are.
- No em-dash inside a sentence. The em-dash alone marks an empty table
  cell, and it is a catalog entry (R67).
- Rounding follows the original: a replica count and a pod count are
  rounded half to even and printed whole; percentages one decimal; trailing
  zeros removed.
- The same input gives the same snapshot and the same `report.json`, byte
  for byte. `report.json` is strict in both directions: a field enters the
  contract with the feature that prints it; a table with no row is left
  out and its fallback sentence printed.
- The files of `testdata/prometheus/`, `testdata/prometheus-own/`, and
  `testdata/python/` are recorded answers. They are never edited; a file of
  `testdata/python/` is replaced only by recording the original again,
  whole (Task 8). The parity fixture and its pages do not change.
- `--check` sends no query of a collector and, after this part, reads the
  clock once (R68). A run that collects reads it twice. Every failure
  prints one line `error: <message>` on standard error.
- No test skips. A test never writes a golden file. No comment speaks of
  the process. Diagnostics are in English.
- Scratch files go under `.cache/phase-2/part-4/`, never under `/tmp`.
- Commits follow `type(scope): description`, imperative, subject under 50
  characters.

## Review Focus

Each line names an input that the spec implies and does not describe, and the
task whose tests pin it.

1. **Two workloads of one name and two kinds, both with findings.** Each is
   its own row of the priority tables, named `api (Deployment)` and
   `api (StatefulSet)` as one identifier each; a reader must tell them
   apart. Pinned in Task 6 (`TestPriorityRowsTellHomonymsApart`).
2. **An HPA at its maximum whose target the run could not tell.** The row
   names the HPA, then the placeholder of an unidentified target, and the
   kind cell reads unavailable; two such HPAs are two rows, in name order.
   Pinned in Task 6 (`TestReplicaCeilingUnidentifiedTarget`).
3. **A qualifier that depends on the included set.** At the minimum
   `critical`, an action that two rows shared at `attention` stands alone,
   so the qualifier disappears; the plan is built from the included rows
   and nothing else. Pinned in Task 6 (`TestActionPlanQualifiesOnlyIncludedCollisions`)
   and against the original in Task 8 (edge-scaling at the two minimums).
4. **A pool whose own findings are below a node's.** The pool row carries
   the severity and the observation of its node's finding only; the pool's
   own attention findings do not print beside a critical one. Pinned in
   Task 6 (`TestNodepoolStatusKeepsTheHighestSeverityOnly`) and Task 8
   (edge-scaling, `pool-a`).
5. **A workload finding on a row with no pod count.** A hand-written
   snapshot may name a coverage-only row in a resource finding, which
   `Check` accepts; the pods cell reads the em-dash alone and the row is
   otherwise whole. Pinned in Task 6 (`TestPriorityRowsWithoutPods`).

## Rulings made on the owner's behalf

Each of these is a decision the spec leaves open. Each is written as
decision, reason, and cost if wrong. The owner may overturn any of them when
reviewing this plan. The numbering continues that of part 2.3.

| ID | Ruling |
|---|---|
| R61 | Ruling: the detailed profile prints the executive body as its section 5, between section 4 and the appendix, with the minimum severity raised to at least `attention` for that section alone, and the appendix becomes section 6 — the original's `_detailed_body` does exactly this (R/sections.py:504-511) and the parity fixture of phase 1 already holds the blocks (`parity.pt-BR.json` blocks 23 to 38); the pages of `capacity-parity` do not change — every detailed PDF grows by the pages of section 5 |
| R62 | Ruling: the executive profile prints the cover, the opening page, the executive body with the minimum raised to at least `attention`, the appendix with the minimum as configured, and the interpretation notes; no section 1 to 4 — the original (R/typst.py:19-28) — none |
| R63 | Ruling: `analysis.Recommendations` groups the findings above healthy by code, in first-seen order, with the highest severity, the most exact confidence, the sorted distinct scope names, and the scope type of the first finding, and sorts them by severity descending, confidence from exact, first scope name, code; `snapshot.Build` derives them from the findings it was given, stores them in that order, and `Check` recomputes them from the stored findings and refuses a file whose list differs — the original groups by code and action text, and a code has one action; the original stores them in another order, `(scope_type, code, scopes)`, which this project does not keep, as R57 kept the analysis order of the findings; a stored derivation must agree with its source — a hand-written file must carry exact recommendations |
| R64 | Ruling: the sentences of the findings enter the catalog by family and resource, `finding.<family>.<resource>.summary` and `.action` for the nine families with a `resource` parameter and `finding.<family>.summary` and `.action` for the fifteen without; the six node and pool families that the executive pool table names also carry `.observation`, with one entry per resource where the family has one; `catalog.Keys()` derives the set from `analysis.Registry`, so `TestKeysEqualTheFiles` is the guard that ties the registry to the catalog in both directions, and a second test resolves every definition to its two or three entries — R21, spec 9.1; the two keys of part 2.3 keep their names — none known |
| R65 | Ruling: the qualifier of an action of the plan is added when two included rows share the rendered action text, as the original judges it; a guard refuses two pairs of family and resource whose actions are one text in one language, since the only intended sharing is between a NAMESPACE code and a WORKLOAD code of one family and resource, which share the entry — the reader sees texts, not keys — none while the guard holds |
| R66 | Ruling: the texts of the original that no input of this project can reach are not entered: `Tipo não informado`, the namespace `Indisponível` of a scope without a slash, `Escopo geral`, `escopos identificados`, `statefulsets identificados`, `clusters identificados`, `Baixa`, and the qualifiers and summaries of the node and nodepool scopes; the document returns an error naming the code or the scope type when it reaches one — an entry nothing prints cannot be reviewed on a page (R21); every workload finding names `namespace/name` of a row and carries a kind, every scope type is one of six, the node and nodepool codes never enter the plan, a healthy code is never recommended, and one cluster exists — a hand-written file that reaches one stops with an error, where the original printed a placeholder |
| R67 | Ruling: the em-dash alone is the entry `placeholder.empty_cell`, printed in the pods cell of a priority row whose workload row carries no pod count and in the observation of a stable pool — spec 9.3 names the em-dash alone as the mark of an empty cell, and the wording guard refuses it as a Go literal — none |
| R68 | Ruling: `--check` reads the clock once, for the emission time that fixes the instant of its queries; the wrapper that read it again goes (F-92) — the rule of `AGENTS.md` says a run that collects reads it twice, and a check collects nothing — none |
| R69 | Ruling: a threshold, in the configuration and in a snapshot, and the timeout, in the configuration, must be a finite number; `nan`, `inf`, and `-inf` are refused with a message that says so (F-88) — a comparison with `NaN` is always false, so the range checks let it through and `Encode` failed at the end of the run — none |
| R70 | Ruling: `InstanceHost` keeps the zone of an IPv6 address, the text after `%`, as it was written and lowers only the host before it, as `urlsplit` does; the lowering of a host stays Go's simple case mapping, which differs from Python's full mapping on `İ` (U+0130, Go `i`, Python `i` with U+0307) and on a final sigma (Go `σ`, Python `ς`), measured on 2026-09-30 and pinned by a test that names the difference; F-83 closes — an instance of Prometheus is `host:port` in ASCII, and a link-local address with a zone is a real input where the two Unicode cases are not — a node whose instance holds one of those two letters resolves by its address instead of its host, as it does today |
| R71 | Ruling: the default name of a PDF of the executive profile carries `-executive` in place of the severity suffix, as the original (K/application.py:55-63); R4 of part 2.1 otherwise stands — a script that expects a severity suffix on an executive report must change |
| R72 | Ruling: the executive rows are compared with the original on eight scenarios, at the two effective minimums, through the executive profile in Portuguese: the number of rows, the kind and status of every run, every identifier letter for letter, and every number token of a text; the words are not compared, because they follow the catalog, and the parity test of the document compares the words through the catalog on the fixture — a difference of wording is seen only on the fixture — none known |
| R73 | Ruling: two scenarios get executive data files in both languages, `direct-complete` and `edge-scaling`, named `<scenario>.executive.<lang>.json` beside the detailed ones; the executive body of every scenario is already pinned by its detailed data file, and the two files pin what the profile adds: no section 1 to 4, and the note of section 5 after the table when the table has rows (edge-scaling) and absent when the HPA data is whole (direct-complete) — four more golden files to keep |
| R74 | Ruling: the recorded files of the original gain `recommendations`, `summary`, `workload_coverage`, and `executive` (the rows of the seven tables at the two minimums, as runs), recorded whole again with the fake server built from this branch (R58); the mapper of the original needs `pydantic`, which the environment lacks, and a stand-in that fills the fields of a record from its keyword arguments and class defaults stands for it, as `record.py` says — R25 allowed one installation and the mapper only constructs records — every changed number is read before the commit |
| R75 | Ruling: `summary.recommendations` enters `Summary` after `findings`, as the original orders its keys (mapper_v1.py:329-344); the eight golden snapshots are regenerated — one more count to check — none |
| R76 | Ruling: the rows of the pool situation table sort by the displayed name of the pool, so the undetermined pool sorts by the word of the language, as the original sorts by `Indeterminado`; and the rows of the replica ceiling table sort by the displayed kind and name of the target, so an unidentified target sorts by the words of the language — the original sorts by the words it prints — the place of such a row may differ between the two languages |
| R77 | Ruling: the run against a real cluster (Task 11) needs a configuration that only the owner has; the owner gives its path and the name of the token variable with the approval of this plan, or says that the run waits; without it by the close, Task 17 of part 2.1, F-35, and F-66 stay deferred with the owner's word recorded — nothing of this part has run against a real endpoint — the proof of spec 16 on a real cluster waits again |

## Findings this plan closes or records

| Finding | How |
|---|---|
| F-83 | Closed by Task 3: the zone is kept; the two Unicode cases are pinned as Go's (R70) |
| F-85 | Closed by Task 8: the summary counts and the coverage of every row are compared with the original |
| F-88 | Closed by Task 3 (R69) |
| F-92 | Closed by Task 3 (R68) |
| F-35, F-66, Task 17 of part 2.1 | Taken up by Task 11, or deferred with the owner's word (R77) |
| F-90, F-91 | The owner's calls on the wording, handed over again at the close with the complete catalog |

## File structure

| Path | Responsibility in this part |
|---|---|
| `internal/analysis/recommendations.go`, `recommendations_test.go` | `Recommendation`, `Recommendations`, `ConfidenceRank` |
| `internal/snapshot/snapshot.go`, `build.go`, `codec.go` | `Recommendation`, `Snapshot.Recommendations`, `Summary.Recommendations`; the derivation in `Build`; `checkRecommendations`, the count in `checkSummary`, the finite thresholds |
| `internal/config/validate.go` | The finite checks (F-88) |
| `internal/cli/run.go`, `output.go` | `--check` reads the clock once; the executive profile accepted; the output name with the profile |
| `internal/resolve/node.go` | The zone of an IPv6 host (F-83) |
| `internal/catalog/keys.go`, `locales/*.toml`, `guard_test.go`, `keys_test.go` | The finding sentences by family; the wording of the executive; the key functions; the guards |
| `internal/document/reasons.go` | `reasonFragment` for node and workload findings; `workloadReason` |
| `internal/document/executive.go` | The executive body: the summaries and the seven tables |
| `internal/document/build.go` | The two profiles |
| `internal/document/sections_test.go` | The parity test over blocks 2 to 38 |
| `internal/goldentest/goldentest.go`, `golden_test.go`, `original_test.go` | The executive data files; the comparison of recommendations, summary, coverage, executive rows |
| `cmd/update-testdata/main.go` | Writes the executive data files |
| `testdata/python/` | Eight recorded files, recorded again, and the README |
| `testdata/snapshot/`, `template/tests/capacity/fixtures/command/` | Regenerated; four executive data files added |
| `template/golden/snapshots.go`, `template/tests/golden/` | The golden pages of section 5 and of the executive profile |
| `README.md`, `AGENTS.md`, `CHANGELOG.md` | The documents |

---

### Task 0: Open the part

**Executor:** Main thread.

- [ ] **Step 1:** Run `git status`, `git log --oneline -3`, and
  `go test -count=1 ./...`. Read `ROADMAP.md` and `CHANGELOG.md` and confirm
  they describe the real state: `main` at `afaced6`, clean, every package
  green, no entry of this part in the changelog. Correct `ROADMAP.md` first
  if it disagrees (the ruling E78 of part 2.3 was missing from its table
  and was added on 2026-09-30).
- [ ] **Step 2:** Create the branch and confirm it.

```bash
git switch -c feat/executive-profile
git branch --show-current
```

- [ ] **Step 3:** Record in `ROADMAP.md`: the summary line of 2.4 names this
  plan and the branch; a section "Phase 2.4" with the items P24-0 to P24-12
  of this plan, all `OPEN`; the rulings R61 to R77 as proposed, awaiting
  the owner's word.
- [ ] **Step 4:** Commit the plan.

```bash
git add docs/superpowers/plans/2026-09-30-phase-2-4-executive-profile.md
git commit -m "docs(plan): add the plan of part 2.4"
```

**Done when:** the branch exists, the plan is committed, and `ROADMAP.md`
holds the part.

---

### Task 1: The recommendations of the analysis (R63)

**Executor:** Subagent.

**Files:**
- Create: `internal/analysis/recommendations.go`, `internal/analysis/recommendations_test.go`

**Interfaces:**
- Consumes: `analysis.Finding`, `analysis.Rank`, the severities and the scope constants of `internal/analysis/registry.go`.
- Produces:

```go
// Recommendation is one action of the report: the findings of one code
// above healthy, merged. It carries no sentence.
type Recommendation struct {
	Code       string
	Severity   string   // the highest of the merged findings
	ScopeType  string   // the scope type of the first merged finding
	Scopes     []string // the distinct scope names, sorted; never empty
	Confidence string   // the most exact of the merged findings
}

// ConfidenceRank numbers a confidence as the original does: 0 for exact,
// 1 for inferred, 2 for unknown. It returns 3 for any other text.
func ConfidenceRank(confidence string) int

// Recommendations follows build_recommendations of the original: it
// skips the findings whose severity is healthy, groups the rest by code
// in the order the first finding of each code appears, and returns one
// Recommendation per group, sorted by severity from the highest, then
// ConfidenceRank, then the first scope name, then the code.
func Recommendations(findings []Finding) []Recommendation
```

`Recommendations` returns an empty, non-nil slice for no finding. Scopes
are sorted with `sort.Strings` (byte order, which is code point order for
UTF-8, as Python sorts). The sort is `sort.SliceStable`.

**Tests** (`recommendations_test.go`; every expected value is a literal, read
from `.cache/phase-2/part-4/python/out/` on 2026-09-30, where the original
gave it):

| Test | Input | Expected |
|---|---|---|
| `TestRecommendationsSkipHealthy` | one healthy finding, one attention finding | one recommendation, the attention one |
| `TestRecommendationsMergeOneCode` | `WORKLOAD_CPU_REQUEST_CRITICAL` on `payments/b` (exact) and `payments/a` (critical, inferred), then `WORKLOAD_CPU_REQUEST_CRITICAL` on `payments/b` again with severity attention | one recommendation: severity `critical`, `Scopes` `["payments/a", "payments/b"]`, `Confidence` `exact`, `ScopeType` `workload` |
| `TestRecommendationsOrder` | three findings: `CRITICAL_INFERRED` critical inferred on `payments/a`; `ATTENTION` attention exact on `payments/a`; `CRITICAL_EXACT` critical exact on `payments/z` | codes in the order `CRITICAL_EXACT`, `CRITICAL_INFERRED`, `ATTENTION` (the test of the original, T/test_recommendations.py:44-61) |
| `TestRecommendationsOrderByFirstScopeThenCode` | four attention, exact findings: code `B` on `x`, code `A` on `y`, code `A` on `x`, code `C` on `x` | after merging, `A` (scopes `x`, `y`), `B` (`x`), `C` (`x`): first scope `x` ties, then code |
| `TestRecommendationsOfDirectComplete` | the 13 findings of `direct-complete` written as literals (code, severity, scope type, scope name, kind, confidence; the evidence left empty) in the order of `testdata/python/direct-complete.json` | 8 recommendations in this order: `NAMESPACE_MEMORY_LIMIT_CRITICAL` (critical, exact, `[payments]`, namespace), `NAMESPACE_MEMORY_REQUEST_CRITICAL`, `WORKLOAD_MEMORY_LIMIT_CRITICAL` (`[payments/payments-web]`, workload), `WORKLOAD_MEMORY_REQUEST_CRITICAL`, `NAMESPACE_CPU_REQUEST_ATTENTION` (attention, exact), `WORKLOAD_CPU_REQUEST_ATTENTION`, `NODE_AUTOSCALING_NEAR_MAX` (attention, inferred, `[cluster]`, cluster), `HPA_REPLICA_DIVERGENCE` (attention, inferred, `[payments/payments-web]`, hpa) |
| `TestRecommendationsOfEdgeScalingPutConfidenceBeforeScope` | the five critical, exact findings and the four critical, inferred findings of `edge-scaling` (codes, scopes, and confidences from `original-recommendations.md` section 1) | the nine codes in the order `NODE_CPU_USAGE_CRITICAL`, `WORKLOAD_MEMORY_LIMIT_CRITICAL`, `WORKLOAD_MEMORY_REQUEST_CRITICAL`, `HPA_MAX_REPLICAS_REACHED`, `NODE_CPU_REQUEST_HIGH`, `NAMESPACE_CPU_LIMIT_CRITICAL`, `NAMESPACE_CPU_REQUEST_CRITICAL`, `NAMESPACE_MEMORY_LIMIT_CRITICAL`, `NAMESPACE_MEMORY_REQUEST_CRITICAL`: `aks-agentpool-12345678-vmss000000` < `payments/payments-web` < `payments/web` < `pool-a-1` among the exact, then `payments` among the inferred |
| `TestRecommendationsEmpty` | no finding | an empty, non-nil slice |
| `TestConfidenceRank` | `exact`, `inferred`, `unknown`, `x` | 0, 1, 2, 3 |

- [ ] **Step 1:** Write the tests. Run `go test ./internal/analysis/ -run Recommend -v`. Expected: build failure on the missing names, then, with stubs that return nothing, every test red.
- [ ] **Step 2:** Implement. Run the same. Expected: pass.
- [ ] **Step 3:** Run `make lint` and `go test -count=1 ./internal/analysis/`. Commit.

```bash
git add internal/analysis/recommendations.go internal/analysis/recommendations_test.go
git commit -m "feat(analysis): add the recommendations"
```

**Done when:** the eight tests pass and nothing else changed.

---

### Task 2: Recommendations in the snapshot (R63, R75)

**Executor:** Subagent for the code and the tests; the main thread regenerates the golden snapshots at the end.

**Files:**
- Modify: `internal/snapshot/snapshot.go`, `build.go`, `codec.go`, `build_test.go`, `codec_test.go`

**Interfaces:**
- Consumes: `analysis.Recommendations`, `analysis.Recommendation`, `analysis.ConfidenceRank` (Task 1).
- Produces:

```go
// Recommendation is one action of the report, spec 7.4: the findings of
// one code above healthy, merged. It carries no sentence.
type Recommendation struct {
	Code       string   `json:"code"`
	Severity   string   `json:"severity"`
	ScopeType  string   `json:"scope_type"`
	Scopes     []string `json:"scopes"` // sorted, never empty
	Confidence string   `json:"confidence"`
}
```

`Snapshot` gains `Recommendations []Recommendation` with the tag
`recommendations`, after `Findings`. `Summary` gains `Recommendations float64`
with the tag `recommendations`, after `Findings` and before `Healthy`.
`Build` derives the list with `analysis.Recommendations(in.Findings)` and
copies it, field by field, with `buildRecommendations`; `buildSummary` counts
it. The package comment of `snapshot.go` says the twelve sections are
filled and no part is left.

`Check` gains, after `checkFindingsOrder`, `checkRecommendations(s)`: it
rebuilds `[]analysis.Finding` from `s.Findings` (code, severity, scope type,
kind, name, confidence; no evidence), calls `analysis.Recommendations`, and
compares the result with `s.Recommendations` entry by entry. Messages:

- `recommendations holds %d entries, want %d from the findings`
- `recommendations[%d] differs from the findings: got %s, want %s`, where
  each side is written as `code severity scope_type [scopes] confidence`.

`checkSummary` adds the row `{"recommendations", s.Summary.Recommendations, float64(len(s.Recommendations))}`
after `findings`. The key walk needs no change: a slice of structs and a
slice of strings are supported (codec.go:199-234).

**Tests:**

| Test | Input | Expected |
|---|---|---|
| `TestBuildFillsTheRecommendations` (build_test.go) | the input of `TestBuildFillsTheNewSections` with findings that give two recommendations | `snap.Recommendations` equals the two, in order, field by field; `snap.Summary.Recommendations` is 2 |
| `TestBuildWritesNoRecommendationForHealthy` | findings all healthy | `Recommendations` is `[]` (not null in the encoding) and the count 0 |
| `TestBuildEveryFieldChangesTheBytes` | add the case: a finding's severity raised from healthy to attention changes the bytes of `recommendations` | pass |
| `TestKeyPathsAreTheContract` (codec_test.go) | | the paths `summary.recommendations`, `recommendations`, `recommendations[].code`, `.severity`, `.scope_type`, `.scopes`, `.confidence` are in the contract, in that place |
| `TestDecodeReadsWhatEncodeWrote`, `TestEncodeDecodeRoundTrip` | a snapshot with two recommendations | round trip equal |
| `TestCheckRecomputesTheRecommendations` | a valid snapshot whose list is then changed: an entry removed; an entry added; two entries swapped; a scope renamed; a confidence changed; the severity changed | each refused with the message above, naming the index of the first difference |
| `TestCheckCountsTheRecommendations` | `Summary.Recommendations` set to one more | `summary.recommendations is 3, want 2` |
| `TestDecodeRefusesMissingNewSection` | a file without `recommendations`; a file without `summary.recommendations` | refused by the key walk, naming the key |
| `TestBuildOutputAlwaysPassesDecode` | | still passes |

Readers that fail after this task until the main thread regenerates:
`TestGoldenSnapshots` of `internal/goldentest` (eight files), and the
snapshot tests of `internal/cli` that decode `testdata/snapshot/*.json`
(`report_test.go`, `run_test.go`; see `go-interfaces.md` section 10),
`internal/snapshot/codec_test.go:2108` which reads `edge-scaling.json`,
`internal/snapshot/value_test.go:157`. The brief says so and names them;
the subagent runs the suite and reports which failed and why; the main
thread checks the list against this one.

- [ ] **Step 1:** Write the tests. Run `go test ./internal/snapshot/ -run 'Recommend|KeyPaths|MissingNewSection' -v`. Expected: red.
- [ ] **Step 2:** Implement. Run `go test -count=1 ./internal/snapshot/`. Expected: pass, except the one test that reads `testdata/snapshot/edge-scaling.json`, which fails on the missing key until step 4.
- [ ] **Step 3:** Run `make lint`. Commit.

```bash
git add internal/snapshot
git commit -m "feat(snapshot): store the recommendations and their count"
```

- [ ] **Step 4 (main thread):** Run `make update-testdata`. Read every changed file: the eight snapshots gain `recommendations` and `summary.recommendations`; every count equals the table of `original-recommendations.md` section 2; no data file changes. Run `go test -count=1 ./...`. Commit.

```bash
git add testdata/snapshot
git commit -m "test(golden): store the recommendations in the snapshots"
```

**Done when:** every package is green and the eight snapshots hold the
counts of the original.

---

### Task 3: Three small closures: F-88, F-92, F-83, and the output name (R68 to R71)

**Executor:** Subagent, three commits.

**Files:**
- Modify: `internal/config/validate.go`, `validate_test.go`; `internal/snapshot/codec.go`, `codec_test.go`; `internal/cli/run.go`, `run_test.go`, `output.go`, `output_test.go`; `internal/resolve/node.go`, `node_test.go`

**Part A, the finite numbers (F-88, R69).**

`internal/config/validate.go`: two new errors,
`errThresholdFinite = errors.New("threshold values must be finite numbers")`
and `errMetricsTimeoutFinite = errors.New("metrics.timeout_seconds must be a finite number")`.
`validateThresholds` checks every one of the seven with `math.IsNaN` or
`math.IsInf` before any range check and returns `errThresholdFinite`;
`validateMetrics` checks the timeout the same way, before the positive
check, and returns `errMetricsTimeoutFinite`. The TOML reader accepts
`nan`, `inf`, and `-inf` as numbers (BurntSushi/toml 1.6.0, measured on
2026-09-30), so the tests go through `config.Load` with a file.

`internal/snapshot/codec.go`: `Check` refuses a threshold or a timeout
that is not finite, before the range checks it already has, with the
messages `report.thresholds.<name> must be a finite number` and
`report.settings.timeout_seconds must be a finite number`. A JSON file
cannot carry one, so the test calls `Check` on a built snapshot whose
field was set by hand.

| Test | Input | Expected |
|---|---|---|
| `TestValidateRefusesANonFiniteThreshold` | a file with `warning_percent = nan`; with `bursty_gap_percent = inf`; with `hpa_replica_divergence = -inf`; with `low_utilization_percent = inf` | `errThresholdFinite` each |
| `TestValidateRefusesANonFiniteTimeout` | `timeout_seconds = nan`; `= inf` | `errMetricsTimeoutFinite` |
| `TestValidateAcceptsTheBoundaries` (exists or add) | `warning_percent = 0`, `critical_percent = 100` | accepted |
| `TestCheckRefusesANonFiniteThreshold` (codec_test.go) | each of the seven set to `math.NaN()`; `TimeoutSeconds` set to `math.Inf(1)` | the message above, with the name |

**Part B, the clock of `--check` (F-92, R68).**

Remove `collect` of `run.go:484-495`; `runCheck` calls `preflight`
directly and drops `collectionCompletedAt`; the comment of `preflight`
says a check reads the clock once, before the preflight, and a run that
collects reads it again when the collectors return.

| Test | Input | Expected |
|---|---|---|
| `TestCheckReadsTheClockOnce` (run_test.go) | an `Env` whose `Now` counts its calls; `report --config <direct-complete> --check` | exit 0; one call |
| `TestSnapshotReadsTheClockTwice` | the same with `--snapshot` | exit 0; two calls |

**Part C, the output name (R71).**

`defaultOutputName(cluster, profile, minSeverity, language string, emittedAt time.Time) string`:
the suffix is `-executive` when `profile == "executive"`, else `-<minSeverity>`
when the severity is not `all`, else empty. `runPDF` passes
`loaded.Config.Report.Profile`; `runFromSnapshot` passes the profile it
resolved (the option, else the snapshot's).

| Test | Input | Expected |
|---|---|---|
| `TestDefaultOutputName` (output_test.go, extend) | `("prod", "executive", "critical", "en", t)` | `report-resources-prod-executive-en-20260715-143000.pdf` |
| | `("prod", "detailed", "critical", "pt-BR", t)` | `report-resources-prod-critical-pt-br-20260715-143000.pdf` |
| | `("prod", "detailed", "all", "en", t)` | `report-resources-prod-en-20260715-143000.pdf` |

The command tests of the name in the executive profile come with Task 7,
when the profile is accepted.

**Part D, the zone of a host (F-83, R70).**

`InstanceHost`: after the host is cut, split it at the first `%`; lower
the part before it and keep the rest as written. `netip.ParseAddr` accepts
a zone, so the bracket check is unchanged.

| Test | Input | Expected |
|---|---|---|
| `TestInstanceHostKeepsTheZone` | `[fe80::1%ETH0]:9100` | `fe80::1%ETH0` |
| | `[FE80::1%eth0]:9100` | `fe80::1%eth0` |
| `TestInstanceHostLowersAsGoDoes` | `İSTANBUL:9100` | `istanbul` (the original gives `i̇stanbul`, with U+0307); the test's comment names the difference and R70 |
| | `ΟΔΥΣΣΕΎΣ:9100` | `οδυσσεύσ` (the original gives `οδυσσεύς`) |
| | `ẞ:1` | `ß` (both agree) |

- [ ] **Step 1:** Part A: tests red, implement, green. Commit `fix(config): refuse a threshold or timeout that is not finite` with the four files of config and snapshot.
- [ ] **Step 2:** Part B and C: tests red, implement, green. Commit `fix(cli): read the clock once in a check and name the profile` with the four files of cli.
- [ ] **Step 3:** Part D: tests red, implement, green. Commit `fix(resolve): keep the zone of an instance host` with the two files of resolve.
- [ ] **Step 4:** `make lint`; `go test -count=1 ./...`.

**Done when:** the three commits are in, every package is green, and no
message of the program changed but the ones named here.

---

### Task 4: The finding sentences of the catalog, family by family (R64, R65)

**Executor:** Main thread writes the entries below into both files; a
subagent adds the key functions, derives `Keys()` from the registry, adds
the guard tables and the two new guards, and adapts the two call sites of
`internal/document/nodepools.go`.

**Files:**
- Modify: `internal/catalog/keys.go`, `keys_test.go`, `guard_test.go`, `locales/pt-BR.toml`, `locales/en.toml`, `internal/document/nodepools.go` (the two calls of `catalog.FindingSummary`)

**Interfaces:**
- Consumes: `analysis.Registry`, `analysis.Definition` (`Family`, `Params["resource"]`).
- Produces, in `keys.go`; `FindingFamilies` and the old `FindingSummary(family string)` go:

```go
// FindingSummary returns the key of the summary of the findings of
// family, one of the families of analysis.Registry, for resource, the
// "resource" parameter of the definition, or "" when it has none:
// finding.<family>.<resource>.summary, or finding.<family>.summary.
func FindingSummary(family, resource string) Key

// FindingAction returns the key of the action of the same family and
// resource: finding.<family>[.<resource>].action.
func FindingAction(family, resource string) Key

// FindingObservation returns the key of the short observation the
// executive pool table prints for the family, one of ObservationFamilies,
// and resource: finding.<family>[.<resource>].observation.
func FindingObservation(family, resource string) Key

// ObservationFamilies lists the families that carry an observation: the
// families of the codes of the node and nodepool scopes.
var ObservationFamilies = []string{"node_usage", "node_request_high", "node_overrequested", "node_low_utilization", "nodepool_uneven", "nodepool_unknown"}

// FindingPairs returns the distinct pairs of family and resource of
// analysis.Registry, in the order of the registry.
func FindingPairs() []FindingPair
type FindingPair struct{ Family, Resource string }
```

`Keys()` appends, for every pair, `FindingSummary` and `FindingAction`, and,
for a pair whose family is in `ObservationFamilies`, `FindingObservation`.
`internal/catalog` imports `internal/analysis` (rank 60 below 80); the
guard of the layers needs no change.

**The entries.** Portuguese under the wording rule, from the sentences of
`original-sentences.md` section 1; English new. The TOML holds one table
per pair, `[finding.<family>.<resource>]` or `[finding.<family>]`, with
`summary`, `action`, and, where the family is in `ObservationFamilies`,
`observation`. The two existing summaries keep their text.

| Pair | pt-BR summary | pt-BR action | en summary | en action |
|---|---|---|---|---|
| `resource_request_usage.cpu` | Uso de CPU em relação ao request. | Revisar o request de CPU e o consumo observado. | CPU usage against the request. | Review the CPU request and the observed consumption. |
| `resource_request_usage.memory` | Uso de memória em relação ao request. | Revisar o request de memória e o consumo observado. | Memory usage against the request. | Review the memory request and the observed consumption. |
| `resource_limit_usage.cpu` | O uso de CPU em relação ao limit indica risco aumentado de throttling de CPU. | Revisar o limit de CPU e o perfil de consumo observado. | CPU usage against the limit indicates an increased risk of CPU throttling. | Review the CPU limit and the observed consumption profile. |
| `resource_limit_usage.memory` | O uso de memória em relação ao limit indica risco aumentado de OOMKill. | Revisar o limit de memória e o perfil de consumo observado. | Memory usage against the limit indicates an increased risk of OOMKill. | Review the memory limit and the observed consumption profile. |
| `resource_request_missing.cpu` | Há uma lacuna de configuração: request de CPU não observado. | Definir o request de CPU com base no consumo observado. | There is a configuration gap: no CPU request was observed. | Set the CPU request from the observed consumption. |
| `resource_request_missing.memory` | Há uma lacuna de configuração: request de memória não observado. | Definir o request de memória com base no consumo observado. | There is a configuration gap: no memory request was observed. | Set the memory request from the observed consumption. |
| `resource_limit_missing.cpu` | Há uma lacuna de configuração: limit de CPU não observado. | Definir ou justificar o limit de CPU conforme a política do workload. | There is a configuration gap: no CPU limit was observed. | Set or justify the CPU limit according to the policy of the workload. |
| `resource_limit_missing.memory` | Há uma lacuna de configuração: limit de memória não observado. | Definir ou justificar o limit de memória conforme a política do workload. | There is a configuration gap: no memory limit was observed. | Set or justify the memory limit according to the policy of the workload. |
| `resource_overrequested.cpu` | O uso médio de CPU ficou baixo em relação ao request. | Revisar o dimensionamento do request de CPU. | Average CPU usage was low against the request. | Review the sizing of the CPU request. |
| `resource_overrequested.memory` | O uso médio de memória ficou baixo em relação ao request. | Revisar o dimensionamento do request de memória. | Average memory usage was low against the request. | Review the sizing of the memory request. |
| `resource_bursty.cpu` | O consumo de CPU apresentou variação relevante entre média e pico. | Inspecionar o perfil de rajadas de CPU antes do dimensionamento. | CPU consumption varied markedly between average and peak. | Inspect the CPU burst profile before sizing. |
| `resource_bursty.memory` | O consumo de memória apresentou variação relevante entre média e pico. | Inspecionar o perfil de rajadas de memória antes do dimensionamento. | Memory consumption varied markedly between average and peak. | Inspect the memory burst profile before sizing. |
| `node_usage.cpu` | Uso observado de CPU no node. | Acompanhar pressão e capacidade do node. | Observed CPU usage on the node. | Follow the pressure and the capacity of the node. |
| `node_usage.memory` | Uso observado de memória no node. | Acompanhar pressão e capacidade do node. | Observed memory usage on the node. | Follow the pressure and the capacity of the node. |
| `node_request_high.cpu` | Requests de CPU ocupam parcela elevada do node. | Revisar requests e margem de agendamento. | CPU requests take a high share of the node. | Review the requests and the scheduling headroom. |
| `node_request_high.memory` | Requests de memória ocupam parcela elevada do node. | Revisar requests e margem de agendamento. | Memory requests take a high share of the node. | Review the requests and the scheduling headroom. |
| `node_overrequested` | Requests elevados coexistem com baixa utilização real observada. | Revisar o dimensionamento dos workloads agendados no node. | High requests coexist with low observed utilization. | Review the sizing of the workloads scheduled on the node. |
| `node_low_utilization` | O node apresentou baixa utilização real média. | Avaliar consolidação e redução segura de capacidade. | The node showed low average utilization. | Assess consolidation and a safe reduction of capacity. |
| `nodepool_unknown` | O node pool não pôde ser determinado. | Confirmar o node pool sem inferir identidade do provedor. | The node pool could not be determined. | Confirm the node pool without inferring the identity of the provider. |
| `nodepool_uneven.cpu` | A pressão de pico de CPU variou entre nodes do pool. | Investigar distribuição de carga e restrições de agendamento. | Peak CPU pressure varied among the nodes of the pool. | Investigate the load distribution and the scheduling constraints. |
| `nodepool_uneven.memory` | A pressão de pico de memória variou entre nodes do pool. | Investigar distribuição de carga e restrições de agendamento. | Peak memory pressure varied among the nodes of the pool. | Investigate the load distribution and the scheduling constraints. |
| `hpa_pinned` | O HPA está fixado em um número constante de replicas. | Confirmar se o autoscaling é desejado para este workload. | The HPA is pinned to a constant number of replicas. | Confirm whether autoscaling is wanted for this workload. |
| `hpa_max_reached` | O HPA atingiu o máximo observado de replicas. | Revisar o máximo configurado e a capacidade do workload. | The HPA reached its observed maximum of replicas. | Review the configured maximum and the capacity of the workload. |
| `hpa_near_max` | O HPA operou frequentemente próximo ao máximo observado. | Validar a margem e o máximo configurado do HPA. | The HPA often ran close to its observed maximum. | Validate the headroom and the configured maximum of the HPA. |
| `hpa_divergence` | Há diferença média persistente entre replicas atuais e desejados. | Investigar eventos, restrições de agendamento e tempo de estabilização. | There is a persistent average difference between current and desired replicas. | Investigate the events, the scheduling constraints, and the stabilization time. |
| `deployment_without_hpa` | Deployment sem HPA observado. | Validar se o autoscaling horizontal é necessário para este Deployment. | Deployment observed without HPA. | Validate whether horizontal autoscaling is needed for this Deployment. |
| `statefulset_without_hpa` | StatefulSet sem HPA observado; registro informativo. | Confirmar a estratégia de escala adequada ao StatefulSet. | StatefulSet observed without HPA; for information. | Confirm the scaling strategy suited to the StatefulSet. |
| `autoscaling_unknown` | O estado do autoscaling de nodes não pôde ser determinado. | Confirmar operacionalmente se o autoscaling do cluster está habilitado. | The state of node autoscaling could not be determined. | Confirm operationally whether cluster autoscaling is enabled. |
| `autoscaling_disabled` | O autoscaling de nodes está configurado como desabilitado. | Planejar capacidade manual e revisar a decisão de autoscaling. | Node autoscaling is configured as disabled. | Plan capacity by hand and review the autoscaling decision. |
| `autoscaling_near_max` | O número de nodes está próximo ao máximo observado. | Revisar o máximo configurado e a margem do cluster. | The number of nodes is close to the observed maximum. | Review the configured maximum and the headroom of the cluster. |
| `autoscaling_bounds_unknown` | Autoscaling habilitado sem mínimo e máximo observáveis nas métricas. | Registrar o mínimo e o máximo operacionais sem inferir valores ausentes. | Autoscaling enabled with no observable bounds in the metrics. | Record the operational bounds without inferring absent values. |
| `autoscaling_at_min` | Autoscaling operando no mínimo observado sem pressão de nodes. | Manter acompanhamento da demanda. | Autoscaling running at the observed minimum with no node pressure. | Keep following the demand. |
| `autoscaling_enabled` | Autoscaling de nodes habilitado com margem observada. | Manter acompanhamento do mínimo, do máximo e dos eventos de escala. | Node autoscaling enabled with observed headroom. | Keep following the bounds and the scaling events. |

Where the original wrote `limite máximo` of an HPA, `limites` of the
autoscaling, `escalonamento automático`, `limitação de CPU`, `carga de
trabalho`, `nó`, `pool de nós`, `réplicas`, and `solicitação`, the entries
above write `máximo configurado`, `mínimo e máximo`, `autoscaling`,
`throttling de CPU`, `workload`, `node`, `node pool`, `replicas`, and
`request`, by spec 9.3. The participle agrees with the masculine English
noun: `replicas atuais e desejados`, `request não observado`.

The observations:

| Pair | pt-BR observation | en observation |
|---|---|---|
| `node_usage.cpu` | Uso de CPU requer atenção | CPU usage needs attention |
| `node_usage.memory` | Uso de memória requer atenção | Memory usage needs attention |
| `node_request_high.cpu` | Requests de CPU elevados | High CPU requests |
| `node_request_high.memory` | Requests de memória elevados | High memory requests |
| `node_overrequested` | Requests elevados com baixa utilização | High requests with low utilization |
| `node_low_utilization` | Baixa utilização média | Low average utilization |
| `nodepool_uneven.cpu` | Distribuição desigual de CPU | Uneven CPU distribution |
| `nodepool_uneven.memory` | Distribuição desigual de memória | Uneven memory distribution |
| `nodepool_unknown` | Identificação do pool indisponível | Pool identification unavailable |

**Tests** (guard_test.go, keys_test.go):

| Test | Input | Expected |
|---|---|---|
| `TestFindingPairs` | | 33 pairs, the first `{resource_request_usage, cpu}`, the last `{autoscaling_enabled, ""}`; `autoscaling_disabled` once although two codes carry it |
| `TestEveryDefinitionHasItsSentences` | every definition of `analysis.Registry`, both files | `FindingSummary` and `FindingAction` of its family and resource are entries with no parameter; `FindingObservation` is an entry exactly when the family is in `ObservationFamilies` |
| `TestFindingActionsAreDistinct` | both files | no two pairs give one action text in one language (R65); the same for the summaries |
| `TestFindingWordingMatchesThePlan` | the tables above as a Go table (`findingWordingTable` with pair, pt summary, pt action, en summary, en action; `observationWordingTable`) | every row equals the file |
| `TestKeysEqualTheFiles` | | passes with the 75 keys of the two tables (66 plus 9 observations) and without `finding.node_overrequested.summary` counted twice |
| `TestNoBannedTerm` | | no entry named |
| `TestEveryKeyIsUsed` | | red on `FindingAction` and `FindingObservation`, which Task 6 calls; the brief says so (E74) |
| `TestKeysHoldTheNewFamilies` (keys_test.go) | | adds the count of `finding.` keys: 75 |

- [ ] **Step 1 (main thread):** Write the entries into both files. Commit them alone, so the subagent starts from a committed tree.

```bash
git add internal/catalog/locales
git commit -m "feat(catalog): add the sentences of the 66 findings"
```

- [ ] **Step 2 (subagent):** Write the tests; run `go test ./internal/catalog/ -run 'FindingPairs|Definition|Distinct|FindingWording|KeysEqual|Banned|HoldTheNew' -v`; red. Implement `keys.go`; adapt the two calls in `internal/document/nodepools.go:378,380` to `catalog.FindingSummary("node_overrequested", "")` and `("node_low_utilization", "")`; green. Run the whole catalog package: only `TestEveryKeyIsUsed` is red, on the two uncalled functions. Run `go test -count=1 ./internal/document/`: green.
- [ ] **Step 3:** `make lint`. Commit.

```bash
git add internal/catalog/keys.go internal/catalog/keys_test.go internal/catalog/guard_test.go internal/document/nodepools.go
git commit -m "feat(catalog): tie the finding families to the registry"
```

**Done when:** every test above but `TestEveryKeyIsUsed` passes, and that
one names exactly `FindingAction` and `FindingObservation`.

---

### Task 5: The wording of the executive body

**Executor:** Main thread writes the entries into both files; a subagent
adds the constants, the key functions, the lists, and the guard tables.

**Files:**
- Modify: `internal/catalog/keys.go`, `keys_test.go`, `guard_test.go`, `locales/pt-BR.toml`, `locales/en.toml`

**Interfaces:**
- Produces the constants below in `wordingKeys`, the extension of
  `ReasonFamilies`, and four key functions with their lists:

```go
// ReasonFamilies gains request_peak, request_average, limit_peak,
// limit_average, request_missing, limit_missing, bursty, after the three
// of part 2.3.

// Priorities lists the priorities the executive body prints: attention
// and critical. Priority returns the key of the label of one:
// priority.<severity>.
var Priorities = []string{"attention", "critical"}
func Priority(severity string) Key

// RiskSeverities lists the two severities a namespace at risk counts.
// RiskCount returns the key of the count of workloads at severity, one of
// RiskSeverities: risk_status.<severity>.singular for a count of 1,
// .plural for any other count.
var RiskSeverities = []string{"critical", "attention"}
func RiskCount(severity string, count float64) Key

// ScopeQualifiers lists the scope types whose action the plan may
// qualify. ScopeQualifier returns scope_qualifier.<scopeType>.
var ScopeQualifiers = []string{"namespace", "workload", "hpa", "cluster"}
func ScopeQualifier(scopeType string) Key

// ScopeSummaries lists the kinds of a scope list of more than three
// names. ScopeSummary returns scope_summary.<kind>.
var ScopeSummaries = []string{"namespace", "workload", "hpa", "deployment"}
func ScopeSummary(kind string) Key
```

`Keys()` appends the keys of the four lists as it appends the confidences.

**The entries of `[sections.executive]`** (constants `SectionsExecutive` plus the upper camel case of the key):

| Key | pt-BR | en |
|---|---|---|
| `heading` | Resumo executivo | Executive summary |
| `intro` | Síntese dos principais riscos identificados no período de {{.Period}}, com foco em pontos de prioridade crítica e alta que requerem atenção imediata. | Summary of the main risks identified over the period of {{.Period}}, with a focus on points of critical and high priority that need immediate attention. |
| `critical_heading` | Workloads com prioridade crítica | Workloads with critical priority |
| `critical_empty` | Nenhum workload com prioridade crítica foi identificado. | No workload with critical priority was identified. |
| `attention_heading` | Workloads com prioridade alta | Workloads with high priority |
| `attention_empty` | Nenhum workload com prioridade alta foi identificado. | No workload with high priority was identified. |
| `header_namespace` | Namespace | Namespace |
| `header_workload` | Workload | Workload |
| `header_pods` | Qtde. Pods | Pods |
| `header_reason` | Motivo | Reason |
| `namespaces_heading` | Namespaces com workload em risco | Namespaces with workloads at risk |
| `namespaces_empty` | Nenhum namespace com workload em risco foi identificado. | No namespace with a workload at risk was identified. |
| `header_cpu_limit` | CPU % / Limit | CPU % / Limit |
| `header_memory_limit` | Mem % / Limit | Mem % / Limit |
| `header_status` | Status | Status |
| `header_priority` | Prioridade | Priority |
| `scaling_heading` | Workloads com máximo de replicas e sem HPA | Workloads at their replica maximum and without HPA |
| `scaling_empty` | Nenhum workload atingiu o máximo de replicas ou apresentou ausência de HPA com risco automático no período analisado. | No workload reached its replica maximum or lacked an HPA with automatic risk in the analysis period. |
| `header_kind` | Tipo | Kind |
| `header_replicas` | Replicas | Replicas |
| `header_situation` | Situação | Situation |
| `replicas_observed` | {{.Peak}} observados / máximo {{.Max}} | {{.Peak}} observed / maximum {{.Max}} |
| `active_replicas.singular` | 1 ativo | 1 active |
| `active_replicas.plural` | {{.Count}} ativos | {{.Count}} active |
| `situation_hpa_max` | Máximo do HPA atingido | HPA maximum reached |
| `target_unknown` | (alvo não identificado) | (target not identified) |
| `nodepools_heading` | Situação dos node pools | Situation of the node pools |
| `header_pool` | Node pool | Node pool |
| `header_pool_situation` | Situação | Situation |
| `header_remark` | Observação | Remark |
| `pool_stable` | Estável | Stable |
| `pool_attention` | Atenção | Attention |
| `pool_critical` | Crítica | Critical |
| `actions_heading` | Plano de ação priorizado | Prioritized action plan |
| `actions_empty` | Nenhuma ação priorizada foi identificada. | No prioritized action was identified. |
| `header_order` | Ordem | Order |
| `header_action` | Ação | Action |
| `header_scope` | Workloads / Escopo | Workloads / Scope |
| `action_qualified` | {{.Action}} ({{.Qualifier}}) | {{.Action}} ({{.Qualifier}}) |
| `limitations_heading` | Limitações materiais | Material limitations |
| `limitations_empty` | Nenhuma limitação material foi identificada. | No material limitation was identified. |
| `header_capability` | Capacidade | Capability |
| `header_coverage` | Cobertura | Coverage |
| `interpretation` | Interpretação | Interpretation |
| `reason_all_missing` | Request e limit de CPU e memória não observados | CPU and memory request and limit not observed |
| `risk_status_both` | {{.Critical}} · {{.Attention}} | {{.Critical}} · {{.Attention}} |

The fallback of the pool table is `sections.nodepools.empty_summary`,
which already reads `Nenhum node pool foi identificado.`, the sentence the
original prints in both places.

**The other entries:**

| Key | pt-BR | en |
|---|---|---|
| `placeholder.empty_cell` | — | — |
| `priority.attention` | Alta | High |
| `priority.critical` | Crítica | Critical |
| `risk_status.critical.singular` | 1 crítica | 1 critical |
| `risk_status.critical.plural` | {{.Count}} críticas | {{.Count}} critical |
| `risk_status.attention.singular` | 1 alta | 1 high |
| `risk_status.attention.plural` | {{.Count}} altas | {{.Count}} high |
| `scope_qualifier.namespace` | namespaces | namespaces |
| `scope_qualifier.workload` | workloads | workloads |
| `scope_qualifier.hpa` | HPAs | HPAs |
| `scope_qualifier.cluster` | cluster | cluster |
| `scope_summary.namespace` | {{.Count}} namespaces identificados | {{.Count}} namespaces identified |
| `scope_summary.workload` | {{.Count}} workloads identificados | {{.Count}} workloads identified |
| `scope_summary.hpa` | {{.Count}} HPAs identificados | {{.Count}} HPAs identified |
| `scope_summary.deployment` | {{.Count}} Deployments identificados | {{.Count}} Deployments identified |
| `reason.request_peak.cpu` | CPU: pico de {{.Percent}} do request | CPU: peak of {{.Percent}} of the request |
| `reason.request_peak.memory` | Memória: pico de {{.Percent}} do request | Memory: peak of {{.Percent}} of the request |
| `reason.request_average.cpu` | CPU: média de {{.Percent}} do request | CPU: average of {{.Percent}} of the request |
| `reason.request_average.memory` | Memória: média de {{.Percent}} do request | Memory: average of {{.Percent}} of the request |
| `reason.limit_peak.cpu` | CPU: pico de {{.Percent}} do limit | CPU: peak of {{.Percent}} of the limit |
| `reason.limit_peak.memory` | Memória: pico de {{.Percent}} do limit | Memory: peak of {{.Percent}} of the limit |
| `reason.limit_average.cpu` | CPU: média de {{.Percent}} do limit | CPU: average of {{.Percent}} of the limit |
| `reason.limit_average.memory` | Memória: média de {{.Percent}} do limit | Memory: average of {{.Percent}} of the limit |
| `reason.request_missing.cpu` | Request de CPU não observado | CPU request not observed |
| `reason.request_missing.memory` | Request de memória não observado | Memory request not observed |
| `reason.limit_missing.cpu` | Limit de CPU não observado | CPU limit not observed |
| `reason.limit_missing.memory` | Limit de memória não observado | Memory limit not observed |
| `reason.bursty.cpu` | CPU: variação de {{.Percent}} | CPU: variation of {{.Percent}} |
| `reason.bursty.memory` | Memória: variação de {{.Percent}} | Memory: variation of {{.Percent}} |

The joiner of `risk_status_both` is the middle dot, U+00B7, the character
the original writes (R/severity.py:87). The em-dash of
`placeholder.empty_cell` is the one entry the guard of the em-dash lets
through, the em-dash alone.

**Tests:**

| Test | Input | Expected |
|---|---|---|
| `TestWordingMatchesThePlan`, `TestPortugueseOfTheSectionsMatchesThePlan` | the tables above added to `wordingTable` (both languages) and `sectionsWordingTable` | every row |
| `TestRiskCountChoosesTheEntry` (keys_test.go) | `("critical", 1)`, `("critical", 2)`, `("attention", 0)` | `risk_status.critical.singular`, `.plural`, `risk_status.attention.plural` |
| `TestKeysHoldTheNewFamilies` | | the 2 priorities, 4 risk counts, 4 qualifiers, 4 summaries, and 14 new reasons are in `Keys()` |
| `TestLanguagesUseTheSameParameters` | | `Period`, `Peak`, `Max`, `Count`, `Action`, `Qualifier`, `Critical`, `Attention`, `Percent` agree |
| `TestNoEmDashInsideAnEntry` | | passes: the em-dash alone is allowed |
| `TestNoBannedTerm` | | no entry named |
| `TestEveryKeyIsUsed` | | red on the constants of this task and the four functions, until Task 6 |

- [ ] **Step 1 (main thread):** Write the entries into both files. Commit.

```bash
git add internal/catalog/locales
git commit -m "feat(catalog): add the wording of the executive body"
```

- [ ] **Step 2 (subagent):** Add the constants, the functions, the lists, the table rows, and the tests. Run `go test -count=1 ./internal/catalog/ -run 'Wording|Portuguese|RiskCount|HoldTheNew|Parameters|EmDash|Banned|KeysEqual|Constant' -v`. Expected: pass. Run the whole package: `TestEveryKeyIsUsed` names the keys of this task and of Task 4 and nothing else.
- [ ] **Step 3:** `make lint`. Commit.

```bash
git add internal/catalog/keys.go internal/catalog/keys_test.go internal/catalog/guard_test.go
git commit -m "feat(catalog): add the keys of the executive body"
```

**Done when:** every guard but the usage guard passes, and the usage guard
names only what Task 6 will use.

---

### Task 6: The executive body as section 5 of the detailed report (R61, R65, R66, R67, R76)

**Executor:** Subagent.

**Files:**
- Create: `internal/document/reasons.go`, `reasons_test.go`, `internal/document/executive.go`, `executive_test.go`
- Modify: `internal/document/nodepools.go` (move `reasonFragment` and `evidenceNumber` out), `build.go`, `build_test.go`, `sections_test.go` (the parity test)

**Interfaces:**
- Consumes: `snapshot.Recommendation` (Task 2); the keys and functions of Tasks 4 and 5; `analysis.Lookup`, `analysis.Rank`, `analysis.ConfidenceRank`; the helpers `percentRun`, `poolRun`, `podsRun`, `effectiveCapabilities`, `appendixRemark`, `appendixStatus`, `hpaNote`, `minimumRank`, `namespaceHeading` that exist.
- Produces, in `reasons.go`:

```go
// reasonFragment writes the short reason of a finding, as the original's
// _finding_reason_fragment: the branches below are tried in order and the
// first that holds with its number present wins; a finding that reaches
// none keeps the first error of the builder with "finding %s has no
// reason text".
//   - a code ending in _REQUEST_MISSING: reason.request_missing.<resource>
//   - a code ending in _LIMIT_MISSING: reason.limit_missing.<resource>
//   - a code ending in _REQUEST_HIGH with "percent": reason.request_high
//   - a code holding _REQUEST_ with "peak_percent": reason.request_peak,
//     else with "average_percent": reason.request_average
//   - a code holding _LIMIT_: reason.limit_peak, else reason.limit_average
//   - a code holding _USAGE_: reason.usage_peak, else reason.usage_average
//   - a code ending in _OVERREQUESTED with "average_percent": reason.request_average
//   - a code ending in _BURSTY with "gap_percent": reason.bursty
//   - NODE_OVERREQUESTED and NODE_LOW_UTILIZATION: the summary of the
//     family without its trailing periods
// The resource is memory when the code holds _MEMORY_, else cpu.
func reasonFragment(b *builder, f snapshot.Finding) string

// evidenceNumber returns the number a finding carries under name.
func evidenceNumber(f snapshot.Finding, name string) (float64, bool)

// workloadReason joins the fragments of the findings of one workload, in
// the order given, each once, with "; ". When the four *_MISSING codes of
// a workload are all present, the entry sections.executive.reason_all_missing
// comes first and those four give no fragment of their own.
func workloadReason(b *builder, findings []snapshot.Finding) string
```

- Produces, in `executive.go`:

```go
// raiseToAttention returns severity, or attention when it is all: the
// executive body never shows a healthy row.
func raiseToAttention(severity string) string

// buildExecutiveSection assembles the executive body: the numbered
// heading, the intro, and the seven level-2 numbered subsections, each a
// heading and a table or a fallback paragraph, in the order of the
// original: critical workloads, attention workloads, namespaces at risk,
// replica ceiling, pool situation, action plan, material limitations. The
// minimum is raised to at least attention. In the executive profile the
// note of the HPA data follows the replica ceiling table when it has
// rows; in both profiles it follows the fallback sentence when it has
// none.
func buildExecutiveSection(b *builder, s snapshot.Snapshot, thresholds config.Thresholds, severity string, executiveProfile bool) []Block

// workloadSummary is one workload of the priority tables.
type workloadSummary struct {
	Namespace, Kind, Name string
	Pods                  *float64
	Severity              string
	Findings              []snapshot.Finding // severity descending, then code
}

// workloadSummaries groups the workload findings at or above attention,
// the two coverage codes left out, by scope name and kind, in that
// order; keeps the groups whose highest severity reaches minRank; names
// each by the row of the snapshot it matches, or by the finding's own
// kind with no pods when no observed row matches; and returns them
// sorted by namespace, kind, name.
func workloadSummaries(s snapshot.Snapshot, minRank int) ([]workloadSummary, error)

func priorityRows(b *builder, summaries []workloadSummary, severity string) [][][]Run
func namespacesAtRisk(b *builder, s snapshot.Snapshot, summaries []workloadSummary, thresholds config.Thresholds) []Block
func riskStatus(b *builder, critical, attention int) string
func replicaCeiling(b *builder, s snapshot.Snapshot, minRank int, executiveProfile bool) []Block
func nodepoolStatus(b *builder, s snapshot.Snapshot) []Block
func poolObservation(b *builder, f snapshot.Finding) string
func actionPlan(b *builder, s snapshot.Snapshot, minRank int) []Block
func recommendationScope(b *builder, r snapshot.Recommendation) Run
func materialLimitations(b *builder, s snapshot.Snapshot) []Block
```

**The rules**, from `original-executive.md` (OE) and
`original-recommendations.md` (OR); the implementer reads those sections
first and stops if the brief and the original disagree:

1. *Headings.* Section heading level 1 numbered `sections.executive.heading`;
   a paragraph `intro` with `Period` from `b.period`; seven level-2
   numbered headings. A subsection with no row prints its fallback
   paragraph (OE 1).
2. *Summaries* (OE 2.1). A workload finding whose scope name holds no
   slash is a builder error `finding %s names no namespace`. The candidate
   rows are those with `ObservedUsage` true and the same namespace, name,
   and kind; with none, one summary with the finding's kind and nil pods.
   The findings of a summary are sorted by rank descending, then code
   (`sort.SliceStable`).
3. *Priority rows* (OE 2.2). The rows of the summaries whose severity
   equals the table's; the name is `Ident(name)`, or
   `Ident(name + " (" + kind + ")")` when another summary of any severity
   has the same namespace and name; the pods `podsRun`-like: `Count` of the
   pods, or `placeholder.empty_cell` when nil (R67); the reason
   `workloadReason`. Table id `priority-workloads`, headers
   `header_namespace`, `header_workload`, `header_pods`, `header_reason`.
   The attention table is built only when the raised minimum is attention;
   at critical it prints its fallback.
4. *Namespaces at risk* (OE 3). Group the summaries by namespace in
   first-seen order; dominant is the first summary of the highest
   severity; counts of critical and attention; the namespace row of the
   snapshot gives the two limit percent peaks, printed with `percentRun`
   (unavailable when the state is not value or the peak nil); the reason
   is `workloadReason(dominant.Findings)`; the status `riskStatus`: the
   count entries `RiskCount` for each non-zero count, joined by
   `risk_status_both` when both are present; the priority
   `StatusText(severity, Priority(severity))`. Sorted by rank descending,
   the largest available peak descending (none counts as negative
   infinity), namespace ascending. Id `namespaces-at-risk`.
5. *Replica ceiling* (OE 4). The HPAs at their maximum (the rule of
   `hpaSituation`), each as a row keyed by the displayed kind and name of
   the target: `TargetKind` and `TargetName` when both are non-empty, else
   the text of `placeholder.unavailable` for the kind and `Name + " " +
   target_unknown` for the name; sorted by namespace, displayed kind,
   displayed name, HPA name; a second row of the same namespace, kind, and
   name is skipped. Cells: `Ident(namespace)`; `Ident(targetName)`, or
   `Ident(hpa.Name), Text(" "), Placeholder(target_unknown)`; `Text(kind)`
   (a text run, as the original's `typst_content`); `Text` of
   `replicas_observed` with `Peak` and `Max` through `Count`; status
   critical `situation_hpa_max`. Then, when the raised minimum is
   attention, every workload row whose `HPACoverage` is uncovered and
   whose kind is `Deployment`, in the order of the snapshot (namespace,
   name), skipped when its namespace, kind, and name were seen: cells
   `Ident(namespace)`, `Ident(name)`, `Text(kind)`, the replicas: the
   `Count` of the row's pods with `active_replicas` (singular at 1), or
   `placeholder.unavailable` when the pods are nil; status attention
   `sections.hpas.situation_no_hpa`. Id `replica-ceiling`. The note: with
   rows and `executiveProfile`, the callout of `hpaNote` after the table;
   with no rows, the fallback paragraph and then the callout when
   `hpaNote` gives one.
6. *Pool situation* (OE 5). For every pool, the findings of scope
   nodepool with its name or of scope node with one of its node names;
   the highest severity; the observations of the findings at that
   severity when it is not healthy, each once, through `poolObservation`,
   which looks the code up in `analysis.Lookup` and reads
   `FindingObservation(family, resource)`, a builder error for a family
   outside `ObservationFamilies`; the label `pool_stable`, `pool_attention`,
   `pool_critical`; the cells `poolRun(name)`, `StatusText(severity, label)`,
   `Text` of the observations joined with `"; "` or `placeholder.empty_cell`.
   Sorted by rank descending, then the displayed name: the text of
   `placeholder.undetermined` for the pool `unknown`, else the name
   (R76). Not filtered by the minimum. Id `nodepool-status`; the fallback
   `sections.nodepools.empty_summary`.
7. *Action plan* (OR 3). The recommendations whose rank reaches the
   raised minimum and whose code has neither the prefix `NODE_` nor
   `NODEPOOL_`, sorted by rank descending, code, then the scopes compared
   element by element (`slices.Compare`). The action of each is
   `FindingAction(family, resource)` of its code's definition. Count the
   rows by action text; a row whose text appears more than once prints
   `action_qualified` with `ScopeQualifier(scopeType)`, a builder error
   `recommendation of scope type %q has no qualifier` for a scope type
   outside `ScopeQualifiers`. Cells: `Text(b.cat.Integer(i + 1))`,
   `StatusText(severity, Priority(severity))`, the action, and
   `recommendationScope`: with 1 to 3 scopes, the texts after the last
   slash of each, or the whole scopes when two short names collide,
   joined with `", "`, as one `Ident`; with more, `Placeholder` of
   `ScopeSummary(kind)` with `Count`, the kind `namespace`, `hpa`,
   `workload`, or `deployment` for a workload whose code starts with
   `DEPLOYMENT_`, a builder error `recommendation of scope type %q has no
   summary` for `cluster` and the rest. Id `action-plan`.
8. *Material limitations* (OE 6). For every effective capability whose
   state is not available: `Text(CapabilityName)`,
   `StatusText(appendixStatus(state), State(state))`,
   `Text(appendixRemark(reasons))`; then, for every warning of the
   snapshot whose text (the reason entry for a reason code, else the
   warning entry, as `buildNotes` chooses) equals no remark printed above:
   `Text(interpretation)`, `StatusText("attention", State("partial"))`,
   `Text(text)`. Not filtered by the minimum. Id `material-limitations`.
**The tables, their ids, and their headers** (every header a key of
`sections.executive`, in column order):

| Subsection | Id | Headers |
|---|---|---|
| critical workloads, attention workloads | `priority-workloads` | `header_namespace`, `header_workload`, `header_pods`, `header_reason` |
| namespaces at risk | `namespaces-at-risk` | `header_namespace`, `header_cpu_limit`, `header_memory_limit`, `header_reason`, `header_status`, `header_priority` |
| replica ceiling | `replica-ceiling` | `header_namespace`, `header_workload`, `header_kind`, `header_replicas`, `header_situation` |
| pool situation | `nodepool-status` | `header_pool`, `header_pool_situation`, `header_remark` |
| action plan | `action-plan` | `header_order`, `header_priority`, `header_action`, `header_scope` |
| material limitations | `material-limitations` | `header_capability`, `header_coverage`, `header_remark` |

The headings of the subsections are `critical_heading`, `attention_heading`,
`namespaces_heading`, `scaling_heading`, `nodepools_heading`, `actions_heading`,
`limitations_heading`; the fallbacks `critical_empty`, `attention_empty`,
`namespaces_empty`, `scaling_empty`, `sections.nodepools.empty_summary`,
`actions_empty`, `limitations_empty`.

9. *Build.* In `Build`, for the profile `detailed`, the blocks of
   `buildExecutiveSection(b, s, thresholds, severity, false)` go after
   `nodepools` and before the weak page break. The appendix keeps the
   unraised severity.

**Tests** (`reasons_test.go`, `executive_test.go`, `build_test.go`,
`sections_test.go`; the snapshots are built with the helpers of the
package, `baseSnapshot`, `valueStat`, `unavailableWorkload`, `testNode`,
`nodeFinding`, and a new `workloadFinding(code, severity, namespace, name,
kind string, evidence ...snapshot.Evidence)`; the catalog is Portuguese
unless said; expected texts are read through the catalog by key, the
numbers are literals):

| Test | Input | Expected |
|---|---|---|
| `TestReasonFragmentOfWorkloads` | `WORKLOAD_CPU_REQUEST_CRITICAL` with peak 97 (and average 88); the same with average only (72); `WORKLOAD_MEMORY_LIMIT_ATTENTION` peak 78; `WORKLOAD_CPU_OVERREQUESTED` average 7.1; `WORKLOAD_CPU_BURSTY` gap 96.1; `WORKLOAD_CPU_REQUEST_MISSING`; `WORKLOAD_MEMORY_LIMIT_MISSING` | `CPU: pico de 97% do request`; `CPU: média de 72% do request`; `Memória: pico de 78% do limit`; `CPU: média de 7,1% do request`; `CPU: variação de 96,1%`; `Request de CPU não observado`; `Limit de memória não observado` |
| `TestReasonFragmentOfNodesUnchanged` | the four node cases of the existing `reasonFragment` tests, moved | unchanged |
| `TestReasonFragmentRefusesACodeWithoutText` | `HPA_NEAR_MAX_REPLICAS` | builder error `finding HPA_NEAR_MAX_REPLICAS has no reason text` |
| `TestWorkloadReasonCollapsesTheFourMissing` | the four `WORKLOAD_*_MISSING` of one workload, and `WORKLOAD_CPU_BURSTY` gap 50 | `Request e limit de CPU e memória não observados; CPU: variação de 50%` |
| `TestWorkloadReasonKeepsThreeMissingApart` | three of the four | three fragments, no collapsed sentence |
| `TestWorkloadReasonDeduplicates` | two findings with one fragment | once |
| `TestExecutiveOrder` | the parity snapshot, detailed | the heading `Resumo executivo` numbered level 1, the intro paragraph with `24 horas`, then seven level-2 numbered headings in the order of rule 1 |
| `TestExecutiveRaisesAllToAttention` | a snapshot with a healthy workload finding only, minimum `all` | both priority tables print their fallback; no healthy row anywhere |
| `TestExecutiveAtCriticalOmitsAttention` | `WORKLOAD_CPU_REQUEST_CRITICAL` on `api` and `WORKLOAD_MEMORY_REQUEST_ATTENTION` on `worker`, minimum `critical` | the critical table has `api`; the attention subsection is the fallback paragraph; the namespaces table counts `1 crítica` only |
| `TestPriorityRowsConsolidateOneWorkload` | `api` with `WORKLOAD_MEMORY_REQUEST_CRITICAL` peak 131.2, `WORKLOAD_CPU_BURSTY` gap 96.1, `WORKLOAD_CPU_OVERREQUESTED` average 7.1, pods 50 | one critical row: `payments`, `api`, `50`, `Memória: pico de 131,2% do request; CPU: variação de 96,1%; CPU: média de 7,1% do request`; nothing in the attention table |
| `TestPriorityRowsTellHomonymsApart` | rows `api` Deployment (pods 4) and `api` StatefulSet (pods 3); `WORKLOAD_MEMORY_LIMIT_CRITICAL` on the Deployment (peak 95), `WORKLOAD_CPU_REQUEST_ATTENTION` on the StatefulSet (average 72) | critical row `Ident("api (Deployment)")` with `Memória: pico de 95% do limit`; attention row `Ident("api (StatefulSet)")` with `CPU: média de 72% do request` |
| `TestPriorityRowsWithoutPods` | a coverage-only row `jobs` (observed_usage false) and a `WORKLOAD_MEMORY_LIMIT_ATTENTION` finding naming it | an attention row with the pods cell `—` and the kind of the finding |
| `TestCoverageCodesNeverReachThePriorityTables` | `DEPLOYMENT_WITHOUT_HPA` on `worker` only | both fallbacks; the namespaces fallback |
| `TestNamespacesAtRiskRows` | namespaces `payments` (cpu limit peak 49.9, memory 76.8) and `stock` (37.9, 45.4); `WORKLOAD_CPU_REQUEST_CRITICAL` on `payments/api` peak 97, `WORKLOAD_CPU_REQUEST_ATTENTION` on `stock/worker` peak 75 | `payments` before `stock`; cells healthy `49,9%`, attention `76,8%`; reasons; `1 crítica`, `1 alta`; priorities `Crítica`, `Alta` |
| `TestNamespacesAtRiskSortsByLargestPeak` | two namespaces at critical, limit peaks 90 and 95 | 95 first |
| `TestNamespacesAtRiskCountsBoth` | `api` critical and `queue` attention in `payments` | `1 crítica · 1 alta` |
| `TestNamespacesAtRiskUnavailablePercent` | the namespace's limit percents unavailable | two unavailable status runs |
| `TestReplicaCeilingRows` | HPA `api` (target Deployment `api`, min 2, max 10, peak 10) and uncovered Deployment `worker` with pods 2 | two rows: `payments`, `api`, `Deployment`, `10 observados / máximo 10`, critical `Máximo do HPA atingido`; `payments`, `worker`, `Deployment`, `2 ativos`, attention `Sem HPA` |
| `TestReplicaCeilingSingularActive` | pods 1 | `1 ativo` |
| `TestReplicaCeilingUncoveredWithoutPods` | a coverage-only uncovered Deployment | `Indisponível` in the replicas cell |
| `TestReplicaCeilingSkipsPinnedAndStatefulSets` | a pinned HPA (min 1, max 1, peak 1); an uncovered StatefulSet | no row, the fallback |
| `TestReplicaCeilingUnidentifiedTarget` | HPA `api-hpa` with empty target, peak 10, max 10; and a second `api-cpu` with target name only | rows in name order `api-cpu`, `api-hpa`, each `Ident(name), Text(" "), Placeholder("(alvo não identificado)")` and the kind `Indisponível` as a text run |
| `TestReplicaCeilingDeduplicatesTargets` | two HPAs at max with one target | one row |
| `TestReplicaCeilingAtCriticalHidesUncovered` | the rows of `TestReplicaCeilingRows`, minimum `critical` | the HPA row only |
| `TestReplicaCeilingNotePlacement` | HPA capabilities unavailable; (a) no row, executive profile; (b) no row, detailed; (c) rows, executive; (d) rows, detailed | (a, b) fallback paragraph then the callout `Dados incompletos`; (c) table then the callout; (d) table, no callout |
| `TestNodepoolStatusRows` | pools `apps` (node `worker-a` with `NODE_MEMORY_USAGE_CRITICAL`) and `stable` (`worker-b`, no finding) | `apps` critical `Crítica` `Uso de memória requer atenção` before `stable` healthy `Estável` `—` |
| `TestNodepoolStatusKeepsTheHighestSeverityOnly` | pool `pool-a` with `NODEPOOL_CPU_UNEVEN` and `NODEPOOL_MEMORY_UNEVEN` (attention) and node `pool-a-1` with `NODE_CPU_REQUEST_HIGH` critical | `Requests de CPU elevados` alone, critical |
| `TestNodepoolStatusUndeterminedSortsByItsWord` | pools `unknown` (attention, `NODEPOOL_UNKNOWN`) and `zeta` (attention) | `Indeterminado` before `zeta` in Portuguese; `Undetermined` before `zeta` in English; and with pool `alpha` at attention, `alpha` before both |
| `TestNodepoolStatusNotFiltered` | minimum `critical`, a stable pool | the stable row prints |
| `TestActionPlanRows` | the recommendations of direct-complete (eight, as literals) | seven rows in the order of OR section 3 with the qualifiers `(namespaces)` on rows 1, 2, 6 and `(workloads)` on 3, 4, 7, and none on 5; scopes `Ident("payments")`, `Ident("payments-web")` |
| `TestActionPlanQualifiesOnlyIncludedCollisions` | the nineteen of edge-scaling, at attention and at critical | at attention row 3 reads `... (namespaces)`; at critical the same action has no qualifier |
| `TestActionPlanExcludesNodeAndPoolCodes` | `NODE_MEMORY_USAGE_CRITICAL`, `NODEPOOL_MEMORY_UNEVEN`, `NODE_AUTOSCALING_DISABLED_PRESSURE` | no row, the fallback |
| `TestActionPlanScopeSummary` | `DEPLOYMENT_WITHOUT_HPA` with four scopes `payments/app-1..4`; `NAMESPACE_CPU_REQUEST_ATTENTION` with four namespaces; `HPA_MAX_REPLICAS_REACHED` with four | placeholders `4 Deployments identificados`, `4 namespaces identificados`, `4 HPAs identificados` |
| `TestActionPlanScopeNames` | scopes `payments/api`, `payments/worker`; scopes `a/api`, `b/api` | `Ident("api, worker")`; `Ident("a/api, b/api")` |
| `TestActionPlanRefusesAnUnnameableScope` | a cluster recommendation with four scopes, written by hand | builder error naming `cluster` |
| `TestMaterialLimitationsRows` | the parity capabilities and warning | `Escalonamento automático de nós`-keyed row critical `Indisponível` with the reason; `Interpretação` attention `Parcial` with the warning |
| `TestMaterialLimitationsSkipsAWarningThatIsARemark` | `hpa_history` available with the warning `HPA_AMBIGUOUS_SPEC` | one row, the effective capability, no interpretation row |
| `TestMaterialLimitationsNotFiltered` | minimum `critical`, a partial capability | the partial row prints |
| `TestBuildOrdersTheSections` (extend) | the detailed profile | section 5 heading between section 4 and the weak page break; the appendix after it |
| `TestDocumentEqualsParityForItsSections` (extend) | the parity snapshot of `paritySnapshot`, changed as below | blocks 2 to 38 of the fixture and 42 to 43 equal |

The parity snapshot changes to hold the findings and the capabilities the
fixture's executive blocks need: the findings `WORKLOAD_CPU_REQUEST_CRITICAL`
on `payments/api` Deployment (average 88, peak 97),
`WORKLOAD_CPU_REQUEST_HEALTHY` on `payments/worker` (40, 50),
`HPA_NEAR_MAX_REPLICAS` on `payments/api` (current_average 7, spec_max
10), and `NODE_MEMORY_USAGE_ATTENTION` on `worker-a` (64, 80), in the order
`analysis.Sort` gives; the recommendations the two of `payments/api`;
the summary counts 4 findings, 1 critical, 2 attention, 1 healthy, 2
recommendations; the capabilities all available but `node_autoscaling`,
unavailable with the one reason `NODE_AUTOSCALING_UNDETERMINED`.
`parityTexts` gains the texts of the executive blocks (the headings, the
headers, the labels, `Eventos históricos indisponíveis.` →
`CapabilityReason("NODE_AUTOSCALING_UNDETERMINED")`, `Escalonamento
automático de nós` → `CapabilityName("node_autoscaling")`, `Parcial` →
`State("partial")`, `Uso de memória requer atenção` →
`FindingObservation("node_usage", "memory")`, the two actions →
`FindingAction("resource_request_usage", "cpu")` and
`FindingAction("hpa_near_max", "")`, `Máximo do HPA atingido`,
`Crítica` → `Priority("critical")`), and `parityComposed` the composed
texts (`CPU: pico de 97% da solicitação` → `Reason("request_peak", "cpu")`
with `97%`; `10 observadas / máximo 10` → `replicas_observed`; `2 ativas`
→ `active_replicas.plural` with `2`; `1 crítica` → `RiskCount("critical", 1)`;
the intro with `24 horas`). `Motivo` already maps to the node header; the
two keys read one word, and the test's comment says so.

- [ ] **Step 1:** Move `reasonFragment` and `evidenceNumber` to `reasons.go` without a change; run `go test ./internal/document/`; green. Commit `refactor(document): move the reason fragments`.
- [ ] **Step 2:** Write the reason tests; red; implement; green.
- [ ] **Step 3:** Write the executive tests; red (missing names); implement `executive.go` and the change of `build.go`; green.
- [ ] **Step 4:** Extend the parity test and the build order test; green. Run `go test -count=1 ./internal/catalog/`: `TestEveryKeyIsUsed` is green again.
- [ ] **Step 5:** `make lint`. Run `go test -count=1 ./...` and report which tests outside the package are red: expected `TestGoldenDataFiles` of `internal/goldentest` (the data files gain section 5), the tests of `internal/cli` that compare a PDF with a golden data file or count pages (`report_test.go:261,290,345`, `report_real_test.go:166,527`, see `go-interfaces.md` 10.1). The tagged tests of the command are Task 7's.
- [ ] **Step 6:** Commit.

```bash
git add internal/document
git commit -m "feat(document): print the executive summary as section 5"
```

- [ ] **Step 7 (main thread):** Run `make update-testdata`; read every changed data file: twelve files gain the blocks of section 5 and the appendix heading moves after them; compare the rows of `direct-complete.pt-BR.json` and `edge-scaling.pt-BR.json` with the rows of `.cache/phase-2/part-4/python/out/` by eye. Commit `test(golden): add section 5 to the data files`.

**Done when:** every test of the package passes, the parity test covers
blocks 2 to 38, the usage guard is green, and the data files are
regenerated and read.

---

### Task 7: The executive profile and the command (R62, R71)

**Executor:** Subagent.

**Files:**
- Modify: `internal/document/build.go`, `build_test.go`; `internal/cli/run.go`, `run_test.go`, `report_real_test.go`, `report_test.go`

**Interfaces:**
- Consumes: `buildExecutiveSection` (Task 6), `defaultOutputName` with the profile (Task 3).
- Produces: `Build` accepts the profile `executive`: the blocks are the
  front, the hard page break, `buildExecutiveSection(b, s, thresholds, severity, true)`,
  the weak page break, the appendix with the unraised severity, and the
  notes. A profile other than `detailed` and `executive` is still refused
  with `unknown report profile %q`. `executiveError` and its two calls
  go; `runPDF` and `runFromSnapshot` pass the profile to `defaultOutputName`.

**Tests:**

| Test | Input | Expected |
|---|---|---|
| `TestBuildExecutiveProfile` (build_test.go) | the parity snapshot, `Options{Profile: "executive"}` | the blocks: front, page break, the executive heading, ..., weak page break, appendix heading, notes; no heading of sections 1 to 4 |
| `TestBuildExecutiveRaisesTheMinimum` | the same with `MinSeverity: "all"` | no healthy row; the appendix keeps every capability (its rows are those of `all`) |
| `TestBuildRefuses` | `Profile: "x"` | `unknown report profile "x"`; the `executive` case is removed |
| `TestExecutiveProfile` (run_test.go, replaces `TestExecutiveIsRefused`) | the five former cases: flag, file, environment, from-snapshot flag, from-snapshot environment, each with `--output` into a temporary folder | exit 0 and a PDF; the five subtests of refusal go |
| `TestExecutiveOutputName` (run_test.go) | `--report-profile executive` without `--output`, in a temporary output directory | the file `report-resources-<cluster>-executive-<lang>-<stamp>.pdf` |
| `TestRealReportExecutiveProfile` (report_real_test.go, tagged) | `direct-complete`, both languages, `--report-profile executive` | the PDF text holds the executive heading and the appendix heading, and not the headings of sections 1 to 4; the outline on page 1 lists sections 1 and 2 only (the executive summary and the appendix) |
| `TestRealReportHasThreePages` (adapt) | `direct-complete` detailed | the page count the run now gives, with its reason written in the test: the opening page, sections 1 to 4, section 5, the appendix |
| `TestRealReportTextOfEveryScenario` (adapt) | | the outline holds `{5, SectionsExecutiveHeading}` and `{6, AppendixHeading}` |
| `TestFromSnapshotPrintsTheSections` (adapt) | | section 5 among the sections |

- [ ] **Step 1:** Document tests red; implement; green.
- [ ] **Step 2:** Command tests red; implement; green: `go test -count=1 ./internal/cli/` and `go test -count=1 -tags=template ./internal/cli/`.
- [ ] **Step 3:** `make lint`; `go test -count=1 ./...`: only `TestGoldenFoldersHoldNoStranger` and the golden page tests may be red, and only if Task 8 has not run; name them.
- [ ] **Step 4:** Commit.

```bash
git add internal/document internal/cli
git commit -m "feat(cli): accept the executive profile"
```

**Done when:** the executive profile writes a PDF in both languages from a
run and from a snapshot, and the tests of the command are green.

---

### Task 8: The proofs: the original recorded again, the executive data files, the comparison (R72, R73, R74, F-85)

**Executor:** Main thread records; a subagent writes the comparison and the data files' mechanism; the main thread regenerates.

**Files:**
- Modify: `testdata/python/*.json`, `testdata/python/README.md` (main thread); `internal/goldentest/goldentest.go`, `golden_test.go`, `original_test.go`; `cmd/update-testdata/main.go`; `template/tests/capacity/fixtures/command/` (four new files, main thread)

**Step 1 (main thread), the recording.** Build the fake server from the
head of the branch into `.cache/phase-2/part-4/python/fake-prometheus`,
run `.cache/phase-2/part-4/python/record_all.sh`, and copy the eight
files of `out/` over `testdata/python/<scenario>.json`. The eight query
lists are recorded again with `queries.py` and compared with the tracked
ones: unchanged. Rewrite the README: the new fields (`recommendations`:
code, severity, scope type, scopes, confidence, and the action text of the
original for reference; `summary`: the eleven counts the mapper writes;
`workload_coverage`: every row the mapper publishes with `observed_usage`
and `hpa_coverage`; `executive`: the rows of the seven tables at the two
effective minimums, each cell a list of runs read from the Typst snippets
of the row functions, as `record.py` parses them), the pydantic stand-in
(R74), the date, the commit of the fake server; the SHA-256 table. Read
every changed number against `original-recommendations.md` section 2 and
`original-executive.md`. Commit.

```bash
git add testdata/python
git commit -m "test: record the recommendations and executive rows"
```

**Interfaces** (subagent):

```go
// DataFile decodes an encoded snapshot and returns the encoded document
// in one language and one profile, with the severity stored in the
// snapshot. An empty profile keeps the snapshot's.
func DataFile(snap []byte, lang catalog.Language, profile string) ([]byte, error)

// ExecutiveDataFileScenarios names the scenarios that have golden data
// files of the executive profile too.
var ExecutiveDataFileScenarios = []string{"direct-complete", "edge-scaling"}

// ExecutiveDataFilePath returns the place of a scenario's golden data
// file of the executive profile, in one language:
// fixtures/command/<scenario>.executive.<lang>.json.
func ExecutiveDataFilePath(scenario string, lang catalog.Language) (string, error)
```

`cmd/update-testdata` writes the executive files after the detailed ones.
`TestGoldenDataFiles` compares both kinds; `TestGoldenFoldersHoldNoStranger`
expects the sixteen names; `template/capacity_test.go:29,56`
(`commandFixtureStems`) compiles every file of the folder and needs no
change but a count if it has one; `internal/cli/report_test.go:328` reads
`DataFileScenarios` and keeps doing so.

The recording structs of `original_test.go` gain:

```go
type pythonRecommendation struct {
	Code       string   `json:"code"`
	Severity   string   `json:"severity"`
	ScopeType  string   `json:"scope_type"`
	Scopes     []string `json:"scopes"`
	Confidence string   `json:"confidence"`
	Action     string   `json:"action"` // not compared
}
type pythonSummary struct { Namespaces, Workloads, HPAs, Nodes, Nodepools, Findings, Recommendations, Healthy, Attention, Critical, WorkloadsWithoutHPA float64 } // json names as the file
type pythonCoverageRow struct { Namespace, Kind, Name string; ObservedUsage bool `json:"observed_usage"`; HPACoverage string `json:"hpa_coverage"` }
type pythonRun struct { Kind string `json:"kind"`; Status string `json:"status"`; Text string `json:"text"` }
type pythonExecutive struct {
	CriticalWorkloads   [][][]pythonRun `json:"critical_workloads"`
	AttentionWorkloads  [][][]pythonRun `json:"attention_workloads"`
	NamespacesAtRisk    [][][]pythonRun `json:"namespaces_at_risk"`
	ReplicaCeiling      [][][]pythonRun `json:"replica_ceiling"`
	HPANote             string          `json:"hpa_note"`
	NodepoolStatus      [][][]pythonRun `json:"nodepool_status"`
	ActionPlan          [][][]pythonRun `json:"action_plan"`
	MaterialLimitations [][][]pythonRun `json:"material_limitations"`
}
```

and `pythonRecording` the fields `Recommendations []pythonRecommendation`,
`Summary pythonSummary`, `WorkloadCoverage []pythonCoverageRow`,
`Executive map[string]pythonExecutive` (keys `attention`, `critical`).

`compareNumbers` gains, after the findings:

- `compareRecommendations(scenario, got []snapshot.Recommendation, want []pythonRecommendation)`:
  matched by code and scope type; severity, scopes (as lists), and
  confidence compared; a missing or extra entry named; the order of the
  codes compared when the sets agree. Messages in the style of
  `compareFindings`: `%s: recommendation %s (%s): scopes: got %v, want %v`.
- `compareSummary(scenario, got snapshot.Summary, want pythonSummary)`:
  the eleven counts, `%s: summary.%s: got %v, want %v`.
- `compareCoverage(scenario, got []snapshot.Workload, want []pythonCoverageRow)`:
  every row by namespace, kind, name: `observed_usage` and `hpa_coverage`;
  the order of the keys; missing and extra rows.
- `compareExecutive(scenario, snap snapshot.Snapshot, want map[string]pythonExecutive)`:
  for each of `attention` and `critical`, build the Portuguese document
  with `document.Build(snap, cat, document.Options{Profile: "executive", MinSeverity: <that>})`,
  walk its blocks from the executive heading, and for each of the seven
  subsections find its heading by the catalog text and take the block
  after it: a `Table` means rows, a `Paragraph` means none. Compare each
  table with the recorded rows: the count of rows; for each cell the
  count of runs; for each run the kind, the status, and, for an `ident`
  run, the text letter for letter; for a `text`, `placeholder`, or
  `status` run, the number tokens of the text, `[0-9]+(,[0-9]+)?%?` in
  order, and for the action cell of the plan whether the text ends in `)`.
  The note: a `Callout` after the replica ceiling table or after its
  fallback paragraph means `partial` or `incomplete` by its title text,
  else the empty text; compared with `hpa_note`. Messages name the
  subsection, the row, the cell, and the run.

`TestComparisonCanFail` gains five cases: a recommendation's scope renamed
in the recording; `summary.recommendations` set to one more; a coverage of
a row set to `covered`; an ident of the action plan renamed; the number
token of a replicas cell changed.

| Test | Input | Expected |
|---|---|---|
| `TestNumbersEqualTheOriginal` | eight scenarios | zero differences, the four new comparisons included |
| `TestComparisonCanFail` | the five cases above | each the one expected message |
| `TestGoldenDataFiles` | the six detailed and the two executive scenarios, both languages | equal to the files |
| `TestGoldenFoldersHoldNoStranger` | | the sixteen stems |
| `TestRecordedFilesAreUntouched` | | the SHA table of the new README |

- [ ] **Step 1 (main thread):** record and commit, as above.
- [ ] **Step 2 (subagent):** write the comparison tests and the data-file mechanism; red; implement; green on `go test -count=1 ./internal/goldentest/` except `TestGoldenDataFiles` and the folder test, which wait for the files; `make lint`; commit.

```bash
git add internal/goldentest cmd/update-testdata
git commit -m "test: compare the recommendations and executive rows"
```

- [ ] **Step 3 (main thread):** `make update-testdata`; read the four new files whole; `go test -count=1 ./...`: green. Commit `test(golden): add the executive data files`.

**Done when:** the comparison reads every section of the recording, zero
differences on eight scenarios, and the sixteen data files are in place.

---

### Task 9: Golden pages and a PDF from each scenario

**Executor:** Main thread.

- [ ] **Step 1:** `open -a Docker`, wait for `docker info`; `make image`; `make test-template`. Every data file compiles with no error and no warning.
- [ ] **Step 2:** Adapt `.cache/phase-2/part-3/python/make_pdfs.sh` into `.cache/phase-2/part-4/python/make_pdfs.sh`, writing the sixteen detailed PDFs and sixteen executive PDFs (`--report-profile executive`) into `.cache/phase-2/part-4/pdf/`, each run against a fresh fake server. Count the pages of each.
- [ ] **Step 3:** Choose the golden pages after the files exist. The command snapshots `command-complete-pt`, `command-complete-en`, `command-partial-en`, `command-edge-pt`, `command-scaling-pt`, `command-scaling-en` keep their names and get the page lists of the new documents, their `Protects` naming section 5 where it stands and their `Omits` saying why a page is left out; two new entries: `command-executive-pt` from `edge-scaling.executive.pt-BR.json`, every page, protecting the executive profile in Portuguese: no section 1 to 4, the replica ceiling with an uncovered Deployment without pods, the note after the table, the undetermined pool in the situation table, the plan with a qualifier, the two material limitations; `command-executive-en` from `direct-complete.executive.en.json`, every page, protecting the English headers of the seven tables and the fallback sentences of the empty subsections. The `Protects` text of `command-scaling-en` says section 5 instead of "section 5 move to pages 3 and 4" where that sentence stood for the appendix.
- [ ] **Step 4:** `make update-golden`. Open every page that changed or is new and read every header, cell, note, and fallback against the tables of this plan and the rows of `.cache/phase-2/part-4/python/out/`. Write one line per page to `.cache/phase-2/part-4/pages.md`. The pages of `capacity-parity`, `capacity-pt`, `capacity-en`, `capacity-minimal`, and `capacity-identifiers` did not change.
- [ ] **Step 5:** `make test-golden` twice in a row; the log names every command snapshot.
- [ ] **Step 6:** Commit.

```bash
git add template/golden/snapshots.go template/tests/golden
git commit -m "test(golden): add the pages of section 5 and the profile"
```

**Done when:** every page was read, the golden gate passed twice, and the
parity pages are unchanged.

---

### Task 10: Documents

**Executor:** Subagent for the text; the main thread reads it back.

**Files:**
- Modify: `README.md` (the executive profile under the four modes; "The report holds four sections, numbered 1 to 4, and the appendix, 5" becomes five sections and the appendix, 6, with section 5 described: the workloads with critical and high priority, the namespaces at risk, the workloads at their replica maximum and without HPA, the situation of the node pools, the prioritized action plan, the material limitations; the executive profile prints the opening page, section 5 as its only section, and the appendix, with the minimum severity raised to at least attention; the default name with `-executive`; the snapshot holds the recommendations), `AGENTS.md` (the sentence that the snapshot of schema `1.0` holds every section; the executive profile among the rules kept by review: "The executive body raises the minimum to attention for its own rows and never for the appendix"; the recording script of part 4 and the pydantic stand-in under the golden snapshots section), `CHANGELOG.md`

**The changelog**, under `Unreleased`, `Added`:

- The report holds an executive summary, section 5: the workloads with critical and high priority, the namespaces with a workload at risk, the workloads at their replica maximum and without HPA, the situation of the node pools, the prioritized action plan, and the material limitations.
- The `executive` report profile writes the opening page, the executive summary, and the quality appendix, with the minimum severity raised to at least attention; the default name of its PDF ends in `-executive` in place of the severity.
- The snapshot file holds the recommendations and their count.

Under `Fixed`:

- A threshold or a timeout that is not a finite number is refused by the configuration.
- The host of an IPv6 instance keeps the letter case of its zone when it is matched to a node.

Left out, and said in the report: the finding sentences of the catalog
(no page prints a summary other than the two of part 2.3; the actions
appear through the plan, which the first entry names), the clock of
`--check`, the comparison with the original, the recorded files, the
golden files and pages.

- [ ] **Step 1:** Write. Run `go test -count=1 ./internal/guard/ -run Changelog`.
- [ ] **Step 2 (main thread):** Read the three files back.
- [ ] **Step 3:** Commit.

```bash
git add README.md AGENTS.md CHANGELOG.md
git commit -m "docs: describe the executive summary and profile"
```

**Done when:** the three files say what the program does.

---

### Task 11: The run against a real cluster (R77; Task 17 of part 2.1, F-35, F-66)

**Executor:** The owner, and the main thread with the owner's approval for
each command. This task reaches a real system with a real credential.

**Requirements:**

- The owner provides a `report.toml` outside the repository and names the
  variable that holds the token. The token is never written to a file of
  the repository, to `.cache/`, or to a message; the redactor removes it
  from every line the program prints.
- The runs are `--check --verbose` first, then `--snapshot`, then
  `--from-snapshot` in both languages and both profiles.

- [ ] **Step 1:** Ask the owner for the path of the configuration and the name of the variable; or record the owner's word that the run waits, and stop here.
- [ ] **Step 2:** `bin/k8s-capacity-report report --config <path> --check --verbose`. Record the lines (redacted) in `.cache/phase-2/part-4/real/check.txt`.
- [ ] **Step 3:** `--snapshot .cache/phase-2/part-4/real/cluster.json`, then four PDFs from it. Read the snapshot for any address, host name, or provider identifier (spec 8.3): finding one is a defect.
- [ ] **Step 4 (F-35):** Measure the size of the largest answer: run the snapshot through a local logging proxy under `.cache/phase-2/part-4/real/proxy.py` (from the one of part 2.3's review), which records the length of every answer and nothing of its content; the token passes through the proxy's memory and is not logged. Record the largest length and the period, and compare with the 256 MiB limit of R19. If the largest answer is within a tenth of the limit, record a finding.
- [ ] **Step 5 (F-66):** From the snapshot, list the longest workload, HPA, and node names and whether any has no hyphen and more letters than its cell can hold (the fixture of F-66 broke at 253 letters; the cells of sections 2 and 5 hold about 30 letters of the identifier font). Put the list before the owner with the question of F-66: break a name with no hyphen, which changes the parity pages, or keep the limit of spec 10.4.
- [ ] **Step 6:** Record every finding in `ROADMAP.md` with the source `PROD`. A finding from this task takes precedence over one from reading code.

**Done when:** the runs ended and the two measurements are recorded, or
the owner decided that the run waits, and either outcome is in
`ROADMAP.md`.

---

### Task 12: Close the part

**Executor:** Main thread.

- [ ] **Step 1:** Hand the wording of both languages to the owner as `.cache/phase-2/part-4/wording.md`: the entries of doubt first (`replicas_observed` and `active_replicas` with the masculine agreement, `header_cpu_limit` and `header_memory_limit` with `Limit` for `Lim`, `scope_summary.deployment` with `Deployments` capitalised, `hpa_divergence` with `desejados`, `autoscaling_bounds_unknown` and `autoscaling_enabled` rewritten around `limites`, `resource_limit_usage.cpu` with `throttling de CPU`, and the open F-90 and F-91), then every entry of Tasks 4 and 5, then the complete catalog of both languages as one table, with the thirty-two PDFs. Apply what the owner changes, run `make update-testdata` and `make update-golden`, and review what changed.
- [ ] **Step 2:** Write the diff of the branch to `.cache/phase-2/part-4/branch.diff`. Run the adversarial review by area, one reviewer for each, with differential runs against the original through `.cache/phase-2/part-4/python/record.py` and the fake server: the recommendations and the snapshot; the catalog and the document; the command, the golden files, and the recording; the three closures of Task 3. Each writes its findings to a file under `.cache/phase-2/part-4/adversarial/` and repeats them in its last message.
- [ ] **Step 3:** Record every finding in `ROADMAP.md`. Fix the ones assigned to this part, each with a brief that names the class. Run the review again over the fixes.
- [ ] **Step 4:** Run `make gate`. Keep the logs of `.cache/gate/` in `.cache/phase-2/part-4/gate/`.
- [ ] **Step 5:** Update `ROADMAP.md`: every item `DONE` with its commit; F-83, F-85, F-88, F-92 `DONE`; F-35, F-66, and Task 17 of part 2.1 by the outcome of Task 11; the summary line of 2.4; the rulings R61 to R77 and the rulings of the execution, with what the owner said of each. Read `ROADMAP.md` and `CHANGELOG.md` back.
- [ ] **Step 6:** Write the closing report to `.cache/phase-2/part-4/closing-report.md` and ask the owner for the merge and for the decision on the version: part 2.4 completes the snapshot of schema `1.0`, so a version and a tag may be asked for. The recommendation is written in the report with its reasons: the standalone command is whole, the schema is whole, the proof on eight scenarios holds every section; against it, the real cluster of Task 11 and the open wording.
- [ ] **Step 7:** After the owner approves, merge by fast-forward; if a version is decided, write it to `VERSION`, head the changelog with it and the date, and tag after `make gate` passes on that commit, with the logs under `.cache/release-X.Y.Z/`.

```bash
git switch main
git merge --ff-only feat/executive-profile
```

**Done when:** `make gate` passed on the last commit, the review has no open
finding assigned to this part, and the owner approved the merge.

---

## Self-review of this plan

| Requirement of the spec | Task |
|---|---|
| 5.1 the four modes with the executive profile; 5.2 `report.profile` | 7 |
| 5.3 exit code 2 for a configuration that is not a number | 3 (R69) |
| 7.3 the executive profile raises the minimum to attention; the minimum is applied when shown, never when stored | 6, 7 (R61, R62) |
| 7.4 recommendations: grouped by code above healthy, scopes merged, highest severity, most exact confidence, ordered by severity, confidence, scope, code | 1, 2 (R63) |
| 8 `recommendations`, `summary` with every count; 8.3 integrity | 2 (R63, R75) |
| 9.1 finding sentences and action sentences by family; short fragments; placeholders | 4, 5 (R64, R66, R67) |
| 9.3 the vocabulary, one entry per resource; the em-dash alone | 4, 5 |
| 9.5 a changed sentence is a minor version | 12 |
| 10.2 the data file strict in both directions | 6, 8 |
| 10.3 the detailed and executive profiles keep the section order, tables, columns, sort orders, and fallbacks of the original | 6, 7 |
| 13.2 data file goldens per language and profile; the comparison with the original | 8 |
| 13.3 golden pages in both languages, each with the behaviour it protects | 9 |
| 15 changelog | 10 |
| 16 a PDF from each scenario, then from a real cluster | 9, 11 |
| 17 the English wording reviewed before phase 2 closes; the numbers equal the original | 12, 8 |
| Review focus 1 | 6 (`TestPriorityRowsTellHomonymsApart`) |
| Review focus 2 | 6 (`TestReplicaCeilingUnidentifiedTarget`) |
| Review focus 3 | 6 (`TestActionPlanQualifiesOnlyIncludedCollisions`), 8 |
| Review focus 4 | 6 (`TestNodepoolStatusKeepsTheHighestSeverityOnly`), 8 |
| Review focus 5 | 6 (`TestPriorityRowsWithoutPods`) |

Placeholders: none. Every task names its files, its signatures, its texts,
and its tests with inputs and expected results.

Types across tasks: `analysis.Recommendation`, `Recommendations`, and
`ConfidenceRank` (Task 1) are used by the snapshot (2) and the document
(6); `snapshot.Recommendation` (2) by the document (6) and the comparison
(8); `FindingSummary`, `FindingAction`, `FindingObservation`, `FindingPairs`
(4) and `Priority`, `RiskCount`, `ScopeQualifier`, `ScopeSummary`, the
extended `Reason` (5) by the document (6); `buildExecutiveSection` (6) by
`Build` in both profiles (7); `DataFile` with a profile (8) by
`cmd/update-testdata` (8); `defaultOutputName` with a profile (3) by the
command (7).

Order of the red suite: the golden snapshots between Task 2's code and its
regeneration; `TestEveryKeyIsUsed` from Task 4 to Task 6; the data files
between Task 6's code and its regeneration; the executive data files and
the folder test between Task 7 and Task 8; the golden pages from Task 6 to
Task 9. Each is named in its task.
