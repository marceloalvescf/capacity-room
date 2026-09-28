# Phase 2.1: Command Spine Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** `k8s-capacity-report report` reads its configuration, talks to a Prometheus-compatible API, discovers the cluster scope and the nine capabilities, stores a snapshot, and writes a PDF with the cover, the opening page, and the quality appendix, in Portuguese (Brazil) and in English.

**Architecture:** One thread through every layer of spec section 4.3, with no capacity numbers yet: `config -> prom -> discovery -> snapshot -> catalog + document -> typst -> PDF`. The snapshot is the only handover: everything after `snapshot.Build` reads the snapshot and nothing else, so `--snapshot` and `--from-snapshot` are the two halves of the same run. Core packages receive the part of `config.Config` they need and never read the environment, the clock, or the file system on their own; `internal/cli` injects all three.

**Tech Stack:** Go 1.27, `github.com/spf13/cobra`, `github.com/BurntSushi/toml` (both already in `go.mod`), `github.com/nicksnyder/go-i18n/v2`, `golang.org/x/text`, `github.com/google/uuid`, Typst 0.15.1, `pdftotext`, Docker.

**Spec:** `docs/superpowers/specs/2026-09-27-k8s-capacity-report-design.md`, sections 4, 5, 6.1 to 6.3, 8, 9, 10, 13.2, 15, and 16.

**Depends on:** Phases 0 and 1 and the adversarial review of 2026-09-28, merged into `main` at `7f150d9`.

## Where this part sits

The owner divided phase 2 into four parts on 2026-09-28. Each part has its own
plan and its own branch, and is merged into `main` by fast-forward after
`make gate` and an adversarial review of that branch.

| Part | Delivers | PDF at the end |
|---|---|---|
| **2.1 Spine (this plan)** | Configuration, redaction, client, discovery, the fake server, the catalog mechanism, the four modes of `report` | Cover, opening page, quality appendix |
| 2.2 Namespaces and workloads | Two collectors, workload identity, the finding registry, 40 finding codes | Sections 1 and 2 |
| 2.3 Autoscalers and nodes | HPA, nodes, node pools, cluster autoscaling, 26 finding codes | Sections 3 and 4 |
| 2.4 Executive profile and proofs | Recommendations, minimum severity, the executive body, the complete catalog, the comparison with the Python tool, a real cluster | Both profiles |

### Out of this part

These belong to a later part and must not be built now.

| Left out | Arrives in | Reason |
|---|---|---|
| `QueryRange`, matrix results, the step table | 2.2 | Discovery asks instant queries only |
| Collectors, resolvers, analysis, findings | 2.2, 2.3 | No capacity number is printed in this part |
| `quality.warnings` and `quality.notices` in the snapshot | 2.2 | The collectors produce them; discovery produces none |
| Formats for decimal, percent, CPU, memory, and period | 2.2, 2.3 | A format enters with the first value that prints it |
| The executive profile | 2.4 | It is made of findings |
| The comparison with the Python tool | 2.2 onwards | There is no number to compare yet |
| A limit on the size of a response body | 2.2 | It needs the size of a real range answer; recorded as F-35 |

## How to read this plan

The owner's orchestration rule applies: the plan states what each task must
deliver and how it is proven, and the implementer writes the code. Each task
gives exact paths, exact signatures, and a table of tests with inputs and
expected results. Write each test first, watch it fail, then implement.

Executors:

- **Subagent**: a Sonnet subagent. It may run `go`, `make build`, `make test`,
  and `make lint`, and the tagged tests with the native tools:
  `go test -count=1 -tags=template ./internal/cli/`. It must not run Docker
  and must not declare a task finished.
- **Main thread**: needs Docker, judgment, wording, page review, or the owner.

Rules for every subagent brief:

- Tasks run in sequence. Two agents never share the checkout.
- A reviewer receives a diff file under `.cache/phase-2/part-1/` and runs
  tests only on packages nobody is editing.
- Every commit names its paths: `git add <paths>`, never `git add -A`.
- A fix brief names the class of the defect and asks for its other members.
- A brief that says "report every X" also lists, as tests, the cases that
  must not match.

Reference material, read-only and never tracked:

- `older/kubernetes-report/kube_capacity_report/`: the Python source, which
  is the reference for every detail of behaviour that this plan does not
  repeat.
- `.cache/phase-2/briefs/python-preflight.md`: the facts extracted from that
  source for this part, with file and line for each.
- `docs/reference/inventory/collection.md` and `document-frontend.md`.

Inside `older/`, nobody reads `.env`, `.envrc`, `report.toml`, anything under
`deploy/secrets`, `.venv`, `.git`, `.worktrees`, `build`, or `node_modules`.

## Global Constraints

- Go 1.27. Module `github.com/marceloalvescf/k8s-capacity-report`.
- Metrics only. No Kubernetes or cloud client may appear in `go.mod`.
- Imports point one way. Every new package is added to `guard.Layers`.
- Every query is `POST` with a form-encoded body. There is no `GET`.
- Proxy variables from the environment are ignored. Redirects are never
  followed.
- Timeout is 90 seconds per request by default. Three attempts, with
  exponential backoff, on connection failure, 429, and 5xx. `Retry-After` is
  honoured up to 60 seconds.
- A sample that is `NaN` or infinite is "no data". It is never zero.
- Exit codes: `0` success, `2` configuration, `3` metrics, `4` cluster scope,
  `5` insufficient capabilities, `6` output, including a missing or wrong
  `typst`.
- `metrics.token` in the file is rejected. The token is held in memory only.
- Every message printed or logged passes through `internal/redact`.
- Internal addresses, host names, provider identifiers, endpoint addresses,
  and tokens never enter the snapshot.
- The snapshot holds codes and numbers. It holds no sentence.
- Wording lives only in `internal/catalog`. No other package contains a
  sentence meant for a reader.
- Diagnostics, logs, and help text are in English and are not in the catalog.
- The Portuguese sentences are carried over unchanged from the original.
- No em-dash inside a sentence. The em-dash alone marks an empty table cell.
- Kubernetes names and codes stay as they are in both languages.
- The same input gives the same snapshot and the same `report.json`, byte for
  byte. The clock and the report identifier are injected.
- Output files are written atomically.
- Typst is pinned at exactly `0.15.1`, called by absolute path with an empty
  environment. A compiler warning is a failure.
- `template.Unpack` writes only into a fresh folder. The command creates a
  new folder for each run and discards it afterwards.
- `report.json` is strict in both directions. A field enters the contract
  with the feature that prints it.
- No test skips. Every build tag is declared in `guard.BuildTags`.
- A test never writes a golden file. A `make` target does, and every changed
  file is reviewed before it is committed.
- Scratch files go under `.cache/`, never under `/tmp` on the host.
- Commits follow `type(scope): description`, imperative, subject under 50
  characters.

## Review Focus

Each line names an input that the spec implies and does not describe, and the
task whose tests pin it.

1. **A secret inside a refused address.** `metrics.url` is
   `https://user:hunter2@prom.example/?token=abc`. The run must stop with
   exit code 2, and neither `hunter2` nor `abc` may appear on either stream.
   Pinned in Task 14.
2. **An output path that cannot be written.** `--output` names a folder that
   does not exist, names an existing folder, or names an existing file. The
   first two stop with exit code 6 and name the path. The third replaces the
   file only when the new one is complete; when the compile fails, the old
   file is untouched and no temporary file is left beside it. Pinned in
   Task 14.
3. **A work folder that outlives the run.** The run fails in the middle, or
   the temporary folder has a space in its path. No work folder is left
   behind in either case, and the compile works in the second. Pinned in
   Task 14 and Task 15.
4. **An answer that is not Prometheus.** A proxy answers 200 with an HTML
   sign-in page. The message must say that the endpoint did not answer as a
   Prometheus API, and must not be the text of a JSON parse error. Pinned in
   Task 6.
5. **A snapshot file that this version cannot read.** The file holds another
   `schema_version`, an unknown key, a second JSON document after the first,
   or is cut in the middle. Each stops with exit code 2 and its own message.
   Pinned in Task 9 and Task 14.

## Rulings made on the owner's behalf

Each of these is a decision the spec leaves open. The owner may overturn any
of them when reviewing this plan.

| ID | Ruling |
|---|---|
| R1 | Ruling: the four modes of `report` exclude each other; `--snapshot` together with `--output` is refused — one collection still gives both files, through `--snapshot` and then `--from-snapshot` — a user who wants both types two commands |
| R2 | Ruling: `--from-snapshot` reads no configuration file and refuses `--config` and every collection flag; it takes `--lang`, `--report-profile`, `--min-severity`, `--output`, and their environment variables — the snapshot is the only handover, and `metrics.url` would otherwise be required for a run that makes no request — a user who keeps the language in the file must pass `--lang` |
| R3 | Ruling: the profile `executive` is refused with exit code 2 until part 2.4 — printing the detailed body under the name of another profile would mislead the reader — a user cannot try the executive profile before part 2.4 |
| R4 | Ruling: `--output` and `--snapshot` are used as given, as `--output` is in the original; without `--output` the PDF goes to `report.output_directory` under `report-resources-<cluster><suffix>-<language>-<YYYYMMDD-HHMMSS>.pdf`; `--output` must end in `.pdf` and `--snapshot` in `.json` — this is the original rule, plus the language, because two PDFs of one snapshot share the emission time — a script that expects the old file name must change. Closes F-31 |
| R5 | Ruling: the standalone command keeps no digest of a snapshot file — a digest kept beside a file protects against nothing that the strict reader does not already refuse, and the digest of spec 8.3 belongs to the stored snapshot of phase 3 — a file damaged into another valid snapshot is rendered without a warning. Closes F-32 |
| R6 | Ruling: a `--from-snapshot` file that is unreadable or invalid stops with exit code 2 — it is an input named by the operator, as the configuration file is — a script that tells a bad snapshot from a bad configuration by exit code cannot. Closes F-33 |
| R7 | Ruling: a failure that is none of the five kinds, including a recovered panic, stops with exit code 1 and the line `error: internal failure: <redacted text>` — the spec lists no code for it, and 1 is what the original gave through the interpreter — none known |
| R8 | Ruling: when `metrics.token_env` names a variable that is unset or empty, the run stops with exit code 2; the original went on with no token — the operator otherwise reads "rejected authentication" and looks for the fault in the endpoint — a setup that names a variable and relies on an open endpoint fails until the key is removed |
| R9 | Ruling: capability evidence holds metric names only; the original also held two Portuguese phrases and the words `enabled` and `disabled` — the snapshot holds no sentence, and the collectors of part 2.2 read the evidence as names — the data quality view of phase 4 lists other evidence than the Python platform did. Details in Task 8 |
| R10 | Ruling: the snapshot of this part holds `report`, `scope`, and `quality.capabilities` with the collection times; `warnings`, `notices`, the excluded namespaces, and every later section enter with the part that produces them — a field that nothing fills cannot be tested — a snapshot written by part 2.1 is refused by part 2.2, which is accepted because no version is released |
| R11 | Ruling: the golden data files of the command live under `template/tests/capacity/fixtures/command/` — one file is then at once the golden output of `document`, a fixture of the template tests, and the source of golden pages — the `document` tests read a folder of another package |
| R12 | Ruling: in a run that writes a PDF, `typst` is located and probed before the first request to Prometheus — a collection of minutes is not wasted on a missing compiler — none known |
| R13 | Ruling: `--check` prints the lines of the original, in English, unchanged — scripts may read them — the lines say less than they could |
| R14 | Ruling: the English label of a warning callout is `Warning`; the fixture of phase 1 used `Attention` — `Attention` is also a severity, and one word should not have two jobs — the owner reviews the wording in any case |
| R15 | Ruling: `--config` defaults to `report.toml` and a missing file is an error, as in the original — a run cannot be configured from the environment alone — a user without a file must create one |
| R16 | Ruling: capabilities are insufficient when `namespace_usage`, `workload_usage`, `hpa_history`, `workloads_without_hpa`, and `node_utilization` are all unavailable and `node_autoscaling` is not available — the rule of the original can never fire, because it waits for `node_autoscaling` to be unavailable, a state that capability never takes (F-37) — a run against an empty Prometheus now stops at the preflight with exit code 5, where the original stopped with the same code after collecting |

## Findings this plan closes or records

| ID | What happens in this part |
|---|---|
| F-1 | Closed by Task 1 |
| F-9 | Closed by Task 2 |
| F-30 | Closed by Task 0 |
| F-31, F-32, F-33 | Closed by the rulings R4, R5, R6 and the tests of Task 14 |
| F-35 | Recorded by Task 0: no limit on the size of a response body; assigned to part 2.2 |
| F-36 | Recorded by Task 0: the original lists two of the three raw node metrics as evidence and names no exact recording rule; this port lists exact names (R9) |
| F-37 | Recorded by Task 0 and closed by Task 8: the preflight rule of the original for insufficient capabilities can never fire (R16) |
| F-40 | Recorded during Task 5: the scenario file `value-edge-cases.json` of the original is byte for byte `direct-complete.json`, so it holds no value at an edge; assigned to part 2.2, which prints the first numbers |

---

## File structure

| Path | Responsibility |
|---|---|
| `internal/guard/imports.go` | Layers table; reports an edge to an unclassified package |
| `internal/guard/wording.go` | Finds reader-facing text outside the catalog |
| `internal/guard/binary_test.go` | The production binary holds no test support |
| `internal/typst/compile.go`, `toolchain.go` | One refusal message per cause |
| `internal/redact/redact.go` | Removes secrets from text |
| `internal/config/config.go` | The `Config` types and their defaults |
| `internal/config/load.go` | File, then environment, then flags |
| `internal/config/validate.go` | Every rule of spec 5.2 |
| `internal/promtest/fixture.go` | Reads a scenario file; answers requests from it |
| `internal/promtest/scenarios.go` | The list of the six scenarios |
| `cmd/fake-prometheus/main.go` | Serves one scenario file over HTTP |
| `testdata/prometheus/*.json` | The six scenarios, carried over unchanged |
| `internal/prom/endpoint.go` | Candidate roots and endpoint kind |
| `internal/prom/client.go` | `POST`, retries, errors, the probe |
| `internal/prom/result.go` | Reads a vector answer |
| `internal/prom/selector.go` | Selectors and the two-step escaping |
| `internal/discovery/scope.go` | Cluster scope, three filter modes |
| `internal/discovery/capabilities.go` | The nine capabilities |
| `internal/discovery/registry.go` | Names, states, strategies, reason codes |
| `internal/snapshot/snapshot.go` | The types of schema `1.0` |
| `internal/snapshot/build.go` | From configuration and discovery to a snapshot |
| `internal/snapshot/codec.go` | Strict reading, fixed writing, integrity |
| `internal/goldentest/goldentest.go` | Runs a scenario to a snapshot and to data files, for the golden tests and the update command |
| `testdata/snapshot/*.json` | Golden snapshots, one per scenario |
| `internal/catalog/catalog.go` | Loads a language; turns a key into text |
| `internal/catalog/keys.go` | Every valid key |
| `internal/catalog/format.go` | Date and time, whole numbers |
| `internal/catalog/locales/pt-BR.toml`, `en.toml` | The wording |
| `internal/document/document.go` | The types of `report.json`, schema `1` |
| `internal/document/build.go` | From a snapshot and a language to a document |
| `internal/document/front.go`, `appendix.go` | The two parts this plan prints |
| `template/tests/capacity/fixtures/command/*.json` | Golden data files |
| `cmd/update-testdata/main.go` | Rewrites golden snapshots and data files |
| `internal/cli/report.go` | The `report` command: flags and modes |
| `internal/cli/run.go` | The flow, the exit codes, the streams |
| `internal/cli/output.go` | Output paths and atomic writing |
| `internal/cli/report_real_test.go` | Build tag `template`: a PDF from each scenario |
| `Makefile`, `README.md`, `AGENTS.md`, `CHANGELOG.md` | Gate, documents |

---

### Task 0: Open the part

**Executor:** Main thread.

- [ ] **Step 1:** Run `git status`, `git log --oneline -5`, and
  `go test -count=1 ./...`. Read `ROADMAP.md` and `CHANGELOG.md` and confirm
  they describe what the commands show. Correct the record before going on.
- [ ] **Step 2:** Confirm the branch. It was created when this plan was
  written.

```bash
git branch --show-current
```

Expected: `feat/command-spine`.

- [ ] **Step 3:** In `ROADMAP.md`, add phase lines 2.1 to 2.4 to the summary
  table in place of the line of phase 2, add the section
  `## Phase 2.1. Command spine` with one row per task of this plan, and
  record F-35, F-36, and F-37 in the findings log with the text of the table
  above.
- [ ] **Step 4:** Close F-30. In the spec, line 4, replace
  `Status: awaiting review` by `Status: approved by the owner on 2026-09-27`.
- [ ] **Step 5:** Commit the plan and the spec line.

```bash
git add docs/superpowers/plans/2026-09-28-phase-2-1-command-spine.md \
  docs/superpowers/specs/2026-09-27-k8s-capacity-report-design.md
git commit -m "docs(plan): add the plan of phase 2.1"
```

**Done when:** the branch is `feat/command-spine`, `ROADMAP.md` names every
task of this plan, and F-30 names the commit.

---

### Task 1: The import guard reports every edge (F-1)

**Executor:** Subagent.

**Class of the defect:** a check stays silent because it trusts another check
to speak. `ImportViolations` skips an import of an unclassified package,
because that package is reported when it is visited as a source. With a
partial map it is never visited. The brief asks for the other members of this
class inside `internal/guard`, and lists what was found in the report.

**Files:**
- Modify: `internal/guard/imports.go:16-26` (the table) and `:103-129`
- Modify: `internal/guard/imports_test.go`

**Interfaces:**
- Consumes: nothing new.
- Produces: the same signature,
  `func ImportViolations(module string, layers []Layer, imports map[string][]string) []string`,
  and this table:

```go
var Layers = []Layer{
    {Path: "internal/guard", Rank: 0},
    {Path: "internal/typst", Rank: 0},
    {Path: "internal/redact", Rank: 0},
    {Path: "internal/promtest", Rank: 0},
    {Path: "template", Rank: 0},
    {Path: "template/golden", Rank: 10},
    {Path: "internal/config", Rank: 20},
    {Path: "internal/prom", Rank: 30},
    {Path: "internal/discovery", Rank: 40},
    {Path: "internal/snapshot", Rank: 70},
    {Path: "internal/catalog", Rank: 80},
    {Path: "internal/document", Rank: 90},
    {Path: "internal/goldentest", Rank: 95},
    {Path: "internal/cli", Rank: 100},
}
```

