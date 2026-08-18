package orchestrator

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/panda4man/homelab-media-metadata-backup/internal/filesystem"
	"github.com/panda4man/homelab-media-metadata-backup/internal/influx"
	"github.com/panda4man/homelab-media-metadata-backup/internal/inventory"
	"github.com/panda4man/homelab-media-metadata-backup/internal/offsite"
	"github.com/panda4man/homelab-media-metadata-backup/internal/runlock"
	"github.com/panda4man/homelab-media-metadata-backup/internal/validation"
)

func TestRun_HappyPath_ValidStateAllArtifactsWritten(t *testing.T) {
	dir := t.TempDir()
	cfg := healthyConfig(dir)

	var wroteJSON, wroteMoviesCSV, wroteEpisodesCSV, wroteSHA, updatedLatest, updatedLKG, synced, pruned, published bool
	deps := healthyDeps()
	deps.WriteJSON = func(d string, s inventory.Snapshot, now time.Time) (string, error) {
		wroteJSON = true
		return d + "/media-inventory-test.json", nil
	}
	deps.WriteMoviesCSV = func(d string, m []inventory.Movie, now time.Time) (string, error) {
		wroteMoviesCSV = true
		return d + "/movies.csv", nil
	}
	deps.WriteEpisodesCSV = func(d string, s []inventory.Series, now time.Time) (string, error) {
		wroteEpisodesCSV = true
		return d + "/episodes.csv", nil
	}
	deps.WriteSHA256 = func(p string) (string, error) { wroteSHA = true; return p + ".sha256", nil }
	deps.UpdateLatest = func(d, p string) error { updatedLatest = true; return nil }
	deps.UpdateLastKnownGood = func(d, p string) error { updatedLKG = true; return nil }
	deps.OffsiteSync = func(ctx context.Context, d string) (offsite.Result, error) {
		synced = true
		return offsite.Result{Success: true}, nil
	}
	deps.Prune = func(d string, keep int) ([]string, error) { pruned = true; return nil, nil }
	deps = withPublishRecorder(deps, &published)

	result, err := Run(context.Background(), cfg, deps)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.State != validation.StateValid {
		t.Errorf("State = %v, want StateValid", result.State)
	}
	if !wroteJSON || !wroteMoviesCSV || !wroteEpisodesCSV || !wroteSHA {
		t.Errorf("artifacts written: json=%v movies=%v episodes=%v sha=%v, want all true", wroteJSON, wroteMoviesCSV, wroteEpisodesCSV, wroteSHA)
	}
	if !updatedLatest || !updatedLKG {
		t.Errorf("pointers updated: latest=%v lkg=%v, want both true on a valid run", updatedLatest, updatedLKG)
	}
	if !synced {
		t.Error("off-site sync was not invoked")
	}
	if !pruned {
		t.Error("retention pruning was not invoked")
	}
	if !published {
		t.Error("metrics were not published")
	}
}

func TestRun_RadarrPingFails_FailedStateNoSnapshotWritten(t *testing.T) {
	dir := t.TempDir()
	cfg := healthyConfig(dir)
	deps := healthyDeps()
	deps.RadarrPing = func(ctx context.Context) error { return errors.New("connection refused") }

	var wroteJSON, published bool
	deps.WriteJSON = func(d string, s inventory.Snapshot, now time.Time) (string, error) {
		wroteJSON = true
		return "", nil
	}
	deps = withPublishRecorder(deps, &published)

	result, err := Run(context.Background(), cfg, deps)
	if err != nil {
		t.Fatalf("Run() error = %v, want nil (a failed state is not itself a Go error)", err)
	}
	if result.State != validation.StateFailed {
		t.Errorf("State = %v, want StateFailed", result.State)
	}
	if wroteJSON {
		t.Error("WriteJSON was called, want no snapshot written when Radarr is unreachable")
	}
	if !published {
		t.Error("metrics were not published even though the run failed early")
	}
}

func TestRun_SonarrPingFails_FailedStateNoSnapshotWritten(t *testing.T) {
	dir := t.TempDir()
	cfg := healthyConfig(dir)
	deps := healthyDeps()
	deps.SonarrPing = func(ctx context.Context) error { return errors.New("connection refused") }

	var wroteJSON bool
	deps.WriteJSON = func(d string, s inventory.Snapshot, now time.Time) (string, error) {
		wroteJSON = true
		return "", nil
	}

	result, err := Run(context.Background(), cfg, deps)
	if err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}
	if result.State != validation.StateFailed {
		t.Errorf("State = %v, want StateFailed", result.State)
	}
	if wroteJSON {
		t.Error("WriteJSON was called, want no snapshot written when Sonarr is unreachable")
	}
}

