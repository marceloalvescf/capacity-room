# Phase 2.2: Namespaces and Workloads Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** `k8s-capacity-report report` collects the consumption, the requests, and the limits of each namespace and of each workload over the period, judges them against the thresholds, stores them in the snapshot, and prints sections 1 and 2 of the report and the interpretation notes, in Portuguese (Brazil) and in English, with the same numbers as the Python tool.

**Architecture:** The second thread through every layer of spec section 4.3, now with the three packages that part 2.1 left out: `config -> prom -> discovery -> collect + resolve -> analysis -> snapshot -> catalog + document -> typst -> PDF`. The collectors send the queries of the original, in its order, and reduce the answers to four numbers per series. The analysis turns rows into findings that carry a code and numbers and no sentence. The snapshot stays the only handover: the document reads the rows, the findings, and the thresholds from it, and nothing else.

**Tech Stack:** Go 1.27 and the modules already in `go.mod`. No new module. Typst 0.15.1, `pdftotext`, Docker. Python 3.11 or newer, once, on the main thread, to record the numbers of the original (Task 15).

**Spec:** `docs/superpowers/specs/2026-09-27-k8s-capacity-report-design.md`, sections 6.1, 6.4, 7.1 to 7.3, 8, 9.1 to 9.3, 10.2, 10.3, 13.2, 13.3, 15, 16, and 17.

**Depends on:** Part 2.1, merged into `main` at `2d38ce2`.

## Where this part sits

| Part | Delivers | PDF at the end |
|---|---|---|
| 2.1 Spine, merged | Configuration, redaction, client, discovery, the fake server, the catalog mechanism, the four modes of `report` | Cover, opening page, quality appendix |
| **2.2 Namespaces and workloads (this plan)** | Range queries, two collectors, workload identity, the finding registry with 40 codes, five formats, the first comparison with the Python tool | Sections 1 and 2, interpretation notes |
| 2.3 Autoscalers and nodes | HPA, nodes, node pools, cluster autoscaling, 26 finding codes | Sections 3 and 4 |
| 2.4 Executive profile and proofs | Recommendations, minimum severity of the executive body, the complete catalog, the finding sentences, a real cluster | Both profiles |

### Out of this part

| Left out | Arrives in | Reason |
|---|---|---|
| HPA, node, node pool, and cluster autoscaling collectors | 2.3 | Their sections are 3 and 4 |
| `hpas`, `nodes`, `nodepools` in the snapshot; autoscaler coverage on a workload row | 2.3 | A field enters with the part that fills it (R10) |
| `summary` and `recommendations` in the snapshot | 2.4 | The executive body prints them |
| The sentences of the findings in the catalog | 2.4 | No page of this part prints one (R21) |
| The format of a period, such as `24 horas` | 2.3 | The HPA section prints the first one |
| The second rule of insufficient capabilities, "no useful report sections were collected" | 2.3 | It reads the results of all five collectors (R28) |
| The executive profile | 2.4 | R3 of part 2.1 stands |
| The run against a real cluster | 2.4 | The owner's decision of 2026-09-29 |

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
- **Main thread**: needs Docker, judgment, wording, page review, the network,
  or the owner.

Rules for every subagent brief:

- Tasks run in sequence. Two agents never share the checkout.
- A reviewer receives a diff file under `.cache/phase-2/part-2/` and runs
  tests only on packages nobody is editing.
- Every commit names its paths: `git add <paths>`, never `git add -A`.
- A fix brief names the class of the defect and asks for its other members.
- A brief that says "report every X" also lists, as tests, the cases that
  must not match.
- A test of a fix goes red on the code before the fix, test by test.
- A subagent has no `ReportFindings` tool. A reviewer writes its findings to
  a file under `.cache/phase-2/part-2/` and repeats them in its last message.

Reference material, read-only and never tracked:

- `older/kubernetes-report/kube_capacity_report/`: the Python source, which
  is the reference for every detail of behaviour that this plan does not
  repeat.
- `.cache/phase-2/part-2/research/original-collection.md`,
  `original-analysis-document.md`, and `go-interfaces.md`: the facts
  extracted for this part, with file and line for each.
- `docs/reference/inventory/collection.md`, `analysis.md`, and
  `document-frontend.md`.

Inside `older/`, nobody reads `.env`, `.envrc`, `report.toml`, anything under
`deploy/secrets`, `.venv`, `.git`, `.worktrees`, `build`, or `node_modules`.

## Global Constraints

Every constraint of the plan of part 2.1 stands. The ones below are added or
are the ones this part leans on most.

- Go 1.27. Module `github.com/marceloalvescf/k8s-capacity-report`. No new
  module enters `go.mod`.
- Metrics only. Imports point one way. Every new package is added to
  `guard.Layers`.
- Every query is `POST` with a form-encoded body, on `/query` and
  `/query_range`. There is no `GET`.
- The query shapes of the original are ported exactly, including the order
  of the matchers inside a selector and the two-step escaping of regular
  expressions.
- The collectors send their queries in the order of the original. The fake
  server answers a query with the first recorded answer whose text is part
  of the query, so another order consumes the wrong answers.
- The range step comes from the fixed table: up to 30 minutes `30s`, up to 6
  hours `2m`, up to 24 hours `5m`, up to 7 days `15m`, otherwise `1h`. No
  alignment is added.
- A sample that is `NaN` or infinite is "no data". It is never zero. A
  measured zero is a number.
- A ratio is computed sample by sample, at equal timestamps, and then
  reduced. It is never the ratio of two averages.
- A workload with no observed consumption is left out of the inventory. Its
  consumption still counts in the namespace totals. So does that of a Job.
- Every output list is sorted. Warnings and notices keep first-seen order
  without duplicates.
- The 40 finding codes of this part are the strings of the original,
  unchanged. A finding carries no sentence.
- The snapshot holds codes and numbers. It holds no sentence, no address, no
  host name, no provider identifier, and no token.
- Wording lives only in `internal/catalog`. The Portuguese sentences are
  carried over unchanged from the original.
- Rounding follows the original: one decimal for percentages and
  millicores, two for cores and memory, trailing zeros removed. No thousands
  separator.
- No em-dash inside a sentence. The em-dash alone marks an empty table cell.
- The same input gives the same snapshot and the same `report.json`, byte
  for byte.
- The clock is read twice in a run that collects.
- `report.json` is strict in both directions. A field enters the contract
  with the feature that prints it.
- The files of `testdata/prometheus/` and of `testdata/python/` are recorded
  answers. They are never edited.
- No test skips. A test never writes a golden file.
- No comment speaks of the process.
- Scratch files go under `.cache/`, never under `/tmp` on the host.
- Commits follow `type(scope): description`, imperative, subject under 50
  characters.

## Review Focus

Each line names an input that the spec implies and does not describe, and the
task whose tests pin it.

1. **A namespace that answers nothing.** `cluster.namespaces` holds
   `payments` and `paymets`, a typing mistake. The run goes on. The second
   namespace has a row whose ten statistics are `unavailable` and whose pod
   count is unknown, and the page prints that row, as the original does,
   because a namespace that vanished is a fact the reader needs. Pinned in
   Task 6 and Task 12.
2. **A series that holds no number.** Every sample of a series is `NaN`, or
   the base of a ratio is zero at every timestamp, or the base is present at
   one timestamp and absent at the next. The first gives `unavailable`, the
   second `no_base`, the third a ratio marked approximate. None gives zero,
   and none divides by zero. Pinned in Task 4.
3. **Two workloads with one name.** A `Deployment` and a `StatefulSet` are
   both named `api` in one namespace. They are two rows, each with its own
   findings, and the minimum severity judges each by its own. Pinned in
   Task 8 and Task 12.
4. **A name that means something to a regular expression.** The namespace
   `orders.v2` must not select `ordersXv2`, and the excluded container
   `istio-proxy` must reach the query as the original writes it, hyphen
   included. Pinned in Task 3.
5. **An answer that is too large, or a period that asks too much.** The
   endpoint answers with more bytes than the limit, or refuses the range
   because it holds too many points. Each stops the run with exit code 3 and
   a message that says which of the two happened. Neither is tried again.
   Pinned in Task 3.

## Rulings made on the owner's behalf

Each of these is a decision the spec leaves open. Each is written as
decision, reason, and cost if wrong. The owner may overturn any of them when
reviewing this plan. The numbering continues that of part 2.1.

| ID | Ruling |
|---|---|
| R17 | Ruling: three packages enter the layers table: `internal/resolve` at rank 50, `internal/collect` at rank 55, `internal/analysis` at rank 60 — the collectors use the resolver, and the analysis reads the rows of the collectors — a later need of `resolve` for something of `collect` would ask for another split |
| R18 | Ruling: the snapshot reader accepts two new shapes, a number that may be `null` and an object whose values are all numbers — spec 8.1 writes `null` in a statistic and spec 8.2 writes the parameters of a warning as an object; the reader of part 2.1 knows neither — two more shapes to keep strict |
| R19 | Ruling: an answer larger than 256 MiB is refused with exit code 3 and is not tried again; the limit is fixed, not configurable — a range answer in raw mode is grouped by pod, and 5000 pods with 720 points each come to about 110 MB; F-35 stays open for the measurement on a real cluster in part 2.4 — a very large cluster with a long period is refused until the limit is raised |
| R20 | Ruling: the evidence of a finding is a list of named numbers, in the order of the original; the mark `approximate` of the original is not repeated in it — the confidence `inferred` of the finding and the flag of the row already say it; three evidence values of part 2.3 are not numbers, and part 2.3 decides their form before any version is released — part 2.3 changes the type of the evidence |
| R21 | Ruling: the registry of findings enters now with code, message family, fixed parameters, and evidence names; the sentences of the findings enter the catalog in part 2.4, with the pages that print them — no page of this part prints a finding sentence, and an entry that nothing prints cannot be reviewed on a page — the guard that ties the registry to the catalog arrives in part 2.4 |
| R22 | Ruling: the seven warnings and the two notices of the collectors get codes, listed in Task 6 and Task 7; the original has sentences only — the snapshot holds no sentence — a code chosen now is kept for as long as schema `1.0` is supported |
| R23 | Ruling: the period start keeps the arithmetic that part 2.1 wrote, by the wall clock; when the start so computed is not before the end, the run stops before the first request, with exit code 2 and a message that names the period and the time zone — spec 6.4 keeps the arithmetic, and a window of no length gives a report of nothing. Measured on 2026-09-29 with Go 1.27.1: when the wall clock of the start falls in an hour that the zone skips, Go moves it forward in a zone east of UTC, so that in `Europe/Berlin` a period of `1h` at 03:30 of 2026-03-29 starts where it ends and a period of `1m` at 03:00 starts 59 minutes after its end; in a zone west of UTC Go moves it back, so that in `America/New_York` the same period is one whole hour, where the original has a period of no length (F-61). Closes F-47 — in a zone east of UTC that changes its offset, a run whose period is shorter than the skipped hour fails in that hour, once a year; in a zone west of UTC the run of that hour differs from the original, which collects nothing there |
| R24 | Ruling: a seventh scenario, `edge-values`, is written by this project and lives in `testdata/prometheus-own/`; the six files of the original stay untouched — the file `value-edge-cases.json` of the original holds no value at an edge, and spec 13.2 names that scenario. Closes F-40 — one more scenario to keep |
| R25 | Ruling: the numbers of the original are recorded once, on the main thread, by running the Python source against each scenario; the script and its environment stay under `.cache/`, and only the recorded numbers are tracked, in `testdata/python/` — spec 17 asks for the comparison number by number, and a recorded file lets the gate make it on every run. It needs the network once, to install `requests`, and so waits for the owner's approval — if the owner declines, the comparison rests on the values of the golden files of the original, which cover one namespace |
| R26 | Ruling: a row of the snapshot holds no state of requests and limits of its own; the analysis reads it from the collectors, and the document reads the state of each statistic — spec 8 lists pods, statistics, and the approximate flag, and the capability `requests_limits` already holds the state — the data view of phase 4 cannot show the state per row |
| R27 | Ruling: `--check` sends no query of a collector, as in the original — a check is the preflight — a check cannot say that a collector would fail |
| R28 | Ruling: the second rule of insufficient capabilities of the original waits for part 2.3 — it judges the results of five collectors, and this part has two — a run whose two collectors return nothing goes on and prints two empty sections |
| R29 | Ruling: `schema_version` stays `"1.0"`, and a snapshot written by part 2.1 is refused by this part for the keys it lacks — no version is released, as in R10 — a file kept from a run of part 2.1 must be written again |
| R30 | Ruling: the interpretation notes are printed in this part, after the quality appendix — the collectors of this part produce the first warnings and notices, and a field of the snapshot enters with the page that prints it — none known |
| R31 | Ruling: a notice that counts has two entries in the catalog, one for the count of one, named `singular`, and one for any other count, named `plural`, and Go chooses between them — the entries of the catalog use the one form `{{.Name}}`, and both languages of this project have exactly these two forms — a third language with more plural forms needs another mechanism |
| R32 | Ruling: the units `mCPU`, `cores`, `B`, `KiB`, `MiB`, `GiB`, and `TiB` are the same in both languages and are written by the format functions, not by the catalog files — spec 9.2 shows them equal, and the original writes `1 cores` — a language that translates a unit needs an entry |
| R33 | Ruling: the minimum severity judges a workload by the findings that carry its name and its kind; the original compares the name only — two workloads of one name would otherwise share the worst severity of both — the pages differ from the original when two workloads share a name and differ in severity |
| R34 | Ruling: a run that collects prints one line for each collector after the lines of the capabilities, `namespace collection: <state>` and `workload collection: <state>`, as the original does — scripts may read them (R13) — none known |
| R35 | Ruling: the document classifies a percentage with the thresholds stored in the snapshot; the original keeps a second copy of the two thresholds in its render options — one source, and a PDF from a snapshot then judges as the run that made it — none known |

## Findings this plan closes or records

| ID | What happens in this part |
|---|---|
| F-35 | Narrowed by Task 3: the limit exists (R19); the measurement on a real cluster stays open for part 2.4 |
| F-38 | Closed by Task 1 |
| F-39 | Closed by Task 2 |
| F-40 | Closed by Task 14 (R24) |
| F-47 | Closed by Task 13 (R23) |
| F-61 | Recorded while this plan was written: the wall clock of a skipped hour is moved forward east of UTC and back west of it, so the period of F-47 is empty or inverted in the first case and differs from the original in the second. Task 4 pins both by tests |

---

## File structure

