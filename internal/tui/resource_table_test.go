package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

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

	// The name starts after the KIND column; find where, per kind's row.
	nameColumn := func(kind, name string) int {
		for _, line := range strings.Split(view, "\n") {
			if at := strings.Index(line, "│    "+kind); at >= 0 {
				rest := line[at+len("│    "+kind):]
				return ansi.StringWidth(rest[:strings.Index(rest, name)]) + len(kind)
			}
		}
		t.Fatalf("no %s row:\n%s", kind, view)
		return 0
	}
	deployment, replicaSet, pod := nameColumn("Deployment", "web"), nameColumn("ReplicaSet", "web-5d"), nameColumn("Pod", "web-5d-x")
	if deployment >= replicaSet || replicaSet >= pod {
		t.Fatalf("name columns %d, %d, %d are not growing with depth:\n%s", deployment, replicaSet, pod, view)
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

func cardTitle(view, kind string) string {
	for _, line := range strings.Split(view, "\n") {
		if strings.Contains(line, "╭─ "+kind) || strings.Contains(line, "┏━ "+kind) {
			return line
		}
	}
	return ""
}

func TestCardsThatCanFoldCarryAnIdentifier(t *testing.T) {
	model, _ := openDeep(t)
	view := model.View()

	for _, kind := range []string{"Deployment", "ReplicaSet"} {
		if title := cardTitle(view, kind); !strings.Contains(title, kind+" ▾") {
			t.Errorf("%s has resources under it but no ▾ in its title: %q", kind, title)
		}
	}
	for _, kind := range []string{"Pod", "Service"} {
		if title := cardTitle(view, kind); strings.Contains(title, "▾") || strings.Contains(title, "▸") {
			t.Errorf("%s has nothing to fold but carries a marker: %q", kind, title)
		}
	}
	if root := cardTitle(view, "Application"); strings.Contains(root, "▾") {
		t.Errorf("the Application's own card cannot be folded but carries a marker: %q", root)
	}
}

func TestAFoldedCardChangesItsIdentifier(t *testing.T) {
	model, _ := openDeep(t)

	folded := press(press(model, "j"), "left").View()

	if title := cardTitle(folded, "Deployment"); !strings.Contains(title, "Deployment ▸") || strings.Contains(title, "▾") {
		t.Fatalf("a folded Deployment should show ▸: %q", title)
	}
	if strings.Contains(folded, "ReplicaSet") {
		t.Fatalf("the folded Deployment still shows its ReplicaSet:\n%s", folded)
	}
}

func TestTableRowsThatCanFoldCarryTheSameIdentifier(t *testing.T) {
	model, _ := openDeep(t)
	row := func(view, name string) string {
		for _, line := range strings.Split(view, "\n") {
			if strings.Contains(line, " "+name+" ") {
				return line
			}
		}
		return ""
	}
	table := press(model, "w").View()

	if line := row(table, "web"); !strings.Contains(line, "▾ web") {
		t.Errorf("the Deployment row has no ▾: %q", line)
	}
	if line := row(table, "web-5d-x"); strings.Contains(line, "▾") || strings.Contains(line, "▸") {
		t.Errorf("a Pod row carries a marker: %q", line)
	}
	folded := press(press(press(model, "w"), "j"), "left").View()
	if line := row(folded, "web"); !strings.Contains(line, "▸ web") {
		t.Errorf("a folded row should show ▸: %q", line)
	}
}

func TestHintColumnsAlignEvenWithWideKeyNames(t *testing.T) {
	rendered := ansi.Strip(renderHints([]keyHint{{"←→", "Fold"}, {"z", "Everything"}, {"space", "Mark"}}))

	var columns []int
	for _, line := range strings.Split(rendered, "\n") {
		for _, action := range []string{"Fold", "Everything", "Mark"} {
			if at := strings.Index(line, action); at >= 0 {
				columns = append(columns, ansi.StringWidth(line[:at]))
			}
		}
	}
	if len(columns) != 3 || columns[0] != columns[1] || columns[1] != columns[2] {
		t.Fatalf("labels start at columns %v, want one column; hints:\n%s", columns, rendered)
	}
}

func TestTheDependencyHintsNameTheViewTheyWillSwitchTo(t *testing.T) {
	model, _ := openDeep(t)

	if view := model.View(); !strings.Contains(view, "Table") || strings.Contains(view, "Cards/Table") {
		t.Fatalf("the cards view should offer Table:\n%s", view)
	}
	if view := press(model, "w").View(); !strings.Contains(view, "Cards") || strings.Contains(view, "Cards/Table") {
		t.Fatalf("the table view should offer Cards:\n%s", view)
	}
	if view := model.View(); strings.Contains(view, "←→ z") {
		t.Fatalf("the fold hint is still the wide ←→ z keycap:\n%s", view)
	}
}

func TestASearchShowsMatchesUnderFoldedCards(t *testing.T) {
	model, _ := openDeep(t)
	folded := press(press(model, "j"), "left") // the Deployment hides its ReplicaSet and Pod
	if got := strings.Join(visibleKinds(folded), ","); got != "Application,Deployment,Service" {
		t.Fatalf("setup: folded cards = %s", got)
	}

	searched := typeKeys(folded, "/", "w", "e", "b", "-", "5", "d", "-", "x", "enter")

	if got := strings.Join(visibleKinds(searched), ","); !strings.Contains(got, "Pod") {
		t.Fatalf("searching for web-5d-x showed %s, want the Pod even though its owner is folded", got)
	}
	table := press(searched, "w")
	if view := table.View(); !strings.Contains(view, "web-5d-x") {
		t.Fatalf("the table hides a search match under a folded row:\n%s", view)
	}
}

func TestAFilterShowsMatchesUnderFoldedCards(t *testing.T) {
	model, _ := openDeep(t)
	folded := press(press(model, "j"), "left") // fold the Deployment away

	filtered := command(folded, ":health Progressing")

	if got := strings.Join(visibleKinds(filtered), ","); !strings.Contains(got, "Pod") {
		t.Fatalf("filtering for Progressing showed %s, want the Pod even though its owner is folded", got)
	}
	// The fold comes back once the filter does.
	cleared := command(filtered, ":health")
	if got := strings.Join(visibleKinds(cleared), ","); got != "Application,Deployment,Service" {
		t.Fatalf("clearing the filter showed %s, want the fold back", got)
	}
}
