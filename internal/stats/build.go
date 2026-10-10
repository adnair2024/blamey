package stats

import (
	"sort"
	"time"

	"github.com/adnair2024/blamey/internal/git"
)

// Input is everything Build needs; it is plain data, so tests can construct it
// by hand.
type Input struct {
	Meta        Meta
	Commits     []git.Commit
	Blame       map[string]git.FileBlame
	Filter      *git.Filter // nil means default filtering
	MergeByName bool
	Now         time.Time // zero means time.Now()
}

// acc collects one contributor's numbers before they are finalized.
type acc struct {
	c     Contributor
	sizes []int
	files map[string]struct{}
	days  map[int64]struct{}
	// months holds "2006-01" keys of active months.
	months map[string]struct{}
	langs  map[string]int
	dirs   map[string]int
	weekly map[int]int
}

type builder struct {
	in     Input
	filter *git.Filter
	res    *git.Resolver
	loc    *time.Location
	now    time.Time
	weeks  weekGrid
	accs   map[string]*acc
	files  []fileAgg
}

// Build computes the full Report. Time windows apply to the history only;
// ownership always reflects the blamed revision.
func Build(in Input) Report {
	b := &builder{in: in, filter: in.Filter, accs: map[string]*acc{}}
	if b.filter == nil {
		b.filter = git.NewFilter(nil, nil)
	}
	b.now = in.Now
	if b.now.IsZero() {
		b.now = time.Now()
	}
	b.registerIdentities()
	b.loc = dominantZone(in.Commits)
	b.weeks = newWeekGrid(in.Commits, b.loc)

	r := Report{Meta: in.Meta, Daily: map[string]int{}}
	r.Meta.Timezone = b.loc.String()
	if r.Meta.GeneratedAt.IsZero() {
		r.Meta.GeneratedAt = b.now
	}

	r.Weeks = b.weeks.weeks()
	b.accumulateHistory(&r)
	b.accumulateBlame(&r)
	r.Contributors = b.finalizeContributors(r.Totals.SurvivingLines)
	b.finalizeFiles(&r)
	r.Months = b.months()
	r.Ownership = ownershipOf(r.Contributors, r.Totals.SurvivingLines)

	r.Totals.Commits = len(in.Commits)
	r.Totals.Contributors = len(r.Contributors)
	for _, c := range r.Contributors {
		if c.Bot {
			r.Totals.Bots++
		} else {
			r.Totals.Humans++
		}
	}
	return r
}

// registerIdentities shows the resolver every alias up front so merge-by-name
// unions are complete before the first Resolve.
func (b *builder) registerIdentities() {
	b.res = git.NewResolver(b.in.MergeByName)
	for _, c := range b.in.Commits {
		b.res.Add(c.AuthorName, c.AuthorEmail)
	}
	paths := make([]string, 0, len(b.in.Blame))
	for p := range b.in.Blame {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	for _, p := range paths {
		for a := range b.in.Blame[p].Lines {
			b.res.Add(a.Name, a.Email)
		}
	}
}

func (b *builder) acc(id git.Identity) *acc {
	a, ok := b.accs[id.ID]
	if !ok {
		a = &acc{
			c:      Contributor{ID: id.ID, Name: id.Name, Email: id.Email, Bot: id.Bot},
			files:  map[string]struct{}{},
			days:   map[int64]struct{}{},
			months: map[string]struct{}{},
			langs:  map[string]int{},
			dirs:   map[string]int{},
			weekly: map[int]int{},
		}
		b.accs[id.ID] = a
	}
	return a
}

// topN returns the n largest entries, ties broken by name.
func topN(m map[string]int, n int) []NamedCount {
	out := make([]NamedCount, 0, len(m))
	for k, v := range m {
		out = append(out, NamedCount{Name: k, Lines: v})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Lines != out[j].Lines {
			return out[i].Lines > out[j].Lines
		}
		return out[i].Name < out[j].Name
	})
	if len(out) > n {
		out = out[:n]
	}
	return out
}
