package explorer

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
	Name      string
	Namespace string
	Project   string
	Sync      string
	Health    string
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

// SetScreen selects a screen and resets its row selection.
func (m *Model) SetScreen(screen Screen) {
	m.screen = screen
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

// Applications returns the Applications in the selected project.
func (m Model) Applications() []Application {
	if m.project == "" {
		return m.snapshot.Applications
	}
	var filtered []Application
	for _, application := range m.snapshot.Applications {
		if application.Project == m.project {
			filtered = append(filtered, application)
		}
	}
	return filtered
}

// RowCount returns the number of selectable rows on the current screen.
func (m Model) RowCount() int {
	switch m.screen {
	case ApplicationsScreen:
		return len(m.Applications())
	case ProjectsScreen:
		return len(m.snapshot.Projects)
	case ClustersScreen:
		return len(m.snapshot.Clusters)
	default:
		return 0
	}
}

// SelectedName returns the selected resource name, if the screen has a selection.
func (m Model) SelectedName() string {
	switch m.screen {
	case ApplicationsScreen:
		if applications := m.Applications(); len(applications) > 0 {
			return applications[m.cursor].Name
		}
	case ProjectsScreen:
		if len(m.snapshot.Projects) > 0 {
			return m.snapshot.Projects[m.cursor].Name
		}
	case ClustersScreen:
		if len(m.snapshot.Clusters) > 0 {
			return m.snapshot.Clusters[m.cursor].Name
		}
	}
	return ""
}

// Snapshot returns the current resource state.
func (m Model) Snapshot() Snapshot {
	return m.snapshot
}

// ReplaceSnapshot updates resource state and keeps the selection valid.
func (m *Model) ReplaceSnapshot(snapshot Snapshot) {
	m.snapshot = snapshot
	if m.cursor >= m.RowCount() {
		m.cursor = max(0, m.RowCount()-1)
	}
}
