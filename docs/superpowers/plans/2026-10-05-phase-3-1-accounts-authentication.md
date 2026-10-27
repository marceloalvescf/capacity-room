# Phase 3.1: Database, Accounts, Authentication, and the Serve Skeleton Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** `k8s-capacity-report serve` listens on a port, applies the schema under a database lock, and answers sign-in, sign-out, the session, the reader's own account, and the administration of users, with the three roles of the design, the throttle, the two CSRF checks, the audit rows, the error envelope, the response headers, and the health routes; `admin migrate`, `admin bootstrap-admin`, and `admin probe` exist; the platform tests run against PostgreSQL 18 started by Compose.

**Architecture:** The first thread of phase 3 through the platform layers of spec 4.1: `internal/platform/settings` reads the deployment file and the DSN; `internal/platform/store` holds the pool, the embedded migrations, and every SQL statement; `internal/platform/audit` writes the append-only rows through the metadata gate; `internal/platform/auth` holds usernames, passwords, tokens, the throttle, the sessions, the users, and the account; `internal/platform/api` is the HTTP surface: the error table, the headers, the routes, and the Go types generated from `api/openapi.yaml`; `internal/cli` wires `serve` and `admin`. Nothing of the core changes. The snapshot, the profiles, the queue, and the report routes arrive in parts 3.2 to 3.4.

**Tech Stack:** Go 1.27. New modules: `github.com/jackc/pgx/v5` v5.11.0, `github.com/pressly/goose/v3` v3.28.0, `github.com/alexedwards/argon2id` v1.0.0, and `gopkg.in/yaml.v3` v3.0.1, the last used by one test alone. `golang.org/x/text` (already required) for NFKC and case folding. `github.com/oapi-codegen/oapi-codegen/v2` v2.8.0 run through `go run`, pinned in the Makefile as golangci-lint is, never required by `go.mod`. PostgreSQL 18 through Docker Compose on the main thread.

**Spec:** `docs/superpowers/specs/2026-09-27-k8s-capacity-report-design.md`, sections 2, 3, 4, 11.1 to 11.4, 11.9, 11.10 (the auth, account, and user rows), 11.12, 13.1, 13.2, 13.5, 14, 15, and 16. The inventory of the original is `docs/reference/inventory/platform.md`. The decisions of the owner of 2026-10-05 are in `ROADMAP.md`.

**Depends on:** Release 0.1.0, `main` at `5de4568`, tagged `v0.1.0`.

## Where this part sits

| Part | Delivers | Proof at the end |
|---|---|---|
| **3.1 Database, accounts, authentication, serve skeleton (this plan)** | The schema and migrations, the deployment settings, the three roles, sign-in and sign-out, sessions, CSRF, the throttle, the audit rows, the users and account routes, the error envelope, the headers, the health routes, `serve`, `admin migrate`, `admin bootstrap-admin`, `admin probe`, the PostgreSQL test harness | An administrator bootstrapped from the command line signs in through the API, creates an operator, and the operator changes their own password |
| 3.2 Profiles, credential files, outbound policy | The profile tables and revisions, the credential and CA folders, the outbound request policy, the profile routes and the profile test | A profile created through the API and tested against the fake Prometheus |
| 3.3 River, worker, storage, retention, audit of reports | The report rows, River, the worker, the stored snapshot and PDFs, expiry, the report events; F-41, F-89, F-93, F-102 | A report requested through the API and its PDF compiled by the worker |
| 3.4 API, report view, OpenAPI contract | The report routes, the report view, pagination, idempotency, admission, the complete contract | A report requested and downloaded through the API, as spec 16 asks |

### Out of this part

| Left out | Arrives in | Reason |
|---|---|---|
| `admin import-profiles`, the profile routes, the outbound policy | 3.2 | They need the profile tables |
| The report routes, the report view, `Idempotency-Key`, admission, pagination | 3.3 and 3.4 | They need the report rows and the queue |
| `worker`, `deploy/compose.yaml`, `deploy/compose.integration.yaml` | 3.3 | The Compose stack of the design names the worker; this part ships `deploy/compose.test.yaml` alone |
| The report events of the audit | 3.3 | The audit table and its writer arrive now, because sign-in writes a row (R95) |
| The static files of the frontend, the `static_index` readiness check, the pinned CSP hashes | 4 | There is no frontend yet (R89, R90) |
| A purge of old throttle rows | 3.3 | Recorded as F-109; the retention job of 3.3 is the place for it |

## How to read this plan

The owner's orchestration rule applies: the plan states what each task must
deliver and how it is proven, and the implementer writes the code. Each task
gives exact paths, exact signatures, and a table of tests with inputs and
expected results. Write each test first, watch it fail, then implement.

Executors:

- **Subagent**: a Sonnet subagent. It may run `go`, `make build`, `make test`,
  `make lint`, `make generate`, and the tagged tests. It runs the PostgreSQL
  tests only against the server the main thread started, through
  `KCR_TEST_DATABASE_DSN`, and never starts or stops Docker. It must not
  run `make update-testdata` or `make update-golden`, and must not declare
  a task finished.
- **Main thread**: needs Docker, judgment, wording, API design, or the owner.

Rules for every subagent brief:

- Tasks run in sequence. Two agents never share the checkout.
- A reviewer receives a diff file under `.cache/phase-3/part-1/` and runs
  tests only on packages nobody is editing.
- Every commit names its paths: `git add <paths>`, never `git add -A`.
- A subagent never runs `git stash`, `git restore`, `git checkout` of a
  path, or `git clean`.
- A test of a change goes red on the code before the change, test by test,
  and the brief asks for the red run.
- A brief that says "report every X" also lists, as tests, the cases that
  must not match.
- A subagent has no `ReportFindings` tool. A reviewer writes its findings to
  a file under `.cache/phase-3/part-1/` and repeats them in its last message.
- A decision flagged at the end of a report has already been made. A brief
  that says "stop and report" means stop before deciding.
- The main thread commits its own changes before it dispatches, and starts
  PostgreSQL with `make postgres-up` before dispatching a task that runs
  the `postgres` tests. The brief carries the DSN.
- A placeholder in a dispatch is a defect of the dispatch. Compute first,
  dispatch after.

Reference material, read-only and never tracked:

- `older/kubernetes-report/kube_capacity_report/platform/`: the Python
  source of the original platform. Inside `older/`, nobody reads `.env`,
  `.envrc`, `report.toml`, anything under `deploy/secrets`, `.venv`, `.git`,
  `.worktrees`, `build`, or `node_modules`.
- `.cache/phase-3/part-1/research/original-auth.md`: the facts extracted
  from the original for this part, with file and line: the schema, the
  authentication, the throttle, the error table, the headers, the health
  routes, the commands, and the settings. A brief quotes the lines it
  needs; an implementer that finds the brief and the original in
  disagreement stops and reports.

## Global Constraints

Every constraint of the plans of phases 0 to 2 stands. The ones below are
added or are the ones this part leans on most.

- Go 1.27. Module `github.com/marceloalvescf/k8s-capacity-report`. Four
  modules enter `go.mod`, at the versions named above, and no other. No
  Kubernetes or cloud client. `make generate` runs oapi-codegen through
  `go run` at a version pinned in the Makefile; it is never required.
- Imports point one way. The platform packages rank above every core
  package and below `internal/cli` (R78). The core never imports
  `internal/platform/`.
- Metrics only. Every diagnostic is in English. Reader text lives in the
  catalog; the fixed sentences of the error envelope are diagnostics for
  API clients, held in one table (R88).
- No test skips. A test that needs PostgreSQL carries the build tag
  `postgres` and fails when `KCR_TEST_DATABASE_DSN` is unset (R94). A test
  never writes a golden file. No comment speaks of the process.
- Every message of `serve` and `admin` passes through the one redactor of
  `cli.Run`; the DSN is added to it the moment the settings are loaded.
  Every failure of a command prints one line `error: <message>` on standard
  error and nothing on standard output. No message repeats a value that
  was not understood.
- A password is never logged, never audited, never written to a message.
  A session token and a CSRF token are stored as SHA-256 digests only.
  Comparisons of secrets are constant time (`crypto/subtle`,
  `crypto/hmac`).
- The role is read from the database on every authenticated request. Every
  check of a route runs in this order: session (401), role (403), CSRF
  (403), body (400).
- Every table of this part has `ON DELETE RESTRICT` on every foreign key.
  Deletion is deactivation. The audit table refuses an update and a delete
  by a trigger (R80).
- Every timestamp is `timestamptz` and is passed from Go, so a test
  controls the clock; no statement calls `now()`.
- `api/openapi.yaml` describes every route under `/api/v1` of this part.
  The Go types of the bodies are generated from it, and a test compares the
  routes the program registers with the paths of the file, both ways (R97).
- Scratch files go under `.cache/phase-3/part-1/`, never under `/tmp`.
- Commits follow `type(scope): description`, imperative, subject under 50
  characters, counted before they are written.

## Review Focus

Each line names an input that the spec implies and does not describe, and the
task whose tests pin it.

1. **A username that folds into the pattern.** `Ⅷ` (U+2167) normalises
   under NFKC to `VIII` and folds to `viii`, a valid name; `Admin ` folds and
   trims to `admin`; a name with a NUL byte or a space inside never reaches
   the database. Pinned in Task 6 (`TestNormalizeUsernameFoldsAndTrims`,
   `TestValidateUsernameRefusesControlCharacters`).
2. **An `X-Forwarded-For` header from an untrusted peer.** The throttle is
   keyed by the peer address, and the header is read only when the peer is
   inside `trusted_proxies`; a forged header from elsewhere never changes
   the key. Pinned in Task 9 (`TestClientAddressIgnoresForwardedFromUntrustedPeer`).
3. **A request body that is not the shape of the contract.** An unknown
   key, a repeated key, a wrong type, a missing field, a trailing second
   document, and a body over 64 KiB each answer 400 `REQUEST_INVALID` with
   the field named, never 500 and never the text of the decoder. Pinned in
   Task 9 (`TestDecodeBodyRefusals`).
4. **A session whose account was deactivated after sign-in.** The next
   request answers 401, the session row is revoked with the reason
   `user_inactive`, and the cookie is cleared. Pinned in Task 7
   (`TestAuthenticateRevokesSessionOfInactiveUser`) and Task 10
   (`TestDeactivatedUserIsSignedOutOnNextRequest`).
5. **The last active administrator.** They cannot deactivate or demote
   themselves (`SELF_DEACTIVATE`, `SELF_DEMOTE`, checked first), and the
   service refuses with `LAST_ADMIN` any change that would leave no active
   administrator, counting active ones only. Through the routes a single
   actor cannot reach `LAST_ADMIN`, since the actor is an active
   administrator and a change to themself is refused first; the rule guards
   the race of two administrators and a caller of the service. Pinned in
   Task 8 (`TestUpdateRefusesLastAdministratorTransitions`) and the mapping
   in Task 11 (`TestUsersErrorMapping`).

## Rulings made on the owner's behalf

Each of these is a decision the spec leaves open, written as decision,
reason, and cost if wrong. The owner may overturn any of them when reviewing
this plan. The numbering continues that of part 2.4.

