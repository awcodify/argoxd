package tui

import (
	"errors"
	"strings"
	"testing"

	"github.com/awcodify/argoxd/internal/explorer"
)

func appOfAppsSnapshot() explorer.Snapshot {
	return explorer.Snapshot{Applications: []explorer.Application{
		{Name: "platform-root", Project: "platform", Sync: "Synced", Health: "Healthy"},
		{Name: "payments", Project: "payments", Sync: "OutOfSync", Health: "Degraded"},
	}}
}

// appOfAppsSource has a root Application that deploys the payments Application
// and a ConfigMap.
func appOfAppsSource() *fakeSource {
	child := explorer.ResourceNode{Group: "argoproj.io", Version: "v1alpha1", Kind: "Application", Namespace: "argocd", Name: "payments", Sync: "OutOfSync", Health: "Degraded"}
	return &fakeSource{
		snapshot: appOfAppsSnapshot(),
		trees: map[string]explorer.ResourceTree{
			"platform-root": {Application: "platform-root", Nodes: []explorer.ResourceNode{
				child,
				{Version: "v1", Kind: "ConfigMap", Namespace: "argocd", Name: "settings", Sync: "Synced"},
			}},
			"payments": {Application: "payments", Nodes: []explorer.ResourceNode{
				{Group: "apps", Version: "v1", Kind: "Deployment", Namespace: "pay", Name: "ledger", Health: "Degraded", Sync: "OutOfSync"},
			}},
		},
	}
}

// openRoot opens platform-root, whose cards are the Application itself, the
// payments Application and the ConfigMap.
func openRoot(t *testing.T, source *fakeSource) Model {
	t.Helper()
	model := resize(New(source, "test", "argocd", explorer.Snapshot{}), 140, 40)
	model = settle(model, source.loadCommand())
	model = run(model, "enter")
	if !strings.Contains(model.View(), "applications › platform-root") {
		t.Fatalf("platform-root did not open:\n%s", model.View())
	}
	return model
}

func TestEnterOnAChildApplicationOpensItsDependencies(t *testing.T) {
	model := run(press(openRoot(t, appOfAppsSource()), "j"), "enter")

	view := model.View()

	for _, want := range []string{"applications › platform-root › payments", "Deployment", "ledger"} {
		if !strings.Contains(view, want) {
			t.Fatalf("the child's dependencies do not show %q:\n%s", want, view)
		}
	}
}

func TestEscReturnsFromAChildApplicationToItsParent(t *testing.T) {
	model := run(press(openRoot(t, appOfAppsSource()), "j"), "enter")

	back := press(model, "esc")

	view := back.View()
	if !strings.Contains(view, "applications › platform-root") || strings.Contains(view, "› payments") || strings.Contains(view, "ledger") {
		t.Fatalf("esc did not return to platform-root:\n%s", view)
	}
	if back.treeCursor != 1 {
		t.Fatalf("cursor = %d, want the child's card selected again", back.treeCursor)
	}
	if list := press(back, "esc").View(); !strings.Contains(list, "applications · all · 2") {
		t.Fatalf("a second esc did not return to the list:\n%s", list)
	}
}

func TestEnterOnOtherCardsDoesNotOpenAnything(t *testing.T) {
	source := appOfAppsSource()
	model := openRoot(t, source)
	loads := source.treeLoads

	for _, moves := range []int{0, 2} { // the Application's own card, then the ConfigMap
		selected := model
		for range moves {
			selected = press(selected, "j")
		}
		updated, command := selected.Update(key("enter"))
		view := updated.(Model).View()
		if command != nil || !strings.Contains(view, "applications › platform-root") || strings.Contains(view, "› payments") {
			t.Fatalf("enter on the card %d rows down opened something:\n%s", moves, view)
		}
	}
	if source.treeLoads != loads {
		t.Fatalf("tree loads = %d, want %d", source.treeLoads, loads)
	}
}

func TestFailingToOpenAChildKeepsTheParentOpen(t *testing.T) {
	source := appOfAppsSource()
	source.treeErrors = map[string]error{"payments": errors.New("permission denied")}
	model := run(press(openRoot(t, source), "j"), "enter")

	view := model.View()

	if !strings.Contains(view, "permission denied") || !strings.Contains(view, "applications › platform-root") || strings.Contains(view, "› payments") {
		t.Fatalf("the failure should leave platform-root open:\n%s", view)
	}
	if list := press(model, "esc").View(); !strings.Contains(list, "applications · all · 2") {
		t.Fatalf("esc should leave platform-root, not return to a child that never opened:\n%s", list)
	}
}

func TestActionsInAChildApplicationActOnTheChild(t *testing.T) {
	model := run(press(openRoot(t, appOfAppsSource()), "j"), "enter")

	view := press(model, "s").View()

	if !strings.Contains(view, "Sync payments?") {
		t.Fatalf("sync in the child should ask about payments:\n%s", view)
	}
}

func TestParentMarksSurviveAVisitToAChild(t *testing.T) {
	model := press(press(openRoot(t, appOfAppsSource()), "j"), "j")
	model = press(press(model, " "), "k") // marks the ConfigMap card, then selects the payments card
	if model.markCount() != 1 {
		t.Fatalf("marks = %d, want 1", model.markCount())
	}

	child := run(model, "enter")
	if child.markCount() != 0 {
		t.Fatalf("the child starts with %d marks, want none", child.markCount())
	}
	back := press(child, "esc")

	if back.markCount() != 1 || !strings.Contains(back.View(), "applications › platform-root") {
		t.Fatalf("the parent should be back with its mark (marks = %d):\n%s", back.markCount(), back.View())
	}
	if cleared := press(back, "esc"); cleared.markCount() != 0 || !strings.Contains(cleared.View(), "applications › platform-root") {
		t.Fatalf("esc should first clear the mark and stay in the parent:\n%s", cleared.View())
	}
}

func TestKeyHintsOfferOpeningAChildApplication(t *testing.T) {
	model := openRoot(t, appOfAppsSource())

	if view := model.View(); strings.Contains(view, "Open app") {
		t.Fatalf("the root card should not offer to open an app:\n%s", view)
	}
	if view := press(model, "j").View(); !strings.Contains(view, "Open app") {
		t.Fatalf("a child Application card should offer to open it:\n%s", view)
	}
}
