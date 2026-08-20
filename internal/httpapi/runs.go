package httpapi

import (
	"sync"
	"time"
)

// Run is the JSON-visible record of one triggered orchestrator run.
// Status is one of "running", "completed", or "rejected"; State mirrors
// validation.State.String() and is only populated once the run completes.
type Run struct {
	ID             string    `json:"id"`
	Status         string    `json:"status"`
	Reason         string    `json:"reason,omitempty"`
	StartedAt      time.Time `json:"started_at"`
	FinishedAt     time.Time `json:"finished_at,omitzero"`
	State          string    `json:"state,omitempty"`
	SnapshotPath   string    `json:"snapshot_path,omitempty"`
	OffsiteSuccess bool      `json:"offsite_success,omitempty"`
	Error          string    `json:"error,omitempty"`
}

const runStoreCap = 20

// runStore holds the most recently created runs in memory, evicting the
// oldest insertion once more than runStoreCap records exist.
type runStore struct {
	mu    sync.Mutex
	byID  map[string]*Run
	order []string
}

func newRunStore() *runStore {
	return &runStore{byID: make(map[string]*Run)}
}

// create inserts a new run record, evicting the oldest one by insertion
// order if the store is now over capacity.
func (s *runStore) create(r Run) {
	s.mu.Lock()
	defer s.mu.Unlock()

	stored := r
	s.byID[r.ID] = &stored
	s.order = append(s.order, r.ID)

	for len(s.order) > runStoreCap {
		oldest := s.order[0]
		s.order = s.order[1:]
		delete(s.byID, oldest)
	}
}

// update overwrites an existing run record without affecting eviction
// order; it is a no-op if the id is unknown (e.g. already evicted).
func (s *runStore) update(r Run) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.byID[r.ID]; !ok {
		return
	}
	stored := r
	s.byID[r.ID] = &stored
}

func (s *runStore) get(id string) (Run, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	r, ok := s.byID[id]
	if !ok {
		return Run{}, false
	}
	return *r, true
}
