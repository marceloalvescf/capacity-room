#import "../lib.typ": (
  graphite-report,
  callout,
  keep-together,
)

#show: graphite-report.with(
  title: [Relatório mensal de operações],
  subtitle: [Resultados consolidados de julho de 2026],
  organization: [Empresa Exemplo],
  author: ("Ana Souza", "Bruno Lima"),
  date: datetime(year: 2026, month: 7, day: 31),
  classification: [Uso interno],
  outline: true,
)

= Resumo executivo

O período encerrou com estabilidade operacional e conclusão das ações prioritárias.

#callout(kind: "info")[
  Os valores apresentados foram consolidados no fechamento de 31/07/2026.
]

= Indicadores

#figure(
  table(
    columns: (2fr, 1fr, 1fr),
    table.header([Indicador], [Meta], [Realizado]),
    [Disponibilidade], [99,5%], [99,8%],
    [Demandas concluídas], [120], [127],
    [Prazo médio], [5 dias], [4,6 dias],
  ),
  caption: [Indicadores consolidados do período.],
) <indicators>

Conforme a @indicators, todas as metas principais foram atingidas.

= Processo de acompanhamento

#figure(
  image(
    "assets/process-flow.svg",
    width: 90%,
    alt: "Fluxo corporativo com as etapas planejar, executar e revisar.",
  ),
  caption: [Ciclo utilizado no acompanhamento mensal.],
) <process-flow>

O fluxo da @process-flow organiza a revisão recorrente das entregas.

#callout(kind: "warning", title: [Decisão necessária])[
  Confirmar os responsáveis pelo próximo ciclo até 5 de agosto.
]

#keep-together[
  == Próximos passos

  - Publicar o plano consolidado.
  - Comunicar responsáveis e prazos.
]

= Apêndice

#lorem(220)
