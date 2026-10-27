package tui

import (
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// manyEvents is a table of n events, the oldest first, as the sources return it.
func manyEvents(n int) string {
	lines := []string{"AGE  TYPE    REASON   OBJECT   MESSAGE"}
	for i := 1; i <= n; i++ {
		lines = append(lines, fmt.Sprintf("%dm   Normal  Event%02d  Pod/web  message number %02d", n-i+1, i, i))
	}
	return strings.Join(lines, "\n")
}

func scrollModel(t *testing.T, events int) Model {
	t.Helper()
	source := &fakeSource{snapshot: storeSnapshot(), tree: checkoutTree(), events: manyEvents(events)}
	model := resize(openCheckout(t, source), 140, 44)
	return keyed(model, "j", "E") // the Deployment, pane open
}

func TestThePaneStartsAtTheNewestEvents(t *testing.T) {
	view := scrollModel(t, 30).View()

	if !strings.Contains(view, "Event30") || strings.Contains(view, "Event01 ") {
		t.Fatalf("the pane should end at the newest event:\n%s", view)
	}
}

func TestBracketKeysScrollThePaneBackToOlderEventsAndForwardAgain(t *testing.T) {
	model := scrollModel(t, 30)

	older := press(model, "[")
	if view := older.View(); strings.Contains(view, "Event30") || !strings.Contains(view, "Event2") {
		t.Fatalf("[ should show older events and drop the newest:\n%s", view)
	}
	back := press(older, "]")
	if view := back.View(); !strings.Contains(view, "Event30") {
		t.Fatalf("] should return to the newest:\n%s", view)
	}
}

func TestScrollingStopsAtBothEnds(t *testing.T) {
	model := scrollModel(t, 30)

	atNewest := press(model, "]")
	if atNewest.paneOffset != 0 {
		t.Fatalf("offset = %d at the newest event, want 0", atNewest.paneOffset)
	}
	oldest := model
	for range 20 {
		oldest = press(oldest, "[")
	}
	if view := oldest.View(); !strings.Contains(view, "Event01 ") {
		t.Fatalf("the oldest event cannot be reached:\n%s", view)
	}
	if more := press(oldest, "["); more.paneOffset != oldest.paneOffset {
		t.Fatalf("scrolled past the oldest event: %d → %d", oldest.paneOffset, more.paneOffset)
	}
}

func TestThePaneTitleSaysHowManyEventsAreOffScreen(t *testing.T) {
	model := scrollModel(t, 30)
	if title := paneTitle(model.View()); !strings.Contains(title, "older") || strings.Contains(title, "newer") {
		t.Fatalf("at the newest end the title should mention older events only: %q", title)
	}
	scrolled := press(model, "[")
	if title := paneTitle(scrolled.View()); !strings.Contains(title, "older") || !strings.Contains(title, "newer") {
		t.Fatalf("in the middle the title should mention both: %q", title)
	}
	few := scrollModel(t, 3)
	if title := paneTitle(few.View()); strings.Contains(title, "older") || strings.Contains(title, "newer") {
		t.Fatalf("with everything visible the title should not mention hidden events: %q", title)
	}
}

func paneTitle(view string) string {
	for _, line := range strings.Split(view, "\n") {
		if strings.Contains(line, "events ·") {
			return line
		}
	}
	return ""
}

func TestTheWheelOverThePaneScrollsItAndAnywhereElseMovesTheSelection(t *testing.T) {
	model := scrollModel(t, 30)
	top, bottom, ok := model.paneRows()
	if !ok || bottom <= top {
		t.Fatalf("paneRows() = %d, %d, %v", top, bottom, ok)
	}
	selected := model.selectedCard().Name

	overPane, _ := model.Update(tea.MouseMsg{Button: tea.MouseButtonWheelUp, Action: tea.MouseActionPress, X: 20, Y: top + 2})
	scrolled := overPane.(Model)
	if scrolled.paneOffset == 0 || scrolled.selectedCard().Name != selected {
		t.Fatalf("a wheel over the pane: offset %d, selected %q (was %q); want it scrolled and the selection kept", scrolled.paneOffset, scrolled.selectedCard().Name, selected)
	}

	elsewhere, _ := model.Update(tea.MouseMsg{Button: tea.MouseButtonWheelDown, Action: tea.MouseActionPress, X: 20, Y: 8})
	if moved := elsewhere.(Model); moved.paneOffset != 0 || moved.selectedCard().Name == selected {
		t.Fatalf("a wheel over the cards: offset %d, selected %q; want the selection to move", moved.paneOffset, moved.selectedCard().Name)
	}
}

func TestMovingToAnotherCardShowsItsNewestEventsAgain(t *testing.T) {
	model := press(scrollModel(t, 30), "[")
	if model.paneOffset == 0 {
		t.Fatal("setup: the pane did not scroll")
	}

	moved := keyed(model, "j")

	if moved.paneOffset != 0 {
		t.Fatalf("offset = %d after moving to another card, want 0", moved.paneOffset)
	}
}

func TestScrollKeysDoNothingWithoutAPane(t *testing.T) {
	source := &fakeSource{snapshot: storeSnapshot(), tree: checkoutTree(), events: manyEvents(30)}
	model := resize(openCheckout(t, source), 140, 44)

	after := press(press(model, "["), "]")

	if after.paneOffset != 0 || strings.Contains(after.View(), "events ·") {
		t.Fatal("the scroll keys changed something with no pane open")
	}
}

func TestThePaneTitleTeachesTheScrollKeysOnlyWhenThereIsSomethingToScroll(t *testing.T) {
	if title := paneTitle(scrollModel(t, 30).View()); !strings.Contains(title, "[ ] scroll") {
		t.Fatalf("a pane with hidden events should say how to scroll: %q", title)
	}
	if title := paneTitle(scrollModel(t, 2).View()); strings.Contains(title, "scroll") {
		t.Fatalf("a pane showing everything should not mention scrolling: %q", title)
	}
}
