package git

import (
	"regexp"
	"sort"
	"strings"
)

// Identity is one human (or bot) after merging aliases.
type Identity struct {
	ID    string // stable key, also used to hash a display color
	Name  string // most frequently seen name
	Email string // most frequently seen email
	Bot   bool
}

var noreplyRe = regexp.MustCompile(`^(?:\d+\+)?([^@]+)@users\.noreply\.github\.com$`)

// emailKey normalizes an email for merging: lowercased, and GitHub noreply
// addresses with and without the numeric prefix collapse to one key.
func emailKey(email string) string {
	e := strings.ToLower(strings.TrimSpace(email))
	if m := noreplyRe.FindStringSubmatch(e); m != nil {
		return m[1] + "@users.noreply.github.com"
	}
	return e
}

// NormalizeName lowercases and collapses whitespace.
func NormalizeName(name string) string {
	return strings.ToLower(strings.Join(strings.Fields(name), " "))
}

// IsBot reports whether a name or email looks like an automation account.
func IsBot(name, email string) bool {
	return strings.HasSuffix(strings.ToLower(strings.TrimSpace(name)), "[bot]") ||
		strings.Contains(strings.ToLower(email), "[bot]@")
}

// Resolver merges author aliases. Mailmap is already applied by git, so the
// remaining rules are, in order: same email (case-insensitive), GitHub noreply
// equivalence, and optionally the same normalized name.
//
// Call Add for every commit first, then Resolve; Resolve on an unseen pair
// still works but registers it as a new alias. Not safe for concurrent use.
type Resolver struct {
	mergeByName bool
	parent      map[string]string // union-find over email keys
	names       map[string]map[string]int
	emails      map[string]map[string]int
	nameOwner   map[string]string // normalized name -> first email key
	bot         map[string]bool
	cache       map[string]Identity
}

func NewResolver(mergeByName bool) *Resolver {
	return &Resolver{
		mergeByName: mergeByName,
		parent:      map[string]string{},
		names:       map[string]map[string]int{},
		emails:      map[string]map[string]int{},
		nameOwner:   map[string]string{},
		bot:         map[string]bool{},
	}
}

func (r *Resolver) find(k string) string {
	for r.parent[k] != k {
		r.parent[k] = r.parent[r.parent[k]]
		k = r.parent[k]
	}
	return k
}

func (r *Resolver) union(a, b string) {
	ra, rb := r.find(a), r.find(b)
	if ra == rb {
		return
	}
	// Smaller key becomes the root so IDs do not depend on input order.
	if rb < ra {
		ra, rb = rb, ra
	}
	r.parent[rb] = ra
}

// Add records one observation of (name, email).
func (r *Resolver) Add(name, email string) {
	r.cache = nil
	k := emailKey(email)
	if _, ok := r.parent[k]; !ok {
		r.parent[k] = k
		r.names[k] = map[string]int{}
		r.emails[k] = map[string]int{}
	}
	r.names[k][name]++
	r.emails[k][email]++
	if IsBot(name, email) {
		r.bot[k] = true
	}
	if r.mergeByName {
		if n := NormalizeName(name); n != "" {
			if owner, ok := r.nameOwner[n]; ok {
				r.union(owner, k)
			} else {
				r.nameOwner[n] = k
			}
		}
	}
}

// Resolve returns the merged identity for a (name, email) pair.
func (r *Resolver) Resolve(name, email string) Identity {
	k := emailKey(email)
	if _, ok := r.parent[k]; !ok {
		r.Add(name, email)
	}
	if r.cache == nil {
		r.buildCache()
	}
	return r.cache[r.find(k)]
}

func (r *Resolver) buildCache() {
	names := map[string]map[string]int{}
	emails := map[string]map[string]int{}
	bot := map[string]bool{}
	for k := range r.parent {
		root := r.find(k)
		if names[root] == nil {
			names[root], emails[root] = map[string]int{}, map[string]int{}
		}
		for n, c := range r.names[k] {
			names[root][n] += c
		}
		for e, c := range r.emails[k] {
			emails[root][e] += c
		}
		bot[root] = bot[root] || r.bot[k]
	}
	r.cache = make(map[string]Identity, len(names))
	for root := range names {
		r.cache[root] = Identity{
			ID:    root,
			Name:  mostCommon(names[root]),
			Email: mostCommon(emails[root]),
			Bot:   bot[root],
		}
	}
}

// mostCommon picks the highest count, breaking ties alphabetically so the
// result is deterministic.
func mostCommon(m map[string]int) string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	best, bestN := "", -1
	for _, k := range keys {
		if m[k] > bestN {
			best, bestN = k, m[k]
		}
	}
	return best
}
