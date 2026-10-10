package stats

import (
	"math"
	"reflect"
	"testing"
	"time"

	"github.com/adnair2024/blamey/internal/git"
)

func at(layout string, tz int) time.Time {
	t, err := time.Parse("2006-01-02 15:04", layout)
	if err != nil {
		panic(err)
	}
	loc := time.FixedZone("", tz)
	return time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), 0, 0, loc)
}

func commit(name, email, when string, tz int, files ...git.FileChange) git.Commit {
	return git.Commit{Hash: name + when, AuthorName: name, AuthorEmail: email, Time: at(when, tz), Files: files}
}

func fc(p string, add, del int) git.FileChange {
	return git.FileChange{Path: p, Added: add, Deleted: del}
}

func near(t *testing.T, name string, got, want float64) {
	t.Helper()
	if math.Abs(got-want) > 1e-9 {
		t.Errorf("%s = %v, want %v", name, got, want)
	}
}

// fixture is hand-computed; see the expected values in the tests below.
func fixture() Input {
	const ist = 5*3600 + 1800
	commits := []git.Commit{
		commit("Alice", "alice@x", "2024-01-01 10:00", 0, fc("a.go", 10, 0), fc("README.md", 2, 0)),
		commit("Alice", "alice@x", "2024-01-02 23:00", 0, fc("a.go", 4, 2)),
		commit("Alice", "alice@x", "2024-01-03 09:00", 0, fc("vendor/x.go", 100, 0), fc("b.go", 1, 0),
			git.FileChange{Path: "logo.png", Binary: true}),
		commit("Bob", "bob@x", "2024-01-09 15:00", ist, fc("b.go", 20, 1)),
		commit("dependabot[bot]", "bot@x", "2024-02-01 10:00", 0, fc("go.sum", 50, 0)),
		commit("Bob", "bob@x", "2024-09-09 08:00", 0, fc("b.go", 5, 5)),
	}
	blame := map[string]git.FileBlame{
		"a.go":          {Total: 10, Lines: map[git.Author]int{{Name: "Alice", Email: "alice@x"}: 8, {Name: "Bob", Email: "bob@x"}: 2}, LastTime: 1000},
		"b.go":          {Total: 20, Lines: map[git.Author]int{{Name: "Bob", Email: "bob@x"}: 18, {Name: "Alice", Email: "alice@x"}: 2}},
		"README.md":     {Total: 2, Lines: map[git.Author]int{{Name: "Alice", Email: "alice@x"}: 2}},
		"lib/util/z.go": {Total: 3, Lines: map[git.Author]int{{Name: "Bob", Email: "Bob@X"}: 2, {Name: "Bobby", Email: "bob@x"}: 1}},
		"vendor/x.go":   {Total: 100, Lines: map[git.Author]int{{Name: "Alice", Email: "alice@x"}: 100}},
	}
	return Input{Commits: commits, Blame: blame, Now: at("2024-09-10 12:00", 0)}
}

func byName(r Report, name string) Contributor {
	for _, c := range r.Contributors {
		if c.Name == name {
			return c
		}
	}
	panic("no contributor " + name)
}

func TestBuildTotalsAndOrdering(t *testing.T) {
	r := Build(fixture())
	if r.Totals.SurvivingLines != 35 || r.Totals.Commits != 6 || r.Totals.Files != 4 {
		t.Errorf("totals = %+v", r.Totals)
	}
	if r.Totals.Contributors != 3 || r.Totals.Humans != 2 || r.Totals.Bots != 1 {
		t.Errorf("contributor counts = %+v", r.Totals)
	}
	var names []string
	for _, c := range r.Contributors {
		names = append(names, c.Name)
	}
	if !reflect.DeepEqual(names, []string{"Bob", "Alice", "dependabot[bot]"}) {
		t.Errorf("order = %v", names)
	}
	if r.Meta.Timezone != "UTC" {
		t.Errorf("timezone = %q", r.Meta.Timezone)
	}
}

