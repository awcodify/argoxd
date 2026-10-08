package tui

import "github.com/awcodify/argoxd/internal/explorer"

// listOrigin is the list that Enter opened the Applications list from, as a
// Project or an ApplicationSet does: the screen, its search and the row that
// was selected.
type listOrigin struct {
	set    bool
	screen explorer.Screen
	search string
	name   string
}

// currentOrigin records the list on screen so Esc can come back to it.
func (m Model) currentOrigin() listOrigin {
	return listOrigin{set: true, screen: m.explorer.Screen(), search: m.explorer.Search(), name: m.explorer.SelectedName()}
}

// returnToOrigin shows the list the Applications were opened from, with its
// search and the same row selected.
func (m *Model) returnToOrigin() {
	origin := m.origin
	m.showScreen(origin.screen)
	m.explorer.SetProject("") // also drops the ApplicationSet filter, so Applications are whole again
	m.explorer.SetSearch(origin.search)
	m.explorer.SelectName(origin.name)
}
