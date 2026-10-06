package tui

import (
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/awcodify/argoxd/internal/explorer"
	"github.com/charmbracelet/lipgloss"
)

const (
	promptHeight      = 3
	visibleSuggestion = 6
)

// renderContent returns the frame title and body lines for the active view.
func (m Model) renderContent(width, height int) (string, []string) {
	switch m.view {
	case inventoryTreeView:
		return viewTitle("inventory", "", -1), m.renderInventory(width, height)
	case applicationTreeView:
		application := m.application()
		title := viewTitle(application.Name, application.Project, len(m.resourceTree.Nodes))
		return withSearch(title, m.treeSearch, m.treeFilter), m.renderDependencies(width, height)
	case textViewMode:
		return m.viewerTitle(), m.viewer.render(width, height)
	case historyViewMode:
		return viewTitle("history", m.history.application, len(m.history.entries)), m.historyTable().render(m.history.cursor, width, height)
	}

	switch m.explorer.Screen() {
	case explorer.ApplicationsScreen:
		project := m.explorer.Project()
		if project == "" {
			project = "all"
		}
		return m.listTitle("applications", project), m.applicationsTable().render(m.explorer.Cursor(), width, height)
	case explorer.ProjectsScreen:
		return m.listTitle("projects", ""), m.projectsTable().render(m.explorer.Cursor(), width, height)
	case explorer.ClustersScreen:
		return m.listTitle("clusters", ""), m.clustersTable().render(m.explorer.Cursor(), width, height)
	default:
		return viewTitle("settings", "", -1), []string{
			"",
			" " + mutedStyle.Render(padRight("Connection", 12)) + brightStyle.Render(m.connection),
			" " + mutedStyle.Render(padRight("Namespace", 12)) + brightStyle.Render(m.namespace),
			"",
			mutedStyle.Render(" Configure the connection with command-line flags. Press r to refresh."),
		}
	}
}

// viewerTitle names the open text, and for a diff explains which side is which.
func (m Model) viewerTitle() string {
	title := viewTitle(m.viewer.kind, m.viewer.subject, -1)
	if m.viewer.kind == "logs" {
		return m.logsTitle()
	}
	if m.viewer.kind != "diff" {
		return title
	}
	return strings.TrimSuffix(title, " ") + mutedStyle.Render(" · ") +
		diffRemovedStyle.Render(" − live ") + " " + diffAddedStyle.Render(" + desired ") + " "
}

// listTitle renders the frame title of a list, showing the active search and filter.
func (m Model) listTitle(name, scope string) string {
	return withSearch(viewTitle(name, scope, m.explorer.RowCount()), m.explorer.Search(), m.explorer.Filter())
}

// withSearch appends the active search and filter to a frame title, e.g. "· /pod · health:Degraded".
func withSearch(title, search string, filter explorer.Filter) string {
	var parts []string
	if search != "" {
		parts = append(parts, accentStyle.Render("/"+search))
	}
	for _, label := range filter.Labels() {
		parts = append(parts, accentStyle.Render(label))
	}
	if len(parts) == 0 {
		return title
	}
	return strings.TrimSuffix(title, " ") + mutedStyle.Render(" · ") + strings.Join(parts, mutedStyle.Render(" · ")) + " "
}

func (m Model) applicationsTable() table {
	result := table{
		columns: []string{"NAME", "PROJECT", "SYNC", "HEALTH", "REVISION", "DESTINATION", "LAST SYNC"},
		status:  map[int]bool{2: true, 3: true},
	}
	for _, application := range m.explorer.Applications() {
		result.rows = append(result.rows, []string{
			application.Name, application.Project, application.Sync, application.Health,
			application.Revision, application.Destination, age(application.LastSync),
		})
	}
	return result
}

// age describes how long ago something happened, e.g. "45s", "5m", "3h" or "2d".
func age(moment time.Time) string {
	if moment.IsZero() {
		return ""
	}
	elapsed := time.Since(moment)
	switch {
	case elapsed < time.Minute:
		return strconv.Itoa(int(elapsed.Seconds())) + "s"
	case elapsed < time.Hour:
		return strconv.Itoa(int(elapsed.Minutes())) + "m"
	case elapsed < 24*time.Hour:
		return strconv.Itoa(int(elapsed.Hours())) + "h"
	default:
		return strconv.Itoa(int(elapsed.Hours()/24)) + "d"
	}
}

