package tui

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/awcodify/argoxd/internal/argocd"
	"github.com/awcodify/argoxd/internal/explorer"
	tea "github.com/charmbracelet/bubbletea"
)

// confirmKind is the action a confirmation asks about.
type confirmKind int

const (
	noConfirmation confirmKind = iota
	confirmSync
	confirmHardRefresh
	confirmDeleteApplications
	confirmRestart
	confirmDeleteResources
	confirmRollback
)

// confirmation asks before an action runs, listing everything it will touch.
// It names Applications, or resources of one Application, or a deployment to
// roll back to.
type confirmation struct {
	kind confirmKind
	// application is the one Application acted on, or the owner of resources.
	application string
	// applications are several Applications; application is empty then.
	applications []string
	resources    []explorer.ResourceNode
	entry        explorer.HistoryEntry
	options      argocd.SyncOptions
}

// target is one line of the list in a confirmation.
type target struct{ label, detail string }

func (c confirmation) active() bool { return c.kind != noConfirmation }

// usesOptions reports whether prune and dry run apply.
func (c confirmation) usesOptions() bool { return c.kind == confirmSync || c.kind == confirmRollback }

// destructive actions are drawn in the error color.
func (c confirmation) destructive() bool {
	return c.kind == confirmDeleteApplications || c.kind == confirmDeleteResources
}

func count(n int, noun string) string {
	if n == 1 {
		return "1 " + noun
	}
	return strconv.Itoa(n) + " " + noun + "s"
}

// subject names what the action is about: the Application, "3 applications",
// or "Deployment/web".
func (c confirmation) subject() string {
	switch {
	case len(c.applications) > 0:
		return count(len(c.applications), "application")
	case len(c.resources) > 1:
		return count(len(c.resources), "resource")
	case len(c.resources) == 1 && c.kind != confirmSync:
		return resourceName(c.resources[0])
	}
	return c.application
}

func resourceName(node explorer.ResourceNode) string { return node.Kind + "/" + node.Name }

// question is the headline, e.g. "Sync 2 applications?".
func (c confirmation) question() string {
	switch c.kind {
	case confirmSync:
		if len(c.resources) > 0 {
			return "Sync " + count(len(c.resources), "resource") + " of " + c.application + "?"
		}
		return "Sync " + c.subject() + "?"
	case confirmHardRefresh:
		return "Hard refresh " + c.subject() + "?"
	case confirmDeleteApplications:
		if len(c.applications) > 0 {
			return "Delete " + c.subject() + " and their managed resources?"
		}
		return "Delete " + c.subject() + " and its managed resources?"
	case confirmRestart:
		if len(c.resources) > 1 {
			return "Restart " + count(len(c.resources), "workload") + "?"
		}
		return "Restart " + c.subject() + "?"
	case confirmDeleteResources:
		if len(c.resources) > 1 {
			return "Delete " + count(len(c.resources), "pod") + "?"
		}
		return "Delete " + c.subject() + "?"
	case confirmRollback:
		return "Roll back " + c.application + " to " + revisionLabel(c.entry) + "?"
	}
	return ""
}

func (c confirmation) glyph() string {
	switch c.kind {
	case confirmRestart:
		return "↻"
	case confirmRollback:
		return "↩"
	case confirmDeleteApplications, confirmDeleteResources:
		return "✗"
	}
	return "⟳"
}

// targets lists what the action will touch.
func (m Model) targets(c confirmation) []target {
	var targets []target
	names := c.applications
	if len(names) == 0 && len(c.resources) == 0 && c.kind != confirmRollback {
		names = []string{c.application}
	}
	for _, name := range names {
		application := m.applicationNamed(name)
		targets = append(targets, target{name, joinNonEmpty(application.Sync, application.Health)})
	}
	for _, resource := range c.resources {
		targets = append(targets, target{resourceName(resource), resource.Namespace})
	}
	if c.kind == confirmRollback {
		targets = append(targets, target{revisionLabel(c.entry), "deployment #" + strconv.FormatInt(c.entry.ID, 10)})
	}
	return targets
}

// applicationNamed returns the Application from the last snapshot, or a bare one.
func (m Model) applicationNamed(name string) explorer.Application {
	for _, application := range m.explorer.Snapshot().Applications {
		if application.Name == name {
			return application
		}
	}
	return explorer.Application{Name: name}
}

// confirmationKeys are the keys the dialog answers to.
func (c confirmation) keys() string {
	confirm := "confirm"
	switch c.kind {
	case confirmSync:
		confirm = "sync"
	case confirmRollback:
		confirm = "roll back"
	}
	line := keycap("enter", confirm)
	if c.usesOptions() {
		line += "  " + keycap("p", "prune "+toggle(c.options.Prune)) + "  " + keycap("r", "dry run "+toggle(c.options.DryRun))
	}
	return line + "  " + keycap("esc", "cancel")
}

