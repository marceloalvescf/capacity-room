# AGENTS.md
This file provides guidance for LLMs working with this repository's code.

## Environment and commands

Linux/Python is the canonical environment. Create the environment and install the test extras with:

```bash
python -m venv .venv
.venv/bin/python -m pip install -e '.[test]'
.venv/bin/python -m pytest -q
.venv/bin/python -m pytest \
  tests/test_typst_renderer.py::test_preamble_uses_current_graphite_report_api \
  -q
.venv/bin/python -m pytest tests/test_typst_renderer.py -k golden -q
```

The full suite needs a live PostgreSQL: tests marked `postgres` fail, never skip, without it.
Check `docker ps` first, the container may have exited between sessions:

```bash
docker compose -f deploy/compose.test.yaml up -d --wait
TEST_POSTGRES_DSN='postgresql+psycopg://kcr_test:kcr_test_only@127.0.0.1:55432/kcr_test' \
  PYTHONPATH="$PWD" .venv/bin/python -m pytest -q
```

Only one pytest run at a time: `pgrep -af '^\S*python\S* -m pytest'` must print nothing before
starting another. A `ConnectionResetError` traceback in the middle of a run is noise from a fake
TLS server, not a failure. In a worktree, use the main checkout's `.venv/bin/python` with
`PYTHONPATH=<worktree>`.

The frontend lives in `frontend/` (React, TypeScript, Vite). A worktree needs its own
`npm ci --prefer-offline`:

```bash
cd frontend
npx vitest run                 # unit and component tests
npx tsc --noEmit -p .          # type check
npm run build                  # tsc -b and the production build
KCR_E2E_CHROMIUM=/path/to/chromium npx playwright test   # against a running Compose stack
```

The integration stack is Compose. The end-to-end suite runs against it without any operator
overlay:

```bash
env -u KCR_APP_VERSION GRAPHITE_REPORT_CONTEXT=/path/to/graphite-report \
  docker compose -f deploy/compose.yaml -f deploy/compose.integration.yaml \
  up -d --build --force-recreate --wait
```

Before closing out a release, it is mandatory to run the compilation and PDF text-layer checks
against the available sibling package:

```bash
export GRAPHITE_ROOT=/path/to/graphite-report
nix shell nixpkgs#poppler-utils --command \
  nix develop "path:$GRAPHITE_ROOT" --command \
  .venv/bin/python -m pytest -m typst -q -rs
```

Six checks carry the `typst` marker; the expected result is `6 passed` with no skip.

