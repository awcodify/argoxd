package tui

import (
	"fmt"
	"strings"
	"testing"

	"github.com/awcodify/argoxd/internal/argocd"
)

// openTallPodLogs opens checkout's Pod log in a viewer tall enough to show a few lines.
func openTallPodLogs(t *testing.T, source *fakeSource) Model {
	t.Helper()
	return resize(openPodLogs(t, source), 100, 24)
}

func TestSlashFiltersTheLogAsYouType(t *testing.T) {
	source := &fakeSource{snapshot: storeSnapshot(), tree: barePodTree(), logs: "alpha\nbeta\ngamma error\ndelta ERROR here"}
	model := openTallPodLogs(t, source)

	searching := typeKeys(model, "/", "e", "r", "r")
	view := searching.View()
	if !strings.Contains(view, "gamma error") || !strings.Contains(view, "delta ERROR here") || strings.Contains(view, "alpha") || strings.Contains(view, "beta") {
		t.Fatalf("typing did not narrow the log to the matching lines, ignoring case:\n%s", view)
	}

	kept := press(searching, "enter")
	if view := kept.View(); kept.prompt.active || !strings.Contains(view, "/err") || strings.Contains(view, "alpha") {
		t.Fatalf("enter did not keep the search:\n%s", view)
	}

	cleared := press(kept, "esc")
	if view := cleared.View(); !strings.Contains(view, "alpha") || !strings.Contains(view, "checkout › logs") {
		t.Fatalf("the first esc did not clear the search and stay in the log:\n%s", view)
	}

	if view := press(cleared, "esc").View(); strings.Contains(view, "checkout › logs") {
		t.Fatalf("the second esc did not leave the log:\n%s", view)
	}
}

func TestEscInTheSearchBarClearsTheSearch(t *testing.T) {
	source := &fakeSource{snapshot: storeSnapshot(), tree: barePodTree(), logs: "alpha\nbeta"}
	model := typeKeys(openTallPodLogs(t, source), "/", "b", "e")

	cleared := press(model, "esc")

	if view := cleared.View(); cleared.prompt.active || !strings.Contains(view, "alpha") || strings.Contains(view, "/be") {
		t.Fatalf("esc in the bar did not clear the search:\n%s", view)
	}
}

func TestSearchAppliesToLinesThatArriveWhileFollowing(t *testing.T) {
	var entries []string
	source := &fakeSource{snapshot: storeSnapshot(), tree: barePodTree(), stream: lines()}
	model, next := follow(t, openPodLogs(t, source), 0)
	model = typeKeys(model, "/", "l", "i", "n", "e", "-", "enter")
	for index := range 10 {
		entries = append(entries, fmt.Sprintf("noise-%d", index), fmt.Sprintf("line-%d", index))
	}
	for _, line := range entries {
		source.stream <- argocd.LogEntry{Line: line}
	}

	for range entries {
		model, next = stepT(t, model, next)
	}

	view := model.View()
	if !strings.Contains(view, "line-9") || strings.Contains(view, "noise") || strings.Contains(view, "line-0") {
		t.Fatalf("the filtered log did not stay at its newest matching lines:\n%s", view)
	}
}

func TestSearchOnlyOpensInLogs(t *testing.T) {
	source := &fakeSource{snapshot: storeSnapshot(), tree: barePodTree(), manifest: "kind: Pod"}
	model := run(press(press(openCheckout(t, source), "j"), "j"), "y")

	if updated := press(model, "/"); updated.prompt.active {
		t.Fatal("/ opened a search in a YAML view")
	}
}

func TestLogsHeaderListsSearch(t *testing.T) {
	source := &fakeSource{snapshot: storeSnapshot(), tree: barePodTree(), logs: "x", manifest: "kind: Pod"}
	model := resize(openCheckout(t, source), 160, 40)
	model = press(press(model, "j"), "j")

	if view := run(model, "l").View(); !strings.Contains(view, "/  Search") {
		t.Fatalf("the logs header does not list /:\n%s", view)
	}
	if view := run(model, "y").View(); strings.Contains(view, "/  Search") {
		t.Fatalf("the YAML header lists /:\n%s", view)
	}
}
