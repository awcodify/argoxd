package tui

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/awcodify/argoxd/internal/argocd"
	"github.com/awcodify/argoxd/internal/explorer"
)

// actionSource is a source whose resources offer actions, such as an Argo CD
// that knows a Rollout's promote and abort.
type actionSource struct {
	*fakeSource
	actions   []argocd.ResourceAction
	listErr   error
	listed    int
	ran       []string
	runTarget explorer.ResourceNode
	runErr    error
}

func (s *actionSource) ResourceActions(_ context.Context, _ string, _ explorer.ResourceNode) ([]argocd.ResourceAction, error) {
	s.listed++
	return s.actions, s.listErr
}

func (s *actionSource) RunResourceAction(_ context.Context, _ string, resource explorer.ResourceNode, action string) error {
	s.ran = append(s.ran, action)
	s.runTarget = resource
	return s.runErr
}

func newActionSource() *actionSource {
	return &actionSource{
		fakeSource: &fakeSource{snapshot: storeSnapshot(), tree: checkoutTree()},
		actions: []argocd.ResourceAction{
			{Name: "promote-full"}, {Name: "abort"}, {Name: "retry", Disabled: true}, {Name: "restart"},
		},
	}
}

func openActionCard(t *testing.T, source *actionSource) Model {
	t.Helper()
	model := resize(New(source, "test", "argocd", explorer.Snapshot{}), 140, 40)
	model = settle(model, source.loadCommand())
	model = run(press(model, "j"), "enter")
	if !strings.Contains(model.View(), "applications › checkout") {
		t.Fatalf("checkout did not open:\n%s", model.View())
	}
	return press(model, "j")
}

func TestAOpensAMenuOfTheActionsTheResourceOffers(t *testing.T) {
	source := newActionSource()
	model := run(openActionCard(t, source), "a")

	view := model.View()
	for _, want := range []string{"promote-full", "abort", "restart"} {
		if !strings.Contains(view, want) {
			t.Fatalf("the menu does not offer %q:\n%s", want, view)
		}
	}
	if strings.Contains(view, "retry") {
		t.Fatalf("the menu offers a disabled action:\n%s", view)
	}
	if !model.prompt.active || model.prompt.field != actionField {
		t.Fatalf("the action bar is not open (field %q)", model.prompt.field)
	}
}

func TestTheActionLookupRunsInACommandNotInTheKeyHandler(t *testing.T) {
	source := newActionSource()
	model := openActionCard(t, source)

	updated, next := model.Update(key("a"))

	if source.listed != 0 || next == nil || updated.(Model).prompt.active {
		t.Fatalf("listed = %d, command = %v, bar open = %v; want the lookup deferred to a command", source.listed, next != nil, updated.(Model).prompt.active)
	}
}

func TestChoosingAnActionAsksFirstAndYRunsIt(t *testing.T) {
	source := newActionSource()
	model := press(run(openActionCard(t, source), "a"), "down")
	model = press(model, "enter")

	if view := model.View(); !strings.Contains(view, "Run abort on Deployment/web?") || len(source.ran) != 0 {
		t.Fatalf("choosing did not ask first (%d runs):\n%s", len(source.ran), view)
	}

	loadsBefore := source.treeLoads
	model = run(model, "y")

	if len(source.ran) != 1 || source.ran[0] != "abort" || source.runTarget.Name != "web" {
		t.Fatalf("ran = %v on %q, want abort on web", source.ran, source.runTarget.Name)
	}
	if view := model.View(); !strings.Contains(view, "abort requested for Deployment/web") {
		t.Fatalf("the action was not reported:\n%s", view)
	}
	if source.treeLoads <= loadsBefore {
		t.Fatal("the dependency view was not reloaded after the action")
	}
}

