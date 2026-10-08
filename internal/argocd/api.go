package argocd

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/awcodify/argoxd/internal/explorer"
)

// APISource loads resources from the Argo CD API.
type APISource struct {
	baseURL string
	token   string
	client  *http.Client
}

// NewAPISource creates an Argo CD API source.
func NewAPISource(server, token string, insecure bool) *APISource {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: insecure} //nolint:gosec // Explicit opt-in through --insecure.
	transport.ResponseHeaderTimeout = 15 * time.Second

	return &APISource{
		baseURL: strings.TrimRight(server, "/"),
		token:   token,
		client:  &http.Client{Transport: transport, Timeout: 15 * time.Second},
	}
}

// Load returns the Applications, Projects, and clusters visible to the API token.
func (s *APISource) Load(ctx context.Context) (explorer.Snapshot, error) {
	var applications applicationList
	if err := s.get(ctx, "/api/v1/applications", &applications); err != nil {
		return explorer.Snapshot{}, fmt.Errorf("load applications: %w", err)
	}

	var projects projectList
	if err := s.get(ctx, "/api/v1/projects", &projects); err != nil {
		return explorer.Snapshot{}, fmt.Errorf("load projects: %w", err)
	}

	var clusters clusterList
	if err := s.get(ctx, "/api/v1/clusters", &clusters); err != nil {
		return explorer.Snapshot{}, fmt.Errorf("load clusters: %w", err)
	}

	snapshot := explorer.Snapshot{
		Applications: make([]explorer.Application, 0, len(applications.Items)),
		Projects:     make([]explorer.Project, 0, len(projects.Items)),
		Clusters:     make([]explorer.Cluster, 0, len(clusters.Items)),
	}
	for _, application := range applications.Items {
		snapshot.Applications = append(snapshot.Applications, explorer.Application{
			Name:        application.Metadata.Name,
			Namespace:   application.Metadata.Namespace,
			Project:     application.Spec.Project,
			Sync:        application.Status.Sync.Status,
			Health:      application.Status.Health.Status,
			Revision:    application.Spec.targetRevision(),
			Destination: destination(application.Spec.Destination.Name, application.Spec.Destination.Server, application.Spec.Destination.Namespace),
			LastSync:    application.Status.OperationState.finishedAt(),
			Policy:      application.Spec.policy(),
			Operation:   application.Status.OperationState.operation(),
			Owner:       application.Metadata.applicationSetOwner(),
		})
		for _, condition := range application.Status.Conditions {
			last := &snapshot.Applications[len(snapshot.Applications)-1]
			last.Conditions = append(last.Conditions, explorer.Condition{Type: condition.Type, Message: condition.Message})
		}
	}
	for _, project := range projects.Items {
		snapshot.Projects = append(snapshot.Projects, explorer.Project{
			Name:        project.Metadata.Name,
			Description: project.Spec.Description,
		})
	}
	for _, cluster := range clusters.Items {
		snapshot.Clusters = append(snapshot.Clusters, explorer.Cluster{Name: cluster.Name, Server: cluster.Server})
	}

	// Argo CD may not have ApplicationSets, or the token may not list them.
	var applicationSets applicationSetList
	if err := s.get(ctx, "/api/v1/applicationsets", &applicationSets); err != nil && !unavailable(err) {
		return explorer.Snapshot{}, fmt.Errorf("load application sets: %w", err)
	}
	for _, applicationSet := range applicationSets.Items {
		snapshot.ApplicationSets = append(snapshot.ApplicationSets, explorer.ApplicationSet{
			Name: applicationSet.Metadata.Name, Namespace: applicationSet.Metadata.Namespace,
			Generators: generatorSummary(applicationSet.Spec.Generators),
			Problems:   applicationSetProblems(applicationSet.Status.Conditions),
		})
	}
	return snapshot, nil
}

func (s *APISource) get(ctx context.Context, path string, target any) error {
	return s.request(ctx, http.MethodGet, path, nil, target)
}

