# Phase 2.3: Autoscalers and Nodes Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** `k8s-capacity-report report` collects the HPAs, the workloads without an HPA, the nodes, the node pools, and the cluster autoscaling over the period, judges them with the 26 remaining finding codes, stores them in the snapshot, and prints sections 3 and 4 of the report, in Portuguese (Brazil) under the wording rule and in English, with the same numbers as the Python tool, whose queries it now sends to the last one.

**Architecture:** The third thread through every layer of spec section 4.3: three collectors and two resolvers join the packages of part 2.2, the analysis gains the node, pool, HPA, and autoscaling rules and the node pressure, the snapshot gains `summary`, `hpas`, `nodes`, `nodepools`, the coverage of each workload, and evidence values of three kinds, and the document gains two sections built from six tables that the template already lays out. Two defects found by reading enter first: the average of a series is a plain sum where the original's is compensated (F-76), and the value of an evidence is a number only where the original writes a boolean and a text (R20).

**Tech Stack:** Go 1.27 and the modules already in `go.mod`. No new module. Typst 0.15.1, `pdftotext`, Docker. Python 3.14.7 in the environment of part 2.2, on the main thread, to record the original again (Task 16).

**Spec:** `docs/superpowers/specs/2026-09-27-k8s-capacity-report-design.md`, sections 5.3, 6.3, 6.4, 7.1 to 7.3, 8, 9.1 to 9.3, 10.2, 10.3, 13.2, 13.3, 15, 16, and 17. The decisions of the owner of 2026-09-30 are in `ROADMAP.md`.

**Depends on:** Part 2.2b, merged into `main` at `995fd89`.

## Where this part sits

| Part | Delivers | PDF at the end |
|---|---|---|
| 2.1 Spine, merged | Configuration, redaction, client, discovery, the fake server, the catalog mechanism, the four modes of `report` | Cover, opening page, quality appendix |
| 2.2 Namespaces and workloads, merged | Range queries, two collectors, workload identity, 40 codes, five formats, the comparison with the Python tool | Sections 1 and 2, interpretation notes |
| 2.2b The wording rule, merged | Technical terms in English in both languages; the guard that keeps it | The same pages |
| **2.3 Autoscalers and nodes (this plan)** | Three collectors, two resolvers, 26 codes, the second insufficiency rule, `summary`, `hpas`, `nodes`, `nodepools`, coverage, the period format, the last queries of the original | Sections 3 and 4 |
| 2.4 Executive profile and proofs | Recommendations, the minimum severity of the executive body, the complete catalog, the finding sentences, a real cluster | Both profiles |

### Out of this part

| Left out | Arrives in | Reason |
|---|---|---|
| `recommendations` and the count `summary.recommendations` | 2.4 | A field enters with the part that fills it (R10) |
| The sentences of the findings, other than the two that section 4 prints | 2.4 | No page of this part prints them (R21, R43) |
| The executive profile and its eight tables | 2.4 | R3 of part 2.1 stands |
| The run against a real cluster; the measurement of F-35 | 2.4 | The owner's decision of 2026-09-29 |
| F-66, a name with no place to break | 2.4 | The owner's decision of 2026-09-30 |
| A change to the template | none | The five tables of sections 3 and 4 exist in `template/capacity/tables.typ` since phase 1, and the parity pages must stay equal to the original's |

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
- A reviewer receives a diff file under `.cache/phase-2/part-3/` and runs
  tests only on packages nobody is editing.
- Every commit names its paths: `git add <paths>`, never `git add -A`.
- A subagent never runs `git stash`, `git restore`, `git checkout` of a
  path, or `git clean`.
- A test of a change goes red on the code before the change, test by test,
  and the brief asks for the red run.
- A brief that says "report every X" also lists, as tests, the cases that
  must not match.
- A subagent has no `ReportFindings` tool. A reviewer writes its findings to
  a file under `.cache/phase-2/part-3/` and repeats them in its last message.
- A decision flagged at the end of a report has already been made. A brief
  that says "stop and report" means stop before deciding.
- When a brief lists the tests that fail after a change, it names every
  reader of the artefact that changes, found by grep, not the one test that
  owns it.

Reference material, read-only and never tracked:

- `older/kubernetes-report/kube_capacity_report/`: the Python source, which
  is the reference for every detail of behaviour that this plan does not
  repeat. Inside `older/`, nobody reads `.env`, `.envrc`, `report.toml`,
  anything under `deploy/secrets`, `.venv`, `.git`, `.worktrees`, `build`,
  or `node_modules`.
- `.cache/phase-2/part-3/research/original-collection.md`,
  `original-analysis-snapshot.md`, `original-document.md`, and
  `go-interfaces.md`: the facts extracted for this part, with file and line
  for each. A brief quotes the lines it needs; an implementer that finds
  the brief and the original in disagreement stops and reports.
- `testdata/python/<scenario>.queries.txt`: every query the original sends,
  in order. Lines 44 to 69 of `direct-complete.queries.txt` are the queries
  of this part.

## Global Constraints

Every constraint of the plans of parts 2.1, 2.2, and 2.2b stands. The ones
below are added or are the ones this part leans on most.

- Go 1.27. Module `github.com/marceloalvescf/k8s-capacity-report`. No new
  module enters `go.mod`. The one new package, `internal/arith`, enters
  `guard.Layers` at rank 0.
- Metrics only. Imports point one way. The snapshot is the only handover.
- Every query is `POST` with a form-encoded body, on `/query` and
  `/query_range`. The text of every query equals the original letter for
  letter, and the collectors send them in the order of the original:
  namespaces, workloads, HPAs, the coverage inventory, nodes, cluster
  autoscaling. After this part the list of queries of each scenario equals
  the recorded list of the original, end to end.
- Inside a selector the matchers of one group are sorted by label name, so
  the scope label is not always first: `{cluster="prod",condition="Ready"}`,
  `{cluster="prod",resource="cpu"}`.
- A sample that is `NaN` or infinite is "no data". It is never zero. A
  measured zero is a number. A label that is absent reads `unknown`; a
  label that is present and empty stays empty; the target labels of an HPA
  default to the empty text.
- A ratio is computed sample by sample and then reduced. The request
  percent of a node is the one exception the original makes: one ratio of
  two instant values, requested over allocatable, with no denominator of
  zero.
- Where two series share a key and a timestamp, the collectors of this part
  keep the maximum, not the sum. The autoscaling collector sums across
  groups the current value of each group, and only there.
- **The average of a series is a compensated sum divided by the count**, the
  sum written as CPython writes its builtin `sum` of floats (F-76). Every
  mean of this part uses it.
- The numbers equal the original exactly: the same `float64`, no tolerance.
  Counts of the autoscaler are truncated toward zero as `int()` does, and
  stored as whole numbers.
- The 26 finding codes are the strings of the original, unchanged. A
  finding carries no sentence. An evidence value is a number, a boolean, or
  a text, and the registry names the kind of each.
- The snapshot holds codes and numbers. It holds no address, no host name,
  no provider identifier, and no token: the internal IP, the hostname, and
  the provider id of a node never leave the collector.
- Every output list is sorted: HPAs by namespace and name; nodes by name;
  pools by name, `unknown` among them; workloads by namespace, name, kind.
  Warnings and notices keep first-seen order without duplicates, in the
  order namespace, workload, hpa, hpa coverage, node.
- Wording lives only in `internal/catalog`. Every technical term is English
  in both languages; only a common noun is Portuguese (spec 9.3). A noun is
  never pasted into a sentence: one entry per resource.
- Rounding follows the original: a replica count and a node count are
  rounded half to even; the replica average has one decimal; percentages
  one; cores and bytes two; trailing zeros removed.
- No em-dash inside a sentence. The em-dash alone marks an empty table cell.
- The same input gives the same snapshot and the same `report.json`, byte
  for byte. `report.json` is strict in both directions: a field enters the
  contract with the feature that prints it; a table with no row is left
  out.
- The files of `testdata/prometheus/`, `testdata/prometheus-own/`, and
  `testdata/python/` are recorded answers. They are never edited; a file of
  `testdata/python/` is replaced only by recording the original again,
  whole. The parity fixture and its pages do not change.
- `--check` sends no query of a collector. The clock is read twice in a run
  that collects. Every failure prints one line `error: <message>` on
  standard error.
- No test skips. A test never writes a golden file. No comment speaks of
  the process. Diagnostics are in English.
- Scratch files go under `.cache/phase-2/part-3/`, never under `/tmp`.
- Commits follow `type(scope): description`, imperative, subject under 50
  characters.

## Review Focus

Each line names an input that the spec implies and does not describe, and the
task whose tests pin it.

1. **Two nodes with one internal IP.** The index of the resolver drops a
   value that two nodes share, so a utilisation series that names only that
   instance resolves to no node and is left out; neither node gets the
   other's numbers. Pinned in Task 3 and Task 6.
2. **An HPA whose maximum is zero.** With `spec_max` 0 and a current peak of
   0, the original emits `HPA_MAX_REPLICAS_REACHED` at critical, because
   that rule has no guard on zero where the near-max rule has one. The port
   reproduces it. Pinned in Task 9.
3. **Autoscaler series without a group label.** With no `node_group`,
   `nodegroup`, `node_group_name`, or `node_group_id` label, the series of
   one metric are merged by maximum at each timestamp, not summed; with
   some grouped and some not, the ungrouped ones are ignored. Pinned in
   Task 7.
4. **A node known only by its requests.** A node that appears in the
   requests series and in no `kube_node_info` series is a row with an empty
   identity, marks the collection partial, and, with no capability reason
   to say why, adds the warning that the capacity evidence is incomplete.
   Pinned in Task 6.
5. **A series of three samples.** `3.3`, `1.1`, `2.2` average to `2.2` in
   the original and to `2.1999999999999997` in a plain loop; every scenario
   of part 2.2 had two samples, where both agree. Pinned in Task 1, and
   against the original in Task 16 through the scenario of Task 15.

## Rulings made on the owner's behalf

Each of these is a decision the spec leaves open. Each is written as
decision, reason, and cost if wrong. The owner may overturn any of them when
reviewing this plan. The numbering continues that of part 2.2; W1 to W9 are
the rulings of part 2.2b.

| ID | Ruling |
|---|---|
| R36 | Ruling: the compensated sum lives in a new package `internal/arith` at rank 0, `Sum` and `Mean`, and both `internal/collect` and `internal/document` use it — the collectors average series and the document averages nodes, and a document that imports `collect` for one function blurs the handover — one package of two functions |
| R37 | Ruling: an evidence value is a number, a boolean, or a text, written in JSON as its own kind (`4`, `true`, `"cpu,memory"`), and the registry names the kind of each evidence name; `resources` stays the text the original joins with a comma, `node_pressure` a boolean, `affected_nodes` a number — the recorded files of the original hold exactly these three kinds, and a list of names, which the plan of 2.2b proposed, exists nowhere in the original — a reader of the snapshot splits one text at a comma |
| R38 | Ruling: `summary` enters now with every count but `recommendations`, and with `autoscaling` as a struct that is never null — each count is a fact of this part's data, the autoscaling evidence feeds seven findings, and a summary of counts that lies about recommendations would be worse than one that lacks the key — one more key in part 2.4 |
| R39 | Ruling: the workloads of the snapshot are the measured rows and, after them in the same sorted list, one row for each uncovered workload that no collector measured, with `observed_usage` false, no pods, and ten unavailable statistics; every row carries `hpa_coverage` as `covered`, `uncovered`, or `unknown`; section 2 prints the observed rows only — this is the shape of the original's snapshot, and it lets section 3 and the web view of phase 4 read one list — a consumer must filter by `observed_usage` |
| R40 | Ruling: the four warnings and the one notice of this part get the codes `HPA_AMBIGUOUS_SPEC`, `HPA_AMBIGUOUS_TARGET`, `NODE_IDENTITY_UNAVAILABLE`, `NODE_CAPACITY_EVIDENCE_INCOMPLETE`, and `WORKLOADS_OMITTED_SCALE_ZERO` with the parameter `count` — the original has sentences only (R22) — a code chosen now is kept for as long as schema `1.0` is supported |
| R41 | Ruling: the period is formatted by `Catalog.Period`, which computes the value and the unit as the original does and reads the eight unit words from the catalog, `period.<unit>.singular` and `.plural`, plural for every value but 1 — spec 9.2 puts the format in Go and spec 9.4 puts every reader word in the catalog — none known |
| R42 | Ruling: the callout of section 3 has the kind `note`, the default of the generic layer, which the original calls without a kind — none |
| R43 | Ruling: the two sentences that the reason column of section 4 prints for `NODE_OVERREQUESTED` and `NODE_LOW_UTILIZATION` enter the catalog now as `finding.node_overrequested.summary` and `finding.node_low_utilization.summary`, and the document strips the trailing periods as the original does; the fragments of the other node codes are `reason.*` entries with the percent as a parameter and one entry per resource — R21 lets a sentence in with the page that prints it, and part 2.4 adds the other families beside these two — a rename if 2.4 names the families otherwise |
| R44 | Ruling: the situation "Dentro do limite" of an HPA is written "Abaixo do máximo" in Portuguese, because the guard bans `limite` in every sense (X2), and "Below the maximum" in English — a departure of one cell from the original's words, already a consequence of the wording rule |
| R45 | Ruling: the second rule of insufficient capabilities is `collect.CheckUseful`, called after the five collectors and before the analysis, in the command and in `goldentest`; it fails with the message of the original, `no useful report sections were collected`, exit code 5, when no result of the five holds a row with a state other than unavailable and the autoscaling status is `unknown` (R28) — spec 5.3 gives the code, the original gives the rule — none |
| R46 | Ruling: the document gains a `Callout` block and a `Placeholder` run, which the template already validates and renders; the template is not touched — spec 10.2 lists both — none |
| R47 | Ruling: an eighth scenario, `edge-scaling`, is written by this project under `testdata/prometheus-own/`, with the values at the edges of this part that no recorded answer holds, `edge-values` staying as it is because its numbers are recorded — R24 stands — one more scenario to keep |
| R48 | Ruling: `hpa_coverage` of a workload row is `covered` when a reliable HPA target names it, `uncovered` when the coverage inventory lists it, `unknown` otherwise, which includes every workload of a namespace with an ambiguous HPA target — this is the original's derivation — none |
| R49 | Ruling: an HPA whose current peak is 0 is stored in the snapshot and hidden from the table of section 3, as the original hides it — storage is never narrowed (spec 8.3) — none |
| R50 | Ruling: the integrity rule that a pool names no absent node runs when the snapshot is written as well as when it is read; the original checks it only when reading — spec 8.3 lists it among the writer's checks — none |
| R51 | Ruling: the request percent of a node is stored as one number or null, `cpu_request_percent` and `memory_request_percent`, not as a statistic, because the original computes one ratio and leaves the other three numbers empty — a statistic of one number would invent three — the comparison with the original reads `.average` of the recorded statistic |
| R52 | Ruling: `ready` and `schedulable` of a node are `true`, `false`, or `null`, and the key walk of the snapshot learns `*bool` — the original writes the three — none |
| R53 | Ruling: the pool named `unknown` is stored with that name and printed as `Indeterminado` or `Undetermined`; it sorts among the other names, as in the original — spec 9.3 — none |
| R54 | Ruling: a run that collects prints, after the two lines of part 2.2, `hpa collection: <state>`, `hpa coverage collection: <state>`, `node collection: <state>`, and `autoscaling collection: <status>`, the last one `enabled`, `disabled`, or `unknown`, as the original prints them (R34) — none known |
| R55 | Ruling: section 3 walks the namespaces of the scope of the snapshot, sorted and unique, as sections 1 and 2 do; the original walks the configured list in its order, repeats included — one order for the whole document — a page differs from the original's when the configuration lists namespaces unsorted or twice; the fixed input has one namespace |
| R56 | Ruling: the original's rule that turns an available `hpa_history` or `workloads_without_hpa` into a partial one with the ambiguity warning as its remark, on the opening page, in the appendix, and in the note of section 3, is applied by the document from the warnings of the snapshot; the capabilities of the snapshot stay as discovered — the snapshot stores facts, the document interprets them, and the rule is reachable only now that the warnings exist — none |
| R57 | Ruling: the analysis of the findings keeps the sort of part 2.2, severity descending, scope name, scope kind, code; the original sorts by severity, scope name, code and keeps the concatenation order for ties — the comparison of Task 16 checks the order with the kind left out, as part 2.2 did — a difference of order between two findings of one scope name and two kinds, which the recorded scenarios will show if it exists |
| R58 | Ruling: the eight scenarios are recorded again, whole, the seven files of `testdata/python/` replaced and the SHA-256 table of its README rewritten; the test that compared the beginning of the query lists compares the whole lists — a file is replaced only whole — every changed number is read before the commit |
| R59 | Ruling: the node collector, like the original, always sends its nine fixed queries, and skips the labels, the requests, and the utilisation queries only when their capability is unavailable; the autoscaling collector sends its six queries in `auto` mode whatever the state of `node_autoscaling` — the queries of the original — none |
| R60 | Ruling: the HPA, coverage, and node collectors add the reason codes of their capabilities to their warnings, as the original adds the reason sentences: `hpa_history` for the history, `workloads_without_hpa` for the coverage, `node_utilization`, `nodepool_discovery`, and `requests_limits` for the nodes, each code once; a warning of the snapshot may therefore carry a reason code, the snapshot accepts `discovery.ReasonCodes` beside `collect.WarningCodes` there, and the notes render such a warning with the entry of the reason — the original prints these sentences among the interpretation notes and reads them to mark a capability partial (R56), and the namespace and workload collectors of the original add none, so part 2.2 was right to add none — a note that repeats the remark of the appendix, as the original's does |

