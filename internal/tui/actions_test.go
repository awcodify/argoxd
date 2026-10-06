package tui

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/awcodify/argoxd/internal/argocd"
	"github.com/awcodify/argoxd/internal/explorer"
	tea "github.com/charmbracelet/bubbletea"
)

func TestRequestsGiveUpAfterTheTimeout(t *testing.T) {
	source := &fakeSource{hang: true}
	model := New(source, "test", "argocd", explorer.Snapshot{})
	model.requestTimeout = 10 * time.Millisecond

	view := settle(model, model.Init()).View()

	if !strings.Contains(view, "deadline exceeded") {
		t.Fatalf("a hung request was not reported:\n%s", view)
	}
}

func TestAutoRefreshReloadsTheSnapshot(t *testing.T) {
	source := &fakeSource{snapshot: storeSnapshot()}
	model := New(source, "test", "argocd", explorer.Snapshot{}).WithRefresh(time.Millisecond)

	updated, command := model.Update(refreshTick{})
	view := settle(updated.(Model), command).View()

	if source.loads != 1 || !strings.Contains(view, "checkout") {
		t.Fatalf("refresh tick loaded %d times:\n%s", source.loads, view)
	}
}

func TestAutoRefreshSkipsWhileARefreshIsInFlight(t *testing.T) {
	source := &fakeSource{snapshot: storeSnapshot()}
	model := New(source, "test", "argocd", explorer.Snapshot{}).WithRefresh(time.Millisecond)

	first, _ := model.Update(refreshTick{})
	second, command := first.Update(refreshTick{})
	settle(second.(Model), command)

	if source.loads != 0 {
		t.Fatalf("a second tick started another load while one was in flight")
	}
}

func TestAutoRefreshKeepsTheSelectedCard(t *testing.T) {
	source := &fakeSource{snapshot: storeSnapshot(), tree: checkoutTree()}
	model := openCheckout(t, source).WithRefresh(time.Millisecond)
	model = typeKeys(model, "j", "j")

	updated, command := model.Update(refreshTick{})
	view := settle(updated.(Model), command).View()

	if !strings.Contains(view, "Kind       Pod") {
		t.Fatalf("refresh lost the selected card:\n%s", view)
	}
}

func TestSlashSearchesTheListAsYouType(t *testing.T) {
	model := New(nil, "test", "argocd", storeSnapshot())

	filtered := typeKeys(model, "/", "c", "h")
	if view := filtered.View(); !strings.Contains(view, "checkout") || strings.Contains(view, "grafana") {
		t.Fatalf("search did not narrow the list:\n%s", view)
	}

	kept := press(filtered, "enter")
	if view := kept.View(); !strings.Contains(view, "/ch") || strings.Contains(view, "grafana") {
		t.Fatalf("enter did not keep the search:\n%s", view)
	}

	if view := press(kept, "esc").View(); !strings.Contains(view, "grafana") {
		t.Fatalf("escape did not clear the search:\n%s", view)
	}
}

func TestSyncDialogAppliesTheChosenOptions(t *testing.T) {
	source := &fakeSource{snapshot: storeSnapshot()}
	model := settle(New(source, "test", "argocd", explorer.Snapshot{}), source.loadCommand())
	model = typeKeys(model, "j", "s")

	if view := model.View(); !strings.Contains(view, "Sync checkout?") {
		t.Fatalf("s did not open the sync dialog:\n%s", view)
	}
	updated, command := typeKeys(model, "p").Update(key("enter"))
	settle(updated.(Model), command)

	if len(source.synced) != 1 || source.synced[0] != (argocd.SyncOptions{Prune: true}) {
		t.Fatalf("synced = %+v, want one pruning sync", source.synced)
	}
}

func TestShiftRHardRefreshesTheApplication(t *testing.T) {
	source := &fakeSource{snapshot: storeSnapshot()}
	model := settle(New(source, "test", "argocd", explorer.Snapshot{}), source.loadCommand())

	updated, command := model.Update(key("R"))
	settle(updated.(Model), command)

	if len(source.refreshed) != 1 || source.refreshed[0] != "grafana" {
		t.Fatalf("refreshed = %v, want grafana", source.refreshed)
	}
}

