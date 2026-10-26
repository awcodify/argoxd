package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const cliConfig = `contexts:
- name: argocd.example.com
  server: argocd.example.com
  user: argocd.example.com
- name: staging
  server: staging.example.com:8443
  user: staging-admin
- name: local
  server: localhost:8080
  user: local
current-context: argocd.example.com
servers:
- server: argocd.example.com
  grpc-web: true
- server: staging.example.com:8443
  insecure: true
- server: localhost:8080
  plain-text: true
users:
- name: argocd.example.com
  auth-token: prod-token
  refresh-token: ignored
- name: staging-admin
  auth-token: staging-token
- name: local
  auth-token: local-token
`

func writeCLIConfig(t *testing.T, contents string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config")
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoadArgoCDLoginReadsTheCurrentContext(t *testing.T) {
	login, err := loadArgoCDLogin(writeCLIConfig(t, cliConfig), "")
	if err != nil {
		t.Fatal(err)
	}

	if login.Server != "https://argocd.example.com" || login.Token != "prod-token" {
		t.Fatalf("login = %+v, want the current context's server and token", login)
	}
}

func TestLoadArgoCDLoginPicksANamedContextAndHonoursTheServerScheme(t *testing.T) {
	path := writeCLIConfig(t, cliConfig)

	staging, err := loadArgoCDLogin(path, "staging")
	if err != nil || staging.Server != "https://staging.example.com:8443" || staging.Token != "staging-token" || !staging.Insecure {
		t.Fatalf("staging = %+v, %v", staging, err)
	}
	local, err := loadArgoCDLogin(path, "local")
	if err != nil || local.Server != "http://localhost:8080" || local.Token != "local-token" {
		t.Fatalf("local = %+v, %v; a plain-text server must use http", local, err)
	}
}

func TestLoadArgoCDLoginExplainsWhatIsMissing(t *testing.T) {
	path := writeCLIConfig(t, cliConfig)
	if _, err := loadArgoCDLogin(path, "nope"); err == nil || !strings.Contains(err.Error(), `"nope"`) || !strings.Contains(err.Error(), "staging") {
		t.Fatalf("an unknown context: error = %v, want it named and the real ones listed", err)
	}
	if _, err := loadArgoCDLogin(filepath.Join(t.TempDir(), "missing"), ""); !os.IsNotExist(unwrapPathError(err)) {
		t.Fatalf("a missing file: error = %v, want a not-exist error", err)
	}
	if _, err := loadArgoCDLogin(writeCLIConfig(t, "contexts: [unclosed"), ""); err == nil {
		t.Fatal("a broken file was accepted")
	}
	loggedOut := writeCLIConfig(t, "contexts:\n- name: a\n  server: a.example.com\n  user: a\ncurrent-context: a\nusers:\n- name: a\n")
	if login, err := loadArgoCDLogin(loggedOut, ""); err != nil || login.Token != "" || login.Server != "https://a.example.com" {
		t.Fatalf("a logged-out context: %+v, %v; want the server and an empty token", login, err)
	}
}

func TestExplicitSettingsBeatTheCLILogin(t *testing.T) {
	path := writeCLIConfig(t, cliConfig)

	filled, err := withArgoCDLogin(options{source: "api", argocdConfig: path})
	if err != nil || filled.server != "https://argocd.example.com" || filled.token != "prod-token" {
		t.Fatalf("filled = %+v, %v; want the CLI's server and token", filled, err)
	}
	mixed, err := withArgoCDLogin(options{source: "api", argocdConfig: path, server: "https://mine.example.com"})
	if err != nil || mixed.server != "https://mine.example.com" || mixed.token != "prod-token" {
		t.Fatalf("mixed = %+v, %v; want the given server and the CLI token", mixed, err)
	}
	given, err := withArgoCDLogin(options{source: "api", argocdConfig: path, server: "https://mine.example.com", token: "mine"})
	if err != nil || given.server != "https://mine.example.com" || given.token != "mine" {
		t.Fatalf("given = %+v, %v; nothing should be overridden", given, err)
	}
}

func TestTheCLILoginIsOnlyForTheAPISourceAndOnlyWhenNeeded(t *testing.T) {
	path := writeCLIConfig(t, cliConfig)

	if got, _ := withArgoCDLogin(options{source: "kubeconfig", argocdConfig: path}); got.server != "" || got.token != "" {
		t.Fatalf("the kubeconfig source read the CLI login: %+v", got)
	}
	if got, err := withArgoCDLogin(options{source: "api", argocdConfig: filepath.Join(t.TempDir(), "absent")}); err != nil || got.server != "" {
		t.Fatalf("a missing CLI config must be skipped quietly: %+v, %v", got, err)
	}
	if _, err := withArgoCDLogin(options{source: "api", argocdConfig: path, argocdContext: "nope"}); err == nil {
		t.Fatal("a context that was asked for by name but does not exist must be an error")
	}
}

func TestCLILoginFlagsAndConfigKeys(t *testing.T) {
	isolateConfig(t, "argocdContext: staging\nargocdConfig: /home/me/argocd-config\n")
	options, err := parseOptions(nil)
	if err != nil || options.argocdContext != "staging" || options.argocdConfig != "/home/me/argocd-config" {
		t.Fatalf("options = %+v, %v", options, err)
	}
	if options, _ = parseOptions([]string{"--argocd-context", "local", "--argocd-config", "/x"}); options.argocdContext != "local" || options.argocdConfig != "/x" {
		t.Fatalf("options = %+v, want the flags to win", options)
	}
}
