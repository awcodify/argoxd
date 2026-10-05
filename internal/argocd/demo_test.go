package argocd

import (
	"context"
	"strings"
	"testing"

	"github.com/awcodify/argoxd/internal/explorer"
)

func TestDemoSourceSyncMakesApplicationSynced(t *testing.T) {
	source := NewDemoSource()
	ctx := context.Background()

	if err := source.SyncApplication(ctx, "cart", SyncOptions{DryRun: true}); err != nil {
		t.Fatal(err)
	}
	if application, _ := source.find("cart"); application.Sync != "OutOfSync" {
		t.Fatalf("dry run changed sync to %q", application.Sync)
	}

	if err := source.SyncApplication(ctx, "cart", SyncOptions{}); err != nil {
		t.Fatal(err)
	}
	if application, _ := source.find("cart"); application.Sync != "Synced" || application.Health != "Healthy" {
		t.Fatalf("application = %+v, want Synced and Healthy", application)
	}
}

func TestDemoSourceDeleteRemovesApplication(t *testing.T) {
	source := NewDemoSource()
	ctx := context.Background()

	if err := source.DeleteApplication(ctx, "cart"); err != nil {
		t.Fatal(err)
	}
	snapshot, _ := source.Load(ctx)
	for _, application := range snapshot.Applications {
		if application.Name == "cart" {
			t.Fatal("cart still listed after delete")
		}
	}
	if err := source.DeleteApplication(ctx, "cart"); err == nil {
		t.Fatal("deleting a missing application succeeded")
	}
}

func TestDemoSourceTreeNestsPodsUnderDeployment(t *testing.T) {
	source := NewDemoSource()
	tree, err := source.LoadResourceTree(context.Background(), "payments")
	if err != nil {
		t.Fatal(err)
	}

	hierarchy := tree.Hierarchy()
	var deployment explorer.ResourceNode
	for _, root := range hierarchy {
		if root.Kind == "Deployment" {
			deployment = root
		}
	}
	if len(deployment.Children) != 1 || len(deployment.Children[0].Children) != 3 {
		t.Fatalf("deployment children = %+v, want one ReplicaSet owning three Pods", deployment.Children)
	}
	logs, _ := source.ResourceLogs(context.Background(), "payments", deployment.Children[0].Children[0])
	if !strings.Contains(logs, "ERROR") {
		t.Fatalf("degraded pod logs have no error:\n%s", logs)
	}
}
