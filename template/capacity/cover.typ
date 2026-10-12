// cover.typ builds the title band of the first page, passed to
// `graphite-report` as its `title`. Every word comes from `data.cover`; this
// module only arranges it: the title, the cluster, the period, and the
// profile on the left, and the author and the issue time on the right. The
// rule under the band is the shell's own.

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
    stack(
      spacing: 4pt,
      text(size: 18pt, weight: "bold", cover.title),
      {
        cover.cluster_label
        [ ]
        text(font: mono, hyphenate: false, lang: "en", cover.cluster)
        h(8pt)
        mono-numbers(cover.period)
        h(8pt)
        cover.profile
      },
    ),
    text(size: 8pt, fill: muted, {
      if cover.author != none [#cover.author_label #cover.author \ ]
      [#cover.issued_label #text(font: mono, cover.issued)]
    }),
  ))
}
