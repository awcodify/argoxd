// Package tui contains the interactive terminal interface.
package tui

import (
	"slices"
	"strings"
	"time"

	"github.com/awcodify/argoxd/internal/argocd"
	"github.com/awcodify/argoxd/internal/explorer"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const (
	defaultWidth          = 120
	defaultHeight         = 40
	defaultRequestTimeout = 15 * time.Second
)

// Model is the Bubble Tea model for argoxd.
type Model struct {
	explorer        explorer.Model
	source          argocd.Source
	connection      string
	namespace       string
	requestTimeout  time.Duration
	refreshInterval time.Duration
	loading         bool
	refreshing      bool
	err             error
	width           int
	height          int
	view            viewMode
	tree            []treeItem
	resourceTree    explorer.ResourceTree
	treeCursor      int
	treeSearch      string
	treeFilter      explorer.Filter
	expanded        map[string]bool
	viewer          textView
	viewerFromList  bool
	marked          map[string]bool
	markedCards     map[explorer.ResourceReference]bool
	prompt          prompt
	confirm         confirmation
	history         historyList
	follow          followState
	logTarget       explorer.ResourceNode
	logContainers   argocd.Containers
	containersKnown bool
	logContainer    string
	logSeen         string
	logWorkload     explorer.ResourceNode
	logPod          string
	logPods         []string
	status          string
}

type viewMode int

const (
	listView viewMode = iota
	inventoryTreeView
	applicationTreeView
	textViewMode
	historyViewMode
)

type treeItem struct {
	key         string
	parent      string
	label       string
	detail      string
	application string
	depth       int
	expandable  bool
}

// New creates the TUI from an initial resource snapshot.
func New(source argocd.Source, connection, namespace string, snapshot explorer.Snapshot) Model {
	return Model{
		explorer:       explorer.NewModel(snapshot),
		source:         source,
		connection:     connection,
		namespace:      namespace,
		requestTimeout: defaultRequestTimeout,
		width:          defaultWidth,
		height:         defaultHeight,
		expanded:       make(map[string]bool),
	}
}

// WithRefresh reloads resources every interval; zero disables auto-refresh.
func (m Model) WithRefresh(interval time.Duration) Model {
	m.refreshInterval = interval
	return m
}

// Init loads live data when a source is configured.
func (m Model) Init() tea.Cmd {
	if m.source == nil {
		return nil
	}
	return tea.Batch(m.load(), m.scheduleRefresh())
}

// Update handles TUI messages.
func (m Model) Update(message tea.Msg) (tea.Model, tea.Cmd) {
	switch message := message.(type) {
	case tea.KeyMsg:
		return m.updateKey(message)
	case tea.WindowSizeMsg:
		m.width = message.Width
		m.height = message.Height
	case refreshTick:
		return m.autoRefresh()
	case loadedSnapshot:
		m.loading = false
		m.refreshing = false
		m.err = message.err
		if message.err == nil {
			m.explorer.ReplaceSnapshot(message.snapshot)
			m.pruneMarks()
		}
	case loadedTree:
		m.loading = false
		m.applyTree(message)
	case loadedText:
		m.loading = false
		m.err = message.err
		if message.err == nil {
			if message.kind == "logs" {
				m.resetLogOptions()
			}
			m.viewer = newTextView(message.kind, message.subject, message.text)
			m.viewerFromList = message.fromList
			m.view = textViewMode
		}
	case loadedHistory:
		m.loading = false
		m.applyHistory(message)
	case loadedContainers:
		return m.applyContainers(message)
	case loadedLogs:
		return m.applyLogs(message)
	case logStreamStarted:
		return m.applyLogStream(message)
	case logLine:
		return m.applyLogLine(message)
	case logStreamEnded:
		if m.following() && message.stream == m.follow.id {
			m.stopFollowing()
			m.status = "log stream ended"
		}
	case operationCompleted:
		m.loading = false
		m.err = message.err
		if message.err == nil {
			subject := message.application
			if message.subject != "" {
				subject = message.subject
			}
			m.status = message.action + " requested for " + subject
			m.loading = true
			if message.refreshTree {
				return m, tea.Batch(m.load(), m.loadApplicationTree(message.application, true))
			}
			return m, m.load()
		}
	}
	return m, nil
}

// updateKey routes a key to the dialog or prompt that has focus, or else to the active view.
func (m Model) updateKey(message tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case m.confirm.active():
		return m.updateConfirm(message)
	case m.prompt.active:
		return m.updatePrompt(message)
	case m.view == textViewMode:
		return m.updateViewer(message)
	case m.view == historyViewMode:
		return m.updateHistory(message)
	}

	switch key := message.String(); key {
	case "ctrl+c", "q":
		return m, tea.Quit
	case "up", "k":
		m.moveUp()
	case "down", "j":
		m.moveDown()
	case ":":
		m.prompt = prompt{active: true}
	case "/":
		switch m.view {
		case listView:
			m.prompt = prompt{active: true, search: true, input: m.explorer.Search()}
		case applicationTreeView:
			m.prompt = prompt{active: true, search: true, input: m.treeSearch}
		}
	case "H", "S", "K":
		m.openFilter(key)
	case "0", "1", "2", "3", "4", "5", "6", "7", "8", "9":
		m.selectProject(key)
	case "t":
		m.openInventoryTree()
	case "enter":
		if m.view == listView {
			switch m.explorer.Screen() {
			case explorer.ApplicationsScreen:
				return m.openDependencies()
			case explorer.ProjectsScreen:
				m.openProjectApplications()
				return m, nil
			}
		}
		m.toggleTreeItem()
	case " ":
		m.toggleMark()
	case "right", "left":
		m.toggleTreeItem()
	case "esc":
		if m.searching() {
			m.clearSearchAndFilter()
			break
		}
		if m.markCount() > 0 {
			m.clearMarks()
			break
		}
		m.closeTree()
	case "y", "d", "l":
		if m.view == applicationTreeView {
			return m.inspect(key)
		}
	case "e":
		return m.showEvents()
	case "i":
		return m.showSyncDetails()
	case "s":
		m = m.askApplicationAction(confirmSync)
	case "x", "X":
		if m.view == applicationTreeView {
			return m.askResourceAction(key)
		}
	case "h":
		if application := m.selectedApplication(); application != "" {
			return m.openHistory(application)
		}
	case "R":
		m = m.askApplicationAction(confirmHardRefresh)
	case "D":
		m = m.askApplicationAction(confirmDeleteApplications)
	case "r":
		if m.source != nil && !m.loading {
			m.loading = true
			m.err = nil
			return m, m.load()
		}
	}
	return m, nil
}

