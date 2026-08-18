package snapshot

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/panda4man/homelab-media-metadata-backup/internal/inventory"
)

func writeRawJSON(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("WriteFile(%q): %v", path, err)
	}
}

func TestLoadPrevious_LastKnownGoodPresent_Loaded(t *testing.T) {
	dir := t.TempDir()
	want := testSnapshot()
	if _, err := writeAtomic(dir, "last-known-good.json", mustMarshal(t, want), nil); err != nil {
		t.Fatalf("writeAtomic() error = %v", err)
	}

	got, found, err := LoadPrevious(dir)
	if err != nil {
		t.Fatalf("LoadPrevious() error = %v", err)
	}
	if !found {
		t.Fatal("found = false, want true")
	}
	if got.Hostname != want.Hostname {
		t.Errorf("Hostname = %q, want %q", got.Hostname, want.Hostname)
	}
}

func TestLoadPrevious_NoLastKnownGood_FallsBackToNewestDated(t *testing.T) {
	dir := t.TempDir()
	older := testSnapshot()
	newer := testSnapshot()
	newer.Hostname = "newer-host"

	if _, err := WriteJSON(dir, older, time.Date(2026, 8, 9, 0, 0, 0, 0, time.UTC)); err != nil {
		t.Fatalf("WriteJSON() error = %v", err)
	}
	if _, err := WriteJSON(dir, newer, time.Date(2026, 8, 16, 0, 0, 0, 0, time.UTC)); err != nil {
		t.Fatalf("WriteJSON() error = %v", err)
	}

	got, found, err := LoadPrevious(dir)
	if err != nil {
		t.Fatalf("LoadPrevious() error = %v", err)
	}
	if !found {
		t.Fatal("found = false, want true")
	}
	if got.Hostname != "newer-host" {
		t.Errorf("Hostname = %q, want newer-host (the newest dated snapshot)", got.Hostname)
	}
}

func TestLoadPrevious_NoSnapshotsAtAll_NotFoundNoError(t *testing.T) {
	dir := t.TempDir()

	_, found, err := LoadPrevious(dir)
	if err != nil {
		t.Fatalf("LoadPrevious() error = %v, want nil (first run is legal)", err)
	}
	if found {
		t.Error("found = true, want false")
	}
}

func TestLoadPrevious_CorruptLastKnownGood_FallsBackToNewestDated(t *testing.T) {
	dir := t.TempDir()
	writeRawJSON(t, LastKnownGoodPath(dir), "{not valid json")

	fallback := testSnapshot()
	fallback.Hostname = "fallback-host"
	if _, err := WriteJSON(dir, fallback, time.Date(2026, 8, 9, 0, 0, 0, 0, time.UTC)); err != nil {
		t.Fatalf("WriteJSON() error = %v", err)
	}

	got, found, err := LoadPrevious(dir)
	if err != nil {
		t.Fatalf("LoadPrevious() error = %v, want nil (should fall back)", err)
	}
	if !found || got.Hostname != "fallback-host" {
		t.Errorf("got = %+v, found = %v, want the fallback dated snapshot", got, found)
	}
}

func TestLoadPrevious_CorruptLastKnownGoodAndCorruptFallback_ReturnsError(t *testing.T) {
	dir := t.TempDir()
	writeRawJSON(t, LastKnownGoodPath(dir), "{not valid json")
	writeRawJSON(t, JSONPath(dir, time.Date(2026, 8, 9, 0, 0, 0, 0, time.UTC)), "{also not valid")

	_, _, err := LoadPrevious(dir)
	if err == nil {
		t.Fatal("LoadPrevious() error = nil, want an error - both files are corrupt")
	}
}

func TestLoadPrevious_TmpFileIgnoredByNewestScan(t *testing.T) {
	dir := t.TempDir()
	real := testSnapshot()
	real.Hostname = "real-host"
	if _, err := WriteJSON(dir, real, time.Date(2026, 8, 9, 0, 0, 0, 0, time.UTC)); err != nil {
		t.Fatalf("WriteJSON() error = %v", err)
	}
	// A stray .tmp with a "newer" date must never be picked up.
	writeRawJSON(t, filepath.Join(dir, "media-inventory-2099-01-01.json.tmp"), "garbage")

	got, found, err := LoadPrevious(dir)
	if err != nil {
		t.Fatalf("LoadPrevious() error = %v", err)
	}
	if !found || got.Hostname != "real-host" {
		t.Errorf("got = %+v, found = %v, want the real dated snapshot, not the .tmp", got, found)
	}
}

func TestLoadPrevious_UnsupportedSchemaVersion_ErrorSurfacedNotSwallowed(t *testing.T) {
	dir := t.TempDir()
	// No last-known-good.json; the only dated snapshot is schema_version 2.
	writeRawJSON(t, JSONPath(dir, time.Date(2026, 8, 9, 0, 0, 0, 0, time.UTC)),
		`{"schema_version":2,"generated_at":"2026-08-09T00:00:00Z","hostname":"h","summary":{},"movies":[],"series":[]}`)

	_, found, err := LoadPrevious(dir)
	if err == nil {
		t.Fatal("LoadPrevious() error = nil, want ErrUnsupportedSchema")
	}
	if !errors.Is(err, inventory.ErrUnsupportedSchema) {
		t.Errorf("err = %v, want it to wrap inventory.ErrUnsupportedSchema", err)
	}
	if found {
		t.Error("found = true, want false - a schema mismatch must not be silently treated as empty")
	}
}

func mustMarshal(t *testing.T, s inventory.Snapshot) []byte {
	t.Helper()
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		t.Fatalf("marshaling snapshot: %v", err)
	}
	return data
}