## Findings this plan closes or records

| Finding | How |
|---|---|
| F-62 | Closed by Task 13: the parity test of the document compares the notes through the mapping of 2.2b |
| F-76 | Closed by Task 1 and proven against the original by Tasks 15 and 16 |

## File structure

| Path | Responsibility in this part |
|---|---|
| `internal/arith/sum.go`, `sum_test.go` | `Sum`, `Mean`: the compensated sum |
| `internal/guard/imports.go` | `internal/arith` at rank 0 |
| `internal/analysis/analysis.go` | `Value`, `Evidence`, the new scope types, `All` with six groups, `NodePressure` |
| `internal/analysis/registry.go` | `EvidenceSpec`, `Severities`, the 26 definitions |
| `internal/analysis/hpas.go`, `nodes.go`, `pools.go`, `autoscaling.go` | The rules of the 26 codes |
| `internal/resolve/node.go`, `nodepool.go` | Node identity index, `InstanceHost`, node pool identity |
| `internal/collect/gauges.go` | `ReduceMax`, `InstantValues`, `Percent`, `Number`, `Flag` |
| `internal/collect/hpas.go`, `nodes.go`, `autoscaling.go` | The three collectors and `Pools` |
| `internal/collect/collect.go` | `Collected` with six results, `All`, `CheckUseful`, the new codes |
| `internal/collect/statistics.go` | The average through `arith.Sum` |
| `internal/snapshot/snapshot.go`, `build.go`, `codec.go` | `Summary`, `HPA`, `Node`, `NodePool`, `Autoscaling`, `Value`, coverage; the rules; the walk |
| `internal/catalog/keys.go`, `format.go`, `locales/*.toml` | The keys of this part, `Period`, `Confidence`, the entries |
| `internal/document/document.go` | `Callout`, `Placeholder` |
| `internal/document/hpas.go`, `nodepools.go` | Sections 3 and 4 |
| `internal/document/build.go`, `front.go`, `appendix.go` | The block order; the effective capabilities (R56) |
| `internal/cli/run.go` | The four lines; exit code 5 on `ErrNoUsefulSections` |
| `internal/goldentest/` | Scenario `edge-scaling`; the recording structs; the queries compared whole |
| `testdata/prometheus-own/edge-scaling.json`, `README.md` | The eighth scenario |
| `testdata/python/` | Eight recorded files, eight query lists, the README |
| `template/golden/snapshots.go` | The golden pages of sections 3 and 4 |
| `README.md`, `AGENTS.md`, `CHANGELOG.md` | The documents |

---

### Task 0: Open the part

**Executor:** Main thread.

- [ ] **Step 1:** Run `git status`, `git log --oneline -3`, and
  `go test -count=1 ./...`. Read `ROADMAP.md` and `CHANGELOG.md` and confirm
  they describe what the commands show: `main` at `995fd89`, parts 2.2 and
  2.2b merged, F-74 and F-75 done, F-76 open.
- [ ] **Step 2:** Confirm the branch.

```bash
git branch --show-current
```

Expected: `feat/autoscalers-nodes`.

- [ ] **Step 3:** In `ROADMAP.md`, set the summary row of 2.3 to this plan
  and the branch, add the section `## Phase 2.3. Autoscalers and nodes —
  OPEN` with one row per task, identifiers `P23-0` to `P23-19`, source
  `PLAN`, and record under the decisions of the owner what the owner said of
  the rulings R36 to R59.
- [ ] **Step 4:** Commit the plan.

```bash
git add docs/superpowers/plans/2026-09-30-phase-2-3-autoscalers-nodes.md
git commit -m "docs(plan): add the plan of phase 2.3"
```

**Done when:** the branch is `feat/autoscalers-nodes`, `ROADMAP.md` names
every task of this plan, and the owner has approved the plan and its rulings.

---

### Task 1: The compensated sum (F-76)

**Executor:** Subagent.

**Files:**
- Create: `internal/arith/sum.go`, `internal/arith/sum_test.go`
- Modify: `internal/guard/imports.go` (the `Layers` table: `internal/arith` at rank 0), `internal/guard/imports_test.go` if it counts the packages
- Modify: `internal/collect/statistics.go:134-149` (`statisticsOf`: the average through `arith.Sum`)
- Modify: `AGENTS.md`, the table of layers: `internal/arith` in the row of rank 0

**Interfaces:**
- Produces:

```go
package arith

// Sum adds values as CPython's builtin sum adds floats: Neumaier's
// compensated summation, the compensation added at the end unless it is
// not finite, so that an overflowed sum stays infinite.
func Sum(values []float64) float64

// Mean returns Sum(values) divided by the count. It panics on an empty list;
// a caller checks for one first.
func Mean(values []float64) float64
```

**Requirements:**

- `Sum` is written as the float path of `builtin_sum` in CPython's
  `Python/bltinmodule.c`: running sum `f` and compensation `c` start at 0;
  for each `x`, `t := f + x`; if `|f| >= |x|` then `c += (f - t) + x` else
  `c += (x - t) + f`; `f = t`; at the end, if `c != 0` and `c` is finite,
  `f += c`. Nothing else: no sorting, no pairwise summation.
- `Sum` of an empty list is `0`. `Mean` of an empty list panics with a
  message that names the function.
- `statisticsOf` computes the average as `arith.Sum(values) / float64(len)`
  over the values in timestamp order, replacing the loop.
- Every other number of `statisticsOf` is unchanged. The golden snapshots do
  not change: `go test -count=1 ./internal/goldentest/` passes without
  `make update-testdata`, because every recorded series has two samples.

**Tests:**

| Test | Input | Expected |
|---|---|---|
| `TestSumMatchesPython` | `[3.3, 1.1, 2.2]` | `6.6` (a plain loop gives `6.6000000000000005`) |
| | `[1e16, 1, -1e16]` | `1` |
| | `[0.1 ×10]` | `1` |
| | `[1, 1e-16, 1e-16, 1e-16]` | `1.0000000000000002` |
| | `[1e308, 1e308, -1e308]` | `+Inf` |
| | `[]` | `0` |
| `TestMeanMatchesPython` | the first four lists | `2.1999999999999997`, `0.3333333333333333`, `0.1`, `0.25000000000000006` |
| `TestMeanOfNothingPanics` | `[]` | a panic whose message holds `Mean` |
| `TestSumDiffersFromAPlainLoop` | `[3.3, 1.1, 2.2]` | a plain loop written in the test gives another `float64` than `Sum`; the test fails if they agree, so that the proof of F-76 cannot rot |
| `TestAverageIsCompensated` (in `internal/collect`) | one series of three points `3.3`, `1.1`, `2.2` at rising timestamps | `Average` is exactly `2.2` |
| `TestLayersHoldArith` (in `internal/guard`) | the table | `internal/arith` at rank 0 |

The expected numbers are literals, written as above. Python 3.14.7 gave
each of them on 2026-09-30.

- [ ] **Step 1:** Write the tests. Run them. Expected: the arith tests fail
  to build; `TestAverageIsCompensated` fails with `2.1999999999999997`.
- [ ] **Step 2:** Implement `Sum`, `Mean`, the layer, the average.
- [ ] **Step 3:** Run `go test -count=1 ./internal/arith/ ./internal/collect/ ./internal/guard/ ./internal/goldentest/` and `make lint`.
- [ ] **Step 4:** Commit.

```bash
git add internal/arith internal/guard/imports.go internal/guard/imports_test.go internal/collect/statistics.go internal/collect/statistics_test.go AGENTS.md
git commit -m "fix(collect): sum a series as the original does"
```

**Done when:** the tests pass, the golden snapshots are unchanged, and F-76
names the commit.

---

### Task 2: Evidence of three kinds (R20, R37)

**Executor:** Subagent.

**Files:**
- Modify: `internal/analysis/analysis.go` (`Evidence`, new `Value`), `internal/analysis/registry.go` (`Definition`), `internal/analysis/resources.go` (the constructors), their tests
- Modify: `internal/snapshot/snapshot.go` (`Evidence`, new `Value`), `internal/snapshot/build.go` (`buildEvidence`), `internal/snapshot/codec.go` (`supportedKey`, `checkKeys`, `checkFindingRegistry`), `internal/snapshot/codec_test.go`
- Modify: `internal/goldentest/original_test.go` (`pythonEvidencePair` reads a number, a boolean, or a text)

**Interfaces:**
- Produces, in `internal/analysis`:

```go
type ValueKind string

const (
    KindNumber  ValueKind = "number"
    KindBoolean ValueKind = "boolean"
    KindText    ValueKind = "text"
)

// Value is the value of an evidence: a number, a boolean, or a text.
type Value struct {
    Kind    ValueKind
    Number  float64
    Boolean bool
    Text    string
}

func Number(v float64) Value
func Boolean(b bool) Value
func Text(s string) Value

type Evidence struct {
    Name  string
    Value Value
}

// EvidenceSpec names one evidence of a definition and the kind it carries.
type EvidenceSpec struct {
    Name string
    Kind ValueKind
}

type Definition struct {
    Code       string
    Family     string
    Severities []string // the severities the code may carry, in Severities order
    ScopeType  string
    Params     map[string]string
    Evidence   []EvidenceSpec
}
```

- Produces, in `internal/snapshot`:

```go
// Value is the value of an evidence as the snapshot writes it: a JSON
// number, boolean, or string, one of the three.
type Value struct {
    Kind    string // "number", "boolean", "text"
    Number  float64
    Boolean bool
    Text    string
}

func (v Value) MarshalJSON() ([]byte, error)   // 4, true, "cpu,memory"
func (v *Value) UnmarshalJSON(b []byte) error  // refuses null, an object, a list

type Evidence struct {
    Name  string `json:"name"`
    Value Value  `json:"value"`
}
```

**Requirements:**

- The 40 definitions of part 2.2 keep their evidence names, each with the
  kind `number`, and their one severity each becomes a list of one.
- `checkKeys` accepts, for a field of type `snapshot.Value`, a JSON number,
  a JSON boolean, or a JSON string, and refuses null, an object, and a list
  with the message `snapshot holds a value of the wrong kind at <path>`.
  `supportedKey` accepts the type `Value`.
- `checkFindingRegistry` checks the severity of a finding against the
  `Severities` of its definition and the kind of each evidence value against
  its `EvidenceSpec`, in order, and names the finding, the evidence, the
  kind found, and the kind wanted.
- A number that is `NaN` or infinite is still refused; a text is refused
  when empty.
- The golden snapshots do not change a byte: a number is written as before.
- `pythonEvidencePair` keeps `Name` and gains `Value analysis.Value`,
  reading a JSON number, boolean, or string; the boolean `approximate` pair
  is still left out of the comparison (R20); the comparison of the evidence
  of a finding compares kind and value, a number as the same `float64`.

**Tests:**

| Test | Input | Expected |
|---|---|---|
| `TestValueMarshalsAsItsKind` | `Number(4)`, `Boolean(true)`, `Text("cpu,memory")` | `4`, `true`, `"cpu,memory"` |
| `TestValueUnmarshalsEachKind` | `4`, `4.5`, `true`, `"cpu"` | the values; `null`, `{}`, `[]` refused |
| `TestDecodeRefusesAnEvidenceOfTheWrongKind` | a golden snapshot with one evidence value changed to `true` | `Decode` fails naming the finding and the evidence |
| `TestDecodeRefusesAnEvidenceObject` | one evidence value changed to `{}` | `snapshot holds a value of the wrong kind at findings[0].evidence[0].value` |
| `TestRegistryHoldsTheKindOfEveryEvidence` | the 40 definitions | every evidence `number`; every `Severities` of length one |
| `TestGoldenSnapshotsAreUnchanged` | the seven golden files | `Encode(Decode(file))` equals the file |
| `TestPythonEvidenceReadsThreeKinds` | `["node_pressure", true]`, `["resources", "cpu,memory"]`, `["affected_nodes", 1]` | the three values |
| `TestKeyPathsAreTheContract` | the walk | unchanged count; `findings[].evidence[].value` still one path |

- [ ] **Step 1:** Write the tests. Run them. Expected: fail.
- [ ] **Step 2:** Implement.
- [ ] **Step 3:** Run `go test -count=1 ./internal/analysis/ ./internal/snapshot/ ./internal/goldentest/ ./internal/document/` and `make lint`. Expected: pass; no file under `testdata/` changed.
- [ ] **Step 4:** Commit.

```bash
git add internal/analysis internal/snapshot internal/goldentest/original_test.go
git commit -m "feat(snapshot): let an evidence carry three kinds"
```

**Done when:** the tests pass and `git status` shows no change under
`testdata/` or `template/`.

---

### Task 3: Node identity and node pool identity

**Executor:** Subagent.

**Files:**
- Create: `internal/resolve/node.go`, `node_test.go`, `internal/resolve/nodepool.go`, `nodepool_test.go`

**Interfaces:**
- Produces:

```go
package resolve

// InstanceHost returns the host of an instance label: the port, IPv6
// brackets, and any user information removed, the host in lower case; the
// text itself when it has no host part; the empty text for an empty
// instance or a malformed one.
func InstanceHost(instance string) string

// NodeRecord is what the index knows of a node.
type NodeRecord struct {
    Name       string
    InternalIP string
    Hostname   string
}

// NodeIndex resolves a series to a node by its node label, then by the host
// of its instance label against the internal IP, the hostname, and the
// name, in that order. A value that two nodes share resolves to no node.
type NodeIndex struct{ /* three maps */ }

func NewNodeIndex(nodes []NodeRecord) NodeIndex
func (x NodeIndex) Resolve(nodeLabel, instance string) (name string, ok bool)

const PoolUnknown = "unknown"

// KnownPoolLabels are the labels tried after the configured one, in order.
var KnownPoolLabels = []string{
    "label_kubernetes_azure_com_agentpool",
    "label_agentpool",
    "label_cloud_google_com_gke_nodepool",
    "label_eks_amazonaws_com_nodegroup",
}

// SharedNameLabels returns the values of label_name that more than one node
// carries.
func SharedNameLabels(labelsByNode map[string]map[string]string) map[string]bool

// NodePool returns the pool of a node, where it came from, and the
// confidence, by the six steps of the original in order.
func NodePool(nodeName string, labels map[string]string, configuredLabel string, sharedNames map[string]bool) (pool, source, confidence string)
```

  Confidences are the strings of `discovery`: `exact`, `inferred`,
  `unknown`; `resolve` is below `discovery` in the layers, so it declares
  its own three constants with the same values and a test in
  `internal/discovery` or `internal/collect` pins the equality.

**Requirements:**

- `InstanceHost` follows `urlsplit("//" + instance).hostname or instance`:
  `10.0.0.1:9100` gives `10.0.0.1`; `[fd00::1]:9100` gives `fd00::1`;
  `Node-A:9100` gives `node-a`; `user@host:1` gives `host`; `host` gives
  `host`; `` gives ``; `[abc` gives `` (malformed). Go's `net/url` does not
  lower-case a host and refuses other inputs; write the parse by hand and
  pin these seven cases.
- `NewNodeIndex` builds three indexes, by name, by internal IP, by
  hostname, skipping empty values; a value seen on a second node is removed
  and marked, so that a third node with the same value is skipped too.
- `Resolve`: the node label as given against the name index; else the host
  of the instance against IP, hostname, name, in that order; an empty host
  matches nothing. The node label is not lower-cased.
- `NodePool` steps: 1 the configured label, when set and the node has it
  with a value, exact; 2 the four known labels in order, exact; 3 the AKS
  pattern `^aks-([a-z0-9]+?)-[0-9]+-vmss`, anchored at the start only,
  inferred, source `aks-node-name`; 4 an OKE name, split at `-`, at least
  four parts and the first `oke`, pool `pool-<third part>`, inferred,
  source `oke-node-name`; 5 `label_name` when its value is in
  `sharedNames`, inferred, source `label_name`; 6 `unknown`, source `none`,
  confidence unknown.

