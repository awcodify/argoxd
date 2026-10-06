package argocd

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"sync"
	"testing"
)

func manifestResponse(t *testing.T, object map[string]any) string {
	t.Helper()
	inner, err := json.Marshal(object)
	if err != nil {
		t.Fatal(err)
	}
	outer, err := json.Marshal(map[string]string{"manifest": string(inner)})
	if err != nil {
		t.Fatal(err)
	}
	return string(outer)
}

func twoContainerPod() map[string]any {
	return map[string]any{
		"metadata": map[string]any{"name": "web-1-abc"},
		"spec": map[string]any{"containers": []any{
			map[string]any{"name": "app"}, map[string]any{"name": "sidecar"},
		}},
	}
}

// containerServer answers manifest and log requests, and records which
// containers the log requests asked for. A manifest status other than 200 makes
// the manifest endpoint fail.
type containerServer struct {
	*httptest.Server
	mu             sync.Mutex
	logContainers  []string
	manifestStatus int
}

func newContainerServer(t *testing.T, manifest map[string]any) *containerServer {
	t.Helper()
	server := &containerServer{manifestStatus: http.StatusOK}
	server.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/applications/checkout/resource":
			if server.manifestStatus != http.StatusOK {
				w.WriteHeader(server.manifestStatus)
				return
			}
			_, _ = w.Write([]byte(manifestResponse(t, manifest)))
		case "/api/v1/applications/checkout/logs":
			container := r.URL.Query().Get("container")
			server.mu.Lock()
			server.logContainers = append(server.logContainers, container)
			server.mu.Unlock()
			_, _ = w.Write([]byte(`{"result":{"content":"line from ` + container + `","podName":"web-1-abc"}}` + "\n" +
				`{"result":{"content":"","last":true}}` + "\n"))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)
	return server
}

func (s *containerServer) requested() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	containers := slices.Clone(s.logContainers)
	slices.Sort(containers)
	return containers
}

func TestAPISourceStreamsTheChosenContainerWithoutReadingTheManifest(t *testing.T) {
	server := newContainerServer(t, twoContainerPod())
	server.manifestStatus = http.StatusInternalServerError

	stream, err := NewAPISource(server.URL, "", false).StreamLogs(context.Background(), "checkout", webPod, "sidecar")
	if err != nil {
		t.Fatal(err)
	}

	entries := collect(t, stream)
	if len(entries) != 1 || entries[0].Container != "sidecar" || entries[0].Line != "line from sidecar" {
		t.Fatalf("entries = %+v, want the sidecar's line tagged with its container", entries)
	}
	if got := server.requested(); !slices.Equal(got, []string{"sidecar"}) {
		t.Fatalf("log requests = %v, want one for the sidecar", got)
	}
}

func TestAPISourceStreamsTheDefaultContainerWhenNoneIsChosen(t *testing.T) {
	server := newContainerServer(t, twoContainerPod())

	stream, err := NewAPISource(server.URL, "", false).StreamLogs(context.Background(), "checkout", webPod, "")
	if err != nil {
		t.Fatal(err)
	}

	entries := collect(t, stream)
	if len(entries) != 1 || entries[0].Container != "app" {
		t.Fatalf("entries = %+v, want the first container's line", entries)
	}
	if got := server.requested(); !slices.Equal(got, []string{"app"}) {
		t.Fatalf("log requests = %v, want one for the default container", got)
	}
}

func TestAPISourceStreamsEveryContainerOfAPod(t *testing.T) {
	server := newContainerServer(t, twoContainerPod())

	stream, err := NewAPISource(server.URL, "", false).StreamLogs(context.Background(), "checkout", webPod, AllContainers)
	if err != nil {
		t.Fatal(err)
	}

	containers := map[string]string{}
	for _, entry := range collect(t, stream) {
		containers[entry.Container] = entry.Line
	}
	if len(containers) != 2 || containers["app"] != "line from app" || containers["sidecar"] != "line from sidecar" {
		t.Fatalf("containers = %v, want a line from each", containers)
	}
	if got := server.requested(); !slices.Equal(got, []string{"app", "sidecar"}) {
		t.Fatalf("log requests = %v, want one per container", got)
	}
}

func TestAPISourceFallsBackToArgoCDsChoiceWhenTheManifestCannotBeRead(t *testing.T) {
	server := newContainerServer(t, twoContainerPod())
	server.manifestStatus = http.StatusForbidden

	stream, err := NewAPISource(server.URL, "", false).StreamLogs(context.Background(), "checkout", webPod, "")
	if err != nil {
		t.Fatal(err)
	}

	if entries := collect(t, stream); len(entries) != 1 {
		t.Fatalf("entries = %+v, want logs even without the manifest", entries)
	}
	if got := server.requested(); !slices.Equal(got, []string{""}) {
		t.Fatalf("log requests = %v, want one without a container", got)
	}
}

func TestAPISourceSnapshotOfAChosenOrEveryContainer(t *testing.T) {
	server := newContainerServer(t, twoContainerPod())
	source := NewAPISource(server.URL, "", false)

	one, err := source.ResourceLogs(context.Background(), "checkout", webPod, "sidecar")
	if err != nil || one != "line from sidecar" {
		t.Fatalf("ResourceLogs(sidecar) = %q, %v", one, err)
	}

	all, err := source.ResourceLogs(context.Background(), "checkout", webPod, AllContainers)
	if err != nil || all != "app │ line from app\nsidecar │ line from sidecar" {
		t.Fatalf("ResourceLogs(all) = %q, %v, want each container's lines under its name", all, err)
	}
}