Ranks 50 and 60 are kept for `collect`, `resolve`, and `analysis`.

**Requirements:**

- An import of a package of this module that is not the root package, is not
  under `cmd/`, and matches no layer is reported at the edge:
  `<source> imports <target>: <target> is not classified in guard.Layers`.
- The rule holds for every source: a layered package, the root package, and a
  package under `cmd/`. A package under `cmd/` is still free to import any
  classified package.
- A package that is a key of the map and matches no layer is still reported
  by its own line, as today. Both lines appear when both are true.

**Tests:**

| Test | Input | Expected |
|---|---|---|
| `TestImportViolationsReportsEdgeToUnclassified` | only key `internal/cli`, importing `internal/mystery` | one entry, naming both packages |
| `TestImportViolationsReportsEdgeFromCmd` | only key `cmd/tool`, importing `internal/mystery` | one entry, naming both |
| `TestImportViolationsReportsSourceAndEdge` | keys `internal/cli` and `internal/mystery`, the first importing the second | two entries |
| `TestImportViolationsEdgeIgnoresStandardLibrary` | `internal/cli` imports `fmt` and `github.com/spf13/cobra` | empty |
| `TestImportViolationsEdgeIgnoresRoot` | `internal/cli` imports the root package | empty |
| `TestImportViolationsEdgeAllowsCmdToClassified` | `cmd/tool` imports `internal/cli` | empty |
| `TestLayersHoldEveryPackageOfThisPart` | the table above | each of the nine new paths is present with its rank |
| every existing test of `imports_test.go` | unchanged | passes unchanged |

- [ ] **Step 1:** Write the tests.
- [ ] **Step 2:** Run `go test ./internal/guard/ -run ImportViolations -v`.
  Expected: the three `Reports` tests fail, because the entries are missing.
- [ ] **Step 3:** Change `ImportViolations` and the table.
- [ ] **Step 4:** Run `go test -count=1 ./internal/guard/`. Expected: pass.
- [ ] **Step 5:** Run `make lint`. Expected: pass.
- [ ] **Step 6:** Commit.

```bash
git add internal/guard/imports.go internal/guard/imports_test.go
git commit -m "fix(guard): report edge to unclassified package"
```

**Done when:** the tests pass, the report lists the other members of the
class that were looked for, and F-1 names the commit.

---

### Task 2: One refusal message for each cause (F-9)

**Executor:** Subagent.

**Class of the defect:** one message answers for several causes, so the
operator cannot tell which one happened. Known members, all in
`internal/typst`:

| Place | One message today | Causes |
|---|---|---|
| `compile.go:103` | `pages and ppi apply only to PNG` | `Pages` set; `PPI` set |
| `toolchain.go:30` | `KCR_TYPST must be an absolute path to the typst binary` | not absolute; names no file; names something that is not a regular file |
| `toolchain.go:36-43` | `typst not found: set KCR_TYPST or add typst to PATH` | not on `PATH`; found, and the path could not be made absolute |

The brief asks for other members in `internal/typst` and `template/embed.go`.

**Files:**
- Modify: `internal/typst/compile.go`, `internal/typst/compile_test.go`
- Modify: `internal/typst/toolchain.go`, `internal/typst/toolchain_test.go`

**Interfaces:** no signature changes.

**Requirements:** each cause has its own message.

| Cause | Message |
|---|---|
| `Pages` set with `PDF` | `pages applies only to PNG` |
| `PPI` set with `PDF` | `ppi applies only to PNG` |
| both set with `PDF` | the message of `Pages`, which is checked first |
| `KCR_TYPST` not absolute | `KCR_TYPST must be an absolute path: "<value>"` |
| `KCR_TYPST` names no file | `KCR_TYPST names no file: "<value>"` |
| `KCR_TYPST` names a folder or another kind of entry | `KCR_TYPST is not a regular file: "<value>"` |
| `typst` not on `PATH` | `typst not found: set KCR_TYPST or add typst to PATH` |
| found, not made absolute | `typst found at "<path>", which could not be made absolute: <cause>` |

**Tests:**

| Test | Input | Expected |
|---|---|---|
| `TestArgsRejectsPagesForPDF` | PDF with `Pages: "1"` | exactly `pages applies only to PNG` |
| `TestArgsRejectsPPIForPDF` | PDF with `PPI: 144` | exactly `ppi applies only to PNG` |
| `TestArgsRejectsPagesBeforePPI` | PDF with both | the message of `Pages` |
| `TestLocateRejectsRelativeOverride` | `KCR_TYPST=bin/typst` | message has `must be an absolute path` |
| `TestLocateRejectsMissingOverride` | absolute path of no file | message has `names no file` |
| `TestLocateRejectsFolderOverride` | absolute path of a folder | message has `is not a regular file` |
| `TestLocateAcceptsSymbolicLinkOverride` | a link to a regular file, as today | no error |

- [ ] **Step 1:** Write the tests, and change the existing ones that compare
  the old text.
- [ ] **Step 2:** Run `go test ./internal/typst/ -v`. Expected: the new tests
  fail on the text.
- [ ] **Step 3:** Change the messages.
- [ ] **Step 4:** Run `go test -count=1 ./internal/typst/` and
  `go test -count=1 -tags=toolchain ./internal/typst/`. Expected: pass.
- [ ] **Step 5:** Run `make lint`, then commit.

```bash
git add internal/typst/compile.go internal/typst/compile_test.go \
  internal/typst/toolchain.go internal/typst/toolchain_test.go
git commit -m "fix(typst): give each refusal its own message"
```

**Done when:** the tests pass and F-9 names the commit.

---

### Task 3: Redaction

**Executor:** Subagent.

**Reference:** `older/kubernetes-report/kube_capacity_report/redaction.py` and
`tests/test_redaction.py`; section 2 of `python-preflight.md`.

**Files:**
- Create: `internal/redact/redact.go`, `internal/redact/redact_test.go`

**Interfaces:**
- Consumes: nothing.
- Produces:

```go
package redact

// Mark replaces every secret found.
const Mark = "[redacted]"

// Redactor removes a fixed set of literal secrets and every pattern of
// spec 5.4 from text.
type Redactor struct{ /* unexported */ }

// New keeps the secrets that are not empty.
func New(secrets ...string) *Redactor

// Text returns s with every secret removed.
func (r *Redactor) Text(s string) string

// Writer returns a writer that redacts each Write call as one text and
// passes the result to w. It reports len(p) on success, so that a caller
// never sees a short write caused by the redaction.
func (r *Redactor) Writer(w io.Writer) io.Writer
```

**Requirements:** six rules, applied in this order. Rules 2 to 6 are the
regular expressions of the original, quoted in `python-preflight.md`.

| Order | Rule | Result |
|---|---|---|
| 1 | each literal secret, longest first | `[redacted]` |
| 2 | a quoted `authorization` key with a quoted value, any letter case | the value becomes `[redacted]`, the quotes stay |
| 3 | a bare `authorization:` or `authorization=` up to the end of the line | `authorization: [redacted]`, keeping what was matched before the value |
| 4 | `token`, `access_token`, or `api_key` followed by `:` or `=` and a value | `<name>=[redacted]`, always with `=` |
| 5 | the query string of an `http` or `https` address | `<address>?[redacted]` |
| 6 | user info in an address of any scheme | `<scheme>://[redacted]@` |

- A zero `Redactor`, and one built by `New()` with no secret, still apply
  rules 2 to 6.
- "Longest first" is a difference from the original, which keeps the order
  given. It stops a short secret that is part of a longer one from leaving
  the rest of the longer one in the text.

**Tests:**

| Test | Input | Expected |
|---|---|---|
| `TestTextRemovesLiteralSecret` | secret `s3cr3t`, text `token was s3cr3t here` | `token was [redacted] here` |
| `TestTextIgnoresEmptySecret` | secret `""`, text `abc` | `abc` |
| `TestTextRemovesLongerSecretFirst` | secrets `abc`, `abcdef`; text `x abcdef y` | `x [redacted] y` |
| `TestTextRedactsQuotedAuthorization` | `{"Authorization": "Bearer x.y"}` | `{"Authorization": "[redacted]"}` |
| `TestTextRedactsSingleQuotedAuthorization` | `{'authorization': 'Bearer x'}` | `{'authorization': '[redacted]'}` |
| `TestTextRedactsBareAuthorizationLine` | `Authorization: Bearer abc\nnext` | `Authorization: [redacted]\nnext` |
| `TestTextRedactsAssignments` (table) | `token=abc`, `access_token: abc`, `API_KEY = abc` | `token=[redacted]`, `access_token=[redacted]`, `API_KEY=[redacted]` |
| `TestTextAssignmentStopsAtSeparator` | `token=abc, next` | `token=[redacted], next` |
| `TestTextRedactsQueryString` | `https://h.example/p?a=1&b=2 done` | `https://h.example/p?[redacted] done` |
| `TestTextRedactsUserInfo` | `connection failed: postgresql+psycopg://kcr_admin:SuperSecretPass123@10.0.0.5/kcr` | `connection failed: postgresql+psycopg://[redacted]@10.0.0.5/kcr` |
| `TestTextLeavesPlainTextAlone` (table) | `tokenizer`, `the token bucket`, `https://h.example/p`, `a@b` | unchanged |
| `TestTextIsStableWhenAppliedTwice` | every expected value above | `Text(Text(x)) == Text(x)` |
| `TestWriterRedactsEachWrite` | secret `s3cr3t`; one write of `a s3cr3t b\n` | the inner writer received `a [redacted] b\n`; the call returned 11 and no error |
| `TestWriterReturnsInnerError` | inner writer fails | the same error |

- [ ] **Step 1:** Write the tests.
- [ ] **Step 2:** Run `go test ./internal/redact/ -v`. Expected: build
  failure.
- [ ] **Step 3:** Write `redact.go`.
- [ ] **Step 4:** Run `go test -count=1 ./internal/redact/`. Expected: pass.
- [ ] **Step 5:** Run `make test` and `make lint`, then commit.

```bash
git add internal/redact/
git commit -m "feat(redact): remove secrets from text"
```

**Done when:** the tests pass and the import guard accepts the package.

---

### Task 4: Configuration

**Executor:** Subagent.

**Reference:** `older/kubernetes-report/kube_capacity_report/config.py` and
`tests/test_config.py`; section 1 of `python-preflight.md`; spec 5.2.

**Files:**
- Create: `internal/config/config.go`, `load.go`, `validate.go`
- Create: `internal/config/config_test.go`, `load_test.go`, `validate_test.go`
- Create: `internal/config/testdata/*.toml`

**Interfaces:**
- Consumes: `github.com/BurntSushi/toml`, already in `go.mod`.
- Produces:

```go
package config

type Config struct {
    Metrics    Metrics
    Cluster    Cluster
    Analysis   Analysis
    Thresholds Thresholds
    Report     Report
}

type Metrics struct {
    URL            string  // metrics.url
    EndpointType   string  // auto, prometheus, grafana
    TokenEnv       string
    TLSVerify      bool
    CABundle       string
    TimeoutSeconds float64
}

type Cluster struct {
    Name          string
    Namespaces    []string
    Label         string
    FilterMode    string  // required, auto, none
    NodepoolLabel string
}

type Analysis struct {
    Period             string  // for example "24h"
    Timezone           string
    NodeAutoscaling    string  // auto, enabled, disabled
    ExcludedContainers []string
}

type Thresholds struct {
    WarningPercent           float64
    CriticalPercent          float64
    LowUtilizationPercent    float64
    AutoscalerNearMaxPercent float64
    NodepoolSpreadPoints     float64
    BurstyGapPercent         float64
    HPAReplicaDivergence     float64
}

type Report struct {
    Profile         string  // detailed, executive
    MinSeverity     string  // all, attention, critical
    Language        string  // pt-BR, en
    Organization    string
    Classification  string
    Author          string
    OutputDirectory string
}

// Default returns the defaults of spec 5.2. Required keys are empty.
func Default() Config

// Flags holds the values given on the command line. A nil field was not
// given.
type Flags struct {
    MetricsURL, EndpointType, Token                *string
    Cluster, ClusterLabel, ClusterFilter           *string
    Namespaces, Period, Timezone, NodeAutoscaling  *string
    MinSeverity, ReportProfile, Language           *string
    Insecure                                       bool
}

type Source struct {
    File   string                // path of the TOML file
    Getenv func(string) string
    Flags  Flags
}

// Loaded is a valid configuration and the token, which is kept out of
// Config so that nothing that stores or prints a Config can hold it.
type Loaded struct {
    Config Config
    Token  string
}

// Load reads the file, then the environment, then the flags, and
// validates the result.
func Load(src Source) (Loaded, error)

// Validate checks every rule of spec 5.2 and returns the first failure,
// in the order of the table of rules.
func (c Config) Validate() error

// Duration returns the length of analysis.period.
func (a Analysis) Duration() (time.Duration, error)

// Location returns the zone of analysis.timezone, from the data embedded
// in the binary.
func (a Analysis) Location() (*time.Location, error)

// RenderOptions are the settings that --from-snapshot reads: no file, the
// environment, then the flags. A value left empty means "as stored in the
// snapshot" for Profile and MinSeverity.
type RenderOptions struct {
    Language        string
    Profile         string
    MinSeverity     string
    OutputDirectory string
}

func LoadRenderOptions(getenv func(string) string, flags Flags) (RenderOptions, error)
```

**Requirements:**

- Precedence: defaults, then the file, then the environment, then the flags.
  The later one wins, key by key.
- The keys, types, and defaults are exactly the table of spec 5.2. The key
  `report.graphite_package` of the original does not exist.
- Environment: `KCR_METRICS_URL`, `KCR_ENDPOINT_TYPE`, `KCR_CLUSTER`,
  `KCR_CLUSTER_LABEL`, `KCR_CLUSTER_FILTER`, `KCR_NAMESPACES`, `KCR_PERIOD`,
  `KCR_TIMEZONE`, `KCR_NODE_AUTOSCALING`, `KCR_MIN_SEVERITY`,
  `KCR_REPORT_PROFILE`, `KCR_OUTPUT_DIRECTORY`, `KCR_LANGUAGE`. A variable
  that is empty counts as not set.
- `KCR_NAMESPACES` and `Flags.Namespaces` are split on commas. Each item is
  trimmed, an empty item is dropped, the order is kept, and a repeated name
  is kept once, at its first place. Dropping the repeat is a difference from
  the original, which kept it and asked the same question twice.
- `Flags.Insecure` sets `metrics.tls_verify` to false. Nothing on the command
  line sets it to true.
- The token: `Flags.Token` when given and not empty; otherwise the value of
  the variable named by `metrics.token_env`; otherwise empty. When
  `metrics.token_env` is not empty and the variable is unset or empty, and
  `Flags.Token` was not given, `Load` fails (R8).
- `analysis.timezone` is resolved from `time/tzdata`, which `config` imports
  for its effect, so that the binary carries the zones.
- Every message is a fixed English sentence. No message repeats the value of
  `metrics.url`, of the token, or of any environment variable.

Messages, in the order in which `Validate` and `Load` check:

| Rule | Message |
|---|---|
| the file cannot be read | `configuration file cannot be read: <path>: <cause>` |
| the file is not valid TOML | `configuration file is not valid TOML: line <n>, column <n>: <cause>` |
| a value has the wrong type | `configuration file has a wrong type at <key>: line <n>: <cause>` |
| an unknown section | `unknown configuration sections: <names, sorted, joined by ", ">` |
| a section that is not a table | `<section> must be a TOML table` |
| `metrics.token` in the file | `metrics.token is forbidden; use metrics.token_env` |
| an unknown key in a section | `unknown <section> keys: <names, sorted, joined by ", ">` |
| `metrics.url` missing | `metrics.url is required` |
| `metrics.url` not absolute `http` or `https` | `metrics.url must be an absolute HTTP or HTTPS URL` |
| `metrics.url` with user info | `metrics.url must not embed credentials` |
| `metrics.url` with a query | `metrics.url must not include a query string` |
| `metrics.url` with a fragment | `metrics.url must not include a fragment` |
| `metrics.endpoint_type` | `metrics.endpoint_type must be auto, prometheus, or grafana` |
| `grafana` and the path holds neither `datasource` nor `proxy` | `metrics.url must identify a Grafana datasource proxy API root` |
| the token variable is unset or empty | `metrics.token_env names <name>, which is unset or empty` |
| `metrics.ca_bundle` set and not a file | `metrics.ca_bundle must be an existing file` |
| `metrics.timeout_seconds` not above zero | `metrics.timeout_seconds must be positive` |
| `cluster.name` blank | `cluster.name is required` |
| `cluster.namespaces` empty or with a blank item | `cluster.namespaces must contain at least one non-empty namespace` |
| `cluster.label` | `cluster.label must be a valid Prometheus label name` |
| `cluster.filter_mode` | `cluster.filter_mode must be required, auto, or none` |
| `cluster.nodepool_label` set and invalid | `cluster.nodepool_label must be a valid Prometheus label name` |
| `analysis.period` | `analysis.period must look like 30m, 4h, 24h, or 7d` |
| `analysis.timezone` unknown | `analysis.timezone is unknown` |
| `analysis.node_autoscaling` | `analysis.node_autoscaling must be auto, enabled, or disabled` |
| a percent outside 0 to 100 | `threshold values must be within 0..100` |
| `thresholds.bursty_gap_percent` below zero | `thresholds.bursty_gap_percent must not be negative` |
| `thresholds.hpa_replica_divergence` below zero | `thresholds.hpa_replica_divergence must not be negative` |
| warning not below critical | `thresholds.warning_percent must be lower than critical_percent` |
| `report.profile` | `report.profile must be detailed or executive` |
| `report.min_severity` | `report.min_severity must be all, attention, or critical` |
| `report.language` | `report.language must be pt-BR or en` |
| `report.output_directory` | `report.output_directory must exist and be writable` |
| the probe file of that check could not be closed or removed | `report.output_directory holds a probe file that could not be removed: <path>: <cause>` |

