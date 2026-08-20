package httpapi

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/panda4man/homelab-media-metadata-backup/internal/config"
	"github.com/panda4man/homelab-media-metadata-backup/internal/orchestrator"
	"github.com/panda4man/homelab-media-metadata-backup/internal/runlock"
	"github.com/panda4man/homelab-media-metadata-backup/internal/validation"
)

// testConfig builds a Config wired to a fresh temp-dir lock path and a
// sequential NewID/fixed Now, ready for a test to override RunFn.
func testConfig(t *testing.T) Config {
	t.Helper()
	dir := t.TempDir()
	var seq int64
	return Config{
		Token:    "test-token",
		Ctx:      context.Background(),
		Cfg:      config.Config{SnapshotPath: dir},
		LockPath: dir + "/run.lock",
		NewID: func() string {
			n := atomic.AddInt64(&seq, 1)
			return fmt.Sprintf("id-%d", n)
		},
		Now: func() time.Time { return time.Unix(0, 0).UTC() },
	}
}

// waitForRunStatus polls GET on location until the run reaches status,
// bounded by a deadline channel rather than a single guessed sleep.
func waitForRunStatus(t *testing.T, handler http.Handler, token, location, status string) *httptest.ResponseRecorder {
	t.Helper()
	deadline := time.After(2 * time.Second)
	ticker := time.NewTicker(time.Millisecond)
	defer ticker.Stop()

	for {
		req := httptest.NewRequest("GET", location, nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if strings.Contains(rec.Body.String(), `"status":"`+status+`"`) {
			return rec
		}
		select {
		case <-ticker.C:
		case <-deadline:
			t.Fatalf("run at %s did not reach status %q in time; last body = %s", location, status, rec.Body.String())
		}
	}
}

func TestRunsCreate_ValidToken_Returns202WithRunningStatus(t *testing.T) {
	done := make(chan struct{})
	cfg := testConfig(t)
	cfg.RunFn = func(ctx context.Context, c config.Config) (orchestrator.Result, error) {
		defer close(done)
		return orchestrator.Result{State: validation.StateValid}, nil
	}
	handler := New(cfg)

	req := httptest.NewRequest("POST", "/v1/runs", nil)
	req.Header.Set("Authorization", "Bearer "+cfg.Token)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want 202, body = %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, `"status":"running"`) {
		t.Errorf("body = %q, want status running", body)
	}
	if !strings.Contains(body, `"id":"id-`) {
		t.Errorf("body = %q, want a non-empty id", body)
	}
	loc := rec.Header().Get("Location")
	if !strings.HasPrefix(loc, "/v1/runs/") || loc == "/v1/runs/" {
		t.Errorf("Location = %q, want /v1/runs/<id>", loc)
	}

	<-done // avoid leaking the run goroutine past the end of the test
}

func TestRunsCreate_InvokesRunFnOnceWithServerConfig(t *testing.T) {
	var calls int32
	var gotCfg config.Config
	done := make(chan struct{})
	cfg := testConfig(t)
	cfg.Cfg = config.Config{SnapshotPath: cfg.Cfg.SnapshotPath, MediaMoviesPath: "/movies"}
	cfg.RunFn = func(ctx context.Context, c config.Config) (orchestrator.Result, error) {
		atomic.AddInt32(&calls, 1)
		gotCfg = c
		close(done)
		return orchestrator.Result{}, nil
	}
	handler := New(cfg)

	req := httptest.NewRequest("POST", "/v1/runs", nil)
	req.Header.Set("Authorization", "Bearer "+cfg.Token)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	<-done
	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Fatalf("RunFn called %d times, want 1", got)
	}
	if gotCfg.MediaMoviesPath != "/movies" {
		t.Errorf("RunFn received cfg = %+v, want MediaMoviesPath=/movies", gotCfg)
	}
}

func TestRunsGet_AfterCompletion_ReportsCompletedState(t *testing.T) {
	cfg := testConfig(t)
	cfg.RunFn = func(ctx context.Context, c config.Config) (orchestrator.Result, error) {
		return orchestrator.Result{State: validation.StateValid, SnapshotPath: "/x.json", OffsiteSuccess: true}, nil
	}
	handler := New(cfg)

	req := httptest.NewRequest("POST", "/v1/runs", nil)
	req.Header.Set("Authorization", "Bearer "+cfg.Token)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	loc := rec.Header().Get("Location")

	getRec := waitForRunStatus(t, handler, cfg.Token, loc, "completed")
	if getRec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body=%s", getRec.Code, getRec.Body.String())
	}
	body := getRec.Body.String()
	for _, want := range []string{`"status":"completed"`, `"state":"valid"`, `"snapshot_path":"/x.json"`, `"offsite_success":true`} {
		if !strings.Contains(body, want) {
			t.Errorf("body = %q, want it to contain %q", body, want)
		}
	}
}

func TestRunsGet_RunFnReturnsErrLocked_ReportsRejectedNotValidState(t *testing.T) {
	cfg := testConfig(t)
	cfg.RunFn = func(ctx context.Context, c config.Config) (orchestrator.Result, error) {
		return orchestrator.Result{}, runlock.ErrLocked
	}
	handler := New(cfg)

	req := httptest.NewRequest("POST", "/v1/runs", nil)
	req.Header.Set("Authorization", "Bearer "+cfg.Token)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	loc := rec.Header().Get("Location")

	getRec := waitForRunStatus(t, handler, cfg.Token, loc, "rejected")
	body := getRec.Body.String()
	if !strings.Contains(body, `"reason":"already_running"`) {
		t.Errorf("body = %q, want reason already_running", body)
	}
	// Regression guard: Result's zero value reads as StateValid (iota==0),
	// so a rejected run must never let that leak through as state=valid.
	if strings.Contains(body, `"state":"valid"`) {
		t.Errorf("body = %q, want state not reported as valid", body)
	}
}

