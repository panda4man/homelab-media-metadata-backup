package validation

import (
	"strings"
	"testing"
)

func defaultThresholds() Thresholds {
	return Thresholds{
		MaxMediaBytesDecreasePercent: 5,
		MaxMovieDecreasePercent:      5,
		MaxEpisodeDecreasePercent:    5,
		MaxFilesRemoved:              100,
		MaxUnmatchedPercent:          5,
	}
}

// healthyContext is a RunContext where nothing should trigger: complete
// scan, both Arrs reachable, no inaccessible/empty roots, and a Diff with
// no meaningful shrinkage.
func healthyContext() RunContext {
	return RunContext{
		Diff:         Diff{},
		ScanComplete: true,
		RadarrUp:     true,
		SonarrUp:     true,
	}
}

func findCheck(t *testing.T, checks []Check, id string) Check {
	t.Helper()
	for _, c := range checks {
		if c.ID == id {
			return c
		}
	}
	t.Fatalf("no check with ID %q in %v", id, checks)
	return Check{}
}

func TestEvaluate_TotalBytesDecrease(t *testing.T) {
	tests := []struct {
		name    string
		pct     float64
		trigger bool
	}{
		{"just over threshold", -5.01, true},
		{"exactly at threshold does not trigger", -5.0, false},
		{"under threshold", -4.0, false},
		{"growth never triggers", 10, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := healthyContext()
			ctx.Diff.TotalSizeChangePercent = tt.pct
			c := findCheck(t, Evaluate(ctx, defaultThresholds()), "total_bytes_decrease")
			if c.Triggered != tt.trigger {
				t.Errorf("Triggered = %v, want %v (pct=%v)", c.Triggered, tt.trigger, tt.pct)
			}
		})
	}
}

func TestEvaluate_MovieCountDecrease(t *testing.T) {
	tests := []struct {
		name    string
		pct     float64
		trigger bool
	}{
		{"just over threshold", -5.5, true},
		{"exactly at threshold does not trigger", -5.0, false},
		{"under threshold", -1.0, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := healthyContext()
			ctx.Diff.MovieChangePercent = tt.pct
			c := findCheck(t, Evaluate(ctx, defaultThresholds()), "movie_count_decrease")
			if c.Triggered != tt.trigger {
				t.Errorf("Triggered = %v, want %v (pct=%v)", c.Triggered, tt.trigger, tt.pct)
			}
		})
	}
}

func TestEvaluate_EpisodeCountDecrease(t *testing.T) {
	tests := []struct {
		name    string
		pct     float64
		trigger bool
	}{
		{"just over threshold", -6.0, true},
		{"exactly at threshold does not trigger", -5.0, false},
		{"under threshold", -2.0, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := healthyContext()
			ctx.Diff.EpisodeChangePercent = tt.pct
			c := findCheck(t, Evaluate(ctx, defaultThresholds()), "episode_count_decrease")
			if c.Triggered != tt.trigger {
				t.Errorf("Triggered = %v, want %v (pct=%v)", c.Triggered, tt.trigger, tt.pct)
			}
		})
	}
}

func TestEvaluate_FilesRemoved(t *testing.T) {
	tests := []struct {
		name    string
		removed int
		trigger bool
	}{
		{"just over threshold", 101, true},
		{"exactly at threshold does not trigger", 100, false},
		{"under threshold", 50, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := healthyContext()
			ctx.Diff.FilesRemoved = tt.removed
			c := findCheck(t, Evaluate(ctx, defaultThresholds()), "files_removed_threshold")
			if c.Triggered != tt.trigger {
				t.Errorf("Triggered = %v, want %v (removed=%d)", c.Triggered, tt.trigger, tt.removed)
			}
		})
	}
}

func TestEvaluate_FilesRemoved_CustomThreshold(t *testing.T) {
	ctx := healthyContext()
	ctx.Diff.FilesRemoved = 11
	th := defaultThresholds()
	th.MaxFilesRemoved = 10

	c := findCheck(t, Evaluate(ctx, th), "files_removed_threshold")
	if !c.Triggered {
		t.Error("Triggered = false, want true with custom MaxFilesRemoved=10 and 11 removed")
	}
}

func TestEvaluate_UnmatchedPercent(t *testing.T) {
	tests := []struct {
		name             string
		unmatched, total int
		trigger          bool
	}{
		{"just over threshold", 6, 100, true},
		{"exactly at threshold does not trigger", 5, 100, false},
		{"under threshold", 1, 100, false},
		{"zero total files - no div by zero, no trigger", 0, 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := healthyContext()
			ctx.UnmatchedFiles = tt.unmatched
			ctx.TotalFiles = tt.total
			c := findCheck(t, Evaluate(ctx, defaultThresholds()), "unmatched_percent_threshold")
			if c.Triggered != tt.trigger {
				t.Errorf("Triggered = %v, want %v (unmatched=%d/%d)", c.Triggered, tt.trigger, tt.unmatched, tt.total)
			}
		})
	}
}

func TestEvaluate_SonarrUnreachable(t *testing.T) {
	ctx := healthyContext()
	ctx.SonarrUp = false
	c := findCheck(t, Evaluate(ctx, defaultThresholds()), "sonarr_unreachable")
	if !c.Triggered {
		t.Error("Triggered = false, want true when Sonarr is unreachable")
	}

	ctx.SonarrUp = true
	c = findCheck(t, Evaluate(ctx, defaultThresholds()), "sonarr_unreachable")
	if c.Triggered {
		t.Error("Triggered = true, want false when Sonarr is reachable")
	}
}

