# Phase 0: Foundation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** A repository where `make gate` passes: one Go binary that prints its version, guards that protect the structure, a container image with the pinned Typst, and the working documents.

**Architecture:** One Go module with a root package that owns the version, a `cmd/` entry point, and `internal/` packages. The container image is built in stages and carries the Go binary and `typst` 0.15.1. Every check is a `make` target, and `make gate` runs them all.

**Tech Stack:** Go 1.27, Cobra 1.10, golangci-lint 2.14, Docker with Buildx, Typst 0.15.1.

**Spec:** `docs/superpowers/specs/2026-09-27-k8s-capacity-report-design.md`

## How to read this plan

The owner's orchestration rule applies: the plan states what each task must
deliver and how it is proven, and the implementer writes the code. Each task
gives exact paths, exact signatures, and a table of tests with inputs and
expected results. Write each test first, watch it fail, then implement.

Every task names its executor:

- **Subagent**: a Sonnet subagent. It may run `go` and `make test`, `make lint`.
  It must not run Docker.
- **Main thread**: needs Docker, judgment, or wording.

## Global Constraints

- Go 1.27. Module path `github.com/marceloalvescf/k8s-capacity-report`.
- Typst is pinned at exactly `0.15.1`. Any other version is a failure.
- Base image for Typst: `ghcr.io/typst/typst:0.15.1`.
- The container runs as user `10001`.
- No test skips. A test that needs a tool is selected by a build tag and fails
  when the tool is missing. `t.Skip` is forbidden in this repository.
- No Kubernetes or cloud client may appear in `go.mod`.
- `older/`, `.cache/`, and `ROADMAP.md` are never tracked.
- Repository documents are in English.
- Commits follow `type(scope): description`, imperative, subject under 50
  characters.
- Operator diagnostics (errors, logs, help text) are in English.
- Never write scratch files to `/tmp` on the host. Use `.cache/`.

## Review Focus

1. **`VERSION` with a trailing newline or spaces.** Editors add them. The
   version must still read as `0.0.0`. Pinned in Task 1.
2. **`VERSION` with content that is not a version**, such as `v0.1.0` or an
   empty file. The build must fail with a clear message, not print garbage.
   Pinned in Task 1.
3. **A `typst` that exists but is the wrong version, or prints something
   unexpected.** The probe must name the version it found. Pinned in Task 5.
4. **A forbidden dependency that arrives indirectly.** `go.mod` lists indirect
   requirements too, and the guard must read them. Pinned in Task 3.
5. **A new package that nobody classified.** The import guard must fail on an
   unknown package, not ignore it. Pinned in Task 3.

---

## File structure

| Path | Responsibility |
|---|---|
| `VERSION` | The version, one line |
| `version.go` | Root package `kcr`; embeds and validates `VERSION` |
| `version_test.go` | Tests for the above |
| `cmd/k8s-capacity-report/main.go` | Calls `cli.Run` and exits with its code |
| `internal/cli/root.go` | Root command and `--version` |
| `internal/cli/root_test.go` | Tests for the above |
| `internal/guard/gomod.go` | Reads `go.mod`, finds forbidden modules |
| `internal/guard/imports.go` | Import direction rule |
| `internal/guard/changelog.go` | Reads the newest version heading |
| `internal/guard/*_test.go` | Rule tests and the real scans |
| `internal/typst/toolchain.go` | Locate the binary, read and check its version |
| `internal/typst/toolchain_test.go` | Tests with a fake binary |
| `internal/typst/toolchain_real_test.go` | Build tag `toolchain`; real binary |
| `deploy/Containerfile` | Image stages |
| `.dockerignore` | Keeps `older/`, `.cache/`, `.git/` out of the build |
| `.golangci.yml` | Linter settings |
| `Makefile` | Every check |
| `README.md`, `AGENTS.md`, `CLAUDE.md`, `CHANGELOG.md`, `LICENSE` | Documents |
| `docs/decisions/0001-golden-architecture.md` | Result of Task 7 |

---

### Task 0: First commit and phase branch

**Executor:** Main thread.

**Files:**
- Modify: `.gitignore`

- [ ] **Step 1: Add the untracked working file to `.gitignore`**

`.gitignore` must contain exactly:

