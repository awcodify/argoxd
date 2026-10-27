package tui

import (
	"fmt"
	"strings"
)

// theme is a complete set of colors, written as #rrggbb, or as ANSI numbers
// for the theme that uses the terminal's own palette.
type theme struct {
	name string
	// accent marks the selection and keys; it is also the update badge's fill.
	accent string
	// healthy, info, warning, danger and suspended color the statuses: Healthy
	// and Synced, Progressing, OutOfSync, Degraded and Suspended.
	healthy, info, warning, danger, suspended string
	// muted is for labels and borders of context, text for ordinary text and
	// bright for names and the selected row.
	muted, text, bright string
	border, surface     string
	// The tinted bands of a diff, and the background of selected lines.
	addedFG, addedBG, removedFG, removedBG string
	selection                              string
	// badgeText is drawn on the accent color.
	badgeText string
}

var themes = []theme{
	{
		name: "default", accent: "#EF7B4D", healthy: "#18BE94", info: "#0DADEA", warning: "#F4C030", danger: "#E96D76", suspended: "#A78BFA",
		muted: "#64748B", text: "#CBD5E1", bright: "#F8FAFC", border: "#475569", surface: "#1E293B",
		addedFG: "#BBF7D0", addedBG: "#12372A", removedFG: "#FECDD3", removedBG: "#4A1D27", selection: "#1E3A5F", badgeText: "#0D1117",
	},
	{
		name: "light", accent: "#C2410C", healthy: "#047857", info: "#0369A1", warning: "#B45309", danger: "#BE123C", suspended: "#6D28D9",
		muted: "#64748B", text: "#334155", bright: "#0F172A", border: "#94A3B8", surface: "#E2E8F0",
		addedFG: "#14532D", addedBG: "#BBF7D0", removedFG: "#7F1D1D", removedBG: "#FECDD3", selection: "#BFDBFE", badgeText: "#FFFFFF",
	},
	{
		name: "high-contrast", accent: "#FFB000", healthy: "#00FF7F", info: "#00BFFF", warning: "#FFFF00", danger: "#FF5555", suspended: "#D7A6FF",
		muted: "#B0B0B0", text: "#FFFFFF", bright: "#FFFFFF", border: "#BBBBBB", surface: "#303030",
		addedFG: "#FFFFFF", addedBG: "#006400", removedFG: "#FFFFFF", removedBG: "#8B0000", selection: "#00008B", badgeText: "#000000",
	},
	{
		// Okabe-Ito colors: healthy is blue and degraded vermillion, so the two
		// differ for the common kinds of color blindness.
		name: "colorblind", accent: "#E69F00", healthy: "#56B4E9", info: "#0072B2", warning: "#F0E442", danger: "#D55E00", suspended: "#CC79A7",
		muted: "#8A94A6", text: "#CBD5E1", bright: "#F8FAFC", border: "#475569", surface: "#1E293B",
		addedFG: "#E0F2FE", addedBG: "#0B3A5C", removedFG: "#FFEDD5", removedBG: "#5C2A00", selection: "#1E3A5F", badgeText: "#0D1117",
	},
	{
		name: "nord", accent: "#D08770", healthy: "#A3BE8C", info: "#88C0D0", warning: "#EBCB8B", danger: "#BF616A", suspended: "#B48EAD",
		muted: "#7B88A1", text: "#D8DEE9", bright: "#ECEFF4", border: "#4C566A", surface: "#3B4252",
		addedFG: "#E5F0DD", addedBG: "#2F4A3A", removedFG: "#F3DADD", removedBG: "#4F2F35", selection: "#434C5E", badgeText: "#2E3440",
	},
	{
		name: "dracula", accent: "#FF79C6", healthy: "#50FA7B", info: "#8BE9FD", warning: "#F1FA8C", danger: "#FF5555", suspended: "#BD93F9",
		muted: "#6272A4", text: "#F8F8F2", bright: "#FFFFFF", border: "#44475A", surface: "#343746",
		addedFG: "#D7FFE0", addedBG: "#1F4A2E", removedFG: "#FFD7D7", removedBG: "#5A2230", selection: "#44475A", badgeText: "#282A36",
	},
	{
		// The terminal's own 16 colors, so argoxd follows whatever scheme it runs in.
		name: "terminal", accent: "9", healthy: "2", info: "6", warning: "3", danger: "1", suspended: "5",
		muted: "8", text: "7", bright: "15", border: "8", surface: "0",
		addedFG: "2", addedBG: "0", removedFG: "1", removedBG: "0", selection: "8", badgeText: "0",
	},
}

// currentTheme is the name of the theme in use.
var currentTheme = "default"

// ThemeNames lists the themes, the default first.
func ThemeNames() []string {
	names := make([]string, len(themes))
	for index, t := range themes {
		names[index] = t.name
	}
	return names
}

func lookupTheme(name string) (theme, bool) {
	for _, t := range themes {
		if t.name == strings.ToLower(strings.TrimSpace(name)) {
			return t, true
		}
	}
	return theme{}, false
}

// setTheme applies the named theme to everything argoxd draws.
func setTheme(name string) error {
	t, ok := lookupTheme(name)
	if !ok {
		return fmt.Errorf("theme %q not found; the themes are %s", name, strings.Join(ThemeNames(), ", "))
	}
	applyTheme(t)
	currentTheme = t.name
	return nil
}

// SetTheme chooses the theme before the interface starts.
func SetTheme(name string) error { return setTheme(name) }

// themeCommand is ":theme": with a name it switches, without one it says which
// theme is in use and which others there are.
const themeCommand = "theme"

func (m *Model) runThemeCommand(name string) {
	if name == "" {
		m.err = nil
		m.status = "Theme " + currentTheme + "; others: " + strings.Join(without(ThemeNames(), currentTheme), ", ")
		return
	}
	if err := setTheme(name); err != nil {
		m.err = err
		return
	}
	m.err = nil
	m.status = "Theme " + currentTheme
}

func without(names []string, drop string) []string {
	kept := make([]string, 0, len(names))
	for _, name := range names {
		if name != drop {
			kept = append(kept, name)
		}
	}
	return kept
}