func (m Model) projectsTable() table {
	result := table{columns: []string{"NAME", "APPS", "DESCRIPTION"}}
	for _, project := range m.explorer.Projects() {
		count := 0
		for _, application := range m.explorer.Snapshot().Applications {
			if application.Project == project.Name {
				count++
			}
		}
		result.rows = append(result.rows, []string{project.Name, strconv.Itoa(count), project.Description})
	}
	return result
}

func (m Model) clustersTable() table {
	result := table{columns: []string{"NAME", "SERVER"}}
	for _, cluster := range m.explorer.Clusters() {
		result.rows = append(result.rows, []string{cluster.Name, cluster.Server})
	}
	return result
}

func (m Model) renderInventory(width, height int) []string {
	visible := m.visibleTree()
	if len(visible) == 0 {
		return []string{mutedStyle.Render("  Nothing to show yet.")}
	}
	offset := scrollOffset(m.treeCursor, 1, height)
	lines := make([]string, 0, height)
	for index := offset; index < len(visible) && index < offset+height; index++ {
		item := visible[index]
		marker := "·"
		if item.expandable {
			marker = "▸"
			if m.expanded[item.key] {
				marker = "▾"
			}
		}
		row := " " + strings.Repeat("  ", item.depth) + marker + " " + item.label
		if index == m.treeCursor {
			lines = append(lines, selectedBar+selectedText.Render(padRight(row+"   "+item.detail, width-1)))
			continue
		}
		lines = append(lines, " "+textStyle.Render(row)+"   "+mutedStyle.Render(item.detail))
	}
	return lines
}

// renderCrumbs shows where the user is, e.g. "◆ applications › checkout".
func (m Model) renderCrumbs() string {
	crumbs := []string{screenName(m.explorer.Screen())}
	switch m.view {
	case inventoryTreeView:
		crumbs = []string{"inventory"}
	case applicationTreeView:
		crumbs = append(crumbs, m.resourceTree.Application)
	case textViewMode:
		if m.viewerFromList {
			crumbs = append(crumbs, m.viewer.subject, m.viewer.kind)
		} else {
			crumbs = append(crumbs, m.resourceTree.Application, m.viewer.kind)
		}
	case historyViewMode:
		crumbs = append(crumbs, m.history.application, "history")
	}
	rendered := make([]string, len(crumbs))
	for index, crumb := range crumbs {
		rendered[index] = mutedStyle.Render(crumb)
		if index == len(crumbs)-1 {
			rendered[index] = brightStyle.Render(crumb)
		}
	}
	return " " + accentStyle.Render("◆") + " " + strings.Join(rendered, mutedStyle.Render(" › "))
}

// renderFlash shows the most urgent message: a pending confirmation, an error, or progress.
func (m Model) renderFlash() string {
	switch {
	case m.syncing.application != "":
		return " " + accentStyle.Render("⟳ Sync "+m.syncing.application+"?") +
			"   " + keycap("enter", "sync") +
			"  " + keycap("p", "prune "+toggle(m.syncing.options.Prune)) +
			"  " + keycap("r", "dry run "+toggle(m.syncing.options.DryRun)) +
			"  " + keycap("esc", "cancel")
	case m.rollingBack.application != "":
		return " " + accentStyle.Render("↩ Roll back "+m.rollingBack.application+" to "+revisionLabel(m.rollingBack.entry)+"?") +
			"   " + keycap("enter", "roll back") +
			"  " + keycap("p", "prune "+toggle(m.rollingBack.options.Prune)) +
			"  " + keycap("r", "dry run "+toggle(m.rollingBack.options.DryRun)) +
			"  " + keycap("esc", "cancel")
	case m.resourceAction.action == "restart":
		return " " + accentStyle.Render("↻ Restart "+m.resourceAction.subject()+"?") +
			"   " + keycap("y", "confirm") + "  " + keycap("n", "cancel")
	case m.resourceAction.action == "delete":
		return " " + errorStyle.Render("✗ Delete "+m.resourceAction.subject()+"?") +
			"   " + keycap("y", "confirm") + "  " + keycap("n", "cancel")
	case m.confirming != "":
		return " " + errorStyle.Render("✗ Delete "+m.confirming+" and its managed resources?") +
			"   " + keycap("y", "confirm") + "  " + keycap("n", "cancel")
	case m.err != nil:
		return " " + errorStyle.Render("✗ "+m.err.Error())
	case m.loading:
		return " " + infoStyle.Render("◐ Refreshing…")
	case m.status != "":
		return " " + infoStyle.Render("● "+m.status)
	default:
		return ""
	}
}