**Tests:**

| Test | Input | Expected |
|---|---|---|
| `TestInstanceHost` | the seven cases above | as listed |
| `TestIndexDropsASharedValue` | nodes `a` and `b` with IP `10.0.0.1`, `c` with `10.0.0.3` | `Resolve("", "10.0.0.1:9100")` not ok; `Resolve("", "10.0.0.3:9100")` gives `c` |
| `TestIndexPrefersTheNodeLabel` | node `a` | `Resolve("a", "10.9.9.9:1")` gives `a` |
| `TestIndexOrderIPHostnameName` | node `a` with hostname `h`, node `h` with IP `` | `Resolve("", "h:1")` gives `a` (hostname before name) |
| `TestIndexDoesNotLowerTheNodeLabel` | node `Node-A` | `Resolve("node-a", "")` not ok; `Resolve("", "Node-A:1")` gives `Node-A` |
| `TestNodePoolConfiguredLabelWins` | configured `label_pool`, labels `{label_pool: x, label_agentpool: y}` | `x`, `label_pool`, `exact` |
| `TestNodePoolKnownLabelsInOrder` | labels `{label_agentpool: b, label_kubernetes_azure_com_agentpool: a}` | `a`, exact |
| `TestNodePoolAKSName` | `aks-nodepool1-12345678-vmss000000` | `nodepool1`, `aks-node-name`, `inferred` |
| `TestNodePoolAKSNameNeedsThePrefix` | `xaks-nodepool1-1-vmss` | not step 3 |
| `TestNodePoolOKEName` | `oke-cabc-nxyz-swxyz-0` | `pool-nxyz`, `oke-node-name`, `inferred` |
| `TestNodePoolOKENameNeedsFourParts` | `oke-a-b` | not step 4 |
| `TestNodePoolSharedName` | `label_name: workers`, shared `{workers}` | `workers`, `label_name`, `inferred`; with shared empty, `unknown` |
| `TestNodePoolUnknown` | no labels, plain name | `unknown`, `none`, `unknown` |
| `TestSharedNameLabels` | three nodes, two with `label_name: w`, one with `v` | `{w}` |

- [ ] **Step 1:** Write the tests. Run them. Expected: fail to build.
- [ ] **Step 2:** Implement.
- [ ] **Step 3:** Run `go test -count=1 ./internal/resolve/` and `make lint`.
- [ ] **Step 4:** Commit.

```bash
git add internal/resolve
git commit -m "feat(resolve): identify nodes and node pools"
```

**Done when:** the tests pass and `internal/resolve` still imports nothing
of the module.

---

### Task 4: Gauges, instant values, and the numbers with a flag

**Executor:** Subagent.

**Files:**
- Create: `internal/collect/gauges.go`, `gauges_test.go`

**Interfaces:**
- Produces:

```go
// Number is a value that may be unknown.
type Number struct {
    Value float64
    Known bool
}

// Flag is a boolean that may be unknown.
type Flag struct {
    Value bool
    Known bool
}

// ReduceMax is Reduce with the maximum in place of the sum where two series
// share a key and a timestamp. It is the gauge aggregation of the original.
func ReduceMax(series []prom.Series, labels []string) map[Key]Statistics

// InstantValues reads the value of each key at its latest timestamp from
// instant samples. A key with more than one distinct value at that
// timestamp is ambiguous: it is absent from values and present in
// ambiguous. Invalid samples are skipped.
func InstantValues(samples []prom.Sample, at float64, labels []string) (values map[Key]float64, ambiguous map[Key]bool)

// Percent returns numerator over denominator times 100, evaluated in that
// order, and false when either is unknown or not finite or the denominator
// is not above zero.
func Percent(numerator, denominator Number) Number
```

**Requirements:**

- `ReduceMax` shares the grouping of `Reduce` (`samplesByTimestamp`) with
  the reducer as a parameter; the average uses `arith.Sum`. A key whose
  samples are all invalid is absent.
- `InstantValues` takes the timestamp of every sample as `at` (an instant
  answer of the client keeps no timestamp; the original compares
  timestamps within one answer, which are all equal), so ambiguity is
  "more than one distinct value for one key". `1` and `1.0` are one value.
- The key of `InstantValues` follows `keyOf`: an absent label reads
  `unknown`.
- `Percent(n, d)` is `n.Value / d.Value * 100`, left to right.

**Tests:**

| Test | Input | Expected |
|---|---|---|
| `TestReduceMaxKeepsTheLargerSample` | two series, one key, `t1` `2` and `t1` `5` | `Peak` `5`, `Average` `5` |
| `TestReduceMaxAverageIsCompensated` | `3.3`, `1.1`, `2.2` | `2.2` |
| `TestInstantValuesReadsOnePerKey` | keys `a` `1`, `b` `4` | the two |
| `TestInstantValuesMarksAmbiguity` | key `a` `4` and `6` | `a` absent from values, present in ambiguous |
| `TestInstantValuesIgnoresEqualDuplicates` | key `a` `4` and `4` | `a` `4`, not ambiguous |
| `TestInstantValuesSkipsInvalid` | key `a` `NaN` only | absent, not ambiguous |
| `TestPercent` | `(3.6, 4)` | `90`; `(1, 0)` unknown; `(1, unknown)` unknown; `(+Inf, 1)` unknown |

- [ ] **Step 1:** Write the tests. Run them. Expected: fail to build.
- [ ] **Step 2:** Implement.
- [ ] **Step 3:** Run `go test -count=1 ./internal/collect/` and `make lint`.
- [ ] **Step 4:** Commit.

```bash
git add internal/collect/gauges.go internal/collect/gauges_test.go internal/collect/statistics.go
git commit -m "feat(collect): reduce gauges and instant values"
```

**Done when:** the tests pass.

---

### Task 5: The HPA collector and the coverage inventory

**Executor:** Subagent.

**Files:**
- Create: `internal/collect/hpas.go`, `hpas_test.go`
- Modify: `internal/collect/collect.go` (the codes of this task)

**Interfaces:**
- Consumes: `Input`, `Querier`, `Result[T]`, `Message`, `findCapability`, `prom.Selector`, `ReduceMax`, `InstantValues`, `instantSeries`.
- Produces:

```go
type HPA struct {
    Namespace, Name       string
    TargetKind, TargetName string // empty when unknown or ambiguous
    SpecMin, SpecMax       Number // unknown when absent or ambiguous
    Current, Desired       Statistics
}

// WorkloadRef names a workload of the coverage inventory.
type WorkloadRef struct{ Namespace, Kind, Name string }

const (
    WarningHPAAmbiguousSpec   = "HPA_AMBIGUOUS_SPEC"
    WarningHPAAmbiguousTarget = "HPA_AMBIGUOUS_TARGET"
    NoticeWorkloadsOmittedScaleZero = "WORKLOADS_OMITTED_SCALE_ZERO"
)

// HPAs collects the HPAs of the scope and then the coverage inventory, in
// the order of the original, sharing the one answer of the info query.
func HPAs(ctx context.Context, q Querier, in Input) (history Result[HPA], coverage Result[WorkloadRef], err error)
```

  `WarningCodes` gains the two warnings after the seven of part 2.2,
  `NoticeCodes` gains the notice after `JOBS_OMITTED`, `NoticeParams` maps
  it to `{"count"}`.

**Requirements, the history:**

- Gate: capability `hpa_history` unavailable: no query; `Result{State:
  unavailable, Strategy: the capability's, Rows: [], Warnings: [], Notices: []}`.
  A partial capability runs every query.
- Queries, in order, with the selector `{<scope matchers>,namespace=~"(?:<namespaces in configured order>)"}`:
  1. instant `kube_horizontalpodautoscaler_info`
  2. instant `kube_horizontalpodautoscaler_spec_min_replicas`
  3. instant `kube_horizontalpodautoscaler_spec_max_replicas`
  4. range `kube_horizontalpodautoscaler_status_current_replicas`
  5. range `kube_horizontalpodautoscaler_status_desired_replicas`
  Instant queries at `in.Window.End`; range queries with the window.
- Identity of an HPA: labels `namespace`, `horizontalpodautoscaler`, an
  absent one `unknown`. Specs through `InstantValues`. Current and desired
  through `ReduceMax` by the same labels; a key with none is a statistic of
  state unavailable.
- Targets: from the info samples, per key, the set of `(scaletargetref_kind,
  scaletargetref_name)` pairs, absent labels empty; one distinct pair is the
  target; more than one is an ambiguous target, and the HPA's target stays
  empty.
- Rows: one for every key of targets, min, max, current, desired, and the
  ambiguous keys; sorted by namespace then name.
- State: `partial` when any spec or target is ambiguous, else the
  capability's state; strategy `raw_metrics`. Warnings: the reason codes of
  the capability `hpa_history`, in their order, as the original adds the
  reason sentence (R60); then `HPA_AMBIGUOUS_SPEC` when any spec or target
  is ambiguous. No notice.

**Requirements, the coverage:**

- Gate: capability `workloads_without_hpa` unavailable: no query; the
  unavailable result.
- Queries, in order: the info query only when the history did not send it;
  then the deployment inventory, `kube_deployment_labels` if it is in the
  capability's evidence, else `kube_deployment_spec_replicas` if it is, else
  none; then the StatefulSet inventory likewise with `kube_statefulset_labels`
  and `kube_statefulset_replicas`. Same selector shape.
- Keys: labels `namespace` and `deployment` or `statefulset`, through
  `InstantValues`; ambiguous keys are neither kept nor counted. With the
  `labels` family every key is kept; with the `replicas` family a key whose
  value is `0` is left out and counted as omitted.
- Reliable targets: `(namespace, kind, name)` of every HPA whose target is
  known, the kind as the label wrote it, compared as it is. Ambiguous
  namespaces: those of an HPA with an ambiguous target.
- Rows: the identities of kind `Deployment` and `StatefulSet` that are not
  reliable targets and whose namespace is not ambiguous, sorted by
  namespace, name, kind.
- State: `partial` when a target is ambiguous, else the capability's;
  strategy `raw_metrics`. Warnings: the reason codes of the capability
  `workloads_without_hpa` (R60); then `HPA_AMBIGUOUS_TARGET` when a target
  is ambiguous. Notice: `WORKLOADS_OMITTED_SCALE_ZERO` with `count` when
  the omitted count is above zero.

**Tests:** each builds the answers by hand and a `Querier` that records the
queries it sees.

| Test | Input | Expected |
|---|---|---|
| `TestHPAQueriesInOrder` | every metric answered | the seven query texts of lines 44 to 50 of `direct-complete.queries.txt`, in order; the info query once |
| `TestHPAUnavailableSendsNothing` | `hpa_history` unavailable, coverage available | no history query; the coverage sends the info query first |
| `TestHPAWarningsHoldTheReasons` | `hpa_history` partial with reason `HPA_HISTORY_INCOMPLETE` and an ambiguous spec | warnings `HPA_HISTORY_INCOMPLETE`, then `HPA_AMBIGUOUS_SPEC`, each with empty params |
| `TestHPAPartialRunsEveryQuery` | `hpa_history` partial | five queries; state `partial` |
| `TestHPARowFromTheCompleteScenario` | the answers of `direct-complete` | one row `payments/payments-web`, target `Deployment/payments-web`, min `1`, max `4`, current average `2.5` peak `3` current `3`, desired average `3.5` |
| `TestHPAAmbiguousSpec` | two `spec_max` samples `4` and `6` for one key | `SpecMax` unknown; state `partial`; warning `HPA_AMBIGUOUS_SPEC` |
| `TestHPAAmbiguousTarget` | two info samples with two targets for one key | target empty; the namespace's workloads absent from the coverage rows; coverage `partial` with `HPA_AMBIGUOUS_TARGET` |
| `TestHPAWithoutSpecsStillARow` | current only | a row with unknown specs and unavailable desired |
| `TestCoverageLabelsFamily` | deployments `web` (covered), `api`; statefulset `queue` | rows `api/Deployment`, `queue/StatefulSet` sorted by name; no notice |
| `TestCoverageReplicasFamily` | evidence without `kube_deployment_labels`; `kube_deployment_spec_replicas` `web` 3, `batch` 0 | `batch` omitted; notice count `1`; query text `kube_deployment_spec_replicas{...}` |
| `TestCoverageKindIsCaseSensitive` | HPA target kind `deployment` (lower) for `web` | `web` is uncovered |
| `TestCoverageSortsByNamespaceNameKind` | `b/Deployment`, `a/StatefulSet`, `a/Deployment` | `a/Deployment`, `a/StatefulSet`, `b/Deployment` |
| `TestCoverageUnavailableSendsNothing` | capability unavailable | no query, unavailable result |
| `TestNoticeAndWarningCodesGrew` | the lists | nine warnings, three notices, `NoticeParams` of the new notice `{"count"}` |

- [ ] **Step 1:** Write the tests. Run them. Expected: fail.
- [ ] **Step 2:** Implement.
- [ ] **Step 3:** Run `go test -count=1 ./internal/collect/ ./internal/catalog/` and `make lint`. The catalog test `TestEveryWarningHasItsEntry` fails for the new codes until Task 12; state it in the report and stop there: the codes are the brief's, the entries are the main thread's.
- [ ] **Step 4:** Commit.

```bash
git add internal/collect/hpas.go internal/collect/hpas_test.go internal/collect/collect.go internal/collect/collect_test.go
git commit -m "feat(collect): collect HPAs and their coverage"
```

**Done when:** the collect tests pass and the only failing test of the
suite is the catalog's check of the new codes.

---

### Task 6: The node collector

**Executor:** Subagent.

**Files:**
- Create: `internal/collect/nodes.go`, `nodes_test.go`
- Modify: `internal/collect/collect.go` (the two warning codes)

**Interfaces:**
- Consumes: `Input`, `Querier`, `Result[T]`, `ReduceMax`, `Percent`, `Number`, `Flag`, `resolve.NewNodeIndex`, `resolve.NodePool`, `resolve.SharedNameLabels`, `resolve.InstanceHost`, `usesLegacyResourceNames`.
- Produces:

```go
type Node struct {
    Name           string
    Pool           string // "unknown" when undetermined
    PoolConfidence string // exact, inferred, unknown
    CPUCapacity, MemoryCapacity, CPUAllocatable, MemoryAllocatable Number
    CPUUsagePercent, MemoryUsagePercent Statistics
    CPURequestPercent, MemoryRequestPercent Number // one ratio, requested over allocatable
    Ready, Schedulable Flag
}

type Pool struct {
    Name       string
    Confidence string   // the lowest of its nodes: unknown < inferred < exact
    Nodes      []string // names, sorted
}

const (
    WarningNodeIdentityUnavailable         = "NODE_IDENTITY_UNAVAILABLE"
    WarningNodeCapacityEvidenceIncomplete  = "NODE_CAPACITY_EVIDENCE_INCOMPLETE"
)

// Nodes collects the nodes of the scope in the order of the original.
func Nodes(ctx context.Context, q Querier, in Input) (Result[Node], error)

// Pools groups nodes by pool name, sorted by name, "unknown" among them.
func Pools(nodes []Node) []Pool
```

  `WarningCodes` gains the two codes after those of Task 5.

**Requirements:**

- Queries, in order, every selector `{<scope matchers>[, extra]}` with the
  matchers sorted by label name, so that `cluster` comes before `condition`
  and `resource`:
  1. instant `kube_node_info`
  2. instant `node_uname_info`
  3. instant `kube_node_labels`, unless `nodepool_discovery` is unavailable
  4. instant `kube_node_status_capacity{...,resource="cpu"}`
  5. instant `kube_node_status_capacity{...,resource="memory"}`
  6. instant `kube_node_status_allocatable{...,resource="cpu"}`
  7. instant `kube_node_status_allocatable{...,resource="memory"}`
  8. instant `kube_node_status_condition{...,condition="Ready"}`
  9. instant `kube_node_spec_unschedulable`
  10. and 11. unless `requests_limits` is unavailable: instant
      `sum by (node, instance) (kube_pod_container_resource_requests{...,resource="cpu"})`
      then `memory`; with legacy names,
      `sum by (node, instance) (kube_pod_container_resource_requests_cpu_cores{...})`
      only when that name is in the evidence, likewise memory; a legacy name
      not in the evidence sends nothing for that resource
  12. and 13. unless `node_utilization` is unavailable, range queries, CPU
      first: with the strategy `recording_rule`,
      `instance:node_cpu_utilisation:rate5m{...} * 100` and
      `instance:node_memory_utilisation:ratio{...} * 100`; with `raw_metrics`,
      `100 - avg by(instance) (rate(node_cpu_seconds_total{...,mode="idle"}[<step>])) * 100`
      and `(1 - node_memory_MemAvailable_bytes{...} / node_memory_MemTotal_bytes{...}) * 100`,
      the step the window's step text such as `5m`
