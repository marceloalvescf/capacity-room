# Phase 4.5: Administration, Profiles and Users Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** An administrator manages the metric-source profiles and the users of the platform from the web: lists them, opens a row in place, creates, edits, tests a profile before saving it, deletes a profile, and changes, deactivates, or reactivates a user.

**Architecture:** One new backend route, `POST /admin/profiles/test`, runs the existing preflight on a draft document; everything else uses the routes of spec 11.10 that exist. The frontend adds four screens behind an administrator guard. The report's `DataTable` moves to `components/` and becomes the one table of the app, with the window fit built into it; the profile form is React Hook Form with Zod, with two folded sections, and a leave guard built on React Router's `useBlocker`.

**Tech Stack:** As in part 4.4, with no new dependency. Go 1.27; React 19.3.0, React Router 8.4.0 (data router), TanStack Query 5.104.1, TanStack Table 9.2.6, React Hook Form 7.89.0 with `@hookform/resolvers` 5.9.1 and Zod 4.6.5, Radix UI 1.7.0, Tailwind CSS 4.3.3, react-i18next 17.0.16, openapi-typescript 7.13.0; Vitest with MSW; Playwright.

**Spec:** `docs/superpowers/specs/2026-09-27-k8s-capacity-report-design.md` as amended in `0ea5daa` (approved by the owner on 2026-10-09): sections 11.6, 11.10, 12.2.1 (the fit line), and the new 12.2.2. The owner's answers are in `.cache/phase-4/part-5/brainstorm-decisions.md` (1 to 13). The mockups they approved are in `.cache/phase-4/brainstorm/.superpowers/brainstorm/16696-1791545815/content/`: `q3-profile-form.html` (form 4C), `q4-profile-list.html` (list 5B), `q5-users.html` (users 6A). Their source is `.cache/phase-4/part-5/mock/src/`; an implementer may read it for layout and classes. The words in this plan win over the mock's.

**Depends on:** `main` at `2f0e020` (release 0.5.0). Branch `feat/administration`, which holds the spec amendment `0ea5daa`.

## Global Constraints

Every constraint of the plans of phases 0 to 4.4 stands, and `AGENTS.md` binds every task. The ones this part leans on most:

- **Every command runs inside `nix develop`** from the repository root. The host is NixOS: only `/bin/sh` and `/usr/bin/env` exist under `/bin` and `/usr/bin`.
- **No backward compatibility, and stored data is junk before 1.0**: no fallback, alias, or migration for a renamed key, path, or prop.
- **Every text the reader sees, and every accessible name, comes from `web/src/locales/{en,pt-BR}.json`** through `t()`. Each task carries its words in its own table; an implementer transcribes and never invents one. A word missing from a table is a stop-and-report.
- **Technical terms are English in both languages** (endpoint, label, node, node pool, autoscaling, replicas, request, limit, CA bundle, timeout); the vocabulary test enforces `internal/catalog/banned-terms.json` in both files.
- **Diagnostics are English and fixed**: an error of the API is a code; the frontend words the code.
- **Status is a shape, then a label, then a colour**, with `StatusMark`'s five shapes.
- **Names and numbers use the monospaced font** (identifiers, user names, cluster names, endpoints, file names, namespaces, list text boxes, numbers).
- **No inline style**: no `style` prop, no `dangerouslySetInnerHTML`, no `<style>` added at run time; every new Radix layer joins `web/src/components/no-style.test.tsx`; the content of a layer sits at `z-50`.
- **Contrast**: a new pair of tokens drawn together enters `web/src/theme/pairs.ts`.
- **The contract is `api/openapi.yaml`**; `make generate` writes both type files and `make generate-check` passes; the route table and the contract agree both ways.
- **The checks of a route run in one order**: session (401), role (403), CSRF (403), body (400), then the service.
- **Every npm dependency keeps an exact version**; no dependency is added by this part.
- **Commits:** `type(scope): description`, imperative, under 50 characters (each subject below is counted); stage with `git add <paths>`.
- **Comments say what the code does**; none names a task, step, ruling, brief, finding, or `.cache/` path.
- Scratch goes under `.cache/phase-4/part-5/`.

## Review Focus

Each line names an input the spec implies and no requirement spells out, and the test that pins it.

1. **A profile edited by someone else while the form is open.** The save answers `PROFILE_REVISION_STALE`; the form keeps every typed value and says so; loading the latest revision asks first when the form has changes. Pinned in Task 6 (`stale revision keeps the values`, `load latest asks first`).
2. **An API refusal on a field the reader cannot see** (a folded section, or `thresholds`, which the API names as one field). The folded section opens, the message shows in it, and the focus moves to the first field in error. Pinned in Task 5 (`refusal opens the folded section`).
3. **The last row of a long table opened near the bottom of a short window.** The row and all it holds scroll into view, and nothing scrolls sideways. Pinned in Task 2 (`opening reveals the row`) and Task 7 (the fit check in a real browser at three sizes).
4. **An administrator acting on their own account.** The own row offers no role, password, or deactivate control; `LAST_ADMIN` from another row is worded in that row. Pinned in Task 3 (`own row`, `last administrator`).
5. **A draft whose values change after a test.** The result no longer describes the form, so it is cleared on the first change. Pinned in Task 6 (`a change clears the result`).

## Where this part sits

| Part | Delivers | Proof at the end |
|---|---|---|
| 4.1 to 4.4 | Foundation, report screens, calendar ranges, the report | Done, released as 0.3.0 to 0.5.0 |
| **4.5 Administration (this plan)** | Profiles and users on the web; the draft test; one table with the window fit | In a real browser: an administrator creates a user and a profile, tests the profile, edits and deletes it; every opened row fits the window at three sizes |
| 4.6 End-to-end | Security, concurrency, PDF recovery specs | `make e2e` runs the four specs of spec 13.4 |

### Out of this part

| Left out | Reason |
|---|---|
| A history of revisions | No route reads past revisions; the spec names none |
| A listing of the credential and CA files | The names of secret files stay out of the API (decision 3 of the brainstorm) |
| A merge of two edits of one profile | The owner chose A on question 7 |
| A leave guard on the users screen | Its edits are saved by their own buttons (decision 7) |
| F-143 except its two table items; F-132 to F-136, F-137's rest, F-140 | Their files are not touched by this part |

## How to read this plan

As in parts 4.2 to 4.4: each task gives exact paths, interfaces, its words, and a table of tests with inputs and expected results. Write each test first, watch it fail, then implement.

Executors:
- **Subagent:** a Sonnet subagent in the main checkout, no worktree. Every command inside `nix develop`. It may run `go`, `make build`, `make test`, `make lint`, `make generate`, `make generate-check`, `make web`, and `npm` inside `web/`. It never runs Docker, `make e2e`, `make postgres-up`, `make test-postgres`, or `make update-*`, and cannot declare a task finished. For a test tagged `postgres`, the main thread runs `make postgres-up` and gives the subagent the DSN, and the subagent runs `KCR_TEST_DATABASE_DSN=<dsn> go test -count=1 -tags=postgres <packages>`.
- **Main thread:** Docker, judgment, wording, design, and the owner.

Rules for every brief:
- Tasks run in sequence. A reviewer reads a diff file and runs tests only on packages nobody edits.
- Stage with `git add <paths>`; never stash, restore, check out a path, or clean.
- A test of a change goes red first, on the code before the change; a test row that passed before its code existed is proven by a mutation.
- The brief appends this plan's rulings for its task, the Global Constraints, and the Review Focus rows its task owns.

**Order:** Task 1 → Task 2 → Task 3 → Task 4 → Task 5 → Task 6 → Task 7 → Task 8. Tasks 2 to 7 share the locale files, `families.ts`, the router, `tests/msw.ts`, `tests/screens.test.tsx`, and `no-style.test.tsx`; Task 1 writes `web/src/api/schema.gen.ts`, which Tasks 4 to 6 read.

## Rulings made on the owner's behalf

Each ruling is a decision the amended spec leaves open: decision, grounds, cost if wrong. The numbering continues from R231.

