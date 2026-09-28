// validate.typ is the only file of the capacity-report layer that calls
// `panic`. Every diagnostic is produced by `fail`, which looks up the
// fixed part of its message in `diagnostics`. Everything else in
// `template/capacity/` calls `fail`, never `panic` directly, so that a
// message can never drift from this one dictionary.
//
// The strict checking itself also lives here: `validate(data)` walks the
// whole JSON document once, and every other module (`cover.typ`,
// `blocks.typ`, `runs.typ`, `tables.typ`) trusts the result without
// checking it again.
//
// Every field is read only after two things are confirmed, in order: the
// key exists (`missing-key`), and its value has the right type
// (`wrong-type`). Only then does a check reach for the value itself, and
// by that point its type is known, so formatting it (with a plain string
// concatenation, or `str()` for a confirmed `int`) can never crash. A
// value whose type is right but content is wrong is quoted with `repr()`
// instead, never `str()`.

#import "tables.typ": layouts

// diagnostics:start
#let diagnostics = (
  missing-input: "missing input \"data\"",
  unsupported-schema: "unsupported schema",
  unknown-key: "unknown key",
  missing-key: "missing key",
  wrong-type: "`",
  unknown-language: "unknown language",
  no-blocks: "`blocks` must hold at least one block",
  unknown-block-type: "unknown block type",
  heading-level: "heading level must be 1, 2, or 3; received",
  unknown-table: "unknown table id",
  header-count: "table",
  no-rows: "table",
  cell-count: "table",
  unknown-run-kind: "unknown run kind",
  unknown-status: "unknown status",
  empty-runs: "`runs` must not be empty at",
  empty-list: "`items` must not be empty at",
  misplaced-front: "block \"front\" is allowed only as the first block",
  nested-block: "block",
  duplicate-outline: "block \"outline\" is allowed only once; found again at",
)
// diagnostics:end

#let fail(name, detail) = panic(
  "capacity-report: " + diagnostics.at(name) + detail,
)

// type-name is safe for any value, including `none` and booleans:
// `type(value)` never fails, and `str()` accepts a `type`.
#let type-name(value) = str(type(value))

// check-type is the building block behind every `wrong-type` diagnostic:
// the fixed fragment kept in the dictionary is just the opening backtick,
// so `detail` supplies the path, the closing backtick, and everything
// after it, in that order.
#let check-type(path, expected, value, predicate) = {
  if not predicate(value) {
    fail("wrong-type", path + "` must be " + expected + "; received " + type-name(value))
  }
}

#let expect-string(path, value) = check-type(path, "a string", value, v => type(v) == str)
#let expect-bool(path, value) = check-type(path, "a boolean", value, v => type(v) == bool)
#let expect-array(path, value) = check-type(path, "an array", value, v => type(v) == array)
#let expect-dict(path, value) = check-type(path, "a dictionary", value, v => type(v) == dictionary)
#let expect-string-or-none(path, value) = check-type(
  path,
  "a string or null",
  value,
  v => v == none or type(v) == str,
)

// check-exact-keys backs every `unknown-key` and `missing-key` diagnostic,
// for every object of the data file: the report itself, `theme`,
// `labels`, `cover`, `footer`, every block (including one nested inside
// `front`), and every run (wherever it appears).
#let check-exact-keys(place, value, allowed) = {
  expect-dict(place, value)
  for key in value.keys() {
    if not (key in allowed) {
      fail(
        "unknown-key",
        " \"" + key + "\" in " + place + "; allowed keys: " + allowed.join(", "),
      )
    }
  }
  for key in allowed {
    if not (key in value.keys()) {
      fail("missing-key", " \"" + key + "\" in " + place)
    }
  }
}

#let allowed-top-keys = ("schema", "language", "theme", "labels", "cover", "footer", "blocks")
// `author`, `date`, `classification`, and `outline` were removed: the
// report layer never passes them (or `outline: true`) to `graphite-report`,
// the only place that would render them, so they reached no page. See the
// final review fix report for this round.
#let label-keys = ("note", "info", "warning")
#let cover-keys = (
  "title",
  "cluster_label",
  "cluster",
  "author_label",
  "author",
  "issued_label",
  "issued",
)
#let footer-keys = ("classification", "page_label")

