package components

import (
	"strings"

	"charm.land/lipgloss/v2"
)

// Heatmap renders a grid of counts. Intensity is relative to the largest
// value: 0 is level 0 and any positive value is at least level 1, up to
// len(levels)-1. Each level has its own glyph as well as a style, so the
// intensity stays readable without color (NO_COLOR). pad is appended to every
// cell. Each row is one line.
func Heatmap(rows [][]int, levels []lipgloss.Style, glyphs []string, pad string) string {
	if len(levels) == 0 || len(glyphs) < len(levels) {
		return ""
	}
	peak := 0
	for _, r := range rows {
		for _, v := range r {
			peak = max(peak, v)
		}
	}
	lines := make([]string, len(rows))
	for i, r := range rows {
		var b strings.Builder
		for _, v := range r {
			lv := level(v, peak, len(levels))
			b.WriteString(levels[lv].Render(glyphs[lv] + pad))
		}
		lines[i] = b.String()
	}
	return strings.Join(lines, "\n")
}

func level(v, peak, n int) int {
	if v <= 0 || peak <= 0 || n < 2 {
		return 0
	}
	// ceil(v/peak * (n-1)), kept in integers.
	return (v*(n-1) + peak - 1) / peak
}
