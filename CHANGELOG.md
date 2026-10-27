# Changelog

All notable changes to argoxd are listed here. The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## [0.5.0] - 2026-10-26

### Added

- **Custom CA and client certificates.** `--ca-cert` trusts an internal certificate authority for `--source api`, and `--client-cert` with `--client-key` presents a client certificate to a server that asks for one. Use these instead of `--insecure`. Also `caCert`, `clientCert` and `clientKey` in the config file. An unreadable file is reported at startup.
- **Request timeout.** `--request-timeout` and `requestTimeout` set how long a request to Argo CD or Kubernetes may take (15 seconds by default), for installs with thousands of Applications.
- **Sorting.** `:sort name`, `:sort health` (worst first) and `:sort sync` (out of sync first) order the Applications; add `reverse` to flip them, or run `:sort` alone to restore Argo CD's order. Running the same sort again flips its direction. The list's title and the sorted column's header (`▲` or `▼`) show the active sort, and it survives `:ctx` and `:ns`. Set a default with `sort` and `sortReverse` in the config file.
- **Mouse wheel.** `--mouse` or `mouse: true` scrolls lists and viewers with the wheel. It is off by default, because capturing the mouse means the terminal needs Shift to select text.

### Fixed

- **Tab in the command bar.** After Tab filled in a suggestion for a command with an argument, such as `:sort `, `:health `, `:app ` or `:ctx `, the other suggestions disappeared and Tab did nothing more. Tab and the arrow keys now keep moving through the same list, and Shift+Tab goes back.

- **Resources as a table.** `w` in an Application's dependency view lists its resources as a table (kind, name, namespace, health, sync, and `✂ to prune` or `◌ orphaned`), indented under their owners, with the same search, filters, marks and actions as the cards. `resourceView: table` or `--resource-view table` starts there.
- **Fold cards.** `←` folds the selected card (and on a card with nothing to fold, goes to its owner), `→` unfolds it, and `z` folds or unfolds every card at once. A `▾` marks a card that can fold, `▸` one that is folded, and a folded card says how many resources it hides.
- **Start view.** `startView` or `--start-view` opens a view first: `proj`, `cluster`, `appset`, `pulse`, or `app store` for one project.
- **argocd CLI login.** With `--source api` and no server or token, argoxd reads them from `~/.config/argocd/config`, so `argocd login` once is enough. `--argocd-context` and `--argocd-config` choose another login or file. When the saved token has expired, the error says to run `argocd login`.

