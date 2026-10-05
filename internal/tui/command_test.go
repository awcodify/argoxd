package tui

import (
	"slices"
	"testing"

	"github.com/awcodify/argoxd/internal/explorer"
)

func TestSuggestionsCompleteCommandNames(t *testing.T) {
	got := suggestions("pr", nil, nil)

	if want := []string{"proj", "projects"}; !slices.Equal(got, want) {
		t.Fatalf("suggestions = %v, want %v", got, want)
	}
}

func TestSuggestionsCompleteProjectArgument(t *testing.T) {
	got := suggestions("apps s", []string{"platform", "store", "search"}, nil)

	if want := []string{"apps search", "apps store"}; !slices.Equal(got, want) {
		t.Fatalf("suggestions = %v, want %v", got, want)
	}
}

func TestSuggestionsAreEmptyForUnknownInput(t *testing.T) {
	if got := suggestions("zz", nil, nil); len(got) != 0 {
		t.Fatalf("suggestions = %v, want none", got)
	}
}

func TestValueSuggestionsStartWithAllAndMatchThePrefixIgnoringCase(t *testing.T) {
	for _, test := range []struct {
		field, input string
		kinds        []string
		want         []string
	}{
		{"health", "", nil, []string{"all", "Healthy", "Progressing", "Degraded", "Suspended", "Missing", "Unknown"}},
		{"health", "d", nil, []string{"Degraded"}},
		{"health", "a", nil, []string{"all"}},
		{"sync", "O", nil, []string{"OutOfSync"}},
		{"kind", "p", []string{"Pod", "Service"}, []string{"Pod"}},
		{"kind", "", nil, []string{"all"}},
	} {
		if got := valueSuggestions(test.field, test.input, test.kinds); !slices.Equal(got, test.want) {
			t.Errorf("valueSuggestions(%q, %q) = %v, want %v", test.field, test.input, got, test.want)
		}
	}
}

func TestSuggestionsCompleteFilterCommandsAndTheirValues(t *testing.T) {
	filters := map[string][]string{"health": explorer.HealthStatuses, "sync": explorer.SyncStatuses}
	for input, want := range map[string][]string{
		"he":       {"health"},
		"health d": {"health Degraded"},
		"health ":  {"health Healthy", "health Progressing", "health Degraded", "health Suspended", "health Missing", "health Unknown"},
		"sync OUT": {"sync OutOfSync"},
		"k":        nil,
		"kind Pod": nil,
	} {
		got := suggestions(input, nil, filters)
		slices.Sort(want)
		if !slices.Equal(got, want) {
			t.Errorf("suggestions(%q) = %v, want %v", input, got, want)
		}
	}
	if got := suggestions("he", nil, nil); len(got) != 0 {
		t.Errorf("health suggested where it cannot filter: %v", got)
	}
}