| Path | Responsibility |
|---|---|
| `internal/guard/changelog.go` | Checks every version heading, not the first alone |
| `template/embed.go` | Every refusal of `Unpack` starts with `template: ` |
| `internal/guard/imports.go` | The layers table with the three new packages |
| `internal/prom/range.go` | `QueryRange` and the reader of a matrix answer |
| `internal/prom/selector.go` | Matchers of four kinds; the escaping of a literal in a regular expression |
| `internal/prom/client.go` | The limit on the size of an answer |
| `internal/resolve/workload.go` | From owner records to workload identities; the canonical kind |
| `internal/collect/window.go` | The query window and the step table |
| `internal/collect/statistics.go` | From series to four numbers; ratios sample by sample |
| `internal/collect/collect.go` | The types of the rows and of the results; the codes of warnings and notices; `All` |
| `internal/collect/namespaces.go` | The namespace collector |
| `internal/collect/workloads.go` | The workload collector, both strategies |
| `internal/analysis/registry.go` | The 40 codes: family, fixed parameters, evidence names |
| `internal/analysis/resources.go` | The findings of a namespace and of a workload |
| `internal/analysis/analysis.go` | `All`, and the order of the findings |
| `internal/snapshot/snapshot.go` | The new sections and the statistic |
| `internal/snapshot/build.go` | Fills the new sections |
| `internal/snapshot/codec.go` | The two new shapes of the reader; the integrity rules |
| `internal/catalog/format.go` | Decimal, percent, CPU, memory, count |
| `internal/catalog/keys.go`, `locales/*.toml` | The wording of this part |
| `internal/document/document.go` | The run `ident` |
| `internal/document/sections.go` | Sections 1 and 2 |
| `internal/document/appendix.go` | The interpretation notes |
| `internal/cli/run.go` | The collectors in the flow; the empty period; the two new lines |
| `internal/goldentest/goldentest.go` | The same flow, for the golden files |
| `internal/goldentest/original_test.go` | The comparison with the recorded numbers of the original |
| `internal/promtest/scenarios.go` | The seventh scenario and its folder |
| `testdata/prometheus-own/edge-values.json`, `README.md` | The scenario written by this project |
| `testdata/python/*.json`, `README.md` | The recorded numbers of the original |
| `testdata/snapshot/`, `testdata/queries/`, `template/tests/capacity/fixtures/command/` | Golden files, rewritten |
| `template/golden/snapshots.go`, `template/tests/golden/` | Golden pages |
| `README.md`, `AGENTS.md`, `CHANGELOG.md` | Documents |

---

### Task 0: Open the part

**Executor:** Main thread.

- [ ] **Step 1:** Run `git status`, `git log --oneline -5`, and
  `go test -count=1 ./...`. Read `ROADMAP.md` and `CHANGELOG.md` and confirm
  they describe what the commands show. Correct the record before going on.
- [ ] **Step 2:** Confirm the branch.

```bash
git branch --show-current
```

Expected: `feat/namespaces-workloads`.

- [ ] **Step 3:** In `ROADMAP.md`, add the section
  `## Phase 2.2. Namespaces and workloads` with one row per task of this
  plan, identifiers `P22-0` to `P22-18`, set the summary line of 2.2 to the
  plan and the branch, and record under the decisions of the owner what the
  owner said of each ruling R17 to R35.
- [ ] **Step 4:** Commit the plan.

```bash
git add docs/superpowers/plans/2026-09-29-phase-2-2-namespaces-workloads.md
git commit -m "docs(plan): add the plan of phase 2.2"
```

**Done when:** the branch is `feat/namespaces-workloads`, `ROADMAP.md` names
every task of this plan, and the owner has approved the plan and its rulings.

---

### Task 1: The changelog guard reads every heading (F-38)

**Executor:** Subagent.

**Class of the defect:** a check that returns at the first thing it accepts
and never looks at the rest. `NewestVersionHeading` stops at the first
well-formed version heading, so a malformed older heading is never reported.
The brief asks for the other members of this class inside `internal/guard`,
and the report lists where they were looked for.

**Files:**
- Modify: `internal/guard/changelog.go`
- Modify: `internal/guard/changelog_test.go`

**Interfaces:**
- Consumes: nothing new.
- Produces: the exported names of `changelog.go` keep their signatures.

**Requirements:**

- Every level-two heading of the changelog is checked, in order. The first
  one that is neither `## [Unreleased]` nor `## [X.Y.Z] - YYYY-MM-DD` is
  reported with its line number, wherever it stands.
- Version headings stand in descending order of version. One that is not
  lower than the one above it is reported.
- The newest version heading still equals `VERSION`, as today.

**Tests:**

| Test | Input | Expected |
|---|---|---|
| `TestChangelogReportsMalformedOlderHeading` | a good heading `1.1.0`, then `## 1.0.0 (2026-01-01)` | an error that names the line of the second |
| `TestChangelogReportsHeadingsOutOfOrder` | `## [1.0.0] - 2026-01-01`, then `## [1.1.0] - 2026-02-01` | an error that names the second |
| `TestChangelogReportsRepeatedVersion` | the heading of `1.0.0` twice | an error |
| `TestChangelogAcceptsManyGoodHeadings` | `Unreleased`, `1.1.0`, `1.0.1`, `1.0.0` | no error |
| every existing test of `changelog_test.go` | unchanged | passes unchanged |

- [ ] **Step 1:** Write the tests.
- [ ] **Step 2:** Run `go test ./internal/guard/ -run Changelog -v`.
  Expected: the three `Reports` tests fail, because no error is returned.
- [ ] **Step 3:** Change `changelog.go`.
- [ ] **Step 4:** Run `go test -count=1 ./internal/guard/` and `make lint`.
  Expected: pass.
- [ ] **Step 5:** Commit.

```bash
git add internal/guard/changelog.go internal/guard/changelog_test.go
git commit -m "fix(guard): check every changelog heading"
```

**Done when:** the tests pass, the report lists the other members of the
class that were looked for, and F-38 names the commit.

---

### Task 2: Every refusal of `Unpack` names its package (F-39)

**Executor:** Subagent.

**Class of the defect:** an error of the standard library returned bare,
beside refusals that carry the prefix of the package. The brief asks for the
other members of this class in `template/` and in `internal/typst`.

**Files:**
- Modify: `template/embed.go:41-51`
- Modify: `template/embed_test.go`

**Requirements:**

- The errors of `os.Lstat` and of `os.ReadDir` inside `Unpack` are wrapped,
  start with `template: `, and still answer `errors.Is` for the error of the
  standard library.
- No message of `Unpack` changes otherwise.

**Tests:**

| Test | Input | Expected |
|---|---|---|
| `TestUnpackPrefixesStatFailure` | a target under a folder with no search permission | the message starts with `template: `; `errors.Is(err, fs.ErrPermission)` |
| `TestUnpackPrefixesReadFailure` | a target folder with no read permission | the same two checks |
| `TestUnpackEveryRefusalHasPrefix` | each refusal the existing tests already provoke | each message starts with `template: ` |

- [ ] **Step 1:** Write the tests.
- [ ] **Step 2:** Run `go test ./template/ -run Unpack -v`. Expected: the two
  `Prefixes` tests fail on the prefix.
- [ ] **Step 3:** Wrap the two errors.
- [ ] **Step 4:** Run `go test -count=1 ./template/` and `make lint`.
- [ ] **Step 5:** Commit.

```bash
git add template/embed.go template/embed_test.go
git commit -m "fix(template): prefix every refusal of Unpack"
```

**Done when:** the tests pass, the report lists the other members of the
class that were looked for, and F-39 names the commit.

---

### Task 3: Range queries, selectors, and the size limit

**Executor:** Subagent.

**Files:**
- Create: `internal/prom/range.go`, `internal/prom/range_test.go`
- Modify: `internal/prom/selector.go`, `internal/prom/selector_test.go`
- Modify: `internal/prom/client.go`, `internal/prom/client_test.go`
- Modify: `internal/prom/result.go` (the message of the refused result type)

**Interfaces:**
- Consumes: `prom.Client`, `prom.Error`, `prom.Quote`, as they are.
- Produces:

```go
// Point is one sample of a series of a range answer. Time is the
// timestamp as the endpoint wrote it, in seconds. Valid is false when the
// value was NaN or infinite.
type Point struct {
    Time  float64
    Value float64
    Valid bool
}

// Series is one series of a range answer.
type Series struct {
    Labels map[string]string
    Points []Point
}

func (c *Client) QueryRange(ctx context.Context, query string, start, end time.Time, step string) ([]Series, error)

// MaxResponseBytes is the largest answer the client reads.
const MaxResponseBytes = 256 << 20

type Selector struct {
    Metric   string
    Equal    map[string]string
    Regex    map[string][]string // label =~ one of these literal values
    NotEqual map[string]string
    NotRegex map[string][]string // label !~ one of these literal values
}

// Literals writes values as one regular expression that matches any of
// them and nothing else, ready to stand between the quotes of a matcher.
func Literals(values []string) string
```

**Requirements:**

- `QueryRange` sends `POST` to `<root>/query_range` with the form fields
  `query`, `start`, `end`, and `step`. `start` and `end` are whole seconds
  in decimal digits, as `Query` writes `time`. `step` is sent as given.
- `QueryRange` shares with `Query` the attempts, the waits, the errors, the
  token, and the rule of the first answer. It accepts the result type
  `matrix` only. `Query` still accepts `vector` only.
- A series keeps its points in the order of the answer. A value that is
  `NaN`, `+Inf`, or `-Inf` gives a point with `Valid` false.
- A selector writes its matchers in this order: `Equal`, `Regex`,
  `NotEqual`, `NotRegex`, each group by label name. This is the order of
  `selector()` in `metrics/selectors.py:18-35`.
- `Literals` writes `(?:a|b)`. Each value is escaped for a regular
  expression as `re.escape` of Python does, which puts a backslash before
  each of these characters and no other:
  `( ) [ ] { } ? * + - | ^ $ \ . & ~ #`, the space, the tab, the line feed,
  the carriage return, the vertical tab, and the form feed. The whole is
  then escaped by `Quote`. `regexp.QuoteMeta` does not escape the hyphen and
  must not be used.
- The values keep the order they are given in. The caller decides the order.
- The client reads at most `MaxResponseBytes` of a body. A longer body gives
  `&Error{Kind: KindMetrics}` with the message
  `metrics endpoint answered with more than 256 MiB` and is not tried again.
  The limit holds for `Query` and for `QueryRange`. The tests set a smaller
  limit through an unexported field of the client.
- An answer of status 422 or 400 whose body is a Prometheus error keeps the
  `errorType` and the `error` text of the body in the message, and is not
  tried again.

**Tests:**

| Test | Input | Expected |
|---|---|---|
| `TestQueryRangeSendsTheForm` | start `1784050200`, end `1784136600`, step `5m` | path ends in `/query_range`; method `POST`; the four fields; no query string on the address |
| `TestQueryRangeReadsAMatrix` | two series of two points | two `Series`, labels and points in order |
| `TestQueryRangeMarksNonFinite` | values `"NaN"`, `"+Inf"`, `"-Inf"`, `"0"` | three points not valid; the fourth valid with value 0 |
| `TestQueryRangeRefusesAVector` | a `vector` answer | `KindQuery`, message names `vector` |
| `TestQueryRefusesAMatrix` | a `matrix` answer to `Query` | `KindQuery`, message names `matrix` |
| `TestQueryRangeRefusesBadPoint` | a point of one element; a point whose value is a number and not a text | `KindQuery` for each |
| `TestQueryRangeRetriesOn503` | 503, 503, then a matrix | the matrix; three requests |
| `TestQueryRangeKeepsTheRefusalOfTheEndpoint` | 422 with `errorType` `execution` and `error` `exceeded maximum resolution of 11,000 points per timeseries` | `KindQuery`; the message holds both texts; one request |
| `TestAnswerAboveTheLimitIsRefused` | limit 64 bytes, body of 65 | `KindMetrics`; the message above; one request |
| `TestAnswerAtTheLimitIsRead` | limit 64 bytes, a valid body of 64 | no error |
| `TestSelectorWritesFourGroupsInOrder` | `Equal{cluster: prod}`, `Regex{namespace: [payments, orders]}`, `NotRegex{container: ["", POD, istio-proxy]}` | `m{cluster="prod",namespace=~"(?:payments\|orders)",container!~"(?:\|POD\|istio\\-proxy)"}` |
| `TestLiteralsEscapesTwice` | `payments`, `orders.v2` | `(?:payments\|orders\\.v2)` |
| `TestLiteralsEscapesAQuote` | `a"b` | `(?:a\"b)` |
| `TestLiteralsEscapesEveryListedCharacter` | each character of the list above | a backslash, doubled by `Quote`, before each |
| `TestLiteralsLeavesOtherCharactersAlone` | `a_b/c:d,e=f` | unchanged between `(?:` and `)` |
| every existing test of the package | unchanged | passes unchanged |

The expected texts of the three selector tests equal the ones of
`tests/test_selectors.py:4-32` of the original. Read them there and keep
them letter for letter.

- [ ] **Step 1:** Write the tests.
- [ ] **Step 2:** Run `go test ./internal/prom/ -v`. Expected: the new tests
  fail, because the names are missing or the behaviour is.
- [ ] **Step 3:** Implement.
- [ ] **Step 4:** Run `go test -count=1 ./internal/prom/` and `make lint`.
- [ ] **Step 5:** Commit.

```bash
git add internal/prom/
git commit -m "feat(prom): add range queries and size limit"
```

**Done when:** the tests pass and F-35 names the commit as the one that
added the limit.

---

### Task 4: From series to numbers

**Executor:** Subagent.

**Files:**
- Modify: `internal/guard/imports.go`, `internal/guard/imports_test.go`
- Create: `internal/collect/window.go`, `window_test.go`
- Create: `internal/collect/statistics.go`, `statistics_test.go`

**Interfaces:**
- Consumes: `prom.Series`, `prom.Point`, `config.Analysis`.
- Produces, in `guard.Layers`, between `internal/discovery` and
  `internal/snapshot`:

```go
{Path: "internal/resolve", Rank: 50},
{Path: "internal/collect", Rank: 55},
{Path: "internal/analysis", Rank: 60},
```

- Produces, in `internal/collect`:

```go
// Window is the span of time the collectors ask about.
type Window struct {
    Start time.Time
    End   time.Time
    Step  string
}

// ErrEmptyPeriod is the error of a period whose start is not before its end.
var ErrEmptyPeriod = errors.New("the analysis period is empty")

func NewWindow(analysis config.Analysis, emittedAt time.Time) (Window, error)
func Step(period time.Duration) string

const (
    StateValue       = "value"
    StateUnavailable = "unavailable"
    StateNoBase      = "no_base"
)

// Statistics is a series reduced to four numbers. The numbers mean
// something only when State is StateValue.
type Statistics struct {
    State   string
    Average float64
    Minimum float64
    Peak    float64
    Current float64
}

// Key is the values of the grouping labels of one row, in the order of
// the labels.
type Key [3]string

func Reduce(series []prom.Series, labels []string) map[Key]Statistics
func Ratios(usage, base []prom.Series, labels []string, baseState string) (ratios map[Key]Statistics, approximate map[Key]bool)
```

