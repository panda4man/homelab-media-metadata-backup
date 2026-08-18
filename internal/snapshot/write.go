// Package snapshot writes inventory snapshots and their checksum sidecars
// to disk atomically: a snapshot is never written directly to its final
// filename, so a terminated container or host shutdown can never leave a
// partially written file that appears valid.
package snapshot

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/panda4man/homelab-media-metadata-backup/internal/inventory"
)

// WriteJSON marshals s and writes it atomically to the dated snapshot path
// under dir, using now's own location to determine the date. The snapshot
// directory is created if it does not already exist.
func WriteJSON(dir string, s inventory.Snapshot, now time.Time) (string, error) {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return "", fmt.Errorf("snapshot: marshaling: %w", err)
	}
	data = append(data, '\n')

	validate := func(b []byte) error {
		var check inventory.Snapshot
		return json.Unmarshal(b, &check)
	}
	return writeAtomic(dir, filenameFor(now), data, validate)
}

// WriteSHA256 computes the SHA-256 checksum of the file at jsonPath and
// writes it as a sidecar in the `sha256sum -c`-compatible format:
// "<64 hex chars>  <basename>\n".
func WriteSHA256(jsonPath string) (string, error) {
	data, err := os.ReadFile(jsonPath)
	if err != nil {
		return "", fmt.Errorf("snapshot: reading %s for checksum: %w", jsonPath, err)
	}
	sum := sha256.Sum256(data)
	line := fmt.Sprintf("%s  %s\n", hex.EncodeToString(sum[:]), filepath.Base(jsonPath))

	sidecarPath := SHA256Path(jsonPath)
	return writeAtomic(filepath.Dir(sidecarPath), filepath.Base(sidecarPath), []byte(line), nil)
}

// writeAtomic writes data to a temp file in the same directory as the
// final target, flushes and closes it, optionally validates the bytes,
// then atomically renames it into place. On any failure the temp file is
// removed and the target is left untouched.
func writeAtomic(dir, filename string, data []byte, validate func([]byte) error) (string, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("snapshot: creating directory %s: %w", dir, err)
	}

	target := filepath.Join(dir, filename)
	tmp := target + ".tmp"

	f, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return "", fmt.Errorf("snapshot: creating temp file: %w", err)
	}
	if _, err := f.Write(data); err != nil {
		f.Close()
		os.Remove(tmp)
		return "", fmt.Errorf("snapshot: writing temp file: %w", err)
	}
	if err := f.Sync(); err != nil {
		f.Close()
		os.Remove(tmp)
		return "", fmt.Errorf("snapshot: syncing temp file: %w", err)
	}
	if err := f.Close(); err != nil {
		os.Remove(tmp)
		return "", fmt.Errorf("snapshot: closing temp file: %w", err)
	}

	if validate != nil {
		if err := validate(data); err != nil {
			os.Remove(tmp)
			return "", fmt.Errorf("snapshot: validating written data: %w", err)
		}
	}

	if err := os.Rename(tmp, target); err != nil {
		os.Remove(tmp)
		return "", fmt.Errorf("snapshot: renaming into place: %w", err)
	}
	return target, nil
}
