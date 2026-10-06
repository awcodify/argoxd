// Package argocd provides resource sources backed by Argo CD.
package argocd

import (
	"context"

	"github.com/awcodify/argoxd/internal/explorer"
)

// logTailLines is how many of the most recent log lines are fetched for a Pod.
const logTailLines = 200

// Source loads the resources shown by the explorer.
type Source interface {
	Load(ctx context.Context) (explorer.Snapshot, error)
}

// ApplicationOperator supports interactive application operations.
type ApplicationOperator interface {
	LoadResourceTree(ctx context.Context, application string) (explorer.ResourceTree, error)
	SyncApplication(ctx context.Context, application string, options SyncOptions) error
	RefreshApplication(ctx context.Context, application string) error
	DeleteApplication(ctx context.Context, application string) error
}

// ResourceInspector shows the live state of an Application's resources.
type ResourceInspector interface {
	ResourceManifest(ctx context.Context, application string, resource explorer.ResourceNode) (string, error)
	ResourceDiff(ctx context.Context, application string, resource explorer.ResourceNode) (string, error)
	ResourceLogs(ctx context.Context, application string, pod explorer.ResourceNode) (string, error)
}

// RollbackOperator lists an Application's past deployments and redeploys one of them.
type RollbackOperator interface {
	// ApplicationHistory returns the recorded deployments, newest first.
	ApplicationHistory(ctx context.Context, application string) ([]explorer.HistoryEntry, error)
	// RollbackApplication redeploys the deployment recorded under id. Argo CD
	// refuses it while auto-sync is enabled.
	RollbackApplication(ctx context.Context, application string, id int64, options SyncOptions) error
}

// SyncOptions change how a sync, or a rollback, is applied.
type SyncOptions struct {
	// Prune deletes resources that are no longer in Git.
	Prune bool
	// DryRun previews the sync without changing the cluster.
	DryRun bool
}