| ID | Ruling |
|---|---|
| R78 | Ruling: the platform packages of this part are `internal/platform/settings` (rank 110), `internal/platform/store` (120), `internal/platform/storetest` (125, test support, forbidden in the production binary beside `internal/promtest`), `internal/platform/audit` (130), `internal/platform/auth` (140), and `internal/platform/api` (150); `internal/cli` moves from rank 100 to 200, so that the parts to come have room between 150 and 200 — spec 4.1 names the platform packages and spec 4.4 wants the core below them; `settings` is not in the list of 4.1 and is added, since the deployment file is read by `serve`, `worker`, and `admin` alike and belongs to none of the other packages — a package added later needs a rank, which is one line |
| R79 | Ruling: PostgreSQL access is `pgx/v5` with a pool; migrations are SQL files embedded under `internal/platform/store/migrations/` and applied by goose's provider through `pgx/v5/stdlib`, under goose's PostgreSQL session locker with the lock id `72427301`, the number of the original's entrypoint (`deploy/entrypoint.sh:26`); passwords are hashed by `alexedwards/argon2id` with the parameters pinned at the original's library defaults, memory 65536 KiB, iterations 3, parallelism 4, salt 16 bytes, key 32 bytes (`argon2/profiles.py:49-56` of argon2-cffi 25.1.0), stored as a PHC string; the SQL of the store is written by hand, and no query generator enters the project — the queries of this part are twenty, and a generator would add a toolchain step for them; goose and pgx are the established choices and goose's locker is the lock spec 14.2 asks for — a later part with many more queries may revisit the generator, at the cost of rewriting the store |
| R80 | Ruling: the schema of this part holds `users`, `sessions`, `login_rate_limits`, and `audit_events`, with the columns of Task 3; `users.role` is one of `administrator`, `operator`, `reader`, and `users.language` one of `pt-BR`, `en`, both checked by the schema; there is no `platform_settings` table, since spec 3 moves every setting to the deployment file; `audit_events` carries a trigger that raises on `UPDATE` and `DELETE`, where the original kept append-only by convention alone (`migration.py:386-406`, security.md:61) — spec 11.9 calls the table append-only, and a trigger costs one statement — an operator who must correct a row does it as the superuser after dropping the trigger, which is the point |
| R81 | Ruling: the bootstrap administrator and every user created through the API carry a language, `pt-BR` or `en`, required in the create body with no default; `bootstrap-admin` takes `--language`, default `pt-BR` — spec 11.3 gives every user a language and spec 12.4 applies the saved choice after sign-in; a default in the create body would hide a choice the administrator should make — none |
| R82 | Ruling: the deployment file is TOML, read by `serve` and `admin` from `--config`, default `platform.toml`; unknown keys and sections are refused as in `report.toml`; the keys of this part are `session_ttl_seconds` (default 43200, 60 to 2592000), `session_idle_seconds` (default 3600, 60 to 2592000, at most the ttl), `server_secret_path` (required, a file of at least 32 bytes), and `trusted_proxies` (a list of CIDRs, default empty); the DSN comes from `KCR_DATABASE_DSN` alone and must be a `postgres://` or `postgresql://` URL; the keys of later parts arrive with them — spec 11.2 names the file and the variable and lists the settings by topic; the original's `database_dsn_env` indirection goes, since one variable name is enough — a deployment that already names another variable renames it |
| R83 | Ruling: the session cookie is `kcr_session`, `Path=/api`, `Secure`, `HttpOnly`, `SameSite=Lax`, `Max-Age` equal to the ttl, as spec 11.4 fixes it; the request CSRF token is returned in the body of sign-in and of `GET /auth/session` only, and the original's `kcr_csrf` cookie, which the server never compared (`api/auth.py:137-162`), is not ported; `GET /auth/session` of a signed-in reader issues a new CSRF token each time it is called, overwriting the digest of the session row; the sign-in CSRF cookie `kcr_login_csrf` has the same attributes and no `Max-Age`, and is cleared by a successful sign-in — one copy of a secret is one copy; the frontend calls `GET /auth/session` once when it loads and keeps the token in memory (spec 12) — a client that opens two tabs invalidates the older tab's token, which the frontend of phase 4 answers by reloading the session on a 403 |
| R84 | Ruling: the client address is the host of `RemoteAddr`, unless the peer is inside one of `trusted_proxies`, in which case it is the last address of `X-Forwarded-For`; an empty list trusts no proxy — the original trusted `127.0.0.1` through uvicorn, which this program does not have; the address keys the throttle and nothing else, and an untrusted header would let an attacker pick their own key — a deployment behind a proxy that does not list it throttles the proxy's address, which blocks every user after five failures; the operations guide of phase 5 says so |
| R85 | Ruling: the throttle key is HMAC-SHA256 of the normalised username, a NUL byte, and the client address, keyed by the server secret file read whole; five failures in 300 seconds block for 300 seconds; the fifth failure blocks; `Retry-After` is the ceiling of the seconds left, at least 1; the row is read `FOR UPDATE`; a successful sign-in resets the row; a throttled attempt costs no hash, writes no audit row, and answers 429 — the original, letter for letter (`auth.py:421-538`), with the server secret in place of the keyring's derived key, as spec 11.2 says — none |
| R86 | Ruling: a session token is 32 random bytes, base64url without padding; its SHA-256 digest is stored; `expires_at` starts at the smaller of the ttl and the idle window after creation and is moved on every authenticated request to the smaller of `created_at` plus the ttl and now plus the idle window; a session is active when `revoked_at IS NULL AND expires_at > now`; the reasons of revocation are `rotated`, `logout`, `user_inactive`, `privilege_changed`, `password_changed`, `user_deleted`; a sign-in that presents an active session revokes it as `rotated` and records it as the parent of the new one only when both belong to the same user — the original (`auth.py:563-650`, `repositories.py:847-957`) — none |
| R87 | Ruling: `PATCH /account` takes `language`, or `current_password` with `password`, or all three; a wrong current password answers 401 `INVALID_CREDENTIALS` and counts as a failure of the throttle for the reader's own name; a password change revokes every session of the reader, the current one included, and the response clears the cookie, as spec 11.4 says a password change revokes every session; the response is always 200 with the principal — spec 11.10 gives the route one line; the original had no self-service route at all — a reader who changes their password signs in again, which the frontend of phase 4 does for them |
| R88 | Ruling: the error envelope is `{"error":{"code":"<CODE>","message":"<sentence>","fields":{...}}}`, `fields` always present and a map of string to string; the codes and sentences of this part are the original's (`errors.py:124-179`), quoted in Task 9; a body that fails to decode answers 400 `REQUEST_INVALID` with one field per problem whose value is one of `missing`, `invalid`, `unknown`, or `too_long`, never the text of the decoder; the sentences are diagnostics for API clients, in English, held in one table in `internal/platform/api/errors.go`, and `AGENTS.md` says so — spec 11.10: "The message is a fixed English sentence chosen by code, never text from an exception. The frontend shows its own translation of the code" — a sentence a reader sees in the browser comes from the frontend's catalog, never from this table |
| R89 | Ruling: every response carries `Content-Security-Policy: default-src 'self'; script-src 'self'; style-src 'self'; font-src 'self'; img-src 'self' data:; connect-src 'self'; object-src 'none'; base-uri 'none'; frame-ancestors 'none'; form-action 'self'`, `X-Content-Type-Options: nosniff`, `Referrer-Policy: same-origin`, `X-Frame-Options: DENY`, and `Cache-Control: no-store` unless the handler set one; no `Server` header; the pinned hashes of `style-src` arrive with the component library in phase 4, each with its test, as spec 11.12 says; a recovered panic answers 500 `INTERNAL_ERROR` with the same headers — none |
| R90 | Ruling: `GET /health/live` answers 200 `{"status":"live"}`; `GET /health/ready` answers 200 `{"status":"ready","checks":{"database":true,"migrations":true}}` or 503 with `not_ready` and the failing check false; `migrations` is false when goose reports a pending migration, and is not evaluated when the database is down; `static_index` arrives in phase 4 — the original (`health.py:44-83`) without the frontend — none |
| R91 | Ruling: `serve` takes `--config` (default `platform.toml`) and `--listen` (default `:8080`); at start it loads the settings, adds the DSN to the redactor, opens the pool, applies the migrations under the lock, and listens; on `SIGTERM` or `SIGINT` it stops accepting, waits up to 30 seconds for the requests in flight, closes the pool, and exits 0; it logs through `log/slog` in text form to standard error, through the redactor, one line per event: start, stop, every 5xx (method, route pattern, status, code), and nothing per request; the exit codes of `serve` and `admin` are 0, 1 for a failure at run time (database, listen, a refusal of `bootstrap-admin`), and 2 for the configuration, the arguments, or the settings file — spec 5.3 numbers the report's exit codes and says nothing of the platform's; three codes are what a Compose health check and an operator need — none |
| R92 | Ruling: `admin bootstrap-admin --username <name> --password-stdin [--language pt-BR] [--config platform.toml]` refuses first, before reading anything, unless `KCR_BOOTSTRAP_MODE` is exactly `1`; `--password-stdin` is required, and there is no `--password` flag; the password is one line of standard input, with one trailing `\n` and then one trailing `\r` removed; the username and the password are validated before the database is touched; the users table is locked `IN SHARE ROW EXCLUSIVE MODE` and the command refuses when any user exists, active or not; the audit row `auth.bootstrap` has no actor; on success it prints `created administrator <name>` on standard output; a refusal prints one of the sentences of Task 11 and exits 1 — the original (`commands.py:218-358`, `bootstrap.py:39-69`) — none |
| R93 | Ruling: `admin migrate [--config platform.toml]` applies the migrations under the lock `72427301` and prints `migrate: schema at version <n>`; `serve` applies the same migrations the same way before it listens, so a Compose deployment needs no migration step and a Kubernetes deployment may run the command as a Job — spec 14.2 and spec 4.2 — none |
| R94 | Ruling: the PostgreSQL tests carry the build tag `postgres` and read `KCR_TEST_DATABASE_DSN`; `storetest.New(t)` creates a database named `kcr_test_<16 hex>` on the server of that DSN, applies the migrations, returns a pool, and drops the database when the test ends; `make postgres-up` starts `deploy/compose.test.yaml` (PostgreSQL 18 on `127.0.0.1:55432`, user `kcr_test`, password `kcr_test_only`, database `kcr_test`) and waits for health; `make postgres-down` removes it with its volume; `make test-postgres` runs the three in order and removes the server even when the tests fail; the gate runs `test-postgres` after `test`; the lint vets with the tag; a subagent runs the tagged tests with the DSN the main thread gives it — spec 13.5 and the decision of 2026-10-05 — none |
| R95 | Ruling: the audit table and its writer arrive in this part, with the actions `auth.bootstrap`, `auth.login.succeeded`, `auth.login.failed`, `auth.logout`, `user.create`, `user.update`, `user.password_changed`, `user.delete`, `account.password_changed`, `account.language_changed`; the metadata gate refuses a key that matches `password|token|cookie|hash|digest|secret|dsn` after folding, a value that is not a string, a number, a boolean, or null, and a nesting deeper than one level, by returning an error, which the caller treats as a defect (500) — the decision of 2026-10-05 put the audit in 3.3, and sign-in cannot wait for it; 3.3 adds the report and profile actions — none |
| R96 | Ruling: a user has a `language` column, `pt-BR` or `en`, and the principal of every session answer carries it, so the frontend of phase 4 applies it after sign-in — spec 12.4 — none |
| R97 | Ruling: `api/openapi.yaml` is OpenAPI 3.0.3 and describes the routes of this part under `/api/v1`; `make generate` writes the Go types of its schemas into `internal/platform/api/openapi.gen.go` with oapi-codegen, models only, and the gate fails when the committed file differs from a fresh generation; the handlers are written by hand on `net/http`'s `ServeMux` with method patterns, registered from one table of routes, and a test compares that table with the paths and methods of the file both ways, the health routes excluded as spec 11.10 says they are outside the contract — spec 2 says Go and TypeScript types are generated from one file; generating the server too would bind the handlers to a generated interface for no gain at twenty routes — phase 4 generates the TypeScript types from the same file |
| R98 | Ruling: a request body is read with a limit of 64 KiB and decoded with `DisallowUnknownFields`, one JSON value and nothing after it; a repeated key is refused by walking the tokens before decoding, as the snapshot reader does; a string field that is longer than its maximum answers `too_long` — spec 11.10 wants one error shape, and a decoder's own text would be a second one — none |

## Findings this plan closes or records

| Finding | How |
|---|---|
| F-109 (new) | The original never purges `login_rate_limits` or revoked `sessions` rows (`research/original-auth.md` section 3, "Cleanup"). Recorded for the retention job of 3.3 |
| F-110 (new) | The original's `kcr_csrf` cookie was never compared by the server; not ported (R83). Recorded so that the frontend of phase 4 does not look for it |

## File structure

| Path | Responsibility in this part |
|---|---|
| `internal/guard/imports.go`, `imports_test.go`, `buildtags.go`, `binary_test.go` | The six platform ranks, `cli` at 200, the tag `postgres`, `storetest` forbidden in the binary |
| `internal/platform/settings/settings.go`, `settings_test.go` | `Settings`, `Load`, the validation, the DSN, the server secret |
| `internal/platform/store/store.go`, `migrate.go`, `migrations/00001_accounts.sql` | `Open`, `Migrate`, `HasPending`, `Transact`, the schema |
| `internal/platform/store/users.go`, `sessions.go`, `throttle.go`, `audit.go`, `*_test.go` | Every SQL statement of this part and its row type |
| `internal/platform/storetest/storetest.go` | `New(t)`: a fresh database per test |
| `internal/platform/audit/audit.go`, `audit_test.go` | `Event`, `Metadata`, `Append` |
| `internal/platform/auth/username.go`, `password.go`, `token.go`, `throttle.go`, `session.go`, `users.go`, `account.go`, `*_test.go` | The services |
| `internal/platform/api/errors.go`, `headers.go`, `body.go`, `client.go`, `routes.go`, `server.go`, `health.go`, `auth.go`, `account.go`, `users.go`, `openapi.gen.go`, `*_test.go` | The HTTP surface |
| `api/openapi.yaml`, `api/oapi-codegen.yaml` | The contract and the generator's configuration |
| `internal/cli/serve.go`, `admin.go`, `root_test.go`, `serve_test.go`, `admin_test.go` | The commands |
| `deploy/compose.test.yaml` | PostgreSQL 18 for the tests |
| `Makefile` | `BUILD_TAGS`, `generate`, `postgres-up`, `postgres-down`, `test-postgres`, the gate |
| `AGENTS.md`, `README.md`, `CHANGELOG.md` | The documents |

---

### Task 0: Open the part

**Executor:** Main thread.

- [ ] **Step 1:** Run `git status`, `git log --oneline -3`, `git describe --tags`, and `go test -count=1 ./...`. Confirm `main` at `5de4568`, clean, every package green, the changelog's `## [Unreleased]` empty.
- [ ] **Step 2:** Create the branch and confirm it.

```bash
git switch -c feat/platform-accounts
git branch --show-current
```

- [ ] **Step 3:** Record in `ROADMAP.md`: the summary line of 3.1 names this plan and the branch; a section "Phase 3.1" with the items P31-0 to P31-13 of this plan, all `OPEN`; the rulings R78 to R98 as proposed, awaiting the owner's word; F-109 and F-110 in the findings log.
- [ ] **Step 4:** Commit the plan.

```bash
git add docs/superpowers/plans/2026-10-05-phase-3-1-accounts-authentication.md
git commit -m "docs(plan): add the plan of part 3.1"
```

**Done when:** the branch exists, the plan is committed, and `ROADMAP.md` holds the part.

---

### Task 1: The guards and the Makefile learn the platform (R78, R94)

**Executor:** Subagent.

**Files:**
- Modify: `internal/guard/imports.go:21-40` (`Layers`), `internal/guard/imports_test.go:301-327` (`TestLayersHoldEveryPackageOfThisPart`), `internal/guard/buildtags.go:17` (`BuildTags`), `internal/guard/binary_test.go:44-49` (the forbidden list)
- Modify: `Makefile` (`BUILD_TAGS`, the vet lines of `lint`)
- Create: `internal/platform/settings/doc.go`, `internal/platform/store/doc.go`, `internal/platform/storetest/doc.go`, `internal/platform/audit/doc.go`, `internal/platform/auth/doc.go`, `internal/platform/api/doc.go`, each a package clause with a one-sentence package comment, so that `go list ./...` sees the packages the table ranks and the import scan reads a real package from the first commit

**Interfaces:**
- Produces: the ranks below, which every later task relies on.

```go
var Layers = []Layer{
	{Path: "internal/guard", Rank: 0},
	{Path: "internal/arith", Rank: 0},
	{Path: "internal/typst", Rank: 0},
	{Path: "internal/redact", Rank: 0},
	{Path: "internal/promtest", Rank: 0},
	{Path: "template", Rank: 0},
	{Path: "template/golden", Rank: 10},
	{Path: "internal/config", Rank: 20},
	{Path: "internal/prom", Rank: 30},
	{Path: "internal/discovery", Rank: 40},
	{Path: "internal/resolve", Rank: 50},
	{Path: "internal/collect", Rank: 55},
	{Path: "internal/analysis", Rank: 60},
	{Path: "internal/snapshot", Rank: 70},
	{Path: "internal/catalog", Rank: 80},
	{Path: "internal/document", Rank: 90},
	{Path: "internal/goldentest", Rank: 95},
	{Path: "internal/platform/settings", Rank: 110},
	{Path: "internal/platform/store", Rank: 120},
	{Path: "internal/platform/storetest", Rank: 125},
	{Path: "internal/platform/audit", Rank: 130},
	{Path: "internal/platform/auth", Rank: 140},
	{Path: "internal/platform/api", Rank: 150},
	{Path: "internal/cli", Rank: 200},
}

var BuildTags = []string{"golden", "postgres", "template", "toolchain"}
```

