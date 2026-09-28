#import "../lib.typ": graphite-report

#show: graphite-report.with(
  title: [Relatório com ajustes controlados],
  author: "Equipe de Planejamento",
  theme: (
    text-size: 10.5pt,
    margin: 2.8cm,
    muted: luma(35%),
    border: luma(70%),
    surface: luma(92%),
  ),
)

= Objetivo

Este exemplo altera somente chaves documentadas e preserva a identidade monocromática.
