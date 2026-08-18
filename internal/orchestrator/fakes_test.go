package orchestrator

import (
	"context"
	"io"
	"log/slog"
	"time"

	"github.com/panda4man/homelab-media-metadata-backup/internal/clockx"
	"github.com/panda4man/homelab-media-metadata-backup/internal/config"
	"github.com/panda4man/homelab-media-metadata-backup/internal/filesystem"
	"github.com/panda4man/homelab-media-metadata-backup/internal/influx"
	"github.com/panda4man/homelab-media-metadata-backup/internal/inventory"
	"github.com/panda4man/homelab-media-metadata-backup/internal/offsite"
	"github.com/panda4man/homelab-media-metadata-backup/internal/radarr"
	"github.com/panda4man/homelab-media-metadata-backup/internal/sonarr"
)

var fixedNow = time.Date(2026, 8, 16, 23, 0, 0, 0, time.UTC)

func healthyConfig(snapshotDir string) config.Config {
	return config.Config{
		TZ:                           "UTC",
		MediaMoviesPath:              "/media/movies",
		MediaTVPath:                  "/media/tv",
		RadarrURL:                    "http://radarr:7878",
		RadarrAPIKey:                 "k",
		SonarrURL:                    "http://sonarr:8989",
		SonarrAPIKey:                 "k",
		SnapshotPath:                 snapshotDir,
		SnapshotRetention:            5,
		MaxMediaBytesDecreasePercent: 5,
		MaxMovieDecreasePercent:      5,
		MaxEpisodeDecreasePercent:    5,
		MaxFilesRemoved:              100,
		MaxUnmatchedPercent:          5,
	}
}

// healthyDeps returns a Deps fixture where every step succeeds with a
// small, consistent one-movie inventory. Tests override individual
// fields to exercise a specific failure path.
func healthyDeps() Deps {
	return Deps{
		Clock:    clockx.Fixed{Instant: fixedNow},
		Logger:   slog.New(slog.NewTextHandler(io.Discard, nil)),
		Hostname: "test-host",

		WalkRoots: func(ctx context.Context, roots []filesystem.Root) (filesystem.Result, error) {
			return filesystem.Result{
				Complete:     true,
				RootsScanned: []string{"movies", "tv"},
				Files: []filesystem.File{
					{Root: "movies", RelPath: "A (2001)/a.mkv", Bytes: 100, ModTime: fixedNow},
					{Root: "tv", RelPath: "Show (2020)/Season 01/S01E01.mkv", Bytes: 200, ModTime: fixedNow},
				},
			}, nil
		},

		RadarrPing: func(ctx context.Context) error { return nil },
		RadarrMovies: func(ctx context.Context) ([]radarr.Movie, error) {
			return []radarr.Movie{
				{Title: "A", Year: 2001, TMDBID: 1, Path: "/media/movies/A (2001)", RelativePath: "a.mkv", Bytes: 100, HasFile: true},
			}, nil
		},

		SonarrPing: func(ctx context.Context) error { return nil },
		SonarrSeries: func(ctx context.Context) ([]sonarr.Series, error) {
			return []sonarr.Series{
				{ID: 1, Title: "Show", Year: 2020, TVDBID: 1, Path: "/media/tv/Show (2020)"},
			}, nil
		},
		SonarrEpisodes: func(ctx context.Context, seriesID int) ([]sonarr.Episode, error) {
			return []sonarr.Episode{
				{Season: 1, Episode: 1, Title: "Pilot", HasFile: true, RelativePath: "Season 01/S01E01.mkv", Bytes: 200},
			}, nil
		},

		WriteJSON: func(dir string, s inventory.Snapshot, now time.Time) (string, error) {
			return dir + "/media-inventory-test.json", nil
		},
		WriteSHA256: func(jsonPath string) (string, error) { return jsonPath + ".sha256", nil },
		WriteMoviesCSV: func(dir string, movies []inventory.Movie, now time.Time) (string, error) {
			return dir + "/media-inventory-test-movies.csv", nil
		},
		WriteEpisodesCSV: func(dir string, series []inventory.Series, now time.Time) (string, error) {
			return dir + "/media-inventory-test-episodes.csv", nil
		},
		LoadPrevious: func(dir string) (inventory.Snapshot, bool, error) {
			return inventory.Snapshot{}, false, nil
		},
		UpdateLatest:        func(dir, snapshotPath string) error { return nil },
		UpdateLastKnownGood: func(dir, snapshotPath string) error { return nil },
		Prune:               func(dir string, keep int) ([]string, error) { return nil, nil },

		OffsiteSync: func(ctx context.Context, localDir string) (offsite.Result, error) {
			return offsite.Result{Success: true}, nil
		},

		PublishMetrics: func(ctx context.Context, m influx.Metrics) {},
	}
}

// recordingDeps wraps healthyDeps so every step appends its name to
// *calls before doing its (still successful) work, so tests can assert
// on relative ordering.
func recordingDeps(calls *[]string) Deps {
	d := healthyDeps()

	origWalk := d.WalkRoots
	d.WalkRoots = func(ctx context.Context, roots []filesystem.Root) (filesystem.Result, error) {
		*calls = append(*calls, "walk")
		return origWalk(ctx, roots)
	}
	origWriteJSON := d.WriteJSON
	d.WriteJSON = func(dir string, s inventory.Snapshot, now time.Time) (string, error) {
		*calls = append(*calls, "write_json")
		return origWriteJSON(dir, s, now)
	}
	origUpdateLatest := d.UpdateLatest
	d.UpdateLatest = func(dir, snapshotPath string) error {
		*calls = append(*calls, "update_latest")
		return origUpdateLatest(dir, snapshotPath)
	}
	origUpdateLKG := d.UpdateLastKnownGood
	d.UpdateLastKnownGood = func(dir, snapshotPath string) error {
		*calls = append(*calls, "update_last_known_good")
		return origUpdateLKG(dir, snapshotPath)
	}
	origOffsite := d.OffsiteSync
	d.OffsiteSync = func(ctx context.Context, localDir string) (offsite.Result, error) {
		*calls = append(*calls, "offsite_sync")
		return origOffsite(ctx, localDir)
	}
	origPrune := d.Prune
	d.Prune = func(dir string, keep int) ([]string, error) {
		*calls = append(*calls, "prune")
		return origPrune(dir, keep)
	}
	origPublish := d.PublishMetrics
	d.PublishMetrics = func(ctx context.Context, m influx.Metrics) {
		*calls = append(*calls, "publish_metrics")
		origPublish(ctx, m)
	}
	return d
}
