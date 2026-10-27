package main

import (
	"fmt"
	"slices"
	"strconv"

	"github.com/awcodify/argoxd/internal/tui"
)

// registerThemes adds the themes defined in the config file, in name order so
// that a mistake is always reported for the same theme first. A theme's "base"
// names the theme it starts from; every other key is a color.
func registerThemes(defined map[string]customTheme) error {
	names := make([]string, 0, len(defined))
	for name := range defined {
		names = append(names, name)
	}
	slices.Sort(names)
	for _, name := range names {
		colors := map[string]string{}
		base := ""
		for key, value := range defined[name] {
			text, err := themeText(value)
			if err != nil {
				return fmt.Errorf("themes.%s.%s: %w", name, key, err)
			}
			if key == "base" {
				base = text
				continue
			}
			colors[key] = text
		}
		if err := tui.RegisterTheme(name, base, colors); err != nil {
			return fmt.Errorf("themes.%s: %w", name, err)
		}
	}
	return nil
}

// themeText turns a value from the config file into text. A color written as a
// bare number, such as 208, arrives as a number; a hex color written without
// quotes arrives as nothing, because YAML reads "#" as the start of a comment.
func themeText(value any) (string, error) {
	switch typed := value.(type) {
	case string:
		return typed, nil
	case float64:
		return strconv.FormatFloat(typed, 'f', -1, 64), nil
	case nil:
		return "", fmt.Errorf(`the value is empty; if it is a hex color, put it in quotes, as in "#ff5500", because YAML reads # as a comment`)
	}
	return "", fmt.Errorf("a color is text such as \"#ff5500\" or an ANSI number such as 208, not %v", value)
}
