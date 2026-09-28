#import "../scope.typ": require-scope

#let allowed-kinds = ("note", "info", "warning")

#let callout(kind: "note", title: none, breakable: false, body) = {
  if type(kind) != str or not allowed-kinds.contains(kind) {
    panic(
      // No backticks around it: `repr` already returns the value in quotes,
      // and wrapping it produced `"success"` with two delimiters. See P4.
      "graphite-report: invalid callout kind " + repr(kind)
        + "; allowed kinds: " + allowed-kinds.join(", "),
    )
  }
  if title != none and type(title) != content {
    panic("graphite-report: `callout.title` must be content or none")
  }
  if type(body) != content {
    panic("graphite-report: `callout.body` must be content")
  }
  if type(breakable) != bool {
    panic("graphite-report: `callout.breakable` must be a boolean")
  }

  // Without the show rule from `theme.typ` this metadata produces no output
  // at all, and the callout would silently disappear from the page. The
  // guard fails at layout time when there is no scope. See `src/scope.typ`.
  require-scope("callout")

  let payload = (kind: kind, title: title, body: body, breakable: breakable)
  [#metadata(payload) <graphite-callout>]
}

#let render-callout(payload, theme, labels) = {
  let marker = (
    note: "●",
    info: "■",
    warning: "▲",
  ).at(payload.kind)
  let heading = if payload.title == none {
    labels.at(payload.kind)
  } else {
    payload.title
  }

  // `breakable: false` by default. The real consumer wrapped 17 of 17
  // callouts in `keep-together` to achieve this, and a line repeated 17
  // times is a missing parameter — see F5.
  block(
    width: 100%,
    breakable: payload.breakable,
    fill: theme.surface,
    stroke: (
      left: 2pt + theme.foreground,
      top: 0.5pt + theme.border,
      right: 0.5pt + theme.border,
      bottom: 0.5pt + theme.border,
    ),
    radius: 2pt,
    inset: 10pt,
    above: 1.2em,
    below: 1.2em,
    grid(
      columns: (auto, 1fr),
      column-gutter: 8pt,
      align: top,
      text(weight: "bold", marker),
      [
        #text(weight: "bold", heading)
        #v(4pt)
        #payload.body
      ],
    ),
  )
}
