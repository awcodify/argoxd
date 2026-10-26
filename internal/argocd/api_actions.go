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

var (
	_ ResourceActor        = (*APISource)(nil)
	_ ResourceActionRunner = (*APISource)(nil)
)

// resourceQuery names a resource of an Application in Argo CD's resource endpoints.
func resourceQuery(resource explorer.ResourceNode) url.Values {
	return url.Values{
		"resourceName": {resource.Name},
		"namespace":    {resource.Namespace},
		"group":        {resource.Group},
		"version":      {resource.Version},
		"kind":         {resource.Kind},
	}
}

// ResourceActions lists the actions Argo CD defines for a resource, including
// the ones custom resources add through argocd-cm.
func (s *APISource) ResourceActions(ctx context.Context, application string, resource explorer.ResourceNode) ([]ResourceAction, error) {
	var response struct {
		Actions []struct {
			Name     string            `json:"name"`
			Disabled bool              `json:"disabled"`
			Params   []json.RawMessage `json:"params"`
		} `json:"actions"`
	}
	if err := s.request(ctx, http.MethodGet, applicationPath(application, "resource/actions", resourceQuery(resource)), nil, &response); err != nil {
		return nil, fmt.Errorf("list actions of %s/%s: %w", resource.Kind, resource.Name, err)
	}
	actions := make([]ResourceAction, 0, len(response.Actions))
	for _, action := range response.Actions {
		actions = append(actions, ResourceAction{Name: action.Name, Disabled: action.Disabled, NeedsParameters: len(action.Params) > 0})
	}
	return actions, nil
}

// RunResourceAction runs one of the resource's actions. Argo CD takes the
// action's name as the whole request body and the resource in the query.
func (s *APISource) RunResourceAction(ctx context.Context, application string, resource explorer.ResourceNode, action string) error {
	body, err := json.Marshal(action)
	if err != nil {
		return fmt.Errorf("encode %s request: %w", action, err)
	}
	if err := s.request(ctx, http.MethodPost, applicationPath(application, "resource/actions", resourceQuery(resource)), bytes.NewBuffer(body), nil); err != nil {
		return fmt.Errorf("%s %s/%s: %w", action, resource.Kind, resource.Name, err)
	}
	return nil
}

// RestartResource runs Argo CD's restart action on a Deployment, StatefulSet or
// DaemonSet.
func (s *APISource) RestartResource(ctx context.Context, application string, resource explorer.ResourceNode) error {
	if !Restartable(resource.Kind) {
		return fmt.Errorf("%s/%s cannot be restarted: only Deployments, StatefulSets and DaemonSets can", resource.Kind, resource.Name)
	}
	return s.RunResourceAction(ctx, application, resource, "restart")
}

// DeleteResource deletes a resource of the Application.
func (s *APISource) DeleteResource(ctx context.Context, application string, resource explorer.ResourceNode) error {
	if err := s.request(ctx, http.MethodDelete, applicationPath(application, "resource", resourceQuery(resource)), nil, nil); err != nil {
		return fmt.Errorf("delete %s/%s: %w", resource.Kind, resource.Name, err)
	}
	return nil
}
