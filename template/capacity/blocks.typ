// blocks.typ turns one entry of `data.blocks`, already checked by
// `validate.typ`, into Typst content. Every word comes from the block
// itself; this module only dispatches by `type` and lays the result out.

#import "../graphite/lib.typ": callout
#import "runs.typ": runs-to-content
#import "tables.typ": render-table

#let build-block(block) = {
  if block.type == "outline" {
    outline(title: text(block.title), depth: block.depth)
  } else if block.type == "heading" {
    heading(level: block.level, runs-to-content(block.runs))
  } else if block.type == "paragraph" {
    par(runs-to-content(block.runs))
  } else if block.type == "table" {
    render-table(
      block.id,
      block.headers,
      block.rows.map(row => row.map(cell => runs-to-content(cell))),
    )
  } else if block.type == "callout" {
    let title = if block.title == none { none } else { runs-to-content(block.title) }
    callout(kind: block.kind, title: title, runs-to-content(block.runs))
  } else {
    pagebreak(weak: block.weak)
  }
}
