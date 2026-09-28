#import "../../../graphite/src/config.typ": normalize
#import "../../../graphite/src/theme.typ": apply-theme
#import "../../../graphite/src/components/callout.typ": callout
#import "../../../graphite/src/components/keep-together.typ": keep-together

// The components are exercised under `apply-theme`, which is what installs
// the callout's show rule. Before P1 this fixture called them out of scope,
// compiled clean, and the three callouts vanished from the page without
// anyone noticing — see T2.
#let config = normalize(title: [Componentes])

#apply-theme(config, [
  #callout(kind: "note")[Uma observação.]
  #callout(kind: "info", title: [Contexto])[Informação adicional.]
  #callout(kind: "warning")[Requer atenção.]

  #keep-together[
    *Decisão:* manter este conteúdo unido.
  ]

  [components-ok]
])
