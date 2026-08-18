package snapshot

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/panda4man/homelab-media-metadata-backup/internal/inventory"
)

// LoadPrevious loads the previous successful snapshot: last-known-good.json
// if present and parseable, otherwise the newest dated snapshot in dir.
// found is false only when there is genuinely nothing to load yet (a
// legitimate first run) - that case returns no error. A snapshot that
// exists but fails to parse (corruption, or a schema_version this build
// does not understand) is a real problem: if a fallback dated snapshot can
// be loaded instead, LoadPrevious falls back and still succeeds; if not,
// the parse error is returned rather than being swallowed as "not found".
func LoadPrevious(dir string) (inventory.Snapshot, bool, error) {
	snap, lkgErr := loadSnapshotFile(LastKnownGoodPath(dir))
	if lkgErr == nil {
		return snap, true, nil
	}

	newest, found, err := findNewestDatedSnapshot(dir)
	if err != nil {
		return inventory.Snapshot{}, false, err
	}
	if !found {
		if !os.IsNotExist(lkgErr) {
			// last-known-good.json existed but was unreadable, and there is
			// no dated fallback either - surface the real problem.
			return inventory.Snapshot{}, false, lkgErr
		}
		return inventory.Snapshot{}, false, nil
	}

	fallbackSnap, err := loadSnapshotFile(newest)
	if err != nil {
		return inventory.Snapshot{}, false, err
	}
	return fallbackSnap, true, nil
}

func loadSnapshotFile(path string) (inventory.Snapshot, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return inventory.Snapshot{}, err
	}
	var s inventory.Snapshot
	if err := json.Unmarshal(data, &s); err != nil {
		return inventory.Snapshot{}, fmt.Errorf("snapshot: parsing %s: %w", path, err)
	}
	return s, nil
}

// findNewestDatedSnapshot scans dir for files named
// media-inventory-YYYY-MM-DD.json and returns the newest by date. Pointer
// files (latest.json, last-known-good.json), CSV exports, .sha256
// sidecars, and .tmp files never match this pattern and are ignored.
func findNewestDatedSnapshot(dir string) (path string, found bool, err error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return "", false, nil
		}
		return "", false, fmt.Errorf("snapshot: reading directory %s: %w", dir, err)
	}

	var newestDate time.Time
	var newestName string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		d, ok := parseSnapshotDate(name)
		if !ok {
			continue
		}
		if newestName == "" || d.After(newestDate) {
			newestDate = d
			newestName = name
		}
	}
	if newestName == "" {
		return "", false, nil
	}
	return filepath.Join(dir, newestName), true, nil
}

// parseSnapshotDate reports the date encoded in a dated snapshot filename
// (media-inventory-YYYY-MM-DD.json), or ok=false for anything else -
// pointer files, CSV exports, .sha256 sidecars, .tmp files, and any
// unrelated file all fail this check and are left alone by every caller.
func parseSnapshotDate(name string) (time.Time, bool) {
	if !strings.HasPrefix(name, "media-inventory-") || !strings.HasSuffix(name, ".json") {
		return time.Time{}, false
	}
	datePart := strings.TrimSuffix(strings.TrimPrefix(name, "media-inventory-"), ".json")
	d, err := time.Parse("2006-01-02", datePart)
	if err != nil {
		return time.Time{}, false
	}
	return d, true
}
