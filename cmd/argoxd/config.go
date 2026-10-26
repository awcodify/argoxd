package main

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/awcodify/argoxd/internal/explorer"
	"sigs.k8s.io/yaml"
)

// fileConfig holds the defaults read from the config file. Pointer fields
// distinguish "not set" from a zero value such as `refresh: 0`. The auth token
// is deliberately absent: keep it in ARGOCD_AUTH_TOKEN or pass --auth-token.
type fileConfig struct {
	Source     *string `json:"source"`
	Server     *string `json:"server"`
	Insecure   *bool   `json:"insecure"`
	Kubeconfig *string `json:"kubeconfig"`
	Context    *string `json:"context"`
	Namespace  *string `json:"namespace"`
	Refresh    *string `json:"refresh"`
	ReadOnly   *bool   `json:"readOnly"`

	CheckUpdates *bool `json:"checkUpdates"`

	CACert         *string `json:"caCert"`
	ClientCert     *string `json:"clientCert"`
	ClientKey      *string `json:"clientKey"`
	RequestTimeout *string `json:"requestTimeout"`

	Sort        *string `json:"sort"`
	SortReverse *bool   `json:"sortReverse"`
	Mouse       *bool   `json:"mouse"`

	ResourceView *string `json:"resourceView"`
}

// configPath returns the config file location: $ARGOXD_CONFIG, else
// $XDG_CONFIG_HOME/argoxd/config.yaml, else ~/.config/argoxd/config.yaml.
func configPath() string {
	if path := os.Getenv("ARGOXD_CONFIG"); path != "" {
		return path
	}
	if dir := os.Getenv("XDG_CONFIG_HOME"); dir != "" {
		return filepath.Join(dir, "argoxd", "config.yaml")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".config", "argoxd", "config.yaml")
}

// loadConfig reads the config file at path. A missing file is not an error.
func loadConfig(path string) (fileConfig, error) {
	var config fileConfig
	if path == "" {
		return config, nil
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return config, nil
	}
	if err != nil {
		return config, err
	}
	if err := yaml.UnmarshalStrict(data, &config); err != nil {
		return config, fmt.Errorf("%s: %w", path, err)
	}
	return config, nil
}

// apply overlays the values set in the file onto defaults.
func (c fileConfig) apply(defaults options) (options, error) {
	if c.Source != nil {
		defaults.source = *c.Source
	}
	if c.Server != nil {
		defaults.server = *c.Server
	}
	if c.Insecure != nil {
		defaults.insecure = *c.Insecure
	}
	if c.Kubeconfig != nil {
		defaults.kubeconfig = *c.Kubeconfig
	}
	if c.Context != nil {
		defaults.context = *c.Context
	}
	if c.Namespace != nil {
		defaults.namespace = *c.Namespace
	}
	if c.Refresh != nil {
		refresh, err := time.ParseDuration(*c.Refresh)
		if err != nil {
			return defaults, fmt.Errorf("refresh: %w", err)
		}
		defaults.refresh = refresh
	}
	if c.ReadOnly != nil {
		defaults.readOnly = *c.ReadOnly
	}
	if c.CheckUpdates != nil {
		defaults.checkUpdates = *c.CheckUpdates
	}
	if c.CACert != nil {
		defaults.caCert = *c.CACert
	}
	if c.ClientCert != nil {
		defaults.clientCert = *c.ClientCert
	}
	if c.ClientKey != nil {
		defaults.clientKey = *c.ClientKey
	}
	if c.Sort != nil {
		if _, ok := explorer.ParseSortField(*c.Sort); !ok {
			return defaults, fmt.Errorf("sort: %q is not one of name, health or sync", *c.Sort)
		}
		defaults.sort = *c.Sort
	}
	if c.ResourceView != nil {
		defaults.resourceView = *c.ResourceView
	}
	if c.Mouse != nil {
		defaults.mouse = *c.Mouse
	}
	if c.SortReverse != nil {
		defaults.sortReverse = *c.SortReverse
	}
	if c.RequestTimeout != nil {
		timeout, err := time.ParseDuration(*c.RequestTimeout)
		if err != nil {
			return defaults, fmt.Errorf("requestTimeout: %w", err)
		}
		defaults.requestTimeout = timeout
	}
	return defaults, nil
}
