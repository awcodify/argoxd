package tui

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/awcodify/argoxd/internal/explorer"
)

// workloadsTree has two Deployments, each with a Pod, in the order
// web, web-abc, api, api-xyz (cards 1 to 4 under the Application card).
func workloadsTree() explorer.ResourceTree {
	deployment := func(name string) explorer.ResourceNode {
		return explorer.ResourceNode{Group: "apps", Version: "v1", Kind: "Deployment", Namespace: "store", Name: name, Health: "Healthy"}
	}
	pod := func(name, owner string) explorer.ResourceNode {
		return explorer.ResourceNode{Version: "v1", Kind: "Pod", Namespace: "store", Name: name, Health: "Healthy",
			Parents: []explorer.ResourceReference{{Group: "apps", Kind: "Deployment", Namespace: "store", Name: owner}}}
	}
	return explorer.ResourceTree{Application: "checkout", Nodes: []explorer.ResourceNode{
		deployment("web"), pod("web-abc", "web"), deployment("api"), pod("api-xyz", "api"),
	}}
}

func openWorkloads(t *testing.T) (*fakeSource, Model) {
	t.Helper()
	source := &fakeSource{snapshot: storeSnapshot(), tree: workloadsTree()}
	return source, openCheckout(t, source)
}

// markCards marks the cards at the given positions under the Application card.
func markCards(model Model, positions ...int) Model {
	for _, position := range positions {
		for model.treeCursor < position {
			model = press(model, "j")
		}
		model = press(model, " ")
	}
	return model
}

func TestTheSyncDialogListsEveryApplicationItWillSync(t *testing.T) {
	source := &fakeSource{snapshot: storeSnapshot()}
	model := typeKeys(openApplications(t, source), " ", " ", "s")

	view := model.View()

	for _, want := range []string{"Sync 2 applications?", "• checkout", "• grafana", "OutOfSync"} {
		if !strings.Contains(view, want) {
			t.Fatalf("the dialog does not contain %q:\n%s", want, view)
		}
	}
	if len(source.syncedApps) != 0 {
		t.Fatalf("synced before confirming: %v", source.syncedApps)
	}
}

func TestTheSelectiveSyncDialogListsTheResourcesAndTheirApplication(t *testing.T) {
	_, model := openWorkloads(t)

	view := press(markCards(model, 1, 4), "s").View()

	for _, want := range []string{"Sync 2 resources of checkout?", "• Deployment/web", "• Pod/api-xyz", "store"} {
		if !strings.Contains(view, want) {
			t.Fatalf("the dialog does not contain %q:\n%s", want, view)
		}
	}
	if strings.Contains(view, "Pod/web-abc") {
		t.Fatalf("the dialog lists a resource that is not marked:\n%s", view)
	}
}

func TestEveryActionListsItsTargetBeforeRunning(t *testing.T) {
	source := &fakeSource{snapshot: storeSnapshot(), history: sampleHistory()}
	list := openApplications(t, source)
	history := press(openHistory(t, source), "j")
	_, workloads := openWorkloads(t)

	for name, test := range map[string]struct {
		model Model
		key   string
		want  []string
	}{
		"sync":         {list, "s", []string{"Sync grafana?", "• grafana"}},
		"hard refresh": {list, "R", []string{"Hard refresh grafana?", "• grafana"}},
		"delete":       {list, "D", []string{"Delete grafana and its managed resources?", "• grafana"}},
		"rollback":     {history, "enter", []string{"Roll back grafana to aaa1111?", "• aaa1111"}},
		"restart":      {press(workloads, "j"), "x", []string{"Restart Deployment/web?", "• Deployment/web"}},
		"delete pod":   {press(press(workloads, "j"), "j"), "X", []string{"Delete Pod/web-abc?", "• Pod/web-abc"}},
	} {
		view := press(test.model, test.key).View()
		for _, want := range test.want {
			if !strings.Contains(view, want) {
				t.Errorf("%s: the dialog does not contain %q:\n%s", name, want, view)
			}
		}
	}
}

func TestHardRefreshAsksFirstAndEscCancels(t *testing.T) {
	source := &fakeSource{snapshot: storeSnapshot()}
	model := press(openApplications(t, source), "R")

	if view := model.View(); !strings.Contains(view, "Hard refresh grafana?") {
		t.Fatalf("R did not ask first:\n%s", view)
	}
	run(model, "esc")

	if len(source.refreshed) != 0 {
		t.Fatalf("refreshed = %v after cancelling", source.refreshed)
	}
}

