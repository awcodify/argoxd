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
	// ResourceLogs returns the recent log lines of a Pod. An empty container
	// means its default container; AllContainers returns every container's
	// lines, grouped by container and prefixed with its name.
	ResourceLogs(ctx context.Context, application string, pod explorer.ResourceNode, container string) (string, error)
}

// LogEntry is one line of a followed log, or the error that ended the stream.
type LogEntry struct {
	// Pod and Container are where the line came from, when the source knows.
	Pod       string
	Container string
	Line      string
	Err       error
}

// AllContainers asks for the logs of every container instead of one.
const AllContainers = "*"

// LogSeparator ends the name that prefixes a line when logs from several
// Pods or containers are shown together.
const LogSeparator = " │ "

// LogStreamer follows the logs of a Pod, or of all the Pods of a workload.
type LogStreamer interface {
	// StreamLogs sends the recent log lines of a Pod, or of every Pod of a
	// Deployment, StatefulSet, DaemonSet, ReplicaSet or Job, and then new lines
	// as they are written. The channel is closed when the stream ends, after a
	// final entry with Err set if it failed. Cancel ctx to stop the stream.
	// An empty container means each Pod's default container; AllContainers
	// follows every container.
	StreamLogs(ctx context.Context, application string, resource explorer.ResourceNode, container string) (<-chan LogEntry, error)
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
