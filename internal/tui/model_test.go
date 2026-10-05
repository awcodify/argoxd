package tui

import (
	"fmt"
	"strings"
	"testing"

	"github.com/awcodify/argoxd/internal/explorer"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func TestApplicationCommandChangesExplorerScreen(t *testing.T) {
	model := press(New(nil, "test", "argocd", explorer.Snapshot{}), "3")

	got := command(model, ":app")

	if got.Explorer().Screen() != explorer.ApplicationsScreen {
		t.Fatalf("screen = %v, want %v", got.Explorer().Screen(), explorer.ApplicationsScreen)
	}
}

func TestNumberFiltersApplicationsByProject(t *testing.T) {
	model := New(nil, "test", "argocd", storeSnapshot())

	view := press(model, "2").View()

	if !strings.Contains(view, "applications · store · 1") || strings.Contains(view, "grafana") {
		t.Fatalf("2 did not filter applications by the second project:\n%s", view)
	}
}

func TestZeroShowsApplicationsFromAllProjects(t *testing.T) {
	model := press(New(nil, "test", "argocd", storeSnapshot()), "2")

	view := press(model, "0").View()

	if !strings.Contains(view, "applications · all · 2") {
		t.Fatalf("0 did not clear the project filter:\n%s", view)
	}
}

func TestHeaderListsProjectShortcuts(t *testing.T) {
	view := New(nil, "test", "argocd", storeSnapshot()).View()

	for _, want := range []string{"0  all", "1  platform", "2  store"} {
		if !strings.Contains(view, want) {
			t.Fatalf("header does not contain project shortcut %q:\n%s", want, view)
		}
	}
}

func TestInvalidProjectNumberKeepsCurrentScreen(t *testing.T) {
	model := New(nil, "test", "argocd", explorer.Snapshot{
		Projects: []explorer.Project{{Name: "platform"}},
	})

	got := press(model, "2")

	if got.Explorer().Screen() != explorer.ApplicationsScreen {
		t.Fatalf("screen = %v, want %v", got.Explorer().Screen(), explorer.ApplicationsScreen)
	}
	if !strings.Contains(got.View(), "Project 2 is not available") {
		t.Fatalf("view does not report invalid project shortcut:\n%s", got.View())
	}
}

func TestCommandModeShowsAndCancelsCommand(t *testing.T) {
	model := New(nil, "test", "argocd", explorer.Snapshot{})

	got := typeKeys(model, ":", "c", "l")
	if !strings.Contains(got.View(), "❯ cl") {
		t.Fatalf("view does not show the command prompt:\n%s", got.View())
	}

	got = press(got, "esc")
	if strings.Contains(got.View(), "❯") {
		t.Fatalf("escape should close the command prompt:\n%s", got.View())
	}
}

func TestCommandPromptShowsMatchingSuggestions(t *testing.T) {
	model := typeKeys(New(nil, "test", "argocd", explorer.Snapshot{}), ":", "p", "r")

	view := model.View()

	for _, want := range []string{"proj", "projects"} {
		if !strings.Contains(view, want) {
			t.Fatalf("prompt does not suggest %q:\n%s", want, view)
		}
	}
}

func TestTabAcceptsTheSelectedSuggestion(t *testing.T) {
	model := typeKeys(New(nil, "test", "argocd", explorer.Snapshot{}), ":", "c", "l", "down", "tab", "enter")

	if model.Explorer().Screen() != explorer.ClustersScreen {
		t.Fatalf("screen = %v, want %v", model.Explorer().Screen(), explorer.ClustersScreen)
	}
}

func TestApplicationCommandAcceptsProject(t *testing.T) {
	model := New(nil, "test", "argocd", storeSnapshot())

	view := command(model, ":app store").View()

	if !strings.Contains(view, "applications · store · 1") {
		t.Fatalf(":app store did not filter by project:\n%s", view)
	}
}

func TestCommandPromptSuggestsProjectNames(t *testing.T) {
	model := typeKeys(New(nil, "test", "argocd", storeSnapshot()), ":", "a", "p", "p", " ", "s")

	if view := model.View(); !strings.Contains(view, "app store") {
		t.Fatalf("prompt does not suggest the store project:\n%s", view)
	}
}

func TestUnknownCommandIsReported(t *testing.T) {
	view := command(New(nil, "test", "argocd", explorer.Snapshot{}), ":nope").View()

	if !strings.Contains(view, `command "nope" not found`) {
		t.Fatalf("unknown command was not reported:\n%s", view)
	}
}

func TestQuitKeyQuits(t *testing.T) {
	model := New(nil, "test", "argocd", explorer.Snapshot{})

	_, command := model.Update(key("q"))
	if command == nil {
		t.Fatal("q did not return a quit command")
	}
}

func TestInventoryTreeGroupsApplicationsByProject(t *testing.T) {
	model := New(nil, "test", "argocd", explorer.Snapshot{
		Applications: []explorer.Application{{Name: "checkout", Project: "store"}},
		Projects:     []explorer.Project{{Name: "store"}},
	})

	view := press(model, "t").View()

	for _, want := range []string{"store", "checkout", "◆ inventory"} {
		if !strings.Contains(view, want) {
			t.Fatalf("inventory view does not contain %q:\n%s", want, view)
		}
	}
}

func TestViewRendersK9sStyleChrome(t *testing.T) {
	model := New(nil, "kubeconfig current context", "argocd", explorer.Snapshot{
		Applications: []explorer.Application{{Name: "checkout", Project: "store", Sync: "Synced", Health: "Healthy"}},
	})

	view := model.View()

	for _, want := range []string{
		"Context", "kubeconfig current context", "Namespace", "argocd",
		"s  Sync", "enter  Dependencies",
		"applications · all · 1",
		"NAME", "PROJECT", "SYNC", "HEALTH", "checkout", "♥ Healthy", "✓ Synced",
		"applications",
	} {
		if !strings.Contains(view, want) {
			t.Fatalf("view does not contain %q:\n%s", want, view)
		}
	}
}

func TestViewFillsTheTerminal(t *testing.T) {
	model := resize(New(nil, "test", "argocd", storeSnapshot()), 100, 30)

	for name, view := range map[string]string{
		"list":   model.View(),
		"prompt": press(model, ":").View(),
	} {
		lines := strings.Split(view, "\n")
		if len(lines) != 30 {
			t.Fatalf("%s view has %d lines, want 30", name, len(lines))
		}
		for index, line := range lines {
			if width := lipgloss.Width(line); width > 100 {
				t.Fatalf("%s line %d is %d columns wide, want at most 100:\n%s", name, index, width, line)
			}
		}
	}
}

func TestApplicationListKeepsSelectionVisible(t *testing.T) {
	model := resize(New(nil, "test", "argocd", explorer.Snapshot{Applications: applications(40)}), 100, 20)

	for range 39 {
		model = press(model, "j")
	}
	view := model.View()

	if !strings.Contains(view, "app-39") {
		t.Fatalf("selected application is not visible:\n%s", view)
	}
	if strings.Contains(view, "app-00") {
		t.Fatalf("list did not scroll past the first application:\n%s", view)
	}
}

func TestApplicationDetailRendersDependencyCards(t *testing.T) {
	model := withApplicationTree(New(nil, "test", "argocd", explorer.Snapshot{
		Applications: []explorer.Application{{Name: "checkout", Project: "store", Sync: "Synced", Health: "Healthy"}},
	}), explorer.ResourceTree{
		Application: "checkout",
		Nodes: []explorer.ResourceNode{
			{Group: "apps", Kind: "Deployment", Name: "web", Sync: "Synced", Health: "Healthy"},
			{Group: "apps", Kind: "ReplicaSet", Name: "web-7d9f", Health: "Healthy",
				Parents: []explorer.ResourceReference{{Group: "apps", Kind: "Deployment", Name: "web"}}},
			{Kind: "Service", Name: "web", Sync: "OutOfSync", Health: "Healthy"},
		},
	})

	view := model.View()

	for _, want := range []string{
		"checkout · store · 3", "applications › checkout",
		"Application", "Deployment", "ReplicaSet", "web-7d9f", "Service", "OutOfSync",
		"├─", "└─",
	} {
		if !strings.Contains(view, want) {
			t.Fatalf("application detail does not contain %q:\n%s", want, view)
		}
	}
}

func TestApplicationDetailSummarisesStatusAndDescribesSelectedCard(t *testing.T) {
	model := withApplicationTree(resize(New(nil, "test", "argocd", explorer.Snapshot{}), 140, 40), explorer.ResourceTree{
		Application: "checkout",
		Nodes: []explorer.ResourceNode{
			{Group: "apps", Kind: "Deployment", Namespace: "store", Name: "web", Sync: "Synced", Health: "Healthy"},
			{Kind: "Service", Namespace: "store", Name: "web", Sync: "OutOfSync", Health: "Degraded"},
		},
	})

	view := press(model, "j").View()

	for _, want := range []string{
		"1 healthy", "1 degraded", "1 out of sync",
		"Kind       Deployment", "Group      apps", "Namespace  store",
	} {
		if !strings.Contains(view, want) {
			t.Fatalf("application detail does not contain %q:\n%s", want, view)
		}
	}
}

func TestApplicationDetailScrollsToSelectedCard(t *testing.T) {
	tree := explorer.ResourceTree{Application: "checkout"}
	for index := range 20 {
		tree.Nodes = append(tree.Nodes, explorer.ResourceNode{Kind: "ConfigMap", Name: fmt.Sprintf("config-%02d", index)})
	}
	model := withApplicationTree(resize(New(nil, "test", "argocd", explorer.Snapshot{}), 100, 30), tree)

	for range 20 {
		model = press(model, "j")
	}
	view := model.View()

	if !strings.Contains(view, "config-19") {
		t.Fatalf("selected card is not visible:\n%s", view)
	}
	if strings.Contains(view, "config-00") {
		t.Fatalf("detail did not scroll past the first card:\n%s", view)
	}
}

func TestEscapeReturnsFromApplicationDetail(t *testing.T) {
	model := withApplicationTree(New(nil, "test", "argocd", explorer.Snapshot{}), explorer.ResourceTree{Application: "checkout"})

	view := press(model, "esc").View()

	if strings.Contains(view, "› checkout") {
		t.Fatalf("escape did not leave the application detail:\n%s", view)
	}
}

func TestDeletePromptsForConfirmation(t *testing.T) {
	model := New(nil, "test", "argocd", explorer.Snapshot{Applications: applications(1)})

	view := press(model, "d").View()

	if !strings.Contains(view, "Delete app-00") {
		t.Fatalf("delete did not ask for confirmation:\n%s", view)
	}
}

func key(value string) tea.KeyMsg {
	special := map[string]tea.KeyType{
		"esc": tea.KeyEsc, "enter": tea.KeyEnter, "tab": tea.KeyTab,
		"up": tea.KeyUp, "down": tea.KeyDown, "backspace": tea.KeyBackspace, " ": tea.KeySpace,
	}
	if keyType, found := special[value]; found {
		return tea.KeyMsg{Type: keyType}
	}
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(value)}
}

