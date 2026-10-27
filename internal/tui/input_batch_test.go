package tui

import (
	"testing"

	"github.com/awcodify/argoxd/internal/explorer"
	tea "github.com/charmbracelet/bubbletea"
)

// burst is one key message carrying several runes, the way Bubble Tea delivers
// fast typing (paste false) or bracketed paste (paste true).
func burst(text string, paste bool) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(text), Paste: paste}
}

func TestABurstOfTypedRunesIsNotLost(t *testing.T) {
	model := New(nil, "test", "argocd", explorer.Snapshot{Applications: applications(3)})

	updated, _ := model.Update(burst("jj", false))

	if got := updated.(Model).explorer.SelectedName(); got != "app-02" {
		t.Fatalf("selected %q after jj in one message, want app-02", got)
	}
}

func TestAPastedCommandOpensTheCommandBar(t *testing.T) {
	model := New(nil, "test", "argocd", storeSnapshot())

	updated, _ := model.Update(burst(":app store", true))

	typed := updated.(Model)
	if !typed.prompt.active || typed.prompt.input != "app store" {
		t.Fatalf("prompt = %+v, want the pasted command in the bar", typed.prompt)
	}
	ran, _ := typed.Update(key("enter"))
	if got := ran.(Model).explorer.Project(); got != "store" {
		t.Fatalf("project = %q after Enter, want store", got)
	}
}

func TestAPastedCommandKeepsItsSpacesAndDropsControlCharacters(t *testing.T) {
	model := New(nil, "test", "argocd", storeSnapshot())

	updated, _ := model.Update(burst(":sort health\n", true))

	if got := updated.(Model).prompt.input; got != "sort health" {
		t.Fatalf("input = %q, want %q", got, "sort health")
	}
	ran, _ := updated.(Model).Update(key("enter"))
	if field, _ := ran.(Model).explorer.Sort(); field != explorer.SortHealth {
		t.Fatalf("sort = %q after the pasted command, want health", field)
	}
}

func TestPastingTextOutsideAPromptDoesNothing(t *testing.T) {
	model := New(nil, "test", "argocd", storeSnapshot())
	before := model.explorer.SelectedName()

	for _, text := range []string{"jj", "q", "x"} {
		updated, command := model.Update(burst(text, true))
		got := updated.(Model)
		if command != nil {
			t.Fatalf("%q returned a command, want none", text)
		}
		if got.prompt.active || got.explorer.SelectedName() != before {
			t.Fatalf("%q did something: prompt %v, selected %q", text, got.prompt.active, got.explorer.SelectedName())
		}
	}
}

func TestPastingIntoAnOpenPromptFillsThePrompt(t *testing.T) {
	model := press(New(nil, "test", "argocd", storeSnapshot()), ":")

	updated, _ := model.Update(burst("app store", true))

	if got := updated.(Model).prompt.input; got != "app store" {
		t.Fatalf("input = %q, want the pasted text", got)
	}
}

func TestAPasteBehindAConfirmationIsIgnored(t *testing.T) {
	model := press(New(nil, "test", "argocd", storeSnapshot()), "s")
	if !model.confirm.active() {
		t.Fatal("setup: s did not ask about a sync")
	}

	updated, _ := model.Update(burst(":sort health", true))

	if got := updated.(Model); !got.confirm.active() || got.prompt.active {
		t.Fatalf("paste reached the dialog: confirm %v, prompt %v", got.confirm.active(), got.prompt.active)
	}
}