// openProjectApplications filters Applications to the selected Project.
func (m *Model) openProjectApplications() {
	project := m.explorer.SelectedName()
	if project == "" {
		return
	}
	m.explorer.SetProject(project)
	m.showScreen(explorer.ApplicationsScreen)
}

// View renders the header, the optional prompt, the framed view, the
// breadcrumbs and the status line.
func (m Model) View() string {
	sections := []string{m.renderHeader(m.width)}
	if m.prompt.active {
		sections = append(sections, m.renderPrompt(m.width))
	}
	title, lines := m.renderContent(m.width-2, m.bodyHeight())
	main := box{
		border: lipgloss.RoundedBorder(),
		color:  colorBorder,
		title:  title,
		width:  m.width,
		height: m.bodyHeight() + 2,
	}.render(lines)
	return strings.Join(append(sections, main, m.renderCrumbs(), m.renderFlash()), "\n")
}

// bodyHeight is the number of lines available inside the main frame.
func (m Model) bodyHeight() int {
	height := m.height - headerHeight - 2
	if m.prompt.active {
		height -= promptHeight
	}
	return max(1, height-2)
}

// Explorer returns the resource-navigation state.
func (m Model) Explorer() explorer.Model {
	return m.explorer
}

func (m *Model) showScreen(screen explorer.Screen) {
	m.explorer.SetScreen(screen)
	m.closeTree()
}

// closeTree returns to the resource list, keeping its selection.
func (m *Model) closeTree() {
	m.view = listView
	m.tree = nil
	m.resourceTree = explorer.ResourceTree{}
	m.treeCursor = 0
	m.treeSearch = ""
	m.treeFilter = explorer.Filter{}
	m.markedCards = nil
}

// searchAndFilter returns the search text and status filter of the active view:
// the list or the dependency tree.
func (m Model) searchAndFilter() (string, explorer.Filter) {
	switch m.view {
	case listView:
		return m.explorer.Search(), m.explorer.Filter()
	case applicationTreeView:
		return m.treeSearch, m.treeFilter
	default:
		return "", explorer.Filter{}
	}
}

// searching reports whether the active view is narrowed by a search or filter.
func (m Model) searching() bool {
	search, filter := m.searchAndFilter()
	return search != "" || filter.Active()
}

// applySearch narrows the list or, in the dependency view, the card tree.
func (m *Model) applySearch(text string) {
	if m.view == textViewMode {
		m.viewer.search = text
		m.viewer.offset = max(0, len(m.viewer.shown())-m.bodyHeight())
		return
	}
	if m.view == applicationTreeView {
		m.treeSearch = text
		m.treeCursor = 0
		return
	}
	m.explorer.SetSearch(text)
}

// clearSearchAndFilter removes the search and status filter of the active view.
func (m *Model) clearSearchAndFilter() {
	m.applySearch("")
	m.setFilter(explorer.Filter{})
}

func (m *Model) setFilter(filter explorer.Filter) {
	if m.view == applicationTreeView {
		m.treeFilter = filter
		m.treeCursor = 0
		return
	}
	m.explorer.SetFilter(filter)
}

