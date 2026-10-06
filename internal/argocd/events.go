package argocd

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/awcodify/argoxd/internal/explorer"
	corev1 "k8s.io/api/core/v1"
)

// maxEventObjects bounds how many resources a workload's events are gathered
// from: the workload itself and the resources nested under it.
const maxEventObjects = 30

// EventLister shows the Kubernetes events of an Application and its resources.
type EventLister interface {
	// ApplicationEvents returns the events of the Application itself as a
	// table, oldest first.
	ApplicationEvents(ctx context.Context, application string) (string, error)
	// ResourceEvents returns the events of a resource and of the resources
	// nested under it (a Deployment's ReplicaSets and Pods), oldest first.
	ResourceEvents(ctx context.Context, application string, resource explorer.ResourceNode) (string, error)
}

// withDescendants returns the resource followed by the resources nested under
// it, depth first, up to maxEventObjects.
func withDescendants(resource explorer.ResourceNode) []explorer.ResourceNode {
	var nodes []explorer.ResourceNode
	var walk func(explorer.ResourceNode)
	walk = func(node explorer.ResourceNode) {
		if len(nodes) >= maxEventObjects {
			return
		}
		nodes = append(nodes, node)
		for _, child := range node.Children {
			walk(child)
		}
	}
	walk(resource)
	return nodes
}

// eventTime is when an event last happened.
func eventTime(event corev1.Event) time.Time {
	switch {
	case !event.LastTimestamp.IsZero():
		return event.LastTimestamp.Time
	case !event.EventTime.IsZero():
		return event.EventTime.Time
	}
	return event.FirstTimestamp.Time
}

// formatEvents renders events as a table, the oldest first, like `kubectl get
// events` with the age of each. It returns nothing when there are none.
func formatEvents(events []corev1.Event, now time.Time) string {
	if len(events) == 0 {
		return ""
	}
	events = slices.Clone(events)
	slices.SortStableFunc(events, func(a, b corev1.Event) int { return eventTime(a).Compare(eventTime(b)) })

	rows := [][]string{{"AGE", "TYPE", "REASON", "OBJECT", "MESSAGE"}}
	for _, event := range events {
		message := strings.ReplaceAll(strings.TrimSpace(event.Message), "\n", " ")
		if event.Count > 1 {
			message += fmt.Sprintf(" (x%d)", event.Count)
		}
		rows = append(rows, []string{
			age(now.Sub(eventTime(event))), event.Type, event.Reason,
			event.InvolvedObject.Kind + "/" + event.InvolvedObject.Name, message,
		})
	}

	widths := make([]int, len(rows[0])-1)
	for _, row := range rows {
		for column := range widths {
			widths[column] = max(widths[column], len(row[column]))
		}
	}
	lines := make([]string, 0, len(rows))
	for _, row := range rows {
		var line strings.Builder
		for column, width := range widths {
			line.WriteString(fmt.Sprintf("%-*s  ", width, row[column]))
		}
		line.WriteString(row[len(widths)])
		lines = append(lines, line.String())
	}
	return strings.Join(lines, "\n")
}

// age shortens a duration the way kubectl does: 45s, 2m, 3h, 5d.
func age(duration time.Duration) string {
	switch {
	case duration < time.Minute:
		return fmt.Sprintf("%ds", max(0, int(duration.Seconds())))
	case duration < time.Hour:
		return fmt.Sprintf("%dm", int(duration.Minutes()))
	case duration < 24*time.Hour:
		return fmt.Sprintf("%dh", int(duration.Hours()))
	}
	return fmt.Sprintf("%dd", int(duration.Hours()/24))
}
