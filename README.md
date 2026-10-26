# argoxd

A terminal UI for Argo CD: browse Applications, spot drift, and sync, refresh or roll back without leaving the terminal. It reads from the Kubernetes API or the Argo CD API.

argoxd sticks to the GitOps lifecycle. It does not exec into Pods, port-forward, or edit live manifests.

![argoxd demo](demo/demo.gif)

## Install

```sh
brew install awcodify/tap/argoxd                          # macOS and Linux
go install github.com/awcodify/argoxd/cmd/argoxd@latest   # with Go
```

Prebuilt binaries for Linux, macOS and Windows are on the [releases page](https://github.com/awcodify/argoxd/releases). `argoxd --version` prints the installed version.

## Quick start

```sh
argoxd --demo                     # sample data, no cluster needed
argoxd                            # current kubeconfig context, Argo CD in "argocd"
argoxd -c production -n argocd    # another context and namespace
argoxd --source api -s https://argocd.example.com --auth-token "$TOKEN"
```

With `--source api`, `ARGOCD_SERVER` and `ARGOCD_AUTH_TOKEN` work in place of the flags.

If your Argo CD uses a certificate from an internal CA, trust it with `--ca-cert corp-ca.pem` instead of `-k` (`--insecure`), which skips verification and is only for development. If the server asks for a client certificate, add `--client-cert client.crt --client-key client.key`. These three apply to `--source api`.

In the demo, `cart` is blocked by a sync window and has drift, `search` has a running operation to terminate, and `:proj` → `Y` on `store` shows a full AppProject. Actions change the sample data in memory only.

## Configuration

Defaults go in `~/.config/argoxd/config.yaml` (or `$XDG_CONFIG_HOME/argoxd/config.yaml`, or the path in `ARGOXD_CONFIG`). Every key is optional:

```yaml
source: kubeconfig   # kubeconfig or api
context: production
namespace: argocd
kubeconfig: ~/.kube/config
server: https://argocd.example.com   # for source: api
insecure: false
refresh: 5s          # 0s turns auto-refresh off
readOnly: false      # block every action that changes something
checkUpdates: true   # false stops the daily check for a newer release
requestTimeout: 15s  # raise it for installs with thousands of Applications
sort: health         # name, health (worst first) or sync (out of sync first)
sortReverse: false
mouse: false         # true scrolls with the wheel; the terminal then needs Shift to select text
caCert: /etc/ssl/corp-ca.pem         # for source: api
clientCert: /etc/argoxd/client.crt   # for source: api, with clientKey
clientKey: /etc/argoxd/client.key
```

argoxd checks GitHub for a newer release at most once a day, in the background, and quietly does nothing if it cannot connect. Development builds and `--demo` skip the check. To turn it off, set `checkUpdates: false` or the environment variable `ARGOXD_NO_UPDATE_CHECK=1`.

Flags beat environment variables, which beat the config file. The API token is never read from the file; use `ARGOCD_AUTH_TOKEN`. Unknown keys and bad durations fail at startup.

| Flag | Short | Default |
| --- | --- | --- |
| `--source` | | `kubeconfig` |
| `--context` | `-c` | current context |
| `--namespace` | `-n` | `argocd` |
| `--kubeconfig` | | default loading rules |
| `--server` | `-s` | `$ARGOCD_SERVER` |
| `--auth-token` | | `$ARGOCD_AUTH_TOKEN` |
| `--insecure` | `-k` | `false` |
| `--ca-cert` | | none |
| `--client-cert`, `--client-key` | | none |
| `--request-timeout` | | `15s` |
| `--refresh` | `-r` | `5s` |
| `--mouse` | | `false` |
| `--read-only` | | `false` |
| `--demo` | | `false` |
| `--version` | | |

## Features

- **Applications list** with Argo CD's health and sync glyphs (`♥ ✓ ◐ ⟳ ✗`), `⚠` for conditions and `⊘` for a sync-window block. `:sort` orders it by name, health or sync.
- **Dependency view** (Enter): the Application's resources as cards nested under their owners (Deployment → ReplicaSet → Pod), with a status summary and the first three conditions.
- **Inspect**: live YAML (`Y`), diff against Git (`d`), logs (`l`), events (`e`) and the last sync, resource by resource (`i`).
- **Logs**: follow (`f`), pick the container (`c`) or Pod (`p`), and read up to 30 Pods of a workload at once.
- **Actions**: sync, selective sync, refresh, rollback, sync policy, terminate, delete, rolling restart and Pod delete. Each one asks first.
- **Marking**: Space marks Applications or cards; actions then run on all of them, one after another.
- **Prune and orphans**: `✂ to prune` for resources a pruning sync would delete, `◌ orphaned` for resources no Application manages.
- **App of apps**: Enter on a child Application opens it; Esc returns to the parent.
- **ApplicationSets** (`:appset`) with their generators, status and generated Applications.
- **Pulse** (`:pulse`): health and sync at a glance, and the Applications that need attention, worst first.
- **Runtime switching**: `:ctx` changes the kubeconfig context, `:ns` the Argo CD namespace, without restarting.
- **Read-only mode** (`--read-only`): every mutating action is blocked and the header shows `[READ-ONLY]`.
- **Clipboard and browser**: `y` copies, `v` selects lines in a viewer, `o` opens the Application in the Argo CD web UI.

Some features need `--source api`: diffs, orphaned resources, and `o`.

## Commands

Press `:` to open the command bar. It suggests as you type: Tab fills in the highlighted suggestion and, pressed again, moves to the next; ↑ and ↓ move through them too. Enter runs, Esc closes.

| Command | Opens |
| --- | --- |
| `:app [project\|all]` | Applications, optionally of one project |
| `:proj` | Projects (`Y` shows the AppProject) |
| `:cluster` | Clusters |
| `:appset` | ApplicationSets |
| `:pulse`, `:overview` | Overview of what needs attention |
| `:ctx [name]` | Switch kubeconfig context (picker without a name) |
| `:ns [name]` | Switch the Argo CD namespace (picker without a name) |
| `:sort [name\|health\|sync] [reverse]` | Order the Applications; health and sync put the worst first. Run it again with the same field to flip the direction, add `reverse` to force it, or use `:sort` alone to restore Argo CD's order |
| `:health`, `:sync`, `:kind` `<value>` | Filter, same as `H`, `S`, `K` |
| `:settings` | Connection settings |
| `:q` | Quit |

`:ctx` and `:ns` need `--source kubeconfig`.

## Keys

**Navigate**

| Key | Action |
| --- | --- |
| `j` `k` / ↓ ↑ | Move, or scroll a viewer |
| `g` `G` | Top / bottom of a viewer |
| Enter | Open the dependencies, an ApplicationSet's Applications, or a child Application |
| Esc | Back, or clear the search, filters and marks |
| `0`, `1`–`9` | All projects, or the numbered project in the header |
| `/` | Search by name, or by `kind/name` on cards; in logs and events, keep matching lines |
| `H` `S` `K` | Filter by health, sync status or kind |
| `t` | Inventory tree (Space or ←/→ expands) |
| `r` | Reload now |
| `q` | Quit |

**Inspect**

| Key | Action |
| --- | --- |
| `Y` | YAML of the Application, card or AppProject |
| `d` | Diff (`--source api`) |
| `e` | Events, including those of a workload's Pods |
| `i` | Last sync, resource by resource |
| `h` | Deployment history; Enter rolls back |
| `l` | Logs; on a workload, all its Pods |
| `f` `c` `p` | In logs: follow, choose container, choose Pod |
| `y` | Copy the selection, marked items or viewer text |
| `v` | Select lines in a viewer, then `y` |
| `o` | Open in the Argo CD web UI (`--source api`) |

**Act** (every action asks first; Enter or `y` confirms)

| Key | Action |
| --- | --- |
| Space | Mark or unmark |
| `s` | Sync; in the dependency view, sync only the marked cards. `p` prune, `r` dry run |
| `R` | Refresh; `h` for a hard refresh |
| `P` | Sync policy: `a` auto-sync, `h` self-heal, `p` prune |
| `T` | Terminate the running operation |
| `D` | Delete the Application and its resources |
| `x` | Rolling restart of a Deployment, StatefulSet or DaemonSet |
| `X` | Delete a Pod so its workload replaces it |

Rollback is refused while auto-sync is on, as with `argocd app rollback`. Turn it off with `P` first.

## Permissions

With `--source kubeconfig`, the identity needs to list `applications`, `appprojects` and Argo CD's cluster Secrets. Anything else it cannot read is left out, and the rest still loads.

| To | Needs |
| --- | --- |
| See ReplicaSets, Jobs and Pods | `list` on them in the destination namespaces |
| See ApplicationSets | `list` on `applicationsets` |
| View logs | `get` on `pods/log` |
| View events | `list` on `events`, including the Argo CD namespace |
| Change sync policy, terminate, roll back | `get` and `patch` on `applications` |
| Restart a workload | `patch` on it |
| Delete a Pod | `delete` on `pods` |
| List namespaces in `:ns` | `list` on `namespaces` (optional; `:ns <name>` works without it) |

With `--source api`, the token needs the matching Argo CD RBAC: `get` and `sync` on Applications, `update` for sync policy, and `action/…/restart` and resource delete for `x` and `X`. Orphaned resources only appear when the AppProject monitors them.

A read-only identity for `--source kubeconfig`, with Argo CD in `argocd`. The built-in `view` ClusterRole covers Pods, logs, events, workloads and namespaces, so only Argo CD's own objects need a Role:

```yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: argoxd-viewer
  namespace: argocd
---
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: argoxd-viewer
  namespace: argocd
rules:
  - apiGroups: ["argoproj.io"]
    resources: ["applications", "appprojects", "applicationsets"]
    verbs: ["get", "list"]
  - apiGroups: [""]
    resources: ["secrets"]   # Argo CD's cluster Secrets, for :cluster
    verbs: ["list"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: argoxd-viewer
  namespace: argocd
subjects:
  - kind: ServiceAccount
    name: argoxd-viewer
    namespace: argocd
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: argoxd-viewer
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: argoxd-viewer
subjects:
  - kind: ServiceAccount
    name: argoxd-viewer
    namespace: argocd
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: view
```

Run argoxd with `--read-only` alongside it, so actions are refused up front instead of failing on RBAC. Kubernetes cannot limit `list` to labelled Secrets, so this Role can read every Secret in `argocd`. Where that matters, use `--source api` with a read-only token.

Requests that take longer than `requestTimeout` (15 seconds by default) are abandoned and reported as errors.

## Development

```sh
make test   # go vet and go test
make build  # dist/argoxd
make demo   # re-record demo/demo.gif; needs vhs
```

Pushing a tag such as `v0.4.1` publishes release binaries and the Homebrew cask with GoReleaser.

## License

[MIT](LICENSE)
