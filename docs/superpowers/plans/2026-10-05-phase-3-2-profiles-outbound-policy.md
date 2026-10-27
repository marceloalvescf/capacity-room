# Phase 3.2: Profiles, Credential Files, and the Outbound Policy Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** An administrator creates, revises, deactivates, and tests a metric-source profile through the API; every revision is immutable and the current one is pinned by a pointer; a profile names a credential file and a CA bundle file by name only, resolved from folders the deployment mounts, never at write time; every request to a metric source passes the outbound policy of spec 11.5, ported in full, through the transport the Prometheus client already accepts; `admin import-profiles` loads profiles from a TOML file and is repeatable; the two residual races of part 3.1 (F-112, F-113) close.

**Architecture:** Three packages join the platform: `internal/platform/netpolicy` (the static URL rule, the address classes, the allowlist, the resolution, and the transport that dials the literal address, compares the peer, and refuses a redirect), `internal/platform/profiles` (the document and its rules, the canonical endpoint, the fingerprint, the reference files, the revisions service, the profile test), and the store and api parts they need. The preflight of the `report` command moves into `internal/discovery`, so the profile test and the worker of part 3.3 run the same three steps the command runs. The deployment file gains the allowlists and the two folders.

**Tech Stack:** Go 1.27 and the modules already required. No new module. PostgreSQL 18 through the harness of part 3.1. The fake Prometheus of `internal/promtest` on the loopback, admitted by an allowlist entry of exactly one address, proves the policy end to end.

**Spec:** `docs/superpowers/specs/2026-09-27-k8s-capacity-report-design.md`, sections 2, 3, 4, 5.2 (the fields a profile carries), 6.1 (the candidate roots), 11.2, 11.5, 11.6, 11.9, 11.10 (the profile rows), 13.2, 15. The inventory of the original is `docs/reference/inventory/platform.md`; the facts of this part, quoted from the original with file and line, are in `.cache/phase-3/part-2/research/original-profiles.md`. The decisions of the owner are in `ROADMAP.md`.

**Depends on:** Part 3.1, merged into `main` at `41e720b`.

## Where this part sits

| Part | Delivers | Proof at the end |
|---|---|---|
| 3.1, merged | Settings, store, audit, accounts, sessions, the API skeleton, `serve`, `admin` | An administrator bootstrapped from the command line signs in and creates an operator |
| **3.2 Profiles, credential files, outbound policy (this plan)** | The profile tables and revisions, the document rules, the reference files, the outbound policy and its transport, the profile routes and the profile test, `admin import-profiles`, the preflight shared by command and platform; F-112, F-113 | A profile created through the API is tested against the fake Prometheus on the loopback, admitted by the policy; the same test refused when the policy does not admit the address |
| 3.3 River, worker, storage, retention, audit of reports | The report rows and the queue, the worker that runs a pinned revision through the preflight and the collectors, the stored snapshot and PDFs, expiry; F-41, F-89, F-93, F-102, F-109 | A report requested through the API and its PDF compiled by the worker |
| 3.4 API, report view, OpenAPI contract | The report routes, the report view, pagination, idempotency, admission | A report requested and downloaded through the API, as spec 16 asks |

### Out of this part

