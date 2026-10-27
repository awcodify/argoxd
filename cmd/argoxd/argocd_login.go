package main

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"sigs.k8s.io/yaml"
)

// argoCDLogin is the server and token the argocd CLI saved with "argocd login".
type argoCDLogin struct {
	// Server is a full URL: https, or http for a plain-text server.
	Server string
	// Token is empty when the context is logged out.
	Token string
	// Insecure records that the CLI skips certificate checks for this server.
	// argoxd does not follow it on its own; -k is still needed.
	Insecure bool
}

// argoCDConfig is the part of ~/.config/argocd/config that argoxd reads.
type argoCDConfig struct {
	CurrentContext string `json:"current-context"`
	Contexts       []struct {
		Name   string `json:"name"`
		Server string `json:"server"`
		User   string `json:"user"`
	} `json:"contexts"`
	Servers []struct {
		Server    string `json:"server"`
		Insecure  bool   `json:"insecure"`
		PlainText bool   `json:"plain-text"`
	} `json:"servers"`
	Users []struct {
		Name      string `json:"name"`
		AuthToken string `json:"auth-token"`
	} `json:"users"`
}

// argoCDConfigPath is where the argocd CLI keeps its login.
func argoCDConfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".config", "argocd", "config")
}

// loadArgoCDLogin reads the login of contextName, or of the CLI's current
// context when it is empty.
func loadArgoCDLogin(path, contextName string) (argoCDLogin, error) {
	var login argoCDLogin
	data, err := os.ReadFile(path)
	if err != nil {
		return login, err
	}
	var config argoCDConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return login, fmt.Errorf("%s: %w", path, err)
	}
	if contextName == "" {
		contextName = config.CurrentContext
	}
	var names []string
	for _, context := range config.Contexts {
		names = append(names, context.Name)
		if context.Name != contextName {
			continue
		}
		login.Server = "https://" + context.Server
		for _, server := range config.Servers {
			if server.Server == context.Server {
				login.Insecure = server.Insecure
				if server.PlainText {
					login.Server = "http://" + context.Server
				}
			}
		}
		for _, user := range config.Users {
			if user.Name == context.User {
				login.Token = user.AuthToken
			}
		}
		return login, nil
	}
	if contextName == "" {
		return login, fmt.Errorf("%s has no current context; run argocd login", path)
	}
	slices.Sort(names)
	return login, fmt.Errorf("argocd context %q not found in %s; it has: %s", contextName, path, strings.Join(names, ", "))
}

// withArgoCDLogin fills in the server and token that were not given from the
// argocd CLI's saved login, so `argocd login` once is enough for --source api.
// Anything given by a flag, the environment or the config file wins. A missing
// CLI config is not an error, unless a context was asked for by name.
func withArgoCDLogin(options options) (options, error) {
	if options.source != "api" || (options.server != "" && options.token != "") {
		return options, nil
	}
	path := options.argocdConfig
	if path == "" {
		path = argoCDConfigPath()
	}
	login, err := loadArgoCDLogin(path, options.argocdContext)
	if errors.Is(err, fs.ErrNotExist) {
		if options.argocdContext == "" {
			return options, nil
		}
		// The user asked for a context by name, so say why there is no file
		// instead of passing on a bare "open …: no such file".
		return options, fmt.Errorf("argocd config %s not found; run: argocd login", path)
	}
	if err != nil {
		return options, err
	}
	if options.server == "" {
		options.server = login.Server
	}
	if options.token == "" && login.Token != "" {
		options.token = login.Token
		options.tokenFromCLI = true
	}
	return options, nil
}