#let allowed-run-kinds = ("text", "strong", "ident", "placeholder", "status")
#let allowed-statuses = ("healthy", "attention", "critical", "undefined", "unavailable")

// allowed-run-keys-for-kind: `text`, `strong`, `ident`, and `placeholder`
// runs allow only `kind` and `text`; `status` runs also allow `status`.
#let allowed-run-keys-for-kind = (
  text: ("kind", "text"),
  strong: ("kind", "text"),
  ident: ("kind", "text"),
  placeholder: ("kind", "text"),
  status: ("kind", "status", "text"),
)

// validate-run checks one run object, wherever it appears (a heading's,
// paragraph's, or callout's `runs`, a callout `title`, a list item, or a
// table cell). `path` is that run's own place: it names the run for
// `missing-key`, `unknown-key`, `wrong-type`, `unknown-run-kind`, and
// `unknown-status` alike. The caller is responsible for the `empty-runs`
// or `empty-list` check on the array that holds the run, since an empty
// table cell is legal.
#let validate-run(path, run) = {
  expect-dict(path, run)
  if not ("kind" in run.keys()) {
    fail("missing-key", " \"kind\" in " + path)
  }
  let kind = run.kind
  if type(kind) != str {
    fail("wrong-type", path + ".kind` must be a string; received " + type-name(kind))
  }
  if not (kind in allowed-run-kinds) {
    fail("unknown-run-kind", " \"" + kind + "\" at " + path)
  }
  // `kind` is now a confirmed, known string: safe to use as a dictionary
  // key and to interpolate directly, with no further conversion.
  check-exact-keys(path, run, allowed-run-keys-for-kind.at(kind))

  if kind == "status" {
    let status = run.status
    if type(status) != str {
      fail("wrong-type", path + ".status` must be a string; received " + type-name(status))
    }
    if not (status in allowed-statuses) {
      fail("unknown-status", " \"" + status + "\" at " + path)
    }
    expect-string-or-none(path + ".text", run.text)
  } else {
    expect-string(path + ".text", run.text)
  }
}

// validate-runs checks a non-empty array of runs (a heading's, a
// paragraph's, or a callout body's `runs`). `path` names the array itself
// for the `empty-runs` diagnostic, and each run for the others.
#let validate-runs(path, runs) = {
  expect-array(path, runs)
  if runs.len() == 0 {
    fail("empty-runs", " " + path)
  }
  for (i, run) in runs.enumerate() {
    validate-run(path + "[" + str(i) + "]", run)
  }
}

#let allowed-callout-kinds = ("note", "info", "warning")

#let validate-outline-block(place, block) = {
  check-exact-keys(place, block, ("type", "title", "depth"))
  expect-string(place + ".title", block.title)
  let depth = block.depth
  if type(depth) != int {
    fail("wrong-type", place + ".depth` must be 1, 2, or 3; received " + type-name(depth))
  }
  if not (depth in (1, 2, 3)) {
    fail("wrong-type", place + ".depth` must be 1, 2, or 3; received " + repr(depth))
  }
}

#let validate-heading-block(place, block) = {
  check-exact-keys(place, block, ("type", "level", "numbered", "runs"))
  let level = block.level
  if type(level) != int {
    fail("wrong-type", place + ".level` must be 1, 2, or 3; received " + type-name(level))
  }
  if not (level in (1, 2, 3)) {
    fail("heading-level", " " + str(level) + " at " + place)
  }
  let numbered = block.numbered
  if type(numbered) != bool {
    fail("wrong-type", place + ".numbered` must be a boolean; received " + type-name(numbered))
  }
  validate-runs(place + ".runs", block.runs)
}

#let validate-paragraph-block(place, block) = {
  check-exact-keys(place, block, ("type", "runs"))
  validate-runs(place + ".runs", block.runs)
}

