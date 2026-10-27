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
	StartView    *string `json:"startView"`

	K9sCommand *string `json:"k9sCommand"`
	Theme      *string `json:"theme"`
	EventsPane *bool   `json:"eventsPane"`
	// Themes defines themes of your own: a name, then optionally a base theme
	// and the colors to change.
	Themes        map[string]customTheme `json:"themes"`
	DiffViewer    *string                `json:"diffViewer"`
	ArgoCDContext *string                `json:"argocdContext"`
	ArgoCDConfig  *string                `json:"argocdConfig"`
}

// customTheme is one theme defined in the config file: "base" names the theme
// it starts from and every other key is a color, such as accent or danger. The
// values are read as whatever YAML made of them, so that an unquoted number and
// an unquoted hex color, which YAML turns into a comment, can be explained.
type customTheme map[string]any

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
	if c.Themes != nil {
		defaults.themes = c.Themes
	}
	if c.EventsPane != nil {
		defaults.eventsPane = *c.EventsPane
	}
	if c.Theme != nil {
		defaults.theme = *c.Theme
	}
	if c.DiffViewer != nil {
		defaults.diffViewer = *c.DiffViewer
	}
	if c.K9sCommand != nil {
		defaults.k9sCommand = *c.K9sCommand
	}
	if c.ArgoCDContext != nil {
		defaults.argocdContext = *c.ArgoCDContext
	}
	if c.ArgoCDConfig != nil {
		defaults.argocdConfig = *c.ArgoCDConfig
	}
	if c.StartView != nil {
		defaults.startView = *c.StartView
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
