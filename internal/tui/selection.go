package tui

import (
	"context"
	"fmt"
	"slices"
	"strconv"

	"github.com/awcodify/argoxd/internal/argocd"
	"github.com/awcodify/argoxd/internal/explorer"
	tea "github.com/charmbracelet/bubbletea"
)

var errNoResourceSync = fmt.Errorf("the active source cannot sync individual resources")

// markGlyph flags a marked Application row or resource card.
const markGlyph = "●"

// toggleMark marks or unmarks the selected Application, or the selected card
// in the dependency view, and moves on to the next one.
func (m *Model) toggleMark() {
	switch m.view {
	case listView:
		name := m.explorer.SelectedName()
		if m.explorer.Screen() != explorer.ApplicationsScreen || name == "" {
			return
		}
		if m.marked == nil {
			m.marked = make(map[string]bool)
		}
		flip(m.marked, name)
		m.explorer.MoveDown()
	case applicationTreeView:
		card := m.selectedCard()
		if card.Kind == "Application" {
			m.status = "Select a resource card to mark it"
			return
		}
		if m.markedCards == nil {
			m.markedCards = make(map[explorer.ResourceReference]bool)
		}
		flip(m.markedCards, card.Reference())
		m.moveDown()
	default:
		m.toggleTreeItem()
		return
	}
	m.status = ""
	if count := m.markCount(); count > 0 {
		m.status = strconv.Itoa(count) + " marked"
	}
}

func flip[K comparable](set map[K]bool, key K) {
	if set[key] {
		delete(set, key)
	} else {
		set[key] = true
	}
}

// markCount is how many Applications or cards are marked in the active view.
func (m Model) markCount() int {
	if m.view == applicationTreeView {
		return len(m.markedCards)
	}
	return len(m.marked)
}

// clearMarks forgets the marks of the active view.
func (m *Model) clearMarks() {
	if m.view == applicationTreeView {
		m.markedCards = nil
	} else {
		m.marked = nil
	}
	m.status = ""
}

// pruneMarks drops marks of Applications that are gone.
func (m *Model) pruneMarks() {
	for name := range m.marked {
		if !slices.ContainsFunc(m.explorer.Snapshot().Applications, func(a explorer.Application) bool { return a.Name == name }) {
			delete(m.marked, name)
		}
	}
}

// markedNodes returns the marked cards in the order the tree shows them,
// including the ones an active search or filter hides.
func (m Model) markedNodes() []explorer.ResourceNode {
	var marked []explorer.ResourceNode
	var walk func([]explorer.ResourceNode)
	walk = func(nodes []explorer.ResourceNode) {
		for _, node := range nodes {
			if m.markedCards[node.Reference()] {
				marked = append(marked, node)
			}
			walk(node.Children)
		}
	}
	walk(m.resourceTree.Hierarchy())
	return marked
}

// markedApplications returns the marked Applications by name.
func (m Model) markedApplications() []string {
	names := make([]string, 0, len(m.marked))
	for name := range m.marked {
		names = append(names, name)
	}
	slices.Sort(names)
	return names
}

// applicationTargets is what an Application action applies to: the marked
// Applications on the list, or else the selected one.
func (m Model) applicationTargets() []string {
	if m.view == listView && len(m.marked) > 0 {
		return m.markedApplications()
	}
	if application := m.selectedApplication(); application != "" {
		return []string{application}
	}
	return nil
}

// askApplicationAction opens the confirmation for sync (s), hard refresh (R)
// or delete (D). In the dependency view, sync covers only the marked cards.
func (m Model) askApplicationAction(kind confirmKind) Model {
	if kind == confirmSync && m.view == applicationTreeView && len(m.markedCards) > 0 {
		m.confirm = confirmation{kind: confirmSync, application: m.resourceTree.Application, resources: m.markedNodes()}
		return m
	}
	if names := m.applicationTargets(); len(names) > 0 {
		m.confirm = forApplications(kind, names)
	}
	return m
}

// syncResources syncs only the given resources of an Application.
func (m Model) syncResources(application string, nodes []explorer.ResourceNode, options argocd.SyncOptions) tea.Cmd {
	resources := make([]explorer.ResourceReference, len(nodes))
	for index, node := range nodes {
		resources[index] = node.Reference()
	}
	syncer, ok := m.source.(argocd.ResourceSyncer)
	if !ok {
		return func() tea.Msg {
			return operationCompleted{action: syncAction(options), application: application, err: errNoResourceSync}
		}
	}
	return m.request(func(ctx context.Context) tea.Msg {
		return operationCompleted{
			action: syncAction(options), application: application, subject: count(len(resources), "resource") + " of " + application,
			refreshTree: true, err: syncer.SyncResources(ctx, application, resources, options),
		}
	})
}

func syncAction(options argocd.SyncOptions) string {
	if options.DryRun {
		return "dry-run sync"
	}
	return "sync"
}
