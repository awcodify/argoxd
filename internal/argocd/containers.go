package argocd

import (
	"context"
	"fmt"

	"github.com/awcodify/argoxd/internal/explorer"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"
)

// defaultContainerAnnotation names the container `kubectl logs` shows when a
// Pod has several and none is chosen.
const defaultContainerAnnotation = "kubectl.kubernetes.io/default-container"

// Containers are the containers of a Pod, or of the Pods a workload runs.
type Containers struct {
	// Names lists the regular containers, in the order they are declared.
	Names []string
	// Default is the container shown when none is chosen.
	Default string
}

// LogContainers reads the containers of a Pod or workload from its manifest.
func LogContainers(ctx context.Context, inspector ResourceInspector, application string, resource explorer.ResourceNode) (Containers, error) {
	manifest, err := inspector.ResourceManifest(ctx, application, resource)
	if err != nil {
		return Containers{}, fmt.Errorf("read the containers of %s/%s: %w", resource.Kind, resource.Name, err)
	}
	return containersFromManifest(manifest), nil
}

// containersFromManifest lists the containers of a Pod manifest, or of the Pod
// template of a workload manifest. Init containers have finished by the time
// anyone reads logs, so they are left out.
func containersFromManifest(manifest string) Containers {
	var object map[string]any
	if err := yaml.Unmarshal([]byte(manifest), &object); err != nil {
		return Containers{}
	}
	if template, found, _ := unstructured.NestedMap(object, "spec", "template"); found {
		object = template
	}

	declared, _, _ := unstructured.NestedSlice(object, "spec", "containers")
	var containers Containers
	for _, container := range declared {
		if fields, ok := container.(map[string]any); ok {
			if name := nestedString(fields, "name"); name != "" {
				containers.Names = append(containers.Names, name)
			}
		}
	}
	if len(containers.Names) == 0 {
		return Containers{}
	}

	annotations, _, _ := unstructured.NestedStringMap(object, "metadata", "annotations")
	containers.Default = defaultContainer(containers.Names, annotations)
	return containers
}

// defaultContainer is the annotated container if the Pod has it, else the first.
func defaultContainer(names []string, annotations map[string]string) string {
	for _, name := range names {
		if name == annotations[defaultContainerAnnotation] {
			return name
		}
	}
	return names[0]
}
