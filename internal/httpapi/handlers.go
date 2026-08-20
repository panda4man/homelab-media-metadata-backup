package httpapi

import (
	"errors"
	"net/http"
	"sync"

	"github.com/panda4man/homelab-media-metadata-backup/internal/runlock"
)

// runsHandler serves POST /v1/runs and GET /v1/runs/{id}. busy tracks
// whether a run is currently in flight, guarding against a second trigger
// without needing to hold the run lock file for the whole run duration.
type runsHandler struct {
	cfg   Config
	store *runStore

	mu   sync.Mutex
	busy bool
}

func newRunsHandler(cfg Config) *runsHandler {
	return &runsHandler{cfg: cfg, store: newRunStore()}
}

func (h *runsHandler) create(w http.ResponseWriter, r *http.Request) {
	h.mu.Lock()
	if h.busy {
		h.mu.Unlock()
		writeJSON(w, http.StatusConflict, ErrorResponse{Error: "already_running"})
		return
	}
	h.mu.Unlock()

	// A preflight, non-held Acquire/release checks whether the cron
	// process (or another instance) already owns the lock file, without
	// this handler holding it for the run's whole duration.
	release, lockErr := runlock.Acquire(h.cfg.LockPath)
	if lockErr != nil {
		writeJSON(w, http.StatusConflict, ErrorResponse{Error: "already_running"})
		return
	}
	_ = release()

	h.mu.Lock()
	h.busy = true
	h.mu.Unlock()

	run := Run{ID: h.cfg.NewID(), Status: "running", StartedAt: h.cfg.Now()}
	h.store.create(run)

	go h.execute(run.ID)

	w.Header().Set("Location", "/v1/runs/"+run.ID)
	writeJSON(w, http.StatusAccepted, run)
}

// execute runs in its own goroutine, outliving the request that started
// it: h.cfg.Ctx is a server-scoped context, never r.Context().
func (h *runsHandler) execute(id string) {
	defer func() {
		h.mu.Lock()
		h.busy = false
		h.mu.Unlock()
	}()

	result, err := h.cfg.RunFn(h.cfg.Ctx, h.cfg.Cfg)

	updated, _ := h.store.get(id)
	updated.FinishedAt = h.cfg.Now()

	// Must check ErrLocked before touching result.State: a rejected run
	// returns a zero-value Result, and StateValid is zero (iota-based),
	// so reading result.State first would misreport it as "valid".
	if errors.Is(err, runlock.ErrLocked) {
		updated.Status = "rejected"
		updated.Reason = "already_running"
	} else {
		updated.Status = "completed"
		updated.State = result.State.String()
		updated.SnapshotPath = result.SnapshotPath
		updated.OffsiteSuccess = result.OffsiteSuccess
		if err != nil {
			updated.Error = err.Error()
		}
	}

	h.store.update(updated)
}

func (h *runsHandler) get(w http.ResponseWriter, r *http.Request) {
	run, ok := h.store.get(r.PathValue("id"))
	if !ok {
		writeJSON(w, http.StatusNotFound, ErrorResponse{Error: "not_found"})
		return
	}
	writeJSON(w, http.StatusOK, run)
}
