package runlock

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

func TestAcquire_FreeLock_Succeeds(t *testing.T) {
	path := filepath.Join(t.TempDir(), "run.lock")

	release, err := Acquire(path)
	if err != nil {
		t.Fatalf("Acquire() error = %v", err)
	}
	if err := release(); err != nil {
		t.Errorf("release() error = %v", err)
	}
}

func TestAcquire_AlreadyHeld_ReturnsErrLocked(t *testing.T) {
	path := filepath.Join(t.TempDir(), "run.lock")

	release, err := Acquire(path)
	if err != nil {
		t.Fatalf("first Acquire() error = %v", err)
	}
	defer release()

	_, err = Acquire(path)
	if !errors.Is(err, ErrLocked) {
		t.Errorf("second Acquire() error = %v, want ErrLocked", err)
	}
}

func TestAcquire_AfterRelease_Succeeds(t *testing.T) {
	path := filepath.Join(t.TempDir(), "run.lock")

	release, err := Acquire(path)
	if err != nil {
		t.Fatalf("first Acquire() error = %v", err)
	}
	if err := release(); err != nil {
		t.Fatalf("release() error = %v", err)
	}

	release2, err := Acquire(path)
	if err != nil {
		t.Fatalf("second Acquire() error = %v, want success after release", err)
	}
	release2()
}

func TestAcquire_StaleLockFromDeadProcess_Reclaimed(t *testing.T) {
	path := filepath.Join(t.TempDir(), "run.lock")

	// Simulate a crashed holder: flock the file directly and close the fd
	// WITHOUT unlocking - this is exactly what happens when a process dies
	// unexpectedly. The kernel releases the flock when the fd closes.
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		t.Fatalf("OpenFile() error = %v", err)
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		t.Fatalf("Flock() error = %v", err)
	}
	f.Close() // no unlock - simulates a crash

	release, err := Acquire(path)
	if err != nil {
		t.Fatalf("Acquire() error = %v, want the stale lock reclaimed", err)
	}
	release()
}

func TestRelease_Idempotent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "run.lock")

	release, err := Acquire(path)
	if err != nil {
		t.Fatalf("Acquire() error = %v", err)
	}
	if err := release(); err != nil {
		t.Errorf("first release() error = %v", err)
	}
	if err := release(); err != nil {
		t.Errorf("second release() error = %v, want idempotent no-op", err)
	}
}

func TestAcquire_LockFileHasSanePermissions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "run.lock")

	release, err := Acquire(path)
	if err != nil {
		t.Fatalf("Acquire() error = %v", err)
	}
	defer release()

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat() error = %v", err)
	}
	if info.Mode().Perm()&0002 != 0 {
		t.Errorf("lock file mode = %v, want not world-writable", info.Mode())
	}
}
