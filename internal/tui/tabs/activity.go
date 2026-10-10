package tabs

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/adnair2024/blamey/internal/stats"
	"github.com/adnair2024/blamey/internal/tui/components"
	"github.com/adnair2024/blamey/internal/tui/keys"
	"github.com/adnair2024/blamey/internal/tui/styles"
)

const topSeries = 5

// Activity shows weekly commits stacked by the top contributors and a
// GitHub-style daily contribution graph.
type Activity struct{ base }

func NewActivity(st *styles.Styles, km keys.Map) *Activity {
	return &Activity{base{st: st, km: km}}
}

func (a *Activity) Name() string                  { return "Activity" }
func (a *Activity) Update(tea.Msg) (Tab, tea.Cmd) { return a, nil }

func (a *Activity) View() string {
	r := a.report()
	if r == nil {
		return ""
	}
	if len(r.Weeks) == 0 {
		return a.empty("No commits in the selected window.")
	}
	const calendarLines = 10 // title, month row, 7 days, spacer
	chartH := max(a.h-calendarLines-5, 4)
	parts := []string{a.chart(chartH), "", a.calendar()}
	return clip(strings.Join(parts, "\n"), a.h)
}

// series picks the top contributors by commits and folds the rest into one.
func (a *Activity) series() (people []stats.Contributor, others []int) {
	all := a.data.people()
	byCommits := append([]stats.Contributor(nil), all...)
	// Stable insertion by commits keeps the chart order deterministic.
	for i := 1; i < len(byCommits); i++ {
		for j := i; j > 0 && byCommits[j].Commits > byCommits[j-1].Commits; j-- {
			byCommits[j], byCommits[j-1] = byCommits[j-1], byCommits[j]
		}
	}
	if len(byCommits) <= topSeries {
		return byCommits, nil
	}
	others = make([]int, len(a.report().Weeks))
	for _, c := range byCommits[topSeries:] {
		for i, n := range c.WeeklyCommits {
			others[i] += n
		}
	}
	return byCommits[:topSeries], others
}

func (a *Activity) chart(height int) string {
	r := a.report()
	people, others := a.series()
	axisW := 6
	cols := min(len(r.Weeks), max(a.w-axisW, 8))

	var styl []lipgloss.Style
	var stacks [][]int
	for _, p := range people {
		styl = append(styl, a.st.Contributor(p.ID))
		stacks = append(stacks, components.Bucket(p.WeeklyCommits, cols))
	}
	if others != nil {
		styl = append(styl, a.st.Other())
		stacks = append(stacks, components.Bucket(others, cols))
	}
	if len(stacks) == 0 {
		return a.empty("No contributors to chart (press b to show bots).")
	}
	n := len(stacks[0])
	columns := make([][]int, n)
	peak := 0
	for x := 0; x < n; x++ {
		columns[x] = make([]int, len(stacks))
		total := 0
		for s := range stacks {
			columns[x][s] = stacks[s][x]
			total += stacks[s][x]
		}
		peak = max(peak, total)
	}

	rows := strings.Split(components.StackedColumns(columns, styl, height), "\n")
	var b strings.Builder
	b.WriteString(a.section("Commits per week") + a.subtle(bucketNote(len(r.Weeks), n)) + "\n")
	for i, row := range rows {
		label := ""
		switch i {
		case 0:
			label = num(peak)
		case len(rows) - 1:
			label = "0"
		}
		b.WriteString(a.subtle(fitRight(label, axisW-1)) + " " + row + "\n")
	}
	b.WriteString(fit("", axisW) + a.subtle(r.Weeks[0].Start.Format("2006-01")) +
		a.subtle(fitRight(r.Weeks[len(r.Weeks)-1].Start.Format("2006-01"), max(n-7, 0))) + "\n")
	b.WriteString(a.legend(people, others != nil))
	return b.String()
}

func bucketNote(weeks, cols int) string {
	if weeks <= cols {
		return ""
	}
	return fmt.Sprintf("  (%d weeks per column)", (weeks+cols-1)/cols)
}

func (a *Activity) legend(people []stats.Contributor, hasOthers bool) string {
	var items []string
	for _, p := range people {
		items = append(items, a.st.Contributor(p.ID).Render("█")+" "+p.Name)
	}
	if hasOthers {
		items = append(items, a.st.Other().Render("█")+" others")
	}
	return strings.Join(items, "  ")
}

// calendar draws up to 53 weeks of daily commits, Monday on top.
func (a *Activity) calendar() string {
	r := a.report()
	weeks := r.Weeks
	pad, cellW := " ", 2
	maxWeeks := 53
	if a.w < 4+maxWeeks*cellW {
		pad, cellW = "", 1
	}
	maxWeeks = min(maxWeeks, max((a.w-4)/cellW, 4))
	if len(weeks) > maxWeeks {
		weeks = weeks[len(weeks)-maxWeeks:]
	}
	grid := make([][]int, 7)
	for d := range grid {
		grid[d] = make([]int, len(weeks))
		for w, wk := range weeks {
			grid[d][w] = r.Daily[wk.Start.AddDate(0, 0, d).Format("2006-01-02")]
		}
	}
	rows := strings.Split(components.Heatmap(grid, a.st.Heat, a.st.HeatGlyphs, pad), "\n")

	// Month names start at the column of the month's first week; a label is
	// skipped when it would overlap the previous one.
	var months strings.Builder
	last, col := "", 0
	for i, wk := range weeks {
		m := wk.Start.Format("Jan")
		if m == last {
			continue
		}
		last = m
		if at := i * cellW; at >= col && at+len(m) <= len(weeks)*cellW {
			months.WriteString(strings.Repeat(" ", at-col) + m)
			col = at + len(m)
		}
	}
	labels := []string{"Mon", "", "Wed", "", "Fri", "", ""}
	out := []string{a.section("Daily commits"), a.subtle(fit("", 4) + months.String())}
	for d, row := range rows {
		out = append(out, a.subtle(fit(labels[d], 4))+row)
	}
	return strings.Join(out, "\n")
}
