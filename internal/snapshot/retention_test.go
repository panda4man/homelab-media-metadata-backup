package snapshot

import (
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"
)

// seedSnapshot writes a full snapshot (JSON + sha256 + both CSVs) for the
// given date and returns the base filename (no extension) it used.
func seedSnapshot(t *testing.T, dir string, date time.Time) string {
	t.Helper()
	jsonPath, err := WriteJSON(dir, testSnapshot(), date)
	if err != nil {
		t.Fatalf("WriteJSON() error = %v", err)
	}
	if _, err := WriteSHA256(jsonPath); err != nil {
		t.Fatalf("WriteSHA256() error = %v", err)
	}
	if _, err := WriteMoviesCSV(dir, csvFixtureMovies(), date); err != nil {
		t.Fatalf("WriteMoviesCSV() error = %v", err)
	}
	if _, err := WriteEpisodesCSV(dir, csvFixtureSeries(), date); err != nil {
		t.Fatalf("WriteEpisodesCSV() error = %v", err)
	}
	return filenameFor(date)[:len(filenameFor(date))-len(".json")]
}

func listDir(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir() error = %v", err)
	}
	names := make([]string, len(entries))
	for i, e := range entries {
		names[i] = e.Name()
	}
	sort.Strings(names)
	return names
}

func dateFor(dayOffset int) time.Time {
	return time.Date(2026, 8, 1+dayOffset, 0, 0, 0, 0, time.UTC)
}

func TestPrune_MoreThanKeep_DeletesOldest(t *testing.T) {
	dir := t.TempDir()
	for i := 0; i < 7; i++ {
		seedSnapshot(t, dir, dateFor(i))
	}

	removed, err := Prune(dir, 5)
	if err != nil {
		t.Fatalf("Prune() error = %v", err)
	}
	// 2 oldest snapshots * 4 files each (json, sha256, movies csv, episodes csv)
	if len(removed) != 8 {
		t.Fatalf("removed = %v, want 8 files (2 oldest snapshots)", removed)
	}

	remaining := listDir(t, dir)
	for i := 0; i < 2; i++ { // the two oldest (day offsets 0, 1) must be gone
		base := filenameFor(dateFor(i))
		for _, n := range remaining {
			if n == base {
				t.Errorf("oldest snapshot %s still present", base)
			}
		}
	}
	// The 5 newest (offsets 2-6) must remain.
	for i := 2; i < 7; i++ {
		found := false
		want := filenameFor(dateFor(i))
		for _, n := range remaining {
			if n == want {
				found = true
			}
		}
		if !found {
			t.Errorf("snapshot for offset %d missing after prune, want it kept", i)
		}
	}
}

func TestPrune_FewerThanKeep_DeletesNothing(t *testing.T) {
	dir := t.TempDir()
	for i := 0; i < 3; i++ {
		seedSnapshot(t, dir, dateFor(i))
	}

	removed, err := Prune(dir, 5)
	if err != nil {
		t.Fatalf("Prune() error = %v", err)
	}
	if len(removed) != 0 {
		t.Errorf("removed = %v, want none (3 snapshots, keep 5)", removed)
	}
}

func TestPrune_ExactlyKeep_DeletesNothing(t *testing.T) {
	dir := t.TempDir()
	for i := 0; i < 5; i++ {
		seedSnapshot(t, dir, dateFor(i))
	}

	removed, err := Prune(dir, 5)
	if err != nil {
		t.Fatalf("Prune() error = %v", err)
	}
	if len(removed) != 0 {
		t.Errorf("removed = %v, want none (exactly 5, keep 5)", removed)
	}
}

func TestPrune_PointerFilesNeverDeleted(t *testing.T) {
	dir := t.TempDir()
	var lastJSON string
	for i := 0; i < 7; i++ {
		date := dateFor(i)
		seedSnapshot(t, dir, date)
		lastJSON = JSONPath(dir, date)
	}
	if err := UpdateLatest(dir, lastJSON); err != nil {
		t.Fatalf("UpdateLatest() error = %v", err)
	}
	if err := UpdateLastKnownGood(dir, lastJSON); err != nil {
		t.Fatalf("UpdateLastKnownGood() error = %v", err)
	}

	if _, err := Prune(dir, 5); err != nil {
		t.Fatalf("Prune() error = %v", err)
	}

	for _, name := range []string{"latest.json", "last-known-good.json"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Errorf("%s missing after Prune(), want it never a candidate: %v", name, err)
		}
	}
}

func TestPrune_LastKnownGoodTarget_PreservedOutsideRetentionWindow(t *testing.T) {
	dir := t.TempDir()
	oldest := dateFor(0)
	oldestJSON, err := WriteJSON(dir, testSnapshot(), oldest)
	if err != nil {
		t.Fatalf("WriteJSON() error = %v", err)
	}
	if err := UpdateLastKnownGood(dir, oldestJSON); err != nil {
		t.Fatalf("UpdateLastKnownGood() error = %v", err)
	}
	// 6 more, newer snapshots - pushes the oldest outside a keep-5 window.
	for i := 1; i <= 6; i++ {
		seedSnapshot(t, dir, dateFor(i))
	}

	if _, err := Prune(dir, 5); err != nil {
		t.Fatalf("Prune() error = %v", err)
	}

	oldestBase := filenameFor(oldest)
	for _, n := range listDir(t, dir) {
		if n == oldestBase {
			return // found - preserved
		}
	}
	t.Errorf("the dated snapshot last-known-good.json points to was pruned despite being outside the retention window")
}

