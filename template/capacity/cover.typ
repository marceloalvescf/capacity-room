// cover.typ builds the report cover, passed to `graphite-report` as its
// `title`. Reproduces `report-cover` from the original Python renderer
// (older/kubernetes-report/kube_capacity_report/renderers/sections.py,
// lines 92-110). Every word comes from `data.cover`; this module only
// arranges it. `graphite-report` wraps the whole thing in its own 22pt
// bold text, exactly as the original did, so the title line below repeats
// that styling rather than relying on it.

#let build-cover(cover) = align(center)[
  #text(size: 22pt, weight: "bold", cover.title)
  #v(5pt)
  #text(size: 12pt, weight: "regular")[
    #cover.cluster_label #emph(text(hyphenate: false, lang: "en", cover.cluster))
  ]
  #v(12pt)
  #if cover.author != none [
    #text(size: 9pt, weight: "regular")[#text(weight: "bold", cover.author_label) #cover.author]
    #v(4pt)
  ]
  #text(size: 9pt, weight: "regular")[#text(weight: "bold", cover.issued_label) #cover.issued]
]
