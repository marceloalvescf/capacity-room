#import "../../../graphite/lib.typ": graphite-report

// Before P3 the error came from `set page` inside `layout.typ`, without
// naming the key the consumer wrote.
#show: graphite-report.with(title: [Relatório], theme: (margin: (topo: 5cm)))

= Conteúdo
