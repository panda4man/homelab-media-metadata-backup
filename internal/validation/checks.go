package validation

import "fmt"

// Thresholds holds the configurable sanity-check limits. It is populated
// from config.Config by the caller - this package does not import config,
// to keep the dependency direction one-way.
type Thresholds struct {
	MaxMediaBytesDecreasePercent float64
	MaxMovieDecreasePercent      float64
	MaxEpisodeDecreasePercent    float64
	MaxFilesRemoved              int
	MaxUnmatchedPercent          float64
}

// RunContext is everything Evaluate needs to know about a single run,
// gathered by the orchestrator before sanity checks are evaluated.
type RunContext struct {
	Diff Diff

	UnmatchedFiles int
	TotalFiles     int

	RootsInaccessible []string
	EmptyRoots        []string
	ScanComplete      bool

	RadarrUp bool
	SonarrUp bool
}

// Check is the result of evaluating one sanity check.
type Check struct {
	ID          string
	Description string
	Triggered   bool
	Detail      string
}

// Evaluate runs every default sanity check against ctx and returns all of
// them, in a fixed stable order, each carrying whether it triggered and a
// human-readable detail of the actual value versus its threshold.
// Percentage-based checks never trigger on a first run, since Compare
// already zeroes those fields when ctx.Diff.IsFirstRun is true.
func Evaluate(ctx RunContext, t Thresholds) []Check {
	unmatchedPercent := 0.0
	if ctx.TotalFiles > 0 {
		unmatchedPercent = float64(ctx.UnmatchedFiles) / float64(ctx.TotalFiles) * 100
	}

	return []Check{
		{
			ID:          "total_bytes_decrease",
			Description: "total media bytes decreased by more than the configured threshold",
			Triggered:   ctx.Diff.TotalSizeChangePercent < -t.MaxMediaBytesDecreasePercent,
			Detail:      fmt.Sprintf("changed %.2f%%, threshold %.2f%%", ctx.Diff.TotalSizeChangePercent, -t.MaxMediaBytesDecreasePercent),
		},
		{
			ID:          "movie_count_decrease",
			Description: "movie count decreased by more than the configured threshold",
			Triggered:   ctx.Diff.MovieChangePercent < -t.MaxMovieDecreasePercent,
			Detail:      fmt.Sprintf("changed %.2f%%, threshold %.2f%%", ctx.Diff.MovieChangePercent, -t.MaxMovieDecreasePercent),
		},
		{
			ID:          "episode_count_decrease",
			Description: "episode count decreased by more than the configured threshold",
			Triggered:   ctx.Diff.EpisodeChangePercent < -t.MaxEpisodeDecreasePercent,
			Detail:      fmt.Sprintf("changed %.2f%%, threshold %.2f%%", ctx.Diff.EpisodeChangePercent, -t.MaxEpisodeDecreasePercent),
		},
		{
			ID:          "files_removed_threshold",
			Description: "more media files disappeared than the configured threshold",
			Triggered:   ctx.Diff.FilesRemoved > t.MaxFilesRemoved,
			Detail:      fmt.Sprintf("removed %d, threshold %d", ctx.Diff.FilesRemoved, t.MaxFilesRemoved),
		},
		{
			ID:          "unmatched_percent_threshold",
			Description: "more discovered media is unmatched against Sonarr/Radarr than the configured threshold",
			Triggered:   unmatchedPercent > t.MaxUnmatchedPercent,
			Detail:      fmt.Sprintf("unmatched %.2f%%, threshold %.2f%%", unmatchedPercent, t.MaxUnmatchedPercent),
		},
		{
			ID:          "sonarr_unreachable",
			Description: "Sonarr could not be reached",
			Triggered:   !ctx.SonarrUp,
			Detail:      fmt.Sprintf("reachable=%v", ctx.SonarrUp),
		},
		{
			ID:          "radarr_unreachable",
			Description: "Radarr could not be reached",
			Triggered:   !ctx.RadarrUp,
			Detail:      fmt.Sprintf("reachable=%v", ctx.RadarrUp),
		},
		{
			ID:          "media_root_inaccessible",
			Description: "one or more configured media roots could not be accessed",
			Triggered:   len(ctx.RootsInaccessible) > 0,
			Detail:      fmt.Sprintf("inaccessible roots: %v", ctx.RootsInaccessible),
		},
		{
			ID:          "media_root_empty",
			Description: "a configured media root unexpectedly contained zero files",
			Triggered:   len(ctx.EmptyRoots) > 0,
			Detail:      fmt.Sprintf("empty roots: %v", ctx.EmptyRoots),
		},
		{
			ID:          "scan_incomplete",
			Description: "the scan terminated before all configured roots were processed",
			Triggered:   !ctx.ScanComplete,
			Detail:      fmt.Sprintf("complete=%v", ctx.ScanComplete),
		},
	}
}