// openFilter opens the prompt that sets the health (H), sync (S) or kind (K)
// filter. Applications have no kind, so K only applies to the dependency view.
func (m *Model) openFilter(key string) {
	_, filter := m.searchAndFilter()
	field, current := "health", filter.Health
	switch key {
	case "S":
		field, current = "sync", filter.Sync
	case "K":
		field, current = "kind", filter.Kind
	}
	if m.canFilterBy(field) {
		// Start on the current value so Tab moves on to the next one.
		options := valueSuggestions(field, "", m.filterKinds())
		m.prompt = prompt{active: true, field: field, selection: max(0, slices.Index(options, current))}
	}
}

// applyTree shows a loaded resource tree. A tree from a background refresh
// only replaces the one on screen, keeping the selected card.
func (m *Model) applyTree(message loadedTree) {
	if message.background {
		showing := m.view == applicationTreeView || m.view == textViewMode || m.view == historyViewMode
		if message.err == nil && showing && m.resourceTree.Application == message.tree.Application {
			m.resourceTree = message.tree
			m.treeCursor = min(m.treeCursor, m.cardCount()-1)
		}
		return
	}
	m.err = message.err
	if message.err == nil {
		m.view = applicationTreeView
		m.resourceTree = message.tree
		m.treeCursor = 0
	}
}

func (m *Model) openInventoryTree() {
	m.view = inventoryTreeView
	m.tree = nil
	m.treeCursor = 0

	snapshot := m.explorer.Snapshot()
	projectsKey := "inventory:projects"
	clustersKey := "inventory:clusters"
	m.expanded[projectsKey] = true
	m.expanded[clustersKey] = true
	m.tree = append(m.tree, treeItem{key: projectsKey, label: "Projects", expandable: true})
	for _, project := range snapshot.Projects {
		projectKey := "project:" + project.Name
		m.expanded[projectKey] = true
		m.tree = append(m.tree, treeItem{key: projectKey, parent: projectsKey, label: project.Name, detail: project.Description, depth: 1, expandable: true})
		for _, application := range snapshot.Applications {
			if application.Project == project.Name {
				m.tree = append(m.tree, treeItem{
					key:         "application:" + application.Name,
					parent:      projectKey,
					label:       application.Name,
					detail:      joinNonEmpty(application.Sync, application.Health),
					application: application.Name,
					depth:       2,
				})
			}
		}
	}
	m.tree = append(m.tree, treeItem{key: clustersKey, label: "Clusters", expandable: true})
	for _, cluster := range snapshot.Clusters {
		m.tree = append(m.tree, treeItem{key: "cluster:" + cluster.Name, parent: clustersKey, label: cluster.Name, detail: cluster.Server, depth: 1})
	}
}

func (m *Model) moveUp() {
	if m.view == listView {
		m.explorer.MoveUp()
		return
	}
	if m.treeCursor > 0 {
		m.treeCursor--
	}
}

func (m *Model) moveDown() {
	if m.view == listView {
		m.explorer.MoveDown()
		return
	}
	if m.treeCursor < m.treeRowCount()-1 {
		m.treeCursor++
	}
}

// treeRowCount counts the selectable rows of a tree view.
func (m Model) treeRowCount() int {
	if m.view == applicationTreeView {
		return m.cardCount()
	}
	return len(m.visibleTree())
}

func (m *Model) toggleTreeItem() {
	if m.view != inventoryTreeView || len(m.visibleTree()) == 0 {
		return
	}
	item := m.visibleTree()[m.treeCursor]
	if item.expandable {
		m.expanded[item.key] = !m.expanded[item.key]
	}
}

func (m Model) selectedApplication() string {
	switch m.view {
	case applicationTreeView, textViewMode:
		return m.resourceTree.Application
	case inventoryTreeView:
		if visible := m.visibleTree(); len(visible) > 0 {
			return visible[m.treeCursor].application
		}
	default:
		if m.explorer.Screen() == explorer.ApplicationsScreen {
			return m.explorer.SelectedName()
		}
	}
	return ""
}

func (m Model) visibleTree() []treeItem {
	visible := make([]treeItem, 0, len(m.tree))
	visibleKeys := make(map[string]bool, len(m.tree))
	for _, item := range m.tree {
		if item.parent == "" || (visibleKeys[item.parent] && m.expanded[item.parent]) {
			visible = append(visible, item)
			visibleKeys[item.key] = true
		}
	}
	return visible
}

// application returns the Application whose dependencies are displayed.
func (m Model) application() explorer.Application {
	name := m.resourceTree.Application
	for _, application := range m.explorer.Snapshot().Applications {
		if application.Name == name {
			return application
		}
	}
	return explorer.Application{Name: name, Sync: "Unknown", Health: "Unknown"}
}
