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
