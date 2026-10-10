package output

import (
	"fmt"
	"strconv"
	"time"
)

// Num formats an integer with thousands separators.
func Num(n int) string {
	s := strconv.Itoa(n)
	neg := n < 0
	if neg {
		s = s[1:]
	}
	for i := len(s) - 3; i > 0; i -= 3 {
		s = s[:i] + "," + s[i:]
	}
	if neg {
		s = "-" + s
	}
	return s
}

// Pct formats a fraction as a percentage with one decimal. Rounding happens
// only here; stored values stay exact.
func Pct(frac float64) string { return fmt.Sprintf("%.1f%%", frac*100) }

// Ago renders a relative time ("3 months ago") plus the absolute date.
func Ago(t, now time.Time) string {
	if t.IsZero() {
		return "never"
	}
	return fmt.Sprintf("%s (%s)", Relative(t, now), t.Format("2006-01-02"))
}

// Relative renders the distance between t and now in the largest sensible unit.
func Relative(t, now time.Time) string {
	d := now.Sub(t)
	switch {
	case d < 0:
		return "in the future"
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return plural(int(d/time.Minute), "minute")
	case d < 24*time.Hour:
		return plural(int(d/time.Hour), "hour")
	case d < 30*24*time.Hour:
		return plural(int(d/(24*time.Hour)), "day")
	case d < 365*24*time.Hour:
		return plural(int(d/(30*24*time.Hour)), "month")
	}
	return plural(int(d/(365*24*time.Hour)), "year")
}

func plural(n int, unit string) string {
	if n == 1 {
		return "1 " + unit + " ago"
	}
	return fmt.Sprintf("%d %ss ago", n, unit)
}
