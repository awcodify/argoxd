// argoxd is a terminal interface for Argo CD.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"runtime/debug"
	"slices"
	"strings"
	"time"

	"github.com/awcodify/argoxd/internal/argocd"
	"github.com/awcodify/argoxd/internal/explorer"
	"github.com/awcodify/argoxd/internal/tui"
	"github.com/awcodify/argoxd/internal/update"
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

	checkUpdates bool

	caCert         string
	clientCert     string
	clientKey      string
	requestTimeout time.Duration

	sort        string
	sortReverse bool
	mouse       bool

	resourceView string
	startView    string

	k9sCommand    string
	theme         string
	diffViewer    string
	argocdContext string
	argocdConfig  string
	tokenFromCLI  bool
}

func main() {
	options, err := parseOptions(os.Args[1:])
	if err != nil {
		log.Fatal(err)
	}
	info, _ := debug.ReadBuildInfo()
	if options.version {
		fmt.Println(versionString(version, info))
		return
	}
	if options, err = withArgoCDLogin(options); err != nil {
		log.Fatal(err)
	}
	source, err := buildSource(options)
	if err != nil {
		log.Fatal(err)
	}

	model := tui.New(source, connectionDescription(options), options.namespace, explorer.Snapshot{}).
		WithRefresh(options.refresh).
		WithReadOnly(options.readOnly).
		WithRequestTimeout(options.requestTimeout).
		WithResourceView(options.resourceView).
		WithStartView(options.startView).
		WithExternalTools(tui.ExternalTools{K9s: options.k9sCommand, KubeContext: options.context, DiffViewer: options.diffViewer})
	if field, ok := explorer.ParseSortField(options.sort); ok {
		model = model.WithSort(field, options.sortReverse)
	}
	if options.checkUpdates && !options.demo {
		current := releaseVersion(version, info)
		checker := update.NewChecker()
		model = model.WithUpdateCheck(func(ctx context.Context) string { return checker.Available(ctx, current) })
	}
	program := []tea.ProgramOption{tea.WithAltScreen()}
	if options.mouse {
		program = append(program, tea.WithMouseCellMotion())
	}
	if _, err := tea.NewProgram(model, program...).Run(); err != nil {
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

		checkUpdates: true,

		requestTimeout: 15 * time.Second,
		resourceView:   "cards",
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
	if os.Getenv("ARGOXD_NO_UPDATE_CHECK") != "" {
		options.checkUpdates = false
	}

	flags := newFlagSet(&options)
	_ = flags.Parse(arguments) // ExitOnError exits on invalid flags.
	if options.theme != "" {
		if err := tui.SetTheme(options.theme); err != nil {
			return options, err
		}
	}
	if options.startView != "" {
		name, _, _ := strings.Cut(strings.TrimSpace(options.startView), " ")
		if !slices.Contains(tui.StartViewCommands(), name) {
			return options, fmt.Errorf("start view %q must be one of %s, optionally followed by a project", options.startView, strings.Join(tui.StartViewCommands(), ", "))
		}
	}
	if options.resourceView != "cards" && options.resourceView != "table" {
		return options, fmt.Errorf("resource view must be cards or table, got %q", options.resourceView)
	}
	if options.requestTimeout <= 0 {
		return options, fmt.Errorf("request timeout must be positive, got %s", options.requestTimeout)
	}
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
	flags.StringVar(&options.caCert, "ca-cert", options.caCert, "PEM file of extra certificate authorities to trust for --source api")
	flags.StringVar(&options.clientCert, "client-cert", options.clientCert, "PEM client certificate for --source api, if the server asks for one")
	flags.StringVar(&options.clientKey, "client-key", options.clientKey, "PEM private key of the client certificate")
	flags.DurationVar(&options.requestTimeout, "request-timeout", options.requestTimeout, "How long a request to Argo CD or Kubernetes may take")
	flags.StringVar(&options.argocdContext, "argocd-context", options.argocdContext, "Context of the argocd CLI login to use for --source api when no server or token is given")
	flags.StringVar(&options.argocdConfig, "argocd-config", options.argocdConfig, "Path of the argocd CLI config (default ~/.config/argocd/config)")
	flags.StringVar(&options.theme, "theme", options.theme, "Color theme: "+strings.Join(tui.ThemeNames(), ", "))
	flags.StringVar(&options.startView, "start-view", options.startView, "View to open first, as a command without the colon, e.g. proj or \"app store\"")
	flags.StringVar(&options.resourceView, "resource-view", options.resourceView, "How an Application's resources are first shown: cards or table")
	flags.BoolVar(&options.mouse, "mouse", options.mouse, "Scroll with the mouse wheel; the terminal then needs Shift to select text")
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
		return argocd.NewConfiguredAPISource(options.server, options.token, argocd.APIOptions{
			Insecure: options.insecure, CACertFile: options.caCert,
			ClientCertFile: options.clientCert, ClientKeyFile: options.clientKey, Timeout: options.requestTimeout,
			TokenFromCLI: options.tokenFromCLI,
		})
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
