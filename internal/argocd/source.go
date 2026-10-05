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

// SyncOptions change how a sync is applied.
type SyncOptions struct {
	// Prune deletes resources that are no longer in Git.
	Prune bool
	// DryRun previews the sync without changing the cluster.
	DryRun bool
}
