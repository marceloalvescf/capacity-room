#import "../../../graphite/src/config.typ": normalize
#import "../../../graphite/src/theme.typ": apply-theme
#import "../../../graphite/src/components/callout.typ": callout

#let config = normalize(title: [Teste visual])

#apply-theme(config, [
  = Título

  Um parágrafo corporativo com texto suficiente para validar a composição.

  ```typst
  #let status = "verificado"
  ```

  - Primeiro item
  - Segundo item

  #table(
    columns: (1fr, 1fr),
    table.header([Campo], [Valor]),
    [Ambiente], [Produção],
  )

  #callout(kind: "warning")[Validar antes da publicação.]

  #figure(
    rect(width: 3cm, height: 1cm),
    caption: [Elemento demonstrativo.],
  )
])
