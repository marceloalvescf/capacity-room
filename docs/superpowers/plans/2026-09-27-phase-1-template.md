# Phase 1: Template Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Both template layers live in the repository and turn a hand-written `report.json` into a reviewed PDF, in Portuguese (Brazil) and in English.

**Architecture:** `template/graphite/` is a port of `graphite-report` 1.0.0 and stays a valid Typst package. `template/capacity/` reads a JSON data file, validates it strictly, and lays out the capacity report on top of the generic layer. Go compiles through `internal/typst`, and a Go harness replaces the original Python test suite.

**Tech Stack:** Typst 0.15.1, Go 1.27, Liberation Sans, `pdftotext` from Poppler, Docker.

**Spec:** `docs/superpowers/specs/2026-09-27-k8s-capacity-report-design.md`, sections 10, 13.3, and 14.1.

**Depends on:** Phase 0 merged into `main`.

## How to read this plan

The owner's orchestration rule applies: the plan states what each task must
deliver and how it is proven, and the implementer writes the code. Each task
gives exact paths, exact signatures, and a table of tests with inputs and
expected results. Write each test first, watch it fail, then implement.

Executors:

- **Subagent**: a Sonnet subagent. It may run `go`, `make test`, `make lint`,
  and, when the native tools of the prerequisite below are installed,
  `go test -tags=template`. It must not run Docker and must not declare a task
  finished.
- **Main thread**: needs Docker, judgment, wording, or page review.

Reference material, read-only and never tracked:

- `older/graphite-report/` at commit `aea51af`: the package being ported.
- `older/kubernetes-report/kube_capacity_report/renderers/sections.py`: the
  cover, the footer, and the Typst helpers of the capacity report.
- `older/kubernetes-report/tests/golden/typst/detailed-all.typ`: a complete
  report in Portuguese, the source for fixture wording.
- `docs/reference/inventory/template.md` and
  `docs/reference/inventory/document-frontend.md`.

## Prerequisite for fast feedback

With `typst` and `pdftotext` installed on the Mac, subagents can compile
natively. This needs the owner's approval:

```bash
brew install typst poppler
typst --version      # must print 0.15.1
```

If Homebrew offers another Typst version, do not install it. Without native
tools every `template` test runs on the main thread through Docker. Golden
comparison always runs in the image, whatever is installed.

## Global Constraints

- Typst is pinned at exactly `0.15.1`.
- Every compile uses the four fixed flags together: `--root`, `--font-path`,
  `--ignore-system-fonts`, `--ignore-embedded-fonts`.
- A compiler warning is a failure.
- The template never evaluates a value as markup. `eval` is forbidden in
  every `.typ` file under `template/`.
- The public surface of the generic layer is exactly `graphite-report`,
  `callout`, and `keep-together`, with the arguments, defaults, label keys,
  and theme keys of version 1.0.0.
- Every diagnostic of the generic layer starts with `graphite-report: `.
  Every diagnostic of the report layer starts with `capacity-report: `.
  All diagnostics are in English.
- No brand, logo, or organisation name in `template/`.
- No page may contain a coloured pixel.
- Golden pages are rendered at 144 PPI and compared by SHA-256.
- A test never writes a golden image. Only `make update-golden` does, and
  every page is reviewed before it is committed.
- No test skips. Build tags select tests that need tools: `template` for
  `typst` and `pdftotext`, `golden` for byte comparison in the image.
- Code comments and documents are in English. Example documents keep their
  Portuguese content, since Portuguese is the default language of the layer.
- Status marks: healthy `●`, attention `▲`, critical `♦`, no base `○`,
  unavailable `—`.
- Commits follow `type(scope): description`, subject under 50 characters.

## Review Focus

1. **Text that looks like Typst markup.** A namespace called `#prod` or a
   value such as `*x*`, `$y$`, `<a>`, `@b`, or `[c]` must print exactly as
   written. Pinned in Task 8, fixture `literal`.
2. **A row with the wrong number of cells.** The compile must stop and name
   the table and the row. Pinned in Task 6.
3. **No author and no classification.** The cover must leave no empty line,
   and the footer must show only the page number. Pinned in Task 8, fixture
   `minimal`.
4. **A table with no rows.** A header with nothing below it misleads the
   reader. The compile must stop, so that the producer sends the fallback
   sentence. Pinned in Task 6.
5. **A work folder whose path contains a space.** Common on macOS. The
   compile must work. Pinned in Task 1.

---

## File structure

| Path | Responsibility |
|---|---|
| `internal/typst/compile.go` | Build arguments and run a compile |
| `internal/typst/compile_test.go` | Tests with a fake binary |
| `internal/typst/compile_real_test.go` | Build tag `template`; real binary |
| `template/graphite/typst.toml` | Package manifest |
| `template/graphite/lib.typ` | The three public functions |
| `template/graphite/src/` | `config`, `layout`, `theme`, `scope`, `components/` |
| `template/graphite/assets/fonts/` | Four font faces and their licence |
| `template/graphite/template/main.typ` | Starter document |
| `template/graphite/examples/` | Four example documents and one image |
| `template/graphite/README.md` | How to use the package, with recipes |
| `template/graphite/LICENSE`, `thumbnail.png` | Package entries |
| `template/capacity/main.typ` | Entry: reads data, validates, renders |
| `template/capacity/validate.typ` | Every check on the data file |
| `template/capacity/runs.typ` | The four run kinds and the status mark |
| `template/capacity/tables.typ` | Table layout by identifier |
| `template/capacity/blocks.typ` | Turns each block into content |
| `template/capacity/cover.typ` | Cover and footer |
| `template/embed.go` | Embeds both layers; unpacks them |
| `template/golden/snapshots.go` | The one list of golden pages |
| `template/golden/render.go` | Render, digest, compare, difference image |
| `template/golden/cmd/update-golden/main.go` | Rewrites golden pages |
| `template/tests/graphite/unit/` | Assertion files |
| `template/tests/graphite/positive/` | Fixtures that must compile |
| `template/tests/graphite/negative/` | Fixtures that must fail, each with `.expect` |
| `template/tests/capacity/fixtures/` | Hand-written `report.json` files |
| `template/tests/capacity/negative/` | Bad data files, each with `.expect` |
| `template/tests/golden/` | PNG pages and `SHA256SUMS` |
| `template/*_test.go` | The harness, by group |

---

### Task 0: Open the phase

**Executor:** Main thread.

- [ ] **Step 1:** Read `ROADMAP.md` and `CHANGELOG.md`. Confirm phase 0 is
  closed in the summary line and in its section, and that phase 1 is open.
  Correct the record before going on.
- [ ] **Step 2:** Open the branch.

```bash
git switch main
git switch -c feat/template
git branch --show-current
```

Expected: `feat/template`.

---

### Task 1: Compile through `internal/typst`

**Executor:** Subagent.

**Files:**
- Create: `internal/typst/compile.go`, `internal/typst/compile_test.go`
- Create: `internal/typst/compile_real_test.go`

