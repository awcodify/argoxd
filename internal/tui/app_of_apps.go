package tui

import (
	"github.com/awcodify/argoxd/internal/argocd"
	"github.com/awcodify/argoxd/internal/explorer"
	tea "github.com/charmbracelet/bubbletea"
)

// treeFrame is the dependency view of an Application that is left open while
// one of the Applications it deploys is shown.
type treeFrame struct {
	tree   explorer.ResourceTree
	cursor int
	search string
	filter explorer.Filter
	marked map[explorer.ResourceReference]bool
}

// onRootCard reports whether the selected card is the open Application's own
// card, which comes first. A card of the same kind further down is a child.
func (m Model) onRootCard() bool {
	return m.treeCursor == 0
}

// onChildApplication reports whether the selected card is an Application that
// the open Application deploys, as an app of apps does.
func (m Model) onChildApplication() bool {
	if m.view != applicationTreeView || m.onRootCard() {
		return false
	}
	card := m.selectedCard()
	return card.Kind == "Application" && card.Group == "argoproj.io"
}

// openChildApplication shows the dependencies of the selected child Application.
// Esc comes back to the one it was opened from.
func (m Model) openChildApplication() (tea.Model, tea.Cmd) {
	if !m.onChildApplication() {
		return m, nil
	}
	if _, ok := m.source.(argocd.ApplicationOperator); !ok {
		m.err = errNoOperations
		return m, nil
	}
	m.loading = true
	load := m.loadApplicationTree(m.selectedCard().Name, false)
	return m, func() tea.Msg {
		message := load()
		if tree, ok := message.(loadedTree); ok {
			tree.child = true
			return tree
		}
		return message
	}
}

// enterChild remembers the open Application and starts the child with no
// search, filter or marks.
func (m *Model) enterChild() {
	m.parents = append(m.parents, treeFrame{
		tree: m.resourceTree, cursor: m.treeCursor, search: m.treeSearch, filter: m.treeFilter, marked: m.markedCards,
	})
	m.treeSearch, m.treeFilter, m.markedCards = "", explorer.Filter{}, nil
}

// leaveChild goes back to the Application the child was opened from.
func (m *Model) leaveChild() {
	frame := m.parents[len(m.parents)-1]
	m.parents = m.parents[:len(m.parents)-1]
	m.resourceTree, m.treeCursor = frame.tree, frame.cursor
	m.treeSearch, m.treeFilter, m.markedCards = frame.search, frame.filter, frame.marked
	m.status = ""
}

// treePath names the Applications from the first one opened to the one shown.
func (m Model) treePath() []string {
	path := make([]string, 0, len(m.parents)+1)
	for _, frame := range m.parents {
		path = append(path, frame.tree.Application)
	}
	return append(path, m.resourceTree.Application)
}
