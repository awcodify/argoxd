# argoxd

`argoxd` is an interactive terminal user interface for Argo CD, inspired by the resource-focused workflow of k9s. It lists Applications, AppProjects, destination clusters, and connection settings from either the Kubernetes API or the Argo CD API.

## Install

```sh
go install github.com/awcodify/argoxd/cmd/argoxd@latest
```

For local development:

```sh
go run ./cmd/argoxd
```

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

## Layout

The header shows the connection, the project shortcuts and the key hints. Below it is the active resource list in a titled frame such as `◆ applications · store · 3`, then breadcrumbs and a status line. Status columns use Argo CD's colors and glyphs: `♥ Healthy`, `✓ Synced`, `◐ Progressing`, `⟳ OutOfSync`, `✗ Degraded`.

Press Enter on an Application to open its dependencies. A summary strip counts resources by status. The Application and its resources are shown as cards nested under their owners (Deployment → ReplicaSet → Pod), and a details pane describes the selected card. Press `/` to filter the cards by `kind/name` (e.g. `pod`, `web`, `deploy/web`): matching cards stay with the cards on their path, which are dimmed for context. From a card you can open its live YAML (`y`), its diff against the desired state (`d`, needs `--source api`), or a Pod's recent logs (`l`).

## Commands

Press `/` on a list to filter it by name as you type: Enter keeps the filter, Esc clears it.

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
| `/` | Filter the list by name, or the dependency cards by `kind/name` |
| `j` / Down, `k` / Up | Move the selection, or scroll a YAML, diff or log view |
| `g` / `G` | Jump to the top or bottom of a YAML, diff or log view |
| `t` | Open the inventory tree: Projects → Applications and Clusters |
| Enter | Open the selected Application's dependencies |
| `y` / `d` / `l` | Show the selected card's YAML, diff, or logs |
| Space / Left / Right | Expand or collapse the selected inventory node |
| Esc | Go back, or clear the filter |
| `s` | Sync the selected Application; toggle `p` prune and `r` dry run, then Enter |
| `R` | Hard refresh the selected Application, bypassing Argo CD's manifest cache |
| `D`, then `y` | Delete the selected Application and its managed resources |
| `r` | Reload resources now |
| `q` / Ctrl+C | Quit |

## Development

```sh
go test ./...
go build ./cmd/argoxd
```
