package argocd

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/awcodify/argoxd/internal/explorer"
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
	if err := source.SyncApplication(context.Background(), "payments", SyncOptions{}); err != nil {
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

func TestAPISourceReadsApplicationDetailsAndResourceVersions(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/applications":
			_, _ = w.Write([]byte(`{"items":[{"metadata":{"name":"checkout"},"spec":{"sources":[{"targetRevision":"v1.2.0"}],"destination":{"name":"in-cluster","namespace":"store"}},"status":{"operationState":{"finishedAt":"2026-10-05T10:00:00Z"}}}]}`))
		case "/api/v1/applications/checkout/resource-tree":
			_, _ = w.Write([]byte(`{"nodes":[{"group":"apps","version":"v1","kind":"Deployment","namespace":"store","name":"web"}]}`))
		default:
			_, _ = w.Write([]byte(`{"items":[]}`))
		}
	}))
	defer server.Close()
	source := NewAPISource(server.URL, "", false)

	snapshot, err := source.Load(context.Background())
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	application := snapshot.Applications[0]
	if application.Revision != "v1.2.0" || application.Destination != "in-cluster/store" || application.LastSync.IsZero() {
		t.Fatalf("application = %+v, want revision, destination and last sync", application)
	}

	tree, err := source.LoadResourceTree(context.Background(), "checkout")
	if err != nil {
		t.Fatalf("LoadResourceTree() error = %v", err)
	}
	if tree.Nodes[0].Version != "v1" {
		t.Fatalf("version = %q, want v1", tree.Nodes[0].Version)
	}
}

func TestAPISourceSyncsWithOptionsAndRefreshes(t *testing.T) {
	var requests []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		requests = append(requests, r.Method+" "+r.URL.RequestURI()+" "+string(body))
		_, _ = w.Write([]byte(`{}`))
	}))
	defer server.Close()
	source := NewAPISource(server.URL, "", false)

	if err := source.SyncApplication(context.Background(), "payments", SyncOptions{Prune: true, DryRun: true}); err != nil {
		t.Fatalf("SyncApplication() error = %v", err)
	}
	if err := source.RefreshApplication(context.Background(), "payments"); err != nil {
		t.Fatalf("RefreshApplication() error = %v", err)
	}

	want := []string{
		`POST /api/v1/applications/payments/sync {"prune":true,"dryRun":true}`,
		`GET /api/v1/applications/payments?refresh=hard `,
	}
	if !slices.Equal(requests, want) {
		t.Fatalf("requests = %q, want %q", requests, want)
	}
}

