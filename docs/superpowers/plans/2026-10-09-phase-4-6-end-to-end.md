# Phase 4.6: End-to-End Security, Concurrency, and PDF Recovery Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** `make e2e` runs the four specs of spec 13.4 (main path, security, concurrency, PDF recovery) against the Compose stack, checks every fake Prometheus and the logs of the stack, and each new check is seen failing once.

**Architecture:** Three new Playwright specs join the three that exist. A second fake Prometheus serves the recovery spec's one collection, and profiles whose address nothing listens at keep reports in flight with no fake. The worker of the test stack runs a test image whose `typst` is a switch the Playwright container turns on and off through a shared volume. The Makefile checks each fake's count and searches the logs of the stack for every secret the run used.

**Tech Stack:** As in part 4.5, with no new dependency. Go 1.27 (standard library only for the two test programs); Playwright 1.63.0 in its pinned image; Docker Compose; GNU make with bash.

**Spec:** `docs/superpowers/specs/2026-09-27-k8s-capacity-report-design.md` as amended in `875caca` (approved by the owner on 2026-10-09): the new section 13.4.1 and the sub-bullets of 14.2. The owner's answers are in `.cache/phase-4/part-6/brainstorm-decisions.md` (1 to 5).

**Depends on:** `main` at `b5a00e8` (release 0.6.0). Branch `feat/end-to-end`, which holds the spec amendment `875caca`.

## Global Constraints

Every constraint of the plans of phases 0 to 4.5 stands, and `AGENTS.md` binds every task. The ones this part leans on most:

- **Every command runs inside `nix develop`** from the repository root. The host is NixOS: only `/bin/sh` and `/usr/bin/env` exist under `/bin` and `/usr/bin`.
- **A subagent never runs Docker**: `make image`, `make e2e`, and `make gate` are the main thread's. A subagent may run `go`, `make build`, `make test`, `make lint`, `make web`, and `npm` inside `web/`.
- **No test skips, no focused test, no retry**: Playwright keeps `retries: 0` and `workers: 1`; ESLint refuses `skip`, `only`, `fixme`.
- **Every page a spec shows is checked with axe** (`expectNoViolations`), and a policy violation fails the test unless the test is the one that injects it (R265).
- **Every word a spec asserts comes from `web/src/locales/en.json`** through the imported `en` object, never a literal; a sentence with a parameter is matched by a pattern built from its key.
- **The production binary holds no test support**: `cmd/typst-switch` joins the forbidden list of `internal/guard/binary_test.go`.
- **No change to the product**: no file under `internal/` other than `internal/guard`, no file under `web/src/`, and no file of `api/` changes in this part. A defect found in the product is a stop-and-report: it is recorded in `ROADMAP.md` and ruled by the main thread.
- **Comments say what the code does**; none names a task, step, ruling, brief, finding, or `.cache/` path.
- **Commits:** `type(scope): description`, imperative, under 50 characters (each subject below is counted); stage with `git add <paths>`.
- Scratch goes under `.cache/phase-4/part-6/`.

## Review Focus

Each line names a condition the spec implies and no requirement spells out, and what pins it.

1. **A spec that fails midway leaves the stack changed for the specs after it** (the switch on, `e2e-admin` demoted). Playwright runs files in name order: admin, concurrency, recovery, reports, security, sign-in, so a switch left on by the recovery spec fails the reports spec's PDF. Pinned by R264: every change to shared state is undone in a `finally`, and the reports spec reads its own row by id.
2. **A check that passes because the state it judges never happened.** Examples: the list's self-refresh when the report finished before the list loaded, or the CSRF healing when no token was ever refused. Pinned by asserting the precondition first: the row is seen queued or running (Task 6), and exactly one `403 CSRF_REJECTED` is seen before the success (Task 4).
3. **A secret that matches everything.** An empty line in the pattern list matches every log line, and a pattern with a regular-expression character matches the wrong text. Pinned in Task 3: patterns are fixed strings (`grep -F`), empty lines are removed, and the check fails when the specs recorded no secret.
4. **The test helper's own request rotates the CSRF token** (`GET /auth/session` rotates it). A helper call between the rotation and the write under test changes what the test measures. Pinned in Task 4: no helper call sits between tab B's load and tab A's write.
5. **Reports left in flight by the concurrency spec** (on unreachable profiles, retried for about 17 s) count toward the global admission limit of 20 for the next spec. At most 8 are in flight, so the next spec stays under the limit. The concurrency spec's own admission check counts by profile. Pinned by R266's numbers.

## Where this part sits

| Part | Delivers | Proof at the end |
|---|---|---|
| 4.1 to 4.5 | Foundation, report screens, calendar ranges, the report, administration | Done, released as 0.3.0 to 0.6.0 |
| **4.6 End-to-end (this plan)** | Security, concurrency, and PDF recovery specs; a second fake; the typst switch; the log check | `make e2e` runs the four specs of spec 13.4 twice in a row from a clean stack, each new check seen failing once; `make gate` passes |
| 5. Release 1.0.0 | Security review, operations guide | Tag after the gate |

### Out of this part

| Left out | Reason |
|---|---|
| Session expiry by time | The server's clock cannot be moved inside the stack; the platform tests prove it (spec 13.4.1) |
| The role check of every route | The platform tests walk the route table |
| The outbound policy beyond the profile tests | The admin spec covers a metadata address and an unreachable one |
| A worker stopped mid-job, reconcile | Need a container stopped mid-job or a River job removed behind the platform; the platform tests prove both |
| A hung compile and its deadline | The owner left it out (decision 4): the page shows the same failure as a refused compile |
| A report deleted while open in another tab | F-143's known behaviour; the spec asks nothing of it |

## How to read this plan

As in parts 4.2 to 4.5: each task gives exact paths, interfaces, and what each check asserts. The Go programs are written test first. A Playwright check can only fail in the stack, so its proof that it can fail is the mutation table of R267, run by the main thread.

Executors:
- **Subagent:** a Sonnet subagent in the main checkout, no worktree. It writes the change and runs `make test` and `make lint`; `make lint` runs `tsc -b` and ESLint over `web/`, `e2e/` included. It never runs Docker and cannot declare a task finished.
- **Main thread:** Docker (`make image`, `make e2e`, the mutation runs, `make gate`), judgment, the owner.

Rules for every brief:
- Tasks run in sequence; Tasks 3 to 6 share `web/e2e/fixtures.ts`, and Tasks 3 and 6 share the Compose files.
- Stage with `git add <paths>`; never stash, restore, check out a path, or clean.
- The brief appends this plan's rulings for its task, the Global Constraints, and the Review Focus rows its task owns.
- Before `make e2e`: `systemctl is-active docker` prints `active`, `docker context show` prints `default`, `docker info --format '{{.OperatingSystem}}'` starts with `NixOS`, and port 8080 is free (stop the look stack with `.cache/phase-4/part-4/look/stack/down.sh`). `make e2e` runs as a background task with its log under `.cache/phase-4/part-6/e2e/`.

**Order:** Task 1 → Task 2 → Task 3 → Task 4 → Task 5 → Task 6 → Task 7.

## Rulings made on the owner's behalf

Each ruling is a decision the amended spec leaves open: decision, grounds, cost if wrong. The numbering continues from R253.

