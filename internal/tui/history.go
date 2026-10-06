package tui

import (
	"context"
	"errors"
	"strconv"

	"github.com/awcodify/argoxd/internal/argocd"
	"github.com/awcodify/argoxd/internal/explorer"
	tea "github.com/charmbracelet/bubbletea"
)

var errNoHistory = errors.New("the active source cannot show or roll back deployment history")

type loadedHistory struct {
	application string
	entries     []explorer.HistoryEntry
	err         error
}

// historyList is an Application's deployments, newest first, with the view to
// return to when it closes.
type historyList struct {
	application string
	entries     []explorer.HistoryEntry
	cursor      int
	from        viewMode
}

// rollbackDialog asks how to roll back before redeploying a past deployment.
type rollbackDialog struct {
	application string
	entry       explorer.HistoryEntry
	options     argocd.SyncOptions
}

// openHistory loads the deployments of an Application and shows them.
func (m Model) openHistory(application string) (tea.Model, tea.Cmd) {
	operator, ok := m.source.(argocd.RollbackOperator)
	if !ok {
		m.err = errNoHistory
		return m, nil
	}
	m.loading = true
	return m, m.request(func(ctx context.Context) tea.Msg {
		entries, err := operator.ApplicationHistory(ctx, application)
		return loadedHistory{application: application, entries: entries, err: err}
	})
}

func (m *Model) applyHistory(message loadedHistory) {
	m.err = message.err
	if message.err != nil {
		return
	}
	m.history = historyList{application: message.application, entries: message.entries, from: m.view}
	m.view = historyViewMode
}

func (m Model) updateHistory(message tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch message.String() {
	case "ctrl+c", "q":
		return m, tea.Quit
	case "up", "k":
		m.history.cursor = max(0, m.history.cursor-1)
	case "down", "j":
		m.history.cursor = min(len(m.history.entries)-1, m.history.cursor+1)
	case "enter":
		if m.history.cursor < len(m.history.entries) {
			m.rollingBack = rollbackDialog{application: m.history.application, entry: m.history.entries[m.history.cursor]}
		}
	case "esc":
		m.view = m.history.from
		m.history = historyList{}
	}
	return m, nil
}

func (m Model) updateRollbackDialog(message tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch message.String() {
	case "p":
		m.rollingBack.options.Prune = !m.rollingBack.options.Prune
	case "r":
		m.rollingBack.options.DryRun = !m.rollingBack.options.DryRun
	case "enter":
		dialog := m.rollingBack
		m.rollingBack = rollbackDialog{}
		m.loading = true
		return m, m.rollback(dialog)
	case "esc", "n":
		m.rollingBack = rollbackDialog{}
	}
	return m, nil
}

func (m Model) rollback(dialog rollbackDialog) tea.Cmd {
	action := "rollback"
	if dialog.options.DryRun {
		action = "dry-run rollback"
	}
	operator, ok := m.source.(argocd.RollbackOperator)
	if !ok {
		return func() tea.Msg {
			return operationCompleted{action: action, application: dialog.application, err: errNoHistory}
		}
	}
	return m.request(func(ctx context.Context) tea.Msg {
		err := operator.RollbackApplication(ctx, dialog.application, dialog.entry.ID, dialog.options)
		return operationCompleted{action: action, application: dialog.application, err: err}
	})
}

func (m Model) historyTable() table {
	result := table{columns: []string{"ID", "REVISION", "DEPLOYED", "STATE", "REPO"}}
	for index, entry := range m.history.entries {
		state := ""
		if index == 0 {
			state = "current"
		}
		result.rows = append(result.rows, []string{
			strconv.FormatInt(entry.ID, 10), shortRevision(entry.Revision), age(entry.DeployedAt), state, entry.Repo,
		})
	}
	return result
}

// revisionLabel names a deployment in a prompt: its revision, or else its id.
func revisionLabel(entry explorer.HistoryEntry) string {
	if entry.Revision == "" {
		return "deployment #" + strconv.FormatInt(entry.ID, 10)
	}
	return shortRevision(entry.Revision)
}

// shortRevision abbreviates a full commit SHA to seven characters and leaves
// branch names, tags and short SHAs alone.
func shortRevision(revision string) string {
	if len(revision) != 40 {
		return revision
	}
	for _, character := range revision {
		if (character < '0' || character > '9') && (character < 'a' || character > 'f') {
			return revision
		}
	}
	return revision[:7]
}