A valid label name matches `^[A-Za-z_][A-Za-z0-9_]*$`. A valid period matches
`^[1-9][0-9]*[mhd]$`.

**Tests:**

| Test | Input | Expected |
|---|---|---|
| `TestDefaultMatchesSpec` | `Default()` | each default of spec 5.2, field by field |
| `TestLoadReadsEveryKey` | `testdata/full.toml`, which sets every key to a value that is not its default | each field holds the value of the file |
| `TestLoadMinimalFile` | `testdata/minimal.toml`: `metrics.url`, `cluster.name`, `cluster.namespaces` | valid; every other field is its default |
| `TestLoadEnvironmentOverridesFile` (table) | each of the 13 variables in turn, over `full.toml` | that field holds the value of the variable, and no other field changed |
| `TestLoadFlagOverridesEnvironment` (table) | each flag in turn, with its variable also set | the value of the flag |
| `TestLoadEmptyVariableIsNotSet` | `KCR_CLUSTER=""` | the value of the file |
| `TestLoadSplitsNamespaces` (table) | `a,b`; ` a , b `; `a,,b`; `a,b,a`; `,` | `[a b]`; `[a b]`; `[a b]`; `[a b]`; the namespaces message |
| `TestLoadInsecureTurnsVerificationOff` | `Insecure: true` over a file with `tls_verify = true` | false |
| `TestLoadTokenFromVariable` | `token_env = "KCR_TOKEN"`, variable `abc` | `Token == "abc"` |
| `TestLoadTokenFlagWins` | the same, and `Flags.Token = "xyz"` | `xyz` |
| `TestLoadTokenVariableUnset` | `token_env = "KCR_TOKEN"`, variable unset | the message of R8, naming `KCR_TOKEN` |
| `TestLoadTokenVariableUnsetWithFlag` | the same, and `Flags.Token = "xyz"` | valid, `xyz` |
| `TestLoadNoTokenAtAll` | no `token_env`, no flag | valid, empty token |
| `TestLoadRejectsTokenInFile` | `metrics.token = "abc"` | the forbidden message, and the text holds no `abc` |
| `TestLoadRejectsUnknownSection` | `[extra]` | `unknown configuration sections: extra` |
| `TestLoadRejectsUnknownKey` | `metrics.colour = 1` | `unknown metrics keys: colour` |
| `TestLoadRejectsOldPackageKey` | `report.graphite_package = "x"` | `unknown report keys: graphite_package` |
| `TestLoadReportsLineAndColumn` | `testdata/broken.toml`, broken on line 3 | the message holds `line 3` and a column above zero |
| `TestLoadReportsWrongType` | `metrics.timeout_seconds = "fast"` | the message names `metrics.timeout_seconds` |
| `TestLoadMissingFile` | a path of no file | the cannot-be-read message, naming the path |
| `TestValidate` (table) | one row for each rule above, each from a valid configuration with one field changed | exactly the message of that rule |
| `TestValidateReportsFirstFailure` | two fields wrong | the message of the one that comes first in the table |
| `TestValidateAcceptsBoundaries` (table) | percent 0 and 100; period `1m`; timeout `0.001` | valid |
| `TestValidateGrafanaPath` (table) | `grafana` with `/api/datasources/uid/x`, with `/myproxything`, with `/prom` | valid; valid; the Grafana message |
| `TestMessagesNeverRepeatTheAddress` | every rule of `metrics.url`, with `https://user:hunter2@h.example/?token=abc#f` | no message holds `hunter2`, `abc`, or `h.example` |
| `TestLocationUsesEmbeddedZones` | `America/Sao_Paulo`, with `ZONEINFO` set to a folder that does not exist | the zone is found |
| `TestDuration` (table) | `30m`, `24h`, `7d` | 30 minutes, 24 hours, 168 hours |
| `TestLoadRenderOptions` (table) | nothing; `KCR_LANGUAGE=en`; flag `Language=pt-BR` over it; `Language=fr` | `pt-BR` with empty profile and severity; `en`; `pt-BR`; the language message |

- [ ] **Step 1:** Write the test files and the files under `testdata/`.
- [ ] **Step 2:** Run `go test ./internal/config/ -v`. Expected: build
  failure.
- [ ] **Step 3:** Write `config.go`, `load.go`, and `validate.go`.
- [ ] **Step 4:** Run `go test -count=1 ./internal/config/`. Expected: pass.
- [ ] **Step 5:** Run `make test` and `make lint`, then commit.

```bash
git add internal/config/
git commit -m "feat(config): load and validate the configuration"
```

**Done when:** the tests pass, and the report states the column that
`TestLoadReportsLineAndColumn` pins and that it was checked against the file
by eye.

---

### Task 5: The six scenarios and the fake server

**Executor:** Main thread for Step 1 and Step 2. Subagent for the rest.

**Reference:** `older/kubernetes-report/tests/fakes/prometheus.py`,
`tests/fixtures/prometheus/`, and `tests/test_end_to_end.py`; section 9 of
`python-preflight.md`.

**Files:**
- Create: `testdata/prometheus/direct-complete.json`,
  `grafana-authenticated.json`, `thanos-multicluster.json`,
  `raw-fallback.json`, `partial-data.json`, `value-edge-cases.json`
- Create: `testdata/prometheus/README.md`
- Create: `internal/promtest/fixture.go`, `scenarios.go`, `fixture_test.go`
- Create: `cmd/fake-prometheus/main.go`
- Create: `internal/guard/binary_test.go`

**Interfaces:**
- Consumes: nothing of this module.
- Produces:

```go
package promtest

// Request is one scripted answer of a scenario file.
type Request struct {
    Method        string            `json:"method"`
    Path          string            `json:"path"`
    QueryContains string            `json:"query_contains"`
    Authorization string            `json:"authorization"`
    Status        int               `json:"status"`
    Headers       map[string]string `json:"headers"`
    Body          json.RawMessage   `json:"body"`
}

// Fixture is a scenario file.
type Fixture struct {
    Requests []Request `json:"requests"`
}

// Load reads and checks a scenario file. An unknown key is an error.
func Load(path string) (Fixture, error)

// Seen is one request the server received.
type Seen struct {
    Method, Path, Query, Authorization string
    Form url.Values
    Matched bool
}

// Server answers from a copy of the fixture. Each scripted request answers
// once. It is safe for use by several goroutines.
type Server struct{ /* unexported */ }

func NewServer(f Fixture) *Server
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request)
func (s *Server) Seen() []Seen
func (s *Server) Remaining() int

// Scenario names one of the six scenarios and the settings the original
// tests ran it with.
type Scenario struct {
    Name         string // file stem
    APIRoot      string // path to add to the server address
    EndpointType string
    Token        string // empty when the scenario needs none
    // NodeAutoscaling is the value of analysis.node_autoscaling the
    // original tests ran the scenario with.
    NodeAutoscaling string
}

var Scenarios = []Scenario{ /* the six, in the order of the table below */ }

// Path returns the scenario file of name under testdata/prometheus, found
// by walking up from the working directory to the folder that holds go.mod.
func Path(name string) (string, error)
```

| Scenario | API root | Endpoint type | Token | Node autoscaling |
|---|---|---|---|---|
| `direct-complete` | `/api/v1` | `prometheus` | none | `auto` |
| `grafana-authenticated` | `/api/datasources/uid/prom/resources/api/v1` | `grafana` | `fixture-token` | `auto` |
| `thanos-multicluster` | `/api/v1` | `prometheus` | none | `auto` |
| `raw-fallback` | `/api/v1` | `prometheus` | none | `auto` |
| `partial-data` | `/api/v1` | `prometheus` | none | `auto` |
| `value-edge-cases` | `/api/v1` | `prometheus` | none | `disabled` |

The file `value-edge-cases.json` of the original is byte for byte the file
`direct-complete.json` (F-40). The original ran it with node autoscaling
`disabled`, and that setting is all that tells the two scenarios apart.

Every scenario runs with cluster `prod`, namespaces `payments`, filter mode
`required`, period `24h`, zone `America/Sao_Paulo`, and the clock at
2026-07-15 14:30 in that zone.

**Requirements:**

- The six files are copied byte for byte. `README.md` records the source
  path, the version `1.11.0`, and the SHA-256 of each file.
- A scripted request matches when the method is equal (a missing method
  means `GET`), the path is equal, `query_contains` is empty or is part of
  the `query` field of the form, and `authorization` is empty or equals the
  header.
- The first scripted request that matches answers and is removed. The order
  of arrival does not matter.
- No match: status 400 with the body
  `{"status":"error","errorType":"fixture","error":"<message>"}`. The message
  is `unexpected request` when nothing is left, `expected query for "<path>"`
  when something is left for that path, and `unexpected path "<path>"`
  otherwise.
- `fake-prometheus --fixture <file> --listen <address>` serves the file and
  prints `listening on http://<address>` on the first line of standard
  output. The default address is `127.0.0.1:9090`. On `SIGINT` or `SIGTERM`
  it prints `remaining: <n>` and stops with code 0.
- The production binary never holds `internal/promtest`.

**Tests:**

| Test | Input | Expected |
|---|---|---|
| `TestLoadReadsEachScenario` (table) | the six files | 70, 70, 70, 73, 63, 70 requests, in the order of the table above |
| `TestLoadRejectsUnknownKey` | a file with `"delay": 1` in a request | error naming `delay` |
| `TestLoadRejectsFileWithoutRequests` | `{}` | error |
| `TestServerMatchesOnFormQuery` | `POST` with form `query=vector(1)` | the scripted body, status 200 |
| `TestServerIgnoresQueryInAddress` | `POST /api/v1/query?query=vector(1)` with an empty form | status 400: the query is read from the form only |
| `TestServerAnswersEachRequestOnce` | the same request twice, scripted once | 200, then 400 |
| `TestServerMatchesOutOfOrder` | the second scripted request first | 200, and `Remaining()` fell by one |
| `TestServerRequiresAuthorization` | `grafana-authenticated`, no header | 400 |
| `TestServerErrorMessages` (table) | nothing left; a known path with another query; an unknown path | the three messages |
| `TestServerRecordsWhatItSaw` | two requests | `Seen()` holds both, in order, with `Matched` true and false |
| `TestServerIsSafeForConcurrentUse` | 50 goroutines, run with `-race` | no race, and `Remaining()` is right |
| `TestScenariosMatchTheFiles` | `Scenarios` | one entry for each file of the folder and no other |
| `TestProductionBinaryHoldsNoTestSupport` in `internal/guard` | `go list -deps ./cmd/k8s-capacity-report` | no package under `internal/promtest`, `internal/goldentest`, `cmd/fake-prometheus`, or `cmd/update-testdata` |
| `TestFakePrometheusBuilds` in `internal/guard` | `go build -o <temp> ./cmd/fake-prometheus` | exit 0 |

- [ ] **Step 1 (main thread):** Read the six files for a real host name, a
  real cluster or organisation name, and a real credential. The reader
  found none; confirm it before anything enters Git.

```bash
grep -h -o -E '"(instance|node|internal_ip|provider_id|cluster|namespace)": *"[^"]*"' \
  older/kubernetes-report/tests/fixtures/prometheus/*.json | sort | uniq -c | sort -rn
grep -n -i -E 'bearer|token|password|secret' \
  older/kubernetes-report/tests/fixtures/prometheus/*.json
```

Expected: only invented names, and `Bearer fixture-token` as the only
credential.

- [ ] **Step 2 (main thread):** Copy the six files and write `README.md`
  with their digests.

```bash
mkdir -p testdata/prometheus
cp older/kubernetes-report/tests/fixtures/prometheus/*.json testdata/prometheus/
shasum -a 256 testdata/prometheus/*.json
```

- [ ] **Step 3:** Write the tests.
- [ ] **Step 4:** Run `go test ./internal/promtest/ -v`. Expected: build
  failure.
- [ ] **Step 5:** Write `fixture.go`, `scenarios.go`, and
  `cmd/fake-prometheus/main.go`.
- [ ] **Step 6:** Run `go test -count=1 -race ./internal/promtest/ ./internal/guard/`.
  Expected: pass.
- [ ] **Step 7:** See the guard fail. Add
  `import _ "github.com/marceloalvescf/k8s-capacity-report/internal/promtest"`
  to `cmd/k8s-capacity-report/main.go`, run
  `go test ./internal/guard/ -run TestProductionBinaryHoldsNoTestSupport`,
  and confirm it fails naming the package. Remove the line.
- [ ] **Step 8:** Run `make test` and `make lint`, then commit.

```bash
git add testdata/prometheus/ internal/promtest/ cmd/fake-prometheus/ \
  internal/guard/binary_test.go
git commit -m "test(prom): carry over the six scenarios"
```

**Done when:** the tests pass, the guard was seen failing, and the report of
Step 1 is in `.cache/phase-2/part-1/fixture-review.md`.

---

### Task 6: The Prometheus client

**Executor:** Subagent.

**Reference:** `older/kubernetes-report/kube_capacity_report/metrics/client.py`,
`endpoint.py`, `selectors.py`, and their tests; sections 3, 4, and 6 of
`python-preflight.md`; spec 6.1.

**Files:**
- Create: `internal/prom/endpoint.go`, `client.go`, `result.go`,
  `selector.go`
- Create: `internal/prom/endpoint_test.go`, `client_test.go`,
  `result_test.go`, `selector_test.go`, `scenario_test.go`

**Interfaces:**
- Consumes: `config.Metrics`; `promtest` in tests only.
- Produces:

```go
package prom

// Kind says which class of failure an Error is.
type Kind int

const (
    KindMetrics Kind = iota // the endpoint failed or answered badly
    KindAuthentication      // HTTP 401
    KindAuthorization       // HTTP 403
    KindTLS
    KindEndpointNotFound    // no candidate answered as a Prometheus API
    KindQuery               // the API refused or garbled one query
)

type Error struct {
    Kind    Kind
    Status  int    // zero when there was no answer
    Message string
}

func (e *Error) Error() string // the message

// Candidates returns the API roots to try, in order, and the endpoint
// kind, "prometheus" or "grafana". The address was validated by config.
func Candidates(address, endpointType string) (roots []string, kind string)

type Options struct {
    Metrics   config.Metrics
    Token     string
    // Sleep waits between attempts. Nil means a real wait that ends early
    // when the context ends.
    Sleep     func(ctx context.Context, d time.Duration) error
    // Transport replaces the transport built from Metrics. Phase 3 uses it
    // for the outbound policy. Nil means the built one.
    Transport http.RoundTripper
}

type Client struct{ /* unexported */ }

// Connect tries each candidate root with the probe `vector(1)` and returns
// a client bound to the first root that answers as a Prometheus API.
func Connect(ctx context.Context, opts Options) (*Client, error)

// Root returns the API root that answered. Kind returns the endpoint kind.
func (c *Client) Root() string
func (c *Client) Kind() string

// Sample is one series of an instant answer. Valid is false when the
// value was NaN or infinite, which means "no data".
type Sample struct {
    Labels map[string]string
    Value  float64
    Valid  bool
}

// Query asks an instant query at the given time.
func (c *Client) Query(ctx context.Context, query string, at time.Time) ([]Sample, error)

// Selector builds one PromQL selector.
type Selector struct {
    Metric string
    Equal  map[string]string
}

func (s Selector) String() string

// Quote escapes a value for a PromQL string, without the quotes around it.
func Quote(value string) string
```

**Requirements:**

- The request: `POST` to `<root>/query`, body form-encoded, fields `query`
  and `time`. `time` is the Unix time in whole seconds. The probe sends no
  `time`. Headers: `Accept: application/json`, and
  `Authorization: Bearer <token>` when the token is not empty.
- The address of the request never holds a query string.
- Transport: no proxy, whatever the environment holds; no redirect is
  followed, and the redirect answer itself is handled by its status; the
  timeout of `Metrics.TimeoutSeconds` covers one attempt, headers and body.
- TLS: `Metrics.CABundle`, when set, is the only source of trusted roots.
  Otherwise the system roots are used. `Metrics.TLSVerify` false turns the
  check off.
- Attempts: three. A connection failure or a timeout waits `1s` after the
  first attempt and `2s` after the second. Status 429 and status 500 or
  above wait the same, unless `Retry-After` holds only digits, which then
  gives the wait in seconds. No wait is longer than 60 seconds. There is no
  wait after the last attempt. Nothing else is tried again.
- An error of `Options.Transport` that is not a connection failure, a
  timeout, or a TLS failure is returned as it is, after one call.

Roots:

| Address | Type | Roots | Kind |
|---|---|---|---|
| `https://p.example` | `prometheus` | `https://p.example`, `https://p.example/api/v1` | `prometheus` |
| `https://p.example/` | `auto` | the same two | `prometheus` |
| `https://p.example/api/v1` | `auto` | `https://p.example/api/v1` | `prometheus` |
| `https://p.example/api/v1/` | `auto` | `https://p.example/api/v1` | `prometheus` |
| `https://g.example/api/datasources/uid/x/resources/api/v1` | `auto` | that address alone | `grafana` |
| `https://g.example/api/datasources/proxy/1` | `auto` | that address, and that address plus `/api/v1` | `grafana` |
| `https://g.example/myproxything` | `auto` | that address, and that address plus `/api/v1` | `prometheus` |
| `https://g.example/myproxything` | `grafana` | the same two | `grafana` |

The second root is left out when the path ends in `/api/v1` or holds
`/api/v1/`.

Failures. "Before" means during `Connect`, until the probe succeeds. "After"
means in `Query`.

