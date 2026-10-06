package argocd

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/awcodify/argoxd/internal/explorer"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
)

var _ ResourceActor = (*KubernetesSource)(nil)

// restartedAtAnnotation is what `kubectl rollout restart` stamps on a workload's
// Pod template to make it roll out again.
const restartedAtAnnotation = "kubectl.kubernetes.io/restartedAt"

// RestartResource rolls out the Pods of a Deployment, StatefulSet or DaemonSet again.
func (s *KubernetesSource) RestartResource(ctx context.Context, _ string, resource explorer.ResourceNode) error {
	if !Restartable(resource.Kind) {
		return fmt.Errorf("%s/%s cannot be restarted: only Deployments, StatefulSets and DaemonSets can", resource.Kind, resource.Name)
	}
	client, err := s.client()
	if err != nil {
		return err
	}
	patch := map[string]any{"spec": map[string]any{"template": map[string]any{"metadata": map[string]any{
		"annotations": map[string]any{restartedAtAnnotation: time.Now().UTC().Format(time.RFC3339)},
	}}}}
	body, err := json.Marshal(patch)
	if err != nil {
		return err
	}
	if _, err := resourceClient(client, resource).Patch(ctx, resource.Name, types.MergePatchType, body, metav1.PatchOptions{}); err != nil {
		return fmt.Errorf("restart %s/%s: %w", resource.Kind, resource.Name, err)
	}
	return nil
}

// DeleteResource deletes a resource.
func (s *KubernetesSource) DeleteResource(ctx context.Context, _ string, resource explorer.ResourceNode) error {
	client, err := s.client()
	if err != nil {
		return err
	}
	if err := resourceClient(client, resource).Delete(ctx, resource.Name, metav1.DeleteOptions{}); err != nil {
		return fmt.Errorf("delete %s/%s: %w", resource.Kind, resource.Name, err)
	}
	return nil
}

// resourceClient addresses the resource's kind, within its namespace if it has one.
func resourceClient(client dynamic.Interface, resource explorer.ResourceNode) dynamic.ResourceInterface {
	plural, _ := meta.UnsafeGuessKindToResource(schema.GroupVersionKind{Group: resource.Group, Version: resource.Version, Kind: resource.Kind})
	if resource.Namespace == "" {
		return client.Resource(plural)
	}
	return client.Resource(plural).Namespace(resource.Namespace)
}
