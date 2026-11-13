package main

import (
	"os"
	"strings"
	"testing"

	"sigs.k8s.io/yaml"
)

func TestTheCLITokenStoreReadsTheContextsUser(t *testing.T) {
	store := cliTokenStore{path: writeCLIConfig(t, cliConfig)}

	token, refreshToken, err := store.Load()

	if err != nil || token != "prod-token" || refreshToken != "ignored" {
		t.Fatalf("Load() = %q, %q, %v; want the current context's tokens", token, refreshToken, err)
	}
	staging := cliTokenStore{path: store.path, context: "staging"}
	if token, refreshToken, _ := staging.Load(); token != "staging-token" || refreshToken != "" {
		t.Fatalf("staging Load() = %q, %q; want its token and no refresh token", token, refreshToken)
	}
}

func TestTheCLITokenStoreSavesOnlyTheTokensOfTheContextsUser(t *testing.T) {
	config := cliConfig + "prompts-enabled: true\n"
	path := writeCLIConfig(t, config)
	store := cliTokenStore{path: path}

	if err := store.Save("new-token", "new-refresh"); err != nil {
		t.Fatal(err)
	}

	if token, refreshToken, _ := store.Load(); token != "new-token" || refreshToken != "new-refresh" {
		t.Fatalf("after Save, Load() = %q, %q", token, refreshToken)
	}
	data, _ := os.ReadFile(path)
	var saved map[string]any
	if err := yaml.Unmarshal(data, &saved); err != nil {
		t.Fatal(err)
	}
	if saved["prompts-enabled"] != true || saved["current-context"] != "argocd.example.com" {
		t.Fatalf("saved config lost other settings:\n%s", data)
	}
	if !strings.Contains(string(data), "staging-token") || !strings.Contains(string(data), "grpc-web: true") {
		t.Fatalf("saved config changed other users or servers:\n%s", data)
	}
	info, _ := os.Stat(path)
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("saved config mode = %v, want 0600", info.Mode().Perm())
	}
}

func TestTheCLITokenStoreDropsAnEmptyRefreshTokenLikeTheCLI(t *testing.T) {
	path := writeCLIConfig(t, cliConfig)
	store := cliTokenStore{path: path, context: "staging"}

	if err := store.Save("renewed", ""); err != nil {
		t.Fatal(err)
	}

	data, _ := os.ReadFile(path)
	if strings.Count(string(data), "refresh-token") != 1 {
		t.Fatalf("saved config:\n%s\nwant no empty refresh-token for staging-admin", data)
	}
}

func TestACLILoginWithARefreshTokenCanBeRenewed(t *testing.T) {
	path := writeCLIConfig(t, cliConfig)

	sso, err := withArgoCDLogin(options{source: "api", argocdConfig: path})
	if err != nil {
		t.Fatal(err)
	}
	if sso.tokenStore == nil {
		t.Fatal("a CLI login with a refresh token has no token store")
	}
	given, _ := withArgoCDLogin(options{source: "api", argocdConfig: path, token: "mine"})
	if given.tokenStore != nil {
		t.Fatal("a token the user gave has a token store; it must never be renewed")
	}
}
