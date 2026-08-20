package orchestrator

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/panda4man/homelab-media-metadata-backup/internal/assemble"
	"github.com/panda4man/homelab-media-metadata-backup/internal/config"
	"github.com/panda4man/homelab-media-metadata-backup/internal/filesystem"
	"github.com/panda4man/homelab-media-metadata-backup/internal/influx"
	"github.com/panda4man/homelab-media-metadata-backup/internal/inventory"
	"github.com/panda4man/homelab-media-metadata-backup/internal/match"
	"github.com/panda4man/homelab-media-metadata-backup/internal/offsite"
	"github.com/panda4man/homelab-media-metadata-backup/internal/runlock"
	"github.com/panda4man/homelab-media-metadata-backup/internal/validation"
)

// ErrAlreadyRunning is returned when another run already holds the lock.
var ErrAlreadyRunning = runlock.ErrLocked

// Result summarizes what a run produced. OffsiteSuccess is reported
// separately from State: the spec treats "the snapshot is a trustworthy
// disaster-recovery record" and "the off-site copy is up to date" as two
// distinct kinds of success.
type Result struct {
	State          validation.State
	SnapshotPath   string
	OffsiteSuccess bool
}

const lockFileName = ".media-inventory.lock"

// LockPath returns the run lock file path for cfg, so callers outside this
// package (the HTTP API) can probe the same path Run itself acquires.
func LockPath(cfg config.Config) string {
	return cfg.SnapshotPath + "/" + lockFileName
}

