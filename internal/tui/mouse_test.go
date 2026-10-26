package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func wheel(model Model, button tea.MouseButton) Model {
	updated, _ := model.Update(tea.MouseMsg{Button: button, Action: tea.MouseActionPress})
	return updated.(Model)
}

func TestWheelMovesTheSelectionLikeJAndK(t *testing.T) {
	model, _ := sortModel()
	first := model.explorer.SelectedName()

	down := wheel(model, tea.MouseButtonWheelDown)
	if down.explorer.SelectedName() == first {
		t.Fatalf("wheel down left the selection on %q", first)
	}
	if up := wheel(down, tea.MouseButtonWheelUp); up.explorer.SelectedName() != first {
		t.Fatalf("wheel up went to %q, want %q", up.explorer.SelectedName(), first)
	}
}

func TestWheelScrollsAnOpenViewer(t *testing.T) {
	source := &fakeSource{snapshot: storeSnapshot(), tree: checkoutTree(), manifest: strings.Repeat("line\n", 200)}
	model := resize(press(openCheckout(t, source), "j"), 140, 20)
	model = run(model, "Y")
	before := model.viewer.offset

	after := wheel(wheel(model, tea.MouseButtonWheelDown), tea.MouseButtonWheelDown)

	if after.viewer.offset <= before {
		t.Fatalf("viewer offset = %d after two wheel steps, want it to scroll past %d", after.viewer.offset, before)
	}
}

func TestWheelIsIgnoredWhileAPromptOrADialogIsOpen(t *testing.T) {
	model, _ := sortModel()
	first := model.explorer.SelectedName()

	prompted := wheel(press(model, ":"), tea.MouseButtonWheelDown)
	if !prompted.prompt.active || prompted.prompt.selection != 0 {
		t.Fatalf("the wheel moved the command bar's suggestion (selection %d)", prompted.prompt.selection)
	}
	asking := wheel(press(model, "s"), tea.MouseButtonWheelDown)
	if !asking.confirm.active() || asking.explorer.SelectedName() != first {
		t.Fatalf("the wheel acted behind the confirmation (selected %q)", asking.explorer.SelectedName())
	}
}

func TestOtherMouseEventsDoNothing(t *testing.T) {
	model, _ := sortModel()
	first := model.explorer.SelectedName()

	for _, button := range []tea.MouseButton{tea.MouseButtonLeft, tea.MouseButtonRight, tea.MouseButtonMiddle} {
		if got := wheel(model, button).explorer.SelectedName(); got != first {
			t.Fatalf("%v moved the selection to %q", button, got)
		}
	}
}
