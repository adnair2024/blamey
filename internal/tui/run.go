package tui

import (
	"context"
	"fmt"

	tea "charm.land/bubbletea/v2"
)

// Run starts the interactive program and blocks until it exits. Cancelling ctx
// quits the program and kills any running git processes.
func Run(ctx context.Context, load LoadFunc, opts Options) error {
	m := New(ctx, load, opts)
	defer m.cancel()
	if _, err := tea.NewProgram(m, tea.WithContext(ctx)).Run(); err != nil {
		return fmt.Errorf("run tui: %w", err)
	}
	return nil
}
