# Phase 4.3: Calendar Ranges Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** A report covers either the last N, as today, or a calendar range that ends at or before the run, set in `report.toml`, on the command line, through the API, or in the form, where six calendar ranges (today so far, yesterday, this week so far, previous week, this month so far, previous month) are resolved when the request is sent. Closes F-131.

**Architecture:** A range is two wall times, `YYYY-MM-DD HH:MM`, read in the report's time zone. One parser in `internal/config` serves the configuration, the window, and the platform. `config.Analysis` gains `Start` and `End`, exclusive with `Period`. `collect.NewWindow` takes a range as given, and the last N path stays byte-identical. The snapshot becomes `2.0`: `report.period` leaves it, and the end may precede the emission. The platform carries `start` and `end` in the request JSON it already stores. The frontend's picker holds one of three kinds of value, and a calendar range becomes a typed range at sending.

**Tech Stack:** As in part 4.2, plus one dependency: `@daypicker/react` 10.0.2, exact, which brings `react-day-picker`, `date-fns`, and `@date-fns/tz` transitively. Go 1.27; React 19.3.0, TanStack Query 5.104.1, React Hook Form 7.89.0 with Zod 4.6.5, Radix UI 1.7.0, Tailwind CSS 4.3.3, react-i18next 17.0.16; Vitest 5.0.3 with MSW 3.0.2; Playwright 1.63.0.

**Spec:** `docs/superpowers/specs/2026-09-27-k8s-capacity-report-design.md` as amended in `693ae1e` (approved by the owner on 2026-10-07): sections 3, 5.1, 5.2, 6.4, 8, 8.3, 11.6, 11.7, 12.3, 13.2. The mockups the owner chose are under `.cache/phase-4/brainstorm/.superpowers/brainstorm/34726-1791403860/content/`: `range-picker.html` (option 1A, chosen) and `section-3-form.html` (the behaviour and every word, approved). The HTML they were drawn from is `.cache/phase-4/part-3/look/recap.html`.

**Depends on:** Release 0.3.0 (`f391d26`), on `main` and pushed. Branch `feat/calendar-ranges`, opened from `f391d26`.

## Global Constraints

Every constraint of the plans of phases 0 to 4.2 stands, and `AGENTS.md` binds every task. The ones this part leans on most:

- **No backward compatibility** (the owner's rule): `report.period` leaves the snapshot with no reader of `1.0`; a report stored before this part fails its view and its new PDFs, and is requested again.
- **The numbers equal the original exactly**, and the last N path is byte-identical: the eight scenarios keep their queries and their numbers. Every comparison with `testdata/python/` keeps walking exactly the eight.
- **The wall-time grammar is one**: `YYYY-MM-DD HH:MM`, a real calendar date, hours `00`–`23`, minutes `00`–`59`, read in the report's time zone; a wall time that does not exist in the zone, or occurs twice in it, is refused.
- **A range ends at or before the emission time**; the start comes before the end.
- **Diagnostics are English**; no message repeats a value that was not understood (a malformed `--start` is not echoed; a well-formed one that does not exist in the zone is named).
- **Every text the reader sees, and every accessible name, comes from `web/src/locales/{en,pt-BR}.json`.** Task 4 carries every word of this part; an implementer transcribes and never invents one.
- **No inline style**, no `<style>` added at run time; the CSP stays `script-src 'self'` and `style-src 'self'`. The calendar is styled through `classNames` with the theme's tokens, and `@daypicker/react/style.css` is never imported.
- **Names and numbers use the monospaced font**: the From and To fields are mono.
- **Contrast**: every new pair enters `web/src/theme/pairs.ts` at 4.5:1 for text, 3:1 for a border or focus ring, in both themes.
- **Every npm dependency has an exact version**; `npm ci` with no `--force`.
- **Commits:** `type(scope): description`, imperative, under 50 characters; every subject below is counted. Stage with `git add <paths>`.
- Scratch goes under `.cache/phase-4/part-3/`.

## Review Focus

Each line names an input the spec implies and no requirement spells out, and the test that pins it.

1. **"Today so far" at 00:00, or "This month so far" in the first minute of the month.** The range is empty; the form refuses it at sending with `period.range.order` on the period, and sends nothing. Pinned in Task 5 (`an empty so-far range`).
2. **A wall time in a daylight-saving gap or overlap** (`America/New_York`, `2026-03-08 02:30` and `2026-11-01 01:30`). The command exits 2 naming the time and the zone; the API answers 400 `invalid` on that field; the form shows `period.range.wallClock`. Pinned in Task 1 (`TestWallTimeInAGap`, `TestWallTimeInAnOverlap`), Task 3 (`TestRequestRangeInAGap`), Task 5 (`server wall-clock word`).
3. **A browser in another zone than the report's.** At 2026-10-08 01:30 UTC with the report in `America/Sao_Paulo` (22:30 on the 7th there), "Yesterday" is `2026-10-06 00:00` to `2026-10-07 00:00`. Pinned in Task 4 (`resolve in the report's zone`).
4. **An end in the current minute.** "So far" ends at the minute of sending, floored, so it is never after the request time; an end one minute later than the request time is `future`. Pinned in Task 3 (`TestRequestRangeEndingNow`, `TestRequestRangeInTheFuture`).
5. **A range carried through the form.** Changing the time zone keeps the typed walls; "Try again" from a range report brings back a typed range with the same walls and `period` null. Pinned in Task 5 (`zone change keeps the walls`, `from a range report`).

## Where this part sits

| Part | Delivers | Proof at the end |
|---|---|---|
| 4.1 Foundation | Toolchain, serving, theme, i18n, session, sign-in, account | Done (`2f52cd7`) |
| 4.2 Reports | List, form, progress, PDFs, delete | Done (`5090fb7`), released as 0.3.0 |
| **4.3 Calendar ranges (this plan)** | Ranges in the command, the snapshot `2.0`, the API, and the picker | In a real browser: a report of the previous week requested and collected; on the command line, a report of a past day |
| 4.4 The report | The seven tabs | Every tab of a golden scenario in both languages |
| 4.5 Administration | Profiles and users | An administrator creates a profile and a user |
| 4.6 End-to-end | Security, concurrency, PDF recovery specs | `make e2e` runs the four specs of spec 13.4 |

### Out of this part

| Left out | Reason |
|---|---|
| A profile limit on the length of a range | The owner chose no new field (2026-10-07) |
| Showing the period on the list and the progress page | They show none today; the report of 4.4 shows it |
| Calendar names in the command (`--range previous-month`) | The owner chose the form alone; a cron job computes its dates |
| A reader of snapshot `1.0` | The owner's decision: no fallback |

## How to read this plan

As in part 4.2: each task gives exact paths, signatures, and a table of tests with inputs and expected results. Write each test first, watch it fail, then implement.

Executors:
- **Subagent:** a Sonnet subagent. It may run `go`, `make build`, `make test`, `make lint`, `make generate`, `make generate-check`, `make web`, `npm` inside `web/`, and the tagged Go tests against the PostgreSQL the main thread started (`KCR_TEST_DATABASE_DSN`), and the template tests with the native `typst` (`go test -count=1 -tags=template ./internal/cli/`). It never runs Docker, `make e2e`, or `make update-*`, and cannot declare a task finished. `make lint` is the only formatter check; no prettier or other formatter is run.
- **Main thread:** Docker, `make update-testdata`, judgment, wording, design, and the owner.

Rules for every brief:
- Tasks run in sequence. A reviewer reads a diff file and runs tests only on packages nobody edits.
- Stage with `git add <paths>`; never stash, restore, check out a path, or clean; never overwrite a file from git.
- A test of a change goes red first, on the code before the change.
- The harness creates worktrees from `main`: the implementer's first step fast-forwards its worktree to `feat/calendar-ranges`.
- A shell loop left waiting for `go test` is found with `pgrep -fl "until grep"` and stopped.

**Order:** Task 1 → Task 2 (uses `ParseWallTime` and `Analysis.Start`/`End`) → main thread's testdata commit → Task 3 (uses `RunConfig` and the window) → Task 4 (needs `schema.gen.ts` with `start`, `end`) → Task 5 → Task 6 → Task 7 → Task 8.

## Rulings made on the owner's behalf

Each ruling is a decision the amended spec leaves open: decision, grounds, cost if wrong. The numbering continues from R197.

| ID | Ruling |
|---|---|
| R198 | One parser. `internal/config` exports `const WallTimeLayout = "2006-01-02 15:04"`, the sentinels `ErrWallTimeShape`, `ErrWallTimeAbsent`, `ErrWallTimeTwice`, and `func ParseWallTime(text string, loc *time.Location) (time.Time, error)`. It matches `^[0-9]{4}-[0-9]{2}-[0-9]{2} [0-9]{2}:[0-9]{2}$`, builds `time.Date(y, m, d, h, mi, 0, 0, loc)` from the fields, and refuses with `ErrWallTimeShape` when the date is not real (`time.Date` normalised it: the fields of the result in UTC wall terms differ). It then finds every instant of that wall clock: for each offset among the offset at the candidate, the offset just before `ZoneBounds()` start, and the offset at `ZoneBounds()` end, the instant `wall-as-UTC − offset` counts when its wall clock in `loc` equals the input; no instant is `ErrWallTimeAbsent`, two distinct instants `ErrWallTimeTwice`, one is returned. Grounds: spec 5.2 as amended; Go documents that `time.Date` picks an unspecified instant in a gap or overlap. Cost if wrong: one function. |
| R199 | The configuration. `config.Analysis` gains `Start, End string` (wall times; empty when unused). TOML `analysis.start`, `analysis.end`; environment `KCR_START`, `KCR_END`; flags `--start`, `--end` (`config.Flags.Start`, `.End`). The length and the range are one setting: within one source, setting `period` with either of `start`/`end` is refused (`the configuration file sets both analysis.period and a range`; `the environment sets both KCR_PERIOD and KCR_START or KCR_END`; `the flags set both --period and --start or --end`); a source that sets `start` or `end` clears `Period` (the default `24h` included) and a source that sets `period` clears `Start` and `End`. `Validate` then requires exactly one form: `Period` non-empty, or both `Start` and `End` (`analysis.start and analysis.end must be set together`). With a range: each parses in `analysis.timezone` (`analysis.start must look like 2026-09-01 00:00`, never echoing the value; `analysis.start 2026-03-08 02:30 does not exist in America/New_York`; `... occurs twice in ...`; the same for `end`), and the start comes before the end (`analysis.start must come before analysis.end`). `Analysis.Range() (start, end time.Time, err error)` returns the two instants. Every refusal is a configuration error, exit 2. Grounds: spec 5.2 as amended. Cost if wrong: messages only. |
| R200 | The window. `NewWindow(analysis, emittedAt)`: when `analysis.Start != ""`, `Start` and `End` come from `analysis.Range()`, `Step` is `Step(End.Sub(Start))`, and an `End` after `emittedAt.Truncate(time.Second)` is refused by `rangeAfterRunError`, which wraps the new sentinel `ErrRangeAfterRun`: `analysis.end: the range ends at 2026-10-09 00:00, after the run started at 2026-10-07 18:00 in America/Sao_Paulo`. The last N path is untouched. `internal/cli/run.go` maps `ErrRangeAfterRun` to exit 2 at the three places that map `ErrEmptyPeriod`. In the worker, the error takes the path an empty period takes today, unchanged. Grounds: spec 5.2 and 6.4 as amended. Cost if wrong: none found. |
| R201 | The snapshot `2.0`. `snapshot.SchemaVersion = "2.0"`; `Report.Period` leaves the struct, `build.go`, the walker's field set, and `Check`; `periodRe` and `periodTooLong` leave `codec.go`; `Check` requires `period_start` before `period_end` and `period_end` not after `emitted_at`, which replaces their equality (`report.period_end is after report.emitted_at`). Decode's message for `1.0` stands: `snapshot has schema_version "1.0"; this version reads "2.0"`. Grounds: spec 8 as amended; the owner's rule of 2026-10-05. Cost if wrong: none found. |
| R202 | The range scenario. `promtest.Scenario` gains `Fixture string` (the file stem; empty means `Name`), `Start`, and `End`; `Path` and `goldentest.run` read the fixture through it. A new list `promtest.RangeScenarios` holds one entry, `{Name: "direct-complete-range", Fixture: "direct-complete", Folder: "prometheus", APIRoot: "/api/v1", EndpointType: "prometheus", NodeAutoscaling: "auto", Start: "2026-07-13 00:00", End: "2026-07-14 00:00"}`, kept apart from `promtest.Scenarios`, so every comparison with the original walks the eight it walks today. `fixedConfig` sets the range when `Start` is set. Twenty-four hours keeps every query text equal to `direct-complete`'s (the `[86400s]` span included), so the fake's recorded answers match. `make update-testdata` writes `testdata/snapshot/direct-complete-range.json` and `testdata/queries/direct-complete-range.txt`; the golden tests of snapshots and queries walk both lists; no data file and no golden page is added. Grounds: spec 13.2 as amended; the queries file holds path and text alone, so the times are proved from `Seen().Form`. Cost if wrong: one scenario. |
| R203 | The request. `reports.Selection` gains `Start *string \`json:"start"\`` and `End *string \`json:"end"\``; `reports.Resolved` gains `Start string \`json:"start"\``, `End string \`json:"end"\`` (a length has them empty; a range has `Period` empty). `checkSyntax` adds: a shape refused by `config.ParseWallTime(text, time.UTC)` is `invalid` on its field; one end without the other is `missing` on the absent one; a `period` with either is `conflict` on `period`. After `resolve` fills the zone, a new `judgeRange(start, end string, zone *time.Location, now time.Time) map[string]string` answers `invalid` on a field whose wall time is absent or twice in the zone, `order` on `start` when it is not before the end, and `future` on `end` when it is after `now` (the service's clock, one reading per request); its words are `REQUEST_INVALID` (400), judged before the permits. The permit check adds `not_permitted` on `start` when the profile's `permitted_periods` is not empty (422 `REPORT_SELECTION_NOT_PERMITTED`). The digest stays `json.Marshal(Selection)`. Grounds: spec 11.6, 11.7 as amended. Cost if wrong: the words are in the contract. |
| R204 | The worker. `profiles.Document.RunConfig(period, start, end, timezone, reportProfile, minSeverity string, namespaces []string) config.Config` sets `Analysis.Period`, `.Start`, `.End` as given; `worker/collect.go` passes `r.Period, r.Start, r.End`. Grounds: one way to build the window. Cost if wrong: none found. |
| R205 | The contract. `ReportRequest` gains `start` and `end`: `type: string, nullable: true, pattern: "^[0-9]{4}-[0-9]{2}-[0-9]{2} [0-9]{2}:[0-9]{2}$"`, described as wall times in the request's resolved time zone. `ReportSelection.period` becomes `nullable: true`; `start` and `end` join it, required and nullable. The 400 of `POST /reports` lists `missing, invalid, repeated, empty, unknown, conflict, order, future`. `reportOf` maps an empty string to null. `make generate` writes both type files. Grounds: spec 11.7 as amended. Cost if wrong: none found. |
| R206 | The period module. `web/src/features/reports/period.ts` holds `type PeriodValue = { kind: "length"; period: string } \| { kind: "calendar"; name: CalendarRange } \| { kind: "range"; start: string; end: string }`, `calendarRanges = ["today", "yesterday", "thisWeek", "previousWeek", "thisMonth", "previousMonth"] as const`, `parseWallTime(input): string \| null` (trimmed; shape and a real date), `wallNow(zone, now: Date): string` (the zone's wall clock through `Intl.DateTimeFormat("en-CA", { timeZone, hourCycle: "h23", … }).formatToParts`, floored to the minute), `resolveCalendar(name, zone, now): { start: string; end: string }` (date arithmetic on the zone's date with `Date.UTC`; a week starts on Monday; today: `today 00:00`–`now`; yesterday: `today−1 00:00`–`today 00:00`; this week: `monday 00:00`–`now`; previous week: `monday−7 00:00`–`monday 00:00`; this month: `1st 00:00`–`now`; previous month: `1st of the previous month 00:00`–`1st 00:00`), `checkRange(start, end, zone, now): { start?: RangeProblem; end?: RangeProblem }` with `RangeProblem = "invalid" \| "order" \| "future"` (wall strings of this grammar compare as text), `periodText(t, language, value)`, and `periodDays(value)` (a range: the wall difference in days, daylight saving ignored). No date library is imported here. Grounds: spec 12.3 as amended; the walls are the API's. Cost if wrong: one module. |
| R207 | The form. `values.period` is a `PeriodValue`, starting at `{ kind: "length", period: profile.default_period }`. The body of `POST /reports` is `{ period, start: null, end: null }` for a length, and `{ period: null, start, end }` for a typed range or for a calendar range resolved by `resolveCalendar(name, values.timezone, new Date())` at sending; an empty resolved range (Review Focus 1) is refused with `period.range.order` and nothing is sent. `?from=<id>` reads `selection.period` (a length) or `selection.start`/`end` (a typed range). The server's fields `start` and `end` mark the period: `invalid` → `period.range.wallClock` with the zone, `order` → `period.range.order`, `future` → `period.range.future`, `not_permitted` → `validation.notPermitted`; `missing` and `conflict` fall back to the code's alert. The 15-day warning reads `periodDays`. Grounds: spec 12.3 as amended. Cost if wrong: none found. |
| R208 | The picker. Not restricted: the left column starts with `Segmented` (`period.mode.length`, `period.mode.range`), on Range when the value is a range or a calendar name, and on Length otherwise; Length is today's custom field. Range: two `Field`s (mono, labels `period.range.from`/`.to`), each followed by an `IconButton` (`CalendarBlankIcon`, names `period.range.pickFrom`/`.pickTo`, `aria-expanded`) that opens a `DayPicker` (`mode="single"`, `weekStartsOn={1}`, `locale` `enUS` or `ptBR` from `@daypicker/react/locale`, `labels` `labelPrevious`/`labelNext`/`labelNav` from `period.range.previous`/`.next`/`.months`, `classNames` on the theme's tokens) under that field; one calendar is open at a time; a day picked replaces the date of its field and keeps its time, then closes the calendar and returns focus to its button; Esc does the same. The hint `period.range.hint` names the zone. The fields open on the current range, a calendar name resolved now, or yesterday. Apply runs `checkRange`; a problem shows under its field and keeps the picker open. The right column lists the six lengths, a rule, and the six calendar names (`aria-pressed` on the current one). Restricted: today's list and `period.restricted`, no Segmented, no calendar names. The trigger shows `periodText`. Grounds: option 1A and `section-3-form.html`, chosen by the owner. Cost if wrong: one component. |
| R209 | The words, as the owner approved them in `section-3-form.html`, plus the calendar's three accessible names; `period.custom.title` leaves; `period.calendar.*` joins `families.ts`. The table is in Task 4. Grounds: the companion page of 2026-10-07. Cost if wrong: wording only. |
| R210 | `@daypicker/react` 10.0.2, exact, in `dependencies`; the lock file committed. Its day buttons and grid take their names from the `date-fns` locale (`enUS`, `ptBR`), as dates formatted by `Intl` do; its three navigation names come from the locale files. `no-style.test.tsx` opens the calendar and finds no `style` attribute and no added `<style>`; if the library writes one that `classNames` cannot remove, the task stops and reports. Grounds: spec 12.5; R189. Cost if wrong: a different calendar. |
| R211 | The reports spec. `web/e2e/reports.spec.ts` requests the report with "Previous week" in place of the default period, and checks through `GET /reports/{id}` (fetched by the page) that `selection.period` is null and `selection.start`/`end` are the two Mondays at `00:00` in the profile's zone, computed in the spec from `Date.now()` with the same rule. The rest of the spec is unchanged. Grounds: the fake serves one collection per run; the last N stays covered by every unit test. Cost if wrong: one spec step. |
| R212 | Version 0.4.0 is proposed at the close: a new feature and a changed snapshot. The changelog says that a report stored before 0.4.0 can no longer be opened or given a new PDF, and is requested again. Grounds: the owner's decision of 2026-10-07. Cost if wrong: the owner's call at the close. |

## Findings this plan closes

| Finding | How |
|---|---|
| F-131 | Tasks 1 to 6 |

## File structure

| Path | Responsibility |
|---|---|
| `internal/config/{config.go,load.go,validate.go,walltime.go}` and tests; `internal/cli/report.go` | The wall time, the range in the configuration, the flags (Task 1) |
| `internal/collect/window.go` and tests; `internal/snapshot/{snapshot.go,build.go,codec.go}` and tests; `internal/cli/run.go`; `internal/promtest/scenarios.go`; `internal/goldentest/*`; `cmd/update-testdata/main.go` | The window of a range, the snapshot `2.0`, the range scenario (Task 2) |
| `testdata/snapshot/*.json`, `testdata/queries/direct-complete-range.txt` | Written by `make update-testdata` (main thread, after Task 2) |
| `internal/platform/reports/{service.go,request.go}` and tests; `internal/platform/profiles/document.go` and tests; `internal/platform/worker/collect.go` and tests; `internal/platform/api/reports.go` and tests; `api/openapi.yaml`; generated files | The request, the worker, the contract (Task 3) |
| `web/package.json`, `web/package-lock.json`; `web/src/features/reports/period.ts` and test; `web/src/locales/*.json`; `web/src/i18n/families.ts`; `web/src/theme/pairs.ts` | The dependency, the period module, every word (Task 4) |
| `web/src/features/reports/{period-picker.tsx,generate-page.tsx}` and tests; `web/src/components/no-style.test.tsx`; `web/tests/msw.ts` | The picker and the form (Task 5) |
| `web/e2e/reports.spec.ts` | The browser (Task 6) |
| `AGENTS.md`, `README.md`, `CHANGELOG.md` | The documents (Task 8) |

---

### Task 0: Open the part

**Executor:** Main thread.

- [x] **Step 1:** `ROADMAP.md` and `CHANGELOG.md` read and true; `main` at `f391d26`, pushed, tag `v0.3.0`.
- [x] **Step 2:** `git switch -c feat/calendar-ranges`.
- [x] **Step 3:** The spec amendment, `693ae1e`, approved by the owner on 2026-10-07.
- [ ] **Step 4:** `ROADMAP.md`: P43-1 to P43-8 `OPEN`, the rulings R198 to R212 named; the plan's commit named in P43-0.

---

### Task 1: The wall time and the range in the configuration (R198, R199)

**Executor:** Subagent.

**Files:**
- Create: `internal/config/walltime.go`, `internal/config/walltime_test.go`
- Modify: `internal/config/config.go` (`Analysis`, `Flags`), `internal/config/load.go` (`sectionOrder`/known keys near line 26, `tomlConfig`, `fromConfig`, `toConfig`, `applyFile`, `applyEnvironment`, `applyFlags`), `internal/config/validate.go` (`validateAnalysis`), `internal/cli/report.go` (the flags near line 76 and the help)
- Test: `internal/config/load_test.go`, `internal/config/validate_test.go`, `internal/cli/report_test.go` (or the file that tests the flags)

**Interfaces:**

```go
// WallTimeLayout is the layout of a wall time of spec 5.2.
const WallTimeLayout = "2006-01-02 15:04"

var (
	ErrWallTimeShape  = errors.New("a wall time must look like 2026-09-01 00:00")
	ErrWallTimeAbsent = errors.New("the wall time does not exist in the zone")
	ErrWallTimeTwice  = errors.New("the wall time occurs twice in the zone")
)

// ParseWallTime reads text as a wall time in loc and returns its one instant.
func ParseWallTime(text string, loc *time.Location) (time.Time, error)

// Range returns the instants of analysis.start and analysis.end.
func (a Analysis) Range() (start, end time.Time, err error)
```

- Produces: `config.Analysis.Start`, `.End`; `config.Flags.Start`, `.End`; the three sentinels.

- [ ] **Step 1: Write the failing tests**

| Test | Input | Expected |
|---|---|---|
| `TestWallTimeShape` | `"2026-09-01 00:00"`, `" 2026-09-01 00:00"`, `"2026-9-01 00:00"`, `"2026-09-01T00:00"`, `"2026-09-01 24:00"`, `"2026-02-30 00:00"`, `"2026-09-01 00:00:00"`, `""` in `UTC` | the first parses to `2026-09-01T00:00:00Z`; every other is `ErrWallTimeShape` |
| `TestWallTimeInAZone` | `"2026-09-01 00:00"` in `America/Sao_Paulo` | `2026-09-01T03:00:00Z` |
| `TestWallTimeInAGap` (Review Focus 2) | `"2026-03-08 02:30"` in `America/New_York` | `ErrWallTimeAbsent` |
| `TestWallTimeInAnOverlap` (Review Focus 2) | `"2026-11-01 01:30"` in `America/New_York` | `ErrWallTimeTwice`; `"2026-11-01 00:59"` and `"2026-11-01 02:00"` parse |
| `TestRangeFromTheFile` | `[analysis] start = "2026-09-01 00:00"`, `end = "2026-10-01 00:00"` | `Period == ""`, `Start`, `End` as written |
| `TestRangeReplacesALowerPeriod` | file `period = "7d"`; flags `--start`, `--end` | `Period == ""`, the range of the flags |
| `TestPeriodReplacesALowerRange` | file `start`/`end`; `KCR_PERIOD=36h` | `Period == "36h"`, `Start == ""`, `End == ""` |
| `TestOneSourceSetsBoth` | in the file, in the environment, in the flags: a period and a start | each refused with its sentence of R199 |
| `TestRangeNeedsBothEnds` | only `--start` | `analysis.start and analysis.end must be set together` |
| `TestRangeOrder` | start `2026-10-01 00:00`, end `2026-09-01 00:00`; then start equal to end | `analysis.start must come before analysis.end` both times |
| `TestRangeShapeIsNotEchoed` | `--start "tomorrow"` | `analysis.start must look like 2026-09-01 00:00`; the message holds no `tomorrow` |
| `TestRangeNamesItsZone` | `--timezone America/New_York --start "2026-03-08 02:30" --end "2026-03-09 00:00"` | `analysis.start 2026-03-08 02:30 does not exist in America/New_York` |
| `TestReportCommandRefusesARange` (`cli`) | `report --start x --end y` with a valid file otherwise | exit 2, one line `error: analysis.start must look like 2026-09-01 00:00` |
| existing config tests | — | pass unchanged; the default is still `Period == "24h"` |

- [ ] **Step 2:** `go test -count=1 ./internal/config/ ./internal/cli/ -run 'WallTime|Range|OneSource|Period'`. Expected: FAIL to compile, then each for its reason.
- [ ] **Step 3:** Implement R198 and R199.
- [ ] **Step 4:** `make test`, `make lint`. All pass.
- [ ] **Step 5:** Commit `feat(config): read a calendar range` (35 characters).

**Done when:** every row passes and `make test` is green.

---

### Task 2: The window of a range, the snapshot 2.0, the range scenario (R200, R201, R202)

**Executor:** Subagent; then the main thread writes the testdata.

**Files:**
- Modify: `internal/collect/window.go`, `internal/snapshot/snapshot.go` (`SchemaVersion`, `Report`), `internal/snapshot/build.go` (near line 56), `internal/snapshot/codec.go` (lines 25-57, 495-499, 547-551, the field table near 685), `internal/cli/run.go` (near lines 475, 507, 552), `internal/promtest/scenarios.go`, `internal/goldentest/goldentest.go` (`fixedConfig`, `run`), `internal/goldentest/golden_test.go` (the snapshot and query goldens walk `promtest.Scenarios` and `promtest.RangeScenarios`), `cmd/update-testdata/main.go` (writes both lists' snapshots and queries)
- Test: `internal/collect/window_test.go`, `internal/snapshot/codec_test.go`, `internal/snapshot/build_test.go`, `internal/goldentest/range_test.go` (new), `internal/cli/run_test.go`; every test that builds a `snapshot.Report` with `Period` loses it

**Interfaces:**
- Consumes: `config.Analysis.Range()`, `config.Analysis.Start`.
- Produces: `collect.ErrRangeAfterRun`; `snapshot.SchemaVersion == "2.0"`; `promtest.Scenario.Fixture`, `.Start`, `.End`; `promtest.RangeScenarios`.

- [ ] **Step 1: Write the failing tests**

| Test | Input | Expected |
|---|---|---|
| `TestWindowOfARange` | start `2026-07-13 00:00`, end `2026-07-14 00:00`, `America/Sao_Paulo`, emitted `2026-07-15 14:30` | `Start` `2026-07-13T03:00:00Z`, `End` `2026-07-14T03:00:00Z`, `Step` `5m` |
| `TestWindowStepFollowsTheRange` | ranges of 30 minutes, 6 hours, 7 days, 31 days | `30s`, `2m`, `15m`, `1h` |
| `TestWindowRangeEndingAtTheRun` | end equal to the emission, cut to the second | accepted |
| `TestWindowRangeAfterTheRun` | end one minute after the emission | `errors.Is(err, ErrRangeAfterRun)`; the message names the end, the emission, and the zone |
| `TestWindowOfTheLastN` (existing) | `24h` | unchanged |
| `TestRunRefusesARangeAfterTheRun` (`cli`) | a range ending after the fixed clock | exit 2 before any request (the fake saw no request) |
| `TestSnapshotHasNoPeriod` | a built snapshot | encoded JSON holds `"schema_version":"2.0"` and no key `period` under `report` |
| `TestSnapshotEndMayPrecedeTheEmission` | `period_end` one day before `emitted_at` | decodes |
| `TestSnapshotEndAfterTheEmission` | `period_end` one second after `emitted_at` | `report.period_end is after report.emitted_at` |
| `TestSnapshotNamesVersionOne` | a golden snapshot with `"schema_version":"1.0"` | `snapshot has schema_version "1.0"; this version reads "2.0"` |
| `TestSnapshotRefusesAPeriodKey` | a `2.0` snapshot that still holds `report.period` | refused by the walk as an unknown key |
| `TestRangeScenarioSendsItsTimes` (`goldentest`) | `direct-complete-range` | every `/query_range` the fake saw has `start=1783911600`, `end=1783998000`, `step=5m`; every `/query` has `time=1783998000` (the preflight's included) |
| `TestRangeScenarioEqualsItsBase` (`goldentest`) | the snapshots of `direct-complete-range` and `direct-complete` | equal in every field but `report.period_start` and `report.period_end`; the query lists equal line for line |
| `TestOriginalComparisonsWalkEight` (`goldentest`) | `len(promtest.Scenarios)` | 8, and no entry of it has `Start` set |

The numbers in `TestRangeScenarioSendsItsTimes` are the Unix seconds of `2026-07-13 00:00` and `2026-07-14 00:00` in `America/Sao_Paulo` (UTC−3), checked when this plan was written.

- [ ] **Step 2:** Run the new tests; expected FAIL.
- [ ] **Step 3:** Implement R200 to R202. Leave `testdata/` alone: the golden tests of snapshots fail until the main thread writes them; report which ones fail and why.
- [ ] **Step 4:** `make lint`; `go test ./internal/collect/ ./internal/snapshot/ ./internal/config/ ./internal/cli/` pass; `./internal/goldentest/` fails only on the golden snapshot comparisons.
- [ ] **Step 5:** Commit `feat(snapshot): cover a range that ended` (40 characters).
- [ ] **Step 6 (main thread):** `make update-testdata`. Read every changed file whole: each of the eight snapshots changes in `schema_version` and in the loss of `report.period` alone; `direct-complete-range.json` and `.txt` are new; nothing under `template/tests/` changes (`git status --short template/` is empty). `make test`, then `go test -count=1 -tags=template ./template/ ./internal/typst/ ./internal/cli/`. Commit `test(testdata): write the 2.0 snapshots` (39 characters).

**Done when:** every row passes, the eight snapshot diffs are the two lines each, and `make test` is green.

---

### Task 3: The request, the worker, the contract (R203, R204, R205)

**Executor:** Subagent, against the running PostgreSQL.

**Files:**
- Modify: `internal/platform/reports/service.go` (`Selection`, `Resolved`), `internal/platform/reports/request.go` (`checkSyntax`, `resolve`, the new `judgeRange`, the word constants), `internal/platform/profiles/document.go` (`RunConfig`), `internal/platform/worker/collect.go` (near line 215), `internal/platform/api/reports.go` (`reportOf`), `api/openapi.yaml` (`ReportSelection` near 1082, `ReportRequest` near 1102, the 400 of `POST /reports` near 478)
- Regenerate: `internal/platform/api/openapi.gen.go`, `web/src/api/schema.gen.ts`
- Test: `internal/platform/reports/request_test.go`, `internal/platform/profiles/document_test.go`, `internal/platform/worker/collect_test.go`, `internal/platform/api/reports_test.go`

**Interfaces:**
- Consumes: `config.ParseWallTime`, `config.Analysis.Start/End`, the window of Task 2.
- Produces: `Selection.Start/End *string`, `Resolved.Start/End string`; `RunConfig(period, start, end, timezone, reportProfile, minSeverity string, namespaces []string)`; `ReportRequest.start/end`, `ReportSelection.start/end` (nullable), `ReportSelection.period` nullable, in `schema.gen.ts`.

- [ ] **Step 1: Write the failing tests**

| Test | Input | Expected |
|---|---|---|
| `TestRequestStoresARange` | `start "2026-09-01 00:00"`, `end "2026-10-01 00:00"`, zone default `America/Sao_Paulo`, clock 2026-10-07 | accepted; `Resolved{Period: "", Start, End}` as sent; `GET` shows `period` null |
| `TestRequestRangeShape` | `start "2026-9-1 0:00"` | 400 `REQUEST_INVALID`, `{"start": "invalid"}` |
| `TestRequestRangeNeedsBothEnds` | `start` alone | 400, `{"end": "missing"}` |
| `TestRequestRangeWithAPeriod` | `period "7d"` and a range | 400, `{"period": "conflict"}` |
| `TestRequestRangeOrder` | start after end; start equal to end | 400, `{"start": "order"}` both times |
| `TestRequestRangeInAGap` (Review Focus 2) | `timezone "America/New_York"`, `start "2026-03-08 02:30"` | 400, `{"start": "invalid"}` |
| `TestRequestRangeEndingNow` (Review Focus 4) | clock `2026-10-07 18:00:42` in the zone; `end "2026-10-07 18:00"` | accepted |
| `TestRequestRangeInTheFuture` (Review Focus 4) | same clock; `end "2026-10-07 18:01"` | 400, `{"end": "future"}` |
| `TestRequestRangeUnderPermittedPeriods` | profile `permitted_periods ["24h"]`, a valid range | 422 `REPORT_SELECTION_NOT_PERMITTED`, `{"start": "not_permitted"}` |
| `TestRequestRangeWordsBeforePermits` | the same profile, start after end | 400 `{"start": "order"}` |
| `TestKeyReusedWithAnotherRange` | one key, then the same body with another end | `IDEMPOTENCY_KEY_REUSED` |
| `TestRunConfigCarriesARange` (`profiles`) | `RunConfig("", "2026-09-01 00:00", "2026-10-01 00:00", "UTC", …)` | `Analysis{Period: "", Start, End, …}` |
| `TestCollectARange` (`worker`, `postgres template`) | a report with a range of 24 hours ending one day before the worker's clock, over `direct-complete` | collected; the stored snapshot's `period_start`/`period_end` are the range's; `emitted_at` is the worker's clock |
| `TestPostReportWithARange` (`api`) | `POST /reports` with `start`/`end` | 202; `GET /reports/{id}` has `"period": null, "start": …, "end": …` |
| `TestPostReportWithALength` (`api`) | `POST /reports` with `period "7d"` | `GET` has `"period": "7d", "start": null, "end": null` |
| `TestRoutesMatchTheContract` (existing) | — | passes both ways |
| `TestViewMatchesTheContract` (existing) | the golden snapshots `2.0` | passes |

- [ ] **Step 2:** `go test -count=1 -tags=postgres ./internal/platform/reports/ ./internal/platform/profiles/ ./internal/platform/api/ -run 'Range|RunConfig|WithA'` and `-tags='postgres template' ./internal/platform/worker/ -run Range`. Expected: FAIL.
- [ ] **Step 3:** Implement; `make generate`.
- [ ] **Step 4:** `make test`, `make lint`, `make generate-check`, `go test -count=1 -tags=postgres ./internal/platform/...` (with `-tags='postgres template'` for `./internal/platform/worker ./internal/cli`). All pass.
- [ ] **Step 5:** Commit `feat(reports): request a calendar range` (39 characters).

**Done when:** every row passes and `make generate-check` is clean.

---

### Task 4: The dependency, the period module, every word (R206, R209, R210)

**Executor:** Subagent. Follows Task 3.

**Files:**
- Modify: `web/package.json`, `web/package-lock.json` (`npm install --save-exact @daypicker/react@10.0.2`), `web/src/features/reports/period.ts`, `web/src/locales/{en,pt-BR}.json`, `web/src/i18n/families.ts`
- Test: `web/src/features/reports/period.test.ts`

**Interfaces:** as R206.

```ts
export const calendarRanges = ["today", "yesterday", "thisWeek", "previousWeek", "thisMonth", "previousMonth"] as const;
export type CalendarRange = (typeof calendarRanges)[number];
export type PeriodValue =
  | { kind: "length"; period: string }
  | { kind: "calendar"; name: CalendarRange }
  | { kind: "range"; start: string; end: string };
export type RangeProblem = "invalid" | "order" | "future";
export function parseWallTime(input: string): string | null;
export function wallNow(zone: string, now: Date): string;
export function resolveCalendar(name: CalendarRange, zone: string, now: Date): { start: string; end: string };
export function checkRange(start: string, end: string, zone: string, now: Date): { start?: RangeProblem; end?: RangeProblem };
export function periodText(t: TFunction, language: Language, value: PeriodValue): string;
export function periodDays(value: PeriodValue): number | null;
```

**The words** (both files; keys under `period`):

| Key | English | Português (Brasil) |
|---|---|---|
| `period.mode.length` | Length | Duração |
| `period.mode.range` | Range | Intervalo |
| `period.range.from` | From | De |
| `period.range.to` | To | Até |
| `period.range.value` | {{start}} to {{end}} | {{start}} a {{end}} |
| `period.range.hint` | Year-month-day and hour:minute, in the report's time zone, {{zone}}. | Ano-mês-dia e hora:minuto, no fuso horário do relatório, {{zone}}. |
| `period.range.pickFrom` | Pick the start day | Escolher o dia de início |
| `period.range.pickTo` | Pick the end day | Escolher o dia de fim |
| `period.range.previous` | Show the previous month | Mostrar o mês anterior |
| `period.range.next` | Show the next month | Mostrar o próximo mês |
| `period.range.months` | Months | Meses |
| `period.range.invalid` | Write a date and time such as 2026-09-01 00:00. | Escreva uma data e hora como 2026-09-01 00:00. |
| `period.range.order` | The start must come before the end. | O início deve vir antes do fim. |
| `period.range.future` | The end cannot be after now. | O fim não pode ser depois de agora. |
| `period.range.wallClock` | This time does not exist, or occurs twice, in {{zone}}. | Este horário não existe, ou ocorre duas vezes, em {{zone}}. |
| `period.calendar.today` | Today so far | Hoje até agora |
| `period.calendar.yesterday` | Yesterday | Ontem |
| `period.calendar.thisWeek` | This week so far | Esta semana até agora |
| `period.calendar.previousWeek` | Previous week | Semana anterior |
| `period.calendar.thisMonth` | This month so far | Este mês até agora |
| `period.calendar.previousMonth` | Previous month | Mês anterior |

`period.custom.title` leaves both files. A key not yet used by a screen is used by Task 5; the test "every key is used" runs green at the end of Task 5, and this task's report says which keys wait for it.

- [ ] **Step 1: Write the failing tests** (`period.test.ts`, a fixed `now`)

| Test | Input | Expected |
|---|---|---|
| `wall time grammar` | the inputs of `TestWallTimeShape` | the same verdicts; `" 2026-09-01 00:00 "` trims |
| `wall now floors the minute` | `2026-10-07T21:00:42Z`, `America/Sao_Paulo` | `"2026-10-07 18:00"` |
| `each calendar range` | `now` Wednesday `2026-10-07T21:00:42Z`, `America/Sao_Paulo` | today `2026-10-07 00:00`–`2026-10-07 18:00`; yesterday `2026-10-06 00:00`–`2026-10-07 00:00`; this week `2026-10-05 00:00`–`2026-10-07 18:00`; previous week `2026-09-28 00:00`–`2026-10-05 00:00`; this month `2026-10-01 00:00`–`2026-10-07 18:00`; previous month `2026-09-01 00:00`–`2026-10-01 00:00` |
| `a Monday and a Sunday` | `now` Monday `2026-10-05 12:00` and Sunday `2026-10-11 12:00` in the zone | this week starts `2026-10-05 00:00` both times; previous week `2026-09-28`–`2026-10-05` both times |
| `year boundary` | `now` `2026-01-03 10:00` (a Saturday) | previous month `2025-12-01 00:00`–`2026-01-01 00:00`; previous week `2025-12-22 00:00`–`2025-12-29 00:00` |
| `resolve in the report's zone` (Review Focus 3) | `now` `2026-10-08T01:30:00Z`, zone `America/Sao_Paulo`, the test's process in `UTC` | yesterday `2026-10-06 00:00`–`2026-10-07 00:00` |
| `check a range` | order, equal ends, an end one minute after `wallNow`, a bad shape in each field | `{start: "order"}`, `{start: "order"}`, `{end: "future"}`, `{start: "invalid"}` / `{end: "invalid"}` |
| `period text` | a quick length, a typed length, a calendar name, a range, in both languages | the quick key; `period.custom.value`; `period.calendar.<name>`; `2026-09-01 00:00 to 2026-10-01 00:00` / `… a …` |
| `period days` | `30d`; range `2026-09-01 00:00`–`2026-10-01 00:00`; `previousMonth` resolved | 30; 30; 30 |
| locale tests (existing) | both files | same keys, same parameters; the banned terms absent |

- [ ] **Step 2:** `cd web && npx vitest run src/features/reports/period.test.ts`. Expected: FAIL.
- [ ] **Step 3:** Install the dependency; implement; add the words.
- [ ] **Step 4:** `make lint`; `cd web && npx vitest run` passes except "every key is used", whose missing keys are listed.
- [ ] **Step 5:** Commit `feat(web): add the calendar range words` (39 characters).

---

### Task 5: The picker and the form (R207, R208, R210)

**Executor:** Subagent. Follows Task 4.

**Files:**
- Modify: `web/src/features/reports/period-picker.tsx`, `web/src/features/reports/generate-page.tsx`, `web/src/components/no-style.test.tsx`, `web/src/theme/pairs.ts` (the calendar's selected day `accent-fg` on `accent`, its in-range day `info-surface-fg` on `info-surface`, if not listed), `web/tests/msw.ts` (the selection's `start`/`end`)
- Test: `web/src/features/reports/period-picker.test.tsx`, `web/src/features/reports/generate-page.test.tsx`, `web/tests/screens.test.tsx` (the picker open on Range)

**What it shows:** `range-picker.html` option 1A and `section-3-form.html`, with R208's behaviour.

- [ ] **Step 1: Write the failing tests**

| Test | Input | Expected |
|---|---|---|
| `opens on the value` | value a range; then a calendar name; then a length | Range with the walls; Range with the name resolved now; Length |
| `typed range applies` | type `2026-09-01 00:00` and `2026-10-01 00:00`, Apply | `onChange({kind: "range", …})`; the trigger reads `2026-09-01 00:00 to 2026-10-01 00:00` |
| `range problems` | each problem of `check a range` | its sentence under its field; Apply keeps the picker open |
| `calendar picks a day` | open From's calendar, pick the 15th | From becomes `2026-09-15 00:00` (time kept); the calendar closes; focus on its button |
| `one calendar at a time` | open From's, then To's | only To's is open |
| `calendar starts on Monday` | open a calendar in pt-BR | the first column header is Monday's; the month name is Portuguese; the navigation names are `period.range.previous`/`.next` |
| `calendar name applies` | click "Previous month" | `onChange({kind: "calendar", name: "previousMonth"})`; the trigger reads the name |
| `restricted` | `permitted ["24h","7d"]` | the two lengths and `period.restricted`; no Segmented, no calendar names |
| `sends a calendar range resolved` | fake timers at `2026-10-07T21:00:42Z`, "Previous week", send | body `{period: null, start: "2026-09-28 00:00", end: "2026-10-05 00:00"}` |
| `sends a length` | "Last 7 days", send | body `{period: "7d", start: null, end: null}` |
| `an empty so-far range` (Review Focus 1) | fake timers at `2026-10-07T03:00:20Z` (00:00 in the zone), "Today so far", send | `period.range.order` on the period; no request |
| `server wall-clock word` (Review Focus 2) | 400 `{"start":"invalid"}` | `period.range.wallClock` with the zone, under the period; the options open |
| `server future word` | 400 `{"end":"future"}` | `period.range.future` |
| `range not permitted` | 422 `{"start":"not_permitted"}` | `validation.notPermitted` |
| `zone change keeps the walls` (Review Focus 5) | a typed range, then another zone, send | the same walls, the new zone |
| `from a range report` (Review Focus 5) | `/generate?from=<id>` whose selection has `period: null`, `start`, `end` | a typed range with those walls |
| `long range` | a range of 16 days; of 15 days | `generate.longPeriod` shows; does not |
| `no inline style` (`no-style.test.tsx`) | open the picker on Range and open a calendar | no `style` attribute in the document; no `<style>` added |
| `every key is used` (existing) | — | passes |
| screens (existing, new entry) | the picker open on Range with a calendar, both languages and themes | passes |

- [ ] **Step 2:** Run; expected FAIL.
- [ ] **Step 3:** Implement R207 and R208.
- [ ] **Step 4:** `make lint`, `make test`. All pass.
- [ ] **Step 5:** Commit `feat(web): pick a calendar range` (32 characters).

---

### Task 6: The browser (R211)

**Executor:** Subagent writes the change; the main thread runs `make e2e`.

**Files:** Modify `web/e2e/reports.spec.ts` (step 2 of the spec: open the options, the period picker, "Previous week"; after the request, the check of R211).

- [ ] **Step 1 (subagent):** Write the change; `make lint` passes.
- [ ] **Step 2 (main thread):** `orb start`; `docker context show` prints `orbstack`; `make e2e`, both specs pass. Prove the new check: a scratch edit that sends the last N in place of the range fails it (log under `.cache/phase-4/part-3/e2e/`), reverted.
- [ ] **Step 3:** Commit `test(e2e): request a calendar range` (35 characters).

**Done when:** `make e2e` passes twice in a row from a clean stack.

---

### Task 7: The combined run, the command, and the owner's look

**Executor:** Main thread.

- [ ] **Step 1:** `make test`, `make lint`, `make generate-check`, `make test-postgres`, `make image`, `make check-runtime`, `make e2e` over the combined branch; logs under `.cache/phase-4/part-3/combined/`.
- [ ] **Step 2:** The command by hand against the fake Prometheus: `report --start "2026-07-13 00:00" --end "2026-07-14 00:00"` writes a PDF whose cover reads that range; `--start` in a gap exits 2 with its line; a range ending after now exits 2. Logs under `.cache/phase-4/part-3/command/`.
- [ ] **Step 3:** Screenshots of the real build (the picker on Length, on Range with a calendar open, a range error, the restricted list, the options line with a range and with a calendar name; light and dark; en and pt-BR) on the companion page, opened for the owner, beside the mockups they approved. A change the owner asks for is made and the screenshots taken again.

**Done when:** every command passes and the owner has seen the screens.

---

### Task 8: The documents, review, gate, close

**Executor:** Main thread.

- [ ] **Step 1: `AGENTS.md`.** "The command": the period is judged before the first request, the range included (`ErrRangeAfterRun`, exit 2). "The snapshot file has one reading": `2.0`, `1.0` named and refused. "The platform": a request names a period or a range, with its words. "The interface": the picker's three kinds of value, calendar ranges resolved at sending, weeks from Monday. "Golden snapshots and data files": `direct-complete-range` and `promtest.RangeScenarios`, outside every comparison with the original.
- [ ] **Step 2: `README.md`.** The configuration table gains `analysis.start` and `analysis.end`; the usage gains the range; the API section names `start` and `end`.
- [ ] **Step 3: `CHANGELOG.md`, under Unreleased.**
  - `### Added`: a report can cover a calendar range that ended in the past, typed or picked on a calendar in the form, or given with `--start` and `--end` on the command line; the form offers today so far, yesterday, this week so far, previous week, this month so far, and previous month.
  - `### Changed`: a report stored before this version can no longer be opened or given a new PDF; request it again.
  - Omitted, and said so: the range scenario and its golden files, the browser spec.
- [ ] **Step 4: The adversarial review of the branch, by areas**, probes under `.cache/phase-4/part-3/review/probe/`: (a) the wall time, the configuration, the window, and the snapshot; (b) the request, the worker, and the contract; (c) the period module, the picker, and the form; (d) wording, contrast, accessibility, no inline style. One fix wave, then one re-review.
- [ ] **Step 5:** `make gate` with logs under `.cache/phase-4/part-3/gate/`.
- [ ] **Step 6: Close.** `ROADMAP.md` (P43 items, F-131); the closing report `.cache/phase-4/part-3/closing-report.md`; the owner's word on the merge, the push, and version 0.4.0 (R212).

**Done when:** the gate passes from images built without cache, the review is clean, and the owner has the closing report.
