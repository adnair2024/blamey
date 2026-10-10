// Package stats turns parsed git data into a Report. It is pure computation:
// no I/O and no git calls.
//
// Ratios are stored unrounded (fractions in [0,1]); formatting to a percentage
// happens at render time.
package stats

import "time"

// Report is the single output of the stats layer. The TUI, --plain and --json
// all render this same value.
type Report struct {
	Meta         Meta           `json:"meta"`
	Totals       Totals         `json:"totals"`
	Contributors []Contributor  `json:"contributors"` // by surviving lines, then commits
	Ownership    Ownership      `json:"ownership"`
	Silos        []Silo         `json:"silos"`
	Files        []FileStat     `json:"files"` // by surviving lines
	Dirs         []DirStat      `json:"dirs"`  // every directory; "." is the root
	Weeks        []Week         `json:"weeks"` // contiguous, Monday-based
	Months       []Month        `json:"months"`
	Daily        map[string]int `json:"daily_commits"` // "2006-01-02" -> commits
}

// Meta describes what was analyzed.
type Meta struct {
	Path        string    `json:"path"`
	Rev         string    `json:"rev"`
	Head        string    `json:"head,omitempty"`
	Since       string    `json:"since,omitempty"`
	Until       string    `json:"until,omitempty"`
	Shallow     bool      `json:"shallow"`
	GeneratedAt time.Time `json:"generated_at"`
	// Timezone is the repo's dominant commit offset, used for calendar days.
	Timezone string   `json:"timezone"`
	Warnings []string `json:"warnings,omitempty"`
}

// Totals are repo-level counts.
type Totals struct {
	SurvivingLines int `json:"surviving_lines"`
	Commits        int `json:"commits"`
	Files          int `json:"files"`
	Contributors   int `json:"contributors"` // humans + bots
	Humans         int `json:"humans"`
	Bots           int `json:"bots"`
}

// NamedCount is a label with a surviving-line count.
type NamedCount struct {
	Name  string `json:"name"`
	Lines int    `json:"lines"`
}

// Contributor holds every per-author metric. Field comments give the formula.
type Contributor struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
	Bot   bool   `json:"bot"`

	// Commits is the number of non-merge commits authored (merges only when
	// --include-merges is set).
	Commits int `json:"commits"`
	// Added and Deleted sum numstat over history, filtered paths only.
	Added   int `json:"added"`
	Deleted int `json:"deleted"`
	// Net = Added - Deleted.
	Net int `json:"net"`
	// Surviving is the number of lines at the blamed revision that blame
	// attributes to this author.
	Surviving int `json:"surviving"`
	// Ownership = Surviving / total surviving lines.
	Ownership float64 `json:"ownership"`
	// SurvivalRate = Surviving / Added; 0 when Added is 0. Not meaningful with
	// --since, because Surviving ignores the window while Added does not.
	SurvivalRate float64 `json:"survival_rate"`
	// Churn is approximate: max(0, Added - Surviving), i.e. lines they wrote
	// that no longer exist, whether they or someone else removed them.
	Churn int `json:"churn_approx"`

	FilesTouched int `json:"files_touched"`
	// FilesOwned counts files where this author holds the largest share of
	// surviving lines (ties broken by identity ID).
	FilesOwned int `json:"files_owned"`

	FirstCommit time.Time `json:"first_commit,omitempty"`
	LastCommit  time.Time `json:"last_commit,omitempty"`

	// ActiveDays counts distinct calendar days (repo timezone) with a commit.
	ActiveDays    int `json:"active_days"`
	LongestStreak int `json:"longest_streak"`
	// CurrentStreak is the run of consecutive active days ending today or
	// yesterday; 0 if the last commit is older.
	CurrentStreak int `json:"current_streak"`

	// AvgCommitSize = (Added + Deleted) / Commits; MedianCommitSize is the
	// median of the per-commit (added + deleted).
	AvgCommitSize    float64 `json:"avg_commit_size"`
	MedianCommitSize float64 `json:"median_commit_size"`

	TopLanguages []NamedCount `json:"top_languages"`
	TopDirs      []NamedCount `json:"top_dirs"`

	// Heat[weekday][hour] counts commits in the author's own commit timezone.
	// weekday follows time.Weekday (Sunday = 0).
	Heat [7][24]int `json:"heat"`
	// WeeklyCommits is aligned to Report.Weeks.
	WeeklyCommits []int `json:"weekly_commits"`
}

// HourCounts sums Heat over weekdays.
func (c Contributor) HourCounts() [24]int {
	var out [24]int
	for _, row := range c.Heat {
		for h, n := range row {
			out[h] += n
		}
	}
	return out
}

// WeekdayCounts sums Heat over hours.
func (c Contributor) WeekdayCounts() [7]int {
	var out [7]int
	for d, row := range c.Heat {
		for _, n := range row {
			out[d] += n
		}
	}
	return out
}

// TopShare is the combined ownership of the N largest owners.
type TopShare struct {
	N     int     `json:"n"`
	Share float64 `json:"share"`
}

// Ownership summarizes how concentrated the code is.
type Ownership struct {
	// Gini is the Gini coefficient over per-contributor surviving lines
	// (0 = evenly shared, ->1 = one person owns everything).
	Gini float64 `json:"gini"`
	// BusFactor is the minimum number of contributors whose combined surviving
	// lines exceed 50% of the total.
	BusFactor int        `json:"bus_factor"`
	TopShares []TopShare `json:"top_shares"`
}

// OwnerShare is one owner's surviving lines within a file or directory.
type OwnerShare struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Lines int    `json:"lines"`
}

// FileStat is the per-file view at the blamed revision.
type FileStat struct {
	Path          string `json:"path"`
	Lines         int    `json:"lines"`
	TopOwnerID    string `json:"top_owner_id"`
	TopOwner      string `json:"top_owner"`
	TopOwnerLines int    `json:"top_owner_lines"`
	Authors       int    `json:"authors"`
	// LastModified is the newest author time among surviving lines.
	LastModified time.Time `json:"last_modified,omitempty"`
	SingleOwner  bool      `json:"single_owner"`
	Silo         bool      `json:"silo"`
}

// TopOwnerShare = TopOwnerLines / Lines.
func (f FileStat) TopOwnerShare() float64 {
	if f.Lines == 0 {
		return 0
	}
	return float64(f.TopOwnerLines) / float64(f.Lines)
}

// DirStat aggregates a directory and everything below it.
type DirStat struct {
	Path      string       `json:"path"`
	Lines     int          `json:"lines"`
	Files     int          `json:"files"`
	BusFactor int          `json:"bus_factor"`
	Owners    []OwnerShare `json:"owners"` // by lines, descending
}

// Silo is a file mostly owned by one person who has gone quiet.
type Silo struct {
	Path       string    `json:"path"`
	Owner      string    `json:"owner"`
	OwnerID    string    `json:"owner_id"`
	Share      float64   `json:"share"`
	Lines      int       `json:"lines"`
	LastCommit time.Time `json:"owner_last_commit,omitempty"`
}

// Week is one Monday-based week of repo activity.
type Week struct {
	Start   time.Time `json:"start"`
	Commits int       `json:"commits"`
	Added   int       `json:"added"`
	Deleted int       `json:"deleted"`
}

// Month counts contributors by whether this was their first active month.
type Month struct {
	Month     string `json:"month"` // "2006-01"
	New       int    `json:"new"`
	Returning int    `json:"returning"`
}
