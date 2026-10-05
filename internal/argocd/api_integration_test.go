//go:build integration

package argocd

import (
	"context"
	"os"
	"testing"
)

func TestAPISourceIntegration(t *testing.T) {
	server := os.Getenv("ARGOCD_API_SERVER")
	token := os.Getenv("ARGOCD_AUTH_TOKEN")
	if server == "" || token == "" {
		t.Skip("ARGOCD_API_SERVER and ARGOCD_AUTH_TOKEN are required")
	}

	source := NewAPISource(server, token, true)
	snapshot, err := source.Load(context.Background())
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(snapshot.Applications) == 0 {
		t.Fatal("Load() returned no Applications")
	}
}