- Identity: from `kube_node_info`, per valid sample with a non-empty `node`
  label, the sets of `internal_ip`, `hostname`, `provider_id`; an attribute
  with exactly one value is kept, else empty. The names of every other
  answer (labels, the four capacities, ready, unschedulable, the two usage
  series, the two request answers) join the set of nodes; a name found only
  there has an empty identity. Hostnames: from `node_uname_info`, each
  valid sample resolved through the index by `node` and `instance`; the
  `nodename` through `InstanceHost` is a candidate; a node with no hostname
  takes its one candidate. The provider id is read and never used.
- Labels: from `kube_node_labels`, per node, every label but `node` whose
  value is one across the node's series; `SharedNameLabels` over them;
  `NodePool(name, labels, in.Cluster.NodepoolLabel, shared)`.
- Capacity and allocatable: `ReduceMax` by `node`, the `Current` value.
  Requests: `ReduceMax` by `node` (the `instance` of the `by` clause is
  ignored), `Current`; request percent = `Percent(requested, allocatable)`.
- Utilisation: each series resolved through the index by `node` and
  `instance`; unresolved series dropped; the resolved ones re-keyed to the
  node's name and reduced with `ReduceMax`.
- Ready: samples with non-empty `node` and `status`; per node the statuses
  whose current value is above zero; `{true}` is ready, `{false}` not
  ready, anything else unknown. Schedulable: the unschedulable value `0` is
  schedulable, another value not, absent unknown.
- Rows sorted by name. `Pools` groups by `Pool`, confidence the lowest.
- State and warnings: no identity at all gives `unavailable` with the one
  warning `NODE_IDENTITY_UNAVAILABLE` and no row. Otherwise `incomplete`
  when a name was found outside `kube_node_info`, or any of the three
  capabilities is not available, or a node has an unknown pool
  confidence, or lacks a capacity, an allocatable, or a usage current, or
  has an allocatable above zero and no request percent. Warnings: the
  reason codes of `node_utilization`, `nodepool_discovery`, and
  `requests_limits`, in that order, each once (R60); when incomplete and
  no reason exists, `NODE_CAPACITY_EVIDENCE_INCOMPLETE`. State `partial`
  when incomplete, else `available`; strategy the strategy of
  `node_utilization`.

**Tests:**

| Test | Input | Expected |
|---|---|---|
| `TestNodeQueriesInOrder` | every capability available, recording rule | the thirteen texts of lines 51 to 63 of `direct-complete.queries.txt`, in order |
| `TestNodeQueriesRaw` | `node_utilization` raw, step `5m` | the two raw expressions of `raw-fallback.queries.txt` |
| `TestNodeQueriesSkipWhatIsUnavailable` | `nodepool_discovery`, `requests_limits`, `node_utilization` unavailable | eight queries |
| `TestNodeRowFromTheCompleteScenario` | the answers of `direct-complete` | `node-1`, pool `pool-a` exact, capacity `4`/`8`, allocatable `4`/`8`, CPU usage average `57.5` peak `60`, memory `62.5`/`65`, ready true, schedulable true |
| `TestNodeResolvesByInstance` | a usage series with `instance` `10.0.0.1:9100` and no `node`; info gives `node-1` at `10.0.0.1` | the series counts for `node-1` |
| `TestNodeDropsAnUnresolvedSeries` | two nodes with IP `10.0.0.1`; a series by that instance | neither node has usage; both unavailable |
| `TestNodeKnownByRequestsOnly` | `orphan` in the requests answer only | a row `orphan` with empty identity; state `partial`; warning `NODE_CAPACITY_EVIDENCE_INCOMPLETE` |
| `TestNodeWarningsHoldTheReasons` | `nodepool_discovery` partial with `NODEPOOL_LABELS_NOT_FOUND` | that code once; no `NODE_CAPACITY_EVIDENCE_INCOMPLETE` |
| `TestNodeNoIdentity` | every answer empty | unavailable; `NODE_IDENTITY_UNAVAILABLE`; no row |
| `TestNodeReadyStates` | statuses `true`, `false`, `unknown`, none | ready, not ready, unknown, unknown |
| `TestNodeRequestPercent` | requests `3.6`, allocatable `4` | `90`; allocatable `0` unknown |
| `TestNodePoolFromTheName` | `aks-agentpool-12345678-vmss000000` with no labels | pool `agentpool` inferred |
| `TestPools` | nodes in `pool-b`, `pool-a`, `unknown` with mixed confidences | sorted `pool-a`, `pool-b`, `unknown`; confidence the lowest |
| `TestNodeHostnameLowered` | `nodename` `Node-1` | hostname `node-1` |

- [ ] **Step 1:** Write the tests. Run them. Expected: fail.
- [ ] **Step 2:** Implement.
- [ ] **Step 3:** Run `go test -count=1 ./internal/collect/` and `make lint`.
- [ ] **Step 4:** Commit.

```bash
git add internal/collect/nodes.go internal/collect/nodes_test.go internal/collect/collect.go internal/collect/collect_test.go
git commit -m "feat(collect): collect nodes and node pools"
```

**Done when:** the collect tests pass; the catalog check of the new codes is
the only failing test of the suite.

---

### Task 7: The cluster autoscaling collector

**Executor:** Subagent.

**Files:**
- Create: `internal/collect/autoscaling.go`, `autoscaling_test.go`

**Interfaces:**
- Produces:

```go
const (
    AutoscalingEnabled  = "enabled"
    AutoscalingDisabled = "disabled"
    AutoscalingUnknown  = "unknown"
)

type AutoscalingEvidence struct {
    Status     string // enabled, disabled, unknown
    Source     string // a discovery strategy: configured, raw_metrics, none
    Confidence string // exact, inferred, unknown
    ManagedNodeGroups, CurrentNodes, MinNodes, MaxNodes Number // whole numbers, truncated
    ScaleEvents, UnschedulablePods                       Number
}

// Autoscaling collects the cluster autoscaling evidence. With the mode
// configured as enabled or disabled it sends nothing.
func Autoscaling(ctx context.Context, q Querier, in Input) (AutoscalingEvidence, error)
```

**Requirements:**

- `in.Analysis.NodeAutoscaling` `enabled` or `disabled`: no query;
  `{Status: that, Source: configured, Confidence: exact}`, numbers unknown.
- Otherwise six range queries in this order, the selector
  `<metric>{<scope matchers>}`, no namespace matcher, whatever the state of
  the capability `node_autoscaling`:
  1. `cluster_autoscaler_node_groups_count`
  2. `cluster_autoscaler_nodes_count`
  3. `cluster_autoscaler_node_group_min_nodes_count`
  4. `cluster_autoscaler_node_group_max_nodes_count`
  5. `increase(cluster_autoscaler_scaled_up_nodes_total{...}[<N>s])`, `N` the
     whole seconds from the window start to its end, at least 1
  6. `cluster_autoscaler_unschedulable_pods_count`
- Value of a metric: the group of a series is the first non-empty label
  among `node_group`, `nodegroup`, `node_group_name`, `node_group_id`. With
  grouped series and a summed field (the first five): per group `ReduceMax`
  and `Current`, then `arith.Sum` of the currents; ungrouped series ignored.
  With grouped series and the sixth field: grouped series only, one key,
  `ReduceMax`, `Current`. With no grouped series: every series, one key,
  `ReduceMax`, `Current`. No series with a valid point: unknown.
- Every value unknown: `{unknown, none, unknown}`. Otherwise `{enabled,
  raw_metrics, inferred}` with the four counts truncated toward zero
  (`math.Trunc`) and the two others as they are.

**Tests:**

| Test | Input | Expected |
|---|---|---|
| `TestAutoscalingConfiguredSendsNothing` | `disabled` | no query; `{disabled, configured, exact}` |
| `TestAutoscalingQueriesInOrder` | `auto`, window of 86400 s | the six texts of lines 64 to 69 of `direct-complete.queries.txt` |
| `TestAutoscalingWindowAtLeastOneSecond` | start equal to end | `[1s]` |
| `TestAutoscalingFromTheCompleteScenario` | the answers of `direct-complete` | groups `2`, nodes `3`, min `1`, max `3`, scale events `1`, unschedulable `1`; `enabled`, `raw_metrics`, `inferred` |
| `TestAutoscalingSumsGroups` | `nodes_count` groups `a` `[2,3]`, `b` `[1,1]` | `4` |
| `TestAutoscalingIgnoresUngroupedBesideGrouped` | one grouped `[2,2]`, one ungrouped `[9,9]` | `2` |
| `TestAutoscalingMergesUngroupedByMax` | two ungrouped series `[2,3]` and `[5,1]` | current `3` (the maximum at the last timestamp) |
| `TestAutoscalingTruncates` | `nodes_count` `2.9` | `2`; `scale_events` `2.9` stays `2.9` |
| `TestAutoscalingUnknown` | every answer empty | `{unknown, none, unknown}` |
| `TestAutoscalingOneMetricIsEnough` | only `unschedulable_pods` answered | `enabled`, the others unknown |

- [ ] **Step 1:** Write the tests. Run them. Expected: fail.
- [ ] **Step 2:** Implement.
- [ ] **Step 3:** Run `go test -count=1 ./internal/collect/` and `make lint`.
- [ ] **Step 4:** Commit.

```bash
git add internal/collect/autoscaling.go internal/collect/autoscaling_test.go
git commit -m "feat(collect): collect the cluster autoscaling"
```

**Done when:** the tests pass.

---

### Task 8: The five collectors in order, the second insufficiency rule, and the original recorded again

**Executor:** Subagent for the Go code; main thread for the recording and
the golden files.

**Files:**
- Modify: `internal/collect/collect.go` (`Collected`, `All`, `Warnings`, `Notices`, `CheckUseful`, `ErrNoUsefulSections`), `collect_test.go`
- Modify: `internal/snapshot/codec.go` (`checkMessageCodes`: a warning code is one of `collect.WarningCodes` or `discovery.ReasonCodes`), `codec_test.go`
- Modify: `internal/document/appendix.go` (`buildNotes`: a warning whose code is a reason code is rendered with `catalog.CapabilityReason`), `appendix_test.go`
- Modify: `internal/goldentest/goldentest.go` (`run` calls `CheckUseful` after `collect.All`), `internal/goldentest/golden_test.go` (the tests that count queries: `TestQueriesTellTheProbesApart`, `TestQueriesTellTheCollectorsApart`, `collectorQueryFloor`), `internal/goldentest/original_test.go` (`TestQueriesBeginTheListOfTheOriginal` becomes `TestQueriesEqualTheOriginal`; `originalWarnings` gains the sentences of the reason codes, the two HPA warnings, and the two node warnings; `originalNotices` gains the scale-zero notice; the comparison reads the five collection states and the autoscaling status)
- Rewritten by the main thread: `testdata/python/<scenario>.json`, seven files, and `testdata/python/README.md`
- Rewritten by `make update-testdata`: `testdata/queries/*.txt`, `testdata/snapshot/*.json` where a warning of the new collectors appears, `template/tests/capacity/fixtures/command/*.json` where the notes change

**Interfaces:**
- Produces:

```go
type Collected struct {
    Namespaces  Result[Namespace]
    Workloads   Result[Workload]
    HPAs        Result[HPA]
    Coverage    Result[WorkloadRef]
    Nodes       Result[Node]
    Autoscaling AutoscalingEvidence
}

// All runs the five collectors in the order of the original and stops at
// the first error.
func All(ctx context.Context, q Querier, in Input) (Collected, error)

// ErrNoUsefulSections is the second rule of insufficient capabilities: no
// collector returned a row and the autoscaling status is unknown.
var ErrNoUsefulSections = errors.New("no useful report sections were collected")

// CheckUseful reports ErrNoUsefulSections when no result of the five holds
// a row with a state other than unavailable and the autoscaling status is
// unknown.
func CheckUseful(c Collected) error
```

**Requirements, the Go code:**

- `All`: namespaces, workloads, `HPAs` (history and coverage), nodes,
  autoscaling. `Warnings()` and `Notices()` join the five results in the
  order namespace, workload, hpa, hpa coverage, node, deduplicated by code
  and params.
- A warning whose code is a reason code (R60) passes `snapshot.Check` and
  is printed among the notes with the sentence of the reason. The notes
  are still deduplicated by their final text.
- After this task the query list of every scenario equals the recorded
  list of the original, line for line: `TestQueriesEqualTheOriginal`
  compares the whole files and fails on any difference, in either length.

**Requirements, the recording** (main thread, R58). The script
`.cache/phase-2/part-2/python/record.py` is copied to
`.cache/phase-2/part-3/python/record.py` beside the same environment and
extended, so that a recorded file holds, beside what it held:

```json
{
  "hpa_collection": "available",
  "hpa_coverage_collection": "available",
  "node_collection": "available",
  "autoscaling_status": "enabled",
  "hpas": [{"namespace": "payments", "name": "payments-web", "target_kind": "Deployment", "target_name": "payments-web", "spec_min": 1.0, "spec_max": 4.0, "current": {"average": 2.5, "minimum": 2.0, "peak": 3.0, "current": 3.0, "no_base": false}, "desired": {"average": 3.5, "minimum": 3.0, "peak": 4.0, "current": 4.0, "no_base": false}}],
  "workloads_without_hpa": [{"namespace": "payments", "kind": "StatefulSet", "name": "payments-worker"}],
  "nodes": [{"name": "node-1", "pool": "pool-a", "pool_confidence": "exact", "cpu_capacity": 4.0, "memory_capacity": 8.0, "cpu_allocatable": 4.0, "memory_allocatable": 8.0, "cpu_usage_percent": {"average": 57.5, "minimum": 55.0, "peak": 60.0, "current": 60.0, "no_base": false}, "memory_usage_percent": {"average": 62.5, "minimum": 60.0, "peak": 65.0, "current": 65.0, "no_base": false}, "cpu_request_percent": null, "memory_request_percent": null, "ready": true, "schedulable": true}],
  "nodepools": [{"name": "pool-a", "confidence": "exact", "nodes": ["node-1"]}],
  "autoscaling": {"status": "enabled", "source": "raw_metrics", "confidence": "inferred", "managed_node_groups": 2, "current_nodes": 3, "min_nodes": 1, "max_nodes": 3, "scale_events": 1.0, "unschedulable_pods": 1.0},
  "node_pressure": false,
  "findings": ["every finding of the original, no filter, evidence values as Python writes them: a number, true or false, or a text"],
  "warnings": ["the sentences of the five results in the order namespace, workload, hpa, hpa coverage, node, each once"],
  "notices": ["likewise"]
}
```

  The example numbers are what the answers of `direct-complete` give; the
  script writes what the original computes, and the numbers above are
  checked against the file after the run. `cpu_request_percent` is the
  `average` of the original's statistic, or null. The seven scenarios are
  recorded again, whole; the query lists are recorded again with
  `queries.py` and must not change. The README is rewritten: the recorded
  fields, the date, the commit of `cmd/fake-prometheus` used, the sentence
  about the original sending more requests removed, the SHA-256 table.
  Every changed number of the seven files is read before the commit.

**Tests:**

| Test | Input | Expected |
|---|---|---|
| `TestAllRunsTheFiveInOrder` | a `Querier` that records | the queries of namespaces, then workloads, then the seven of the HPAs, then the nodes, then the six of autoscaling |
| `TestAllStopsAtTheFirstError` | the node collector fails | no autoscaling query |
| `TestWarningsKeepTheResultOrder` | a warning in each result | namespace, workload, hpa, coverage, node |
| `TestCheckUsefulPasses` | one namespace row | nil |
| `TestCheckUsefulFails` | every result empty or unavailable; autoscaling unknown | `ErrNoUsefulSections` |
| `TestCheckUsefulSavedByAutoscaling` | every result empty; autoscaling `disabled` | nil |
| `TestCheckUsefulIgnoresUnavailableRows` | rows under a state `unavailable` only | `ErrNoUsefulSections` |
| `TestCheckAcceptsAReasonCodeAsWarning` | a snapshot with the warning `METRICS_NOT_FOUND` | `Check` passes; an unknown code still refused |
| `TestNotesRenderAReasonCode` | that snapshot, pt-BR | the sentence of `capability.reason.METRICS_NOT_FOUND` once |
| `TestQueriesEqualTheOriginal` | the seven scenarios | equal, whole |
| `TestQueriesTellTheCollectorsApart` | the lists | counts rewritten for the queries the program now asks: read what the test counts before changing the number |
| `TestNumbersEqualTheOriginal` | the seven scenarios | the warnings and notices of the five results equal the recorded sentences, the states equal; the rest as before |
| `TestOriginalSentencesCoverEveryCode` | every warning code, every reason code, every notice code | a sentence for each |