**Requirements:**

- `NewWindow` computes the start by the wall clock of the emission time in
  the zone of the analysis, less the period, and lets the zone normalise the
  result. This is the code of `periodStart` in `internal/snapshot/build.go`,
  moved here. `End` is the emission time. Both are cut to whole seconds.
  `Step` comes from the table. When `Start` is not before `End`, the error
  wraps `ErrEmptyPeriod` and reads
  `analysis.period: the period <period> is empty at <end> in <zone>, because the offset of the zone changes inside it; choose a longer period`,
  with the end written as RFC 3339.
- `Step`: up to 30 minutes `30s`; up to 6 hours `2m`; up to 24 hours `5m`;
  up to 7 days `15m`; otherwise `1h`. Each limit is inclusive.
- `Reduce` follows `_samples_by_timestamp` and `_statistics` of
  `collectors/common.py:60-105`:
  - a point that is not valid is left out;
  - points of several series with the same key and the same timestamp are
    added;
  - a sum that is not finite is left out;
  - a key with no point left has no entry in the result;
  - the average is the sum of the values divided by their count, the values
    taken in the order of their timestamps;
  - the minimum and the peak are the smallest and the largest value;
  - the current value is the one of the last timestamp.
- A label of `labels` that a series lacks counts as the text `unknown`, as
  in the original. `labels` holds one to three names.
- `Ratios` follows `aggregate_percentages` and `percent` of
  `collectors/common.py:118-174`, line by line:
  - both inputs are first reduced by timestamp as above;
  - for each key of the usage, and each of its timestamps, the base is the
    one of the same timestamp, or, when there is none, the base of the
    latest timestamp the base has;
  - the ratio is usage divided by base, times 100; a base that is absent,
    zero, or negative gives no ratio for that timestamp;
  - a timestamp that used the base of another timestamp and gave a ratio
    marks the key as approximate;
  - the ratios of a key are reduced to a statistic as above;
  - a key with no ratio at all is `no_base` when the base is zero at some
    timestamp of a finite usage, whatever `baseState` is, or when the key
    has usage, the base has no point for the key, and `baseState` is
    `available`; otherwise the key has no entry.
- A key with no entry reads as `Statistics{State: StateUnavailable}` for the
  caller, which looks it up with a helper of this file.
- Nothing here rounds.

**Tests:**

The first five transcribe `tests/test_collector_common.py` of the original.
Read each test there and keep its samples and its expected numbers.

| Test | Input | Expected |
|---|---|---|
| `TestReduceKeepsZeroAndSkipsNonFinite` | the samples of `test_aggregate_statistics_keeps_zero_and_skips_non_finite_or_missing` | its expected statistic; the zero is the minimum |
| `TestRatiosAlignSamples` | the samples of the test that aligns samples | its expected ratios; not the ratio of the sums |
| `TestRatiosFallBackToLatestBase` | the base lacks the second timestamp | the ratio uses the base of the latest timestamp; the key is approximate |
| `TestRatiosZeroBaseIsNoBase` | the samples of `test_zero_denominator_is_reported_as_missing_base_not_as_unavailable` | `no_base` |
| `TestRatiosAbsentBaseFollowsTheState` | usage present, base absent; `baseState` `available`, `partial`, `unavailable` | `no_base`; no entry; no entry |
| `TestReduceAddsSeriesOfOneKey` | two series of one namespace, values 1 and 2 at one timestamp | one point of 3 |
| `TestReduceDropsAKeyOfNaNOnly` | one series, every point not valid | no entry |
| `TestReduceCurrentIsTheLastTimestamp` | points given out of order of time | current is the value of the largest timestamp; the average counts each once |
| `TestReduceUsesThreeLabels` | labels `namespace`, `workload`, `workload_type` | keys of three values |
| `TestRatiosNegativeBaseGivesNoRatio` | base `-1` | no ratio for that timestamp |
| `TestRatiosScenarioNumbers` | usage `0.2`, `0.4`; base `0.5`, `0.5` at the same two timestamps | peak 80; minimum and average as the division gives them; not approximate |
| `TestStepTable` | 30m, 30m1s, 6h, 6h1s, 24h, 24h1s, 7d, 7d1s, 30d | `30s`, `2m`, `2m`, `5m`, `5m`, `15m`, `15m`, `1h`, `1h` |
| `TestNewWindowOfTheGoldenInput` | 2026-07-15 14:30 in `America/Sao_Paulo`, `24h` | start `1784050200`, end `1784136600`, step `5m` |
| `TestNewWindowCountsTheWallClock` | `America/New_York`, 2026-03-08 12:00, `24h` | start is 2026-03-07 12:00 of the zone, 23 hours before the end |
| `TestNewWindowRefusesAnEmptyPeriod` | `Europe/Berlin`, 2026-03-29 03:30, `1h` | the error wraps `ErrEmptyPeriod` and names `1h` and the zone |
| `TestNewWindowRefusesAStartAfterTheEnd` | `Europe/Berlin`, 2026-03-29 03:00, `1m` | the same error |
| `TestNewWindowAcrossTheSkippedHour` | `Europe/Berlin`, 2026-03-29 03:30, `2h`; `America/New_York`, 2026-03-08 03:30, `1h` | start 01:30 of offset `+01:00`, one hour before the end; start 01:30 of offset `-05:00`, one hour before the end |
| `TestLayersHoldThePackagesOfThisPart` | the table | the three new paths with their ranks |

- [ ] **Step 1:** Write the tests.
- [ ] **Step 2:** Run `go test ./internal/collect/ ./internal/guard/ -v`.
  Expected: fail, because the package does not exist.
- [ ] **Step 3:** Implement. `internal/snapshot/build.go` keeps its own
  `periodStart` until Task 9 replaces it.
- [ ] **Step 4:** Run `go test -count=1 ./internal/collect/ ./internal/guard/`
  and `make lint`.
- [ ] **Step 5:** Commit.

```bash
git add internal/guard/imports.go internal/guard/imports_test.go internal/collect/
git commit -m "feat(collect): reduce series to statistics"
```

**Done when:** the tests pass, and the report names, for each of the five
transcribed tests, the test of the original it came from.

---

### Task 5: Workload identity

**Executor:** Subagent.

**Files:**
- Create: `internal/resolve/workload.go`, `workload_test.go`

**Interfaces:**
- Consumes: nothing of this module.
- Produces:

```go
// Owner is one ownership record: child is owned by the object of
// OwnerKind and OwnerName. OwnerKind and OwnerName are empty when the
// child has no controller.
type Owner struct {
    Namespace string
    Child     string
    OwnerKind string
    OwnerName string
}

// Identity names a workload.
type Identity struct {
    Namespace string
    Kind      string
    Name      string
}

// Pod names a pod.
type Pod struct {
    Namespace string
    Name      string
}

const (
    KindJob           = "Job"
    KindPod           = "Pod"
    KindStandalonePod = "StandalonePod"
)

func Workloads(pods, replicaSets []Owner) map[Pod]Identity
func CanonicalKind(kind string) string
```

**Requirements:**

- `Workloads` follows `resolve_workloads` of `resolvers/workload.py:16-33`:
  - a pod with no owner kind or no owner name becomes a workload of kind
    `StandalonePod` with the name of the pod;
  - a pod whose owner kind is exactly `ReplicaSet` takes the owner of that
    replica set, found by namespace and name, when that owner has a kind
    and a name; otherwise it keeps the replica set;
  - any other owner is kept as it is, one step and no more;
  - when two records name the same pod, the last one wins, as in the
    original.
- `CanonicalKind` follows `canonical_workload_kind` of
  `collectors/common.py:22-45`: the nine known kinds, compared without
  regard to letter case, are written `Deployment`, `StatefulSet`,
  `DaemonSet`, `ReplicaSet`, `ReplicationController`, `Job`, `CronJob`,
  `Pod`, `StandalonePod`; any other text, the empty one included, is
  returned as given.

**Tests:**

The first four transcribe `tests/test_resolvers.py:1-60` of the original.

| Test | Input | Expected |
|---|---|---|
| `TestReplicaSetGivesItsDeployment` | pod owned by `ReplicaSet` `web-abc`, owned by `Deployment` `web` | `Deployment` `web` |
| `TestDirectOwnersAreKept` | owners `StatefulSet` `db` and `CustomOwner` `x` | kept as given |
| `TestJobIsNotRolledUp` | pod owned by `Job` `api`; a replica set `api` owned by `Deployment` `api` | `Job` `api` |
| `TestPodWithoutOwnerStandsAlone` | empty kind and name | `StandalonePod` with the name of the pod |
| `TestReplicaSetWithoutOwnerIsKept` | no record of the replica set; a record with empty owner | `ReplicaSet` with its name, both times |
| `TestReplicaSetIsFoundInItsNamespace` | replica sets of one name in two namespaces | each pod takes the owner of its own namespace |
| `TestCanonicalKindTable` | `deployment`, `STATEFULSET`, `job`, `Rollout`, `rollout`, the empty text | `Deployment`, `StatefulSet`, `Job`, `Rollout`, `rollout`, the empty text |

- [ ] **Step 1:** Write the tests.
- [ ] **Step 2:** Run `go test ./internal/resolve/ -v`. Expected: fail,
  because the package does not exist.
- [ ] **Step 3:** Implement.
- [ ] **Step 4:** Run `go test -count=1 ./internal/resolve/` and `make lint`.
- [ ] **Step 5:** Commit.

```bash
git add internal/resolve/
git commit -m "feat(resolve): resolve workload identities"
```

**Done when:** the tests pass.

---

### Task 6: The namespace collector

**Executor:** Subagent.

**Files:**
- Create: `internal/collect/collect.go`, `collect_test.go`
- Create: `internal/collect/namespaces.go`, `namespaces_test.go`

**Interfaces:**
- Consumes: `prom.Selector`, `prom.Literals`, `prom.Series`, `prom.Sample`,
  `discovery.Scope`, `discovery.Capability`, `config.Cluster`,
  `config.Analysis`, `Window`, `Reduce`, `Ratios`.
- Produces:

```go
// Querier is what a collector needs of the client.
type Querier interface {
    Query(ctx context.Context, query string, at time.Time) ([]prom.Sample, error)
    QueryRange(ctx context.Context, query string, start, end time.Time, step string) ([]prom.Series, error)
}

// Input is what every collector reads.
type Input struct {
    Scope        discovery.Scope
    Cluster      config.Cluster
    Analysis     config.Analysis
    Capabilities []discovery.Capability
    Window       Window
}

// Resources holds the ten statistics of a row.
type Resources struct {
    CPUUsage             Statistics
    MemoryUsage          Statistics
    CPURequests          Statistics
    MemoryRequests       Statistics
    CPULimits            Statistics
    MemoryLimits         Statistics
    CPURequestPercent    Statistics
    MemoryRequestPercent Statistics
    CPULimitPercent      Statistics
    MemoryLimitPercent   Statistics
}

// Namespace is the row of one namespace.
type Namespace struct {
    Name                string
    Pods                int
    PodsKnown           bool
    Approximate         bool
    RequestsLimitsState string // a state of discovery
    Resources           Resources
}

// Message is a warning or a notice: a code and its named numbers.
type Message struct {
    Code   string
    Params map[string]float64 // never nil
}

// Result is what a collector returns.
type Result[T any] struct {
    State    string // a state of discovery
    Strategy string // a strategy of discovery
    Rows     []T       // sorted, never nil
    Warnings []Message // never nil
    Notices  []Message // never nil
}

const (
    WarningNamespaceUsageIncomplete            = "NAMESPACE_USAGE_INCOMPLETE"
    WarningNamespaceRequestsLimitsIncomplete   = "NAMESPACE_REQUESTS_LIMITS_INCOMPLETE"
    WarningNamespaceRequestsLimitsUnavailable  = "NAMESPACE_REQUESTS_LIMITS_UNAVAILABLE"
    WarningWorkloadUsageIncomplete             = "WORKLOAD_USAGE_INCOMPLETE"
    WarningWorkloadRequestsLimitsIncomplete    = "WORKLOAD_REQUESTS_LIMITS_INCOMPLETE"
    WarningWorkloadRequestsLimitsUnavailable   = "WORKLOAD_REQUESTS_LIMITS_UNAVAILABLE"
    WarningWorkloadHistoryCurrentAssociation   = "WORKLOAD_HISTORY_CURRENT_ASSOCIATION"
    NoticeWorkloadsOmittedNoUsage              = "WORKLOADS_OMITTED_NO_USAGE"
    NoticeJobsOmitted                          = "JOBS_OMITTED"
)

// WarningCodes and NoticeCodes list every code, in the order above.
var WarningCodes []string
var NoticeCodes []string

// NoticeParams names the parameters of each notice: count, for both.
var NoticeParams map[string][]string

func Namespaces(ctx context.Context, q Querier, in Input) (Result[Namespace], error)
```

**Requirements:**

The collector follows `NamespaceCollector.collect` of
`collectors/namespaces.py:78-246`. Read it before writing.

- The queries, in this order:
  1. range: `sum by (namespace) (rate(<cpu>[<step>]))`
  2. range: `sum by (namespace) (max by (namespace, pod, container) (<memory>))`
  3. to 6. range, only when the state of `requests_limits` is not
     `unavailable`: `sum by (namespace) (<requests, cpu>)`, then requests of
     memory, limits of CPU, limits of memory
  7. instant, at the end of the window:
     `sum by (namespace) (<kube_pod_status_phase, phase="Running"> == 1)`
- Every selector carries the matcher of the cluster scope when the scope
  enables it, and the matcher `namespace=~` with the namespaces of the
  configuration in the order they were given. Every selector of a metric of
  containers, which is the first six, carries `container!~` with the empty
  text, `POD`, and then the excluded containers of the configuration, in the
  order they were given. The seventh carries no matcher of containers.
- The memory query has the wrapper `max by`. The CPU query has not.
- When the evidence of `requests_limits` holds a name that ends in
  `_cpu_cores` or `_memory_bytes`, the four queries use the legacy names,
  `kube_pod_container_resource_requests_cpu_cores` and so on, and carry no
  matcher `resource`.
- The ratios are usage over requests and usage over limits, by `Ratios`,
  with the state of `requests_limits` as the state of the base.
- There is one row for each namespace of the configuration, sorted, each
  name once, whether or not any answer names it.
- `Pods` is the value of the instant answer, cut to a whole number.
  `PodsKnown` is false when the answer holds no valid value for the
  namespace.
- `Approximate` is true when any of the four ratios of the row is.
- `RequestsLimitsState` is the state of the capability, the same on every
  row.
