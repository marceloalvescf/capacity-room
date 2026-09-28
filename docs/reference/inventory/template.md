# Inventory: graphite-report 1.0.0 (read directly, 2026-09-27)

Source: older/graphite-report, commit aea51af (v1.0.0 plus two documentation commits)

## Files that make the package
typst.toml (name graphite-report, version 1.0.0, entrypoint lib.typ, compiler 0.15.0, categories report, [template] path template, thumbnail)
lib.typ (three exports; imports src lazily inside the function body)
src/config.typ, src/layout.typ, src/theme.typ, src/scope.typ, src/components/callout.typ, src/components/keep-together.typ
assets/fonts/LiberationSans-{Regular,Bold,Italic,BoldItalic}.ttf + LICENSE (SIL Open Font License 1.1)
template/main.typ (starter for `typst init`), thumbnail.png, LICENSE (MIT)
Total Typst source: about 14 kB in nine files.

## Public surface
graphite-report(title, subtitle, organization, author, date, classification, logo, outline false, heading-numbering "1.1", header none, footer auto, labels, theme, body)
callout(kind "note", title none, breakable false, body); kinds note, info, warning
keep-together(body)

## Defaults
labels: author Responsável, date Emissão, classification Classificação, outline Sumário, note Nota, info Informação, warning Atenção
theme: font "Liberation Sans", text-size 11pt, paper "a4", margin 2.5cm, foreground black, muted luma(40%), border luma(75%), surface luma(95%), lang "pt", region "BR", date-format "[day padding:zero]/[month padding:zero]/[year]"

## Validation (src/config.typ)
- normalize() validates every argument and returns the merged configuration; layout and theme trust it.
- Unknown key in labels, theme, or theme.margin is a panic naming the key and listing the allowed ones.
- theme.paper is checked for type only; the name is left to Typst on purpose.
- callout and keep-together validate their own arguments.

## Scope guard (src/scope.typ)
state "graphite-report:in-scope"; apply-theme sets it; callout calls require-scope, which panics at layout time outside a graphite-report document.
This message is the only diagnostic written in Portuguese:
"graphite-report: `callout` só funciona dentro de um documento sob `graphite-report`; aplique `#show: graphite-report.with(...)` antes de usá-lo"

## Callout protocol
callout() validates, checks scope, and emits metadata((kind, title, body, breakable)) labelled <graphite-callout>.
theme.typ has `show <graphite-callout>` calling render-callout(payload, theme, labels).
Markers: note "●", info "■", warning "▲". Block: surface fill, 2pt left rule in foreground, 0.5pt border elsewhere, radius 2pt, inset 10pt, above and below 1.2em.

## Layout (src/layout.typ)
set document(title, author, date); set page(paper, margin, fill white, numbering "1", number-align bottom + right, header, footer)
Compact opening, unbreakable: brand row (organization 9pt bold, logo) -> title 22pt bold -> subtitle 12pt muted -> metadata grid 9pt (author, date, classification) -> 0.8pt rule.
Fields equal to none take no space. Optional outline after the opening.

## Theme (src/theme.typ)
text(font, size, fill, lang, region); par(justify true, leading 0.65em, spacing 1.2em); heading numbering; list marker •; raw theme none
table(inset x 6pt y 5pt, stroke 0.5pt border, header row filled with surface); first row bold
headings: level 1 16pt, level 2 13pt, level 3 11pt, all bold and sticky
figure tables breakable; captions 9pt muted; links and outline entries in foreground; quote with left rule; raw blocks on surface with border

## All diagnostics (12 negative fixtures)
`title` must be content; received none
unknown theme key `accent`
unknown labels key `summary`
invalid callout kind "success"; allowed kinds:
`header` must be auto, none, or content
`footer` must be auto, none, or content
`callout.breakable` must be a boolean
`theme.lang` must be a string
unknown theme.margin key `topo`
`theme.margin.top` must be a length or relative value
`author` must be a string, array of strings, or none
`callout` só funciona dentro de um documento   (scope guard)
Every message starts with "graphite-report: ".

## Tests (Python, pytest)
- toolchain: pinned version, four font faces exist and are not empty, `typst fonts` lists Liberation Sans, the four deterministic flags
- compilation: 3 unit files, 4 examples + starter, 10 positive fixtures; exit 0, no "warning:" in stderr, PDF not empty
- diagnostics: 12 negative fixtures; expected fragments live in a table inside the test module
- visual: 10 snapshots, 13 pages, 144 PPI, compared by SHA-256 against tests/golden/SHA256SUMS
  complete (2 pages), minimal, theme, running (2), table (2), template, callout-break, locale, components, identifiers
  each snapshot states the one contract it alone protects
- monochrome: tests/unit/theme-valid.typ rendered and decoded; no pixel may have colour (Typst always writes RGBA, so the check reads pixels, not the encoding)
- consistency: both manifests same version; every @local identifier uses it; installed folder holds only package entries; every top-level entry classified
- readme: the first typst block of each "### Receita" section compiles cleanly; the list of recipe titles is pinned (3 recipes)
- a difference image is written only when Pillow is installed

## Recipes in the README
dense tables (data-table helper); machine identifiers in tables (text with hyphenate false and lang "en"); running elements from the second page

## Deferred in the original and not carried over
Publication to Typst Universe and the @preview address (F1, F7, F8). PDF/A or PDF/UA validation was measured and dismissed (F6): Typst emits tagged PDF by default.

## How kubernetes-report uses it
Only title (a custom cover block), outline false, and a custom footer. No header, labels, or theme passed. The cover block becomes the PDF metadata title.
Its data-table helper overrides the theme's table inset and stroke.