- [ ] **Step 1 (subagent):** Write the Go tests but the last three. Run
  them. Expected: fail.
- [ ] **Step 2 (subagent):** Implement. Run `go test -count=1 ./internal/collect/ ./internal/snapshot/ ./internal/document/` and `make lint`. Commit.

```bash
git add internal/collect internal/snapshot internal/document
git commit -m "feat(collect): run the five collectors in order"
```

- [ ] **Step 3 (main thread):** Extend and run the recording script over
  the seven scenarios. Read the seven files whole against the answers of
  each scenario. Rewrite the README and its SHA-256 table.
- [ ] **Step 4 (subagent):** Extend `original_test.go`: the recording
  struct reads the new keys (the ones this task compares; the others are
  read in Task 16), the sentence tables, the comparison of states and
  sentences, `TestQueriesEqualTheOriginal`. Run
  `go test -count=1 ./internal/goldentest/`. Expected: the query test
  fails because the project's lists are still short; the numbers test
  fails on the warnings of `partial-data`.
- [ ] **Step 5 (main thread):** Run `make update-testdata`. Read every
  changed file whole: a query list gains the lines of the original from
  line 44 on; a snapshot changes only in `quality.warnings`; a data file
  only in its notes. Run `go test -count=1 ./...` and `make lint`: the
  catalog check of the new codes still fails; every other package passes.
- [ ] **Step 6:** Commit.

```bash
git add internal/goldentest testdata/python testdata/queries testdata/snapshot template/tests/capacity/fixtures/command
git commit -m "test(golden): record the whole original"
```

**Done when:** every query list equals the original's, whole, and the
sentences of the five results equal the recorded ones in seven scenarios.
The golden pages of `command-partial-en` are stale from here until Task 17
regenerates them; the golden gate is not run in between.

---

### Task 9: The 26 codes and the analyses

**Executor:** Subagent.

**Files:**
- Modify: `internal/analysis/registry.go` (the 26 definitions appended, the scope constants), `registry_test.go`
- Create: `internal/analysis/hpas.go`, `nodes.go`, `pools.go`, `autoscaling.go`, with tests
- Modify: `internal/analysis/analysis.go` (`All`, `NodePressure`), `analysis_test.go`

**Interfaces:**
- Produces:

```go
const (
    ScopeHPA      = "hpa"
    ScopeNode     = "node"
    ScopeNodepool = "nodepool"
    ScopeCluster  = "cluster"
)

func HPAs(hpas []collect.HPA, uncovered []collect.WorkloadRef, t config.Thresholds) []Finding
func Nodes(nodes []collect.Node, t config.Thresholds) []Finding          // includes NODEPOOL_UNKNOWN
func Pools(pools []collect.Pool, nodes []collect.Node, t config.Thresholds) []Finding
func NodePressure(nodeFindings, poolFindings []Finding) bool
func Autoscaling(a collect.AutoscalingEvidence, t config.Thresholds, nodePressure bool) []Finding
func All(c collect.Collected, t config.Thresholds) []Finding // namespaces, workloads, HPAs, nodes, pools, autoscaling; Sort
```

**The definitions**, appended to the registry after the 40, in this order.
Families are named for the catalog of part 2.4; `Severities` lists what
the code may carry; evidence names and kinds in order.

| Code | Family | Severities | Scope | Evidence |
|---|---|---|---|---|
| `NODE_CPU_USAGE_HEALTHY`, `_ATTENTION`, `_CRITICAL` | `node_usage`, resource `cpu` | one each | node | `average_percent` number, `peak_percent` number, each present only when its value is known |
| `NODE_MEMORY_USAGE_HEALTHY`, `_ATTENTION`, `_CRITICAL` | `node_usage`, resource `memory` | one each | node | the same |
| `NODE_CPU_REQUEST_HIGH` | `node_request_high`, resource `cpu` | attention, critical | node | `percent` number |
| `NODE_MEMORY_REQUEST_HIGH` | `node_request_high`, resource `memory` | attention, critical | node | `percent` number |
| `NODE_OVERREQUESTED` | `node_overrequested` | attention | node | `resources` text |
| `NODE_LOW_UTILIZATION` | `node_low_utilization` | attention | node | `maximum_average_percent` number |
| `NODEPOOL_UNKNOWN` | `nodepool_unknown` | attention | nodepool | `affected_nodes` number |
| `NODEPOOL_CPU_UNEVEN`, `NODEPOOL_MEMORY_UNEVEN` | `nodepool_uneven`, resource | attention | nodepool | `minimum_peak_percent`, `maximum_peak_percent`, `spread_percent` numbers |
| `HPA_PINNED_REPLICAS` | `hpa_pinned` | healthy | hpa | `spec_min`, `spec_max` numbers |
| `HPA_MAX_REPLICAS_REACHED` | `hpa_max_reached` | critical | hpa | `current_peak`, `spec_max` |
| `HPA_NEAR_MAX_REPLICAS` | `hpa_near_max` | attention | hpa | `current_average`, `spec_max` |
| `HPA_REPLICA_DIVERGENCE` | `hpa_divergence` | attention | hpa | `average_difference` |
| `DEPLOYMENT_WITHOUT_HPA` | `deployment_without_hpa` | attention | workload | none |
| `STATEFULSET_WITHOUT_HPA_INFO` | `statefulset_without_hpa` | healthy | workload | none |
| `NODE_AUTOSCALING_UNKNOWN` | `autoscaling_unknown` | attention | cluster | none |
| `NODE_AUTOSCALING_DISABLED` | `autoscaling_disabled` | attention | cluster | `node_pressure` boolean |
| `NODE_AUTOSCALING_DISABLED_PRESSURE` | `autoscaling_disabled` | critical | cluster | `node_pressure` boolean |
| `NODE_AUTOSCALING_NEAR_MAX` | `autoscaling_near_max` | attention | cluster | `current_nodes`, `max_nodes` numbers |
| `NODE_AUTOSCALING_BOUNDS_UNKNOWN` | `autoscaling_bounds_unknown` | healthy | cluster | none |
| `NODE_AUTOSCALING_AT_MIN` | `autoscaling_at_min` | healthy | cluster | `min_nodes` number |
| `NODE_AUTOSCALING_ENABLED` | `autoscaling_enabled` | healthy | cluster | none |

The evidence of a usage code lists two names; a finding carries only the
ones whose number is known, in order, as the original does. The registry
check of the snapshot therefore matches the evidence of a finding as a
subsequence of the definition's list, in order, for the six usage codes,
and exactly for the others: add the flag `Optional bool` to `EvidenceSpec`
for `average_percent` and `peak_percent` of `node_usage`.

**The rules**, each read from the research document
`original-analysis-snapshot.md`, section 1, and the source it cites. In
short, with every comparison inclusive as written:

- Nodes, per node in name order, emitted in this order: CPU usage, memory
  usage, CPU request high, memory request high, overrequested, low
  utilisation. Usage: the higher of `Classify(average)` and
  `Classify(peak)` when at least one is known, confidence exact. Request
  high: `Classify(average of the request percent)` at attention or above,
  severity that classification, evidence `percent` the average. Over
  requested: the resources, `cpu` then `memory`, whose request high held
  and whose usage average is known and `<= LowUtilizationPercent`; when
  any, one finding with the text joined by a comma, confidence inferred.
  Low utilisation: the known usage averages, when any and their maximum
  `<= LowUtilizationPercent`, evidence that maximum. After the loop, when
  any node has the pool `unknown` or an unknown pool confidence, one
  `NODEPOOL_UNKNOWN` of scope name `unknown`, evidence the count,
  confidence unknown.
- Pools, per pool in name order, CPU then memory: the known usage peaks of
  its nodes; fewer than two, nothing; spread = maximum minus minimum;
  `>= NodepoolSpreadPoints`, one finding with the three numbers, confidence
  the pool's. The pool `unknown` is judged like any other.
- HPAs, per HPA in order, scope name `namespace/name`: first match of
  pinned (both specs known, finite, and equal; healthy; exact), maximum
  reached (spec max known, current peak known, `peak >= max`; critical),
  near maximum (spec max known and `> 0`, current average known,
  `average / max * 100 >= AutoscalerNearMaxPercent`; attention); then,
  independently, divergence (both averages known,
  `|current - desired| >= HPAReplicaDivergence`; attention; inferred). Then
  per uncovered workload in order: `Deployment` gives
  `DEPLOYMENT_WITHOUT_HPA`, `StatefulSet` gives
  `STATEFULSET_WITHOUT_HPA_INFO`, scope workload, scope name
  `namespace/name`, scope kind the kind, no evidence, exact; another kind
  nothing.
- Node pressure: any finding of the node or pool lists at attention or
  above.
- Autoscaling, first match: status `unknown` gives `NODE_AUTOSCALING_UNKNOWN`
  (confidence unknown); `disabled` gives `_DISABLED_PRESSURE` critical under
  pressure, else `_DISABLED` attention, evidence the boolean, confidence the
  evidence's; then, with max known and `> 0` and current known and
  `current / max * 100 >= AutoscalerNearMaxPercent`, `_NEAR_MAX`; then max
  or min unknown gives `_BOUNDS_UNKNOWN`; then current equal to min and no
  pressure gives `_AT_MIN`; else `_ENABLED`. All but the first carry the
  evidence's confidence.
- `All`: the six groups in the order namespaces, workloads, HPAs, nodes,
  pools, autoscaling, then `Sort`.

**Tests**, each with literal expected numbers:

| Test | Input | Expected |
|---|---|---|
| `TestRegistryHoldsSixtySixCodes` | the registry | 66 codes, none twice, the 26 in the order above with the fields of the table |
| `TestNodeUsageTakesTheHigher` | average `60`, peak `86` | `NODE_CPU_USAGE_CRITICAL`, evidence `60`, `86` |
| `TestNodeUsageWithOneNumber` | peak only `50` | `_HEALTHY` with evidence `peak_percent` `50` only |
| `TestNodeRequestHighReadsTheAverage` | request average `90` | `NODE_CPU_REQUEST_HIGH` critical, evidence `percent` `90` |
| `TestNodeOverrequested` | CPU request `90`, CPU usage average `20`; memory request `72`, usage average `25` | `resources` `cpu,memory`, inferred |
| `TestNodeOverrequestedNeedsBoth` | request `90`, usage average `31` | no finding |
| `TestNodeLowUtilization` | usage averages `20` and `25` | evidence `25`; with `20` and `31`, none |
| `TestNodepoolUnknownCountsOnce` | two nodes in `unknown`, one exact | one finding, `affected_nodes` `2`, scope name `unknown` |
| `TestNodepoolUneven` | peaks `20`, `60` | `NODEPOOL_CPU_UNEVEN` `20`, `60`, `40`; with `20`, `49`, none; with one node, none |
| `TestHPAPinnedWinsOverMax` | min `3` max `3` peak `3` | `HPA_PINNED_REPLICAS` only |
| `TestHPAMaxReached` | max `4`, peak `4` | critical, `4`, `4` |
| `TestHPAMaxReachedAtZero` | max `0`, peak `0` | critical (review focus 2) |
| `TestHPANearMax` | max `5`, average `4` | attention, `4`, `5`; with max `0` nothing |
| `TestHPADivergence` | current average `1`, desired `2` | `1`, inferred; also beside a max-reached |
| `TestHPAUnknownSpecSkips` | max unknown | neither max nor near max |
| `TestWorkloadsWithoutHPA` | `Deployment api`, `StatefulSet queue`, `DaemonSet d` | two findings, kinds as scope kinds, the third nothing |
| `TestNodePressure` | a `NODE_LOW_UTILIZATION` only | true; a healthy usage only, false |
| `TestAutoscalingChain` | the seven states of the table above, with and without pressure | each code, its severity, its evidence, its confidence |
| `TestAutoscalingNearMaxIsIntegerDivision` | current `3`, max `3` | `100`, near max |
| `TestAllOrdersSixGroups` | one finding of each group | `Sort` applied; the node pressure computed from nodes and pools only |
| `TestEveryFindingFollowsItsDefinition` | rows that give each of the 26 codes | severity in `Severities`, scope type, evidence names and kinds as defined |

- [ ] **Step 1:** Write the tests. Run them. Expected: fail.
- [ ] **Step 2:** Implement.
- [ ] **Step 3:** Run `go test -count=1 ./internal/analysis/ ./internal/snapshot/` and `make lint`. The snapshot tests still pass: no code of the 26 reaches a snapshot before Task 10.
- [ ] **Step 4:** Commit.

```bash
git add internal/analysis
git commit -m "feat(analysis): judge HPAs, nodes, pools, autoscaling"
```

**Done when:** the tests pass and the registry holds 66 codes.

---
### Task 10: The snapshot

**Executor:** Subagent, then the main thread for the golden files.

**Files:**
- Modify: `internal/snapshot/snapshot.go`, `build.go`, `codec.go`, their tests, `internal/snapshot/testdata/` if a fixture is added
- Rewritten by `make update-testdata`: `testdata/snapshot/*.json` (every file), `template/tests/capacity/fixtures/command/*.json` (unchanged bytes, because the document does not read the new sections until Task 13; the command checks it)

**Interfaces:**
- Produces, in declaration order, which is the JSON order:

```go
type Snapshot struct {
    SchemaVersion string      `json:"schema_version"`
    Report        Report      `json:"report"`
    Scope         Scope       `json:"scope"`
    Quality       Quality     `json:"quality"`
    Summary       Summary     `json:"summary"`
    Namespaces    []Namespace `json:"namespaces"`
    Workloads     []Workload  `json:"workloads"`
    HPAs          []HPA       `json:"hpas"`
    Nodes         []Node      `json:"nodes"`
    Nodepools     []NodePool  `json:"nodepools"`
    Findings      []Finding   `json:"findings"`
}

type Summary struct {
    Namespaces          float64     `json:"namespaces"`
    Workloads           float64     `json:"workloads"`            // observed rows only
    HPAs                float64     `json:"hpas"`
    Nodes               float64     `json:"nodes"`
    Nodepools           float64     `json:"nodepools"`
    Findings            float64     `json:"findings"`
    Healthy             float64     `json:"healthy"`
    Attention           float64     `json:"attention"`
    Critical            float64     `json:"critical"`
    WorkloadsWithoutHPA float64     `json:"workloads_without_hpa"` // rows with hpa_coverage uncovered
    Autoscaling         Autoscaling `json:"autoscaling"`
}

type Autoscaling struct {
    Status            string   `json:"status"`     // enabled, disabled, unknown
    Source            string   `json:"source"`     // configured, raw_metrics, none
    Confidence        string   `json:"confidence"`
    ManagedNodeGroups *float64 `json:"managed_node_groups"`
    CurrentNodes      *float64 `json:"current_nodes"`
    MinNodes          *float64 `json:"min_nodes"`
    MaxNodes          *float64 `json:"max_nodes"`
    ScaleEvents       *float64 `json:"scale_events"`
    UnschedulablePods *float64 `json:"unschedulable_pods"`
}

type Workload struct {
    Namespace     string   `json:"namespace"`
    Kind          string   `json:"kind"`
    Name          string   `json:"name"`
    ObservedUsage bool     `json:"observed_usage"`
    HPACoverage   string   `json:"hpa_coverage"` // covered, uncovered, unknown
    Pods          *float64 `json:"pods"`
    Approximate   bool     `json:"approximate"`
    // the ten Statistics as before
}

type HPA struct {
    Namespace  string     `json:"namespace"`
    Name       string     `json:"name"`
    TargetKind string     `json:"target_kind"`
    TargetName string     `json:"target_name"`
    SpecMin    *float64   `json:"spec_min"`
    SpecMax    *float64   `json:"spec_max"`
    Current    Statistics `json:"current"`
    Desired    Statistics `json:"desired"`
}

type Node struct {
    Name                 string     `json:"name"`
    Pool                 string     `json:"pool"`
    PoolConfidence       string     `json:"pool_confidence"`
    CPUCapacity          *float64   `json:"cpu_capacity"`
    MemoryCapacity       *float64   `json:"memory_capacity"`
    CPUAllocatable       *float64   `json:"cpu_allocatable"`
    MemoryAllocatable    *float64   `json:"memory_allocatable"`
    CPUUsagePercent      Statistics `json:"cpu_usage_percent"`
    MemoryUsagePercent   Statistics `json:"memory_usage_percent"`
    CPURequestPercent    *float64   `json:"cpu_request_percent"`
    MemoryRequestPercent *float64   `json:"memory_request_percent"`
    Ready                *bool      `json:"ready"`
    Schedulable          *bool      `json:"schedulable"`
}

type NodePool struct {
    Name       string   `json:"name"`
    Confidence string   `json:"confidence"`
    Nodes      []string `json:"nodes"` // sorted
}
```

