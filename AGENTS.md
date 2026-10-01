# AGENTS.md

This file guides LLMs and people working in this repository.

`k8s-capacity-report` generates Kubernetes capacity reports from a
Prometheus-compatible API, using metrics only. The design is in
`docs/superpowers/specs/2026-09-27-k8s-capacity-report-design.md`, and it is
the authority when this file and the code disagree with it.

## Environment

- Development happens on macOS with native Go 1.27.
- `typst` 0.15.1 and `pdftotext` are installed natively for fast feedback.
- Anything whose result must be exact runs in the Linux test image, through
  Docker. A native result never closes a gate.
- There are no Nix files in this repository.
- Scratch files go under `.cache/`, never under `/tmp` on the host.
- `older/` holds the two original projects as read-only reference. It is
  never tracked and never edited.

## Commands

```bash
make build            # bin/k8s-capacity-report
make test             # Go tests that need no service and no tool
make lint             # gofmt, go vet, golangci-lint
make image            # runtime image and test image
make test-toolchain   # checks typst and pdftotext inside the test image
make test-template    # compiles the template and writes reports in the test image
make test-golden      # compares rendered pages with the golden pages
make gate             # everything above, from freshly built images
make update-golden    # rewrites the golden pages after an intended change
make update-testdata  # rewrites the golden snapshots and data files
```

`make gate` writes one log per step under `.cache/gate/`. It builds the images
without cache and runs the tests without cached results.

Run one test while iterating:

```bash
go test ./internal/guard/ -run TestForbiddenModules -v
```

## Rules enforced by a guard

Each of these fails `make test` or `make image` when broken. The guards live
in `internal/guard`, except the Typst pin, which `internal/typst` and the
image build enforce.

- **No test skips.** `Skip`, `Skipf`, and `SkipNow` are forbidden in test
  files and in any file that imports `testing`. A test that needs a tool
  carries a build tag and fails when the tool is missing. A skipped gate is
  not a checked gate.
- **Every build tag is declared.** `guard.BuildTags` lists them. A tag that is
  not in the list fails the guard, so that no file hides from the scans. The
  legacy `// +build` form is forbidden; write `//go:build`.
- **Metrics only.** No Kubernetes or cloud client may appear in `go.mod`.
- **Imports point one way.** `guard.Layers` holds the table of layers. A new
  package must be added to that table. Nothing outside `cmd/` imports a
  package under `cmd/`, and the root package imports nothing of this module.
  An import of a package that the table does not hold is reported at the
  import, whatever imports it. An import made only by a test file is outside
  the rule: a test of a low package may use a higher one.
- **Reader text lives in the catalog.** A guard reads every string literal of
  the Go files under `internal/` and `cmd/`, test files left out. It reports
  a literal with a letter outside ASCII. In `internal/document` it also
  reports two words with only white space or percent signs between them, a word
  that starts with a capital letter followed by a lower-case one, and a
  format verb beside a word. An argument of `errors.New`, `fmt.Errorf`, or a
  function whose name ends in `Errorf` is a diagnostic and is left alone, and
  so is the name of a parameter in a `catalog.Params`. A literal that the
  guard takes wrongly for text carries the comment
  `// wording: not reader text, <reason>` at the end of its line.
- **The catalog agrees with itself.** Every key exists in both languages with
  the same parameters, and every code of the registries of
  `internal/discovery` has its entry. Every key is used: a key of a family,
  such as `state.available`, by being a code of its registry, and every other
  key by being named in `internal/document`. An entry uses
  the one template form `{{.Name}}`. No entry holds a banned term, and none
  holds an em-dash unless it is the em-dash alone.
- **The production binary holds no test support.** `internal/promtest`,
  `internal/goldentest`, `cmd/fake-prometheus`, and `cmd/update-testdata` are
  not among its dependencies, and `time/tzdata` is.
- **Typst is pinned at exactly 0.15.1.** The toolchain probe refuses any other
  version, and the image build asserts it.
- **One version.** `VERSION` holds it, and the newest changelog heading must
  equal it. The only level-two headings the changelog may hold are
  `## [Unreleased]` and `## [X.Y.Z] - YYYY-MM-DD`, and it holds no heading
  written as text over a line of dashes.

## Rules kept by review

