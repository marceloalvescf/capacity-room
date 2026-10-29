# Phase 17: the home page

Date: 2026-10-21. Status: approved by the owner on 2026-10-21. Phase 15 (the public release) stays paused; the order of the two
is the owner's call.

## Purpose

A signed-in reader lands on the Reports list, a log of requests, newest
first. To learn which cluster needs attention they must read the list row by
row and open each report. The home page answers that first: every cluster by
its latest report, and the chosen cluster's answer below, in the words and
numbers of the report itself.

The phase adds a page, a navigation entry, and one read route to the API. It
ends as a minor version, 2.8.0 (the owner's decision of 2026-10-21, which
withdrew an earlier 3.0.0). Nothing of the public surface of spec 15.1
breaks: an upgrade from 2.7.0 keeps every stored row, route, flag, and key.

## Decisions of the owner (2026-10-21)

### H1. The structure: Cluster dossier

Picked on the decision page from three structures in the app's own look
(sketches `.impeccable/mocks/decision/home/`: `dossier.html` chosen,
`lanes.html` and `board.html` not). Inspiration came from SRE portal home
pages: Grafana 13.2 (recent and starred, firing alerts at a glance), Port's
personal widgets, Backstage Home, Datadog's triggered monitors, Headlamp's
cluster overview.

### H2. The brief, agreed as written

- `/` serves Home instead of redirecting to `/reports`; Home is the first
  entry of the side navigation; the Reports page does not change.
- A strip of chips, one per active profile: the status mark of its latest
  report, the profile's display name, and the age of that report.
- Below the strip, the chosen cluster's latest report answered with the
  overview's own parts.
- The chosen cluster lives in the address, `?cluster=<profile id>`.
- The worst severity sorts first.
- New report repeats the latest request through `/generate?from=<id>`, or
  opens the form on that profile through `/generate?profile=<id>` when the
  cluster has no report.
- A new route, `GET /home`, gives each active profile its reports.
- A chip whose latest report changed since the reader last looked carries a
  mark in the accent.

## What the page shows

### The title row

The headline "Home", and New report at the right for an operator and an
administrator, as on the Reports page. A reader sees no New report.

### The cluster strip

A `label` heading, "Clusters, by their latest report", then one chip per
active profile. A chip is a link to `?cluster=<profile id>` (written with
replace, as every state of a tab), and the chosen one carries
`aria-current="true"`. Its parts:

1. The status mark (`StatusMark`, shape then colour), the word for screen
   readers.
2. The profile's `display_name` in the body face, with a report or
   without. Two profiles can collect the same cluster, so the cluster's own
   name (`cluster_name`) would give two chips one name; it stays in the
   answer's meta line.
3. A muted line: the age of the latest collected report ("2 h ago"), or the
   state of the newest request when one is in flight ("Collecting",
   "Queued", "Rendering the PDF"), or "No report yet".

The chosen chip takes the pressed look of a severity toggle: Panel fill and
an Ink outline. The others sit on the Ground with a Hairline outline and take
Hover under the pointer. The chips wrap to as many rows as they need; the
strip never scrolls sideways.

**The mark of a chip:**

| The profile's reports | Mark | Muted line |
|---|---|---|
| A collected report that has not expired | its `worst_severity` | its age |
| The same, with `worst_severity` null (collected before 1.6.0) | unavailable | its age |
| None collected and live, but an expired one | unavailable | "Expired" |
| None at all | no base (open circle) | "No report yet" |

