package clockx

import (
	"testing"
	"time"
)

func TestFixed_Now_ReturnsConfiguredInstantRepeatedly(t *testing.T) {
	want := time.Date(2026, 8, 16, 23, 0, 0, 0, time.UTC)
	c := Fixed{Instant: want}

	for i := 0; i < 3; i++ {
		if got := c.Now(); !got.Equal(want) {
			t.Fatalf("call %d: Now() = %v, want %v", i, got, want)
		}
	}
}

func TestSystem_Now_ReturnsCurrentTime(t *testing.T) {
	c := System{}
	before := time.Now()
	got := c.Now()
	after := time.Now()

	if got.Before(before) || got.After(after) {
		t.Fatalf("Now() = %v, want between %v and %v", got, before, after)
	}
}