func TestPrune_SortsByFilenameDateNotMtime(t *testing.T) {
	dir := t.TempDir()
	// Write oldest-dated snapshot LAST so its mtime is newest, proving Prune
	// must sort by the date in the filename, not by mtime.
	for i := 6; i >= 0; i-- {
		seedSnapshot(t, dir, dateFor(i))
	}

	removed, err := Prune(dir, 5)
	if err != nil {
		t.Fatalf("Prune() error = %v", err)
	}
	if len(removed) != 8 {
		t.Fatalf("removed = %v, want 8 (still the 2 oldest by filename date)", removed)
	}
	remaining := listDir(t, dir)
	for i := 0; i < 2; i++ {
		base := filenameFor(dateFor(i))
		for _, n := range remaining {
			if n == base {
				t.Errorf("oldest-by-filename-date snapshot %s still present", base)
			}
		}
	}
}

func TestPrune_UnrelatedFilesNeverDeleted(t *testing.T) {
	dir := t.TempDir()
	for i := 0; i < 7; i++ {
		seedSnapshot(t, dir, dateFor(i))
	}
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("hi"), 0644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "media-inventory-2099-01-01.json.tmp"), []byte("stray"), 0644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	if _, err := Prune(dir, 5); err != nil {
		t.Fatalf("Prune() error = %v", err)
	}

	remaining := listDir(t, dir)
	for _, want := range []string{"README.md", "media-inventory-2099-01-01.json.tmp"} {
		found := false
		for _, n := range remaining {
			if n == want {
				found = true
			}
		}
		if !found {
			t.Errorf("%s was deleted, want unrelated files left alone", want)
		}
	}
}

func TestPrune_KeepNotPositive_ValidationErrorDeletesNothing(t *testing.T) {
	dir := t.TempDir()
	for i := 0; i < 7; i++ {
		seedSnapshot(t, dir, dateFor(i))
	}
	before := listDir(t, dir)

	for _, keep := range []int{0, -1} {
		_, err := Prune(dir, keep)
		if err == nil {
			t.Errorf("Prune(dir, %d) error = nil, want validation error", keep)
		}
	}

	after := listDir(t, dir)
	if len(before) != len(after) {
		t.Errorf("directory changed after invalid Prune() calls: before=%v after=%v", before, after)
	}
}

func TestPrune_EmptyDirectory_NoOpNoError(t *testing.T) {
	dir := t.TempDir()
	removed, err := Prune(dir, 5)
	if err != nil {
		t.Fatalf("Prune() error = %v, want nil", err)
	}
	if len(removed) != 0 {
		t.Errorf("removed = %v, want none", removed)
	}
}

func TestPrune_DeleteFailureOnOneFile_ErrorButOtherDeletionsStillAttempted(t *testing.T) {
	dir := t.TempDir()
	for i := 0; i < 7; i++ {
		seedSnapshot(t, dir, dateFor(i))
	}

	// Sabotage one sidecar of the OLDEST snapshot: replace the movies CSV
	// with a non-empty directory, so os.Remove on it fails reliably.
	oldestJSONName := filenameFor(dateFor(0))
	oldestBase := oldestJSONName[:len(oldestJSONName)-len(".json")]
	sabotaged := filepath.Join(dir, oldestBase+"-movies.csv")
	if err := os.Remove(sabotaged); err != nil {
		t.Fatalf("Remove() error = %v", err)
	}
	if err := os.MkdirAll(filepath.Join(sabotaged, "inner"), 0755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(sabotaged, "inner", "f.txt"), []byte("x"), 0644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	removed, err := Prune(dir, 5)
	if err == nil {
		t.Fatal("Prune() error = nil, want an error from the sabotaged file")
	}

	// The other 3 sidecars of the sabotaged (oldest) snapshot, plus all 4
	// of the second-oldest snapshot, should still have been removed.
	wantMinRemoved := 3 + 4
	if len(removed) < wantMinRemoved {
		t.Errorf("removed = %v (%d files), want at least %d - other deletions must still be attempted", removed, len(removed), wantMinRemoved)
	}

	secondOldestBase := filenameFor(dateFor(1))
	for _, n := range listDir(t, dir) {
		if n == secondOldestBase {
			t.Errorf("second-oldest snapshot %s still present, want it pruned despite the first failure", secondOldestBase)
		}
	}
}

func TestPrune_ReturnedRemovedList_ExactlyMatchesDeletedFiles(t *testing.T) {
	dir := t.TempDir()
	var wantRemoved []string
	for i := 0; i < 2; i++ {
		base := filenameFor(dateFor(i))
		seedSnapshot(t, dir, dateFor(i))
		wantRemoved = append(wantRemoved,
			filepath.Join(dir, base),
			filepath.Join(dir, base+".sha256"),
			filepath.Join(dir, base[:len(base)-len(".json")]+"-movies.csv"),
			filepath.Join(dir, base[:len(base)-len(".json")]+"-episodes.csv"),
		)
	}
	for i := 2; i < 7; i++ {
		seedSnapshot(t, dir, dateFor(i))
	}
	sort.Strings(wantRemoved)

	removed, err := Prune(dir, 5)
	if err != nil {
		t.Fatalf("Prune() error = %v", err)
	}
	sort.Strings(removed)

	if len(removed) != len(wantRemoved) {
		t.Fatalf("removed = %v, want %v", removed, wantRemoved)
	}
	for i := range removed {
		if removed[i] != wantRemoved[i] {
			t.Errorf("removed[%d] = %q, want %q", i, removed[i], wantRemoved[i])
		}
	}
}
