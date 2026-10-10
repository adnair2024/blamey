// Package components has the hand-rolled charts. Everything here is a pure
// function of its inputs so it can be tested without a terminal.
package components

import (
	"math"
	"strings"

	"charm.land/lipgloss/v2"
)

// eighths are left-aligned partial blocks, 1/8 to 7/8 of a cell.
var eighths = []rune{'▏', '▎', '▍', '▌', '▋', '▊', '▉'}

// HBar renders frac (0..1) as a horizontal bar of exactly width cells, using
// partial blocks for sub-cell precision. The unfilled part is spaces.
func HBar(frac float64, width int) string {
	if width <= 0 {
		return ""
	}
	frac = math.Min(math.Max(frac, 0), 1)
	eighthsTotal := int(math.Round(frac * float64(width) * 8))
	full, rem := eighthsTotal/8, eighthsTotal%8
	var b strings.Builder
	b.WriteString(strings.Repeat("█", full))
	used := full
	if rem > 0 && used < width {
		b.WriteRune(eighths[rem-1])
		used++
	}
	b.WriteString(strings.Repeat(" ", width-used))
	return b.String()
}

// Segment is one slice of a stacked bar.
type Segment struct {
	Frac  float64 // share of the whole, 0..1
	Style lipgloss.Style
}

// StackedBar renders segments left to right in exactly width cells. Cells are
// allocated with the largest-remainder method so the total never drifts, and
// any leftover (the segments sum to less than 1) is blank.
func StackedBar(segs []Segment, width int) string {
	if width <= 0 {
		return ""
	}
	cells := allocate(fracs(segs), width)
	var b strings.Builder
	used := 0
	for i, n := range cells {
		if n > 0 {
			b.WriteString(segs[i].Style.Render(strings.Repeat("█", n)))
			used += n
		}
	}
	b.WriteString(strings.Repeat(" ", width-used))
	return b.String()
}

func fracs(segs []Segment) []float64 {
	out := make([]float64, len(segs))
	for i, s := range segs {
		out[i] = math.Max(s.Frac, 0)
	}
	return out
}

// allocate splits at most total cells among shares (each a fraction of the
// whole) using largest remainders. Shares summing to <1 leave cells unassigned.
func allocate(shares []float64, total int) []int {
	out := make([]int, len(shares))
	sum := 0.0
	for _, s := range shares {
		sum += s
	}
	target := int(math.Round(math.Min(sum, 1) * float64(total)))
	type rem struct {
		i int
		r float64
	}
	var rems []rem
	assigned := 0
	for i, s := range shares {
		exact := s * float64(total)
		out[i] = int(exact)
		assigned += out[i]
		rems = append(rems, rem{i, exact - float64(out[i])})
	}
	for assigned < target {
		best := -1
		for j, r := range rems {
			if r.r > 0 && (best < 0 || r.r > rems[best].r) {
				best = j
			}
		}
		if best < 0 {
			break
		}
		out[rems[best].i]++
		rems[best].r = 0
		assigned++
	}
	return out
}
