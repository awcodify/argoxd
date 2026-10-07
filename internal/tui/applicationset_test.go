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
