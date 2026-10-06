package tui

import (
	"context"
	"errors"

	"github.com/awcodify/argoxd/internal/argocd"
	"github.com/awcodify/argoxd/internal/explorer"
	tea "github.com/charmbracelet/bubbletea"
)

var errNoActions = errors.New("the active source cannot act on resources")

// resourceActionDialog asks for confirmation before acting on a card's resource.
type resourceActionDialog struct {
	// action is "restart" or "delete".
	action      string
	application string
	resource    explorer.ResourceNode
}

func (d resourceActionDialog) subject() string {
	return d.resource.Kind + "/" + d.resource.Name
}

// askResourceAction opens the confirmation for restarting (x) the selected
// workload or deleting (X) the selected Pod.
func (m Model) askResourceAction(key string) (tea.Model, tea.Cmd) {
	selected := m.selectedCard()
	action := "restart"
	if key == "X" {
		action = "delete"
	}
	switch {
	case selected.Kind == "Application":
		m.status = "Select a resource card to act on it"
		return m, nil
	case action == "restart" && !argocd.Restartable(selected.Kind):
		m.status = "Only Deployments, StatefulSets and DaemonSets can be restarted"
		return m, nil
	case action == "delete" && selected.Kind != "Pod":
		m.status = "Only Pods can be deleted here"
		return m, nil
	}
	if _, ok := m.source.(argocd.ResourceActor); !ok {
		m.err = errNoActions
		return m, nil
	}
	m.resourceAction = resourceActionDialog{action: action, application: m.resourceTree.Application, resource: selected}
	return m, nil
}

func (m Model) updateResourceAction(message tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch message.String() {
	case "y":
		dialog := m.resourceAction
		m.resourceAction = resourceActionDialog{}
		m.loading = true
		return m, m.actOnResource(dialog)
	case "n", "esc":
		m.resourceAction = resourceActionDialog{}
	}
	return m, nil
}

func (m Model) actOnResource(dialog resourceActionDialog) tea.Cmd {
	actor, ok := m.source.(argocd.ResourceActor)
	if !ok {
		return func() tea.Msg {
			return operationCompleted{action: dialog.action, application: dialog.application, err: errNoActions}
		}
	}
	return m.request(func(ctx context.Context) tea.Msg {
		var err error
		switch dialog.action {
		case "restart":
			err = actor.RestartResource(ctx, dialog.application, dialog.resource)
		case "delete":
			err = actor.DeleteResource(ctx, dialog.application, dialog.resource)
		}
		return operationCompleted{
			action: dialog.action, application: dialog.application, subject: dialog.subject(), refreshTree: true, err: err,
		}
	})
}
