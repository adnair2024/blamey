package tui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

var tabKeys = [...]string{"1", "2", "3", "4", "5"}

func (m *Model) header() string {
	title := m.st.Title.Render("blamey")
	meta := m.report.Meta
	loc := meta.Path
	if meta.Head != "" {
		loc += " @ " + meta.Rev + " (" + meta.Head[:min(8, len(meta.Head))] + ")"
	}
	left := title + "  " + m.st.Subtle.Render(loc)
	for _, w := range meta.Warnings {
		left += "  " + m.st.Warn.Render("⚠ "+w)
	}
	return fit(left, m.w) + "\n" + m.tabBar()
}

func (m *Model) tabBar() string {
	var parts []string
	for i, t := range m.tabs {
		label := fmt.Sprintf("%s %s", tabKeys[i], t.Name())
		if i == m.active && !m.showHelp {
			parts = append(parts, m.st.TabActive.Render(label))
		} else {
			parts = append(parts, m.st.Tab.Render(label))
		}
	}
	bots := "bots: hidden"
	if m.showBots {
		bots = "bots: shown"
	}
	left := strings.Join(parts, "")
	right := m.st.Subtle.Render(bots)
	gap := max(m.w-lipgloss.Width(left)-lipgloss.Width(right), 1)
	return fit(left+strings.Repeat(" ", gap)+right, m.w)
}

func (m *Model) footer() string {
	m.help.SetWidth(m.w)
	return fit(m.help.ShortHelpView(m.km.ShortHelp()), m.w)
}

func (m *Model) loadingView() string {
	var b strings.Builder
	b.WriteString(m.st.Title.Render("blamey") + "\n\n")
	phase := m.prog.Phase
	if m.prog.Total > 0 {
		phase = fmt.Sprintf("%s %s/%s files", phase, groupDigits(m.prog.Done), groupDigits(m.prog.Total))
	}
	b.WriteString(m.spin.View() + " " + phase + "\n\n")
	if m.prog.Total > 0 {
		b.WriteString(m.bar.ViewAs(float64(m.prog.Done)/float64(m.prog.Total)) + "\n\n")
	}
	b.WriteString(m.st.Footer.Render("q to quit"))
	return m.st.Centered(m.w, m.h, b.String())
}

func (m *Model) errorView() string {
	msg := m.st.Bad.Render("blamey could not analyze this repository") + "\n\n" +
		ansi.Wrap(m.err.Error(), max(m.w-8, 20), "") + "\n\n" + m.st.Footer.Render("q to quit")
	return m.st.Centered(m.w, m.h, msg)
}

func (m *Model) tooSmall() string {
	msg := fmt.Sprintf("Terminal too small\n%d×%d, need at least %d×%d", m.w, m.h, minWidth, minHeight)
	return m.st.Centered(max(m.w, 1), max(m.h, 1), msg)
}

// fit truncates a possibly styled line to w cells.
func fit(s string, w int) string { return ansi.Truncate(s, w, "…") }

// groupDigits formats 1234567 as 1,234,567.
func groupDigits(n int) string {
	s := fmt.Sprintf("%d", n)
	for i := len(s) - 3; i > 0; i -= 3 {
		s = s[:i] + "," + s[i:]
	}
	return s
}
