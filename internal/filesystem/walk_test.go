package filesystem

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func writeFile(t *testing.T, path string, size int) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("MkdirAll(%q): %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, make([]byte, size), 0644); err != nil {
		t.Fatalf("WriteFile(%q): %v", path, err)
	}
}

func TestWalkRoots_NestedTree_ReturnsMediaFilesWithRelativeSlashPaths(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "Movies", "Inception (2010)", "Inception.mkv"), 10)
	writeFile(t, filepath.Join(dir, "Movies", "Inception (2010)", "Inception.nfo"), 5)

	result, err := WalkRoots(context.Background(), []Root{{Name: "movies", Path: dir}})
	if err != nil {
		t.Fatalf("WalkRoots() error = %v", err)
	}
	if len(result.Files) != 1 {
		t.Fatalf("Files = %v, want 1 entry", result.Files)
	}
	want := "Movies/Inception (2010)/Inception.mkv"
	if result.Files[0].RelPath != want {
		t.Errorf("RelPath = %q, want %q", result.Files[0].RelPath, want)
	}
}

func TestWalkRoots_NonMediaFilesExcluded(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.srt"), 1)
	writeFile(t, filepath.Join(dir, "b.jpg"), 1)
	writeFile(t, filepath.Join(dir, "c.txt"), 1)
	writeFile(t, filepath.Join(dir, "d.mkv"), 1)

	result, err := WalkRoots(context.Background(), []Root{{Name: "r", Path: dir}})
	if err != nil {
		t.Fatalf("WalkRoots() error = %v", err)
	}
	if len(result.Files) != 1 || result.Files[0].RelPath != "d.mkv" {
		t.Errorf("Files = %v, want only d.mkv", result.Files)
	}
}

func TestWalkRoots_ExtensionMatchingCaseInsensitive(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.MKV"), 1)

	result, err := WalkRoots(context.Background(), []Root{{Name: "r", Path: dir}})
	if err != nil {
		t.Fatalf("WalkRoots() error = %v", err)
	}
	if len(result.Files) != 1 {
		t.Errorf("Files = %v, want 1 entry for uppercase extension", result.Files)
	}
}

func TestWalkRoots_HiddenAndNoiseEntriesSkipped(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, ".hidden.mkv"), 1)
	writeFile(t, filepath.Join(dir, "@eaDir", "thumb.mkv"), 1)
	writeFile(t, filepath.Join(dir, "lost+found", "orphan.mkv"), 1)
	writeFile(t, filepath.Join(dir, "real.mkv"), 1)

	result, err := WalkRoots(context.Background(), []Root{{Name: "r", Path: dir}})
	if err != nil {
		t.Fatalf("WalkRoots() error = %v", err)
	}
	if len(result.Files) != 1 || result.Files[0].RelPath != "real.mkv" {
		t.Errorf("Files = %v, want only real.mkv", result.Files)
	}
}

func TestWalkRoots_ZeroByteFileIncluded(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "empty.mkv"), 0)

	result, err := WalkRoots(context.Background(), []Root{{Name: "r", Path: dir}})
	if err != nil {
		t.Fatalf("WalkRoots() error = %v", err)
	}
	if len(result.Files) != 1 || result.Files[0].Bytes != 0 {
		t.Errorf("Files = %v, want one zero-byte entry", result.Files)
	}
}

func TestWalkRoots_BytesAndModTimeMatchStat(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a.mkv")
	writeFile(t, path, 42)

	wantInfo, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat() error = %v", err)
	}

	result, err := WalkRoots(context.Background(), []Root{{Name: "r", Path: dir}})
	if err != nil {
		t.Fatalf("WalkRoots() error = %v", err)
	}
	if len(result.Files) != 1 {
		t.Fatalf("Files = %v, want 1 entry", result.Files)
	}
	f := result.Files[0]
	if f.Bytes != wantInfo.Size() {
		t.Errorf("Bytes = %d, want %d", f.Bytes, wantInfo.Size())
	}
	if !f.ModTime.Equal(wantInfo.ModTime()) {
		t.Errorf("ModTime = %v, want %v", f.ModTime, wantInfo.ModTime())
	}
}