- **Themes.** `:theme <name>`, `theme:` in the config file or `--theme` choose `default`, `light`, `high-contrast`, `colorblind` (blue for healthy, vermillion for degraded), `nord`, `dracula` or `terminal` (your terminal's own colors). The change is immediate and covers every color, including diffs and the status glyphs. `:theme` alone lists them.
- **Custom themes.** Define themes under `themes:` in the config file, each starting from a built-in `base` and changing only the colors you list (17 names, as `#rrggbb`, `#rgb` or an ANSI number), then pick one with `theme:` or `:theme`. Mistakes are reported at startup with the theme and color named, including the YAML trap where an unquoted `#ff5500` is read as a comment.
- **k9s.** `:k9s` opens k9s on the selected resource's namespace and kind, with the kubeconfig context argoxd uses, and returns to argoxd when you quit it. `--read-only` blocks it, because k9s can change things. `k9sCommand` sets its path.
- **External diff viewer.** `x` in a diff opens it in the program set by `diffViewer`, such as `meld {left} {right}` or `code --diff {left} {right} --wait`, with the live and desired text as two temporary files that are removed afterwards.

- **Events pane.** `E` in an Application's dependency view opens a pane under the cards with the events of the selected card, or of the Application on its own card. It follows the cursor, reloads on every refresh, keeps the old events on screen until the new ones arrive, and shrinks to fit a short terminal. `[` and `]` scroll it through older and newer events, and so does the mouse wheel over it with `--mouse`; the title says how many events are off screen. `eventsPane: true` or `--events-pane` opens it by default. It needs `list` on `events`, like `e`.

### Changed

- **Dependency view hints.** The key hints are five steady columns, so an optional hint no longer shifts the others. Their padding counted bytes instead of width, which a two-column key such as `←→` threw off.

## [0.4.2] - 2026-10-26

### Added

- **Update notice.** argoxd checks GitHub once a day, in the background, for a newer release and shows an orange `↑ <version> available` badge at the right of the breadcrumb row. It never delays startup and says nothing when it cannot connect. Turn it off with `checkUpdates: false` in the config file or `ARGOXD_NO_UPDATE_CHECK=1`. Development builds and `--demo` never check.

### Fixed

- **`:ns` on a slow cluster.** Listing namespaces for the picker, or checking one you typed, no longer freezes the screen for up to 15 seconds. The picker opens at once and fills in when the cluster answers.

## [0.4.1] - 2026-10-22

### Added

- **Runtime namespaces.** `:ns` opens a namespace picker; `:ns <name>` switches the Argo CD namespace and reloads, without restarting.
- **Short flags.** `-n` (`--namespace`), `-c` (`--context`), `-s` (`--server`), `-r` (`--refresh`) and `-k` (`--insecure`). `--help` lists each beside its long form, e.g. `-n, --namespace`.
- **Version.** `--version` prints the argoxd version and exits.

### Changed

- **Marking with Space.** Space marks or unmarks the selected row or card and stays on it instead of moving down.

### Fixed

- **Marks after `:ctx`.** Switching the kubeconfig context now clears marked Applications. Before, an Application with the same name in the new context stayed marked, so `s`, `R`, `D` or `P` could act on it.

## [0.4.0] - 2026-10-15

### Added

- **Clipboard and browser.** `y` copies the selected item or open viewer text; `o` opens the selected Application in the Argo CD web UI when using `--source api`.
- **Runtime contexts.** `:ctx` opens a filterable context picker for the active kubeconfig; `:ctx <name>` switches directly and reconnects.
- **Sync-window blocks.** Applications that Argo CD reports as blocked by a sync window show `⊘` beside their existing condition marker.
- **AppProject inspection.** On `:proj`, `Y` opens the selected AppProject manifest without changing Enter's existing Application-filter workflow.
- **Application inspection.** `Y` shows an Application custom resource from the Applications list or its root dependency card. `d` there shows the concatenated diff of drifted resources.
- **Refresh modes.** `R` requests a normal Argo CD refresh by default; toggle `h` in its confirmation for a hard refresh that bypasses the manifest cache.
- **Operation termination.** `T` terminates a selected Application's active operation after confirmation.
- **Read-only mode.** `--read-only` and `readOnly: true` disable mutating actions and show a `[READ-ONLY]` header badge. `h` still opens an Application's history; rolling back from it is blocked.

### Changed

- **Keymap: `y` now yanks; `Y` now shows YAML.** This intentionally changes the former lowercase `y` YAML shortcut. Use uppercase `Y` for Application and resource manifests; lowercase `y` copies the selected item or the open viewer text to the clipboard. In viewers, both `y` and `Y` yank, and `v` starts visual line selection to copy a chosen range of lines. Marking applications or dependency cards with Space also allows `y` to yank all marked items.

### Permissions

- Terminating an Application operation with `--source kubeconfig` needs `patch` on `applications.argoproj.io`.

## [0.3.0] - 2026-10-08

### Added

- **Conditions.** An Application with conditions, such as a `ComparisonError` or `SyncError`, has a `⚠` on the list, and the dependency view shows the first three, up to two lines each, in a summary box below the cards.
- **Sync details.** `i` shows how an Application's last sync went: its phase, message, revision and times, and the result for each resource or hook, with the ones that did not sync first.
- **Sync policy.** `P` turns auto-sync, self-heal and prune on or off for the marked or selected Applications. Other sync options are left as they are.
- **Resources to prune and orphans.** Dependency cards show `✂ to prune` for a resource that a sync with prune would delete and `◌ orphaned` for one that no Application manages. The summary box counts both.
- **App of apps.** Enter on a card that is an Application opens its dependencies, Esc returns to the parent, and the breadcrumbs show the path.
- **ApplicationSets.** `:appset` lists them with their generators, how many Applications each generated and any problem it reports. Enter shows the Applications it generated.
- **Pulse.** `:pulse` (or `:overview`) shows Applications by health and sync status as bars, how many use auto-sync, self-heal and prune, and the Applications that need attention with the reasons. Narrow or short terminals get a text summary.

### Changed

- The Homebrew package is now published as a cask, because GoReleaser deprecated formulas. A hook removes the macOS quarantine flag after installing, since the binary is not signed or notarized.
- Esc on the Applications that Enter opened from a project or an ApplicationSet now returns to that list, with the same row selected. It first clears a search, as before. Esc did nothing there before.

### Fixed

- With `--source kubeconfig`, syncing, syncing selected resources and rolling back now apply the Application's sync options, such as `CreateNamespace=true` or `ServerSideApply=true`, as Argo CD's own sync does. They were ignored before, so a sync of an Application that relies on `CreateNamespace=true` failed with "namespace not found".
- On the card of a child Application (an app of apps), Space now marks it so it can be synced selectively, `y` and `d` show its manifest and diff, and `e` shows its own events instead of the parent's.

### Permissions

- ApplicationSets need `list` on `applicationsets.argoproj.io` (or access to `/api/v1/applicationsets` with `--source api`). Without it, or on an Argo CD that has none, they are left out and the rest still loads.
- Changing the sync policy needs `patch` on `applications.argoproj.io`, or permission to update Applications with `--source api`.
- With `--source api`, resources to prune are only marked when the token can read the Application, and orphans only appear when the AppProject monitors orphaned resources. `--source kubeconfig` shows resources to prune but not orphans.

## [0.2.0] - 2026-10-06

### Added

- **Events view.** Press `e` on an Application, or on a resource card in the dependency view, to see its Kubernetes events. A workload's events include those of the ReplicaSets and Pods under it. Search them with `/`.
- **Marking.** Space marks Applications on the list and resource cards in the dependency view. `s`, `R`, `D`, `x` and `X` act on everything marked. In the dependency view `s` syncs only the marked resources (a selective sync). Esc clears the marks.
- **Deployment history and rollback.** `h` lists an Application's deployments, newest first, and Enter rolls back to one, with prune and dry-run options.
- **Following logs.** `f` follows a Pod's log. On a Deployment, StatefulSet, DaemonSet, ReplicaSet or Job, `l` follows the logs of all its Pods, and opening a Pod's log from its workload filters to that Pod.
- **Log controls.** `c` chooses the container, `p` chooses the Pod, and `/` keeps only the lines that contain the text.
- **Restart and delete from the dependency view.** `x` restarts a Deployment, StatefulSet or DaemonSet with a rolling restart. `X` deletes a Pod so its workload replaces it.

### Changed

- **Every action now asks first.** A confirmation replaces the view and lists each Application or resource the action will touch. Enter or `y` confirms, Esc or `n` cancels.
- **Hard refresh (`R`) now asks for confirmation.** It used to run immediately.
- **Several marked items run one after another.** If some fail, the status line reports how many and why, and the rest still run. A marked card that cannot take the action (`x` on a Pod, `X` on a Deployment) refuses the whole selection instead of being skipped.

### Fixed

- Rollback is refused while the Application has another operation in progress, and when it has no history.

### Permissions

- Viewing events needs `list` on `events` in the namespaces involved, and in the Argo CD namespace for an Application's own events (`--source kubeconfig`).
- Restarting a workload needs `patch` on it, deleting a Pod needs `delete` on `pods`, and viewing a Pod's log needs `pods/log`.
- With `--source api`, the token needs Argo CD's `action/…/restart` and resource delete permissions to restart and delete, and permission to sync for selective syncs.

## [0.1.0] - 2026-10-05

Initial release: browse Applications, Projects and Clusters, open an Application's dependency cards, view live YAML, diffs and logs, and sync, hard refresh or delete an Application, from the Kubernetes API or the Argo CD API.
