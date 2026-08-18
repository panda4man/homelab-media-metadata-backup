package snapshot

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// sidecarSuffixes are every file that belongs to one dated snapshot,
// pruned together with it.
var sidecarSuffixes = []string{".json", ".json.sha256", "-movies.csv", "-episodes.csv"}

type datedSnapshot struct {
	date time.Time
	base string
}

// Prune deletes dated snapshots beyond the keep-most-recent-N retention
// window, along with each one's sidecars (.sha256, CSV exports).
// latest.json and last-known-good.json are never candidates - they don't
// match the dated-snapshot filename pattern at all. The dated snapshot
// that last-known-good.json currently points to (identified by content,
// since that is what UpdateLastKnownGood actually copies) is preserved
// even if it would otherwise fall outside the window, so the
// disaster-recovery anchor is never pruned out from under itself.
//
// Prune attempts every deletion even if one fails, so a single stuck file
// never blocks pruning of everything else; removed reports exactly what
// was actually deleted, and a non-nil error is returned if anything failed.
func Prune(dir string, keep int) (removed []string, err error) {
	if keep <= 0 {
		return nil, fmt.Errorf("snapshot: keep must be >= 1, got %d", keep)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("snapshot: reading directory %s: %w", dir, err)
	}

	var snapshots []datedSnapshot
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		d, ok := parseSnapshotDate(name)
		if !ok {
			continue
		}
		snapshots = append(snapshots, datedSnapshot{date: d, base: name[:len(name)-len(".json")]})
	}
	if len(snapshots) <= keep {
		return nil, nil
	}

	sort.Slice(snapshots, func(i, j int) bool { return snapshots[i].date.After(snapshots[j].date) })

	protectedBase := lastKnownGoodBase(dir, snapshots)

	toDelete := make([]datedSnapshot, 0, len(snapshots))
	kept := 0
	for _, s := range snapshots {
		switch {
		case s.base == protectedBase:
			continue
		case kept < keep:
			kept++
		default:
			toDelete = append(toDelete, s)
		}
	}

	var errs []error
	for _, s := range toDelete {
		for _, suffix := range sidecarSuffixes {
			p := filepath.Join(dir, s.base+suffix)
			if _, statErr := os.Stat(p); statErr != nil {
				continue
			}
			if err := os.Remove(p); err != nil {
				errs = append(errs, fmt.Errorf("snapshot: removing %s: %w", p, err))
				continue
			}
			removed = append(removed, p)
		}
	}

	sort.Strings(removed)
	if len(errs) > 0 {
		return removed, errors.Join(errs...)
	}
	return removed, nil
}

// lastKnownGoodBase identifies which dated snapshot's content matches
// last-known-good.json (that is what UpdateLastKnownGood copies), so
// retention can protect it. Returns "" if there is no last-known-good.json
// or its content does not match any dated snapshot on disk.
func lastKnownGoodBase(dir string, snapshots []datedSnapshot) string {
	lkgData, err := os.ReadFile(LastKnownGoodPath(dir))
	if err != nil {
		return ""
	}
	for _, s := range snapshots {
		data, err := os.ReadFile(filepath.Join(dir, s.base+".json"))
		if err != nil {
			continue
		}
		if bytes.Equal(data, lkgData) {
			return s.base
		}
	}
	return ""
}