func TestEvaluate_RadarrUnreachable(t *testing.T) {
	ctx := healthyContext()
	ctx.RadarrUp = false
	c := findCheck(t, Evaluate(ctx, defaultThresholds()), "radarr_unreachable")
	if !c.Triggered {
		t.Error("Triggered = false, want true when Radarr is unreachable")
	}

	ctx.RadarrUp = true
	c = findCheck(t, Evaluate(ctx, defaultThresholds()), "radarr_unreachable")
	if c.Triggered {
		t.Error("Triggered = true, want false when Radarr is reachable")
	}
}

func TestEvaluate_MediaRootInaccessible(t *testing.T) {
	ctx := healthyContext()
	ctx.RootsInaccessible = []string{"movies"}
	c := findCheck(t, Evaluate(ctx, defaultThresholds()), "media_root_inaccessible")
	if !c.Triggered {
		t.Error("Triggered = false, want true with an inaccessible root")
	}
	if !strings.Contains(c.Detail, "movies") {
		t.Errorf("Detail = %q, want it to name the inaccessible root", c.Detail)
	}
}

func TestEvaluate_MediaRootEmpty(t *testing.T) {
	ctx := healthyContext()
	ctx.EmptyRoots = []string{"tv"}
	c := findCheck(t, Evaluate(ctx, defaultThresholds()), "media_root_empty")
	if !c.Triggered {
		t.Error("Triggered = false, want true with an unexpectedly empty root")
	}
	if !strings.Contains(c.Detail, "tv") {
		t.Errorf("Detail = %q, want it to name the empty root", c.Detail)
	}
}

func TestEvaluate_ScanIncomplete(t *testing.T) {
	ctx := healthyContext()
	ctx.ScanComplete = false
	c := findCheck(t, Evaluate(ctx, defaultThresholds()), "scan_incomplete")
	if !c.Triggered {
		t.Error("Triggered = false, want true when the scan did not complete")
	}
}

func TestEvaluate_HealthyGrowth_NothingTriggers(t *testing.T) {
	ctx := healthyContext()
	ctx.Diff.TotalSizeChangePercent = 10
	ctx.Diff.MovieChangePercent = 5
	ctx.Diff.EpisodeChangePercent = 8

	for _, c := range Evaluate(ctx, defaultThresholds()) {
		if c.Triggered {
			t.Errorf("check %q triggered on a healthy growing library", c.ID)
		}
	}
}

func TestEvaluate_FirstRun_NoPercentageChecksButRootChecksStillFire(t *testing.T) {
	ctx := RunContext{
		Diff:              Diff{IsFirstRun: true},
		ScanComplete:      true,
		RadarrUp:          false, // still fatal even on a first run
		SonarrUp:          true,
		RootsInaccessible: []string{"movies"},
	}

	checks := Evaluate(ctx, defaultThresholds())
	for _, id := range []string{"total_bytes_decrease", "movie_count_decrease", "episode_count_decrease", "files_removed_threshold"} {
		if c := findCheck(t, checks, id); c.Triggered {
			t.Errorf("check %q triggered on first run, want it suppressed", id)
		}
	}
	if c := findCheck(t, checks, "radarr_unreachable"); !c.Triggered {
		t.Error("radarr_unreachable did not trigger on first run despite Radarr being down")
	}
	if c := findCheck(t, checks, "media_root_inaccessible"); !c.Triggered {
		t.Error("media_root_inaccessible did not trigger on first run despite an inaccessible root")
	}
}

func TestEvaluate_MultipleTriggers_AllReturnedInStableOrder(t *testing.T) {
	ctx := healthyContext()
	ctx.RadarrUp = false
	ctx.SonarrUp = false
	ctx.ScanComplete = false

	checks := Evaluate(ctx, defaultThresholds())
	wantOrder := []string{
		"total_bytes_decrease", "movie_count_decrease", "episode_count_decrease",
		"files_removed_threshold", "unmatched_percent_threshold",
		"sonarr_unreachable", "radarr_unreachable",
		"media_root_inaccessible", "media_root_empty", "scan_incomplete",
	}
	if len(checks) != len(wantOrder) {
		t.Fatalf("len(checks) = %d, want %d", len(checks), len(wantOrder))
	}
	for i, id := range wantOrder {
		if checks[i].ID != id {
			t.Errorf("checks[%d].ID = %q, want %q (order must be stable)", i, checks[i].ID, id)
		}
	}

	triggeredCount := 0
	for _, c := range checks {
		if c.Triggered {
			triggeredCount++
		}
	}
	if triggeredCount != 3 {
		t.Errorf("triggeredCount = %d, want 3 (sonarr, radarr, scan_incomplete)", triggeredCount)
	}
}

func TestEvaluate_TriggeredCheck_DetailContainsActualAndThreshold(t *testing.T) {
	ctx := healthyContext()
	ctx.Diff.TotalSizeChangePercent = -12.5

	c := findCheck(t, Evaluate(ctx, defaultThresholds()), "total_bytes_decrease")
	if !strings.Contains(c.Detail, "12.5") {
		t.Errorf("Detail = %q, want it to contain the actual value 12.5", c.Detail)
	}
	if !strings.Contains(c.Detail, "5") {
		t.Errorf("Detail = %q, want it to contain the threshold value 5", c.Detail)
	}
}
