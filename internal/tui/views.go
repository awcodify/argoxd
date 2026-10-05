package tui

import (
	"strconv"
	"strings"

	"github.com/awcodify/argoxd/internal/explorer"
	"github.com/charmbracelet/lipgloss"
)

const (
	promptHeight      = 3
	visibleSuggestion = 6
)

// renderContent returns the frame title and body lines for the active view.
func (m Model) renderContent(width, height int) (string, []string) {
	snapshot := m.explorer.Snapshot()
	switch m.view {
	case inventoryTreeView:
		return viewTitle("inventory", "", -1), m.renderInventory(width, height)
	case applicationTreeView:
		application := m.application()
		return viewTitle(application.Name, application.Project, len(m.resourceTree.Nodes)), m.renderDependencies(width, height)
	}

	switch m.explorer.Screen() {
	case explorer.ApplicationsScreen:
		project := m.explorer.Project()
		if project == "" {
			project = "all"
		}
		return viewTitle("applications", project, m.explorer.RowCount()), m.applicationsTable().render(m.explorer.Cursor(), width, height)
	case explorer.ProjectsScreen:
		return viewTitle("projects", "", len(snapshot.Projects)), m.projectsTable().render(m.explorer.Cursor(), width, height)
	case explorer.ClustersScreen:
		return viewTitle("clusters", "", len(snapshot.Clusters)), m.clustersTable().render(m.explorer.Cursor(), width, height)
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

func (m Model) applicationsTable() table {
	result := table{columns: []string{"NAME", "PROJECT", "SYNC", "HEALTH"}, status: map[int]bool{2: true, 3: true}}
	for _, application := range m.explorer.Applications() {
		result.rows = append(result.rows, []string{application.Name, application.Project, application.Sync, application.Health})
	}
	return result
}

func (m Model) projectsTable() table {
	snapshot := m.explorer.Snapshot()
	result := table{columns: []string{"NAME", "APPS", "DESCRIPTION"}}
	for _, project := range snapshot.Projects {
		count := 0
		for _, application := range snapshot.Applications {
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
	for _, cluster := range m.explorer.Snapshot().Clusters {
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
	line := " " + accentStyle.Render("❯") + " " + brightStyle.Render(m.prompt.input)
	if ghost, found := strings.CutPrefix(suggestion, m.prompt.input); found {
		line += mutedStyle.Render(ghost)
	}
	line += accentStyle.Render("▏")

	matches := m.suggestions()
	if len(matches) > visibleSuggestion {
		matches = matches[:visibleSuggestion]
	}
	chips := make([]string, 0, len(matches))
	for _, match := range matches {
		if match == suggestion {
			chips = append(chips, keycapStyle.Render(match))
			continue
		}
		chips = append(chips, mutedStyle.Render(" "+match+" "))
	}
	if len(chips) > 0 {
		line += "    " + strings.Join(chips, " ") + "  " + mutedStyle.Render("⇥ complete  ↑↓ choose")
	}

	return box{
		border: lipgloss.RoundedBorder(),
		color:  colorAccent,
		title:  " " + accentStyle.Render("command") + " ",
		width:  width,
		height: promptHeight,
	}.render([]string{line})
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