| What happened | Before | After |
|---|---|---|
| TLS failure | `KindTLS`, `TLS validation failed` | the same |
| connection failure or timeout, three times | `KindMetrics`, `could not connect to the metrics endpoint` | the same text plus ` mid-collection` |
| status 401 | `KindAuthentication`, `metrics endpoint rejected authentication` | the same |
| status 403 | `KindAuthorization`, `metrics endpoint rejected authorization` | the same |
| status 404 | next candidate | `KindMetrics`, `Prometheus API path was not found mid-collection` |
| status 405 | next candidate | `KindMetrics`, `Prometheus API path rejected the query method mid-collection` |
| status 429, or 500 and above, three times | `KindMetrics`, `metrics endpoint returned HTTP <status>` | the same |
| any other status that is not 200 to 299 | `KindQuery`, `metrics endpoint returned HTTP <status>` | the same |
| the body is not JSON, is not an object, or has no `status` | next candidate | `KindMetrics`, `metrics endpoint did not answer the Prometheus API mid-collection` |
| `status` is not `success` | `KindQuery`, `Prometheus query failed with type <errorType, or unknown>` | the same |
| no `data` object | `KindQuery`, `Prometheus response has no data object` | the same |
| `resultType` is not `vector` | `KindQuery`, `unsupported Prometheus result type: <type>` | the same |
| a result item is not an object | `KindQuery`, `Prometheus response contains an invalid result item` | the same |
| `metric` is not an object of strings | `KindQuery`, `Prometheus response contains invalid labels` | the same |
| an item has no `value` | `KindQuery`, `Prometheus vector result has no value` | the same |
| `value` is not a pair, or a member cannot be read | `KindQuery`, `Prometheus response contains an invalid sample` | the same |
| every candidate was the wrong one | `KindEndpointNotFound`, `no candidate endpoint answered the Prometheus API` | does not apply |

`matrix` answers are refused as an unsupported result type in this part.
Part 2.2 adds them with `QueryRange`.

Selectors: the matchers are written in the order of their label names,
whatever the order of insertion. `Quote` doubles each backslash first, then
turns a line break into `\n`, then puts a backslash before each double
quote.

**Tests.** Unless a row says otherwise, the server is `httptest.NewServer`
and `Sleep` records the waits and returns at once.

| Test | Input | Expected |
|---|---|---|
| `TestCandidates` (table) | each row of the table of roots | the roots and the kind |
| `TestConnectSendsProbeAsForm` | a server that records | one `POST` to `/query`; body `query=vector%281%29`; no `time`; no `?` in the address; `Accept: application/json` |
| `TestConnectSendsBearerToken` | token `abc` | `Authorization: Bearer abc` |
| `TestConnectSendsNoAuthorizationWithoutToken` | empty token | the header is absent |
| `TestConnectFallsToSecondCandidate` (table) | the first root answers 404; 405; 200 with `<html>`; 200 with `[]`; 200 with `{"data":{}}` | the client is bound to the second root |
| `TestConnectStopsOnOtherErrors` (table) | the first root answers 401; 403; 302; 418 | the kind and message of the table; the second root received nothing |
| `TestConnectReportsNoCandidate` | both roots answer 404 | `KindEndpointNotFound` and its message |
| `TestConnectNamesAnAnswerThatIsNotPrometheus` | both roots answer 200 with an HTML sign-in page | the message is exactly `no candidate endpoint answered the Prometheus API`, and holds no `invalid character` |
| `TestQueryAfterConnectIsMidCollection` (table) | after a good probe: 404; 405; `<html>` | `KindMetrics` and the three texts ending in ` mid-collection` |
| `TestQuerySendsTimeInSeconds` | `at` = 2026-07-15 14:30:00.7 -03:00 | `time=1784136600` |
| `TestQueryNeverFollowsRedirect` | 302 to a second server | `KindQuery`, `metrics endpoint returned HTTP 302`; the second server received nothing |
| `TestRetryOnServerError` | 500, 500, 200 | success; waits `[1s 2s]` |
| `TestRetryGivesUpAfterThree` | 503 four times | `metrics endpoint returned HTTP 503`; three requests; waits `[1s 2s]` |
| `TestRetryHonoursRetryAfter` | 429 with `Retry-After: 7`, then 200 | waits `[7s]` |
| `TestRetryCapsRetryAfter` | 429 with `Retry-After: 3600`, then 200 | waits `[60s]` |
| `TestRetryIgnoresOtherRetryAfter` (table) | `Retry-After` of `1.5`, `-1`, `Wed, 21 Oct 2026 07:28:00 GMT` | waits `[1s]` |
| `TestRetryOnConnectionFailure` | a listener that closes each connection | `could not connect to the metrics endpoint`; waits `[1s 2s]` |
| `TestNoRetryOnClientError` (table) | 400, 401, 403, 404 | one request, no wait |
| `TestSleepEndsWithContext` | the real wait, a context of 50 ms, a server that answers 500 | an error within 1 second that wraps `context.DeadlineExceeded` |
| `TestTimeoutCoversTheBody` | the server sends headers and then waits 2 s; timeout 0.2 s | a connection failure, not a hang |
| `TestTLSRejectsUnknownAuthority` | `httptest.NewTLSServer`, no bundle | `KindTLS`, `TLS validation failed` |
| `TestTLSAcceptsBundle` | the same, with the certificate of the server written to a file as the bundle | success |
| `TestTLSInsecureSkipsCheck` | the same, no bundle, `TLSVerify` false | success |
| `TestTransportUsesNoProxy` | inside the package: the transport built from any `Metrics` | its `Proxy` field is nil |
| `TestForeignTransportErrorIsReturnedOnce` | `Options.Transport` that fails with its own error type | that same error, by `errors.Is`; one call; no wait |
| `TestQueryReadsSamples` | a vector of two items with labels | two samples, labels and values equal |
| `TestQueryTreatsNonFiniteAsNoData` (table) | value `NaN`, `+Inf`, `-Inf`, `Inf` | `Valid` false |
| `TestQueryKeepsMeasuredZero` | value `0` | `Valid` true, `Value` 0 |
| `TestQueryFailures` (table) | one body for each row of the table of failures from `status` down | the kind and the message |
| `TestSelectorSortsMatchers` | metric `m`, `resource=cpu` inserted before `cluster=prod` | `m{cluster="prod",resource="cpu"}` |
| `TestSelectorWithoutMatchers` | metric `m` | `m` |
| `TestQuote` (table) | `prod"west\one`; a value with a line break; `a\"b` | `prod\"west\\one`; `\n` in place of the break; `a\\\"b` |
| `TestScenarioProbe` (table) | each entry of `promtest.Scenarios`, served by `promtest.NewServer`, with its API root and token | `Connect` succeeds, `Kind()` is the type of the scenario, and exactly one scripted request was used |
| `TestScenarioGrafanaNeedsToken` | `grafana-authenticated` with no token | an error, and no success was recorded |

- [ ] **Step 1:** Write the tests.
- [ ] **Step 2:** Run `go test ./internal/prom/ -v`. Expected: build failure.
- [ ] **Step 3:** Write the four files.
- [ ] **Step 4:** Run `go test -count=1 -race ./internal/prom/`. Expected:
  pass.
- [ ] **Step 5:** Run `make test` and `make lint`, then commit.

```bash
git add internal/prom/
git commit -m "feat(prom): add the Prometheus client"
```

**Done when:** the tests pass, and the report says how the value of `time`
in `TestQuerySendsTimeInSeconds` was worked out.

---

### Task 7: Cluster scope

**Executor:** Subagent.

**Reference:**
`older/kubernetes-report/kube_capacity_report/metrics/capabilities.py:48-112`
and `tests/test_capabilities.py:44-97`; section 6 of `python-preflight.md`;
spec 6.2.

**Files:**
- Create: `internal/discovery/registry.go`, `scope.go`, `scope_test.go`

**Interfaces:**
- Consumes: `prom.Sample`, `config.Cluster`.
- Produces:

```go
package discovery

// Querier is the part of the client that discovery needs.
type Querier interface {
    Query(ctx context.Context, query string, at time.Time) ([]prom.Sample, error)
}

const (
    ConfidenceExact    = "exact"
    ConfidenceInferred = "inferred"
    ConfidenceUnknown  = "unknown"
)

type Scope struct {
    ClusterName      string
    ClusterLabel     string
    FilterMode       string
    MatcherEnabled   bool
    Confidence       string
    DiscoveredValues []string // sorted, never nil
}

// Matchers returns the label and value to add to a selector: one entry
// when the matcher is on, none when it is off.
func (s Scope) Matchers() map[string]string

// ScopeError is a cluster scope failure. The command maps it to exit
// code 4.
type ScopeError struct{ Message string }

func (e *ScopeError) Error() string

func ResolveScope(ctx context.Context, q Querier, cluster config.Cluster, at time.Time) (Scope, error)
```

**Requirements:**

- First query: `count by (<label>) (kube_node_info)`. The values are the
  distinct values of the label that are not empty, sorted. "Unlabelled" is
  true when any series has no value for the label.
- Second query, only when the first gave no value:
  `count by (<label>) (up)`. Its result replaces the first one whole, values
  and "unlabelled" alike. The two results are never mixed.
- An error of the client is returned as it is.

| Mode | Condition | Result |
|---|---|---|
| `required` | the name is among the values | matcher on, `exact` |
| `required` | the name is not among the values | `cluster "<name>" was not found in label "<label>"; visible values: <values joined by ", ", or none>` |
| `none` | values exist and "unlabelled" is true | `labeled and unlabeled cluster groups are visible while filter mode is none` |
| `none` | more than one value | `multiple cluster values are visible while filter mode is none` |
| `none` | otherwise | matcher off, `exact` |
| `auto` | the name is among the values | matcher on, `exact` |
| `auto` | values exist, the name is not among them | `cluster "<name>" was not found in visible label values: <values joined by ", ">` |
| `auto` | no value | matcher off, `inferred`, no discovered value |

**Tests.** The querier is a fake that answers by query text and records the
queries.

| Test | Input | Expected |
|---|---|---|
| `TestScope` (table) | one row for each row of the table above | the scope, or a `*ScopeError` with exactly that message |
| `TestScopeAsksWithTheConfiguredLabel` | label `k8s_cluster` | the query is `count by (k8s_cluster) (kube_node_info)` |
| `TestScopeFallsBackToUp` | the first query gives nothing, the second gives `prod` | two queries; values `[prod]` |
| `TestScopeDoesNotFallBackWhenValuesExist` | the first query gives `dev` | one query |
| `TestScopeFallbackReplacesTheFirstResult` | the first query gives one series with no label; the second gives `prod` with no unlabelled series; mode `none` | matcher off, `exact`, values `[prod]`; no error about unlabelled groups |
| `TestScopeSortsAndDeduplicatesValues` | series `prod`, `dev`, `prod` | `[dev prod]` |
| `TestScopeReturnsClientError` | the querier fails with a `*prom.Error` | that error, by `errors.As` |
| `TestScopePassesTheTime` | `at` | both queries received `at` |
| `TestMatchers` (table) | matcher on; matcher off | one entry `label: name`; an empty map that is not nil |
| `TestScopeScenario` (table) | each entry of `promtest.Scenarios`, through `prom.Connect`, mode `required` | matcher on, `exact`; `thanos-multicluster` has values `[dev prod]`, the others `[prod]` |
| `TestScopeScenarioMultipleClusters` | `thanos-multicluster`, mode `none` | the message about multiple values |

- [ ] **Step 1:** Write the tests.
- [ ] **Step 2:** Run `go test ./internal/discovery/ -v`. Expected: build
  failure.
- [ ] **Step 3:** Write `registry.go` with the three confidences, and
  `scope.go`.
- [ ] **Step 4:** Run `go test -count=1 ./internal/discovery/`. Expected:
  pass.
- [ ] **Step 5:** Run `make test` and `make lint`, then commit.

```bash
git add internal/discovery/
git commit -m "feat(discovery): resolve the cluster scope"
```

**Done when:** the tests pass.

---

### Task 8: The nine capabilities

**Executor:** Subagent.

**Reference:**
`older/kubernetes-report/kube_capacity_report/metrics/capabilities.py:144-455`,
`metrics/names.py`, `collection.py:67-74` and `:198-207`, and
`tests/test_capabilities.py`; section 7 of `python-preflight.md`; spec 6.3.

**Files:**
- Modify: `internal/discovery/registry.go`
- Create: `internal/discovery/capabilities.go`, `capabilities_test.go`

**Interfaces:**
- Consumes: `Querier`, `Scope`, `prom.Selector`, `config.Analysis`.
- Produces:

```go
const (
    StateAvailable   = "available"
    StatePartial     = "partial"
    StateUnavailable = "unavailable"

    StrategyRecordingRule = "recording_rule"
    StrategyRawMetrics    = "raw_metrics"
    StrategyConfigured    = "configured"
    StrategyNone          = "none"
)

// Names lists the nine capabilities in their fixed order.
var Names = []string{
    "namespace_usage", "workload_usage", "requests_limits", "hpa_history",
    "workloads_without_hpa", "ip_node_mapping", "nodepool_discovery",
    "node_utilization", "node_autoscaling",
}

var States = []string{StateAvailable, StatePartial, StateUnavailable}

var Strategies = []string{
    StrategyRecordingRule, StrategyRawMetrics, StrategyConfigured, StrategyNone,
}

// ReasonCodes lists every reason a capability can carry.
var ReasonCodes = []string{
    "METRICS_NOT_FOUND",
    "CPU_USAGE_METRIC_NOT_FOUND",
    "MEMORY_USAGE_METRIC_NOT_FOUND",
    "USAGE_METRICS_NOT_FOUND",
    "WORKLOAD_HISTORY_INEXACT",
    "WORKLOAD_ASSOCIATION_UNAVAILABLE",
    "REQUESTS_LIMITS_INCOMPLETE",
    "LEGACY_METRICS_PARTIAL",
    "HPA_HISTORY_INCOMPLETE",
    "WORKLOAD_INVENTORY_INCOMPLETE",
    "NODE_ADDRESS_MAPPING_INCOMPLETE",
    "NODEPOOL_LABELS_NOT_FOUND",
    "NODE_AUTOSCALING_UNDETERMINED",
}

type Capability struct {
    Name     string
    State    string
    Strategy string
    Evidence []string // never nil
    Reasons  []string // reason codes, never nil
}

// Detect returns the nine capabilities, in the order of Names.
func Detect(ctx context.Context, q Querier, scope Scope, analysis config.Analysis, at time.Time) ([]Capability, error)

// ErrInsufficient is returned by CheckSufficient. The command maps it to
// exit code 5.
var ErrInsufficient = errors.New("all analytical capabilities are unavailable")

func CheckSufficient(capabilities []Capability) error
```

**Requirements:**

- A metric is present when `count(<selector>)` gives a sample that is valid
  and above zero. The selector carries the matchers of the scope.
- The probes are the ones of the original, asked in the same order and with
  the same short cuts: a probe that the original does not ask is not asked.
  The scenario files depend on it, because a scripted request answers the
  first query that holds its text.
- The probe of `node_autoscaling` is
  `count({__name__=~"cluster_autoscaler_.+"})`, with the matcher of the scope
  added after the name matcher when it is on. It is asked in every case,
  also when the configuration decides the answer.
- The reason sentences of the original become codes:

| Sentence of the original | Code |
|---|---|
| As métricas necessárias não foram encontradas. | `METRICS_NOT_FOUND` |
| A métrica de uso de CPU não foi encontrada. | `CPU_USAGE_METRIC_NOT_FOUND` |
| A métrica de uso de memória não foi encontrada. | `MEMORY_USAGE_METRIC_NOT_FOUND` |
| As métricas de uso de CPU e memória não foram encontradas. | `USAGE_METRICS_NOT_FOUND` |
| A associação histórica entre pods e cargas de trabalho não pôde ser reconstruída com exatidão. | `WORKLOAD_HISTORY_INEXACT` |
| A associação por carga de trabalho não está disponível; a cobertura está limitada a pods. | `WORKLOAD_ASSOCIATION_UNAVAILABLE` |
| A cobertura de solicitações e limites está incompleta para CPU ou memória. | `REQUESTS_LIMITS_INCOMPLETE` |
| As métricas legadas oferecem cobertura parcial de recursos. | `LEGACY_METRICS_PARTIAL` |
| A cobertura histórica dos HPAs está incompleta. | `HPA_HISTORY_INCOMPLETE` |
| O inventário de cargas de trabalho ou alvos de HPA está incompleto. | `WORKLOAD_INVENTORY_INCOMPLETE` |
| A associação entre endereços e nós pode estar incompleta. | `NODE_ADDRESS_MAPPING_INCOMPLETE` |
| Os rótulos de pools de nós não foram encontrados. | `NODEPOOL_LABELS_NOT_FOUND` |
| O estado do escalonamento automático de nós não pôde ser determinado. | `NODE_AUTOSCALING_UNDETERMINED` |

- Where the original joins two sentences, the capability carries two codes
  in that order. The only case is `workload_usage` with raw ownership and
  one usage metric missing: `WORKLOAD_HISTORY_INEXACT`, then the code of the
  missing metric.
- State and strategy follow section 7 of `python-preflight.md`, case by
  case.
- Evidence (R9). Names are in the order the original declares them.

| Capability | Case | Evidence |
|---|---|---|
| `namespace_usage` | any | the usage metrics found |
| `workload_usage` | recording rule | the rule, then the usage metrics found |
| `workload_usage` | raw ownership | `kube_pod_owner`, then the usage metrics found |
| `workload_usage` | usage, no ownership | the usage metrics found |
| `workload_usage` | no usage | the ownership metrics found |
| `requests_limits` | all four unified | `kube_pod_container_resource_requests`, `kube_pod_container_resource_limits` |
| `requests_limits` | all four legacy | the four legacy names |
| `requests_limits` | partial | the names found, each once |
| `hpa_history` | any | the names found |
| `workloads_without_hpa` | any | the names found, of the five |
| `ip_node_mapping` | available | `kube_node_info` |
| `ip_node_mapping` | partial | the node utilisation names found: the two rules, or the three raw metrics |
| `nodepool_discovery` | available | `kube_node_labels` |
| `nodepool_discovery` | partial | `kube_node_info` |
| `node_utilization` | recording rules | `instance:node_cpu_utilisation:rate5m`, `instance:node_memory_utilisation:ratio` |
| `node_utilization` | raw | `node_cpu_seconds_total`, `node_memory_MemAvailable_bytes`, `node_memory_MemTotal_bytes` |
| `node_autoscaling` | configured | none |
| `node_autoscaling` | metrics found | `cluster_autoscaler_*`, the one entry that is a pattern, because the probe asks for a family of names |
| any | unavailable, not listed above | none |

- `CheckSufficient` follows R16.

**Tests.** The querier is a fake built from the set of metric names that are
present. It records every query.

