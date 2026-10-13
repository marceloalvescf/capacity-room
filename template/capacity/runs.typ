// runs.typ turns an array of run objects, already checked by
// `validate.typ`, into Typst content. A run's `text` (and a callout
// title's or table cell's runs) is only ever handed to `text(...)`; this
// module never parses a string as markup. It draws the runs of the data
// file: text, strong words, identifiers, placeholders, and statuses.

#let mono = "JetBrains Mono"

#let status-symbols = (
  healthy: "●",
  attention: "▲",
  critical: "◆",
  undefined: "○",
  unavailable: "—",
)

#let status-fill(status) = if status in ("healthy", "unavailable", "undefined") { luma(45%) } else { black }

// status-glyph sets the mark of a status in the monospaced face, which
// holds all five glyphs; the face of the words holds none of them.
#let status-glyph(status) = text(
  font: mono,
  fill: status-fill(status),
  weight: if status == "critical" { "bold" } else { "regular" },
  status-symbols.at(status),
)

// status-mark is the glyph as the PDF's tags give it to a screen reader.
// With no `alt`, the word of the status stands beside the mark, in its run
// or in the next cell of the legend, so the mark is an artifact that the
// reader skips. With an `alt`, the text beside the mark is a value, and the
// mark carries the word of its status as its alternate text; the page is
// drawn the same either way.
#let status-mark(status, alt) = if alt == none {
  pdf.artifact(status-glyph(status))
} else {
  box(figure(alt: alt, box(status-glyph(status))))
}

#let run-to-content(run) = {
  if run.kind == "text" {
    text(run.text)
  } else if run.kind == "strong" {
    text(weight: "bold", run.text)
  } else if run.kind == "ident" {
    text(font: mono, size: 0.92em, hyphenate: false, lang: "en", run.text)
  } else if run.kind == "placeholder" {
    text(run.text)
  } else {
    let weight = if run.status == "critical" { "bold" } else { "regular" }
    box(text(fill: status-fill(run.status), weight: weight)[
      #status-mark(run.status, run.at("alt", default: none))#if run.text != none { h(2pt); text(run.text) }
    ])
  }
}

// runs-to-content joins the content of every run into a single value, in
// order: a heading, a paragraph, a table cell, and a callout title or body
// are all built this way.
#let runs-to-content(runs) = runs.map(run-to-content).join()
