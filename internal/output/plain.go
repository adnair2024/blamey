package output

import (
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/mattn/go-runewidth"

	"github.com/adnair2024/blamey/internal/stats"
)

// Options tune the plain summary.
type Options struct {
	ShowBots bool
	TopN     int       // contributors listed; default 15
	Now      time.Time // for relative times; zero means time.Now()
}

// Plain writes a human-readable summary with no colors or cursor control, so
// it is safe in CI logs and pipes.
func Plain(w io.Writer, r stats.Report, o Options) {
	if o.TopN <= 0 {
		o.TopN = 15
	}
	if o.Now.IsZero() {
		o.Now = time.Now()
	}
	m, t := r.Meta, r.Totals

	fmt.Fprintf(w, "blamey: %s @ %s\n", m.Path, shortRev(m))
	for _, warn := range m.Warnings {
		fmt.Fprintf(w, "warning: %s\n", warn)
	}
	if m.Since != "" || m.Until != "" {
		fmt.Fprintf(w, "history window: since=%q until=%q (ownership is always at %s and ignores the window)\n",
			m.Since, m.Until, m.Rev)
	}
	fmt.Fprintf(w, "\n%s surviving lines in %s files · %s commits · %d contributors (%d bots)\n",
		Num(t.SurvivingLines), Num(t.Files), Num(t.Commits), t.Contributors, t.Bots)
	fmt.Fprintf(w, "bus factor %d · gini %.2f", r.Ownership.BusFactor, r.Ownership.Gini)
	for _, ts := range r.Ownership.TopShares {
		if ts.N == 1 || ts.N == 3 {
			fmt.Fprintf(w, " · top %d own %s", ts.N, Pct(ts.Share))
		}
	}
	fmt.Fprint(w, "\n\n")

	writeContributors(w, r, o)
	writeSilos(w, r, o)
	fmt.Fprintln(w, "\n~ marks approximations (churn). Line counts measure volume, not value.")
}

func shortRev(m stats.Meta) string {
	if len(m.Head) >= 8 {
		return m.Rev + " (" + m.Head[:8] + ")"
	}
	return m.Rev
}

func writeContributors(w io.Writer, r stats.Report, o Options) {
	header := []string{"CONTRIBUTOR", "COMMITS", "+LINES", "-LINES", "SURVIVING", "OWN%", "~CHURN", "LAST ACTIVE"}
	rows := [][]string{header}
	shown := 0
	for _, c := range r.Contributors {
		if c.Bot && !o.ShowBots {
			continue
		}
		if shown == o.TopN {
			break
		}
		shown++
		name := c.Name
		if c.Bot {
			name += " (bot)"
		}
		rows = append(rows, []string{
			name, Num(c.Commits), Num(c.Added), Num(c.Deleted), Num(c.Surviving),
			Pct(c.Ownership), "~" + Num(c.Churn), Ago(c.LastCommit, o.Now),
		})
	}
	writeTable(w, rows, map[int]bool{0: true, 7: true})
}

func writeSilos(w io.Writer, r stats.Report, o Options) {
	if len(r.Silos) == 0 {
		return
	}
	fmt.Fprintf(w, "\nknowledge silos (>80%% one owner, silent 6+ months): %d files\n", len(r.Silos))
	rows := [][]string{{"FILE", "OWNER", "SHARE", "LINES", "OWNER LAST ACTIVE"}}
	for i, s := range r.Silos {
		if i == 10 {
			break
		}
		rows = append(rows, []string{s.Path, s.Owner, Pct(s.Share), Num(s.Lines), Ago(s.LastCommit, o.Now)})
	}
	writeTable(w, rows, map[int]bool{0: true, 1: true, 4: true})
}

// writeTable pads columns by display width (names may contain CJK or emoji).
// Columns in left are left-aligned, the rest right-aligned.
func writeTable(w io.Writer, rows [][]string, left map[int]bool) {
	widths := make([]int, len(rows[0]))
	for _, row := range rows {
		for i, cell := range row {
			widths[i] = max(widths[i], runewidth.StringWidth(cell))
		}
	}
	for _, row := range rows {
		var b strings.Builder
		for i, cell := range row {
			pad := strings.Repeat(" ", widths[i]-runewidth.StringWidth(cell))
			if i > 0 {
				b.WriteString("  ")
			}
			if left[i] {
				b.WriteString(cell + pad)
			} else {
				b.WriteString(pad + cell)
			}
		}
		fmt.Fprintln(w, strings.TrimRight(b.String(), " "))
	}
}
