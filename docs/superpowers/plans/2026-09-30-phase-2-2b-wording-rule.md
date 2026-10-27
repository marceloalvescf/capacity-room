# Phase 2.2b: The Wording Rule Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** The Portuguese report writes every technical term in English and every common noun in Portuguese, the guard of the catalog enforces that rule, and every test that compared the Portuguese wording with the original compares by code or by key instead, so that the numbers still equal the original while the words follow the owner.

**Architecture:** Nothing moves between layers. The rule lives in three places: the design (spec 9.3 and section 3), the banned-terms guard of `internal/catalog`, which changes sides, and 58 entries of `locales/pt-BR.toml`. Two tests that read the Portuguese wording of the original, the comparison with the Python tool and the parity test of the document, are made independent of the wording first, so that the catalog can change without touching a recorded file or the parity fixture. The golden data files and pages in Portuguese are regenerated last and reviewed page by page.

**Tech Stack:** Go 1.27 and the modules already in `go.mod`. No new module. Typst 0.15.1, `pdftotext`, Docker for the golden pages and the gate.

**Spec:** `docs/superpowers/specs/2026-09-27-k8s-capacity-report-design.md`, sections 3, 9.1, 9.3, 9.5, 10.3, 13.2, 15; Task 1 of this plan amends 3 and 9.3. The decision of the owner of 2026-09-30 is recorded in `ROADMAP.md` under the decisions of the owner and as finding F-74.

**Depends on:** Part 2.2, merged into `main` at `8847162`.

## Where this part sits

| Part | Delivers | PDF at the end |
|---|---|---|
| 2.1 Spine, merged | Configuration, redaction, client, discovery, the fake server, the catalog mechanism, the four modes of `report` | Cover, opening page, quality appendix |
| 2.2 Namespaces and workloads, merged | Range queries, two collectors, workload identity, the finding registry with 40 codes, five formats, the first comparison with the Python tool | Sections 1 and 2, interpretation notes |
| **2.2b The wording rule (this plan)** | The rule in the design, the guard, 58 Portuguese entries, two comparisons freed from the wording | The same pages, reworded in Portuguese |
| 2.3 Autoscalers and nodes | HPA, nodes, node pools, cluster autoscaling, 26 finding codes, written under the rule | Sections 3 and 4 |
| 2.4 Executive profile and proofs | Recommendations, minimum severity of the executive body, the complete catalog, the finding sentences, a real cluster | Both profiles |

The part is numbered 2.2b, and not 2.3, so that every reference to parts 2.3 and 2.4 in `ROADMAP.md`, in the plan of part 2.2, and in the findings F-35, F-62, and F-66 keeps its meaning.

### Out of this part

| Left out | Arrives in | Reason |
|---|---|---|
| Any collector, code, section, or snapshot field | 2.3 | This part changes words and the tests of words, nothing else |
| The English entries of the second batch | none | The owner asked for no change; `1 cores` stays, as ruling R32 of part 2.2 keeps units out of the catalog |
| The Portuguese sentences of the findings and the recommendations | 2.4 | They enter the catalog with the pages that print them (R21) |
| The fixtures of the template layer in Portuguese: `detailed`, `minimal`, `identifiers`, `parity` | none | Ruling W3 below |

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
- **Main thread**: needs Docker, judgment, wording, page review, or the
  owner.

Rules for every subagent brief:

- Tasks run in sequence. Two agents never share the checkout.
- A reviewer receives a diff file under `.cache/phase-2/part-2b/` and runs
  tests only on packages nobody is editing.
- Every commit names its paths: `git add <paths>`, never `git add -A`.
- A subagent never runs `git stash`, `git restore`, `git checkout` of a
  path, or `git clean`.
- A test of a change goes red on the code before the change, test by test,
  and the brief asks for the red run.
- A subagent has no `ReportFindings` tool. A reviewer writes its findings to
  a file under `.cache/phase-2/part-2b/` and repeats them in its last message.
- A decision flagged at the end of a report has already been made. A brief
  that says "stop and report" means stop before deciding.

Reference material, read-only and never tracked:

- `older/kubernetes-report/kube_capacity_report/`: the Python source. Its
  Portuguese sentences are the ones the tables of Task 2 hold.
- `.cache/phase-2/part-2/english-wording.md`: the second English batch, with
  the Portuguese of the original beside it.

## Global Constraints

Every constraint of the plans of parts 2.1 and 2.2 stands. The ones below
are the ones this part leans on, and the one it changes.

- Go 1.27. Module `github.com/marceloalvescf/k8s-capacity-report`. No new
  module enters `go.mod`.
- Wording lives only in `internal/catalog`. **Every technical term is
  written in English in both languages; only a common noun is written in
  Portuguese.** This replaces the constraint of part 2.2 that the Portuguese
  sentences are carried over unchanged from the original.
- Kubernetes names (`Deployment`, `StatefulSet`, `DaemonSet`, `Job`,
  `CronJob`, `Pod`, namespace, `HPA`, `OOMKill`) and finding codes stay as
  they are in both languages.
- No em-dash inside a sentence. The em-dash alone marks an empty table cell.
- Every key exists in both languages with the same parameters. An entry uses
  the one template form `{{.Name}}`. No entry holds a banned term.
- The numbers equal the original exactly. The files of
  `testdata/prometheus/`, `testdata/prometheus-own/`, and `testdata/python/`
  are recorded answers and are never edited.
- The pages of the golden snapshot `capacity-parity` equal, byte for byte,
  the pages of the original report. The fixture
  `template/tests/capacity/fixtures/parity.pt-BR.json` is not edited.
- A test never writes a golden file or a golden page. `make update-testdata`
  and `make update-golden` do, on the main thread.
- No test skips. No comment speaks of the process.
- Diagnostics are in English and are not in the catalog.
- Scratch files go under `.cache/`, never under `/tmp` on the host.
- Commits follow `type(scope): description`, imperative, subject under 50
  characters.

## The vocabulary

The owner approved this table on 2026-09-30. It replaces the table of spec
9.3 in Task 1, and the guard of Task 3 enforces its first two rows.

| Concept | Portuguese report | English report |
|---|---|---|
| request, limit, workload, label, node, node pool, replica, autoscaling, autoscaler, recording rule, raw metrics, throttling | the English word: request, limit, workload, label, node, node pool, replica, autoscaling, autoscaler, recording rule, raw metrics, throttling | the same |
| Kubernetes names and finding codes | as they are | as they are |
| memory, resource, usage, peak, average, coverage, source, remark, period, confidence, headroom, right-sizing | memória, recurso, uso, pico, média, cobertura, fonte, observação, período, confiança, margem, dimensionamento | memory, resource, usage, peak, average, coverage, source, remark, period, confidence, headroom, right-sizing |

An English noun inside a Portuguese sentence is masculine and takes an
English plural: o request, os requests; o limit, os limits; o workload, os
workloads; o node, os nodes; o node pool, os node pools; o label, os labels;
o HPA, os HPAs. A participle or adjective agrees with it: "1 workload
deixado de fora", "os dados de utilização dos nodes".

