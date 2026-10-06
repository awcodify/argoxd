package argocd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"github.com/awcodify/argoxd/internal/explorer"
)

var _ ResourceActor = (*APISource)(nil)

// RestartResource runs Argo CD's restart action on a Deployment, StatefulSet or
// DaemonSet. Argo CD takes the action's name as the whole request body and the
// resource in the query.
func (s *APISource) RestartResource(ctx context.Context, application string, resource explorer.ResourceNode) error {
	if !Restartable(resource.Kind) {
		return fmt.Errorf("%s/%s cannot be restarted: only Deployments, StatefulSets and DaemonSets can", resource.Kind, resource.Name)
	}
	body, err := json.Marshal("restart")
	if err != nil {
		return fmt.Errorf("encode restart request: %w", err)
	}
	query := url.Values{
		"resourceName": {resource.Name},
		"namespace":    {resource.Namespace},
		"group":        {resource.Group},
		"version":      {resource.Version},
		"kind":         {resource.Kind},
	}
	if err := s.request(ctx, http.MethodPost, applicationPath(application, "resource/actions", query), bytes.NewBuffer(body), nil); err != nil {
		return fmt.Errorf("restart %s/%s: %w", resource.Kind, resource.Name, err)
	}
	return nil
}

// DeleteResource deletes a resource of the Application.
func (s *APISource) DeleteResource(ctx context.Context, application string, resource explorer.ResourceNode) error {
	query := url.Values{
		"resourceName": {resource.Name},
		"namespace":    {resource.Namespace},
		"group":        {resource.Group},
		"version":      {resource.Version},
		"kind":         {resource.Kind},
	}
	if err := s.request(ctx, http.MethodDelete, applicationPath(application, "resource", query), nil, nil); err != nil {
		return fmt.Errorf("delete %s/%s: %w", resource.Kind, resource.Name, err)
	}
	return nil
}
