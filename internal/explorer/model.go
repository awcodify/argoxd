package explorer

import (
	"strings"
	"time"
)

// Screen identifies the resource displayed in the main pane.
type Screen int

const (
	ApplicationsScreen Screen = iota
	ProjectsScreen
	ClustersScreen
	SettingsScreen
)

// Snapshot is the Argo CD state displayed by the explorer.
type Snapshot struct {
	Applications []Application
	Projects     []Project
	Clusters     []Cluster
}

// Application is an Argo CD application summary.
type Application struct {
	Name        string
	Namespace   string
	Project     string
	Sync        string
	Health      string
	Revision    string
	Destination string
	LastSync    time.Time
}

// Project is an Argo CD project summary.
type Project struct {
	Name        string
	Description string
}

// Cluster is an Argo CD destination-cluster summary.
type Cluster struct {
	Name   string
	Server string
}

// ResourceTree contains the resources managed by an Application.
type ResourceTree struct {
	Application string
	Nodes       []ResourceNode
}

// ResourceNode is a managed Kubernetes resource.
type ResourceNode struct {
	Group     string
	Version   string
	Kind      string
	Namespace string
	Name      string
	Sync      string
	Health    string
	Parents   []ResourceReference
	Children  []ResourceNode
}

// ResourceReference identifies a resource that owns or precedes another resource.
type ResourceReference struct {
	Group     string
	Kind      string
	Namespace string
	Name      string
}

// Model holds the explorer navigation state independently of its TUI.
type Model struct {
	snapshot Snapshot
	screen   Screen
	cursor   int
	project  string
	filter   string
}

// NewModel creates an explorer with Applications selected.
func NewModel(snapshot Snapshot) Model {
	return Model{snapshot: snapshot, screen: ApplicationsScreen}
}

// Screen returns the selected explorer screen.
func (m Model) Screen() Screen {
	return m.screen
}

// Cursor returns the selected row index on the current screen.
func (m Model) Cursor() int {
	return m.cursor
}

// SetScreen selects a screen, clears its filter and resets its row selection.
func (m *Model) SetScreen(screen Screen) {
	m.screen = screen
	m.filter = ""
	m.cursor = 0
}

// MoveDown selects the next row when one exists.
func (m *Model) MoveDown() {
	if m.cursor < m.RowCount()-1 {
		m.cursor++
	}
}

// MoveUp selects the previous row when one exists.
func (m *Model) MoveUp() {
	if m.cursor > 0 {
		m.cursor--
	}
}

// SelectRow selects a row on the current screen when it exists.
func (m *Model) SelectRow(index int) bool {
	if index < 0 || index >= m.RowCount() {
		return false
	}
	m.cursor = index
	return true
}

// Project returns the project Applications are filtered by; empty means all projects.
func (m Model) Project() string {
	return m.project
}

// SetProject filters Applications by project and resets the selection.
func (m *Model) SetProject(project string) {
	m.project = project
	m.cursor = 0
}

// Filter returns the text rows on the current screen are filtered by.
func (m Model) Filter() string {
	return m.filter
}

// SetFilter narrows the current screen to rows whose name contains the text,
// ignoring case, and resets the selection.
func (m *Model) SetFilter(text string) {
	m.filter = text
	m.cursor = 0
}

// Applications returns the Applications in the selected project that match the filter.
func (m Model) Applications() []Application {
	var matches []Application
	for _, application := range m.snapshot.Applications {
		if (m.project == "" || application.Project == m.project) && m.matches(application.Name) {
			matches = append(matches, application)
		}
	}
	return matches
}

// Projects returns the projects that match the filter.
func (m Model) Projects() []Project {
	var matches []Project
	for _, project := range m.snapshot.Projects {
		if m.matches(project.Name) {
			matches = append(matches, project)
		}
	}
	return matches
}

// Clusters returns the clusters that match the filter.
func (m Model) Clusters() []Cluster {
	var matches []Cluster
	for _, cluster := range m.snapshot.Clusters {
		if m.matches(cluster.Name) {
			matches = append(matches, cluster)
		}
	}
	return matches
}

func (m Model) matches(name string) bool {
	return strings.Contains(strings.ToLower(name), strings.ToLower(m.filter))
}

// RowCount returns the number of selectable rows on the current screen.
func (m Model) RowCount() int {
	return len(m.rowNames())
}

// SelectedName returns the selected resource name, if the screen has a selection.
func (m Model) SelectedName() string {
	if names := m.rowNames(); m.cursor < len(names) {
		return names[m.cursor]
	}
	return ""
}

func (m Model) rowNames() []string {
	var names []string
	switch m.screen {
	case ApplicationsScreen:
		for _, application := range m.Applications() {
			names = append(names, application.Name)
		}
	case ProjectsScreen:
		for _, project := range m.Projects() {
			names = append(names, project.Name)
		}
	case ClustersScreen:
		for _, cluster := range m.Clusters() {
			names = append(names, cluster.Name)
		}
	}
	return names
}

// Snapshot returns the current resource state.
func (m Model) Snapshot() Snapshot {
	return m.snapshot
}

// ReplaceSnapshot updates resource state, keeping the selected resource
// selected when it still exists.
func (m *Model) ReplaceSnapshot(snapshot Snapshot) {
	selected := m.SelectedName()
	m.snapshot = snapshot
	for index, name := range m.rowNames() {
		if name == selected {
			m.cursor = index
			return
		}
	}
	m.cursor = min(m.cursor, max(0, m.RowCount()-1))
}