## Review Focus

Each line names an input that the spec implies and does not describe, and the
task whose tests pin it.

1. **A whole English sentence pasted into the Portuguese file.** Every noun
   in it may be technical, so the rule on nouns lets it through; the English
   function words `the`, `is`, `are`, `not`, `with`, and their neighbours
   stay banned in Portuguese and catch it. Pinned in Task 3.
2. **The word `nós` as a pronoun.** It is also the Portuguese for `nodes`,
   which the rule bans. No reader sentence of the report says "we", so the
   guard bans the word in every sense. Pinned in Task 3.
3. **A text of the parity fixture that the mapping does not know.** After
   the catalog changes, such a text stays as the original wrote it, the
   document writes the new word, and the comparison fails naming the block.
   A test proves the substitution changes the outcome. Pinned in Task 2.
4. **A notice of the original with a count other than one.** The recorded
   scenarios hold counts of one only. The table of the original's sentences
   renders the plural with the count written as Python writes an integer,
   and a unit test pins `3`. Pinned in Task 2.
5. **A code with no sentence of the original in the table.** The comparison
   would then pass by comparing nothing. A test walks
   `collect.WarningCodes` and `collect.NoticeCodes` and wants a sentence for
   each. Pinned in Task 2.

## Rulings made on the owner's behalf

Each ruling names its cost if wrong. The owner confirms or overturns them
with the plan.

| ID | Ruling | Cost if wrong |
|---|---|---|
| W1 | The part is numbered 2.2b, its branch is `feat/wording-rule`, its identifiers in `ROADMAP.md` are `P22B-0` to `P22B-5`, its scratch folder is `.cache/phase-2/part-2b/` | a rename |
| W2 | `headroom` and `right-sizing` stay Portuguese, `margem` and `dimensionamento`, and the two English words stay banned in Portuguese: each has a plain common noun, and the owner's table did not name them | two words in the finding sentences of part 2.4 |
| W3 | The fixtures of the template layer in Portuguese, `detailed`, `minimal`, `identifiers`, and `parity`, keep the words of the original: they are inputs of the template, transcribed from the original, not text the program writes; the parity pages must stay equal to the original's | none for the reader; a page of the template tests shows the old Portuguese |
| W4 | The comparison with the Python tool renders the original's Portuguese sentences from a table in the test, keyed by code, and compares them with the recorded sentences; the recorded files are not touched and the catalog is not read | the table holds the original's sentences a second time, in a test |
| W5 | The parity test of the document maps each text of the fixture that is a catalog text to its key and reads the current text of that key from the catalog; numbers, statuses, identifiers, and the shape of the blocks are compared as before | a text of the fixture that also appears as a number would be mapped; none does |
| W6 | The change is a `Changed` entry under `Unreleased` in `CHANGELOG.md`: the Portuguese report is what a reader perceives, and the seven `Added` entries of part 2.2 describe the sections, not their words | one changelog line too many, in an unreleased section |
| W7 | The word `nodepool`, one word, is banned in Portuguese as it is in English; the report writes `node pool` in both | none |
| W8 | The test `TestPortugueseOfThisPartEqualsTheOriginal` is renamed `TestPortugueseOfTheSectionsMatchesThePlan`, keeps its three tables with the new wording, and its comment says that it pins the wording the owner approved | a rename |
| W9 | The heading `Capacidade por Namespace` keeps its capital `N`, as the original wrote it; the rule speaks of the language of a word, not of its case | a capital letter |

## Findings this plan closes or records

| Finding | How |
|---|---|
| F-74 | Closed by Tasks 1, 3, and 4 |

## File structure

| Path | Responsibility in this part |
|---|---|
| `docs/superpowers/specs/2026-09-27-k8s-capacity-report-design.md` | Section 3 gains a row; section 9.3 gets the new table and rule |
| `AGENTS.md` | The rule, under "Rules kept by review" |
| `CHANGELOG.md` | One `Changed` entry under `Unreleased` |
| `internal/catalog/locales/pt-BR.toml` | The 58 entries |
| `internal/catalog/guard_test.go` | `bannedTermsFor` changes sides; the wording tables hold the new Portuguese |
| `internal/goldentest/original_test.go` | The sentences of the original in a table, keyed by code |
| `internal/document/sections_test.go` | The parity test maps the fixture's texts to catalog keys |
| `template/tests/capacity/fixtures/command/*.pt-BR.json` | Regenerated by `make update-testdata` |
| `template/tests/golden/command-complete-pt-*.png`, `command-edge-pt-*.png`, `SHA256SUMS` | Regenerated by `make update-golden` |

---

### Task 0: Open the part

**Executor:** Main thread.

- [ ] **Step 1:** Run `git status`, `git log --oneline -3`, and
  `go test -count=1 ./...`. Read `ROADMAP.md` and `CHANGELOG.md` and confirm
  they describe what the commands show: `main` at `8847162`, part 2.2 merged,
  seven entries of part 2.2 under `Unreleased`.
- [ ] **Step 2:** Confirm the branch.

```bash
git branch --show-current
```

Expected: `feat/wording-rule`.

- [ ] **Step 3:** In `ROADMAP.md`, add the row `2.2b The wording rule` to the
  summary table, after 2.2, with this plan's path and the status `OPEN,
  branch feat/wording-rule`; add the section `## Phase 2.2b. The wording
  rule — OPEN` with one row per task, identifiers `P22B-0` to `P22B-5`,
  source `PLAN`; and record under the decisions of the owner what the owner
  said of the rulings W1 to W9.
- [ ] **Step 4:** Commit the plan.

```bash
git add docs/superpowers/plans/2026-09-30-phase-2-2b-wording-rule.md
git commit -m "docs(plan): add the plan of phase 2.2b"
```

**Done when:** the branch is `feat/wording-rule`, `ROADMAP.md` names every
task of this plan, and the owner has approved the plan and its rulings.

---

### Task 1: The design says the rule

**Executor:** Main thread. Wording of the design stays on the main thread.

**Files:**
- Modify: `docs/superpowers/specs/2026-09-27-k8s-capacity-report-design.md`, section 3 (the table of deliberate differences) and section 9.3
- Modify: `AGENTS.md`, "Rules kept by review"
- Modify: `CHANGELOG.md`, `## [Unreleased]`

- [ ] **Step 1:** In section 3 of the spec, add this row at the end of the
  table of deliberate differences:

```markdown
| Portuguese sentences translate every term: solicitação, limite, carga de trabalho, pool de nós | Every technical term is English in both languages; only a common noun is Portuguese | Owner's decision of 2026-09-30: Brazilian readers use the Kubernetes vocabulary in English |
```

- [ ] **Step 2:** Replace the table of section 9.3 and its first two bullets
  with this text. The remaining bullets of the section, on the em-dash, the
  marker `unknown`, and one entry per resource, stay as they are.

