package argocd

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAPISourceInspectsProjects(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/projects/platform" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(`{"apiVersion":"argoproj.io/v1alpha1","kind":"AppProject","spec":{"sourceRepos":["https://example.com/platform.git"]}}`))
	}))
	defer server.Close()

	manifest, err := NewAPISource(server.URL, "", false).ProjectManifest(context.Background(), "platform")
	if err != nil || !strings.Contains(manifest, "kind: AppProject") || !strings.Contains(manifest, "platform.git") {
		t.Fatalf("ProjectManifest() = %q, %v", manifest, err)
	}
}
