package tui

import (
	"context"
	"strings"

	"github.com/awcodify/argoxd/internal/argocd"
	"github.com/awcodify/argoxd/internal/explorer"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

const (
	// eventsPaneHeight is the pane's height with its border.
	eventsPaneHeight = 9
	// minCardsHeight is what the cards need before the pane may take room.
	minCardsHeight = 12
)

// paneEventsLoaded is the events of the card the pane was following.
type paneEventsLoaded struct {
	target string
	text   string
	err    error
}

// WithEventsPane opens the events pane whenever an Application's resources
// are shown. E opens and closes it either way.
func (m Model) WithEventsPane(open bool) Model {
	m.eventsPane = open
	return m
}

// paneTarget names what the pane is following: the Application and the
// selected card, or the Application alone on its own card.
func (m Model) paneTarget() string {
	if m.view != applicationTreeView || m.resourceTree.Application == "" {
		return ""
	}
	target := m.resourceTree.Application
	if !m.onRootCard() {
		card := m.selectedCard()
		target += "/" + card.Kind + "/" + card.Name
	} else {
		target += "/Application/" + m.resourceTree.Application
	}
	return target
}

// paneSubject is the title of the pane's events.
func (m Model) paneSubject() string {
	if m.onRootCard() {
		return m.resourceTree.Application
	}
	card := m.selectedCard()
	return card.Kind + "/" + card.Name
}

// syncEventsPane starts loading the events of the selected card when the
// selection moved, and reloads them on each refresh tick. The old events stay
// on screen until the new ones arrive.
func (m Model) syncEventsPane(message tea.Msg) (Model, tea.Cmd) {
	target := m.paneTarget()
	if !m.eventsPane || target == "" {
		m.paneFollowing = ""
		return m, nil
	}
	_, tick := message.(refreshTick)
	if target == m.paneFollowing && !tick {
		return m, nil
	}
	lister, ok := m.source.(argocd.EventLister)
	if !ok {
		m.paneFollowing, m.paneText, m.paneErr = target, "", errNoEvents
		return m, nil
	}
	if target != m.paneFollowing {
		m.paneText, m.paneErr = "", nil
	}
	m.paneFollowing = target
	application := m.resourceTree.Application
	onRoot := m.onRootCard()
	var resource explorer.ResourceNode
	if !onRoot {
		resource = m.selectedCard()
	}
	return m, m.request(func(ctx context.Context) tea.Msg {
		var text string
		var err error
		if onRoot {
			text, err = lister.ApplicationEvents(ctx, application)
		} else {
			text, err = lister.ResourceEvents(ctx, application, resource)
		}
		return paneEventsLoaded{target: target, text: text, err: err}
	})
}

// showPaneEvents keeps the answer if it is for what the pane is following now.
func (m Model) showPaneEvents(message paneEventsLoaded) Model {
	if message.target != m.paneFollowing {
		return m
	}
	m.paneText, m.paneErr, m.paneLoaded = message.text, message.err, true
	return m
}

// renderEventsPane frames the newest events of the selected card, or nothing
// when the pane is off or the terminal is too short to share.
func (m Model) renderEventsPane(width, height int) []string {
	if !m.eventsPane || m.paneFollowing == "" || height-eventsPaneHeight < minCardsHeight {
		return nil
	}
	rows := eventsPaneHeight - 2
	var content []string
	switch {
	case m.paneErr != nil:
		content = []string{" " + errorStyle.Render(m.paneErr.Error())}
	case !m.paneLoaded && m.paneText == "":
		content = []string{" " + mutedStyle.Render("Loading events…")}
	case strings.TrimSpace(m.paneText) == "":
		content = []string{" " + mutedStyle.Render("No events")}
	default:
		lines := strings.Split(strings.TrimRight(m.paneText, "\n"), "\n")
		header, events := lines[0], lines[1:]
		content = []string{" " + columnStyle.Render(ansi.Truncate(header, width-4, "…"))}
		// The newest events are last; show as many as fit, newest at the bottom.
		if len(events) > rows-1 {
			events = events[len(events)-(rows-1):]
		}
		for _, line := range events {
			style := textStyle
			if strings.Contains(line, "Warning") {
				style = warningStyle
			}
			content = append(content, " "+style.Render(ansi.Truncate(line, width-4, "…")))
		}
	}
	framed := box{
		border: lipgloss.RoundedBorder(),
		color:  colorBorder,
		title:  " " + accentStyle.Render("events · "+m.paneSubject()) + " ",
		width:  max(0, width-2),
		height: eventsPaneHeight,
	}.render(content)
	lines := strings.Split(framed, "\n")
	for row := range lines {
		lines[row] = " " + lines[row]
	}
	return lines
}
