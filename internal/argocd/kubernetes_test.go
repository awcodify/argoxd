package argocd

import (
	"context"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/awcodify/argoxd/internal/explorer"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic/fake"
	typedfake "k8s.io/client-go/kubernetes/fake"
)

func TestSnapshotFromKubernetesResources(t *testing.T) {
	applications := []unstructured.Unstructured{{
		Object: map[string]any{
			"metadata": map[string]any{"name": "checkout", "namespace": "argocd"},
			"spec":     map[string]any{"project": "store"},
			"status": map[string]any{
				"sync":   map[string]any{"status": "OutOfSync"},
				"health": map[string]any{"status": "Degraded"},
			},
		},
	}}
	projects := []unstructured.Unstructured{{
		Object: map[string]any{
			"metadata": map[string]any{"name": "store"},
			"spec":     map[string]any{"description": "Store applications"},
		},
	}}
	clusters := []unstructured.Unstructured{{
		Object: map[string]any{
			"metadata": map[string]any{"name": "cluster-production"},
			"data":     map[string]any{"name": "production", "server": "https://production.example.com"},
		},
	}}

	snapshot := snapshotFromKubernetesResources(applications, projects, clusters)

	if got := snapshot.Applications[0].Sync; got != "OutOfSync" {
		t.Fatalf("application sync = %q", got)
	}

	if got := snapshot.Projects[0].Description; got != "Store applications" {
		t.Fatalf("project description = %q", got)
	}
	if got := snapshot.Clusters[0].Name; got != "production" {
		t.Fatalf("cluster name = %q", got)
	}
}

func TestResourceTreeFromApplicationStatus(t *testing.T) {
	application := unstructured.Unstructured{Object: map[string]any{
		"status": map[string]any{
			"resources": []any{
				map[string]any{
					"kind":      "Service",
					"namespace": "default",
					"name":      "checkout",
					"health":    map[string]any{"status": "Healthy"},
					"status":    "Synced",
				},
			},
		},
	}}

	tree := resourceTreeFromApplication(application)
	if len(tree.Nodes) != 1 || tree.Nodes[0].Kind != "Service" {
		t.Fatalf("tree nodes = %#v, want service", tree.Nodes)
	}
}

func TestSnapshotReadsApplicationRevisionDestinationAndLastSync(t *testing.T) {
	applications := []unstructured.Unstructured{{Object: map[string]any{
		"metadata": map[string]any{"name": "checkout"},
		"spec": map[string]any{
			"source":      map[string]any{"targetRevision": "main"},
			"destination": map[string]any{"server": "https://kubernetes.default.svc", "namespace": "store"},
		},
		"status": map[string]any{
			"operationState": map[string]any{"finishedAt": "2026-10-05T10:00:00Z"},
		},
	}}}

	application := snapshotFromKubernetesResources(applications, nil, nil).Applications[0]

	if application.Revision != "main" {
		t.Fatalf("revision = %q, want main", application.Revision)
	}
	if application.Destination != "https://kubernetes.default.svc/store" {
		t.Fatalf("destination = %q", application.Destination)
	}
	if want := time.Date(2026, 10, 5, 10, 0, 0, 0, time.UTC); !application.LastSync.Equal(want) {
		t.Fatalf("last sync = %v, want %v", application.LastSync, want)
	}
}

func TestKubernetesSourceTreeIncludesOwnedResources(t *testing.T) {
	application := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "argoproj.io/v1alpha1",
		"kind":       "Application",
		"metadata":   map[string]any{"namespace": "argocd", "name": "checkout"},
		"status": map[string]any{"resources": []any{map[string]any{
			"group": "apps", "version": "v1", "kind": "Deployment", "namespace": "store", "name": "web",
			"status": "Synced", "health": map[string]any{"status": "Healthy"},
		}}},
	}}
	replicaSet := object("apps/v1", "ReplicaSet", "store", "web-1", owner("apps/v1", "Deployment", "web"))
	pod := object("v1", "Pod", "store", "web-1-abc", owner("apps/v1", "ReplicaSet", "web-1"))
	source := fakeKubernetesSource(application, &replicaSet, &pod)

	tree, err := source.LoadResourceTree(context.Background(), "checkout")
	if err != nil {
		t.Fatalf("LoadResourceTree() error = %v", err)
	}

	if tree.Application != "checkout" || len(tree.Nodes) != 3 {
		t.Fatalf("tree = %+v, want checkout with deployment, replica set and pod", tree)
	}
	if tree.Nodes[0].Version != "v1" {
		t.Fatalf("deployment version = %q, want v1", tree.Nodes[0].Version)
	}
}