**Interfaces:**
- Consumes: `Toolchain` from phase 0.
- Produces:

```go
type Format string

const (
    PDF Format = "pdf"
    PNG Format = "png"
)

type Job struct {
    Root        string            // absolute
    FontPath    string            // absolute
    Source      string            // absolute, inside Root
    Output      string            // absolute; for PNG it contains "{p}"
    Format      Format
    Inputs      map[string]string // passed as --input key=value
    PackagePath string            // optional, absolute
    Pages       string            // optional, PNG only, for example "1,2"
    PPI         int               // optional, PNG only
}

type CompileError struct {
    ExitCode int
    Stderr   string
}

func (e *CompileError) Error() string

var ErrWarning = errors.New("typst reported a warning")

// Args returns the arguments after the binary name.
func Args(job Job) ([]string, error)

// Compile runs the job. It returns stderr in every case.
func (t Toolchain) Compile(ctx context.Context, job Job) (stderr string, err error)
```

**Requirements:**

- `Args` is pure. The order is fixed:
  `compile`, `--diagnostic-format`, `short`, `--root`, root, `--font-path`,
  fonts, `--ignore-system-fonts`, `--ignore-embedded-fonts`, then
  `--package-path` when set, then `--format` and its value, then `--pages`
  and `--ppi` when set, then one `--input key=value` per input sorted by key,
  then source, then output.
- `Args` refuses, each with its own message:
  - a path that is not absolute, naming the field;
  - a source outside the root: `source must be inside root`;
  - an unknown format;
  - `PNG` whose output lacks `{p}`;
  - `Pages` or `PPI` set with `PDF`;
  - a negative `PPI`;
  - an input key that is empty or contains `=`.
- `Compile` runs the binary with an empty environment.
- A non-zero exit returns `*CompileError` with the code and stderr.
- A zero exit whose stderr contains `warning:` returns an error that wraps
  `ErrWarning` and includes stderr.
- `CompileError.Error()` is
  `typst compile failed (exit <n>): <first line of stderr>`.

**Tests, no build tag:**

| Test | Input | Expected |
|---|---|---|
| `TestArgsOrderForPDF` | full PDF job with two inputs `b` and `a` | the exact slice, inputs in order `a`, `b` |
| `TestArgsOrderForPNG` | PNG with pages `1,2`, PPI 144, package path | the exact slice |
| `TestArgsAlwaysHasTheFourFixedFlags` | any valid job | contains all four |
| `TestArgsRejectsRelativePath` (table) | each path field relative in turn | error naming that field |
| `TestArgsRejectsSourceOutsideRoot` | root `/a`, source `/b/x.typ` | `source must be inside root` |
| `TestArgsRejectsSiblingPrefix` | root `/a/b`, source `/a/bc/x.typ` | same error |
| `TestArgsRejectsPNGWithoutPagePlaceholder` | output `/o/page.png` | error |
| `TestArgsRejectsPagesForPDF` | PDF with pages | error |
| `TestArgsRejectsBadInputKey` (table) | `""`, `"a=b"` | error |
| `TestCompileReturnsStderrOnSuccess` | fake prints nothing, exits 0 | empty, no error |
| `TestCompileFailsOnNonZeroExit` | fake prints `x.typ:1:1: error: boom`, exits 1 | `*CompileError`, code 1, message has `boom` |
| `TestCompileFailsOnWarning` | fake prints `x.typ:2:1: warning: unused`, exits 0 | `errors.Is(err, ErrWarning)` |
| `TestCompilePassesArguments` | fake writes its arguments to a file | file equals `Args(job)` |
| `TestCompileUsesEmptyEnvironment` | fake exits 9 when `HOME` or `PATH` is set | no error |
| `TestCompileHonoursDeadline` | fake sleeps 5 seconds; context 200 ms | error within 1 second |

**Tests with build tag `template`:**

| Test | Setup | Expected |
|---|---|---|
| `TestRealCompileWritesPDF` | one-line document in a temp root, fonts from `template/graphite/assets/fonts` | file starts with `%PDF-` |
| `TestRealCompileInFolderWithSpace` | root is `<temp>/my work folder` | PDF written |
| `TestRealCompileReadsInput` | document prints `sys.inputs.at("name")`; input `name=valor` | `pdftotext` output contains `valor` |
| `TestRealCompileReportsError` | document calls `panic("stop here")` | `*CompileError`, stderr has `stop here` |
| `TestRealCompileReportsWarning` | document that Typst warns about: a `#set text(font: "No Such Font")` | `ErrWarning` |
| `TestRealCompileIsDeterministic` | same job twice | both PDFs have the same SHA-256 |

`TestRealCompileWritesPDF` and its neighbours need the fonts of Task 2. Write
them now and run them after Task 2.

- [ ] **Step 1:** Write `compile_test.go`.
- [ ] **Step 2:** Run `go test ./internal/typst/ -v`. Expected: build failure.
- [ ] **Step 3:** Write `compile.go`.
- [ ] **Step 4:** Run `go test ./internal/typst/ -v`. Expected: PASS.
- [ ] **Step 5:** Write `compile_real_test.go` with `//go:build template`.
- [ ] **Step 6:** Run `go vet -tags=template ./internal/typst/`. Expected: no output.
- [ ] **Step 7:** Commit.

```bash
git add internal/typst
git commit -m "feat(typst): compile with fixed flags"
```

---

### Task 2: Port the generic layer

**Executor:** Subagent for the files. Main thread judges the README wording.

**Files:**
- Create under `template/graphite/`: `typst.toml`, `lib.typ`, `LICENSE`,
  `thumbnail.png`, `README.md`, `src/config.typ`, `src/layout.typ`,
  `src/theme.typ`, `src/scope.typ`, `src/components/callout.typ`,
  `src/components/keep-together.typ`, `assets/fonts/` (four `.ttf` files and
  `LICENSE`), `template/main.typ`, `examples/complete.typ`,
  `examples/custom-running-elements.typ`, `examples/custom-theme.typ`,
  `examples/minimal.typ`, `examples/assets/process-flow.svg`
- Create under `template/tests/graphite/`: `unit/`, `positive/`, `negative/`

**Source:** `older/graphite-report/` at commit `aea51af`.

**Requirements:**

- Copy the nine Typst source files. Change only what this list names:
  1. Translate every comment to English. Keep the finding identifiers they
     cite, such as `P1` and `F5`.
  2. In `src/scope.typ`, replace the Portuguese message with:
     `` "graphite-report: `" + symbol + "` only works inside a document under `graphite-report`; apply `#show: graphite-report.with(...)` before using it" ``
  3. Nothing else in `src/` or `lib.typ` changes. Default labels stay in
     Portuguese.
- Copy the binary files byte for byte: the four fonts, the font licence,
  `thumbnail.png`, `process-flow.svg`, and `LICENSE`. Confirm each with
  `shasum -a 256` against the source.
- `typst.toml`:

