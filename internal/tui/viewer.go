package tui

import (
	"regexp"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// Changed diff lines are drawn as full-width tinted bands, like a code review.
var (
	diffAddedStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#BBF7D0")).Background(lipgloss.Color("#12372A"))
	diffRemovedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#FECDD3")).Background(lipgloss.Color("#4A1D27"))
)

// yamlKey matches the key of a YAML line, including any list marker before it.
var yamlKey = regexp.MustCompile(`^(\s*(?:- )?)([^\s:#][^:#]*):(\s|$)`)

// textView is a scrollable, read-only view of a manifest, diff or log.
type textView struct {
	kind    string
	subject string
	lines   []string
	offset  int
}

func newTextView(kind, subject, text string) textView {
	text = strings.TrimRight(text, "\n")
	if text == "" {
		text = "Nothing to show."
	}
	return textView{kind: kind, subject: subject, lines: strings.Split(text, "\n")}
}

func (m Model) updateViewer(message tea.KeyMsg) (tea.Model, tea.Cmd) {
	last := max(0, len(m.viewer.lines)-m.bodyHeight())
	switch message.String() {
	case "ctrl+c", "q":
		m.stopFollowing()
		return m, tea.Quit
	case "esc":
		m.stopFollowing()
		m.view = applicationTreeView
	case "f":
		return m.toggleFollow()
	case "down", "j":
		m.viewer.offset++
	case "up", "k":
		m.viewer.offset--
	case "pgdown", " ", "ctrl+d":
		m.viewer.offset += m.bodyHeight()
	case "pgup", "ctrl+u":
		m.viewer.offset -= m.bodyHeight()
	case "g", "home":
		m.viewer.offset = 0
	case "G", "end":
		m.viewer.offset = last
	}
	m.viewer.offset = min(max(0, m.viewer.offset), last)
	return m, nil
}

func (v textView) render(width, height int) []string {
	end := min(len(v.lines), v.offset+height)
	lines := make([]string, 0, end-v.offset)
	for _, line := range v.lines[v.offset:end] {
		lines = append(lines, v.highlight(line, width))
	}
	return lines
}

func (v textView) highlight(line string, width int) string {
	switch v.kind {
	case "diff":
		band := padRight(ansi.Truncate(" "+line, width, "…"), width)
		switch {
		case strings.HasPrefix(line, "+"):
			return diffAddedStyle.Render(band)
		case strings.HasPrefix(line, "-"):
			return diffRemovedStyle.Render(band)
		}
	case "yaml":
		if match := yamlKey.FindStringSubmatchIndex(line); match != nil {
			keyEnd := match[5]
			return " " + textStyle.Render(line[:match[4]]) +
				lipgloss.NewStyle().Foreground(colorSky).Render(line[match[4]:keyEnd]) +
				textStyle.Render(line[keyEnd:])
		}
	}
	return " " + textStyle.Render(line)
}
