package tui

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/awcodify/argoxd/internal/explorer"
	tea "github.com/charmbracelet/bubbletea"
)

func TestSplitDiffRebuildsTheLiveAndDesiredSides(t *testing.T) {
	diff := "  apiVersion: v1\n- replicas: 2\n+ replicas: 3\n  kind: Deployment\n+ paused: false"

	live, desired := splitDiff(diff)

	if live != "apiVersion: v1\nreplicas: 2\nkind: Deployment\n" {
		t.Fatalf("live = %q", live)
	}
	if desired != "apiVersion: v1\nreplicas: 3\nkind: Deployment\npaused: false\n" {
		t.Fatalf("desired = %q", desired)
	}
}

func TestSplitDiffKeepsSectionHeadersOnBothSides(t *testing.T) {
	live, desired := splitDiff("## Deployment/web\n- a: 1\n+ a: 2\n\n## Service/web\n  b: 1")

	for name, side := range map[string]string{"live": live, "desired": desired} {
		if !strings.Contains(side, "## Deployment/web\n") || !strings.Contains(side, "## Service/web\n") {
			t.Errorf("%s lost a section header: %q", name, side)
		}
	}
}

// viewerLaunch records the command and what the two files held when it started.
type viewerLaunch struct {
	args        []string
	left, right string
}

func (v *viewerLaunch) launch(cmd *exec.Cmd, done tea.ExecCallback) tea.Cmd {
	v.args = cmd.Args
	if len(cmd.Args) >= 3 {
		left, _ := os.ReadFile(cmd.Args[len(cmd.Args)-2])
		right, _ := os.ReadFile(cmd.Args[len(cmd.Args)-1])
		v.left, v.right = string(left), string(right)
	}
	return func() tea.Msg { return done(nil) }
}

func openDiff(t *testing.T) Model {
	t.Helper()
	source := &fakeSource{snapshot: storeSnapshot(), tree: checkoutTree(), diff: "  kind: Deployment\n- replicas: 2\n+ replicas: 3"}
	model := press(resize(openCheckout(t, source), 140, 30), "j")
	return run(model, "d")
}

func TestXOpensTheDiffInTheConfiguredViewer(t *testing.T) {
	model := openDiff(t).WithExternalTools(ExternalTools{DiffViewer: "true --diff {left} {right}"})
	recorded := &viewerLaunch{}
	model.launch = recorded.launch

	model = run(model, "x")

	if len(recorded.args) != 4 || filepath.Base(recorded.args[0]) != "true" || recorded.args[1] != "--diff" {
		t.Fatalf("started %v, want true --diff <left> <right>", recorded.args)
	}
	if recorded.left != "kind: Deployment\nreplicas: 2\n" || recorded.right != "kind: Deployment\nreplicas: 3\n" {
		t.Fatalf("the files held %q and %q", recorded.left, recorded.right)
	}
	if _, err := os.Stat(recorded.args[2]); !os.IsNotExist(err) {
		t.Fatalf("the temporary left file was not removed: %v", err)
	}
}

func TestXWithoutAViewerSaysHowToSetOne(t *testing.T) {
	model := openDiff(t)

	model = press(model, "x")

	if view := model.View(); !strings.Contains(view, "diffViewer") {
		t.Fatalf("no hint about diffViewer:\n%s", view)
	}
}

func TestXOnlyAppliesToDiffs(t *testing.T) {
	source := &fakeSource{snapshot: storeSnapshot(), tree: checkoutTree(), manifest: "kind: Deployment\n"}
	model := run(press(resize(openCheckout(t, source), 140, 30), "j"), "Y").WithExternalTools(ExternalTools{DiffViewer: "true {left} {right}"})
	recorded := &viewerLaunch{}
	model.launch = recorded.launch

	press(model, "x")

	if recorded.args != nil {
		t.Fatalf("a YAML view started %v", recorded.args)
	}
}

func TestAViewerThatIsNotInstalledIsReported(t *testing.T) {
	model := openDiff(t).WithExternalTools(ExternalTools{DiffViewer: "no-such-diff-tool-xyz {left} {right}"})
	recorded := &viewerLaunch{}
	model.launch = recorded.launch

	model = press(model, "x")

	if recorded.args != nil || model.err == nil || !strings.Contains(model.err.Error(), "no-such-diff-tool-xyz") {
		t.Fatalf("started %v, err = %v; want the missing program named", recorded.args, model.err)
	}
}

func TestADiffViewerTemplateNeedsBothFiles(t *testing.T) {
	model := openDiff(t).WithExternalTools(ExternalTools{DiffViewer: "meld {left}"})
	recorded := &viewerLaunch{}
	model.launch = recorded.launch

	model = press(model, "x")

	if recorded.args != nil || model.err == nil || !strings.Contains(model.err.Error(), "{right}") {
		t.Fatalf("started %v, err = %v; want a message that {left} and {right} are both needed", recorded.args, model.err)
	}
}

var _ = explorer.Snapshot{}