| Test | Present | Expected |
|---|---|---|
| `TestDetectReturnsNineInOrder` | everything | the names equal `Names` |
| `TestNamespaceUsage` (table) | both; CPU only; memory only; none | available; partial with the memory code; partial with the CPU code; unavailable with `USAGE_METRICS_NOT_FOUND` and strategy `none` |
| `TestWorkloadUsage` (table) | rule and both usage; rule and CPU only; raw owner and both; raw owner and CPU only; usage and no owner; owner and no usage | available, `recording_rule`; partial with the memory code; partial, `raw_metrics`, `WORKLOAD_HISTORY_INEXACT`; partial with two codes in order; partial with `WORKLOAD_ASSOCIATION_UNAVAILABLE`; unavailable with `USAGE_METRICS_NOT_FOUND` |
| `TestRequestsLimits` (table) | four unified; four legacy; three unified; two legacy; none | available; available; partial with `REQUESTS_LIMITS_INCOMPLETE`; partial with `LEGACY_METRICS_PARTIAL`; unavailable with `METRICS_NOT_FOUND` |
| `TestHPAHistory` (table) | five; three; none | available; partial with its code and the three names; unavailable |
| `TestHPAMetricsAreProbedOnceEach` | everything | each of the five names appears in exactly one query |
| `TestWorkloadsWithoutHPA` (table) | labels of both and HPA info; replicas of both and HPA info; deployment labels only; none | available; available; partial with its code; unavailable |
| `TestInventoryShortCut` | `kube_deployment_labels` present | no query holds `kube_deployment_spec_replicas` |
| `TestIPNodeMapping` (table) | node info; no node info and the rules; no node info and raw; none | available; partial with the two rule names; partial with the three raw names; unavailable |
| `TestNodepoolDiscovery` (table) | node labels; node info only; none | available; partial with `kube_node_info`; unavailable |
| `TestNodeUtilization` (table) | both rules; one rule and three raw; three raw; two raw; none | available, `recording_rule`; available, `raw_metrics`; available, `raw_metrics`, three names; unavailable; unavailable |
| `TestNodeUtilizationShortCut` | the CPU rule absent | no query holds the memory rule |
| `TestNodeAutoscaling` (table) | configured `enabled`, metrics absent; configured `disabled`, metrics present; `auto`, present; `auto`, absent | available, `configured`, no evidence; the same; available, `raw_metrics`, `cluster_autoscaler_*`; partial, `none`, `NODE_AUTOSCALING_UNDETERMINED` |
| `TestNodeAutoscalingIsProbedWhenConfigured` | configured `enabled` | one query holds `cluster_autoscaler_` |
| `TestProbesCarryTheMatcher` | scope with matcher on, cluster `prod` | every `count(` query holds `cluster="prod"` |
| `TestProbesEscapeTheMatcher` | cluster `prod"west\one` | every `count(` query holds `cluster="prod\"west\\one"` |
| `TestProbesWithoutMatcher` | matcher off | no query holds `cluster=` |
| `TestProbeSortsMatchers` | matcher on | the query of the CPU requests holds `{cluster="prod",resource="cpu"}` |
| `TestPresentNeedsAValueAboveZero` (table) | the count is `0`; `NaN`; no sample | absent |
| `TestDetectReturnsClientError` | the third query fails | that error, and no further query |
| `TestEveryReasonIsInTheRegistry` | all the tables above | each code given is in `ReasonCodes`, and each code of `ReasonCodes` was given by some row |
| `TestCheckSufficient` (table) | all nine available; the five unavailable and autoscaling partial; the five unavailable and autoscaling available; four unavailable and one partial | nil; `ErrInsufficient`; nil; nil |
| `TestDetectScenario` (table) | each entry of `promtest.Scenarios`, through `prom.Connect` and `ResolveScope` | the table of scenario results below; `Seen()` holds no unmatched request |
| `TestDetectScenarioUsesTheExpectedRequests` (table) | the same | scripted requests used, probe and scope included: 30 for `raw-fallback`, 26 for `partial-data`, 29 for each of the others. The files hold an answer for every probe; the short cuts leave the memory rule unasked in both, and the two raw memory metrics unasked in `partial-data` |

Scenario results. Every capability that is not listed is available.

| Scenario | Differences |
|---|---|
| `direct-complete`, `grafana-authenticated`, `thanos-multicluster` | none; `workload_usage` and `node_utilization` use `recording_rule` |
| `value-edge-cases` | as `direct-complete`, but `node_autoscaling` is available with strategy `configured` and no evidence |
| `raw-fallback` | `workload_usage` partial, `raw_metrics`, `WORKLOAD_HISTORY_INEXACT`; `workloads_without_hpa` available with the two `replicas` names; `node_utilization` available, `raw_metrics`; `node_autoscaling` partial, `NODE_AUTOSCALING_UNDETERMINED` |
| `partial-data` | `hpa_history` unavailable, `METRICS_NOT_FOUND`; `workloads_without_hpa` partial, `WORKLOAD_INVENTORY_INCOMPLETE`; `nodepool_discovery` partial, `NODEPOOL_LABELS_NOT_FOUND`; `node_utilization` unavailable, `METRICS_NOT_FOUND`; `node_autoscaling` partial, `NODE_AUTOSCALING_UNDETERMINED` |

If a scenario gives another result, the subagent stops and reports. It does
not change the expected table: the main thread decides whether the table or
the code is wrong.

- [ ] **Step 1:** Write the tests.
- [ ] **Step 2:** Run `go test ./internal/discovery/ -run 'Detect|Usage|Limits|HPA|Node|Probe|Sufficient|Reason' -v`.
  Expected: build failure.
- [ ] **Step 3:** Write `capabilities.go` and complete `registry.go`.
- [ ] **Step 4:** Run `go test -count=1 ./internal/discovery/`. Expected:
  pass.
- [ ] **Step 5:** Run `make test` and `make lint`, then commit.

```bash
git add internal/discovery/
git commit -m "feat(discovery): detect the nine capabilities"
```

**Done when:** the tests pass with the expected tables unchanged, and F-37
names the commit.

---

### Task 9: The snapshot

**Executor:** Subagent.

**Reference:** spec 8;
`older/kubernetes-report/kube_capacity_report/snapshots/dto_v1.py`, for
comparison only.

**Files:**
- Create: `internal/snapshot/snapshot.go`, `build.go`, `codec.go`
- Create: `internal/snapshot/build_test.go`, `codec_test.go`

**Interfaces:**
- Consumes: `config.Config`, `discovery.Scope`, `discovery.Capability`.
- Produces:

```go
package snapshot

const SchemaVersion = "1.0"

type Snapshot struct {
    SchemaVersion string  `json:"schema_version"`
    Report        Report  `json:"report"`
    Scope         Scope   `json:"scope"`
    Quality       Quality `json:"quality"`
}

type Report struct {
    ID               string     `json:"id"`
    GeneratorVersion string     `json:"generator_version"`
    EmittedAt        string     `json:"emitted_at"`   // RFC 3339, with offset
    Period           string     `json:"period"`
    PeriodStart      string     `json:"period_start"`
    PeriodEnd        string     `json:"period_end"`
    Timezone         string     `json:"timezone"`
    Profile          string     `json:"profile"`
    MinSeverity      string     `json:"min_severity"`
    Organization     string     `json:"organization"`
    Classification   string     `json:"classification"`
    Author           string     `json:"author"`
    Thresholds       Thresholds `json:"thresholds"`
    Settings         Settings   `json:"settings"`
}

type Thresholds struct {
    WarningPercent           float64 `json:"warning_percent"`
    CriticalPercent          float64 `json:"critical_percent"`
    LowUtilizationPercent    float64 `json:"low_utilization_percent"`
    AutoscalerNearMaxPercent float64 `json:"autoscaler_near_max_percent"`
    NodepoolSpreadPoints     float64 `json:"nodepool_spread_points"`
    BurstyGapPercent         float64 `json:"bursty_gap_percent"`
    HPAReplicaDivergence     float64 `json:"hpa_replica_divergence"`
}

type Settings struct {
    EndpointType    string  `json:"endpoint_type"` // the kind that answered
    TLSVerify       bool    `json:"tls_verify"`
    TimeoutSeconds  float64 `json:"timeout_seconds"`
    NodeAutoscaling string  `json:"node_autoscaling"`
    NodepoolLabel   string  `json:"nodepool_label"`
}

type Scope struct {
    ClusterName        string   `json:"cluster_name"`
    ClusterLabel       string   `json:"cluster_label"`
    FilterMode         string   `json:"filter_mode"`
    MatcherEnabled     bool     `json:"matcher_enabled"`
    Confidence         string   `json:"confidence"`
    DiscoveredValues   []string `json:"discovered_values"`
    Namespaces         []string `json:"namespaces"`
    ExcludedContainers []string `json:"excluded_containers"`
}

type Quality struct {
    Capabilities          []Capability `json:"capabilities"`
    CollectionStartedAt   string       `json:"collection_started_at"`
    CollectionCompletedAt string       `json:"collection_completed_at"`
}

type Capability struct {
    Name     string   `json:"name"`
    State    string   `json:"state"`
    Strategy string   `json:"strategy"`
    Evidence []string `json:"evidence"`
    Reasons  []string `json:"reasons"`
}

type Input struct {
    ID                  string
    GeneratorVersion    string
    EmittedAt           time.Time
    Config              config.Config
    EndpointKind        string
    Scope               discovery.Scope
    Capabilities        []discovery.Capability
    CollectionStarted   time.Time
    CollectionCompleted time.Time
}

// Build makes a snapshot and checks it.
func Build(in Input) (Snapshot, error)

// Check reports the first integrity failure.
func (s Snapshot) Check() error

// Encode writes the snapshot with two spaces of indentation and one line
// break at the end. The same snapshot always gives the same bytes.
func Encode(s Snapshot) ([]byte, error)

// Decode reads exactly one snapshot and checks it.
func Decode(data []byte) (Snapshot, error)

// Time reads one of the time fields of a snapshot.
func Time(value string) (time.Time, error)
```

**Requirements:**

- Every time is written in the zone of `analysis.timezone`, as RFC 3339 with
  the offset, cut to whole seconds.
- `period_end` equals `emitted_at`. `period_start` is the wall clock of
  `emitted_at` in that zone, less the period, read again in that zone. No
  daylight-saving correction is made, as in the original.
- `scope.namespaces` holds the configured namespaces, sorted.
  `excluded_containers` holds the configured ones, sorted. No list is ever
  `null`; an empty list is `[]`.
- `settings.endpoint_type` is the kind that answered, `prometheus` or
  `grafana`, never `auto`.
- The snapshot holds no address, no host name, no token, no name of an
  environment variable, and no path of a file.
- `Decode` refuses, each with its own message:

| Input | Message |
|---|---|
| not JSON, or cut in the middle | `snapshot is not valid JSON: <cause>` |
| anything after the first document | `snapshot holds data after the document` |
| an unknown key | `snapshot holds an unknown key: <cause of the decoder>` |
| `schema_version` missing | `snapshot has no schema_version` |
| another `schema_version` | `snapshot has schema_version "<value>"; this version reads "1.0"` |
| an integrity failure | the message of `Check` |

- `Check` refuses, in this order:

| Condition | Message |
|---|---|
| `id` or `generator_version` empty | `report.<field> is empty` |
| a time that cannot be read | `<path> is not an RFC 3339 time: "<value>"` |
| `period_end` not equal to `emitted_at` | `report.period_end differs from report.emitted_at` |
| `period_start` not before `period_end` | `report.period_start is not before report.period_end` |
| collection completed before it started | `quality.collection_completed_at is before quality.collection_started_at` |
| `profile`, `min_severity`, `filter_mode`, `confidence`, `endpoint_type` outside their values | `<path> has an unknown value: "<value>"` |
| `cluster_name` empty, or no namespace | `scope.<field> is empty` |
| a list that is not sorted or holds a repeat | `<path> is not sorted or holds a repeated value` |
| the capabilities are not the nine of `discovery.Names` in order | `quality.capabilities must hold the nine capabilities in their order; found: <names>` |
| a state, a strategy, or a reason outside the registry | `quality.capabilities[<n>].<field> has an unknown value: "<value>"` |

**Tests:**

| Test | Input | Expected |
|---|---|---|
| `TestBuildCopiesEveryField` | an input where every field holds its own distinct value | each field of the snapshot holds the value of its source; checked field by field, so that a swapped pair fails |
| `TestBuildEveryFieldChangesTheBytes` (table) | the same input, with one field changed, for every field of `Input` and of the parts of `Config` that the snapshot reads | the bytes of `Encode` differ |
| `TestBuildWritesTimesInTheZone` | `EmittedAt` in UTC, zone `America/Sao_Paulo` | `emitted_at` ends in `-03:00` and names the same instant |
| `TestBuildCutsToSeconds` | `EmittedAt` with 700 ms | no fraction in the text |
| `TestBuildPeriodStart` (table) | `24h` from 2026-07-15 14:30 in Sao Paulo; `7d` from the same; `24h` from 2026-03-08 12:00 in `America/New_York` | `2026-07-14T14:30:00-03:00`; `2026-07-08T14:30:00-03:00`; `2026-03-07T12:00:00-05:00` |
| `TestBuildSortsLists` | namespaces `b, a`; excluded containers `y, x` | `[a b]`, `[x y]` |
| `TestBuildNeverWritesNull` | no excluded container, no evidence, no reason | the bytes hold `[]` and no `null` |
| `TestBuildKeepsSecretsOut` | address `https://metrics.internal.example:9443/prom`, token `hunter2`, `token_env` `KCR_SECRET_NAME`, bundle `/etc/ssl/private/ca.pem`, output folder `/var/reports` | the bytes hold none of `metrics.internal.example`, `9443`, `hunter2`, `KCR_SECRET_NAME`, `ca.pem`, `/var/reports` |
| `TestEncodeIsStable` | the same snapshot twice | equal bytes, ending in one line break |
| `TestDecodeReadsWhatEncodeWrote` | any built snapshot | equal to the source, by `reflect.DeepEqual` |
| `TestDecodeRefuses` (table) | one row for each row of the `Decode` table; "cut in the middle" is an encoded snapshot less its last 40 bytes | exactly that message |
| `TestCheckRefuses` (table) | one row for each row of the `Check` table, each from a valid snapshot with one field changed | exactly that message |

- [ ] **Step 1:** Write the tests.
- [ ] **Step 2:** Run `go test ./internal/snapshot/ -v`. Expected: build
  failure.
- [ ] **Step 3:** Write the three files.
- [ ] **Step 4:** Run `go test -count=1 ./internal/snapshot/`. Expected:
  pass.
- [ ] **Step 5:** Run `make test` and `make lint`, then commit.

```bash
git add internal/snapshot/
git commit -m "feat(snapshot): add the snapshot contract"
```

**Done when:** the tests pass. The golden snapshots of the six scenarios
come in Task 13.

---

### Task 10: The catalog

**Executor:** Main thread for the wording, which is the two tables below.
Subagent for the code, the two files, and the guards.

**Reference:** spec 9 and 13.2;
`older/kubernetes-report/kube_capacity_report/renderers/text.py` and
`tests/test_vocabulary.py`; section 10 of `python-preflight.md`.

**Files:**
- Create: `internal/catalog/catalog.go`, `keys.go`, `format.go`
- Create: `internal/catalog/locales/pt-BR.toml`, `locales/en.toml`
- Create: `internal/catalog/catalog_test.go`, `format_test.go`,
  `guard_test.go`

**Interfaces:**
- Consumes: `github.com/nicksnyder/go-i18n/v2/i18n`,
  `golang.org/x/text/language`, `github.com/BurntSushi/toml`; the registries
  of `discovery`.
- Produces:

```go
package catalog

type Language string

const (
    PortugueseBrazil Language = "pt-BR"
    English          Language = "en"
)

// Languages returns the two languages, Portuguese first.
func Languages() []Language

func ParseLanguage(value string) (Language, error)

// Key names one entry of the catalog.
type Key string

// Params are the named values of a sentence, already formatted as text.
type Params map[string]string

type Catalog struct{ /* unexported */ }

// Load reads the embedded file of one language.
func Load(lang Language) (*Catalog, error)

func (c *Catalog) Language() Language

// Theme returns the language and region codes that drive hyphenation:
// "pt" and "BR", or "en" and "US".
func (c *Catalog) Theme() (lang, region string)

// Text returns the entry of key with its parameters filled in. A missing
// entry, a missing parameter, and a parameter the entry does not use are
// errors.
func (c *Catalog) Text(key Key, params Params) (string, error)

// DateTime writes the wall clock of t: 15/07/2026 14:30 in Portuguese,
// 07/15/2026 14:30 in English.
func (c *Catalog) DateTime(t time.Time) string

// Integer writes a whole number with no separator.
func (c *Catalog) Integer(n int) string

// Keys returns every valid key, sorted: the constants of keys.go and the
// keys made from the registries of discovery.
func Keys() []Key

func CapabilityName(name string) Key            // capability.name.<name>
func CapabilityReason(code string) Key          // capability.reason.<CODE>
func CapabilityEvidence(name, state string) Key // capability.evidence.<name>.<state>
func State(state string) Key                    // state.<state>
func Strategy(strategy string) Key              // strategy.<strategy>
```

`keys.go` declares one constant for each key of the first table below, named
after the key in upper camel case: `cover.title` is `CoverTitle`,
`front.metadata.period_range` is `FrontMetadataPeriodRange`.

**Requirements:**

- One TOML file for each language, embedded. Keys are nested tables, which
  go-i18n joins with a dot.
- No segment of a key is a word that go-i18n reserves: `id`, `description`,
  `hash`, `leftdelim`, `rightdelim`, `zero`, `one`, `two`, `few`, `many`,
  `other`. A table made only of such words would be read as one message.
- A value is never pasted into a sentence as a noun. Where grammar differs,
  there is one entry for each case. The 27 evidence sentences are written
  out whole.
- `Text` never returns the text `<no value>`.

Wording. Portuguese is carried over from the original unchanged. English is
new and is reviewed by the owner.

