package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// isolateConfig points argoxd at a config file in a temp dir and clears the
// environment variables that feed option defaults.
func isolateConfig(t *testing.T, contents string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	if contents != "" {
		if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("ARGOXD_CONFIG", path)
	t.Setenv("ARGOCD_SERVER", "")
	t.Setenv("ARGOCD_AUTH_TOKEN", "")
}

func TestBuildSourceRequiresAPIServer(t *testing.T) {
	_, err := buildSource(options{source: "api"})
	if err == nil {
		t.Fatal("buildSource() error = nil, want missing server error")
	}
}

func TestBuildSourceRejectsUnknownMode(t *testing.T) {
	_, err := buildSource(options{source: "unknown"})
	if err == nil {
		t.Fatal("buildSource() error = nil, want invalid mode error")
	}
}

func TestRefreshFlagDefaultsToFiveSeconds(t *testing.T) {
	isolateConfig(t, "")
	options, err := parseOptions(nil)
	if err != nil {
		t.Fatal(err)
	}

	if options.refresh != 5*time.Second {
		t.Fatalf("refresh = %v, want 5s", options.refresh)
	}
}

func TestConfigFileSetsDefaults(t *testing.T) {
	isolateConfig(t, "context: production\nnamespace: gitops\nrefresh: 30s\n")

	options, err := parseOptions(nil)
	if err != nil {
		t.Fatal(err)
	}

	if options.context != "production" || options.namespace != "gitops" || options.refresh != 30*time.Second {
		t.Fatalf("options = %+v, want values from config file", options)
	}
}

func TestConfigFileAllowsRefreshZero(t *testing.T) {
	isolateConfig(t, "refresh: 0s\n")

	options, err := parseOptions(nil)
	if err != nil {
		t.Fatal(err)
	}

	if options.refresh != 0 {
		t.Fatalf("refresh = %v, want 0", options.refresh)
	}

}

func TestFlagsOverrideEnvironmentOverridesConfigFile(t *testing.T) {
	isolateConfig(t, "server: https://file.example.com\nnamespace: gitops\n")
	t.Setenv("ARGOCD_SERVER", "https://env.example.com")

	options, err := parseOptions(nil)
	if err != nil {
		t.Fatal(err)
	}
	if options.server != "https://env.example.com" {
		t.Fatalf("server = %q, want environment value over config file", options.server)
	}

	options, err = parseOptions([]string{"--server", "https://flag.example.com", "--namespace", "flagged"})
	if err != nil {
		t.Fatal(err)
	}
	if options.server != "https://flag.example.com" || options.namespace != "flagged" {
		t.Fatalf("options = %+v, want flag values", options)
	}
}

func TestConfigFileRejectsUnknownKeysAndBadDurations(t *testing.T) {
	for _, contents := range []string{"unknown: 1\n", "refresh: soon\n"} {
		isolateConfig(t, contents)
		if _, err := parseOptions(nil); err == nil {
			t.Fatalf("parseOptions() error = nil for config %q", contents)
		}
	}
}

func TestDemoFlagUsesSampleDataWithoutConnection(t *testing.T) {
	isolateConfig(t, "")
	options, err := parseOptions([]string{"--demo"})
	if err != nil {
		t.Fatal(err)
	}

	source, err := buildSource(options)
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := source.Load(context.Background())
	if err != nil || len(snapshot.Applications) == 0 {
		t.Fatalf("Load() = %d applications, %v; want sample applications", len(snapshot.Applications), err)
	}
}

func TestShortFlagsMatchTheirLongForms(t *testing.T) {
	isolateConfig(t, "")

	options, err := parseOptions([]string{"-n", "gitops", "-c", "production", "-s", "https://argocd.example.com", "-r", "10s", "-k"})
	if err != nil {
		t.Fatal(err)
	}

	if options.namespace != "gitops" || options.context != "production" || options.server != "https://argocd.example.com" ||
		options.refresh != 10*time.Second || !options.insecure {
		t.Fatalf("options = %+v", options)
	}
}

func TestHelpShowsShortFlagsOnTheRowOfTheirLongForm(t *testing.T) {
	var buffer bytes.Buffer
	flags := newFlagSet(&options{namespace: "argocd"})
	flags.SetOutput(&buffer)

	flags.Usage()

	help := buffer.String()
	for _, row := range []string{"-n, --namespace string", "-c, --context string", "-s, --server string", "-r, --refresh duration", "-k, --insecure"} {
		if !strings.Contains(help, row) {
			t.Errorf("help does not contain %q:\n%s", row, help)
		}
	}
	if strings.Contains(help, "Shorthand for") || strings.Contains(help, "  -n string") {
		t.Errorf("shorthands are listed on their own rows:\n%s", help)
	}
}

func TestUpdateChecksCanBeSwitchedOff(t *testing.T) {
	isolateConfig(t, "")
	options, err := parseOptions(nil)
	if err != nil || !options.checkUpdates {
		t.Fatalf("checkUpdates = %v, %v; want on by default", options.checkUpdates, err)
	}

	isolateConfig(t, "checkUpdates: false\n")
	if options, err = parseOptions(nil); err != nil || options.checkUpdates {
		t.Fatalf("checkUpdates = %v, %v; want off from the config file", options.checkUpdates, err)
	}

	isolateConfig(t, "")
	t.Setenv("ARGOXD_NO_UPDATE_CHECK", "1")
	if options, err = parseOptions(nil); err != nil || options.checkUpdates {
		t.Fatalf("checkUpdates = %v, %v; want off from ARGOXD_NO_UPDATE_CHECK", options.checkUpdates, err)
	}
}

func TestConnectionFlagsAndConfigKeysForTLSAndTimeout(t *testing.T) {
	isolateConfig(t, "caCert: /etc/ssl/corp-ca.pem\nclientCert: /etc/argoxd/client.crt\nclientKey: /etc/argoxd/client.key\nrequestTimeout: 45s\n")

	options, err := parseOptions(nil)
	if err != nil {
		t.Fatal(err)
	}
	if options.caCert != "/etc/ssl/corp-ca.pem" || options.clientCert != "/etc/argoxd/client.crt" || options.clientKey != "/etc/argoxd/client.key" || options.requestTimeout != 45*time.Second {
		t.Fatalf("options = %+v, want the config file's values", options)
	}

	options, err = parseOptions([]string{"--ca-cert", "ca.pem", "--client-cert", "c.crt", "--client-key", "c.key", "--request-timeout", "2m"})
	if err != nil {
		t.Fatal(err)
	}
	if options.caCert != "ca.pem" || options.clientCert != "c.crt" || options.clientKey != "c.key" || options.requestTimeout != 2*time.Minute {
		t.Fatalf("options = %+v, want the flags to win over the config file", options)
	}
}

func TestRequestTimeoutDefaultsToFifteenSecondsAndRejectsNonsense(t *testing.T) {
	isolateConfig(t, "")
	options, err := parseOptions(nil)
	if err != nil || options.requestTimeout != 15*time.Second {
		t.Fatalf("requestTimeout = %v, %v; want 15s", options.requestTimeout, err)
	}

	for _, contents := range []string{"requestTimeout: soon\n", "requestTimeout: 0s\n", "requestTimeout: -5s\n"} {
		isolateConfig(t, contents)
		if _, err := parseOptions(nil); err == nil {
			t.Fatalf("parseOptions() accepted config %q", contents)
		}
	}
}

func TestBuildSourcePassesTLSOptionsToTheAPISource(t *testing.T) {
	if _, err := buildSource(options{source: "api", server: "https://argocd.example.com", caCert: "/no/such/ca.pem"}); err == nil || !strings.Contains(err.Error(), "/no/such/ca.pem") {
		t.Fatalf("buildSource() error = %v, want the unreadable CA file named", err)
	}
	if _, err := buildSource(options{source: "api", server: "https://argocd.example.com", clientCert: "c.crt"}); err == nil || !strings.Contains(err.Error(), "client key") {
		t.Fatalf("buildSource() error = %v, want a missing client key reported", err)
	}
	if _, err := buildSource(options{source: "kubeconfig", caCert: "/no/such/ca.pem"}); err != nil {
		t.Fatalf("a CA file set for another source must not fail the kubeconfig source: %v", err)
	}
}

func TestHelpListsTheNewConnectionFlags(t *testing.T) {
	var buffer bytes.Buffer
	flags := newFlagSet(&options{namespace: "argocd"})
	flags.SetOutput(&buffer)

	flags.Usage()

	for _, flag := range []string{"--ca-cert", "--client-cert", "--client-key", "--request-timeout"} {
		if !strings.Contains(buffer.String(), flag) {
			t.Errorf("help does not list %s:\n%s", flag, buffer.String())
		}
	}
}

func TestSortFromTheConfigFile(t *testing.T) {
	isolateConfig(t, "sort: health\nsortReverse: true\n")
	options, err := parseOptions(nil)
	if err != nil || options.sort != "health" || !options.sortReverse {
		t.Fatalf("options = %+v, %v; want sort health, reversed", options, err)
	}

	isolateConfig(t, "sort: revision\n")
	if _, err := parseOptions(nil); err == nil || !strings.Contains(err.Error(), "name, health or sync") {
		t.Fatalf("a bad sort field: error = %v, want a message listing the fields", err)
	}
}

func TestMouseIsOffUnlessAskedFor(t *testing.T) {
	isolateConfig(t, "")
	if options, err := parseOptions(nil); err != nil || options.mouse {
		t.Fatalf("mouse = %v, %v; want off by default so the terminal can still select text", options.mouse, err)
	}
	if options, _ := parseOptions([]string{"--mouse"}); !options.mouse {
		t.Fatal("--mouse did not turn it on")
	}
	isolateConfig(t, "mouse: true\n")
	if options, err := parseOptions(nil); err != nil || !options.mouse {
		t.Fatalf("mouse = %v, %v; want it on from the config file", options.mouse, err)
	}
}

func TestResourceViewFromTheConfigFileAndFlag(t *testing.T) {
	isolateConfig(t, "")
	if options, err := parseOptions(nil); err != nil || options.resourceView != "cards" {
		t.Fatalf("resourceView = %q, %v; want cards by default", options.resourceView, err)
	}
	isolateConfig(t, "resourceView: table\n")
	if options, err := parseOptions(nil); err != nil || options.resourceView != "table" {
		t.Fatalf("resourceView = %q, %v; want table from the config file", options.resourceView, err)
	}
	if options, _ := parseOptions([]string{"--resource-view", "cards"}); options.resourceView != "cards" {
		t.Fatalf("resourceView = %q, want the flag to win", options.resourceView)
	}
	isolateConfig(t, "resourceView: grid\n")
	if _, err := parseOptions(nil); err == nil || !strings.Contains(err.Error(), "cards or table") {
		t.Fatalf("a bad resourceView: error = %v, want one naming cards or table", err)
	}
}

func TestStartViewFromTheConfigFileAndFlag(t *testing.T) {
	isolateConfig(t, "startView: appset\n")
	if options, err := parseOptions(nil); err != nil || options.startView != "appset" {
		t.Fatalf("startView = %q, %v", options.startView, err)
	}
	if options, _ := parseOptions([]string{"--start-view", "app store"}); options.startView != "app store" {
		t.Fatalf("startView = %q, want the flag to win", options.startView)
	}
	for _, bad := range []string{"startView: dashboard\n", "startView: ctx\n", "startView: sort name\n"} {
		isolateConfig(t, bad)
		if _, err := parseOptions(nil); err == nil || !strings.Contains(err.Error(), "start view") {
			t.Fatalf("%q: error = %v, want one about the start view", bad, err)
		}
	}
}

func TestK9sCommandFromTheConfigFile(t *testing.T) {
	isolateConfig(t, "k9sCommand: /opt/bin/k9s\n")
	if options, err := parseOptions(nil); err != nil || options.k9sCommand != "/opt/bin/k9s" {
		t.Fatalf("k9sCommand = %q, %v", options.k9sCommand, err)
	}
}

func TestDiffViewerFromTheConfigFile(t *testing.T) {
	isolateConfig(t, "diffViewer: meld {left} {right}\n")
	if options, err := parseOptions(nil); err != nil || options.diffViewer != "meld {left} {right}" {
		t.Fatalf("diffViewer = %q, %v", options.diffViewer, err)
	}
}

func TestThemeFromTheConfigFileAndFlag(t *testing.T) {
	isolateConfig(t, "theme: nord\n")
	if options, err := parseOptions(nil); err != nil || options.theme != "nord" {
		t.Fatalf("theme = %q, %v", options.theme, err)
	}
	if options, _ := parseOptions([]string{"--theme", "light"}); options.theme != "light" {
		t.Fatalf("theme = %q, want the flag to win", options.theme)
	}
	isolateConfig(t, "theme: neon\n")
	if _, err := parseOptions(nil); err == nil || !strings.Contains(err.Error(), "nord") {
		t.Fatalf("a bad theme: error = %v, want the available themes listed", err)
	}
}
