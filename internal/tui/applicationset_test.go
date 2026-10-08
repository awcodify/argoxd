package tui

import (
	"strings"
	"testing"

	"github.com/awcodify/argoxd/internal/explorer"
)

func applicationSetSnapshot() explorer.Snapshot {
	snapshot := storeSnapshot()
	snapshot.Applications[1].Owner = "store-services" // checkout
	snapshot.ApplicationSets = []explorer.ApplicationSet{
		{Name: "store-services", Generators: []string{"git", "matrix(list, clusters)"}},
		{Name: "broken", Generators: []string{"clusters"}, Problems: []explorer.Condition{{Type: "ErrorOccurred", Message: "no matching clusters"}}},
	}
	return snapshot
}

func TestApplicationSetCommandListsApplicationSets(t *testing.T) {
	for _, name := range []string{":appset", ":appsets", ":applicationset", ":applicationsets"} {
		model := resize(New(nil, "test", "argocd", applicationSetSnapshot()), 140, 30)

		view := command(model, name).View()

		for _, want := range []string{"applicationsets · 2", "store-services", "git, matrix(list, clusters)", "broken", "clusters"} {
			if !strings.Contains(view, want) {
				t.Fatalf("%s does not show %q:\n%s", name, want, view)
			}
		}
	}
}

func TestApplicationSetListCountsGeneratedApplicationsAndShowsProblems(t *testing.T) {
	model := resize(New(nil, "test", "argocd", applicationSetSnapshot()), 140, 30)

	lines := strings.Split(command(model, ":appset").View(), "\n")

	for _, line := range lines {
		switch {
		case strings.Contains(line, "store-services"):
			if !strings.Contains(line, " 1 ") || strings.Contains(line, conditionGlyph) {
				t.Fatalf("store-services should generate 1 application and have no problem: %q", line)
			}
		case strings.Contains(line, "broken"):
			for _, want := range []string{" 0 ", conditionGlyph + " ErrorOccurred", "no matching clusters"} {
				if !strings.Contains(line, want) {
					t.Fatalf("broken is missing %q: %q", want, line)
				}
			}
		}
	}
}

func TestEnterOnAnApplicationSetShowsTheApplicationsItGenerated(t *testing.T) {
	model := resize(New(nil, "test", "argocd", applicationSetSnapshot()), 140, 30)

	view := press(command(model, ":appset"), "enter").View()

	if !strings.Contains(view, "applications · appset/store-services · 1") || !strings.Contains(view, "checkout") || strings.Contains(view, "grafana") {
		t.Fatalf("enter did not list the applications store-services generated:\n%s", view)
	}
}

func TestZeroShowsEveryApplicationAfterOpeningAnApplicationSet(t *testing.T) {
	model := resize(New(nil, "test", "argocd", applicationSetSnapshot()), 140, 30)
	model = press(command(model, ":appset"), "enter")

	view := press(model, "0").View()

	if !strings.Contains(view, "applications · all · 2") {
		t.Fatalf("0 did not clear the application set filter:\n%s", view)
	}
}

func TestApplicationSetsWithoutAnyShowNothing(t *testing.T) {
	model := resize(New(nil, "test", "argocd", storeSnapshot()), 140, 30)

	view := command(model, ":appset").View()

	if !strings.Contains(view, "applicationsets · 0") || !strings.Contains(view, "Nothing to show yet.") {
		t.Fatalf("an empty application set list is not explained:\n%s", view)
	}
}

func TestApplicationSetsCanBeSearched(t *testing.T) {
	model := resize(New(nil, "test", "argocd", applicationSetSnapshot()), 140, 30)

	view := typeKeys(command(model, ":appset"), "/", "b", "r", "o", "enter").View()

	if !strings.Contains(view, "broken") || strings.Contains(view, "store-services") {
		t.Fatalf("the search did not narrow the application sets:\n%s", view)
	}
}

// openFirstApplicationSet shows the ApplicationSets and opens the one at the
// given position, which lists the Applications it generated.
func openApplicationSetAt(model Model, moves int) Model {
	model = command(model, ":appset")
	for range moves {
		model = press(model, "j")
	}
	return press(model, "enter")
}

