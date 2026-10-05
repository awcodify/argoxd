package tui

import (
	"slices"
	"testing"
)

func TestSuggestionsCompleteCommandNames(t *testing.T) {
	got := suggestions("pr", nil)

	if want := []string{"proj", "projects"}; !slices.Equal(got, want) {
		t.Fatalf("suggestions = %v, want %v", got, want)
	}
}

func TestSuggestionsCompleteProjectArgument(t *testing.T) {
	got := suggestions("apps s", []string{"platform", "store", "search"})

	if want := []string{"apps search", "apps store"}; !slices.Equal(got, want) {
		t.Fatalf("suggestions = %v, want %v", got, want)
	}
}

func TestSuggestionsAreEmptyForUnknownInput(t *testing.T) {
	if got := suggestions("zz", nil); len(got) != 0 {
		t.Fatalf("suggestions = %v, want none", got)
	}
}
