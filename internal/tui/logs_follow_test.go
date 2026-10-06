package tui

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/awcodify/argoxd/internal/argocd"
	"github.com/awcodify/argoxd/internal/explorer"
	tea "github.com/charmbracelet/bubbletea"
)

func (f *fakeSource) StreamLogs(ctx context.Context, _ string, pod explorer.ResourceNode, container string) (<-chan argocd.LogEntry, error) {
	f.streamCtx = ctx
	f.streamedPods = append(f.streamedPods, pod)
	f.streamedContainers = append(f.streamedContainers, container)
	return f.stream, nil
}

func lines(values ...string) chan argocd.LogEntry {
	stream := make(chan argocd.LogEntry, 100)
	for _, value := range values {
		stream <- argocd.LogEntry{Line: value}
	}
	return stream
}

// openPodLogs opens the logs of checkout's Pod in a viewer three lines tall.
func openPodLogs(t *testing.T, source *fakeSource) Model {
	t.Helper()
	model := resize(openCheckout(t, source), 100, 12)
	model = run(press(press(model, "j"), "j"), "l")
	if !strings.Contains(model.View(), "checkout › logs") {
		t.Fatalf("l did not open the logs:\n%s", model.View())
	}
	return model
}

// step feeds one command's message back into the model and returns the next command.
func step(model Model, command tea.Cmd) (Model, tea.Cmd) {
	updated, next := model.Update(command())
	return updated.(Model), next
}

// stepT is step, failing the test instead of panicking when there is no command.
func stepT(t *testing.T, model Model, command tea.Cmd) (Model, tea.Cmd) {
	t.Helper()
	if command == nil {
		t.Fatalf("expected a command to run next, got none:\n%s", model.View())
	}
	return step(model, command)
}

// follow presses f and delivers the stream's start and its first count entries.
// The returned command waits for the next entry.
func follow(t *testing.T, model Model, count int) (Model, tea.Cmd) {
	t.Helper()
	updated, command := model.Update(key("f"))
	if command == nil {
		t.Fatalf("f did not start a stream:\n%s", updated.View())
	}
	model, command = step(updated.(Model), command)
	for range count {
		if command == nil {
			t.Fatal("the stream stopped before every entry was delivered")
		}
		model, command = step(model, command)
	}
	return model, command
}

func TestFFollowsThePodLogs(t *testing.T) {
	source := &fakeSource{snapshot: storeSnapshot(), tree: checkoutTree(), logs: "old", stream: lines("old", "new")}
	model := openPodLogs(t, source)

	model, _ = follow(t, model, 2)

	view := model.View()
	if !strings.Contains(view, "● following") || !strings.Contains(view, "new") {
		t.Fatalf("f did not follow the logs:\n%s", view)
	}
	if len(source.streamedPods) != 1 || source.streamedPods[0].Name != "web-abc" {
		t.Fatalf("streamed pods = %+v, want web-abc", source.streamedPods)
	}
}

func TestFollowingStaysAtTheBottomUntilYouScrollUp(t *testing.T) {
	var all []string
	for index := range 10 {
		all = append(all, fmt.Sprintf("line-%d", index))
	}
	source := &fakeSource{snapshot: storeSnapshot(), tree: checkoutTree(), stream: lines(all...)}
	model, next := follow(t, openPodLogs(t, source), 10)

	if view := model.View(); !strings.Contains(view, "line-9") || strings.Contains(view, "line-0") {
		t.Fatalf("the view did not stay at the bottom:\n%s", view)
	}

	model = press(model, "k")
	source.stream <- argocd.LogEntry{Line: "line-10"}
	model, _ = step(model, next)

	if view := model.View(); strings.Contains(view, "line-10") {
		t.Fatalf("a new line pulled the view down after scrolling up:\n%s", view)
	}
}

func TestFStopsFollowingAndKeepsTheLines(t *testing.T) {
	source := &fakeSource{snapshot: storeSnapshot(), tree: checkoutTree(), stream: lines("kept")}
	model, _ := follow(t, openPodLogs(t, source), 1)

	stopped := press(model, "f")

	if source.streamCtx.Err() == nil {
		t.Fatal("the stream was not cancelled")
	}
	if view := stopped.View(); strings.Contains(view, "following") || !strings.Contains(view, "kept") {
		t.Fatalf("f did not stop following and keep the lines:\n%s", view)
	}
}

