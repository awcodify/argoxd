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
