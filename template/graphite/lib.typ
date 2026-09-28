#let graphite-report(
  title: none,
  subtitle: none,
  organization: none,
  author: none,
  date: none,
  classification: none,
  logo: none,
  outline: false,
  heading-numbering: "1.1",
  header: none,
  footer: auto,
  labels: (:),
  theme: (:),
  body,
) = {
  import "src/config.typ" as config-module
  import "src/layout.typ" as layout-module

  let config = config-module.normalize(
    title: title,
    subtitle: subtitle,
    organization: organization,
    author: author,
    date: date,
    classification: classification,
    logo: logo,
    outline: outline,
    heading-numbering: heading-numbering,
    header: header,
    footer: footer,
    labels: labels,
    theme: theme,
  )
  layout-module.render-report(config, body)
}

#let callout = {
  import "src/components/callout.typ" as callout-module
  callout-module.callout
}

#let keep-together = {
  import "src/components/keep-together.typ" as keep-module
  keep-module.keep-together
}