func TestTypingNarrowsTheActionMenu(t *testing.T) {
	source := newActionSource()
	model := typeKeys(run(openActionCard(t, source), "a"), "p", "r", "o", "m")

	view := model.View()
	if !strings.Contains(view, "promote-full") || strings.Contains(view, "abort") {
		t.Fatalf("typing did not narrow the menu:\n%s", view)
	}
	if model = press(model, "enter"); !strings.Contains(model.View(), "Run promote-full on") {
		t.Fatalf("Enter did not choose the match:\n%s", model.View())
	}
}

func TestEscClosesTheMenuAndNCancelsTheConfirmation(t *testing.T) {
	source := newActionSource()
	model := run(openActionCard(t, source), "a")

	if closed := press(model, "esc"); closed.prompt.active || len(source.ran) != 0 {
		t.Fatalf("esc did not close the menu (%d runs)", len(source.ran))
	}
	asking := press(model, "enter")
	if cancelled := run(asking, "n"); strings.Contains(cancelled.View(), "Run promote-full") || len(source.ran) != 0 {
		t.Fatalf("n did not cancel (%d runs):\n%s", len(source.ran), cancelled.View())
	}
}

func TestAResourceWithoutActionsSaysSo(t *testing.T) {
	source := newActionSource()
	source.actions = []argocd.ResourceAction{{Name: "retry", Disabled: true}}

	model := run(openActionCard(t, source), "a")

	if model.prompt.active || !strings.Contains(model.View(), "No actions for Deployment/web") {
		t.Fatalf("no-actions case:\n%s", model.View())
	}
}

func TestActionsThatNeedParametersAreLeftOutAndExplained(t *testing.T) {
	source := newActionSource()
	source.actions = []argocd.ResourceAction{{Name: "scale", NeedsParameters: true}, {Name: "pause"}}

	model := run(openActionCard(t, source), "a")
	if view := model.View(); !strings.Contains(view, "pause") || strings.Contains(view, "scale") {
		t.Fatalf("the menu should offer pause but not scale:\n%s", view)
	}

	source.actions = []argocd.ResourceAction{{Name: "scale", NeedsParameters: true}}
	model = run(openActionCard(t, source), "a")
	if model.prompt.active || !strings.Contains(model.View(), "only has actions that need parameters") {
		t.Fatalf("a resource with only parameterized actions:\n%s", model.View())
	}
}

func TestAFailedLookupIsReported(t *testing.T) {
	source := newActionSource()
	source.listErr = errors.New("permission denied")

	model := run(openActionCard(t, source), "a")

	if model.prompt.active || !strings.Contains(model.View(), "permission denied") {
		t.Fatalf("lookup failure:\n%s", model.View())
	}
}

func TestReadOnlyModeBlocksTheActionMenu(t *testing.T) {
	source := newActionSource()
	model := openActionCard(t, source).WithReadOnly(true)

	model = run(model, "a")

	if source.listed != 0 || model.prompt.active || !strings.Contains(model.View(), "Read-only mode") {
		t.Fatalf("read-only did not block the menu (listed %d):\n%s", source.listed, model.View())
	}
}

func TestTheActionMenuNeedsAResourceCardAndASourceThatCanRunActions(t *testing.T) {
	source := newActionSource()
	model := resize(New(source, "test", "argocd", explorer.Snapshot{}), 140, 40)
	model = run(press(settle(model, source.loadCommand()), "j"), "enter")

	if root := run(model, "a"); root.prompt.active || !strings.Contains(root.View(), "Select a resource card") {
		t.Fatalf("a on the root card:\n%s", root.View())
	}

	plain := &fakeSource{snapshot: storeSnapshot(), tree: checkoutTree()}
	card := run(deploymentCard(t, plain), "a")
	if card.prompt.active || card.err == nil {
		t.Fatalf("a on a source without actions: err = %v", card.err)
	}
}

func TestAShowsInTheKeyHintsOfTheDependencyView(t *testing.T) {
	model := openActionCard(t, newActionSource())

	if view := model.View(); !strings.Contains(view, "Actions") {
		t.Fatalf("the header does not hint at a:\n%s", view)
	}
}
