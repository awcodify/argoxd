package main

import (
	"fmt"
	"os"
	"path/filepath"

	"sigs.k8s.io/yaml"
)

// cliTokenStore is the login of one context in the argocd CLI's config file.
// argoxd saves a renewed SSO token there, as "argocd" does, so the next run of
// either one starts with it.
type cliTokenStore struct {
	path string
	// context is the CLI context; empty means its current one.
	context string
}

func (c cliTokenStore) Load() (string, string, error) {
	config, err := c.read()
	if err != nil {
		return "", "", err
	}
	user := c.user(config)
	if user == nil {
		return "", "", fmt.Errorf("argocd context %q has no user in %s", c.context, c.path)
	}
	token, _ := user["auth-token"].(string)
	refreshToken, _ := user["refresh-token"].(string)
	return token, refreshToken, nil
}

// Save replaces the tokens of the context's user and leaves everything else in
// the file as it was.
func (c cliTokenStore) Save(token, refreshToken string) error {
	config, err := c.read()
	if err != nil {
		return err
	}
	user := c.user(config)
	if user == nil {
		return fmt.Errorf("argocd context %q has no user in %s", c.context, c.path)
	}
	user["auth-token"] = token
	if refreshToken == "" {
		delete(user, "refresh-token")
	} else {
		user["refresh-token"] = refreshToken
	}
	data, err := yaml.Marshal(config)
	if err != nil {
		return err
	}
	// Write a new file and move it over the old one, so the CLI never reads
	// half a config.
	temporary, err := os.CreateTemp(filepath.Dir(c.path), ".config-*")
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(temporary.Name()) }()
	if _, err := temporary.Write(data); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	if err := os.Chmod(temporary.Name(), 0o600); err != nil {
		return err
	}
	return os.Rename(temporary.Name(), c.path)
}

func (c cliTokenStore) read() (map[string]any, error) {
	data, err := os.ReadFile(c.path)
	if err != nil {
		return nil, err
	}
	var config map[string]any
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("%s: %w", c.path, err)
	}
	return config, nil
}

// user returns the entry of the context's user, which changes in place.
func (c cliTokenStore) user(config map[string]any) map[string]any {
	name := c.context
	if name == "" {
		name, _ = config["current-context"].(string)
	}
	var userName string
	for _, entry := range list(config["contexts"]) {
		if entry["name"] == name {
			userName, _ = entry["user"].(string)
		}
	}
	for _, entry := range list(config["users"]) {
		if userName != "" && entry["name"] == userName {
			return entry
		}
	}
	return nil
}

func list(value any) []map[string]any {
	items, _ := value.([]any)
	entries := make([]map[string]any, 0, len(items))
	for _, item := range items {
		if entry, ok := item.(map[string]any); ok {
			entries = append(entries, entry)
		}
	}
	return entries
}