| ID | Ruling |
|---|---|
| R232 | The draft test in the contract. Path `/admin/profiles/test`, `post`, `operationId: testProfileDraft`, summary "Run the preflight of a report against the source of a draft profile, through the outbound policy, before it is saved; answer the outcome as a code." Parameters: `CSRFHeader`. Body `ProfileDraft`: `allOf` `ProfileDocument` and an object with `required: [display_name, endpoint_url, cluster_name, allowed_namespaces]` and the property `id` (string, maxLength 63, "The identifier of the profile the draft belongs to, when it has one; it names the target of the audit row."). Responses: `200` `ProfileTestResult` (description as the saved test's), `400` `RequestInvalid`, `401` `Unauthenticated`, `403` `ForbiddenOrCSRF`, `422` `ProfileRefused`, `default` `Unexpected`. A new enum schema `ProfileTestCode`, in this order: `PROFILE_OK`, `ENDPOINT_URL_REJECTED`, `ENDPOINT_ADDRESS_REJECTED`, `ENDPOINT_RESOLUTION_FAILED`, `ENDPOINT_CONNECTION_REJECTED`, `ENDPOINT_REDIRECT_REJECTED`, `PROFILE_CREDENTIAL_UNAVAILABLE`, `PROFILE_CA_BUNDLE_UNAVAILABLE`, `ENDPOINT_TLS_FAILED`, `ENDPOINT_AUTH_REJECTED`, `ENDPOINT_API_NOT_FOUND`, `ENDPOINT_UNREACHABLE`, `CLUSTER_SCOPE_NOT_FOUND`, `CAPABILITIES_INSUFFICIENT`; `ProfileTestResult.code` becomes `$ref: ProfileTestCode` (the description's list of codes is removed from the field; the enum is the list). `ProfileDraft` joins `exclude-schemas` in `api/oapi-codegen.yaml` (the handler decodes into the generated `ProfileCreate`, R233); `ProfileTestCode` is generated. `internal/platform/profiles` exports `var TestCodes = []string{…}` in the enum's order, the five policy codes taken from `netpolicy`; `TestProfileTestCodesEqualTheService` (no build tag, in `internal/platform/api/contract_test.go`) reads the enum through `loadSchemas` and compares it with `profiles.TestCodes` as an ordered list. Grounds: spec 11.6, 11.10; AGENTS ("the frontend's types come from the contract"). Cost if wrong: contract text. |
| R233 | The draft test in Go. `func (s *Service) TestDraft(ctx context.Context, actor auth.Principal, id string, d Document) (TestResult, error)`, in `internal/platform/profiles/test.go`: `requireAdministrator`; when `id != ""` and `!ValidReferenceName(id)`, `&DocumentError{Problems: []Problem{{Field: "id", Word: "invalid"}}}`; `s.checkDocument(&d)` (problems and policy refusals return as the create returns them); `s.runTest(ctx, d)`; then one `audit.Append` on `context.WithoutCancel(ctx)` with `ActorID: &actor.ID`, `Action: ActionProfileTest`, `TargetType: targetProfile` and `TargetID: id` when `id != ""` (both empty otherwise), `Outcome` succeeded or failed as `Test` sets it, `Metadata{"code": result.Code, "draft": true}`. It reads and writes no profile row. The handler `testProfileDraft` in `internal/platform/api/profiles.go` decodes into `ProfileCreate` with `DecodeBody(r, &body, []string{"display_name", "endpoint_url", "cluster_name", "allowed_namespaces"}, profileLimits)`, builds the document with `documentOf(body)`, calls `TestDraft(ctx, principal, body.Id, doc)`, answers `failProfile` on error and `200` with `ProfileTestResult{Ok, Code: ProfileTestCode(result.Code), Capabilities}` (an empty slice, never null), as `testProfile` does. The route `{"POST", "/api/v1/admin/profiles/test", s.adminOnly(s.requireCSRF(s.testProfileDraft))}` enters `routes.go` beside the saved test. Grounds: one validation for create and draft; the saved test's audit, with the draft marked. Cost if wrong: one function. |
| R234 | The table moves. `web/src/features/report/data-table.tsx` → `web/src/components/data-table.tsx` (its test → `web/src/components/data-table.test.tsx`), `web/src/features/report/address.ts` → `web/src/lib/address.ts`, `web/src/features/report/search-field.tsx` → `web/src/components/search-field.tsx`; every importer changes its import; the old files are removed. `ReportColumn` is renamed `Column`. The props `searchLabel` and `searchText` become one optional prop `search?: { label: string; text: (row: Row) => string }`: without it there is no search field, no `q` in the address, and no row count line. A new required prop `empty: string` is the sentence shown for no rows (the report tabs pass `t("report.table.empty")`). The keys `report.table.rows`, `.noMatch`, `.open`, `.close`, `.details` move to `table.rows`, `table.noMatch`, `table.open`, `table.close`, `table.details`, with the same words; `report.table.empty` stays. Grounds: the owner chose one table (decision 11); AGENTS (no compatibility layer). Cost if wrong: imports only. |
| R235 | The fit, in `DataTable`. (a) Vertical: when the opened key changes to a row that is rendered (a click, or the address on load), after layout, the table reveals the pair of rows (the row and its opened row): with `head` the height of the sticky `thead`, `top` the row's top and `bottom` the opened row's bottom in the viewport, if `bottom > innerHeight - 8` it scrolls by `min(bottom - innerHeight + 8, top - head - 8)`; else if `top < head + 8` it scrolls by `top - head - 8`. The scroll is `behavior: "smooth"` when `matchMedia("(prefers-reduced-motion: no-preference)")` matches, `"auto"` otherwise. Closing never scrolls. The function `revealRows(row: HTMLElement, opened: HTMLElement, head: number): void` lives in `web/src/components/reveal.ts` so it is tested alone. (b) Horizontal: every `td` of the table carries `[overflow-wrap:anywhere]`; the opened row's cell holds its content in a `div` with `min-w-0`. A grid of facts inside an opened row uses `grid-cols-[repeat(auto-fit,minmax(14rem,1fr))]`. Grounds: spec 12.2.2, last line; the owner's clarification (decision 10). Cost if wrong: one function and classes; proven in a real browser in Task 7. |
| R236 | F-143's two table items. (a) A third click on a sorted header removes the sort: `sort` leaves the address and the default order returns (the test is new; the behaviour is TanStack's cycle). (b) When the search text changes, and only then (not on the first render), the table announces `table.rows` with the count of rows shown through `useAnnounce`, after the rows render. Grounds: F-143; the table is opened anyway. Cost if wrong: one effect. |
| R237 | The dialog. `web/src/components/ui/dialog.tsx` exports `Dialog({ open, onOpenChange, title, children, onCloseAutoFocus? })` on Radix `Dialog` from `radix-ui`: `Dialog.Portal`, an overlay `fixed inset-0 z-50 bg-fg/40`, content `fixed top-1/2 left-1/2 z-50 flex max-h-[calc(100vh-2rem)] w-[min(36rem,calc(100vw-2rem))] -translate-x-1/2 -translate-y-1/2 flex-col gap-4 overflow-y-auto rounded-md border border-border bg-surface p-6 text-surface-fg shadow-md`, `aria-describedby={undefined}`, a `Dialog.Title` `text-lg font-semibold`, then the children. Escape and the overlay close it through `onOpenChange(false)`. It joins `no-style.test.tsx` (open, check, close). Grounds: the owner chose 6A (decision 6); the confirmation dialog's look. Cost if wrong: one component. |
| R238 | The text box. `web/src/components/ui/textarea.tsx` exports `TextArea` with `Field`'s API (`label`, `hint`, `error`, `id` from `useId`, `aria-invalid`, `aria-describedby`) over a `<textarea>` with the classes of `Field`'s input plus `py-2 font-mono`, and `rows` passed through. A list field converts with `toLines(text: string): string[]` (split on `\n`, trim each line, drop empty lines) and `fromLines(items: readonly string[]): string` (join with `\n`), both in `web/src/features/admin/profiles/lines.ts`. Duplicates are left to the API (`repeated`). Grounds: decision 3 (one item per line, pasted from a list). Cost if wrong: one component. |
| R239 | The guard and the routes. `web/src/session/require-administrator.tsx` exports `RequireAdministrator`, written as `RequireOperator` with the test `principal.role === "administrator"`. In `router.tsx`, inside the shell, a sibling of `generate`: `{ element: <RequireAdministrator />, children: [{ path: "profiles", element: <ProfilesPage /> }, { path: "profiles/new", element: <ProfileFormPage /> }, { path: "profiles/:id/edit", element: <ProfileFormPage /> }, { path: "users", element: <UsersPage /> }] }`. The shell's navigation lists, for an administrator only, `nav.profiles` and `nav.users` after `nav.generate` and before `nav.account`. Screen titles: `admin.profiles.title`, `admin.profile.newTitle`, `admin.profile.editScreen`, `admin.users.title`. Grounds: spec 12.2, 12.2.2. Cost if wrong: none found. |
| R240 | Queries. `web/src/features/admin/profiles/api.ts`: `adminProfilesKey = ["admin", "profiles"]`; `useAdminProfiles()` (`GET /admin/profiles`), `useAdminProfile(id)` (key `[...adminProfilesKey, id]`, `GET /admin/profiles/{id}`), mutations `createProfile`, `reviseProfile`, `deleteProfile`, `testProfile(id)`, `testDraft(body)`. Every successful create, revise, and delete invalidates `adminProfilesKey` and `profilesKey` (`["profiles"]`, the summaries the new report form reads). `web/src/features/admin/users/api.ts`: `adminUsersKey = ["admin", "users"]`; `useAdminUsers()`; mutations `createUser`, `changeUser(id, change)`, `deactivateUser(id)`; each success invalidates `adminUsersKey`. All calls through `useApi()` and openapi-fetch, as `features/reports/api.ts` does; a failure throws `ApiFailure`. Grounds: AGENTS (TanStack Query, openapi-fetch). Cost if wrong: none found. |
| R241 | The users screen. Heading `admin.users.title` and, at the end of its line, the primary button `admin.users.new`. `DataTable` with `label` `admin.users.title`, no search, `defaultOrder: "view"` (the API's name order), `rowKey` the id, `rowName` the user name, `empty` `admin.users.empty`, columns: user (`username` in mono, then on the signed-in administrator's own row a space and `admin.users.you` in muted sans), role (`admin.users.role.<role>`), state (`admin.state.active`, or `admin.state.inactive` muted). The opened row (max width `max-w-2xl`): `admin.users.created` with the date (`formatDate`, mono); then, for another user, three parts separated by a top border: (1) two selects `admin.users.roleLabel` (`admin.users.role.<role>`) and `admin.users.languageLabel` (`language.<code>`), the hint `admin.users.roleHint`, and the primary button `admin.users.save` (`admin.users.saving` while sending; disabled when neither value changed); it sends `PATCH /admin/users/{id}` with the changed keys alone; success shows `admin.users.savedSignedOut` when the role changed, else `admin.users.saved`, as a notice in the part; (2) a password field `admin.users.password.label` (`type="password"`, `autoComplete="new-password"`) with `admin.users.password.hint`, and the button `admin.users.password.submit`; the browser refuses fewer than 12 or more than 1024 characters (`validation.passwordShort`, `validation.passwordLong`); success empties the field and shows `admin.users.password.done`; (3) for an active user the danger button `admin.users.deactivate` and the hint `admin.users.deactivateHint`; it opens `ConfirmDialog` (`admin.users.deactivateTitle`, `admin.users.deactivateBody`, `admin.users.deactivateConfirm`, `admin.users.cancel`), and on confirm sends `DELETE /admin/users/{id}`; success announces `admin.users.deactivated`; for an inactive user, the button `admin.users.reactivate`, which sends `PATCH` with `active: true` at once and announces `admin.users.reactivated`. A failure of a part shows its sentence (`errorText`) as an error `Alert` inside that part: `LAST_ADMIN`, `USER_NOT_FOUND`, and the rest. For the signed-in administrator's own row the opened row shows the date and `admin.users.own` with the link `admin.users.ownLink` to `/account`, and nothing else. New user opens `Dialog` (`admin.users.new`) with a form (React Hook Form and Zod): `admin.users.username` (mono, `autoComplete="off"`, required), `admin.users.passwordLabel` (`type="password"`, `autoComplete="new-password"`, 12 to 1024, hint `admin.users.passwordHint`), `admin.users.roleLabel` (default `reader`), `admin.users.languageLabel` (default the administrator's interface language); buttons `admin.users.cancel` and the primary `admin.users.create` (`admin.users.creating`). `USERNAME_TAKEN` and `INVALID_USERNAME` land on the user name, `PASSWORD_TOO_SHORT` and `PASSWORD_TOO_LONG` on the password, through `errorText`; any other failure shows an error `Alert` in the dialog. Success closes the dialog, announces `admin.users.createdNotice`, and opens the new row (`?open=<id>`). Grounds: spec 12.2.2; mockup 6A. Cost if wrong: layout only. |
| R242 | The profile list. Heading `admin.profiles.title` and the primary link-button `admin.profiles.new` to `/profiles/new`. `DataTable` with no search, `defaultOrder: "view"` (identifier order), `rowKey`/`rowName` the id, `empty` `admin.profiles.empty`, columns: profile (`display_name`, under it the id in small mono muted), cluster (`cluster_name`, mono), state (`admin.state.active` / `admin.state.inactive` muted). The opened row: a `dl` grid (R235) of `admin.profiles.fact.endpoint` (mono, the whole value, wrapping), `.type` (`endpointType.<type>`), `.credential` (mono, or `admin.profiles.none` muted when empty), `.ca` (likewise), `.revision` (`admin.profiles.revisionValue` with `revision` and `formatTime(created_at)`, mono), `.namespaces` (joined with `, `, mono, spanning the grid), `.choices` (the choices summary of R246, mono, spanning the grid); then a line with the button `admin.profiles.test` (`admin.profiles.testing` while running) and its result (R247), and at its end the primary link-button `admin.profiles.edit` to `/profiles/<id>/edit` and the button `admin.profiles.delete`. Delete opens `ConfirmDialog` (`admin.profiles.deleteTitle` with `name` the display name, `admin.profiles.deleteBody`, `admin.profiles.deleteConfirm`, `admin.profiles.cancel`); success closes the row (removes `open`) and announces `admin.profiles.deleted`; a failure shows `errorText` in an error `Alert` above the table. A result of a saved test is kept per profile id in the page's state until the page unmounts. An empty list shows `admin.profiles.empty` and the button. Grounds: spec 12.2.2; mockup 5B. Cost if wrong: layout only. |
| R243 | The form's values. `web/src/features/admin/profiles/form.ts` exports `type ProfileValues` (strings for every text and number field, so that a typed value is kept as typed; booleans for `active` and `tls_verify`; string arrays for the two checkbox groups; text-box strings for the four lists of names), `newProfileValues(): ProfileValues` (the API's defaults: `endpoint_type` `auto`, `timeout_seconds` `"90"`, `tls_verify` true, `cluster_label` `"cluster"`, `filter_mode` `required`, `node_autoscaling` `auto`, the seven thresholds of `config.Default()` as strings `"70"`, `"85"`, `"30"`, `"80"`, `"30"`, `"100"`, `"1"`, `active` false, every other field empty), `valuesOf(profile: Profile): ProfileValues`, and `documentOf(values: ProfileValues): ProfileDocument` (numbers with `Number`, lists through `toLines`, a default left empty sent as `null`, file names and labels as typed, `active` and `tls_verify` as booleans). A new profile also carries `id`. The placeholder of an empty default is its fallback: `24h`, `America/Sao_Paulo`, `reportProfile.detailed`, `minSeverity.all`. Grounds: spec 12.2.2 ("a default left empty is sent as null"); the API's `Defaults()`. Cost if wrong: one module. |
| R244 | The form's checks (Zod, messages are locale keys). Required: `display_name`, `endpoint_url`, `cluster_name` (`validation.required`), and for a new profile `id` (`validation.required`, then the pattern `^[a-z0-9](?:[a-z0-9._-]{0,61}[a-z0-9])?$` without `..`, else `validation.identifier`); `allowed_namespaces` holds at least one line (`validation.namespaceList`). `timeout_seconds` is a number over 0 and at most 600 (`validation.timeout`). `ca_bundle_file` not empty while `tls_verify` is false → `validation.caWithoutTls` on `ca_bundle_file`. Each default not empty and a permitted list not empty → the default is in the list (`validation.defaultNotPermitted` on the default). Each threshold is a number of 0 or more (`validation.notNegative`); `warning_percent` below `critical_percent` (`validation.warningBelowCritical` on `warning_percent`). Nothing else is checked in the browser: syntax of periods, zones, labels, and names is the API's. Grounds: spec 12.2.2 ("checks what the API checks before it sends", read as the checks the spec and the brainstorm named). Cost if wrong: a refusal arrives from the API instead, on the same field. |
| R245 | The API's refusal on the form. A `422` `PROFILE_DOCUMENT_INVALID` sets, for each key of `fields` that is a field of the form, the message `fieldProblem.<word>` on that field; the key `thresholds` sets it on `warning_percent` (the first threshold field); a key the form does not hold shows `error.PROFILE_DOCUMENT_INVALID` in the form's error `Alert`. `ENDPOINT_URL_REJECTED` and `ENDPOINT_ADDRESS_REJECTED` set `error.<CODE>` on `endpoint_url`. `PROFILE_ID_TAKEN` sets `error.PROFILE_ID_TAKEN` on `id`. When a field in error sits in a folded section, the section opens; the focus moves to the first field in error in the order of the form. Any other failure shows `errorText` in the form's error `Alert` above the save bar. Grounds: spec 12.2.2; Review Focus 2. Cost if wrong: one function, `applyRefusal(failure, setError, open)`, in `form.ts`. |
| R246 | The layout of the form (mockup 4C). Above: on a new profile, the heading `admin.profile.newTitle`; on an edit, the small muted line `admin.profile.eyebrow`, the heading the display name as saved, and one muted line with the id in mono and `admin.profile.revision` (`revision`, `formatTime(created_at)`). Sections, each a `section` with an `h2` (`text-lg font-semibold`): `admin.profile.section.identity` (the id field, new only, mono, with `admin.profile.idHint`; the display name; the checkbox `admin.profile.active` with `admin.profile.activeHint`), `.source` (endpoint URL mono; type select of `endpointType.<v>`; timeout mono; checkbox `admin.profile.tls`; credential file mono with its hint; CA bundle file mono with its hint; the draft test line, R247), `.cluster` (cluster name mono; cluster label mono; filter select `filterMode.<v>`; node autoscaling select `nodeAutoscaling.<v>`; node pool label mono with its hint), `.namespaces` (two `TextArea`s, rows 5: allowed namespaces and excluded containers, each with `admin.profile.onePerLine`). Then two folded sections, each a box (`rounded-md border border-border bg-surface p-3`) with its title in bold, its summary in mono muted, and the button `admin.profile.change` at the end of the line, which opens the section in the same box and becomes `admin.profile.fold` (`aria-expanded`, `aria-controls`): `.choices` (permitted periods `TextArea` rows 3 with `admin.profile.periodsHint`; default period with placeholder `24h`; permitted zones `TextArea` with `admin.profile.zonesHint`; default zone with placeholder; checkboxes `admin.profile.profiles` of `reportProfile.<v>` with `admin.profile.noneTickedHint`; select default report profile with an empty first option `admin.profile.fallback` (`value` `reportProfile.detailed`); checkboxes `admin.profile.severities` of `minSeverity.<v>` with the same hint; select default minimum severity with the empty option `admin.profile.fallback` (`value` `minSeverity.all`)), `.thresholds` (seven numeric fields in a three-column grid, mono, labelled `admin.profile.threshold.<key>`). The choices summary is `[permitted periods joined with ", " or admin.profile.anyPeriod, the default period or 24h, the default zone or America/Sao_Paulo, reportProfile.<default or detailed>, minSeverity.<default or all>]` joined with ` · `; the thresholds summary is `admin.profile.short.<key>` and its value, the seven joined with ` · `; both from `summaries(values, t)` in `form.ts`, also used by the list (R242). The save bar sticks to the bottom of the window (`sticky bottom-0 z-10 border-t border-border bg-bg py-3`): the primary `admin.profile.save` (`admin.profile.saving`), `admin.profile.cancel` (a link-button to `/profiles`), and the muted hint `admin.profile.saveHintNew` or `admin.profile.saveHintEdit` (`next` = revision + 1). Grounds: spec 12.2.2; mockup 4C. Cost if wrong: layout only. |
| R247 | A test result on the screen (the list's saved test and the form's draft test). A `200`: `StatusMark` `healthy` when `ok`, else `critical`, labelled `profileTest.<code>`, in `text-sm`. The form's draft test sends `documentOf(values)` plus `id` when typed, without the browser's checks (the API judges the draft); a `422` applies R245 to the form and shows `admin.profile.testRefused` as the result line, muted; any other failure shows `errorText` with the `critical` mark. While a test runs its button is disabled and reads `admin.profiles.testing`. The form's result is cleared by the first change of any value after it. Grounds: spec 12.2.2; Review Focus 5. Cost if wrong: one component, `TestResultLine`, in `web/src/features/admin/profiles/test-result.tsx`. |
| R248 | Saving. New: `POST /admin/profiles` with `{ id, ...documentOf(values) }`; success announces `admin.profile.created`, navigates to `/profiles?open=<id>` with the guard released. Edit: `PATCH /admin/profiles/{id}` with `{ base_revision_id, ...documentOf(values) }` (the whole document); success announces `admin.profile.saved` and navigates likewise. `409 PROFILE_REVISION_STALE`: an error `Alert` above the save bar with `admin.profile.stale` and the button `admin.profile.loadLatest`; the values stay. `loadLatest` refetches `GET /admin/profiles/{id}`, and when the form has changes it first asks with the leave dialog's words (R249); it then resets the form to `valuesOf(latest)` and the base revision to the latest's. Cancel navigates to `/profiles` without asking. A load of an edit that answers `404` shows `error.PROFILE_NOT_FOUND` and a link `admin.profile.back` to `/profiles`. Grounds: spec 12.2.2; decision 8. Cost if wrong: one component. |
| R249 | The leave guard, in the form page alone. `useBlocker(({ currentLocation, nextLocation }) => dirty && !releasing && currentLocation.pathname !== nextLocation.pathname)`, where `dirty` is React Hook Form's `isDirty` and `releasing` a ref set before the navigations of save and cancel. A blocked navigation opens `ConfirmDialog` (`admin.profile.leaveTitle`, `admin.profile.leaveBody`, confirm `admin.profile.leave`, cancel `admin.profile.stay`; Stay holds the focus first, as the dialog does): Leave calls `blocker.proceed()`, Stay `blocker.reset()`. While `dirty`, a `beforeunload` listener calls `event.preventDefault()`. Grounds: decision 7; React Router 8.4 is a data router here (`createBrowserRouter`). Cost if wrong: one hook. |
| R250 | Families, in `web/src/i18n/families.ts`: `profileTest` (`profileTestCodeValues`), `endpointType` (`endpointTypeValues`), `filterMode` (`filterModeValues`), `nodeAutoscaling` (`nodeAutoscalingValues`), `fieldProblem` (the six words `missing`, `invalid`, `repeated`, `empty`, `not_permitted`, `incoherent`, as a list exported from `form.ts`), `admin.users.role` (`roleValues`), `admin.profile.threshold` and `admin.profile.short` (the seven keys of `ProfileThresholds`, as a list exported from `form.ts`). The generator's names are read from `schema.gen.ts`; a different spelling is used as generated. Grounds: AGENTS (a family is declared). Cost if wrong: none found. |
| R251 | The mock API. `web/tests/msw.ts` gains `adminProfile(overrides)`, `adminUser(overrides)`, and in `Mock` the arrays `adminProfiles` and `adminUsers`, the map `profileTests` (id or `"draft"` → a `ProfileTestResult` or a failure), and handlers for every admin route this part calls, each recording its call; a mutation changes the arrays as the API would (create appends, revise replaces and raises `revision`, delete removes, deactivate sets `active: false`), and a failure set on the mock is answered once. Grounds: the tests of the screens. Cost if wrong: test support only. |
| R252 | The browser test. `web/e2e/admin.spec.ts`, one test, "an administrator manages users and profiles", in steps, the administrator `e2e-admin` signed in: (1) on `/users`, New user creates `e2e-admin-reader` (reader, English) through the dialog; the row opens; (2) a second context signs in as that user and sees `nav.reports` and `nav.account` and neither `nav.profiles` nor `nav.users`, and `/users` shows the not-found screen; (3) on `/profiles/new` the administrator fills a profile `e2e-lab` (`Lab`, endpoint `http://10.0.0.1:9090/api/v1`, cluster `lab`, namespace `payments`) and tests it: `error.ENDPOINT_ADDRESS_REJECTED` under the endpoint field; (4) changes the endpoint to `http://fake-prometheus:9091/api/v1` and tests: `profileTest.ENDPOINT_UNREACHABLE`; (5) ticks Active and saves: the list with `e2e-lab` opened; the new report form offers `Lab`; (6) on Edit, changes the display name and clicks `nav.reports`: the leave dialog; Stay keeps the form; Cancel returns to the list with no dialog; (7) deletes `e2e-lab`: gone from the list and from the new report form; (8) the fit check: at 1366×768, 1200×1000, and 1024×768, the last row of the users table and the last row of the profile list are opened, and `expectFits` passes for each; (9) `expectNoViolations` on the users screen with a row open, with the dialog open, on the list with a row open, on the form with both folded sections open, and with the leave dialog open. `web/e2e/fixtures.ts` gains `expectFits(page, opened: Locator)`: the opened row and the row above it lie inside the viewport (`boundingBox`), and `document.documentElement.scrollWidth <= clientWidth`. The reports spec gains the fit check on the last row of its workloads table at the three sizes, after "the operator reads the report". Addresses: `10.0.0.1` is outside the e2e allowlist (`allowed_endpoint_hostnames = ["fake-prometheus"]`, no CIDR), so the policy refuses it before any request; `fake-prometheus:9091` is allowed by name and nothing listens there (the fake listens on `:9090`), so the preflight's connection is refused and `runTest` answers `ENDPOINT_UNREACHABLE`, as `caseUnreachable` proves for a closed port in Go; the fake receives no request and `remaining: 1` holds. The main thread confirms both answers on the first run; another answer stops the task. Grounds: decision 9; spec 12.2.2. Cost if wrong: one spec. |
| R253 | Version 0.6.0 is proposed at the close: new screens. Released only on the owner's word. Grounds: AGENTS (a new feature is a minor version). Cost if wrong: the owner's call. |