func TestCardActionsOpenTheViewer(t *testing.T) {
	source := &fakeSource{
		snapshot: storeSnapshot(),
		tree:     checkoutTree(),
		manifest: "kind: Deployment\nspec:\n  replicas: 2",
		diff:     "  kind: Deployment\n- replicas: 1\n+ replicas: 2",
		logs:     "starting\nready",
	}
	deployment := press(openCheckout(t, source), "j")

	yaml := run(deployment, "y")
	if view := yaml.View(); !strings.Contains(view, "replicas: 2") || !strings.Contains(view, "checkout › yaml") {
		t.Fatalf("y did not show the manifest:\n%s", view)
	}
	if view := press(yaml, "esc").View(); !strings.Contains(view, "Kind       Deployment") {
		t.Fatalf("escape did not return to the selected card:\n%s", view)
	}

	if view := run(deployment, "d").View(); !strings.Contains(view, "+ replicas: 2") {
		t.Fatalf("d did not show the diff:\n%s", view)
	}

	if view := run(deployment, "l").View(); !strings.Contains(view, "only available for Pods") {
		t.Fatalf("l on a Deployment was not refused:\n%s", view)
	}
	if view := run(press(deployment, "j"), "l").View(); !strings.Contains(view, "ready") {
		t.Fatalf("l on a Pod did not show logs:\n%s", view)
	}
}

func TestViewerScrollsToTheEnd(t *testing.T) {
	var lines []string
	for index := range 100 {
		lines = append(lines, fmt.Sprintf("line-%03d", index))
	}
	source := &fakeSource{snapshot: storeSnapshot(), tree: checkoutTree(), manifest: strings.Join(lines, "\n")}
	model := run(press(resize(openCheckout(t, source), 100, 30), "j"), "y")

	view := press(model, "G").View()

	if !strings.Contains(view, "line-099") || strings.Contains(view, "line-000") {
		t.Fatalf("G did not scroll to the last line:\n%s", view)
	}
}

func TestApplicationsTableShowsRevisionDestinationAndLastSync(t *testing.T) {
	model := New(nil, "test", "argocd", explorer.Snapshot{Applications: []explorer.Application{{
		Name: "checkout", Revision: "main", Destination: "in-cluster/store", LastSync: time.Now().Add(-5 * time.Minute),
	}}})

	view := model.View()

	for _, want := range []string{"REVISION", "DESTINATION", "LAST SYNC", "main", "in-cluster/store", "5m"} {
		if !strings.Contains(view, want) {
			t.Fatalf("applications table does not contain %q:\n%s", want, view)
		}
	}
}

// fakeSource records operations and serves canned resources.
type fakeSource struct {
	snapshot  explorer.Snapshot
	tree      explorer.ResourceTree
	hang      bool
	loads     int
	synced    []argocd.SyncOptions
	refreshed []string
	manifest  string
	diff      string
	logs      string

	history    []explorer.HistoryEntry
	rolledBack []rollbackCall
}

func (f *fakeSource) Load(ctx context.Context) (explorer.Snapshot, error) {
	if f.hang {
		<-ctx.Done()
		return explorer.Snapshot{}, ctx.Err()
	}
	f.loads++
	return f.snapshot, nil
}

func (f *fakeSource) loadCommand() tea.Cmd {
	return func() tea.Msg { return loadedSnapshot{snapshot: f.snapshot} }
}

func (f *fakeSource) LoadResourceTree(context.Context, string) (explorer.ResourceTree, error) {
	return f.tree, nil
}

func (f *fakeSource) SyncApplication(_ context.Context, _ string, options argocd.SyncOptions) error {
	f.synced = append(f.synced, options)
	return nil
}

func (f *fakeSource) RefreshApplication(_ context.Context, application string) error {
	f.refreshed = append(f.refreshed, application)
	return nil
}

func (f *fakeSource) DeleteApplication(context.Context, string) error { return nil }

func (f *fakeSource) ResourceManifest(context.Context, string, explorer.ResourceNode) (string, error) {
	return f.manifest, nil
}

func (f *fakeSource) ResourceDiff(context.Context, string, explorer.ResourceNode) (string, error) {
	return f.diff, nil
}

func (f *fakeSource) ResourceLogs(context.Context, string, explorer.ResourceNode) (string, error) {
	return f.logs, nil
}

