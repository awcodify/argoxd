package tui

import (
	"strings"
	"testing"

	"github.com/awcodify/argoxd/internal/argocd"
	"github.com/awcodify/argoxd/internal/explorer"
)

func TestDemoSourceDrivesTheExplorer(t *testing.T) {
	model := resize(New(argocd.NewDemoSource(), "demo", "argocd", explorer.Snapshot{}), 150, 40)
	model = settle(model, model.Init())
	list := model.View()
	for _, want := range []string{"checkout", "Degraded", "OutOfSync"} {
		if !strings.Contains(list, want) {
			t.Fatalf("demo list is missing %q:\n%s", want, list)
		}
	}

	model = run(model, "enter")
	tree := model.View()
	for _, want := range []string{"Deployment", "ReplicaSet", "Pod"} {
		if !strings.Contains(tree, want) {
			t.Fatalf("demo dependencies are missing %q:\n%s", want, tree)
		}
	}
}
