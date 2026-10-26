package tui

import (
	"strings"
	"testing"

	"github.com/awcodify/argoxd/internal/explorer"
)

func applicationOrder(model Model) []string {
	var names []string
	for _, application := range model.explorer.Applications() {
		names = append(names, application.Name)
	}
	return names
}

func sortModel() (Model, *fakeSource) {
	source := &fakeSource{snapshot: explorer.Snapshot{
		Projects: []explorer.Project{{Name: "store"}},
		Applications: []explorer.Application{
			{Name: "grafana", Project: "store", Sync: "Synced", Health: "Healthy"},
			{Name: "checkout", Project: "store", Sync: "OutOfSync", Health: "Degraded"},
			{Name: "cart", Project: "store", Sync: "Synced", Health: "Progressing"},
		},
	}}
	return resize(settle(New(source, "test", "argocd", explorer.Snapshot{}), source.loadCommand()), 140, 30), source
}

func TestSortCommandOrdersTheApplicationsAndSaysSo(t *testing.T) {
	model, _ := sortModel()

	model = command(model, ":sort health")
	if got := applicationOrder(model); strings.Join(got, ",") != "checkout,cart,grafana" {
		t.Fatalf("order = %v, want the worst health first", got)
	}
	if view := model.View(); !strings.Contains(view, "sorted by health") {
		t.Fatalf("the title does not say how the list is sorted:\n%s", view)
	}

	model = command(model, ":sort name reverse")
	if got := applicationOrder(model); strings.Join(got, ",") != "grafana,checkout,cart" {
		t.Fatalf("order = %v, want names Z to A", got)
	}
	if view := model.View(); !strings.Contains(view, "sorted by name") || !strings.Contains(view, "reverse") {
		t.Fatalf("the title does not say the order is reversed:\n%s", view)
	}
}

func TestSortCommandWithoutAFieldGoesBackToTheSourceOrder(t *testing.T) {
	model, _ := sortModel()
	model = command(command(model, ":sort name"), ":sort")

	if got := applicationOrder(model); strings.Join(got, ",") != "grafana,checkout,cart" {
		t.Fatalf("order = %v, want the source order", got)
	}
	if view := model.View(); strings.Contains(view, "sorted by") {
		t.Fatalf("the title still says the list is sorted:\n%s", view)
	}
}

func TestSortCommandRejectsUnknownFieldsAndWordsAndNamesWhatWorks(t *testing.T) {
	model, _ := sortModel()

	for _, input := range []string{":sort revision", ":sort name sideways"} {
		bad := command(model, input)
		if bad.err == nil || !strings.Contains(bad.err.Error(), "name, health or sync") {
			t.Fatalf("%s: err = %v, want a message listing name, health or sync", input, bad.err)
		}
		if got := applicationOrder(bad); strings.Join(got, ",") != "grafana,checkout,cart" {
			t.Fatalf("%s changed the order: %v", input, got)
		}
	}
}

func TestSortSurvivesSwitchingNamespace(t *testing.T) {
	source := &namespaceSource{
		fakeSource: &fakeSource{snapshot: sortSnapshot()},
		namespaces: []string{"argocd", "gitops"},
	}
	model := resize(settle(New(source, "test", "argocd", explorer.Snapshot{}), source.loadCommand()), 140, 30)
	model = command(model, ":sort name")

	model = settle(commandSettled(model, ":ns gitops"), source.loadCommand())

	if got := applicationOrder(model); strings.Join(got, ",") != "cart,checkout,grafana" {
		t.Fatalf("order after :ns = %v, want it still sorted by name", got)
	}
}

func sortSnapshot() explorer.Snapshot {
	snapshot, _ := sortModel()
	return snapshot.explorer.Snapshot()
}

func TestSortCommandIsSuggested(t *testing.T) {
	model, _ := sortModel()

	if view := typeKeys(model, ":", "s", "o").View(); !strings.Contains(view, "sort") {
		t.Fatalf(":so did not suggest sort:\n%s", view)
	}
	if view := typeKeys(model, ":", "s", "o", "r", "t", " ").View(); !strings.Contains(view, "sort health") || !strings.Contains(view, "sort sync") {
		t.Fatalf(":sort did not suggest its fields:\n%s", view)
	}
}

func TestWithSortStartsTheListSorted(t *testing.T) {
	source := &fakeSource{snapshot: sortSnapshot()}
	model := New(source, "test", "argocd", explorer.Snapshot{}).WithSort(explorer.SortHealth, false)

	model = settle(model, source.loadCommand())

	if got := applicationOrder(model); strings.Join(got, ",") != "checkout,cart,grafana" {
		t.Fatalf("order = %v, want it sorted from the start", got)
	}
}
