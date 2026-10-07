package tui

import (
	"strings"
	"testing"

	"github.com/awcodify/argoxd/internal/explorer"
)

func snapshotWithConditions(conditions ...explorer.Condition) explorer.Snapshot {
	snapshot := storeSnapshot()
	snapshot.Applications[1].Conditions = conditions
	return snapshot
}

func TestApplicationsListMarksApplicationsWithConditions(t *testing.T) {
	model := resize(New(nil, "test", "argocd", snapshotWithConditions(explorer.Condition{Type: "SyncError", Message: "boom"})), 140, 30)

	view := model.View()

	for _, line := range strings.Split(view, "\n") {
		if strings.Contains(line, "checkout") && !strings.Contains(line, conditionGlyph) {
			t.Fatalf("checkout has conditions but its row has no %s:\n%s", conditionGlyph, view)
		}
		if strings.Contains(line, "grafana") && strings.Contains(line, conditionGlyph) {
			t.Fatalf("grafana has no conditions but its row has a %s:\n%s", conditionGlyph, view)
		}
	}
}

func TestDependencyViewShowsEachConditionAboveTheCards(t *testing.T) {
	source := &fakeSource{
		snapshot: snapshotWithConditions(
			explorer.Condition{Type: "ComparisonError", Message: "repository not found"},
			explorer.Condition{Type: "SyncError", Message: "one or more objects failed to apply"},
		),
		tree: checkoutTree(),
	}

	view := openCheckout(t, source).View()

	for _, want := range []string{"ComparisonError", "repository not found", "SyncError", "one or more objects failed to apply"} {
		if !strings.Contains(view, want) {
			t.Fatalf("dependency view does not show %q:\n%s", want, view)
		}
	}
	if banner, card := strings.Index(view, "ComparisonError"), strings.Index(view, "Deployment"); banner > card {
		t.Fatalf("conditions should come before the cards:\n%s", view)
	}
}

func TestDependencyViewSummarizesConditionsBeyondTheFirstThree(t *testing.T) {
	var conditions []explorer.Condition
	for _, name := range []string{"ErrorOne", "ErrorTwo", "ErrorThree", "ErrorFour", "ErrorFive"} {
		conditions = append(conditions, explorer.Condition{Type: name, Message: "failed"})
	}
	source := &fakeSource{snapshot: snapshotWithConditions(conditions...), tree: checkoutTree()}

	view := openCheckout(t, source).View()

	if !strings.Contains(view, "ErrorThree") || strings.Contains(view, "ErrorFour") {
		t.Fatalf("only the first three conditions should be listed:\n%s", view)
	}
	if !strings.Contains(view, "+2 more") {
		t.Fatalf("the other conditions are not counted:\n%s", view)
	}
}

func TestDependencyViewWithoutConditionsHasNoBanner(t *testing.T) {
	source := &fakeSource{snapshot: storeSnapshot(), tree: checkoutTree()}

	if view := openCheckout(t, source).View(); strings.Contains(view, conditionGlyph) {
		t.Fatalf("an application without conditions shows a banner:\n%s", view)
	}
}

func TestDependencyViewKeepsTheSelectedCardVisibleBelowTheBanner(t *testing.T) {
	source := &fakeSource{
		snapshot: snapshotWithConditions(explorer.Condition{Type: "SyncError", Message: "boom"}),
		tree:     checkoutTree(),
	}
	model := openCheckout(t, source)
	for range 3 {
		model = press(model, "j")
	}

	if view := model.View(); !strings.Contains(view, "Pod") {
		t.Fatalf("selected card scrolled out of view:\n%s", view)
	}
}