```markdown
### 9.3 Vocabulary

Every technical term is written in English in both languages. Only a common
noun is written in Portuguese. A term is technical when it names a
Kubernetes or Prometheus object, field, or mechanism.

| Concept | Portuguese report | English report |
|---|---|---|
| request, limit, workload, label, node, node pool, replica, autoscaling, autoscaler, recording rule, raw metrics, throttling | the English word | the same |
| Kubernetes names and finding codes | as they are | as they are |
| memory, resource, usage, peak, average, coverage, source, remark, period, confidence, headroom, right-sizing | memória, recurso, uso, pico, média, cobertura, fonte, observação, período, confiança, margem, dimensionamento | memory, resource, usage, peak, average, coverage, source, remark, period, confidence, headroom, right-sizing |

- An English noun inside a Portuguese sentence is masculine and takes an
  English plural: o request, os requests; o workload, os workloads; o node
  pool, os node pools. A participle or adjective agrees with it.
- Kubernetes names (`Deployment`, `StatefulSet`, `DaemonSet`, `Job`,
  `CronJob`, `Pod`, namespace, `HPA`, `OOMKill`) and finding codes stay as
  they are in both languages.
- The Portuguese sentences follow the sense of the original and its
  numbers; their words follow this table. The guard of the catalog bans the
  translations of the first row in Portuguese, and English function words
  beside them.
```

- [ ] **Step 3:** In `AGENTS.md`, under "Rules kept by review", add this
  bullet after "Diagnostics are in English":

```markdown
- **Technical terms are English in both languages.** A Portuguese sentence
  writes request, limit, workload, node, node pool, label, replica,
  autoscaling, recording rule, raw metrics, and throttling as English words,
  masculine, with an English plural. Only a common noun is Portuguese. The
  guard bans the translations; the reviewer judges the border cases against
  the table of spec 9.3.
```

- [ ] **Step 4:** In `CHANGELOG.md`, under `## [Unreleased]`, add a group
  `### Changed` after the `### Added` group, with one item:

```markdown
### Changed

- The Portuguese report writes the technical terms of Kubernetes in English: request, limit, workload, node, node pool, label, and autoscaling.
```

- [ ] **Step 5:** Run the guard of the changelog and read the three files
  back.

```bash
go test -count=1 ./internal/guard/ -run Changelog -v
git diff --stat
```

Expected: the guard passes; three files changed.

- [ ] **Step 6:** Commit.

```bash
git add docs/superpowers/specs/2026-09-27-k8s-capacity-report-design.md AGENTS.md CHANGELOG.md
git commit -m "docs(spec): write technical terms in English"
```

**Done when:** spec 9.3 holds the table above and no longer says that the
Portuguese is carried over unchanged, section 3 has the row, `AGENTS.md` has
the rule, and the changelog has the entry.

---

### Task 2: The comparisons stop reading the Portuguese wording

**Executor:** Subagent.

Two tests compare text that the catalog produces with text that the original
wrote. After this task both compare by code or by key, and both pass before
and after the catalog changes. The recorded files and the parity fixture are
not edited.

**Files:**
- Modify: `internal/goldentest/original_test.go` (the function `catalogTexts` at about line 504, its caller `compareNumbers` at about line 561, and the import of `internal/catalog` at line 16)
- Modify: `internal/document/sections_test.go` (the function `TestDocumentEqualsParityForItsSections` at about line 516)

**Interfaces:**
- Consumes: `collect.WarningCodes`, `collect.NoticeCodes`, and the constants `collect.Warning*` and `collect.Notice*` of `internal/collect/collect.go`; `snapshot.Message` with fields `Code string` and `Params map[string]float64`; `catalog.Catalog.Text(key catalog.Key, params catalog.Params) (string, error)`; the key constants of `internal/catalog/keys.go`.
- Produces, in `original_test.go`: `originalWarnings map[string]string`, `originalNotices map[string]originalNotice` with `type originalNotice struct{ singular, plural string }`, `func originalWarningText(code string) (string, error)`, `func originalNoticeText(code string, count float64) (string, error)`, and `func originalTexts(warnings, notices []snapshot.Message) ([]string, []string, error)`, which replaces `catalogTexts`.
- Produces, in `sections_test.go`: `parityTexts map[string]catalog.Key` and `func withCatalogTexts(t *testing.T, cat *catalog.Catalog, v any) any`.

#### Part A: the sentences of the original, in the goldentest package

- [ ] **Step 1:** Write the failing tests in `original_test.go`, beside the
  other tests of the file.

```go
// TestOriginalSentencesCoverEveryCode wants a sentence of the original for
// every warning code and for both forms of every notice code, so that the
// comparison with the recorded files never passes by comparing nothing.
func TestOriginalSentencesCoverEveryCode(t *testing.T) {
	for _, code := range collect.WarningCodes {
		if _, err := originalWarningText(code); err != nil {
			t.Errorf("warning %s: %v", code, err)
		}
	}
	for _, code := range collect.NoticeCodes {
		for _, count := range []float64{1, 2} {
			if _, err := originalNoticeText(code, count); err != nil {
				t.Errorf("notice %s with count %v: %v", code, count, err)
			}
		}
	}
	if _, err := originalWarningText("NO_SUCH_WARNING"); err == nil {
		t.Error("originalWarningText(NO_SUCH_WARNING) = nil error, want one")
	}
	if _, err := originalNoticeText("NO_SUCH_NOTICE", 1); err == nil {
		t.Error("originalNoticeText(NO_SUCH_NOTICE, 1) = nil error, want one")
	}
}

// TestOriginalNoticeTextWritesTheCount pins the singular form for a count
// of one and the plural form with the count written as Python writes an
// integer for any other count.
func TestOriginalNoticeTextWritesTheCount(t *testing.T) {
	tests := []struct {
		code  string
		count float64
		want  string
	}{
		{collect.NoticeWorkloadsOmittedNoUsage, 1, "1 carga de trabalho deixada de fora por não ter consumo observado no período."},
		{collect.NoticeWorkloadsOmittedNoUsage, 3, "3 cargas de trabalho deixadas de fora por não terem consumo observado no período."},
		{collect.NoticeJobsOmitted, 1, "1 Job foi deixado de fora do inventário de cargas de trabalho; o consumo continua contabilizado no total do namespace."},
		{collect.NoticeJobsOmitted, 12, "12 Jobs foram deixados de fora do inventário de cargas de trabalho; o consumo continua contabilizado nos totais dos namespaces."},
	}
	for _, tt := range tests {
		got, err := originalNoticeText(tt.code, tt.count)
		if err != nil {
			t.Fatalf("originalNoticeText(%s, %v): %v", tt.code, tt.count, err)
		}
		if got != tt.want {
			t.Errorf("originalNoticeText(%s, %v) = %q, want %q", tt.code, tt.count, got, tt.want)
		}
	}
}
```

- [ ] **Step 2:** Run them and watch them fail.

```bash
go test -count=1 ./internal/goldentest/ -run 'TestOriginalSentences|TestOriginalNoticeText' -v
```

Expected: a build failure naming `originalWarningText` and
`originalNoticeText` as undefined.

