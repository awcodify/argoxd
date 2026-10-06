package argocd

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/awcodify/argoxd/internal/explorer"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func workloadObject(apiVersion, kind string) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": apiVersion,
		"kind":       kind,
		"metadata":   map[string]any{"namespace": "store", "name": "web"},
		"spec":       map[string]any{"template": map[string]any{"metadata": map[string]any{"labels": map[string]any{"app": "web"}}}},
	}}
}

func podObject(name string) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "v1", "kind": "Pod",
		"metadata": map[string]any{"namespace": "store", "name": name},
	}}
}

func TestKubernetesSourceRestartsAWorkloadByStampingItsPodTemplate(t *testing.T) {
	for _, test := range []struct {
		apiVersion string
		node       explorer.ResourceNode
	}{
		{"apps/v1", webDeployment},
		{"apps/v1", explorer.ResourceNode{Group: "apps", Version: "v1", Kind: "StatefulSet", Namespace: "store", Name: "web"}},
		{"apps/v1", explorer.ResourceNode{Group: "apps", Version: "v1", Kind: "DaemonSet", Namespace: "store", Name: "web"}},
	} {
		source := fakeKubernetesSource(workloadObject(test.apiVersion, test.node.Kind))
		before := time.Now().Add(-2 * time.Second)

		if err := source.RestartResource(context.Background(), "checkout", test.node); err != nil {
			t.Fatalf("%s: RestartResource() error = %v", test.node.Kind, err)
		}

		got, err := source.dynamicGet(test.node)
		if err != nil {
			t.Fatal(err)
		}
		stamp, _, _ := unstructured.NestedString(got.Object, "spec", "template", "metadata", "annotations", "kubectl.kubernetes.io/restartedAt")
		restartedAt, err := time.Parse(time.RFC3339, stamp)
		if err != nil || restartedAt.Before(before) {
			t.Fatalf("%s: restartedAt = %q, want a current RFC 3339 time", test.node.Kind, stamp)
		}
		if app, _, _ := unstructured.NestedString(got.Object, "spec", "template", "metadata", "labels", "app"); app != "web" {
			t.Fatalf("%s: the restart dropped the template's labels: %v", test.node.Kind, got.Object["spec"])
		}
	}
}

func TestKubernetesSourceRefusesToRestartOtherKinds(t *testing.T) {
	source := fakeKubernetesSource()
	configMap := explorer.ResourceNode{Version: "v1", Kind: "ConfigMap", Namespace: "store", Name: "settings"}

	if err := source.RestartResource(context.Background(), "checkout", configMap); err == nil || !strings.Contains(err.Error(), "cannot be restarted") {
		t.Fatalf("error = %v, want a message that ConfigMaps cannot be restarted", err)
	}
	if err := source.RestartResource(context.Background(), "checkout", webDeployment); err == nil {
		t.Fatal("restarting a missing Deployment succeeded")
	}
}

func TestKubernetesSourceDeletesAPod(t *testing.T) {
	source := fakeKubernetesSource(podObject("web-abc"), podObject("web-def"))

	if err := source.DeleteResource(context.Background(), "checkout", containerNode("web-abc")); err != nil {
		t.Fatalf("DeleteResource() error = %v", err)
	}

	client, _ := source.client()
	pods, err := client.Resource(schema.GroupVersionResource{Version: "v1", Resource: "pods"}).Namespace("store").List(context.Background(), metav1.ListOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(pods.Items) != 1 || pods.Items[0].GetName() != "web-def" {
		t.Fatalf("pods left = %v, want only web-def", pods.Items)
	}
	if err := source.DeleteResource(context.Background(), "checkout", containerNode("web-abc")); err == nil {
		t.Fatal("deleting a missing Pod succeeded")
	}
}

func TestAPISourceRestartsAWorkloadThroughAResourceAction(t *testing.T) {
	var method, path, body string
	var query map[string][]string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method, path, query = r.Method, r.URL.Path, r.URL.Query()
		payload, _ := io.ReadAll(r.Body)
		body = string(payload)
	}))
	defer server.Close()

	if err := NewAPISource(server.URL, "", false).RestartResource(context.Background(), "checkout", webDeployment); err != nil {
		t.Fatalf("RestartResource() error = %v", err)
	}

	// Argo CD takes the action name as the whole body and the resource in the query.
	if method != http.MethodPost || path != "/api/v1/applications/checkout/resource/actions" || body != `"restart"` {
		t.Fatalf("request = %s %s with body %s, want a POST of the JSON string \"restart\"", method, path, body)
	}
	for key, want := range map[string]string{"resourceName": "web", "namespace": "store", "kind": "Deployment", "group": "apps", "version": "v1"} {
		if got := query[key]; len(got) != 1 || got[0] != want {
			t.Fatalf("query = %v, want %s = %s", query, key, want)
		}
	}
}

func TestAPISourceDeletesAResource(t *testing.T) {
	var method, path string
	var query map[string][]string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method, path, query = r.Method, r.URL.Path, r.URL.Query()
	}))
	defer server.Close()

	if err := NewAPISource(server.URL, "", false).DeleteResource(context.Background(), "checkout", containerNode("web-abc")); err != nil {
		t.Fatalf("DeleteResource() error = %v", err)
	}

	if method != http.MethodDelete || path != "/api/v1/applications/checkout/resource" {
		t.Fatalf("request = %s %s", method, path)
	}
	for key, want := range map[string]string{"resourceName": "web-abc", "namespace": "store", "kind": "Pod", "version": "v1"} {
		if got := query[key]; len(got) != 1 || got[0] != want {
			t.Fatalf("query = %v, want %s = %s", query, key, want)
		}
	}
}

