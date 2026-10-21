# Phase 16: visual reports, the browser's language, the range bar

Date: 2026-10-20. Status: decided in conversation by the owner, written for
review. The public release (phase 15, draft spec
`2026-10-20-public-release-design.md` on `feat/public-release`) waits for
this phase.

## Purpose

Three critiques of the owner, after reading real reports:

1. The reports show numbers almost only. A reader must read tables to see
   where the pressure is; the web view and the PDF have no chart of the data
   (F-289).
2. In a browser set wholly to pt-BR the web app stays in English (F-290).
3. The proof bar of "Peak CPU pressure varied among the nodes of the pool"
   fills from the middle of its track, and reads as a defect (F-291).

The phase answers the three, and the PDF takes the status colours of the
web, which reverses the monochrome rule of 2026-10-12. It ends as a minor
version, 2.7.0: it changes visual defaults and adds to the API.

## Decisions of the owner (2026-10-20)

The pictures were picked on the companion page, from real renders in the
app's tokens and faces, light and dark; their sources are under
`.cache/phase-16/look/` (`dash.html`, `pdf6.html`, `spread.html`).

### D1. The overview: one donut and a bar per scope (pick 3B)

"Pressure by scope" becomes a chart. On the left, a donut of every finding
of the report by severity (critical, attention, healthy), the total in its
middle, and a legend under it with each severity's shape, count, and word.
Under the donut, the cluster autoscaling card as today. On the right, one
row per scope type: its worst severity's shape and its name, a stacked bar
of its findings by severity on a scale shared by every row (the longest
row's total), the sentence of its worst finding as a link that opens that
finding, and its three counts.

The answer strip (the three totals and "See all findings") stays above it.
The ranked bars of usage against the request or limit (picks 2A to 2C) were
declined.

### D2. The Findings tab: the same chart above the filters (pick 4D)

Above the search, severity, and scope filters, the D1 chart again: the donut
on the left, the bar per scope on the right. A segment of a bar is a link
that sets the existing `scope` and `severity` filters of the address, so the
list narrows to it. Chosen over bars by kind of finding (4E) for one look
across the two tabs.

### D3. The PDF carries D1 on its opening page (pick 5A)

The verdict page keeps its two columns: the donut over cluster autoscaling
on the left, the scope bars with their worst finding and counts on the
right, then the first actions of the plan and the legend. The PDF and the
web overview read the same. A separate chart page (5B) was declined.

### D4. The PDF takes the status colours, as the web does (pick 6A)

The monochrome rule of 2026-10-12 is reversed. The owner: "As the same way
web is coloured, PDF must be too." The PDF is coloured exactly as the web is
today:

- every status shape (`◆`, `▲`, `●`, `○`, `—`) takes its status colour;
- the donut, the scope bars, and the depth gauge take the status colours as
  the web draws them;
- a peak cell of a table takes the tint of its status, as the web's peak
  cell does (the worse of the statuses of its peak and its share);
- every word and every number stays in ink. Coloured numbers and words (6B)
  and tinted rows of the plan (6C) were declined.

The colours are the web's light theme: critical `#be1e45`, attention
`#8f5b00`, healthy `#24774a`, neutral `#5c5c5e`, the critical surface
`#fdedf0`, the attention surface `#fdf4e3`. The shapes stay, so a page
printed in black and white still reads.

### D5. A signed-in reader follows the browser's language (F-290)

- Before sign-in, nothing changes: the stored choice, else the browser.
- After sign-in, the reader follows the browser's language unless their
  account pins one.
- Picking a language in the header menu or on the Account page pins it, on
  every device, as today.
- The Account page offers "Browser language" beside the two languages; it
  unpins.
- A new account follows the browser unless a language is given: the chart's
  `admin.language`, `admin bootstrap-admin`, and the New user dialog default
  to it.
- Existing accounts keep their stored language, pinned; nobody loses a
  choice made on purpose. The owner switches their own account once.

### D6. A range is drawn as a range (pick 7B, F-291)

A proof bar whose evidence is a range (today the two node-pool spread
findings, `NODEPOOL_CPU_UNEVEN` and `NODEPOOL_MEMORY_UNEVEN`, whose evidence
names the lowest and the highest peak) is drawn on a labelled 0 to 100%
scale as an open dot at the lowest value and a filled dot at the highest,
each labelled with its value, the span between them in the status colour,
and the spread written under the span. Web only: the PDF draws no proof bar,
and its evidence stays numbers.

### D7. The node names of a pool open on demand (pick 8B, F-292, 2026-10-21)

The Nodes column of the Node pools table on the Nodes tab no longer prints
every node name under the count. The cell shows the count and the word
("11 nodes") with a chevron, as a button; it opens a popover that lists the
pool's node names in mono, each with its copy button, and a copy of all the
names at its head, under the heading "The 11 nodes of user"; past eight
names the list scrolls. The table never changes height, and the popover
closes on a click outside or on Escape. The disclosure in the cell (8A) and
the expandable row (8C) were declined.

### D8. The chart's cards share one row, and the Findings tab folds its groups (2026-10-21)

After a test on a real cluster (474 findings), the owner picked two changes on
the companion page:

- **L3, the chart as a dashboard row (F-296).** The donut card and the scope
  card sit side by side at one height, the donut one third and the scope card
  two thirds of the width, the scope rows spread over that height. On the
  overview the cluster autoscaling card takes its own full-width row under
  them. Below `lg` everything stacks.
- **S2, folded severity groups (F-297).** On the Findings tab each severity is
  one row with its mark, its count, and a chevron; the rows open in the page,
  each list with its own scroll, and the rows below move down. A floating
  popover (S1) was declined because no other tab works that way, and tabs per
  severity (S3) were declined.
- **No finding is chosen by default.** The tab opens with every group folded
  and the detail panel showing a prompt to choose a finding. A `finding` in
  the address (from the overview's links or a bar segment) unfolds its group
  and opens it. The fold of each group is the reader's own and is not written
  to the address, and it starts again when the chosen finding changes; a
  search, a severity filter, or a scope filter in the address (the overview's
  totals and the chart's bar segments write the last two) unfolds every group
  that still shows a finding (the assistant's calls).

## The design of the parts

### The counts come from the view, decided once in Go

`AGENTS.md` keeps every rule of the overview in `internal/document`: the web
draws what the view gives. So the counts of D1 are decided by
`decideVerdict` and travel in the view and in the PDF's data file; neither
the web nor the template counts or orders anything.

- `ViewVerdict` gains `scopes`: one entry per scope type other than the
  cluster that has at least one finding in the view, a scope whose findings
  are all healthy included (an all-green bar is information), each with its
  `scope_type`, its worst `severity`, `finding` (the index in `findings` of
  the first finding of that severity, as `ViewPressure.finding`), and the
  counts of its findings `critical`, `attention`, and `healthy`.
- Every count is a count of findings, not of rows. `ViewPressure` counts
  rows; the new list does not.
- Order (the assistant's call): worst severity first, then more critical
  findings, then more attention findings, then the fixed order of
  `ViewScopeType`.
- The donut's three counts are the totals the answer strip already shows,
  over every finding, the cluster's included; the view adds none. The
  cluster's findings have their own card and no bar, so the bars sum to the
  donut's total less the cluster's findings. This is intended.
- The counts follow the view's findings, so the executive profile, whose
  minimum severity is attention, shows no healthy slice and no healthy
  count.
- When the view holds no finding (an executive report with only healthy
  findings), the donut and the bars give way to the words the tables use for
  a row with no finding ("No finding"), on the web and in the PDF; the
  cluster autoscaling card stays. A scope list that is empty while the donut
  is not (every finding is the cluster's) shows the donut alone.
- `ViewVerdict.pressure` stays in the contract with its meaning, since
  removing a field breaks the API (spec 15.1). Nothing in the app reads it
  after this phase. Its removal waits for a major version, recorded as a
  finding.
- The PDF's data file gains the same list in its verdict block, worded by
  `internal/document` (scope names, the finding's sentence, the counts), and
  `validate.typ` checks it with a new diagnostic for each new shape error.
  The count of the report layer's diagnostics in `AGENTS.md` changes with
  it.

### The web charts

- The donut, the bars, and the range bar are SVG drawn by components of
  `web/src/features/report/`, coloured through the status classes
  (`fill-critical`, `fill-attention`, `fill-healthy`), never a `style` prop.
- Each new foreground and background pair drawn together enters
  `web/src/theme/pairs.ts`: a slice or a bar against the surface, at 3:1 in
  both themes.
- The drawing is hidden from assistive technology. Every count it shows is
  also printed as text beside it, with its shape and its word
  (`StatusMark`), so nothing is said by colour alone.
- Every new word (the "findings" under the total, the captions, the range's
  "lowest", "highest", and "spread") enters both locale files.
- A segment link of D2 writes `scope` and `severity` with replace, as the
  filters do today.
- Motion, if any, appears only under `motion-safe:`.
- `DESIGN.md` gains the donut, the scope bars, and the range bar, from the
  built code.

### The PDF's colours

- The template takes the status colours of D4 as constants in one Typst
  file of `template/capacity/`. A Go test compares them with the light
  values of `web/src/theme/theme.css`, so the two cannot drift. `○`
  (undefined) and `—` (unavailable) take the neutral colour, as on the web;
  the depth gauge takes the colour of the hazard the data file gives it, as
  the web's gauge does.
- `TestGoldenPagesHaveNoColour` leaves. In its place, a test reads every
  `.typ` file of `template/` and fails on a colour literal (`rgb(`, `cmyk(`,
  `color.`, a named colour other than `black` and `white`, or a `luma(`
  outside the existing greys) anywhere but the constants file, so no colour
  enters a page outside the palette. It is seen failing before it is kept.
  The rule "No page may contain a coloured pixel" leaves `AGENTS.md`,
  `PRODUCT.md`, and `DESIGN.md`'s PDF section, each rewritten to D4.
- A peak cell's tint needs the cell's status in the data file. Where a
  table's cell carries no status today, `internal/document` adds it, through
  the existing `status` run or a new cell field, as the builder finds
  simplest. Each new shape error gets its diagnostic and fixture, and the
  count of 23 in `AGENTS.md` follows.
- The donut and the bars are drawn in Typst. The template still adds no word
  and evaluates nothing as markup.
- Every golden page that changes is regenerated by `make update-golden` and
  reviewed one by one. `capacity-parity` still pins the numbers: no number
  changes.

### The language

The API change is additive, so the version stays minor (the assistant's
design):

- `users` gains `language_follows_browser boolean NOT NULL`, through a new
  migration: the column is added with `DEFAULT false`, which carries every
  existing row forward pinned, and the default is then dropped, so every
  insert names the value.
- `Principal` and `User` gain the required field `language_follows_browser`.
  `language` keeps its type and stays required: the pinned language, or,
  while the account follows the browser, the language it last pinned or was
  created with.
- `PATCH /account` with `language` pins it (the flag becomes false). A new
  boolean `language_follows_browser` accepts only `true`, which unpins and
  keeps the stored `language`; `false` is a 400 (to pin, send the language),
  and so is a body that sends both fields. The administrator's update of a
  user takes the same two fields with the same rules.
- An unpin writes the audit row a language change writes today, with the
  new state in its detail.
- `POST /admin/users` takes `language` as optional. When it is absent, the
  account follows the browser and stores `en` as its `language`.
- `admin bootstrap-admin --language` becomes optional, with the same
  meaning. The chart's `admin.language` defaults to empty and passes no flag
  when empty; `values.schema.json` and the chart's README follow.
- After sign-in the web app uses `browserLanguage()` when the principal
  follows the browser, else `principal.language`; the stored choice of
  before sign-in is removed after sign-in, as today. The header menu shows
  the language in use; picking one sends the `PATCH` and pins it, even when
  it equals the browser's. The Account page and the user row offer "Browser
  language".
- The report form already defaults to the language in use (`i18n.language`),
  not to `principal.language`, so it needs no change. The builder lists
  every other reader of `principal.language` and of `User.language` and
  moves each to the language in use where it means the reader's language.
- `docs/operations.md` (the flag, the value) and the chart's README change
  in the same commit as the code.

### The range bar

`proof.ts` recognises a range by its evidence (the lowest and the highest
peak), not by the value of `from`, and says so in what it returns, so a
range whose lowest value is 0% is still drawn as a range. The range is drawn
by its own component; `ProofBar` loses `from` and every other bar is
unchanged, filling from zero.

### The node pool popover

- The popover is the Radix popover already in use, so the five layers of the
  no-inline-style test stay five; its content sits at `z-50` and lifts with
  `shadow-float`.
- The button's accessible name says the count and the pool ("11 nodes of
  user"), from the locale files, which also hold the heading and the copy
  labels.
- The Node pools table becomes a `DataTable`, the app's one table, which
  `AGENTS.md` requires and which this table is not today (the assistant's
  call, F-293); it has no expander column, since its rows open nothing.
- `DESIGN.md` gains the count-button and its popover.

## What is out of scope

- Charts of usage over time: the snapshot holds no series, and collecting
  them is another phase.
- The ranked usage bars (2A to 2C) and the bars by kind of finding (4E).
- Coloured words or numbers (6B, 6C), in the web or the PDF.
- A chart in the PDF beyond D3.
- Removing `ViewVerdict.pressure`.

## Proof

- `make gate` passes on the branch, from images built without cache.
- The golden pages, snapshots, data files, and views are regenerated by the
  `make update-*` targets and read whole before the commit.
- The built screens are shown to the owner on the companion page, from the
  running demo stack, light and dark, before the version commit: the
  overview and the Findings tab of the staging-shaped data, the range bar,
  the Account page's language choice, and the opening page of the PDF in
  both languages.
- An adversarial review of the whole branch runs before the merge.
- The changelog's `[Unreleased]` names, for a user: the charts on the
  overview, the Findings tab, and the PDF's first page; the PDF in colour;
  the app following the browser's language; the range drawn as a range;
  the optional `--language` and `admin.language`.

## Left to the builder

- The exact geometry of the donut and the bars, within the mockups.
- How the Typst donut is drawn (arcs or curves).
- The wording of the new captions, proposed by the main thread and checked
  in both languages.
