package argocd

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/awcodify/argoxd/internal/explorer"
)

const podManifest = `
apiVersion: v1
kind: Pod
metadata:
  name: web-abc
spec:
  initContainers:
    - name: init
  containers:
    - name: app
    - name: sidecar
`

func TestContainersFromAPodManifest(t *testing.T) {
	got := containersFromManifest(podManifest)

	if !slices.Equal(got.Names, []string{"app", "sidecar"}) || got.Default != "app" {
		t.Fatalf("containers = %+v, want app and sidecar with app by default (init containers left out)", got)
	}
}

func TestDefaultContainerAnnotationChoosesTheDefault(t *testing.T) {
	manifest := `
metadata:
  annotations:
    kubectl.kubernetes.io/default-container: sidecar
spec:
  containers:
    - name: app
    - name: sidecar
`
	if got := containersFromManifest(manifest); got.Default != "sidecar" {
		t.Fatalf("default = %q, want the annotated container", got.Default)
	}

	unknown := `
metadata:
  annotations:
    kubectl.kubernetes.io/default-container: gone
spec:
  containers:
    - name: app
`
	if got := containersFromManifest(unknown); got.Default != "app" {
		t.Fatalf("default = %q, want the first container when the annotation names none", got.Default)
	}
}

func TestContainersFromAWorkloadManifestComeFromItsPodTemplate(t *testing.T) {
	manifest := `
kind: Deployment
metadata:
  annotations:
    kubectl.kubernetes.io/default-container: not-this
spec:
  template:
    metadata:
      annotations:
        kubectl.kubernetes.io/default-container: proxy
    spec:
      containers:
        - name: web
        - name: proxy
`
	got := containersFromManifest(manifest)

	if !slices.Equal(got.Names, []string{"web", "proxy"}) || got.Default != "proxy" {
		t.Fatalf("containers = %+v, want the template's containers with proxy by default", got)
	}
}

func TestContainersFromAnUnreadableManifestAreEmpty(t *testing.T) {
	for _, manifest := range []string{"", "not: [valid", "kind: ConfigMap"} {
		if got := containersFromManifest(manifest); len(got.Names) != 0 || got.Default != "" {
			t.Fatalf("containersFromManifest(%q) = %+v, want none", manifest, got)
		}
	}
}

type manifestInspector struct {
	manifest string
	err      error
	asked    explorer.ResourceNode
}

func (m *manifestInspector) ResourceManifest(_ context.Context, _ string, resource explorer.ResourceNode) (string, error) {
	m.asked = resource
	return m.manifest, m.err
}

func (m *manifestInspector) ResourceDiff(context.Context, string, explorer.ResourceNode) (string, error) {
	return "", nil
}

func (m *manifestInspector) ResourceLogs(context.Context, string, explorer.ResourceNode, string) (string, error) {
	return "", nil
}

func TestLogContainersReadsTheResourcesManifest(t *testing.T) {
	inspector := &manifestInspector{manifest: podManifest}

	got, err := LogContainers(context.Background(), inspector, "checkout", webPod)
	if err != nil {
		t.Fatal(err)
	}

	if !slices.Equal(got.Names, []string{"app", "sidecar"}) || inspector.asked.Name != "web-1-abc" {
		t.Fatalf("containers = %+v for %+v", got, inspector.asked)
	}

	failing := &manifestInspector{err: errors.New("forbidden")}
	if _, err := LogContainers(context.Background(), failing, "checkout", webPod); err == nil {
		t.Fatal("a manifest error was swallowed")
	}
}