| ID | Ruling |
|---|---|
| R254 | Two fakes. `fake-prometheus` serves the reports spec, and `fake-recovery` serves the recovery spec. Both serve `direct-complete` from the same image and the same mounted fixture. The concurrency spec's profiles point at `http://fake-prometheus:9091/api/v1`, where nothing listens; a collection there fails `ENDPOINT_UNREACHABLE`, which River retries up to `job_max_attempts` (3) — owner's decision 1 — costs one Compose service and one profile. |
| R255 | The fake answers `GET /-/ready` with 204 from `cmd/fake-prometheus`, outside `promtest`, so the route spends no recorded answer. A `--probe <url>` mode GETs the URL and exits 0 on a 2xx answer, 1 otherwise. The Compose healthcheck runs it every second, up to 30 times, and `serve` and `worker` wait for `service_healthy` on both fakes — F-137 (c) — costs one flag and one route. |
| R256 | The fake gains `--delay <duration>` (default 0), slept before each answer other than the ready route. `fake-recovery` runs `--delay 150ms`: a collection asks 69 of `direct-complete`'s 70 answers, about 10 s, so the list is loaded while the report is in flight. The list polls only while a loaded row is in flight (`web/src/features/reports/api.ts:26-30, 62-63`), so a collection that ends first leaves nothing to prove — spec 13.4.1's self-refresh line — costs about 10 s of the run. |
| R257 | The fake's build stage stops deriving from `build`. It copies `go.mod`, `go.sum`, `internal/promtest/`, and `cmd/fake-prometheus/`, then builds. A build of it with no cache then rebuilds neither the frontend nor the product binary — F-137 (b) — costs one stage rewritten. |
| R258 | `cmd/typst-switch` runs `/usr/local/bin/typst` with its arguments and its environment through `syscall.Exec`. When the first argument is `compile` and `/switch/fail` exists, it prints `typst-switch: compile refused by the switch` on standard error and exits 1. Both paths are constants: the worker runs typst with an empty environment (`internal/typst/compile.go:167`, `toolchain.go:55`), so nothing can be configured through the environment. A failure to stat the switch other than "does not exist" also exits 1 — spec 14.2 — costs nothing. |
| R259 | `deploy/worker-e2e.Containerfile` builds the switch in a `golang` stage, from the same pinned line as `deploy/Containerfile`, copying only `go.mod`, `go.sum`, and `cmd/typst-switch/`. It then copies the binary onto `FROM ${RUNTIME_IMAGE}`, whose value `make e2e` passes as `$(IMAGE)`. A guard test holds every `FROM golang:` line of the two Containerfiles equal. In Compose the worker runs `k8s-capacity-report-worker-e2e:${KCR_VERSION}`, with `KCR_TYPST=/usr/local/bin/typst-switch` and the named volume `switch` read-only at `/switch`. The Playwright service mounts `switch` read-write at `/switch` — spec 14.2 — costs one file and one guard test. |
| R260 | `make gate` runs `$(MAKE) e2e IMAGE_DEP= BUILD_FLAGS="--no-cache --pull"`. The e2e target builds the fake with `$(BUILD_FLAGS)` and the test worker with `$(filter-out --pull,$(BUILD_FLAGS))`, since its base is the local tag the gate just built — F-137 (b), spec 14.2 — costs a few minutes of the gate. |
| R261 | `deploy/platform/credentials/e2e-recovery` holds `e2e-Recovery-Credential-06` and one line break. `platform.toml` sets `credential_dir = "/etc/kcr/credentials"` and adds `fake-recovery` to `allowed_endpoint_hostnames`. `profiles.toml` adds `e2e-recovery`: display name `Recovery`, endpoint `http://fake-recovery:9090/api/v1`, type `prometheus`, cluster `prod`, namespaces `["payments"]`, `credential_file = "e2e-recovery"`, active. The recorded answers name no authorization, so the fake answers any. The worker reads the file and sends `Authorization: Bearer` (`internal/prom/client.go:401`) — owner's decision 2, item 7 — costs one file. |
| R262 | **Secrets.** `fixtures.ts` exports `secret(value)`, which appends the value and a line break to `/results/secrets.txt` and returns it. Every password literal under `web/e2e/` is declared through it. The Playwright command empties the file first (`: > /results/secrets.txt && npm ci && npx playwright test`); the container owns the file, so the host never deletes it. The Makefile tees the one-shot services' output into `.cache/e2e/one-shot.log`, and after the stack stops writes the logs of `serve` and `worker` and that file into `.cache/e2e/platform.log`. It builds `.cache/e2e/secret-patterns` from `secrets.txt`, `deploy/platform/e2e-admin-password`, `deploy/platform/credentials/e2e-recovery`, and the database password `kcr_e2e_only`, with empty lines removed. It fails when `secrets.txt` is empty or missing, or when `grep -qF -f secret-patterns platform.log` finds a match. It prints `e2e: a secret reached the logs of the stack` and never the secret — owner's decision 2, item 7 — costs one helper and a block of the recipe. |
| R263 | Names per spec: security `e2e-s-*`, concurrency `e2e-c-*`, recovery `e2e-r-*`. Users and profiles are created through the API with the administrator's session, as the admin spec does. Every password follows the existing pattern (`e2e-<Name>-Pass-NN!`, at least 12 characters), numbered from 07 upward in the order the tasks write them. |
| R264 | **Order independence.** The reports spec stops assuming an empty list: it finds its row by the report's id (the row whose link targets `/jobs/<id>`), and its delete step expects that row gone, not the empty text. The recovery spec turns the switch off in a `finally`. The concurrency spec restores `e2e-admin` to an active administrator in a `finally`. No spec creates an administrator that it leaves active. |
| R265 | **Security spec**, one `test()` per check, so a mutation run shows each failure. The six checks of spec 13.4.1 are written as Task 4 lists them. The policy check runs in its own browser context with its own `watchPolicy`, outside the `page` fixture's guard, and expects the violation. The literal CSP is the string of `internal/platform/api/headers.go:10`. |
| R266 | **Concurrency spec.** Profiles `e2e-c-replay` and `e2e-c-admission` are created through the API, both unreachable, `allowed_namespaces ["payments"]`, active. Replay: one lost answer and one double click give 2 reports in all on `e2e-c-replay`. Admission: 5 reports through the API on `e2e-c-admission`, each seen `queued` or `running`, then the sixth through the form is refused with `error.ADMISSION_PROFILE_LIMIT`. At most 8 reports are in flight (Review Focus 5). The race runs between `e2e-admin` and `e2e-c-admin`, two `PATCH /admin/users/{id}` with `{"role":"operator"}` sent through `Promise.all` from two browser contexts. Exactly one answers 200; the other answers 401 or 409 `LAST_ADMIN`, since the actor's role is not read again under the table lock (`internal/platform/auth/users.go:182-205`). One active administrator remains. |
| R267 | **Every new check is seen failing once,** by the mutation the table of Task 7 names, on a scratch copy kept under `.cache/phase-4/part-6/mutations/`, then reverted and the clean run repeated. A mutation of `internal/` or `web/src/` needs a mutated image: the main thread groups the server mutations into one image (M1) and the frontend mutations into another (M2), runs `make e2e` once on each, and reads each check's failure in the log. A check whose failure cannot be attributed to its own mutation in that run is run again with that mutation alone. The race check (R266) depends on timing: if M1 does not make it fail in three runs, the main thread records that, and the check stands as a property check. |
| R268 | **Close:** no version, no tag, no `CHANGELOG.md` entry, since users perceive nothing (AGENTS, "Changelog and version"); the closing report states the omission. `AGENTS.md` describes the two fakes, the switch, the credential, the log check, and the order rules. `ROADMAP.md` closes F-137. |

