package argocd

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAPISourceTerminatesOperation(t *testing.T) {
	var request string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		request = r.Method + " " + r.URL.Path
	}))
	defer server.Close()

	if err := NewAPISource(server.URL, "", false).TerminateOperation(context.Background(), "payments"); err != nil {
		t.Fatalf("TerminateOperation() error = %v", err)
	}
	if request != "DELETE /api/v1/applications/payments/operation" {
		t.Fatalf("request = %q", request)
	}
}

func TestAPISourceInspectsApplicationsAndOnlyReturnsDriftedResources(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/applications/checkout":
			_, _ = w.Write([]byte(`{"apiVersion":"argoproj.io/v1alpha1","kind":"Application","spec":{"source":{"repoURL":"https://example.com/store.git"}}}`))
		case "/api/v1/applications/checkout/managed-resources":
			_, _ = w.Write([]byte(`{"items":[
				{"kind":"Deployment","namespace":"store","name":"web","normalizedLiveState":"{\"spec\":{\"replicas\":1}}","predictedLiveState":"{\"spec\":{\"replicas\":2}}"},
				{"kind":"Service","namespace":"store","name":"web","normalizedLiveState":"{\"spec\":{\"ports\":[80]}}","predictedLiveState":"{\"spec\":{\"ports\":[80]}}"}
			]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	source := NewAPISource(server.URL, "", false)

	manifest, err := source.ApplicationManifest(context.Background(), "checkout")
	if err != nil || !strings.Contains(manifest, "repoURL: https://example.com/store.git") {
		t.Fatalf("ApplicationManifest() = %q, %v", manifest, err)
	}
	diff, err := source.ApplicationDiff(context.Background(), "checkout")
	if err != nil || !strings.Contains(diff, "## Deployment/web (store)") || strings.Contains(diff, "Service/web") {
		t.Fatalf("ApplicationDiff() = %q, %v", diff, err)
	}
}
