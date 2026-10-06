package tui

import (
	"errors"
	"strings"
	"testing"

	"github.com/awcodify/argoxd/internal/argocd"
)

func containerSource() *fakeSource {
	return &fakeSource{
		snapshot: storeSnapshot(), tree: checkoutTree(), manifest: "spec:\n  containers:\n    - name: app\n    - name: sidecar\n", logs: "default log",
		logsByContainer: map[string]string{
			"sidecar":            "sidecar log",
			argocd.AllContainers: "app │ from app\nsidecar │ from sidecar",
		},
	}
}

// containerBar presses c, which loads the containers and opens the bar.
func containerBar(t *testing.T, source *fakeSource) Model {
	t.Helper()
	model := run(openPodLogs(t, source), "c")
	if !model.prompt.active || model.prompt.field != "container" {
		t.Fatalf("c did not open the container bar:\n%s", model.View())
	}
	return model
}

func TestCOpensABarWithTheContainersAndAll(t *testing.T) {
	source := containerSource()

	model := containerBar(t, source)

	view := model.View()
	for _, want := range []string{"▾", "app", "sidecar", "all"} {
		if !strings.Contains(view, want) {
			t.Fatalf("the container bar does not contain %q:\n%s", want, view)
		}
	}
	if len(source.loggedContainers) != 1 {
		t.Fatalf("opening the bar reloaded the log: %q", source.loggedContainers)
	}
}

func TestTabMovesToTheNextContainerAndEnterShowsIt(t *testing.T) {
	source := containerSource()
	model := press(containerBar(t, source), "tab")

	model = run(model, "enter")

	view := model.View()
	if !strings.Contains(view, "sidecar log") || !strings.Contains(view, "container: sidecar") || model.prompt.active {
		t.Fatalf("enter did not show the sidecar's log:\n%s", view)
	}
	if got := strings.Join(source.loggedContainers, ","); got != ",sidecar" {
		t.Fatalf("snapshots were read for containers %q, want the default then the sidecar", got)
	}
}

func TestTabReachesAllContainers(t *testing.T) {
	source := containerSource()
	model := typeKeys(containerBar(t, source), "tab", "tab")

	view := run(model, "enter").View()

	if !strings.Contains(view, "app │ from app") || !strings.Contains(view, "container: all containers") {
		t.Fatalf("the second tab did not choose every container:\n%s", view)
	}
	if got := source.loggedContainers[len(source.loggedContainers)-1]; got != argocd.AllContainers {
		t.Fatalf("last snapshot was read for %q, want every container", got)
	}
}

func TestShiftTabWrapsAroundToAllContainers(t *testing.T) {
	source := containerSource()
	model := press(containerBar(t, source), "shift+tab")

	if view := run(model, "enter").View(); !strings.Contains(view, "container: all containers") {
		t.Fatalf("shift+tab did not wrap to every container:\n%s", view)
	}
}

func TestTypingNarrowsTheContainers(t *testing.T) {
	source := containerSource()
	model := typeKeys(containerBar(t, source), "s", "i")

	view := run(model, "enter").View()

	if !strings.Contains(view, "sidecar log") || !strings.Contains(view, "container: sidecar") {
		t.Fatalf("typing si and enter did not choose the sidecar:\n%s", view)
	}
}

func TestTypingMatchesAnyPartOfAContainerName(t *testing.T) {
	source := containerSource()
	model := typeKeys(containerBar(t, source), "c", "a", "r")

	view := run(model, "enter").View()

	if !strings.Contains(view, "sidecar log") || !strings.Contains(view, "container: sidecar") {
		t.Fatalf("typing car did not find the sidecar:\n%s", view)
	}
}

func TestPrefixMatchesComeBeforeOtherMatches(t *testing.T) {
	containers := argocd.Containers{Names: []string{"web-proxy", "proxy", "app"}}

	got := containerSuggestions("pro", containers)

	if strings.Join(got, ",") != "proxy,web-proxy" {
		t.Fatalf("suggestions = %q, want the prefix match first", got)
	}
	if got := containerSuggestions("", containers); strings.Join(got, ",") != "web-proxy,proxy,app,all" {
		t.Fatalf("suggestions for no input = %q, want every container then all", got)
	}
}

func TestEscClosesTheContainerBarWithoutChangingTheLog(t *testing.T) {
	source := containerSource()
	model := press(press(containerBar(t, source), "tab"), "esc")

	if model.prompt.active || strings.Contains(model.View(), "container:") || len(source.loggedContainers) != 1 {
		t.Fatalf("esc changed the log (%q):\n%s", source.loggedContainers, model.View())
	}
}

func TestChoosingTheCurrentContainerDoesNotReloadTheLog(t *testing.T) {
	source := containerSource()

	model := run(containerBar(t, source), "enter")

	if view := model.View(); !strings.Contains(view, "container: app") || len(source.loggedContainers) != 1 {
		t.Fatalf("choosing the current container reloaded the log (%q):\n%s", source.loggedContainers, view)
	}
}

