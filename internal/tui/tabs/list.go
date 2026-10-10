package tabs

import (
	"sort"
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/table"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	"github.com/adnair2024/blamey/internal/tui/keys"
	"github.com/adnair2024/blamey/internal/tui/styles"
)

// column describes one table column. flex columns share the width left over by
// the fixed ones; text columns sort ascending first, numeric ones descending.
type column struct {
	title string
	width int
	flex  bool
	text  bool
}

// listTable is a sortable, filterable table over items of type T, built on the
// Bubbles table. Sorting, filtering and key handling live here so the
// contributors and files tabs stay small.
type listTable[T any] struct {
	st  *styles.Styles
	km  keys.Map
	tbl table.Model

	cols  []column
	items []T
	shown []T

	cells func(it T, widths []int) []string
	less  func(a, b T, col int) bool
	match func(T, string) bool

	sortCol int
	desc    bool

	filter textinput.Model
	typing bool

	w, h int
}

// newListTable sorts by cols[sortCol], descending, until the user changes it.
func newListTable[T any](st *styles.Styles, km keys.Map, cols []column, sortCol int) *listTable[T] {
	l := &listTable[T]{st: st, km: km, cols: cols, sortCol: sortCol, desc: true}
	l.tbl = table.New(table.WithFocused(true), table.WithStyles(st.Table))
	l.tbl.KeyMap = tableKeys(km)
	l.filter = textinput.New()
	l.filter.Prompt = "/ "
	l.filter.Placeholder = "filter"
	return l
}

// tableKeys maps the table's movement onto our key.Map so the keys stay
// defined in one place.
func tableKeys(km keys.Map) table.KeyMap {
	t := table.DefaultKeyMap()
	t.LineUp, t.LineDown = km.Up, km.Down
	t.PageUp, t.PageDown = km.PageUp, km.PageDown
	// Half pages, top and bottom are intentionally unbound.
	t.HalfPageUp, t.HalfPageDown = key.NewBinding(key.WithDisabled()), key.NewBinding(key.WithDisabled())
	t.GotoTop, t.GotoBottom = key.NewBinding(key.WithDisabled()), key.NewBinding(key.WithDisabled())
	return t
}

func (l *listTable[T]) setStyles(st *styles.Styles) {
	l.st = st
	l.tbl.SetStyles(st.Table)
	l.refresh()
}

func (l *listTable[T]) setItems(items []T) {
	l.items = items
	l.refresh()
}

func (l *listTable[T]) resize(w, h int) {
	l.w, l.h = w, h
	l.refresh()
}

func (l *listTable[T]) capturing() bool { return l.typing }

// selected returns the item under the cursor.
func (l *listTable[T]) selected() (T, bool) {
	var zero T
	i := l.tbl.Cursor()
	if i < 0 || i >= len(l.shown) {
		return zero, false
	}
	return l.shown[i], true
}

// update handles filter and sort keys, then movement. It reports whether the
// key was consumed so the owning tab can act on the rest (enter, esc).
func (l *listTable[T]) update(msg tea.Msg) (consumed bool, cmd tea.Cmd) {
	kp, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return false, nil
	}
	if l.typing {
		switch {
		case key.Matches(kp, l.km.Back):
			l.filter.Reset()
			l.stopTyping()
		case key.Matches(kp, l.km.Enter):
			l.stopTyping()
		default:
			l.filter, cmd = l.filter.Update(msg)
		}
		l.refresh()
		return true, cmd
	}
	switch {
	case key.Matches(kp, l.km.Filter):
		l.typing = true
		l.refresh()
		return true, l.filter.Focus()
	case key.Matches(kp, l.km.Sort):
		l.sortCol = (l.sortCol + 1) % len(l.cols)
		l.desc = !l.cols[l.sortCol].text
		l.refresh()
		return true, nil
	case key.Matches(kp, l.km.SortReverse):
		l.desc = !l.desc
		l.refresh()
		return true, nil
	case key.Matches(kp, l.km.Back) && l.filter.Value() != "":
		l.filter.Reset()
		l.refresh()
		return true, nil
	case key.Matches(kp, l.km.Up, l.km.Down, l.km.PageUp, l.km.PageDown):
		l.tbl, cmd = l.tbl.Update(msg)
		return true, cmd
	}
	return false, nil
}

func (l *listTable[T]) stopTyping() {
	l.typing = false
	l.filter.Blur()
}

// refresh re-filters, re-sorts and rebuilds the visible table.
func (l *listTable[T]) refresh() {
	q := strings.ToLower(strings.TrimSpace(l.filter.Value()))
	l.shown = l.shown[:0:0]
	for _, it := range l.items {
		if q == "" || l.match(it, q) {
			l.shown = append(l.shown, it)
		}
	}
	sort.SliceStable(l.shown, func(i, j int) bool {
		a, b := l.shown[i], l.shown[j]
		if l.desc {
			return l.less(b, a, l.sortCol)
		}
		return l.less(a, b, l.sortCol)
	})

	widths := l.columnWidths()
	cols := make([]table.Column, len(l.cols))
	for i, c := range l.cols {
		title := c.title
		if i == l.sortCol {
			title += map[bool]string{true: " ▼", false: " ▲"}[l.desc]
		}
		cols[i] = table.Column{Title: title, Width: widths[i]}
	}
	rows := make([]table.Row, len(l.shown))
	for i, it := range l.shown {
		rows[i] = l.cells(it, widths)
	}
	cur := l.tbl.Cursor()
	l.tbl.SetColumns(cols)
	l.tbl.SetRows(rows)
	l.tbl.SetCursor(max(min(cur, len(rows)-1), 0))
	l.tbl.SetWidth(l.w)
	l.tbl.SetHeight(max(l.h-l.filterLines(), 2))
}

func (l *listTable[T]) filterLines() int {
	if l.typing || l.filter.Value() != "" {
		return 1
	}
	return 0
}

// columnWidths distributes the width: fixed columns keep theirs (minus
// padding), flex columns split the remainder, never below 8.
func (l *listTable[T]) columnWidths() []int {
	const pad = 2 // the table adds one cell of padding each side
	fixed, flex := 0, 0
	for _, c := range l.cols {
		fixed += pad
		if c.flex {
			flex++
		} else {
			fixed += c.width
		}
	}
	out := make([]int, len(l.cols))
	left := max(l.w-fixed, 8*max(flex, 1))
	for i, c := range l.cols {
		if c.flex {
			out[i] = left / flex
		} else {
			out[i] = c.width
		}
	}
	return out
}

func (l *listTable[T]) view() string {
	var b strings.Builder
	if l.filterLines() > 0 {
		l.filter.SetWidth(max(l.w-4, 4))
		b.WriteString(l.filter.View())
		b.WriteString("\n")
	}
	b.WriteString(l.tbl.View())
	return b.String()
}

// position summarises the cursor for a status line, e.g. "3/42".
func (l *listTable[T]) count() (shown, total int) { return len(l.shown), len(l.items) }
