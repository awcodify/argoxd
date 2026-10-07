package tui

import (
	"context"
	"errors"
	"fmt"

	"github.com/awcodify/argoxd/internal/argocd"
	"github.com/awcodify/argoxd/internal/explorer"
	tea "github.com/charmbracelet/bubbletea"
)

var errNoActions = errors.New("the active source cannot act on resources")

// askResourceAction opens the confirmation for restarting (x) the marked
// workloads or deleting (X) the marked Pods, or else the selected card.
func (m Model) askResourceAction(key string) (tea.Model, tea.Cmd) {
	kind := confirmRestart
	if key == "X" {
		kind = confirmDeleteResources
	}
	targets, refusal := m.resourceTargets(kind)
	if refusal != "" {
		m.status = refusal
		return m, nil
	}
	if _, ok := m.source.(argocd.ResourceActor); !ok {
		m.err = errNoActions
		return m, nil
	}
	m.confirm = confirmation{kind: kind, application: m.resourceTree.Application, resources: targets}
	return m, nil
}

// resourceTargets returns the cards an action applies to, or why it cannot:
// every marked card must take the action, and none is skipped silently.
func (m Model) resourceTargets(kind confirmKind) ([]explorer.ResourceNode, string) {
	if len(m.markedCards) == 0 {
		selected := m.selectedCard()
		switch {
		case m.onRootCard():
			return nil, "Select a resource card to act on it"
		case kind == confirmRestart && !argocd.Restartable(selected.Kind):
			return nil, "Only Deployments, StatefulSets and DaemonSets can be restarted"
		case kind == confirmDeleteResources && selected.Kind != "Pod":
			return nil, "Only Pods can be deleted here"
		}
		return []explorer.ResourceNode{selected}, ""
	}
	targets := m.markedNodes()
	for _, node := range targets {
		switch {
		case kind == confirmRestart && !argocd.Restartable(node.Kind):
			return nil, fmt.Sprintf("%s cannot be restarted: only Deployments, StatefulSets and DaemonSets can be", resourceName(node))
		case kind == confirmDeleteResources && node.Kind != "Pod":
			return nil, fmt.Sprintf("%s cannot be deleted here: only Pods can be", resourceName(node))
		}
	}
	return targets, ""
}

// actOnResources restarts or deletes the confirmed resources, one after another.
func (m Model) actOnResources(c confirmation) tea.Cmd {
	action := "restart"
	if c.kind == confirmDeleteResources {
		action = "delete"
	}
	actor, ok := m.source.(argocd.ResourceActor)
	if !ok {
		return func() tea.Msg {
			return operationCompleted{action: action, application: c.application, err: errNoActions}
		}
	}
	act := func(ctx context.Context, resource explorer.ResourceNode) error {
		if action == "restart" {
			return actor.RestartResource(ctx, c.application, resource)
		}
		return actor.DeleteResource(ctx, c.application, resource)
	}
	if len(c.resources) == 1 {
		return m.request(func(ctx context.Context) tea.Msg {
			return operationCompleted{
				action: action, application: c.application, subject: c.subject(), refreshTree: true, err: act(ctx, c.resources[0]),
			}
		})
	}
	labels := make([]string, len(c.resources))
	for index, resource := range c.resources {
		labels[index] = resourceName(resource)
	}
	return m.each(action, c.subject(), c.application, labels, true, func(ctx context.Context, index int) error {
		return act(ctx, c.resources[index])
	})
}
