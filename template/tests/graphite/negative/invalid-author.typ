#import "../../../graphite/lib.typ": graphite-report

// The type of `author` was the only contract of `normalize()` without a
// negative case in the public API: it only existed in
// `tests/unit/config-invalid-type.typ`, which no test ran. See T1.
#show: graphite-report.with(title: [Relatório], author: 42)

= Conteúdo