```toml
[package]
name = "graphite-report"
version = "0.0.0"
entrypoint = "lib.typ"
authors = ["Marcelo Alves"]
license = "MIT"
description = "Neutral monochrome template for corporate reports."
compiler = "0.15.1"
categories = ["report"]
exclude = ["examples", "README.md"]

[template]
path = "template"
entrypoint = "main.typ"
thumbnail = "thumbnail.png"
```

- Examples import the package by path: `#import "../lib.typ": ...`.
- The starter `template/main.typ` imports by name, because it is what a user
  receives: `#import "@local/graphite-report:0.0.0": graphite-report`.
- Move the three unit files to `template/tests/graphite/unit/`, the ten
  positive fixtures to `positive/`, and the twelve negative fixtures to
  `negative/`. Rewrite their imports as relative paths to
  `template/graphite/lib.typ` or to `template/graphite/src/...`.
- `positive/complete.typ`, `custom-running-elements.typ`, and
  `custom-theme.typ` include the example of the same name.
- Beside each negative fixture `name.typ`, write `name.expect`: one line,
  the exact fragment that stderr must contain. Take the twelve fragments from
  `older/graphite-report/tests/test_diagnostics.py`. The fragment for
  `callout-outside-scope` becomes
  ``graphite-report: `callout` only works inside a document``.
- `README.md` is the original README in English, with these changes:
  - requirements: Typst `0.15.1`; no Python;
  - inside this repository the package is imported by path;
  - for other documents, copy the package entries to
    `<data folder>/typst/packages/local/graphite-report/<version>/`;
  - the three recipes keep their code unchanged, under headings that start
    with `### Recipe`;
  - the test section describes `make test-template` and `make update-golden`;
  - known limits as in spec section 10.4.

**Verification:**

- [ ] **Step 1:** Copy and adapt the files.
- [ ] **Step 2:** Confirm the binary files.

```bash
for f in assets/fonts/LiberationSans-Regular.ttf assets/fonts/LiberationSans-Bold.ttf \
         assets/fonts/LiberationSans-Italic.ttf assets/fonts/LiberationSans-BoldItalic.ttf \
         assets/fonts/LICENSE thumbnail.png LICENSE examples/assets/process-flow.svg; do
  a=$(shasum -a 256 < "older/graphite-report/$f"); b=$(shasum -a 256 < "template/graphite/$f")
  test "$a" = "$b" || echo "DIFFERENT: $f"
done
```

Expected: no output.

- [ ] **Step 3:** Confirm that only comments and the one message changed in
  the Typst sources. For each of the nine files, strip comment lines from both
  versions and compare:

```bash
for f in lib.typ src/config.typ src/layout.typ src/theme.typ src/scope.typ \
         src/components/callout.typ src/components/keep-together.typ; do
  diff <(grep -v '^\s*//' "older/graphite-report/$f") \
       <(grep -v '^\s*//' "template/graphite/$f") || echo "^ $f"
done
```

Expected: one difference, in `src/scope.typ`, the three lines of the message.

- [ ] **Step 4:** Confirm there is no `@local` import outside the starter and
  the README.

```bash
grep -rn '@local/graphite-report' template/ | grep -v -e 'template/graphite/template/main.typ' -e 'README.md'
```

Expected: no output.

- [ ] **Step 5:** With native tools, compile one example.

```bash
mkdir -p .cache/phase1
typst compile --root template --font-path template/graphite/assets/fonts \
  --ignore-system-fonts --ignore-embedded-fonts \
  template/graphite/examples/complete.typ .cache/phase1/complete.pdf
```

Expected: exit 0, no output. Without native tools, the main thread runs the
same command in the test image.

- [ ] **Step 6:** Main thread reads `README.md` against the original and
  corrects the wording.
- [ ] **Step 7:** Commit.

```bash
git add template/graphite template/tests/graphite
git commit -m "feat(template): port the generic layer"
```

---

### Task 3: Harness for the generic layer

**Executor:** Subagent. Main thread runs it in the image.

**Files:**
- Create: `template/embed.go`
- Create: `template/harness_test.go`, `template/toolchain_test.go`,
  `template/compile_test.go`, `template/negative_test.go`,
  `template/recipes_test.go`, `template/package_test.go`,
  `template/consistency_test.go`
- Modify: `internal/guard/imports.go` (add layers)
- Modify: `Makefile` (add `test-template`)

**Interfaces:**
- Consumes: `typst.Locate`, `Toolchain.Probe`, `Toolchain.Compile`, `typst.Job`.
- Produces, in package `template`:

```go
// Unpack writes the embedded template under dir, creating it if needed.
func Unpack(dir string) error

// FontPath returns the font folder inside an unpacked template.
func FontPath(dir string) string // dir + "/graphite/assets/fonts"

// Entry returns the report entry point inside an unpacked template.
func Entry(dir string) string // dir + "/capacity/main.typ"
```

  `embed.go` embeds `graphite/typst.toml`, `graphite/lib.typ`, `graphite/src`,
  and `graphite/assets`. Task 6 adds `capacity`.

- Produces, in `harness_test.go`, for every test file of this package:

```go
// workspace copies the template folder from disk into a fresh temp folder
// and returns it. It is the compile root.
func workspace(t *testing.T) string

// toolchain locates and probes typst once per test binary.
func toolchain(t *testing.T) typst.Toolchain

// compilePDF compiles source, a path relative to the workspace.
func compilePDF(t *testing.T, root, source string, inputs map[string]string) (pdf string, stderr string, err error)

// pdfText returns the text layer through pdftotext -layout.
func pdfText(t *testing.T, pdf string) string
```

**Requirements:**

- Every test file except `embed_test.go` starts with `//go:build template`.
- `guard.Layers` gains `template` with rank 0 and `template/golden` with
  rank 10. `internal/cli` stays at 100.
- `make test-template` runs, in the test image, as user `10001`, with the
  source mounted read-only at `/src`:
  `go test -tags=template ./internal/typst/... ./template/...`.
- `make lint` also runs `go vet -tags=template ./...`.

**Tests, no build tag (`embed_test.go`):**

| Test | Expected |
|---|---|
| `TestUnpackWritesEveryEmbeddedFile` | each embedded file exists on disk with the same bytes |
| `TestUnpackIntoExistingFolder` | second call succeeds and leaves the same files |
| `TestEmbeddedFilesMatchDisk` | every embedded file equals the file in the source tree |
| `TestFontLicenceIsEmbedded` | `graphite/assets/fonts/LICENSE` is present |

**Tests with tag `template`:**

