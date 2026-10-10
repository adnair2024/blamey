// Package styles holds every color and lipgloss style used by the TUI. Views
// never build styles inline.
//
// lipgloss v2 has no adaptive colors, so a Styles value is built for either a
// dark or a light background; the root model rebuilds it when the terminal
// reports its background. Downsampling to the terminal's color depth and
// NO_COLOR handling happen when Bubble Tea writes the frame.
package styles

import (
	"hash/fnv"
	"image/color"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/table"
	"charm.land/lipgloss/v2"
)

// Styles is the complete visual theme.
type Styles struct {
	Dark bool

	Title, Subtle, Accent, Good, Warn, Bad, Bold lipgloss.Style
	Tab, TabActive, TabBar                       lipgloss.Style
	Card, CardTitle, CardValue                   lipgloss.Style
	Section                                      lipgloss.Style
	Selected                                     lipgloss.Style
	Footer                                       lipgloss.Style
	Silo, SingleOwner                            lipgloss.Style
	Bot                                          lipgloss.Style
	// Heat is the intensity ramp for heatmaps, from empty to hottest.
	Heat []lipgloss.Style
	// HeatGlyphs pairs with Heat so intensity survives without color.
	HeatGlyphs []string
	// Bar fills a single-color horizontal bar; Track is its empty part.
	Bar, Track lipgloss.Style

	Table table.Styles
	Help  help.Styles

	palette []color.Color
}

// New builds the theme for a dark or light terminal background.
func New(dark bool) *Styles {
	ld := lipgloss.LightDark(dark)
	c := func(light, dark string) color.Color { return ld(lipgloss.Color(light), lipgloss.Color(dark)) }

	text := c("#1f2328", "#e6edf3")
	subtle := c("#6e7781", "#7d8590")
	accent := c("#0969da", "#58a6ff")
	good := c("#1a7f37", "#3fb950")
	warn := c("#9a6700", "#d29922")
	bad := c("#cf222e", "#f85149")
	border := c("#d0d7de", "#30363d")
	selBG := c("#ddf4ff", "#1c2d41")

	s := &Styles{Dark: dark}
	s.Title = lipgloss.NewStyle().Bold(true).Foreground(accent)
	s.Subtle = lipgloss.NewStyle().Foreground(subtle)
	s.Accent = lipgloss.NewStyle().Foreground(accent)
	s.Good = lipgloss.NewStyle().Foreground(good)
	s.Warn = lipgloss.NewStyle().Foreground(warn)
	s.Bad = lipgloss.NewStyle().Foreground(bad)
	s.Bold = lipgloss.NewStyle().Bold(true).Foreground(text)

	s.Tab = lipgloss.NewStyle().Foreground(subtle).Padding(0, 1)
	s.TabActive = lipgloss.NewStyle().Bold(true).Foreground(accent).Underline(true).Padding(0, 1)
	s.TabBar = lipgloss.NewStyle()

	s.Card = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(border).Padding(0, 1)
	s.CardTitle = lipgloss.NewStyle().Foreground(subtle)
	s.CardValue = lipgloss.NewStyle().Bold(true).Foreground(text)
	s.Section = lipgloss.NewStyle().Bold(true).Foreground(text)
	s.Selected = lipgloss.NewStyle().Bold(true).Background(selBG)
	s.Footer = lipgloss.NewStyle().Foreground(subtle)
	s.Silo = lipgloss.NewStyle().Foreground(warn)
	s.SingleOwner = lipgloss.NewStyle().Foreground(accent)
	s.Bot = lipgloss.NewStyle().Foreground(subtle).Italic(true)

	s.Bar = lipgloss.NewStyle().Foreground(accent)
	s.Track = lipgloss.NewStyle().Foreground(border)
	s.Heat = []lipgloss.Style{
		lipgloss.NewStyle().Foreground(c("#d0d7de", "#21262d")),
		lipgloss.NewStyle().Foreground(c("#9be9a8", "#0e4429")),
		lipgloss.NewStyle().Foreground(c("#40c463", "#006d32")),
		lipgloss.NewStyle().Foreground(c("#30a14e", "#26a641")),
		lipgloss.NewStyle().Foreground(c("#216e39", "#39d353")),
	}

	s.HeatGlyphs = []string{"·", "░", "▒", "▓", "█"}

	s.Table = table.Styles{
		Header:   lipgloss.NewStyle().Bold(true).Foreground(subtle).Padding(0, 1),
		Cell:     lipgloss.NewStyle().Padding(0, 1),
		Selected: s.Selected,
	}
	s.Help = help.DefaultStyles(dark)

	s.palette = []color.Color{
		c("#0969da", "#58a6ff"), c("#bf3989", "#f778ba"), c("#1a7f37", "#56d364"),
		c("#bc4c00", "#ffa657"), c("#8250df", "#d2a8ff"), c("#0a7e8c", "#39c5cf"),
		c("#9a6700", "#e3b341"), c("#cf222e", "#ff7b72"), c("#116329", "#7ee787"),
		c("#6639ba", "#a371f7"),
	}
	return s
}

// ContributorColor gives a person the same color on every screen by hashing
// their identity ID.
func (s *Styles) ContributorColor(id string) color.Color {
	h := fnv.New32a()
	_, _ = h.Write([]byte(id))
	return s.palette[int(h.Sum32())%len(s.palette)]
}

// Contributor is the foreground style for a person's color.
func (s *Styles) Contributor(id string) lipgloss.Style {
	return lipgloss.NewStyle().Foreground(s.ContributorColor(id))
}

// Other colors the combined "everyone else" series.
func (s *Styles) Other() lipgloss.Style { return s.Track }

// Block is a fixed-size box: content is padded or cut to exactly w×h cells.
func (s *Styles) Block(w, h int) lipgloss.Style {
	return lipgloss.NewStyle().Width(w).Height(h).MaxWidth(w).MaxHeight(h)
}

// Centered places content in the middle of a w×h area.
func (s *Styles) Centered(w, h int, content string) string {
	return lipgloss.Place(w, h, lipgloss.Center, lipgloss.Center, content)
}