**Requirements:**

- `supportedKey` and `checkKeys` accept `*bool` (null or a boolean). The
  hand-written `contractPaths` of `codec_test.go` and its count are
  rewritten for the new keys in declaration order.
- `Build`: `summary` from the lists it builds; `workloads` as R39 says,
  coverage as R48 says, the coverage-only rows with `ObservedUsage` false,
  `Pods` nil, `Approximate` false, ten statistics unavailable; `hpas` from
  `Collected.HPAs.Rows`; `nodes` from `Collected.Nodes.Rows`; `nodepools`
  from `collect.Pools`; `autoscaling` from `Collected.Autoscaling` with the
  counts as whole numbers.
- `Check`, added rules in this order after the workload rules: `summary`
  counts equal the lengths and the severity counts of the findings; the
  autoscaling status, source, and confidence in their enums, a configured
  source only with a status `enabled` or `disabled`, a status `unknown`
  only with source `none` and confidence `unknown`, every count a whole
  number at or above zero; `hpas` sorted by namespace then name with no
  repeat, namespace in the scope, name non-empty, no slash in either,
  `target_kind` and `target_name` both empty or both set, specs finite
  when set; `nodes` sorted by name with no repeat, name non-empty, pool
  non-empty, confidence in the enum, numbers finite and at or above zero,
  request percents at or above zero; `nodepools` sorted by name with no
  repeat, each node name a node of the snapshot exactly once across pools,
  every node in exactly one pool, the pool's confidence the lowest of its
  nodes, and the pool of each node row equal to the pool that lists it
  (R50); `hpa_coverage` in the enum; an uncovered row is the target of no
  HPA row (`namespace`, `target_name`, `target_kind` equal to its
  namespace, name, kind); a row with `observed_usage` false has
  `hpa_coverage` uncovered, no pods, and ten unavailable statistics;
  `checkFindingScope` learns the four scope types: `hpa` names an HPA row
  by `namespace/name`; `node` names a node; `nodepool` names a pool;
  `cluster` is the name `cluster`; the two coverage codes name an
  uncovered workload row with their scope kind equal to its kind.
- `Decode` of `testdata/schema-1.0-before-rows.json` still fails with
  `snapshot lacks the key quality.warnings`: `summary` comes after
  `quality` in the walk.

**Tests:**

| Test | Input | Expected |
|---|---|---|
| `TestBuildFillsTheNewSections` | a `Collected` with one of each | every section filled, `summary` counts right, `hpa_coverage` of each row right |
| `TestBuildAddsUncoveredRows` | a measured `web` covered, an uncovered `api` never measured | rows `api` (uncovered, not observed, no numbers) and `web` (covered, observed) in sorted order |
| `TestBuildMarksUnknownCoverage` | a measured workload in an ambiguous namespace | `unknown` |
| `TestBuildRefusesAnUncoveredTarget` | an uncovered row that an HPA targets | `Check` fails naming the row |
| `TestBuildRefusesAPoolNamingAnAbsentNode` | pool `a` with node `ghost` | fails naming the pool and the node |
| `TestBuildRefusesANodeInTwoPools` | | fails |
| `TestBuildRefusesAnHPAOutOfOrder` | `b` before `a` | fails |
| `TestBuildRefusesAWrongSummary` | `nodes` count off by one | fails |
| `TestBuildRefusesAnAutoscalingContradiction` | status `unknown` with source `configured` | fails |
| `TestBuildRefusesAScopeThatDoesNotExist` | a node finding for `ghost`; an HPA finding for `x/y`; a coverage finding for a covered row | each fails |
| `TestDecodeNullBoolean` | `ready: null` | decodes to nil; `ready: 1` refused as the wrong kind |
| `TestDecodeRefusesMissingNewSection` | `schema-1.0-before-rows.json` | still `snapshot lacks the key quality.warnings` |
| `TestKeyPathsAreTheContract` | the walk | the new count, every new path in order |
| `TestBuildEveryFieldChangesTheBytes` | each new input field changed | the bytes change |

- [ ] **Step 1:** Write the tests. Run them. Expected: fail.
- [ ] **Step 2:** Implement.
- [ ] **Step 3:** Run `go test -count=1 ./internal/snapshot/ ./internal/document/ ./internal/cli/` and `make lint`. The goldentest package fails on `TestGoldenSnapshots` until Step 4.
- [ ] **Step 4 (main thread):** Run `make update-testdata`. Read every changed snapshot whole against the answers of its scenario: the new sections, the coverage of each workload, the findings of the 26 codes with their evidence values. The data files must not change: if one did, stop, because the document read a section it does not print yet.
- [ ] **Step 5:** Run `go test -count=1 ./...` and `make lint`. Expected: the catalog check of the new codes is the only failure.
- [ ] **Step 6:** Commit.

```bash
git add internal/snapshot testdata/snapshot
git commit -m "feat(snapshot): store HPAs, nodes, pools, and the summary"
```

**Done when:** the tests pass, every golden snapshot holds the new sections
with numbers that the answers explain, and the data files are unchanged.

---

### Task 11: The formats of this part

**Executor:** Subagent.

**Files:**
- Modify: `internal/catalog/format.go`, `format_test.go`
- Modify: `internal/catalog/keys.go` (the eight period keys), `keys_test.go`, `locales/pt-BR.toml`, `locales/en.toml` (the eight entries: main thread wording, given below)

**Interfaces:**
- Produces:

```go
// Period writes the length of a period as the original does: whole seconds
// rounded half to even, then days for seven days or more when the count is
// exact, else hours when exact, else minutes when exact, else seconds; the
// unit word from the catalog, plural for every value but 1.
func (c *Catalog) Period(start, end time.Time) (string, error)
```

  Keys: `PeriodDaySingular = "period.day.singular"`, `PeriodDayPlural`,
  and likewise `hour`, `minute`, `second`. Entries:

| Key | pt-BR | en |
|---|---|---|
| `period.day.singular` | dia | day |
| `period.day.plural` | dias | days |
| `period.hour.singular` | hora | hour |
| `period.hour.plural` | horas | hours |
| `period.minute.singular` | minuto | minute |
| `period.minute.plural` | minutos | minutes |
| `period.second.singular` | segundo | second |
| `period.second.plural` | segundos | seconds |

  The count is written with `Integer`. `Count` serves replica counts and
  node counts; `Decimal(v, 1)` the replica average; no other formatter
  enters.

**Tests:**

| Test | Input (start to end) | pt-BR | en |
|---|---|---|---|
| `TestPeriod` | 24 h | `24 horas` | `24 hours` |
| | 7 d | `7 dias` | `7 days` |
| | 6 d | `144 horas` | `144 hours` |
| | 1 h | `1 hora` | `1 hour` |
| | 90 min | `90 minutos` | `90 minutes` |
| | 90 s | `90 segundos` | `90 seconds` |
| | 1 s | `1 segundo` | `1 second` |
| | 0 | `0 segundos` | `0 seconds` |
| | 8 d | `8 dias` | `8 days` |
| | 2.5 s | `2 segundos` (half to even) | |
| `TestPeriodNeedsTheEntries` | a catalog without them | an error naming the key |
| `TestEveryKeyIsUsed` | | the eight keys used by `Period` |

- [ ] **Step 1:** Write the tests. Run them. Expected: fail.
- [ ] **Step 2:** Implement.
- [ ] **Step 3:** Run `go test -count=1 ./internal/catalog/` and `make lint`.
- [ ] **Step 4:** Commit.

```bash
git add internal/catalog
git commit -m "feat(catalog): format a period"
```

**Done when:** the tests pass.

---

### Task 12: The wording of this part

**Executor:** Main thread writes the entries below into both files; a
subagent adds the keys, the tables of the guard tests, and the constants,
and runs the guards.

**Files:**
- Modify: `internal/catalog/keys.go`, `keys_test.go`, `guard_test.go`, `locales/pt-BR.toml`, `locales/en.toml`

**Interfaces:**
- Produces the key constants below, in `wordingKeys`, plus two key functions:

```go
func Confidence(confidence string) Key            // "confidence." + exact|inferred|unknown
func Reason(family, resource string) Key          // "reason." + family + "." + resource
func FindingSummary(family string) Key            // "finding." + family + ".summary"
```

**The entries.** Portuguese under the wording rule; English new. The
Portuguese sentences of the original are in `original-document.md`,
sections 1, 2, and 7, and in `original-collection.md`, sections 2.7 and
3.9.

| Key | pt-BR | en |
|---|---|---|
| `sections.hpas.heading` | Histórico de HPA | HPA history |
| `sections.hpas.intro` | Comportamento dos Horizontal Pod Autoscalers no período de {{.Period}}. A situação indica se o máximo configurado foi observado. | Behaviour of the Horizontal Pod Autoscalers over the period of {{.Period}}. The situation says whether the configured maximum was observed. |
| `sections.hpas.observed_heading` | HPAs observados | Observed HPAs |
| `sections.hpas.uncovered_heading` | Workloads sem HPA | Workloads without HPA |
| `sections.hpas.header_hpa` | HPA | HPA |
| `sections.hpas.header_spec_min` | Spec Min | Spec min |
| `sections.hpas.header_spec_max` | Spec Max | Spec max |
| `sections.hpas.header_current_average` | Atual (Avg) | Current (avg) |
| `sections.hpas.header_min_reached` | Mín Atingido | Min reached |
| `sections.hpas.header_max_reached` | Máx Atingido | Max reached |
| `sections.hpas.header_situation` | Situação | Situation |
| `sections.hpas.header_kind` | Tipo | Kind |
| `sections.hpas.header_workload` | Workload | Workload |
| `sections.hpas.header_pods` | Qtde. Pods | Pods |
| `sections.hpas.situation_max_reached` | Máximo atingido | Maximum reached |
| `sections.hpas.situation_pinned` | Fixado | Pinned |
| `sections.hpas.situation_below_max` | Abaixo do máximo | Below the maximum |
| `sections.hpas.situation_no_hpa` | Sem HPA | No HPA |
| `sections.hpas.situation_specific_strategy` | Estratégia de escala específica | Specific scaling strategy |
| `sections.hpas.empty_namespace` | Nenhum HPA ou workload sem HPA foi observado. | No HPA or workload without HPA was observed. |
| `sections.hpas.empty_namespace_partial` | Nenhum registro válido foi obtido para este namespace. | No valid record was obtained for this namespace. |
| `sections.hpas.note_incomplete_title` | Dados incompletos | Incomplete data |
| `sections.hpas.note_incomplete` | Parte dos dados de HPA está indisponível para o período analisado. | Part of the HPA data is unavailable for the analysis period. |
| `sections.hpas.note_partial_title` | Cobertura parcial | Partial coverage |
| `sections.hpas.note_partial` | A cobertura de HPA está parcial; somente registros válidos são exibidos. | HPA coverage is partial; only valid records are shown. |
| `sections.nodepools.heading` | Uso de recursos por node pool | Resource usage by node pool |
| `sections.nodepools.summary_heading` | Sumário por node pool | Summary by node pool |
| `sections.nodepools.profile_heading` | Perfil de uso agregado por node pool | Aggregate usage profile by node pool |
| `sections.nodepools.detail_heading` | Detalhamento por node | Detail by node |
| `sections.nodepools.pool_label` | Node pool: | Node pool: |
| `sections.nodepools.header_pool` | Node pool | Node pool |
| `sections.nodepools.header_nodes` | Nodes | Nodes |
| `sections.nodepools.header_cpu_average` | CPU média | Average CPU |
| `sections.nodepools.header_cpu_peak` | CPU pico | Peak CPU |
| `sections.nodepools.header_memory_average` | Memória média | Average memory |
| `sections.nodepools.header_memory_peak` | Memória pico | Peak memory |
| `sections.nodepools.header_confidence` | Confiança | Confidence |
| `sections.nodepools.header_cpu_allocatable` | CPU alocável/node | Allocatable CPU/node |
| `sections.nodepools.header_cpu_used` | CPU usada/node | Used CPU/node |
| `sections.nodepools.header_cpu_requested` | CPU solicitada/node | Requested CPU/node |
| `sections.nodepools.header_memory_allocatable` | Memória alocável/node | Allocatable memory/node |
| `sections.nodepools.header_memory_used` | Memória usada/node | Used memory/node |
| `sections.nodepools.header_memory_requested` | Memória solicitada/node | Requested memory/node |
| `sections.nodepools.header_node` | Node | Node |
| `sections.nodepools.header_cpu_avg_peak` | CPU méd/pico | CPU avg/peak |
| `sections.nodepools.header_memory_avg_peak` | Mem méd/pico | Mem avg/peak |
| `sections.nodepools.header_reason` | Motivo | Reason |
| `sections.nodepools.header_state` | Estado | State |
| `sections.nodepools.empty_summary` | Nenhum node pool foi identificado. | No node pool was identified. |
| `sections.nodepools.empty_profile` | Nenhum perfil agregado está disponível. | No aggregate profile is available. |
| `sections.nodepools.empty_detail` | Nenhum node foi identificado para os filtros atuais. | No node was identified for the current filters. |
| `sections.nodepools.no_finding` | Sem achado | No finding |
| `node_state.ready` | Pronto | Ready |
| `node_state.not_ready` | Não pronto | Not ready |
| `node_state.schedulable` | Agendável | Schedulable |
| `node_state.unschedulable` | Não agendável | Unschedulable |
| `confidence.exact` | Exata | Exact |
| `confidence.inferred` | Inferida | Inferred |
| `confidence.unknown` | Desconhecida | Unknown |
| `placeholder.undetermined` | Indeterminado | Undetermined |
| `reason.usage_peak.cpu` | CPU: pico de {{.Percent}} de utilização | CPU: peak of {{.Percent}} utilization |
| `reason.usage_peak.memory` | Memória: pico de {{.Percent}} de utilização | Memory: peak of {{.Percent}} utilization |
| `reason.usage_average.cpu` | CPU: média de {{.Percent}} de utilização | CPU: average of {{.Percent}} utilization |
| `reason.usage_average.memory` | Memória: média de {{.Percent}} de utilização | Memory: average of {{.Percent}} utilization |
| `reason.request_high.cpu` | CPU: {{.Percent}} solicitada do alocável | CPU: {{.Percent}} of the allocatable requested |
| `reason.request_high.memory` | Memória: {{.Percent}} solicitada do alocável | Memory: {{.Percent}} of the allocatable requested |
| `finding.node_overrequested.summary` | Requests elevados coexistem com baixa utilização real observada. | High requests coexist with low observed utilization. |
| `finding.node_low_utilization.summary` | O node apresentou baixa utilização real média. | The node showed low average utilization. |
| `warning.HPA_AMBIGUOUS_SPEC` | Valores ambíguos de configuração ou alvo de HPA foram omitidos. | Ambiguous HPA configuration or target values were omitted. |
| `warning.HPA_AMBIGUOUS_TARGET` | Alvos ambíguos de HPA impedem uma cobertura confiável dos workloads. | Ambiguous HPA targets prevent reliable coverage of the workloads. |
| `warning.NODE_IDENTITY_UNAVAILABLE` | As métricas de identidade dos nodes estão indisponíveis. | The identity metrics of the nodes are unavailable. |
| `warning.NODE_CAPACITY_EVIDENCE_INCOMPLETE` | As evidências de capacidade dos nodes estão incompletas. | The capacity evidence of the nodes is incomplete. |
| `notice.WORKLOADS_OMITTED_SCALE_ZERO.singular` | 1 workload deixado de fora do inventário de cobertura por estar com escala zero. | 1 workload was left out of the coverage inventory because it is scaled to zero. |
| `notice.WORKLOADS_OMITTED_SCALE_ZERO.plural` | {{.Count}} workloads deixados de fora do inventário de cobertura por estarem com escala zero. | {{.Count}} workloads were left out of the coverage inventory because they are scaled to zero. |

  The guard tables of `guard_test.go` gain these rows, so that the
  approved wording is pinned. `TestEveryKeyIsUsed` fails until Task 13
  uses the keys of the sections; the brief says so, and Task 13 makes it
  pass.

**Tests:**

