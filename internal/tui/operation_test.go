package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/awcodify/argoxd/internal/explorer"
)

func failedOperation() *explorer.Operation {
	return &explorer.Operation{
		Phase:      "Failed",
		Message:    "one or more objects failed to apply",
		Revision:   "7d3f9a2",
		StartedAt:  time.Now().Add(-5 * time.Hour),
		FinishedAt: time.Now().Add(-5*time.Hour + 30*time.Second),
		Results: []explorer.OperationResult{
			{Kind: "Job", Namespace: "store", Name: "migrate", Status: "Synced", HookType: "PreSync", SyncPhase: "PreSync"},
			{Group: "apps", Kind: "Deployment", Namespace: "store", Name: "checkout", Status: "SyncFailed", Message: "image: Required value", SyncPhase: "Sync"},
		},
	}
}

func snapshotWithOperation(operation *explorer.Operation) explorer.Snapshot {
	snapshot := storeSnapshot()
	snapshot.Applications[1].Operation = operation
	return snapshot
}

func TestIShowsTheLastSyncOfTheSelectedApplication(t *testing.T) {
	model := resize(New(nil, "test", "argocd", snapshotWithOperation(failedOperation())), 140, 30)
	model = press(model, "j") // checkout

	view := press(model, "i").View()

	for _, want := range []string{"sync", "checkout", "Failed", "one or more objects failed to apply", "7d3f9a2", "5h ago", "Deployment/checkout", "SyncFailed", "image: Required value", "Job/migrate", "PreSync"} {
		if !strings.Contains(view, want) {
			t.Fatalf("sync details do not show %q:\n%s", want, view)
		}
	}
}

func TestSyncDetailsListFailedResourcesFirst(t *testing.T) {
	model := press(resize(New(nil, "test", "argocd", snapshotWithOperation(failedOperation())), 140, 30), "j")

	view := press(model, "i").View()

	if failed, synced := strings.Index(view, "Deployment/checkout"), strings.Index(view, "Job/migrate"); failed < 0 || failed > synced {
		t.Fatalf("the failed resource should come before the synced one:\n%s", view)
	}
}

func TestSyncDetailsEscReturnsToTheList(t *testing.T) {
	model := press(resize(New(nil, "test", "argocd", snapshotWithOperation(failedOperation())), 140, 30), "j")

	view := press(press(model, "i"), "esc").View()

	if !strings.Contains(view, "applications · all · 2") {
		t.Fatalf("esc did not return to the applications list:\n%s", view)
	}
}

func TestIOnAnApplicationThatNeverSyncedSaysSo(t *testing.T) {
	model := press(resize(New(nil, "test", "argocd", storeSnapshot()), 140, 30), "j")

	view := press(model, "i").View()

	if !strings.Contains(view, "has not been synced") {
		t.Fatalf("an application without an operation should say it was never synced:\n%s", view)
	}
}

func TestIInTheDependencyViewShowsTheApplicationsLastSync(t *testing.T) {
	source := &fakeSource{snapshot: snapshotWithOperation(failedOperation()), tree: checkoutTree()}
	model := openCheckout(t, source)

	view := press(model, "i").View()

	if !strings.Contains(view, "SyncFailed") {
		t.Fatalf("sync details do not show the failed resource:\n%s", view)
	}
	if back := press(press(model, "i"), "esc").View(); !strings.Contains(back, "applications › checkout") {
		t.Fatalf("esc did not return to the dependency view:\n%s", back)
	}
}

func TestKeyHintsListSyncDetails(t *testing.T) {
	list := resize(New(nil, "test", "argocd", storeSnapshot()), 140, 30).View()
	tree := openCheckout(t, &fakeSource{snapshot: storeSnapshot(), tree: checkoutTree()}).View()

	for name, view := range map[string]string{"list": list, "dependency view": tree} {
		if !strings.Contains(view, "Sync details") {
			t.Fatalf("%s does not hint at sync details:\n%s", name, view)
		}
	}
}
