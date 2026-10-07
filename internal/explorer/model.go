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
	ApplicationSetsScreen
	PulseScreen
)

// Snapshot is the Argo CD state displayed by the explorer.
type Snapshot struct {
	Applications    []Application
	Projects        []Project
	Clusters        []Cluster
	ApplicationSets []ApplicationSet
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
	// Conditions are the warnings and errors Argo CD reports for the Application.
	Conditions []Condition
	// Policy is how the Application syncs on its own.
	Policy SyncPolicy
	// Operation is the Application's last sync; nil if it never synced.
	Operation *Operation
	// Owner is the ApplicationSet that generated the Application; empty if
	// someone created it.
	Owner string
}

// ApplicationSet generates Applications from templates, one for each set of
// parameters its generators produce.
type ApplicationSet struct {
	Name      string
	Namespace string
	// Generators names the generators, such as "git" or "matrix(git, clusters)".
	Generators []string
	// Problems are the conditions that say something is wrong, such as an ErrorOccurred.
	Problems []Condition
}

// Operation is the outcome of an Application's last sync.
type Operation struct {
	// Phase is Running, Succeeded, Failed, Error or Terminating.
	Phase      string
	Message    string
	Revision   string
	StartedAt  time.Time
	FinishedAt time.Time
	Results    []OperationResult
}

// OperationResult is what a sync did to one resource or hook.
type OperationResult struct {
	Group     string
	Kind      string
	Namespace string
	Name      string
	// Status is the result of applying the resource, such as Synced or SyncFailed.
	Status  string
	Message string
	// HookType is set for a hook, such as PreSync.
	HookType  string
	HookPhase string
	// SyncPhase is when in the sync the resource was applied: PreSync, Sync, PostSync or SyncFail.
	SyncPhase string
}

// Condition is a warning or error Argo CD reports for an Application, such as
// a ComparisonError.
type Condition struct {
	Type    string
	Message string
}

// SyncPolicy is the automation of an Application's syncs.
type SyncPolicy struct {
	// Automated syncs the Application whenever it is out of sync.
	Automated bool
	// SelfHeal also syncs when the live cluster drifts from Git.
	SelfHeal bool
	// Prune lets an automated sync delete resources that are no longer in Git.
	Prune bool
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

// HistoryEntry is one deployment recorded in an Application's sync history.
type HistoryEntry struct {
	ID         int64
	Revision   string
	DeployedAt time.Time
	Repo       string
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
	// UID identifies the live object; only the Argo CD API reports it.
	UID    string
	Sync   string
	Health string
	// RequiresPruning marks a resource that is no longer in Git and that a
	// sync with prune would delete.
	RequiresPruning bool
	// Orphaned marks a resource in the Application's namespace that no
	// Application manages; only the Argo CD API reports these.
	Orphaned bool
	Parents  []ResourceReference
	Children []ResourceNode
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
	// owner narrows Applications to those an ApplicationSet generated; it and
	// project replace each other.
	owner  string
	search string
	filter Filter
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

// SetScreen selects a screen, clears its search and filter and resets its row selection.
func (m *Model) SetScreen(screen Screen) {
	m.screen = screen
	m.search = ""
	m.filter = Filter{}
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
	m.project, m.owner = project, ""
	m.cursor = 0
}

// Owner returns the ApplicationSet whose Applications are shown; empty means any.
func (m Model) Owner() string {
	return m.owner
}

// SetOwner shows only the Applications an ApplicationSet generated, replacing
// the project filter, and resets the selection.
func (m *Model) SetOwner(owner string) {
	m.project, m.owner = "", owner
	m.cursor = 0
}

// ApplicationSets returns the ApplicationSets that match the search.
func (m Model) ApplicationSets() []ApplicationSet {
	var matches []ApplicationSet
	for _, applicationSet := range m.snapshot.ApplicationSets {
		if m.matchesSearch(applicationSet.Name) {
			matches = append(matches, applicationSet)
		}
	}
	return matches
}

// GeneratedApplications counts the Applications an ApplicationSet generated.
func (m Model) GeneratedApplications(name string) int {
	generated := 0
	for _, application := range m.snapshot.Applications {
		if application.Owner == name {
			generated++
		}
	}
	return generated
}

// Search returns the text rows on the current screen are searched by.
func (m Model) Search() string {
	return m.search
}

// SetSearch narrows the current screen to rows whose name contains the text,
// ignoring case, and resets the selection.
func (m *Model) SetSearch(text string) {
	m.search = text
	m.cursor = 0
}

// Filter returns the status filter applied to Applications.
func (m Model) Filter() Filter {
	return m.filter
}

// SetFilter narrows Applications by health and sync status and resets the selection.
func (m *Model) SetFilter(filter Filter) {
	m.filter = filter
	m.cursor = 0
}

// Applications returns the Applications in the selected project that match the search and filter.
func (m Model) Applications() []Application {
	var matches []Application
	for _, application := range m.snapshot.Applications {
		if (m.project == "" || application.Project == m.project) && (m.owner == "" || application.Owner == m.owner) && m.matchesSearch(application.Name) &&
			m.filter.Matches(application.Sync, application.Health, "") {
			matches = append(matches, application)
		}
	}
	return matches
}

// Projects returns the projects that match the search.
func (m Model) Projects() []Project {
	var matches []Project
	for _, project := range m.snapshot.Projects {
		if m.matchesSearch(project.Name) {
			matches = append(matches, project)
		}
	}
	return matches
}

// Clusters returns the clusters that match the search.
func (m Model) Clusters() []Cluster {
	var matches []Cluster
	for _, cluster := range m.snapshot.Clusters {
		if m.matchesSearch(cluster.Name) {
			matches = append(matches, cluster)
		}
	}
	return matches
}

func (m Model) matchesSearch(name string) bool {
	return strings.Contains(strings.ToLower(name), strings.ToLower(m.search))
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
	case ApplicationSetsScreen:
		for _, applicationSet := range m.ApplicationSets() {
			names = append(names, applicationSet.Name)
		}
	case PulseScreen:
		for _, item := range m.Attention() {
			names = append(names, item.Application.Name)
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
