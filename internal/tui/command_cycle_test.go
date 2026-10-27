package tui

import (
	"strings"
	"testing"

	"github.com/awcodify/argoxd/internal/explorer"
)

func commandBar(typed ...string) Model {
	model := New(nil, "test", "argocd", storeSnapshot())
	return typeKeys(model, append([]string{":"}, typed...)...)
}

func inputs(model Model, keys ...string) []string {
	var seen []string
	for _, key := range keys {
		model = press(model, key)
		seen = append(seen, model.prompt.input)
	}
	return seen
}

func equal(got, want []string) bool { return strings.Join(got, "|") == strings.Join(want, "|") }

func TestTabKeepsCyclingThroughTheSuggestionsOfACommand(t *testing.T) {
	model := commandBar("a", "p", "p", " ")

	got := inputs(model, "tab", "tab", "tab", "tab")
	if want := []string{"app platform", "app store", "app platform", "app store"}; !equal(got, want) {
		t.Fatalf("Tab gave %v, want it to keep changing and wrap: %v", got, want)
	}
	if got := inputs(commandBar("a", "p", "p", " "), "tab", "shift+tab", "shift+tab"); !equal(got, []string{"app platform", "app store", "app platform"}) {
		t.Fatalf("Shift+Tab gave %v, want it to go back through the same list", got)
	}
}

func TestArrowsKeepChangingTheChoiceOnceTabStartedCompleting(t *testing.T) {
	model := press(commandBar("a", "p", "p", " "), "tab")

	got := inputs(model, "down", "down", "up", "up")
	if want := []string{"app store", "app platform", "app store", "app platform"}; !equal(got, want) {
		t.Fatalf("arrows gave %v, want %v", got, want)
	}
}

func TestTheSuggestionListStaysWhileCycling(t *testing.T) {
	model := typeKeys(commandBar("a", "p", "p", " "), "tab", "tab")

	if view := model.View(); !strings.Contains(view, "app platform") || !strings.Contains(view, "app store") {
		t.Fatalf("the other suggestions vanished after Tab:\n%s", view)
	}
}

func TestEnterRunsTheSuggestionTheCyclingStoppedOn(t *testing.T) {
	model := typeKeys(commandBar("a", "p", "p", " "), "tab", "tab", "enter")

	if view := model.View(); !strings.Contains(view, "applications · store · 1") {
		t.Fatalf("Enter did not run the chosen project:\n%s", view)
	}
}

func TestBeforeTabArrowsOnlyMoveTheHighlight(t *testing.T) {
	model := press(commandBar("a", "p", "p", " "), "down")

	if model.prompt.input != "app " || model.selectedSuggestion() != "app store" {
		t.Fatalf("input = %q, highlighted = %q; want the typed text kept and the next suggestion highlighted", model.prompt.input, model.selectedSuggestion())
	}
	if accepted := press(model, "tab"); accepted.prompt.input != "app store" {
		t.Fatalf("Tab accepted %q, want the highlighted app store", accepted.prompt.input)
	}
}

func TestTypingAfterTabSuggestsFromTheNewText(t *testing.T) {
	model := press(commandBar("a", "p", "p", " "), "tab")

	model = press(model, "backspace")

	if matches := model.suggestions(); len(matches) != 1 || matches[0] != "app platfor" && matches[0] != "app platform" {
		t.Fatalf("after Backspace the suggestions = %v, want them recomputed from %q", matches, model.prompt.input)
	}
	if model.prompt.base != "" {
		t.Fatalf("base = %q, want editing to end the cycling", model.prompt.base)
	}
}

func TestSortAndFilterCommandsCycleToo(t *testing.T) {
	if got := inputs(commandBar("s", "o", "r", "t", " "), "tab", "tab", "tab", "tab"); !equal(got, []string{"sort health", "sort name", "sort sync", "sort health"}) {
		t.Fatalf(":sort cycled %v", got)
	}
	got := inputs(commandBar("h", "e", "a", "l", "t", "h", " "), "tab", "tab", "tab")
	if len(got) != 3 || got[0] == got[1] || got[1] == got[2] || !strings.HasPrefix(got[0], "health ") {
		t.Fatalf(":health cycled %v, want a different value each time", got)
	}
}

func TestTopLevelCommandNamesCycleToo(t *testing.T) {
	got := inputs(commandBar("p"), "tab", "tab", "tab")

	if got[0] == got[1] || got[1] == got[2] {
		t.Fatalf(":p cycled %v, want the matching commands one after another", got)
	}
}

func TestContextAndNamespaceCommandsCycleToo(t *testing.T) {
	source := &contextSource{fakeSource: &fakeSource{snapshot: storeSnapshot()}, contexts: []string{"development", "production"}}
	model := settle(New(source, "test", "argocd", explorer.Snapshot{}), source.loadCommand())

	got := inputs(typeKeys(model, ":", "c", "t", "x", " "), "tab", "tab", "tab")
	if want := []string{"ctx development", "ctx production", "ctx development"}; !equal(got, want) {
		t.Fatalf(":ctx cycled %v, want %v", got, want)
	}
}
