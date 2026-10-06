package argocd

import (
	"slices"

	"github.com/awcodify/argoxd/internal/explorer"
)

// errAutoSyncEnabled is why Argo CD refuses a rollback: the next auto-sync
// would undo it.
const errAutoSyncEnabled = "auto-sync is enabled; disable it to roll back"

// newestFirst orders history entries from the latest deployment to the oldest.
func newestFirst(entries []explorer.HistoryEntry) []explorer.HistoryEntry {
	slices.SortStableFunc(entries, func(a, b explorer.HistoryEntry) int {
		switch {
		case a.ID > b.ID:
			return -1
		case a.ID < b.ID:
			return 1
		}
		return 0
	})
	return entries
}

// historyRevision picks the revision to show for a deployment: the revision of
// a single-source Application, or the first one of a multi-source Application.
func historyRevision(revision string, revisions []string) string {
	if revision == "" && len(revisions) > 0 {
		return revisions[0]
	}
	return revision
}
