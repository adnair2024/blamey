package stats

import "sort"

// busFactor is the minimum number of owners whose combined lines exceed 50% of
// total. lines need not be sorted.
func busFactor(lines []int, total int) int {
	if total <= 0 {
		return 0
	}
	s := append([]int(nil), lines...)
	sort.Sort(sort.Reverse(sort.IntSlice(s)))
	sum := 0
	for i, n := range s {
		sum += n
		if sum*2 > total {
			return i + 1
		}
	}
	return len(s)
}

// gini computes the Gini coefficient of non-negative values:
//
//	G = 2·Σ(i·x_i) / (n·Σx) − (n+1)/n   with x sorted ascending, i = 1..n
//
// Returns 0 for empty or all-zero input.
func gini(xs []int) float64 {
	n := len(xs)
	if n == 0 {
		return 0
	}
	s := append([]int(nil), xs...)
	sort.Ints(s)
	var sum, weighted float64
	for i, x := range s {
		sum += float64(x)
		weighted += float64(i+1) * float64(x)
	}
	if sum == 0 {
		return 0
	}
	return 2*weighted/(float64(n)*sum) - float64(n+1)/float64(n)
}

// ownershipOf computes repo-level concentration from contributors that hold
// surviving lines. Contributors must be sorted by Surviving, descending.
func ownershipOf(cs []Contributor, total int) Ownership {
	var lines []int
	for _, c := range cs {
		if c.Surviving > 0 {
			lines = append(lines, c.Surviving)
		}
	}
	o := Ownership{Gini: gini(lines), BusFactor: busFactor(lines, total)}
	for _, n := range []int{1, 3, 5, 10} {
		sum := 0
		for i := 0; i < n && i < len(lines); i++ {
			sum += lines[i]
		}
		share := 0.0
		if total > 0 {
			share = float64(sum) / float64(total)
		}
		o.TopShares = append(o.TopShares, TopShare{N: n, Share: share})
	}
	return o
}