// renderPrompt draws the command line: the input, the selected suggestion as
// ghost text after it, and the other matches alongside.
func (m Model) renderPrompt(width int) string {
	suggestion := m.selectedSuggestion()
	symbol, title := "❯", "command"
	switch {
	case m.prompt.search:
		symbol, title = "/", "search"
	case m.prompt.field != "":
		symbol, title = "▾", m.prompt.field
	}
	line := " " + accentStyle.Render(symbol) + " " + brightStyle.Render(m.prompt.input)
	// Completing what was typed shows the rest as ghost text; with nothing typed
	// the selected chip already says it.
	if ghost, found := strings.CutPrefix(suggestion, m.prompt.input); found && m.prompt.input != "" {
		line += mutedStyle.Render(ghost)
	}
	line += accentStyle.Render("▏")

	matches := m.suggestions()
	if len(matches) > visibleSuggestion {
		// Scroll the chips so the selected one stays in view.
		selected := m.prompt.selection % len(matches)
		start := max(0, selected-visibleSuggestion+1)
		matches = matches[start : start+visibleSuggestion]
	}
	chips := make([]string, 0, len(matches))
	selectedChip := 0
	for _, match := range matches {
		label := match
		if command, found := findFilterCommand(match); found && !m.prompt.search && m.prompt.field == "" {
			label += " " + mutedStyle.Render("shift+"+command.shortcut)
		}
		if match == suggestion {
			selectedChip = len(chips)
			chips = append(chips, keycapStyle.Render(label))
			continue
		}
		chips = append(chips, mutedStyle.Render(" "+label+" "))
	}
	if len(chips) > 0 {
		hint := mutedStyle.Render("⇥ complete  ↑↓ choose")
		budget := width - 2 - lipgloss.Width(line) - 4 - 2 - lipgloss.Width(hint)
		line += "    " + strings.Join(fitChips(chips, selectedChip, budget), " ") + "  " + hint
	}

	return box{
		border: lipgloss.RoundedBorder(),
		color:  colorAccent,
		title:  " " + accentStyle.Render(title) + " ",
		width:  width,
		height: promptHeight,
	}.render([]string{line})
}

func toggle(on bool) string {
	if on {
		return "● on"
	}
	return "○ off"
}

func screenName(screen explorer.Screen) string {
	switch screen {
	case explorer.ApplicationsScreen:
		return "applications"
	case explorer.ProjectsScreen:
		return "projects"
	case explorer.ClustersScreen:
		return "clusters"
	default:
		return "settings"
	}
}

// logsTitle names the open log and says what it shows: the Pod and container
// chosen, the search, and whether it is followed. Long names can leave no room
// for all of that, so the workload, the Pod and the container drop out in turn
// while the search and the following marker stay.
func (m Model) logsTitle() string {
	levels := []struct{ subject, pod, container bool }{
		{true, true, true}, {false, true, true}, {false, false, true}, {false, false, false},
	}
	var title string
	for _, level := range levels {
		subject := ""
		if level.subject {
			subject = m.viewer.subject
		}
		title = strings.TrimSuffix(viewTitle("logs", subject, -1), " ")
		if level.pod && m.logPod != "" {
			title += mutedStyle.Render(" · ") + accentStyle.Render("pod: "+m.logPod)
		}
		if label := m.containerLabel(); level.container && label != "" {
			title += mutedStyle.Render(" · ") + accentStyle.Render("container: "+label)
		}
		if m.viewer.search != "" {
			title += mutedStyle.Render(" · ") + accentStyle.Render("/"+m.viewer.search)
		}
		if m.following() {
			title += mutedStyle.Render(" · ") + infoStyle.Render("● following")
		}
		title += " "
		if lipgloss.Width(title) <= m.width-6 {
			break
		}
	}
	return title
}

// fitChips keeps the chips that fit in budget columns, scrolling so the selected
// one stays visible. A "…" marks each side where chips are hidden.
func fitChips(chips []string, selected, budget int) []string {
	const marker = 2
	fits := func(from, to int) bool {
		used := 0
		for _, chip := range chips[from:to] {
			used += lipgloss.Width(chip) + 1
		}
		if from > 0 {
			used += marker
		}
		if to < len(chips) {
			used += marker
		}
		return used <= budget
	}

	start := 0
	for start < selected && !fits(start, selected+1) {
		start++
	}
	end := selected + 1
	for end < len(chips) && fits(start, end+1) {
		end++
	}

	fitted := slices.Clone(chips[start:end])
	if start > 0 {
		fitted = append([]string{mutedStyle.Render("…")}, fitted...)
	}
	if end < len(chips) {
		fitted = append(fitted, mutedStyle.Render("…"))
	}
	return fitted
}
