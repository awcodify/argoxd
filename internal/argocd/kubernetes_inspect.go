package argocd

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/awcodify/argoxd/internal/explorer"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/yaml"
)

// errDiffNeedsAPI explains why diffs are unavailable: the desired state is
// rendered by the Argo CD repo server, which only the API exposes.
var errDiffNeedsAPI = errors.New("diffs need the Argo CD API; run argoxd with --source api")

// ResourceManifest returns the live manifest of a resource as YAML.
func (s *KubernetesSource) ResourceManifest(ctx context.Context, _ string, resource explorer.ResourceNode) (string, error) {
	client, err := s.client()
	if err != nil {
		return "", err
	}
	plural, _ := meta.UnsafeGuessKindToResource(schema.GroupVersionKind{Group: resource.Group, Version: resource.Version, Kind: resource.Kind})
	var object *unstructured.Unstructured
	if resource.Namespace == "" {
		object, err = client.Resource(plural).Get(ctx, resource.Name, metav1.GetOptions{})
	} else {
		object, err = client.Resource(plural).Namespace(resource.Namespace).Get(ctx, resource.Name, metav1.GetOptions{})
	}
	if err != nil {
		return "", fmt.Errorf("get %s/%s: %w", resource.Kind, resource.Name, err)
	}
	unstructured.RemoveNestedField(object.Object, "metadata", "managedFields")
	manifest, err := yaml.Marshal(object.Object)
	if err != nil {
		return "", fmt.Errorf("encode %s/%s: %w", resource.Kind, resource.Name, err)
	}
	return string(manifest), nil
}

// ResourceDiff is not available from Kubernetes alone.
func (s *KubernetesSource) ResourceDiff(context.Context, string, explorer.ResourceNode) (string, error) {
	return "", errDiffNeedsAPI
}

// ResourceLogs returns the most recent log lines of a Pod's container. An empty
// container means its default one; AllContainers groups each container's lines
// under its name.
func (s *KubernetesSource) ResourceLogs(ctx context.Context, _ string, pod explorer.ResourceNode, container string) (string, error) {
	clients, err := s.clients()
	if err != nil {
		return "", err
	}
	var lines []string
	for _, name := range containersToRead(readPod(ctx, clients.typed, pod.Namespace, pod.Name, container), container) {
		tail := int64(logTailLines)
		logs, err := clients.typed.CoreV1().Pods(pod.Namespace).GetLogs(pod.Name, &corev1.PodLogOptions{Container: name, TailLines: &tail}).DoRaw(ctx)
		if err != nil {
			return "", fmt.Errorf("load logs for %s: %w", pod.Name, err)
		}
		text := strings.TrimRight(string(logs), "\n")
		if container != AllContainers {
			return text, nil
		}
		if text != "" {
			for _, line := range strings.Split(text, "\n") {
				lines = append(lines, name+LogSeparator+line)
			}
		}
	}
	return strings.Join(lines, "\n"), nil
}
