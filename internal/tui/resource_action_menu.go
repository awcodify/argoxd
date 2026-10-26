package tui

import (
	"context"
	"fmt"
	"slices"

	"github.com/awcodify/argoxd/internal/argocd"
	"github.com/awcodify/argoxd/internal/explorer"
	tea "github.com/charmbracelet/bubbletea"
)

// actionField is the prompt field that chooses one of a resource's actions,
// like the container and Pod bars.
const actionField = "action"

// loadedActions is the actions Argo CD offers for the card that was selected
// when the menu was asked for.
type loadedActions struct {
	application string
	resource    explorer.ResourceNode
	actions     []argocd.ResourceAction
	err         error
}

// askResourceActions looks up the actions of the selected card. The lookup runs
// in a command so a slow Argo CD never freezes the screen.
func (m Model) askResourceActions() (tea.Model, tea.Cmd) {
	if m.onRootCard() {
		m.status = "Select a resource card to act on it"
		return m, nil
	}
	runner, ok := m.source.(argocd.ResourceActionRunner)
	if !ok {
		m.err = errNoActions
		return m, nil
	}
	application, resource := m.resourceTree.Application, m.selectedCard()
	return m, m.request(func(ctx context.Context) tea.Msg {
		actions, err := runner.ResourceActions(ctx, application, resource)
		return loadedActions{application: application, resource: resource, actions: actions, err: err}
	})
}

// applyActions opens the action bar once the lookup is back, unless the user
// has moved on to another card or view in the meantime.
func (m Model) applyActions(message loadedActions) (tea.Model, tea.Cmd) {
	if m.view != applicationTreeView || m.onRootCard() || m.selectedCard().Reference() != message.resource.Reference() {
		return m, nil
	}
	if message.err != nil {
		m.err = message.err
		return m, nil
	}
	var names []string
	needParameters := false
	for _, action := range message.actions {
		switch {
		case action.Disabled:
		case action.NeedsParameters:
			needParameters = true
		default:
			names = append(names, action.Name)
		}
	}
	switch {
	case len(names) > 0:
	case needParameters:
		m.status = resourceName(message.resource) + " only has actions that need parameters, which argoxd cannot enter yet"
		return m, nil
	default:
		m.status = "No actions for " + resourceName(message.resource)
		return m, nil
	}
	m.actionOptions = names
	m.prompt = prompt{active: true, field: actionField}
	return m, nil
}

// confirmAction asks before running the action chosen in the bar.
func (m Model) confirmAction(choice string) (tea.Model, tea.Cmd) {
	options := m.actionOptions
	m.actionOptions = nil
	if choice == "" {
		return m, nil
	}
	if !slices.Contains(options, choice) {
		m.err = fmt.Errorf("action %q not found", choice)
		return m, nil
	}
	m.confirm = confirmation{
		kind: confirmResourceAction, application: m.resourceTree.Application,
		resources: []explorer.ResourceNode{m.selectedCard()}, action: choice,
	}
	return m, nil
}
