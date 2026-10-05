// Package argocd provides resource sources backed by Argo CD.
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

// Source loads the resources shown by the explorer.
type Source interface {
	Load(ctx context.Context) (explorer.Snapshot, error)
}

// ApplicationOperator supports interactive application operations.
type ApplicationOperator interface {
	LoadResourceTree(ctx context.Context, application string) (explorer.ResourceTree, error)
	SyncApplication(ctx context.Context, application string) error
	DeleteApplication(ctx context.Context, application string) error
}

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
			Name:      application.Metadata.Name,
			Namespace: application.Metadata.Namespace,
			Project:   application.Spec.Project,
			Sync:      application.Status.Sync.Status,
			Health:    application.Status.Health.Status,
		})
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
		Nodes:       make([]explorer.ResourceNode, 0, len(response.Nodes)),
	}
	for _, node := range response.Nodes {
		parents := make([]explorer.ResourceReference, 0, len(node.ParentRefs))
		for _, parent := range node.ParentRefs {
			parents = append(parents, explorer.ResourceReference{
				Group: parent.Group, Kind: parent.Kind, Namespace: parent.Namespace, Name: parent.Name,
			})
		}
		tree.Nodes = append(tree.Nodes, explorer.ResourceNode{
			Group:     node.Group,
			Kind:      node.Kind,
			Namespace: node.Namespace,
			Name:      node.Name,
			Sync:      node.Status,
			Health:    node.Health.Status,
			Parents:   parents,
		})
	}
	return tree, nil
}

// SyncApplication starts a sync operation for an Application.
func (s *APISource) SyncApplication(ctx context.Context, application string) error {
	path := "/api/v1/applications/" + url.PathEscape(application) + "/sync"
	if err := s.request(ctx, http.MethodPost, path, bytes.NewBufferString("{}"), nil); err != nil {
		return fmt.Errorf("sync application: %w", err)
	}
	return nil
}

// DeleteApplication deletes an Application and its managed resources.
func (s *APISource) DeleteApplication(ctx context.Context, application string) error {
	path := "/api/v1/applications/" + url.PathEscape(application)
	if err := s.request(ctx, http.MethodDelete, path, nil, nil); err != nil {
		return fmt.Errorf("delete application: %w", err)
	}
	return nil
}

func (s *APISource) request(ctx context.Context, method, path string, body *bytes.Buffer, target any) error {
	endpoint, err := url.Parse(s.baseURL + path)
	if err != nil {
		return fmt.Errorf("parse endpoint: %w", err)
	}
	var reader io.Reader
	if body != nil {
		reader = body
	}
	request, err := http.NewRequestWithContext(ctx, method, endpoint.String(), reader)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	if s.token != "" {
		request.Header.Set("Authorization", "Bearer "+s.token)
	}

	response, err := s.client.Do(request)
	if err != nil {
		return fmt.Errorf("request %s: %w", path, err)
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("%s returned %s", path, response.Status)
	}
	if target != nil {
		if err := json.NewDecoder(response.Body).Decode(target); err != nil {
			return fmt.Errorf("decode response: %w", err)
		}
	}
	return nil
}

type metadata struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
}

type applicationList struct {
	Items []struct {
		Metadata metadata `json:"metadata"`
		Spec     struct {
			Project string `json:"project"`
		} `json:"spec"`
		Status struct {
			Sync struct {
				Status string `json:"status"`
			} `json:"sync"`
			Health struct {
				Status string `json:"status"`
			} `json:"health"`
		} `json:"status"`
	} `json:"items"`
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
	Nodes []struct {
		Group     string `json:"group"`
		Kind      string `json:"kind"`
		Namespace string `json:"namespace"`
		Name      string `json:"name"`
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
	} `json:"nodes"`
}
