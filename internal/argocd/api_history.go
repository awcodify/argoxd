package argocd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/awcodify/argoxd/internal/explorer"
)

var _ RollbackOperator = (*APISource)(nil)

// ApplicationHistory lists the deployments Argo CD recorded for an Application, newest first.
func (s *APISource) ApplicationHistory(ctx context.Context, application string) ([]explorer.HistoryEntry, error) {
	var response applicationHistoryResponse
	if err := s.get(ctx, "/api/v1/applications/"+url.PathEscape(application), &response); err != nil {
		return nil, fmt.Errorf("load application history: %w", err)
	}
	entries := make([]explorer.HistoryEntry, 0, len(response.Status.History))
	for _, deployment := range response.Status.History {
		entry := explorer.HistoryEntry{
			ID:         deployment.ID,
			Revision:   historyRevision(deployment.Revision, deployment.Revisions),
			DeployedAt: deployment.DeployedAt,
		}
		if deployment.Source != nil {
			entry.Repo = deployment.Source.RepoURL
		} else if len(deployment.Sources) > 0 {
			entry.Repo = deployment.Sources[0].RepoURL
		}
		entries = append(entries, entry)
	}
	return newestFirst(entries), nil
}

// RollbackApplication redeploys the revision recorded under the history id.
func (s *APISource) RollbackApplication(ctx context.Context, application string, id int64, options SyncOptions) error {
	body, err := json.Marshal(rollbackRequest{ID: id, Prune: options.Prune, DryRun: options.DryRun})
	if err != nil {
		return fmt.Errorf("encode rollback request: %w", err)
	}
	path := "/api/v1/applications/" + url.PathEscape(application) + "/rollback"
	if err := s.request(ctx, http.MethodPost, path, bytes.NewBuffer(body), nil); err != nil {
		return fmt.Errorf("roll back application: %w", err)
	}
	return nil
}

type rollbackRequest struct {
	ID     int64 `json:"id"`
	Prune  bool  `json:"prune"`
	DryRun bool  `json:"dryRun"`
}

type applicationHistoryResponse struct {
	Status struct {
		History []struct {
			ID         int64               `json:"id"`
			Revision   string              `json:"revision"`
			Revisions  []string            `json:"revisions"`
			DeployedAt time.Time           `json:"deployedAt"`
			Source     *applicationSource  `json:"source"`
			Sources    []applicationSource `json:"sources"`
		} `json:"history"`
	} `json:"status"`
}
