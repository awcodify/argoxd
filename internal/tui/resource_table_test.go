package tui

import (
	"strings"
	"testing"

	"github.com/awcodify/argoxd/internal/explorer"
)

// deepTree is Deployment web → ReplicaSet web-5d → Pod web-5d-x, plus a Service.
func deepTree() explorer.ResourceTree {
	deployment := explorer.ResourceReference{Group: "apps", Kind: "Deployment", Namespace: "store", Name: "web"}
	replicaSet := explorer.ResourceReference{Group: "apps", Kind: "ReplicaSet", Namespace: "store", Name: "web-5d"}
	return explorer.ResourceTree{Application: "checkout", Nodes: []explorer.ResourceNode{
		{Group: "apps", Version: "v1", Kind: "Deployment", Namespace: "store", Name: "web", Health: "Healthy", Sync: "Synced"},
		{Group: "apps", Version: "v1", Kind: "ReplicaSet", Namespace: "store", Name: "web-5d", Health: "Healthy", Parents: []explorer.ResourceReference{deployment}},
		{Version: "v1", Kind: "Pod", Namespace: "store", Name: "web-5d-x", Health: "Progressing", Parents: []explorer.ResourceReference{replicaSet}},
		{Version: "v1", Kind: "Service", Namespace: "store", Name: "web-svc", Health: "Healthy", Sync: "OutOfSync", RequiresPruning: true},
	}}
}

func openDeep(t *testing.T) (Model, *fakeSource) {
	t.Helper()
	source := &fakeSource{snapshot: storeSnapshot(), tree: deepTree()}
	return resize(openCheckout(t, source), 140, 40), source
}

func hasCards(view string) bool {
	return strings.Contains(view, "╭─ Deployment") || strings.Contains(view, "┏━ Application")
}

func TestWShowsTheResourcesAsATableAndWAgainAsCards(t *testing.T) {
	model, _ := openDeep(t)
	if !hasCards(model.View()) {
		t.Fatalf("the dependency view should start as cards:\n%s", model.View())
	}

	table := press(model, "w")
	view := table.View()
	for _, want := range []string{"KIND", "NAME", "NAMESPACE", "HEALTH", "SYNC", "Deployment", "ReplicaSet", "web-5d-x", "Service", "web-svc"} {
		if !strings.Contains(view, want) {
			t.Fatalf("the table does not show %q:\n%s", want, view)
		}
	}
	if hasCards(view) {
		t.Fatalf("the table view still draws cards:\n%s", view)
	}
	if back := press(table, "w"); !hasCards(back.View()) {
		t.Fatalf("w did not return to the cards:\n%s", back.View())
	}
}

func TestTheTableIndentsEachResourceUnderItsOwner(t *testing.T) {
	model, _ := openDeep(t)
	view := press(model, "w").View()

	row := func(name string) string {
		for _, line := range strings.Split(view, "\n") {
			if strings.Contains(line, name+" ") || strings.HasSuffix(strings.TrimRight(line, " │"), name) {
				return line
			}
		}
		return ""
	}
	indent := func(line, name string) int { return strings.Index(line, name) }
	deployment, replicaSet, pod := row("web"), row("web-5d"), row("web-5d-x")
	if indent(replicaSet, "web-5d") <= indent(deployment, "web") || indent(pod, "web-5d-x") <= indent(replicaSet, "web-5d") {
		t.Fatalf("names are not indented by depth:\n%s", view)
	}
}

func TestActionsAndFiltersWorkOnTheTableRows(t *testing.T) {
	model, source := openDeep(t)
	model = press(press(model, "w"), "j") // the Deployment: row 0 is the Application

	if view := press(model, "x").View(); !strings.Contains(view, "Restart Deployment/web?") {
		t.Fatalf("x on a table row did not ask about the Deployment:\n%s", view)
	}
	searched := typeKeys(press(openDeepFrom(source, t), "w"), "/", "w", "e", "b", "-", "s", "v", "c", "enter").View()
	if !strings.Contains(searched, "web-svc") || strings.Contains(searched, "web-5d-x") {
		t.Fatalf("searching did not narrow the table:\n%s", searched)
	}
}

