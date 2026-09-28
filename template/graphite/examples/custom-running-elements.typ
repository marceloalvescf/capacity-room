#import "../lib.typ": graphite-report

#let report-header = context {
  if counter(page).get().first() > 1 {
    grid(
      columns: (1fr, auto),
      [Operações corporativas],
      [Relatório mensal],
    )
  }
}

#let report-footer = context {
  if counter(page).get().first() > 1 {
    grid(
      columns: (1fr, auto),
      [Classificação: Uso interno],
      counter(page).display("1"),
    )
  }
}

#show: graphite-report.with(
  title: [Relatório com elementos correntes],
  header: report-header,
  footer: report-footer,
)

= Conteúdo

#lorem(547)
