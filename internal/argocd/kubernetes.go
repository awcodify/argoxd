package argocd

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/awcodify/argoxd/internal/explorer"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
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
	namespace string
	clients   func() (kubernetesClients, error)
}

// kubernetesClients are created once and shared by every request.
type kubernetesClients struct {
	dynamic dynamic.Interface
	typed   kubernetes.Interface
}

// NewKubernetesSource creates a source backed by an Argo CD installation in Kubernetes.
func NewKubernetesSource(kubeconfig, context, namespace string) *KubernetesSource {
	return &KubernetesSource{
		namespace: namespace,
		clients: sync.OnceValues(func() (kubernetesClients, error) {
			return connect(kubeconfig, context)
		}),
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

// LoadResourceTree returns the resources reported in the Application status
// together with the ReplicaSets, Jobs and Pods they own.
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
	return attachOwnedResources(tree, s.listOwnedResources(ctx, client, tree)), nil
}

// listOwnedResources lists the owned kinds in every namespace the Application
// deploys to. Kinds the identity may not list are skipped, not reported as errors.
func (s *KubernetesSource) listOwnedResources(ctx context.Context, client dynamic.Interface, tree explorer.ResourceTree) []unstructured.Unstructured {
	namespaces := make(map[string]bool)
	for _, node := range tree.Nodes {
		if node.Namespace != "" {
			namespaces[node.Namespace] = true
		}
	}
	var objects []unstructured.Unstructured
	for namespace := range namespaces {
		for _, resource := range ownedResources {
			list, err := client.Resource(resource).Namespace(namespace).List(ctx, metav1.ListOptions{})
			if err != nil {
				continue
			}
			objects = append(objects, list.Items...)
		}
	}
	return objects
}

// SyncApplication requests a sync through the Application operation field.
func (s *KubernetesSource) SyncApplication(ctx context.Context, application string, options SyncOptions) error {
	patch := map[string]any{"operation": map[string]any{"sync": map[string]any{
		"prune":  options.Prune,
		"dryRun": options.DryRun,
	}}}
	if err := s.patchApplication(ctx, application, patch); err != nil {
		return fmt.Errorf("sync application %q: %w", application, err)
	}
	return nil
}

// RefreshApplication asks Argo CD to compare the Application with Git again,
// bypassing its manifest cache.
func (s *KubernetesSource) RefreshApplication(ctx context.Context, application string) error {
	patch := map[string]any{"metadata": map[string]any{"annotations": map[string]any{
		"argocd.argoproj.io/refresh": "hard",
	}}}
	if err := s.patchApplication(ctx, application, patch); err != nil {
		return fmt.Errorf("refresh application %q: %w", application, err)
	}
	return nil
}

func (s *KubernetesSource) patchApplication(ctx context.Context, application string, patch map[string]any) error {
	client, err := s.client()
	if err != nil {
		return err
	}
	body, err := json.Marshal(patch)
	if err != nil {
		return err
	}
	_, err = client.Resource(applicationsResource).Namespace(s.namespace).Patch(ctx, application, types.MergePatchType, body, metav1.PatchOptions{})
	return err
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
	clients, err := s.clients()
	return clients.dynamic, err
}

func connect(kubeconfig, context string) (kubernetesClients, error) {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	loadingRules.ExplicitPath = kubeconfig
	clientConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		loadingRules,
		&clientcmd.ConfigOverrides{CurrentContext: context},
	)
	config, err := clientConfig.ClientConfig()
	if err != nil {
		return kubernetesClients{}, fmt.Errorf("load kubeconfig: %w", err)
	}
	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		return kubernetesClients{}, fmt.Errorf("create Kubernetes client: %w", err)
	}
	typedClient, err := kubernetes.NewForConfig(config)
	if err != nil {
		return kubernetesClients{}, fmt.Errorf("create Kubernetes client: %w", err)
	}
	return kubernetesClients{dynamic: dynamicClient, typed: typedClient}, nil
}

func snapshotFromKubernetesResources(applications, projects, clusters []unstructured.Unstructured) explorer.Snapshot {
	snapshot := explorer.Snapshot{
		Applications: make([]explorer.Application, 0, len(applications)),
		Projects:     make([]explorer.Project, 0, len(projects)),
		Clusters:     make([]explorer.Cluster, 0, len(clusters)),
	}
	for _, application := range applications {
		snapshot.Applications = append(snapshot.Applications, explorer.Application{
			Name:        application.GetName(),
			Namespace:   application.GetNamespace(),
			Project:     nestedString(application.Object, "spec", "project"),
			Sync:        nestedString(application.Object, "status", "sync", "status"),
			Health:      nestedString(application.Object, "status", "health", "status"),
			Revision:    targetRevision(application.Object),
			Destination: destination(nestedString(application.Object, "spec", "destination", "name"), nestedString(application.Object, "spec", "destination", "server"), nestedString(application.Object, "spec", "destination", "namespace")),
			LastSync:    parseTime(nestedString(application.Object, "status", "operationState", "finishedAt")),
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

// targetRevision reads the revision of a single-source Application, or of the
// first source of a multi-source one.
func targetRevision(application map[string]any) string {
	if revision := nestedString(application, "spec", "source", "targetRevision"); revision != "" {
		return revision
	}
	sources, _, _ := unstructured.NestedSlice(application, "spec", "sources")
	if len(sources) > 0 {
		if source, ok := sources[0].(map[string]any); ok {
			return nestedString(source, "targetRevision")
		}
	}
	return ""
}

// destination describes where an Application deploys, e.g. "in-cluster/store".
func destination(name, server, namespace string) string {
	target := name
	if target == "" {
		target = server
	}
	return strings.Trim(target+"/"+namespace, "/")
}

func parseTime(value string) time.Time {
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return time.Time{}
	}
	return parsed
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
			Version:   nestedString(object, "version"),
			Kind:      nestedString(object, "kind"),
			Namespace: nestedString(object, "namespace"),
			Name:      nestedString(object, "name"),
			Sync:      nestedString(object, "status"),
			Health:    nestedString(object, "health", "status"),
		})
	}
	return tree
}