```gitignore
# Original projects kept as local reference only; never tracked.
/older/
# Working record of findings; a tool, not a deliverable.
/ROADMAP.md
# Build output.
/bin/
```

- [ ] **Step 2: Confirm nothing unwanted is staged**

Run: `git add -A && git status --short`
Expected: only `.gitignore` and files under `docs/`. No path under `older/`,
`.cache/`, or `.claude/`.

- [ ] **Step 3: Commit on `main`**

```bash
git commit -m "docs: add design spec and references"
```

- [ ] **Step 4: Open the phase branch**

```bash
git switch -c chore/foundation
git branch --show-current
```

Expected: `chore/foundation`. Nothing else in this plan is done on `main`.

---

### Task 1: Module, version, and command skeleton

**Executor:** Subagent.

**Files:**
- Create: `go.mod`, `VERSION`, `version.go`, `version_test.go`
- Create: `cmd/k8s-capacity-report/main.go`
- Create: `internal/cli/root.go`, `internal/cli/root_test.go`

**Interfaces:**
- Consumes: nothing.
- Produces:
  - package `kcr` at the module root:
    `func Version() string` and
    `func ParseVersion(raw string) (string, error)`.
  - package `cli`:
    `func Run(args []string, stdout, stderr io.Writer) int`.

**Requirements:**

- `VERSION` contains `0.0.0` followed by one newline.
- `version.go` embeds `VERSION` with `//go:embed`.
- `ParseVersion` trims surrounding white space and accepts only
  `^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$`. On failure the error
  text is `invalid VERSION "<raw trimmed>": want MAJOR.MINOR.PATCH`.
- `Version` returns the parsed embedded value and panics with the same message
  when the file is invalid. A broken version must stop the program at start.
- `cli.Run` builds a Cobra root command named `k8s-capacity-report`.
  - `--version` prints `k8s-capacity-report <version>` and a newline to
    `stdout`, returns `0`.
  - No arguments prints the help to `stdout`, returns `0`.
  - An unknown command or flag prints the error to `stderr`, returns `2`.
  - Cobra's own error and usage printing is silenced; `Run` prints once.
- `main.go` is three lines: call `cli.Run(os.Args[1:], os.Stdout, os.Stderr)`
  and pass the result to `os.Exit`.

**Tests:**

| Test | Input | Expected |
|---|---|---|
| `TestParseVersionAcceptsPlainVersion` | `"1.2.3"` | `"1.2.3"`, no error |
| `TestParseVersionTrimsWhitespace` | `" 0.0.0\n"` and `"0.0.0\r\n"` | `"0.0.0"` |
| `TestParseVersionRejects` (table) | `""`, `"v0.1.0"`, `"1.2"`, `"1.2.3.4"`, `"01.2.3"`, `"1.2.3-rc1"`, `"1.2.3\n4.5.6"` | error containing `want MAJOR.MINOR.PATCH` |
| `TestVersionMatchesFile` | reads `VERSION` from disk | equals `Version()` |
| `TestRunVersionFlag` | `["--version"]` | `stdout` is `k8s-capacity-report 0.0.0\n`, code `0`, `stderr` empty |
| `TestRunNoArgumentsPrintsHelp` | `[]` | `stdout` contains `Usage:`, code `0` |
| `TestRunUnknownCommand` | `["nope"]` | code `2`, `stderr` contains `unknown command "nope"`, `stdout` empty |
| `TestRunUnknownFlag` | `["--nope"]` | code `2`, `stderr` contains `unknown flag` |

- [ ] **Step 1:** `go mod init github.com/marceloalvescf/k8s-capacity-report`,
  set `go 1.27` in `go.mod`, and add `github.com/spf13/cobra@v1.10.2`.
- [ ] **Step 2:** Write `version_test.go` with the four version tests.
- [ ] **Step 3:** Run `go test . -run 'Version' -v`. Expected: build failure,
  `undefined: ParseVersion`.
- [ ] **Step 4:** Write `VERSION` and `version.go`.
- [ ] **Step 5:** Run `go test . -v`. Expected: PASS.
- [ ] **Step 6:** Write `internal/cli/root_test.go` with the four `Run` tests.
- [ ] **Step 7:** Run `go test ./internal/cli/ -v`. Expected: build failure,
  `undefined: Run`.
