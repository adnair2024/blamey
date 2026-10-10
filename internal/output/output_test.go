package output

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/mattn/go-runewidth"

	"github.com/adnair2024/blamey/internal/stats"
)

func TestFormat(t *testing.T) {
	for n, want := range map[int]string{0: "0", 999: "999", 1000: "1,000", -1234567: "-1,234,567"} {
		if got := Num(n); got != want {
			t.Errorf("Num(%d) = %q, want %q", n, got, want)
		}
	}
	if got := Pct(23.0 / 35); got != "65.7%" {
		t.Errorf("Pct = %q", got)
	}
	now := time.Date(2024, 9, 10, 0, 0, 0, 0, time.UTC)
	tests := map[time.Duration]string{
		30 * time.Second: "just now", time.Hour: "1 hour ago", 3 * 24 * time.Hour: "3 days ago",
		95 * 24 * time.Hour: "3 months ago", 800 * 24 * time.Hour: "2 years ago",
	}
	for d, want := range tests {
		if got := Relative(now.Add(-d), now); got != want {
			t.Errorf("Relative(%v) = %q, want %q", d, got, want)
		}
	}
	if Ago(time.Time{}, now) != "never" {
		t.Error("zero time")
	}
}

func TestPlain(t *testing.T) {
	now := time.Date(2024, 9, 10, 0, 0, 0, 0, time.UTC)
	r := stats.Report{
		Meta:   stats.Meta{Path: "/repo", Rev: "HEAD", Head: "0123456789abcdef", Since: "2024-01-01", Warnings: []string{"shallow clone: history is incomplete"}},
		Totals: stats.Totals{SurvivingLines: 12345, Files: 3, Commits: 9, Contributors: 3, Bots: 1},
		Contributors: []stats.Contributor{
			{Name: "李雷", Commits: 5, Added: 1000, Surviving: 10000, Ownership: 0.81, LastCommit: now.Add(-48 * time.Hour)},
			{Name: "Zed", Commits: 2, Added: 5, Surviving: 5, Ownership: 0.0004, Churn: 0},
			{Name: "dependabot[bot]", Bot: true, Commits: 2},
		},
		Ownership: stats.Ownership{BusFactor: 1, TopShares: []stats.TopShare{{N: 1, Share: 0.81}, {N: 3, Share: 0.9}}},
		Silos:     []stats.Silo{{Path: "old.go", Owner: "Zed", Share: 0.9, Lines: 40}},
	}
	var buf bytes.Buffer
	Plain(&buf, r, Options{Now: now})
	out := buf.String()
	for _, want := range []string{
		"shallow clone", "ownership is always at HEAD", "12,345 surviving lines",
		"bus factor 1", "81.0%", "2 days ago", "knowledge silos", "old.go", "~",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "dependabot") {
		t.Error("bots must be hidden by default")
	}
	// Display-width alignment: the OWN% column must end at the same width on
	// every contributor row even with CJK names.
	var ends []int
	for _, l := range strings.Split(out, "\n") {
		if strings.HasPrefix(l, "李雷") || strings.HasPrefix(l, "Zed ") || strings.HasPrefix(l, "CONTRIBUTOR") {
			ends = append(ends, runewidth.StringWidth(l[:strings.Index(l, "%")+1]))
		}
	}
	if len(ends) == 3 && (ends[1] != ends[2]) {
		t.Errorf("columns misaligned: %v", ends)
	}

	buf.Reset()
	Plain(&buf, r, Options{Now: now, ShowBots: true})
	if !strings.Contains(buf.String(), "dependabot[bot] (bot)") {
		t.Error("ShowBots should list bots")
	}
}
