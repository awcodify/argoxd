# Changelog

All notable changes to argoxd are listed here. The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

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
