package argocd

import (
	"context"
	"encoding/base64"
	"fmt"

	"github.com/awcodify/argoxd/internal/explorer"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/clientcmd"
)

var (
	applicationsResource = schema.GroupVersionResource{
		Group: "argoproj.io", Version: "v1alpha1", Resource: "applications",
	}
	projectsResource = schema.GroupVersionResource{
		Group: "argoproj.io", Version: "v1alpha1", Resource: "appprojects",
	}
	secretsResource = schema.GroupVersionResource{
		Version: "v1", Resource: "secrets",
	}
)

// KubernetesSource loads Argo CD custom resources from a kubeconfig context.
type KubernetesSource struct {
	kubeconfig string
	context    string
	namespace  string
}

// NewKubernetesSource creates a source backed by an Argo CD installation in Kubernetes.
func NewKubernetesSource(kubeconfig, context, namespace string) *KubernetesSource {
	return &KubernetesSource{
		kubeconfig: kubeconfig,
		context:    context,
		namespace:  namespace,
	}
}

// Load lists Applications, AppProjects, and cluster secrets.
func (s *KubernetesSource) Load(ctx context.Context) (explorer.Snapshot, error) {
	client, err := s.client()
	if err != nil {
		return explorer.Snapshot{}, err
	}

	applications, err := client.Resource(applicationsResource).Namespace(s.namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return explorer.Snapshot{}, fmt.Errorf("list applications: %w", err)
	}
	projects, err := client.Resource(projectsResource).Namespace(s.namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return explorer.Snapshot{}, fmt.Errorf("list projects: %w", err)
	}
	clusters, err := client.Resource(secretsResource).Namespace(s.namespace).List(ctx, metav1.ListOptions{
		LabelSelector: "argocd.argoproj.io/secret-type=cluster",
	})
	if err != nil {
		return explorer.Snapshot{}, fmt.Errorf("list clusters: %w", err)
	}
	return snapshotFromKubernetesResources(applications.Items, projects.Items, clusters.Items), nil
}

// LoadResourceTree returns resources reported in the Application status.
func (s *KubernetesSource) LoadResourceTree(ctx context.Context, application string) (explorer.ResourceTree, error) {
	client, err := s.client()
	if err != nil {
		return explorer.ResourceTree{}, err
	}
	resource, err := client.Resource(applicationsResource).Namespace(s.namespace).Get(ctx, application, metav1.GetOptions{})
	if err != nil {
		return explorer.ResourceTree{}, fmt.Errorf("get application %q: %w", application, err)
	}
	tree := resourceTreeFromApplication(*resource)
	tree.Application = application
	return tree, nil
}

// SyncApplication requests a sync through the Application operation field.
func (s *KubernetesSource) SyncApplication(ctx context.Context, application string) error {
	client, err := s.client()
	if err != nil {
		return err
	}
	_, err = client.Resource(applicationsResource).Namespace(s.namespace).Patch(
		ctx,
		application,
		types.MergePatchType,
		[]byte(`{"operation":{"sync":{}}}`),
		metav1.PatchOptions{},
	)
	if err != nil {
		return fmt.Errorf("sync application %q: %w", application, err)
	}
	return nil
}

// DeleteApplication deletes an Application and its managed resources.
func (s *KubernetesSource) DeleteApplication(ctx context.Context, application string) error {
	client, err := s.client()
	if err != nil {
		return err
	}
	propagation := metav1.DeletePropagationForeground
	if err := client.Resource(applicationsResource).Namespace(s.namespace).Delete(ctx, application, metav1.DeleteOptions{
		PropagationPolicy: &propagation,
	}); err != nil {
		return fmt.Errorf("delete application %q: %w", application, err)
	}
	return nil
}

func (s *KubernetesSource) client() (dynamic.Interface, error) {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	loadingRules.ExplicitPath = s.kubeconfig
	clientConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		loadingRules,
		&clientcmd.ConfigOverrides{CurrentContext: s.context},
	)
	config, err := clientConfig.ClientConfig()
	if err != nil {
		return nil, fmt.Errorf("load kubeconfig: %w", err)
	}
	client, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("create Kubernetes client: %w", err)
	}
	return client, nil
}

func snapshotFromKubernetesResources(applications, projects, clusters []unstructured.Unstructured) explorer.Snapshot {
	snapshot := explorer.Snapshot{
		Applications: make([]explorer.Application, 0, len(applications)),
		Projects:     make([]explorer.Project, 0, len(projects)),
		Clusters:     make([]explorer.Cluster, 0, len(clusters)),
	}
	for _, application := range applications {
		snapshot.Applications = append(snapshot.Applications, explorer.Application{
			Name:      application.GetName(),
			Namespace: application.GetNamespace(),
			Project:   nestedString(application.Object, "spec", "project"),
			Sync:      nestedString(application.Object, "status", "sync", "status"),
			Health:    nestedString(application.Object, "status", "health", "status"),
		})
	}
	for _, project := range projects {
		snapshot.Projects = append(snapshot.Projects, explorer.Project{
			Name:        project.GetName(),
			Description: nestedString(project.Object, "spec", "description"),
		})
	}
	for _, cluster := range clusters {
		snapshot.Clusters = append(snapshot.Clusters, explorer.Cluster{
			Name:   secretValue(cluster.Object, "name"),
			Server: secretValue(cluster.Object, "server"),
		})
	}
	return snapshot
}

func nestedString(object map[string]any, fields ...string) string {
	value, found, err := unstructured.NestedString(object, fields...)
	if err != nil || !found {
		return ""
	}
	return value
}

func secretValue(object map[string]any, key string) string {
	value := nestedString(object, "data", key)
	decoded, err := base64.StdEncoding.DecodeString(value)
	if err == nil {
		return string(decoded)
	}
	return value
}

func resourceTreeFromApplication(application unstructured.Unstructured) explorer.ResourceTree {
	resources, found, err := unstructured.NestedSlice(application.Object, "status", "resources")
	if err != nil || !found {
		return explorer.ResourceTree{}
	}

	tree := explorer.ResourceTree{Nodes: make([]explorer.ResourceNode, 0, len(resources))}
	for _, resource := range resources {
		object, ok := resource.(map[string]any)
		if !ok {
			continue
		}
		tree.Nodes = append(tree.Nodes, explorer.ResourceNode{
			Group:     nestedString(object, "group"),
			Kind:      nestedString(object, "kind"),
			Namespace: nestedString(object, "namespace"),
			Name:      nestedString(object, "name"),
			Sync:      nestedString(object, "status"),
			Health:    nestedString(object, "health", "status"),
		})
	}
	return tree
}
