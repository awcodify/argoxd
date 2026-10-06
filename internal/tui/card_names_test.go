package tui

import (
	"strings"
	"testing"

	"github.com/awcodify/argoxd/internal/explorer"
	"github.com/charmbracelet/lipgloss"
)

func TestLongCardNamesKeepTheEndThatTellsPodsApart(t *testing.T) {
	first := explorer.ResourceNode{Version: "v1", Kind: "Pod", Namespace: "store", Name: "argoxd-multicontainer-prometheus-server-65f4b76f96-svt8n", Health: "Healthy"}
	second := first
	second.Name = "argoxd-multicontainer-prometheus-server-65f4b76f96-vtscr"

	one, two := resourceCard(first, false, false), resourceCard(second, false, false)

	if !strings.Contains(one, "svt8n") || !strings.Contains(two, "vtscr") || one == two {
		t.Fatalf("the cards do not tell the pods apart:\n%s\n%s", one, two)
	}
	for _, card := range []string{one, two} {
		if !strings.Contains(card, "argoxd-multicontainer") || !strings.Contains(card, "…") {
			t.Fatalf("a long name was not shortened in the middle:\n%s", card)
		}
		for _, line := range strings.Split(card, "\n") {
			if got := lipgloss.Width(line); got != cardWidth {
				t.Fatalf("a card line is %d columns wide, want %d: %q", got, cardWidth, line)
			}
		}
	}
}

func TestShortCardNamesAreLeftAlone(t *testing.T) {
	node := explorer.ResourceNode{Version: "v1", Kind: "Pod", Namespace: "store", Name: "web-abc", Health: "Healthy"}

	if card := resourceCard(node, false, false); !strings.Contains(card, "web-abc") || strings.Contains(card, "…") {
		t.Fatalf("a short name was changed:\n%s", card)
	}
}

func TestTheDetailsKeepTheEndOfALongName(t *testing.T) {
	node := explorer.ResourceNode{Version: "v1", Kind: "Pod", Namespace: "store", Name: "argoxd-multicontainer-prometheus-server-65f4b76f96-svt8n", Health: "Healthy"}

	details := strings.Join(renderDetails(card{node: node}), "\n")

	if !strings.Contains(details, "svt8n") {
		t.Fatalf("the details lost the end of the name:\n%s", details)
	}
}

func TestTruncateMiddleKeepsTheStartAndTheEnd(t *testing.T) {
	for name, want := range map[string]string{
		"short":                     "short",
		"abcdefghijklmnopqrstuvwxy": "abcdefghijklmnopqrstuvwxy",
	} {
		if got := truncateMiddle(name, 25); got != want {
			t.Errorf("truncateMiddle(%q) = %q, want it unchanged", name, got)
		}
	}
	got := truncateMiddle("argoxd-multicontainer-prometheus-server-65f4b76f96-svt8n", 24)
	if lipgloss.Width(got) != 24 || !strings.HasPrefix(got, "argoxd-") || !strings.HasSuffix(got, "-svt8n") || !strings.Contains(got, "…") {
		t.Fatalf("truncateMiddle = %q (%d wide), want 24 columns with the start and the end", got, lipgloss.Width(got))
	}
}
