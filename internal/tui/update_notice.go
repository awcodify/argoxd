package tui

import (
	"context"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

const updateCheckTimeout = 3 * time.Second

// updateAvailable carries the newer release number the check found, or "".
type updateAvailable struct{ version string }

// WithUpdateCheck looks for a newer release in the background after startup.
// check returns the release to announce, or "" when there is none.
func (m Model) WithUpdateCheck(check func(context.Context) string) Model {
	m.updateCheck = check
	return m
}

// checkForUpdate runs the check as a command so it never blocks the interface.
func (m Model) checkForUpdate() tea.Cmd {
	check := m.updateCheck
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), updateCheckTimeout)
		defer cancel()
		return updateAvailable{version: check(ctx)}
	}
}