## Findings this plan closes

| Finding | How |
|---|---|
| F-143, the third header click and the row count of a search | Task 2 (R236); the rest of F-143 stays deferred |

## File structure

| Path | Responsibility |
|---|---|
| `api/openapi.yaml`, `api/oapi-codegen.yaml`, `internal/platform/api/openapi.gen.go`, `web/src/api/schema.gen.ts` | The draft route, `ProfileDraft`, `ProfileTestCode` (Task 1) |
| `internal/platform/profiles/test.go`, `test_test.go` | `TestDraft`, `TestCodes` (Task 1) |
| `internal/platform/api/{profiles.go,routes.go,profiles_test.go,contract_test.go}` | The handler, the route, its tests, the enum test (Task 1) |
| `web/src/components/{data-table.tsx,data-table.test.tsx,search-field.tsx,reveal.ts,reveal.test.ts}`, `web/src/lib/address.ts` | The shared table and its fit (Task 2) |
| `web/src/features/report/*-tab.tsx` | Imports and props only (Task 2) |
| `web/src/components/ui/{dialog.tsx,textarea.tsx}` | The dialog (Task 3) and the text box (Task 5) |
| `web/src/session/require-administrator.tsx`, `web/src/app/{router.tsx,shell.tsx}` | Guard, routes, navigation (Tasks 3 and 4) |
| `web/src/features/admin/users/{api.ts,users-page.tsx,user-row.tsx,new-user-dialog.tsx,users.test.tsx}` | Users (Task 3) |
| `web/src/features/admin/profiles/{api.ts,profiles-page.tsx,test-result.tsx,profiles.test.tsx}` | The profile list (Task 4) |
| `web/src/features/admin/profiles/{form.ts,lines.ts,form.test.ts}` | The form's values and summaries (Task 4), its checks and refusals (Task 5) |
| `web/src/features/admin/profiles/{profile-form-page.tsx,folded.tsx,profile-form.test.tsx,leave-guard.ts}` | The profile form (Tasks 5 and 6) |
| `web/src/locales/*.json`, `web/src/i18n/families.ts`, `web/tests/{msw.ts,screens.test.tsx}`, `web/src/components/no-style.test.tsx`, `web/src/theme/pairs.ts` | Shared; each task adds its part |
| `web/e2e/{admin.spec.ts,fixtures.ts,reports.spec.ts}` | Task 7 |
| `AGENTS.md`, `README.md`, `CHANGELOG.md` | Task 8 |

