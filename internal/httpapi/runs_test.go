package httpapi

import (
	"fmt"
	"testing"
)

func TestRunStore_CreateThenGet_ReturnsTheRun(t *testing.T) {
	s := newRunStore()
	s.create(Run{ID: "abc", Status: "running"})

	got, ok := s.get("abc")
	if !ok {
		t.Fatal("get() ok = false, want true")
	}
	if got.Status != "running" {
		t.Errorf("Status = %q, want running", got.Status)
	}
}

func TestRunStore_GetMissing_ReturnsFalse(t *testing.T) {
	s := newRunStore()

	if _, ok := s.get("nope"); ok {
		t.Error("get() ok = true, want false for a missing id")
	}
}

func TestRunStore_Update_MutatesTheExistingRecord(t *testing.T) {
	s := newRunStore()
	s.create(Run{ID: "abc", Status: "running"})
	s.update(Run{ID: "abc", Status: "completed", State: "valid"})

	got, ok := s.get("abc")
	if !ok {
		t.Fatal("get() ok = false, want true")
	}
	if got.Status != "completed" || got.State != "valid" {
		t.Errorf("got = %+v, want status=completed state=valid", got)
	}
}

func TestRunStore_CapAt20_EvictsOldestByInsertionOrder(t *testing.T) {
	s := newRunStore()
	for i := range 21 {
		s.create(Run{ID: fmt.Sprintf("run-%d", i), Status: "running"})
	}

	if _, ok := s.get("run-0"); ok {
		t.Error(`get("run-0") ok = true, want it evicted as the oldest insertion`)
	}
	if _, ok := s.get("run-20"); !ok {
		t.Error(`get("run-20") ok = false, want the most recent run present`)
	}

	remaining := 0
	for i := range 21 {
		if _, ok := s.get(fmt.Sprintf("run-%d", i)); ok {
			remaining++
		}
	}
	if remaining != 20 {
		t.Errorf("remaining entries = %d, want 20", remaining)
	}
}