**`pdftotext` doesn't come from either devshell** — neither this repo's `flake.nix` nor the
sibling's declares it, and without it four of the six checks skip. It comes from
`nixpkgs#poppler-utils`, as above. `typst` doesn't come from either devshell on its own either —
it only becomes available through `nix develop "path:$GRAPHITE_ROOT"` (see "Getting the `typst`
binary" below), so the command above nests both wrappers rather than relying on just one.

A skipped test does not count as a check that ran; the skip reason tells you whether the
`typst` binary, the `pdftotext` binary, the `GRAPHITE_ROOT` variable, or some package file is
missing. **A gate that skips is not a gate that was checked**: between 2026-08-10 and 2026-08-15
the third check skipped silently, because the sibling package renamed its installer and the test
was looking for the old name (N30).

`pyproject.toml` doesn't define a dedicated Python linter or type checker; don't invent a project
lint command.

Use the CLI with a valid `report.toml`:

```bash
.venv/bin/python -m kube_capacity_report --config report.toml --check
mkdir -p build
.venv/bin/python -m kube_capacity_report \
  --config report.toml \
  --output build/report.typ
```

Optional PDF validation uses the sibling package `graphite-report`. Installation and compilation
must share the same `XDG_DATA_HOME`:

```bash
GRAPHITE_ROOT=../graphite-report
REPORT_XDG="$GRAPHITE_ROOT/build/xdg"
XDG_DATA_HOME="$REPORT_XDG" python3 "$GRAPHITE_ROOT/scripts/install-local.py"
nix develop "path:$GRAPHITE_ROOT" --command env \
  XDG_DATA_HOME="$REPORT_XDG" \
  typst compile \
  --root "$PWD" \
  --font-path "$GRAPHITE_ROOT/assets/fonts" \
  --ignore-system-fonts \
  --ignore-embedded-fonts \
  build/report.typ \
  build/report.pdf
```

On native Windows, use only the commands below:

```powershell
py -3.11 -m venv .venv
.\.venv\Scripts\python.exe -m pip install -e ".[test]" tzdata
.\.venv\Scripts\python.exe -m pytest -q
```

`tzdata` provides IANA zones such as `America/Sao_Paulo`. **PDF compilation has no path on native
Windows**: the sibling package dropped native support in `5f5c7bd` and no longer ships the
`install-local.ps1` this file used to reference. Use WSL or Linux for the PDF gate.

### Getting the `typst` binary

`typst` is not on `PATH`. It comes from the sibling package's devshell and must always be
obtained via `nix develop`, which performs the derivation and makes the binary available to the
command it wraps:

```bash
nix develop "path:$GRAPHITE_ROOT" --command typst --version
```

Use the same form for any compilation, wrapping the entire command in
`nix develop ... --command bash -c '...'` when more than one step is needed.

### Environment limitations that have already cost time

- **Subagents can't use Nix.** The sandbox denies the daemon
  (`/nix/var/nix/daemon-socket/socket: Operation not permitted`, and `NIX_REMOTE=local` fails on
  the database lock). Never instruct a subagent to run `nix develop` or `nix shell`: it will hang
  without producing any work. Perform the derivation on the main thread and have the subagent
  call `.venv/bin/python` by its direct path.
- **Subagents don't run Docker, `npm ci`, or the end-to-end suite either.** The stack rebuild,
  the Playwright run, and every probe against the live stack stay on the main thread. A subagent
  may run `npx vitest run`, `npx tsc --noEmit -p .`, and `npm run build` in a worktree whose
  `node_modules` the main thread installed.
- **The worker container's root filesystem is read-only**, so `docker cp` into it fails. To run a
  probe inside it, pipe the file into `/tmp`: `docker exec -i <container> sh -c 'cat > /tmp/x'`.
- **`.venv` breaks after garbage collection.** The interpreter points to a store path that isn't
  a GC root. The symptom is `No such file or directory` on a symlink that looks intact.
  Recreating the venv from inside `nix develop` fixes it.
- **Devshell binaries don't survive GC.** This applies to both `typst` and Python itself. If a
  check needs them durably, that requires an explicit GC root, not `nix develop`.
- **A Codex job can hang without warning, and working is indistinguishable from hung by the
  process.** On 2026-08-04 a dispatch stayed alive for **7 hours** with **6 seconds** of
  accumulated CPU and **zero** writes to the repository. The three signals that tell them apart:
  `ps -o time` (accumulated CPU, not instantaneous `%CPU`), the `mtime` of repository files, and
  log growth under `~/.codex/`. **An untouched working tree after the time ceiling isn't "still
  reading," it's a failure** — and the subagent often returns before the job does, so the final
  report never reaches the main thread: judge the diff and run the mutations yourself.
- **When closing out a hung job, kill the whole tree, from the parent `node` process down** —
  killing only `codex app-server` leaves the plugin thinking it still has a server, and the next
  dispatch dies at the start. And **never kill by name pattern**: `pkill -f codex` hits the
  launcher of **other open Claude sessions**. What separates them is ancestry — a job dispatched
  in the background hangs off `systemd --user`; a live session hangs off its own `claude` binary.

## Delegating to subagents

On parallelism:

- **Tasks that don't compete with each other go in parallel**, one subagent per task, dispatched
  in the same round.
- **Tasks that compete with each other go in sequence**, in the order the dependency requires,
  with each result verified before dispatching the next.

Two tasks **compete** when they share any of the following: the same code or test file; a golden
file under `tests/golden/typst`; `ROADMAP.md`, `CHANGELOG.md`, or the version in `pyproject.toml`
and `kube_capacity_report/__init__.py`. When in doubt, treat them as competing: an unnecessary
sequence costs time, a silent overwrite costs the entire fix.

Two subagents may share one worktree in parallel only when their paths are disjoint (for example
`kube_capacity_report/` plus `tests/` against `frontend/`) and every commit names its paths:
`git commit -m "..." -- <paths>`, never `git add -A` or `git commit -a`. Wait for a running
pytest to finish before a subagent starts editing Python files: the suite reads sources at test
time.

### What can't be delegated

The subagent sandbox can't reach the Nix daemon (see *Environment limitations*). The steps that
depend on it, plus the documentation exception below, always stay on the main thread:

- creating or recreating `.venv`, which requires `nix develop`;
- compiling Typst and extracting PDF text, which depend on the devshell's `typst` and
  `pdftotext`;
- the `pytest -m typst` gate, which needs `GRAPHITE_ROOT` and the two binaries above;
- `ROADMAP.md` is written by the main thread, never by a subagent: it records what was actually
  completed and in which commit, and is the convergence point for the work. A subagent only sees
  its own task and can't judge overall state. `README.md`, `CHANGELOG.md`, and docstrings are
  delegated normally.

The subagent receives the absolute path to `.venv/bin/python` and runs the suite through it.
Never instruct a subagent to invoke `nix`.

### Closing out

After any parallel round, the full suite is run once against the combined result, either by the
last subagent to finish or by the main thread. A subagent that only runs its own test files can't
see cross-cutting regressions; that's how thirteen tests broke without anyone noticing.

## Fix roadmap

`ROADMAP.md`, at the repository root, is the living record of review findings and the status of
each fix. It **is not version-controlled** (it's in `.gitignore`): it's a working tool, not a
deliverable. If the file doesn't exist, create it from the structure described below before
starting any fix.

Minimum structure:

- **How to use it** and a status **legend** (`DONE`, `OPEN`, `PARTIAL`, `DEFERRED`) and source
  (`REV` code review, `PROD` real-world execution, `JUDGE` review of an agent's work). The file
  itself is written in Portuguese, so the statuses appear there as `FEITO`, `ABERTO`, `PARCIAL`,
  and `ADIADO`.
- One section per phase, each item with a stable ID, source, a one-line description, and
  `file:line`.
- **Findings log**, where every new finding is recorded before being assigned a phase.
- **Learned process rules**, so orchestration mistakes aren't repeated.

Usage rules:

- Check the current phase and confirm the item is still `OPEN` before starting. A `DONE` item
  carries the commit that closed it.
- On completion, mark it `DONE` and note the commit. A `DONE` item with no corresponding test
  doesn't count.
- When something new is discovered during a fix, log it in the *Findings log* with an ID,
  evidence, and file, and assign it a phase before closing the task. A finding with no phase
  becomes invisible debt.
- A finding from real execution (cluster, compiled PDF) takes precedence over a finding from
  reading code: it has already proven it affects the output.
- IDs are stable and never reused, even after the item is closed.

## Per-branch workflow

Each set of fixes lives on its own branch, usually in a worktree under `.worktrees/`.
`flake.nix` and `flake.lock` are not version-controlled and only exist in the main working
directory, so `path:` references must point to the main repository, never to the worktree.

**Integration happens by phase, not by item.** A phase is one work package (`WPnn`) since the
web platform. The branch only goes to `master` when the last
item in the phase closes, via fast-forward, accompanied by a tag when the version changed. An item that's ready, tested,
and even validated against a real cluster is not merged alone — it waits for its siblings. This
keeps the history readable by phase and makes the tag mean something.

When wrapping up the tasks planned for the branch, before merging:

1. Run the full suite, not just the touched files. Cross-area regressions only show up in the
   full suite.
2. Update `ROADMAP.md`: mark items `DONE` with the commit, and log anything discovered along the
   way in the *Findings log*.
3. Update `CHANGELOG.md` per the section below.
4. Bump the version only if there's a user-visible effect.

## Changelog and versioning

`CHANGELOG.md` is version-controlled, unlike `ROADMAP.md`. It follows
[Keep a Changelog](https://keepachangelog.com/pt-BR/1.1.0/) and Semantic Versioning, and it is
written in Portuguese (Brazil).

Writing rules:

- Group entries under `### Adicionado`, `### Alterado`, `### Obsoleto`, `### Removido`,
  `### Corrigido`, and `### Segurança` (Added, Changed, Deprecated, Removed, Fixed, Security).
  Include a subheading only when it has items. The entry heading is `## [X.Y.Z] - YYYY-MM-DD`.
- Each item is a single, concise sentence describing the observable effect. No internal jargon,
  no function names, no commit hashes.
- Describe what the user perceives, not what the commit did. "The inventory stops showing up
  empty" is worth more than "adds a fallback in `workloads_without_hpa`".
- Omit commits with no effect for the tool's users: `.gitignore` tweaks, tests, internal
  documentation, and contract changes that only enable another item already listed.
- Disclose omissions when delivering. A silent omission is indistinguishable from forgetting.

Versioning rules:

- When bumping, change all seven places in the same commit, `chore(release): X.Y.Z`:
  `pyproject.toml`, `kube_capacity_report/__init__.py`, the image tag in
  `deploy/kubernetes/base/web-deployment.yaml`, `worker-deployment.yaml`, and `migration-job.yaml`,
  the Job name `kcr-migrate-X-Y-Z` in `migration-job.yaml`, and `docs/web-platform-operations.md`,
  which carries both the dotted image tag and the dashed Job name. They must not diverge. Two
  tests pin them, and only the full suite runs both:
  `test_version_literals_agree_across_the_release_surface` (`tests/test_readme.py`) compares
  `pyproject.toml` with `__init__.py`, the newest `CHANGELOG.md` heading, and the two strings in
  the operations document; `test_image_tag_and_job_name_follow_the_project_version`
  (`tests/integration/test_kubernetes_manifests.py`) compares it with the three manifests. Because
  of the first, an unreleased section above the current version in `CHANGELOG.md` fails the suite.
- The release gates run on the release commit, with their logs kept under
  `.cache/release-X.Y.Z/`: the full suite with PostgreSQL, the `typst` gate with no skip, Vitest,
  the type check, the production build, a stack rebuilt from that commit with the integration
  overlay (`deploy/compose.integration.yaml`, which the end-to-end suite needs for its fake
  Prometheus and profiles) and without any operator overlay, and the end-to-end suite. The tag is
  annotated and is created only after the gates pass.
- Merging to master and bumping the version are independent decisions. A phase made up only of
  tests or internal refactoring is merged without a bump — and, with no new version, no tag. A
  tag that repeats the previous version lies about what changed; record the decision in
  `ROADMAP.md`.
- The tag names the version present in `pyproject.toml` at the merged commit, in the format
  `vX.Y.Z`.
- A defect fix is PATCH. A change to when a finding fires, or to the text of a report section, is
  observable behavior and calls for MINOR.

## Architectural flow

```text
TOML < environment < CLI
  -> endpoint candidates and Prometheus client
  -> cluster scope and capability discovery
  -> collectors and identity resolvers
  -> capacity/risk analysis and recommendations
  -> ReportData
  -> deterministic Typst renderer
  -> atomic .typ write
```

The web platform reuses the same collection and analysis and adds a stored snapshot between them
and the two outputs:

```text
browser (frontend/) -> API (platform/api, FastAPI) -> job in the PostgreSQL queue
  -> worker: profile -> collection -> analysis -> ReportData
  -> snapshots/mapper_v1 -> SnapshotV1, stored
       -> served as JSON -> interactive HTML in the browser
       -> snapshots/adapter_v1 -> ReportData -> Typst renderer -> pinned typst -> PDF in object storage
```

- `application.py` orchestrates the flow and isolates collectors via `CollectorFactory`.
- `metrics/` handles HTTP, endpoint shapes, selectors, scope, and strategy discovery.
- `collectors/` gathers domain measurements; `resolvers/` normalizes workload, node, and
  node-pool identities.
- `analysis/` produces findings and recommendations with no presentation responsibilities
  beyond the sentences themselves, which it writes in their final wording (see *Invariants*).
- `models.py` is the contract between collection, analysis, and rendering.
- `snapshots/` is the public, versioned contract of the web platform: `dto_v1.py` defines
  `SnapshotV1`, `mapper_v1.py` writes it from `ReportData`, `adapter_v1.py` reads it back for the
  PDF. Both pass prose through untouched. `schema_version` changes only when the shape changes.
- `platform/` is the web platform: API, authentication, profiles, the transactional queue, the
  worker, artifacts, retention. `platform/commands.py` is the operator command line.
- `frontend/` is the browser application. Enum and identifier labels live in
  `frontend/src/features/report/format.ts` and are meant to say the same as the label maps of
  `renderers/text.py`. Nothing enforces it: the two are kept equal by hand, no test compares
  them, and today the strategy labels differ (`Configuração` and `Sem fonte` in the document,
  `Configurada` and `Nenhuma` in the web). Change a label on both sides.
  Icons are `@phosphor-icons/react`; Inter is the interface font and Geist Mono (class
  `font-mono-data`) is only for identifiers and metrics; the Content Security Policy is
  `style-src 'self'` plus the documented hashes, so no inline style.
- `renderers/` is a package, not a single file: D2 split `typst.py` into **nine modules**,
  alongside `escaping.py`, which already existed. `typst.py` is the **facade** — about 35 lines,
  exposing `render_report` and `RenderOptions` — and the pt-BR editorial presentation lives in
  `options.py` (rendering configuration), `text.py` (label maps and formatters), `severity.py`,
  `tables.py`, `capabilities.py`, `nodes.py`, `workloads.py`, and `sections.py`. `escaping.py`
  holds emission. The import order is strict and never goes upward:
  `options <- text <- severity <- tables <- {capabilities, nodes} <- workloads <- sections <- typst`.
- The facade **doesn't re-export anything it doesn't use itself**, and the test
  `test_facade_imports_nothing_it_does_not_use` keeps it that way. A test that needs an internal
  detail imports from the owning module, not from the facade.
- Only the local package `@local/graphite-report:1.0.0`'s public API is emitted.

## Invariants

- The tool is metrics-only: don't add dependencies on Kubernetes or cloud APIs.
- Unavailable, partial, approximate, and zero values are distinct.
- Prometheus query strategies prefer recording rules and keep raw-metric fallbacks.
- Every dynamic Typst value goes through `typst_string`, `typst_content`, or an existing safe
  helper.
- Package API incompatibilities are fixed in this renderer, not in `graphite-report`.
- Fake Prometheus scenarios validate end-to-end query behavior; `tests/golden/typst` protects
  the deterministic source output.
- **Reader-facing prose is written once, in final Portuguese (Brazil), by the code that produces
  it**: findings and recommendations in `analysis/`, capability reasons in
  `metrics/capabilities.py`, warnings and notices in `collectors/`, two HPA warnings in
  `models.py`. The mapper, the adapter, the renderer, and the frontend print it untouched; there
  is no translation table. Consumers own only labels for enums and identifiers. Exception
  messages and log lines are operator diagnostics and stay in English.
- The vocabulary is fixed: solicitação (request), limite (limit), carga de trabalho (workload),
  pool de nós (node pool, never "grupo de nós"), dimensionamento (right-sizing), margem
  (headroom), escalonamento automático (autoscaling), limitação de CPU (throttling), rótulo
  (label). Kubernetes nouns (`Deployment`, `StatefulSet`, `DaemonSet`, `Job`, `CronJob`, `Pod`,
  namespace, `HPA`, `OOMKill`), finding codes, capability identifiers, metric names, and route
  paths stay as they are. No em-dash inside reader-visible prose. The document does use the
  glyph alone as its empty-cell placeholder (`renderers/nodes.py`, `renderers/workloads.py`,
  `renderers/sections.py`); that is a table mark, not prose, and no guard scans for the
  character.
- `tests/test_vocabulary.py` enforces it on the Python side: every string literal in
  `collectors/`, `metrics/`, `analysis/`, and `models.py` is either clean reader text or listed
  in `not_reader_prose` with the reason it never reaches the report. A new literal must be
  classified. `frontend/tests/vocabulary.test.tsx` enforces it on the rendered tabs, including
  `aria-label`, `title`, and `placeholder`. A guard over rendered fixtures is blind to a shape
  the fixture lacks, so a layout or wording change is also probed on a real report.
- The node-pool sentinel `unknown` is never shown raw. The document prints it through
  `report-placeholder`; the frontend decides it only with the helpers of `format.ts`:
  `formatScopeName` and `formatPoolName` for the text, `isUnknownPool` where the markup itself
  changes (the findings detail pane replaces the copy-identifier control by plain text). That
  keeps the displayed text, the searched text, and the accessible name in agreement.
  "Indeterminado" is prose and never takes the identifier style.
- In Typst, identifiers go through `report-ident` (tagged `lang: "en"`) and prose placeholders
  through `report-placeholder` (document language). No prose inside `report-ident`.
- A stored snapshot is never rewritten. The PDF is compiled from the stored snapshot, which is
  never narrowed; filtering happens when the report is served. A report stored by an earlier
  version keeps the wording it was collected with.
- A workload with no observed consumption in the period doesn't appear in the report. This
  includes a `Deployment`, `StatefulSet`, or `DaemonSet` scaled to zero and a `CronJob` with no
  run in the window: since the report is a capacity survey based on actual resource usage, a
  workload at zero conveys nothing and only competes for the reader's attention. Aggregate
  namespace consumption continues to be counted normally.
