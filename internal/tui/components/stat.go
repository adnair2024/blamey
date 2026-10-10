package components

import "charm.land/lipgloss/v2"

// StatCard renders a bordered card with a label and a big value. width is the
// total outer width including the border.
func StatCard(label, value string, width int, card, labelStyle, valueStyle lipgloss.Style) string {
	inner := max(width-card.GetHorizontalFrameSize(), 1)
	body := labelStyle.Render(label) + "\n" + valueStyle.Render(value)
	return card.Width(inner + card.GetHorizontalFrameSize()).Render(body)
}
