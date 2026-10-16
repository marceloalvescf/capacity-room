# Phase 14: node autoscaling from kube-state-metrics — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** A report on an AKS or OKE cluster says whether node autoscaling is enabled or disabled, read from kube-state-metrics alone, and every PDF carries its report's emission time as its creation date; released as 2.4.0.

**Architecture:** Five implementation tasks, in sequence, then the main thread's closing work. Task 1 gives every PDF compile a creation time (F-269). Task 2 lets a recorded scenario carry answers this project adds, in `testdata/prometheus-added/`, without editing the recorded files. Task 3 replaces the `cluster_autoscaler_*` probe and collector with kube-state-metrics, with its answers, its parity record, and the regenerated golden data. Task 4 shows only the numbers that are known, on the PDF's verdict and on the web, with the new wording. Task 5 writes the documents. Tasks 6 and 7 are the main thread's: golden pages, e2e, the AKS check, the reviews, the version, and the gate.

**Tech Stack:** Go 1.27, typst 0.15.1, React and TypeScript (Vitest), PromQL over kube-state-metrics, GNU make, all inside `nix develop`.

**Spec:** `docs/superpowers/specs/2026-10-16-node-autoscaling-design.md` (c751feb, amended 1c70089 and af04888, approved 2026-10-16). Read it before any task; this plan argues from it.

## Global Constraints

- The status ConfigMap is `kube-system/cluster-autoscaler-status`, fixed; no setting names it.
- The five collector queries are instant queries at `in.Window.End`, in this order and with exactly these texts, where `<scope>` is the scope's matchers merged in (with the cluster matcher on: `cluster="prod"`, matchers sorted by label name) and `W` is `max(1, end-start)` in whole seconds:
  1. `max_over_time(kube_configmap_info{<scope>,configmap="cluster-autoscaler-status",namespace="kube-system"}[Ws])`
  2. `changes(kube_configmap_metadata_resource_version{<scope>,configmap="cluster-autoscaler-status",namespace="kube-system"}[Ws])`
  3. `count(count by (node) (kube_node_info{<scope>}))`
  4. `count(count by (node) (max_over_time(kube_node_created{<scope>}[Ws]) > time() - W))`
  5. `count(count by (namespace, pod) (kube_pod_status_unschedulable{<scope>} == 1))`
- Amended after the whole-branch review (2026-10-16): queries 1 and 2 drop `namespace="kube-system"` and look back `max(W, 300)` seconds (spec section 3).
- The probe, the ninth and last of the preflight, is `count(kube_configmap_info{<scope>})` at the end of the window; the preflight still sends 29 requests.
- The snapshot stays at schema `2.0`; its reader accepts one new combination: status `disabled`, source `raw_metrics`, confidence `inferred`.
- `NODE_AUTOSCALING_NEAR_MAX`, `NODE_AUTOSCALING_AT_MIN`, and `NODE_AUTOSCALING_BOUNDS_UNKNOWN` stay registered, with their catalog entries, and are never produced.
- Wording, exactly (Task 4 enters it):

  | Key | en | pt-BR |
  |---|---|---|
  | `finding.autoscaling_disabled.summary` | Node autoscaling is disabled. | O autoscaling de nodes está desabilitado. |
  | `finding.autoscaling_enabled.summary` | Node autoscaling is enabled. | O autoscaling de nodes está habilitado. |
  | `finding.autoscaling_enabled.action` | Keep following the nodes added and the unschedulable pods. | Manter acompanhamento dos nodes adicionados e dos pods não agendáveis. |
  | `finding.autoscaling_unknown.action` | Check that kube-state-metrics collects ConfigMaps, or state the node autoscaling in the configuration. | Verificar se o kube-state-metrics coleta ConfigMaps, ou informar o autoscaling de nodes na configuração. |
  | `verdict.growth.scale_events` | Nodes added | Nodes adicionados |
  | `verdict.growth.state.enabled` (new) | Enabled | Ativo |
  | `verdict.growth.current_nodes` (new) | Current nodes | Nodes atuais |
  | web `report.autoscaling.scaleEvents` | Nodes added | Nodes adicionados |

- The files of `testdata/prometheus/`, `testdata/prometheus-own/`, and `testdata/python/` are never edited.
- A number in a test is a literal, never the expression of the code written again; a test of a comparison has no tolerance.
- No test skip (`Skip`, `Skipf`, `SkipNow`; Vitest `skip`, `only`, `todo`).
- Every subagent: never wait on standard input, never write to `/tmp` (scratch goes under `.cache/`), leave no background process, run `go` and `make` only inside `nix develop -c ...`, run `npm` only inside `web/`, never run Docker, never run `git stash`, `git restore`, `git checkout` of a path, or `git clean`; every commit names its paths (`git add <paths>`), never `git add -A` or `git commit -a`.
- Comments say what the code does; no comment names a task, a phase, a step, a ruling, or a path under `.cache/`. A comment may name a section of the design and the original program.
- Reader text lives in the catalog and the locale files; no Go string literal under `internal/` or `cmd/` holds a letter outside ASCII (test files excepted).
- Commit subjects: `type(scope): description`, imperative, under 50 characters.
- No name, address, or context of a work cluster enters a tracked file.

## Review Focus

- **Two replicas of kube-state-metrics.** Each exports every series, so an instant answer holds one sample per replica. A reader expects the status and the counts to be what one replica says. Task 3 answers query 2 with two samples, `0` and `57`, and expects enabled; the `count by` of queries 3 to 5 is pinned by their texts.
- **A ConfigMap left behind.** An autoscaler turned off can leave its ConfigMap, never written again. A reader expects disabled, never enabled. Task 3 answers query 1 with `1` and query 2 with `0` and expects disabled; `edge-scaling` carries it into the golden data.
- **kube-state-metrics answers the nodes but holds no created node and no unschedulable pod.** A reader expects zeros, not unknown counts; and with no answer for the nodes, all three counts unknown while the status still comes from the ConfigMap. Task 3 pins both.
- **A window of zero length.** A reader expects `[1s]` and `time() - 1`, never `[0s]`. Task 3 pins it.
- **A report stored by 2.3.0.** It holds `NODE_AUTOSCALING_NEAR_MAX`, a minimum, a maximum, and the gauge. A reader expects it to load, to view, and to render the pair with its gauge after the upgrade. Task 3 keeps a copy of today's `direct-complete` snapshot before regenerating; Task 3 decodes it and Task 4 builds its verdict and its view.

---

### Task 1: every PDF carries a creation time (F-269)

> **Superseded during execution (2026-10-16).** A report PDF holds no date (the template sets `date: none`), so the creation-time plumbing below would change no byte of a report. It was implemented (33a1fc8) and reverted; the task became: `TestRealCompileIsDeterministic` compiles a document that sets `date: none`, and a test tagged `template` compiles a real report data file twice 1.1 s apart and finds equal bytes and no date. Spec section 9 says so.

**Files:**
- Modify: `internal/typst/compile.go` (the `Job` struct at :24-34, `Args`)
- Modify: `internal/typst/compile_test.go` (`validJob`, `TestArgsOrderForPDF`, `TestCompilePassesArguments`, the PDF jobs at :27, :37, :140, :155)
- Modify: `internal/typst/compile_real_test.go` (every PDF job; `TestRealCompileIsDeterministic` at :173)
- Modify: `internal/render/render.go:44-61` (`Compile`)
- Modify: `internal/render/compile_test.go` (:34, :49, :61, :89)
- Modify: `internal/cli/run.go` (`compilePDF` at :788 and its callers at :632 and :715)
- Modify: `internal/platform/worker/render.go:~197-205` (`compile`)
- Modify: `internal/cli/helpers_test.go:37` (`fakeTypstPath`), and the callers of `render.Compile` in `internal/cli/api_end_to_end_test.go:290`, `internal/platform/worker/render_test.go:260`, `internal/platform/worker/end_to_end_test.go:184`
- Modify: `template/capacity_test.go:586-592`, `template/package_test.go:83-89`, `template/harness_test.go:126-132` (PDF jobs in tests tagged `template`)