| Group | Test | Expected |
|---|---|---|
| Toolchain | `TestFontFacesExist` | four files present and not empty |
| Toolchain | `TestTypstListsLiberationSans` | `typst fonts --font-path ... --ignore-system-fonts --ignore-embedded-fonts` prints a line starting with `Liberation Sans` |
| Assertions | `TestUnitFilesCompileCleanly` | each file in `tests/graphite/unit/` compiles, no warning, PDF not empty |
| Positive | `TestExamplesCompileCleanly` | each file in `graphite/examples/` |
| Positive | `TestPositiveFixturesCompileCleanly` | each file in `tests/graphite/positive/` |
| Negative | `TestNegativeFixturesFailWithExpectedMessage` | each file in `negative/` fails, stderr contains the content of its `.expect` |
| Negative | `TestEveryNegativeFixtureHasAnExpectation` | each `.typ` has a non-empty `.expect`, each `.expect` has a `.typ` |
| Negative | `TestNegativeFixtureCount` | exactly 12 in `tests/graphite/negative/` |
| Recipes | `TestReadmeDeclaresTheKnownRecipes` | headings starting with `### Recipe` are exactly three, in the order of the README |
| Recipes | `TestEveryReadmeRecipeCompilesCleanly` | the first `typst` block under each recipe heading compiles |
| Package | `TestPackageInstallsByCopy` | see below |
| Consistency | `TestManifestVersionEqualsVersionFile` | `typst.toml` version equals `kcr.Version()` |
| Consistency | `TestManifestCompilerIsThePinnedVersion` | `compiler` equals `typst.RequiredVersion` |
| Consistency | `TestEveryPackageIdentifierUsesTheVersion` | every `@local/graphite-report:X.Y.Z` in `.typ`, `.toml`, and `.md` under `template/` uses the version; at least one was found |
| Consistency | `TestNoEvalInTemplate` | no `.typ` file under `template/` contains `eval(` |

- Recipe extraction: for each heading that starts with `### Recipe`, take the
  first fenced block marked `typst` before the next heading of level 2 or 3.
  When the block has no `#import`, prepend
  `#import "/graphite/lib.typ": graphite-report, callout, keep-together`.
  When it has no `#show: graphite-report`, prepend
  `#show: graphite-report.with(title: [Recipe from the README])`.
  Write the document inside the workspace.
- `TestPackageInstallsByCopy`:
  1. Read `exclude` from `typst.toml`.
  2. Copy every entry of `template/graphite/` that is not excluded to
     `<temp>/pkgs/local/graphite-report/<version>/`.
  3. Copy `template/graphite/template/main.typ` to `<temp>/doc/main.typ`.
  4. Compile with root `<temp>/doc`, package path `<temp>/pkgs`, and the font
     folder of the copied package.
  5. Expect a clean compile and the text `Relatório corporativo` in the text
     layer.

- [ ] **Step 1:** Write `embed_test.go`. Run `go test ./template/ -v`.
  Expected: build failure.
- [ ] **Step 2:** Write `embed.go`. Run again. Expected: PASS.
- [ ] **Step 3:** Add the layers to `guard.Layers`. Run `go test ./internal/guard/ -v`.
  Expected: PASS.
- [ ] **Step 4:** Write `harness_test.go` and the six test files.
- [ ] **Step 5:** Run `go vet -tags=template ./template/`. Expected: no output.
- [ ] **Step 6:** With native tools, run `go test -tags=template ./template/ ./internal/typst/ -v`.
  Expected: PASS.
- [ ] **Step 7:** Main thread adds `test-template` to the `Makefile`, runs
  `make test-template`. Expected: PASS, with no test reported as skipped.
- [ ] **Step 8:** Prove three tests can fail, one at a time, restoring each:
  change one `.expect` fragment; add `#let x = eval("1")` to an example;
  change `compiler` in `typst.toml` to `0.15.0`.
- [ ] **Step 9:** Commit.

```bash
git add template internal/guard Makefile
git commit -m "test(template): add harness for generic layer"
```

---

### Task 4: Golden pages

**Executor:** Subagent for the code. Main thread for Docker.

**Files:**
- Create: `template/golden/snapshots.go`, `template/golden/render.go`,
  `template/golden/render_test.go`
- Create: `template/golden/cmd/update-golden/main.go`
- Create: `template/golden_test.go` (tag `golden`)
- Modify: `Makefile` (add `test-golden`, `update-golden`; extend `gate`)

**Interfaces:**
- Produces, in package `golden`:

```go
const PPI = 144

type Snapshot struct {
    Name     string            // file stem, for example "complete"
    Source   string            // path relative to the template folder
    Pages    []int
    Inputs   map[string]string
    Protects string            // the one behaviour this snapshot alone protects
}

var Snapshots []Snapshot

// Files returns the PNG names a snapshot produces, for example "complete-1.png".
func (s Snapshot) Files() []string

// Render writes the pages of s into dir.
func Render(ctx context.Context, tc typst.Toolchain, root string, s Snapshot, dir string) error

// Digest returns the SHA-256 of a file in hexadecimal.
func Digest(path string) (string, error)

// ReadSums parses a SHA256SUMS file.
func ReadSums(path string) (map[string]string, error)

// WriteSums writes a SHA256SUMS file sorted by name.
func WriteSums(path string, sums map[string]string) error

// HasColour reports whether any pixel has different red, green, and blue.
func HasColour(path string) (bool, error)

// WriteDifference writes an image that marks the pixels that differ.
func WriteDifference(golden, actual, out string) error
```

**Requirements:**

- `Snapshots` holds the ten snapshots of the original, with the sources moved
  to their new paths and `Protects` translated from the comments of
  `older/graphite-report/tests/harness/snapshots.py`:

| Name | Source | Pages |
|---|---|---|
| `complete` | `graphite/examples/complete.typ` | 1, 2 |
| `minimal` | `graphite/examples/minimal.typ` | 1 |
| `theme` | `graphite/examples/custom-theme.typ` | 1 |
| `running` | `graphite/examples/custom-running-elements.typ` | 1, 2 |
| `table` | `tests/graphite/positive/multipage-table.typ` | 1, 2 |
| `starter` | `tests/graphite/positive/starter.typ` | 1 |
| `callout-break` | `tests/graphite/positive/callout-page-break.typ` | 1 |
| `locale` | `tests/graphite/positive/locale-english.typ` | 1 |
| `components` | `tests/graphite/unit/components-valid.typ` | 1 |
| `identifiers` | `tests/graphite/positive/identifier-wrapping.typ` | 1 |

- `tests/graphite/positive/starter.typ` has the content of the starter with
  the import by path. It replaces the original `template` snapshot, which
  needed an installed package.
- `SHA256SUMS` format: `<digest><two spaces><file name>`, one per line, sorted.
- `HasColour` decodes with `image/png`.
- `update-golden` deletes every PNG in `template/tests/golden/`, renders every
  snapshot, and writes `SHA256SUMS`. It is never called by a test.
- `make test-golden` runs in the test image:
  `go test -tags='template golden' ./template/ -run Golden`.
  It mounts `.cache/template/` at `/out` and sets `KCR_GOLDEN_OUT=/out`, where
  the test writes actual pages and difference images when a page differs.
- `make update-golden` runs the command in the test image with
  `template/tests/golden/` mounted for writing.
- If the phase 0 decision record says architectures differ, both targets pass
  `--platform linux/amd64`.
- `make gate` gains `test-template` and `test-golden`.

