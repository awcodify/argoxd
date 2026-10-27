package tui

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"

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

func TestUpdateBadgeSharesTheRowOfTheBreadcrumbs(t *testing.T) {
	_, lines := viewWithUpdate(t, 140)

	row := lines[len(lines)-2]
	if !strings.Contains(row, "applications") || !strings.HasSuffix(row, "↑ 0.4.2 available ") || lipgloss.Width(row) != 140 {
		t.Fatalf("the breadcrumb row is not %q ending in the badge (width %d): %q", "applications", lipgloss.Width(row), row)
	}
	if strings.Contains(lines[len(lines)-1], "available") {
		t.Fatalf("the status line carries the badge: %q", lines[len(lines)-1])
	}
	for _, line := range lines[:headerHeight] {
		if strings.Contains(line, "available") {
			t.Fatalf("the update notice is still in the header:\n%s", strings.Join(lines[:headerHeight], "\n"))
		}
	}
}

func TestUpdateBadgeDoesNotTouchTheStatusLine(t *testing.T) {
	model, _ := viewWithUpdate(t, 140)
	model.status = "Switching to namespace gitops"

	if last := lastLine(model.View()); last != " "+infoStyle.Render("● Switching to namespace gitops") {
		t.Fatalf("status line = %q, want the plain status message", last)
	}
}

func TestUpdateBadgeSurvivesALongBreadcrumbPathAndANarrowTerminal(t *testing.T) {
	_, lines := viewWithUpdate(t, 30)

	if row := lines[len(lines)-2]; !strings.HasSuffix(row, "available ") || lipgloss.Width(row) > 30 {
		t.Fatalf("badge is missing or too wide at 30 columns (width %d): %q", lipgloss.Width(row), row)
	}
}

func lastLine(view string) string {
	lines := strings.Split(view, "\n")
	return lines[len(lines)-1]
}

func TestUpdateBadgeIsFilledWithTheAccentColor(t *testing.T) {
	useTheme(t, "default")
	badge := renderUpdateBadge("0.4.2")

	if strings.Contains(badge, "\n") || !strings.Contains(badge, "↑ 0.4.2 available") {
		t.Fatalf("badge = %q, want one line naming the version", badge)
	}
	// useTheme forces the true-color profile, so a background is drawn as
	// "48;2;r;g;b". The badge must carry the accent's, not just its text.
	accent, _ := lookupTheme("default")
	value, err := strconv.ParseUint(strings.TrimPrefix(accent.accent, "#"), 16, 32)
	if err != nil {
		t.Fatalf("accent %q: %v", accent.accent, err)
	}
	want := fmt.Sprintf("48;2;%d;%d;%d", value>>16, (value>>8)&0xff, value&0xff)
	if !strings.Contains(badge, want) {
		t.Fatalf("badge = %q, want the accent %s as its background (%s)", badge, accent.accent, want)
	}
}

func TestWithRequestTimeoutSetsHowLongSourceCallsMayTake(t *testing.T) {
	model := New(&fakeSource{}, "test", "argocd", explorer.Snapshot{})
	if model.requestTimeout != defaultRequestTimeout {
		t.Fatalf("requestTimeout = %v, want the default", model.requestTimeout)
	}

	if model = model.WithRequestTimeout(45 * time.Second); model.requestTimeout != 45*time.Second {
		t.Fatalf("requestTimeout = %v, want 45s", model.requestTimeout)
	}
	if model = model.WithRequestTimeout(0); model.requestTimeout != 45*time.Second {
		t.Fatalf("a zero timeout replaced the setting: %v", model.requestTimeout)
	}
}

func TestALongErrorIsCutToTheScreenWidthWithAnEllipsis(t *testing.T) {
	model, _ := sortModel()
	model.err = fmt.Errorf("%s", strings.Repeat("a long failure message ", 20))

	for _, width := range []int{140, 80, 40} {
		line := lastLine(resize(model, width, 30).View())
		if lipgloss.Width(line) > width || !strings.HasSuffix(strings.TrimRight(line, " "), "…") {
			t.Errorf("width %d: the status line is %d wide and ends %q; want it cut with …", width, lipgloss.Width(line), line[max(0, len(line)-12):])
		}
	}
	short := resize(model, 140, 30)
	short.err = fmt.Errorf("short")
	if line := lastLine(short.View()); strings.Contains(line, "…") {
		t.Errorf("a short error was cut: %q", line)
	}
}
