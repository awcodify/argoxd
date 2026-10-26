package argocd

import (
	"context"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/awcodify/argoxd/internal/explorer"
)

var rolloutNode = explorer.ResourceNode{Group: "argoproj.io", Version: "v1alpha1", Kind: "Rollout", Namespace: "store", Name: "web"}

func TestAPISourceListsTheActionsArgoCDOffersForAResource(t *testing.T) {
	var method, path string
	var query map[string][]string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method, path, query = r.Method, r.URL.Path, r.URL.Query()
		_, _ = w.Write([]byte(`{"actions":[{"name":"promote-full","disabled":false},{"name":"abort","disabled":true},{"name":"restart"}]}`))
	}))
	defer server.Close()

	actions, err := NewAPISource(server.URL, "", false).ResourceActions(context.Background(), "checkout", rolloutNode)
	if err != nil {
		t.Fatalf("ResourceActions() error = %v", err)
	}

	want := []ResourceAction{{Name: "promote-full"}, {Name: "abort", Disabled: true}, {Name: "restart"}}
	if !slices.Equal(actions, want) {
		t.Fatalf("actions = %+v, want %+v", actions, want)
	}
	if method != http.MethodGet || path != "/api/v1/applications/checkout/resource/actions" {
		t.Fatalf("request = %s %s, want a GET of the resource's actions", method, path)
	}
	for key, want := range map[string]string{"resourceName": "web", "namespace": "store", "kind": "Rollout", "group": "argoproj.io", "version": "v1alpha1"} {
		if got := query[key]; len(got) != 1 || got[0] != want {
			t.Fatalf("query = %v, want %s = %s", query, key, want)
		}
	}
}

func TestAPISourceMarksActionsThatNeedParameters(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"actions":[{"name":"scale","params":[{"name":"replicas"}]},{"name":"pause"},{"name":"resume","disabled":true}]}`))
	}))
	defer server.Close()

	actions, err := NewAPISource(server.URL, "", false).ResourceActions(context.Background(), "checkout", webDeployment)

	want := []ResourceAction{{Name: "scale", NeedsParameters: true}, {Name: "pause"}, {Name: "resume", Disabled: true}}
	if err != nil || !slices.Equal(actions, want) {
		t.Fatalf("ResourceActions() = %+v, %v; want %+v", actions, err, want)
	}
}

func TestAPISourceTreatsAResourceWithoutActionsAsEmpty(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte(`{}`)) }))
	defer server.Close()

	actions, err := NewAPISource(server.URL, "", false).ResourceActions(context.Background(), "checkout", webDeployment)

	if err != nil || len(actions) != 0 {
		t.Fatalf("ResourceActions() = %v, %v; want no actions and no error", actions, err)
	}
}

func TestAPISourceRunsAnyNamedResourceAction(t *testing.T) {
	var method, body string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method = r.Method
		payload := make([]byte, 64)
		n, _ := r.Body.Read(payload)
		body = string(payload[:n])
	}))
	defer server.Close()

	if err := NewAPISource(server.URL, "", false).RunResourceAction(context.Background(), "checkout", rolloutNode, "promote-full"); err != nil {
		t.Fatalf("RunResourceAction() error = %v", err)
	}

	if method != http.MethodPost || body != `"promote-full"` {
		t.Fatalf("request = %s with body %s, want a POST of the JSON string \"promote-full\"", method, body)
	}
}

func TestAPISourceActionsReportArgoCDsMessage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"message":"permission denied"}`))
	}))
	defer server.Close()
	source := NewAPISource(server.URL, "", false)

	if _, err := source.ResourceActions(context.Background(), "checkout", rolloutNode); err == nil || !strings.Contains(err.Error(), "permission denied") {
		t.Fatalf("ResourceActions() error = %v, want Argo CD's message", err)
	}
	if err := source.RunResourceAction(context.Background(), "checkout", rolloutNode, "abort"); err == nil || !strings.Contains(err.Error(), "permission denied") {
		t.Fatalf("RunResourceAction() error = %v, want Argo CD's message", err)
	}
}

func TestKubernetesSourceOffersOnlyRestartAndExplainsTheRest(t *testing.T) {
	source := fakeKubernetesSource(workloadObject("apps/v1", "Deployment"))

	actions, err := source.ResourceActions(context.Background(), "checkout", webDeployment)
	if err != nil || !slices.Equal(actions, []ResourceAction{{Name: "restart"}}) {
		t.Fatalf("ResourceActions(Deployment) = %v, %v; want only restart", actions, err)
	}
	if actions, err = source.ResourceActions(context.Background(), "checkout", containerNode("web-abc")); err != nil || len(actions) != 0 {
		t.Fatalf("ResourceActions(Pod) = %v, %v; want none", actions, err)
	}

	if err := source.RunResourceAction(context.Background(), "checkout", webDeployment, "restart"); err != nil {
		t.Fatalf("RunResourceAction(restart) error = %v", err)
	}
	err = source.RunResourceAction(context.Background(), "checkout", rolloutNode, "promote-full")
	if err == nil || !strings.Contains(err.Error(), "--source api") {
		t.Fatalf("RunResourceAction(promote-full) error = %v, want a pointer to --source api", err)
	}
}

func TestDemoSourceOffersRestartOnWorkloadsAndRunsIt(t *testing.T) {
	source := NewDemoSource()
	snapshot, err := source.Load(context.Background())
	if err != nil || len(snapshot.Applications) == 0 {
		t.Fatalf("Load() = %v, %v", snapshot.Applications, err)
	}
	tree, err := source.LoadResourceTree(context.Background(), snapshot.Applications[0].Name)
	if err != nil {
		t.Fatal(err)
	}
	var workload explorer.ResourceNode
	for _, node := range tree.Nodes {
		if Restartable(node.Kind) {
			workload = node
		}
	}
	if workload.Name == "" {
		t.Skip("the first demo Application has no restartable workload")
	}

	actions, err := source.ResourceActions(context.Background(), tree.Application, workload)
	if err != nil || !slices.Contains(actions, ResourceAction{Name: "restart"}) {
		t.Fatalf("ResourceActions() = %v, %v; want restart among them", actions, err)
	}
	if err := source.RunResourceAction(context.Background(), tree.Application, workload, "restart"); err != nil {
		t.Fatalf("RunResourceAction(restart) error = %v", err)
	}
	if err := source.RunResourceAction(context.Background(), tree.Application, workload, "no-such-action"); err == nil {
		t.Fatal("running an unknown action succeeded")
	}
}
