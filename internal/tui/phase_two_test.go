package tui

import (
	"errors"
	"strings"
	"testing"

	"github.com/awcodify/argoxd/internal/argocd"
	"github.com/awcodify/argoxd/internal/explorer"
)

type browserSource struct{ *fakeSource }

func (s *browserSource) ApplicationURL(application string) string {
	return "https://argocd.example.com/applications/" + application
}

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

func TestYankCopiesSelectedItemAndViewerText(t *testing.T) {
	var copied []string
	copyToClipboard = func(text string) error {
		copied = append(copied, text)
		return nil
	}
	t.Cleanup(func() { copyToClipboard = systemCopyToClipboard })

	source := &fakeSource{snapshot: storeSnapshot(), tree: checkoutTree(), manifest: "kind: Deployment\nmetadata:\n  name: web\n"}
	list := settle(New(source, "test", "argocd", explorer.Snapshot{}), source.loadCommand())
	list = press(list, "y")
	if len(copied) != 1 || copied[0] != "grafana" || !strings.Contains(list.View(), "Copied to clipboard") {
		t.Fatalf("list copy = %v:\n%s", copied, list.View())
	}

	viewer := run(press(openCheckout(t, source), "j"), "Y")
	viewer = press(viewer, "y")
	if len(copied) != 2 || copied[1] != strings.TrimRight(source.manifest, "\n") {
		t.Fatalf("viewer copy = %v, want manifest", copied)
	}
}

func TestYankReportsClipboardFailures(t *testing.T) {
	copyToClipboard = func(string) error { return errors.New("clipboard unavailable") }
	t.Cleanup(func() { copyToClipboard = systemCopyToClipboard })

	model := press(New(nil, "test", "argocd", storeSnapshot()), "y")

	if view := model.View(); !strings.Contains(view, "clipboard unavailable") {
		t.Fatalf("clipboard error was not shown:\n%s", view)
	}
}

func TestOpenInBrowserUsesAPISourceOnly(t *testing.T) {
	var opened string
	openInBrowser = func(url string) error {
		opened = url
		return nil
	}
	t.Cleanup(func() { openInBrowser = systemOpenInBrowser })

	source := &browserSource{&fakeSource{snapshot: storeSnapshot()}}
	model := settle(New(source, "test", "argocd", explorer.Snapshot{}), source.loadCommand())
	model = press(model, "o")
	if opened != "https://argocd.example.com/applications/grafana" || !strings.Contains(model.View(), "Opened grafana in browser") {
		t.Fatalf("opened = %q:\n%s", opened, model.View())
	}

	unsupported := press(New(&fakeSource{snapshot: storeSnapshot()}, "test", "argocd", storeSnapshot()), "o")
	if view := unsupported.View(); !strings.Contains(view, "needs --source api") {
		t.Fatalf("non-API browser opening was not refused:\n%s", view)
	}
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
