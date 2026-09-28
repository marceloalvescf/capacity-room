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
make gate             # everything above, from freshly built images
```

`make gate` writes one log per step under `.cache/gate/`. It builds the images
without cache and runs the tests without cached results.

Run one test while iterating:

```bash
go test ./internal/guard/ -run TestForbiddenModules -v
```

## Rules enforced by a guard

Each of these fails `make test` when broken. The guards live in
`internal/guard`.

- **No test skips.** `Skip`, `Skipf`, and `SkipNow` are forbidden in test
  files and in any file that imports `testing`. A test that needs a tool
  carries a build tag and fails when the tool is missing. A skipped gate is
  not a checked gate.
- **Every build tag is declared.** `guard.BuildTags` lists them. A tag that is
  not in the list fails the guard, so that no file hides from the scans.
- **Metrics only.** No Kubernetes or cloud client may appear in `go.mod`.
- **Imports point one way.** `guard.Layers` holds the table of layers. A new
  package must be added to that table. Nothing outside `cmd/` imports a
  package under `cmd/`, and the root package imports nothing of this module.
- **Typst is pinned at exactly 0.15.1.** The toolchain probe refuses any other
  version, and the image build asserts it.
- **One version.** `VERSION` holds it, and the newest changelog heading must
  equal it.

## Rules kept by review

No guard checks these. The reviewer of each task does.

- **Diagnostics are in English.** This covers errors, logs, and help text.
- **Nothing else declares a version** without a test that compares it with
  `VERSION`.
- **Reader-facing report text lives in the catalog only**, once it exists.

Build tags in use:

| Tag | Needs | Runs in |
|---|---|---|
| `toolchain` | `typst`, `pdftotext` | `make test-toolchain` |

## Delegating to subagents

The main thread plans, judges, and runs the gates. Subagents implement.

- A subagent may run `go`, `make build`, `make test`, and `make lint`.
- A subagent must not run Docker, so `make image`, `make test-toolchain`, and
  `make gate` stay on the main thread.
- A subagent cannot declare a task or a phase finished.
- Interface design, API design, and catalog wording stay on the main thread.
- Tasks that share a file run in sequence. When in doubt, treat two tasks as
  sharing: an unneeded sequence costs time, a silent overwrite costs the work.
- Every commit names its paths: `git add <paths>`, never `git add -A` and
  never `git commit -a`.
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
