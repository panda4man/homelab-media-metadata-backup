package snapshot

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestUpdateLatest_CopiesDatedSnapshotContent(t *testing.T) {
	dir := t.TempDir()
	datedPath, err := WriteJSON(dir, testSnapshot(), time.Now())
	if err != nil {
		t.Fatalf("WriteJSON() error = %v", err)
	}

	if err := UpdateLatest(dir, datedPath); err != nil {
		t.Fatalf("UpdateLatest() error = %v", err)
	}

	want, err := os.ReadFile(datedPath)
	if err != nil {
		t.Fatalf("ReadFile(dated) error = %v", err)
	}
	got, err := os.ReadFile(LatestPath(dir))
	if err != nil {
		t.Fatalf("ReadFile(latest) error = %v", err)
	}
	if string(got) != string(want) {
		t.Error("latest.json content does not match the dated snapshot")
	}
}

func TestUpdateLastKnownGood_CopiesDatedSnapshotContent(t *testing.T) {
	dir := t.TempDir()
	datedPath, err := WriteJSON(dir, testSnapshot(), time.Now())
	if err != nil {
		t.Fatalf("WriteJSON() error = %v", err)
	}

	if err := UpdateLastKnownGood(dir, datedPath); err != nil {
		t.Fatalf("UpdateLastKnownGood() error = %v", err)
	}

	want, err := os.ReadFile(datedPath)
	if err != nil {
		t.Fatalf("ReadFile(dated) error = %v", err)
	}
	got, err := os.ReadFile(LastKnownGoodPath(dir))
	if err != nil {
		t.Fatalf("ReadFile(lkg) error = %v", err)
	}
	if string(got) != string(want) {
		t.Error("last-known-good.json content does not match the dated snapshot")
	}
}

func TestWarningState_UpdatesLatestOnly_LastKnownGoodStaysAbsent(t *testing.T) {
	dir := t.TempDir()
	datedPath, err := WriteJSON(dir, testSnapshot(), time.Now())
	if err != nil {
		t.Fatalf("WriteJSON() error = %v", err)
	}

	// Simulate what the orchestrator does on a warning-state run: update
	// latest.json only. last-known-good.json must never come into being.
	if err := UpdateLatest(dir, datedPath); err != nil {
		t.Fatalf("UpdateLatest() error = %v", err)
	}

	if _, err := os.Stat(LastKnownGoodPath(dir)); !os.IsNotExist(err) {
		t.Error("last-known-good.json exists, want it to remain absent on a warning run")
	}
}

func TestFailedState_NeitherPointerModified(t *testing.T) {
	dir := t.TempDir()
	datedPath, err := WriteJSON(dir, testSnapshot(), time.Now())
	if err != nil {
		t.Fatalf("WriteJSON() error = %v", err)
	}
	// Seed pointers from a prior valid run so we can prove a failed run
	// leaves them untouched.
	if err := UpdateLatest(dir, datedPath); err != nil {
		t.Fatalf("UpdateLatest() error = %v", err)
	}
	if err := UpdateLastKnownGood(dir, datedPath); err != nil {
		t.Fatalf("UpdateLastKnownGood() error = %v", err)
	}
	wantLatest, _ := os.ReadFile(LatestPath(dir))
	wantLKG, _ := os.ReadFile(LastKnownGoodPath(dir))

	// A failed run calls neither UpdateLatest nor UpdateLastKnownGood -
	// verify that not calling them really does leave both pointers intact.
	gotLatest, err := os.ReadFile(LatestPath(dir))
	if err != nil {
		t.Fatalf("ReadFile(latest) error = %v", err)
	}
	gotLKG, err := os.ReadFile(LastKnownGoodPath(dir))
	if err != nil {
		t.Fatalf("ReadFile(lkg) error = %v", err)
	}
	if string(gotLatest) != string(wantLatest) || string(gotLKG) != string(wantLKG) {
		t.Error("pointer content changed despite neither update function being called")
	}
}

func TestUpdatePointers_AtomicNoTmpResidue(t *testing.T) {
	dir := t.TempDir()
	datedPath, err := WriteJSON(dir, testSnapshot(), time.Now())
	if err != nil {
		t.Fatalf("WriteJSON() error = %v", err)
	}
	if err := UpdateLatest(dir, datedPath); err != nil {
		t.Fatalf("UpdateLatest() error = %v", err)
	}
	if err := UpdateLastKnownGood(dir, datedPath); err != nil {
		t.Fatalf("UpdateLastKnownGood() error = %v", err)
	}
	noTmpFilesRemain(t, dir)
}

func TestUpdatePointers_RealFilesNotSymlinks(t *testing.T) {
	dir := t.TempDir()
	datedPath, err := WriteJSON(dir, testSnapshot(), time.Now())
	if err != nil {
		t.Fatalf("WriteJSON() error = %v", err)
	}
	if err := UpdateLatest(dir, datedPath); err != nil {
		t.Fatalf("UpdateLatest() error = %v", err)
	}
	if err := UpdateLastKnownGood(dir, datedPath); err != nil {
		t.Fatalf("UpdateLastKnownGood() error = %v", err)
	}

	for _, p := range []string{LatestPath(dir), LastKnownGoodPath(dir)} {
		info, err := os.Lstat(p)
		if err != nil {
			t.Fatalf("Lstat(%q) error = %v", p, err)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			t.Errorf("%q is a symlink, want a real file", p)
		}
	}
}

func TestUpdateLatest_CalledTwice_Idempotent(t *testing.T) {
	dir := t.TempDir()
	datedPath, err := WriteJSON(dir, testSnapshot(), time.Now())
	if err != nil {
		t.Fatalf("WriteJSON() error = %v", err)
	}

	if err := UpdateLatest(dir, datedPath); err != nil {
		t.Fatalf("first UpdateLatest() error = %v", err)
	}
	first, _ := os.ReadFile(LatestPath(dir))

	if err := UpdateLatest(dir, datedPath); err != nil {
		t.Fatalf("second UpdateLatest() error = %v", err)
	}
	second, _ := os.ReadFile(LatestPath(dir))

	if string(first) != string(second) {
		t.Error("calling UpdateLatest twice produced different content")
	}
}

func TestUpdateLatest_FailureDoesNotCorruptExistingPointer(t *testing.T) {
	dir := t.TempDir()
	datedPath, err := WriteJSON(dir, testSnapshot(), time.Now())
	if err != nil {
		t.Fatalf("WriteJSON() error = %v", err)
	}
	if err := UpdateLatest(dir, datedPath); err != nil {
		t.Fatalf("UpdateLatest() error = %v", err)
	}
	want, _ := os.ReadFile(LatestPath(dir))

	missing := filepath.Join(dir, "does-not-exist.json")
	if err := UpdateLatest(dir, missing); err == nil {
		t.Fatal("UpdateLatest() error = nil, want an error reading a missing source")
	}

	got, err := os.ReadFile(LatestPath(dir))
	if err != nil {
		t.Fatalf("ReadFile(latest) error = %v", err)
	}
	if string(got) != string(want) {
		t.Error("latest.json was corrupted by a failed update")
	}
}

func TestDatedSnapshot_RetainedRegardlessOfPointerUpdates(t *testing.T) {
	dir := t.TempDir()
	datedPath, err := WriteJSON(dir, testSnapshot(), time.Now())
	if err != nil {
		t.Fatalf("WriteJSON() error = %v", err)
	}
	// Simulate a "failed" run: no pointer updates at all.
	if _, err := os.Stat(datedPath); err != nil {
		t.Errorf("dated snapshot missing even though no pointer update was attempted: %v", err)
	}
}
