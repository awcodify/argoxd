package tui

import (
	"regexp"
	"strings"
	"testing"

	"github.com/awcodify/argoxd/internal/explorer"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

func cells(bar string) (filled, empty int) {
	plain := ansi.Strip(bar)
	return strings.Count(plain, "█"), strings.Count(plain, "░")
}

func segmentWidths(counts map[string]int, order []string, width int) map[string]int {
	widths := map[string]int{}
	for _, segment := range barSegments(counts, order, width) {
		widths[segment.status] = segment.cells
	}
	return widths
}

func TestBarSegmentsShareTheWidthInProportionToTheCounts(t *testing.T) {
	got := segmentWidths(map[string]int{"Healthy": 3, "Degraded": 1}, []string{"Healthy", "Degraded"}, 8)

	if got["Healthy"] != 6 || got["Degraded"] != 2 {
		t.Fatalf("segments = %v, want Healthy 6 and Degraded 2", got)
	}
	if got := segmentWidths(map[string]int{"Healthy": 3}, []string{"Healthy"}, 8); got["Healthy"] != 8 {
		t.Fatalf("a single status should fill the bar, got %v", got)
	}
}

func TestBarSegmentsGiveEveryStatusAtLeastOneCellAndFillTheWidth(t *testing.T) {
	for _, width := range []int{3, 10, 17, 40} {
		got := segmentWidths(map[string]int{"Healthy": 200, "Degraded": 1, "Missing": 1}, []string{"Healthy", "Degraded", "Missing"}, width)

		total := 0
		for status, cells := range got {
			if cells < 1 {
				t.Fatalf("width %d: %s has %d cells, want at least 1 (%v)", width, status, cells, got)
			}
			total += cells
		}
		if total != width || len(got) != 3 {
			t.Fatalf("width %d: segments %v fill %d cells, want %d", width, got, total, width)
		}
	}
}

func TestBarSegmentsIncludeStatusesOutsideTheUsualOrder(t *testing.T) {
	got := segmentWidths(map[string]int{"Healthy": 1, "Hibernating": 1}, []string{"Healthy"}, 6)

	if got["Healthy"] != 3 || got["Hibernating"] != 3 {
		t.Fatalf("segments = %v, want 3 and 3", got)
	}
}

func TestStackedBarDrawsOneBlockPerCell(t *testing.T) {
	if filled, empty := cells(stackedBar(map[string]int{"Healthy": 3, "Degraded": 1}, []string{"Healthy", "Degraded"}, 8)); filled != 8 || empty != 0 {
		t.Fatalf("bar has %d filled and %d empty cells, want 8 and 0", filled, empty)
	}
}

func TestStackedBarOfNothingIsAnEmptyTrack(t *testing.T) {
	if filled, empty := cells(stackedBar(nil, []string{"Healthy"}, 6)); filled != 0 || empty != 6 {
		t.Fatalf("empty bar has %d filled and %d empty cells, want 0 and 6", filled, empty)
	}
}

func TestProgressBarFillsInProportion(t *testing.T) {
	for _, test := range []struct {
		done, total, filled int
	}{{0, 8, 0}, {4, 8, 5}, {8, 8, 10}, {1, 100, 1}, {0, 0, 0}} {
		bar := progressBar(test.done, test.total, 10)
		if filled, empty := cells(bar); filled != test.filled || filled+empty != 10 {
			t.Fatalf("progressBar(%d, %d, 10) has %d filled and %d empty cells, want %d filled of 10", test.done, test.total, filled, empty, test.filled)
		}
	}
}

func pulseSnapshot() explorer.Snapshot {
	snapshot := storeSnapshot() // grafana is Synced and Healthy; checkout is OutOfSync and Degraded
	snapshot.Applications[1].Operation = &explorer.Operation{Phase: "Failed", Message: "boom"}
	snapshot.Applications[1].Conditions = []explorer.Condition{{Type: "SyncError", Message: "boom"}}
	snapshot.Applications[0].Policy = explorer.SyncPolicy{Automated: true}
	snapshot.ApplicationSets = []explorer.ApplicationSet{
		{Name: "a", Generators: []string{"git"}},
		{Name: "b", Generators: []string{"git"}, Problems: []explorer.Condition{{Type: "ErrorOccurred", Message: "x"}}},
	}
	return snapshot
}

func TestPulseCommandShowsTheOverviewInPanels(t *testing.T) {
	for _, name := range []string{":pulse", ":overview"} {
		model := resize(New(nil, "test", "argocd", pulseSnapshot()), 140, 30)

		view := command(model, name).View()

		for _, want := range []string{
			"pulse · 1", "╭─ applications", "╭─ health", "╭─ sync", "╭─ policy",
			"2 applications", "2 application sets", "1 with problems",
			"Healthy", "Degraded", "Synced", "Out of sync", "1 of 2",
		} {
			if !strings.Contains(view, want) {
				t.Fatalf("%s does not show %q:\n%s", name, want, view)
			}
		}
	}
}

func TestPulseLegendsPairEachStatusWithItsCount(t *testing.T) {
	model := resize(New(nil, "test", "argocd", pulseSnapshot()), 140, 30)

	view := ansi.Strip(command(model, ":pulse").View())

	for _, status := range []string{"Healthy", "Degraded", "Synced", "Out of sync"} {
		if !regexp.MustCompile(status + `\s+1\s`).MatchString(view) {
			t.Fatalf("no legend entry pairs %s with its count of 1:\n%s", status, view)
		}
	}
}

func TestPulsePanelsDrawBarsAndStayWithinTheScreen(t *testing.T) {
	model := resize(New(nil, "test", "argocd", pulseSnapshot()), 140, 30)

	view := command(model, ":pulse").View()

	if !strings.Contains(view, "█") || !strings.Contains(view, "░") {
		t.Fatalf("the panels draw no bars:\n%s", view)
	}
	for index, line := range strings.Split(view, "\n") {
		if width := lipgloss.Width(line); width > 140 {
			t.Fatalf("line %d is %d columns wide on a 140 column screen: %q", index, width, line)
		}
	}
}

func TestPulseFallsBackToTextOnANarrowScreen(t *testing.T) {
	model := resize(New(nil, "test", "argocd", pulseSnapshot()), 80, 30)

	view := command(model, ":pulse").View()

	for _, want := range []string{"2 applications", "2 application sets", "1 with problems", "1 Healthy", "1 Degraded", "1 Synced", "1 out of sync", "auto-sync on 1 of 2", "checkout"} {
		if !strings.Contains(view, want) {
			t.Fatalf("the text summary does not show %q:\n%s", want, view)
		}
	}
	if strings.Contains(view, "╭─ health") {
		t.Fatalf("a narrow screen should not draw panels:\n%s", view)
	}
}

func TestPulseFallsBackToTextOnAShortScreen(t *testing.T) {
	model := resize(New(nil, "test", "argocd", pulseSnapshot()), 140, 18)

	view := command(model, ":pulse").View()

	if strings.Contains(view, "╭─ health") || !strings.Contains(view, "checkout") {
		t.Fatalf("a short screen should keep the list and drop the panels:\n%s", view)
	}
}

func TestPulseListsApplicationsNeedingAttentionWithTheirReasons(t *testing.T) {
	model := resize(New(nil, "test", "argocd", pulseSnapshot()), 140, 30)

	view := command(model, ":pulse").View()

	for _, want := range []string{"checkout", "Degraded", "sync Failed", "1 condition"} {
		if !strings.Contains(view, want) {
			t.Fatalf("the list does not show %q:\n%s", want, view)
		}
	}
	if strings.Contains(view, "grafana") {
		t.Fatalf("grafana is fine but is listed:\n%s", view)
	}
}

func TestPulseSaysWhenEverythingIsFine(t *testing.T) {
	snapshot := explorer.Snapshot{Applications: []explorer.Application{{Name: "grafana", Sync: "Synced", Health: "Healthy"}}}
	model := resize(New(nil, "test", "argocd", snapshot), 140, 30)

	view := command(model, ":pulse").View()

	if !strings.Contains(view, "Everything looks fine") || strings.Contains(view, "Nothing to show yet.") {
		t.Fatalf("an overview with nothing wrong should say so:\n%s", view)
	}
}

func TestEnterOnAPulseRowOpensTheApplicationAndEscComesBack(t *testing.T) {
	source := &fakeSource{snapshot: pulseSnapshot(), tree: checkoutTree()}
	model := resize(New(source, "test", "argocd", explorer.Snapshot{}), 140, 40)
	model = command(settle(model, source.loadCommand()), ":pulse")

	opened := run(model, "enter")

	if view := opened.View(); !strings.Contains(view, "pulse › checkout") || !strings.Contains(view, "Deployment") {
		t.Fatalf("enter did not open checkout from the overview:\n%s", view)
	}
	if back := press(opened, "esc").View(); !strings.Contains(back, "pulse · 1") {
		t.Fatalf("esc did not return to the overview:\n%s", back)
	}
}

func TestIOnAPulseRowShowsWhyTheSyncFailed(t *testing.T) {
	model := resize(New(nil, "test", "argocd", pulseSnapshot()), 140, 30)

	view := press(command(model, ":pulse"), "i").View()

	if !strings.Contains(view, "Failed") || !strings.Contains(view, "boom") {
		t.Fatalf("i did not show the failed sync of checkout:\n%s", view)
	}
}

func TestSyncFromThePulseIgnoresApplicationsMarkedOnTheList(t *testing.T) {
	source := &fakeSource{snapshot: pulseSnapshot()}
	model := typeKeys(openApplications(t, source), " ") // marks grafana, which the overview does not list
	model = command(model, ":pulse")

	view := press(model, "s").View()

	if !strings.Contains(view, "Sync checkout?") {
		t.Fatalf("sync should ask about the selected row, not the marks on the list:\n%s", view)
	}
}
