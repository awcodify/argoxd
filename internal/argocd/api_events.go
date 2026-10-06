package argocd

import (
	"context"
	"fmt"
	"net/url"
	"time"

	"github.com/awcodify/argoxd/internal/explorer"
	corev1 "k8s.io/api/core/v1"
)

var _ EventLister = (*APISource)(nil)

// ApplicationEvents returns the events Argo CD recorded for the Application.
func (s *APISource) ApplicationEvents(ctx context.Context, application string) (string, error) {
	events, err := s.events(ctx, application, nil)
	if err != nil {
		return "", err
	}
	return formatEvents(events, time.Now()), nil
}

// ResourceEvents returns the events of a resource and of the resources under
// it. Argo CD looks a resource's events up by its UID, one request each.
func (s *APISource) ResourceEvents(ctx context.Context, application string, resource explorer.ResourceNode) (string, error) {
	var events []corev1.Event
	for _, node := range withDescendants(resource) {
		found, err := s.events(ctx, application, url.Values{
			"resourceName":      {node.Name},
			"resourceNamespace": {node.Namespace},
			"resourceUID":       {node.UID},
		})
		if err != nil {
			return "", err
		}
		events = append(events, found...)
	}
	return formatEvents(events, time.Now()), nil
}

func (s *APISource) events(ctx context.Context, application string, query url.Values) ([]corev1.Event, error) {
	var list corev1.EventList
	if err := s.get(ctx, applicationPath(application, "events", query), &list); err != nil {
		return nil, fmt.Errorf("load events: %w", err)
	}
	return list.Items, nil
}