The forbidden list of `TestProductionBinaryHoldsNoTestSupport` gains
`module + "/internal/platform/storetest"`. In the Makefile,
`BUILD_TAGS := golden,postgres,template,toolchain`, and `lint` gains the line
`go vet -tags=postgres ./...`.

**Tests:**

| Test | Change | Expected |
|---|---|---|
| `TestLayersHoldEveryPackageOfThisPart` | the map gains the six platform packages and `internal/cli` reads 200; the length check stays | passes with 24 entries |
| `TestLayersHoldThePlatformPackages` (new, same file) | `checkLayerRanks` with the six platform paths and their ranks | passes |
| `TestRepositoryDeclaresEveryBuildTag`, `TestMakefileBuildTagsMatchDeclaredTags` | unchanged | pass with the four tags |
| `TestProductionBinaryHoldsNoTestSupport` | unchanged body, longer list | passes |
| `TestRepositoryImportsPointOneWay` | unchanged | passes with the six empty packages |

- [ ] **Step 1:** Write `TestLayersHoldThePlatformPackages` and change the map of `TestLayersHoldEveryPackageOfThisPart`. Run `go test ./internal/guard/ -run Layers -v`. Expected: both fail, `Layers is missing "internal/platform/settings"`.
- [ ] **Step 2:** Change `Layers`, `BuildTags`, the forbidden list, the Makefile, and create the six `doc.go` files. Run `go test -count=1 ./internal/guard/`. Expected: pass.
- [ ] **Step 3:** Run `make lint` and `make test`. Commit.

```bash
git add internal/guard/imports.go internal/guard/imports_test.go internal/guard/buildtags.go internal/guard/binary_test.go Makefile internal/platform
git commit -m "chore(guard): rank the platform packages"
```

**Done when:** the guard tests pass, `make lint` passes, and `go list ./...` names the six packages.

---

### Task 2: The deployment settings (R82)

**Executor:** Subagent.

**Files:**
- Create: `internal/platform/settings/settings.go`, `internal/platform/settings/settings_test.go`

**Interfaces:**
- Consumes: `github.com/BurntSushi/toml` (already required), `net/netip`.
- Produces:

```go
// Settings is the deployment file of spec 11.2, read once at start by
// serve and admin, plus the DSN of the environment.
type Settings struct {
	SessionTTL       time.Duration // session_ttl_seconds
	SessionIdle      time.Duration // session_idle_seconds
	ServerSecretPath string        // server_secret_path
	TrustedProxies   []netip.Prefix // trusted_proxies
	DatabaseDSN      string        // KCR_DATABASE_DSN; never written to a message
}

// Load reads the TOML file at path, then the environment through getenv,
// and validates the result. An unknown key or section is an error that
// names the key and not its value. The DSN is read from KCR_DATABASE_DSN
// and is required.
func Load(path string, getenv func(string) string) (Settings, error)

// ReadServerSecret reads the file named by ServerSecretPath whole and
// returns its bytes. A file shorter than 32 bytes is refused.
func (s Settings) ReadServerSecret() ([]byte, error)
```

The file is decoded with `toml.DecodeFile` into a struct with the four keys
and `md.Undecoded()` refuses any other key, in the style of
`internal/config/load.go`. The messages, each a `var err... = errors.New(...)`
defined once:

| Condition | Message |
|---|---|
| file unreadable | `cannot read <path>: <os error>` |
| invalid TOML | `<path> is not valid TOML: <line>:<column>` (from `toml.ParseError`) |
| unknown key | `<path>: unknown key <key>` |
| ttl outside 60..2592000 | `session_ttl_seconds must be between 60 and 2592000` |
| idle outside 60..2592000 | `session_idle_seconds must be between 60 and 2592000` |
| idle > ttl | `session_idle_seconds must not exceed session_ttl_seconds` |
| secret path empty | `server_secret_path is required` |
| secret path relative | `server_secret_path must be absolute` |
| a proxy not a CIDR | `trusted_proxies must hold CIDR ranges` (the value is not repeated) |
| DSN unset or empty | `KCR_DATABASE_DSN is not set` |
| DSN scheme other | `KCR_DATABASE_DSN must be a postgres:// or postgresql:// URL` (the value is not repeated) |
| secret file unreadable | `cannot read the server secret at <path>: <os error>`; the path comes from the file the user named, and the bytes are never repeated |
| secret shorter than 32 | `the server secret must hold at least 32 bytes` |

