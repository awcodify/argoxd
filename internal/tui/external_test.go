package tui

import (
	"os/exec"
	"strings"
	"testing"

	"github.com/awcodify/argoxd/internal/explorer"
	tea "github.com/charmbracelet/bubbletea"
)

// recordedLaunch captures what would have been started, instead of starting it.
type recordedLaunch struct {
	args []string
}

func (r *recordedLaunch) launch(cmd *exec.Cmd, done tea.ExecCallback) tea.Cmd {
	r.args = cmd.Args
	return func() tea.Msg { return done(nil) }
}

func withK9s(model Model, path string) (Model, *recordedLaunch) {
	recorded := &recordedLaunch{}
	model = model.WithExternalTools(ExternalTools{K9s: path, KubeContext: "production"})
	model.launch = recorded.launch
	return model, recorded
}

func TestK9sOpensOnTheSelectedCardsNamespaceAndKind(t *testing.T) {
	model, _ := openDeep(t)
	model = press(model, "j") // Deployment web in store
	model, recorded := withK9s(model, "/usr/bin/true")

	model = command(model, ":k9s")

	want := "/usr/bin/true --context production -n store -c deployments"
	if got := strings.Join(recorded.args, " "); got != want {
		t.Fatalf("started %q, want %q", got, want)
	}
}

func TestK9sForAPodUsesPods(t *testing.T) {
	model, _ := openDeep(t)
	model = typeKeys(model, "j", "j", "j")
	model, recorded := withK9s(model, "/usr/bin/true")

	command(model, ":k9s")

	if got := strings.Join(recorded.args, " "); !strings.HasSuffix(got, "-n store -c pods") {
		t.Fatalf("started %q, want it on the pods of store", got)
	}
}

func TestK9sWithoutASelectedResourceJustOpensK9s(t *testing.T) {
	source := &fakeSource{snapshot: storeSnapshot()}
	model := resize(settle(New(source, "test", "argocd", explorer.Snapshot{}), source.loadCommand()), 140, 30)
	model, recorded := withK9s(model, "/usr/bin/true")

	command(model, ":k9s")

	if got := strings.Join(recorded.args, " "); got != "/usr/bin/true --context production" {
		t.Fatalf("started %q, want plain k9s on the context", got)
	}
}

func TestK9sNotInstalledIsReportedAndNothingStarts(t *testing.T) {
	model, _ := openDeep(t)
	model, recorded := withK9s(model, "definitely-not-a-real-k9s-binary")

	model = command(model, ":k9s")

	if recorded.args != nil || model.err == nil || !strings.Contains(model.err.Error(), "k9s") || !strings.Contains(model.err.Error(), "PATH") {
		t.Fatalf("started %v, err = %v; want a message that k9s is not in PATH", recorded.args, model.err)
	}
}

func TestK9sIsSuggestedAndComesBackToArgoxd(t *testing.T) {
	model, recorded := withK9s(resize(settle(New(&fakeSource{snapshot: storeSnapshot()}, "test", "argocd", explorer.Snapshot{}), nil), 140, 30), "/usr/bin/true")

	if view := typeKeys(model, ":", "k", "9").View(); !strings.Contains(view, "k9s") {
		t.Fatalf(":k9 did not suggest k9s:\n%s", view)
	}
	updated, next := model.Update(key("enter"))
	_ = updated
	_ = next
	if recorded.args != nil {
		t.Fatal("an empty command bar started k9s")
	}
}

func TestReadOnlyModeBlocksK9sBecauseK9sCanChangeThings(t *testing.T) {
	model, _ := openDeep(t)
	model, recorded := withK9s(model.WithReadOnly(true), "/usr/bin/true")

	model = command(model, ":k9s")

	if recorded.args != nil || !strings.Contains(model.View(), "Read-only mode") {
		t.Fatalf("read-only mode let k9s start (%v):\n%s", recorded.args, model.View())
	}
}