| Test | Input | Expected |
|---|---|---|
| `TestWordingMatchesThePlan`, `TestPortugueseOfTheSectionsMatchesThePlan` | both files | every row above |
| `TestNoBannedTerm` | both files | no entry named |
| `TestEveryWarningHasItsEntry`, `TestEveryNoticeHasBothEntries` | the registries | pass again |
| `TestLanguagesUseTheSameParameters` | | `Period`, `Percent`, `Count` agree |
| `TestKeysHoldEveryConstant` | | pass |

- [ ] **Step 1 (main thread):** Write the entries into both files.
- [ ] **Step 2 (subagent):** Add the constants, the key functions, the table rows. Run `go test -count=1 ./internal/catalog/ -run 'Wording|Portuguese|Banned|Warning|Notice|Parameters|Constant' -v`. Expected: pass. Run the whole catalog package: `TestEveryKeyIsUsed` names the keys of the sections and nothing else.
- [ ] **Step 3:** Run `make lint`. Commit.

```bash
git add internal/catalog
git commit -m "feat(catalog): word sections 3 and 4"
```

**Done when:** every guard but `TestEveryKeyIsUsed` passes, and that one
names only keys that Task 13 prints.

---

### Task 13: The document

**Executor:** Subagent.

**Files:**
- Modify: `internal/document/document.go` (`Callout`, `Placeholder`), `document_test.go`
- Create: `internal/document/hpas.go`, `hpas_test.go`, `internal/document/nodepools.go`, `nodepools_test.go`
- Modify: `internal/document/build.go` (the block order), `front.go` and `appendix.go` (the effective capabilities, R56), `sections_test.go` (the parity test grows), their tests
- Rewritten by `make update-testdata` on the main thread: `template/tests/capacity/fixtures/command/*.json`

**Interfaces:**
- Produces:

```go
// Callout is a note beside the flow of the text.
type Callout struct {
    Kind  string // note, info, warning
    Title []Run  // nil for no title
    Runs  []Run
}
// JSON: {"type": "callout", "kind": ..., "title": [...] or null, "runs": [...]}

// Placeholder is a run that stands for a value the language names, such
// as an undetermined pool.
func Placeholder(text string) Run // kind "placeholder"

func buildHPAsSection(b *builder, s snapshot.Snapshot) []Block
func buildNodepoolsSection(b *builder, s snapshot.Snapshot, thresholds config.Thresholds, severity string) []Block

// effectiveCapabilities returns the capabilities of the snapshot with an
// available hpa_history turned partial when the warning HPA_AMBIGUOUS_SPEC
// is present, and an available workloads_without_hpa turned partial when
// HPA_AMBIGUOUS_TARGET is, the warning's code standing as the one reason.
func effectiveCapabilities(s snapshot.Snapshot) []snapshot.Capability
```

**Requirements, the order:** front, page break, section 1, section 2,
section 3, section 4, weak page break, appendix, notes. The front counts
and the appendix read `effectiveCapabilities`; the remark of a capability
turned partial is the sentence of the warning.

**Requirements, section 3** (`original-document.md`, section 1; the
original applies no minimum severity here, `R/workloads.py:425-549`; an
implementer that finds one stops and reports):

1. Heading level 1, numbered, `sections.hpas.heading`.
2. Paragraph `sections.hpas.intro` with `Period` = `Period(period_start,
   period_end)` of the report.
3. The note: the effective states of `hpa_history` and
   `workloads_without_hpa`; any `unavailable` gives `Callout{note,
   [Text(note_incomplete_title)], [Text(note_incomplete)]}`; else any
   `partial` gives the partial note; else nothing.
4. For each namespace of `scope.namespaces` (R55): heading level 2,
   unnumbered, `sections.namespace_label` + space, then `Ident(namespace)`.
   Then the HPAs of the namespace whose current peak is not a known zero
   (R49), sorted by name: when any, heading level 3 unnumbered
   `observed_heading` and the table `hpa-observed`. Then the workload rows
   of the namespace with `hpa_coverage` `uncovered`, sorted by kind then
   name: when any, heading level 3 unnumbered `uncovered_heading` and the
   table `workloads-without-hpa`. When both are empty, one paragraph:
   `empty_namespace_partial` when the note exists, else `empty_namespace`.
5. Cells of `hpa-observed`: `Ident(name)`; `Count(spec_min)` or
   `Text(placeholder.unavailable)`; the same for spec max; `Decimal(current
   average, 1)` or unavailable; `Count(current minimum)`; `Count(current
   peak)`; the situation, first match: spec max known, peak known, not
   pinned, `peak >= max` gives `StatusText(critical, situation_max_reached)`;
   both specs known and equal gives `StatusText(healthy, situation_pinned)`;
   spec max and peak known gives `StatusText(healthy, situation_below_max)`;
   else `StatusText(unavailable, placeholder.unavailable)`.
6. Cells of `workloads-without-hpa`: `Text(kind)`; `Ident(name)`; the pods
   of the row through `Count`, or `Text(placeholder.unavailable)` when nil;
   `Deployment` gives `StatusText(attention, situation_no_hpa)`, any other
   kind `StatusText(unavailable, situation_specific_strategy)`.

**Requirements, section 4** (`original-document.md`, section 2):

1. Heading level 1, numbered, `sections.nodepools.heading`.
2. The pool filter of the summary and the profile: a pool stays when the
   highest severity of the findings of scope `nodepool` and its name is at
   or above the minimum (`scopeSeverity`); the node filter of the detail: a
   node stays when the highest severity of the findings of scope `node`
   and its name is at or above the minimum. The two filters differ, as in
   the original.
3. Summary: heading level 2, numbered, `summary_heading`; when no pool
   passes, a paragraph `empty_summary`; else the table `nodepool-summary`:
   the pool name as `Ident`, or `Placeholder(placeholder.undetermined)` for
   the pool `unknown`; `Count(number of nodes)`; the CPU average as
   `arith.Mean` of the known usage averages of its nodes, shown as
   `StatusText(Classify(v), Percent(v))`, or
   `StatusText(unavailable, placeholder.unavailable)` when none is known;
   the CPU peak the maximum of the known peaks, likewise; memory likewise;
   `Text(confidence.<confidence>)`.
4. Profile: heading level 2, numbered, `profile_heading`; the same filter;
   paragraph `empty_profile` or the table `nodepool-profile`: the pool
   name; the mean of the known CPU allocatables as `CPU(v)`; the mean over
   the nodes that have both of `allocatable * usage average / 100`,
   evaluated in that order; the same with the request percent; then the
   three of memory with `Memory(v)`; an unknown mean is
   `Text(placeholder.unavailable)`.
5. Detail: heading level 2, numbered, `detail_heading`, always. For each
   pool in order, the nodes that pass the node filter; a pool with none is
   skipped; else heading level 3 unnumbered `pool_label` + space then the
   pool name run, and the table `node-detail`: `Ident(name)`; the cell
   `[percent status of the CPU average, Text(" / "), percent status of the
   CPU peak]`, an unknown number as `StatusText(unavailable,
   placeholder.unavailable)`; memory likewise; the reason; the state
   `Text(<ready word>; <schedulable word>)` with `node_state.ready` or
   `not_ready` or `placeholder.unavailable`, then `; `, then
   `node_state.schedulable` or `unschedulable` or the placeholder. When no
   pool had a row, a paragraph `empty_detail` under the heading.
6. The reason: the findings of scope `node` and the node's name, in the
   order of the snapshot; none gives `Text(no_finding)`; else the highest
   severity among them, and the fragments of the findings of that severity
   in order, each once, joined by `; `, as `StatusText(severity, joined)`.
   Fragment of a code: `NODE_<R>_USAGE_*` with `peak_percent` in its
   evidence gives `reason.usage_peak.<r>` with `Percent`; with
   `average_percent` only, `reason.usage_average.<r>`; `NODE_<R>_REQUEST_HIGH`
   gives `reason.request_high.<r>` with its `percent`; `NODE_OVERREQUESTED`
   gives `finding.node_overrequested.summary` with every trailing period
   removed; `NODE_LOW_UTILIZATION` likewise its summary. `<r>` is `cpu` or
   `memory` from the code.

**Requirements, the parity test:** `TestDocumentEqualsParityForItsSections`
grows to blocks 2 to 22 of the fixture, and to its last two blocks, the
notes. The snapshot given to the builders holds, beside the rows of part
2.2: the HPA `payments/api`, target `Deployment/api`, spec min `2`, spec
max `10`, current average `7`, minimum `2`, peak `10`, current `10`; an
uncovered `Deployment worker` with pods `2` (an observed row, uncovered);
the node `worker-a` in pool `apps`, exact, allocatable `7.5` cores and
`30` bytes, CPU usage average `55` peak `78`, memory `64` and `80`, CPU
request percent `60`, memory `70`, ready and schedulable; the findings
`NODE_CPU_USAGE_ATTENTION` (55, 78) and `NODE_MEMORY_USAGE_ATTENTION` (64,
80); the warning `WORKLOAD_HISTORY_CURRENT_ASSOCIATION`. One cell of the
fixture cannot be produced: the "Mín Atingido" of `api` reads
`Indisponível` while its average and peak are known, a state a statistic
of the snapshot cannot hold, so the test corrects that one cell of the
expected side to `2` through a table `parityCorrections` whose comment
says why. The text of the note of the fixture is mapped to the sentence of
that warning through `parityTexts` (F-62). Everything else is compared as
it is, the texts through the catalog as in part 2.2b.

**Tests:**

| Test | Input | Expected |
|---|---|---|
| `TestCalloutMarshals` | a callout with and without title | `{"type":"callout","kind":"note","title":[...],"runs":[...]}`; `"title":null` |
| `TestPlaceholderRun` | | `{"kind":"placeholder","text":"..."}` |
| `TestBuildOrdersTheSections` | the golden snapshot of `direct-complete` | headings 1 to 4 numbered in order, then the appendix |
| `TestHPASectionNote` | states unavailable / partial / available; a warning `HPA_AMBIGUOUS_SPEC` beside available | incomplete note / partial note / none / partial note |
| `TestHPASectionHidesScaledToZero` | an HPA with peak `0` | not in the table; a peak unknown is kept |
| `TestHPASectionSituation` | the four cases | the four texts and marks; a max of `0` with peak `0` is "maximum reached" |
| `TestHPASectionEmptyNamespace` | no HPA, no uncovered; with and without note | the two paragraphs |
| `TestHPASectionOneTableOnly` | uncovered only | one heading level 3 and one table, no paragraph |
| `TestUncoveredRowCells` | a Deployment with pods `2`; a StatefulSet with no pods | `2`, `Sem HPA` attention; `Indisponível`, `Estratégia de escala específica` unavailable |
| `TestPoolFilterReadsPoolFindingsOnly` | a pool with a critical node and no pool finding, minimum `attention` | absent from summary and profile; the node present in the detail |
| `TestPoolSummaryMeans` | nodes with averages `20` and `57.5` | `38,8%`; peaks `20`, `60` gives `60%` |
| `TestPoolSummaryUnknownPool` | pool `unknown` | placeholder `Indeterminado`, confidence `Desconhecida` |
| `TestPoolProfileMeans` | allocatable `7.5`, usage `55`, request `60` | `7,5 cores`, `4,12 cores`, `4,5 cores`; memory `30`, `64`, `70` gives `30 B`, `19,2 B`, `21 B` |
| `TestPoolProfileSubsetMeans` | one node with allocatable only | used and requested unavailable, allocatable known |
| `TestNodeDetailReason` | findings of two severities | the higher severity's fragments only, in order, once each, joined by `; ` |
| `TestNodeDetailReasonFragments` | each of the six node codes | the six texts |
| `TestNodeDetailNoFinding` | none | `Sem achado` |
| `TestNodeDetailState` | the nine combinations | the joined texts |
| `TestNodeDetailEmpty` | every node filtered out | the paragraph under the heading |
| `TestEffectiveCapabilities` | available `hpa_history` with the spec warning | partial with reason `HPA_AMBIGUOUS_SPEC`; the front counts move one from available to partial; the appendix remark is the warning's sentence |
| `TestDocumentEqualsParityForItsSections` | as above | blocks 2 to 22 and the two notes blocks equal |
| `TestEveryKeyIsUsed` (catalog) | | passes again |

- [ ] **Step 1:** Write the tests. Run them. Expected: fail.
- [ ] **Step 2:** Implement.
- [ ] **Step 3:** Run `go test -count=1 ./internal/document/ ./internal/catalog/` and `make lint`.
- [ ] **Step 4 (main thread):** Run `make update-testdata`. Read every changed data file whole against its snapshot. Run `go test -count=1 ./...`.
- [ ] **Step 5:** Commit.

```bash
git add internal/document internal/catalog template/tests/capacity/fixtures/command
git commit -m "feat(document): print HPAs, node pools, and nodes"
```

**Done when:** the tests pass, every guard of the catalog passes, and the
parity test covers blocks 2 to 22 and the notes.

---

### Task 14: The command

**Executor:** Subagent.

**Files:**
- Modify: `internal/cli/run.go` (the four lines; `CheckUseful` after `collect.All` in `runSnapshot` and `runPDF`; `classify` maps `collect.ErrNoUsefulSections` to exit code 5), `run_test.go`, `report_real_test.go` (the headings of sections 3 and 4 in the PDF text)

**Requirements:**

- After `workload collection: <state>` the lines `hpa collection:
  <state>`, `hpa coverage collection: <state>`, `node collection:
  <state>`, `autoscaling collection: <status>` (R54).
- `CheckUseful` runs after the collectors and before the second reading of
  the clock; its failure prints `error: no useful report sections were
  collected` and exits 5; no snapshot, no PDF, no file is written.
- `--check` sends no query of a collector, as before.

**Tests:**

| Test | Input | Expected |
|---|---|---|
| `TestRunPrintsTheCollectionLines` | `direct-complete` | the six lines in order with the states of the scenario |
| `TestNoUsefulSectionsExitsFive` | a fixture built in the test whose probes answer but whose collectors answer nothing and whose autoscaler metrics are absent | exit 5, the one line on stderr, nothing on stdout, no file; the server saw the queries of the five collectors |
| `TestNoUsefulSectionsSavedByConfiguredAutoscaling` | the same with `--node-autoscaling disabled` | exit 0 |
| `TestCheckSendsNoCollectorQuery` | `--check` | unchanged: the preflight requests only |
| `TestRealReportHoldsSectionsThreeAndFour` (tag template) | each scenario and language | the headings of sections 3 and 4 in the PDF text, each once |
| `TestGoldenFlowEqualsTheCommand` | every scenario | unchanged: equal |

- [ ] **Step 1:** Write the tests. Run them. Expected: fail.
- [ ] **Step 2:** Implement.
- [ ] **Step 3:** Run `go test -count=1 ./internal/cli/`, `go test -count=1 -tags=template ./internal/cli/`, and `make lint`.
- [ ] **Step 4:** Commit.

```bash
git add internal/cli
git commit -m "feat(cli): report the five collections"
```

**Done when:** the tests pass.

---

### Task 15: A scenario at the edges of this part (R47)

**Executor:** Main thread writes the file and its README; a subagent adds
the scenario to the lists and the tests.

**Files:**
- Create: `testdata/prometheus-own/edge-scaling.json`
- Modify: `testdata/prometheus-own/README.md`
- Modify: `internal/promtest/scenarios.go`, `scenarios_test.go`, `internal/goldentest/goldentest.go` (`DataFileScenarios` gains `edge-scaling`)
- Rewritten by `make update-testdata`: the golden files of the scenario

**The scenario.** It starts as a copy of `direct-complete.json`: the same
probes, the same namespace and workload answers, with these changes. The
entries stay in the order the program sends the queries, and the
`query_contains` of an entry that a later query would also contain comes
first in the file. Every capability stays available, with the recording
rule for node utilisation, except `workloads_without_hpa`, whose
`kube_deployment_labels` probe answers nothing so that the program probes
`kube_deployment_spec_replicas` and finds it. Timestamps `t1` = 1784050200,
`tm` = 1784071800, `t2` = 1784093400.

Namespace `payments`: the CPU usage series holds three points, `t1` `3.3`,
`tm` `1.1`, `t2` `2.2` (F-76). Everything else as in `direct-complete`.

HPAs, namespace `payments`:

| HPA | Target | Spec min | Spec max | Current | Desired | Exercises |
|---|---|---|---|---|---|---|
| `web` | `Deployment`/`payments-web` | `2` | `4` | `2`, `4` | `2`, `4` | maximum reached; the measured workload covered |
| `fixed` | `Deployment`/`fixed` | `3` | `3` | `3`, `3` | `3`, `3` | pinned |
| `idle` | `Deployment`/`idle` | `1` | `5` | `0`, `0` | `1`, `1` | scaled to zero, hidden; divergence `1` |
| `dup` | `Deployment`/`dup` | `1` | two series, `4` and `6` | `1`, `2` | `1`, `2` | an ambiguous spec |