func TestAPISourceActionsReportArgoCDsMessageAndRefuseOtherKinds(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"message":"permission denied"}`))
	}))
	defer server.Close()
	source := NewAPISource(server.URL, "", false)

	if err := source.RestartResource(context.Background(), "checkout", webDeployment); err == nil || !strings.Contains(err.Error(), "permission denied") {
		t.Fatalf("restart error = %v, want Argo CD's message", err)
	}
	if err := source.DeleteResource(context.Background(), "checkout", webPod); err == nil || !strings.Contains(err.Error(), "permission denied") {
		t.Fatalf("delete error = %v, want Argo CD's message", err)
	}

	before := requests
	configMap := explorer.ResourceNode{Version: "v1", Kind: "ConfigMap", Namespace: "store", Name: "settings"}
	if err := source.RestartResource(context.Background(), "checkout", configMap); err == nil || !strings.Contains(err.Error(), "cannot be restarted") || requests != before {
		t.Fatalf("error = %v after %d extra requests, want a local refusal", err, requests-before)
	}
}

func demoPodNames(t *testing.T, source *DemoSource) (replicaSet string, pods []string) {
	t.Helper()
	tree, err := source.LoadResourceTree(context.Background(), "cart")
	if err != nil {
		t.Fatal(err)
	}
	for _, node := range tree.Nodes {
		switch node.Kind {
		case "ReplicaSet":
			replicaSet = node.Name
		case "Pod":
			pods = append(pods, node.Name)
		}
	}
	return replicaSet, pods
}

func TestDemoSourceRestartReplacesTheReplicaSetAndEveryPod(t *testing.T) {
	source := NewDemoSource()
	oldSet, oldPods := demoPodNames(t, source)

	if err := source.RestartResource(context.Background(), "cart", demoDeployment); err != nil {
		t.Fatal(err)
	}

	newSet, newPods := demoPodNames(t, source)
	if newSet == oldSet || len(newPods) != len(oldPods) {
		t.Fatalf("replica set %q -> %q with %d -> %d pods, want a new one with as many pods", oldSet, newSet, len(oldPods), len(newPods))
	}
	for _, pod := range newPods {
		for _, old := range oldPods {
			if pod == old {
				t.Fatalf("pod %s survived the restart", pod)
			}
		}
	}
}

func TestDemoSourceDeleteReplacesOnlyThatPod(t *testing.T) {
	source := NewDemoSource()
	_, before := demoPodNames(t, source)

	if err := source.DeleteResource(context.Background(), "cart", demoPod); err != nil {
		t.Fatal(err)
	}

	_, after := demoPodNames(t, source)
	if len(after) != len(before) {
		t.Fatalf("pods = %v, want as many as before", after)
	}
	kept := 0
	for _, pod := range after {
		if pod == demoPod.Name {
			t.Fatalf("the deleted pod %s is still there", pod)
		}
		for _, old := range before {
			if pod == old {
				kept++
			}
		}
	}
	if kept != len(before)-1 {
		t.Fatalf("kept %d of %d pods, want all but the deleted one: %v -> %v", kept, len(before), before, after)
	}
}

func TestDemoSourceRefusesWhatItCannotAct(t *testing.T) {
	source := NewDemoSource()
	ctx := context.Background()

	if err := source.RestartResource(ctx, "missing", demoDeployment); err == nil {
		t.Fatal("restarting in a missing application succeeded")
	}
	if err := source.RestartResource(ctx, "cart", demoPod); err == nil || !strings.Contains(err.Error(), "cannot be restarted") {
		t.Fatalf("error = %v, want a message that a Pod cannot be restarted", err)
	}
	if err := source.DeleteResource(ctx, "cart", explorer.ResourceNode{Version: "v1", Kind: "Pod", Namespace: "store", Name: "nope"}); err == nil {
		t.Fatal("deleting a missing pod succeeded")
	}
}

// dynamicGet reads a resource back through the dynamic client.
func (s *KubernetesSource) dynamicGet(resource explorer.ResourceNode) (*unstructured.Unstructured, error) {
	client, err := s.client()
	if err != nil {
		return nil, err
	}
	plural, _ := meta.UnsafeGuessKindToResource(schema.GroupVersionKind{Group: resource.Group, Version: resource.Version, Kind: resource.Kind})
	return client.Resource(plural).Namespace(resource.Namespace).Get(context.Background(), resource.Name, metav1.GetOptions{})
}

func TestAPISourceDeletesSendAJSONContentType(t *testing.T) {
	var types []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Content-Type") != "application/json" {
			w.WriteHeader(http.StatusUnsupportedMediaType)
		}
		types = append(types, r.Method+" "+r.Header.Get("Content-Type"))
	}))
	defer server.Close()
	source := NewAPISource(server.URL, "", false)

	if err := source.DeleteResource(context.Background(), "checkout", webPod); err != nil {
		t.Fatalf("DeleteResource() error = %v (requests: %v)", err, types)
	}
	if err := source.DeleteApplication(context.Background(), "checkout"); err != nil {
		t.Fatalf("DeleteApplication() error = %v (requests: %v)", err, types)
	}
}
