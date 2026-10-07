package argocd

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/awcodify/argoxd/internal/explorer"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic/fake"
	k8stesting "k8s.io/client-go/testing"
)

func TestGeneratorSummaryNamesEachGeneratorAndNestedOnes(t *testing.T) {
	generators := []any{
		map[string]any{"list": map[string]any{"elements": []any{}}},
		map[string]any{"git": map[string]any{"repoURL": "x"}},
		map[string]any{"matrix": map[string]any{"generators": []any{
			map[string]any{"git": map[string]any{}},
			map[string]any{"clusters": map[string]any{}},
		}}},
		map[string]any{"merge": map[string]any{"generators": []any{map[string]any{"list": map[string]any{}}}}},
		map[string]any{"selector": map[string]any{}},
	}

	got := generatorSummary(generators)

	want := []string{"list", "git", "matrix(git, clusters)", "merge(list)", "unknown"}
	if !slices.Equal(got, want) {
		t.Fatalf("generators = %q, want %q", got, want)
	}
}

func TestApplicationSetProblemsKeepOnlyWhatIsWrong(t *testing.T) {
	conditions := []appSetCondition{
		{Type: "ErrorOccurred", Status: "False", Message: "fine"},
		{Type: "ParametersGenerated", Status: "True", Message: "fine"},
		{Type: "ResourcesUpToDate", Status: "True", Message: "fine"},
		{Type: "ErrorOccurred", Status: "True", Message: "no matching clusters"},
		{Type: "ParametersGenerated", Status: "False", Message: "generator failed"},
	}

	got := applicationSetProblems(conditions)

	want := []explorer.Condition{
		{Type: "ErrorOccurred", Message: "no matching clusters"},
		{Type: "ParametersGenerated", Message: "generator failed"},
	}
	if !slices.Equal(got, want) {
		t.Fatalf("problems = %+v, want %+v", got, want)
	}
}

func applicationSetObject(name string) unstructured.Unstructured {
	return unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "argoproj.io/v1alpha1",
		"kind":       "ApplicationSet",
		"metadata":   map[string]any{"name": name, "namespace": "argocd"},
		"spec": map[string]any{"generators": []any{
			map[string]any{"git": map[string]any{}},
		}},
		"status": map[string]any{"conditions": []any{
			map[string]any{"type": "ErrorOccurred", "status": "True", "message": "boom"},
			map[string]any{"type": "ResourcesUpToDate", "status": "True", "message": "ok"},
		}},
	}}
}

func TestApplicationSetsFromKubernetesResources(t *testing.T) {
	got := applicationSetsFromKubernetesResources([]unstructured.Unstructured{applicationSetObject("store-services")})

	want := []explorer.ApplicationSet{{
		Name: "store-services", Namespace: "argocd", Generators: []string{"git"},
		Problems: []explorer.Condition{{Type: "ErrorOccurred", Message: "boom"}},
	}}
	if len(got) != 1 || got[0].Name != want[0].Name || got[0].Namespace != want[0].Namespace ||
		!slices.Equal(got[0].Generators, want[0].Generators) || !slices.Equal(got[0].Problems, want[0].Problems) {
		t.Fatalf("application sets = %+v, want %+v", got, want)
	}
}

func TestSnapshotReadsTheApplicationSetAnApplicationBelongsTo(t *testing.T) {
	owned := func(name, kind, owner string) unstructured.Unstructured {
		return unstructured.Unstructured{Object: map[string]any{"metadata": map[string]any{
			"name": name,
			"ownerReferences": []any{
				map[string]any{"apiVersion": "argoproj.io/v1alpha1", "kind": kind, "name": owner},
			},
		}}}
	}
	applications := []unstructured.Unstructured{
		owned("cart", "ApplicationSet", "store-services"),
		owned("other", "Deployment", "web"),
		{Object: map[string]any{"metadata": map[string]any{"name": "manual"}}},
	}

	got := snapshotFromKubernetesResources(applications, nil, nil).Applications

	if got[0].Owner != "store-services" || got[1].Owner != "" || got[2].Owner != "" {
		t.Fatalf("owners = %q, %q, %q, want store-services and two empty", got[0].Owner, got[1].Owner, got[2].Owner)
	}
}

func fakeSourceWithApplicationSets(objects ...runtime.Object) (*KubernetesSource, *fake.FakeDynamicClient) {
	source := fakeKubernetesSource(objects...)
	client, _ := source.client()
	return source, client.(*fake.FakeDynamicClient)
}

func TestKubernetesSourceLoadsApplicationSets(t *testing.T) {
	appSet := applicationSetObject("store-services")
	source, _ := fakeSourceWithApplicationSets(&appSet)

	snapshot, err := source.Load(context.Background())
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if len(snapshot.ApplicationSets) != 1 || snapshot.ApplicationSets[0].Name != "store-services" {
		t.Fatalf("application sets = %+v, want store-services", snapshot.ApplicationSets)
	}
}