No guard checks these. The reviewer of each task does.

- **Diagnostics are in English.** This covers errors, logs, and help text.
- **Technical terms are English in both languages.** A Portuguese sentence
  writes request, limit, workload, node, node pool, label, replica,
  autoscaling, recording rule, raw metrics, and throttling as English words,
  masculine, with an English plural. Only a common noun is Portuguese. The
  guard bans the translations; the reviewer judges the border cases against
  the table of spec 9.3.
- **Nothing else declares a version** without a test that compares it with
  `VERSION`.
- **The wording guard reads text, not meaning.** It lets through one word
  alone, words parted by digits or a slash, and a sentence joined from parts.
  The reviewer of a task reads the string literals of `internal/document`,
  and the golden data files show every text that reaches a page.
- **No comment speaks of the process.** A comment says what the code does. It
  names no task, step, round, ruling, brief, or role, and no path under
  `.cache/`. It may name a section of the design and the original program.
- **A ratio is worked out sample by sample.** Consumption over request, and
  over limit, is computed at each timestamp and then reduced. It is never
  the ratio of two averages, which gives other numbers than the original.
- **The numbers equal the original exactly.** Two numbers are equal when
  they are the same `float64`. A test of the comparison has no tolerance,
  and an expected number in a test is a literal, never the expression of
  the code written again.

Build tags in use:

| Tag | Needs | Runs in |
|---|---|---|
| `toolchain` | `typst`, `pdftotext` | `make test-toolchain` |
| `template` | `typst`, `pdftotext` | `make test-template`, which also runs `internal/cli` |
| `golden` | `typst`, with `template` | `make test-golden` |

With the native tools installed, a subagent runs the tagged tests itself:

```bash
go test -count=1 -tags=template ./template/ ./internal/typst/ ./internal/cli/
go test -count=1 -tags='template golden' ./template/ -run 'Golden|Snapshots'
```

## The layers

`guard.Layers` holds this table. A package imports only packages of a lower
rank, or of its own layer.

| Rank | Packages |
|---|---|
| 0 | `internal/guard`, `internal/arith`, `internal/typst`, `internal/redact`, `internal/promtest`, `template` |
| 10 | `template/golden` |
| 20 | `internal/config` |
| 30 | `internal/prom` |
| 40 | `internal/discovery` |
| 50 | `internal/resolve` |
| 55 | `internal/collect` |
| 60 | `internal/analysis` |
| 70 | `internal/snapshot` |
| 80 | `internal/catalog` |
| 90 | `internal/document` |
| 95 | `internal/goldentest` |
| 100 | `internal/cli` |

The snapshot is the only handover. Everything after `snapshot.Build` reads
the snapshot and nothing else.

## The command

- **One redactor for the whole run.** `cli.Run` makes it on its first line
  and wraps both streams with it. Nothing below `Run` sees a raw stream. A
  secret is added to it as soon as it is known: the value of `--token` before
  the flags are read, the token of the configuration as soon as it is loaded.
  No function passes a list of secrets.
- **One message, one write.** The redactor reads each write as one text, so a
  line is built whole and written once.
- **No message repeats an argument that was not understood.** It may be a
  secret typed at the wrong place. A path that the user gave is repeated on
  purpose, and so is a value read from a file that the user named.
- **The program has the commands the design names, and no other.** The
  completion command that the command-line library adds is turned off. A
  test names the commands it expects and runs each with a stray argument.
- **Every failure prints one line**, `error: <message>`, on standard error,
  and nothing on standard output.
- **Output files are written beside their target and renamed.** They follow
  the umask, which is never read or set. A file that the compiler wrote is
  renamed only when it starts as a PDF does.
- **A token is at least 8 characters long.** A shorter one cannot be told
  from ordinary text, and is refused before it reaches the redactor.
- **The snapshot file has one reading.** The reader walks the file against
  the contract, letter for letter, and fills the snapshot from what it
  walked. It names a wrong schema version before anything else.
- **The probes keep the order of the original**, short cuts included. The
  fake server answers a query with the first recorded answer whose text is
  part of the query, so another order consumes the wrong answers.
- **The collectors keep the order of the original too:** the namespaces,
  then the workloads, then the HPAs and their coverage, then the nodes, then
  the cluster autoscaling, and inside each the queries in its order. The text of
  every query equals the original letter for letter.
