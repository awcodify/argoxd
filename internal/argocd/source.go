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

// ApplicationURLer provides browser URLs for Applications.
type ApplicationURLer interface {
	ApplicationURL(application string) string
}

// ContextSwitcher lists kubeconfig contexts and creates a source for one.
type ContextSwitcher interface {
	Contexts() ([]string, error)
	WithContext(context string) Source
}

// NamespaceSwitcher lists the namespaces of the connected cluster and creates
// a source that looks for Argo CD in another one.
type NamespaceSwitcher interface {
	Namespaces(ctx context.Context) ([]string, error)
	WithNamespace(namespace string) Source
}

// ApplicationOperator supports interactive application operations.
type ApplicationOperator interface {
	LoadResourceTree(ctx context.Context, application string) (explorer.ResourceTree, error)
	SyncApplication(ctx context.Context, application string, options SyncOptions) error
	RefreshApplication(ctx context.Context, application string, hard bool) error
	TerminateOperation(ctx context.Context, application string) error
	DeleteApplication(ctx context.Context, application string) error
}

// ResourceSyncer syncs only some of an Application's resources.
type ResourceSyncer interface {
	// SyncResources syncs the listed resources and leaves the rest of the
	// Application as it is.
	SyncResources(ctx context.Context, application string, resources []explorer.ResourceReference, options SyncOptions) error
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

// ApplicationInspector shows an Application's manifest and its drifted resources.
type ApplicationInspector interface {
	ApplicationManifest(ctx context.Context, application string) (string, error)
	ApplicationDiff(ctx context.Context, application string) (string, error)
}

// ProjectInspector shows an AppProject manifest.
type ProjectInspector interface {
	ProjectManifest(ctx context.Context, project string) (string, error)
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

// ResourceActor acts on the live resources of an Application.
type ResourceActor interface {
	// RestartResource rolls out the Pods of a Deployment, StatefulSet or
	// DaemonSet again, like `kubectl rollout restart`.
	RestartResource(ctx context.Context, application string, resource explorer.ResourceNode) error
	// DeleteResource deletes a resource. A Pod that a workload owns is replaced by it.
	DeleteResource(ctx context.Context, application string, resource explorer.ResourceNode) error
}

// SyncPolicySetter changes how an Application syncs on its own.
type SyncPolicySetter interface {
	// SetSyncPolicy turns auto-sync on or off and sets its self-heal and prune
	// options. Self-heal and prune only apply while auto-sync is on, and are
	// dropped with it.
	SetSyncPolicy(ctx context.Context, application string, policy explorer.SyncPolicy) error
}

// syncPolicyPatch is the merge patch that gives an Application the policy. It
// leaves the rest of the Application's sync policy, such as its sync options,
// as it is.
func syncPolicyPatch(policy explorer.SyncPolicy) map[string]any {
	var automated any
	if policy.Automated {
		automated = map[string]any{"prune": policy.Prune, "selfHeal": policy.SelfHeal}
	}
	return map[string]any{"spec": map[string]any{"syncPolicy": map[string]any{"automated": automated}}}
}

// Restartable reports whether resources of the kind can be restarted.
func Restartable(kind string) bool {
	switch kind {
	case "Deployment", "StatefulSet", "DaemonSet":
		return true
	}
	return false
}

// SyncOptions change how a sync, or a rollback, is applied.
type SyncOptions struct {
	// Prune deletes resources that are no longer in Git.
	Prune bool
	// DryRun previews the sync without changing the cluster.
	DryRun bool
}

// refreshMode is the value Argo CD expects for a normal or hard refresh.
func refreshMode(hard bool) string {
	if hard {
		return "hard"
	}
	return "normal"
}