- The state of the result: `unavailable` when neither usage answer holds a
  series; `available` when both do and `requests_limits` is `available`;
  `partial` otherwise. The strategy is `raw_metrics`.
- Warnings, in this order: `NAMESPACE_USAGE_INCOMPLETE` when fewer than two
  usage answers hold a series; `NAMESPACE_REQUESTS_LIMITS_INCOMPLETE` when
  `requests_limits` is `partial`; `NAMESPACE_REQUESTS_LIMITS_UNAVAILABLE`
  when it is `unavailable`. No notice.
- An error of any query stops the collector and is returned as it is.
- The function reads no clock and no environment.

**Tests:**

The tests use a querier written in the test file, which records each query
and answers from a table. The expected query texts of the first test equal
the ones of `tests/test_namespace_collector.py:78-111` of the original.

| Test | Input | Expected |
|---|---|---|
| `TestNamespaceQueriesInOrder` | cluster `prod`, namespaces `payments`, `orders`, excluded `istio-proxy`, step `30s`, everything available | seven queries, in order, each equal to the text of the original, letter for letter |
| `TestNamespaceQueriesWithoutScopeMatcher` | the scope disables the matcher | no query holds `cluster=` |
| `TestNamespaceSkipsRequestsAndLimits` | `requests_limits` unavailable | three queries; none holds `resource_requests` or `resource_limits`; the warning `NAMESPACE_REQUESTS_LIMITS_UNAVAILABLE`; the state `partial` |
| `TestNamespaceUsesLegacyNames` | the evidence holds the four legacy names | the four queries name them and hold no `resource=` |
| `TestNamespaceRowForEachConfigured` | namespaces `payments`, `paymets`; answers name `payments` only | two rows, sorted; the second has ten statistics `unavailable`, `PodsKnown` false |
| `TestNamespaceRowsAreSortedAndUnique` | namespaces `b`, `a`, `b` | rows `a`, `b` |
| `TestNamespaceNumbersOfTheScenario` | the answers of `direct-complete` for the namespace | CPU usage peak 0.4, current 0.4, minimum 0.2; memory usage average 110; requests 0.5; limits 1; CPU request percent peak 80; pods 3; not approximate |
| `TestNamespaceNoBaseWhenAvailable` | usage present, the four answers of requests and limits empty, `requests_limits` available | the four ratios `no_base`; the four statistics of requests and limits `unavailable` |
| `TestNamespaceUnavailableWhenPartial` | the same answers, `requests_limits` partial | the four ratios `unavailable`; the warning `NAMESPACE_REQUESTS_LIMITS_INCOMPLETE` |
| `TestNamespaceApproximate` | the base lacks one timestamp of the usage | `Approximate` true on that row only |
| `TestNamespaceUsageIncomplete` | the memory answer is empty | the warning `NAMESPACE_USAGE_INCOMPLETE`; the state `partial` |
| `TestNamespaceNothingAnswered` | both usage answers empty | the state `unavailable`; the rows still exist |
| `TestNamespaceStopsAtTheFirstError` | the third query fails | that error; three queries sent |
| `TestRegistriesOfCodes` | `WarningCodes`, `NoticeCodes`, `NoticeParams` | seven codes, two codes, `count` for each notice |

- [ ] **Step 1:** Write the tests.
- [ ] **Step 2:** Run `go test ./internal/collect/ -run 'Namespace|Registries' -v`.
  Expected: fail, because `Namespaces` does not exist.
- [ ] **Step 3:** Implement.
- [ ] **Step 4:** Run `go test -count=1 ./internal/collect/` and `make lint`.
- [ ] **Step 5:** Commit.

```bash
git add internal/collect/
git commit -m "feat(collect): collect namespaces"
```

**Done when:** the tests pass, and the report shows the seven query texts
beside the ones of the original.

---

### Task 7: The workload collector

**Executor:** Subagent.

**Files:**
- Create: `internal/collect/workloads.go`, `workloads_test.go`
- Modify: `internal/collect/collect.go`, `collect_test.go`

**Interfaces:**
- Consumes: everything of Task 6, `resolve.Workloads`,
  `resolve.CanonicalKind`, `resolve.Owner`.
- Produces:

```go
// Workload is the row of one workload.
type Workload struct {
    Namespace           string
    Kind                string
    Name                string
    Pods                int
    PodsKnown           bool
    Approximate         bool
    RequestsLimitsState string
    Resources           Resources
}

func Workloads(ctx context.Context, q Querier, in Input) (Result[Workload], error)

// Collected is the result of every collector of a run.
type Collected struct {
    Namespaces Result[Namespace]
    Workloads  Result[Workload]
}

// All runs the collectors in the order of the original: namespaces, then
// workloads.
func All(ctx context.Context, q Querier, in Input) (Collected, error)

// Warnings and Notices join the messages of the collectors, in the order
// of the collectors, each message once.
func (c Collected) Warnings() []Message
func (c Collected) Notices() []Message
```

**Requirements:**

The collector follows `WorkloadCollector` of
`collectors/workloads.py:34-505`. Read it before writing.

- When `workload_usage` is `unavailable`, or its strategy is `none`, the
  collector sends no query and returns the state `unavailable`, the strategy
  of the capability, and no row, no warning, and no notice.
- **Strategy `recording_rule`.** Seven queries, in this order: six range
  queries, CPU usage, memory usage, and, only when `requests_limits` is not
  `unavailable`, requests of CPU, requests of memory, limits of CPU, limits
  of memory; then one instant query at the end of the window.
  - Each range query is
    `sum by (namespace, workload, workload_type) (<expression> * on (<join>) group_left(workload, workload_type) <ownership>)`.
  - `<ownership>` is `namespace_workload_pod:kube_pod_owner:relabel` with
    the matcher of the scope and the matcher of the namespaces, and no
    matcher of containers.
  - `<join>` is `cluster, namespace, pod` when the scope enables its
    matcher, with the label of the scope in place of `cluster`, and
    `namespace, pod` otherwise.
  - `<expression>` is `rate(<cpu>[<step>])` for CPU usage,
    `max by (<join>, container) (<memory>)` for memory usage, and the bare
    selector for the other four.
  - The instant query is
    `count by (namespace, workload, workload_type) (<ownership>)`. Its
    values are the pod counts.
  - The state of the result follows the rule of the namespace collector.
    The strategy is `recording_rule`.
- **Strategy `raw_metrics`.** Eight queries, in this order: two instant
  queries at the end of the window,
  `kube_pod_owner{...,owner_is_controller="true",...}` and
  `kube_replicaset_owner{...,owner_is_controller="true",...}`; then the six
  range queries, each `sum by (namespace, pod) (<expression>)`, with
  `max by (namespace, pod, container)` around the memory selector.
  - The owner records are read from the labels `namespace`, `pod` or
    `replicaset`, `owner_kind`, and `owner_name`, and given to
    `resolve.Workloads`.
  - Each series of a range answer takes the identity of its pod. A pod that
    the owner records do not name becomes a workload of kind `Pod` with the
    name of the pod. This kind differs from `StandalonePod` on purpose.
  - The pod count of a workload is the number of pods that the owner
    records gave to it. No query asks for it.
  - The state of the result is always `partial`. The strategy is
    `raw_metrics`. The first warning is always
    `WORKLOAD_HISTORY_CURRENT_ASSOCIATION`.
- **Both strategies.**
  - Every kind passes through `resolve.CanonicalKind`, before any
    comparison and before the rows are keyed. A kind that is absent or
    empty is `Unknown` first, as in the original.
  - The keys of the rows are the keys of the pod counts and of every
    statistic. A workload of kind `Job` is taken out and counted.
  - When `workload_usage` is `available`, a workload with no statistic of
    CPU usage and none of memory usage is taken out and counted. When it is
    `partial`, the workload stays.
  - Rows are sorted by namespace, then name, then kind.
  - Warnings, after the one of the raw strategy:
    `WORKLOAD_USAGE_INCOMPLETE` when fewer than two usage answers hold a
    series, in the strategy of the recording rule only, as in the original;
    then `WORKLOAD_REQUESTS_LIMITS_INCOMPLETE` or
    `WORKLOAD_REQUESTS_LIMITS_UNAVAILABLE` by the state of
    `requests_limits`.
  - Notices, in this order: `WORKLOADS_OMITTED_NO_USAGE` with `count`, when
    the count is above zero; `JOBS_OMITTED` with `count`, when above zero.
- `All` stops at the first error and returns it.

**Tests:**

Each test below names the test of `tests/test_workload_collector.py` of the
original that it transcribes. Keep its samples and its expected values.

| Test | Transcribes | Expected |
|---|---|---|
| `TestWorkloadRecordingQueriesInOrder` | the queries of the recording strategy | seven texts, in order; the CPU text equals the one in section 1.3.a of `original-collection.md`, letter for letter |
| `TestWorkloadRecordingJoinWithoutScope` | — | `on (namespace, pod)` and no `cluster=` |
| `TestWorkloadRecordingJoinUsesTheScopeLabel` | — | scope label `k8s_cluster`: `on (k8s_cluster, namespace, pod)` |
| `TestWorkloadRawQueriesInOrder` | `test_raw_owner_fallback_uses_only_controllers_and_is_partial` | eight texts; both owner queries hold `owner_is_controller="true"`; state `partial` |
| `TestWorkloadRawKeepsUnownedPod` | `test_raw_metrics_preserve_unowned_pods_as_per_pod_data` | a row `payments`, `Pod`, `orphan-123` |
| `TestWorkloadRawCountsPodsFromOwners` | — | three pods of one deployment give `Pods` 3; no query `count by` |
| `TestWorkloadRawAlwaysWarnsOfHistory` | — | the first warning is `WORKLOAD_HISTORY_CURRENT_ASSOCIATION` |
| `TestWorkloadUnavailableSendsNothing` | `assert reader.queries == []` | no query; state `unavailable`; no row |
| `TestWorkloadSkipsRequestsAndLimits` | — | three queries in the recording strategy, four in the raw one |
| `TestWorkloadOmitsWithoutConsumption` | the sparse usage readers | one row; the notice with `count` 1 |
| `TestWorkloadPartialKeepsWithoutConsumption` | `test_partial_capability_keeps_workloads_without_observed_consumption` | both rows; no notice |
| `TestWorkloadOmitsJobs` | the three Job readers | no row of kind `Job`; the notice `JOBS_OMITTED` with the count of each reader |
| `TestWorkloadOmitsAJobSeenOnlyInTheCount` | the reader of the unobserved Job | the notice once, with `count` 1 |
| `TestWorkloadKindIsCanonical` | the two readers of lower-case kinds | `statefulset` gives a row of kind `StatefulSet` that finds its pod count; `job` is omitted |
| `TestWorkloadNoBase` | `test_available_request_family_without_workload_series_renders_not_defined`, at the collector | the request ratios of that workload are `no_base` |
| `TestWorkloadNumbersOfRawFallback` | the answers of the scenario `raw-fallback` | one row `payments`, `Deployment`, `payments-web`; pods 1; CPU request percent peak as `0.4 / 0.6 * 100` gives it |
| `TestWorkloadRowsAreSorted` | workloads in two namespaces, two of one name and two kinds | namespace, then name, then kind |
| `TestAllRunsNamespacesFirst` | everything available | fourteen queries; the first seven are the ones of the namespace collector |
| `TestAllJoinsMessagesOnce` | both collectors warn of their own state | warnings in the order of the collectors; none twice |

- [ ] **Step 1:** Write the tests.
- [ ] **Step 2:** Run `go test ./internal/collect/ -run 'Workload|All' -v`.
  Expected: fail, because `Workloads` does not exist.
- [ ] **Step 3:** Implement.
- [ ] **Step 4:** Run `go test -count=1 ./internal/collect/ ./internal/guard/`
  and `make lint`.
- [ ] **Step 5:** Commit.

```bash
git add internal/collect/
git commit -m "feat(collect): collect workloads"
```

**Done when:** the tests pass, and the report shows the query texts of both
strategies beside the ones of the original.

---

### Task 8: The finding registry and the resource findings

**Executor:** Subagent.

**Files:**
- Create: `internal/analysis/registry.go`, `registry_test.go`
- Create: `internal/analysis/resources.go`, `resources_test.go`
- Create: `internal/analysis/analysis.go`, `analysis_test.go`

**Interfaces:**
- Consumes: `collect.Namespace`, `collect.Workload`, `collect.Statistics`,
  `collect.Collected`, `config.Thresholds`, the states and confidences of
  `discovery`.
- Produces:

```go
const (
    SeverityHealthy   = "healthy"
    SeverityAttention = "attention"
    SeverityCritical  = "critical"
)

// Severities lists the severities, from the lowest.
var Severities = []string{SeverityHealthy, SeverityAttention, SeverityCritical}

const (
    ScopeNamespace = "namespace"
    ScopeWorkload  = "workload"
)

// Evidence is one named number of a finding.
type Evidence struct {
    Name  string
    Value float64
}

// Finding is one judgment of the analysis. It carries no sentence.
type Finding struct {
    Code       string
    Severity   string
    ScopeType  string
    ScopeKind  string // the kind of a workload; empty for a namespace
    ScopeName  string // the namespace, or namespace/name for a workload
    Evidence   []Evidence // never nil
    Confidence string
}

// Definition is what the registry knows of one code.
type Definition struct {
    Code      string
    Family    string
    Severity  string
    ScopeType string
    Params    map[string]string // fixed parameters, never nil
    Evidence  []string          // the names the finding carries, in order, never nil
}

// Registry lists every code this version can write.
var Registry []Definition

func Lookup(code string) (Definition, bool)
func Classify(percent float64, t config.Thresholds) string
func Rank(severity string) int
func Namespaces(rows []collect.Namespace, t config.Thresholds) []Finding
func Workloads(rows []collect.Workload, t config.Thresholds) []Finding
func All(c collect.Collected, t config.Thresholds) []Finding
func Sort(findings []Finding)
```

**Requirements:**

The analysis follows `analysis/capacity.py:20-281` of the original. Read it
before writing.

- The registry holds 40 codes: the prefix `NAMESPACE` or `WORKLOAD`, the
  resource `CPU` or `MEMORY`, and one of ten endings.

| Ending | Family | Severity | Fixed parameters | Evidence |
|---|---|---|---|---|
| `REQUEST_HEALTHY` | `resource_request_usage` | healthy | `resource` | `average_percent`, `peak_percent` |
| `REQUEST_ATTENTION` | `resource_request_usage` | attention | `resource` | the same |
| `REQUEST_CRITICAL` | `resource_request_usage` | critical | `resource` | the same |
| `LIMIT_HEALTHY` | `resource_limit_usage` | healthy | `resource`, `risk` | the same |
| `LIMIT_ATTENTION` | `resource_limit_usage` | attention | `resource`, `risk` | the same |
| `LIMIT_CRITICAL` | `resource_limit_usage` | critical | `resource`, `risk` | the same |
| `REQUEST_MISSING` | `resource_request_missing` | attention | `resource` | none |
| `LIMIT_MISSING` | `resource_limit_missing` | attention | `resource` | none |
| `OVERREQUESTED` | `resource_overrequested` | attention | `resource` | `average_percent`, `peak_percent` |
| `BURSTY` | `resource_bursty` | attention | `resource` | `average`, `peak`, `gap_percent` |

