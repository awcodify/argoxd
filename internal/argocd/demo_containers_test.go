package argocd

import (
	"context"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/awcodify/argoxd/internal/explorer"
)

var (
	demoPod        = explorer.ResourceNode{Version: "v1", Kind: "Pod", Namespace: "store", Name: "cart-5d8f7c-x7k2p", Health: "Healthy"}
	demoDeployment = explorer.ResourceNode{Group: "apps", Version: "v1", Kind: "Deployment", Namespace: "store", Name: "cart"}
)

func TestDemoPodsAndDeploymentsHaveAnAppAndAMetricsContainer(t *testing.T) {
	source := NewDemoSource()

	for _, resource := range []explorer.ResourceNode{demoPod, demoDeployment} {
		got, err := LogContainers(context.Background(), source, "cart", resource)
		if err != nil {
			t.Fatal(err)
		}
		if !slices.Equal(got.Names, []string{"app", "metrics"}) || got.Default != "app" {
			t.Fatalf("containers of %s = %+v, want app and metrics with app by default", resource.Kind, got)
		}
	}
}

func TestDemoSourceSnapshotOfAChosenOrEveryContainer(t *testing.T) {
	source := NewDemoSource()
	ctx := context.Background()

	defaults, err := source.ResourceLogs(ctx, "cart", demoPod, "")
	if err != nil || !strings.Contains(defaults, "starting cart-5d8f7c-x7k2p") {
		t.Fatalf("default snapshot = %q, %v, want the app container's log", defaults, err)
	}
	metrics, err := source.ResourceLogs(ctx, "cart", demoPod, "metrics")
	if err != nil || !strings.Contains(metrics, "scrape /metrics") || strings.Contains(metrics, "starting") {
		t.Fatalf("metrics snapshot = %q, %v, want only the metrics container's log", metrics, err)
	}
	all, err := source.ResourceLogs(ctx, "cart", demoPod, AllContainers)
	if err != nil || !strings.Contains(all, "app │ ") || !strings.Contains(all, "metrics │ ") {
		t.Fatalf("snapshot of all containers = %q, %v, want lines under each container's name", all, err)
	}
	if _, err := source.ResourceLogs(ctx, "cart", demoPod, "missing"); err == nil {
		t.Fatal("a missing container was accepted")
	}
}

// containersSeen reads count entries and returns the containers they came from.
func containersSeen(t *testing.T, stream <-chan LogEntry, count int) []string {
	t.Helper()
	seen := map[string]bool{}
	for range count {
		entry := receive(t, stream)
		if entry.Container == "" {
			t.Fatalf("entry = %+v, want its container named", entry)
		}
		seen[entry.Container] = true
	}
	containers := make([]string, 0, len(seen))
	for container := range seen {
		containers = append(containers, container)
	}
	slices.Sort(containers)
	return containers
}

func TestDemoSourceStreamsTheChosenOrEveryContainer(t *testing.T) {
	source := NewDemoSource()
	source.logInterval = time.Millisecond
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	for container, want := range map[string][]string{
		"":            {"app"},
		"metrics":     {"metrics"},
		AllContainers: {"app", "metrics"},
		"app":         {"app"},
	} {
		stream, err := source.StreamLogs(ctx, "cart", demoPod, container)
		if err != nil {
			t.Fatal(err)
		}
		if got := containersSeen(t, stream, 20); !slices.Equal(got, want) {
			t.Fatalf("container %q streamed %v, want %v", container, got, want)
		}
	}

	workload, err := source.StreamLogs(ctx, "cart", demoDeployment, "metrics")
	if err != nil {
		t.Fatal(err)
	}
	if got := containersSeen(t, workload, 20); !slices.Equal(got, []string{"metrics"}) {
		t.Fatalf("workload streamed %v, want only the metrics containers", got)
	}
	if _, err := source.StreamLogs(ctx, "cart", demoPod, "missing"); err == nil {
		t.Fatal("streaming a missing container succeeded")
	}
}