func TestEscStopsFollowingAndGoesBack(t *testing.T) {
	source := &fakeSource{snapshot: storeSnapshot(), tree: checkoutTree(), stream: lines("a")}
	model, _ := follow(t, openPodLogs(t, source), 1)

	back := press(model, "esc")

	if source.streamCtx.Err() == nil {
		t.Fatal("leaving the logs did not cancel the stream")
	}
	if view := back.View(); !strings.Contains(view, "applications › checkout") || strings.Contains(view, "following") {
		t.Fatalf("esc did not return to the dependencies:\n%s", view)
	}
}

func TestAStreamErrorStopsFollowingAndIsShown(t *testing.T) {
	stream := lines()
	stream <- argocd.LogEntry{Err: errors.New("connection reset")}
	source := &fakeSource{snapshot: storeSnapshot(), tree: checkoutTree(), stream: stream}

	model, _ := follow(t, openPodLogs(t, source), 1)

	if view := model.View(); !strings.Contains(view, "connection reset") || strings.Contains(view, "following") {
		t.Fatalf("a stream error was not shown:\n%s", view)
	}
	if source.streamCtx.Err() == nil {
		t.Fatal("the failed stream was not cancelled")
	}
}

func TestAClosedStreamIsReported(t *testing.T) {
	stream := lines("last")
	close(stream)
	source := &fakeSource{snapshot: storeSnapshot(), tree: checkoutTree(), stream: stream}

	model, _ := follow(t, openPodLogs(t, source), 2)

	if view := model.View(); !strings.Contains(view, "log stream ended") || strings.Contains(view, "following") {
		t.Fatalf("the end of the stream was not reported:\n%s", view)
	}
}

func TestFollowingKeepsOnlyTheLatestLines(t *testing.T) {
	source := &fakeSource{snapshot: storeSnapshot(), tree: checkoutTree(), stream: lines("first")}
	model, _ := follow(t, openPodLogs(t, source), 1)

	for index := range maxLogLines + 5 {
		updated, _ := model.Update(logLine{stream: model.follow.id, entry: argocd.LogEntry{Line: fmt.Sprintf("n-%d", index)}})
		model = updated.(Model)
	}

	if got := len(model.viewer.lines); got != maxLogLines {
		t.Fatalf("kept %d lines, want %d", got, maxLogLines)
	}
	if model.viewer.lines[0] != "n-5" {
		t.Fatalf("first kept line = %q, want the oldest dropped", model.viewer.lines[0])
	}
}

func TestLinesFromAStoppedStreamAreIgnored(t *testing.T) {
	source := &fakeSource{snapshot: storeSnapshot(), tree: checkoutTree(), stream: lines("a")}
	model, _ := follow(t, openPodLogs(t, source), 1)
	id := model.follow.id
	model = press(model, "f")

	updated, _ := model.Update(logLine{stream: id, entry: argocd.LogEntry{Line: "late"}})

	if view := updated.(Model).View(); strings.Contains(view, "late") {
		t.Fatalf("a line from a stopped stream was shown:\n%s", view)
	}
}

func TestFOnlyFollowsLogs(t *testing.T) {
	source := &fakeSource{snapshot: storeSnapshot(), tree: checkoutTree(), manifest: "kind: Pod", stream: lines("x")}
	model := run(press(press(openCheckout(t, source), "j"), "j"), "y")

	_, command := model.Update(key("f"))

	if command != nil || len(source.streamedPods) != 0 {
		t.Fatal("f started a stream from a YAML view")
	}
}

func TestFollowingNeedsASourceThatStreams(t *testing.T) {
	source := &logsOnlySource{fakeSource: &fakeSource{snapshot: storeSnapshot(), tree: checkoutTree(), logs: "x"}}
	model := resize(New(source, "test", "argocd", explorer.Snapshot{}), 100, 12)
	model = settle(model, source.loadCommand())
	model = run(press(run(press(model, "j"), "enter"), "j"), "j")
	model = run(model, "l")

	if view := press(model, "f").View(); !strings.Contains(view, "cannot follow logs") {
		t.Fatalf("f did not explain the missing support:\n%s", view)
	}
}

// logsOnlySource serves logs but cannot stream them.
type logsOnlySource struct{ *fakeSource }

func (logsOnlySource) StreamLogs() {}

func TestLogsHeaderListsFollow(t *testing.T) {
	source := &fakeSource{snapshot: storeSnapshot(), tree: checkoutTree(), logs: "x", manifest: "kind: Pod"}
	model := resize(openCheckout(t, source), 160, 40)
	model = press(press(model, "j"), "j")

	if view := run(model, "l").View(); !strings.Contains(view, "f  Follow") {
		t.Fatalf("the logs header does not list f:\n%s", view)
	}
	if view := run(model, "y").View(); strings.Contains(view, "f  Follow") {
		t.Fatalf("the YAML header lists f:\n%s", view)
	}
}
