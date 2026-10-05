package tui

import (
	"strconv"
	"strings"

	"github.com/awcodify/argoxd/internal/explorer"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

const (
	cardWidth    = 46
	cardHeight   = 4
	detailsWidth = 40
	minTreeWidth = 56
)

// summaryOrder lists the statuses counted in the summary strip.
var summaryOrder = []string{"Healthy", "Progressing", "Suspended", "Unknown", "Missing", "Degraded", "OutOfSync"}

// card is a resource in the dependency view together with its owner.
type card struct {
	node  explorer.ResourceNode
	owner string
}

// renderDependencies shows a status summary, the Application and its managed
// resources as a tree of cards, and the details of the selected card.
func (m Model) renderDependencies(width, height int) []string {
	root := m.dependencyRoot()

	index := 0
	tree := renderBranch(root, m.treeCursor, m.treeSearch, m.treeFilter, "", &index)
	for row := range tree {
		tree[row] = " " + tree[row]
	}
	treeHeight := max(0, height-2)
	tree = tree[scrollOffset(m.treeCursor, cardHeight, treeHeight):]

	lines := []string{" " + resourceSummary(m.resourceTree.Nodes), ""}
	if width < minTreeWidth+detailsWidth {
		return append(lines, tree...)
	}

	details := renderDetails(flatten(root, "")[m.treeCursor])
	treeWidth := width - detailsWidth - 2
	for row := 0; row < treeHeight && (row < len(tree) || row < len(details)); row++ {
		left, right := "", ""
		if row < len(tree) {
			left = ansi.Truncate(tree[row], treeWidth, "…")
		}
		if row < len(details) {
			right = details[row]
		}
		lines = append(lines, padRight(left, treeWidth)+" "+right)
	}
	return lines
}

// dependencyRoot is the Application card with its resources nested beneath it.
func (m Model) dependencyRoot() explorer.ResourceNode {
	application := m.application()
	return explorer.ResourceNode{
		Group:     "argoproj.io",
		Kind:      "Application",
		Namespace: application.Namespace,
		Name:      application.Name,
		Sync:      application.Sync,
		Health:    application.Health,
		Children:  explorer.FilterHierarchy(m.resourceTree.Hierarchy(), m.treeSearch, m.treeFilter, application.Sync),
	}
}

// cardCount is the number of cards shown, after filtering.
func (m Model) cardCount() int {
	return len(flatten(m.dependencyRoot(), ""))
}

// selectedCard returns the resource of the selected card.
func (m Model) selectedCard() explorer.ResourceNode {
	return flatten(m.dependencyRoot(), "")[m.treeCursor].node
}

// renderBranch renders a card followed by its children, numbering cards in
// the same depth-first order the cursor moves through them. While searching or filtering,
// cards shown only to give a match its context are dimmed.
func renderBranch(node explorer.ResourceNode, selected int, search string, filter explorer.Filter, inheritedSync string, index *int) []string {
	context := (search != "" || filter.Active()) && *index > 0 && !node.Matches(search, filter, inheritedSync)
	lines := strings.Split(resourceCard(node, *index == selected, context), "\n")
	*index++
	for position, child := range node.Children {
		last := position == len(node.Children)-1
		for row, line := range renderBranch(child, selected, search, filter, node.EffectiveSync(inheritedSync), index) {
			lines = append(lines, "  "+mutedStyle.Render(connector(row, last))+line)
		}
	}
	return lines
}

func connector(row int, last bool) string {
	switch {
	case row == 0 && last:
		return "└─"
	case row == 0:
		return "├─"
	case last:
		return "  "
	default:
		return "│ "
	}
}

// resourceCard shows the kind in a border colored by status, the name with
// its namespace, and the health and sync status.
func resourceCard(node explorer.ResourceNode, selected, context bool) string {
	color := statusColor(node.Health, node.Sync)
	border := lipgloss.RoundedBorder()
	nameStyle := brightStyle
	switch {
	case selected:
		color = colorAccent
		border = lipgloss.ThickBorder()
	case context:
		color = colorBorder
		nameStyle = mutedStyle
	}
	inner := cardWidth - 4
	name := nameStyle.Render(ansi.Truncate(node.Name, inner, "…"))
	status := joinNonEmpty(statusBadge(node.Health), statusBadge(node.Sync))
	if status == "" {
		status = mutedStyle.Render("no status reported")
	}
	return box{
		border: border,
		color:  color,
		title:  " " + lipgloss.NewStyle().Foreground(color).Bold(true).Render(node.Kind) + " ",
		width:  cardWidth,
		height: cardHeight,
	}.render([]string{
		" " + spaceBetween(name, mutedStyle.Render(node.Namespace), inner),
		" " + status,
	})
}

// spaceBetween pushes right to the end of the line, or drops it when it does not fit.
func spaceBetween(left, right string, width int) string {
	gap := width - lipgloss.Width(left) - lipgloss.Width(right)
	if right == "" || gap < 2 {
		return left
	}
	return left + strings.Repeat(" ", gap) + right
}

func resourceSummary(nodes []explorer.ResourceNode) string {
	if len(nodes) == 0 {
		return mutedStyle.Render("No managed resources reported yet.")
	}
	counts := make(map[string]int)
	for _, node := range nodes {
		if node.Health != "" {
			counts[node.Health]++
		}
		if node.Sync == "OutOfSync" {
			counts[node.Sync]++
		}
	}
	var parts []string
	for _, status := range summaryOrder {
		if counts[status] == 0 {
			continue
		}
		label := strings.ToLower(status)
		if status == "OutOfSync" {
			label = "out of sync"
		}
		text := lookupStatus(status).glyph + " " + strconv.Itoa(counts[status]) + " " + label
		parts = append(parts, lipgloss.NewStyle().Foreground(statusColor(status)).Render(text))
	}
	return strings.Join(parts, "   ")
}

func flatten(node explorer.ResourceNode, owner string) []card {
	cards := []card{{node: node, owner: owner}}
	for _, child := range node.Children {
		cards = append(cards, flatten(child, node.Kind+"/"+node.Name)...)
	}
	return cards
}

func renderDetails(selected card) []string {
	node := selected.node
	rows := [][2]string{
		{"Kind", brightStyle.Render(node.Kind)},
		{"Group", textStyle.Render(node.Group)},
		{"Name", brightStyle.Render(node.Name)},
		{"Namespace", textStyle.Render(node.Namespace)},
		{"Health", statusBadge(node.Health)},
		{"Sync", statusBadge(node.Sync)},
		{"Owner", textStyle.Render(selected.owner)},
		{"Dependents", textStyle.Render(strconv.Itoa(len(node.Children)))},
	}
	lines := []string{""}
	for _, row := range rows {
		value := row[1]
		if lipgloss.Width(value) == 0 {
			value = mutedStyle.Render("—")
		}
		lines = append(lines, " "+mutedStyle.Render(padRight(row[0], 11))+value)
	}
	return strings.Split(box{
		border: lipgloss.RoundedBorder(),
		color:  colorBorder,
		title:  " " + accentStyle.Render("details") + " ",
		width:  detailsWidth,
		height: len(lines) + 3,
	}.render(lines), "\n")
}
