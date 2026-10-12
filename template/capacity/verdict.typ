// verdict.typ draws the first page: the answer strip, whether the cluster
// can grow, where it runs short, the first actions, and the legend. Every
// word comes from the block; the soundings of the gauge are the integers it
// was given.

#import "runs.typ": mono, runs-to-content, status-glyph
#import "tables.typ": render-table

#let muted = luma(35%)
#let hair = luma(78%)
#let tints = (luma(93%), luma(85%), luma(74%))
#let band-stroke = 0.4pt + luma(55%)

// A status glyph in the monospaced face, then the label.
#let mark(status, label) = box[#status-glyph(status)#h(3pt)#label]

#let num(body) = text(font: mono, body)

#let eyebrow(body) = text(size: 7pt, weight: "semibold", tracking: 0.08em, fill: muted, upper(body))

// The soundings under the scale: every node up to ten, else ten values
// spread evenly, the first and the last included.
#let soundings(max) = {
  if max <= 10 {
    range(max + 1)
  } else {
    range(10).map(i => calc.round(i * max / 9)).dedup()
  }
}

#let hatch-fill = tiling(size: (5pt, 5pt))[
  #place(rect(width: 5pt, height: 5pt, fill: tints.at(2), stroke: none))
  #place(line(start: (0pt, 5pt), end: (5pt, 0pt), stroke: 1.1pt + black))
  #place(line(start: (-1pt, 1pt), end: (1pt, -1pt), stroke: 1.1pt + black))
  #place(line(start: (4pt, 6pt), end: (6pt, 4pt), stroke: 1.1pt + black))
]

// The depth gauge in black: up to sixty nodes, one band per node, filled
// bands tinted by thirds; above that, three bands over the thirds of the
// scale, each filled to the share of the current count that falls in it.
#let gauge(spec) = layout(size => {
  let w = size.width
  let height = 16pt
  let max = spec.max
  let full = spec.hazard and spec.current >= max
  box(width: w, height: height + 10pt, {
    if max <= 60 {
      let band = w / max
      for i in range(max) {
        let filled = i < spec.current
        let tint = tints.at(calc.min(2, calc.floor(i * 3 / max)))
        let fill = if filled and full and i == max - 1 { hatch-fill } else if filled { tint } else { white }
        place(dx: i * band, rect(width: band, height: height, fill: fill, stroke: band-stroke))
      }
    } else {
      let third = max / 3
      let band = w / 3
      for k in range(3) {
        let share = calc.clamp((spec.current - k * third) / third, 0, 1)
        place(dx: k * band, rect(width: band, height: height, fill: white, stroke: band-stroke))
        if share > 0 {
          let fill = if full and k == 2 { hatch-fill } else { tints.at(k) }
          place(dx: k * band, rect(width: band * share, height: height, fill: fill, stroke: none))
        }
        place(dx: k * band, rect(width: band, height: height, fill: none, stroke: band-stroke))
      }
    }
    place(rect(width: w, height: height, stroke: 0.9pt + black, fill: none))
    if spec.min != none and spec.min > 0 and spec.min < max {
      place(
        dx: spec.min * w / max,
        line(start: (0pt, -2pt), end: (0pt, height + 2pt), stroke: (paint: black, thickness: 0.7pt, dash: "dashed")),
      )
    }
    for value in soundings(max) {
      place(
        dx: value * w / max - 4pt,
        dy: height + 2pt,
        box(width: 8pt, align(center, text(font: mono, size: 5.5pt, style: "italic", fill: muted, str(value)))),
      )
    }
  })
})

#let count-line(counts) = {
  counts
    .map(c => mark(c.status, [#c.label #num(c.count)]))
    .join(h(4pt) + text(fill: muted, "·") + h(4pt))
}

#let build-totals(totals) = {
  if totals.none != none {
    text(size: 13pt, totals.none)
  } else {
    let lead = totals.items.position(item => item.count != "0")
    text(size: 13pt, totals.items.enumerate().map(((i, item)) => {
      let weight = if i == lead or item.status == "critical" { "bold" } else { "regular" }
      text(weight: weight, mark(item.status, [#item.label #num(item.count)]))
    }).join(h(10pt)))
  }
}

#let build-quality(quality) = {
  let parts = quality.items.join(" · ", default: "")
  if quality.more != none {
    parts = parts + " " + quality.more
  }
  mark("attention", text(weight: "bold", quality.label)) + [ ] + num(quality.count) + h(6pt) + text(fill: muted, parts)
}

#let build-growth(growth) = {
  eyebrow(growth.title)
  v(2pt)
  if growth.current != none {
    text(font: mono, size: 30pt, weight: "semibold", tracking: -0.04em)[#growth.current#h(0.12em)/#h(0.12em)#growth.max]
  } else if growth.state != none {
    text(size: 20pt, weight: "semibold", growth.state)
  }
  if growth.gauge != none {
    v(-2pt)
    gauge(growth.gauge)
    if growth.caption != none {
      text(size: 7.5pt, growth.caption)
    }
  }
  if growth.finding != none {
    v(6pt)
    mark(growth.finding.status, text(weight: "bold", growth.finding.label))
    linebreak()
    growth.finding.sentence
  }
  if growth.warning != none {
    v(4pt)
    mark("attention", growth.warning)
  }
  if growth.facts.len() > 0 {
    v(4pt)
    text(fill: muted, growth.facts.map(f => [#f.label #num(f.value)]).join(" · "))
  }
}

#let build-pressure(pressure) = {
  eyebrow(pressure.title)
  v(2pt)
  if pressure.empty != none {
    pressure.empty
  }
  for row in pressure.rows {
    line(length: 100%, stroke: 0.5pt + hair)
    grid(
      columns: (7.8em, 1fr),
      column-gutter: 6pt,
      row-gutter: 3pt,
      mark(row.status, text(weight: "semibold", row.scope)),
      {
        row.sentence
        if row.name.len() > 0 or row.peak != none {
          linebreak()
        }
        if row.name.len() > 0 {
          runs-to-content(row.name)
        }
        if row.peak != none {
          h(6pt)
          text(fill: muted)[#row.peak.label #row.peak.value]
        }
        linebreak()
        count-line(row.counts)
      },
    )
  }
}

#let build-plan(plan) = {
  text(size: 11pt, weight: "semibold", plan.title)
  v(2pt)
  if plan.table != none {
    render-table(plan.table.id, plan.table.headers, plan.table.rows)
  } else if plan.empty != none {
    plan.empty
  }
  if plan.more != none {
    text(size: 7.5pt, fill: muted, plan.more)
  }
}

#let build-verdict(entry) = {
  set text(hyphenate: false)
  set text(size: 9pt)
  set par(justify: false, leading: 0.5em, spacing: 0.7em)
  build-totals(entry.totals)
  if entry.quality != none {
    v(4pt)
    build-quality(entry.quality)
  }
  v(10pt)
  grid(
    columns: (0.9fr, 1.5fr),
    column-gutter: 18pt,
    build-growth(entry.growth), build-pressure(entry.pressure),
  )
  v(10pt)
  build-plan(entry.plan)
  v(6pt)
  text(size: 7.5pt, entry.legend.map(item => mark(item.status, item.label)).join(h(6pt)))
}