- **`--check` is the preflight and nothing else.** It sends no query of a
  collector, and the requests it sends are the first requests of a run.
- **A run with nothing to report stops with exit code 5.** When the five
  collectors return no row and the status of the cluster autoscaling is
  unknown, the run stops after the collectors and before the clock is read
  the second time.
- **The period is judged before the first request.** A period whose start is
  not before its end stops the run with exit code 2.
- **The clock is read twice in a run that collects:** for the emission time,
  which is also the start of the collection, and for its end.

### Golden snapshots and data files

- `testdata/snapshot/` holds one snapshot for each scenario, and
  `testdata/queries/` the queries the program sent for it, in order. The
  recorded answers match a query by a part of its text, so only the list of
  queries can show that a probe asks what it should.
  `template/tests/capacity/fixtures/command/` holds the data files made from
  them, which are also fixtures of the template and sources of golden pages.
- A test never writes one. `make update-testdata` does. It needs no image.
- After `make update-testdata`, read every file that changed, whole, before
  committing. A value that cannot be explained is a defect.
- The files of `testdata/prometheus/` are the recorded answers of the
  original, byte for byte. They are never edited.
- `testdata/prometheus-own/` holds the scenarios that this project wrote,
  for values that no recorded answer holds. It holds two scenarios,
  `edge-values` and `edge-scaling`. Its README says what each
  answer exercises. A file there is not edited once its recorded numbers
  exist.
- `testdata/python/` holds what the original computes and asks for each of
  the eight scenarios: its numbers for every section the program computes,
  and its queries in order. The recording script lives under
  `.cache/phase-2/part-3/python/`. A test compares this
  project with them on every run. They are never edited; they are replaced
  only by recording the original again, and its README says how.
- A test that walks the scenarios or reads a list of queries counts what it
  means to count. A filter that matched one thing when it was written
  matches more when the program asks more.

## The template

`template/graphite/` is the generic page shell, a port of `graphite-report`
1.0.0 that stays a valid Typst package. `template/capacity/` is the layout of
the capacity report. Go decides what is said; the template decides how it
looks.

- The report layer reads one JSON data file. Its contract is in the plan of
  phase 1, and `template/capacity/validate.typ` enforces it.
- `template.Unpack` writes only into a fresh folder: one that does not exist,
  or exists and is empty, and is not a symbolic link. The caller creates a new
  folder for each run and discards it afterwards.
- Every word on the page arrives in the data file. The template adds no
  wording of its own and never evaluates a value as markup. `eval` is
  forbidden, and a test scans for it.
- Every diagnostic of the generic layer starts with `graphite-report: `, and
  every diagnostic of the report layer with `capacity-report: `. All of the
  report layer's diagnostics live in one dictionary in `validate.typ`, and
  each has a fixture that triggers it. A data file that is not valid JSON
  stops with Typst's own message, because Typst cannot catch a parse
  error; the command that writes the file answers for its syntax.
- No page may contain a coloured pixel.
- The public surface of the generic layer is frozen: `graphite-report`,
  `callout`, `keep-together`, with the keys its README documents.

### Golden pages

- `template/golden/snapshots.go` is the one list of golden pages. Each entry
  states the one behaviour it alone protects.
- A snapshot covers every page of its document, or says in `Omits` which
  pages it leaves out and why. A test compares the list with the page count.
- Pages are rendered at 144 PPI and compared by SHA-256.
- A test never writes a golden page. `make update-golden` does, and only for
  an intended visual change.
- After `make update-golden`, open every page that changed and review it
  before committing. A page that changed without a reason you can name is a
  defect, not a new golden page.
- The pages of the snapshot `capacity-parity` equal, byte for byte, the pages
  of the original report. A change that alters them has moved away from the
  original, and needs a decision recorded in `ROADMAP.md`.
- Rendering was measured equal on linux/amd64, linux/arm64, and macOS
  (`docs/decisions/0001-golden-architecture.md`). Measure again when the
  pinned Typst version changes.

## Delegating to subagents

The main thread plans, judges, and runs the gates. Subagents implement.