// renderConfirmation draws the dialog in place of the view. The list of
// targets is cut to fit, so the keys always stay on screen.
func (m Model) renderConfirmation(width, height int) []string {
	c := m.confirm
	headline := accentStyle
	if c.destructive() {
		headline = errorStyle
	}
	targets := m.targets(c)
	room := max(1, height-5)
	hidden := 0
	if len(targets) > room {
		hidden = len(targets) - (room - 1)
		targets = targets[:room-1]
	}

	lines := []string{"", " " + headline.Render(c.glyph()+" "+c.question()), ""}
	labelWidth := 0
	for _, t := range targets {
		labelWidth = max(labelWidth, len(t.label))
	}
	for _, t := range targets {
		lines = append(lines, "   "+mutedStyle.Render("•")+" "+brightStyle.Render(padRight(t.label, labelWidth))+"  "+mutedStyle.Render(t.detail))
	}
	if hidden > 0 {
		lines = append(lines, "   "+mutedStyle.Render(fmt.Sprintf("… and %d more", hidden)))
	}
	return append(lines, "", " "+c.keys())
}

// updateConfirm answers the dialog: y or enter confirms, esc or n cancels,
// and p and r toggle prune and dry run where they apply.
func (m Model) updateConfirm(message tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch message.String() {
	case "p":
		if m.confirm.usesOptions() {
			m.confirm.options.Prune = !m.confirm.options.Prune
		}
	case "r":
		if m.confirm.usesOptions() {
			m.confirm.options.DryRun = !m.confirm.options.DryRun
		}
	case "y", "enter":
		confirmed := m.confirm
		m.confirm = confirmation{}
		if confirmed.kind != confirmRollback {
			m.marked, m.markedCards, m.status = nil, nil, ""
		}
		m.loading = true
		return m, m.perform(confirmed)
	case "esc", "n":
		m.confirm = confirmation{}
	}
	return m, nil
}

// forApplications builds a confirmation for one or several Applications.
func forApplications(kind confirmKind, names []string) confirmation {
	if len(names) == 1 {
		return confirmation{kind: kind, application: names[0]}
	}
	return confirmation{kind: kind, applications: names}
}

// perform starts the confirmed action.
func (m Model) perform(c confirmation) tea.Cmd {
	switch c.kind {
	case confirmSync:
		switch {
		case len(c.resources) > 0:
			return m.syncResources(c.application, c.resources, c.options)
		case len(c.applications) > 0:
			return m.eachApplication(syncAction(c.options), c.applications, func(ctx context.Context, operator argocd.ApplicationOperator, name string) error {
				return operator.SyncApplication(ctx, name, c.options)
			})
		}
		return m.sync(c.application, c.options)
	case confirmHardRefresh:
		if len(c.applications) > 0 {
			return m.eachApplication("hard refresh", c.applications, func(ctx context.Context, operator argocd.ApplicationOperator, name string) error {
				return operator.RefreshApplication(ctx, name)
			})
		}
		return m.hardRefresh(c.application)
	case confirmDeleteApplications:
		if len(c.applications) > 0 {
			return m.eachApplication("delete", c.applications, func(ctx context.Context, operator argocd.ApplicationOperator, name string) error {
				return operator.DeleteApplication(ctx, name)
			})
		}
		return m.deleteApplication(c.application)
	case confirmRestart, confirmDeleteResources:
		return m.actOnResources(c)
	case confirmRollback:
		return m.rollback(c)
	}
	return nil
}

// eachApplication runs a call for each Application in turn, each within its own
// timeout, and reports the ones that failed together.
func (m Model) eachApplication(action string, names []string, call func(context.Context, argocd.ApplicationOperator, string) error) tea.Cmd {
	operator, ok := m.source.(argocd.ApplicationOperator)
	if !ok {
		return func() tea.Msg { return operationCompleted{action: action, err: errNoOperations} }
	}
	return m.each(action, count(len(names), "application"), "", names, false, func(ctx context.Context, index int) error {
		return call(ctx, operator, names[index])
	})
}

// each runs call for every label in turn and folds the outcome into one
// operationCompleted: an error that lists the failures, or success.
func (m Model) each(action, subject, application string, labels []string, refreshTree bool, call func(ctx context.Context, index int) error) tea.Cmd {
	timeout := m.requestTimeout
	return func() tea.Msg {
		var failures []string
		for index, label := range labels {
			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			err := call(ctx, index)
			cancel()
			if err != nil {
				failures = append(failures, label+": "+err.Error())
			}
		}
		completed := operationCompleted{action: action, application: application, subject: subject, refreshTree: refreshTree}
		if len(failures) > 0 {
			completed.err = fmt.Errorf("%s failed for %d of %d (%s)", action, len(failures), len(labels), strings.Join(failures, "; "))
		}
		return completed
	}
}
