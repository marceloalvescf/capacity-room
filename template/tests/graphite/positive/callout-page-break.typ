#import "../../../graphite/lib.typ": graphite-report, callout

// The text pushes the callout to the edge of the page on purpose. Without
// passing `breakable`, the default applies, and the page 1 golden pins the
// result: the whole block moves down instead of splitting. Calibrated —
// below lorem(380) both modes produce the same page and the fixture would
// prove nothing. See F5.
#show: graphite-report.with(title: [Callout na borda da página])

= Conteúdo

#lorem(420)

#callout(kind: "warning", title: [Bloco longo])[
  #lorem(140)
]
