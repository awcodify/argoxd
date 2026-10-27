package tui

import (
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// keyBadgeLife is how long a pressed key stays on screen.
const keyBadgeLife = 1800 * time.Millisecond

// keyBandRows is how many rows the floating caption band takes.
const keyBandRows = 3

// keyBadgeExpired fades the badge of one key press, unless a newer key replaced it.
type keyBadgeExpired struct{ press int }

// WithShowKeys floats each key pressed in the middle of the screen for a
// moment, so a recording of argoxd shows what is being typed.
func (m Model) WithShowKeys(show bool) Model {
	m.showKeys = show
	return m
}

// keyLabel names a key the way a keycap would: arrows as arrows, the others
// by their usual names.
func keyLabel(message tea.KeyMsg) string {
	switch message.Type {
	case tea.KeyUp:
		return "↑"
	case tea.KeyDown:
		return "↓"
	case tea.KeyLeft:
		return "←"
	case tea.KeyRight:
		return "→"
	case tea.KeyBackspace:
		return "⌫"
	case tea.KeyShiftTab:
		return "⇧ tab"
	case tea.KeySpace:
		return "space"
	}
	if label := message.String(); label == " " {
		return "space"
	} else if label != "" {
		return label
	}
	return "?"
}

// notePressedKey records a key for the badge and starts the timer that fades it.
func (m Model) notePressedKey(message tea.KeyMsg) (Model, tea.Cmd) {
	m.keyPress++
	m.keyShown = keyLabel(message)
	press := m.keyPress
	return m, tea.Tick(keyBadgeLife, func(time.Time) tea.Msg { return keyBadgeExpired{press: press} })
}

// withKeyOverlay captions the pressed key in a band across the middle of the
// screen, the way a tutorial video captions the keystrokes. The band spans
// every column instead of following the frame, so it reads as a layer above the
// interface rather than another pane of it. It is translucent rather than
// solid: the rows it covers stay readable through it, dimmed.
func (m Model) withKeyOverlay(content string) string {
	if !m.showKeys || m.keyShown == "" || m.width < 20 {
		return content
	}

	lines := strings.Split(content, "\n")
	row := len(lines) / 2
	top := row - keyBandRows/2
	if top < headerHeight || top+keyBandRows > len(lines) {
		return content
	}

	sheet := lipgloss.NewStyle().Foreground(colorSlate).Background(keyScrim)
	key := sheet.Render("  key   ") + keyBadgeStyle.Padding(0, 3).Render(m.keyShown) + sheet.Render("  ")
	width := lipgloss.Width(key)
	if width >= m.width {
		return content
	}
	left := (m.width - width) / 2

	for index := 0; index < keyBandRows; index++ {
		target := top + index
		behind := padRight(ansi.Strip(lines[target]), m.width)
		if target != row {
			lines[target] = sheet.Render(behind)
			continue
		}
		lines[target] = sheet.Render(ansi.Cut(behind, 0, left)) + key +
			sheet.Render(ansi.Cut(behind, left+width, m.width))
	}
	return strings.Join(lines, "\n")
}