**Interfaces:**
- Produces: `typst.Job.Created time.Time` — required for `PDF`, refused for `PNG`; `Args` writes `--creation-timestamp <Created.Unix()>` right after `--format pdf`.
- Produces: `render.Compile(ctx context.Context, tc typst.Toolchain, tempDir string, data []byte, created time.Time) ([]byte, error)`.

- [ ] **Step 1: Write the failing tests of `Args`**

In `internal/typst/compile_test.go`, give `validJob` a creation time when its format is PDF, `Created: time.Date(2026, 7, 15, 17, 30, 0, 0, time.UTC)`, and add:

```go
func TestArgsRefusesAPDFWithoutACreationTime(t *testing.T) {
	job := validJob(t)
	job.Created = time.Time{}
	if _, err := Args(job); err == nil || err.Error() != "a PDF needs a creation time" {
		t.Fatalf("Args() error = %v, want %q", err, "a PDF needs a creation time")
	}
}

func TestArgsRefusesACreationTimeForPNG(t *testing.T) {
	job := validJob(t)
	job.Format = PNG
	job.Output = filepath.Join(filepath.Dir(job.Output), "page-{p}.png")
	if _, err := Args(job); err == nil || err.Error() != "a creation time applies only to PDF" {
		t.Fatalf("Args() error = %v, want %q", err, "a creation time applies only to PDF")
	}
}
```

In `TestArgsOrderForPDF`, insert `"--creation-timestamp", "1784136600"` right after `"--format", "pdf"` in the expected list (1784136600 is 2026-07-15 17:30:00 UTC). In `TestCompilePassesArguments`, the recorded arguments already come from `validJob`, so they now hold the flag; add an explicit check that the recorded list contains `--creation-timestamp` followed by `1784136600`.

- [ ] **Step 2: Run them to see them fail**

Run: `nix develop -c go test ./internal/typst/ -run 'TestArgs|TestCompilePassesArguments' -v`
Expected: FAIL: `unknown field Created` at compile time.

- [ ] **Step 3: Implement**

In `internal/typst/compile.go`, add the field and the checks beside the Pages and PPI checks, and the flag right after `--format`:

```go
	PPI         int               // optional, PNG only
	Created     time.Time         // required for PDF, refused for PNG: the document's creation date
}
```

```go
	if job.Format == PDF && job.Created.IsZero() {
		return nil, errors.New("a PDF needs a creation time")
	}
	if job.Format == PNG && !job.Created.IsZero() {
		return nil, errors.New("a creation time applies only to PDF")
	}
```

```go
	args = append(args, "--format", string(job.Format))
	if job.Format == PDF {
		args = append(args, "--creation-timestamp", strconv.FormatInt(job.Created.Unix(), 10))
	}
```

The field comment says why it exists: typst otherwise takes `SOURCE_DATE_EPOCH` or the current second, and the toolchain runs typst with an empty environment, so two compiles a second apart differ.

- [ ] **Step 4: Run the package**

Run: `nix develop -c go test ./internal/typst/`
Expected: PASS.

- [ ] **Step 5: `render.Compile` takes the creation time; its test records the flag**

Change the signature to `Compile(ctx context.Context, tc typst.Toolchain, tempDir string, data []byte, created time.Time)` and set `Created: created` in its job. Its doc comment adds: "created is the PDF's creation date; a report passes the emission time of its snapshot, so the same snapshot gives the same bytes."

In `internal/render/compile_test.go`, make the inline script fake at :49 also write its arguments, one per line, to a file beside the output, and add a test that compiles with `time.Date(2026, 7, 15, 17, 30, 0, 0, time.UTC)` and finds `--creation-timestamp` followed by `1784136600` in that file. Pass a fixed time at :34, :61, :89. Run it red first (before the signature change it does not compile), then green:

Run: `nix develop -c go test ./internal/render/`
Expected: PASS.

- [ ] **Step 6: The callers pass the emission time of the snapshot**

- `internal/cli/run.go`: `compilePDF` takes `created time.Time` and passes it to `render.Compile`. At :632, parse `res.Snapshot.Report.EmittedAt` with `snapshot.Time` (an error there is a fault of the snapshot the run just built: return it); at :715 pass the `emittedAt` already parsed at :680 (move its declaration out of an inner block if it sits in one).
- `internal/platform/worker/render.go`: after `snapshot.Decode(body)`, parse `snap.Report.EmittedAt` with `snapshot.Time` and pass it.
- `internal/cli/helpers_test.go:37` `fakeTypstPath`: after the fake PDF content, append a line `creation-timestamp <value>` holding the value that followed `--creation-timestamp` in its arguments, or `creation-timestamp none`. Then add one assertion to a cli test of `--pdf` and one to a worker render test: the PDF bytes hold `creation-timestamp <unix of the snapshot's emitted_at>` (write the number as a literal: read the snapshot the test uses and take its `emitted_at`). A test that compared the fake PDF byte for byte follows the new line.
- The other test callers of `render.Compile` pass the emission time of the snapshot they hold (`want.Snapshot.Report.EmittedAt`, `snap.Report.EmittedAt`), parsed with `snapshot.Time`.