- `resource` is `cpu` or `memory`. `risk` is `cpu_throttling` for CPU and
  `oomkill` for memory. The order of the registry is: prefix `NAMESPACE`
  then `WORKLOAD`; resource `CPU` then `MEMORY`; ending in the order of the
  table.
- `Classify`: at or above the critical threshold, critical; at or above the
  warning threshold, attention; below, healthy. Both limits are inclusive.
- For each row and each resource, in this order:
  1. the request ratio, when its state is `value`: the higher of the
     classes of its average and of its peak gives the severity and the
     ending; evidence `average_percent`, `peak_percent`;
  2. the limit ratio, the same way;
  3. `REQUEST_MISSING`, when the state of requests and limits of the row is
     `available` and the statistic of the requests is not `value`;
  4. `LIMIT_MISSING`, the same way for the limits;
  5. `OVERREQUESTED`, when the request ratio is `value` and its average is
     at or below the threshold of low utilization; evidence as in 1;
  6. `BURSTY`, when the usage is `value`, its average is above zero, and
     `(peak - average) / average * 100` is at or above the threshold of the
     bursty gap; evidence `average`, `peak`, `gap_percent`.
- Confidence: `inferred` when the row is approximate, for the findings of
  1, 2, and 5; `exact` otherwise, and always for 3, 4, and 6.
- The scope of a namespace is its name, with an empty kind. The scope of a
  workload is `namespace/name`, with its kind.
- A finding of severity healthy is written, as in the original.
- `Sort` orders by severity from the highest, then scope name, then scope
  kind, then code. The original has no kind in its order; the kind is added
  so that two workloads of one name keep a fixed order.
- `All` gives the findings of the namespaces and of the workloads, sorted
  together.
- Every finding that `Namespaces` and `Workloads` write has its code in the
  registry, the severity and the scope type of its definition, and exactly
  the evidence names of its definition, in order.

**Tests:**

The tests marked with a name of the original transcribe
`tests/test_capacity_analysis.py`.

| Test | Transcribes or input | Expected |
|---|---|---|
| `TestClassifyBoundaries` | `test_threshold_boundaries_are_inclusive_at_attention_and_critical` | 69.999 healthy; 70 attention; 84.999 attention; 85 critical |
| `TestRegistryHoldsFortyCodes` | the registry | 40 codes, none twice, in the stated order; every family, severity, and evidence list as in the table |
| `TestRegistryRiskFollowsTheResource` | the twelve `LIMIT_` codes of usage | `cpu_throttling` for CPU, `oomkill` for memory |
| `TestPeakDecidesWhenHigher` | request ratio average 60, peak 80 | `..._REQUEST_ATTENTION`, evidence 60 and 80 |
| `TestAverageDecidesWhenHigher` | average 90, peak 86 | `..._REQUEST_CRITICAL` |
| `TestHealthyFindingIsWritten` | average 10, peak 20 | `..._REQUEST_HEALTHY` and, with it, `..._OVERREQUESTED` |
| `TestMissingNeedsAnAvailableState` | requests `unavailable`; row state `available`, `partial`, `unavailable` | `REQUEST_MISSING`; nothing; nothing |
| `TestMissingIsNotZero` | the test of the original that tells missing from zero | its expected findings |
| `TestOverrequestedBoundary` | average 30, 30.001 | written; not written |
| `TestBurstyBoundary` | average 1, peak 2; average 1, peak 1.999; average 0, peak 5 | written with gap 100; not written; not written |
| `TestApproximateGivesInferred` | an approximate row with all six kinds of finding | `inferred` on kinds 1, 2, and 5; `exact` on 3, 4, and 6 |
| `TestNoBaseGivesNoRatioFinding` | request ratio `no_base` | no finding of kind 1 or 5 |
| `TestWorkloadScope` | workload `payments`, `Deployment`, `api` | scope type `workload`, name `payments/api`, kind `Deployment` |
| `TestTwoWorkloadsOfOneName` | `Deployment` `api` critical, `StatefulSet` `api` healthy | each finding carries the kind of its own row |
| `TestSortOrder` | findings of three severities, two scopes, two kinds | severity, scope name, scope kind, code |
| `TestFindingsOfDirectComplete` | the rows of the scenario, thresholds by default | eight findings: `NAMESPACE_MEMORY_LIMIT_CRITICAL`, `NAMESPACE_MEMORY_REQUEST_CRITICAL`, `WORKLOAD_MEMORY_LIMIT_CRITICAL`, `WORKLOAD_MEMORY_REQUEST_CRITICAL`, `NAMESPACE_CPU_REQUEST_ATTENTION`, `WORKLOAD_CPU_REQUEST_ATTENTION`, `NAMESPACE_CPU_LIMIT_HEALTHY`, `WORKLOAD_CPU_LIMIT_HEALTHY`, in this order |
| `TestEveryFindingFollowsItsDefinition` | rows built to give each of the 40 codes | each finding agrees with the registry in severity, scope type, and evidence names |

The expected findings of `TestFindingsOfDirectComplete` were worked out from
the rule of the program and from the answers of the scenario, not by running
the original. Task 15 confirms them against the recorded numbers.

- [ ] **Step 1:** Write the tests.
- [ ] **Step 2:** Run `go test ./internal/analysis/ -v`. Expected: fail,
  because the package does not exist.
- [ ] **Step 3:** Implement.
- [ ] **Step 4:** Run `go test -count=1 ./internal/analysis/ ./internal/guard/`
  and `make lint`.
- [ ] **Step 5:** Commit.

```bash
git add internal/analysis/
git commit -m "feat(analysis): judge namespaces and workloads"
```

**Done when:** the tests pass.

---

### Task 9: The snapshot

**Executor:** Subagent.

**Files:**
- Modify: `internal/snapshot/snapshot.go`
- Modify: `internal/snapshot/build.go`, `build_test.go`
- Modify: `internal/snapshot/codec.go`, `codec_test.go`

**Interfaces:**
- Consumes: `collect.Collected`, `collect.Window`, `collect.WarningCodes`,
  `collect.NoticeCodes`, `collect.NoticeParams`, `analysis.Finding`,
  `analysis.Lookup`, `analysis.Severities`.
- Produces:

```go
type Snapshot struct {
    SchemaVersion string      `json:"schema_version"`
    Report        Report      `json:"report"`
    Scope         Scope       `json:"scope"`
    Quality       Quality     `json:"quality"`
    Namespaces    []Namespace `json:"namespaces"` // sorted, never nil
    Workloads     []Workload  `json:"workloads"`  // sorted, never nil
    Findings      []Finding   `json:"findings"`   // sorted, never nil
}

type Quality struct {
    Capabilities          []Capability `json:"capabilities"`
    Warnings              []Message    `json:"warnings"` // never nil
    Notices               []Message    `json:"notices"`  // never nil
    CollectionStartedAt   string       `json:"collection_started_at"`
    CollectionCompletedAt string       `json:"collection_completed_at"`
}

type Message struct {
    Code   string             `json:"code"`
    Params map[string]float64 `json:"params"` // never nil
}

// Statistics is spec 8.1. A number is nil when the state is not value.
type Statistics struct {
    State   string   `json:"state"`
    Average *float64 `json:"average"`
    Minimum *float64 `json:"minimum"`
    Peak    *float64 `json:"peak"`
    Current *float64 `json:"current"`
}

type Namespace struct {
    Name                 string     `json:"name"`
    Pods                 *float64   `json:"pods"` // nil when unknown
    Approximate          bool       `json:"approximate"`
    CPUUsage             Statistics `json:"cpu_usage"`
    MemoryUsage          Statistics `json:"memory_usage"`
    CPURequests          Statistics `json:"cpu_requests"`
    MemoryRequests       Statistics `json:"memory_requests"`
    CPULimits            Statistics `json:"cpu_limits"`
    MemoryLimits         Statistics `json:"memory_limits"`
    CPURequestPercent    Statistics `json:"cpu_request_percent"`
    MemoryRequestPercent Statistics `json:"memory_request_percent"`
    CPULimitPercent      Statistics `json:"cpu_limit_percent"`
    MemoryLimitPercent   Statistics `json:"memory_limit_percent"`
}

type Workload struct {
    Namespace string   `json:"namespace"`
    Kind      string   `json:"kind"`
    Name      string   `json:"name"`
    Pods      *float64 `json:"pods"`
    Approximate bool   `json:"approximate"`
    // the same ten statistics, with the same keys
}

type Finding struct {
    Code       string     `json:"code"`
    Severity   string     `json:"severity"`
    ScopeType  string     `json:"scope_type"`
    ScopeKind  string     `json:"scope_kind"`
    ScopeName  string     `json:"scope_name"`
    Evidence   []Evidence `json:"evidence"` // never nil
    Confidence string     `json:"confidence"`
}

type Evidence struct {
    Name  string  `json:"name"`
    Value float64 `json:"value"`
}

type Input struct {
    // the fields of part 2.1, and:
    Window    collect.Window
    Collected collect.Collected
    Findings  []analysis.Finding
}
```

**Requirements:**

- `Build` takes the period start from `Input.Window.Start` and the period
  end from `Input.Window.End`. The function `periodStart` of `build.go`
  goes away. The golden snapshots keep the period they have.
- `Build` copies rows, messages, and findings into the types of the
  snapshot, field by field. The types of the snapshot stay its own, as
  `Scope` and `Capability` are.
- A statistic whose state is `value` has its four numbers. Any other state
  has four `null`.
- The reader accepts two new shapes and refuses every other as before:
  - a field of type `*float64`: the value is a number or `null`;
  - a field of type `map[string]float64`: the value is an object, and each
    of its values is a number. A `null` in place of the object, or inside
    it, is refused, and the message names the path.
- The writer writes the keys of an object of parameters in sorted order.
- `Check` gains these rules, each with its own message, in this order:
  1. a code of a warning is in `collect.WarningCodes`, and of a notice in
     `collect.NoticeCodes`; no message stands twice in its list;
  2. the parameters of a notice are exactly the names of
     `collect.NoticeParams`; a warning has none; a count is a whole number
     above zero;
  3. the state of a statistic is one of the three; the state `value` has at
     least one number; any other state has none; no number is `NaN` or
     infinite;
  4. a pod count, when present, is a whole number, zero or above;
  5. the namespaces are sorted by name, none twice, and are exactly the
     namespaces of the scope;
  6. the workloads are sorted by namespace, name, and kind, none twice;
     each has a name, a kind, and a namespace of the scope; none has the
     kind `Job`;
  7. a finding has a code of `analysis.Registry`, and the severity, the
     scope type, and the evidence names of its definition; its confidence
     is one of the three; its evidence holds no number that is `NaN` or
     infinite;
  8. a finding of a namespace names a namespace of the snapshot and has an
     empty kind; a finding of a workload names, by `namespace/name` and
     kind, a workload of the snapshot;
  9. the findings are in the order of `analysis.Sort`, none twice.
- `Decode` reads the schema version first, as today.

**Tests:**

| Test | Input | Expected |
|---|---|---|
| `TestBuildCopiesEveryRow` | the rows and findings of `direct-complete` | one namespace, one workload, eight findings, each field equal |
| `TestBuildWritesNullForNoNumber` | a statistic `no_base` and one `unavailable` | `"average": null` and the three others, in the bytes of `Encode` |
| `TestBuildWritesUnknownPodsAsNull` | `PodsKnown` false | `"pods": null` |
| `TestBuildTakesThePeriodFromTheWindow` | a window | the period start and end of the window |
| `TestEncodeDecodeRoundTrip` | a snapshot with every new section filled | `Decode(Encode(s))` equals `s`; `Encode` of that equals the first bytes |
| `TestEncodeSortsParams` | — | the keys of an object of parameters are sorted in the bytes |
| `TestDecodeAcceptsNullNumber` | `"average": null` | read as nil |
| `TestDecodeRefusesNullElsewhere` | `null` at a text, at a list, at an object of parameters, inside it | each refused; the message names the path |
| `TestDecodeRefusesTextAsNumber` | `"average": "1"`; `"count": "3"` | each refused as a value of the wrong kind |
| `TestDecodeRefusesUnknownKeyInNewSections` | an unknown key in a namespace, a workload, a statistic, a finding, an evidence, a message | each refused; the message names the path |
| `TestDecodeRefusesMissingNewSection` | a snapshot of part 2.1, from `git show 2d38ce2:testdata/snapshot/direct-complete.json`, kept as a fixture | refused: `snapshot lacks the key quality.warnings` |
| `TestCheckRules` | one case for each of the nine rules, and for each part of a rule | each refused with the message of its rule |
| `TestCheckAcceptsTwoWorkloadsOfOneName` | `Deployment` `api` and `StatefulSet` `api` | accepted |
| `TestKeyWalkRefusesOtherPointersAndMaps` | a type with `*string`; one with `map[string]string` | the message of the unsupported key |
| every existing test of the package | the builders of their fixtures give the new sections as empty lists | passes |

- [ ] **Step 1:** Write the tests.
- [ ] **Step 2:** Run `go test ./internal/snapshot/ -v`. Expected: fail to
  build, then fail on behaviour. Report the red run on behaviour.
- [ ] **Step 3:** Implement. `internal/cli` and `internal/goldentest` give
  `Build` a window from `collect.NewWindow` and an empty `Collected`, so that
  the module builds; Task 13 gives them the real ones.
- [ ] **Step 4:** Run `go test -count=1 ./internal/snapshot/ ./internal/guard/`
  and `go build ./...` and `make lint`. The golden tests of
  `internal/goldentest` fail now, because the golden snapshots lack the new
  keys. Report them as expected failures; Task 13 rewrites the files.
- [ ] **Step 5:** Commit.

```bash
git add internal/snapshot/ internal/cli/run.go internal/goldentest/goldentest.go
git commit -m "feat(snapshot): store rows and findings"
```

**Done when:** the tests of `internal/snapshot` pass, and the only failing
tests of the module are the golden comparisons, named one by one in the
report.

---

### Task 10: The formats

**Executor:** Subagent.

**Files:**
- Modify: `internal/catalog/format.go`, `format_test.go`

**Interfaces:**
- Consumes: the language of the catalog.
- Produces:

```go
func (c *Catalog) Decimal(value float64, digits int) string
func (c *Catalog) Percent(value float64) string
func (c *Catalog) CPU(cores float64) string
func (c *Catalog) Memory(bytes float64) string
func (c *Catalog) Count(value float64) string
```