## Findings this plan closes

| Finding | How |
|---|---|
| F-137, the client-side navigation of step 5 | Task 6: the list refreshes itself while the recovery report collects; step 5 keeps proving the refetch on arrival |
| F-137, the gate's fake image built from cache | Task 3 (R257, R260) |
| F-137, the fake's healthcheck | Tasks 1 and 3 (R255) |

## File structure

| Path | Responsibility |
|---|---|
| `cmd/fake-prometheus/main.go`, `main_test.go` | The ready route, `--probe`, `--delay` (Task 1) |
| `cmd/typst-switch/main.go`, `main_test.go` | The switch (Task 2) |
| `internal/guard/binary_test.go`, `internal/guard/containerfile_test.go` | The forbidden list; the equal Go base (Task 2) |
| `deploy/Containerfile` | The fake's stage (Task 3) |
| `deploy/worker-e2e.Containerfile` | The test worker (Task 2) |
| `deploy/compose.integration.yaml` | `fake-recovery`, the healthchecks, the test worker, the switch volume, the Playwright command (Task 3) |
| `deploy/platform/{platform.toml,profiles.toml,credentials/e2e-recovery,README.md}` | The credential and the recovery profile (Task 3) |
| `Makefile` | The builds, the count of each fake, the one-shot log, the secret check, the gate's flags (Task 3) |
| `web/e2e/fixtures.ts` | `secret`, `csrfHeaders`, `createUser` with any role, `requestReport`, `previousWeek`, `switchCompiles`, `reportRow` (Tasks 3 to 6) |
| `web/e2e/{reports,admin,sign-in}.spec.ts` | Passwords through `secret`; the reports spec's order independence (Task 3) |
| `web/e2e/security.spec.ts` | Task 4 |
| `web/e2e/concurrency.spec.ts` | Task 5 |
| `web/e2e/recovery.spec.ts` | Task 6 |
| `AGENTS.md`, `ROADMAP.md` | Task 7 |

---

### Task 0: Open the part

**Executor:** Main thread.

- [x] **Step 1:** `ROADMAP.md` and `CHANGELOG.md` read and true (4.5 DONE, 0.6.0 released, `[Unreleased]` empty).
- [x] **Step 2:** The brainstorm (answers 1 to 5), the branch `feat/end-to-end` from `b5a00e8`, the spec amendment `875caca`, approved by the owner on 2026-10-09.
- [ ] **Step 3:** This plan committed; `ROADMAP.md`: a "Phase 4.6" section with P46-0 to P46-7 `OPEN`, the rulings R254 to R268 named, the plan's commit in P46-0, and the summary row of phase 4 naming 4.6 as open.

---

### Task 1: The fake: ready route, probe, delay (R255, R256)

**Executor:** Subagent.

**Files:** Modify `cmd/fake-prometheus/main.go`; create `cmd/fake-prometheus/main_test.go`.

**Interfaces:**
- Produces: the flags `--probe <url>` and `--delay <duration>`; the route `GET /-/ready` (204, empty body). Unchanged: `--fixture`, `--listen`, the `listening on` and `remaining: N` lines.
- Inside `main.go`: `func handler(server *promtest.Server, delay time.Duration) http.Handler` and `func probe(url string, stderr io.Writer) int`, so the tests reach them.

- [ ] **Step 1: Write the failing tests** in `cmd/fake-prometheus/main_test.go` (package `main`):

| Test | Input | Expected |
|---|---|---|
| `TestReadyRouteSpendsNothing` | `handler(promtest.NewServer(fixture with one POST /api/v1/query answer), 0)`, `GET /-/ready` through `httptest` | 204; `server.Remaining()` still 1; `server.Seen()` empty |
| `TestDelayHoldsEachAnswer` | `handler(server, 50*time.Millisecond)`, one form POST to `/api/v1/query` with `query=up` matching the answer | 200 with the recorded body; elapsed ≥ 50 ms; `Remaining()` 0 |
| `TestReadyIsNotDelayed` | `handler(server, 2*time.Second)`, `GET /-/ready` | 204 in under 1 s |
| `TestProbe` | `probe` against an `httptest` server answering 204; one answering 500; a closed address (`http://127.0.0.1:1/-/ready`) | 0; 1; 1. On 1, standard error holds one line starting `fake-prometheus: ` |
| `TestRunRefusesFixtureAndProbeTogether` | `run([]string{"--probe", "http://x/-/ready", "--fixture", "f.json"}, ...)` | 2, with a line on standard error |

A fixture is built in the test with `promtest.Fixture{Requests: []promtest.Request{{Method: "POST", Path: "/api/v1/query", QueryContains: "up", Body: json.RawMessage(`{"status":"success"}`)}}}`.

- [ ] **Step 2:** Run `go test ./cmd/fake-prometheus/` and see each test fail (undefined `handler` and `probe`).
- [ ] **Step 3: Implement.** In `run`, add the flags. `--probe` alone runs `probe` and returns its code. `--probe` with `--fixture` returns 2 with `fake-prometheus: --probe takes no other flag`. Serve `handler(server, *delay)` in place of `server`:

```go
// handler answers the ready route itself, so a health check spends no
// recorded answer, and holds every other answer for delay before the
// recorded server writes it.
func handler(server *promtest.Server, delay time.Duration) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /-/ready", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if delay > 0 {
			time.Sleep(delay)
		}
		server.ServeHTTP(w, r)
	})
	return mux
}

// probe asks url once and reports whether it answered with a 2xx status.
func probe(url string, stderr io.Writer) int {
	client := &http.Client{Timeout: 2 * time.Second}
	response, err := client.Get(url)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "fake-prometheus: %v\n", err)
		return 1
	}
	_ = response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode > 299 {
		_, _ = fmt.Fprintf(stderr, "fake-prometheus: %s answered %d\n", url, response.StatusCode)
		return 1
	}
	return 0
}
```

The package comment changes to say the command also stands in for a Prometheus-compatible API in the end-to-end stack.
- [ ] **Step 4:** `go test ./cmd/fake-prometheus/ ./internal/promtest/` passes; `make test` and `make lint` pass.
- [ ] **Step 5:** Commit `test(fake): add ready probe and answer delay` (43 characters): `git add cmd/fake-prometheus/main.go cmd/fake-prometheus/main_test.go`.

**Done when:** the five tests pass, and they fail on the code before the change.

---

### Task 2: The typst switch and the test worker image (R258, R259)

**Executor:** Subagent writes; the main thread builds the image.

**Files:** Create `cmd/typst-switch/main.go`, `cmd/typst-switch/main_test.go`, `deploy/worker-e2e.Containerfile`, `internal/guard/containerfile_test.go`; modify `internal/guard/binary_test.go`.

