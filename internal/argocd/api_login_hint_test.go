package argocd

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestExpiredLoginErrorsPointAtArgoCDLoginWhenTheTokenCameFromTheCLI(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"invalid session","code":16,"message":"invalid session"}`))
	}))
	defer server.Close()

	hinted, _ := NewConfiguredAPISource(server.URL, "old", APIOptions{TokenFromCLI: true})
	if _, err := hinted.Load(context.Background()); err == nil || !strings.Contains(err.Error(), "argocd login") {
		t.Fatalf("a CLI token that was refused: error = %v, want it to suggest argocd login", err)
	}
	plain, _ := NewConfiguredAPISource(server.URL, "old", APIOptions{})
	if _, err := plain.Load(context.Background()); err == nil || strings.Contains(err.Error(), "argocd login") {
		t.Fatalf("a token the user gave: error = %v, want no argocd login advice", err)
	}
}

func TestTheLoginAdviceComesBeforeTheServersLongDetail(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"invalid session: token is malformed: could not JSON decode header: invalid character 'e' looking for beginning of value","code":16,"message":"invalid session: token is malformed: could not JSON decode header: invalid character 'e' looking for beginning of value"}`))
	}))
	defer server.Close()
	source, _ := NewConfiguredAPISource(server.URL, "old", APIOptions{TokenFromCLI: true})

	_, err := source.Load(context.Background())

	if err == nil {
		t.Fatal("Load() = nil, want an error")
	}
	advice, detail := strings.Index(err.Error(), "argocd login"), strings.Index(err.Error(), "malformed")
	if advice < 0 || detail < 0 || advice > detail || advice > 40 {
		t.Fatalf("error = %q; want the advice (at %d) before the detail (at %d) and near the front", err.Error(), advice, detail)
	}
}
