package components

import (
	"strings"

	"charm.land/lipgloss/v2"
)

// StackedColumns draws a stacked column chart. cols[x][s] is series s at
// column x; styles has one style per series. The tallest column fills height
// rows and the rest scale to it. Rows are returned top to bottom.
func StackedColumns(cols [][]int, styles []lipgloss.Style, height int) string {
	if height <= 0 || len(cols) == 0 {
		return ""
	}
	peak := 0
	for _, c := range cols {
		peak = max(peak, sum(c))
	}
	// cells[x][s] = rows occupied by series s in column x.
	cells := make([][]int, len(cols))
	for x, c := range cols {
		shares := make([]float64, len(c))
		total := float64(sum(c))
		if peak > 0 {
			for s, v := range c {
				shares[s] = float64(v) / float64(peak)
			}
		}
		cells[x] = allocate(shares, height)
		// A column with any activity shows at least one row.
		if total > 0 && sum(cells[x]) == 0 {
			cells[x][argmax(c)] = 1
		}
	}
	lines := make([]string, height)
	for row := 0; row < height; row++ {
		fromBottom := height - 1 - row
		var b strings.Builder
		for x := range cols {
			b.WriteString(cellAt(cells[x], styles, fromBottom))
		}
		lines[row] = b.String()
	}
	return strings.Join(lines, "\n")
}

// cellAt returns the styled glyph at a height above the baseline; series are
// stacked from the bottom in index order.
func cellAt(stack []int, styles []lipgloss.Style, h int) string {
	acc := 0
	for s, n := range stack {
		acc += n
		if h < acc {
			return styles[s].Render("█")
		}
	}
	return " "
}

func sum(xs []int) int {
	t := 0
	for _, x := range xs {
		t += x
	}
	return t
}

func argmax(xs []int) int {
	best := 0
	for i, x := range xs {
		if x > xs[best] {
			best = i
		}
	}
	return best
}