// Run executes one full scheduled run: verify sources, gather metadata,
// walk the filesystem, build and write the snapshot, validate it, sync
// off-site, publish metrics, and prune old snapshots - all behind a
// single-instance lock. Every exit path releases the lock and publishes
// metrics exactly once, including the early-abort paths where no
// snapshot was ever built.
func Run(ctx context.Context, cfg config.Config, deps Deps) (result Result, err error) {
	start := deps.Clock.Now()
	logger := deps.Logger

	if mkErr := os.MkdirAll(cfg.SnapshotPath, 0755); mkErr != nil {
		return Result{State: validation.StateFailed}, fmt.Errorf("orchestrator: creating snapshot directory: %w", mkErr)
	}

	release, lockErr := runlock.Acquire(LockPath(cfg))
	if lockErr != nil {
		return Result{}, lockErr
	}
	defer release()

	logger.Info("inventory run starting")

	var (
		snap          inventory.Snapshot
		diff          validation.Diff
		state         = validation.StateFailed
		offsiteResult offsite.Result
	)

	defer func() {
		duration := deps.Clock.Now().Sub(start)
		metrics := buildMetrics(snap, diff, state, duration, offsiteResult)
		deps.PublishMetrics(ctx, metrics)
		logger.Info("inventory run completed",
			"state", state.String(),
			"duration", duration,
			"offsite_success", offsiteResult.Success,
			"offsite_skipped", offsiteResult.Skipped,
			"movies", snap.Summary.Movies,
			"series", snap.Summary.Series,
			"episodes", snap.Summary.Episodes,
		)
	}()

	// Steps 3-4: verify Sonarr and Radarr are reachable.
	sonarrUp := true
	if pingErr := deps.SonarrPing(ctx); pingErr != nil {
		sonarrUp = false
		logger.Error("sonarr unreachable", "error", pingErr)
	}
	radarrUp := true
	if pingErr := deps.RadarrPing(ctx); pingErr != nil {
		radarrUp = false
		logger.Error("radarr unreachable", "error", pingErr)
	}
	if !sonarrUp || !radarrUp {
		state = validation.Resolve(validation.Evaluate(validation.RunContext{
			ScanComplete: true,
			RadarrUp:     radarrUp,
			SonarrUp:     sonarrUp,
		}, thresholdsFrom(cfg)))
		return Result{State: state}, nil
	}

	// Step 5: load Sonarr metadata (series, then per-series episodes).
	logger.Info("loading sonarr/radarr metadata")
	seriesList, seriesErr := deps.SonarrSeries(ctx)
	if seriesErr != nil {
		sonarrUp = false
		logger.Error("failed to load sonarr series", "error", seriesErr)
		state = validation.Resolve(validation.Evaluate(validation.RunContext{
			ScanComplete: true, RadarrUp: radarrUp, SonarrUp: sonarrUp,
		}, thresholdsFrom(cfg)))
		return Result{State: state}, nil
	}
	var seriesWithEpisodes []match.SeriesWithEpisodes
	for _, s := range seriesList {
		episodes, epErr := deps.SonarrEpisodes(ctx, s.ID)
		if epErr != nil {
			logger.Warn("failed to load episodes for series, skipping it this run", "series", s.Title, "error", epErr)
			continue
		}
		seriesWithEpisodes = append(seriesWithEpisodes, match.SeriesWithEpisodes{Series: s, Episodes: episodes})
	}

	// Step 6: load Radarr metadata.
	movies, moviesErr := deps.RadarrMovies(ctx)
	if moviesErr != nil {
		radarrUp = false
		logger.Error("failed to load radarr movies", "error", moviesErr)
		state = validation.Resolve(validation.Evaluate(validation.RunContext{
			ScanComplete: true, RadarrUp: radarrUp, SonarrUp: sonarrUp,
		}, thresholdsFrom(cfg)))
		return Result{State: state}, nil
	}

	// Steps 2+7: verify roots and walk the filesystem in one pass.
	roots := []filesystem.Root{
		{Name: "movies", Path: cfg.MediaMoviesPath},
		{Name: "tv", Path: cfg.MediaTVPath},
	}
	logger.Info("scanning filesystem", "roots", []string{"movies", "tv"})
	walkResult, walkErr := deps.WalkRoots(ctx, roots)
	if walkErr != nil && (errors.Is(walkErr, context.Canceled) || errors.Is(walkErr, context.DeadlineExceeded)) {
		logger.Error("scan cancelled", "error", walkErr)
		state = validation.StateFailed
		return Result{State: state}, walkErr
	}
	logger.Info("filesystem scan complete", "files", len(walkResult.Files), "roots_scanned", walkResult.RootsScanned)
	// A root-level failure (walkErr wrapping ErrRootInaccessible) does not
	// abort the run: continue with whatever files the other root(s)
	// yielded, write a forensic snapshot, and let the sanity checks below
	// flag it as failed. This matches "a suspicious/failed snapshot is
	// still retained" from the spec's failure philosophy.

	movieFiles, tvFiles := splitFilesByRoot(walkResult.Files)

	// Steps 8-9: match against Arr metadata and build the snapshot.
	snap, buildStats := assemble.Build(assemble.Input{
		Hostname:     deps.Hostname,
		GeneratedAt:  deps.Clock.Now(),
		MovieFiles:   movieFiles,
		RadarrMovies: movies,
		TVFiles:      tvFiles,
		Series:       seriesWithEpisodes,
	})

	// Step 10: load the previous successful snapshot.
	prevSnap, prevFound, prevErr := deps.LoadPrevious(cfg.SnapshotPath)
	if prevErr != nil {
		logger.Warn("failed to load previous snapshot, treating this run as a first run", "error", prevErr)
		prevFound = false
	}

	// Step 11: diff.
	diff = validation.Compare(prevSnap, prevFound, snap)

	// Step 12: sanity checks.
	totalFiles := len(movieFiles) + len(tvFiles)
	checks := validation.Evaluate(validation.RunContext{
		Diff:              diff,
		UnmatchedFiles:    buildStats.UnmatchedFiles,
		TotalFiles:        totalFiles,
		RootsInaccessible: rootFailureNames(walkResult.RootsFailed),
		EmptyRoots:        emptyRootNames(roots, walkResult),
		ScanComplete:      walkResult.Complete,
		RadarrUp:          radarrUp,
		SonarrUp:          sonarrUp,
	}, thresholdsFrom(cfg))
	state = validation.Resolve(checks)
	for _, c := range checks {
		if c.Triggered {
			logger.Warn("sanity check triggered", "check", c.ID, "detail", c.Detail)
		}
	}

	// Step 13: write the dated JSON snapshot. This is retained regardless
	// of state - even a failed run's snapshot has forensic value.
	jsonPath, writeErr := deps.WriteJSON(cfg.SnapshotPath, snap, deps.Clock.Now())
	if writeErr != nil {
		logger.Error("failed to write snapshot json", "error", writeErr)
		state = validation.StateFailed
		return Result{State: state}, writeErr
	}
	result.SnapshotPath = jsonPath

	// Step 14: CSV exports (best-effort; JSON remains authoritative).
	if _, csvErr := deps.WriteMoviesCSV(cfg.SnapshotPath, snap.Movies, deps.Clock.Now()); csvErr != nil {
		logger.Warn("failed to write movies csv", "error", csvErr)
	}
	if _, csvErr := deps.WriteEpisodesCSV(cfg.SnapshotPath, snap.Series, deps.Clock.Now()); csvErr != nil {
		logger.Warn("failed to write episodes csv", "error", csvErr)
	}

	// Step 15: checksum sidecar (best-effort).
	if _, shaErr := deps.WriteSHA256(jsonPath); shaErr != nil {
		logger.Warn("failed to write sha256 sidecar", "error", shaErr)
	}

	// Steps 16-17: update pointers according to state.
	if state == validation.StateValid || state == validation.StateWarning {
		if updErr := deps.UpdateLatest(cfg.SnapshotPath, jsonPath); updErr != nil {
			logger.Error("failed to update latest.json", "error", updErr)
		}
	}
	if state == validation.StateValid {
		if updErr := deps.UpdateLastKnownGood(cfg.SnapshotPath, jsonPath); updErr != nil {
			logger.Error("failed to update last-known-good.json", "error", updErr)
		}
	}

	// Step 18: off-site sync.
	logger.Info("starting offsite sync")
	offsiteResult, offsiteErr := deps.OffsiteSync(ctx, cfg.SnapshotPath)
	if offsiteErr != nil {
		logger.Error("offsite sync failed", "error", offsiteErr)
	} else {
		logger.Info("offsite sync complete", "success", offsiteResult.Success, "skipped", offsiteResult.Skipped)
	}

	// Step 19 (metrics) happens in the deferred func above.

	// Step 20: retention pruning (best-effort; never changes state).
	if _, pruneErr := deps.Prune(cfg.SnapshotPath, cfg.SnapshotRetention); pruneErr != nil {
		logger.Warn("retention pruning failed", "error", pruneErr)
	}

	result.State = state
	result.OffsiteSuccess = offsiteResult.Success || offsiteResult.Skipped
	if offsiteErr != nil {
		return result, fmt.Errorf("orchestrator: off-site sync failed: %w", offsiteErr)
	}
	return result, nil
}