func TestAnUnknownContainerIsRefused(t *testing.T) {
	source := containerSource()
	model := typeKeys(containerBar(t, source), "z", "z")

	view := run(model, "enter").View()

	if !strings.Contains(view, `container "zz" not found`) || len(source.loggedContainers) != 1 {
		t.Fatalf("an unknown container was not refused (%q):\n%s", source.loggedContainers, view)
	}
}

func TestCOnASingleContainerExplainsItself(t *testing.T) {
	source := containerSource()
	source.manifest = "spec:\n  containers:\n    - name: app\n"

	model := run(openPodLogs(t, source), "c")

	if view := model.View(); !strings.Contains(view, "single container") || model.prompt.active || len(source.loggedContainers) != 1 {
		t.Fatalf("c on a single-container Pod did not explain itself:\n%s", view)
	}
}

func TestChoosingWhileFollowingRestartsTheStreamOnThatContainer(t *testing.T) {
	source := containerSource()
	source.stream = lines("first")
	model, _ := follow(t, openPodLogs(t, source), 1)
	oldStream := source.streamCtx

	updated, command := model.Update(key("c"))
	model, _ = stepT(t, updated.(Model), command) // the containers arrive and the bar opens
	updated, command = press(model, "tab").Update(key("enter"))
	model, _ = stepT(t, updated.(Model), command) // the new stream starts

	if oldStream.Err() == nil {
		t.Fatal("the stream of the previous container was not cancelled")
	}
	if got := source.streamedContainers; got[len(got)-1] != "sidecar" {
		t.Fatalf("streamed containers = %q, want the restart on the sidecar", got)
	}
	if view := model.View(); !strings.Contains(view, "● following") || !strings.Contains(view, "container: sidecar") {
		t.Fatalf("the restarted stream is not shown as following the sidecar:\n%s", view)
	}
}

func TestEveryContainerOfAWorkloadIsPrefixedWithPodAndContainer(t *testing.T) {
	source := containerSource()
	source.stream = lines()
	model := followFrom(t, resize(press(openCheckout(t, source), "j"), 120, 14), "l", 0)

	updated, command := model.Update(key("c"))
	model, _ = stepT(t, updated.(Model), command)
	updated, command = typeKeys(model, "tab", "tab").Update(key("enter")) // every container
	model, command = stepT(t, updated.(Model), command)
	source.stream <- argocd.LogEntry{Pod: "web-abc", Container: "app", Line: "hello"}
	model, _ = stepT(t, model, command)

	view := model.View()
	if !strings.Contains(view, "web-abc/app │ hello") || !strings.Contains(view, "container: all containers") {
		t.Fatalf("a line of every container was not prefixed with pod and container:\n%s", view)
	}
}

func TestAPodLogOfEveryContainerIsPrefixedWithTheContainerOnly(t *testing.T) {
	source := containerSource()
	source.stream = lines()
	model, _ := follow(t, openPodLogs(t, source), 0)

	updated, command := model.Update(key("c"))
	model, _ = stepT(t, updated.(Model), command)
	updated, command = typeKeys(model, "tab", "tab").Update(key("enter"))
	model, command = stepT(t, updated.(Model), command)
	source.stream <- argocd.LogEntry{Pod: "web-abc", Container: "sidecar", Line: "ping"}
	model, _ = stepT(t, model, command)

	view := model.View()
	if !strings.Contains(view, "sidecar │ ping") || strings.Contains(view, "web-abc/sidecar") {
		t.Fatalf("a Pod's log was prefixed with more than the container:\n%s", view)
	}
}

func TestAContainerListForAnotherLogIsIgnored(t *testing.T) {
	source := containerSource()
	model := openPodLogs(t, source)

	updated, command := model.Update(loadedContainers{subject: "Pod/other", containers: argocd.Containers{Names: []string{"a", "b"}, Default: "a"}})

	if command != nil || updated.(Model).prompt.active || strings.Contains(updated.View(), "container:") {
		t.Fatalf("a stale container list changed the view:\n%s", updated.View())
	}
}

func TestAFailedContainerLookupIsShown(t *testing.T) {
	source := containerSource()
	source.manifestErr = errors.New("forbidden")
	model := openPodLogs(t, source)

	model = run(model, "c")

	if view := model.View(); !strings.Contains(view, "forbidden") || model.prompt.active || len(source.loggedContainers) != 1 {
		t.Fatalf("the failed lookup was not shown:\n%s", view)
	}
}

func TestCOutsideLogsDoesNothing(t *testing.T) {
	source := containerSource()
	model := run(press(press(openCheckout(t, source), "j"), "j"), "y")

	if _, command := model.Update(key("c")); command != nil {
		t.Fatal("c did something in a YAML view")
	}
}

func TestLogsHeaderListsContainer(t *testing.T) {
	source := containerSource()
	model := resize(openCheckout(t, source), 160, 40)
	model = press(press(model, "j"), "j")

	if view := run(model, "l").View(); !strings.Contains(view, "c  Container") {
		t.Fatalf("the logs header does not list c:\n%s", view)
	}
	if view := run(model, "y").View(); strings.Contains(view, "c  Container") {
		t.Fatalf("the YAML header lists c:\n%s", view)
	}
}