func TestRunsGet_RunFnReturnsNonLockError_ReportsCompletedWithError(t *testing.T) {
	cfg := testConfig(t)
	cfg.RunFn = func(ctx context.Context, c config.Config) (orchestrator.Result, error) {
		return orchestrator.Result{State: validation.StateFailed}, errors.New("boom")
	}
	handler := New(cfg)

	req := httptest.NewRequest("POST", "/v1/runs", nil)
	req.Header.Set("Authorization", "Bearer "+cfg.Token)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	loc := rec.Header().Get("Location")

	getRec := waitForRunStatus(t, handler, cfg.Token, loc, "completed")
	body := getRec.Body.String()
	if !strings.Contains(body, `"error":"boom"`) {
		t.Errorf("body = %q, want error boom", body)
	}
	if !strings.Contains(body, `"state":"failed"`) {
		t.Errorf("body = %q, want state failed", body)
	}
}

func TestRunsCreate_SecondPostWhileInFlight_Returns409(t *testing.T) {
	release := make(chan struct{})
	started := make(chan struct{})
	var calls int32
	cfg := testConfig(t)
	cfg.RunFn = func(ctx context.Context, c config.Config) (orchestrator.Result, error) {
		atomic.AddInt32(&calls, 1)
		close(started)
		<-release
		return orchestrator.Result{State: validation.StateValid}, nil
	}
	handler := New(cfg)

	req1 := httptest.NewRequest("POST", "/v1/runs", nil)
	req1.Header.Set("Authorization", "Bearer "+cfg.Token)
	rec1 := httptest.NewRecorder()
	handler.ServeHTTP(rec1, req1)
	if rec1.Code != http.StatusAccepted {
		t.Fatalf("first POST status = %d, want 202", rec1.Code)
	}

	<-started

	req2 := httptest.NewRequest("POST", "/v1/runs", nil)
	req2.Header.Set("Authorization", "Bearer "+cfg.Token)
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req2)

	if rec2.Code != http.StatusConflict {
		t.Fatalf("second POST status = %d, want 409, body=%s", rec2.Code, rec2.Body.String())
	}
	if !strings.Contains(rec2.Body.String(), `"error":"already_running"`) {
		t.Errorf("body = %q, want error already_running", rec2.Body.String())
	}

	close(release)
	waitForRunStatus(t, handler, cfg.Token, rec1.Header().Get("Location"), "completed")

	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Errorf("RunFn called %d times, want 1", got)
	}
}

func TestRunsCreate_LockFileAlreadyHeld_Returns409WithoutInvokingRunFn(t *testing.T) {
	cfg := testConfig(t)
	release, err := runlock.Acquire(cfg.LockPath)
	if err != nil {
		t.Fatalf("Acquire() error = %v", err)
	}
	defer release()

	var calls int32
	cfg.RunFn = func(ctx context.Context, c config.Config) (orchestrator.Result, error) {
		atomic.AddInt32(&calls, 1)
		return orchestrator.Result{}, nil
	}
	handler := New(cfg)

	req := httptest.NewRequest("POST", "/v1/runs", nil)
	req.Header.Set("Authorization", "Bearer "+cfg.Token)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409, body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"error":"already_running"`) {
		t.Errorf("body = %q, want error already_running", rec.Body.String())
	}
	if atomic.LoadInt32(&calls) != 0 {
		t.Errorf("RunFn called %d times, want 0", calls)
	}
}

func TestRunsGet_UnknownID_404(t *testing.T) {
	cfg := testConfig(t)
	handler := New(cfg)

	req := httptest.NewRequest("GET", "/v1/runs/does-not-exist", nil)
	req.Header.Set("Authorization", "Bearer "+cfg.Token)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404, body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"error":"not_found"`) {
		t.Errorf("body = %q, want error not_found", rec.Body.String())
	}
}

func TestRunsCreate_NoToken_401WithoutInvokingRunFn(t *testing.T) {
	var calls int32
	cfg := testConfig(t)
	cfg.RunFn = func(ctx context.Context, c config.Config) (orchestrator.Result, error) {
		atomic.AddInt32(&calls, 1)
		return orchestrator.Result{}, nil
	}
	handler := New(cfg)

	req := httptest.NewRequest("POST", "/v1/runs", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
	if atomic.LoadInt32(&calls) != 0 {
		t.Errorf("RunFn called %d times, want 0", calls)
	}
}

func TestRunsCreate_RunFnContextOutlivesRequest(t *testing.T) {
	cfg := testConfig(t)
	serverCtx, cancelServerCtx := context.WithCancel(context.Background())
	defer cancelServerCtx()
	cfg.Ctx = serverCtx

	errCh := make(chan error, 1)
	cfg.RunFn = func(runCtx context.Context, c config.Config) (orchestrator.Result, error) {
		// Sleep past the point the request's own context would be
		// cancelled, to prove RunFn wasn't handed r.Context().
		time.Sleep(20 * time.Millisecond)
		errCh <- runCtx.Err()
		return orchestrator.Result{}, nil
	}
	handler := New(cfg)

	reqCtx, cancelReq := context.WithCancel(context.Background())
	req := httptest.NewRequest("POST", "/v1/runs", nil).WithContext(reqCtx)
	req.Header.Set("Authorization", "Bearer "+cfg.Token)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	cancelReq()

	select {
	case err := <-errCh:
		if err != nil {
			t.Errorf("ctx.Err() = %v, want nil - RunFn's context must outlive the request", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("RunFn did not report back in time")
	}
}