---

### Task 0: Open the part

**Executor:** Main thread.

- [x] **Step 1:** `ROADMAP.md` and `CHANGELOG.md` read and true; F-144 names `7578916`.
- [x] **Step 2:** The brainstorm (answers 1 to 13), the branch `feat/administration` from `2f0e020`, the spec amendment `0ea5daa`, approved by the owner on 2026-10-09.
- [ ] **Step 3:** This plan committed; `ROADMAP.md`: P45-1 to P45-8 `OPEN` under "Phase 4.5", the rulings R232 to R253 named, the plan's commit in P45-0.

---

### Task 1: The draft test (R232, R233)

**Executor:** Subagent. The main thread starts PostgreSQL (`make postgres-up`) and gives the DSN.

**Files:**
- Modify: `api/openapi.yaml`, `api/oapi-codegen.yaml`, `internal/platform/profiles/test.go`, `internal/platform/profiles/test_test.go`, `internal/platform/api/profiles.go`, `internal/platform/api/routes.go`, `internal/platform/api/profiles_test.go`, `internal/platform/api/contract_test.go`
- Regenerate (`make generate`): `internal/platform/api/openapi.gen.go`, `web/src/api/schema.gen.ts`

**Interfaces:**

```go
// internal/platform/profiles
var TestCodes = []string{ /* the fourteen of R232, in order */ }
func (s *Service) TestDraft(ctx context.Context, actor auth.Principal, id string, d Document) (TestResult, error)
// internal/platform/api
func (s *Server) testProfileDraft(w http.ResponseWriter, r *http.Request, a auth.Authenticated)
```

```ts
// web/src/api/schema.gen.ts (generated)
components["schemas"]["ProfileDraft"]; components["schemas"]["ProfileTestCode"]; profileTestCodeValues
```

- [ ] **Step 1: Write the failing tests**