- [ ] **Step 8:** Write `internal/cli/root.go` and `cmd/k8s-capacity-report/main.go`.
- [ ] **Step 9:** Run `go test ./... -v`. Expected: PASS.
- [ ] **Step 10:** Run `go run ./cmd/k8s-capacity-report --version`.
  Expected: `k8s-capacity-report 0.0.0`.
- [ ] **Step 11:** Commit.

```bash
git add go.mod go.sum VERSION version.go version_test.go cmd internal/cli
git commit -m "feat(cli): add module, version, and root command"
```

---

### Task 2: Repository documents and changelog guard

**Executor:** Main thread writes the documents. Subagent writes the guard.

**Files:**
- Create: `README.md`, `AGENTS.md`, `CLAUDE.md`, `CHANGELOG.md`, `LICENSE`
- Create: `internal/guard/changelog.go`, `internal/guard/changelog_test.go`

**Interfaces:**
- Consumes: `kcr.Version()` from Task 1.
- Produces: package `guard`:
  `func NewestVersionHeading(changelog string) (version string, found bool, err error)`.

**Document requirements:**

- `LICENSE`: the MIT licence, copyright `2026 Marcelo Alves`.
- `CHANGELOG.md`: a title, one paragraph naming Keep a Changelog 1.1.0 and
  Semantic Versioning, and one heading `## [Unreleased]`. No version heading.
- `README.md`: what the project is, in three sentences; the requirement list
  (Go 1.27, Docker); the commands `make test`, `make lint`, `make gate`; a
  pointer to the spec. It documents only what exists at the end of phase 0.
- `AGENTS.md`: the working agreement from spec section 15, plus:
  environment (macOS, native Go, Docker for exact-output checks), the list of
  `make` targets, which steps subagents may run, the rule that no test skips,
  and the `ROADMAP.md` structure (status legend `DONE`, `OPEN`, `PARTIAL`,
  `DEFERRED`, `BLOCKED`, `REMOVED`; sources `REV`, `PROD`, `JUDGE`; one
  section per phase; findings log; learned process rules).
- `CLAUDE.md`: one line, `@AGENTS.md`.
- `ROADMAP.md` is created from that structure and stays untracked.

**Guard requirements:**

- A version heading is a line matching
  `^## \[(\d+\.\d+\.\d+)\] - \d{4}-\d{2}-\d{2}$`.
- `NewestVersionHeading` returns the first such heading from the top.
- A line that starts with `## [` and a digit but does not match the full form
  is an error: `malformed changelog heading on line <n>: "<line>"`.
- `## [Unreleased]` is allowed and ignored.

**Tests:**

| Test | Input | Expected |
|---|---|---|
| `TestNewestVersionHeadingNone` | only `## [Unreleased]` | `found == false`, no error |
| `TestNewestVersionHeadingFirstWins` | `## [Unreleased]`, `## [0.2.0] - 2026-10-01`, `## [0.1.0] - 2026-09-30` | `"0.2.0"`, `true` |
| `TestNewestVersionHeadingMalformed` (table) | `## [0.2.0]`, `## [0.2] - 2026-10-01`, `## [0.2.0] - 2026-10-1` | error naming the line number |
| `TestNewestVersionHeadingIgnoresDeeperHeadings` | `### [0.9.9] - 2026-01-01` above `## [0.1.0] - 2026-09-30` | `"0.1.0"` |
| `TestChangelogAgreesWithVersion` | real `CHANGELOG.md` and `kcr.Version()` | if a heading is found it equals the version; if none is found the version is `0.0.0` |

- [ ] **Step 1:** Main thread writes the five documents and `ROADMAP.md`.
- [ ] **Step 2:** Subagent writes `changelog_test.go`.
- [ ] **Step 3:** Run `go test ./internal/guard/ -v`. Expected: build failure.
- [ ] **Step 4:** Subagent writes `changelog.go`.
- [ ] **Step 5:** Run `go test ./internal/guard/ -v`. Expected: PASS.
- [ ] **Step 6:** Commit.

```bash
git add README.md AGENTS.md CLAUDE.md CHANGELOG.md LICENSE internal/guard
git commit -m "docs: add working documents and changelog guard"
```

---

### Task 3: Structure guards

**Executor:** Subagent.

