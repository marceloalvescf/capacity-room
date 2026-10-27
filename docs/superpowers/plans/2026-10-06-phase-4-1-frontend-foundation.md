# Phase 4.1: The Frontend Foundation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** A reader opens `serve` in a browser, signs in, changes their language and password, switches between light and dark, and signs out, in Portuguese (Brazil) or English. The interface is built into the one binary and proven end to end by Playwright against the Compose stack. Before that, `serve` stops keeping a large row's buffer on idle database connections (F-127).

**Architecture:** `web/` holds a Vite, React, and TypeScript application and a small Go package `web` that embeds its build. `serve` hands every unmatched `GET` or `HEAD` outside `/api/` and `/health/` to that package, which answers a file of the build or `index.html`. The application speaks to the API through a client generated from `api/openapi.yaml`. It holds the session in one TanStack Query entry, words every text through react-i18next locale files, and draws its colours from token pairs that a test checks for contrast. `deploy/compose.yaml` runs the platform. `deploy/compose.integration.yaml` adds a Playwright service that shares the network of `serve`.

**Tech Stack:**
- **Backend:** Go 1.27 with the modules already in `go.mod`; no new Go module.
- **Runtime:** Node 26 (native 26.10.0; image `node:26-trixie-slim`) and npm with a lockfile.
- **Application:** React 19.3.0, React Router 8.4.0, TanStack Query 5.104.1, React Hook Form 7.89.0 with Zod 4.6.5, Radix UI 1.7.0, Tailwind CSS 4.3.3, Phosphor icons 2.1.10, react-i18next 17.0.16 with i18next 26.4.2.
- **Build and lint:** Vite 8.3.3, TypeScript 5.9.3 (R165), ESLint 9.39.5.
- **Tests:** Vitest 5.0.3 with jsdom, Testing Library, axe-core 4.14.0, and MSW 3.0.2. Playwright 1.63.0 with `@axe-core/playwright` 4.13.0.
- **Generated client:** `openapi-typescript` 7.13.0 and `openapi-fetch` 0.17.0.
- **Database:** PostgreSQL 18 through the harness of 3.1.

**Spec:** `docs/superpowers/specs/2026-09-27-k8s-capacity-report-design.md`, sections 2, 4.1, 11.3, 11.4, 11.8, 11.10, 11.12, 12 (all of it, as amended in `82b070e`), 13.1, 13.4, 13.5, 14, 15, 16. The original frontend, read-only, is `older/kubernetes-report/frontend/`; its stack and patterns are summarised in `docs/reference/inventory/document-frontend.md`. The measurement behind F-127 is in `.cache/phase-3/part-4/review/rereview.md` ("E182 (download slots)").

**Depends on:** Release 0.2.0, `main` at `855a93d`, tagged `v0.2.0` and pushed on 2026-10-06.

## Where this part sits

| Part | Delivers | Proof at the end |
|---|---|---|
| **4.1 Foundation (this plan)** | F-127; `web/` with its toolchain and generated client; the build embedded and served by `serve`; theme, fonts, i18n, and the session; the shell, sign-in, account, and not-found screens; Compose files; Playwright; `make web` and `make e2e` in the gate | In a real browser against the real image: sign in, switch to Portuguese and back, change the password, sign out by it, sign in with the new password, sign out |
| 4.2 Reports | Report list, new report form, progress, PDF downloads and retry, delete; the fake Prometheus and fixture profiles in the integration stack | A report requested, followed, and downloaded from the browser |
| 4.3 The report | The seven tabs of the report view | Every tab of a golden scenario in both languages |
| 4.4 Administration | Profiles and users | An administrator creates a profile and a user from the browser |
| 4.5 End-to-end | Security, concurrency, and PDF recovery specs | `make e2e` runs the four specs of spec 13.4 |

### Out of this part

| Left out | Arrives in | Reason |
|---|---|---|
| Every screen but sign-in, account, and not found | 4.2 to 4.4 | The split the owner chose on 2026-10-06 |
| TanStack Table, the Radix Select, and the style hash of the Select's viewport | 4.2 or 4.3, with the first screen that needs them | R172: no screen of 4.1 uses them, and a style hash enters the CSP only with the style it allows |
| The fake Prometheus, fixture profiles, and the compile-failure switch in the integration stack | 4.2 and 4.5 | Nothing in 4.1 requests a report |
| A TLS proxy in Compose | none | The owner's ruling of 2026-10-06: the operator's proxy fronts `serve` |
| F-128 and F-129 | not this part | Minors with no phase; Task 1 does not change the shutdown or the download deadline |
| F-123 | none | Deferred |

## How to read this plan

The owner's orchestration rule applies. The plan states what each task must deliver and how it is proven, and the implementer writes the code. Each task gives exact paths, exact signatures, and a table of tests with inputs and expected results. Write each test first, watch it fail, then implement.

The plan itself carries every word on screen (Task 6, "The wording") and every colour (Task 6, "The tokens"). An implementer transcribes them and never invents one. A text or a colour the implementer finds missing is a stop-and-report, because wording and design stay on the main thread.

Executors:

- **Subagent:** a Sonnet subagent. It may run:
  - `go`, `make build`, `make test`, `make lint`, `make generate`, `make generate-check`, and `make web`;
  - `npm` inside `web/` (`npm ci`, `npm run …`);
  - the tagged Go tests against the PostgreSQL the main thread started (`KCR_TEST_DATABASE_DSN`).

  It never starts or stops Docker, never runs `make e2e` or `make update-*`, and cannot declare a task finished. A first `npm ci` needs the network, which this approval grants for the registry `registry.npmjs.org` only.
- **Main thread:** needs Docker, judgment, API design, wording, design, or the owner. Docker is OrbStack's engine (`docker context show` prints `orbstack`).

Rules for every subagent brief, as in the plans of phase 3:
- **Order and isolation:**
  - Tasks run in sequence unless this plan says they may run side by side.
  - A reviewer reads a diff file and runs tests only on packages nobody edits.
- **Git:**
  - Stage with `git add <paths>`.
  - Never stash, restore, check out a path, or clean.
- **Tests:** a test of a change goes red first, test by test, on the code before the change.
- **Briefs:**
  - A brief that says "report every X" lists what must not match.
  - A decision flagged at the end of a report has already been made.
  - The main thread commits before dispatching and computes before dispatching.

The rules learned through 3.4 bind every task:

- **Check a plan's signatures against AGENTS.md before cutting briefs.**
- **A ruling made on an implementer's report goes back to the implementer with a fix request before the review.**
- **The harness creates worktrees from `main`.** The implementer's first step fast-forwards its worktree to `feat/frontend-foundation`. A report is copied with `cp -n` to a new name.
- **Kill the process that holds the port, not its parent.** This applies to `serve`, `vite`, and `vite preview` started by hand.
- **Run the binary by hand before the review.**
- **A copy of a document carries the document's marks, not only its words.** For this part that means the interface draws the PDF's status shapes (R171).

