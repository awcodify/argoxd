package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

const (
	slateForeground = "38;2;100;116;139"
	anyBackground   = "48;2;"
)

func TestDiffKeepsUnchangedLinesReadableAndHighlightsChanges(t *testing.T) {
	withTrueColor(t)
	view := newTextView("diff", "Deployment/web", "  kind: Deployment\n-   replicas: 1\n+   replicas: 2")

	lines := view.render(60, 10)

	if strings.Contains(lines[0], slateForeground) {
		t.Fatalf("unchanged line is dimmed: %q", lines[0])
	}
	for _, line := range lines[1:] {
		if !strings.Contains(line, anyBackground) {
			t.Fatalf("changed line has no background band: %q", line)
		}
		if width := lipgloss.Width(line); width != 60 {
			t.Fatalf("changed line band is %d columns wide, want 60: %q", width, line)
		}
	}
}

func TestDiffTitleExplainsBothSides(t *testing.T) {
	source := &fakeSource{snapshot: storeSnapshot(), tree: checkoutTree(), diff: "-   replicas: 1\n+   replicas: 2"}

	view := run(press(openCheckout(t, source), "j"), "d").View()

	if !strings.Contains(view, "− live") || !strings.Contains(view, "+ desired") {
		t.Fatalf("diff title does not explain the two sides:\n%s", view)
	}
}

func withTrueColor(t *testing.T) {
	t.Helper()
	previous := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	t.Cleanup(func() { lipgloss.SetColorProfile(previous) })
}
