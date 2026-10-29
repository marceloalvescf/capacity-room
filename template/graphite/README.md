# Graphite Report

`graphite-report` is an opinionated Typst package/template for generic corporate reports. Content is written directly in Typst; the package applies a monochrome visual identity, a compact opening, metadata, and consistent pagination.

## Scope

This README documents the code as it stands.

The package offers:

- a small API with `graphite-report`, `callout`, and `keep-together`;
- A4 portrait, 2.5 cm margins, and Liberation Sans;
- justified text in Brazilian Portuguese;
- black and neutral grays;
- no header and a page number at the bottom right by default;
- local installation compatible with Typst's package structure;
- compilation and visual regression tests.

There is no Markdown conversion, DOCX generation, incident logic, automatic charts, identity of a specific organization, or publication on Typst Universe.

## Requirements

- Typst CLI `0.15.1`;
- the package folder, including `assets/fonts`.

There is nothing else. The package needs no installation step to be used inside this repository.

## Quick start

Compile an example using only the package's own fonts:

```bash
mkdir -p build
typst compile \
  --root "$PWD" \
  --font-path assets/fonts \
  --ignore-system-fonts \
  --ignore-embedded-fonts \
  examples/minimal.typ \
  build/minimal.pdf
```

Run this from inside `template/graphite`. The examples import the package by relative path, so no installation step is needed to compile them.

## Using the package in another document

Inside this repository, other Typst files import the package by relative path:

```typst
#import "../lib.typ": graphite-report
```

To use the package from another document, copy the package folder, without `examples/` and `README.md`, into `<data folder>/typst/packages/local/graphite-report/<version>/`. Then import it by name:

```typst
#import "@local/graphite-report:4.1.0": graphite-report

#show: graphite-report.with(
  title: [Relatório mensal],
)

= Resumo executivo

Escreva o conteúdo normalmente com elementos nativos do Typst.
```

There is no intermediate step or automatic conversion: the `.typ` file is the source of the document. [examples/minimal.typ](examples/minimal.typ) holds the same structure in a version ready to compile.

## Public API

The entry point exports only the three documented symbols below. Modules under `src/` are internal.

### `graphite-report`

Full usage:

```typst
#import "@local/graphite-report:4.1.0": (
  graphite-report,
  callout,
  keep-together,
)

#show: graphite-report.with(
  title: [Relatório mensal de operações],
  subtitle: [Resultados consolidados de julho de 2026],
  organization: [Empresa Exemplo],
  author: "Time de Operações",
  date: datetime(year: 2026, month: 7, day: 31),
  classification: [Uso interno],
  logo: none,
  outline: false,
  heading-numbering: "1.1",
  header: none,
  footer: auto,
  labels: (:),
  theme: (:),
)
```

| Parameter | Type | Default | Description |
|---|---|---|---|
| `title` | content | required | Visible title and PDF metadata. |
| `subtitle` | content or `none` | `none` | Subtitle of the compact opening. |
| `organization` | content or `none` | `none` | Organization shown at the top. |
| `author` | string, array of strings, or `none` | `none` | Visible author and PDF author metadata. |
| `date` | `datetime` or `none` | `none` | Visible date and PDF metadata; `none` avoids a non-deterministic current date. |
| `classification` | content or `none` | `none` | Visible classification. |
| `logo` | content or `none` | `none` | Ready content, usually `image(...)`; no logo is provided. |
| `outline` | boolean | `false` | Inserts the table of contents after the opening. |
| `heading-numbering` | string or `none` | `"1.1"` | Native heading numbering pattern. |
| `header` | `auto`, `none`, or content | `none` | Native or custom header. |
| `footer` | `auto`, `none`, or content | `auto` | `auto` shows only the page number at the right. |
| `labels` | dictionary | `(:)` | Controlled label overrides. |
| `theme` | dictionary | `(:)` | Controlled visual overrides. |

Optional values equal to `none` take no space in the opening.

### Labels

The automatic labels use pt-BR. Every value is of type content:

| Key | Default |
|---|---|
| `author` | Responsável |
| `date` | Emissão |
| `classification` | Classificação |
| `outline` | Sumário |
| `note` | Nota |
| `info` | Informação |
| `warning` | Atenção |

Example of a manual override:

```typst
labels: (
  outline: [Índice],
  author: [Elaboração],
)
```

Unknown keys raise an error. Translating the labels rarely suffices on its own: see `lang`, `region`, and `date-format` under *Controlled theme*. The package does not promise a complete localization system with per-language catalogs.

### Controlled theme

| Key | Accepted type | Default |
|---|---|---|
| `font` | string | `"Liberation Sans"` |
| `text-size` | length | `11pt` |
| `paper` | string | `"a4"` |
| `margin` | length, relative, or dictionary | `2.5cm` |
| `foreground` | color | black |
| `muted` | color | 40% luma |
| `border` | color | 75% luma |
| `surface` | color | 95% luma |
| `lang` | string | `"pt"` |
| `region` | string | `"BR"` |
| `date-format` | string | `"[day padding:zero]/[month padding:zero]/[year]"` |