func typeKeys(model Model, values ...string) Model {
	for _, value := range values {
		model = press(model, value)
	}
	return model
}

func storeSnapshot() explorer.Snapshot {
	return explorer.Snapshot{
		Projects: []explorer.Project{{Name: "platform"}, {Name: "store"}},
		Applications: []explorer.Application{
			{Name: "grafana", Project: "platform", Sync: "Synced", Health: "Healthy"},
			{Name: "checkout", Project: "store", Sync: "OutOfSync", Health: "Degraded"},
		},
	}
}

func press(model Model, value string) Model {
	updated, _ := model.Update(key(value))
	return updated.(Model)
}

func command(model Model, value string) Model {
	for _, character := range value {
		model = press(model, string(character))
	}
	return press(model, "enter")
}

func resize(model Model, width, height int) Model {
	updated, _ := model.Update(tea.WindowSizeMsg{Width: width, Height: height})
	return updated.(Model)
}

func withApplicationTree(model Model, tree explorer.ResourceTree) Model {
	updated, _ := model.Update(loadedTree{tree: tree})
	return updated.(Model)
}

func applications(count int) []explorer.Application {
	result := make([]explorer.Application, 0, count)
	for index := range count {
		result = append(result, explorer.Application{Name: fmt.Sprintf("app-%02d", index), Project: "default"})
	}
	return result
}
