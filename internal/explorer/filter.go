package explorer

import (
	"fmt"
	"slices"
	"strings"
)

// Argo CD's health and sync statuses, in the order the filter cycles through them.
var (
	HealthStatuses = []string{"Healthy", "Progressing", "Degraded", "Suspended", "Missing", "Unknown"}
	SyncStatuses   = []string{"Synced", "OutOfSync", "Unknown"}
)

// Filter narrows rows by status, and resources by kind. Empty fields match everything.
type Filter struct {
	Health string
	Sync   string
	Kind   string
}

// Active reports whether the filter excludes anything.
func (f Filter) Active() bool {
	return f != Filter{}
}

// Matches reports whether a row with the given statuses and kind passes the
// filter, ignoring case.
func (f Filter) Matches(sync, health, kind string) bool {
	return equalOrEmpty(f.Health, health) && equalOrEmpty(f.Sync, sync) && equalOrEmpty(f.Kind, kind)
}

func equalOrEmpty(want, got string) bool {
	return want == "" || strings.EqualFold(want, got)
}

// Labels describes the active fields, e.g. "health:Degraded".
func (f Filter) Labels() []string {
	var labels []string
	for _, field := range [][2]string{{"health", f.Health}, {"sync", f.Sync}, {"kind", f.Kind}} {
		if field[1] != "" {
			labels = append(labels, field[0]+":"+field[1])
		}
	}
	return labels
}

// FilterValues lists the values a filter field ("health", "sync" or "kind") accepts.
func FilterValues(key string, kinds []string) []string {
	switch key {
	case "health":
		return HealthStatuses
	case "sync":
		return SyncStatuses
	case "kind":
		return kinds
	default:
		return nil
	}
}

// AllValues is the choice that clears a filter field.
const AllValues = "all"

// With sets one filter field to a value, matched case-insensitively. An empty
// value, or "all", clears the field.
func (f Filter) With(key, value string, kinds []string) (Filter, error) {
	if strings.EqualFold(value, AllValues) {
		value = ""
	}
	if value != "" {
		values := FilterValues(key, kinds)
		index := slices.IndexFunc(values, func(candidate string) bool { return strings.EqualFold(candidate, value) })
		if index < 0 {
			return f, fmt.Errorf("%s:%s is not a known value; use %s", key, value, strings.Join(values, ", "))
		}
		value = values[index]
	}
	switch key {
	case "health":
		f.Health = value
	case "sync":
		f.Sync = value
	case "kind":
		f.Kind = value
	}
	return f, nil
}

// Kinds lists the distinct resource kinds of the tree, sorted.
func (t ResourceTree) Kinds() []string {
	var kinds []string
	for _, node := range t.Nodes {
		if node.Kind != "" && !slices.Contains(kinds, node.Kind) {
			kinds = append(kinds, node.Kind)
		}
	}
	slices.Sort(kinds)
	return kinds
}
