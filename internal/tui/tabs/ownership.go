package tabs

import (
	"fmt"
	"path"
	"sort"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/adnair2024/blamey/internal/stats"
	"github.com/adnair2024/blamey/internal/tui/components"
	"github.com/adnair2024/blamey/internal/tui/keys"
	"github.com/adnair2024/blamey/internal/tui/styles"
)

// entry is one row of the ownership tree: a directory (enterable) or a file.
type entry struct {
	name      string
	path      string
	dir       bool
	lines     int
	busFactor int
	owners    []stats.OwnerShare
}

// Ownership is a directory tree with a stacked ownership bar per entry.
type Ownership struct {
	base
	cwd     string // "." is the repository root
	cursor  int
	history []int // cursor to restore for each parent level
	entries []entry
}

func NewOwnership(st *styles.Styles, km keys.Map) *Ownership {
	return &Ownership{base: base{st: st, km: km}, cwd: "."}
}

func (o *Ownership) Name() string { return "Ownership" }

func (o *Ownership) SetData(d Data) {
	o.data = d
	o.cwd, o.cursor, o.history = ".", 0, nil
	o.rebuild()
}

func (o *Ownership) Update(msg tea.Msg) (Tab, tea.Cmd) {
	kp, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return o, nil
	}
	switch {
	case key.Matches(kp, o.km.Up):
		o.cursor = max(o.cursor-1, 0)
	case key.Matches(kp, o.km.Down):
		o.cursor = min(o.cursor+1, max(len(o.entries)-1, 0))
	case key.Matches(kp, o.km.PageUp):
		o.cursor = max(o.cursor-o.rowsVisible(), 0)
	case key.Matches(kp, o.km.PageDown):
		o.cursor = min(o.cursor+o.rowsVisible(), max(len(o.entries)-1, 0))
	case key.Matches(kp, o.km.Enter):
		o.drillIn()
	case key.Matches(kp, o.km.Back, o.km.BackDir):
		o.drillOut()
	}
	return o, nil
}

func (o *Ownership) drillIn() {
	if o.cursor >= len(o.entries) || !o.entries[o.cursor].dir {
		return
	}
	o.history = append(o.history, o.cursor)
	o.cwd, o.cursor = o.entries[o.cursor].path, 0
	o.rebuild()
}

func (o *Ownership) drillOut() {
	if o.cwd == "." {
		return
	}
	o.cwd = path.Dir(o.cwd)
	o.cursor = 0
	if n := len(o.history); n > 0 {
		o.cursor, o.history = o.history[n-1], o.history[:n-1]
	}
	o.rebuild()
}

// rebuild lists the children of cwd: directories first, then files, each by
// descending lines.
func (o *Ownership) rebuild() {
	o.entries = nil
	r := o.report()
	if r == nil {
		return
	}
	for _, d := range r.Dirs {
		if d.Path != "." && path.Dir(d.Path) == o.cwd {
			o.entries = append(o.entries, entry{
				name: path.Base(d.Path), path: d.Path, dir: true,
				lines: d.Lines, busFactor: d.BusFactor, owners: d.Owners,
			})
		}
	}
	sortEntries(o.entries)
	var files []entry
	for _, f := range r.Files {
		if path.Dir(f.Path) == o.cwd {
			files = append(files, entry{
				name: path.Base(f.Path), path: f.Path, lines: f.Lines,
				owners: []stats.OwnerShare{{ID: f.TopOwnerID, Name: f.TopOwner, Lines: f.TopOwnerLines}},
			})
		}
	}
	sortEntries(files)
	o.entries = append(o.entries, files...)
	o.cursor = min(o.cursor, max(len(o.entries)-1, 0))
}

func sortEntries(es []entry) {
	sort.SliceStable(es, func(i, j int) bool {
		if es[i].lines != es[j].lines {
			return es[i].lines > es[j].lines
		}
		return es[i].name < es[j].name
	})
}

func (o *Ownership) dirStat() (stats.DirStat, bool) {
	for _, d := range o.report().Dirs {
		if d.Path == o.cwd {
			return d, true
		}
	}
	return stats.DirStat{}, false
}

// rowsVisible is the number of entry rows that fit under the header.
func (o *Ownership) rowsVisible() int { return max(o.h-4, 1) }

func (o *Ownership) View() string {
	if o.report() == nil {
		return ""
	}
	d, ok := o.dirStat()
	if !ok {
		return o.empty("No surviving lines to show.")
	}
	crumb := "Ownership › " + strings.ReplaceAll(strings.TrimPrefix(o.cwd, "."), "/", " › ")
	crumb = strings.TrimSuffix(strings.TrimSpace(crumb), "›")
	head := o.section(strings.TrimSpace(crumb)) + o.subtle(fmt.Sprintf("   %s lines · %s files · bus factor %d",
		num(d.Lines), num(d.Files), d.BusFactor))
	lines := []string{head, o.ownerBar(d.Owners, d.Lines, o.w), ""}

	visible := o.rowsVisible()
	start := 0
	if o.cursor >= visible {
		start = o.cursor - visible + 1
	}
	if len(o.entries) == 0 {
		lines = append(lines, o.subtle("Nothing here."))
	}
	nameW := min(max(o.w/4, 12), 32)
	barW := max(o.w-nameW-49, 6)
	for i := start; i < len(o.entries) && i < start+visible; i++ {
		lines = append(lines, o.row(o.entries[i], i == o.cursor, nameW, barW))
	}
	foot := "enter open directory · esc/backspace up a level"
	return clip(strings.Join(lines, "\n")+"\n"+o.subtle(foot), o.h)
}

func (o *Ownership) row(e entry, selected bool, nameW, barW int) string {
	marker, name := "  ", e.name
	if e.dir {
		name += "/"
	}
	name = fit(name, nameW)
	if selected {
		marker, name = o.st.Accent.Render("▌ "), o.st.Bold.Render(name)
	} else if e.dir {
		name = o.st.Accent.Render(name)
	}
	top := "—"
	if len(e.owners) > 0 && e.lines > 0 {
		top = fmt.Sprintf("%s %s", o.st.Contributor(e.owners[0].ID).Render(truncName(e.owners[0].Name, 16)),
			pct(float64(e.owners[0].Lines)/float64(e.lines)))
	}
	bf := ""
	if e.dir {
		bf = fmt.Sprintf("bf %d", e.busFactor)
	}
	return fmt.Sprintf("%s%s %s %s  %s  %s", marker, name, o.ownerBar(e.owners, e.lines, barW),
		fitRight(num(e.lines), 8), fit(top, 26), o.subtle(bf))
}

func truncName(s string, w int) string { return strings.TrimSpace(fit(s, w)) }

// ownerBar stacks the five largest owners; the rest (and any unattributed
// remainder) is drawn in the neutral track color.
func (o *Ownership) ownerBar(owners []stats.OwnerShare, total, width int) string {
	if total <= 0 {
		return strings.Repeat(" ", width)
	}
	var segs []components.Segment
	used := 0
	for i, ow := range owners {
		if i == 5 {
			break
		}
		segs = append(segs, components.Segment{Frac: float64(ow.Lines) / float64(total), Style: o.st.Contributor(ow.ID)})
		used += ow.Lines
	}
	if rest := total - used; rest > 0 {
		segs = append(segs, components.Segment{Frac: float64(rest) / float64(total), Style: o.st.Other()})
	}
	return components.StackedBar(segs, width)
}
