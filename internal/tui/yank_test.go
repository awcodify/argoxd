package tui

import (
	"errors"
	"strings"
	"testing"

	"github.com/awcodify/argoxd/internal/explorer"
)

func TestYankCopiesSelectedItemAndViewerText(t *testing.T) {
	var copied []string
	copyToClipboard = func(text string) error {
		copied = append(copied, text)
		return nil
	}
	t.Cleanup(func() { copyToClipboard = systemCopyToClipboard })

	source := &fakeSource{snapshot: storeSnapshot(), tree: checkoutTree(), manifest: "kind: Deployment\nmetadata:\n  name: web\n"}
	list := settle(New(source, "test", "argocd", explorer.Snapshot{}), source.loadCommand())
	list = press(list, "y")
	if len(copied) != 1 || copied[0] != "grafana" || !strings.Contains(list.View(), "Copied to clipboard") {
		t.Fatalf("list copy = %v:\n%s", copied, list.View())
	}

	viewer := run(press(openCheckout(t, source), "j"), "Y")
	viewer = press(viewer, "y")
	if len(copied) != 2 || copied[1] != strings.TrimRight(source.manifest, "\n") {
		t.Fatalf("viewer copy = %v, want manifest", copied)
	}
}

func TestYankInViewerUppercaseY(t *testing.T) {
	var copied []string
	copyToClipboard = func(text string) error {
		copied = append(copied, text)
		return nil
	}
	t.Cleanup(func() { copyToClipboard = systemCopyToClipboard })

	source := &fakeSource{snapshot: storeSnapshot(), tree: checkoutTree(), manifest: "kind: Deployment\nmetadata:\n  name: web\n"}
	viewer := run(press(openCheckout(t, source), "j"), "Y")
	viewer = press(viewer, "Y")
	if len(copied) != 1 || copied[0] != strings.TrimRight(source.manifest, "\n") {
		t.Fatalf("viewer uppercase Y copy = %v, want manifest", copied)
	}
}

func TestYankVisualSelectionInViewer(t *testing.T) {
	var copied []string
	copyToClipboard = func(text string) error {
		copied = append(copied, text)
		return nil
	}
	t.Cleanup(func() { copyToClipboard = systemCopyToClipboard })

	manifest := "line 1\nline 2\nline 3\nline 4\nline 5\n"
	source := &fakeSource{snapshot: storeSnapshot(), tree: checkoutTree(), manifest: manifest}
	viewer := run(press(openCheckout(t, source), "j"), "Y")

	// Enter visual selection mode with 'v'
	viewer = press(viewer, "v")
	if !strings.Contains(viewer.View(), "Select") {
		t.Fatalf("visual mode hints missing in view:\n%s", viewer.View())
	}

	// Move cursor down 2 lines (selecting lines 1, 2, 3)
	viewer = press(press(viewer, "j"), "j")

	// Yank selection with 'y'
	viewer = press(viewer, "y")
	want := "line 1\nline 2\nline 3"
	if len(copied) != 1 || copied[0] != want {
		t.Fatalf("copied visual selection = %q, want %q", copied, want)
	}
	if !strings.Contains(viewer.View(), "Copied 3 lines to clipboard") {
		t.Fatalf("expected status message 'Copied 3 lines to clipboard':\n%s", viewer.View())
	}
}

func TestYankVisualSelectionCancel(t *testing.T) {
	manifest := "line 1\nline 2\nline 3\n"
	source := &fakeSource{snapshot: storeSnapshot(), tree: checkoutTree(), manifest: manifest}
	viewer := run(press(openCheckout(t, source), "j"), "Y")

	viewer = press(viewer, "v")
	if !viewer.viewer.visual {
		t.Fatalf("expected visual mode active")
	}

	viewer = press(viewer, "esc")
	if viewer.viewer.visual {
		t.Fatalf("expected visual mode canceled by esc")
	}
	if viewer.view != textViewMode {
		t.Fatalf("expected to stay in textViewMode after esc in visual mode, got %v", viewer.view)
	}
}

func TestYankCopiesMarkedApplications(t *testing.T) {
	var copied []string
	copyToClipboard = func(text string) error {
		copied = append(copied, text)
		return nil
	}
	t.Cleanup(func() { copyToClipboard = systemCopyToClipboard })

	source := &fakeSource{snapshot: storeSnapshot(), tree: checkoutTree()}
	list := settle(New(source, "test", "argocd", explorer.Snapshot{}), source.loadCommand())

	// Mark first two applications with space
	list = press(list, " ")
	list = press(list, " ")

	list = press(list, "y")
	if len(copied) != 1 {
		t.Fatalf("expected 1 copy, got %v", copied)
	}
	if !strings.Contains(copied[0], "\n") {
		t.Fatalf("expected multiple applications joined by newline, got %q", copied[0])
	}
	if !strings.Contains(list.View(), "items to clipboard") {
		t.Fatalf("expected status 'Copied X items to clipboard':\n%s", list.View())
	}
}

func TestYankReportsClipboardFailures(t *testing.T) {
	copyToClipboard = func(string) error { return errors.New("clipboard unavailable") }
	t.Cleanup(func() { copyToClipboard = systemCopyToClipboard })

	model := press(New(nil, "test", "argocd", storeSnapshot()), "y")

	if view := model.View(); !strings.Contains(view, "clipboard unavailable") {
		t.Fatalf("clipboard error was not shown:\n%s", view)
	}
}