// LoadResourceTree returns the resources managed by an Application.
func (s *APISource) LoadResourceTree(ctx context.Context, application string) (explorer.ResourceTree, error) {
	var response resourceTreeResponse
	path := "/api/v1/applications/" + url.PathEscape(application) + "/resource-tree"
	if err := s.get(ctx, path, &response); err != nil {
		return explorer.ResourceTree{}, fmt.Errorf("load resource tree: %w", err)
	}

	tree := explorer.ResourceTree{
		Application: application,
		Nodes:       make([]explorer.ResourceNode, 0, len(response.Nodes)+len(response.OrphanedNodes)),
	}
	pruning := s.resourcesToPrune(ctx, application)
	for _, node := range response.Nodes {
		resource := node.resourceNode()
		resource.RequiresPruning = pruning[resource.Reference()]
		tree.Nodes = append(tree.Nodes, resource)
	}
	for _, node := range response.OrphanedNodes {
		resource := node.resourceNode()
		resource.Orphaned = true
		tree.Nodes = append(tree.Nodes, resource)
	}
	return tree, nil
}

// resourcesToPrune lists the resources a sync with prune would delete. The
// resource tree does not say, so it comes from the Application. It is extra
// information: when the Application cannot be read, nothing is marked.
func (s *APISource) resourcesToPrune(ctx context.Context, application string) map[explorer.ResourceReference]bool {
	var response applicationResources
	if err := s.get(ctx, "/api/v1/applications/"+url.PathEscape(application), &response); err != nil {
		return nil
	}
	pruning := make(map[explorer.ResourceReference]bool)
	for _, resource := range response.Status.Resources {
		if resource.RequiresPruning {
			pruning[explorer.ResourceReference{Group: resource.Group, Kind: resource.Kind, Namespace: resource.Namespace, Name: resource.Name}] = true
		}
	}
	return pruning
}

// SyncApplication starts a sync operation for an Application.
func (s *APISource) SyncApplication(ctx context.Context, application string, options SyncOptions) error {
	return s.sync(ctx, application, nil, options)
}

var _ ResourceSyncer = (*APISource)(nil)

// SyncResources starts a sync operation for only the listed resources.
func (s *APISource) SyncResources(ctx context.Context, application string, resources []explorer.ResourceReference, options SyncOptions) error {
	return s.sync(ctx, application, resources, options)
}

// sync posts the operation; no resources means the whole Application.
func (s *APISource) sync(ctx context.Context, application string, resources []explorer.ResourceReference, options SyncOptions) error {
	request := syncRequest{Prune: options.Prune, DryRun: options.DryRun}
	for _, resource := range resources {
		request.Resources = append(request.Resources, syncResource{
			Group: resource.Group, Kind: resource.Kind, Namespace: resource.Namespace, Name: resource.Name,
		})
	}
	body, err := json.Marshal(request)
	if err != nil {
		return fmt.Errorf("encode sync request: %w", err)
	}
	path := "/api/v1/applications/" + url.PathEscape(application) + "/sync"
	if err := s.request(ctx, http.MethodPost, path, bytes.NewBuffer(body), nil); err != nil {
		return fmt.Errorf("sync application: %w", err)
	}
	return nil
}

// RefreshApplication makes Argo CD compare the Application with Git again,
// bypassing its manifest cache.
func (s *APISource) RefreshApplication(ctx context.Context, application string) error {
	path := "/api/v1/applications/" + url.PathEscape(application) + "?refresh=hard"
	if err := s.get(ctx, path, nil); err != nil {
		return fmt.Errorf("refresh application: %w", err)
	}
	return nil
}

var _ SyncPolicySetter = (*APISource)(nil)

// SetSyncPolicy turns auto-sync on or off and sets its self-heal and prune
// options, with the same merge patch the Kubernetes source applies.
func (s *APISource) SetSyncPolicy(ctx context.Context, application string, policy explorer.SyncPolicy) error {
	patch, err := json.Marshal(syncPolicyPatch(policy))
	if err != nil {
		return fmt.Errorf("encode sync policy: %w", err)
	}
	body, err := json.Marshal(patchRequest{Patch: string(patch), PatchType: "merge"})
	if err != nil {
		return fmt.Errorf("encode sync policy request: %w", err)
	}
	if err := s.request(ctx, http.MethodPatch, "/api/v1/applications/"+url.PathEscape(application), bytes.NewBuffer(body), nil); err != nil {
		return fmt.Errorf("set sync policy: %w", err)
	}
	return nil
}

type patchRequest struct {
	Patch     string `json:"patch"`
	PatchType string `json:"patchType"`
}

// DeleteApplication deletes an Application and its managed resources.
func (s *APISource) DeleteApplication(ctx context.Context, application string) error {
	path := "/api/v1/applications/" + url.PathEscape(application)
	if err := s.request(ctx, http.MethodDelete, path, nil, nil); err != nil {
		return fmt.Errorf("delete application: %w", err)
	}
	return nil
}

