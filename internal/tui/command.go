package tui

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/awcodify/argoxd/internal/argocd"
	"github.com/awcodify/argoxd/internal/explorer"
	tea "github.com/charmbracelet/bubbletea"
)

// viewCommand is a view the ':' prompt can open. Every name is an alias.
type viewCommand struct {
	names        []string
	screen       explorer.Screen
	takesProject bool
}

var commands = []viewCommand{
	{names: []string{"app", "apps", "applications"}, screen: explorer.ApplicationsScreen, takesProject: true},
	{names: []string{"proj", "projects"}, screen: explorer.ProjectsScreen},
	{names: []string{"cluster", "clusters"}, screen: explorer.ClustersScreen},
	{names: []string{"appset", "appsets", "applicationset", "applicationsets"}, screen: explorer.ApplicationSetsScreen},
	{names: []string{"pulse", "overview"}, screen: explorer.PulseScreen},
	{names: []string{"settings"}, screen: explorer.SettingsScreen},
}

var quitCommands = []string{"q", "quit"}

var contextCommands = []string{"ctx", "context"}

var namespaceCommands = []string{"ns", "namespace"}

const (
	contextField   = "context"
	namespaceField = "namespace"
)

// filterCommand is a ':' command that sets one filter, e.g. "health degraded".
// Its shortcut opens the same filter from the keyboard.
type filterCommand struct {
	name     string
	shortcut string
}

var filterCommands = []filterCommand{
	{name: "health", shortcut: "H"},
	{name: "sync", shortcut: "S"},
	{name: "kind", shortcut: "K"},
}

func findFilterCommand(name string) (filterCommand, bool) {
	for _, command := range filterCommands {
		if command.name == name {
			return command, true
		}
	}
	return filterCommand{}, false
}

// prompt is the command line opened with ':', the search opened with '/',
// or the filter of one field opened with H, S or K.
type prompt struct {
	active    bool
	search    bool
	field     string
	input     string
	selection int
}

func findCommand(name string) (viewCommand, bool) {
	for _, command := range commands {
		if slices.Contains(command.names, name) {
			return command, true
		}
	}
	return viewCommand{}, false
}

// suggestions completes the input: command names first, then the argument of
// commands that accept one: the project in "app store", or a value of a
// filter that is available here, such as "health Degraded".
func suggestions(input string, projects []string, filters map[string][]string) []string {
	if input == "" {
		return nil
	}
	var matches []string
	if name, argument, hasArgument := strings.Cut(input, " "); hasArgument {
		if command, found := findCommand(name); found && command.takesProject {
			for _, project := range projects {
				if strings.HasPrefix(project, argument) {
					matches = append(matches, name+" "+project)
				}
			}
		}
		for _, value := range filters[name] {
			if strings.HasPrefix(strings.ToLower(value), strings.ToLower(argument)) {
				matches = append(matches, name+" "+value)
			}
		}
	} else {
		for _, command := range commands {
			for _, alias := range command.names {
				if strings.HasPrefix(alias, input) {
					matches = append(matches, alias)
				}
			}
		}
		if strings.HasPrefix("quit", input) {
			matches = append(matches, "quit")
		}
		for _, command := range contextCommands {
			if strings.HasPrefix(command, input) {
				matches = append(matches, command)
			}
		}
		for _, command := range namespaceCommands {
			if strings.HasPrefix(command, input) {
				matches = append(matches, command)
			}
		}
		for _, command := range filterCommands {
			if _, available := filters[command.name]; available && strings.HasPrefix(command.name, input) {
				matches = append(matches, command.name)
			}
		}
	}
	slices.Sort(matches)
	return matches
}

// typed returns the prompt with new input and the first suggestion selected.
func (p prompt) typed(input string) prompt {
	p.input = input
	p.selection = 0
	return p
}

