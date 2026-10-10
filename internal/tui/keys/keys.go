// Package keys defines every key binding in one place. Update code matches
// against these with key.Matches and never compares key strings.
package keys

import "charm.land/bubbles/v2/key"

// Map is the full set of bindings.
type Map struct {
	NextTab, PrevTab, Tabs     key.Binding
	Up, Down, PageUp, PageDown key.Binding
	Enter, Back, BackDir       key.Binding
	Sort, SortReverse          key.Binding
	Filter, Bots, Help         key.Binding
	Quit, ForceQuit            key.Binding
}

// Default returns the standard bindings.
func Default() Map {
	return Map{
		NextTab:     key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "next tab")),
		PrevTab:     key.NewBinding(key.WithKeys("shift+tab"), key.WithHelp("shift+tab", "prev tab")),
		Tabs:        key.NewBinding(key.WithKeys("1", "2", "3", "4", "5"), key.WithHelp("1-5", "jump to tab")),
		Up:          key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑/k", "up")),
		Down:        key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("↓/j", "down")),
		PageUp:      key.NewBinding(key.WithKeys("pgup", "ctrl+u"), key.WithHelp("pgup", "page up")),
		PageDown:    key.NewBinding(key.WithKeys("pgdown", "ctrl+d"), key.WithHelp("pgdn", "page down")),
		Enter:       key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "open")),
		Back:        key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "back")),
		BackDir:     key.NewBinding(key.WithKeys("backspace"), key.WithHelp("backspace", "up a level")),
		Sort:        key.NewBinding(key.WithKeys("s"), key.WithHelp("s", "sort column")),
		SortReverse: key.NewBinding(key.WithKeys("S"), key.WithHelp("S", "reverse sort")),
		Filter:      key.NewBinding(key.WithKeys("/"), key.WithHelp("/", "filter")),
		Bots:        key.NewBinding(key.WithKeys("b"), key.WithHelp("b", "toggle bots")),
		Help:        key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),
		Quit:        key.NewBinding(key.WithKeys("q"), key.WithHelp("q", "quit")),
		ForceQuit:   key.NewBinding(key.WithKeys("ctrl+c"), key.WithHelp("ctrl+c", "quit")),
	}
}

// ShortHelp implements help.KeyMap.
func (k Map) ShortHelp() []key.Binding {
	return []key.Binding{k.Tabs, k.Up, k.Enter, k.Back, k.Sort, k.Filter, k.Bots, k.Help, k.Quit}
}

// FullHelp implements help.KeyMap.
func (k Map) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.NextTab, k.PrevTab, k.Tabs},
		{k.Up, k.Down, k.PageUp, k.PageDown},
		{k.Enter, k.Back, k.BackDir},
		{k.Sort, k.SortReverse, k.Filter},
		{k.Bots, k.Help, k.Quit},
	}
}