func TestWalkRoots_NonexistentRoot_RecordedAsFailedWithSentinelError(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "does-not-exist")

	result, err := WalkRoots(context.Background(), []Root{{Name: "movies", Path: missing}})
	if err == nil {
		t.Fatal("WalkRoots() error = nil, want ErrRootInaccessible")
	}
	if !errors.Is(err, ErrRootInaccessible) {
		t.Errorf("err = %v, want it to wrap ErrRootInaccessible", err)
	}
	if len(result.RootsFailed) != 1 || result.RootsFailed[0].Root != "movies" {
		t.Errorf("RootsFailed = %v, want one entry for movies", result.RootsFailed)
	}
}

func TestWalkRoots_UnreadableSubdir_RecordedAsErrorOtherRootsContinue(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permission bits behave differently on windows")
	}
	if os.Geteuid() == 0 {
		t.Skip("running as root ignores permission bits")
	}

	dirA := t.TempDir()
	blocked := filepath.Join(dirA, "blocked")
	writeFile(t, filepath.Join(blocked, "hidden.mkv"), 1)
	writeFile(t, filepath.Join(dirA, "visible.mkv"), 1)
	if err := os.Chmod(blocked, 0000); err != nil {
		t.Fatalf("Chmod() error = %v", err)
	}
	t.Cleanup(func() { os.Chmod(blocked, 0755) })

	dirB := t.TempDir()
	writeFile(t, filepath.Join(dirB, "other.mkv"), 1)

	result, err := WalkRoots(context.Background(), []Root{
		{Name: "a", Path: dirA},
		{Name: "b", Path: dirB},
	})
	if err != nil {
		t.Fatalf("WalkRoots() error = %v, want nil (subdir errors are non-fatal)", err)
	}
	if len(result.Errors) == 0 {
		t.Error("Errors = empty, want the blocked subdir recorded")
	}
	if result.Complete {
		t.Error("Complete = true, want false when a subdir could not be read")
	}

	var gotB, gotVisible bool
	for _, f := range result.Files {
		if f.Root == "b" && f.RelPath == "other.mkv" {
			gotB = true
		}
		if f.Root == "a" && f.RelPath == "visible.mkv" {
			gotVisible = true
		}
	}
	if !gotB {
		t.Error("root b's file missing, want walk of other roots to continue")
	}
	if !gotVisible {
		t.Error("root a's visible file missing despite sibling dir being blocked")
	}
}

func TestWalkRoots_EmptyRoot_ZeroFilesNoFailure(t *testing.T) {
	dir := t.TempDir()

	result, err := WalkRoots(context.Background(), []Root{{Name: "r", Path: dir}})
	if err != nil {
		t.Fatalf("WalkRoots() error = %v, want nil", err)
	}
	if len(result.Files) != 0 {
		t.Errorf("Files = %v, want none", result.Files)
	}
	if len(result.RootsFailed) != 0 {
		t.Errorf("RootsFailed = %v, want none (empty root is not a failure)", result.RootsFailed)
	}
	if len(result.RootsScanned) != 1 || result.RootsScanned[0] != "r" {
		t.Errorf("RootsScanned = %v, want [r]", result.RootsScanned)
	}
}

func TestWalkRoots_ContextCancelled_ReturnsErrAndIncomplete(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.mkv"), 1)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	result, err := WalkRoots(ctx, []Root{{Name: "r", Path: dir}})
	if !errors.Is(err, context.Canceled) {
		t.Errorf("err = %v, want context.Canceled", err)
	}
	if result.Complete {
		t.Error("Complete = true, want false after cancellation")
	}
}

func TestWalkRoots_SymlinkedFile_FollowedAndCountedOnce(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation requires elevated privileges on windows")
	}
	dir := t.TempDir()
	real := filepath.Join(dir, "real.mkv")
	writeFile(t, real, 5)
	link := filepath.Join(dir, "link.mkv")
	if err := os.Symlink(real, link); err != nil {
		t.Fatalf("Symlink() error = %v", err)
	}

	result, err := WalkRoots(context.Background(), []Root{{Name: "r", Path: dir}})
	if err != nil {
		t.Fatalf("WalkRoots() error = %v", err)
	}
	if len(result.Files) != 2 {
		t.Fatalf("Files = %v, want 2 entries (real + followed symlink)", result.Files)
	}
}

