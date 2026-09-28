#let default-labels = (
  author: [Responsável],
  date: [Emissão],
  classification: [Classificação],
  outline: [Sumário],
  note: [Nota],
  info: [Informação],
  warning: [Atenção],
)

#let default-theme = (
  font: "Liberation Sans",
  text-size: 11pt,
  paper: "a4",
  margin: 2.5cm,
  foreground: black,
  muted: luma(40%),
  border: luma(75%),
  surface: luma(95%),
  // The three below complete what `labels` started. Translating the labels
  // without being able to touch this left the report hyphenating as
  // Portuguese and printing the date in pt-BR order — see F2.
  lang: "pt",
  region: "BR",
  date-format: "[day padding:zero]/[month padding:zero]/[year]",
)

#let fail-type(name, expected, value) = {
  let received = if value == none { repr(value) } else { repr(type(value)) }
  panic(
    "graphite-report: `" + name + "` must be " + expected
      + "; received " + received,
  )
}

#let validate-keys(name, value, allowed) = {
  if type(value) != dictionary {
    fail-type(name, "a dictionary", value)
  }
  for key in value.keys() {
    if not allowed.contains(key) {
      panic(
        "graphite-report: unknown " + name + " key `" + key
          + "`; allowed keys: " + allowed.join(", "),
      )
    }
  }
}

#let validate-optional-content(name, value) = {
  if value != none and type(value) != content {
    fail-type(name, "content or none", value)
  }
}

// The keys that Typst's `page(margin:)` accepts. A small and stable set,
// unlike the `paper` list, and that is why it is worth checking here: the
// error then names `theme.margin` instead of pointing inside `layout.typ`.
#let margin-keys = (
  "left",
  "top",
  "right",
  "bottom",
  "outside",
  "inside",
  "x",
  "y",
  "rest",
)

#let validate-theme(theme) = {
  validate-keys("theme", theme, default-theme.keys())
  let merged = default-theme + theme

  if type(merged.font) != str {
    fail-type("theme.font", "a string", merged.font)
  }
  if type(merged.text-size) != length {
    fail-type("theme.text-size", "a length", merged.text-size)
  }
  // `theme.paper` stops here on purpose. Typst validates the name against its
  // own list, which has more than a hundred entries and changes between
  // compiler versions; duplicating it here just to change the error text
  // would be debt that rots. The native message enumerates all accepted
  // names, which no message of ours would do better. Declared exception to
  // the boundary invariant.
  if type(merged.paper) != str {
    fail-type("theme.paper", "a string", merged.paper)
  }
  for key in ("lang", "region", "date-format") {
    if type(merged.at(key)) != str {
      fail-type("theme." + key, "a string", merged.at(key))
    }
  }
  if not (type(merged.margin) in (length, relative, dictionary)) {
    fail-type("theme.margin", "a length, relative value, or dictionary", merged.margin)
  }
  if type(merged.margin) == dictionary {
    validate-keys("theme.margin", merged.margin, margin-keys)
    for (key, value) in merged.margin {
      if not (type(value) in (length, relative)) {
        fail-type("theme.margin." + key, "a length or relative value", value)
      }
    }
  }
  for key in ("foreground", "muted", "border", "surface") {
    let value = merged.at(key)
    if type(value) != color {
      fail-type("theme." + key, "a color", value)
    }
  }
  merged
}

#let normalize(
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
) = {
  if type(title) != content {
    fail-type("title", "content", title)
  }
  validate-optional-content("subtitle", subtitle)
  validate-optional-content("organization", organization)
  validate-optional-content("classification", classification)
  validate-optional-content("logo", logo)

  let valid-author = author == none or type(author) == str or (
    type(author) == array and author.all(item => type(item) == str)
  )
  if not valid-author {
    fail-type("author", "a string, array of strings, or none", author)
  }
  if date != none and type(date) != datetime {
    fail-type("date", "a datetime or none", date)
  }
  if type(outline) != bool {
    fail-type("outline", "a boolean", outline)
  }
  if heading-numbering != none and type(heading-numbering) != str {
    fail-type("heading-numbering", "a string or none", heading-numbering)
  }
  for (name, value) in (("header", header), ("footer", footer)) {
    if value != none and value != auto and type(value) != content {
      fail-type(name, "auto, none, or content", value)
    }
  }

  validate-keys("labels", labels, default-labels.keys())
  let normalized-labels = default-labels + labels
  for (key, value) in normalized-labels {
    if type(value) != content {
      fail-type("labels." + key, "content", value)
    }
  }

  (
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
    labels: normalized-labels,
    theme: validate-theme(theme),
  )
}
