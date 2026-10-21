// verdict.typ draws the first page: the answer strip, whether the cluster
// can grow, where it runs short, the first actions, and the legend. Every
// word comes from the block; the soundings of the gauge are the integers it
// was given.

#import "palette.typ": status-colour
#import "runs.typ": mono, runs-to-content, status-glyph, status-mark
#import "tables.typ": num-run, render-table

#let muted = luma(35%)
#let hair = luma(78%)
#let track = luma(93%)
#let tints = (luma(93%), luma(85%), luma(74%))
#let band-stroke = 0.4pt + luma(55%)

// A status glyph in the monospaced face, then the label. The glyph is an
// artifact, since the label or the sentence after it tells the status,
// unless `alt` gives the word of the status for a label that does not.
#let mark(status, label, alt: none) = box[#status-mark(status, alt)#h(3pt)#label]

#let num(body) = text(font: mono, body)

// A value set by the digit rule of the tables: monospaced only when it
// holds a digit, so a placeholder word keeps the body face.
#let value(body) = num-run((kind: "text", text: body))

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

// The hatch of a full gauge: diagonal strokes over the colour of the last band.
#let hatch-fill(base, ink) = tiling(size: (5pt, 5pt))[
  #place(rect(width: 5pt, height: 5pt, fill: base, stroke: none))
  #place(line(start: (0pt, 5pt), end: (5pt, 0pt), stroke: 1.1pt + ink))
  #place(line(start: (-1pt, 1pt), end: (1pt, -1pt), stroke: 1.1pt + ink))
  #place(line(start: (4pt, 6pt), end: (6pt, 4pt), stroke: 1.1pt + ink))
]

