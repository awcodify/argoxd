package tui

import (
	"context"
	"errors"

	"github.com/awcodify/argoxd/internal/argocd"
	"github.com/awcodify/argoxd/internal/explorer"
	tea "github.com/charmbracelet/bubbletea"
)

var errNoEvents = errors.New("the active source cannot list events")

// showEvents opens the events of the selected Application on the list, or of
// the selected card in the dependency view; the Application's own card shows
// the Application's events.
func (m Model) showEvents() (tea.Model, tea.Cmd) {
	var application string
	var resource explorer.ResourceNode
	fromList := false
	switch m.view {
	case listView:
		if !m.onApplicationRows() || m.explorer.SelectedName() == "" {
			return m, nil
		}
		application, fromList = m.explorer.SelectedName(), true
	case applicationTreeView:
		application, resource = m.resourceTree.Application, m.selectedCard()
	default:
		return m, nil
	}
	lister, ok := m.source.(argocd.EventLister)
	if !ok {
		m.err = errNoEvents
		return m, nil
	}

	subject := application
	if resource.Kind != "" && resource.Kind != "Application" {
		subject = resource.Kind + "/" + resource.Name
	}
	m.loading = true
	return m, m.request(func(ctx context.Context) tea.Msg {
		var text string
		var err error
		if subject == application {
			text, err = lister.ApplicationEvents(ctx, application)
		} else {
			text, err = lister.ResourceEvents(ctx, application, resource)
		}
		return loadedText{kind: "events", subject: subject, text: text, fromList: fromList, err: err}
	})
}