**Tests, no build tag (`render_test.go`):**

| Test | Input | Expected |
|---|---|---|
| `TestEverySnapshotStatesWhatItProtects` | `Snapshots` | no empty `Protects`, no two equal |
| `TestSnapshotNamesAreUnique` | `Snapshots` | no repeated `Name` |
| `TestSnapshotSourcesExist` | `Snapshots` | every source is a file |
| `TestFilesNamesPages` | name `x`, pages 1 and 3 | `x-1.png`, `x-3.png` |
| `TestSumsRoundTrip` | write then read | same map; file sorted by name |
| `TestReadSumsRejectsMalformedLine` | line without two spaces | error naming the line number |
| `TestReadSumsRejectsDuplicate` | same name twice | error |
| `TestHasColourFalseForGrey` | generated PNG, all pixels grey, RGBA | `false` |
| `TestHasColourTrueForOneRedPixel` | same with one red pixel | `true` |
| `TestWriteDifferenceMarksChangedPixels` | two generated images, one pixel apart | output exists; exactly one marked pixel |
| `TestWriteDifferenceRejectsDifferentSizes` | 2x2 and 3x3 | error naming both sizes |

**Tests with tags `template golden` (`golden_test.go`):**

| Test | Expected |
|---|---|
| `TestGoldenManifestMatchesFiles` | names in `SHA256SUMS` equal the PNG files present; each digest matches |
| `TestGoldenCountMatchesSnapshots` | number of PNG files equals the sum of pages |
| `TestGoldenPagesMatch` | each rendered page has the digest of its golden page; on mismatch the actual page and a difference image are written and named in the failure |
| `TestGoldenPagesHaveNoColour` | `HasColour` is false for every rendered page |

- [ ] **Step 1:** Write `render_test.go`. Run `go test ./template/golden/ -v`.
  Expected: build failure.
- [ ] **Step 2:** Write `snapshots.go` and `render.go`. Run again. Expected: PASS.
- [ ] **Step 3:** Write the command and `golden_test.go`.
- [ ] **Step 4:** Run `go vet -tags='template golden' ./template/...`.
  Expected: no output.
- [ ] **Step 5:** Main thread adds the `Makefile` targets.
- [ ] **Step 6:** Commit the code without any PNG.

```bash
git add template/golden template/golden_test.go template/tests/graphite/positive/starter.typ Makefile
git commit -m "test(template): add golden page harness"
```

---

### Task 5: Produce and review the generic golden pages

**Executor:** Main thread.

- [ ] **Step 1:** Run `make update-golden`. Expected: 13 PNG files and
  `SHA256SUMS` in `template/tests/golden/`.
- [ ] **Step 2:** Compare each digest with the original's, in
  `older/graphite-report/tests/golden/SHA256SUMS`. Record in `ROADMAP.md`
  which pages are byte-identical under 0.15.1 and which are not. The
  `starter` page corresponds to the original `template-1.png`.
- [ ] **Step 3:** Open every page that differs, beside the original page, and
  review it: opening, spacing, table header fill, callout marks, page number,
  hyphenation. A difference caused by the compiler version is accepted and
  recorded. A difference caused by the port is a defect of Task 2 and is
  fixed there.
- [ ] **Step 4:** Open every page once more on its own, including the
  byte-identical ones, and confirm each shows what its `Protects` line says.
- [ ] **Step 5:** Run `make test-golden`. Expected: PASS.
- [ ] **Step 6:** Prove the comparison can fail. Change `text-size` from
  `11pt` to `11.5pt` in `src/config.typ`, run `make test-golden`, confirm the
  failure names the pages and that difference images appear under
  `.cache/template/`, then restore the value.
- [ ] **Step 7:** Commit.

```bash
git add template/tests/golden
git commit -m "test(template): add generic golden pages"
```

---

### Task 6: Report layer

**Executor:** Subagent. Main thread judges the layout in Task 9.

**Files:**
- Create: `template/capacity/main.typ`, `validate.typ`, `runs.typ`,
  `tables.typ`, `blocks.typ`, `cover.typ`
- Create: `template/tests/capacity/negative/*.json` with `.expect` files
- Create: `template/capacity_negative_test.go` (tag `template`)
- Modify: `template/embed.go` (embed `capacity`)

**Interfaces:**
- Consumes: `/graphite/lib.typ`; the input `data`, a path relative to the
  compile root.
- Produces: the data contract below, which phase 2 writes from Go.

**The data contract, schema `"1"`:**

```json
{
  "schema": "1",
  "language": "pt-BR",
  "theme": {
    "lang": "pt",
    "region": "BR",
    "date_format": "[day padding:zero]/[month padding:zero]/[year]"
  },
  "labels": {
    "author": "Responsável",
    "date": "Emissão",
    "classification": "Classificação",
    "outline": "Sumário",
    "note": "Nota",
    "info": "Informação",
    "warning": "Atenção"
  },
  "cover": {
    "title": "LEVANTAMENTO DE CAPACIDADE",
    "cluster_label": "Cluster:",
    "cluster": "production-east",
    "author_label": "ELABORADO POR:",
    "author": "Time SRE",
    "issued_label": "EMITIDO EM:",
    "issued": "15/07/2026 14:30"
  },
  "footer": {
    "classification": "Uso interno",
    "page_label": "Página"
  },
  "blocks": [
    { "type": "outline", "title": "Sumário", "depth": 2 },
    { "type": "heading", "level": 1, "runs": [ { "kind": "text", "text": "Resumo executivo" } ] },
    { "type": "paragraph", "runs": [ { "kind": "text", "text": "Período analisado: 24 horas." } ] },
    {
      "type": "table",
      "id": "status-legend",
      "headers": [ "Status", "Interpretação" ],
      "rows": [
        [
          [ { "kind": "status", "status": "critical", "text": "Crítico" } ],
          [ { "kind": "text", "text": "Requer ação imediata." } ]
        ]
      ]
    },
    { "type": "callout", "kind": "warning", "title": null, "runs": [ { "kind": "text", "text": "Dados parciais." } ] },
    { "type": "pagebreak", "weak": true }
  ]
}
```

| Element | Rule |
|---|---|
| Top level | Exactly the keys `schema`, `language`, `theme`, `labels`, `cover`, `footer`, `blocks` |
| `schema` | The string `"1"` |
| `language` | `"pt-BR"` or `"en"` |
| `theme` | Exactly `lang`, `region`, `date_format`, all strings |
| `labels` | Exactly the seven keys, all strings |
| `cover` | Exactly the seven keys; `author` is a string or null; the rest are strings |
| `footer` | Exactly two keys; `classification` is a string or null |
| `blocks` | An array, at least one block |
| Block `outline` | `title` string; `depth` 1, 2, or 3 |
| Block `front` | `blocks` not empty; holds any block except `front` and `pagebreak`; only as the first block |
| Block `heading` | `level` 1, 2, or 3; `numbered` boolean; `runs` not empty |
| Block `list` | `items` not empty; each item is an array of runs, not empty |
| Block `paragraph` | `runs` not empty |
| Block `table` | `id` known; `headers` strings, as many as the columns; `rows` not empty; every row has as many cells as the columns; a cell is an array of runs and may be empty |
| Block `callout` | `kind` is `note`, `info`, or `warning`; `title` is null or runs; `runs` not empty |
| Block `pagebreak` | `weak` boolean |
| Run | `kind` is `text`, `strong`, `ident`, `placeholder`, or `status`; `text` is a string |
| Run `status` | Also `status`, one of `healthy`, `attention`, `critical`, `undefined`, `unavailable`; `text` may be null |