func fakeKubernetesSource(objects ...runtime.Object) *KubernetesSource {
	listKinds := map[schema.GroupVersionResource]string{
		{Group: "apps", Version: "v1", Resource: "deployments"}: "DeploymentList",
		applicationsResource: "ApplicationList",
		projectsResource:     "AppProjectList",
		secretsResource:      "SecretList",
	}
	for _, resource := range ownedResources {
		listKinds[resource] = "List"
	}
	client := fake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(), listKinds, objects...)
	return &KubernetesSource{
		namespace: "argocd",
		clients: func() (kubernetesClients, error) {
			return kubernetesClients{dynamic: client, typed: typedfake.NewClientset()}, nil
		},
	}
}

func TestKubernetesSourceSyncsWithOptionsAndRefreshes(t *testing.T) {
	application := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "argoproj.io/v1alpha1",
		"kind":       "Application",
		"metadata":   map[string]any{"namespace": "argocd", "name": "checkout"},
	}}
	source := fakeKubernetesSource(application)

	if err := source.SyncApplication(context.Background(), "checkout", SyncOptions{Prune: true}); err != nil {
		t.Fatalf("SyncApplication() error = %v", err)
	}
	if err := source.RefreshApplication(context.Background(), "checkout"); err != nil {
		t.Fatalf("RefreshApplication() error = %v", err)
	}

	client, _ := source.client()
	got, err := client.Resource(applicationsResource).Namespace("argocd").Get(context.Background(), "checkout", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("get application: %v", err)
	}
	if prune, _, _ := unstructured.NestedBool(got.Object, "operation", "sync", "prune"); !prune {
		t.Fatalf("operation = %v, want a pruning sync", got.Object["operation"])
	}
	if got.GetAnnotations()["argocd.argoproj.io/refresh"] != "hard" {
		t.Fatalf("annotations = %v, want a hard refresh", got.GetAnnotations())
	}
}

func TestKubernetesSourceInspectsResources(t *testing.T) {
	deployment := object("apps/v1", "Deployment", "store", "web")
	deployment.Object["spec"] = map[string]any{"replicas": int64(2)}
	source := fakeKubernetesSource(&deployment)
	node := explorer.ResourceNode{Group: "apps", Version: "v1", Kind: "Deployment", Namespace: "store", Name: "web"}

	manifest, err := source.ResourceManifest(context.Background(), "checkout", node)
	if err != nil || !strings.Contains(manifest, "replicas: 2") {
		t.Fatalf("ResourceManifest() = %q, %v", manifest, err)
	}

	if _, err := source.ResourceDiff(context.Background(), "checkout", node); err == nil {
		t.Fatal("ResourceDiff() error = nil, want an error explaining that diffs need the API")
	}

	pod := explorer.ResourceNode{Version: "v1", Kind: "Pod", Namespace: "store", Name: "web-1-abc"}
	logs, err := source.ResourceLogs(context.Background(), "checkout", pod, "")
	if err != nil || logs != "fake logs" {
		t.Fatalf("ResourceLogs() = %q, %v", logs, err)
	}
}

func historyApplication(automated bool) *unstructured.Unstructured {
	spec := map[string]any{}
	if automated {
		spec["syncPolicy"] = map[string]any{"automated": map[string]any{}}
	}
	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "argoproj.io/v1alpha1",
		"kind":       "Application",
		"metadata":   map[string]any{"namespace": "argocd", "name": "checkout"},
		"spec":       spec,
		"status": map[string]any{"history": []any{
			map[string]any{"id": int64(1), "revision": "aaa1111", "deployedAt": "2026-10-01T10:00:00Z",
				"source": map[string]any{"repoURL": "https://example.com/a.git", "path": "old"}},
			map[string]any{"id": int64(2), "revision": "bbb2222", "deployedAt": "2026-10-02T10:00:00Z",
				"source": map[string]any{"repoURL": "https://example.com/a.git", "path": "new"}},
		}},
	}}
}

func TestKubernetesSourceReadsHistory(t *testing.T) {
	source := fakeKubernetesSource(historyApplication(false))

	history, err := source.ApplicationHistory(context.Background(), "checkout")
	if err != nil {
		t.Fatal(err)
	}

	if len(history) != 2 || history[0].ID != 2 || history[0].Revision != "bbb2222" || history[0].Repo != "https://example.com/a.git" || history[1].ID != 1 {
		t.Fatalf("history = %+v", history)
	}
}

func TestKubernetesSourceRollbackSyncsTheRecordedRevision(t *testing.T) {
	source := fakeKubernetesSource(historyApplication(false))

	if err := source.RollbackApplication(context.Background(), "checkout", 1, SyncOptions{Prune: true}); err != nil {
		t.Fatalf("RollbackApplication() error = %v", err)
	}

	got, _ := source.getApplication(context.Background(), "checkout")
	sync, _, _ := unstructured.NestedMap(got.Object, "operation", "sync")
	path, _, _ := unstructured.NestedString(sync, "source", "path")
	if sync["revision"] != "aaa1111" || sync["prune"] != true || path != "old" {
		t.Fatalf("operation.sync = %v, want revision aaa1111, prune and the old source", sync)
	}
}