func TestAPISourceInspectsResources(t *testing.T) {
	deployment := explorer.ResourceNode{Group: "apps", Version: "v1", Kind: "Deployment", Namespace: "store", Name: "web"}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()
		switch r.URL.Path {
		case "/api/v1/applications/checkout/resource":
			if query.Get("resourceName") != "web" || query.Get("kind") != "Deployment" || query.Get("version") != "v1" {
				t.Errorf("resource query = %v", query)
			}
			_, _ = w.Write([]byte(`{"manifest":"{\"kind\":\"Deployment\",\"spec\":{\"replicas\":2}}"}`))
		case "/api/v1/applications/checkout/managed-resources":
			_, _ = w.Write([]byte(`{"items":[{"normalizedLiveState":"{\"spec\":{\"replicas\":1}}","predictedLiveState":"{\"spec\":{\"replicas\":2}}"}]}`))
		case "/api/v1/applications/checkout/logs":
			if query.Get("podName") != "web-1-abc" || query.Get("follow") != "false" {
				t.Errorf("logs query = %v", query)
			}
			_, _ = w.Write([]byte("{\"result\":{\"content\":\"starting\"}}\n{\"result\":{\"content\":\"ready\"}}\n"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	source := NewAPISource(server.URL, "", false)

	manifest, err := source.ResourceManifest(context.Background(), "checkout", deployment)
	if err != nil || !strings.Contains(manifest, "kind: Deployment") || !strings.Contains(manifest, "replicas: 2") {
		t.Fatalf("ResourceManifest() = %q, %v", manifest, err)
	}

	diff, err := source.ResourceDiff(context.Background(), "checkout", deployment)
	if err != nil || !strings.Contains(diff, "-   replicas: 1") || !strings.Contains(diff, "+   replicas: 2") {
		t.Fatalf("ResourceDiff() = %q, %v", diff, err)
	}

	pod := explorer.ResourceNode{Version: "v1", Kind: "Pod", Namespace: "store", Name: "web-1-abc"}
	logs, err := source.ResourceLogs(context.Background(), "checkout", pod, "app")
	if err != nil || logs != "starting\nready" {
		t.Fatalf("ResourceLogs() = %q, %v", logs, err)
	}
}

func TestAPISourceReportsResourcesWithoutDesiredState(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"items":[]}`))
	}))
	defer server.Close()

	_, err := NewAPISource(server.URL, "", false).ResourceDiff(context.Background(), "checkout", explorer.ResourceNode{Kind: "Pod", Name: "web-1-abc"})
	if err == nil {
		t.Fatal("ResourceDiff() error = nil, want an error for an unmanaged resource")
	}
}

func TestAPISourceErrorsIncludeArgoCDMessage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"Deployment apps web not found as part of application checkout","code":3,"message":"Deployment apps web not found as part of application checkout"}`))
	}))
	defer server.Close()

	_, err := NewAPISource(server.URL, "", false).ResourceManifest(context.Background(), "checkout", explorer.ResourceNode{Kind: "Deployment", Name: "web"})

	if err == nil || !strings.Contains(err.Error(), "Deployment apps web not found as part of application checkout") {
		t.Fatalf("error = %v, want Argo CD's explanation", err)
	}
	if strings.Count(err.Error(), "not found as part of") != 1 {
		t.Fatalf("error repeats the explanation: %v", err)
	}
}

func TestAPISourceReadsHistoryAndRollsBack(t *testing.T) {
	var rollback string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			body, _ := io.ReadAll(r.Body)
			rollback = r.URL.Path + " " + string(body)
			return
		}
		fmt.Fprint(w, `{"status":{"history":[
			{"id":1,"revision":"aaa1111","deployedAt":"2026-10-01T10:00:00Z","source":{"repoURL":"https://example.com/a.git"}},
			{"id":2,"revisions":["bbb2222"],"deployedAt":"2026-10-02T10:00:00Z","sources":[{"repoURL":"https://example.com/b.git"}]}]}}`)
	}))
	defer server.Close()
	source := NewAPISource(server.URL, "", false)

	history, err := source.ApplicationHistory(context.Background(), "checkout")
	if err != nil {
		t.Fatal(err)
	}
	if len(history) != 2 || history[0].ID != 2 || history[0].Revision != "bbb2222" || history[0].Repo != "https://example.com/b.git" || history[1].Revision != "aaa1111" {
		t.Fatalf("history = %+v", history)
	}

	if err := source.RollbackApplication(context.Background(), "checkout", 1, SyncOptions{Prune: true}); err != nil {
		t.Fatal(err)
	}
	if want := `/api/v1/applications/checkout/rollback {"id":1,"prune":true,"dryRun":false}`; rollback != want {
		t.Fatalf("rollback request = %q, want %q", rollback, want)
	}
}

func TestAPISourceReadsConditionsAndSyncPolicy(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/applications":
			_, _ = w.Write([]byte(`{"items":[
				{"metadata":{"name":"checkout"},"spec":{"project":"store","syncPolicy":{"automated":{"prune":true,"selfHeal":true}}},
				 "status":{"conditions":[{"type":"ComparisonError","message":"repository not found"}]}},
				{"metadata":{"name":"manual"},"spec":{"project":"store"}}]}`))
		default:
			_, _ = w.Write([]byte(`{"items":[]}`))
		}
	}))
	defer server.Close()

	snapshot, err := NewAPISource(server.URL, "", false).Load(context.Background())
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	checkout, manual := snapshot.Applications[0], snapshot.Applications[1]
	if want := []explorer.Condition{{Type: "ComparisonError", Message: "repository not found"}}; !slices.Equal(checkout.Conditions, want) {
		t.Fatalf("conditions = %+v, want %+v", checkout.Conditions, want)
	}
	if want := (explorer.SyncPolicy{Automated: true, SelfHeal: true, Prune: true}); checkout.Policy != want {
		t.Fatalf("policy = %+v, want %+v", checkout.Policy, want)
	}
	if len(manual.Conditions) != 0 || manual.Policy != (explorer.SyncPolicy{}) {
		t.Fatalf("manual application = %+v, want no conditions and no automation", manual)
	}
}

