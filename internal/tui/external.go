package tui

import (
	"fmt"
	"os/exec"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// ExternalTools are the programs argoxd can hand the terminal to.
type ExternalTools struct {
	// K9s is the k9s command; empty means "k9s".
	K9s string
	// KubeContext is the kubeconfig context k9s opens on; empty uses the current one.
	KubeContext string
}

// WithExternalTools sets the programs started by :k9s.
func (m Model) WithExternalTools(tools ExternalTools) Model {
	m.tools = tools
	return m
}

// launch starts a program in the terminal and returns to argoxd when it ends.
// Tests replace it so no real program starts.
func defaultLaunch(cmd *exec.Cmd, done tea.ExecCallback) tea.Cmd {
	return tea.ExecProcess(cmd, done)
}

// externalDone reports a finished external program.
type externalDone struct {
	program string
	err     error
}

const k9sCommandName = "k9s"

// openK9s runs k9s on the kubeconfig context, opened on the selected card's
// namespace and kind when the dependency view has one selected. k9s can change
// resources, so read-only mode does not start it.
func (m *Model) openK9s() tea.Cmd {
	if m.blockReadOnly() {
		return nil
	}
	program := m.tools.K9s
	if program == "" {
		program = k9sCommandName
	}
	path, err := exec.LookPath(program)
	if err != nil {
		m.err = fmt.Errorf("%s was not found in your PATH; install it from https://k9scli.io", program)
		return nil
	}
	args := []string{}
	if m.tools.KubeContext != "" {
		args = append(args, "--context", m.tools.KubeContext)
	}
	if m.view == applicationTreeView && !m.onRootCard() {
		card := m.selectedCard()
		if card.Namespace != "" {
			args = append(args, "-n", card.Namespace)
		}
		plural, _ := meta.UnsafeGuessKindToResource(schema.GroupVersionKind{Group: card.Group, Version: card.Version, Kind: card.Kind})
		args = append(args, "-c", strings.ToLower(plural.Resource))
	}
	launch := m.launch
	if launch == nil {
		launch = defaultLaunch
	}
	m.err = nil
	m.status = "Opened k9s; quit it to come back"
	return launch(exec.Command(path, args...), func(err error) tea.Msg {
		return externalDone{program: program, err: err}
	})
}