**Requirements:**

The formats follow `renderers/text.py:111-162` of the original.

- `Decimal` writes the value with `digits` decimals, removes the zeros at
  the end and then a decimal mark left at the end, and uses the comma in
  Portuguese and the point in English. It adds no thousands separator.
- `Percent` is `Decimal` of one digit, then `%`.
- `CPU`: a value whose absolute is below 1 is written in millicores, the
  value times 1000 with one digit, then ` mCPU`; any other with two digits,
  then ` cores`.
- `Memory`: the value is divided by 1024 while its absolute is at or above
  1024 and a larger unit exists; the units are `B`, `KiB`, `MiB`, `GiB`,
  `TiB`; two digits, a space, the unit.
- `Count` rounds to the nearest whole number, a half to the even one, and
  writes it with no separator.
- None of the five is given a value that is not a number: the caller asks
  the state first. Each writes the placeholder of the caller for nothing;
  it has none of its own.

**Tests:**

| Value | Function | Portuguese | English |
|---|---|---|---|
| 3.5 | `Decimal`, 2 | `3,5` | `3.5` |
| 3 | `Decimal`, 2 | `3` | `3` |
| 0 | `Decimal`, 1 | `0` | `0` |
| 1234567.891 | `Decimal`, 2 | `1234567,89` | `1234567.89` |
| 78.64 | `Percent` | `78,6%` | `78.6%` |
| 91 | `Percent` | `91%` | `91%` |
| 24000 | `Percent` | `24000%` | `24000%` |
| 0.5 | `CPU` | `500 mCPU` | `500 mCPU` |
| 0.0004 | `CPU` | `0,4 mCPU` | `0.4 mCPU` |
| 0.99996 | `CPU` | `1000 mCPU` | `1000 mCPU` |
| 1 | `CPU` | `1 cores` | `1 cores` |
| 3.5 | `CPU` | `3,5 cores` | `3.5 cores` |
| 4.125 | `CPU` | `4,12 cores` | `4.12 cores` |
| 0 | `CPU` | `0 mCPU` | `0 mCPU` |
| 12 | `Memory` | `12 B` | `12 B` |
| 1.5 | `Memory` | `1,5 B` | `1.5 B` |
| 1023 | `Memory` | `1023 B` | `1023 B` |
| 1024 | `Memory` | `1 KiB` | `1 KiB` |
| 1342177280 | `Memory` | `1,25 GiB` | `1.25 GiB` |
| 2 × 1024⁵ | `Memory` | `2048 TiB` | `2048 TiB` |
| 2.5 | `Count` | `2` | `2` |
| 3.5 | `Count` | `4` | `4` |
| 3 | `Count` | `3` | `3` |

Each row is one case of a table test, in both languages. The rows of
`0.99996` and of `1 cores` are what the original prints, and are kept.

- [ ] **Step 1:** Write the tests.
- [ ] **Step 2:** Run `go test ./internal/catalog/ -run Format -v`.
  Expected: fail, because the functions do not exist.
- [ ] **Step 3:** Implement.
- [ ] **Step 4:** Run `go test -count=1 ./internal/catalog/` and `make lint`.
- [ ] **Step 5:** Commit.

```bash
git add internal/catalog/format.go internal/catalog/format_test.go
git commit -m "feat(catalog): format quantities and ratios"
```

**Done when:** the tests pass.

---

### Task 11: The wording of this part

**Executor:** Subagent, for the keys, the files, and the guards. The wording
below is the main thread's and is copied, not rewritten.

**Files:**
- Modify: `internal/catalog/keys.go`, `keys_test.go`
- Modify: `internal/catalog/locales/pt-BR.toml`, `en.toml`
- Modify: `internal/catalog/guard_test.go`
- Modify: `internal/guard/imports.go` only if the table needs it: it does
  not, because `internal/catalog` at rank 80 may import `internal/collect`.

**Interfaces:**
- Consumes: `collect.WarningCodes`, `collect.NoticeCodes`,
  `collect.NoticeParams`.
- Produces: one constant for each key of the first table below, added to
  `wordingKeys`, and:

```go
// Warning returns the key of warning code, one of collect.WarningCodes.
func Warning(code string) Key { return Key("warning." + code) }

// Notice returns the key of notice code, one of collect.NoticeCodes, for
// a count: the entry "singular" for a count of 1, "plural" for any other.
func Notice(code string, count float64) Key
```

`Keys` also returns `Warning` of every code and both entries of `Notice` of
every code.

**The wording.** The Portuguese column is the text of the original. The
English column is new and is reviewed by the owner in Task 18.

| Key | Portuguese (Brazil) | English |
|---|---|---|
| `sections.namespace_label` | Namespace: | Namespace: |
| `sections.namespaces.heading` | Capacidade por Namespace | Capacity by namespace |
| `sections.namespaces.empty` | Nenhum dado de namespace está disponível para os filtros atuais. | No namespace data is available for the current filters. |
| `sections.namespaces.header_resource` | Recurso | Resource |
| `sections.namespaces.header_average` | Uso médio | Average usage |
| `sections.namespaces.header_peak` | Pico | Peak |
| `sections.namespaces.header_request` | Solicitação | Request |
| `sections.namespaces.header_limit` | Limite | Limit |
| `sections.namespaces.header_peak_request` | % Pico/Solicitação | % Peak/Request |
| `sections.namespaces.header_peak_limit` | % Pico/Limite | % Peak/Limit |
| `sections.namespaces.row_cpu` | CPU | CPU |
| `sections.namespaces.row_memory` | Memória | Memory |
| `sections.workloads.heading` | Recursos por carga de trabalho | Resources by workload |
| `sections.workloads.empty` | Nenhum dado de carga de trabalho está disponível para os filtros atuais. | No workload data is available for the current filters. |
| `sections.workloads.header_workload` | Carga de trabalho | Workload |
| `sections.workloads.header_pods` | Pods | Pods |
| `sections.workloads.header_cpu_average` | CPU média | Average CPU |
| `sections.workloads.header_cpu_peak` | CPU pico | Peak CPU |
| `sections.workloads.header_cpu_request` | % CPU/Solicitação | % CPU/Request |
| `sections.workloads.header_cpu_limit` | % CPU/Limite | % CPU/Limit |
| `sections.workloads.header_memory_average` | Memória média | Average memory |
| `sections.workloads.header_memory_peak` | Memória pico | Peak memory |
| `sections.workloads.header_memory_request` | % Memória/Solicitação | % Memory/Request |
| `sections.workloads.header_memory_limit` | % Memória/Limite | % Memory/Limit |
| `placeholder.unavailable` | Indisponível | Unavailable |
| `placeholder.no_base` | Não definido | Not defined |
| `notes.heading` | Avisos de interpretação | Interpretation notes |
| `notes.header_remark` | Observação | Remark |

| Key | Portuguese (Brazil) | English |
|---|---|---|
| `warning.NAMESPACE_USAGE_INCOMPLETE` | As métricas de uso dos namespaces estão incompletas. | The usage metrics of the namespaces are incomplete. |
| `warning.NAMESPACE_REQUESTS_LIMITS_INCOMPLETE` | As métricas de solicitações e limites dos namespaces estão incompletas. | The request and limit metrics of the namespaces are incomplete. |
| `warning.NAMESPACE_REQUESTS_LIMITS_UNAVAILABLE` | As métricas de solicitações e limites dos namespaces estão indisponíveis. | The request and limit metrics of the namespaces are unavailable. |
| `warning.WORKLOAD_USAGE_INCOMPLETE` | As métricas de uso das cargas de trabalho estão incompletas. | The usage metrics of the workloads are incomplete. |
| `warning.WORKLOAD_REQUESTS_LIMITS_INCOMPLETE` | As métricas de solicitações e limites das cargas de trabalho estão incompletas. | The request and limit metrics of the workloads are incomplete. |
| `warning.WORKLOAD_REQUESTS_LIMITS_UNAVAILABLE` | As métricas de solicitações e limites das cargas de trabalho estão indisponíveis. | The request and limit metrics of the workloads are unavailable. |
| `warning.WORKLOAD_HISTORY_CURRENT_ASSOCIATION` | O histórico usa a associação atual entre pods ativos e cargas de trabalho. | The history uses the current association between active pods and workloads. |
| `notice.WORKLOADS_OMITTED_NO_USAGE.singular` | 1 carga de trabalho deixada de fora por não ter consumo observado no período. | 1 workload was left out because it had no observed consumption in the period. |
| `notice.WORKLOADS_OMITTED_NO_USAGE.plural` | {{.Count}} cargas de trabalho deixadas de fora por não terem consumo observado no período. | {{.Count}} workloads were left out because they had no observed consumption in the period. |
| `notice.JOBS_OMITTED.singular` | 1 Job foi deixado de fora do inventário de cargas de trabalho; o consumo continua contabilizado no total do namespace. | 1 Job was left out of the workload inventory; its consumption still counts in the total of the namespace. |
| `notice.JOBS_OMITTED.plural` | {{.Count}} Jobs foram deixados de fora do inventário de cargas de trabalho; o consumo continua contabilizado nos totais dos namespaces. | {{.Count}} Jobs were left out of the workload inventory; their consumption still counts in the totals of the namespaces. |

**Requirements:**

- Both files gain every entry above, letter for letter. The entries of a
  family stand under one table of the file, as the capability reasons do.
- The entry `singular` of a notice takes no parameter. The entry `plural`
  takes
  `Count`. The guard that compares the parameters of both languages
  compares each entry with the same entry of the other language.
- The guard of the catalog gains: every code of `collect.WarningCodes` has
  its entry in both languages; every code of `collect.NoticeCodes` has both
  of its entries in both languages; no key of the families `warning.` and
  `notice.` exists for a code that is in no registry.
- The keys of the first table are used by being named in
  `internal/document`, which Task 12 does. Until then the guard of unused
  keys fails for them. Report these failures as expected, key by key.

**Tests:**

| Test | Input | Expected |
|---|---|---|
| `TestNoticeChoosesTheEntry` | counts 1, 2, 0, 1.0 | `singular`, `plural`, `plural`, `singular` |
| `TestEveryWarningHasItsEntry` | both languages | the seven entries exist and take no parameter |
| `TestEveryNoticeHasBothEntries` | both languages | four entries in each; `plural` takes `Count` |
| `TestNoFamilyKeyWithoutACode` | an entry `warning.UNKNOWN` added to a copy of a file | reported |
| `TestPortugueseOfThisPartEqualsTheOriginal` | each Portuguese text of both tables | equal, letter for letter, to the text in the test file, which is copied from this plan |
| `TestKeysHoldTheNewFamilies` | `Keys()` | holds the 28 constants, 7 warnings, 4 notice entries |

- [ ] **Step 1:** Write the tests.
- [ ] **Step 2:** Run `go test ./internal/catalog/ -v`. Expected: fail,
  because the entries are missing.
- [ ] **Step 3:** Add the keys and the entries.
- [ ] **Step 4:** Run `go test -count=1 ./internal/catalog/` and `make lint`.
  Expected: pass, except the guard of unused keys, for the 28 keys of the
  first table.
- [ ] **Step 5:** Commit.

```bash
git add internal/catalog/
git commit -m "feat(catalog): add wording of sections 1 and 2"
```

**Done when:** the tests pass with the one expected exception, named key by
key in the report.

---

### Task 12: The document

**Executor:** Subagent.

**Files:**
- Modify: `internal/document/document.go`, `document_test.go`
- Create: `internal/document/sections.go`, `sections_test.go`
- Modify: `internal/document/appendix.go`, `appendix_test.go`
- Modify: `internal/document/build.go`, `build_test.go`

**Interfaces:**
- Consumes: the snapshot of Task 9, the formats of Task 10, the keys of
  Task 11, `analysis.Classify`, `analysis.Rank`.
- Produces:

```go
// Ident is a real name: a namespace, a workload.
func Ident(text string) Run
```

and the blocks of sections 1 and 2 and of the interpretation notes, in the
document that `Build` returns.

**The pages.** The blocks stand in this order, which is the one of the
fixture `template/tests/capacity/fixtures/parity.pt-BR.json`, read from the
output of the original:

```text
front
pagebreak, not weak
heading 1, numbered      Capacidade por Namespace
  for each namespace shown:
    heading 2, not numbered   "Namespace: " as text, the name as ident
    table namespace-capacity  two rows: CPU, memory
heading 1, numbered      Recursos por carga de trabalho
  for each namespace that has a workload shown:
    heading 2, not numbered   the same two runs
    table workload-resources  one row for each workload shown
pagebreak, weak
heading 1, numbered      Apêndice técnico de qualidade
table quality-appendix, or its fallback paragraph
  when there is a warning or a notice:
    heading 2, not numbered   Avisos de interpretação
    table interpretation-notes  one row for each
```

**Requirements:**

- The text run of the heading of a namespace is the entry
  `sections.namespace_label` followed by one space.
- **Which rows are shown.** The severity of a namespace is the highest
  severity of the findings whose scope type is `namespace` and whose scope
  name is its name, and healthy when there is none. The severity of a
  workload is the highest of the findings whose scope type is `workload`,
  whose scope name is `namespace/name`, and whose scope kind is its kind. A
  row is shown when its severity is at or above the minimum severity; the
  minimum `all` shows every row.
- A section with no row shown has its heading and then one paragraph, the
  entry `empty` of the section, and no table.
- **The table of a namespace.** Headers: the seven entries
  `sections.namespaces.header_*`, in the order of the wording table. Two
  rows, CPU and then memory. The cells of a row:
  1. the entry `row_cpu` or `row_memory`, as text;
  2. the average of the usage;
  3. the peak of the usage;
  4. the average of the requests;
  5. the average of the limits;
  6. the ratio cell of the request ratio;
  7. the ratio cell of the limit ratio.
- **The table of workloads.** Headers: the ten entries
  `sections.workloads.header_*`, in order. The cells of a row:
  1. the name of the workload, as `ident`; the kind is not printed;
  2. the pod count, by `Count`;
  3. and 4. the average and the peak of the CPU usage;
  5. and 6. the ratio cells of the CPU request and limit ratios;
  7. and 8. the average and the peak of the memory usage;
  9. and 10. the ratio cells of the memory request and limit ratios.
- A quantity of CPU is written by `CPU`, of memory by `Memory`. A quantity
  whose statistic is not `value`, and a pod count that is unknown, is the
  entry `placeholder.unavailable`, as a text run.
- **A ratio cell** is one status run, judged on the peak of the ratio:
  - state `value`: the status is `analysis.Classify` of the peak with the
    thresholds of the snapshot; the text is `Percent` of the peak;
  - state `no_base`: the status `undefined`, the text
    `placeholder.no_base`;
  - state `unavailable`: the status `unavailable`, the text
    `placeholder.unavailable`.