- A subagent may run `go`, `make build`, `make test`, and `make lint`.
- A subagent must not run Docker, so `make image`, `make test-toolchain`,
  `make test-template`, `make test-golden`, `make update-golden`, and
  `make gate` stay on the main thread.
- Two agents in one checkout break each other's builds when they share a Go
  package. Give a reviewer a diff file, and let it run tests only on packages
  nobody is editing.
- A subagent cannot declare a task or a phase finished.
- Interface design, API design, and catalog wording stay on the main thread.
- Tasks that share a file run in sequence. When in doubt, treat two tasks as
  sharing: an unneeded sequence costs time, a silent overwrite costs the work.
- Every commit names its paths: `git add <paths>`, never `git add -A` and
  never `git commit -a`.
- A subagent never runs `git stash`, `git restore`, `git checkout` of a
  path, or `git clean`. The checkout holds changes of the main thread that
  are not its own.
- A brief is cut from the plan. When an implementer finds that the brief and
  the original disagree, it stops and reports; the main thread runs the
  original, rules, corrects the plan, and cuts the brief again.
- After a round of parallel work, the full suite runs once over the combined
  result.
- An adversarial review runs over the whole branch before a phase is merged.

## Phases and branches

Work is organised in phases, one implementation plan per phase, under
`docs/superpowers/plans/`.

1. **Before opening a phase,** read `ROADMAP.md` and `CHANGELOG.md` and
   confirm they describe the real state. Correct them first.
2. **Opening a phase is creating its branch.** Nothing is edited on `main`.
   Confirm with `git branch --show-current` before touching a file.
3. **Integration is by phase, not by item.** The branch is merged into `main`
   by fast-forward when the last item of the phase closes.
4. **Before merging:** run `make gate`; update `ROADMAP.md`; review
   `CHANGELOG.md`; read both files back to confirm they changed.

## ROADMAP.md

`ROADMAP.md` sits at the repository root and is not tracked. It is a working
tool, not a deliverable. If it does not exist, create it with this structure
before starting any work:

- **How to use** and the **legend**. Status: `DONE`, `OPEN`, `PARTIAL`,
  `DEFERRED`, `BLOCKED`, `REMOVED`. Source: `PLAN` item of an implementation
  plan, `REV` code review, `PROD` real execution, `JUDGE` review of an
  agent's work.
- A summary table with one line per phase.
- One section per phase. Each item has a stable identifier, a source, a
  one-line description, and `file:line`.
- **Findings log**, where every new finding is recorded before it receives a
  phase.
- **Learned process rules**, so that an orchestration mistake is not repeated.

Rules:

- Every finding goes into `ROADMAP.md`, wherever it appeared. A finding
  without a phase becomes invisible debt.
- A `DONE` item names the commit that closed it. A `DONE` item with no test
  does not count.
- A finding from real execution takes precedence over one from reading code.
- Identifiers are stable and never reused.
- `REMOVED` is for an item whose target left the project.

## Changelog and version

`CHANGELOG.md` is tracked. It follows Keep a Changelog 1.1.0 and Semantic
Versioning.

- Entries are grouped under `### Added`, `### Changed`, `### Deprecated`,
  `### Removed`, `### Fixed`, and `### Security`. A group appears only when it
  has items.
- A version heading is `## [X.Y.Z] - YYYY-MM-DD`. A guard rejects any other
  form.
- Each item is one sentence about what a user perceives. No function names,
  no commit hashes, no internal terms.
- Work with no effect for users is left out: tests, internal documents,
  repository settings. State each omission when reporting, because a silent
  omission cannot be told from forgetting.
- Merging and raising the version are separate decisions. A phase made only
  of tests or internal work is merged with no new version and no tag.
- The newest version heading equals `VERSION`. A guard checks it.
- A defect fix is a patch. A change to when a finding appears, to a sentence
  of the report, or to a visual default is a minor version.
- A tag is `vX.Y.Z`, annotated, and created only after `make gate` passes on
  that commit. Its logs are kept under `.cache/release-X.Y.Z/`.

## Commits

Conventional format, `type(scope): description`, imperative, subject under 50
characters. Types: `feat`, `fix`, `docs`, `style`, `refactor`, `perf`, `test`,
`chore`, `ci`, `build`. The body, when needed, uses `*` bullets.