- [ ] **Step 3:** Write the table and the three functions in
  `original_test.go`, in place of `catalogTexts`. The sentences are the
  literals of the original, at `collectors/namespaces.py`,
  `collectors/workloads.py`, and `collectors/common.py` of
  `older/kubernetes-report/kube_capacity_report/`; they equal, letter for
  letter, the Portuguese column of `warningWordingTable` and
  `noticeWordingTable` in `internal/catalog/guard_test.go` as that file
  stands before Task 3.

```go
// originalWarnings holds the sentence the original writes for each warning
// code, in Portuguese, letter for letter. The recorded files of
// testdata/python/ hold these sentences, and the comparison renders them
// from the codes of the snapshot rather than from the catalog, whose
// Portuguese follows the vocabulary of spec 9.3 and not the original.
var originalWarnings = map[string]string{
	collect.WarningNamespaceUsageIncomplete:           "As métricas de uso dos namespaces estão incompletas.",
	collect.WarningNamespaceRequestsLimitsIncomplete:  "As métricas de solicitações e limites dos namespaces estão incompletas.",
	collect.WarningNamespaceRequestsLimitsUnavailable: "As métricas de solicitações e limites dos namespaces estão indisponíveis.",
	collect.WarningWorkloadUsageIncomplete:            "As métricas de uso das cargas de trabalho estão incompletas.",
	collect.WarningWorkloadRequestsLimitsIncomplete:   "As métricas de solicitações e limites das cargas de trabalho estão incompletas.",
	collect.WarningWorkloadRequestsLimitsUnavailable:  "As métricas de solicitações e limites das cargas de trabalho estão indisponíveis.",
	collect.WarningWorkloadHistoryCurrentAssociation:  "O histórico usa a associação atual entre pods ativos e cargas de trabalho.",
}

// originalNotice holds the two forms of a notice of the original: the
// sentence for a count of one, and the sentence for any other count with
// %s standing for the count.
type originalNotice struct {
	singular string
	plural   string
}

var originalNotices = map[string]originalNotice{
	collect.NoticeWorkloadsOmittedNoUsage: {
		singular: "1 carga de trabalho deixada de fora por não ter consumo observado no período.",
		plural:   "%s cargas de trabalho deixadas de fora por não terem consumo observado no período.",
	},
	collect.NoticeJobsOmitted: {
		singular: "1 Job foi deixado de fora do inventário de cargas de trabalho; o consumo continua contabilizado no total do namespace.",
		plural:   "%s Jobs foram deixados de fora do inventário de cargas de trabalho; o consumo continua contabilizado nos totais dos namespaces.",
	},
}

// originalWarningText returns the sentence of the original for a warning
// code, or an error naming a code the table does not hold.
func originalWarningText(code string) (string, error) {
	text, ok := originalWarnings[code]
	if !ok {
		return "", fmt.Errorf("no sentence of the original for warning %s", code)
	}
	return text, nil
}

// originalNoticeText returns the sentence of the original for a notice code
// and a count: the singular form for one, otherwise the plural form with
// the count written as Python writes an integer.
func originalNoticeText(code string, count float64) (string, error) {
	notice, ok := originalNotices[code]
	if !ok {
		return "", fmt.Errorf("no sentence of the original for notice %s", code)
	}
	if count == 1 {
		return notice.singular, nil
	}
	return fmt.Sprintf(notice.plural, strconv.FormatFloat(count, 'f', -1, 64)), nil
}

// originalTexts renders the warnings and notices of a snapshot as the
// original would have written them, in the order of the snapshot.
func originalTexts(warnings, notices []snapshot.Message) ([]string, []string, error) {
	warningTexts := make([]string, len(warnings))
	for i, w := range warnings {
		text, err := originalWarningText(w.Code)
		if err != nil {
			return nil, nil, err
		}
		warningTexts[i] = text
	}
	noticeTexts := make([]string, len(notices))
	for i, n := range notices {
		text, err := originalNoticeText(n.Code, n.Params["count"])
		if err != nil {
			return nil, nil, err
		}
		noticeTexts[i] = text
	}
	return warningTexts, noticeTexts, nil
}
```

- [ ] **Step 4:** In `compareNumbers`, remove the load of the Portuguese
  catalog and call `originalTexts` in place of `catalogTexts`:

```go
	gotWarnings, gotNotices, err := originalTexts(snap.Quality.Warnings, snap.Quality.Notices)
	if err != nil {
		diffs = append(diffs, fmt.Sprintf("%s: rendering warnings and notices: %v", scenario, err))
		return diffs
	}
```

  Remove the import of `internal/catalog` if nothing else in the file uses
  it, and add `strconv` to the imports. Update the comment of
  `compareNumbers` where it speaks of the catalog.

- [ ] **Step 5:** Run the tests of the package.

```bash
go test -count=1 ./internal/goldentest/ -v 2>&1 | grep -E '^(=== RUN|--- (PASS|FAIL)|ok|FAIL)' | grep -E 'Original|Numbers|ok|FAIL'
```

Expected: `TestOriginalSentencesCoverEveryCode`,
`TestOriginalNoticeTextWritesTheCount`, `TestNumbersEqualTheOriginal`, and
the test that mutates a recorded notice (at about line 990) all pass; the
package passes.

#### Part B: the parity test maps the fixture's texts to keys

- [ ] **Step 6:** Write the failing test in `sections_test.go`, after
  `TestDocumentEqualsParityForItsSections`.

```go
// TestParitySubstitutionChangesTheFixture proves that withCatalogTexts
// rewrites a text the mapping knows and leaves alone a number, an
// identifier, and a text it does not know, so that the parity comparison
// cannot pass by mapping nothing.
func TestParitySubstitutionChangesTheFixture(t *testing.T) {
	cat := mustCatalog(t, catalog.PortugueseBrazil)
	in := map[string]any{
		"type":    "table",
		"id":      "namespace-capacity",
		"headers": []any{"Recurso", "Solicitação", "% Pico/Limite"},
		"rows": []any{
			[]any{
				[]any{map[string]any{"kind": "ident", "text": "Solicitação"}},
				[]any{map[string]any{"kind": "text", "text": "3,5 cores"}},
				[]any{map[string]any{"kind": "status", "status": "unavailable", "text": "Indisponível"}},
			},
		},
	}

	got := withCatalogTexts(t, cat, in).(map[string]any)

	want := func(key catalog.Key) string {
		text, err := cat.Text(key, nil)
		if err != nil {
			t.Fatalf("Text(%q): %v", key, err)
		}
		return text
	}
	headers := got["headers"].([]any)
	if headers[0] != want(catalog.SectionsNamespacesHeaderResource) ||
		headers[1] != want(catalog.SectionsNamespacesHeaderRequest) ||
		headers[2] != want(catalog.SectionsNamespacesHeaderPeakLimit) {
		t.Errorf("headers = %v, want the catalog texts of resource, request, and peak/limit", headers)
	}
	row := got["rows"].([]any)[0].([]any)
	ident := row[0].([]any)[0].(map[string]any)
	if ident["text"] != "Solicitação" {
		t.Errorf("ident run = %q, want it left alone", ident["text"])
	}
	number := row[1].([]any)[0].(map[string]any)
	if number["text"] != "3,5 cores" {
		t.Errorf("number run = %q, want it left alone", number["text"])
	}
	status := row[2].([]any)[0].(map[string]any)
	if status["text"] != want(catalog.PlaceholderUnavailable) {
		t.Errorf("status run = %q, want the catalog text of the placeholder", status["text"])
	}

	// The input is not changed in place.
	if in["headers"].([]any)[1] != "Solicitação" {
		t.Error("withCatalogTexts changed its input")
	}
}
```

