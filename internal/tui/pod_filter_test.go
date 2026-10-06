package tui

import (
	"strings"
	"testing"

	"github.com/awcodify/argoxd/internal/argocd"
	"github.com/awcodify/argoxd/internal/explorer"
	"github.com/charmbracelet/lipgloss"
)

// twoPodTree is a Deployment with two Pods.
func twoPodTree() explorer.ResourceTree {
	deployment := explorer.ResourceReference{Group: "apps", Kind: "Deployment", Namespace: "store", Name: "web"}
	return explorer.ResourceTree{Application: "checkout", Nodes: []explorer.ResourceNode{
		{Group: "apps", Version: "v1", Kind: "Deployment", Namespace: "store", Name: "web", Health: "Healthy"},
		{Version: "v1", Kind: "Pod", Namespace: "store", Name: "web-abc", Health: "Healthy", Parents: []explorer.ResourceReference{deployment}},
		{Version: "v1", Kind: "Pod", Namespace: "store", Name: "web-def", Health: "Healthy", Parents: []explorer.ResourceReference{deployment}},
	}}
}

func podSource() *fakeSource {
	source := containerSource()
	source.tree = twoPodTree()
	source.stream = lines()
	return source
}

// followDeployment follows the logs of the Deployment, and returns the command that waits for the next line.
func followDeployment(t *testing.T, source *fakeSource) Model {
	t.Helper()
	return followFrom(t, resize(press(openCheckout(t, source), "j"), 120, 16), "l", 0)
}

func podBar(t *testing.T, model Model) Model {
	t.Helper()
	model = press(model, "p")
	if !model.prompt.active || model.prompt.field != "pod" {
		t.Fatalf("p did not open the pod bar:\n%s", model.View())
	}
	return model
}

func TestPOpensABarWithThePodsOfTheWorkloadAndAll(t *testing.T) {
	model := podBar(t, followDeployment(t, podSource()))

	view := model.View()
	for _, want := range []string{"▾", "web-abc", "web-def", "all"} {
		if !strings.Contains(view, want) {
			t.Fatalf("the pod bar does not contain %q:\n%s", want, view)
		}
	}
}

func TestTabChoosesAPodAndTheLogFollowsOnlyThatPod(t *testing.T) {
	source := podSource()
	model := podBar(t, followDeployment(t, source))
	oldStream := source.streamCtx

	updated, command := press(model, "tab").Update(key("enter"))
	model, next := stepT(t, updated.(Model), command)
	source.stream <- argocd.LogEntry{Pod: "web-abc", Container: "app", Line: "hello"}
	model, _ = stepT(t, model, next)

	if oldStream.Err() == nil {
		t.Fatal("the stream of all the pods was not cancelled")
	}
	last := source.streamedPods[len(source.streamedPods)-1]
	if last.Kind != "Pod" || last.Name != "web-abc" || last.Namespace != "store" {
		t.Fatalf("streamed resource = %+v, want the Pod web-abc", last)
	}
	view := model.View()
	if !strings.Contains(view, "pod: web-abc") || !strings.Contains(view, "hello") || strings.Contains(view, "web-abc │") || !strings.Contains(view, "● following") {
		t.Fatalf("the log is not following only web-abc:\n%s", view)
	}
}

func TestChoosingAllReturnsToTheWholeWorkload(t *testing.T) {
	source := podSource()
	model := press(podBar(t, followDeployment(t, source)), "tab")
	updated, command := model.Update(key("enter"))
	model, _ = stepT(t, updated.(Model), command)

	model = typeKeys(podBar(t, model), "a", "l")
	updated, command = model.Update(key("enter"))
	model, next := stepT(t, updated.(Model), command)
	source.stream <- argocd.LogEntry{Pod: "web-def", Container: "app", Line: "back"}
	model, _ = stepT(t, model, next)

	last := source.streamedPods[len(source.streamedPods)-1]
	if last.Kind != "Deployment" || last.Name != "web" {
		t.Fatalf("streamed resource = %+v, want the Deployment again", last)
	}
	if view := model.View(); strings.Contains(view, "pod:") || !strings.Contains(view, "web-def │ back") {
		t.Fatalf("all pods are not shown with their prefixes again:\n%s", view)
	}
}

func TestTypingAnyPartOfAPodNameChoosesIt(t *testing.T) {
	source := podSource()
	model := typeKeys(podBar(t, followDeployment(t, source)), "d", "e", "f")

	updated, command := model.Update(key("enter"))
	stepT(t, updated.(Model), command)

	if last := source.streamedPods[len(source.streamedPods)-1]; last.Name != "web-def" {
		t.Fatalf("streamed resource = %+v, want web-def", last)
	}
}

func TestAnUnknownPodIsRefused(t *testing.T) {
	source := podSource()
	model := typeKeys(podBar(t, followDeployment(t, source)), "z", "z")

	updated, command := model.Update(key("enter"))

	if command != nil || !strings.Contains(updated.View(), `pod "zz" not found`) || len(source.streamedPods) != 1 {
		t.Fatalf("an unknown pod was not refused (%d streams):\n%s", len(source.streamedPods), updated.View())
	}
}

