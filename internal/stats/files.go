package stats

import (
	"path"
	"sort"
	"strings"
	"time"
)

// fileOwner is one author's lines in a single file, post identity merge.
type fileOwner struct {
	id    string
	lines int
}

// fileAgg is the blame result of one file after identity resolution.
type fileAgg struct {
	path   string
	owners []fileOwner // lines desc, then id
	total  int
	last   int64
}

// accumulateBlame resolves blame authors to identities and records surviving
// lines, languages and directories per contributor. File and directory stats
// are derived afterwards in finalizeFiles from b.files.
func (b *builder) accumulateBlame(r *Report) {
	paths := make([]string, 0, len(b.in.Blame))
	for p := range b.in.Blame {
		if b.filter.Allow(p) {
			paths = append(paths, p)
		}
	}
	sort.Strings(paths)

	for _, p := range paths {
		fb := b.in.Blame[p]
		byID := map[string]int{}
		for au, n := range fb.Lines {
			byID[b.res.Resolve(au.Name, au.Email).ID] += n
		}
		agg := fileAgg{path: p, total: fb.Total, last: fb.LastTime}
		for id, n := range byID {
			agg.owners = append(agg.owners, fileOwner{id, n})
		}
		sort.Slice(agg.owners, func(i, j int) bool {
			if agg.owners[i].lines != agg.owners[j].lines {
				return agg.owners[i].lines > agg.owners[j].lines
			}
			return agg.owners[i].id < agg.owners[j].id
		})
		b.files = append(b.files, agg)

		lang, top := languageOf(p), topDir(p)
		for id, n := range byID {
			a := b.accs[id]
			a.c.Surviving += n
			a.langs[lang] += n
			a.dirs[top] += n
		}
		if len(agg.owners) > 0 {
			b.accs[agg.owners[0].id].c.FilesOwned++
		}
		r.Totals.SurvivingLines += fb.Total
	}
}

// topDir is the first path segment, or "." for files in the root.
func topDir(p string) string {
	if i := strings.IndexByte(p, '/'); i >= 0 {
		return p[:i]
	}
	return "."
}

// dirsOf lists the directories containing p, root first: "a/b/c.go" yields
// ".", "a", "a/b".
func dirsOf(p string) []string {
	var rev []string
	for d := path.Dir(p); d != "." && d != "/"; d = path.Dir(d) {
		rev = append(rev, d)
	}
	out := []string{"."}
	for i := len(rev) - 1; i >= 0; i-- {
		out = append(out, rev[i])
	}
	return out
}

// finalizeFiles fills Files, Dirs and Silos. Silos are files where one person
// holds more than 80% of the lines and has not committed for six months.
func (b *builder) finalizeFiles(r *Report) {
	names := map[string]string{}
	last := map[string]time.Time{}
	for id, a := range b.accs {
		names[id], last[id] = a.c.Name, a.c.LastCommit
	}
	type dirAgg struct {
		files  int
		owners map[string]int
	}
	dirs := map[string]*dirAgg{}

	for _, f := range b.files {
		fs := FileStat{Path: f.path, Lines: f.total, Authors: len(f.owners)}
		if f.last > 0 {
			fs.LastModified = time.Unix(f.last, 0).UTC()
		}
		if len(f.owners) > 0 {
			top := f.owners[0]
			fs.TopOwnerID, fs.TopOwner, fs.TopOwnerLines = top.id, names[top.id], top.lines
			fs.SingleOwner = len(f.owners) == 1
			// top/total > 0.8 in integers.
			if f.total > 0 && top.lines*5 > f.total*4 && silent(last[top.id], b.now) {
				fs.Silo = true
				r.Silos = append(r.Silos, Silo{
					Path: f.path, Owner: fs.TopOwner, OwnerID: top.id,
					Share: fs.TopOwnerShare(), Lines: f.total, LastCommit: last[top.id],
				})
			}
		}
		r.Files = append(r.Files, fs)

		for _, d := range dirsOf(f.path) {
			da := dirs[d]
			if da == nil {
				da = &dirAgg{owners: map[string]int{}}
				dirs[d] = da
			}
			da.files++
			for _, o := range f.owners {
				da.owners[o.id] += o.lines
			}
		}
	}
	r.Totals.Files = len(r.Files)

	sort.Slice(r.Files, func(i, j int) bool {
		if r.Files[i].Lines != r.Files[j].Lines {
			return r.Files[i].Lines > r.Files[j].Lines
		}
		return r.Files[i].Path < r.Files[j].Path
	})
	sort.Slice(r.Silos, func(i, j int) bool {
		if r.Silos[i].Lines != r.Silos[j].Lines {
			return r.Silos[i].Lines > r.Silos[j].Lines
		}
		return r.Silos[i].Path < r.Silos[j].Path
	})

	for p, da := range dirs {
		ds := DirStat{Path: p, Files: da.files}
		var lines []int
		for id, n := range da.owners {
			ds.Owners = append(ds.Owners, OwnerShare{ID: id, Name: names[id], Lines: n})
			ds.Lines += n
			lines = append(lines, n)
		}
		sort.Slice(ds.Owners, func(i, j int) bool {
			if ds.Owners[i].Lines != ds.Owners[j].Lines {
				return ds.Owners[i].Lines > ds.Owners[j].Lines
			}
			return ds.Owners[i].ID < ds.Owners[j].ID
		})
		ds.BusFactor = busFactor(lines, ds.Lines)
		r.Dirs = append(r.Dirs, ds)
	}
	sort.Slice(r.Dirs, func(i, j int) bool { return r.Dirs[i].Path < r.Dirs[j].Path })
}
