package tui

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/awcodify/argoxd/internal/argocd"
	"github.com/awcodify/argoxd/internal/explorer"
)

type rollbackCall struct {
	application string
	id          int64
	options     argocd.SyncOptions
}

func (f *fakeSource) ApplicationHistory(context.Context, string) ([]explorer.HistoryEntry, error) {
	return f.history, nil
}

func (f *fakeSource) RollbackApplication(_ context.Context, application string, id int64, options argocd.SyncOptions) error {
	f.rolledBack = append(f.rolledBack, rollbackCall{application, id, options})
	return nil
}

// sampleHistory is two deployments of grafana, newest first.
func sampleHistory() []explorer.HistoryEntry {
	return []explorer.HistoryEntry{
		{ID: 2, Revision: "bbb2222", DeployedAt: time.Now().Add(-2 * time.Hour), Repo: "https://example.com/grafana.git"},
		{ID: 1, Revision: "aaa1111", DeployedAt: time.Now().Add(-50 * time.Hour), Repo: "https://example.com/grafana.git"},
	}
}

func openHistory(t *testing.T, source *fakeSource) Model {
	t.Helper()
	model := resize(New(source, "test", "argocd", explorer.Snapshot{}), 140, 40)
	model = settle(model, source.loadCommand())
	model = run(model, "h")
	if !strings.Contains(model.View(), "history") {
		t.Fatalf("h did not open the history:\n%s", model.View())
	}
	return model
}

func TestHOpensTheApplicationHistoryNewestFirst(t *testing.T) {
	source := &fakeSource{snapshot: storeSnapshot(), history: sampleHistory()}

	view := openHistory(t, source).View()

	for _, want := range []string{"history", "grafana", "REVISION", "DEPLOYED", "bbb2222", "aaa1111", "2h", "2d", "current"} {
		if !strings.Contains(view, want) {
			t.Fatalf("history does not contain %q:\n%s", want, view)
		}
	}
	if strings.Index(view, "bbb2222") > strings.Index(view, "aaa1111") {
		t.Fatalf("history is not newest first:\n%s", view)
	}
}

func TestEnterRollsBackToTheSelectedDeploymentWithTheChosenOptions(t *testing.T) {
	source := &fakeSource{snapshot: storeSnapshot(), history: sampleHistory()}
	model := press(openHistory(t, source), "j")

	asking := press(model, "enter")
	if view := asking.View(); !strings.Contains(view, "Roll back grafana to aaa1111?") {
		t.Fatalf("enter did not ask to confirm the rollback:\n%s", view)
	}
	if len(source.rolledBack) != 0 {
		t.Fatalf("rolled back before confirming: %+v", source.rolledBack)
	}

	run(typeKeys(asking, "p"), "enter")

	want := rollbackCall{"grafana", 1, argocd.SyncOptions{Prune: true}}
	if len(source.rolledBack) != 1 || source.rolledBack[0] != want {
		t.Fatalf("rolledBack = %+v, want %+v", source.rolledBack, want)
	}
}

func TestEscCancelsTheRollbackThenClosesTheHistory(t *testing.T) {
	source := &fakeSource{snapshot: storeSnapshot(), history: sampleHistory()}
	asking := press(openHistory(t, source), "enter")

	cancelled := press(asking, "esc")
	if view := cancelled.View(); strings.Contains(view, "Roll back grafana") || !strings.Contains(view, "DEPLOYED") {
		t.Fatalf("esc did not cancel only the dialog:\n%s", view)
	}
	if len(source.rolledBack) != 0 {
		t.Fatalf("cancelling still rolled back: %+v", source.rolledBack)
	}

	if view := press(cancelled, "esc").View(); !strings.Contains(view, "applications ·") || strings.Contains(view, "DEPLOYED") {
		t.Fatalf("esc did not return to the applications list:\n%s", view)
	}
}

func TestEscFromTheHistoryReturnsToTheDependencies(t *testing.T) {
	source := &fakeSource{snapshot: storeSnapshot(), tree: checkoutTree(), history: sampleHistory()}

	model := press(run(openCheckout(t, source), "h"), "esc")

	if view := model.View(); !strings.Contains(view, "applications › checkout") || strings.Contains(view, "DEPLOYED") {
		t.Fatalf("esc did not return to the dependency view:\n%s", view)
	}
}

func TestHistoryNeedsASourceThatSupportsIt(t *testing.T) {
	model := New(nil, "test", "argocd", storeSnapshot())

	view := press(model, "h").View()

	if !strings.Contains(view, "cannot show or roll back") {
		t.Fatalf("h did not explain the missing support:\n%s", view)
	}
}

func TestHeaderHintsListHistory(t *testing.T) {
	source := &fakeSource{snapshot: storeSnapshot(), tree: checkoutTree()}
	list := settle(resize(New(source, "test", "argocd", explorer.Snapshot{}), 160, 40), source.loadCommand())

	for name, view := range map[string]string{"list": list.View(), "dependencies": openCheckout(t, source).View()} {
		if !strings.Contains(view, "h  History") {
			t.Fatalf("%s header does not list the history key:\n%s", name, view)
		}
	}
}

func TestShortRevisionAbbreviatesCommitSHAsOnly(t *testing.T) {
	for revision, want := range map[string]string{
		"4be7d10a9f2c1ab4be7d10a9f2c1ab4be7d10a9f": "4be7d10",
		"4be7d10": "4be7d10",
		"release-2026-10-01-hotfix-for-the-cart-service": "release-2026-10-01-hotfix-for-the-cart-service",
		"": "",
	} {
		if got := shortRevision(revision); got != want {
			t.Errorf("shortRevision(%q) = %q, want %q", revision, got, want)
		}
	}
}
