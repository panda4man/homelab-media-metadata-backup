package snapshot

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/panda4man/homelab-media-metadata-backup/internal/inventory"
)

func testSnapshot() inventory.Snapshot {
	return inventory.NewSnapshot(
		time.Date(2026, 8, 16, 23, 0, 0, 0, time.UTC),
		"unraid",
		[]inventory.Movie{
			{Title: "Inception", Year: 2010, TMDBID: 27205, Dir: "Inception (2010)", Path: "Inception (2010)/Inception.mkv", Bytes: 1000, MTime: time.Now().UTC()},
		},
		nil,
	)
}

func noTmpFilesRemain(t *testing.T, dir string) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir(%q): %v", dir, err)
	}
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".tmp") {
			t.Errorf("stray temp file left behind: %s", e.Name())
		}
	}
}

func TestWriteJSON_UsesGivenTimeLocationForFilename(t *testing.T) {
	dir := t.TempDir()
	edt := time.FixedZone("EDT", -4*3600)
	// 2026-08-16 23:00 EDT is 2026-08-17 03:00 UTC - the filename must use
	// the local date the caller passed in, not the UTC date.
	now := time.Date(2026, 8, 16, 23, 0, 0, 0, edt)

	path, err := WriteJSON(dir, testSnapshot(), now)
	if err != nil {
		t.Fatalf("WriteJSON() error = %v", err)
	}
	if filepath.Base(path) != "media-inventory-2026-08-16.json" {
		t.Errorf("path = %q, want the 2026-08-16 filename (local date, not UTC)", path)
	}
}

func TestWriteJSON_NoTmpFilesRemainAfterSuccess(t *testing.T) {
	dir := t.TempDir()

	if _, err := WriteJSON(dir, testSnapshot(), time.Now()); err != nil {
		t.Fatalf("WriteJSON() error = %v", err)
	}
	noTmpFilesRemain(t, dir)
}

func TestWriteJSON_WrittenFileParsesBackEqual(t *testing.T) {
	dir := t.TempDir()
	want := testSnapshot()

	path, err := WriteJSON(dir, want, time.Now())
	if err != nil {
		t.Fatalf("WriteJSON() error = %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	var got inventory.Snapshot
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if got.Hostname != want.Hostname || len(got.Movies) != len(want.Movies) {
		t.Errorf("got = %+v, want %+v", got, want)
	}
}

func TestWriteJSON_OverwritesExistingTarget(t *testing.T) {
	dir := t.TempDir()
	now := time.Now()

	if _, err := WriteJSON(dir, testSnapshot(), now); err != nil {
		t.Fatalf("first WriteJSON() error = %v", err)
	}
	path, err := WriteJSON(dir, testSnapshot(), now)
	if err != nil {
		t.Fatalf("second WriteJSON() error = %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	var s inventory.Snapshot
	if err := json.Unmarshal(data, &s); err != nil {
		t.Fatalf("re-written file does not parse: %v", err)
	}
	noTmpFilesRemain(t, dir)
}

func TestWriteJSON_CreatesSnapshotDirIfMissing(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "does", "not", "exist", "yet")

	path, err := WriteJSON(dir, testSnapshot(), time.Now())
	if err != nil {
		t.Fatalf("WriteJSON() error = %v, want the directory to be created", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("Stat(%q) error = %v, want the file to exist", path, err)
	}
}

func TestWriteJSON_UnwritableDirectory_TypedErrorNoPartialFiles(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permission bits behave differently on windows")
	}
	if os.Geteuid() == 0 {
		t.Skip("running as root ignores permission bits")
	}

	parent := t.TempDir()
	if err := os.Chmod(parent, 0555); err != nil {
		t.Fatalf("Chmod() error = %v", err)
	}
	t.Cleanup(func() { os.Chmod(parent, 0755) })
	dir := filepath.Join(parent, "snapshots")

	_, err := WriteJSON(dir, testSnapshot(), time.Now())
	if err == nil {
		t.Fatal("WriteJSON() error = nil, want a permission error")
	}
}

func TestWriteAtomic_ValidateFailure_TargetNotCreatedTmpCleanedUp(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target.txt")

	alwaysFail := func([]byte) error { return errors.New("simulated validation failure") }
	_, err := writeAtomic(dir, "target.txt", []byte("data"), alwaysFail)
	if err == nil {
		t.Fatal("writeAtomic() error = nil, want validation failure")
	}
	if _, statErr := os.Stat(target); !os.IsNotExist(statErr) {
		t.Errorf("target file exists after validation failure, want it absent")
	}
	noTmpFilesRemain(t, dir)
}

func TestWriteSHA256_FormatMatchesConvention(t *testing.T) {
	dir := t.TempDir()
	jsonPath, err := WriteJSON(dir, testSnapshot(), time.Now())
	if err != nil {
		t.Fatalf("WriteJSON() error = %v", err)
	}

	sidecarPath, err := WriteSHA256(jsonPath)
	if err != nil {
		t.Fatalf("WriteSHA256() error = %v", err)
	}

	data, err := os.ReadFile(sidecarPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	line := string(data)
	if !strings.HasSuffix(line, "\n") {
		t.Errorf("sidecar line = %q, want trailing newline", line)
	}
	fields := strings.SplitN(strings.TrimSuffix(line, "\n"), "  ", 2)
	if len(fields) != 2 {
		t.Fatalf("sidecar line = %q, want '<hex>  <basename>' format", line)
	}
	if _, err := hex.DecodeString(fields[0]); err != nil || len(fields[0]) != 64 {
		t.Errorf("hash field = %q, want 64 hex characters", fields[0])
	}
	if fields[1] != filepath.Base(jsonPath) {
		t.Errorf("basename field = %q, want %q", fields[1], filepath.Base(jsonPath))
	}
}

func TestWriteSHA256_HashMatchesIndependentComputation(t *testing.T) {
	dir := t.TempDir()
	jsonPath, err := WriteJSON(dir, testSnapshot(), time.Now())
	if err != nil {
		t.Fatalf("WriteJSON() error = %v", err)
	}

	sidecarPath, err := WriteSHA256(jsonPath)
	if err != nil {
		t.Fatalf("WriteSHA256() error = %v", err)
	}

	jsonData, err := os.ReadFile(jsonPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	sum := sha256.Sum256(jsonData)
	wantSum := hex.EncodeToString(sum[:])

	sidecarData, err := os.ReadFile(sidecarPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if !strings.HasPrefix(string(sidecarData), wantSum) {
		t.Errorf("sidecar = %q, want it to start with independently computed hash %q", sidecarData, wantSum)
	}
}

func TestWriteSHA256_RequiresJSONFileToExist(t *testing.T) {
	dir := t.TempDir()
	missing := filepath.Join(dir, "media-inventory-2099-01-01.json")

	_, err := WriteSHA256(missing)
	if err == nil {
		t.Fatal("WriteSHA256() error = nil, want error when the JSON file does not exist yet")
	}
}
