package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/muesli/termenv"
)

func TestThemesThatNeedABackgroundHaveOne(t *testing.T) {
	light, _ := lookupTheme("light")
	if light.background == "" {
		t.Fatal("the light theme has dark text; without its own background it is unreadable on a dark terminal")
	}
	for _, name := range []string{"default", "terminal", "high-contrast", "colorblind"} {
		if theme, _ := lookupTheme(name); theme.background != "" {
			t.Errorf("theme %s paints a background %q; it should keep the terminal's own", name, theme.background)
		}
	}
}

func TestPaintBackgroundIsANoOpForThemesWithoutOne(t *testing.T) {
	useTheme(t, "default")

	if got := paintBackground("a\nb", 10, 5); got != "a\nb" {
		t.Fatalf("paintBackground changed the view for a theme without a background: %q", got)
	}
}

func TestPaintBackgroundFillsEveryCellAndSurvivesInnerResets(t *testing.T) {
	useTheme(t, "light")
	prefix, _, _ := strings.Cut(lipgloss.NewStyle().Background(themeBackground).Render("x"), "x")
	if prefix == "" {
		t.Fatal("setup: the background produced no escape code")
	}
	view := "plain\n" + accentStyle.Render("colored") + " and " + mutedStyle.Render("more")

	lines := strings.Split(paintBackground(view, 40, 4), "\n")

	if len(lines) != 4 {
		t.Fatalf("got %d lines, want the full height of 4", len(lines))
	}
	for index, line := range lines {
		if !strings.HasPrefix(line, prefix) || ansi.StringWidth(line) != 40 {
			t.Errorf("line %d does not start with the background or is %d wide: %q", index, ansi.StringWidth(line), line)
		}
		rest := line
		for {
			at := strings.Index(rest, "\x1b[0m")
			if at < 0 || at+4 >= len(rest) {
				break
			}
			if !strings.HasPrefix(rest[at+4:], prefix) && !strings.HasSuffix(rest[:at+4], "\x1b[0m") {
				t.Errorf("line %d: the background is lost after a reset: %q", index, rest[at:])
				break
			}
			rest = rest[at+4:]
		}
	}
}

func TestTheViewPaintsTheWholeScreenWithTheLightBackground(t *testing.T) {
	useTheme(t, "light")
	model, _ := sortModel()

	lines := strings.Split(resize(model, 100, 30).View(), "\n")

	if len(lines) != 30 {
		t.Fatalf("the view is %d lines, want the 30 of the terminal", len(lines))
	}
	for index, line := range lines {
		if ansi.StringWidth(line) != 100 {
			t.Errorf("line %d is %d columns wide, want 100", index, ansi.StringWidth(line))
		}
	}
}

func TestSwitchingBackFromALightThemeRemovesTheBackground(t *testing.T) {
	useTheme(t, "light")
	model, _ := sortModel()
	model = resize(model, 100, 30)
	painted := model.View()

	if err := setTheme("default"); err != nil {
		t.Fatal(err)
	}
	plain := model.View()

	if painted == plain || strings.Contains(plain, "48;2;248;250;252") {
		t.Fatal("the light background is still drawn after switching to the default theme")
	}
}

func TestNoColorTerminalsGetNoEscapeCodesFromTheBackground(t *testing.T) {
	useTheme(t, "light")
	previous := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.Ascii)
	t.Cleanup(func() { lipgloss.SetColorProfile(previous) })

	if got := paintBackground("hello", 20, 2); strings.Contains(got, "\x1b") {
		t.Fatalf("an Ascii terminal got escape codes: %q", got)
	}
}

func TestACustomThemeCanSetItsOwnBackground(t *testing.T) {
	registered(t, "paper", "light", map[string]string{"background": "#fffdf5"})

	custom, _ := lookupTheme("paper")
	if custom.background != "#fffdf5" {
		t.Fatalf("background = %q", custom.background)
	}
	registered(t, "plain", "light", map[string]string{"background": "none"})
	if cleared, _ := lookupTheme("plain"); cleared.background != "" {
		t.Fatalf("background %q; \"none\" should give the terminal's own", cleared.background)
	}
}
