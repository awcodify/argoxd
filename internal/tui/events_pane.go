package tui

import (
	"context"
	"strconv"
	"strings"

	"github.com/awcodify/argoxd/internal/argocd"
	"github.com/awcodify/argoxd/internal/explorer"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

const (
	// maxEventsPaneHeight and minEventsPaneHeight are the pane's height with its
	// border: it takes what the cards can spare, between these two.
	maxEventsPaneHeight = 9
	minEventsPaneHeight = 5
	// minCardsHeight is what the cards must keep, enough for the selected card
	// and one more; the table needs only a header and a few rows.
	minCardsHeight = 8
	minTableHeight = 6
)

// eventsPaneHeight is how tall the pane is when there are free lines for it
// and the cards, or 0 when it would leave the cards too little.
func (m Model) eventsPaneHeight(free int) int {
	needed := minCardsHeight
	if m.resourceTable {
		needed = minTableHeight
	}
	height := min(maxEventsPaneHeight, free-needed)
	if height < minEventsPaneHeight {
		return 0
	}
	return height
}

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
		m.paneText, m.paneErr, m.paneOffset, m.paneLoaded = "", nil, 0, false
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
	paneHeight := m.eventsPaneHeight(height)
	if !m.eventsPane || m.paneFollowing == "" || paneHeight == 0 {
		return nil
	}
	rows := paneHeight - 2
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
		// The newest events are last. Show as many as fit, ending paneOffset
		// events before the newest.
		offset := min(m.paneOffset, max(0, len(events)-(rows-1)))
		end := len(events) - offset
		events = events[max(0, end-(rows-1)):end]
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
		title:  " " + accentStyle.Render("events · "+m.paneSubject()) + m.paneScrollNote(paneHeight-3) + " ",
		width:  max(0, width-2),
		height: paneHeight,
	}.render(content)
	lines := strings.Split(framed, "\n")
	for row := range lines {
		lines[row] = " " + lines[row]
	}
	return lines
}

// paneEventCount is how many events the pane has, not counting its header line.
func (m Model) paneEventCount() int {
	if strings.TrimSpace(m.paneText) == "" {
		return 0
	}
	return len(strings.Split(strings.TrimRight(m.paneText, "\n"), "\n")) - 1
}

// maxPaneOffset is how far the pane can scroll back, given how many events it
// has room for.
func (m Model) maxPaneOffset() int {
	rows := m.currentPaneHeight() - 3
	return max(0, m.paneEventCount()-rows)
}

// currentPaneHeight is the pane's height on the present screen, 0 when it is not drawn.
func (m Model) currentPaneHeight() int {
	return m.eventsPaneHeight(m.bodyHeight() - len(m.renderSummary(m.width-2)))
}

// paneScrollNote says how many events are off screen, so a pane that looks
// complete is not mistaken for one.
func (m Model) paneScrollNote(rows int) string {
	count := m.paneEventCount()
	offset := min(m.paneOffset, max(0, count-rows))
	older, newer := count-rows-offset, offset
	var parts []string
	if older > 0 {
		parts = append(parts, "↑ "+strconv.Itoa(older)+" older")
	}
	if newer > 0 {
		parts = append(parts, "↓ "+strconv.Itoa(newer)+" newer")
	}
	if len(parts) == 0 {
		return ""
	}
	return mutedStyle.Render(" · " + strings.Join(parts, " · ") + " · [ ] scroll")
}

// scrollPane moves the pane back (positive) or forward (negative) through the
// events, within what there is.
func (m *Model) scrollPane(lines int) {
	if !m.eventsPane || m.paneFollowing == "" {
		return
	}
	m.paneOffset = max(0, min(m.maxPaneOffset(), m.paneOffset+lines))
}

// paneRows gives the screen rows the pane covers, counting from the top of the
// terminal, or false when it is not drawn.
func (m Model) paneRows() (top, bottom int, ok bool) {
	if !m.eventsPane || m.paneFollowing == "" {
		return 0, 0, false
	}
	height := m.currentPaneHeight()
	if height == 0 {
		return 0, 0, false
	}
	contentTop := headerHeight + 1
	if m.prompt.active {
		contentTop += promptHeight
	}
	top = contentTop + m.bodyHeight() - len(m.renderSummary(m.width-2)) - height
	return top, top + height - 1, true
}
