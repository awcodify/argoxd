package argocd

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/awcodify/argoxd/internal/explorer"
)

// DemoSource serves built-in sample data so argoxd can be tried, and recorded,
// without an Argo CD instance. Sync and delete change the in-memory state only.
type DemoSource struct {
	mu           sync.Mutex
	applications []explorer.Application
}

var (
	_ Source              = (*DemoSource)(nil)
	_ ApplicationOperator = (*DemoSource)(nil)
	_ ResourceInspector   = (*DemoSource)(nil)
)

// NewDemoSource returns a source with a handful of sample Applications.
func NewDemoSource() *DemoSource {
	now := time.Now()
	app := func(name, namespace, project, sync, health, revision string, age time.Duration) explorer.Application {
		return explorer.Application{
			Name: name, Namespace: "argocd", Project: project, Sync: sync, Health: health,
			Revision: revision, Destination: "in-cluster/" + namespace, LastSync: now.Add(-age),
		}
	}
	return &DemoSource{applications: []explorer.Application{
		app("cart", "store", "store", "OutOfSync", "Healthy", "9f2c1ab", 3*time.Hour),
		app("checkout", "store", "store", "Synced", "Healthy", "4be7d10", 26*time.Hour),
		app("search", "store", "store", "Synced", "Progressing", "c81a05e", 2*time.Minute),
		app("billing-worker", "payments", "payments", "OutOfSync", "Degraded", "7d3f9a2", 5*time.Hour),
		app("payments", "payments", "payments", "Synced", "Degraded", "7d3f9a2", 5*time.Hour),
		app("ingress-nginx", "platform", "platform", "Synced", "Healthy", "1a6e44c", 72*time.Hour),
		app("grafana", "observability", "observability", "Synced", "Healthy", "e05b8d3", 48*time.Hour),
		app("prometheus", "observability", "observability", "OutOfSync", "Healthy", "e05b8d3", 48*time.Hour),
	}}
}

// Load returns the sample snapshot.
func (s *DemoSource) Load(context.Context) (explorer.Snapshot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return explorer.Snapshot{
		Applications: append([]explorer.Application(nil), s.applications...),
		Projects: []explorer.Project{
			{Name: "store", Description: "Storefront services"},
			{Name: "payments", Description: "Billing and payment processing"},
			{Name: "platform", Description: "Cluster add-ons"},
			{Name: "observability", Description: "Metrics and dashboards"},
		},
		Clusters: []explorer.Cluster{
			{Name: "in-cluster", Server: "https://kubernetes.default.svc"},
			{Name: "staging", Server: "https://staging.example.com"},
		},
	}, nil
}

// LoadResourceTree builds a Deployment, ReplicaSet, Pods, Service and ConfigMap
// for the Application, reflecting its current sync and health status.
func (s *DemoSource) LoadResourceTree(_ context.Context, name string) (explorer.ResourceTree, error) {
	application, err := s.find(name)
	if err != nil {
		return explorer.ResourceTree{}, err
	}
	namespace := strings.TrimPrefix(application.Destination, "in-cluster/")
	deploySync := "Synced"
	if application.Sync == "OutOfSync" {
		deploySync = "OutOfSync"
	}
	podHealth := func(index int) string {
		if application.Health == "Degraded" && index == 0 {
			return "Degraded"
		}
		if application.Health == "Progressing" && index == 1 {
			return "Progressing"
		}
		return "Healthy"
	}

	deployment := explorer.ResourceReference{Group: "apps", Kind: "Deployment", Namespace: namespace, Name: name}
	replicaSet := explorer.ResourceReference{Group: "apps", Kind: "ReplicaSet", Namespace: namespace, Name: name + "-5d8f7c"}
	nodes := []explorer.ResourceNode{
		{Version: "v1", Kind: "ConfigMap", Namespace: namespace, Name: name + "-config", Sync: "Synced"},
		{Version: "v1", Kind: "Service", Namespace: namespace, Name: name, Sync: "Synced", Health: "Healthy"},
		{Group: "apps", Version: "v1", Kind: "Deployment", Namespace: namespace, Name: name, Sync: deploySync, Health: application.Health},
		{Group: "apps", Version: "v1", Kind: "ReplicaSet", Namespace: namespace, Name: replicaSet.Name, Health: "Healthy",
			Parents: []explorer.ResourceReference{deployment}},
	}
	for index := range 3 {
		nodes = append(nodes, explorer.ResourceNode{
			Version: "v1", Kind: "Pod", Namespace: namespace, Name: fmt.Sprintf("%s-5d8f7c-%s", name, podSuffixes[index]),
			Health: podHealth(index), Parents: []explorer.ResourceReference{replicaSet},
		})
	}
	return explorer.ResourceTree{Application: name, Nodes: nodes}, nil
}

