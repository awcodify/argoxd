package tui

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/awcodify/argoxd/internal/explorer"
)

func refreshOnce(t *testing.T, model Model) Model {
	t.Helper()
	updated, command := model.Update(refreshTick{})
	return settle(updated.(Model), command)
}

func TestARefreshKeepsTheStatusMessage(t *testing.T) {
	source := &fakeSource{snapshot: storeSnapshot()}
	model := New(source, "test", "argocd", storeSnapshot()).WithRefresh(time.Millisecond)
	model.status = "sync requested for checkout"

	if view := refreshOnce(t, model).View(); !strings.Contains(view, "sync requested for checkout") {
		t.Fatalf("the refresh cleared the status message:\n%s", view)
	}
}

func TestARefreshKeepsTheErrorOfAnAction(t *testing.T) {
	source := &fakeSource{snapshot: storeSnapshot()}
	model := New(source, "test", "argocd", storeSnapshot()).WithRefresh(time.Millisecond)
	updated, _ := model.Update(operationCompleted{action: "sync", application: "checkout", err: errors.New("permission denied: applications, sync")})

	if view := refreshOnce(t, updated.(Model)).View(); !strings.Contains(view, "permission denied") {
		t.Fatalf("the refresh cleared the sync's error:\n%s", view)
	}
}

func TestARefreshKeepsTheErrorOfOpeningAnApplication(t *testing.T) {
	source := &fakeSource{snapshot: storeSnapshot()}
	model := New(source, "test", "argocd", storeSnapshot()).WithRefresh(time.Millisecond)
	updated, _ := model.Update(loadedTree{err: errors.New("load resource tree: forbidden")})

	if view := refreshOnce(t, updated.(Model)).View(); !strings.Contains(view, "forbidden") {
		t.Fatalf("the refresh cleared the error:\n%s", view)
	}
}

func TestARefreshThatWorksAgainClearsTheRefreshError(t *testing.T) {
	source := &fakeSource{snapshot: storeSnapshot()}
	model := New(source, "test", "argocd", explorer.Snapshot{}).WithRefresh(time.Millisecond)
	failed, _ := model.Update(loadedSnapshot{err: errors.New("load applications: connection refused")})

	if view := refreshOnce(t, failed.(Model)).View(); strings.Contains(view, "connection refused") {
		t.Fatalf("a refresh that worked kept the old refresh error:\n%s", view)
	}
}
