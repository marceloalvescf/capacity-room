// main.typ is the entry point of the capacity-report layer. It reads the
// JSON data file named by the `data` input, checks it strictly against
// schema "1", and lays the report out on top of the generic graphite
// shell. Every sentence, label, and number on the page comes from the
// data file; this module never builds wording of its own.

#import "../graphite/lib.typ": graphite-report
#import "validate.typ": fail, validate
#import "cover.typ": build-cover
#import "blocks.typ": build-block

#if not ("data" in sys.inputs) {
  fail("missing-input", "")
}

#let data = validate(json(sys.inputs.at("data")))

// Only the three labels the generic layer actually renders become
// content: `graphite-report` is never given `author`, `date`,
// `classification`, or `outline: true` below, so its own labels for them
// would never reach a page. `note`/`info`/`warning` back a callout's
// default title.
#let report-labels = (
  note: text(data.labels.note),
  info: text(data.labels.info),
  warning: text(data.labels.warning),
)

// Reproduces `report-footer` from the original Python renderer
// (older/kubernetes-report/kube_capacity_report/renderers/sections.py,
// lines 112-118): right aligned, starting on the second page.
#let report-footer = context {
  if counter(page).get().first() > 1 {
    align(right)[
      #if data.footer.classification != none [#data.footer.classification · ]
      #data.footer.page_label #counter(page).display()
    ]
  }
}

#show: graphite-report.with(
  title: build-cover(data.cover),
  outline: false,
  footer: report-footer,
  labels: report-labels,
  theme: (
    lang: data.theme.lang,
    region: data.theme.region,
    // The owner's margins of 2026-10-02: a narrower bottom than the
    // shell's default of 2.5cm on every side, for more rows per page.
    margin: (top: 1in, bottom: 0.5in, left: 1in, right: 1in),
    font: "Public Sans",
  ),
)

// A paragraph that directly follows a heading is its introduction: it sticks
// to the block after it, so that the heading, the introduction, and the next
// block fall on one page.
#for (index, block) in data.blocks.enumerate() {
  let intro = block.type == "paragraph" and index > 0 and data.blocks.at(index - 1).type == "heading"
  if intro {
    std.block(sticky: true, build-block(block))
  } else {
    build-block(block)
  }
}
