package tui

import (
	"slices"
	"strings"

	"github.com/awcodify/argoxd/internal/explorer"
	tea "github.com/charmbracelet/bubbletea"
)

// showSyncDetails opens the outcome of the last sync of the selected
// Application on the list, or of the Application in the dependency view.
func (m Model) showSyncDetails() (tea.Model, tea.Cmd) {
	var name string
	fromList := false
	switch m.view {
	case listView:
		if m.explorer.Screen() != explorer.ApplicationsScreen || m.explorer.SelectedName() == "" {
			return m, nil
		}
		name, fromList = m.explorer.SelectedName(), true
	case applicationTreeView:
		name = m.resourceTree.Application
	default:
		return m, nil
	}
	m.viewer = newTextView("sync", name, syncDetailsText(m.applicationNamed(name)))
	m.viewerFromList = fromList
	m.view = textViewMode
	return m, nil
}

// syncDetailsText describes an Application's last sync: how it ended, then
// what it did to each resource, with the resources that did not sync first.
func syncDetailsText(application explorer.Application) string {
	operation := application.Operation
	if operation == nil {
		return application.Name + " has not been synced yet."
	}
	var text strings.Builder
	field := func(label, value string) {
		if value != "" {
			text.WriteString(padRight(label, 10) + value + "\n")
		}
	}
	field("Phase", operation.Phase)
	field("Message", operation.Message)
	field("Revision", shortRevision(operation.Revision))
	if !operation.StartedAt.IsZero() {
		field("Started", age(operation.StartedAt)+" ago")
	}
	if !operation.FinishedAt.IsZero() {
		field("Finished", age(operation.FinishedAt)+" ago")
	}
	if len(operation.Results) == 0 {
		return text.String()
	}

	results := slices.Clone(operation.Results)
	slices.SortStableFunc(results, func(a, b explorer.OperationResult) int {
		return resultRank(a) - resultRank(b)
	})
	rows := [][]string{{"RESOURCE", "NAMESPACE", "PHASE", "STATUS", "MESSAGE"}}
	for _, result := range results {
		phase := result.SyncPhase
		if result.HookType != "" {
			phase += " hook"
		}
		rows = append(rows, []string{result.Kind + "/" + result.Name, result.Namespace, phase, result.Status, result.Message})
	}
	widths := make([]int, len(rows[0])-1)
	for _, row := range rows {
		for column := range widths {
			widths[column] = max(widths[column], len(row[column]))
		}
	}
	text.WriteString("\n")
	for _, row := range rows {
		for column, width := range widths {
			text.WriteString(padRight(row[column], width+2))
		}
		text.WriteString(row[len(widths)] + "\n")
	}
	return text.String()
}

// resultRank orders resources that did not sync before those that did.
func resultRank(result explorer.OperationResult) int {
	if result.Status == "Synced" || result.Status == "" {
		return 1
	}
	return 0
}