func checkoutTree() explorer.ResourceTree {
	return explorer.ResourceTree{Application: "checkout", Nodes: []explorer.ResourceNode{
		{Group: "apps", Version: "v1", Kind: "Deployment", Namespace: "store", Name: "web", Health: "Healthy"},
		{Version: "v1", Kind: "Pod", Namespace: "store", Name: "web-abc", Health: "Healthy",
			Parents: []explorer.ResourceReference{{Group: "apps", Kind: "Deployment", Namespace: "store", Name: "web"}}},
	}}
}

// openCheckout loads the snapshot and opens the checkout Application's dependencies.
func openCheckout(t *testing.T, source *fakeSource) Model {
	t.Helper()
	model := resize(New(source, "test", "argocd", explorer.Snapshot{}), 140, 40)
	model = settle(model, source.loadCommand())
	model = press(model, "j")
	model = run(model, "enter")
	if !strings.Contains(model.View(), "applications › checkout") {
		t.Fatalf("checkout did not open:\n%s", model.View())
	}
	return model
}

// run presses a key and settles the command it returns.
func run(model Model, value string) Model {
	updated, command := model.Update(key(value))
	return settle(updated.(Model), command)
}

// settle runs a command and feeds its messages back into the model, the way
// Bubble Tea would. Refresh ticks are dropped so tests never loop.
func settle(model Model, command tea.Cmd) Model {
	if command == nil {
		return model
	}
	switch message := command().(type) {
	case tea.BatchMsg:
		for _, command := range message {
			model = settle(model, command)
		}
		return model
	case refreshTick:
		return model
	default:
		updated, next := model.Update(message)
		return settle(updated.(Model), next)
	}
}

func TestHeaderHintsFollowTheActiveView(t *testing.T) {
	source := &fakeSource{snapshot: storeSnapshot(), tree: checkoutTree(), manifest: "kind: Deployment"}
	list := settle(resize(New(source, "test", "argocd", explorer.Snapshot{}), 160, 40), source.loadCommand())
	dependencies := openCheckout(t, source)
	viewer := run(press(dependencies, "j"), "y")

	for name, test := range map[string]struct {
		view string
		want []string
	}{
		"list":         {list.View(), []string{"/  Search", "H  Health", "S  Sync status", "R  Hard refresh", "D  Delete"}},
		"dependencies": {dependencies.View(), []string{"y  YAML", "d  Diff", "l  Logs", "D  Delete", "/  Search", "K  Kind"}},
		"viewer":       {viewer.View(), []string{"G  Bottom", "esc  Back"}},
	} {
		for _, want := range test.want {
			if !strings.Contains(test.view, want) {
				t.Fatalf("%s header does not contain %q:\n%s", name, want, test.view)
			}
		}
	}
}

func TestDiffKeyDoesNotDeleteFromTheDependencyView(t *testing.T) {
	source := &fakeSource{snapshot: storeSnapshot(), tree: checkoutTree(), diff: "+ replicas: 2"}

	view := run(press(openCheckout(t, source), "j"), "d").View()

	if strings.Contains(view, "Delete checkout") {
		t.Fatalf("d asked to delete the Application instead of showing the diff:\n%s", view)
	}
}

func TestSlashSearchesTheDependencyTree(t *testing.T) {
	source := &fakeSource{snapshot: storeSnapshot(), tree: explorer.ResourceTree{Application: "checkout", Nodes: []explorer.ResourceNode{
		{Group: "apps", Kind: "Deployment", Namespace: "store", Name: "web"},
		{Kind: "Pod", Namespace: "store", Name: "web-abc", Parents: []explorer.ResourceReference{{Group: "apps", Kind: "Deployment", Namespace: "store", Name: "web"}}},
		{Kind: "Service", Namespace: "store", Name: "frontend"},
	}}}
	model := openCheckout(t, source)

	filtered := typeKeys(model, "/", "p", "o", "d", "enter")
	view := filtered.View()
	if !strings.Contains(view, "web-abc") || !strings.Contains(view, "Deployment") || strings.Contains(view, "frontend") {
		t.Fatalf("search did not keep the Pod with its Deployment and hide the Service:\n%s", view)
	}
	if !strings.Contains(view, "/pod") {
		t.Fatalf("title does not show the search:\n%s", view)
	}

	selected := typeKeys(filtered, "j", "j").View()
	if !strings.Contains(selected, "Kind       Pod") {
		t.Fatalf("cursor did not move through the filtered cards:\n%s", selected)
	}

	cleared := press(filtered, "esc")
	if view := cleared.View(); !strings.Contains(view, "frontend") || !strings.Contains(view, "applications › checkout") {
		t.Fatalf("escape did not clear the search and stay on the tree:\n%s", view)
	}
	if view := press(cleared, "esc").View(); strings.Contains(view, "› checkout") {
		t.Fatalf("a second escape did not return to the list:\n%s", view)
	}
}

