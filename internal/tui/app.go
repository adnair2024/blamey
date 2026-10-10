// Package tui is the interactive Bubble Tea front end. It only renders a
// stats.Report; all git work arrives through a LoadFunc.
package tui

import (
	"context"
	"strings"
	"time"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/progress"
	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"

	"github.com/adnair2024/blamey/internal/stats"
	"github.com/adnair2024/blamey/internal/tui/keys"
	"github.com/adnair2024/blamey/internal/tui/styles"
	"github.com/adnair2024/blamey/internal/tui/tabs"
)

const (
	minWidth  = 60
	minHeight = 16
	// chrome is the rows the root draws around the active tab: two header
	// rows, a spacer and one footer row.
	chrome = 4
)

type state int

const (
	stateLoading state = iota
	stateReady
	stateFailed
)

// Options configures the root model.
type Options struct {
	ShowBots bool
	// Now fixes the clock used for relative times; zero means time.Now().
	Now time.Time
	// Light starts with the light theme instead of the dark default, until
	// the terminal reports its real background.
	Light bool
}

// Model is the root model: it owns the window size, routes messages to the
// active tab and draws the chrome around it.
type Model struct {
	load   LoadFunc
	ldr    *loader
	cancel context.CancelFunc
	ctx    context.Context

	state state
	err   error
	prog  Progress
	spin  spinner.Model
	bar   progress.Model

	report   stats.Report
	tabs     []tabs.Tab
	active   int
	showBots bool
	showHelp bool
	now      time.Time

	km   keys.Map
	st   *styles.Styles
	help help.Model
	w, h int
}

// New creates the root model. The load starts when the program runs Init.
func New(ctx context.Context, load LoadFunc, opts Options) *Model {
	ctx, cancel := context.WithCancel(ctx)
	km := keys.Default()
	st := styles.New(!opts.Light)
	m := &Model{
		load: load, ldr: newLoader(), ctx: ctx, cancel: cancel,
		prog:     Progress{Phase: "starting"},
		spin:     spinner.New(spinner.WithSpinner(spinner.Dot), spinner.WithStyle(st.Accent)),
		bar:      progress.New(progress.WithWidth(40)),
		showBots: opts.ShowBots, now: opts.Now,
		km: km, st: st, help: help.New(),
	}
	m.help.Styles = st.Help
	m.tabs = []tabs.Tab{
		tabs.NewOverview(st, km), tabs.NewContributors(st, km), tabs.NewOwnership(st, km),
		tabs.NewActivity(st, km), tabs.NewFiles(st, km),
	}
	return m
}

func (m *Model) Init() tea.Cmd {
	return tea.Batch(m.spin.Tick, m.ldr.start(m.ctx, m.load), tea.RequestBackgroundColor)
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.w, m.h = msg.Width, msg.Height
		m.layout()
		return m, nil
	case tea.BackgroundColorMsg:
		m.setTheme(msg.IsDark())
		return m, nil
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spin, cmd = m.spin.Update(msg)
		return m, cmd
	case progressMsg:
		m.prog = Progress(msg)
		return m, m.ldr.wait()
	case reportReadyMsg:
		m.setReport(msg.report)
		return m, nil
	case errMsg:
		m.state, m.err = stateFailed, msg.err
		return m, nil
	case tea.KeyPressMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

func (m *Model) handleKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if key.Matches(msg, m.km.ForceQuit) {
		return m, m.quit()
	}
	if m.state != stateReady {
		if key.Matches(msg, m.km.Quit, m.km.Back) {
			return m, m.quit()
		}
		return m, nil
	}
	tab := m.tabs[m.active]
	if tab.Capturing() {
		return m, m.forward(msg)
	}
	switch {
	case key.Matches(msg, m.km.Quit):
		return m, m.quit()
	case key.Matches(msg, m.km.Help):
		m.showHelp = !m.showHelp
	case m.showHelp && key.Matches(msg, m.km.Back):
		m.showHelp = false
	case key.Matches(msg, m.km.NextTab):
		m.switchTab((m.active + 1) % len(m.tabs))
	case key.Matches(msg, m.km.PrevTab):
		m.switchTab((m.active + len(m.tabs) - 1) % len(m.tabs))
	case key.Matches(msg, m.km.Tabs):
		if i := int(msg.String()[0] - '1'); i >= 0 && i < len(m.tabs) {
			m.switchTab(i)
		}
	case key.Matches(msg, m.km.Bots):
		m.showBots = !m.showBots
		m.pushData()
	default:
		if !m.showHelp {
			return m, m.forward(msg)
		}
	}
	return m, nil
}

func (m *Model) forward(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd
	m.tabs[m.active], cmd = m.tabs[m.active].Update(msg)
	return cmd
}

func (m *Model) switchTab(i int) {
	m.active, m.showHelp = i, false
}

// quit cancels the load so git child processes die, then exits.
func (m *Model) quit() tea.Cmd {
	m.cancel()
	return tea.Quit
}

func (m *Model) setTheme(dark bool) {
	m.st = styles.New(dark)
	m.help.Styles = m.st.Help
	m.spin.Style = m.st.Accent
	for _, t := range m.tabs {
		t.SetStyles(m.st)
	}
	m.pushData()
}

func (m *Model) setReport(r stats.Report) {
	m.report = r
	m.state = stateReady
	m.pushData()
	m.layout()
}

func (m *Model) pushData() {
	if m.state != stateReady {
		return
	}
	now := m.now
	if now.IsZero() {
		now = time.Now()
	}
	d := tabs.Data{Report: &m.report, ShowBots: m.showBots, Now: now}
	for _, t := range m.tabs {
		t.SetData(d)
	}
	m.layout()
}

func (m *Model) layout() {
	bodyH := max(m.h-chrome, 1)
	for _, t := range m.tabs {
		t.Resize(m.w, bodyH)
	}
	m.help.SetWidth(m.w)
	m.bar.SetWidth(max(min(m.w-10, 50), 10))
}

func (m *Model) View() tea.View {
	v := tea.NewView(m.render())
	v.AltScreen = true
	v.WindowTitle = "blamey"
	return v
}

func (m *Model) render() string {
	if m.w < minWidth || m.h < minHeight {
		return m.tooSmall()
	}
	switch m.state {
	case stateLoading:
		return m.loadingView()
	case stateFailed:
		return m.errorView()
	}
	body := m.tabs[m.active].View()
	if m.showHelp {
		body = m.helpView()
	}
	// Pin the body to its allotted height so the footer never jumps.
	body = m.st.Block(m.w, max(m.h-chrome, 1)).Render(body)
	return strings.Join([]string{m.header(), "", body, m.footer()}, "\n")
}