| Test | Where | Input | Expected |
|---|---|---|---|
| `TestTestDraftOK` | `profiles/test_test.go` (postgres) | a draft of `baseDocument()` at the fake's endpoint, id `""` | `ok` true, `PROFILE_OK`, the capabilities of the saved test of the same document; no profile row exists afterwards (`store` count of profiles 0); one audit row `profile.test`, no target, metadata `{"code":"PROFILE_OK","draft":true}` |
| `TestTestDraftNamesItsTarget` | same | id `prod`, no such profile | the audit row's target is `profile`/`prod`; no profile row created |
| `TestTestDraftRefusesTheDocument` | same | `ClusterName` `""`, `TimeoutSeconds` 0 | `*DocumentError` with `cluster_name`/`empty` and `timeout_seconds`/`invalid`; no audit row |
| `TestTestDraftRefusesTheID` | same | id `Bad..Name` | `*DocumentError` `id`/`invalid`; no request sent (the fake's count unchanged) |
| `TestTestDraftRefusedByPolicy` | same | policy networks swapped to `10.0.0.0/8` as `caseRefusedByPolicy` does | a `*netpolicy.Error` with `ENDPOINT_ADDRESS_REJECTED`, no request sent, no audit row |
| `TestTestDraftUnreachable` | same | endpoint `http://metrics.test:1/api/v1`, timeout 1 | `ENDPOINT_UNREACHABLE` in under 6 s; audit outcome failed |
| `TestTestDraftAsOperator` | same | an operator principal | `ErrForbidden`; no audit row |
| `TestAdminProfilesTestDraft` | `api/profiles_test.go` (postgres) | `POST /api/v1/admin/profiles/test` with `createBody(endpoint, credential, nil)` minus `id`, signed in as administrator with CSRF | `200`, `{"ok":true,"code":"PROFILE_OK","capabilities":[…]}`; `GET /admin/profiles` still empty |
| `TestAdminProfilesTestDraftOrder` | same | no session; a reader; an administrator without CSRF; an administrator with a body over 64 KiB; with an unknown key; with `display_name` missing | `401 UNAUTHENTICATED`; `403 FORBIDDEN`; `403 CSRF_REJECTED`; `400 REQUEST_INVALID` with `{"":"too_long"}`; `400` naming the key `unknown`; `400` naming `display_name` `missing` (with `wantProblem`) |
| `TestAdminProfilesTestDraftRefused` | same | `cluster_name` `" "`; endpoint `http://10.0.0.1/api/v1` | `422 PROFILE_DOCUMENT_INVALID` with `{"cluster_name":"empty"}`; `422 ENDPOINT_ADDRESS_REJECTED` with `{"endpoint_url":"invalid"}` |
| `TestAdminProfilesNamedTest` | same | create a profile with id `test`, then `GET`, `PATCH` (display name), `DELETE` on `/api/v1/admin/profiles/test` | `200`, `200` with the new name, `204`: the draft route shadows no profile named `test` |
| `TestProfileTestCodesEqualTheService` | `api/contract_test.go` (no tag) | the contract's `ProfileTestCode` | equals `profiles.TestCodes` as an ordered list |
| `TestRoutesMatchTheContract` (existing) | — | the new route in both | passes |

- [ ] **Step 2:** Run `go test ./internal/platform/api/ -run Contract` and the tagged tests; expected FAIL.
- [ ] **Step 3:** Edit the contract (R232), `make generate`; implement R233.
- [ ] **Step 4:** `make lint`, `make test`, `make generate-check`, and `KCR_TEST_DATABASE_DSN=<dsn> go test -count=1 -tags=postgres ./internal/platform/profiles/ ./internal/platform/api/`. All pass.
- [ ] **Step 5:** Commit `feat(api): test a draft profile` (31 characters).

---

### Task 2: One table for the app, and the window fit (R234, R235, R236)

**Executor:** Subagent. Follows Task 1.

**Files:**
- Move: `web/src/features/report/data-table.tsx` → `web/src/components/data-table.tsx`; `web/src/features/report/data-table.test.tsx` → `web/src/components/data-table.test.tsx`; `web/src/features/report/address.ts` → `web/src/lib/address.ts`; `web/src/features/report/search-field.tsx` → `web/src/components/search-field.tsx` (with `git mv`)
- Create: `web/src/components/reveal.ts`, `web/src/components/reveal.test.ts`
- Modify: `web/src/features/report/{findings-tab,namespaces-tab,workloads-tab,autoscaling-tab,nodes-tab,quality-tab}.tsx` (imports, `search`, `empty`), `web/src/locales/{en,pt-BR}.json` (the moved keys)

**Interfaces:**

```ts
export interface Column<Row> {
  id: string;
  header: string;
  cell: (row: Row) => ReactNode;
  sortValue?: (row: Row) => number | string | null;
  align?: "start" | "end";
}
export function DataTable<Row extends RowData>(props: {
  label: string;
  rows: readonly Row[];
  rowKey: (row: Row) => string;
  rowName: (row: Row) => string;
  columns: Column<Row>[];
  renderOpened: (row: Row) => ReactNode;
  empty: string;
  search?: { label: string; text: (row: Row) => string };
  defaultOrder?: "status" | "view";
  status?: (row: Row) => string;
}): JSX.Element;
export function revealRows(row: HTMLElement, opened: HTMLElement, head: number): void; // reveal.ts
export const factGrid: string; // "grid gap-4 grid-cols-[repeat(auto-fit,minmax(14rem,1fr))]", from data-table.tsx
```

**The words:** `report.table.rows`, `.noMatch`, `.open`, `.close`, `.details` are renamed `table.rows`, `table.noMatch`, `table.open`, `table.close`, `table.details` in both files, their texts unchanged. No new text.

- [ ] **Step 1: Write the failing tests**

| Test | Where | Input | Expected |
|---|---|---|---|
| existing tests of `data-table.test.tsx` | moved | the new props (`search`, `empty`) | pass unchanged in what they assert |
| `no search` | `data-table.test.tsx` | a table without `search`, `?q=web` in the address | no search field, no row count line, every row shown |
| `third click removes the sort` (R236 a) | same | click a sortable header three times | `sort` absent from the address after the third; the default order returns; `aria-sort` absent |
| `announces the count of a search` (R236 b) | same | render (no announcement), then type `web` | the live region holds `Rows: N` with the shown count after the typing, and nothing after the first render |
| `opening reveals the row` (Review Focus 3) | same | `revealRows` spied; open a row | called once with the row, its opened row, and the `thead` height; closing the row does not call it |
| `reveal scrolls down` | `reveal.test.ts` | `innerHeight` 768, head 40, row top 600, opened bottom 1000 | `scrollBy` with `top: 240` (1000 − 768 + 8) |
| `reveal keeps the row's top` | same | head 40, row top 300, opened bottom 1400 | `scrollBy` with `top: 252` (300 − 40 − 8), not 640 |
| `reveal scrolls up` | same | head 40, row top 20, opened bottom 300 | `scrollBy` with `top: -28` |
| `reveal stays` | same | head 40, row top 100, opened bottom 500 | no scroll |
| `reveal follows the motion setting` | same | `prefers-reduced-motion: reduce` matched, then not | `behavior: "auto"`, then `"smooth"` |
| `cells wrap` | `data-table.test.tsx` | any table | every `td` has the class `[overflow-wrap:anywhere]` |
| report tab tests (existing) | `features/report/*.test.tsx`, `tests/screens.test.tsx` | unchanged | pass |

- [ ] **Step 2:** Run `cd web && npx vitest run src/components/`; expected FAIL.
- [ ] **Step 3:** Move the files, rename, implement R235 and R236.
- [ ] **Step 4:** `make lint`, `make test`. All pass.
- [ ] **Step 5:** Commit `refactor(web): share the table, fit opened rows` (47 characters).

---

### Task 3: The users screen (R237, R239, R240, R241, R251)

**Executor:** Subagent. Follows Task 2.

**Files:**
- Create: `web/src/components/ui/dialog.tsx`, `web/src/session/require-administrator.tsx`, `web/src/features/admin/users/{api.ts,users-page.tsx,user-row.tsx,new-user-dialog.tsx,users.test.tsx}`
- Modify: `web/src/app/router.tsx` (the administrator branch with `users`), `web/src/app/shell.tsx` (`nav.users`), `web/src/locales/{en,pt-BR}.json`, `web/src/i18n/families.ts` (`admin.users.role`), `web/tests/msw.ts` (users part of R251), `web/tests/screens.test.tsx`, `web/src/components/no-style.test.tsx`

**Interfaces:**

```ts
export function Dialog(props: { open: boolean; onOpenChange: (open: boolean) => void; title: string; children: ReactNode; onCloseAutoFocus?: (event: Event) => void }): JSX.Element;
export function RequireAdministrator(): JSX.Element;
export const adminUsersKey: readonly ["admin", "users"];
export function useAdminUsers(): UseQueryResult<User[]>;
export function UsersPage(): JSX.Element;
```

**The words** (both files):

| Key | English | Português (Brasil) |
|---|---|---|
| `nav.users` | Users | Usuários |
| `admin.state.active` | Active | Ativo |
| `admin.state.inactive` | Inactive | Inativo |
| `admin.users.title` | Users | Usuários |
| `admin.users.new` | New user | Novo usuário |
| `admin.users.empty` | No user yet. | Nenhum usuário ainda. |
| `admin.users.loadFailed` | The users could not be loaded. | Não foi possível carregar os usuários. |
| `admin.users.column.user` | User | Usuário |
| `admin.users.column.role` | Role | Papel |
| `admin.users.column.state` | State | Estado |
| `admin.users.you` | (you) | (você) |
| `admin.users.role.administrator` | Administrator | Administrador |
| `admin.users.role.operator` | Operator | Operador |
| `admin.users.role.reader` | Reader | Leitor |
| `admin.users.created` | Created {{date}} | Criado em {{date}} |
| `admin.users.roleLabel` | Role | Papel |
| `admin.users.languageLabel` | Language | Idioma |
| `admin.users.roleHint` | Changing the role signs the user out everywhere. | Mudar o papel encerra todas as sessões do usuário. |
| `admin.users.save` | Save changes | Salvar alterações |
| `admin.users.saving` | Saving… | Salvando… |
| `admin.users.saved` | Saved. | Salvo. |
| `admin.users.savedSignedOut` | Saved. The user was signed out everywhere. | Salvo. Todas as sessões do usuário foram encerradas. |
| `admin.users.password.label` | New password | Nova senha |
| `admin.users.password.hint` | 12 to 1024 characters. Setting it signs the user out everywhere. | De 12 a 1024 caracteres. Defini-la encerra todas as sessões do usuário. |
| `admin.users.password.submit` | Set password | Definir senha |
| `admin.users.password.done` | Password set. The user was signed out everywhere. | Senha definida. Todas as sessões do usuário foram encerradas. |
| `admin.users.deactivate` | Deactivate | Desativar |
| `admin.users.deactivateHint` | Signs the user out and blocks sign-in. Their reports stay. | Encerra as sessões e bloqueia a entrada. Os relatórios do usuário ficam. |
| `admin.users.deactivateTitle` | Deactivate {{name}}? | Desativar {{name}}? |
| `admin.users.deactivateBody` | {{name}} is signed out and can no longer sign in. Their reports stay, and the account can be reactivated. | {{name}} tem as sessões encerradas e não pode mais entrar. Os relatórios ficam, e a conta pode ser reativada. |
| `admin.users.deactivateConfirm` | Deactivate | Desativar |
| `admin.users.cancel` | Cancel | Cancelar |
| `admin.users.deactivated` | {{name}} was deactivated. | {{name}} foi desativado. |
| `admin.users.reactivate` | Reactivate | Reativar |
| `admin.users.reactivated` | {{name}} can sign in again. | {{name}} pode entrar de novo. |
| `admin.users.own` | This is your account. Change your password and language on the account page. | Esta é a sua conta. Altere a senha e o idioma na página da conta. |
| `admin.users.ownLink` | Open the account page | Abrir a página da conta |
| `admin.users.username` | User name | Nome de usuário |
| `admin.users.passwordLabel` | Password | Senha |
| `admin.users.passwordHint` | 12 to 1024 characters. The user can change it on their account page. | De 12 a 1024 caracteres. O usuário pode alterá-la na página da conta. |
| `admin.users.create` | Create user | Criar usuário |
| `admin.users.creating` | Creating… | Criando… |
| `admin.users.createdNotice` | {{name}} was created. | {{name}} foi criado. |

- [ ] **Step 1: Write the failing tests** (`users.test.tsx` through `renderApp` and the mock API)

| Test | Input | Expected |
|---|---|---|
| `lists the users` | the mock's users `admin` (you), `ana.souza` operator, `carla.lima` reader inactive | three rows in the API's order; `(you)` on `admin`; `Inactive` on `carla.lima`, muted |
| `only an administrator` | an operator opens `/users`; a reader opens `/users`; an administrator opens `/reports` | the not-found screen for the first two, with no `Users` in their navigation; the administrator's navigation holds `Users` after `New report` |
| `changes role and language` | open `ana.souza`, choose Reader and English, Save changes | one `PATCH` with `{"role":"reader","language":"en"}`; the notice `Saved. The user was signed out everywhere.`; the row shows Reader |
| `language alone` | change only the language | the body holds `language` alone; the notice `Saved.` |
| `save waits for a change` | open a row | Save changes disabled until a select changes |
| `sets a password` | type 11 characters, Set password; then 12 | `Use at least 12 characters.` and no call; then one `PATCH` with `password`, the field emptied, the notice |
| `deactivates after asking` | Deactivate on `ana.souza` | the alert dialog `Deactivate ana.souza?` with Cancel focused; Cancel sends nothing; confirm sends `DELETE`; the row reads Inactive and offers Reactivate; the live region holds `ana.souza was deactivated.` |
| `reactivates` | Reactivate on `carla.lima` | one `PATCH` `{"active":true}`; Active |
| `last administrator` (Review Focus 4) | the mock answers `409 LAST_ADMIN` to a role change of `diego` | the sentence of `error.LAST_ADMIN` in an error alert inside that part of the row |
| `own row` (Review Focus 4) | open `admin` (you) | the date, the sentence `admin.users.own` and the link to `/account`; no select, no password field, no Deactivate |
| `creates a user` | New user; Create with nothing; then `bruno`, a 12-character password, Operator, Português | `Fill in this field.` on the name and the password, no call; then one `POST` with the four keys; the dialog closes; the row `bruno` is opened (`?open=<id>`); the announcement |
| `taken name` | the mock answers `409 USERNAME_TAKEN` | the sentence of `error.USERNAME_TAKEN` under the user name; the dialog stays |
| `escape closes the dialog` | open, press Escape | closed; focus back on New user |
| no-style (new entry) | open the dialog, close it | no `style` attribute or element added |
| screens (new entry) | `/users` with a row opened, both languages and themes | axe and vocabulary pass |

- [ ] **Step 2:** Run; expected FAIL.
- [ ] **Step 3:** Implement R237, R239 (users part), R240 (users part), R241, R251 (users part); add the words and the family.
- [ ] **Step 4:** `make lint`, `make test`. All pass.
- [ ] **Step 5:** Commit `feat(web): manage users` (23 characters).

---

### Task 4: The profile list (R238, R239, R240, R242, R243, R246, R247, R250, R251)

**Executor:** Subagent. Follows Task 3.

**Files:**
- Create: `web/src/features/admin/profiles/{api.ts,profiles-page.tsx,test-result.tsx,profiles.test.tsx,lines.ts,form.ts,form.test.ts}`: `form.ts` with `ProfileValues`, `valuesOf`, `summaries`, and the two key lists (the rest of the module comes in Task 5); `lines.ts` with `toLines` and `fromLines` (R238)
- Modify: `web/src/app/router.tsx` (`profiles`), `web/src/app/shell.tsx` (`nav.profiles` before `nav.users`), `web/src/locales/{en,pt-BR}.json`, `web/src/i18n/families.ts` (`profileTest`, `endpointType`), `web/tests/msw.ts` (profiles part of R251), `web/tests/screens.test.tsx`

**Interfaces:**

```ts
export const adminProfilesKey: readonly ["admin", "profiles"];
export function useAdminProfiles(): UseQueryResult<Profile[]>;
export function ProfilesPage(): JSX.Element;
export function TestResultLine(props: { outcome: TestOutcome | null }): JSX.Element | null;
export type TestOutcome = { kind: "result"; result: ProfileTestResult } | { kind: "refused" } | { kind: "failed"; text: string };
export type ProfileValues = { /* R243 */ };
export function valuesOf(profile: Profile): ProfileValues;
export function toLines(text: string): string[]; // lines.ts
export function fromLines(items: readonly string[]): string; // lines.ts
export function summaries(values: ProfileValues, t: TFunction): { choices: string; thresholds: string };
export const thresholdKeys: readonly ["warning_percent", "critical_percent", "low_utilization_percent", "autoscaler_near_max_percent", "nodepool_spread_points", "bursty_gap_percent", "hpa_replica_divergence"];
```

The list calls `summaries(valuesOf(profile), t)`.

**The words** (both files):

| Key | English | Português (Brasil) |
|---|---|---|
| `nav.profiles` | Profiles | Perfis |
| `admin.profiles.title` | Profiles | Perfis |
| `admin.profiles.new` | New profile | Novo perfil |
| `admin.profiles.empty` | No profile yet. | Nenhum perfil ainda. |
| `admin.profiles.loadFailed` | The profiles could not be loaded. | Não foi possível carregar os perfis. |
| `admin.profiles.column.profile` | Profile | Perfil |
| `admin.profiles.column.cluster` | Cluster | Cluster |
| `admin.profiles.column.state` | State | Estado |
| `admin.profiles.fact.endpoint` | Endpoint | Endpoint |
| `admin.profiles.fact.type` | Type | Tipo |
| `admin.profiles.fact.credential` | Credential file | Arquivo de credencial |
| `admin.profiles.fact.ca` | CA bundle file | Arquivo do CA bundle |
| `admin.profiles.fact.revision` | Revision | Revisão |
| `admin.profiles.fact.namespaces` | Namespaces | Namespaces |
| `admin.profiles.fact.choices` | Report form choices | Opções do formulário de relatório |
| `admin.profiles.revisionValue` | {{revision}}, saved {{time}} | {{revision}}, salva em {{time}} |
| `admin.profiles.none` | None | Nenhum |
| `admin.profiles.test` | Test connection | Testar conexão |
| `admin.profiles.testing` | Testing… | Testando… |
| `admin.profiles.edit` | Edit | Editar |
| `admin.profiles.delete` | Delete | Excluir |
| `admin.profiles.deleteTitle` | Delete the profile {{name}}? | Excluir o perfil {{name}}? |
| `admin.profiles.deleteBody` | It leaves this list and the new report form. Reports already requested keep it, and its identifier can be used again. | Ele sai desta lista e do formulário de novo relatório. Os relatórios já pedidos o mantêm, e o identificador pode ser usado de novo. |
| `admin.profiles.deleteConfirm` | Delete | Excluir |
| `admin.profiles.cancel` | Cancel | Cancelar |
| `admin.profiles.deleted` | The profile {{name}} was deleted. | O perfil {{name}} foi excluído. |
| `admin.profile.anyPeriod` | Any period | Qualquer período |
| `admin.profile.short.warning_percent` | Attention {{value}} % | Atenção {{value}} % |
| `admin.profile.short.critical_percent` | Critical {{value}} % | Crítico {{value}} % |
| `admin.profile.short.low_utilization_percent` | Low utilization {{value}} % | Baixa utilização {{value}} % |
| `admin.profile.short.autoscaler_near_max_percent` | Near maximum {{value}} % | Perto do máximo {{value}} % |
| `admin.profile.short.nodepool_spread_points` | Node pool spread {{value}} | Diferença entre node pools {{value}} |
| `admin.profile.short.bursty_gap_percent` | Variation {{value}} % | Variação {{value}} % |
| `admin.profile.short.hpa_replica_divergence` | Divergence {{value}} | Divergência {{value}} |
| `endpointType.auto` | Detect | Detectar |
| `endpointType.prometheus` | Prometheus | Prometheus |
| `endpointType.grafana` | Grafana | Grafana |
| `profileTest.PROFILE_OK` | Connected. The source answers every query the report needs. | Conectado. A fonte responde a todas as consultas que o relatório usa. |
| `profileTest.ENDPOINT_URL_REJECTED` | The endpoint URL is not allowed. | A URL do endpoint não é permitida. |
| `profileTest.ENDPOINT_ADDRESS_REJECTED` | The address of the endpoint is not allowed by this installation. | O endereço do endpoint não é permitido por esta instalação. |
| `profileTest.ENDPOINT_RESOLUTION_FAILED` | The host name of the endpoint could not be resolved. | Não foi possível resolver o nome do host do endpoint. |
| `profileTest.ENDPOINT_CONNECTION_REJECTED` | The connection reached an address that is not allowed. | A conexão chegou a um endereço que não é permitido. |
| `profileTest.ENDPOINT_REDIRECT_REJECTED` | The endpoint answered with a redirect, which is not followed. | O endpoint respondeu com um redirect, que não é seguido. |
| `profileTest.PROFILE_CREDENTIAL_UNAVAILABLE` | The credential file could not be read. | Não foi possível ler o arquivo de credencial. |
| `profileTest.PROFILE_CA_BUNDLE_UNAVAILABLE` | The CA bundle file could not be read. | Não foi possível ler o arquivo do CA bundle. |
| `profileTest.ENDPOINT_TLS_FAILED` | The TLS connection failed. | A conexão TLS falhou. |
| `profileTest.ENDPOINT_AUTH_REJECTED` | The source refused the credential. | A fonte recusou a credencial. |
| `profileTest.ENDPOINT_API_NOT_FOUND` | No Prometheus API answers at this URL. | Nenhuma API do Prometheus responde nesta URL. |
| `profileTest.ENDPOINT_UNREACHABLE` | The source could not be reached. | Não foi possível alcançar a fonte. |
| `profileTest.CLUSTER_SCOPE_NOT_FOUND` | No series of this cluster was found at the source. | Nenhuma série deste cluster foi encontrada na fonte. |
| `profileTest.CAPABILITIES_INSUFFICIENT` | The source lacks metrics the report needs. | A fonte não tem métricas de que o relatório precisa. |

- [ ] **Step 1: Write the failing tests** (`profiles.test.tsx`)

| Test | Input | Expected |
|---|---|---|
| `lists the profiles` | the mock's `dr`, `lab` (inactive), `production` | three rows in the API's order; display name over the id in mono; the cluster in mono; `Inactive` muted on `lab` |
| `opens a profile` | open `production` | the endpoint whole, `Detect`, `production-token`, CA `None` muted, `3, saved <time>`, the namespaces joined, the choices summary `Any period · 24h · America/Sao_Paulo · Detailed · All`; the address holds `open=production` |
| `tests the saved revision` | Test connection on `production` (mock ok), then on `staging` (mock `ENDPOINT_AUTH_REJECTED`) | `Connected. The source answers every query the report needs.` with the healthy shape; `The source refused the credential.` with the critical shape; the button reads `Testing…` and is disabled while the call is open; the result of `production` is still shown when its row is opened again |
| `test fails` | the mock answers `500 INTERNAL_ERROR` | the sentence of `error.INTERNAL_ERROR` with the critical shape |
| `deletes after asking` | Delete on `lab` | the alert dialog `Delete the profile Lab?` with the body; confirm sends `DELETE /admin/profiles/lab`; the row is gone; `open` left the address; the announcement; the summaries of `/profiles` are fetched again (a second `GET /profiles` recorded) |
| `empty list` | no profiles | `No profile yet.` and New profile |
| `navigation` | an administrator | `Profiles` then `Users` after `New report`, before `Account` |
| `lines` (`form.test.ts`) | `"a\n  b \n\n a"`; `["a","b"]` | `["a","b","a"]`; `"a\nb"` |
| `values of a profile` | `valuesOf` of the mock's `production` | every field as R243 says: numbers as strings, lists as lines, a null default as `""` |
| `summaries` (`form.test.ts`) | permitted periods `24h`, `7d`, default `7d`, zone null, profile `executive`, severity null; thresholds of the defaults | `24h, 7d · 7d · America/Sao_Paulo · Executive · All`; `Attention 70 % · Critical 85 % · Low utilization 30 % · Near maximum 80 % · Node pool spread 30 · Variation 100 % · Divergence 1` |
| screens (new entry) | `/profiles` with a row opened, both languages and themes | axe and vocabulary pass |

- [ ] **Step 2:** Run; expected FAIL.
- [ ] **Step 3:** Implement R239 (profiles route), R240 (profiles part), R242, R243 (`ProfileValues`, `valuesOf`), R238 (`lines.ts`), R246 (`summaries`), R247 (`TestResultLine`), R250 (`profileTest`, `endpointType`), R251 (profiles part).
- [ ] **Step 4:** `make lint`, `make test`. All pass.
- [ ] **Step 5:** Commit `feat(web): list and test profiles` (33 characters).

---

### Task 5: The profile form, new and edit (R238, R243, R244, R245, R246, R248 without the stale path, R250)

**Executor:** Subagent. Follows Task 4.

**Files:**
- Create: `web/src/components/ui/textarea.tsx`, `web/src/features/admin/profiles/{profile-form-page.tsx,folded.tsx,profile-form.test.tsx}`
- Modify: `web/src/features/admin/profiles/{form.ts,form.test.ts,api.ts}`, `web/src/app/router.tsx` (`profiles/new`, `profiles/:id/edit`), `web/src/locales/{en,pt-BR}.json`, `web/src/i18n/families.ts` (`filterMode`, `nodeAutoscaling`, `fieldProblem`, `admin.profile.threshold`, `admin.profile.short`), `web/tests/msw.ts`, `web/tests/screens.test.tsx`

**Interfaces:**

```ts
export function TextArea(props: Omit<ComponentProps<"textarea">, "id"> & { label: string; hint?: string; error?: string }): JSX.Element;
export function newProfileValues(): ProfileValues; // form.ts, beside Task 4's ProfileValues and valuesOf
export function documentOf(values: ProfileValues): ProfileDocument;
export function profileSchema(isNew: boolean): z.ZodType<ProfileValues>;
export function applyRefusal(failure: ApiFailure, setError: UseFormSetError<ProfileValues>, open: (section: "choices" | "thresholds") => void): boolean; // false when nothing landed on a field
export const fieldProblems: readonly ["missing", "invalid", "repeated", "empty", "not_permitted", "incoherent"];
export function Folded(props: { title: string; summary: string; open: boolean; onToggle: () => void; children: ReactNode }): JSX.Element;
export function ProfileFormPage(): JSX.Element;
```

**The words** (both files):

| Key | English | Português (Brasil) |
|---|---|---|
| `admin.profile.newTitle` | New profile | Novo perfil |
| `admin.profile.editScreen` | Edit profile | Editar perfil |
| `admin.profile.eyebrow` | Profile | Perfil |
| `admin.profile.revision` | Revision {{revision}}, saved {{time}} | Revisão {{revision}}, salva em {{time}} |
| `admin.profile.back` | Back to the profiles | Voltar aos perfis |
| `admin.profile.section.identity` | Identity | Identificação |
| `admin.profile.section.source` | Metric source | Fonte de métricas |
| `admin.profile.section.cluster` | Cluster | Cluster |
| `admin.profile.section.namespaces` | Namespaces | Namespaces |
| `admin.profile.section.choices` | Report form choices | Opções do formulário de relatório |
| `admin.profile.section.thresholds` | Thresholds | Limiares |
| `admin.profile.change` | Change | Alterar |
| `admin.profile.fold` | Fold | Recolher |
| `admin.profile.id` | Identifier | Identificador |
| `admin.profile.idHint` | Lowercase letters, digits, dots, hyphens, and underscores. It cannot change once the profile is created. | Letras minúsculas, dígitos, pontos, hífens e sublinhados. Não muda depois que o perfil é criado. |
| `admin.profile.name` | Display name | Nome de exibição |
| `admin.profile.active` | Active | Ativo |
| `admin.profile.activeHint` | An active profile is offered in the new report form. | Um perfil ativo aparece no formulário de novo relatório. |
| `admin.profile.url` | Endpoint URL | URL do endpoint |
| `admin.profile.type` | Type | Tipo |
| `admin.profile.timeout` | Timeout (seconds) | Timeout (segundos) |
| `admin.profile.tls` | Verify the TLS certificate | Verificar o certificado TLS |
| `admin.profile.credential` | Credential file | Arquivo de credencial |
| `admin.profile.credentialHint` | A file name in the credential folder of the installation. Empty: no credential. | Nome de um arquivo na pasta de credenciais da instalação. Vazio: sem credencial. |
| `admin.profile.ca` | CA bundle file | Arquivo do CA bundle |
| `admin.profile.caHint` | A file name in the CA folder of the installation. Empty: the system's. | Nome de um arquivo na pasta de CA da instalação. Vazio: o do sistema. |
| `admin.profile.clusterName` | Cluster name | Nome do cluster |
| `admin.profile.clusterLabel` | Cluster label | Label do cluster |
| `admin.profile.filter` | Filter by cluster | Filtro por cluster |
| `admin.profile.autoscaling` | Node autoscaling | Autoscaling de nodes |
| `admin.profile.poolLabel` | Node pool label | Label do node pool |
| `admin.profile.poolHint` | Empty: the node pool is detected. | Vazio: o node pool é detectado. |
| `admin.profile.allowed` | Allowed namespaces | Namespaces permitidos |
| `admin.profile.excluded` | Excluded containers | Containers excluídos |
| `admin.profile.onePerLine` | One per line. | Um por linha. |
| `admin.profile.periods` | Permitted periods | Períodos permitidos |
| `admin.profile.periodsHint` | One length per line, such as 24h or 7d. Empty: any length and any calendar range. | Uma duração por linha, como 24h ou 7d. Vazio: qualquer duração e qualquer intervalo de calendário. |
| `admin.profile.defaultPeriod` | Default period | Período padrão |
| `admin.profile.zones` | Permitted time zones | Fusos horários permitidos |
| `admin.profile.zonesHint` | One per line. Empty: any. | Um por linha. Vazio: qualquer um. |
| `admin.profile.defaultZone` | Default time zone | Fuso horário padrão |
| `admin.profile.profiles` | Permitted report profiles | Perfis de relatório permitidos |
| `admin.profile.defaultProfile` | Default report profile | Perfil de relatório padrão |
| `admin.profile.severities` | Permitted minimum severities | Severidades mínimas permitidas |
| `admin.profile.defaultSeverity` | Default minimum severity | Severidade mínima padrão |
| `admin.profile.noneTickedHint` | None ticked: any. | Nenhum marcado: qualquer um. |
| `admin.profile.fallback` | Default ({{value}}) | Padrão ({{value}}) |
| `admin.profile.threshold.warning_percent` | Attention from (%) | Atenção a partir de (%) |
| `admin.profile.threshold.critical_percent` | Critical from (%) | Crítico a partir de (%) |
| `admin.profile.threshold.low_utilization_percent` | Low utilization below (%) | Baixa utilização abaixo de (%) |
| `admin.profile.threshold.autoscaler_near_max_percent` | Autoscaler near its maximum from (%) | Autoscaler perto do máximo a partir de (%) |
| `admin.profile.threshold.nodepool_spread_points` | Node pool spread (points) | Diferença entre node pools (pontos) |
| `admin.profile.threshold.bursty_gap_percent` | Variation between average and peak (%) | Variação entre média e pico (%) |
| `admin.profile.threshold.hpa_replica_divergence` | HPA replica divergence (replicas) | Divergência de replicas do HPA (replicas) |
| `admin.profile.save` | Save | Salvar |
| `admin.profile.saving` | Saving… | Salvando… |
| `admin.profile.cancel` | Cancel | Cancelar |
| `admin.profile.saveHintNew` | Saving creates revision 1. | Salvar cria a revisão 1. |
| `admin.profile.saveHintEdit` | Saving writes revision {{next}}. Reports already requested keep the revision they were requested with. | Salvar grava a revisão {{next}}. Relatórios já pedidos mantêm a revisão com que foram pedidos. |
| `admin.profile.created` | The profile {{name}} was created. | O perfil {{name}} foi criado. |
| `admin.profile.saved` | The profile {{name}} was saved. | O perfil {{name}} foi salvo. |
| `filterMode.required` | Required | Obrigatório |
| `filterMode.auto` | Detect | Detectar |
| `filterMode.none` | None | Nenhum |
| `nodeAutoscaling.auto` | Detect | Detectar |
| `nodeAutoscaling.enabled` | Enabled | Ativado |
| `nodeAutoscaling.disabled` | Disabled | Desativado |
| `fieldProblem.missing` | Fill in this field. | Preencha este campo. |
| `fieldProblem.invalid` | This value is not valid. | Este valor não é válido. |
| `fieldProblem.repeated` | A value appears twice. | Um valor aparece duas vezes. |
| `fieldProblem.empty` | Give at least one value. | Informe pelo menos um valor. |
| `fieldProblem.not_permitted` | This value is not among the permitted ones. | Este valor não está entre os permitidos. |
| `fieldProblem.incoherent` | This value does not agree with another field. | Este valor não combina com outro campo. |
| `validation.identifier` | Use lowercase letters, digits, dots, hyphens, and underscores, starting and ending with a letter or a digit. | Use letras minúsculas, dígitos, pontos, hífens e sublinhados, começando e terminando com uma letra ou um dígito. |
| `validation.namespaceList` | Give at least one namespace. | Informe pelo menos um namespace. |
| `validation.timeout` | Use a number over 0 and up to 600. | Use um número acima de 0 e até 600. |
| `validation.caWithoutTls` | A CA bundle needs the TLS certificate verified. | Um CA bundle exige verificar o certificado TLS. |
| `validation.defaultNotPermitted` | Use one of the permitted values. | Use um dos valores permitidos. |
| `validation.notNegative` | Use a number of 0 or more. | Use um número igual ou maior que 0. |
| `validation.warningBelowCritical` | Use a value below the critical one. | Use um valor abaixo do crítico. |

`admin.profile.fallback`'s `value` is the worded fallback (`reportProfile.detailed`, `minSeverity.all`).

- [ ] **Step 1: Write the failing tests** (`form.test.ts` for the module, `profile-form.test.tsx` for the page)

| Test | Input | Expected |
|---|---|---|
| `new values` | `newProfileValues()` | the defaults of R243 exactly |
| `round trip` | `documentOf(valuesOf(p))` for the mock's `production` | equal to `p`'s document fields (nulls kept null) |
| `empty default is null` | default period `""`, zone `"Europe/Lisbon"` | `default_period: null`, `default_timezone: "Europe/Lisbon"` |
| `checks` (R244) | one value per row: id `""`; id `Bad`; id `a..b`; timeout `0`; timeout `601`; CA `ca` with TLS off; permitted `24h` with default `7d`; threshold `-1`; warning `90` with critical `85`; allowed namespaces empty | `validation.required`; `validation.identifier`; `validation.identifier`; `validation.timeout`; `validation.timeout`; `validation.caWithoutTls` on `ca_bundle_file`; `validation.defaultNotPermitted` on `default_period`; `validation.notNegative`; `validation.warningBelowCritical` on `warning_percent`; `validation.namespaceList` |
| `new profile` | `/profiles/new`: fill id `lab`, name, endpoint, cluster, one namespace; Save | one `POST /admin/profiles` with `id` `lab`, `active` false, `default_period` null, the thresholds as numbers; then `/profiles?open=lab`; the announcement `The profile Lab was created.` |
| `layout` | `/profiles/new` | four open sections with their headings; two folded boxes with their summaries and Change (`aria-expanded` false); the id field present; the save bar with `Saving creates revision 1.` |
| `edit` | `/profiles/production/edit` | the heading `Production`, the line with `production` and `Revision 3, saved <time>`; no id field; the fields hold the profile; the hint `Saving writes revision 4. …` |
| `edit saves the whole document` | change the display name; Save | one `PATCH` with `base_revision_id` and every document key; `/profiles?open=production` |
| `refusal opens the folded section` (Review Focus 2) | the mock answers `422 PROFILE_DOCUMENT_INVALID` `{"default_timezone":"invalid","thresholds":"invalid"}` | both folded sections open; `This value is not valid.` under the default time zone and under Attention from; the focus on the default time zone |
| `refusal of a field the form lacks` | `{"nope":"invalid"}` | `error.PROFILE_DOCUMENT_INVALID` in the form's alert |
| `address refused` | `422 ENDPOINT_ADDRESS_REJECTED` | its sentence under the endpoint URL |
| `taken identifier` | `409 PROFILE_ID_TAKEN` | its sentence under the identifier |
| `cancel` | Cancel on a changed form | `/profiles`, no dialog |
| `profile not found` | `/profiles/nope/edit`, the mock answers 404 | `error.PROFILE_NOT_FOUND` and the link back |
| screens (new entries) | `/profiles/new` and `/profiles/production/edit` with both folded sections open, both languages and themes | axe and vocabulary pass |

- [ ] **Step 2:** Run; expected FAIL.
- [ ] **Step 3:** Implement R238 (`TextArea`), R243 (the rest), R244 to R246, R248 (create, edit, cancel, not found), R250 (the rest).
- [ ] **Step 4:** `make lint`, `make test`. All pass.
- [ ] **Step 5:** Commit `feat(web): create and edit profiles` (35 characters).

---

### Task 6: Test before saving, a stale revision, and the leave guard (R247, R248, R249)

**Executor:** Subagent. Follows Task 5.

**Files:**
- Create: `web/src/features/admin/profiles/leave-guard.ts`
- Modify: `web/src/features/admin/profiles/{profile-form-page.tsx,profile-form.test.tsx,api.ts}`, `web/src/locales/{en,pt-BR}.json`, `web/tests/msw.ts` (the draft route), `web/src/components/no-style.test.tsx` (the leave dialog over the form)

**Interfaces:**

```ts
export function useLeaveGuard(dirty: boolean): { release: () => void; dialog: JSX.Element }; // leave-guard.ts
export function useTestDraft(): UseMutationResult<ProfileTestResult, ApiFailure, ProfileDraft>; // api.ts
```

**The words** (both files):

| Key | English | Português (Brasil) |
|---|---|---|
| `admin.profile.testRefused` | Correct the marked fields, then test again. | Corrija os campos marcados e teste de novo. |
| `admin.profile.stale` | Someone saved this profile after you opened it. Your values are kept here. | Alguém salvou este perfil depois que você o abriu. Seus valores continuam aqui. |
| `admin.profile.loadLatest` | Load the latest revision | Carregar a revisão mais recente |
| `admin.profile.leaveTitle` | Leave without saving? | Sair sem salvar? |
| `admin.profile.leaveBody` | The changes to this profile will be lost. | As alterações deste perfil serão perdidas. |
| `admin.profile.leave` | Leave | Sair |
| `admin.profile.stay` | Stay | Ficar |

- [ ] **Step 1: Write the failing tests** (`profile-form.test.tsx`)

| Test | Input | Expected |
|---|---|---|
| `tests the draft` | `/profiles/new`, fill the endpoint and the required fields, Test connection (mock ok) | one `POST /admin/profiles/test` with the document and `id` when typed; `Connected. …` with the healthy shape; no `POST /admin/profiles` |
| `tests without the browser's checks` | an empty display name, Test connection | the call is sent; the API's answer is shown |
| `draft refused` | the mock answers `422 ENDPOINT_ADDRESS_REJECTED` | its sentence under the endpoint URL; the result line `Correct the marked fields, then test again.` |
| `draft failure` | `ENDPOINT_UNREACHABLE` in a `200` | `The source could not be reached.` with the critical shape |
| `a change clears the result` (Review Focus 5) | after a result, type one character in the cluster name | no result line |
| `stale revision keeps the values` (Review Focus 1) | edit, change the name, Save; the mock answers `409 PROFILE_REVISION_STALE` | the alert `Someone saved this profile …` with `Load the latest revision`; the typed name still in its field |
| `load latest asks first` (Review Focus 1) | then Load the latest revision; Stay; then Load again; Leave | Stay: no fetch, values kept; Leave: `GET /admin/profiles/production` again, the form holds the latest (revision 4), the hint says revision 5, the alert gone |
| `leave guard inside the app` | change a value, click `Reports` in the navigation | the dialog `Leave without saving?` with Stay focused; Stay keeps the location; again and Leave goes to `/reports` |
| `no guard when clean or saving` | an untouched form, click `Reports`; a changed form, Save | no dialog in either |
| `beforeunload` | a changed form, dispatch `beforeunload` | `defaultPrevented` true; after Save, a new event is not prevented |
| no-style (new entry) | the leave dialog over the form | no style added |

- [ ] **Step 2:** Run; expected FAIL.
- [ ] **Step 3:** Implement R247 (the draft side), R248 (the stale path), R249.
- [ ] **Step 4:** `make lint`, `make test`. All pass.
- [ ] **Step 5:** Commit `feat(web): test a profile before saving it` (42 characters).

---

### Task 7: The browser (R252)

**Executor:** Subagent writes the change; the main thread runs `make e2e`.

**Files:** Create `web/e2e/admin.spec.ts`; modify `web/e2e/fixtures.ts` (`expectFits`), `web/e2e/reports.spec.ts` (the fit check on the workloads table), and the `Makefile` only if the new spec is not picked up by the Playwright service as it stands.

- [ ] **Step 1 (subagent):** Write the spec and the helper of R252. `make lint` passes.
- [ ] **Step 2 (main thread):** The Docker checks of AGENTS.md; `make e2e` passes, and the log shows `remaining: 1`. The two answers of R252 (`ENDPOINT_ADDRESS_REJECTED` on the field, `ENDPOINT_UNREACHABLE` as the result) are seen in the run; another answer stops the task. Prove the fit check bites: a scratch change that removes `[overflow-wrap:anywhere]` and gives the opened cell a `w-[2000px]` child makes the spec fail on `expectFits` (log under `.cache/phase-4/part-5/e2e/`), reverted. Prove the guard check bites: a scratch change that disables the blocker makes step 6 fail, reverted.
- [ ] **Step 3:** Commit `test(e2e): manage users and profiles` (36 characters).

**Done when:** `make e2e` passes twice in a row from a clean stack.

---

### Task 8: The combined run, the owner's look, documents, review, gate, close

**Executor:** Main thread.

- [ ] **Step 1:** `make test`, `make lint`, `make generate-check`, `make test-postgres`, `make image`, `make check-runtime`, `make e2e` over the combined branch; logs under `.cache/phase-4/part-5/combined/`.
- [ ] **Step 2: The owner's look.** The look stack (`.cache/phase-4/part-4/look/stack/up.sh`) on this branch's build; screenshots of the real build at 1200×1000 (the users screen with a row open and the dialog open; the profile list with a row open and a test result; the form, new and edit, with both folded sections open and a refusal shown; the leave dialog; the stale alert) in light and dark, en and pt-BR, on the companion page beside the approved mockups, opened. Small deviations from an approved mockup are ruled by the main thread and listed; a change the owner asks for is made and the screens taken again.
- [ ] **Step 3: `AGENTS.md`.** "The interface": the administration screens and their guard; `DataTable` in `components/` as the one table of the app, with its fit; the dialog layer; the leave guard of the profile form. "The platform": the draft test route and its audit.
- [ ] **Step 4: `README.md`.** The web section names the profile and user screens.
- [ ] **Step 5: `CHANGELOG.md`, under Unreleased.** `### Added`: an administrator manages the profiles on the web (list, create, edit, delete) and tests a profile, saved or before saving it; an administrator manages the users on the web (create, change the role, the language, or the password, deactivate, reactivate). `### Changed`: an opened row of a table scrolls into view and fits the window. Omitted, and said so: the contract enum of the test codes, the table's move, the tests.
- [ ] **Step 6: The adversarial review of the branch, by areas**, probes under `.cache/phase-4/part-5/review/probe/`: (a) the draft route, its audit, the contract; (b) the shared table and the fit; (c) the users screen; (d) the profile list and form against the spec; (e) wording, contrast, accessibility, no inline style. One fix wave, then one re-review.
- [ ] **Step 7:** `make gate` with logs under `.cache/phase-4/part-5/gate/`.
- [ ] **Step 8: Close.** `ROADMAP.md` (P45 items, F-143's two items); the closing report `.cache/phase-4/part-5/closing-report.md`; the owner's word on the merge, the push, and version 0.6.0 (R253), each naming its target.

**Done when:** the gate passes from images built without cache, the review is clean, and the owner has the closing report.
