#import "components/callout.typ": render-callout
#import "scope.typ": enter-scope

#let apply-theme(config, body) = {
  let theme = config.theme

  set text(
    font: theme.font,
    size: theme.text-size,
    fill: theme.foreground,
    lang: theme.lang,
    region: theme.region,
  )
  set par(
    justify: true,
    leading: 0.65em,
    spacing: 1.2em,
  )
  set heading(numbering: config.heading-numbering)
  set list(marker: [•])
  set raw(theme: none)
  set table(
    inset: (x: 6pt, y: 5pt),
    stroke: 0.5pt + theme.border,
    fill: (_, row) => if row == 0 { theme.surface } else { none },
  )

  show heading.where(level: 1): it => block(
    sticky: true,
    above: 1.6em,
    below: 0.65em,
    text(size: 16pt, weight: "bold", it),
  )
  show heading.where(level: 2): it => block(
    sticky: true,
    above: 1.4em,
    below: 0.55em,
    text(size: 13pt, weight: "bold", it),
  )
  show heading.where(level: 3): it => block(
    sticky: true,
    above: 1.2em,
    below: 0.45em,
    text(size: 11pt, weight: "bold", it),
  )
  show table.cell.where(y: 0): set text(weight: "bold")
  show figure.where(kind: table): set block(breakable: true)
  show figure.caption: set text(size: 9pt, fill: theme.muted)
  show outline.entry: set text(fill: theme.foreground)
  show link: set text(fill: theme.foreground)
  show quote: it => block(
    stroke: (left: 2pt + theme.border),
    inset: (left: 10pt),
    text(fill: theme.muted, it),
  )
  show raw.where(block: true): it => block(
    fill: theme.surface,
    stroke: 0.5pt + theme.border,
    radius: 2pt,
    inset: 8pt,
    it,
  )
  show <graphite-callout>: it => render-callout(
    it.value,
    theme,
    config.labels,
  )

  // The scope mark belongs here, not in `render-report`: what enables
  // `callout` is the show rule above, and it is `apply-theme` that installs
  // it. Marking it one layer above would make the P1 guard reject anyone who
  // uses `apply-theme` directly, which is exactly the case of
  // `tests/unit/theme-valid.typ`.
  enter-scope()
  body
}