func TestEscClosesThePodBarWithoutChangingTheLog(t *testing.T) {
	source := podSource()
	model := press(press(podBar(t, followDeployment(t, source)), "tab"), "esc")

	if model.prompt.active || len(source.streamedPods) != 1 || strings.Contains(model.View(), "pod: web") {
		t.Fatalf("esc changed the log (%d streams):\n%s", len(source.streamedPods), model.View())
	}
}

func TestPOnAPodLogExplainsItself(t *testing.T) {
	model := press(openPodLogs(t, containerSource()), "p")

	if view := model.View(); model.prompt.active || !strings.Contains(view, "workload") {
		t.Fatalf("p on a Pod's log did not explain itself:\n%s", view)
	}
}

func TestPWithoutPodsExplainsItself(t *testing.T) {
	source := podSource()
	source.tree = explorer.ResourceTree{Application: "checkout", Nodes: twoPodTree().Nodes[:1]}
	model := press(followDeployment(t, source), "p")

	if view := model.View(); model.prompt.active || !strings.Contains(view, "No pods") {
		t.Fatalf("p on a workload without pods did not explain itself:\n%s", view)
	}
}

func TestAFilteredPodKeepsTheChosenContainer(t *testing.T) {
	source := podSource()
	model := press(podBar(t, followDeployment(t, source)), "tab")
	updated, command := model.Update(key("enter"))
	model, _ = stepT(t, updated.(Model), command) // following web-abc

	updated, command = model.Update(key("c"))
	model, _ = stepT(t, updated.(Model), command) // the containers arrive and the bar opens
	updated, command = press(model, "tab").Update(key("enter"))
	stepT(t, updated.(Model), command)

	last := source.streamedPods[len(source.streamedPods)-1]
	if last.Name != "web-abc" || source.streamedContainers[len(source.streamedContainers)-1] != "sidecar" {
		t.Fatalf("restart streamed %+v with container %q, want web-abc and the sidecar", last, source.streamedContainers)
	}
}

func TestLogsHeaderListsPodOnlyForWorkloads(t *testing.T) {
	workload := followDeployment(t, podSource())
	pod := openPodLogs(t, containerSource())

	if view := resize(workload, 170, 40).View(); !strings.Contains(view, "p  Pod") {
		t.Fatalf("a workload log header does not list p:\n%s", view)
	}
	if view := resize(pod, 170, 40).View(); strings.Contains(view, "p  Pod") {
		t.Fatalf("a Pod log header lists p:\n%s", view)
	}
}

// longNameTree is a Deployment with the long names a Helm release gives its Pods.
func longNameTree() explorer.ResourceTree {
	deployment := explorer.ResourceReference{Group: "apps", Kind: "Deployment", Namespace: "store", Name: "argoxd-multicontainer-prometheus-server"}
	return explorer.ResourceTree{Application: "checkout", Nodes: []explorer.ResourceNode{
		{Group: "apps", Version: "v1", Kind: "Deployment", Namespace: "store", Name: "argoxd-multicontainer-prometheus-server", Health: "Healthy"},
		{Version: "v1", Kind: "Pod", Namespace: "store", Name: "argoxd-multicontainer-prometheus-server-587b857675-mvp75", Health: "Healthy", Parents: []explorer.ResourceReference{deployment}},
		{Version: "v1", Kind: "Pod", Namespace: "store", Name: "argoxd-multicontainer-prometheus-server-587b857675-zjvlk", Health: "Healthy", Parents: []explorer.ResourceReference{deployment}},
	}}
}

func titleRow(view string) string {
	for _, line := range strings.Split(view, "\n") {
		if strings.HasPrefix(line, "╭") {
			return line
		}
	}
	return ""
}

func TestTheLogTitleKeepsItsStateMarkersWhenItIsTooLong(t *testing.T) {
	for _, width := range []int{170, 120, 90} {
		source := podSource()
		source.tree = longNameTree()
		model := followFrom(t, resize(press(openCheckout(t, source), "j"), width, 16), "l", 0)
		model = press(podBar(t, model), "tab")
		updated, command := model.Update(key("enter"))
		model, _ = stepT(t, updated.(Model), command)
		model = typeKeys(model, "/", "r", "e", "a", "d", "y", "enter")

		title := titleRow(model.View())

		for _, want := range []string{"/ready", "● following"} {
			if !strings.Contains(title, want) {
				t.Fatalf("at %d columns the title lost %q:\n%s", width, want, title)
			}
		}
		if got := lipgloss.Width(title); got > width {
			t.Fatalf("at %d columns the title row is %d wide", width, got)
		}
	}
}

func TestTheLogTitleShowsEverythingWhenItFits(t *testing.T) {
	source := podSource()
	model := press(podBar(t, followDeployment(t, source)), "tab")
	updated, command := model.Update(key("enter"))
	model, _ = stepT(t, updated.(Model), command)

	title := titleRow(resize(model, 140, 16).View())

	for _, want := range []string{"Deployment/web", "pod: web-abc", "● following"} {
		if !strings.Contains(title, want) {
			t.Fatalf("the title dropped %q though it fits:\n%s", want, title)
		}
	}
}
