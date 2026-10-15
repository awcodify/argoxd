package tui

import (
	"context"
	"strings"
	"testing"

	"github.com/awcodify/argoxd/internal/explorer"
)

type projectSource struct {
	*fakeSource
	manifest string
}

func (s *projectSource) ProjectManifest(context.Context, string) (string, error) {
	return s.manifest, nil
}

func TestProjectYShowsAppProjectManifest(t *testing.T) {
	source := &projectSource{
		fakeSource: &fakeSource{snapshot: storeSnapshot()},
		manifest:   "kind: AppProject\nspec:\n  sourceRepos:\n    - https://example.com/platform.git\n",
	}
	model := settle(New(source, "test", "argocd", explorer.Snapshot{}), source.loadCommand())
	model = command(model, ":proj")

	viewer := run(model, "Y")
	if view := viewer.View(); !strings.Contains(view, "kind: AppProject") || !strings.Contains(view, "platform.git") {
		t.Fatalf("Y did not show the AppProject manifest:\n%s", view)
	}
	if view := press(viewer, "esc").View(); !strings.Contains(view, "projects") {
		t.Fatalf("escape did not return to the projects list:\n%s", view)
	}
}
