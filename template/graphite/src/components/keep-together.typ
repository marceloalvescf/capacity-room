#let keep-together(body) = {
  if type(body) != content {
    panic("graphite-report: `keep-together.body` must be content")
  }
  block(breakable: false, body)
}