func TestKubernetesSourceLoadsWithoutApplicationSetsItCannotList(t *testing.T) {
	resource := schema.GroupResource{Group: "argoproj.io", Resource: "applicationsets"}
	for name, failure := range map[string]error{
		"forbidden": apierrors.NewForbidden(resource, "", errors.New("no access")),
		"not found": apierrors.NewNotFound(resource, ""),
	} {
		t.Run(name, func(t *testing.T) {
			source, client := fakeSourceWithApplicationSets()
			client.PrependReactor("list", "applicationsets", func(k8stesting.Action) (bool, runtime.Object, error) {
				return true, nil, failure
			})

			snapshot, err := source.Load(context.Background())
			if err != nil {
				t.Fatalf("Load() error = %v, want the rest of the snapshot", err)
			}
			if len(snapshot.ApplicationSets) != 0 {
				t.Fatalf("application sets = %+v, want none", snapshot.ApplicationSets)
			}
		})
	}
}

func TestKubernetesSourceReportsOtherApplicationSetErrors(t *testing.T) {
	source, client := fakeSourceWithApplicationSets()
	client.PrependReactor("list", "applicationsets", func(k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, errors.New("connection reset")
	})

	_, err := source.Load(context.Background())

	if err == nil || !strings.Contains(err.Error(), "application sets") || !strings.Contains(err.Error(), "connection reset") {
		t.Fatalf("Load() error = %v, want the application sets failure", err)
	}
}

func applicationSetServer(t *testing.T, applicationSets func(http.ResponseWriter)) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/applications":
			_, _ = w.Write([]byte(`{"items":[{"metadata":{"name":"cart","ownerReferences":[{"kind":"ApplicationSet","name":"store-services"}]}},{"metadata":{"name":"manual"}}]}`))
		case "/api/v1/applicationsets":
			applicationSets(w)
		default:
			_, _ = w.Write([]byte(`{"items":[]}`))
		}
	}))
}

func TestAPISourceLoadsApplicationSetsAndWhoOwnsEachApplication(t *testing.T) {
	server := applicationSetServer(t, func(w http.ResponseWriter) {
		_, _ = w.Write([]byte(`{"items":[{"metadata":{"name":"store-services","namespace":"argocd"},
			"spec":{"generators":[{"git":{}},{"clusters":{}}]},
			"status":{"conditions":[{"type":"ErrorOccurred","status":"True","message":"boom"},{"type":"ResourcesUpToDate","status":"True","message":"ok"}]}}]}`))
	})
	defer server.Close()

	snapshot, err := NewAPISource(server.URL, "", false).Load(context.Background())
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if len(snapshot.ApplicationSets) != 1 {
		t.Fatalf("application sets = %+v, want one", snapshot.ApplicationSets)
	}
	appSet := snapshot.ApplicationSets[0]
	if appSet.Name != "store-services" || !slices.Equal(appSet.Generators, []string{"git", "clusters"}) ||
		!slices.Equal(appSet.Problems, []explorer.Condition{{Type: "ErrorOccurred", Message: "boom"}}) {
		t.Fatalf("application set = %+v", appSet)
	}
	if snapshot.Applications[0].Owner != "store-services" || snapshot.Applications[1].Owner != "" {
		t.Fatalf("owners = %q, %q, want store-services and none", snapshot.Applications[0].Owner, snapshot.Applications[1].Owner)
	}
}

func TestAPISourceLoadsWithoutApplicationSetsItCannotList(t *testing.T) {
	for name, status := range map[string]int{"forbidden": http.StatusForbidden, "not found": http.StatusNotFound} {
		t.Run(name, func(t *testing.T) {
			server := applicationSetServer(t, func(w http.ResponseWriter) { http.Error(w, "nope", status) })
			defer server.Close()

			snapshot, err := NewAPISource(server.URL, "", false).Load(context.Background())
			if err != nil {
				t.Fatalf("Load() error = %v, want the rest of the snapshot", err)
			}
			if len(snapshot.ApplicationSets) != 0 || len(snapshot.Applications) != 2 {
				t.Fatalf("snapshot = %+v, want applications and no application sets", snapshot)
			}
		})
	}
}

func TestAPISourceReportsOtherApplicationSetErrors(t *testing.T) {
	server := applicationSetServer(t, func(w http.ResponseWriter) { http.Error(w, "broken", http.StatusInternalServerError) })
	defer server.Close()

	_, err := NewAPISource(server.URL, "", false).Load(context.Background())

	if err == nil || !strings.Contains(err.Error(), "application sets") {
		t.Fatalf("Load() error = %v, want the application sets failure", err)
	}
}

func TestDemoApplicationSetsOwnTheApplicationsTheyName(t *testing.T) {
	snapshot, _ := NewDemoSource().Load(context.Background())

	owned := map[string]int{}
	for _, application := range snapshot.Applications {
		if application.Owner != "" {
			owned[application.Owner]++
		}
	}
	if len(snapshot.ApplicationSets) < 2 {
		t.Fatalf("application sets = %+v, want at least 2", snapshot.ApplicationSets)
	}
	problems := 0
	for _, appSet := range snapshot.ApplicationSets {
		if owned[appSet.Name] == 0 || len(appSet.Generators) == 0 {
			t.Fatalf("%s generates %d applications with generators %v, want some of both", appSet.Name, owned[appSet.Name], appSet.Generators)
		}
		problems += len(appSet.Problems)
		delete(owned, appSet.Name)
	}
	if len(owned) != 0 {
		t.Fatalf("applications owned by unknown application sets: %v", owned)
	}
	if problems == 0 {
		t.Fatal("no demo application set has a problem to show")
	}
}
