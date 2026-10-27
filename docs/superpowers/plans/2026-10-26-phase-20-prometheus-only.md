# Phase 20: Prometheus-compatible sources only — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Remove Grafana as a metric source and, with it, the `endpoint_type` option everywhere, carrying stored data forward to snapshot schema 4.0.

**Architecture:** A source is one URL to a Prometheus-compatible API (Prometheus, Thanos Query Frontend, VictoriaMetrics, Cortex). `prom.Candidates` keeps its two candidate roots (the URL, and the URL with `/api/v1` appended) and loses the kind. The snapshot drops `report.settings.endpoint_type` and becomes schema 4.0; migration `00009` rewrites every stored 3.0 snapshot and drops the key from every profile revision, one way, as `00008` did for 3.0.0.

**Tech Stack:** Go 1.27, PostgreSQL 18 with goose migrations, React and TypeScript, OpenAPI code generation (`make generate`).

**Spec:** The owner's rulings of 2026-10-26 (recorded in `ROADMAP.md`, phase 20): remove `endpoint_type` whole; schema 4.0 with a one-way migration; remove the scenario `grafana-authenticated`. The design spec `docs/superpowers/specs/2026-09-27-k8s-capacity-report-design.md` is amended in Task 5.

## Global Constraints

- No compatibility path: no reader of schema 3.0, no tolerated `endpoint_type` key, no mapping. An input that still names it is refused where unknown keys are refused today (`report.toml`, the profile document, the API body, the import file, the chart values); the variable `CAPACITYROOM_ENDPOINT_TYPE` is no longer read.
- `SchemaVersion` becomes `"4.0"`. A snapshot `3.0` is named and refused by the existing version check.
- Released migrations are never edited. `00008_drop_min_severity.sql` stays byte for byte; its tests move to fixtures of their own (Task 1).
- The numbers equal the original exactly for every remaining scenario. The recorded files of the removed scenario are deleted, not edited; no other file of `testdata/prometheus/` or `testdata/python/` changes.
- No comment speaks of the process; diagnostics in English; every commit names its paths.
- Subagents never run Docker; `make update-golden`, `make test-postgres`, `make gate` stay on the main thread, which hands a running PostgreSQL DSN to the implementer.

## Review Focus

- A stored snapshot whose settings held `"endpoint_type": "grafana"` must migrate like a `"prometheus"` one. Task 3 migrates the 3.0 fixture of the removed scenario.
- A profile revision whose URL is a Grafana datasource proxy (`/api/datasources/uid/x/resources/api/v1`) must still load and validate after the migration: its URL stays and is now a plain root. Task 3 asserts that such a revision reads back valid.
- A snapshot whose bytes were not written by `Encode` (another indentation) cannot be rewritten by the pattern. The migration must fail loudly, as `00008` does, not leave a 3.0 row behind. Task 3 asserts the refusal.
- A `report.toml` that still holds `endpoint_type`, and a profile body or import file that still holds it, must be refused with the existing unknown-key error, not ignored. Tasks 2 and 3 each add one test.
- A URL whose path holds `datasource` or `/proxy/` must no longer get any special treatment. Task 2 asserts that `Candidates` returns the same roots for such a path as for any other.

---

### Task 1: Fixtures of schema 3.0 for the migration tests (main thread, before any code change)

**Files:**
- Create: `internal/platform/store/testdata/schema-3.0/*.json` (copies of every `testdata/snapshot/*.json` at 5748b4f, `grafana-authenticated.json` included)
- Modify: `internal/platform/store/migrate_minimum_severity_test.go` (compare the migrated 2.0 rows with `testdata/schema-3.0/` instead of `testdata/snapshot/`, and migrate to version 8 alone)

- [ ] Copy the files; run `go test -tags=postgres -run MigrationEight ./internal/platform/store/` against the running PostgreSQL: PASS.
- [ ] Commit `test(store): pin the 3.0 snapshots of migration 8`.

### Task 2: The core without the endpoint kind