func openDeepFrom(source *fakeSource, t *testing.T) Model {
	t.Helper()
	return resize(openCheckout(t, source), 140, 40)
}

func TestTableRowsShowMarksAndPruneFlags(t *testing.T) {
	model, _ := openDeep(t)
	model = press(press(model, "w"), "j")

	view := press(model, " ").View()

	if !strings.Contains(view, "●") || !strings.Contains(view, "to prune") {
		t.Fatalf("marked rows or the prune flag are missing:\n%s", view)
	}
}

func TestTheResourceViewCanStartAsATable(t *testing.T) {
	source := &fakeSource{snapshot: storeSnapshot(), tree: deepTree()}
	model := resize(New(source, "test", "argocd", explorer.Snapshot{}).WithResourceView("table"), 140, 40)
	model = settle(model, source.loadCommand())
	model = run(press(model, "j"), "enter")

	if view := model.View(); hasCards(view) || !strings.Contains(view, "NAMESPACE") {
		t.Fatalf("the view did not start as a table:\n%s", view)
	}
}

func visibleKinds(model Model) []string {
	var kinds []string
	for _, c := range flatten(model.dependencyRoot(), "") {
		kinds = append(kinds, c.node.Kind)
	}
	return kinds
}

func TestLeftCollapsesACardAndRightExpandsItAgain(t *testing.T) {
	model, _ := openDeep(t)
	model = press(model, "j") // Deployment
	if got := strings.Join(visibleKinds(model), ","); got != "Application,Deployment,ReplicaSet,Pod,Service" {
		t.Fatalf("cards = %s", got)
	}

	collapsed := press(model, "left")
	if got := strings.Join(visibleKinds(collapsed), ","); got != "Application,Deployment,Service" {
		t.Fatalf("after Left: %s, want the Deployment's descendants hidden", got)
	}
	if view := collapsed.View(); !strings.Contains(view, "2 hidden") {
		t.Fatalf("a collapsed card does not say what it hides:\n%s", view)
	}
	if collapsed.selectedCard().Kind != "Deployment" {
		t.Fatalf("the cursor left the collapsed card: %s", collapsed.selectedCard().Kind)
	}

	expanded := press(collapsed, "right")
	if got := strings.Join(visibleKinds(expanded), ","); got != "Application,Deployment,ReplicaSet,Pod,Service" {
		t.Fatalf("after Right: %s, want everything back", got)
	}
}

func TestLeftOnACardWithNothingToCollapseMovesToItsOwner(t *testing.T) {
	model, _ := openDeep(t)
	pod := typeKeys(model, "j", "j", "j") // Application, Deployment, ReplicaSet, Pod
	if pod.selectedCard().Kind != "Pod" {
		t.Fatalf("selected = %s, want Pod", pod.selectedCard().Kind)
	}

	if got := press(pod, "left").selectedCard().Kind; got != "ReplicaSet" {
		t.Fatalf("Left on a Pod selected %s, want its ReplicaSet", got)
	}
}

func TestZCollapsesEveryParentAndZAgainExpandsThem(t *testing.T) {
	model, _ := openDeep(t)

	collapsed := press(model, "z")
	if got := strings.Join(visibleKinds(collapsed), ","); got != "Application,Deployment,Service" {
		t.Fatalf("after z: %s, want only the top-level resources", got)
	}
	if expanded := press(collapsed, "z"); len(visibleKinds(expanded)) != 5 {
		t.Fatalf("a second z left %v", visibleKinds(expanded))
	}
}

func TestCollapsedCardsAreForgottenWhenAnotherApplicationOpens(t *testing.T) {
	model, _ := openDeep(t)
	model = press(press(model, "j"), "left")

	reopened := run(press(run(model, "esc"), "j"), "enter")

	if len(reopened.collapsed) != 0 {
		t.Fatalf("collapsed = %v after reopening, want it cleared", reopened.collapsed)
	}
}
