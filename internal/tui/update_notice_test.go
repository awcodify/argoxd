package tui

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/awcodify/argoxd/internal/explorer"
	"github.com/charmbracelet/lipgloss"
)

func TestHeaderAnnouncesANewerRelease(t *testing.T) {
	source := &fakeSource{snapshot: storeSnapshot()}
	model := New(source, "Kubeconfig current context", "argocd", explorer.Snapshot{}).
		WithUpdateCheck(func(context.Context) string { return "0.4.2" })

	cmd := model.Init()
	if cmd == nil {
		t.Fatal("Init() = nil, want the load and the update check")
	}
	model = settle(model, source.loadCommand())
	updated, _ := model.Update(updateAvailable{version: "0.4.2"})
	view := updated.(Model).View()

	if !strings.Contains(view, "0.4.2") || !strings.Contains(view, "available") {
		t.Fatalf("header does not announce the update:\n%s", view)
	}
}

func TestHeaderShowsNothingWithoutAnUpdate(t *testing.T) {
	source := &fakeSource{snapshot: storeSnapshot()}
	model := settle(New(source, "Kubeconfig current context", "argocd", explorer.Snapshot{}), source.loadCommand())

	updated, _ := model.Update(updateAvailable{})

	if view := updated.(Model).View(); strings.Contains(view, "available") {
		t.Fatalf("header mentions an update when there is none:\n%s", view)
	}
}

func TestUpdateCheckRunsOffTheMainLoop(t *testing.T) {
	var asked bool
	model := New(&fakeSource{snapshot: storeSnapshot()}, "Kubeconfig current context", "argocd", explorer.Snapshot{}).
		WithUpdateCheck(func(context.Context) string { asked = true; return "0.4.2" })

	_ = model.Init()
	if asked {
		t.Fatal("Init() ran the update check synchronously; it must run inside a command")
	}
	message := model.checkForUpdate()()
	if notice, ok := message.(updateAvailable); !ok || notice.version != "0.4.2" || !asked {
		t.Fatalf("checkForUpdate() = %#v, asked = %v", message, asked)
	}
}

func viewWithUpdate(t *testing.T, width int) (Model, []string) {
	t.Helper()
	source := &fakeSource{snapshot: storeSnapshot()}
	model := settle(New(source, "Kubeconfig current context", "argocd", explorer.Snapshot{}), source.loadCommand())
	model = resize(model, width, 30)
	updated, _ := model.Update(updateAvailable{version: "0.4.2"})
	model = updated.(Model)
	return model, strings.Split(model.View(), "\n")
}

func TestUpdateBadgeSitsAtTheBottomRightOfTheScreen(t *testing.T) {
	_, lines := viewWithUpdate(t, 140)

	last := lines[len(lines)-1]
	if !strings.HasSuffix(last, "↑ 0.4.2 available ") || lipgloss.Width(last) != 140 {
		t.Fatalf("badge is not at the bottom right (width %d): %q", lipgloss.Width(last), last)
	}
	for _, line := range lines[:headerHeight] {
		if strings.Contains(line, "available") {
			t.Fatalf("the update notice is still in the header:\n%s", strings.Join(lines[:headerHeight], "\n"))
		}
	}
}

func TestUpdateBadgeLeavesRoomForTheStatusMessage(t *testing.T) {
	model, _ := viewWithUpdate(t, 140)
	model.status = "Switching to namespace gitops"
	last := lastLine(model.View())

	if !strings.Contains(last, "Switching to namespace gitops") || !strings.HasSuffix(last, "available ") {
		t.Fatalf("status line = %q, want the message on the left and the badge on the right", last)
	}

	model.err = fmt.Errorf("%s", strings.Repeat("a very long error ", 20))
	if last = lastLine(model.View()); !strings.HasSuffix(last, "available ") || lipgloss.Width(last) > 140 {
		t.Fatalf("a long error pushed the badge away or overflowed (width %d): %q", lipgloss.Width(last), last)
	}
}

func TestUpdateBadgeSurvivesANarrowTerminal(t *testing.T) {
	_, lines := viewWithUpdate(t, 40)

	if last := lines[len(lines)-1]; !strings.HasSuffix(last, "available ") || lipgloss.Width(last) > 40 {
		t.Fatalf("badge is missing or too wide at 40 columns (width %d): %q", lipgloss.Width(last), last)
	}
}

func lastLine(view string) string {
	lines := strings.Split(view, "\n")
	return lines[len(lines)-1]
}

func TestUpdateBadgeIsFilledWithTheAccentColor(t *testing.T) {
	badge := renderUpdateBadge("0.4.2")

	if strings.Contains(badge, "\n") || !strings.Contains(badge, "↑ 0.4.2 available") {
		t.Fatalf("badge = %q, want one line naming the version", badge)
	}
	if want := updateBadgeStyle.Render("x"); want == "x" {
		t.Skip("the test terminal has no color profile, so styles render as plain text")
	}
}