func TestEscFromAnApplicationSetsApplicationsReturnsToTheSameApplicationSet(t *testing.T) {
	model := resize(New(nil, "test", "argocd", applicationSetSnapshot()), 140, 30)
	opened := openApplicationSetAt(model, 1) // broken, the second one
	if !strings.Contains(opened.View(), "appset/broken") {
		t.Fatalf("broken's applications did not open:\n%s", opened.View())
	}

	back := press(opened, "esc")

	if back.Explorer().Screen() != explorer.ApplicationSetsScreen || back.Explorer().SelectedName() != "broken" {
		t.Fatalf("esc went to screen %v with %q selected, want the application sets with broken selected:\n%s", back.Explorer().Screen(), back.Explorer().SelectedName(), back.View())
	}
	if back.Explorer().Owner() != "" || !strings.Contains(back.View(), "applicationsets · 2") {
		t.Fatalf("the application sets list should be whole again:\n%s", back.View())
	}
}

func TestEscClearsTheSearchBeforeLeavingAnApplicationSetsApplications(t *testing.T) {
	model := resize(New(nil, "test", "argocd", applicationSetSnapshot()), 140, 30)
	opened := typeKeys(openApplicationSetAt(model, 0), "/", "c", "h", "enter")
	if opened.Explorer().Search() != "ch" {
		t.Fatalf("search = %q, want ch", opened.Explorer().Search())
	}

	cleared := press(opened, "esc")
	if cleared.Explorer().Screen() != explorer.ApplicationsScreen || cleared.Explorer().Search() != "" {
		t.Fatalf("the first esc should only clear the search:\n%s", cleared.View())
	}
	if back := press(cleared, "esc"); back.Explorer().Screen() != explorer.ApplicationSetsScreen {
		t.Fatalf("the second esc should leave for the application sets:\n%s", back.View())
	}
}

func TestEscFromAnApplicationOpenedFromAnApplicationSetStepsBackOneLevelAtATime(t *testing.T) {
	source := &fakeSource{snapshot: applicationSetSnapshot(), tree: checkoutTree()}
	model := resize(New(source, "test", "argocd", explorer.Snapshot{}), 140, 40)
	model = command(settle(model, source.loadCommand()), ":appset")
	apps := press(model, "enter") // store-services: checkout
	tree := run(apps, "enter")
	if !strings.Contains(tree.View(), "applications › checkout") {
		t.Fatalf("checkout did not open:\n%s", tree.View())
	}

	list := press(tree, "esc")
	if list.Explorer().Screen() != explorer.ApplicationsScreen || !strings.Contains(list.View(), "appset/store-services") {
		t.Fatalf("the first esc should return to the application set's applications:\n%s", list.View())
	}
	if back := press(list, "esc"); back.Explorer().Screen() != explorer.ApplicationSetsScreen {
		t.Fatalf("the second esc should return to the application sets:\n%s", back.View())
	}
}

func TestEscOnApplicationsOpenedDirectlyStaysOnTheList(t *testing.T) {
	model := resize(New(nil, "test", "argocd", applicationSetSnapshot()), 140, 30)

	still := press(command(model, ":app"), "esc")

	if still.Explorer().Screen() != explorer.ApplicationsScreen {
		t.Fatalf("esc left the applications list that was opened directly:\n%s", still.View())
	}
}

func TestLeavingForAnotherViewForgetsWhereTheApplicationsCameFrom(t *testing.T) {
	model := resize(New(nil, "test", "argocd", applicationSetSnapshot()), 140, 30)
	opened := openApplicationSetAt(model, 0)

	elsewhere := press(command(command(opened, ":proj"), ":app"), "esc")

	if elsewhere.Explorer().Screen() != explorer.ApplicationsScreen {
		t.Fatalf("esc went back to the application sets after visiting other views:\n%s", elsewhere.View())
	}
}

func TestEscFromAProjectsApplicationsReturnsToTheSameProject(t *testing.T) {
	model := resize(New(nil, "test", "argocd", storeSnapshot()), 140, 30)
	model = press(command(model, ":projects"), "j") // store, the second project

	back := press(press(model, "enter"), "esc")

	if back.Explorer().Screen() != explorer.ProjectsScreen || back.Explorer().SelectedName() != "store" {
		t.Fatalf("esc went to screen %v with %q selected, want the projects with store selected:\n%s", back.Explorer().Screen(), back.Explorer().SelectedName(), back.View())
	}
}

func TestApplicationsOpenedFromAnApplicationSetHintAtEsc(t *testing.T) {
	model := resize(New(nil, "test", "argocd", applicationSetSnapshot()), 140, 30)

	if view := command(model, ":app").View(); strings.Contains(view, "Back") {
		t.Fatalf("a list opened directly has nowhere to go back to:\n%s", view)
	}
	if view := openApplicationSetAt(model, 0).View(); !strings.Contains(view, "Back") {
		t.Fatalf("applications opened from an application set should hint at esc:\n%s", view)
	}
}
