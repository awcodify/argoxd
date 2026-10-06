package argocd

import (
	"context"
	"fmt"

	"github.com/awcodify/argoxd/internal/explorer"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

var _ RollbackOperator = (*KubernetesSource)(nil)

// ApplicationHistory lists the deployments in the Application's status, newest first.
func (s *KubernetesSource) ApplicationHistory(ctx context.Context, application string) ([]explorer.HistoryEntry, error) {
	object, err := s.getApplication(ctx, application)
	if err != nil {
		return nil, fmt.Errorf("load application history %q: %w", application, err)
	}
	deployments, _, _ := unstructured.NestedSlice(object.Object, "status", "history")
	entries := make([]explorer.HistoryEntry, 0, len(deployments))
	for _, deployment := range deployments {
		fields, ok := deployment.(map[string]any)
		if !ok {
			continue
		}
		id, ok := historyID(fields["id"])
		if !ok {
			continue
		}
		revisions, _, _ := unstructured.NestedStringSlice(fields, "revisions")
		entries = append(entries, explorer.HistoryEntry{
			ID:         id,
			Revision:   historyRevision(nestedString(fields, "revision"), revisions),
			DeployedAt: parseTime(nestedString(fields, "deployedAt")),
			Repo:       historyRepo(fields),
		})
	}
	return newestFirst(entries), nil
}

// RollbackApplication requests a sync of the revision recorded under the
// history id, the way `argocd app rollback` does.
func (s *KubernetesSource) RollbackApplication(ctx context.Context, application string, id int64, options SyncOptions) error {
	object, err := s.getApplication(ctx, application)
	if err != nil {
		return fmt.Errorf("roll back application %q: %w", application, err)
	}
	if _, automated, _ := unstructured.NestedMap(object.Object, "spec", "syncPolicy", "automated"); automated {
		return fmt.Errorf("roll back application %q: %s", application, errAutoSyncEnabled)
	}

	if _, pending, _ := unstructured.NestedMap(object.Object, "operation"); pending {
		return fmt.Errorf("roll back application %q: %s", application, errOperationInProgress)
	}

	deployments, _, _ := unstructured.NestedSlice(object.Object, "status", "history")
	for _, deployment := range deployments {
		fields, ok := deployment.(map[string]any)
		if !ok {
			continue
		}
		if got, ok := historyID(fields["id"]); !ok || got != id {
			continue
		}
		sync := map[string]any{"prune": options.Prune, "dryRun": options.DryRun}
		for _, field := range []string{"revision", "revisions", "source", "sources"} {
			if value, found := fields[field]; found {
				sync[field] = value
			}
		}
		patch := map[string]any{"operation": map[string]any{"sync": sync}}
		if err := s.patchApplication(ctx, application, patch); err != nil {
			return fmt.Errorf("roll back application %q: %w", application, err)
		}
		return nil
	}
	return fmt.Errorf("roll back application %q: no deployment with id %d in its history", application, id)
}

func (s *KubernetesSource) getApplication(ctx context.Context, application string) (*unstructured.Unstructured, error) {
	client, err := s.client()
	if err != nil {
		return nil, err
	}
	return client.Resource(applicationsResource).Namespace(s.namespace).Get(ctx, application, metav1.GetOptions{})
}

// historyID reads a history id, which decoders return as an int64, an int or a float64.
func historyID(value any) (int64, bool) {
	switch id := value.(type) {
	case int64:
		return id, true
	case int:
		return int64(id), true
	case float64:
		return int64(id), true
	}
	return 0, false
}

// historyRepo reads the repository of a deployment's single source, or of the
// first source of a multi-source one.
func historyRepo(deployment map[string]any) string {
	if repo := nestedString(deployment, "source", "repoURL"); repo != "" {
		return repo
	}
	sources, _, _ := unstructured.NestedSlice(deployment, "sources")
	if len(sources) > 0 {
		if source, ok := sources[0].(map[string]any); ok {
			return nestedString(source, "repoURL")
		}
	}
	return ""
}
