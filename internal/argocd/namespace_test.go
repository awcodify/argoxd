package argocd

import (
	"context"
	"slices"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic/fake"
	typedfake "k8s.io/client-go/kubernetes/fake"
)

func TestKubernetesSourceListsAndSwitchesNamespaces(t *testing.T) {
	typed := typedfake.NewClientset(
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "gitops"}},
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "argocd"}},
	)
	source := &KubernetesSource{
		namespace: "argocd",
		context:   "development",
		clients: func() (kubernetesClients, error) {
			return kubernetesClients{dynamic: fake.NewSimpleDynamicClient(runtime.NewScheme()), typed: typed}, nil
		},
	}

	namespaces, err := source.Namespaces(context.Background())
	if err != nil || !slices.Equal(namespaces, []string{"argocd", "gitops"}) {
		t.Fatalf("Namespaces() = %v, %v", namespaces, err)
	}
	switched, ok := source.WithNamespace("gitops").(*KubernetesSource)
	if !ok || switched.namespace != "gitops" || switched.context != "development" {
		t.Fatalf("WithNamespace() = %#v", switched)
	}
	if source.namespace != "argocd" {
		t.Fatalf("WithNamespace changed the original source to %q", source.namespace)
	}
}

var _ NamespaceSwitcher = (*KubernetesSource)(nil)
