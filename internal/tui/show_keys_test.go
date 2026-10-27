package tui

import (
	"fmt"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/awcodify/argoxd/internal/explorer"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/muesli/termenv"
)

func TestKeyLabelsAreReadableForSpecialKeys(t *testing.T) {
	for _, test := range []struct {
		key  tea.KeyMsg
		want string
	}{
		{tea.KeyMsg{Type: tea.KeyEnter}, "enter"},
		{tea.KeyMsg{Type: tea.KeyEsc}, "esc"},
		{tea.KeyMsg{Type: tea.KeyTab}, "tab"},
		{tea.KeyMsg{Type: tea.KeyShiftTab}, "⇧ tab"},
		{tea.KeyMsg{Type: tea.KeyUp}, "↑"},
		{tea.KeyMsg{Type: tea.KeyDown}, "↓"},
		{tea.KeyMsg{Type: tea.KeyLeft}, "←"},
		{tea.KeyMsg{Type: tea.KeyRight}, "→"},
		{tea.KeyMsg{Type: tea.KeyBackspace}, "⌫"},
		{tea.KeyMsg{Type: tea.KeySpace}, "space"},
		{tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(" ")}, "space"},
		{tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("w")}, "w"},
		{tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("E")}, "E"},
		{tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(":")}, ":"},
		{tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("[")}, "["},
		{tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("é")}, "é"},
		{tea.KeyMsg{Type: tea.KeyCtrlC}, "ctrl+c"},
	} {
		if got := keyLabel(test.key); got != test.want {
			t.Errorf("keyLabel(%q) = %q, want %q", test.key.String(), got, test.want)
		}
	}
}

func keysModel() Model {
	model, _ := sortModel()
	return model.WithShowKeys(true)
}

// badgeRow returns the index of the line that names the pressed key, or -1 when
// no badge is on screen. The label "key " is unique to the badge.
func badgeRow(view string) int {
	for index, line := range strings.Split(view, "\n") {
		if strings.Contains(ansi.Strip(line), "key ") {
			return index
		}
	}
	return -1
}

func TestWithShowKeysFloatsEachPressedKeyInTheMiddle(t *testing.T) {
	model := keysModel()
	if badgeRow(model.View()) >= 0 {
		t.Fatal("a key badge showed before any key was pressed")
	}

	model = press(model, "j")

	view := model.View()
	lines := strings.Split(view, "\n")
	row := badgeRow(view)
	if row < 0 {
		t.Fatalf("the pressed key did not show:\n%s", view)
	}
	if row < len(lines)/3 || row > len(lines)*2/3 {
		t.Fatalf("the badge is at row %d of %d, want it near the middle", row, len(lines))
	}
	if line := ansi.Strip(lines[row]); !strings.Contains(line, "j") {
		t.Fatalf("the badge line %q does not name the pressed key", line)
	}
}

// cellBackgrounds reads the background color of every cell of a rendered line,
// so a test can check that an overlay left no hole in its sheet. Colors come
// back as "#rrggbb", or "" for the terminal's own background.
func cellBackgrounds(line string) []string {
	backgrounds := make([]string, 0, ansi.StringWidth(line))
	background := ""
	for len(line) > 0 {
		if strings.HasPrefix(line, "\x1b[") {
			end := strings.IndexByte(line, 'm')
			if end < 0 {
				break
			}
			background = backgroundAfter(line[2:end], background)
			line = line[end+1:]
			continue
		}
		character, size := utf8.DecodeRuneInString(line)
		for range max(1, ansi.StringWidth(string(character))) {
			backgrounds = append(backgrounds, background)
		}
		line = line[size:]
	}
	return backgrounds
}

// backgroundAfter applies one SGR sequence to the current background.
func backgroundAfter(parameters, background string) string {
	values := strings.Split(parameters, ";")
	number := func(index int) int {
		value, _ := strconv.Atoi(values[index])
		return value
	}
	for index := 0; index < len(values); index++ {
		switch number(index) {
		case 0, 49:
			background = ""
		case 48:
			if index+1 < len(values) && number(index+1) == 5 && index+2 < len(values) {
				background = fmt.Sprintf("color%d", number(index+2))
				index += 2
			} else if index+1 < len(values) && number(index+1) == 2 && index+4 < len(values) {
				background = fmt.Sprintf("#%02x%02x%02x", number(index+2), number(index+3), number(index+4))
				index += 4
			}
		}
	}
	return background
}