- [ ] **Step 7:** Run it and watch it fail.

```bash
go test -count=1 ./internal/document/ -run TestParitySubstitution -v
```

Expected: a build failure naming `withCatalogTexts` as undefined.

- [ ] **Step 8:** Write the mapping and the walker in `sections_test.go`,
  before `TestDocumentEqualsParityForItsSections`.

```go
// parityTexts maps each text of template/tests/capacity/fixtures/parity.pt-BR.json
// that the catalog produces to its key. The fixture is a transcription of
// the original report and keeps the original's words; the document writes
// the words of the catalog. The parity comparison reads the current text of
// each key so that it compares numbers, statuses, identifiers, and shape,
// and not the words, which the owner may change.
var parityTexts = map[string]catalog.Key{
	"Capacidade por Namespace":       catalog.SectionsNamespacesHeading,
	"Recurso":                        catalog.SectionsNamespacesHeaderResource,
	"Uso médio":                      catalog.SectionsNamespacesHeaderAverage,
	"Pico":                           catalog.SectionsNamespacesHeaderPeak,
	"Solicitação":                    catalog.SectionsNamespacesHeaderRequest,
	"Limite":                         catalog.SectionsNamespacesHeaderLimit,
	"% Pico/Solicitação":             catalog.SectionsNamespacesHeaderPeakRequest,
	"% Pico/Limite":                  catalog.SectionsNamespacesHeaderPeakLimit,
	"CPU":                            catalog.SectionsNamespacesRowCPU,
	"Memória":                        catalog.SectionsNamespacesRowMemory,
	"Recursos por carga de trabalho": catalog.SectionsWorkloadsHeading,
	"Carga de trabalho":              catalog.SectionsWorkloadsHeaderWorkload,
	"Pods":                           catalog.SectionsWorkloadsHeaderPods,
	"CPU média":                      catalog.SectionsWorkloadsHeaderCPUAverage,
	"CPU pico":                       catalog.SectionsWorkloadsHeaderCPUPeak,
	"% CPU/Solicitação":              catalog.SectionsWorkloadsHeaderCPURequest,
	"% CPU/Limite":                   catalog.SectionsWorkloadsHeaderCPULimit,
	"Memória média":                  catalog.SectionsWorkloadsHeaderMemoryAverage,
	"Memória pico":                   catalog.SectionsWorkloadsHeaderMemoryPeak,
	"% Memória/Solicitação":          catalog.SectionsWorkloadsHeaderMemoryRequest,
	"% Memória/Limite":               catalog.SectionsWorkloadsHeaderMemoryLimit,
	"Indisponível":                   catalog.PlaceholderUnavailable,
	"Não definido":                   catalog.PlaceholderNoBase,
}

// parityLabelText is the one run of the fixture that is a catalog text
// followed by a space: the namespace label before the identifier.
const parityLabelText = "Namespace: "

// withCatalogTexts returns a copy of v, a decoded JSON value, in which every
// header and every run that is not an identifier and whose text parityTexts
// knows carries the current catalog text of its key. Every other value is
// copied as it is.
func withCatalogTexts(t *testing.T, cat *catalog.Catalog, v any) any {
	t.Helper()
	replace := func(s string) string {
		if s == parityLabelText {
			text, err := cat.Text(catalog.SectionsNamespaceLabel, nil)
			if err != nil {
				t.Fatalf("Text(%q): %v", catalog.SectionsNamespaceLabel, err)
			}
			return text + " "
		}
		key, ok := parityTexts[s]
		if !ok {
			return s
		}
		text, err := cat.Text(key, nil)
		if err != nil {
			t.Fatalf("Text(%q): %v", key, err)
		}
		return text
	}

	switch x := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(x))
		for k, val := range x {
			out[k] = val
		}
		if headers, ok := x["headers"].([]any); ok {
			replaced := make([]any, len(headers))
			for i, h := range headers {
				if s, ok := h.(string); ok {
					replaced[i] = replace(s)
				} else {
					replaced[i] = h
				}
			}
			out["headers"] = replaced
		}
		if kind, ok := x["kind"].(string); ok && kind != "ident" {
			if s, ok := x["text"].(string); ok {
				out["text"] = replace(s)
			}
		}
		for k, val := range out {
			if k == "headers" || k == "text" {
				continue
			}
			out[k] = withCatalogTexts(t, cat, val)
		}
		return out
	case []any:
		out := make([]any, len(x))
		for i, val := range x {
			out[i] = withCatalogTexts(t, cat, val)
		}
		return out
	default:
		return v
	}
}
```

- [ ] **Step 9:** In `TestDocumentEqualsParityForItsSections`, apply the
  walker to the expected side before the comparison. The lines that decode
  `want` into `wantNorm` gain one call:

```go
	if err := json.Unmarshal(want, &wantNorm); err != nil {
		t.Fatalf("unmarshal want: %v", err)
	}
	wantNorm = withCatalogTexts(t, b.cat, wantNorm)
```

  Update the comment of the test: it proves that the sections built from
  the snapshot equal blocks 2 to 7 of the parity fixture in every number,
  status, identifier, and block, with the words of the fixture read through
  the catalog.

- [ ] **Step 10:** Run the tests of the package.

```bash
go test -count=1 ./internal/document/ -v 2>&1 | grep -E '^(--- (PASS|FAIL)|ok|FAIL)' | grep -E 'Parity|ok|FAIL'
```

Expected: `TestParitySubstitutionChangesTheFixture` and
`TestDocumentEqualsParityForItsSections` pass; the package passes.

- [ ] **Step 11:** Run lint and the whole suite.

```bash
make lint
go test -count=1 ./...
```

Expected: no issue; every package passes. Nothing under `testdata/` or
`template/tests/` changed: `git status --short` names only the two test
files.

- [ ] **Step 12:** Commit.

```bash
git add internal/goldentest/original_test.go internal/document/sections_test.go
git commit -m "test: compare with the original by code and key"
```

**Tests of this task:**

| Test | Input | Expected |
|---|---|---|
| `TestOriginalSentencesCoverEveryCode` | every code of `collect.WarningCodes` and `collect.NoticeCodes`; two unknown codes | a sentence for each; an error for each unknown code |
| `TestOriginalNoticeTextWritesTheCount` | counts 1, 3, 1, 12 | the four sentences above |
| `TestNumbersEqualTheOriginal` | the seven scenarios | unchanged: zero differences |
| the mutation test of a recorded notice, about line 990 | a recorded notice removed, then changed | a difference at that index, as before |
| `TestParitySubstitutionChangesTheFixture` | a table with three headers and three runs | headers and status run replaced; ident and number left alone; input unchanged |
| `TestDocumentEqualsParityForItsSections` | the parity fixture | unchanged: equal blocks |

