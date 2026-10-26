package tui

import (
	"context"
	"strings"
	"testing"

	"github.com/awcodify/argoxd/internal/argocd"
	"github.com/awcodify/argoxd/internal/explorer"
	tea "github.com/charmbracelet/bubbletea"
)

type namespaceSource struct {
	*fakeSource
	namespaces []string
	switched   string
	listed     int
}

func (s *namespaceSource) Namespaces(context.Context) ([]string, error) {
	s.listed++
	return s.namespaces, nil
}

// enter types a command and presses Enter, returning the model and the command
// that Bubble Tea would run next.
func enter(model Model, value string) (Model, tea.Cmd) {
	for _, character := range value {
		model = press(model, string(character))
	}
	updated, next := model.Update(key("enter"))
	return updated.(Model), next
}

// commandSettled is command, but also runs the commands it returns.
func commandSettled(model Model, value string) Model {
	model, next := enter(model, value)
	return settle(model, next)
}

func (s *namespaceSource) WithNamespace(namespace string) argocd.Source {
	s.switched = namespace
	return s
}

func newNamespaceModel() (Model, *namespaceSource) {
	source := &namespaceSource{
		fakeSource: &fakeSource{snapshot: storeSnapshot()},
		namespaces: []string{"argocd", "gitops"},
	}
	return settle(New(source, "Kubeconfig current context", "argocd", explorer.Snapshot{}), source.loadCommand()), source
}

func TestNamespaceCommandSwitchesAndReloads(t *testing.T) {
	model, source := newNamespaceModel()

	model = commandSettled(model, ":ns gitops")
	if source.switched != "gitops" || model.namespace != "gitops" || !strings.Contains(model.View(), "gitops") {
		t.Fatalf("namespace switch = %q, model %q:\n%s", source.switched, model.namespace, model.View())
	}
}

func TestNamespaceCommandOpensAPicker(t *testing.T) {
	model, source := newNamespaceModel()

	picker := commandSettled(model, ":ns")
	if !picker.prompt.active || picker.prompt.field != namespaceField {
		t.Fatalf(":ns did not open the namespace picker:\n%s", picker.View())
	}
	if view := picker.View(); !strings.Contains(view, "argocd") || !strings.Contains(view, "gitops") {
		t.Fatalf("picker does not list namespaces:\n%s", view)
	}
	switched := run(press(picker, "down"), "enter")
	if source.switched != "gitops" || switched.namespace != "gitops" {
		t.Fatalf("picker switched to %q (model %q)", source.switched, switched.namespace)
	}
}

func TestNamespaceSwitchIgnoresLoadsFromThePreviousNamespace(t *testing.T) {
	model, _ := newNamespaceModel()
	inFlight := model.load()

	model, _ = enter(model, ":namespace gitops")
	updated, _ := model.Update(namespacesListed{namespaces: []string{"argocd", "gitops"}, switchTo: "gitops"})
	model = updated.(Model)
	updated, _ = model.Update(inFlight())
	model = updated.(Model)

	if apps := model.explorer.Snapshot().Applications; len(apps) != 0 {
		t.Fatalf("a load from the previous namespace replaced the new data: %v", apps)
	}
}

func TestNamespaceCommandNeedsAKubeconfigSource(t *testing.T) {
	model := settle(New(&fakeSource{snapshot: storeSnapshot()}, "Argo CD API", "argocd", explorer.Snapshot{}), nil)

	model = command(model, ":ns gitops")
	if model.namespace != "argocd" || model.err == nil {
		t.Fatalf("namespace = %q, err = %v", model.namespace, model.err)
	}
}

func TestSwitchingNamespaceOrContextForgetsMarks(t *testing.T) {
	model, source := newNamespaceModel()
	model = press(model, " ")
	if len(model.marked) != 1 {
		t.Fatalf("marked = %v, want one marked application", model.marked)
	}

	model = settle(commandSettled(model, ":ns gitops"), source.loadCommand())
	if len(model.marked) != 0 {
		t.Fatalf("marks from namespace argocd survived the switch to gitops: %v", model.marked)
	}

	contexts := &contextSource{fakeSource: &fakeSource{snapshot: storeSnapshot()}, contexts: []string{"production"}}
	model = press(settle(New(contexts, "Kubeconfig context: development", "argocd", explorer.Snapshot{}), contexts.loadCommand()), " ")
	model = settle(command(model, ":ctx production"), contexts.loadCommand())
	if len(model.marked) != 0 {
		t.Fatalf("marks from context development survived the switch to production: %v", model.marked)
	}
}

func TestNamespaceLookupsRunInACommandNotInTheKeyHandler(t *testing.T) {
	model, source := newNamespaceModel()

	picker, next := enter(model, ":ns")
	if source.listed != 0 {
		t.Fatalf("opening the picker listed namespaces inside Update (%d calls); that freezes the screen on a slow cluster", source.listed)
	}
	if !picker.prompt.active || next == nil {
		t.Fatalf("picker open = %v, command = %v; want the picker open at once and a lookup command", picker.prompt.active, next != nil)
	}
	picker = settle(picker, next)
	if source.listed != 1 || !strings.Contains(picker.View(), "gitops") {
		t.Fatalf("listed = %d; picker after the lookup:\n%s", source.listed, picker.View())
	}

	before := source.listed
	switching, next := enter(model, ":ns gitops")
	if source.listed != before || source.switched != "" || switching.namespace != "argocd" {
		t.Fatalf("switching checked the namespace inside Update: listed %d, switched %q, namespace %q", source.listed, source.switched, switching.namespace)
	}
	if switching = settle(switching, next); switching.namespace != "gitops" || source.switched != "gitops" {
		t.Fatalf("namespace = %q, switched = %q after the lookup", switching.namespace, source.switched)
	}
}

func TestNamespaceSwitchToAMissingNamespaceReportsItAfterTheLookup(t *testing.T) {
	model, source := newNamespaceModel()

	model = commandSettled(model, ":ns nowhere")

	if model.namespace != "argocd" || source.switched != "" || model.err == nil || !strings.Contains(model.err.Error(), "nowhere") {
		t.Fatalf("namespace = %q, switched = %q, err = %v", model.namespace, source.switched, model.err)
	}
}