func TestAPISourceReadsTheLastOperation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/applications" {
			_, _ = w.Write([]byte(`{"items":[]}`))
			return
		}
		_, _ = w.Write([]byte(`{"items":[
			{"metadata":{"name":"checkout"},"status":{"operationState":{"phase":"Failed","message":"boom",
				"startedAt":"2026-10-07T10:00:00Z","finishedAt":"2026-10-07T10:00:30Z",
				"syncResult":{"revision":"7d3f9a2","resources":[
					{"group":"apps","kind":"Deployment","namespace":"store","name":"checkout","status":"SyncFailed","message":"invalid","syncPhase":"Sync"}]}}}},
			{"metadata":{"name":"never-synced"}}]}`))
	}))
	defer server.Close()

	snapshot, err := NewAPISource(server.URL, "", false).Load(context.Background())
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if snapshot.Applications[1].Operation != nil {
		t.Fatalf("operation = %+v, want none for an application that never synced", snapshot.Applications[1].Operation)
	}
	operation := snapshot.Applications[0].Operation
	if operation == nil || operation.Phase != "Failed" || operation.Message != "boom" || operation.Revision != "7d3f9a2" {
		t.Fatalf("operation = %+v", operation)
	}
	want := []explorer.OperationResult{{Group: "apps", Kind: "Deployment", Namespace: "store", Name: "checkout", Status: "SyncFailed", Message: "invalid", SyncPhase: "Sync"}}
	if !slices.Equal(operation.Results, want) {
		t.Fatalf("results = %+v, want %+v", operation.Results, want)
	}
}

func TestAPISourceMarksResourcesThatRequirePruningAndOrphans(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/applications/payments/resource-tree":
			_, _ = w.Write([]byte(`{"nodes":[
				{"kind":"ConfigMap","namespace":"pay","name":"old","status":"OutOfSync"},
				{"kind":"ConfigMap","namespace":"pay","name":"current","status":"Synced"}],
				"orphanedNodes":[{"kind":"Secret","namespace":"pay","name":"stray"}]}`))
		case "/api/v1/applications/payments":
			_, _ = w.Write([]byte(`{"status":{"resources":[
				{"kind":"ConfigMap","namespace":"pay","name":"old","requiresPruning":true},
				{"kind":"ConfigMap","namespace":"pay","name":"current"}]}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	tree, err := NewAPISource(server.URL, "", false).LoadResourceTree(context.Background(), "payments")
	if err != nil {
		t.Fatalf("LoadResourceTree() error = %v", err)
	}

	got := map[string][2]bool{}
	for _, node := range tree.Nodes {
		got[node.Name] = [2]bool{node.RequiresPruning, node.Orphaned}
	}
	want := map[string][2]bool{"old": {true, false}, "current": {false, false}, "stray": {false, true}}
	if len(got) != len(want) || got["old"] != want["old"] || got["current"] != want["current"] || got["stray"] != want["stray"] {
		t.Fatalf("[requires pruning, orphaned] by name = %v, want %v", got, want)
	}
}

func TestAPISourceLoadsTheTreeWhenTheApplicationCannotBeRead(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/applications/payments/resource-tree" {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		_, _ = w.Write([]byte(`{"nodes":[{"kind":"ConfigMap","namespace":"pay","name":"old"}]}`))
	}))
	defer server.Close()

	tree, err := NewAPISource(server.URL, "", false).LoadResourceTree(context.Background(), "payments")
	if err != nil {
		t.Fatalf("LoadResourceTree() error = %v, want the tree without prune marks", err)
	}
	if len(tree.Nodes) != 1 || tree.Nodes[0].RequiresPruning {
		t.Fatalf("nodes = %+v, want one node without a prune mark", tree.Nodes)
	}
}
