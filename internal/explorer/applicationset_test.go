package explorer

import "testing"

func applicationSetSnapshot() Snapshot {
	return Snapshot{
		Projects: []Project{{Name: "store"}, {Name: "platform"}},
		Applications: []Application{
			{Name: "cart", Project: "store", Owner: "store-services"},
			{Name: "checkout", Project: "store", Owner: "store-services"},
			{Name: "grafana", Project: "platform", Owner: "addons"},
			{Name: "standalone", Project: "store"},
		},
		ApplicationSets: []ApplicationSet{
			{Name: "store-services", Generators: []string{"git"}},
			{Name: "addons", Generators: []string{"clusters"}},
			{Name: "empty"},
		},
	}
}

func names(applications []Application) []string {
	var names []string
	for _, application := range applications {
		names = append(names, application.Name)
	}
	return names
}

func TestApplicationSetsScreenListsAndSearchesApplicationSets(t *testing.T) {
	model := NewModel(applicationSetSnapshot())

	model.SetScreen(ApplicationSetsScreen)
	if model.RowCount() != 3 || model.SelectedName() != "store-services" {
		t.Fatalf("rows = %d, selected = %q, want 3 and store-services", model.RowCount(), model.SelectedName())
	}

	model.SetSearch("ADD")
	if sets := model.ApplicationSets(); len(sets) != 1 || sets[0].Name != "addons" {
		t.Fatalf("application sets matching ADD = %+v, want addons", sets)
	}
}

func TestGeneratedApplicationsCountsTheApplicationsAnApplicationSetOwns(t *testing.T) {
	model := NewModel(applicationSetSnapshot())

	for name, want := range map[string]int{"store-services": 2, "addons": 1, "empty": 0, "missing": 0} {
		if got := model.GeneratedApplications(name); got != want {
			t.Fatalf("GeneratedApplications(%q) = %d, want %d", name, got, want)
		}
	}
}

func TestSetOwnerShowsOnlyTheApplicationsAnApplicationSetGenerated(t *testing.T) {
	model := NewModel(applicationSetSnapshot())

	model.SetOwner("store-services")

	got := names(model.Applications())
	if len(got) != 2 || got[0] != "cart" || got[1] != "checkout" {
		t.Fatalf("applications = %v, want cart and checkout", got)
	}
	if model.Owner() != "store-services" {
		t.Fatalf("owner = %q, want store-services", model.Owner())
	}
}

func TestOwnerAndProjectFiltersReplaceEachOther(t *testing.T) {
	model := NewModel(applicationSetSnapshot())

	model.SetProject("platform")
	model.SetOwner("store-services")
	if model.Project() != "" || len(model.Applications()) != 2 {
		t.Fatalf("project = %q, applications = %v, want the owner filter alone", model.Project(), names(model.Applications()))
	}

	model.SetProject("platform")
	if model.Owner() != "" || len(model.Applications()) != 1 {
		t.Fatalf("owner = %q, applications = %v, want the project filter alone", model.Owner(), names(model.Applications()))
	}

	model.SetOwner("addons")
	model.SetProject("")
	if model.Owner() != "" || len(model.Applications()) != 4 {
		t.Fatalf("clearing the project left owner = %q and %d applications, want all 4", model.Owner(), len(model.Applications()))
	}
}