An unknown key anywhere is an error.

**Table layouts, in `tables.typ`:**

| Identifier | Columns, in `fr` | Size | Inset y |
|---|---|---|---|
| `report-metadata` | 1, 3 | 8.5pt | 2pt |
| `status-legend` | 0.7, 3 | 8.5pt | 2pt |
| `namespace-capacity` | 0.8, 1, 1, 1, 1, 0.9, 0.9 | 7.1pt | 3.5pt |
| `workload-resources` | 2.2, 0.45, 0.65, 0.65, 0.72, 0.72, 0.72, 0.72, 0.72, 0.72 | 6.2pt | 3.5pt |
| `hpa-observed` | 2.2, 0.65, 0.65, 0.8, 0.72, 0.72, 1.15 | 7pt | 3.5pt |
| `workloads-without-hpa` | 0.9, 2.2, 0.65, 2.1 | 7.4pt | 3.5pt |
| `nodepool-summary` | 1.6, 0.45, 0.75, 0.75, 0.85, 0.85, 0.9 | 7.1pt | 3.5pt |
| `nodepool-profile` | 1.5, 0.9, 0.85, 0.95, 1, 0.95, 1.05 | 6.8pt | 3.5pt |
| `node-detail` | 2.4, 1.5, 1.6, 2.2, 1.2 | 7.1pt | 3.5pt |
| `priority-workloads` | 1.25, 2, 0.55, 4 | 7.2pt | 3.5pt |
| `namespaces-at-risk` | 1.35, 0.75, 0.75, 3, 1.25, 0.85 | 7.2pt | 3.5pt |
| `replica-ceiling` | 1.25, 2, 0.9, 1.5, 1.7 | 7.2pt | 3.5pt |
| `nodepool-status` | 1.8, 0.9, 3.5 | 7.3pt | 3.5pt |
| `action-plan` | 0.8, 0.8, 2.7, 2.6 | 7.2pt | 3.5pt |
| `material-limitations` | 1.8, 0.8, 3 | 7.5pt | 3.5pt |
| `quality-appendix` | 1.6, 0.8, 1, 2, 2.4 | 7pt | 3.5pt |
| `interpretation-notes` | 1 | 7.5pt | 3.5pt |

**Rendering requirements:**

- `main.typ` reads the file named by `sys.inputs.at("data")` with `json`,
  validates it, and applies
  `graphite-report.with(title: <cover>, outline: false, footer: <footer>, labels: <labels as content>, theme: (lang, region, date-format))`.
- Every table goes through one helper that reproduces `data-table` from
  `sections.py` lines 44 to 89: text size from the layout, paragraph not
  justified with leading `0.45em` and spacing `0.4em`, inset `x: 3pt` and the
  layout's `y`, stroke `0.5pt`, alignment `left + top`, header cells bold,
  `table.header(repeat: true, ...)`.
- Block `front` reproduces the opening block of the original: its content is
  set with text size `8pt` and paragraph leading `0.45em` and spacing `0.4em`.
- A heading with `numbered` false is set with `numbering: none` and
  `outlined: false`.
- Block `list` is a bullet list.
- Runs:
  - `text`: the string as plain text;
  - `strong`: the string in bold;
  - `ident`: `text(size: 0.92em, hyphenate: false, lang: "en", value)`;
  - `placeholder`: `text(size: 0.92em, value)`;
  - `status`: the mark, then `h(2pt)` and the label when there is one, inside
    a `box`; grey `luma(45%)` for `healthy`, `unavailable`, and `undefined`,
    black otherwise; bold for `critical`.
- A string becomes content by passing it to `text(...)`. It is never parsed.
- Cover and footer reproduce `report-cover` and `report-footer` from
  `sections.py` lines 92 to 136. The author line is left out when `author` is
  null. The footer starts on page 2, is right aligned, and reads
  `<classification> · <page_label> <n>`; without a classification it reads
  `<page_label> <n>`.
- The cluster name on the cover is an `ident` run.

**Diagnostics.** Each is a `panic` whose message starts with
`capacity-report: `:

| Condition | Message after the prefix |
|---|---|
| Input `data` absent | `missing input "data"` |
| Wrong schema | `unsupported schema "<value>"; this template reads schema "1"` |
| Unknown key | `unknown key "<key>" in <place>; allowed keys: <list>` |
| Missing key | `missing key "<key>" in <place>` |
| Wrong type | `` `<path>` must be <expected>; received <type> `` |
| Unknown language | `unknown language "<value>"; allowed: pt-BR, en` |
| No blocks | `` `blocks` must hold at least one block `` |
| Unknown block type | `unknown block type "<value>" at blocks[<i>]` |
| Heading level | `heading level must be 1, 2, or 3; received <n> at blocks[<i>]` |
| Unknown table | `unknown table id "<id>" at blocks[<i>]` |
| Header count | `table "<id>" has <n> columns but <m> headers` |
| No rows | `table "<id>" at blocks[<i>] has no rows` |
| Cell count | `table "<id>" has <n> columns but row <r> has <m> cells` |
| Unknown run kind | `unknown run kind "<value>" at <path>` |
| Unknown status | `unknown status "<value>" at <path>` |
| Empty runs | `` `runs` must not be empty at <path> `` |
| Empty list | `` `items` must not be empty at blocks[<i>] `` |
| Misplaced front | `block "front" is allowed only as the first block` |
| Nested block | `block "<type>" is not allowed inside "front" at blocks[0].blocks[<j>]` |

Row numbers start at 1. `<place>` is `report`, `theme`, `labels`, `cover`,
`footer`, or `blocks[<i>]`.

**Negative fixtures.** One data file per row of the table above, in
`template/tests/capacity/negative/`, each with its `.expect`. Start each from
a small valid file and break one thing. The file for the absent input is
replaced by a test that compiles without the input.

**Tests with tag `template`:**

| Test | Expected |
|---|---|
| `TestCapacityNegativeFixturesFail` | each file fails and stderr contains its `.expect` |
| `TestCapacityEveryNegativeHasAnExpectation` | pairs complete |
| `TestCapacityMissingInputFails` | compile without `--input` fails with `missing input "data"` |
| `TestCapacityEveryDiagnosticIsCovered` | each message pattern in `validate.typ` that starts with `capacity-report: ` is matched by at least one `.expect` |
| `TestCapacityTableLayoutsMatchThePlan` | compiles a document that imports `tables.typ` and asserts, with `#assert`, the 17 identifiers and the column count of each |

