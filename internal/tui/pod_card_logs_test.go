package tui

import (
	"strings"
	"testing"

	"github.com/awcodify/argoxd/internal/argocd"
	"github.com/awcodify/argoxd/internal/explorer"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// podCard selects the Pod web-abc, the first Pod under the Deployment.
func podCard(t *testing.T, source *fakeSource, jumps int) Model {
	t.Helper()
	model := resize(openCheckout(t, source), 140, 16)
	for range jumps {
		model = press(model, "j")
	}
	if !strings.Contains(model.View(), "Kind       Pod") {
		t.Fatalf("the Pod card is not selected:\n%s", model.View())
	}
	return model
}

// lastStreamed is the resource of the latest stream the source was asked for.
func lastStreamed(t *testing.T, source *fakeSource) explorer.ResourceNode {
	t.Helper()
	if len(source.streamedPods) == 0 {
		t.Fatal("no stream was started")
	}
	return source.streamedPods[len(source.streamedPods)-1]
}

// openPodCardLog presses l on the Pod card and delivers the start of its stream.
// The returned command waits for the next line.
func openPodCardLog(t *testing.T, model Model) (Model, tea.Cmd) {
	t.Helper()
	updated, command := model.Update(key("l"))
	if command == nil {
		t.Fatalf("l on the Pod did not start a stream:\n%s", updated.View())
	}
	return stepT(t, updated.(Model), command)
}

func TestLOnAPodUnderAWorkloadFollowsTheWorkloadsLogFilteredToThatPod(t *testing.T) {
	source := podSource()
	model, next := openPodCardLog(t, podCard(t, source, 2))
	source.stream <- argocd.LogEntry{Pod: "web-abc", Container: "app", Line: "hello"}
	model, _ = stepT(t, model, next)

	last := lastStreamed(t, source)
	if last.Kind != "Pod" || last.Name != "web-abc" || last.Namespace != "store" {
		t.Fatalf("streamed resource = %+v, want the Pod web-abc", last)
	}
	view := model.View()
	for _, want := range []string{"Deployment/web", "pod: web-abc", "● following", "hello"} {
		if !strings.Contains(view, want) {
			t.Fatalf("the Pod's log does not contain %q:\n%s", want, view)
		}
	}
	if strings.Contains(view, "web-abc │") {
		t.Fatalf("a single Pod's lines were prefixed with its name:\n%s", view)
	}
}

func TestPInAPodsLogListsItsSiblingsStartingOnThePod(t *testing.T) {
	source := podSource()
	model, _ := openPodCardLog(t, podCard(t, source, 2))
	model = podBar(t, model)

	if view := model.View(); !strings.Contains(view, "web-def") || !strings.Contains(view, "all") {
		t.Fatalf("the pod bar does not list the sibling pods:\n%s", view)
	}

	if _, command := model.Update(key("enter")); command != nil || len(source.streamedPods) != 1 {
		t.Fatalf("choosing the current pod restarted the stream (%d streams)", len(source.streamedPods))
	}

	updated, command := press(model, "tab").Update(key("enter"))
	stepT(t, updated.(Model), command)
	if last := lastStreamed(t, source); last.Name != "web-def" {
		t.Fatalf("streamed resource = %+v, want the sibling web-def", last)
	}
}

func TestChoosingAllInAPodsLogShowsTheWholeWorkload(t *testing.T) {
	source := podSource()
	model, _ := openPodCardLog(t, podCard(t, source, 2))
	model = typeKeys(podBar(t, model), "a", "l")

	updated, command := model.Update(key("enter"))
	model, _ = stepT(t, updated.(Model), command)

	last := lastStreamed(t, source)
	if last.Kind != "Deployment" || last.Name != "web" {
		t.Fatalf("streamed resource = %+v, want the Deployment", last)
	}
	if view := model.View(); strings.Contains(view, "pod:") || !strings.Contains(view, "● following") {
		t.Fatalf("the log is not showing the whole workload:\n%s", view)
	}
}

func TestEscFromAPodsLogReturnsToItsCard(t *testing.T) {
	source := podSource()
	model, _ := openPodCardLog(t, podCard(t, source, 2))

	back := press(model, "esc")

	if view := back.View(); !strings.Contains(view, "Kind       Pod") || !strings.Contains(view, "applications › checkout") || strings.Contains(view, "following") {
		t.Fatalf("esc did not return to the Pod's card:\n%s", view)
	}
	if source.streamCtx == nil || source.streamCtx.Err() == nil {
		t.Fatal("leaving the log did not stop the stream")
	}
}

func TestAPodUnderAReplicaSetUsesTheDeploymentAsItsWorkload(t *testing.T) {
	deployment := explorer.ResourceReference{Group: "apps", Kind: "Deployment", Namespace: "store", Name: "web"}
	replicaSet := explorer.ResourceReference{Group: "apps", Kind: "ReplicaSet", Namespace: "store", Name: "web-5d8f7c"}
	source := podSource()
	source.tree = explorer.ResourceTree{Application: "checkout", Nodes: []explorer.ResourceNode{
		{Group: "apps", Version: "v1", Kind: "Deployment", Namespace: "store", Name: "web"},
		{Group: "apps", Version: "v1", Kind: "ReplicaSet", Namespace: "store", Name: "web-5d8f7c", Parents: []explorer.ResourceReference{deployment}},
		{Version: "v1", Kind: "Pod", Namespace: "store", Name: "web-abc", Parents: []explorer.ResourceReference{replicaSet}},
	}}
	model, _ := openPodCardLog(t, podCard(t, source, 3))
	model = typeKeys(podBar(t, model), "a", "l")

	updated, command := model.Update(key("enter"))
	stepT(t, updated.(Model), command)

	if last := lastStreamed(t, source); last.Kind != "Deployment" || last.Name != "web" {
		t.Fatalf("streamed resource = %+v, want the Deployment above the ReplicaSet", last)
	}
}

func TestAPodNoWorkloadOwnsStillShowsItsSnapshot(t *testing.T) {
	source := &fakeSource{snapshot: storeSnapshot(), tree: barePodTree(), logs: "starting\nready", stream: lines("never")}
	model := resize(openCheckout(t, source), 140, 16)
	model = run(press(press(model, "j"), "j"), "l")

	if view := model.View(); !strings.Contains(view, "ready") || strings.Contains(view, "following") || len(source.streamedPods) != 0 {
		t.Fatalf("a bare Pod's log was not its snapshot:\n%s", view)
	}
}

func TestAPodLogFallsBackToItsSnapshotWhenTheSourceCannotStream(t *testing.T) {
	source := &logsOnlySource{fakeSource: &fakeSource{snapshot: storeSnapshot(), tree: twoPodTree(), logs: "the snapshot"}}
	model := resize(New(source, "test", "argocd", explorer.Snapshot{}), 140, 16)
	model = settle(model, source.loadCommand())
	model = press(press(run(press(model, "j"), "enter"), "j"), "j")

	model = run(model, "l")

	if view := model.View(); !strings.Contains(view, "the snapshot") || strings.Contains(view, "cannot follow") {
		t.Fatalf("the log did not fall back to the snapshot:\n%s", view)
	}
}

func TestTheContainerChoiceWorksInAPodCardsLog(t *testing.T) {
	source := podSource()
	model, _ := openPodCardLog(t, podCard(t, source, 2))

	updated, command := model.Update(key("c"))
	model, _ = stepT(t, updated.(Model), command) // the containers arrive and the bar opens
	updated, command = press(model, "tab").Update(key("enter"))
	stepT(t, updated.(Model), command)

	last := lastStreamed(t, source)
	if last.Name != "web-abc" || source.streamedContainers[len(source.streamedContainers)-1] != "sidecar" {
		t.Fatalf("restart streamed %+v with container %q, want web-abc and the sidecar", last, source.streamedContainers)
	}
}

// longPodCardBar opens the pod bar from the log of the first Pod under a Deployment with long names.
func longPodCardBar(t *testing.T, width int) Model {
	t.Helper()
	source := podSource()
	source.tree = longNameTree()
	model := resize(openCheckout(t, source), width, 16)
	model = press(press(model, "j"), "j")
	model, _ = openPodCardLog(t, model)
	return podBar(t, model)
}

func barRow(view string) string {
	for _, line := range strings.Split(view, "\n") {
		if strings.Contains(line, "▾") {
			return line
		}
	}
	return ""
}

func TestThePodBarShowsEverySiblingAndAllWhenALongPodIsSelected(t *testing.T) {
	row := barRow(longPodCardBar(t, 170).View())

	for _, want := range []string{"mvp75", "zjvlk", " all"} {
		if !strings.Contains(row, want) {
			t.Fatalf("the bar lost %q:\n%s", want, row)
		}
	}
	if count := strings.Count(row, "587b857675-mvp75"); count != 1 {
		t.Fatalf("the selected pod is shown %d times, want once:\n%s", count, row)
	}
}

func TestThePodBarScrollsItsChipsToKeepTheSelectedOneVisible(t *testing.T) {
	model := longPodCardBar(t, 100)

	for step, want := range []string{"mvp75", "zjvlk", "all"} {
		row := barRow(model.View())
		if !strings.Contains(row, want) {
			t.Fatalf("after %d tabs the selected %q is not visible:\n%s", step, want, row)
		}
		if got := lipgloss.Width(row); got > 100 {
			t.Fatalf("after %d tabs the bar is %d columns wide, want at most 100:\n%s", step, got, row)
		}
		if !strings.Contains(row, "…") {
			t.Fatalf("after %d tabs the bar does not show that chips are hidden:\n%s", step, row)
		}
		model = press(model, "tab")
	}
}
