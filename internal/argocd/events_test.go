package argocd

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/awcodify/argoxd/internal/explorer"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	typedfake "k8s.io/client-go/kubernetes/fake"
)

var eventsNow = time.Date(2026, 10, 6, 12, 0, 0, 0, time.UTC)

func podEvent(namespace, kind, name, reason, message string, age time.Duration) *corev1.Event {
	return &corev1.Event{
		ObjectMeta:     metav1.ObjectMeta{Namespace: namespace, Name: kind + "-" + name + "." + reason},
		InvolvedObject: corev1.ObjectReference{Kind: kind, Name: name, Namespace: namespace},
		Type:           corev1.EventTypeNormal,
		Reason:         reason,
		Message:        message,
		LastTimestamp:  metav1.NewTime(eventsNow.Add(-age)),
	}
}

func eventsKubernetesSource(events ...runtime.Object) *KubernetesSource {
	return &KubernetesSource{
		namespace: "argocd",
		clients: func() (kubernetesClients, error) {
			return kubernetesClients{typed: typedfake.NewClientset(events...)}, nil
		},
	}
}

// webWorkload is a Deployment with a ReplicaSet and two Pods under it.
func webWorkload() explorer.ResourceNode {
	pod := func(name string) explorer.ResourceNode {
		return explorer.ResourceNode{Version: "v1", Kind: "Pod", Namespace: "store", Name: name, UID: "uid-" + name}
	}
	deployment := webDeployment
	deployment.UID = "uid-web"
	deployment.Children = []explorer.ResourceNode{{
		Group: "apps", Version: "v1", Kind: "ReplicaSet", Namespace: "store", Name: "web-1", UID: "uid-web-1",
		Children: []explorer.ResourceNode{pod("web-1-a"), pod("web-1-b")},
	}}
	return deployment
}

func TestFormatEventsListsTheOldestFirstWithTheirAge(t *testing.T) {
	warning := podEvent("store", "Pod", "web-1-a", "BackOff", "Back-off restarting failed container", 2*time.Minute)
	warning.Type = corev1.EventTypeWarning
	warning.Count = 5
	scheduled := podEvent("store", "Pod", "web-1-a", "Scheduled", "Assigned store/web-1-a to node-1", 3*time.Hour)

	text := formatEvents([]corev1.Event{*warning, *scheduled}, eventsNow)

	lines := strings.Split(text, "\n")
	if len(lines) != 3 || !strings.HasPrefix(lines[0], "AGE") {
		t.Fatalf("formatEvents() = %q, want a header and two events", text)
	}
	for index, want := range [][]string{
		{"3h", "Normal", "Scheduled", "Pod/web-1-a", "Assigned store/web-1-a to node-1"},
		{"2m", "Warning", "BackOff", "Pod/web-1-a", "Back-off restarting failed container (x5)"},
	} {
		fields := strings.Join(strings.Fields(lines[index+1]), " ")
		if fields != strings.Join(want, " ") {
			t.Fatalf("line %d = %q, want %q", index+1, fields, strings.Join(want, " "))
		}
	}
}

func TestFormatEventsIsEmptyWithoutEvents(t *testing.T) {
	if text := formatEvents(nil, eventsNow); text != "" {
		t.Fatalf("formatEvents(nil) = %q, want nothing", text)
	}
}

func TestKubernetesSourceListsEventsOfAWorkloadAndItsPods(t *testing.T) {
	source := eventsKubernetesSource(
		podEvent("store", "Deployment", "web", "ScalingReplicaSet", "Scaled up replica set web-1 to 2", time.Hour),
		podEvent("store", "Pod", "web-1-a", "Started", "Started container app", time.Minute),
		podEvent("store", "Pod", "other", "Started", "Started container other", time.Minute),
		podEvent("billing", "Pod", "web-1-a", "Started", "a Pod of the same name elsewhere", time.Minute),
	)

	text, err := source.ResourceEvents(context.Background(), "checkout", webWorkload())
	if err != nil {
		t.Fatalf("ResourceEvents() error = %v", err)
	}

	for _, want := range []string{"ScalingReplicaSet", "Started container app"} {
		if !strings.Contains(text, want) {
			t.Fatalf("events miss %q:\n%s", want, text)
		}
	}
	for _, unwanted := range []string{"container other", "elsewhere"} {
		if strings.Contains(text, unwanted) {
			t.Fatalf("events include %q:\n%s", unwanted, text)
		}
	}
}