- [ ] **Step 1:** Write the negative fixtures, the `.expect` files, and
  `capacity_negative_test.go`.
- [ ] **Step 2:** With native tools, run
  `go test -tags=template ./template/ -run Capacity -v`.
  Expected: FAIL, the entry file does not exist.
- [ ] **Step 3:** Write the six Typst files and extend `embed.go`.
- [ ] **Step 4:** Run the same command. Expected: PASS.
- [ ] **Step 5:** Run `go test ./template/ -v`. Expected: PASS, including the
  embed tests with the new folder.
- [ ] **Step 6:** Commit.

```bash
git add template/capacity template/tests/capacity/negative template/capacity_negative_test.go template/embed.go
git commit -m "feat(template): add the report layer"
```

---

### Task 7: Report fixtures

**Executor:** Subagent for Portuguese. Main thread for English.

**Files:**
- Create in `template/tests/capacity/fixtures/`: `detailed.pt-BR.json`,
  `detailed.en.json`, `minimal.pt-BR.json`, `identifiers.pt-BR.json`,
  `literal.pt-BR.json`

**Requirements:**

| Fixture | Content |
|---|---|
| `detailed.pt-BR` | The detailed profile in the section order of spec section 10.3. Every block type and all 17 table identifiers appear at least once. Wording and values come from `detailed-all.typ`. At least 4 pages. |
| `detailed.en` | The same structure and values as `detailed.pt-BR`, in English, with `theme.lang` `en`, `region` `US`, date as month/day/year, decimals with a point. |
| `minimal.pt-BR` | `author` null, `classification` null, one heading, one paragraph, one `pagebreak`, one more paragraph, so that page 2 exists. |
| `identifiers.pt-BR` | One `node-detail` table. Row 1: `aks-generalpool-29715314-vmss000001` as `ident`. Row 2: the same string as `text`, the control. Row 3: `aksgeneralpool29715314vmss000001` as `ident`, which has no break point. |
| `literal.pt-BR` | One paragraph per string of the list below, each as `text`, and the same strings again as `ident` in an `interpretation-notes` table. |

Strings for `literal`:

```text
#let x = 1
*negrito*
_sublinhado_
$x^2$
<rotulo>
@referencia
`codigo`
[colchetes]
// comentario
\n literal
https://example.com/a?b=c&d=e
linha um<actual newline>linha dois
tab<actual tab>ulado
ação çedilha ñ ü 日本語
```

The English vocabulary is fixed by spec section 9.3: request, limit, workload,
node pool, right-sizing, headroom, autoscaling, CPU throttling, label.
Kubernetes names and finding codes are not translated. The unknown node pool
reads `Undetermined`.

- [ ] **Step 1:** Subagent writes the four Portuguese fixtures.
- [ ] **Step 2:** Confirm each is valid JSON: `python3 -m json.tool < file > /dev/null`.
- [ ] **Step 3:** Confirm coverage of `detailed.pt-BR`: every block type and
  every table identifier appears.

```bash
python3 - <<'EOF'
import json
d = json.load(open("template/tests/capacity/fixtures/detailed.pt-BR.json"))
types = {b["type"] for b in d["blocks"]}
ids = {b["id"] for b in d["blocks"] if b["type"] == "table"}
print(sorted({"outline","heading","paragraph","table","callout","pagebreak"} - types))
print(17 - len(ids))
EOF
```

Expected: `[]` and `0`.

- [ ] **Step 4:** Main thread writes `detailed.en.json` from the Portuguese
  one and checks that both have the same number of blocks, the same block
  types in the same order, and the same table identifiers.
- [ ] **Step 5:** Commit.

```bash
git add template/tests/capacity/fixtures
git commit -m "test(template): add report fixtures"
```

---

### Task 8: Report tests

**Executor:** Subagent. Main thread runs them in the image.

**Files:**
- Create: `template/capacity_test.go` (tag `template`)
- Modify: `template/golden/snapshots.go`

**Tests with tag `template`:**

