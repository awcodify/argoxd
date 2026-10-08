package explorer

import (
	"slices"
	"strconv"
	"strings"
)

// Attention is an Application that needs a look, with the reasons why.
type Attention struct {
	Application Application
	Reasons     []string
}

// NeedingAttention lists the Applications that are degraded or missing, whose
// last sync failed, or that have conditions. The ones with the most reasons
// come first, then they are ordered by name.
func (s Snapshot) NeedingAttention() []Attention {
	var items []Attention
	for _, application := range s.Applications {
		var reasons []string
		if application.Health == "Degraded" || application.Health == "Missing" {
			reasons = append(reasons, application.Health)
		}
		if operation := application.Operation; operation != nil && (operation.Phase == "Failed" || operation.Phase == "Error") {
			reasons = append(reasons, "sync "+operation.Phase)
		}
		if conditions := len(application.Conditions); conditions > 0 {
			noun := " condition"
			if conditions > 1 {
				noun += "s"
			}
			reasons = append(reasons, strconv.Itoa(conditions)+noun)
		}
		if len(reasons) > 0 {
			items = append(items, Attention{Application: application, Reasons: reasons})
		}
	}
	slices.SortStableFunc(items, func(a, b Attention) int {
		if len(a.Reasons) != len(b.Reasons) {
			return len(b.Reasons) - len(a.Reasons)
		}
		return strings.Compare(a.Application.Name, b.Application.Name)
	})
	return items
}

// CountByHealth counts the Applications by health status; one that reports
// none counts as Unknown.
func (s Snapshot) CountByHealth() map[string]int {
	return s.count(func(application Application) string { return application.Health })
}

// CountBySync counts the Applications by sync status; one that reports none
// counts as Unknown.
func (s Snapshot) CountBySync() map[string]int {
	return s.count(func(application Application) string { return application.Sync })
}

func (s Snapshot) count(status func(Application) string) map[string]int {
	counts := make(map[string]int)
	for _, application := range s.Applications {
		value := status(application)
		if value == "" {
			value = "Unknown"
		}
		counts[value]++
	}
	return counts
}

// Attention returns the Applications needing attention whose name matches the search.
func (m Model) Attention() []Attention {
	var matches []Attention
	for _, item := range m.snapshot.NeedingAttention() {
		if m.matchesSearch(item.Application.Name) {
			matches = append(matches, item)
		}
	}
	return matches
}