func TestKubernetesSourceListsEventsOfAnApplication(t *testing.T) {
	source := eventsKubernetesSource(
		podEvent("argocd", "Application", "checkout", "ResourceUpdated", "Updated sync status: OutOfSync -> Synced", time.Minute),
		podEvent("argocd", "Application", "billing", "ResourceUpdated", "billing changed", time.Minute),
		podEvent("store", "Pod", "checkout", "Started", "not the Application", time.Minute),
	)

	text, err := source.ApplicationEvents(context.Background(), "checkout")
	if err != nil {
		t.Fatalf("ApplicationEvents() error = %v", err)
	}

	if !strings.Contains(text, "Updated sync status") || strings.Contains(text, "billing changed") || strings.Contains(text, "not the Application") {
		t.Fatalf("events = %q, want only the Application's", text)
	}
}

const apiEventList = `{"items":[{"involvedObject":{"kind":"Pod","name":"web-1-a","namespace":"store"},"type":"Warning","reason":"BackOff","message":"Back-off restarting failed container","count":3,"lastTimestamp":"2026-10-06T11:58:00Z"}]}`

func TestAPISourceListsEventsOfEachResourceByUID(t *testing.T) {
	var queries []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/applications/checkout/events" {
			http.NotFound(w, r)
			return
		}
		queries = append(queries, r.URL.Query().Get("resourceUID")+"/"+r.URL.Query().Get("resourceName")+"/"+r.URL.Query().Get("resourceNamespace"))
		_, _ = w.Write([]byte(apiEventList))
	}))
	defer server.Close()

	text, err := NewAPISource(server.URL, "", false).ResourceEvents(context.Background(), "checkout", webWorkload())
	if err != nil {
		t.Fatalf("ResourceEvents() error = %v", err)
	}

	want := []string{"uid-web/web/store", "uid-web-1/web-1/store", "uid-web-1-a/web-1-a/store", "uid-web-1-b/web-1-b/store"}
	if strings.Join(queries, " ") != strings.Join(want, " ") {
		t.Fatalf("queries = %v, want %v", queries, want)
	}
	if !strings.Contains(text, "BackOff") || !strings.Contains(text, "(x3)") {
		t.Fatalf("events = %q", text)
	}
}

func TestAPISourceListsEventsOfAnApplicationWithoutAResource(t *testing.T) {
	var query string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		query = r.URL.RawQuery
		_, _ = w.Write([]byte(apiEventList))
	}))
	defer server.Close()

	if _, err := NewAPISource(server.URL, "", false).ApplicationEvents(context.Background(), "checkout"); err != nil {
		t.Fatalf("ApplicationEvents() error = %v", err)
	}
	if query != "" {
		t.Fatalf("query = %q, want none", query)
	}
}

func TestAPISourceReadsResourceUIDs(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"nodes":[{"kind":"Deployment","namespace":"store","name":"web","uid":"abc-123"}]}`))
	}))
	defer server.Close()

	tree, err := NewAPISource(server.URL, "", false).LoadResourceTree(context.Background(), "checkout")
	if err != nil || len(tree.Nodes) != 1 || tree.Nodes[0].UID != "abc-123" {
		t.Fatalf("tree = %#v, err = %v, want the node's uid", tree, err)
	}
}

func TestDemoSourceListsEvents(t *testing.T) {
	source := NewDemoSource()
	tree, err := source.LoadResourceTree(context.Background(), "checkout")
	if err != nil {
		t.Fatal(err)
	}
	pod := tree.Hierarchy()[0]

	for name, load := range map[string]func() (string, error){
		"application": func() (string, error) { return source.ApplicationEvents(context.Background(), "checkout") },
		"resource":    func() (string, error) { return source.ResourceEvents(context.Background(), "checkout", pod) },
	} {
		text, err := load()
		if err != nil || !strings.HasPrefix(text, "AGE") {
			t.Fatalf("%s events = %q, err = %v", name, text, err)
		}
	}
}
