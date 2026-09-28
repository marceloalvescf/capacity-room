// Pins what Phase 6 measured in a real consumer report: how a machine
// identifier behaves when it breaks inside a cell, and what happens to what
// does not fit. The two tables below are the golden of P5 and T9 — no other
// fixture shows either of the two things.
//
// The 7.1 pt and the widths in centimeters are not decorative: they are the
// density and the slack the real consumer operates under, and that is what
// forces the break. Changing either one invalidates the proof.
#import "../../../graphite/lib.typ": graphite-report

#show: graphite-report.with(title: [Quebra de identificador])

#let ident = "aks-generalpool-29715314-vmss000001"

= Identificador que quebra no hífen

#block(width: 100%)[
  #set text(size: 7.1pt)
  #table(
    columns: (3.2cm, 3.2cm),
    table.header([Sem a receita], [Com a receita]),
    // `lang` inherited from the document, which the package fixes at pt:
    // Typst repeats the hyphen at the start of the next line. That is
    // correct convention for prose and misleading for a resource name — the
    // reader cannot tell one hyphen from two.
    [#text(hyphenate: false, ident)],
    // The recipe from the README: `lang: "en"` only on this text. It breaks
    // at the same points, without repeating the hyphen, and what gets
    // copied from the PDF stays the real name, byte for byte.
    [#text(hyphenate: false, lang: "en", ident)],
  )
]

= Cadeia sem ponto de quebra

Sem hífen não há onde quebrar. O Typst não avisa: a compilação sai limpa e o
conteúdo invade a célula vizinha, sobrescrevendo o que estiver lá.

#block(width: 100%)[
  #set text(size: 7.1pt)
  #table(
    columns: (2.2cm, 1fr),
    table.header([Nó], [Estado]),
    [#text(hyphenate: false, "aksgeneralpool29715314vmss000001")],
    [Pronto],
  )
]
