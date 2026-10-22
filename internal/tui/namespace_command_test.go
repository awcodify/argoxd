package tui

import (
	"context"
	"strings"
	"testing"

	"github.com/awcodify/argoxd/internal/argocd"
	"github.com/awcodify/argoxd/internal/explorer"
)

type namespaceSource struct {
	*fakeSource
	namespaces []string
	switched   string
}

func (s *namespaceSource) Namespaces(context.Context) ([]string, error) {
	return s.namespaces, nil
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

	model = command(model, ":ns gitops")
	if source.switched != "gitops" || model.namespace != "gitops" || !strings.Contains(model.View(), "gitops") {
		t.Fatalf("namespace switch = %q, model %q:\n%s", source.switched, model.namespace, model.View())
	}
}

func TestNamespaceCommandOpensAPicker(t *testing.T) {
	model, source := newNamespaceModel()

	picker := command(model, ":ns")
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

	model = command(model, ":namespace gitops")
	updated, _ := model.Update(inFlight())
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
