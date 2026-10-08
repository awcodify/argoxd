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
	// maxBannerConditions is how many of an Application's conditions are listed
	// in the summary below its cards.
	maxBannerConditions = 3
	// maxConditionLines is how many lines one condition may take before it is cut.
	maxConditionLines = 2
)

// summaryOrder lists the statuses counted in the summary strip.
var summaryOrder = []string{"Healthy", "Progressing", "Suspended", "Unknown", "Missing", "Degraded", "OutOfSync"}

// card is a resource in the dependency view together with its owner.
type card struct {
	node  explorer.ResourceNode
	owner string
}

// renderDependencies shows the Application and its managed resources as a tree
// of cards, the details of the selected card beside it, and below them a
// summary of the resources' statuses and the Application's conditions.
func (m Model) renderDependencies(width, height int) []string {
	root := m.dependencyRoot()

	index := 0
	tree := renderBranch(root, m.treeCursor, m.treeSearch, m.treeFilter, m.markedCards, "", &index)
	for row := range tree {
		tree[row] = " " + tree[row]
	}
	summary := m.renderSummary(width)
	treeHeight := max(0, height-len(summary))
	tree = tree[scrollOffset(m.treeCursor, cardHeight, treeHeight):]

	var lines []string
	if width < minTreeWidth+detailsWidth {
		lines = tree[:min(len(tree), treeHeight)]
	} else {
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
	}
	// The summary stays at the bottom, whatever the number of cards.
	for len(lines) < treeHeight {
		lines = append(lines, "")
	}
	return append(lines, summary...)
}

// renderSummary frames the count of resources by status and the first
// maxBannerConditions conditions of the Application. The frame is amber when
// there are conditions.
func (m Model) renderSummary(width int) []string {
	conditions := m.application().Conditions
	content := []string{" " + resourceSummary(m.resourceTree.Nodes)}
	for _, line := range conditionBanner(conditions, width-7) { // two margins, two borders and the box's own margin
		content = append(content, " "+line)
	}
	color := colorBorder
	if len(conditions) > 0 {
		color = colorAmber
	}
	lines := strings.Split(box{
		border: lipgloss.RoundedBorder(),
		color:  color,
		title:  " " + accentStyle.Render("summary") + " ",
		width:  max(0, width-2),
		height: len(content) + 2,
	}.render(content), "\n")
	for row := range lines {
		lines[row] = " " + lines[row]
	}
	return lines
}

// conditionBanner lists the first maxBannerConditions of an Application's
// conditions and counts the rest. A condition takes up to maxConditionLines
// lines of width, the later ones indented under its type, and is cut with an
// ellipsis if it needs more.
func conditionBanner(conditions []explorer.Condition, width int) []string {
	var lines []string
	for _, condition := range conditions[:min(len(conditions), maxBannerConditions)] {
		text := conditionGlyph + " " + condition.Type + ": " + condition.Message
		wrapped := strings.Split(ansi.Wrap(text, max(1, width-2), ""), "\n")
		if len(wrapped) > maxConditionLines {
			wrapped = wrapped[:maxConditionLines]
			wrapped[maxConditionLines-1] = strings.TrimRight(wrapped[maxConditionLines-1], " ") + "…"
		}
		for row, line := range wrapped {
			if row > 0 {
				line = "  " + line
			}
			lines = append(lines, warningStyle.Render(line))
		}
	}
	if more := len(conditions) - maxBannerConditions; more > 0 {
		lines = append(lines, mutedStyle.Render("  +"+strconv.Itoa(more)+" more"))
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
func renderBranch(node explorer.ResourceNode, selected int, search string, filter explorer.Filter, marked map[explorer.ResourceReference]bool, inheritedSync string, index *int) []string {
	context := (search != "" || filter.Active()) && *index > 0 && !node.Matches(search, filter, inheritedSync)
	lines := strings.Split(drawCard(node, *index == selected, context, marked[node.Reference()]), "\n")
	*index++
	for position, child := range node.Children {
		last := position == len(node.Children)-1
		for row, line := range renderBranch(child, selected, search, filter, marked, node.EffectiveSync(inheritedSync), index) {
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
	return drawCard(node, selected, context, false)
}

// drawCard renders a card; a marked one has a ● before its kind.
func drawCard(node explorer.ResourceNode, selected, context, marked bool) string {
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
	name := nameStyle.Render(truncateMiddle(node.Name, inner))
	status := joinNonEmpty(statusBadge(node.Health), statusBadge(node.Sync))
	if node.RequiresPruning {
		status = joinNonEmpty(status, warningStyle.Render(pruneGlyph+" to prune"))
	}
	if node.Orphaned {
		status = joinNonEmpty(status, warningStyle.Render(orphanGlyph+" orphaned"))
	}
	if status == "" {
		status = mutedStyle.Render("no status reported")
	}
	return box{
		border: border,
		color:  color,
		title:  " " + markPrefix(marked) + lipgloss.NewStyle().Foreground(color).Bold(true).Render(node.Kind) + " ",
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
	toPrune, orphaned := 0, 0
	for _, node := range nodes {
		if node.RequiresPruning {
			toPrune++
		}
		if node.Orphaned {
			orphaned++
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
	if toPrune > 0 {
		parts = append(parts, warningStyle.Render(pruneGlyph+" "+strconv.Itoa(toPrune)+" to prune"))
	}
	if orphaned > 0 {
		parts = append(parts, warningStyle.Render(orphanGlyph+" "+strconv.Itoa(orphaned)+" orphaned"))
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
		{"Name", brightStyle.Render(truncateMiddle(node.Name, detailsWidth-2-1-11))},
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
	var notes []string
	if node.RequiresPruning {
		notes = []string{"No longer in Git. It will be", "deleted by a sync with prune."}
	}
	if node.Orphaned {
		notes = []string{"In the destination namespace but", "not managed by this application."}
	}
	if len(notes) > 0 {
		lines = append(lines, "")
	}
	for _, note := range notes {
		lines = append(lines, " "+warningStyle.Render(note))
	}
	return strings.Split(box{
		border: lipgloss.RoundedBorder(),
		color:  colorBorder,
		title:  " " + accentStyle.Render("details") + " ",
		width:  detailsWidth,
		height: len(lines) + 3,
	}.render(lines), "\n")
}

// truncateMiddle shortens a name to width columns by cutting out its middle, so
// the end stays visible. Generated names, such as a Pod's, differ only there.
func truncateMiddle(name string, width int) string {
	if ansi.StringWidth(name) <= width {
		return name
	}
	tail := min(12, width/3)
	head := max(0, width-tail-1)
	return ansi.Truncate(name, head, "") + "…" + ansi.TruncateLeft(name, ansi.StringWidth(name)-tail, "")
}

func markPrefix(marked bool) string {
	if marked {
		return accentStyle.Render(markGlyph) + " "
	}
	return ""
}