func TestHealthKeyOpensASuggestionBarThatFiltersApplications(t *testing.T) {
	model := resize(New(nil, "test", "argocd", storeSnapshot()), 160, 40)

	bar := press(model, "H")
	for _, want := range []string{"health", "Healthy", "Degraded", "Progressing"} {
		if view := bar.View(); !strings.Contains(view, want) {
			t.Fatalf("H did not suggest %q:\n%s", want, view)
		}
	}

	narrowed := press(bar, "d")
	if view := narrowed.View(); strings.Contains(view, "Progressing") || !strings.Contains(view, "Degraded") {
		t.Fatalf("typing d did not narrow the suggestions:\n%s", view)
	}

	applied := press(narrowed, "enter")
	if view := applied.View(); !strings.Contains(view, "checkout") || strings.Contains(view, "grafana") || !strings.Contains(view, "health:Degraded") {
		t.Fatalf("enter did not apply health:Degraded:\n%s", view)
	}

	// The sync filter combines with health. The bar starts on "all", so down and tab
	// move on to Synced and then OutOfSync.
	chosen := typeKeys(applied, "S", "down", "tab", "enter")
	if view := chosen.View(); !strings.Contains(view, "sync:OutOfSync") || !strings.Contains(view, "health:Degraded") || !strings.Contains(view, "checkout") {
		t.Fatalf("S did not add the sync filter next to health:\n%s", view)
	}

	// Choosing "all" clears just that field.
	if view := typeKeys(chosen, "H", "a", "enter").View(); strings.Contains(view, "health:") || !strings.Contains(view, "sync:OutOfSync") {
		t.Fatalf("choosing all did not clear only the health filter:\n%s", view)
	}

	if view := press(press(applied, "H"), "esc").View(); !strings.Contains(view, "health:Degraded") {
		t.Fatalf("escape in the bar changed the filter:\n%s", view)
	}
	if view := press(applied, "esc").View(); strings.Contains(view, "health:") || !strings.Contains(view, "grafana") {
		t.Fatalf("escape on the list did not clear the filter:\n%s", view)
	}
	if view := command(press(model, "H"), "zzz").View(); !strings.Contains(view, "zzz") && !strings.Contains(view, "not a known value") {
		t.Fatalf("unknown value was not reported:\n%s", view)
	}
	if view := press(model, "K").View(); strings.Contains(view, "kind") {
		t.Fatalf("K opened a kind filter on the application list:\n%s", view)
	}
}

func TestKindAndHealthBarsFilterTheDependencyTree(t *testing.T) {
	source := &fakeSource{snapshot: storeSnapshot(), tree: explorer.ResourceTree{Application: "checkout", Nodes: []explorer.ResourceNode{
		{Group: "apps", Kind: "Deployment", Namespace: "store", Name: "web", Health: "Healthy"},
		{Kind: "Pod", Namespace: "store", Name: "web-abc", Health: "Degraded", Parents: []explorer.ResourceReference{{Group: "apps", Kind: "Deployment", Namespace: "store", Name: "web"}}},
		{Kind: "Service", Namespace: "store", Name: "frontend", Health: "Healthy"},
	}}}
	model := openCheckout(t, source)

	bar := press(model, "K").View()
	for _, want := range []string{"Deployment", "Pod", "Service"} {
		if !strings.Contains(bar, want) {
			t.Fatalf("K did not suggest the kind %q:\n%s", want, bar)
		}
	}

	view := typeKeys(model, "K", "s", "enter").View()
	if !strings.Contains(view, "kind:Service") || !strings.Contains(view, "frontend") || strings.Contains(view, "web-abc") {
		t.Fatalf("kind:Service did not narrow the tree to Services:\n%s", view)
	}

	view = typeKeys(model, "H", "d", "enter").View()
	if !strings.Contains(view, "health:Degraded") || !strings.Contains(view, "web-abc") || strings.Contains(view, "frontend") {
		t.Fatalf("health:Degraded did not narrow the tree to degraded resources:\n%s", view)
	}
}

