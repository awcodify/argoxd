package argocd

import (
	"context"
	"fmt"
	"time"

	"github.com/awcodify/argoxd/internal/explorer"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ EventLister = (*KubernetesSource)(nil)

// clusterScopedEventNamespace is where Kubernetes records the events of
// resources that have no namespace.
const clusterScopedEventNamespace = "default"

// ApplicationEvents returns the events recorded for the Application.
func (s *KubernetesSource) ApplicationEvents(ctx context.Context, application string) (string, error) {
	node := explorer.ResourceNode{Kind: "Application", Namespace: s.namespace, Name: application}
	return s.eventsOf(ctx, []explorer.ResourceNode{node})
}

// ResourceEvents returns the events of a resource and of the resources under it.
func (s *KubernetesSource) ResourceEvents(ctx context.Context, _ string, resource explorer.ResourceNode) (string, error) {
	return s.eventsOf(ctx, withDescendants(resource))
}

// eventsOf lists the events of the given resources, reading each namespace once.
func (s *KubernetesSource) eventsOf(ctx context.Context, nodes []explorer.ResourceNode) (string, error) {
	clients, err := s.clients()
	if err != nil {
		return "", err
	}
	type object struct{ namespace, kind, name string }
	wanted := make(map[object]bool, len(nodes))
	namespaces := make(map[string]bool)
	for _, node := range nodes {
		namespace := node.Namespace
		if namespace == "" {
			namespace = clusterScopedEventNamespace
		}
		wanted[object{namespace, node.Kind, node.Name}] = true
		namespaces[namespace] = true
	}

	var events []corev1.Event
	for namespace := range namespaces {
		list, err := clients.typed.CoreV1().Events(namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			return "", fmt.Errorf("load events in %s: %w", namespace, err)
		}
		for _, event := range list.Items {
			if wanted[object{namespace, event.InvolvedObject.Kind, event.InvolvedObject.Name}] {
				events = append(events, event)
			}
		}
	}
	return formatEvents(events, time.Now()), nil
}
