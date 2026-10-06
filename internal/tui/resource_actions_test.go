package tui

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/awcodify/argoxd/internal/explorer"
)

func (f *fakeSource) RestartResource(_ context.Context, _ string, resource explorer.ResourceNode) error {
	f.restarted = append(f.restarted, resource)
	if err := f.failures[resource.Name]; err != nil {
		return err
	}
	return f.actionErr
}

func (f *fakeSource) DeleteResource(_ context.Context, _ string, resource explorer.ResourceNode) error {
	f.deleted = append(f.deleted, resource)
	if err := f.failures[resource.Name]; err != nil {
		return err
	}
	return f.actionErr
}

// noActionsSource can show resources but cannot act on them.
type noActionsSource struct{ *fakeSource }

func (noActionsSource) RestartResource() {}
func (noActionsSource) DeleteResource()  {}

func deploymentCard(t *testing.T, source *fakeSource) Model {
	t.Helper()
	return resize(press(openCheckout(t, source), "j"), 140, 30)
}

func podCardOf(t *testing.T, source *fakeSource) Model {
	t.Helper()
	return resize(press(press(openCheckout(t, source), "j"), "j"), 140, 30)
}

func TestXAsksToRestartTheSelectedWorkloadAndYConfirms(t *testing.T) {
	source := &fakeSource{snapshot: storeSnapshot(), tree: checkoutTree()}
	model := press(deploymentCard(t, source), "x")

	if view := model.View(); !strings.Contains(view, "Restart Deployment/web?") || len(source.restarted) != 0 {
		t.Fatalf("x did not ask first (%d restarts):\n%s", len(source.restarted), view)
	}

	loadsBefore := source.treeLoads
	model = run(model, "y")

	if len(source.restarted) != 1 || source.restarted[0].Kind != "Deployment" || source.restarted[0].Name != "web" {
		t.Fatalf("restarted = %+v, want the Deployment web", source.restarted)
	}
	if view := model.View(); !strings.Contains(view, "restart requested for Deployment/web") {
		t.Fatalf("the restart was not reported:\n%s", view)
	}
	if source.treeLoads <= loadsBefore {
		t.Fatal("the dependency view was not reloaded after the restart")
	}
}

func TestNOrEscCancelsTheRestart(t *testing.T) {
	for _, cancel := range []string{"n", "esc"} {
		source := &fakeSource{snapshot: storeSnapshot(), tree: checkoutTree()}
		model := run(press(deploymentCard(t, source), "x"), cancel)

		if view := model.View(); strings.Contains(view, "Restart Deployment") || len(source.restarted) != 0 {
			t.Fatalf("%s did not cancel the restart (%d restarts):\n%s", cancel, len(source.restarted), view)
		}
	}
}

func TestShiftXAsksToDeleteThePodAndYConfirms(t *testing.T) {
	source := &fakeSource{snapshot: storeSnapshot(), tree: checkoutTree()}
	model := press(podCardOf(t, source), "X")

	if view := model.View(); !strings.Contains(view, "Delete Pod/web-abc?") || len(source.deleted) != 0 {
		t.Fatalf("X did not ask first (%d deletions):\n%s", len(source.deleted), view)
	}

	model = run(model, "y")

	if len(source.deleted) != 1 || source.deleted[0].Kind != "Pod" || source.deleted[0].Name != "web-abc" || source.deleted[0].Namespace != "store" {
		t.Fatalf("deleted = %+v, want the Pod web-abc", source.deleted)
	}
	if view := model.View(); !strings.Contains(view, "delete requested for Pod/web-abc") {
		t.Fatalf("the deletion was not reported:\n%s", view)
	}
}

func TestNCancelsTheDeletion(t *testing.T) {
	source := &fakeSource{snapshot: storeSnapshot(), tree: checkoutTree()}
	model := run(press(podCardOf(t, source), "X"), "n")

	if view := model.View(); strings.Contains(view, "Delete Pod") || len(source.deleted) != 0 {
		t.Fatalf("n did not cancel the deletion:\n%s", view)
	}
}

func TestOnlyWorkloadsCanBeRestartedAndOnlyPodsDeleted(t *testing.T) {
	source := &fakeSource{snapshot: storeSnapshot(), tree: checkoutTree()}

	for name, test := range map[string]struct {
		model Model
		key   string
		want  string
	}{
		"restart a pod":           {podCardOf(t, source), "x", "Only Deployments, StatefulSets and DaemonSets can be restarted"},
		"delete a deployment":     {deploymentCard(t, source), "X", "Only Pods can be deleted here"},
		"restart the application": {resize(openCheckout(t, source), 140, 30), "x", "Select a resource card"},
		"delete the application":  {resize(openCheckout(t, source), 140, 30), "X", "Select a resource card"},
	} {
		view := press(test.model, test.key).View()
		if !strings.Contains(view, test.want) || strings.Contains(view, "confirm") {
			t.Fatalf("%s: want %q and no confirmation:\n%s", name, test.want, view)
		}
		if len(source.restarted) != 0 || len(source.deleted) != 0 {
			t.Fatalf("%s: an action ran", name)
		}
	}
}

func TestResourceActionsOnlyOpenInTheDependencyView(t *testing.T) {
	source := &fakeSource{snapshot: storeSnapshot(), tree: checkoutTree()}
	list := settle(resize(New(source, "test", "argocd", explorer.Snapshot{}), 140, 30), source.loadCommand())

	for _, pressed := range []string{"x", "X"} {
		if view := press(list, pressed).View(); strings.Contains(view, "Restart ") || strings.Contains(view, "Delete Pod") {
			t.Fatalf("%s opened a resource action in the list:\n%s", pressed, view)
		}
	}
}

func TestResourceActionsNeedASourceThatSupportsThem(t *testing.T) {
	source := noActionsSource{&fakeSource{snapshot: storeSnapshot(), tree: checkoutTree()}}
	model := resize(New(source, "test", "argocd", explorer.Snapshot{}), 140, 30)
	model = settle(model, source.loadCommand())
	deployment := press(run(press(model, "j"), "enter"), "j")
	pod := press(deployment, "j")

	for pressed, selected := range map[string]Model{"x": deployment, "X": pod} {
		view := press(selected, pressed).View()
		if !strings.Contains(view, "cannot act on resources") || strings.Contains(view, "confirm") {
			t.Fatalf("%s did not explain the missing support:\n%s", pressed, view)
		}
	}
}

func TestAFailedResourceActionIsShown(t *testing.T) {
	source := &fakeSource{snapshot: storeSnapshot(), tree: checkoutTree(), actionErr: errors.New("forbidden by RBAC")}
	model := run(press(deploymentCard(t, source), "x"), "y")

	view := model.View()
	if !strings.Contains(view, "forbidden by RBAC") || strings.Contains(view, "restart requested") {
		t.Fatalf("the failure was not shown instead of a success:\n%s", view)
	}
}

func TestDependencyHeaderListsTheResourceActions(t *testing.T) {
	source := &fakeSource{snapshot: storeSnapshot(), tree: checkoutTree()}
	view := resize(openCheckout(t, source), 170, 30).View()

	for _, want := range []string{"x  Restart", "X  Delete pod"} {
		if !strings.Contains(view, want) {
			t.Fatalf("the dependency header does not list %q:\n%s", want, view)
		}
	}
}