// valueSuggestions lists the choices of a filter field that start with the
// input: "all", which clears the field, then its values.
func valueSuggestions(field, input string, kinds []string) []string {
	var matches []string
	for _, value := range append([]string{explorer.AllValues}, explorer.FilterValues(field, kinds)...) {
		if strings.HasPrefix(strings.ToLower(value), strings.ToLower(input)) {
			matches = append(matches, value)
		}
	}
	return matches
}

// filterKinds lists the resource kinds a filter can select; only the
// dependency view has any.
func (m Model) filterKinds() []string {
	if m.view == applicationTreeView {
		return m.resourceTree.Kinds()
	}
	return nil
}

func (m Model) suggestions() []string {
	if m.prompt.search {
		return nil
	}
	if m.prompt.field == containerField {
		return containerSuggestions(m.prompt.input, m.logContainers)
	}
	if m.prompt.field == podField {
		return matchOptions(m.prompt.input, podOptions(m.logPods))
	}
	if m.prompt.field == contextField {
		return matchOptions(m.prompt.input, m.contextOptions)
	}
	if m.prompt.field == namespaceField {
		return matchOptions(m.prompt.input, m.namespaceOptions)
	}
	if m.prompt.field != "" {
		return valueSuggestions(m.prompt.field, m.prompt.input, m.filterKinds())
	}
	if name, context, hasContext := strings.Cut(m.prompt.input, " "); hasContext && slices.Contains(contextCommands, name) {
		var matches []string
		for _, candidate := range m.contextOptions {
			if strings.HasPrefix(candidate, context) {
				matches = append(matches, name+" "+candidate)
			}
		}
		return matches
	}
	projects := make([]string, 0, len(m.explorer.Snapshot().Projects))
	for _, project := range m.explorer.Snapshot().Projects {
		projects = append(projects, project.Name)
	}
	return suggestions(m.prompt.input, projects, m.availableFilters())
}

// canFilterBy reports whether the active view has a filter on the field. Only
// the Applications list and the dependency view do, and only the latter has kinds.
func (m Model) canFilterBy(field string) bool {
	switch {
	case m.view == applicationTreeView:
		return true
	case m.view == listView && m.explorer.Screen() == explorer.ApplicationsScreen:
		return field != "kind"
	default:
		return false
	}
}

// availableFilters maps each filter of the active view to the values it accepts.
func (m Model) availableFilters() map[string][]string {
	filters := make(map[string][]string)
	for _, command := range filterCommands {
		if m.canFilterBy(command.name) {
			filters[command.name] = append([]string{explorer.AllValues}, explorer.FilterValues(command.name, m.filterKinds())...)
		}
	}
	return filters
}

func (m Model) selectedSuggestion() string {
	if matches := m.suggestions(); len(matches) > 0 {
		return matches[m.prompt.selection%len(matches)]
	}
	return ""
}

func (m Model) updatePrompt(message tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.prompt.search {
		return m.updateSearch(message)
	}
	switch message.Type {
	case tea.KeyEsc:
		m.prompt = prompt{}
	case tea.KeyEnter:
		input := strings.TrimSpace(m.prompt.input)
		field, chosen := m.prompt.field, m.selectedSuggestion()
		m.prompt = prompt{}
		if field != "" {
			if chosen != "" {
				input = chosen
			}
			switch field {
			case containerField:
				return m.showContainer(input)
			case podField:
				return m.showPod(input)
			case contextField:
				return m, m.switchContext(input)
			case namespaceField:
				return m, m.switchNamespace(input)
			}
			m.runFilter(field, input)
			return m, nil
		}
		return m, m.runCommand(input)
	case tea.KeyTab, tea.KeyRight:
		if m.prompt.field != "" {
			m.prompt.selection++
		} else if suggestion := m.selectedSuggestion(); suggestion != "" {
			m.prompt = m.prompt.typed(suggestion)
		}
	case tea.KeyDown:
		m.prompt.selection++
	case tea.KeyUp, tea.KeyShiftTab:
		if count := len(m.suggestions()); count > 0 {
			m.prompt.selection = (m.prompt.selection + count - 1) % count
		}
	case tea.KeyBackspace:
		if m.prompt.input == "" {
			m.prompt = prompt{}
			break
		}
		runes := []rune(m.prompt.input)
		m.prompt = m.prompt.typed(string(runes[:len(runes)-1]))
	case tea.KeySpace:
		m.prompt = m.prompt.typed(m.prompt.input + " ")
	case tea.KeyRunes:
		m.prompt = m.prompt.typed(m.prompt.input + string(message.Runes))
	}
	return m, nil
}

