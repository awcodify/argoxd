// argoxd is a terminal interface for Argo CD.
package main

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/awcodify/argoxd/internal/argocd"
	"github.com/awcodify/argoxd/internal/explorer"
	"github.com/awcodify/argoxd/internal/tui"
	tea "github.com/charmbracelet/bubbletea"
)

type options struct {
	source     string
	server     string
	token      string
	insecure   bool
	kubeconfig string
	context    string
	namespace  string
	refresh    time.Duration
	demo       bool
}

func main() {
	options, err := parseOptions(os.Args[1:])
	if err != nil {
		log.Fatal(err)
	}
	source, err := buildSource(options)
	if err != nil {
		log.Fatal(err)
	}

	model := tui.New(source, connectionDescription(options), options.namespace, explorer.Snapshot{}).
		WithRefresh(options.refresh)
	if _, err := tea.NewProgram(model, tea.WithAltScreen()).Run(); err != nil {
		log.Fatal(err)
	}
}

// parseOptions resolves settings in increasing priority: built-in defaults,
// the config file, environment variables, then command-line flags.
func parseOptions(arguments []string) (options, error) {
	options := options{
		source:    "kubeconfig",
		server:    os.Getenv("ARGOCD_SERVER"),
		token:     os.Getenv("ARGOCD_AUTH_TOKEN"),
		namespace: "argocd",
		refresh:   5 * time.Second,
	}
	config, err := loadConfig(configPath())
	if err != nil {
		return options, err
	}
	envServer := options.server
	if options, err = config.apply(options); err != nil {
		return options, err
	}
	if envServer != "" {
		options.server = envServer
	}

	flags := flag.NewFlagSet("argoxd", flag.ExitOnError)
	flags.StringVar(&options.source, "source", options.source, "Data source: kubeconfig or api")
	flags.StringVar(&options.server, "server", options.server, "Argo CD API server URL")
	flags.StringVar(&options.token, "auth-token", options.token, "Argo CD API token")
	flags.BoolVar(&options.insecure, "insecure", options.insecure, "Skip TLS certificate verification for API connections")
	flags.StringVar(&options.kubeconfig, "kubeconfig", options.kubeconfig, "Path to kubeconfig (uses default loading rules when empty)")
	flags.StringVar(&options.context, "context", options.context, "Kubeconfig context (uses current context when empty)")
	flags.StringVar(&options.namespace, "namespace", options.namespace, "Namespace where Argo CD is installed")
	flags.DurationVar(&options.refresh, "refresh", options.refresh, "How often to reload resources; 0 disables auto-refresh")
	flags.BoolVar(&options.demo, "demo", false, "Show built-in sample data instead of connecting to Argo CD")
	_ = flags.Parse(arguments) // ExitOnError exits on invalid flags.
	return options, nil
}

func buildSource(options options) (argocd.Source, error) {
	if options.demo {
		return argocd.NewDemoSource(), nil
	}
	switch options.source {
	case "kubeconfig":
		return argocd.NewKubernetesSource(options.kubeconfig, options.context, options.namespace), nil
	case "api":
		if options.server == "" {
			return nil, errors.New("--server or ARGOCD_SERVER is required when --source=api")
		}
		return argocd.NewAPISource(options.server, options.token, options.insecure), nil
	default:
		return nil, fmt.Errorf("unsupported --source %q; use kubeconfig or api", options.source)
	}
}

func connectionDescription(options options) string {
	if options.demo {
		return "Demo data (not connected)"
	}
	if options.source == "api" {
		return "Argo CD API: " + options.server
	}
	if options.context != "" {
		return "Kubeconfig context: " + options.context
	}
	return "Kubeconfig current context"
}
