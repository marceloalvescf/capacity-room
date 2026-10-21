// palette.typ holds the only colours of the report: the light status colours
// of the web app, which `template/palette_test.go` holds equal to
// `web/src/theme/theme.css`. A word and a number stay in ink; a colour
// draws a status shape, a chart, and the tint of a peak cell.

#let critical = rgb("#be1e45")
#let attention = rgb("#8f5b00")
#let healthy = rgb("#24774a")
#let neutral = rgb("#5c5c5e")
#let critical-surface = rgb("#fdedf0")
#let attention-surface = rgb("#fdf4e3")

// status-colour is the colour of a status shape; a status that is no
// severity (undefined, unavailable) takes the neutral.
#let status-colour(status) = if status == "critical" {
  critical
} else if status == "attention" {
  attention
} else if status == "healthy" {
  healthy
} else {
  neutral
}