Run: `nix develop -c go test ./internal/cli/ ./internal/platform/worker/ ./internal/render/`
Expected: PASS (the worker's PostgreSQL tests are tagged and run on the main thread).

- [ ] **Step 7: The real compile is deterministic across a second**

In `internal/typst/compile_real_test.go` (tag `template`), give every PDF job `Created: time.Date(2026, 7, 15, 17, 30, 0, 0, time.UTC)`. In `TestRealCompileIsDeterministic`, sleep `1100 * time.Millisecond` between the two compiles, keep the byte comparison, and add: the first PDF contains `CreationDate(D:20260715173000Z)`. In the `template` package tests, give the three PDF jobs the same fixed time.

Run: `nix develop -c go test -count=1 -tags=template ./internal/typst/ ./template/ -run 'Real|Compile|Package|Harness'`
Expected: PASS. Then remove the `--creation-timestamp` lines from `Args` for a moment and run `TestRealCompileIsDeterministic` again: it must FAIL (two compiles 1.1 s apart differ). Put the lines back.

- [ ] **Step 8: Whole suite and lint, then commit**

Run: `nix develop -c make test && nix develop -c make lint`
Expected: PASS.

```bash
git add internal/typst/ internal/render/ internal/cli/run.go internal/cli/helpers_test.go internal/cli/api_end_to_end_test.go internal/platform/worker/render.go internal/platform/worker/render_test.go internal/platform/worker/end_to_end_test.go template/capacity_test.go template/package_test.go template/harness_test.go
git commit -m "fix(pdf): date a PDF by its report's emission"
```

(Add any other file the steps touched, by name.)

---

### Task 2: answers this project adds to a recorded scenario

**Files:**
- Create: `testdata/prometheus-added/README.md`
- Create: `testdata/prometheus-added/{direct-complete,grafana-authenticated,thanos-multicluster,raw-fallback,partial-data,value-edge-cases,edge-values,edge-scaling}.json`, each `{"requests": []}` followed by a newline
- Modify: `internal/promtest/fixture.go` (add `Merge`), `internal/promtest/scenarios.go` (add `AddedPath`, `LoadScenario`)
- Test: `internal/promtest/fixture_test.go`, `internal/promtest/scenarios_test.go`
- Modify: every caller that loads a scenario through `promtest.Path` and `promtest.Load`: `internal/goldentest/goldentest.go:114-123`, `internal/cli/helpers_test.go:152-160`, `internal/cli/run_test.go:1677-1685, 1733-1758, 1875-1883`, `internal/cli/worker_wiring_test.go:55-63`, `internal/discovery/scope_test.go:416-424, 465-473`, `internal/discovery/capabilities_test.go:994-1002, 1178-1186`, `internal/discovery/preflight_test.go:35-43`, `internal/pipeline/pipeline_test.go:104-112, 153-161`, `internal/platform/worker/collect_test.go:344-348`, `internal/platform/worker/render_test.go:353`, `internal/platform/worker/end_to_end_test.go:140`, `internal/platform/profiles/service_test.go:660-668`, `internal/platform/api/profiles_test.go:32-40`, `internal/platform/netpolicy/transport_test.go:378-386`, `internal/prom/scenario_test.go:19-27, 66-74`
- Modify: `cmd/fake-prometheus/main.go` (flag `--added`), its test
- Modify: `deploy/compose.integration.yaml` (mount and flag for `fake-prometheus` and `fake-recovery`)

**Interfaces:**
- Produces: `promtest.Merge(fixtures ...Fixture) Fixture` — the requests of each fixture in turn.
- Produces: `promtest.AddedPath(name string) (string, error)` — `testdata/prometheus-added/<fixture stem>.json` for a scenario of `Scenarios` or `RangeScenarios` (the range scenario takes its `Fixture`'s file).
- Produces: `promtest.LoadScenario(name string) (Fixture, error)` — the added answers, then the recorded answers, as one fixture; it fails when either file is missing.
- Produces: `fake-prometheus --added <path>` — optional; when given, its answers come first.

- [ ] **Step 1: Write the failing tests**

In `internal/promtest/fixture_test.go`:

```go
func TestMergeKeepsTheOrderOfEachFixture(t *testing.T) {
	a := Fixture{Requests: []Request{{Path: "/a1"}, {Path: "/a2"}}}
	b := Fixture{Requests: []Request{{Path: "/b1"}}}
	got := Merge(a, b)
	var paths []string
	for _, r := range got.Requests {
		paths = append(paths, r.Path)
	}
	if want := []string{"/a1", "/a2", "/b1"}; !slices.Equal(paths, want) {
		t.Fatalf("paths = %q, want %q", paths, want)
	}
}
```

In `internal/promtest/scenarios_test.go`:

```go
func TestEveryScenarioHasItsAddedAnswers(t *testing.T) {
	for _, s := range slices.Concat(Scenarios, RangeScenarios) {
		if _, err := LoadScenario(s.Name); err != nil {
			t.Errorf("LoadScenario(%q): %v", s.Name, err)
		}
	}
}

func TestAddedAnswersComeFirst(t *testing.T) {
	recorded, err := Load(mustPath(t, "direct-complete"))
	if err != nil {
		t.Fatal(err)
	}
	added, err := Load(mustAddedPath(t, "direct-complete"))
	if err != nil {
		t.Fatal(err)
	}
	got, err := LoadScenario("direct-complete")
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Requests) != len(added.Requests)+len(recorded.Requests) {
		t.Fatalf("requests = %d, want %d", len(got.Requests), len(added.Requests)+len(recorded.Requests))
	}
	if !reflect.DeepEqual(got.Requests[len(added.Requests):], recorded.Requests) {
		t.Fatal("the recorded requests do not follow the added ones unchanged")
	}
}

func TestAddedAnswersMatchTheirChecksums(t *testing.T) {
	// Modelled on TestOwnScenariosMatchTheirChecksums: the README's SHA-256
	// block names exactly the eight files of the folder, and each digest matches.
}
```

Write `TestAddedAnswersMatchTheirChecksums` in full after `TestOwnScenariosMatchTheirChecksums` (scenarios_test.go:~128), with these differences: the folder is `testdata/prometheus-added`, it requires exactly 8 names, and it also fails on a `*.json` file of the folder that the README does not name. `mustPath` and `mustAddedPath` are small helpers that fail the test on an error.

- [ ] **Step 2: Run them to see them fail**

Run: `nix develop -c go test ./internal/promtest/ -v -run 'Merge|Added'`
Expected: FAIL: `undefined: Merge`, `undefined: LoadScenario`.

- [ ] **Step 3: Create the folder**

Write the eight files, each exactly `{"requests": []}` and a newline. Write `testdata/prometheus-added/README.md`:

```markdown
# Answers added to the recorded scenarios

Each file holds answers to queries that the original program never sent, for
the scenario of the same name in `testdata/prometheus/` or
`testdata/prometheus-own/`, whose files are never edited.
`promtest.LoadScenario` reads a scenario as the answers of this folder
followed by the recorded answers, so a recorded answer never takes a query
meant for an added one. The format is the format of the recorded files.

## What each file answers

(Filled in with the answers.)

## SHA-256

<sha256>  direct-complete.json
...
```

Compute the eight digests with `sha256sum` and write them in the block, sorted by file name. The section "What each file answers" says, for now, "No answer yet." under the heading; Task 3 fills it.

- [ ] **Step 4: Implement `Merge`, `AddedPath`, `LoadScenario`**

```go
// Merge returns one fixture holding the requests of each fixture in turn.
func Merge(fixtures ...Fixture) Fixture {
	var merged Fixture
	for _, f := range fixtures {
		merged.Requests = append(merged.Requests, f.Requests...)
	}
	return merged
}
```

```go
// AddedPath returns the file of the answers this project adds to the
// scenario of name, in testdata/prometheus-added.
func AddedPath(name string) (string, error) {
	root, err := moduleRoot()
	if err != nil {
		return "", err
	}
	for _, s := range slices.Concat(Scenarios, RangeScenarios) {
		if s.Name == name {
			return filepath.Join(root, "testdata", "prometheus-added", s.fixture()+".json"), nil
		}
	}
	return "", fmt.Errorf("no scenario named %q", name)
}

// LoadScenario reads the scenario of name: the answers this project adds,
// then the recorded answers, as one fixture.
func LoadScenario(name string) (Fixture, error) {
	added, err := AddedPath(name)
	if err != nil {
		return Fixture{}, err
	}
	recorded, err := Path(name)
	if err != nil {
		return Fixture{}, err
	}
	a, err := Load(added)
	if err != nil {
		return Fixture{}, err
	}
	r, err := Load(recorded)
	if err != nil {
		return Fixture{}, err
	}
	return Merge(a, r), nil
}
```

Run: `nix develop -c go test ./internal/promtest/`
Expected: PASS.

- [ ] **Step 5: Every caller loads the scenario**

Replace each `promtest.Path(name)` followed by `promtest.Load(path)` in the files listed above with `promtest.LoadScenario(name)`. Where a caller used the path for something else, keep `Path` for that use alone. If `Path` is then used only inside the package, unexport it. `internal/discovery/capabilities_test.go` reads `fixture.Requests` directly to replay the matching rule (:1058, :1088): it reads the merged fixture, so its replay sees the added answers first, as the server does.

Run: `nix develop -c make test`
Expected: PASS, with every count unchanged (the added files are empty).

- [ ] **Step 6: The fake takes the added answers**

In `cmd/fake-prometheus/main.go`, add `--added` (string, optional, "a fixture whose answers come before those of --fixture"). When it is set, load it and serve `promtest.Merge(added, fixture)`. Add a test beside the existing ones of the command: with `--added` naming a file of one answer and `--fixture` a file of one answer for the same query text, the first request gets the added answer.

In `deploy/compose.integration.yaml`, mount `../testdata/prometheus-added:/fixtures-added:ro` in `fake-prometheus` and `fake-recovery`, and add `--added /fixtures-added/direct-complete.json` to both commands, beside their `--fixture /fixtures/direct-complete.json`.

Run: `nix develop -c go test ./cmd/fake-prometheus/ && nix develop -c make lint`
Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add testdata/prometheus-added/ internal/promtest/ cmd/fake-prometheus/ deploy/compose.integration.yaml <every caller file changed in Step 5>
git commit -m "test(prom): load added answers before recorded"
```

---

### Task 3: the probe and the collector read kube-state-metrics

**Files:**
- Create: `internal/snapshot/testdata/stored-2.3.0-direct-complete.json` (a copy, before any regeneration)
- Modify: `internal/discovery/capabilities.go:88-111` (`hasAutoscaler` becomes `hasConfigMaps`), `:355-370` (`nodeAutoscalingCapability`), `:168` (its call)
- Modify: `internal/collect/autoscaling.go` (whole file), `internal/collect/autoscaling_test.go`
- Modify: `internal/snapshot/codec.go:1404-1427` (`autoscalingShapes` and its comment), `internal/snapshot/codec_test.go`
- Modify: `internal/analysis/autoscaling.go:27-71`, its test
- Create answers: `testdata/prometheus-added/*.json` (all eight), `testdata/prometheus-added/README.md`
- Modify: `docs/decisions/0002-differences-from-the-original.md` (a new section)
- Modify: `internal/goldentest/original_test.go` (`TestQueriesEqualTheOriginal` :1732, `compareAutoscaling` :1516, `compareCollectionStates` :1549, `withoutRuledCodes` ~:2553, `TestComparisonCanFail` sub-test at :2148)
- Modify tests that name the old queries or their counts: `internal/discovery/capabilities_test.go:122, 587-630`, `internal/goldentest/detect_test.go:53, :205`, `internal/pipeline/pipeline_test.go:141-171`, `internal/platform/worker/collect_test.go:386-393`, `internal/cli/run_test.go:1498` and `:1898-1917`, `internal/collect/collect_test.go:139`, `internal/goldentest/edge_values_test.go:44-107`, `internal/promtest/fixture_test.go` (`TestLoadReadsEachScenario`), `Makefile:210`
- Regenerate: `testdata/snapshot/`, `testdata/queries/`, `template/tests/capacity/fixtures/command/`, `web/tests/fixtures/view/` (by `make update-testdata`)

**Interfaces:**
- Consumes: `promtest.LoadScenario` (Task 2).
- Produces: capability `node_autoscaling` with evidence `[]string{"kube_configmap_info"}` when available from raw metrics.
- Produces: `collect.Autoscaling` unchanged in signature; `AutoscalingEvidence` unchanged in shape.

- [ ] **Step 1: Keep a report stored by 2.3.0**

Copy `testdata/snapshot/direct-complete.json` byte for byte to `internal/snapshot/testdata/stored-2.3.0-direct-complete.json` before anything else, and check that it holds `"code": "NODE_AUTOSCALING_NEAR_MAX"`, `"min_nodes": 1`, and `"max_nodes": 3`. Add to `internal/snapshot/codec_test.go`:

```go
func TestDecodesAReportStoredBy230(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("testdata", "stored-2.3.0-direct-complete.json"))
	if err != nil {
		t.Fatal(err)
	}
	s, err := Decode(raw)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	a := s.Summary.Autoscaling
	if a.Status != "enabled" || a.MaxNodes == nil || *a.MaxNodes != 3 || a.MinNodes == nil || *a.MinNodes != 1 {
		t.Fatalf("autoscaling = %+v, want enabled with min 1 and max 3", a)
	}
	if !slices.ContainsFunc(s.Findings, func(f Finding) bool { return f.Code == "NODE_AUTOSCALING_NEAR_MAX" }) {
		t.Fatal("no NODE_AUTOSCALING_NEAR_MAX finding")
	}
}
```

Run: `nix develop -c go test ./internal/snapshot/ -run TestDecodesAReportStoredBy230 -v`
Expected: PASS (it guards what follows).

- [ ] **Step 2: The probe asks whether ConfigMaps are collected (failing tests first)**

In `internal/discovery/capabilities_test.go`, change the probe's answer key at :122 to `count(kube_configmap_info{cluster="prod"})` (match the matcher state of that test: without the matcher it is `count(kube_configmap_info)`), the table at :595 to evidence `[]string{"kube_configmap_info"}`, the answer at :602, and the check at :625-630 to look for `kube_configmap_info` ("want the probe asked even when configured"). In `internal/goldentest/detect_test.go` (`detectQueries` :53 and :205), change the ninth probe the same way. Run them red:

Run: `nix develop -c go test ./internal/discovery/ ./internal/goldentest/ -run 'Capabilit|Detect' `
Expected: FAIL on the probe text.

Then implement in `internal/discovery/capabilities.go`:

```go
// hasConfigMaps runs the node_autoscaling probe: whether kube-state-metrics
// exports any ConfigMap, the ground the collector reads the status ConfigMap
// of the cluster autoscaler on.
func (p *prober) hasConfigMaps() bool {
	if p.err != nil {
		return false
	}
	query := "count(" + prom.Selector{Metric: "kube_configmap_info", Equal: p.scope.Matchers()}.String() + ")"
	samples, err := p.q.Query(p.ctx, query, p.at)
	if err != nil {
		p.err = err
		return false
	}
	for _, sample := range samples {
		if sample.Valid && sample.Value > 0 {
			return true
		}
	}
	return false
}
```

and in `nodeAutoscalingCapability(configured string, configMaps bool)` the available branch reads `Evidence: []string{"kube_configmap_info"}`; its comment says the capability is available when ConfigMaps are collected, the ground of the status. Rename the call at :168. Check `p.scope.Matchers()` exists on the prober's scope type (it is `discovery.Scope`); if the prober holds the matcher fields only, build the map as `collect` does.

Run: `nix develop -c go test ./internal/discovery/`
Expected: PASS.

- [ ] **Step 3: The collector's tests (failing first)**

Rewrite `internal/collect/autoscaling_test.go`. The querier answers instant queries; extend `scriptedQuerier` use with its `queryFunc`. Keep `TestAutoscalingConfiguredSendsNothing`. Write:

```go
const (
	queryInfo          = `max_over_time(kube_configmap_info{cluster="prod",configmap="cluster-autoscaler-status",namespace="kube-system"}[86400s])`
	queryWrites        = `changes(kube_configmap_metadata_resource_version{cluster="prod",configmap="cluster-autoscaler-status",namespace="kube-system"}[86400s])`
	queryNodes         = `count(count by (node) (kube_node_info{cluster="prod"}))`
	queryAdded         = `count(count by (node) (max_over_time(kube_node_created{cluster="prod"}[86400s]) > time() - 86400))`
	queryUnschedulable = `count(count by (namespace, pod) (kube_pod_status_unschedulable{cluster="prod"} == 1))`
)

// autoscalingQuerier answers each instant query with the samples its text
// maps to, and nothing else.
func autoscalingQuerier(answers map[string][]prom.Sample) *scriptedQuerier {
	return &scriptedQuerier{queryFunc: func(query string) ([]prom.Sample, error) {
		return answers[query], nil
	}}
}

func samples(values ...float64) []prom.Sample {
	out := make([]prom.Sample, len(values))
	for i, v := range values {
		out[i] = prom.Sample{Value: v, Valid: true}
	}
	return out
}
```

`autoscalingInput(mode)` builds its capabilities with a `node_autoscaling` capability available from raw metrics; add `autoscalingInputWithout()` whose `node_autoscaling` capability is partial with strategy none. Check the fields of `prom.Sample` (`Valid`, `Value`, and any time field) and fill them as the type needs.

Tests, each with literal expectations:

1. `TestAutoscalingQueriesInOrder`: all five, in order, equal to the five constants; every query instant (none through `QueryRange`).
2. `TestAutoscalingWindowAtLeastOneSecond`: with `Start == End`, query 2 ends `[1s])` and query 4 is `count(count by (node) (max_over_time(kube_node_created{cluster="prod"}[1s]) > time() - 1))`.
3. `TestAutoscalingWithoutConfigMapsSendsNothing`: with the partial capability, status `unknown`, source `none`, confidence `unknown`, every count unknown, no query sent.
4. `TestAutoscalingEnabledWhenWritten`: info `1`, writes `118`, nodes `3`, added `1`, unschedulable `2` → enabled, raw_metrics, inferred, current 3, scale events 1, unschedulable 2, managed, min, max unknown.
5. `TestAutoscalingReadsTheLargestReplica`: writes `samples(0, 57)`, info `samples(1, 1)`, nodes `3` → enabled.
6. `TestAutoscalingDisabledWhenLeftBehind`: info `1`, writes `0`, nodes `4` → disabled, raw_metrics, inferred, current 4, added 0, unschedulable 0.
7. `TestAutoscalingDisabledWithoutTheConfigMap`: info empty, writes empty, nodes `1` → disabled, current 1, added 0, unschedulable 0.
8. `TestAutoscalingEnabledByPresenceWithoutVersions`: info `1`, writes empty, nodes `1` → enabled.
9. `TestAutoscalingUnknownCountsWithoutNodes`: info `1`, writes `5`, nodes empty, added `2`, unschedulable `1` → enabled, and current, added, and unschedulable all unknown.
10. `TestAutoscalingIgnoresInvalidSamples`: writes `[]prom.Sample{{Value: 3, Valid: false}}`, info empty, nodes `1` → disabled.

Run: `nix develop -c go test ./internal/collect/ -run Autoscaling -v`
Expected: FAIL (the collector still sends the range queries).

- [ ] **Step 4: Implement the collector**

Replace `internal/collect/autoscaling.go` below the status constants and `AutoscalingEvidence` (keep both, and the struct's fields, unchanged) with:

```go
// statusConfigMap names the ConfigMap the cluster autoscaler writes its
// status to by default, on AKS and in the OKE add-on alike.
var statusConfigMap = map[string]string{"namespace": "kube-system", "configmap": "cluster-autoscaler-status"}

// Autoscaling reads the node autoscaling of the scope from kube-state-metrics
// (design, node autoscaling): enabled when the status ConfigMap of the
// cluster autoscaler was written during the window, or appeared in it when
// no resource version is exported; disabled otherwise. With the mode
// configured, or with no ConfigMap collected at all, it sends nothing.
func Autoscaling(ctx context.Context, q Querier, in Input) (AutoscalingEvidence, error) {
	if mode := in.Analysis.NodeAutoscaling; mode == AutoscalingEnabled || mode == AutoscalingDisabled {
		return AutoscalingEvidence{Status: mode, Source: discovery.StrategyConfigured, Confidence: discovery.ConfidenceExact}, nil
	}
	capability, err := findCapability(in.Capabilities, "node_autoscaling")
	if err != nil {
		return AutoscalingEvidence{}, err
	}
	if capability.State != discovery.StateAvailable {
		return AutoscalingEvidence{Status: AutoscalingUnknown, Source: discovery.StrategyNone, Confidence: discovery.ConfidenceUnknown}, nil
	}

	seconds := strconv.FormatInt(max(1, in.Window.End.Unix()-in.Window.Start.Unix()), 10)
	scope := in.Scope.Matchers()
	configMap := maps.Clone(statusConfigMap)
	maps.Copy(configMap, scope)
	queries := []string{
		"max_over_time(" + prom.Selector{Metric: "kube_configmap_info", Equal: configMap}.String() + "[" + seconds + "s])",
		"changes(" + prom.Selector{Metric: "kube_configmap_metadata_resource_version", Equal: configMap}.String() + "[" + seconds + "s])",
		"count(count by (node) (" + prom.Selector{Metric: "kube_node_info", Equal: scope}.String() + "))",
		"count(count by (node) (max_over_time(" + prom.Selector{Metric: "kube_node_created", Equal: scope}.String() + "[" + seconds + "s]) > time() - " + seconds + "))",
		"count(count by (namespace, pod) (" + prom.Selector{Metric: "kube_pod_status_unschedulable", Equal: scope}.String() + " == 1))",
	}
	var values [5]Number
	for i, query := range queries {
		answer, err := q.Query(ctx, query, in.Window.End)
		if err != nil {
			return AutoscalingEvidence{}, err
		}
		values[i] = largest(answer)
	}
	info, writes, nodes, added, unschedulable := values[0], values[1], values[2], values[3], values[4]

	status := AutoscalingDisabled
	if (writes.Known && writes.Value > 0) || (!writes.Known && info.Known) {
		status = AutoscalingEnabled
	}
	if !nodes.Known {
		added, unschedulable = Number{}, Number{}
	} else {
		added, unschedulable = orZero(added), orZero(unschedulable)
	}
	return AutoscalingEvidence{
		Status:            status,
		Source:            discovery.StrategyRawMetrics,
		Confidence:        discovery.ConfidenceInferred,
		CurrentNodes:      nodes,
		ScaleEvents:       added,
		UnschedulablePods: unschedulable,
	}, nil
}

// largest returns the largest valid, finite value of an instant answer, or
// an unknown number when it holds none: each replica of kube-state-metrics
// answers with a sample of its own.
func largest(samples []prom.Sample) Number {
	var n Number
	for _, s := range samples {
		if s.Valid && isFinite(s.Value) && (!n.Known || s.Value > n.Value) {
			n = Number{Value: s.Value, Known: true}
		}
	}
	return n
}

// orZero reads an empty answer as zero: once kube-state-metrics answered for
// the nodes, no created node or no unschedulable pod is a count of none.
func orZero(n Number) Number {
	if n.Known {
		return n
	}
	return Number{Value: 0, Known: true}
}
```

Delete the old metric constants, `autoscalerGroupLabels`, `autoscalerGroupKey`, `truncated`, `observedValue`, `hasFinitePoint`, and `groupOf` when nothing else uses them (`go vet` and the linter say). The count of a `count(...)` is whole, so `current_nodes` passes the codec's whole-number check without truncation. Fix the imports (`maps`; drop `math`, `slices`, `arith` if unused).

A collect test that builds an `Input` without a `node_autoscaling` capability now gets the error of `findCapability` from `Autoscaling`; give such inputs the capability rather than loosen the lookup.

Run: `nix develop -c go test ./internal/collect/`
Expected: the autoscaling tests PASS; `TestAllStopsAtTheFirstError` (collect_test.go:139) greps for `cluster_autoscaler` — make it look for `kube_configmap_info` instead, and run again: PASS.

- [ ] **Step 5: The snapshot accepts disabled from raw metrics**

Add to `internal/snapshot/codec_test.go` a test that a snapshot whose autoscaling is `disabled`, `raw_metrics`, `inferred` with current 4, scale events 0, unschedulable 0, and the finding `NODE_AUTOSCALING_DISABLED_PRESSURE` decodes; build it the way the neighbouring autoscaling tests build theirs. Run it red, then add the shape:

```go
// autoscalingShapes are the combinations analysis writes: a configured
// mode, no evidence at all, and the evidence of kube-state-metrics, enabled
// or disabled.
var autoscalingShapes = []autoscalingShape{
	{collect.AutoscalingEnabled, discovery.StrategyConfigured, discovery.ConfidenceExact},
	{collect.AutoscalingDisabled, discovery.StrategyConfigured, discovery.ConfidenceExact},
	{collect.AutoscalingUnknown, discovery.StrategyNone, discovery.ConfidenceUnknown},
	{collect.AutoscalingEnabled, discovery.StrategyRawMetrics, discovery.ConfidenceInferred},
	{collect.AutoscalingDisabled, discovery.StrategyRawMetrics, discovery.ConfidenceInferred},
}
```

`autoscalingCodes` (:1218) stays as it is: an enabled status still accepts the three dormant codes, so a stored report loads.

Run: `nix develop -c go test ./internal/snapshot/`
Expected: PASS.

- [ ] **Step 6: The analysis gives four findings**

In the test of `internal/analysis/autoscaling.go`, change the cases that expect `NODE_AUTOSCALING_NEAR_MAX`, `NODE_AUTOSCALING_BOUNDS_UNKNOWN`, or `NODE_AUTOSCALING_AT_MIN` to expect `NODE_AUTOSCALING_ENABLED`, keeping their evidence inputs (current 3 against max 3 included), and add a case: enabled with current 4 and no bounds gives `NODE_AUTOSCALING_ENABLED` with no evidence. Run red, then replace everything after the `disabled` case with:

```go
	return finding("NODE_AUTOSCALING_ENABLED", SeverityHealthy)
```

and the doc comment: "otherwise one finding, the first that holds of unknown, disabled, and enabled. The registry keeps the codes of near maximum, bounds unknown, and at minimum for reports stored before; they are no longer given." Drop the `config.Thresholds` parameter only if nothing else needs it; `autoscaler_near_max_percent` stays, since the HPA findings read it. Fix the callers.

Run: `nix develop -c go test ./internal/analysis/`
Expected: PASS.

- [ ] **Step 7: The added answers**

Fill the eight files of `testdata/prometheus-added/`. Each request is `"method": "POST"`, the `path` of the scenario's own instant queries (copy it from the scenario's recorded request whose `query_contains` is `cluster_autoscaler_`: `/api/v1/query`, or `/api/datasources/uid/prom/resources/api/v1/query` with `"authorization": "Bearer fixture-token"` for `grafana-authenticated`), `"status": 200`, and a vector body whose timestamp is the one of that same recorded probe answer, for example:

```json
{"method": "POST", "path": "/api/v1/query", "query_contains": "count(kube_configmap_info", "status": 200,
 "body": {"status": "success", "data": {"resultType": "vector", "result": [{"metric": {}, "value": [1784136600, "1"]}]}}}
```

An empty answer is `"result": []`. A ConfigMap answer carries the labels kube-state-metrics gives it: `{"configmap": "cluster-autoscaler-status", "namespace": "kube-system", "instance": "10.0.0.5:8080", "job": "kube-state-metrics"}`. The `query_contains` of each kind, never with a label or a span:

| Kind | `query_contains` |
|---|---|
| probe | `count(kube_configmap_info` |
| 1 | `max_over_time(kube_configmap_info` |
| 2 | `changes(kube_configmap_metadata_resource_version` |
| 3 | `count(count by (node) (kube_node_info` |
| 4 | `max_over_time(kube_node_created` |
| 5 | `kube_pod_status_unschedulable` |

The answers, in that order:

| Scenario | probe | 1 | 2 | 3 | 4 | 5 | Status |
|---|---|---|---|---|---|---|---|
| `direct-complete`, `grafana-authenticated`, `thanos-multicluster` | 41 | 1 | 120 | 1 | 1 | 1 | enabled |
| `raw-fallback` | 41 | 1 | empty | 1 | 1 | empty | enabled by presence; unschedulable 0 |
| `edge-values` | 5 | empty | empty | 1 | empty | empty | disabled; added 0, unschedulable 0 |
| `edge-scaling` | 12 | 1 | 0 | 4 | 2 | 2 | disabled, left behind; with node pressure |
| `partial-data` | empty | — | — | — | — | — | unknown; the collector sends nothing |
| `value-edge-cases` | 41 | — | — | — | — | — | configured disabled; the collector sends nothing |

Write the README's "What each file answers" from this table, one line per file, and recompute the SHA-256 block.

- [ ] **Step 8: The parity record**

Add a section to `docs/decisions/0002-differences-from-the-original.md`, in the style of its others:

```markdown
## Node autoscaling from kube-state-metrics

The original reads node autoscaling from the metrics of the cluster
autoscaler: a probe `count({__name__=~"cluster_autoscaler_.+"})` and six
range queries of `cluster_autoscaler_*`. A managed autoscaler, as on AKS,
exports none of them to a Prometheus inside the cluster, so the original
says on every such cluster that the state could not be determined. This
project reads the status ConfigMap of the cluster autoscaler and the nodes
from kube-state-metrics instead (design, node autoscaling, 2026-10-16).

The comparison with `testdata/python` therefore leaves out, on both sides:
the node autoscaling probe and the collector's queries in the lists of
queries; `summary.autoscaling` and the autoscaling collection state; and
every `NODE_AUTOSCALING_*` finding, with its recommendation, its executive
row, and its count in the severity totals. Every other query, number, and
finding is still compared.
```

In `internal/goldentest/original_test.go`:

- `TestQueriesEqualTheOriginal`: before comparing, drop from the original's lines every line containing `cluster_autoscaler_`, and from this project's lines every line containing `kube_configmap_`, `kube_node_created`, `kube_pod_status_unschedulable`, or `count(count by (node) (kube_node_info`. Pin what is dropped: for every scenario the original loses exactly 7 lines, except `value-edge-cases` (configured, 1 line: the probe); this project loses 6, except `partial-data` and `value-edge-cases` (1: the probe). Write those numbers as literals in a table of the test, and fail when a count differs.
- `compareAutoscaling` and the autoscaling state in `compareCollectionStates`: removed from the comparison, with a comment naming the decision.
- Extend `withoutRuledCodes` (or add `withoutNodeAutoscaling` beside it, applied in the same places on both sides) so that every finding whose code starts with `NODE_AUTOSCALING_`, its recommendation, its executive row, and its count in the severity totals leave both the snapshot and the original's record before the comparison. Follow how the readiness codes are taken out today, including the adjustment of the summary counts.
- `TestComparisonCanFail`: replace the sub-test "a number of the autoscaling changed" with "a finding of the nodes changed" or another sub-test that changes a number still compared, so the test still proves a failure in every compared part.

- [ ] **Step 9: The tests that name the old requests or their counts**

- `internal/pipeline/pipeline_test.go` (`emptyCollectors` :150-171) and `internal/platform/worker/collect_test.go:386-393`: the substring `cluster_autoscaler_` becomes `kube_configmap_info` for the probe; where they answer the collectors empty, they also answer the five new queries empty (`count(count by` and `max_over_time(kube_configmap_info`, `changes(kube_configmap_` cover them).
- `internal/cli/run_test.go:1498` `TestSnapshotRunSendsTheCollectorQueries`: 29 preflight requests and 68 in all (five autoscaling queries in place of six). `:1898-1917` `TestNoUsefulSectionsExitsFive`: its helper answers the probe so that the status is unknown (no ConfigMap).
- `internal/promtest/fixture_test.go` `TestLoadReadsEachScenario`: it counts the recorded files alone; leave it, and add the counts of `LoadScenario` per scenario as literals: the recorded count plus 6 for the six scenarios that run the collector (`direct-complete`, `grafana-authenticated`, `thanos-multicluster`, `raw-fallback`, `edge-values`, `edge-scaling`; `direct-complete-range` shares the file of `direct-complete`), plus 1 for `partial-data` and `value-edge-cases`.
- `internal/goldentest/edge_values_test.go`: run the three tests and write the counts they now find as their literals: the seven recorded autoscaler requests go unused, so `direct-complete` should leave 8 (1 before), `edge-values` 8 (1 before), and `edge-scaling` 7 (0 before). If a count differs from these, stop and report: an added answer went unasked or was taken by another query. Keep each doc comment true: name the unused HPA answer and the seven recorded answers of the old autoscaler; a test whose name says no answer is left unused is renamed to say which answers are left (for example `TestEdgeScalingLeavesOnlyTheAutoscalerAnswers`).
- `Makefile:210`: `remaining: 1` becomes `remaining: 8`, and the comment above it (:185-189) says why.

- [ ] **Step 10: Regenerate and read**

Run: `nix develop -c make update-testdata`
Then `git diff --stat` and read every changed file whole. Expected: the queries lists lose the old probe and six range lines and gain the new probe and five instant lines (or the probe alone); each snapshot's `summary.autoscaling` and its autoscaling finding follow the table of Step 7 (`direct-complete` enabled with current 1, scale events 1, unschedulable 1, finding `NODE_AUTOSCALING_ENABLED`; `edge-scaling` disabled with current 4, scale events 2, unschedulable 2, finding `NODE_AUTOSCALING_DISABLED_PRESSURE`; `edge-values` disabled with 1, 0, 0, `NODE_AUTOSCALING_DISABLED`; `raw-fallback` enabled with 1, 1, 0; `partial-data` unknown; `value-edge-cases` configured disabled), the capability evidence reads `kube_configmap_info`, and nothing else moves except the severity totals and the counts that follow from the findings. A value you cannot explain is a defect: stop and report it.

- [ ] **Step 11: The query rule is held**

Add to `internal/goldentest/golden_test.go`:

```go
func TestNoQueryNamesTheClusterAutoscaler(t *testing.T) {
	for _, s := range slices.Concat(promtest.Scenarios, promtest.RangeScenarios) {
		for _, line := range queriesLines(t, s) {
			if strings.Contains(line, "cluster_autoscaler_") {
				t.Errorf("%s: %q names the cluster autoscaler", s.Name, line)
			}
		}
	}
}
```

- [ ] **Step 12: Whole suite, lint, commit**

Run: `nix develop -c make test && nix develop -c make lint && cd web && npx vitest run`
Expected: PASS. A web test that read the regenerated views and fails on the autoscaling display belongs to Task 4: if one fails, report it rather than change the display here.

```bash
git add internal/snapshot/testdata/stored-2.3.0-direct-complete.json internal/discovery/ internal/collect/ internal/snapshot/ internal/analysis/ internal/goldentest/ internal/pipeline/pipeline_test.go internal/platform/worker/collect_test.go internal/cli/run_test.go internal/promtest/ testdata/prometheus-added/ testdata/snapshot/ testdata/queries/ template/tests/capacity/fixtures/command/ web/tests/fixtures/view/ docs/decisions/0002-differences-from-the-original.md Makefile
git commit -m "feat(collect): read node autoscaling from KSM"
```

---

### Task 4: the report shows what is known

**Files:**
- Modify: `internal/document/verdict.go:92-135` (`verdictGrowth`), its tests in `internal/document/verdict_test.go` (`TestVerdictOfEdgeScalingGrowth` :50, `WithoutAMaximum` ~:219, `WithoutACurrentCount`, `WritesAnAbsentFactAsUnavailable` ~:291), `internal/document/document.go:246-256` (the comment of `VerdictGrowth`), `internal/document/document_test.go:214,230,261`
- Modify: `internal/catalog/keys.go` (:263-267, :379-380, :411-421, :581-582), `internal/catalog/locales/en.toml`, `internal/catalog/locales/pt-BR.toml`
- Modify: `web/src/features/report/overview-tab.tsx:33-84` (`GrowthPair`), `web/src/features/report/autoscaling-tab.tsx:102-117`, `web/src/locales/en.json`, `web/src/locales/pt-BR.json`
- Test: `web/src/features/report/overview-tab.test.tsx`, `web/src/features/report/autoscaling-nodes.test.tsx`
- Regenerate: `template/tests/capacity/fixtures/command/`, `web/tests/fixtures/view/` (by `make update-testdata`)

**Interfaces:**
- Consumes: the snapshots of Task 3 and `internal/snapshot/testdata/stored-2.3.0-direct-complete.json`.
- Produces: catalog keys `verdict.growth.state.enabled` and `verdict.growth.current_nodes` (Go: `catalog.VerdictGrowthCurrentNodes`; `VerdictGrowthStates` gains `enabled`).
- Produces: the PDF data file's `growth` block: `current`, `max`, `caption` set together or not at all; `state` set when they are not; `facts` holds only known values.

- [ ] **Step 1: The verdict's tests (failing first)**

In `internal/document/verdict_test.go`, write the rule as four cases, each with literal texts in English:

1. Enabled with current 1, no maximum, scale events 1, unschedulable 1 (today's `direct-complete` after Task 3): `State` is "Enabled"; `Current`, `Max`, `Caption`, and `Gauge` are nil; `Facts` are, in order, Current nodes "1", Unschedulable pods "1", Nodes added "1".
2. Enabled with current 3, maximum 3, minimum unknown, scale events 1, unschedulable 2: `Current` "3", `Max` "3", the caption, the gauge; `State` nil; `Facts` are Unschedulable pods "2", Nodes added "1" (no minimum).
3. The stored 2.3.0 report (`internal/snapshot/testdata/stored-2.3.0-direct-complete.json`, read through `snapshot.Decode`): `Current` "3", `Max` "3", the gauge, `Facts` Minimum nodes "1", Unschedulable pods "1", Nodes added "1".
4. Disabled from raw metrics with current 4: `State` "Disabled", no facts.

Replace `WithoutAMaximum` and `WritesAnAbsentFactAsUnavailable` with cases 1 and 2; `WithoutACurrentCount` (enabled, maximum known, current unknown) now expects the state word and the facts that are known.

Run: `nix develop -c go test ./internal/document/ -run Verdict -v`
Expected: FAIL.

- [ ] **Step 2: Catalog keys and wording**

In `internal/catalog/keys.go`: `VerdictGrowthStates = []string{"enabled", "disabled", "unknown"}` with its comment "lists the autoscaling statuses the verdict words as a state: every status, when the pair of current against maximum is not drawn"; add `VerdictGrowthCurrentNodes Key = "verdict.growth.current_nodes"` beside `VerdictGrowthMinNodes`, and add it wherever the growth keys are listed (:379-380). In both `locales/*.toml`, enter the wording of the Global Constraints table exactly: `[verdict.growth]` gains `current_nodes`, `scale_events` changes, `[verdict.growth.state]` gains `enabled`, and the three finding entries change.

Run: `nix develop -c go test ./internal/catalog/ ./internal/guard/`
Expected: PASS once the verdict uses the new key (the guard wants every key named in `internal/document`); if the guard fails only for that reason, go on to Step 3.

- [ ] **Step 3: Implement the verdict**

```go
	if a.Status != collect.AutoscalingEnabled || a.CurrentNodes == nil || a.MaxNodes == nil {
		state := b.text(catalog.VerdictGrowthState(a.Status), nil)
		growth.State = &state
		if a.Status == collect.AutoscalingEnabled {
			growth.Facts = knownFacts(b, vb, []growthFact{
				{catalog.VerdictGrowthCurrentNodes, a.CurrentNodes},
				{catalog.VerdictGrowthUnschedulablePods, a.UnschedulablePods},
				{catalog.VerdictGrowthScaleEvents, a.ScaleEvents},
			})
		}
		return growth
	}

	current := vb.number(a.CurrentNodes, b.cat.Count).Text
	growth.Current = &current
	maximum := vb.number(a.MaxNodes, b.cat.Count).Text
	growth.Max = &maximum
	caption := b.text(catalog.VerdictGrowthCaption, nil)
	growth.Caption = &caption
	if g := facts.Gauge; g != nil {
		growth.Gauge = &VerdictGauge{Current: g.Current, Max: g.Max, Min: g.Min, Hazard: g.Hazard != ""}
	}
	growth.Facts = knownFacts(b, vb, []growthFact{
		{catalog.VerdictGrowthMinNodes, a.MinNodes},
		{catalog.VerdictGrowthUnschedulablePods, a.UnschedulablePods},
		{catalog.VerdictGrowthScaleEvents, a.ScaleEvents},
	})
	return growth
}

// growthFact is a count the verdict may state beside the growth of the
// cluster, with the key of its label.
type growthFact struct {
	label catalog.Key
	value *float64
}

// knownFacts words the facts whose count is known, in order, and leaves out
// the others: a report from kube-state-metrics holds no minimum.
func knownFacts(b *builder, vb *viewBuilder, facts []growthFact) []VerdictFact {
	words := []VerdictFact{}
	for _, f := range facts {
		if f.value == nil {
			continue
		}
		words = append(words, VerdictFact{Label: b.text(f.label, nil), Value: vb.number(f.value, b.cat.Count).Text})
	}
	return words
}
```

Change the comment of `verdictGrowth` to say the pair is drawn only when the status is enabled and both counts are known, and the comment of `VerdictGrowth` (document.go:248-249) to say a fact is left out when its count is absent. Check the type names (`catalog.Key`, `*float64` for the summary counts) against the code and adapt.

Run: `nix develop -c go test ./internal/document/`
Expected: the verdict tests PASS; fix `document_test.go:214,230,261` to the new shape.

- [ ] **Step 4: Regenerate the data files and views, read them**

Run: `nix develop -c make update-testdata`, then read every changed file whole. Expected: in the data files, the `growth` block of `direct-complete` and `raw-fallback` holds `"state": "Enabled"` (or "Ativo"), `current`, `max`, and `caption` null, and the known facts; `edge-scaling` and `edge-values` hold "Disabled" ("Inativo") and no facts; `partial-data` "Unknown"; the words of the changed findings follow the table. The views change only in the finding texts.

- [ ] **Step 5: The web's tests (failing first)**

In `web/src/features/report/overview-tab.test.tsx`, for the growth pair:

1. From the `direct-complete` view (English): the proof shows "Enabled", no caption "Nodes · current / maximum", no proof bar; the claim's facts read "Current nodes 1", "Unschedulable pods 1", "Nodes added 1", in that order, and nothing reads "Unavailable" or "Minimum nodes".
2. From the `direct-complete` view with `summary.autoscaling` overridden to current 3, maximum 3, minimum null, and a gauge in `verdict.gauge`: the caption, "3/3", the proof bar; facts "Unschedulable pods 1", "Nodes added 1".
3. From the `edge-scaling` view: "Disabled" and no facts.
4. In Portuguese from the `direct-complete` view: "Ativo", "Nodes atuais 1".

In `web/src/features/report/autoscaling-nodes.test.tsx` ("cluster line and counts"): from `edge-scaling`, the terms are Current nodes, Nodes added, Unschedulable pods, Workloads without HPA (no Minimum, Maximum, or Managed node pools), with their values from the regenerated fixture as literals.

Run: `cd web && npx vitest run src/features/report/overview-tab.test.tsx src/features/report/autoscaling-nodes.test.tsx`
Expected: FAIL.

- [ ] **Step 6: Implement the web**

In `GrowthPair`:

```tsx
  const enabled = autoscaling.status === "enabled";
  const paired = enabled && autoscaling.current_nodes.raw !== null && autoscaling.max_nodes.raw !== null;
  const facts = (
    paired
      ? [
          { label: t("report.autoscaling.minNodes"), value: autoscaling.min_nodes },
          { label: t("report.autoscaling.unschedulablePods"), value: autoscaling.unschedulable_pods },
          { label: t("report.autoscaling.scaleEvents"), value: autoscaling.scale_events },
        ]
      : [
          { label: t("report.autoscaling.currentNodes"), value: autoscaling.current_nodes },
          { label: t("report.autoscaling.unschedulablePods"), value: autoscaling.unschedulable_pods },
          { label: t("report.autoscaling.scaleEvents"), value: autoscaling.scale_events },
        ]
  ).filter((fact) => fact.value.raw !== null);
```

The facts line shows when `enabled && facts.length > 0`; the proof is the caption, the pair, and the bar when `paired`, and otherwise the status word (`t(\`autoscalingStatus.${autoscaling.status}\`)`), as the non-enabled branch draws it today. In `autoscaling-tab.tsx`, `Counts` receives only the items whose value is known (`raw !== null`); `workloads_without_hpa` keeps its place. In both locale files, `report.autoscaling.scaleEvents` becomes "Nodes added" / "Nodes adicionados". Update the comment above `GrowthPair`.

Run: `cd web && npx vitest run && npx tsc --noEmit -p . && npx eslint .`
Expected: PASS.

- [ ] **Step 7: The stored report still renders its pair**

Add to `internal/document` a test that builds the PDF data file and the view of `internal/snapshot/testdata/stored-2.3.0-direct-complete.json` in both languages without error, that the data file's growth block holds the pair "3" / "3" and a gauge, and that the view's `verdict.gauge` is not null.

Run: `nix develop -c go test ./internal/document/`
Expected: PASS.

- [ ] **Step 8: Whole suite, lint, commit**

Run: `nix develop -c make test && nix develop -c make lint`
Expected: PASS.

```bash
git add internal/document/ internal/catalog/ web/src/features/report/ web/src/locales/ template/tests/capacity/fixtures/command/ web/tests/fixtures/view/ testdata/snapshot/
git commit -m "feat(report): show only known autoscaling numbers"
```

---

### Task 5: the documents

**Files:**
- Modify: `docs/operations.md`, `docs/superpowers/specs/2026-09-27-k8s-capacity-report-design.md`, `AGENTS.md`, `README.md`, `CHANGELOG.md`

- [ ] **Step 1: The guide**

In `docs/operations.md`, in the section that lists what the metric source must hold (find where kube-state-metrics and the node metrics are named), add:

```markdown
### Node autoscaling

The report reads node autoscaling from kube-state-metrics, as
kube-prometheus-stack collects it, with nothing to configure. The cluster
autoscaler writes its status to the ConfigMap
`kube-system/cluster-autoscaler-status`, on AKS and in the OKE add-on alike;
kube-state-metrics exports its existence (`kube_configmap_info`) and each
change of it (`kube_configmap_metadata_resource_version`).

| Status | When |
|---|---|
| enabled | the ConfigMap was written during the period, or, when kube-state-metrics exports no resource version, it appeared in the period |
| disabled | ConfigMaps are collected, and that one never appeared or was never written during the period |
| unknown | kube-state-metrics exports no ConfigMap at all |

The report also states the current nodes, the nodes added during the
period, and the unschedulable pods, from `kube_node_info`,
`kube_node_created`, and `kube_pod_status_unschedulable`. It does not know
the minimum or the maximum of a node pool.

Two installations need the state written in the profile or in
`analysis.node_autoscaling`: a kube-state-metrics whose collector list was
trimmed and no longer holds `configmaps`, and an OKE add-on whose
`statusConfigMapName` was changed. A cluster that scales its nodes with
Karpenter alone reads disabled.
```

- [ ] **Step 2: The design spec**

In `docs/superpowers/specs/2026-09-27-k8s-capacity-report-design.md`, find every place that names the cluster autoscaler's metrics, the regex probe, the `increase` span of the autoscaling (:338), the findings of near maximum, bounds unknown, or at minimum as given, the autoscaling evidence (:409), and the web's autoscaling line (:1063), and bring each in line with the spec of phase 14: the probe and the five queries, the status rule, the evidence, the four findings, and the known numbers alone. Name the spec of phase 14 once where the collector is described.

- [ ] **Step 3: AGENTS.md**

- In "The command", the rule "The collectors keep the order of the original too ... The text of every query equals the original letter for letter." ends: "except the node autoscaling probe and collector, which read kube-state-metrics (decision 0002); the comparison with the original leaves them out."
- In "The interface", the `make e2e` paragraph: `remaining: 1` becomes `remaining: 8`: "the HPA answer of `direct-complete` that no run asks and the seven recorded answers of the cluster autoscaler"; and the fakes read `testdata/prometheus-added/direct-complete.json` through `--added`.
- In "Golden snapshots and data files", add: "`testdata/prometheus-added/` holds, for each scenario, the answers to the queries the original never sent, read before the recorded answers by `promtest.LoadScenario`; its README pins each file by SHA-256 and says what each answer exercises."

- [ ] **Step 4: README and changelog**

In `README.md`, change only a line that names the cluster autoscaler's metrics or its bounds (:152, :164, :230 are the candidates); the exit code 5 line stays true. Under `## [Unreleased]` in `CHANGELOG.md`:

```markdown
### Changed

- Node autoscaling is now read from kube-state-metrics, as kube-prometheus-stack collects it, so a report on a cluster whose autoscaler is managed, as on AKS, or not scraped, as on OKE, says whether node autoscaling is enabled or disabled instead of saying it could not be determined.
- A report no longer shows the autoscaler's minimum and maximum nodes, the node pools it manages, or the gauge of nodes against the maximum, which kube-state-metrics does not hold; a report made before keeps them.
- "Scale events" now reads "Nodes added" and counts every node added during the period.
- A count of node autoscaling that is not known is left out of the report instead of reading Unavailable.
```

The fix of F-269 is a test alone and changes no report, so the changelog leaves it out; say so in the report of the task.

- [ ] **Step 5: Check and commit**

Run: `nix develop -c make test` (the guide's tables are compared with the code by two tests).
Expected: PASS.

```bash
git add docs/operations.md docs/superpowers/specs/2026-09-27-k8s-capacity-report-design.md AGENTS.md README.md CHANGELOG.md
git commit -m "docs: read node autoscaling from KSM"
```

---

### Task 6: golden pages, e2e, the AKS check, reviews (main thread)

- [ ] `make update-golden`; open every page that changed and review it: only page 1 of the command pages moves (the growth block); `capacity-verdict` and `capacity-verdict-many` (hand fixtures with the pair) do not. Commit the pages with the reason.
- [ ] `make test-template`, `make test-golden`, `make test-postgres` (the worker's render tests pass a creation time).
- [ ] `make e2e`: both fakes log `remaining: 8`. Check the port 8080 is free first (the look stack).
- [ ] The AKS check after implementation (spec section 8), on the owner's word naming the target: `bin/capacity-room report --check` and a snapshot and a PDF against the staging AKS cluster through a port-forward, with a `report.toml` under `.cache/phase-14/aks/`. Expected: `node_autoscaling` available, `raw_metrics`; status enabled; no "could not be determined". Record in ROADMAP, never in a tracked file.
- [ ] Whole-branch adversarial review over a diff file (leave out `deploy/helm/schemas/`), one fix wave, re-review.

### Task 7: version, gate (main thread)

- [ ] Version commit 2.4.0: `VERSION`, the changelog heading `## [2.4.0] - <date>`, `deploy/helm/capacity-room/Chart.yaml` (`version`, `appVersion`), and `template/graphite` (`typst.toml`, the `@local/graphite-report:2.4.0` identifiers of its `README.md` and `template/main.typ`).
- [ ] `make gate` until "gate passed for version 2.4.0", logs under `.cache/release-2.4.0/`; `make audit`.
- [ ] ROADMAP and CHANGELOG read back; merge, tag, image, and push each on the owner's word, naming the target.
