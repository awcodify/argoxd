package argocd

import (
	"slices"
	"testing"
)

func TestLineDiffMarksRemovedAndAddedLines(t *testing.T) {
	got := lineDiff("kind: Deployment\nreplicas: 1\nimage: web:1\n", "kind: Deployment\nreplicas: 2\nimage: web:1\n")

	want := []string{"  kind: Deployment", "- replicas: 1", "+ replicas: 2", "  image: web:1"}
	if !slices.Equal(got, want) {
		t.Fatalf("lineDiff() = %q, want %q", got, want)
	}
}

func TestLineDiffOfIdenticalTextHasNoChanges(t *testing.T) {
	for _, line := range lineDiff("a\nb\n", "a\nb\n") {
		if line[0] != ' ' {
			t.Fatalf("unexpected change %q", line)
		}
	}
}