func TestMarkedApplicationsAreHardRefreshedAndDeletedTogether(t *testing.T) {
	for _, test := range []struct {
		key, question string
		done          func(*fakeSource) []string
	}{
		{"R", "Hard refresh 2 applications?", func(s *fakeSource) []string { return s.refreshed }},
		{"D", "Delete 2 applications and their managed resources?", func(s *fakeSource) []string { return s.deletedApps }},
	} {
		source := &fakeSource{snapshot: storeSnapshot()}
		model := press(typeKeys(openApplications(t, source), " ", " "), test.key)

		if view := model.View(); !strings.Contains(view, test.question) || !strings.Contains(view, "• grafana") || !strings.Contains(view, "• checkout") {
			t.Fatalf("%s: the dialog is wrong:\n%s", test.key, view)
		}
		model = run(model, "y")

		done := test.done(source)
		slices.Sort(done)
		if !slices.Equal(done, []string{"checkout", "grafana"}) {
			t.Fatalf("%s acted on %v, want both", test.key, done)
		}
		if view := model.View(); strings.Count(view, "●") > 1 {
			t.Fatalf("%s: marks outlived the action:\n%s", test.key, view)
		}
	}
}

func TestMarkedWorkloadsAreRestartedTogether(t *testing.T) {
	source, model := openWorkloads(t)
	model = press(markCards(model, 1, 3), "x")

	if view := model.View(); !strings.Contains(view, "Restart 2 workloads?") || !strings.Contains(view, "• Deployment/api") || !strings.Contains(view, "• Deployment/web") {
		t.Fatalf("the dialog is wrong:\n%s", view)
	}
	model = run(model, "enter")

	var names []string
	for _, node := range source.restarted {
		names = append(names, node.Name)
	}
	if !slices.Equal(names, []string{"web", "api"}) {
		t.Fatalf("restarted %v, want web then api", names)
	}
	if view := model.View(); !strings.Contains(view, "restart requested for 2 resources") {
		t.Fatalf("the restart was not reported:\n%s", view)
	}
}

func TestMarkedPodsAreDeletedTogether(t *testing.T) {
	source, model := openWorkloads(t)
	model = press(markCards(model, 2, 4), "X")

	if view := model.View(); !strings.Contains(view, "Delete 2 pods?") || !strings.Contains(view, "• Pod/web-abc") || !strings.Contains(view, "• Pod/api-xyz") {
		t.Fatalf("the dialog is wrong:\n%s", view)
	}
	run(model, "y")

	if len(source.deleted) != 2 || source.deleted[0].Name != "web-abc" || source.deleted[1].Name != "api-xyz" {
		t.Fatalf("deleted %+v, want both Pods", source.deleted)
	}
}

func TestMarkedCardsThatCannotTakeTheActionAreNamedAndNothingRuns(t *testing.T) {
	for _, test := range []struct{ key, want string }{
		{"x", "Pod/web-abc cannot be restarted"},
		{"X", "Deployment/web cannot be deleted here"},
	} {
		source, model := openWorkloads(t)
		model = press(markCards(model, 1, 2), test.key)

		view := model.View()
		if !strings.Contains(view, test.want) || strings.Contains(view, "?   enter") {
			t.Fatalf("%s: the refusal is missing %q:\n%s", test.key, test.want, view)
		}
		run(model, "y")
		if len(source.restarted)+len(source.deleted) != 0 {
			t.Fatalf("%s acted on a mixed selection", test.key)
		}
	}
}

func TestAFailureInTheMiddleIsReportedAndTheRestStillRun(t *testing.T) {
	source := &fakeSource{snapshot: storeSnapshot(), failures: map[string]error{"checkout": errors.New("denied")}}
	model := run(press(typeKeys(openApplications(t, source), " ", " "), "D"), "y")

	view := model.View()
	if len(source.deletedApps) != 2 || !strings.Contains(view, "1 of 2") || !strings.Contains(view, "checkout: denied") {
		t.Fatalf("deleted %v:\n%s", source.deletedApps, view)
	}
}

func TestLongTargetListsAreCutShort(t *testing.T) {
	names := make([]string, 80)
	snapshot := explorer.Snapshot{}
	for index := range names {
		snapshot.Applications = append(snapshot.Applications, explorer.Application{Name: fmt.Sprintf("app-%02d", index), Sync: "Synced", Health: "Healthy"})
	}
	source := &fakeSource{snapshot: snapshot}
	model := openApplications(t, source)
	for range names {
		model = press(model, " ")
	}

	view := press(model, "s").View()

	if !strings.Contains(view, "Sync 80 applications?") || !strings.Contains(view, "more") || !strings.Contains(view, "enter") {
		t.Fatalf("the long list pushed the dialog's keys away:\n%s", view)
	}
}

func TestYAndEnterBothConfirm(t *testing.T) {
	for _, confirm := range []string{"y", "enter"} {
		source := &fakeSource{snapshot: storeSnapshot()}
		run(press(openApplications(t, source), "D"), confirm)

		if len(source.deletedApps) != 1 {
			t.Fatalf("%s did not confirm: deleted %v", confirm, source.deletedApps)
		}
	}
}
