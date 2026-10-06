package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// table is a scrollable resource list. Status columns are drawn as colored
// badges and the selected row is marked with an accent bar.
type table struct {
	columns []string
	rows    [][]string
	status  map[int]bool
	// markable adds a column that flags the rows set in marked.
	markable bool
	marked   []bool
}

func (t table) render(cursor, width, height int) []string {
	cells := t.labels()
	widths := t.columnWidths(cells)
	lines := []string{
		columnStyle.Render(padRight(t.format(t.columns, widths), width)),
		mutedStyle.Render(strings.Repeat("─", width)),
	}
	if len(t.rows) == 0 {
		return append(lines, mutedStyle.Render("  Nothing to show yet."))
	}

	visible := max(0, height-len(lines))
	offset := scrollOffset(cursor, 1, visible)
	for index := offset; index < len(t.rows) && index < offset+visible; index++ {
		lines = append(lines, t.renderRow(index, cells[index], widths, width, index == cursor))
	}
	return lines
}

// labels returns the text of every cell, with glyphs added to status cells.
func (t table) labels() [][]string {
	cells := make([][]string, len(t.rows))
	for index, row := range t.rows {
		cells[index] = make([]string, len(row))
		for column, value := range row {
			cells[index][column] = value
			if t.status[column] {
				cells[index][column] = statusLabel(value)
			}
		}
	}
	return cells
}

func (t table) renderRow(index int, cells []string, widths []int, width int, selected bool) string {
	base := textStyle
	marker := " "
	if selected {
		base = selectedText
		marker = selectedBar
	}
	parts := []string{marker}
	used := 1
	if t.markable {
		mark := " "
		if t.marked[index] {
			mark = accentStyle.Render(markGlyph)
		}
		parts = append(parts, base.Render(mark))
		used++
	}
	for column, cell := range cells {
		style := base
		switch {
		case t.status[column]:
			style = style.Foreground(statusColor(t.rows[index][column]))
		case column == 0:
			style = style.Foreground(colorBright)
		}
		text := " " + padRight(cell, widths[column]) + "  "
		parts = append(parts, style.Render(text))
		used += lipgloss.Width(text)
	}
	parts = append(parts, base.Render(strings.Repeat(" ", max(0, width-used))))
	return strings.Join(parts, "")
}

func (t table) columnWidths(cells [][]string) []int {
	widths := make([]int, len(t.columns))
	for column, header := range t.columns {
		widths[column] = len(header)
		for _, row := range cells {
			widths[column] = max(widths[column], lipgloss.Width(row[column]))
		}
	}
	return widths
}

func (t table) format(cells []string, widths []int) string {
	var line strings.Builder
	line.WriteString(" ")
	if t.markable {
		line.WriteString(" ")
	}
	for column, cell := range cells {
		line.WriteString(" " + padRight(cell, widths[column]) + "  ")
	}
	return line.String()
}

// scrollOffset returns the first line to show so that the selected item,
// which is size lines tall, stays inside a window of height lines.
func scrollOffset(selected, size, height int) int {
	return max(0, (selected+1)*size-height)
}
