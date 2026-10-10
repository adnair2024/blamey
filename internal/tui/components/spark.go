package components

import "strings"

var sparkRunes = []rune("▁▂▃▄▅▆▇█")

// Sparkline draws data in exactly width cells. When there are more points than
// cells, neighbouring points are summed into buckets; when fewer, the line is
// shorter and padded with spaces on the right. Zero is the lowest bar, and an
// all-zero series is flat.
func Sparkline(data []int, width int) string {
	if width <= 0 {
		return ""
	}
	buckets := Bucket(data, width)
	peak := 0
	for _, v := range buckets {
		peak = max(peak, v)
	}
	var b strings.Builder
	for _, v := range buckets {
		idx := 0
		if peak > 0 && v > 0 {
			// Non-zero values always get at least the second glyph so they
			// are distinguishable from empty weeks.
			idx = 1 + (v*(len(sparkRunes)-2))/peak
		}
		b.WriteRune(sparkRunes[idx])
	}
	b.WriteString(strings.Repeat(" ", width-len(buckets)))
	return b.String()
}

// Bucket sums data into at most n contiguous buckets of near-equal size.
func Bucket(data []int, n int) []int {
	if n <= 0 || len(data) == 0 {
		return nil
	}
	if len(data) <= n {
		return append([]int(nil), data...)
	}
	out := make([]int, n)
	for i, v := range data {
		out[i*n/len(data)] += v
	}
	return out
}
