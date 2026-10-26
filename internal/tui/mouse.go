package tui

import tea "github.com/charmbracelet/bubbletea"

// updateMouse turns the wheel into the same moves as the arrow keys: it
// changes the selection in a list and scrolls a viewer. Clicks do nothing, and
// nothing happens behind a prompt or a confirmation.
func (m Model) updateMouse(message tea.MouseMsg) (tea.Model, tea.Cmd) {
	if m.prompt.active || m.confirm.active() || message.Action != tea.MouseActionPress {
		return m, nil
	}
	switch message.Button {
	case tea.MouseButtonWheelUp:
		return m.updateKey(tea.KeyMsg{Type: tea.KeyUp})
	case tea.MouseButtonWheelDown:
		return m.updateKey(tea.KeyMsg{Type: tea.KeyDown})
	}
	return m, nil
}