| Key | Portuguese (Brazil) | English |
|---|---|---|
| `labels.note` | Nota | Note |
| `labels.info` | Informação | Information |
| `labels.warning` | Atenção | Warning |
| `cover.title` | LEVANTAMENTO DE CAPACIDADE | CAPACITY REPORT |
| `cover.cluster_label` | Cluster: | Cluster: |
| `cover.author_label` | ELABORADO POR: | PREPARED BY: |
| `cover.issued_label` | EMITIDO EM: | ISSUED ON: |
| `footer.page_label` | Página | Page |
| `front.outline_title` | Sumário | Contents |
| `front.metadata.heading` | Metadados do relatório | Report metadata |
| `front.metadata.header_field` | Campo | Field |
| `front.metadata.header_value` | Valor | Value |
| `front.metadata.environment` | Ambiente | Environment |
| `front.metadata.period` | Período analisado | Analysis period |
| `front.metadata.period_range` | {{.Start}} a {{.End}} | {{.Start}} to {{.End}} |
| `front.metadata.namespaces` | Namespaces | Namespaces |
| `front.metadata.excluded_containers` | Containers excluídos | Excluded containers |
| `front.metadata.no_excluded_containers` | Nenhum | None |
| `front.metadata.organization` | Organização | Organization |
| `front.metadata.classification` | Classificação | Classification |
| `front.legend.heading` | Legenda de status | Status legend |
| `front.legend.header_status` | Status | Status |
| `front.legend.header_meaning` | Interpretação | Meaning |
| `front.legend.healthy` | Saudável | Healthy |
| `front.legend.attention` | Atenção / prioridade alta | Attention / high priority |
| `front.legend.critical` | Crítico / prioridade crítica | Critical / critical priority |
| `front.legend.unavailable` | Dado indisponível | Data unavailable |
| `front.legend.undefined` | Sem base de cálculo | No base for the ratio |
| `front.quality.heading` | Qualidade dos dados | Data quality |
| `front.quality.available` | Disponíveis: | Available: |
| `front.quality.partial` | Parciais: | Partial: |
| `front.quality.unavailable` | Indisponíveis: | Unavailable: |
| `appendix.heading` | Apêndice técnico de qualidade | Technical quality appendix |
| `appendix.header_capability` | Capacidade | Capability |
| `appendix.header_coverage` | Cobertura | Coverage |
| `appendix.header_source` | Fonte | Source |
| `appendix.header_remark` | Observação | Remark |
| `appendix.header_evidence` | Evidência técnica | Technical evidence |
| `appendix.no_remark` | Sem observação | No remark |
| `appendix.empty` | Nenhuma evidência técnica está disponível para os filtros atuais. | No technical evidence is available for the current filters. |
| `state.available` | Disponível | Available |
| `state.partial` | Parcial | Partial |
| `state.unavailable` | Indisponível | Unavailable |
| `strategy.recording_rule` | Regra de gravação | Recording rule |
| `strategy.raw_metrics` | Métricas brutas | Raw metrics |
| `strategy.configured` | Configuração | Configuration |
| `strategy.none` | Sem fonte | No source |
| `capability.name.namespace_usage` | Uso por namespace | Usage by namespace |
| `capability.name.workload_usage` | Uso por carga de trabalho | Usage by workload |
| `capability.name.requests_limits` | Solicitações e limites | Requests and limits |
| `capability.name.hpa_history` | Histórico de HPA | HPA history |
| `capability.name.workloads_without_hpa` | Cobertura de cargas de trabalho sem HPA | Coverage of workloads without HPA |
| `capability.name.ip_node_mapping` | Associação entre endereço e nó | Mapping between address and node |
| `capability.name.nodepool_discovery` | Identificação de pools de nós | Identification of node pools |
| `capability.name.node_utilization` | Utilização dos nós | Node utilization |
| `capability.name.node_autoscaling` | Escalonamento automático de nós | Node autoscaling |
| `capability.reason.METRICS_NOT_FOUND` | As métricas necessárias não foram encontradas. | The required metrics were not found. |
| `capability.reason.CPU_USAGE_METRIC_NOT_FOUND` | A métrica de uso de CPU não foi encontrada. | The CPU usage metric was not found. |
| `capability.reason.MEMORY_USAGE_METRIC_NOT_FOUND` | A métrica de uso de memória não foi encontrada. | The memory usage metric was not found. |
| `capability.reason.USAGE_METRICS_NOT_FOUND` | As métricas de uso de CPU e memória não foram encontradas. | The CPU and memory usage metrics were not found. |
| `capability.reason.WORKLOAD_HISTORY_INEXACT` | A associação histórica entre pods e cargas de trabalho não pôde ser reconstruída com exatidão. | The history of which pod belonged to which workload could not be rebuilt exactly. |
| `capability.reason.WORKLOAD_ASSOCIATION_UNAVAILABLE` | A associação por carga de trabalho não está disponível; a cobertura está limitada a pods. | Pods cannot be matched to workloads; coverage is limited to pods. |
| `capability.reason.REQUESTS_LIMITS_INCOMPLETE` | A cobertura de solicitações e limites está incompleta para CPU ou memória. | The coverage of requests and limits is incomplete for CPU or memory. |
| `capability.reason.LEGACY_METRICS_PARTIAL` | As métricas legadas oferecem cobertura parcial de recursos. | The legacy metrics give partial coverage of resources. |
| `capability.reason.HPA_HISTORY_INCOMPLETE` | A cobertura histórica dos HPAs está incompleta. | The historical coverage of the HPAs is incomplete. |
| `capability.reason.WORKLOAD_INVENTORY_INCOMPLETE` | O inventário de cargas de trabalho ou alvos de HPA está incompleto. | The inventory of workloads or of HPA targets is incomplete. |
| `capability.reason.NODE_ADDRESS_MAPPING_INCOMPLETE` | A associação entre endereços e nós pode estar incompleta. | The mapping between addresses and nodes may be incomplete. |
| `capability.reason.NODEPOOL_LABELS_NOT_FOUND` | Os rótulos de pools de nós não foram encontrados. | The node pool labels were not found. |
| `capability.reason.NODE_AUTOSCALING_UNDETERMINED` | O estado do escalonamento automático de nós não pôde ser determinado. | The state of node autoscaling could not be determined. |

Evidence sentences, `capability.evidence.<name>.<state>`. Each of the 27
entries is the subject of its row, one space, the ending of its state, and a
full stop, written out whole in the file.

| Name | Subject in Portuguese | Subject in English |
|---|---|---|
| `namespace_usage` | Os dados de utilização de CPU e memória por namespace | CPU and memory usage data by namespace |
| `workload_usage` | Os dados de utilização por carga de trabalho | Workload usage data |
| `requests_limits` | Os dados de solicitações e limites de recursos | Data on resource requests and limits |
| `hpa_history` | Os dados históricos e de configuração dos HPAs | HPA history and configuration data |
| `workloads_without_hpa` | Os dados de cobertura das cargas de trabalho sem HPA | Coverage data for workloads without HPA |
| `ip_node_mapping` | Os dados de associação entre endereço e nó | Data on the mapping between address and node |
| `nodepool_discovery` | Os dados de identificação dos pools de nós | Node pool identification data |
| `node_utilization` | Os dados de utilização dos nós | Node utilization data |
| `node_autoscaling` | Os dados de escalonamento automático dos nós | Node autoscaling data |

| State | Ending in Portuguese | Ending in English |
|---|---|---|
| `available` | estão disponíveis para o período analisado | is available for the analysis period |
| `partial` | estão disponíveis parcialmente para o período analisado | is partly available for the analysis period |
| `unavailable` | não foram localizados para o período analisado | was not found for the analysis period |

Banned terms. A word is a run of letters, digits, `_`, and `-`, so that
`requests_limits` and `WORKLOAD_CPU_LIMIT_MISSING` are one word each and
never match.

| Language | Banned, in any letter case | Banned as written |
|---|---|---|
| Portuguese | `request`, `requests`, `limit`, `limits`, `workload`, `workloads`, `right-sizing`, `headroom`, `autoscaling`, `autoscaler`, `throttling`, `unknown`, `memory`; the phrases `node pool`, `node pools`, `grupo de nós`, `grupos de nós`; the English words `the`, `is`, `are`, `was`, `were`, `not`, `found`, `with`, `from`, `omitted`, `cannot`, `may`, `only`, `uses` | `cpu` |
| English | the phrases `node group`, `node groups`; `nodepool`, `nodepools`; the Portuguese words `não`, `está`, `estão`, `são`, `foram`, `para`, `uma`, `dos`, `das` | `cpu` |

**Tests:**

| Test | Input | Expected |
|---|---|---|
| `TestParseLanguage` (table) | `pt-BR`, `en`, `pt-br`, `fr`, empty | the two languages; then three errors that list the accepted values |
| `TestLoadBothLanguages` | each language | no error |
| `TestTextPlainEntry` (table) | `cover.title` in each language | the two titles of the table |
| `TestTextFillsParameters` | `front.metadata.period_range`, `Start` and `End` | `14/07/2026 14:30 a 15/07/2026 14:30`, and the English one with `to` |
| `TestTextRefusesMissingEntry` | key `nope` | error naming `nope` and the language |
| `TestTextRefusesMissingParameter` | the range with `Start` only | error naming `End`; the text holds no `<no value>` |
| `TestTextRefusesUnusedParameter` | `cover.title` with a parameter | error naming the parameter |
| `TestTextDoesNotReadMarkup` | a parameter `{{.End}}` given as the value of `Start` | the value is printed as it is |
| `TestTheme` (table) | each language | `pt`, `BR`; `en`, `US` |
| `TestDateTime` (table) | 2026-07-15 14:30 -03:00; 2026-01-02 03:04 | `15/07/2026 14:30` and `07/15/2026 14:30`; `02/01/2026 03:04` and `01/02/2026 03:04` |
| `TestDateTimeUsesTheWallClockGiven` | the same instant in UTC and in Sao Paulo | two different texts |
| `TestInteger` (table) | 0, 7, 1234567 | `0`, `7`, `1234567` |
| `TestWordingMatchesThePlan` (table) | every row of the two wording tables, and the 27 evidence sentences | each entry equals its text, in both languages |
| `TestKeysEqualTheFiles` | `Keys()` and the keys of each file | the three sets are equal; a difference is listed by name |
| `TestEveryRegistryCodeHasAnEntry` | `discovery.Names`, `States`, `Strategies`, `ReasonCodes` | each has its key in both languages |
| `TestLanguagesUseTheSameParameters` | every key | the names inside `{{.…}}` are equal in both languages |
| `TestNoReservedSegment` | every key | no segment is a reserved word |
| `TestNoEmDashInsideAnEntry` | every entry | no entry holds `—`, unless the entry is `—` alone |
| `TestBannedTermsFinder` (table) | `Revisar o request de CPU e o consumo observado.`; `Uso observado de cpu no nó.`; `O grupo de nós não pôde ser determinado.`; `autoscaling state is unknown`; `Revisar a solicitação de CPU e o consumo observado.`; `Definir o limite de memória.`; `WORKLOAD_CPU_LIMIT_MISSING requests_limits report-ident`; `Risco aumentado de OOMKill no Deployment.` | in Portuguese: `request`; `cpu`; `grupo de nós`; `autoscaling`, `is`, `unknown`; and nothing for the last four |
| `TestNoBannedTerm` | every entry of each language, with what stands between `{{` and `}}` removed | nothing found |
| `TestEmbeddedFilesAreValidTOML` | both files | they parse, and no value is empty |

- [ ] **Step 1:** Write the tests.
- [ ] **Step 2:** Run `go test ./internal/catalog/ -v`. Expected: build
  failure.
- [ ] **Step 3:** Write the two TOML files from the tables, then the three
  Go files.
- [ ] **Step 4:** Run `go test -count=1 ./internal/catalog/`. Expected:
  pass.
- [ ] **Step 5:** See the guards fail. For each of `TestKeysEqualTheFiles`,
  `TestLanguagesUseTheSameParameters`, `TestNoEmDashInsideAnEntry`, and
  `TestNoBannedTerm`, plant one violation in `en.toml`, run the test, read
  the failure, and remove the violation. The report quotes the four
  failures.
- [ ] **Step 6:** Run `make test` and `make lint`, then commit.

```bash
git add internal/catalog/ go.mod go.sum
git commit -m "feat(catalog): add the wording in two languages"
```

**Done when:** the tests pass, the four guards were seen failing, and
`go.mod` holds no forbidden module. The guard against an entry that nothing
uses comes in Task 11, with the first package that uses the entries.

---

### Task 11: The document

**Executor:** Subagent.

**Reference:** spec 10.1 to 10.3;
`older/kubernetes-report/kube_capacity_report/renderers/sections.py:139-246`
and `:297-329`, and `renderers/capabilities.py:141-157`; section 10 of
`python-preflight.md`; `template/capacity/validate.typ`, which is the
contract; `template/tests/capacity/fixtures/parity.pt-BR.json`, which shows
the opening page and the appendix as the original printed them.

**Files:**
- Create: `internal/document/document.go`, `build.go`, `front.go`,
  `appendix.go`
- Create: `internal/document/document_test.go`, `build_test.go`
- Modify: `internal/catalog/guard_test.go`, to add `TestEveryKeyIsUsed`

**Interfaces:**
- Consumes: `snapshot.Snapshot`, `snapshot.Time`, `*catalog.Catalog`.
- Produces:

```go
package document

const Schema = "1"

type Document struct {
    Schema   string  `json:"schema"`
    Language string  `json:"language"`
    Theme    Theme   `json:"theme"`
    Labels   Labels  `json:"labels"`
    Cover    Cover   `json:"cover"`
    Footer   Footer  `json:"footer"`
    Blocks   []Block `json:"blocks"`
}

type Theme struct {
    Lang   string `json:"lang"`
    Region string `json:"region"`
}

type Labels struct {
    Note    string `json:"note"`
    Info    string `json:"info"`
    Warning string `json:"warning"`
}

type Cover struct {
    Title        string  `json:"title"`
    ClusterLabel string  `json:"cluster_label"`
    Cluster      string  `json:"cluster"`
    AuthorLabel  string  `json:"author_label"`
    Author       *string `json:"author"` // null when not set
    IssuedLabel  string  `json:"issued_label"`
    Issued       string  `json:"issued"`
}

type Footer struct {
    Classification *string `json:"classification"` // null when not set
    PageLabel      string  `json:"page_label"`
}

// Block is one entry of "blocks". Each kind below writes exactly the keys
// that validate.typ accepts for it.
type Block interface{ blockType() string }

type Front struct{ Blocks []Block }
type Outline struct {
    Title string
    Depth int
}
type Heading struct {
    Level    int
    Numbered bool
    Runs     []Run
}
type Paragraph struct{ Runs []Run }
type List struct{ Items [][]Run }
type Table struct {
    ID      string
    Headers []string
    Rows    [][][]Run // rows, cells, runs
}
type PageBreak struct{ Weak bool }

// Run is one run of a cell, a paragraph, a heading, or a list item.
type Run struct{ /* unexported */ }

func Text(text string) Run
func Strong(text string) Run
func Status(status string) Run            // the mark alone; "text" is null
func StatusText(status, text string) Run  // the mark and its label

type Options struct {
    Profile     string // empty means the one stored in the snapshot
    MinSeverity string // empty means the one stored in the snapshot
}

func Build(s snapshot.Snapshot, c *catalog.Catalog, opts Options) (Document, error)

// Encode writes the document with two spaces of indentation and one line
// break at the end.
func Encode(d Document) ([]byte, error)
```

The block kind `callout` and the run kinds `ident` and `placeholder` are not
declared. They enter with the first feature that prints them, in part 2.2.

**Requirements:**

- Every word comes from the catalog or from the snapshot. `document` holds no
  sentence.
- Blocks, in this order:

| # | Block | Content |
|---|---|---|
| 1 | `front` | the seven blocks below |
| 1.1 | `outline` | title `front.outline_title`, depth 2 |
| 1.2 | `heading` | level 2, not numbered, `front.metadata.heading` |
| 1.3 | `table` `report-metadata` | headers Field and Value; the rows below |
| 1.4 | `heading` | level 2, not numbered, `front.legend.heading` |
| 1.5 | `table` `status-legend` | five rows, in the order healthy, attention, critical, unavailable, undefined; first cell the mark alone, second cell the text |
| 1.6 | `heading` | level 2, not numbered, `front.quality.heading` |
| 1.7 | `list` | three items, available, partial, unavailable; each is a `strong` run with the label and a `text` run with one space and the count |
| 2 | `pagebreak` | not weak |
| 3 | `pagebreak` | weak |
| 4 | `heading` | level 1, numbered, `appendix.heading` |
| 5 | `table` `quality-appendix`, or `paragraph` | the rows below; when no row passes the minimum severity, a paragraph with `appendix.empty` |

- Rows of the metadata table, each a label and a value, both `text` runs:

| Row | Value | Present |
|---|---|---|
| Environment | the cluster name | always |
| Analysis period | `front.metadata.period_range` with the start and the end, by `DateTime` | always |
| Namespaces | the namespaces, joined by `, ` | always |
| Excluded containers | the names, joined by `, `, or `front.metadata.no_excluded_containers` | always |
| Organization | the organization | when not empty |
| Classification | the classification | when not empty |

- Rows of the appendix, one for each capability that passes, in the order of
  the snapshot:

| Cell | Content |
|---|---|
| Capability | `text`: the name of the capability |
| Coverage | `status` with its label: the state |
| Source | `text`: the strategy |
| Remark | `text`: the sentences of the reasons, joined by one space, or `appendix.no_remark` |
| Technical evidence | `text`: the evidence sentence of the name and state |

- The status of a state: `available` is `healthy`, `partial` is `attention`,
  `unavailable` is `critical`.
- The minimum severity applies to the appendix rows by that status: `all`
  keeps every row, `attention` keeps partial and unavailable, `critical`
  keeps unavailable. It does not change the three counts of the opening
  page, which always count all nine.
- The times of the snapshot are printed by their own wall clock and offset.
  `document` reads no time zone data.
- Cover: `cluster` is the cluster name; `issued` is the emission time;
  `author` is null when the author is empty. Footer: `classification` is
  null when it is empty.
- `Build` refuses a profile other than `detailed`:
  `report profile "<value>" is not available in this version`. It refuses an
  unknown minimum severity and a snapshot that fails `Check`.

**Tests.** `base` is a snapshot built by hand in the test file, with nine
capabilities: six available, two partial, one unavailable with two reasons.

