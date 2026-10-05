package argocd

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAPISourceLoadsArgoCDResources(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer token" {
			t.Fatalf("Authorization = %q, want bearer token", got)
		}

		switch r.URL.Path {
		case "/api/v1/applications":
			_, _ = w.Write([]byte(`{"items":[{"metadata":{"name":"payments","namespace":"argocd"},"spec":{"project":"platform"},"status":{"sync":{"status":"Synced"},"health":{"status":"Healthy"}}}]}`))
		case "/api/v1/projects":
			_, _ = w.Write([]byte(`{"items":[{"metadata":{"name":"platform"},"spec":{"description":"Platform applications"}}]}`))
		case "/api/v1/clusters":
			_, _ = w.Write([]byte(`{"items":[{"name":"production","server":"https://kubernetes.example.com"}]}`))
		case "/api/v1/applications/payments/resource-tree":
			_, _ = w.Write([]byte(`{"nodes":[{"kind":"Deployment","namespace":"default","name":"payments","health":{"status":"Healthy"},"status":"Synced","parentRefs":[{"kind":"Service","namespace":"default","name":"payments"}]}]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	source := NewAPISource(server.URL, "token", false)
	snapshot, err := source.Load(context.Background())
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if got := snapshot.Applications[0].Name; got != "payments" {
		t.Fatalf("application name = %q, want payments", got)
	}
	if got := snapshot.Applications[0].Health; got != "Healthy" {
		t.Fatalf("application health = %q, want Healthy", got)
	}
	if got := snapshot.Projects[0].Description; got != "Platform applications" {
		t.Fatalf("project description = %q", got)
	}
	if got := snapshot.Clusters[0].Server; got != "https://kubernetes.example.com" {
		t.Fatalf("cluster server = %q", got)
	}
}

func TestAPISourceLoadsApplicationResourceTree(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/applications/payments/resource-tree" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(`{"nodes":[{"kind":"Deployment","namespace":"default","name":"payments","health":{"status":"Healthy"},"status":"Synced","parentRefs":[{"kind":"Service","namespace":"default","name":"payments"}]}]}`))
	}))
	defer server.Close()

	tree, err := NewAPISource(server.URL, "", false).LoadResourceTree(context.Background(), "payments")
	if err != nil {
		t.Fatalf("LoadResourceTree() error = %v", err)
	}
	if len(tree.Nodes) != 1 || tree.Nodes[0].Kind != "Deployment" {
		t.Fatalf("tree nodes = %#v, want deployment", tree.Nodes)
	}
	if len(tree.Nodes[0].Parents) != 1 || tree.Nodes[0].Parents[0].Kind != "Service" {
		t.Fatalf("node parents = %#v, want service", tree.Nodes[0].Parents)
	}
}

func TestAPISourceSyncsAndDeletesApplication(t *testing.T) {
	var methods []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		methods = append(methods, r.Method+" "+r.URL.Path)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	source := NewAPISource(server.URL, "", false)
	if err := source.SyncApplication(context.Background(), "payments"); err != nil {
		t.Fatalf("SyncApplication() error = %v", err)
	}
	if err := source.DeleteApplication(context.Background(), "payments"); err != nil {
		t.Fatalf("DeleteApplication() error = %v", err)
	}
	if len(methods) != 2 || methods[0] != "POST /api/v1/applications/payments/sync" || methods[1] != "DELETE /api/v1/applications/payments" {
		t.Fatalf("methods = %v", methods)
	}
}

func TestAPISourceReturnsEndpointErrors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "denied", http.StatusUnauthorized)
	}))
	defer server.Close()

	_, err := NewAPISource(server.URL, "", false).Load(context.Background())
	if err == nil {
		t.Fatal("Load() error = nil, want error")
	}
}
