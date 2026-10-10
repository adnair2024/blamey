package tui

import (
	"context"
	"errors"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/adnair2024/blamey/internal/stats"
)

var update = flag.Bool("update", false, "rewrite golden files")

func neverLoad(ctx context.Context, _ func(Progress)) (stats.Report, error) {
	<-ctx.Done()
	return stats.Report{}, ctx.Err()
}

func newSized(t *testing.T, w, h int) *Model {
	t.Helper()
	m := New(context.Background(), neverLoad, Options{Now: fixedNow})
	t.Cleanup(m.cancel)
	m.Update(tea.WindowSizeMsg{Width: w, Height: h})
	return m
}

func newReady(t *testing.T, w, h int) *Model {
	t.Helper()
	m := newSized(t, w, h)
	m.Update(reportReadyMsg{fixtureReport()})
	return m
}

func keyMsg(s string) tea.KeyPressMsg {
	switch s {
	case "tab":
		return tea.KeyPressMsg{Code: tea.KeyTab}
	case "shift+tab":
		return tea.KeyPressMsg{Code: tea.KeyTab, Mod: tea.ModShift}
	case "enter":
		return tea.KeyPressMsg{Code: tea.KeyEnter}
	case "esc":
		return tea.KeyPressMsg{Code: tea.KeyEscape}
	case "backspace":
		return tea.KeyPressMsg{Code: tea.KeyBackspace}
	case "down":
		return tea.KeyPressMsg{Code: tea.KeyDown}
	case "up":
		return tea.KeyPressMsg{Code: tea.KeyUp}
	case "ctrl+c":
		return tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl}
	}
	r := []rune(s)[0]
	return tea.KeyPressMsg{Code: r, Text: s}
}

func press(m *Model, keys ...string) tea.Cmd {
	var cmd tea.Cmd
	for _, k := range keys {
		_, cmd = m.Update(keyMsg(k))
	}
	return cmd
}

// plain strips colors and trailing spaces so goldens stay stable across
// terminals and themes.
func plain(m *Model) string {
	lines := strings.Split(ansi.Strip(m.View().Content), "\n")
	for i, l := range lines {
		lines[i] = strings.TrimRight(l, " ")
	}
	return strings.Join(lines, "\n") + "\n"
}

func golden(t *testing.T, name, got string) {
	t.Helper()
	path := filepath.Join("testdata", name+".golden")
	if *update {
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("missing golden %s (run with -update): %v", path, err)
	}
	if string(want) != got {
		t.Errorf("%s differs from golden.\n--- want\n%s\n--- got\n%s", name, want, got)
	}
}

var sizes = []struct {
	name string
	w, h int
}{{"80x24", 80, 24}, {"200x50", 200, 50}}

func TestGoldenTabs(t *testing.T) {
	names := []string{"overview", "contributors", "ownership", "activity", "files"}
	for _, sz := range sizes {
		for i, n := range names {
			t.Run(n+"_"+sz.name, func(t *testing.T) {
				m := newReady(t, sz.w, sz.h)
				press(m, string(rune('1'+i)))
				golden(t, n+"_"+sz.name, plain(m))
			})
		}
	}
}

func TestGoldenScreens(t *testing.T) {
	t.Run("loading", func(t *testing.T) {
		m := newSized(t, 80, 24)
		m.Update(progressMsg{Phase: "blaming", Done: 412, Total: 1830})
		golden(t, "loading_80x24", plain(m))
	})
	t.Run("help", func(t *testing.T) {
		m := newReady(t, 100, 40)
		press(m, "?")
		golden(t, "help_100x40", plain(m))
	})
	t.Run("detail", func(t *testing.T) {
		m := newReady(t, 120, 60)
		press(m, "2", "enter")
		golden(t, "detail_120x60", plain(m))
	})
	t.Run("detail small", func(t *testing.T) {
		m := newReady(t, 80, 24)
		press(m, "2", "enter")
		golden(t, "detail_80x24", plain(m))
	})
	t.Run("too small", func(t *testing.T) {
		golden(t, "toosmall", plain(newReady(t, 40, 10)))
	})
}

// Every tab must fill exactly the window, at any size, with no line wider than
// the terminal.
func TestLayoutInvariants(t *testing.T) {
	for _, sz := range [][2]int{{60, 16}, {80, 24}, {132, 43}, {200, 50}} {
		for tab := 1; tab <= 5; tab++ {
			m := newReady(t, sz[0], sz[1])
			press(m, string(rune('0'+tab)))
			for _, extra := range [][]string{nil, {"enter"}, {"b"}} {
				press(m, extra...)
				lines := strings.Split(ansi.Strip(m.View().Content), "\n")
				if len(lines) != sz[1] {
					t.Errorf("%dx%d tab %d %v: %d lines", sz[0], sz[1], tab, extra, len(lines))
				}
				for i, l := range lines {
					if w := lipgloss.Width(l); w > sz[0] {
						t.Errorf("%dx%d tab %d %v line %d is %d wide: %q", sz[0], sz[1], tab, extra, i, w, l)
					}
				}
			}
		}
	}
}

func TestResizeBetweenSizes(t *testing.T) {
	m := newReady(t, 200, 50)
	for tab := 1; tab <= 5; tab++ {
		press(m, string(rune('0'+tab)))
		m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
		if n := len(strings.Split(plain(m), "\n")); n != 25 { // 24 + trailing newline
			t.Errorf("tab %d after shrink: %d lines", tab, n)
		}
		m.Update(tea.WindowSizeMsg{Width: 200, Height: 50})
	}
}

