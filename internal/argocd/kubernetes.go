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
	apierrors "k8s.io/apimachinery/pkg/api/errors"
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
	applicationSetsResource = schema.GroupVersionResource{
		Group: "argoproj.io", Version: "v1alpha1", Resource: "applicationsets",
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
	snapshot := snapshotFromKubernetesResources(applications.Items, projects.Items, clusters.Items)

	// Argo CD may not have ApplicationSets, or the identity may not list them.
	applicationSets, err := client.Resource(applicationSetsResource).Namespace(s.namespace).List(ctx, metav1.ListOptions{})
	if err != nil && !apierrors.IsNotFound(err) && !apierrors.IsForbidden(err) {
		return explorer.Snapshot{}, fmt.Errorf("list application sets: %w", err)
	}
	if err == nil {
		snapshot.ApplicationSets = applicationSetsFromKubernetesResources(applicationSets.Items)
	}
	return snapshot, nil
}

func applicationSetsFromKubernetesResources(items []unstructured.Unstructured) []explorer.ApplicationSet {
	applicationSets := make([]explorer.ApplicationSet, 0, len(items))
	for _, item := range items {
		generators, _, _ := unstructured.NestedSlice(item.Object, "spec", "generators")
		var conditions []appSetCondition
		statuses, _, _ := unstructured.NestedSlice(item.Object, "status", "conditions")
		for _, status := range statuses {
			if fields, ok := status.(map[string]any); ok {
				conditions = append(conditions, appSetCondition{
					Type: nestedString(fields, "type"), Status: nestedString(fields, "status"), Message: nestedString(fields, "message"),
				})
			}
		}
		applicationSets = append(applicationSets, explorer.ApplicationSet{
			Name: item.GetName(), Namespace: item.GetNamespace(),
			Generators: generatorSummary(generators), Problems: applicationSetProblems(conditions),
		})
	}
	return applicationSets
}

// applicationSetOwner names the ApplicationSet that generated an Application.
func applicationSetOwner(application map[string]any) string {
	owners, _, _ := unstructured.NestedSlice(application, "metadata", "ownerReferences")
	for _, owner := range owners {
		if fields, ok := owner.(map[string]any); ok && nestedString(fields, "kind") == "ApplicationSet" {
			return nestedString(fields, "name")
		}
	}
	return ""
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
	return s.sync(ctx, application, nil, options)
}

var _ ResourceSyncer = (*KubernetesSource)(nil)

// SyncResources requests a sync of only the listed resources.
func (s *KubernetesSource) SyncResources(ctx context.Context, application string, resources []explorer.ResourceReference, options SyncOptions) error {
	return s.sync(ctx, application, resources, options)
}

// sync writes the operation; no resources means the whole Application.
func (s *KubernetesSource) sync(ctx context.Context, application string, resources []explorer.ResourceReference, options SyncOptions) error {
	operation := map[string]any{"prune": options.Prune, "dryRun": options.DryRun}
	if len(resources) > 0 {
		chosen := make([]any, 0, len(resources))
		for _, resource := range resources {
			chosen = append(chosen, map[string]any{
				"group": resource.Group, "kind": resource.Kind, "namespace": resource.Namespace, "name": resource.Name,
			})
		}
		operation["resources"] = chosen
	}
	patch := map[string]any{"operation": map[string]any{"sync": operation}}
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

var _ SyncPolicySetter = (*KubernetesSource)(nil)

// SetSyncPolicy turns auto-sync on or off and sets its self-heal and prune options.
func (s *KubernetesSource) SetSyncPolicy(ctx context.Context, application string, policy explorer.SyncPolicy) error {
	if err := s.patchApplication(ctx, application, syncPolicyPatch(policy)); err != nil {
		return fmt.Errorf("set sync policy of application %q: %w", application, err)
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
			Conditions:  conditions(application.Object),
			Policy:      syncPolicy(application.Object),
			Operation:   lastOperation(application.Object),
			Owner:       applicationSetOwner(application.Object),
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

// conditions reads the warnings and errors in an Application's status.
func conditions(application map[string]any) []explorer.Condition {
	items, _, _ := unstructured.NestedSlice(application, "status", "conditions")
	var conditions []explorer.Condition
	for _, item := range items {
		if object, ok := item.(map[string]any); ok {
			conditions = append(conditions, explorer.Condition{Type: nestedString(object, "type"), Message: nestedString(object, "message")})
		}
	}
	return conditions
}

// lastOperation reads the outcome of an Application's last sync, or nil if it
// has none.
func lastOperation(application map[string]any) *explorer.Operation {
	state, found, _ := unstructured.NestedMap(application, "status", "operationState")
	if !found {
		return nil
	}
	operation := &explorer.Operation{
		Phase:      nestedString(state, "phase"),
		Message:    nestedString(state, "message"),
		Revision:   nestedString(state, "syncResult", "revision"),
		StartedAt:  parseTime(nestedString(state, "startedAt")),
		FinishedAt: parseTime(nestedString(state, "finishedAt")),
	}
	results, _, _ := unstructured.NestedSlice(state, "syncResult", "resources")
	for _, item := range results {
		if result, ok := item.(map[string]any); ok {
			operation.Results = append(operation.Results, explorer.OperationResult{
				Group: nestedString(result, "group"), Kind: nestedString(result, "kind"),
				Namespace: nestedString(result, "namespace"), Name: nestedString(result, "name"),
				Status: nestedString(result, "status"), Message: nestedString(result, "message"),
				HookType: nestedString(result, "hookType"), HookPhase: nestedString(result, "hookPhase"),
				SyncPhase: nestedString(result, "syncPhase"),
			})
		}
	}
	return operation
}

// syncPolicy reads how an Application syncs on its own. An empty automated
// block still means automated sync.
func syncPolicy(application map[string]any) explorer.SyncPolicy {
	automated, found, _ := unstructured.NestedMap(application, "spec", "syncPolicy", "automated")
	if !found {
		return explorer.SyncPolicy{}
	}
	prune, _, _ := unstructured.NestedBool(automated, "prune")
	selfHeal, _, _ := unstructured.NestedBool(automated, "selfHeal")
	return explorer.SyncPolicy{Automated: true, SelfHeal: selfHeal, Prune: prune}
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

			RequiresPruning: requiresPruning(object),
		})
	}
	return tree
}

func requiresPruning(resource map[string]any) bool {
	required, _, _ := unstructured.NestedBool(resource, "requiresPruning")
	return required
}
