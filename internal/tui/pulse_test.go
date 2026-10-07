package tui

import (
	"strings"
	"testing"

	"github.com/awcodify/argoxd/internal/explorer"
)

func pulseSnapshot() explorer.Snapshot {
	snapshot := storeSnapshot() // grafana is Synced and Healthy; checkout is OutOfSync and Degraded
	snapshot.Applications[1].Operation = &explorer.Operation{Phase: "Failed", Message: "boom"}
	snapshot.Applications[1].Conditions = []explorer.Condition{{Type: "SyncError", Message: "boom"}}
	snapshot.Applications[0].Policy = explorer.SyncPolicy{Automated: true}
	snapshot.ApplicationSets = []explorer.ApplicationSet{
		{Name: "a", Generators: []string{"git"}},
		{Name: "b", Generators: []string{"git"}, Problems: []explorer.Condition{{Type: "ErrorOccurred", Message: "x"}}},
	}
	return snapshot
}

func TestPulseCommandShowsTheOverview(t *testing.T) {
	for _, name := range []string{":pulse", ":overview"} {
		model := resize(New(nil, "test", "argocd", pulseSnapshot()), 140, 30)

		view := command(model, name).View()

		for _, want := range []string{
			"pulse · 1", "2 applications", "2 application sets", "1 with problems",
			"1 Healthy", "1 Degraded", "1 Synced", "1 out of sync", "auto-sync on 1 of 2",
		} {
			if !strings.Contains(view, want) {
				t.Fatalf("%s does not show %q:\n%s", name, want, view)
			}
		}
	}
}

func TestPulseListsApplicationsNeedingAttentionWithTheirReasons(t *testing.T) {
	model := resize(New(nil, "test", "argocd", pulseSnapshot()), 140, 30)

	view := command(model, ":pulse").View()

	for _, want := range []string{"checkout", "Degraded", "sync Failed", "1 condition"} {
		if !strings.Contains(view, want) {
			t.Fatalf("the list does not show %q:\n%s", want, view)
		}
	}
	if strings.Contains(view, "grafana") {
		t.Fatalf("grafana is fine but is listed:\n%s", view)
	}
}

func TestPulseSaysWhenEverythingIsFine(t *testing.T) {
	snapshot := explorer.Snapshot{Applications: []explorer.Application{{Name: "grafana", Sync: "Synced", Health: "Healthy"}}}
	model := resize(New(nil, "test", "argocd", snapshot), 140, 30)

	view := command(model, ":pulse").View()

	if !strings.Contains(view, "Everything looks fine") || strings.Contains(view, "Nothing to show yet.") {
		t.Fatalf("an overview with nothing wrong should say so:\n%s", view)
	}
}

func TestEnterOnAPulseRowOpensTheApplicationAndEscComesBack(t *testing.T) {
	source := &fakeSource{snapshot: pulseSnapshot(), tree: checkoutTree()}
	model := resize(New(source, "test", "argocd", explorer.Snapshot{}), 140, 40)
	model = command(settle(model, source.loadCommand()), ":pulse")

	opened := run(model, "enter")

	if view := opened.View(); !strings.Contains(view, "pulse › checkout") || !strings.Contains(view, "Deployment") {
		t.Fatalf("enter did not open checkout from the overview:\n%s", view)
	}
	if back := press(opened, "esc").View(); !strings.Contains(back, "pulse · 1") {
		t.Fatalf("esc did not return to the overview:\n%s", back)
	}
}

func TestIOnAPulseRowShowsWhyTheSyncFailed(t *testing.T) {
	model := resize(New(nil, "test", "argocd", pulseSnapshot()), 140, 30)

	view := press(command(model, ":pulse"), "i").View()

	if !strings.Contains(view, "Failed") || !strings.Contains(view, "boom") {
		t.Fatalf("i did not show the failed sync of checkout:\n%s", view)
	}
}

func TestSyncFromThePulseIgnoresApplicationsMarkedOnTheList(t *testing.T) {
	source := &fakeSource{snapshot: pulseSnapshot()}
	model := typeKeys(openApplications(t, source), " ") // marks grafana, which the overview does not list
	model = command(model, ":pulse")

	view := press(model, "s").View()

	if !strings.Contains(view, "Sync checkout?") {
		t.Fatalf("sync should ask about the selected row, not the marks on the list:\n%s", view)
	}
}
