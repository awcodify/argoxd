package argocd

import (
	"context"
	"slices"
	"testing"

	"github.com/awcodify/argoxd/internal/explorer"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

var checkoutSyncOptions = []string{"CreateNamespace=true", "ServerSideApply=true"}

// applicationWithSyncOptions is an Application whose sync policy lists sync
// options, which Argo CD's own API adds to every manual sync.
func applicationWithSyncOptions(options ...string) *unstructured.Unstructured {
	application := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "argoproj.io/v1alpha1",
		"kind":       "Application",
		"metadata":   map[string]any{"namespace": "argocd", "name": "checkout"},
		"spec":       map[string]any{"project": "store"},
	}}
	if len(options) > 0 {
		_ = unstructured.SetNestedStringSlice(application.Object, options, "spec", "syncPolicy", "syncOptions")
	}
	return application
}

func recordedSyncOptions(t *testing.T, source *KubernetesSource) ([]string, bool) {
	t.Helper()
	got, err := source.getApplication(context.Background(), "checkout")
	if err != nil {
		t.Fatalf("get application: %v", err)
	}
	sync, _, _ := unstructured.NestedMap(got.Object, "operation", "sync")
	options, found, err := unstructured.NestedStringSlice(sync, "syncOptions")
	if err != nil {
		t.Fatalf("operation.sync.syncOptions: %v", err)
	}
	return options, found
}

func TestKubernetesSourceSyncPassesTheApplicationsSyncOptions(t *testing.T) {
	source := fakeKubernetesSource(applicationWithSyncOptions(checkoutSyncOptions...))

	if err := source.SyncApplication(context.Background(), "checkout", SyncOptions{Prune: true}); err != nil {
		t.Fatalf("SyncApplication() error = %v", err)
	}

	if options, _ := recordedSyncOptions(t, source); !slices.Equal(options, checkoutSyncOptions) {
		t.Fatalf("operation.sync.syncOptions = %v, want %v", options, checkoutSyncOptions)
	}
}

func TestKubernetesSourceSelectiveSyncPassesTheApplicationsSyncOptions(t *testing.T) {
	source := fakeKubernetesSource(applicationWithSyncOptions(checkoutSyncOptions...))
	resources := []explorer.ResourceReference{{Group: "apps", Kind: "Deployment", Namespace: "store", Name: "web"}}

	if err := source.SyncResources(context.Background(), "checkout", resources, SyncOptions{}); err != nil {
		t.Fatalf("SyncResources() error = %v", err)
	}

	if options, _ := recordedSyncOptions(t, source); !slices.Equal(options, checkoutSyncOptions) {
		t.Fatalf("operation.sync.syncOptions = %v, want %v", options, checkoutSyncOptions)
	}
}

func TestKubernetesSourceSyncOfAnApplicationWithoutSyncOptionsSendsNone(t *testing.T) {
	source := fakeKubernetesSource(applicationWithSyncOptions())

	if err := source.SyncApplication(context.Background(), "checkout", SyncOptions{}); err != nil {
		t.Fatalf("SyncApplication() error = %v", err)
	}

	if options, found := recordedSyncOptions(t, source); found || len(options) != 0 {
		t.Fatalf("operation.sync.syncOptions = %v (present: %v), want none", options, found)
	}
}

func TestKubernetesSourceRollbackPassesTheApplicationsSyncOptions(t *testing.T) {
	application := historyApplication(false)
	if err := unstructured.SetNestedStringSlice(application.Object, checkoutSyncOptions, "spec", "syncPolicy", "syncOptions"); err != nil {
		t.Fatalf("set sync options: %v", err)
	}
	source := fakeKubernetesSource(application)

	if err := source.RollbackApplication(context.Background(), "checkout", 1, SyncOptions{}); err != nil {
		t.Fatalf("RollbackApplication() error = %v", err)
	}

	if options, _ := recordedSyncOptions(t, source); !slices.Equal(options, checkoutSyncOptions) {
		t.Fatalf("operation.sync.syncOptions = %v, want %v", options, checkoutSyncOptions)
	}
}