| Test | Fixture | Expected |
|---|---|---|
| `TestCapacityFixturesCompileCleanly` | all five | clean compile, PDF not empty |
| `TestCapacityIsDeterministic` | `detailed.pt-BR`, twice | same SHA-256 |
| `TestCapacityIdentifiersSurviveTheTextLayer` | `detailed.pt-BR` and `detailed.en` | every `ident` string of the fixture is found in the text layer once white space is removed from both |
| `TestCapacityIdentDoesNotRepeatTheHyphen` | `identifiers.pt-BR` | with white space removed, the text holds `aks-generalpool-29715314-vmss000001` from row 1 |
| `TestCapacityControlRepeatsTheHyphen` | `identifiers.pt-BR` | the text of row 2 holds a doubled hyphen. If this fails, the test above proves nothing. |
| `TestCapacityPrintsMarkupLiterally` | `literal.pt-BR` | each string of Task 7, except the two with control characters, is found in the text layer |
| `TestCapacityControlCharactersAreNotEscaped` | `literal.pt-BR` | the text holds `linha um` and `linha dois`, and holds neither the two characters `\` `n` from that string nor `\t` |
| `TestCapacityMinimalCoverOmitsAuthor` | `minimal.pt-BR` | page 1 text has no `ELABORADO POR` |
| `TestCapacityFooterWithoutClassification` | `minimal.pt-BR` | page 2 text holds `Página 2` and no `·` |
| `TestCapacityFooterWithClassification` | `detailed.pt-BR` | page 2 text holds `Uso interno · Página 2` |
| `TestCapacityFirstPageHasNoFooter` | `detailed.pt-BR` | page 1 text holds no `Página 1` |
| `TestCapacityEnglishUsesEnglishLabels` | `detailed.en` | page 2 text holds `Page 2`; no `Página` anywhere |
| `TestCapacityPassesEveryLabel` | `main.typ` source | the call to `graphite-report` passes `labels` with the seven keys and `theme` with `lang`, `region`, and `date-format` |

Text of one page comes from `pdftotext -f <n> -l <n> -layout`.

**Snapshots to add:**

| Name | Source | Input | Pages | Protects |
|---|---|---|---|---|
| `capacity-pt` | `capacity/main.typ` | `detailed.pt-BR` | 1, 2, 3 | Cover, auxiliary page, and the first dense tables in Portuguese |
| `capacity-en` | `capacity/main.typ` | `detailed.en` | 1, 2, 3 | The same pages in English: labels, date order, and hyphenation |
| `capacity-minimal` | `capacity/main.typ` | `minimal.pt-BR` | 1, 2 | A cover with no author and a footer with no classification |
| `capacity-identifiers` | `capacity/main.typ` | `identifiers.pt-BR` | 1 | Identifier wrapping in the report's own table, and silent overflow |

The harness copies the fixture into the workspace as `report.json` and passes
`data=/report.json`.

- [ ] **Step 1:** Write `capacity_test.go` and extend `Snapshots`.
- [ ] **Step 2:** With native tools, run
  `go test -tags=template ./template/ -run Capacity -v`. Expected: PASS.
- [ ] **Step 3:** Main thread runs `make test-template`. Expected: PASS.
- [ ] **Step 4:** Prove two tests can fail, one at a time, restoring each: in
  `runs.typ` remove `lang: "en"` from `ident`, expect
  `TestCapacityIdentDoesNotRepeatTheHyphen` to fail; in `cover.typ` always
  print the author line, expect `TestCapacityMinimalCoverOmitsAuthor` to fail.
- [ ] **Step 5:** Commit.

```bash
git add template/capacity_test.go template/golden/snapshots.go
git commit -m "test(template): cover the report layer"
```

---

### Task 9: Produce and review the report golden pages

**Executor:** Main thread.

- [ ] **Step 1:** Run `make update-golden`. Expected: 9 new PNG files, 22 in
  total. The 13 generic pages must keep their digests; if one changed, stop
  and find out why.
- [ ] **Step 2:** Review each new page against the matching page of a PDF
  compiled from `older/kubernetes-report/tests/golden/typst/detailed-all.typ`
  under the original package. Check the cover, the outline, table widths,
  text sizes, header repetition, status marks, and the footer.
- [ ] **Step 3:** Review the English pages for wording, date order, and
  hyphenation of English prose.
- [ ] **Step 4:** Review `capacity-identifiers-1.png`: row 1 wraps without a
  doubled hyphen, row 2 shows the doubled hyphen, row 3 overflows its cell.
- [ ] **Step 5:** Record every layout finding in `ROADMAP.md`. Fix those that
  belong to this phase in Task 6 and produce the pages again.
- [ ] **Step 6:** Run `make test-golden`. Expected: PASS.
- [ ] **Step 7:** Commit.

```bash
git add template/tests/golden
git commit -m "test(template): add report golden pages"
```

---

### Task 10: Compile as the runtime user

**Executor:** Main thread.

**Files:**
- Modify: `template/capacity_test.go`
- Modify: `Makefile`

**Requirements:**

- Add `TestCapacityCompilesFromUnpackedTemplate` (tag `template`): call
  `template.Unpack` into a temp folder, write `detailed.pt-BR` there as
  `report.json`, compile with `template.Entry` and `template.FontPath`, and
  expect a clean compile whose PDF has the same SHA-256 as the one compiled
  from the source tree. This proves the embedded copy is what production
  will run.
- `make test-template` already runs as user `10001` over a read-only source.
  Add to it `--read-only` and `--tmpfs /tmp`, so the test image has the same
  file system limits as the runtime image.

- [ ] **Step 1:** Write the test.
- [ ] **Step 2:** Change the `Makefile`.
- [ ] **Step 3:** Run `make test-template`. Expected: PASS.
- [ ] **Step 4:** Commit.

```bash
git add template/capacity_test.go Makefile
git commit -m "test(template): compile from embedded copy"
```

---

### Task 11: Close the phase

**Executor:** Main thread.

- [ ] **Step 1:** Run `make gate`. Expected: pass, with `test-template` and
  `test-golden` among the steps.
- [ ] **Step 2:** Confirm from the gate logs that no test was skipped and
  that the counts are what this plan lists.
- [ ] **Step 3:** Dispatch the `adversarial-reviewer` subagent over
  `git diff main...feat/template` with the spec and this plan. Ask it in
  particular to look for a way to make the template interpret data as markup,
  and for a malformed data file that compiles without error.
- [ ] **Step 4:** Judge each finding. Fix what is confirmed. Record what is
  declined in `ROADMAP.md` with the reason.
- [ ] **Step 5:** Update `ROADMAP.md` in its four places and read it back.
- [ ] **Step 6:** Update `README.md` and `AGENTS.md` with the new targets
  `test-template`, `test-golden`, and `update-golden`, and with the rule for
  golden pages.
- [ ] **Step 7:** `CHANGELOG.md` stays as it is: no command uses the template
  yet, so a user perceives nothing. Declare the omission when reporting.
  `VERSION` stays `0.0.0`. No tag.
- [ ] **Step 8:** Run `make gate` again if anything changed since Step 1.
- [ ] **Step 9:** Integrate by fast-forward.

```bash
git switch main
git merge --ff-only feat/template
git branch -d feat/template
```

- [ ] **Step 10:** Report to the owner: what was delivered, which generic
  pages are byte-identical to the original, the layout findings, the declared
  omission, and what is left open.

## Definition of done

- `make gate` passes on `main` after the merge.
- The generic layer differs from `graphite-report` 1.0.0 only in comments,
  one diagnostic, the manifest, and import paths.
- 22 golden pages exist, each reviewed, and none has a coloured pixel.
- The report compiles from `detailed.pt-BR.json` and from `detailed.en.json`.
- Every diagnostic of both layers has a failing fixture.
- Text that looks like markup is printed as written.
- The embedded template produces the same PDF as the source tree.
- The package installs by copy and compiles through its name.

## Contract extension of 2026-09-28

The first rendered pages, compared with the original, showed four things the
contract lacked: the opening block set in smaller type, headings that are
not numbered, a bullet list, and bold labels inside a sentence. The contract
gained the block types `front` and `list`, the heading field `numbered`, and
the run kind `strong`. Spec section 10.2 was updated to match.

## Changes after the adversarial review of 2026-09-28

The text of Tasks 4, 6, and 8 above is kept as it was written. These changes
win where they disagree with it.

- `theme` holds `lang` and `region` only. `labels` holds `note`, `info`, and
  `warning` only. `date_format` and the labels `author`, `date`,
  `classification`, and `outline` were required and reached no page.
- Three diagnostics were added: `duplicate-outline`, `trailing-pagebreak`,
  and an empty or blank table header under `wrong-type`. With the three of
  the contract extension, the report layer has twenty-one.
- `Snapshot` has a field `Data`, the data file of a report snapshot, and a
  field `Omits`, which says which pages a snapshot leaves out and why.
- `capacity-pt` and `capacity-en` cover every page of their documents.
- A fifth report snapshot, `capacity-parity`, renders an exact transcription
  of the original report.

## Self-review against the spec

| Spec | Task |
|---|---|
| 10.1 Division of work | 6 |
| 10.2 Data file | 6, 7. The cover and footer are top-level keys, not blocks, because the generic layer receives them as arguments. |
| 10.3 Content | 7, 9. Section order is fixed by the fixture now and by Go in phase 2. |
| 10.4 Generic layer | 2, 3 |
| 10.5 Compile | 1, 10. The absolute path and empty environment are in `internal/typst`. |
| 10.6 Licences | 2, 3 (`TestFontLicenceIsEmbedded`). The image part belongs to phase 2, when the binary embeds the template. |
| 13.3 Template tests | 3, 4, 5, 8, 9 |
| 14.1 Compile as the runtime user | 10, in the test image. The runtime image gains a compile command in phase 2. |
