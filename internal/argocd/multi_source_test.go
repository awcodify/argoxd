package argocd

import (
	"context"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// multiSourceApplication is an Application that pulls from two repositories,
// as Argo CD 2.6 and later allows, with one deployment in its history.
func multiSourceApplication() *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "argoproj.io/v1alpha1",
		"kind":       "Application",
		"metadata":   map[string]any{"namespace": "argocd", "name": "checkout"},
		"spec": map[string]any{"sources": []any{
			map[string]any{"repoURL": "https://example.com/chart.git", "targetRevision": "v1.4.0", "chart": "web"},
			map[string]any{"repoURL": "https://example.com/values.git", "targetRevision": "main", "ref": "values"},
		}},
		"status": map[string]any{"history": []any{
			map[string]any{"id": int64(1), "deployedAt": "2026-10-01T10:00:00Z",
				"revisions": []any{"aaa1111", "ccc3333"},
				"sources": []any{
					map[string]any{"repoURL": "https://example.com/chart.git", "targetRevision": "v1.3.0"},
					map[string]any{"repoURL": "https://example.com/values.git", "targetRevision": "main"},
				}},
		}},
	}}
}

func TestKubernetesSourceShowsTheFirstSourceOfAMultiSourceApplication(t *testing.T) {
	snapshot, err := fakeKubernetesSource(multiSourceApplication()).Load(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	if len(snapshot.Applications) != 1 || snapshot.Applications[0].Revision != "v1.4.0" {
		t.Fatalf("applications = %+v, want revision v1.4.0 from the first source", snapshot.Applications)
	}
}

func TestKubernetesSourceReadsTheHistoryOfAMultiSourceApplication(t *testing.T) {
	history, err := fakeKubernetesSource(multiSourceApplication()).ApplicationHistory(context.Background(), "checkout")
	if err != nil {
		t.Fatal(err)
	}

	if len(history) != 1 || history[0].Revision != "aaa1111" || history[0].Repo != "https://example.com/chart.git" {
		t.Fatalf("history = %+v, want the first source's revision and repository", history)
	}
}

func TestKubernetesSourceRollsBackEverySourceOfAMultiSourceApplication(t *testing.T) {
	source := fakeKubernetesSource(multiSourceApplication())

	if err := source.RollbackApplication(context.Background(), "checkout", 1, SyncOptions{}); err != nil {
		t.Fatalf("RollbackApplication() error = %v", err)
	}

	got, _ := source.getApplication(context.Background(), "checkout")
	revisions, _, _ := unstructured.NestedStringSlice(got.Object, "operation", "sync", "revisions")
	sources, _, _ := unstructured.NestedSlice(got.Object, "operation", "sync", "sources")
	if len(revisions) != 2 || revisions[0] != "aaa1111" || revisions[1] != "ccc3333" || len(sources) != 2 {
		t.Fatalf("operation.sync revisions = %v, sources = %v; want both revisions and both sources", revisions, sources)
	}
}