func TestFilterCommandsWorkFromTheCommandBarAndShowTheShortcut(t *testing.T) {
	model := resize(New(nil, "test", "argocd", storeSnapshot()), 160, 40)

	if view := typeKeys(model, ":", "h", "e").View(); !strings.Contains(view, "health shift+H") {
		t.Fatalf("typing he did not suggest health with its shortcut:\n%s", view)
	}

	view := command(press(model, ":"), "health degraded").View()
	if !strings.Contains(view, "health:Degraded") || strings.Contains(view, "grafana") || !strings.Contains(view, "checkout") {
		t.Fatalf(":health degraded did not filter the list:\n%s", view)
	}
	if !strings.Contains(view, "shift+H") {
		t.Fatalf("the shortcut was not shown after the command:\n%s", view)
	}

	if view := command(press(model, ":"), "health sick").View(); !strings.Contains(view, "not a known value") {
		t.Fatalf("unknown value was not reported:\n%s", view)
	}
	if view := command(press(model, ":"), "kind pod").View(); !strings.Contains(view, "cannot filter by kind here") {
		t.Fatalf("kind should not be available on the list:\n%s", view)
	}
	if view := command(press(command(press(model, ":"), "health degraded"), ":"), "health").View(); strings.Contains(view, "health:") {
		t.Fatalf(":health without a value did not clear the filter:\n%s", view)
	}
	if view := command(press(model, ":"), "clusters").View(); strings.Contains(view, "shift+") {
		t.Fatalf("no shortcut hint expected for a view command:\n%s", view)
	}
}

func TestTabMovesThroughTheValuesOfTheFilter(t *testing.T) {
	source := &fakeSource{snapshot: storeSnapshot(), tree: explorer.ResourceTree{Application: "checkout", Nodes: []explorer.ResourceNode{
		{Group: "apps", Kind: "Deployment", Namespace: "store", Name: "web", Health: "Healthy"},
		{Kind: "Pod", Namespace: "store", Name: "web-abc", Health: "Degraded", Parents: []explorer.ResourceReference{{Group: "apps", Kind: "Deployment", Namespace: "store", Name: "web"}}},
		{Kind: "Service", Namespace: "store", Name: "frontend", Health: "Healthy"},
	}}}
	model := openCheckout(t, source)

	// The kinds are all, Deployment, Pod, Service: tab walks through them and wraps around.
	for presses, want := range map[int]string{1: "kind:Deployment", 2: "kind:Pod", 3: "kind:Service"} {
		keys := []string{"K"}
		for range presses {
			keys = append(keys, "tab")
		}
		if view := typeKeys(model, append(keys, "enter")...).View(); !strings.Contains(view, want) {
			t.Fatalf("K and %d tabs did not select %s:\n%s", presses, want, view)
		}
	}
	wrapped := typeKeys(model, "K", "tab", "tab", "tab", "tab", "enter").View()
	if strings.Contains(wrapped, "kind:") || !strings.Contains(wrapped, "frontend") {
		t.Fatalf("a fourth tab did not wrap around to all:\n%s", wrapped)
	}
	if view := typeKeys(model, "K", "shift+tab", "enter").View(); !strings.Contains(view, "kind:Service") {
		t.Fatalf("shift+tab did not go back to the last kind:\n%s", view)
	}

	// The bar starts on the current value, so tab moves to the one after it.
	pod := typeKeys(model, "K", "tab", "tab", "enter")
	if view := typeKeys(pod, "K", "tab", "enter").View(); !strings.Contains(view, "kind:Service") {
		t.Fatalf("tab from kind:Pod did not select Service:\n%s", view)
	}

	// Health works the same way: all, Healthy, Progressing, Degraded.
	if view := typeKeys(model, "H", "tab", "tab", "tab", "enter").View(); !strings.Contains(view, "health:Degraded") || strings.Contains(view, "frontend") {
		t.Fatalf("H and tab did not move through the health values:\n%s", view)
	}
}