// request sends a request and decodes the JSON response into target, when given.
func (s *APISource) request(ctx context.Context, method, path string, body *bytes.Buffer, target any) error {
	response, err := s.do(ctx, method, path, body)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if target != nil {
		if err := json.NewDecoder(response.Body).Decode(target); err != nil {
			return fmt.Errorf("decode response: %w", err)
		}
	}
	return nil
}

// do sends a request and returns the response of a successful call. The
// caller must close the response body.
func (s *APISource) do(ctx context.Context, method, path string, body *bytes.Buffer) (*http.Response, error) {
	return s.send(ctx, s.client, method, path, body)
}

// send is do through the given client.
func (s *APISource) send(ctx context.Context, client *http.Client, method, path string, body *bytes.Buffer) (*http.Response, error) {
	endpoint, err := url.Parse(s.baseURL + path)
	if err != nil {
		return nil, fmt.Errorf("parse endpoint: %w", err)
	}
	var reader io.Reader
	if body != nil {
		reader = body
	}
	request, err := http.NewRequestWithContext(ctx, method, endpoint.String(), reader)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	// Argo CD refuses any request that changes something without a JSON content
	// type, even one with no body, such as a delete.
	if body != nil || (method != http.MethodGet && method != http.MethodHead) {
		request.Header.Set("Content-Type", "application/json")
	}
	if s.token != "" {
		request.Header.Set("Authorization", "Bearer "+s.token)
	}

	response, err := client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("request %s: %w", endpoint.Path, err)
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		defer response.Body.Close()
		return nil, &httpStatusError{
			message: fmt.Sprintf("%s returned %s%s", endpoint.Path, response.Status, errorMessage(response.Body)),
			code:    response.StatusCode,
		}
	}
	return response, nil
}

// errorMessage reads the explanation Argo CD puts in an error response, if any.
func errorMessage(body io.Reader) string {
	var failure struct {
		Message string `json:"message"`
	}
	if err := json.NewDecoder(io.LimitReader(body, 64*1024)).Decode(&failure); err != nil || failure.Message == "" {
		return ""
	}
	return ": " + failure.Message
}

type syncRequest struct {
	Prune     bool           `json:"prune"`
	DryRun    bool           `json:"dryRun"`
	Resources []syncResource `json:"resources,omitempty"`
}

type syncResource struct {
	Group     string `json:"group"`
	Kind      string `json:"kind"`
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
}

type metadata struct {
	Name            string `json:"name"`
	Namespace       string `json:"namespace"`
	OwnerReferences []struct {
		Kind string `json:"kind"`
		Name string `json:"name"`
	} `json:"ownerReferences"`
}

// applicationSetOwner names the ApplicationSet that generated the resource.
func (m metadata) applicationSetOwner() string {
	for _, owner := range m.OwnerReferences {
		if owner.Kind == "ApplicationSet" {
			return owner.Name
		}
	}
	return ""
}

type applicationSetList struct {
	Items []struct {
		Metadata metadata `json:"metadata"`
		Spec     struct {
			Generators []any `json:"generators"`
		} `json:"spec"`
		Status struct {
			Conditions []appSetCondition `json:"conditions"`
		} `json:"status"`
	} `json:"items"`
}

type applicationList struct {
	Items []struct {
		Metadata metadata        `json:"metadata"`
		Spec     applicationSpec `json:"spec"`
		Status   struct {
			Sync struct {
				Status string `json:"status"`
			} `json:"sync"`
			Health struct {
				Status string `json:"status"`
			} `json:"health"`
			OperationState *operationState `json:"operationState"`
			Conditions     []struct {
				Type    string `json:"type"`
				Message string `json:"message"`
			} `json:"conditions"`
		} `json:"status"`
	} `json:"items"`
}

type operationState struct {
	Phase      string    `json:"phase"`
	Message    string    `json:"message"`
	StartedAt  time.Time `json:"startedAt"`
	FinishedAt time.Time `json:"finishedAt"`
	SyncResult struct {
		Revision  string `json:"revision"`
		Resources []struct {
			Group     string `json:"group"`
			Kind      string `json:"kind"`
			Namespace string `json:"namespace"`
			Name      string `json:"name"`
			Status    string `json:"status"`
			Message   string `json:"message"`
			HookType  string `json:"hookType"`
			HookPhase string `json:"hookPhase"`
			SyncPhase string `json:"syncPhase"`
		} `json:"resources"`
	} `json:"syncResult"`
}