func TestAlice(t *testing.T) {
	a := byName(Build(fixture()), "Alice")
	if a.Commits != 3 || a.Added != 17 || a.Deleted != 2 || a.Net != 15 {
		t.Errorf("history numbers: %+v", a)
	}
	if a.Surviving != 12 || a.FilesOwned != 2 || a.FilesTouched != 3 {
		t.Errorf("surviving/owned/touched: %d %d %d", a.Surviving, a.FilesOwned, a.FilesTouched)
	}
	near(t, "ownership", a.Ownership, 12.0/35)
	near(t, "survival", a.SurvivalRate, 12.0/17)
	if a.Churn != 5 {
		t.Errorf("churn = %d", a.Churn)
	}
	near(t, "avg size", a.AvgCommitSize, 19.0/3) // (12+6+1)/3
	near(t, "median size", a.MedianCommitSize, 6)
	if a.ActiveDays != 3 || a.LongestStreak != 3 || a.CurrentStreak != 0 {
		t.Errorf("days/streaks: %d %d %d", a.ActiveDays, a.LongestStreak, a.CurrentStreak)
	}
	if !a.FirstCommit.Equal(at("2024-01-01 10:00", 0)) || !a.LastCommit.Equal(at("2024-01-03 09:00", 0)) {
		t.Errorf("first/last = %v / %v", a.FirstCommit, a.LastCommit)
	}
	wantLang := []NamedCount{{"Go", 10}, {"Markdown", 2}}
	if !reflect.DeepEqual(a.TopLanguages, wantLang) {
		t.Errorf("languages = %v", a.TopLanguages)
	}
	if a.Heat[time.Monday][10] != 1 || a.Heat[time.Tuesday][23] != 1 || a.Heat[time.Wednesday][9] != 1 {
		t.Errorf("heat wrong: %v", a.Heat)
	}
}

func TestBobMergedAliasesAndOwnTimezone(t *testing.T) {
	r := Build(fixture())
	b := byName(r, "Bob")
	if b.Surviving != 23 || b.Commits != 2 || b.Added != 25 || b.Deleted != 6 {
		t.Errorf("bob = %+v", b)
	}
	near(t, "median", b.MedianCommitSize, 15.5)
	if b.CurrentStreak != 1 || b.LongestStreak != 1 {
		t.Errorf("streaks %d %d", b.CurrentStreak, b.LongestStreak)
	}
	// 15:00 in the author's own +0530 zone, not 09:30 UTC.
	if b.Heat[time.Tuesday][15] != 1 || b.HourCounts()[15] != 1 || b.WeekdayCounts()[time.Tuesday] != 1 {
		t.Errorf("own-timezone heat wrong: %v", b.Heat)
	}
	wantDirs := []NamedCount{{".", 20}, {"lib", 3}}
	if !reflect.DeepEqual(b.TopDirs, wantDirs) {
		t.Errorf("dirs = %v", b.TopDirs)
	}
}

func TestOwnershipMetrics(t *testing.T) {
	r := Build(fixture())
	if r.Ownership.BusFactor != 1 {
		t.Errorf("bus factor = %d", r.Ownership.BusFactor)
	}
	near(t, "gini", r.Ownership.Gini, 11.0/70)
	near(t, "top1", r.Ownership.TopShares[0].Share, 23.0/35)
	near(t, "top3", r.Ownership.TopShares[1].Share, 1) // only two owners
}

func TestFilesDirsSilos(t *testing.T) {
	r := Build(fixture())
	if len(r.Files) != 4 || r.Files[0].Path != "b.go" {
		t.Fatalf("files = %+v", r.Files)
	}
	for _, f := range r.Files {
		if f.Path == "vendor/x.go" {
			t.Error("vendored file leaked into stats")
		}
	}
	// a.go: Alice holds exactly 80%, which is not "more than 80%".
	if len(r.Silos) != 1 || r.Silos[0].Path != "README.md" || r.Silos[0].Owner != "Alice" {
		t.Errorf("silos = %+v", r.Silos)
	}
	var util *DirStat
	for i := range r.Dirs {
		if r.Dirs[i].Path == "lib/util" {
			util = &r.Dirs[i]
		}
	}
	if util == nil || util.Lines != 3 || util.BusFactor != 1 || util.Owners[0].Name != "Bob" {
		t.Errorf("lib/util = %+v", util)
	}
	if len(r.Dirs) != 3 || r.Dirs[0].Path != "." || r.Dirs[0].Lines != 35 {
		t.Errorf("dirs = %+v", r.Dirs)
	}
}

