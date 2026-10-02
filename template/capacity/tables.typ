// tables.typ holds the layout (column widths, text size, and row inset)
// for every table id the capacity report can render, and the one helper
// that reproduces `data-table` from the original Python renderer
// (older/kubernetes-report/kube_capacity_report/renderers/sections.py,
// lines 44-89), with one difference: every cell, the header row included,
// is centred horizontally and vertically, where the original set them at
// the top left. `validate.typ` reads `layouts` to check a table block's
// column count before any content is built; `blocks.typ` calls
// `render-table` once the shape is already known to be correct.

#let layouts = (
  report-metadata: (columns: (1fr, 3fr), size: 8.5pt, inset-y: 2pt),
  status-legend: (columns: (0.7fr, 3fr), size: 8.5pt, inset-y: 2pt),
  namespace-capacity: (
    columns: (0.8fr, 1fr, 1fr, 1fr, 1fr, 0.9fr, 0.9fr),
    size: 7.1pt,
    inset-y: 3.5pt,
  ),
  workload-resources: (
    columns: (2.2fr, 0.45fr, 0.65fr, 0.65fr, 0.72fr, 0.72fr, 0.72fr, 0.72fr, 0.72fr, 0.72fr),
    size: 6.2pt,
    inset-y: 3.5pt,
  ),
  hpa-observed: (
    columns: (2.2fr, 0.65fr, 0.65fr, 0.8fr, 0.72fr, 0.72fr, 1.15fr),
    size: 7pt,
    inset-y: 3.5pt,
  ),
  workloads-without-hpa: (
    columns: (0.9fr, 2.2fr, 0.65fr, 2.1fr),
    size: 7.4pt,
    inset-y: 3.5pt,
  ),
  nodepool-summary: (
    columns: (1.6fr, 0.45fr, 0.75fr, 0.75fr, 0.85fr, 0.85fr, 0.9fr),
    size: 7.1pt,
    inset-y: 3.5pt,
  ),
  nodepool-profile: (
    columns: (1.5fr, 0.9fr, 0.85fr, 0.95fr, 1fr, 0.95fr, 1.05fr),
    size: 6.8pt,
    inset-y: 3.5pt,
  ),
  node-detail: (
    columns: (2.4fr, 1.5fr, 1.6fr, 2.2fr, 1.2fr),
    size: 7.1pt,
    inset-y: 3.5pt,
  ),
  priority-workloads: (
    columns: (1.25fr, 2fr, 0.55fr, 4fr),
    size: 7.2pt,
    inset-y: 3.5pt,
  ),
  namespaces-at-risk: (
    columns: (1.35fr, 0.75fr, 0.75fr, 3fr, 1.25fr, 0.85fr),
    size: 7.2pt,
    inset-y: 3.5pt,
  ),
  replica-ceiling: (
    columns: (1.25fr, 2fr, 0.9fr, 1.5fr, 1.7fr),
    size: 7.2pt,
    inset-y: 3.5pt,
  ),
  nodepool-status: (
    columns: (1.8fr, 0.9fr, 3.5fr),
    size: 7.3pt,
    inset-y: 3.5pt,
  ),
  action-plan: (
    columns: (0.8fr, 0.8fr, 2.7fr, 2.6fr),
    size: 7.2pt,
    inset-y: 3.5pt,
  ),
  material-limitations: (
    columns: (1.8fr, 0.8fr, 3fr),
    size: 7.5pt,
    inset-y: 3.5pt,
  ),
  quality-appendix: (
    columns: (1.6fr, 0.8fr, 1fr, 2fr, 2.4fr),
    size: 7pt,
    inset-y: 3.5pt,
  ),
  interpretation-notes: (columns: (1fr,), size: 7.5pt, inset-y: 3.5pt),
)

// render-table assumes `id`, `headers`, and `rows` already passed
// `validate.typ`'s checks: `id` is a known layout, `headers` holds as many
// strings as `layout.columns`, and every row of `rows` (already converted
// to content by `runs.typ`) holds as many cells.
#let render-table(id, headers, rows) = {
  let layout = layouts.at(id)
  block(width: 100%)[
    #set text(size: layout.size)
    #set par(justify: false, leading: 0.45em, spacing: 0.4em)
    #set table(inset: (x: 3pt, y: layout.inset-y), stroke: 0.5pt)
    #let header-cells = headers.map(item => text(weight: "bold", item))
    #table(
      columns: layout.columns,
      align: center + horizon,
      table.header(repeat: true, ..header-cells),
      ..rows.flatten(),
    )
  ]
}