| Left out | Arrives in | Reason |
|---|---|---|
| The jobs that pin a revision, the selection of a report (namespaces, period, time zone, report profile, minimum severity judged against the profile's permitted lists), `PROFILE_INACTIVE` and `PROFILE_SUPERSEDED` | 3.3 and 3.4 | They belong to the request of a report; the policy of permitted and default values is stored and validated now |
| The worker's resolution of the references at execution | 3.3 | The functions arrive now (`profiles.ResolveCredential`, `ResolveCABundle`, `Document.RunConfig`); the worker calls them |
| An approved outbound proxy | none | Spec 11.5 lists six rules and names no proxy; the original's `approved_proxy` is not ported (R114) |
| The pasted credential (`managed_secret`) and the keyring | none | Spec 3: a profile only names a credential file |
| File profiles shadowing database profiles | none | Spec 3: database profiles only, plus `admin import-profiles` |

## How to read this plan

The owner's orchestration rule applies: the plan states what each task must deliver and how it is proven, and the implementer writes the code. Each task gives exact paths, exact signatures, and a table of tests with inputs and expected results. Write each test first, watch it fail, then implement.

Executors:

- **Subagent**: a Sonnet subagent. It may run `go`, `make build`, `make test`, `make lint`, `make generate`, `make generate-check`, and the tagged tests against the PostgreSQL the main thread started (`KCR_TEST_DATABASE_DSN`). It never starts or stops Docker, never runs `make update-*`, and cannot declare a task finished.
- **Main thread**: needs Docker, judgment, API design, wording, or the owner.

Rules for every subagent brief: as in the plan of part 3.1 (tasks in sequence; a reviewer reads a diff file and runs tests only on packages nobody edits; `git add <paths>`; no stash, restore, checkout of a path, or clean; a test of a change goes red first; a brief that says "report every X" lists what must not match; a decision flagged at the end of a report has already been made; the main thread commits before dispatching and computes before dispatching).

Reference material, read-only and never tracked:

- `older/kubernetes-report/kube_capacity_report/platform/`: `network.py` (the policy, the reference for every rule of Tasks 4 and 5), `profiles.py`, `settings.py` (the document), `crypto.py:402-583` (the reference files), `api/profiles.py`.
- `.cache/phase-3/part-2/research/original-profiles.md`: the facts of this part with file and line. A brief quotes the lines it needs; an implementer that finds the brief and the original in disagreement stops and reports.
- `older/kubernetes-report/tests/platform/test_network_policy.py`, `test_profiles.py`, `test_profile_preflight.py`, `test_crypto.py`: the tests of the original, the source of every input in the tables of Tasks 4, 5, 7, and 8.

## Global Constraints

Every constraint of the plans of phases 0 to 3.1 stands. The ones below are added or are the ones this part leans on most.

- No new module. The platform packages rank between `internal/document` (90) and `internal/cli` (200). The core never imports `internal/platform/`; `internal/discovery` gains `Preflight` and imports nothing new (R109).
- Metrics only. A profile names files; it never holds a credential value, a path, or a resolved address. The snapshot of a later part holds none of them either (spec 8.3).
- **The outbound policy of spec 11.5 is ported in full**, every rule of the six, with the address classes of the original quoted in Task 4. Every request of the profile test, and of the worker of 3.3, goes through `netpolicy.Transport`; the built transport of `internal/prom` is never used by the platform.
- A reference name matches `^[a-z0-9](?:[a-z0-9._-]{0,61}[a-z0-9])?$` and holds no `..`; a file is read only under its folder after symbolic links are followed, as a regular file of at most 1 MiB; a resolution error carries a code and never a path, a name, or content.
- An error of the policy carries a code and never the URL, the host name, or an address. The profile test answers 200 with `{ok, code, capabilities}` and never a URL, a name, an address, or exception text.
- Every revision is immutable (a trigger refuses `UPDATE` and `DELETE`); the pointer of a profile names a revision of that profile (a deferred constraint trigger); a revision number is computed inside the insert; a profile is deleted by clearing its pointer, never by deleting rows.
- The checks of a route run in the order session, role, CSRF, body. `fields` of a `PROFILE_DOCUMENT_INVALID` answer map a field of the document to one of `missing`, `invalid`, `repeated`, `empty`, `not_permitted`, `incoherent`, never a value (R111).
- Every timestamp is passed from Go. No test skips. Tagged tests carry `//go:build postgres`. Comments never name a task, plan, or phase. Diagnostics in English. Every literal ASCII.
- Scratch under `.cache/phase-3/part-2/`. Commits `type(scope): description`, imperative, under 50 characters.

## Review Focus

Each line names an input the spec implies and does not describe, and the task whose tests pin it.

1. **An endpoint whose root passes the static policy while its second candidate fails.** `https://metrics.internal/api/v1/../x` is refused at the path; `https://metrics.internal` with a candidate `https://metrics.internal/api/v1` is accepted only when both candidates and their `/query` and `/query_range` pass. Pinned in Task 7 (`TestCanonicalEndpointValidatesEveryCandidate`).
2. **A resolution that mixes an allowed address with a loopback one, and a peer that differs from the address dialled.** Both refuse, whatever the order of the answers. Pinned in Task 4 (`TestResolveMixedAnswersFailClosed`, both orders) and Task 5 (`TestTransportRefusesARebindingPeer`).
3. **A reference name that is a symbolic link escaping the folder, a directory, or the folder itself.** Each is `CREDENTIAL_FILE_INVALID`; a link that stays inside is followed. Pinned in Task 7 (`TestResolveReferenceConfinement`).
4. **Two revisions of one profile sent at the same time with the same `base_revision_id`.** One wins and the other answers `PROFILE_REVISION_STALE`; the revision numbers never collide. Pinned in Task 8 (`TestReviseConcurrentlyOneWins`).
5. **A profile whose permitted periods leave out the fallback default while `default_period` is null.** Refused at write time with `{"default_period":"not_permitted"}`, since a request of a report would otherwise fail closed every time. Pinned in Task 7 (`TestDocumentDefaultsWithinPermitted`).

## Rulings made on the owner's behalf

Each is a decision the spec leaves open, written as decision, reason, and cost if wrong. The owner may overturn any of them when reviewing this plan. The numbering continues that of part 3.1.

| ID | Ruling |
|---|---|
| R99 | Ruling: the schema of this part is `profiles` (`id text` primary key, `current_revision_id uuid` nullable with a deferred foreign key to `profile_revisions`, `created_at`, `updated_at`) and `profile_revisions` (`id uuid`, `profile_id`, `revision integer`, `fingerprint text`, `display_name`, `active boolean`, `document jsonb`, `credential_file text`, `ca_bundle_file text`, `created_by uuid` nullable, `created_at`), with `UNIQUE (profile_id, revision)`, `UNIQUE (profile_id, fingerprint)`, the immutability trigger, and the deferred constraint trigger that the pointer names a revision of the same profile; there is no `source` column, no file pointer, and no `managed_secrets` table — spec 3 keeps database profiles only and no pasted credential; the original's two pointers existed for file profiles (`migration.py:144-219`) — none |
| R100 | Ruling: a profile document carries the fields of spec 11.6 under the names of the configuration where one exists (`endpoint_url`, `endpoint_type`, `timeout_seconds`, `tls_verify`, `credential_file`, `ca_bundle_file`, `cluster_name`, `cluster_label`, `filter_mode`, `nodepool_label`, `node_autoscaling`, `allowed_namespaces`, `excluded_containers`, `permitted_periods`, `default_period`, `permitted_timezones`, `default_timezone`, `permitted_report_profiles`, `default_report_profile`, `permitted_min_severities`, `default_min_severity`, `thresholds`, plus `display_name` and `active`); there is no `credential_mode`: an empty `credential_file` means no credential — the original's three modes collapse to two once the pasted credential goes — none |
| R101 | Ruling: a null default of a selection dimension means the default of spec 5.2 (`24h`, `America/Sao_Paulo`, `detailed`, `all`), and a default, explicit or null, must be within the permitted list when that list is not empty, checked at write time with `not_permitted`; an empty permitted list means unrestricted — the original checked an inherited default only at job submission, which fails closed every time for such a profile (`profiles.py:179-238`); spec 3 removes the platform settings those defaults came from — a profile that forbids the spec default must name its own, which is one line of the body |
| R102 | Ruling: the endpoint stored is the canonical form of the policy (scheme and host lower-case, the host in IDNA, no trailing dot, the default port dropped, the path canonical with no trailing slash, no query, no fragment), and the static policy is applied, at create, at revise when the endpoint or its type changes, and at import, to every candidate root of `prom.Candidates` and to the `/query` and `/query_range` of each, before anything is written — the original (`profiles.py:1285-1297`); a candidate that fails must refuse the profile — a stored URL differs in letter case from what the administrator typed |
| R103 | Ruling: `netpolicy` holds the policy as a value built from the two allowlists (`Policy{Hostnames []string; Networks []netip.Prefix}`), with `ValidateURL`, `Resolve` (taking a resolver function, the system one by default), `AddressAllowed`, and `Transport`, whose round trip validates the URL again, resolves again, dials the first address as a literal with the host name only in SNI and the `Host` header, compares the peer address after connecting, refuses any 3xx with `ENDPOINT_REDIRECT_REJECTED`, uses no proxy, and reuses no connection across requests (one connection per request, closed after the body) — spec 11.5 items 1 to 6 and the original's transport (`network.py:480-531`); keep-alive across requests would reuse a connection resolved earlier, which rule 4 forbids — each query pays a handshake, which the original paid too |
| R104 | Ruling: the address classes refused are the original's twelve networks and two addresses, quoted in Task 4, applied after an IPv4-mapped IPv6 address is unmapped and a zone is dropped; an allowlist network of exactly one address lifts the refusal for that address alone; with both allowlists empty every address is refused — `network.py:74-91, 374-386`; spec 11.5 item 2 — none |
| R105 | Ruling: a credential file is read as UTF-8 with one trailing `\r\n`, else one `\n`, removed, and must satisfy `config.CheckToken` (at least 8 characters) or it is `CREDENTIAL_FILE_INVALID`; a CA bundle must hold at least one PEM certificate; a resolution happens only in the profile test and, in 3.3, at job execution, never at create, revise, or import — `crypto.py:402-583`; the token rule of part 2.1 (R38) holds for every token the program sends — a credential shorter than 8 characters, which the redactor cannot mask, is refused as it is on the command line |
| R106 | Ruling: a revise names `base_revision_id`; when it is not the current revision the answer is 409 `PROFILE_REVISION_STALE`; the identity row is read `FOR UPDATE`; a revise whose merged document equals the current one writes nothing and answers 200 with the current revision; the fingerprint is the SHA-256 of the canonical JSON of the document chained with the parent revision id, so a change back to an earlier document is a new revision; a `DELETE` clears the pointer and keeps every revision, and the id may be created again, continuing the revision count — the original (`profiles.py:976-1079`) except the no-op, which the users service of 3.1 already treats as nothing — none |
| R107 | Ruling: the routes are those of spec 11.10: `GET /profiles` answers the active profiles as summaries (`id`, `display_name`, `allowed_namespaces`, the four permitted lists and the four defaults resolved to the fallback), in id order; `GET /admin/profiles` and `GET /admin/profiles/{id}` answer the full document plus `revision_id`, `revision`, `created_at`, never a resolved path; `POST` answers 201; `PATCH` takes `base_revision_id` and the changed fields, a field set to `null` clearing it; `DELETE` answers 204; `POST /admin/profiles/{id}/test` answers 200 with `{"ok","code","capabilities"}` whatever the outcome, and runs for an inactive profile too — spec 11.10 and the original (`api/profiles.py`) — none |
| R108 | Ruling: the codes of the profile test are `PROFILE_OK`, the six of the policy, `PROFILE_CREDENTIAL_UNAVAILABLE`, `PROFILE_CA_BUNDLE_UNAVAILABLE`, `ENDPOINT_TLS_FAILED`, `ENDPOINT_AUTH_REJECTED`, `ENDPOINT_API_NOT_FOUND`, `ENDPOINT_UNREACHABLE`, `CLUSTER_SCOPE_NOT_FOUND`, `CAPABILITIES_INSUFFICIENT`, mapped from `*netpolicy.Error`, the resolution errors, `prom.Error` by kind (`KindTLS`, `KindAuthentication` and `KindAuthorization`, `KindEndpointNotFound`, every other kind), `*discovery.ScopeError`, and `discovery.ErrInsufficient`, in that order; the test is bounded by a context deadline of three times `timeout_seconds` and holds no pool connection while it runs; it writes the audit row `profile.test` with the code and the revision id, outcome `failed` when `ok` is false — the original (`profiles.py:1182-1205`) — none |
| R109 | Ruling: `discovery.RunPreflight(ctx, opts prom.Options, cluster config.Cluster, analysis config.Analysis, at time.Time) (Preflight, error)` connects, resolves the scope, detects the capabilities, and checks they are sufficient, returning the client, the scope, and the capabilities; `internal/cli` calls it in place of its own `preflight`; the four milestone lines stay in the command — spec 4.3 names the same steps for the command and the worker; one function keeps them equal — none |
| R110 | Ruling: `admin import-profiles --file <path> [--config platform.toml]` reads a TOML file with an array of tables `[[profiles]]`, each the document of a profile with its `id`, refuses an unknown key, an inline credential key (`token`, `token_env`, `credential`, `credential_value`, `password`, `api_key`, `ca_bundle`, `ca_bundle_path`), and a repeated id, validates every document and every endpoint before any write, then in one transaction creates each absent profile and revises each one whose document differs from its current revision, with `created_by` null; profiles in the database that the file does not name are left alone; it prints one line per profile, `created <id>`, `revised <id>`, or `unchanged <id>`, and exits 0; it writes `profile.create` or `profile.revise` with no actor — spec 11.6 says the command "creates or revises profiles from a TOML file and is repeatable"; the original's startup sync (`profiles.py:1083-1124`) released file pointers, which do not exist here — an operator who removes a profile from the file deactivates it in the interface |
| R111 | Ruling: a document refused at create, revise, or import answers 422 `PROFILE_DOCUMENT_INVALID` with `fields` mapping each field to one of `missing`, `invalid`, `repeated`, `empty`, `not_permitted`, `incoherent`; the policy refusals of the endpoint answer 422 `ENDPOINT_URL_REJECTED` or `ENDPOINT_ADDRESS_REJECTED` with `fields` `{"endpoint_url": "invalid"}`; `admin import-profiles` prints the same facts as `error: profile <id>: <field> <word>` — spec 11.10 wants one error shape and a fixed sentence; the original joined pydantic sentences into one field — an administrator reads a word per field, as for every other body |
| R112 | Ruling: the audit actions of this part are `profile.create` (`{"revision_id","revision","fields"}` with the sorted field names), `profile.revise` (`{"revision_id","revision","base_revision_id","fields"}` with the changed names), `profile.activate` and `profile.deactivate` (the same, when `active` is among the changed fields), `profile.delete` (`{"revision_id"}`), `profile.test` (`{"code","revision_id"}`) — the original (`profiles.py:166-171`) — none |
| R113 | Ruling: the deployment file gains `credential_dir` and `ca_bundle_dir` (absolute, no `..`, defaults `/run/secrets/metrics` and `/run/config/ca`, not read at load), `allowed_endpoint_hostnames` (each lower-cased, one trailing dot removed, a host name with no wildcard, unique) and `allowed_endpoint_cidrs` (strict CIDR, unique, and none that covers a metadata address with more than that one address) — spec 11.2 — none |
| R114 | Ruling: no approved proxy; a deployment behind a proxy is out of scope of this project, since spec 11.5 names none and rule 6 ignores the variables; recorded as F-114 for the owner — the original had one — a deployment that needs an egress proxy cannot use the platform |
| R115 | Ruling: F-112 closes by reading the user `FOR SHARE` in the session transaction of a sign-in (`store.GetUserForShare`); F-113 closes by `Users.Update` writing role, active, and language through `store.UpdateUserFields` and the hash through `UpdatePasswordHash`, so `UpdateUser` goes — the residuals of the adversarial review of 3.1 — none |
| R116 | Ruling: `config` exports `ValidateThresholds`, `ValidatePeriod`, and `ValidateTimezone`, thin wrappers of the existing functions with the existing messages, for the profile document — one place for each rule — none |
| R117 | Ruling: the platform's profile test and the worker of 3.3 pass the profile's `timeout_seconds` to the Prometheus client as `Metrics.TimeoutSeconds`, so one attempt is bounded as on the command line; the retry rule of the client (three attempts) stands — the original (`profiles.py:584`, attempts 3) — none |
| R118 | Ruling: a host name that is not ASCII is refused by the policy; an administrator writes an internationalised name in its punycode form — the module requires no IDNA library and spec 11.5 names none — a profile with a non-ASCII host name is typed as punycode |

## Findings this plan closes or records

| Finding | How |
|---|---|
| F-112 | Closed by Task 2 (R115) |
| F-113 | Closed by Task 2 (R115) |
| F-114 (new) | No approved proxy (R114); recorded for the owner |

## File structure

| Path | Responsibility in this part |
|---|---|
| `internal/guard/imports.go`, `imports_test.go` | The ranks of `netpolicy` (105) and `profiles` (145) |
| `internal/config/validate.go` | `ValidateThresholds`, `ValidatePeriod`, `ValidateTimezone` |
| `internal/discovery/preflight.go`, `preflight_test.go` | `Preflight` |
| `internal/cli/run.go` | Calls `discovery.Preflight` |
| `internal/platform/store/users.go`, `internal/platform/auth/session.go`, `users.go` | F-112, F-113 |
| `internal/platform/settings/settings.go` | The four new keys |
| `internal/platform/netpolicy/url.go`, `classes.go`, `resolve.go`, `transport.go`, `errors.go` and tests | The policy |
| `internal/platform/store/migrations/00002_profiles.sql`, `store/profiles.go` and tests | The schema and the queries |
| `internal/platform/profiles/document.go`, `endpoint.go`, `fingerprint.go`, `refs.go`, `service.go`, `test.go`, `importer.go` and tests | The document, the references, the service, the test, the import |
| `internal/platform/api/profiles.go`, `profiles_test.go`, `errors.go`, `openapi.gen.go`; `api/openapi.yaml` | The routes and the contract |
| `internal/cli/admin.go`, `admin_test.go` | `admin import-profiles` |
| `AGENTS.md`, `README.md`, `CHANGELOG.md` | The documents |

---

### Task 0: Open the part

**Executor:** Main thread.

- [ ] **Step 1:** `git status`, `git log --oneline -3`, `go test -count=1 ./...`: `main` at `41e720b`, clean, green.
- [ ] **Step 2:** `git switch -c feat/platform-profiles` and confirm.
- [ ] **Step 3:** `ROADMAP.md`: the summary line of 3.2 names this plan and the branch; a section "Phase 3.2" with P32-0 to P32-12, all `OPEN`; the rulings R99 to R118; F-114 in the findings log.
- [ ] **Step 4:** Commit the plan: `docs(plan): add the plan of part 3.2`.

**Done when:** the branch exists, the plan is committed, `ROADMAP.md` holds the part.

---

### Task 1: The ranks, the exported validators, and the shared preflight (R109, R116)

**Executor:** Subagent.

**Files:**
- Modify: `internal/guard/imports.go` (`Layers`: `internal/platform/netpolicy` 105, `internal/platform/profiles` 145), `internal/guard/imports_test.go` (`TestLayersHoldEveryPackageOfThisPart` gains the two; `TestLayersHoldThePlatformPackages` gains the two; the count becomes 26)
- Create: `internal/platform/netpolicy/doc.go`, `internal/platform/profiles/doc.go` (a package clause and one sentence each)
- Modify: `internal/config/validate.go` (export three wrappers), `internal/config/validate_test.go`
- Create: `internal/discovery/preflight.go`, `internal/discovery/preflight_test.go`
- Modify: `internal/cli/run.go` (`preflight` and `collected` go; the three callers use `discovery.RunPreflight`), `internal/cli/run_test.go` if it names `preflight`

**Interfaces:**

```go
// config
func ValidateThresholds(th Thresholds) error // validateThresholds, same messages
func ValidatePeriod(period string) error      // errPeriodShape or errPeriodTooLong, as validateAnalysis judges a.Period
func ValidateTimezone(zone string) error      // errTimezoneUnknown, as validateAnalysis judges a.Timezone

// discovery
// Preflight is what every run shares before it collects: the client bound
// to the root that answered, the scope, and the capabilities.
type Preflight struct {
	Client       *prom.Client
	Scope        Scope
	Capabilities []Capability
}

// RunPreflight connects with opts, resolves the scope of cluster at `at`,
// detects the capabilities for analysis at `at`, and checks they are
// sufficient. It reads the clock nowhere.
func RunPreflight(ctx context.Context, opts prom.Options, cluster config.Cluster, analysis config.Analysis, at time.Time) (Preflight, error)
```

`internal/cli` builds `prom.Options{Metrics: cfg.Metrics, Token: token}` and calls `discovery.RunPreflight`; the output lines and exit codes of the command do not change: every test of `internal/cli` passes unchanged, which is the proof.

**Tests:**

| Test | Input | Expected |
|---|---|---|
| `TestLayersHoldEveryPackageOfThisPart`, `TestLayersHoldThePlatformPackages` | the two ranks | pass, 26 entries |
| `TestValidateThresholdsWrapsTheRule` | the defaults; warning 90 critical 85; NaN | nil; `thresholds.warning_percent must be lower than critical_percent`; `threshold values must be finite numbers` |
| `TestValidatePeriod` | `24h`; `x`; `99999999999999999999h` | nil; `analysis.period must look like 30m, 4h, 24h, or 7d`; `analysis.period is too long` |
| `TestValidateTimezone` | `UTC`; `Mars/Olympus` | nil; `analysis.timezone is unknown` |
| `TestRunPreflightAgainstTheFakeServer` (discovery, through `RunPreflight`) | `promtest` server of `direct-complete`, the configuration of that scenario (as `internal/cli`'s tests build it) | a client bound to the root, a resolved scope, nine capabilities, no error |
| `TestRunPreflightStopsAtTheFirstFailure` | a server that answers 401 | the error of `prom` with `KindAuthentication`; no later query sent (the server's `Seen` holds the probe only) |
| `TestRunPreflightInsufficient` | a scenario with no capability (the fake server answering an empty vector to every probe) | `discovery.ErrInsufficient` |
| the suite of `internal/cli` | unchanged | passes |

- [ ] Steps: tests red, implement, `make lint`, `make test`, commit `refactor(discovery): share the preflight with the platform`.

**Done when:** the suites pass and `internal/cli/run.go` holds no `preflight` of its own.

---

### Task 2: The two residual races of part 3.1 (R115)

**Executor:** Subagent, against the running server.

**Files:**
- Modify: `internal/platform/store/users.go` (+ `GetUserForShare`, `UpdateUserFields`; `UpdateUser` removed), `users_test.go`
- Modify: `internal/platform/auth/session.go` (the session transaction reads with `GetUserForShare`), `users.go` (`Update` writes with `UpdateUserFields` and `UpdatePasswordHash`), their tests

**Interfaces:**

```go
// GetUserForShare reads the user and holds a share lock on the row for the
// transaction, so a concurrent change of the row waits until it commits.
func GetUserForShare(ctx context.Context, tx pgx.Tx, id uuid.UUID) (User, error)

// UpdateUserFields writes role, language, active, and updated_at of the
// row with id, and never the hash or the username.
func UpdateUserFields(ctx context.Context, q Querier, id uuid.UUID, role, language string, active bool, now time.Time) error
```

**Tests:**

| Test | Input | Expected |
|---|---|---|
| `TestGetUserForShareBlocksAnUpdate` (store) | tx1 reads for share; tx2 runs `UpdatePasswordHash` on the row | tx2 blocks until tx1 commits (`expectBlocksUntilCommit`) |
| `TestUpdateUserFieldsNeverWritesTheHash` (store) | a row; `UpdateUserFields` | role, language, active, updated_at change; the hash does not |
| `TestLoginSessionWaitsForAPasswordChange` (auth) | through the `beforeSession` hook, start an `Account.Change` of the password in a goroutine while the session transaction holds the share lock | the sign-in completes first and its session is revoked by the change, or the change commits first and the sign-in refuses; in both cases no live session remains for the old password (assert `FindActiveSessionByTokenHash` is `ErrNotFound` after both) |
| `TestUpdateKeepsAConcurrentPasswordChange` (auth) | `Users.Update` setting the role, concurrent with `Account.Change` of the password | after both: the role is the new one and the hash is the new one |
| every existing test of the two packages | unchanged | passes |

- [ ] Steps: tests red, implement, `make lint`, `make test`, tagged suite twice, commit `fix(auth): close the two residual races`.

**Done when:** the tests pass twice and `UpdateUser` no longer exists.

---

### Task 3: The settings of the policy and the folders (R113)

**Executor:** Subagent.

**Files:**
- Modify: `internal/platform/settings/settings.go`, `settings_test.go`

**Interfaces:**

```go
type Settings struct {
	// ... the fields of 3.1 ...
	CredentialDir            string         // credential_dir, default /run/secrets/metrics
	CABundleDir              string         // ca_bundle_dir, default /run/config/ca
	AllowedEndpointHostnames []string       // normalised, unique
	AllowedEndpointCIDRs     []netip.Prefix // unique
}
```

Messages, each once: `credential_dir must be absolute`, `credential_dir must not hold a .. segment`, the same two for `ca_bundle_dir`, `allowed_endpoint_hostnames entry <n> is not a host name`, `allowed_endpoint_hostnames repeats entry <n>`, `allowed_endpoint_cidrs entry <n> is not a CIDR block`, `allowed_endpoint_cidrs entry <n> covers a metadata address; list that single address instead`, `allowed_endpoint_cidrs repeats entry <n>` (the entry number is repeated, never the value). A host name entry is trimmed, one trailing dot removed, lower-cased, and must match `^[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?(?:\.[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?)*$`. A CIDR is parsed with `netip.ParsePrefix` and must equal its `Masked()` form (no host bits). The metadata addresses are `169.254.169.254` and `fd00:ec2::254`.

**Tests:**

| Test | Input | Expected |
|---|---|---|
| `TestLoadFolderDefaults` | no folder keys | the two defaults |
| `TestLoadFolderRules` | `credential_dir = "secrets"`; `"/run/../x"` | the two messages |
| `TestLoadHostnamesNormalised` | `["Prometheus.Example.Invalid.", " a.b "]` | `prometheus.example.invalid`, `a.b` |
| `TestLoadHostnamesRefused` | `["*.example"]`; `["metrics_internal"]`; `["a", "A"]` | not a host name (twice); repeats entry 2 |
| `TestLoadCIDRsRefused` | `["10.0.0.1"]`; `["10.0.0.1/8"]`; `["169.254.0.0/16"]`; `["fd00:ec2::/64"]`; `["10.0.0.0/8", "10.0.0.0/8"]` | not a CIDR; not a CIDR (host bits); covers (twice); repeats entry 2 |
| `TestLoadCIDRsAccepted` | `["169.254.169.254/32", "10.0.0.0/8", "fd00:ec2::254/128"]` | the three prefixes |

- [ ] Steps: tests red, implement, `make lint`, `make test`, commit `feat(settings): add the outbound allowlists`.

---

### Task 4: The policy: the URL rule, the classes, the allowlist, the resolution (R103, R104)

**Executor:** Subagent.

**Files:**
- Create: `internal/platform/netpolicy/errors.go`, `url.go`, `classes.go`, `resolve.go`, and `url_test.go`, `classes_test.go`, `resolve_test.go`

**Interfaces:**

```go
// Code is a refusal of the policy. It is the whole of what an Error says.
type Code string

const (
	CodeURLRejected        Code = "ENDPOINT_URL_REJECTED"
	CodeAddressRejected    Code = "ENDPOINT_ADDRESS_REJECTED"
	CodeResolutionFailed   Code = "ENDPOINT_RESOLUTION_FAILED"
	CodeConnectionRejected Code = "ENDPOINT_CONNECTION_REJECTED"
	CodeRedirectRejected   Code = "ENDPOINT_REDIRECT_REJECTED"
)

// Error carries a code and nothing else: no URL, host, or address.
type Error struct{ Code Code }

func (e *Error) Error() string // the code as text

// Origin is a validated endpoint.
type Origin struct {
	Scheme   string // http or https
	Host     string // the canonical host: lower-case, IDNA, no trailing dot; an IP literal without zone
	Port     int    // the explicit port or the scheme's default
	Path     string // the canonical path, "/" when empty
	Literal  bool   // Host is an IP literal
}

// URL is the canonical text: scheme, authority (the port only when not the
// default, the host bracketed when IPv6), and the path ("" for "/").
func (o Origin) URL() string
// Authority is host, or host:port when the port is not the default, the
// host bracketed when IPv6.
func (o Origin) Authority() string

// MaxURLLength bounds a URL.
const MaxURLLength = 2048


// Blocked reports whether addr falls in a refused class, after unmapping an
// IPv4-mapped address. The classes are the original's.
func Blocked(addr netip.Addr) bool

// Policy is the allowlist of a deployment.
type Policy struct {
	Hostnames []string
	Networks  []netip.Prefix
}

// ClassPermitted reports whether addr is outside every refused class, or
// inside a network of the allowlist that holds exactly one address.
func (p Policy) ClassPermitted(addr netip.Addr) bool

// AddressAllowed reports whether addr may be connected to for host: its
// class is permitted and either host is in Hostnames or addr is inside a
// network of the allowlist.
func (p Policy) AddressAllowed(addr netip.Addr, host string) bool

// ValidateURL applies the static rules in the order of the original; a
// literal host must be ClassPermitted.
func (p Policy) ValidateURL(raw string) (Origin, error)

// Resolver answers the addresses of a host, in order.
type Resolver func(ctx context.Context, host string) ([]netip.Addr, error)

// SystemResolver uses net.DefaultResolver.
func SystemResolver(ctx context.Context, host string) ([]netip.Addr, error)

// Resolve returns the addresses of o that the policy allows, canonical and
// without repeats, in the resolver's order: the literal alone, or every
// answer of the resolver. One refused answer refuses all of them.
func (p Policy) Resolve(ctx context.Context, o Origin, resolve Resolver) ([]netip.Addr, error)
```

`ValidateURL`, in order: empty or over 2048 characters; any rune below `0x21` or equal to `0x7f`, or a backslash; `?` or `#` anywhere; `url.Parse` fails; scheme not `http`/`https`; user info present; no host; the host lower-cased; a host that is not ASCII is refused (R118: the module offers no IDNA, and the administrator writes the punycode; `mé.internal` refused, `xn--m-bga.internal` accepted); one trailing dot removed; a non-literal host must match the host name pattern of Task 3; a port `0` or over 65535 refused; the path canonical: each segment must not be empty, `.`, or `..`, raw or after percent-decoding, must hold only `A-Za-z0-9-._~!$&'()*+,;=:@%`, and `%` must be followed by two hex digits; a trailing slash dropped; an empty path is `/`. A literal host is judged by `Policy.ClassPermitted`: `ValidateURL` is a method of `Policy` (`func (p Policy) ValidateURL(raw string) (Origin, error)`), so a blocked literal is `CodeAddressRejected` unless a one-address entry of the policy lifts it, as the original's `validate_url` judges a literal with `_class_permitted` (`network.py:365-369`). The address classes:

```go
var blockedNetworks = []string{
	"0.0.0.0/8", "127.0.0.0/8", "169.254.0.0/16", "224.0.0.0/4", "240.0.0.0/4",
	"::/128", "::1/128", "fe80::/10", "ff00::/8", "64:ff9b::/96", "2002::/16", "2001::/32",
}
var blockedAddresses = []string{"169.254.169.254", "fd00:ec2::254"}
```

**Tests** (every input of the original's `test_network_policy.py` named in the research, sections 9 and 5; every expected value a literal):

| Test | Input | Expected |
|---|---|---|
| `TestValidateURLRefusesUnsafe` | with an empty policy: `ftp://metrics.internal/api/v1`; `https://u:p@metrics.internal/api/v1`; `https://metrics.internal/api/v1?q=x`; `https://metrics.internal/api/v1#x`; `//metrics.internal/api/v1`; `https://[::1]/api/v1` | the first five `ENDPOINT_URL_REJECTED`, the last `ENDPOINT_ADDRESS_REJECTED` |
| `TestValidateURLRefusesMalformed` | `https://h/?`; `https://h/#`; `https://@h/`; `https://h:0/`; `https://h:70000/`; `https://h//x`; `https://h/./x`; `https://h/../x`; `https://h/%2x`; a tab in the path; a backslash; a path of 2049 characters; `https://metrics_internal/` | `ENDPOINT_URL_REJECTED` each |
| `TestValidateURLRefusesEncodedDotsAndSlashes` | `%2e%2e`, `%2F`, `%2E`, `%5C` as segments | refused |
| `TestValidateURLKeepsEncodedSegments` | `/a;x=1`, `/a%20b` | accepted, kept as written |
| `TestValidateURLCanonical` | `HTTPS://Metrics.Internal./api/v1/` | `https://metrics.internal/api/v1`, port 443, host `metrics.internal` |
| `TestValidateURLPorts` | `https://h:8443/x`; `https://h:443/x`; `http://h:80/x` | authority `h:8443`; `h`; `h` |
| `TestValidateURLIPv6Literal` | `https://[2001:db8::1]:8443/x` | host `2001:db8::1`, literal, authority `[2001:db8::1]:8443` |
| `TestValidateURLLiteralLiftedByThePolicy` | `https://127.0.0.1:9090/` with networks `[127.0.0.1/32]`; with an empty policy | accepted; `ENDPOINT_ADDRESS_REJECTED` |
| `TestValidateURLIDNA` | `https://mé.internal/`; `https://xn--m-bga.internal/` | refused; accepted |
| `TestValidateURLRootPath` | `https://h`; `https://h/` | path `/`, URL `https://h` both |
| `TestBlockedClasses` | `127.0.0.1`, `::1`, `169.254.169.254`, `fe80::1`, `224.0.0.1`, `0.0.0.0`, `::ffff:127.0.0.1`, `fd00:ec2::254`, `64:ff9b::7f00:1`, `2002:7f00:1::`, `2001:0:1::1`, `255.255.255.255`, `::` | true each |
| `TestNotBlocked` | `10.0.0.1`, `172.16.0.1`, `192.168.1.1`, `100.64.0.1`, `fc00::1`, `8.8.8.8`, `::ffff:10.20.0.4` | false each |
| `TestClassPermittedLiftsOneAddress` | policy networks `[169.254.169.254/32]`: `169.254.169.254`, `169.254.169.253`; policy hostnames `[metrics.internal]` and no network: `127.0.0.1` | true, false; false |
| `TestAddressAllowed` | hostnames `[h]`, networks `[]`: `(10.1.1.1, h)`, `(10.1.1.1, other)`; networks `[10.0.0.0/8]`: `(10.1.1.1, other)`, `(11.0.0.1, other)`; empty policy: `(10.1.1.1, h)` | true, false; true, false; false |
| `TestResolveLiteralNeverCallsResolver` | origin literal `10.1.1.1` with a resolver that fails the test | `[10.1.1.1]` |
| `TestValidateURLDropsTheZone` | `https://[2001:db8::1%25eth0]/`; `https://[fe80::1%25eth0]/` | host `2001:db8::1`, literal; `ENDPOINT_ADDRESS_REJECTED` (link-local after the zone is dropped) |
| `TestResolveMixedAnswersFailClosed` | answers `[10.1.1.1, 127.0.0.1]` and `[127.0.0.1, 10.1.1.1]` under networks `[10.0.0.0/8]` | `ENDPOINT_ADDRESS_REJECTED` both |
| `TestResolveDedupes` | `[10.1.1.1, 10.1.1.1, ::ffff:10.1.1.2]` | `[10.1.1.1, 10.1.1.2]` |
| `TestResolveEmptyAnswers` | `[]`; a resolver error | `ENDPOINT_RESOLUTION_FAILED` both, and the error text holds neither the host nor the resolver's text |
| `TestResolveHostnameEntryAdmitsAnyClassSafeAnswer` | hostnames `[h]`, answer `203.0.113.5` | allowed |
| `TestErrorCarriesOnlyTheCode` | every refusal above | `err.Error()` equals the code; `errors.As` to `*Error` |

- [ ] Steps: tests red, implement, `make lint`, `make test`, commit `feat(netpolicy): add the outbound policy`.

**Done when:** the twenty tests pass and no error text can hold a URL, a host, or an address.

---

### Task 5: The policy transport (R103)

**Executor:** Subagent.

**Files:**
- Create: `internal/platform/netpolicy/transport.go`, `transport_test.go`

**Interfaces:**

```go
// Transport is the http.RoundTripper of every request to a metric source:
// it validates the URL, resolves the host again, dials the first allowed
// address as a literal, compares the peer, refuses a redirect, and closes
// the connection after the body. It uses no proxy and keeps no connection.
type Transport struct {
	Policy    Policy
	Resolve   Resolver    // nil means SystemResolver
	TLS       *tls.Config // ServerName is set per request from the origin's host
	Dialer    *net.Dialer // nil means a dialer with no timeout; the client's timeout bounds the request
}

func (t *Transport) RoundTrip(r *http.Request) (*http.Response, error)
```

The round trip: `t.Policy.ValidateURL(r.URL.String())`; `t.Policy.Resolve`; `addr := addrs[0]`; dial `net.JoinHostPort(addr.String(), port)` through `Dialer.DialContext`; compare `conn.RemoteAddr()`'s address (unmapped, no zone) with `addr`, closing the connection and answering `CodeConnectionRejected` when they differ; for `https`, wrap in `tls.Client` with a clone of `TLS` whose `ServerName` is the origin's host (an IP literal gets no `ServerName` and `InsecureSkipVerify` as the caller set it), handshake with the request's context; write the request with `Host` set to the origin's authority through an `http.Transport` built per request with `DialContext` returning that one connection, `DisableKeepAlives: true`, `Proxy: nil`, `ForceAttemptHTTP2: false`; a response with status 300 to 399 is closed and answered with `CodeRedirectRejected`. A TLS failure is returned as the underlying error (the client classifies it as `KindTLS`), never wrapped into a policy code. The body of the response, when read to the end or closed, closes the connection.

**Tests** (a loopback server through `httptest`; the policy admits it with `Networks: [127.0.0.1/32]` and, for IPv6, `[::1/128]`; a resolver stub answers the loopback for a chosen name):

| Test | Input | Expected |
|---|---|---|
| `TestTransportPostsOverLoopback` | `POST http://metrics.test/api/v1/query` with the stub resolving `metrics.test` to `127.0.0.1` on the server's port | 200, the server saw `Host: metrics.test:<port>` and the body; one resolver call |
| `TestTransportNeverResolvesALiteral` | `http://127.0.0.1:<port>/x` with a resolver that fails the test | 200 |
| `TestTransportRefusesAnUnlistedAddress` | the stub answers `127.0.0.1` but the policy's networks are `[10.0.0.0/8]` | `ENDPOINT_ADDRESS_REJECTED`, the server saw no request |
| `TestTransportRefusesARebindingPeer` | a `Dialer` whose `DialContext` returns a connection to a second server on another port | `ENDPOINT_CONNECTION_REJECTED`, the connection closed, the second server saw no request |
| `TestTransportRefusesEveryRedirect` | the server answers 301, 302, 303, 307, 308 | `ENDPOINT_REDIRECT_REJECTED` each; the body closed; the server saw one request each |
| `TestTransportRefusesAnInvalidURL` | `http://metrics.test/../x` | `ENDPOINT_URL_REJECTED`, no resolution, no request |
| `TestTransportResolvesPerRequest` | two requests | two resolver calls |
| `TestTransportTLSWithMatchingName` | `httptest.NewTLSServer`, whose certificate names `example.com` and `127.0.0.1`; `TLS.RootCAs` holding that certificate; the stub resolving `example.com` to the loopback | 200 |
| `TestTransportTLSNameMismatchFails` | the stub resolving `other.test` to the loopback | a `*tls.CertificateVerificationError` (or `x509.HostnameError`) reaches the caller unwrapped; `errors.As` finds it |
| `TestTransportTLSInsecureSkipVerify` | the same with `InsecureSkipVerify` | 200 |
| `TestTransportNoProxy` | `HTTPS_PROXY=http://127.0.0.1:1` in the environment of the test (`t.Setenv`) | 200 over the loopback: the variable is ignored |
| `TestTransportNoKeepAlive` | two requests | the server's `ConnState` saw two connections |
| `TestTransportThroughThePromClient` | `prom.Connect` with `Options{Transport: &Transport{...}}` against the `promtest` server of `direct-complete` with the stub resolver | a bound client; a query answers samples |

- [ ] Steps: tests red, implement, `make lint`, `make test`, commit `feat(netpolicy): add the policy transport`.

**Done when:** the thirteen tests pass and the transport holds no connection between requests.

---

### Task 6: The profile tables and queries (R99, R106)

**Executor:** Subagent, against the running server.

**Files:**
- Create: `internal/platform/store/migrations/00002_profiles.sql`, `internal/platform/store/profiles.go`, `profiles_test.go`; `migrate_test.go` gains the trigger tests

The migration:

```sql
-- +goose Up
CREATE TABLE profiles (
    id                  text        NOT NULL,
    current_revision_id uuid,
    created_at          timestamptz NOT NULL,
    updated_at          timestamptz NOT NULL,
    CONSTRAINT pk_profiles PRIMARY KEY (id),
    CONSTRAINT ck_profiles_id CHECK (id ~ '^[a-z0-9]([a-z0-9._-]{0,61}[a-z0-9])?$' AND id NOT LIKE '%..%')
);

CREATE TABLE profile_revisions (
    id              uuid        NOT NULL,
    profile_id      text        NOT NULL,
    revision        integer     NOT NULL,
    fingerprint     text        NOT NULL,
    display_name    text        NOT NULL,
    active          boolean     NOT NULL,
    document        jsonb       NOT NULL,
    credential_file text        NOT NULL,
    ca_bundle_file  text        NOT NULL,
    created_by      uuid,
    created_at      timestamptz NOT NULL,
    CONSTRAINT pk_profile_revisions PRIMARY KEY (id),
    CONSTRAINT fk_profile_revisions_profile FOREIGN KEY (profile_id) REFERENCES profiles (id) ON DELETE RESTRICT,
    CONSTRAINT fk_profile_revisions_created_by FOREIGN KEY (created_by) REFERENCES users (id) ON DELETE RESTRICT,
    CONSTRAINT uq_profile_revisions_profile_revision UNIQUE (profile_id, revision),
    CONSTRAINT uq_profile_revisions_profile_fingerprint UNIQUE (profile_id, fingerprint),
    CONSTRAINT ck_profile_revisions_revision CHECK (revision >= 1)
);

ALTER TABLE profiles ADD CONSTRAINT fk_profiles_current_revision
    FOREIGN KEY (current_revision_id) REFERENCES profile_revisions (id) ON DELETE RESTRICT
    DEFERRABLE INITIALLY DEFERRED;

-- +goose StatementBegin
CREATE FUNCTION reject_profile_revision_mutation() RETURNS trigger AS $$
BEGIN
    RAISE EXCEPTION 'profile revisions are immutable' USING ERRCODE = 'integrity_constraint_violation';
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd
CREATE TRIGGER profile_revisions_are_immutable
    BEFORE UPDATE OR DELETE ON profile_revisions
    FOR EACH ROW EXECUTE FUNCTION reject_profile_revision_mutation();
CREATE TRIGGER profile_revisions_are_not_truncated
    BEFORE TRUNCATE ON profile_revisions
    FOR EACH STATEMENT EXECUTE FUNCTION reject_profile_revision_mutation();

-- +goose StatementBegin
CREATE FUNCTION check_profile_pointer() RETURNS trigger AS $$
BEGIN
    IF NEW.current_revision_id IS NOT NULL THEN
        PERFORM 1 FROM profile_revisions WHERE id = NEW.current_revision_id AND profile_id = NEW.id;
        IF NOT FOUND THEN
            RAISE EXCEPTION 'current_revision_id must name a revision of the same profile' USING ERRCODE = 'integrity_constraint_violation';
        END IF;
    END IF;
    RETURN NULL;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd
CREATE CONSTRAINT TRIGGER profile_pointer_matches_its_revision
    AFTER INSERT OR UPDATE ON profiles
    DEFERRABLE INITIALLY DEFERRED
    FOR EACH ROW EXECUTE FUNCTION check_profile_pointer();

-- +goose Down
ALTER TABLE profiles DROP CONSTRAINT fk_profiles_current_revision;
DROP TABLE profile_revisions;
DROP FUNCTION reject_profile_revision_mutation();
DROP FUNCTION check_profile_pointer();
DROP TABLE profiles;
```

**Interfaces:**

```go
type Profile struct {
	ID                string
	CurrentRevisionID *uuid.UUID
	CreatedAt, UpdatedAt time.Time
}

type ProfileRevision struct {
	ID             uuid.UUID
	ProfileID      string
	Revision       int
	Fingerprint    string
	DisplayName    string
	Active         bool
	Document       []byte // the canonical JSON of the document
	CredentialFile string
	CABundleFile   string
	CreatedBy      *uuid.UUID
	CreatedAt      time.Time
}

// GetOrCreateProfile inserts the identity when absent (ON CONFLICT DO NOTHING)
// and returns it locked FOR UPDATE for the transaction.
func GetOrCreateProfile(ctx context.Context, tx pgx.Tx, id string, now time.Time) (Profile, error)
// GetProfileForUpdate reads and locks the identity; ErrNotFound when absent.
func GetProfileForUpdate(ctx context.Context, tx pgx.Tx, id string) (Profile, error)
// AppendProfileRevision inserts a revision whose number is
// (SELECT coalesce(max(revision), 0) + 1 ...) computed inside the insert,
// and returns it. ErrFingerprintExists maps SQLSTATE 23505 on the
// fingerprint constraint.
func AppendProfileRevision(ctx context.Context, tx pgx.Tx, r ProfileRevision) (ProfileRevision, error)
// SetCurrentRevision moves the pointer (nil clears it) and updated_at.
func SetCurrentRevision(ctx context.Context, q Querier, profileID string, revisionID *uuid.UUID, now time.Time) error
// GetProfileRevision reads one revision by id.
func GetProfileRevision(ctx context.Context, q Querier, id uuid.UUID) (ProfileRevision, error)
// GetCurrentRevision reads the current revision of a profile; ErrNotFound
// when the profile is absent or its pointer is null.
func GetCurrentRevision(ctx context.Context, q Querier, profileID string) (ProfileRevision, error)
// ListCurrentRevisions reads every current revision, in profile id order.
func ListCurrentRevisions(ctx context.Context, q Querier) ([]ProfileRevision, error)
// FindProfileRevisionByFingerprint reads the revision with that fingerprint.
func FindProfileRevisionByFingerprint(ctx context.Context, q Querier, profileID, fingerprint string) (ProfileRevision, error)
// LatestProfileRevision reads the revision with the highest number of the
// profile, whether or not the pointer names it; ErrNotFound when none.
func LatestProfileRevision(ctx context.Context, q Querier, profileID string) (ProfileRevision, error)
```

**Tests:**

| Test | Input | Expected |
|---|---|---|
| `TestMigrateToVersionTwo` | `storetest.New` | version 2, both tables exist |
| `TestProfileRevisionsAreImmutable` | insert, then `UPDATE`, `DELETE`, `TRUNCATE` | SQLSTATE `23000` each |
| `TestProfilePointerMustNameOwnRevision` | profile `a`'s pointer set to a revision of `b`, in a transaction | the commit fails with `23000` |
| `TestProfilePointerDeferred` | insert the identity with the pointer, then the revision, in one transaction | commits |
| `TestProfileIDCheck` | id `A`, `..a`, `a..b`, 64 letters | `23514` each |
| `TestGetOrCreateProfileIsIdempotentAndLocks` | twice; a second transaction blocks | one row; blocks until commit |
| `TestAppendRevisionNumbersInsideTheInsert` | two revisions | 1, 2; a third after a `DELETE`-like pointer clear is 3 |
| `TestAppendRevisionRefusesARepeatedFingerprint` | the same fingerprint twice | `ErrFingerprintExists` |
| `TestAppendRevisionRace` | two transactions append at once with different fingerprints | both commit with 1 and 2, or one waits; never two with the same number |
| `TestSetCurrentAndGetCurrent` | set; clear | the revision; `ErrNotFound` |
| `TestListCurrentOrdersByID` | `zed`, `amy` with pointers, `kim` without | `amy`, `zed` |
| `TestFindByFingerprint` | present; absent | the row; `ErrNotFound` |
| `TestLatestProfileRevision` | three revisions, the pointer cleared; no revision | revision 3; `ErrNotFound` |

- [ ] Steps: tests red, implement, `make lint`, `make test`, tagged twice, commit `feat(store): add the profile tables`.

---

### Task 7: The document, the endpoint, the fingerprint, and the reference files (R100, R101, R102, R105, R111)

**Executor:** Subagent (the wording of the field words is fixed by R111).

**Files:**
- Create: `internal/platform/profiles/document.go`, `endpoint.go`, `fingerprint.go`, `refs.go`, and `document_test.go`, `endpoint_test.go`, `fingerprint_test.go`, `refs_test.go`

**Interfaces:**

```go
// Document is a profile as an administrator writes it and as a revision
// stores it. Its JSON form is the body of the API and the row's document.
type Document struct {
	DisplayName             string            `json:"display_name"`
	Active                  bool              `json:"active"`
	EndpointURL             string            `json:"endpoint_url"`
	EndpointType            string            `json:"endpoint_type"`     // auto, prometheus, grafana
	TimeoutSeconds          float64           `json:"timeout_seconds"`   // > 0 and <= 600
	TLSVerify               bool              `json:"tls_verify"`
	CredentialFile          string            `json:"credential_file"`   // "" for none
	CABundleFile            string            `json:"ca_bundle_file"`    // "" for none
	ClusterName             string            `json:"cluster_name"`
	ClusterLabel            string            `json:"cluster_label"`
	FilterMode              string            `json:"filter_mode"`       // required, auto, none
	NodepoolLabel           string            `json:"nodepool_label"`
	NodeAutoscaling         string            `json:"node_autoscaling"`  // auto, enabled, disabled
	AllowedNamespaces       []string          `json:"allowed_namespaces"`
	ExcludedContainers      []string          `json:"excluded_containers"`
	PermittedPeriods        []string          `json:"permitted_periods"`
	DefaultPeriod           *string           `json:"default_period"`
	PermittedTimezones      []string          `json:"permitted_timezones"`
	DefaultTimezone         *string           `json:"default_timezone"`
	PermittedReportProfiles []string          `json:"permitted_report_profiles"`
	DefaultReportProfile    *string           `json:"default_report_profile"`
	PermittedMinSeverities  []string          `json:"permitted_min_severities"`
	DefaultMinSeverity      *string           `json:"default_min_severity"`
	Thresholds              config.Thresholds `json:"thresholds"`
}

// Defaults are the values a body may leave out: endpoint_type auto,
// timeout_seconds 90, tls_verify true, cluster_label cluster, filter_mode
// required, node_autoscaling auto, the thresholds of config.Default(), the
// lists empty, the defaults null.
func Defaults() Document

// Problem names what is wrong with one field.
type Problem struct {
	Field string
	Word  string // missing, invalid, repeated, empty, not_permitted, incoherent
}

// Validate applies every rule of the document and returns one Problem per
// field, in field order, or nil. It reads no file and resolves no name.
func (d Document) Validate() []Problem

// ReferenceNamePattern is the shape of a credential or CA bundle file name.
var ReferenceNamePattern = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9._-]{0,61}[a-z0-9])?$`)

// ValidReferenceName reports whether name matches the pattern and holds no "..".
func ValidReferenceName(name string) bool

// Fallbacks are the defaults of spec 5.2 a null default resolves to.
const (
	FallbackPeriod        = "24h"
	FallbackTimezone      = "America/Sao_Paulo"
	FallbackReportProfile = "detailed"
	FallbackMinSeverity   = "all"
)

// Resolved returns the four effective defaults of d.
func (d Document) ResolvedDefaults() (period, timezone, reportProfile, minSeverity string)

// CanonicalEndpoint applies the static rules of the policy to every
// candidate root of the endpoint and to the /query and /query_range of
// each, and returns the canonical URL of the endpoint. It resolves nothing.
func CanonicalEndpoint(p netpolicy.Policy, endpointURL, endpointType string) (string, error)

// Fingerprint is the SHA-256, in hex, of "kcr.profile.v1\n" followed by the
// canonical JSON of d (keys sorted, no spaces) and "\n" and the parent
// revision id, or "" for the first revision.
func Fingerprint(d Document, parent *uuid.UUID) string

// CanonicalJSON is the JSON of d with sorted keys and no spaces, the form
// stored in the row and hashed.
func CanonicalJSON(d Document) ([]byte, error)

// RunConfig builds the configuration of a run from the document, the
// resolved file paths, and the selection; the worker and the profile test
// use it. credentialPath and caBundlePath are "" when none.
func (d Document) RunConfig(caBundlePath string, period, timezone, reportProfile, minSeverity string, namespaces []string) config.Config

// The codes of a reference resolution.
type RefCode string

const (
	CodeCredentialInvalid     RefCode = "CREDENTIAL_FILE_INVALID"
	CodeCredentialUnreadable  RefCode = "CREDENTIAL_FILE_UNREADABLE"
	CodeCredentialTooLarge    RefCode = "CREDENTIAL_FILE_TOO_LARGE"
	CodeCABundleInvalid       RefCode = "CA_BUNDLE_INVALID"
	CodeCABundleUnreadable    RefCode = "CA_BUNDLE_UNREADABLE"
	CodeCABundleTooLarge      RefCode = "CA_BUNDLE_TOO_LARGE"
)

// RefError carries a code and nothing else.
type RefError struct{ Code RefCode }

// MaxReferenceBytes bounds a referenced file.
const MaxReferenceBytes = 1 << 20

// ResolveCredential reads the credential named under dir and returns the
// token: UTF-8, one trailing CRLF or LF removed, at least 8 characters.
func ResolveCredential(dir, name string) (string, error)

// ResolveCABundle checks the CA bundle named under dir holds at least one
// PEM certificate and returns its path, for the TLS configuration.
func ResolveCABundle(dir, name string) (string, error)
```

The rules of `Validate`, in field order, each yielding the word of R111 (one per field; the first failing rule of a field): `display_name` trimmed non-empty (`empty`), at most 128 runes and no control character (`invalid`); `endpoint_url` non-empty (`missing`; its policy check is `CanonicalEndpoint`, called by the service); `endpoint_type` one of three (`invalid`); `timeout_seconds` finite, `> 0`, `<= 600` (`invalid`); `credential_file` empty or a valid reference name (`invalid`); `ca_bundle_file` empty or a valid name (`invalid`), and set while `tls_verify` is false (`incoherent`); `cluster_name` trimmed non-empty (`empty`), no control character (`invalid`); `cluster_label` a Prometheus label name (`invalid`); `filter_mode` one of three; `nodepool_label` empty or a label name; `node_autoscaling` one of three; `allowed_namespaces` at least one (`empty`), each `^[a-z0-9](?:[-a-z0-9]{0,61}[a-z0-9])?$` (`invalid`), no repeat (`repeated`); `excluded_containers` each `^[A-Za-z0-9][A-Za-z0-9._-]{0,252}$`, no repeat; `permitted_periods` each `config.ValidatePeriod` ok, no repeat; `default_period` null or valid (`invalid`), and the resolved default within a non-empty permitted list (`not_permitted`); the same for time zones (`config.ValidateTimezone`), report profiles (`detailed`, `executive`), minimum severities (`all`, `attention`, `critical`); `thresholds` through `config.ValidateThresholds` (`invalid`).

`ResolveCredential` and `ResolveCABundle` share `locate(dir, name, invalid, unreadable RefCode) (string, error)`: an invalid name is `invalid`; `filepath.EvalSymlinks(dir)` failing is `unreadable`; `filepath.EvalSymlinks(filepath.Join(base, name))` failing with not-exist is `invalid`, with another error `unreadable`; the result must have `base + string(filepath.Separator)` as prefix, differ from `base`, and be a regular file (`os.Lstat` after the links are followed, so `Stat`), else `invalid`. Reading: `io.LimitReader` of `MaxReferenceBytes + 1`; more than the limit is `too_large`; a read error `unreadable`. The credential must be valid UTF-8 (`invalid`), stripped of one `\r\n` else one `\n`, and `config.CheckToken` must accept it (`invalid`). The CA bundle must yield at least one block of type `CERTIFICATE` through `encoding/pem` and `x509.ParseCertificate` (`invalid`).

**Tests:**

| Test | Input | Expected |
|---|---|---|
| `TestDefaults` | `Defaults()` | the values named above |
| `TestValidateAcceptsTheExample` | a complete valid document | nil |
| `TestValidateEachField` | one table row per rule above, one field broken at a time (about thirty rows) | the one `Problem{field, word}` of the rule |
| `TestValidateReportsEveryField` | three fields broken | three problems in field order |
| `TestValidateReferenceNames` | `../corp-root`, `/etc/ssl/certs/corp.pem`, `corp root`, `Corp-Root`, `.`, 64 letters, `a..b` | `invalid` each; `corp-root`, `a.b_c-d`, 63 characters accepted |
| `TestDocumentDefaultsWithinPermitted` | permitted periods `["7d"]`, default null; permitted `["7d"]`, default `7d`; permitted `[]`, default null | `not_permitted`; nil; nil (the fallback resolves to `24h`) |
| `TestResolvedDefaults` | all null; all set | the four fallbacks; the four values |
| `TestCanonicalEndpointValidatesEveryCandidate` | with an empty policy: `https://metrics.internal` (two candidates); `https://metrics.internal/api/v1`; `https://metrics.internal/api/v1/../x`; `https://127.0.0.1:9090`; with networks `[127.0.0.1/32]`: `https://127.0.0.1:9090` | `https://metrics.internal`; `https://metrics.internal/api/v1`; `ENDPOINT_URL_REJECTED`; `ENDPOINT_ADDRESS_REJECTED`; `https://127.0.0.1:9090` |
| `TestCanonicalEndpointForm` | `HTTPS://Metrics.Internal.:443/api/v1/` | `https://metrics.internal/api/v1` |
| `TestFingerprintChainsTheParent` | the same document with parent nil and with a parent id; A, B, A with chained parents | different; the third differs from the first |
| `TestCanonicalJSONIsStable` | a document decoded from its canonical JSON and encoded again | byte-equal; the lists keep their order |
| `TestRunConfig` | a document, a CA path, the selection | `config.Config` with the fields mapped: `Metrics.URL`, `EndpointType`, `TLSVerify`, `CABundle`, `TimeoutSeconds`; `Cluster`; `Analysis` with the selection; `Thresholds`; `Report.Profile`, `MinSeverity`; `TokenEnv` empty |
| `TestResolveReferenceConfinement` | a good name; a link inside; a link escaping; a directory; the folder itself (name `.` is invalid by pattern, so a link named `self` to the folder); a missing name; an unreadable folder | token; token; `CREDENTIAL_FILE_INVALID`; invalid; invalid; invalid; `CREDENTIAL_FILE_UNREADABLE` |
| `TestResolveCredentialText` | `token-one\r\n`; `token-one\n`; `token-one\n\n`; invalid UTF-8; `short\n`; 1 MiB + 1 byte | `token-one`; `token-one`; `token-one\n`; `CREDENTIAL_FILE_INVALID`; `CREDENTIAL_FILE_INVALID`; `CREDENTIAL_FILE_TOO_LARGE` |
| `TestResolveCABundle` | a PEM certificate (generate one in the test); a non-PEM file; a PEM of another block type; 1 MiB + 1 | the path; `CA_BUNDLE_INVALID`; invalid; `CA_BUNDLE_TOO_LARGE` |
| `TestRefErrorCarriesOnlyTheCode` | every refusal | the text equals the code, no path |

- [ ] Steps: tests red, implement, `make lint`, `make test`, commit `feat(profiles): add the document and the references`.

---

### Task 8: The profiles service and the profile test (R106, R107, R108, R112)

**Executor:** Subagent, against the running server.

**Files:**
- Create: `internal/platform/profiles/service.go`, `test.go`, and `service_test.go`, `test_test.go` (`//go:build postgres`)

**Interfaces:**

```go
type Service struct {
	Pool     *pgxpool.Pool
	Policy   netpolicy.Policy
	Settings settings.Settings // the two folders
	Now      func() time.Time
	Resolve  netpolicy.Resolver // nil means the system resolver
}

var (
	ErrNotFound       = errors.New("profile not found")
	ErrIDTaken        = errors.New("profile id taken")
	ErrRevisionStale  = errors.New("profile revision stale")
	ErrForbidden      = auth.ErrForbidden
)

// DocumentError carries the problems of a refused document.
type DocumentError struct{ Problems []Problem }

// Current is a profile as the administration sees it.
type Current struct {
	ID         string
	RevisionID uuid.UUID
	Revision   int
	Document   Document
	CreatedAt  time.Time
}

// Summary is a profile as every signed-in reader sees it.
type Summary struct {
	ID, DisplayName       string
	AllowedNamespaces     []string
	PermittedPeriods      []string; DefaultPeriod string
	PermittedTimezones    []string; DefaultTimezone string
	PermittedReportProfiles []string; DefaultReportProfile string
	PermittedMinSeverities []string; DefaultMinSeverity string
}

func (s *Service) Create(ctx context.Context, actor auth.Principal, id string, d Document) (Current, error)
// Change is the body of a revise: base and the fields present. A pointer
// field nil is absent; the service merges the present ones over the
// current document. Nullable defaults are cleared with an explicit null,
// carried as a present pointer to nil; the api decodes the body into a
// map to tell absent from null and builds Change from it.
type Change struct {
	BaseRevisionID uuid.UUID
	Fields         map[string]json.RawMessage // the fields present, by JSON name
}
func (s *Service) Revise(ctx context.Context, actor auth.Principal, id string, c Change) (Current, error)
func (s *Service) Delete(ctx context.Context, actor auth.Principal, id string) error
func (s *Service) Get(ctx context.Context, actor auth.Principal, id string) (Current, error)
func (s *Service) List(ctx context.Context, actor auth.Principal) ([]Current, error)
// ListActive answers the summaries of the active profiles, for any reader.
func (s *Service) ListActive(ctx context.Context) ([]Summary, error)

// TestResult is the answer of a profile test.
type TestResult struct {
	OK           bool
	Code         string
	Capabilities []string // the names whose state is not unavailable, in their order
}

// Test resolves the references, runs the preflight through the policy
// transport, and answers a result; it never returns an error for a failure
// of the source, only for the database or a missing profile.
func (s *Service) Test(ctx context.Context, actor auth.Principal, id string) (TestResult, error)
```

`Create`: role; `d.Validate()` then `CanonicalEndpoint` (a policy error is returned as `*netpolicy.Error`); in a transaction: `GetOrCreateProfile` (locked), refuse `ErrIDTaken` when the pointer is set; `AppendProfileRevision` with the fingerprint of parent nil when no revision exists, else the id of `store.LatestProfileRevision`; `SetCurrentRevision`; audit `profile.create`. `Revise`: role; in a transaction: `GetProfileForUpdate` (`ErrNotFound` also when the pointer is null); the stale check against the pointer; decode the current document, apply the present fields (an explicit null clears a nullable default; a null on another field is `invalid`), validate, canonical endpoint when `endpoint_url` or `endpoint_type` is present; when the merged canonical JSON equals the current one, return the current and write nothing; else append with parent `BaseRevisionID`, move the pointer, audit `profile.revise`, or `profile.activate`/`deactivate` when `active` changed. `Delete`: role; lock; `ErrNotFound` when absent or cleared; clear the pointer; audit `profile.delete`. `Test`: role; read the current revision (no lock, no transaction held afterwards); `ResolveCredential` when named (`PROFILE_CREDENTIAL_UNAVAILABLE` on any `RefError`); `ResolveCABundle` when named and `tls_verify` (`PROFILE_CA_BUNDLE_UNAVAILABLE`); `RunConfig` with the resolved defaults and `allowed_namespaces`; a context with deadline `3 * timeout_seconds`; `discovery.RunPreflight` with `prom.Options{Metrics, Token, Transport: &netpolicy.Transport{Policy, Resolve, TLS}}` where `TLS` carries `RootCAs` from the bundle and `InsecureSkipVerify` when `tls_verify` is false; map the error as R108 says; `Capabilities` from the preflight; audit `profile.test`.

**Tests** (the harness seeds `root` administrator and `eve` reader; a `Service` with `Policy{Networks: [127.0.0.1/32]}`, folders under `t.TempDir()`, and a `Resolve` stub mapping `metrics.test` to `127.0.0.1`; the fake Prometheus is `promtest.NewServer` of `direct-complete` on `httptest`):

| Test | Input | Expected |
|---|---|---|
| `TestCreateStoresCanonicalAndAudits` | `root` creates `prod` with `HTTPS://Metrics.Test./api/v1/` | `Current` with the canonical endpoint, revision 1; the row's document equal; audit `profile.create` with `fields` sorted |
| `TestCreateRefusesNonAdministrator` | `eve` | `ErrForbidden`, nothing written |
| `TestCreateRefusesDocument` | `cluster_name` empty | `*DocumentError` with `{cluster_name, empty}`, nothing written |
| `TestCreateRefusesPolicy` | `http://169.254.169.254/` | `*netpolicy.Error` `ENDPOINT_ADDRESS_REJECTED`, nothing written |
| `TestCreateRefusesTakenID` | twice | `ErrIDTaken` |
| `TestCreateAfterDeleteContinuesTheCount` | create, delete, create | revision 2 |
| `TestReviseAppendsAndKeepsHistory` | revise `display_name` | revision 2 current; revision 1 still readable; audit `profile.revise` with `fields: ["display_name"]` |
| `TestReviseStale` | base = revision 1 after revision 2 exists | `ErrRevisionStale` |
| `TestReviseNullClearsADefault` | `default_period: null` | the default null; `fields: ["default_period"]` |
| `TestReviseAbsentKeepsValue` | only `display_name` present | every other field unchanged |
| `TestReviseNoOpWritesNothing` | the same values | the current revision, no row, no audit |
| `TestReviseRefusesMergedDocument` | `tls_verify: false` while a CA bundle is named | `{ca_bundle_file, incoherent}`, nothing written |
| `TestReviseEndpointRevalidated` | `endpoint_url: "http://169.254.169.254/"` | `ENDPOINT_ADDRESS_REJECTED`, nothing written |
| `TestReviseABAProducesThreeRevisions` | A, B, A | three revisions, three fingerprints |
| `TestReviseActiveWritesActivateOrDeactivate` | `active: true`; then `false` | `profile.activate`; `profile.deactivate` |
| `TestReviseConcurrentlyOneWins` | two goroutines revise with base = revision 1 | one `Current` at revision 2, one `ErrRevisionStale`; two rows |
| `TestDeleteClearsAndRetains` | delete | `Get` is `ErrNotFound`; the revisions remain; audit `profile.delete` |
| `TestDeleteUnknown` | an unknown id | `ErrNotFound` |
| `TestListActiveSummaries` | an active and an inactive profile | one summary with the eleven fields and the resolved defaults, no endpoint, no file names |
| `TestListIncludesInactiveForAdministrators` | the same | two `Current` |
| `TestTestOK` | a profile at `http://metrics.test:<port>`; when the recorded requests of the scenario carry an `Authorization` header (`promtest.Fixture`), a credential file holding that token, else no credential | `{true, PROFILE_OK, [the capability names whose state is not unavailable]}`; audit `profile.test` succeeded with the code |
| `TestTestRefusedByPolicy` | the policy's networks `[10.0.0.0/8]` | `{false, ENDPOINT_ADDRESS_REJECTED, nil}`, audit failed; no request reached the server |
| `TestTestCredentialUnavailable` | a credential name with no file | `PROFILE_CREDENTIAL_UNAVAILABLE`; the server saw no request |
| `TestTestCABundleUnavailable` | a CA bundle name of a non-PEM file | `PROFILE_CA_BUNDLE_UNAVAILABLE` |
| `TestTestAuthRejected` | a server answering 401 | `ENDPOINT_AUTH_REJECTED` |
| `TestTestAPINotFound` | a server answering 404 to every root | `ENDPOINT_API_NOT_FOUND` |
| `TestTestUnreachable` | a closed port with `timeout_seconds` 1 | `ENDPOINT_UNREACHABLE` within the deadline |
| `TestTestScopeNotFound` | `filter_mode` required and a cluster name the scenario does not carry | `CLUSTER_SCOPE_NOT_FOUND` |
| `TestTestRunsForInactive` | an inactive profile | runs |
| `TestTestResultCarriesNoSecret` | every failing result above | the JSON of the result holds no `http`, no address, no file name, no token |

- [ ] Steps: tests red, implement, `make lint`, `make test`, tagged twice, commit `feat(profiles): add the service and the test`.

---

### Task 9: The profile routes and the contract (R107, R111)

**Executor:** Main thread writes the contract additions; a subagent the routes.

**Files:**
- Modify: `api/openapi.yaml` (the seven routes, the schemas `ProfileDocument`, `ProfileCreate`, `ProfileChange`, `Profile`, `ProfileList`, `ProfileSummary`, `ProfileSummaryList`, `ProfileTestResult`, the codes), `internal/platform/api/openapi.gen.go` (regenerated), `errors.go` (the codes `PROFILE_NOT_FOUND` 404, `PROFILE_ID_TAKEN` 409, `PROFILE_REVISION_STALE` 409, `PROFILE_DOCUMENT_INVALID` 422, `ENDPOINT_URL_REJECTED` 422, `ENDPOINT_ADDRESS_REJECTED` 422, with the original's sentences), `server.go` (the field `Profiles *profiles.Service`), `contract_test.go`
- Create: `internal/platform/api/profiles.go`, `profiles_test.go`

Routes, in `Routes()` order after the user routes:

| Method and path | Checks | Answer |
|---|---|---|
| `GET /api/v1/profiles` | session | 200 `{"profiles":[summary...]}` |
| `GET /api/v1/admin/profiles` | session, administrator | 200 `{"profiles":[profile...]}` |
| `POST /api/v1/admin/profiles` | session, administrator, CSRF, body `ProfileCreate` = `{"id", ...document}` with `id` required and the document fields optional over `Defaults()` except `display_name`, `endpoint_url`, `cluster_name`, `allowed_namespaces` (required) | 201 `profile` |
| `GET /api/v1/admin/profiles/{id}` | session, administrator | 200 `profile`; an invalid or unknown id 404 |
| `PATCH /api/v1/admin/profiles/{id}` | session, administrator, CSRF, body `{"base_revision_id", ...fields}` | 200 `profile` |
| `DELETE /api/v1/admin/profiles/{id}` | session, administrator, CSRF | 204 |
| `POST /api/v1/admin/profiles/{id}/test` | session, administrator, CSRF | 200 `{"ok","code","capabilities"}` |

`profile` is `{"id","revision_id","revision","created_at", ...document}`. The create body is decoded with `DecodeBody` into `ProfileCreate` (with `maxLen` 128 for `display_name`, 2048 for `endpoint_url`, 253 for `cluster_name`, 63 for `id`); the change body is decoded into a `map[string]json.RawMessage` after the same walk (so absent and null are told apart), `base_revision_id` parsed, the rest handed to `profiles.Change`.

**Tests** (through the harness of 3.1, extended with a `Profiles` service on the loopback policy):

| Test | Input | Expected |
|---|---|---|
| `TestProfilesListAsReader` | `eve` | 200, the summaries, exactly the eleven keys |
| `TestProfilesListWithoutSession` | nothing | 401 |
| `TestAdminProfilesCreateReadReviseDelete` | the sequence | 201, 200, 200 (revision 2), 204, then 404 `PROFILE_NOT_FOUND` |
| `TestAdminProfilesAsReader` | `eve` on each admin route | 403 before any lookup (an unknown id too) |
| `TestAdminProfilesCreateRefusals` | missing `cluster_name`; `cluster_name` empty; `endpoint_url` `not a url`; a repeated namespace; an unknown key | 400 `{"cluster_name":"missing"}`; 422 `PROFILE_DOCUMENT_INVALID` `{"cluster_name":"empty"}`; 422 `ENDPOINT_URL_REJECTED` `{"endpoint_url":"invalid"}`; 422 `{"allowed_namespaces":"repeated"}`; 400 `{"x":"unknown"}` |
| `TestAdminProfilesCreateTaken` | twice | 409 `PROFILE_ID_TAKEN` |
| `TestAdminProfilesPatchStale` | an old base | 409 `PROFILE_REVISION_STALE` |
| `TestAdminProfilesPatchNull` | `{"base_revision_id":..., "default_period": null}` | 200 with `default_period` null |
| `TestAdminProfilesPatchRequiresBase` | no `base_revision_id` | 400 `{"base_revision_id":"missing"}` |
| `TestAdminProfilesTest` | a profile on the fake server | 200 `{"ok":true,"code":"PROFILE_OK","capabilities":[...]}`; the body holds no `http` |
| `TestAdminProfilesTestRefused` | a profile the policy refuses | 200 `{"ok":false,"code":"ENDPOINT_ADDRESS_REJECTED","capabilities":[]}` |
| `TestProfilesRoutesRequireCSRF` | `POST`, `PATCH`, `DELETE`, `test` without the header | 403 each |
| `TestRoutesMatchTheContract` | unchanged | passes with sixteen routes |

- [ ] Step 1 (main thread): write the contract, `make generate`, commit `feat(api): extend the contract with profiles`.
- [ ] Steps (subagent): tests red, implement, `make lint`, `make test`, `make generate-check`, tagged twice, commit `feat(api): add the profile routes`.

---

### Task 10: `admin import-profiles` (R110)

**Executor:** Subagent, against the running server.

**Files:**
- Create: `internal/platform/profiles/importer.go`, `importer_test.go`
- Modify: `internal/cli/admin.go`, `admin_test.go`

**Interfaces:**

```go
// ImportFile is the file of admin import-profiles.
type ImportFile struct {
	Profiles []ImportEntry `toml:"profiles"`
}
// ImportEntry is one profile of the file: its id and its document, under
// the JSON names of the document as TOML keys.
type ImportEntry struct {
	ID string `toml:"id"`
	// the fields of Document, with toml tags equal to the json tags
}

// ReadImportFile parses and validates the file whole: an unknown key, an
// inline credential key, a repeated id, an invalid document, or a refused
// endpoint is an error that names the profile and the field, never a value.
func ReadImportFile(path string) ([]ImportEntry, error)

// Outcome says what the import did with one profile.
type Outcome struct {
	ID     string
	Action string // created, revised, unchanged
}

// Import creates or revises the profiles in one transaction, with no actor.
func (s *Service) Import(ctx context.Context, entries []ImportEntry) ([]Outcome, error)
```

The command: `admin import-profiles --file <path> [--config platform.toml]`; `openPlatform`, `store.Migrate` as the other admin commands, `ReadImportFile` (an error is exit 2: `error: <path>: profile <id>: <field> <word>` or `error: <path>: unknown key <key>` or `error: <path>: profile <id> is repeated`), `Import` (an error exit 1), then one line per outcome on standard output, exit 0.

**Tests:**

| Test | Input | Expected |
|---|---|---|
| `TestReadImportFileAcceptsTheExample` | two profiles | two entries with defaults filled |
| `TestReadImportFileRefusals` | an unknown key; `token = "x"`; `ca_bundle = "/x"`; a repeated id; an empty `cluster_name`; a metadata endpoint | the messages, none holding `x` or the URL |
| `TestImportCreatesRevisesAndLeavesUnchanged` (postgres) | first run with two profiles; second run with one changed; third run equal | `created` twice; `revised`, `unchanged`; `unchanged` twice; the revisions and the audit rows with no actor |
| `TestImportLeavesOtherProfilesAlone` (postgres) | a profile in the database not in the file | untouched |
| `TestImportIsOneTransaction` (postgres) | the second entry's endpoint refused | nothing written for the first either (the file is validated whole before any write) |
| `TestAdminImportProfilesCommand` (postgres) | through `Run` with a file | stdout `created a` and `created b`, exit 0; a second run `unchanged` |
| `TestAdminImportProfilesRefusals` | a missing file; an invalid file | exit 2, one `error:` line each |
| `TestAdminListsFourCommands` | `admin hunter2` | `the commands are: bootstrap-admin, import-profiles, migrate, probe` |

- [ ] Steps: tests red, implement, `make lint`, `make test`, tagged twice, commit `feat(cli): add admin import-profiles`.

---

### Task 11: Documents, changelog, review, gate, close

**Executor:** Main thread.

- [ ] `AGENTS.md`: the layer table (105, 145); "The platform" gains the policy (the six rules in one paragraph, the loopback admitted only by a one-address entry), the references (the name rule, the folder confinement, the 1 MiB cap, resolution only in the test and the worker), the revisions (immutable, the pointer, `base_revision_id`), `admin import-profiles`; the command list gains it.
- [ ] `README.md`: the profile routes in one paragraph, the four settings, the import file with an example.
- [ ] `CHANGELOG.md`, `### Added` under Unreleased: the profiles through the API; the profile test; the outbound policy and its two allowlists; the credential and CA bundle folders; `admin import-profiles`. `### Fixed`: nothing user-visible (F-112, F-113 are internal to a behaviour already described).
- [ ] The adversarial review of the branch (`.cache/phase-3/part-2/review/`), with the policy and the references as its focus; one fix wave; one re-review.
- [ ] `make gate`; logs to `.cache/phase-3/part-2/gate/`.
- [ ] `ROADMAP.md` updated; the closing report; the owner's word to merge by fast-forward, no version, no tag.
