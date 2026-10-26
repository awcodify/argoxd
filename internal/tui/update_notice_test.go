package tui

import (
	"context"
	"strings"
	"testing"

	"github.com/awcodify/argoxd/internal/explorer"
)

func TestHeaderAnnouncesANewerRelease(t *testing.T) {
	source := &fakeSource{snapshot: storeSnapshot()}
	model := New(source, "Kubeconfig current context", "argocd", explorer.Snapshot{}).
		WithUpdateCheck(func(context.Context) string { return "0.4.2" })

	cmd := model.Init()
	if cmd == nil {
		t.Fatal("Init() = nil, want the load and the update check")
	}
	model = settle(model, source.loadCommand())
	updated, _ := model.Update(updateAvailable{version: "0.4.2"})
	view := updated.(Model).View()

	if !strings.Contains(view, "0.4.2") || !strings.Contains(view, "available") {
		t.Fatalf("header does not announce the update:\n%s", view)
	}
}

func TestHeaderShowsNothingWithoutAnUpdate(t *testing.T) {
	source := &fakeSource{snapshot: storeSnapshot()}
	model := settle(New(source, "Kubeconfig current context", "argocd", explorer.Snapshot{}), source.loadCommand())

	updated, _ := model.Update(updateAvailable{})

	if view := updated.(Model).View(); strings.Contains(view, "available") {
		t.Fatalf("header mentions an update when there is none:\n%s", view)
	}
}

func TestUpdateCheckRunsOffTheMainLoop(t *testing.T) {
	var asked bool
	model := New(&fakeSource{snapshot: storeSnapshot()}, "Kubeconfig current context", "argocd", explorer.Snapshot{}).
		WithUpdateCheck(func(context.Context) string { asked = true; return "0.4.2" })

	_ = model.Init()
	if asked {
		t.Fatal("Init() ran the update check synchronously; it must run inside a command")
	}
	message := model.checkForUpdate()()
	if notice, ok := message.(updateAvailable); !ok || notice.version != "0.4.2" || !asked {
		t.Fatalf("checkForUpdate() = %#v, asked = %v", message, asked)
	}
}
