package tui

import (
	"strings"
	"testing"

	"github.com/awcodify/argoxd/internal/explorer"
)

func pruneTree() explorer.ResourceTree {
	return explorer.ResourceTree{Application: "checkout", Nodes: []explorer.ResourceNode{
		{Group: "apps", Version: "v1", Kind: "Deployment", Namespace: "store", Name: "web", Health: "Healthy", Sync: "Synced"},
		{Version: "v1", Kind: "ConfigMap", Namespace: "store", Name: "legacy", Sync: "OutOfSync", RequiresPruning: true},
		{Version: "v1", Kind: "Secret", Namespace: "store", Name: "stray", Orphaned: true},
	}}
}

func TestCardsMarkResourcesThatNeedPruningAndOrphans(t *testing.T) {
	model := openCheckout(t, &fakeSource{snapshot: storeSnapshot(), tree: pruneTree()})

	view := model.View()

	for _, want := range []string{"to prune", "orphaned"} {
		if strings.Count(view, want) < 2 {
			t.Fatalf("%q should mark its card and be counted in the summary:\n%s", want, view)
		}
	}
}

func TestSummaryCountsResourcesToPruneAndOrphans(t *testing.T) {
	model := openCheckout(t, &fakeSource{snapshot: storeSnapshot(), tree: pruneTree()})

	view := model.View()

	if !strings.Contains(view, "1 to prune") || !strings.Contains(view, "1 orphaned") {
		t.Fatalf("the summary does not count them:\n%s", view)
	}
}

func TestCardsWithoutPrunesOrOrphansHaveNoMarkers(t *testing.T) {
	model := openCheckout(t, &fakeSource{snapshot: storeSnapshot(), tree: checkoutTree()})

	if view := model.View(); strings.Contains(view, "to prune") || strings.Contains(view, "orphaned") {
		t.Fatalf("an ordinary tree shows prune or orphan markers:\n%s", view)
	}
}

func TestDetailsExplainWhyAResourceIsMarked(t *testing.T) {
	model := openCheckout(t, &fakeSource{snapshot: storeSnapshot(), tree: pruneTree()})
	// Cards are Application, Deployment, ConfigMap, Secret; the details follow the cursor.
	pruning := press(press(model, "j"), "j").View()
	orphan := press(press(press(model, "j"), "j"), "j").View()

	if !strings.Contains(pruning, "deleted by a sync with prune") {
		t.Fatalf("details do not explain the pending prune:\n%s", pruning)
	}
	if !strings.Contains(orphan, "not managed by this application") {
		t.Fatalf("details do not explain the orphan:\n%s", orphan)
	}
}
