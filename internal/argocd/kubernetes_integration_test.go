//go:build integration

package argocd

import (
	"context"
	"testing"
)

func TestKubernetesSourceIntegration(t *testing.T) {
	source := NewKubernetesSource("", "", "argocd")
	snapshot, err := source.Load(context.Background())
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(snapshot.Applications) == 0 {
		t.Fatal("Load() returned no Applications")
	}
	if len(snapshot.Projects) == 0 {
		t.Fatal("Load() returned no AppProjects")
	}

	tree, err := source.LoadResourceTree(context.Background(), snapshot.Applications[0].Name)
	if err != nil {
		t.Fatalf("LoadResourceTree() error = %v", err)
	}
	if tree.Application == "" {
		t.Fatal("LoadResourceTree() returned no Application name")
	}
}
