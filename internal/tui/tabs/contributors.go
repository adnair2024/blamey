package tabs

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"

	"github.com/adnair2024/blamey/internal/stats"
	"github.com/adnair2024/blamey/internal/tui/keys"
	"github.com/adnair2024/blamey/internal/tui/styles"
)

// Contributors lists everyone in a sortable table; enter opens a detail view.
type Contributors struct {
	base
	list *listTable[stats.Contributor]

	detail   bool
	detailID string
	vp       viewport.Model
}

func NewContributors(st *styles.Styles, km keys.Map) *Contributors {
	c := &Contributors{base: base{st: st, km: km}}
	c.list = newListTable[stats.Contributor](st, km, []column{
		{title: "Name", flex: true, text: true},
		{title: "Commits", width: 8},
		{title: "+ / −", width: 15},
		{title: "Surviving", width: 12},
		{title: "Own%", width: 7},
		{title: "Last active", width: 14},
	}, 3)
	c.list.cells = func(p stats.Contributor, _ []int) []string {
		return []string{
			c.nameCell(p), num(p.Commits),
			fmt.Sprintf("+%s/−%s", num(p.Added), num(p.Deleted)),
			num(p.Surviving), pct(p.Ownership), c.ago(p.LastCommit),
		}
	}
	c.list.less = lessContributor
	c.list.match = func(p stats.Contributor, q string) bool {
		return strings.Contains(strings.ToLower(p.Name+" "+p.Email), q)
	}
	c.vp = viewport.New()
	return c
}

func lessContributor(a, b stats.Contributor, col int) bool {
	switch col {
	case 0:
		return strings.ToLower(a.Name) < strings.ToLower(b.Name)
	case 1:
		return a.Commits < b.Commits
	case 2:
		return a.Added < b.Added
	case 3:
		return a.Surviving < b.Surviving
	case 4:
		return a.Ownership < b.Ownership
	}
	return a.LastCommit.Before(b.LastCommit)
}

func (c *Contributors) Name() string    { return "Contributors" }
func (c *Contributors) Capturing() bool { return !c.detail && c.list.capturing() }

func (c *Contributors) SetStyles(s *styles.Styles) {
	c.st = s
	c.list.setStyles(s)
	c.renderDetail()
}

func (c *Contributors) SetData(d Data) {
	c.data = d
	c.list.setItems(d.people())
	if c.detail && !c.hasDetailTarget() {
		c.detail = false
	}
	c.renderDetail()
}

func (c *Contributors) hasDetailTarget() bool {
	for _, p := range c.data.people() {
		if p.ID == c.detailID {
			return true
		}
	}
	return false
}

func (c *Contributors) Resize(w, h int) {
	c.w, c.h = w, h
	c.list.resize(w, max(h-1, 3))
	c.vp.SetWidth(w)
	c.vp.SetHeight(max(h-1, 3))
	c.renderDetail()
}

func (c *Contributors) Update(msg tea.Msg) (Tab, tea.Cmd) {
	kp, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return c, nil
	}
	if c.detail {
		switch {
		case key.Matches(kp, c.km.Back, c.km.BackDir):
			c.detail = false
		case key.Matches(kp, c.km.Up):
			c.vp.ScrollUp(1)
		case key.Matches(kp, c.km.Down):
			c.vp.ScrollDown(1)
		case key.Matches(kp, c.km.PageUp):
			c.vp.PageUp()
		case key.Matches(kp, c.km.PageDown):
			c.vp.PageDown()
		}
		return c, nil
	}
	if consumed, cmd := c.list.update(msg); consumed {
		return c, cmd
	}
	if key.Matches(kp, c.km.Enter) {
		if p, ok := c.list.selected(); ok {
			c.detail, c.detailID = true, p.ID
			c.renderDetail()
			c.vp.GotoTop()
		}
	}
	return c, nil
}

func (c *Contributors) View() string {
	if c.report() == nil {
		return ""
	}
	if c.detail {
		return c.vp.View() + "\n" + c.subtle("esc back · ↑/↓ scroll")
	}
	shown, total := c.list.count()
	return c.list.view() + "\n" + c.subtle(fmt.Sprintf("%d of %d contributors · ownership is at %s", shown, total, c.report().Meta.Rev))
}

func (c *Contributors) renderDetail() {
	if !c.detail || c.report() == nil {
		return
	}
	for _, p := range c.data.people() {
		if p.ID == c.detailID {
			c.vp.SetContent(c.detailView(p))
			return
		}
	}
}
