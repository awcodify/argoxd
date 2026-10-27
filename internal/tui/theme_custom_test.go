package tui

import (
	"slices"
	"strings"
	"testing"
)

// registered adds a custom theme for one test and forgets it afterwards.
func registered(t *testing.T, name, base string, colors map[string]string) {
	t.Helper()
	t.Cleanup(func() {
		_ = setTheme("default")
		ForgetCustomThemes()
	})
	if err := RegisterTheme(name, base, colors); err != nil {
		t.Fatal(err)
	}
}

func TestACustomThemeStartsFromItsBaseAndChangesOnlyWhatYouSet(t *testing.T) {
	registered(t, "sunset", "nord", map[string]string{"accent": "#ff5500", "healthy": "#0f0"})

	custom, ok := lookupTheme("sunset")
	nord, _ := lookupTheme("nord")
	if !ok {
		t.Fatal("the custom theme cannot be found")
	}
	if custom.accent != "#ff5500" || custom.healthy != "#00ff00" {
		t.Fatalf("accent = %q, healthy = %q; want the colors given, #rgb written out in full", custom.accent, custom.healthy)
	}
	custom.accent, custom.healthy, custom.name = nord.accent, nord.healthy, nord.name
	if custom != nord {
		t.Fatalf("the colors that were not set differ from the base:\n%+v\n%+v", custom, nord)
	}
}

func TestACustomThemeWithoutABaseUsesTheDefault(t *testing.T) {
	registered(t, "mine", "", map[string]string{"danger": "#ff0000"})

	custom, _ := lookupTheme("mine")
	base, _ := lookupTheme("default")
	if custom.danger != "#ff0000" || custom.text != base.text {
		t.Fatalf("custom = %+v, want the default with only danger changed", custom)
	}
}

func TestCustomThemesAreListedAfterTheBuiltInOnesAndUsable(t *testing.T) {
	useTheme(t, "default")
	registered(t, "sunset", "default", map[string]string{"accent": "#ff5500"})

	names := ThemeNames()
	if names[len(names)-1] != "sunset" || !slices.Contains(names, "terminal") || slices.Index(names, "terminal") > slices.Index(names, "sunset") {
		t.Fatalf("names = %v, want sunset after the built-in themes", names)
	}
	before := accentStyle.Render("x")
	if err := setTheme("sunset"); err != nil || accentStyle.Render("x") == before || currentTheme != "sunset" {
		t.Fatalf("err = %v, theme = %q; want the custom accent to be drawn", err, currentTheme)
	}
}

func TestCustomThemesWorkFromTheThemeCommandAndItsSuggestions(t *testing.T) {
	useTheme(t, "default")
	registered(t, "sunset", "default", map[string]string{"accent": "#ff5500"})
	model, _ := sortModel()

	typed := typeKeys(model, ":", "t", "h", "e", "m", "e", " ", "s", "u")
	if got := typed.suggestions(); len(got) != 1 || got[0] != "theme sunset" {
		t.Fatalf("suggestions = %v, want theme sunset", got)
	}
	if model = command(model, ":theme sunset"); currentTheme != "sunset" || !strings.Contains(model.status, "sunset") {
		t.Fatalf("theme = %q, status = %q", currentTheme, model.status)
	}
}

func TestRegisterThemeRefusesWhatItCannotUse(t *testing.T) {
	t.Cleanup(ForgetCustomThemes)
	tests := []struct {
		name, base string
		colors     map[string]string
		want       string
	}{
		{"", "", nil, "name"},
		{"two words", "", nil, "name"},
		{"nord", "", nil, "built-in"},
		{"mine", "neon", nil, `base "neon"`},
		{"mine", "", map[string]string{"colour": "#fff"}, `"colour"`},
		{"mine", "", map[string]string{"accent": "red"}, `accent`},
		{"mine", "", map[string]string{"accent": "#12"}, `accent`},
		{"mine", "", map[string]string{"accent": "#gggggg"}, `accent`},
		{"mine", "", map[string]string{"accent": "256"}, `accent`},
		{"mine", "", map[string]string{"accent": ""}, `accent`},
	}
	for _, test := range tests {
		err := RegisterTheme(test.name, test.base, test.colors)
		if err == nil || !strings.Contains(err.Error(), test.want) {
			t.Errorf("RegisterTheme(%q, %q, %v) error = %v, want one mentioning %q", test.name, test.base, test.colors, err, test.want)
		}
	}
	if slices.Contains(ThemeNames(), "mine") {
		t.Fatal("a theme that was refused is still listed")
	}
}

func TestAnUnknownColorKeyListsTheValidOnes(t *testing.T) {
	t.Cleanup(ForgetCustomThemes)

	err := RegisterTheme("mine", "", map[string]string{"colour": "#fff"})

	for _, key := range []string{"accent", "healthy", "danger", "addedBackground", "badgeText"} {
		if !strings.Contains(err.Error(), key) {
			t.Errorf("the error does not list %q: %v", key, err)
		}
	}
}

func TestAnsiNumbersAndShortHexAreAccepted(t *testing.T) {
	registered(t, "mine", "", map[string]string{"accent": "208", "info": "#AbC", "muted": "0", "text": "255"})

	custom, _ := lookupTheme("mine")
	if custom.accent != "208" || custom.info != "#aabbcc" || custom.muted != "0" || custom.text != "255" {
		t.Fatalf("custom = %+v", custom)
	}
}

func TestRegisteringTheSameCustomNameAgainReplacesIt(t *testing.T) {
	registered(t, "mine", "", map[string]string{"accent": "#111111"})

	if err := RegisterTheme("mine", "", map[string]string{"accent": "#222222"}); err != nil {
		t.Fatal(err)
	}

	custom, _ := lookupTheme("mine")
	if custom.accent != "#222222" || strings.Count(strings.Join(ThemeNames(), ","), "mine") != 1 {
		t.Fatalf("accent = %q, names = %v; want one theme with the new color", custom.accent, ThemeNames())
	}
}

func TestEveryColorKeyCanBeSet(t *testing.T) {
	colors := map[string]string{}
	for _, key := range ThemeColorKeys() {
		colors[key] = "#010203"
	}
	registered(t, "all", "", colors)

	custom, _ := lookupTheme("all")
	for field, value := range map[string]string{
		"accent": custom.accent, "healthy": custom.healthy, "info": custom.info, "warning": custom.warning,
		"danger": custom.danger, "suspended": custom.suspended, "muted": custom.muted, "text": custom.text,
		"bright": custom.bright, "border": custom.border, "surface": custom.surface, "addedText": custom.addedFG,
		"addedBackground": custom.addedBG, "removedText": custom.removedFG, "removedBackground": custom.removedBG,
		"selection": custom.selection, "badgeText": custom.badgeText,
	} {
		if value != "#010203" {
			t.Errorf("%s was not set: %q", field, value)
		}
	}
	if len(ThemeColorKeys()) != 17 {
		t.Errorf("ThemeColorKeys() = %v, want 17 keys", ThemeColorKeys())
	}
}
