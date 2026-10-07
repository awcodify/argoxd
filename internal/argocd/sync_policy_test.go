package argocd

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"

	"github.com/awcodify/argoxd/internal/explorer"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func applicationWithPolicy(syncPolicy map[string]any) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "argoproj.io/v1alpha1",
		"kind":       "Application",
		"metadata":   map[string]any{"namespace": "argocd", "name": "checkout"},
		"spec":       map[string]any{"project": "store", "syncPolicy": syncPolicy},
	}}
}

func storedSyncPolicy(t *testing.T, source *KubernetesSource) map[string]any {
	t.Helper()
	client, _ := source.client()
	got, err := client.Resource(applicationsResource).Namespace("argocd").Get(context.Background(), "checkout", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("get application: %v", err)
	}
	policy, _, _ := unstructured.NestedMap(got.Object, "spec", "syncPolicy")
	return policy
}

func TestKubernetesSourceTurnsOnAutoSyncWithSelfHealAndKeepsOtherOptions(t *testing.T) {
	source := fakeKubernetesSource(applicationWithPolicy(map[string]any{"syncOptions": []any{"CreateNamespace=true"}}))

	err := source.SetSyncPolicy(context.Background(), "checkout", explorer.SyncPolicy{Automated: true, SelfHeal: true})
	if err != nil {
		t.Fatalf("SetSyncPolicy() error = %v", err)
	}

	policy := storedSyncPolicy(t, source)
	if selfHeal, _, _ := unstructured.NestedBool(policy, "automated", "selfHeal"); !selfHeal {
		t.Fatalf("sync policy = %v, want self-heal on", policy)
	}
	if prune, found, _ := unstructured.NestedBool(policy, "automated", "prune"); !found || prune {
		t.Fatalf("sync policy = %v, want prune explicitly off", policy)
	}
	if options, _, _ := unstructured.NestedStringSlice(policy, "syncOptions"); !slices.Equal(options, []string{"CreateNamespace=true"}) {
		t.Fatalf("sync policy = %v, want sync options kept", policy)
	}
}

func TestKubernetesSourceTurnsOffAutoSyncAndKeepsOtherOptions(t *testing.T) {
	source := fakeKubernetesSource(applicationWithPolicy(map[string]any{
		"automated":   map[string]any{"prune": true, "selfHeal": true},
		"syncOptions": []any{"CreateNamespace=true"},
	}))

	if err := source.SetSyncPolicy(context.Background(), "checkout", explorer.SyncPolicy{}); err != nil {
		t.Fatalf("SetSyncPolicy() error = %v", err)
	}

	policy := storedSyncPolicy(t, source)
	if _, found := policy["automated"]; found {
		t.Fatalf("sync policy = %v, want auto-sync removed", policy)
	}
	if options, _, _ := unstructured.NestedStringSlice(policy, "syncOptions"); !slices.Equal(options, []string{"CreateNamespace=true"}) {
		t.Fatalf("sync policy = %v, want sync options kept", policy)
	}
}

func TestKubernetesSourceSyncPolicyOfUnknownApplicationFails(t *testing.T) {
	err := fakeKubernetesSource().SetSyncPolicy(context.Background(), "missing", explorer.SyncPolicy{Automated: true})
	if err == nil {
		t.Fatal("SetSyncPolicy() error = nil, want an error")
	}
}

func TestAPISourcePatchesTheSyncPolicy(t *testing.T) {
	var requests []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		requests = append(requests, r.Method+" "+r.URL.RequestURI()+" "+string(body))
		_, _ = w.Write([]byte(`{}`))
	}))
	defer server.Close()
	source := NewAPISource(server.URL, "", false)

	if err := source.SetSyncPolicy(context.Background(), "payments", explorer.SyncPolicy{Automated: true, Prune: true}); err != nil {
		t.Fatalf("SetSyncPolicy() error = %v", err)
	}
	if err := source.SetSyncPolicy(context.Background(), "payments", explorer.SyncPolicy{}); err != nil {
		t.Fatalf("SetSyncPolicy() error = %v", err)
	}

	want := []string{
		`PATCH /api/v1/applications/payments {"patch":"{\"spec\":{\"syncPolicy\":{\"automated\":{\"prune\":true,\"selfHeal\":false}}}}","patchType":"merge"}`,
		`PATCH /api/v1/applications/payments {"patch":"{\"spec\":{\"syncPolicy\":{\"automated\":null}}}","patchType":"merge"}`,
	}
	if !slices.Equal(requests, want) {
		t.Fatalf("requests = %q, want %q", requests, want)
	}
}

func TestDemoSourceChangesTheSyncPolicy(t *testing.T) {
	source := NewDemoSource()
	ctx := context.Background()

	if err := source.SetSyncPolicy(ctx, "cart", explorer.SyncPolicy{Automated: true, SelfHeal: true}); err != nil {
		t.Fatalf("SetSyncPolicy() error = %v", err)
	}
	if err := source.SetSyncPolicy(ctx, "missing", explorer.SyncPolicy{}); err == nil {
		t.Fatal("SetSyncPolicy() of an unknown application = nil, want an error")
	}

	snapshot, _ := source.Load(ctx)
	for _, application := range snapshot.Applications {
		if application.Name == "cart" && application.Policy != (explorer.SyncPolicy{Automated: true, SelfHeal: true}) {
			t.Fatalf("policy = %+v, want auto-sync with self-heal", application.Policy)
		}
	}
}

func TestDemoSourceStartsWithAMixOfSyncPolicies(t *testing.T) {
	snapshot, _ := NewDemoSource().Load(context.Background())

	automated := 0
	for _, application := range snapshot.Applications {
		if application.Policy.Automated {
			automated++
		}
	}
	if automated == 0 || automated == len(snapshot.Applications) {
		t.Fatalf("%d of %d demo applications sync automatically, want some but not all", automated, len(snapshot.Applications))
	}
}