Defaults: ttl 43200 s, idle 3600 s, no proxies. A missing file is an error
(the file is required, unlike the original's optional one), since a
deployment without a server secret cannot start.

**Tests** (`settings_test.go`, every file written under `t.TempDir()`):

| Test | Input | Expected |
|---|---|---|
| `TestLoadDefaults` | a file with `server_secret_path` only; `KCR_DATABASE_DSN=postgres://u:p@h/db` | ttl 12h, idle 1h, no proxies, the DSN |
| `TestLoadEveryKey` | all four keys, `trusted_proxies = ["10.0.0.0/8", "::1/128"]` | each field equals its value |
| `TestLoadRefusesUnknownKey` | `worker_slots = 2` | error `<path>: unknown key worker_slots` |
| `TestLoadRefusesUnknownSection` | `[overrides]` | error names `overrides` |
| `TestLoadRefusesInvalidTOML` | `session_ttl_seconds = ` | error holds `is not valid TOML` and a line and column |
| `TestLoadRangeOfTTL` | 59 and 2592001 | the ttl message |
| `TestLoadRangeOfIdle` | 59 and 2592001 | the idle message |
| `TestLoadIdleOverTTL` | ttl 3600, idle 7200 | `session_idle_seconds must not exceed session_ttl_seconds` |
| `TestLoadRequiresSecretPath` | no key | `server_secret_path is required` |
| `TestLoadRequiresAbsoluteSecretPath` | `secret.bin` | `server_secret_path must be absolute` |
| `TestLoadRefusesBadProxy` | `["10.0.0.1"]` | the proxies message, and the error text does not hold `10.0.0.1` |
| `TestLoadRequiresDSN` | variable unset; variable empty | `KCR_DATABASE_DSN is not set` both times |
| `TestLoadRefusesDSNScheme` | `mysql://x` | the scheme message, and the text does not hold `mysql` |
| `TestLoadMissingFile` | a path that does not exist | error starts with `cannot read ` and names the path |
| `TestReadServerSecret` | a file of 32 bytes; of 31 bytes; a missing file | the bytes; `the server secret must hold at least 32 bytes`; an error without the bytes |

- [ ] **Step 1:** Write the tests. Run `go test ./internal/platform/settings/ -v`. Expected: build failure, then every test red with stubs.
- [ ] **Step 2:** Implement. Run the same. Expected: pass.
- [ ] **Step 3:** Run `make lint`. Commit.

```bash
git add internal/platform/settings
git commit -m "feat(settings): read the deployment file"
```

**Done when:** the fifteen tests pass and no message repeats a DSN or a proxy value.

---

### Task 3: The store, the schema, and the PostgreSQL harness (R79, R80, R94)

**Executor:** Subagent, with the main thread's `make postgres-up` before the dispatch.

**Files:**
- Create: `internal/platform/store/store.go`, `internal/platform/store/migrate.go`, `internal/platform/store/migrations/00001_accounts.sql`, `internal/platform/store/migrate_test.go`
- Create: `internal/platform/storetest/storetest.go`
- Create: `deploy/compose.test.yaml`
- Modify: `Makefile` (`postgres-up`, `postgres-down`, `test-postgres`, the gate), `go.mod`, `go.sum`

**Interfaces:**
- Consumes: `github.com/jackc/pgx/v5` v5.11.0 (`pgxpool`, `stdlib`), `github.com/pressly/goose/v3` v3.28.0 (`goose`, `goose/lock`).
- Produces:

```go
// LockID is the advisory lock every migration run takes, the number of the
// original's entrypoint.
const LockID int64 = 72427301

// Open connects a pool to dsn and pings it once.
func Open(ctx context.Context, dsn string) (*pgxpool.Pool, error)

// Migrate applies every pending migration of the embedded folder under the
// session lock LockID and returns the version the schema is at afterwards.
func Migrate(ctx context.Context, pool *pgxpool.Pool) (int64, error)

// HasPending reports whether a migration of the embedded folder is not yet
// applied. It takes no lock.
func HasPending(ctx context.Context, pool *pgxpool.Pool) (bool, error)

// Transact runs fn inside one transaction, committing when fn returns nil
// and rolling back otherwise, including on a panic, which it re-raises.
func Transact(ctx context.Context, pool *pgxpool.Pool, fn func(pgx.Tx) error) error
```

`Migrate` and `HasPending` build a `*sql.DB` with `stdlib.OpenDBFromPool(pool)`
and a `goose.NewProvider(goose.DialectPostgres, db, migrations,
goose.WithSessionLocker(locker), goose.WithDisableGlobalRegistry(true))`
where `locker, _ := lock.NewPostgresSessionLocker(lock.WithLockID(LockID))`;
the `*sql.DB` is closed after use. The folder is `//go:embed migrations/*.sql`.

The schema, `00001_accounts.sql`, in goose's annotated form:

```sql
-- +goose Up
CREATE TABLE users (
    id            uuid        NOT NULL,
    username      text        NOT NULL,
    password_hash text        NOT NULL,
    role          text        NOT NULL,
    language      text        NOT NULL,
    active        boolean     NOT NULL,
    created_at    timestamptz NOT NULL,
    updated_at    timestamptz NOT NULL,
    CONSTRAINT pk_users PRIMARY KEY (id),
    CONSTRAINT uq_users_username UNIQUE (username),
    CONSTRAINT ck_users_role CHECK (role IN ('administrator', 'operator', 'reader')),
    CONSTRAINT ck_users_language CHECK (language IN ('pt-BR', 'en'))
);

CREATE TABLE sessions (
    id              uuid        NOT NULL,
    token_hash      bytea       NOT NULL,
    csrf_hash       bytea       NOT NULL,
    user_id         uuid        NOT NULL,
    created_at      timestamptz NOT NULL,
    expires_at      timestamptz NOT NULL,
    revoked_at      timestamptz,
    revoked_reason  text,
    rotated_from_id uuid,
    CONSTRAINT pk_sessions PRIMARY KEY (id),
    CONSTRAINT uq_sessions_token_hash UNIQUE (token_hash),
    CONSTRAINT fk_sessions_user FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE RESTRICT,
    CONSTRAINT fk_sessions_rotated_from FOREIGN KEY (rotated_from_id) REFERENCES sessions (id) ON DELETE RESTRICT,
    CONSTRAINT ck_sessions_revocation CHECK ((revoked_at IS NULL) = (revoked_reason IS NULL))
);
CREATE INDEX ix_sessions_active_by_user ON sessions (user_id) WHERE revoked_at IS NULL;

CREATE TABLE login_rate_limits (
    key_hash          bytea       NOT NULL,
    window_started_at timestamptz NOT NULL,
    failures          integer     NOT NULL,
    blocked_until     timestamptz,
    updated_at        timestamptz NOT NULL,
    CONSTRAINT pk_login_rate_limits PRIMARY KEY (key_hash)
);

CREATE TABLE audit_events (
    id          bigint      GENERATED ALWAYS AS IDENTITY,
    actor_id    uuid,
    occurred_at timestamptz NOT NULL,
    action      text        NOT NULL,
    target_type text,
    target_id   text,
    outcome     text        NOT NULL,
    metadata    jsonb       NOT NULL,
    CONSTRAINT pk_audit_events PRIMARY KEY (id),
    CONSTRAINT fk_audit_events_actor FOREIGN KEY (actor_id) REFERENCES users (id) ON DELETE RESTRICT,
    CONSTRAINT ck_audit_events_outcome CHECK (outcome IN ('succeeded', 'failed'))
);

-- +goose StatementBegin
CREATE FUNCTION reject_audit_event_mutation() RETURNS trigger AS $$
BEGIN
    RAISE EXCEPTION 'audit events are append-only' USING ERRCODE = 'integrity_constraint_violation';
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd
CREATE TRIGGER audit_events_are_append_only
    BEFORE UPDATE OR DELETE ON audit_events
    FOR EACH ROW EXECUTE FUNCTION reject_audit_event_mutation();

-- +goose Down
DROP TABLE audit_events;
DROP FUNCTION reject_audit_event_mutation();
DROP TABLE login_rate_limits;
DROP TABLE sessions;
DROP TABLE users;
```

The harness:

```go
// New creates a database on the server of KCR_TEST_DATABASE_DSN, named
// kcr_test_<16 hex characters>, applies the migrations to it, and returns a
// pool connected to it. The database is dropped when t ends. The test
// fails when the variable is unset.
func New(t *testing.T) *pgxpool.Pool

// NewEmpty is New without the migrations: a fresh, empty database.
func NewEmpty(t *testing.T) *pgxpool.Pool
```

`New` connects to the DSN as given, runs `CREATE DATABASE <name>`, builds a
second DSN with the path replaced by the name (through `net/url`), opens the
pool with `store.Open`, runs `store.Migrate`, and registers a cleanup that
closes the pool and runs `DROP DATABASE <name>` through the first
connection. The name is validated against `^kcr_test_[0-9a-f]{16}$` before
it is written into SQL. The file carries `//go:build postgres`.

`deploy/compose.test.yaml`:

```yaml
# PostgreSQL alone, for `make test-postgres`. The image is pinned by tag
# and by index digest, as the Containerfile pins its bases.
name: kcr-test

services:
  postgres:
    image: postgres:18-alpine@sha256:77f585114c32fbca283dc835b0596f4e52b51b4c6662d7810b2f4084f60a1873
    environment:
      POSTGRES_DB: kcr_test
      POSTGRES_USER: kcr_test
      POSTGRES_PASSWORD: kcr_test_only
    ports:
      - "127.0.0.1:55432:5432"
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U kcr_test -d kcr_test"]
      interval: 1s
      timeout: 3s
      retries: 30
```

The Makefile:

```make
TEST_DSN := postgres://kcr_test:kcr_test_only@127.0.0.1:55432/kcr_test?sslmode=disable

postgres-up:
	docker compose --file deploy/compose.test.yaml up --detach --wait

postgres-down:
	docker compose --file deploy/compose.test.yaml down --volumes

# Starts PostgreSQL, runs the tagged tests, and removes the server even when
# a test fails; the exit status is the tests'.
test-postgres:
	@set -o pipefail; $(MAKE) postgres-up; \
	KCR_TEST_DATABASE_DSN="$(TEST_DSN)" go test $(TEST_FLAGS) -tags=postgres ./internal/platform/... ./internal/cli/...; \
	status=$$?; $(MAKE) postgres-down; exit $$status
```

and the gate gains, after the `test` line,
`@set -o pipefail; $(MAKE) test-postgres TEST_FLAGS="-count=1" 2>&1 | tee $(GATE_LOGS)/test-postgres.log`.
`.PHONY` lists the three targets.

**Tests** (`migrate_test.go`, `//go:build postgres`):

| Test | Input | Expected |
|---|---|---|
| `TestNewRefusesUnsetVariable` (in `storetest`, tagged) | runs `go test -tags=postgres -run TestNewProbe .` as a child process with the variable removed from its environment; `TestNewProbe`, in the same file, calls `New` and is selected only by that child | the child fails and its output holds `KCR_TEST_DATABASE_DSN is not set` |
| `TestMigrateIsIdempotent` | `storetest.New` (which migrated once), then `store.Migrate` again | version 1 both times, no error, `HasPending` false |
| `TestHasPendingOnEmptyDatabase` | a database created without migrating (the harness exposes `NewEmpty(t)` for this) | true before `Migrate`, false after |
| `TestSchemaRefusesBadRole` | `INSERT INTO users ... role = 'user'` | an error whose SQLSTATE is `23514` |
| `TestSchemaRefusesBadLanguage` | `language = 'fr'` | `23514` |
| `TestAuditEventsAreAppendOnly` | insert a row, then `UPDATE` and `DELETE` it | both fail with SQLSTATE `23000` and the message holds `append-only` |
| `TestSessionRevocationIsPaired` | insert a session with `revoked_at` set and `revoked_reason` null | `23514` |
| `TestTransactCommits` | `Transact` inserting a user and returning nil | the row exists |
| `TestTransactRollsBack` | `Transact` inserting a user and returning an error | no row; the error is returned |
| `TestTransactRollsBackOnPanic` | `fn` panics | the panic reaches the caller, no row |

- [ ] **Step 1:** Add the modules: `go get github.com/jackc/pgx/v5@v5.11.0 github.com/pressly/goose/v3@v3.28.0`, then `go mod tidy`. Run `go test ./internal/guard/ -run Forbidden`. Expected: pass.
- [ ] **Step 2:** Write the tests and the harness. Run `KCR_TEST_DATABASE_DSN=<the DSN of the brief> go test -tags=postgres ./internal/platform/store/ -v`. Expected: build failure on the missing names, then red.
- [ ] **Step 3:** Write the schema, `store.go`, `migrate.go`, the Compose file, and the Makefile targets. Run the same. Expected: pass. Run it twice in a row.
- [ ] **Step 4:** Run `make lint`, `make test`, and `go vet -tags=postgres ./...`. Commit.

```bash
git add go.mod go.sum internal/platform/store internal/platform/storetest deploy/compose.test.yaml Makefile
git commit -m "feat(store): add the schema and the migrations"
```

**Done when:** the ten tests pass twice against the server, `make test` passes without the server, and the main thread has run `make test-postgres` once end to end, watching it bring the server up and down.

---

### Task 4: The queries of the store

**Executor:** Subagent, against the running server.

**Files:**
- Create: `internal/platform/store/users.go`, `sessions.go`, `throttle.go`, `audit.go`, and `users_test.go`, `sessions_test.go`, `throttle_test.go`, `audit_test.go` (all tests `//go:build postgres`)

**Interfaces:**
- Consumes: `pgx.Tx`, `github.com/google/uuid`, the schema of Task 3.
- Produces:

```go
// Querier is what every statement of this package runs on: a pool or a
// transaction.
type Querier interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

type User struct {
	ID           uuid.UUID
	Username     string
	PasswordHash string
	Role         string
	Language     string
	Active       bool
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// ErrNotFound is returned by every lookup that finds no row.
var ErrNotFound = errors.New("no such row")

// ErrUsernameTaken is returned by InsertUser when the unique constraint
// refuses the name.
var ErrUsernameTaken = errors.New("username taken")

func InsertUser(ctx context.Context, q Querier, u User) error
func GetUser(ctx context.Context, q Querier, id uuid.UUID) (User, error)
// FindActiveUserByUsername finds the active user of the given stored name.
func FindActiveUserByUsername(ctx context.Context, q Querier, username string) (User, error)
// ListUsers returns every user ordered by username.
func ListUsers(ctx context.Context, q Querier) ([]User, error)
// UpdateUser writes role, language, active, password_hash, and updated_at
// of the row with u.ID. The username is never written.
func UpdateUser(ctx context.Context, q Querier, u User) error
// CountActiveAdministrators counts users with role administrator and active.
func CountActiveAdministrators(ctx context.Context, q Querier) (int, error)
// LockUsersAndCheckEmpty locks the table IN SHARE ROW EXCLUSIVE MODE for
// the transaction and reports whether it holds no row at all.
func LockUsersAndCheckEmpty(ctx context.Context, tx pgx.Tx) (bool, error)

type Session struct {
	ID            uuid.UUID
	TokenHash     []byte
	CSRFHash      []byte
	UserID        uuid.UUID
	CreatedAt     time.Time
	ExpiresAt     time.Time
	RevokedAt     *time.Time
	RevokedReason *string
	RotatedFromID *uuid.UUID
}

func InsertSession(ctx context.Context, q Querier, s Session) error
// TouchSession moves expires_at of the active session with this token hash
// to least(created_at + ttl, now + idle) and returns the row after the
// move. ErrNotFound when no active session has the hash.
func TouchSession(ctx context.Context, q Querier, tokenHash []byte, now time.Time, ttl, idle time.Duration) (Session, error)
func FindActiveSessionByTokenHash(ctx context.Context, q Querier, tokenHash []byte, now time.Time) (Session, error)
func FindActiveSessionByID(ctx context.Context, q Querier, id uuid.UUID, now time.Time) (Session, error)
// ReplaceSessionCSRF overwrites csrf_hash of the active session with id,
// and returns ErrNotFound when no active session has it.
func ReplaceSessionCSRF(ctx context.Context, q Querier, id uuid.UUID, csrfHash []byte, now time.Time) error
// RevokeSession sets revoked_at and revoked_reason where revoked_at IS NULL.
// It reports whether a row changed.
func RevokeSession(ctx context.Context, q Querier, id uuid.UUID, reason string, now time.Time) (bool, error)
// RevokeSessionsOfUser revokes every unrevoked session of the user and
// returns the count.
func RevokeSessionsOfUser(ctx context.Context, q Querier, userID uuid.UUID, reason string, now time.Time) (int, error)

type Throttle struct {
	KeyHash         []byte
	WindowStartedAt time.Time
	Failures        int
	BlockedUntil    *time.Time
	UpdatedAt       time.Time
}

// GetThrottleForUpdate reads the row and locks it for the transaction.
func GetThrottleForUpdate(ctx context.Context, tx pgx.Tx, keyHash []byte) (Throttle, error)
func GetThrottle(ctx context.Context, q Querier, keyHash []byte) (Throttle, error)
// UpsertThrottle inserts the row or replaces every column but the key.
func UpsertThrottle(ctx context.Context, q Querier, t Throttle) error

type AuditRow struct {
	ActorID    *uuid.UUID
	OccurredAt time.Time
	Action     string
	TargetType *string
	TargetID   *string
	Outcome    string
	Metadata   []byte // a JSON object, already checked by the audit package
}

func InsertAuditEvent(ctx context.Context, q Querier, r AuditRow) error
// ListAuditEvents returns every row in id order; a reader for tests.
func ListAuditEvents(ctx context.Context, q Querier) ([]AuditEvent, error)

type AuditEvent struct {
	ID int64
	AuditRow
}
```

`TouchSession` is one statement:

```sql
UPDATE sessions
   SET expires_at = LEAST(created_at + $3::interval, $2::timestamptz + $4::interval)
 WHERE token_hash = $1 AND revoked_at IS NULL AND expires_at > $2
RETURNING id, token_hash, csrf_hash, user_id, created_at, expires_at, revoked_at, revoked_reason, rotated_from_id
```

with `$3` and `$4` passed as `pgtype.Interval` or as the text of the
duration in seconds; the implementer picks one and pins it with the test
`TestTouchSessionCapsAtAbsoluteLifetime`. `InsertUser` maps SQLSTATE `23505`
on `uq_users_username` to `ErrUsernameTaken` through `*pgconn.PgError`.
Every lookup maps `pgx.ErrNoRows` to `ErrNotFound`.

**Tests** (every test opens `storetest.New(t)`; `now` is a literal
`time.Date(2026, 10, 5, 12, 0, 0, 0, time.UTC)`; every expected time is a
literal):

| Test | Input | Expected |
|---|---|---|
| `TestInsertAndGetUser` | a user | `GetUser` returns every field equal, times in UTC |
| `TestInsertUserRefusesTakenName` | the same username twice | second returns `ErrUsernameTaken` |
| `TestGetUserNotFound` | a random id | `ErrNotFound` |
| `TestFindActiveUserByUsername` | an active and an inactive user | the active is found; the inactive returns `ErrNotFound` |
| `TestListUsersOrdersByUsername` | `zed`, `amy`, `kim` | `amy`, `kim`, `zed` |
| `TestUpdateUserNeverWritesUsername` | update with `Username` changed | the stored name is unchanged; role, language, active, hash, updated_at are |
| `TestCountActiveAdministrators` | two active administrators, one inactive, one operator | 2 |
| `TestLockUsersAndCheckEmpty` | empty table; one user | true; false. A second transaction opened while the first holds the lock blocks on its own `LockUsersAndCheckEmpty` until the first commits (proved with a goroutine and a channel, 2 s timeout) |
| `TestInsertAndFindSession` | a session with `ExpiresAt` now+1h | found by hash at now; `ErrNotFound` at now+2h; `ErrNotFound` by a wrong hash |
| `TestTouchSessionSlides` | created now, expires now+1h, ttl 12h, idle 1h; touch at now+30m | `ExpiresAt` now+1h30m |
| `TestTouchSessionCapsAtAbsoluteLifetime` | created now, ttl 12h, idle 1h; touch at now+11h30m | `ExpiresAt` now+12h |
| `TestTouchSessionIgnoresExpiredAndRevoked` | expired; revoked | `ErrNotFound` both |
| `TestReplaceSessionCSRF` | replace on an active session; on a revoked one | the hash changes; the revoked row is unchanged and the call returns `ErrNotFound` |
| `TestRevokeSessionOnce` | revoke twice | true then false; `RevokedReason` is the first reason |
| `TestRevokeSessionsOfUser` | three sessions of a user, one already revoked, and one of another user | 2; the other user's is untouched |
| `TestRotatedFromReferencesASession` | `RotatedFromID` of a random id | SQLSTATE `23503` |
| `TestThrottleUpsert` | insert, then upsert with failures 3 and a block | `GetThrottle` returns the second state |
| `TestGetThrottleForUpdateLocks` | two transactions on one key | the second blocks until the first commits |
| `TestGetThrottleNotFound` | a random key | `ErrNotFound` |
| `TestInsertAuditEventAndList` | two rows, one with no actor | listed in id order, every field equal, metadata bytes equal |
| `TestAuditActorMustExist` | a random actor id | `23503` |

- [ ] **Step 1:** Write the tests. Run `KCR_TEST_DATABASE_DSN=... go test -tags=postgres ./internal/platform/store/ -v`. Expected: build failure, then red.
- [ ] **Step 2:** Implement. Run the same, twice. Expected: pass.
- [ ] **Step 3:** Run `make lint` and `go vet -tags=postgres ./...`. Commit.

```bash
git add internal/platform/store
git commit -m "feat(store): add the queries of the accounts"
```

**Done when:** the twenty-one tests pass twice and no statement calls `now()`.

---

### Task 5: The audit writer and the metadata gate (R95)

**Executor:** Subagent, against the running server for one test.

**Files:**
- Create: `internal/platform/audit/audit.go`, `internal/platform/audit/audit_test.go` (no tag), `internal/platform/audit/append_test.go` (`//go:build postgres`)

**Interfaces:**
- Consumes: `store.InsertAuditEvent`, `store.AuditRow`, `store.Querier`.
- Produces:

```go
// The actions of this part. A later part appends its own; a constant is
// never renamed.
const (
	ActionBootstrap              = "auth.bootstrap"
	ActionLoginSucceeded         = "auth.login.succeeded"
	ActionLoginFailed            = "auth.login.failed"
	ActionLogout                 = "auth.logout"
	ActionUserCreate             = "user.create"
	ActionUserUpdate             = "user.update"
	ActionUserPasswordChanged    = "user.password_changed"
	ActionUserDelete             = "user.delete"
	ActionAccountPasswordChanged = "account.password_changed"
	ActionAccountLanguageChanged = "account.language_changed"
)

const (
	OutcomeSucceeded = "succeeded"
	OutcomeFailed    = "failed"
)

// Metadata is the document of an event: a map whose values are strings,
// numbers, booleans, nil, or one more map of those.
type Metadata map[string]any

// ErrMetadata is wrapped by every refusal of Encode.
var ErrMetadata = errors.New("audit metadata refused")

// Encode checks m and returns its JSON. It refuses a key that matches
// password|token|cookie|hash|digest|secret|dsn after case folding, a value
// of another type, and a map inside a map.
func Encode(m Metadata) ([]byte, error)

type Event struct {
	ActorID    *uuid.UUID
	Action     string
	TargetType string // empty means none
	TargetID   string // empty means none
	Outcome    string // OutcomeSucceeded when empty
	Metadata   Metadata
}

// Append encodes the metadata and inserts one row at now.
func Append(ctx context.Context, q store.Querier, now time.Time, e Event) error
```

Numbers accepted by `Encode`: `int`, `int64`, `float64`. Keys are matched
with `regexp.MustCompile("password|token|cookie|hash|digest|secret|dsn")`
on `cases.Fold().String(key)`. The JSON has its keys sorted
(`encoding/json` sorts a map).

**Tests:**

| Test | Input | Expected |
|---|---|---|
| `TestEncodeAcceptsScalars` | `{"role": "operator", "count": 2, "ok": true, "none": nil}` | `{"count":2,"none":null,"ok":true,"role":"operator"}` |
| `TestEncodeAcceptsOneLevel` | `{"changed": {"role": "reader"}}` | the nested JSON |
| `TestEncodeRefusesForbiddenKeys` | each of `password`, `new_token`, `Cookie`, `DIGEST`, `secret_ref`, `dsn`, `password_hash`, `HASH` | `ErrMetadata`, and the error names the key |
| `TestEncodeRefusesNestedKeys` | `{"changed": {"password": "x"}}` | refused |
| `TestEncodeRefusesDeepNesting` | `{"a": {"b": {"c": 1}}}` | refused |
| `TestEncodeRefusesOtherTypes` | a slice; a struct; a `time.Time` | refused |
| `TestEncodeEmpty` | `nil`; `Metadata{}` | `{}` both |
| `TestAppendWritesRow` (postgres) | an event with an actor, a target, and metadata | one row, every column equal, outcome `succeeded` |
| `TestAppendDefaultsOutcome` (postgres) | `Outcome` empty | `succeeded` |
| `TestAppendRefusedMetadataWritesNothing` (postgres) | a forbidden key | error, zero rows |

- [ ] **Step 1:** Write the tests. Run `go test ./internal/platform/audit/ -v` and the tagged run. Expected: red.
- [ ] **Step 2:** Implement. Run both. Expected: pass.
- [ ] **Step 3:** `make lint`. Commit.

```bash
git add internal/platform/audit
git commit -m "feat(audit): add the writer and its gate"
```

**Done when:** the ten tests pass.

---

### Task 6: Usernames, passwords, tokens, and the throttle (R85)

**Executor:** Subagent, against the running server for the throttle tests.

**Files:**
- Create: `internal/platform/auth/username.go`, `password.go`, `token.go`, `throttle.go`, and `username_test.go`, `password_test.go`, `token_test.go`, `throttle_test.go` (the throttle's decision tests untagged, its database tests in `throttle_db_test.go` with `//go:build postgres`)
- Modify: `go.mod`, `go.sum` (`github.com/alexedwards/argon2id` v1.0.0)

**Interfaces:**
- Produces:

```go
// NormalizeUsername applies NFKC, Unicode case folding, and trims white
// space, as the original's normalize_username does.
func NormalizeUsername(raw string) string

// ErrInvalidUsername is returned when the normalised name does not match
// ^[a-z0-9][a-z0-9._-]{0,63}$.
var ErrInvalidUsername = errors.New("invalid username")

// ValidateUsername normalises raw and returns the stored form, or
// ErrInvalidUsername.
func ValidateUsername(raw string) (string, error)

// UsernameMatchesPattern reports whether a normalised name matches the
// pattern, for the sign-in path, which never refuses a name but skips the
// lookup of one that cannot be stored.
func UsernameMatchesPattern(normalized string) bool

var (
	ErrPasswordTooShort = errors.New("password too short")
	ErrPasswordTooLong  = errors.New("password too long")
)

// CheckPassword applies the policy: 12 to 1024 code points.
func CheckPassword(password string) error

// HashParams are the pinned Argon2id parameters.
var HashParams = &argon2id.Params{Memory: 65536, Iterations: 3, Parallelism: 4, SaltLength: 16, KeyLength: 32}

// HashPassword returns the PHC string of password under HashParams.
func HashPassword(password string) (string, error)

// VerifyPassword reports whether password matches hash and whether the
// hash was made with other parameters than HashParams and should be
// written again. A hash that cannot be parsed matches nothing and needs
// a rehash.
func VerifyPassword(password, hash string) (match, needsRehash bool)

// DummyHash is a hash of a random password, made once per process, so
// that an unknown user costs one verification like a known one.
func DummyHash() string

// NewToken returns 32 random bytes as base64url without padding.
func NewToken() (string, error)

// Digest is the SHA-256 of the UTF-8 bytes of token.
func Digest(token string) []byte

// Throttle is the sign-in rate limit of spec 11.4.
type Throttle struct {
	Secret []byte // the server secret
}

const (
	ThrottleMaxFailures = 5
	ThrottleWindow      = 300 * time.Second
	ThrottleBlock       = 300 * time.Second
)

// Key is HMAC-SHA256 of normalised username, a NUL byte, and the client
// address, under the secret.
func (t Throttle) Key(username, clientAddress string) []byte

// ErrThrottled carries the seconds a client must wait.
type ErrThrottled struct{ RetryAfter int }

func (e *ErrThrottled) Error() string

// Check returns *ErrThrottled when the key is blocked at now.
func (t Throttle) Check(ctx context.Context, q store.Querier, key []byte, now time.Time) error

// RecordFailure counts one failure in a transaction, starting a window
// when none is open or the open one ran out, and blocks on the fifth.
func (t Throttle) RecordFailure(ctx context.Context, pool *pgxpool.Pool, key []byte, now time.Time) error

// RecordSuccess resets the row when it exists.
func (t Throttle) RecordSuccess(ctx context.Context, q store.Querier, key []byte, now time.Time) error

// Decide is the pure part of Check: the seconds to wait and whether the
// key is blocked at now.
func Decide(row *store.Throttle, now time.Time) (retryAfter int, blocked bool)
```

`Decide`: no row or `BlockedUntil == nil` means allowed; `remaining :=
BlockedUntil.Sub(now)`; `remaining <= 0` allowed; else
`int(math.Ceil(remaining.Seconds()))`, at least 1. `RecordFailure` runs
`GetThrottleForUpdate` inside `store.Transact`; when `ErrNotFound` or
`now.Sub(WindowStartedAt) >= ThrottleWindow`, the window starts at now with
one failure; otherwise failures plus one; `BlockedUntil` is `now +
ThrottleBlock` when failures reach `ThrottleMaxFailures`, else nil; then
`UpsertThrottle`.

**Tests:**

| Test | Input | Expected |
|---|---|---|
| `TestNormalizeUsernameFoldsAndTrims` | `"  Admin "`; `"Ⅷ"`; `"ﬁsh"`; `"Straße"` | `admin`; `viii`; `fish`; `strasse` |
| `TestValidateUsernameAccepts` | `a`, `a.b_c-d`, 64 characters | the normalised name |
| `TestValidateUsernameRefuses` | empty; `.a`; 65 characters; `a b`; `"a\x00b"`; `"José"` (ends `josé`, non-ASCII) | `ErrInvalidUsername` each |
| `TestValidateUsernameRefusesControlCharacters` | `"a\nb"`, `"a\tb"` | refused |
| `TestUsernameMatchesPattern` | `admin`; `Admin` (not normalised); `a\x00` | true; false; false |
| `TestCheckPassword` | 11 ASCII letters; 12; 1024; 1025; 12 runes of `á` (24 bytes); 1024 runes of `é` (2048 bytes) | short; ok; ok; long; ok; ok |
| `TestHashPasswordPins` | hash `"correct horse battery staple"` | starts with `$argon2id$v=19$m=65536,t=3,p=4$` |
| `TestVerifyPasswordMatches` | the hash and the password; a wrong password | true, false; false, false |
| `TestVerifyPasswordNeedsRehash` | a hash made with `Iterations: 1` through the library directly | true, true |
| `TestVerifyPasswordGarbage` | `"nothash"` | false, true |
| `TestDummyHashIsStableAndRandom` | call twice in one process | equal; not equal to a hash of the empty string |
| `TestNewTokenShape` | two tokens | 43 characters each, base64url alphabet, no `=`, different |
| `TestDigestIsSHA256` | `"abc"` | the 32 bytes of SHA-256(`abc`), as a literal hex `ba7816bf...` |
| `TestThrottleKeyIsHMACWithNUL` | secret `"k"`, user `"Admin"`, address `"10.0.0.1"` | equals `hmac.New(sha256.New, []byte("k"))` over `"admin\x0010.0.0.1"`, computed in the test; differs for `("adm", "in\x0010...")` |
| `TestDecide` | nil row; row with nil block; block 0.5 s ahead; block 299.2 s ahead; block 1 s behind | allowed; allowed; 1 blocked; 300 blocked; allowed |
| `TestRecordFailureBlocksOnFifth` (postgres) | five failures at now, now+1s, ..., now+4s | after the fourth `Check` passes; after the fifth `Check` returns `ErrThrottled{300}` at now+4s, `{299}` at now+5s |
| `TestRecordFailureStartsNewWindowAfter300s` (postgres) | four failures at now; a fifth at now+300s | failures 1, no block |
| `TestRecordSuccessResets` (postgres) | three failures then success; success on an unknown key | failures 0, no block; no row inserted |
| `TestBlockExpires` (postgres) | the block of the fifth failure | `Check` at now+304s passes |

- [ ] **Step 1:** `go get github.com/alexedwards/argon2id@v1.0.0 && go mod tidy`. Write the tests. Run `go test ./internal/platform/auth/ -v` and the tagged run. Expected: red.
- [ ] **Step 2:** Implement. Run both. Expected: pass.
- [ ] **Step 3:** `make lint`. Commit.

```bash
git add go.mod go.sum internal/platform/auth
git commit -m "feat(auth): add usernames, passwords, and throttle"
```

**Done when:** the nineteen tests pass and the hash prefix is pinned.

---

### Task 7: The sessions service (R83, R86)

**Executor:** Subagent, against the running server.

**Files:**
- Create: `internal/platform/auth/session.go`, `internal/platform/auth/session_test.go` (`//go:build postgres`)

**Interfaces:**
- Consumes: Task 4's session and user queries, Task 5's `audit.Append`, Task 6.
- Produces:

```go
// Principal is what a session proves: the account as the database holds it
// at this request.
type Principal struct {
	ID       uuid.UUID
	Username string
	Role     string
	Language string
}

type Role = string

const (
	RoleAdministrator Role = "administrator"
	RoleOperator      Role = "operator"
	RoleReader        Role = "reader"
)

// Sessions signs readers in and out and authenticates their requests.
type Sessions struct {
	Pool     *pgxpool.Pool
	Throttle Throttle
	TTL      time.Duration
	Idle     time.Duration
	Now      func() time.Time
}

var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrUnauthenticated    = errors.New("unauthenticated")
	ErrCSRF               = errors.New("csrf rejected")
)

// Issued is a new session: the raw tokens, given to the client once.
type Issued struct {
	SessionID uuid.UUID
	Token     string
	CSRFToken string
	ExpiresAt time.Time
	Principal Principal
}

// Login runs the sign-in of spec 11.4: the throttle, one hash
// verification, the audit row, the rotation of a presented session, the
// new session. presented is the session cookie of the request, or empty.
func (s *Sessions) Login(ctx context.Context, username, password, clientAddress, presented string) (Issued, error)

// Authenticated is a request's session after the slide.
type Authenticated struct {
	SessionID uuid.UUID
	ExpiresAt time.Time
	Principal Principal
}

// Authenticate slides the session of the cookie and re-reads the account.
// A missing, expired, or revoked session, and an inactive account, answer
// ErrUnauthenticated; an inactive account revokes the session first.
func (s *Sessions) Authenticate(ctx context.Context, token string) (Authenticated, error)

// VerifyCSRF compares the header token with the digest of the session row.
func (s *Sessions) VerifyCSRF(ctx context.Context, sessionID uuid.UUID, headerToken string) error

// RotateCSRF issues a new CSRF token for the session and stores its digest.
func (s *Sessions) RotateCSRF(ctx context.Context, sessionID uuid.UUID) (string, error)

// Logout revokes the session of the token as "logout" and writes the audit
// row. An unknown token is not an error.
func (s *Sessions) Logout(ctx context.Context, token string) error
```

The order inside `Login`, as `auth.py:563-620`:

1. `normalized := NormalizeUsername(username)`; `key := Throttle.Key(normalized, clientAddress)`; `Throttle.Check`; `*ErrThrottled` returns as is.
2. In one transaction: `user, err := FindActiveUserByUsername(normalized)` only when `UsernameMatchesPattern(normalized)`; `stored := DummyHash()` when not found; `match, rehash := VerifyPassword(password, stored)`; when found and match: rehash when asked (`UpdateUser` with the new hash); otherwise append `auth.login.failed` with no actor, metadata `{"code": "INVALID_CREDENTIALS"}`, target `user` and the id when the user exists, outcome `failed`; the transaction commits either way.
3. On failure: `Throttle.RecordFailure`, then `ErrInvalidCredentials`.
4. On success: `Throttle.RecordSuccess`; mint `token` and `csrf`; in one transaction: when `presented != ""`, find the active session of its digest, revoke it as `rotated`, and keep its id as the parent only when its user is the principal; insert the session with `ExpiresAt = now + min(TTL, Idle)`; append `auth.login.succeeded` with the actor, target `session` and the id.

`Authenticate`: `TouchSession`; `ErrNotFound` to `ErrUnauthenticated`; `GetUser`; when missing or inactive, `RevokeSession(..., "user_inactive")` and `ErrUnauthenticated`.

**Tests** (each with two users made through `store.InsertUser` with hashes of `HashPassword`, a `Sessions` with a fixed `Now` that the test advances by reassigning, ttl 12h, idle 1h, a secret of 32 bytes):

| Test | Input | Expected |
|---|---|---|
| `TestLoginIssuesSession` | right name and password | a token of 43 characters, a CSRF token, `ExpiresAt` now+1h, the principal with role and language; one session row with both digests; one audit row `auth.login.succeeded` with the actor and the session id |
| `TestLoginWrongPassword` | wrong password | `ErrInvalidCredentials`; audit `auth.login.failed` with target user id and outcome failed; throttle row with one failure |
| `TestLoginUnknownUser` | a name that matches the pattern but exists not | `ErrInvalidCredentials`; audit row with no target; one failure |
| `TestLoginInactiveUserLooksUnknown` | an inactive user, right password | `ErrInvalidCredentials`; audit row with no target |
| `TestLoginNameOutsidePatternSkipsLookup` | `"a\x00b"` | `ErrInvalidCredentials`, no panic, one failure counted |
| `TestLoginThrottledWritesNoAudit` | five wrong passwords, then the right one | `*ErrThrottled` with 300; exactly five `auth.login.failed` rows |
| `TestLoginRotatesPresentedSession` | sign in, sign in again presenting the first token | the first row revoked `rotated`; the second row's `RotatedFromID` is the first id |
| `TestLoginDoesNotChainAcrossAccounts` | user A signs in; user B signs in presenting A's token | A's row revoked `rotated`; B's `RotatedFromID` nil |
| `TestLoginRehashesOutdatedHash` | a user whose hash was made with `Iterations: 1` | after sign-in the stored hash starts with the pinned prefix |
| `TestLoginNormalisesUsername` | `"  ADMIN "` | signs in as `admin` |
| `TestAuthenticateSlides` | sign in; authenticate at now+30m | `ExpiresAt` now+1h30m; principal equal |
| `TestAuthenticateExpired` | authenticate at now+61m | `ErrUnauthenticated` |
| `TestAuthenticateRevokesSessionOfInactiveUser` | deactivate the user; authenticate | `ErrUnauthenticated`; the row revoked `user_inactive` |
| `TestAuthenticateReadsRoleFromDatabase` | change the role to reader in the table; authenticate | principal role `reader` |
| `TestVerifyCSRF` | the issued token; a wrong one; empty; a revoked session | nil; `ErrCSRF`; `ErrCSRF`; `ErrCSRF` |
| `TestRotateCSRF` | rotate | a new token verifies, the old one no longer |
| `TestLogout` | logout with the token | the row revoked `logout`; audit `auth.logout` with the actor; `Authenticate` then `ErrUnauthenticated` |
| `TestLogoutUnknownToken` | a random token | nil, no audit row |

- [ ] **Step 1:** Write the tests. Run the tagged tests of the package. Expected: red.
- [ ] **Step 2:** Implement. Run twice. Expected: pass.
- [ ] **Step 3:** `make lint`. Commit.

```bash
git add internal/platform/auth
git commit -m "feat(auth): add the sessions service"
```

**Done when:** the eighteen tests pass twice.

---

### Task 8: The users and account services (R81, R87)

**Executor:** Subagent, against the running server.

**Files:**
- Create: `internal/platform/auth/users.go`, `account.go`, and `users_test.go`, `account_test.go` (`//go:build postgres`)

**Interfaces:**
- Produces:

```go
// Users is the administration of accounts, spec 11.3.
type Users struct {
	Pool *pgxpool.Pool
	Now  func() time.Time
}

var (
	ErrForbidden      = errors.New("forbidden")
	ErrUserNotFound   = errors.New("user not found")
	ErrUsernameTaken  = store.ErrUsernameTaken
	ErrLastAdmin      = errors.New("last administrator")
	ErrSelfDelete     = errors.New("self delete")
	ErrSelfDeactivate = errors.New("self deactivate")
	ErrSelfDemote     = errors.New("self demote")
	ErrInvalidRole    = errors.New("invalid role")
	ErrInvalidLanguage = errors.New("invalid language")
)

type NewUser struct {
	Username string
	Password string
	Role     Role
	Language string
}

// Create validates the name, the password, the role, and the language,
// hashes, inserts, and audits user.create with {"role": role}. actor must
// be an administrator.
func (u *Users) Create(ctx context.Context, actor Principal, n NewUser) (store.User, error)

type UserChange struct {
	Role     *string
	Active   *bool
	Password *string
	Language *string
}

// Update applies the fields that are set, in the order of the original:
// the self checks, the load, the moved fields, the last-administrator rule,
// then the password; a role or active change revokes every session as
// privilege_changed, a password change as password_changed. It audits
// user.update with {"changed": {"role": "<new>", ...}} naming the fields
// that moved and their new values for role, active, and language only, plus
// {"sessions_revoked": n}; a password change audits user.password_changed
// with {"sessions_revoked": n}. A change that moves nothing writes nothing.
func (u *Users) Update(ctx context.Context, actor Principal, id uuid.UUID, c UserChange) (store.User, error)

// Delete deactivates the user and revokes every session as user_deleted,
// auditing user.delete with {"sessions_revoked": n}.
func (u *Users) Delete(ctx context.Context, actor Principal, id uuid.UUID) error

func (u *Users) Get(ctx context.Context, actor Principal, id uuid.UUID) (store.User, error)
func (u *Users) List(ctx context.Context, actor Principal) ([]store.User, error)

// Account is the reader's own account, spec 11.10 PATCH /account.
type Account struct {
	Pool     *pgxpool.Pool
	Sessions *Sessions
	Now      func() time.Time
}

type AccountChange struct {
	CurrentPassword *string
	Password        *string
	Language        *string
}

// Change applies a language change and, when Password is set, verifies
// CurrentPassword, checks the policy, writes the hash, revokes every
// session of the reader as password_changed, and audits both actions. A
// wrong current password returns ErrInvalidCredentials and counts a
// failure of the throttle under the reader's name and clientAddress.
func (a *Account) Change(ctx context.Context, p Principal, clientAddress string, c AccountChange) (store.User, error)
```

The `LAST_ADMIN` rule: refused when the target is an active administrator,
the change deactivates or demotes them, and `CountActiveAdministrators`
is at most 1, counted inside the transaction.

**Tests** (an administrator `root`, an operator `ops`, a reader `eve`, each
inserted directly):

| Test | Input | Expected |
|---|---|---|
| `TestCreateUser` | `root` creates `kim`, operator, `en`, a 12-letter password | the row, hash with the pinned prefix, audit `user.create` with `{"role":"operator"}` and the target id |
| `TestCreateRefusesNonAdministrator` | `ops` creates | `ErrForbidden`, no row |
| `TestCreateValidatesInOrder` | bad name and short password; good name and short password; good both and role `boss`; language `fr` | `ErrInvalidUsername`; `ErrPasswordTooShort`; `ErrInvalidRole`; `ErrInvalidLanguage`; no row and no audit each time |
| `TestCreateRefusesTakenName` | `kim` twice; `KIM` after `kim` | `ErrUsernameTaken` both |
| `TestUpdateRoleRevokesSessions` | `ops` has two sessions; `root` sets role reader | role reader; both revoked `privilege_changed`; audit `user.update` with `{"changed":{"role":"reader"},"sessions_revoked":2}` |
| `TestUpdateActiveFalseRevokes` | `root` deactivates `ops` | inactive; revoked `privilege_changed` |
| `TestUpdatePasswordRevokes` | `root` sets a new password for `eve` with one session | the hash changes; revoked `password_changed`; audit `user.password_changed` `{"sessions_revoked":1}` |
| `TestUpdateLanguageOnly` | `root` sets `en` on `eve` with a session | the language; the session is not revoked; audit `{"changed":{"language":"en"},"sessions_revoked":0}` |
| `TestUpdateNoOpWritesNothing` | `root` sets role operator on `ops` | no audit row; `updated_at` unchanged |
| `TestUpdateSelfChecks` | `root` sets own active false; own role operator | `ErrSelfDeactivate`; `ErrSelfDemote` |
| `TestUpdateRefusesLastAdministratorTransitions` | a second administrator `two` deactivates `root` while `two` is inactive (so `root` is the last active); `two` active demotes `root` | `ErrLastAdmin`; allowed (two active administrators) |
| `TestUpdateUnknownUser` | a random id | `ErrUserNotFound` |
| `TestUpdateShortPasswordCostsNoHash` | a password of 3 letters | `ErrPasswordTooShort`, the hash unchanged |
| `TestDeleteDeactivates` | `root` deletes `eve` with a session | inactive; revoked `user_deleted`; audit `user.delete` |
| `TestDeleteSelf` | `root` deletes `root` | `ErrSelfDelete` |
| `TestDeleteLastAdministrator` | `two` (inactive) tries; then `two` active deletes `root` | `ErrLastAdmin`; allowed |
| `TestListAndGet` | `ops` lists; `root` lists; `root` gets `eve` | `ErrForbidden`; three in name order; the row |
| `TestAccountChangeLanguage` | `eve` sets `en` | the language; audit `account.language_changed` with `{"language":"en"}`; her session survives |
| `TestAccountChangePassword` | `eve` with the right current password and a new one, two sessions | the hash changes; both revoked `password_changed`; audit `account.password_changed` `{"sessions_revoked":2}` |
| `TestAccountWrongCurrentPassword` | wrong current | `ErrInvalidCredentials`; hash unchanged; one throttle failure under `eve` and the address |
| `TestAccountPasswordWithoutCurrent` | `Password` set, `CurrentPassword` nil | `ErrInvalidCredentials` |
| `TestAccountNoChange` | an empty change | the row, no audit |

- [ ] **Step 1:** Write the tests. Run the tagged tests. Expected: red.
- [ ] **Step 2:** Implement. Run twice. Expected: pass.
- [ ] **Step 3:** `make lint`. Commit.

```bash
git add internal/platform/auth
git commit -m "feat(auth): add the users and account services"
```

**Done when:** the twenty-two tests pass twice.

---

### Task 9: The HTTP skeleton: errors, headers, bodies, client address, health (R84, R88, R89, R90, R98)

**Executor:** Subagent, with one tagged test.

**Files:**
- Create: `internal/platform/api/errors.go`, `headers.go`, `body.go`, `client.go`, `routes.go`, `server.go`, `health.go`, and `errors_test.go`, `headers_test.go`, `body_test.go`, `client_test.go`, `server_test.go`, `health_test.go` (`health_test.go` tagged `postgres`)

**Interfaces:**
- Produces:

```go
// Code is an error code of the contract. The table below holds its status
// and sentence.
type Code string

const (
	CodeRequestInvalid     Code = "REQUEST_INVALID"
	CodeInvalidCredentials Code = "INVALID_CREDENTIALS"
	CodeUnauthenticated    Code = "UNAUTHENTICATED"
	CodeCSRFRejected       Code = "CSRF_REJECTED"
	CodeForbidden          Code = "FORBIDDEN"
	CodeUserNotFound       Code = "USER_NOT_FOUND"
	CodeNotFound           Code = "NOT_FOUND"
	CodeMethodNotAllowed   Code = "METHOD_NOT_ALLOWED"
	CodeUsernameTaken      Code = "USERNAME_TAKEN"
	CodeLastAdmin          Code = "LAST_ADMIN"
	CodeSelfDelete         Code = "SELF_DELETE"
	CodeSelfDeactivate     Code = "SELF_DEACTIVATE"
	CodeSelfDemote         Code = "SELF_DEMOTE"
	CodeInvalidUsername    Code = "INVALID_USERNAME"
	CodePasswordTooShort   Code = "PASSWORD_TOO_SHORT"
	CodePasswordTooLong    Code = "PASSWORD_TOO_LONG"
	CodeAuthRateLimited    Code = "AUTH_RATE_LIMITED"
	CodeDatabaseUnavailable Code = "DATABASE_UNAVAILABLE"
	CodeInternalError      Code = "INTERNAL_ERROR"
)

// Problem is one failure to answer.
type Problem struct {
	Code       Code
	Fields     map[string]string // nil means {}
	RetryAfter int               // seconds; written as Retry-After when > 0
}

func (p *Problem) Error() string

// WriteProblem writes the envelope with the status and sentence of the
// table, and the headers of the route.
func WriteProblem(w http.ResponseWriter, p *Problem)

// Table of the codes: status and sentence, the original's letter for letter.
var problems = map[Code]struct {
	Status   int
	Sentence string
}{
	CodeRequestInvalid:      {400, "The request could not be parsed."},
	CodeInvalidCredentials:  {401, "The username or password is incorrect."},
	CodeUnauthenticated:     {401, "Authentication is required."},
	CodeCSRFRejected:        {403, "The request could not be verified."},
	CodeForbidden:           {403, "You are not allowed to perform this action."},
	CodeUserNotFound:        {404, "No such user was found."},
	CodeNotFound:            {404, "The requested resource was not found."},
	CodeMethodNotAllowed:    {405, "This method is not allowed for this resource."},
	CodeUsernameTaken:       {409, "This username is already in use."},
	CodeLastAdmin:           {409, "The last active administrator cannot be changed this way."},
	CodeSelfDelete:          {409, "You cannot remove your own account."},
	CodeSelfDeactivate:      {409, "You cannot deactivate your own account."},
	CodeSelfDemote:          {409, "You cannot demote your own account."},
	CodeInvalidUsername:     {422, "This username is not valid."},
	CodePasswordTooShort:    {422, "This password is too short."},
	CodePasswordTooLong:     {422, "This password is too long."},
	CodeAuthRateLimited:     {429, "Too many login attempts. Try again later."},
	CodeDatabaseUnavailable: {503, "The database is temporarily unavailable."},
	CodeInternalError:       {500, "An unexpected error occurred."},
}

// SecurityHeaders wraps next: every response carries the five headers of
// R89 (Cache-Control only when the handler set none) and a recovered panic
// answers 500 INTERNAL_ERROR with the same headers.
func SecurityHeaders(next http.Handler) http.Handler

// ClientAddress returns the host of r.RemoteAddr, or, when that host is
// inside one of trusted, the last address of X-Forwarded-For.
func ClientAddress(r *http.Request, trusted []netip.Prefix) string

// MaxBody is the limit on a request body.
const MaxBody = 64 << 10

// DecodeBody reads at most MaxBody bytes of r and decodes one JSON object
// into v, refusing an unknown key, a repeated key, a wrong type, a missing
// required field, and anything after the object. It returns a Problem
// REQUEST_INVALID whose fields name each problem with missing, invalid,
// unknown, or too_long. required lists the keys that must be present;
// maxLen gives the longest value of a string key.
func DecodeBody(r *http.Request, v any, required []string, maxLen map[string]int) *Problem

// Route is one line of the table the server registers and the contract
// test reads.
type Route struct {
	Method  string
	Pattern string // the path, as the ServeMux pattern without the method
	Handler http.HandlerFunc
}

// Server builds the handler of serve.
type Server struct {
	Pool           *pgxpool.Pool
	Settings       settings.Settings
	Sessions       *auth.Sessions
	Users          *auth.Users
	Account        *auth.Account
	Now            func() time.Time
	Logger         *slog.Logger
}

// Routes returns the table of every route under /api/v1, in a fixed order,
// without the health routes.
func (s *Server) Routes() []Route

// Handler registers the routes, the health routes, the 404 and 405
// answers in the envelope, and wraps everything in SecurityHeaders.
func (s *Server) Handler() http.Handler
```

`Handler` registers each route as `mux.HandleFunc(method+" "+pattern,
handler)` and a catch-all `mux.HandleFunc("/", ...)` answering 404 in the
envelope for any path under `/api/`, and a plain text `Not Found` for other
paths (the frontend of phase 4 takes that place). A method that the mux
knows for the pattern but not this method: `net/http` answers 405 with
`Allow`; the server wraps the mux so that a 405 is rewritten into the
envelope (`CodeMethodNotAllowed`) while keeping `Allow`: use a
`http.ResponseWriter` that intercepts `WriteHeader(405)` and discards the
mux's body. `health.go` registers `GET /health/live` and `GET /health/ready`;
ready runs `SELECT 1` on the pool with a 2 s timeout, then
`store.HasPending` only when that passed. A 5xx is logged once at `Error`
level with `method`, `route`, `status`, `code`.

**Tests** (every HTTP test uses `httptest.NewRecorder` and the handler of a
`Server` whose pool is nil unless tagged):

| Test | Input | Expected |
|---|---|---|
| `TestProblemTableIsComplete` | every `Code` constant | has an entry with status and a sentence ending in a full stop |
| `TestWriteProblemShape` | `Problem{Code: CodeForbidden}` | 403, `application/json`, body exactly `{"error":{"code":"FORBIDDEN","message":"You are not allowed to perform this action.","fields":{}}}` plus a newline |
| `TestWriteProblemFieldsAndRetryAfter` | fields `{"username":"missing"}`, retry 7 | fields in the body; `Retry-After: 7` |
| `TestSecurityHeadersOnEveryResponse` | a handler writing 200; one writing 404; one panicking | the five headers with the exact values of R89 on each; the panic answers 500 with the INTERNAL_ERROR envelope and the headers |
| `TestSecurityHeadersKeepsHandlerCacheControl` | a handler setting `Cache-Control: private, max-age=0, must-revalidate` | kept |
| `TestNoServerHeader` | any response | no `Server` header |
| `TestClientAddressUsesRemoteAddr` | `RemoteAddr "203.0.113.5:4321"`, no proxies, header `X-Forwarded-For: 10.0.0.1` | `203.0.113.5` |
| `TestClientAddressIgnoresForwardedFromUntrustedPeer` | proxies `[10.0.0.0/8]`, peer `203.0.113.5`, header `1.2.3.4` | `203.0.113.5` |
| `TestClientAddressTrustsListedProxy` | proxies `[10.0.0.0/8]`, peer `10.1.1.1:80`, header `203.0.113.9`; the same peer with header `1.1.1.1, 203.0.113.9`; the same peer with no header | `203.0.113.9`; `203.0.113.9` (the last address); `10.1.1.1` |
| `TestClientAddressIPv6` | peer `[::1]:5` | `::1` |
| `TestClientAddressNoPort` | `RemoteAddr "unix"` (no port) | `unix` returned as is |
| `TestDecodeBodyRefusals` | unknown key; repeated key; wrong type (`"username": 1`); missing required; `{} {}` (two documents); 65 KiB body; empty body; `null`; a string over `maxLen` | each a `Problem` `REQUEST_INVALID` with fields: `{"extra":"unknown"}`, `{"username":"invalid"}`, `{"username":"invalid"}`, `{"password":"missing"}`, `{"":"invalid"}` (the body as a whole, key empty), `{"":"too_long"}`, `{"":"missing"}`, `{"":"invalid"}`, `{"username":"too_long"}` |
| `TestDecodeBodyAccepts` | a well-formed body | nil, fields filled |
| `TestRoutesAreUnique` | `Routes()` | no two routes share method and pattern; every pattern starts with `/api/v1/` |
| `TestUnknownAPIPathIsEnvelope` | `GET /api/v1/nothing` | 404 NOT_FOUND envelope |
| `TestUnknownOtherPathIsPlain` | `GET /nothing` | 404, `text/plain`, body `Not Found` |
| `TestWrongMethodIsEnvelope` | `DELETE /api/v1/auth/session` | 405 METHOD_NOT_ALLOWED envelope with `Allow` holding `GET` |
| `TestHealthLive` | `GET /health/live` | 200 `{"status":"live"}` |
| `TestHealthReady` (postgres) | the harness pool | 200 `{"status":"ready","checks":{"database":true,"migrations":true}}` |
| `TestHealthReadyWithoutMigrations` (postgres) | `storetest.NewEmpty` | 503, `migrations` false, `database` true |
| `TestHealthReadyDatabaseDown` (postgres) | a pool built with `pgxpool.New` (which connects lazily) on `postgres://x:y@127.0.0.1:1/x` | 503, `database` false, `migrations` false |
| `TestHealthRoutesNeedNoSession` | both routes without a cookie | not 401 |

- [ ] **Step 1:** Write the tests. Run `go test ./internal/platform/api/ -v` and the tagged run. Expected: red.
- [ ] **Step 2:** Implement. Run both. Expected: pass.
- [ ] **Step 3:** `make lint`. Commit.

```bash
git add internal/platform/api
git commit -m "feat(api): add the envelope, headers, and health"
```

**Done when:** the twenty-three tests pass and the sentence table equals the original's for every code it holds.

---

### Task 10: The auth and account routes (R83, R87)

**Executor:** Subagent, against the running server.

**Files:**
- Create: `internal/platform/api/cookies.go`, `middleware.go`, `auth.go`, `account.go`, and `auth_test.go`, `account_test.go` (`//go:build postgres`), `cookies_test.go` (untagged)

**Interfaces:**
- Produces:

```go
const (
	SessionCookie   = "kcr_session"
	LoginCSRFCookie = "kcr_login_csrf"
	CSRFHeader      = "X-CSRF-Token"
	CookiePath      = "/api"
)

// sessionCookie builds the cookie of R83 with the given Max-Age in seconds;
// 0 means none (a browser-session cookie); -1 clears it.
func sessionCookie(name, value string, maxAge int) *http.Cookie

// SessionHandler is a handler of a route that needs a signed-in reader.
type SessionHandler func(w http.ResponseWriter, r *http.Request, a auth.Authenticated)

// requireSession authenticates the request and hands the Authenticated to
// next; 401 otherwise, clearing the cookie.
func (s *Server) requireSession(next SessionHandler) http.HandlerFunc

// requireRole wraps requireSession and answers 403 when the role is not
// one of roles.
func (s *Server) requireRole(roles []auth.Role, next SessionHandler) http.HandlerFunc

// requireCSRF wraps a SessionHandler: it verifies X-CSRF-Token against the
// session before calling next; 403 otherwise.
func (s *Server) requireCSRF(next SessionHandler) SessionHandler
```

Routes of this task, in `Routes()` order:

| Method and path | Checks | Body | Answer |
|---|---|---|---|
| `GET /api/v1/auth/session` | none | none | anonymous: 200 `{"authenticated":false,"login_csrf":"<t>"}` and `Set-Cookie: kcr_login_csrf=<t>; Path=/api; Secure; HttpOnly; SameSite=Lax`; signed in: 200 `{"authenticated":true,"principal":{"id","username","role","language"},"csrf_token":"<new>","expires_at":"<RFC3339>"}` |
| `POST /api/v1/auth/login` | login CSRF: cookie `kcr_login_csrf` and header `X-CSRF-Token` both present and equal (constant time), else 403; then the body `{"username","password"}` (both required, username at most 256, password at most 1024 — a longer one is `too_long`, never hashed) | | 200 the signed-in shape; `Set-Cookie: kcr_session=<token>; Path=/api; Max-Age=<ttl>; Secure; HttpOnly; SameSite=Lax`; and `kcr_login_csrf` cleared (`Max-Age=0`) |
| `POST /api/v1/auth/logout` | session, CSRF | none | 204; `kcr_session` cleared |
| `PATCH /api/v1/account` | session, CSRF, body `{"current_password"?,"password"?,"language"?}` | | 200 the principal `{"id","username","role","language"}`; when the password changed, `kcr_session` cleared in the same response |

Errors: `ErrThrottled` to 429 `AUTH_RATE_LIMITED` with `Retry-After`;
`ErrInvalidCredentials` to 401; `ErrPasswordTooShort/Long` to 422; `ErrInvalidLanguage` to 400 `REQUEST_INVALID` with `{"language":"invalid"}`; a database error to 503 `DATABASE_UNAVAILABLE` when it is a connection error (`pgconn.SafeToRetry` or a timeout), 500 otherwise, logged.

**Tests** (a harness `newTestServer(t)` in `api_test.go` that builds a
`Server` on `storetest.New(t)`, seeds `root` administrator and `eve`
reader with known passwords, and offers `signIn(t, username)` returning
the session cookie and the CSRF token by running the two requests):

| Test | Input | Expected |
|---|---|---|
| `TestSessionAnonymous` | `GET /auth/session` | 200, `authenticated` false, `login_csrf` 43 characters, the cookie with the exact attributes `Path=/api; HttpOnly; Secure; SameSite=Lax` and no `Max-Age` |
| `TestLoginRequiresLoginCSRF` | no cookie; cookie without header; cookie and a different header | 403 `CSRF_REJECTED` each; no session row |
| `TestLoginSucceeds` | cookie and header equal, right body | 200, the shape, `principal.role` `administrator`, `expires_at` equals now+1h in RFC3339 UTC; `Set-Cookie` of `kcr_session` with `Max-Age=43200; Path=/api; HttpOnly; Secure; SameSite=Lax`; `kcr_login_csrf` with `Max-Age=0` |
| `TestLoginWrongPassword` | wrong | 401 `INVALID_CREDENTIALS`, no cookie |
| `TestLoginBodyRefusals` | missing password; unknown key; a password of 1025 letters | 400 with the fields; the 1025 case is `{"password":"too_long"}` and no audit row |
| `TestLoginThrottled` | five wrong, then right | 429, `Retry-After: 300`, the sentence |
| `TestLoginCSRFBeforeThrottle` | a throttled client with a bad CSRF | 403, not 429 |
| `TestSessionSignedIn` | `GET /auth/session` with the cookie | 200, `authenticated` true, a `csrf_token` that differs from the one of sign-in and now verifies |
| `TestLogout` | with cookie and CSRF | 204; `kcr_session` cleared; the row revoked `logout`; a second `GET /auth/session` is anonymous |
| `TestLogoutRequiresCSRF` | cookie, no header | 403 |
| `TestLogoutWithoutSession` | nothing | 401 |
| `TestDeactivatedUserIsSignedOutOnNextRequest` | sign in as `eve`; deactivate the row; `GET /auth/session` | 200 anonymous shape; the row revoked `user_inactive`; `Set-Cookie` clears `kcr_session` |
| `TestAccountLanguage` | `PATCH /account` `{"language":"en"}` as `eve` | 200 principal with `en`; cookie kept |
| `TestAccountPassword` | right current, new password | 200; `kcr_session` cleared; the old cookie then answers 401 |
| `TestAccountWrongCurrent` | wrong current | 401 `INVALID_CREDENTIALS` |
| `TestAccountShortPassword` | 3 letters | 422 `PASSWORD_TOO_SHORT` |
| `TestAccountInvalidLanguage` | `fr` | 400 `{"language":"invalid"}` |
| `TestAccountRequiresCSRF` | no header | 403 |
| `TestSessionCookieShape` (untagged) | `sessionCookie("kcr_session", "v", 43200)` | `String()` equals `kcr_session=v; Path=/api; Max-Age=43200; HttpOnly; Secure; SameSite=Lax` |

- [ ] **Step 1:** Write the tests. Run the tagged tests. Expected: red.
- [ ] **Step 2:** Implement. Run twice. Expected: pass.
- [ ] **Step 3:** `make lint`. Commit.

```bash
git add internal/platform/api
git commit -m "feat(api): add sign-in, sign-out, and the account"
```

**Done when:** the nineteen tests pass twice.

---

### Task 11: The user routes

**Executor:** Subagent, against the running server.

**Files:**
- Create: `internal/platform/api/users.go`, `internal/platform/api/users_test.go` (`//go:build postgres`)

Routes, appended to `Routes()`:

| Method and path | Checks | Body | Answer |
|---|---|---|---|
| `GET /api/v1/admin/users` | session, administrator | | 200 `{"users":[<user>...]}` in name order |
| `POST /api/v1/admin/users` | session, administrator, CSRF, body `{"username","password","role","language"}` all required | | 201 `<user>` |
| `GET /api/v1/admin/users/{id}` | session, administrator | | 200 `<user>`; a malformed id or an unknown one 404 `USER_NOT_FOUND` |
| `PATCH /api/v1/admin/users/{id}` | session, administrator, CSRF, body `{"role"?,"active"?,"password"?,"language"?}` | | 200 `<user>` |
| `DELETE /api/v1/admin/users/{id}` | session, administrator, CSRF | | 204 |

`<user>` is `{"id","username","role","language","active","created_at","updated_at"}`,
times in RFC3339 UTC, never the hash. Errors map: `ErrUsernameTaken` 409,
`ErrLastAdmin` 409, `ErrSelfDelete` 409, `ErrSelfDeactivate` 409,
`ErrSelfDemote` 409, `ErrInvalidUsername` 422, `ErrPasswordTooShort/Long`
422, `ErrInvalidRole` 400 `{"role":"invalid"}`, `ErrInvalidLanguage` 400
`{"language":"invalid"}`, `ErrUserNotFound` 404.

**Tests:**

| Test | Input | Expected |
|---|---|---|
| `TestUsersListAsAdministrator` | `root` | 200, two users in order, no `password_hash` key in the body |
| `TestUsersListAsReader` | `eve` | 403 `FORBIDDEN` |
| `TestUsersListWithoutSession` | nothing | 401 |
| `TestUsersCreate` | `root` creates `kim` operator `en` | 201, the user; `GET /admin/users/{id}` returns it |
| `TestUsersCreateRefusals` | missing role; role `boss`; short password; name `.x`; name `eve` again | 400 `{"role":"missing"}`; 400 `{"role":"invalid"}`; 422 `PASSWORD_TOO_SHORT`; 422 `INVALID_USERNAME`; 409 `USERNAME_TAKEN` |
| `TestUsersCreateRequiresCSRF` | no header | 403 |
| `TestUsersPatchRole` | `root` sets `eve` operator | 200 role operator; `eve`'s session now 401 |
| `TestUsersPatchSelfDemote` | `root` sets own role reader | 409 `SELF_DEMOTE` |
| `TestUsersPatchLastAdmin` | seed a second administrator `two`, inactive; `root` signs in and sends `PATCH /admin/users/{two}` `{"active":false}`; then `{"role":"reader"}` on `root`'s own id | 200 (a no-op on an inactive user); 409 `SELF_DEMOTE` |
| `TestUsersPatchLastAdminByAnother` | `two` active; `two` signs in and sends `PATCH /admin/users/{root}` `{"role":"reader"}` | 200, since two administrators are active; `root`'s session answers 401 afterwards |
| `TestUsersErrorMapping` (untagged) | the function that turns a service error into a `*Problem`, called with each of `ErrUsernameTaken`, `ErrLastAdmin`, `ErrSelfDelete`, `ErrSelfDeactivate`, `ErrSelfDemote`, `ErrInvalidUsername`, `ErrPasswordTooShort`, `ErrPasswordTooLong`, `ErrInvalidRole`, `ErrInvalidLanguage`, `ErrUserNotFound`, `ErrForbidden`, and an unknown error | the codes of the table above in that order, `FORBIDDEN` for `ErrForbidden`, and `INTERNAL_ERROR` for the unknown one |
| `TestUsersDelete` | `root` deletes `eve` | 204; `eve` inactive; her session 401 |
| `TestUsersDeleteSelf` | `root` deletes `root` | 409 `SELF_DELETE` |
| `TestUsersUnknownID` | `GET`, `PATCH`, `DELETE` on a random uuid; on `not-a-uuid` | 404 `USER_NOT_FOUND` each |
| `TestUsersPatchNoOp` | `{}` | 200 unchanged, no audit row |

- [ ] **Step 1:** Write the tests. Run the tagged tests. Expected: red.
- [ ] **Step 2:** Implement. Run twice. Expected: pass.
- [ ] **Step 3:** `make lint`. Commit.

```bash
git add internal/platform/api
git commit -m "feat(api): add the user routes"
```

**Done when:** the fifteen tests pass twice.

---

### Task 12: The OpenAPI file and the generated types (R97)

**Executor:** Main thread writes `api/openapi.yaml` (API design stays on the main thread); a subagent wires the generation and the comparison test.

**Files:**
- Create: `api/openapi.yaml`, `api/oapi-codegen.yaml`, `internal/platform/api/openapi.gen.go`, `internal/platform/api/contract_test.go`
- Modify: `Makefile` (`generate`, `GENERATE_CHECK`, the gate), `internal/platform/api/auth.go`, `account.go`, `users.go` (the body and answer structs become the generated types)

`api/oapi-codegen.yaml`:

```yaml
package: api
output: internal/platform/api/openapi.gen.go
generate:
  models: true
output-options:
  skip-prune: true
```

The Makefile:

```make
OAPI_CODEGEN := github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@v2.8.0

generate:
	go run $(OAPI_CODEGEN) -config api/oapi-codegen.yaml api/openapi.yaml
	gofmt -w internal/platform/api/openapi.gen.go

# Fails when a fresh generation differs from the committed file.
generate-check: generate
	git diff --exit-code -- internal/platform/api/openapi.gen.go
```

and the gate gains `generate-check` before `lint`.

`api/openapi.yaml` (OpenAPI 3.0.3): `info.title` `k8s-capacity-report API`,
`info.version` read as `1`; `servers: [{url: /api/v1}]`; the seven paths of
Tasks 10 and 11 with their methods, request bodies, and responses; the
schemas `Error`, `ErrorBody` (`code`, `message`, `fields` as
`additionalProperties: string`), `Principal`, `AnonymousSession`,
`SignedInSession`, `LoginRequest`, `AccountChange`, `User`, `UserList`,
`NewUser`, `UserChange`, and the enums `Role` (`administrator`, `operator`,
`reader`) and `Language` (`pt-BR`, `en`); the security scheme
`cookieAuth` (`apiKey` in `cookie`, name `kcr_session`); every error
response of a route listed with its code in the description. The main
thread writes it with the handlers of Tasks 10 and 11 beside it.

The contract test reads `api/openapi.yaml` with `gopkg.in/yaml.v3` v3.0.1,
which enters `go.mod` as a dependency of this one test file. A Kubernetes
YAML module is forbidden by the guard and is not used.

```go
func TestRoutesMatchTheContract(t *testing.T) {
	// paths of the file: for each path and each method, "<METHOD> /api/v1<path>"
	// routes of the program: for each Route, "<Method> <Pattern>"
	// the two sets must be equal; the failure names each missing side
}
```

**Tests:**

| Test | Input | Expected |
|---|---|---|
| `TestRoutesMatchTheContract` | the file and `Routes()` | equal sets |
| `TestContractNamesEveryCode` | every `Code` constant except `INTERNAL_ERROR`, `DATABASE_UNAVAILABLE`, `NOT_FOUND`, `METHOD_NOT_ALLOWED` | appears in the file text at least once |
| `TestGeneratedTypesAreUsed` | compile-time: the handlers of Tasks 10 and 11 decode into `LoginRequest`, `AccountChange`, `NewUser`, `UserChange` and encode `SignedInSession`, `AnonymousSession`, `Principal`, `User`, `UserList`, `Error` | the package builds and the tests of Tasks 10 and 11 still pass |

- [ ] **Step 1 (main thread):** Write `api/openapi.yaml`. Commit it.
- [ ] **Step 2 (subagent):** Write the config, run `make generate`, replace the hand-written body and answer types with the generated ones, write the two tests. Run `go test ./internal/platform/api/` and the tagged run. Expected: pass, and `make generate-check` passes.
- [ ] **Step 3:** `make lint`. Commit.

```bash
git add api internal/platform/api Makefile go.mod go.sum
git commit -m "feat(api): generate the types from the contract"
```

**Done when:** `make generate-check` passes, the three tests pass, and every route of the program is in the file.

---

### Task 13: The commands `serve` and `admin` (R91, R92, R93)

**Executor:** Subagent, against the running server.

**Files:**
- Create: `internal/cli/serve.go`, `internal/cli/admin.go`, `internal/cli/platform.go` (what both share: loading the settings, the redactor, the pool), `internal/cli/serve_test.go`, `internal/cli/admin_test.go` (`//go:build postgres`), `internal/cli/platform_test.go` (untagged: the arguments and the messages that need no database)
- Modify: `internal/cli/run.go:208` (`AddCommand` for `serve` and `admin`), `internal/cli/run.go:40-60` (`Env` gains `Stdin io.Reader`, `RealEnv` sets `os.Stdin`), `internal/cli/root_test.go` (every `want` that names the commands reads `admin, report, serve`; the walks of `TestGuardNoticesAMissingCommand` and `TestRootHelpListsOnlyItsCommands` expect `admin`, `help`, `report`, `serve`, and `admin` has `bootstrap-admin`, `migrate`, `probe`)

**Interfaces:**

```go
// openPlatform loads the settings of --config, adds the DSN to the
// redactor, and opens the pool. It is the first thing serve and the admin
// commands that need the database do.
func openPlatform(ctx context.Context, env Env, redactor *redact.Redactor, configPath string) (settings.Settings, *pgxpool.Pool, error)
```

`serve`: flags `--config` (default `platform.toml`), `--listen` (default
`:8080`). Order: `openPlatform` (an error is exit 2 when it comes from
`settings.Load` or `ReadServerSecret`, 1 when from `store.Open`); `store.Migrate`
(1); build `api.Server` with the services; `net.Listen` (1, message `cannot
listen on <address>: <os error>`); log `serve: listening on <address>`;
`http.Server{Handler, ReadHeaderTimeout: 10 * time.Second}`; on `ctx.Done()`
log `serve: stopping`, `Shutdown` with 30 s, close the pool, exit 0.
`admin migrate --config`: `openPlatform`, `store.Migrate`, print
`migrate: schema at version <n>` on standard output, exit 0.

`admin bootstrap-admin --username <name> --password-stdin [--language pt-BR] [--config platform.toml]`:

1. `env.Getenv("KCR_BOOTSTRAP_MODE") != "1"`: `error: bootstrap-admin is off; set KCR_BOOTSTRAP_MODE=1 to run it`, exit 1.
2. `--password-stdin` absent: `error: the administrator password is read from standard input; pass --password-stdin and write it there`, exit 2.
3. Read one line of `env.Stdin` (`bufio.Reader.ReadString('\n')`, EOF without a newline accepted), remove one `\n`, then one `\r`.
4. `auth.ValidateUsername`: `error: the username is not valid`, exit 1. `auth.CheckPassword`: `error: the password is too short` or `too long`, exit 1. `--language` not `pt-BR` or `en`: `error: --language must be pt-BR or en`, exit 2.
5. `openPlatform`; in `store.Transact`: `LockUsersAndCheckEmpty`; not empty: `error: a user already exists; bootstrap-admin creates only the first`, exit 1; insert the administrator (`uuid.NewRandom` through `env.NewID`, times from `env.Now`); `audit.Append` `auth.bootstrap` without actor, metadata `{"role":"administrator"}`, target `user` and the id.
6. Print `created administrator <name>`, exit 0.

`admin probe <url>`: `GET` with a 5 s timeout; 200 prints `probe: ok`
exit 0; another status prints `error: probe: <status code>` exit 1; a
connection error `error: probe: cannot reach <url>: <error>` exit 1. The
URL is repeated: the user gave it.

**Tests:**

| Test | Input | Expected |
|---|---|---|
| `TestRootListsThreeCommands` (root_test) | the root | `admin, report, serve` in every message that lists them |
| `TestAdminListsItsCommands` | `admin` alone | help naming `bootstrap-admin`, `migrate`, `probe`; `admin hunter2` answers `error: unknown command; the commands are: bootstrap-admin, migrate, probe` exit 2 without `hunter2` |
| `TestServeRefusesArguments` | `serve extra` | `error: serve takes no argument; found 1`, exit 2 |
| `TestServeMissingConfig` | `serve --config nope.toml` | exit 2, `error: cannot read nope.toml: ...` |
| `TestServeRedactsDSN` (untagged) | a settings file and a DSN whose password is `hunter2secret`, pointing at `127.0.0.1:1` | exit 1, the message holds `[redacted]` (`redact.Mark`) and not `hunter2secret` |
| `TestServeListensAndStops` (postgres) | `serve --listen 127.0.0.1:<port>`, the port found free by the test, run in a goroutine; the context is cancelled after `GET /health/ready` answers 200 | exit 0; the log holds `listening on` and `stopping` |
| `TestServeEndToEnd` (postgres) | `bootstrap-admin` through `Run` with stdin, then `serve`, then over HTTP: anonymous session, login, create an operator, the operator signs in and changes their password, the old cookie is 401 | every status as the routes say; this is the proof of the part |
| `TestBootstrapOff` | variable unset; `true` | the off message, exit 1, nothing read from stdin (a reader that fails the test when read) |
| `TestBootstrapNeedsStdinFlag` | no `--password-stdin` | exit 2, the message |
| `TestBootstrapNoPasswordFlag` | `--password x` | exit 2, `error: unknown flag: --password`, and `x` not in the output |
| `TestBootstrapReadsOneLine` (postgres) | stdin `"correct horse battery staple\r\n"` then more | the password is the line without `\r\n`; signing in over the services with it works |
| `TestBootstrapValidatesBeforeDatabase` | bad username with a settings file pointing at a closed port | exit 1 `the username is not valid`, in under 1 s (no connection attempt) |
| `TestBootstrapRefusesSecondUser` (postgres) | twice | second: exit 1, the message; one row |
| `TestBootstrapAudits` (postgres) | once | audit `auth.bootstrap`, no actor, `{"role":"administrator"}` |
| `TestBootstrapLanguage` (postgres) | `--language en`; `--language fr` | `en` stored; exit 2 |
| `TestMigrate` (postgres) | `admin migrate` on `storetest.NewEmpty` | stdout `migrate: schema at version 1`, exit 0; again: the same |
| `TestProbe` | a test server answering 200; 503; a closed port | `probe: ok` 0; `error: probe: 503` 1; the cannot-reach line 1 |

- [ ] **Step 1:** Write the tests. Run `go test ./internal/cli/ -run 'Root|Admin|Serve|Bootstrap|Probe' -v` and the tagged run. Expected: red.
- [ ] **Step 2:** Implement. Run both, and the whole `./internal/cli/` untagged suite. Expected: pass.
- [ ] **Step 3:** `make lint`, `make build`, and by hand: `KCR_BOOTSTRAP_MODE=1 KCR_DATABASE_DSN=... printf 'x\n' | bin/k8s-capacity-report admin bootstrap-admin --username admin --password-stdin --config .cache/phase-3/part-1/platform.toml` and read the output. Commit.

```bash
git add internal/cli
git commit -m "feat(cli): add the serve and admin commands"
```

**Done when:** the seventeen tests pass, the end-to-end test proves the part, and the program run by hand bootstraps and serves.

---

### Task 14: Documents, changelog, and the close

**Executor:** Main thread.

- [ ] **Step 1:** `AGENTS.md`: the layer table gains the six platform ranks and `cli` at 200; the build tags table gains `postgres` (needs PostgreSQL, runs in `make test-postgres`); the commands section says `serve` and `admin` exist with the rules of R91 to R93; the rule that the error sentences of the API are diagnostics (R88); the delegation section says a subagent runs the `postgres` tests against the DSN the main thread gives it and never starts Docker; the commands list gains `make generate`, `make generate-check`, `make postgres-up`, `make postgres-down`, `make test-postgres`.
- [ ] **Step 2:** `README.md`: a section on `serve`, the deployment file, `KCR_DATABASE_DSN`, `admin migrate`, `admin bootstrap-admin`, and `admin probe`, with one example each.
- [ ] **Step 3:** `CHANGELOG.md`, under `## [Unreleased]`, `### Added`: one sentence per perceivable thing: the `serve` command and its port; sign-in with local accounts and the three roles; the session rules; the throttle; the account route; the administration of users; `admin migrate`; `admin bootstrap-admin`; `admin probe`; the health routes; the deployment file and `KCR_DATABASE_DSN`. Left out, as internal: the test harness, the guards, the generated types.
- [ ] **Step 4:** Dispatch the adversarial review of the whole branch (`.cache/phase-3/part-1/review/`), with the diff against `main`, the plan, and the research file. Fix what it finds that is real; record the rest in `ROADMAP.md`.
- [ ] **Step 5:** `open -a Docker`; `make gate` (which now runs `generate-check`, `lint`, `test`, `test-postgres`, the images, the template tests, the goldens, `check-runtime`); logs under `.cache/gate/`, copied to `.cache/phase-3/part-1/gate/`.
- [ ] **Step 6:** Update `ROADMAP.md`: every item of 3.1 `DONE` with its commit; the rulings E-numbered of the execution; the findings of the review. Read `ROADMAP.md` and `CHANGELOG.md` back.
- [ ] **Step 7:** Hand over to the owner: the gate log, the rulings made during execution, the findings, and the request for the word to merge `feat/platform-accounts` into `main` by fast-forward, with no new version and no tag (the platform is not usable by a reader until phase 4).

**Done when:** the owner has the closing report.
