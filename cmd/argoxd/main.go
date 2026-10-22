// argoxd is a terminal interface for Argo CD.
package main

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"runtime/debug"
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
	readOnly   bool
	demo       bool
	version    bool
}

func main() {
	options, err := parseOptions(os.Args[1:])
	if err != nil {
		log.Fatal(err)
	}
	if options.version {
		info, _ := debug.ReadBuildInfo()
		fmt.Println(versionString(version, info))
		return
	}
	source, err := buildSource(options)
	if err != nil {
		log.Fatal(err)
	}

	model := tui.New(source, connectionDescription(options), options.namespace, explorer.Snapshot{}).
		WithRefresh(options.refresh).
		WithReadOnly(options.readOnly)
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

	flags := newFlagSet(&options)
	_ = flags.Parse(arguments) // ExitOnError exits on invalid flags.
	return options, nil
}

// shorthands maps each short flag to the long flag it abbreviates.
var shorthands = map[string]string{"s": "server", "k": "insecure", "c": "context", "n": "namespace", "r": "refresh"}

// newFlagSet declares the command-line flags, defaulting to the values in options.
func newFlagSet(options *options) *flag.FlagSet {
	flags := flag.NewFlagSet("argoxd", flag.ExitOnError)
	flags.StringVar(&options.source, "source", options.source, "Data source: kubeconfig or api")
	flags.StringVar(&options.server, "server", options.server, "Argo CD API server URL")
	flags.StringVar(&options.server, "s", options.server, "Shorthand for --server")
	flags.StringVar(&options.token, "auth-token", options.token, "Argo CD API token")
	flags.BoolVar(&options.insecure, "insecure", options.insecure, "Skip TLS certificate verification for API connections")
	flags.BoolVar(&options.insecure, "k", options.insecure, "Shorthand for --insecure")
	flags.StringVar(&options.kubeconfig, "kubeconfig", options.kubeconfig, "Path to kubeconfig (uses default loading rules when empty)")
	flags.StringVar(&options.context, "context", options.context, "Kubeconfig context (uses current context when empty)")
	flags.StringVar(&options.context, "c", options.context, "Shorthand for --context")
	flags.StringVar(&options.namespace, "namespace", options.namespace, "Namespace where Argo CD is installed")
	flags.StringVar(&options.namespace, "n", options.namespace, "Shorthand for --namespace")
	flags.DurationVar(&options.refresh, "refresh", options.refresh, "How often to reload resources; 0 disables auto-refresh")
	flags.DurationVar(&options.refresh, "r", options.refresh, "Shorthand for --refresh")
	flags.BoolVar(&options.readOnly, "read-only", options.readOnly, "Disable actions that change Argo CD or live resources")
	flags.BoolVar(&options.demo, "demo", false, "Show built-in sample data instead of connecting to Argo CD")
	flags.BoolVar(&options.version, "version", false, "Print the argoxd version and exit")
	flags.Usage = func() { printUsage(flags) }
	return flags
}

// printUsage lists the flags with each shorthand on the row of its long flag,
// e.g. "-n, --namespace string".
func printUsage(flags *flag.FlagSet) {
	short := make(map[string]string, len(shorthands))
	for abbreviation, long := range shorthands {
		short[long] = abbreviation
	}
	fmt.Fprintf(flags.Output(), "Usage of %s:\n", flags.Name())
	flags.VisitAll(func(f *flag.Flag) {
		if _, isShort := shorthands[f.Name]; isShort {
			return
		}
		name := "--" + f.Name
		if abbreviation, found := short[f.Name]; found {
			name = "-" + abbreviation + ", " + name
		}
		if kind, _ := flag.UnquoteUsage(f); kind != "" {
			name += " " + kind
		}
		fmt.Fprintf(flags.Output(), "  %s\n    \t%s", name, f.Usage)
		if f.DefValue != "" && f.DefValue != "false" && f.DefValue != "0s" {
			fmt.Fprintf(flags.Output(), " (default %s)", f.DefValue)
		}
		fmt.Fprintln(flags.Output())
	})
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