// finishedAt is when the last sync ended; zero if there was none.
func (s *operationState) finishedAt() time.Time {
	if s == nil {
		return time.Time{}
	}
	return s.FinishedAt
}

// operation converts the state of an Application's last sync, or nil if it has none.
func (s *operationState) operation() *explorer.Operation {
	if s == nil {
		return nil
	}
	operation := &explorer.Operation{
		Phase: s.Phase, Message: s.Message, Revision: s.SyncResult.Revision,
		StartedAt: s.StartedAt, FinishedAt: s.FinishedAt,
	}
	for _, result := range s.SyncResult.Resources {
		operation.Results = append(operation.Results, explorer.OperationResult{
			Group: result.Group, Kind: result.Kind, Namespace: result.Namespace, Name: result.Name,
			Status: result.Status, Message: result.Message,
			HookType: result.HookType, HookPhase: result.HookPhase, SyncPhase: result.SyncPhase,
		})
	}
	return operation
}

type applicationSource struct {
	RepoURL        string `json:"repoURL"`
	TargetRevision string `json:"targetRevision"`
}

type applicationSpec struct {
	Project     string              `json:"project"`
	Source      *applicationSource  `json:"source"`
	Sources     []applicationSource `json:"sources"`
	Destination struct {
		Name      string `json:"name"`
		Server    string `json:"server"`
		Namespace string `json:"namespace"`
	} `json:"destination"`
	SyncPolicy struct {
		Automated *struct {
			Prune    bool `json:"prune"`
			SelfHeal bool `json:"selfHeal"`
		} `json:"automated"`
	} `json:"syncPolicy"`
}

// policy reads how an Application syncs on its own.
func (s applicationSpec) policy() explorer.SyncPolicy {
	automated := s.SyncPolicy.Automated
	if automated == nil {
		return explorer.SyncPolicy{}
	}
	return explorer.SyncPolicy{Automated: true, SelfHeal: automated.SelfHeal, Prune: automated.Prune}
}

// targetRevision reads the revision of a single-source Application, or of the
// first source of a multi-source one.
func (s applicationSpec) targetRevision() string {
	if s.Source != nil {
		return s.Source.TargetRevision
	}
	if len(s.Sources) > 0 {
		return s.Sources[0].TargetRevision
	}
	return ""
}

type projectList struct {
	Items []struct {
		Metadata metadata `json:"metadata"`
		Spec     struct {
			Description string `json:"description"`
		} `json:"spec"`
	} `json:"items"`
}

type cluster struct {
	Name   string `json:"name"`
	Server string `json:"server"`
}

type clusterList struct {
	Items []cluster `json:"items"`
}

type resourceTreeResponse struct {
	Nodes         []resourceTreeNode `json:"nodes"`
	OrphanedNodes []resourceTreeNode `json:"orphanedNodes"`
}

type resourceTreeNode struct {
	Group     string `json:"group"`
	Version   string `json:"version"`
	Kind      string `json:"kind"`
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
	UID       string `json:"uid"`
	Status    string `json:"status"`
	Health    struct {
		Status string `json:"status"`
	} `json:"health"`
	ParentRefs []struct {
		Group     string `json:"group"`
		Kind      string `json:"kind"`
		Namespace string `json:"namespace"`
		Name      string `json:"name"`
	} `json:"parentRefs"`
}

// resourceNode converts a node of the resource tree.
func (n resourceTreeNode) resourceNode() explorer.ResourceNode {
	parents := make([]explorer.ResourceReference, 0, len(n.ParentRefs))
	for _, parent := range n.ParentRefs {
		parents = append(parents, explorer.ResourceReference{
			Group: parent.Group, Kind: parent.Kind, Namespace: parent.Namespace, Name: parent.Name,
		})
	}
	return explorer.ResourceNode{
		Group: n.Group, Version: n.Version, Kind: n.Kind, Namespace: n.Namespace, Name: n.Name,
		UID: n.UID, Sync: n.Status, Health: n.Health.Status, Parents: parents,
	}
}

// applicationResources are the resources in an Application's status, which
// know whether a sync with prune would delete them.
type applicationResources struct {
	Status struct {
		Resources []struct {
			Group           string `json:"group"`
			Kind            string `json:"kind"`
			Namespace       string `json:"namespace"`
			Name            string `json:"name"`
			RequiresPruning bool   `json:"requiresPruning"`
		} `json:"resources"`
	} `json:"status"`
}
