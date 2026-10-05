package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// box draws a bordered frame with its title set into the top border.
// lipgloss cannot place text inside a border, so the frame is drawn by hand.
type box struct {
	border lipgloss.Border
	color  lipgloss.TerminalColor
	title  string
	width  int
	height int
}

func (b box) render(lines []string) string {
	paint := lipgloss.NewStyle().Foreground(b.color).Render
	inner := max(0, b.width-2)
	title := ansi.Truncate(b.title, max(0, inner-1), "")
	trailing := max(0, inner-1-lipgloss.Width(title))

	rows := make([]string, 0, b.height)
	rows = append(rows, paint(b.border.TopLeft+b.border.Top)+title+
		paint(strings.Repeat(b.border.Top, trailing)+b.border.TopRight))
	for index := range max(0, b.height-2) {
		line := ""
		if index < len(lines) {
			line = ansi.Truncate(lines[index], inner, "…")
		}
		rows = append(rows, paint(b.border.Left)+padRight(line, inner)+paint(b.border.Right))
	}
	rows = append(rows, paint(b.border.BottomLeft+strings.Repeat(b.border.Bottom, inner)+b.border.BottomRight))
	return strings.Join(rows, "\n")
}

func padRight(line string, width int) string {
	return line + strings.Repeat(" ", max(0, width-lipgloss.Width(line)))
}