#let validate-list-block(place, block) = {
  check-exact-keys(place, block, ("type", "items"))
  expect-array(place + ".items", block.items)
  if block.items.len() == 0 {
    fail("empty-list", " " + place)
  }
  for (m, item) in block.items.enumerate() {
    let item-path = place + ".items[" + str(m) + "]"
    expect-array(item-path, item)
    if item.len() == 0 {
      fail("empty-list", " " + item-path)
    }
    for (k, run) in item.enumerate() {
      validate-run(item-path + "[" + str(k) + "]", run)
    }
  }
}

#let validate-table-block(place, block) = {
  check-exact-keys(place, block, ("type", "id", "headers", "rows"))
  let id = block.id
  expect-string(place + ".id", id)
  if not (id in layouts.keys()) {
    fail("unknown-table", " \"" + id + "\" at " + place)
  }
  let columns = layouts.at(id).columns.len()

  expect-array(place + ".headers", block.headers)
  for (i, header) in block.headers.enumerate() {
    let header-path = place + ".headers[" + str(i) + "]"
    expect-string(header-path, header)
    // A header of only white space is indistinguishable, on the page,
    // from one that lost its label entirely.
    if header.trim() == "" {
      fail("wrong-type", header-path + "` must be a non-empty string; received " + repr(header))
    }
  }
  if block.headers.len() != columns {
    fail(
      "header-count",
      " \"" + id + "\" has " + str(columns) + " columns but " + str(block.headers.len()) + " headers",
    )
  }

  expect-array(place + ".rows", block.rows)
  if block.rows.len() == 0 {
    fail("no-rows", " \"" + id + "\" at " + place + " has no rows")
  }
  for (r, row) in block.rows.enumerate() {
    let row-path = place + ".rows[" + str(r) + "]"
    expect-array(row-path, row)
    if row.len() != columns {
      fail(
        "cell-count",
        " \"" + id + "\" has " + str(columns) + " columns but row " + str(r + 1) + " has " + str(row.len()) + " cells",
      )
    }
    for (c, cell) in row.enumerate() {
      let cell-path = row-path + "[" + str(c) + "]"
      expect-array(cell-path, cell)
      for (k, run) in cell.enumerate() {
        validate-run(cell-path + "[" + str(k) + "]", run)
      }
    }
  }
}

#let validate-callout-block(place, block) = {
  check-exact-keys(place, block, ("type", "kind", "title", "runs"))
  let kind = block.kind
  if type(kind) != str {
    fail("wrong-type", place + ".kind` must be a string; received " + type-name(kind))
  }
  if not (kind in allowed-callout-kinds) {
    fail(
      "wrong-type",
      place + ".kind` must be \"note\", \"info\", or \"warning\"; received " + repr(kind),
    )
  }
  let title = block.title
  if title != none {
    expect-array(place + ".title", title)
    for (i, run) in title.enumerate() {
      validate-run(place + ".title[" + str(i) + "]", run)
    }
  }
  validate-runs(place + ".runs", block.runs)
}

#let validate-pagebreak-block(place, block) = {
  check-exact-keys(place, block, ("type", "weak"))
  expect-bool(place + ".weak", block.weak)
}

#let allowed-top-block-types = (
  "outline", "front", "heading", "paragraph", "list", "table", "callout", "pagebreak",
)
#let allowed-front-block-types = ("outline", "heading", "paragraph", "list", "table", "callout")