A request in flight (queued, running, or a pending PDF) replaces the muted
line with its step, and the mark stays that of the latest collected report.
When the newest request failed and the profile has no collected report that
is still live, the muted line says the request failed (the Reports list's
word); with a live one, the line keeps its age and the answer carries the
failure (builder's call).

**The order:** critical, attention, healthy, unavailable, no base; ties by
display name.

**The unseen mark:** a 6 px dot in the accent at the chip's top right. A
visually hidden phrase, "New report since your last visit", is added to the
chip's accessible name. It means that the latest collected report of the
profile is not the one the reader last saw on this page.

- The page stores, per browser and per signed-in user (the key holds the
  user id, so two accounts on one browser keep their own), a map from
  profile id to the report id last shown in the answer. It uses
  `localStorage`, wrapped in try and catch, like the stored language
  (builder's call).
- A profile with no entry is recorded silently and gets no dot, so a first
  visit is not all dots.
- Showing a cluster's answer records its report id.
- When storage is refused, no dot is ever drawn.

The brief said a ring. A dot is used because the focus ring is already a
2 px accent outline, and two rings would read as focus (builder's call,
marked for review).

### The answer of the chosen cluster

When no `?cluster` is given, or the given one is unknown, the first chip is
chosen.

1. **A heading row.** The profile's display name at title size, its id in
   mono and muted, then "Open report" at the right, a link to
   `/reports/<id>/overview`.
2. **The meta line.** It is the one the report page shows: the period in
   words, the range, the namespaces ("All namespaces" with the count it
   covered, a button that opens their list, or the names in mono),
   the report profile, and "by <owner>". A report that covers some
   namespaces says so here, so its mark is never read as the whole cluster.
3. **The overview's parts,** with the same view, components, and wording as
   the report's Overview tab:
   - the answer strip, with the data quality notice beside it from `xl`;
   - the findings chart with the growth pair under the donut;
   - the first three rows of the action plan, then a link "See the action
     plan" to the overview.

   About is left out. Every link of these parts leads into the report's own
   tabs (`/reports/<id>/findings?severity=critical` and so on).
4. **A request in flight.** A one-line notice above the heading row says
   that a new report is collecting, queued, or rendering a PDF. It gives the
   time since it was requested, and Follow, a link to `/jobs/<id>`. The
   answer below it stays that of the latest collected report.
5. **The newest request failed.** When the newest request failed after the
   latest collected report, a notice in the same place gives the failure's
   word (the one the Reports list shows) and Open, a link to that request's
   progress page. It stands above an empty state too (no report yet, only
   expired), and it stays until a newer request exists; nothing dismisses
   it (builder's call).

**When there is no answer:**

- **No report yet.** The section says so. An operator or administrator gets
  "Request a report", which opens `/generate?profile=<id>`.
- **Only expired.** The section says the latest report has expired. An
  operator gets "Request a report", which opens `/generate?profile=<id>`:
  `from` cannot serve here, because the form reads its source through
  `GET /reports/{id}`, which answers 410 for an expired report. `from` is
  safe for a live report of any owner, since every signed-in user reads
  every report.
- **Loading.** While the view loads, the heading and meta lines stand and the
  body shows the app's loading state. The previous cluster's answer stays
  until the new one arrives, as the report screen keeps a held view.
- **The view answers 404, 409, or 410.** The report was deleted or expired
  between the two reads. The section says so and the home query is fetched
  again.

### When there is no active profile

The page shows the title and one notice. An administrator reads that no
metric source is set up yet, with a link to `/profiles/new`; everyone else
reads that an administrator must add one.

## The route `GET /home`

One read for the whole page. It sits under `/api/v1` and is open to every
signed-in reader (`readerRoute`), with the checks in the usual order. It has
no query parameter.

**The answer is `Home`:**

```yaml
Home:
  type: object
  required: [clusters]
  properties:
    clusters:
      type: array
      items: { $ref: "#/components/schemas/HomeCluster" }
HomeCluster:
  type: object
  required: [profile_id, display_name, latest, newest]
  properties:
    profile_id: { type: string }
    display_name: { type: string }
    latest:
      allOf: [{ $ref: "#/components/schemas/ReportSummary" }]
      nullable: true
      description: >-
        The newest collected report that is not deleted and has not expired,
        judged with the request's one reading of the clock.
    newest:
      allOf: [{ $ref: "#/components/schemas/ReportSummary" }]
      nullable: true
      description: >-
        The newest report that is not deleted, whatever its status; equal to
        latest when nothing was requested after it.
```

**Rules of the answer:**

- One entry per active profile, in profile id order, the active profiles of
  `GET /profiles`. A profile that is not active is left out, even when it has
  reports.
- Each `ReportSummary` is built the way the list builds it (`summaryOf`,
  `reportSummaryOf`), so `expired`, `pdfs`, `phase`, and `worst_severity`
  mean what they mean there.
- `latest` excludes an expired report; `newest` may be one.

**The store.**

- One statement reads both reports for every profile id given to it: for
  each profile, the newest row with `deleted_at IS NULL`, and the newest such
  row with `status = 'collected'`, `expired_at IS NULL`, and
  `expires_at > now`. `now` is passed from Go, as every timestamp of this
  project. The builder picks `DISTINCT ON` or a lateral join.
- It reads through the main pool, never the download pool.
- A new migration, `00007_reports_by_profile.sql`, adds the index
  `ix_reports_by_profile ON reports (profile_id, created_at DESC, id DESC)
  WHERE deleted_at IS NULL`, so the read stays one index walk per profile. A
  store test shows the plan uses it, as `TestListReportsUsesTheIndex` does for
  the list. The released migrations are not touched.

**The contract.**

- `api/openapi.yaml` gains the path and the two schemas, and
  `TestRoutesMatchTheContract` holds them.
- `make generate` writes the Go and TypeScript types.
- The client's default 30 s time bound applies.

## The frontend

### The overview's parts, shared

The overview's parts read the view through `useView()`, the outlet context of
`/reports/:id`, and link relatively (`../findings`). These are the answer
strip, the quality notice, the findings chart with its growth pair, and the
plan. They become one set of components with two callers, never a copy:

- Each part takes the view and the report's base path (`/reports/<id>`) as
  props, and builds absolute links from the base.
- `OverviewTab` passes `useView()` and its own base. Home passes the view it
  fetched and the chosen report's base.
- `findings-chart.tsx` follows the same rule; the Findings tab passes its
  view too.
- The plan takes an optional limit; the overview shows every row as today.

The overview and the Findings tab must render exactly as before; their tests
and the e2e hold that.

### Data

- `useHome()` is a query on `GET /home`. It fetches again every 5 seconds
  while any `newest` is active by the list's own `isActive`, and not
  otherwise, the rule of the Reports list.
- The answer uses `useReportView(id, language)` with its existing query key
  and the language the report screen takes (`i18n.language`, else `en`).
  Opening the report after Home reuses the cached view.
- A view request keeps the 150 s bound it already has.

### The route and the navigation

- The index route of the shell renders `HomePage` in place of
  `<Navigate to="/reports">`.
- The side navigation gains `nav.home` first, linking to `/`, current only on
  `/` itself.
- The page title is "Home" through the title helper.
- After sign-in with no `next`, the reader lands on `/`.

### The new form parameter

`/generate?profile=<id>` opens the form with that profile chosen and its
defaults resolved. An unknown or inactive id falls back to the first profile,
as today, with no notice. `from` wins when both are given.

### Words

Every new text has keys in `en.json` and `pt-BR.json`, with the same
parameters in both, under a new `home.*` family where it is a family. Ages
use `Intl.RelativeTimeFormat`, as the progress page words elapsed time if it
already does (the builder checks and reuses). No banned term; technical terms
stay English in both languages.

### Look

DESIGN.md gains the cluster chip and the unseen dot under Components, and
`home` under Layout, in the same commit as the code. Every new pair enters
`pairs.ts`:

- the Ink outline of the chosen chip on Panel, at 3:1;
- the accent dot on Panel and on Ground, at 3:1 as a graphic.

No inline style, no new Radix layer. Motion only under `motion-safe:`.
PRODUCT.md lists Home among the capabilities.

## Rules this adds to AGENTS.md

In the section on the interface:

- **Home answers from two reads.** It reads `GET /home` and the chosen
  report's view. A chip's mark is the `worst_severity` of its latest report
  that is collected and has not expired, never a rule of the web app. A
  report that covers some namespaces says so in the meta line.
- **The overview's parts have one implementation,** used by the Overview tab
  and by Home, with links built from the report's base path.
- **Home checks only while something moves,** the rule of the Reports list.

The route enters the list of report routes as a read for every signed-in
user.

## What is out of scope

- A layout the reader can arrange, starred clusters, recently viewed.
- History over time, trends, or any chart of more than one report.
- A count or a judgement the API does not give: no "stale", no fleet total
  of findings.
- Any change to the Reports list, the report screen, or the PDF.
- A per-user server-side record of what was seen.

## Proof

1. **Go tests** of the store (both reports per profile, deleted and expired
   left out where the rules say, the index used), of the service (inactive
   profiles left out, one reading of the clock), and of the handler (401
   without a session, the shape). These are tagged `postgres` where they need
   the database.
2. **Vitest:**
   - each row of the table of chip marks;
   - the order;
   - the unseen dot (first visit silent, a change dotted, refused storage
     never dotted);
   - `?cluster` (default, unknown, chosen);
   - every state of the answer;
   - the roles (New report and Request hidden from a reader);
   - the overview and the Findings tab unchanged, with absolute links;
   - `?profile=` on the form.
3. **The vocabulary, contrast, inline-style, and locale tests** pass with the
   new keys and pairs.
4. **e2e.** The reports spec, after its one collection, opens `/` and finds
   its cluster's chip with the report's mark and the answer strip. A new spec
   `home.spec.ts` runs with no collection: the strip of the e2e profiles, a
   profile with no report and its Request link for an operator, and no
   Request for a reader. The prefix is `e2e-h-`. Every spec keeps its CSP and
   axe checks.
5. **A look check.** Built screens of Home in light and dark, at 1366×768 and
   1024×768, open on the companion page beside the locked sketch, before the
   merge.
6. **`make gate`.**

## Left to the builder

- `DISTINCT ON` or a lateral join, and where the statement lives in the
  store.
- The names of the shared overview components and their file split.
- The exact words of the new keys, within the meanings above. Catalog
  wording stays on the main thread for review.
- Whether the age reuses an existing elapsed-time helper.