Neutral example:

```typst
theme: (
  text-size: 10.5pt,
  margin: 2.8cm,
  muted: luma(35%),
  border: luma(70%),
  surface: luma(92%),
)
```

Unknown keys raise an error. Internal keys, component spacing, and opening measurements are not part of the public API.

`lang` and `region` feed Typst's hyphenation and typographic conventions; `date-format` is passed to `datetime.display`. Translating only the labels leaves the text hyphenating in Portuguese and the date in pt-BR order, so the three keys travel together with `labels`. A report entirely in English:

```typst
#show: graphite-report.with(
  title: [Monthly report],
  date: datetime(year: 2026, month: 7, day: 31),
  labels: (author: [Owner], date: [Issued]),
  theme: (
    lang: "en",
    region: "US",
    date-format: "[month padding:zero]/[day padding:zero]/[year]",
  ),
)
```

### `callout`

```typst
#callout(kind: "warning", title: [Decisão necessária])[
  Confirme os responsáveis antes da publicação.
]
```

| Parameter | Type | Default |
|---|---|---|
| `kind` | string | `"note"` |
| `title` | content or `none` | `none` |
| `breakable` | boolean | `false` |
| final body | content | required |

The kinds accepted in `kind` are `note`, `info`, and `warning`. They are told apart by label, shape, and typographic weight, without relying on color. Without `title`, the component uses the matching label from `labels`.

By default a callout **does not split across pages**: if it does not fit in what remains of the page, it moves down whole. Pass `breakable: true` to allow the split, which makes sense for long blocks, where moving down whole would leave too much empty space.

The component only renders inside a document under `graphite-report`. Used outside one, it stops the compile with an explanatory message instead of disappearing from the page.

### `keep-together`

```typst
#keep-together[
  == Próximos passos
  Este título e este conteúdo curto permanecem juntos.
]
```

The only parameter is the final body, of type content and required. Use it only for small blocks. Large, unbreakable content can overflow or leave excessive space at the end of the page.

## Native elements

Headings, lists, tables, figures, footnotes, links, labels, references, and `pagebreak()` remain native Typst elements.

Table with a semantic, repeatable header:

```typst
#table(
  columns: (2fr, 1fr),
  table.header([Indicador], [Valor]),
  [Disponibilidade], [99,8%],
  [Prazo médio], [4,6 dias],
)
```

### Recipe for dense tables

The theme styles the table at the density of body text, which is comfortable for a few columns and tight when the table is wide. A report with many columns usually needs a smaller type and a shorter line spacing, and the useful density varies from table to table: in a real consumer, five tables use five different sizes, between 6.2 pt and 7.5 pt, each calibrated for its number of columns.

That is why the package does not export a ready-made helper: a global value would not serve, and the recipe below has fourteen lines and no dependency on the package's internals. Copy it into your project and adjust it.

```typst
#let data-table(
  headers,
  rows,
  columns: auto,
  size: 8pt,
  inset-y: 3.5pt,
) = block(width: 100%)[
  #set text(size: size)
  #set par(justify: false, leading: 0.45em, spacing: 0.4em)
  #set table(inset: (x: 3pt, y: inset-y))
  #table(
    columns: columns,
    table.header(..headers),
    ..rows.flatten().map(item => [#item]),
  )
]

#data-table(
  ([Indicador], [Meta], [Realizado]),
  ((  [Disponibilidade], [99,5%], [99,8%]),),
  columns: (2fr, 1fr, 1fr),
  size: 7.2pt,
)
```

The theme already applies bold to the first row's cells, so the recipe does not need to repeat that. The header remains `table.header`, and therefore keeps repeating at every page break.

### Recipe: machine identifiers in tables

The package writes in Portuguese by default, and Typst applies that language's spelling convention: when a line break falls on a hyphen that already exists in the word, the hyphen is **repeated** at the start of the next line. For prose that is correct. For the name of a resource, host, or file it is misleading: `aks-generalpool-29715314-vmss000001` leaves the page as `aks-generalpool-29715314-` followed by `-vmss000001`, and the reader has no way to tell whether the name has one hyphen or two.

Declare the language **only on the identifier's text**:

```typst
#let ident(value) = text(hyphenate: false, lang: "en", value)

#table(
  columns: (3.2cm, 1fr),
  table.header([Nó], [Estado]),
  [#ident("aks-generalpool-29715314-vmss000001")], [Pronto],
)
```

The break still happens at the same points, the hyphen stops being repeated, and the text copied from the PDF stays the real name, byte for byte. Declaring `lang` on the whole document would also fix this, but it would also carry along the prose's hyphenation and the date format, which is exactly what the theme's `lang` key controls.

Three outputs that look like they would work and do not, measured before writing this recipe:

| Attempt | Why not |
|---|---|
| `hyphenate: false` alone | governs dictionary-based hyphenation, not the break on a hyphen already in the string; the hyphen keeps being repeated |
| wrapping in `box()` | does not prevent the internal break; the output is identical to doing nothing |
| replacing `-` with `‑` (U+2011) | stops the repetition because it forbids the break, but then the identifier overflows the cell, and whoever copies it from the PDF gets a character that is not part of the real name |

The fixture `template/tests/graphite/positive/identifier-wrapping.typ` exercises the recipe and the case without it side by side, and the golden page `identifiers-1.png` pins both outputs.

Accessible, referenceable figure:

```typst
#figure(
  image("assets/process-flow.svg", alt: "Fluxo em três etapas"),
  caption: [Ciclo mensal de acompanhamento.],
) <process-flow>

Consulte a @process-flow.
```

## Custom header and footer

Pass ready content in `header` and `footer`. Setting an explicit footer replaces the page's automatic numbering; include `counter(page).display("1")` in the custom content.

Generic example from the second page onward:

```typst
#let report-header = context {
  if counter(page).get().first() > 1 {
    grid(columns: (1fr, auto), [Área responsável], [Relatório mensal])
  }
}

#let report-footer = context {
  if counter(page).get().first() > 1 {
    grid(
      columns: (1fr, auto),
      [Classificação: Uso interno],
      counter(page).display("1"),
    )
  }
}

#show: graphite-report.with(
  title: [Relatório],
  header: report-header,
  footer: report-footer,
)
```

The matching runnable example is [examples/custom-running-elements.typ](examples/custom-running-elements.typ).

### Recipe: running elements from the second page

A common pattern in corporate reports: the first page stays clean, and from the second page onward the header carries the area and the logo, and the footer carries the classification and the page number.

The recipe below receives the logo as a parameter instead of hard-coding a path, which keeps it independent of the organization and of the directory layout of whoever uses it:

```typst
#let running-elements(logo, area: [Área responsável], classification: [Uso interno]) = (
  header: context {
    if counter(page).get().first() > 1 {
      grid(
        columns: (1fr, auto),
        column-gutter: 12pt,
        align: horizon,
        text(size: 8.5pt, weight: "bold", area),
        logo,
      )
    }
  },
  footer: context {
    if counter(page).get().first() > 1 {
      grid(
        columns: (1fr, auto),
        text(size: 8pt, [Classificação: #classification]),
        text(size: 8pt, counter(page).display("1")),
      )
    }
  },
)

#let running = running-elements(
  // O logo é conteúdo, não caminho: aqui vai um retângulo só para o exemplo
  // rodar. No seu projeto troque por
  // `image("assets/logo.svg", height: 0.8cm, alt: "Logotipo da organização")`,
  // e mantenha o `alt`, que é o que chega a tecnologia assistiva.
  rect(width: 1.6cm, height: 0.8cm, stroke: 0.5pt),
  area: [Operações],
)

#show: graphite-report.with(
  title: [Relatório],
  header: running.header,
  footer: running.footer,
)
```

The numbering is not reset: the first page is counted, it simply does not display the number, so the second page shows `2`.

**An organization's identity does not live in this package.** The logo, the area name, and the classification belong to the consuming project, and a company preset is a module of that project; nothing here needs to be imported to build one, as the recipe above demonstrates. The package does not ship a brand, logo, or named preset, and that is a declared non-goal, not an omission.

> The content of the `header` and `footer` regions is not exposed to assistive technology by Typst. Essential information should also exist in the body or in the document's metadata.

## Tests and visual regression

From the repository root:

```bash
make test-template
```

This runs the template's test suite, which compiles the unit files, examples, and fixtures, and checks that the negative fixtures fail with the expected diagnostic.

Golden pages are never changed by a test run. To propose a visual update:

```bash
make update-golden
```

Review each generated page, page by page, before committing it.

## Known limitations

- Text that does not fit in a table cell overflows without any warning, and can overwrite the next cell. A long string with no space and no hyphen is the usual case. Size the column for it; nothing checks this for you. The golden page `identifiers-1.png` shows the effect.
- A small table can split at the end of a page. Wrap it in `keep-together` when it must stay whole.
- `keep-together` does not measure or split large content.
- Header and footer content is not exposed to assistive technology, so anything essential must also appear in the body.
- The package controls only the documented keys. Deeper changes need a change to its API.
- Pages render the same only under Typst 0.15.1 with the fonts in `assets/fonts`.
- Liberation Sans has no glyphs for Chinese, Japanese, or Korean characters, among others. A character with no glyph prints as an empty box.

## Licenses

The project's code uses the MIT license, available at [LICENSE](LICENSE).

The Liberation Sans files under `assets/fonts` use the upstream license preserved at [assets/fonts/LICENSE](assets/fonts/LICENSE); that license does not replace the project's code license.
