package tui

import (
	"context"
	"errors"
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

const sortCommand = "sort"

// sortFields are what the Applications list can be sorted by, as typed.
var sortFields = []string{"name", "health", "sync"}

const sortHelp = ":sort takes name, health or sync, and optionally reverse"

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
	// base is what was typed before Tab started filling in suggestions. While
	// it is set the suggestions come from it, so Tab and the arrows keep
	// moving through the same list instead of narrowing to the one chosen.
	base string
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
		if name == sortCommand {
			field, _, _ := strings.Cut(argument, " ")
			for _, candidate := range sortFields {
				if strings.HasPrefix(candidate, strings.ToLower(field)) {
					matches = append(matches, sortCommand+" "+candidate)
				}
			}
			if _, rest, hasRest := strings.Cut(argument, " "); hasRest && strings.HasPrefix("reverse", rest) {
				matches = append(matches, sortCommand+" "+strings.TrimSuffix(argument, rest)+"reverse")
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
		if strings.HasPrefix(sortCommand, input) {
			matches = append(matches, sortCommand)
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
	p.base = ""
	return p
}

// typedInput is the text suggestions are made from: what the user typed, even
// while Tab has filled in one of them.
func (p prompt) typedInput() string {
	if p.base != "" {
		return p.base
	}
	return p.input
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
	if name, context, hasContext := strings.Cut(m.prompt.typedInput(), " "); hasContext && slices.Contains(contextCommands, name) {
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
	return suggestions(m.prompt.typedInput(), projects, m.availableFilters())
}

// completeCommand fills the command bar with a suggestion. The first Tab takes
// the highlighted one; after that Tab and Down move on, Up and Shift+Tab go
// back, and all of them wrap around the same list.
func (m *Model) completeCommand(step int) {
	matches := m.suggestions()
	if len(matches) == 0 {
		return
	}
	index := m.prompt.selection % len(matches)
	if m.prompt.base != "" {
		index = (index + step + len(matches)) % len(matches)
	} else {
		m.prompt.base = m.prompt.input
	}
	m.prompt.selection = index
	m.prompt.input = matches[index]
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
		} else {
			m.completeCommand(1)
		}
	case tea.KeyDown:
		if m.prompt.field == "" && m.prompt.base != "" {
			m.completeCommand(1)
		} else {
			m.prompt.selection++
		}
	case tea.KeyUp, tea.KeyShiftTab:
		if m.prompt.field == "" && m.prompt.base != "" {
			m.completeCommand(-1)
		} else if count := len(m.suggestions()); count > 0 {
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
			return m.openNamespacePicker()
		}
		return m.switchNamespace(strings.TrimSpace(project))
	}
	if name == sortCommand {
		m.runSortCommand(strings.TrimSpace(project))
		return nil
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
	m.resetExplorer()
	m.closeTree()
	m.marked = nil
	m.contextOptions = nil
	m.loading = true
	m.err = nil
	m.status = "Switching to kubeconfig context " + context
	return m.load()
}

// runSortCommand orders the Applications list: ":sort health", ":sort name
// reverse", or ":sort" alone for the order the source returned.
func (m *Model) runSortCommand(arguments string) {
	words := strings.Fields(arguments)
	if len(words) > 2 || (len(words) == 2 && !strings.EqualFold(words[1], "reverse")) {
		m.err = errors.New(sortHelp)
		return
	}
	var input string
	if len(words) > 0 {
		input = words[0]
	}
	field, ok := explorer.ParseSortField(input)
	if !ok {
		m.err = errors.New(sortHelp)
		return
	}
	// An explicit "reverse" always reverses. Without it, sorting by the field
	// that is already in use flips the direction, and a new field starts in its
	// normal direction.
	reverse := len(words) == 2
	if current, currentReverse := m.explorer.Sort(); len(words) == 1 && field != explorer.SortDefault && current == field {
		reverse = !currentReverse
	}
	m.explorer.SetSort(field, reverse)
	m.err = nil
	if field == explorer.SortDefault {
		m.status = "Sorted in the order Argo CD returned"
		return
	}
	m.status = "Sorted by " + string(field)
	if reverse {
		m.status += " (reverse)"
	}
}

// resetExplorer empties the lists for another context or namespace, keeping
// how the Applications are sorted.
func (m *Model) resetExplorer() {
	field, reverse := m.explorer.Sort()
	m.explorer = explorer.NewModel(explorer.Snapshot{})
	m.explorer.SetSort(field, reverse)
}

// namespacesListed carries the cluster's namespaces, or the error from asking.
// switchTo is set when the lookup checks a namespace the user asked to switch to.
type namespacesListed struct {
	namespaces []string
	err        error
	switchTo   string
}

// listNamespaces asks the cluster for its namespaces in a command, so a slow
// or unreachable cluster never freezes the screen.
func (m Model) listNamespaces(switcher argocd.NamespaceSwitcher, switchTo string) tea.Cmd {
	timeout := m.requestTimeout
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		namespaces, err := switcher.Namespaces(ctx)
		return namespacesListed{namespaces: namespaces, err: err, switchTo: switchTo}
	}
}

func (m *Model) openNamespacePicker() tea.Cmd {
	switcher, ok := m.source.(argocd.NamespaceSwitcher)
	if !ok {
		m.err = fmt.Errorf(":ns is available only with --source kubeconfig")
		return nil
	}
	// Listing namespaces is a cluster-wide permission; without it the picker
	// stays empty and a typed name still works.
	m.namespaceOptions = nil
	m.prompt = prompt{active: true, field: namespaceField}
	return m.listNamespaces(switcher, "")
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
	m.err = nil
	m.status = "Switching to namespace " + namespace
	return m.listNamespaces(switcher, namespace)
}

// finishNamespaceLookup fills the picker, or finishes a switch once the namespace
// is known to exist. A cluster that cannot list namespaces is not an obstacle.
func (m *Model) finishNamespaceLookup(listed namespacesListed) tea.Cmd {
	if listed.switchTo == "" {
		if m.prompt.active && m.prompt.field == namespaceField {
			m.namespaceOptions = listed.namespaces
		}
		return nil
	}
	if listed.err == nil && !slices.Contains(listed.namespaces, listed.switchTo) {
		m.err = fmt.Errorf("namespace %q not found", listed.switchTo)
		m.status = ""
		return nil
	}
	switcher, ok := m.source.(argocd.NamespaceSwitcher)
	if !ok {
		return nil
	}
	m.source = switcher.WithNamespace(listed.switchTo)
	m.sourceGeneration++
	m.namespace = listed.switchTo
	m.resetExplorer()
	m.closeTree()
	m.marked = nil
	m.namespaceOptions = nil
	m.loading = true
	m.err = nil
	m.status = "Switching to namespace " + listed.switchTo
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