**Done when:** both packages pass, `TestNumbersEqualTheOriginal` reads no
catalog, the parity test reads the fixture's words through the catalog, and
no file under `testdata/` or `template/tests/` changed.

---

### Task 3: The guard and the catalog follow the rule

**Executor:** Subagent. The Portuguese of the 58 entries is given below,
letter for letter; the implementer copies it and does not reword.

**Files:**
- Modify: `internal/catalog/guard_test.go` (`bannedTermsFor` at about line 668, `TestBannedTermsFinder` at about line 748, `wordingTable`, `evidenceSubjects`, `sectionsWordingTable`, `warningWordingTable`, `noticeWordingTable`, `TestPortugueseOfThisPartEqualsTheOriginal` at about line 339)
- Modify: `internal/catalog/locales/pt-BR.toml`
- Modify: `internal/document/sections_test.go:314` (a comment)

**Interfaces:**
- Consumes: `fileEntries(t, lang) map[Key]string`, `findBannedTerms(lang, text) []string`, `wordSet(words ...string) map[string]bool`, `sameSet`, all in `guard_test.go`.
- Produces: nothing new. The keys of `keys.go` do not change.

- [ ] **Step 1:** Rewrite the Portuguese branch of `bannedTermsFor`. The
  English branch does not change.

```go
	if lang == PortugueseBrazil {
		anyCase = wordSet(
			"solicitação", "solicitações", "limite", "limites",
			"rótulo", "rótulos", "nó", "nós", "réplica", "réplicas",
			"nodepool", "nodepools", "right-sizing", "headroom",
			"unknown", "memory",
			"the", "is", "are", "was", "were", "not", "found", "with", "from",
			"omitted", "cannot", "may", "only", "uses",
		)
		phrases = [][]string{
			{"carga", "de", "trabalho"}, {"cargas", "de", "trabalho"},
			{"pool", "de", "nós"}, {"pools", "de", "nós"},
			{"grupo", "de", "nós"}, {"grupos", "de", "nós"},
			{"regra", "de", "gravação"}, {"regras", "de", "gravação"},
			{"métricas", "brutas"},
			{"escalonamento", "automático"},
			{"limitação", "de", "cpu"},
		}
		return anyCase, phrases, asWritten
	}
```

  Update the comment of `bannedTermsFor`: in Portuguese it bans the
  translations of the technical terms that spec 9.3 writes in English, the
  single word `nodepool`, the two words `headroom` and `right-sizing` that
  have Portuguese common nouns, and English function words that betray a
  pasted English sentence.

- [ ] **Step 2:** Rewrite the cases of `TestBannedTermsFinder` for
  Portuguese. Keep the cases `identifiers are one word each` and
  `kubernetes names stay as they are`. Replace the others with these:

```go
		{
			name: "solicitação in any case",
			lang: PortugueseBrazil,
			text: "Revisar a Solicitação de CPU e o consumo observado.",
			want: []string{"solicitação"},
		},
		{
			name: "request is the English word the rule asks for",
			lang: PortugueseBrazil,
			text: "Revisar o request de CPU e o consumo observado.",
			want: nil,
		},
		{
			name: "limite is banned, limit is not",
			lang: PortugueseBrazil,
			text: "Definir o limite de memória. Definir o limit de memória.",
			want: []string{"limite"},
		},
		{
			name: "the phrase carga de trabalho",
			lang: PortugueseBrazil,
			text: "Recursos por carga de trabalho e por cargas de trabalho.",
			want: []string{"carga de trabalho", "cargas de trabalho"},
		},
		{
			name: "nós in every sense",
			lang: PortugueseBrazil,
			text: "Utilização dos nós.",
			want: []string{"nós"},
		},
		{
			name: "node pool is allowed and nodepool is not",
			lang: PortugueseBrazil,
			text: "Identificação de node pools; nodepool.",
			want: []string{"nodepool"},
		},
		{
			name: "the phrase escalonamento automático",
			lang: PortugueseBrazil,
			text: "Escalonamento automático de nodes.",
			want: []string{"escalonamento automático"},
		},
		{
			name: "cpu as written",
			lang: PortugueseBrazil,
			text: "Uso observado de cpu no node.",
			want: []string{"cpu"},
		},
		{
			name: "the phrase grupo de nós",
			lang: PortugueseBrazil,
			text: "O grupo de nós não pôde ser determinado.",
			want: []string{"grupo de nós", "nós"},
		},
		{
			name: "english leftovers",
			lang: PortugueseBrazil,
			text: "autoscaling state is unknown",
			want: []string{"is", "unknown"},
		},
		{
			name: "memory is a common noun and stays Portuguese",
			lang: PortugueseBrazil,
			text: "Uso de memory acima do limit.",
			want: []string{"memory"},
		},
		{
			name: "headroom keeps its Portuguese noun",
			lang: PortugueseBrazil,
			text: "Margem observada; headroom.",
			want: []string{"headroom"},
		},
```

- [ ] **Step 3:** Run the finder test and the catalog-wide check, and watch
  the second fail.

```bash
go test -count=1 ./internal/catalog/ -run 'TestBannedTermsFinder|TestNoBannedTerm' -v 2>&1 | grep -E '^(--- |    guard_test|ok|FAIL)' | head -80
```

Expected: `TestBannedTermsFinder` passes; `TestNoBannedTerm` fails on
`locales/pt-BR.toml` with exactly 58 entries named, and on no entry of
`locales/en.toml`. Count the lines. A count other than 58 means the list
below or the guard is wrong: stop and report the difference.

- [ ] **Step 4:** Rewrite the 58 entries of `locales/pt-BR.toml`. Every
  other entry of the file stays as it is.

