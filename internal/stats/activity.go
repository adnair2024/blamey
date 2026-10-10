package stats

import (
	"sort"
	"time"

	"github.com/adnair2024/blamey/internal/git"
)

// dominantZone picks the most common commit UTC offset; ties go to the smaller
// absolute offset, then the smaller value. No commits means UTC.
func dominantZone(commits []git.Commit) *time.Location {
	counts := map[int]int{}
	for _, c := range commits {
		_, off := c.Time.Zone()
		counts[off]++
	}
	best, bestN := 0, 0
	for off, n := range counts {
		if n > bestN || (n == bestN && lessOffset(off, best)) {
			best, bestN = off, n
		}
	}
	if bestN == 0 || best == 0 {
		return time.UTC
	}
	name := time.Unix(0, 0).In(time.FixedZone("", best)).Format("-0700")
	return time.FixedZone(name, best)
}

func lessOffset(a, b int) bool {
	abs := func(x int) int {
		if x < 0 {
			return -x
		}
		return x
	}
	if abs(a) != abs(b) {
		return abs(a) < abs(b)
	}
	return a < b
}

// dayNumber is the calendar day of t in loc as days since 1970-01-01.
func dayNumber(t time.Time, loc *time.Location) int64 {
	y, m, d := t.In(loc).Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC).Unix() / 86400
}

// mondayOf returns the day number of the Monday on or before dn.
// 1970-01-01 was a Thursday, hence the +3.
func mondayOf(dn int64) int64 {
	return dn - ((dn+3)%7+7)%7
}

// weekGrid maps days to indexes in a contiguous run of Monday-based weeks.
type weekGrid struct {
	first int64 // day number of the first Monday
	count int
	loc   *time.Location
}

func newWeekGrid(commits []git.Commit, loc *time.Location) weekGrid {
	if len(commits) == 0 {
		return weekGrid{loc: loc}
	}
	lo, hi := dayNumber(commits[0].Time, loc), dayNumber(commits[0].Time, loc)
	for _, c := range commits[1:] {
		d := dayNumber(c.Time, loc)
		lo, hi = min(lo, d), max(hi, d)
	}
	first := mondayOf(lo)
	return weekGrid{first: first, count: int((mondayOf(hi)-first)/7) + 1, loc: loc}
}

func (g weekGrid) index(t time.Time) int {
	return int((mondayOf(dayNumber(t, g.loc)) - g.first) / 7)
}

func (g weekGrid) weeks() []Week {
	out := make([]Week, g.count)
	for i := range out {
		u := time.Unix((g.first+int64(i)*7)*86400, 0).UTC()
		out[i].Start = time.Date(u.Year(), u.Month(), u.Day(), 0, 0, 0, 0, g.loc)
	}
	return out
}

// streaks returns the longest run of consecutive days, and the run that ends
// on today or yesterday (0 if the latest active day is older).
func streaks(days map[int64]struct{}, today int64) (longest, current int) {
	if len(days) == 0 {
		return 0, 0
	}
	ds := make([]int64, 0, len(days))
	for d := range days {
		ds = append(ds, d)
	}
	sort.Slice(ds, func(i, j int) bool { return ds[i] < ds[j] })
	run := 1
	longest = 1
	for i := 1; i < len(ds); i++ {
		if ds[i] == ds[i-1]+1 {
			run++
		} else {
			run = 1
		}
		longest = max(longest, run)
	}
	if last := ds[len(ds)-1]; last >= today-1 {
		current = run
	}
	return longest, current
}

// months builds the new-vs-returning contributors series over the month range
// of recorded activity. A contributor is "new" in the first month they were
// active and "returning" in every later month they were active again.
func (b *builder) months() []Month {
	first := map[string]string{}
	active := map[string]map[string]bool{}
	for id, a := range b.accs {
		for m := range a.months {
			if f, ok := first[id]; !ok || m < f {
				first[id] = m
			}
			if active[m] == nil {
				active[m] = map[string]bool{}
			}
			active[m][id] = true
		}
	}
	if len(active) == 0 {
		return nil
	}
	keys := make([]string, 0, len(active))
	for m := range active {
		keys = append(keys, m)
	}
	sort.Strings(keys)

	var out []Month
	cur, _ := time.Parse("2006-01", keys[0])
	end, _ := time.Parse("2006-01", keys[len(keys)-1])
	for ; !cur.After(end); cur = cur.AddDate(0, 1, 0) {
		m := Month{Month: cur.Format("2006-01")}
		for id := range active[m.Month] {
			if first[id] == m.Month {
				m.New++
			} else {
				m.Returning++
			}
		}
		out = append(out, m)
	}
	return out
}
