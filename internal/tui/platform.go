package tui

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"
)

var (
	copyToClipboard = systemCopyToClipboard
	openInBrowser   = systemOpenInBrowser
)

func systemCopyToClipboard(text string) error {
	switch runtime.GOOS {
	case "darwin":
		return runClipboardCommand(text, "pbcopy")
	case "windows":
		return runClipboardCommand(text, "clip")
	default:
		if err := runClipboardCommand(text, "wl-copy"); err == nil {
			return nil
		}
		return runClipboardCommand(text, "xclip", "-selection", "clipboard")
	}
}

func runClipboardCommand(text, name string, arguments ...string) error {
	command := exec.Command(name, arguments...)
	command.Stdin = strings.NewReader(text)
	if err := command.Run(); err != nil {
		return fmt.Errorf("copy to clipboard: %w", err)
	}
	return nil
}

func systemOpenInBrowser(url string) error {
	var command *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		command = exec.Command("open", url)
	case "windows":
		command = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		command = exec.Command("xdg-open", url)
	}
	if err := command.Run(); err != nil {
		return fmt.Errorf("open browser: %w", err)
	}
	return nil
}
