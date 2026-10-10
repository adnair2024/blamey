package stats

import (
	"sort"
	"time"
)

// accumulateHistory folds the commit log into per-contributor and weekly
// aggregates. Only filtered, non-binary paths contribute line counts.
func (b *builder) accumulateHistory(r *Report) {
	for _, c := range b.in.Commits {
		a := b.acc(b.res.Resolve(c.AuthorName, c.AuthorEmail))
		a.c.Commits++

		if a.c.FirstCommit.IsZero() || c.Time.Before(a.c.FirstCommit) {
			a.c.FirstCommit = c.Time
		}
		if c.Time.After(a.c.LastCommit) {
			a.c.LastCommit = c.Time
		}
		// Heat uses the author's own timezone; calendar days use the repo's.
		a.c.Heat[c.Time.Weekday()][c.Time.Hour()]++
		dn := dayNumber(c.Time, b.loc)
		a.days[dn] = struct{}{}
		a.months[c.Time.In(b.loc).Format("2006-01")] = struct{}{}
		wi := b.weeks.index(c.Time)
		a.weekly[wi]++
		r.Daily[c.Time.In(b.loc).Format("2006-01-02")]++
		r.Weeks[wi].Commits++

		size := 0
		for _, f := range c.Files {
			if f.Binary || !b.filter.Allow(f.Path) {
				continue
			}
			a.files[f.Path] = struct{}{}
			a.c.Added += f.Added
			a.c.Deleted += f.Deleted
			size += f.Added + f.Deleted
			r.Weeks[wi].Added += f.Added
			r.Weeks[wi].Deleted += f.Deleted
		}
		a.sizes = append(a.sizes, size)
	}
}

// finalizeContributors derives the ratios and returns the sorted slice.
// total is the repo's surviving line count.
func (b *builder) finalizeContributors(total int) []Contributor {
	today := dayNumber(b.now, b.loc)
	out := make([]Contributor, 0, len(b.accs))
	for _, a := range b.accs {
		c := a.c
		c.Net = c.Added - c.Deleted
		if total > 0 {
			c.Ownership = float64(c.Surviving) / float64(total)
		}
		if c.Added > 0 {
			c.SurvivalRate = float64(c.Surviving) / float64(c.Added)
		}
		c.Churn = max(0, c.Added-c.Surviving)
		c.FilesTouched = len(a.files)
		c.ActiveDays = len(a.days)
		c.LongestStreak, c.CurrentStreak = streaks(a.days, today)
		if c.Commits > 0 {
			c.AvgCommitSize = float64(c.Added+c.Deleted) / float64(c.Commits)
		}
		c.MedianCommitSize = median(a.sizes)
		c.TopLanguages = topN(a.langs, 5)
		c.TopDirs = topN(a.dirs, 5)
		c.WeeklyCommits = make([]int, b.weeks.count)
		for wi, n := range a.weekly {
			c.WeeklyCommits[wi] = n
		}
		out = append(out, c)
	}
	sort.Slice(out, func(i, j int) bool {
		x, y := out[i], out[j]
		switch {
		case x.Surviving != y.Surviving:
			return x.Surviving > y.Surviving
		case x.Commits != y.Commits:
			return x.Commits > y.Commits
		}
		return x.ID < y.ID
	})
	return out
}

func median(xs []int) float64 {
	if len(xs) == 0 {
		return 0
	}
	s := append([]int(nil), xs...)
	sort.Ints(s)
	if n := len(s); n%2 == 1 {
		return float64(s[n/2])
	}
	return float64(s[len(s)/2-1]+s[len(s)/2]) / 2
}

// silent reports whether last is zero (no known commits) or older than six
// months before now.
func silent(last, now time.Time) bool {
	return last.IsZero() || last.Before(now.AddDate(0, -6, 0))
}
