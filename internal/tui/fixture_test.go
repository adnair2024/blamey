package tui

import (
	"fmt"
	"sync"
	"time"

	"github.com/adnair2024/blamey/internal/git"
	"github.com/adnair2024/blamey/internal/stats"
)

var fixedNow = time.Date(2024, 9, 10, 12, 0, 0, 0, time.UTC)

type person struct {
	name, email string
	every       int // commits every N days
	start, end  int // day offsets from 2023-10-01
	hour, tz    int
}

// fixtureReport builds a deterministic multi-author report: five humans (one
// with a CJK name, one dormant), a bot, several directories, silos and a
// single-owner file.
var (
	fixtureOnce sync.Once
	fixtureVal  stats.Report
)

// fixtureReport returns the shared report; tests treat it as read-only.
func fixtureReport() stats.Report {
	fixtureOnce.Do(func() { fixtureVal = buildFixture() })
	return fixtureVal
}

func buildFixture() stats.Report {
	base := time.Date(2023, 10, 2, 0, 0, 0, 0, time.UTC) // a Monday
	people := []person{
		{"Alice Johnson", "alice@x.io", 2, 0, 340, 10, 0},
		{"Bob Smith", "bob@x.io", 3, 20, 340, 15, -5 * 3600},
		{"李雷", "lilei@x.io", 5, 60, 300, 21, 8 * 3600},
		{"Dana Reyes", "dana@x.io", 9, 100, 340, 9, 0},
		{"Eli Dormant", "eli@x.io", 4, 0, 90, 14, 0},
		{"dependabot[bot]", "bot@x.io", 14, 30, 340, 3, 0},
	}
	paths := []string{"cmd/app/main.go", "internal/api/server.go", "internal/store/db.go", "docs/guide.md", "README.md"}
	var commits []git.Commit
	n := 0
	for _, p := range people {
		for d := p.start; d <= p.end; d += p.every {
			n++
			when := base.AddDate(0, 0, d).Add(time.Duration(p.hour) * time.Hour)
			when = when.In(time.FixedZone("", p.tz))
			f := paths[n%len(paths)]
			commits = append(commits, git.Commit{
				Hash: fmt.Sprintf("%040d", n), AuthorName: p.name, AuthorEmail: p.email, Time: when,
				Files: []git.FileChange{{Path: f, Added: 5 + n%17, Deleted: n % 5}},
			})
		}
	}
	au := func(name, email string) git.Author { return git.Author{Name: name, Email: email} }
	alice, bob, lei := au("Alice Johnson", "alice@x.io"), au("Bob Smith", "bob@x.io"), au("李雷", "lilei@x.io")
	dana, eli, bot := au("Dana Reyes", "dana@x.io"), au("Eli Dormant", "eli@x.io"), au("dependabot[bot]", "bot@x.io")
	blame := map[string]git.FileBlame{
		"cmd/app/main.go":         {Total: 200, LastTime: base.AddDate(0, 5, 0).Unix(), Lines: map[git.Author]int{alice: 150, bob: 30, dana: 20}},
		"cmd/app/flags.go":        {Total: 80, LastTime: base.AddDate(0, 8, 0).Unix(), Lines: map[git.Author]int{lei: 80}},
		"internal/api/server.go":  {Total: 600, LastTime: base.AddDate(0, 10, 0).Unix(), Lines: map[git.Author]int{bob: 350, lei: 150, alice: 100}},
		"internal/api/auth.go":    {Total: 300, LastTime: base.AddDate(0, 2, 0).Unix(), Lines: map[git.Author]int{eli: 290, alice: 10}},
		"internal/store/db.go":    {Total: 450, LastTime: base.AddDate(0, 9, 0).Unix(), Lines: map[git.Author]int{dana: 300, bob: 100, lei: 50}},
		"internal/store/sql/q.go": {Total: 120, LastTime: base.AddDate(0, 7, 0).Unix(), Lines: map[git.Author]int{dana: 120}},
		"docs/guide.md":           {Total: 90, LastTime: base.AddDate(0, 6, 0).Unix(), Lines: map[git.Author]int{lei: 60, alice: 30}},
		"README.md":               {Total: 40, LastTime: base.AddDate(0, 4, 0).Unix(), Lines: map[git.Author]int{alice: 40}},
		"go.mod":                  {Total: 12, LastTime: base.AddDate(0, 11, 0).Unix(), Lines: map[git.Author]int{bot: 12}},
	}
	return stats.Build(stats.Input{
		Meta:    stats.Meta{Path: "/home/dev/acme", Rev: "HEAD", Head: "0123456789abcdef0123456789abcdef01234567"},
		Commits: commits, Blame: blame, Now: fixedNow,
	})
}
