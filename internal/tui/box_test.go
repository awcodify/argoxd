package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func TestBoxSetsTitleIntoBorderAndKeepsEveryLineTheSameWidth(t *testing.T) {
	rendered := box{border: lipgloss.NormalBorder(), title: " pods ", width: 20, height: 4}.
		render([]string{"short", "a line that is far too long to fit"})

	lines := strings.Split(rendered, "\n")
	if len(lines) != 4 {
		t.Fatalf("box has %d lines, want 4:\n%s", len(lines), rendered)
	}
	if lines[0] != "┌─ pods ───────────┐" {
		t.Fatalf("top border = %q", lines[0])
	}
	for index, line := range lines {
		if width := lipgloss.Width(line); width != 20 {
			t.Fatalf("line %d is %d columns wide, want 20: %q", index, width, line)
		}
	}
}
