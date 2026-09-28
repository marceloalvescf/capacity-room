// blocks.typ turns one entry of `data.blocks`, already checked by
// `validate.typ`, into Typst content. Every word comes from the block
// itself; this module only dispatches by `type` and lays the result out.
//
// The parameter is named `entry`, not `block`, because `front` needs to
// call the native `block(...)` layout function from inside this file, and
// a parameter named `block` would shadow it.

#import "../graphite/lib.typ": callout
#import "runs.typ": runs-to-content
#import "tables.typ": render-table

#let build-block(entry) = {
  if entry.type == "outline" {
    outline(title: text(entry.title), depth: entry.depth)
  } else if entry.type == "front" {
    // Reproduces the opening block of the original
    // (older/kubernetes-report/tests/golden/typst/detailed-all.typ,
    // lines 82-132): smaller text and tighter leading than the rest of
    // the report.
    block(width: 100%)[
      #set text(size: 8pt)
      #set par(leading: 0.45em, spacing: 0.4em)
      #entry.blocks.map(build-block).join()
    ]
  } else if entry.type == "heading" {
    if entry.numbered {
      heading(level: entry.level, runs-to-content(entry.runs))
    } else {
      heading(level: entry.level, numbering: none, outlined: false, runs-to-content(entry.runs))
    }
  } else if entry.type == "paragraph" {
    par(runs-to-content(entry.runs))
  } else if entry.type == "list" {
    list(..entry.items.map(item => runs-to-content(item)))
  } else if entry.type == "table" {
    render-table(
      entry.id,
      entry.headers,
      entry.rows.map(row => row.map(cell => runs-to-content(cell))),
    )
  } else if entry.type == "callout" {
    let title = if entry.title == none { none } else { runs-to-content(entry.title) }
    callout(kind: entry.kind, title: title, runs-to-content(entry.runs))
  } else {
    pagebreak(weak: entry.weak)
  }
}
