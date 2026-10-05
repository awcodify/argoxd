package tui

import (
	"fmt"
	"slices"
	"strings"

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
	{names: []string{"settings"}, screen: explorer.SettingsScreen},
}

var quitCommands = []string{"q", "quit"}

// prompt is the command line opened with ':'.
type prompt struct {
	active    bool
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

// suggestions completes the input: command names first, then the project
// argument of commands that accept one, such as "app store".
func suggestions(input string, projects []string) []string {
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
	}
	slices.Sort(matches)
	return matches
}

func (m Model) suggestions() []string {
	projects := make([]string, 0, len(m.explorer.Snapshot().Projects))
	for _, project := range m.explorer.Snapshot().Projects {
		projects = append(projects, project.Name)
	}
	return suggestions(m.prompt.input, projects)
}

func (m Model) selectedSuggestion() string {
	if matches := m.suggestions(); len(matches) > 0 {
		return matches[m.prompt.selection%len(matches)]
	}
	return ""
}

func (m Model) updatePrompt(message tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch message.Type {
	case tea.KeyEsc:
		m.prompt = prompt{}
	case tea.KeyEnter:
		input := strings.TrimSpace(m.prompt.input)
		m.prompt = prompt{}
		return m, m.runCommand(input)
	case tea.KeyTab, tea.KeyRight:
		if suggestion := m.selectedSuggestion(); suggestion != "" {
			m.prompt = prompt{active: true, input: suggestion}
		}
	case tea.KeyDown:
		m.prompt.selection++
	case tea.KeyUp:
		if count := len(m.suggestions()); count > 0 {
			m.prompt.selection = (m.prompt.selection + count - 1) % count
		}
	case tea.KeyBackspace:
		if m.prompt.input == "" {
			m.prompt = prompt{}
			break
		}
		runes := []rune(m.prompt.input)
		m.prompt = prompt{active: true, input: string(runes[:len(runes)-1])}
	case tea.KeySpace:
		m.prompt = prompt{active: true, input: m.prompt.input + " "}
	case tea.KeyRunes:
		m.prompt = prompt{active: true, input: m.prompt.input + string(message.Runes)}
	}
	return m, nil
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