**Files:**
- Create: `internal/guard/gomod.go`, `internal/guard/gomod_test.go`
- Create: `internal/guard/imports.go`, `internal/guard/imports_test.go`

**Interfaces:**
- Consumes: nothing.
- Produces:
  - `func ForbiddenModules(gomod []byte) ([]string, error)`
  - `type Layer struct { Path string; Rank int }`
  - `func ImportViolations(module string, layers []Layer, imports map[string][]string) []string`
  - `var Layers []Layer`, the table later phases extend.

**Requirements for the dependency guard:**

- Parse with `golang.org/x/mod/modfile`.
- Check every `require`, direct and indirect, and every `replace` target.
- Forbidden prefixes, matched on path segments so that `k8s.io` does not
  match `k8s.iox`:
  `k8s.io`, `sigs.k8s.io`, `github.com/aws`, `cloud.google.com/go`,
  `github.com/Azure`, `github.com/oracle/oci-go-sdk`,
  `github.com/kubernetes`.
- Return the offending module paths sorted.

**Requirements for the import guard:**

- `imports` maps each package of this module to the packages it imports.
  Imports outside the module are ignored.
- The real scan reads the `Imports` field of `go list`. Imports made only by
  test files are outside the rule.
- A package matches a layer when its path equals `module + "/" + layer.Path`
  or is below it. The longest matching layer wins.
- A package may import another only when the other's rank is strictly lower,
  or when both belong to the same layer.
- A package of this module that matches no layer is a violation:
  `<package>: not classified in guard.Layers`.
- A rank violation reads:
  `<package> (rank <n>) imports <other> (rank <m>)`.
- Violations are returned sorted.
- Packages under `cmd/` and the root package are outside the rule.
- `Layers` for this phase:

| Path | Rank |
|---|---|
| `internal/guard` | 0 |
| `internal/typst` | 0 |
| `internal/cli` | 100 |

Later phases insert the core packages between 0 and 100 in the order of spec
section 4.1.

**Tests:**

| Test | Input | Expected |
|---|---|---|
| `TestForbiddenModulesClean` | `go.mod` text with cobra only | empty |
| `TestForbiddenModulesDirect` | adds `require k8s.io/client-go v0.30.0` | `["k8s.io/client-go"]` |
| `TestForbiddenModulesIndirect` | adds `github.com/aws/aws-sdk-go-v2 v1.0.0 // indirect` | that path |
| `TestForbiddenModulesReplace` | `replace example.com/x => sigs.k8s.io/yaml v1.0.0` | `["sigs.k8s.io/yaml"]` |
| `TestForbiddenModulesSegmentBoundary` | `require k8s.iox/thing v1.0.0` | empty |
| `TestForbiddenModulesMalformed` | `requir x` | error |
| `TestRepositoryHasNoForbiddenModule` | real `go.mod` | empty |
| `TestImportViolationsLowerRankAllowed` | `cli` imports `typst` | empty |
| `TestImportViolationsHigherRankRefused` | `typst` imports `cli` | one violation naming both ranks |
| `TestImportViolationsSameLayerAllowed` | `internal/typst/a` imports `internal/typst/b` | empty |
| `TestImportViolationsUnclassified` | package `internal/mystery` present | `...: not classified in guard.Layers` |
| `TestImportViolationsLongestLayerWins` | layers `internal/platform` rank 50 and `internal/platform/api` rank 60; `api` imports `platform/store` | empty |
| `TestImportViolationsIgnoresOutsideModule` | package imports `fmt` and `github.com/spf13/cobra` | empty |
| `TestRepositoryImportsPointOneWay` | runs `go list -json ./...` from the module root | empty |

`TestRepositoryImportsPointOneWay` finds the module root by walking up from
the working folder to the first `go.mod`. It fails when `go list` returns no
package, because a scan that finds nothing protects nothing.

- [ ] **Step 1:** Write `gomod_test.go`.
- [ ] **Step 2:** Run `go test ./internal/guard/ -run Forbidden -v`.
  Expected: build failure.
- [ ] **Step 3:** Write `gomod.go`; add `golang.org/x/mod` to `go.mod`.
- [ ] **Step 4:** Run the same command. Expected: PASS.
- [ ] **Step 5:** Write `imports_test.go`.
- [ ] **Step 6:** Run `go test ./internal/guard/ -run Import -v`.
  Expected: build failure.
