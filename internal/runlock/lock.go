// Package runlock prevents two inventory runs from executing
// concurrently, using an advisory flock on a lock file. flock is
// self-healing: if the holding process dies for any reason, the kernel
// releases the lock automatically when the file descriptor closes, so a
// crashed run can never leave a permanently stuck lock behind.
package runlock

import (
	"errors"
	"fmt"
	"os"
	"syscall"
)

// ErrLocked means another run already holds the lock.
var ErrLocked = errors.New("runlock: another run is already in progress")

// Acquire takes an exclusive, non-blocking lock on the file at path,
// creating it if necessary. The returned release function unlocks and
// closes it; it is idempotent and safe to call from a defer.
func Acquire(path string) (release func() error, err error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return nil, fmt.Errorf("runlock: opening lock file %s: %w", path, err)
	}

	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		f.Close()
		if errors.Is(err, syscall.EWOULDBLOCK) {
			return nil, fmt.Errorf("%w: %s", ErrLocked, path)
		}
		return nil, fmt.Errorf("runlock: locking %s: %w", path, err)
	}

	released := false
	release = func() error {
		if released {
			return nil
		}
		released = true
		unlockErr := syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
		closeErr := f.Close()
		if unlockErr != nil {
			return fmt.Errorf("runlock: unlocking %s: %w", path, unlockErr)
		}
		if closeErr != nil {
			return fmt.Errorf("runlock: closing %s: %w", path, closeErr)
		}
		return nil
	}
	return release, nil
}
