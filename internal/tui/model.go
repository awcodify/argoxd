// Package tui contains the interactive terminal interface.
package tui

import (
	"context"
	"fmt"
	"strings"

	"github.com/awcodify/argoxd/internal/argocd"
	"github.com/awcodify/argoxd/internal/explorer"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const (
	defaultWidth  = 120
	defaultHeight = 40
)

// Model is the Bubble Tea model for argoxd.
type Model struct {
	explorer     explorer.Model
	source       argocd.Source
	connection   string
	namespace    string
	loading      bool
	err          error
	width        int
	height       int
	view         viewMode
	tree         []treeItem
	resourceTree explorer.ResourceTree
	treeCursor   int
	expanded     map[string]bool
	confirming   string
	prompt       prompt
	status       string
}

type viewMode int

const (
	listView viewMode = iota
	inventoryTreeView
	applicationTreeView
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
		explorer:   explorer.NewModel(snapshot),
		source:     source,
		connection: connection,
		namespace:  namespace,
		width:      defaultWidth,
		height:     defaultHeight,
		expanded:   make(map[string]bool),
	}
}

// Init loads live data when a source is configured.
func (m Model) Init() tea.Cmd {
	if m.source == nil {
		return nil
	}
	return m.load()
}

// Update handles TUI messages.
func (m Model) Update(message tea.Msg) (tea.Model, tea.Cmd) {
	switch message := message.(type) {
	case tea.KeyMsg:
		if m.confirming != "" {
			switch message.String() {
			case "y":
				application := m.confirming
				m.confirming = ""
				m.loading = true
				return m, m.operate("delete", application)
			case "n", "esc":
				m.confirming = ""
			}
			return m, nil
		}
		if m.prompt.active {
			return m.updatePrompt(message)
		}
		switch message.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "up", "k":
			m.moveUp()
		case "down", "j":
			m.moveDown()
		case ":":
			m.prompt = prompt{active: true}
		case "0", "1", "2", "3", "4", "5", "6", "7", "8", "9":
			m.selectProject(message.String())
		case "t":
			m.openInventoryTree()
		case "enter":
			if m.view == listView && m.explorer.Screen() == explorer.ApplicationsScreen {
				application := m.explorer.SelectedName()
				if application == "" {
					return m, nil
				}
				if _, ok := m.source.(argocd.ApplicationOperator); !ok {
					m.err = fmt.Errorf("the active source does not support application trees")
					return m, nil
				}
				m.loading = true
				return m, m.loadApplicationTree(application)
			}
			m.toggleTreeItem()
		case " ", "right", "left":
			m.toggleTreeItem()
		case "esc":
			m.closeTree()
		case "s":
			if application := m.selectedApplication(); application != "" {
				m.loading = true
				return m, m.operate("sync", application)
			}
		case "d":
			if application := m.selectedApplication(); application != "" {
				m.confirming = application
			}
		case "r":
			if m.source != nil && !m.loading {
				m.loading = true
				m.err = nil
				return m, m.load()
			}
		}
	case tea.WindowSizeMsg:
		m.width = message.Width
		m.height = message.Height
	case loadedSnapshot:
		m.loading = false
		m.err = message.err
		if message.err == nil {
			m.explorer.ReplaceSnapshot(message.snapshot)
		}
	case loadedTree:
		m.loading = false
		m.err = message.err
		if message.err == nil {
			m.setApplicationTree(message.tree)
		}
	case operationCompleted:
		m.loading = false
		m.err = message.err
		if message.err == nil {
			m.status = message.action + " requested for " + message.application
			m.loading = true
			return m, m.load()
		}
	}
	return m, nil
}

// View renders the header, the optional command prompt, the framed resource
// view, the breadcrumbs and the status line.
func (m Model) View() string {
	sections := []string{m.renderHeader(m.width)}
	frameHeight := max(3, m.height-headerHeight-2)
	if m.prompt.active {
		sections = append(sections, m.renderPrompt(m.width))
		frameHeight = max(3, frameHeight-promptHeight)
	}
	title, lines := m.renderContent(m.width-2, frameHeight-2)
	main := box{
		border: lipgloss.RoundedBorder(),
		color:  colorBorder,
		title:  title,
		width:  m.width,
		height: frameHeight,
	}.render(lines)
	return strings.Join(append(sections, main, m.renderCrumbs(), m.renderFlash()), "\n")
}

// Explorer returns the resource-navigation state.
func (m Model) Explorer() explorer.Model {
	return m.explorer
}

type loadedSnapshot struct {
	snapshot explorer.Snapshot
	err      error
}

type loadedTree struct {
	tree explorer.ResourceTree
	err  error
}

type operationCompleted struct {
	action      string
	application string
	err         error
}

func (m Model) load() tea.Cmd {
	return func() tea.Msg {
		snapshot, err := m.source.Load(context.Background())
		return loadedSnapshot{snapshot: snapshot, err: err}
	}
}

func (m Model) loadApplicationTree(application string) tea.Cmd {
	operator, ok := m.source.(argocd.ApplicationOperator)
	if !ok {
		return nil
	}
	return func() tea.Msg {
		tree, err := operator.LoadResourceTree(context.Background(), application)
		return loadedTree{tree: tree, err: err}
	}
}

func (m Model) operate(action, application string) tea.Cmd {
	operator, ok := m.source.(argocd.ApplicationOperator)
	if !ok {
		return func() tea.Msg {
			return operationCompleted{action: action, application: application, err: fmt.Errorf("the active source does not support application operations")}
		}
	}
	return func() tea.Msg {
		var err error
		if action == "sync" {
			err = operator.SyncApplication(context.Background(), application)
		} else {
			err = operator.DeleteApplication(context.Background(), application)
		}
		return operationCompleted{action: action, application: application, err: err}
	}
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
}

func (m *Model) setApplicationTree(tree explorer.ResourceTree) {
	m.view = applicationTreeView
	m.resourceTree = tree
	m.treeCursor = 0
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

// treeRowCount counts the selectable rows of a tree view. The dependency
// view has one card for the Application plus one per managed resource.
func (m Model) treeRowCount() int {
	if m.view == applicationTreeView {
		return 1 + len(m.resourceTree.Nodes)
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
	case applicationTreeView:
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