// validate-block checks one entry of `blocks` (`in-front: false`) or of a
// `front` block's own `blocks` (`in-front: true`). It recurses into itself
// for `front`'s content: Typst closures capture the scope at definition
// time, so a function can call itself but not a sibling defined afterward
// — this is why `front` is handled inline here rather than through a
// separate `validate-front-block` that would need to call this one back.
#let validate-block(index, block, in-front) = {
  let place = if in-front {
    "blocks[0].blocks[" + str(index) + "]"
  } else {
    "blocks[" + str(index) + "]"
  }
  expect-dict(place, block)
  if not ("type" in block.keys()) {
    fail("missing-key", " \"type\" in " + place)
  }
  let block-type = block.type
  if type(block-type) != str {
    fail("wrong-type", place + ".type` must be a string; received " + type-name(block-type))
  }

  if in-front and block-type in ("front", "pagebreak") {
    fail("nested-block", " \"" + block-type + "\" is not allowed inside \"front\" at " + place)
  }

  let allowed-here = if in-front { allowed-front-block-types } else { allowed-top-block-types }
  if not (block-type in allowed-here) {
    fail("unknown-block-type", " \"" + block-type + "\" at " + place)
  }

  if block-type == "front" and index != 0 {
    fail("misplaced-front", "")
  }

  if block-type == "outline" {
    validate-outline-block(place, block)
  } else if block-type == "front" {
    check-exact-keys(place, block, ("type", "blocks"))
    expect-array(place + ".blocks", block.blocks)
    if block.blocks.len() == 0 {
      fail("no-blocks", "")
    }
    for (j, inner) in block.blocks.enumerate() {
      validate-block(j, inner, true)
    }
  } else if block-type == "heading" {
    validate-heading-block(place, block)
  } else if block-type == "paragraph" {
    validate-paragraph-block(place, block)
  } else if block-type == "list" {
    validate-list-block(place, block)
  } else if block-type == "table" {
    validate-table-block(place, block)
  } else if block-type == "callout" {
    validate-callout-block(place, block)
  } else {
    validate-pagebreak-block(place, block)
  }
}

// validate checks the whole document against schema "1" and returns it
// unchanged so the rest of the layer can read it without checking it
// again.
#let validate(data) = {
  check-exact-keys("report", data, allowed-top-keys)

  expect-string("schema", data.schema)
  if data.schema != "1" {
    fail("unsupported-schema", " \"" + data.schema + "\"; this template reads schema \"1\"")
  }

  expect-string("language", data.language)
  if not (data.language in ("pt-BR", "en")) {
    fail("unknown-language", " \"" + data.language + "\"; allowed: pt-BR, en")
  }

  // `date_format` was removed: the report layer never passes a `date:`
  // value to `graphite-report`, so no page ever reads it. See the final
  // review fix report for this round.
  check-exact-keys("theme", data.theme, ("lang", "region"))
  expect-string("theme.lang", data.theme.lang)
  expect-string("theme.region", data.theme.region)
  if data.theme.lang.match(regex("^[a-z]{2,3}$")) == none {
    fail(
      "wrong-type",
      "theme.lang` must be a language code of two or three lower-case letters; received "
        + repr(data.theme.lang),
    )
  }
  if data.theme.region.match(regex("^[A-Z]{2}$")) == none {
    fail(
      "wrong-type",
      "theme.region` must be a region code of two upper-case letters; received "
        + repr(data.theme.region),
    )
  }

  check-exact-keys("labels", data.labels, label-keys)
  for key in label-keys {
    expect-string("labels." + key, data.labels.at(key))
  }

  check-exact-keys("cover", data.cover, cover-keys)
  for key in cover-keys {
    if key == "author" {
      expect-string-or-none("cover.author", data.cover.author)
    } else {
      expect-string("cover." + key, data.cover.at(key))
    }
  }

  check-exact-keys("footer", data.footer, footer-keys)
  expect-string-or-none("footer.classification", data.footer.classification)
  expect-string("footer.page_label", data.footer.page_label)

  expect-array("blocks", data.blocks)
  if data.blocks.len() == 0 {
    fail("no-blocks", "")
  }
  for (index, block) in data.blocks.enumerate() {
    validate-block(index, block, false)
  }

  // Every block is individually well-formed by this point, so this second
  // pass can read `.type` and `front`'s `.blocks` without checking either
  // again. It counts every `outline` in document order — front's, since
  // `front` can only be the first block, always come first — and fails on
  // the second one, wherever it is.
  let outline-count = 0
  for (index, block) in data.blocks.enumerate() {
    if block.type == "outline" {
      outline-count += 1
      if outline-count > 1 {
        fail("duplicate-outline", " " + "blocks[" + str(index) + "]")
      }
    } else if block.type == "front" {
      for (j, inner) in block.blocks.enumerate() {
        if inner.type == "outline" {
          outline-count += 1
          if outline-count > 1 {
            fail("duplicate-outline", " " + "blocks[0].blocks[" + str(j) + "]")
          }
        }
      }
    }
  }

  data
}
