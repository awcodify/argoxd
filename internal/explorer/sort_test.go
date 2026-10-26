package explorer

import (
	"slices"
	"testing"
)

func sortSnapshot() Snapshot {
	return Snapshot{Applications: []Application{
		{Name: "gamma", Project: "store", Sync: "Synced", Health: "Healthy"},
		{Name: "alpha", Project: "store", Sync: "OutOfSync", Health: "Progressing"},
		{Name: "delta", Project: "platform", Sync: "Synced", Health: "Degraded"},
		{Name: "beta", Project: "store", Sync: "Unknown", Health: "Missing"},
		{Name: "epsilon", Project: "store", Sync: "OutOfSync", Health: "Degraded"},
	}}
}

func applicationNames(model Model) []string {
	var names []string
	for _, application := range model.Applications() {
		names = append(names, application.Name)
	}
	return names
}

func TestApplicationsKeepTheSourceOrderUntilSorted(t *testing.T) {
	model := NewModel(sortSnapshot())

	if got, want := applicationNames(model), []string{"gamma", "alpha", "delta", "beta", "epsilon"}; !slices.Equal(got, want) {
		t.Fatalf("applications = %v, want the source order %v", got, want)
	}
}

func TestApplicationsSortByNameHealthOrSync(t *testing.T) {
	tests := []struct {
		field   SortField
		reverse bool
		want    []string
	}{
		{SortName, false, []string{"alpha", "beta", "delta", "epsilon", "gamma"}},
		{SortName, true, []string{"gamma", "epsilon", "delta", "beta", "alpha"}},
		// Worst first: Degraded, Missing, Progressing, then Healthy; ties by name.
		{SortHealth, false, []string{"delta", "epsilon", "beta", "alpha", "gamma"}},
		{SortHealth, true, []string{"gamma", "alpha", "beta", "epsilon", "delta"}},
		// OutOfSync first, then Unknown, then Synced.
		{SortSync, false, []string{"alpha", "epsilon", "beta", "delta", "gamma"}},
		{SortSync, true, []string{"gamma", "delta", "beta", "epsilon", "alpha"}},
	}
	for _, test := range tests {
		model := NewModel(sortSnapshot())
		model.SetSort(test.field, test.reverse)

		if got := applicationNames(model); !slices.Equal(got, test.want) {
			t.Errorf("sort %s reverse=%v: %v, want %v", test.field, test.reverse, got, test.want)
		}
	}
}

func TestSortingKeepsTheSelectedApplicationSelected(t *testing.T) {
	model := NewModel(sortSnapshot())
	model.MoveDown() // alpha
	if model.SelectedName() != "alpha" {
		t.Fatalf("selected = %q, want alpha", model.SelectedName())
	}

	model.SetSort(SortHealth, false)

	if model.SelectedName() != "alpha" {
		t.Fatalf("sorting moved the selection to %q, want alpha to stay selected", model.SelectedName())
	}
}

func TestSortAppliesToSearchesFiltersAndNewSnapshots(t *testing.T) {
	model := NewModel(sortSnapshot())
	model.SetSort(SortName, false)
	model.SetProject("store")
	if got, want := applicationNames(model), []string{"alpha", "beta", "epsilon", "gamma"}; !slices.Equal(got, want) {
		t.Fatalf("sorted within a project: %v, want %v", got, want)
	}

	model.ReplaceSnapshot(Snapshot{Applications: []Application{{Name: "zulu", Project: "store"}, {Name: "bravo", Project: "store"}}})

	if got, want := applicationNames(model), []string{"bravo", "zulu"}; !slices.Equal(got, want) {
		t.Fatalf("after a reload: %v, want the sort kept: %v", got, want)
	}
	if field, reverse := model.Sort(); field != SortName || reverse {
		t.Fatalf("Sort() = %q, %v", field, reverse)
	}
}

func TestParseSortField(t *testing.T) {
	for input, want := range map[string]SortField{"name": SortName, "HEALTH": SortHealth, "sync": SortSync, "": SortDefault} {
		if got, ok := ParseSortField(input); !ok || got != want {
			t.Errorf("ParseSortField(%q) = %q, %v; want %q", input, got, ok, want)
		}
	}
	if _, ok := ParseSortField("revision"); ok {
		t.Error("ParseSortField accepted revision")
	}
}
