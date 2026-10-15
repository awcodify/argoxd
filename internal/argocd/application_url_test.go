package argocd

import "testing"

func TestAPISourceApplicationURL(t *testing.T) {
	source := NewAPISource("https://argocd.example.com/", "", false)

	if url := source.ApplicationURL("team/payments"); url != "https://argocd.example.com/applications/team%2Fpayments" {
		t.Fatalf("ApplicationURL() = %q", url)
	}
}