func TestKubernetesSourceRollbackRefusesAutoSyncAndUnknownIds(t *testing.T) {
	automated := fakeKubernetesSource(historyApplication(true))
	if err := automated.RollbackApplication(context.Background(), "checkout", 1, SyncOptions{}); err == nil || !strings.Contains(err.Error(), "auto-sync") {
		t.Fatalf("error = %v, want an auto-sync refusal", err)
	}

	manual := fakeKubernetesSource(historyApplication(false))
	if err := manual.RollbackApplication(context.Background(), "checkout", 9, SyncOptions{}); err == nil {
		t.Fatal("rolling back to an unknown id succeeded")
	}
}

func TestKubernetesSourceRollbackRefusesWhileAnotherOperationIsPending(t *testing.T) {
	application := historyApplication(false)
	application.Object["operation"] = map[string]any{"sync": map[string]any{"revision": "ccc3333"}}
	source := fakeKubernetesSource(application)

	err := source.RollbackApplication(context.Background(), "checkout", 1, SyncOptions{})

	if err == nil || !strings.Contains(err.Error(), "in progress") {
		t.Fatalf("error = %v, want a refusal because an operation is in progress", err)
	}
	got, _ := source.getApplication(context.Background(), "checkout")
	if revision, _, _ := unstructured.NestedString(got.Object, "operation", "sync", "revision"); revision != "ccc3333" {
		t.Fatalf("the pending operation was overwritten: %v", got.Object["operation"])
	}
}

func TestSnapshotReadsConditionsAndSyncPolicy(t *testing.T) {
	applications := []unstructured.Unstructured{{Object: map[string]any{
		"metadata": map[string]any{"name": "checkout"},
		"spec": map[string]any{
			"syncPolicy": map[string]any{
				"automated": map[string]any{"prune": true, "selfHeal": true},
			},
		},
		"status": map[string]any{
			"conditions": []any{
				map[string]any{"type": "ComparisonError", "message": "repository not found"},
				map[string]any{"type": "SyncError", "message": "one or more objects failed to apply"},
			},
		},
	}}, {Object: map[string]any{
		"metadata": map[string]any{"name": "manual"},
		"spec":     map[string]any{},
	}}}

	snapshot := snapshotFromKubernetesResources(applications, nil, nil)

	checkout := snapshot.Applications[0]
	wantConditions := []explorer.Condition{
		{Type: "ComparisonError", Message: "repository not found"},
		{Type: "SyncError", Message: "one or more objects failed to apply"},
	}
	if !slices.Equal(checkout.Conditions, wantConditions) {
		t.Fatalf("conditions = %+v, want %+v", checkout.Conditions, wantConditions)
	}
	if want := (explorer.SyncPolicy{Automated: true, SelfHeal: true, Prune: true}); checkout.Policy != want {
		t.Fatalf("policy = %+v, want %+v", checkout.Policy, want)
	}
	manual := snapshot.Applications[1]
	if len(manual.Conditions) != 0 || manual.Policy != (explorer.SyncPolicy{}) {
		t.Fatalf("manual application = %+v, want no conditions and no automation", manual)
	}
}

func TestSnapshotTreatsAutomatedSyncWithoutOptionsAsAutomated(t *testing.T) {
	applications := []unstructured.Unstructured{{Object: map[string]any{
		"metadata": map[string]any{"name": "checkout"},
		"spec":     map[string]any{"syncPolicy": map[string]any{"automated": map[string]any{}}},
	}}}

	policy := snapshotFromKubernetesResources(applications, nil, nil).Applications[0].Policy

	if want := (explorer.SyncPolicy{Automated: true}); policy != want {
		t.Fatalf("policy = %+v, want %+v", policy, want)
	}
}

func TestResourceTreeFromApplicationReadsRequiresPruning(t *testing.T) {
	application := unstructured.Unstructured{Object: map[string]any{
		"status": map[string]any{
			"resources": []any{
				map[string]any{"kind": "ConfigMap", "name": "old", "requiresPruning": true},
				map[string]any{"kind": "ConfigMap", "name": "current"},
			},
		},
	}}

	nodes := resourceTreeFromApplication(application).Nodes

	if !nodes[0].RequiresPruning || nodes[1].RequiresPruning {
		t.Fatalf("requires pruning = %v, %v, want true, false", nodes[0].RequiresPruning, nodes[1].RequiresPruning)
	}
}
