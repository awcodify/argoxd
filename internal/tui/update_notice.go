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

// withUpdateBadge puts the update badge at the right edge of the breadcrumb
// row. The breadcrumbs are cut short rather than pushing the badge off screen.
func (m Model) withUpdateBadge(crumbs string) string {
	if m.updateVersion == "" {
		return crumbs
	}
	badge := renderUpdateBadge(m.updateVersion)
	room := m.width - lipgloss.Width(badge) - 1
	if room < 0 {
		return crumbs
	}
	crumbs = ansi.Truncate(crumbs, room, "…")
	return crumbs + strings.Repeat(" ", room-lipgloss.Width(crumbs)+1) + badge
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
