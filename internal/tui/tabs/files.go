package tabs

import (
	"strings"

	"github.com/charmbracelet/x/ansi"

	tea "charm.land/bubbletea/v2"

	"github.com/adnair2024/blamey/internal/stats"
	"github.com/adnair2024/blamey/internal/tui/keys"
	"github.com/adnair2024/blamey/internal/tui/styles"
)

// Files lists every blamed file with its top owner. Knowledge silos are marked
// "!" and single-owner files "*" (and colored), so the marks survive NO_COLOR.
type Files struct {
	base
	list *listTable[stats.FileStat]
}

func NewFiles(st *styles.Styles, km keys.Map) *Files {
	f := &Files{base: base{st: st, km: km}}
	f.list = newListTable[stats.FileStat](st, km, []column{
		{title: "File", flex: true, text: true},
		{title: "Lines", width: 7},
		{title: "Top owner", width: 16, text: true},
		{title: "Owner%", width: 7},
		{title: "Authors", width: 7},
		{title: "Modified", width: 13},
	}, 1)
	f.list.cells = func(p stats.FileStat, w []int) []string {
		return []string{
			f.pathCell(p, w[0]), num(p.Lines),
			f.st.Contributor(p.TopOwnerID).Render(p.TopOwner),
			pct(p.TopOwnerShare()), num(p.Authors), f.ago(p.LastModified),
		}
	}
	f.list.less = lessFile
	f.list.match = func(p stats.FileStat, q string) bool {
		return strings.Contains(strings.ToLower(p.Path+" "+p.TopOwner), q)
	}
	return f
}

// pathCell keeps the end of a long path (the file name matters most) and
// prefixes the silo / single-owner mark.
func (f *Files) pathCell(p stats.FileStat, w int) string {
	short := p.Path
	if over := ansi.StringWidth(short) - (w - 2); over > 0 {
		short = ansi.TruncateLeft(short, over+1, "…")
	}
	switch {
	case p.Silo:
		return f.st.Silo.Render("! " + short)
	case p.SingleOwner:
		return f.st.SingleOwner.Render("* " + short)
	}
	return "  " + short
}

func lessFile(a, b stats.FileStat, col int) bool {
	switch col {
	case 0:
		return a.Path < b.Path
	case 1:
		return a.Lines < b.Lines
	case 2:
		return strings.ToLower(a.TopOwner) < strings.ToLower(b.TopOwner)
	case 3:
		return a.TopOwnerShare() < b.TopOwnerShare()
	case 4:
		return a.Authors < b.Authors
	}
	return a.LastModified.Before(b.LastModified)
}

func (f *Files) Name() string    { return "Files" }
func (f *Files) Capturing() bool { return f.list.capturing() }

func (f *Files) SetStyles(s *styles.Styles) {
	f.st = s
	f.list.setStyles(s)
}

func (f *Files) SetData(d Data) {
	f.data = d
	if d.Report != nil {
		f.list.setItems(d.Report.Files)
	}
}

func (f *Files) Resize(w, h int) {
	f.w, f.h = w, h
	f.list.resize(w, max(h-1, 3))
}

func (f *Files) Update(msg tea.Msg) (Tab, tea.Cmd) {
	_, cmd := f.list.update(msg)
	return f, cmd
}

func (f *Files) View() string {
	if f.report() == nil {
		return ""
	}
	shown, total := f.list.count()
	legend := f.st.Silo.Render("! silo") + f.subtle(" (>80% one owner, silent 6+ months)  ") +
		f.st.SingleOwner.Render("* single owner") + f.subtle("  ·  ") +
		f.subtle(num(shown)+" of "+num(total)+" files")
	return f.list.view() + "\n" + legend
}
