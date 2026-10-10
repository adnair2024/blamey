package tui

import (
	"context"

	tea "charm.land/bubbletea/v2"

	"github.com/adnair2024/blamey/internal/stats"
)

// Progress describes the current loading phase. Total is 0 when the phase has
// no countable steps.
type Progress struct {
	Phase       string
	Done, Total int
}

// LoadFunc does the slow work (git, stats). The TUI runs it off the UI
// goroutine and cancels ctx when the user quits.
type LoadFunc func(ctx context.Context, progress func(Progress)) (stats.Report, error)

type (
	progressMsg    Progress
	reportReadyMsg struct{ report stats.Report }
	errMsg         struct{ err error }
)

// loader runs a LoadFunc and feeds its messages through a channel that a
// self-reissuing Cmd drains, as Bubble Tea recommends for long jobs.
type loader struct {
	ch chan tea.Msg
}

func newLoader() *loader { return &loader{ch: make(chan tea.Msg, 1)} }

// start launches the worker and waits for its first message.
func (l *loader) start(ctx context.Context, load LoadFunc) tea.Cmd {
	return func() tea.Msg {
		go l.run(ctx, load)
		return l.wait()()
	}
}

// wait blocks for the next message; the root re-issues it after each progress
// message until the final one arrives.
func (l *loader) wait() tea.Cmd {
	return func() tea.Msg { return <-l.ch }
}

func (l *loader) run(ctx context.Context, load LoadFunc) {
	report, err := load(ctx, func(p Progress) {
		// Drop updates the UI has not consumed yet; only the latest matters.
		select {
		case l.ch <- progressMsg(p):
		default:
		}
	})
	var final tea.Msg = reportReadyMsg{report}
	if err != nil {
		final = errMsg{err}
	}
	// Do not leak this goroutine if the user quit and nobody is listening.
	select {
	case l.ch <- final:
	case <-ctx.Done():
	}
}
