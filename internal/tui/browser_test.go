package tui

import (
	"strings"
	"testing"

	"github.com/awcodify/argoxd/internal/explorer"
)

type browserSource struct{ *fakeSource }

func (s *browserSource) ApplicationURL(application string) string {
	return "https://argocd.example.com/applications/" + application
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