| Key | Portuguese, new |
|---|---|
| `strategy.recording_rule` | Recording rule |
| `strategy.raw_metrics` | Raw metrics |
| `capability.name.workload_usage` | Uso por workload |
| `capability.name.requests_limits` | Requests e limits |
| `capability.name.workloads_without_hpa` | Cobertura de workloads sem HPA |
| `capability.name.ip_node_mapping` | Associação entre endereço e node |
| `capability.name.nodepool_discovery` | Identificação de node pools |
| `capability.name.node_utilization` | Utilização dos nodes |
| `capability.name.node_autoscaling` | Autoscaling de nodes |
| `capability.reason.WORKLOAD_HISTORY_INEXACT` | A associação histórica entre pods e workloads não pôde ser reconstruída com exatidão. |
| `capability.reason.WORKLOAD_ASSOCIATION_UNAVAILABLE` | A associação por workload não está disponível; a cobertura está limitada a pods. |
| `capability.reason.REQUESTS_LIMITS_INCOMPLETE` | A cobertura de requests e limits está incompleta para CPU ou memória. |
| `capability.reason.WORKLOAD_INVENTORY_INCOMPLETE` | O inventário de workloads ou alvos de HPA está incompleto. |
| `capability.reason.NODE_ADDRESS_MAPPING_INCOMPLETE` | A associação entre endereços e nodes pode estar incompleta. |
| `capability.reason.NODEPOOL_LABELS_NOT_FOUND` | Os labels de node pools não foram encontrados. |
| `capability.reason.NODE_AUTOSCALING_UNDETERMINED` | O estado do autoscaling de nodes não pôde ser determinado. |
| `sections.namespaces.header_request` | Request |
| `sections.namespaces.header_limit` | Limit |
| `sections.namespaces.header_peak_request` | % Pico/Request |
| `sections.namespaces.header_peak_limit` | % Pico/Limit |
| `sections.workloads.heading` | Recursos por workload |
| `sections.workloads.empty` | Nenhum dado de workload está disponível para os filtros atuais. |
| `sections.workloads.header_workload` | Workload |
| `sections.workloads.header_cpu_request` | % CPU/Request |
| `sections.workloads.header_cpu_limit` | % CPU/Limit |
| `sections.workloads.header_memory_request` | % Memória/Request |
| `sections.workloads.header_memory_limit` | % Memória/Limit |
| `warning.NAMESPACE_REQUESTS_LIMITS_INCOMPLETE` | As métricas de requests e limits dos namespaces estão incompletas. |
| `warning.NAMESPACE_REQUESTS_LIMITS_UNAVAILABLE` | As métricas de requests e limits dos namespaces estão indisponíveis. |
| `warning.WORKLOAD_USAGE_INCOMPLETE` | As métricas de uso dos workloads estão incompletas. |
| `warning.WORKLOAD_REQUESTS_LIMITS_INCOMPLETE` | As métricas de requests e limits dos workloads estão incompletas. |
| `warning.WORKLOAD_REQUESTS_LIMITS_UNAVAILABLE` | As métricas de requests e limits dos workloads estão indisponíveis. |
| `warning.WORKLOAD_HISTORY_CURRENT_ASSOCIATION` | O histórico usa a associação atual entre pods ativos e workloads. |
| `notice.WORKLOADS_OMITTED_NO_USAGE.singular` | 1 workload deixado de fora por não ter consumo observado no período. |
| `notice.WORKLOADS_OMITTED_NO_USAGE.plural` | {{.Count}} workloads deixados de fora por não terem consumo observado no período. |
| `notice.JOBS_OMITTED.singular` | 1 Job foi deixado de fora do inventário de workloads; o consumo continua contabilizado no total do namespace. |
| `notice.JOBS_OMITTED.plural` | {{.Count}} Jobs foram deixados de fora do inventário de workloads; o consumo continua contabilizado nos totais dos namespaces. |

  The 21 evidence sentences, three for each of seven capabilities, are the
  subject below followed by a space, the ending of the state, and a full
  stop. The endings do not change: `estão disponíveis para o período
  analisado`, `estão disponíveis parcialmente para o período analisado`,
  `não foram localizados para o período analisado`.

| Capability | Subject, new |
|---|---|
| `workload_usage` | Os dados de utilização por workload |
| `requests_limits` | Os dados de requests e limits de recursos |
| `workloads_without_hpa` | Os dados de cobertura dos workloads sem HPA |
| `ip_node_mapping` | Os dados de associação entre endereço e node |
| `nodepool_discovery` | Os dados de identificação dos node pools |
| `node_utilization` | Os dados de utilização dos nodes |
| `node_autoscaling` | Os dados de autoscaling dos nodes |

  The count: 2 strategies, 7 capability names, 7 reasons, 21 evidence
  sentences, 4 headers of the namespace table, 7 entries of the workload
  section, 6 warnings, 4 notices. 58.

- [ ] **Step 5:** Update the expected Portuguese in `guard_test.go` so that
  every table agrees with the file: the `pt` column of `wordingTable` for
  the two strategies, the seven capability names, and the seven reasons; the
  `pt` field of `evidenceSubjects` for the seven capabilities;
  `sectionsWordingTable` for the eleven section entries;
  `warningWordingTable` for the six warnings; `noticeWordingTable` for the
  four notice forms. The English columns do not change.

- [ ] **Step 6:** Rename `TestPortugueseOfThisPartEqualsTheOriginal` to
  `TestPortugueseOfTheSectionsMatchesThePlan` and rewrite its comment:

```go
// TestPortugueseOfTheSectionsMatchesThePlan proves that the Portuguese text
// of the sections "Capacidade por Namespace" and "Recursos por workload",
// and of the warnings and notices they use, equals the wording the owner
// approved, letter for letter. The wording follows the vocabulary of spec
// 9.3, which writes every technical term in English.
```

  Rewrite the comments of `sectionsWordingTable`, `warningWordingTable`, and
  `noticeWordingTable` where they say the text is the original's: it is the
  approved wording.

- [ ] **Step 7:** In `internal/document/sections_test.go:314`, the comment
  `// Recursos por carga de trabalho` becomes `// Recursos por workload`.

- [ ] **Step 8:** Run the tests of the catalog and the document.

```bash
go test -count=1 ./internal/catalog/ ./internal/document/
```

Expected: both pass. `TestNoBannedTerm` names no entry.
`TestDocumentEqualsParityForItsSections` passes through the mapping of
Task 2.

- [ ] **Step 9:** Run the whole suite and read the failures.

```bash
go test -count=1 ./... 2>&1 | grep -E '^(ok|FAIL|---)' | grep -v '^ok'
```

Expected: `internal/goldentest` fails in `TestGoldenDataFiles` for the five
Portuguese data files under `template/tests/capacity/fixtures/command/`,
each message naming `make update-testdata`, and in no other test. Every
other package passes. This is the one failure the plan allows between two
commits; Task 4 regenerates the files. `internal/cli` passes: its test of
the real report reads the headings from the catalog.

- [ ] **Step 10:** Run lint.

```bash
make lint
```

Expected: no issue.

- [ ] **Step 11:** Commit.

```bash
git add internal/catalog/locales/pt-BR.toml internal/catalog/guard_test.go internal/document/sections_test.go
git commit -m "feat(catalog): write technical terms in English"
```

**Tests of this task:**

| Test | Input | Expected |
|---|---|---|
| `TestBannedTermsFinder` | the fourteen cases above | each `want` list |
| `TestNoBannedTerm`, before Step 4 | the catalog as it was | 58 Portuguese entries named, no English entry |
| `TestNoBannedTerm`, after Step 4 | the catalog rewritten | no entry named |
| `TestWordingMatchesThePlan` | both files | every row and every crossed evidence sentence equal |
| `TestPortugueseOfTheSectionsMatchesThePlan` | `pt-BR.toml` | the three tables equal |
| `TestLanguagesUseTheSameParameters`, `TestNoStrayWhiteSpaceInAnEntry`, `TestNoEmDashInsideAnEntry`, `TestEntriesUseThePlainFormOnly` | both files | unchanged: pass |