- [ ] **Step 7:** Write `imports.go`.
- [ ] **Step 8:** Run `go test ./... -v`. Expected: PASS.
- [ ] **Step 9:** Prove the real scan can fail. Temporarily add
  `import _ "github.com/marceloalvescf/k8s-capacity-report/internal/cli"` to a
  file in `internal/guard`, run the tests, confirm
  `TestRepositoryImportsPointOneWay` fails, then remove the line.
- [ ] **Step 10:** Commit.

```bash
git add go.mod go.sum internal/guard
git commit -m "test(guard): protect dependencies and imports"
```

---

### Task 4: Forbid skipped tests

**Executor:** Subagent.

**Files:**
- Create: `internal/guard/noskip.go`, `internal/guard/noskip_test.go`

**Interfaces:**
- Produces: `func SkipCalls(filename string, source []byte) ([]string, error)`.

**Requirements:**

- Parse the file with `go/parser` and walk the syntax tree.
- Report every call whose selector is `Skip`, `Skipf`, or `SkipNow`, as
  `<filename>:<line>: <selector> is forbidden; a skipped gate is not a checked gate`.
- The real scan reads every `*_test.go` below the module root, leaving out
  `older/`, `.cache/`, `web/`, and `.git/`.
- The guard's own test file contains the word only inside string literals,
  which the syntax walk does not report.

**Tests:**

| Test | Input | Expected |
|---|---|---|
| `TestSkipCallsFindsSkip` | source calling `t.Skip("x")` on line 5 | one entry with `:5:` |
| `TestSkipCallsFindsSkipfAndSkipNow` | both calls | two entries |
| `TestSkipCallsIgnoresStringsAndComments` | `// t.Skip()` and `"t.Skip()"` | empty |
| `TestSkipCallsIgnoresOtherSelectors` | `t.Skipped()` | empty |
| `TestRepositoryHasNoSkippedTest` | real tree | empty; fails if zero test files were read |

- [ ] **Step 1:** Write `noskip_test.go`.
- [ ] **Step 2:** Run `go test ./internal/guard/ -run Skip -v`. Expected: build failure.
- [ ] **Step 3:** Write `noskip.go`.
- [ ] **Step 4:** Run `go test ./... -v`. Expected: PASS.
- [ ] **Step 5:** Commit.

```bash
git add internal/guard
git commit -m "test(guard): forbid skipped tests"
```

---

### Task 5: Typst toolchain probe

**Executor:** Subagent. It does not run the `toolchain` tag tests.

**Files:**
- Create: `internal/typst/toolchain.go`, `internal/typst/toolchain_test.go`
- Create: `internal/typst/toolchain_real_test.go`

**Interfaces:**
- Produces, in package `typst`:

```go
const RequiredVersion = "0.15.1"

type Toolchain struct {
    Binary string // absolute path
}

// Locate returns the binary named by KCR_TYPST, or "typst" found on PATH.
func Locate(getenv func(string) string, lookPath func(string) (string, error)) (Toolchain, error)

// Version runs "<binary> --version" and returns the second field.
func (t Toolchain) Version(ctx context.Context) (string, error)

// Probe fails unless Version returns RequiredVersion.
func (t Toolchain) Probe(ctx context.Context) error
```

**Requirements:**

- `Locate`:
  - When `KCR_TYPST` is set, it must be an absolute path to an existing
    regular file. Otherwise the error is
    `KCR_TYPST must be an absolute path to the typst binary: "<value>"`.
  - When it is not set, `lookPath("typst")` is used and the result is made
    absolute. On failure the error is
    `typst not found: set KCR_TYPST or add typst to PATH`.
- `Version`:
  - Runs the binary by its absolute path with an empty environment.
  - `typst --version` prints `typst 0.15.1 (hash)`. The version is the second
    field.
  - A non-zero exit, or fewer than two fields, is an error that includes the
    exit code and the first line of output:
    `typst --version failed (exit <n>): <first line or "no output">`.
  - The context deadline is honoured. The caller passes one; the function
    adds none.
- `Probe`: on mismatch the error is
  `typst <found> found at <binary>; this build requires <RequiredVersion>`.

**Tests, no build tag, using a fake binary:**

