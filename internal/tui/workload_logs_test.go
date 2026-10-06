package tui

import (
	"strings"
	"testing"

	"github.com/awcodify/argoxd/internal/argocd"
	"github.com/awcodify/argoxd/internal/explorer"
	tea "github.com/charmbracelet/bubbletea"
)

// followFrom presses a key that opens a followed log and delivers the stream's
// start and its first count entries.
func followFrom(t *testing.T, model Model, pressed string, count int) Model {
	t.Helper()
	updated, command := model.Update(key(pressed))
	if command == nil {
		t.Fatalf("%s did not start a stream:\n%s", pressed, updated.View())
	}
	model, command = step(updated.(Model), command)
	for range count {
		model, command = step(model, command)
	}
	return model
}

func podLine(pod, line string) argocd.LogEntry {
	return argocd.LogEntry{Pod: pod, Line: line}
}

func TestLOnADeploymentFollowsTheLogsOfAllItsPods(t *testing.T) {
	stream := lines()
	stream <- podLine("web-abc", "hello")
	stream <- podLine("web-def", "world")
	source := &fakeSource{snapshot: storeSnapshot(), tree: checkoutTree(), stream: stream}
	deployment := press(resize(openCheckout(t, source), 120, 14), "j")

	view := followFrom(t, deployment, "l", 2).View()

	for _, want := range []string{"checkout › logs", "Deployment/web", "● following", "web-abc │ hello", "web-def │ world"} {
		if !strings.Contains(view, want) {
			t.Fatalf("the workload log does not contain %q:\n%s", want, view)
		}
	}
	if len(source.streamedPods) != 1 || source.streamedPods[0].Kind != "Deployment" || source.streamedPods[0].Name != "web" {
		t.Fatalf("streamed resources = %+v, want the Deployment", source.streamedPods)
	}
}

func TestAPodLogHasNoPodPrefix(t *testing.T) {
	stream := lines()
	stream <- podLine("web-abc", "hello")
	source := &fakeSource{snapshot: storeSnapshot(), tree: checkoutTree(), logs: "old", stream: stream}
	model := openPodLogs(t, source)

	model, _ = follow(t, model, 1)

	if view := model.View(); !strings.Contains(view, "hello") || strings.Contains(view, "web-abc │") {
		t.Fatalf("a single Pod's log was prefixed with its name:\n%s", view)
	}
}

func TestStoppingAWorkloadLogKeepsItsLinesAndFRestartsIt(t *testing.T) {
	stream := lines()
	stream <- podLine("web-abc", "hello")
	source := &fakeSource{snapshot: storeSnapshot(), tree: checkoutTree(), stream: stream}
	model := followFrom(t, resize(press(openCheckout(t, source), "j"), 120, 14), "l", 1)

	stopped := press(model, "f")
	if view := stopped.View(); strings.Contains(view, "following") || !strings.Contains(view, "web-abc │ hello") {
		t.Fatalf("f did not stop the workload log and keep its lines:\n%s", view)
	}

	source.stream = lines()
	if _, command := stopped.Update(key("f")); command == nil {
		t.Fatal("f did not restart the workload log")
	}
}

func TestLOnAResourceWithoutLogsIsRefused(t *testing.T) {
	source := &fakeSource{snapshot: storeSnapshot(), stream: lines(), tree: explorer.ResourceTree{
		Application: "checkout",
		Nodes:       []explorer.ResourceNode{{Version: "v1", Kind: "ConfigMap", Namespace: "store", Name: "settings"}},
	}}

	view := press(press(openCheckout(t, source), "j"), "l").View()

	if !strings.Contains(view, "Logs are available for Pods and workloads") {
		t.Fatalf("l on a ConfigMap was not refused:\n%s", view)
	}
	if len(source.streamedPods) != 0 {
		t.Fatalf("a stream was started for a ConfigMap: %+v", source.streamedPods)
	}
}

func TestWorkloadLogsNeedASourceThatStreams(t *testing.T) {
	source := &logsOnlySource{fakeSource: &fakeSource{snapshot: storeSnapshot(), tree: checkoutTree()}}
	model := resize(New(source, "test", "argocd", explorer.Snapshot{}), 100, 12)
	model = settle(model, source.loadCommand())
	model = press(run(press(model, "j"), "enter"), "j")

	updated, command := model.Update(key("l"))

	if command != nil || !strings.Contains(updated.View(), "cannot follow logs") {
		t.Fatalf("l on a Deployment did not explain the missing support:\n%s", updated.View())
	}
	var _ tea.Model = updated
}

func TestPodPrefixesAreColoredPerPod(t *testing.T) {
	withTrueColor(t)
	view := newTextView("logs", "Deployment/web", "")
	view.prefixed = true

	prefixes := map[string]bool{}
	for _, pod := range []string{"web-a", "web-b", "web-c", "web-d", "web-e", "web-f"} {
		first := view.highlight(pod+" │ started", 80)
		again := view.highlight(pod+" │ started", 80)
		if first != again {
			t.Fatalf("the color of %s is not stable: %q vs %q", pod, first, again)
		}
		if !strings.Contains(first, "started") {
			t.Fatalf("the message is missing: %q", first)
		}
		// Compare the styling alone, without the pod name it wraps.
		prefixes[strings.ReplaceAll(strings.SplitN(first, "│", 2)[0], pod, "")] = true
	}
	if len(prefixes) < 2 {
		t.Fatalf("every pod got the same styling: %v", prefixes)
	}
}
