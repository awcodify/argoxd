package argocd

import (
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
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