// TestTheBandLeavesNoGapBehindTheKey is the regression test for the bug where
// the keycap was cut into the band with cells that had no background of their
// own, so the page showed through around and inside the key.
func TestTheBandLeavesNoGapBehindTheKey(t *testing.T) {
	previous := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	defer lipgloss.SetColorProfile(previous)

	model := press(keysModel(), "j")
	lines := strings.Split(model.View(), "\n")
	row := badgeRow(model.View())
	if row < 0 {
		t.Fatalf("the pressed key did not show:\n%s", model.View())
	}

	backgrounds := cellBackgrounds(lines[row])
	if len(backgrounds) != model.width {
		t.Fatalf("the band row has %d cells, want %d", len(backgrounds), model.width)
	}
	scrim := strings.ToLower(string(keyScrim))
	key := strings.ToLower(string(keyBadgeStyle.GetBackground().(lipgloss.Color)))
	for column, background := range backgrounds {
		if background != scrim && background != key {
			t.Fatalf("cell %d of the band has background %q, want the sheet %s or the key %s: %q",
				column, background, scrim, key, ansi.Strip(lines[row]))
		}
	}
}

// TestTheBandSpansTheWholeScreen checks that the caption is a layer of its own:
// it covers every column, including the frame's edges, and keeps the key in the
// middle of the screen.
func TestTheBandSpansTheWholeScreen(t *testing.T) {
	model := press(keysModel(), "j")

	lines := strings.Split(model.View(), "\n")
	row := badgeRow(model.View())
	if row < 0 {
		t.Fatalf("the pressed key did not show:\n%s", model.View())
	}
	if width := ansi.StringWidth(lines[row]); width != model.width {
		t.Fatalf("the band is %d columns wide, want the whole %d-column screen", width, model.width)
	}
	stripped := ansi.Strip(lines[row])
	column := ansi.StringWidth(stripped[:strings.Index(stripped, "key")])
	if column < model.width/2-20 || column > model.width/2 {
		t.Fatalf("the key starts at column %d of %d, want it centered", column, model.width)
	}
}

func TestTheOverlayKeepsTheContentBehindItVisible(t *testing.T) {
	model := press(keysModel(), "j")

	lines := strings.Split(model.View(), "\n")
	row := badgeRow(model.View())
	if line := ansi.Strip(lines[row]); !strings.HasPrefix(line, "│") || !strings.HasSuffix(line, "│") {
		t.Fatalf("the badge hid the frame behind it: %q", line)
	}
}

func TestTheBadgeShowsTheLatestKeyAndNamesSpecialOnes(t *testing.T) {
	model := press(press(keysModel(), "j"), "enter")

	lines := strings.Split(model.View(), "\n")
	if row := badgeRow(model.View()); row < 0 {
		t.Fatal("the badge did not show")
	} else if line := ansi.Strip(lines[row]); !strings.Contains(line, "enter") || strings.Contains(line, " j") {
		t.Fatalf("badge = %q, want only the newest key", line)
	}
}

func TestTheBadgeFadesWhenNoNewerKeyHasBeenPressed(t *testing.T) {
	model := press(keysModel(), "j")
	stale := model.keyPress

	faded, _ := model.Update(keyBadgeExpired{press: stale})

	if badgeRow(faded.(Model).View()) >= 0 {
		t.Fatal("the badge did not fade")
	}
}

func TestAnOlderFadeNeverHidesANewerKey(t *testing.T) {
	model := press(keysModel(), "j")
	older := model.keyPress
	model = press(model, "k")

	kept, _ := model.Update(keyBadgeExpired{press: older})

	lines := strings.Split(kept.(Model).View(), "\n")
	if row := badgeRow(kept.(Model).View()); row < 0 {
		t.Fatal("the badge did not show")
	} else if line := ansi.Strip(lines[row]); !strings.Contains(line, "k") || !strings.Contains(line, "key ") {
		t.Fatalf("an old timer hid the newer key: %q", line)
	}
}

