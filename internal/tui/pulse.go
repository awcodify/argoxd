package tui

import (
	"strconv"
	"strings"

	"github.com/awcodify/argoxd/internal/explorer"
	"github.com/charmbracelet/lipgloss"
)

var (
	// pulseHealthOrder and pulseSyncOrder list the statuses counted in the overview.
	pulseHealthOrder = []string{"Healthy", "Progressing", "Suspended", "Missing", "Degraded", "Unknown"}
	pulseSyncOrder   = []string{"Synced", "OutOfSync", "Unknown"}
)

// renderPulse shows how many Applications there are in each state, then the
// ones that need attention and why.
func (m Model) renderPulse(width, height int) (string, []string) {
	snapshot := m.explorer.Snapshot()
	lines := append(pulseSummary(snapshot), "")
	title := m.listTitle("pulse", "")
	if len(snapshot.NeedingAttention()) == 0 {
		return title, append(lines, " "+lipgloss.NewStyle().Foreground(statusColor("Healthy")).Render("✓ Everything looks fine."))
	}

	attention := table{columns: []string{"NAME", "PROJECT", "HEALTH", "SYNC", "WHY"}, status: map[int]bool{2: true, 3: true}}
	for _, item := range m.explorer.Attention() {
		application := item.Application
		attention.rows = append(attention.rows, []string{
			application.Name, application.Project, application.Health, application.Sync, strings.Join(item.Reasons, ", "),
		})
	}
	return title, append(lines, attention.render(m.explorer.Cursor(), width, height-len(lines))...)
}

// pulseSummary counts Applications and ApplicationSets, and the Applications
// by health, sync status and auto-sync.
func pulseSummary(snapshot explorer.Snapshot) []string {
	headline := brightStyle.Render(count(len(snapshot.Applications), "application"))
	if sets := len(snapshot.ApplicationSets); sets > 0 {
		headline += mutedStyle.Render(" · ") + brightStyle.Render(count(sets, "application set"))
		problems := 0
		for _, applicationSet := range snapshot.ApplicationSets {
			if len(applicationSet.Problems) > 0 {
				problems++
			}
		}
		if problems > 0 {
			headline += " " + warningStyle.Render("("+strconv.Itoa(problems)+" with problems)")
		}
	}

	automated := 0
	for _, application := range snapshot.Applications {
		if application.Policy.Automated {
			automated++
		}
	}
	return []string{
		" " + headline,
		pulseLine("Health", pulseCounts(snapshot.CountByHealth(), pulseHealthOrder)),
		pulseLine("Sync", pulseCounts(snapshot.CountBySync(), pulseSyncOrder)),
		pulseLine("Policy", textStyle.Render("auto-sync on "+strconv.Itoa(automated)+" of "+strconv.Itoa(len(snapshot.Applications)))),
	}
}

func pulseLine(label, value string) string {
	return " " + mutedStyle.Render(padRight(label, 8)) + value
}

// pulseCounts draws "♥ 4 Healthy   ✗ 2 Degraded" for the statuses with any Applications.
func pulseCounts(counts map[string]int, order []string) string {
	var parts []string
	for _, status := range order {
		if counts[status] == 0 {
			continue
		}
		label := status
		if status == "OutOfSync" {
			label = "out of sync"
		}
		text := lookupStatus(status).glyph + " " + strconv.Itoa(counts[status]) + " " + label
		parts = append(parts, lipgloss.NewStyle().Foreground(statusColor(status)).Render(text))
	}
	return strings.Join(parts, "   ")
}