**Files:**
- Modify: `internal/prom/endpoint.go`, `internal/prom/client.go` (drop `kind`, `Kind()`), `internal/config/config.go`, `load.go`, `validate.go`, `internal/cli/report.go`, `internal/cli/run.go`, `internal/snapshot/snapshot.go`, `build.go`, `codec.go`, `internal/pipeline/pipeline.go`, `internal/goldentest/goldentest.go`, `internal/promtest/scenarios.go`
- Delete: `testdata/prometheus/grafana-authenticated.json`, `testdata/prometheus-added/grafana-authenticated.json`, `testdata/python/grafana-authenticated.json`, `testdata/python/grafana-authenticated.queries.txt`, `testdata/queries/grafana-authenticated.txt`, `testdata/snapshot/grafana-authenticated.json`, and their rows in `testdata/prometheus/README.md`, `testdata/prometheus-added/README.md`, `testdata/python/README.md`
- Modify tests: those the map names (`internal/config/validate_test.go`, `load_test.go`, `internal/prom/endpoint_test.go`, `scenario_test.go`, `internal/snapshot/build_test.go`, `internal/promtest/fixture_test.go`, `internal/discovery/capabilities_test.go`, `preflight_test.go`, `internal/goldentest/*_test.go`, `internal/document/view_test.go`), `internal/config/testdata/full.toml`, `broken.toml`

**Interfaces:**
- Produces: `func Candidates(address string) []string` in `internal/prom`; `snapshot.SchemaVersion == "4.0"`; `snapshot.Settings` without `EndpointType`; `config.Metrics` without `EndpointType`; `promtest.Scenarios` of seven.

- [ ] Tests first: `TestCandidatesIgnoresGrafanaPaths` (a path with `datasource` and one with `/proxy/` give the URL and the URL plus `/api/v1`, as any path does); `TestLoadRefusesEndpointType` (a `report.toml` with `[metrics] endpoint_type = "prometheus"` fails with the unknown-key error); `TestReadRefusesSchemaThree` (a snapshot with `"schema_version": "3.0"` fails naming `"3.0"` and `"4.0"`). Run them: FAIL.
- [ ] Remove the kind, the key, the flag `--endpoint-type`, the variable, the validation errors `errMetricsURLGraf` and `errEndpointType`, the snapshot field and enum, and the scenario. The `--endpoint-type` flag is then refused as unknown by the one flag-error function.
- [ ] Fix the count tests by meaning: `len(promtest.Scenarios)` is 7; a comparison with the original walks the five recorded scenarios of `testdata/prometheus/` it still has.
- [ ] `make update-testdata`; read every changed file whole. Each snapshot loses one line and its version reads `4.0`; a query list, a data file, or a view that changes otherwise is a defect.
- [ ] `go test ./...` and the tagged `template` tests: PASS. The `TestTheCommandsHaveTheirFlags`-style test names no `--endpoint-type`.
- [ ] Commit by path: `feat(core)!: drop the endpoint type`.

### Task 3: The platform and migration 00009

**Files:**
- Create: `internal/platform/store/migrations/00009_drop_endpoint_type.sql`, `internal/platform/store/migrate_endpoint_type_test.go`
- Modify: `internal/platform/profiles/document.go`, `endpoint.go` (`CanonicalEndpoint(address string)`), `importer.go`, `service.go`, `internal/platform/api/profiles.go`, `api/openapi.yaml` (remove the `EndpointType` schema and the field from every profile schema and `required`), regenerated `internal/platform/api/openapi.gen.go`, `deploy/helm/capacity-room/values.schema.json` (the `endpoint_type` profile key), `deploy/helm/capacity-room/examples/{gateway,ingress}.yaml`, `internal/chart/testdata/profiles.yaml`, `deploy/platform/profiles.toml`, and the tests of these packages

**Interfaces:**
- Consumes: `prom.Candidates(address string) []string` (Task 2).
- Produces: no `endpoint_type` in the profile document, the API, the import file, or the chart; `make generate` output for Task 4.

