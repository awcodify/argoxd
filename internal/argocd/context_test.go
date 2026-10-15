package argocd

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestKubernetesSourceListsAndSwitchesKubeconfigContexts(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config")
	config := []byte(`apiVersion: v1
kind: Config
current-context: development
contexts:
- name: production
  context:
    cluster: production
    user: operator
- name: development
  context:
    cluster: development
    user: operator
`)
	if err := os.WriteFile(path, config, 0o600); err != nil {
		t.Fatal(err)
	}
	source := NewKubernetesSource(path, "development", "argocd")

	contexts, err := source.Contexts()
	if err != nil || !slices.Equal(contexts, []string{"development", "production"}) {
		t.Fatalf("Contexts() = %v, %v", contexts, err)
	}
	switched, ok := source.WithContext("production").(*KubernetesSource)
	if !ok || switched.context != "production" || switched.kubeconfig != path || switched.namespace != "argocd" {
		t.Fatalf("WithContext() = %#v", switched)
	}
}