**Order and parallelism:**
- Tasks 1 and 2 share no package and may run side by side.
- Task 3 follows Task 1, because both edit `internal/platform/api`.
- Task 4 may start beside Tasks 1 and 2: it touches only `web/` (outside `web/*.go`), `Makefile`, `.gitignore`, and `.dockerignore`.
- Task 5 follows Tasks 3 and 4.
- Task 6 follows Task 4.
- Task 7 follows Tasks 5 and 6.
- Task 8 follows Task 7.
- Task 9 (combined run and owner's look) and Task 10 (close) end the part.

**Reference material, read-only and never tracked:**
- From `older/kubernetes-report/frontend/`: `vite.config.ts`, `src/lib/remove-scroll-bar.ts`, `src/app/tokens.css` (its scroll-lock rules), `tests/csp-style.test.tsx`, and `e2e/security.spec.ts`.
- `docs/reference/inventory/document-frontend.md`.

A brief quotes the lines it needs. An implementer that finds the brief and the original in disagreement stops and reports.

## Global Constraints

Every constraint of the plans of phases 0 to 3.4 stands. The ones below are added or are the ones this part leans on most.

**Spec and Go**
- The design spec as amended in `82b070e` is the authority. Section 12 is read whole before any frontend task.
- `guard.Layers` gains `web` at rank 0 (Task 5), and no other package. No new Go module.

**Pinning and contract**
- Every npm dependency is pinned to an exact version in `web/package.json` (no `^`, no `~`), and `web/package-lock.json` is committed. `npm ci` installs without `--force` and without `--legacy-peer-deps`.
- The contract is `api/openapi.yaml`. A frontend type is generated from it, never written by hand. `web/src/api/schema.gen.ts` is committed and `make generate-check` diffs it.

**Wording and design**
- **Every text the reader sees, and every accessible name, comes from `web/src/locales/{en,pt-BR}.json`** through `t()`. No string literal in a component reaches the screen, except the product name of R174 through its key, and the language endonyms of R169 through their keys.
- **Technical terms are English in both languages** (the owner's rule of 2026-09-30); the banned terms of `internal/catalog/banned-terms.json` hold for the interface (Task 2, Task 6).
- **No inline style:** no `style` prop, no `dangerouslySetInnerHTML`, no `<style>` element injected at run time, and the CSP stays `style-src 'self'` in this part (R172).
- **Status is a shape and a label, colour third**, with the PDF's five shapes (R171).
- **Names and numbers use the monospaced font** (user names, the version, codes), and **motion appears only under `motion-safe:`** (spec 12.5).
- **Contrast:**
  - text pairs are at least 4.5:1 in both themes;
  - a large heading (24 px and over) and the border of an input or a focus ring against its surface are at least 3:1.

**Tests and process**
- **No skipped or focused test** in Vitest or Playwright, by lint rule (R176). Go keeps its guard.
- **No comment speaks of the process,** in TypeScript as in Go. Comments never name a task, plan, phase, ruling, or role.
- Diagnostics (console errors, thrown `Error` messages, build output) are English.
- Scratch goes under `.cache/phase-4/part-1/`.

**Commits**
- Format `type(scope): description`, imperative, under 50 characters. Each subject below is counted.

## Review Focus

Each line names an input the spec implies and does not describe, and the task whose tests pin it.

1. **A request refused with `CSRF_REJECTED` because another tab loaded the session (F-111).** The interface loads the session again and repeats the request once, with the new token. A second refusal is shown as the translated error, with no third attempt and no loop. Pinned in Task 7 (`csrf retries once`, `csrf gives up after one retry`).
2. **A `next` parameter that leaves the site:** `//evil.example`, `/\evil.example`, `https://evil.example`, `javascript:alert(1)`, an encoded `%2F%2Fevil.example`, and an empty value. Each lands on the start page; only a path with a single leading `/` is followed. Pinned in Task 7 (`next is local only`).
3. **A client route reloaded in the browser, and a mistyped API path.** `/account` and `/no-such-page` answer `index.html` with the security headers. `/api/v1/no-such-route` answers the JSON 404 envelope. `/health/nothing` answers a plain 404. `/assets/missing.js` answers a plain 404, never `index.html`, because a missing script must not load as HTML. Pinned in Task 5 (`TestStaticFallback`).
4. **Storage the browser refuses (private mode, blocked site data).** The language choice made before sign-in and the theme toggle still work for the page's lifetime, and nothing throws. Pinned in Task 6 (`storage refused`).
5. **A password change answered by the server with the session already revoked.** The interface does not show a 401 error. It opens sign-in with the notice of the changed password, and the old cookie can no longer read the session. Pinned in Task 7 (`password change signs out`) and Task 8 (the end-to-end spec).

## Rulings made on the owner's behalf

Each ruling is a decision the spec leaves open, written as decision, reason, and cost if wrong. The owner may overturn any of them when reviewing this plan. The numbering continues that of part 3.4.

| ID | Ruling |
|---|---|
| R164 | Ruling (F-127): `store.OpenDownloads(ctx, dsn)` opens a pool with `MaxConns` = `store.DownloadConns` = 4 and `MinConns` 0. Its `AfterRelease` returns false, so pgxpool destroys every released connection. `reports.Service` gains `Bodies *pgxpool.Pool`, and `Service.Snapshot` and `Service.PDF` read their body through it. The view reads through `Service.Snapshot`. Every other read stays on `Pool`. `api`'s `defaultDownloadSlots` becomes `store.DownloadConns`, so the slots and the pool cannot drift apart. `serve` opens the pool after the main one and closes it before. `worker` does not open it. Grounds: the owner's ruling of 2026-10-06, and pgx v5.11.0 `pgxpool/pool.go:151-153` (`AfterRelease` false destroys the connection). Cost if wrong: one connection setup per download, a few milliseconds against a body of up to 64 MiB. |
| R165 | Ruling: TypeScript 5.9.3. typescript-eslint 8.71.1 supports `>=4.8.4 <6.1.0`, and openapi-typescript 7.13.0 declares `typescript ^5.x`, so neither works with TypeScript 7.0.2, and `npm ci` would refuse the peer conflict. ESLint 9.39.5, because eslint-plugin-jsx-a11y 6.10.2 stops at ESLint 9. Grounds: the peer ranges read from the registry on 2026-10-06. Cost if wrong: a later move to TypeScript 7 is a version bump in one file once the tools follow. |
| R166 | Ruling: the Go package `web` lives at `web/embed.go` and `web/handler.go`. It embeds `all:dist` and exports `func Handler() http.Handler`. `api.Server` gains `Static http.Handler`. `Handler()` hands it every request that matches no route, whose method is `GET` or `HEAD`, and whose path is neither `/api`, nor under `/api/`, nor under `/health/`. Every other unmatched request keeps today's answers. The static handler serves the file of the build at the cleaned path when one exists and is not a folder. A path under `/assets/` that names no file is a plain 404. Any other path answers `index.html`. Grounds: spec 12.1 as amended. Cost if wrong: none found. |
| R167 | Ruling: the cache and type rules of the static handler. A file under `/assets/` (Vite's hashed output) gets `Cache-Control: public, max-age=31536000, immutable`. `index.html` and every other file get `Cache-Control: no-cache`. The answer goes through `http.ServeContent` with a zero modification time, so `Range` and `If-None-Match` behave as the standard library does. `Content-Type` comes from a table of the package for `.html`, `.js`, `.css`, `.woff2`, `.svg`, `.ico`, and `.json`; any other extension is refused at build time by a test that walks `dist`. Grounds: spec 12.1. The distroless image has no `mime.types`, and Go's built-in table lacks `.woff2`, while `X-Content-Type-Options: nosniff` makes a wrong type fatal for a script. Cost if wrong: a new asset type needs a line in the table. |
| R168 | Ruling (F-110, F-111): `web/src/api/client.ts` wraps `openapi-fetch`. Every call reads the current CSRF token from the session entry at the moment it is sent, and a state-changing method carries it as `X-CSRF-Token`. A `403` whose code is `CSRF_REJECTED` refetches `GET /auth/session` and repeats the call once with the new token. A second `CSRF_REJECTED` is returned as an error. A `401` from any route except `POST /auth/login` and `PATCH /account` clears the query cache and navigates to `/login?next=<path>&ended=1`. Those two routes answer 401 `INVALID_CREDENTIALS` for a wrong password, which is a form error. Grounds: spec 12.6; the contract's 401 descriptions. Cost if wrong: none found. |
| R169 | Ruling: the language rules. Before sign-in, the language is the one stored under `kcr.language` in `localStorage` when that holds `en` or `pt-BR`. Otherwise it is `pt-BR` when `navigator.languages[0]` (or `navigator.language`) starts with `pt` in any case, and `en` otherwise. Choosing in the switch stores the choice. After sign-in, the principal's `language` applies and the stored choice is removed. While signed in, the switch sends `PATCH /account {"language"}` and applies the answer's language. `<html lang>` follows. The switch names each language by its endonym, "English" and "Português (Brasil)", in both locales. Grounds: spec 12.4 as amended. Cost if wrong: none found. |
| R170 | Ruling: the theme. `kcr.theme` in `localStorage` holds `system`, `light`, or `dark`; absent or anything else means `system`. `web/public/theme-boot.js` is a classic script loaded from `<head>` before the stylesheet. It reads the key and sets `data-theme` on `<html>` to `light` or `dark`, resolving `system` through `matchMedia('(prefers-color-scheme: dark)')`, which it also listens to while the choice is `system`. The toggle is a Radix `DropdownMenu` with three radio items. Every access to storage is wrapped so that a refusal keeps the choice in memory. Grounds: spec 12.5 as amended; CSP `script-src 'self'` forbids an inline script. Cost if wrong: none found. |
| R171 | Ruling: `<StatusMark status label?>` draws the PDF's shape as an inline SVG with `aria-hidden="true"`, beside its label: a filled circle for healthy, a filled triangle for attention, a filled diamond for critical, an outlined circle for undefined, and a dash for unavailable. The label is bold for critical, as the PDF sets it. The colour is the status token of Task 6. Grounds: spec 12.5 ("the same shapes as the PDF"). The characters ● ▲ ♦ ○ are not in the latin subsets of Inter or Geist Mono, so the system's fallback fonts would draw them, and ♦ (U+2666) shows as an emoji on some systems. Cost if wrong: none found. |
| R172 | Ruling: this part adds no style hash to the CSP. The scroll lock of every Radix layer goes through the replacement of `react-remove-scroll-bar` that the original wrote. The alias is in `vite.config.ts`, the module is `web/src/lib/remove-scroll-bar.ts`, and the rules are `body[data-scroll-locked]` in `theme.css`. A test opens every Radix layer of the interface and finds no `<style>` element added to the document. The style hash of the Select's viewport (`sha256-441zG27rExd4/il+NvIqyL8zFx5XmyNQtE381kSkUJk=` in the original) enters with the first screen that uses a Select, with its test. Grounds: spec 11.12 ("the pinned hashes are those of the inline styles that the component library itself injects"); the original's `vite.config.ts` and `remove-scroll-bar.ts`. Cost if wrong: none found. |
| R173 | Ruling: the version reaches the page as `<meta name="kcr-version" content="%VITE_KCR_VERSION%">` in `index.html`. `vite.config.ts` reads `../VERSION`, trims it, and sets `process.env.VITE_KCR_VERSION` before Vite reads its environment. The account screen shows `import.meta.env.VITE_KCR_VERSION`. A Go test of `web` compares the meta of the embedded `index.html` with the file `VERSION`. Grounds: spec 15 ("the frontend reads it") and 13.4 ("the built version equals VERSION"). Cost if wrong: none found. |
| R174 | Ruling: the product name is the locale key `app.name`, "kcr - Kubernetes Capacity Report", equal in both files. The page title is `<screen> · kcr - Kubernetes Capacity Report`. The sidebar shows "kcr" in bold over "Kubernetes Capacity Report" in muted text, from the keys `app.short` and `app.long`, and a test asserts that `app.short + " - " + app.long` equals `app.name`. Grounds: the owner's answer of 2026-10-06. Cost if wrong: none found. |
| R175 | Ruling: the contract gains the schema `ErrorCode`, a string enum of every code of the table of `internal/platform/api/errors.go`, and `ErrorBody.code` references it. `api/oapi-codegen.yaml` excludes `ErrorCode` from the Go generation, since Go keeps its `api.Code`. `TestContractNamesEveryCode` becomes a comparison both ways between the enum and the table. `openapi-typescript` runs with `--enum-values`, so the frontend has the list at run time, and its locale test requires `error.<CODE>` for every member. Grounds: spec 11.10 ("the frontend shows its own translation of the code") and 12.4 as amended. Cost if wrong: none found. |
| R176 | Ruling: the lint rules of `web/eslint.config.js`. They include `@eslint/js` recommended, typescript-eslint `strictTypeChecked`, react-hooks recommended, jsx-a11y strict, `@vitest/eslint-plugin` with `no-disabled-tests` and `no-focused-tests` as errors, and eslint-plugin-playwright with `no-skipped-test` and `no-focused-test` as errors. `no-restricted-syntax` forbids four things: a JSX attribute named `style` or `dangerouslySetInnerHTML`; a call of a member named `skip`, `only`, `todo`, or `fixme` on `it`, `test`, or `describe`; a string literal of letters as a JSX child (the wording rule); and `document.createElement('style')`. `make lint` runs `npm run typecheck` and `npm run lint`. Grounds: spec 13.1 and 13.4, R172. Cost if wrong: a rule that blocks a legitimate pattern is amended by the main thread with its reason. |
| R177 | Ruling: the banned terms move out of `bannedTermsFor` into `internal/catalog/banned-terms.json`. The file holds `{"<lang>": {"any_case": [...], "phrases": [[...]], "as_written": [...]}}` for `en` and `pt-BR`, with the exact lists of today. The Go guard reads it with `os.ReadFile` in its test. The frontend's vocabulary test reads it by its path from `web/`. It applies the same matching: `{{…}}` stripped, words matched by `[\p{L}\p{N}_-]+`, lower-cased for `any_case` and phrases, as written for `as_written`. Grounds: the owner's ruling of 2026-10-06; spec 13.4 as amended. Cost if wrong: none found. |
| R178 | Ruling: the routes of 4.1 are `/login`, `/account`, `/` (to `/account` when signed in, `/login` otherwise), and a catch-all not-found screen inside the shell. The sidebar lists Account alone. `RequireSession` wraps every route but `/login`. A signed-in reader who opens `/login` is sent to `next` or the start page. Grounds: spec 12.2 as amended ("lists only the screens that exist"). Cost if wrong: part 4.2 changes the start page to `/reports`, one line. |
| R179 | Ruling: the integration stack of 4.1. `deploy/compose.yaml` holds the services `postgres`, `serve`, and `worker`, the image `k8s-capacity-report:${KCR_VERSION}`, a read-only root, a `tmpfs` at `/tmp`, `no-new-privileges`, `cap_drop: [ALL]`, health checks through `admin probe`, and `platform.toml` and `server-secret` mounted from `deploy/platform/`. `deploy/compose.integration.yaml` adds three things. A one-shot `bootstrap` service runs `admin bootstrap-admin` with `KCR_BOOTSTRAP_MODE=1`, user `e2e-admin`, and the password of `deploy/platform/e2e-admin-password`; `serve` depends on its success. A `playwright` service uses `mcr.microsoft.com/playwright:v1.63.0-noble@sha256:eff16c30e6f3f4af0a03fa4b706120d5e9b0891c344a27d64559aff5900a4a27` with `network_mode: service:serve` and `depends_on: serve: condition: service_healthy`, runs `npx playwright test` in `/web` (mounted read-only, with `node_modules` installed in the container by `npm ci` into a volume), and writes results to `.cache/e2e/`. The test password and secret are fixtures committed under `deploy/platform/`, named as test fixtures in their file headers, and never used by `compose.yaml` alone. Grounds: spec 14.2 as amended; the owner's TLS ruling. Cost if wrong: none found. |
| R180 | Ruling: `make e2e` runs `make image`, then, with `KCR_VERSION=$(VERSION)` in the environment, `docker compose -f deploy/compose.yaml -f deploy/compose.integration.yaml up --abort-on-container-exit --exit-code-from playwright`, and always `down --volumes` after, keeping the exit status of `up`. Its log goes to `.cache/gate/e2e.log` in the gate. The gate runs it after `check-runtime`. Grounds: spec 13.5. Cost if wrong: none found. |

## Findings this plan closes

| Finding | How |
|---|---|
| F-127 | Task 1 (R164) |
| F-110 | Task 7 (R168): the token is read from the body of `GET /auth/session` |
| F-111 | Task 7 (R168): one retry after `CSRF_REJECTED` |

## File structure

| Path | Responsibility in this part |
|---|---|
| `internal/platform/store/store.go`, `downloads_test.go`; `internal/platform/reports/service.go`, `read.go`; `internal/platform/api/server.go`; `internal/cli/serve.go` and its tests | The download pool (Task 1) |
| `internal/catalog/banned-terms.json`, `internal/catalog/guard_test.go` | The shared banned terms (Task 2) |
| `api/openapi.yaml`, `api/oapi-codegen.yaml`, `internal/platform/api/contract_test.go`, `openapi.gen.go` | The `ErrorCode` enum (Task 3) |
| `web/package.json`, `package-lock.json`, `tsconfig*.json`, `vite.config.ts`, `eslint.config.js`, `index.html`, `src/main.tsx`, `src/api/schema.gen.ts`, `tests/setup.ts`; `Makefile`; `.gitignore`; `.dockerignore` | The toolchain (Task 4) |
| `web/embed.go`, `web/handler.go`, `web/handler_test.go`, `web/version_test.go`; `internal/guard/imports.go`; `internal/platform/api/server.go`; `internal/cli/serve.go`, `serve_static_test.go`; `deploy/Containerfile` | The Go package and its serving (Task 5) |
| `web/src/theme/{theme.css,fonts.css,pairs.ts,theme.ts}`, `web/public/theme-boot.js`, `web/src/i18n/{i18n.ts,language.ts,families.ts}`, `web/src/locales/{en,pt-BR}.json`, `web/src/lib/remove-scroll-bar.ts`, `web/src/components/ui/*`, `web/src/components/status-mark.tsx` and tests | Theme, fonts, wording, primitives (Task 6) |
| `web/src/api/{client.ts,errors.ts}`, `web/src/session/*`, `web/src/app/{router.tsx,shell.tsx,providers.tsx}`, `web/src/features/{auth,account}/*`, `web/src/app/not-found.tsx` and tests | Session, shell, and screens (Task 7) |
| `deploy/compose.yaml`, `deploy/compose.integration.yaml`, `deploy/platform/*`, `web/playwright.config.ts`, `web/e2e/sign-in.spec.ts`; `Makefile` | Compose and the end-to-end spec (Task 8) |
| `AGENTS.md`, `README.md`, `CHANGELOG.md` | The documents (Task 10) |

---

### Task 0: Open the part

**Executor:** Main thread.

- [x] **Step 1:** `main` is at `855a93d`, tagged and pushed. `ROADMAP.md` and `CHANGELOG.md` were read and are true. The decisions log lacked the 3.4 merge and release 0.2.0, and those entries are now added. `feat/platform-worker` is deleted at the owner's word.
- [x] **Step 2:** `git switch -c feat/frontend-foundation`.
- [x] **Step 3:** The spec amendment, committed as `82b070e` and approved by the owner.
- [ ] **Step 4:** `ROADMAP.md`:
  - the summary line of 4.1 names this plan;
  - a section "Phase 4.1" holds P41-0 to P41-10, all `OPEN`, and the rulings R164 to R180;
  - the owner's approval, with its date.
- [ ] **Step 5:** Commit the plan: `docs(plan): add the plan of part 4.1` (36 characters).

**Done when:** the plan is committed on the branch, approved by the owner, and `ROADMAP.md` holds the part.

---

### Task 1: The download pool (R164; F-127)

**Executor:** Subagent, against the running server. It may run beside Tasks 2 and 4. Opus reviews it, because it changes how memory is bounded.

**Files:**
- Modify: `internal/platform/store/store.go`, `internal/platform/reports/service.go`, `internal/platform/reports/read.go`, `internal/platform/api/server.go`, `internal/cli/serve.go`
- Test: `internal/platform/store/downloads_test.go` (new, `postgres`), `internal/platform/reports/read_test.go`, `internal/cli/serve_wiring_test.go`

**Interfaces:**

```go
// DownloadConns is the number of connections of the pool that reads the
// body of a view, a snapshot, or a PDF, and the number of downloads the API
// serves at once.
const DownloadConns = 4

// OpenDownloads connects the pool that reads report bodies to dsn and pings
// it once. A connection of this pool is closed when it is released, so no
// idle connection keeps the buffer of a large row.
func OpenDownloads(ctx context.Context, dsn string) (*pgxpool.Pool, error)
```

- `reports.Service` gains `Bodies *pgxpool.Pool`. `Service.Snapshot` (`read.go:173`) and `Service.PDF` (`read.go:202`) pass `s.Bodies` to `store.ReadSnapshot` and `store.ReadPDF`. The view's route reads through `Service.Snapshot` (`api/reports.go:219`), so it uses the pool too. Every construction of `reports.Service`, in tests included, sets `Bodies`; there is no fallback to `Pool`.
- `api.defaultDownloadSlots` is `store.DownloadConns`.
- `buildServer(pool, downloads *pgxpool.Pool, loaded, secret, now, logger, redactor)`: the second parameter is new. `runServe` opens it with `store.OpenDownloads(ctx, loaded.DatabaseDSN)` after the migrations and closes it with a `defer` placed after the main pool's, so it closes first. A failure to open it is exit 1 with `cannot connect to the database: …`.

- [ ] **Step 1: Write the failing tests**

| Test | Input | Expected |
|---|---|---|
| `TestDownloadPoolSize` | `OpenDownloads` on the test DSN | `Config().MaxConns == 4`, `Config().MinConns == 0` |
| `TestDownloadPoolKeepsNoIdleConnection` | read a stored 8 MiB PDF through `store.ReadPDF` on the download pool, three times in sequence | after each read: `Stat().IdleConns() == 0` and `Stat().TotalConns() == 0` |
| `TestBodiesReadThroughTheDownloadPool` | a service with `Pool` and `Bodies` distinct; `Service.Snapshot`, then `Service.PDF` | `Bodies.Stat().AcquireCount()` grows by 2; `Pool.Stat().AcquireCount()` does not grow |
| `TestSlotsEqualTheDownloadPool` | `api.Server{}` with no slot override | the slot channel's capacity is `store.DownloadConns` |
| `TestServeOpensTheDownloadPool` (`internal/cli`, `postgres`) | `buildServer` with both pools | `server.Reports.Bodies` is the second pool, not the first |

- [ ] **Step 2:** Run `go test -count=1 -tags=postgres ./internal/platform/store/ ./internal/platform/reports/ ./internal/platform/api/ ./internal/cli/ -run 'DownloadPool|BodiesRead|SlotsEqual|OpensTheDownloadPool' -v`. Expected: FAIL to compile, then FAIL for each reason.
- [ ] **Step 3:** Implement.
- [ ] **Step 4: Measure by hand.** Write a scratch test `.cache/phase-4/part-1/probe/probe_f127_test.go`, copied into `internal/platform/api/` under the name `zz_probe_f127_test.go` for the run, then deleted before the commit. It does what the probe `TestRRProbeSlowReaders` of `rereview.md` did:
  - store a 48 MiB PDF;
  - serve it through the handler with 8 concurrent downloads of a 1 s deadline;
  - sample the heap until every slot is free, run `runtime.GC()` twice, and read `runtime.MemStats.HeapInuse`.

  Record the numbers in the report beside the old 243–291 MiB. Expected: within 16 MiB of the heap before the downloads.
- [ ] **Step 5:** Run `make test`, `make lint`, and `go test -count=1 -tags=postgres ./internal/platform/... ./internal/cli/...`. All pass.
- [ ] **Step 6:** Commit `fix(serve): read report bodies in their own pool` (48 characters).

**Done when:**
- every row passes;
- the probe's numbers are in the report;
- the mutation that makes `AfterRelease` return true turns `TestDownloadPoolKeepsNoIdleConnection` red.

---

### Task 2: The shared banned terms (R177)

**Executor:** Subagent. It may run beside Tasks 1 and 4.

**Files:**
- Create: `internal/catalog/banned-terms.json`
- Modify: `internal/catalog/guard_test.go` (the function `bannedTermsFor` and its comment)

**Interfaces:** the file's shape, exactly:

```json
{
  "en": {
    "any_case": ["nodepool", "nodepools", "não", "está", "estão", "são", "foram", "para", "uma", "dos", "das"],
    "phrases": [["node", "group"], ["node", "groups"]],
    "as_written": ["cpu"]
  },
  "pt-BR": {
    "any_case": ["solicitação", "solicitações", "limite", "limites", "rótulo", "rótulos", "nó", "nós", "réplica", "réplicas", "nodepool", "nodepools", "right-sizing", "headroom", "unknown", "memory", "the", "is", "are", "was", "were", "not", "found", "with", "from", "omitted", "cannot", "may", "only", "uses", "and", "of", "for", "by", "on", "to", "in", "an", "be", "could", "this", "that", "between", "without", "usage", "coverage"],
    "phrases": [["carga", "de", "trabalho"], ["cargas", "de", "trabalho"], ["pool", "de", "nós"], ["pools", "de", "nós"], ["grupo", "de", "nós"], ["grupos", "de", "nós"], ["regra", "de", "gravação"], ["regras", "de", "gravação"], ["métricas", "brutas"], ["escalonamento", "automático"], ["limitação", "de", "cpu"]],
    "as_written": ["cpu"]
  }
}
```

- `bannedTermsFor(lang)` keeps its signature and returns the three sets read from the file once per test binary. The keys are `"en"` and `"pt-BR"`, the strings of `catalog.English` and `catalog.PortugueseBrazil`. Its comment keeps the reasons (the translations that spec 9.3 writes in English, `nodepool`, the two words with Portuguese nouns, and the English function words) and adds the file's path.
- The file is read with `os.ReadFile("banned-terms.json")`, because a test runs in its package's folder. A missing file, a language other than the two, an unknown key, or an empty list fails the test with the path.

- [ ] **Step 1: Write the failing test.** `TestBannedTermsFileShape`: the file decodes with `DisallowUnknownFields` into the shape above; both languages are present; every list is non-empty; every phrase has at least two words, all lower case. It is red because the file is missing.
- [ ] **Step 2:** Run `go test ./internal/catalog/ -run 'Banned' -v`. Expected: FAIL.
- [ ] **Step 3:** Write the file and read it in `bannedTermsFor`.
- [ ] **Step 4:** Run `make test` and `make lint`. All pass, `TestBannedTermsFinder` included, unchanged.
- [ ] **Step 5:** Commit `refactor(catalog): read banned terms from a file` (48 characters).

**Done when:**
- the catalog tests pass unchanged;
- removing `"limite"` from the file turns the test that finds `limite` in a fixture red;
- the report names that test.

---

### Task 3: Every error code in the contract (R175)

**Executor:** Main thread for Step 1, which edits the contract. Subagent for the rest. Follows Task 1.

**Files:**
- Modify: `api/openapi.yaml`, `api/oapi-codegen.yaml`, `internal/platform/api/openapi.gen.go`, `internal/platform/api/contract_test.go`

- [ ] **Step 1 (main thread):** Add `components.schemas.ErrorCode`: a string enum of every code of the table of `errors.go`, in the table's order, with the description "A code of the contract; the interface shows its own translation." `ErrorBody.code` becomes `$ref: "#/components/schemas/ErrorCode"`. Add `ErrorCode` to `exclude-schemas` in `api/oapi-codegen.yaml`. Run `make generate`. Commit `docs(api): name every error code` (32 characters).
- [ ] **Step 2: Write the failing test.** `TestContractNamesEveryCode` is rewritten:
  - the members of `ErrorCode` in the file equal the codes of the table, both ways, each named once;
  - the description test of each response is kept as it is.

  Show it red by adding a code to the table without the file.
- [ ] **Step 3:** Run `make test`, `make lint`, and `make generate-check`. All pass.
- [ ] **Step 4:** Commit `test(api): compare the code enum with the table` (47 characters).

**Done when:** the enum and the table agree both ways, and the mutations "a code in the table only" and "a code in the file only" each turn the test red.

---

### Task 4: The frontend toolchain (R165, R176)

**Executor:** Subagent. It may start beside Tasks 1 and 2.

**Files:**
- Create in `web/`:
  - `package.json`, `package-lock.json`;
  - `tsconfig.json`, `tsconfig.app.json`, `tsconfig.node.json`;
  - `vite.config.ts`, `eslint.config.js`, `index.html`;
  - `src/main.tsx`, `src/app/app.tsx`, `src/vite-env.d.ts`, `src/api/schema.gen.ts`;
  - `tests/setup.ts`, `tests/smoke.test.tsx`.
- Modify: `Makefile`, `.gitignore`, `.dockerignore`

**`web/package.json`:**
- `"name": "kcr-web"`, `"private": true`, `"type": "module"`, `"engines": {"node": ">=26"}`.
- Scripts:
  - `"dev": "vite"`, `"build": "vite build"`;
  - `"typecheck": "tsc -b --noEmit"`;
  - `"lint": "eslint --max-warnings 0 ."`;
  - `"test": "vitest run"`;
  - `"e2e": "playwright test"`;
  - `"generate": "openapi-typescript ../api/openapi.yaml --enum-values --output src/api/schema.gen.ts"`.

Dependencies, exact versions:

| Package | Version | Package | Version |
|---|---|---|---|
| react | 19.3.0 | react-dom | 19.3.0 |
| react-router | 8.4.0 | @tanstack/react-query | 5.104.1 |
| react-hook-form | 7.89.0 | @hookform/resolvers | 5.9.1 |
| zod | 4.6.5 | radix-ui | 1.7.0 |
| @phosphor-icons/react | 2.1.10 | react-i18next | 17.0.16 |
| i18next | 26.4.2 | openapi-fetch | 0.17.0 |
| clsx | 2.1.1 | tailwind-merge | 3.7.0 |
| @fontsource-variable/inter | 5.3.0 | @fontsource-variable/geist-mono | 5.3.0 |

Development dependencies, exact versions:

| Package | Version | Package | Version |
|---|---|---|---|
| typescript | 5.9.3 | vite | 8.3.3 |
| @vitejs/plugin-react | 6.1.2 | tailwindcss | 4.3.3 |
| @tailwindcss/vite | 4.3.3 | vitest | 5.0.3 |
| jsdom | 30.1.2 | @testing-library/react | 16.3.3 |
| @testing-library/dom | 10.4.2 | @testing-library/user-event | 14.6.7 |
| @testing-library/jest-dom | 7.0.1 | axe-core | 4.14.0 |
| msw | 3.0.2 | openapi-typescript | 7.13.0 |
| @playwright/test | 1.63.0 | @axe-core/playwright | 4.13.0 |
| eslint | 9.39.5 | @eslint/js | 9.39.5 |
| typescript-eslint | 8.71.1 | eslint-plugin-react-hooks | 7.1.1 |
| eslint-plugin-jsx-a11y | 6.10.2 | @vitest/eslint-plugin | 1.6.27 |
| eslint-plugin-playwright | 2.12.1 | globals | 17.13.0 |
| @types/react | 19.3.0 | @types/react-dom | 19.3.0 |
| @types/node | 26.6.4 | | |

If `npm ci` reports a peer conflict among these, stop and report the conflict as `npm` prints it. Do not pick another version.

**`vite.config.ts`:**
- The plugins are `react()` and `tailwindcss()`. `@` is an alias of `./src`.
- `VITE_KCR_VERSION` comes from `../VERSION` (R173).
- `build`: `outDir: "dist"`, `emptyOutDir: true`, `assetsInlineLimit: 0`, `chunkSizeWarningLimit: 1000`.
- `server.proxy`: `{"/api": "http://localhost:8080", "/health": "http://localhost:8080"}`.
- `test`: `environment: "jsdom"`, `setupFiles: ["./tests/setup.ts"]`, `exclude: [...configDefaults.exclude, "e2e/**"]`.

**`tsconfig.app.json`:** `strict`, `noUncheckedIndexedAccess`, `exactOptionalPropertyTypes`, `noImplicitOverride`, `verbatimModuleSyntax`, `jsx: "react-jsx"`, `moduleResolution: "bundler"`, `target: "ES2024"`, `lib: ["ES2024", "DOM", "DOM.Iterable"]`, `types: ["vite/client"]`.

**`eslint.config.js`:** as R176. The literal-text rule is `no-restricted-syntax` on `JSXText[value=/\\p{L}/u]` with the message "Words on screen come from the locale files". It is turned off in `tests/**` and `e2e/**`, and `src/api/schema.gen.ts` is ignored.

**`index.html`:**
- `<html lang="en">`.
- `<meta charset="utf-8">`, the viewport meta, and the version meta of R173.
- `<title>kcr - Kubernetes Capacity Report</title>`, which the app replaces at run time; it is the one literal the wording rule allows in `index.html`.
- `<div id="root"></div>` and `<script type="module" src="/src/main.tsx"></script>`.

**`src/main.tsx` and `src/app/app.tsx`:** render `<App />`, which returns `null` until Task 7. `tests/smoke.test.tsx` renders it and asserts nothing throws.

**Makefile:**
- `WEB_STAMP := web/node_modules/.package-lock.json`. The target `$(WEB_STAMP): web/package-lock.json` runs `cd web && npm ci`.
- `web: $(WEB_STAMP)` runs `cd web && npm run build`. It is always run (`.PHONY`), since Vite is fast and a stale `dist` is worse.
- `build`, `test`, and `lint` depend on `web`.
- `test` runs `cd web && npm test` after the Go tests.
- `lint` runs `cd web && npm run typecheck && npm run lint` after the Go linters.
- `generate` depends on `$(WEB_STAMP)` and also runs `cd web && npm run generate`.
- `generate-check` diffs `web/src/api/schema.gen.ts` too.
- `test-template` and `test-golden` depend on `web`, because `internal/cli` embeds it (Task 5) and the test image mounts the source.

**Ignore files:**
- `.gitignore`: add `/web/node_modules/`, `/web/dist/`, `/web/test-results/`, `/web/playwright-report/`.
- `.dockerignore`: add `web/dist/`, `web/test-results/`, `web/playwright-report/`.

- [ ] **Step 1:** Write `package.json` and run `npm install` once to produce the lockfile. Then `npm ci` passes from a clean `node_modules`.
- [ ] **Step 2:** Write the configs, `index.html`, `main.tsx`, `app.tsx`, and the smoke test. Run `npm run generate`. `--enum-values` exports one array per enum of the contract; the report names the exported names for `ErrorCode`, `Role`, and `Language`, which Task 6 uses as they are. If Task 3 is not in yet, regenerate once it is.
- [ ] **Step 3: Show each guard of R176 red.** In a scratch file `web/src/zz-lint-probe.tsx`, deleted after, write:
  - `it.skip`;
  - `test.only`;
  - `<div style={{}}/>`;
  - `<p>Hello</p>`;
  - `dangerouslySetInnerHTML`;
  - `document.createElement("style")`.

  `npm run lint` must report each one. Record the six messages in the report.
- [ ] **Step 4:** Run `make web`, `make test`, and `make lint`. All pass. `make generate-check` passes after a fresh `make generate`.
- [ ] **Step 5:** Commit `build(web): add the frontend toolchain` (38 characters).

**Done when:**
- `make test` and `make lint` run the frontend steps and pass;
- the six lint probes were each reported;
- `git status` shows no file under `web/dist` or `web/node_modules`.

---

### Task 5: The build served by `serve` (R166, R167, R173)

**Executor:** Subagent, against the running server for the `internal/cli` tests. Follows Tasks 3 and 4. Opus reviews it, because it is a new surface of `serve`.

**Files:**
- Create: `web/embed.go`, `web/handler.go`, `web/handler_test.go`, `web/version_test.go`, `internal/cli/serve_static_test.go`
- Modify: `internal/guard/imports.go` (`{Path: "web", Rank: 0}` after `template`), `internal/platform/api/server.go`, `internal/cli/serve.go`, `deploy/Containerfile`

**Interfaces:**

```go
// Package web holds the built interface and the handler that serves it.
package web

// Handler serves the files of the built interface: a file under /assets/
// with a year-long immutable cache, any other file with no-cache, a missing
// file under /assets/ as a plain 404, and index.html for any other path.
func Handler() http.Handler
```

```go
// in api.Server
// Static serves a GET or HEAD that matches no route and is outside /api
// and /health/; nil answers those with the plain 404 of today.
Static http.Handler
```

- `buildServer` sets `Static: web.Handler()`.
- `deploy/Containerfile` gains a first stage, `FROM node:26-trixie-slim@sha256:930557a230abacbc3f4fd9b8648abf8f4bee1e17cb72195dcdfb2f709bc85b33 AS frontend`. It works in `/src/web`: `COPY web/package.json web/package-lock.json ./`, `npm ci`, then `COPY api/openapi.yaml /src/api/openapi.yaml`, `COPY VERSION /src/VERSION`, `COPY web/ ./`, and `npm run build`.
- The `build` stage adds `COPY --from=frontend /src/web/dist /src/web/dist` after `COPY . .`. The `test` stage needs nothing, since it mounts the source.

- [ ] **Step 1: Write the failing tests**

| Test | Input | Expected |
|---|---|---|
| `TestStaticFallback` (`web`, over a `fstest.MapFS` passed to an unexported `handler(fs.FS)`) | `GET /`; `/account`; `/no-such-page`; `/assets/app-1.js`; `/assets/missing.js`; `/theme-boot.js`; `HEAD /account`; `/../etc/passwd` | `index.html` 200 `no-cache`; the same ×2; the file, 200, `public, max-age=31536000, immutable`, `text/javascript; charset=utf-8`; plain 404; the file, `no-cache`; 200 with no body; `index.html` (cleaned path) |
| `TestStaticTypes` | `.html`, `.js`, `.css`, `.woff2`, `.svg`, `.ico`, `.json` | `text/html; charset=utf-8`, `text/javascript; charset=utf-8`, `text/css; charset=utf-8`, `font/woff2`, `image/svg+xml`, `image/x-icon`, `application/json` |
| `TestBuildHoldsOnlyKnownTypes` | walk the embedded `dist` | every file's extension is in the table; `index.html` exists; at least one `.js` under `assets/` |
| `TestBuildVersion` | the meta `kcr-version` of the embedded `index.html` | equals `VERSION`, trimmed |
| `TestBuildHasNoInlineStyle` | the embedded `index.html` | no `style=` attribute and no `<style` element |
| `TestStaticConditional` | `GET /assets/app-1.js` with `Range: bytes=0-3` | 206 with 4 bytes |
| `TestUnmatchedRoutes` (`internal/platform/api`) | `Static` set to a stub; `GET /api/v1/nothing`; `GET /api`; `GET /health/nothing`; `POST /account`; `GET /account` | JSON 404 `NOT_FOUND`; plain 404; plain 404; plain 404, never the stub; the stub |
| `TestStaticCarriesSecurityHeaders` (`internal/platform/api`) | `GET /account` through `Handler()` with the stub | the five security headers; the stub's `Cache-Control` kept |
| `TestServeServesTheInterface` (`internal/cli`, `postgres`) | the handler of `buildServer`; `GET /`; `GET /account`; `GET /api/v1/auth/session` | `index.html` holding `<div id="root">`, twice; the JSON session |
| `TestLayersHoldWeb` (`internal/guard`) | the table | `web` at rank 0; `internal/cli` imports it without violation |

- [ ] **Step 2:** Run `go test -count=1 ./web/ ./internal/platform/api/ ./internal/guard/ -v` and `go test -count=1 -tags=postgres ./internal/cli/ -run ServesTheInterface -v`. Expected: FAIL.
- [ ] **Step 3:** Implement.
- [ ] **Step 4: Run the binary by hand.**
  1. Start `make postgres-up` and `make build`.
  2. Run `bin/k8s-capacity-report serve` with a scratch `platform.toml` under `.cache/phase-4/part-1/manual/`.
  3. `curl -i` the requests `/`, `/account`, `/assets/<a real file>`, `/assets/missing.js`, `/api/v1/nothing`, and `/health/live`.
  4. Write the transcript to `.cache/phase-4/part-1/manual/static.md`.
  5. Stop the process that holds the port.
- [ ] **Step 5:** Run `make test`, `make lint`, and `go test -count=1 -tags=postgres ./internal/platform/... ./internal/cli/...`. All pass.
- [ ] **Step 6:** Commit `feat(serve): serve the built interface` (38 characters).

**Done when:**
- every row passes and the transcript shows each answer of R166 and R167;
- the mutation that drops the `/assets/` 404 turns `TestStaticFallback` red.

`make image` and `make check-runtime` run on the main thread after the commit. The task closes only when the image builds with the frontend stage and `check-runtime` passes.

---

### Task 6: Theme, fonts, wording, and the primitives (R169 to R172, R174, R177)

**Executor:** Subagent, transcribing the tokens and the wording below exactly. Follows Task 4. Opus reviews the tests; the main thread reads every rendered screen in Task 9.

**Files:**
- Create:
  - theme: `web/src/theme/theme.css`, `fonts.css`, `pairs.ts`, `theme.ts`, `theme.test.ts`, `contrast.test.ts`; `web/public/theme-boot.js`;
  - wording: `web/src/i18n/i18n.ts`, `language.ts`, `families.ts`, `language.test.ts`, `locales.test.ts`, `vocabulary.ts`, `vocabulary.test.ts`; `web/src/locales/en.json`, `pt-BR.json`;
  - primitives: `web/src/lib/remove-scroll-bar.ts`, `web/src/lib/storage.ts`; `web/src/components/status-mark.tsx`, `status-mark.test.tsx`; `web/src/components/ui/button.tsx`, `field.tsx`, `alert.tsx`, `menu.tsx`, `ui.test.tsx`; `web/src/components/no-style.test.tsx`.
- Modify:
  - `web/vite.config.ts`: the alias of `react-remove-scroll-bar` and of `react-remove-scroll-bar/constants` to `./src/lib/remove-scroll-bar.ts`, as the original's `vite.config.ts` does, with a comment that says what the replacement keeps and drops;
  - `web/index.html`: `<script src="/theme-boot.js"></script>` in `<head>`, before the module script;
  - `web/src/main.tsx`: imports `theme.css`, `fonts.css`, and `i18n.ts`.

#### The tokens

`theme.css` starts with `@import "tailwindcss";`. It sets the tokens below as custom properties under `:root` (light) and `:root[data-theme="dark"]`. A `@theme inline` block maps each to a Tailwind colour of the same name (`--color-bg: var(--bg)`, and so on), and maps `--font-sans` to `"Inter Variable", system-ui, sans-serif` and `--font-mono` to `"Geist Mono Variable", ui-monospace, monospace`. It also carries the scroll-lock rules of the original's `tokens.css` for `body[data-scroll-locked]`.

| Token | Light | Dark | Used for |
|---|---|---|---|
| `bg` | `#f6f7f9` | `#111318` | page |
| `fg` | `#15181d` | `#e6e8ec` | text |
| `surface` | `#ffffff` | `#1a1d23` | panels, header, inputs |
| `surface-fg` | `#15181d` | `#e6e8ec` | text on panels |
| `sidebar` | `#eceef2` | `#16181d` | sidebar |
| `sidebar-fg` | `#4a515c` | `#aab0bb` | sidebar text |
| `sidebar-active` | `#dfe3ea` | `#252932` | current item |
| `sidebar-active-fg` | `#15181d` | `#f1f2f4` | current item text |
| `muted-fg` | `#5e6570` | `#9aa1ad` | secondary text on `surface` and `bg` |
| `border` | `#dde0e6` | `#2a2e36` | dividers (decorative) |
| `input-border` | `#8a909a` | `#6b7280` | input and button outlines |
| `accent` | `#2f5bd3` | `#7d9bf0` | links, focus ring, primary button, current tab |
| `accent-fg` | `#ffffff` | `#0d1530` | text on `accent` |
| `healthy` | `#1d6b40` | `#6fcf97` | mark and label |
| `attention` | `#a15c00` | `#f2b84b` | mark and label |
| `critical` | `#b42318` | `#f28b82` | mark and label |
| `neutral` | `#5e6570` | `#9aa1ad` | undefined and unavailable marks |
| `critical-surface` | `#fdecea` | `#2a1716` | error alert background |
| `critical-border` | `#f1b5ae` | `#6b2a26` | error alert border |
| `critical-surface-fg` | `#8c1d13` | `#f2b8b2` | error alert text |
| `info-surface` | `#eef2fc` | `#18213a` | notice alert background |
| `info-surface-fg` | `#2443a3` | `#9db4f5` | notice alert text |

`pairs.ts` exports the pairs the contrast test checks: `{fg, bg, min}`, with `min` 4.5 unless marked 3.
- `fg` on `bg`, `surface-fg` on `surface`, and `fg` on `surface`.
- `sidebar-fg` on `sidebar`, and `sidebar-active-fg` on `sidebar-active`.
- `muted-fg` on `surface`, and `muted-fg` on `bg`.
- `accent` on `surface` and `accent` on `bg` (link text); `accent-fg` on `accent`.
- `healthy`, `attention`, `critical`, and `neutral`, each on `surface` and on `bg`.
- `critical-surface-fg` on `critical-surface`, and `info-surface-fg` on `info-surface`.
- Minimum 3: `input-border` on `surface`, and `accent` on `surface` as a focus ring.

If a pair fails, stop and report its ratio. The main thread changes the colour.

`fonts.css` declares four `@font-face` rules, all with `font-weight: 100 900` and `font-display: swap`:

| Family | File | Subset |
|---|---|---|
| `Inter Variable` | `@fontsource-variable/inter/files/inter-latin-wght-normal.woff2` | latin |
| `Inter Variable` | `@fontsource-variable/inter/files/inter-latin-ext-wght-normal.woff2` | latin-ext |
| `Geist Mono Variable` | `@fontsource-variable/geist-mono/files/geist-mono-latin-wght-normal.woff2` | latin |
| `Geist Mono Variable` | `@fontsource-variable/geist-mono/files/geist-mono-latin-ext-wght-normal.woff2` | latin-ext |

Each `src` is a relative `url()` from `web/src/theme/` into `../../node_modules/`, which Vite resolves and emits under `/assets/`. Each rule's `unicode-range` is copied from the same subset's rule in the package's `wght.css`.

#### The wording

`en.json` and `pt-BR.json` are nested objects whose leaves are strings. Interpolation uses `{{name}}`. Every key below exists in both files; no other key exists.

| Key | en | pt-BR |
|---|---|---|
| `app.name` | kcr - Kubernetes Capacity Report | kcr - Kubernetes Capacity Report |
| `app.short` | kcr | kcr |
| `app.long` | Kubernetes Capacity Report | Kubernetes Capacity Report |
| `app.title` | {{screen}} · kcr - Kubernetes Capacity Report | {{screen}} · kcr - Kubernetes Capacity Report |
| `app.skip` | Skip to content | Pular para o conteúdo |
| `app.version` | Version {{version}} | Versão {{version}} |
| `nav.label` | Main navigation | Navegação principal |
| `nav.account` | Account | Conta |
| `header.language` | Language: {{language}} | Idioma: {{language}} |
| `header.theme` | Theme: {{theme}} | Tema: {{theme}} |
| `header.signOut` | Sign out | Sair |
| `language.en` | English | English |
| `language.pt-BR` | Português (Brasil) | Português (Brasil) |
| `theme.system` | System | Sistema |
| `theme.light` | Light | Claro |
| `theme.dark` | Dark | Escuro |
| `role.administrator` | administrator | administrador |
| `role.operator` | operator | operador |
| `role.reader` | reader | leitor |
| `status.healthy` | Healthy | Saudável |
| `status.attention` | Attention | Atenção |
| `status.critical` | Critical | Crítico |
| `status.undefined` | Undefined | Indefinido |
| `status.unavailable` | Unavailable | Indisponível |
| `login.title` | Sign in | Entrar |
| `login.subtitle` | Kubernetes capacity reports | Relatórios de capacidade do Kubernetes |
| `login.username` | User name | Usuário |
| `login.password` | Password | Senha |
| `login.submit` | Sign in | Entrar |
| `login.submitting` | Signing in… | Entrando… |
| `login.passwordChanged` | Your password was changed. Sign in again with the new password. | Sua senha foi alterada. Entre de novo com a nova senha. |
| `login.sessionEnded` | Your session ended. Sign in again. | Sua sessão terminou. Entre de novo. |
| `account.title` | Account | Conta |
| `account.signedInAs` | Signed in as {{username}}, {{role}}. | Conectado como {{username}}, {{role}}. |
| `account.language.title` | Language | Idioma |
| `account.language.saved` | Language saved. | Idioma salvo. |
| `account.password.title` | Password | Senha |
| `account.password.current` | Current password | Senha atual |
| `account.password.new` | New password | Nova senha |
| `account.password.repeat` | Repeat the new password | Repita a nova senha |
| `account.password.hint` | 12 to 1024 characters. Changing it signs you out everywhere. | De 12 a 1024 caracteres. Alterar a senha encerra todas as suas sessões. |
| `account.password.submit` | Change password | Alterar senha |
| `account.password.submitting` | Changing… | Alterando… |
| `validation.required` | Fill in this field. | Preencha este campo. |
| `validation.passwordShort` | Use at least 12 characters. | Use pelo menos 12 caracteres. |
| `validation.passwordLong` | Use at most 1024 characters. | Use no máximo 1024 caracteres. |
| `validation.passwordsDiffer` | The two passwords differ. | As duas senhas são diferentes. |
| `notFound.title` | Page not found | Página não encontrada |
| `notFound.body` | This address leads to no screen of the console. | Este endereço não leva a nenhuma tela do console. |
| `notFound.home` | Go to the start page | Ir para o início |
| `error.general` | Something went wrong. Try again. | Algo deu errado. Tente de novo. |
| `error.network` | The server could not be reached. | Não foi possível falar com o servidor. |
| `error.REQUEST_INVALID` | Some of the information sent is not valid. | Parte das informações enviadas não é válida. |
| `error.INVALID_CREDENTIALS` | The user name or the password is wrong. | Usuário ou senha incorretos. |
| `error.UNAUTHENTICATED` | Your session ended. Sign in again. | Sua sessão terminou. Entre de novo. |
| `error.CSRF_REJECTED` | The page was out of date. Try again. | A página estava desatualizada. Tente de novo. |
| `error.FORBIDDEN` | Your role does not allow this. | Seu papel não permite esta ação. |
| `error.USER_NOT_FOUND` | No such user was found. | Usuário não encontrado. |
| `error.NOT_FOUND` | Nothing was found at this address. | Nada foi encontrado neste endereço. |
| `error.METHOD_NOT_ALLOWED` | This action is not available here. | Esta ação não está disponível aqui. |
| `error.USERNAME_TAKEN` | This user name is already taken. | Este nome de usuário já está em uso. |
| `error.LAST_ADMIN` | The last active administrator cannot be removed. | O último administrador ativo não pode ser removido. |
| `error.SELF_DELETE` | You cannot delete your own account. | Você não pode excluir a sua própria conta. |
| `error.SELF_DEACTIVATE` | You cannot deactivate your own account. | Você não pode desativar a sua própria conta. |
| `error.SELF_DEMOTE` | You cannot lower your own role. | Você não pode rebaixar o seu próprio papel. |
| `error.INVALID_USERNAME` | This user name is not valid. | Este nome de usuário não é válido. |
| `error.PASSWORD_TOO_SHORT` | The password must have at least 12 characters. | A senha precisa ter pelo menos 12 caracteres. |
| `error.PASSWORD_TOO_LONG` | The password must have at most 1024 characters. | A senha pode ter no máximo 1024 caracteres. |
| `error.AUTH_RATE_LIMITED` | Too many failed attempts. Try again in {{seconds}} seconds. | Muitas tentativas falharam. Tente de novo em {{seconds}} segundos. |
| `error.DATABASE_UNAVAILABLE` | The database is unavailable. Try again shortly. | O banco de dados está indisponível. Tente de novo em instantes. |
| `error.PROFILE_NOT_FOUND` | No such profile was found. | Perfil não encontrado. |
| `error.PROFILE_ID_TAKEN` | This profile id is already taken. | Este id de perfil já está em uso. |
| `error.PROFILE_REVISION_STALE` | This profile changed since you opened it. Load it again. | Este perfil mudou desde que você o abriu. Carregue-o de novo. |
| `error.PROFILE_DOCUMENT_INVALID` | The profile is not valid. | O perfil não é válido. |
| `error.ENDPOINT_URL_REJECTED` | The address of the metric source is not allowed. | O endereço da fonte de métricas não é permitido. |
| `error.ENDPOINT_ADDRESS_REJECTED` | The metric source resolves to an address that is not allowed. | A fonte de métricas aponta para um endereço que não é permitido. |
| `error.INTERNAL_ERROR` | Something went wrong on the server. | Algo deu errado no servidor. |
| `error.IDEMPOTENCY_KEY_INVALID` | The request could not be identified. Try again. | Não foi possível identificar o pedido. Tente de novo. |
| `error.CURSOR_INVALID` | The list could not continue. Load it again. | A lista não pôde continuar. Carregue-a de novo. |
| `error.REPORT_NOT_FOUND` | No such report was found. | Relatório não encontrado. |
| `error.IDEMPOTENCY_KEY_REUSED` | This request was already sent with other values. | Este pedido já foi enviado com outros valores. |
| `error.PROFILE_INACTIVE` | This profile is not active. | Este perfil não está ativo. |
| `error.SNAPSHOT_NOT_READY` | This report's data is not ready yet. | Os dados deste relatório ainda não estão prontos. |
| `error.PDF_NOT_READY` | This PDF is not ready yet. | Este PDF ainda não está pronto. |
| `error.PDF_RETRY_NOT_ALLOWED` | This PDF cannot be generated again now. | Este PDF não pode ser gerado de novo agora. |
| `error.REPORT_EXPIRED` | This report has expired. | Este relatório expirou. |
| `error.REPORT_SELECTION_NOT_PERMITTED` | The profile does not permit this selection. | O perfil não permite esta seleção. |
| `error.ADMISSION_GLOBAL_LIMIT` | The platform is generating as many reports as it can. Try again in {{seconds}} seconds. | A plataforma já está gerando o máximo de relatórios. Tente de novo em {{seconds}} segundos. |
| `error.ADMISSION_PROFILE_LIMIT` | This profile is generating as many reports as it can. Try again in {{seconds}} seconds. | Este perfil já está gerando o máximo de relatórios. Tente de novo em {{seconds}} segundos. |

If the enum of Task 3 holds a code that this table lacks, stop and report it. The main thread words it.

#### The rules of the wording files

- `families.ts` declares each key family with the run-time list its members must equal:
  - `error.*` = the generated values of `ErrorCode` in `schema.gen.ts`, plus `general` and `network`;
  - `role.*` = the generated values of `Role`;
  - `language.*` = the generated values of `Language`;
  - `status.*` = `statusValues` of `status-mark.tsx`;
  - `theme.*` = `["system", "light", "dark"]`.
- A key outside a family counts as used when its full literal appears in a `.ts` or `.tsx` file under `src/`.
- `vocabulary.ts` exports `findBannedTerms(lang, text): string[]`, the port of R177.

- [ ] **Step 1: Write the failing tests**

| Test | Input | Expected |
|---|---|---|
| `contrast: every pair meets its minimum` | `theme.css` parsed for both themes; `pairs.ts` | every ratio ≥ its `min`, computed by the WCAG 2.2 relative-luminance formula; a failure names the pair, the theme, and the ratio |
| `contrast: every token has a dark twin` | `theme.css` | the same token names under `:root` and `[data-theme="dark"]` |
| `locales: same keys, same parameters` | both files | equal key sets; for each key, equal sets of `{{…}}` names |
| `locales: families are exact` | `families.ts` | each family's keys equal its list, both ways |
| `locales: every key is used` | `src/**` | each non-family key's literal appears in source |
| `locales: product name` | both | `app.short + " - " + app.long === app.name`, in both files |
| `vocabulary: no banned term in a locale` | every leaf of both files | `findBannedTerms` empty, each in its own language |
| `vocabulary: the finder` | `"o limite de CPU"` in pt-BR; `"cpu"` in en; `"node group"` in en; `"{{limite}} ok"` in pt-BR | `["limite"]`; `["cpu"]`; `["node group"]`; `[]` |
| `language: browser default` | `navigator.languages` `["pt-PT"]`; `["PT"]`; `["en-US"]`; `[]` with `navigator.language` `"pt-BR"`; `["de"]` | `pt-BR`; `pt-BR`; `en`; `pt-BR`; `en` |
| `language: stored choice wins before sign-in` | storage `kcr.language` = `en`, browser `pt-BR`; storage `fr` | `en`; the browser's |
| `storage refused` | `localStorage.getItem` and `setItem` throw | language and theme functions return the default, keep a choice in memory for the page, and throw nothing |
| `theme: boot script` | `theme-boot.js` evaluated in jsdom with `kcr.theme` = `dark`; `light`; absent and `matchMedia` dark; absent and `matchMedia` light; `bogus` | `data-theme` `dark`; `light`; `dark`; `light`; resolves as system |
| `theme: toggle` | the toggle chooses `dark`, then `system` | `data-theme` `dark` and stored `dark`; then follows `matchMedia`, stored `system` |
| `status mark` | each of the five statuses, in each language | the label's text from `status.*`; an `svg` with `aria-hidden="true"` and the shape's element (`circle` filled, `polygon` of 3 points, `polygon` of 4 points, `circle` with no fill, `line`); the label bold only for critical |
| `no style element` | open the `menu.tsx` dropdown (modal), then close it | no `<style>` element in `document` at any point; `body[data-scroll-locked]` set while open, removed after |
| `ui: axe` | each primitive rendered in both themes (`data-theme` set) | `axe.run` reports no violation |

- [ ] **Step 2:** Run `cd web && npm test`. Expected: FAIL.
- [ ] **Step 3:** Implement and transcribe. Show `contrast` red by changing `muted-fg` light to `#9aa1ad` in a scratch copy, then restore it. Show `vocabulary` red by adding `limite` to a pt-BR entry, then restore it.
- [ ] **Step 4:** Run `make web`, `make test`, and `make lint`. All pass. Run `ls web/dist/assets/*.woff2 | wc -l`: it prints `4`.
- [ ] **Step 5:** Commit `feat(web): add the theme, wording, and marks` (44 characters).

**Done when:**
- every row passes;
- the two mutations of Step 3 were seen red;
- the build carries exactly the four font files.

---

### Task 7: Session, shell, and the three screens (R168, R169, R174, R178)

**Executor:** Subagent. Follows Tasks 5 and 6. Opus reviews it (CSRF, `next`, the 401 path).

**Files:**
- Create:
  - API layer: `web/src/api/client.ts`, `errors.ts`, `client.test.ts`;
  - session: `web/src/session/session.ts`, `require-session.tsx`, `session.test.tsx`;
  - app: `web/src/app/router.tsx`, `shell.tsx`, `providers.tsx`, `live-region.tsx`, `not-found.tsx`, `title.ts`;
  - screens: `web/src/features/auth/login-page.tsx`, `login-page.test.tsx`; `web/src/features/account/account-page.tsx`, `account-page.test.tsx`;
  - test support: `web/tests/msw.ts` (handlers typed by `schema.gen.ts`), `web/tests/screens.test.tsx`.
- Modify: `web/src/app/app.tsx`

**Interfaces:**

```ts
// session.ts
export const sessionKey = ["session"] as const;
export function useSession(): UseQueryResult<Session>;
export function currentCsrf(client: QueryClient): string | null; // login_csrf or csrf_token
export function safeNext(raw: string | null): string;             // R178 and Review Focus 2

// client.ts
export type ApiFailure = { status: number; code: ErrorCode | null; retryAfter: number | null };
export function createApi(queryClient: QueryClient, navigate: (to: string) => void): Api;
// Api exposes the openapi-fetch methods; every call resolves to { data } or rejects with ApiFailure.

// errors.ts
export function errorText(t: TFunction, failure: ApiFailure | Error): string;
// error.<CODE> with {seconds} when retryAfter is set; error.network on a fetch failure; error.general otherwise.
```

**What each screen and piece does:**

- **Shell:**
  - A skip link (`app.skip`) to `#main`.
  - The sidebar: the name of R174, the navigation `nav.label` holding `nav.account`, and, at the bottom, the user name and the role.
  - The header, on the right: the language menu (`header.language`, items by endonym), the theme menu (`header.theme`, three radio items), and `header.signOut`.
  - `<main id="main" tabIndex={-1}>`, and one polite live region.
  - The title of every screen is set through `title.ts` with `app.title`.
  - The user name in the sidebar and the version on the account screen are set in `font-mono`.
- **Sign-in (`/login`):**
  - It has no sidebar, and the header shows the name and the language menu.
  - The card holds `login.title`, `login.subtitle`, the two fields, and the submit button.
  - Notices: `login.passwordChanged` when `?changed=1`, and `login.sessionEnded` when `?ended=1`, as `info` alerts.
  - Errors: a `critical` alert with `role="alert"`, worded by `errorText`.
  - The submit is disabled while pending and shows `login.submitting`.
  - On success, the session entry is set from the answer, and the screen navigates to `safeNext(next)`.
- **Account (`/account`):**
  - `account.signedInAs`, then a language section of two choices, then a password section of three fields.
  - Zod checks: required, 12 to 1024 characters, and the two new entries equal.
  - On `200` for the password, it clears the cache and navigates to `/login?changed=1`.
  - On `401 INVALID_CREDENTIALS`, the field error goes on `account.password.current`. On `422`, the policy text appears. On `429`, the wait is shown.
  - The footer reads `app.version`.
- **Not found:** inside the shell when signed in. Its link goes to `/`.
- **Sign out:** `POST /auth/logout`, then clear the cache and navigate to `/login`.

- [ ] **Step 1: Write the failing tests** (MSW for the API; Testing Library; `axe.run` on every screen)

| Test | Input | Expected |
|---|---|---|
| `client: csrf header` | `PATCH /account` after a signed-in session | header `X-CSRF-Token` equals the session's `csrf_token`; a `GET` carries none |
| `client: login uses the sign-in token` | anonymous session, `POST /auth/login` | header equals `login_csrf` |
| `csrf retries once` | `PATCH /account` answers 403 `CSRF_REJECTED`, then the session answers a new token, then 200 | two `PATCH` calls, the second with the new token; resolves with data |
| `csrf gives up after one retry` | two 403 `CSRF_REJECTED` in a row | exactly two `PATCH` calls and one session refetch; rejects with `code: "CSRF_REJECTED"` |
| `401 opens sign-in` | `GET /auth/session` signed in, then any `GET` answers 401 | cache cleared; navigated to `/login?next=%2Faccount&ended=1` |
| `401 of a wrong password stays on the form` | `PATCH /account` answers 401 `INVALID_CREDENTIALS` | no navigation; the error on the current-password field |
| `next is local only` | `safeNext` of `/account`, `/account?x=1#y`, `//evil.example`, `/\evil.example`, `https://evil.example`, `javascript:alert(1)`, `%2F%2Fevil.example`, `""`, `null` | `/account`, `/account?x=1#y`, then `/` for each of the rest |
| `login: success` | valid credentials | navigates to `next` or `/`; the session entry holds the principal |
| `login: wrong credentials` | 401 `INVALID_CREDENTIALS` | the alert holds `error.INVALID_CREDENTIALS` in the current language; focus moves to the alert |
| `login: rate limited` | 429 `AUTH_RATE_LIMITED`, `Retry-After: 120` | the alert holds `error.AUTH_RATE_LIMITED` with `120` |
| `login: language before sign-in` | browser `pt-BR`; the menu chooses English | the card is in Portuguese, then in English; `localStorage` `kcr.language` = `en`; `<html lang>` follows |
| `login: account language applies after sign-in` | stored `en`, the principal's language `pt-BR` | after sign-in, Portuguese; `kcr.language` removed |
| `account: language` | signed in `en`; choose Português | one `PATCH /account {"language":"pt-BR"}`; the screen in Portuguese; the live region announces `account.language.saved` in Portuguese |
| `account: password policy` | new password of 11 characters; two entries that differ | `validation.passwordShort`; `validation.passwordsDiffer`; no request |
| `account: server policy` | 422 `PASSWORD_TOO_SHORT` | `error.PASSWORD_TOO_SHORT` shown |
| `password change signs out` | 200 to the password change | cache cleared; navigated to `/login?changed=1`; the notice shown; no error alert |
| `sign out` | the header button | `POST /auth/logout` with the token; navigated to `/login` |
| `routes` | anonymous `/account`; signed-in `/login`; signed-in `/`; signed-in `/nothing` | `/login?next=%2Faccount`; `/account`; `/account`; the not-found screen inside the shell |
| `title` | each screen, each language | `document.title` is `app.title` with the screen's title |
| `screens: axe and vocabulary` | each of sign-in, account, and not found × each language × each theme | `axe.run` has no violation; `findBannedTerms` of the visible text and of every accessible name (`aria-label`, label text, button names) is empty in that language |

- [ ] **Step 2:** Run `cd web && npm test`. Expected: FAIL.
- [ ] **Step 3:** Implement.
- [ ] **Step 4: Run it by hand.**
  1. Start `make postgres-up`.
  2. Run `serve` natively with an administrator bootstrapped, as in Task 5.
  3. Run `npm run dev` and `npm run build`, and open both `vite` and the built binary in a browser.
  4. In each, sign in, switch the language, switch the theme, change the password, and sign out.
  5. Write the transcript, with one screenshot per screen, theme, and language, to `.cache/phase-4/part-1/manual/screens/`.
  6. Stop the processes that hold the ports.
- [ ] **Step 5:** Run `make test` and `make lint`. All pass.
- [ ] **Step 6:** Commit `feat(web): add sign-in, account, and the shell` (46 characters).

**Done when:**
- every row passes and the screenshots exist;
- the mutation that drops the retry turns `csrf retries once` red;
- the mutation that accepts `//` in `safeNext` turns `next is local only` red.

---

### Task 8: Compose and the end-to-end spec (R179, R180)

**Executor:** Subagent writes the files. The main thread runs Docker and iterates with the subagent. Follows Task 7.

**Files:**
- Create: `deploy/compose.yaml`, `deploy/compose.integration.yaml`, `deploy/platform/platform.toml`, `deploy/platform/server-secret`, `deploy/platform/e2e-admin-password`, `deploy/platform/README.md`, `web/playwright.config.ts`, `web/e2e/sign-in.spec.ts`
- Modify: `Makefile` (`e2e` target, gate step)

**What each file does:**
- **`deploy/platform/platform.toml`:** the defaults, with `server_secret_path = "/run/kcr/server-secret"` and `trusted_proxies = []`. No allowlist entry in this part.
- **`server-secret`:** 32 bytes from `/dev/urandom`, written once and committed, with the README naming it a test fixture.
- **`e2e-admin-password`:** a fixed 20-character password, also a test fixture.
- **`deploy/platform/README.md`:** a few lines. It says these files serve the integration stack and must never be used for a deployment.
- **`compose.yaml`:**
  - `postgres:18-alpine@sha256:77f585114c32fbca283dc835b0596f4e52b51b4c6662d7810b2f4084f60a1873`, with a named volume and a health check.
  - `serve` and `worker` run the image of R179 with `KCR_DATABASE_DSN`, `serve` listening on `:8080` and published on `127.0.0.1:8080`.
  - Health checks: `["CMD", "/usr/local/bin/k8s-capacity-report", "admin", "probe", "http://127.0.0.1:8080/health/ready"]` for `serve`, and port 8091 for `worker`.
- **`playwright.config.ts`:**
  - `testDir: "e2e"`, `baseURL: "http://localhost:8080"`;
  - `projects: [{ name: "chromium" }]`, `retries: 0`, `workers: 1`;
  - `reporter: [["list"], ["html", { outputFolder: "/results/report", open: "never" }]]`, `outputDir: "/results/artifacts"`.
- **`sign-in.spec.ts`:**
  - One test: open `/account` while anonymous and get sent to sign-in. Sign in as `e2e-admin`. Switch to Português and see `Conta`, then switch back to English.
  - Change the password and land on sign-in with the notice. Sign in with the old password and see `INVALID_CREDENTIALS` worded. Sign in with the new one, then sign out.
  - A fixture fails the test on any console message containing `Content-Security-Policy`, and on any `securitypolicyviolation` event. The page installs a listener for that event through `page.addInitScript` from a file, not inline code.
  - `AxeBuilder` runs on each of the three screens and must report no violation.

- [ ] **Step 1:** The subagent writes the files and runs `docker compose -f deploy/compose.yaml -f deploy/compose.integration.yaml config`. That check needs no engine; if the CLI refuses without one, the main thread runs it.
- [ ] **Step 2 (main thread):** `make e2e`. The spec passes. Keep the log under `.cache/phase-4/part-1/e2e/`.
- [ ] **Step 3 (main thread): Show the spec can fail.**
  - Add `style="color:red"` to `index.html` in a scratch build: the CSP fixture fails the run.
  - Remove a label from the sign-in form: axe fails the run.
  - Restore both, and record the failures.
- [ ] **Step 4:** Commit `test(e2e): sign in through the Compose stack` (44 characters).

**Done when:** `make e2e` passes on OrbStack, and the two failures of Step 3 were seen.

---

### Task 9: The combined run and the owner's look

**Executor:** Main thread.

- [ ] **Step 1:** Run `make test`, `make lint`, `make generate-check`, `make test-postgres`, `make image`, `make check-runtime`, and `make e2e` over the combined branch. Logs go under `.cache/phase-4/part-1/combined/`.
- [ ] **Step 2:** Show the screenshots of Task 7 to the owner in the companion: sign-in, account, and not found, in both themes and both languages. A change the owner asks for is made by the main thread (tokens, wording) or sent back to Task 7's implementer (layout), with the screenshots taken again.

**Done when:** every command passes and the owner has seen the screens.

---

### Task 10: The documents, review, gate, close

**Executor:** Main thread.

- [ ] **Step 1: `AGENTS.md`.**
  - The layers table gains `web` at rank 0.
  - "Commands" gains `make web` and `make e2e`.
  - "Delegating to subagents" adds `make e2e` to the steps kept on the main thread, and `npm` inside `web/` to those a subagent may run.
  - A new section, "The interface", holds the rules of this part:
    - the wording comes only from the locale files, with the families and the parity test;
    - the banned-terms file is shared;
    - token pairs and the contrast test;
    - no inline style and no `<style>` element, with the CSP unchanged until a hash enters with its test;
    - status as shape, label, then colour;
    - the static fallback and its exceptions;
    - the session rules of spec 12.6;
    - no skipped or focused test, by lint;
    - exact versions with a committed lockfile.
  - "The download pool" joins "The platform": R164.
- [ ] **Step 2: `README.md`.**
  - The development loop: `make postgres-up`, `serve`, then `npm run dev`.
  - `make web` and `make e2e`.
  - The Compose files, and the TLS note.
- [ ] **Step 3: `CHANGELOG.md`, under Unreleased, `### Added`:**
  - the web interface served by `serve`, with sign-in and an account screen where a reader changes their password and language;
  - the interface in Portuguese (Brazil) and English, following the browser's language before sign-in and the account's after;
  - light and dark themes that follow the system unless the reader chooses one;
  - a Compose file that runs `serve`, `worker`, and PostgreSQL.

  `### Fixed`: `serve` no longer keeps the memory of a large download on idle database connections.

  Omitted, and said so in the report: the error-code enum of the contract, the banned-terms file, the lint rules, the end-to-end harness, and the integration Compose file.
- [ ] **Step 4: The adversarial review of the branch, by areas.** Probes go under `.cache/phase-4/part-1/review/probe/`:
  - (a) the static handler and its fallback, path cleaning, types, and caching;
  - (b) the session, CSRF, `next`, and the 401 and 403 paths in the browser code;
  - (c) the download pool and the slots;
  - (d) wording, vocabulary, contrast, and accessibility.

  One fix wave follows, then one re-review.
- [ ] **Step 5:** Run `make gate` on OrbStack, with logs under `.cache/phase-4/part-1/gate/`. Read that the frontend tests, `TestServeServesTheInterface`, `TestDownloadPoolKeepsNoIdleConnection`, and the end-to-end spec are named in the logs.
- [ ] **Step 6: Close.**
  - Update `ROADMAP.md` and close F-110, F-111, and F-127.
  - Write the closing report `.cache/phase-4/part-1/closing-report.md`.
  - Get the owner's word on the merge by fast-forward and the push. No version: part 4.1 merges without one unless the owner says otherwise.

**Done when:** the gate passes from images built without cache, the review is clean, and the owner has the closing report.
