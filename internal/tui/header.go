package tui

import (
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

const (
	headerHeight     = 5
	projectNameWidth = 14
)

type keyHint struct {
	key    string
	action string
}

// renderHeader lays out the connection details, project shortcuts, key hints
// and the wordmark side by side. The wordmark is dropped when space runs out.
func (m Model) renderHeader(width int) string {
	columns := []string{m.renderConnection()}
	for _, hints := range chunk(m.projectHints(), headerHeight) {
		columns = append(columns, renderHints(hints))
	}
	for _, hints := range chunk(m.keyHints(), headerHeight) {
		columns = append(columns, renderHints(hints))
	}

	left := lipgloss.JoinHorizontal(lipgloss.Top, columns...)
	mark := renderWordmark()
	header := lipgloss.NewStyle().MaxWidth(width).Render(left)
	if gap := width - lipgloss.Width(left) - lipgloss.Width(mark); gap > 0 {
		header = lipgloss.JoinHorizontal(lipgloss.Top, left, strings.Repeat(" ", gap), mark)
	}
	return lipgloss.NewStyle().Height(headerHeight).MaxHeight(headerHeight).Render(header)
}

func (m Model) renderConnection() string {
	project := m.explorer.Project()
	if project == "" {
		project = "all"
	}
	rows := [][2]string{
		{"Context", m.connection},
		{"Namespace", m.namespace},
		{"Project", project},
		{"Apps", strconv.Itoa(len(m.explorer.Applications()))},
	}
	lines := make([]string, 0, len(rows))
	for _, row := range rows {
		lines = append(lines, " "+mutedStyle.Render(padRight(row[0], 11))+brightStyle.Render(row[1]))
	}
	return lipgloss.NewStyle().MarginRight(4).Render(strings.Join(lines, "\n"))
}

// projectHints lists the number shortcuts: 0 for every project, 1–9 for the first nine.
func (m Model) projectHints() []keyHint {
	hints := []keyHint{{"0", "all"}}
	for index, project := range m.explorer.Snapshot().Projects {
		if index == 9 {
			break
		}
		hints = append(hints, keyHint{strconv.Itoa(index + 1), ansi.Truncate(project.Name, projectNameWidth, "…")})
	}
	return hints
}

func (m Model) keyHints() []keyHint {
	hints := []keyHint{{":", "Command"}, {"t", "Inventory"}}
	if m.view != listView {
		hints = append(hints, keyHint{"esc", "Back"})
	}
	if m.selectedApplication() != "" {
		if m.view == listView {
			hints = append(hints, keyHint{"enter", "Dependencies"})
		}
		hints = append(hints, keyHint{"s", "Sync"}, keyHint{"d", "Delete"})
	}
	return append(hints, keyHint{"r", "Refresh"}, keyHint{"q", "Quit"})
}

// renderHints right-aligns the keycaps so their actions line up.
func renderHints(hints []keyHint) string {
	keyWidth := 0
	for _, hint := range hints {
		keyWidth = max(keyWidth, len(hint.key))
	}
	lines := make([]string, 0, len(hints))
	for _, hint := range hints {
		lines = append(lines, strings.Repeat(" ", keyWidth-len(hint.key))+keycap(hint.key, hint.action))
	}
	return lipgloss.NewStyle().MarginRight(3).Render(strings.Join(lines, "\n"))
}

func renderWordmark() string {
	argo := lipgloss.NewStyle().Foreground(colorAccent).Bold(true)
	xd := lipgloss.NewStyle().Foreground(colorTeal).Bold(true)
	return strings.Join([]string{
		argo.Render("▄▀█ █▀█ █▀▀ █▀█ ") + xd.Render("▀▄▀ █▀▄ "),
		argo.Render("█▀█ █▀▄ █▄█ █▄█ ") + xd.Render("█ █ █▄▀ "),
		"",
		mutedStyle.Render("Argo CD, from the terminal "),
	}, "\n")
}

func chunk[T any](items []T, size int) [][]T {
	var chunks [][]T
	for len(items) > size {
		chunks = append(chunks, items[:size])
		items = items[size:]
	}
	return append(chunks, items)
}
