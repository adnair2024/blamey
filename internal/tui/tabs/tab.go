// Package tabs holds one Bubble Tea sub-model per screen. Tabs only render a
// stats.Report; they never compute statistics or touch git.
package tabs

import (
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/adnair2024/blamey/internal/stats"
	"github.com/adnair2024/blamey/internal/tui/styles"
)

// Data is what the root model hands to every tab.
type Data struct {
	Report   *stats.Report
	ShowBots bool
	Now      time.Time
}

// Tab is one screen. The root routes messages to the active tab and owns the
// window size; a tab reports the area it may draw in through Resize.
type Tab interface {
	Name() string
	Init() tea.Cmd
	Update(msg tea.Msg) (Tab, tea.Cmd)
	View() string
	Resize(width, height int)
	SetData(d Data)
	SetStyles(s *styles.Styles)
	// Capturing is true while the tab needs raw key input (a filter box is
	// focused), so the root must not treat letters as global shortcuts.
	Capturing() bool
}

// people returns the contributors to show, hiding bots unless asked.
func (d Data) people() []stats.Contributor {
	if d.Report == nil {
		return nil
	}
	out := make([]stats.Contributor, 0, len(d.Report.Contributors))
	for _, c := range d.Report.Contributors {
		if c.Bot && !d.ShowBots {
			continue
		}
		out = append(out, c)
	}
	return out
}

// fit pads or truncates s to exactly w display cells (ANSI and wide-rune safe).
func fit(s string, w int) string {
	if w <= 0 {
		return ""
	}
	s = ansi.Truncate(s, w, "…")
	if pad := w - ansi.StringWidth(s); pad > 0 {
		s += strings.Repeat(" ", pad)
	}
	return s
}

// fitRight right-aligns s within w cells, truncating if needed.
func fitRight(s string, w int) string {
	if w <= 0 {
		return ""
	}
	s = ansi.Truncate(s, w, "…")
	if pad := w - ansi.StringWidth(s); pad > 0 {
		s = strings.Repeat(" ", pad) + s
	}
	return s
}

// clip limits a block to height lines, dropping the excess.
func clip(block string, height int) string {
	if height <= 0 {
		return ""
	}
	lines := strings.Split(block, "\n")
	if len(lines) > height {
		lines = lines[:height]
	}
	return strings.Join(lines, "\n")
}