func thresholdsFrom(cfg config.Config) validation.Thresholds {
	return validation.Thresholds{
		MaxMediaBytesDecreasePercent: cfg.MaxMediaBytesDecreasePercent,
		MaxMovieDecreasePercent:      cfg.MaxMovieDecreasePercent,
		MaxEpisodeDecreasePercent:    cfg.MaxEpisodeDecreasePercent,
		MaxFilesRemoved:              cfg.MaxFilesRemoved,
		MaxUnmatchedPercent:          cfg.MaxUnmatchedPercent,
	}
}

func splitFilesByRoot(files []filesystem.File) (movieFiles, tvFiles []filesystem.File) {
	for _, f := range files {
		switch f.Root {
		case "movies":
			movieFiles = append(movieFiles, f)
		case "tv":
			tvFiles = append(tvFiles, f)
		}
	}
	return movieFiles, tvFiles
}

func rootFailureNames(failed []filesystem.RootFailure) []string {
	names := make([]string, len(failed))
	for i, f := range failed {
		names[i] = f.Root
	}
	return names
}

func emptyRootNames(roots []filesystem.Root, result filesystem.Result) []string {
	hasFiles := make(map[string]bool)
	for _, f := range result.Files {
		hasFiles[f.Root] = true
	}
	scanned := make(map[string]bool)
	for _, name := range result.RootsScanned {
		scanned[name] = true
	}

	var empty []string
	for _, r := range roots {
		if scanned[r.Name] && !hasFiles[r.Name] {
			empty = append(empty, r.Name)
		}
	}
	return empty
}

func buildMetrics(snap inventory.Snapshot, diff validation.Diff, state validation.State, duration time.Duration, offsiteResult offsite.Result) influx.Metrics {
	return influx.Metrics{
		Movies:               snap.Summary.Movies,
		Series:               snap.Summary.Series,
		Episodes:             snap.Summary.Episodes,
		MediaFiles:           snap.Summary.MediaFiles,
		TotalBytes:           snap.Summary.TotalBytes,
		MoviesAdded:          diff.MoviesAdded,
		MoviesRemoved:        diff.MoviesRemoved,
		SeriesAdded:          diff.SeriesAdded,
		SeriesRemoved:        diff.SeriesRemoved,
		EpisodesAdded:        diff.EpisodesAdded,
		EpisodesRemoved:      diff.EpisodesRemoved,
		FilesAdded:           diff.FilesAdded,
		FilesRemoved:         diff.FilesRemoved,
		BytesAdded:           diff.BytesAdded,
		BytesRemoved:         diff.BytesRemoved,
		ScanDurationSeconds:  duration.Seconds(),
		SnapshotValid:        state == validation.StateValid,
		SnapshotWarning:      state == validation.StateWarning,
		OffsiteUploadSuccess: offsiteResult.Success,
	}
}
