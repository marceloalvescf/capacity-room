// runs.typ turns an array of run objects, already checked by
// `validate.typ`, into Typst content. A run's `text` (and a callout
// title's or table cell's runs) is only ever handed to `text(...)`; this
// module never parses a string as markup. Reproduces `report-ident`,
// `report-placeholder`, and `report-status` from the original Python
// renderer (older/kubernetes-report/kube_capacity_report/renderers/
// sections.py, lines 44-68).

#let status-symbols = (
  healthy: "●",
  attention: "▲",
  critical: "♦",
  undefined: "○",
  unavailable: "—",
)

#let run-to-content(run) = {
  if run.kind == "text" {
    text(run.text)
  } else if run.kind == "ident" {
    text(size: 0.92em, hyphenate: false, lang: "en", run.text)
  } else if run.kind == "placeholder" {
    text(size: 0.92em, run.text)
  } else {
    let symbol = status-symbols.at(run.status)
    let fill = if run.status in ("healthy", "unavailable", "undefined") { luma(45%) } else { black }
    let weight = if run.status == "critical" { "bold" } else { "regular" }
    box(text(fill: fill, weight: weight)[
      #symbol#if run.text != none { h(2pt); text(run.text) }
    ])
  }
}

// runs-to-content joins the content of every run into a single value, in
// order: a heading, a paragraph, a table cell, and a callout title or body
// are all built this way.
#let runs-to-content(runs) = runs.map(run-to-content).join()
