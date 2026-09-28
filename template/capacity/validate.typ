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
)
// diagnostics:end

#let fail(name, detail) = panic(
  "capacity-report: " + diagnostics.at(name) + detail,
)

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

// check-exact-keys backs every `unknown-key` and `missing-key` diagnostic.
// `place` is one of the six values the contract names: `report`, `theme`,
// `labels`, `cover`, `footer`, or `blocks[<i>]`.
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
#let label-keys = ("author", "date", "classification", "outline", "note", "info", "warning")
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

#let allowed-run-kinds = ("text", "ident", "placeholder", "status")
#let allowed-statuses = ("healthy", "attention", "critical", "undefined", "unavailable")

// validate-run checks one run object, wherever it appears (a heading's,
// paragraph's, or callout's `runs`, a table cell, or a callout `title`).
// `path` identifies the run for `unknown-run-kind`, `unknown-status`, and
// `wrong-type`; the caller is responsible for the `empty-runs` check on
// the array that holds it, since an empty table cell is legal.
#let validate-run(path, run) = {
  expect-dict(path, run)
  let kind = run.at("kind", default: none)
  if not (type(kind) == str and kind in allowed-run-kinds) {
    fail("unknown-run-kind", " \"" + str(kind) + "\" at " + path)
  }
  if kind == "status" {
    let status = run.at("status", default: none)
    if not (type(status) == str and status in allowed-statuses) {
      fail("unknown-status", " \"" + str(status) + "\" at " + path)
    }
    expect-string-or-none(path + ".text", run.at("text", default: none))
  } else {
    expect-string(path + ".text", run.at("text", default: none))
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

#let allowed-block-types = ("outline", "heading", "paragraph", "table", "callout", "pagebreak")
#let allowed-callout-kinds = ("note", "info", "warning")

#let validate-outline-block(place, block) = {
  check-exact-keys(place, block, ("type", "title", "depth"))
  expect-string(place + ".title", block.title)
  let depth = block.depth
  if not (type(depth) == int and depth in (1, 2, 3)) {
    fail("wrong-type", place + ".depth` must be 1, 2, or 3; received " + type-name(depth))
  }
}

#let validate-heading-block(place, block, index) = {
  check-exact-keys(place, block, ("type", "level", "runs"))
  let level = block.level
  if not (type(level) == int and level in (1, 2, 3)) {
    fail("heading-level", " " + str(level) + " at " + place)
  }
  validate-runs(place + ".runs", block.runs)
}

#let validate-paragraph-block(place, block) = {
  check-exact-keys(place, block, ("type", "runs"))
  validate-runs(place + ".runs", block.runs)
}

#let validate-table-block(place, block, index) = {
  check-exact-keys(place, block, ("type", "id", "headers", "rows"))
  let id = block.id
  expect-string(place + ".id", id)
  if not (id in layouts.keys()) {
    fail("unknown-table", " \"" + id + "\" at " + place)
  }
  let columns = layouts.at(id).columns.len()

  expect-array(place + ".headers", block.headers)
  for (i, header) in block.headers.enumerate() {
    expect-string(place + ".headers[" + str(i) + "]", header)
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
      let cell-path = row-path + ".cells[" + str(c) + "]"
      expect-array(cell-path, cell)
      for (j, run) in cell.enumerate() {
        validate-run(cell-path + ".runs[" + str(j) + "]", run)
      }
    }
  }
}

#let validate-callout-block(place, block) = {
  check-exact-keys(place, block, ("type", "kind", "title", "runs"))
  let kind = block.kind
  if not (type(kind) == str and kind in allowed-callout-kinds) {
    fail(
      "wrong-type",
      place + ".kind` must be \"note\", \"info\", or \"warning\"; received " + type-name(kind),
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

#let validate-block(index, block) = {
  let place = "blocks[" + str(index) + "]"
  expect-dict(place, block)
  let block-type = block.at("type", default: none)
  if not (type(block-type) == str and block-type in allowed-block-types) {
    fail("unknown-block-type", " \"" + str(block-type) + "\" at " + place)
  }
  if block-type == "outline" {
    validate-outline-block(place, block)
  } else if block-type == "heading" {
    validate-heading-block(place, block, index)
  } else if block-type == "paragraph" {
    validate-paragraph-block(place, block)
  } else if block-type == "table" {
    validate-table-block(place, block, index)
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

  check-exact-keys("theme", data.theme, ("lang", "region", "date_format"))
  for key in ("lang", "region", "date_format") {
    expect-string("theme." + key, data.theme.at(key))
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
    validate-block(index, block)
  }

  data
}
