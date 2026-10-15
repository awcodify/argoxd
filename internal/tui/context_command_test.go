package tui

import (
	"strings"
	"testing"

	"github.com/awcodify/argoxd/internal/argocd"
	"github.com/awcodify/argoxd/internal/explorer"
)

type contextSource struct {
	*fakeSource
	contexts []string
	switched string
}

func (s *contextSource) Contexts() ([]string, error) {
	return s.contexts, nil
}

func (s *contextSource) WithContext(context string) argocd.Source {
	s.switched = context
	return s
}

func TestContextCommandReconnectsKubeconfigSource(t *testing.T) {
	source := &contextSource{
		fakeSource: &fakeSource{snapshot: storeSnapshot()},
		contexts:   []string{"development", "production"},
	}
	model := settle(New(source, "Kubeconfig context: development", "argocd", explorer.Snapshot{}), source.loadCommand())

	model = command(model, ":ctx production")
	if source.switched != "production" || !strings.Contains(model.View(), "Kubeconfig context: production") {
		t.Fatalf("context switch = %q:\n%s", source.switched, model.View())
	}

	suggestions := typeKeys(model, ":", "c", "t", "x", " ")
	if view := suggestions.View(); !strings.Contains(view, "ctx production") {
		t.Fatalf(":ctx did not suggest kubeconfig contexts:\n%s", view)
	}
}

func TestContextCommandOpensAPicker(t *testing.T) {
	source := &contextSource{
		fakeSource: &fakeSource{snapshot: storeSnapshot()},
		contexts:   []string{"development", "production"},
	}
	model := settle(New(source, "Kubeconfig context: development", "argocd", explorer.Snapshot{}), source.loadCommand())

	picker := command(model, ":ctx")
	if !picker.prompt.active || picker.prompt.field != contextField {
		t.Fatalf(":ctx did not open the context picker:\n%s", picker.View())
	}
	if view := picker.View(); !strings.Contains(view, "development") || !strings.Contains(view, "production") {
		t.Fatalf("context picker does not list kubeconfig contexts:\n%s", view)
	}
	switched := run(press(picker, "down"), "enter")
	if source.switched != "production" || !strings.Contains(switched.View(), "Kubeconfig context: production") {
		t.Fatalf("picker switched to %q:\n%s", source.switched, switched.View())
	}
}

func TestContextCommandRejectsUnsupportedSources(t *testing.T) {
	model := command(New(&fakeSource{snapshot: storeSnapshot()}, "test", "argocd", storeSnapshot()), ":ctx production")

	if view := model.View(); !strings.Contains(view, "available only with --source kubeconfig") {
		t.Fatalf("unsupported context switch was not explained:\n%s", view)
	}
}

var _ argocd.ContextSwitcher = (*contextSource)(nil)
