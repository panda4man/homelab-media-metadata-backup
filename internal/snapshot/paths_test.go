package snapshot

import (
	"testing"
	"time"
)

func TestFilenameFor_FormatsDate(t *testing.T) {
	tests := []struct {
		name string
		when time.Time
		want string
	}{
		{"ordinary date", time.Date(2026, 8, 16, 23, 0, 0, 0, time.UTC), "media-inventory-2026-08-16.json"},
		{"leap day", time.Date(2028, 2, 29, 0, 0, 0, 0, time.UTC), "media-inventory-2028-02-29.json"},
		{"year boundary", time.Date(2025, 12, 31, 23, 59, 59, 0, time.UTC), "media-inventory-2025-12-31.json"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := filenameFor(tt.when); got != tt.want {
				t.Errorf("filenameFor(%v) = %q, want %q", tt.when, got, tt.want)
			}
		})
	}
}

func TestPathHelpers(t *testing.T) {
	dir := "/data/snapshots"

	if got, want := LatestPath(dir), "/data/snapshots/latest.json"; got != want {
		t.Errorf("LatestPath() = %q, want %q", got, want)
	}
	if got, want := LastKnownGoodPath(dir), "/data/snapshots/last-known-good.json"; got != want {
		t.Errorf("LastKnownGoodPath() = %q, want %q", got, want)
	}
	if got, want := SHA256Path("/data/snapshots/media-inventory-2026-08-16.json"), "/data/snapshots/media-inventory-2026-08-16.json.sha256"; got != want {
		t.Errorf("SHA256Path() = %q, want %q", got, want)
	}
	if got, want := JSONPath(dir, time.Date(2026, 8, 16, 0, 0, 0, 0, time.UTC)), "/data/snapshots/media-inventory-2026-08-16.json"; got != want {
		t.Errorf("JSONPath() = %q, want %q", got, want)
	}
}
