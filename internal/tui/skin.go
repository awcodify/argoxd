package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// The palette borrows Argo CD's brand orange and its status colors.
var (
	colorAccent  = lipgloss.Color("#EF7B4D")
	colorTeal    = lipgloss.Color("#18BE94")
	colorSky     = lipgloss.Color("#0DADEA")
	colorAmber   = lipgloss.Color("#F4C030")
	colorRose    = lipgloss.Color("#E96D76")
	colorViolet  = lipgloss.Color("#A78BFA")
	colorSlate   = lipgloss.Color("#64748B")
	colorText    = lipgloss.Color("#CBD5E1")
	colorBright  = lipgloss.Color("#F8FAFC")
	colorBorder  = lipgloss.Color("#475569")
	colorSurface = lipgloss.Color("#1E293B")
)

// pruneGlyph flags a resource that a sync with prune would delete, and
// orphanGlyph one that no Application manages.
const (
	pruneGlyph  = "✂"
	orphanGlyph = "◌"
)

// conditionGlyph flags an Application that has warnings or errors.
const conditionGlyph = "⚠"

var (
	accentStyle  = lipgloss.NewStyle().Foreground(colorAccent).Bold(true)
	brightStyle  = lipgloss.NewStyle().Foreground(colorBright).Bold(true)
	textStyle    = lipgloss.NewStyle().Foreground(colorText)
	mutedStyle   = lipgloss.NewStyle().Foreground(colorSlate)
	keycapStyle  = lipgloss.NewStyle().Foreground(colorAccent).Background(colorSurface).Bold(true).Padding(0, 1)
	columnStyle  = lipgloss.NewStyle().Foreground(colorSlate).Bold(true)
	errorStyle   = lipgloss.NewStyle().Foreground(colorRose).Bold(true)
	warningStyle = lipgloss.NewStyle().Foreground(colorAmber)
	infoStyle    = lipgloss.NewStyle().Foreground(colorSky)
	selectedBar  = lipgloss.NewStyle().Foreground(colorAccent).Background(colorSurface).Render("▌")
	selectedText = lipgloss.NewStyle().Foreground(colorBright).Background(colorSurface).Bold(true)
)

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
