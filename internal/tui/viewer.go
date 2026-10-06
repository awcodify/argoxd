package tui

import (
	"hash/fnv"
	"regexp"
	"strings"

	"github.com/awcodify/argoxd/internal/argocd"
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
	// prefixed marks lines that start with the name of the Pod they came from.
	prefixed bool
	// search keeps only the lines that contain it, ignoring case.
	search string
}

func newTextView(kind, subject, text string) textView {
	text = strings.TrimRight(text, "\n")
	if text == "" {
		text = "Nothing to show."
	}
	return textView{kind: kind, subject: subject, lines: strings.Split(text, "\n")}
}

func (m Model) updateViewer(message tea.KeyMsg) (tea.Model, tea.Cmd) {
	last := max(0, len(m.viewer.shown())-m.bodyHeight())
	switch message.String() {
	case "ctrl+c", "q":
		m.stopFollowing()
		return m, tea.Quit
	case "esc":
		if m.viewer.search != "" {
			m.applySearch("")
			return m, nil
		}
		m.stopFollowing()
		m.view = applicationTreeView
		if m.viewerFromList {
			m.view = listView
		}
	case "/":
		if m.viewer.kind == "logs" || m.viewer.kind == "events" {
			m.prompt = prompt{active: true, search: true, input: m.viewer.search}
			return m, nil
		}
	case "p":
		if m.viewer.kind == "logs" {
			return m.openPodBar()
		}
	case "f":
		return m.toggleFollow()
	case "c":
		if m.viewer.kind == "logs" {
			return m.chooseContainer()
		}
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

// shown returns the lines that contain the search, or all of them.
func (v textView) shown() []string {
	if v.search == "" {
		return v.lines
	}
	needle := strings.ToLower(v.search)
	var matching []string
	for _, line := range v.lines {
		if strings.Contains(strings.ToLower(line), needle) {
			matching = append(matching, line)
		}
	}
	return matching
}

func (v textView) render(width, height int) []string {
	shown := v.shown()
	if len(shown) == 0 && v.search != "" {
		return []string{mutedStyle.Render("  No lines match /" + v.search)}
	}
	start := min(v.offset, max(0, len(shown)-height))
	end := min(len(shown), start+height)
	lines := make([]string, 0, end-start)
	for _, line := range shown[start:end] {
		lines = append(lines, v.highlight(line, width))
	}
	return lines
}

// podColors are the colors a workload log gives its Pods, one per name.
var podColors = []lipgloss.Color{colorSky, colorTeal, colorAmber, colorViolet, colorRose, colorAccent}

// podStyle gives a Pod the same color every time it is drawn.
func podStyle(pod string) lipgloss.Style {
	sum := fnv.New32a()
	_, _ = sum.Write([]byte(pod))
	return lipgloss.NewStyle().Foreground(podColors[sum.Sum32()%uint32(len(podColors))])
}

func (v textView) highlight(line string, width int) string {
	if pod, message, found := strings.Cut(line, argocd.LogSeparator); found && v.prefixed {
		return " " + podStyle(pod).Render(pod+strings.TrimRight(argocd.LogSeparator, " ")) + " " + textStyle.Render(message)
	}
	switch v.kind {
	case "events":
		switch {
		case strings.HasPrefix(line, "AGE "):
			return " " + mutedStyle.Render(line)
		case strings.Contains(line, " Warning "):
			return " " + warningStyle.Render(line)
		}
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