- Namespaces and workloads keep the order of the snapshot.
- **The interpretation notes.** The rows are the warnings, then the
  notices, in the order of the snapshot, each text once. The text of a
  warning is its entry. The text of a notice is the entry that
  `catalog.Notice` chooses by its `count`, with `Count` written by `Count`.
  The header is `notes.header_remark`. The minimum severity does not filter
  the notes. With no warning and no notice, neither the heading nor the
  table is written.
- No string literal of `internal/document` holds wording. The guard of
  wording passes with no new exception comment.

**Tests:**

| Test | Input | Expected |
|---|---|---|
| `TestIdentRun` | `Ident("api")` | `{"kind":"ident","text":"api"}` |
| `TestSectionsOfDirectCompleteInPortuguese` | the snapshot of the scenario | namespace rows: `CPU`, `300 mCPU`, `400 mCPU`, `500 mCPU`, `1 cores`, attention `80%`, healthy `40%`; then `Memória`, `110 B`, `120 B`, `0,5 B`, `1 B`, critical `24000%`, critical `12000%` |
| `TestSectionsOfDirectCompleteInEnglish` | the same | `Memory`, `0.5 B`; headers in English |
| `TestWorkloadRow` | the same | `payments-web` as `ident`; pods `1`; ten cells |
| `TestBlocksFollowTheOrderOfTheOriginal` | the same | the types, levels, and numbering of the list above |
| `TestHeadingOfANamespaceHasTwoRuns` | — | text `Namespace: `, then ident `payments` |
| `TestRatioCellStates` | peaks 69.9, 70, 85; `no_base`; `unavailable` | healthy; attention; critical; undefined with `Não definido`; unavailable with `Indisponível` |
| `TestRatioCellUsesTheThresholdsOfTheSnapshot` | warning 50, critical 60, peak 55 | attention |
| `TestQuantityWithoutValue` | usage `unavailable`; pods unknown | the placeholder, as a text run, in each cell |
| `TestNamespaceThatAnsweredNothingIsPrinted` | a row of ten `unavailable` | the row is on the page with the placeholder in every cell |
| `TestMinimumSeverityFiltersRows` | two namespaces, one with a critical finding and one with none; minimum `all`, `attention`, `critical` | both; the first; the first |
| `TestMinimumSeverityJudgesByKind` | `Deployment` `api` critical and `StatefulSet` `api` healthy; minimum `critical` | one row |
| `TestWorkloadSectionSkipsANamespaceWithoutRows` | two namespaces, the workloads of one filtered out | one heading of level 2 |
| `TestEmptySections` | no row passes | each section is a heading and its paragraph |
| `TestNotesOfRawFallback` | the snapshot of the scenario | one row, the text of `WORKLOAD_HISTORY_CURRENT_ASSOCIATION` |
| `TestNotesChooseThePlural` | the notice with `count` 1, then 3 | the entry `singular`; the entry `plural` with `3` |
| `TestNotesAreLeftOutWhenEmpty` | no warning, no notice | the appendix ends with its table |
| `TestNotesIgnoreTheMinimumSeverity` | minimum `critical`, one warning | the note is written |
| `TestDocumentEqualsParityForItsSections` | a snapshot built to hold the values of the parity fixture for sections 1 and 2 | blocks 2 to 7 of the document equal blocks 2 to 7 of the fixture, as JSON |
| every existing test of the package | unchanged but for the new empty sections | passes |

`TestDocumentEqualsParityForItsSections` builds its snapshot in the test
file: namespace `payments` and the workloads `worker` and `api`, with the
numbers that give the texts of the fixture. The fixture names `worker`
before `api`, which is not the order of the collectors; the test gives the
rows to the builder of the section in the order of the fixture, without
passing through `Check`, and says so in a comment.

- [ ] **Step 1:** Write the tests.
- [ ] **Step 2:** Run `go test ./internal/document/ -v`. Expected: fail.
- [ ] **Step 3:** Implement.
- [ ] **Step 4:** Run
  `go test -count=1 ./internal/document/ ./internal/catalog/ ./internal/guard/`
  and `make lint`. The guard of unused keys passes now.
- [ ] **Step 5:** Commit.

```bash
git add internal/document/
git commit -m "feat(document): print sections 1 and 2"
```

**Done when:** the tests pass, the guard of unused keys passes, and the
report lists the string literals added to `internal/document`.

---

### Task 13: The command and the golden files

**Executor:** Subagent for steps 1 to 5. Main thread for steps 6 to 8.

**Files:**
- Modify: `internal/cli/run.go`, `run_test.go`
- Modify: `internal/goldentest/goldentest.go`, `golden_test.go`
- Rewritten by `make update-testdata`: `testdata/snapshot/*.json`,
  `testdata/queries/*.txt`,
  `template/tests/capacity/fixtures/command/*.json`

**Interfaces:**
- Consumes: `collect.NewWindow`, `collect.All`, `analysis.All`,
  `snapshot.Input`.
- Produces: the flow below, the same in `internal/cli` and in
  `internal/goldentest`.

**Requirements:**

- The flow of a run that collects:
  1. the clock is read, for the emission time;
  2. `collect.NewWindow`; an error that wraps `collect.ErrEmptyPeriod`
     stops the run with exit code 2, before any request;
  3. in a run that writes a PDF, `typst` is located and probed;
  4. connect, resolve the scope, detect the capabilities, check they are
     sufficient, with the end of the window as the time of each instant
     query;
  5. `collect.All`;
  6. the clock is read, for the end of the collection;
  7. `analysis.All` with the thresholds of the configuration;
  8. `snapshot.Build`.
- `--check` stops after step 4. It sends no range query and no query of a
  collector, and reads the clock as it does today.
- An error of a collector is an error of the client and stops the run with
  exit code 3. No file is written.
- A run that collects prints, after the lines of the capabilities,
  `namespace collection: <state>` and `workload collection: <state>`.
  `--check` and `--from-snapshot` print neither.
- The clock is read twice. A test counts the readings.
- `internal/goldentest` runs steps 2 to 8 with its fixed input.

**Tests:**

| Test | Input | Expected |
|---|---|---|
| `TestCheckSendsNoRangeQuery` | `--check` on `direct-complete` | no request to `/query_range`; the same requests as before this part |
| `TestSnapshotRunSendsTheCollectorQueries` | `--snapshot` on `direct-complete` | the requests of the preflight, then fourteen more |
| `TestRunPrintsCollectionLines` | `--snapshot` on `raw-fallback` | `namespace collection: available`, `workload collection: partial`, after the capability lines |
| `TestCheckPrintsNoCollectionLine` | `--check` | neither line |
| `TestEmptyPeriodStopsBeforeAnyRequest` | zone `Europe/Berlin`, the clock at 2026-03-29 03:30, period `1h` | exit code 2; one line `error: analysis.period: ...`; the server saw no request; no file |
| `TestCollectorFailureStopsWithThree` | the server answers 500 to the first range query, three times | exit code 3; no snapshot file; no temporary file |
| `TestClockIsReadTwice` | each of the three modes that collect | two readings |
| `TestFromSnapshotPrintsTheSections` | a snapshot of this part; build tag `template` | the PDF text holds `payments-web` and `Capacity by namespace` |
| `TestFromSnapshotRefusesASnapshotOfThePartBefore` | the fixture of Task 9 | exit code 2; the message names `quality.warnings` |
| `TestGoldenFlowEqualsTheCommand` | `direct-complete`, through the command with the fixed clock and identifier, and through `goldentest` | the same snapshot bytes, but for the collection times, which the test sets equal |

- [ ] **Step 1:** Write the tests.
- [ ] **Step 2:** Run `go test ./internal/cli/ ./internal/goldentest/ -v`.
  Expected: fail.
- [ ] **Step 3:** Implement.
- [ ] **Step 4:** Run `go test -count=1 ./...` and `make lint`. Expected:
  pass, except the comparisons with the golden files.
- [ ] **Step 5:** Commit the code.

```bash
git add internal/cli/ internal/goldentest/
git commit -m "feat(cli): collect namespaces and workloads"
```

- [ ] **Step 6 (main thread):** Run the program by hand, before any review,
  against `cmd/fake-prometheus` with `direct-complete` and with
  `raw-fallback`: `--check`, `--snapshot`, `--from-snapshot` in both
  languages, and a PDF with a relative `--output`. Read both PDFs.
- [ ] **Step 7 (main thread):** Run `make update-testdata`. Read every file
  that changed, whole. For each scenario, check against the expected values
  below. A value that cannot be explained is a defect.

| Scenario | Namespace `payments` | Workload | Findings | Notes |
|---|---|---|---|---|
| `direct-complete`, `grafana-authenticated`, `thanos-multicluster`, `value-edge-cases` | pods 3; CPU request ratio peak 80; memory request ratio peak 24000 | `Deployment` `payments-web`, pods 1, the same numbers | the eight of Task 8 | none |
| `raw-fallback` | the same | `Deployment` `payments-web`, pods 1; CPU request ratio peak about 66.7; memory request ratio about 20000 at both timestamps | namespace as above; workload: CPU request healthy, CPU limit healthy, memory request critical, memory limit critical | the warning of the history |
| `partial-data` | usage as above; requests and limits `unavailable`; the four ratios `no_base` | as `direct-complete` | namespace: the four `MISSING` codes; workload: the four of `direct-complete` | none |

  These values were worked out from the rule of the program and from the
  recorded answers. When a file disagrees, find which of the two is wrong
  before changing either.

- [ ] **Step 8 (main thread):** Check the lists of queries. Each file of
  `testdata/queries/` holds the queries of the preflight, unchanged, and
  then the queries of the collectors in the order of Task 6 and Task 7. In
  the log of `go test -v`, check that the golden tests name every scenario.
  Commit the golden files.

```bash
git add testdata/snapshot testdata/queries template/tests/capacity/fixtures/command
git commit -m "test(golden): record sections 1 and 2"
```

**Done when:** `go test -count=1 ./...` passes, every changed golden file
was read and explained, and the run by hand showed no defect that is not
recorded in `ROADMAP.md`.

---

### Task 14: A scenario with values at the edge (F-40)

**Executor:** Main thread writes the series below into the file. Subagent
adds the scenario to the lists and the tests.

**Files:**
- Create: `testdata/prometheus-own/edge-values.json`
- Create: `testdata/prometheus-own/README.md`
- Modify: `internal/promtest/scenarios.go`, `scenarios_test.go`
- Modify: `internal/goldentest/goldentest.go`
- Modify: `testdata/prometheus/README.md`: one sentence that points to the
  new folder
- Rewritten by `make update-testdata`: the golden files of the new scenario

**Interfaces:**
- Produces:

```go
type Scenario struct {
    // the fields of part 2.1, and:
    Folder string // "prometheus" or "prometheus-own"
}
```

  `promtest.Path` finds a scenario in its folder. `promtest.Scenarios`
  holds seven entries. `goldentest.DataFileScenarios` gains `edge-values`.

**The scenario.** It starts as a copy of the requests of `direct-complete`
and replaces the answers of the fourteen queries of the collectors. The
timestamps are `t1` = 1784050200 and `t2` = 1784093400. Every capability
stays available, with the recording rule.

Namespace `payments`:

| Query | Series and points | What it exercises |
|---|---|---|
| CPU usage | one series: `t1` `0.5`, `t2` `NaN` | a sample that is not a number |
| memory usage | two series of the namespace: `t1` `100`, `t2` `+Inf`; and `t1` `50`, `t2` `60` | two series of one key; an infinite sample |
| requests of CPU | `t1` `0` | a base of zero |
| requests of memory | `t1` `200` | a base that lacks `t2` |
| limits of CPU | no series | an absent base |
| limits of memory | `t1` `300`, `t2` `300` | — |
| pods | `3.0` | — |

Workloads, all of namespace `payments`:

| Workload | Type as answered | CPU usage | Memory usage | Requests and limits | Pod count |
|---|---|---|---|---|---|
| `api` | `Deployment` | `0.2`, `0.9` | `100`, `120` | CPU requests `1`, `1`; CPU limits `1`, `1`; memory requests `400`, `400`; memory limits `800`, `800` | 4 |
| `queue` | `statefulset` | `0.1`, `0.1` | `10`, `10` | no series of any of the four | 2 |
| `idle` | `Deployment` | no series | no series | CPU requests `1`, `1` | 1 |
| `nightly` | `Job` | `0.3`, `0.3` | `30`, `30` | none | 1 |

**Expected, by the rule of the program:**

| Row | Expected |
|---|---|
| namespace, CPU usage | average, minimum, peak, and current `0.5` |
| namespace, memory usage | the point of `t2` holds `60` only, because the infinite sample is left out before the sum; `t1` holds `150`; average `105`, peak `150`, current `60` |
| namespace, CPU request ratio | `no_base` |
| namespace, memory request ratio | `t1` `75`; `t2` `30`, with the base of `t1`; approximate |
| namespace, CPU limit ratio | `no_base` |
| namespace, memory limit ratio | `50`, `20` |
| namespace, findings | `NAMESPACE_CPU_LIMIT_MISSING`; `NAMESPACE_MEMORY_REQUEST_ATTENTION`, inferred; `NAMESPACE_MEMORY_LIMIT_HEALTHY`, inferred; no `NAMESPACE_CPU_REQUEST_MISSING`, because the requests of CPU hold a number |
| `api` | CPU request and limit ratios `20`, `90`: both critical; bursty gap about `63.6`: no finding; memory request ratio `25`, `30`: healthy, and, with an average of `27.5`, `WORKLOAD_MEMORY_OVERREQUESTED`; memory limit ratio `12.5`, `15`: healthy |
| `queue` | kind `StatefulSet`; pods 2; the four ratios `no_base`; the four `MISSING` codes |
| `idle` | left out; the notice `WORKLOADS_OMITTED_NO_USAGE` with `count` 1 |
| `nightly` | left out; the notice `JOBS_OMITTED` with `count` 1 |

The row of the memory usage depends on the order in which the original
leaves out a sample and adds. Task 15 confirms it against the original, and
the expected value above is corrected there if the original says otherwise.

**Requirements:**

- The README of the new folder says that the file was written by this
  project, names what each series exercises, holds the SHA-256 of the file,
  and says that the file is not edited once its recorded numbers exist.
- The fixed input of `goldentest` is unchanged.

**Tests:**

| Test | Input | Expected |
|---|---|---|
| `TestScenariosHoldSeven` | the list | seven names; six with the folder `prometheus`, one with `prometheus-own` |
| `TestPathFollowsTheFolder` | `edge-values` | a path under `testdata/prometheus-own` |
| `TestOriginalScenariosAreUntouched` | the six files | each has the SHA-256 of `testdata/prometheus/README.md` |
| `TestEdgeScenarioLeavesNoAnswerUnused` | a run of `goldentest` | the server has no answer left of the fourteen of the collectors |