func TestActivitySeries(t *testing.T) {
	r := Build(fixture())
	if len(r.Weeks) != 37 {
		t.Fatalf("weeks = %d, want 37", len(r.Weeks))
	}
	if w := r.Weeks[0]; w.Commits != 3 || w.Added != 17 || w.Deleted != 2 || w.Start.Weekday() != time.Monday {
		t.Errorf("week 0 = %+v", w)
	}
	if r.Weeks[1].Commits != 1 || r.Weeks[4].Commits != 1 || r.Weeks[36].Commits != 1 {
		t.Errorf("week commits wrong")
	}
	if got := byName(r, "Alice").WeeklyCommits; len(got) != 37 || got[0] != 3 {
		t.Errorf("alice weekly = %v", got)
	}
	if r.Daily["2024-01-01"] != 1 || r.Daily["2024-09-09"] != 1 {
		t.Errorf("daily = %v", r.Daily)
	}
	if len(r.Months) != 9 || r.Months[0].New != 2 || r.Months[1].New != 1 || r.Months[8].Returning != 1 {
		t.Errorf("months = %+v", r.Months)
	}
}

func TestBuildEmpty(t *testing.T) {
	r := Build(Input{})
	if r.Totals.Commits != 0 || len(r.Contributors) != 0 || len(r.Weeks) != 0 || r.Ownership.BusFactor != 0 {
		t.Errorf("empty report not empty: %+v", r)
	}
}

func TestBusFactor(t *testing.T) {
	tests := []struct {
		lines []int
		total int
		want  int
	}{
		{[]int{51, 49}, 100, 1},
		{[]int{50, 50}, 100, 2}, // exactly 50% does not exceed it
		{[]int{30, 30, 40}, 100, 2},
		{[]int{10, 10, 10, 10, 10, 10}, 60, 4},
		{nil, 0, 0},
	}
	for _, tt := range tests {
		if got := busFactor(tt.lines, tt.total); got != tt.want {
			t.Errorf("busFactor(%v) = %d, want %d", tt.lines, got, tt.want)
		}
	}
}

func TestGini(t *testing.T) {
	near(t, "equal", gini([]int{5, 5, 5, 5}), 0)
	near(t, "empty", gini(nil), 0)
	near(t, "zeros", gini([]int{0, 0}), 0)
	// One owner of four: 2·(4·10)/(4·10) − 5/4 = 0.75.
	near(t, "one of four", gini([]int{0, 0, 0, 10}), 0.75)
}

func TestHelpers(t *testing.T) {
	if got := dirsOf("a/b/c.go"); !reflect.DeepEqual(got, []string{".", "a", "a/b"}) {
		t.Errorf("dirsOf = %v", got)
	}
	if got := dirsOf("x.go"); !reflect.DeepEqual(got, []string{"."}) {
		t.Errorf("dirsOf root = %v", got)
	}
	// 2024-01-03 is a Wednesday; its Monday is 2024-01-01.
	d := dayNumber(at("2024-01-03 01:00", 0), time.UTC)
	if m := mondayOf(d); m != dayNumber(at("2024-01-01 00:00", 0), time.UTC) {
		t.Errorf("mondayOf = %d", m)
	}
	// Tie between +0000 and +0100 resolves to the smaller offset.
	cs := []git.Commit{commit("a", "a", "2024-01-01 00:00", 0), commit("a", "a", "2024-01-01 00:00", 3600)}
	if _, off := time.Now().In(dominantZone(cs)).Zone(); off != 0 {
		t.Errorf("tie offset = %d", off)
	}
	if languageOf("Makefile") != "Makefile" || languageOf("x/Y.GO") != "Go" || languageOf("a.zzz") != "Other" {
		t.Error("languageOf")
	}
	if median([]int{1, 2, 3, 10}) != 2.5 || median(nil) != 0 {
		t.Error("median")
	}
}
