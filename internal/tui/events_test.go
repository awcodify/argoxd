package tui

import (
	"context"
	"strings"
	"testing"

	"github.com/awcodify/argoxd/internal/argocd"
	"github.com/awcodify/argoxd/internal/explorer"
)

const sampleEvents = "AGE  TYPE     REASON   OBJECT       MESSAGE\n2m   Warning  BackOff  Pod/web-abc  Back-off restarting failed container\n3h   Normal   Started  Pod/web-abc  Started container app"

var _ argocd.EventLister = (*fakeSource)(nil)

func (f *fakeSource) ApplicationEvents(_ context.Context, application string) (string, error) {
	f.eventRequests = append(f.eventRequests, "application/"+application)
	return f.events, nil
}

func (f *fakeSource) ResourceEvents(_ context.Context, _ string, resource explorer.ResourceNode) (string, error) {
	names := resource.Kind + "/" + resource.Name
	for _, child := range resource.Children {
		names += "+" + child.Kind + "/" + child.Name
	}
	f.eventRequests = append(f.eventRequests, names)
	return f.events, nil
}

func TestEOpensTheEventsOfTheSelectedCardAndItsChildren(t *testing.T) {
	source := &fakeSource{snapshot: storeSnapshot(), tree: checkoutTree(), events: sampleEvents}
	model := openCheckout(t, source)

	model = run(model, "j")
	model = run(model, "e")

	view := model.View()
	if !strings.Contains(view, "events") || !strings.Contains(view, "Back-off restarting failed container") {
		t.Fatalf("e did not show the events:\n%s", view)
	}
	if len(source.eventRequests) != 1 || source.eventRequests[0] != "Deployment/web+Pod/web-abc" {
		t.Fatalf("event requests = %v, want the Deployment with its Pod", source.eventRequests)
	}
}

func TestEOnTheApplicationCardShowsTheApplicationsEvents(t *testing.T) {
	source := &fakeSource{snapshot: storeSnapshot(), tree: checkoutTree(), events: sampleEvents}
	model := run(openCheckout(t, source), "e")

	if len(source.eventRequests) != 1 || source.eventRequests[0] != "application/checkout" {
		t.Fatalf("event requests = %v, want the Application's", source.eventRequests)
	}
	if !strings.Contains(model.View(), "BackOff") {
		t.Fatalf("events are not shown:\n%s", model.View())
	}
}

func TestEOnTheApplicationListShowsTheSelectedApplicationsEvents(t *testing.T) {
	source := &fakeSource{snapshot: storeSnapshot(), events: sampleEvents}
	model := resize(New(source, "test", "argocd", explorer.Snapshot{}), 140, 40)
	model = settle(model, source.loadCommand())
	model = press(model, "j")

	model = run(model, "e")

	if len(source.eventRequests) != 1 || source.eventRequests[0] != "application/checkout" {
		t.Fatalf("event requests = %v, want checkout's", source.eventRequests)
	}
	if !strings.Contains(model.View(), "BackOff") {
		t.Fatalf("events are not shown:\n%s", model.View())
	}
	if back := press(model, "esc").View(); !strings.Contains(back, "grafana") || strings.Contains(back, "BackOff") {
		t.Fatalf("esc did not return to the list:\n%s", back)
	}
}

func TestEscLeavesTheEventsForTheDependencies(t *testing.T) {
	source := &fakeSource{snapshot: storeSnapshot(), tree: checkoutTree(), events: sampleEvents}
	model := run(run(openCheckout(t, source), "j"), "e")

	view := press(model, "esc").View()

	if strings.Contains(view, "BackOff") || !strings.Contains(view, "Kind       Deployment") {
		t.Fatalf("esc did not return to the dependencies:\n%s", view)
	}
}

func TestSlashKeepsTheEventLinesThatMatch(t *testing.T) {
	source := &fakeSource{snapshot: storeSnapshot(), tree: checkoutTree(), events: sampleEvents}
	model := run(run(openCheckout(t, source), "j"), "e")

	view := typeKeys(model, "/", "w", "a", "r", "n").View()
	if strings.Contains(view, "Started container") {
		t.Fatalf("search did not narrow the events:\n%s", view)
	}

	view = typeKeys(model, "/", "w", "a", "r", "n", "enter").View()
	if !strings.Contains(view, "Warning") {
		t.Fatalf("search lost the matching event:\n%s", view)
	}
}

func TestEventsNeedASourceThatListsThem(t *testing.T) {
	source := &fakeSource{snapshot: storeSnapshot(), tree: checkoutTree()}
	model := openCheckout(t, source)
	model.source = struct {
		argocd.Source
		argocd.ApplicationOperator
	}{source, source}

	model = run(model, "e")

	if !strings.Contains(model.View(), "cannot list events") {
		t.Fatalf("a source without events was not reported:\n%s", model.View())
	}
}
