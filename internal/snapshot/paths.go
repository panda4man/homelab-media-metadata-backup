package snapshot

import (
	"fmt"
	"path/filepath"
	"time"
)

func filenameFor(now time.Time) string {
	return fmt.Sprintf("media-inventory-%s.json", now.Format("2006-01-02"))
}

// JSONPath returns the dated snapshot path for the given time, using that
// time's own location - callers control the timezone by what they pass in.
func JSONPath(dir string, now time.Time) string {
	return filepath.Join(dir, filenameFor(now))
}

// LatestPath is the pointer updated after every valid or warning run.
func LatestPath(dir string) string {
	return filepath.Join(dir, "latest.json")
}

// LastKnownGoodPath is the pointer updated only after a valid run.
func LastKnownGoodPath(dir string) string {
	return filepath.Join(dir, "last-known-good.json")
}

// SHA256Path returns the checksum sidecar path for a given snapshot path.
func SHA256Path(jsonPath string) string {
	return jsonPath + ".sha256"
}
