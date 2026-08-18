// Package clockx provides an injectable time source so callers can control
// timestamps in tests without touching the real wall clock.
package clockx

import "time"

// Clock returns the current time. Production code uses System; tests use
// Fixed so timestamps in output are deterministic.
type Clock interface {
	Now() time.Time
}

// System is the real wall-clock implementation.
type System struct{}

func (System) Now() time.Time { return time.Now() }

// Fixed always returns Instant, regardless of how many times Now is called.
type Fixed struct {
	Instant time.Time
}

func (f Fixed) Now() time.Time { return f.Instant }
