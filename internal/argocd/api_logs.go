package argocd

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"

	"github.com/awcodify/argoxd/internal/explorer"
)

var _ LogStreamer = (*APISource)(nil)

// StreamLogs follows the logs of a Pod, or of all the Pods of a workload,
// through the Argo CD API. The API serves one container per request, so
// following every container opens one stream for each.
func (s *APISource) StreamLogs(ctx context.Context, application string, resource explorer.ResourceNode, container string) (stream <-chan LogEntry, err error) {
	ctx, cancel := context.WithCancel(ctx)
	defer func() {
		if err != nil {
			cancel()
		}
	}()

	names := s.logContainerNames(ctx, application, resource, container)
	streams := make([]<-chan LogEntry, 0, len(names))
	for _, name := range names {
		opened, err := s.streamContainer(ctx, application, resource, name)
		if err != nil {
			return nil, err
		}
		streams = append(streams, opened)
	}
	if len(streams) == 1 {
		return streams[0], nil
	}
	return mergeStreams(ctx, streams...), nil
}

// logContainerNames resolves which containers to read. Argo CD would ask for a
// container name on a Pod with several, so the default is looked up in the
// manifest; if that cannot be read, the choice is left to Argo CD (an empty name).
func (s *APISource) logContainerNames(ctx context.Context, application string, resource explorer.ResourceNode, container string) []string {
	if container != "" && container != AllContainers {
		return []string{container}
	}
	containers, err := LogContainers(ctx, s, application, resource)
	if err != nil || len(containers.Names) == 0 {
		return []string{""}
	}
	if container == AllContainers {
		return containers.Names
	}
	return []string{containers.Default}
}

func (s *APISource) streamContainer(ctx context.Context, application string, resource explorer.ResourceNode, container string) (<-chan LogEntry, error) {
	query := url.Values{
		"namespace": {resource.Namespace},
		"tailLines": {strconv.Itoa(logTailLines)},
		"follow":    {"true"},
	}
	if resource.Kind == "Pod" {
		query.Set("podName", resource.Name)
	} else {
		query.Set("group", resource.Group)
		query.Set("kind", resource.Kind)
		query.Set("resourceName", resource.Name)
	}
	if container != "" {
		query.Set("container", container)
	}
	// The shared client's timeout covers reading the whole body, which would
	// end the stream; the transport still bounds the wait for the response.
	client := &http.Client{Transport: s.client.Transport}
	response, err := s.send(ctx, client, http.MethodGet, applicationPath(application, "logs", query), nil)
	if err != nil {
		return nil, fmt.Errorf("follow logs: %w", err)
	}
	return streamLines(ctx, response.Body, func(line []byte) (LogEntry, bool) {
		entry, ok := parseLogEntry(line)
		entry.Container = container
		return entry, ok
	}), nil
}

// parseLogEntry reads one entry of the logs endpoint, which streams one JSON
// entry per line. The last entry of a stream has no content.
func parseLogEntry(line []byte) (LogEntry, bool) {
	var entry struct {
		Result struct {
			Content string `json:"content"`
			PodName string `json:"podName"`
		} `json:"result"`
	}
	if err := json.Unmarshal(line, &entry); err != nil || entry.Result.Content == "" {
		return LogEntry{}, false
	}
	return LogEntry{Pod: entry.Result.PodName, Line: entry.Result.Content}, true
}