// updateSearch narrows the list or dependency tree as the user types. Enter keeps the search,
// Esc clears it.
func (m Model) updateSearch(message tea.KeyMsg) (tea.Model, tea.Cmd) {
	input := m.prompt.input
	switch message.Type {
	case tea.KeyEsc:
		m.prompt = prompt{}
		m.applySearch("")
		return m, nil
	case tea.KeyEnter:
		m.prompt = prompt{}
		return m, nil
	case tea.KeyBackspace:
		if input == "" {
			m.prompt = prompt{}
			return m, nil
		}
		runes := []rune(input)
		input = string(runes[:len(runes)-1])
	case tea.KeySpace:
		input += " "
	case tea.KeyRunes:
		input += string(message.Runes)
	}
	m.prompt.input = input
	m.applySearch(input)
	return m, nil
}

// runFilter sets one filter field to the value chosen in the filter prompt.
// Empty input clears the field.
func (m *Model) runFilter(field, value string) {
	_, filter := m.searchAndFilter()
	filter, err := filter.With(field, value, m.filterKinds())
	if err != nil {
		m.err = err
		return
	}
	m.err = nil
	m.status = ""
	m.setFilter(filter)
}

// runFilterCommand handles ":health degraded" and its siblings, then points out
// the shortcut that does the same.
func (m *Model) runFilterCommand(command filterCommand, value string) {
	if !m.canFilterBy(command.name) {
		m.err = fmt.Errorf("cannot filter by %s here", command.name)
		return
	}
	m.runFilter(command.name, value)
	if m.err == nil {
		m.status = "Shortcut: press shift+" + command.shortcut + " to filter by " + command.name
	}
}

// runCommand opens the view named by the input, e.g. "app store" or "clusters".
func (m *Model) runCommand(input string) tea.Cmd {
	if input == "" {
		return nil
	}
	name, project, _ := strings.Cut(input, " ")
	if slices.Contains(quitCommands, name) {
		return tea.Quit
	}
	if slices.Contains(contextCommands, name) {
		if strings.TrimSpace(project) == "" {
			m.openContextPicker()
			return nil
		}
		return m.switchContext(strings.TrimSpace(project))
	}
	if slices.Contains(namespaceCommands, name) {
		if strings.TrimSpace(project) == "" {
			m.openNamespacePicker()
			return nil
		}
		return m.switchNamespace(strings.TrimSpace(project))
	}
	if filter, found := findFilterCommand(name); found {
		m.runFilterCommand(filter, strings.TrimSpace(project))
		return nil
	}

	command, found := findCommand(name)
	if !found {
		m.err = fmt.Errorf("command %q not found", name)
		return nil
	}
	if project != "" && !command.takesProject {
		m.err = fmt.Errorf("%s does not take a project", name)
		return nil
	}
	if project != "" && project != "all" && !m.hasProject(project) {
		m.err = fmt.Errorf("project %q not found", project)
		return nil
	}

	m.err = nil
	m.showScreen(command.screen)
	switch project {
	case "":
	case "all":
		m.explorer.SetProject("")
	default:
		m.explorer.SetProject(project)
	}
	return nil
}

// kubeconfigContexts lists the contexts the source can switch to, or none when
// it cannot switch. Reading them parses the kubeconfig, so callers keep the result.
func (m Model) kubeconfigContexts() []string {
	switcher, ok := m.source.(argocd.ContextSwitcher)
	if !ok {
		return nil
	}
	contexts, err := switcher.Contexts()
	if err != nil {
		return nil
	}
	return contexts
}