- [ ] **Step 1 (main thread):** Write the scenario file and its README.
- [ ] **Step 2:** Write the tests. Run them. Expected: fail.
- [ ] **Step 3:** Implement.
- [ ] **Step 4:** Run `go test -count=1 ./internal/promtest/ ./internal/goldentest/`
  and `make lint`.
- [ ] **Step 5 (main thread):** Run `make update-testdata`. Read the four
  new files, whole, against the table of expected values.
- [ ] **Step 6:** Commit.

```bash
git add testdata/prometheus-own testdata/prometheus/README.md \
  internal/promtest internal/goldentest \
  testdata/snapshot/edge-values.json testdata/queries/edge-values.txt \
  template/tests/capacity/fixtures/command/edge-values.pt-BR.json \
  template/tests/capacity/fixtures/command/edge-values.en.json
git commit -m "test(golden): add scenario of edge values"
```

**Done when:** the tests pass, the golden files of the scenario agree with
the table or the table was corrected with a reason, and F-40 names the
commit.

---

### Task 15: The numbers of the original

**Executor:** Main thread. This task installs one Python package from the
network, once, and waits for the owner's approval (R25).

**Files:**
- Create: `testdata/python/<scenario>.json`, seven files
- Create: `testdata/python/README.md`
- Create: `internal/goldentest/original_test.go`
- Not tracked: `.cache/phase-2/part-2/python/`, with the environment and
  the script

**The recording.**

- [ ] **Step 1:** Ask the owner for approval of the installation.
- [ ] **Step 2:** Make the environment and install the one dependency of
  the core of the original.

```bash
python3 -m venv .cache/phase-2/part-2/python/venv
.cache/phase-2/part-2/python/venv/bin/pip install 'requests>=2.31,<3'
```

- [ ] **Step 3:** Write `.cache/phase-2/part-2/python/record.py`. For each
  of the seven scenarios it:
  - starts `bin/fake-prometheus` with the file of the scenario, on a port
    of the loopback address;
  - builds the configuration of the original with the fixed input of
    `goldentest`: cluster `prod`, namespaces `payments`, excluded
    containers `istio-proxy`, period `24h`, zone `America/Sao_Paulo`, the
    endpoint type, the token, and the node autoscaling of the scenario;
  - calls `collect_report` of `kube_capacity_report.collection` with the
    emission time 2026-07-15 14:30 of that zone;
  - writes the namespaces, the workloads, the findings of scope type
    `namespace` and `workload` whose code starts with `NAMESPACE_` or
    `WORKLOAD_`, the warnings, and the notices of the two collectors.

  The script imports the source from `older/kubernetes-report` by
  `PYTHONPATH` and writes nothing there.
- [ ] **Step 4:** Run it. Read the seven files, whole.

**The recorded file.**

```json
{
  "scenario": "direct-complete",
  "source": "kubernetes-report 1.11.0",
  "namespaces": [
    {
      "namespace": "payments",
      "active_pods": 3,
      "approximate_ratios": false,
      "cpu_usage": { "average": 0.3, "minimum": 0.2, "peak": 0.4, "current": 0.4, "no_base": false }
    }
  ],
  "workloads": [
    { "namespace": "payments", "kind": "Deployment", "name": "payments-web", "active_pods": 1 }
  ],
  "findings": [
    {
      "code": "NAMESPACE_CPU_REQUEST_ATTENTION",
      "severity": "attention",
      "scope_type": "namespace",
      "scope_kind": "",
      "scope_name": "payments",
      "confidence": "exact",
      "evidence": [["average_percent", 60.0], ["peak_percent", 80.0]]
    }
  ],
  "warnings": [],
  "notices": []
}
```

Each row holds the ten statistics, each with its five fields, a number that
is absent written as `null`. The example shows one, and its numbers are
written short. Numbers are written as Python writes them, which is the
shortest text that reads back as the same number.

**Requirements of the test:**

- `TestNumbersEqualTheOriginal` runs each scenario through `goldentest` and
  compares with the recorded file, row by row and number by number.
- Two numbers are equal when they are the same `float64`. There is no
  tolerance. A difference is a finding, recorded in `ROADMAP.md` with the
  source `PROD`, and is explained before anything is changed.
- A statistic of the original with `no_base` true is the state `no_base`;
  with four absent numbers and `no_base` false, `unavailable`; otherwise
  `value`.
- The evidence `approximate` of the original is compared with the
  confidence of the finding and is not looked for in the evidence (R20).
- The findings are compared as sets keyed by code, scope name, and scope
  kind, and then the order of the Go findings is compared with the order of
  the original, the kind left out of the comparison.
- A warning or a notice of the original is a sentence. The test writes each
  message of the Go snapshot with the Portuguese catalog and compares the
  sentences, in order.
- The test reads the recorded files and never writes one.

**Tests:**

| Test | Input | Expected |
|---|---|---|
| `TestNumbersEqualTheOriginal` | the seven scenarios | equal |
| `TestRecordedFilesAreUntouched` | the seven files | each has the SHA-256 of `testdata/python/README.md` |
| `TestComparisonCanFail` | a copy of a recorded file with one number changed in its last digit | the comparison reports that number, by scenario, row, and field |

- [ ] **Step 5:** Write `testdata/python/README.md`: the version of the
  original, the version of Python, the fixed input, the date, the SHA-256
  of each file, and the sentence that the files are recorded answers and
  are never edited.
- [ ] **Step 6:** Write the test, by a subagent brief that holds the
  requirements above. Run it. Record every difference in `ROADMAP.md`.
- [ ] **Step 7:** For each difference, find which tool is right by reading
  the source of the original. Correct the Go code by a fix brief that names
  the class, or record a ruling when the difference is deliberate. Correct
  the expected tables of Task 13 and Task 14 in this plan where the
  original showed them wrong, and say so in the rulings of the execution.
- [ ] **Step 8:** Commit.

```bash
git add testdata/python internal/goldentest/original_test.go
git commit -m "test(golden): compare with the original"
```

**Done when:** `TestNumbersEqualTheOriginal` passes on the seven scenarios,
or each remaining difference is a ruling that the owner has seen. When the
owner declined the installation, the task records that in `ROADMAP.md`, and
the comparison of this part is the test
`TestDocumentEqualsParityForItsSections` of Task 12.

---

### Task 16: Golden pages and a PDF from each scenario

**Executor:** Main thread.

**Files:**
- Modify: `template/golden/snapshots.go`
- Rewritten by `make update-golden`: `template/tests/golden/`
- Modify: `internal/cli/report_real_test.go`

- [ ] **Step 1:** Run `make test-template`. It compiles every data file of
  `template/tests/capacity/fixtures/command/`. A compile that fails or
  warns is a defect of Task 12.
- [ ] **Step 2:** Write the fourteen PDFs, one for each scenario and
  language, into `.cache/phase-2/part-2/pdf/`, by runs that collect against
  the recorded server. Read each one. Look for text that leaves its cell in
  the table of workloads, in English above all, where the headers are new.
- [ ] **Step 3:** Count the pages of each document. Choose the golden pages
  now, with the documents in hand. Each entry of `snapshots.go` states the
  one behaviour it alone protects. At the least:
  - `command-complete-pt` and `command-complete-en` cover every page of
    their documents;
  - one entry from `edge-values` protects the statuses `undefined` and
    `unavailable` in a ratio cell and the two notices in the notes;
  - one entry from `raw-fallback` protects the warning in the notes;
  - `command-partial-en` keeps what it protects, with its page number
    corrected.
- [ ] **Step 4:** Extend `report_real_test.go`: for each scenario and
  language, the text of the PDF holds the heading of each section, the name
  of the workload as written in the snapshot, and no repeated hyphen at a
  line break.
- [ ] **Step 5:** Run `make update-golden`. Open every page that changed.
  A page that changed without a reason that can be named is a defect. The
  pages of `capacity-parity` must not change.
- [ ] **Step 6:** Run `make test-golden` twice in a row. In its log, check
  that every entry of `snapshots.go` is named.
- [ ] **Step 7:** Commit.

```bash
git add template/golden/snapshots.go template/tests/golden internal/cli/report_real_test.go
git commit -m "test(golden): add pages of sections 1 and 2"
```

**Done when:** `make test-template` and `make test-golden` pass, every
changed page was opened, and the pages of `capacity-parity` are unchanged.

---

### Task 17: Documents

**Executor:** Main thread.

**Files:**
- Modify: `README.md`, `AGENTS.md`, `CHANGELOG.md`

- [ ] **Step 1:** `README.md`: what the report holds now; the limit on the
  size of an answer; the refusal of an empty period; the two new lines of a
  run.
- [ ] **Step 2:** `AGENTS.md`:
  - the layers table, with the ranks 50, 55, and 60;
  - under the command: the collectors keep the order of the original, as
    the probes do; `--check` sends no query of a collector;
  - under the golden snapshots: `testdata/prometheus-own/` and
    `testdata/python/`, and the rule that neither is edited;
  - under the rules kept by review: a ratio is never the ratio of two
    averages.
- [ ] **Step 3:** `CHANGELOG.md`, under `Unreleased`, `Added`, each item one
  sentence about what a user perceives:
  - the report holds the capacity of each namespace and the resources of
    each workload, with the peak of consumption judged against the requests
    and the limits;
  - the report lists the interpretation notes of the collection after the
    quality appendix;
  - the snapshot file holds the namespaces, the workloads, the findings,
    the warnings, and the notices;
  - a run prints one line for the collection of the namespaces and one for
    the collection of the workloads;
  - an answer of the endpoint larger than 256 MiB stops the run;
  - a period that is empty because of a change of the offset of the time
    zone stops the run with a message that names it.
- [ ] **Step 4:** Left out of the changelog, and named in the closing
  report: the guards, the scenario of edge values, the recorded numbers of
  the original, the golden files, and the fixes F-38 and F-39.
- [ ] **Step 5:** Run `make test` and `make lint`. Commit.

```bash
git add README.md AGENTS.md CHANGELOG.md
git commit -m "docs: describe sections 1 and 2"
```

**Done when:** the three files are read back and say what the code does.

---

### Task 18: Close the part

**Executor:** Main thread.

- [ ] **Step 1:** Hand the English wording of this part to the owner: the
  two tables of Task 11, as `.cache/phase-2/part-2/english-wording.md`, with
  the entries of doubt first, and the fourteen PDFs. Apply what the owner
  changes, run `make update-testdata` and `make update-golden`, and review
  what changed.
- [ ] **Step 2:** Write the diff of the branch to
  `.cache/phase-2/part-2/branch.diff`. Run the adversarial review by area,
  one reviewer for each: the client and the collectors; the analysis and
  the snapshot; the document and the catalog; the command and the golden
  files. Each writes its findings to a file under
  `.cache/phase-2/part-2/adversarial/` and repeats them in its last message.
- [ ] **Step 3:** Record every finding in `ROADMAP.md`. Fix the ones
  assigned to this part, each with a brief that names the class. Run the
  review again over the fixes.
- [ ] **Step 4:** Run `make gate`. Keep the logs of `.cache/gate/` in
  `.cache/phase-2/part-2/gate/`.
- [ ] **Step 5:** Update `ROADMAP.md`: every item `DONE` with its commit;
  the summary line of 2.2; the rulings R17 to R35 and the rulings of the
  execution, with what the owner said of each. Read `ROADMAP.md` and
  `CHANGELOG.md` back.
- [ ] **Step 6:** Write the closing report to
  `.cache/phase-2/part-2/closing-report.md` and ask the owner for the merge
  and for the decision on the version. The recommendation is to merge with
  no new version and no tag: the snapshot of schema `1.0` is not complete
  before part 2.4.
- [ ] **Step 7:** After the owner approves, merge by fast-forward.

```bash
git switch main
git merge --ff-only feat/namespaces-workloads
```

**Done when:** `make gate` passed on the last commit, the review has no open
finding assigned to this part, and the owner approved the merge.

---

## Self-review of this plan

| Spec | Task |
|---|---|
| 6.1 client: `/query_range`, `POST` only, the attempts | 3 |
| 6.1 a sample that is `NaN` or infinite is no data | 3, 4 |
| 6.4 the query shapes, label order, two-step escaping | 3, 6, 7 |
| 6.4 the step table, no alignment | 4 |
| 6.4 the period start | 4, 13 (R23) |
| 6.4 a workload with no consumption; sorted lists; first-seen order | 7 |
| 6.4 node pool identity | part 2.3 |
| 7.1 the codes, the conditions, the healthy finding | 8, for 40 of the 66 |
| 7.2 the registry | 8; its tie to the catalog in part 2.4 (R21) |
| 7.3 severity and confidence | 8; the minimum severity in 12 |
| 7.4 recommendations | part 2.4 |
| 8 snapshot: `quality.warnings`, `notices`, `namespaces`, `workloads`, `findings` | 9 |
| 8 snapshot: `summary`, `hpas`, `nodes`, `nodepools`, `recommendations` | parts 2.3 and 2.4 |
| 8.1 statistics | 4, 9 |
| 8.2 warnings and notices as codes with parameters | 6, 7, 9 |
| 8.3 integrity | 9 |
| 9.1 content, for what this part prints | 11 |
| 9.2 formats: decimal, percent, CPU, memory | 10 |
| 9.2 formats: period | part 2.3 |
| 9.3 vocabulary | 11 |
| 10.2 the run `ident` | 12 |
| 10.3 sections 1 and 2, their tables, order, and fallback sentences | 12 |
| 10.3 status marks | 12, by the statuses the template already draws |
| 13.2 snapshot and data file goldens | 13, 14 |
| 13.2 the scenario of value edge cases | 14 (R24) |
| 13.3 golden pages in both languages | 16 |
| 16 proof: a PDF from each scenario | 16 |
| 16 proof: a real cluster | part 2.4 |
| 17 the English wording reviewed | 18, second batch |
| 17 the comparison with the Python tool, number by number | 15 |

Known limits of this plan:

- The expected values of Task 13 and Task 14 were worked out from the rule
  of the program and from the recorded answers, not by running the
  original. Task 15 is where they are confirmed or corrected.
- The expected row of the memory usage in Task 14 depends on a detail of
  the original, the moment at which a sample that is not finite is left
  out, that was read and not run.
- The English headers of the table of workloads are longer than the
  Portuguese ones in two columns. Whether they fit is seen on the page, in
  Task 16, and the wording may change there.
- The limit of 256 MiB comes from an estimate. No real answer was measured.
- The Go documentation does not promise which way a wall clock inside a
  skipped hour is moved. The three tests of Task 4 pin what Go 1.27.1 does;
  a later Go that does otherwise fails them, and R23 is then read again.
- The comparison of Task 15 needs the owner's approval of one installation
  from the network.
