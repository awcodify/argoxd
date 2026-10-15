package argocd

import (
	"context"
	"strings"
	"testing"

	"github.com/awcodify/argoxd/internal/explorer"
)

func TestDemoSourceShowsASyncWindowBlockAndRunningOperation(t *testing.T) {
	snapshot, err := NewDemoSource().Load(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	applications := map[string]explorer.Application{}
	for _, application := range snapshot.Applications {
		applications[application.Name] = application
	}
	if !applications["cart"].SyncWindowBlocked() {
		t.Fatalf("cart conditions = %+v, want a sync-window block", applications["cart"].Conditions)
	}
	if operation := applications["search"].Operation; operation == nil || operation.Phase != "Running" {
		t.Fatalf("search operation = %+v, want a running operation", operation)
	}
}

func TestDemoSourceProjectManifestShowsPolicyAndSyncWindow(t *testing.T) {
	manifest, err := NewDemoSource().ProjectManifest(context.Background(), "store")
	if err != nil || !strings.Contains(manifest, "roles:") || !strings.Contains(manifest, "syncWindows:") || !strings.Contains(manifest, "store-freeze") {
		t.Fatalf("ProjectManifest() = %q, %v", manifest, err)
	}
}
