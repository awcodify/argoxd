package tui

import (
	"slices"
	"strconv"
	"strings"

	"github.com/awcodify/argoxd/internal/explorer"
	"github.com/charmbracelet/x/ansi"
)

// WithResourceView chooses how an Application's resources are first shown:
// "table", or anything else for cards. w switches between them.
func (m Model) WithResourceView(view string) Model {
	m.resourceTable = view == "table"
	return m
}

// collapsedRoot is the Application's card with its resources nested beneath
// it, minus what the user folded away. hidden counts, for each folded card,
// the resources under it that are not shown. A search or filter shows every
// match, so folding is ignored while one is active: otherwise a match under a
// folded card would be invisible.
func (m Model) collapsedRoot() (root explorer.ResourceNode, hidden map[explorer.ResourceReference]int) {
	root = m.filteredRoot()
	if len(m.collapsed) == 0 || m.searching() {
		return root, nil
	}
	hidden = make(map[explorer.ResourceReference]int)
	var fold func(node explorer.ResourceNode, isRoot bool) explorer.ResourceNode
	fold = func(node explorer.ResourceNode, isRoot bool) explorer.ResourceNode {
		if !isRoot && m.collapsed[node.Reference()] && len(node.Children) > 0 {
			hidden[node.Reference()] = len(flatten(node, "")) - 1
			node.Children = nil
			return node
		}
		children := make([]explorer.ResourceNode, len(node.Children))
		for index, child := range node.Children {
			children[index] = fold(child, false)
		}
		node.Children = children
		return node
	}
	return fold(root, true), hidden
}

// collapseSelected folds the selected card's resources away, or, when it has
// none showing, moves to its owner.
func (m *Model) collapseSelected() {
	if m.onRootCard() {
		return
	}
	cards := flatten(m.dependencyRoot(), "")
	selected := cards[m.treeCursor]
	if len(selected.node.Children) > 0 {
		if m.collapsed == nil {
			m.collapsed = make(map[explorer.ResourceReference]bool)
		}
		m.collapsed[selected.node.Reference()] = true
		return
	}
	for index := m.treeCursor - 1; index >= 0; index-- {
		if cards[index].depth < selected.depth {
			m.treeCursor = index
			return
		}
	}
}

// expandSelected shows the resources a folded card hides.
func (m *Model) expandSelected() {
	if m.onRootCard() {
		return
	}
	delete(m.collapsed, m.selectedCard().Reference())
}

// toggleFoldAll folds every card that has resources under it, or, when all are
// folded already, shows everything again. The cursor stays on the selected
// card, or on the nearest card that is still showing.
func (m *Model) toggleFoldAll() {
	full := flatten(m.filteredRoot(), "")
	foldable := make(map[explorer.ResourceReference]bool)
	for _, candidate := range full[1:] {
		if len(candidate.node.Children) > 0 {
			foldable[candidate.node.Reference()] = true
		}
	}
	if len(foldable) == 0 {
		return
	}
	// The card selected before folding, read from the cards showing now.
	showing := flatten(m.dependencyRoot(), "")
	selected := showing[max(0, min(m.treeCursor, len(showing)-1))]
	allFolded := true
	for reference := range foldable {
		if !m.collapsed[reference] {
			allFolded = false
			break
		}
	}
	if allFolded {
		m.collapsed = nil
	} else {
		m.collapsed = foldable
	}
	m.relocate(selected, full)
}

// relocate puts the cursor on card, or on the nearest owner of it that is
// still showing. full lists every card, so its owners can be found.
func (m *Model) relocate(target card, full []card) {
	showing := flatten(m.dependencyRoot(), "")
	position := slices.IndexFunc(full, func(candidate card) bool {
		return candidate.node.Reference() == target.node.Reference()
	})
	if position < 0 {
		m.treeCursor = 0
		return
	}
	// The target itself when it is still showing, else walk up its owners,
	// which come before it in depth-first order at decreasing depths.
	wanted := target.depth
	for index := position; index >= 0; index-- {
		candidate := full[index]
		if index != position && candidate.depth >= wanted {
			continue
		}
		wanted = candidate.depth
		if shown := slices.IndexFunc(showing, func(shown card) bool {
			return shown.node.Reference() == candidate.node.Reference()
		}); shown >= 0 {
			m.treeCursor = shown
			return
		}
	}
	m.treeCursor = 0
}

// renderResourceTable lists the Application's resources as a table, indented
// under their owners, in the same order and with the same rows as the cards.
func (m Model) renderResourceTable(width, height int) []string {
	root, hidden := m.collapsedRoot()
	t := table{
		columns:  []string{"KIND", "NAME", "NAMESPACE", "HEALTH", "SYNC", "STATE"},
		status:   map[int]bool{3: true, 4: true},
		markable: true,
	}
	for position, item := range flatten(root, "") {
		node := item.node
		fold := foldMark(node, hidden[node.Reference()], position == 0)
		if fold == "" {
			fold = " "
		}
		var state []string
		if node.RequiresPruning {
			state = append(state, pruneGlyph+" to prune")
		}
		if node.Orphaned {
			state = append(state, orphanGlyph+" orphaned")
		}
		if count := hidden[node.Reference()]; count > 0 {
			state = append(state, "▸ "+strconv.Itoa(count)+" hidden")
		}
		t.rows = append(t.rows, []string{
			node.Kind, strings.Repeat("  ", item.depth) + fold + " " + node.Name, node.Namespace, node.Health, node.Sync, strings.Join(state, "  "),
		})
		t.marked = append(t.marked, m.markedCards[node.Reference()])
	}
	lines := t.render(m.treeCursor, width, height)
	for index := range lines {
		lines[index] = ansi.Truncate(lines[index], width, "…")
	}
	return lines
}
