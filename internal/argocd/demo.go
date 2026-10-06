package argocd

import (
	"context"
	"fmt"
	"hash/fnv"
	"slices"
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
	// history holds each Application's deployments, oldest first.
	history map[string][]explorer.HistoryEntry
	// logInterval is how often a followed log gets a new line.
	logInterval time.Duration
}

var (
	_ Source              = (*DemoSource)(nil)
	_ ApplicationOperator = (*DemoSource)(nil)
	_ ResourceInspector   = (*DemoSource)(nil)
	_ RollbackOperator    = (*DemoSource)(nil)
	_ LogStreamer         = (*DemoSource)(nil)
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
	applications := []explorer.Application{
		app("cart", "store", "store", "OutOfSync", "Healthy", "9f2c1ab", 3*time.Hour),
		app("checkout", "store", "store", "Synced", "Healthy", "4be7d10", 26*time.Hour),
		app("search", "store", "store", "Synced", "Progressing", "c81a05e", 2*time.Minute),
		app("billing-worker", "payments", "payments", "OutOfSync", "Degraded", "7d3f9a2", 5*time.Hour),
		app("payments", "payments", "payments", "Synced", "Degraded", "7d3f9a2", 5*time.Hour),
		app("ingress-nginx", "platform", "platform", "Synced", "Healthy", "1a6e44c", 72*time.Hour),
		app("grafana", "observability", "observability", "Synced", "Healthy", "e05b8d3", 48*time.Hour),
		app("prometheus", "observability", "observability", "OutOfSync", "Healthy", "e05b8d3", 48*time.Hour),
	}
	history := make(map[string][]explorer.HistoryEntry, len(applications))
	for _, application := range applications {
		history[application.Name] = sampleHistory(application)
	}
	return &DemoSource{applications: applications, history: history, logInterval: time.Second}
}

