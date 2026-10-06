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

// StreamLogs follows the logs of a Pod through the Argo CD API.
func (s *APISource) StreamLogs(ctx context.Context, application string, pod explorer.ResourceNode) (<-chan LogEntry, error) {
	query := url.Values{
		"podName":   {pod.Name},
		"namespace": {pod.Namespace},
		"tailLines": {strconv.Itoa(logTailLines)},
		"follow":    {"true"},
	}
	// The shared client's timeout covers reading the whole body, which would
	// end the stream; the transport still bounds the wait for the response.
	client := &http.Client{Transport: s.client.Transport}
	response, err := s.send(ctx, client, http.MethodGet, applicationPath(application, "logs", query), nil)
	if err != nil {
		return nil, fmt.Errorf("follow logs: %w", err)
	}
	return streamLines(ctx, response.Body, logContent), nil
}

// logContent reads the text of one entry of the logs endpoint, which streams
// one JSON entry per line.
func logContent(line []byte) (string, bool) {
	var entry struct {
		Result struct {
			Content string `json:"content"`
		} `json:"result"`
	}
	if err := json.Unmarshal(line, &entry); err != nil || entry.Result.Content == "" {
		return "", false
	}
	return entry.Result.Content, true
}