func TestQuitDuringLoadingCancels(t *testing.T) {
	m := newSized(t, 80, 24)
	cmd := press(m, "q")
	if cmd == nil {
		t.Fatal("q during loading must quit")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Error("expected tea.QuitMsg")
	}
	if m.ctx.Err() == nil {
		t.Error("quit must cancel the load context so git children die")
	}
	m2 := newSized(t, 80, 24)
	if cmd := press(m2, "ctrl+c"); cmd == nil || m2.ctx.Err() == nil {
		t.Error("ctrl+c during loading must quit and cancel")
	}
}

func TestLoaderFlow(t *testing.T) {
	load := func(ctx context.Context, progress func(Progress)) (stats.Report, error) {
		progress(Progress{Phase: "blaming", Done: 1, Total: 2})
		return fixtureReport(), nil
	}
	m := New(context.Background(), load, Options{Now: fixedNow})
	defer m.cancel()
	m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})

	msg := m.ldr.start(m.ctx, m.load)()
	for {
		_, cmd := m.Update(msg)
		if m.state == stateReady {
			return
		}
		if cmd == nil {
			t.Fatal("loader stalled")
		}
		msg = cmd()
	}
}

func TestLoadError(t *testing.T) {
	m := newSized(t, 80, 24)
	m.Update(errMsg{errors.New("git exploded")})
	if out := plain(m); !strings.Contains(out, "git exploded") || !strings.Contains(out, "q to quit") {
		t.Errorf("error view:\n%s", out)
	}
	if cmd := press(m, "q"); cmd == nil {
		t.Error("q should quit from the error screen")
	}
}

func TestTabNavigation(t *testing.T) {
	m := newReady(t, 100, 30)
	press(m, "tab")
	if m.active != 1 {
		t.Errorf("tab -> %d", m.active)
	}
	m.Update(keyMsg("shift+tab"))
	m.Update(keyMsg("shift+tab"))
	if m.active != 4 {
		t.Errorf("shift+tab wraps to %d", m.active)
	}
	press(m, "3")
	if m.active != 2 {
		t.Errorf("3 -> %d", m.active)
	}
}

func TestBotsToggle(t *testing.T) {
	m := newReady(t, 120, 40)
	press(m, "2")
	if strings.Contains(plain(m), "dependabot") {
		t.Error("bots must be hidden by default")
	}
	press(m, "b")
	if !strings.Contains(plain(m), "dependabot[bot] (bot)") {
		t.Error("b should reveal bots")
	}
	if !strings.Contains(plain(m), "bots: shown") {
		t.Error("header should say bots are shown")
	}
}

func TestContributorsSortFilterDetail(t *testing.T) {
	m := newReady(t, 120, 40)
	press(m, "2")
	first := func() string {
		for _, l := range strings.Split(plain(m), "\n") {
			for _, n := range []string{"Alice Johnson", "Bob Smith", "李雷", "Dana Reyes", "Eli Dormant"} {
				if strings.Contains(l, n) {
					return n
				}
			}
		}
		return ""
	}
	byOwnership := first()
	press(m, "s", "s", "s") // surviving -> own% -> last active -> name, ascending
	if got := first(); got != "Alice Johnson" {
		t.Errorf("name sort first = %q (was %q)", got, byOwnership)
	}
	press(m, "S")
	if got := first(); got != "李雷" {
		t.Errorf("reversed name sort first = %q", got)
	}

	press(m, "/", "d", "a", "n")
	if m.tabs[1].Capturing() == false {
		t.Error("typing in the filter must capture keys")
	}
	press(m, "q") // must be typed into the box, not quit
	if m.ctx.Err() != nil {
		t.Fatal("q while filtering must not quit")
	}
	press(m, "backspace", "enter")
	out := plain(m)
	if !strings.Contains(out, "Dana Reyes") || strings.Contains(out, "Bob Smith") {
		t.Errorf("filter result wrong:\n%s", out)
	}
	press(m, "enter")
	if out := plain(m); !strings.Contains(out, "Survival rate") || !strings.Contains(out, "Weekly commits") {
		t.Errorf("detail view missing:\n%s", out)
	}
	press(m, "esc")
	if strings.Contains(plain(m), "Survival rate") {
		t.Error("esc should leave the detail view")
	}
}

func TestOwnershipDrillAndBack(t *testing.T) {
	m := newReady(t, 120, 30)
	press(m, "3")
	out := plain(m)
	if !strings.Contains(out, "internal/") || !strings.Contains(out, "README.md") {
		t.Fatalf("root listing:\n%s", out)
	}
	press(m, "enter") // first entry is the largest directory: internal/
	if out := plain(m); !strings.Contains(out, "api/") || !strings.Contains(out, "store/") {
		t.Errorf("after drill in:\n%s", out)
	}
	press(m, "backspace")
	if !strings.Contains(plain(m), "cmd/") {
		t.Error("backspace should return to the root")
	}
}

func TestFilesMarkers(t *testing.T) {
	m := newReady(t, 140, 40)
	press(m, "5")
	out := plain(m)
	if !strings.Contains(out, "! internal/api/auth.go") {
		t.Errorf("silo marker missing:\n%s", out)
	}
	if !strings.Contains(out, "* internal/store/sql/q.go") {
		t.Errorf("single-owner marker missing:\n%s", out)
	}
}

func TestEmptyReport(t *testing.T) {
	m := newSized(t, 80, 24)
	m.Update(reportReadyMsg{stats.Build(stats.Input{Meta: stats.Meta{Path: "/x", Rev: "HEAD", Warnings: []string{"no commits at HEAD"}}})})
	for tab := 1; tab <= 5; tab++ {
		press(m, string(rune('0'+tab)))
		_ = plain(m) // must not panic
	}
}
