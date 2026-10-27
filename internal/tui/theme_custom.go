package tui

import (
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

// colorKeys are the names a custom theme may set, as written in the config
// file, each with the field of a theme it sets.
var colorKeys = map[string]func(*theme) *string{
	"accent":            func(t *theme) *string { return &t.accent },
	"healthy":           func(t *theme) *string { return &t.healthy },
	"info":              func(t *theme) *string { return &t.info },
	"warning":           func(t *theme) *string { return &t.warning },
	"danger":            func(t *theme) *string { return &t.danger },
	"suspended":         func(t *theme) *string { return &t.suspended },
	"muted":             func(t *theme) *string { return &t.muted },
	"text":              func(t *theme) *string { return &t.text },
	"bright":            func(t *theme) *string { return &t.bright },
	"border":            func(t *theme) *string { return &t.border },
	"surface":           func(t *theme) *string { return &t.surface },
	"addedText":         func(t *theme) *string { return &t.addedFG },
	"addedBackground":   func(t *theme) *string { return &t.addedBG },
	"removedText":       func(t *theme) *string { return &t.removedFG },
	"removedBackground": func(t *theme) *string { return &t.removedBG },
	"selection":         func(t *theme) *string { return &t.selection },
	"badgeText":         func(t *theme) *string { return &t.badgeText },
	// "none" gives the terminal's own background.
	"background": func(t *theme) *string { return &t.background },
}

// ThemeColorKeys lists the colors a custom theme can set, sorted.
func ThemeColorKeys() []string {
	keys := make([]string, 0, len(colorKeys))
	for key := range colorKeys {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	return keys
}

// builtinThemeCount is how many themes ship with argoxd; custom ones follow them.
var builtinThemeCount = len(themes)

var (
	hexColor = regexp.MustCompile(`^#([0-9a-fA-F]{3}|[0-9a-fA-F]{6})$`)
	// A theme name is one word, because ":theme <name>" takes a single argument.
	themeName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]*$`)
)

// normalizeColor accepts #rrggbb, #rgb or an ANSI color number from 0 to 255,
// and returns it as #rrggbb, lower case, or the plain number.
func normalizeColor(value string) (string, bool) {
	value = strings.TrimSpace(value)
	if hexColor.MatchString(value) {
		value = strings.ToLower(value)
		if len(value) == 4 {
			value = "#" + strings.Repeat(string(value[1]), 2) + strings.Repeat(string(value[2]), 2) + strings.Repeat(string(value[3]), 2)
		}
		return value, true
	}
	if number, err := strconv.Atoi(value); err == nil && number >= 0 && number <= 255 && value == strconv.Itoa(number) {
		return value, true
	}
	return "", false
}

// RegisterTheme adds a theme of your own, listed after the built-in ones. It
// starts from the base theme (the default when base is empty) and changes the
// colors given, by name: see ThemeColorKeys. Registering a name again replaces
// that theme.
func RegisterTheme(name, base string, colors map[string]string) error {
	if !themeName.MatchString(name) {
		return fmt.Errorf("theme name %q must be one word of letters, digits, - or _", name)
	}
	if index := slices.IndexFunc(themes[:builtinThemeCount], func(t theme) bool { return t.name == strings.ToLower(name) }); index >= 0 {
		return fmt.Errorf("theme %q is a built-in theme; choose another name", name)
	}
	if base == "" {
		base = "default"
	}
	custom, ok := lookupTheme(base)
	if !ok {
		return fmt.Errorf("base %q not found; the themes are %s", base, strings.Join(ThemeNames(), ", "))
	}
	custom.name = strings.ToLower(name)
	keys := make([]string, 0, len(colors))
	for key := range colors {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	for _, key := range keys {
		field, known := colorKeys[key]
		if !known {
			return fmt.Errorf("%q is not a theme color; the colors are %s", key, strings.Join(ThemeColorKeys(), ", "))
		}
		if key == "background" && strings.EqualFold(strings.TrimSpace(colors[key]), "none") {
			custom.background = ""
			continue
		}
		value, valid := normalizeColor(colors[key])
		if !valid {
			return fmt.Errorf("%s: %q is not a color; use #rrggbb, #rgb or an ANSI number from 0 to 255", key, colors[key])
		}
		*field(&custom) = value
	}
	if index := slices.IndexFunc(themes[builtinThemeCount:], func(t theme) bool { return t.name == custom.name }); index >= 0 {
		themes[builtinThemeCount+index] = custom
	} else {
		themes = append(themes, custom)
	}
	return nil
}

// ForgetCustomThemes removes every theme added with RegisterTheme.
func ForgetCustomThemes() {
	themes = themes[:builtinThemeCount]
}