func TestWalkRoots_SymlinkedDirectory_NotFollowed(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation requires elevated privileges on windows")
	}
	dir := t.TempDir()
	realDir := filepath.Join(dir, "real")
	writeFile(t, filepath.Join(realDir, "inside.mkv"), 1)
	linkDir := filepath.Join(dir, "linked")
	if err := os.Symlink(realDir, linkDir); err != nil {
		t.Fatalf("Symlink() error = %v", err)
	}

	done := make(chan struct{})
	var result Result
	var err error
	go func() {
		result, err = WalkRoots(context.Background(), []Root{{Name: "r", Path: dir}})
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("WalkRoots() did not return - possible symlink loop")
	}
	if err != nil {
		t.Fatalf("WalkRoots() error = %v", err)
	}
	if len(result.Files) != 1 || result.Files[0].RelPath != "real/inside.mkv" {
		t.Errorf("Files = %v, want only real/inside.mkv (symlinked dir not followed)", result.Files)
	}
}

func TestWalkRoots_TwoRoots_FilesCarryCorrectRootLabel(t *testing.T) {
	movies := t.TempDir()
	tv := t.TempDir()
	writeFile(t, filepath.Join(movies, "a.mkv"), 1)
	writeFile(t, filepath.Join(tv, "b.mkv"), 1)

	result, err := WalkRoots(context.Background(), []Root{
		{Name: "movies", Path: movies},
		{Name: "tv", Path: tv},
	})
	if err != nil {
		t.Fatalf("WalkRoots() error = %v", err)
	}
	if len(result.Files) != 2 {
		t.Fatalf("Files = %v, want 2 entries", result.Files)
	}
	for _, f := range result.Files {
		if f.RelPath == "a.mkv" && f.Root != "movies" {
			t.Errorf("a.mkv Root = %q, want movies", f.Root)
		}
		if f.RelPath == "b.mkv" && f.Root != "tv" {
			t.Errorf("b.mkv Root = %q, want tv", f.Root)
		}
	}
}

func TestWalkRoots_ResultsDeterministicOrder(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "z.mkv"), 1)
	writeFile(t, filepath.Join(dir, "a.mkv"), 1)
	writeFile(t, filepath.Join(dir, "m.mkv"), 1)

	var lastOrder []string
	for i := 0; i < 5; i++ {
		result, err := WalkRoots(context.Background(), []Root{{Name: "r", Path: dir}})
		if err != nil {
			t.Fatalf("WalkRoots() error = %v", err)
		}
		order := make([]string, len(result.Files))
		for i, f := range result.Files {
			order[i] = f.RelPath
		}
		if lastOrder != nil {
			if len(order) != len(lastOrder) {
				t.Fatalf("order length changed: %v vs %v", order, lastOrder)
			}
			for i := range order {
				if order[i] != lastOrder[i] {
					t.Fatalf("non-deterministic order: %v vs %v", order, lastOrder)
				}
			}
		}
		lastOrder = order
	}
	want := []string{"a.mkv", "m.mkv", "z.mkv"}
	for i, w := range want {
		if lastOrder[i] != w {
			t.Errorf("order = %v, want sorted %v", lastOrder, want)
		}
	}
}

func TestWalkRoots_NeverModifiesFilesystem(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a.mkv")
	writeFile(t, path, 5)

	before, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat() error = %v", err)
	}

	if _, err := WalkRoots(context.Background(), []Root{{Name: "r", Path: dir}}); err != nil {
		t.Fatalf("WalkRoots() error = %v", err)
	}

	after, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat() error = %v", err)
	}
	if before.Size() != after.Size() || !before.ModTime().Equal(after.ModTime()) {
		t.Error("fixture file changed after WalkRoots(), want walker to be read-only")
	}
}
