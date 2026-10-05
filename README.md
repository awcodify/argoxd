# argoxd

`argoxd` is an interactive terminal user interface for Argo CD, inspired by the resource-focused workflow of k9s. It lists Applications, AppProjects, destination clusters, and connection settings from either the Kubernetes API or the Argo CD API.

![argoxd demo](demo/demo.gif)

## Install

```sh
go install github.com/awcodify/argoxd/cmd/argoxd@latest
```

Prebuilt binaries for Linux, macOS and Windows are attached to each [GitHub release](https://github.com/awcodify/argoxd/releases).

For local development:

```sh
go run ./cmd/argoxd
```

## Try it without Argo CD

```sh
argoxd --demo
```

`--demo` shows built-in sample Applications, resources, YAML, diffs and logs, and needs no cluster or credentials. Sync and delete change the sample data in memory only.

## Connect with kubeconfig

This is the default. `argoxd` loads the active kubeconfig context and looks for Argo CD in the `argocd` namespace.

```sh
argoxd
argoxd --context production --namespace argocd
argoxd --kubeconfig ~/.kube/config --context production
```

The selected identity must be allowed to list `applications.argoproj.io`, `appprojects.argoproj.io`, and Argo CD cluster Secrets. To show the ReplicaSets, Jobs and Pods behind each workload, it also needs to list those in the Application's destination namespaces; kinds it cannot list are left out. Viewing a Pod's logs needs `pods/log`.

Resources reload every 5 seconds. Change this with `--refresh 30s`, or turn it off with `--refresh 0`. Requests that take longer than 15 seconds are abandoned and reported as errors.

## Connect to the Argo CD API

```sh
ARGOCD_SERVER=https://argocd.example.com \
ARGOCD_AUTH_TOKEN="$TOKEN" \
argoxd --source api
```

Or provide the values directly:

```sh
argoxd --source api --server https://argocd.example.com --auth-token "$TOKEN"
```

Use `--insecure` only when connecting to a development instance with an untrusted TLS certificate.

## Configuration

Defaults can be kept in `~/.config/argoxd/config.yaml` (or `$XDG_CONFIG_HOME/argoxd/config.yaml`; set `ARGOXD_CONFIG` to use another path). Every key is optional:

```yaml
source: kubeconfig   # kubeconfig or api
context: production
namespace: argocd
kubeconfig: ~/.kube/config
server: https://argocd.example.com   # for source: api
insecure: false
refresh: 5s          # 0s turns auto-refresh off
```

Settings are resolved in this order, later ones winning: built-in defaults, the config file, the `ARGOCD_SERVER` and `ARGOCD_AUTH_TOKEN` environment variables, then command-line flags. The API token is not read from the config file; keep it in `ARGOCD_AUTH_TOKEN`. Unknown keys and invalid durations are reported as errors at startup.

## Layout

The header shows the connection, the project shortcuts and the key hints. Below it is the active resource list in a titled frame such as `◆ applications · store · 3`, then breadcrumbs and a status line. Status columns use Argo CD's colors and glyphs: `♥ Healthy`, `✓ Synced`, `◐ Progressing`, `⟳ OutOfSync`, `✗ Degraded`.

Press Enter on an Application to open its dependencies. A summary strip counts resources by status. The Application and its resources are shown as cards nested under their owners (Deployment → ReplicaSet → Pod), and a details pane describes the selected card. Press `/` to search the cards by `kind/name` (e.g. `pod`, `web`, `deploy/web`): matching cards stay with the cards on their path, which are dimmed for context. From a card you can open its live YAML (`y`), its diff against the desired state (`d`, needs `--source api`), or a Pod's recent logs (`l`).

## Commands

Press `/` on a list to search it by name as you type: Enter keeps the search, Esc clears it.

`H`, `S` and `K` open a bar like `:` with suggestions for one field: health (Healthy, Progressing, Degraded, Suspended, Missing, Unknown), sync status (Synced, OutOfSync, Unknown) and, on dependency cards, resource kind. The bar lists `all` followed by the values, starting on the current one. Tab or ↓ moves to the next value, Shift+Tab or ↑ to the previous one (both wrap around), and Enter applies the highlighted value. Type to narrow the list instead. Choosing `all` clears just that field. Health and sync work on Applications and on dependency cards. Filters combine with each other and with the search, and Esc clears them together. The same filters work from the command bar: `:health degraded`, `:sync outofsync` and `:kind pod` complete as you type, the suggestions show their shortcut (`shift+H`, `shift+S`, `shift+K`), and `:health all` (or `:health` alone) clears that filter.

Press `:` to open the command bar. Matching commands are suggested as you type: Tab or Right accepts the highlighted suggestion, Up/Down picks another, Enter runs what you typed, Esc closes it.

| Command | Opens |
| --- | --- |
| `:app`, `:apps`, `:applications` | Applications, optionally filtered: `:app store`, `:app all` |
| `:proj`, `:projects` | Projects |
| `:cluster`, `:clusters` | Clusters |
| `:settings` | Connection settings |
| `:q`, `:quit` | Quit |

## Keys

| Key | Action |
| --- | --- |
| `0` | Show Applications from every project |
| `1`–`9` | Show Applications from the project listed under that number in the header |
| `:` | Open the command bar |
| `/` | Search the list by name, or the dependency cards by `kind/name` |
| `H` / `S` | Filter by health / sync status, with suggestions (Applications and dependency cards) |
| `K` | Filter dependency cards by resource kind, with suggestions |
| `j` / Down, `k` / Up | Move the selection, or scroll a YAML, diff or log view |
| `g` / `G` | Jump to the top or bottom of a YAML, diff or log view |
| `t` | Open the inventory tree: Projects → Applications and Clusters |
| Enter | Open the selected Application's dependencies |
| `y` / `d` / `l` | Show the selected card's YAML, diff, or logs |
| Space / Left / Right | Expand or collapse the selected inventory node |
| Esc | Go back, or clear the search and filters |
| `s` | Sync the selected Application; toggle `p` prune and `r` dry run, then Enter |
| `R` | Hard refresh the selected Application, bypassing Argo CD's manifest cache |
| `D`, then `y` | Delete the selected Application and its managed resources |
| `r` | Reload resources now |
| `q` / Ctrl+C | Quit |

## License

[MIT](LICENSE)

## Development

```sh
go test ./...
go build ./cmd/argoxd
```

`make demo` re-records `demo/demo.gif` from the sample data; it needs [vhs](https://github.com/charmbracelet/vhs).

Pushing a tag such as `v0.1.0` builds and publishes release binaries with GoReleaser.
