package tui

import (
	"strings"
	"testing"

	"github.com/awcodify/argoxd/internal/explorer"
	"github.com/charmbracelet/x/ansi"
)

func pickerModel(t *testing.T, current string) Model {
	t.Helper()
	useTheme(t, current)
	model, _ := sortModel()
	return command(model, ":theme")
}

func accentLook() string { return accentStyle.Render("x") + statusBadge("Degraded") }

func TestThemeAloneOpensAPickerOnTheCurrentTheme(t *testing.T) {
	model := pickerModel(t, "nord")

	if !model.prompt.active || model.prompt.field != themeField {
		t.Fatalf(":theme did not open the picker (active %v, field %q)", model.prompt.active, model.prompt.field)
	}
	if got := model.selectedSuggestion(); got != "nord" {
		t.Fatalf("the picker starts on %q, want the theme in use", got)
	}
	view := model.View()
	for _, name := range []string{"default", "light", "nord", "dracula"} {
		if !strings.Contains(view, name) {
			t.Errorf("the picker does not list %s:\n%s", name, view)
		}
	}
}

func TestMovingThroughThePickerRepaintsAtOnce(t *testing.T) {
	model := pickerModel(t, "default")
	before := accentLook()

	next := press(model, "tab")

	if currentTheme == "default" || accentLook() == before {
		t.Fatalf("Tab changed the picker's choice to %q but the colors did not change", next.selectedSuggestion())
	}
	if currentTheme != next.selectedSuggestion() {
		t.Fatalf("previewing %q but the highlighted theme is %q", currentTheme, next.selectedSuggestion())
	}
	if !next.prompt.active {
		t.Fatal("previewing closed the picker")
	}
}

func TestEveryWayOfMovingPreviewsTheHighlightedTheme(t *testing.T) {
	for _, move := range []string{"tab", "down", "shift+tab", "up"} {
		model := pickerModel(t, "default")

		moved := press(model, move)

		if currentTheme != moved.selectedSuggestion() || currentTheme == "default" {
			t.Errorf("%s: previewing %q, highlighted %q", move, currentTheme, moved.selectedSuggestion())
		}
	}
}

func TestEnterKeepsThePreviewedTheme(t *testing.T) {
	model := press(pickerModel(t, "default"), "tab")
	chosen := currentTheme

	model = press(model, "enter")

	if currentTheme != chosen || model.prompt.active || !strings.Contains(model.status, chosen) {
		t.Fatalf("theme = %q (chose %q), picker open = %v, status %q", currentTheme, chosen, model.prompt.active, model.status)
	}
	if again := press(press(model, ":"), "esc"); currentTheme != chosen || again.prompt.active {
		t.Fatalf("an unrelated prompt changed the theme to %q", currentTheme)
	}
}

func TestEscPutsTheOriginalThemeBack(t *testing.T) {
	model := pickerModel(t, "dracula")
	original := accentLook()
	model = typeKeys(model, "tab", "tab", "tab")
	if accentLook() == original {
		t.Fatal("setup: the preview did not change the colors")
	}

	model = press(model, "esc")

	if currentTheme != "dracula" || accentLook() != original || model.prompt.active {
		t.Fatalf("theme = %q after Esc, picker open = %v; want dracula restored", currentTheme, model.prompt.active)
	}
	if model.status != "" && strings.Contains(model.status, "Theme") {
		t.Fatalf("cancelling left a confirmation behind: %q", model.status)
	}
}

func TestBackspaceOnAnEmptyPickerCancelsLikeEsc(t *testing.T) {
	model := press(pickerModel(t, "default"), "tab")

	model = press(model, "backspace")

	if currentTheme != "default" || model.prompt.active {
		t.Fatalf("theme = %q, picker open = %v; want it cancelled and restored", currentTheme, model.prompt.active)
	}
}

func TestEnterWithoutMovingKeepsTheCurrentTheme(t *testing.T) {
	model := press(pickerModel(t, "light"), "enter")

	if currentTheme != "light" || model.prompt.active {
		t.Fatalf("theme = %q, open = %v", currentTheme, model.prompt.active)
	}
}

func TestTypingNarrowsThePickerAndPreviewsTheFirstMatch(t *testing.T) {
	model := typeKeys(pickerModel(t, "default"), "n", "o")

	if got := model.suggestions(); len(got) != 1 || got[0] != "nord" {
		t.Fatalf("suggestions = %v, want only nord", got)
	}
	if currentTheme != "nord" {
		t.Fatalf("previewing %q, want nord", currentTheme)
	}
	if kept := press(model, "enter"); currentTheme != "nord" || kept.prompt.active {
		t.Fatalf("theme = %q after Enter", currentTheme)
	}
}

func TestTypingSomethingThatMatchesNothingKeepsThePreviousPreview(t *testing.T) {
	model := typeKeys(pickerModel(t, "default"), "n", "o")

	model = press(press(model, "z"), "z")

	if currentTheme != "nord" {
		t.Fatalf("theme = %q with no match, want the last preview kept", currentTheme)
	}
	press(model, "esc")
	if currentTheme != "default" {
		t.Fatalf("Esc restored %q, want default", currentTheme)
	}
}

func TestThePickerOffersAndPreviewsCustomThemes(t *testing.T) {
	registered(t, "sunset", "default", map[string]string{"accent": "#ff5500"})
	model := pickerModel(t, "default")

	model = typeKeys(model, "s", "u")

	if currentTheme != "sunset" || !strings.Contains(model.View(), "sunset") {
		t.Fatalf("theme = %q; want the custom theme previewed", currentTheme)
	}
}

func TestOtherPromptsAreNotAffectedByThePreviewLogic(t *testing.T) {
	useTheme(t, "nord")
	model, _ := sortModel()

	typed := typeKeys(model, ":", "s", "o", "r", "t", " ", "tab", "esc")

	if currentTheme != "nord" || typed.prompt.active {
		t.Fatalf("a :sort prompt changed the theme to %q", currentTheme)
	}
}

func TestThePickerStaysOnScreenWhileThemesChange(t *testing.T) {
	model := press(pickerModel(t, "default"), "tab")

	view := ansi.Strip(model.View())

	if !strings.Contains(view, "default") || !strings.Contains(view, "light") || !strings.Contains(view, "▾ ▏") {
		t.Fatalf("the picker disappeared or lost its chips after a preview:\n%s", view)
	}
	_ = explorer.Snapshot{}
}
