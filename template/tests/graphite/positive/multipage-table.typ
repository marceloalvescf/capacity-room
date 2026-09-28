#import "../../../graphite/lib.typ": graphite-report
#show: graphite-report.with(title: [Tabela multipágina])

= Registros

#table(
  columns: (1fr, 4fr),
  table.header([ID], [Descrição]),
  ..range(120).map(number => (
    [#(number + 1)],
    [Registro corporativo para validação de quebra e repetição do cabeçalho.],
  )).flatten(),
)