func TestEveryKeyPressStartsAFadeTimer(t *testing.T) {
	_, command := keysModel().Update(key("j"))

	if command == nil {
		t.Fatal("pressing a key with --show-keys returned no command to fade the badge")
	}
}

func TestKeysAreNotShownByDefault(t *testing.T) {
	model, _ := sortModel()

	pressed := press(model, "j")
	_, command := model.Update(key("j"))

	if badgeRow(pressed.View()) >= 0 || command != nil {
		t.Fatalf("keys are shown or timed without --show-keys: command %v", command != nil)
	}
}

func TestTheKeyBadgeAndTheUpdateBadgeLiveTogether(t *testing.T) {
	model := press(keysModel(), "j")
	updated, _ := model.Update(updateAvailable{version: "0.5.0"})

	view := updated.(Model).View()
	lines := strings.Split(ansi.Strip(view), "\n")
	if !strings.Contains(lines[len(lines)-2], "0.5.0 available") {
		t.Fatalf("the update badge is missing from the breadcrumb row:\n%s", strings.Join(lines[len(lines)-3:], "\n"))
	}
	if badgeRow(view) < 0 {
		t.Fatal("the key badge is missing")
	}
}

func TestTheBadgeFitsOnANarrowTerminal(t *testing.T) {
	model := press(resize(keysModel(), 40, 20), "j")
	model.status = strings.Repeat("a long status message ", 5)

	for _, line := range strings.Split(model.View(), "\n") {
		if ansi.StringWidth(line) > 40 {
			t.Fatalf("a line is %d wide, wider than the 40-column terminal: %q", ansi.StringWidth(line), ansi.Strip(line))
		}
	}
	if badgeRow(model.View()) < 0 {
		t.Fatal("the badge is missing on a narrow terminal")
	}
}

func TestALongKeyNameStillFitsTheBand(t *testing.T) {
	model := press(resize(keysModel(), 40, 20), "shift+tab")

	view := model.View()
	if row := badgeRow(view); row < 0 {
		t.Fatalf("the pressed key did not show:\n%s", view)
	}
	for _, line := range strings.Split(view, "\n") {
		if width := ansi.StringWidth(line); width > 40 {
			t.Fatalf("a line is %d wide, wider than the 40-column terminal: %q", width, ansi.Strip(line))
		}
	}
}

func TestTheBandStaysBelowTheHeader(t *testing.T) {
	for _, height := range []int{12, 14, 20} {
		model := press(resize(keysModel(), 60, height), "j")
		row := badgeRow(model.View())
		if row < 0 {
			continue
		}
		if row-keyBandRows/2 < headerHeight {
			t.Fatalf("on a %d-row terminal the band starts at row %d, over the %d-row header", height, row-keyBandRows/2, headerHeight)
		}
	}
}

func TestTheBadgeAlsoShowsKeysTypedIntoAPrompt(t *testing.T) {
	model := typeKeys(keysModel(), ":", "s")

	lines := strings.Split(model.View(), "\n")
	if row := badgeRow(model.View()); row < 0 {
		t.Fatal("a key typed in the command bar is not shown")
	} else if line := ansi.Strip(lines[row]); !strings.Contains(line, "s") {
		t.Fatalf("the badge does not name the typed key: %q", line)
	}
}

func TestTheBandTakesItsToneFromTheTheme(t *testing.T) {
	defer func() { _ = setTheme("default") }()

	if err := setTheme("light"); err != nil {
		t.Fatal(err)
	}
	if !isLightColor(string(keyScrim)) {
		t.Fatalf("the light theme's sheet is %s, want a light one", keyScrim)
	}

	if err := setTheme("default"); err != nil {
		t.Fatal(err)
	}
	if isLightColor(string(keyScrim)) {
		t.Fatalf("the dark theme's sheet is %s, want a dark one", keyScrim)
	}
}

var _ = explorer.Snapshot{}
