package tabs

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/adnair2024/blamey/internal/stats"
	"github.com/adnair2024/blamey/internal/tui/components"
	"github.com/adnair2024/blamey/internal/tui/keys"
	"github.com/adnair2024/blamey/internal/tui/styles"
)

func stringWidth(s string) int { return ansi.StringWidth(s) }

// Overview shows headline numbers, the top owners and repo-wide activity.
type Overview struct{ base }

func NewOverview(st *styles.Styles, km keys.Map) *Overview {
	return &Overview{base{st: st, km: km}}
}

func (o *Overview) Name() string                  { return "Overview" }
func (o *Overview) Update(tea.Msg) (Tab, tea.Cmd) { return o, nil }

func (o *Overview) View() string {
	r := o.report()
	if r == nil {
		return ""
	}
	if r.Totals.Commits == 0 && r.Totals.SurvivingLines == 0 {
		return o.empty("No commits to analyze.")
	}
	var parts []string
	parts = append(parts, o.cards())

	people := o.data.people()
	// Leave room for cards, the activity block and spacing.
	rows := min(10, max(o.h-cardsHeight(o.w)-7, 3))
	parts = append(parts, o.owners(people, rows))
	parts = append(parts, o.activity())
	if r.Meta.Since != "" || r.Meta.Until != "" {
		parts = append(parts, o.subtle("Ownership is measured at "+r.Meta.Rev+" and ignores --since/--until."))
	}
	return clip(strings.Join(parts, "\n\n"), o.h)
}

func cardsHeight(w int) int {
	if w >= 72 {
		return 4
	}
	return 8
}

func (o *Overview) cards() string {
	r := o.report()
	type card struct{ label, value string }
	cs := []card{
		{"Surviving lines", num(r.Totals.SurvivingLines)},
		{"Contributors", fmt.Sprintf("%d  (%d bots)", r.Totals.Contributors, r.Totals.Bots)},
		{"Commits", num(r.Totals.Commits)},
		{"Bus factor", fmt.Sprintf("%d", r.Ownership.BusFactor)},
	}
	perRow := 4
	if o.w < 72 {
		perRow = 2
	}
	var rows []string
	for i := 0; i < len(cs); i += perRow {
		var cells []string
		for j := i; j < i+perRow; j++ {
			w := o.w / perRow
			if j%perRow == perRow-1 {
				w = o.w - (o.w/perRow)*(perRow-1)
			}
			cells = append(cells, components.StatCard(cs[j].label, cs[j].value, w, o.st.Card, o.st.CardTitle, o.st.CardValue))
		}
		rows = append(rows, joinCols(0, cells...))
	}
	return strings.Join(rows, "\n")
}

func (o *Overview) owners(people []stats.Contributor, n int) string {
	if len(people) == 0 {
		return o.section("Top owners") + "\n" + o.subtle("No surviving lines attributed.")
	}
	people = people[:min(n, len(people))]
	nameW := 0
	for _, p := range people {
		nameW = max(nameW, stringWidth(p.Name))
	}
	nameW = min(nameW, max(o.w/3, 12))
	const tail = 18 // "  100.0%  1,234,567"
	barW := max(o.w-nameW-tail-4, 8)
	scale := people[0].Ownership
	if scale <= 0 {
		scale = 1
	}
	lines := []string{o.section("Top owners (surviving lines at HEAD)")}
	for _, p := range people {
		bar := o.st.Contributor(p.ID).Render(components.HBar(p.Ownership/scale, barW))
		name := fit(p.Name, nameW)
		if p.Bot {
			name = o.st.Bot.Render(name)
		} else {
			name = o.st.Contributor(p.ID).Render(name)
		}
		lines = append(lines, fmt.Sprintf("%s  %s  %s  %s", name, bar, fitRight(pct(p.Ownership), 6), fitRight(num(p.Surviving), 9)))
	}
	return strings.Join(lines, "\n")
}

func (o *Overview) activity() string {
	r := o.report()
	weeks := make([]int, len(r.Weeks))
	for i, w := range r.Weeks {
		weeks[i] = w.Commits
	}
	if len(weeks) == 0 {
		return ""
	}
	spark := o.st.Accent.Render(components.Sparkline(weeks, o.w))
	span := fmt.Sprintf("%s → %s · commits per week · timezone %s",
		r.Weeks[0].Start.Format("2006-01-02"), r.Weeks[len(r.Weeks)-1].Start.Format("2006-01-02"), r.Meta.Timezone)
	return o.section("Activity") + "\n" + spark + "\n" + o.subtle(span)
}