**Done when:** `TestNoBannedTerm` was seen failing on 58 entries and then
passing, the catalog and document packages pass, lint passes, and the only
failing test of the suite is `TestGoldenDataFiles` on the five Portuguese
data files.

---

### Task 4: The golden files and pages

**Executor:** Main thread. Needs Docker and page review.

**Files:**
- Regenerate: `template/tests/capacity/fixtures/command/*.pt-BR.json` (five files)
- Regenerate: `template/tests/golden/command-complete-pt-1.png`, `-2.png`, `-3.png`, `command-edge-pt-2.png`, `-3.png`, and `template/tests/golden/SHA256SUMS`

- [ ] **Step 1:** Regenerate the data files and read what changed.

```bash
make update-testdata
git status --short
git diff --stat
```

Expected: the five Portuguese data files changed and nothing else. No file
under `testdata/snapshot/` or `testdata/queries/` changed, because the
snapshot holds codes, and no English data file changed. If anything else
changed, stop: the catalog change reached a number.

- [ ] **Step 2:** Read the diff of each of the five files, whole. Every
  changed line is a text of the table of Task 3 and nothing else: no
  number, no status, no identifier, no block order.

```bash
git diff -- template/tests/capacity/fixtures/command/ | grep -E '^[-+]' | grep -v -E '^(\+\+\+|---)' | sort | uniq -c | sort -rn
```

Expected: pairs of removed and added lines that differ only in the words of
the table.

- [ ] **Step 3:** Run the Go suite.

```bash
go test -count=1 ./...
```

Expected: every package passes.

- [ ] **Step 4:** Build the images and regenerate the golden pages.

```bash
make image
make update-golden
git status --short template/tests/golden/
```

Expected: `command-complete-pt-1.png`, `-2.png`, `-3.png`,
`command-edge-pt-2.png`, `-3.png`, and `SHA256SUMS` changed. No English
page and no page of the template fixtures changed; above all,
`capacity-parity-*.png` did not change. If a parity page changed, stop: the
template layer was touched.

- [ ] **Step 5:** Open each of the five changed pages and read every table
  header and every note. Each new word is a word of the table of Task 3;
  no header wraps where it did not wrap before; no row overflows. Write one
  line per page to `.cache/phase-2/part-2b/pages.md`, naming what changed on
  it.

- [ ] **Step 6:** Run the golden gate twice in a row.

```bash
make test-golden
make test-golden
```

Expected: both pass; the log names `command-complete-pt` and
`command-edge-pt`.

- [ ] **Step 7:** Write the fourteen PDFs for the owner's review, one for
  each scenario and language, into `.cache/phase-2/part-2b/pdf/`, the way
  part 2.2 did: `cmd/fake-prometheus` serving each scenario file with
  `--fixture`, and `bin/k8s-capacity-report report` collecting from it with
  the configuration of that scenario, once with `--lang pt-BR` and once
  with `--lang en`. Read the Portuguese ones.

- [ ] **Step 8:** Commit.

```bash
git add template/tests/capacity/fixtures/command/ template/tests/golden/
git commit -m "test: regenerate the Portuguese golden files"
```

**Done when:** the suite passes, `make test-golden` passed twice, every
changed page was read and its change is a word of the table, and the parity
pages did not change.

---

### Task 5: Close the part

**Executor:** Main thread.

- [ ] **Step 1:** Write `.cache/phase-2/part-2b/portuguese-wording.md` for
  the owner: the 58 entries with the old Portuguese, the new Portuguese,
  and the English beside them; the rulings W1 to W9; and the Portuguese
  PDFs to open. The Portuguese is under review from now on, as the English
  is. Apply what the owner changes, run `make update-testdata` and
  `make update-golden`, and review what changed.
- [ ] **Step 2:** Write the diff of the branch to
  `.cache/phase-2/part-2b/branch.diff`. Run one adversarial reviewer over
  it, with the rule and the vocabulary in its brief. It writes its findings
  to `.cache/phase-2/part-2b/adversarial/` and repeats them in its last
  message.
- [ ] **Step 3:** Record every finding in `ROADMAP.md`. Fix the ones
  assigned to this part, each with a brief that names the class. Run the
  review again over the fixes.
- [ ] **Step 4:** Run `make gate`. Keep the logs of `.cache/gate/` in
  `.cache/phase-2/part-2b/gate/`. Read `gate.out` back and check that the
  golden log names `command-complete-pt`.
- [ ] **Step 5:** Update `ROADMAP.md`: every item `DONE` with its commit;
  F-74 `DONE` with its commits; the summary line of 2.2b; the rulings W1 to
  W9 with what the owner said of each. Read `ROADMAP.md` and `CHANGELOG.md`
  back.
- [ ] **Step 6:** Write the closing report to
  `.cache/phase-2/part-2b/closing-report.md` and ask the owner for the
  merge. The recommendation is to merge with no new version and no tag: the
  snapshot of schema `1.0` is not complete before part 2.4, and nothing was
  released with the old Portuguese.
- [ ] **Step 7:** After the owner approves, merge by fast-forward.

```bash
git switch main
git merge --ff-only feat/wording-rule
```

**Done when:** `make gate` passed on the last commit, the review has no open
finding assigned to this part, the owner has the Portuguese wording, and the
owner approved the merge.

---

## Self-review of this plan

| Requirement | Task |
|---|---|
| Spec 3, a deliberate difference is recorded with its reason | 1 |
| Spec 9.3, the vocabulary | 1 (the text), 3 (the guard and the entries) |
| Spec 9.5, a changed sentence is a visible change | 1 (the changelog entry, W6); no version, nothing released |
| Spec 13.2, the catalog guards | 3 |
| Spec 10.3, the pages keep the tables and columns of the original | 4 (the pages are read; only words change) |
| AGENTS.md, the numbers equal the original exactly | 2 (the comparison reads no catalog) |
| AGENTS.md, the parity pages equal the original byte for byte | 4 (checked in Step 4), W3 |
| AGENTS.md, a test never writes a golden file | 4 (the main thread runs the two commands) |
| Review focus 1, 2 | 3 (`english leftovers`, `nós in every sense`) |
| Review focus 3 | 2 (`TestParitySubstitutionChangesTheFixture`) |
| Review focus 4 | 2 (`TestOriginalNoticeTextWritesTheCount`) |
| Review focus 5 | 2 (`TestOriginalSentencesCoverEveryCode`) |

Placeholders: none. Every step names its file, its text, or its command.

Types: `originalNotice`, `originalWarnings`, `originalNotices`,
`originalWarningText`, `originalNoticeText`, `originalTexts` appear in Task 2
only; `parityTexts`, `parityLabelText`, `withCatalogTexts` appear in Task 2
and in no other task; `bannedTermsFor`, `findBannedTerms`, `wordSet`,
`fileEntries` are the existing names of `guard_test.go`. The key constants
named in Task 2 exist in `keys.go` as written.

Order: Task 2 runs before Task 3 so that no comparison with the original
fails at any commit. The one failure between commits, `TestGoldenDataFiles`
between Tasks 3 and 4, is stated in Task 3 Step 9.