- [ ] The migration, on the model of `00008`: with `report_snapshots_are_immutable` disabled for the one statement, every row at `schema_version = '3.0'` gets its body rewritten (`'^\{\n  "schema_version": "3\.0",'` to 4.0, and `'\n      "endpoint_type": "(prometheus|grafana)",'` removed), its `sha256`, `size_bytes`, and `schema_version = '4.0'`; a `DO` block raises when a row is left at 3.0 or still matches `'\n      "endpoint_type": '`; with `profile_revisions_are_immutable` disabled, `document = document - 'endpoint_type'`; `reports.request` loses `endpoint_type` only if a stored request can hold it (check `internal/platform/reports` first, and leave it out if not). The down refuses, as `00008`'s does.
- [ ] Tests (tag `postgres`): `TestMigrationNineCarriesStoredDataForward` inserts every `testdata/schema-3.0/` fixture at version 8, migrates, and compares each body with `testdata/snapshot/<name>.json` byte for byte, with its digest and size; the fixture `grafana-authenticated` is compared with itself minus its version and endpoint lines, and decodes under `snapshot.Decode`. `TestMigrationNineDropsTheKeyFromRevisions` stores a revision with `"endpoint_type": "grafana"` and a datasource-proxy URL, migrates, and reads the profile back valid with the URL unchanged. `TestMigrationNineRefusesASnapshotItCannotCarry` stores a 3.0 body indented by four and expects the migration to fail and roll back. `TestMigrationNineCannotBeReverted`.
- [ ] Tests (untagged where possible): a profile body, a draft test body, and an import file with `endpoint_type` are refused as an unknown key (400 for the API, exit 2 for `admin import-profiles`).
- [ ] `make generate`, `make generate-check`; the route-table and contract tests pass.
- [ ] `go test ./...`, then the main thread runs `make test-postgres` and `make test-chart`.
- [ ] Commit by path: `feat(platform)!: drop the endpoint type`.

### Task 4: The web

**Files:**
- Modify: `web/src/features/admin/profiles/form.ts`, `profile-form-page.tsx`, `profiles-page.tsx`, `web/src/i18n/families.ts`, `web/src/locales/en.json`, `pt-BR.json` (the `endpointType` block), `web/src/features/admin/profiles/form.test.ts`, `profile-form.test.tsx`, `web/tests/msw.ts`, `web/e2e/concurrency.spec.ts`, `web/e2e/home.spec.ts`, `DESIGN.md` if it names the select

**Interfaces:**
- Consumes: `web/src/api/schema.gen.ts` from Task 3, with no `EndpointType`.

- [ ] Remove the select, the form value, the profile fact, the family, and the locale keys; every test body drops the key.
- [ ] `npx tsc --noEmit -p .`, `npx eslint .`, `npx vitest run`: PASS, with the locale and family tests finding no orphan key.
- [ ] Commit by path: `feat(web)!: drop the endpoint type`.

### Task 5: Documents (main thread wording)

**Files:**
- Modify: `docs/superpowers/specs/2026-09-27-k8s-capacity-report-design.md` (the config table row, the variable, the flag, the snapshot contract, the scenario list), `AGENTS.md` (the schema is `4.0`; "the eight it always walked" becomes seven; a rule for `00009` beside the one for `00008`), `docs/operations.md` (the `report.toml` keys, an "Upgrading from 3.x" section: back up, stop the worker, and the one edit that converts a 3.0 snapshot file: `sed -i -e '2s/"3\.0"/"4.0"/' -e '/^      "endpoint_type": /d' <file>`), `deploy/helm/capacity-room/README.md` (the upgrade note), `CHANGELOG.md`

- [ ] The guide's tests (`docs/operations.md` against the code) pass; a 3.0 snapshot of 3.1.0, converted by the documented line, renders with `capacity-room report --from-snapshot <file>` on 4.0.0-dev. Read the guide's "Upgrading from 2.x" section first and write the new one in its shape.
- [ ] CHANGELOG under `[Unreleased]`: `### Removed` (Grafana as a source and the endpoint type: the flag, the key, the profile field, the API field, and the variable; a snapshot file of 3.x is refused, and the guide shows the edit), `### Changed` (the upgrade carries every stored report and profile forward, one way; back up first).
- [ ] Commit by path: `docs: sources are Prometheus-compatible only`.

### Task 6: Proof (main thread)

- [ ] `make update-golden` only if a page changed; open every changed page.
- [ ] Upgrade run: the 3.1.0 stack with a collected report and a profile at a datasource-proxy URL, a database backup, then 4.0.0-dev: the report opens, its snapshot downloads at 4.0, the profile edits and saves.
- [ ] Adversarial review of the whole branch; `make gate`; `make audit`. The version is the owner's word.
