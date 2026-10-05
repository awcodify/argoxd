package explorer

import "testing"

func TestModelDefaultsToApplications(t *testing.T) {
	model := NewModel(Snapshot{
		Applications: []Application{{Name: "payments", Namespace: "argocd"}},
		Projects:     []Project{{Name: "default"}},
		Clusters:     []Cluster{{Name: "in-cluster", Server: "https://kubernetes.default.svc"}},
	})

	if model.Screen() != ApplicationsScreen {
		t.Fatalf("screen = %v, want %v", model.Screen(), ApplicationsScreen)
	}
	if model.RowCount() != 1 {
		t.Fatalf("row count = %d, want 1", model.RowCount())
	}
	if got := model.SelectedName(); got != "payments" {
		t.Fatalf("selected name = %q, want %q", got, "payments")
	}
}

func TestModelNavigationIsBoundedPerScreen(t *testing.T) {
	model := NewModel(Snapshot{
		Applications: []Application{
			{Name: "payments"},
			{Name: "catalog"},
		},
		Projects: []Project{{Name: "platform"}},
	})

	model.MoveDown()
	model.MoveDown()
	if got := model.Cursor(); got != 1 {
		t.Fatalf("cursor = %d, want 1", got)
	}
	if got := model.SelectedName(); got != "catalog" {
		t.Fatalf("selected after moving down = %q, want %q", got, "catalog")
	}

	model.SetScreen(ProjectsScreen)
	if got := model.SelectedName(); got != "platform" {
		t.Fatalf("selected project = %q, want %q", got, "platform")
	}

	model.MoveUp()
	if got := model.SelectedName(); got != "platform" {
		t.Fatalf("selected after moving above top = %q, want %q", got, "platform")
	}
}

func TestModelExposesAllExplorerScreens(t *testing.T) {
	model := NewModel(Snapshot{})

	for _, screen := range []Screen{ApplicationsScreen, ProjectsScreen, ClustersScreen, SettingsScreen} {
		model.SetScreen(screen)
		if model.Screen() != screen {
			t.Fatalf("screen = %v, want %v", model.Screen(), screen)
		}
	}
}

func TestProjectFilterNarrowsApplications(t *testing.T) {
	model := NewModel(Snapshot{Applications: []Application{
		{Name: "grafana", Project: "platform"},
		{Name: "checkout", Project: "store"},
		{Name: "payments", Project: "store"},
	}})
	model.MoveDown()

	model.SetProject("store")

	if model.Project() != "store" {
		t.Fatalf("project = %q, want store", model.Project())
	}
	if model.RowCount() != 2 || model.SelectedName() != "checkout" {
		t.Fatalf("rows = %d selected = %q, want 2 rows with checkout selected", model.RowCount(), model.SelectedName())
	}

	model.SetProject("")
	if got := len(model.Applications()); got != 3 {
		t.Fatalf("applications without a filter = %d, want 3", got)
	}
}

func TestReplaceSnapshotKeepsTheSelectedResource(t *testing.T) {
	model := NewModel(Snapshot{Applications: []Application{{Name: "catalog"}, {Name: "payments"}}})
	model.MoveDown()

	model.ReplaceSnapshot(Snapshot{Applications: []Application{{Name: "accounts"}, {Name: "catalog"}, {Name: "payments"}}})

	if got := model.SelectedName(); got != "payments" {
		t.Fatalf("selected after refresh = %q, want payments", got)
	}
}

func TestReplaceSnapshotClampsSelectionWhenResourceDisappears(t *testing.T) {
	model := NewModel(Snapshot{Applications: []Application{{Name: "catalog"}, {Name: "payments"}}})
	model.MoveDown()

	model.ReplaceSnapshot(Snapshot{Applications: []Application{{Name: "catalog"}}})

	if got := model.SelectedName(); got != "catalog" {
		t.Fatalf("selected after refresh = %q, want catalog", got)
	}
}

func TestSearchMatchesNamesOnEveryScreen(t *testing.T) {
	model := NewModel(Snapshot{
		Applications: []Application{{Name: "payments-api"}, {Name: "catalog"}, {Name: "payments-worker"}},
		Projects:     []Project{{Name: "store"}, {Name: "platform"}},
		Clusters:     []Cluster{{Name: "production"}, {Name: "staging"}},
	})

	model.SetSearch("PAY")
	if model.RowCount() != 2 || model.SelectedName() != "payments-api" {
		t.Fatalf("searched applications: rows = %d selected = %q", model.RowCount(), model.SelectedName())
	}

	model.SetScreen(ProjectsScreen)
	if model.Search() != "" {
		t.Fatalf("switching screens kept search %q", model.Search())
	}
	model.SetSearch("plat")
	if got := model.Projects(); len(got) != 1 || got[0].Name != "platform" {
		t.Fatalf("searched projects = %+v", got)
	}

	model.SetScreen(ClustersScreen)
	model.SetSearch("stag")
	if got := model.Clusters(); len(got) != 1 || got[0].Name != "staging" {
		t.Fatalf("searched clusters = %+v", got)
	}
}

func TestFilterNarrowsApplicationsByHealthAndSync(t *testing.T) {
	model := NewModel(Snapshot{Applications: []Application{
		{Name: "api", Health: "Healthy", Sync: "Synced"},
		{Name: "worker", Health: "Degraded", Sync: "OutOfSync"},
		{Name: "cron", Health: "Healthy", Sync: "OutOfSync"},
	}})

	model.SetFilter(Filter{Health: "healthy"})
	if got := model.RowCount(); got != 2 {
		t.Fatalf("healthy applications = %d, want 2", got)
	}
	model.SetFilter(Filter{Health: "Healthy", Sync: "OutOfSync"})
	if model.RowCount() != 1 || model.SelectedName() != "cron" {
		t.Fatalf("healthy and out of sync: rows = %d selected = %q", model.RowCount(), model.SelectedName())
	}
	model.SetSearch("api")
	if got := model.RowCount(); got != 0 {
		t.Fatalf("search and filter apply together: rows = %d, want 0", got)
	}

	model.SetScreen(ApplicationsScreen)
	if model.Filter().Active() {
		t.Fatalf("switching screens kept filter %+v", model.Filter())
	}
}

func TestFilterWithSetsOneFieldCaseInsensitively(t *testing.T) {
	filter, err := Filter{Sync: "Synced"}.With("health", "degraded", nil)
	if want := (Filter{Health: "Degraded", Sync: "Synced"}); err != nil || filter != want {
		t.Fatalf("filter = %+v, %v; want %+v", filter, err, want)
	}
	if filter, err = filter.With("health", "", nil); err != nil || filter != (Filter{Sync: "Synced"}) {
		t.Fatalf("empty value = %+v, %v; want the health field cleared", filter, err)
	}
	if filter, err = filter.With("kind", "pod", []string{"Pod"}); err != nil || filter.Kind != "Pod" {
		t.Fatalf("kind = %+v, %v; want Pod", filter, err)
	}
}

func TestFilterWithRejectsUnknownValues(t *testing.T) {
	for key, value := range map[string]string{"health": "sick", "sync": "late", "kind": "Pod"} {
		if _, err := (Filter{}).With(key, value, nil); err == nil {
			t.Errorf("With(%q, %q) succeeded, want an error", key, value)
		}
	}
}
