package tui

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/awcodify/argoxd/internal/argocd"
	"github.com/awcodify/argoxd/internal/explorer"
)

var _ argocd.ResourceSyncer = (*fakeSource)(nil)

func (f *fakeSource) SyncResources(_ context.Context, application string, resources []explorer.ResourceReference, _ argocd.SyncOptions) error {
	f.syncedApps = append(f.syncedApps, application)
	f.syncedResources = append(f.syncedResources, resources)
	return f.syncErrors[application]
}

// openApplications loads the two sample Applications, grafana then checkout.
func openApplications(t *testing.T, source *fakeSource) Model {
	t.Helper()
	model := resize(New(source, "test", "argocd", explorer.Snapshot{}), 140, 40)
	return settle(model, source.loadCommand())
}

func TestSpaceMarksApplicationsAndSyncSyncsThemAll(t *testing.T) {
	source := &fakeSource{snapshot: storeSnapshot()}
	model := openApplications(t, source)

	model = typeKeys(model, " ", " ")
	if view := model.View(); strings.Count(view, "●") < 2 {
		t.Fatalf("marked applications are not shown:\n%s", view)
	}
	model = press(model, "s")
	if view := model.View(); !strings.Contains(view, "Sync 2 applications?") {
		t.Fatalf("the dialog does not name the selection:\n%s", view)
	}
	model = run(model, "enter")

	slices.Sort(source.syncedApps)
	if !slices.Equal(source.syncedApps, []string{"checkout", "grafana"}) {
		t.Fatalf("synced = %v, want both", source.syncedApps)
	}
	if view := model.View(); !strings.Contains(view, "sync requested for 2 applications") {
		t.Fatalf("the result is not reported:\n%s", view)
	}
	// The status line starts with a ● of its own.
	if view := model.View(); strings.Count(view, "●") > 1 {
		t.Fatalf("marks outlived the sync:\n%s", view)
	}
}

func TestSpaceTogglesAndSyncUsesTheDialogOptions(t *testing.T) {
	source := &fakeSource{snapshot: storeSnapshot()}
	model := openApplications(t, source)

	// Mark grafana, then step back and unmark it again: nothing is marked.
	model = typeKeys(model, " ", "k", " ", "k")
	model = run(typeKeys(model, "s", "p"), "enter")

	if !slices.Equal(source.syncedApps, []string{"grafana"}) || !source.synced[0].Prune {
		t.Fatalf("synced %v with %+v, want only the selected one, pruning", source.syncedApps, source.synced)
	}
}

func TestEscClearsTheMarksBeforeAnythingElse(t *testing.T) {
	source := &fakeSource{snapshot: storeSnapshot()}
	model := typeKeys(openApplications(t, source), " ")

	model = press(model, "esc")
	model = run(typeKeys(model, "s"), "enter")

	if len(source.syncedApps) != 1 {
		t.Fatalf("synced = %v, want only the selected Application once the marks are cleared", source.syncedApps)
	}
}

// The Applications that did sync show up on the next refresh.
func TestSyncingSeveralReportsTheOnesThatFailed(t *testing.T) {
	source := &fakeSource{snapshot: storeSnapshot(), syncErrors: map[string]error{"checkout": errors.New("denied")}}
	model := openApplications(t, source)

	model = run(typeKeys(model, " ", " ", "s"), "enter")

	view := model.View()
	if !strings.Contains(view, "1 of 2") || !strings.Contains(view, "checkout") || !strings.Contains(view, "denied") {
		t.Fatalf("the failure is not reported:\n%s", view)
	}
}

func TestSpaceMarksCardsAndSyncSyncsOnlyThoseResources(t *testing.T) {
	source := &fakeSource{snapshot: storeSnapshot(), tree: checkoutTree()}
	model := openCheckout(t, source)

	model = typeKeys(model, "j", " ", " ")
	if view := model.View(); strings.Count(view, "●") < 2 {
		t.Fatalf("marked cards are not shown:\n%s", view)
	}
	model = press(model, "s")
	if view := model.View(); !strings.Contains(view, "Sync 2 resources of checkout?") {
		t.Fatalf("the dialog does not name the selection:\n%s", view)
	}
	model = run(model, "enter")

	if len(source.syncedResources) != 1 || len(source.syncedResources[0]) != 2 {
		t.Fatalf("synced resources = %v, want one sync of two", source.syncedResources)
	}
	got := source.syncedResources[0]
	if got[0].Kind != "Deployment" || got[1].Kind != "Pod" || len(source.synced) != 0 {
		t.Fatalf("synced %v and full syncs %v", got, source.synced)
	}
	if view := model.View(); strings.Count(view, "●") > 1 {
		t.Fatalf("marks outlived the sync:\n%s", view)
	}
}

func TestTheApplicationCardCannotBeMarked(t *testing.T) {
	source := &fakeSource{snapshot: storeSnapshot(), tree: checkoutTree()}
	model := press(openCheckout(t, source), " ")

	if view := model.View(); !strings.Contains(view, "Select a resource card") {
		t.Fatalf("marking the Application card was not refused:\n%s", view)
	}
	// Space moved nothing; s still syncs the whole Application.
	model = run(press(model, "s"), "enter")
	if len(source.synced) != 1 || len(source.syncedResources) != 0 {
		t.Fatalf("synced %v and resources %v, want one full sync", source.synced, source.syncedResources)
	}
}

func TestLeavingTheDependenciesForgetsTheMarkedCards(t *testing.T) {
	source := &fakeSource{snapshot: storeSnapshot(), tree: checkoutTree()}
	model := typeKeys(openCheckout(t, source), "j", " ")

	model = command(press(model, ":"), "app")
	model = run(model, "enter")
	model = run(press(model, "s"), "enter")

	if len(source.syncedResources) != 0 || len(source.synced) != 1 {
		t.Fatalf("a mark survived leaving the view: %v", source.syncedResources)
	}
}

func TestSpaceStillExpandsInventoryNodes(t *testing.T) {
	model := resize(New(nil, "test", "argocd", storeSnapshot()), 140, 40)
	model = press(model, "t")
	before := model.View()

	after := typeKeys(model, " ").View()

	if before == after {
		t.Fatalf("space no longer collapses the selected inventory node:\n%s", after)
	}
}
