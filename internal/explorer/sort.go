package explorer

import (
	"slices"
	"strings"
)

// SortField is what the Applications list is ordered by.
type SortField string

const (
	// SortDefault keeps the order the source returned.
	SortDefault SortField = ""
	SortName    SortField = "name"
	// SortHealth puts the Applications in the worst health first.
	SortHealth SortField = "health"
	// SortSync puts the Applications that are out of sync first.
	SortSync SortField = "sync"
)

// ParseSortField reads a sort field typed by the user; an empty string means the default order.
func ParseSortField(input string) (SortField, bool) {
	switch field := SortField(strings.ToLower(strings.TrimSpace(input))); field {
	case SortDefault, SortName, SortHealth, SortSync:
		return field, true
	}
	return SortDefault, false
}

// healthRank orders health from worst to best.
func healthRank(health string) int {
	switch health {
	case "Degraded":
		return 0
	case "Missing":
		return 1
	case "Progressing":
		return 2
	case "Suspended":
		return 3
	case "Healthy":
		return 5
	}
	return 4
}

// syncRank orders sync status from worst to best.
func syncRank(sync string) int {
	switch sync {
	case "OutOfSync":
		return 0
	case "Synced":
		return 2
	}
	return 1
}

// sortApplications orders applications in place. Equal applications are
// ordered by name, so the result does not depend on the source's order.
func sortApplications(applications []Application, field SortField, reverse bool) {
	if field == SortDefault {
		return
	}
	compare := func(a, b Application) int {
		var result int
		switch field {
		case SortHealth:
			result = healthRank(a.Health) - healthRank(b.Health)
		case SortSync:
			result = syncRank(a.Sync) - syncRank(b.Sync)
		}
		if result == 0 {
			result = strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name))
		}
		if reverse {
			return -result
		}
		return result
	}
	slices.SortStableFunc(applications, compare)
}

// SetSort orders the Applications list and keeps the selected Application selected.
func (m *Model) SetSort(field SortField, reverse bool) {
	selected := m.SelectedName()
	m.sortField, m.sortReverse = field, reverse
	m.refreshApplications()
	for index, name := range m.rowNames() {
		if name == selected {
			m.cursor = index
			return
		}
	}
}

// Sort returns how the Applications list is ordered.
func (m Model) Sort() (SortField, bool) {
	return m.sortField, m.sortReverse
}