Coverage: `kube_deployment_spec_replicas` `payments-web` `3`, `api` `2`,
`batch` `0`; `kube_statefulset_labels` `queue`.

Nodes:

| Node | Info | Labels | Capacity cpu/mem | Allocatable | Ready | Unschedulable | Requests cpu/mem | CPU usage | Memory usage | Exercises |
|---|---|---|---|---|---|---|---|---|---|---|
| `pool-a-1` | ip `10.0.0.1`, hostname `pool-a-1` | `label_agentpool` `pool-a` | `4` / `8e9` | `4` / `7e9` | `true` `1` | `0` | `3.6` / `3.5e9` | `20`, `20` | `25`, `25` | request high critical, over requested, low utilisation |
| `pool-a-2` | ip `10.0.0.2` | `label_agentpool` `pool-a` | `4` / `8e9` | `4` / `7e9` | `false` `1` | `1` | `1` / `1e9` | `55`, `60` | `50`, `55` | not ready, not schedulable; the uneven pool |
| `aks-agentpool-12345678-vmss000000` | ip `10.0.0.3` | none | `8` / `16e9` | `8` / `15e9` | `true` `1` | `0` | `2` / `3e9` | `90`, `95`, the series by `instance` `10.0.0.3:9100` with no `node` label | `70`, `72` | pool from the name; resolution by IP; critical usage |
| `orphan` | none | none | `2` / `4e9` | none | none | none | `0.5` / none | none | none | known by other series only; unknown pool |

`node_uname_info` names the three nodes of `kube_node_info` by their
instance and `nodename`.

Cluster autoscaling, in `auto` mode: `node_groups_count` groups `pool-a`
`1`, `1` and `agentpool` `1`, `1`; `nodes_count` `2`, `2` and `1`, `1`;
`min` `1`, `1` and `1`, `1`; `max` `2`, `2` and `1`, `1`; `scaled_up`
group `pool-a` `0`, `1`; `unschedulable_pods` one series with no group
label, `0`, `2`.

**Expected, by the rule of the program.** Task 16 confirms every value
against the original and corrects this table where the original says
otherwise.

| Row | Expected |
|---|---|
| namespace CPU usage | average `2.2`, minimum `1.1`, peak `3.3`, current `2.2` |
| HPAs, sorted | `dup` (spec max unknown), `fixed`, `idle`, `web`; state `partial`; warning `HPA_AMBIGUOUS_SPEC` |
| coverage | `api`/`Deployment`, `queue`/`StatefulSet`; notice `WORKLOADS_OMITTED_SCALE_ZERO` count `1`; `payments-web` covered |
| HPA findings | `web` `HPA_MAX_REPLICAS_REACHED` (4, 4); `fixed` `HPA_PINNED_REPLICAS` (3, 3); `idle` `HPA_REPLICA_DIVERGENCE` (1); `dup` none; `DEPLOYMENT_WITHOUT_HPA` `api`; `STATEFULSET_WITHOUT_HPA_INFO` `queue` |
| nodes, sorted | `aks-...` pool `agentpool` inferred; `orphan` pool `unknown`; `pool-a-1`, `pool-a-2` pool `pool-a` exact |
| node findings | `aks`: `NODE_CPU_USAGE_CRITICAL` (92.5, 95), `NODE_MEMORY_USAGE_ATTENTION` (71, 72); `pool-a-1`: `NODE_CPU_USAGE_HEALTHY` (20, 20), `NODE_MEMORY_USAGE_HEALTHY` (25, 25), `NODE_CPU_REQUEST_HIGH` critical (90), `NODE_OVERREQUESTED` `cpu`, `NODE_LOW_UTILIZATION` (25); `pool-a-2`: the two healthy usage codes (57.5, 60) and (52.5, 55); `orphan`: none; `NODEPOOL_UNKNOWN` (1) |
| pool findings | `pool-a`: `NODEPOOL_CPU_UNEVEN` (20, 60, 40), `NODEPOOL_MEMORY_UNEVEN` (25, 55, 30) |
| node collection | `partial`, warning `NODE_CAPACITY_EVIDENCE_INCOMPLETE` |
| autoscaling | groups `2`, nodes `3`, min `2`, max `3`, scale events `1`, unschedulable `2`; `enabled`, `raw_metrics`, `inferred`; `NODE_AUTOSCALING_NEAR_MAX` (3, 3), node pressure true |
| section 3 | the partial note; the table with `dup`, `fixed`, `web`; the uncovered table with `api` and `queue`, pods unavailable |
| section 4 summary | `agentpool` 1 node, `92,5%` critical, `95%` critical, `71%`, `72%`, `Inferida`; `pool-a` 2 nodes, `38,8%`, `60%`, `38,8%`, `55%`, `Exata`; `Indeterminado` 1 node, four unavailable, `Desconhecida` |
| section 4 detail | `aks` reason `CPU: pico de 95% de utilização` critical; `pool-a-1` reason `CPU: 90% solicitada do alocável` critical, state `Pronto; Agendável`; `pool-a-2` reason `CPU: pico de 60% de utilização; Memória: pico de 55% de utilização` healthy, state `Não pronto; Não agendável`; `orphan` `Sem achado`, `Indisponível; Indisponível` |

**Tests:**

| Test | Input | Expected |
|---|---|---|
| `TestScenariosHoldEight` | the list | eight names; two with the folder `prometheus-own` |
| `TestOriginalScenariosAreUntouched` | the six files | unchanged |
| `TestEdgeScenariosLeaveNoAnswerUnused` | a run of each own scenario | the server has no answer left |

- [ ] **Step 1 (main thread):** Write the scenario file and its README entry: what each series exercises, the SHA-256, the sentence that the file is not edited once its numbers are recorded.
- [ ] **Step 2 (subagent):** Add the scenario and the tests. Run `go test -count=1 ./internal/promtest/ ./internal/goldentest/` and `make lint`.
- [ ] **Step 3 (main thread):** Run `make update-testdata`. Read the new golden files whole against the table above.
- [ ] **Step 4:** Commit.

```bash
git add testdata/prometheus-own internal/promtest internal/goldentest \
  testdata/snapshot/edge-scaling.json testdata/queries/edge-scaling.txt \
  template/tests/capacity/fixtures/command/edge-scaling.pt-BR.json \
  template/tests/capacity/fixtures/command/edge-scaling.en.json
git commit -m "test(golden): add scenario at the edges of scaling"
```

**Done when:** the tests pass and the golden files of the scenario agree
with the table, or the table was corrected with a reason.

---

### Task 16: The numbers of the original, whole

**Executor:** Main thread records; a subagent writes the comparison.

**Files:**
- Modify: `internal/goldentest/original_test.go`
- Create: `testdata/python/edge-scaling.json`, `edge-scaling.queries.txt`; modify `testdata/python/README.md`

**Requirements:**

- The main thread records `edge-scaling` with the script of Task 8 and
  adds the two files and their SHA-256 to the README.
- The comparison reads every key of the recording: `hpas` (specs and the
  two statistics, target kind and name), `workloads_without_hpa` (the
  uncovered rows of the snapshot), `nodes` (every field; the request
  percents against the one number; `ready` and `schedulable`),
  `nodepools`, `autoscaling` (every field), `node_pressure` (recomputed
  from the findings of the snapshot), the five collection states and the
  autoscaling status, and every finding, the evidence compared by kind and
  value, a number as the same `float64`, `approximate` left out (R20). The
  findings are compared as a set keyed by code, scope type, scope name,
  scope kind, then in order with the kind left out (R57).
- `TestComparisonCanFail` gains one case per new section: a number of an
  HPA, of a node, of the autoscaling, and a text of an evidence, each
  changed, each reported by scenario, row, and field.

- [ ] **Step 1 (main thread):** Record `edge-scaling`. Read the two files whole against the table of Task 15.
- [ ] **Step 2 (subagent):** Extend the recording structs and `compareNumbers`. Run `go test -count=1 ./internal/goldentest/ -run 'Original|Numbers|Comparison|Queries' -v`.
- [ ] **Step 3 (main thread):** For each difference, find which tool is right by reading the source of the original and running it by hand. Correct the Go code by a fix brief that names the class, or record a ruling when the difference is deliberate. Correct the tables of Task 15 where the original showed them wrong, and say so in the rulings of the execution.
- [ ] **Step 4:** Commit.

```bash
git add testdata/python internal/goldentest/original_test.go
git commit -m "test(golden): compare every section with the original"
```

**Done when:** `TestNumbersEqualTheOriginal` passes on the eight scenarios,
or each remaining difference is a ruling that the owner has seen.

---

### Task 17: Golden pages and a PDF from each scenario

**Executor:** Main thread.

- [ ] **Step 1:** Run `make image` and `make test-template`. Every data file compiles with no error and no warning.
- [ ] **Step 2:** Write the sixteen PDFs, one for each scenario and language, into `.cache/phase-2/part-3/pdf/`, each run against a fresh fake server (one server serves one run). Count the pages of each document.
- [ ] **Step 3:** Choose the golden pages after the fixtures exist: `command-complete-pt` and `command-complete-en` cover every page; `command-partial-en` covers every page but the first and says why; `command-edge-pt` likewise; a new `command-scaling-pt` covers every page of `edge-scaling` in Portuguese, protecting the partial note, the hidden HPA, the unavailable situation, the placeholder of the undetermined pool, the two-line reason, and the unavailable node state; a new `command-scaling-en` covers the pages of sections 3 and 4 in English. Each entry says the one behaviour it alone protects.
- [ ] **Step 4:** Run `make update-golden`. Open every page that changed or is new and read every header, cell, note, and fallback against the tables of this plan. Write one line per page to `.cache/phase-2/part-3/pages.md`. The parity pages did not change.
- [ ] **Step 5:** Run `make test-golden` twice in a row; the log names every command snapshot.
- [ ] **Step 6:** Commit.

```bash
git add template/golden/snapshots.go template/tests/golden
git commit -m "test(golden): add the pages of sections 3 and 4"
```

**Done when:** every page was read, the golden gate passed twice, and the
parity pages are unchanged.

---

### Task 18: Documents

**Executor:** Subagent for the text; the main thread reads it back.

**Files:**
- Modify: `README.md` (sections 3 and 4 of the report; the six collection lines; the second stop with exit code 5; the eight scenarios), `AGENTS.md` (the layers table already has `internal/arith`; the collectors' order names five; the second insufficiency rule; `testdata/python/` holds every section; the recording script of part 3), `CHANGELOG.md`

**The changelog**, under `Unreleased`, `Added`:

- The report holds the history of the HPAs of each namespace and the workloads without one, with the situation of each against its configured maximum.
- The report holds the usage of the node pools, a profile of allocatable, used, and requested resources per node, and the detail of each node with the reason of its status.
- The snapshot file holds the HPAs, the nodes, the node pools, the coverage of each workload, the cluster autoscaling evidence, and the counts of the report.
- A run that collects prints one line for each of the five collections and one for the cluster autoscaling.
- A run whose collectors return nothing, with the state of the cluster autoscaling unknown, stops with exit code 5.

Under `Fixed`:

- The average of a series equals the original's when the series has more than two samples.

Left out, and said in the report: the guards, the scenario, the recorded
numbers, the golden files and pages.

- [ ] **Step 1:** Write. Run `go test -count=1 ./internal/guard/ -run Changelog`.
- [ ] **Step 2 (main thread):** Read the three files back.
- [ ] **Step 3:** Commit.

```bash
git add README.md AGENTS.md CHANGELOG.md
git commit -m "docs: describe sections 3 and 4"
```

**Done when:** the three files say what the program does.

---

### Task 19: Close the part

**Executor:** Main thread.

- [ ] **Step 1:** Hand the wording of this part to the owner, both languages, as `.cache/phase-2/part-3/wording.md`, the entries of doubt first, with the sixteen PDFs. Apply what the owner changes, run `make update-testdata` and `make update-golden`, and review what changed.
- [ ] **Step 2:** Write the diff of the branch to `.cache/phase-2/part-3/branch.diff`. Run the adversarial review by area, one reviewer for each: the collectors and the resolvers; the analysis and the snapshot; the document and the catalog; the command, the golden files, and the recording. Each writes its findings to a file under `.cache/phase-2/part-3/adversarial/` and repeats them in its last message.
- [ ] **Step 3:** Record every finding in `ROADMAP.md`. Fix the ones assigned to this part, each with a brief that names the class. Run the review again over the fixes.
- [ ] **Step 4:** Run `make gate`. Keep the logs of `.cache/gate/` in `.cache/phase-2/part-3/gate/`.
- [ ] **Step 5:** Update `ROADMAP.md`: every item `DONE` with its commit; F-62 and F-76 `DONE`; the summary line of 2.3; the rulings R36 to R60 and the rulings of the execution, with what the owner said of each. Read `ROADMAP.md` and `CHANGELOG.md` back.
- [ ] **Step 6:** Write the closing report to `.cache/phase-2/part-3/closing-report.md` and ask the owner for the merge and for the decision on the version. The recommendation is to merge with no new version and no tag: the snapshot of schema `1.0` lacks `recommendations` before part 2.4.
- [ ] **Step 7:** After the owner approves, merge by fast-forward.

```bash
git switch main
git merge --ff-only feat/autoscalers-nodes
```

**Done when:** `make gate` passed on the last commit, the review has no open
finding assigned to this part, and the owner approved the merge.

---

## Self-review of this plan

| Requirement of the spec | Task |
|---|---|
| 5.3 exit code 5 for insufficient capabilities, the second rule | 8, 14 (R45) |
| 6.3 collectors read the evidence to choose metric names | 5 (the inventory), 6 (legacy requests, rule against raw utilisation) |
| 6.4 the 17 query shapes, the order, the escaping | 5, 6, 7, 8; `TestQueriesEqualTheOriginal` |
| 6.4 node pool identity in the original's order | 3 |
| 6.4 every output list sorted; first-seen order of messages | 5, 6, 8, 10 |
| 7.1 the 66 codes, exact triggers, healthy findings emitted | 9 |
| 7.2 the registry as the single list; evidence names | 2, 9 |
| 7.3 severities and confidences | 9 |
| 8 `summary`, `hpas`, `nodes`, `nodepools`, coverage; 8.1 statistics; 8.2 codes with parameters; 8.3 integrity rules | 10 (R38, R39, R50, R51, R52) |
| 9.1 headings, headers, fallbacks, fragments, labels, placeholders | 12 |
| 9.2 the period format | 11 (R41) |
| 9.3 the vocabulary, one entry per resource | 12 (R44) |
| 10.2 callout and placeholder in the data file | 13 (R46) |
| 10.3 section order, tables, columns, sort orders, fallbacks of the original | 13 |
| 13.2 snapshot goldens, data file goldens, the comparison with the original | 8, 10, 13, 15, 16 |
| 13.3 golden pages in both languages, each with the behaviour it protects | 17 |
| 15 changelog | 18 |
| 17 the numbers equal the original | 1, 8, 16 |
| Review focus 1 | 3 (`TestIndexDropsASharedValue`), 6 (`TestNodeDropsAnUnresolvedSeries`) |
| Review focus 2 | 9 (`TestHPAMaxReachedAtZero`), 13 (`TestHPASectionSituation`) |
| Review focus 3 | 7 (`TestAutoscalingMergesUngroupedByMax`, `TestAutoscalingIgnoresUngroupedBesideGrouped`) |
| Review focus 4 | 6 (`TestNodeKnownByRequestsOnly`), 15 (`orphan`) |
| Review focus 5 | 1, 15, 16 |

Placeholders: none. Every task names its files, its signatures, its texts,
and its tests with inputs and expected results.

Types across tasks: `collect.Number` and `collect.Flag` (Task 4) are used
by `HPA` (5), `Node` (6), `AutoscalingEvidence` (7); `collect.WorkloadRef`
(5) by the coverage result (8), the analysis (9), the snapshot (10);
`collect.Pool` and `Pools` (6) by the analysis (9) and the snapshot (10);
`analysis.Value` and `EvidenceSpec` (2) by the registry (9) and the
snapshot (10); `snapshot.Value` (2) by the walk (10); `Callout` and
`Placeholder` (13) by nothing else; `Catalog.Period` (11) by the document
(13); the key functions `Confidence`, `Reason`, `FindingSummary` (12) by
the document (13); `collect.CheckUseful` and `ErrNoUsefulSections` (8) by
`goldentest` (8) and the command (14).

Order of the red suite: the catalog check of the new codes fails from Task
5 to Task 12, by the same rule as E46; `TestGoldenSnapshots` between Task
10's code and its regeneration; the golden pages between Task 8 and Task
17. Each is named in its task.
