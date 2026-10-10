package components

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

func TestHBar(t *testing.T) {
	tests := []struct {
		frac  float64
		width int
		want  string
	}{
		{0, 4, "    "},
		{1, 4, "████"},
		{0.5, 4, "██  "},
		{0.375, 4, "█▌  "}, // 3/8 of 4 cells = 12 eighths = 1.5 cells
		{0.125 / 2, 4, "▎   "},
		{-1, 3, "   "},
		{9, 3, "███"},
	}
	for _, tt := range tests {
		got := HBar(tt.frac, tt.width)
		if got != tt.want {
			t.Errorf("HBar(%v, %d) = %q, want %q", tt.frac, tt.width, got, tt.want)
		}
		if w := ansi.StringWidth(got); w != tt.width {
			t.Errorf("HBar(%v, %d) width %d", tt.frac, tt.width, w)
		}
	}
	if HBar(0.5, 0) != "" {
		t.Error("zero width")
	}
}

func TestStackedBarWidthNeverDrifts(t *testing.T) {
	var zero lipgloss.Style
	for width := 1; width <= 40; width++ {
		segs := []Segment{{0.333, zero}, {0.333, zero}, {0.334, zero}}
		if w := ansi.StringWidth(StackedBar(segs, width)); w != width {
			t.Fatalf("width %d: got %d", width, w)
		}
	}
	got := StackedBar([]Segment{{0.5, zero}, {0.25, zero}}, 8)
	if got != "██████  " {
		t.Errorf("partial stack = %q", got)
	}
	if got := StackedBar([]Segment{{1, zero}}, 5); got != "█████" {
		t.Errorf("full = %q", got)
	}
}

func TestSparkline(t *testing.T) {
	if got := Sparkline([]int{0, 1, 4, 8}, 4); got != "▁▂▅█" {
		t.Errorf("sparkline = %q", got)
	}
	if got := Sparkline([]int{0, 0, 0}, 3); got != "▁▁▁" {
		t.Errorf("flat = %q", got)
	}
	// Tiny non-zero values stay distinguishable from empty weeks.
	if got := Sparkline([]int{0, 1, 1000}, 3); got[len("▁"):len("▁")+3] != "▂" {
		t.Errorf("small value lost: %q", got)
	}
	got := Sparkline([]int{1, 1, 1, 1, 1, 1}, 3)
	if ansi.StringWidth(got) != 3 || strings.Count(got, "█") != 3 {
		t.Errorf("bucketed = %q", got)
	}
	if got := Sparkline([]int{3, 3}, 5); ansi.StringWidth(got) != 5 {
		t.Errorf("short data should pad: %q", got)
	}
	if Sparkline(nil, 5) != "     " {
		t.Errorf("empty = %q", Sparkline(nil, 5))
	}
}

func TestBucket(t *testing.T) {
	if got := Bucket([]int{1, 2, 3, 4, 5, 6}, 3); len(got) != 3 || got[0] != 3 || got[1] != 7 || got[2] != 11 {
		t.Errorf("bucket = %v", got)
	}
	total := 0
	for _, v := range Bucket([]int{1, 2, 3, 4, 5, 6, 7}, 3) {
		total += v
	}
	if total != 28 {
		t.Errorf("bucketing must preserve the sum, got %d", total)
	}
}

func TestHeatmap(t *testing.T) {
	var zero lipgloss.Style
	levels := []lipgloss.Style{zero, zero, zero, zero, zero}
	glyphs := []string{"·", "░", "▒", "▓", "█"}
	got := Heatmap([][]int{{0, 1, 5, 10}, {10, 0, 0, 2}}, levels, glyphs, "")
	// peak 10: 1 -> ceil(0.4)=1, 5 -> ceil(2)=2, 10 -> 4, 2 -> ceil(0.8)=1
	if want := "·░▒█\n█··░"; got != want {
		t.Errorf("heatmap = %q, want %q", got, want)
	}
	if Heatmap([][]int{{1}}, nil, nil, "") != "" {
		t.Error("no levels")
	}
}

func TestStackedColumns(t *testing.T) {
	var zero lipgloss.Style
	got := StackedColumns([][]int{{4, 0}, {2, 2}, {0, 0}, {1, 0}}, []lipgloss.Style{zero, zero}, 4)
	want := "██  \n██  \n██  \n██ █"
	if got != want {
		t.Errorf("columns:\n%s\nwant:\n%s", got, want)
	}
	if StackedColumns(nil, nil, 4) != "" || StackedColumns([][]int{{1}}, []lipgloss.Style{zero}, 0) != "" {
		t.Error("empty input")
	}
}
