package tui

import (
	"strings"
	"testing"

	"github.com/awcodify/argoxd/internal/explorer"
	"github.com/charmbracelet/lipgloss"
)

// rowOf returns the line of the view that contains the text, or -1.
func rowOf(view, text string) int {
	for index, line := range strings.Split(view, "\n") {
		if strings.Contains(line, text) {
			return index
		}
	}
	return -1
}

func TestSummaryBoxHoldsTheStatusCountsBelowTheCards(t *testing.T) {
	view := openCheckout(t, &fakeSource{snapshot: storeSnapshot(), tree: checkoutTree()}).View()

	box, counts, lastCard := rowOf(view, "╭─ summary"), rowOf(view, "2 healthy"), rowOf(view, "web-abc")
	if box < 0 || counts < 0 || lastCard < 0 {
		t.Fatalf("the summary box, its counts or the last card is missing (rows %d, %d, %d):\n%s", box, counts, lastCard, view)
	}
	if box < lastCard || counts <= box {
		t.Fatalf("the summary box should sit below the cards with the counts inside it (box %d, counts %d, last card %d):\n%s", box, counts, lastCard, view)
	}
}

func TestSummaryBoxHoldsTheConditionsWithTheCounts(t *testing.T) {
	source := &fakeSource{
		snapshot: snapshotWithConditions(explorer.Condition{Type: "SyncError", Message: "one or more objects failed to apply"}),
		tree:     pruneTree(),
	}

	view := openCheckout(t, source).View()

	box := rowOf(view, "╭─ summary")
	if box < 0 {
		t.Fatalf("there is no summary box:\n%s", view)
	}
	for _, want := range []string{"1 to prune", "1 orphaned", conditionGlyph + " SyncError: one or more objects failed to apply"} {
		if row := rowOf(view, want); row <= box {
			t.Fatalf("%q should be inside the summary box (row %d, box at %d):\n%s", want, row, box, view)
		}
	}
}

func TestSummaryBoxKeepsItsRowWhateverTheNumberOfCards(t *testing.T) {
	few := openCheckout(t, &fakeSource{snapshot: storeSnapshot(), tree: checkoutTree()}).View()
	more := openCheckout(t, &fakeSource{snapshot: storeSnapshot(), tree: pruneTree()}).View()

	if a, b := rowOf(few, "╭─ summary"), rowOf(more, "╭─ summary"); a < 0 || a != b {
		t.Fatalf("the summary box is at row %d with few cards and %d with more, want the same row", a, b)
	}
}

func TestSummaryBoxAppearsOnANarrowScreen(t *testing.T) {
	model := resize(openCheckout(t, &fakeSource{snapshot: storeSnapshot(), tree: checkoutTree()}), 80, 40)

	view := model.View()

	if !strings.Contains(view, "╭─ summary") || !strings.Contains(view, "2 healthy") {
		t.Fatalf("a narrow screen lost the summary:\n%s", view)
	}
}

func TestSummaryBoxTruncatesLongConditionsToTheScreen(t *testing.T) {
	long := strings.Repeat("a very long message ", 20)
	source := &fakeSource{snapshot: snapshotWithConditions(explorer.Condition{Type: "SyncError", Message: long}), tree: checkoutTree()}

	view := openCheckout(t, source).View()

	for index, line := range strings.Split(view, "\n") {
		if width := lipgloss.Width(line); width > 140 {
			t.Fatalf("line %d is %d columns wide on a 140 column screen: %q", index, width, line)
		}
	}
	if !strings.Contains(view, "SyncError: a very long message") {
		t.Fatalf("the start of the condition should still be readable:\n%s", view)
	}
}

func TestSummaryBoxWrapsALongConditionOntoASecondLine(t *testing.T) {
	message := strings.Repeat("alpha ", 25) + "OMEGA"
	source := &fakeSource{snapshot: snapshotWithConditions(explorer.Condition{Type: "SyncError", Message: message}), tree: checkoutTree()}

	view := openCheckout(t, source).View()

	first, last := rowOf(view, "SyncError: alpha"), rowOf(view, "OMEGA")
	if first < 0 || last != first+1 {
		t.Fatalf("the end of the message should continue on the line below it (start row %d, end row %d):\n%s", first, last, view)
	}
}

func TestSummaryBoxCutsAConditionAfterTwoLines(t *testing.T) {
	message := strings.Repeat("alpha ", 80) + "OMEGA"
	source := &fakeSource{snapshot: snapshotWithConditions(explorer.Condition{Type: "SyncError", Message: message}), tree: checkoutTree()}

	view := openCheckout(t, source).View()

	if strings.Contains(view, "OMEGA") || !strings.Contains(view, "…") {
		t.Fatalf("a message of more than two lines should be cut with an ellipsis:\n%s", view)
	}
	rows := 0
	for _, line := range strings.Split(view, "\n") {
		if strings.Contains(line, "alpha alpha") {
			rows++
		}
	}
	if rows != 2 {
		t.Fatalf("the message is on %d lines, want exactly 2:\n%s", rows, view)
	}
}

func TestSummaryBoxHasAMarginOnBothSides(t *testing.T) {
	view := openCheckout(t, &fakeSource{snapshot: storeSnapshot(), tree: checkoutTree()}).View()

	line := strings.Split(view, "\n")[rowOf(view, "╭─ summary")]
	if !strings.HasPrefix(line, "│ ╭─ summary") || !strings.HasSuffix(strings.TrimRight(line, " "), "╮ │") {
		t.Fatalf("the summary box should be inset from the frame like the cards: %q", line)
	}
}
