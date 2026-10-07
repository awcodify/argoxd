# argoxd

`argoxd` is an interactive terminal user interface for Argo CD, inspired by the resource-focused workflow of k9s. It lists Applications, ApplicationSets, AppProjects, destination clusters, and connection settings from either the Kubernetes API or the Argo CD API.

![argoxd demo](demo/demo.gif)

## Install

With Homebrew (macOS and Linux):

```sh
brew install awcodify/tap/argoxd
```

With Go:

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

The selected identity must be allowed to list `applications.argoproj.io`, `appprojects.argoproj.io`, and Argo CD cluster Secrets. To show the ReplicaSets, Jobs and Pods behind each workload, it also needs to list those in the Application's destination namespaces; kinds it cannot list are left out. Viewing a Pod's logs needs `pods/log`, and viewing events needs `list` on `events` in the namespaces involved (and in the Argo CD namespace for an Application's own events). Restarting a workload needs `patch` on it and deleting a Pod needs `delete` on `pods`. With `--source api`, the token needs Argo CD's `action/…/restart` and resource delete permissions.

ApplicationSets are optional. With `list` on `applicationsets.argoproj.io` (or access to `/api/v1/applicationsets` with `--source api`) they appear under `:appset`; without it, or on an Argo CD that has none, they are left out and everything else still loads. Changing an Application's sync policy (`P`) needs `patch` on `applications.argoproj.io`, or permission to update Applications with `--source api`.

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

Press Enter on an Application to open its dependencies. A summary strip counts resources by status. The Application and its resources are shown as cards nested under their owners (Deployment → ReplicaSet → Pod), and a details pane describes the selected card. Press `/` to search the cards by `kind/name` (e.g. `pod`, `web`, `deploy/web`): matching cards stay with the cards on their path, which are dimmed for context. From a card you can open its live YAML (`y`), its diff against the desired state (`d`, needs `--source api`), a Pod's recent logs (`l`), or its Kubernetes events (`e`). The events of a workload include those of the ReplicaSets and Pods under it (up to 30 resources). On the Applications list, `e` shows the selected Application's events.

Press Space to mark Applications on the list, or resource cards in this view. `s`, `R`, `D`, `x` and `X` then act on everything marked instead of the selected item: each marked Application, or in this view only the marked resources. Marked rows and cards show a `●`; Esc clears the marks.

Every action asks first. A confirmation replaces the view and lists each Application or resource the action will touch, then waits for Enter (or `y`) to run it or Esc (or `n`) to cancel. Sync and rollback also offer `p` prune and `r` dry run there. When several items are marked they run one after another; if some fail, the status line says how many and why, and the rest still run.

Rollback follows `argocd app rollback`: Argo CD refuses it while the Application has auto-sync enabled, so disable auto-sync first (`P`). With `--source kubeconfig` the identity also needs to `get` and `patch` `applications.argoproj.io`.

### Conditions and sync details

An Application that Argo CD reports conditions for, such as a `ComparisonError` or a `SyncError`, has a `⚠` after its name on the list. The dependency view lists its first three conditions above the cards and counts the rest.

`i` shows how the Application's last sync went: its phase, message and revision, how long ago it started and finished, and what it did to each resource or hook (its sync phase and result), with the resources that did not sync first. It works on the Applications list, in the dependency view and in the overview.

### Sync policy

`P` sets how the marked or selected Applications sync on their own. The dialog starts from the first Application's policy: `a` toggles auto-sync, `h` self-heal and `p` prune, and Enter applies it. Self-heal and prune only count while auto-sync is on, so turning it off clears them. Other sync options, such as `CreateNamespace=true`, are left as they are.

### Resources to prune and orphans

In the dependency view `✂ to prune` marks a resource that is no longer in Git and that a sync with prune would delete, and `◌ orphaned` marks a resource in the destination namespace that no Application manages. The summary strip counts both and the details pane explains each. Resources to prune work with both sources; with `--source api` they need the token to be able to read the Application, and without that they are not marked. Orphans only come from the Argo CD API, and only when the AppProject monitors orphaned resources.

### App of apps

A card that is itself an Application, as an app of apps deploys, opens with Enter. Esc returns to the Application it came from, with its cursor, search, filter and marks as they were, and the breadcrumbs show the whole path. Actions in the child act on the child. Cards of child Applications can be marked and synced selectively like any other resource.

### ApplicationSets

`:appset` lists each ApplicationSet with its generators (such as `git` or `matrix(list, clusters)`), how many Applications it generated, and its status: `OK`, or the problem it reports. Enter shows the Applications it generated (`applications · appset/<name>`); `0` or choosing a project shows them all again.

### Pulse

`:pulse` (or `:overview`) shows the state of everything at once. On a terminal at least 100 columns wide it draws four panels: the number of Applications and ApplicationSets, health and sync as bars sized by count with a legend, and how many Applications use auto-sync, self-heal and prune. Narrower or shorter terminals get the same counts as text. Below them it lists the Applications that need attention, worst first: degraded or missing ones, ones whose last sync failed, and ones with conditions, with the reasons. Enter opens one, and `i`, `e`, `s`, `P`, `h`, `R` and `D` work on its row.

## Commands

Press `/` on a list to search it by name as you type: Enter keeps the search, Esc clears it.

`H`, `S` and `K` open a bar like `:` with suggestions for one field: health (Healthy, Progressing, Degraded, Suspended, Missing, Unknown), sync status (Synced, OutOfSync, Unknown) and, on dependency cards, resource kind. The bar lists `all` followed by the values, starting on the current one. Tab or ↓ moves to the next value, Shift+Tab or ↑ to the previous one (both wrap around), and Enter applies the highlighted value. Type to narrow the list instead. Choosing `all` clears just that field. Health and sync work on Applications and on dependency cards. Filters combine with each other and with the search, and Esc clears them together. The same filters work from the command bar: `:health degraded`, `:sync outofsync` and `:kind pod` complete as you type, the suggestions show their shortcut (`shift+H`, `shift+S`, `shift+K`), and `:health all` (or `:health` alone) clears that filter.

Press `:` to open the command bar. Matching commands are suggested as you type: Tab or Right accepts the highlighted suggestion, Up/Down picks another, Enter runs what you typed, Esc closes it.

| Command | Opens |
| --- | --- |
| `:app`, `:apps`, `:applications` | Applications, optionally filtered: `:app store`, `:app all` |
| `:proj`, `:projects` | Projects |
| `:cluster`, `:clusters` | Clusters |
| `:appset`, `:appsets`, `:applicationset`, `:applicationsets` | ApplicationSets |
| `:pulse`, `:overview` | Overview of the Applications that need attention |
| `:settings` | Connection settings |
| `:q`, `:quit` | Quit |

## Keys

| Key | Action |
| --- | --- |
| `0` | Show Applications from every project |
| `1`–`9` | Show Applications from the project listed under that number in the header |
| `:` | Open the command bar |
| `/` | Search the list by name, or the dependency cards by `kind/name`; in a log, keep only the lines that contain the text (ignoring case). Enter keeps the filter, Esc clears it |
| `H` / `S` | Filter by health / sync status, with suggestions (Applications and dependency cards) |
| `K` | Filter dependency cards by resource kind, with suggestions |
| `j` / Down, `k` / Up | Move the selection, or scroll a YAML, diff or log view |
| `g` / `G` | Jump to the top or bottom of a YAML, diff or log view |
| `t` | Open the inventory tree: Projects → Applications and Clusters |
| Enter | Open the selected Application's dependencies, an ApplicationSet's Applications, or, in the dependency view, the Application a card stands for |
| Space | Mark or unmark the selected Application, or the selected resource card, and move to the next; `s`, `R`, `D`, `x` and `X` then act on the marked ones. Esc clears the marks. In the inventory tree, Space expands or collapses the selected node |
| `y` / `d` / `l` | Show the selected card's YAML, diff, or logs. On a Deployment, StatefulSet, DaemonSet, ReplicaSet or Job, `l` follows the logs of all its Pods, each line starting with the Pod's name (up to 30 Pods). On a Pod that such a workload runs, `l` opens that same log already filtered to the Pod and followed, so `p` can switch to its siblings or to `all` |
| `f` | In a log view, follow new lines as they arrive (the last 10,000 are kept); press again, or Esc, to stop |
| `c` | In a log view, open a bar to choose the container, like `H`, `S` and `K`: it lists the containers and `all`; Tab or ↓ and Shift+Tab or ↑ move through them, typing narrows them, Enter shows the highlighted one. With `all`, lines start with `container`, or `pod/container` on a workload. A Pod starts on its `kubectl.kubernetes.io/default-container`, else its first container |
| `p` | In the log of a workload, or of a Pod it runs, open a bar to choose one of the workload's Pods, or `all`; it works like the container bar and matches any part of a Pod's name |
| Left / Right | Expand or collapse the selected inventory node |
| Esc | Go back, or clear the search and filters |
| `e` | Show the events of the selected Application, or of the selected card and its Pods. In the events view, `/` keeps only the lines that contain the text |
| `s` | Sync the marked Applications, or the marked resources in the dependency view, else the selected Application. Confirm with Enter; toggle `p` prune and `r` dry run first if needed |
| `h` | Show the selected Application's deployment history, newest first. `j`/`k` pick a deployment, Enter rolls back to it (toggle `p` prune and `r` dry run, then Enter) |
| `i` | Show how the selected Application's last sync went, resource by resource |
| `P` | Set the sync policy of the marked or selected Applications: `a` auto-sync, `h` self-heal, `p` prune, Enter applies |
| `R` | Hard refresh the marked or selected Applications, bypassing Argo CD's manifest cache. Asks first |
| `D` | Delete the marked or selected Applications and their managed resources. Asks first |
| `x` | In the dependency view, restart the marked or selected Deployments, StatefulSets and DaemonSets with a rolling restart, like `kubectl rollout restart`. Asks first; refused if a marked card cannot be restarted |
| `X` | In the dependency view, delete the marked or selected Pods; the workload that owns each starts a replacement. Asks first; refused if a marked card is not a Pod |
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
