package tui

import (
	"fmt"
	"slices"
	"strings"

	"github.com/awcodify/argoxd/internal/explorer"
	tea "github.com/charmbracelet/bubbletea"
)

// podField is the prompt field that chooses a Pod of a workload's log.
const podField = "pod"

// allPodsOption is how the bar offers the whole workload again.
const allPodsOption = "all"

// podOptions are the choices of the pod bar: each Pod, then all.
func podOptions(pods []string) []string {
	return append(slices.Clone(pods), allPodsOption)
}

// workloadPods names the Pods under the workload whose log is open, as the
// resource tree currently shows them.
func (m Model) workloadPods() []string {
	var pods []string
	var walk func(nodes []explorer.ResourceNode, inside bool)
	walk = func(nodes []explorer.ResourceNode, inside bool) {
		for _, node := range nodes {
			if inside && node.Kind == "Pod" {
				pods = append(pods, node.Name)
			}
			walk(node.Children, inside || node.Reference() == m.logWorkload.Reference())
		}
	}
	walk(m.resourceTree.Hierarchy(), false)
	slices.Sort(pods)
	return slices.Compact(pods)
}

// openPodBar asks which Pod of the workload to show, starting on the current
// choice so that Tab moves on to the next.
func (m Model) openPodBar() (tea.Model, tea.Cmd) {
	if m.logWorkload.Kind == "" {
		m.status = "The pod filter is for the logs of a workload, not of a single Pod"
		return m, nil
	}
	pods := m.workloadPods()
	if len(pods) == 0 {
		m.status = "No pods found for " + m.logWorkload.Kind + "/" + m.logWorkload.Name
		return m, nil
	}
	m.logPods = pods
	current := m.logPod
	if current == "" {
		current = allPodsOption
	}
	m.prompt = prompt{active: true, field: podField, selection: max(0, slices.Index(podOptions(pods), current))}
	return m, nil
}

// showPod follows only the chosen Pod of the workload, or all of them again.
func (m Model) showPod(choice string) (tea.Model, tea.Cmd) {
	switch {
	case choice == "":
		return m, nil
	case strings.EqualFold(choice, allPodsOption):
		choice = ""
	case !slices.Contains(m.logPods, choice):
		m.err = fmt.Errorf("pod %q not found", choice)
		return m, nil
	}
	if choice == m.logPod {
		return m, nil
	}

	m.logPod = choice
	m.logSeen = ""
	m.err, m.status = nil, ""
	m.logTarget = m.logWorkload
	if choice != "" {
		m.logTarget = explorer.ResourceNode{Version: "v1", Kind: "Pod", Namespace: m.logWorkload.Namespace, Name: choice}
	}
	return m.reloadLogs()
}