**Interfaces:**
- Produces: the binary `/usr/local/bin/typst-switch` in the image `k8s-capacity-report-worker-e2e:<version>`; the switch file `/switch/fail`.
- Inside `main.go`: `func run(args []string, stderr io.Writer, stat func(string) (fs.FileInfo, error), exec func(string, []string, []string) error) int`.

- [ ] **Step 1: Write the failing tests** in `cmd/typst-switch/main_test.go` (package `main`), with a `stat` that answers from a map and an `exec` that records its call and returns an error (a real `syscall.Exec` never returns on success):

| Test | Input | Expected |
|---|---|---|
| `TestVersionPassesThrough` | args `--version`; switch present | `exec` called with `/usr/local/bin/typst`, argv `[/usr/local/bin/typst --version]`; return 1 after the recorded error, with `typst-switch: ` on standard error |
| `TestCompilePassesWhenOff` | args `compile --root /r a.typ b.pdf`; `stat` answers `fs.ErrNotExist` | `exec` called with argv `[/usr/local/bin/typst compile --root /r a.typ b.pdf]` |
| `TestCompileRefusedWhenOn` | same args; `stat` answers a file | `exec` not called; return 1; standard error is exactly `typst-switch: compile refused by the switch\n` |
| `TestUnreadableSwitchRefuses` | same args; `stat` answers `fs.ErrPermission` | `exec` not called; return 1; standard error starts `typst-switch: ` |
| `TestNoArgumentsPassThrough` | args empty | `exec` called with argv `[/usr/local/bin/typst]` |

In `internal/guard/containerfile_test.go`, `TestContainerfilesShareTheGoBase` reads `deploy/Containerfile` and `deploy/worker-e2e.Containerfile` (paths from the repository root, as the other guard tests find it). It collects every line that starts with `FROM golang:`, cut at the first `AS` (`FROM golang:<tag>@sha256:<digest>`), and fails unless each file holds at least one and all are equal. Test rows: the two real files pass. A scratch pair written under `t.TempDir()` with a different digest fails; the check is factored as `func sameGoBase(files map[string]string) error` so the scratch pair is tested without the tree. A file with no `FROM golang:` line fails.

In `internal/guard/binary_test.go`, add `module + "/cmd/typst-switch"` to `forbidden`.

- [ ] **Step 2:** `go test ./cmd/typst-switch/ ./internal/guard/ -run 'Switch|Version|Compile|Unreadable|NoArguments|GoBase'` fails (undefined `run`, missing file).
- [ ] **Step 3: Implement** `cmd/typst-switch/main.go`:

```go
// Command typst-switch stands in for typst in the worker of the end-to-end
// stack. It runs the real typst with the same arguments and environment,
// unless the switch file exists, which makes every compile fail.
package main

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"syscall"
)

// The worker runs typst with an empty environment, so both paths are fixed.
const (
	realTypst  = "/usr/local/bin/typst"
	switchFile = "/switch/fail"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stderr, os.Stat, syscall.Exec))
}

func run(args []string, stderr io.Writer, stat func(string) (fs.FileInfo, error), exec func(string, []string, []string) error) int {
	if len(args) > 0 && args[0] == "compile" {
		_, err := stat(switchFile)
		if err == nil {
			_, _ = fmt.Fprintln(stderr, "typst-switch: compile refused by the switch")
			return 1
		}
		if !errors.Is(err, fs.ErrNotExist) {
			_, _ = fmt.Fprintf(stderr, "typst-switch: %v\n", err)
			return 1
		}
	}
	err := exec(realTypst, append([]string{realTypst}, args...), os.Environ())
	_, _ = fmt.Fprintf(stderr, "typst-switch: %v\n", err)
	return 1
}
```

`deploy/worker-e2e.Containerfile`:

```dockerfile
# The worker of the end-to-end stack: the runtime image named by
# RUNTIME_IMAGE, plus typst-switch, which the worker runs as typst. `make e2e`
# builds it; `make image` never does. The Go base equals the one of
# deploy/Containerfile, which a guard test checks.
ARG RUNTIME_IMAGE

FROM golang:1.27@sha256:3680233e3204827fbdc66088528ae6d4b3d034f51d03a99d454f6de034888244 AS switch
WORKDIR /src
COPY go.mod go.sum ./
COPY cmd/typst-switch/ ./cmd/typst-switch/
RUN CGO_ENABLED=0 go build -trimpath -o /out/typst-switch ./cmd/typst-switch

FROM ${RUNTIME_IMAGE}
COPY --from=switch /out/typst-switch /usr/local/bin/typst-switch
```

- [ ] **Step 4:** The tests pass; `make test` and `make lint` pass. Mutation: change one character of the digest in a scratch copy of the new Containerfile, run `TestContainerfilesShareTheGoBase` against it through `sameGoBase`, and see it fail. The table row covers this, so no tree edit is needed.
- [ ] **Step 5 (main thread):** `make image`, then `docker build --file deploy/worker-e2e.Containerfile --build-arg RUNTIME_IMAGE=k8s-capacity-report:$(cat VERSION) --tag k8s-capacity-report-worker-e2e:$(cat VERSION) .` succeeds. Then run each of these and see what it prints:
  - `docker run --rm --entrypoint /usr/local/bin/typst-switch k8s-capacity-report-worker-e2e:<v> --version` prints `typst 0.15.1 …`.
  - With a host folder `.cache/phase-4/part-6/switch/` holding an empty file `fail`, mounted at `/switch:ro`, `typst-switch compile x.typ y.pdf` prints `typst-switch: compile refused by the switch` and exits 1. The image has no shell, so the file is made on the host.
- [ ] **Step 6:** Commit `test(e2e): add the typst switch and its worker` (46 characters): `git add cmd/typst-switch/ deploy/worker-e2e.Containerfile internal/guard/containerfile_test.go internal/guard/binary_test.go`.

**Done when:** the Go tests pass and the image answers both runs of Step 5.

---

### Task 3: The stack, the checks of `make e2e`, and the order of the specs (R254 to R257, R260 to R264)

**Executor:** Subagent writes; the main thread runs `make e2e`.

**Files:** Modify `deploy/Containerfile`, `deploy/compose.integration.yaml`, `deploy/platform/platform.toml`, `deploy/platform/profiles.toml`, `deploy/platform/README.md`, `Makefile`, `web/e2e/fixtures.ts`, `web/e2e/reports.spec.ts`, `web/e2e/admin.spec.ts`, `web/e2e/sign-in.spec.ts`; create `deploy/platform/credentials/e2e-recovery`.