// The depth gauge: up to sixty nodes, one band per node, filled
// bands in the colour of its hazard (greys when it has none) by thirds; above
// that, three bands over the thirds of the
// scale, each filled to the share of the current count that falls in it.
#let gauge(spec) = layout(size => {
  let w = size.width
  let height = 16pt
  let max = spec.max
  let full = spec.hazard and spec.current >= max
  let thirds = if spec.status == none { tints } else {
    let base = status-colour(spec.status)
    (base.lighten(70%), base.lighten(40%), base)
  }
  let hatch-ink = if spec.status == none { black } else { white }
  box(width: w, height: height + 10pt, {
    if max <= 60 {
      let band = w / max
      for i in range(max) {
        let filled = i < spec.current
        let tint = thirds.at(calc.min(2, calc.floor(i * 3 / max)))
        let fill = if filled and full and i == max - 1 { hatch-fill(thirds.at(2), hatch-ink) } else if filled { tint } else { white }
        place(dx: i * band, rect(width: band, height: height, fill: fill, stroke: band-stroke))
      }
    } else {
      let third = max / 3
      let band = w / 3
      for k in range(3) {
        let share = calc.clamp((spec.current - k * third) / third, 0, 1)
        place(dx: k * band, rect(width: band, height: height, fill: white, stroke: band-stroke))
        if share > 0 {
          let fill = if full and k == 2 { hatch-fill(thirds.at(2), hatch-ink) } else { thirds.at(k) }
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

// The values with every non-zero one raised to at least `floor`, the
// difference taken from the largest, so the sum is kept and no count
// vanishes from a chart.
#let lift(values, floor) = {
  let sized = values.map(v => if v > 0 and v < floor { floor } else { v })
  let extra = sized.sum(default: 0) - values.sum(default: 0)
  let big = sized.position(v => v == calc.max(..sized))
  sized.enumerate().map(((i, v)) => if i == big { v - extra } else { v })
}

// The ring of the findings: one slice per status with a count, clockwise
// from the top, the slices parted by a small gap. A ring of one slice is a
// full circle.
#let ring-outer = 70pt
#let ring-inner = 51pt
#let ring-gap = 2pt
// The least sweep of a slice, its gap included.
#let ring-floor = 6deg

#let ring-slice(from, to, fill) = if to > from {
  let steps = calc.max(2, calc.ceil((to - from) / 1deg))
  let point(radius, angle) = (
    ring-outer + radius * calc.sin(angle),
    ring-outer - radius * calc.cos(angle),
  )
  let outer = range(steps + 1).map(i => point(ring-outer, from + (to - from) * i / steps))
  let inner = range(steps + 1).rev().map(i => point(ring-inner, from + (to - from) * i / steps))
  place(polygon(fill: fill, stroke: none, ..(outer + inner)))
}

#let donut(totals, findings) = {
  let sum = totals.map(item => item.value).sum(default: 0)
  let shown = totals.filter(item => item.value > 0)
  let size = 2 * ring-outer
  eyebrow(findings.title)
  v(4pt)
  align(center, pdf.artifact(box(width: size, height: size, {
    if shown.len() == 1 {
      place(circle(radius: ring-outer, fill: status-colour(shown.first().status), stroke: none))
      place(dx: ring-outer - ring-inner, dy: ring-outer - ring-inner, circle(radius: ring-inner, fill: white, stroke: none))
    } else {
      let gap = ring-gap / ((ring-outer + ring-inner) / 2) * 1rad
      let turn = 0deg
      let shares = lift(shown.map(item => item.value), sum * ring-floor / 360deg)
      for (item, share) in shown.zip(shares) {
        let span = 360deg * share / sum
        ring-slice(turn + gap / 2, turn + span - gap / 2, status-colour(item.status))
        turn += span
      }
    }
    place(center + horizon, align(center, {
      text(font: mono, size: 26pt, weight: "bold", str(sum))
      linebreak()
      text(size: 8pt, fill: muted, findings.total_label)
    }))
  })))
  v(6pt)
  align(center, grid(
    columns: 4,
    column-gutter: 5pt,
    row-gutter: 3pt,
    align: (left, left, right, left),
    ..totals
      .map(item => (
        box(rect(width: 12pt, height: 8pt, fill: status-colour(item.status), stroke: none)),
        pdf.artifact(status-glyph(item.status)),
        num(str(item.value)),
        item.label,
      ))
      .flatten()
  ))
}

// The counts of a row as a monospaced line: each count after the glyph of
// its status, which carries the word of the status for a screen reader.
#let count-line(counts) = {
  counts
    .map(c => mark(c.status, num(c.count), alt: c.label))
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
    // Two counts are set large; a word standing for an unknown count would
    // not fit the column at that size.
    let known = growth.current.contains(regex("\\d")) and growth.max.contains(regex("\\d"))
    text(size: if known { 30pt } else { 15pt }, weight: "semibold", tracking: -0.04em)[#value(growth.current)#h(0.12em)#text(font: mono)[/]#h(0.12em)#value(growth.max)]
  } else if growth.state != none {
    text(size: 20pt, weight: "semibold", growth.state)
  }
  if growth.gauge != none {
    v(-2pt)
    gauge(growth.gauge)
  }
  if growth.caption != none {
    v(2pt)
    text(size: 7.5pt, growth.caption)
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
    text(fill: muted, growth.facts.map(f => box[#f.label #value(f.value)]).join(" · "))
  }
}

// The bar of a row: its findings per status against the scale, over a light
// track; the segments are parted by a thin white line.
// The least share of the scale a segment takes, its stroke included.
#let bar-floor = 0.03

#let pressure-bar(bar) = {
  let parts = ("critical", "attention", "healthy")
    .map(status => (status, bar.at(status)))
    .filter(((_, count)) => count > 0)
  let rest = calc.max(0, bar.scale - parts.map(((_, count)) => count).sum(default: 0))
  let sizes = lift(parts.map(((_, count)) => count) + (rest,), bar.scale * bar-floor)
  grid(
    columns: sizes.map(size => size * 1fr),
    rows: 5pt,
    fill: (x, _) => if x < parts.len() { status-colour(parts.at(x).first()) } else { track },
    stroke: (x, _) => if x < parts.len() { (right: 0.8pt + white) } else { none },
    ..range(parts.len() + 1).map(_ => []),
  )
}

// The word of a status as the legend gives it, or none when the legend
// does not hold the status.
#let status-word(legend, status) = {
  let item = legend.find(item => item.status == status)
  if item == none { none } else { item.label }
}

#let build-pressure(pressure, legend) = {
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
      mark(row.status, text(weight: "semibold", row.label), alt: status-word(legend, row.status)),
      {
        pressure-bar(row.bar)
        v(3pt)
        row.sentence
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
    {
      // With no finding at all there is no ring to draw: the strip above
      // says so, and the growth block stands alone.
      if entry.totals.items.map(item => item.value).sum(default: 0) > 0 {
        donut(entry.totals.items, entry.findings)
        v(14pt)
      }
      build-growth(entry.growth)
    },
    build-pressure(entry.pressure, entry.legend),
  )
  v(10pt)
  build-plan(entry.plan)
  v(6pt)
  text(size: 7.5pt, entry.legend.map(item => mark(item.status, item.label)).join(h(6pt)))
}
