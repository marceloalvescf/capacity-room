// cover.typ builds the title band of the first page, passed to
// `graphite-report` as its `title`. Every word comes from `data.cover`; this
// module only arranges it: the title, the cluster, and the period on the
// left, and the profile, the author, and the issue time on the right. The
// cluster and the period are each kept whole, so a long line wraps between
// them and never inside a word. The rule under the band is the shell's own.

#import "runs.typ": mono

#let muted = luma(35%)

// The period reads as words with every number in the monospaced face.
#let mono-numbers(body) = {
  show regex("[0-9][0-9/:. ]*[0-9]"): it => text(font: mono, it)
  body
}

#let build-cover(cover) = {
  set text(size: 9pt, weight: "regular")
  set par(leading: 0.5em, spacing: 0.5em)
  block(above: 0pt, below: 0pt, grid(
    columns: (1fr, auto),
    align: (left + bottom, right + bottom),
    column-gutter: 16pt,
    stack(
      spacing: 10pt,
      text(size: 18pt, weight: "bold", cover.title),
      par(justify: false, {
        box[#cover.cluster_label #text(font: mono, hyphenate: false, lang: "en", cover.cluster)]
        // The gap disappears where the line wraps; the zero-width space is
        // the one place it may wrap.
        h(10pt, weak: true)
        sym.zws
        box(mono-numbers(cover.period))
      }),
    ),
    text(size: 8pt, fill: muted, hyphenate: false, {
      [#cover.profile \ ]
      if cover.author != none [#cover.author_label #cover.author \ ]
      [#cover.issued_label #text(font: mono, cover.issued)]
    }),
  ))
}