| Test | Input | Expected |
|---|---|---|
| `TestEveryKeyIsUsed`, in `internal/catalog` | every constant of `keys.go` and every key function | each is named in a file of `internal/document` that is not a test |
| `TestBuildTopLevel` (table) | `base`, each language | schema `1`; the language; the theme; the three labels; the footer label |
| `TestBuildCover` (table) | `base`, each language | the seven fields; `issued` is `15/07/2026 14:30` or `07/15/2026 14:30` |
| `TestBuildCoverWithoutAuthor` | author empty | `"author": null` in the bytes |
| `TestBuildFooterWithoutClassification` | classification empty | `"classification": null` in the bytes |
| `TestBuildBlockOrder` | `base` | the kinds, in order: front, pagebreak, pagebreak, heading, table; inside front: outline, heading, table, heading, table, heading, list |
| `TestBuildMetadataRows` (table) | all set; no organization; no classification; no excluded container | 6 rows; 5; 5; 6 with `None` or `Nenhum` |
| `TestBuildMetadataJoinsNames` | namespaces `a`, `b`, `c` | `a, b, c` |
| `TestBuildLegendRows` (table) | each language | five rows; the statuses in order; the first cell of each holds one `status` run whose text is null |
| `TestBuildQualityCounts` | `base` | ` 6`, ` 2`, ` 1`, each after a `strong` label |
| `TestBuildQualityCountsIgnoreSeverity` | `base`, minimum `critical` | the same three counts |
| `TestBuildAppendixRows` | `base` | nine rows, in the order of the snapshot, five cells each |
| `TestBuildAppendixCoverage` (table) | the three states | `healthy`, `attention`, `critical`, each with its label |
| `TestBuildAppendixRemark` (table) | no reason; one reason; two reasons | `No remark`; the sentence; the two sentences with one space between |
| `TestBuildAppendixEvidence` (table) | each name with each state, 27 rows | the sentence of the catalog |
| `TestBuildAppendixSeverity` (table) | `all`; `attention`; `critical` | 9 rows; 3 rows; 1 row |
| `TestBuildAppendixEmpty` | all nine available, minimum `attention` | block 5 is a paragraph with `appendix.empty`, and no table follows |
| `TestBuildOptionsDefaultToTheSnapshot` | the snapshot stores `attention`; options empty | 3 rows |
| `TestBuildOptionsOverrideTheSnapshot` | the snapshot stores `attention`; options `all` | 9 rows |
| `TestBuildRefuses` (table) | profile `executive`; profile `x`; severity `x`; a snapshot with eight capabilities | the messages |
| `TestBuildPrintsNamesAsTheyAre` | cluster `#prod *x* $y$ <a> @b [c]`, organization `ACME & Filhos "Ltda."` | the same texts in the document, character for character |
| `TestBuildNeedsNoZoneData` | `ZONEINFO` set to a folder that does not exist; a snapshot in `-03:00` | `14:30` on the cover |
| `TestEncodeIsStable` | the same document twice | equal bytes |
| `TestEncodeWritesExactKeys` (table) | one block of each kind, one run of each kind | the keys of each object, sorted, equal the list that `validate.typ` accepts for it, which the test file states kind by kind |
| `TestEncodeNeverWritesNullLists` | a table cell with no run | `[]` |
| `TestSameSnapshotGivesSameBytes` | `base`, built and encoded twice, each language | equal bytes |
| `TestLanguagesDifferOnlyInWording` | `base` in both languages | the same kinds of block, in the same order, with the same number of rows |

- [ ] **Step 1:** Write the tests.
- [ ] **Step 2:** Run `go test ./internal/document/ -v`. Expected: build
  failure.
- [ ] **Step 3:** Write the four files.
- [ ] **Step 4:** Run `go test -count=1 ./internal/document/ ./internal/catalog/`.
  Expected: pass.
- [ ] **Step 4a:** See the guard fail. Add the constant `Planted Key = "planted"`
  to `keys.go`, run `TestEveryKeyIsUsed` and `TestKeysEqualTheFiles`, read
  both failures, and remove the constant.
- [ ] **Step 5:** Run `make test` and `make lint`. Expected: pass.
- [ ] **Step 6:** Commit.

```bash
git add internal/document/ internal/catalog/guard_test.go
git commit -m "feat(document): build front page and appendix"
```

**Done when:** the tests pass and `make test` is green.

---

### Task 12: The wording guard

**Executor:** Subagent.

**Class it protects against:** a sentence for a reader written outside the
catalog. The brief lists, as tests, the texts that must not match.

**Files:**
- Create: `internal/guard/wording.go`, `internal/guard/wording_test.go`

**Interfaces:**
- Consumes: `go/parser`, `go/ast`. No new dependency.
- Produces:

```go
// ReaderText parses source as a Go file and returns one entry for each
// string literal that looks like text for a reader, in source order, as
// "<file>:<line>: <literal>". With strict false it applies the rule that
// holds for every package; with strict true it adds the rules that hold
// for internal/document.
func ReaderText(filename string, source []byte, strict bool) ([]string, error)
```

**Requirements:**

- Never reported: an import path; a struct tag; an argument of `errors.New`,
  of `fmt.Errorf`, or of a call whose function name ends in `Errorf`; a
  literal on a line that ends with the comment
  `// wording: not reader text, <reason>`, where the reason is not empty.
- Rule for every package: a literal that holds a letter outside ASCII.
- Rules added by `strict`, applied after every `%` verb is removed:
  - two words of two or more letters each, with one space between them;
  - a word that starts with one upper-case letter followed by a lower-case
    letter.
- The repository test reads the Go files that git knows about, test files
  left out, under `internal/` and `cmd/`. `internal/catalog` and
  `internal/guard` are left out. `internal/document` is read with `strict`.

**Tests:**

| Test | Input | Expected |
|---|---|---|
| `TestReaderTextFindsPortuguese` | `x := "Sem observação"` | one entry, with the line |
| `TestReaderTextFindsSentenceWhenStrict` | `x := "No remark"` | strict: one entry; not strict: none |
| `TestReaderTextFindsCapitalisedWordWhenStrict` | `x := "Page"` | strict: one entry |
| `TestReaderTextIgnoresIdentifiers` (table) | `report-metadata`, `text`, `pt-BR`, `healthy`, `schema_version`, `2006-01-02T15:04:05Z07:00`, `02/01/2006 15:04`, `, `, ` `, `%s: %d`, `—`, `●` | strict: none |
| `TestReaderTextIgnoresDiagnostics` (table) | `errors.New("Report profile is wrong")`; `fmt.Errorf("Unknown value %q", v)`; `t.Errorf("Got it")` | strict: none |
| `TestReaderTextIgnoresTagsAndImports` | a struct with `json:"cluster name"`; an import | none |
| `TestReaderTextHonoursTheComment` (table) | `x := "America/Sao_Paulo" // wording: not reader text, a zone name`; the same with an empty reason; the same comment on the line above | none; one entry; one entry |
| `TestReaderTextReportsEveryLiteral` | three literals, two of them text | two entries, in order |
| `TestReaderTextRejectsBrokenSource` | a file that does not parse | an error |
| `TestRepositoryHoldsNoReaderTextOutsideTheCatalog` | the repository | nothing found |

- [ ] **Step 1:** Write the tests.
- [ ] **Step 2:** Run `go test ./internal/guard/ -run ReaderText -v`.
  Expected: build failure.
- [ ] **Step 3:** Write `wording.go`.
- [ ] **Step 4:** Run `go test -count=1 ./internal/guard/`. Expected: pass.
  If the repository test finds a literal, the subagent stops and reports
  it. It does not add the comment on its own: the main thread decides
  whether the literal is wording.
- [ ] **Step 5:** See the guard fail. Add `var planted = "Sem observação"` to
  `internal/document/build.go`, run the repository test, read the failure,
  and remove the line.
- [ ] **Step 6:** Run `make test` and `make lint`, then commit.

```bash
git add internal/guard/wording.go internal/guard/wording_test.go
git commit -m "test(guard): keep reader text in the catalog"
```

**Done when:** the tests pass and the guard was seen failing.

---

### Task 13: Golden snapshots and golden data files

**Executor:** Subagent for the code. Main thread for Step 5 and Step 6.

**Files:**
- Create: `internal/goldentest/goldentest.go`, `golden_test.go`
- Create: `cmd/update-testdata/main.go`
- Create: `testdata/snapshot/<scenario>.json`, six files
- Create: `template/tests/capacity/fixtures/command/<scenario>.<language>.json`,
  eight files
- Modify: `template/capacity_test.go:29-60`, so that
  `TestCapacityFixturesCompileCleanly` also compiles every file of
  `fixtures/command/`
- Modify: `Makefile`

**Interfaces:**
- Consumes: `promtest`, `prom`, `discovery`, `snapshot`, `catalog`,
  `document`, `config`.
- Produces:

```go
package goldentest

// DataFileScenarios names the scenarios that have golden data files. The
// other two give, in this part, the same document as direct-complete.
var DataFileScenarios = []string{
    "direct-complete", "partial-data", "raw-fallback", "value-edge-cases",
}

// Snapshot serves the scenario in this process, runs the connection, the
// scope, and the capabilities against it with the fixed input, and returns
// the encoded snapshot.
func Snapshot(ctx context.Context, scenario promtest.Scenario) ([]byte, error)

// DataFile decodes an encoded snapshot and returns the encoded document in
// one language, with the profile and the severity stored in the snapshot.
func DataFile(snapshot []byte, lang catalog.Language) ([]byte, error)

// SnapshotPath and DataFilePath return the place of a golden file, from
// the folder that holds go.mod.
func SnapshotPath(scenario string) (string, error)
func DataFilePath(scenario string, lang catalog.Language) (string, error)
```

Fixed input:

| Field | Value |
|---|---|
| Report identifier | `00000000-0000-4000-8000-000000000001` |
| Generator version | `0.0.0-golden` |
| Emitted at | 2026-07-15 14:30:00 in `America/Sao_Paulo` |
| Collection started, completed | the same instant, and 42 seconds later |
| Cluster, namespaces, filter mode, period | `prod`, `payments`, `required`, `24h` |
| Organization, classification, author | `ACME`, `Uso interno`, `Time SRE` |
| Excluded containers | `istio-proxy` |
| Endpoint type, API root, token, node autoscaling | those of the scenario |
| Everything else | the defaults |

**Requirements:**

- `go run ./cmd/update-testdata` rewrites the fourteen files and prints one
  line for each: `written`, or `unchanged`. `make update-testdata` runs it.
- A test never writes a golden file. A missing or different file fails the
  test, and the message names `make update-testdata`.
- A file that the lists do not name, found in either folder, fails a test.

**Tests:**

| Test | Input | Expected |
|---|---|---|
| `TestGoldenSnapshots` (table) | each of the six scenarios | the bytes equal the file |
| `TestGoldenDataFiles` (table) | each of the four scenarios, each language | the bytes equal the file |
| `TestGoldenFoldersHoldNoStranger` | the two folders | every file is named by the lists |
| `TestSnapshotIsTheSameTwice` | `direct-complete`, run twice | equal bytes |
| `TestOtherScenariosGiveTheSameDocument` | `grafana-authenticated`, `thanos-multicluster` | their documents equal that of `direct-complete` in each language, which is why they have no file of their own; the snapshots differ only in `scope.discovered_values` and `settings.endpoint_type` |
| `TestGoldenFailureNamesTheCommand` | a golden file replaced, in a temporary copy, by another text | the failure text holds `make update-testdata` |

- [ ] **Step 1:** Write the tests.
- [ ] **Step 2:** Run `go test ./internal/goldentest/ -v`. Expected: build
  failure.
- [ ] **Step 3:** Write `goldentest.go`, `cmd/update-testdata/main.go`, and
  the `update-testdata` target, which needs no image.
- [ ] **Step 4:** Run `go test ./internal/goldentest/ -v`. Expected:
  `TestGoldenSnapshots` and `TestGoldenDataFiles` fail, because the files do
  not exist, and the failure names the command.
- [ ] **Step 5 (main thread):** Run `make update-testdata`. Read each of the
  fourteen files whole. For each snapshot, confirm the nine capabilities
  against the table of Task 8 and look for any address or host name. For
  each data file, read every sentence in both languages. A value that
  cannot be explained is a defect, not a golden file.
- [ ] **Step 6 (main thread):** Compile the eight data files with the native
  tools. The PDFs to open come in Task 15.

```bash
go test -count=1 -tags=template ./template/ -run TestCapacityFixturesCompileCleanly -v
```

Expected: pass, and the output names the eight files of `fixtures/command/`.

- [ ] **Step 7:** Run `make test` and `make lint`, then commit.

```bash
git add internal/goldentest/ cmd/update-testdata/ testdata/snapshot/ \
  template/tests/capacity/fixtures/command/ template/capacity_test.go Makefile
git commit -m "test(golden): pin snapshots and data files"
```

**Done when:** the tests pass, the fourteen files were read by the main
thread, and the review is recorded in
`.cache/phase-2/part-1/golden-review.md`.

---

### Task 14: The `report` command

**Executor:** Subagent.

**Reference:** spec 5; `older/kubernetes-report/kube_capacity_report/cli.py`
and `application.py`; sections 5 and 8 of `python-preflight.md`.

**Files:**
- Modify: `internal/cli/root.go`, `internal/cli/root_test.go`
- Modify: `cmd/k8s-capacity-report/main.go`
- Create: `internal/cli/report.go`, `run.go`, `output.go`
- Create: `internal/cli/report_test.go`, `run_test.go`, `output_test.go`
- Create: `internal/cli/testdata/fake-typst.sh`

**Interfaces:**
- Consumes: everything built so far, `typst.Locate`, `Toolchain.Probe`,
  `Toolchain.Compile`, `template.Unpack`, `template.FontPath`,
  `template.Entry`, `github.com/google/uuid`.
- Produces:

```go
package cli

// Env is everything the command takes from outside itself.
type Env struct {
    Getenv   func(string) string
    LookPath func(string) (string, error)
    Now      func() time.Time
    NewID    func() (string, error)
    TempDir  string // parent of the work folder; empty means os.TempDir()
    Version  string // written to the snapshot as the generator version
}

// RealEnv returns the environment of the running process.
func RealEnv() Env

// Run executes the command line and returns the exit code.
func Run(ctx context.Context, env Env, args []string, stdout, stderr io.Writer) int
```

`main.go` calls `Run` with `RealEnv()` and a context that ends on `SIGINT`
and `SIGTERM`.

**Requirements:**

Flags of `report`:

| Flag | Value | Mode |
|---|---|---|
| `--config` | path, default `report.toml` | collecting |
| `--metrics-url`, `--endpoint-type`, `--token`, `--insecure`, `--cluster`, `--cluster-label`, `--cluster-filter`, `--namespaces`, `--period`, `--timezone`, `--node-autoscaling` | as in spec 5.2 | collecting |
| `--min-severity`, `--report-profile`, `--lang` | as in spec 5.2 | every mode that writes a PDF |
| `--output` | path of the PDF | PDF |
| `--snapshot` | path of the snapshot to write | snapshot |
| `--from-snapshot` | path of the snapshot to read | from snapshot |
| `--check` | none | check |
| `--verbose` | none | every mode |

Modes:

| Mode | Chosen by | Does |
|---|---|---|
| check | `--check` | configuration, connection, scope, capabilities |
| snapshot | `--snapshot` | the same, then writes the snapshot; needs no `typst` |
| PDF | none of the three | the same, then builds the snapshot in memory and writes the PDF |
| from snapshot | `--from-snapshot` | reads the snapshot and writes the PDF; makes no request |

Order of a PDF run. Each step stops the run when it fails.

1. Load the configuration.
2. Refuse the profile `executive` (R3).
3. Resolve the output path and check it.
4. Locate and probe `typst` (R12).
5. Connect, resolve the scope, detect the capabilities, check that they are
   sufficient.
6. Build the snapshot.
7. Build the document in the language, encode it.
8. Create the work folder, unpack the template, write `report.json` in it.
9. Compile to a temporary file beside the output path, then rename it.
10. Remove the work folder, whatever happened.

Exit codes and messages. Every failure prints one line, `error: <message>`,
on standard error, and nothing on standard output.

| Failure | Code |
|---|---|
| a flag that the mode does not take; two modes at once | 2 |
| any error of `config.Load` or `config.LoadRenderOptions` | 2 |
| the profile `executive` | 2 |
| a snapshot that cannot be read or is refused by `snapshot.Decode` | 2 |
| any `*prom.Error` | 3 |
| any `*discovery.ScopeError` | 4 |
| `discovery.ErrInsufficient` | 5 |
| the output path, `typst` missing or of another version, the work folder, the compile, the writing | 6 |
| a cancelled context: `interrupted` | 1 |
| anything else, and a recovered panic: `internal failure: <text>` | 1 |

Messages of the command itself:

| Condition | Message |
|---|---|
| `--check` with `--output`, `--snapshot`, or `--from-snapshot` | `--check cannot be combined with <flag>` |
| `--snapshot` with `--output` | `--snapshot cannot be combined with --output` |
| `--from-snapshot` with `--snapshot`, `--config`, or a collecting flag | `--from-snapshot cannot be combined with <flag>` |
| `--lang` with `--check` or `--snapshot` | `--lang applies only when a PDF is written` |
| `--output` not ending in `.pdf` | `output path must end in .pdf` |
| `--snapshot` not ending in `.json` | `snapshot path must end in .json` |
| the folder of the path is missing or cannot be written | `output directory must exist and be writable: <folder>` |
| the path names a folder | `output path is a folder: <path>` |

Lines on standard output, on success:

| Mode | Lines |
|---|---|
| check | `config: loaded`, `endpoint: available`, `scope: resolved`, `capability: discovered`, then `<name>: <state>` for each of the nine, in order |
| snapshot | the lines of check, then `snapshot: <path>` |
| PDF | the lines of check, then `report: <path>` |
| from snapshot | `snapshot: loaded`, then `report: <path>` |

`--verbose` adds, after those lines: `config path: <path>`,
`endpoint path: <path of the address, or />`, `retries: 2 (3 attempts)`,
`strategies: <the strategies other than none, sorted, joined by ", ">`. The
first three are left out in the mode from snapshot, which adds
`generator: <version stored in the snapshot>`.

Paths (R4):

- The default name is
  `report-resources-<slug><suffix>-<language>-<YYYYMMDD-HHMMSS>.pdf`. The
  slug is the cluster name in lower case, with every run of characters
  outside `a-z0-9` turned into one `-`, and with no `-` at either end; an
  empty slug is `cluster`. The suffix is `-<severity>` when the minimum
  severity is not `all`, and empty otherwise. The language is in lower case.
  The time is the emission time, in its own zone.
