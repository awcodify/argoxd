package tui

import (
	"strings"
	"testing"

	"github.com/awcodify/argoxd/internal/explorer"
)

func startModel(view string) (Model, *fakeSource) {
	source := &fakeSource{snapshot: storeSnapshot()}
	model := resize(New(source, "test", "argocd", explorer.Snapshot{}).WithStartView(view), 140, 30)
	return settle(model, source.loadCommand()), source
}

func TestTheStartViewOpensOnceTheFirstLoadArrives(t *testing.T) {
	for view, want := range map[string]explorer.Screen{
		"proj": explorer.ProjectsScreen, "clusters": explorer.ClustersScreen,
		"appset": explorer.ApplicationSetsScreen, "pulse": explorer.PulseScreen,
	} {
		if model, _ := startModel(view); model.explorer.Screen() != want {
			t.Errorf("start view %q opened screen %v, want %v", view, model.explorer.Screen(), want)
		}
	}
}

func TestTheStartViewCanScopeToAProject(t *testing.T) {
	model, _ := startModel("app store")

	if model.explorer.Screen() != explorer.ApplicationsScreen || model.explorer.Project() != "store" {
		t.Fatalf("screen %v, project %q; want the store Applications", model.explorer.Screen(), model.explorer.Project())
	}
	if view := model.View(); !strings.Contains(view, "applications · store") {
		t.Fatalf("the title does not show the project:\n%s", view)
	}
}

func TestTheStartViewIsNotAppliedAgainOnRefresh(t *testing.T) {
	model, source := startModel("proj")
	model = command(model, ":app")

	updated, _ := model.Update(loadedSnapshot{snapshot: source.snapshot, generation: model.sourceGeneration})

	if updated.(Model).explorer.Screen() != explorer.ApplicationsScreen {
		t.Fatal("a refresh sent the user back to the start view")
	}
}

func TestAnUnknownProjectInTheStartViewIsReported(t *testing.T) {
	model, _ := startModel("app nowhere")

	if model.err == nil || !strings.Contains(model.err.Error(), "nowhere") {
		t.Fatalf("err = %v, want the missing project named", model.err)
	}
}

func TestNoStartViewLeavesTheApplicationsList(t *testing.T) {
	if model, _ := startModel(""); model.explorer.Screen() != explorer.ApplicationsScreen {
		t.Fatalf("screen = %v, want Applications", model.explorer.Screen())
	}
}
