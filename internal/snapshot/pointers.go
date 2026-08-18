package snapshot

import (
	"fmt"
	"os"
)

// UpdateLatest copies the dated snapshot at snapshotPath to latest.json,
// atomically. Callers update latest.json for both valid and warning runs.
func UpdateLatest(dir, snapshotPath string) error {
	return copyPointer(dir, "latest.json", snapshotPath)
}

// UpdateLastKnownGood copies the dated snapshot at snapshotPath to
// last-known-good.json, atomically. Callers update this only for valid
// runs - a warning or failed run must never make its snapshot the new
// disaster-recovery anchor.
func UpdateLastKnownGood(dir, snapshotPath string) error {
	return copyPointer(dir, "last-known-good.json", snapshotPath)
}

func copyPointer(dir, filename, snapshotPath string) error {
	data, err := os.ReadFile(snapshotPath)
	if err != nil {
		return fmt.Errorf("snapshot: reading %s to update %s: %w", snapshotPath, filename, err)
	}
	_, err = writeAtomic(dir, filename, data, nil)
	return err
}
