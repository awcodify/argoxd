package tui

import (
	"context"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

const updateCheckTimeout = 3 * time.Second

// updateBadgeStyle fills the notice with the accent color so it stands out
// from the muted header text around it.
var updateBadgeStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("#0D1117")).
	Background(colorAccent).
	Bold(true).
	Padding(0, 1)

// renderUpdateBadge is the one-line notice that a newer release exists.
func renderUpdateBadge(version string) string {
	return updateBadgeStyle.Render("↑ " + version + " available")
}

// updateAvailable carries the newer release number the check found, or "".
type updateAvailable struct{ version string }

// WithUpdateCheck looks for a newer release in the background after startup.
// check returns the release to announce, or "" when there is none.
func (m Model) WithUpdateCheck(check func(context.Context) string) Model {
	m.updateCheck = check
	return m
}

// renderFooter is the bottom line: the status message on the left and, when a
// newer release exists, the update badge at the right edge. The message is cut
// short rather than pushing the badge off screen.
func (m Model) renderFooter() string {
	flash := m.renderFlash()
	if m.updateVersion == "" {
		return flash
	}
	badge := renderUpdateBadge(m.updateVersion)
	room := m.width - lipgloss.Width(badge) - 1
	if room < 0 {
		return flash
	}
	flash = ansi.Truncate(flash, room, "…")
	return flash + strings.Repeat(" ", room-lipgloss.Width(flash)+1) + badge
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
