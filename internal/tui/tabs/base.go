package tabs

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/adnair2024/blamey/internal/output"
	"github.com/adnair2024/blamey/internal/stats"
	"github.com/adnair2024/blamey/internal/tui/keys"
	"github.com/adnair2024/blamey/internal/tui/styles"
)

// base carries the state every tab shares. Tabs embed it and override only
// what differs.
type base struct {
	st   *styles.Styles
	km   keys.Map
	data Data
	w, h int
}

func (b *base) Init() tea.Cmd               { return nil }
func (b *base) Capturing() bool             { return false }
func (b *base) Resize(w, h int)             { b.w, b.h = w, h }
func (b *base) SetData(d Data)              { b.data = d }
func (b *base) SetStyles(s *styles.Styles)  { b.st = s }
func (b *base) report() *stats.Report       { return b.data.Report }
func (b *base) section(title string) string { return b.st.Section.Render(title) }
func (b *base) subtle(s string) string      { return b.st.Subtle.Render(s) }

// empty is shown when there is nothing to draw.
func (b *base) empty(msg string) string { return b.subtle(msg) }

func num(n int) string     { return output.Num(n) }
func pct(f float64) string { return output.Pct(f) }

// nameCell renders a contributor name in their stable color, dimming bots.
func (b *base) nameCell(c stats.Contributor) string {
	if c.Bot {
		return b.st.Bot.Render(c.Name + " (bot)")
	}
	return b.st.Contributor(c.ID).Render(c.Name)
}

// kv renders a label/value pair with a fixed label width.
func (b *base) kv(label, value string, labelW int) string {
	return b.st.Subtle.Render(fit(label, labelW)) + value
}

// joinCols places already-rendered blocks side by side separated by gap cells.
func joinCols(gap int, blocks ...string) string {
	sep := strings.Repeat(" ", gap)
	split := make([][]string, len(blocks))
	rows := 0
	for i, b := range blocks {
		split[i] = strings.Split(b, "\n")
		rows = max(rows, len(split[i]))
	}
	widths := make([]int, len(blocks))
	for i, lines := range split {
		for _, l := range lines {
			widths[i] = max(widths[i], stringWidth(l))
		}
	}
	out := make([]string, rows)
	for r := range out {
		var parts []string
		for i, lines := range split {
			cell := ""
			if r < len(lines) {
				cell = lines[r]
			}
			parts = append(parts, fit(cell, widths[i]))
		}
		out[r] = strings.TrimRight(strings.Join(parts, sep), " ")
	}
	return strings.Join(out, "\n")
}

func plural(n int, unit string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, unit)
	}
	return fmt.Sprintf("%d %ss", n, unit)
}
