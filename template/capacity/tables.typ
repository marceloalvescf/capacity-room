// tables.typ holds the layout (column widths, text size, row inset, and
// the face of each column) for every table id the capacity report can
// render, and the one helper that draws a table. `validate.typ` reads
// `layouts` to check a table block's column count before any content is
// built; `blocks.typ` calls `render-table` once the shape is already known
// to be correct.

#import "palette.typ": attention-surface, critical-surface
#import "runs.typ": mono, run-to-content

#let layouts = (
  report-metadata: (columns: (1fr, 3fr), size: 8.5pt, inset-y: 2pt, faces: ("text", "text")),
  status-legend: (columns: (0.7fr, 3fr), size: 8.5pt, inset-y: 2pt, faces: ("mark", "text")),
  namespace-capacity: (
    columns: (0.8fr, 1fr, 1fr, 1fr, 1fr, 0.9fr, 0.9fr),
    size: 7.1pt,
    inset-y: 3.5pt,
    faces: ("text", "num", "num", "num", "num", "num", "num"),
  ),
  workload-resources: (
    columns: (1.28fr, 0.45fr, 0.95fr, 0.95fr, 0.72fr, 0.72fr, 0.95fr, 0.95fr, 0.72fr, 0.72fr),
    size: 6.8pt,
    inset-y: 3.5pt,
    faces: ("name", "num", "num", "num", "num", "num", "num", "num", "num", "num"),
  ),
  hpa-observed: (
    columns: (2.2fr, 0.65fr, 0.65fr, 0.8fr, 0.72fr, 0.72fr, 1.15fr),
    size: 7pt,
    inset-y: 3.5pt,
    faces: ("name", "num", "num", "num", "num", "num", "text"),
  ),
  workloads-without-hpa: (
    columns: (0.9fr, 2.2fr, 0.65fr, 2.1fr),
    size: 7.4pt,
    inset-y: 3.5pt,
    faces: ("text", "name", "num", "text"),
  ),
  nodepool-summary: (
    columns: (1.6fr, 0.45fr, 0.75fr, 0.75fr, 0.85fr, 0.85fr, 0.9fr),
    size: 7.1pt,
    inset-y: 3.5pt,
    faces: ("name", "num", "num", "num", "num", "num", "text"),
  ),
  nodepool-profile: (
    columns: (1.5fr, 0.9fr, 0.85fr, 0.95fr, 1fr, 0.95fr, 1.05fr),
    size: 6.8pt,
    inset-y: 3.5pt,
    faces: ("name", "num", "num", "num", "num", "num", "num"),
  ),
  node-detail: (
    columns: (2.4fr, 1.5fr, 1.6fr, 2.2fr, 1.2fr),
    size: 7.1pt,
    inset-y: 3.5pt,
    faces: ("name", "num", "num", "text", "text"),
  ),
  priority-workloads: (
    columns: (1.25fr, 2fr, 0.55fr, 4fr),
    size: 7.2pt,
    inset-y: 3.5pt,
    faces: ("name", "name", "num", "text"),
  ),
  namespaces-at-risk: (
    columns: (1.35fr, 0.75fr, 0.75fr, 3fr, 1.25fr, 0.85fr),
    size: 7.2pt,
    inset-y: 3.5pt,
    faces: ("name", "num", "num", "text", "text", "mark"),
  ),
  replica-ceiling: (
    columns: (1.25fr, 2fr, 0.9fr, 1.5fr, 1.7fr),
    size: 7.2pt,
    inset-y: 3.5pt,
    faces: ("name", "name", "text", "num", "text"),
  ),
  nodepool-status: (
    columns: (1.8fr, 0.9fr, 3.5fr),
    size: 7.3pt,
    inset-y: 3.5pt,
    faces: ("name", "mark", "text"),
  ),
  action-plan: (
    columns: (0.3fr, 1fr, 4fr, 2.2fr),
    size: 7.2pt,
    inset-y: 3.5pt,
    faces: ("num", "mark", "text", "name"),
  ),
  verdict-plan: (
    columns: (0.3fr, 1fr, 4fr, 2.2fr),
    size: 8pt,
    inset-y: 4pt,
    faces: ("num", "mark", "text", "name"),
  ),
  material-limitations: (
    columns: (1.8fr, 0.8fr, 3fr),
    size: 7.5pt,
    inset-y: 3.5pt,
    faces: ("text", "mark", "text"),
  ),
  quality-appendix: (
    columns: (1.6fr, 0.95fr, 1fr, 2fr, 2.4fr),
    size: 7pt,
    inset-y: 3.5pt,
    faces: ("text", "mark", "text", "text", "text"),
  ),
  interpretation-notes: (columns: (1fr,), size: 7.5pt, inset-y: 3.5pt, faces: ("text",)),
)

// A face says how a column reads: a name or a text from the left, a
// number from the right in the monospaced face, a mark from the left.
#let face-align = (name: left, num: right, text: left, mark: left)

// A cell of a number column sets in the monospaced face only the runs whose
// text holds a digit; a word keeps the body face and may wrap.
#let num-run(run) = {
  let content = run-to-content(run)
  if run.kind in ("text", "strong", "placeholder", "status") and run.text != none and run.text.contains(regex("\\d")) {
    text(font: mono, content)
  } else {
    content
  }
}

// A number cell holding a critical or attention status run is tinted with the
// surface of the worse one; every other cell has no fill.
#let cell-fill(cell) = {
  let statuses = cell.filter(run => run.kind == "status").map(run => run.status)
  if "critical" in statuses { critical-surface } else if "attention" in statuses { attention-surface } else { none }
}

// render-table assumes `id`, `headers`, and `rows` already passed
// `validate.typ`'s checks: `id` is a known layout, `headers` holds as many
// strings as `layout.columns`, and every row of `rows` (an array of runs per
// cell) holds as many cells.
#let render-table(id, headers, rows) = {
  let layout = layouts.at(id)
  let faces = layout.faces
  block(width: 100%)[
    #set text(size: layout.size)
    #set par(justify: false, leading: 0.45em, spacing: 0.4em)
    #set table(inset: (x: 3pt, y: layout.inset-y), fill: none, stroke: (x, y) => (
      top: if y == 0 { 0.8pt + black } else { 0.5pt + luma(78%) },
      bottom: 0.5pt + luma(78%),
    ))
    #let header-cells = headers.enumerate().map(((i, item)) => table.cell(
      align: face-align.at(faces.at(i)) + horizon,
      text(weight: "semibold", item),
    ))
    #let body = rows.map(row => row.enumerate().map(((i, cell)) => table.cell(
      align: face-align.at(faces.at(i)) + horizon,
      fill: if faces.at(i) == "num" { cell-fill(cell) } else { none },
      if faces.at(i) == "num" { cell.map(num-run).join() } else { cell.map(run-to-content).join() },
    ))).flatten()
    #table(
      columns: layout.columns,
      table.header(repeat: true, ..header-cells),
      ..body,
    )
  ]
}