**Interfaces:**
- Produces, in `web/e2e/fixtures.ts`:
  - `secret(value: string): string` (R262)
  - `csrfHeaders(page: Page): Promise<{ "X-CSRF-Token": string }>` (exported; it exists)
  - `createUser(page, user, role: "administrator" | "operator" | "reader"): Promise<string>` (returns the new user's id, read from the 201 answer)
  - `requestReport(page: Page, body: Record<string, unknown>, key?: string): Promise<APIResponse>` (POST `/api/v1/reports` with `Idempotency-Key` = `key ?? crypto.randomUUID()` and the CSRF header)
  - `previousWeek(now: Date, zone: string): { start: string; end: string }` (the `mondayText` of `reports.spec.ts` moved here, weeks -1 and 0)
  - `reportRow(page: Page, id: string): Locator` (the row of the report list whose controls name the report's id; R264)
  - `switchCompiles(failing: boolean): Promise<void>` (writes or removes `/switch/fail`)
- Produces, in the stack: the services `fake-prometheus` and `fake-recovery`, both healthy only once they answer `/-/ready`; the worker on `k8s-capacity-report-worker-e2e`; the profile `e2e-recovery`.

- [ ] **Step 1: The fake's stage** in `deploy/Containerfile` (R257):

```dockerfile
# The fake Prometheus of the end-to-end stack, on the same base as the runtime
# image. It needs only its own package and promtest, so a build with no cache
# rebuilds neither the frontend nor the product. `make e2e` builds it;
# `make image` never does.
FROM golang:1.27@sha256:3680233e3204827fbdc66088528ae6d4b3d034f51d03a99d454f6de034888244 AS fake-prometheus-build
WORKDIR /src
COPY go.mod go.sum ./
COPY internal/promtest/ ./internal/promtest/
COPY cmd/fake-prometheus/ ./cmd/fake-prometheus/
RUN CGO_ENABLED=0 go build -trimpath -o /out/fake-prometheus ./cmd/fake-prometheus
```

Before relying on this, confirm that `internal/promtest` imports only the standard library (`go list -deps ./cmd/fake-prometheus | grep k8s-capacity-report` prints only `internal/promtest` and `cmd/fake-prometheus`). If it imports more, stop and report.

- [ ] **Step 2: Compose** (`deploy/compose.integration.yaml`):
  - A YAML anchor `x-fake` holds the fake's image, `read_only`, `security_opt`, `cap_drop`, the fixture volume, and a healthcheck: `["CMD", "/usr/local/bin/fake-prometheus", "--probe", "http://127.0.0.1:9090/-/ready"]`, interval 1s, timeout 3s, retries 30.
  - `fake-prometheus` keeps its command.
  - `fake-recovery` runs `["--fixture", "/fixtures/direct-complete.json", "--listen", ":9090", "--delay", "150ms"]`.
  - `serve` and `worker` depend on both fakes with `condition: service_healthy`.
  - `worker` gets `image: k8s-capacity-report-worker-e2e:${KCR_VERSION:?set KCR_VERSION}`, `environment: { KCR_TYPST: /usr/local/bin/typst-switch }` (merged with the DSN of `compose.yaml`), and the volume `switch:/switch:ro`.
  - `playwright` gets the volume `switch:/switch`, and its command becomes `["sh", "-c", ": > /results/secrets.txt && npm ci && npx playwright test"]`.
  - `volumes:` declares `switch:`.
  - After writing, run `KCR_VERSION=x KCR_CONFIG_DIR=$PWD/deploy/platform KCR_POSTGRES_PASSWORD=x docker compose -f deploy/compose.yaml -f deploy/compose.integration.yaml config`. This is the main thread's step, since it is Docker. The merged `worker.environment` holds both `KCR_DATABASE_DSN` and `KCR_TYPST`, and the merged `worker.volumes` holds both `/etc/kcr` and `/switch`.
- [ ] **Step 3: The platform files** (R261): the credential file, `credential_dir = "/etc/kcr/credentials"`, `allowed_endpoint_hostnames = ["fake-prometheus", "fake-recovery"]`, and the `[[profiles]]` table of `e2e-recovery`. `deploy/platform/README.md` adds one line each for the credential folder and the recovery profile.
- [ ] **Step 4: The Makefile** (R260, R262). Replace the `e2e` recipe and the gate's e2e line with these. Every line that pipes runs under `set -o pipefail`, as the gate's lines do.

```make
# The end-to-end stack: PostgreSQL, the schema, the first administrator, the
# profiles, then serve, worker, two fake Prometheus servers, and the Playwright
# container, whose exit status is the target's. The worker runs the test image
# whose typst the specs can make fail. Each fake stops with one answer unasked,
# as a whole run of direct-complete does in internal/goldentest, and no log of
# the stack holds a secret the run used. The stack is removed with its volumes
# first and last.
E2E_COMPOSE := docker compose -f deploy/compose.yaml -f deploy/compose.integration.yaml
E2E_FAKES := fake-prometheus fake-recovery

e2e: $(IMAGE_DEP)
	docker build $(BUILD_FLAGS) --file deploy/Containerfile --target fake-prometheus --tag k8s-capacity-report-fake-prometheus:$(VERSION) .
	docker build $(filter-out --pull,$(BUILD_FLAGS)) --file deploy/worker-e2e.Containerfile --build-arg RUNTIME_IMAGE=$(IMAGE) --tag k8s-capacity-report-worker-e2e:$(VERSION) .
	@set -o pipefail; \
	export KCR_VERSION=$(VERSION) KCR_CONFIG_DIR=$(CURDIR)/deploy/platform KCR_POSTGRES_PASSWORD=kcr_e2e_only; \
	mkdir -p .cache/e2e; : > .cache/e2e/one-shot.log; \
	$(E2E_COMPOSE) down --volumes; \
	$(E2E_COMPOSE) up -d --wait postgres \
		&& $(E2E_COMPOSE) run --rm -T migrate 2>&1 | tee -a .cache/e2e/one-shot.log \
		&& $(E2E_COMPOSE) run --rm -T bootstrap < deploy/platform/e2e-admin-password 2>&1 | tee -a .cache/e2e/one-shot.log \
		&& $(E2E_COMPOSE) run --rm -T profiles 2>&1 | tee -a .cache/e2e/one-shot.log \
		&& $(E2E_COMPOSE) up --abort-on-container-exit --exit-code-from playwright serve worker playwright; \
	status=$$?; \
	$(E2E_COMPOSE) stop $(E2E_FAKES); \
	for fake in $(E2E_FAKES); do \
		$(E2E_COMPOSE) logs --no-log-prefix $$fake | grep -qx 'remaining: 1' \
			|| { echo "e2e: $$fake did not leave exactly the one unasked answer; its last lines:"; \
			$(E2E_COMPOSE) logs --no-log-prefix --tail 5 $$fake; [ $$status -ne 0 ] || status=1; }; \
	done; \
	$(E2E_COMPOSE) logs --no-log-prefix serve worker > .cache/e2e/platform.log; \
	cat .cache/e2e/one-shot.log >> .cache/e2e/platform.log; \
	if [ ! -s .cache/e2e/secrets.txt ]; then \
		echo 'e2e: the specs recorded no secret'; [ $$status -ne 0 ] || status=1; \
	fi; \
	cat .cache/e2e/secrets.txt deploy/platform/e2e-admin-password deploy/platform/credentials/e2e-recovery 2>/dev/null \
		| { cat; echo kcr_e2e_only; } | grep -v '^$$' > .cache/e2e/secret-patterns; \
	if grep -qF -f .cache/e2e/secret-patterns .cache/e2e/platform.log; then \
		echo 'e2e: a secret reached the logs of the stack'; [ $$status -ne 0 ] || status=1; \
	fi; \
	$(E2E_COMPOSE) down --volumes; exit $$status
```

The gate's line becomes `@set -o pipefail; $(MAKE) e2e IMAGE_DEP= BUILD_FLAGS="--no-cache --pull" 2>&1 | tee $(GATE_LOGS)/e2e.log`.
- [ ] **Step 5: The fixtures** (`web/e2e/fixtures.ts`). Add the helpers of the Interfaces block:
  - `secret` uses `appendFileSync` from `node:fs`. `admin.password` becomes `secret("e2e-Fixture-Pass-01!")`.
  - `switchCompiles(true)` writes an empty `/switch/fail`, and `switchCompiles(false)` removes it with `{ force: true }`.
  - `reportRow`: a row's link targets `/jobs/<id>` (`web/src/features/reports/reports-page.tsx:263`), so the locator is `page.getByRole("row").filter({ has: page.locator(`a[href="/jobs/${id}"]`) })`.
  - `createUser` reads `{ id }` from the 201 answer and returns it.
- [ ] **Step 6: The existing specs** (R262, R264):
  - Every password literal of `reports.spec.ts`, `admin.spec.ts` (including the 20 fit users and `newUser`), and `sign-in.spec.ts` (`password`, `newPassword`) is declared through `secret(...)`.
  - `reports.spec.ts`:
    - It imports `previousWeek` and drops its own `mondayText`.
    - The first step stops expecting `en.reports.empty`.
    - "the list shows the report without a reload" and the reader's list check use `reportRow(page, id)` instead of filtering rows by the collected text.
    - The delete step expects `reportRow(page, id)` to have count 0, not the empty text.
    - Its comment about the one collection stays true: it still runs the one collection of `fake-prometheus`.
  - The subagent reads `admin.spec.ts` and `sign-in.spec.ts` for any assertion that counts a shared list or assumes an empty one, and lists what it found. It changes only what R264 names; anything else is a stop-and-report.
- [ ] **Step 7:** `make lint` passes (ESLint and tsc over `web/`, including `e2e/`); `make test` passes.
- [ ] **Step 8 (main thread):** The Docker checks, then `make e2e`:
  - It passes, and its log shows the two builds.
  - `serve` and `worker` start only after both fakes are healthy, read from the order of the Compose lines.
  - `remaining: 1` holds for `fake-prometheus`.
  - `fake-recovery` has no spec yet, so it leaves `remaining: 70`, and the target fails on it. This is the expected first failure: it proves the loop reads each fake.
  - Record that run, then run with `E2E_FAKES=fake-prometheus` on the command line to see the rest green.
  - `.cache/e2e/secrets.txt` holds the recorded passwords, and the secret check finds nothing.
- [ ] **Step 9:** Commit `test(e2e): check every fake and the stack's logs` (47 characters): `git add Makefile deploy/Containerfile deploy/compose.integration.yaml deploy/platform/ web/e2e/fixtures.ts web/e2e/reports.spec.ts web/e2e/admin.spec.ts web/e2e/sign-in.spec.ts`.

**Done when:** `make e2e` with `E2E_FAKES=fake-prometheus` passes, and the full run fails only on `fake-recovery`'s count. Task 6 closes that.

---

### Task 4: The security spec (R263, R265)

**Executor:** Subagent writes; the main thread runs `make e2e` (with `E2E_FAKES=fake-prometheus` until Task 6 lands).

**Files:** Create `web/e2e/security.spec.ts`.

**Interfaces:** Consumes `admin`, `signIn`, `createUser`, `csrfHeaders`, `secret`, `watchPolicy`, `expectNoViolations`, `test`, `expect` from `fixtures.ts`.

Each check is its own `test()`, and each creates its own user through the administrator's session in a page of its own. Users: `e2e-s-ended` (operator), `e2e-s-deactivated` (operator), `e2e-s-role` (operator), `e2e-s-csrf` (operator), `e2e-s-policy` (reader), `e2e-s-throttle` (reader). Passwords run `e2e-S-Ended-Pass-07!` to `e2e-S-Throttle-Pass-12!` through `secret`. Signing in on an open login page fills `en.login.username` and `en.login.password` and clicks `en.login.submit`, as `signIn` does without its `goto`.

1. **`a session ended in another tab sends the page to sign-in and back`.**
   - Tab A signs in at `/account`.
   - Tab B (`context.newPage()`, the same context) opens `/reports` and clicks `en.header.signOut`.
   - Tab A clicks the navigation link `en.nav.reports`. It expects the URL `/login?next=%2Freports&ended=1` (match `next` and `ended` as query parameters, not the order) and the text `en.login.sessionEnded`, then runs axe.
   - Tab A signs in. It expects the URL `/reports` and the heading `en.reports.title`.
2. **`a deactivated user is signed out and cannot sign in`.**
   - The operator signs in at `/reports` in a context of their own.
   - The administrator sends `PATCH /api/v1/admin/users/{id}` `{"active": false}`, and it answers 200.
   - The operator clicks `en.nav.account`. They land on sign-in with `en.login.sessionEnded`.
   - Signing in again shows `en.error.INVALID_CREDENTIALS`. Axe runs on both screens.
3. **`a role change ends the session and the new role holds`.**
   - The operator signs in at `/reports` and sees the link `en.nav.generate`.
   - The administrator sends `PATCH {"role": "reader"}` (200).
   - The operator clicks `en.nav.account`. They land on sign-in with `en.login.sessionEnded`, sign in, and return to `/account`.
   - `/reports` shows no `en.nav.generate` link, and `/generate` shows the heading `en.notFound.title`.
4. **`a write without the CSRF token is refused, and a rotated token heals`.**
   - (a) In the operator's page, `page.request.post("/api/v1/auth/logout")` with no header answers 403 with `code` `CSRF_REJECTED`. `GET /api/v1/auth/session` then answers `authenticated: true`.
   - (b) Tab A is on `/account`, and a response listener counts the answers to `PATCH` requests under `/api/v1/account`. The subagent reads `web/src/` for the exact path of the language change and uses it.
   - Tab B in the same context opens `/reports`. Loading the app reads the session, which rotates the token, and nothing else runs between this load and tab A's write.
   - Tab A switches the language to pt-BR through the header menu, as `sign-in.spec.ts` does. It expects:
     - the listener saw exactly one 403 whose body `code` is `CSRF_REJECTED`, then one 200, on the same URL;
     - the page shows the pt-BR heading of the account screen.
   - The language goes back to English the same way. The second switch may heal again, so it is not counted.
5. **`an injected inline script is blocked, and the headers and the cookie hold`.**
   - A new context and page with its own `violations = await watchPolicy(page)`. This test does not use the `page` fixture, so the guard of `fixtures.ts` does not run on it. The reader signs in at `/account`.
   - `page.evaluate` appends `<script>window.kcrInjected = true</script>` to `document.body`, created with `textContent`.
   - It expects:
     - `await page.evaluate(() => "kcrInjected" in window)` is false;
     - `violations` has exactly one entry, which contains `script-src`.
   - Headers: `page.request.get("/")`, `page.request.get("/api/v1/auth/session")`, and `page.request.get(<asset>)` each carry:
     - `content-security-policy` equal to `default-src 'self'; script-src 'self'; style-src 'self'; font-src 'self'; img-src 'self' data:; connect-src 'self'; object-src 'none'; base-uri 'none'; frame-ancestors 'none'; form-action 'self'`;
     - `x-content-type-options: nosniff`;
     - `referrer-policy: same-origin`;
     - `x-frame-options: DENY`.
   - `<asset>` is the first `src="/assets/…js"` read from the body of `/`.
   - If the asset lacks a header, the subagent does not weaken the test: it reports, and the main thread rules (spec 11.12).
   - Cookie: `context.cookies()` holds `kcr_session` with `httpOnly: true`, `secure: true`, `sameSite: "Lax"`, `path: "/api"`.
6. **`five wrong passwords block the sixth sign-in`.**
   - In a new context, five sign-ins with `wrong-Password-00!` (also declared through `secret`) each show `en.error.INVALID_CREDENTIALS`.
   - The sixth, with the right password, shows the text of `en.error.AUTH_RATE_LIMITED`, matched as a pattern with `{{seconds}}` read as `\d+`.
   - The page stays on sign-in. Axe runs.

- [ ] **Step 1:** Write the spec as above. `make lint` passes.
- [ ] **Step 2 (main thread):** `make e2e` passes with `E2E_FAKES=fake-prometheus`. The six test names appear with `✓` in the log, and so do the three of the specs that exist. Then a second run in a row.
- [ ] **Step 3:** Commit `test(e2e): prove the security of the platform` (45 characters): `git add web/e2e/security.spec.ts web/e2e/fixtures.ts`.

**Done when:** two runs in a row pass. The mutations are run in Task 7.

---

### Task 5: The concurrency spec (R263, R266)

**Executor:** Subagent writes; the main thread runs `make e2e`.

**Files:** Create `web/e2e/concurrency.spec.ts`; `fixtures.ts` only if a helper is missing.

**Interfaces:** Consumes `admin`, `signIn`, `createUser`, `createProfile`, `csrfHeaders`, `requestReport`, `secret`, `expectNoViolations`, `test`, `expect`.

Setup, in the administrator's page:
- Operator `e2e-c-operator`, password `e2e-C-Operator-Pass-13!`.
- Administrator `e2e-c-admin`, password `e2e-C-Admin-Pass-14!`.
- Profiles `e2e-c-replay` (display name `Replay`) and `e2e-c-admission` (display name `Admission`): endpoint `http://fake-prometheus:9091/api/v1`, type `prometheus`, cluster `prod`, `allowed_namespaces: ["payments"]`, active.

Each check is its own `test()`. The setup runs in a `test.beforeAll` that signs the administrator in, through a page of its own. It is idempotent per run because the stack is fresh.

1. **`a lost answer and a double click make one report each`.**
   - The operator opens `/generate`, picks `Replay`, the namespace `payments`, and English. The period stays the profile's default.
   - A `page.route` on `**/api/v1/reports` lets the first `POST` through with `route.fetch()`, records the answer's `id`, then calls `route.abort("failed")`.
   - The form shows an alert holding `en.error.network`. After `page.unroute`, the operator clicks `en.generate.submit` again. The URL becomes `/jobs/<recorded id>`.
   - Then `/generate` again, the same choices, and `dblclick` on `en.generate.submit`. The URL becomes `/jobs/<id>` with a new id.
   - `GET /api/v1/reports` (all pages, following `next_cursor`) holds exactly 2 items whose `profile_id` is `e2e-c-replay`.
2. **`a request beyond the admission limit of a profile is refused`.**
   - Five `requestReport` calls on `e2e-c-admission` answer 202. Each body: `profile_id`, `language: "en"`, `namespaces: ["payments"]`, and no period, so the profile's default applies (`internal/platform/profiles/document.go:97`). If the API refuses a body with no period, the subagent stops and reports.
   - Precondition: `GET /api/v1/reports` shows those five with `status` `queued` or `running`.
   - The operator sends a sixth through the form on `Admission`. The form's alert shows `en.error.ADMISSION_PROFILE_LIMIT`, matched as a pattern with `{{seconds}}` read as `\d+`. The URL stays `/generate`. Axe runs.
3. **`a second save of one profile is refused as stale`.**
   - The administrator opens `/profiles/e2e-c-replay/edit` in tab A and in tab B (same context).
   - Tab A sets `en.admin.profile.name` to `Replay A` and saves. The URL becomes `/profiles?open=e2e-c-replay`.
   - Tab B sets the name to `Replay B` and saves. Tab B shows `en.admin.profile.stale` and the button `en.admin.profile.loadLatest`, and its name field still holds `Replay B`. Axe runs.
4. **`two administrators lowering each other leave one`.**
   - Read the ids of `e2e-admin` and `e2e-c-admin` from `GET /api/v1/admin/users`.
   - Precondition: exactly two users are active administrators.
   - Sign `e2e-c-admin` in, in a second context. Take each side's CSRF header first, since `csrfHeaders` itself reads the session.
   - `Promise.all` sends the two `PATCH {"role": "operator"}` requests, each lowering the other.
   - It expects:
     - exactly one answer is 200;
     - the other is 401, or 409 with `code` `LAST_ADMIN`;
     - `GET /api/v1/admin/users`, read with the session of whichever administrator stayed, shows exactly one active administrator.
   - `finally` (R264): sign in with whichever administrator stayed.
     - If `e2e-admin` was lowered, `e2e-c-admin` raises it back to `administrator` and `e2e-admin` signs in again.
     - Then `e2e-admin` deactivates `e2e-c-admin`.
     - It ends by asserting that `e2e-admin` is the one active administrator.

- [ ] **Step 1:** Write the spec. `make lint` passes.
- [ ] **Step 2 (main thread):** `make e2e` (`E2E_FAKES=fake-prometheus`) passes twice in a row. The four tests show `✓`, and the reports and security specs, which run after this one, still pass.
- [ ] **Step 3:** Commit `test(e2e): prove requests that race` (35 characters): `git add web/e2e/concurrency.spec.ts web/e2e/fixtures.ts`.

**Done when:** two runs in a row pass.

---

### Task 6: The PDF recovery spec (R256, R258, R263, R264)

**Executor:** Subagent writes; the main thread runs `make e2e`.

**Files:** Create `web/e2e/recovery.spec.ts`.

**Interfaces:** Consumes `admin`, `signIn`, `createUser`, `requestReport`, `previousWeek`, `reportRow`, `switchCompiles`, `secret`, `expectNoViolations`, `test`, `expect`.

One `test()` with steps, since each step needs the report the one before made. The whole body sits in `try { … } finally { await switchCompiles(false); }`. Operator `e2e-r-operator`, password `e2e-R-Operator-Pass-15!`. This spec runs the one collection of `fake-recovery`, and its comment says so, as the reports spec says it of its own.

1. **`the list shows the report collected with no reload`.**
   - The administrator creates the operator. `switchCompiles(true)`.
   - The operator signs in at `/reports`, then sends `requestReport` with `profile_id: "e2e-recovery"`, `language: "en"`, `namespaces: ["payments"]`, `timezone: "America/Sao_Paulo"`, and `start` and `end` from `previousWeek(new Date(), "America/Sao_Paulo")`. That is the selection of the reports spec, which leaves `remaining: 1`. It reads the `id`.
   - The operator calls `page.reload()` once, to load the list with the new row. This is the last navigation of the step.
   - Precondition: `reportRow(page, id)` shows `en.reportStatus.queued` or `en.reportStatus.running`.
   - Then, with no navigation, the row shows `en.reportStatus.collected` within 60 s, and its PDF cell shows `en.pdfStatus.failed`.
   - A `page.on("framenavigated")` counter on the main frame stays at 0 from the precondition to the end of the step.
2. **`a failed PDF is made again once the cause is gone`.**
   - The operator opens `/jobs/<id>`. The group `en.language.en` shows `en.pdfStatus.failed`, the text `en.pdfFailure.PDF_COMPILE_FAILED`, and the button `en.progress.retryPdf`. Axe runs.
   - `switchCompiles(false)`. Click `en.progress.retryPdf`. The group shows `en.pdfStatus.ready` within 60 s.
   - The link `en.progress.download` downloads bytes starting `%PDF-`.
3. **`a failed PDF of the second language leaves the first`.**
   - `switchCompiles(true)`. In the group `en.language["pt-BR"]`, click `en.progress.generatePdf`. The group shows `en.pdfStatus.failed`.
   - The English group still shows `en.pdfStatus.ready`, and its download still starts `%PDF-`.
4. **`two requests for one PDF make one`.**
   - `switchCompiles(false)`. A second tab in the same context opens `/jobs/<id>`. Both show the pt-BR `en.progress.retryPdf`.
   - `Promise.all` clicks it in both tabs. Both tabs show the pt-BR `en.pdfStatus.ready` within 60 s, and neither shows an alert (`getByRole("alert")` count 0 inside `#main`).
   - `GET /api/v1/reports/<id>` holds exactly two `pdfs`, `en` and `pt-BR`, both `ready`, with no `failure_code`.
   - The pt-BR download starts `%PDF-`. Axe runs in both tabs.

- [ ] **Step 1:** Write the spec. `make lint` passes.
- [ ] **Step 2 (main thread):** `make e2e` with both fakes passes twice in a row, from a clean stack. Both fakes log `remaining: 1`. The secret check finds nothing, and `e2e-Recovery-Credential-06` is among the patterns.
- [ ] **Step 3:** Commit `test(e2e): prove the recovery of a failed PDF` (44 characters): `git add web/e2e/recovery.spec.ts web/e2e/fixtures.ts`.

**Done when:** two full runs in a row pass with both fakes.

---

### Task 7: Seen failing, combined run, documents, review, gate, close (R267, R268)

**Executor:** Main thread.

- [ ] **Step 1: Seen failing (R267).** Each mutation is made on a scratch copy kept under `.cache/phase-4/part-6/mutations/`, run, and reverted. Each run's log goes beside it. The expected failure is read from the log.

| Check | Mutation | Run | Expected failure |
|---|---|---|---|
| Each fake's count | Expect `remaining: 2` for `fake-recovery` only | Makefile copy | The loop names `fake-recovery` |
| The fake's healthcheck | Probe `/-/nothing` | Compose copy | The stack never starts `serve` |
| No secret recorded | Playwright command without the specs (`npx playwright test --list`) | Compose copy | `e2e: the specs recorded no secret` |
| A secret in the logs | Add `listening` to the patterns | Makefile copy | `e2e: a secret reached the logs of the stack` |
| Ended session (S1) | The client's 401 handler stops navigating (`web/src/api/client.ts`) | M2 | S1 fails waiting for sign-in |
| CSRF heals (S4b) | No retry on `CSRF_REJECTED` | M2 | S4b fails on the language |
| Lost answer (C1) | The form renews its key on every submit | M2 | C1 counts 3 reports |
| Double click (C1) | The form's send lock removed, with the renewal | M2 | C1 counts more than 2 |
| List self-refresh (R1) | `refetchInterval` returns false | M2 | R1 times out on `collected` |
| Deactivated (S2) | The session check ignores `active` | M1 | S2 fails waiting for sign-in |
| Role change (S3) | A role change revokes no session | M1 | S3 fails waiting for sign-in |
| CSRF refused (S4a) | `VerifyCSRF` returns nil | M1 | S4a sees 200 |
| CSP enforced (S5) | `script-src 'self' 'unsafe-inline'` | M1 | S5: the script ran; the header differs |
| Headers (S5) | `X-Frame-Options` not set | M1 | S5 header assertion |
| Throttle (S6) | `ThrottleMaxFailures = 50` | M1 | S6 sees `INVALID_CREDENTIALS` on the sixth |
| Stale (C3) | The revise skips the base revision check | M1 | C3 sees no stale alert |
| Race (C4) | `checkLastAdministrator` returns nil | M1 | C4 sees two 200s or no administrator (up to three runs; R267) |
| Admission (C2) | `admission_profile_limit = 10` | `platform.toml` copy | C2 sees a 202 |
| Failed PDF (R2) | The worker without `KCR_TYPST` | Compose copy | R1 or R2 sees no failure |
| Second language (R3) | The render writes its outcome to the `en` row whatever its language | M1 | R3 sees the English PDF failed |
| Two at once (R4) | `POST /reports/{id}/pdfs` on a pending row answers 500 | M1 | R4 sees an alert |

M1 and M2 are two scratch branches. Each holds its mutations as one commit, never merged; an image is built from each with `make image`, then `make e2e`. Both branches are deleted after their runs. A mutation that turns out wrong (the check passed) is a defect of the check: fix the check, not the mutation, and rerun.
- [ ] **Step 2: Combined run** over the branch: `make test`, `make lint`, `make generate-check`, `make test-postgres`, `make image`, `make check-runtime`, `make e2e`, with logs under `.cache/phase-4/part-6/combined/`.
- [ ] **Step 3: `AGENTS.md`.**
  - "The interface", the `make e2e` bullet: the two fakes and R256's delay; the test worker and its switch; the credential profile; the log check; the four specs and their name prefixes; the order rule (R264).
  - "Rules enforced by a guard": the forbidden list names `cmd/typst-switch`, and the Go base of both Containerfiles is held equal.
  - "Delegating to subagents" is unchanged.
- [ ] **Step 4: `CHANGELOG.md`** gets no entry (R268). The closing report says so.
- [ ] **Step 5: Adversarial review of the branch**, by areas, with probes under `.cache/phase-4/part-6/review/`:
  - (a) the two Go programs and the guard;
  - (b) the Makefile, Compose, and the platform files, including that a secret cannot be printed and that the gate builds with no cache;
  - (c) the three specs against spec 13.4.1, checking preconditions, vacuous passes, and order independence.

  Then one fix wave and one re-review.
- [ ] **Step 6:** `make gate` with logs under `.cache/phase-4/part-6/gate/`. The gate's e2e log shows the fake's `RUN go build` not `CACHED`.
- [ ] **Step 7: Close.**
  - `ROADMAP.md`: the P46 items with their commits, F-137 `DONE`, every finding of the part recorded.
  - The closing report goes to `.cache/phase-4/part-6/closing-report.md`.
  - The owner's word on the merge of `feat/end-to-end` into `main` by fast-forward, and on the push, each naming its target. No version and no tag (R268).

**Done when:** every check has been seen failing, the gate passes from images built without cache, the review is clean, and the owner has the closing report.