func TestRun_MediaRootInaccessible_FailedState_SnapshotWrittenButNoPointerUpdates(t *testing.T) {
	dir := t.TempDir()
	cfg := healthyConfig(dir)
	deps := healthyDeps()
	deps.WalkRoots = func(ctx context.Context, roots []filesystem.Root) (filesystem.Result, error) {
		result := filesystem.Result{
			Complete:     false,
			RootsScanned: []string{"tv"},
			RootsFailed:  []filesystem.RootFailure{{Root: "movies", Err: filesystem.ErrRootInaccessible}},
		}
		return result, fmt.Errorf("wrap: %w", filesystem.ErrRootInaccessible)
	}

	var wroteJSON, updatedLatest, updatedLKG bool
	deps.WriteJSON = func(d string, s inventory.Snapshot, now time.Time) (string, error) {
		wroteJSON = true
		return d + "/media-inventory-test.json", nil
	}
	deps.UpdateLatest = func(d, p string) error { updatedLatest = true; return nil }
	deps.UpdateLastKnownGood = func(d, p string) error { updatedLKG = true; return nil }

	result, err := Run(context.Background(), cfg, deps)
	if err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}
	if result.State != validation.StateFailed {
		t.Errorf("State = %v, want StateFailed", result.State)
	}
	if !wroteJSON {
		t.Error("WriteJSON was not called, want the forensic snapshot still written on a root-inaccessible failure")
	}
	if updatedLatest || updatedLKG {
		t.Errorf("pointers updated: latest=%v lkg=%v, want neither on a failed run", updatedLatest, updatedLKG)
	}
}

func TestRun_ThresholdBreach_WarningState_LatestUpdatedNotLKG(t *testing.T) {
	dir := t.TempDir()
	cfg := healthyConfig(dir)
	deps := healthyDeps()
	// A previous snapshot with many more movies than the current run finds
	// triggers the movie/bytes decrease sanity checks -> warning, not failed.
	deps.LoadPrevious = func(d string) (inventory.Snapshot, bool, error) {
		prev := inventory.NewSnapshot(fixedNow, "h",
			[]inventory.Movie{
				{Title: "A", Year: 2001, TMDBID: 1, Bytes: 100},
				{Title: "B", Year: 2002, TMDBID: 2, Bytes: 1000},
				{Title: "C", Year: 2003, TMDBID: 3, Bytes: 1000},
			}, nil)
		return prev, true, nil
	}

	var updatedLatest, updatedLKG bool
	deps.UpdateLatest = func(d, p string) error { updatedLatest = true; return nil }
	deps.UpdateLastKnownGood = func(d, p string) error { updatedLKG = true; return nil }

	result, err := Run(context.Background(), cfg, deps)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.State != validation.StateWarning {
		t.Fatalf("State = %v, want StateWarning", result.State)
	}
	if !updatedLatest {
		t.Error("latest.json was not updated on a warning run")
	}
	if updatedLKG {
		t.Error("last-known-good.json was updated on a warning run, want it left untouched")
	}
}

func TestRun_OffsiteSyncFails_StateStaysValidErrorReturned(t *testing.T) {
	dir := t.TempDir()
	cfg := healthyConfig(dir)
	deps := healthyDeps()
	deps.OffsiteSync = func(ctx context.Context, d string) (offsite.Result, error) {
		return offsite.Result{Success: false}, errors.New("remote unreachable")
	}

	result, err := Run(context.Background(), cfg, deps)
	if result.State != validation.StateValid {
		t.Errorf("State = %v, want StateValid - offsite failure must not affect snapshot state", result.State)
	}
	if err == nil {
		t.Error("Run() error = nil, want a non-nil error reporting the offsite failure")
	}
	if result.OffsiteSuccess {
		t.Error("OffsiteSuccess = true, want false")
	}
}

func TestRun_FirstRun_NoPreviousSnapshot_Valid(t *testing.T) {
	dir := t.TempDir()
	cfg := healthyConfig(dir)
	deps := healthyDeps() // LoadPrevious already returns found=false

	result, err := Run(context.Background(), cfg, deps)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.State != validation.StateValid {
		t.Errorf("State = %v, want StateValid on a first run", result.State)
	}
}

func TestRun_PruneFails_LoggedStateUnaffected(t *testing.T) {
	dir := t.TempDir()
	cfg := healthyConfig(dir)
	deps := healthyDeps()
	deps.Prune = func(d string, keep int) ([]string, error) {
		return nil, errors.New("permission denied")
	}

	result, err := Run(context.Background(), cfg, deps)
	if err != nil {
		t.Fatalf("Run() error = %v, want nil - prune failure must not fail the run", err)
	}
	if result.State != validation.StateValid {
		t.Errorf("State = %v, want StateValid despite the prune failure", result.State)
	}
}