The tests write a small shell script into `t.TempDir()`, make it executable,
and point `Toolchain.Binary` at it. The script starts with `#!/bin/sh`, uses
shell built-ins, and names any other program by absolute path, because it runs
with an empty environment.

| Test | Fake binary | Expected |
|---|---|---|
| `TestLocateUsesEnvironment` | file exists, `KCR_TYPST` set | that path |
| `TestLocateRejectsRelativePath` | `KCR_TYPST=bin/typst` | error with `must be an absolute path` |
| `TestLocateRejectsMissingFile` | `KCR_TYPST=/nonexistent/typst` | same error |
| `TestLocateRejectsDirectory` | `KCR_TYPST` is a folder | same error |
| `TestLocateFallsBackToPath` | variable empty, `lookPath` returns a path | that path, absolute |
| `TestLocateNotFound` | `lookPath` returns an error | `typst not found: set KCR_TYPST ...` |
| `TestVersionParsesSecondField` | prints `typst 0.15.1 (abc123)` | `"0.15.1"` |
| `TestVersionFailsOnNonZeroExit` | prints `boom` to stderr, exits 3 | error with `exit 3` and `boom` |
| `TestVersionFailsOnShortOutput` | prints `typst` | error with `typst --version failed` |
| `TestVersionFailsOnNoOutput` | prints nothing, exits 0 | error with `no output` |
| `TestVersionHonoursDeadline` | sleeps 5 seconds; context of 200 ms | error within 1 second |
| `TestVersionUsesEmptyEnvironment` | prints `typst 0.15.1 (x)` only when `HOME` and `PATH` are both unset; otherwise exits 9 | `"0.15.1"` |
| `TestProbeAcceptsRequiredVersion` | prints `typst 0.15.1 (x)` | no error |
| `TestProbeRejectsOtherVersion` | prints `typst 0.15.0 (x)` | error with `typst 0.15.0 found at` and `requires 0.15.1` |

**Tests with build tag `toolchain`, against the real binary:**

| Test | Expected |
|---|---|
| `TestRealTypstIsThePinnedVersion` | `Locate` then `Probe` succeed |
| `TestRealPdftotextExists` | `exec.LookPath("pdftotext")` succeeds |

Both fail when the tool is missing.

- [ ] **Step 1:** Write `toolchain_test.go`.
- [ ] **Step 2:** Run `go test ./internal/typst/ -v`. Expected: build failure.
- [ ] **Step 3:** Write `toolchain.go`.
- [ ] **Step 4:** Run `go test ./internal/typst/ -v`. Expected: PASS.
- [ ] **Step 5:** Write `toolchain_real_test.go` with `//go:build toolchain`.
- [ ] **Step 6:** Run `go vet -tags=toolchain ./internal/typst/`.
  Expected: no output. This proves the tagged file compiles.
- [ ] **Step 7:** Commit.

```bash
git add internal/typst
git commit -m "feat(typst): locate and probe the toolchain"
```

---

### Task 6: Container image, linter, and Makefile

**Executor:** Main thread. Every step needs Docker.

**Files:**
- Create: `deploy/Containerfile`, `.dockerignore`, `.golangci.yml`, `Makefile`

**Interfaces:**
- Consumes: the binary of Task 1 and the tagged tests of Task 5.
- Produces: `make` targets `build`, `test`, `lint`, `image`,
  `test-toolchain`, `gate`; images `k8s-capacity-report:<version>` and
  `k8s-capacity-report-test:<version>`.

**Containerfile requirements:**

| Stage | Base | Does |
|---|---|---|
| `build` | `golang:1.27` | Copies `go.mod`, `go.sum`, runs `go mod download`, copies the source, builds with `CGO_ENABLED=0` and `-trimpath` to `/out/k8s-capacity-report` |
| `typst` | `ghcr.io/typst/typst:0.15.1` | Source of the `typst` binary only |
| `test` | `golang:1.27` | Installs `poppler-utils`; copies `typst` to `/usr/local/bin/typst`; asserts the version; runs `go mod download`; creates user `10001` |
| `runtime` | `gcr.io/distroless/static-debian12` | Copies the binary and `typst` to `/usr/local/bin/`; `USER 10001:10001`; entry point is the binary |

- The `test` stage asserts the version at build time:

