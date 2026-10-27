package tui

import (
	"fmt"
	"os"
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
	// Kubeconfig is the kubeconfig file argoxd reads; empty uses k9s's default.
	Kubeconfig string
	// KubeContext is the kubeconfig context k9s opens on; empty uses the current one.
	KubeContext string
	// DiffViewer is a command that compares two files, written with {left}
	// and {right} for them, e.g. "meld {left} {right}".
	DiffViewer string
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

// openK9s runs k9s on argoxd's kubeconfig and context, opened on the selected
// card's namespace and kind when the dependency view has one selected. k9s can
// change resources, so read-only mode does not start it.
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
	if m.tools.Kubeconfig != "" {
		args = append(args, "--kubeconfig", m.tools.Kubeconfig)
	}
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

// splitDiff rebuilds the two sides of a diff: the live text from the lines
// kept or removed, and the desired text from the lines kept or added.
// Headers that name a resource, such as "## Deployment/web", stay on both.
func splitDiff(diff string) (live, desired string) {
	var left, right strings.Builder
	for _, line := range strings.Split(diff, "\n") {
		switch {
		case strings.HasPrefix(line, "- "):
			left.WriteString(line[2:] + "\n")
		case strings.HasPrefix(line, "+ "):
			right.WriteString(line[2:] + "\n")
		case strings.HasPrefix(line, "  "):
			left.WriteString(line[2:] + "\n")
			right.WriteString(line[2:] + "\n")
		default:
			left.WriteString(line + "\n")
			right.WriteString(line + "\n")
		}
	}
	return left.String(), right.String()
}

// openDiffViewer shows the open diff in the configured external program, which
// gets the live and the desired text as two temporary files.
func (m *Model) openDiffViewer() tea.Cmd {
	if m.viewer.kind != "diff" {
		m.status = "x opens a diff in your diff viewer; open a diff with d first"
		return nil
	}
	template := strings.Fields(m.tools.DiffViewer)
	if len(template) == 0 {
		m.status = `Set diffViewer in the config file to open diffs elsewhere, e.g. diffViewer: "meld {left} {right}"`
		return nil
	}
	if !strings.Contains(m.tools.DiffViewer, "{left}") || !strings.Contains(m.tools.DiffViewer, "{right}") {
		m.err = fmt.Errorf("diffViewer must contain both {left} and {right}, got %q", m.tools.DiffViewer)
		return nil
	}
	program, err := exec.LookPath(template[0])
	if err != nil {
		m.err = fmt.Errorf("%s was not found in your PATH (diffViewer)", template[0])
		return nil
	}
	live, desired := splitDiff(strings.Join(m.viewer.lines, "\n"))
	dir, err := os.MkdirTemp("", "argoxd-diff-")
	if err != nil {
		m.err = err
		return nil
	}
	files := map[string]string{"{left}": dir + "/live.yaml", "{right}": dir + "/desired.yaml"}
	for placeholder, text := range map[string]string{"{left}": live, "{right}": desired} {
		if err := os.WriteFile(files[placeholder], []byte(text), 0o600); err != nil {
			_ = os.RemoveAll(dir)
			m.err = err
			return nil
		}
	}
	args := make([]string, 0, len(template)-1)
	for _, word := range template[1:] {
		for placeholder, path := range files {
			word = strings.ReplaceAll(word, placeholder, path)
		}
		args = append(args, word)
	}
	launch := m.launch
	if launch == nil {
		launch = defaultLaunch
	}
	m.err, m.status = nil, "Opened the diff in "+template[0]
	return launch(exec.Command(program, args...), func(err error) tea.Msg {
		_ = os.RemoveAll(dir)
		return externalDone{program: template[0], err: err}
	})
}
