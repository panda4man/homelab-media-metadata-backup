// Package orchestrator wires every other package into the scheduled
// run's workflow: verify sources, gather metadata, walk the filesystem,
// build and write the snapshot, validate it, sync off-site, publish
// metrics, and prune old snapshots - all behind a single-instance lock.
package orchestrator

import (
	"context"
	"log/slog"
	"time"

	"github.com/panda4man/homelab-media-metadata-backup/internal/clockx"
	"github.com/panda4man/homelab-media-metadata-backup/internal/filesystem"
	"github.com/panda4man/homelab-media-metadata-backup/internal/influx"
	"github.com/panda4man/homelab-media-metadata-backup/internal/inventory"
	"github.com/panda4man/homelab-media-metadata-backup/internal/offsite"
	"github.com/panda4man/homelab-media-metadata-backup/internal/radarr"
	"github.com/panda4man/homelab-media-metadata-backup/internal/sonarr"
)

// Deps is every external capability Run needs, as plain functions rather
// than named interfaces - each field is declared here, at the consumer,
// and satisfied directly by the real package functions/methods in
// production or by closures in tests. This avoids a wrapper interface
// type existing solely to make one function fakable.
type Deps struct {
	Clock    clockx.Clock
	Logger   *slog.Logger
	Hostname string

	WalkRoots func(ctx context.Context, roots []filesystem.Root) (filesystem.Result, error)

	RadarrPing   func(ctx context.Context) error
	RadarrMovies func(ctx context.Context) ([]radarr.Movie, error)

	SonarrPing     func(ctx context.Context) error
	SonarrSeries   func(ctx context.Context) ([]sonarr.Series, error)
	SonarrEpisodes func(ctx context.Context, seriesID int) ([]sonarr.Episode, error)

	WriteJSON           func(dir string, s inventory.Snapshot, now time.Time) (string, error)
	WriteSHA256         func(jsonPath string) (string, error)
	WriteMoviesCSV      func(dir string, movies []inventory.Movie, now time.Time) (string, error)
	WriteEpisodesCSV    func(dir string, series []inventory.Series, now time.Time) (string, error)
	LoadPrevious        func(dir string) (inventory.Snapshot, bool, error)
	UpdateLatest        func(dir, snapshotPath string) error
	UpdateLastKnownGood func(dir, snapshotPath string) error
	Prune               func(dir string, keep int) ([]string, error)

	OffsiteSync func(ctx context.Context, localDir string) (offsite.Result, error)

	PublishMetrics func(ctx context.Context, m influx.Metrics)
}
