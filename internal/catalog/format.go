package catalog

import (
	"math"
	"strconv"
	"strings"
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

// Decimal writes value with digits decimals, trims the trailing zeros and
// then a decimal mark left at the end, and uses the comma in Portuguese and
// the point in English. It adds no thousands separator.
func (c *Catalog) Decimal(value float64, digits int) string {
	rendered := strconv.FormatFloat(value, 'f', digits, 64)
	rendered = strings.TrimRight(rendered, "0")
	rendered = strings.TrimRight(rendered, ".")
	if c.lang == PortugueseBrazil {
		rendered = strings.Replace(rendered, ".", ",", 1)
	}
	return rendered
}

// Percent writes value as a decimal of one digit, followed by "%".
func (c *Catalog) Percent(value float64) string {
	return c.Decimal(value, 1) + "%"
}

// CPU writes cores in millicores when its absolute value is below 1, and in
// cores otherwise.
func (c *Catalog) CPU(cores float64) string {
	if math.Abs(cores) < 1 {
		return c.Decimal(cores*1000, 1) + " mCPU"
	}
	return c.Decimal(cores, 2) + " cores"
}

// Memory writes bytes in the largest of B, KiB, MiB, GiB, and TiB whose
// absolute value stays at or above 1, with two digits.
func (c *Catalog) Memory(bytes float64) string {
	units := []string{"B", "KiB", "MiB", "GiB", "TiB"}
	scaled := bytes
	unit := units[0]
	for _, candidate := range units[1:] {
		if math.Abs(scaled) < 1024 {
			break
		}
		scaled /= 1024
		unit = candidate
	}
	return c.Decimal(scaled, 2) + " " + unit
}

// Count rounds value to the nearest whole number, a half to the even one,
// and writes it with no separator.
func (c *Catalog) Count(value float64) string {
	return strconv.Itoa(int(math.RoundToEven(value)))
}
