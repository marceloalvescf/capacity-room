// Marks whether the current document is under `graphite-report`.
//
// Exists because of P1: `callout` does not render by itself, it emits a
// labelled `metadata` that `theme.typ` transforms. Outside the package's
// scope there is no show rule, the metadata produces no output, and the text
// vanished from the page without error or warning. The guard below turns
// that silence into a compilation failure.
//
// `render-report` marks the scope; the component checks at layout time.

#let in-graphite-scope = state("graphite-report:in-scope", false)

#let enter-scope() = in-graphite-scope.update(true)

#let require-scope(symbol) = context {
  if not in-graphite-scope.get() {
    panic(
      "graphite-report: `" + symbol + "` only works inside a document under "
        + "`graphite-report`; apply `#show: graphite-report.with(...)` "
        + "before using it",
    )
  }
}