var podSuffixes = [...]string{"x7k2p", "m9q4w", "t5v8z"}

// SyncApplication marks the Application synced and healthy unless it is a dry run.
func (s *DemoSource) SyncApplication(_ context.Context, name string, options SyncOptions) error {
	if options.DryRun {
		_, err := s.find(name)
		return err
	}
	return s.update(name, func(application *explorer.Application) {
		application.Sync, application.Health, application.LastSync = "Synced", "Healthy", time.Now()
	})
}

// RefreshApplication does nothing: the sample data has no manifest cache.
func (s *DemoSource) RefreshApplication(_ context.Context, name string) error {
	_, err := s.find(name)
	return err
}

// DeleteApplication removes the Application from the sample data.
func (s *DemoSource) DeleteApplication(_ context.Context, name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for index, application := range s.applications {
		if application.Name == name {
			s.applications = append(s.applications[:index:index], s.applications[index+1:]...)
			return nil
		}
	}
	return fmt.Errorf("application %q not found", name)
}

// ResourceManifest returns a short live manifest for the resource.
func (s *DemoSource) ResourceManifest(_ context.Context, _ string, resource explorer.ResourceNode) (string, error) {
	apiVersion := resource.Version
	if resource.Group != "" {
		apiVersion = resource.Group + "/" + resource.Version
	}
	manifest := fmt.Sprintf("apiVersion: %s\nkind: %s\nmetadata:\n  name: %s\n  namespace: %s\n",
		apiVersion, resource.Kind, resource.Name, resource.Namespace)
	if resource.Kind == "Deployment" {
		manifest += fmt.Sprintf("  labels:\n    app: %[1]s\nspec:\n  replicas: 3\n  selector:\n    matchLabels:\n      app: %[1]s\n"+
			"  template:\n    metadata:\n      labels:\n        app: %[1]s\n    spec:\n      containers:\n"+
			"        - name: %[1]s\n          image: registry.example.com/%[1]s:1.4.2\n          ports:\n            - containerPort: 8080\n",
			resource.Name)
	}
	return manifest, nil
}

// ResourceDiff shows a sample drift for OutOfSync Applications.
func (s *DemoSource) ResourceDiff(_ context.Context, application string, resource explorer.ResourceNode) (string, error) {
	app, err := s.find(application)
	if err != nil {
		return "", err
	}
	if app.Sync != "OutOfSync" || resource.Kind != "Deployment" {
		return "No differences between the live and desired state.", nil
	}
	return strings.Join([]string{
		"--- live",
		"+++ desired",
		"@@ spec @@",
		"-  replicas: 2",
		"+  replicas: 3",
		"@@ spec.template.spec.containers[0] @@",
		"-  image: registry.example.com/" + resource.Name + ":1.4.1",
		"+  image: registry.example.com/" + resource.Name + ":1.4.2",
	}, "\n"), nil
}

// ResourceLogs returns sample log lines, with errors for a Degraded Pod.
func (s *DemoSource) ResourceLogs(_ context.Context, _ string, pod explorer.ResourceNode) (string, error) {
	lines := []string{
		"2026-10-05T09:12:01Z INFO  starting " + pod.Name,
		"2026-10-05T09:12:02Z INFO  listening on :8080",
	}
	if pod.Health == "Degraded" {
		lines = append(lines,
			"2026-10-05T09:12:09Z ERROR payment gateway unreachable: dial tcp 10.0.4.17:443: i/o timeout",
			"2026-10-05T09:12:09Z FATAL readiness check failed, exiting")
	} else {
		lines = append(lines, "2026-10-05T09:12:10Z INFO  GET /healthz 200 1ms")
	}
	return strings.Join(lines, "\n"), nil
}

func (s *DemoSource) find(name string) (explorer.Application, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, application := range s.applications {
		if application.Name == name {
			return application, nil
		}
	}
	return explorer.Application{}, fmt.Errorf("application %q not found", name)
}

func (s *DemoSource) update(name string, change func(*explorer.Application)) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for index := range s.applications {
		if s.applications[index].Name == name {
			change(&s.applications[index])
			return nil
		}
	}
	return fmt.Errorf("application %q not found", name)
}
