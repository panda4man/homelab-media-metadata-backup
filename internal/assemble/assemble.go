// Package assemble builds a complete inventory.Snapshot from already
// fetched/walked data. Build performs no I/O of its own - the caller
// (the orchestrator) is responsible for walking the filesystem and calling
// Radarr/Sonarr before handing their results to Build.
package assemble

import (
	"time"

	"github.com/panda4man/homelab-media-metadata-backup/internal/filesystem"
	"github.com/panda4man/homelab-media-metadata-backup/internal/inventory"
	"github.com/panda4man/homelab-media-metadata-backup/internal/match"
	"github.com/panda4man/homelab-media-metadata-backup/internal/radarr"
)

// Input is everything Build needs to assemble a snapshot.
type Input struct {
	Hostname    string
	GeneratedAt time.Time

	MovieFiles   []filesystem.File
	RadarrMovies []radarr.Movie

	TVFiles []filesystem.File
	Series  []match.SeriesWithEpisodes
}

// Stats reports counters from the match step, meant to feed the
// validation and metrics stages.
type Stats struct {
	UnmatchedFiles int
}

// Build matches Input's raw filesystem/Arr data into a complete inventory
// Snapshot with correct summary totals.
func Build(in Input) (inventory.Snapshot, Stats) {
	matchedMovies, unmatchedMovies := match.MatchMovies(in.MovieFiles, in.RadarrMovies)
	matchedSeries, unmatchedTV := match.MatchEpisodes(in.TVFiles, in.Series)

	if matchedMovies == nil {
		matchedMovies = []inventory.Movie{}
	}
	if matchedSeries == nil {
		matchedSeries = []inventory.Series{}
	}

	snapshot := inventory.NewSnapshot(in.GeneratedAt, in.Hostname, matchedMovies, matchedSeries)
	stats := Stats{UnmatchedFiles: len(unmatchedMovies) + len(unmatchedTV)}
	return snapshot, stats
}