```dockerfile
RUN test "$(typst --version | cut -d' ' -f2)" = "0.15.1"
```

- The `test` stage sets `GOCACHE=/tmp/gocache`, `HOME=/tmp`, and
  `GOFLAGS=-mod=readonly`, so tests run as user `10001` over a read-only
  source mount.
- The three base images are pinned by digest after the tag. Resolve each with
  `docker buildx imagetools inspect <image>` and write it as
  `image:tag@sha256:...`. For the multi-platform images use the index digest.
  The Typst index digest on 2026-09-27 was
  `sha256:032e292249bcd378480cc7c142cfa324b63ef8aadeb88d7e7230320c4c9c422f`.
- `runtime` is the last stage, so a plain build produces it.

**`.dockerignore` requirements:** exclude `older/`, `.cache/`, `.git/`,
`.claude/`, `bin/`, `ROADMAP.md`, `web/node_modules/`.

**`.golangci.yml` requirements:** version 2 format; enable `errcheck`,
`govet`, `ineffassign`, `staticcheck`, `unused`, `gocritic`, `revive`,
`misspell`; formatter `gofmt`.

**Makefile requirements:**

```make
VERSION := $(shell tr -d '[:space:]' < VERSION)
IMAGE := k8s-capacity-report:$(VERSION)
TEST_IMAGE := k8s-capacity-report-test:$(VERSION)
GOLANGCI := github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.14.0
```

| Target | Does |
|---|---|
| `build` | `go build -o bin/k8s-capacity-report ./cmd/k8s-capacity-report` |
| `test` | `go test ./...` |
| `lint` | Fails if `gofmt -l .` prints anything; `go vet ./...`; `go vet -tags=toolchain ./...`; `go run $(GOLANGCI) run` |
| `image` | Builds `runtime` as `$(IMAGE)` and `test` as `$(TEST_IMAGE)` from `deploy/Containerfile` with context `.` |
| `test-toolchain` | Depends on `image`. Runs the test image as user `10001`, source mounted read-only at `/src`, command `go test -tags=toolchain ./internal/typst/...` |
| `gate` | Runs `lint`, `test`, `image` with `--no-cache --pull`, `test-toolchain`, then checks the runtime image; writes each log under `.cache/gate/` |

- The runtime check inside `gate`:

```bash
test "$(docker run --rm $(IMAGE) --version)" = "k8s-capacity-report $(VERSION)"
test "$(docker run --rm --entrypoint /usr/local/bin/typst $(IMAGE) --version | cut -d' ' -f2)" = "0.15.1"
test "$(docker inspect --format '{{.Config.User}}' $(IMAGE))" = "10001:10001"
```

- Every target is `.PHONY`. `gate` stops at the first failure.

- [ ] **Step 1:** Write `.dockerignore` and `deploy/Containerfile`.
- [ ] **Step 2:** Write `.golangci.yml` and `Makefile`.
- [ ] **Step 3:** Run `make test`. Expected: PASS.
- [ ] **Step 4:** Run `make lint`. Expected: no finding. Fix what it reports
  in the code of Tasks 1 to 5, not by relaxing the linter.
- [ ] **Step 5:** Run `make image`. Expected: both images built.
- [ ] **Step 6:** Run `make test-toolchain`. Expected: two tests pass.
- [ ] **Step 7:** Prove the version assertion can fail. Change `0.15.1` to
  `0.15.0` in the `test` stage `RUN test` line, run `make image`, confirm the
  build fails at that line, then restore it.
- [ ] **Step 8:** Run `make gate`. Expected: every step passes and
  `.cache/gate/` holds one log per step.
- [ ] **Step 9:** Commit.

```bash
git add deploy/Containerfile .dockerignore .golangci.yml Makefile
git commit -m "build: add image, linter, and gate"
```

---

### Task 7: Measure rendering across processor architectures

**Executor:** Main thread.

**Files:**
- Create: `docs/decisions/0001-golden-architecture.md`

**Why:** Golden pages are compared byte for byte. The original ran on one
architecture only. This task finds out whether `linux/amd64` and `linux/arm64`
produce the same PNG bytes.

**Method:** Render the original package's own examples with the pinned image
on both platforms. The source is read from `older/graphite-report`, which is
local reference and never tracked.

- [ ] **Step 1:** Prepare a scratch package path.

