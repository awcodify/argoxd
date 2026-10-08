package tui

import (
	"math"
	"slices"
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

// pulsePanelsMinWidth is the narrowest screen that gets the panels; narrower
// ones, and screens too short to fit them above the list, get a text summary.
const pulsePanelsMinWidth = 100

// pulseListRows is the room the list below the panels needs: its heading, its
// header, a few rows and the gap above them.
const pulseListRows = 8

// renderPulse shows how many Applications there are in each state, then the
// ones that need attention and why.
func (m Model) renderPulse(width, height int) (string, []string) {
	snapshot := m.explorer.Snapshot()
	needing := len(snapshot.NeedingAttention())
	title := m.listTitle("pulse", "")

	lines := pulsePanels(snapshot, needing, width)
	if lines == nil || height < len(lines)+pulseListRows {
		lines = pulseSummary(snapshot)
	}
	lines = append(lines, "")
	if needing == 0 {
		return title, append(lines, " "+lipgloss.NewStyle().Foreground(statusColor("Healthy")).Render("✓ Everything looks fine."))
	}
	lines = append(lines, " "+columnStyle.Render("NEEDS ATTENTION")+" "+brightStyle.Render(strconv.Itoa(needing)))

	attention := table{columns: []string{"NAME", "PROJECT", "HEALTH", "SYNC", "WHY"}, status: map[int]bool{2: true, 3: true}}
	for _, item := range m.explorer.Attention() {
		application := item.Application
		attention.rows = append(attention.rows, []string{
			application.Name, application.Project, application.Health, application.Sync, strings.Join(item.Reasons, ", "),
		})
	}
	return title, append(lines, attention.render(m.explorer.Cursor(), width, height-len(lines))...)
}

// pulsePanels draws the applications, health, sync and policy panels side by
// side, or returns nil when the screen is too narrow for them.
func pulsePanels(snapshot explorer.Snapshot, needing, width int) []string {
	const applicationsWidth, policyWidth, gaps = 24, 30, 3
	if width < pulsePanelsMinWidth {
		return nil
	}
	flex := (width - applicationsWidth - policyWidth - gaps) / 2
	widths := []int{applicationsWidth, flex + (width-applicationsWidth-policyWidth-gaps)%2, flex, policyWidth}

	total := len(snapshot.Applications)
	automated, selfHeal, pruning := 0, 0, 0
	for _, application := range snapshot.Applications {
		if application.Policy.Automated {
			automated++
		}
		if application.Policy.SelfHeal {
			selfHeal++
		}
		if application.Policy.Prune {
			pruning++
		}
	}
	room := func(panel int) int { return widths[panel] - 4 } // the border and a margin on each side

	health := snapshot.CountByHealth()
	sync := snapshot.CountBySync()
	contents := [][]string{
		pulseApplications(snapshot, needing),
		append([]string{stackedBar(health, pulseHealthOrder, room(1)), ""}, pulseLegend(health, pulseHealthOrder, room(1))...),
		append([]string{stackedBar(sync, pulseSyncOrder, room(2)), ""}, pulseLegend(sync, pulseSyncOrder, room(2))...),
		{
			spaceBetween(textStyle.Render("auto-sync"), brightStyle.Render(strconv.Itoa(automated)+" of "+strconv.Itoa(total)), room(3)),
			progressBar(automated, total, room(3)),
			"",
			spaceBetween(textStyle.Render("self-heal"), brightStyle.Render(strconv.Itoa(selfHeal)), room(3)),
			spaceBetween(textStyle.Render("prune"), brightStyle.Render(strconv.Itoa(pruning)), room(3)),
		},
	}
	tallest := 0
	for _, content := range contents {
		tallest = max(tallest, len(content))
	}

	names := []string{"applications", "health", "sync", "policy"}
	rendered := make([][]string, len(contents))
	for index, content := range contents {
		for line := range content {
			content[line] = " " + content[line]
		}
		rendered[index] = strings.Split(box{
			border: lipgloss.RoundedBorder(),
			color:  colorBorder,
			title:  " " + accentStyle.Render(names[index]) + " ",
			width:  widths[index],
			height: tallest + 2,
		}.render(content), "\n")
	}
	rows := make([]string, tallest+2)
	for row := range rows {
		parts := make([]string, len(rendered))
		for index := range rendered {
			parts[index] = rendered[index][row]
		}
		rows[row] = strings.Join(parts, " ")
	}
	return rows
}

// pulseApplications is the first panel: how many Applications and
// ApplicationSets there are, and how many need attention.
func pulseApplications(snapshot explorer.Snapshot, needing int) []string {
	lines := []string{brightStyle.Render(count(len(snapshot.Applications), "application"))}
	if sets := len(snapshot.ApplicationSets); sets > 0 {
		lines = append(lines, textStyle.Render(count(sets, "application set")))
		problems := 0
		for _, applicationSet := range snapshot.ApplicationSets {
			if len(applicationSet.Problems) > 0 {
				problems++
			}
		}
		if problems > 0 {
			lines = append(lines, warningStyle.Render(conditionGlyph+" "+strconv.Itoa(problems)+" with problems"))
		}
	}
	if needing > 0 {
		return append(lines, "", errorStyle.Render("✗ "+strconv.Itoa(needing)+" need attention"))
	}
	return append(lines, "", lipgloss.NewStyle().Foreground(statusColor("Healthy")).Render("✓ all clear"))
}

// pulseLegend names each status in its color with its count at the end of the line.
func pulseLegend(counts map[string]int, preferred []string, width int) []string {
	var lines []string
	for _, status := range orderedStatuses(counts, preferred) {
		label := status
		if status == "OutOfSync" {
			label = "Out of sync"
		}
		name := lipgloss.NewStyle().Foreground(statusColor(status)).Render(lookupStatus(status).glyph + " " + label)
		lines = append(lines, spaceBetween(name, brightStyle.Render(strconv.Itoa(counts[status])), width))
	}
	return lines
}

// orderedStatuses lists the statuses that have a count: the preferred ones in
// their order, then any others by name.
func orderedStatuses(counts map[string]int, preferred []string) []string {
	var statuses []string
	for _, status := range preferred {
		if counts[status] > 0 {
			statuses = append(statuses, status)
		}
	}
	var others []string
	for status, number := range counts {
		if number > 0 && !slices.Contains(preferred, status) {
			others = append(others, status)
		}
	}
	slices.Sort(others)
	return append(statuses, others...)
}

// barSegment is the part of a stacked bar that one status takes.
type barSegment struct {
	status string
	cells  int
}

// barSegments shares width cells among the statuses in proportion to their
// counts. Every status that has any gets at least one cell, and the cells add
// up to width.
func barSegments(counts map[string]int, preferred []string, width int) []barSegment {
	statuses := orderedStatuses(counts, preferred)
	if len(statuses) > width {
		statuses = statuses[:max(0, width)]
	}
	total := 0
	for _, status := range statuses {
		total += counts[status]
	}
	if total == 0 {
		return nil
	}

	segments := make([]barSegment, len(statuses))
	remainders := make([]float64, len(statuses))
	used := 0
	for index, status := range statuses {
		exact := float64(width) * float64(counts[status]) / float64(total)
		segments[index] = barSegment{status: status, cells: max(1, int(exact))}
		remainders[index] = exact - float64(segments[index].cells)
		used += segments[index].cells
	}
	for ; used < width; used++ {
		next := 0
		for index := range remainders {
			if remainders[index] > remainders[next] {
				next = index
			}
		}
		segments[next].cells++
		remainders[next]--
	}
	for ; used > width; used-- {
		widest := 0
		for index := range segments {
			if segments[index].cells > segments[widest].cells {
				widest = index
			}
		}
		segments[widest].cells--
	}
	return segments
}

// stackedBar draws one block per cell, colored by status, with each status
// taking a share of the width in proportion to its count.
func stackedBar(counts map[string]int, preferred []string, width int) string {
	segments := barSegments(counts, preferred, width)
	if len(segments) == 0 {
		return mutedStyle.Render(strings.Repeat("░", max(0, width)))
	}
	var bar strings.Builder
	for _, segment := range segments {
		bar.WriteString(lipgloss.NewStyle().Foreground(statusColor(segment.status)).Render(strings.Repeat("█", segment.cells)))
	}
	return bar.String()
}

// progressBar fills the share of width that done is of total, and always
// shows at least one block when done is not zero.
func progressBar(done, total, width int) string {
	filled := 0
	if total > 0 {
		filled = min(width, int(math.Round(float64(width)*float64(done)/float64(total))))
		if done > 0 {
			filled = max(1, filled)
		}
	}
	return lipgloss.NewStyle().Foreground(colorAccent).Render(strings.Repeat("█", filled)) +
		mutedStyle.Render(strings.Repeat("░", max(0, width-filled)))
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
	for _, status := range orderedStatuses(counts, order) {
		label := status
		if status == "OutOfSync" {
			label = "out of sync"
		}
		text := lookupStatus(status).glyph + " " + strconv.Itoa(counts[status]) + " " + label
		parts = append(parts, lipgloss.NewStyle().Foreground(statusColor(status)).Render(text))
	}
	return strings.Join(parts, "   ")
}
