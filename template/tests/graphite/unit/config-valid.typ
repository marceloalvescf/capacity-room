#import "../../../graphite/src/config.typ": normalize

#let config = normalize(
  title: [Relatório],
  author: ("Ana", "Bruno"),
  labels: (outline: [Índice]),
  theme: (margin: 3cm, surface: luma(90%)),
)

#assert(config.title == [Relatório])
#assert(config.author == ("Ana", "Bruno"))
#assert(config.labels.outline == [Índice])
#assert(config.labels.warning == [Atenção])
#assert(config.theme.margin == 3cm)
#assert(config.theme.font == "Liberation Sans")
#assert(config.footer == auto)
[config-ok]