func TestRun_ContextCancelledMidWalk_FailedNoSnapshotWritten(t *testing.T) {
	dir := t.TempDir()
	cfg := healthyConfig(dir)
	deps := healthyDeps()
	deps.WalkRoots = func(ctx context.Context, roots []filesystem.Root) (filesystem.Result, error) {
		return filesystem.Result{Complete: false}, context.Canceled
	}
	var wroteJSON bool
	deps.WriteJSON = func(d string, s inventory.Snapshot, now time.Time) (string, error) {
		wroteJSON = true
		return "", nil
	}

	result, err := Run(context.Background(), cfg, deps)
	if !errors.Is(err, context.Canceled) {
		t.Errorf("err = %v, want it to wrap context.Canceled", err)
	}
	if result.State != validation.StateFailed {
		t.Errorf("State = %v, want StateFailed", result.State)
	}
	if wroteJSON {
		t.Error("WriteJSON was called, want no snapshot promoted after a cancelled scan")
	}
}

func TestRun_StepOrdering(t *testing.T) {
	dir := t.TempDir()
	cfg := healthyConfig(dir)
	var calls []string
	deps := recordingDeps(&calls)

	if _, err := Run(context.Background(), cfg, deps); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	index := func(name string) int {
		for i, c := range calls {
			if c == name {
				return i
			}
		}
		t.Fatalf("call %q not recorded in %v", name, calls)
		return -1
	}

	if index("write_json") >= index("update_latest") {
		t.Errorf("update_latest happened before write_json: %v", calls)
	}
	if index("write_json") >= index("update_last_known_good") {
		t.Errorf("update_last_known_good happened before write_json: %v", calls)
	}
	if index("update_latest") >= index("offsite_sync") {
		t.Errorf("offsite_sync happened before update_latest: %v", calls)
	}
	// publish_metrics runs in a defer, so it is expected to be the true
	// last call; prune must be the last *step* before that.
	if last := calls[len(calls)-1]; last != "publish_metrics" {
		t.Errorf("last call = %q, want publish_metrics (deferred): %v", last, calls)
	}
	if index("prune") <= index("offsite_sync") {
		t.Errorf("prune did not happen after offsite_sync: %v", calls)
	}
}

func TestRun_LockReleasedOnEveryExitPath(t *testing.T) {
	dir := t.TempDir()
	cfg := healthyConfig(dir)

	// Happy path.
	if _, err := Run(context.Background(), cfg, healthyDeps()); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	release, err := runlock.Acquire(dir + "/" + lockFileName)
	if err != nil {
		t.Fatalf("lock not released after happy-path run: %v", err)
	}
	release()

	// Early-abort path (Radarr unreachable).
	deps := healthyDeps()
	deps.RadarrPing = func(ctx context.Context) error { return errors.New("down") }
	if _, err := Run(context.Background(), cfg, deps); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	release2, err := runlock.Acquire(dir + "/" + lockFileName)
	if err != nil {
		t.Fatalf("lock not released after early-abort run: %v", err)
	}
	release2()
}

func TestRun_AlreadyLocked_ReturnsErrAlreadyRunning(t *testing.T) {
	dir := t.TempDir()
	cfg := healthyConfig(dir)

	release, err := runlock.Acquire(dir + "/" + lockFileName)
	if err != nil {
		t.Fatalf("Acquire() error = %v", err)
	}
	defer release()

	_, err = Run(context.Background(), cfg, healthyDeps())
	if !errors.Is(err, ErrAlreadyRunning) {
		t.Errorf("err = %v, want ErrAlreadyRunning", err)
	}
}

func TestRun_FinalLogLine_ContainsStateDurationCounts(t *testing.T) {
	dir := t.TempDir()
	cfg := healthyConfig(dir)
	var buf bytes.Buffer
	deps := healthyDeps()
	deps.Logger = slog.New(slog.NewTextHandler(&buf, nil))

	if _, err := Run(context.Background(), cfg, deps); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	logged := buf.String()
	for _, want := range []string{"state=valid", "duration=", "offsite_success=true", "movies="} {
		if !strings.Contains(logged, want) {
			t.Errorf("log output missing %q:\n%s", want, logged)
		}
	}
}

// withPublishRecorder wraps deps.PublishMetrics so *called is set to true
// when it runs, without changing its actual (no-op) behavior.
func withPublishRecorder(deps Deps, called *bool) Deps {
	orig := deps.PublishMetrics
	deps.PublishMetrics = func(ctx context.Context, m influx.Metrics) {
		*called = true
		orig(ctx, m)
	}
	return deps
}
