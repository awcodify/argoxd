package tui

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/awcodify/argoxd/internal/explorer"
)

type policyCall struct {
	application string
	policy      explorer.SyncPolicy
}

func (f *fakeSource) SetSyncPolicy(_ context.Context, application string, policy explorer.SyncPolicy) error {
	f.policies = append(f.policies, policyCall{application, policy})
	return f.failures[application]
}

// noPolicySource can show Applications but cannot change their sync policy.
type noPolicySource struct{ *fakeSource }

func (noPolicySource) SetSyncPolicy() {}

func policySnapshot() explorer.Snapshot {
	snapshot := storeSnapshot()
	snapshot.Applications[0].Policy = explorer.SyncPolicy{Automated: true, SelfHeal: true, Prune: true} // grafana
	return snapshot
}

func TestPAsksForTheSyncPolicyOfTheSelectedApplication(t *testing.T) {
	source := &fakeSource{snapshot: policySnapshot()}
	model := openApplications(t, source) // grafana is selected

	view := press(model, "P").View()

	for _, want := range []string{"Set the sync policy of grafana?", "auto-sync ● on", "self-heal ● on", "prune ● on", "enter", "esc"} {
		if !strings.Contains(view, want) {
			t.Fatalf("the dialog does not contain %q:\n%s", want, view)
		}
	}
	if len(source.policies) != 0 {
		t.Fatalf("changed the policy before confirming: %v", source.policies)
	}
}

func TestSyncPolicyDialogWarnsThatRollbackNeedsAutoSyncOff(t *testing.T) {
	model := press(openApplications(t, &fakeSource{snapshot: policySnapshot()}), "P")

	if view := model.View(); !strings.Contains(view, "rollback") {
		t.Fatalf("the dialog does not mention rollback:\n%s", view)
	}
}

func TestEnterAppliesTheTogglesChosenInTheSyncPolicyDialog(t *testing.T) {
	source := &fakeSource{snapshot: policySnapshot()}
	model := press(openApplications(t, source), "P")

	model = press(press(model, "h"), "p") // self-heal and prune off, auto-sync stays on
	run(model, "enter")

	want := []policyCall{{"grafana", explorer.SyncPolicy{Automated: true}}}
	if len(source.policies) != 1 || source.policies[0] != want[0] {
		t.Fatalf("policies = %+v, want %+v", source.policies, want)
	}
}

func TestTurningAutoSyncOffInTheDialogDropsSelfHealAndPrune(t *testing.T) {
	source := &fakeSource{snapshot: policySnapshot()}
	model := press(openApplications(t, source), "P")

	model = press(model, "a")
	run(model, "enter")

	if want := (policyCall{"grafana", explorer.SyncPolicy{}}); len(source.policies) != 1 || source.policies[0] != want {
		t.Fatalf("policies = %+v, want %+v", source.policies, want)
	}
}

func TestSelfHealAndPruneCannotBeToggledWhileAutoSyncIsOff(t *testing.T) {
	source := &fakeSource{snapshot: storeSnapshot()}
	model := press(press(openApplications(t, source), "j"), "P") // checkout syncs manually

	model = press(press(model, "h"), "p")

	if view := model.View(); strings.Contains(view, "● on") {
		t.Fatalf("self-heal or prune turned on without auto-sync:\n%s", view)
	}
	run(model, "enter")
	if want := (policyCall{"checkout", explorer.SyncPolicy{}}); len(source.policies) != 1 || source.policies[0] != want {
		t.Fatalf("policies = %+v, want %+v", source.policies, want)
	}
}

func TestTurningAutoSyncOnThenSelfHealApplies(t *testing.T) {
	source := &fakeSource{snapshot: storeSnapshot()}
	model := press(press(openApplications(t, source), "j"), "P")

	run(press(press(model, "a"), "h"), "enter")

	if want := (policyCall{"checkout", explorer.SyncPolicy{Automated: true, SelfHeal: true}}); len(source.policies) != 1 || source.policies[0] != want {
		t.Fatalf("policies = %+v, want %+v", source.policies, want)
	}
}

func TestEscCancelsTheSyncPolicyDialog(t *testing.T) {
	source := &fakeSource{snapshot: policySnapshot()}
	model := press(press(openApplications(t, source), "P"), "a")

	model = press(model, "esc")

	if len(source.policies) != 0 || strings.Contains(model.View(), "Set the sync policy") {
		t.Fatalf("esc did not cancel the dialog:\n%s", model.View())
	}
}

func TestPAppliesToEveryMarkedApplication(t *testing.T) {
	source := &fakeSource{snapshot: policySnapshot()}
	model := typeKeys(openApplications(t, source), " ", " ", "P")

	view := model.View()
	if !strings.Contains(view, "Set the sync policy of 2 applications?") || !strings.Contains(view, "• checkout") || !strings.Contains(view, "• grafana") {
		t.Fatalf("the dialog does not list both applications:\n%s", view)
	}
	run(press(model, "h"), "enter")

	if len(source.policies) != 2 {
		t.Fatalf("policies = %+v, want one change per application", source.policies)
	}
	for _, call := range source.policies {
		if call.policy.SelfHeal {
			t.Fatalf("policies = %+v, want self-heal turned off for every application", source.policies)
		}
	}
}

func TestPInTheDependencyViewChangesTheOpenApplication(t *testing.T) {
	source := &fakeSource{snapshot: storeSnapshot(), tree: checkoutTree()}
	model := press(openCheckout(t, source), "P")

	run(press(model, "a"), "enter")

	if want := (policyCall{"checkout", explorer.SyncPolicy{Automated: true}}); len(source.policies) != 1 || source.policies[0] != want {
		t.Fatalf("policies = %+v, want %+v", source.policies, want)
	}
}

func TestSyncPolicyFailureIsReported(t *testing.T) {
	source := &fakeSource{snapshot: policySnapshot(), failures: map[string]error{"grafana": errors.New("boom")}}
	model := run(press(openApplications(t, source), "P"), "enter")

	if view := model.View(); !strings.Contains(view, "boom") {
		t.Fatalf("the failure is not reported:\n%s", view)
	}
}

func TestPReportsASourceThatCannotChangeTheSyncPolicy(t *testing.T) {
	source := noPolicySource{&fakeSource{snapshot: policySnapshot()}}
	model := resize(New(source, "test", "argocd", explorer.Snapshot{}), 140, 40)
	model = settle(model, source.loadCommand())
	model = run(press(model, "P"), "enter")

	if view := model.View(); !strings.Contains(view, "cannot change the sync policy") {
		t.Fatalf("the missing support is not reported:\n%s", view)
	}
}

func TestKeyHintsListSyncPolicy(t *testing.T) {
	list := resize(New(nil, "test", "argocd", storeSnapshot()), 140, 30).View()
	tree := openCheckout(t, &fakeSource{snapshot: storeSnapshot(), tree: checkoutTree()}).View()

	for name, view := range map[string]string{"list": list, "dependency view": tree} {
		if !strings.Contains(view, "Sync policy") {
			t.Fatalf("%s does not hint at the sync policy:\n%s", name, view)
		}
	}
}
