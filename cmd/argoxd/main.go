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
}

func main() {
	options := parseOptions(os.Args[1:])
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

func parseOptions(arguments []string) options {
	var options options
	flags := flag.NewFlagSet("argoxd", flag.ExitOnError)
	flags.StringVar(&options.source, "source", "kubeconfig", "Data source: kubeconfig or api")
	flags.StringVar(&options.server, "server", os.Getenv("ARGOCD_SERVER"), "Argo CD API server URL")
	flags.StringVar(&options.token, "auth-token", os.Getenv("ARGOCD_AUTH_TOKEN"), "Argo CD API token")
	flags.BoolVar(&options.insecure, "insecure", false, "Skip TLS certificate verification for API connections")
	flags.StringVar(&options.kubeconfig, "kubeconfig", "", "Path to kubeconfig (uses default loading rules when empty)")
	flags.StringVar(&options.context, "context", "", "Kubeconfig context (uses current context when empty)")
	flags.StringVar(&options.namespace, "namespace", "argocd", "Namespace where Argo CD is installed")
	flags.DurationVar(&options.refresh, "refresh", 5*time.Second, "How often to reload resources; 0 disables auto-refresh")
	_ = flags.Parse(arguments) // ExitOnError exits on invalid flags.
	return options
}

func buildSource(options options) (argocd.Source, error) {
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
	if options.source == "api" {
		return "Argo CD API: " + options.server
	}
	if options.context != "" {
		return "Kubeconfig context: " + options.context
	}
	return "Kubeconfig current context"
}
