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
	if !strings.Contains(model.status, "Sorted by name (reverse)") {
		t.Fatalf("status = %q, want it to say the order is reversed", model.status)
	}
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

func TestTheSortedColumnShowsWhichWayItIsOrdered(t *testing.T) {
	model, _ := sortModel()
	header := func(model Model) string {
		for _, line := range strings.Split(model.View(), "\n") {
			if strings.Contains(line, "NAME") && strings.Contains(line, "PROJECT") {
				return line
			}
		}
		t.Fatalf("no table header:\n%s", model.View())
		return ""
	}

	if plain := header(model); strings.Contains(plain, "▲") || strings.Contains(plain, "▼") {
		t.Fatalf("an unsorted list marks a column: %q", plain)
	}
	for input, column := range map[string]string{":sort name": "NAME ▲", ":sort health": "HEALTH ▲", ":sort sync": "SYNC ▲"} {
		if got := header(command(model, input)); !strings.Contains(got, column) {
			t.Errorf("%s: header = %q, want %q", input, got, column)
		}
	}
	if got := header(command(model, ":sort sync reverse")); !strings.Contains(got, "SYNC ▼") {
		t.Errorf("reverse: header = %q, want SYNC ▼", got)
	}
	if got := header(command(command(model, ":sort sync"), ":sort")); strings.Contains(got, "▲") {
		t.Errorf("after :sort the column is still marked: %q", got)
	}
}

func TestSortingByTheSameFieldAgainFlipsTheDirection(t *testing.T) {
	model, _ := sortModel()

	model = command(model, ":sort name")
	if got := strings.Join(applicationOrder(model), ","); got != "cart,checkout,grafana" {
		t.Fatalf("first :sort name = %s, want A to Z", got)
	}
	model = command(model, ":sort name")
	if got := strings.Join(applicationOrder(model), ","); got != "grafana,checkout,cart" {
		t.Fatalf("second :sort name = %s, want it flipped to Z to A", got)
	}
	if !strings.Contains(model.status, "Sorted by name (reverse)") || !strings.Contains(model.View(), "NAME ▼") {
		t.Fatalf("the flip is not shown: status %q\n%s", model.status, model.View())
	}
	model = command(model, ":sort name")
	if got := strings.Join(applicationOrder(model), ","); got != "cart,checkout,grafana" {
		t.Fatalf("third :sort name = %s, want it flipped back to A to Z", got)
	}
}

func TestSortingByAnotherFieldStartsInTheNormalDirection(t *testing.T) {
	model, _ := sortModel()
	model = command(command(model, ":sort name"), ":sort name") // now reversed

	model = command(model, ":sort health")

	if field, reverse := model.explorer.Sort(); field != explorer.SortHealth || reverse {
		t.Fatalf("sort = %q reverse=%v, want health in its normal direction", field, reverse)
	}
}

func TestSortReverseIsExplicitAndDoesNotFlip(t *testing.T) {
	model, _ := sortModel()

	model = command(command(model, ":sort health reverse"), ":sort health reverse")

	if _, reverse := model.explorer.Sort(); !reverse {
		t.Fatal("repeating :sort health reverse flipped it back; an explicit reverse must stay reversed")
	}
}

func TestSortFlipStartsFromTheConfiguredDefault(t *testing.T) {
	source := &fakeSource{snapshot: sortSnapshot()}
	model := settle(New(source, "test", "argocd", explorer.Snapshot{}).WithSort(explorer.SortHealth, true), source.loadCommand())

	model = command(model, ":sort health")

	if _, reverse := model.explorer.Sort(); reverse {
		t.Fatal("the configured reverse order was kept; :sort health should flip it to the normal direction")
	}
}

func TestSortAloneStillRestoresTheSourceOrderAfterFlips(t *testing.T) {
	model, _ := sortModel()
	model = command(command(command(model, ":sort name"), ":sort name"), ":sort")

	if field, _ := model.explorer.Sort(); field != explorer.SortDefault {
		t.Fatalf("sort = %q after :sort, want the source order", field)
	}
}
