package tui

import (
	"context"
	"errors"
	"time"

	"github.com/awcodify/argoxd/internal/argocd"
	"github.com/awcodify/argoxd/internal/explorer"
	tea "github.com/charmbracelet/bubbletea"
)

var (
	errNoOperations = errors.New("the active source does not support application operations")
	errNoInspection = errors.New("the active source cannot inspect resources")
)

type refreshTick struct{}

type loadedSnapshot struct {
	snapshot explorer.Snapshot
	err      error
}

type loadedTree struct {
	tree       explorer.ResourceTree
	background bool
	// child means the tree is of an Application the open one deploys.
	child bool
	err   error
}

type loadedText struct {
	kind    string
	subject string
	text    string
	// fromList means the text was opened from the Applications list, so Esc returns there.
	fromList bool
	err      error
}

type operationCompleted struct {
	action      string
	application string
	// subject names what the action was about when it is not the Application.
	subject string
	// refreshTree reloads the open dependency view too.
	refreshTree bool
	err         error
}

// request runs a source call off the UI loop and gives up after the request timeout.
func (m Model) request(call func(ctx context.Context) tea.Msg) tea.Cmd {
	timeout := m.requestTimeout
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		return call(ctx)
	}
}

func (m Model) load() tea.Cmd {
	return m.request(func(ctx context.Context) tea.Msg {
		snapshot, err := m.source.Load(ctx)
		return loadedSnapshot{snapshot: snapshot, err: err}
	})
}

func (m Model) loadApplicationTree(application string, background bool) tea.Cmd {
	operator, ok := m.source.(argocd.ApplicationOperator)
	if !ok {
		return nil
	}
	return m.request(func(ctx context.Context) tea.Msg {
		tree, err := operator.LoadResourceTree(ctx, application)
		return loadedTree{tree: tree, background: background, err: err}
	})
}

func (m Model) scheduleRefresh() tea.Cmd {
	if m.refreshInterval <= 0 {
		return nil
	}
	return tea.Tick(m.refreshInterval, func(time.Time) tea.Msg { return refreshTick{} })
}

// autoRefresh reloads the snapshot, and the open dependency tree, unless a
// load is already running.
func (m Model) autoRefresh() (tea.Model, tea.Cmd) {
	next := m.scheduleRefresh()
	if m.source == nil || m.loading || m.refreshing {
		return m, next
	}
	m.refreshing = true
	commands := []tea.Cmd{m.load(), next}
	if application := m.resourceTree.Application; application != "" {
		commands = append(commands, m.loadApplicationTree(application, true))
	}
	return m, tea.Batch(commands...)
}

func (m Model) openDependencies() (tea.Model, tea.Cmd) {
	application := m.explorer.SelectedName()
	if application == "" {
		return m, nil
	}
	if _, ok := m.source.(argocd.ApplicationOperator); !ok {
		m.err = errNoOperations
		return m, nil
	}
	m.loading = true
	return m, m.loadApplicationTree(application, false)
}

func (m Model) sync(application string, options argocd.SyncOptions) tea.Cmd {
	action := "sync"
	if options.DryRun {
		action = "dry-run sync"
	}
	return m.operate(action, application, func(ctx context.Context, operator argocd.ApplicationOperator) error {
		return operator.SyncApplication(ctx, application, options)
	})
}

func (m Model) hardRefresh(application string) tea.Cmd {
	return m.operate("hard refresh", application, func(ctx context.Context, operator argocd.ApplicationOperator) error {
		return operator.RefreshApplication(ctx, application)
	})
}

func (m Model) deleteApplication(application string) tea.Cmd {
	return m.operate("delete", application, func(ctx context.Context, operator argocd.ApplicationOperator) error {
		return operator.DeleteApplication(ctx, application)
	})
}

func (m Model) operate(action, application string, call func(context.Context, argocd.ApplicationOperator) error) tea.Cmd {
	operator, ok := m.source.(argocd.ApplicationOperator)
	if !ok {
		return func() tea.Msg {
			return operationCompleted{action: action, application: application, err: errNoOperations}
		}
	}
	return m.request(func(ctx context.Context) tea.Msg {
		return operationCompleted{action: action, application: application, err: call(ctx, operator)}
	})
}

// inspect opens the YAML (y), diff (d) or logs (l) of the selected card.
func (m Model) inspect(key string) (tea.Model, tea.Cmd) {
	selected := m.selectedCard()
	switch {
	case m.onRootCard():
		m.status = "Select a resource card to inspect it"
		return m, nil
	case key == "l" && !hasLogs(selected.Kind):
		m.status = "Logs are available for Pods and workloads (Deployment, StatefulSet, DaemonSet, ReplicaSet, Job)"
		return m, nil
	case key == "l" && selected.Kind != "Pod":
		return m.followLog(selected, nil)
	case key == "l":
		// A Pod shows its workload's log, filtered to that Pod, when it has one.
		if workload, found := m.workloadOf(selected); found && m.canStream() {
			return m.followLog(workload, &selected)
		}
	}
	inspector, ok := m.source.(argocd.ResourceInspector)
	if !ok {
		m.err = errNoInspection
		return m, nil
	}

	application := m.resourceTree.Application
	subject := selected.Kind + "/" + selected.Name
	m.logTarget = selected
	m.loading = true
	return m, m.request(func(ctx context.Context) tea.Msg {
		var text string
		var err error
		kind := "yaml"
		switch key {
		case "y":
			text, err = inspector.ResourceManifest(ctx, application, selected)
		case "d":
			kind = "diff"
			text, err = inspector.ResourceDiff(ctx, application, selected)
		case "l":
			kind = "logs"
			text, err = inspector.ResourceLogs(ctx, application, selected, "")
		}
		return loadedText{kind: kind, subject: subject, text: text, err: err}
	})
}
