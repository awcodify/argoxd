package tui

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/awcodify/argoxd/internal/argocd"
	tea "github.com/charmbracelet/bubbletea"
)

// loadedContainers is the containers of the resource whose logs are open.
type loadedContainers struct {
	subject    string
	containers argocd.Containers
	err        error
}

// loadedLogs is a reloaded snapshot of the open Pod log.
type loadedLogs struct {
	subject string
	text    string
	err     error
}

// resetContainers forgets the container chosen for the previous log.
func (m *Model) resetContainers() {
	m.logContainers = argocd.Containers{}
	m.containersKnown = false
	m.logContainer = ""
	m.logSeen = ""
}

// prefixesLines reports whether the open log mixes lines from several Pods or
// containers, so each line starts with where it came from.
func (m Model) prefixesLines() bool {
	return m.logTarget.Kind != "Pod" || m.logContainer == argocd.AllContainers
}

// linePrefix says where a followed line came from: its Pod for a workload, and
// its container when every container is shown.
func (m Model) linePrefix(entry argocd.LogEntry) string {
	prefix := ""
	if m.logTarget.Kind != "Pod" {
		prefix = entry.Pod
	}
	if m.logContainer == argocd.AllContainers && entry.Container != "" {
		if prefix != "" {
			prefix += "/"
		}
		prefix += entry.Container
	}
	return prefix
}

// containerLabel names the container being shown, once it is known.
func (m Model) containerLabel() string {
	switch {
	case m.logContainer == argocd.AllContainers:
		return "all containers"
	case m.logContainer != "":
		return m.logContainer
	}
	return m.logSeen
}

// containerField is the prompt field that chooses a container, like the health,
// sync and kind filters.
const containerField = "container"

// allContainersOption is how the bar offers every container at once.
const allContainersOption = "all"

// containerOptions are the choices of the container bar: each container, then all.
func containerOptions(containers argocd.Containers) []string {
	return append(slices.Clone(containers.Names), allContainersOption)
}

// containerSuggestions lists the containers that contain the input, those that
// start with it first. Container names often share a long prefix, such as
// "prometheus-server-configmap-reload", so any part of a name matches.
func containerSuggestions(input string, containers argocd.Containers) []string {
	input = strings.ToLower(input)
	var prefixed, contained []string
	for _, option := range containerOptions(containers) {
		switch lower := strings.ToLower(option); {
		case strings.HasPrefix(lower, input):
			prefixed = append(prefixed, option)
		case strings.Contains(lower, input):
			contained = append(contained, option)
		}
	}
	return append(prefixed, contained...)
}

// chooseContainer opens the container bar. The containers are read from the
// manifest the first time.
func (m Model) chooseContainer() (tea.Model, tea.Cmd) {
	if m.containersKnown {
		return m.openContainerBar()
	}
	inspector, ok := m.source.(argocd.ResourceInspector)
	if !ok {
		m.err = errNoInspection
		return m, nil
	}
	application, target, subject := m.resourceTree.Application, m.logTarget, m.viewer.subject
	return m, m.request(func(ctx context.Context) tea.Msg {
		containers, err := argocd.LogContainers(ctx, inspector, application, target)
		return loadedContainers{subject: subject, containers: containers, err: err}
	})
}

func (m Model) applyContainers(message loadedContainers) (tea.Model, tea.Cmd) {
	if m.view != textViewMode || m.viewer.kind != "logs" || message.subject != m.viewer.subject {
		return m, nil
	}
	if message.err != nil {
		m.err = message.err
		return m, nil
	}
	m.logContainers, m.containersKnown = message.containers, true
	return m.openContainerBar()
}

// openContainerBar asks which container to show, starting on the current one so
// that Tab moves on to the next.
func (m Model) openContainerBar() (tea.Model, tea.Cmd) {
	if len(m.logContainers.Names) < 2 {
		m.status = "This resource has a single container"
		return m, nil
	}
	current := m.logContainer
	switch current {
	case "":
		current = m.logContainers.Default
	case argocd.AllContainers:
		current = allContainersOption
	}
	m.prompt = prompt{active: true, field: containerField, selection: max(0, slices.Index(containerOptions(m.logContainers), current))}
	return m, nil
}

// showContainer shows the container chosen in the bar, or all of them.
func (m Model) showContainer(choice string) (tea.Model, tea.Cmd) {
	switch {
	case choice == "":
		return m, nil
	case strings.EqualFold(choice, allContainersOption):
		choice = argocd.AllContainers
	case !slices.Contains(m.logContainers.Names, choice):
		m.err = fmt.Errorf("container %q not found", choice)
		return m, nil
	}

	shown := m.logContainer
	if shown == "" {
		shown = m.logContainers.Default
	}
	m.err, m.status = nil, ""
	if choice == shown {
		m.logContainer = choice
		return m, nil
	}
	m.logContainer = choice
	m.logSeen = ""
	return m.reloadLogs()
}

// reloadLogs shows the log of the chosen container: a followed log, and the log
// of a workload, which is only ever followed, starts a new stream; a Pod's
// snapshot is read again.
func (m Model) reloadLogs() (tea.Model, tea.Cmd) {
	if m.following() || m.logTarget.Kind != "Pod" {
		m.stopFollowing()
		return m.toggleFollow()
	}
	inspector, ok := m.source.(argocd.ResourceInspector)
	if !ok {
		m.err = errNoInspection
		return m, nil
	}
	application, target, subject, container := m.resourceTree.Application, m.logTarget, m.viewer.subject, m.logContainer
	m.loading = true
	return m, m.request(func(ctx context.Context) tea.Msg {
		text, err := inspector.ResourceLogs(ctx, application, target, container)
		return loadedLogs{subject: subject, text: text, err: err}
	})
}

func (m Model) applyLogs(message loadedLogs) (tea.Model, tea.Cmd) {
	m.loading = false
	if m.view != textViewMode || m.viewer.kind != "logs" || message.subject != m.viewer.subject {
		return m, nil
	}
	if message.err != nil {
		m.err = message.err
		return m, nil
	}
	m.viewer = newTextView("logs", message.subject, message.text)
	m.viewer.prefixed = m.prefixesLines()
	return m, nil
}
