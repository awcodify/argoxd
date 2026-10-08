package explorer

import (
	"slices"
	"testing"
	"time"
)

func pulseSnapshot() Snapshot {
	return Snapshot{Applications: []Application{
		{Name: "fine", Sync: "Synced", Health: "Healthy"},
		{Name: "drifting", Sync: "OutOfSync", Health: "Healthy"},
		{Name: "degraded", Sync: "Synced", Health: "Degraded"},
		{Name: "missing", Sync: "OutOfSync", Health: "Missing"},
		{Name: "failed", Sync: "Synced", Health: "Healthy", Operation: &Operation{Phase: "Failed", FinishedAt: time.Now()}},
		{Name: "errored", Sync: "Synced", Health: "Healthy", Operation: &Operation{Phase: "Error"}},
		{Name: "succeeded", Sync: "Synced", Health: "Healthy", Operation: &Operation{Phase: "Succeeded"}},
		{Name: "warned", Sync: "Synced", Health: "Healthy", Conditions: []Condition{{Type: "SharedResourceWarning"}}},
		{Name: "worst", Sync: "OutOfSync", Health: "Degraded",
			Operation:  &Operation{Phase: "Failed"},
			Conditions: []Condition{{Type: "SyncError"}, {Type: "ComparisonError"}}},
		{Name: "unreported"},
	}}
}

func TestNeedingAttentionListsDegradedMissingFailedAndWarnedApplications(t *testing.T) {
	got := map[string][]string{}
	var order []string
	for _, item := range pulseSnapshot().NeedingAttention() {
		got[item.Application.Name] = item.Reasons
		order = append(order, item.Application.Name)
	}

	want := map[string][]string{
		"degraded": {"Degraded"},
		"missing":  {"Missing"},
		"failed":   {"sync Failed"},
		"errored":  {"sync Error"},
		"warned":   {"1 condition"},
		"worst":    {"Degraded", "sync Failed", "2 conditions"},
	}
	if len(got) != len(want) {
		t.Fatalf("applications needing attention = %v, want %d of them", order, len(want))
	}
	for name, reasons := range want {
		if !slices.Equal(got[name], reasons) {
			t.Fatalf("%s reasons = %q, want %q", name, got[name], reasons)
		}
	}
}

func TestNeedingAttentionPutsTheWorstFirstThenByName(t *testing.T) {
	var order []string
	for _, item := range pulseSnapshot().NeedingAttention() {
		order = append(order, item.Application.Name)
	}

	want := []string{"worst", "degraded", "errored", "failed", "missing", "warned"}
	if !slices.Equal(order, want) {
		t.Fatalf("order = %v, want %v", order, want)
	}
}

func TestCountsGroupApplicationsByHealthAndSyncStatus(t *testing.T) {
	snapshot := pulseSnapshot()

	health := snapshot.CountByHealth()
	sync := snapshot.CountBySync()

	if health["Healthy"] != 6 || health["Degraded"] != 2 || health["Missing"] != 1 || health["Unknown"] != 1 {
		t.Fatalf("health counts = %v", health)
	}
	if sync["Synced"] != 6 || sync["OutOfSync"] != 3 || sync["Unknown"] != 1 {
		t.Fatalf("sync counts = %v", sync)
	}
}

func TestPulseScreenSelectsAmongApplicationsNeedingAttention(t *testing.T) {
	model := NewModel(pulseSnapshot())

	model.SetScreen(PulseScreen)
	if model.RowCount() != 6 || model.SelectedName() != "worst" {
		t.Fatalf("rows = %d, selected = %q, want 6 and worst", model.RowCount(), model.SelectedName())
	}
	model.SetSearch("FAIL")
	if model.RowCount() != 1 || model.SelectedName() != "failed" {
		t.Fatalf("after searching FAIL: rows = %d, selected = %q, want 1 and failed", model.RowCount(), model.SelectedName())
	}
}

func TestPulseIgnoresTheProjectFilter(t *testing.T) {
	snapshot := pulseSnapshot()
	snapshot.Applications[2].Project = "store"
	model := NewModel(snapshot)
	model.SetProject("store")

	model.SetScreen(PulseScreen)

	if model.RowCount() != 6 {
		t.Fatalf("rows = %d, want every application needing attention", model.RowCount())
	}
}