```bash
mkdir -p .cache/arch/pkg/local/graphite-report
cp -R older/graphite-report .cache/arch/pkg/local/graphite-report/1.0.0
rm -rf .cache/arch/pkg/local/graphite-report/1.0.0/.git \
       .cache/arch/pkg/local/graphite-report/1.0.0/.venv \
       .cache/arch/pkg/local/graphite-report/1.0.0/build
```

- [ ] **Step 2:** Render three sources on each platform. For `PLATFORM` in
  `linux/amd64` and `linux/arm64`, and `NAME` in `complete`,
  `custom-running-elements`, and `minimal`:

```bash
docker run --rm --platform "$PLATFORM" \
  -v "$PWD/.cache/arch:/work" \
  ghcr.io/typst/typst:0.15.1 compile \
  --root /work/pkg/local/graphite-report/1.0.0 \
  --package-path /work/pkg \
  --font-path /work/pkg/local/graphite-report/1.0.0/assets/fonts \
  --ignore-system-fonts --ignore-embedded-fonts \
  --format png --ppi 144 \
  "/work/pkg/local/graphite-report/1.0.0/examples/$NAME.typ" \
  "/work/out/${PLATFORM#linux/}/$NAME-{p}.png"
```

Create `.cache/arch/out/amd64` and `.cache/arch/out/arm64` first.

- [ ] **Step 3:** Compare.

```bash
(cd .cache/arch/out/amd64 && shasum -a 256 *.png) > .cache/arch/amd64.sums
(cd .cache/arch/out/arm64 && shasum -a 256 *.png) > .cache/arch/arm64.sums
diff .cache/arch/amd64.sums .cache/arch/arm64.sums
```

- [ ] **Step 4:** Also render the PDF of `complete` on both platforms and
  compare the two files with `shasum`.
- [ ] **Step 5:** Write the decision record. It states the date, the image
  digest, the commands, both lists of digests, and the decision:
  - **Equal:** golden images serve both architectures; the test image builds
    for the host platform.
  - **Different:** the template tests run on `linux/amd64`. Add
    `--platform linux/amd64` to the `test` image build and to every
    `docker run` of the test image in the `Makefile`, and record how much
    slower `make test-toolchain` became on this machine.
- [ ] **Step 6:** If the decision is "different", apply the `Makefile` change
  and run `make gate` again.
- [ ] **Step 7:** Commit.

```bash
git add docs/decisions/0001-golden-architecture.md Makefile
git commit -m "docs(decisions): record golden architecture"
```

---

### Task 8: Close the phase

**Executor:** Main thread.

- [ ] **Step 1:** Run `make gate` on the branch head. Expected: pass.
- [ ] **Step 2:** Dispatch the `adversarial-reviewer` subagent over
  `git diff main...chore/foundation` with the spec and this plan. Judge each
  finding. Fix what is confirmed, record what is declined in `ROADMAP.md`
  with the reason.
- [ ] **Step 3:** Update `ROADMAP.md` in four places: each item's status with
  its commit, the phase summary line, the phase section heading, and the
  findings log. Read the file back and confirm the changes are there.
- [ ] **Step 4:** `CHANGELOG.md` stays as it is. Phase 0 has no effect that a
  user perceives. Declare this omission when reporting.
- [ ] **Step 5:** `VERSION` stays `0.0.0`. No tag.
- [ ] **Step 6:** Run `make gate` again if Step 2 changed any file.
- [ ] **Step 7:** Integrate by fast-forward.

```bash
git switch main
git merge --ff-only chore/foundation
git branch -d chore/foundation
```

- [ ] **Step 8:** Report to the owner: what was delivered, the architecture
  decision, the declared omission, and any finding left open.

## Definition of done

- `make gate` passes on `main` after the merge.
- `k8s-capacity-report --version` prints `k8s-capacity-report 0.0.0`, natively
  and in the runtime image.
- The runtime image runs as `10001:10001` and holds `typst` 0.15.1.
- The guards fail when given a forbidden module, a wrong import, an
  unclassified package, a skipped test, or a changelog that disagrees with
  `VERSION`. Each was seen failing once.
- The architecture decision is recorded.
- `git ls-files` shows nothing under `older/`, `.cache/`, or `.claude/`, and
  no `ROADMAP.md`.