- In the mode from snapshot the name is made from the snapshot.

Writing:

- A file is written to `.<name>.<random>.tmp` in the folder of its final
  path, flushed to the disk, and renamed. The temporary file is removed when
  anything fails.
- The work folder is made by `os.MkdirTemp` under `Env.TempDir`, and removed
  at the end of the run in every case.

Redaction:

- One `redact.Redactor` is built as soon as the flags are read, from the
  value of `--token`, and built again, with the loaded token added, as soon
  as the configuration is loaded.
- Both streams are wrapped by it. Nothing is written to either stream any
  other way.

**Tests.** The server is `httptest.NewServer` over `promtest.NewServer`. The
compiler is `testdata/fake-typst.sh`, named by `KCR_TYPST`: it answers
`--version` with `typst 0.15.1`, and on `compile` it copies the file named by
`--input data=` from the root to its last argument, after the line
`%PDF-fake`. The clock, the identifier, and the version are fixed.

| Test | Input | Expected |
|---|---|---|
| `TestRootStillPrintsVersion` | `--version` | as today |
| `TestReportHelpListsEveryFlag` | `report --help` | every flag of the table |
| `TestCheckPrintsThePreflight` | `direct-complete`, `--check` | code 0; the 13 lines, in order; nothing on standard error |
| `TestCheckMakesNoRangeRequest` | the same | no request to `/query_range` |
| `TestCheckVerbose` | `--check --verbose` | the 13 lines, then the four lines |
| `TestCheckNeedsNoTypst` | `--check`, no compiler anywhere | code 0 |
| `TestSnapshotModeWritesTheGoldenFile` (table) | each scenario, with the fixed input of Task 13 given through a configuration file and `Env`; `--snapshot <temp>/s.json` | code 0; the file equals `testdata/snapshot/<name>.json` |
| `TestSnapshotModeNeedsNoTypst` | no compiler anywhere | code 0 |
| `TestPDFModeWritesTheFile` | `direct-complete`, `--lang en --output <temp>/r.pdf` | code 0; the file starts with `%PDF-fake`; what follows equals the golden data file in English |
| `TestPDFModeDefaultsToPortuguese` | no `--lang` | what follows equals the golden data file in Portuguese |
| `TestPDFModeDefaultName` (table) | cluster `prod`; cluster `Prod West_1`; cluster `---`; minimum `attention`; language `pt-BR` | `report-resources-prod-en-20260715-143000.pdf`; `...-prod-west-1-...`; `...-cluster-...`; `...-prod-attention-en-...`; `...-prod-pt-br-...`, each inside `report.output_directory` |
| `TestFromSnapshotWritesTheFile` (table) | each golden snapshot of `DataFileScenarios`, each language | the golden data file |
| `TestFromSnapshotMakesNoRequest` | a server is running | it received nothing |
| `TestFromSnapshotOverridesSeverity` | `--min-severity critical` on `partial-data` | the appendix holds two rows |
| `TestSnapshotThenFromSnapshotEqualsPDFMode` | `direct-complete`: the PDF mode; then `--snapshot` and `--from-snapshot` | the two outputs are equal |
| `TestModeConflicts` (table) | one row for each row of the first four messages, each flag in turn | code 2 and the message |
| `TestExecutiveIsRefused` (table) | `--report-profile executive`; `report.profile = "executive"` in the file; `KCR_REPORT_PROFILE=executive`; the same three in the mode from snapshot | code 2; `report profile "executive" is not available in this version` |
| `TestExitCodes` (table) | a missing configuration file; an unknown key; a server that answers 401; 503 every time; a TLS server with no bundle; `thanos-multicluster` with `--cluster-filter none`; `--cluster other`; a server where every `count(` gives nothing; no compiler; a compiler that answers `typst 0.15.0`; a compiler that exits 1 on `compile`; a compiler that prints `warning: x` | 2; 2; 3; 3; 3; 4; 4; 5; 6; 6; 6; 6, each with one line that starts with `error: ` |
| `TestFailurePrintsNothingOnStandardOutput` | every row of `TestExitCodes` | standard output is empty |
| `TestTypstIsProbedBeforeAnyRequest` | no compiler, PDF mode | code 6; the server received nothing |
| `TestOutputPathIsCheckedBeforeAnyRequest` (table) | `--output r.txt`; `--output <missing folder>/r.pdf`; `--output <a folder>.pdf` where that folder exists; `--snapshot s.txt` | code 6; the message; the server received nothing |
| `TestOutputReplacesAnExistingFile` | the path holds `old` | the new file; no `.tmp` file beside it |
| `TestFailedCompileKeepsTheOldFile` | the path holds `old`; the compiler exits 1 | code 6; the file still holds `old`; no `.tmp` file beside it |
| `TestSecretInARefusedAddress` | `--metrics-url 'https://user:hunter2@prom.example/?token=abc'` | code 2; neither stream holds `hunter2` or `abc` |
| `TestTokenNeverReachesAStream` (table) | token `s3cr3t-value` by `--token` and by variable; a server that echoes the `Authorization` header in a 500 body; a compiler that prints its environment and arguments and exits 1; `--verbose` | no stream holds `s3cr3t-value` |
| `TestTokenIsRedactedBeforeTheConfigurationLoads` | `--token s3cr3t-value --config <missing>` | code 2; no stream holds the token |
| `TestSnapshotFileRefusals` (table) | `--from-snapshot` with: a missing file; another schema version; an unknown key; two documents; a file cut in the middle | code 2; the message of `snapshot.Decode` |
| `TestPanicIsReportedAndRedacted` | `Env.Now` panics with a text that holds the token | code 1; `error: internal failure:`; no token |
| `TestCancelledContext` | the context is cancelled before the run | code 1; `error: interrupted`; no output file |
| `TestWorkFolderIsRemoved` (table) | a good run; a compile that fails; a panic inside the compile step | `Env.TempDir` is empty afterwards |
| `TestReportJSONIsWrittenInsideTheWorkFolder` | a compiler that records its arguments | `--root` is the work folder; `--input data=/report.json`; `--font-path` is inside the root |
| `TestProductionBinaryEmbedsZones` in `internal/guard` | `go list -deps ./cmd/k8s-capacity-report` | holds `time/tzdata` |

- [ ] **Step 1:** Write the tests and the fake compiler.
- [ ] **Step 2:** Run `go test ./internal/cli/ -v`. Expected: build failure.
- [ ] **Step 3:** Write `report.go`, `run.go`, `output.go`, and change
  `root.go` and `main.go`.
- [ ] **Step 4:** Run `go test -count=1 -race ./internal/cli/ ./internal/guard/`.
  Expected: pass.
- [ ] **Step 5:** Run `make build`, `make test`, and `make lint`, then
  commit.

```bash
git add internal/cli/ cmd/k8s-capacity-report/main.go \
  internal/guard/binary_test.go go.mod go.sum
git commit -m "feat(cli): add the report command"
```

**Done when:** the tests pass, and F-31, F-32, and F-33 name the commit.

---

### Task 15: A PDF from each scenario

**Executor:** Subagent for Step 1 to Step 4. Main thread for the rest,
because it needs Docker and page review.

**Files:**
- Create: `internal/cli/report_real_test.go`, build tag `template`
- Modify: `Makefile`: `test-template`, `check-runtime`
- Modify: `template/golden/snapshots.go`
- Create: `template/tests/golden/command-*.png`, and the new lines of
  `SHA256SUMS`

**Requirements:**

- `make test-template` also runs `./internal/cli/...`.
- `make check-runtime` also writes a PDF in the runtime image, as user
  10001, with a read-only root, `/tmp` as a temporary file system, the
  snapshot `testdata/snapshot/direct-complete.json` mounted read-only, and
  `.cache/check-runtime/` mounted as the only folder that outlives the run.
  It removes that folder first, and checks afterwards that the file starts
  with `%PDF-`.
- The golden pages of the command are chosen after the PDFs exist. The
  candidates are below; each entry of `Snapshots` states the one behaviour
  it alone protects, and one that protects nothing new is left out.

| Candidate | Data file | Would protect |
|---|---|---|
| `command-complete-pt` | `command/direct-complete.pt-BR.json` | the report of the command as a whole, in Portuguese: a numbered appendix that follows the opening page with no empty page between |
| `command-complete-en` | `command/direct-complete.en.json` | the same in English: date order and wording |
| `command-partial-en` | `command/partial-data.en.json` | the three statuses in the coverage column, and a remark that wraps |

**Tests, build tag `template`.** The compiler and `pdftotext` are real.

| Test | Input | Expected |
|---|---|---|
| `TestRealReportFromEachScenario` (table) | each of the six scenarios, each language, PDF mode | code 0; the file starts with `%PDF-`; the text holds `prod`, the cover title of the language, and the nine capability names of the language |
| `TestRealReportHasTwoPages` | `direct-complete` | the text holds two form feeds, one at the end of each page: the two page breaks in a row leave no empty page |
| `TestRealReportShowsPartialData` | `partial-data`, English | the text holds `The required metrics were not found.` and `Unavailable` |
| `TestRealReportFromSnapshotIsTheSame` | `direct-complete`: PDF mode, and the two-step path | the two PDFs have the same SHA-256 |
| `TestRealReportInFolderWithSpace` | `Env.TempDir` is `<temp>/my work folder` | code 0; that folder is empty afterwards |
| `TestRealReportWithAVeryLongName` | a cluster name of 400 characters | code 0; the text overflows without a warning, which is a known limit of the template; `Env.TempDir` is empty afterwards |
| `TestRealReportPrintsNamesAsTheyAre` | cluster `#prod *x* $y$ <a> @b [c]` | the text holds that name, character for character |
| `TestRealReportIsDeterministic` | the same run twice | the same SHA-256 |

- [ ] **Step 1:** Write `report_real_test.go`.
- [ ] **Step 2:** Run `go test -count=1 -tags=template ./internal/cli/ -v`
  with the native tools. Expected: pass, since Task 14 is complete; a
  failure here is a defect of an earlier task, to be reported and not
  worked around.
- [ ] **Step 3:** Change `test-template` and `check-runtime`.
- [ ] **Step 4:** Run `make lint`, then commit.

```bash
git add internal/cli/report_real_test.go Makefile
git commit -m "test(cli): write a PDF from each scenario"
```

- [ ] **Step 5 (main thread):** Run `make test-template`. Find each test
  name of the table in `.cache/gate/` or in the output. A name that is
  missing means a filter hides it.
- [ ] **Step 6 (main thread):** See the new gate step fail. Run
  `make check-runtime` with the path of the snapshot changed to a file that
  does not exist, confirm that it fails, and restore the path.
- [ ] **Step 7 (main thread):** Write the PDFs for the owner.

```bash
mkdir -p .cache/phase-2/part-1/pdf
make build
for s in direct-complete grafana-authenticated thanos-multicluster \
  raw-fallback partial-data value-edge-cases; do
  for l in pt-BR en; do
    bin/k8s-capacity-report report --from-snapshot testdata/snapshot/$s.json \
      --lang $l --output .cache/phase-2/part-1/pdf/$s.$l.pdf
  done
done
```

- [ ] **Step 8 (main thread):** Open the twelve PDFs. Choose the golden
  pages, add their entries to `template/golden/snapshots.go`, and run
  `make update-golden`. Confirm by `git status` that no existing page
  changed, the pages of `capacity-parity` above all. Review each new page.
- [ ] **Step 9 (main thread):** Run `make test-golden`, then commit.

```bash
git add template/golden/snapshots.go template/tests/golden/
git commit -m "test(golden): add the pages of the command"
```

**Done when:** the tagged tests pass in the image, the new gate step was
seen failing, the twelve PDFs exist, and every new golden page was reviewed.

---

### Task 16: Documents

**Executor:** Main thread. Wording stays on the main thread.

**Files:**
- Modify: `README.md`, `AGENTS.md`, `CHANGELOG.md`

**Requirements:**

- `README.md`: "What exists today" names the command and its four modes.
  A new section, "Generating a report", shows a minimal `report.toml`, the
  four commands of spec 5.1, the exit codes, and how the token is given.
  The section "Compiling a report by hand" is removed, because the command
  replaces it. The table of commands gains `make update-testdata`.
- `AGENTS.md`: the table of layers and its ranks; the wording guard and its
  comment; the guard on the production binary; `make update-testdata` and
  the rule that a golden file is read whole before it is committed; the
  rule that the probes keep the order of the original.
- `CHANGELOG.md`, under `## [Unreleased]`, group `### Added`. Each item is
  one sentence about what a user perceives:
  - The `report` command checks a Prometheus-compatible endpoint, the
    cluster scope, and the available metrics with `--check`.
  - The `report` command writes a PDF with the cover, the report metadata,
    the status legend, and the data quality appendix, in Portuguese (Brazil)
    or English.
  - The `report` command stores what it found in a snapshot file with
    `--snapshot`, and writes a PDF from that file with `--from-snapshot`,
    without any request to the endpoint.
  - The configuration is read from a TOML file, then from the environment,
    then from the command line.
- Left out of the changelog, and stated in the closing report: the guards,
  the fake server, the golden files, the fix of F-1, and the fix of F-9,
  whose messages no user has seen, since no command called the compiler
  before.

- [ ] **Step 1:** Write the three files.
- [ ] **Step 2:** Run every command that `README.md` shows, against
  `fake-prometheus`, and confirm that each does what the text says.
- [ ] **Step 3:** Run `make test` and `make lint`, then commit.

```bash
git add README.md AGENTS.md CHANGELOG.md
git commit -m "docs: describe the report command"
```

**Done when:** each command of `README.md` was run as written.

---

### Task 17: The preflight against a real cluster

**Executor:** The owner, or the main thread with the owner's approval for
each command. This task reaches a real system with a real credential.

**Requirements:**

- The owner provides a `report.toml` outside the repository and names the
  variable that holds the token. The token is never written to a file of
  the repository, to `.cache/`, or to a message.
- The run is `--check` first, then `--snapshot`, then `--from-snapshot` in
  both languages.

- [ ] **Step 1:** Ask the owner for approval and for the path of the
  configuration.
- [ ] **Step 2:** Run the check.

```bash
bin/k8s-capacity-report report --config <path> --check --verbose
```

- [ ] **Step 3:** Run the snapshot and the two PDFs into
  `.cache/phase-2/part-1/real/`.
- [ ] **Step 4:** Read the snapshot for any address, host name, or provider
  identifier. Finding one is a defect of spec 8.3.
- [ ] **Step 5:** Record every finding in `ROADMAP.md` with the source
  `PROD`. A finding from this task takes precedence over one from reading
  code.

**Done when:** the three runs ended, or the owner has decided to leave this
task for part 2.4, where the spec places the proof on a real cluster. Either
outcome is recorded in `ROADMAP.md`.

---

### Task 18: Close the part

**Executor:** Main thread.

- [ ] **Step 1:** Hand the English wording of this part to the owner: the
  tables of Task 10, as `.cache/phase-2/part-1/english-wording.md`, with the
  twelve PDFs. Apply what the owner changes, run `make update-testdata` and
  `make update-golden`, and review what changed.
- [ ] **Step 2:** Write the diff of the branch to
  `.cache/phase-2/part-1/branch.diff` and run the `adversarial-reviewer`
  agent over it. It has no `ReportFindings` tool as a subagent, so the
  brief asks for the findings in `.cache/phase-2/part-1/adversarial.md` and
  in its last message.
- [ ] **Step 3:** Record every finding in `ROADMAP.md`. Fix the ones
  assigned to this part, each with a brief that names the class. Run the
  review again over the fixes.
- [ ] **Step 4:** Run `make gate`. Keep the logs of `.cache/gate/` in
  `.cache/phase-2/part-1/gate/`.
- [ ] **Step 5:** Update `ROADMAP.md`: every item `DONE` with its commit;
  the summary line of 2.1; the rulings R1 to R16 with what the owner said of
  each. Read `ROADMAP.md` and `CHANGELOG.md` back.
- [ ] **Step 6:** Ask the owner for the merge and for the decision on the
  version. The recommendation is to merge with no new version and no tag:
  the snapshot of schema `1.0` is not complete before part 2.4, and a
  release would freeze it.
- [ ] **Step 7:** After the owner approves, merge by fast-forward.

```bash
git switch main
git merge --ff-only feat/command-spine
```

**Done when:** `make gate` passed on the last commit, the review has no open
finding assigned to this part, and the owner approved the merge.

---

## Self-review of this plan

| Spec | Task |
|---|---|
| 4.4 imports, wording, metrics only | 1, 12, and the guards of phase 0 |
| 5.1 usage, four modes, atomic output | 14 |
| 5.2 configuration | 4 |
| 5.3 exit codes | 14 |
| 5.4 redaction | 3, 14 |
| 6.1 client | 6 |
| 6.2 cluster scope | 7 |
| 6.3 capabilities | 8 |
| 6.4 collectors and resolvers | part 2.2 and 2.3 |
| 7 analysis | part 2.2 to 2.4 |
| 8 snapshot: `report`, `scope`, `quality` | 9 |
| 8 snapshot: the other sections; 8.2 warnings and notices | part 2.2 to 2.4 |
| 8.3 the digest | phase 3 (R5) |
| 9.1 content, for what this part prints | 10 |
| 9.2 formats: date and time | 10 |
| 9.2 formats: the others | part 2.2 and 2.3 |
| 9.3 vocabulary | 10 |
| 10.1, 10.2 data file | 11 |
| 10.3 content: opening page and appendix | 11 |
| 10.3 content: the sections | part 2.2 to 2.4 |
| 10.5 compile | 14, 15 |
| 13.2 scenarios, snapshot goldens, data file goldens, catalog guards | 5, 13, 10, 12 |
| 16 proof: a PDF from each scenario | 15 |
| 16 proof: a real cluster | 17, and again in part 2.4 |
| 17 the English wording reviewed | 18 for this part; complete in part 2.4 |
| 17 the comparison with the Python tool | part 2.2 onwards |

Known limits of this plan:

- The expected results of the scenarios in Task 8 were worked out by reading
  the scenario files, not by running the original. Task 8 says what to do
  when they disagree with the code.
- The column that Task 4 pins for a TOML error depends on the library and is
  fixed by observation.
