package argocd

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/awcodify/argoxd/internal/explorer"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

var (
	webReference     = explorer.ResourceReference{Group: "apps", Kind: "Deployment", Namespace: "store", Name: "web"}
	webConfigMapRef  = explorer.ResourceReference{Kind: "ConfigMap", Namespace: "store", Name: "web-config"}
	selectedWebSyncs = []explorer.ResourceReference{webReference, webConfigMapRef}
)

func TestKubernetesSourceSyncsOnlyTheChosenResources(t *testing.T) {
	application := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "argoproj.io/v1alpha1",
		"kind":       "Application",
		"metadata":   map[string]any{"namespace": "argocd", "name": "checkout"},
	}}
	source := fakeKubernetesSource(application)

	if err := source.SyncResources(context.Background(), "checkout", selectedWebSyncs, SyncOptions{Prune: true}); err != nil {
		t.Fatalf("SyncResources() error = %v", err)
	}

	client, _ := source.client()
	got, err := client.Resource(applicationsResource).Namespace("argocd").Get(context.Background(), "checkout", metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	resources, _, _ := unstructured.NestedSlice(got.Object, "operation", "sync", "resources")
	want := []any{
		map[string]any{"group": "apps", "kind": "Deployment", "namespace": "store", "name": "web"},
		map[string]any{"group": "", "kind": "ConfigMap", "namespace": "store", "name": "web-config"},
	}
	if !reflect.DeepEqual(resources, want) {
		t.Fatalf("operation.sync.resources = %v, want %v", resources, want)
	}
	if prune, _, _ := unstructured.NestedBool(got.Object, "operation", "sync", "prune"); !prune {
		t.Fatalf("operation = %v, want the options kept", got.Object["operation"])
	}
}

func TestAPISourceSyncsOnlyTheChosenResources(t *testing.T) {
	var path string
	var body struct {
		Prune     bool `json:"prune"`
		DryRun    bool `json:"dryRun"`
		Resources []struct {
			Group     string `json:"group"`
			Kind      string `json:"kind"`
			Namespace string `json:"namespace"`
			Name      string `json:"name"`
		} `json:"resources"`
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path = r.Method + " " + r.URL.Path
		_ = json.NewDecoder(r.Body).Decode(&body)
	}))
	defer server.Close()

	if err := NewAPISource(server.URL, "", false).SyncResources(context.Background(), "checkout", selectedWebSyncs, SyncOptions{DryRun: true}); err != nil {
		t.Fatalf("SyncResources() error = %v", err)
	}

	if path != "POST /api/v1/applications/checkout/sync" || !body.DryRun || len(body.Resources) != 2 {
		t.Fatalf("request = %s %+v", path, body)
	}
	if first := body.Resources[0]; first.Group != "apps" || first.Kind != "Deployment" || first.Namespace != "store" || first.Name != "web" {
		t.Fatalf("first resource = %+v", first)
	}
}

func TestAPISourceFullSyncsSendNoResources(t *testing.T) {
	var body map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&body)
	}))
	defer server.Close()

	if err := NewAPISource(server.URL, "", false).SyncApplication(context.Background(), "checkout", SyncOptions{}); err != nil {
		t.Fatal(err)
	}
	if _, found := body["resources"]; found {
		t.Fatalf("a full sync sent resources: %v", body)
	}
}

func TestDemoSourceSyncsChosenResources(t *testing.T) {
	source := NewDemoSource()

	if err := source.SyncResources(context.Background(), "cart", []explorer.ResourceReference{{Kind: "Deployment", Name: "cart"}}, SyncOptions{}); err != nil {
		t.Fatalf("SyncResources() error = %v", err)
	}

	snapshot, _ := source.Load(context.Background())
	for _, application := range snapshot.Applications {
		if application.Name == "cart" && application.Sync != "Synced" {
			t.Fatalf("cart is %s, want Synced", application.Sync)
		}
	}
}
