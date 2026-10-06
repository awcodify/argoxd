package argocd

import (
	"bufio"
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/awcodify/argoxd/internal/explorer"
	"sigs.k8s.io/yaml"
)

// ResourceManifest returns the live manifest of a resource as YAML.
func (s *APISource) ResourceManifest(ctx context.Context, application string, resource explorer.ResourceNode) (string, error) {
	query := url.Values{
		"namespace":    {resource.Namespace},
		"resourceName": {resource.Name},
		"group":        {resource.Group},
		"version":      {resource.Version},
		"kind":         {resource.Kind},
	}
	var response struct {
		Manifest string `json:"manifest"`
	}
	if err := s.get(ctx, applicationPath(application, "resource", query), &response); err != nil {
		return "", fmt.Errorf("load manifest: %w", err)
	}
	return toYAML(response.Manifest)
}

// ResourceDiff compares the live state of a managed resource with the state
// Argo CD would apply, the same comparison the Argo CD UI shows.
func (s *APISource) ResourceDiff(ctx context.Context, application string, resource explorer.ResourceNode) (string, error) {
	query := url.Values{
		"namespace": {resource.Namespace},
		"name":      {resource.Name},
		"group":     {resource.Group},
		"version":   {resource.Version},
		"kind":      {resource.Kind},
	}
	var response struct {
		Items []struct {
			LiveState           string `json:"liveState"`
			TargetState         string `json:"targetState"`
			NormalizedLiveState string `json:"normalizedLiveState"`
			PredictedLiveState  string `json:"predictedLiveState"`
		} `json:"items"`
	}
	if err := s.get(ctx, applicationPath(application, "managed-resources", query), &response); err != nil {
		return "", fmt.Errorf("load diff: %w", err)
	}
	if len(response.Items) == 0 {
		return "", fmt.Errorf("%s/%s is not managed by Argo CD, so it has no desired state to compare", resource.Kind, resource.Name)
	}

	item := response.Items[0]
	live, err := toYAML(firstNonEmpty(item.NormalizedLiveState, item.LiveState))
	if err != nil {
		return "", err
	}
	desired, err := toYAML(firstNonEmpty(item.PredictedLiveState, item.TargetState))
	if err != nil {
		return "", err
	}
	return strings.Join(lineDiff(live, desired), "\n"), nil
}

// ResourceLogs returns the most recent log lines of a Pod.
func (s *APISource) ResourceLogs(ctx context.Context, application string, pod explorer.ResourceNode) (string, error) {
	query := url.Values{
		"podName":   {pod.Name},
		"namespace": {pod.Namespace},
		"tailLines": {strconv.Itoa(logTailLines)},
		"follow":    {"false"},
	}
	response, err := s.do(ctx, http.MethodGet, applicationPath(application, "logs", query), nil)
	if err != nil {
		return "", fmt.Errorf("load logs: %w", err)
	}
	defer response.Body.Close()

	// The endpoint streams one JSON entry per line.
	var lines []string
	scanner := bufio.NewScanner(response.Body)
	for scanner.Scan() {
		if line, ok := logContent(scanner.Bytes()); ok {
			lines = append(lines, line)
		}
	}
	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("read logs: %w", err)
	}
	return strings.Join(lines, "\n"), nil
}

func applicationPath(application, endpoint string, query url.Values) string {
	return "/api/v1/applications/" + url.PathEscape(application) + "/" + endpoint + "?" + query.Encode()
}

func toYAML(manifest string) (string, error) {
	if manifest == "" {
		return "", nil
	}
	converted, err := yaml.JSONToYAML([]byte(manifest))
	if err != nil {
		return "", fmt.Errorf("convert manifest to YAML: %w", err)
	}
	return string(converted), nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
