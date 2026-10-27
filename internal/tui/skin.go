package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// The colors and styles below are set by setTheme, which init calls with the
// default theme: its palette borrows Argo CD's brand orange and status colors.
var (
	colorAccent  lipgloss.Color
	colorTeal    lipgloss.Color
	colorSky     lipgloss.Color
	colorAmber   lipgloss.Color
	colorRose    lipgloss.Color
	colorViolet  lipgloss.Color
	colorSlate   lipgloss.Color
	colorText    lipgloss.Color
	colorBright  lipgloss.Color
	colorBorder  lipgloss.Color
	colorSurface lipgloss.Color
)

// pruneGlyph flags a resource that a sync with prune would delete, and
// orphanGlyph one that no Application manages.
const (
	pruneGlyph  = "✂"
	orphanGlyph = "◌"
)

// conditionGlyph flags an Application that has warnings or errors.
const conditionGlyph = "⚠"

// syncWindowGlyph flags an Application whose sync is blocked by a window.
const syncWindowGlyph = "⊘"

var (
	accentStyle  lipgloss.Style
	brightStyle  lipgloss.Style
	textStyle    lipgloss.Style
	mutedStyle   lipgloss.Style
	keycapStyle  lipgloss.Style
	columnStyle  lipgloss.Style
	errorStyle   lipgloss.Style
	warningStyle lipgloss.Style
	infoStyle    lipgloss.Style
	selectedBar  string
	selectedText lipgloss.Style

	// Changed diff lines are drawn as full-width tinted bands, like a code review.
	diffAddedStyle      lipgloss.Style
	diffRemovedStyle    lipgloss.Style
	visualSelectedStyle lipgloss.Style
	// updateBadgeStyle fills the update notice so it stands out from the muted
	// header text around it.
	updateBadgeStyle lipgloss.Style
)

func init() { _ = setTheme("default") }

// applyTheme sets the colors and rebuilds every style made from them.
func applyTheme(t theme) {
	colorAccent, colorTeal, colorSky = lipgloss.Color(t.accent), lipgloss.Color(t.healthy), lipgloss.Color(t.info)
	colorAmber, colorRose, colorViolet = lipgloss.Color(t.warning), lipgloss.Color(t.danger), lipgloss.Color(t.suspended)
	colorSlate, colorText, colorBright = lipgloss.Color(t.muted), lipgloss.Color(t.text), lipgloss.Color(t.bright)
	colorBorder, colorSurface = lipgloss.Color(t.border), lipgloss.Color(t.surface)

	accentStyle = lipgloss.NewStyle().Foreground(colorAccent).Bold(true)
	brightStyle = lipgloss.NewStyle().Foreground(colorBright).Bold(true)
	textStyle = lipgloss.NewStyle().Foreground(colorText)
	mutedStyle = lipgloss.NewStyle().Foreground(colorSlate)
	keycapStyle = lipgloss.NewStyle().Foreground(colorAccent).Background(colorSurface).Bold(true).Padding(0, 1)
	columnStyle = lipgloss.NewStyle().Foreground(colorSlate).Bold(true)
	errorStyle = lipgloss.NewStyle().Foreground(colorRose).Bold(true)
	warningStyle = lipgloss.NewStyle().Foreground(colorAmber)
	infoStyle = lipgloss.NewStyle().Foreground(colorSky)
	selectedBar = lipgloss.NewStyle().Foreground(colorAccent).Background(colorSurface).Render("▌")
	selectedText = lipgloss.NewStyle().Foreground(colorBright).Background(colorSurface).Bold(true)

	diffAddedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(t.addedFG)).Background(lipgloss.Color(t.addedBG))
	diffRemovedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(t.removedFG)).Background(lipgloss.Color(t.removedBG))
	visualSelectedStyle = lipgloss.NewStyle().Foreground(colorBright).Background(lipgloss.Color(t.selection))
	updateBadgeStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(t.badgeText)).Background(colorAccent).Bold(true).Padding(0, 1)
}

// status describes how an Argo CD health or sync status is drawn. A higher
// severity wins when several statuses share one color, such as a table row.
type status struct {
	glyph    string
	color    lipgloss.Color
	severity int
}

func lookupStatus(value string) status {
	switch value {
	case "Healthy":
		return status{"♥", colorTeal, 0}
	case "Synced", "Succeeded":
		return status{"✓", colorTeal, 0}
	case "Suspended":
		return status{"‖", colorViolet, 1}
	case "Progressing", "Running":
		return status{"◐", colorSky, 2}
	case "OutOfSync":
		return status{"⟳", colorAmber, 2}
	case "Degraded", "Missing", "Failed", "Error":
		return status{"✗", colorRose, 3}
	default:
		return status{"○", colorSlate, 1}
	}
}

// statusColor returns the color of the most severe of the given statuses.
func statusColor(values ...string) lipgloss.Color {
	worst := status{color: colorTeal, severity: -1}
	for _, value := range values {
		if value == "" {
			continue
		}
		if current := lookupStatus(value); current.severity > worst.severity {
			worst = current
		}
	}
	return worst.color
}

// statusLabel prefixes a status with its glyph, e.g. "♥ Healthy".
func statusLabel(value string) string {
	if value == "" {
		return ""
	}
	return lookupStatus(value).glyph + " " + value
}

// statusBadge renders a status label in its own color.
func statusBadge(value string) string {
	return lipgloss.NewStyle().Foreground(statusColor(value)).Render(statusLabel(value))
}

// keycap renders a key hint such as "s  Sync".
func keycap(key, action string) string {
	return keycapStyle.Render(key) + " " + textStyle.Render(action)
}

// viewTitle renders a frame title such as " ◆ applications · store · 3 ".
func viewTitle(name, scope string, count int) string {
	title := " " + accentStyle.Render("◆") + " " + brightStyle.Render(name)
	if scope != "" {
		title += mutedStyle.Render(" · ") + accentStyle.Render(scope)
	}
	if count >= 0 {
		title += mutedStyle.Render(fmt.Sprintf(" · %d", count))
	}
	return title + " "
}

func joinNonEmpty(values ...string) string {
	var kept []string
	for _, value := range values {
		if value != "" {
			kept = append(kept, value)
		}
	}
	return strings.Join(kept, "  ")
}