// sampleHistory gives an Application two earlier deployments, a day apart each,
// before the one it runs now.
func sampleHistory(application explorer.Application) []explorer.HistoryEntry {
	repo := "https://github.com/example/" + application.Name + ".git"
	entries := make([]explorer.HistoryEntry, 0, 3)
	for id := int64(1); id <= 3; id++ {
		entry := explorer.HistoryEntry{
			ID:         id,
			Revision:   application.Revision,
			DeployedAt: application.LastSync.Add(-time.Duration(3-id) * 24 * time.Hour),
			Repo:       repo,
		}
		if id < 3 {
			sum := fnv.New32a()
			fmt.Fprintf(sum, "%s/%d", application.Name, id)
			entry.Revision = fmt.Sprintf("%07x", sum.Sum32()&0xfffffff)
		}
		entries = append(entries, entry)
	}
	return entries
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

// ApplicationHistory returns the sample deployments, newest first.
func (s *DemoSource) ApplicationHistory(_ context.Context, name string) ([]explorer.HistoryEntry, error) {
	if _, err := s.find(name); err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return newestFirst(slices.Clone(s.history[name])), nil
}

// RollbackApplication runs the Application at an earlier revision, unless it is
// a dry run. As in Argo CD, the Application is then OutOfSync with Git.
func (s *DemoSource) RollbackApplication(_ context.Context, name string, id int64, options SyncOptions) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	index := slices.IndexFunc(s.history[name], func(entry explorer.HistoryEntry) bool { return entry.ID == id })
	if index < 0 {
		return fmt.Errorf("application %q has no deployment with id %d", name, id)
	}
	if options.DryRun {
		return nil
	}
	target := s.history[name][index]
	now := time.Now()
	target.ID, target.DeployedAt = s.history[name][len(s.history[name])-1].ID+1, now
	s.history[name] = append(s.history[name], target)
	for position := range s.applications {
		if s.applications[position].Name == name {
			application := &s.applications[position]
			application.Revision, application.Sync, application.Health, application.LastSync = target.Revision, "OutOfSync", "Healthy", now
		}
	}
	return nil
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
			"        - name: app\n          image: registry.example.com/%[1]s:1.4.2\n          ports:\n            - containerPort: 8080\n"+
			"        - name: metrics\n          image: registry.example.com/metrics-exporter:0.9.0\n          ports:\n            - containerPort: 9102\n",
			resource.Name)
	}
	if resource.Kind == "Pod" {
		manifest += "spec:\n  containers:\n    - name: app\n      image: registry.example.com/app:1.4.2\n" +
			"    - name: metrics\n      image: registry.example.com/metrics-exporter:0.9.0\n"
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

// demoContainerNames are the containers of every sample Pod.
var demoContainerNames = []string{"app", "metrics"}

// demoContainers resolves a chosen container: none means the app container.
func demoContainers(container string) ([]string, error) {
	switch {
	case container == "":
		return []string{"app"}, nil
	case container == AllContainers:
		return demoContainerNames, nil
	case slices.Contains(demoContainerNames, container):
		return []string{container}, nil
	}
	return nil, fmt.Errorf("container %q not found in the pod", container)
}

// ResourceLogs returns sample log lines of a container, with errors for a
// Degraded Pod's app. AllContainers groups the lines under each container's name.
func (s *DemoSource) ResourceLogs(_ context.Context, _ string, pod explorer.ResourceNode, container string) (string, error) {
	containers, err := demoContainers(container)
	if err != nil {
		return "", err
	}
	var lines []string
	for _, name := range containers {
		for _, line := range recentLogLines(pod, name) {
			if container == AllContainers {
				line = name + LogSeparator + line
			}
			lines = append(lines, line)
		}
	}
	return strings.Join(lines, "\n"), nil
}

func recentLogLines(pod explorer.ResourceNode, container string) []string {
	if container == "metrics" {
		return []string{
			"2026-10-05T09:12:01Z INFO  metrics exporter listening on :9102",
			"2026-10-05T09:12:11Z INFO  scrape /metrics 200 2ms",
		}
	}
	lines := []string{
		"2026-10-05T09:12:01Z INFO  starting " + pod.Name,
		"2026-10-05T09:12:02Z INFO  listening on :8080",
	}
	if pod.Health == "Degraded" {
		return append(lines,
			"2026-10-05T09:12:09Z ERROR payment gateway unreachable: dial tcp 10.0.4.17:443: i/o timeout",
			"2026-10-05T09:12:09Z FATAL readiness check failed, exiting")
	}
	return append(lines, "2026-10-05T09:12:10Z INFO  GET /healthz 200 1ms")
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

// StreamLogs sends a Pod's sample log, or the interleaved logs of the Pods of
// a workload, with a new line every logInterval.
func (s *DemoSource) StreamLogs(ctx context.Context, name string, resource explorer.ResourceNode, container string) (<-chan LogEntry, error) {
	if _, err := s.find(name); err != nil {
		return nil, err
	}
	if resource.Kind == "Pod" {
		return s.streamPod(ctx, name, resource, container)
	}
	tree, err := s.LoadResourceTree(ctx, name)
	if err != nil {
		return nil, err
	}
	var streams []<-chan LogEntry
	for _, node := range tree.Nodes {
		if node.Kind != "Pod" {
			continue
		}
		stream, err := s.streamPod(ctx, name, node, container)
		if err != nil {
			return nil, err
		}
		streams = append(streams, stream)
	}
	return mergeStreams(ctx, streams...), nil
}

// streamPod follows the chosen containers of a Pod as one stream.
func (s *DemoSource) streamPod(ctx context.Context, name string, pod explorer.ResourceNode, container string) (<-chan LogEntry, error) {
	containers, err := demoContainers(container)
	if err != nil {
		return nil, err
	}
	streams := make([]<-chan LogEntry, 0, len(containers))
	for _, each := range containers {
		streams = append(streams, s.streamContainer(ctx, pod, each))
	}
	if len(streams) == 1 {
		return streams[0], nil
	}
	return mergeStreams(ctx, streams...), nil
}

func (s *DemoSource) streamContainer(ctx context.Context, pod explorer.ResourceNode, container string) <-chan LogEntry {
	entries := make(chan LogEntry)
	go func() {
		defer close(entries)
		send := func(line string) bool {
			select {
			case entries <- LogEntry{Pod: pod.Name, Container: container, Line: line}:
				return true
			case <-ctx.Done():
				return false
			}
		}
		for _, line := range recentLogLines(pod, container) {
			if !send(line) {
				return
			}
		}
		ticker := time.NewTicker(s.logInterval)
		defer ticker.Stop()
		for count := 1; ; count++ {
			select {
			case now := <-ticker.C:
				if !send(demoLogLine(now, pod, container, count)) {
					return
				}
			case <-ctx.Done():
				return
			}
		}
	}()
	return entries
}

// demoLogLine is the count-th new line of a container's sample log.
func demoLogLine(now time.Time, pod explorer.ResourceNode, container string, count int) string {
	timestamp := now.UTC().Format(time.RFC3339)
	switch {
	case container == "metrics":
		return fmt.Sprintf("%s INFO  scrape /metrics 200 %dms", timestamp, 1+count%4)
	case pod.Health == "Degraded":
		return fmt.Sprintf("%s WARN  payment gateway unreachable, retry %d", timestamp, count)
	}
	return fmt.Sprintf("%s INFO  GET /api/items/%d 200 %dms", timestamp, count, 2+count%7)
}
