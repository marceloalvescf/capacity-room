package catalog

import (
	"strconv"
	"time"
)

// DateTime writes the wall clock of t: 15/07/2026 14:30 in Portuguese,
// 07/15/2026 14:30 in English. It uses t's own wall clock, in whatever
// location t carries, never converting it to another zone.
func (c *Catalog) DateTime(t time.Time) string {
	if c.lang == PortugueseBrazil {
		return t.Format("02/01/2006 15:04")
	}
	return t.Format("01/02/2006 15:04")
}

// Integer writes a whole number with no separator.
func (c *Catalog) Integer(n int) string {
	return strconv.Itoa(n)
}
