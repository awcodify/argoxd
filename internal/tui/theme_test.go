package tui

import (
	"math"
	"strconv"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

// useTheme switches theme for one test and restores the default afterwards.
func useTheme(t *testing.T, name string) {
	t.Helper()
	previous := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	if err := setTheme(name); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = setTheme("default")
		lipgloss.SetColorProfile(previous)
	})
}

func luminance(t *testing.T, hex string) float64 {
	t.Helper()
	if len(hex) != 7 || hex[0] != '#' {
		t.Fatalf("%q is not #rrggbb", hex)
	}
	channel := func(from int) float64 {
		value, err := strconv.ParseUint(hex[from:from+2], 16, 8)
		if err != nil {
			t.Fatal(err)
		}
		c := float64(value) / 255
		if c <= 0.03928 {
			return c / 12.92
		}
		return math.Pow((c+0.055)/1.055, 2.4)
	}
	return 0.2126*channel(1) + 0.7152*channel(3) + 0.0722*channel(5)
}

func TestEveryThemeIsComplete(t *testing.T) {
	names := ThemeNames()
	if len(names) < 6 || names[0] != "default" {
		t.Fatalf("ThemeNames() = %v, want default first and at least six themes", names)
	}
	for _, name := range names {
		theme, ok := lookupTheme(name)
		if !ok {
			t.Fatalf("%s is listed but cannot be found", name)
		}
		for field, value := range map[string]string{
			"accent": theme.accent, "healthy": theme.healthy, "info": theme.info, "warning": theme.warning,
			"danger": theme.danger, "suspended": theme.suspended, "muted": theme.muted, "text": theme.text,
			"bright": theme.bright, "border": theme.border, "surface": theme.surface,
			"added foreground": theme.addedFG, "added background": theme.addedBG,
			"removed foreground": theme.removedFG, "removed background": theme.removedBG,
			"selection": theme.selection, "badge text": theme.badgeText,
		} {
			if value == "" {
				t.Errorf("theme %s has no %s color", name, field)
			}
		}
	}
}

func TestSetThemeRejectsUnknownNamesAndListsTheRealOnes(t *testing.T) {
	err := setTheme("neon")

	if err == nil || !strings.Contains(err.Error(), `"neon"`) || !strings.Contains(err.Error(), "nord") {
		t.Fatalf("error = %v, want the bad name and the available themes", err)
	}
}

func TestSwitchingThemeRepaintsTheStyles(t *testing.T) {
	useTheme(t, "default")
	before := accentStyle.Render("x") + errorStyle.Render("x") + diffAddedStyle.Render("x") + updateBadgeStyle.Render("x") + selectedText.Render("x")

	useTheme(t, "nord")
	after := accentStyle.Render("x") + errorStyle.Render("x") + diffAddedStyle.Render("x") + updateBadgeStyle.Render("x") + selectedText.Render("x")

	if before == after {
		t.Fatal("changing the theme left every style as it was")
	}
	if got := statusBadge("Degraded"); got == statusBadge("Healthy") {
		t.Fatal("the status colors collapsed to one color")
	}
}

func TestStatusColorsFollowTheTheme(t *testing.T) {
	useTheme(t, "default")
	healthy, degraded := statusBadge("Healthy"), statusBadge("Degraded")

	useTheme(t, "colorblind")

	if statusBadge("Healthy") == healthy || statusBadge("Degraded") == degraded {
		t.Fatal("the health colors did not change with the theme")
	}
}

func TestLightThemeUsesDarkTextAndDarkThemesUseLightText(t *testing.T) {
	for _, name := range ThemeNames() {
		theme, _ := lookupTheme(name)
		if theme.text[0] != '#' {
			continue // the terminal theme uses the terminal's own colors
		}
		light := luminance(t, theme.text) < 0.5
		if (name == "light") != light {
			t.Errorf("theme %s: text %s is %s, which does not suit its background", name, theme.text, map[bool]string{true: "dark", false: "light"}[light])
		}
		if name == "light" && luminance(t, theme.bright) > 0.2 {
			t.Errorf("the light theme's bright text %s is too pale", theme.bright)
		}
	}
}

func TestEveryThemeKeepsHealthyAndDegradedApart(t *testing.T) {
	for _, name := range ThemeNames() {
		theme, _ := lookupTheme(name)
		if theme.healthy == theme.danger || theme.healthy == theme.warning || theme.danger == theme.warning {
			t.Errorf("theme %s uses one color for different statuses: %s / %s / %s", name, theme.healthy, theme.warning, theme.danger)
		}
	}
}

func TestThemeCommandSwitchesAndSaysSo(t *testing.T) {
	useTheme(t, "default")
	model, _ := sortModel()

	model = command(model, ":theme dracula")

	if currentTheme != "dracula" || !strings.Contains(model.status, "dracula") {
		t.Fatalf("theme = %q, status = %q; want dracula", currentTheme, model.status)
	}
	if bad := command(model, ":theme neon"); bad.err == nil || !strings.Contains(bad.err.Error(), "nord") || currentTheme != "dracula" {
		t.Fatalf("a bad theme: err = %v, theme = %q; want it refused and the real names listed", bad.err, currentTheme)
	}
}

func TestThemeNamesAreSuggestedAndCycleWithTab(t *testing.T) {
	useTheme(t, "default")
	model, _ := sortModel()

	typed := typeKeys(model, ":", "t", "h", "e", "m", "e", " ")
	if view := typed.View(); !strings.Contains(view, "theme colorblind") || !strings.Contains(view, "theme default") {
		t.Fatalf(":theme did not suggest the themes:\n%s", view)
	}
	if len(typed.suggestions()) != len(ThemeNames()) {
		t.Fatalf("suggestions = %v, want every theme", typed.suggestions())
	}
	first := press(typed, "tab").prompt.input
	second := press(press(typed, "tab"), "tab").prompt.input
	if first == second || !strings.HasPrefix(first, "theme ") || !strings.HasPrefix(second, "theme ") {
		t.Fatalf("Tab gave %q then %q, want different themes", first, second)
	}
}
