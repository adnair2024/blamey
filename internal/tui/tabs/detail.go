package tabs

import (
	"fmt"
	"strings"
	"time"

	"github.com/adnair2024/blamey/internal/output"
	"github.com/adnair2024/blamey/internal/stats"
	"github.com/adnair2024/blamey/internal/tui/components"
)

// ago renders a relative time, or "never" for the zero time.
func (b *base) ago(t time.Time) string {
	if t.IsZero() {
		return "never"
	}
	return output.Relative(t, b.data.Now)
}

// weekdayOrder lists weekdays Monday first, as people read a week.
var weekdayOrder = []time.Weekday{
	time.Monday, time.Tuesday, time.Wednesday, time.Thursday, time.Friday, time.Saturday, time.Sunday,
}

// detailView renders every per-contributor metric plus language and directory
// breakdowns, the hour×weekday heatmap and the weekly activity sparkline.
func (c *Contributors) detailView(p stats.Contributor) string {
	var out []string
	title := c.st.Contributor(p.ID).Bold(true).Render(p.Name)
	if p.Bot {
		title += c.st.Bot.Render("  (bot)")
	}
	out = append(out, title+c.subtle("  <"+p.Email+">"), "")

	out = append(out, c.metrics(p), "")
	out = append(out, c.breakdown("Top languages", p.TopLanguages, p.Surviving), "")
	out = append(out, c.breakdown("Top directories", p.TopDirs, p.Surviving), "")
	out = append(out, c.punchcard(p), "")
	out = append(out, c.weekly(p))
	return strings.Join(out, "\n")
}

func (c *Contributors) metrics(p stats.Contributor) string {
	date := func(t time.Time) string {
		if t.IsZero() {
			return "never"
		}
		return fmt.Sprintf("%s (%s)", c.ago(t), t.Format("2006-01-02"))
	}
	const lw = 18
	left := []string{
		c.kv("Commits", num(p.Commits), lw),
		c.kv("Lines added", num(p.Added), lw),
		c.kv("Lines deleted", num(p.Deleted), lw),
		c.kv("Net lines", num(p.Net), lw),
		c.kv("Surviving lines", num(p.Surviving), lw),
		c.kv("Ownership", pct(p.Ownership), lw),
		c.kv("Survival rate", pct(p.SurvivalRate), lw),
		c.kv("~Churn", "~"+num(p.Churn), lw),
		c.kv("Files touched", num(p.FilesTouched), lw),
	}
	right := []string{
		c.kv("Files owned", num(p.FilesOwned), lw),
		c.kv("First commit", date(p.FirstCommit), lw),
		c.kv("Last commit", date(p.LastCommit), lw),
		c.kv("Active days", num(p.ActiveDays), lw),
		c.kv("Longest streak", plural(p.LongestStreak, "day"), lw),
		c.kv("Current streak", plural(p.CurrentStreak, "day"), lw),
		c.kv("Avg commit size", fmt.Sprintf("%.1f lines", p.AvgCommitSize), lw),
		c.kv("Median commit", fmt.Sprintf("%.1f lines", p.MedianCommitSize), lw),
		c.kv("Peak", c.peak(p), lw),
	}
	if c.w >= 96 {
		return joinCols(4, strings.Join(left, "\n"), strings.Join(right, "\n"))
	}
	return strings.Join(append(left, right...), "\n")
}

// peak names the busiest weekday and hour from the heat grid.
func (c *Contributors) peak(p stats.Contributor) string {
	if p.Commits == 0 {
		return "—"
	}
	days, hours := p.WeekdayCounts(), p.HourCounts()
	bd, bh := 0, 0
	for i, n := range days {
		if n > days[bd] {
			bd = i
		}
	}
	for i, n := range hours {
		if n > hours[bh] {
			bh = i
		}
	}
	return fmt.Sprintf("%s, %02d:00", time.Weekday(bd), bh)
}

// breakdown draws labelled bars relative to the largest entry, with the share
// of the person's surviving lines.
func (c *Contributors) breakdown(title string, items []stats.NamedCount, total int) string {
	lines := []string{c.section(title)}
	if len(items) == 0 {
		return strings.Join(append(lines, c.subtle("none")), "\n")
	}
	nameW := 0
	for _, it := range items {
		nameW = max(nameW, stringWidth(it.Name))
	}
	nameW = min(nameW, 24)
	barW := max(min(c.w-nameW-24, 40), 6)
	for _, it := range items {
		frac := float64(it.Lines) / float64(items[0].Lines)
		share := 0.0
		if total > 0 {
			share = float64(it.Lines) / float64(total)
		}
		lines = append(lines, fmt.Sprintf("%s  %s  %s  %s", fit(it.Name, nameW),
			c.st.Bar.Render(components.HBar(frac, barW)), fitRight(num(it.Lines), 8), fitRight(pct(share), 6)))
	}
	return strings.Join(lines, "\n")
}

// punchcard is the weekday × hour heatmap in the author's own commit timezone.
func (c *Contributors) punchcard(p stats.Contributor) string {
	rows := make([][]int, len(weekdayOrder))
	for i, d := range weekdayOrder {
		rows[i] = p.Heat[d][:]
	}
	grid := strings.Split(components.Heatmap(rows, c.st.Heat, c.st.HeatGlyphs, " "), "\n")
	var b strings.Builder
	b.WriteString(c.section("Commits by weekday and hour (author's own timezone)") + "\n")
	hdr := "    "
	for h := 0; h < 24; h++ {
		if h%3 == 0 {
			hdr += fit(fmt.Sprintf("%02d", h), 6)
		}
	}
	b.WriteString(c.subtle(strings.TrimRight(hdr, " ")) + "\n")
	for i, d := range weekdayOrder {
		b.WriteString(c.subtle(fit(d.String()[:3], 4)) + grid[i])
		if i < len(weekdayOrder)-1 {
			b.WriteString("\n")
		}
	}
	return b.String()
}

func (c *Contributors) weekly(p stats.Contributor) string {
	head := c.section("Weekly commits")
	if len(p.WeeklyCommits) == 0 {
		return head + "\n" + c.subtle("no activity")
	}
	spark := c.st.Contributor(p.ID).Render(components.Sparkline(p.WeeklyCommits, c.w))
	return head + "\n" + spark
}
