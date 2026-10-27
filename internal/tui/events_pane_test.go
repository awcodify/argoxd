package tui

import (
	"strings"
	"testing"

	"github.com/awcodify/argoxd/internal/explorer"
)

func paneModel(t *testing.T) (Model, *fakeSource) {
	t.Helper()
	source := &fakeSource{snapshot: storeSnapshot(), tree: checkoutTree(), events: sampleEvents}
	return resize(openCheckout(t, source), 140, 44), source
}

// open runs the commands a key press returns, as Bubble Tea would.
func keyed(model Model, keys ...string) Model {
	for _, k := range keys {
		model = run(model, k)
	}
	return model
}

func TestEOpensAPaneOfTheSelectedCardsEventsUnderTheCards(t *testing.T) {
	model, source := paneModel(t)
	model = keyed(model, "j") // the Deployment

	if strings.Contains(model.View(), "Back-off restarting") {
		t.Fatal("the events show before the pane was opened")
	}
	model = keyed(model, "E")

	view := model.View()
	if !strings.Contains(view, "events · Deployment/web") || !strings.Contains(view, "Back-off restarting failed container") {
		t.Fatalf("the pane does not show the Deployment's events:\n%s", view)
	}
	if got := strings.Join(source.eventRequests, ","); !strings.Contains(got, "Deployment/web") {
		t.Fatalf("requests = %q, want one for Deployment/web", got)
	}
	if !strings.Contains(view, "┏━ Deployment") && !strings.Contains(view, "╭─ Deployment") {
		t.Fatalf("the cards disappeared when the pane opened:\n%s", view)
	}
}

func TestThePaneFollowsTheSelection(t *testing.T) {
	model, source := paneModel(t)
	model = keyed(model, "E")
	if !strings.Contains(model.View(), "events · checkout") {
		t.Fatalf("on the root card the pane should show the Application's events:\n%s", model.View())
	}

	model = keyed(model, "j")
	if view := model.View(); !strings.Contains(view, "events · Deployment/web") {
		t.Fatalf("the pane did not follow the cursor to the Deployment:\n%s", view)
	}
	model = keyed(model, "j")
	if view := model.View(); !strings.Contains(view, "events · Pod/web-abc") {
		t.Fatalf("the pane did not follow the cursor to the Pod:\n%s", view)
	}
	if len(source.eventRequests) != 3 {
		t.Fatalf("requests = %v, want exactly one per selection", source.eventRequests)
	}
}

func TestAStaleAnswerNeverReplacesTheNewSelectionsEvents(t *testing.T) {
	model, _ := paneModel(t)
	model = keyed(model, "E", "j")

	updated, _ := model.Update(paneEventsLoaded{target: "checkout/Application/checkout", text: "OLD\nstale events"})

	if view := updated.(Model).View(); strings.Contains(view, "stale events") {
		t.Fatalf("an answer for the previous card was shown:\n%s", view)
	}
}

func TestEAgainClosesThePaneAndStopsRequestingEvents(t *testing.T) {
	model, source := paneModel(t)
	model = keyed(model, "E", "E")
	before := len(source.eventRequests)

	model = keyed(model, "j")

	if len(source.eventRequests) != before || strings.Contains(model.View(), "events ·") {
		t.Fatalf("a closed pane still requested events (%d → %d) or drew:\n%s", before, len(source.eventRequests), model.View())
	}
}

func TestThePaneStaysOnWhenYouMoveToAnotherApplicationView(t *testing.T) {
	model, _ := paneModel(t)
	model = keyed(model, "E", "j", "e") // the full events view

	if view := model.View(); !strings.Contains(view, "events") {
		t.Fatalf("the full events view did not open:\n%s", view)
	}
	back := keyed(model, "esc")
	if view := back.View(); !strings.Contains(view, "events · Deployment/web") {
		t.Fatalf("the pane was lost after leaving the events view:\n%s", view)
	}
}

func TestThePaneReloadsOnTheRefreshTickWithoutBlinking(t *testing.T) {
	model, source := paneModel(t)
	model = keyed(model, "E")
	before := len(source.eventRequests)

	updated, command := model.Update(refreshTick{})
	if view := updated.(Model).View(); !strings.Contains(view, "Back-off restarting") {
		t.Fatalf("the events vanished while reloading:\n%s", view)
	}
	settled := settle(updated.(Model), command)

	if len(source.eventRequests) <= before || !strings.Contains(settled.View(), "Back-off restarting") {
		t.Fatalf("requests %d → %d; the tick did not reload the pane", before, len(source.eventRequests))
	}
}

func TestThePaneIsLeftOutWhenThereIsNoRoomForIt(t *testing.T) {
	model, _ := paneModel(t)
	model = resize(keyed(model, "E"), 140, 22)

	view := model.View()

	if strings.Contains(view, "events ·") || !strings.Contains(view, "summary") {
		t.Fatalf("a short terminal should keep the cards and drop the pane:\n%s", view)
	}
}

func TestAnEmptyOrFailedLookupSaysSo(t *testing.T) {
	model, source := paneModel(t)
	source.events = ""
	model = keyed(model, "E")
	if view := model.View(); !strings.Contains(view, "No events") {
		t.Fatalf("an empty result should say so:\n%s", view)
	}

	plain := &fakeSource{snapshot: storeSnapshot(), tree: checkoutTree()}
	noEvents := resize(New(noEventsSource{plain}, "test", "argocd", explorer.Snapshot{}), 140, 44)
	noEvents = settle(noEvents, plain.loadCommand())
	noEvents = keyed(press(noEvents, "j"), "enter")
	if view := keyed(noEvents, "E").View(); !strings.Contains(view, "cannot list events") {
		t.Fatalf("a source without events should say so:\n%s", view)
	}
}

// noEventsSource hides the events methods of the fake source.
type noEventsSource struct{ *fakeSource }

func (noEventsSource) ApplicationEvents() {}
func (noEventsSource) ResourceEvents()    {}

func TestThePaneCanStartOpen(t *testing.T) {
	source := &fakeSource{snapshot: storeSnapshot(), tree: checkoutTree(), events: sampleEvents}
	model := resize(New(source, "test", "argocd", explorer.Snapshot{}).WithEventsPane(true), 140, 44)
	model = settle(model, source.loadCommand())

	model = keyed(press(model, "j"), "enter")

	if view := model.View(); !strings.Contains(view, "events · checkout") {
		t.Fatalf("the pane did not open by itself:\n%s", view)
	}
}

func TestThePaneWorksInTheTableViewToo(t *testing.T) {
	model, _ := paneModel(t)
	model = keyed(model, "w", "E", "j")

	if view := model.View(); !strings.Contains(view, "KIND") || !strings.Contains(view, "events · Deployment/web") {
		t.Fatalf("the table view lost the pane:\n%s", view)
	}
}

func TestTheHintsOfferTheEventsPaneAndStillFitFiveRows(t *testing.T) {
	model, _ := paneModel(t)

	header := strings.Join(strings.Split(model.View(), "\n")[:headerHeight], "\n")

	for _, want := range []string{"Events pane", "Back", "Command", "Quit", "Fold", "Table"} {
		if !strings.Contains(header, want) {
			t.Errorf("the header lost %q:\n%s", want, header)
		}
	}
	for _, hint := range model.keyHints() {
		if hint.key == "" && hint.action != "" {
			t.Errorf("a blank slot with an action: %+v", hint)
		}
	}
}
