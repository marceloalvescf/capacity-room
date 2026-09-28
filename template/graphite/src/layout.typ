#import "theme.typ": apply-theme

#let visible-author(author) = if type(author) == array {
  author.join(", ")
} else {
  author
}

#let metadata-grid(config) = {
  let rows = ()
  if config.author != none {
    rows.push((config.labels.author, visible-author(config.author)))
  }
  if config.date != none {
    rows.push((
      config.labels.date,
      config.date.display(config.theme.date-format),
    ))
  }
  if config.classification != none {
    rows.push((config.labels.classification, config.classification))
  }

  if rows.len() > 0 {
    let cells = rows.map(((label, value)) => (
      text(size: 9pt, weight: "bold", label),
      text(size: 9pt, value),
    )).flatten()
    grid(
      columns: (auto, 1fr),
      column-gutter: 10pt,
      row-gutter: 4pt,
      ..cells,
    )
  }
}

#let brand-row(config) = {
  if config.organization != none and config.logo != none {
    grid(
      columns: (1fr, auto),
      column-gutter: 12pt,
      align: horizon,
      text(size: 9pt, weight: "bold", config.organization),
      config.logo,
    )
  } else if config.organization != none {
    text(size: 9pt, weight: "bold", config.organization)
  } else if config.logo != none {
    align(right, config.logo)
  }
}

#let compact-opening(config) = block(
  breakable: false,
  below: 1.4em,
  [
    #if config.organization != none or config.logo != none {
      brand-row(config)
      v(12pt)
    }
    #text(size: 22pt, weight: "bold", config.title)
    #if config.subtitle != none {
      v(5pt)
      text(size: 12pt, fill: config.theme.muted, config.subtitle)
    }
    #if config.author != none or config.date != none or config.classification != none {
      v(12pt)
      metadata-grid(config)
    }
    #v(12pt)
    #line(length: 100%, stroke: 0.8pt + config.theme.foreground)
  ],
)

#let render-report(config, body) = {
  let metadata-author = if config.author == none { () } else { config.author }

  set document(
    title: config.title,
    author: metadata-author,
    date: config.date,
  )
  set page(
    paper: config.theme.paper,
    flipped: false,
    margin: config.theme.margin,
    fill: white,
    numbering: "1",
    number-align: bottom + right,
    header: config.header,
    footer: config.footer,
  )

  apply-theme(config, [
    #compact-opening(config)
    #if config.outline {
      outline(title: config.labels.outline)
    }
    #body
  ])
}