func (m *Model) openContextPicker() {
	switcher, ok := m.source.(argocd.ContextSwitcher)
	if !ok {
		m.err = fmt.Errorf(":ctx is available only with --source kubeconfig")
		return
	}
	contexts, err := switcher.Contexts()
	if err != nil {
		m.err = err
		return
	}
	m.contextOptions = contexts
	m.prompt = prompt{active: true, field: contextField}
}

func (m *Model) switchContext(context string) tea.Cmd {
	switcher, ok := m.source.(argocd.ContextSwitcher)
	if !ok {
		m.err = fmt.Errorf(":ctx is available only with --source kubeconfig")
		return nil
	}
	if context == "" {
		m.err = fmt.Errorf(":ctx requires a kubeconfig context")
		return nil
	}
	contexts, err := switcher.Contexts()
	if err != nil {
		m.err = err
		return nil
	}
	if !slices.Contains(contexts, context) {
		m.err = fmt.Errorf("kubeconfig context %q not found", context)
		return nil
	}
	m.source = switcher.WithContext(context)
	m.sourceGeneration++
	m.connection = "Kubeconfig context: " + context
	m.explorer = explorer.NewModel(explorer.Snapshot{})
	m.closeTree()
	m.marked = nil
	m.contextOptions = nil
	m.loading = true
	m.err = nil
	m.status = "Switching to kubeconfig context " + context
	return m.load()
}

// clusterNamespaces lists the cluster's namespaces, or an error when the
// source cannot switch namespaces or the identity may not list them.
func (m Model) clusterNamespaces(switcher argocd.NamespaceSwitcher) ([]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), m.requestTimeout)
	defer cancel()
	return switcher.Namespaces(ctx)
}

func (m *Model) openNamespacePicker() {
	switcher, ok := m.source.(argocd.NamespaceSwitcher)
	if !ok {
		m.err = fmt.Errorf(":ns is available only with --source kubeconfig")
		return
	}
	// Listing namespaces is a cluster-wide permission; without it the picker
	// is empty and a typed name still works.
	m.namespaceOptions, _ = m.clusterNamespaces(switcher)
	m.prompt = prompt{active: true, field: namespaceField}
}

func (m *Model) switchNamespace(namespace string) tea.Cmd {
	switcher, ok := m.source.(argocd.NamespaceSwitcher)
	if !ok {
		m.err = fmt.Errorf(":ns is available only with --source kubeconfig")
		return nil
	}
	if namespace == "" {
		m.err = fmt.Errorf(":ns requires a namespace")
		return nil
	}
	if namespaces, err := m.clusterNamespaces(switcher); err == nil && !slices.Contains(namespaces, namespace) {
		m.err = fmt.Errorf("namespace %q not found", namespace)
		return nil
	}
	m.source = switcher.WithNamespace(namespace)
	m.sourceGeneration++
	m.namespace = namespace
	m.explorer = explorer.NewModel(explorer.Snapshot{})
	m.closeTree()
	m.marked = nil
	m.namespaceOptions = nil
	m.loading = true
	m.err = nil
	m.status = "Switching to namespace " + namespace
	return m.load()
}

func (m Model) hasProject(name string) bool {
	return slices.ContainsFunc(m.explorer.Snapshot().Projects, func(project explorer.Project) bool {
		return project.Name == name
	})
}

// selectProject handles the number shortcuts: 0 shows every project and
// 1–9 narrow Applications to that project.
func (m *Model) selectProject(key string) {
	if key == "0" {
		m.showScreen(explorer.ApplicationsScreen)
		m.explorer.SetProject("")
		return
	}
	index := int(key[0] - '1')
	projects := m.explorer.Snapshot().Projects
	if index >= len(projects) {
		m.status = "Project " + key + " is not available"
		return
	}
	m.showScreen(explorer.ApplicationsScreen)
	m.explorer.SetProject(projects[index].Name)
}
